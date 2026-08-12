package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

var (
	ErrPrincipalUnauthorized = errors.New("principal unauthorized")
	ErrForbidden             = errors.New("forbidden")
)

type Principal struct {
	UserID   int64
	TenantID int64
	CorpID   int64
}

type PrincipalResolver interface {
	Resolve(*http.Request) (Principal, error)
}

type Authorizer interface {
	Authorize(context.Context, Principal, int64, string) error
}

func writeEnvelope(w http.ResponseWriter, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": msg, "data": data})
}

func resolvePrincipal(w http.ResponseWriter, r *http.Request, resolver PrincipalResolver) (Principal, bool) {
	p, err := resolver.Resolve(r)
	if err != nil || p.UserID <= 0 || p.TenantID <= 0 || p.CorpID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, "principal unauthorized", nil)
		return Principal{}, false
	}
	return p, true
}

func pathID(r *http.Request) string {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

type KnowledgeBaseHandler struct {
	repo      ports.KnowledgeBaseRepository
	principal PrincipalResolver
	authorize Authorizer
	generate  func() string
}

func NewKnowledgeBaseHandler(repo ports.KnowledgeBaseRepository, p PrincipalResolver, a Authorizer, generate func() string) *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{repo: repo, principal: p, authorize: a, generate: generate}
}

func (h *KnowledgeBaseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := resolvePrincipal(w, r, h.principal)
	if !ok {
		return
	}
	var body map[string]any
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			writeEnvelope(w, http.StatusBadRequest, "invalid json", nil)
			return
		}
	}
	corp := p.CorpID
	permission := "/ai-settings/knowledge-base#get"
	if r.Method != http.MethodGet {
		permission = "/ai-settings/knowledge-base@edit#put"
	}
	if h.authorize != nil {
		if err := h.authorize.Authorize(r.Context(), p, corp, permission); err != nil {
			writeEnvelope(w, http.StatusForbidden, ErrForbidden.Error(), nil)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.repo.List(r.Context(), p.TenantID, corp)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", items)
	case http.MethodPost:
		kb := ports.KnowledgeBase{
			ID: h.generate(), TenantID: p.TenantID, CorpID: corp,
			Name: str(head(body, "name")), Description: str(head(body, "description")),
			DocumentCount: int(num(body["documentCount"])), Status: intStatus(body, "status"),
			CreatedBy: p.UserID, UpdatedBy: p.UserID,
		}
		if strings.TrimSpace(kb.Name) == "" {
			writeEnvelope(w, http.StatusBadRequest, "name is required", nil)
			return
		}
		created, err := h.repo.Create(r.Context(), kb)
		if err != nil {
			writeEnvelope(w, http.StatusConflict, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", created)
	case http.MethodPut:
		id := pathID(r)
		if id == "" || id == "knowledge-bases" {
			writeEnvelope(w, http.StatusBadRequest, "id required", nil)
			return
		}
		kb := ports.KnowledgeBase{
			ID: id, TenantID: p.TenantID, CorpID: corp,
			Name: str(head(body, "name")), Description: str(head(body, "description")),
			DocumentCount: int(num(body["documentCount"])), Status: intStatus(body, "status"),
			UpdatedBy: p.UserID,
		}
		if strings.TrimSpace(kb.Name) == "" {
			writeEnvelope(w, http.StatusBadRequest, "name is required", nil)
			return
		}
		updated, err := h.repo.Update(r.Context(), kb)
		if err != nil {
			writeEnvelope(w, http.StatusConflict, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", updated)
	case http.MethodDelete:
		id := pathID(r)
		if id == "" || id == "knowledge-bases" {
			writeEnvelope(w, http.StatusBadRequest, "id required", nil)
			return
		}
		if err := h.repo.Delete(r.Context(), p.TenantID, corp, id); err != nil {
			writeEnvelope(w, http.StatusConflict, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", map[string]any{"id": id})
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

type AgentHandler struct {
	repo      ports.AgentRepository
	principal PrincipalResolver
	authorize Authorizer
	generate  func() string
}

func NewAgentHandler(repo ports.AgentRepository, p PrincipalResolver, a Authorizer, generate func() string) *AgentHandler {
	return &AgentHandler{repo: repo, principal: p, authorize: a, generate: generate}
}

func (h *AgentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := resolvePrincipal(w, r, h.principal)
	if !ok {
		return
	}
	var body map[string]any
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			writeEnvelope(w, http.StatusBadRequest, "invalid json", nil)
			return
		}
	}
	corp := p.CorpID
	permission := "/ai-settings/agent#get"
	if r.Method != http.MethodGet {
		permission = "/ai-settings/agent@edit#put"
	}
	if h.authorize != nil {
		if err := h.authorize.Authorize(r.Context(), p, corp, permission); err != nil {
			writeEnvelope(w, http.StatusForbidden, ErrForbidden.Error(), nil)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.repo.List(r.Context(), p.TenantID, corp)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", items)
	case http.MethodPost:
		agent := ports.Agent{
			ID: h.generate(), TenantID: p.TenantID, CorpID: corp,
			Name: str(head(body, "name")), Description: str(head(body, "description")),
			KnowledgeBaseIDs: strSlice(body["knowledgeBaseIds"]), Status: intStatus(body, "status"),
			CreatedBy: p.UserID, UpdatedBy: p.UserID,
		}
		if strings.TrimSpace(agent.Name) == "" {
			writeEnvelope(w, http.StatusBadRequest, "name is required", nil)
			return
		}
		created, err := h.repo.Create(r.Context(), agent)
		if err != nil {
			writeEnvelope(w, http.StatusConflict, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", created)
	case http.MethodPut:
		id := pathID(r)
		if id == "" || id == "agents" {
			writeEnvelope(w, http.StatusBadRequest, "id required", nil)
			return
		}
		agent := ports.Agent{
			ID: id, TenantID: p.TenantID, CorpID: corp,
			Name: str(head(body, "name")), Description: str(head(body, "description")),
			KnowledgeBaseIDs: strSlice(body["knowledgeBaseIds"]), Status: intStatus(body, "status"),
			UpdatedBy: p.UserID,
		}
		if strings.TrimSpace(agent.Name) == "" {
			writeEnvelope(w, http.StatusBadRequest, "name is required", nil)
			return
		}
		updated, err := h.repo.Update(r.Context(), agent)
		if err != nil {
			writeEnvelope(w, http.StatusConflict, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", updated)
	case http.MethodDelete:
		id := pathID(r)
		if id == "" || id == "agents" {
			writeEnvelope(w, http.StatusBadRequest, "id required", nil)
			return
		}
		if err := h.repo.Delete(r.Context(), p.TenantID, corp, id); err != nil {
			writeEnvelope(w, http.StatusConflict, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", map[string]any{"id": id})
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func head(m map[string]any, key string) any {
	return m[key]
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func num(v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return 0
}

func intStatus(m map[string]any, key string) int {
	n := num(m[key])
	if n <= 0 {
		return 1
	}
	return int(n)
}

func strSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			result = append(result, s)
		}
	}
	return result
}
