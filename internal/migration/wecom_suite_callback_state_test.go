package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWeComSuiteCallbackMigrationStoresOnlyEncryptedTicketAndReplayLedger(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0170_wecom_suite_callback_state.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0170_wecom_suite_callback_state.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(up))
	for _, required := range []string{"mochat_go_wecom_suite_tickets", "ticket_ciphertext", "ticket_key_id", "mochat_go_wecom_suite_callback_events", "event_digest", "processing", "completed", "failed"} {
		if !strings.Contains(text, required) {
			t.Fatalf("0170 missing %q", required)
		}
	}
	if strings.Contains(text, "`suite_ticket`") || strings.Contains(text, "`permanent_code`") || strings.Contains(text, "`auth_code`") {
		t.Fatal("0170 introduced plaintext suite secret columns")
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS `mochat_go_wecom_suite_callback_events`") || !strings.Contains(string(down), "DROP TABLE IF EXISTS `mochat_go_wecom_suite_tickets`") {
		t.Fatal("0170 down migration is incomplete")
	}
}
