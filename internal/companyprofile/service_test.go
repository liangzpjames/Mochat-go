package companyprofile

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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
	configureAppCalls    int
	callbackReadCalls    int
	callbackRotateCalls  int
	syncCalls            int
	queueCalls           int
	failureCalls         int
	queueResult          EmployeeSyncQueueResult
	queueErr             error
	syncState            string
	lastWeComInput       WeComCredentialsInput
	lastAgentInput       AgentCredentialsInput
	lastApplicationInput ApplicationCredentialsInput
	lastArchiveInput     ArchiveCredentialsInput
	lastCallbackInput    CallbackConfigurationInput
	rotateAgentErr       error
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

func (s *companyProfileContractStore) RotateAgentCredentials(_ context.Context, _ dashboardprincipal.DashboardPrincipal, input AgentCredentialsInput) (Profile, error) {
	s.rotateAgentCalls++
	s.lastAgentInput = input
	if s.rotateAgentErr != nil {
		return Profile{}, s.rotateAgentErr
	}
	return s.profile, nil
}

func (s *companyProfileContractStore) RotateArchiveCredentials(context.Context, dashboardprincipal.DashboardPrincipal, ArchiveCredentialsInput) (Profile, error) {
	s.rotateArchiveCalls++
	return s.profile, nil
}

func (s *companyProfileContractStore) ConfigureApplication(_ context.Context, _ dashboardprincipal.DashboardPrincipal, input ApplicationCredentialsInput) (Profile, error) {
	s.configureAppCalls++
	s.lastApplicationInput = input
	return s.profile, nil
}

func (s *companyProfileContractStore) GetCallbackConfiguration(context.Context, dashboardprincipal.DashboardPrincipal) (CallbackConfiguration, error) {
	s.callbackReadCalls++
	return CallbackConfiguration{CorpID: 303, Token: "callback-token", EncodingAESKey: strings.Repeat("a", 43)}, nil
}

func (s *companyProfileContractStore) RegenerateCallbackConfiguration(_ context.Context, _ dashboardprincipal.DashboardPrincipal, input CallbackConfigurationInput) (CallbackConfiguration, error) {
	s.callbackRotateCalls++
	s.lastCallbackInput = input
	return CallbackConfiguration{CorpID: 303, Token: input.Token, EncodingAESKey: input.EncodingAESKey}, nil
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
	if s.queueErr != nil {
		// Model a worker that completed after Redis accepted the job but before
		// the request-side marker transaction returned its error.
		s.syncState = "completed"
		return EmployeeSyncQueueResult{}, s.queueErr
	}
	s.syncState = "queued"
	result := s.queueResult
	if result.Cursor == "" {
		result.Cursor = "company-sync"
	}
	return result, nil
}

func (s *companyProfileContractStore) RecordEmployeeSyncFailure(context.Context, dashboardprincipal.DashboardPrincipal) error {
	s.failureCalls++
	s.syncState = "failed"
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

func TestServiceAllowsAgentIdentifierOnlyRotationWithoutSecret(t *testing.T) {
	store := &companyProfileContractStore{profile: Profile{BindingVersion: 2}}
	service := NewService(store, &companyProfileTestVerifier{})

	_, err := service.RotateAgentCredentials(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive), AgentCredentialsInput{
		AgentID:         300,
		WXAgentID:       "wx-agent-300",
		ExpectedVersion: 2,
		RequestID:       "agent-identifier-only",
	})
	if err != nil {
		t.Fatalf("identifier-only rotation error = %v, want success", err)
	}
	if store.rotateAgentCalls != 1 {
		t.Fatalf("rotate agent calls = %d, want 1", store.rotateAgentCalls)
	}
}

func TestServiceTreatsBlankAgentSecretAsNoChange(t *testing.T) {
	store := &companyProfileContractStore{profile: Profile{BindingVersion: 2}}
	service := NewService(store, &companyProfileTestVerifier{})
	blankSecret := "   "

	_, err := service.RotateAgentCredentials(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive), AgentCredentialsInput{
		AgentID:         300,
		WXSecret:        &blankSecret,
		ExpectedVersion: 2,
		RequestID:       "agent-blank-secret",
	})
	if err != nil {
		t.Fatalf("blank-secret rotation error = %v, want success", err)
	}
	if store.lastAgentInput.WXSecret != nil {
		t.Fatal("blank agent secret was forwarded as a replacement")
	}
}

func TestServiceReturnsNotFoundWhenAgentDoesNotExist(t *testing.T) {
	store := &companyProfileContractStore{rotateAgentErr: ErrNotFound}
	service := NewService(store, &companyProfileTestVerifier{})

	_, err := service.RotateAgentCredentials(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive), AgentCredentialsInput{
		WXAgentID:       "missing-agent",
		ExpectedVersion: 2,
		RequestID:       "agent-missing",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing agent error = %v, want ErrNotFound", err)
	}
}

