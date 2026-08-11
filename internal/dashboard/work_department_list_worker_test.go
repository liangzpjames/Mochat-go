package dashboard

import (
	"context"
	"encoding/json"
	"log"
	"reflect"
	"testing"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

func TestWorkDepartmentListEventUnmarshalObjectAndLegacyPayloads(t *testing.T) {
	var objectEvent WorkDepartmentListEvent
	if err := json.Unmarshal([]byte(`{"corp_ids":["7",2,7],"user_id":"5","tenant_id":11,"source":" dashboard.department.list "}`), &objectEvent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(objectEvent.CorpIDs, []int{7, 2, 7}) || objectEvent.UserID != 5 || objectEvent.TenantID != 11 || objectEvent.Source != " dashboard.department.list " {
		t.Fatalf("object event = %+v", objectEvent)
	}

	var scalarLegacy WorkDepartmentListEvent
	if err := json.Unmarshal([]byte(`[7,"2",0]`), &scalarLegacy); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scalarLegacy.CorpIDs, []int{7, 2}) || scalarLegacy.UserID != 0 || scalarLegacy.Source != "" {
		t.Fatalf("scalar legacy event = %+v", scalarLegacy)
	}

	var listLegacy WorkDepartmentListEvent
	if err := json.Unmarshal([]byte(`[[7,"2"],5,"php.queue"]`), &listLegacy); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(listLegacy.CorpIDs, []int{7, 2}) || listLegacy.UserID != 5 || listLegacy.Source != "php.queue" {
		t.Fatalf("list legacy event = %+v", listLegacy)
	}
}

func TestWorkDepartmentListWorkerResolvesCorpIDsFromUserAndSyncs(t *testing.T) {
	store := &fakeWorkDepartmentListStore{
		userCorpIDs: map[int][]int{5: {7, 2, 7}},
		credentials: map[int]WorkEmployeeSyncCredential{
			2: {CorpID: 2, TenantID: 11, WXCorpID: "ww-go-2", EmployeeSecret: "employee-secret-2"},
			7: {CorpID: 7, TenantID: 11, WXCorpID: "ww-go-7", EmployeeSecret: "employee-secret-7"},
		},
	}
	client := &fakeEmployeeApplyWorkerClient{
		departments: []WorkEmployeeSyncDepartment{{WXDepartmentID: 1, Name: "总部"}, {WXDepartmentID: 2, Name: "销售部", WXParentID: 1}},
		users: map[int][]WorkEmployeeSyncEmployee{
			1: {{WXUserID: "go-user", Name: "旧名", DepartmentIDs: []int{1}}},
			2: {{WXUserID: "go-user", Name: "新名", Mobile: "13900000000", DepartmentIDs: []int{2}}},
		},
		followUsers: []string{"go-user"},
	}
	worker := NewWorkDepartmentListWorker(nil, store, client, "worker-secret", log.Default())

	if err := worker.Process(context.Background(), WorkDepartmentListEvent{UserID: 5, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if store.lastCorpIDsByUserID != 5 {
		t.Fatalf("CorpIDsByUser user id = %d", store.lastCorpIDsByUserID)
	}
	if !reflect.DeepEqual(store.syncedCorpIDs, []int{2, 7}) {
		t.Fatalf("synced corp ids = %#v", store.syncedCorpIDs)
	}
	if len(store.syncedEmployeesByCorp[2]) != 1 || store.syncedEmployeesByCorp[2][0].WXUserID != "go-user" || store.syncedEmployeesByCorp[2][0].Name != "新名" {
		t.Fatalf("synced employees for corp 2 = %#v", store.syncedEmployeesByCorp[2])
	}
	if store.defaultPasswordHashByCorp[2] != "" || store.defaultPasswordHashByCorp[7] != "" {
		t.Fatalf("employee sync must not create login password hashes = %#v", store.defaultPasswordHashByCorp)
	}
}

func TestWorkDepartmentListWorkerResolvesCorpIDsFromTenant(t *testing.T) {
	store := &fakeWorkDepartmentListStore{
		tenantCorpIDs: map[int][]int{11: {9, 3, 9}},
		credentials: map[int]WorkEmployeeSyncCredential{
			3: {CorpID: 3, TenantID: 11, WXCorpID: "ww-go-3", EmployeeSecret: "employee-secret-3"},
			9: {CorpID: 9, TenantID: 11, WXCorpID: "ww-go-9", EmployeeSecret: "employee-secret-9"},
		},
	}
	client := &fakeEmployeeApplyWorkerClient{
		departments: []WorkEmployeeSyncDepartment{{WXDepartmentID: 1, Name: "总部"}},
		users:       map[int][]WorkEmployeeSyncEmployee{1: {{WXUserID: "go-user", Name: "Go员工", DepartmentIDs: []int{1}}}},
	}
	worker := NewWorkDepartmentListWorker(nil, store, client, "worker-secret", log.Default())

	if err := worker.Process(context.Background(), WorkDepartmentListEvent{TenantID: 11, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if store.lastCorpIDsByTenantID != 11 {
		t.Fatalf("CorpIDsByTenant tenant id = %d", store.lastCorpIDsByTenantID)
	}
	if !reflect.DeepEqual(store.syncedCorpIDs, []int{3, 9}) {
		t.Fatalf("synced corp ids = %#v", store.syncedCorpIDs)
	}
}

func TestWorkDepartmentListWorkerRequiresResolvableCorpIDs(t *testing.T) {
	worker := NewWorkDepartmentListWorker(nil, &fakeWorkDepartmentListStore{}, &fakeEmployeeApplyWorkerClient{}, "worker-secret", log.Default())
	if err := worker.Process(context.Background(), WorkDepartmentListEvent{}); err == nil {
		t.Fatalf("expected missing corp ids error")
	}
}

func TestWorkDepartmentListWorkerAcksSuccessfulDeliveryAndRecordsExecution(t *testing.T) {
	queue := &fakeWorkDepartmentListQueue{}
	store := &fakeWorkDepartmentListStore{
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
	ctx := taskrunner.WithTaskRuntime(context.Background(), QueueNameWorkDepartmentList, "run-department-1", recorder)
	worker := NewWorkDepartmentListWorker(queue, store, client, "worker-secret", log.Default())

	worker.handleDelivery(ctx, WorkDepartmentListDelivery{Raw: "raw-job", Event: WorkDepartmentListEvent{CorpIDs: []int{7}}})

	if queue.ackedRaw != "raw-job" {
		t.Fatalf("acked raw = %q", queue.ackedRaw)
	}
	if queue.retryRaw != "" {
		t.Fatalf("unexpected retry raw = %q", queue.retryRaw)
	}
	running := recordedExecutionByStatus(t, recorder, QueueNameWorkDepartmentList, taskrunner.StatusRunning)
	succeeded := recordedExecutionByStatus(t, recorder, QueueNameWorkDepartmentList, taskrunner.StatusSucceeded)
	if running.ExecutionID == "" || running.TenantID != 11 || succeeded.TenantID != 11 || succeeded.ExecutionID != running.ExecutionID || succeeded.RunID != "run-department-1" || succeeded.StoppedAt == "" || succeeded.Error != "" {
		t.Fatalf("executions = %+v", recorder.executions)
	}
}

func TestWorkDepartmentListWorkerRetriesFailedDelivery(t *testing.T) {
	queue := &fakeWorkDepartmentListQueue{}
	worker := NewWorkDepartmentListWorker(queue, &fakeWorkDepartmentListStore{credentials: map[int]WorkEmployeeSyncCredential{}}, &fakeEmployeeApplyWorkerClient{}, "worker-secret", log.Default())

	worker.handleDelivery(context.Background(), WorkDepartmentListDelivery{Raw: "raw-job", Attempts: 1, Event: WorkDepartmentListEvent{CorpIDs: []int{7}, Source: "test"}})

	if queue.ackedRaw != "" {
		t.Fatalf("unexpected ack raw = %q", queue.ackedRaw)
	}
	if queue.retryRaw != "raw-job" || queue.retryReason == "" || queue.retryMaxAttempts != 3 {
		t.Fatalf("retry = raw:%q reason:%q max:%d", queue.retryRaw, queue.retryReason, queue.retryMaxAttempts)
	}
}

type fakeWorkDepartmentListStore struct {
	tenantIDs                 map[int]int
	userCorpIDs               map[int][]int
	tenantCorpIDs             map[int][]int
	credentials               map[int]WorkEmployeeSyncCredential
	lastCorpIDsByUserID       int
	lastCorpIDsByTenantID     int
	syncedCorpIDs             []int
	syncedDepartmentsByCorp   map[int][]WorkEmployeeSyncDepartment
	syncedEmployeesByCorp     map[int][]WorkEmployeeSyncEmployee
	followUserIDsByCorp       map[int][]string
	defaultPasswordHashByCorp map[int]string
}

func (s *fakeWorkDepartmentListStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	return s.tenantIDs[corpID], nil
}

func (s *fakeWorkDepartmentListStore) CorpIDsByUser(_ context.Context, userID int) ([]int, error) {
	s.lastCorpIDsByUserID = userID
	return append([]int{}, s.userCorpIDs[userID]...), nil
}

func (s *fakeWorkDepartmentListStore) CorpIDsByTenant(_ context.Context, tenantID int) ([]int, error) {
	s.lastCorpIDsByTenantID = tenantID
	return append([]int{}, s.tenantCorpIDs[tenantID]...), nil
}

func (s *fakeWorkDepartmentListStore) WorkEmployeeSyncCredentials(_ context.Context, corpIDs []int) ([]WorkEmployeeSyncCredential, error) {
	out := make([]WorkEmployeeSyncCredential, 0, len(corpIDs))
	for _, corpID := range corpIDs {
		if credential, ok := s.credentials[corpID]; ok {
			out = append(out, credential)
		}
	}
	return out, nil
}

func (s *fakeWorkDepartmentListStore) SyncWorkEmployees(_ context.Context, credential WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee, followUserIDs []string, defaultPasswordHash string) (WorkEmployeeSyncResult, error) {
	if s.syncedDepartmentsByCorp == nil {
		s.syncedDepartmentsByCorp = map[int][]WorkEmployeeSyncDepartment{}
	}
	if s.syncedEmployeesByCorp == nil {
		s.syncedEmployeesByCorp = map[int][]WorkEmployeeSyncEmployee{}
	}
	if s.followUserIDsByCorp == nil {
		s.followUserIDsByCorp = map[int][]string{}
	}
	if s.defaultPasswordHashByCorp == nil {
		s.defaultPasswordHashByCorp = map[int]string{}
	}
	s.syncedCorpIDs = append(s.syncedCorpIDs, credential.CorpID)
	s.syncedDepartmentsByCorp[credential.CorpID] = append([]WorkEmployeeSyncDepartment{}, departments...)
	s.syncedEmployeesByCorp[credential.CorpID] = append([]WorkEmployeeSyncEmployee{}, employees...)
	s.followUserIDsByCorp[credential.CorpID] = append([]string{}, followUserIDs...)
	s.defaultPasswordHashByCorp[credential.CorpID] = defaultPasswordHash
	return WorkEmployeeSyncResult{}, nil
}

type fakeWorkDepartmentListQueue struct {
	ackedRaw         string
	retryRaw         string
	retryReason      string
	retryMaxAttempts int
	deadLettered     bool
}

func (q *fakeWorkDepartmentListQueue) DequeueWorkDepartmentList(_ context.Context, _ time.Duration) (WorkDepartmentListDelivery, bool, error) {
	return WorkDepartmentListDelivery{}, false, nil
}

func (q *fakeWorkDepartmentListQueue) AckWorkDepartmentList(_ context.Context, delivery WorkDepartmentListDelivery) error {
	q.ackedRaw = delivery.Raw
	return nil
}

func (q *fakeWorkDepartmentListQueue) RetryWorkDepartmentList(_ context.Context, delivery WorkDepartmentListDelivery, reason string, maxAttempts int) (bool, error) {
	q.retryRaw = delivery.Raw
	q.retryReason = reason
	q.retryMaxAttempts = maxAttempts
	return q.deadLettered, nil
}

func (q *fakeWorkDepartmentListQueue) RecoverWorkDepartmentListProcessing(_ context.Context, _ time.Duration, _ int) (int, error) {
	return 0, nil
}
