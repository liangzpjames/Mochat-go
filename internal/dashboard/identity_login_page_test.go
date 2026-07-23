package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeIdentityBrandingReader struct {
	profile  SaaSBrandingProfile
	tenantID int
}

type fakeIdentityDomainReader struct {
	domain SaaSTenantDomain
	found  bool
	err    error
	host   string
}

func (r *fakeIdentityDomainReader) SaaSTenantDomainByHostname(_ context.Context, hostname string) (SaaSTenantDomain, bool, error) {
	r.host = hostname
	return r.domain, r.found, r.err
}

func (r *fakeIdentityBrandingReader) SaaSBrandingProfile(_ context.Context, tenantID int) (SaaSBrandingProfile, error) {
	r.tenantID = tenantID
	return r.profile, nil
}

func TestIdentityLoginPageIncludesPasswordMFAAndFrontendTokenCompatibility(t *testing.T) {
	rec := httptest.NewRecorder()
	NewIdentityLoginPageHandler(nil, 1).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/security/login", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, want := range []string{
		`id="password-form"`,
		`id="mfa-form"`,
		`fetch('/dashboard/user/auth'`,
		`fetch('/dashboard/user/authMFA'`,
		`localStorage.setItem('ACCESS_TOKEN', JSON.stringify(authorization))`,
		`localStorage.setItem('mochat_go_saas_admin_token', authorization)`,
		`SameSite=Lax`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("page missing %q", want)
		}
	}
}

func TestIdentityLoginPageHeadHasNoBody(t *testing.T) {
	rec := httptest.NewRecorder()
	NewIdentityLoginPageHandler(nil, 1).ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/security/login", nil))
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestIdentityLoginPagePrefillsCredentialsOnlyOnLoopback(t *testing.T) {
	handler := NewIdentityLoginPageHandlerWithDomainsAndPrefill(nil, nil, 1, " 13800000090 ", "secret090")
	for _, host := range []string{"127.0.0.1:18090", "localhost:18090", "[::1]:18090"} {
		req := httptest.NewRequest(http.MethodGet, "/security/login", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		for _, want := range []string{`value="13800000090"`, `value="secret090"`} {
			if !strings.Contains(rec.Body.String(), want) {
				t.Fatalf("host %q page missing %q", host, want)
			}
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/security/login", nil)
	req.Host = "login.customer.example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	for _, secret := range []string{"13800000090", "secret090"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("customer domain leaked prefill value %q", secret)
		}
	}
}

func TestIdentityLoginPageRendersValidatedTenantBranding(t *testing.T) {
	reader := &fakeIdentityBrandingReader{profile: SaaSBrandingProfile{
		TenantID: 9, Status: SaaSBrandingStatusActive, ProductName: "客户云", ProductSubtitle: "私域运营平台",
		LogoURL: "/img/customer-logo.png", FaviconURL: "/favicon.ico", LoginBackgroundURL: "/static/customer-login.png",
		PrimaryColor: "#123456", AccentColor: "#234567",
	}}
	rec := httptest.NewRecorder()
	NewIdentityLoginPageHandler(reader, 9).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/security/login", nil))
	if rec.Code != http.StatusOK || reader.tenantID != 9 {
		t.Fatalf("status=%d tenant=%d", rec.Code, reader.tenantID)
	}
	for _, want := range []string{"客户云", "私域运营平台", "/img/customer-logo.png", "/static/customer-login.png", "--blue:#123456", "--blue-hover:#234567"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("page missing %q", want)
		}
	}
	if strings.Contains(rec.Body.String(), "ZgotmplZ") {
		t.Fatalf("template rejected validated branding: %s", rec.Body.String())
	}
}

func TestIdentityLoginPageResolvesTenantBrandingFromCustomDomain(t *testing.T) {
	branding := &fakeIdentityBrandingReader{profile: SaaSBrandingProfile{
		TenantID: 23, Status: SaaSBrandingStatusActive, ProductName: "域名客户云",
	}}
	domains := &fakeIdentityDomainReader{found: true, domain: SaaSTenantDomain{
		TenantID: 23, Hostname: "login.customer.example.com", Status: SaaSTenantDomainStatusActive,
		VerifiedAt: "2026-07-11 20:00:00",
	}}
	req := httptest.NewRequest(http.MethodGet, "/security/login", nil)
	req.Host = "Login.Customer.Example.com:443"
	rec := httptest.NewRecorder()
	NewIdentityLoginPageHandlerWithDomains(branding, domains, 1).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || domains.host != "login.customer.example.com" || branding.tenantID != 23 {
		t.Fatalf("status=%d host=%q tenant=%d body=%s", rec.Code, domains.host, branding.tenantID, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "域名客户云") {
		t.Fatalf("tenant branding missing: %s", rec.Body.String())
	}
}

func TestIdentityLoginPageRejectsRegisteredInactiveDomain(t *testing.T) {
	domains := &fakeIdentityDomainReader{found: true, domain: SaaSTenantDomain{
		TenantID: 23, Hostname: "pending.customer.example.com", Status: SaaSTenantDomainStatusPending,
	}}
	req := httptest.NewRequest(http.MethodGet, "/security/login", nil)
	req.Host = "pending.customer.example.com"
	rec := httptest.NewRecorder()
	NewIdentityLoginPageHandlerWithDomains(nil, domains, 1).ServeHTTP(rec, req)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
