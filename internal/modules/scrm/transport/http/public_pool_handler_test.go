package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestPublicPoolHandlerParsesCombinedFiltersAndAuthorizesView(t *testing.T) {
	service := &publicPoolServiceFake{page: ports.AssignmentPage{Items: []domain.CustomerAssignment{{ContactID: "c1", ContactName: "Ada", PoolReason: "expired", PreviousOwnerID: int64Ptr(18)}}}}
	authorizer := &contactAuthorizerFake{}
	handler := NewCustomerLifecycleHandler(service, fakePrincipalResolver{principal: Principal{UserID: 42, TenantID: 7}}, authorizer)
	req := httptest.NewRequest(http.MethodGet, AssignmentsPath+"?corpId=9&keyword=Ada&source=wecom&businessType=retail&tagId=tag-1&region=Shanghai&reason=expired&previousOwnerId=18&cursor=20&pageSize=25", nil)
	res := httptest.NewRecorder()
	handler.ListPublicPool(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	q := service.query
	if authorizer.permission != contactPermissionView || authorizer.corpID != 9 || q.TenantID != 7 || q.CorpID != 9 || q.Keyword != "Ada" || q.Cursor != "20" || q.PageSize != 25 || len(q.Sources) != 1 || len(q.BusinessTypes) != 1 || len(q.TagIDs) != 1 || len(q.Regions) != 1 || len(q.Reasons) != 1 || len(q.PreviousOwnerIDs) != 1 {
		t.Fatalf("auth=%#v query=%#v", authorizer, q)
	}
}

func TestPublicPoolHandlerRequiresClaimUserToMatchPrincipalAndPassesCompleteCommand(t *testing.T) {
	service := &publicPoolServiceFake{}
	handler := NewCustomerLifecycleHandler(service, fakePrincipalResolver{principal: Principal{UserID: 42, TenantID: 7}}, &contactAuthorizerFake{})
	res := httptest.NewRecorder()
	handler.ClaimFromPublicPool(res, httptest.NewRequest(http.MethodPost, AssignmentClaimPath, strings.NewReader(`{"corpId":9,"contactId":"c1","userId":77,"version":3}`)))
	if res.Code != http.StatusForbidden || service.claim.ContactID != "" {
		t.Fatalf("mismatched user status=%d command=%#v", res.Code, service.claim)
	}

	req := httptest.NewRequest(http.MethodPost, AssignmentClaimPath, strings.NewReader(`{"corpId":9,"contactId":"c1","userId":42,"version":3}`))
	req.Header.Set("Idempotency-Key", "claim-1")
	res = httptest.NewRecorder()
	handler.ClaimFromPublicPool(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	want := ports.ClaimPublicPoolCommand{TenantID: 7, CorpID: 9, ContactID: "c1", UserID: 42, Version: 3, IdempotencyKey: "claim-1"}
	if service.claim != want {
		t.Fatalf("claim=%#v want=%#v", service.claim, want)
	}
}

func TestPublicPoolHandlerReturnsPerItemBatchResults(t *testing.T) {
	service := &publicPoolServiceFake{batch: []application.PublicPoolMutationResult{{ID: "c1", Status: "succeeded"}, {ID: "c2", Status: "failed", ErrorCode: "CONFLICT"}}}
	handler := NewCustomerLifecycleHandler(service, fakePrincipalResolver{principal: Principal{UserID: 42, TenantID: 7}}, &contactAuthorizerFake{})
	req := httptest.NewRequest(http.MethodPost, AssignmentBatchClaimPath, strings.NewReader(`{"corpId":9,"userId":42,"targets":[{"contactId":"c1","version":1,"idempotencyKey":"batch-1"},{"contactId":"c2","version":2,"idempotencyKey":"batch-2"}]}`))
	res := httptest.NewRecorder()
	handler.BatchClaimFromPublicPool(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"errorCode":"CONFLICT"`) || len(service.batchCommand.Targets) != 2 {
		t.Fatalf("status=%d body=%s command=%#v", res.Code, res.Body.String(), service.batchCommand)
	}
}

func TestPublicPoolHandlerPersistsMoveActionReasonAndActor(t *testing.T) {
	service := &publicPoolServiceFake{}
	handler := NewCustomerLifecycleHandler(service, fakePrincipalResolver{principal: Principal{UserID: 42, TenantID: 7}}, &contactAuthorizerFake{})
	req := httptest.NewRequest(http.MethodPost, AssignmentReleasePath, strings.NewReader(`{"corpId":9,"contactId":"c1","version":4,"action":"reclaim","reason":"超时未跟进"}`))
	req.Header.Set("Idempotency-Key", "reclaim-1")
	res := httptest.NewRecorder()
	handler.ReleaseToPublicPool(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	want := ports.MoveToPublicPoolCommand{TenantID: 7, CorpID: 9, ContactID: "c1", ActorID: 42, Version: 4, Action: domain.PublicPoolActionReclaim, Reason: "超时未跟进", IdempotencyKey: "reclaim-1"}
	if service.move != want {
		t.Fatalf("move=%#v want=%#v", service.move, want)
	}
}

type publicPoolServiceFake struct {
	query        application.ListPublicPoolQuery
	page         ports.AssignmentPage
	claim        ports.ClaimPublicPoolCommand
	move         ports.MoveToPublicPoolCommand
	batchCommand application.BatchClaimPublicPoolCommand
	batch        []application.PublicPoolMutationResult
}

func (s *publicPoolServiceFake) ListContacts(context.Context, application.ListContactsQuery) (ports.ContactPage, error) {
	return ports.ContactPage{}, nil
}
func (s *publicPoolServiceFake) GetContact(context.Context, int64, int64, string) (ports.ContactDetail, error) {
	return ports.ContactDetail{}, nil
}
func (s *publicPoolServiceFake) ListPublicPool(_ context.Context, query application.ListPublicPoolQuery) (ports.AssignmentPage, error) {
	s.query = query
	return s.page, nil
}
func (s *publicPoolServiceFake) UpdateAssignment(context.Context, ports.UpdateAssignmentCommand) (domain.CustomerAssignment, error) {
	return domain.CustomerAssignment{}, nil
}
func (s *publicPoolServiceFake) MoveToPublicPool(_ context.Context, command ports.MoveToPublicPoolCommand) (domain.CustomerAssignment, error) {
	s.move = command
	return domain.CustomerAssignment{ContactID: command.ContactID, Version: command.Version + 1}, nil
}
func (s *publicPoolServiceFake) ClaimFromPublicPool(_ context.Context, command ports.ClaimPublicPoolCommand) (domain.CustomerAssignment, error) {
	s.claim = command
	return domain.CustomerAssignment{ContactID: command.ContactID, OwnerID: &command.UserID, Version: command.Version + 1}, nil
}
func (s *publicPoolServiceFake) BatchClaimFromPublicPool(_ context.Context, command application.BatchClaimPublicPoolCommand) ([]application.PublicPoolMutationResult, error) {
	s.batchCommand = command
	return s.batch, nil
}

func int64Ptr(value int64) *int64 { return &value }
