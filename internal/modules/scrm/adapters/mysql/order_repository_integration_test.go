//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/domain"
)

func TestOrderRepositoryRoundTripsProductFieldsContactNameAndAudit(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	repository, err := NewSQLOrderRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("order-corp"))
	contactID := namespace.id("order-contact")
	orderID := namespace.id("order")
	now := time.Now().UTC()
	cleanup := func() {
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_audit WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_orders WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err = db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`, contactID, namespace.tenantID, corpID, "张三", "", now, now); err != nil {
		t.Fatal(err)
	}
	created, err := repository.CreateContext(ctx, domain.Order{ID: orderID, TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, OpportunityID: namespace.id("opp"), Title: "年度续费", Note: "客户确认", AmountCents: 1200, Currency: "CNY", Status: domain.OrderPending, Version: 1}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if created.Title != "年度续费" || created.Note != "客户确认" {
		t.Fatalf("created = %#v", created)
	}
	items, err := repository.ListContext(ctx, namespace.tenantID, corpID)
	if err != nil || len(items) != 1 || items[0].ContactName != "张三" || items[0].Title != "年度续费" || items[0].Note != "客户确认" {
		t.Fatalf("items = %#v, err = %v", items, err)
	}
	detail, err := repository.GetContext(ctx, orderID, namespace.tenantID, corpID)
	if err != nil || detail.ContactName != "张三" || detail.Title != "年度续费" || detail.Note != "客户确认" {
		t.Fatalf("detail = %#v, err = %v", detail, err)
	}
	if _, err = repository.TransitionContext(ctx, orderID, namespace.tenantID, corpID, domain.OrderPaid, 1, 8); err != nil {
		t.Fatal(err)
	}
	audit, err := repository.AuditContext(ctx, orderID, namespace.tenantID, corpID)
	if err != nil || len(audit) != 2 || audit[0]["action"] != "created" || audit[1]["action"] != "transition" {
		t.Fatalf("audit = %#v, err = %v", audit, err)
	}
}
