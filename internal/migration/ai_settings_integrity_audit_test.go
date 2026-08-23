package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAISettingsIntegrityAuditMigration(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0155_ai_settings_integrity_audit.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0155_ai_settings_integrity_audit.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"mochat_go_ai_settings_audits", "changed_fields", "actor_user_id", "idx_ai_settings_audits_scope"} {
		if !strings.Contains(string(up), fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS `mochat_go_ai_settings_audits`") {
		t.Fatal("rollback must remove only the audit table introduced by 0155")
	}
	if _, err := SplitSQLStatements(string(up)); err != nil {
		t.Fatalf("up migration is not executable by production splitter: %v", err)
	}
	if _, err := SplitSQLStatements(string(down)); err != nil {
		t.Fatalf("down migration is not executable by production splitter: %v", err)
	}
}
