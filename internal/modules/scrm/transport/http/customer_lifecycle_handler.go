package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

const AssignmentsPath = "/dashboard/scrm/assignments"

type CustomerLifecycleService interface {
	ListPublicPool(context.Context, application.ListPublicPoolQuery) (ports.AssignmentPage, error)
	UpdateAssignment(context.Context, ports.UpdateAssignmentCommand) (domain.CustomerAssignment, error)
	ReleaseToPublicPool(context.Context, int64, int64, string, int64, string) (domain.CustomerAssignment, error)
	ClaimFromPublicPool(context.Context, int64, int64, string, int64, int64, string) (domain.CustomerAssignment, error)
}

type CustomerLifecycleHandler struct {
	service   CustomerLifecycleService
	principal PrincipalResolver
}

func NewCustomerLifecycleHandler(service CustomerLifecycleService, principal PrincipalResolver) *CustomerLifecycleHandler {
	return &CustomerLifecycleHandler{service: service, principal: principal}
}

func (h *CustomerLifecycleHandler) ListPublicPool(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid list query")
		return
	}
	corpID, err := parsePositiveInt64(values.Get("corpId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "corpId is required")
		return
	}
	pageSize, _ := strconv.Atoi(values.Get("pageSize"))
	page, err := h.service.ListPublicPool(r.Context(), application.ListPublicPoolQuery{TenantID: principal.TenantID, CorpID: corpID, Cursor: values.Get("cursor"), PageSize: pageSize})
	if err != nil {
		writeCustomerLifecycleError(w, err)
		return
	}
	items := make([]assignmentJSON, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, assignmentViewJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items, "nextCursor": page.NextCursor}})
}

func (h *CustomerLifecycleHandler) UpdateAssignment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	var req struct {
		CorpID          int64   `json:"corpId"`
		ContactID       string  `json:"contactId"`
		OwnerID         *int64  `json:"ownerId"`
		CollaboratorIDs []int64 `json:"collaboratorIds"`
		Version         int64   `json:"version"`
	}
	if err := decodeRequestJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request JSON")
		return
	}
	item, err := h.service.UpdateAssignment(r.Context(), ports.UpdateAssignmentCommand{TenantID: principal.TenantID, CorpID: req.CorpID, ContactID: req.ContactID, OwnerID: req.OwnerID, CollaboratorIDs: req.CollaboratorIDs, Version: req.Version, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeCustomerLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": assignmentViewJSON(item)})
}

func (h *CustomerLifecycleHandler) ReleaseToPublicPool(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, false)
}
func (h *CustomerLifecycleHandler) ClaimFromPublicPool(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, true)
}

func (h *CustomerLifecycleHandler) transition(w http.ResponseWriter, r *http.Request, claim bool) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	var req struct {
		CorpID    int64  `json:"corpId"`
		ContactID string `json:"contactId"`
		Version   int64  `json:"version"`
		UserID    int64  `json:"userId"`
	}
	if err := decodeRequestJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request JSON")
		return
	}
	var item domain.CustomerAssignment
	var err error
	if claim {
		item, err = h.service.ClaimFromPublicPool(r.Context(), principal.TenantID, req.CorpID, req.ContactID, principal.UserID, req.Version, r.Header.Get("Idempotency-Key"))
	} else {
		item, err = h.service.ReleaseToPublicPool(r.Context(), principal.TenantID, req.CorpID, req.ContactID, req.Version, r.Header.Get("Idempotency-Key"))
	}
	if err != nil {
		writeCustomerLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": assignmentViewJSON(item)})
}

func (h *CustomerLifecycleHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	if h == nil || h.service == nil || h.principal == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return Principal{}, false
	}
	p, err := h.principal.Resolve(r)
	if errors.Is(err, ErrPrincipalUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "authentication service unavailable")
		return Principal{}, false
	}
	if err != nil || p.UserID <= 0 || p.TenantID <= 0 {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return Principal{}, false
	}
	return p, true
}

type assignmentJSON struct {
	ID              string  `json:"id"`
	ContactID       string  `json:"contactId"`
	OwnerID         *int64  `json:"ownerId"`
	CollaboratorIDs []int64 `json:"collaboratorIds"`
	Status          string  `json:"status"`
	Version         int64   `json:"version"`
}

func assignmentViewJSON(item domain.CustomerAssignment) assignmentJSON {
	return assignmentJSON{ID: item.ID, ContactID: item.ContactID, OwnerID: item.OwnerID, CollaboratorIDs: item.CollaboratorIDs, Status: item.Status, Version: item.Version}
}
func parsePositiveInt64(raw string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid positive integer")
	}
	return value, nil
}
func writeCustomerLifecycleError(w http.ResponseWriter, err error) {
	if errors.Is(err, ports.ErrAssignmentConflict) {
		writeError(w, http.StatusConflict, "assignment version conflict")
		return
	}
	if errors.Is(err, application.ErrInvalidArgument) {
		writeError(w, http.StatusUnprocessableEntity, "invalid request")
		return
	}
	writeError(w, http.StatusServiceUnavailable, "service unavailable")
}
