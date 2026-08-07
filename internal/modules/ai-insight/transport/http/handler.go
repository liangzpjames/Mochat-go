package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
)

var (
	ErrPrincipalUnauthorized = errors.New("principal unauthorized")
	ErrForbidden             = errors.New("forbidden")
)

type Principal struct {
	UserID   int64
	TenantID int64
}

type PrincipalResolver interface {
	Resolve(*http.Request) (Principal, error)
}

type Authorizer interface {
	Authorize(context.Context, Principal, int64, string) error
}

type InsightPage struct {
	Page        string   `json:"page"`
	Title       string   `json:"title"`
	Capability  string   `json:"capability"`
	Provider    string   `json:"provider"`
	Limitations []string `json:"limitations"`
	Data        []any    `json:"data"`
	GeneratedAt string   `json:"generatedAt,omitempty"`
}

var pageCatalog = map[string]InsightPage{
	"session-analysis": {
		Page: "session-analysis", Title: "会话分析", Capability: "limited", Provider: "none",
		Limitations: []string{
			"未接入会话存档 Provider，无法生成会话量、时长、关键词等分析结果",
			"接入企业微信会话存档后，本页将自动读取归档消息生成分析",
		},
		Data: []any{},
	},
	"smart-analysis": {
		Page: "smart-analysis", Title: "智能分析", Capability: "limited", Provider: "none",
		Limitations: []string{
			"未接入 AI 模型服务，智能分析能力不可用",
			"配置 AI Provider 后，本页将提供对话摘要、意图识别与建议",
		},
		Data: []any{},
	},
	"emotion": {
		Page: "emotion", Title: "情绪识别", Capability: "limited", Provider: "none",
		Limitations: []string{
			"未接入情绪识别模型服务，情绪分析结果不可用",
			"配置 AI Provider 后，本页将按会话展示情绪倾向与异常波动",
		},
		Data: []any{},
	},
	"employee-score": {
		Page: "employee-score", Title: "员工评分", Capability: "limited", Provider: "none",
		Limitations: []string{
			"未接入评分模型与会话存档 Provider，员工服务评分不可用",
			"接入后本页将按员工展示响应时效、沟通质量与得分",
		},
		Data: []any{},
	},
	"communication-keyword": {
		Page: "communication-keyword", Title: "沟通关键词", Capability: "limited", Provider: "none",
		Limitations: []string{
			"未接入会话存档 Provider，沟通关键词统计不可用",
			"接入归档后本页将展示高频词、敏感词命中与趋势",
		},
		Data: []any{},
	},
}

type InsightHandler struct {
	principal PrincipalResolver
	authorize Authorizer
	db        *sql.DB
	ai        providers.AIProvider
	analysis  AnalysisStore
}

func NewInsightHandler(p PrincipalResolver, a Authorizer) *InsightHandler {
	return &InsightHandler{principal: p, authorize: a}
}

func NewInsightHandlerWithProvider(p PrincipalResolver, a Authorizer, db *sql.DB, ai providers.AIProvider) *InsightHandler {
	handler := NewInsightHandler(p, a)
	handler.db = db
	handler.ai = ai
	if db != nil {
		handler.analysis = NewSQLAnalysisStore(db)
	}
	return handler
}

func (h *InsightHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeEnvelope(w, http.StatusUnauthorized, ErrPrincipalUnauthorized.Error(), nil)
		return
	}
	corp, _ := strconv.ParseInt(r.URL.Query().Get("corpId"), 10, 64)
	if corp <= 0 {
		writeEnvelope(w, http.StatusBadRequest, "corpId required", nil)
		return
	}
	page := pathPage(r.URL.Path)
	config, ok := pageCatalog[page]
	if !ok {
		writeEnvelope(w, http.StatusNotFound, "page not found", nil)
		return
	}
	if h.authorize != nil {
		if err := h.authorize.Authorize(r.Context(), p, corp, "/ai-insight/"+page+"#get"); err != nil {
			writeEnvelope(w, http.StatusForbidden, ErrForbidden.Error(), nil)
			return
		}
	}
	config = h.resolvePage(r, config, corp)
	writeEnvelope(w, http.StatusOK, "success", config)
}

