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
		{name: "change stage", method: http.MethodPost, url: OpportunitiesPath + "/o1/stage", body: `{"corpId":22,"toStage":"won","version":1}`, invoke: (*OpportunityHandler).Stage, permission: "/customer/opportunity@edit#post"},
		{name: "list tags", method: http.MethodGet, url: TagsPath + "?corpId=22", invoke: (*OpportunityHandler).ListTags, permission: "/customer/tags#get"},
		{name: "create tag", method: http.MethodPost, url: TagsPath, body: `{"corpId":22,"name":"VIP"}`, invoke: (*OpportunityHandler).CreateTag, permission: "/customer/tags@add#post"},
		{name: "rename tag", method: http.MethodPut, url: TagsPath + "/t1", body: `{"corpId":22,"name":"Key","version":1}`, invoke: (*OpportunityHandler).RenameTag, permission: "/customer/tags@edit#post"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			authorizer := &contactAuthorizerFake{}
			service := &opportunityServiceFake{}
			handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11}}, authorizer)
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
			if tc.name == "change stage" && service.stageCommand.CorpID != 22 {
				t.Fatalf("stage command corp=%d", service.stageCommand.CorpID)
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
		{http.MethodGet, OpportunitiesPath + "?corpId=23", "", (*OpportunityHandler).List},
		{http.MethodPost, OpportunitiesPath + "/o1/stage", `{"corpId":23,"toStage":"won","version":1}`, (*OpportunityHandler).Stage},
		{http.MethodGet, TagsPath + "?corpId=23", "", (*OpportunityHandler).ListTags},
		{http.MethodPost, TagsPath, `{"corpId":23,"name":"VIP"}`, (*OpportunityHandler).CreateTag},
		{http.MethodPut, TagsPath + "/t1", `{"corpId":23,"name":"Key","version":1}`, (*OpportunityHandler).RenameTag},
	}
	for _, tc := range tests {
		service := &opportunityServiceFake{}
		handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11}}, &contactAuthorizerFake{err: ErrLeadForbidden})
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
	calls        int
	stageCommand ports.ChangeOpportunityStageCommand
}

func (s *opportunityServiceFake) ListOpportunities(context.Context, ports.OpportunityFilter) ([]ports.Opportunity, error) {
	s.calls++
	return nil, nil
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
func (s *opportunityServiceFake) AppendFollowUp(context.Context, ports.AppendFollowUpCommand) (ports.FollowUpRecord, error) {
	s.calls++
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
