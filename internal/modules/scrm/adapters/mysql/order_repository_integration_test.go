//go:build integration

package mysql

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/modules/scrm/domain"
)

func TestOrderRepositoryRoundTripsProductFieldsContactNameAndAudit(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	ensureOrderIdempotencySchema(t, db)
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
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_idempotency_receipts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_audit WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_orders WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err = db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`, contactID, namespace.tenantID, corpID, "张三", "", now, now); err != nil {
		t.Fatal(err)
	}
	created := domain.Order{ID: orderID, TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, OpportunityID: namespace.id("opp"), Title: "年度续费", Note: "客户确认", AmountCents: 1200, Currency: "CNY", Status: domain.OrderPending, Version: 1}
	receipt, err := repository.CreateIdempotentContext(ctx, idempotentOrderCommand(t, created, namespace.key("roundtrip-order")))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.OrderID != created.ID {
		t.Fatalf("created receipt order ID = %q, want %q", receipt.OrderID, created.ID)
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

func TestOrderRepositoryIdempotentCreateReplaysExactReceiptUnderThirtyTwoConcurrentRequests(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	ensureOrderIdempotencySchema(t, db)
	repository, err := NewSQLOrderRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("idem-order-corp"))
	contactID := namespace.id("idem-contact")
	now := time.Now().UTC()
	cleanup := func() {
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_idempotency_receipts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_audit WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_orders WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err = db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`, contactID, namespace.tenantID, corpID, "concurrent contact", "", now, now); err != nil {
		t.Fatal(err)
	}

	const concurrency = 32
	results := make([]domain.OrderCreateReceipt, concurrency)
	errorsByWorker := make([]error, concurrency)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := 0; index < concurrency; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			order := domain.Order{ID: namespace.id(fmt.Sprintf("idem-%02d", index)), TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, Title: "concurrent renewal", AmountCents: 1200, Currency: "CNY", Status: domain.OrderPending, Version: 1}
			results[index], errorsByWorker[index] = repository.CreateIdempotentContext(ctx, idempotentOrderCommand(t, order, namespace.key("shared-order-intent")))
		}(index)
	}
	close(start)
	wait.Wait()

	firstBody := results[0].ResponseBody
	firstOrderID := results[0].OrderID
	createdCount := 0
	for index := range results {
		if errorsByWorker[index] != nil {
			t.Fatalf("worker %d: %v", index, errorsByWorker[index])
		}
		if results[index].ResponseStatus != 200 || results[index].OrderID != firstOrderID || !bytes.Equal(results[index].ResponseBody, firstBody) {
			t.Fatalf("worker %d receipt = %#v body=%q, want order=%q body=%q", index, results[index], results[index].ResponseBody, firstOrderID, firstBody)
		}
		if !results[index].Replayed {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("non-replayed receipts = %d, want exactly 1 creator", createdCount)
	}
	assertScopedOrderCounts(t, db, namespace.tenantID, corpID, 1, 1, 1)

	conflicting := domain.Order{ID: namespace.id("conflict"), TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, Title: "concurrent renewal", AmountCents: 9999, Currency: "CNY", Status: domain.OrderPending, Version: 1}
	if _, err := repository.CreateIdempotentContext(ctx, idempotentOrderCommand(t, conflicting, namespace.key("shared-order-intent"))); !errors.Is(err, domain.ErrOrderIdempotencyConflict) {
		t.Fatalf("conflicting retry error = %v, want idempotency conflict", err)
	}
}

