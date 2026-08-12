package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/domain"
)

const MaxRequestBodyBytes int64 = 64 << 10

var ErrLeadForbidden = errors.New("lead access forbidden")

const (
	leadPermissionView   = "/customer/clue/default#get"
	leadPermissionAdd    = "/customer/clue/default@add#post"
	leadPermissionAssign = "/customer/clue/default@assign#post"
	leadPermissionEdit   = "/customer/clue/default@edit#post"
)

type LeadService interface {
	CreateLead(context.Context, application.CreateLeadCommand) (application.CreateLeadResult, error)
	ListLeads(context.Context, application.ListLeadsQuery) (application.LeadPage, error)
	AssignLeads(context.Context, application.AssignLeadsCommand) ([]application.LeadMutationResult, error)
	TransitionLead(context.Context, application.TransitionLeadCommand) (application.LeadView, error)
	FindDuplicateLeads(context.Context, int64, int64, string, string) ([]application.LeadView, error)
}

type LeadAuthorizer interface {
	Authorize(context.Context, Principal, int64, string) error
}

type LeadHandler struct {
	service    LeadService
	principal  PrincipalResolver
	authorizer LeadAuthorizer
}

func NewLeadHandler(service LeadService, principal PrincipalResolver, authorizer ...LeadAuthorizer) *LeadHandler {
	h := &LeadHandler{service: service, principal: principal}
	if len(authorizer) > 0 {
		h.authorizer = authorizer[0]
	}
	return h
}

func (h *LeadHandler) Create(w nethttp.ResponseWriter, r *nethttp.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	if principal.EmployeeScopeRestricted {
		writeError(w, nethttp.StatusForbidden, "lead owner scope cannot be resolved")
		return
	}
	var request struct {
		BusinessKey string            `json:"businessKey"`
		Name        string            `json:"name"`
		Phone       string            `json:"phone"`
		Source      domain.LeadSource `json:"source"`
	}
	if err := decodeRequestJSON(w, r, &request); err != nil {
		var tooLarge *nethttp.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, nethttp.StatusRequestEntityTooLarge, "request body too large")
		} else {
			writeError(w, nethttp.StatusBadRequest, "invalid request JSON")
		}
		return
	}
	corpID := principal.CorpID
	if !h.authorize(w, r, principal, corpID, leadPermissionAdd) {
		return
	}
	result, err := h.service.CreateLead(r.Context(), application.CreateLeadCommand{TenantID: principal.TenantID, CorpID: corpID, BusinessKey: request.BusinessKey, Name: request.Name, Phone: request.Phone, Source: request.Source})
	if err != nil {
		writeApplicationError(w, err, nethttp.StatusUnprocessableEntity)
		return
	}
	status := nethttp.StatusOK
	if result.Created {
		status = nethttp.StatusCreated
	}
	writeJSON(w, status, map[string]any{"data": leadViewJSON(result.Lead)})
}

func (h *LeadHandler) List(w nethttp.ResponseWriter, r *nethttp.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, nethttp.StatusBadRequest, "invalid list query")
		return
	}
	query, err := parseListQuery(values, principal.TenantID, principal.CorpID)
	if err != nil {
		writeError(w, nethttp.StatusBadRequest, "invalid list query")
		return
	}
	if !h.authorize(w, r, principal, query.CorpID, leadPermissionView) {
		return
	}
	query.AllowedEmployeeIDs = append([]int64(nil), principal.AllowedEmployeeIDs...)
	query.EmployeeScopeRestricted = principal.EmployeeScopeRestricted
	page, err := h.service.ListLeads(r.Context(), query)
	if err != nil {
		writeApplicationError(w, err, nethttp.StatusBadRequest)
		return
	}
	items := make([]leadResponse, 0, len(page.Items))
	for _, lead := range page.Items {
		items = append(items, domainLeadJSON(lead))
	}
	writeJSON(w, nethttp.StatusOK, map[string]any{"data": map[string]any{"items": items, "nextCursor": page.NextCursor}})
}

