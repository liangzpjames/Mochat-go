package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveCodeWorkspaceMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	upPath := filepath.Join(root, "deploy", "standalone", "migrations", "0150_live_code_workspace.up.sql")
	downPath := filepath.Join(root, "deploy", "standalone", "migrations", "0150_live_code_workspace.down.sql")
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
		"validity_kind",
		"valid_from",
		"valid_until",
		"lifecycle_state",
		"provider_state",
		"data_source",
		"CREATE TABLE `mc_group_code_group`",
		"CREATE TABLE `mc_live_code_event`",
		"UNIQUE KEY `uk_live_code_event_source`",
		"SET `data_source` = 'simulation'",
	} {
		if !strings.Contains(string(upBody), required) {
			t.Fatalf("0150 up migration missing %q", required)
		}
	}
	for _, required := range []string{
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
			t.Fatalf("0150 down migration missing %q", required)
		}
	}
}

func TestGroupCodeDirectJoinSchemaAndMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	schemaBody, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "schema", "mochat.sql"))
	if err != nil {
		t.Fatal(err)
	}
	schema := string(schemaBody)
	channelStart := strings.Index(schema, "CREATE TABLE IF NOT EXISTS `mc_channel_code`")
	channelEnd := strings.Index(schema[channelStart:], "CREATE TABLE IF NOT EXISTS `mc_channel_code_group`")
	roomPullStart := strings.Index(schema, "CREATE TABLE IF NOT EXISTS `mc_work_room_auto_pull`")
	roomPullEnd := strings.Index(schema[roomPullStart:], "CREATE TABLE IF NOT EXISTS `mc_work_room_group`")
	if channelStart < 0 || channelEnd < 0 || roomPullStart < 0 || roomPullEnd < 0 {
		t.Fatal("cannot locate live-code tables in fresh schema")
	}
	channelTable := schema[channelStart : channelStart+channelEnd]
	roomPullTable := schema[roomPullStart : roomPullStart+roomPullEnd]
	for _, column := range []string{"`provider_kind`", "`auto_create_room`", "`room_base_name`", "`room_base_id`"} {
		if strings.Contains(channelTable, column) {
			t.Fatalf("fresh channel-code schema must not contain group join-way column %s", column)
		}
		if !strings.Contains(roomPullTable, column) {
			t.Fatalf("fresh work-room-auto-pull schema missing %s", column)
		}
	}

	upBody, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0152_group_code_direct_join.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0152_group_code_direct_join.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"ALTER TABLE `mc_work_room_auto_pull`", "`provider_kind`", "`auto_create_room`", "`room_base_name`", "`room_base_id`"} {
		if !strings.Contains(string(upBody), required) || !strings.Contains(string(downBody), required) {
			t.Fatalf("0152 migration contract missing %q", required)
		}
	}
}
