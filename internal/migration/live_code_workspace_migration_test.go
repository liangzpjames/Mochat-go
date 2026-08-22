package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveCodeWorkspaceMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	upPath := filepath.Join(root, "deploy", "standalone", "migrations", "0153_live_code_workspace.up.sql")
	downPath := filepath.Join(root, "deploy", "standalone", "migrations", "0153_live_code_workspace.down.sql")
	upBody, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"ALTER TABLE `mc_channel_code`",
		"ADD COLUMN IF NOT EXISTS `validity_kind`",
		"ADD COLUMN IF NOT EXISTS `group_id`",
		"ADD KEY IF NOT EXISTS `idx_mc_work_room_auto_pull_group`",
		"validity_kind",
		"valid_from",
		"valid_until",
		"lifecycle_state",
		"provider_state",
		"data_source",
		"CREATE TABLE IF NOT EXISTS `mc_group_code_group`",
		"CREATE TABLE IF NOT EXISTS `mc_live_code_event`",
		"UNIQUE KEY `uk_live_code_event_source`",
		"SET `data_source` = 'simulation'",
	} {
		if !strings.Contains(string(upBody), required) {
			t.Fatalf("0153 up migration missing %q", required)
		}
	}
	for _, required := range []string{
		"@mochat_live_code_legacy",
		"0150_live_code_workspace",
		"DROP TABLE IF EXISTS `mc_live_code_event`",
		"DROP TABLE IF EXISTS `mc_group_code_group`",
		"DROP COLUMN `data_source`",
		"DROP COLUMN `provider_state`",
		"DROP COLUMN `lifecycle_state`",
		"DROP COLUMN `valid_until`",
		"DROP COLUMN `valid_from`",
		"DROP COLUMN `validity_kind`",
	} {
		if !strings.Contains(string(downBody), required) {
			t.Fatalf("0153 down migration missing %q", required)
		}
	}
}
