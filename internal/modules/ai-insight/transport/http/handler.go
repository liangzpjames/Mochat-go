package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
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
}

func NewInsightHandler(p PrincipalResolver, a Authorizer) *InsightHandler {
	return &InsightHandler{principal: p, authorize: a}
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
	writeEnvelope(w, http.StatusOK, "success", config)
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
