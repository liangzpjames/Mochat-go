package dashboard

import (
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

func TestEmployeeStatisticApplyEventUnmarshalObjectAndLegacyPayloads(t *testing.T) {
	var objectEvent EmployeeStatisticApplyEvent
	if err := json.Unmarshal([]byte(`{"corpId":7,"tenantId":8,"source":" employeeStatistic "}`), &objectEvent); err != nil {
		t.Fatal(err)
	}
	if objectEvent.CorpID != 7 || objectEvent.TenantID != 8 || objectEvent.Source != " employeeStatistic " {
		t.Fatalf("object event = %+v", objectEvent)
	}

	var emptyLegacy EmployeeStatisticApplyEvent
	if err := json.Unmarshal([]byte(`[]`), &emptyLegacy); err != nil {
		t.Fatal(err)
	}
	if emptyLegacy.Source != "" {
		t.Fatalf("empty legacy event = %+v", emptyLegacy)
	}

	var legacyEvent EmployeeStatisticApplyEvent
	if err := json.Unmarshal([]byte(`["php.queue"]`), &legacyEvent); err != nil {
		t.Fatal(err)
	}
	if legacyEvent.Source != "php.queue" {
		t.Fatalf("legacy event = %+v", legacyEvent)
	}
}

func TestEmployeeStatisticApplyWorkerRunsCron(t *testing.T) {
	store := &fakeEmployeeStatisticStore{
		targets: []EmployeeStatisticTarget{{ID: 21, CorpID: 7, WXUserID: "go-user", WXCorpID: "ww-go", ContactSecret: "contact-secret"}},
	}
	cache := &fakeEmployeeStatisticCache{}
	client := &fakeEmployeeStatisticClient{
		behavior: []StatisticBehaviorData{{ChatCnt: 9, MessageCnt: 11, NegativeFeedbackCnt: 2, NewApplyCnt: 4, ReplyPercentage: 0.83, AvgReplyTime: 25}},
	}
	worker := NewEmployeeStatisticApplyWorker(nil, store, cache, client, log.Default())

	if err := worker.Process(context.Background(), EmployeeStatisticApplyEvent{Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if len(store.records) != 1 {
		t.Fatalf("records = %#v", store.records)
	}
	if store.records[0].EmployeeID != 21 || store.records[0].NewApplyCnt != 4 || store.records[0].ReplyPercentage != 83 {
		t.Fatalf("record = %#v", store.records[0])
	}
	if !cache.set || cache.corpID != 7 || cache.employeeID != 21 {
		t.Fatalf("cache = set:%v corp:%d employee:%d", cache.set, cache.corpID, cache.employeeID)
	}
}

func TestEmployeeStatisticApplyWorkerScopesCronByCorp(t *testing.T) {
	store := &fakeEmployeeStatisticStore{
		targets: []EmployeeStatisticTarget{
			{ID: 21, CorpID: 7, WXUserID: "go-user", WXCorpID: "ww-go", ContactSecret: "contact-secret"},
			{ID: 22, CorpID: 9, WXUserID: "other-user", WXCorpID: "ww-other", ContactSecret: "contact-secret"},
		},
	}
	cache := &fakeEmployeeStatisticCache{}
	client := &fakeEmployeeStatisticClient{behavior: []StatisticBehaviorData{{NewApplyCnt: 1}}}
	worker := NewEmployeeStatisticApplyWorker(nil, store, cache, client, log.Default())

	if err := worker.Process(context.Background(), EmployeeStatisticApplyEvent{CorpID: 7, Source: "tenant-scope"}); err != nil {
		t.Fatal(err)
	}
	if len(store.records) != 1 {
		t.Fatalf("records = %#v", store.records)
	}
	if store.records[0].CorpID != 7 || store.records[0].EmployeeID != 21 {
		t.Fatalf("record = %#v", store.records[0])
	}
}

func TestEmployeeStatisticApplyWorkerTenantScopeDoesNotFallbackToGlobal(t *testing.T) {
	store := &fakeEmployeeStatisticStore{
		targets: []EmployeeStatisticTarget{
			{ID: 21, CorpID: 7, WXUserID: "go-user", WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		},
		corpIDsByTenant: map[int][]int{8: []int{}},
	}
	cache := &fakeEmployeeStatisticCache{}
	client := &fakeEmployeeStatisticClient{behavior: []StatisticBehaviorData{{NewApplyCnt: 1}}}
	worker := NewEmployeeStatisticApplyWorker(nil, store, cache, client, log.Default())

	if err := worker.Process(context.Background(), EmployeeStatisticApplyEvent{TenantID: 8, Source: "tenant-empty"}); err != nil {
		t.Fatal(err)
	}
	if len(store.records) != 0 {
		t.Fatalf("tenant-scoped empty corp list should not process global records: %#v", store.records)
	}
}

func TestEmployeeStatisticApplyWorkerAcksSuccessfulDeliveryAndRecordsExecution(t *testing.T) {
	queue := &fakeEmployeeStatisticApplyWorkerQueue{}
	store := &fakeEmployeeStatisticStore{
		targets: []EmployeeStatisticTarget{{ID: 21, CorpID: 7, WXUserID: "go-user", WXCorpID: "ww-go", ContactSecret: "contact-secret"}},
		tenantByCorp: map[int]int{
			7: 8,
		},
		status: SaaSQuotaStatus{Metric: SaaSMetricAsyncExecutions, TenantID: 8, Current: 1, Limit: 10},
	}
	cache := &fakeEmployeeStatisticCache{}
	client := &fakeEmployeeStatisticClient{behavior: []StatisticBehaviorData{{NewApplyCnt: 1}}}
	recorder := &fakeWorkerExecutionRecorder{}
	ctx := taskrunner.WithTaskRuntime(context.Background(), QueueNameEmployeeStatisticApply, "run-statistic-1", recorder)
	worker := NewEmployeeStatisticApplyWorker(queue, store, cache, client, log.Default())

	worker.handleDelivery(ctx, EmployeeStatisticApplyDelivery{Raw: "raw-job", Event: EmployeeStatisticApplyEvent{CorpID: 7, Source: "test"}})

	if queue.ackedRaw != "raw-job" {
		t.Fatalf("acked raw = %q", queue.ackedRaw)
	}
	if queue.retryRaw != "" {
		t.Fatalf("unexpected retry raw = %q", queue.retryRaw)
	}
	running := recordedExecutionByStatus(t, recorder, QueueNameEmployeeStatisticApply, taskrunner.StatusRunning)
	succeeded := recordedExecutionByStatus(t, recorder, QueueNameEmployeeStatisticApply, taskrunner.StatusSucceeded)
	if running.ExecutionID == "" || succeeded.ExecutionID != running.ExecutionID || succeeded.RunID != "run-statistic-1" || succeeded.StoppedAt == "" || succeeded.Error != "" {
		t.Fatalf("executions = %+v", recorder.executions)
	}
	if running.TenantID != 8 || succeeded.TenantID != 8 {
		t.Fatalf("execution tenants: running=%d succeeded=%d", running.TenantID, succeeded.TenantID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricAsyncExecutions {
		t.Fatalf("refresh tenant=%d metric=%q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestEmployeeStatisticApplyWorkerRetriesFailedDelivery(t *testing.T) {
	queue := &fakeEmployeeStatisticApplyWorkerQueue{}
	worker := NewEmployeeStatisticApplyWorker(queue, nil, nil, nil, log.Default())

	worker.handleDelivery(context.Background(), EmployeeStatisticApplyDelivery{Raw: "raw-job", Attempts: 1, Event: EmployeeStatisticApplyEvent{Source: "test"}})

	if queue.ackedRaw != "" {
		t.Fatalf("unexpected ack raw = %q", queue.ackedRaw)
	}
	if queue.retryRaw != "raw-job" || queue.retryReason == "" || queue.retryMaxAttempts != 3 {
		t.Fatalf("retry = raw:%q reason:%q max:%d", queue.retryRaw, queue.retryReason, queue.retryMaxAttempts)
	}
}

type fakeEmployeeStatisticApplyWorkerQueue struct {
	ackedRaw         string
	retryRaw         string
	retryReason      string
	retryMaxAttempts int
	deadLettered     bool
}

func (q *fakeEmployeeStatisticApplyWorkerQueue) DequeueEmployeeStatisticApply(_ context.Context, _ time.Duration) (EmployeeStatisticApplyDelivery, bool, error) {
	return EmployeeStatisticApplyDelivery{}, false, nil
}

func (q *fakeEmployeeStatisticApplyWorkerQueue) AckEmployeeStatisticApply(_ context.Context, delivery EmployeeStatisticApplyDelivery) error {
	q.ackedRaw = delivery.Raw
	return nil
}

func (q *fakeEmployeeStatisticApplyWorkerQueue) RetryEmployeeStatisticApply(_ context.Context, delivery EmployeeStatisticApplyDelivery, reason string, maxAttempts int) (bool, error) {
	q.retryRaw = delivery.Raw
	q.retryReason = reason
	q.retryMaxAttempts = maxAttempts
	return q.deadLettered, nil
}

func (q *fakeEmployeeStatisticApplyWorkerQueue) RecoverEmployeeStatisticApplyProcessing(_ context.Context, _ time.Duration, _ int) (int, error) {
	return 0, nil
}
