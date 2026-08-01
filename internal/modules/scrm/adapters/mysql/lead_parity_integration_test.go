//go:build integration

package mysql

import (
	"context"
	"database/sql"
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
	repository, db, namespace := integrationRepository(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("corp-a"))
	otherCorpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("corp-b"))
	ownerID := insertParityEmployee(t, db, corpID, namespace.id("owner"), 1, false)
	leadA := parityTestLead(t, namespace.id("a"), namespace.tenantID, corpID, namespace.key("a"), "Ada North", "13800000001", domain.LeadSourceManual, base)
	leadB := parityTestLead(t, namespace.id("b"), namespace.tenantID, corpID, namespace.key("b"), "Bob South", "13800000002", domain.LeadSourceImport, base.Add(time.Hour))
	leadOtherCorp := parityTestLead(t, namespace.id("other-corp"), namespace.tenantID, otherCorpID, namespace.key("other"), "Ada Hidden", "13800000003", domain.LeadSourceManual, base)
	for _, lead := range []domain.Lead{leadA, leadB, leadOtherCorp} {
		mustCreateLead(t, repository, lead)
	}
	assigned, err := repository.Assign(ctx, ports.AssignLeadCommand{TenantID: namespace.tenantID, CorpID: corpID, LeadID: leadA.ID, OwnerID: ownerID, Version: 1, UpdatedAt: base.Add(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	page, err := repository.List(ctx, ports.ListLeadsFilter{TenantID: namespace.tenantID, CorpID: corpID, Keyword: "Ada", Statuses: []domain.LeadStatus{domain.LeadStatusQualified}, Sources: []domain.LeadSource{domain.LeadSourceManual}, OwnerIDs: []int64{ownerID}, CreatedFrom: base.Add(-time.Minute), CreatedTo: base.Add(time.Minute), Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != assigned.ID {
		t.Fatalf("combined page=%#v", page.Items)
	}
	duplicate := parityTestLead(t, namespace.id("duplicate"), namespace.tenantID, corpID, namespace.key("duplicate"), "Duplicate", "13800000001", domain.LeadSourceWeCom, base)
	if _, _, err := repository.CreateOrGet(ctx, duplicate); !errors.Is(err, ports.ErrDuplicateLead) {
		t.Fatalf("duplicate error=%v", err)
	}
}

func TestLeadParityMariaDBConcurrentVersionAndPartialFailure(t *testing.T) {
	repository, db, namespace := integrationRepository(t)
	ctx := context.Background()
	now := time.Now().UTC()
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("race-corp"))
	owners := []int64{
		insertParityEmployee(t, db, corpID, namespace.id("race-owner-1"), 1, false),
		insertParityEmployee(t, db, corpID, namespace.id("race-owner-2"), 1, false),
	}
	partialOwnerID := insertParityEmployee(t, db, corpID, namespace.id("partial-owner"), 1, false)
	for _, ownerID := range append(append([]int64{}, owners...), partialOwnerID) {
		assertParityOwnerScope(t, db, namespace.tenantID, corpID, ownerID)
	}
	lead := parityTestLead(t, namespace.id("race"), namespace.tenantID, corpID, namespace.key("race"), "Race", "", domain.LeadSourceManual, now)
	mustCreateLead(t, repository, lead)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var successes atomic.Int32
	var wait sync.WaitGroup
	for _, owner := range owners {
		wait.Add(1)
		go func(ownerID int64) {
			defer wait.Done()
			<-start
			_, err := repository.Assign(ctx, ports.AssignLeadCommand{TenantID: namespace.tenantID, CorpID: corpID, LeadID: lead.ID, OwnerID: ownerID, Version: 1, UpdatedAt: now.Add(time.Second)})
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
	results, err := service.AssignLeads(ctx, application.AssignLeadsCommand{TenantID: namespace.tenantID, CorpID: corpID, OwnerID: partialOwnerID, Targets: []application.LeadMutationTarget{{ID: lead.ID, Version: 2}, {ID: namespace.id("missing"), Version: 1}, {ID: lead.ID, Version: 1}}})
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
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("convert-corp"))
	ownerID := insertParityEmployee(t, db, corpID, namespace.id("convert-owner"), 1, false)
	lead := parityTestLead(t, namespace.id("convert"), namespace.tenantID, corpID, namespace.key("convert"), "Convert", "13900000001", domain.LeadSourceManual, now)
	mustCreateLead(t, repository, lead)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM mochat_go_scrm_assignments WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
	})
	qualified, err := repository.Assign(ctx, ports.AssignLeadCommand{TenantID: namespace.tenantID, CorpID: corpID, LeadID: lead.ID, OwnerID: ownerID, Version: 1, UpdatedAt: now.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	converted, err := repository.Transition(ctx, ports.TransitionLeadCommand{TenantID: namespace.tenantID, CorpID: corpID, LeadID: lead.ID, ToStatus: domain.LeadStatusConverted, Version: qualified.Version, UpdatedAt: now.Add(2 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if converted.ConvertedContactID != lead.ID {
		t.Fatalf("converted=%#v", converted)
	}
	var contacts, assignments int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=? AND id=?", namespace.tenantID, corpID, lead.ID).Scan(&contacts); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_scrm_assignments WHERE tenant_id=? AND corp_id=? AND contact_id=? AND owner_id=?", namespace.tenantID, corpID, lead.ID, ownerID).Scan(&assignments); err != nil {
		t.Fatal(err)
	}
	if contacts != 1 || assignments != 1 {
		t.Fatalf("contacts=%d assignments=%d", contacts, assignments)
	}
}

func TestLeadParityMariaDBAssignmentRejectsOutOfScopeOwnersWithoutWrites(t *testing.T) {
	repository, db, namespace := integrationRepository(t)
	ctx := context.Background()
	now := time.Now().UTC()
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("owner-scope-corp"))
	otherCorpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("owner-other-corp"))
	otherTenantCorpID := insertParityCorp(t, db, namespace.otherTenantID, namespace.id("owner-other-tenant-corp"))
	validOwnerID := insertParityEmployee(t, db, corpID, namespace.id("owner-valid"), 1, false)
	otherCorpOwnerID := insertParityEmployee(t, db, otherCorpID, namespace.id("owner-other-corp"), 1, false)
	otherTenantOwnerID := insertParityEmployee(t, db, otherTenantCorpID, namespace.id("owner-other-tenant"), 1, false)
	inactiveOwnerID := insertParityEmployee(t, db, corpID, namespace.id("owner-inactive"), 2, false)
	deletedOwnerID := insertParityEmployee(t, db, corpID, namespace.id("owner-deleted"), 1, true)
	leads := []domain.Lead{
		parityTestLead(t, namespace.id("owner-target-a"), namespace.tenantID, corpID, namespace.key("owner-a"), "Owner A", "", domain.LeadSourceManual, now),
		parityTestLead(t, namespace.id("owner-target-b"), namespace.tenantID, corpID, namespace.key("owner-b"), "Owner B", "", domain.LeadSourceManual, now),
	}
	for _, lead := range leads {
		mustCreateLead(t, repository, lead)
	}
	service, err := application.NewService(repository, parityClock{now: now.Add(time.Second)}, parityIDGenerator{})
	if err != nil {
		t.Fatal(err)
	}
	results, err := service.AssignLeads(ctx, application.AssignLeadsCommand{TenantID: namespace.tenantID, CorpID: corpID, OwnerID: otherCorpOwnerID, Targets: []application.LeadMutationTarget{{ID: leads[0].ID, Version: 1}, {ID: leads[1].ID, Version: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].ErrorCode != "OWNER_OUT_OF_SCOPE" || results[1].ErrorCode != "OWNER_OUT_OF_SCOPE" {
		t.Fatalf("out-of-scope batch results=%#v", results)
	}
	for _, ownerID := range []int64{otherTenantOwnerID, inactiveOwnerID, deletedOwnerID} {
		if _, err := repository.Assign(ctx, ports.AssignLeadCommand{TenantID: namespace.tenantID, CorpID: corpID, LeadID: leads[0].ID, OwnerID: ownerID, Version: 1, UpdatedAt: now.Add(2 * time.Second)}); !errors.Is(err, ports.ErrLeadOwnerOutOfScope) {
			t.Fatalf("owner %d error=%v, want ErrLeadOwnerOutOfScope", ownerID, err)
		}
	}
	var unchanged int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_scrm_leads WHERE tenant_id=? AND corp_id=? AND id IN (?,?) AND status='new' AND owner_id IS NULL AND version=1", namespace.tenantID, corpID, leads[0].ID, leads[1].ID).Scan(&unchanged); err != nil {
		t.Fatal(err)
	}
	if unchanged != 2 {
		t.Fatalf("unchanged leads=%d, want 2", unchanged)
	}
	if _, err := repository.Assign(ctx, ports.AssignLeadCommand{TenantID: namespace.tenantID, CorpID: corpID, LeadID: leads[0].ID, OwnerID: validOwnerID, Version: 1, UpdatedAt: now.Add(3 * time.Second)}); err != nil {
		t.Fatalf("valid owner assignment: %v", err)
	}
}

func insertParityCorp(t *testing.T, db *sql.DB, tenantID int64, name string) int64 {
	t.Helper()
	result, err := db.Exec("INSERT INTO mc_corp (name,wx_corpid,tenant_id,created_at,updated_at) VALUES (?,?,?,?,?)", name, name, tenantID, time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM mc_corp WHERE id=?", id) })
	return id
}

func insertParityEmployee(t *testing.T, db *sql.DB, corpID int64, name string, status int, deleted bool) int64 {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = time.Now()
	}
	result, err := db.Exec("INSERT INTO mc_work_employee (wx_user_id,corp_id,name,status,created_at,updated_at,deleted_at) VALUES (?,?,?,?,?,?,?)", name, corpID, name, status, time.Now(), time.Now(), deletedAt)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM mc_work_employee WHERE id=?", id) })
	return id
}

func assertParityOwnerScope(t *testing.T, db *sql.DB, tenantID, corpID, ownerID int64) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mc_work_employee e INNER JOIN mc_corp c ON c.id=e.corp_id WHERE e.id=? AND e.corp_id=? AND e.status=1 AND e.deleted_at IS NULL AND c.tenant_id=? AND c.deleted_at IS NULL`, ownerID, corpID, tenantID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("owner scope missing: tenant=%d corp=%d owner=%d", tenantID, corpID, ownerID)
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
