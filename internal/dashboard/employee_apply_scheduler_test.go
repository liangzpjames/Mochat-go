package dashboard

import (
	"context"
	"reflect"
	"testing"
)

type captureCompanyEmployeeApplyQueue struct {
	calls int
	event EmployeeApplyEvent
}

func (q *captureCompanyEmployeeApplyQueue) EnqueueEmployeeApply(_ context.Context, event EmployeeApplyEvent) error {
	q.calls++
	q.event = event
	return nil
}

func TestCompanyEmployeeSyncSchedulerUsesTenantBindingEvent(t *testing.T) {
	queue := &captureCompanyEmployeeApplyQueue{}
	scheduler := NewCompanyEmployeeSyncScheduler(queue)

	cursor, err := scheduler.EnqueueEmployeeSync(context.Background(), 202)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != CompanyEmployeeSyncCursor || queue.calls != 1 || queue.event.BindingID != TenantBindingID(202) || queue.event.Source != CompanyEmployeeSyncSource {
		t.Fatalf("cursor=%q calls=%d event=%+v", cursor, queue.calls, queue.event)
	}
	if _, ok := reflect.TypeOf(queue.event).FieldByName("CorpID"); ok {
		t.Fatal("employee apply event must not carry corp id")
	}
}

func TestCompanyEmployeeSyncSchedulerRejectsInvalidTenantBinding(t *testing.T) {
	scheduler := NewCompanyEmployeeSyncScheduler(&captureCompanyEmployeeApplyQueue{})
	if _, err := scheduler.EnqueueEmployeeSync(context.Background(), 0); err == nil {
		t.Fatal("invalid tenant binding unexpectedly enqueued")
	}
}
