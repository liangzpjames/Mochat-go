package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/migration"
	compatserver "jiyi/mochat-go/internal/server"
	"jiyi/mochat-go/internal/taskrunner"
)

type recoveringRedisPinger struct{ down bool }

func (p *recoveringRedisPinger) Ping(context.Context) error {
	if p.down {
		return errors.New("redis unavailable")
	}
	return nil
}

type fakeMigrationStatusReader struct {
	items []migration.StatusItem
	err   error
	calls int
}

func (f *fakeMigrationStatusReader) StatusReadOnly(context.Context) ([]migration.StatusItem, error) {
	f.calls++
	return f.items, f.err
}

func TestMigrationReadinessUsesStableDatabaseAheadCode(t *testing.T) {
	reader := &fakeMigrationStatusReader{items: []migration.StatusItem{
		{Migration: migration.Migration{Version: "0172_known"}, State: "applied"},
		{Migration: migration.Migration{Version: "9999_future"}, State: "database_ahead"},
	}}
	probes := migrationReadinessProbes(reader)
	checks := compatserver.NewReadinessChecker(probes...).Check(context.Background())
	if len(checks) != 1 || checks[0].Code != "migration_database_ahead" || checks[0].Ready || reader.calls != 1 {
		t.Fatalf("checks = %+v calls=%d", checks, reader.calls)
	}

	reader = &fakeMigrationStatusReader{items: []migration.StatusItem{
		{Migration: migration.Migration{Version: "0172_known"}, State: "checksum_mismatch"},
	}}
	checks = compatserver.NewReadinessChecker(migrationReadinessProbes(reader)...).Check(context.Background())
	if len(checks) != 1 || checks[0].Code != "migration_current" || checks[0].Ready || reader.calls != 1 {
		t.Fatalf("checksum checks = %+v calls=%d", checks, reader.calls)
	}
}

func TestMigrationReadinessAcceptsAuditedSupersededMigration(t *testing.T) {
	reader := &fakeMigrationStatusReader{items: []migration.StatusItem{
		{Migration: migration.Migration{Version: "0150_live_code_workspace"}, State: "superseded"},
		{Migration: migration.Migration{Version: "0153_live_code_workspace"}, State: "applied"},
	}}
	checks := compatserver.NewReadinessChecker(migrationReadinessProbes(reader)...).Check(context.Background())
	if len(checks) != 1 || !checks[0].Ready || reader.calls != 1 {
		t.Fatalf("checks = %+v calls=%d", checks, reader.calls)
	}
}

func TestBackgroundTaskReadinessToleratesSingleFailureAndRecoversAfterThree(t *testing.T) {
	snapshots := []taskrunner.Snapshot{{
		Name: "cron-required", Status: taskrunner.StatusRunning, LastSuccessAt: time.Now().Add(-time.Minute).Format(time.RFC3339),
		ConsecutiveFailures: 1,
	}}
	checker := compatserver.NewReadinessChecker(backgroundTasksReadinessProbe(func() []taskrunner.Snapshot { return snapshots }))
	srv, err := compatserver.New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second}, compatserver.WithReadinessChecker(checker))
	if err != nil {
		t.Fatal(err)
	}
	assertReadyStatus := func(want int) {
		t.Helper()
		recorder := httptest.NewRecorder()
		srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if recorder.Code != want {
			t.Fatalf("ready status = %d want %d body=%s snapshots=%+v", recorder.Code, want, recorder.Body.String(), snapshots)
		}
	}
	assertReadyStatus(http.StatusOK)
	snapshots[0].ConsecutiveFailures = 3
	assertReadyStatus(http.StatusServiceUnavailable)
	snapshots[0].ConsecutiveFailures = 0
	snapshots[0].LatestExecution = &taskrunner.ExecutionSnapshot{Status: taskrunner.StatusSucceeded, StoppedAt: time.Now().Format(time.RFC3339)}
	snapshots[0].LastSuccessAt = snapshots[0].LatestExecution.StoppedAt
	assertReadyStatus(http.StatusOK)
}

func TestCallbackOnlyRedisDependencyOverridesRunningWorkerAndRecovers(t *testing.T) {
	redis := &recoveringRedisPinger{down: true}
	snapshots := func() []taskrunner.Snapshot {
		return []taskrunner.Snapshot{{Name: "contact-welcome", Status: taskrunner.StatusRunning}}
	}
	checker := compatserver.NewReadinessChecker(redisReadinessProbe(redis), backgroundTasksReadinessProbe(snapshots))
	srv, err := compatserver.New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second}, compatserver.WithReadinessChecker(checker))
	if err != nil {
		t.Fatal(err)
	}
	request := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		srv.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		return recorder
	}
	if recorder := request(); recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("callback-only redis down status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	redis.down = false
	if recorder := request(); recorder.Code != http.StatusOK {
		t.Fatalf("callback-only redis recovery status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}
