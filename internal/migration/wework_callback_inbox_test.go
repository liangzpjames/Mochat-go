package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWeWorkCallbackInboxMigrationIsScopedFencedAndMySQL57Compatible(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0172_wework_callback_inbox.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0172_wework_callback_inbox.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(up)
	for _, required := range []string{
		"mochat_go_wework_callback_inbox", "`tenant_id`", "`corp_id`", "`event_key`", "`payload_fingerprint`",
		"`lease_token`", "`lease_fence`", "`lease_expires_at`", "`next_attempt_at`", "`dependency_defer_count`",
		"mochat_go_wework_callback_cutovers", "legacy-redis-v1", "`status`", "`imported_count`", "`source_fingerprint`", "`owner_token`", "INSERT IGNORE",
		"UNIQUE KEY `uk_wework_callback_scope_event` (`tenant_id`,`corp_id`,`event_key`)",
		"FOREIGN KEY (`tenant_id`,`corp_id`) REFERENCES `mc_corp` (`tenant_id`,`id`)",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("0172 up migration missing %q", required)
		}
	}
	for _, incompatible := range []string{"SKIP LOCKED", "RETURNING", "CREATE INDEX IF NOT EXISTS"} {
		if strings.Contains(strings.ToUpper(source), incompatible) {
			t.Errorf("0172 uses MySQL 5.7 incompatible syntax %q", incompatible)
		}
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS `mochat_go_wework_callback_inbox`") {
		t.Fatal("0172 down migration does not remove callback inbox")
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS `mochat_go_wework_callback_cutovers`") {
		t.Fatal("0172 down migration does not remove callback cutover marker")
	}
}
