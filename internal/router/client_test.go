package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestUsesBearerKeyAndPreservesUpstreamErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":{"code":"unsupported_size","message":"requested size is unavailable"}}`))
	}))
	defer server.Close()
	client := New("sk-test")
	client.BaseURL = server.URL
	_, status, err := client.Request(context.Background(), http.MethodPost, "/v1/images/generations", map[string]string{"prompt": "test"})
	if status != http.StatusUnprocessableEntity || err == nil || !strings.Contains(err.Error(), "requested size is unavailable") {
		t.Fatalf("upstream error was not preserved: status=%d err=%v", status, err)
	}
}
