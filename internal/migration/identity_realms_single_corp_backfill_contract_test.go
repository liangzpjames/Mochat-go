package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityRealmsSingleCorpBackfillMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	upBody, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0130_identity_realms_single_corp_backfill.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0130_identity_realms_single_corp_backfill.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up := strings.ToLower(strings.ReplaceAll(string(upBody), "`", ""))
	down := strings.ToLower(strings.ReplaceAll(string(downBody), "`", ""))
	for _, required := range []string{
		"mochat_go_identity_migration_ledger",
		"mochat_go_identity_migration_journal",
		"mochat_go_identity_migration_batches",
		"mochat_go_identity_migration_corp_map",
		"mochat_go_saas_admin_users",
		"mochat_go_saas_admin_user_access",
		"mochat_go_saas_admin_user_roles",
		"fk_saas_admin_user_access_identity",
		"fk_saas_admin_user_roles_identity",
		"migration_source",
		"script_checksum",
		"scriptchecksum",
		"active dashboard login identifier is invalid",
		"business tenant is missing or inactive",
		"business superadmin is inactive",
		"cross-tenant historical relation",
		"active dashboard user is missing identity",
		"created_by_saas_user_id",
		"mochat_go_saas_admin_audit_anchor_checkpoints",
		"mochat_go_saas_admin_audit_verifications",
		"mochat_go_saas_admin_health_scans",
		"mochat_go_saas_admin_tasks",
		"legacy actor id",
		"platform_tenant_id",
		"status = 'validated'",
		"column_type",
		"unique key",
		"corp_count > 1",
		"one-corp",
		"field-by-field",
		"same numeric id",
		"preflight",
		"duplicate dashboard login identifier",
		"multiple valid corp",
		"unmapped saas actor",
		"identity conflict",
		"business tenant superadmin",
		"saas platform identity phone/login conflict",
		"unknown actor column",
		"identity actor inventory",
		"actor_inventory_status",
		"migration ledger success",
		"completed batch",
		"signal sqlstate ''45000''",
		"prepare",
		"execute",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0130 up migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"@identity_mapping_verified",
		"@identity_platform_tenant_id",
		"insert ignore",
		"mochat_go_identity_legacy_actor_map",
		"legacy_actor_to_saas",
		"add column identity_migration_source",
	} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("0130 up migration contains unsafe shortcut %q", forbidden)
		}
	}
	for _, required := range []string{
		"drop foreign key fk_saas_admin_user_access_identity",
		"drop foreign key fk_saas_admin_user_roles_identity",
		"drop table mochat_go_identity_migration_corp_map",
		"drop table mochat_go_identity_migration_batches",
		"drop table mochat_go_identity_migration_ledger",
		"drop table mochat_go_identity_migration_journal",
		"migration_name = ''0130_identity_realms_single_corp_backfill''",
		"information_schema.columns",
		"writtenkeyid",
		"afterciphertextsha256",
		"rollback credential journal verification failed",
		"delete_credential_journal",
		"rollback request does not own all metadata",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("0130 down migration missing %q", required)
		}
	}
	firstDDL := firstTask8DDL(up)
	if firstDDL < 0 {
		t.Fatal("0130 up migration has no DDL")
	}
	for _, preflight := range []string{
		"duplicate dashboard login identifier",
		"multiple valid corp",
		"unmapped saas actor",
		"negative tenant",
		"unreadable credential",
	} {
		offset := strings.Index(up, preflight)
		if offset < 0 || offset > firstDDL {
			t.Fatalf("preflight %q must precede first DDL", preflight)
		}
	}
	if strings.Contains(strings.ToUpper(string(upBody)), "DELIMITER") || strings.Contains(strings.ToUpper(string(upBody)), "CREATE PROCEDURE") {
		t.Fatal("0130 migration must not use DELIMITER or stored procedures")
	}
}

func firstTask8DDL(sql string) int {
	positions := []int{}
	for _, marker := range []string{"create table", "alter table", "create index", "drop index"} {
		if index := strings.Index(sql, marker); index >= 0 {
			positions = append(positions, index)
		}
	}
	if len(positions) == 0 {
		return -1
	}
	first := positions[0]
	for _, position := range positions[1:] {
		if position < first {
			first = position
		}
	}
	return first
}
