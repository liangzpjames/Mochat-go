package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
)

func TestChatSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "qwen-test" {
			t.Fatalf("model = %v", body["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "分析结果"}}},
		})
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, APIKey: "test-key", Model: "qwen-test", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	text, err := client.Chat(context.Background(), providers.ChatRequest{System: "sys", Prompt: "分析"})
	if err != nil {
		t.Fatal(err)
	}
	if text != "分析结果" {
		t.Fatalf("text = %q", text)
	}
}

func TestChatNotConfiguredAndServerError(t *testing.T) {
	client, err := New(Config{APIKey: "", Model: "qwen-plus"})
	if err != nil {
		t.Fatal(err)
	}
	if status := client.Status(); status.State != providers.StateLimited {
		t.Fatalf("status = %#v, want limited", status)
	}
	if _, err := client.Chat(context.Background(), providers.ChatRequest{Prompt: "x"}); err == nil {
		t.Fatal("Chat error = nil, want ErrNotConfigured")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()
	client, err = New(Config{BaseURL: server.URL, APIKey: "k", Model: "m", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Chat(context.Background(), providers.ChatRequest{Prompt: "x"}); err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("Chat error = %v, want 502", err)
	}
}
