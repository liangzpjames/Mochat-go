package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapSaaSAdminReadsOnlyRestrictedPasswordFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "password")
	password := "bootstrap-file-password-not-output"
	if err := os.WriteFile(path, []byte(password+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := setBootstrapPasswordFilePermissions(path, true); err != nil {
		t.Fatal(err)
	}
	got, err := readBootstrapSaaSAdminPassword(path)
	if err != nil || got != password {
		t.Fatal("restricted password file was not read correctly")
	}

	if err := setBootstrapPasswordFilePermissions(path, false); err != nil {
		t.Fatal(err)
	}
	if _, err := readBootstrapSaaSAdminPassword(path); err == nil {
		t.Fatal("world-readable password file was accepted")
	}

	if err := os.WriteFile(path, []byte("\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := setBootstrapPasswordFilePermissions(path, true); err != nil {
		t.Fatal(err)
	}
	if _, err := readBootstrapSaaSAdminPassword(path); err == nil {
		t.Fatal("empty password file was accepted")
	}
}

func TestBootstrapSaaSAdminPasswordFileErrorsNeverExposePasswordContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "password")
	password := "bootstrap-file-password-not-output"
	if err := os.WriteFile(path, []byte(password), 0644); err != nil {
		t.Fatal(err)
	}
	if err := setBootstrapPasswordFilePermissions(path, false); err != nil {
		t.Fatal(err)
	}
	_, err := readBootstrapSaaSAdminPassword(path)
	if err == nil || strings.Contains(err.Error(), password) {
		t.Fatal("password file error exposed password material")
	}
}

func TestBootstrapSaaSAdminSourceHasNoLegacySecretOrBusinessProvisioningPath(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, forbidden := range []string{
		"-secret",
		"MOCHAT_SIMPLE_JWT_SECRET",
		"MOCHAT_BOOTSTRAP_PASSWORD",
		"mc_user",
		"mc_tenant",
		"mc_corp",
		"mochat_go_dashboard_identities",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("bootstrap source still contains forbidden legacy path: %s", forbidden)
		}
	}
}

func TestLongRunningComposeDoesNotExposeBootstrapPassword(t *testing.T) {
	path := filepath.Join("..", "..", "deploy", "standalone", "docker-compose.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, forbidden := range []string{
		"MOCHAT_BOOTSTRAP_SAAS_ADMIN_PASSWORD_FILE",
		"mochat_bootstrap_saas_admin_password",
		"BOOTSTRAP_CONTAINER_FILE",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("long-running Compose still exposes bootstrap secret dependency: %s", forbidden)
		}
	}
}

func TestBootstrapProductionEntrypointsDoNotInvokeLegacyTenantBootstrap(t *testing.T) {
	root := filepath.Join("..", "..")
	deploySource, err := os.ReadFile(filepath.Join(root, "scripts", "deploy_docker_desktop.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"mochat-bootstrap",
		"AdminPassword",
		"AdminPhone",
		"'-secret'",
		`"-secret"`,
		" -secret ",
		"'-password'",
		`"-password"`,
		" -password ",
	} {
		if strings.Contains(string(deploySource), forbidden) {
			t.Fatalf("production deploy entrypoint still contains legacy bootstrap material: %s", forbidden)
		}
	}

	for _, relativePath := range []string{
		filepath.Join("scripts", "standalone_acceptance.sh"),
		filepath.Join("scripts", "audit_functional_module_matrix.sh"),
	} {
		raw, readErr := os.ReadFile(filepath.Join(root, relativePath))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(raw), "smoke_bootstrap_standalone.sh") || strings.Contains(string(raw), "smoke_standalone_compose_app.sh") {
			t.Fatalf("production smoke registry still references removed legacy script: %s", relativePath)
		}
	}

	legacySmoke := filepath.Join(root, "scripts", "smoke_bootstrap_standalone.sh")
	if _, statErr := os.Stat(legacySmoke); !os.IsNotExist(statErr) {
		t.Fatalf("legacy standalone bootstrap smoke must be removed, stat error: %v", statErr)
	}
}

func TestBootstrapReadmeUsesUniqueTemporaryPasswordFileAndTruthfulContract(t *testing.T) {
	path := filepath.Join("..", "..", "deploy", "standalone", "README.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	readme := string(raw)
	for _, forbidden := range []string{
		"smoke_bootstrap_standalone.sh",
		"创建默认租户、超级管理员",
		`BOOTSTRAP_CONTAINER_FILE="/tmp/mochat-bootstrap-saas-admin-password"`,
	} {
		if strings.Contains(readme, forbidden) {
			t.Fatalf("standalone README still contains stale bootstrap contract: %s", forbidden)
		}
	}
	for _, required := range []string{
		"mktemp /tmp/mochat-bootstrap-saas-admin.XXXXXX",
		"mochat_go_saas_admin_users",
		"identity-single-corp",
	} {
		if !strings.Contains(readme, required) {
			t.Fatalf("standalone README is missing bootstrap contract marker: %s", required)
		}
	}
}
