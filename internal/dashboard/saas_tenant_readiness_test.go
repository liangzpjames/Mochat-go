package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeSaaSAdminTenantReadinessStore struct {
	*fakeSaaSAdminStore
	page        SaaSAdminTenantReadinessFactPage
	lastOptions SaaSAdminTenantReadinessOptions
	calls       int
}

func (s *fakeSaaSAdminTenantReadinessStore) SaaSAdminTenantReadinessFacts(_ context.Context, options SaaSAdminTenantReadinessOptions) (SaaSAdminTenantReadinessFactPage, error) {
	s.calls++
	s.lastOptions = options
	return s.page, nil
}

func TestBuildSaaSAdminTenantReadinessReportClassifiesAndSortsTenants(t *testing.T) {
	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.Local)
	ready := completeSaaSAdminTenantReadinessFacts(31, "完整租户")
	attention := completeSaaSAdminTenantReadinessFacts(32, "待完善租户")
	attention.SecureNotificationPolicyCount = 0
	blocked := completeSaaSAdminTenantReadinessFacts(33, "阻塞租户")
	blocked.SubscriptionID = 0
	blocked.SubscriptionStatus = ""

	report := BuildSaaSAdminTenantReadinessReport(SaaSAdminTenantReadinessFactPage{
		Facts: []SaaSAdminTenantReadinessFacts{ready, attention, blocked},
	}, SaaSAdminTenantReadinessOptions{PlatformAdminTenantID: 1, State: SaaSAdminTenantReadinessStateAll, Limit: 20}, now)

	if report.Summary.TenantCount != 3 || report.Summary.ReadyCount != 1 || report.Summary.AttentionCount != 1 || report.Summary.BlockedCount != 1 {
		t.Fatalf("summary = %+v", report.Summary)
	}
	if report.Scope != SaaSAdminTenantReadinessScopeBusiness || report.PlatformAdminTenantID != 1 {
		t.Fatalf("scope = %q platformTenantID = %d", report.Scope, report.PlatformAdminTenantID)
	}
	if report.Summary.CoreReadyCount != 2 || report.Summary.FilteredCount != 3 {
		t.Fatalf("summary = %+v", report.Summary)
	}
	if len(report.Tenants) != 3 {
		t.Fatalf("tenants = %d", len(report.Tenants))
	}
	if report.Tenants[0].TenantID != blocked.TenantID || report.Tenants[0].State != SaaSAdminTenantReadinessStateBlocked {
		t.Fatalf("first tenant = %+v", report.Tenants[0])
	}
	if report.Tenants[1].TenantID != attention.TenantID || report.Tenants[1].State != SaaSAdminTenantReadinessStateAttention {
		t.Fatalf("second tenant = %+v", report.Tenants[1])
	}
	if report.Tenants[2].TenantID != ready.TenantID || report.Tenants[2].State != SaaSAdminTenantReadinessStateReady {
		t.Fatalf("third tenant = %+v", report.Tenants[2])
	}
	if readyItem := report.Tenants[2]; readyItem.CompletionPercent != 100 || readyItem.RequiredCheckCount != 8 || readyItem.CheckCount != 11 {
		t.Fatalf("ready item = %+v", readyItem)
	}
	if blockedItem := report.Tenants[0]; blockedItem.FailedCheckCount != 1 || blockedItem.SubscriptionStatus != "" {
		t.Fatalf("blocked item = %+v", blockedItem)
	}
}

func TestBuildSaaSAdminTenantReadinessReportFiltersBeforeLimit(t *testing.T) {
	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.Local)
	ready := completeSaaSAdminTenantReadinessFacts(41, "完整租户")
	attention := completeSaaSAdminTenantReadinessFacts(42, "待完善租户")
	attention.BrandingStatus = ""
	blocked := completeSaaSAdminTenantReadinessFacts(43, "阻塞租户")
	blocked.PackageExpiresAt = "2026-07-17 12:00:00"

	report := BuildSaaSAdminTenantReadinessReport(SaaSAdminTenantReadinessFactPage{
		Facts:     []SaaSAdminTenantReadinessFacts{ready, blocked, attention},
		Truncated: true,
	}, SaaSAdminTenantReadinessOptions{State: SaaSAdminTenantReadinessStateAttention, Limit: 1}, now)

	if !report.Truncated || report.Summary.TenantCount != 3 || report.Summary.FilteredCount != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Tenants) != 1 || report.Tenants[0].TenantID != attention.TenantID {
		t.Fatalf("tenants = %+v", report.Tenants)
	}
}

