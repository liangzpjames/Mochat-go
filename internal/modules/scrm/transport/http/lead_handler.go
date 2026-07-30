package http

import (
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

type LeadService interface {
	CreateLead(context.Context, application.CreateLeadCommand) (application.CreateLeadResult, error)
	ListLeads(context.Context, application.ListLeadsQuery) (application.LeadPage, error)
}

type LeadHandler struct {
	service   LeadService
	principal PrincipalResolver
}

func NewLeadHandler(service LeadService, principal PrincipalResolver) *LeadHandler {
	return &LeadHandler{service: service, principal: principal}
}

func (h *LeadHandler) Create(w nethttp.ResponseWriter, r *nethttp.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}

	var request struct {
		BusinessKey string            `json:"businessKey"`
		Name        string            `json:"name"`
		Source      domain.LeadSource `json:"source"`
	}
	if err := decodeRequestJSON(w, r, &request); err != nil {
		var tooLarge *nethttp.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, nethttp.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, nethttp.StatusBadRequest, "invalid request JSON")
		return
	}

	result, err := h.service.CreateLead(r.Context(), application.CreateLeadCommand{
		TenantID:    principal.TenantID,
		BusinessKey: request.BusinessKey,
		Name:        request.Name,
		Source:      request.Source,
	})
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
	query, err := parseListQuery(values, principal.TenantID)
	if err != nil {
		writeError(w, nethttp.StatusBadRequest, "invalid list query")
		return
	}

	page, err := h.service.ListLeads(r.Context(), query)
	if err != nil {
		writeApplicationError(w, err, nethttp.StatusBadRequest)
		return
	}

	items := make([]leadResponse, 0, len(page.Items))
	for _, lead := range page.Items {
		items = append(items, domainLeadJSON(lead))
	}
	writeJSON(w, nethttp.StatusOK, map[string]any{
		"data": map[string]any{
			"items":      items,
			"nextCursor": page.NextCursor,
		},
	})
}

func (h *LeadHandler) resolvePrincipal(w nethttp.ResponseWriter, r *nethttp.Request) (Principal, bool) {
	if h == nil || h.principal == nil {
		writeError(w, nethttp.StatusUnauthorized, "authentication required")
		return Principal{}, false
	}
	principal, err := h.principal.Resolve(r)
	if err != nil || principal.UserID <= 0 || principal.TenantID <= 0 {
		writeError(w, nethttp.StatusUnauthorized, "authentication required")
		return Principal{}, false
	}
	return principal, true
}

func decodeRequestJSON(w nethttp.ResponseWriter, r *nethttp.Request, destination any) error {
	r.Body = nethttp.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func parseListQuery(values url.Values, tenantID int64) (application.ListLeadsQuery, error) {
	if len(values["cursor"]) > 1 || len(values["pageSize"]) > 1 {
		return application.ListLeadsQuery{}, errors.New("duplicate list parameter")
	}
	cursor := values.Get("cursor")
	if strings.HasPrefix(strings.TrimSpace(cursor), "-") {
		return application.ListLeadsQuery{}, errors.New("invalid cursor")
	}
	var pageSize int
	if rawPageSize := values.Get("pageSize"); rawPageSize != "" {
		parsed, err := strconv.Atoi(rawPageSize)
		if err != nil || parsed < 0 {
			return application.ListLeadsQuery{}, errors.New("invalid page size")
		}
		pageSize = parsed
	}
	return application.ListLeadsQuery{TenantID: tenantID, Cursor: cursor, PageSize: pageSize}, nil
}

type leadResponse struct {
	ID          string            `json:"id"`
	BusinessKey string            `json:"businessKey"`
	Name        string            `json:"name"`
	Source      domain.LeadSource `json:"source"`
	Status      domain.LeadStatus `json:"status"`
	Version     int64             `json:"version"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

func leadViewJSON(lead application.LeadView) leadResponse {
	return leadResponse{
		ID: lead.ID, BusinessKey: lead.BusinessKey, Name: lead.Name,
		Source: lead.Source, Status: lead.Status, Version: lead.Version,
		CreatedAt: lead.CreatedAt.UTC(), UpdatedAt: lead.UpdatedAt.UTC(),
	}
}

func domainLeadJSON(lead domain.Lead) leadResponse {
	return leadResponse{
		ID: lead.ID, BusinessKey: lead.BusinessKey, Name: lead.Name.String(),
		Source: lead.Source, Status: lead.Status, Version: lead.Version,
		CreatedAt: lead.CreatedAt.UTC(), UpdatedAt: lead.UpdatedAt.UTC(),
	}
}

func writeApplicationError(w nethttp.ResponseWriter, err error, invalidStatus int) {
	switch {
	case errors.Is(err, application.ErrInvalidArgument):
		writeError(w, invalidStatus, "invalid request")
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
