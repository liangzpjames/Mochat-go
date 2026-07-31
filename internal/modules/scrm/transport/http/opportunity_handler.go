package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

const OpportunitiesPath = "/dashboard/scrm/opportunities"
const TagsPath = "/dashboard/scrm/tags"

type OpportunityService interface {
	ListOpportunities(context.Context, ports.OpportunityFilter) ([]ports.Opportunity, error)
	CreateOpportunity(context.Context, ports.CreateOpportunityCommand) (ports.Opportunity, error)
	ChangeOpportunityStage(context.Context, ports.ChangeOpportunityStageCommand) (ports.Opportunity, error)
	ListFollowUps(context.Context, int64, int64, string) ([]ports.FollowUpRecord, error)
	AppendFollowUp(context.Context, ports.AppendFollowUpCommand) (ports.FollowUpRecord, error)
	ListTags(context.Context, int64, int64) ([]ports.Tag, error)
	CreateTag(context.Context, int64, int64, string, string) (ports.Tag, error)
	RenameTag(context.Context, int64, int64, string, string, int64, string) (ports.Tag, error)
	BindTags(context.Context, int64, int64, string, []string, string) error
}

type OpportunityHandler struct {
	service   OpportunityService
	principal PrincipalResolver
}

func NewOpportunityHandler(service OpportunityService, principal PrincipalResolver) *OpportunityHandler {
	return &OpportunityHandler{service: service, principal: principal}
}

func (h *OpportunityHandler) List(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	items, err := h.service.ListOpportunities(r.Context(), ports.OpportunityFilter{TenantID: p.TenantID, CorpID: queryInt(r, "corpId"), Stage: r.URL.Query().Get("stage")})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"items": items}})
}
func (h *OpportunityHandler) Create(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	var q ports.CreateOpportunityCommand
	if decodeRequestJSON(w, r, &q) != nil {
		return
	}
	q.TenantID, q.CorpID = p.TenantID, queryInt(r, "corpId")
	q.IdempotencyKey = r.Header.Get("Idempotency-Key")
	item, err := h.service.CreateOpportunity(r.Context(), q)
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": item})
}
func (h *OpportunityHandler) Stage(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	var q ports.ChangeOpportunityStageCommand
	if decodeRequestJSON(w, r, &q) != nil {
		return
	}
	q.TenantID, q.CorpID = p.TenantID, queryInt(r, "corpId")
	q.OpportunityID = strings.TrimPrefix(r.URL.Path, OpportunitiesPath+"/")
	q.OpportunityID = strings.TrimSuffix(q.OpportunityID, "/stage")
	q.IdempotencyKey = r.Header.Get("Idempotency-Key")
	item, err := h.service.ChangeOpportunityStage(r.Context(), q)
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": item})
}
func (h *OpportunityHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	items, err := h.service.ListTags(r.Context(), p.TenantID, queryInt(r, "corpId"))
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"items": items}})
}
func (h *OpportunityHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	var q struct {
		Name string `json:"name"`
	}
	if decodeRequestJSON(w, r, &q) != nil {
		return
	}
	item, err := h.service.CreateTag(r.Context(), p.TenantID, queryInt(r, "corpId"), q.Name, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": item})
}

func queryInt(r *http.Request, key string) int64 {
	var n int64
	_, _ = fmt.Sscan(r.URL.Query().Get(key), &n)
	return n
}
func writeSCRMError(w http.ResponseWriter, err error) {
	if errors.Is(err, application.ErrInvalidArgument) {
		writeError(w, 422, "invalid request")
		return
	}
	writeError(w, 503, "service unavailable")
}
