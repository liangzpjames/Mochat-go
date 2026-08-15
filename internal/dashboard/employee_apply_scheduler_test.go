package dashboard

import (
	"context"
	"reflect"
	"testing"
	"time"

	"jiyi/mochat-go/internal/companyprofile"
)

type captureCompanyEmployeeApplyQueue struct {
	calls int
	event EmployeeApplyEvent
}

func (q *captureCompanyEmployeeApplyQueue) EnqueueEmployeeApplyWithReceipt(_ context.Context, event EmployeeApplyEvent) (companyprofile.EmployeeSyncEnqueueReceipt, error) {
	q.calls++
	q.event = event
	return companyprofile.EmployeeSyncEnqueueReceipt{Cursor: CompanyEmployeeSyncCursor, Ticket: "ticket-1", RequestedAt: time.Unix(10, 0).UTC()}, nil
}

func TestCompanyEmployeeSyncSchedulerUsesTenantBindingEvent(t *testing.T) {
	queue := &captureCompanyEmployeeApplyQueue{}
	scheduler := NewCompanyEmployeeSyncScheduler(queue)

	receipt, err := scheduler.EnqueueEmployeeSync(context.Background(), 202)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Cursor != CompanyEmployeeSyncCursor || receipt.Ticket != "ticket-1" || queue.calls != 1 || queue.event.BindingID != TenantBindingID(202) || queue.event.Source != CompanyEmployeeSyncSource {
		t.Fatalf("receipt=%+v calls=%d event=%+v", receipt, queue.calls, queue.event)
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
