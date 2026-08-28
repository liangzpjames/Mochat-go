package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/observability"
)

// Config configures an OpenAI-compatible chat provider.
type Config struct {
	BaseURL  string
	APIKey   string
	Model    string
	Provider string
	Timeout  time.Duration
	Client   *http.Client
	Logger   *slog.Logger
}

// Client implements providers.AIProvider over /chat/completions.
type Client struct {
	baseURL  string
	apiKey   string
	model    string
	provider string
	timeout  time.Duration
	client   *http.Client
	logger   *slog.Logger
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
	provider := strings.TrimSpace(config.Provider)
	if provider == "" {
		provider = "openai-compatible"
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{baseURL: baseURL, apiKey: strings.TrimSpace(config.APIKey), model: model, provider: provider, timeout: timeout, client: client, logger: logger}, nil
}

func (c *Client) Status() providers.Status {
	if c.apiKey == "" {
		return providers.Status{Kind: "ai", State: providers.StateLimited, Reason: "AI provider credential unavailable"}
	}
	return providers.Status{Kind: "ai", State: providers.StateReady}
}

func (c *Client) Metadata() providers.AIProviderMetadata {
	return providers.AIProviderMetadata{Provider: c.provider, Model: c.model}
}

func (c *Client) Chat(ctx context.Context, req providers.ChatRequest) (string, error) {
	startedAt := time.Now()
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.model
	}
	if c.apiKey == "" {
		c.logChatFailure(ctx, slog.LevelWarn, model, "AI_PROVIDER_NOT_CONFIGURED", 0, startedAt)
		return "", providers.ErrNotConfigured
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
		c.logChatFailure(ctx, slog.LevelError, model, "AI_PROVIDER_REQUEST_INVALID", 0, startedAt)
		return "", fmt.Errorf("marshal chat request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		c.logChatFailure(ctx, slog.LevelError, model, "AI_PROVIDER_REQUEST_INVALID", 0, startedAt)
		return "", errors.New("AI provider request is invalid")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	response, err := c.client.Do(request)
	if err != nil {
		errorCode := "AI_PROVIDER_NETWORK_FAILED"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			errorCode = "AI_PROVIDER_TIMEOUT"
		}
		c.logChatFailure(ctx, slog.LevelWarn, model, errorCode, 0, startedAt)
		return "", errors.New("AI provider request failed")
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		c.logChatFailure(ctx, slog.LevelError, model, "AI_PROVIDER_RESPONSE_READ_FAILED", response.StatusCode, startedAt)
		return "", errors.New("AI provider response could not be read")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		level := slog.LevelWarn
		errorCode := "AI_PROVIDER_HTTP_4XX"
		if response.StatusCode >= http.StatusInternalServerError {
			level = slog.LevelError
			errorCode = "AI_PROVIDER_HTTP_5XX"
		}
		c.logChatFailure(ctx, level, model, errorCode, response.StatusCode, startedAt)
		return "", fmt.Errorf("AI provider returned HTTP %d", response.StatusCode)
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &completion); err != nil {
		c.logChatFailure(ctx, slog.LevelError, model, "AI_PROVIDER_RESPONSE_INVALID", response.StatusCode, startedAt)
		return "", errors.New("AI provider response is invalid")
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		c.logChatFailure(ctx, slog.LevelError, model, "AI_PROVIDER_RESPONSE_EMPTY", response.StatusCode, startedAt)
		return "", errors.New("AI provider returned no content")
	}
	c.logger.InfoContext(ctx, "AI Provider 调用完成", c.chatLogAttrs(ctx, model, "success", "", response.StatusCode, startedAt)...)
	return completion.Choices[0].Message.Content, nil
}

func (c *Client) logChatFailure(ctx context.Context, level slog.Level, model string, errorCode string, httpStatus int, startedAt time.Time) {
	c.logger.Log(ctx, level, "AI Provider 调用失败；请检查 Provider 配置、配额、网络和响应格式",
		c.chatLogAttrs(ctx, model, "failed", errorCode, httpStatus, startedAt)...)
}

func (c *Client) chatLogAttrs(ctx context.Context, model string, result string, errorCode string, statusCode int, startedAt time.Time) []any {
	event := "ai_provider_call_completed"
	if errorCode != "" {
		event = "ai_provider_call_failed"
	}
	attrs := []any{
		"event", event, "component", "ai_provider", "provider", c.provider, "model", model,
		"step", "chat", "result", result, "status_code", statusCode, "duration_ms", time.Since(startedAt).Milliseconds(),
	}
	if requestID := observability.RequestID(ctx); requestID != "" {
		attrs = append(attrs, "request_id", requestID)
	}
	if errorCode != "" {
		attrs = append(attrs, "error_code", errorCode)
	}
	return attrs
}
