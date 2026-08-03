package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTimeoutWarningMigrationDeclaresCompleteProviderSchema(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0111_timeout_warning_provider.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0111_timeout_warning_provider.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{
		"mochat_go_timeout_rules", "mochat_go_timeout_rule_strategies",
		"mochat_go_timeout_rule_quiet_periods", "mochat_go_timeout_rule_notify_targets",
		"mochat_go_timeout_settings", "mochat_go_timeout_records",
		"mochat_go_timeout_record_audits", "mochat_go_timeout_notification_intents",
	} {
		if !strings.Contains(string(up), "CREATE TABLE IF NOT EXISTS `"+table+"`") {
			t.Fatalf("up migration missing table %s", table)
		}
		if !strings.Contains(string(down), "DROP TABLE IF EXISTS `"+table+"`") {
			t.Fatalf("down migration missing table %s", table)
		}
	}
	if !strings.Contains(string(up), "UNIQUE KEY `uk_timeout_record_strategy_message`") {
		t.Fatal("timeout record idempotency key missing")
	}
}
