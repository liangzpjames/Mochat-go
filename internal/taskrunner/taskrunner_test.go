package taskrunner

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroupStartsAndStopsTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	group := New(log.Default())
	group.Add("worker-a", func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})

	if err := group.Start(ctx); err != nil {
		t.Fatal(err)
	}
	<-started
	waitForStatus(t, group, "worker-a", StatusRunning)
	cancel()
	waitForStatus(t, group, "worker-a", StatusStopped)
	snapshot := snapshotByName(t, group, "worker-a")
	if snapshot.StartedAt == "" || snapshot.StoppedAt == "" || snapshot.Error != "" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestGroupMarksFailures(t *testing.T) {
	group := New(log.Default())
	group.Add("broken-worker", func(context.Context) error {
		return errors.New("boom")
	})

	if err := group.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, group, "broken-worker", StatusFailed)
	snapshot := snapshotByName(t, group, "broken-worker")
	if snapshot.Error != "boom" {
		t.Fatalf("error = %q", snapshot.Error)
	}
}

func TestGroupRecoversPanics(t *testing.T) {
	group := New(log.Default())
	group.Add("panic-worker", func(context.Context) error {
		panic("bad task")
	})

	if err := group.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, group, "panic-worker", StatusFailed)
	snapshot := snapshotByName(t, group, "panic-worker")
	if !strings.Contains(snapshot.Error, "panic: bad task") {
		t.Fatalf("error = %q", snapshot.Error)
	}
}

func TestGroupRecordsSnapshots(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder := &memoryRecorder{snapshots: make(chan Snapshot, 4)}
	started := make(chan struct{})
	group := New(log.Default()).WithRecorder(recorder)
	group.Add("worker-a", func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})

	if err := group.Start(ctx); err != nil {
		t.Fatal(err)
	}
	<-started
	running := waitForRecordedStatus(t, recorder, "worker-a", StatusRunning)
	if running.RunID == "" || running.StartedAt == "" || running.StoppedAt != "" || running.Error != "" {
		t.Fatalf("running snapshot = %+v", running)
	}
	cancel()
	stopped := waitForRecordedStatus(t, recorder, "worker-a", StatusStopped)
	if stopped.RunID != running.RunID || stopped.StartedAt == "" || stopped.StoppedAt == "" || stopped.Error != "" {
		t.Fatalf("stopped snapshot = %+v", stopped)
	}
}

func TestGroupContinuesWhenRecorderFails(t *testing.T) {
	group := New(log.Default()).WithRecorder(failingRecorder{})
	group.Add("worker-a", func(context.Context) error { return nil })

	if err := group.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, group, "worker-a", StatusStopped)
}

func TestGroupValidatesTasks(t *testing.T) {
	group := New(log.Default())
	group.Add("", func(context.Context) error { return nil })
	if err := group.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "task name is required") {
		t.Fatalf("error = %v", err)
	}

	group = New(log.Default())
	group.Add("worker-a", nil)
	if err := group.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "run function is required") {
		t.Fatalf("error = %v", err)
	}

	group = New(log.Default())
	group.Add("worker-a", func(context.Context) error { return nil })
	group.Add("worker-a", func(context.Context) error { return nil })
	if err := group.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "duplicate task name") {
		t.Fatalf("error = %v", err)
	}
}

func TestPeriodicRunsOnStartAndInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	run := Periodic(PeriodicConfig{Name: "cron-a", Interval: 10 * time.Millisecond, RunOnStart: true}, func(context.Context) error {
		calls.Add(1)
		return nil
	})

	done := make(chan error, 1)
	go func() {
		done <- run(ctx)
	}()
	waitForCondition(t, func() bool { return calls.Load() >= 2 }, "periodic calls")
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v", err)
	}
}

func TestPeriodicContinuesAfterRunError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	run := Periodic(PeriodicConfig{Name: "cron-a", Interval: 10 * time.Millisecond, RunOnStart: true}, func(context.Context) error {
		calls.Add(1)
		return errors.New("transient")
	})

	done := make(chan error, 1)
	go func() {
		done <- run(ctx)
	}()
	waitForCondition(t, func() bool { return calls.Load() >= 2 }, "periodic retries")
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v", err)
	}
}

func TestPeriodicRecordsExecutions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder := &memoryRecorder{
		snapshots:  make(chan Snapshot, 4),
		executions: make(chan ExecutionSnapshot, 4),
	}
	var calls atomic.Int32
	run := Periodic(PeriodicConfig{Name: "cron-a", Interval: time.Hour, RunOnStart: true}, func(context.Context) error {
		calls.Add(1)
		cancel()
		return nil
	})

	done := make(chan error, 1)
	go func() {
		done <- run(withTaskRuntime(ctx, "cron-a", "run-1", recorder))
	}()
	running := waitForRecordedExecutionStatus(t, recorder, "cron-a", ExecutionKindPeriodicTick, StatusRunning)
	if running.ExecutionID == "" || running.RunID != "run-1" || running.StartedAt == "" {
		t.Fatalf("running execution = %+v", running)
	}
	succeeded := waitForRecordedExecutionStatus(t, recorder, "cron-a", ExecutionKindPeriodicTick, StatusSucceeded)
	if succeeded.ExecutionID != running.ExecutionID || succeeded.RunID != "run-1" || succeeded.StoppedAt == "" || succeeded.Error != "" {
		t.Fatalf("succeeded execution = %+v", succeeded)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d", calls.Load())
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v", err)
	}
}

