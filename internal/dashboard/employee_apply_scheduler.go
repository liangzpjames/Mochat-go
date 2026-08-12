package dashboard

import (
	"context"
	"errors"
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
	EnqueueEmployeeApply(ctx context.Context, event EmployeeApplyEvent) error
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

func (s *CompanyEmployeeSyncScheduler) EnqueueEmployeeSync(ctx context.Context, tenantID int) (string, error) {
	if s == nil || s.queue == nil {
		return "", errors.New("employee sync scheduler is not configured")
	}
	event, err := NewCompanyEmployeeApplyEvent(tenantID)
	if err != nil {
		return "", err
	}
	if err := s.queue.EnqueueEmployeeApply(ctx, event); err != nil {
		return "", err
	}
	return CompanyEmployeeSyncCursor, nil
}
