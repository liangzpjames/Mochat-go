package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestCreateLeadUsesServerInputsAndReturnsExistingOnRetry(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	existing, err := domain.NewLead("persisted-1", 9, "request-1", "Existing", domain.LeadSourceImport, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	repository := &fakeLeadRepository{createResult: existing}
	service, err := NewService(repository, fixedClock{now: now}, fixedIDGenerator{id: "generated-1"})
	if err != nil {
		t.Fatal(err)
	}

	view, err := service.CreateLead(context.Background(), CreateLeadCommand{
		TenantID:    9,
		BusinessKey: "request-1",
		Name:        "Ada",
		Source:      domain.LeadSourceManual,
	})
	if err != nil {
		t.Fatal(err)
	}

	if repository.createdLead.ID != "generated-1" || !repository.createdLead.CreatedAt.Equal(now) || !repository.createdLead.UpdatedAt.Equal(now) {
		t.Fatalf("server-generated lead = %#v", repository.createdLead)
	}
	if view.ID != existing.ID || view.Name != existing.Name.String() || view.Source != existing.Source {
		t.Fatalf("view = %#v, want existing lead %#v", view, existing)
	}
}

func TestCreateLeadRejectsMissingTenant(t *testing.T) {
	service, err := NewService(&fakeLeadRepository{}, fixedClock{}, fixedIDGenerator{id: "generated-1"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.CreateLead(context.Background(), CreateLeadCommand{
		BusinessKey: "request-1",
		Name:        "Ada",
		Source:      domain.LeadSourceManual,
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("error = %v, want ErrInvalidArgument", err)
	}
}

func TestListLeadsAlwaysScopesRepositoryByTenant(t *testing.T) {
	repository := &fakeLeadRepository{}
	service, err := NewService(repository, fixedClock{}, fixedIDGenerator{id: "generated-1"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.ListLeads(context.Background(), ListLeadsQuery{TenantID: 42, Cursor: "cursor-1", PageSize: 3})
	if err != nil {
		t.Fatal(err)
	}
	if repository.listFilter.TenantID != 42 {
		t.Fatalf("repository tenant ID = %d, want 42", repository.listFilter.TenantID)
	}
	if repository.listFilter.TenantID == 0 {
		t.Fatal("repository received an unscoped list query")
	}
}

func TestListLeadsAppliesDefaultAndMaximumPageSize(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pageSize int
		want     int
	}{
		{name: "default", pageSize: 0, want: 20},
		{name: "maximum", pageSize: 101, want: 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := &fakeLeadRepository{}
			service, err := NewService(repository, fixedClock{}, fixedIDGenerator{id: "generated-1"})
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.ListLeads(context.Background(), ListLeadsQuery{TenantID: 42, PageSize: tc.pageSize})
			if err != nil {
				t.Fatal(err)
			}
			if repository.listFilter.Limit != tc.want {
				t.Fatalf("repository limit = %d, want %d", repository.listFilter.Limit, tc.want)
			}
		})
	}
}

func TestListLeadsRejectsNegativeCursorAndPageSize(t *testing.T) {
	for _, query := range []ListLeadsQuery{
		{TenantID: 42, Cursor: "-1", PageSize: 1},
		{TenantID: 42, PageSize: -1},
	} {
		service, err := NewService(&fakeLeadRepository{}, fixedClock{}, fixedIDGenerator{id: "generated-1"})
		if err != nil {
			t.Fatal(err)
		}

		_, err = service.ListLeads(context.Background(), query)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("error = %v, want ErrInvalidArgument", err)
		}
	}
}

func TestNewServiceRejectsNilDependencies(t *testing.T) {
	for _, tc := range []struct {
		name        string
		repository  ports.LeadRepository
		clock       ports.Clock
		idGenerator ports.IDGenerator
	}{
		{name: "repository", clock: fixedClock{}, idGenerator: fixedIDGenerator{}},
		{name: "clock", repository: &fakeLeadRepository{}, idGenerator: fixedIDGenerator{}},
		{name: "id generator", repository: &fakeLeadRepository{}, clock: fixedClock{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewService(tc.repository, tc.clock, tc.idGenerator)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("error = %v, want ErrInvalidArgument", err)
			}
		})
	}
}

type fakeLeadRepository struct {
	createdLead  domain.Lead
	createResult domain.Lead
	createErr    error
	listFilter   ports.ListLeadsFilter
	listPage     ports.LeadPage
	listErr      error
}

func (r *fakeLeadRepository) CreateOrGet(_ context.Context, lead domain.Lead) (domain.Lead, bool, error) {
	r.createdLead = lead
	if r.createErr != nil {
		return domain.Lead{}, false, r.createErr
	}
	if r.createResult.ID != "" {
		return r.createResult, false, nil
	}
	return lead, true, nil
}

func (r *fakeLeadRepository) List(_ context.Context, filter ports.ListLeadsFilter) (ports.LeadPage, error) {
	r.listFilter = filter
	return r.listPage, r.listErr
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type fixedIDGenerator struct {
	id  string
	err error
}

func (g fixedIDGenerator) NewID() (string, error) {
	return g.id, g.err
}
