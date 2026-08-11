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
		"secrets:",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("long-running Compose still exposes bootstrap secret dependency: %s", forbidden)
		}
	}
}
