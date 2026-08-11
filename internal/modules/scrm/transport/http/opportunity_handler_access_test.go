package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestOpportunityHandlerAuthorizesPageAndTagOperationsInRequestedCorp(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		url        string
		body       string
		invoke     func(*OpportunityHandler, http.ResponseWriter, *http.Request)
		permission string
	}{
		{name: "list opportunities", method: http.MethodGet, url: OpportunitiesPath + "?corpId=22", invoke: (*OpportunityHandler).List, permission: "/customer/opportunity#get"},
		{name: "create opportunity", method: http.MethodPost, url: OpportunitiesPath, body: `{"corpId":22,"contactId":"c1","stage":"proposal","amount":1,"startDate":"2026-08-01","endDate":"2026-08-02"}`, invoke: (*OpportunityHandler).Create, permission: "/customer/opportunity@edit#post"},
		{name: "change stage", method: http.MethodPost, url: OpportunitiesPath + "/o1/stage", body: `{"corpId":22,"stageId":"lost","lostReason":"预算取消","version":1}`, invoke: (*OpportunityHandler).Stage, permission: "/customer/opportunity@edit#post"},
		{name: "list tags", method: http.MethodGet, url: TagsPath + "?corpId=22", invoke: (*OpportunityHandler).ListTags, permission: "/customer/tags#get"},
		{name: "create tag", method: http.MethodPost, url: TagsPath, body: `{"corpId":22,"name":"VIP"}`, invoke: (*OpportunityHandler).CreateTag, permission: "/customer/tags@add#post"},
		{name: "rename tag", method: http.MethodPut, url: TagsPath + "/t1", body: `{"corpId":22,"name":"Key","version":1}`, invoke: (*OpportunityHandler).RenameTag, permission: "/customer/tags@edit#post"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			authorizer := &contactAuthorizerFake{}
			service := &opportunityServiceFake{}
			handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}}, authorizer)
			req := httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "key-1")
			res := httptest.NewRecorder()
			tc.invoke(handler, res, req)
			if res.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
			}
			if authorizer.corpID != 22 || authorizer.permission != tc.permission {
				t.Fatalf("authorization=%#v", authorizer)
			}
			if tc.name == "change stage" && (service.stageCommand.CorpID != 22 || service.stageCommand.StageID != "lost" || service.stageCommand.LostReason != "预算取消" || service.stageCommand.IdempotencyKey != "key-1") {
				t.Fatalf("stage command=%#v", service.stageCommand)
			}
		})
	}
}

func TestOpportunityHandlerRejectsSameTenantSecondCorpBeforeRepository(t *testing.T) {
	tests := []struct {
		method string
		url    string
		body   string
		invoke func(*OpportunityHandler, http.ResponseWriter, *http.Request)
	}{
		{http.MethodGet, OpportunitiesPath + "?corpId=22", "", (*OpportunityHandler).List},
		{http.MethodPost, OpportunitiesPath, `{"corpId":22,"contactId":"c1","stage":"proposal","amount":1,"startDate":"2026-08-01","endDate":"2026-08-02"}`, (*OpportunityHandler).Create},
		{http.MethodPost, OpportunitiesPath + "/o1/stage", `{"corpId":22,"stageId":"won","version":1}`, (*OpportunityHandler).Stage},
		{http.MethodGet, TagsPath + "?corpId=22", "", (*OpportunityHandler).ListTags},
		{http.MethodPost, TagsPath, `{"corpId":22,"name":"VIP"}`, (*OpportunityHandler).CreateTag},
		{http.MethodPut, TagsPath + "/t1", `{"corpId":22,"name":"Key","version":1}`, (*OpportunityHandler).RenameTag},
	}
	for _, tc := range tests {
		service := &opportunityServiceFake{}
		handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}}, &contactAuthorizerFake{err: ErrLeadForbidden})
		req := httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		tc.invoke(handler, res, req)
		if res.Code != http.StatusForbidden {
			t.Fatalf("%s %s status=%d body=%s", tc.method, tc.url, res.Code, res.Body.String())
		}
		if service.calls != 0 {
			t.Fatalf("%s %s service called %d times", tc.method, tc.url, service.calls)
		}
	}
}

type opportunityServiceFake struct {
	calls         int
	filter        ports.OpportunityFilter
	stageCommand  ports.ChangeOpportunityStageCommand
	followCommand ports.AppendFollowUpCommand
	err           error
}

