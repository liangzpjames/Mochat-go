package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

const (
	maxJSONBodyBytes = 1 << 20
	maxDocumentCount = 1<<31 - 1
)

const (
	machineCodeDescriptionInvalid      = "AI_SETTINGS_DESCRIPTION_INVALID"
	machineCodeDocumentCountInvalid    = "AI_SETTINGS_DOCUMENT_COUNT_INVALID"
	machineCodeForbidden               = "AI_SETTINGS_FORBIDDEN"
	machineCodeIDRequired              = "AI_SETTINGS_ID_REQUIRED"
	machineCodeInvalidJSON             = "AI_SETTINGS_INVALID_JSON"
	machineCodeInvalidStatus           = "AI_SETTINGS_STATUS_INVALID"
	machineCodeKnowledgeBaseInvalid    = "AI_SETTINGS_KNOWLEDGE_BASE_INVALID"
	machineCodeKnowledgeBaseReferenced = "AI_SETTINGS_KNOWLEDGE_BASE_REFERENCED"
	machineCodeNameRequired            = "AI_SETTINGS_NAME_REQUIRED"
	machineCodeNameInvalid             = "AI_SETTINGS_NAME_INVALID"
	machineCodePrincipalUnauthorized   = "AI_SETTINGS_PRINCIPAL_UNAUTHORIZED"
	machineCodeNotFound                = "AI_SETTINGS_NOT_FOUND"
	machineCodeStorageFailure          = "AI_SETTINGS_STORAGE_FAILURE"
)

var (
	ErrPrincipalUnauthorized      = errors.New("principal unauthorized")
	ErrForbidden                  = errors.New("forbidden")
	errCrossRepositoryUnavailable = errors.New("cross repository unavailable")
	errInvalidStatus              = errors.New("invalid status")
	errKnowledgeBaseInvalid       = errors.New("invalid knowledge base IDs")
	errKnowledgeBaseLookup        = errors.New("knowledge base lookup failed")
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

type knowledgeBaseInput struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DocumentCount int    `json:"documentCount"`
	Status        *int   `json:"status"`
}

type agentInput struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	KnowledgeBaseIDs []string `json:"knowledgeBaseIds"`
	Status           *int     `json:"status"`
}

func writeEnvelope(w http.ResponseWriter, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": msg, "data": data})
}

func resolvePrincipal(w http.ResponseWriter, r *http.Request, resolver PrincipalResolver) (Principal, bool) {
	p, err := resolver.Resolve(r)
	if err != nil || p.UserID <= 0 || p.TenantID <= 0 || p.CorpID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, machineCodePrincipalUnauthorized, nil)
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

func decodeJSON(w http.ResponseWriter, r *http.Request, body any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, machineCodeInvalidJSON, nil)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeEnvelope(w, http.StatusBadRequest, machineCodeInvalidJSON, nil)
		return false
	}
	return true
}

func parseStatus(value *int) (int, error) {
	if value == nil || (*value != 0 && *value != 1) {
		return 0, errInvalidStatus
	}
	return *value, nil
}

func validName(value string) bool {
	count := utf8.RuneCountInString(strings.TrimSpace(value))
	return count >= 2 && count <= 128
}

func validDescription(value string) bool {
	return utf8.RuneCountInString(value) <= 512
}

func authorize(w http.ResponseWriter, r *http.Request, authorizer Authorizer, principal Principal, permission string) bool {
	if authorizer == nil {
		return true
	}
	if err := authorizer.Authorize(r.Context(), principal, principal.CorpID, permission); err != nil {
		writeEnvelope(w, http.StatusForbidden, machineCodeForbidden, nil)
		return false
	}
	return true
}

func writeMutationError(w http.ResponseWriter, err error) {
	if errors.Is(err, ports.ErrNotFound) {
		writeEnvelope(w, http.StatusNotFound, machineCodeNotFound, nil)
		return
	}
	writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
}

type KnowledgeBaseHandler struct {
	repo      ports.KnowledgeBaseRepository
	agents    ports.AgentRepository
	principal PrincipalResolver
	authorize Authorizer
	generate  func() string
}

func NewKnowledgeBaseHandler(repo ports.KnowledgeBaseRepository, agents ports.AgentRepository, p PrincipalResolver, a Authorizer, generate func() string) *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{repo: repo, agents: agents, principal: p, authorize: a, generate: generate}
}

