package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveComponentLocatorMigrationIsTenantScopedAndReversible(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0169_archive_component_locator.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0169_archive_component_locator.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"mochat_go_archive_component_locators", "tenant_id", "corp_id", "msgid",
		"locator_ciphertext", "locator_key_id", "public_key_version",
		"UNIQUE KEY `uk_archive_component_scope_message` (`tenant_id`,`corp_id`,`msgid`)",
		"/dashboard/archive/components/{id}/session", "/dashboard/archive/components/session/{token}",
	} {
		if !strings.Contains(string(up), fragment) {
			t.Errorf("0169 up migration missing %q", fragment)
		}
	}
	if strings.Contains(strings.ToLower(string(up)), "encrypted_secret_key` varchar") || strings.Contains(strings.ToLower(string(up)), "permanent_code") {
		t.Fatal("0169 stores sensitive locator material outside the ciphertext envelope")
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS `mochat_go_archive_component_locators`") {
		t.Fatal("0169 down migration does not remove component locator table")
	}
}

func TestControlledIdentityDependentMigrationsAreNotMariaDBInitScripts(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(compose)
	for _, migration := range []string{
		"0169_archive_component_locator.up.sql:/docker-entrypoint-initdb.d",
		"0170_wecom_suite_callback_state.up.sql:/docker-entrypoint-initdb.d",
		"0171_archive_fixture_dataset_ledger.up.sql:/docker-entrypoint-initdb.d",
	} {
		if strings.Contains(source, migration) {
			t.Errorf("controlled-schema dependent migration runs before the migration ledger: %s", migration)
		}
	}
}
