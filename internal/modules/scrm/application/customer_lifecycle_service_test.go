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
	if _, err := service.ListPublicPool(context.Background(), ListPublicPoolQuery{TenantID: 7, CorpID: 9, Keyword: " Ada ", Sources: []string{"wecom"}, BusinessTypes: []string{"retail"}, TagIDs: []string{"tag-1"}, Regions: []string{"Shanghai"}, Reasons: []string{"expired"}, PreviousOwnerIDs: []int64{42}, PageSize: 3}); err != nil {
		t.Fatal(err)
	}
	if repository.filter.TenantID != 7 || repository.filter.CorpID != 9 || repository.filter.Keyword != "Ada" || repository.filter.Limit != 3 || len(repository.filter.Sources) != 1 || len(repository.filter.BusinessTypes) != 1 || len(repository.filter.TagIDs) != 1 || len(repository.filter.Regions) != 1 || len(repository.filter.Reasons) != 1 || len(repository.filter.PreviousOwnerIDs) != 1 {
		t.Fatalf("filter = %#v", repository.filter)
	}
}

func TestCustomerLifecycleServicePassesCompleteClaimCommandAndRejectsStaleVersion(t *testing.T) {
	repository := &fakeAssignmentRepository{err: ports.ErrAssignmentConflict}
	service, err := NewCustomerLifecycleService(repository)
	if err != nil {
		t.Fatal(err)
	}
	command := ports.ClaimPublicPoolCommand{TenantID: 7, CorpID: 9, ContactID: "contact-1", UserID: 42, Version: 2, IdempotencyKey: "claim-1"}
	_, err = service.ClaimFromPublicPool(context.Background(), command)
	if !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("error = %v, want assignment conflict", err)
	}
	if repository.claimCommand != command {
		t.Fatalf("claim command = %#v, want %#v", repository.claimCommand, command)
	}
}

func TestCustomerLifecycleServiceRequiresAuditedPoolReasonAndIdempotencyKey(t *testing.T) {
	service, err := NewCustomerLifecycleService(&fakeAssignmentRepository{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.MoveToPublicPool(context.Background(), ports.MoveToPublicPoolCommand{TenantID: 7, CorpID: 9, ContactID: "contact-1", ActorID: 42, Version: 1, Action: domain.PublicPoolActionReturn})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("error = %v, want invalid argument", err)
	}
}

func TestCustomerLifecycleServiceBatchClaimReturnsPerItemPartialFailure(t *testing.T) {
	repository := &fakeAssignmentRepository{claimByContact: map[string]error{"stale": ports.ErrAssignmentConflict, "hidden": ports.ErrAssignmentForbidden}}
	service, err := NewCustomerLifecycleService(repository)
	if err != nil {
		t.Fatal(err)
	}
	results, err := service.BatchClaimFromPublicPool(context.Background(), BatchClaimPublicPoolCommand{TenantID: 7, CorpID: 9, UserID: 42, Targets: []PublicPoolClaimTarget{{ContactID: "ok", Version: 1, IdempotencyKey: "batch-ok"}, {ContactID: "stale", Version: 2, IdempotencyKey: "batch-stale"}, {ContactID: "hidden", Version: 3, IdempotencyKey: "batch-hidden"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].Status != "succeeded" || results[1].ErrorCode != "CONFLICT" || results[2].ErrorCode != "FORBIDDEN" {
		t.Fatalf("results = %#v", results)
	}
}

func TestCustomerLifecycleServiceListsContactsWithCombinedScope(t *testing.T) {
	repository := &fakeAssignmentRepository{contacts: ports.ContactPage{Items: []ports.ContactSummary{{ID: "contact-1", Name: "Ada"}}}}
	service, err := NewCustomerLifecycleService(repository)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.ListContacts(context.Background(), ListContactsQuery{
		TenantID: 7, CorpID: 9, Keyword: " Ada ", OwnerIDs: []int64{11}, TagIDs: []string{"tag-1"},
		Statuses: []string{domain.AssignmentOwned}, Cursor: "20", PageSize: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || repository.contactFilter.TenantID != 7 || repository.contactFilter.CorpID != 9 || repository.contactFilter.Keyword != "Ada" || repository.contactFilter.Limit != 30 {
		t.Fatalf("page=%#v filter=%#v", page, repository.contactFilter)
	}
}

func TestCustomerLifecycleServiceReturnsAggregatedContactDetail(t *testing.T) {
	want := ports.ContactDetail{
		ContactSummary: ports.ContactSummary{ID: "contact-1", Name: "Ada", Version: 3},
		Assignment:     domain.CustomerAssignment{ContactID: "contact-1", Version: 4},
		Tags:           []ports.ContactTagSummary{{ID: "tag-1", Name: "VIP"}},
		WeComFriends:   []ports.WeComFriendSummary{{ExternalUserID: "wx-1", EmployeeID: 8}},
		Opportunities:  []ports.ContactOpportunitySummary{{ID: "opp-1", Stage: "proposal"}},
		FollowUps:      []ports.ContactFollowUpSummary{{ID: "follow-1", Content: "called"}},
	}
	repository := &fakeAssignmentRepository{detail: want}
	service, err := NewCustomerLifecycleService(repository)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.GetContact(context.Background(), 7, 9, "contact-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || len(got.Tags) != 1 || len(got.WeComFriends) != 1 || len(got.Opportunities) != 1 || len(got.FollowUps) != 1 {
		t.Fatalf("detail=%#v", got)
	}
}

type fakeAssignmentRepository struct {
	filter         ports.ListPublicPoolFilter
	page           ports.AssignmentPage
	contactFilter  ports.ListContactsFilter
	contacts       ports.ContactPage
	detail         ports.ContactDetail
	err            error
	claimCommand   ports.ClaimPublicPoolCommand
	moveCommand    ports.MoveToPublicPoolCommand
	claimByContact map[string]error
}

func (r *fakeAssignmentRepository) ListContacts(_ context.Context, filter ports.ListContactsFilter) (ports.ContactPage, error) {
	r.contactFilter = filter
	return r.contacts, r.err
}

func (r *fakeAssignmentRepository) GetContact(_ context.Context, _, _ int64, _ string) (ports.ContactDetail, error) {
	return r.detail, r.err
}

func (r *fakeAssignmentRepository) ListPublicPool(_ context.Context, filter ports.ListPublicPoolFilter) (ports.AssignmentPage, error) {
	r.filter = filter
	return r.page, r.err
}

func (r *fakeAssignmentRepository) UpdateAssignment(_ context.Context, _ ports.UpdateAssignmentCommand) (domain.CustomerAssignment, error) {
	return domain.CustomerAssignment{}, r.err
}

func (r *fakeAssignmentRepository) MoveToPublicPool(_ context.Context, command ports.MoveToPublicPoolCommand) (domain.CustomerAssignment, error) {
	r.moveCommand = command
	return domain.CustomerAssignment{}, r.err
}

func (r *fakeAssignmentRepository) ClaimFromPublicPool(_ context.Context, command ports.ClaimPublicPoolCommand) (domain.CustomerAssignment, error) {
	r.claimCommand = command
	if err := r.claimByContact[command.ContactID]; err != nil {
		return domain.CustomerAssignment{}, err
	}
	return domain.CustomerAssignment{ContactID: command.ContactID, OwnerID: &command.UserID, Status: domain.AssignmentOwned, Version: command.Version + 1}, r.err
}
