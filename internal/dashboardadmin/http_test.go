package dashboardadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jiyi/mochat-go/internal/saasauth"
)

type dashboardAdminHTTPStore struct {
	provisionResult  ProvisionResult
	provisionErr     error
	provisionCalls   int
	seenActor        Actor
	seenInput        ProvisionDashboardTenant
	resendResult     ResendActivationResult
	resendErr        error
	resendCalls      int
	seenResend       ResendActivationInput
	replaceResult    GovernanceResult
	replaceErr       error
	replaceCalls     int
	seenReplace      ReplaceSuperAdminInput
	statusResult     GovernanceResult
	statusErr        error
	statusCalls      int
	seenStatus       SuperAdminStatusInput
	governanceResult DashboardAdminGovernanceView
	governanceErr    error
	governanceCalls  int
}

func (store *dashboardAdminHTTPStore) ProvisionDashboardTenant(_ context.Context, actor Actor, input ProvisionDashboardTenant) (ProvisionResult, error) {
	store.provisionCalls++
	store.seenActor = actor
	store.seenInput = input
	return store.provisionResult, store.provisionErr
}

func (store *dashboardAdminHTTPStore) DashboardAdminGovernance(_ context.Context, actor Actor, tenantID int) (DashboardAdminGovernanceView, error) {
	store.governanceCalls++
	store.seenActor = actor
	if tenantID != 41 {
		return DashboardAdminGovernanceView{}, ErrTargetNotFound
	}
	return store.governanceResult, store.governanceErr
}

func (store *dashboardAdminHTTPStore) ResendDashboardActivation(_ context.Context, actor Actor, input ResendActivationInput) (ResendActivationResult, error) {
	store.resendCalls++
	store.seenActor = actor
	store.seenResend = input
	return store.resendResult, store.resendErr
}

func (store *dashboardAdminHTTPStore) ReplaceDashboardSuperAdmin(_ context.Context, actor Actor, input ReplaceSuperAdminInput) (GovernanceResult, error) {
	store.replaceCalls++
	store.seenActor = actor
	store.seenReplace = input
	return store.replaceResult, store.replaceErr
}

func (store *dashboardAdminHTTPStore) SetDashboardSuperAdminStatus(_ context.Context, actor Actor, input SuperAdminStatusInput) (GovernanceResult, error) {
	store.statusCalls++
	store.seenActor = actor
	store.seenStatus = input
	return store.statusResult, store.statusErr
}

