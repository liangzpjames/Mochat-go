package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDashboardAdminApprovalExecuteReturnsNestedOneTimeTokenButPersistsOnlyDeliveryMarker(t *testing.T) {
	const oneTimeFixture = "fixture-activation-token"
	store := &fakeSaaSAdminApprovalStore{
		fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
			fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{
				9: {ID: 9, Name: "审批执行人", TenantID: 1, IsSuperAdmin: 1},
			}},
		},
		beginResult: SaaSAdminApproval{
			ID: 81, ActionType: SaaSAdminApprovalActionDashboardActivationResend,
			Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 8, ExecutionUserID: 9,
			RequestJSON: `{"tenantId":41,"targetUserId":52,"expectedVersion":4}`, Version: 6,
		},
	}
	var callbackActor int
	var callbackApprovalID int64
	var callbackVersion int
	var callbackAction string
	var callbackPayload json.RawMessage
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithDashboardAdminApprovalExecutor(func(_ context.Context, actorUserID int, approvalID int64, approvalVersion int, actionType string, payload json.RawMessage) (map[string]any, error) {
		callbackActor, callbackApprovalID, callbackVersion, callbackAction, callbackPayload = actorUserID, approvalID, approvalVersion, actionType, append(json.RawMessage(nil), payload...)
		return map[string]any{"tenantId": 41, "activationToken": oneTimeFixture, "version": 5}, nil
	})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":81,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("one-time response headers=%v, want no-store/no-cache", rec.Header())
	}
	if callbackActor != 9 || callbackApprovalID != 81 || callbackVersion != 6 || callbackAction != SaaSAdminApprovalActionDashboardActivationResend || string(callbackPayload) != store.beginResult.RequestJSON {
		t.Fatalf("callback actor=%d approval=%d version=%d action=%q payload=%s", callbackActor, callbackApprovalID, callbackVersion, callbackAction, callbackPayload)
	}
	if !strings.Contains(rec.Body.String(), oneTimeFixture) {
		t.Fatal("first execution response did not contain the one-time result")
	}
	persistentToken := strings.Contains(store.finishInput.ResultJSON, oneTimeFixture)
	deliveryMarker := strings.Contains(store.finishInput.ResultJSON, `"activationTokenDelivered":true`)
	if persistentToken || !deliveryMarker {
		t.Fatalf("persistent result tokenPresent=%t deliveryMarker=%t", persistentToken, deliveryMarker)
	}
	if !store.finishInput.Success || store.finishCalls != 1 {
		t.Fatalf("finish=%+v calls=%d, want one successful durable completion", store.finishInput, store.finishCalls)
	}
}

func TestDashboardAdminApprovalExecuteRecoveryNeverReplaysToken(t *testing.T) {
	const oneTimeFixture = "fixture-recovered-token"
	store := &fakeSaaSAdminApprovalStore{
		fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
			fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{
				9: {ID: 9, Name: "审批执行人", TenantID: 1, IsSuperAdmin: 1},
			}},
		},
		beginResult: SaaSAdminApproval{
			ID: 81, ActionType: SaaSAdminApprovalActionDashboardActivationResend,
			Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 8, ExecutionUserID: 9,
			RequestJSON: `{"tenantId":41,"targetUserId":52,"expectedVersion":4}`, Version: 6,
			EffectAppliedAtValue: time.Now(), EffectOperationID: 99,
		},
	}
	callbackCalls := 0
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithDashboardAdminApprovalExecutor(func(context.Context, int, int64, int, string, json.RawMessage) (map[string]any, error) {
		callbackCalls++
		return map[string]any{"activationToken": oneTimeFixture}, nil
	})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":81,"expectedVersion":5}`))
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.ApprovalExecute(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "" || rec.Header().Get("Pragma") != "" {
		t.Fatalf("recovery response unexpectedly marked as one-time secret: headers=%v", rec.Header())
	}
	responseToken := strings.Contains(rec.Body.String(), oneTimeFixture)
	persistentToken := strings.Contains(store.finishInput.ResultJSON, oneTimeFixture)
	if callbackCalls != 0 || responseToken || persistentToken {
		t.Fatalf("recovery replay state callbacks=%d responseToken=%t persistentToken=%t", callbackCalls, responseToken, persistentToken)
	}
}

