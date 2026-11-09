package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPPageAndProtectedEndpoint(t *testing.T) {
	handler := NewHTTPHandler("test-router-key")
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Vimox Router Agent") || !strings.Contains(page.Body.String(), "简体中文") {
		t.Fatalf("unexpected MCP page response: status=%d", page.Code)
	}
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated MCP request returned %d", unauthorized.Code)
	}
	authorized := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	authorized.Header.Set("Authorization", "Bearer test-router-key")
	authorized.Header.Set("Content-Type", "application/json")
	authorized.Header.Set("Accept", "application/json, text/event-stream")
	initialized := httptest.NewRecorder()
	handler.ServeHTTP(initialized, authorized)
	if initialized.Code != http.StatusOK {
		t.Fatalf("authenticated MCP initialize returned %d: %s", initialized.Code, initialized.Body.String())
	}
}

func TestMCPRejectsUnconfirmedPaidGeneration(t *testing.T) {
	handler := NewHTTPHandler("test-router-key")
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"router_image_create","arguments":{"model":"image-model","prompt":"draft","confirmed":false}}}`))
	req.Header.Set("Authorization", "Bearer test-router-key")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("tool call returned HTTP %d: %s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	result, _ := payload["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("unconfirmed generation was not rejected: %s", response.Body.String())
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 || !strings.Contains(content[0].(map[string]any)["text"].(string), "confirmed=true") {
		t.Fatalf("missing confirmation diagnostic: %s", response.Body.String())
	}
}

func TestMCPRejectsUnconfiguredBrowserOrigins(t *testing.T) {
	handler := NewHTTPHandler("test-router-key")
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://unexpected.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unexpected origin returned %d", response.Code)
	}
}
