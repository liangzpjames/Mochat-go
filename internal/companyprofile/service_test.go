package companyprofile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

func companyProfileTestPrincipal(superadmin bool, status dashboardprincipal.CorpBindingStatus) dashboardprincipal.DashboardPrincipal {
	return dashboardprincipal.DashboardPrincipal{
		UserID:       101,
		TenantID:     202,
		CorpID:       303,
		CorpStatus:   status,
		IsSuperAdmin: superadmin,
		AuthVersion:  7,
	}
}

type companyProfileContractStore struct {
	getCalls             int
	updateCalls          int
	verificationCalls    int
	commitVerification   int
	rotateCorpCalls      int
	rotateAgentCalls     int
	rotateArchiveCalls   int
	syncCalls            int
	queueCalls           int
	failureCalls         int
	queueResult          EmployeeSyncQueueResult
	lastWeComInput       WeComCredentialsInput
	profile              Profile
	verificationSnapshot VerificationSnapshot
	auditPage            AuditPage
}

func (s *companyProfileContractStore) GetProfile(context.Context, dashboardprincipal.DashboardPrincipal) (Profile, error) {
	s.getCalls++
	return s.profile, nil
}

func (s *companyProfileContractStore) UpdateProfile(context.Context, dashboardprincipal.DashboardPrincipal, UpdateProfileInput) (Profile, error) {
	s.updateCalls++
	return s.profile, nil
}

func (s *companyProfileContractStore) GetVerificationSnapshot(context.Context, dashboardprincipal.DashboardPrincipal) (VerificationSnapshot, error) {
	s.verificationCalls++
	return s.verificationSnapshot, nil
}

func (s *companyProfileContractStore) CommitVerification(context.Context, dashboardprincipal.DashboardPrincipal, VerifyInput, VerificationResult) (Profile, error) {
	s.commitVerification++
	return s.profile, nil
}

func (s *companyProfileContractStore) RotateWeComCredentials(context.Context, dashboardprincipal.DashboardPrincipal, WeComCredentialsInput) (Profile, error) {
	s.rotateCorpCalls++
	return s.profile, nil
}

func (s *companyProfileContractStore) RotateAgentCredentials(context.Context, dashboardprincipal.DashboardPrincipal, AgentCredentialsInput) (Profile, error) {
	s.rotateAgentCalls++
	return s.profile, nil
}

func (s *companyProfileContractStore) RotateArchiveCredentials(context.Context, dashboardprincipal.DashboardPrincipal, ArchiveCredentialsInput) (Profile, error) {
	s.rotateArchiveCalls++
	return s.profile, nil
}

func (s *companyProfileContractStore) ListAudits(context.Context, dashboardprincipal.DashboardPrincipal, AuditFilter) (AuditPage, error) {
	return s.auditPage, nil
}

func (s *companyProfileContractStore) SyncEmployeeData(context.Context, dashboardprincipal.DashboardPrincipal, EmployeeSyncData) (SyncResult, error) {
	s.syncCalls++
	return SyncResult{Status: "completed"}, nil
}

func (s *companyProfileContractStore) QueueEmployeeSync(context.Context, dashboardprincipal.DashboardPrincipal) (EmployeeSyncQueueResult, error) {
	s.queueCalls++
	result := s.queueResult
	if result.Cursor == "" {
		result.Cursor = "company-sync"
	}
	return result, nil
}

func (s *companyProfileContractStore) RecordEmployeeSyncFailure(context.Context, dashboardprincipal.DashboardPrincipal) error {
	s.failureCalls++
	return nil
}

func (s *companyProfileContractStore) GetSyncStatus(context.Context, dashboardprincipal.DashboardPrincipal) (SyncStatus, error) {
	return SyncStatus{Status: "idle"}, nil
}

type companyProfileTestVerifier struct {
	calls  int
	result VerificationResult
	err    error
}

func (v *companyProfileTestVerifier) Verify(context.Context, VerificationRequest) (VerificationResult, error) {
	v.calls++
	return v.result, v.err
}

func TestServiceRejectsNonSuperAdminBeforeAnyStoreRead(t *testing.T) {
	store := &companyProfileContractStore{}
	service := NewService(store, &companyProfileTestVerifier{})

	_, err := service.GetProfile(context.Background(), companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive))
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("error = %v, want ErrPermissionDenied", err)
	}
	if store.getCalls != 0 {
		t.Fatalf("store reads = %d, want 0", store.getCalls)
	}
}

func TestServiceRejectsSuspendedBindingBeforeAnyStoreRead(t *testing.T) {
	store := &companyProfileContractStore{}
	service := NewService(store, &companyProfileTestVerifier{})

	_, err := service.GetProfile(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusSuspended))
	if !errors.Is(err, ErrTenantAccessDenied) {
		t.Fatalf("error = %v, want ErrTenantAccessDenied", err)
	}
	if store.getCalls != 0 {
		t.Fatalf("store reads = %d, want 0", store.getCalls)
	}
}

