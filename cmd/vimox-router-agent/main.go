package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/neequ/vimox-router-agent/internal/agent"
	"github.com/neequ/vimox-router-agent/internal/router"
	"golang.org/x/term"
)

var version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "version" || args[0] == "--version" {
		fmt.Printf("vimox-router-agent %s\n", version)
		return nil
	}
	switch args[0] {
	case "account":
		return account(args[1:])
	case "models":
		return authenticatedRequest("GET", "/v1/models", nil)
	case "create":
		return create(args[1:])
	case "task":
		return task(args[1:])
	case "mcp":
		key, err := router.LoadKey()
		if err != nil {
			return err
		}
		return agent.RunStdio(key)
	case "serve":
		return serve(args[1:])
	default:
		return errors.New("usage: vimox-router-agent account|models|create|task|mcp|serve")
	}
}

func account(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: account captcha|send-code|register|login|status|logout")
	}
	ctx := context.Background()
	client := router.New("")
	switch args[0] {
	case "captcha":
		flags := flag.NewFlagSet("account captcha", flag.ContinueOnError)
		out := flags.String("out", "router-captcha.svg", "captcha SVG output path")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		request, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer request.Close()
		endpoint := client.BaseURL + "/users/auth/captcha"
		id, err := randomID()
		if err != nil {
			return err
		}
		req, err := httpRequest(ctx, "GET", endpoint, nil, "X-Vimox-Router-Captcha-Session", id)
		if err != nil {
			return err
		}
		var payload struct {
			Image     string `json:"image_data"`
			CaptchaID string `json:"captcha_id"`
		}
		if err := json.Unmarshal(req, &payload); err != nil {
			return err
		}
		if payload.CaptchaID == "" {
			return errors.New("Router captcha response did not include captcha_id")
		}
		encoded := strings.TrimPrefix(payload.Image, "data:image/svg+xml;base64,")
		image, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("decode captcha image: %w", err)
		}
		if _, err := request.Write(image); err != nil {
			return err
		}
		fmt.Printf("Saved captcha to %s\nCaptcha ID: %s\n", *out, payload.CaptchaID)
		return nil
	case "send-code":
		flags := flag.NewFlagSet("account send-code", flag.ContinueOnError)
		channel := flags.String("channel", "email", "email or phone")
		identifier := flags.String("identifier", "", "account email or phone")
		captchaID := flags.String("captcha-id", "", "phone captcha ID")
		captchaAnswer := flags.String("captcha-answer", "", "phone captcha answer")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		body := map[string]string{"channel": *channel, "identifier": *identifier}
		if *captchaID != "" {
			body["captcha_id"] = *captchaID
		}
		if *captchaAnswer != "" {
			body["captcha_answer"] = *captchaAnswer
		}
		return accountRequest(ctx, client, "/users/auth/send-code", body)
	case "register", "login":
		flags := flag.NewFlagSet("account "+args[0], flag.ContinueOnError)
		channel := flags.String("channel", "email", "email or phone")
		identifier := flags.String("identifier", "", "account email or phone")
		code := flags.String("code", "", "registration verification code")
		passwordStdin := flags.Bool("password-stdin", false, "read password from stdin instead of a hidden terminal prompt")
		referral := flags.String("referral-code", "", "optional invitation code")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		password, err := readPassword(*passwordStdin)
		if err != nil {
			return err
		}
		if len(password) == 0 || len(password) > 4096 {
			return errors.New("password input is empty or too long")
		}
		body := map[string]string{"channel": *channel, "identifier": *identifier, "password": strings.TrimRight(string(password), "\r\n"), "redirectUri": "https://router.vimox.cn/users/oauth/callback"}
		path := "/users/auth/login"
		if args[0] == "register" {
			path = "/users/auth/register"
			body["code"] = *code
			if *referral != "" {
				body["referralCode"] = *referral
			}
		}
		session, err := accountResponse(ctx, client, path, body)
		if err != nil {
			return err
		}
		token := firstString(session, "accessToken", "access_token", "token")
		if token == "" && args[0] == "register" {
			session, err = accountResponse(ctx, client, "/users/auth/login", map[string]string{"channel": *channel, "identifier": *identifier, "password": body["password"], "redirectUri": "https://router.vimox.cn/users/oauth/callback"})
			if err != nil {
				return fmt.Errorf("registration succeeded but automatic sign-in failed: %w", err)
			}
			token = firstString(session, "accessToken", "access_token", "token")
		}
		if token == "" {
			return errors.New("account response did not include an access token")
		}
		keyResponse, err := accountResponse(ctx, client, "/users/api-keys", map[string]string{"name": "Vimox Router Agent"}, token)
		if err != nil {
			return fmt.Errorf("account login succeeded but API key creation failed: %w", err)
		}
		key := firstString(keyResponse, "key")
		if key == "" {
			return errors.New("Router did not return the created API key")
		}
		if err := router.SaveKey(key); err != nil {
			return err
		}
		fmt.Printf("API key created and saved to %s\n", router.ConfigPath())
		return nil
	case "status":
		key, err := router.LoadKey()
		if err != nil {
			return err
		}
		if key == "" {
			return errors.New("not signed in; run account login")
		}
		fmt.Printf("Router API key configured (%s)\n", router.ConfigPath())
		return nil
	case "logout":
		if err := router.ClearKey(); err != nil {
			return err
		}
		fmt.Println("Local API key removed. Revoke it from the Router portal to disable it.")
		return nil
	default:
		return fmt.Errorf("unknown account command %q", args[0])
	}
}

