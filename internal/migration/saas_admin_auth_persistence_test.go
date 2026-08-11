package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaaSAdminAuthPersistenceMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	upBytes, err := os.ReadFile(filepath.Join(root, "0129_identity_realms_single_corp_schema.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	downBytes, err := os.ReadFile(filepath.Join(root, "0129_identity_realms_single_corp_schema.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up := strings.ToLower(strings.ReplaceAll(string(upBytes), "`", ""))
	down := strings.ToLower(strings.ReplaceAll(string(downBytes), "`", ""))

	for _, table := range []string{
		"mochat_go_saas_admin_mfa_credentials",
		"mochat_go_saas_admin_mfa_challenges",
		"mochat_go_saas_admin_sessions",
	} {
		if !strings.Contains(up, "create table if not exists "+table) {
			t.Fatalf("0129 up migration missing %s", table)
		}
		if !strings.Contains(down, "drop table if exists "+table) {
			t.Fatalf("0129 down migration missing %s", table)
		}
	}
	for _, required := range []string{
		"token_digest binary(32)",
		"jti_digest binary(32)",
		"secret_ciphertext longtext",
		"encryption_key_id",
		"auth_version",
		"expires_at",
		"consumed_at",
		"revoked_at",
		"unique key uni_saas_admin_mfa_challenge_digest",
		"unique key uni_saas_admin_session_jti_digest",
		"fk_saas_admin_mfa_user",
		"fk_saas_admin_mfa_challenge_user",
		"fk_saas_admin_session_user",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0129 up migration missing %q", required)
		}
	}
	if strings.Contains(up, "raw_token") {
		t.Fatal("0129 SaaS auth migration must persist digests/ciphertext, never raw tokens")
	}
	for _, preflight := range []string{"information_schema.tables", "signal sqlstate ''45000''", "prepare", "execute"} {
		if !strings.Contains(up, preflight) {
			t.Fatalf("0129 migration preflight contract missing %q", preflight)
		}
	}
	if strings.Index(down, "drop table if exists mochat_go_saas_admin_sessions") > strings.Index(down, "drop table if exists mochat_go_saas_admin_mfa_challenges") {
		t.Fatal("0129 down migration must drop sessions before challenges")
	}
	if strings.Index(down, "drop table if exists mochat_go_saas_admin_mfa_challenges") > strings.Index(down, "drop table if exists mochat_go_saas_admin_mfa_credentials") {
		t.Fatal("0129 down migration must drop challenges before credentials")
	}
}
