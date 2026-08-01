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
	filter        ports.ListPublicPoolFilter
	page          ports.AssignmentPage
	contactFilter ports.ListContactsFilter
	contacts      ports.ContactPage
	detail        ports.ContactDetail
	err           error
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

func (r *fakeAssignmentRepository) ReleaseToPublicPool(_ context.Context, _, _ int64, _ string, _ int64, _ string) (domain.CustomerAssignment, error) {
	return domain.CustomerAssignment{}, r.err
}

func (r *fakeAssignmentRepository) ClaimFromPublicPool(_ context.Context, _, _ int64, _ string, _, _ int64, _ string) (domain.CustomerAssignment, error) {
	return domain.CustomerAssignment{}, r.err
}
