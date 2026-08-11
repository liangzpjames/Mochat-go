package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpportunityHandlerAcceptsFrontendStagePayloads(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStage  string
		wantReason string
	}{
		{name: "stage", body: `{"corpId":22,"stageId":"proposal","version":2,"lostReason":""}`, wantStage: "proposal"},
		{name: "won", body: `{"corpId":22,"stageId":"won","version":2,"lostReason":""}`, wantStage: "won"},
		{name: "lost", body: `{"corpId":22,"stageId":"lost","version":2,"lostReason":"budget cancelled"}`, wantStage: "lost", wantReason: "budget cancelled"},
		{name: "matching body fields", body: `{"corpId":22,"opportunityId":"o1","stageId":"won","version":2,"lostReason":"","idempotencyKey":"stage-1"}`, wantStage: "won"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &opportunityServiceFake{}
			handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}})
			req := httptest.NewRequest(http.MethodPost, OpportunitiesPath+"/o1/stage", strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "stage-1")
			res := httptest.NewRecorder()

			handler.Stage(res, req)

			if res.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
			}
			command := service.stageCommand
			if command.TenantID != 11 || command.CorpID != 22 || command.OpportunityID != "o1" || command.StageID != test.wantStage || command.Version != 2 || command.LostReason != test.wantReason || command.IdempotencyKey != "stage-1" {
				t.Fatalf("stage command=%#v", command)
			}
		})
	}
}

func TestOpportunityHandlerAcceptsFrontendFollowUpPayload(t *testing.T) {
	service := &opportunityServiceFake{}
	handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/scrm/contacts/c1/follow-ups", strings.NewReader(`{"corpId":22,"content":"sent proposal"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "follow-1")
	res := httptest.NewRecorder()

	handler.AppendFollowUp(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	command := service.followCommand
	if command.TenantID != 11 || command.CorpID != 22 || command.ContactID != "c1" || command.Content != "sent proposal" || command.CreatedBy != 3 || command.IdempotencyKey != "follow-1" {
		t.Fatalf("follow-up command=%#v", command)
	}
}

func TestOpportunityHandlerAcceptsFollowUpBodyIdempotencyKey(t *testing.T) {
	service := &opportunityServiceFake{}
	handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/scrm/contacts/c1/follow-ups", strings.NewReader(`{"corpId":22,"content":"sent proposal","idempotencyKey":"follow-body-1"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	handler.AppendFollowUp(res, req)

	if res.Code != http.StatusOK || service.followCommand.IdempotencyKey != "follow-body-1" {
		t.Fatalf("status=%d command=%#v body=%s", res.Code, service.followCommand, res.Body.String())
	}
}

func TestOpportunityHandlerRejectsMismatchedPathAndBodyOpportunityID(t *testing.T) {
	service := &opportunityServiceFake{}
	handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}})
	req := httptest.NewRequest(http.MethodPost, OpportunitiesPath+"/o1/stage", strings.NewReader(`{"corpId":22,"opportunityId":"o2","stageId":"won","version":2,"idempotencyKey":"stage-1"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	handler.Stage(res, req)

	if res.Code != http.StatusUnprocessableEntity || service.calls != 0 || res.Body.Len() == 0 {
		t.Fatalf("status=%d calls=%d body=%s", res.Code, service.calls, res.Body.String())
	}
}

func TestOpportunityHandlerRejectsMissingOrMismatchedIdempotencyKey(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		header string
	}{
		{name: "missing", body: `{"corpId":22,"stageId":"won","version":2}`},
		{name: "mismatch", body: `{"corpId":22,"stageId":"won","version":2,"idempotencyKey":"body-key"}`, header: "header-key"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &opportunityServiceFake{}
			handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}})
			req := httptest.NewRequest(http.MethodPost, OpportunitiesPath+"/o1/stage", strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			if test.header != "" {
				req.Header.Set("Idempotency-Key", test.header)
			}
			res := httptest.NewRecorder()

			handler.Stage(res, req)

			if res.Code != http.StatusUnprocessableEntity || service.calls != 0 || res.Body.Len() == 0 {
				t.Fatalf("status=%d calls=%d body=%s", res.Code, service.calls, res.Body.String())
			}
		})
	}
}

func TestOpportunityHandlerWritesErrorsForUnknownJSONFields(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		body   string
		invoke func(*OpportunityHandler, http.ResponseWriter, *http.Request)
	}{
		{name: "stage", url: OpportunitiesPath + "/o1/stage", body: `{"corpId":22,"stageId":"won","version":2,"unexpected":true}`, invoke: (*OpportunityHandler).Stage},
		{name: "follow-up contact id belongs to path", url: "/dashboard/scrm/contacts/c1/follow-ups", body: `{"corpId":22,"contactId":"c1","content":"sent proposal"}`, invoke: (*OpportunityHandler).AppendFollowUp},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &opportunityServiceFake{}
			handler := NewOpportunityHandler(service, fakePrincipalResolver{principal: Principal{UserID: 3, TenantID: 11, CorpID: 22}})
			req := httptest.NewRequest(http.MethodPost, test.url, strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "mutation-1")
			res := httptest.NewRecorder()

			test.invoke(handler, res, req)

			if res.Code != http.StatusBadRequest || service.calls != 0 || res.Body.Len() == 0 {
				t.Fatalf("status=%d calls=%d body=%s", res.Code, service.calls, res.Body.String())
			}
		})
	}
}