func TestDashboardAdminApprovalRequestOtherActorDecisionAndExecutionCarriesDurableReference(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{
		fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{
			fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{
				7: {ID: 7, Name: "申请人", TenantID: 1, IsSuperAdmin: 1},
				9: {ID: 9, Name: "复核执行人", TenantID: 1, IsSuperAdmin: 1},
			}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithDashboardAdminApprovalExecutor(func(_ context.Context, actorUserID int, approvalID int64, approvalVersion int, actionType string, payload json.RawMessage) (map[string]any, error) {
			if actorUserID != 9 || approvalID != 7 || approvalVersion != 3 || actionType != SaaSAdminApprovalActionDashboardActivationResend {
				t.Fatalf("executor actor=%d approval=%d version=%d action=%q", actorUserID, approvalID, approvalVersion, actionType)
			}
			if !strings.Contains(string(payload), `"targetUserId":52`) || strings.Contains(string(payload), "actor") {
				t.Fatalf("executor payload=%s, want normalized target without actor scope", payload)
			}
			return map[string]any{"tenantId": 41, "dashboardUserId": 52, "version": 5}, nil
		})

	request := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalRequest", strings.NewReader(`{"actionType":"dashboard.activation.resend","payload":{"tenantId":41,"targetUserId":52,"expectedVersion":4},"reason":"重发激活","idempotencyKey":"task7-chain-1"}`))
	request.Header.Set("X-Mochat-Go-User-ID", "7")
	requestResponse := httptest.NewRecorder()
	handler.ApprovalRequest(requestResponse, request)
	if requestResponse.Code != http.StatusOK || store.createCalls != 1 {
		t.Fatalf("request status=%d calls=%d", requestResponse.Code, store.createCalls)
	}
	if store.createInput.RequesterUserID != 7 || store.createInput.ActionType != SaaSAdminApprovalActionDashboardActivationResend || strings.Contains(store.createInput.RequestJSON, "actor") {
		t.Fatalf("durable request actor/action/json=%d/%q/%s", store.createInput.RequesterUserID, store.createInput.ActionType, store.createInput.RequestJSON)
	}

	decision := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalDecision", strings.NewReader(`{"approvalId":7,"decision":"approve","reason":"另一人复核","expectedVersion":1}`))
	decision.Header.Set("X-Mochat-Go-User-ID", "9")
	decisionResponse := httptest.NewRecorder()
	handler.ApprovalDecision(decisionResponse, decision)
	if decisionResponse.Code != http.StatusOK || store.decisionInput.ActorUserID != 9 || store.decisionInput.ApprovalID != 7 {
		t.Fatalf("decision status=%d actor=%d approval=%d", decisionResponse.Code, store.decisionInput.ActorUserID, store.decisionInput.ApprovalID)
	}

	store.beginResult = SaaSAdminApproval{
		ID: 7, ActionType: SaaSAdminApprovalActionDashboardActivationResend,
		Status: SaaSAdminApprovalStatusExecuting, RequesterUserID: 7, ExecutionUserID: 9,
		RequestJSON: store.createInput.RequestJSON, Version: 3,
	}
	execute := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/approvalExecute", strings.NewReader(`{"approvalId":7,"expectedVersion":2}`))
	execute.Header.Set("X-Mochat-Go-User-ID", "9")
	executeResponse := httptest.NewRecorder()
	handler.ApprovalExecute(executeResponse, execute)
	if executeResponse.Code != http.StatusOK || store.beginInput.ActorUserID != 9 || store.finishCalls != 1 || !store.finishInput.Success {
		t.Fatalf("execute status=%d actor=%d finishCalls=%d success=%t", executeResponse.Code, store.beginInput.ActorUserID, store.finishCalls, store.finishInput.Success)
	}
}
