package taskrunner

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSQLRecorderCleansExpiredHistoryWithoutBlockingRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("INSERT INTO mochat_go_background_task_executions").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM mochat_go_background_task_executions WHERE created_at < ? LIMIT ?")).
		WithArgs(sqlmock.AnyArg(), 10000).WillDelayFor(250 * time.Millisecond).WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM mochat_go_background_task_runs WHERE created_at < ? LIMIT ?")).
		WithArgs(sqlmock.AnyArg(), 10000).WillReturnResult(sqlmock.NewResult(0, 2))
	recorder := NewSQLRecorder(db, slog.New(slog.NewTextHandler(io.Discard, nil)), WithHistoryRetention(14*24*time.Hour, time.Hour, 10000))
	snapshot := ExecutionSnapshot{ExecutionID: "cron-a-periodic-latest-success", TaskName: "cron-a", Kind: ExecutionKindPeriodicTick, Status: StatusSucceeded, StartedAt: "2026-08-28T10:00:00Z", StoppedAt: "2026-08-28T10:00:01Z"}
	startedAt := time.Now()
	if err := recorder.RecordTaskExecution(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(startedAt); elapsed >= 100*time.Millisecond {
		t.Fatalf("record blocked on history cleanup for %s", elapsed)
	}
	waitForSQLExpectations(t, mock)
	waitForCleanup(t, recorder)
}

func TestSQLRecorderContinuesAfterHistoryCleanupFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var logs bytes.Buffer
	mock.ExpectExec("INSERT INTO mochat_go_background_task_executions").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM mochat_go_background_task_executions WHERE created_at < ? LIMIT ?")).
		WithArgs(sqlmock.AnyArg(), 10000).WillReturnError(context.DeadlineExceeded)
	recorder := NewSQLRecorder(db, slog.New(slog.NewTextHandler(&logs, nil)), WithHistoryRetention(14*24*time.Hour, time.Hour, 10000))
	snapshot := ExecutionSnapshot{ExecutionID: "cron-a-periodic-latest-success", TaskName: "cron-a", Kind: ExecutionKindPeriodicTick, Status: StatusSucceeded, StartedAt: "2026-08-28T10:00:00Z"}
	if err := recorder.RecordTaskExecution(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	waitForSQLExpectations(t, mock)
	waitForCleanup(t, recorder)
	if !strings.Contains(logs.String(), "task_history_cleanup_failed") {
		t.Fatalf("cleanup log = %s", logs.String())
	}
	if strings.Contains(logs.String(), "deadline exceeded") {
		t.Fatalf("cleanup log = %s", logs.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func waitForCleanup(t *testing.T, recorder *SQLRecorder) {
	t.Helper()
	waitForCondition(t, func() bool {
		recorder.cleanupMu.Lock()
		defer recorder.cleanupMu.Unlock()
		return !recorder.cleanupRunning
	}, "history cleanup completion")
}

func waitForSQLExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := mock.ExpectationsWereMet(); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
