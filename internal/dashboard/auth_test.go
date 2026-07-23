package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/authjwt"
)

func TestAuthReturnsPHPCompatibleToken(t *testing.T) {
	passwordHash, err := authjwt.GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeAuthStore{
		user: AuthUser{ID: 7, Status: 1, Password: passwordHash},
	}
	handler := NewAuthHandler(store, "secret", time.Hour)
	handler.now = func() time.Time { return time.Unix(1000, 0) }

	req := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", bytes.NewBufferString(`{"phone":"13800138000","password":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			Token  string `json:"token"`
			Expire int64  `json:"expire"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 200 || body.Data.Expire != 3600 {
		t.Fatalf("body = %+v", body)
	}
	if body.Data.Token == "" {
		t.Fatalf("token is empty")
	}
	parsed, err := authjwt.Parser{
		Secret:        "secret",
		SkipBlacklist: true,
		Now:           func() time.Time { return time.Unix(1200, 0) },
	}.Parse(context.Background(), body.Data.Token)
	if err != nil {
		t.Fatal(err)
	}
	uid, ok := authjwt.UserIDFromPayload(parsed)
	if !ok || uid != 7 {
		t.Fatalf("uid = %d ok=%v", uid, ok)
	}
}

func TestAuthRejectsBadPassword(t *testing.T) {
	passwordHash, err := authjwt.GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAuthHandler(&fakeAuthStore{
		user: AuthUser{ID: 7, Status: 1, Password: passwordHash},
	}, "secret", time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800138000","password":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthRejectsDisabledUser(t *testing.T) {
	passwordHash, err := authjwt.GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAuthHandler(&fakeAuthStore{
		user: AuthUser{ID: 7, Status: 2, Password: passwordHash},
	}, "secret", time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800138000","password":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAuthRejectsDisabledTenant(t *testing.T) {
	passwordHash, err := authjwt.GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAuthHandler(&fakeAuthStore{
		user: AuthUser{ID: 7, Status: 1, Password: passwordHash, TenantID: 902, TenantStatus: 2},
	}, "secret", time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800138000","password":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var body struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Msg != "租户已停用" {
		t.Fatalf("msg = %q", body.Msg)
	}
}

func TestAuthRejectsExpiredTenantPackage(t *testing.T) {
	passwordHash, err := authjwt.GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAuthHandler(&fakeAuthStore{
		user: AuthUser{ID: 7, Status: 1, Password: passwordHash, TenantID: 902, TenantPackageExpired: true, TenantPackageExpiresAt: "2026-07-01 00:00:00"},
	}, "secret", time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800138000","password":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var body struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Msg != "租户套餐已到期" {
		t.Fatalf("msg = %q", body.Msg)
	}
}

func TestAuthUsesManagedSubscriptionAccess(t *testing.T) {
	passwordHash, err := authjwt.GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name       string
		status     string
		allowed    bool
		wantStatus int
		wantMsg    string
	}{
		{name: "grace allowed", status: SaaSAdminSubscriptionStatusGrace, allowed: true, wantStatus: http.StatusOK},
		{name: "past due blocked", status: SaaSAdminSubscriptionStatusPastDue, allowed: false, wantStatus: http.StatusForbidden, wantMsg: "租户订阅已欠费"},
		{name: "canceled blocked", status: SaaSAdminSubscriptionStatusCanceled, allowed: false, wantStatus: http.StatusForbidden, wantMsg: "租户订阅已取消"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := NewAuthHandler(&fakeAuthStore{user: AuthUser{
				ID: 7, Status: 1, Password: passwordHash, TenantID: 902,
				TenantSubscriptionManaged: true, TenantSubscriptionStatus: test.status,
				TenantSubscriptionAccessAllowed: test.allowed,
			}}, "secret", time.Hour)
			req := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800138000","password":"123456"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != test.wantStatus {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if test.wantMsg != "" && !strings.Contains(rec.Body.String(), test.wantMsg) {
				t.Fatalf("body=%s", rec.Body.String())
			}
		})
	}
}

func TestAuthAcceptsFormBody(t *testing.T) {
	passwordHash, err := authjwt.GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAuthHandler(&fakeAuthStore{
		user: AuthUser{ID: 7, Status: 1, Password: passwordHash},
	}, "secret", time.Hour)
	handler.now = func() time.Time { return time.Unix(1000, 0) }

	req := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader("phone=13800138000&password=123456"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthAcceptsURLEncodedBodyWithoutContentType(t *testing.T) {
	passwordHash, err := authjwt.GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAuthHandler(&fakeAuthStore{
		user: AuthUser{ID: 7, Status: 1, Password: passwordHash},
	}, "secret", time.Hour)
	handler.now = func() time.Time { return time.Unix(1000, 0) }

	req := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader("phone=13800138000&password=123456"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

type fakeAuthStore struct {
	user         AuthUser
	tenantUser   AuthUser
	globalCalls  int
	tenantCalls  int
	lastTenantID int
}

func (s *fakeAuthStore) UserAuthByPhone(context.Context, string) (AuthUser, bool, error) {
	s.globalCalls++
	return s.user, s.user.ID != 0, nil
}

func (s *fakeAuthStore) UserAuthByTenantPhone(_ context.Context, tenantID int, _ string) (AuthUser, bool, error) {
	s.tenantCalls++
	s.lastTenantID = tenantID
	return s.tenantUser, s.tenantUser.ID != 0, nil
}

type fakeAuthTenantDomainReader struct {
	domains map[string]SaaSTenantDomain
}

func (reader fakeAuthTenantDomainReader) SaaSTenantDomainByHostname(_ context.Context, hostname string) (SaaSTenantDomain, bool, error) {
	domain, ok := reader.domains[hostname]
	return domain, ok, nil
}

func TestAuthCustomDomainConstrainsTenantLookup(t *testing.T) {
	passwordHash, err := authjwt.GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeAuthStore{
		user:       AuthUser{ID: 1, TenantID: 1, Status: 1, Password: passwordHash},
		tenantUser: AuthUser{ID: 9, TenantID: 9, Status: 1, Password: passwordHash},
	}
	domain := SaaSTenantDomain{TenantID: 9, Hostname: "login.customer.example.com", Status: SaaSTenantDomainStatusActive, VerifiedAt: "2026-07-11 20:00:00"}
	handler := NewAuthHandler(store, "secret", time.Hour).WithTenantDomains(fakeAuthTenantDomainReader{domains: map[string]SaaSTenantDomain{domain.Hostname: domain}})
	request := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800138000","password":"123456"}`))
	request.Host = "login.customer.example.com"
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || store.tenantCalls != 1 || store.globalCalls != 0 || store.lastTenantID != 9 {
		t.Fatalf("status=%d tenantCalls=%d globalCalls=%d tenant=%d body=%s", response.Code, store.tenantCalls, store.globalCalls, store.lastTenantID, response.Body.String())
	}
}

