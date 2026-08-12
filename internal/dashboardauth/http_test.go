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
	appconfig "jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	compatserver "jiyi/mochat-go/internal/server"
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
	mfaStatus           int
	mfaStatusSet        bool
}

type dashboardHTTPTestPrincipalResolver struct {
	persistence *dashboardHTTPTestPersistence
	denied      bool
}

func (resolver dashboardHTTPTestPrincipalResolver) ResolveUser(_ context.Context, userID int, _ time.Time) (dashboardprincipal.DashboardPrincipal, error) {
	if resolver.denied {
		return dashboardprincipal.DashboardPrincipal{}, dashboardprincipal.ErrTenantAccessDenied
	}
	if resolver.persistence == nil || resolver.persistence.principal.UserID != userID {
		return dashboardprincipal.DashboardPrincipal{}, dashboardprincipal.ErrPrincipalUnavailable
	}
	return resolver.persistence.principal, nil
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
	if p.mfaStatusSet {
		return p.mfaStatus, nil
	}
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
		Service:           NewService(p),
		Persistence:       p,
		MFARequired:       true,
		Signer:            tokenConfig,
		Parser:            authrealm.Parser{Config: tokenConfig, ValidateSession: p.CheckSessionToken},
		MFAKey:            []byte("01234567890123456789012345678901"),
		MFAKeyID:          "dashboard-test-mfa",
		PrincipalResolver: dashboardHTTPTestPrincipalResolver{persistence: p},
		TenantGate: func(_ context.Context, tenantID int, _ time.Time) (TenantAccess, error) {
			return TenantAccess{TenantID: tenantID, Allowed: true}, nil
		},
	}
}

func TestDashboardAuthMFARequirementOffIssuesTokenEvenWithActiveCredential(t *testing.T) {
	p := &dashboardHTTPTestPersistence{
		identity:  DashboardIdentity{UserID: 19, LoginIdentifier: "13900000019", PasswordHash: mustDashboardPasswordHash(t, "dashboard-mfa-off-password"), Status: DashboardIdentityStatusActive, AuthVersion: 3, MFARequired: 1},
		principal: dashboardprincipal.DashboardPrincipal{UserID: 19, TenantID: 902, CorpID: 77, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 3},
	}
	config := dashboardHTTPTestConfig(p)
	config.MFARequired = false
	handler, err := NewHTTPHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13900000019","password":"dashboard-mfa-off-password"}`)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"token"`) || p.sessionCalls != 1 {
		t.Fatalf("MFA-off login status=%d sessionCalls=%d bodyBytes=%d", response.Code, p.sessionCalls, response.Body.Len())
	}
}

func TestDashboardAuthMFARequirementOnKeepsLoginChallenge(t *testing.T) {
	p := &dashboardHTTPTestPersistence{
		identity:  DashboardIdentity{UserID: 20, LoginIdentifier: "13900000020", PasswordHash: mustDashboardPasswordHash(t, "dashboard-mfa-on-password"), Status: DashboardIdentityStatusActive, AuthVersion: 4, MFARequired: 1},
		principal: dashboardprincipal.DashboardPrincipal{UserID: 20, TenantID: 902, CorpID: 77, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 4},
	}
	config := dashboardHTTPTestConfig(p)
	config.MFARequired = true
	handler, err := NewHTTPHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13900000020","password":"dashboard-mfa-on-password"}`)))
	if response.Code != http.StatusAccepted || p.sessionCalls != 0 || !strings.Contains(response.Body.String(), `"challengeToken"`) {
		t.Fatalf("MFA-on login status=%d sessionCalls=%d bodyBytes=%d", response.Code, p.sessionCalls, response.Body.Len())
	}
}

func TestDashboardAuthMFARequirementOnKeepsEnrollment(t *testing.T) {
	p := &dashboardHTTPTestPersistence{
		identity:     DashboardIdentity{UserID: 21, LoginIdentifier: "13900000021", PasswordHash: mustDashboardPasswordHash(t, "dashboard-mfa-enrollment-password"), Status: DashboardIdentityStatusActive, AuthVersion: 4, MFARequired: 1},
		principal:    dashboardprincipal.DashboardPrincipal{UserID: 21, TenantID: 902, CorpID: 77, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 4},
		mfaStatus:    DashboardMFAStatusPending,
		mfaStatusSet: true,
	}
	config := dashboardHTTPTestConfig(p)
	config.MFARequired = true
	handler, err := NewHTTPHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13900000021","password":"dashboard-mfa-enrollment-password"}`)))
	if response.Code != http.StatusAccepted || p.sessionCalls != 0 || !strings.Contains(response.Body.String(), `"enrollmentToken"`) {
		t.Fatalf("MFA-on enrollment status=%d sessionCalls=%d bodyBytes=%d", response.Code, p.sessionCalls, response.Body.Len())
	}
}

