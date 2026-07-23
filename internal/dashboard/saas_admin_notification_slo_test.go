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

func TestSaaSAdminBuildNotificationSLOReportFillsDaysAndClassifiesObjectives(t *testing.T) {
	source := SaaSAdminNotificationSLOSource{
		Days: []SaaSAdminNotificationSLODaySnapshot{
			{
				Day: "2026-07-10", TenantCount: 2,
				SaaSAdminNotificationSLOMetricsSnapshot: SaaSAdminNotificationSLOMetricsSnapshot{
					NotificationCount: 9, PendingCount: 1, DeliveredCount: 5, FailedCount: 1, DeadCount: 1, ClosedCount: 1, SuppressedCount: 1,
					DeliveredWithinTarget: 5, TotalAttempts: 10, AverageDeliverySeconds: 20, MaxDeliverySeconds: 40,
				},
			},
			{
				Day: "2026-07-09", TenantCount: 1,
				SaaSAdminNotificationSLOMetricsSnapshot: SaaSAdminNotificationSLOMetricsSnapshot{
					NotificationCount: 4, DeliveredCount: 4, DeliveredWithinTarget: 3, TotalAttempts: 4, AverageDeliverySeconds: 100, MaxDeliverySeconds: 600,
				},
			},
		},
		Tenants: []SaaSAdminNotificationSLOTenantSnapshot{
			{
				TenantID: 901, TenantName: "违约租户", TenantStatus: 1, PackageCode: "growth", PackageName: "增长版",
				SaaSAdminNotificationSLOMetricsSnapshot: SaaSAdminNotificationSLOMetricsSnapshot{
					NotificationCount: 10, DeliveredCount: 7, FailedCount: 1, DeadCount: 1, ClosedCount: 1, DeliveredWithinTarget: 6,
				},
			},
			{
				TenantID: 902, TenantName: "达标租户", TenantStatus: 1, PackageCode: "starter", PackageName: "基础版",
				SaaSAdminNotificationSLOMetricsSnapshot: SaaSAdminNotificationSLOMetricsSnapshot{
					NotificationCount: 2, DeliveredCount: 2, DeliveredWithinTarget: 2,
				},
			},
			{
				TenantID: 903, TenantName: "无数据租户", TenantStatus: 1,
				SaaSAdminNotificationSLOMetricsSnapshot: SaaSAdminNotificationSLOMetricsSnapshot{
					NotificationCount: 2, PendingCount: 1, SuppressedCount: 1,
				},
			},
		},
	}
	options := SaaSAdminNotificationSLOOptions{
		Channel: "webhook", Days: 3, SuccessRateTarget: 0.9, LatencySecondsTarget: 300, LatencyRateTarget: 0.75, Limit: 10,
	}
	report := saasAdminBuildNotificationSLOReport(source, options, time.Date(2026, 7, 10, 13, 0, 0, 0, time.Local))

	if report.WindowStartDate != "2026-07-08" || report.WindowEndDate != "2026-07-10" || report.GeneratedAt != "2026-07-10 13:00:00" {
		t.Fatalf("window=%s..%s generated=%s", report.WindowStartDate, report.WindowEndDate, report.GeneratedAt)
	}
	if len(report.Days) != 3 || report.Days[0].Day != "2026-07-08" || report.Days[0].SLOState != SaaSAdminNotificationSLOStateNoData || report.Days[1].SLOState != SaaSAdminNotificationSLOStateMet || report.Days[2].SLOState != SaaSAdminNotificationSLOStateBreached {
		t.Fatalf("days = %+v", report.Days)
	}
	if report.Days[2].AttemptedCount != 8 || report.Days[2].ClosedCount != 1 || report.Days[2].DeliverySuccessRate != 0.625 || report.Days[1].LatencyAttainmentRate != 0.75 {
		t.Fatalf("day metrics = %+v", report.Days)
	}
	summary := report.Summary
	if summary.DayCount != 3 || summary.MetDayCount != 1 || summary.BreachedDayCount != 1 || summary.NoDataDayCount != 1 {
		t.Fatalf("day summary = %+v", summary)
	}
	if summary.AttemptedCount != 12 || summary.DeliveredCount != 9 || summary.ClosedCount != 1 || summary.DeliverySuccessRate != 0.75 || summary.DeliveredWithinTarget != 8 || summary.LatencyAttainmentRate != 0.8889 || summary.SLOState != SaaSAdminNotificationSLOStateBreached {
		t.Fatalf("measurement summary = %+v", summary)
	}
	if summary.AverageDeliverySeconds != 55.56 || summary.MaxDeliverySeconds != 600 {
		t.Fatalf("latency summary = %+v", summary)
	}
	if summary.TenantCount != 3 || summary.ReturnedTenantCount != 3 || summary.BreachedTenantCount != 1 || summary.MetTenantCount != 1 || summary.NoDataTenantCount != 1 {
		t.Fatalf("tenant summary = %+v", summary)
	}
	if len(report.Tenants) != 3 || report.Tenants[0].TenantID != 901 || report.Tenants[0].SLOState != SaaSAdminNotificationSLOStateBreached || report.Tenants[1].TenantID != 902 || report.Tenants[1].SLOState != SaaSAdminNotificationSLOStateMet || report.Tenants[2].TenantID != 903 || report.Tenants[2].SLOState != SaaSAdminNotificationSLOStateNoData {
		t.Fatalf("tenants = %+v", report.Tenants)
	}
}

