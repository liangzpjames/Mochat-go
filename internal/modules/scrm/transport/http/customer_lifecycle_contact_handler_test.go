package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestContactHandlerAuthorizesAndParsesCombinedFilter(t *testing.T) {
	service := &contactLifecycleServiceFake{page: ports.ContactPage{Items: []ports.ContactSummary{{ID: "c1", Name: "Ada"}}}}
	authorizer := &contactAuthorizerFake{}
	handler := NewCustomerLifecycleHandler(service, fakePrincipalResolver{principal: Principal{UserID: 5, TenantID: 7}}, authorizer)
	req := httptest.NewRequest(http.MethodGet, ContactsPath+"?corpId=9&keyword=Ada&ownerId=11&tagId=tag-1&status=owned&pageSize=30", nil)
	res := httptest.NewRecorder()
	handler.ListContacts(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if authorizer.permission != contactPermissionView || authorizer.corpID != 9 {
		t.Fatalf("auth=%#v", authorizer)
	}
	if service.query.TenantID != 7 || service.query.CorpID != 9 || service.query.Keyword != "Ada" || len(service.query.OwnerIDs) != 1 || len(service.query.TagIDs) != 1 || len(service.query.Statuses) != 1 || service.query.PageSize != 30 {
		t.Fatalf("query=%#v", service.query)
	}
}

func TestContactHandlerHidesSecondCorpAndMapsNotFound(t *testing.T) {
	service := &contactLifecycleServiceFake{detailErr: application.ErrNotFound}
	handler := NewCustomerLifecycleHandler(service, fakePrincipalResolver{principal: Principal{UserID: 5, TenantID: 7}}, &contactAuthorizerFake{err: ErrLeadForbidden})
	res := httptest.NewRecorder()
	handler.GetContact(res, httptest.NewRequest(http.MethodGet, ContactsPath+"/c1?corpId=10", nil))
	if res.Code != http.StatusForbidden {
		t.Fatalf("forbidden status=%d", res.Code)
	}

	handler = NewCustomerLifecycleHandler(service, fakePrincipalResolver{principal: Principal{UserID: 5, TenantID: 7}}, &contactAuthorizerFake{})
	res = httptest.NewRecorder()
	handler.GetContact(res, httptest.NewRequest(http.MethodGet, ContactsPath+"/c1?corpId=9", nil))
	if res.Code != http.StatusNotFound {
		t.Fatalf("not found status=%d body=%s", res.Code, res.Body.String())
	}
}

type contactLifecycleServiceFake struct {
	query     application.ListContactsQuery
	page      ports.ContactPage
	detail    ports.ContactDetail
	detailErr error
}

func (s *contactLifecycleServiceFake) ListContacts(_ context.Context, q application.ListContactsQuery) (ports.ContactPage, error) {
	s.query = q
	return s.page, nil
}
func (s *contactLifecycleServiceFake) GetContact(context.Context, int64, int64, string) (ports.ContactDetail, error) {
	return s.detail, s.detailErr
}
func (s *contactLifecycleServiceFake) ListPublicPool(context.Context, application.ListPublicPoolQuery) (ports.AssignmentPage, error) {
	return ports.AssignmentPage{}, nil
}
func (s *contactLifecycleServiceFake) UpdateAssignment(context.Context, ports.UpdateAssignmentCommand) (domain.CustomerAssignment, error) {
	return domain.CustomerAssignment{}, nil
}
func (s *contactLifecycleServiceFake) ReleaseToPublicPool(context.Context, int64, int64, string, int64, string) (domain.CustomerAssignment, error) {
	return domain.CustomerAssignment{}, nil
}
func (s *contactLifecycleServiceFake) ClaimFromPublicPool(context.Context, int64, int64, string, int64, int64, string) (domain.CustomerAssignment, error) {
	return domain.CustomerAssignment{}, nil
}

type contactAuthorizerFake struct {
	corpID     int64
	permission string
	err        error
}

func (a *contactAuthorizerFake) Authorize(_ context.Context, _ Principal, corpID int64, permission string) error {
	a.corpID = corpID
	a.permission = permission
	return a.err
}

func decodeContactResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

var _ = errors.Is