func TestDashboardIdentityCompositionRequiresPrincipalResolver(t *testing.T) {
	p := &dashboardHTTPTestPersistence{principal: dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: 902, CorpID: 77,
		CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 4,
	}}
	config := dashboardHTTPTestConfig(p)
	config.PrincipalResolver = nil
	if _, err := NewHTTPHandler(config); err == nil {
		t.Fatal("HTTP handler accepted missing Dashboard principal resolver")
	}
	guard, err := NewRequestGuard(config.Parser, p, config.TenantGate)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	token, err := authrealm.Sign(config.Signer, authrealm.Claims{UserID: 7, AuthVersion: 4}, now)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/dashboard/index", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	if guard.Authorize(response, request) {
		t.Fatal("request guard authorized without Dashboard principal resolver")
	}
	if response.Code != http.StatusServiceUnavailable && response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
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
	guard.WithPrincipalResolver(config.PrincipalResolver)
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
	deniedConfig.PrincipalResolver = dashboardHTTPTestPrincipalResolver{persistence: p, denied: true}
	deniedGuard, err := NewRequestGuard(deniedConfig.Parser, p, deniedConfig.TenantGate)
	if err != nil {
		t.Fatal(err)
	}
	deniedGuard.WithPrincipalResolver(deniedConfig.PrincipalResolver)
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
	guard.WithPrincipalResolver(config.PrincipalResolver)

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

func TestDashboardRequestGuardUsesExplicitPublicDashboardContracts(t *testing.T) {
	p := &dashboardHTTPTestPersistence{
		principal: dashboardprincipal.DashboardPrincipal{
			UserID: 7, TenantID: 902, CorpID: 77,
			CorpStatus: dashboardprincipal.CorpBindingStatusPending, AuthVersion: 4,
		},
	}
	config := dashboardHTTPTestConfig(p)
	guard, err := NewRequestGuard(config.Parser, p, config.TenantGate)
	if err != nil {
		t.Fatal(err)
	}
	guard.WithPrincipalResolver(config.PrincipalResolver)
	guard.WithPublicRouteContracts(dashboard.PublicDashboardRouteContracts())

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/dashboard/corp/weWorkCallback"},
		{method: http.MethodPost, path: "/dashboard/corp/weWorkCallback"},
		{method: http.MethodGet, path: "/dashboard/officialAccount/authEventCallback"},
		{method: http.MethodPost, path: "/dashboard/officialAccount/authEventCallback"},
		{method: http.MethodGet, path: "/dashboard/officialAccount/authRedirect/"},
		{method: http.MethodPost, path: "/dashboard/officialAccount/authRedirect/"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, strings.NewReader("callback"))
			response := httptest.NewRecorder()
			if !guard.Authorize(response, request) {
				t.Fatalf("exact callback was blocked: status=%d bodyBytes=%d", response.Code, response.Body.Len())
			}
			businessCalls := 0
			businessHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				businessCalls++
				w.WriteHeader(http.StatusAccepted)
			})
			businessResponse := httptest.NewRecorder()
			businessHandler.ServeHTTP(businessResponse, request)
			if businessCalls != 1 || businessResponse.Code != http.StatusAccepted {
				t.Fatalf("callback business handler was not reached: calls=%d status=%d", businessCalls, businessResponse.Code)
			}
		})
	}

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPut, path: "/dashboard/corp/weWorkCallback"},
		{method: http.MethodGet, path: "/dashboard/corp/weWorkCallback/extra"},
		{method: http.MethodGet, path: "/dashboard/officialAccount/authRedirect"},
		{method: http.MethodPost, path: "/dashboard/auth/password/reset-request"},
		{method: http.MethodGet, path: "/dashboard/auth/session"},
		{method: http.MethodPost, path: "/dashboard/auth/logout"},
	} {
		t.Run("reject "+route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			response := httptest.NewRecorder()
			allowed := guard.Authorize(response, request)
			if allowed || response.Code != http.StatusUnauthorized {
				t.Fatalf("non-contract route bypassed identity guard: allowed=%v status=%d bodyBytes=%d", allowed, response.Code, response.Body.Len())
			}
		})
	}
}