func (h *KnowledgeBaseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := resolvePrincipal(w, r, h.principal)
	if !ok {
		return
	}
	var input knowledgeBaseInput
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		if !decodeJSON(w, r, &input) {
			return
		}
	}
	permission := "/ai-settings/knowledge-base#get"
	if r.Method != http.MethodGet {
		permission = "/ai-settings/knowledge-base@edit#put"
	}
	if !authorize(w, r, h.authorize, p, permission) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.repo.List(r.Context(), p.TenantID, p.CorpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", items)
	case http.MethodPost, http.MethodPut:
		status, err := parseStatus(input.Status)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, machineCodeInvalidStatus, nil)
			return
		}
		if strings.TrimSpace(input.Name) == "" {
			writeEnvelope(w, http.StatusBadRequest, machineCodeNameRequired, nil)
			return
		}
		if !validName(input.Name) {
			writeEnvelope(w, http.StatusBadRequest, machineCodeNameInvalid, nil)
			return
		}
		if input.DocumentCount < 0 || input.DocumentCount > maxDocumentCount {
			writeEnvelope(w, http.StatusBadRequest, machineCodeDocumentCountInvalid, nil)
			return
		}
		if !validDescription(input.Description) {
			writeEnvelope(w, http.StatusBadRequest, machineCodeDescriptionInvalid, nil)
			return
		}
		knowledgeBase := ports.KnowledgeBase{
			TenantID: p.TenantID, CorpID: p.CorpID,
			Name: input.Name, Description: input.Description, DocumentCount: input.DocumentCount, Status: status,
			UpdatedBy: p.UserID,
		}
		if r.Method == http.MethodPost {
			knowledgeBase.ID = h.generate()
			knowledgeBase.CreatedBy = p.UserID
			created, err := h.repo.Create(r.Context(), knowledgeBase)
			if err != nil {
				writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
				return
			}
			writeEnvelope(w, http.StatusOK, "success", created)
			return
		}
		knowledgeBase.ID = pathID(r)
		if knowledgeBase.ID == "" || knowledgeBase.ID == "knowledge-bases" {
			writeEnvelope(w, http.StatusBadRequest, machineCodeIDRequired, nil)
			return
		}
		updated, err := h.repo.Update(r.Context(), knowledgeBase)
		if err != nil {
			writeMutationError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", updated)
	case http.MethodDelete:
		id := pathID(r)
		if id == "" || id == "knowledge-bases" {
			writeEnvelope(w, http.StatusBadRequest, machineCodeIDRequired, nil)
			return
		}
		if h.agents == nil {
			writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
			return
		}
		references, err := h.agents.ListReferencingKnowledgeBase(r.Context(), p.TenantID, p.CorpID, id)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
			return
		}
		if len(references) != 0 {
			writeEnvelope(w, http.StatusConflict, machineCodeKnowledgeBaseReferenced, nil)
			return
		}
		if err := h.repo.Delete(r.Context(), p.TenantID, p.CorpID, p.UserID, id); err != nil {
			writeMutationError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", map[string]any{"id": id})
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

type AgentHandler struct {
	repo           ports.AgentRepository
	knowledgeBases ports.KnowledgeBaseRepository
	principal      PrincipalResolver
	authorize      Authorizer
	generate       func() string
}

func NewAgentHandler(repo ports.AgentRepository, knowledgeBases ports.KnowledgeBaseRepository, p PrincipalResolver, a Authorizer, generate func() string) *AgentHandler {
	return &AgentHandler{repo: repo, knowledgeBases: knowledgeBases, principal: p, authorize: a, generate: generate}
}

func (h *AgentHandler) validateKnowledgeBases(ctx context.Context, principal Principal, ids []string) ([]string, error) {
	unique := make(map[string]struct{}, len(ids))
	normalized := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, errKnowledgeBaseInvalid
		}
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if len(normalized) == 0 {
		return normalized, nil
	}
	if h.knowledgeBases == nil {
		return nil, errCrossRepositoryUnavailable
	}
	found, err := h.knowledgeBases.GetByIDs(ctx, principal.TenantID, principal.CorpID, normalized)
	if err != nil {
		return nil, errKnowledgeBaseLookup
	}
	if len(found) != len(normalized) {
		return nil, errKnowledgeBaseInvalid
	}
	return normalized, nil
}

func (h *AgentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := resolvePrincipal(w, r, h.principal)
	if !ok {
		return
	}
	var input agentInput
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		if !decodeJSON(w, r, &input) {
			return
		}
	}
	permission := "/ai-settings/agent#get"
	if r.Method != http.MethodGet {
		permission = "/ai-settings/agent@edit#put"
	}
	if !authorize(w, r, h.authorize, p, permission) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.repo.List(r.Context(), p.TenantID, p.CorpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", items)
	case http.MethodPost, http.MethodPut:
		status, err := parseStatus(input.Status)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, machineCodeInvalidStatus, nil)
			return
		}
		if strings.TrimSpace(input.Name) == "" {
			writeEnvelope(w, http.StatusBadRequest, machineCodeNameRequired, nil)
			return
		}
		if !validName(input.Name) {
			writeEnvelope(w, http.StatusBadRequest, machineCodeNameInvalid, nil)
			return
		}
		if !validDescription(input.Description) {
			writeEnvelope(w, http.StatusBadRequest, machineCodeDescriptionInvalid, nil)
			return
		}
		knowledgeBaseIDs, err := h.validateKnowledgeBases(r.Context(), p, input.KnowledgeBaseIDs)
		if err != nil {
			if errors.Is(err, errCrossRepositoryUnavailable) || errors.Is(err, errKnowledgeBaseLookup) {
				writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
				return
			}
			writeEnvelope(w, http.StatusBadRequest, machineCodeKnowledgeBaseInvalid, nil)
			return
		}
		agent := ports.Agent{
			TenantID: p.TenantID, CorpID: p.CorpID,
			Name: input.Name, Description: input.Description, KnowledgeBaseIDs: knowledgeBaseIDs, Status: status,
			UpdatedBy: p.UserID,
		}
		if r.Method == http.MethodPost {
			agent.ID = h.generate()
			agent.CreatedBy = p.UserID
			created, err := h.repo.Create(r.Context(), agent)
			if err != nil {
				writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
				return
			}
			writeEnvelope(w, http.StatusOK, "success", created)
			return
		}
		agent.ID = pathID(r)
		if agent.ID == "" || agent.ID == "agents" {
			writeEnvelope(w, http.StatusBadRequest, machineCodeIDRequired, nil)
			return
		}
		updated, err := h.repo.Update(r.Context(), agent)
		if err != nil {
			writeMutationError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", updated)
	case http.MethodDelete:
		id := pathID(r)
		if id == "" || id == "agents" {
			writeEnvelope(w, http.StatusBadRequest, machineCodeIDRequired, nil)
			return
		}
		if err := h.repo.Delete(r.Context(), p.TenantID, p.CorpID, p.UserID, id); err != nil {
			writeMutationError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, "success", map[string]any{"id": id})
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}
