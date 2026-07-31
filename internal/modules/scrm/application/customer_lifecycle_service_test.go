package application

import (
	"context"
	"errors"
	"testing"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestCustomerLifecycleServiceScopesPublicPoolByTenantAndCorp(t *testing.T) {
	repository := &fakeAssignmentRepository{page: ports.AssignmentPage{Items: []domain.CustomerAssignment{{ContactID: "contact-1"}}}}
	service, err := NewCustomerLifecycleService(repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListPublicPool(context.Background(), ListPublicPoolQuery{TenantID: 7, CorpID: 9, PageSize: 3}); err != nil {
		t.Fatal(err)
	}
	if repository.filter.TenantID != 7 || repository.filter.CorpID != 9 || repository.filter.Limit != 3 {
		t.Fatalf("filter = %#v", repository.filter)
	}
}

func TestCustomerLifecycleServiceRejectsStaleAssignmentVersion(t *testing.T) {
	repository := &fakeAssignmentRepository{err: ports.ErrAssignmentConflict}
	service, err := NewCustomerLifecycleService(repository)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ClaimFromPublicPool(context.Background(), 7, 9, "contact-1", 42, 2, "claim-1")
	if !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("error = %v, want assignment conflict", err)
	}
}

func TestCustomerLifecycleServiceRequiresIdempotencyKey(t *testing.T) {
	service, err := NewCustomerLifecycleService(&fakeAssignmentRepository{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ReleaseToPublicPool(context.Background(), 7, 9, "contact-1", 1, "")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("error = %v, want invalid argument", err)
	}
}

type fakeAssignmentRepository struct {
	filter ports.ListPublicPoolFilter
	page   ports.AssignmentPage
	err    error
}

func (r *fakeAssignmentRepository) ListPublicPool(_ context.Context, filter ports.ListPublicPoolFilter) (ports.AssignmentPage, error) {
	r.filter = filter
	return r.page, r.err
}

func (r *fakeAssignmentRepository) UpdateAssignment(_ context.Context, _ ports.UpdateAssignmentCommand) (domain.CustomerAssignment, error) {
	return domain.CustomerAssignment{}, r.err
}

func (r *fakeAssignmentRepository) ReleaseToPublicPool(_ context.Context, _, _ int64, _ string, _ int64, _ string) (domain.CustomerAssignment, error) {
	return domain.CustomerAssignment{}, r.err
}

func (r *fakeAssignmentRepository) ClaimFromPublicPool(_ context.Context, _, _ int64, _ string, _, _ int64, _ string) (domain.CustomerAssignment, error) {
	return domain.CustomerAssignment{}, r.err
}
