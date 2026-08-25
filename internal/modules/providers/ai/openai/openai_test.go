package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
)

type failingTransport struct{ err error }

func (t failingTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, t.err }

func TestChatTransportFailureDoesNotLeakRequestURLOrCredential(t *testing.T) {
	secret := "fixture-secret-transport-1234"
	client, err := New(Config{BaseURL: "https://provider.example.test/v1?private=query", APIKey: secret, Model: "m", Client: &http.Client{Transport: failingTransport{err: errors.New("Post https://provider.example.test/v1?private=query: Authorization: Bearer " + secret)}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Chat(context.Background(), providers.ChatRequest{Prompt: "x"})
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "provider.example.test") || strings.Contains(err.Error(), "Authorization") {
		t.Fatalf("unsafe error: %v", err)
	}
}

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
		http.Error(w, "upstream leaked Authorization: Bearer top-secret", http.StatusBadGateway)
	}))
	defer server.Close()
	client, err = New(Config{BaseURL: server.URL, APIKey: "k", Model: "m", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Chat(context.Background(), providers.ChatRequest{Prompt: "x"}); err == nil || !strings.Contains(err.Error(), "502") || strings.Contains(err.Error(), "top-secret") || strings.Contains(err.Error(), "Authorization") {
		t.Fatalf("Chat error = %v, want 502", err)
	}
}

func TestChatSendsJSONResponseFormatOnlyWhenRequested(t *testing.T) {
	requests := make(chan map[string]any, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests <- body
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]any{"content": "{}"}}}})
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, APIKey: "secret-key", Model: "qwen-json", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Chat(context.Background(), providers.ChatRequest{Prompt: "legacy"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Chat(context.Background(), providers.ChatRequest{Prompt: "structured", JSONMode: true}); err != nil {
		t.Fatal(err)
	}
	legacy, structured := <-requests, <-requests
	if _, exists := legacy["response_format"]; exists {
		t.Fatalf("legacy request changed globally: %#v", legacy)
	}
	format, ok := structured["response_format"].(map[string]any)
	if !ok || format["type"] != "json_object" {
		t.Fatalf("structured response_format = %#v", structured["response_format"])
	}
}

func TestMetadataExposesProviderAndModelWithoutSecrets(t *testing.T) {
	client, err := New(Config{BaseURL: "https://provider.example/v1", APIKey: "top-secret", Model: "qwen-meta"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := client.Metadata()
	encoded, _ := json.Marshal(metadata)
	if metadata.Provider == "" || metadata.Model != "qwen-meta" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if strings.Contains(string(encoded), "top-secret") || strings.Contains(string(encoded), "provider.example") {
		t.Fatalf("metadata leaks secret configuration: %s", encoded)
	}
}
