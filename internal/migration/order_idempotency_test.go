package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderIdempotencyMigrationStoresScopedCompletedReceiptsAndIsMySQL57Compatible(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0173_scrm_order_idempotency.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0173_scrm_order_idempotency.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(up)
	normalizedSource := strings.Join(strings.Fields(source), " ")
	for _, required := range []string{
		"mochat_go_scrm_order_idempotency_receipts",
		"`tenant_id`", "`corp_id`", "`idempotency_key`", "`request_hash`", "`order_id`",
		"`response_status`", "`response_body`",
		"PRIMARY KEY (`tenant_id`,`corp_id`,`idempotency_key`)",
		"ALTER TABLE `mochat_go_scrm_orders` MODIFY COLUMN `idempotency_key` varbinary(128) NOT NULL",
		"`idempotency_key` varbinary(128) NOT NULL",
	} {
		if !strings.Contains(normalizedSource, required) {
			t.Errorf("0173 up migration missing %q", required)
		}
	}
	for _, incompatible := range []string{"SKIP LOCKED", "RETURNING", "CREATE INDEX IF NOT EXISTS", "CHECK ("} {
		if strings.Contains(strings.ToUpper(source), incompatible) {
			t.Errorf("0173 uses MySQL 5.7 incompatible syntax %q", incompatible)
		}
	}
	downSource := strings.Join(strings.Fields(string(down)), " ")
	if !strings.Contains(downSource, "DROP TABLE IF EXISTS `mochat_go_scrm_order_idempotency_receipts`") {
		t.Fatal("0173 down migration does not remove order idempotency receipts")
	}
	if strings.Contains(downSource, "ALTER TABLE `mochat_go_scrm_orders`") {
		t.Fatal("0173 down must preserve VARBINARY opaque-key semantics for all post-up byte-distinct orders")
	}
}
