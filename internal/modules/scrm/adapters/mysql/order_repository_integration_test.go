//go:build integration

package mysql

import (
	"context"
	"errors"
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
	items, _, err := repository.ListContext(ctx, namespace.tenantID, corpID, 1, 20)
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

func TestOrderRepositoryRollsBackCreateAndTransitionWhenAuditFails(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	repository, err := NewSQLOrderRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("atomic-order-corp"))
	contactID := namespace.id("atomic-order-contact")
	now := time.Now().UTC()
	cleanup := func() {
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_audit WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_orders WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err = db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`, contactID, namespace.tenantID, corpID, "atomic contact", "", now, now); err != nil {
		t.Fatal(err)
	}

	repository.auditFailure = errors.New("injected audit failure")
	failedOrderID := namespace.id("create-audit-failure")
	_, err = repository.CreateContext(ctx, domain.Order{ID: failedOrderID, TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, Title: "failed create", AmountCents: 100, Currency: "CNY", Status: domain.OrderPending, Version: 1}, 7)
	if !errors.Is(err, repository.auditFailure) {
		t.Fatalf("CreateContext error = %v, want injected audit failure", err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM mochat_go_scrm_orders WHERE id=? AND tenant_id=? AND corp_id=?", failedOrderID, namespace.tenantID, corpID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("created orders = %d, want 0 after audit rollback", count)
	}

	repository.auditFailure = nil
	transitionOrderID := namespace.id("transition-audit-failure")
	_, err = repository.CreateContext(ctx, domain.Order{ID: transitionOrderID, TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, Title: "failed transition", AmountCents: 100, Currency: "CNY", Status: domain.OrderPending, Version: 1}, 7)
	if err != nil {
		t.Fatal(err)
	}
	repository.auditFailure = errors.New("injected transition audit failure")
	_, err = repository.TransitionContext(ctx, transitionOrderID, namespace.tenantID, corpID, domain.OrderPaid, 1, 8)
	if !errors.Is(err, repository.auditFailure) {
		t.Fatalf("TransitionContext error = %v, want injected audit failure", err)
	}
	var status domain.OrderStatus
	var version int64
	if err = db.QueryRow("SELECT status,version FROM mochat_go_scrm_orders WHERE id=? AND tenant_id=? AND corp_id=?", transitionOrderID, namespace.tenantID, corpID).Scan(&status, &version); err != nil {
		t.Fatal(err)
	}
	if status != domain.OrderPending || version != 1 {
		t.Fatalf("order status/version = %s/%d, want pending/1 after audit rollback", status, version)
	}
}
