//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"jiyi/mochat-go/internal/mysqlconn"
	"jiyi/mochat-go/internal/modules/scrm/ports"
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
	items, err := repository.ListOpportunities(context.Background(), ports.OpportunityFilter{
		TenantID: namespace.tenantID,
		CorpID:   corpID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Stage != "proposal" {
		t.Fatalf("opportunities = %#v, want one proposal opportunity", items)
	}
}
