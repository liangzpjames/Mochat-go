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
const ContactsPath = "/dashboard/scrm/contacts"

const (
	contactPermissionView = "/customer/contact#get"
	contactPermissionEdit = "/customer/contact@edit#post"
)

type CustomerLifecycleService interface {
	ListContacts(context.Context, application.ListContactsQuery) (ports.ContactPage, error)
	GetContact(context.Context, int64, int64, string) (ports.ContactDetail, error)
	ListPublicPool(context.Context, application.ListPublicPoolQuery) (ports.AssignmentPage, error)
	UpdateAssignment(context.Context, ports.UpdateAssignmentCommand) (domain.CustomerAssignment, error)
	ReleaseToPublicPool(context.Context, int64, int64, string, int64, string) (domain.CustomerAssignment, error)
	ClaimFromPublicPool(context.Context, int64, int64, string, int64, int64, string) (domain.CustomerAssignment, error)
}

type CustomerLifecycleHandler struct {
	service    CustomerLifecycleService
	principal  PrincipalResolver
	authorizer LeadAuthorizer
}

func NewCustomerLifecycleHandler(service CustomerLifecycleService, principal PrincipalResolver, authorizer ...LeadAuthorizer) *CustomerLifecycleHandler {
	h := &CustomerLifecycleHandler{service: service, principal: principal}
	if len(authorizer) > 0 {
		h.authorizer = authorizer[0]
	}
	return h
}

func (h *CustomerLifecycleHandler) ListContacts(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid contact query")
		return
	}
	corpID, err := parsePositiveInt64(values.Get("corpId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "corpId is required")
		return
	}
	if !h.authorize(w, r, principal, corpID, contactPermissionView) {
		return
	}
	pageSize, err := parseOptionalNonNegativeInt(values.Get("pageSize"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pageSize")
		return
	}
	ownerIDs, err := parsePositiveInt64List(values["ownerId"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ownerId")
		return
	}
	query := application.ListContactsQuery{TenantID: principal.TenantID, CorpID: corpID, Keyword: values.Get("keyword"), OwnerIDs: ownerIDs, TagIDs: values["tagId"], Statuses: values["status"], Cursor: values.Get("cursor"), PageSize: pageSize}
	page, err := h.service.ListContacts(r.Context(), query)
	if err != nil {
		writeCustomerLifecycleError(w, err)
		return
	}
	items := make([]contactSummaryJSON, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, contactSummaryView(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items, "nextCursor": page.NextCursor}})
}

func (h *CustomerLifecycleHandler) GetContact(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	corpID, err := parsePositiveInt64(r.URL.Query().Get("corpId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "corpId is required")
		return
	}
	if !h.authorize(w, r, principal, corpID, contactPermissionView) {
		return
	}
	detail, err := h.service.GetContact(r.Context(), principal.TenantID, corpID, pathValue(r, "contacts", ""))
	if err != nil {
		writeCustomerLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": contactDetailView(detail)})
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
	if !h.authorize(w, r, principal, corpID, contactPermissionView) {
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
	if !h.authorize(w, r, principal, req.CorpID, contactPermissionEdit) {
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
	if !h.authorize(w, r, principal, req.CorpID, contactPermissionEdit) {
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

func (h *CustomerLifecycleHandler) authorize(w http.ResponseWriter, r *http.Request, principal Principal, corpID int64, permission string) bool {
	if h.authorizer == nil {
		return true
	}
	if corpID <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "corpId is required")
		return false
	}
	err := h.authorizer.Authorize(r.Context(), principal, corpID, permission)
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

type contactSummaryJSON struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Phone             string   `json:"phone"`
	OwnerID           *int64   `json:"ownerId"`
	AssignmentStatus  string   `json:"assignmentStatus"`
	TagNames          []string `json:"tagNames"`
	Version           int64    `json:"version"`
	AssignmentVersion int64    `json:"assignmentVersion"`
	UpdatedAt         string   `json:"updatedAt"`
}
type contactDetailJSON struct {
	contactSummaryJSON
	Assignment            assignmentJSON                    `json:"assignment"`
	Tags                  []ports.ContactTagSummary         `json:"tags"`
	WeComFriends          []ports.WeComFriendSummary        `json:"wecomFriends"`
	WeComFriendsAvailable bool                              `json:"wecomFriendsAvailable"`
	Opportunities         []ports.ContactOpportunitySummary `json:"opportunities"`
	FollowUps             []followUpJSON                    `json:"followUps"`
}

func contactSummaryView(item ports.ContactSummary) contactSummaryJSON {
	return contactSummaryJSON{ID: item.ID, Name: item.Name, Phone: item.Phone, OwnerID: item.OwnerID, AssignmentStatus: item.AssignmentStatus, TagNames: item.TagNames, Version: item.Version, AssignmentVersion: item.AssignmentVersion, UpdatedAt: item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")}
}
func contactDetailView(item ports.ContactDetail) contactDetailJSON {
	followUps := make([]followUpJSON, 0, len(item.FollowUps))
	for _, f := range item.FollowUps {
		followUps = append(followUps, followUpJSON{ID: f.ID, ContactID: item.ID, Content: f.Content, CreatedAt: f.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"), CreatedBy: f.CreatedBy})
	}
	return contactDetailJSON{contactSummaryJSON: contactSummaryView(item.ContactSummary), Assignment: assignmentViewJSON(item.Assignment), Tags: item.Tags, WeComFriends: item.WeComFriends, WeComFriendsAvailable: item.WeComFriendsAvailable, Opportunities: item.Opportunities, FollowUps: followUps}
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
func parsePositiveInt64List(raw []string) ([]int64, error) {
	result := make([]int64, 0, len(raw))
	for _, v := range raw {
		id, err := parsePositiveInt64(v)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}
func parseOptionalNonNegativeInt(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, errors.New("invalid non-negative integer")
	}
	return value, nil
}
func writeCustomerLifecycleError(w http.ResponseWriter, err error) {
	if errors.Is(err, ports.ErrAssignmentForbidden) {
		writeError(w, http.StatusForbidden, "assignment employee is outside corp scope")
		return
	}
	if errors.Is(err, ports.ErrAssignmentConflict) {
		writeError(w, http.StatusConflict, "assignment version conflict")
		return
	}
	if errors.Is(err, application.ErrInvalidArgument) {
		writeError(w, http.StatusUnprocessableEntity, "invalid request")
		return
	}
	if errors.Is(err, application.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact not found")
		return
	}
	writeError(w, http.StatusServiceUnavailable, "service unavailable")
}