func (h *LeadHandler) Assign(w nethttp.ResponseWriter, r *nethttp.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	if principal.EmployeeScopeRestricted {
		writeError(w, nethttp.StatusForbidden, "lead owner scope cannot be resolved")
		return
	}
	var request struct {
		OwnerID int64                            `json:"ownerId"`
		Targets []application.LeadMutationTarget `json:"targets"`
	}
	if err := decodeRequestJSON(w, r, &request); err != nil {
		writeError(w, nethttp.StatusBadRequest, "invalid request JSON")
		return
	}
	corpID := principal.CorpID
	if !h.authorize(w, r, principal, corpID, leadPermissionAssign) {
		return
	}
	if !principal.AllowsEmployee(request.OwnerID) {
		writeError(w, nethttp.StatusForbidden, "employee is outside dashboard scope")
		return
	}
	results, err := h.service.AssignLeads(r.Context(), application.AssignLeadsCommand{TenantID: principal.TenantID, CorpID: corpID, OwnerID: request.OwnerID, Targets: request.Targets})
	if err != nil {
		writeApplicationError(w, err, nethttp.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, nethttp.StatusOK, map[string]any{"data": map[string]any{"results": results}})
}

func (h *LeadHandler) Transition(w nethttp.ResponseWriter, r *nethttp.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	if principal.EmployeeScopeRestricted {
		writeError(w, nethttp.StatusForbidden, "lead owner scope cannot be resolved")
		return
	}
	var request struct {
		ID            string            `json:"id"`
		ToStatus      domain.LeadStatus `json:"toStatus"`
		Version       int64             `json:"version"`
		DiscardReason string            `json:"discardReason"`
	}
	if err := decodeRequestJSON(w, r, &request); err != nil {
		writeError(w, nethttp.StatusBadRequest, "invalid request JSON")
		return
	}
	corpID := principal.CorpID
	if !h.authorize(w, r, principal, corpID, leadPermissionEdit) {
		return
	}
	lead, err := h.service.TransitionLead(r.Context(), application.TransitionLeadCommand{TenantID: principal.TenantID, CorpID: corpID, LeadID: request.ID, ToStatus: request.ToStatus, Version: request.Version, DiscardReason: request.DiscardReason})
	if err != nil {
		writeApplicationError(w, err, nethttp.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, nethttp.StatusOK, map[string]any{"data": leadViewJSON(lead)})
}

func (h *LeadHandler) Duplicates(w nethttp.ResponseWriter, r *nethttp.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	if principal.EmployeeScopeRestricted {
		writeError(w, nethttp.StatusForbidden, "lead owner scope cannot be resolved")
		return
	}
	corpID := principal.CorpID
	if !h.authorize(w, r, principal, corpID, leadPermissionView) {
		return
	}
	items, err := h.service.FindDuplicateLeads(r.Context(), principal.TenantID, corpID, r.URL.Query().Get("businessKey"), r.URL.Query().Get("phone"))
	if err != nil {
		writeApplicationError(w, err, nethttp.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, nethttp.StatusOK, map[string]any{"data": map[string]any{"items": items}})
}

func (h *LeadHandler) authorize(w nethttp.ResponseWriter, r *nethttp.Request, principal Principal, corpID int64, permission string) bool {
	if h.authorizer == nil {
		return true
	}
	if corpID <= 0 {
		writeError(w, nethttp.StatusUnprocessableEntity, "corpId is required")
		return false
	}
	err := h.authorizer.Authorize(r.Context(), principal, corpID, permission)
	if errors.Is(err, ErrLeadForbidden) {
		writeError(w, nethttp.StatusForbidden, "forbidden")
		return false
	}
	if err != nil {
		writeError(w, nethttp.StatusServiceUnavailable, "authorization unavailable")
		return false
	}
	return true
}

func (h *LeadHandler) resolvePrincipal(w nethttp.ResponseWriter, r *nethttp.Request) (Principal, bool) {
	if h == nil || h.principal == nil {
		writeError(w, nethttp.StatusUnauthorized, "authentication required")
		return Principal{}, false
	}
	principal, err := h.principal.Resolve(r)
	if errors.Is(err, ErrPrincipalUnavailable) {
		writeError(w, nethttp.StatusServiceUnavailable, "authentication service unavailable")
		return Principal{}, false
	}
	if err != nil || principal.UserID <= 0 || principal.TenantID <= 0 || principal.CorpID <= 0 {
		writeError(w, nethttp.StatusUnauthorized, "authentication required")
		return Principal{}, false
	}
	return principal, true
}

func decodeRequestJSON(w nethttp.ResponseWriter, r *nethttp.Request, destination any) error {
	r.Body = nethttp.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	// Realm selectors are deliberately ignored at the transport boundary. The
	// only authority for tenant/corp/actor is the server-created principal;
	// stripping these legacy client fields preserves strict decoding for every
	// real business field without allowing a client value to affect scope.
	for _, key := range []string{"tenantId", "tenant_id", "corpId", "corp_id", "actorId", "actor_id"} {
		delete(raw, key)
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	strict := json.NewDecoder(bytes.NewReader(payload))
	strict.DisallowUnknownFields()
	return strict.Decode(destination)
}

func parseListQuery(values url.Values, tenantID, corpID int64) (application.ListLeadsQuery, error) {
	for _, key := range []string{"cursor", "pageSize", "keyword", "createdFrom", "createdTo"} {
		if len(values[key]) > 1 {
			return application.ListLeadsQuery{}, errors.New("duplicate list parameter")
		}
	}
	cursor := values.Get("cursor")
	if strings.HasPrefix(strings.TrimSpace(cursor), "-") {
		return application.ListLeadsQuery{}, errors.New("invalid cursor")
	}
	var pageSize int
	if raw := values.Get("pageSize"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return application.ListLeadsQuery{}, errors.New("invalid page size")
		}
		pageSize = parsed
	}
	statuses := make([]domain.LeadStatus, 0, len(values["status"]))
	for _, raw := range values["status"] {
		status := domain.LeadStatus(raw)
		if status != domain.LeadStatusNew && status != domain.LeadStatusQualified && status != domain.LeadStatusConverted && status != domain.LeadStatusDiscarded {
			return application.ListLeadsQuery{}, errors.New("invalid status")
		}
		statuses = append(statuses, status)
	}
	sources := make([]domain.LeadSource, 0, len(values["source"]))
	for _, raw := range values["source"] {
		source := domain.LeadSource(raw)
		if source != domain.LeadSourceManual && source != domain.LeadSourceImport && source != domain.LeadSourceWeCom {
			return application.ListLeadsQuery{}, errors.New("invalid source")
		}
		sources = append(sources, source)
	}
	ownerIDs := make([]int64, 0, len(values["ownerId"]))
	for _, raw := range values["ownerId"] {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return application.ListLeadsQuery{}, errors.New("invalid owner")
		}
		ownerIDs = append(ownerIDs, id)
	}
	parseTime := func(key string) (time.Time, error) {
		raw := values.Get(key)
		if raw == "" {
			return time.Time{}, nil
		}
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, err
		}
		return value.UTC(), nil
	}
	from, err := parseTime("createdFrom")
	if err != nil {
		return application.ListLeadsQuery{}, err
	}
	to, err := parseTime("createdTo")
	if err != nil {
		return application.ListLeadsQuery{}, err
	}
	return application.ListLeadsQuery{TenantID: tenantID, CorpID: corpID, Keyword: strings.TrimSpace(values.Get("keyword")), Statuses: statuses, Sources: sources, OwnerIDs: ownerIDs, CreatedFrom: from, CreatedTo: to, Cursor: cursor, PageSize: pageSize}, nil
}

type leadResponse struct {
	ID                 string            `json:"id"`
	BusinessKey        string            `json:"businessKey"`
	Name               string            `json:"name"`
	Phone              string            `json:"phone"`
	Source             domain.LeadSource `json:"source"`
	Status             domain.LeadStatus `json:"status"`
	OwnerID            *int64            `json:"ownerId"`
	ConvertedContactID string            `json:"convertedContactId"`
	DiscardReason      string            `json:"discardReason"`
	Version            int64             `json:"version"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
}

func leadViewJSON(lead application.LeadView) leadResponse {
	return leadResponse{ID: lead.ID, BusinessKey: lead.BusinessKey, Name: lead.Name, Phone: lead.Phone, Source: lead.Source, Status: lead.Status, OwnerID: lead.OwnerID, ConvertedContactID: lead.ConvertedContactID, DiscardReason: lead.DiscardReason, Version: lead.Version, CreatedAt: lead.CreatedAt.UTC(), UpdatedAt: lead.UpdatedAt.UTC()}
}
func domainLeadJSON(lead domain.Lead) leadResponse {
	return leadResponse{ID: lead.ID, BusinessKey: lead.BusinessKey, Name: lead.Name.String(), Phone: lead.Phone, Source: lead.Source, Status: lead.Status, OwnerID: lead.OwnerID, ConvertedContactID: lead.ConvertedContactID, DiscardReason: lead.DiscardReason, Version: lead.Version, CreatedAt: lead.CreatedAt.UTC(), UpdatedAt: lead.UpdatedAt.UTC()}
}

func writeApplicationError(w nethttp.ResponseWriter, err error, invalidStatus int) {
	switch {
	case errors.Is(err, application.ErrInvalidArgument):
		writeError(w, invalidStatus, "invalid request")
	case errors.Is(err, application.ErrConflict):
		writeError(w, nethttp.StatusConflict, "version conflict")
	case errors.Is(err, application.ErrNotFound):
		writeError(w, nethttp.StatusNotFound, "lead not found")
	case errors.Is(err, application.ErrDuplicate):
		writeError(w, nethttp.StatusUnprocessableEntity, "duplicate lead")
	case errors.Is(err, application.ErrUnavailable):
		writeError(w, nethttp.StatusServiceUnavailable, "service unavailable")
	default:
		writeError(w, nethttp.StatusInternalServerError, "internal server error")
	}
}

func writeError(w nethttp.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"code": status, "message": message})
}
func writeJSON(w nethttp.ResponseWriter, status int, body any) {
	if status >= 200 && status < 300 {
		if value, ok := body.(map[string]any); ok {
			if data, hasData := value["data"]; hasData {
				if _, hasCode := value["code"]; !hasCode {
					body = map[string]any{"code": status, "msg": "success", "data": data}
				}
			}
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