func TestOrderRepositoryScopesSameIdempotencyKeyByTenantAndCorp(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	ensureOrderIdempotencySchema(t, db)
	repository, err := NewSQLOrderRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	tenantCorp := insertParityCorp(t, db, namespace.tenantID, namespace.id("scope-corp-a"))
	secondCorp := insertParityCorp(t, db, namespace.tenantID, namespace.id("scope-corp-b"))
	otherTenantCorp := insertParityCorp(t, db, namespace.otherTenantID, namespace.id("scope-corp-c"))
	scopes := []struct{ tenantID, corpID int64 }{{namespace.tenantID, tenantCorp}, {namespace.tenantID, secondCorp}, {namespace.otherTenantID, otherTenantCorp}}
	now := time.Now().UTC()
	for index, scope := range scopes {
		contactID := namespace.id(fmt.Sprintf("scope-contact-%d", index))
		if _, err := db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`, contactID, scope.tenantID, scope.corpID, "scoped contact", "", now, now); err != nil {
			t.Fatal(err)
		}
		order := domain.Order{ID: namespace.id(fmt.Sprintf("scope-order-%d", index)), TenantID: scope.tenantID, CorpID: scope.corpID, ContactID: contactID, Title: "scoped renewal", AmountCents: 100, Currency: "CNY", Status: domain.OrderPending, Version: 1}
		if _, err := repository.CreateIdempotentContext(context.Background(), idempotentOrderCommand(t, order, namespace.key("same-key"))); err != nil {
			t.Fatalf("scope %d/%d: %v", scope.tenantID, scope.corpID, err)
		}
		t.Cleanup(func() {
			_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_idempotency_receipts WHERE tenant_id=? AND corp_id=?", scope.tenantID, scope.corpID)
			_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_audit WHERE tenant_id=? AND corp_id=?", scope.tenantID, scope.corpID)
			_, _ = db.Exec("DELETE FROM mochat_go_scrm_orders WHERE tenant_id=? AND corp_id=?", scope.tenantID, scope.corpID)
			_, _ = db.Exec("DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=?", scope.tenantID, scope.corpID)
		})
		assertScopedOrderCounts(t, db, scope.tenantID, scope.corpID, 1, 1, 1)
	}
}

func TestOrderRepositoryTreatsIdempotencyKeysAsOpaqueByteSequences(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	ensureOrderIdempotencySchema(t, db)
	repository, err := NewSQLOrderRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("case-key-corp"))
	contactID := namespace.id("case-key-contact")
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`, contactID, namespace.tenantID, corpID, "case key contact", "", now, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_idempotency_receipts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_audit WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_orders WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
	})
	keys := []string{
		namespace.key("Intent-Key"),
		namespace.key("intent-key"),
		namespace.key("Intent-Key "),
		namespace.key("Intent-Key\u00a0"),
		namespace.key("Intent-Key\u3000"),
	}
	for index, key := range keys {
		order := domain.Order{ID: namespace.id(fmt.Sprintf("case-key-order-%d", index)), TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, Title: "case-sensitive key", AmountCents: 100, Currency: "CNY", Status: domain.OrderPending, Version: 1}
		command := idempotentOrderCommand(t, order, key)
		first, err := repository.CreateIdempotentContext(context.Background(), command)
		if err != nil {
			t.Fatalf("create with opaque key %q: %v", key, err)
		}
		replayed, err := repository.CreateIdempotentContext(context.Background(), command)
		if err != nil || !replayed.Replayed || !bytes.Equal(replayed.ResponseBody, first.ResponseBody) {
			t.Fatalf("exact opaque key replay %q = %#v, err=%v", key, replayed, err)
		}
	}
	assertScopedOrderCounts(t, db, namespace.tenantID, corpID, len(keys), len(keys), len(keys))
}

func TestOrderRepositoryReplaysReceiptAfterOrderChangesOrIsSoftDeleted(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	ensureOrderIdempotencySchema(t, db)
	repository, err := NewSQLOrderRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("immutable-receipt-corp"))
	contactID := namespace.id("immutable-receipt-contact")
	orderID := namespace.id("immutable-receipt-order")
	key := namespace.key("immutable-receipt-key")
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`, contactID, namespace.tenantID, corpID, "immutable receipt contact", "", now, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_idempotency_receipts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_audit WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_orders WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
	})
	order := domain.Order{ID: orderID, TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, Title: "immutable receipt", AmountCents: 100, Currency: "CNY", Status: domain.OrderPending, Version: 1}
	command := idempotentOrderCommand(t, order, key)
	first, err := repository.CreateIdempotentContext(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE mochat_go_scrm_orders SET status='cancelled',version=2,deleted_at=? WHERE id=? AND tenant_id=? AND corp_id=?`, now, orderID, namespace.tenantID, corpID); err != nil {
		t.Fatal(err)
	}
	replayed, err := repository.CreateIdempotentContext(context.Background(), command)
	if err != nil {
		t.Fatalf("replay after order mutation and soft delete: %v", err)
	}
	if !replayed.Replayed || replayed.ResponseStatus != first.ResponseStatus || !bytes.Equal(replayed.ResponseBody, first.ResponseBody) {
		t.Fatalf("replayed receipt = %#v body=%q, want exact first status/body=%d/%q", replayed, replayed.ResponseBody, first.ResponseStatus, first.ResponseBody)
	}
}

