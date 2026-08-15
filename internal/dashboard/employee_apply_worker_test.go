package dashboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

func TestEmployeeApplyWorkerSyncsUniqueCorpIDs(t *testing.T) {
	store := &fakeEmployeeApplyWorkerStore{
		credentials: map[int]WorkEmployeeSyncCredential{
			7: {CorpID: 7, TenantID: 7, CredentialVersion: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret", ContactSecret: "contact-secret"},
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
	worker := NewEmployeeApplyWorker(nil, store, client, log.Default())

	if err := worker.Process(context.Background(), EmployeeApplyEvent{BindingID: 7, Source: "test", QueueTicket: "ticket-1"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.syncedCorpIDs, []int{7}) {
		t.Fatalf("synced corp ids = %#v", store.syncedCorpIDs)
	}
	if len(store.syncedEmployees) != 1 || store.syncedEmployees[0].WXUserID != "go-worker-user" || store.syncedEmployees[0].Name != "员工新名" {
		t.Fatalf("synced employees = %#v", store.syncedEmployees)
	}
	if store.followUserIDs != nil {
		t.Fatalf("follow users = %#v", store.followUserIDs)
	}
	if store.defaultPasswordHash != "" {
		t.Fatalf("employee sync must not create password hashes")
	}
}

func TestEmployeeApplyWorkerRequiresCorpIDs(t *testing.T) {
	worker := NewEmployeeApplyWorker(nil, &fakeEmployeeApplyWorkerStore{}, &fakeEmployeeApplyWorkerClient{}, log.Default())
	if err := worker.Process(context.Background(), EmployeeApplyEvent{}); err == nil {
		t.Fatalf("expected missing corp ids error")
	}
}

func TestEmployeeApplyWorkerRequiresQueueTicketBeforeProviderFetch(t *testing.T) {
	store := &versionFenceEmployeeApplyStore{credential: WorkEmployeeSyncCredential{
		CorpID: 7, TenantID: 7, CredentialVersion: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret",
	}, currentVersion: 1}
	client := &countingEmployeeApplyClient{}
	worker := NewEmployeeApplyWorker(nil, store, client, log.Default())
	if err := worker.Process(context.Background(), EmployeeApplyEvent{BindingID: 7}); err == nil || !strings.Contains(err.Error(), "queue ticket") {
		t.Fatalf("error=%v, want missing queue ticket", err)
	}
	if client.departmentCalls != 0 || store.businessWrites != 0 || store.markerWrites != 0 {
		t.Fatalf("missing-ticket worker fetched/wrote: departments=%d business=%d markers=%d", client.departmentCalls, store.businessWrites, store.markerWrites)
	}
}

func TestEmployeeApplyWorkerFencesRotationAfterExternalFetch(t *testing.T) {
	store := &versionFenceEmployeeApplyStore{credential: WorkEmployeeSyncCredential{
		CorpID: 7, TenantID: 7, CredentialVersion: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret",
	}, currentVersion: 1}
	client := &rotatingEmployeeApplyClient{store: store}
	err := syncCompanyEmployeesForBinding(context.Background(), store, client, 7, "ticket-1")
	if err == nil || !strings.Contains(err.Error(), "credential version stale") {
		t.Fatalf("sync error=%v, want fenced stale version", err)
	}
	if store.businessWrites != 0 {
		t.Fatalf("business writes=%d, want zero after rotation during fetch", store.businessWrites)
	}
	if store.markerWrites != 0 {
		t.Fatalf("marker writes=%d, want zero overwrite by stale worker", store.markerWrites)
	}
}

func TestEmployeeApplyWorkerRejectsCredentialFromWrongTenantBeforeFetch(t *testing.T) {
	store := &versionFenceEmployeeApplyStore{
		credential: WorkEmployeeSyncCredential{
			CorpID: 7, TenantID: 22, CredentialVersion: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret",
		},
		bindingTenantID: 11,
		currentVersion:  1,
	}
	client := &countingEmployeeApplyClient{}
	if err := syncCompanyEmployeesForBinding(context.Background(), store, client, 7, "ticket-1"); err == nil || !strings.Contains(err.Error(), "tenant scope") {
		t.Fatalf("sync error=%v, want tenant scope rejection", err)
	}
	if client.departmentCalls != 0 || store.businessWrites != 0 || store.markerWrites != 0 {
		t.Fatalf("wrong-tenant sync fetched/wrote: departments=%d business=%d markers=%d", client.departmentCalls, store.businessWrites, store.markerWrites)
	}
}

func TestEmployeeApplyWorkerAcksSuccessfulDelivery(t *testing.T) {
	queue := &fakeEmployeeApplyWorkerQueue{}
	store := &fakeEmployeeApplyWorkerStore{
		tenantIDs: map[int]int{7: 11},
		credentials: map[int]WorkEmployeeSyncCredential{
			7: {CorpID: 7, TenantID: 11, CredentialVersion: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret"},
		},
	}
	client := &fakeEmployeeApplyWorkerClient{
		departments: []WorkEmployeeSyncDepartment{{WXDepartmentID: 1, Name: "总部"}},
		users:       map[int][]WorkEmployeeSyncEmployee{1: {{WXUserID: "go-user", Name: "Go员工", DepartmentIDs: []int{1}}}},
	}
	worker := NewEmployeeApplyWorker(queue, store, client, log.Default())

	worker.handleDelivery(context.Background(), EmployeeApplyDelivery{Raw: "raw-job", Event: EmployeeApplyEvent{BindingID: 7, QueueTicket: "ticket-1"}})

	if queue.ackedRaw != "raw-job" {
		t.Fatalf("acked raw = %q", queue.ackedRaw)
	}
	if store.beginCalls != 1 || store.failureCalls != 0 {
		t.Fatalf("lifecycle begin=%d failure=%d", store.beginCalls, store.failureCalls)
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
			7: {CorpID: 7, TenantID: 11, CredentialVersion: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret"},
		},
	}
	client := &fakeEmployeeApplyWorkerClient{
		departments: []WorkEmployeeSyncDepartment{{WXDepartmentID: 1, Name: "总部"}},
		users:       map[int][]WorkEmployeeSyncEmployee{1: {{WXUserID: "go-user", Name: "Go员工", DepartmentIDs: []int{1}}}},
	}
	recorder := &fakeWorkerExecutionRecorder{}
	ctx := taskrunner.WithTaskRuntime(context.Background(), "employee-apply", "run-employee-1", recorder)
	worker := NewEmployeeApplyWorker(queue, store, client, log.Default())

	worker.handleDelivery(ctx, EmployeeApplyDelivery{Raw: "raw-job", Event: EmployeeApplyEvent{BindingID: 7, QueueTicket: "ticket-1"}})

	running := recordedExecutionByStatus(t, recorder, "employee-apply", taskrunner.StatusRunning)
	succeeded := recordedExecutionByStatus(t, recorder, "employee-apply", taskrunner.StatusSucceeded)
	if running.ExecutionID == "" || running.TenantID != 11 || succeeded.TenantID != 11 || succeeded.ExecutionID != running.ExecutionID || succeeded.RunID != "run-employee-1" || succeeded.StoppedAt == "" || succeeded.Error != "" {
		t.Fatalf("executions = %+v", recorder.executions)
	}
}

func TestEmployeeApplyWorkerRetriesFailedDelivery(t *testing.T) {
	queue := &fakeEmployeeApplyWorkerQueue{}
	worker := NewEmployeeApplyWorker(queue, &fakeEmployeeApplyWorkerStore{credentials: map[int]WorkEmployeeSyncCredential{}}, &fakeEmployeeApplyWorkerClient{}, log.Default())

	worker.handleDelivery(context.Background(), EmployeeApplyDelivery{Raw: "raw-job", Attempts: 1, Event: EmployeeApplyEvent{BindingID: 7, Source: "test"}})

	if queue.ackedRaw != "" {
		t.Fatalf("unexpected ack raw = %q", queue.ackedRaw)
	}
	if queue.retryRaw != "raw-job" || queue.retryReason == "" || queue.retryMaxAttempts != 3 {
		t.Fatalf("retry = raw:%q reason:%q max:%d", queue.retryRaw, queue.retryReason, queue.retryMaxAttempts)
	}
}

func TestEmployeeApplyWorkerMarksRunningAndSanitizesProviderFailure(t *testing.T) {
	queue := &fakeEmployeeApplyWorkerQueue{}
	store := &lifecycleEmployeeApplyWorkerStore{
		fakeEmployeeApplyWorkerStore: &fakeEmployeeApplyWorkerStore{
			tenantIDs: map[int]int{7: 11},
			credentials: map[int]WorkEmployeeSyncCredential{
				7: {CorpID: 7, TenantID: 11, CredentialVersion: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret"},
			},
		},
	}
	client := &failingEmployeeApplyWorkerClient{err: fmt.Errorf("provider secret employee-secret failed")}
	var logs bytes.Buffer
	worker := NewEmployeeApplyWorker(queue, store, client, log.New(&logs, "", 0))

	worker.handleDelivery(context.Background(), EmployeeApplyDelivery{Raw: "raw-job", Event: EmployeeApplyEvent{BindingID: 7, Source: "dashboard.company.employee-sync", QueueTicket: "ticket-1"}})

	if store.beginCalls != 1 || store.queuedCalls != 1 || store.failureCalls != 0 || store.queuedErrorCode != "SYNC_FAILED" {
		t.Fatalf("lifecycle begin=%d queued=%d failure=%d errorCode=%q", store.beginCalls, store.queuedCalls, store.failureCalls, store.queuedErrorCode)
	}
	if queue.retryReason != "SYNC_FAILED" || queue.retryReason == "provider secret employee-secret failed" {
		t.Fatalf("retry reason = %q", queue.retryReason)
	}
	if strings.Contains(logs.String(), "employee-secret") || strings.Contains(logs.String(), "provider secret") {
		t.Fatalf("worker log leaked provider error: %s", logs.String())
	}
}

func TestEmployeeApplyWorkerMarksDeadLetterFailed(t *testing.T) {
	queue := &fakeEmployeeApplyWorkerQueue{deadLettered: true}
	store := &lifecycleEmployeeApplyWorkerStore{
		fakeEmployeeApplyWorkerStore: &fakeEmployeeApplyWorkerStore{
			tenantIDs: map[int]int{7: 11},
			credentials: map[int]WorkEmployeeSyncCredential{
				7: {CorpID: 7, TenantID: 11, CredentialVersion: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret"},
			},
		},
	}
	worker := NewEmployeeApplyWorker(queue, store, &failingEmployeeApplyWorkerClient{err: errors.New("provider failure")}, log.Default())

	worker.handleDelivery(context.Background(), EmployeeApplyDelivery{Raw: "raw-job", Event: EmployeeApplyEvent{BindingID: 7, Source: CompanyEmployeeSyncSource, QueueTicket: "ticket-1"}})

	if store.beginCalls != 1 || store.queuedCalls != 0 || store.failureCalls != 1 {
		t.Fatalf("dead-letter lifecycle begin=%d queued=%d failure=%d", store.beginCalls, store.queuedCalls, store.failureCalls)
	}
}

func TestEmployeeApplyWorkerKeepsSyncingWhenQueueRetryFails(t *testing.T) {
	queue := &fakeEmployeeApplyWorkerQueue{retryErr: errors.New("redis unavailable")}
	store := &lifecycleEmployeeApplyWorkerStore{
		fakeEmployeeApplyWorkerStore: &fakeEmployeeApplyWorkerStore{
			credentials: map[int]WorkEmployeeSyncCredential{
				7: {CorpID: 7, TenantID: 7, CredentialVersion: 1, WXCorpID: "ww-go", EmployeeSecret: "employee-secret"},
			},
		},
	}
	worker := NewEmployeeApplyWorker(queue, store, &failingEmployeeApplyWorkerClient{err: errors.New("provider failure")}, log.Default())

	worker.handleDelivery(context.Background(), EmployeeApplyDelivery{Raw: "raw-job", Event: EmployeeApplyEvent{BindingID: 7, Source: CompanyEmployeeSyncSource, QueueTicket: "ticket-1"}})

	if store.beginCalls != 1 || store.queuedCalls != 0 || store.failureCalls != 0 {
		t.Fatalf("retry failure lifecycle begin=%d queued=%d failure=%d", store.beginCalls, store.queuedCalls, store.failureCalls)
	}
}

func TestEmployeeApplyWorkerRecordsFailedQueueItemExecution(t *testing.T) {
	queue := &fakeEmployeeApplyWorkerQueue{}
	recorder := &fakeWorkerExecutionRecorder{}
	ctx := taskrunner.WithTaskRuntime(context.Background(), "employee-apply", "run-employee-1", recorder)
	worker := NewEmployeeApplyWorker(queue, &fakeEmployeeApplyWorkerStore{credentials: map[int]WorkEmployeeSyncCredential{}}, &fakeEmployeeApplyWorkerClient{}, log.Default())

	worker.handleDelivery(ctx, EmployeeApplyDelivery{Raw: "raw-job", Attempts: 1, Event: EmployeeApplyEvent{BindingID: 7, Source: "test", QueueTicket: "ticket-1"}})

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
	beginCalls          int
	queuedCalls         int
	queuedErrorCode     string
	failureCalls        int
}

type versionFenceEmployeeApplyStore struct {
	credential      WorkEmployeeSyncCredential
	bindingTenantID int
	currentVersion  uint64
	businessWrites  int
	markerWrites    int
}

func (s *versionFenceEmployeeApplyStore) TenantIDByBindingID(context.Context, int) (int, error) {
	if s.bindingTenantID > 0 {
		return s.bindingTenantID, nil
	}
	return s.credential.TenantID, nil
}
func (s *versionFenceEmployeeApplyStore) CompanyEmployeeSyncCredentials(context.Context, int) ([]WorkEmployeeSyncCredential, error) {
	return []WorkEmployeeSyncCredential{s.credential}, nil
}
func (s *versionFenceEmployeeApplyStore) BeginCompanyEmployeeSyncAtVersion(_ context.Context, _ int, version uint64, _ string) error {
	if version != s.currentVersion {
		return fmt.Errorf("company credential version stale")
	}
	return nil
}
func (s *versionFenceEmployeeApplyStore) SyncCompanyEmployeesAtVersion(_ context.Context, _ int, version uint64, _ string, _ []WorkEmployeeSyncDepartment, _ []WorkEmployeeSyncEmployee) (WorkEmployeeSyncResult, error) {
	if version != s.currentVersion {
		return WorkEmployeeSyncResult{}, fmt.Errorf("company credential version stale")
	}
	s.businessWrites++
	return WorkEmployeeSyncResult{}, nil
}
func (s *versionFenceEmployeeApplyStore) MarkCompanyEmployeeSyncQueuedAtVersion(context.Context, int, uint64, string, string) error {
	s.markerWrites++
	return nil
}
func (s *versionFenceEmployeeApplyStore) RecordCompanyEmployeeSyncFailureAtVersion(context.Context, int, uint64, string) error {
	s.markerWrites++
	return nil
}

type rotatingEmployeeApplyClient struct {
	store *versionFenceEmployeeApplyStore
}

func (c *rotatingEmployeeApplyClient) Departments(context.Context, WorkEmployeeSyncCredential) ([]WorkEmployeeSyncDepartment, error) {
	c.store.currentVersion = 2
	return []WorkEmployeeSyncDepartment{{WXDepartmentID: 1, Name: "总部"}}, nil
}
func (*rotatingEmployeeApplyClient) DepartmentUsers(context.Context, WorkEmployeeSyncCredential, int) ([]WorkEmployeeSyncEmployee, error) {
	return []WorkEmployeeSyncEmployee{{WXUserID: "employee-1", Name: "员工"}}, nil
}
func (*rotatingEmployeeApplyClient) FollowUsers(context.Context, WorkEmployeeSyncCredential) ([]string, error) {
	return nil, nil
}

type countingEmployeeApplyClient struct {
	departmentCalls int
}

func (c *countingEmployeeApplyClient) Departments(context.Context, WorkEmployeeSyncCredential) ([]WorkEmployeeSyncDepartment, error) {
	c.departmentCalls++
	return nil, nil
}
func (*countingEmployeeApplyClient) DepartmentUsers(context.Context, WorkEmployeeSyncCredential, int) ([]WorkEmployeeSyncEmployee, error) {
	return nil, nil
}
func (*countingEmployeeApplyClient) FollowUsers(context.Context, WorkEmployeeSyncCredential) ([]string, error) {
	return nil, nil
}

func (s *fakeEmployeeApplyWorkerStore) BeginCompanyEmployeeSync(context.Context, int) error {
	s.beginCalls++
	return nil
}

func (s *fakeEmployeeApplyWorkerStore) BeginCompanyEmployeeSyncAtVersion(context.Context, int, uint64, string) error {
	s.beginCalls++
	return nil
}

func (s *fakeEmployeeApplyWorkerStore) RecordCompanyEmployeeSyncFailure(context.Context, int) error {
	s.failureCalls++
	return nil
}

func (s *fakeEmployeeApplyWorkerStore) RecordCompanyEmployeeSyncFailureAtVersion(context.Context, int, uint64, string) error {
	s.failureCalls++
	return nil
}

func (s *fakeEmployeeApplyWorkerStore) MarkCompanyEmployeeSyncQueued(_ context.Context, _ int, errorCode string) error {
	s.queuedCalls++
	s.queuedErrorCode = errorCode
	return nil
}

func (s *fakeEmployeeApplyWorkerStore) MarkCompanyEmployeeSyncQueuedAtVersion(_ context.Context, _ int, _ uint64, _ string, errorCode string) error {
	s.queuedCalls++
	s.queuedErrorCode = errorCode
	return nil
}

type lifecycleEmployeeApplyWorkerStore struct {
	*fakeEmployeeApplyWorkerStore
	beginCalls   int
	failureCalls int
}

func (s *lifecycleEmployeeApplyWorkerStore) BeginCompanyEmployeeSync(context.Context, int) error {
	s.beginCalls++
	return nil
}

func (s *lifecycleEmployeeApplyWorkerStore) BeginCompanyEmployeeSyncAtVersion(context.Context, int, uint64, string) error {
	s.beginCalls++
	return nil
}

func (s *lifecycleEmployeeApplyWorkerStore) RecordCompanyEmployeeSyncFailure(context.Context, int) error {
	s.failureCalls++
	return nil
}

func (s *lifecycleEmployeeApplyWorkerStore) RecordCompanyEmployeeSyncFailureAtVersion(context.Context, int, uint64, string) error {
	s.failureCalls++
	return nil
}

func (s *lifecycleEmployeeApplyWorkerStore) MarkCompanyEmployeeSyncQueued(_ context.Context, _ int, errorCode string) error {
	s.queuedCalls++
	s.queuedErrorCode = errorCode
	return nil
}

func (s *lifecycleEmployeeApplyWorkerStore) MarkCompanyEmployeeSyncQueuedAtVersion(_ context.Context, _ int, _ uint64, _ string, errorCode string) error {
	s.queuedCalls++
	s.queuedErrorCode = errorCode
	return nil
}

type failingEmployeeApplyWorkerClient struct {
	err error
}

func (c *failingEmployeeApplyWorkerClient) Departments(context.Context, WorkEmployeeSyncCredential) ([]WorkEmployeeSyncDepartment, error) {
	return nil, c.err
}

func (*failingEmployeeApplyWorkerClient) DepartmentUsers(context.Context, WorkEmployeeSyncCredential, int) ([]WorkEmployeeSyncEmployee, error) {
	return nil, nil
}

func (*failingEmployeeApplyWorkerClient) FollowUsers(context.Context, WorkEmployeeSyncCredential) ([]string, error) {
	return nil, nil
}

func (s *fakeEmployeeApplyWorkerStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	return s.tenantIDs[corpID], nil
}

func (s *fakeEmployeeApplyWorkerStore) CompanyEmployeeSyncCredentials(_ context.Context, bindingID int) ([]WorkEmployeeSyncCredential, error) {
	out := make([]WorkEmployeeSyncCredential, 0, 1)
	if credential, ok := s.credentials[bindingID]; ok {
		out = append(out, credential)
	}
	return out, nil
}

func (s *fakeEmployeeApplyWorkerStore) SyncCompanyEmployees(_ context.Context, bindingID int, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee) (WorkEmployeeSyncResult, error) {
	credential := s.credentials[bindingID]
	s.syncedCorpIDs = append(s.syncedCorpIDs, credential.CorpID)
	s.syncedDepartments = append([]WorkEmployeeSyncDepartment{}, departments...)
	s.syncedEmployees = append([]WorkEmployeeSyncEmployee{}, employees...)
	s.followUserIDs = nil
	s.defaultPasswordHash = ""
	return WorkEmployeeSyncResult{}, nil
}

func (s *fakeEmployeeApplyWorkerStore) SyncCompanyEmployeesAtVersion(ctx context.Context, bindingID int, _ uint64, _ string, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee) (WorkEmployeeSyncResult, error) {
	return s.SyncCompanyEmployees(ctx, bindingID, departments, employees)
}

func (s *fakeEmployeeApplyWorkerStore) TenantIDByBindingID(_ context.Context, bindingID int) (int, error) {
	if tenantID, ok := s.tenantIDs[bindingID]; ok {
		return tenantID, nil
	}
	return bindingID, nil
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
	retryErr         error
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
	if q.retryErr != nil {
		return false, q.retryErr
	}
	return q.deadLettered, nil
}

func (q *fakeEmployeeApplyWorkerQueue) RecoverEmployeeApplyProcessing(_ context.Context, _ time.Duration, _ int) (int, error) {
	return 0, nil
}
