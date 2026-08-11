package dashboardauth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/authrealm"
	"jiyi/mochat-go/internal/dashboardprincipal"
)

type dashboardHTTPTestPersistence struct {
	identity            DashboardIdentity
	principal           dashboardprincipal.DashboardPrincipal
	activateDigest      [32]byte
	activateCalls       int
	authenticateCalls   int
	sessionCalls        int
	resetDigest         [32]byte
	resetCalls          int
	passwordChangeCalls int
}

func (p *dashboardHTTPTestPersistence) Authenticate(_ context.Context, loginIdentifier string) (DashboardIdentity, error) {
	p.authenticateCalls++
	if loginIdentifier != p.identity.LoginIdentifier {
		return DashboardIdentity{}, ErrIdentityNotFound
	}
	return p.identity, nil
}

func (p *dashboardHTTPTestPersistence) Activate(_ context.Context, digest [32]byte, _ string) error {
	p.activateCalls++
	if p.activateCalls > 1 || digest != p.activateDigest {
		return ErrActivationInvalid
	}
	return nil
}

func (p *dashboardHTTPTestPersistence) CheckSession(context.Context, int, uint64) error {
	return nil
}

func (p *dashboardHTTPTestPersistence) ResolvePrincipal(context.Context, int) (dashboardprincipal.DashboardPrincipal, error) {
	return p.principal, nil
}

func (p *dashboardHTTPTestPersistence) MFAStatus(context.Context, int) (int, error) {
	return DashboardMFAStatusActive, nil
}
func (p *dashboardHTTPTestPersistence) BeginMFAEnrollment(context.Context, int, uint64, [32]byte, time.Time, string, string) error {
	return nil
}
func (p *dashboardHTTPTestPersistence) CreateMFAChallenge(context.Context, int, uint64, string, [32]byte, time.Time) error {
	return nil
}
func (p *dashboardHTTPTestPersistence) FindMFAChallenge(context.Context, [32]byte) (DashboardMFAChallenge, error) {
	return DashboardMFAChallenge{}, ErrMFAChallengeInvalid
}
func (p *dashboardHTTPTestPersistence) RecordMFAFailure(context.Context, [32]byte) error { return nil }
func (p *dashboardHTTPTestPersistence) CompleteMFAChallenge(context.Context, [32]byte, int, uint64, string, int64) (DashboardIdentity, error) {
	return p.identity, nil
}
func (p *dashboardHTTPTestPersistence) CompletePasswordChange(context.Context, [32]byte, string) (DashboardIdentity, error) {
	p.passwordChangeCalls++
	return p.identity, nil
}
func (p *dashboardHTTPTestPersistence) CreateSession(context.Context, int, uint64, [32]byte, time.Time, time.Time) error {
	p.sessionCalls++
	return nil
}
func (p *dashboardHTTPTestPersistence) CheckSessionToken(context.Context, authrealm.Claims) error {
	return nil
}
func (p *dashboardHTTPTestPersistence) RevokeSession(context.Context, authrealm.Claims) error {
	return nil
}

func (p *dashboardHTTPTestPersistence) CreatePasswordReset(_ context.Context, _ int, _ uint64, digest [32]byte, _ time.Time) error {
	p.resetDigest = digest
	p.resetCalls++
	return nil
}
func (p *dashboardHTTPTestPersistence) CompletePasswordReset(context.Context, [32]byte, string) (DashboardIdentity, error) {
	return p.identity, nil
}

func dashboardHTTPTestConfig(p *dashboardHTTPTestPersistence) HTTPConfig {
	tokenConfig := authrealm.TokenConfig{
		Secret:   []byte("dashboard-test-secret"),
		Issuer:   "dashboard-test-issuer",
		Audience: "dashboard-test-audience",
		TTL:      time.Hour,
		Realm:    authrealm.RealmDashboard,
		Prefix:   "dashboard_test_",
	}
	return HTTPConfig{
		Service:     NewService(p),
		Persistence: p,
		Signer:      tokenConfig,
		Parser:      authrealm.Parser{Config: tokenConfig, ValidateSession: p.CheckSessionToken},
		MFAKey:      []byte("01234567890123456789012345678901"),
		MFAKeyID:    "dashboard-test-mfa",
		TenantGate: func(_ context.Context, tenantID int, _ time.Time) (TenantAccess, error) {
			return TenantAccess{TenantID: tenantID, Allowed: true}, nil
		},
	}
}

func TestDashboardLoginUsesDashboardIdentityAndServerPrincipalOnly(t *testing.T) {
	p := &dashboardHTTPTestPersistence{
		identity:  DashboardIdentity{UserID: 7, LoginIdentifier: "13800000000", PasswordHash: mustDashboardPasswordHash(t, "secret"), Status: DashboardIdentityStatusActive, AuthVersion: 4},
		principal: dashboardprincipal.DashboardPrincipal{UserID: 7, TenantID: 902, CorpID: 77, CorpStatus: dashboardprincipal.CorpBindingStatusPending, AuthVersion: 4},
	}
	handler, err := NewHTTPHandler(dashboardHTTPTestConfig(p))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800000000","password":"secret"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || p.authenticateCalls != 1 || p.sessionCalls != 1 {
		t.Fatalf("status=%d authenticateCalls=%d sessionCalls=%d bodyBytes=%d", response.Code, p.authenticateCalls, p.sessionCalls, response.Body.Len())
	}
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("login response was not valid JSON: %v", err)
	}
	for _, field := range []string{"tenantId", "corpId", "actorId", "isSuperAdmin"} {
		if _, ok := envelope.Data[field]; ok {
			t.Fatalf("login response included server-bound field %s", field)
		}
	}
}