func TestSaaSAdminTenantReadinessHandler(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{
		1: {ID: 1, TenantID: 1, IsSuperAdmin: 1, Status: 1},
	}}
	store := &fakeSaaSAdminTenantReadinessStore{
		fakeSaaSAdminStore: base,
		page: SaaSAdminTenantReadinessFactPage{Facts: []SaaSAdminTenantReadinessFacts{
			completeSaaSAdminTenantReadinessFacts(51, "可上线租户"),
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantReadiness?tenantId=51&state=ready&keyword=%E5%8F%AF%E4%B8%8A%E7%BA%BF&limit=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantReadiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.calls != 1 || store.lastOptions.TenantID != 51 || store.lastOptions.PlatformAdminTenantID != 1 || store.lastOptions.State != SaaSAdminTenantReadinessStateReady || store.lastOptions.Limit != 10 || store.lastOptions.Keyword != "可上线" {
		t.Fatalf("calls=%d options=%+v", store.calls, store.lastOptions)
	}
	var body struct {
		Code int                            `json:"code"`
		Data SaaSAdminTenantReadinessReport `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != http.StatusOK || body.Data.Scope != SaaSAdminTenantReadinessScopeBusiness || body.Data.PlatformAdminTenantID != 1 || len(body.Data.Tenants) != 1 || body.Data.Tenants[0].State != SaaSAdminTenantReadinessStateReady {
		t.Fatalf("body = %+v", body)
	}
}

func TestSaaSAdminTenantReadinessHandlerIncludesTenantOne(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{
		1: {ID: 1, TenantID: 1, IsSuperAdmin: 1, Status: 1},
	}}
	store := &fakeSaaSAdminTenantReadinessStore{
		fakeSaaSAdminStore: base,
		page: SaaSAdminTenantReadinessFactPage{Facts: []SaaSAdminTenantReadinessFacts{
			completeSaaSAdminTenantReadinessFacts(1, "首个业务租户"),
		}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantReadiness?tenantId=1", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantReadiness(rec, req)

	if rec.Code != http.StatusOK || store.calls != 1 || store.lastOptions.TenantID != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.calls, rec.Body.String())
	}
	var body struct {
		Code int                            `json:"code"`
		Data SaaSAdminTenantReadinessReport `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != http.StatusOK || len(body.Data.Tenants) != 1 || body.Data.Tenants[0].TenantID != 1 {
		t.Fatalf("body = %+v", body)
	}
}

func TestSaaSAdminTenantReadinessHandlerRejectsInvalidState(t *testing.T) {
	base := &fakeSaaSAdminStore{users: map[int]User{
		1: {ID: 1, TenantID: 1, IsSuperAdmin: 1, Status: 1},
	}}
	store := &fakeSaaSAdminTenantReadinessStore{fakeSaaSAdminStore: base}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantReadiness?state=unknown", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.TenantReadiness(rec, req)

	if rec.Code != http.StatusBadRequest || store.calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.calls, rec.Body.String())
	}
}

func completeSaaSAdminTenantReadinessFacts(tenantID int, tenantName string) SaaSAdminTenantReadinessFacts {
	return SaaSAdminTenantReadinessFacts{
		TenantID:                       tenantID,
		TenantName:                     tenantName,
		TenantStatus:                   1,
		ActiveSuperAdminCount:          1,
		PackageCode:                    "growth",
		PackageName:                    "成长版",
		PackageStatus:                  1,
		SubscriptionID:                 int64(tenantID),
		SubscriptionStatus:             SaaSAdminSubscriptionStatusActive,
		CoreSeedCount:                  saasAdminTenantReadinessRequiredSeedCount,
		BoundCorpCount:                 1,
		ConfiguredCorpCredentialCount:  1,
		ProtectedCorpCredentialCount:   1,
		IdentityPolicyStatus:           "active",
		EnabledNotificationPolicyCount: 1,
		SecureNotificationPolicyCount:  1,
		BrandingStatus:                 "active",
		PrimaryVerifiedDomainCount:     1,
		ReadyPrimaryDomainCount:        1,
	}
}
