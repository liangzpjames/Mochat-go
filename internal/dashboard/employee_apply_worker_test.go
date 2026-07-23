package dashboard

import (
	"context"
	"log"
	"reflect"
	"testing"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

func TestEmployeeApplyWorkerSyncsUniqueCorpIDs(t *testing.T) {
	store := &fakeEmployeeApplyWorkerStore{
		credentials: map[int]WorkEmployeeSyncCredential{
			7: {CorpID: 7, TenantID: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret", ContactSecret: "contact-secret"},
		},
	}
	client := &fakeEmployeeApplyWorkerClient{
		departments: []WorkEmployeeSyncDepartment{{WXDepartmentID: 1, Name: "总部"}, {WXDepartmentID: 2, Name: "销售部", WXParentID: 1}},
		users: map[int][]WorkEmployeeSyncEmployee{
			1: {{WXUserID: "go-worker-user", Name: "员工旧名", DepartmentIDs: []int{1}}},
			2: {{WXUserID: "go-worker-user", Name: "员工新名", Mobile: "13900000000", DepartmentIDs: []int{2}}},
		},
		followUsers: []string{"go-worker-user"},
	}
	worker := NewEmployeeApplyWorker(nil, store, client, "worker-secret", log.Default())

	if err := worker.Process(context.Background(), EmployeeApplyEvent{CorpIDs: []int{0, 7, 7}, UserID: 1, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.syncedCorpIDs, []int{7}) {
		t.Fatalf("synced corp ids = %#v", store.syncedCorpIDs)
	}
	if len(store.syncedEmployees) != 1 || store.syncedEmployees[0].WXUserID != "go-worker-user" || store.syncedEmployees[0].Name != "员工新名" {
		t.Fatalf("synced employees = %#v", store.syncedEmployees)
	}
	if !reflect.DeepEqual(store.followUserIDs, []string{"go-worker-user"}) {
		t.Fatalf("follow users = %#v", store.followUserIDs)
	}
	if store.defaultPasswordHash == "" {
		t.Fatalf("default password hash should not be empty")
	}
}

func TestEmployeeApplyWorkerRequiresCorpIDs(t *testing.T) {
	worker := NewEmployeeApplyWorker(nil, &fakeEmployeeApplyWorkerStore{}, &fakeEmployeeApplyWorkerClient{}, "worker-secret", log.Default())
	if err := worker.Process(context.Background(), EmployeeApplyEvent{}); err == nil {
		t.Fatalf("expected missing corp ids error")
	}
}

func TestEmployeeApplyWorkerAcksSuccessfulDelivery(t *testing.T) {
	queue := &fakeEmployeeApplyWorkerQueue{}
	store := &fakeEmployeeApplyWorkerStore{
		tenantIDs: map[int]int{7: 11},
		credentials: map[int]WorkEmployeeSyncCredential{
			7: {CorpID: 7, TenantID: 11, WXCorpID: "ww-go", EmployeeSecret: "employee-secret"},
		},
	}
	client := &fakeEmployeeApplyWorkerClient{
		departments: []WorkEmployeeSyncDepartment{{WXDepartmentID: 1, Name: "总部"}},
		users:       map[int][]WorkEmployeeSyncEmployee{1: {{WXUserID: "go-user", Name: "Go员工", DepartmentIDs: []int{1}}}},
	}
	worker := NewEmployeeApplyWorker(queue, store, client, "worker-secret", log.Default())

	worker.handleDelivery(context.Background(), EmployeeApplyDelivery{Raw: "raw-job", Event: EmployeeApplyEvent{CorpIDs: []int{7}}})

	if queue.ackedRaw != "raw-job" {
		t.Fatalf("acked raw = %q", queue.ackedRaw)
	}
	if queue.retryRaw != "" {
		t.Fatalf("unexpected retry raw = %q", queue.retryRaw)
	}
}

func TestEmployeeApplyWorkerRecordsQueueItemExecution(t *testing.T) {
	queue := &fakeEmployeeApplyWorkerQueue{}
	store := &fakeEmployeeApplyWorkerStore{
		tenantIDs: map[int]int{7: 11},
		credentials: map[int]WorkEmployeeSyncCredential{
			7: {CorpID: 7, TenantID: 11, WXCorpID: "ww-go", EmployeeSecret: "employee-secret"},
		},
	}
	client := &fakeEmployeeApplyWorkerClient{
		departments: []WorkEmployeeSyncDepartment{{WXDepartmentID: 1, Name: "总部"}},
		users:       map[int][]WorkEmployeeSyncEmployee{1: {{WXUserID: "go-user", Name: "Go员工", DepartmentIDs: []int{1}}}},
	}
	recorder := &fakeWorkerExecutionRecorder{}
	ctx := taskrunner.WithTaskRuntime(context.Background(), "employee-apply", "run-employee-1", recorder)
	worker := NewEmployeeApplyWorker(queue, store, client, "worker-secret", log.Default())

	worker.handleDelivery(ctx, EmployeeApplyDelivery{Raw: "raw-job", Event: EmployeeApplyEvent{CorpIDs: []int{7}}})

	running := recordedExecutionByStatus(t, recorder, "employee-apply", taskrunner.StatusRunning)
	succeeded := recordedExecutionByStatus(t, recorder, "employee-apply", taskrunner.StatusSucceeded)
	if running.ExecutionID == "" || running.TenantID != 11 || succeeded.TenantID != 11 || succeeded.ExecutionID != running.ExecutionID || succeeded.RunID != "run-employee-1" || succeeded.StoppedAt == "" || succeeded.Error != "" {
		t.Fatalf("executions = %+v", recorder.executions)
	}
}

func TestEmployeeApplyWorkerRetriesFailedDelivery(t *testing.T) {
	queue := &fakeEmployeeApplyWorkerQueue{}
	worker := NewEmployeeApplyWorker(queue, &fakeEmployeeApplyWorkerStore{credentials: map[int]WorkEmployeeSyncCredential{}}, &fakeEmployeeApplyWorkerClient{}, "worker-secret", log.Default())

	worker.handleDelivery(context.Background(), EmployeeApplyDelivery{Raw: "raw-job", Attempts: 1, Event: EmployeeApplyEvent{CorpIDs: []int{7}, Source: "test"}})

	if queue.ackedRaw != "" {
		t.Fatalf("unexpected ack raw = %q", queue.ackedRaw)
	}
	if queue.retryRaw != "raw-job" || queue.retryReason == "" || queue.retryMaxAttempts != 3 {
		t.Fatalf("retry = raw:%q reason:%q max:%d", queue.retryRaw, queue.retryReason, queue.retryMaxAttempts)
	}
}

func TestEmployeeApplyWorkerRecordsFailedQueueItemExecution(t *testing.T) {
	queue := &fakeEmployeeApplyWorkerQueue{}
	recorder := &fakeWorkerExecutionRecorder{}
	ctx := taskrunner.WithTaskRuntime(context.Background(), "employee-apply", "run-employee-1", recorder)
	worker := NewEmployeeApplyWorker(queue, &fakeEmployeeApplyWorkerStore{credentials: map[int]WorkEmployeeSyncCredential{}}, &fakeEmployeeApplyWorkerClient{}, "worker-secret", log.Default())

	worker.handleDelivery(ctx, EmployeeApplyDelivery{Raw: "raw-job", Attempts: 1, Event: EmployeeApplyEvent{CorpIDs: []int{7}, Source: "test"}})

	failed := recordedExecutionByStatus(t, recorder, "employee-apply", taskrunner.StatusFailed)
	if failed.ExecutionID == "" || failed.RunID != "run-employee-1" || failed.StoppedAt == "" || failed.Error == "" {
		t.Fatalf("executions = %+v", recorder.executions)
	}
}

type fakeEmployeeApplyWorkerStore struct {
	tenantIDs           map[int]int
	credentials         map[int]WorkEmployeeSyncCredential
	syncedCorpIDs       []int
	syncedDepartments   []WorkEmployeeSyncDepartment
	syncedEmployees     []WorkEmployeeSyncEmployee
	followUserIDs       []string
	defaultPasswordHash string
}

func (s *fakeEmployeeApplyWorkerStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	return s.tenantIDs[corpID], nil
}

func (s *fakeEmployeeApplyWorkerStore) WorkEmployeeSyncCredentials(_ context.Context, corpIDs []int) ([]WorkEmployeeSyncCredential, error) {
	out := make([]WorkEmployeeSyncCredential, 0, len(corpIDs))
	for _, corpID := range corpIDs {
		if credential, ok := s.credentials[corpID]; ok {
			out = append(out, credential)
		}
	}
	return out, nil
}

func (s *fakeEmployeeApplyWorkerStore) SyncWorkEmployees(_ context.Context, credential WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee, followUserIDs []string, defaultPasswordHash string) (WorkEmployeeSyncResult, error) {
	s.syncedCorpIDs = append(s.syncedCorpIDs, credential.CorpID)
	s.syncedDepartments = append([]WorkEmployeeSyncDepartment{}, departments...)
	s.syncedEmployees = append([]WorkEmployeeSyncEmployee{}, employees...)
	s.followUserIDs = append([]string{}, followUserIDs...)
	s.defaultPasswordHash = defaultPasswordHash
	return WorkEmployeeSyncResult{}, nil
}

type fakeEmployeeApplyWorkerClient struct {
	departments []WorkEmployeeSyncDepartment
	users       map[int][]WorkEmployeeSyncEmployee
	followUsers []string
}

func (c *fakeEmployeeApplyWorkerClient) Departments(_ context.Context, _ WorkEmployeeSyncCredential) ([]WorkEmployeeSyncDepartment, error) {
	return append([]WorkEmployeeSyncDepartment{}, c.departments...), nil
}

func (c *fakeEmployeeApplyWorkerClient) DepartmentUsers(_ context.Context, _ WorkEmployeeSyncCredential, wxDepartmentID int) ([]WorkEmployeeSyncEmployee, error) {
	return append([]WorkEmployeeSyncEmployee{}, c.users[wxDepartmentID]...), nil
}

func (c *fakeEmployeeApplyWorkerClient) FollowUsers(_ context.Context, _ WorkEmployeeSyncCredential) ([]string, error) {
	return append([]string{}, c.followUsers...), nil
}

type fakeEmployeeApplyWorkerQueue struct {
	ackedRaw         string
	retryRaw         string
	retryReason      string
	retryMaxAttempts int
	deadLettered     bool
}

func (q *fakeEmployeeApplyWorkerQueue) DequeueEmployeeApply(_ context.Context, _ time.Duration) (EmployeeApplyDelivery, bool, error) {
	return EmployeeApplyDelivery{}, false, nil
}

func (q *fakeEmployeeApplyWorkerQueue) AckEmployeeApply(_ context.Context, delivery EmployeeApplyDelivery) error {
	q.ackedRaw = delivery.Raw
	return nil
}

func (q *fakeEmployeeApplyWorkerQueue) RetryEmployeeApply(_ context.Context, delivery EmployeeApplyDelivery, reason string, maxAttempts int) (bool, error) {
	q.retryRaw = delivery.Raw
	q.retryReason = reason
	q.retryMaxAttempts = maxAttempts
	return q.deadLettered, nil
}

func (q *fakeEmployeeApplyWorkerQueue) RecoverEmployeeApplyProcessing(_ context.Context, _ time.Duration, _ int) (int, error) {
	return 0, nil
}
