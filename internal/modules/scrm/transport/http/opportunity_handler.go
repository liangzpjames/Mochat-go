package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

const OpportunitiesPath = "/dashboard/scrm/opportunities"
const TagsPath = "/dashboard/scrm/tags"
const FollowUpsPath = "/dashboard/scrm/contacts/{contactId}/follow-ups"

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

type opportunityJSON struct {
	ID         string  `json:"id"`
	ContactID  string  `json:"contactId"`
	Stage      string  `json:"stage"`
	Status     string  `json:"status"`
	LostReason string  `json:"lostReason,omitempty"`
	OwnerID    int64   `json:"ownerId"`
	Amount     float64 `json:"amount"`
	StartDate  string  `json:"startDate"`
	EndDate    string  `json:"endDate"`
	Version    int64   `json:"version"`
}
type followUpJSON struct {
	ID        string `json:"id"`
	ContactID string `json:"contactId"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	CreatedBy int64  `json:"createdBy"`
}
type tagJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int64  `json:"version"`
}

func opportunityView(item ports.Opportunity) opportunityJSON {
	return opportunityJSON{ID: item.ID, ContactID: item.ContactID, Stage: item.Stage, Status: item.Status, LostReason: item.LostReason, OwnerID: item.OwnerID, Amount: item.Amount, StartDate: item.StartDate.Format("2006-01-02"), EndDate: item.EndDate.Format("2006-01-02"), Version: item.Version}
}
func followUpView(item ports.FollowUpRecord) followUpJSON {
	return followUpJSON{ID: item.ID, ContactID: item.ContactID, Content: item.Content, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), CreatedBy: item.CreatedBy}
}
func tagView(item ports.Tag) tagJSON {
	return tagJSON{ID: item.ID, Name: item.Name, Version: item.Version}
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
	views := make([]opportunityJSON, 0, len(items))
	for _, item := range items {
		views = append(views, opportunityView(item))
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"items": views}})
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
	q.TenantID, q.CorpID = p.TenantID, q.CorpID
	q.IdempotencyKey = r.Header.Get("Idempotency-Key")
	item, err := h.service.CreateOpportunity(r.Context(), q)
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": opportunityView(item)})
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
	q.TenantID = p.TenantID
	q.OpportunityID = strings.TrimPrefix(r.URL.Path, OpportunitiesPath+"/")
	q.OpportunityID = strings.TrimSuffix(q.OpportunityID, "/stage")
	q.IdempotencyKey = r.Header.Get("Idempotency-Key")
	item, err := h.service.ChangeOpportunityStage(r.Context(), q)
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": opportunityView(item)})
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
	views := make([]tagJSON, 0, len(items))
	for _, item := range items {
		views = append(views, tagView(item))
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"items": views}})
}
func (h *OpportunityHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	var q struct {
		CorpID int64  `json:"corpId"`
		Name   string `json:"name"`
	}
	if decodeRequestJSON(w, r, &q) != nil {
		return
	}
	item, err := h.service.CreateTag(r.Context(), p.TenantID, q.CorpID, q.Name, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": tagView(item)})
}

func (h *OpportunityHandler) ListFollowUps(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	items, err := h.service.ListFollowUps(r.Context(), p.TenantID, queryInt(r, "corpId"), pathValue(r, "contacts", "follow-ups"))
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	views := make([]followUpJSON, 0, len(items))
	for _, item := range items {
		views = append(views, followUpView(item))
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"items": views, "nextCursor": ""}})
}

func (h *OpportunityHandler) AppendFollowUp(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	var q struct {
		CorpID  int64  `json:"corpId"`
		Content string `json:"content"`
	}
	if decodeRequestJSON(w, r, &q) != nil {
		return
	}
	item, err := h.service.AppendFollowUp(r.Context(), ports.AppendFollowUpCommand{TenantID: p.TenantID, CorpID: q.CorpID, ContactID: pathValue(r, "contacts", "follow-ups"), Content: q.Content, CreatedBy: p.UserID, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": followUpView(item)})
}

func (h *OpportunityHandler) RenameTag(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	var q struct {
		CorpID  int64  `json:"corpId"`
		Name    string `json:"name"`
		Version int64  `json:"version"`
	}
	if decodeRequestJSON(w, r, &q) != nil {
		return
	}
	item, err := h.service.RenameTag(r.Context(), p.TenantID, q.CorpID, pathValue(r, "tags", ""), q.Name, q.Version, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": tagView(item)})
}

func (h *OpportunityHandler) BindTags(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	var q struct {
		CorpID     int64    `json:"corpId"`
		ContactIDs []string `json:"contactIds"`
	}
	if decodeRequestJSON(w, r, &q) != nil {
		return
	}
	if err := h.service.BindTags(r.Context(), p.TenantID, q.CorpID, pathValue(r, "tags", ""), q.ContactIDs, r.Header.Get("Idempotency-Key")); err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"ok": true}})
}

func pathValue(r *http.Request, left, right string) string {
	value := strings.TrimPrefix(r.URL.Path, "/dashboard/scrm/")
	value = strings.TrimPrefix(value, left+"/")
	if right != "" {
		value = strings.TrimSuffix(value, "/"+right)
	}
	return value
}

func queryInt(r *http.Request, key string) int64 {
	var n int64
	_, _ = fmt.Sscan(r.URL.Query().Get(key), &n)
	return n
}
func writeSCRMError(w http.ResponseWriter, err error) {
	if errors.Is(err, ports.ErrAssignmentConflict) {
		writeError(w, http.StatusConflict, "resource version conflict")
		return
	}
	if errors.Is(err, application.ErrInvalidArgument) {
		writeError(w, 422, "invalid request")
		return
	}
	writeError(w, 503, "service unavailable")
}
