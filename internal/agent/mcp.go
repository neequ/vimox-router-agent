package agent

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/neequ/vimox-router-agent/internal/router"
)

func newMCP(apiKey string) *server.MCPServer {
	s := server.NewMCPServer("vimox-router-agent", "0.1.0", server.WithToolCapabilities(false))
	client := router.New(apiKey)
	s.AddTool(mcp.NewTool("router_models_list", mcp.WithDescription("List public models available through Vimox Router.")), func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return call(ctx, client, "GET", "/v1/models", nil)
	})
	s.AddTool(mcp.NewTool("router_text_create",
		mcp.WithDescription("Create a text response through Vimox Router. This is a paid model request; only call after the user reviews and confirms the model and prompt."),
		mcp.WithString("model", mcp.Required(), mcp.Description("Public Router model ID")),
		mcp.WithString("prompt", mcp.Required(), mcp.Description("Complete user prompt")),
		mcp.WithBoolean("confirmed", mcp.Required(), mcp.Description("Must be true after explicit user approval"))), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if !req.GetBool("confirmed", false) {
			return mcp.NewToolResultError("Text generation requires confirmed=true after explicit user approval"), nil
		}
		model, err := req.RequireString("model")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prompt, err := req.RequireString("prompt")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body := map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": prompt}}}
		return call(ctx, client, "POST", "/v1/chat/completions", body)
	})
	s.AddTool(mcp.NewTool("router_image_create",
		mcp.WithDescription("Generate an image. This submits paid work; only call after the user reviews and confirms the model, prompt and parameters."),
		mcp.WithString("model", mcp.Required()), mcp.WithString("prompt", mcp.Required()),
		mcp.WithString("size", mcp.Description("Optional image size, such as 1024x1024")),
		mcp.WithBoolean("confirmed", mcp.Required(), mcp.Description("Must be true after explicit user approval"))), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		confirmed := req.GetBool("confirmed", false)
		if !confirmed {
			return mcp.NewToolResultError("Image generation requires confirmed=true after explicit user approval"), nil
		}
		model, err := req.RequireString("model")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prompt, err := req.RequireString("prompt")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body := map[string]any{"model": model, "prompt": prompt}
		if size := req.GetString("size", ""); size != "" {
			body["size"] = size
		}
		return call(ctx, client, "POST", "/v1/images/generations", body)
	})
	s.AddTool(mcp.NewTool("router_video_create",
		mcp.WithDescription("Generate a video. This submits paid work; only call after the user reviews and confirms the model, prompt and parameters."),
		mcp.WithString("model", mcp.Required()), mcp.WithString("prompt", mcp.Required()),
		mcp.WithString("seconds", mcp.Description("Optional video duration in seconds")),
		mcp.WithBoolean("confirmed", mcp.Required(), mcp.Description("Must be true after explicit user approval"))), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		confirmed := req.GetBool("confirmed", false)
		if !confirmed {
			return mcp.NewToolResultError("Video generation requires confirmed=true after explicit user approval"), nil
		}
		model, err := req.RequireString("model")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prompt, err := req.RequireString("prompt")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body := map[string]any{"model": model, "prompt": prompt}
		if seconds := req.GetString("seconds", ""); seconds != "" {
			body["seconds"] = seconds
		}
		return call(ctx, client, "POST", "/v1/videos", body)
	})
	s.AddTool(mcp.NewTool("router_task_get", mcp.WithDescription("Get the status and result of a Router video or asynchronous media task."), mcp.WithString("task_id", mcp.Required())), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("task_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return call(ctx, client, "GET", "/v1/videos/"+url.PathEscape(id), nil)
	})
	return s
}

func call(ctx context.Context, client *router.Client, method, path string, body any) (*mcp.CallToolResult, error) {
	data, _, err := client.Request(ctx, method, path, body)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func RunStdio(apiKey string) error { return server.ServeStdio(newMCP(apiKey)) }

func ServeHTTP(apiKey, listen string) error {
	if apiKey == "" {
		return fmt.Errorf("set VIMOX_ROUTER_API_KEY or sign in with the CLI before starting the MCP server")
	}
	if listen == "" {
		listen = strings.TrimSpace(os.Getenv("VIMOX_ROUTER_LISTEN"))
	}
	if listen == "" {
		listen = "127.0.0.1:8787"
	}
	server := &http.Server{Addr: listen, Handler: NewHTTPHandler(apiKey), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	fmt.Fprintf(os.Stderr, "Vimox Router MCP listening on %s\n", listen)
	return server.ListenAndServe()
}

func NewHTTPHandler(apiKey string) http.Handler {
	protocol := server.NewStreamableHTTPServer(newMCP(apiKey), server.WithStateLess(true))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/mcp", http.StatusTemporaryRedirect)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(landingPage))
	})
	mux.Handle("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte(landingPage))
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !allowedOrigin(origin) {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, MCP-Protocol-Version, MCP-Session-Id, Last-Event-ID")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorized(r, apiKey) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Router API key required", http.StatusUnauthorized)
			return
		}
		protocol.ServeHTTP(w, r)
	}))
	return mux
}

