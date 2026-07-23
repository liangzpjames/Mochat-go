package dashboard

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSaaSAdminEffectiveSubscriptionStatus(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.Local)
	tests := []struct {
		name string
		item SaaSAdminSubscription
		want string
	}{
		{name: "active", item: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusActive, CurrentPeriodEndsAt: "2026-07-20 00:00:00"}, want: SaaSAdminSubscriptionStatusActive},
		{name: "active to grace", item: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusActive, CurrentPeriodEndsAt: "2026-07-09 00:00:00", GraceEndsAt: "2026-07-16 00:00:00"}, want: SaaSAdminSubscriptionStatusGrace},
		{name: "grace to past due", item: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusGrace, GraceEndsAt: "2026-07-10 11:59:59"}, want: SaaSAdminSubscriptionStatusPastDue},
		{name: "trialing", item: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusTrialing, TrialEndsAt: "2026-07-11 00:00:00"}, want: SaaSAdminSubscriptionStatusTrialing},
		{name: "trial to grace", item: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusTrialing, TrialEndsAt: "2026-07-09 00:00:00", GraceEndsAt: "2026-07-12 00:00:00"}, want: SaaSAdminSubscriptionStatusGrace},
		{name: "cancel at period end", item: SaaSAdminSubscription{Status: SaaSAdminSubscriptionStatusActive, CurrentPeriodEndsAt: "2026-07-10 11:00:00", GraceEndsAt: "2026-07-17 00:00:00", CancelAtPeriodEnd: true}, want: SaaSAdminSubscriptionStatusCanceled},
		{name: "tenant disabled", item: SaaSAdminSubscription{TenantStatus: 2, Status: SaaSAdminSubscriptionStatusActive}, want: SaaSAdminSubscriptionStatusSuspended},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SaaSAdminEffectiveSubscriptionStatus(test.item, now); got != test.want {
				t.Fatalf("status = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSaaSAdminSubscriptionAccessAndTransitions(t *testing.T) {
	for _, status := range []string{SaaSAdminSubscriptionStatusTrialing, SaaSAdminSubscriptionStatusActive, SaaSAdminSubscriptionStatusGrace} {
		if !SaaSAdminSubscriptionAllowsAccess(status) {
			t.Fatalf("status %s should allow access", status)
		}
	}
	for _, status := range []string{SaaSAdminSubscriptionStatusPastDue, SaaSAdminSubscriptionStatusSuspended, SaaSAdminSubscriptionStatusCanceled} {
		if SaaSAdminSubscriptionAllowsAccess(status) {
			t.Fatalf("status %s should block access", status)
		}
	}
	if !SaaSAdminSubscriptionTransitionAllowed(SaaSAdminSubscriptionStatusPastDue, SaaSAdminSubscriptionStatusActive) {
		t.Fatal("renewal reactivation should be allowed")
	}
	if SaaSAdminSubscriptionTransitionAllowed(SaaSAdminSubscriptionStatusCanceled, SaaSAdminSubscriptionStatusGrace) {
		t.Fatal("canceled subscription must reactivate before grace")
	}
}

func TestSaaSAdminSubscriptionsRequiresPlatformAndReturnsReport(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			2: {ID: 2, TenantID: 2, IsSuperAdmin: 1},
		},
		subscriptionReport: SaaSAdminSubscriptionReport{
			Summary:       SaaSAdminSubscriptionSummary{SubscriptionCount: 1, GraceCount: 1, AccessAllowedCount: 1},
			Subscriptions: []SaaSAdminSubscription{{TenantID: 8, TenantName: "宽限租户", Status: SaaSAdminSubscriptionStatusGrace, EffectiveStatus: SaaSAdminSubscriptionStatusGrace, AccessAllowed: true, Version: 3}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/subscriptions?status=grace&access=allowed&keyword=%E5%AE%BD%E9%99%90&limit=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Subscriptions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["count"].(float64) != 1 || store.lastSubscriptionOptions.Status != SaaSAdminSubscriptionStatusGrace || store.lastSubscriptionOptions.Access != SaaSAdminSubscriptionAccessAllowed {
		t.Fatalf("data=%+v options=%+v", data, store.lastSubscriptionOptions)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/subscriptions", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec = httptest.NewRecorder()
	handler.Subscriptions(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-platform status = %d", rec.Code)
	}
}

func TestSaaSAdminSubscriptionTransitionCarriesConcurrencyAndAuditFields(t *testing.T) {
	store := &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 1, IsSuperAdmin: 1}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/subscriptionTransition", strings.NewReader(`{
		"tenantId":8,"status":"grace","graceEndsAt":"2026-07-20 00:00:00",
		"expectedVersion":3,"idempotencyKey":"manual:8:3","reason":"等待线下回款"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.TransitionSubscription(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	got := store.lastSubscriptionTransition
	if got.TenantID != 8 || got.Status != SaaSAdminSubscriptionStatusGrace || got.ExpectedVersion != 3 || got.IdempotencyKey != "manual:8:3" || got.ActorUserID != 7 || got.ActorTenantID != 1 || got.Source != "admin" {
		t.Fatalf("transition = %+v", got)
	}
}

func TestSaaSAdminSubscriptionTransitionProtectsPlatformTenant(t *testing.T) {
	store := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/subscriptionTransition", strings.NewReader(`{"tenantId":1,"status":"suspended","reason":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TransitionSubscription(rec, req)
	if rec.Code != http.StatusBadRequest || store.subscriptionTransitionCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.subscriptionTransitionCalls, rec.Body.String())
	}
}

func TestSaaSAdminSubscriptionReconcileAndCSV(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users:                       map[int]User{11: {ID: 11, TenantID: 1, IsSuperAdmin: 1}},
		subscriptionReconcileResult: SaaSAdminSubscriptionReconcileResult{ScannedCount: 2, ReconciliationDue: 1, ChangedCount: 1},
		subscriptionReport:          SaaSAdminSubscriptionReport{Subscriptions: []SaaSAdminSubscription{{TenantID: 8, TenantName: "A", Status: "active", EffectiveStatus: "grace", Version: 2}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/subscriptionReconcile", strings.NewReader(`{"tenantId":8,"limit":20,"dryRun":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "11")
	rec := httptest.NewRecorder()
	handler.ReconcileSubscriptions(rec, req)
	if rec.Code != http.StatusOK || store.lastSubscriptionReconcile.ExcludedTenantID != 1 || !store.lastSubscriptionReconcile.DryRun || store.lastSubscriptionReconcile.ActorUserID != 11 {
		t.Fatalf("status=%d reconcile=%+v body=%s", rec.Code, store.lastSubscriptionReconcile, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=subscriptions", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "11")
	rec = httptest.NewRecorder()
	handler.ExportCSV(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("csv status=%d body=%s", rec.Code, rec.Body.String())
	}
	records := readSaaSAdminCSV(t, rec)
	assertSaaSAdminCSVRowPrefix(t, records, []string{"8", "A", "", "", "active", "grace"})
}

func TestSaaSAdminSubscriptionReconcileCron(t *testing.T) {
	var output bytes.Buffer
	store := &fakeSaaSAdminStore{subscriptionReconcileResult: SaaSAdminSubscriptionReconcileResult{ScannedCount: 3, ReconciliationDue: 2, ChangedCount: 2}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 9)
	cron := NewSaaSAdminSubscriptionReconcileCron(handler, 250, log.New(&output, "", 0))
	if err := cron.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if store.lastSubscriptionReconcile.Limit != 250 || store.lastSubscriptionReconcile.ExcludedTenantID != 9 || store.lastSubscriptionReconcile.ActorTenantID != 9 {
		t.Fatalf("reconcile = %+v", store.lastSubscriptionReconcile)
	}
	if !strings.Contains(output.String(), "changed=2") {
		t.Fatalf("log = %q", output.String())
	}
}