func TestServiceDoesNotAcceptCorpSelectionDuringVerification(t *testing.T) {
	store := &companyProfileContractStore{
		verificationSnapshot: VerificationSnapshot{
			Verified:       true,
			WXCorpID:       "ww-authoritative",
			BindingVersion: 9,
		},
	}
	verifier := &companyProfileTestVerifier{}
	service := NewService(store, verifier)

	_, err := service.Verify(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive), VerifyInput{
		WXCorpID:        "ww-client-selected",
		ExpectedVersion: 9,
	})
	if !errors.Is(err, ErrCorpIDImmutable) {
		t.Fatalf("error = %v, want ErrCorpIDImmutable", err)
	}
	if verifier.calls != 0 || store.commitVerification != 0 {
		t.Fatalf("verification calls=%d commit calls=%d, want 0/0", verifier.calls, store.commitVerification)
	}
}

func TestServiceDoesNotResubmitAuthoritativeCorpIDDuringVerification(t *testing.T) {
	store := &companyProfileContractStore{
		verificationSnapshot: VerificationSnapshot{
			Verified:       true,
			WXCorpID:       "ww-authoritative",
			BindingVersion: 9,
		},
	}
	verifier := &companyProfileTestVerifier{}
	service := NewService(store, verifier)

	_, err := service.Verify(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive), VerifyInput{
		WXCorpID:        "ww-authoritative",
		ExpectedVersion: 9,
	})
	if !errors.Is(err, ErrCorpIDImmutable) {
		t.Fatalf("error = %v, want ErrCorpIDImmutable", err)
	}
	if verifier.calls != 0 || store.commitVerification != 0 {
		t.Fatalf("verification calls=%d commit calls=%d, want 0/0", verifier.calls, store.commitVerification)
	}
}

func TestServiceDoesNotAcceptCorpSelectionDuringCredentialRotation(t *testing.T) {
	store := &companyProfileContractStore{}
	service := NewService(store, &companyProfileTestVerifier{})
	candidate := "ww-client-selected"
	_, err := service.RotateWeComCredentials(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive), WeComCredentialsInput{
		WXCorpID:        &candidate,
		EmployeeSecret:  stringPointer("employee-secret"),
		ExpectedVersion: 1,
	})
	if !errors.Is(err, ErrCorpIDImmutable) {
		t.Fatalf("error = %v, want ErrCorpIDImmutable", err)
	}
	if store.rotateCorpCalls != 0 {
		t.Fatalf("rotate calls = %d, want 0", store.rotateCorpCalls)
	}
}

func TestServiceEmployeeSyncRequiresVerifiedBinding(t *testing.T) {
	store := &companyProfileContractStore{verificationSnapshot: VerificationSnapshot{BindingVersion: 1}}
	scheduler := &companyProfileTestScheduler{}
	service := NewService(store, &companyProfileTestVerifier{}).WithEmployeeSyncScheduler(scheduler)

	_, err := service.StartEmployeeSync(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive))
	if !errors.Is(err, ErrTenantAccessDenied) {
		t.Fatalf("error = %v, want ErrTenantAccessDenied", err)
	}
	if scheduler.calls != 0 || store.queueCalls != 0 || store.syncCalls != 0 {
		t.Fatalf("scheduler calls=%d queue calls=%d sync calls=%d, want 0/0/0", scheduler.calls, store.queueCalls, store.syncCalls)
	}
}

func TestServiceEmployeeSyncUsesVerifiedPrincipalScopeOnly(t *testing.T) {
	store := &companyProfileContractStore{verificationSnapshot: VerificationSnapshot{
		Verified: true, WXCorpID: "ww-authoritative", BindingVersion: 1,
	}}
	scheduler := &companyProfileTestScheduler{cursor: "company-sync"}
	service := NewService(store, &companyProfileTestVerifier{}).WithEmployeeSyncScheduler(scheduler)

	result, err := service.StartEmployeeSync(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "queued" || result.Cursor != "company-sync" || scheduler.calls != 1 || scheduler.bindingID != 202 || store.queueCalls != 1 || store.syncCalls != 0 {
		t.Fatalf("result=%+v scheduler=%+v queueCalls=%d syncCalls=%d", result, scheduler, store.queueCalls, store.syncCalls)
	}
}

func TestProfileJSONContainsNoCredentialMaterial(t *testing.T) {
	profile := Profile{
		TenantID:    202,
		CorpID:      303,
		DisplayName: "示例企业",
		Credentials: CredentialStatuses{
			WeCom:   CredentialStatus{Configured: true, KeyID: "wecom-primary"},
			Agent:   CredentialStatus{Configured: true, KeyID: "wecom-primary"},
			Archive: CredentialStatus{Configured: true, KeyID: "wecom-primary"},
		},
		UpdatedAt: time.Unix(1, 0).UTC(),
	}
	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	serialized := strings.ToLower(string(raw))
	for _, forbidden := range []string{"secret", "ciphertext", "password", "hash", "token"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("profile JSON contains forbidden material %q: %s", forbidden, raw)
		}
	}
}

