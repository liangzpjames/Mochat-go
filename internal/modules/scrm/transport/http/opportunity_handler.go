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

const (
	opportunityPermissionView = "/customer/opportunity#get"
	opportunityPermissionEdit = "/customer/opportunity@edit#post"
	tagPermissionView         = "/customer/tags#get"
	tagPermissionAdd          = "/customer/tags@add#post"
	tagPermissionEdit         = "/customer/tags@edit#post"
)

type OpportunityService interface {
	ListOpportunities(context.Context, ports.OpportunityFilter) (ports.OpportunityPage, error)
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
	service    OpportunityService
	principal  PrincipalResolver
	authorizer LeadAuthorizer
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
type changeOpportunityStageRequest struct {
	OpportunityID  string `json:"opportunityId,omitempty"`
	StageID        string `json:"stageId"`
	Version        int64  `json:"version"`
	LostReason     string `json:"lostReason,omitempty"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}
type appendFollowUpRequest struct {
	Content        string `json:"content"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}
type tagJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int64  `json:"version"`
}

func opportunityView(item ports.Opportunity) opportunityJSON {
	startDate, endDate := "", ""
	if !item.StartDate.IsZero() {
		startDate = item.StartDate.Format("2006-01-02")
	}
	if !item.EndDate.IsZero() {
		endDate = item.EndDate.Format("2006-01-02")
	}
	return opportunityJSON{ID: item.ID, ContactID: item.ContactID, Stage: item.Stage, Status: item.Status, LostReason: item.LostReason, OwnerID: item.OwnerID, Amount: item.Amount, StartDate: startDate, EndDate: endDate, Version: item.Version}
}
func followUpView(item ports.FollowUpRecord) followUpJSON {
	return followUpJSON{ID: item.ID, ContactID: item.ContactID, Content: item.Content, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), CreatedBy: item.CreatedBy}
}
func tagView(item ports.Tag) tagJSON {
	return tagJSON{ID: item.ID, Name: item.Name, Version: item.Version}
}

func NewOpportunityHandler(service OpportunityService, principal PrincipalResolver, authorizer ...LeadAuthorizer) *OpportunityHandler {
	h := &OpportunityHandler{service: service, principal: principal}
	if len(authorizer) > 0 {
		h.authorizer = authorizer[0]
	}
	return h
}

