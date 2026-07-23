package dashboard

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/taskrunner"
)

type fakeWorkerExecutionRecorder struct {
	executions []taskrunner.ExecutionSnapshot
}

func (r *fakeWorkerExecutionRecorder) RecordTaskSnapshot(context.Context, taskrunner.Snapshot) error {
	return nil
}

func (r *fakeWorkerExecutionRecorder) RecordTaskExecution(_ context.Context, snapshot taskrunner.ExecutionSnapshot) error {
	r.executions = append(r.executions, snapshot)
	return nil
}

func recordedExecutionByStatus(t *testing.T, recorder *fakeWorkerExecutionRecorder, taskName string, status string) taskrunner.ExecutionSnapshot {
	t.Helper()
	for _, execution := range recorder.executions {
		if execution.TaskName == taskName && execution.Kind == taskrunner.ExecutionKindQueueItem && execution.Status == status {
			return execution
		}
	}
	t.Fatalf("execution not found: task=%s status=%s executions=%+v", taskName, status, recorder.executions)
	return taskrunner.ExecutionSnapshot{}
}

func TestStartQueueItemExecutionRefreshesAsyncUsage(t *testing.T) {
	recorder := &fakeWorkerExecutionRecorder{}
	store := &fakeQueueExecutionQuotaStore{
		status: SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 8, Current: 2, Limit: 10},
	}
	ctx := taskrunner.WithTaskRuntime(context.Background(), "wework-callback", "run-1", recorder)

	finish := startQueueItemExecution(ctx, log.Default(), "wework-callback", store, 8)
	finish(taskrunner.StatusSucceeded, nil)

	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricAsyncExecutions {
		t.Fatalf("refresh tenant=%d metric=%q", store.refreshTenantID, store.refreshMetric)
	}
	if store.statusTenantID != 8 || store.statusMetric != SaaSMetricAsyncExecutions || store.statusAdditional != 0 {
		t.Fatalf("status tenant=%d metric=%q additional=%d", store.statusTenantID, store.statusMetric, store.statusAdditional)
	}
	if store.alertRecorded {
		t.Fatal("alert should not be recorded when usage is within limit")
	}
	succeeded := recordedExecutionByStatus(t, recorder, "wework-callback", taskrunner.StatusSucceeded)
	if succeeded.TenantID != 8 {
		t.Fatalf("execution tenant = %d", succeeded.TenantID)
	}
}

func TestStartQueueItemExecutionLogsAsyncUsageOverLimit(t *testing.T) {
	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)
	recorder := &fakeWorkerExecutionRecorder{}
	store := &fakeQueueExecutionQuotaStore{
		status: SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 8, Current: 11, Limit: 10},
	}
	ctx := taskrunner.WithTaskRuntime(context.Background(), "contact-welcome", "run-2", recorder)

	finish := startQueueItemExecution(ctx, logger, "contact-welcome", store, 8)
	finish(taskrunner.StatusFailed, context.Canceled)

	if !strings.Contains(logs.String(), "async execution SaaS quota exceeded: tenant=8 current=11 limit=10") {
		t.Fatalf("logs = %q", logs.String())
	}
	if !store.alertRecorded {
		t.Fatal("alert was not recorded")
	}
	if store.alert.Status.TenantID != 8 || store.alert.Status.Metric != SaaSMetricAsyncExecutions || store.alert.Status.Current != 11 || store.alert.Status.Limit != 10 {
		t.Fatalf("alert status = %+v", store.alert.Status)
	}
	if store.alert.AlertType != SaaSAlertTypeQuotaExceeded || store.alert.Severity != SaaSAlertSeverityWarning || store.alert.PeriodKey != SaaSAlertPeriodLifetime || store.alert.Source != "worker.queue_item" {
		t.Fatalf("alert metadata = %+v", store.alert)
	}
}

func TestStartQueueItemExecutionNotifiesAsyncUsageOverLimit(t *testing.T) {
	recorder := &fakeWorkerExecutionRecorder{}
	store := &fakeQueueExecutionQuotaStore{
		status: SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 8, Current: 12, Limit: 10},
	}
	notifier := &fakeSaaSAlertNotifier{}
	ctx := taskrunner.WithTaskRuntime(context.Background(), "employee-apply", "run-3", recorder)
	ctx = WithSaaSAlertNotifier(ctx, notifier)

	finish := startQueueItemExecution(ctx, log.Default(), "employee-apply", store, 8)
	finish(taskrunner.StatusSucceeded, nil)

	if notifier.called != 1 {
		t.Fatalf("notifier called %d times", notifier.called)
	}
	if notifier.alert.Status.TenantID != 8 || notifier.alert.Status.Metric != SaaSMetricAsyncExecutions || notifier.alert.Status.Current != 12 || notifier.alert.Status.Limit != 10 {
		t.Fatalf("notified alert = %+v", notifier.alert)
	}
}

type fakeQueueExecutionQuotaStore struct {
	status           SaaSQuotaStatus
	refreshTenantID  int
	refreshMetric    string
	statusTenantID   int
	statusMetric     string
	statusAdditional int64
	alertRecorded    bool
	alert            SaaSQuotaAlert
}

func (s *fakeQueueExecutionQuotaStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}

func (s *fakeQueueExecutionQuotaStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.statusTenantID = tenantID
	s.statusMetric = metric
	s.statusAdditional = additional
	return s.status, nil
}

func (s *fakeQueueExecutionQuotaStore) RecordSaaSQuotaAlert(_ context.Context, alert SaaSQuotaAlert) error {
	s.alertRecorded = true
	s.alert = alert
	return nil
}

type fakeSaaSAlertNotifier struct {
	called int
	alert  SaaSQuotaAlert
}

func (n *fakeSaaSAlertNotifier) NotifySaaSQuotaAlert(_ context.Context, alert SaaSQuotaAlert) error {
	n.called++
	n.alert = alert
	return nil
}
