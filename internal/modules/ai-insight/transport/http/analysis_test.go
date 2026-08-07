package http

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
)

type fakeAIProvider struct {
	text string
	err  error
}

func (f fakeAIProvider) Chat(_ context.Context, _ providers.ChatRequest) (string, error) {
	return f.text, f.err
}

func (f fakeAIProvider) Status() providers.Status {
	if f.err != nil {
		return providers.Status{State: providers.StateLimited}
	}
	return providers.Status{State: providers.StateReady}
}

func TestRunAnalysisBuildsPageSpecificPrompt(t *testing.T) {
	handler := NewInsightHandlerWithProvider(insightResolver{principal: Principal{UserID: 1, TenantID: 1}}, nil, nil, fakeAIProvider{text: "摘要：客户咨询套餐"})
	text, err := handler.runAnalysis(context.Background(), "session-analysis", []string{"客户问价格"})
	if err != nil {
		t.Fatal(err)
	}
	if text != "摘要：客户咨询套餐" {
		t.Fatalf("text = %q", text)
	}
}

func TestReadyPageFromPayload(t *testing.T) {
	config := pageCatalog["emotion"]
	payload, err := json.Marshal(map[string]any{
		"summary":     "情绪负面",
		"keywords":    []string{"投诉"},
		"generatedAt": "2026-08-07T10:00:00+08:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	ready := readyPageFromPayload(config, string(payload), time.Now())
	if ready.Capability != "ready" || ready.Provider != "dashscope" {
		t.Fatalf("ready = %#v", ready)
	}
	if len(ready.Data) != 1 {
		t.Fatalf("data = %#v", ready.Data)
	}
	row := ready.Data[0].(map[string]any)
	if row["summary"] != "情绪负面" {
		t.Fatalf("row = %#v", row)
	}
}

func TestResolvePageKeepsLimitedWithoutProvider(t *testing.T) {
	handler := NewInsightHandler(insightResolver{principal: Principal{UserID: 1, TenantID: 1}}, nil)
	req := insightRequest(handler, "emotion")
	if req.Code != 200 {
		t.Fatalf("code = %d", req.Code)
	}
	if !strings.Contains(req.Body.String(), `"capability":"limited"`) {
		t.Fatalf("body = %s", req.Body.String())
	}
}
