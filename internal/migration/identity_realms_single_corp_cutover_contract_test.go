package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityRealmsSingleCorpCutoverMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	upPath := filepath.Join(root, "deploy", "standalone", "migrations", "0131_identity_realms_single_corp_cutover.up.sql")
	downPath := filepath.Join(root, "deploy", "standalone", "migrations", "0131_identity_realms_single_corp_cutover.down.sql")
	upBody, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatal(err)
	}
	up := strings.ToLower(strings.ReplaceAll(string(upBody), "`", ""))
	down := strings.ToLower(strings.ReplaceAll(string(downBody), "`", ""))

	for _, required := range []string{
		"preflight",
		"0129_identity_realms_single_corp_schema",
		"0130_identity_realms_single_corp_backfill",
		"mochat_go_identity_migration_ledger",
		"mochat_go_identity_migration_batches",
		"mochat_go_dashboard_identities",
		"mochat_go_tenant_corp_bindings",
		"wecom_credentials_ciphertext",
		"mc_user",
		"password",
		"cutover journal",
		"signal sqlstate ''45000''",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0131 up migration missing %q", required)
		}
	}
	for _, required := range []string{
		"information_schema.columns",
		"restore",
		"mc_user",
		"password",
		"mochat_go_identity_cutover_journal",
		"signal sqlstate ''45000''",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("0131 down migration missing %q", required)
		}
	}
	if !strings.Contains(down, "delete from mochat_go_identity_migration_ledger") {
		t.Fatal("0131 down must remove the applied migration ledger while preserving independent cutover restore evidence")
	}
	for _, required := range []string{
		"update mochat_go_dashboard_permissions",
		"restriction = 'grantable'",
		"superadmin_only = 0",
		"set status = 'rolled_back'",
		"delete resource",
		"mochat_go_dashboard_permission_resources resource",
		"/dashboard/corp/index",
		"/dashboard/corp/show",
		"/dashboard/corp/store",
		"/dashboard/corp/update",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("0131 down must restore the released 0127 company permission/resource mapping: missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"drop table mochat_go_identity_cutover_journal",
		"delete from mochat_go_identity_cutover_journal",
		"drop table mochat_go_identity_cutover_batches",
		"delete from mochat_go_identity_cutover_batches",
	} {
		if strings.Contains(down, forbidden) {
			t.Fatalf("0131 down must retain cutover restore evidence: forbidden %q", forbidden)
		}
	}
	for _, forbidden := range []string{
		"add column if not exists",
		"insert ignore",
		"create procedure",
		"delimiter",
	} {
		if strings.Contains(up, forbidden) || strings.Contains(down, forbidden) {
			t.Fatalf("0131 migration contains forbidden shortcut %q", forbidden)
		}
	}
	firstDDL := firstCutoverDDL(up)
	if firstDDL < 0 {
		t.Fatal("0131 up migration has no DDL")
	}
	for _, preflight := range []string{
		"0129_identity_realms_single_corp_schema",
		"0130_identity_realms_single_corp_backfill",
		"active user is missing dashboard identity",
		"active tenant does not have exactly one binding",
		"undecryptable credential",
		"cross-tenant",
		"dangling",
	} {
		offset := strings.Index(up, preflight)
		if offset < 0 || offset > firstDDL {
			t.Fatalf("0131 preflight %q must precede first DDL", preflight)
		}
	}
	if !strings.Contains(up, "resume_batch_status in ('started', 'completed', 'rolled_back', 'restored')") {
		t.Fatal("0131 up must allow a previously rolled-back cutover request to execute again")
	}
	if !strings.Contains(up, "status = 'started'") {
		t.Fatal("0131 up must reset an existing rollback batch before reapplying")
	}
}

func TestDefaultMigrationsRegistersCutoverAsControlled(t *testing.T) {
	root := filepath.Join("..", "..")
	var found Migration
	for _, candidate := range DefaultMigrations(root) {
		if candidate.Version == "0131_identity_realms_single_corp_cutover" {
			found = candidate
			break
		}
	}
	if found.Version == "" {
		t.Fatal("0131 cutover is missing from DefaultMigrations")
	}
	if found.Kind != MigrationControlled || found.Controlled == nil {
		t.Fatalf("0131 metadata=%+v, want controlled migration", found)
	}
	if found.Controlled.RequiredCLI != "mochat-identity-migrate" {
		t.Fatalf("0131 required cli=%q", found.Controlled.RequiredCLI)
	}
}

func firstCutoverDDL(sqlText string) int {
	positions := make([]int, 0, 4)
	for _, marker := range []string{"create table", "alter table", "drop table", "create index", "drop index"} {
		if index := strings.Index(sqlText, marker); index >= 0 {
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