func TestDashboardRequestGuardLetsExactCallbacksReachServerHandlers(t *testing.T) {
	p := &dashboardHTTPTestPersistence{}
	config := dashboardHTTPTestConfig(p)
	guard, err := NewRequestGuard(config.Parser, p, config.TenantGate)
	if err != nil {
		t.Fatal(err)
	}
	guard.WithPrincipalResolver(config.PrincipalResolver)
	guard.WithPublicRouteContracts(dashboard.PublicDashboardRouteContracts())
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Method + " " + r.URL.Path))
	})
	server, err := compatserver.New(appconfig.Config{Standalone: true},
		compatserver.WithDashboardRequestGuard(guard),
		compatserver.WithWeWorkCallbackHandler(handler),
		compatserver.WithOfficialAccountAuthEventCallbackHandler(handler),
		compatserver.WithOfficialAccountAuthRedirectHandler(handler),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/dashboard/corp/weWorkCallback"},
		{method: http.MethodPost, path: "/dashboard/corp/weWorkCallback"},
		{method: http.MethodGet, path: "/dashboard/officialAccount/authEventCallback"},
		{method: http.MethodPost, path: "/dashboard/officialAccount/authEventCallback"},
		{method: http.MethodGet, path: "/dashboard/officialAccount/authRedirect/"},
		{method: http.MethodPost, path: "/dashboard/officialAccount/authRedirect/"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Body.String() != route.method+" "+route.path {
				t.Fatalf("exact callback did not reach business handler: status=%d bodyBytes=%d", response.Code, response.Body.Len())
			}
		})
	}

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPut, path: "/dashboard/corp/weWorkCallback"},
		{method: http.MethodGet, path: "/dashboard/corp/weWorkCallback/extra"},
		{method: http.MethodGet, path: "/dashboard/officialAccount/authRedirect"},
		{method: http.MethodPost, path: "/dashboard/auth/password/reset-request"},
		{method: http.MethodGet, path: "/dashboard/auth/session"},
		{method: http.MethodPost, path: "/dashboard/auth/logout"},
	} {
		t.Run("reject "+route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized || response.Body.String() == route.method+" "+route.path {
				t.Fatalf("non-contract route bypassed identity guard: status=%d bodyBytes=%d", response.Code, response.Body.Len())
			}
		})
	}
}

func TestDashboardPasswordChangePendingUsesAcceptedEnvelopeAndExpiry(t *testing.T) {
	p := &dashboardHTTPTestPersistence{
		identity: DashboardIdentity{
			UserID: 7, LoginIdentifier: "13800000000", PasswordHash: mustDashboardPasswordHash(t, "secret"),
			Status: DashboardIdentityStatusActive, AuthVersion: 5, MustRotatePassword: 1,
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
	request := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800000000","password":"secret"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || p.sessionCalls != 0 {
		t.Fatalf("password change pending status=%d sessionCalls=%d bodyBytes=%d", response.Code, p.sessionCalls, response.Body.Len())
	}
	var envelope struct {
		ErrorCode string `json:"errorCode"`
		Data      struct {
			PasswordChangeToken string `json:"passwordChangeToken"`
			ExpiresAt           int64  `json:"expiresAt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ErrorCode != CodePasswordChangeRequired || envelope.Data.PasswordChangeToken == "" || envelope.Data.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("password change pending envelope missing machine code/token/expiry: errorCode=%q tokenPresent=%t expiresAt=%d", envelope.ErrorCode, envelope.Data.PasswordChangeToken != "", envelope.Data.ExpiresAt)
	}
}

func TestDashboardSuspendedBindingCannotIssueOrUseDashboardToken(t *testing.T) {
	p := &dashboardHTTPTestPersistence{
		identity: DashboardIdentity{
			UserID: 7, LoginIdentifier: "13800000000", PasswordHash: mustDashboardPasswordHash(t, "secret"),
			Status: DashboardIdentityStatusActive, AuthVersion: 5,
		},
		principal: dashboardprincipal.DashboardPrincipal{
			UserID: 7, TenantID: 902, CorpID: 77,
			CorpStatus: dashboardprincipal.CorpBindingStatusSuspended, AuthVersion: 5,
		},
	}
	config := dashboardHTTPTestConfig(p)
	handler, err := NewHTTPHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	login := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800000000","password":"secret"}`))
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusForbidden || p.sessionCalls != 0 {
		t.Fatalf("suspended binding issued a token: status=%d sessionCalls=%d bodyBytes=%d", loginResponse.Code, p.sessionCalls, loginResponse.Body.Len())
	}

	guard, err := NewRequestGuard(config.Parser, p, config.TenantGate)
	if err != nil {
		t.Fatal(err)
	}
	guard.WithPrincipalResolver(config.PrincipalResolver)
	token, err := authrealm.Sign(config.Signer, authrealm.Claims{UserID: 7, AuthVersion: 5, JWTID: "suspended-jti"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/dashboard/index", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	if guard.Authorize(response, request) || response.Code != http.StatusForbidden {
		t.Fatalf("suspended binding bypassed request guard: status=%d bodyBytes=%d", response.Code, response.Body.Len())
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
