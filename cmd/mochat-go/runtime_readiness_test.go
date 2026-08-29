package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/migration"
	compatserver "jiyi/mochat-go/internal/server"
	"jiyi/mochat-go/internal/taskrunner"
)

type fakeMigrationStatusReader struct {
	items []migration.StatusItem
	err   error
}

func (f fakeMigrationStatusReader) StatusReadOnly(context.Context) ([]migration.StatusItem, error) {
	return f.items, f.err
}

func TestMigrationReadinessUsesStableDatabaseAheadCode(t *testing.T) {
	probes := migrationReadinessProbes(fakeMigrationStatusReader{items: []migration.StatusItem{
		{Migration: migration.Migration{Version: "0172_known"}, State: "applied"},
		{Migration: migration.Migration{Version: "9999_future"}, State: "database_ahead"},
	}})
	checks := compatserver.NewReadinessChecker(probes...).Check(context.Background())
	want := map[string]bool{"migration_current": false, "migration_database_ahead": false}
	for _, check := range checks {
		if ready, ok := want[check.Code]; !ok || ready != check.Ready {
			t.Fatalf("checks = %+v", checks)
		}
		delete(want, check.Code)
	}
	if len(want) != 0 {
		t.Fatalf("missing checks = %+v; got %+v", want, checks)
	}

	checks = compatserver.NewReadinessChecker(migrationReadinessProbes(fakeMigrationStatusReader{items: []migration.StatusItem{
		{Migration: migration.Migration{Version: "0172_known"}, State: "checksum_mismatch"},
	}})...).Check(context.Background())
	for _, check := range checks {
		if check.Code == "migration_current" && check.Ready {
			t.Fatalf("checksum mismatch remained ready: %+v", checks)
		}
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