func (h *OpportunityHandler) List(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	corpID := p.CorpID
	if !h.authorize(w, r, p, corpID, opportunityPermissionView) {
		return
	}
	var ownerID *int64
	if value := queryInt(r, "ownerId"); value > 0 {
		ownerID = &value
	}
	ownerID = application.RestrictOwnerIDForHTTP(ownerID, p.AllowedEmployeeIDs, p.EmployeeScopeRestricted)
	page, err := h.service.ListOpportunities(r.Context(), ports.OpportunityFilter{TenantID: p.TenantID, CorpID: corpID, Stage: strings.TrimSpace(r.URL.Query().Get("stage")), Status: strings.TrimSpace(r.URL.Query().Get("status")), OwnerID: ownerID, AllowedEmployeeIDs: p.AllowedEmployeeIDs, EmployeeScopeRestricted: p.EmployeeScopeRestricted, Cursor: strings.TrimSpace(r.URL.Query().Get("cursor")), PageSize: int(queryInt(r, "pageSize"))})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	views := make([]opportunityJSON, 0, len(page.Items))
	for _, item := range page.Items {
		views = append(views, opportunityView(item))
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"items": views, "nextCursor": page.NextCursor}})
}
func (h *OpportunityHandler) Create(w http.ResponseWriter, r *http.Request) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, 401, "authentication required")
		return
	}
	var request struct {
		ContactID      string  `json:"contactId"`
		Stage          string  `json:"stage"`
		OwnerID        int64   `json:"ownerId"`
		Amount         float64 `json:"amount"`
		StartDate      string  `json:"startDate"`
		EndDate        string  `json:"endDate"`
		IdempotencyKey string  `json:"idempotencyKey,omitempty"`
	}
	if decodeOpportunityRequestJSON(w, r, &request) != nil {
		return
	}
	q := ports.CreateOpportunityCommand{TenantID: p.TenantID, CorpID: p.CorpID, ContactID: request.ContactID, Stage: request.Stage, OwnerID: request.OwnerID, Amount: request.Amount, StartDate: request.StartDate, EndDate: request.EndDate, IdempotencyKey: r.Header.Get("Idempotency-Key")}
	if !h.authorize(w, r, p, q.CorpID, opportunityPermissionEdit) {
		return
	}
	if q.OwnerID > 0 && !p.AllowsEmployee(q.OwnerID) {
		writeError(w, http.StatusForbidden, "employee is outside dashboard scope")
		return
	}
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
	if p.EmployeeScopeRestricted {
		writeError(w, http.StatusForbidden, "opportunity owner scope cannot be resolved")
		return
	}
	var q changeOpportunityStageRequest
	if decodeOpportunityRequestJSON(w, r, &q) != nil {
		return
	}
	corpID := p.CorpID
	if !h.authorize(w, r, p, corpID, opportunityPermissionEdit) {
		return
	}
	opportunityID := pathValue(r, "opportunities", "stage")
	if bodyID := strings.TrimSpace(q.OpportunityID); bodyID != "" && bodyID != opportunityID {
		writeError(w, http.StatusUnprocessableEntity, "opportunityId must match path")
		return
	}
	idempotencyKey, ok := resolveOpportunityIdempotencyKey(w, r, q.IdempotencyKey)
	if !ok {
		return
	}
	item, err := h.service.ChangeOpportunityStage(r.Context(), ports.ChangeOpportunityStageCommand{
		TenantID:       p.TenantID,
		CorpID:         corpID,
		OpportunityID:  opportunityID,
		StageID:        q.StageID,
		Version:        q.Version,
		LostReason:     q.LostReason,
		IdempotencyKey: idempotencyKey,
	})
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
	corpID := p.CorpID
	if !h.authorize(w, r, p, corpID, tagPermissionView) {
		return
	}
	items, err := h.service.ListTags(r.Context(), p.TenantID, corpID)
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
		Name string `json:"name"`
	}
	if decodeOpportunityRequestJSON(w, r, &q) != nil {
		return
	}
	qCorpID := p.CorpID
	if !h.authorize(w, r, p, qCorpID, tagPermissionAdd) {
		return
	}
	item, err := h.service.CreateTag(r.Context(), p.TenantID, qCorpID, q.Name, r.Header.Get("Idempotency-Key"))
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
	corpID := p.CorpID
	if !h.authorize(w, r, p, corpID, contactPermissionView) {
		return
	}
	items, err := h.service.ListFollowUps(r.Context(), p.TenantID, corpID, pathValue(r, "contacts", "follow-ups"))
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
	var q appendFollowUpRequest
	if decodeOpportunityRequestJSON(w, r, &q) != nil {
		return
	}
	corpID := p.CorpID
	if !h.authorizeAny(w, r, p, corpID, contactPermissionEdit, opportunityPermissionEdit) {
		return
	}
	idempotencyKey, ok := resolveOpportunityIdempotencyKey(w, r, q.IdempotencyKey)
	if !ok {
		return
	}
	item, err := h.service.AppendFollowUp(r.Context(), ports.AppendFollowUpCommand{TenantID: p.TenantID, CorpID: corpID, ContactID: pathValue(r, "contacts", "follow-ups"), Content: q.Content, CreatedBy: p.UserID, IdempotencyKey: idempotencyKey})
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
		Name    string `json:"name"`
		Version int64  `json:"version"`
	}
	if decodeOpportunityRequestJSON(w, r, &q) != nil {
		return
	}
	corpID := p.CorpID
	if !h.authorize(w, r, p, corpID, tagPermissionEdit) {
		return
	}
	item, err := h.service.RenameTag(r.Context(), p.TenantID, corpID, pathValue(r, "tags", ""), q.Name, q.Version, r.Header.Get("Idempotency-Key"))
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
		ContactIDs []string `json:"contactIds"`
	}
	if decodeOpportunityRequestJSON(w, r, &q) != nil {
		return
	}
	corpID := p.CorpID
	if !h.authorize(w, r, p, corpID, contactPermissionEdit) {
		return
	}
	if err := h.service.BindTags(r.Context(), p.TenantID, corpID, pathValue(r, "tags", ""), q.ContactIDs, r.Header.Get("Idempotency-Key")); err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"ok": true}})
}

