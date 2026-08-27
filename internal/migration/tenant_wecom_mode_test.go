package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test0167TenantWeComModeMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	var target Migration
	for _, item := range DefaultMigrations(root) {
		if item.Version == "0167_tenant_wecom_mode" {
			target = item
			break
		}
	}
	if target.Version == "" {
		t.Fatal("0167 tenant WeCom mode migration not found")
	}
	upBody, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(target.DownPath)
	if err != nil {
		t.Fatal(err)
	}
	up, down := string(upBody), string(downBody)
	for _, required := range []string{
		"ADD COLUMN `wecom_integration_mode` enum('self_built','third_party_delegated')",
		"FROM `mochat_go_wecom_integrations`",
		"`slot` = 'current'",
		"DEFAULT 'self_built'",
		"INSERT INTO `mochat_go_wecom_integrations`",
		"binding.`wecom_integration_mode`, 'current', 'unconfigured'",
		"AND existing.`slot` = 'current'",
		"ADD COLUMN IF NOT EXISTS `employee_credential_generation` bigint unsigned NOT NULL DEFAULT 1",
		"ADD COLUMN IF NOT EXISTS `contact_credential_generation` bigint unsigned NOT NULL DEFAULT 1",
		"ADD COLUMN IF NOT EXISTS `agent_credential_generation` bigint unsigned NOT NULL DEFAULT 1",
		"ADD COLUMN IF NOT EXISTS `callback_credential_generation` bigint unsigned NOT NULL DEFAULT 1",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("0167 up migration missing %q", required)
		}
	}
	if !strings.Contains(down, "DROP COLUMN `wecom_integration_mode`") {
		t.Fatal("0167 down migration must drop only the mode column")
	}
	for _, forbidden := range []string{"DROP TABLE", "credential_ciphertext", "permanent_code"} {
		if strings.Contains(down, forbidden) {
			t.Errorf("0167 down migration must not contain %q", forbidden)
		}
	}
}
