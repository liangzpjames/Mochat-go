package dashboard

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSaaSAdminBuildNotificationHealthReportClassifiesAndAggregates(t *testing.T) {
	source := SaaSAdminNotificationHealthSource{
		Tenants: []SaaSAdminNotificationHealthSnapshot{
			{
				TenantID: 1, TenantName: "严重租户", PolicyConfigured: true, PolicyEnabled: true,
				NotificationCount: 10, PendingCount: 1, ReadyPendingCount: 1, StalePendingCount: 1,
				DeliveredCount: 4, FailedCount: 1, DeadCount: 1, TotalAttempts: 8,
				AverageDeliverySeconds: 4, MaxDeliverySeconds: 10,
			},
			{
				TenantID: 2, TenantName: "预警租户", PolicyConfigured: true, PolicyEnabled: true,
				NotificationCount: 6, DeliveredCount: 5, FailedCount: 1, TotalAttempts: 7,
				AverageDeliverySeconds: 2, MaxDeliverySeconds: 4,
			},
			{
				TenantID: 3, TenantName: "健康租户", NotificationCount: 7, PendingCount: 1, DeferredCount: 1,
				DeliveredCount: 5, SuppressedCount: 1, TotalAttempts: 5,
				AverageDeliverySeconds: 1, MaxDeliverySeconds: 2,
			},
			{TenantID: 4, TenantName: "无数据租户"},
		},
		FailureReasons: []SaaSAdminNotificationHealthFailureSnapshot{
			{TenantID: 1, Reason: "webhook status 500", Count: 2, LastOccurredAt: "2026-07-10 12:00:00"},
			{TenantID: 2, Reason: "webhook status 500", Count: 1, LastOccurredAt: "2026-07-10 11:00:00"},
			{TenantID: 1, Reason: "connection timeout", Count: 1, LastOccurredAt: "2026-07-10 10:00:00"},
		},
	}
	options := SaaSAdminNotificationHealthOptions{Channel: "webhook", State: "all", WindowHours: 24, StaleMinutes: 15, Limit: 10}
	report := saasAdminBuildNotificationHealthReport(source, options, time.Date(2026, 7, 10, 13, 0, 0, 0, time.Local))

	if report.WindowStartAt != "2026-07-09 13:00:00" || report.WindowEndAt != "2026-07-10 13:00:00" {
		t.Fatalf("window = %s..%s", report.WindowStartAt, report.WindowEndAt)
	}
	if len(report.Tenants) != 4 {
		t.Fatalf("tenants = %+v", report.Tenants)
	}
	wantStates := []string{"critical", "warning", "healthy", "no_data"}
	for i, want := range wantStates {
		if report.Tenants[i].HealthState != want {
			t.Fatalf("tenant %d state = %s want %s", i, report.Tenants[i].HealthState, want)
		}
	}
	if report.Tenants[0].DeliverySuccessRate != 0.6667 || report.Tenants[1].DeliverySuccessRate != 0.8333 || report.Tenants[2].DeliverySuccessRate != 1 {
		t.Fatalf("rates = %.4f %.4f %.4f", report.Tenants[0].DeliverySuccessRate, report.Tenants[1].DeliverySuccessRate, report.Tenants[2].DeliverySuccessRate)
	}
	summary := report.Summary
	if summary.TenantCount != 4 || summary.MatchedTenantCount != 4 || summary.ReturnedTenantCount != 4 || summary.CriticalTenantCount != 1 || summary.WarningTenantCount != 1 || summary.HealthyTenantCount != 1 || summary.NoDataTenantCount != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.NotificationCount != 23 || summary.AttemptedCount != 17 || summary.DeliveredCount != 14 || summary.DeliverySuccessRate != 0.8235 {
		t.Fatalf("delivery summary = %+v", summary)
	}
	if summary.AverageDeliverySeconds != 2.21 || summary.MaxDeliverySeconds != 10 {
		t.Fatalf("latency summary = %+v", summary)
	}
	if len(report.FailureReasons) != 2 || report.FailureReasons[0].Reason != "webhook status 500" || report.FailureReasons[0].Count != 3 || report.FailureReasons[0].TenantCount != 2 {
		t.Fatalf("failure reasons = %+v", report.FailureReasons)
	}
}