func TestDashboardLoginRejectsClientTenantCorpAndActorFields(t *testing.T) {
	p := &dashboardHTTPTestPersistence{identity: DashboardIdentity{UserID: 7, LoginIdentifier: "13800000000", PasswordHash: mustDashboardPasswordHash(t, "secret"), Status: DashboardIdentityStatusActive, AuthVersion: 4}}
	handler, err := NewHTTPHandler(dashboardHTTPTestConfig(p))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800000000","password":"secret","tenantId":902,"corpId":77,"actorId":8}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || p.authenticateCalls != 0 || p.sessionCalls != 0 {
		t.Fatalf("status=%d authenticateCalls=%d sessionCalls=%d bodyBytes=%d", response.Code, p.authenticateCalls, p.sessionCalls, response.Body.Len())
	}
}

func TestDashboardTokenRejectsSaaSToken(t *testing.T) {
	p := &dashboardHTTPTestPersistence{}
	config := dashboardHTTPTestConfig(p)
	handler, err := NewHTTPHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	saasToken, err := authrealm.Sign(authrealm.TokenConfig{
		Secret: []byte("saas-test-secret"), Issuer: "saas-test-issuer", Audience: "saas-test-audience", TTL: time.Hour,
		Realm: authrealm.RealmSaaSAdmin, Prefix: "saas_test_",
	}, authrealm.Claims{UserID: 7, AuthVersion: 1}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/dashboard/auth/session", nil)
	request.Header.Set("Authorization", "Bearer "+saasToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), `"token"`) {
		t.Fatalf("status=%d bodyBytes=%d", response.Code, response.Body.Len())
	}
}

func TestDashboardActivationUsesDigestAndConsumesOnce(t *testing.T) {
	const rawToken = "dashboard-activation-token"
	p := &dashboardHTTPTestPersistence{activateDigest: sha256.Sum256([]byte(rawToken))}
	handler, err := NewHTTPHandler(dashboardHTTPTestConfig(p))
	if err != nil {
		t.Fatal(err)
	}
	post := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/dashboard/auth/activate", strings.NewReader(`{"activationToken":"`+rawToken+`","password":"new-secret"}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	first := post()
	if first.Code != http.StatusNoContent || p.activateCalls != 1 {
		t.Fatalf("first activation status=%d calls=%d bodyBytes=%d", first.Code, p.activateCalls, first.Body.Len())
	}
	if strings.Contains(first.Body.String(), rawToken) {
		t.Fatalf("activation token was echoed in a %d-byte response", first.Body.Len())
	}
	second := post()
	if second.Code == http.StatusNoContent {
		t.Fatalf("second activation unexpectedly succeeded: status=%d bodyBytes=%d", second.Code, second.Body.Len())
	}
}