func TestAuditJSONContainsOnlySafeChangeMetadata(t *testing.T) {
	store := &companyProfileContractStore{auditPage: AuditPage{Items: []Audit{{
		Action: "dashboard.company.wecom_credentials.rotate", TargetType: "company", TargetID: "303",
		ChangedFields: []string{"employeeSecret", "chatSecret"}, RequestID: "task10-audit",
	}}}}
	service := NewService(store, &companyProfileTestVerifier{})
	page, err := service.ListAudits(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive), AuditFilter{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	serialized := strings.ToLower(string(raw))
	for _, forbidden := range []string{"ciphertext", "employee-secret", "chat-secret", "password", "hash", "token"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("audit JSON contains forbidden material %q: %s", forbidden, raw)
		}
	}
}

func TestHTTPRejectsClientRealmSelectorsWithoutCallingService(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/dashboard/company/profile", strings.NewReader(`{"displayName":"新名称","tenantId":999,"corpId":888,"actorUserId":777}`))
	request = request.WithContext(dashboardprincipal.WithPrincipal(request.Context(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive)))
	response := httptest.NewRecorder()
	NewHTTPHandler(nil).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), CodeInvalidRequest) {
		t.Fatalf("body = %s, want %s", response.Body.String(), CodeInvalidRequest)
	}
}

func TestHTTPDoesNotEchoCredentialMaterialOrAcceptItForVerification(t *testing.T) {
	store := &companyProfileContractStore{profile: Profile{BindingVersion: 2}}
	service := NewService(store, &companyProfileTestVerifier{result: VerificationResult{WXCorpID: "ww-authoritative", CorpName: "权威企业"}})
	handler := NewHTTPHandler(service)
	principal := companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive)

	rotateRequest := httptest.NewRequest(http.MethodPut, "/dashboard/company/wecom-credentials", strings.NewReader(`{"employeeSecret":"credential-not-for-response","expectedVersion":1}`))
	rotateRequest = rotateRequest.WithContext(dashboardprincipal.WithPrincipal(rotateRequest.Context(), principal))
	rotateResponse := httptest.NewRecorder()
	handler.ServeHTTP(rotateResponse, rotateRequest)
	if rotateResponse.Code != http.StatusOK || strings.Contains(rotateResponse.Body.String(), "credential-not-for-response") {
		t.Fatalf("rotate response code=%d body=%s", rotateResponse.Code, rotateResponse.Body.String())
	}

	verifyRequest := httptest.NewRequest(http.MethodPost, "/dashboard/company/verify", strings.NewReader(`{"wxCorpId":"ww-candidate","expectedVersion":1,"employeeSecret":"candidate-secret"}`))
	verifyRequest = verifyRequest.WithContext(dashboardprincipal.WithPrincipal(verifyRequest.Context(), principal))
	verifyResponse := httptest.NewRecorder()
	handler.ServeHTTP(verifyResponse, verifyRequest)
	if verifyResponse.Code != http.StatusBadRequest || strings.Contains(verifyResponse.Body.String(), "candidate-secret") {
		t.Fatalf("verify response code=%d body=%s", verifyResponse.Code, verifyResponse.Body.String())
	}
}

func TestHTTPEmployeeSyncQueuesBindingScopedJobAndRejectsClientRealmFields(t *testing.T) {
	store := &companyProfileContractStore{verificationSnapshot: VerificationSnapshot{
		Verified: true, WXCorpID: "ww-authoritative", BindingVersion: 3,
	}}
	scheduler := &companyProfileTestScheduler{cursor: "company-sync"}
	handler := NewHTTPHandler(NewService(store, &companyProfileTestVerifier{}).WithEmployeeSyncScheduler(scheduler))
	principal := companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive)

	request := httptest.NewRequest(http.MethodPost, "/dashboard/company/employee-sync", strings.NewReader(`{}`))
	request = request.WithContext(dashboardprincipal.WithPrincipal(request.Context(), principal))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"queued"`) || !strings.Contains(response.Body.String(), `"cursor":"company-sync"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if scheduler.calls != 1 || scheduler.bindingID != principal.TenantID || store.queueCalls != 1 {
		t.Fatalf("scheduler=%+v queueCalls=%d", scheduler, store.queueCalls)
	}

	badRequest := httptest.NewRequest(http.MethodPost, "/dashboard/company/employee-sync", strings.NewReader(`{"tenantId":999,"corpId":888}`))
	badRequest = badRequest.WithContext(dashboardprincipal.WithPrincipal(badRequest.Context(), principal))
	badResponse := httptest.NewRecorder()
	handler.ServeHTTP(badResponse, badRequest)
	if badResponse.Code != http.StatusBadRequest || store.queueCalls != 1 || scheduler.calls != 1 {
		t.Fatalf("realm selector request status=%d body=%s queueCalls=%d schedulerCalls=%d", badResponse.Code, badResponse.Body.String(), store.queueCalls, scheduler.calls)
	}
}

func stringPointer(value string) *string {
	return &value
}