func TestSaaSAdminBuildNotificationSLOReportUsesDatabaseWindowDates(t *testing.T) {
	report := saasAdminBuildNotificationSLOReport(
		SaaSAdminNotificationSLOSource{
			WindowStartDate: "2026-07-07",
			WindowEndDate:   "2026-07-09",
			Days: []SaaSAdminNotificationSLODaySnapshot{{
				Day: "2026-07-09",
				SaaSAdminNotificationSLOMetricsSnapshot: SaaSAdminNotificationSLOMetricsSnapshot{
					NotificationCount: 1, DeliveredCount: 1, DeliveredWithinTarget: 1,
				},
			}},
		},
		SaaSAdminNotificationSLOOptions{Days: 3, SuccessRateTarget: 0.9, LatencySecondsTarget: 300, LatencyRateTarget: 0.9, Limit: 10},
		time.Date(2026, 7, 10, 1, 0, 0, 0, time.Local),
	)

	if report.WindowStartDate != "2026-07-07" || report.WindowEndDate != "2026-07-09" {
		t.Fatalf("window = %s..%s", report.WindowStartDate, report.WindowEndDate)
	}
	if len(report.Days) != 3 || report.Days[0].Day != "2026-07-07" || report.Days[2].Day != "2026-07-09" || report.Days[2].SLOState != SaaSAdminNotificationSLOStateMet {
		t.Fatalf("days = %+v", report.Days)
	}
}