func TestDashboardAdminHTTPProvisionUsesSaaSPrincipalAndReturnsActivationOnce(t *testing.T) {
	store := &dashboardAdminHTTPStore{provisionResult: ProvisionResult{
		TenantID:            41,
		DashboardUserID:     52,
		BindingCorpID:       63,
		ActivationToken:     "opaque-activation-value",
		ActivationExpiresAt: time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC),
	}}
	handler := NewHTTPHandler(NewService(store))
	body, err := json.Marshal(validProvisionInput())
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenants/provision", bytes.NewReader(body))
	request = request.WithContext(saasauth.WithPrincipal(request.Context(), saasauth.Principal{UserID: 700, AuthVersion: 9}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d bodyBytes=%d, want 201", response.Code, response.Body.Len())
	}
	if store.provisionCalls != 1 || store.seenActor.UserID != 700 || !store.seenActor.Active || len(store.seenActor.Permissions) != 0 {
		t.Fatalf("store actor=%+v calls=%d, want identity-only SaaS principal with Store authorization", store.seenActor, store.provisionCalls)
	}
	if store.seenInput.BodyTenantID != 0 || store.seenInput.BodyActorID != 0 {
		t.Fatal("HTTP body was allowed to supply tenant or actor scope")
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			TenantID            int    `json:"tenantId"`
			DashboardUserID     int    `json:"dashboardUserId"`
			BindingCorpID       int    `json:"bindingCorpId"`
			ActivationToken     string `json:"activationToken"`
			ActivationPath      string `json:"activationPath"`
			ActivationExpiresAt string `json:"activationExpiresAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != http.StatusCreated || envelope.Data.TenantID != 41 || envelope.Data.DashboardUserID != 52 || envelope.Data.BindingCorpID != 63 || envelope.Data.ActivationToken == "" {
		t.Fatalf("provision response code=%d tenant=%d user=%d corp=%d activationPresent=%t, want business result with one activation value", envelope.Code, envelope.Data.TenantID, envelope.Data.DashboardUserID, envelope.Data.BindingCorpID, envelope.Data.ActivationToken != "")
	}
	if envelope.Data.ActivationPath != "/activate#token=opaque-activation-value" || envelope.Data.ActivationExpiresAt != "2026-08-27T12:00:00Z" {
		t.Fatalf("activation delivery=%+v", envelope.Data)
	}
}

func TestDashboardAdminHTTPAuthenticatedPrincipalWithoutTenantManageIsForbidden(t *testing.T) {
	store := &dashboardAdminHTTPStore{governanceErr: ErrPermissionDenied}
	handler := NewHTTPHandler(NewService(store))
	request := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenants/41/dashboard-admins", nil)
	request = request.WithContext(saasauth.WithPrincipal(request.Context(), saasauth.Principal{UserID: 701, AuthVersion: 9}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || store.governanceCalls != 1 {
		t.Fatalf("status=%d calls=%d, want authenticated but unauthorized SaaS actor to receive 403 from Store", response.Code, store.governanceCalls)
	}
	if len(store.seenActor.Permissions) != 0 {
		t.Fatalf("HTTP synthesized permissions=%v, must leave permission resolution to the transaction Store", store.seenActor.Permissions)
	}
}

func TestDashboardAdminHTTPGovernanceListUsesPathTenantAndNeverReturnsCredentialMaterial(t *testing.T) {
	store := &dashboardAdminHTTPStore{governanceResult: DashboardAdminGovernanceView{
		TenantID: 41, BindingVersion: 8,
		Identities: []DashboardIdentityRecord{{ID: 52, Name: "管理员", LoginIdentifier: "13800000000", UserStatus: 1, IdentityStatus: 1, ActivatedAt: "2026-08-11T00:00:00Z", IsSuperAdmin: true}},
	}}
	handler := NewHTTPHandler(NewService(store))
	request := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenants/41/dashboard-admins", nil).WithContext(saasauth.WithPrincipal(context.Background(), saasauth.Principal{UserID: 700, AuthVersion: 9}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || store.governanceCalls != 1 || store.seenActor.UserID != 700 {
		t.Fatalf("status=%d calls=%d actor=%+v, want authenticated governance read", response.Code, store.governanceCalls, store.seenActor)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("password")) || bytes.Contains(response.Body.Bytes(), []byte("digest")) || bytes.Contains(response.Body.Bytes(), []byte("secret")) {
		t.Fatal("governance response exposed credential material")
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"bindingVersion":8`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"isSuperAdmin":true`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"identityStatus":1`)) {
		t.Fatalf("governance response=%s, want binding version and identity facts", response.Body.String())
	}
}

func TestDashboardAdminHTTPProvisionRejectsUnauthenticatedAndMalformedJSON(t *testing.T) {
	malformed := []byte(`{"tenantName":"Tenant","limits":{}} {}`)
	for _, tc := range []struct {
		name    string
		context context.Context
		body    []byte
		want    int
	}{
		{name: "missing SaaS principal", context: context.Background(), body: mustProvisionJSON(t), want: http.StatusUnauthorized},
		{name: "trailing JSON", context: saasauth.WithPrincipal(context.Background(), saasauth.Principal{UserID: 700, AuthVersion: 9}), body: malformed, want: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &dashboardAdminHTTPStore{}
			handler := NewHTTPHandler(NewService(store))
			request := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenants/provision", bytes.NewReader(tc.body)).WithContext(tc.context)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status=%d bodyBytes=%d, want %d", response.Code, response.Body.Len(), tc.want)
			}
			if store.provisionCalls != 0 {
				t.Fatalf("malformed or unauthenticated request reached store: calls=%d", store.provisionCalls)
			}
		})
	}
}

