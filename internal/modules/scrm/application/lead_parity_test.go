package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestListLeadsForwardsCombinedCorpScopedFilters(t *testing.T) {
	repository := &parityLeadRepository{}
	service, err := NewService(repository, fixedClock{}, fixedIDGenerator{id: "lead-1"})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	_, err = service.ListLeads(context.Background(), ListLeadsQuery{
		TenantID: 9, CorpID: 7, Keyword: " Ada ", Statuses: []domain.LeadStatus{domain.LeadStatusNew, domain.LeadStatusQualified},
		Sources: []domain.LeadSource{domain.LeadSourceManual}, OwnerIDs: []int64{12, 13}, CreatedFrom: from, CreatedTo: to, PageSize: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	filter := repository.listFilter
	if filter.TenantID != 9 || filter.CorpID != 7 || filter.Keyword != "Ada" || filter.Limit != 30 {
		t.Fatalf("filter = %#v", filter)
	}
	if len(filter.Statuses) != 2 || len(filter.Sources) != 1 || len(filter.OwnerIDs) != 2 || !filter.CreatedFrom.Equal(from) || !filter.CreatedTo.Equal(to) {
		t.Fatalf("combined filter = %#v", filter)
	}
}

func TestCreateLeadRequiresCorpAndMapsDuplicatePhone(t *testing.T) {
	repository := &parityLeadRepository{createErr: ports.ErrDuplicateLead}
	service, err := NewService(repository, fixedClock{now: time.Now()}, fixedIDGenerator{id: "lead-1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateLead(context.Background(), CreateLeadCommand{TenantID: 9, CorpID: 7, BusinessKey: "request-1", Name: "Ada", Phone: "13800000000", Source: domain.LeadSourceManual})
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("error = %v, want ErrDuplicate", err)
	}
}

func TestAssignLeadsReturnsOneResultPerTargetAndAllowsPartialFailure(t *testing.T) {
	repository := &parityLeadRepository{assignErrors: map[string]error{"missing": ports.ErrLeadNotFound, "stale": ports.ErrLeadConflict, "foreign-owner": ports.ErrLeadOwnerOutOfScope}}
	service, err := NewService(repository, fixedClock{now: time.Now()}, fixedIDGenerator{id: "lead-1"})
	if err != nil {
		t.Fatal(err)
	}

	results, err := service.AssignLeads(context.Background(), AssignLeadsCommand{TenantID: 9, CorpID: 7, OwnerID: 12, Targets: []LeadMutationTarget{{ID: "ok", Version: 1}, {ID: "missing", Version: 1}, {ID: "stale", Version: 1}, {ID: "foreign-owner", Version: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].ID != "ok" || results[0].Status != "succeeded" || results[0].ErrorCode != "" {
		t.Fatalf("success = %#v", results[0])
	}
	if results[1].ID != "missing" || results[1].Status != "failed" || results[1].ErrorCode != "NOT_FOUND" {
		t.Fatalf("missing = %#v", results[1])
	}
	if results[2].ID != "stale" || results[2].Status != "failed" || results[2].ErrorCode != "CONFLICT" {
		t.Fatalf("stale = %#v", results[2])
	}
	if results[3].ID != "foreign-owner" || results[3].Status != "failed" || results[3].ErrorCode != "OWNER_OUT_OF_SCOPE" {
		t.Fatalf("foreign owner = %#v", results[3])
	}
}

func TestTransitionLeadMapsVersionConflictAndRejectsInvalidInput(t *testing.T) {
	repository := &parityLeadRepository{transitionErr: ports.ErrLeadConflict}
	service, err := NewService(repository, fixedClock{now: time.Now()}, fixedIDGenerator{id: "lead-1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.TransitionLead(context.Background(), TransitionLeadCommand{TenantID: 9, CorpID: 7, LeadID: "lead-1", ToStatus: domain.LeadStatusQualified, Version: 2})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("error = %v, want ErrConflict", err)
	}
	_, err = service.TransitionLead(context.Background(), TransitionLeadCommand{TenantID: 9, CorpID: 7, LeadID: "lead-1", ToStatus: domain.LeadStatusNew, Version: 2})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("error = %v, want ErrInvalidArgument", err)
	}
}

type parityLeadRepository struct {
	listFilter    ports.ListLeadsFilter
	createErr     error
	assignErrors  map[string]error
	transitionErr error
}

func (r *parityLeadRepository) CreateOrGet(_ context.Context, lead domain.Lead) (domain.Lead, bool, error) {
	return lead, r.createErr == nil, r.createErr
}
func (r *parityLeadRepository) List(_ context.Context, filter ports.ListLeadsFilter) (ports.LeadPage, error) {
	r.listFilter = filter
	return ports.LeadPage{}, nil
}
func (r *parityLeadRepository) FindDuplicates(context.Context, ports.DuplicateLeadFilter) ([]domain.Lead, error) {
	return nil, nil
}
func (r *parityLeadRepository) Assign(_ context.Context, command ports.AssignLeadCommand) (domain.Lead, error) {
	if err := r.assignErrors[command.LeadID]; err != nil {
		return domain.Lead{}, err
	}
	lead, _ := domain.NewCorpLead(command.LeadID, command.TenantID, command.CorpID, "key-"+command.LeadID, command.LeadID, "", domain.LeadSourceManual, command.UpdatedAt)
	lead.Status, lead.OwnerID, lead.Version = domain.LeadStatusQualified, &command.OwnerID, command.Version+1
	return lead, nil
}
func (r *parityLeadRepository) Transition(context.Context, ports.TransitionLeadCommand) (domain.Lead, error) {
	return domain.Lead{}, r.transitionErr
}
