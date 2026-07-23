package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSaaSAdminAuditIntegrityStore struct {
	*fakeSaaSAdminStore
	overviewOptions SaaSAdminAuditIntegrityOptions
	verifyOptions   SaaSAdminAuditIntegrityOptions
	overview        SaaSAdminAuditIntegrityOverview
	verifyResult    SaaSAdminAuditIntegrityVerifyResult
	overviewCalls   int
	verifyCalls     int
}

func (s *fakeSaaSAdminAuditIntegrityStore) SaaSAdminAuditIntegrityOverview(_ context.Context, options SaaSAdminAuditIntegrityOptions) (SaaSAdminAuditIntegrityOverview, error) {
	s.overviewCalls++
	s.overviewOptions = options
	return s.overview, nil
}

func (s *fakeSaaSAdminAuditIntegrityStore) VerifySaaSAdminAuditIntegrity(_ context.Context, options SaaSAdminAuditIntegrityOptions) (SaaSAdminAuditIntegrityVerifyResult, error) {
	s.verifyCalls++
	s.verifyOptions = options
	return s.verifyResult, nil
}

func TestSaaSAdminAuditIntegrityHandlers(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	store := &fakeSaaSAdminAuditIntegrityStore{
		fakeSaaSAdminStore: base,
		overview: SaaSAdminAuditIntegrityOverview{
			Summary: SaaSAdminAuditIntegritySummary{ChainCount: 1, HealthyChainCount: 1},
			Chains:  []SaaSAdminAuditIntegrityChain{{TenantID: 8, Status: SaaSAdminAuditIntegrityStatusHealthy}},
		},
		verifyResult: SaaSAdminAuditIntegrityVerifyResult{ScannedChains: 1, HealthyChains: 1, OperationID: 88},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	getReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/auditIntegrity?tenantId=8&limit=25&verificationLimit=10", nil)
	getReq.Header.Set("X-Mochat-Go-User-ID", "1")
	getRec := httptest.NewRecorder()
	handler.AuditIntegrity(getRec, getReq)
	if getRec.Code != http.StatusOK || store.overviewCalls != 1 {
		t.Fatalf("GET status=%d calls=%d body=%s", getRec.Code, store.overviewCalls, getRec.Body.String())
	}
	if store.overviewOptions.TenantID != 8 || store.overviewOptions.Limit != 25 || store.overviewOptions.VerificationLimit != 10 {
		t.Fatalf("overview options=%+v", store.overviewOptions)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/auditIntegrityVerify", strings.NewReader(`{"tenantId":8,"limit":25,"verificationLimit":10}`))
	postReq.Header.Set("X-Mochat-Go-User-ID", "1")
	postRec := httptest.NewRecorder()
	handler.AuditIntegrityVerify(postRec, postReq)
	if postRec.Code != http.StatusOK || store.verifyCalls != 1 {
		t.Fatalf("POST status=%d calls=%d body=%s", postRec.Code, store.verifyCalls, postRec.Body.String())
	}
	if store.verifyOptions.TenantID != 8 || store.verifyOptions.Source != "admin_manual" || store.verifyOptions.ActorUserID != 1 || store.verifyOptions.ActorTenantID != 1 || !store.verifyOptions.RecordOperation {
		t.Fatalf("verify options=%+v", store.verifyOptions)
	}
	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(postRec.Body.Bytes(), &payload); err != nil || payload.Data["operationId"] != float64(88) || payload.Data["healthyChains"] != float64(1) {
		t.Fatalf("payload=%+v err=%v", payload, err)
	}

	badReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/auditIntegrityVerify", strings.NewReader(`{"limit":501}`))
	badReq.Header.Set("X-Mochat-Go-User-ID", "1")
	badRec := httptest.NewRecorder()
	handler.AuditIntegrityVerify(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("bad status=%d body=%s", badRec.Code, badRec.Body.String())
	}
}