func TestDashboardAdminHTTPProvisionMapsIdempotencyConflictWithoutToken(t *testing.T) {
	store := &dashboardAdminHTTPStore{provisionErr: ErrIdempotencyConflict}
	handler := NewHTTPHandler(NewService(store))
	request := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenants/provision", bytes.NewReader(mustProvisionJSON(t))).WithContext(saasauth.WithPrincipal(context.Background(), saasauth.Principal{UserID: 700, AuthVersion: 9}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d bodyBytes=%d, want 409", response.Code, response.Body.Len())
	}
	var envelope struct {
		ErrorCode string `json:"errorCode"`
		Data      any    `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ErrorCode != "IDEMPOTENCY_CONFLICT" || envelope.Data != nil {
		t.Fatalf("conflict envelope errorCode=%q data=%v, want stable code without activation data", envelope.ErrorCode, envelope.Data)
	}
}

func TestDashboardAdminHTTPResendReturnsTokenOnceAndUsesHeaderRequestID(t *testing.T) {
	store := &dashboardAdminHTTPStore{resendResult: ResendActivationResult{
		TenantID: 41, DashboardUserID: 52, Version: 4, ActivationToken: "one-time-resend-token",
		ActivationExpiresAt: time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC),
	}}
	handler := NewHTTPHandler(NewService(store))
	request := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenants/41/activation/resend", bytes.NewBufferString(`{"targetUserId":52,"expectedVersion":3}`))
	request.Header.Set("X-Request-ID", "resend-request-1")
	request = request.WithContext(saasauth.WithPrincipal(request.Context(), saasauth.Principal{UserID: 700, AuthVersion: 9}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || store.resendCalls != 1 || store.seenResend.RequestID != "resend-request-1" || store.seenResend.TenantID != 41 || store.seenActor.UserID != 700 {
		t.Fatalf("status=%d calls=%d actor=%+v input=%+v, want one authenticated resend without body tenant", response.Code, store.resendCalls, store.seenActor, store.seenResend)
	}
	var envelope struct {
		Data struct {
			TenantID            int    `json:"tenantId"`
			DashboardUserID     int    `json:"dashboardUserId"`
			Version             uint64 `json:"version"`
			ActivationToken     string `json:"activationToken"`
			ActivationPath      string `json:"activationPath"`
			ActivationExpiresAt string `json:"activationExpiresAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.TenantID != 41 || envelope.Data.DashboardUserID != 52 || envelope.Data.Version != 4 || envelope.Data.ActivationToken != "one-time-resend-token" {
		t.Fatalf("resend response tenant=%d user=%d version=%d activationPresent=%t, want one-time activation data", envelope.Data.TenantID, envelope.Data.DashboardUserID, envelope.Data.Version, envelope.Data.ActivationToken != "")
	}
	if envelope.Data.ActivationPath != "/activate#token=one-time-resend-token" || envelope.Data.ActivationExpiresAt != "2026-08-27T12:00:00Z" {
		t.Fatalf("resend delivery=%+v", envelope.Data)
	}

	store.resendResult = ResendActivationResult{TenantID: 41, DashboardUserID: 52, Version: 4, Idempotent: true}
	retry := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenants/41/activation/resend", bytes.NewBufferString(`{"targetUserId":52,"expectedVersion":3}`))
	retry.Header.Set("X-Request-ID", "resend-request-1")
	retry = retry.WithContext(saasauth.WithPrincipal(retry.Context(), saasauth.Principal{UserID: 700, AuthVersion: 9}))
	retryResponse := httptest.NewRecorder()
	handler.ServeHTTP(retryResponse, retry)
	if retryResponse.Code != http.StatusOK || bytes.Contains(retryResponse.Body.Bytes(), []byte("one-time-resend-token")) {
		t.Fatalf("retry status=%d bodyBytes=%d, raw resend token was replayed", retryResponse.Code, retryResponse.Body.Len())
	}
}

func TestDashboardAdminHTTPGovernanceUsesPathTenantAndMapsLastAdminConflict(t *testing.T) {
	store := &dashboardAdminHTTPStore{
		replaceResult: GovernanceResult{TenantID: 41, DashboardUserID: 63, Version: 5},
		statusErr:     ErrLastSuperAdmin,
	}
	handler := NewHTTPHandler(NewService(store))
	principalContext := func(request *http.Request) *http.Request {
		return request.WithContext(saasauth.WithPrincipal(request.Context(), saasauth.Principal{UserID: 700, AuthVersion: 9}))
	}
	replace := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenants/41/super-admin/replace", bytes.NewBufferString(`{"currentAdminId":52,"newAdminId":63,"expectedVersion":4}`))
	replace.Header.Set("X-Request-ID", "replace-request-1")
	replaceResponse := httptest.NewRecorder()
	handler.ServeHTTP(replaceResponse, principalContext(replace))
	if replaceResponse.Code != http.StatusOK || store.replaceCalls != 1 || store.seenReplace.TenantID != 41 || store.seenReplace.RequestID != "replace-request-1" || store.seenReplace.BodyTenantID != 0 {
		t.Fatalf("replace status=%d calls=%d input=%+v, want path-derived tenant and authenticated actor", replaceResponse.Code, store.replaceCalls, store.seenReplace)
	}
	status := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenants/41/super-admin/status", bytes.NewBufferString(`{"targetUserId":52,"enabled":false,"expectedVersion":5}`))
	status.Header.Set("X-Request-ID", "status-request-1")
	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, principalContext(status))
	if statusResponse.Code != http.StatusConflict || store.statusCalls != 1 || store.seenStatus.TenantID != 41 || store.seenStatus.RequestID != "status-request-1" {
		t.Fatalf("status route response=%d calls=%d input=%+v, want mapped last-admin conflict", statusResponse.Code, store.statusCalls, store.seenStatus)
	}
	var envelope struct {
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ErrorCode != "LAST_SUPER_ADMIN" {
		t.Fatalf("errorCode=%q, want LAST_SUPER_ADMIN", envelope.ErrorCode)
	}
}