func TestAuthCustomDomainDoesNotFallBackToGlobalPhone(t *testing.T) {
	passwordHash, err := authjwt.GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeAuthStore{user: AuthUser{ID: 1, TenantID: 1, Status: 1, Password: passwordHash}}
	domain := SaaSTenantDomain{TenantID: 9, Hostname: "login.customer.example.com", Status: SaaSTenantDomainStatusActive, VerifiedAt: "2026-07-11 20:00:00"}
	handler := NewAuthHandler(store, "secret", time.Hour).WithTenantDomains(fakeAuthTenantDomainReader{domains: map[string]SaaSTenantDomain{domain.Hostname: domain}})
	request := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800138000","password":"123456"}`))
	request.Host = domain.Hostname
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || store.tenantCalls != 1 || store.globalCalls != 0 {
		t.Fatalf("status=%d tenantCalls=%d globalCalls=%d body=%s", response.Code, store.tenantCalls, store.globalCalls, response.Body.String())
	}
}

func TestAuthRegisteredInactiveDomainFailsClosed(t *testing.T) {
	store := &fakeAuthStore{}
	domain := SaaSTenantDomain{TenantID: 9, Hostname: "login.customer.example.com", Status: SaaSTenantDomainStatusPending}
	handler := NewAuthHandler(store, "secret", time.Hour).WithTenantDomains(fakeAuthTenantDomainReader{domains: map[string]SaaSTenantDomain{domain.Hostname: domain}})
	request := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", strings.NewReader(`{"phone":"13800138000","password":"123456"}`))
	request.Host = domain.Hostname
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMisdirectedRequest || store.tenantCalls != 0 || store.globalCalls != 0 {
		t.Fatalf("status=%d tenantCalls=%d globalCalls=%d body=%s", response.Code, store.tenantCalls, store.globalCalls, response.Body.String())
	}
}