func TestPeriodicRecordsFailedExecutions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder := &memoryRecorder{
		snapshots:  make(chan Snapshot, 4),
		executions: make(chan ExecutionSnapshot, 4),
	}
	run := Periodic(PeriodicConfig{Name: "cron-a", Interval: time.Hour, RunOnStart: true}, func(context.Context) error {
		cancel()
		return errors.New("tick failed")
	})

	done := make(chan error, 1)
	go func() {
		done <- run(withTaskRuntime(ctx, "cron-a", "run-1", recorder))
	}()
	_ = waitForRecordedExecutionStatus(t, recorder, "cron-a", ExecutionKindPeriodicTick, StatusRunning)
	failed := waitForRecordedExecutionStatus(t, recorder, "cron-a", ExecutionKindPeriodicTick, StatusFailed)
	if failed.ExecutionID == "" || failed.Error != "tick failed" || failed.StoppedAt == "" {
		t.Fatalf("failed execution = %+v", failed)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v", err)
	}
}

func TestPeriodicValidatesConfig(t *testing.T) {
	err := Periodic(PeriodicConfig{Name: "cron-a"}, func(context.Context) error { return nil })(context.Background())
	if err == nil || !strings.Contains(err.Error(), "interval must be positive") {
		t.Fatalf("error = %v", err)
	}

	err = Periodic(PeriodicConfig{Name: "cron-a", Interval: time.Second}, nil)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "run function is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestNullableSnapshotTime(t *testing.T) {
	if value, err := nullableSnapshotTime(""); err != nil || value != nil {
		t.Fatalf("empty value = %v err=%v", value, err)
	}
	if value, err := nullableSnapshotTime("2026-07-04T12:34:56+08:00"); err != nil || value == nil {
		t.Fatalf("valid value = %v err=%v", value, err)
	}
	if _, err := nullableSnapshotTime("bad time"); err == nil {
		t.Fatal("expected invalid time error")
	}
}

func TestNewRunIDIncludesTaskNameAndIsUnique(t *testing.T) {
	startedAt := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	left := newRunID("Cron Corp/Data", startedAt)
	right := newRunID("Cron Corp/Data", startedAt)
	if left == "" || right == "" || left == right {
		t.Fatalf("run ids = %q %q", left, right)
	}
	if !strings.HasPrefix(left, "cron-corp-data-") {
		t.Fatalf("run id prefix = %q", left)
	}
}

func waitForStatus(t *testing.T, group *Group, name string, status string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if snapshotByName(t, group, name).Status == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s to become %s; snapshots=%+v", name, status, group.Snapshots())
}

type memoryRecorder struct {
	snapshots  chan Snapshot
	executions chan ExecutionSnapshot
}

func (r *memoryRecorder) RecordTaskSnapshot(ctx context.Context, snapshot Snapshot) error {
	select {
	case r.snapshots <- snapshot:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *memoryRecorder) RecordTaskExecution(ctx context.Context, snapshot ExecutionSnapshot) error {
	if r.executions == nil {
		return nil
	}
	select {
	case r.executions <- snapshot:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type failingRecorder struct{}

func (failingRecorder) RecordTaskSnapshot(context.Context, Snapshot) error {
	return errors.New("recorder down")
}

func waitForRecordedStatus(t *testing.T, recorder *memoryRecorder, name string, status string) Snapshot {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case snapshot := <-recorder.snapshots:
			if snapshot.Name == name && snapshot.Status == status {
				return snapshot
			}
		case <-deadline:
			t.Fatalf("timed out waiting for recorded %s to become %s", name, status)
		}
	}
}

func waitForRecordedExecutionStatus(t *testing.T, recorder *memoryRecorder, name string, kind string, status string) ExecutionSnapshot {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case snapshot := <-recorder.executions:
			if snapshot.TaskName == name && snapshot.Kind == kind && snapshot.Status == status {
				return snapshot
			}
		case <-deadline:
			t.Fatalf("timed out waiting for recorded execution %s/%s to become %s", name, kind, status)
		}
	}
}

func snapshotByName(t *testing.T, group *Group, name string) Snapshot {
	t.Helper()
	for _, snapshot := range group.Snapshots() {
		if snapshot.Name == name {
			return snapshot
		}
	}
	return Snapshot{}
}

func waitForCondition(t *testing.T, ok func() bool, description string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", description)
}