func TestOrderIdempotencyMigrationRunsUpAndDownOnMySQL57OrMariaDB(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	table := "mochat_go_scrm_order_idem_" + strings.ReplaceAll(namespace.prefix, "-", "_")
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("migration-corp"))
	keys := []string{
		namespace.key("Downgrade-Key"),
		namespace.key("downgrade-key"),
		namespace.key("Downgrade-Key "),
		namespace.key("Downgrade-Key\u00a0"),
		namespace.key("Downgrade-Key\u3000"),
	}
	orderIDs := make([]string, len(keys))
	for index := range orderIDs {
		orderIDs[index] = namespace.id(fmt.Sprintf("migration-order-%d", index))
	}
	root := filepath.Join("..", "..", "..", "..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0173_scrm_order_idempotency.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0173_scrm_order_idempotency.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	rewrite := func(source []byte) string {
		return strings.ReplaceAll(string(source), "mochat_go_scrm_order_idempotency_receipts", table)
	}
	_, _ = db.Exec("DROP TABLE IF EXISTS `" + table + "`")
	t.Cleanup(func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS `" + table + "`")
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_orders WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
	})
	if err := execOrderMigrationScript(db, rewrite(up)); err != nil {
		t.Fatalf("apply 0173 up: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count); err != nil || count != 1 {
		t.Fatalf("receipt table after up = %d, err=%v", count, err)
	}
	var dataType, columnType string
	var collation sql.NullString
	if err := db.QueryRow(`SELECT data_type,column_type,collation_name FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_orders' AND column_name='idempotency_key'`).Scan(&dataType, &columnType, &collation); err != nil || dataType != "varbinary" || columnType != "varbinary(128)" || collation.Valid {
		t.Fatalf("orders idempotency key type after up = %q/%q collation=%#v, err=%v", dataType, columnType, collation, err)
	}
	now := time.Now().UTC()
	for index, key := range keys {
		if _, err := db.Exec(`INSERT INTO mochat_go_scrm_orders (id,tenant_id,corp_id,contact_id,opportunity_id,title,note,amount_cents,currency,status,version,idempotency_key,created_by,created_at,updated_at) VALUES (?,?,?,?,NULL,?,?,?,'CNY','pending',1,?,7,?,?)`, orderIDs[index], namespace.tenantID, corpID, namespace.id("migration-contact"), "migration order", "", 100, key, now, now); err != nil {
			t.Fatalf("insert post-up case-distinct order %q: %v", key, err)
		}
	}
	if err := execOrderMigrationScript(db, rewrite(down)); err != nil {
		t.Fatalf("apply 0173 down: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count); err != nil || count != 0 {
		t.Fatalf("receipt table after down = %d, err=%v", count, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_scrm_orders WHERE tenant_id=? AND corp_id=?`, namespace.tenantID, corpID).Scan(&count); err != nil || count != len(keys) {
		t.Fatalf("post-up byte-distinct orders after down = %d, err=%v", count, err)
	}
	if err := db.QueryRow(`SELECT data_type,column_type,collation_name FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_scrm_orders' AND column_name='idempotency_key'`).Scan(&dataType, &columnType, &collation); err != nil || dataType != "varbinary" || columnType != "varbinary(128)" || collation.Valid {
		t.Fatalf("orders idempotency key type after down = %q/%q collation=%#v, err=%v", dataType, columnType, collation, err)
	}
}

func TestOrderRepositoryRollsBackCreateAndTransitionWhenAuditFails(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	ensureOrderIdempotencySchema(t, db)
	repository, err := NewSQLOrderRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("atomic-order-corp"))
	contactID := namespace.id("atomic-order-contact")
	now := time.Now().UTC()
	cleanup := func() {
		_, _ = db.Exec("DELETE FROM mochat_go_scrm_order_idempotency_receipts WHERE tenant_id=? AND corp_id=?", namespace.tenantID, corpID)
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
	failedOrder := domain.Order{ID: failedOrderID, TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, Title: "failed create", AmountCents: 100, Currency: "CNY", Status: domain.OrderPending, Version: 1}
	_, err = repository.CreateIdempotentContext(ctx, idempotentOrderCommand(t, failedOrder, namespace.key("audit-failure")))
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
	if err = db.QueryRow("SELECT COUNT(*) FROM mochat_go_scrm_order_idempotency_receipts WHERE tenant_id=? AND corp_id=? AND idempotency_key=?", namespace.tenantID, corpID, namespace.key("audit-failure")).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("receipts = %d, want 0 after audit rollback", count)
	}

	repository.auditFailure = nil
	transitionOrderID := namespace.id("transition-audit-failure")
	transitionOrder := domain.Order{ID: transitionOrderID, TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, Title: "failed transition", AmountCents: 100, Currency: "CNY", Status: domain.OrderPending, Version: 1}
	_, err = repository.CreateIdempotentContext(ctx, idempotentOrderCommand(t, transitionOrder, namespace.key("transition-fixture")))
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

func idempotentOrderCommand(t *testing.T, order domain.Order, key string) domain.OrderCreateCommand {
	t.Helper()
	hash, err := domain.OrderCreateRequestHash(order, "")
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(fmt.Sprintf(`{"code":200,"data":{"id":%q},"msg":"success"}\n`, order.ID))
	return domain.OrderCreateCommand{Order: order, ActorID: 7, IdempotencyKey: key, RequestHash: hash, ResponseStatus: 200, ResponseBody: body}
}

func ensureOrderIdempotencySchema(t *testing.T, db interface {
	Exec(string, ...any) (sql.Result, error)
}) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "..", "deploy", "standalone", "migrations", "0173_scrm_order_idempotency.up.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := execOrderMigrationScript(db, string(body)); err != nil {
		t.Fatal(err)
	}
}

func execOrderMigrationScript(db interface {
	Exec(string, ...any) (sql.Result, error)
}, script string) error {
	statements, err := migration.SplitSQLStatements(script)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func assertScopedOrderCounts(t *testing.T, db *sql.DB, tenantID, corpID int64, wantOrders, wantAudits, wantReceipts int) {
	t.Helper()
	for table, want := range map[string]int{
		"mochat_go_scrm_orders":                     wantOrders,
		"mochat_go_scrm_order_audit":                wantAudits,
		"mochat_go_scrm_order_idempotency_receipts": wantReceipts,
	} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE tenant_id=? AND corp_id=?", tenantID, corpID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Fatalf("%s count = %d, want %d", table, count, want)
		}
	}
}
