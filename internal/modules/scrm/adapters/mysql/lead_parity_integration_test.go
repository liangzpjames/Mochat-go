//go:build integration

package mysql

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/application"
	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestLeadParityMariaDBCorpIsolationCombinedFilterAndDuplicate(t *testing.T) {
	repository, _, namespace := integrationRepository(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	leadA := parityTestLead(t, namespace.id("a"), namespace.tenantID, 101, namespace.key("a"), "Ada North", "13800000001", domain.LeadSourceManual, base)
	leadB := parityTestLead(t, namespace.id("b"), namespace.tenantID, 101, namespace.key("b"), "Bob South", "13800000002", domain.LeadSourceImport, base.Add(time.Hour))
	leadOtherCorp := parityTestLead(t, namespace.id("other-corp"), namespace.tenantID, 102, namespace.key("other"), "Ada Hidden", "13800000003", domain.LeadSourceManual, base)
	for _, lead := range []domain.Lead{leadA, leadB, leadOtherCorp} {
		mustCreateLead(t, repository, lead)
	}
	assigned, err := repository.Assign(ctx, ports.AssignLeadCommand{TenantID: namespace.tenantID, CorpID: 101, LeadID: leadA.ID, OwnerID: 77, Version: 1, UpdatedAt: base.Add(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	page, err := repository.List(ctx, ports.ListLeadsFilter{TenantID: namespace.tenantID, CorpID: 101, Keyword: "Ada", Statuses: []domain.LeadStatus{domain.LeadStatusQualified}, Sources: []domain.LeadSource{domain.LeadSourceManual}, OwnerIDs: []int64{77}, CreatedFrom: base.Add(-time.Minute), CreatedTo: base.Add(time.Minute), Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != assigned.ID {
		t.Fatalf("combined page=%#v", page.Items)
	}
	duplicate := parityTestLead(t, namespace.id("duplicate"), namespace.tenantID, 101, namespace.key("duplicate"), "Duplicate", "13800000001", domain.LeadSourceWeCom, base)
	if _, _, err := repository.CreateOrGet(ctx, duplicate); !errors.Is(err, ports.ErrDuplicateLead) {
		t.Fatalf("duplicate error=%v", err)
	}
}

func TestLeadParityMariaDBConcurrentVersionAndPartialFailure(t *testing.T) {
	repository, _, namespace := integrationRepository(t)
	ctx := context.Background()
	now := time.Now().UTC()
	lead := parityTestLead(t, namespace.id("race"), namespace.tenantID, 201, namespace.key("race"), "Race", "", domain.LeadSourceManual, now)
	mustCreateLead(t, repository, lead)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var successes atomic.Int32
	var wait sync.WaitGroup
	for _, owner := range []int64{11, 12} {
		wait.Add(1)
		go func(ownerID int64) {
			defer wait.Done()
			<-start
			_, err := repository.Assign(ctx, ports.AssignLeadCommand{TenantID: namespace.tenantID, CorpID: 201, LeadID: lead.ID, OwnerID: ownerID, Version: 1, UpdatedAt: now.Add(time.Second)})
			if err == nil {
				successes.Add(1)
			}
			errs <- err
		}(owner)
	}
	close(start)
	wait.Wait()
	close(errs)
	conflicts := 0
	for err := range errs {
		if errors.Is(err, ports.ErrLeadConflict) {
			conflicts++
		} else if err != nil {
			t.Errorf("assign error=%v", err)
		}
	}
	if successes.Load() != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes.Load(), conflicts)
	}
	service, err := application.NewService(repository, parityClock{now: now.Add(2 * time.Second)}, parityIDGenerator{})
	if err != nil {
		t.Fatal(err)
	}
	results, err := service.AssignLeads(ctx, application.AssignLeadsCommand{TenantID: namespace.tenantID, CorpID: 201, OwnerID: 13, Targets: []application.LeadMutationTarget{{ID: lead.ID, Version: 2}, {ID: namespace.id("missing"), Version: 1}, {ID: lead.ID, Version: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].Status != "succeeded" || results[1].ErrorCode != "NOT_FOUND" || results[2].ErrorCode != "CONFLICT" {
		t.Fatalf("partial results=%#v", results)
	}
}

func TestLeadParityMariaDBConversionPersistsContactAndAssignment(t *testing.T) {
	repository, db, namespace := integrationRepository(t)
	ctx := context.Background()
	now := time.Now().UTC()
	lead := parityTestLead(t, namespace.id("convert"), namespace.tenantID, 301, namespace.key("convert"), "Convert", "13900000001", domain.LeadSourceManual, now)
	mustCreateLead(t, repository, lead)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM mochat_go_scrm_assignments WHERE tenant_id=? AND corp_id=?", namespace.tenantID, 301)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, 301)
	})
	qualified, err := repository.Assign(ctx, ports.AssignLeadCommand{TenantID: namespace.tenantID, CorpID: 301, LeadID: lead.ID, OwnerID: 88, Version: 1, UpdatedAt: now.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	converted, err := repository.Transition(ctx, ports.TransitionLeadCommand{TenantID: namespace.tenantID, CorpID: 301, LeadID: lead.ID, ToStatus: domain.LeadStatusConverted, Version: qualified.Version, UpdatedAt: now.Add(2 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if converted.ConvertedContactID != lead.ID {
		t.Fatalf("converted=%#v", converted)
	}
	var contacts, assignments int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=? AND id=?", namespace.tenantID, 301, lead.ID).Scan(&contacts); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_scrm_assignments WHERE tenant_id=? AND corp_id=? AND contact_id=? AND owner_id=88", namespace.tenantID, 301, lead.ID).Scan(&assignments); err != nil {
		t.Fatal(err)
	}
	if contacts != 1 || assignments != 1 {
		t.Fatalf("contacts=%d assignments=%d", contacts, assignments)
	}
}

func parityTestLead(t *testing.T, id string, tenantID, corpID int64, key, name, phone string, source domain.LeadSource, createdAt time.Time) domain.Lead {
	t.Helper()
	lead, err := domain.NewCorpLead(id, tenantID, corpID, key, name, phone, source, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	return lead
}

type parityClock struct{ now time.Time }

func (c parityClock) Now() time.Time { return c.now }

type parityIDGenerator struct{}

func (parityIDGenerator) NewID() (string, error) { return "", fmt.Errorf("unused") }
