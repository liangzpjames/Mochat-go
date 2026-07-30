package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewLeadRejectsInvalidTenantID(t *testing.T) {
	for _, tenantID := range []int64{0, -1} {
		t.Run("tenant", func(t *testing.T) {
			_, err := NewLead("lead-1", tenantID, "request-1", "Ada", LeadSourceManual, time.Now())
			if !errors.Is(err, ErrInvalidTenantID) {
				t.Fatalf("error = %v, want ErrInvalidTenantID", err)
			}
		})
	}
}

func TestNewLeadRejectsInvalidBusinessKey(t *testing.T) {
	for _, businessKey := range []string{"", " \t "} {
		t.Run("business key", func(t *testing.T) {
			_, err := NewLead("lead-1", 1, businessKey, "Ada", LeadSourceManual, time.Now())
			if !errors.Is(err, ErrInvalidBusinessKey) {
				t.Fatalf("error = %v, want ErrInvalidBusinessKey", err)
			}
		})
	}

	_, err := NewLead("lead-1", 1, strings.Repeat("a", 129), "Ada", LeadSourceManual, time.Now())
	if !errors.Is(err, ErrInvalidBusinessKey) {
		t.Fatalf("error = %v, want ErrInvalidBusinessKey", err)
	}
}

func TestNewLeadRejectsInvalidName(t *testing.T) {
	for _, name := range []string{"", " \t ", strings.Repeat("a", 201)} {
		t.Run("name", func(t *testing.T) {
			_, err := NewLead("lead-1", 1, "request-1", name, LeadSourceManual, time.Now())
			if !errors.Is(err, ErrInvalidLeadName) {
				t.Fatalf("error = %v, want ErrInvalidLeadName", err)
			}
		})
	}
}

func TestNewLeadRejectsUnsupportedSource(t *testing.T) {
	_, err := NewLead("lead-1", 1, "request-1", "Ada", LeadSource("api"), time.Now())
	if !errors.Is(err, ErrUnsupportedLeadSource) {
		t.Fatalf("error = %v, want ErrUnsupportedLeadSource", err)
	}
}

func TestNewLeadInitializesStableFields(t *testing.T) {
	now := time.Date(2026, time.July, 30, 8, 9, 10, 0, time.FixedZone("CST", 8*60*60))

	lead, err := NewLead("lead-1", 7, "request-1", "Ada", LeadSourceWeCom, now)
	if err != nil {
		t.Fatal(err)
	}

	if lead.ID != "lead-1" || lead.TenantID != 7 || lead.BusinessKey != "request-1" {
		t.Fatalf("lead identity = %#v", lead)
	}
	if lead.Name.String() != "Ada" || lead.Source != LeadSourceWeCom {
		t.Fatalf("lead details = %#v", lead)
	}
	if lead.Status != LeadStatusNew || lead.Version != 1 {
		t.Fatalf("status/version = %q/%d", lead.Status, lead.Version)
	}
	wantTime := now.UTC()
	if !lead.CreatedAt.Equal(wantTime) || !lead.UpdatedAt.Equal(wantTime) {
		t.Fatalf("timestamps = %s/%s, want %s", lead.CreatedAt, lead.UpdatedAt, wantTime)
	}
	if lead.CreatedAt.Location() != time.UTC || lead.UpdatedAt.Location() != time.UTC {
		t.Fatalf("timestamps must use UTC locations: %s/%s", lead.CreatedAt.Location(), lead.UpdatedAt.Location())
	}
}
