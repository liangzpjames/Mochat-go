package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	CreateContact(context.Context, ports.CreateContactCommand) (ports.ContactSummary, error)
	ListPublicPool(context.Context, application.ListPublicPoolQuery) (ports.AssignmentPage, error)
	UpdateAssignment(context.Context, ports.UpdateAssignmentCommand) (domain.CustomerAssignment, error)
	MoveToPublicPool(context.Context, ports.MoveToPublicPoolCommand) (domain.CustomerAssignment, error)
	ClaimFromPublicPool(context.Context, ports.ClaimPublicPoolCommand) (domain.CustomerAssignment, error)
	BatchClaimFromPublicPool(context.Context, application.BatchClaimPublicPoolCommand) ([]application.PublicPoolMutationResult, error)
}

func (h *CustomerLifecycleHandler) CreateContact(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	var req struct {
		CorpID int64  `json:"corpId"`
		Name   string `json:"name"`
		Phone  string `json:"phone"`
	}
	if decodeRequestJSON(w, r, &req) != nil {
		writeError(w, 400, "invalid request JSON")
		return
	}
	if !h.authorize(w, r, principal, req.CorpID, contactPermissionEdit) {
		return
	}
	item, err := h.service.CreateContact(r.Context(), ports.CreateContactCommand{TenantID: principal.TenantID, CorpID: req.CorpID, ActorID: principal.UserID, Name: req.Name, Phone: req.Phone})
	if err != nil {
		writeCustomerLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": contactSummaryView(item)})
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
	pageSize, err := parseOptionalNonNegativeInt(values.Get("pageSize"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pageSize")
		return
	}
	previousOwnerIDs, err := parsePositiveInt64List(values["previousOwnerId"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid previousOwnerId")
		return
	}
	page, err := h.service.ListPublicPool(r.Context(), application.ListPublicPoolQuery{
		TenantID: principal.TenantID, CorpID: corpID, Keyword: values.Get("keyword"), Sources: values["source"],
		BusinessTypes: values["businessType"], TagIDs: values["tagId"], Regions: values["region"],
		Reasons: values["reason"], PreviousOwnerIDs: previousOwnerIDs, Cursor: values.Get("cursor"), PageSize: pageSize,
	})
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
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	var req struct {
		CorpID    int64  `json:"corpId"`
		ContactID string `json:"contactId"`
		Version   int64  `json:"version"`
		Action    string `json:"action"`
		Reason    string `json:"reason"`
	}
	if err := decodeRequestJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request JSON")
		return
	}
	if !h.authorize(w, r, principal, req.CorpID, contactPermissionEdit) {
		return
	}
	item, err := h.service.MoveToPublicPool(r.Context(), ports.MoveToPublicPoolCommand{TenantID: principal.TenantID, CorpID: req.CorpID, ContactID: req.ContactID, ActorID: principal.UserID, Version: req.Version, Action: req.Action, Reason: req.Reason, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeCustomerLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": assignmentViewJSON(item)})
}
func (h *CustomerLifecycleHandler) ClaimFromPublicPool(w http.ResponseWriter, r *http.Request) {
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
	if req.UserID != principal.UserID {
		writeError(w, http.StatusForbidden, "claim user is outside principal scope")
		return
	}
	item, err := h.service.ClaimFromPublicPool(r.Context(), ports.ClaimPublicPoolCommand{TenantID: principal.TenantID, CorpID: req.CorpID, ContactID: req.ContactID, UserID: req.UserID, Version: req.Version, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeCustomerLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": assignmentViewJSON(item)})
}

func (h *CustomerLifecycleHandler) BatchClaimFromPublicPool(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	var req struct {
		CorpID  int64                               `json:"corpId"`
		UserID  int64                               `json:"userId"`
		Targets []application.PublicPoolClaimTarget `json:"targets"`
	}
	if err := decodeRequestJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request JSON")
		return
	}
	if !h.authorize(w, r, principal, req.CorpID, contactPermissionEdit) {
		return
	}
	if req.UserID != principal.UserID {
		writeError(w, http.StatusForbidden, "claim user is outside principal scope")
		return
	}
	results, err := h.service.BatchClaimFromPublicPool(r.Context(), application.BatchClaimPublicPoolCommand{TenantID: principal.TenantID, CorpID: req.CorpID, UserID: req.UserID, Targets: req.Targets})
	if err != nil {
		writeCustomerLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"results": results}})
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
	ID              string   `json:"id"`
	ContactID       string   `json:"contactId"`
	OwnerID         *int64   `json:"ownerId"`
	CollaboratorIDs []int64  `json:"collaboratorIds"`
	Status          string   `json:"status"`
	Version         int64    `json:"version"`
	ContactName     string   `json:"contactName"`
	Source          string   `json:"source"`
	BusinessType    string   `json:"businessType"`
	TagNames        []string `json:"tagNames"`
	Region          string   `json:"region"`
	RecycleCount    int64    `json:"recycleCount"`
	PoolAction      string   `json:"poolAction"`
	PoolReason      string   `json:"poolReason"`
	PreviousOwnerID *int64   `json:"previousOwnerId"`
	LastFollowUpAt  string   `json:"lastFollowUpAt"`
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
	tagNames := item.TagNames
	if tagNames == nil {
		tagNames = []string{}
	}
	return contactSummaryJSON{ID: item.ID, Name: item.Name, Phone: item.Phone, OwnerID: item.OwnerID, AssignmentStatus: item.AssignmentStatus, TagNames: tagNames, Version: item.Version, AssignmentVersion: item.AssignmentVersion, UpdatedAt: item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")}
}
func contactDetailView(item ports.ContactDetail) contactDetailJSON {
	followUps := make([]followUpJSON, 0, len(item.FollowUps))
	for _, f := range item.FollowUps {
		followUps = append(followUps, followUpJSON{ID: f.ID, ContactID: item.ID, Content: f.Content, CreatedAt: f.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"), CreatedBy: f.CreatedBy})
	}
	tags, weComFriends, opportunities := item.Tags, item.WeComFriends, item.Opportunities
	if tags == nil {
		tags = []ports.ContactTagSummary{}
	}
	if weComFriends == nil {
		weComFriends = []ports.WeComFriendSummary{}
	}
	if opportunities == nil {
		opportunities = []ports.ContactOpportunitySummary{}
	}
	return contactDetailJSON{contactSummaryJSON: contactSummaryView(item.ContactSummary), Assignment: assignmentViewJSON(item.Assignment), Tags: tags, WeComFriends: weComFriends, WeComFriendsAvailable: item.WeComFriendsAvailable, Opportunities: opportunities, FollowUps: followUps}
}

func assignmentViewJSON(item domain.CustomerAssignment) assignmentJSON {
	lastFollowUpAt := ""
	if item.LastFollowUpAt != nil {
		lastFollowUpAt = item.LastFollowUpAt.UTC().Format(time.RFC3339)
	}
	return assignmentJSON{ID: item.ID, ContactID: item.ContactID, OwnerID: item.OwnerID, CollaboratorIDs: item.CollaboratorIDs, Status: item.Status, Version: item.Version, ContactName: item.ContactName, Source: item.Source, BusinessType: item.BusinessType, TagNames: item.TagNames, Region: item.Region, RecycleCount: item.RecycleCount, PoolAction: item.PoolAction, PoolReason: item.PoolReason, PreviousOwnerID: item.PreviousOwnerID, LastFollowUpAt: lastFollowUpAt}
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
