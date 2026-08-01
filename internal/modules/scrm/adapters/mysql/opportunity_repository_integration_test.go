//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/ports"
	"jiyi/mochat-go/internal/mysqlconn"
)

func TestOpportunityRepositoryListUsesPersistedStageID(t *testing.T) {
	dsn := mysqlIntegrationDSN(t)
	db, err := mysqlconn.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	namespace := newIntegrationNamespace()
	corpID := namespace.tenantID + 10
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			"DELETE FROM mochat_go_scrm_opportunities WHERE tenant_id = ? AND corp_id = ?",
			namespace.tenantID, corpID,
		)
		_ = db.Close()
	})

	const opportunityID = "opportunity-stage-id"
	now := time.Now().UTC()
	_, err = db.ExecContext(context.Background(), `
		INSERT INTO mochat_go_scrm_opportunities
			(id, tenant_id, corp_id, contact_id, stage_id, status, lost_reason, version, amount, start_date, end_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'open', '', 1, 0, ?, ?, ?, ?)`,
		namespace.id(opportunityID), namespace.tenantID, corpID, namespace.id("contact"), "proposal",
		now, now, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	repository, err := NewOpportunityRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	page, err := repository.ListOpportunities(context.Background(), ports.OpportunityFilter{
		TenantID: namespace.tenantID,
		CorpID:   corpID,
		Stage:    "proposal",
		Status:   "open",
		PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Stage != "proposal" || page.NextCursor != "" {
		t.Fatalf("opportunities = %#v, want one proposal opportunity", page)
	}
}

func TestOpportunityRepositoryCombinedFilterAndCursorPagination(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("opportunity-filter-corp"))
	ownerID := insertParityEmployee(t, db, corpID, namespace.id("opportunity-owner"), 1, false)
	now := time.Now().UTC()
	for _, id := range []string{"opportunity-3", "opportunity-2", "opportunity-1"} {
		if _, err := db.Exec(`INSERT INTO mochat_go_scrm_opportunities(id,tenant_id,corp_id,contact_id,stage_id,status,lost_reason,owner_id,version,amount,start_date,end_date,created_at,updated_at) VALUES(?,?,?,?,?,'open','',?,1,10,?,?,?,?)`, namespace.id(id), namespace.tenantID, corpID, namespace.id("contact"), "proposal", ownerID, now, now, now, now); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_opportunities WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
	})
	repository, err := NewOpportunityRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.ListOpportunities(context.Background(), ports.OpportunityFilter{TenantID: namespace.tenantID, CorpID: corpID, Stage: "proposal", Status: "open", OwnerID: &ownerID, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page=%#v", first)
	}
	second, err := repository.ListOpportunities(context.Background(), ports.OpportunityFilter{TenantID: namespace.tenantID, CorpID: corpID, Stage: "proposal", Status: "open", OwnerID: &ownerID, Cursor: first.NextCursor, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.NextCursor != "" || second.Items[0].ID == first.Items[0].ID || second.Items[0].ID == first.Items[1].ID {
		t.Fatalf("second page=%#v first=%#v", second, first)
	}
}
