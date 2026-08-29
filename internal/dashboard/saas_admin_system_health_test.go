package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/migration"
)

func TestSaaSAdminSystemHealthMigrationExpectationMatchesRelease(t *testing.T) {
	if SaaSAdminExpectedMigrationVersion != "0175_contact_batch_title" || SaaSAdminExpectedMigrationCount != 175 {
		t.Fatalf("SaaS admin migration baseline = %q/%d, want 0175_contact_batch_title/175", SaaSAdminExpectedMigrationVersion, SaaSAdminExpectedMigrationCount)
	}
	migrations := migration.DefaultMigrations(filepath.Join("..", ".."))
	if len(migrations) != SaaSAdminExpectedMigrationCount {
		t.Fatalf("expected migration count=%d discovered=%d", SaaSAdminExpectedMigrationCount, len(migrations))
	}
	if migrations[len(migrations)-1].Version != SaaSAdminExpectedMigrationVersion {
		t.Fatalf("expected migration version=%s discovered=%s", SaaSAdminExpectedMigrationVersion, migrations[len(migrations)-1].Version)
	}
}

func TestSaaSAdminSystemHealthIncludesRuntimeProbe(t *testing.T) {
	store := &fakeSaaSAdminSystemHealthStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
		checks:             []SaaSAdminSystemHealthCheck{{Code: "schema_migration", Name: "数据库迁移", Category: "database", Status: SaaSAdminSystemHealthStateHealthy}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithSystemHealthProbe(SaaSAdminSystemHealthProbe{
		Code: "redis_connection", Name: "Redis 连接", Category: "runtime", Severity: SaaSAdminSystemHealthStateCritical,
		Check: func(context.Context) error { return errors.New("redis unavailable") },
	})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/systemHealth?failureWindowHours=48&notificationStaleMinutes=30", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.SystemHealth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeSaaSAdminResponse(t, rec)
	summary := data["summary"].(map[string]any)
	if summary["healthState"] != SaaSAdminSystemHealthStateCritical || summary["criticalCount"].(float64) != 1 {
		t.Fatalf("summary=%+v", summary)
	}
	if store.lastOptions.FailureWindowHours != 48 || store.lastOptions.NotificationStaleMins != 30 {
		t.Fatalf("options=%+v", store.lastOptions)
	}
	checks := data["checks"].([]any)
	if len(checks) != 2 || checks[1].(map[string]any)["detail"] != "redis unavailable" {
		t.Fatalf("checks=%+v", checks)
	}
}

func TestSaaSAdminSystemHealthScanPersistsTransitions(t *testing.T) {
	store := &fakeSaaSAdminSystemHealthStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
		checks:             []SaaSAdminSystemHealthCheck{{Code: "notification_dead", Name: "通知耗尽", Category: "notifications", Status: SaaSAdminSystemHealthStateCritical, Severity: SaaSAdminSystemHealthStateCritical, Current: 2, Threshold: 1}},
		persistResult:      SaaSAdminSystemHealthScanResult{Scan: SaaSAdminSystemHealthScan{ID: 7, ScanNo: "HSC-TEST", HealthState: SaaSAdminSystemHealthStateCritical, CheckCount: 1, IssueCount: 1}, OpenedCount: 1, NotificationKeys: []string{"health-notification"}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithSystemHealthNotificationMaxAttempts(5)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/systemHealthScan", strings.NewReader(`{"failureWindowHours":12,"notificationStaleMinutes":20}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.SystemHealthScan(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.persistInput.TriggerType != SaaSAdminSystemHealthTriggerManual || !store.persistInput.Notify ||
		store.persistInput.MaxAttempts != 5 || store.persistInput.ActorUserID != 1 || store.persistInput.PlatformTenantID != 1 ||
		store.persistInput.Options.FailureWindowHours != 12 || store.persistInput.Summary.CriticalCount != 1 {
		t.Fatalf("persist input=%+v", store.persistInput)
	}
	data := decodeSaaSAdminResponse(t, rec)
	if data["openedCount"].(float64) != 1 || data["notificationCount"].(float64) != 1 {
		t.Fatalf("data=%+v", data)
	}
}

func TestSaaSAdminSystemIncidentRequiresOwnerForAcknowledge(t *testing.T) {
	store := &fakeSaaSAdminSystemHealthStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/systemIncident", strings.NewReader(`{"incidentId":7,"action":"acknowledge","expectedVersion":1}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.SystemIncident(rec, req)
	if rec.Code != http.StatusBadRequest || store.updateInput.IncidentID != 0 {
		t.Fatalf("status=%d body=%s input=%+v", rec.Code, rec.Body.String(), store.updateInput)
	}
}

func TestSaaSAdminSystemHealthCronRunsAsSystemActor(t *testing.T) {
	store := &fakeSaaSAdminSystemHealthStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{},
		checks:             []SaaSAdminSystemHealthCheck{{Code: "database_connection", Status: SaaSAdminSystemHealthStateHealthy}},
		persistResult:      SaaSAdminSystemHealthScanResult{Scan: SaaSAdminSystemHealthScan{HealthState: SaaSAdminSystemHealthStateHealthy, CheckCount: 1}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 9)
	cron := NewSaaSAdminSystemHealthCron(handler, 48, 30, 4, nil)
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.persistInput.TriggerType != SaaSAdminSystemHealthTriggerCron || store.persistInput.ActorUserID != 0 ||
		store.persistInput.ActorTenantID != 9 || store.persistInput.PlatformTenantID != 9 || store.persistInput.MaxAttempts != 4 ||
		store.persistInput.Options.FailureWindowHours != 48 || store.persistInput.Options.NotificationStaleMins != 30 {
		t.Fatalf("persist input=%+v", store.persistInput)
	}
}

type fakeSaaSAdminSystemHealthStore struct {
	*fakeSaaSAdminStore
	checks        []SaaSAdminSystemHealthCheck
	scans         []SaaSAdminSystemHealthScan
	incidents     []SaaSAdminSystemIncident
	persistInput  SaaSAdminSystemHealthScanInput
	persistResult SaaSAdminSystemHealthScanResult
	updateInput   SaaSAdminSystemIncidentUpdate
	updateResult  SaaSAdminSystemIncidentUpdateResult
	lastOptions   SaaSAdminSystemHealthOptions
}

func (s *fakeSaaSAdminSystemHealthStore) SaaSAdminSystemHealthChecks(_ context.Context, options SaaSAdminSystemHealthOptions) ([]SaaSAdminSystemHealthCheck, error) {
	s.lastOptions = options
	return append([]SaaSAdminSystemHealthCheck(nil), s.checks...), nil
}

func (s *fakeSaaSAdminSystemHealthStore) PersistSaaSAdminSystemHealthScan(_ context.Context, input SaaSAdminSystemHealthScanInput) (SaaSAdminSystemHealthScanResult, error) {
	s.persistInput = input
	return s.persistResult, nil
}

func (s *fakeSaaSAdminSystemHealthStore) SaaSAdminSystemHealthScans(_ context.Context, _ int) ([]SaaSAdminSystemHealthScan, error) {
	return append([]SaaSAdminSystemHealthScan(nil), s.scans...), nil
}

func (s *fakeSaaSAdminSystemHealthStore) SaaSAdminSystemIncidents(_ context.Context, _ SaaSAdminSystemIncidentOptions) ([]SaaSAdminSystemIncident, error) {
	return append([]SaaSAdminSystemIncident(nil), s.incidents...), nil
}

func (s *fakeSaaSAdminSystemHealthStore) UpdateSaaSAdminSystemIncident(_ context.Context, input SaaSAdminSystemIncidentUpdate) (SaaSAdminSystemIncidentUpdateResult, error) {
	s.updateInput = input
	return s.updateResult, nil
}
