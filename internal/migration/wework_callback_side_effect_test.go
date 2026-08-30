package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWeWorkCallbackSideEffectMigrationIsScopedFailClosedAndMySQL57Compatible(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0174_wework_callback_side_effects.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0174_wework_callback_side_effects.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(up)
	for _, required := range []string{
		"mochat_go_wework_callback_side_effects", "`tenant_id`", "`corp_id`", "`event_key`", "`action_key`", "`payload_hash`",
		"`status`", "'pending'", "'unknown'", "'sent'", "PRIMARY KEY (`tenant_id`,`corp_id`,`event_key`,`action_key`)",
		"FOREIGN KEY (`tenant_id`,`corp_id`,`event_key`) REFERENCES `mochat_go_wework_callback_inbox` (`tenant_id`,`corp_id`,`event_key`)",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("0174 up migration missing %q", required)
		}
	}
	for _, incompatible := range []string{"SKIP LOCKED", "RETURNING", "CREATE INDEX IF NOT EXISTS", "CHECK ("} {
		if strings.Contains(strings.ToUpper(source), incompatible) {
			t.Errorf("0174 uses MySQL 5.7 incompatible syntax %q", incompatible)
		}
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS `mochat_go_wework_callback_side_effects`") {
		t.Fatal("0174 down migration does not remove callback side effects")
	}
}