func TestSaaSAdminNotificationSLORequiresPlatformAdminAndReturnsObjectives(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 1, IsSuperAdmin: 1},
			2: {ID: 2, TenantID: 2, IsSuperAdmin: 1},
		},
		notificationSLOSource: SaaSAdminNotificationSLOSource{
			Tenants: []SaaSAdminNotificationSLOTenantSnapshot{{
				TenantID: 901, TenantName: "达标租户",
				SaaSAdminNotificationSLOMetricsSnapshot: SaaSAdminNotificationSLOMetricsSnapshot{NotificationCount: 2, DeliveredCount: 2, DeliveredWithinTarget: 2},
			}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notificationSlo?tenantId=901&channel=webhook&keyword=%E8%BE%BE%E6%A0%87&days=30&successRateTarget=0.9&latencySecondsTarget=120&latencyRateTarget=0.8&limit=9", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.NotificationSLO(rec, req)

	if rec.Code != http.StatusOK || store.notificationSLOCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.notificationSLOCalls, rec.Body.String())
	}
	options := store.lastNotificationSLOOptions
	if options.TenantID != 901 || options.Channel != "webhook" || options.Keyword != "达标" || options.Days != 30 || options.SuccessRateTarget != 0.9 || options.LatencySecondsTarget != 120 || options.LatencyRateTarget != 0.8 || options.Limit != 9 {
		t.Fatalf("options = %+v", options)
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Objectives       map[string]any   `json:"objectives"`
			MeasurementBasis map[string]any   `json:"measurementBasis"`
			Summary          map[string]any   `json:"summary"`
			Tenants          []map[string]any `json:"tenants"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != 200 || payload.Data.Objectives["latencySecondsTarget"].(float64) != 120 || payload.Data.MeasurementBasis["cohort"] != "created_at" || payload.Data.Summary["metTenantCount"].(float64) != 1 || len(payload.Data.Tenants) != 1 || payload.Data.Tenants[0]["sloState"] != SaaSAdminNotificationSLOStateMet {
		t.Fatalf("payload = %+v body=%s", payload, rec.Body.String())
	}

	forbiddenReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notificationSlo", nil)
	forbiddenReq.Header.Set("X-Mochat-Go-User-ID", "2")
	forbiddenRec := httptest.NewRecorder()
	handler.NotificationSLO(forbiddenRec, forbiddenReq)
	if forbiddenRec.Code != http.StatusForbidden || store.notificationSLOCalls != 1 {
		t.Fatalf("forbidden status=%d calls=%d body=%s", forbiddenRec.Code, store.notificationSLOCalls, forbiddenRec.Body.String())
	}
}

func TestSaaSAdminNotificationSLORejectsInvalidObjectives(t *testing.T) {
	store := &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	paths := []string{
		"/dashboard/saasAdmin/notificationSlo?channel=email",
		"/dashboard/saasAdmin/notificationSlo?days=0",
		"/dashboard/saasAdmin/notificationSlo?days=91",
		"/dashboard/saasAdmin/notificationSlo?successRateTarget=0.49",
		"/dashboard/saasAdmin/notificationSlo?successRateTarget=NaN",
		"/dashboard/saasAdmin/notificationSlo?latencySecondsTarget=0",
		"/dashboard/saasAdmin/notificationSlo?latencySecondsTarget=86401",
		"/dashboard/saasAdmin/notificationSlo?latencyRateTarget=1.1",
		"/dashboard/saasAdmin/notificationSlo?limit=0",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()
		handler.NotificationSLO(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
	if store.notificationSLOCalls != 0 {
		t.Fatalf("calls = %d", store.notificationSLOCalls)
	}
}

func TestSaaSAdminExportNotificationSLOCSV(t *testing.T) {
	store := &fakeSaaSAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}},
		notificationSLOSource: SaaSAdminNotificationSLOSource{
			Days: []SaaSAdminNotificationSLODaySnapshot{{
				Day: "2026-07-10", TenantCount: 1,
				SaaSAdminNotificationSLOMetricsSnapshot: SaaSAdminNotificationSLOMetricsSnapshot{NotificationCount: 2, DeliveredCount: 2, DeliveredWithinTarget: 2},
			}},
			Tenants: []SaaSAdminNotificationSLOTenantSnapshot{{
				TenantID: 901, TenantName: "租户A", TenantStatus: 1, PackageCode: "growth", PackageName: "增长版",
				SaaSAdminNotificationSLOMetricsSnapshot: SaaSAdminNotificationSLOMetricsSnapshot{NotificationCount: 2, DeliveredCount: 2, DeliveredWithinTarget: 2},
			}},
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type=notificationSlo&days=7&successRateTarget=0.9&latencySecondsTarget=120&latencyRateTarget=0.8&limit=1000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ExportCSV(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "notificationSlo") {
		t.Fatalf("status=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(rec.Body.String(), "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 10 || rows[0][0] != "section" || rows[1][0] != "summary" || rows[2][0] != "day" || rows[9][0] != "tenant" || rows[9][8] != "901" || rows[9][9] != "租户A" || rows[9][14] != SaaSAdminNotificationSLOStateMet {
		t.Fatalf("rows = %+v", rows)
	}
	if store.lastNotificationSLOOptions.Days != 7 || store.lastNotificationSLOOptions.SuccessRateTarget != 0.9 || store.lastNotificationSLOOptions.LatencySecondsTarget != 120 || store.lastNotificationSLOOptions.LatencyRateTarget != 0.8 || store.lastNotificationSLOOptions.Limit != 1000 {
		t.Fatalf("options = %+v", store.lastNotificationSLOOptions)
	}
}
