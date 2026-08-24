package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
)

// Config configures an OpenAI-compatible chat provider.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
	Client  *http.Client
}

// Client implements providers.AIProvider over /chat/completions.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	timeout time.Duration
	client  *http.Client
}

var _ providers.AIProvider = (*Client)(nil)

const defaultBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

func New(config Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = "qwen-plus"
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	} else {
		client.Timeout = timeout
	}
	return &Client{baseURL: baseURL, apiKey: strings.TrimSpace(config.APIKey), model: model, timeout: timeout, client: client}, nil
}

func (c *Client) Status() providers.Status {
	if c.apiKey == "" {
		return providers.Status{Kind: "ai", State: providers.StateLimited, Reason: "MOCHAT_GO_AI_PROVIDER_KEY 未配置"}
	}
	return providers.Status{Kind: "ai", State: providers.StateReady}
}

func (c *Client) Metadata() providers.AIProviderMetadata {
	return providers.AIProviderMetadata{Provider: "openai-compatible", Model: c.model}
}

func (c *Client) Chat(ctx context.Context, req providers.ChatRequest) (string, error) {
	if c.apiKey == "" {
		return "", providers.ErrNotConfigured
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.model
	}
	messages := make([]map[string]string, 0, 2)
	if system := strings.TrimSpace(req.System); system != "" {
		messages = append(messages, map[string]string{"role": "system", "content": system})
	}
	messages = append(messages, map[string]string{"role": "user", "content": req.Prompt})
	requestPayload := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": 0.3,
	}
	if req.JSONMode {
		requestPayload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, err := json.Marshal(requestPayload)
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build chat request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	response, err := c.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("call AI provider: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("read AI provider response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("AI provider returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(payload)))
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &completion); err != nil {
		return "", fmt.Errorf("parse AI provider response: %w", err)
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return "", errors.New("AI provider returned no content")
	}
	return completion.Choices[0].Message.Content, nil
}