func readPassword(fromStdin bool) ([]byte, error) {
	if fromStdin {
		return io.ReadAll(io.LimitReader(os.Stdin, 4097))
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, errors.New("password requires an interactive terminal; use --password-stdin to read a pipe")
	}
	fmt.Fprint(os.Stderr, "Password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return password, err
}

func create(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: create text|image|video --model MODEL --prompt PROMPT")
	}
	flags := flag.NewFlagSet("create "+args[0], flag.ContinueOnError)
	model := flags.String("model", "", "public Router model ID")
	prompt := flags.String("prompt", "", "complete prompt")
	size := flags.String("size", "", "image size")
	seconds := flags.String("seconds", "", "video duration in seconds")
	confirmed := flags.Bool("confirm", false, "confirm this paid model request")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *model == "" || *prompt == "" {
		return errors.New("--model and --prompt are required")
	}
	if !*confirmed {
		return errors.New("model requests are billable; review the model and prompt, then pass --confirm")
	}
	var path string
	body := map[string]any{"model": *model, "prompt": *prompt}
	switch args[0] {
	case "text":
		path = "/v1/chat/completions"
	case "image":
		path = "/v1/images/generations"
		if *size != "" {
			body["size"] = *size
		}
	case "video":
		path = "/v1/videos"
		if *seconds != "" {
			body["seconds"] = *seconds
		}
	default:
		return fmt.Errorf("unknown creation type %q", args[0])
	}
	var payload any = body
	if args[0] == "text" {
		payload = map[string]any{"model": *model, "messages": []map[string]string{{"role": "user", "content": *prompt}}}
	}
	return authenticatedRequest("POST", path, payload)
}

func task(args []string) error {
	if len(args) != 2 || args[0] != "get" {
		return errors.New("usage: task get TASK_ID")
	}
	return authenticatedRequest("GET", "/v1/videos/"+url.PathEscape(args[1]), nil)
}

func authenticatedRequest(method, path string, body any) error {
	key, err := router.LoadKey()
	if err != nil {
		return err
	}
	if key == "" {
		return errors.New("not signed in; run account login")
	}
	data, _, err := router.New(key).Request(context.Background(), method, path, body)
	if err != nil {
		return err
	}
	var pretty any
	if json.Unmarshal(data, &pretty) == nil {
		data, _ = json.MarshalIndent(pretty, "", "  ")
	}
	fmt.Println(string(data))
	return nil
}

func accountRequest(ctx context.Context, client *router.Client, path string, body any) error {
	if _, _, err := client.Request(ctx, "POST", path, body); err != nil {
		return err
	}
	fmt.Println("Request completed")
	return nil
}

func accountResponse(ctx context.Context, client *router.Client, path string, body any, token ...string) (map[string]any, error) {
	if len(token) > 0 {
		client.APIKey = token[0]
	}
	data, _, err := client.Request(ctx, "POST", path, body)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := value[key].(string); ok && text != "" {
			return text
		}
	}
	return ""
}

func serve(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := flags.String("listen", "", "HTTP listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	key, err := router.LoadKey()
	if err != nil {
		return err
	}
	return agent.ServeHTTP(key, *listen)
}

func randomID() (string, error) { return agent.RandomID() }

func httpRequest(ctx context.Context, method, endpoint string, body io.Reader, header, value string) ([]byte, error) {
	return agent.CaptchaRequest(ctx, method, endpoint, body, header, value)
}
