package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

const TagGroupsPath = "/dashboard/scrm/tag-groups"
const tagPermissionDelete = "/customer/tags@delete#delete"

type CustomerTagService interface {
	ListCatalog(context.Context, ports.ListTagCatalogFilter) (ports.TagCatalog, error)
	CreateGroup(context.Context, ports.CreateTagGroupCommand) (ports.TagGroup, error)
	RenameGroup(context.Context, ports.RenameTagGroupCommand) (ports.TagGroup, error)
	CreateTag(context.Context, ports.CreateCustomerTagCommand) (ports.CustomerTag, error)
	RenameTag(context.Context, ports.RenameCustomerTagCommand) (ports.CustomerTag, error)
	MoveTag(context.Context, ports.MoveCustomerTagCommand) (ports.CustomerTag, error)
	DeleteTag(context.Context, ports.DeleteCustomerTagCommand) (ports.DeleteCustomerTagResult, error)
	MaintainContacts(context.Context, ports.MaintainTagContactsCommand) (ports.CustomerTag, error)
}

type legacyTagBinder interface {
	BindTags(context.Context, int64, int64, string, []string, string) error
}

type CustomerTagHandler struct {
	service    CustomerTagService
	principal  PrincipalResolver
	authorizer LeadAuthorizer
	legacy     legacyTagBinder
}

func NewCustomerTagHandler(service CustomerTagService, principal PrincipalResolver, authorizer LeadAuthorizer, legacy ...legacyTagBinder) *CustomerTagHandler {
	h := &CustomerTagHandler{service: service, principal: principal, authorizer: authorizer}
	if len(legacy) > 0 {
		h.legacy = legacy[0]
	}
	return h
}

type tagGroupJSON struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Version  int64  `json:"version"`
	TagCount int64  `json:"tagCount"`
}
type customerTagJSON struct {
	ID         string `json:"id"`
	GroupID    string `json:"groupId"`
	Name       string `json:"name"`
	Version    int64  `json:"version"`
	UsageCount int64  `json:"usageCount"`
}

func tagGroupView(item ports.TagGroup) tagGroupJSON {
	return tagGroupJSON{ID: item.ID, Name: item.Name, Version: item.Version, TagCount: item.TagCount}
}
func customerTagView(item ports.CustomerTag) customerTagJSON {
	return customerTagJSON{ID: item.ID, GroupID: item.GroupID, Name: item.Name, Version: item.Version, UsageCount: item.UsageCount}
}

func (h *CustomerTagHandler) ListCatalog(w http.ResponseWriter, r *http.Request) {
	p, corpID, ok := h.readScope(w, r, queryInt(r, "corpId"), tagPermissionView)
	if !ok {
		return
	}
	catalog, err := h.service.ListCatalog(r.Context(), ports.ListTagCatalogFilter{TenantID: p.TenantID, CorpID: corpID, GroupID: r.URL.Query().Get("groupId"), Keyword: r.URL.Query().Get("keyword")})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	groups := make([]tagGroupJSON, 0, len(catalog.Groups))
	for _, item := range catalog.Groups {
		groups = append(groups, tagGroupView(item))
	}
	tags := make([]customerTagJSON, 0, len(catalog.Tags))
	for _, item := range catalog.Tags {
		tags = append(tags, customerTagView(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"groups": groups, "tags": tags}})
}

func (h *CustomerTagHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CorpID int64  `json:"corpId"`
		Name   string `json:"name"`
	}
	if decodeCustomerTagJSON(w, r, &body) != nil {
		return
	}
	p, _, ok := h.readScope(w, r, body.CorpID, tagPermissionAdd)
	if !ok {
		return
	}
	item, err := h.service.CreateGroup(r.Context(), ports.CreateTagGroupCommand{TenantID: p.TenantID, CorpID: body.CorpID, Name: body.Name, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tagGroupView(item)})
}

func (h *CustomerTagHandler) RenameGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CorpID, Version int64
		Name            string
	}
	if decodeCustomerTagJSON(w, r, &body) != nil {
		return
	}
	p, _, ok := h.readScope(w, r, body.CorpID, tagPermissionEdit)
	if !ok {
		return
	}
	item, err := h.service.RenameGroup(r.Context(), ports.RenameTagGroupCommand{TenantID: p.TenantID, CorpID: body.CorpID, GroupID: pathValue(r, "tag-groups", ""), Name: body.Name, Version: body.Version, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tagGroupView(item)})
}