func (h *OpportunityHandler) authorize(w http.ResponseWriter, r *http.Request, p Principal, corpID int64, permission string) bool {
	if h.authorizer == nil {
		return true
	}
	err := h.authorizer.Authorize(r.Context(), p, p.CorpID, permission)
	if errors.Is(err, ErrLeadForbidden) {
		writeError(w, http.StatusForbidden, "forbidden")
		return false
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "authorization unavailable")
		return false
	}
	return true
}

func (h *OpportunityHandler) authorizeAny(w http.ResponseWriter, r *http.Request, p Principal, corpID int64, permissions ...string) bool {
	if h.authorizer == nil {
		return true
	}
	for _, permission := range permissions {
		err := h.authorizer.Authorize(r.Context(), p, p.CorpID, permission)
		if err == nil {
			return true
		}
		if !errors.Is(err, ErrLeadForbidden) {
			writeError(w, http.StatusServiceUnavailable, "authorization unavailable")
			return false
		}
	}
	writeError(w, http.StatusForbidden, "forbidden")
	return false
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

func decodeOpportunityRequestJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if err := decodeRequestJSON(w, r, destination); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request JSON")
		return err
	}
	return nil
}

func resolveOpportunityIdempotencyKey(w http.ResponseWriter, r *http.Request, bodyKey string) (string, bool) {
	bodyKey = strings.TrimSpace(bodyKey)
	headerKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if bodyKey != "" && headerKey != "" && bodyKey != headerKey {
		writeError(w, http.StatusUnprocessableEntity, "idempotencyKey must match Idempotency-Key header")
		return "", false
	}
	if bodyKey != "" {
		return bodyKey, true
	}
	if headerKey != "" {
		return headerKey, true
	}
	writeError(w, http.StatusUnprocessableEntity, "idempotencyKey is required")
	return "", false
}

func writeSCRMError(w http.ResponseWriter, err error) {
	if errors.Is(err, ports.ErrAssignmentForbidden) {
		writeError(w, http.StatusForbidden, "resource is outside corp scope")
		return
	}
	if errors.Is(err, ports.ErrAssignmentConflict) {
		writeError(w, http.StatusConflict, "resource version conflict")
		return
	}
	if errors.Is(err, application.ErrInvalidArgument) {
		writeError(w, 422, "invalid request")
		return
	}
	if errors.Is(err, ports.ErrDuplicateTagName) {
		writeError(w, http.StatusUnprocessableEntity, "duplicate tag name")
		return
	}
	if errors.Is(err, ports.ErrInvalidOpportunityTransition) {
		writeError(w, http.StatusUnprocessableEntity, "invalid opportunity transition")
		return
	}
	if errors.Is(err, application.ErrNotFound) || errors.Is(err, ports.ErrContactNotFound) || errors.Is(err, ports.ErrTagNotFound) || errors.Is(err, ports.ErrTagGroupNotFound) || errors.Is(err, ports.ErrOpportunityNotFound) || errors.Is(err, ports.ErrStageNotFound) {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	writeError(w, 503, "service unavailable")
}
