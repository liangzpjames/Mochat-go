package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test0166WeComIntegrationAndArchiveMediaMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	var target Migration
	for _, item := range DefaultMigrations(root) {
		if item.Version == "0166_wecom_integration_and_archive_media" {
			target = item
			break
		}
	}
	if target.Version == "" {
		t.Fatal("0166 WeCom integration and archive media migration not found")
	}
	if !strings.HasSuffix(filepath.ToSlash(target.DownPath), "0166_wecom_integration_and_archive_media.down.sql") {
		t.Fatalf("0166 down path=%q", target.DownPath)
	}
	upBody, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(target.DownPath)
	if err != nil {
		t.Fatal(err)
	}
	up := string(upBody)
	down := string(downBody)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS `mochat_go_wecom_integrations`",
		"`mode` enum('self_built','third_party_delegated')",
		"`slot` enum('current','candidate')",
		"`status` enum('unconfigured','pending_verification','active','suspended','revoked','failed')",
		"`generation` bigint(20) unsigned",
		"`version` bigint(20) unsigned",
		"UNIQUE KEY `uk_wecom_integration_scope_slot` (`tenant_id`,`corp_id`,`slot`)",
		"FOREIGN KEY (`tenant_id`,`corp_id`) REFERENCES `mc_corp` (`tenant_id`,`id`)",
		"CREATE TABLE IF NOT EXISTS `mochat_go_archive_media_objects`",
		"`sdk_file_id_hash` char(64)",
		"`lease_token` varchar(96)",
		"`lease_expires_at` datetime(6)",
		"`heartbeat_at` datetime(6)",
		"UNIQUE KEY `uk_archive_media_source_identity` (`tenant_id`,`corp_id`,`msgid`,`sdk_file_id_hash`)",
		"KEY `idx_archive_media_claim_lease`",
		"KEY `idx_archive_media_scope_msgid` (`tenant_id`,`corp_id`,`msgid`)",
		"fake_tenant_%",
		"'unconfigured'",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("0166 up migration missing %q", required)
		}
	}
	if strings.Contains(strings.ToLower(up), "fake_tenant_%')\nset `status` = 'active'") {
		t.Fatal("0166 fake tenant backfill may activate a fake corp")
	}
	for _, required := range []string{
		"DROP TABLE IF EXISTS `mochat_go_archive_media_objects`",
		"DROP TABLE IF EXISTS `mochat_go_wecom_integrations`",
	} {
		if !strings.Contains(down, required) {
			t.Errorf("0166 down migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"DROP TABLE `mc_corp`",
		"DROP TABLE `mc_tenant`",
		"DROP TABLE `mochat_go_tenant_corp_bindings`",
	} {
		if strings.Contains(down, forbidden) {
			t.Errorf("0166 down migration must not delete existing table %q", forbidden)
		}
	}
}