func allowedOrigin(origin string) bool {
	for _, allowed := range strings.Split(os.Getenv("VIMOX_ROUTER_CORS_ORIGINS"), ",") {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}

func authorized(r *http.Request, expected string) bool {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) < 8 || !strings.EqualFold(value[:7], "Bearer ") {
		return false
	}
	got := strings.TrimSpace(value[7:])
	if len(got) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func LandingPage() string { return landingPage }

func RandomID() (string, error) {
	data := make([]byte, 24)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func CaptchaRequest(ctx context.Context, method, endpoint string, body io.Reader, header, value string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(header, value)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Router returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

const landingPage = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Vimox Router Agent MCP</title><style>body{font:16px/1.6 system-ui,sans-serif;max-width:760px;margin:48px auto;padding:0 20px;color:#17201c}h1{font-size:30px;margin-bottom:4px}h2{font-size:19px;margin:28px 0 8px}p,li{color:#48534d}code,pre{background:#f0f3f1;border-radius:4px;padding:3px 6px;overflow-wrap:anywhere}pre{padding:14px}a{color:#126b45}nav{display:flex;gap:14px;margin-bottom:24px}button{padding:7px 10px;border:1px solid #b8c5bd;background:white;cursor:pointer}section[hidden]{display:none}</style><nav><button data-lang="en">English</button><button data-lang="zh">简体中文</button></nav><main><section data-copy="en"><h1>Vimox Router Agent</h1><p>Connect an MCP-compatible agent to Router's models and creative tools.</p><h2>Endpoint</h2><p><code>https://router.vimox.cn/mcp</code></p><h2>Connect</h2><ol><li>Sign in or create an account at <a href="https://router.vimox.cn/users/login">Router account</a>.</li><li>Create an API key in <a href="https://router.vimox.cn/users/connect">API key settings</a>.</li><li>Add the endpoint to your MCP client and configure the API key as a Bearer credential.</li></ol><h2>Tools</h2><p>Model discovery, text creation, image generation, video generation, and asynchronous task lookup. All generation tools require user confirmation.</p><pre>Authorization: Bearer &lt;Router API key&gt;</pre><p>CLI: <code>vimox-router-agent mcp</code> for stdio clients. Self-host the HTTP endpoint using <code>vimox-router-agent serve</code>.</p><p><a href="https://github.com/neequ/vimox-router-agent">Source code and releases</a></p></section><section data-copy="zh" hidden><h1>Vimox Router Agent</h1><p>将兼容 MCP 的 Agent 连接到 Router 模型与创作工具。</p><h2>接入地址</h2><p><code>https://router.vimox.cn/mcp</code></p><h2>连接步骤</h2><ol><li>在 <a href="https://router.vimox.cn/users/login">Router 账户页</a>注册或登录。</li><li>在 <a href="https://router.vimox.cn/users/connect">API Key 设置</a>创建密钥。</li><li>将接入地址添加到 MCP 客户端，并把 API Key 配置为 Bearer 凭证。</li></ol><h2>工具</h2><p>模型查询、文本创作、图片生成、视频生成与异步任务查询。所有创作工具均要求用户确认。</p><pre>Authorization: Bearer &lt;Router API Key&gt;</pre><p>CLI stdio 接入：<code>vimox-router-agent mcp</code>。自托管 HTTP 服务：<code>vimox-router-agent serve</code>。</p><p><a href="https://github.com/neequ/vimox-router-agent">源码与发行版</a></p></section></main><script>document.querySelectorAll('[data-lang]').forEach(b=>b.onclick=()=>{document.documentElement.lang=b.dataset.lang==='zh'?'zh-CN':'en-US';document.querySelectorAll('[data-copy]').forEach(s=>s.hidden=s.dataset.copy!==b.dataset.lang)})</script></html>`
