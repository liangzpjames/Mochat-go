package companyprofile

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

const recoveryEventKey = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type recoveryContractStore struct {
	*companyProfileContractStore
	listCalls      int
	detailCalls    int
	reconcileCalls int
	lastPrincipal  dashboardprincipal.DashboardPrincipal
	lastRequestID  string
	recoveryErr    error
}

func (s *recoveryContractStore) ListCallbackSideEffects(_ context.Context, principal dashboardprincipal.DashboardPrincipal, _ CallbackSideEffectListInput) (CallbackSideEffectPage, error) {
	s.listCalls++
	s.lastPrincipal = principal
	if s.recoveryErr != nil {
		return CallbackSideEffectPage{}, s.recoveryErr
	}
	return CallbackSideEffectPage{Items: []CallbackSideEffectSummary{{EventKey: recoveryEventKey, ActionKey: "fission.employee_reminder", Status: "unknown", Version: 3}}}, nil
}

func TestCallbackSideEffectRecoveryDatabaseFailureHasStableServiceUnavailableSemantics(t *testing.T) {
	service, store := recoveryTestService()
	store.recoveryErr = errors.New("database connection refused")
	principal := companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive)
	request := httptest.NewRequest(http.MethodGet, "/dashboard/company/callback-side-effects?status=unknown", nil)
	request = request.WithContext(recoveryContext(principal, true))
	response := httptest.NewRecorder()
	NewHTTPHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "CALLBACK_RECOVERY_UNAVAILABLE") {
		t.Fatalf("database failure status=%d body=%s", response.Code, response.Body.String())
	}
}

func (s *recoveryContractStore) GetCallbackSideEffect(_ context.Context, principal dashboardprincipal.DashboardPrincipal, _, _ string) (CallbackSideEffectDetail, error) {
	s.detailCalls++
	s.lastPrincipal = principal
	return CallbackSideEffectDetail{EventKey: recoveryEventKey}, nil
}

func (s *recoveryContractStore) ReconcileCallbackSideEffect(_ context.Context, principal dashboardprincipal.DashboardPrincipal, eventKey, actionKey, requestID string, _ CallbackSideEffectReconcileInput) (CallbackSideEffectReconcileResult, error) {
	s.reconcileCalls++
	s.lastPrincipal = principal
	s.lastRequestID = requestID
	return CallbackSideEffectReconcileResult{EventKey: eventKey, ActionKey: actionKey, Status: "sent", Version: 4}, nil
}

func recoveryTestService() (*Service, *recoveryContractStore) {
	store := &recoveryContractStore{companyProfileContractStore: &companyProfileContractStore{}}
	return NewService(store, nil), store
}

func recoveryContext(principal dashboardprincipal.DashboardPrincipal, permitted bool) context.Context {
	ctx := dashboardprincipal.WithPrincipal(context.Background(), principal)
	if permitted {
		ctx = dashboardprincipal.WithCapabilityAccess(ctx, false, []string{"dashboard.company_setting.website"})
	}
	return ctx
}

func TestCallbackSideEffectRecoveryRequiresPrincipalPermissionBeforeStore(t *testing.T) {
	service, store := recoveryTestService()
	principal := companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive)
	if _, err := service.ListCallbackSideEffects(context.Background(), principal, CallbackSideEffectListInput{}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("missing permission error = %v, want permission denied", err)
	}
	if store.listCalls != 0 {
		t.Fatalf("store calls = %d, want 0", store.listCalls)
	}
}

func TestCallbackSideEffectRecoveryScopeComesOnlyFromPrincipal(t *testing.T) {
	service, store := recoveryTestService()
	principal := companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive)
	ctx := dashboardprincipal.WithCapabilityAccess(context.Background(), false, []string{"dashboard.company_setting.website"})
	page, err := service.ListCallbackSideEffects(ctx, principal, CallbackSideEffectListInput{Status: "unknown", Limit: 50})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("list result=%+v err=%v", page, err)
	}
	if store.lastPrincipal.TenantID != principal.TenantID || store.lastPrincipal.CorpID != principal.CorpID {
		t.Fatalf("store scope=%d/%d want=%d/%d", store.lastPrincipal.TenantID, store.lastPrincipal.CorpID, principal.TenantID, principal.CorpID)
	}
}

func TestCallbackSideEffectReconcileValidatesDecisionAndIdempotencyKey(t *testing.T) {
	service, store := recoveryTestService()
	principal := companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive)
	ctx := dashboardprincipal.WithCapabilityAccess(context.Background(), false, []string{"dashboard.company_setting.website"})
	valid := CallbackSideEffectReconcileInput{Decision: CallbackSideEffectDecisionConfirmSent, ExpectedVersion: 3, ExpectedInboxLeaseFence: 7, Reason: "企微后台回执已确认", EvidenceKind: "provider_message_id", EvidenceRef: "receipt-20260830-001"}
	if _, err := service.ReconcileCallbackSideEffect(ctx, principal, recoveryEventKey, "fission.employee_reminder", "short", valid); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("short idempotency key error=%v", err)
	}
	valid.Decision = "guess_not_sent"
	if _, err := service.ReconcileCallbackSideEffect(ctx, principal, recoveryEventKey, "fission.employee_reminder", "request-20260830-0001", valid); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid decision error=%v", err)
	}
	valid.Decision = CallbackSideEffectDecisionConfirmNotSentAndRetry
	valid.EvidenceKind = "provider_message_id"
	if _, err := service.ReconcileCallbackSideEffect(ctx, principal, recoveryEventKey, "fission.employee_reminder", "request-20260830-0001", valid); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("decision/evidence mismatch error=%v", err)
	}
	if store.reconcileCalls != 0 {
		t.Fatalf("store reconcile calls=%d, want 0", store.reconcileCalls)
	}
}

func TestCallbackSideEffectRecoveryHTTPRejectsScopeInjectionAndUsesHeaderRequestID(t *testing.T) {
	service, store := recoveryTestService()
	handler := NewHTTPHandler(service)
	principal := companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive)

	injected := httptest.NewRequest(http.MethodPost, "/dashboard/company/callback-side-effects/"+recoveryEventKey+"/fission.employee_reminder/reconcile", strings.NewReader(`{"decision":"confirm_sent","expectedVersion":3,"expectedInboxLeaseFence":7,"reason":"已核验","evidenceKind":"provider_message_id","evidenceRef":"receipt-1","tenantId":999}`))
	injected = injected.WithContext(recoveryContext(principal, true))
	injected.Header.Set("Idempotency-Key", "request-20260830-0001")
	injectedResponse := httptest.NewRecorder()
	handler.ServeHTTP(injectedResponse, injected)
	if injectedResponse.Code != http.StatusBadRequest || store.reconcileCalls != 0 {
		t.Fatalf("scope injection status=%d calls=%d body=%s", injectedResponse.Code, store.reconcileCalls, injectedResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "/dashboard/company/callback-side-effects/"+recoveryEventKey+"/fission.employee_reminder/reconcile", strings.NewReader(`{"decision":"confirm_sent","expectedVersion":3,"expectedInboxLeaseFence":7,"reason":"企微后台回执已确认","evidenceKind":"provider_message_id","evidenceRef":"receipt-1"}`))
	request = request.WithContext(recoveryContext(principal, true))
	request.Header.Set("Idempotency-Key", "request-20260830-0001")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || store.reconcileCalls != 1 || store.lastRequestID != "request-20260830-0001" {
		t.Fatalf("reconcile status=%d calls=%d request=%q body=%s", response.Code, store.reconcileCalls, store.lastRequestID, response.Body.String())
	}
}
