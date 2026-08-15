package dashboard

import (
	"context"
	"errors"

	"jiyi/mochat-go/internal/companyprofile"
)

const (
	CompanyEmployeeSyncSource = "dashboard.company.employee-sync"
	CompanyEmployeeSyncCursor = "company-sync"
)

// TenantBindingID is the tenant_id reference of the unique tenant-corp
// binding. It is not a corp_id and must only be constructed from the
// server-resolved tenant principal.
type TenantBindingID int

type EmployeeApplyEnqueuer interface {
	EnqueueEmployeeApplyWithReceipt(ctx context.Context, event EmployeeApplyEvent) (companyprofile.EmployeeSyncEnqueueReceipt, error)
}

func NewCompanyEmployeeApplyEvent(tenantID int) (EmployeeApplyEvent, error) {
	if tenantID <= 0 {
		return EmployeeApplyEvent{}, errors.New("missing tenant binding id")
	}
	return EmployeeApplyEvent{
		BindingID: TenantBindingID(tenantID),
		Source:    CompanyEmployeeSyncSource,
	}, nil
}

// CompanyEmployeeSyncScheduler is the narrow composition boundary used by
// companyprofile. The queue adapter owns the concrete Redis contract.
type CompanyEmployeeSyncScheduler struct {
	queue EmployeeApplyEnqueuer
}

func NewCompanyEmployeeSyncScheduler(queue EmployeeApplyEnqueuer) *CompanyEmployeeSyncScheduler {
	return &CompanyEmployeeSyncScheduler{queue: queue}
}

func (s *CompanyEmployeeSyncScheduler) EnqueueEmployeeSync(ctx context.Context, tenantID int) (companyprofile.EmployeeSyncEnqueueReceipt, error) {
	if s == nil || s.queue == nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, errors.New("employee sync scheduler is not configured")
	}
	event, err := NewCompanyEmployeeApplyEvent(tenantID)
	if err != nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, err
	}
	receipt, err := s.queue.EnqueueEmployeeApplyWithReceipt(ctx, event)
	if err != nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, err
	}
	if receipt.Cursor == "" {
		receipt.Cursor = CompanyEmployeeSyncCursor
	}
	return receipt, nil
}
