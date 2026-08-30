package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWeWorkCallbackSideEffectReconciliation0176Contract(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0176_wework_callback_side_effect_reconciliation.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0176_wework_callback_side_effect_reconciliation.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	upSQL, downSQL := string(up), strings.ReplaceAll(string(down), "`", "")
	for _, required := range []string{"reconciliation_fence", "reconcile_after", "mochat_go_wework_callback_side_effect_commands", "request_fingerprint", "reservation_token", "remaining_unknown_actions", "dashboard.company_setting.website", "/dashboard/company/callback-side-effects"} {
		if !strings.Contains(upSQL, required) {
			t.Errorf("0176 up missing %q", required)
		}
	}
	if strings.Contains(upSQL, "fk_wework_callback_side_effect_command_action") {
		t.Error("0176 receipt must not lock the action before the canonical inbox/action lock order")
	}
	for _, required := range []string{"DROP TABLE mochat_go_wework_callback_side_effect_commands", "DROP INDEX idx_wework_callback_side_effect_unknown", "DROP COLUMN reconciliation_fence", "DROP COLUMN version"} {
		if !strings.Contains(downSQL, required) {
			t.Errorf("0176 down missing %q", required)
		}
	}
	for _, incompatible := range []string{"SKIP LOCKED", "RETURNING ", "DROP COLUMN IF EXISTS", "CREATE INDEX IF NOT EXISTS"} {
		if strings.Contains(strings.ToUpper(upSQL+"\n"+downSQL), incompatible) {
			t.Errorf("0176 uses MySQL 5.7-incompatible syntax %q", incompatible)
		}
	}
}

func TestDefaultMigrationsLatestIsCallbackSideEffectReconciliation0176(t *testing.T) {
	migrations := DefaultMigrations(filepath.Join("..", ".."))
	if got := migrations[len(migrations)-1].Version; got != "0176_wework_callback_side_effect_reconciliation" {
		t.Fatalf("latest migration=%q", got)
	}
}