func (h *CustomerTagHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CorpID        int64 `json:"corpId"`
		GroupID, Name string
	}
	if decodeCustomerTagJSON(w, r, &body) != nil {
		return
	}
	p, _, ok := h.readScope(w, r, body.CorpID, tagPermissionAdd)
	if !ok {
		return
	}
	item, err := h.service.CreateTag(r.Context(), ports.CreateCustomerTagCommand{TenantID: p.TenantID, CorpID: body.CorpID, GroupID: body.GroupID, Name: body.Name, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": customerTagView(item)})
}

func (h *CustomerTagHandler) RenameTag(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CorpID, Version int64
		Name            string
	}
	if decodeCustomerTagJSON(w, r, &body) != nil {
		return
	}
	p, _, ok := h.readScope(w, r, body.CorpID, tagPermissionEdit)
	if !ok {
		return
	}
	item, err := h.service.RenameTag(r.Context(), ports.RenameCustomerTagCommand{TenantID: p.TenantID, CorpID: body.CorpID, TagID: pathValue(r, "tags", ""), Name: body.Name, Version: body.Version, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": customerTagView(item)})
}

func (h *CustomerTagHandler) MoveTag(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CorpID, Version int64
		GroupID         string
	}
	if decodeCustomerTagJSON(w, r, &body) != nil {
		return
	}
	p, _, ok := h.readScope(w, r, body.CorpID, tagPermissionEdit)
	if !ok {
		return
	}
	item, err := h.service.MoveTag(r.Context(), ports.MoveCustomerTagCommand{TenantID: p.TenantID, CorpID: body.CorpID, TagID: pathValue(r, "tags", "move"), GroupID: body.GroupID, Version: body.Version, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": customerTagView(item)})
}

func (h *CustomerTagHandler) MaintainContacts(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CorpID, Version                 int64
		AddContactIDs, RemoveContactIDs []string
	}
	if decodeCustomerTagJSON(w, r, &body) != nil {
		return
	}
	p, _, ok := h.readScope(w, r, body.CorpID, tagPermissionEdit)
	if !ok {
		return
	}
	item, err := h.service.MaintainContacts(r.Context(), ports.MaintainTagContactsCommand{TenantID: p.TenantID, CorpID: body.CorpID, TagID: pathValue(r, "tags", "contacts"), AddContactIDs: body.AddContactIDs, RemoveContactIDs: body.RemoveContactIDs, Version: body.Version, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": customerTagView(item)})
}

func (h *CustomerTagHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	var body struct{ CorpID, Version int64 }
	if decodeCustomerTagJSON(w, r, &body) != nil {
		return
	}
	p, _, ok := h.readScope(w, r, body.CorpID, tagPermissionDelete)
	if !ok {
		return
	}
	result, err := h.service.DeleteTag(r.Context(), ports.DeleteCustomerTagCommand{TenantID: p.TenantID, CorpID: body.CorpID, TagID: pathValue(r, "tags", ""), Version: body.Version, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *CustomerTagHandler) BindContacts(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CorpID     int64
		ContactIDs []string
	}
	if decodeCustomerTagJSON(w, r, &body) != nil {
		return
	}
	p, _, ok := h.readScope(w, r, body.CorpID, contactPermissionEdit)
	if !ok {
		return
	}
	if h.legacy == nil {
		writeError(w, http.StatusServiceUnavailable, "tag binding unavailable")
		return
	}
	if err := h.legacy.BindTags(r.Context(), p.TenantID, body.CorpID, pathValue(r, "tags", "contacts"), body.ContactIDs, r.Header.Get("Idempotency-Key")); err != nil {
		writeSCRMError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

func (h *CustomerTagHandler) readScope(w http.ResponseWriter, r *http.Request, corpID int64, permission string) (Principal, int64, bool) {
	p, err := h.principal.Resolve(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return Principal{}, 0, false
	}
	if corpID <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "corpId is required")
		return Principal{}, 0, false
	}
	if h.authorizer != nil {
		err := h.authorizer.Authorize(r.Context(), p, corpID, permission)
		if errors.Is(err, ErrLeadForbidden) {
			writeError(w, http.StatusForbidden, "forbidden")
			return Principal{}, 0, false
		}
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "authorization unavailable")
			return Principal{}, 0, false
		}
	}
	return p, corpID, true
}

func decodeCustomerTagJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if err := decodeRequestJSON(w, r, destination); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request JSON")
		return err
	}
	if strings.TrimSpace(r.Header.Get("Idempotency-Key")) == "" {
		writeError(w, http.StatusUnprocessableEntity, "Idempotency-Key is required")
		return application.ErrInvalidArgument
	}
	return nil
}