func TestSaaSAdminBuildNotificationHealthReportFiltersStateBeforeReasons(t *testing.T) {
	source := SaaSAdminNotificationHealthSource{
		Tenants: []SaaSAdminNotificationHealthSnapshot{
			{TenantID: 1, NotificationCount: 1, DeadCount: 1},
			{TenantID: 2, NotificationCount: 1, FailedCount: 1},
		},
		FailureReasons: []SaaSAdminNotificationHealthFailureSnapshot{
			{TenantID: 1, Reason: "dead reason", Count: 2},
			{TenantID: 2, Reason: "warning reason", Count: 3},
		},
	}
	report := saasAdminBuildNotificationHealthReport(source, SaaSAdminNotificationHealthOptions{State: "critical", WindowHours: 24, StaleMinutes: 15, Limit: 10}, time.Now())
	if len(report.Tenants) != 1 || report.Tenants[0].TenantID != 1 || report.Summary.MatchedTenantCount != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.FailureReasons) != 1 || report.FailureReasons[0].Reason != "dead reason" {
		t.Fatalf("failure reasons = %+v", report.FailureReasons)
	}
}

func TestSaaSAdminNotificationHealthRequiresPlatformAdminAndReturnsFilters(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			2: {ID: 2, TenantID: 2, IsSuperAdmin: 1},
		},
		notificationHealthSource: SaaSAdminNotificationHealthSource{Tenants: []SaaSAdminNotificationHealthSnapshot{{
			TenantID: 10, TenantName: "健康租户", NotificationCount: 5, DeliveredCount: 5,
		}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notificationHealth?tenantId=10&state=healthy&channel=webhook&keyword=%E5%81%A5%E5%BA%B7&windowHours=48&staleMinutes=30&limit=9", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.NotificationHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.notificationHealthCalls != 1 {
		t.Fatalf("health calls = %d", store.notificationHealthCalls)
	}
	options := store.lastNotificationHealthOptions
	if options.TenantID != 10 || options.State != "healthy" || options.Channel != "webhook" || options.Keyword != "健康" || options.WindowHours != 48 || options.StaleMinutes != 30 || options.Limit != 9 {
		t.Fatalf("options = %+v", options)
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Filters map[string]any   `json:"filters"`
			Summary map[string]any   `json:"summary"`
			Tenants []map[string]any `json:"tenants"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != 200 || payload.Data.Filters["windowHours"].(float64) != 48 || payload.Data.Summary["healthyTenantCount"].(float64) != 1 || len(payload.Data.Tenants) != 1 {
		t.Fatalf("payload = %+v body=%s", payload, rec.Body.String())
	}

	for _, userID := range []string{"2"} {
		forbiddenReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notificationHealth", nil)
		forbiddenReq.Header.Set("X-Mochat-Go-User-ID", userID)
		forbiddenRec := httptest.NewRecorder()
		handler.NotificationHealth(forbiddenRec, forbiddenReq)
		if forbiddenRec.Code != http.StatusForbidden {
			t.Fatalf("tenant status = %d body=%s", forbiddenRec.Code, forbiddenRec.Body.String())
		}
	}
}

func TestSaaSAdminNotificationHealthRejectsInvalidFilters(t *testing.T) {
	store := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	for _, path := range []string{
		"/dashboard/saasAdmin/notificationHealth?state=bad",
		"/dashboard/saasAdmin/notificationHealth?channel=email",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()
		handler.NotificationHealth(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
	if store.notificationHealthCalls != 0 {
		t.Fatalf("health calls = %d", store.notificationHealthCalls)
	}
}

func TestSaaSAdminExportNotificationHealthCSV(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}},
		notificationHealthSource: SaaSAdminNotificationHealthSource{Tenants: []SaaSAdminNotificationHealthSnapshot{{
			TenantID: 10, TenantName: "租户A", TenantStatus: 1, PackageCode: "growth", PackageName: "成长版",
			PolicyConfigured: true, PolicyEnabled: true, NotificationCount: 6, DeliveredCount: 5, FailedCount: 1,
			AverageDeliverySeconds: 2.5, MaxDeliverySeconds: 6, LastDeliveredAt: "2026-07-10 12:00:00",
		}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=notificationHealth&state=warning&windowHours=48&staleMinutes=20&limit=1000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "notificationHealth") {
		t.Fatalf("status=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(rec.Body.String(), "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0][0] != "windowStartAt" || rows[0][9] != "healthState" || rows[1][4] != "10" || rows[1][5] != "租户A" || rows[1][9] != "warning" || rows[1][12] != "0.8333" {
		t.Fatalf("rows = %+v", rows)
	}
	if store.lastNotificationHealthOptions.WindowHours != 48 || store.lastNotificationHealthOptions.StaleMinutes != 20 || store.lastNotificationHealthOptions.Limit != 1000 {
		t.Fatalf("options = %+v", store.lastNotificationHealthOptions)
	}
}