func (h *InsightHandler) resolvePage(r *http.Request, config InsightPage, corp int64) InsightPage {
	if h.db == nil || h.ai == nil || h.ai.Status().State != providers.StateReady {
		return config
	}
	texts, err := FetchArchiveTexts(r.Context(), h.db, corp, 20)
	if err != nil {
		config.Capability = "limited"
		config.Limitations = []string{"读取归档会话数据失败：" + err.Error()}
		return config
	}
	if len(texts) == 0 {
		config.Capability = "limited"
		config.Limitations = []string{"暂无归档会话数据（未接入会话存档 Provider 或当前企业没有已归档消息）"}
		return config
	}
	refresh := r.URL.Query().Get("refresh") == "1"
	if !refresh && h.analysis != nil {
		if row, err := h.analysis.Latest(r.Context(), corp, config.Page); err == nil && row != nil && time.Since(row.CreatedAt) < 5*time.Minute {
			return readyPageFromPayload(config, row.Payload, row.CreatedAt)
		}
	}
	result, err := h.runAnalysis(r.Context(), config.Page, texts)
	if err != nil {
		config.Capability = "limited"
		config.Limitations = []string{"AI 分析失败：" + err.Error()}
		return config
	}
	now := time.Now()
	payload := map[string]any{"summary": result, "keywords": []any{}, "generatedAt": now.Format(time.RFC3339)}
	if h.analysis != nil {
		if err := h.analysis.Save(r.Context(), corp, config.Page, "succeeded", payload, ""); err != nil {
			config.Capability = "limited"
			config.Limitations = []string{"分析结果落库失败：" + err.Error()}
			return config
		}
	}
	config.Capability = "ready"
	config.Provider = "dashscope"
	config.Limitations = []string{}
	config.GeneratedAt = now.Format(time.RFC3339)
	config.Data = []any{map[string]any{
		"sessionId":   "archive",
		"summary":     result,
		"keywords":    []any{},
		"generatedAt": now.Format(time.RFC3339),
	}}
	return config
}

func readyPageFromPayload(config InsightPage, payload string, createdAt time.Time) InsightPage {
	var stored map[string]any
	if json.Unmarshal([]byte(payload), &stored) == nil {
		summary, _ := stored["summary"].(string)
		keywords, _ := stored["keywords"].([]any)
		generatedAt, _ := stored["generatedAt"].(string)
		config.Capability = "ready"
		config.Provider = "dashscope"
		config.Limitations = []string{}
		config.GeneratedAt = generatedAt
		config.Data = []any{map[string]any{
			"sessionId":   "archive",
			"summary":     summary,
			"keywords":    keywords,
			"generatedAt": generatedAt,
		}}
		_ = createdAt
	}
	return config
}

func (h *InsightHandler) runAnalysis(ctx context.Context, page string, texts []string) (string, error) {
	prompt := fmt.Sprintf("以下是企业微信会话归档文本（共 %d 条）：\n", len(texts))
	for index, text := range texts {
		prompt += fmt.Sprintf("%d. %s\n", index+1, text)
	}
	system := "你是企业微信会话分析助手，只根据提供的归档文本做客观分析，不要编造不存在的事实。"
	var instruction string
	switch page {
	case "session-analysis":
		instruction = "请生成会话分析摘要，并列出 3-5 个沟通关键词，用中文回答。"
	case "smart-analysis":
		instruction = "请识别客户意图，并给出 2-3 条跟进建议，用中文回答。"
	case "emotion":
		instruction = "请判断客户情绪倾向（正面/中性/负面）与异常波动，用中文回答。"
	case "employee-score":
		instruction = "请评估员工服务表现（响应时效、沟通质量），给出百分制得分与理由，用中文回答。"
	case "communication-keyword":
		instruction = "请统计高频词、敏感词命中与沟通趋势，用中文回答。"
	default:
		instruction = "请分析以上会话内容，用中文回答。"
	}
	return h.ai.Chat(ctx, providers.ChatRequest{System: system, Prompt: prompt + instruction})
}

func pathPage(path string) string {
	trimmed := path
	for len(trimmed) > 0 && trimmed[0] == '/' {
		trimmed = trimmed[1:]
	}
	slash := -1
	for i := len(trimmed) - 1; i >= 0; i-- {
		if trimmed[i] == '/' {
			slash = i
			break
		}
	}
	if slash >= 0 {
		return trimmed[slash+1:]
	}
	return trimmed
}

func writeEnvelope(w http.ResponseWriter, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": msg, "data": data})
}