func mustProvisionJSON(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(validProvisionInput())
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestDashboardAdminHTTPHighRiskApprovalRequiredRejectsAllDirectGovernanceWrites(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		wantAction string
	}{
		{name: "provision", path: "/dashboard/saasAdmin/tenants/provision", body: string(mustProvisionJSON(t)), wantAction: ApprovalActionTenantProvision},
		{name: "resend", path: "/dashboard/saasAdmin/tenants/41/activation/resend", body: `{"targetUserId":52,"expectedVersion":4}`, wantAction: ApprovalActionActivationResend},
		{name: "replace", path: "/dashboard/saasAdmin/tenants/41/super-admin/replace", body: `{"currentAdminId":52,"newAdminId":63,"expectedVersion":4}`, wantAction: ApprovalActionSuperAdminReplace},
		{name: "status", path: "/dashboard/saasAdmin/tenants/41/super-admin/status", body: `{"targetUserId":52,"enabled":false,"expectedVersion":4}`, wantAction: ApprovalActionSuperAdminStatus},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &dashboardAdminHTTPStore{}
			var seenAction string
			handler := NewHTTPHandler(NewService(store)).WithApprovalGate(func(_ context.Context, action string) (ApprovalGateResult, error) {
				seenAction = action
				return ApprovalGateResult{Required: true, ActionType: action, RequiredApprovals: 2}, nil
			})
			request := httptest.NewRequest(http.MethodPost, test.path, bytes.NewBufferString(test.body)).WithContext(saasauth.WithPrincipal(context.Background(), saasauth.Principal{UserID: 700, AuthVersion: 9}))
			request.Header.Set("X-Request-ID", "approval-gate-red")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusPreconditionRequired {
				t.Fatalf("status=%d body=%s, want 428", response.Code, response.Body.String())
			}
			if seenAction != test.wantAction {
				t.Fatalf("approval gate action=%q, want %q", seenAction, test.wantAction)
			}
			if store.provisionCalls != 0 || store.resendCalls != 0 || store.replaceCalls != 0 || store.statusCalls != 0 {
				t.Fatalf("direct write reached store: provision=%d resend=%d replace=%d status=%d", store.provisionCalls, store.resendCalls, store.replaceCalls, store.statusCalls)
			}
			var envelope struct {
				ErrorCode string `json:"errorCode"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.ErrorCode != codeApprovalRequired {
				t.Fatalf("errorCode=%q, want %q", envelope.ErrorCode, codeApprovalRequired)
			}
		})
	}
}

func TestDashboardAdminHTTPApprovalGateDisabledPreservesDirectWriteContract(t *testing.T) {
	store := &dashboardAdminHTTPStore{provisionResult: ProvisionResult{TenantID: 41, DashboardUserID: 52, BindingCorpID: 63}}
	handler := NewHTTPHandler(NewService(store)).WithApprovalGate(func(context.Context, string) (ApprovalGateResult, error) {
		return ApprovalGateResult{Required: false}, nil
	})
	request := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/tenants/provision", bytes.NewReader(mustProvisionJSON(t))).WithContext(saasauth.WithPrincipal(context.Background(), saasauth.Principal{UserID: 700, AuthVersion: 9}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || store.provisionCalls != 1 {
		t.Fatalf("status=%d calls=%d, want disabled policy to preserve direct write", response.Code, store.provisionCalls)
	}
}