func TestServiceRejectsAgentRotationWithoutIdentifierOrChange(t *testing.T) {
	store := &companyProfileContractStore{}
	service := NewService(store, &companyProfileTestVerifier{})

	_, err := service.RotateAgentCredentials(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive), AgentCredentialsInput{
		ExpectedVersion: 2,
		RequestID:       "agent-empty",
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("empty agent rotation error = %v, want ErrInvalidRequest", err)
	}
	if store.rotateAgentCalls != 0 {
		t.Fatalf("rotate agent calls = %d, want 0", store.rotateAgentCalls)
	}
}

func TestServiceConfiguresOneApplicationSecretForAllCredentialConsumers(t *testing.T) {
	store := &companyProfileContractStore{profile: Profile{BindingVersion: 8}}
	service := NewService(store, &companyProfileTestVerifier{})

	profile, err := service.ConfigureApplication(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive), ApplicationCredentialsInput{
		WXAgentID: "1000010", Secret: "shared-application-secret", ExpectedVersion: 7, RequestID: "configure-application",
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.BindingVersion != 8 || store.configureAppCalls != 1 {
		t.Fatalf("profile=%+v calls=%d", profile, store.configureAppCalls)
	}
	if store.lastApplicationInput.WXAgentID != "1000010" || store.lastApplicationInput.Secret != "shared-application-secret" {
		t.Fatalf("application input=%+v", store.lastApplicationInput)
	}
}

func TestServiceRejectsIncompleteApplicationConfiguration(t *testing.T) {
	store := &companyProfileContractStore{}
	service := NewService(store, &companyProfileTestVerifier{})
	principal := companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive)

	for _, input := range []ApplicationCredentialsInput{
		{Secret: "secret", ExpectedVersion: 1},
		{WXAgentID: "1000010", ExpectedVersion: 1},
		{WXAgentID: "not-numeric", Secret: "secret", ExpectedVersion: 1},
	} {
		if _, err := service.ConfigureApplication(context.Background(), principal, input); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("input=%+v error=%v, want ErrInvalidRequest", input, err)
		}
	}
	if store.configureAppCalls != 0 {
		t.Fatalf("configure calls=%d, want 0", store.configureAppCalls)
	}
}

func TestServiceGeneratesValidCallbackConfiguration(t *testing.T) {
	store := &companyProfileContractStore{}
	service := NewService(store, &companyProfileTestVerifier{})
	principal := companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive)

	for index := 0; index < 128; index++ {
		configuration, err := service.RegenerateCallbackConfiguration(context.Background(), principal, CallbackConfigurationInput{
			ExpectedVersion: 9, RequestID: "rotate-callback",
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(configuration.Token) != 32 || len(configuration.EncodingAESKey) != 43 {
			t.Fatalf("configuration=%+v", configuration)
		}
		for _, character := range configuration.EncodingAESKey {
			if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')) {
				t.Fatalf("EncodingAESKey contains non-alphanumeric character %q: %q", character, configuration.EncodingAESKey)
			}
		}
	}
	if store.callbackRotateCalls != 128 {
		t.Fatalf("callback rotate calls = %d, want 128", store.callbackRotateCalls)
	}
}

