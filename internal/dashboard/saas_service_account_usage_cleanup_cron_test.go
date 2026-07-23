package dashboard

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

type fakeSaaSServiceAccountUsageCleaner struct {
	limit  int
	result SaaSServiceAccountUsageCleanupResult
	err    error
}

func (f *fakeSaaSServiceAccountUsageCleaner) CleanupSaaSServiceAccountUsage(_ context.Context, limit int) (SaaSServiceAccountUsageCleanupResult, error) {
	f.limit = limit
	return f.result, f.err
}

func TestSaaSServiceAccountUsageCleanupCron(t *testing.T) {
	var output bytes.Buffer
	cleaner := &fakeSaaSServiceAccountUsageCleaner{result: SaaSServiceAccountUsageCleanupResult{
		RetentionDays: 90, CutoffDate: "2026-04-14", EligibleRows: 12, ProtectedRows: 3, DeletedRows: 10, RemainingRows: 2,
	}}
	cron := NewSaaSServiceAccountUsageCleanupCron(cleaner, 10, log.New(&output, "", 0))
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cleaner.limit != 10 || !strings.Contains(output.String(), "protected=3 deleted=10 remaining=2") {
		t.Fatalf("limit=%d log=%q", cleaner.limit, output.String())
	}

	failing := &fakeSaaSServiceAccountUsageCleaner{err: errors.New("cleanup failed")}
	if err := NewSaaSServiceAccountUsageCleanupCron(failing, 0, nil).RunOnce(context.Background()); err == nil {
		t.Fatal("cleanup error was ignored")
	}
	if failing.limit != 10000 {
		t.Fatalf("default limit=%d", failing.limit)
	}
}
