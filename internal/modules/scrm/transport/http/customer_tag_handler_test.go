package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestCustomerTagHandlerUsesScopedRBACAndCompletePayloads(t *testing.T) {
	tests := []struct {
		name, method, url, body, permission string
		invoke                              func(*CustomerTagHandler, http.ResponseWriter, *http.Request)
	}{
		{"catalog", http.MethodGet, TagsPath + "?corpId=22&groupId=g1&keyword=VIP", "", tagPermissionView, (*CustomerTagHandler).ListCatalog},
		{"create group", http.MethodPost, TagGroupsPath, `{"corpId":22,"name":"客户等级"}`, tagPermissionAdd, (*CustomerTagHandler).CreateGroup},
		{"rename group", http.MethodPut, TagGroupsPath + "/g1", `{"corpId":22,"name":"客户分层","version":1}`, tagPermissionEdit, (*CustomerTagHandler).RenameGroup},
		{"create tag", http.MethodPost, TagsPath, `{"corpId":22,"groupId":"g1","name":"VIP"}`, tagPermissionAdd, (*CustomerTagHandler).CreateTag},
		{"rename tag", http.MethodPut, TagsPath + "/t1", `{"corpId":22,"name":"重点","version":2}`, tagPermissionEdit, (*CustomerTagHandler).RenameTag},
		{"move tag", http.MethodPost, TagsPath + "/t1/move", `{"corpId":22,"groupId":"g2","version":2}`, tagPermissionEdit, (*CustomerTagHandler).MoveTag},
		{"maintain contacts", http.MethodPut, TagsPath + "/t1/contacts", `{"corpId":22,"addContactIds":["c1"],"removeContactIds":["c2"],"version":2}`, tagPermissionEdit, (*CustomerTagHandler).MaintainContacts},
		{"delete tag", http.MethodDelete, TagsPath + "/t1", `{"corpId":22,"version":2}`, tagPermissionDelete, (*CustomerTagHandler).DeleteTag},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &customerTagServiceFake{}
			authorizer := &contactAuthorizerFake{}
			handler := NewCustomerTagHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11}}, authorizer)
			request := httptest.NewRequest(test.method, test.url, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "request-1")
			response := httptest.NewRecorder()
			test.invoke(handler, response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if authorizer.corpID != 22 || authorizer.permission != test.permission {
				t.Fatalf("authorization=%#v", authorizer)
			}
			if test.name == "catalog" && (service.filter.GroupID != "g1" || service.filter.Keyword != "VIP") {
				t.Fatalf("filter=%#v", service.filter)
			}
			if test.name != "catalog" && service.key != "request-1" {
				t.Fatalf("idempotency key=%q", service.key)
			}
		})
	}
}

func TestCustomerTagHandlerMapsForbiddenNotFoundConflictAndDuplicate(t *testing.T) {
	service := &customerTagServiceFake{}
	for _, test := range []struct {
		err  error
		want int
	}{
		{ports.ErrTagNotFound, http.StatusNotFound},
		{ports.ErrAssignmentConflict, http.StatusConflict},
		{ports.ErrDuplicateTagName, http.StatusUnprocessableEntity},
	} {
		service.err = test.err
		handler := NewCustomerTagHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11}}, &contactAuthorizerFake{})
		request := httptest.NewRequest(http.MethodGet, TagsPath+"?corpId=22", nil)
		response := httptest.NewRecorder()
		handler.ListCatalog(response, request)
		if response.Code != test.want {
			t.Fatalf("err=%v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
	}

	handler := NewCustomerTagHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11}}, &contactAuthorizerFake{err: ErrLeadForbidden})
	request := httptest.NewRequest(http.MethodGet, TagsPath+"?corpId=23", nil)
	response := httptest.NewRecorder()
	handler.ListCatalog(response, request)
	if response.Code != http.StatusForbidden || service.calls != 3 {
		t.Fatalf("forbidden status=%d calls=%d", response.Code, service.calls)
	}
}

type customerTagServiceFake struct {
	calls  int
	filter ports.ListTagCatalogFilter
	key    string
	err    error
}

func (s *customerTagServiceFake) ListCatalog(_ context.Context, filter ports.ListTagCatalogFilter) (ports.TagCatalog, error) {
	s.calls++
	s.filter = filter
	return ports.TagCatalog{}, s.err
}
func (s *customerTagServiceFake) CreateGroup(_ context.Context, c ports.CreateTagGroupCommand) (ports.TagGroup, error) {
	s.calls++
	s.key = c.IdempotencyKey
	return ports.TagGroup{ID: "g1", Name: c.Name, Version: 1}, s.err
}
func (s *customerTagServiceFake) RenameGroup(_ context.Context, c ports.RenameTagGroupCommand) (ports.TagGroup, error) {
	s.calls++
	s.key = c.IdempotencyKey
	return ports.TagGroup{ID: c.GroupID, Name: c.Name, Version: c.Version + 1}, s.err
}
func (s *customerTagServiceFake) CreateTag(_ context.Context, c ports.CreateCustomerTagCommand) (ports.CustomerTag, error) {
	s.calls++
	s.key = c.IdempotencyKey
	return ports.CustomerTag{ID: "t1", GroupID: c.GroupID, Name: c.Name, Version: 1}, s.err
}
func (s *customerTagServiceFake) RenameTag(_ context.Context, c ports.RenameCustomerTagCommand) (ports.CustomerTag, error) {
	s.calls++
	s.key = c.IdempotencyKey
	return ports.CustomerTag{ID: c.TagID, Name: c.Name, Version: c.Version + 1}, s.err
}
func (s *customerTagServiceFake) MoveTag(_ context.Context, c ports.MoveCustomerTagCommand) (ports.CustomerTag, error) {
	s.calls++
	s.key = c.IdempotencyKey
	return ports.CustomerTag{ID: c.TagID, GroupID: c.GroupID, Version: c.Version + 1}, s.err
}
func (s *customerTagServiceFake) DeleteTag(_ context.Context, c ports.DeleteCustomerTagCommand) (ports.DeleteCustomerTagResult, error) {
	s.calls++
	s.key = c.IdempotencyKey
	return ports.DeleteCustomerTagResult{AffectedResourceCount: 2}, s.err
}
func (s *customerTagServiceFake) MaintainContacts(_ context.Context, c ports.MaintainTagContactsCommand) (ports.CustomerTag, error) {
	s.calls++
	s.key = c.IdempotencyKey
	return ports.CustomerTag{ID: c.TagID, Version: c.Version + 1}, s.err
}
