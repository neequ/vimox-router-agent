package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultBaseURL = "https://router.vimox.cn"

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func New(apiKey string) *Client {
	base := strings.TrimRight(os.Getenv("VIMOX_ROUTER_BASE_URL"), "/")
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{BaseURL: base, APIKey: apiKey, HTTP: &http.Client{Timeout: 10 * time.Minute}}
}

func (c *Client) Request(ctx context.Context, method, path string, body any) (json.RawMessage, int, error) {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, payload)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	response, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return nil, response.StatusCode, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, response.StatusCode, fmt.Errorf("Router returned HTTP %d: %s", response.StatusCode, errorMessage(data))
	}
	return json.RawMessage(data), response.StatusCode, nil
}

func errorMessage(data []byte) string {
	var payload map[string]any
	if json.Unmarshal(data, &payload) == nil {
		if message := nestedMessage(payload); message != "" {
			return message
		}
	}
	return strings.TrimSpace(string(data))
}

func nestedMessage(value map[string]any) string {
	for _, key := range []string{"message", "error", "detail"} {
		switch item := value[key].(type) {
		case string:
			if strings.TrimSpace(item) != "" {
				return strings.TrimSpace(item)
			}
		case map[string]any:
			if message := nestedMessage(item); message != "" {
				return message
			}
		}
	}
	return ""
}

func ConfigPath() string {
	if value := os.Getenv("VIMOX_ROUTER_CONFIG"); value != "" {
		return value
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", ".vimox-router-agent.json")
	}
	return filepath.Join(root, "vimox-router-agent", "config.json")
}

type credentials struct {
	APIKey string `json:"api_key"`
}

func LoadKey() (string, error) {
	if key := strings.TrimSpace(os.Getenv("VIMOX_ROUTER_API_KEY")); key != "" {
		return key, nil
	}
	data, err := os.ReadFile(ConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var value credentials
	if err := json.Unmarshal(data, &value); err != nil {
		return "", fmt.Errorf("read CLI config: %w", err)
	}
	return strings.TrimSpace(value.APIKey), nil
}

func SaveKey(key string) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(credentials{APIKey: key}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func ClearKey() error {
	err := os.Remove(ConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