func (s *opportunityServiceFake) ListOpportunities(_ context.Context, filter ports.OpportunityFilter) (ports.OpportunityPage, error) {
	s.calls++
	s.filter = filter
	return ports.OpportunityPage{}, s.err
}

func TestOpportunityHandlerParsesCombinedFiltersAndMapsMutationErrors(t *testing.T) {
	service := &opportunityServiceFake{}
	handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}}, &contactAuthorizerFake{})
	req := httptest.NewRequest(http.MethodGet, OpportunitiesPath+"?corpId=22&stage=proposal&status=open&ownerId=9&cursor=o9&pageSize=25", nil)
	res := httptest.NewRecorder()
	handler.List(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if service.filter.Stage != "proposal" || service.filter.Status != "open" || service.filter.OwnerID == nil || *service.filter.OwnerID != 9 || service.filter.Cursor != "o9" || service.filter.PageSize != 25 {
		t.Fatalf("filter=%#v", service.filter)
	}

	for _, test := range []struct {
		err  error
		want int
	}{
		{ports.ErrAssignmentConflict, http.StatusConflict},
		{ports.ErrInvalidOpportunityTransition, http.StatusUnprocessableEntity},
		{ports.ErrOpportunityNotFound, http.StatusNotFound},
		{ports.ErrAssignmentForbidden, http.StatusForbidden},
	} {
		service.err = test.err
		request := httptest.NewRequest(http.MethodGet, OpportunitiesPath+"?corpId=22", nil)
		response := httptest.NewRecorder()
		handler.List(response, request)
		if response.Code != test.want {
			t.Fatalf("error=%v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
	}
}

func TestOpportunityFollowUpAllowsOpportunityEditWhenContactEditIsUnavailable(t *testing.T) {
	service := &opportunityServiceFake{}
	authorizer := &opportunityFollowUpAuthorizer{}
	handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}}, authorizer)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/scrm/contacts/c1/follow-ups", strings.NewReader(`{"corpId":22,"content":"已发送方案"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "follow-1")
	res := httptest.NewRecorder()
	handler.AppendFollowUp(res, req)
	if res.Code != http.StatusOK || service.calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", res.Code, service.calls, res.Body.String())
	}
	if len(authorizer.permissions) != 2 || authorizer.permissions[0] != contactPermissionEdit || authorizer.permissions[1] != opportunityPermissionEdit {
		t.Fatalf("permissions=%v", authorizer.permissions)
	}
}

type opportunityFollowUpAuthorizer struct{ permissions []string }

func (a *opportunityFollowUpAuthorizer) Authorize(_ context.Context, _ Principal, _ int64, permission string) error {
	a.permissions = append(a.permissions, permission)
	if permission == contactPermissionEdit {
		return ErrLeadForbidden
	}
	return nil
}
func (s *opportunityServiceFake) CreateOpportunity(context.Context, ports.CreateOpportunityCommand) (ports.Opportunity, error) {
	s.calls++
	return ports.Opportunity{}, nil
}
func (s *opportunityServiceFake) ChangeOpportunityStage(_ context.Context, command ports.ChangeOpportunityStageCommand) (ports.Opportunity, error) {
	s.calls++
	s.stageCommand = command
	return ports.Opportunity{ID: command.OpportunityID}, nil
}
func (s *opportunityServiceFake) ListFollowUps(context.Context, int64, int64, string) ([]ports.FollowUpRecord, error) {
	s.calls++
	return nil, nil
}
func (s *opportunityServiceFake) AppendFollowUp(_ context.Context, command ports.AppendFollowUpCommand) (ports.FollowUpRecord, error) {
	s.calls++
	s.followCommand = command
	return ports.FollowUpRecord{}, nil
}
func (s *opportunityServiceFake) ListTags(context.Context, int64, int64) ([]ports.Tag, error) {
	s.calls++
	return nil, nil
}
func (s *opportunityServiceFake) CreateTag(_ context.Context, tenant, corp int64, name, key string) (ports.Tag, error) {
	s.calls++
	return ports.Tag{TenantID: tenant, CorpID: corp, Name: name}, nil
}
func (s *opportunityServiceFake) RenameTag(_ context.Context, tenant, corp int64, id, name string, version int64, key string) (ports.Tag, error) {
	s.calls++
	return ports.Tag{ID: id, TenantID: tenant, CorpID: corp, Name: name, Version: version + 1}, nil
}
func (s *opportunityServiceFake) BindTags(context.Context, int64, int64, string, []string, string) error {
	s.calls++
	return nil
}