func TestServiceArchiveConfigurationRequiresMatchingRSAKeyPair(t *testing.T) {
	publicKey, privateKey := companyProfileRSAKeyPair(t)
	store := &companyProfileContractStore{profile: Profile{BindingVersion: 2}}
	service := NewService(store, &companyProfileTestVerifier{})
	principal := companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive)

	_, err := service.RotateArchiveCredentials(context.Background(), principal, ArchiveCredentialsInput{
		ChatSecret: stringPointer("archive-secret"), RSAPublicKey: &publicKey, RSAPrivateKey: &privateKey,
		ExpectedVersion: 1, RequestID: "archive-rsa",
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.rotateArchiveCalls != 1 {
		t.Fatalf("archive calls=%d, want 1", store.rotateArchiveCalls)
	}

	otherPublic, _ := companyProfileRSAKeyPair(t)
	_, err = service.RotateArchiveCredentials(context.Background(), principal, ArchiveCredentialsInput{
		RSAPublicKey: &otherPublic, RSAPrivateKey: &privateKey, ExpectedVersion: 1,
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("mismatched RSA error=%v, want ErrInvalidRequest", err)
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

	emptyRequest := httptest.NewRequest(http.MethodPost, "/dashboard/company/employee-sync", strings.NewReader(""))
	emptyRequest = emptyRequest.WithContext(dashboardprincipal.WithPrincipal(emptyRequest.Context(), principal))
	emptyResponse := httptest.NewRecorder()
	handler.ServeHTTP(emptyResponse, emptyRequest)
	if emptyResponse.Code != http.StatusOK || scheduler.calls != 2 || store.queueCalls != 2 {
		t.Fatalf("empty body status=%d body=%s queueCalls=%d schedulerCalls=%d", emptyResponse.Code, emptyResponse.Body.String(), store.queueCalls, scheduler.calls)
	}

	badRequest := httptest.NewRequest(http.MethodPost, "/dashboard/company/employee-sync", strings.NewReader(`{"tenantId":999,"corpId":888}`))
	badRequest = badRequest.WithContext(dashboardprincipal.WithPrincipal(badRequest.Context(), principal))
	badResponse := httptest.NewRecorder()
	handler.ServeHTTP(badResponse, badRequest)
	if badResponse.Code != http.StatusBadRequest || store.queueCalls != 2 || scheduler.calls != 2 {
		t.Fatalf("realm selector request status=%d body=%s queueCalls=%d schedulerCalls=%d", badResponse.Code, badResponse.Body.String(), store.queueCalls, scheduler.calls)
	}
}

func TestHTTPApplicationConfigurationAcceptsOnlyAgentIDAndOneSecret(t *testing.T) {
	store := &companyProfileContractStore{profile: Profile{BindingVersion: 2}}
	handler := NewHTTPHandler(NewService(store, &companyProfileTestVerifier{}))
	request := httptest.NewRequest(http.MethodPut, "/dashboard/company/application-credentials", strings.NewReader(`{"wxAgentId":"1000010","secret":"shared-secret","expectedVersion":1,"requestId":"app-config"}`))
	request = request.WithContext(dashboardprincipal.WithPrincipal(request.Context(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive)))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || store.configureAppCalls != 1 || strings.Contains(response.Body.String(), "shared-secret") {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, store.configureAppCalls, response.Body.String())
	}
	badRequest := httptest.NewRequest(http.MethodPut, "/dashboard/company/application-credentials", strings.NewReader(`{"wxAgentId":"1000010","secret":"shared-secret","employeeSecret":"forbidden","expectedVersion":1}`))
	badRequest = badRequest.WithContext(request.Context())
	badResponse := httptest.NewRecorder()
	handler.ServeHTTP(badResponse, badRequest)
	if badResponse.Code != http.StatusBadRequest || store.configureAppCalls != 1 {
		t.Fatalf("bad status=%d calls=%d body=%s", badResponse.Code, store.configureAppCalls, badResponse.Body.String())
	}
}

func TestHTTPCallbackConfigurationBuildsRequestHostURLAndDisablesCaching(t *testing.T) {
	store := &companyProfileContractStore{}
	handler := NewHTTPHandler(NewService(store, &companyProfileTestVerifier{}))
	request := httptest.NewRequest(http.MethodGet, "http://139.196.34.133/dashboard/company/callback-configuration", nil)
	request = request.WithContext(dashboardprincipal.WithPrincipal(request.Context(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive)))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"callbackUrl":"http://139.196.34.133/weWork/callback?cid=303"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("cache headers=%v", response.Header())
	}
}

func TestHTTPCallbackRegenerationRejectsClientProvidedSecrets(t *testing.T) {
	store := &companyProfileContractStore{}
	handler := NewHTTPHandler(NewService(store, &companyProfileTestVerifier{}))
	principalContext := dashboardprincipal.WithPrincipal(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive))
	request := httptest.NewRequest(http.MethodPost, "/dashboard/company/callback-configuration/regenerate", strings.NewReader(`{"expectedVersion":9,"requestId":"rotate-callback"}`)).WithContext(principalContext)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || store.callbackRotateCalls != 1 || !strings.Contains(response.Body.String(), store.lastCallbackInput.Token) || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, store.callbackRotateCalls, response.Body.String())
	}
	badRequest := httptest.NewRequest(http.MethodPost, "/dashboard/company/callback-configuration/regenerate", strings.NewReader(`{"expectedVersion":9,"token":"client-token"}`)).WithContext(principalContext)
	badResponse := httptest.NewRecorder()
	handler.ServeHTTP(badResponse, badRequest)
	if badResponse.Code != http.StatusBadRequest || store.callbackRotateCalls != 1 {
		t.Fatalf("bad status=%d calls=%d body=%s", badResponse.Code, store.callbackRotateCalls, badResponse.Body.String())
	}
}

func stringPointer(value string) *string {
	return &value
}

func companyProfileRSAKeyPair(t *testing.T) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return string(publicPEM), string(privatePEM)
}
