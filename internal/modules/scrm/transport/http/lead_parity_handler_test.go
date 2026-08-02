package http

import (
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/domain"
)

func TestLeadListParsesCombinedCorpFilterAndAuthorizes(t *testing.T) {
	service := &parityLeadService{}
	authorizer := &fakeLeadAuthorizer{}
	handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41}}, authorizer)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(nethttp.MethodGet, FormalLeadsPath+"?corpId=8&keyword=Ada&status=new&status=qualified&source=manual&ownerId=12&createdFrom=2026-08-01T00:00:00Z&createdTo=2026-08-02T00:00:00Z&pageSize=30", nil)
	handler.List(response, request)
	if response.Code != nethttp.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
	q := service.listQuery
	if q.TenantID != 41 || q.CorpID != 8 || q.Keyword != "Ada" || len(q.Statuses) != 2 || len(q.Sources) != 1 || len(q.OwnerIDs) != 1 || q.PageSize != 30 {
		t.Fatalf("query=%#v", q)
	}
	if authorizer.permission != leadPermissionView || authorizer.corpID != 8 {
		t.Fatalf("authorization=%#v", authorizer)
	}
}

func TestLeadHandlersReturn403BeforeServiceWhenRBACDenies(t *testing.T) {
	service := &parityLeadService{}
	handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41}}, &fakeLeadAuthorizer{err: ErrLeadForbidden})
	response := httptest.NewRecorder()
	handler.List(response, httptest.NewRequest(nethttp.MethodGet, FormalLeadsPath+"?corpId=8", nil))
	if response.Code != nethttp.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
	if service.listCalls != 0 {
		t.Fatalf("service calls=%d", service.listCalls)
	}
}

func TestLeadBatchAssignmentReturnsPerTargetPartialFailure(t *testing.T) {
	service := &parityLeadService{assignResults: []application.LeadMutationResult{{ID: "ok", Status: "succeeded"}, {ID: "stale", Status: "failed", ErrorCode: "CONFLICT"}}}
	handler := NewLeadHandler(service, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41}}, &fakeLeadAuthorizer{})
	response := httptest.NewRecorder()
	handler.Assign(response, httptest.NewRequest(nethttp.MethodPost, LeadAssignmentsPath, strings.NewReader(`{"corpId":8,"ownerId":12,"targets":[{"id":"ok","version":1},{"id":"stale","version":2}]}`)))
	if response.Code != nethttp.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
	var envelope struct {
		Data struct {
			Results []application.LeadMutationResult `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Results) != 2 || envelope.Data.Results[1].ErrorCode != "CONFLICT" {
		t.Fatalf("results=%#v", envelope.Data.Results)
	}
}

func TestLeadTransitionMapsConflictAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{{"conflict", application.ErrConflict, nethttp.StatusConflict}, {"validation", application.ErrInvalidArgument, nethttp.StatusUnprocessableEntity}} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewLeadHandler(&parityLeadService{transitionErr: tc.err}, fakePrincipalResolver{principal: Principal{UserID: 7, TenantID: 41}}, &fakeLeadAuthorizer{})
			response := httptest.NewRecorder()
			handler.Transition(response, httptest.NewRequest(nethttp.MethodPost, LeadTransitionPath, strings.NewReader(`{"corpId":8,"id":"lead-1","toStatus":"qualified","version":1}`)))
			if response.Code != tc.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}

type fakeLeadAuthorizer struct {
	corpID     int64
	permission string
	err        error
}

func (a *fakeLeadAuthorizer) Authorize(_ context.Context, _ Principal, corpID int64, permission string) error {
	a.corpID, a.permission = corpID, permission
	return a.err
}

type parityLeadService struct {
	listQuery     application.ListLeadsQuery
	listCalls     int
	assignResults []application.LeadMutationResult
	transitionErr error
}

func (s *parityLeadService) CreateLead(context.Context, application.CreateLeadCommand) (application.CreateLeadResult, error) {
	return application.CreateLeadResult{}, nil
}
func (s *parityLeadService) ListLeads(_ context.Context, q application.ListLeadsQuery) (application.LeadPage, error) {
	s.listCalls++
	s.listQuery = q
	return application.LeadPage{}, nil
}
func (s *parityLeadService) AssignLeads(context.Context, application.AssignLeadsCommand) ([]application.LeadMutationResult, error) {
	return s.assignResults, nil
}
func (s *parityLeadService) TransitionLead(context.Context, application.TransitionLeadCommand) (application.LeadView, error) {
	return application.LeadView{}, s.transitionErr
}
func (s *parityLeadService) FindDuplicateLeads(context.Context, int64, int64, string, string) ([]application.LeadView, error) {
	return nil, nil
}

var _ = errors.New
var _ = domain.LeadStatusNew