func TestDashboardRequestGuardKeepsResetRequestAuthenticated(t *testing.T) {
	p := &dashboardHTTPTestPersistence{
		principal: dashboardprincipal.DashboardPrincipal{
			UserID:      7,
			TenantID:    902,
			CorpID:      77,
			CorpStatus:  dashboardprincipal.CorpBindingStatusPending,
			AuthVersion: 4,
		},
	}
	config := dashboardHTTPTestConfig(p)
	guard, err := NewRequestGuard(config.Parser, p, config.TenantGate)
	if err != nil {
		t.Fatal(err)
	}
	noTokenRequest := httptest.NewRequest(http.MethodPost, "/dashboard/auth/password/reset-request", strings.NewReader(`{}`))
	noTokenResponse := httptest.NewRecorder()
	if guard.Authorize(noTokenResponse, noTokenRequest) || noTokenResponse.Code != http.StatusUnauthorized {
		t.Fatalf("reset-request without bearer was not rejected: status=%d bodyBytes=%d", noTokenResponse.Code, noTokenResponse.Body.Len())
	}

	now := time.Now().UTC()
	token, err := authrealm.Sign(config.Signer, authrealm.Claims{UserID: 7, AuthVersion: 4}, now)
	if err != nil {
		t.Fatal(err)
	}

	deniedConfig := config
	deniedConfig.TenantGate = func(_ context.Context, tenantID int, _ time.Time) (TenantAccess, error) {
		return TenantAccess{TenantID: tenantID, Allowed: false}, nil
	}
	deniedGuard, err := NewRequestGuard(deniedConfig.Parser, p, deniedConfig.TenantGate)
	if err != nil {
		t.Fatal(err)
	}
	deniedRequest := httptest.NewRequest(http.MethodPost, "/dashboard/auth/password/reset-request", strings.NewReader(`{}`))
	deniedRequest.Header.Set("Authorization", "Bearer "+token)
	deniedResponse := httptest.NewRecorder()
	if deniedGuard.Authorize(deniedResponse, deniedRequest) || deniedResponse.Code != http.StatusForbidden {
		t.Fatalf("reset-request with denied tenant was not rejected: status=%d bodyBytes=%d", deniedResponse.Code, deniedResponse.Body.Len())
	}

	validRequest := httptest.NewRequest(http.MethodPost, "/dashboard/auth/password/reset-request", strings.NewReader(`{}`))
	validRequest.Header.Set("Authorization", "Bearer "+token)
	validResponse := httptest.NewRecorder()
	if !guard.Authorize(validResponse, validRequest) || validRequest.Header.Get("X-Mochat-Go-User-ID") != "7" {
		t.Fatalf("valid reset-request session was not authorized: status=%d user=%q bodyBytes=%d", validResponse.Code, validRequest.Header.Get("X-Mochat-Go-User-ID"), validResponse.Body.Len())
	}

	handler, err := NewHTTPHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/dashboard/auth/password/reset-request", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || p.resetCalls != 1 {
		t.Fatalf("authenticated reset-request status=%d resetCalls=%d bodyBytes=%d", response.Code, p.resetCalls, response.Body.Len())
	}
	var envelope struct {
		Data struct {
			ResetToken string `json:"resetToken"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ResetToken == "" {
		t.Fatalf("authenticated reset-request did not return a one-time token: bodyBytes=%d", response.Body.Len())
	}
	if expected := sha256.Sum256([]byte(envelope.Data.ResetToken)); p.resetDigest != expected {
		t.Fatalf("reset persistence did not receive token digest: got=%x want=%x", p.resetDigest, expected)
	}
}

func TestDashboardRequestGuardSeparatesPublicAndAuthenticatedIdentityRoutes(t *testing.T) {

	p := &dashboardHTTPTestPersistence{
		principal: dashboardprincipal.DashboardPrincipal{
			UserID:      7,
			TenantID:    902,
			CorpID:      77,
			CorpStatus:  dashboardprincipal.CorpBindingStatusPending,
			AuthVersion: 4,
		},
	}
	config := dashboardHTTPTestConfig(p)
	guard, err := NewRequestGuard(config.Parser, p, config.TenantGate)
	if err != nil {
		t.Fatal(err)
	}

	for _, route := range []struct {
		method string
		path   string
		public bool
	}{
		{method: http.MethodPost, path: "/dashboard/user/auth", public: true},
		{method: http.MethodPost, path: "/dashboard/user/authMFA", public: true},
		{method: http.MethodPost, path: "/dashboard/auth/activate", public: true},
		{method: http.MethodPost, path: "/dashboard/auth/password/reset", public: true},
		{method: http.MethodPost, path: "/dashboard/auth/password/reset-request", public: false},
		{method: http.MethodGet, path: "/dashboard/auth/session", public: false},
		{method: http.MethodPost, path: "/dashboard/auth/logout", public: false},
		{method: http.MethodPut, path: "/dashboard/user/logout", public: false},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
			response := httptest.NewRecorder()
			allowed := guard.Authorize(response, request)
			if route.public {
				if !allowed {
					t.Fatalf("identity-public route was blocked: status=%d bodyBytes=%d", response.Code, response.Body.Len())
				}
				return
			}
			if allowed || response.Code != http.StatusUnauthorized {
				t.Fatalf("identity-authenticated route bypassed identity guard: status=%d bodyBytes=%d", response.Code, response.Body.Len())
			}
		})
	}
}

func TestDashboardPasswordChangeChallengeCompletesBeforeSession(t *testing.T) {
	p := &dashboardHTTPTestPersistence{
		identity: DashboardIdentity{
			UserID: 7, LoginIdentifier: "13800000000", Status: DashboardIdentityStatusActive,
			AuthVersion: 5, MFARequired: 1,
		},
		principal: dashboardprincipal.DashboardPrincipal{
			UserID: 7, TenantID: 902, CorpID: 77,
			CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 5,
		},
	}
	handler, err := NewHTTPHandler(dashboardHTTPTestConfig(p))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/dashboard/user/authMFA", strings.NewReader(`{"passwordChangeToken":"password-change-token","newPassword":"rotated-secret"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || p.passwordChangeCalls != 1 || p.sessionCalls != 1 {
		t.Fatalf("status=%d passwordChangeCalls=%d sessionCalls=%d bodyBytes=%d", response.Code, p.passwordChangeCalls, p.sessionCalls, response.Body.Len())
	}
	if strings.Contains(response.Body.String(), "rotated-secret") || !strings.Contains(response.Body.String(), `"token"`) {
		t.Fatalf("password change response leaked password or omitted session: bodyBytes=%d", response.Body.Len())
	}
}

func mustDashboardPasswordHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

var _ DashboardAuthPersistence = (*dashboardHTTPTestPersistence)(nil)
