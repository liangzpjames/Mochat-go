package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaaSMFAEncryptionKeyUsesProtectedSecretFile(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	composeText := string(compose)
	if !strings.Contains(composeText, `file: "${MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE:?`) {
		t.Fatal("Compose must require the SaaS MFA encryption secret file")
	}
	if !strings.Contains(composeText, `MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE: "/run/secrets/`) {
		t.Fatal("app must receive only the mounted SaaS MFA secret-file path")
	}
	if strings.Contains(composeText, "MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY: \"${") {
		t.Fatal("Compose must not inject the SaaS MFA key as a value environment variable")
	}
	if !strings.Contains(composeText, `MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID: "${MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID:?`) {
		t.Fatal("Compose must require the SaaS MFA key id without a default")
	}

	envExample, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	envText := string(envExample)
	if !strings.Contains(envText, "MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE=") || !strings.Contains(envText, "MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_ID=") {
		t.Fatal(".env.example must document the protected SaaS MFA file and key id")
	}
	if strings.Contains(envText, "MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY=CHANGE_ME") {
		t.Fatal(".env.example must not document an inline SaaS MFA secret")
	}
}

func TestReadSecretFilePrefersFileContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saas-mfa-key")
	if err := os.WriteFile(path, []byte("file-only-test-key\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE", path)
	t.Setenv("MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY", "must-not-be-read")
	value, err := readSecretFile("MOCHAT_SAAS_ADMIN_MFA_ENCRYPTION_KEY_FILE")
	if err != nil {
		t.Fatal(err)
	}
	if value != "file-only-test-key" {
		t.Fatalf("secret file value = %q", value)
	}
}

func TestIdentityRealmValidationRequiresSaaSMFAFileAndKeyID(t *testing.T) {
	cfg := Config{
		EnableSaaSAdminDashboard: true,
		SaaSAdminJWTSecret:       "saas-jwt-secret", SaaSAdminJWTIssuer: "saas-issuer", SaaSAdminJWTAudience: "saas-audience", SaaSAdminJWTPrefix: "saas_", SaaSAdminJWTTTL: 1,
		DashboardJWTSecret: "dashboard-jwt-secret", DashboardJWTIssuer: "dashboard-issuer", DashboardJWTAudience: "dashboard-audience", DashboardJWTPrefix: "dashboard_", DashboardJWTTTL: 1,
	}
	if err := cfg.ValidateIdentityRealms(); err == nil || !strings.Contains(err.Error(), "MFA_ENCRYPTION_KEY_FILE") {
		t.Fatalf("missing SaaS MFA file was accepted: %v", err)
	}
	cfg.SaaSAdminMFAEncryptionKey = "file-loaded-key"
	if err := cfg.ValidateIdentityRealms(); err == nil || !strings.Contains(err.Error(), "MFA_ENCRYPTION_KEY_ID") {
		t.Fatalf("missing SaaS MFA key id was accepted: %v", err)
	}
}

func TestIdentityRealmValidationRequiresDashboardMFAFileAndKeyID(t *testing.T) {
	cfg := Config{
		MigrateAuth:        true,
		SaaSAdminJWTSecret: "saas-jwt-secret", SaaSAdminJWTIssuer: "saas-issuer", SaaSAdminJWTAudience: "saas-audience", SaaSAdminJWTPrefix: "saas_", SaaSAdminJWTTTL: 1,
		DashboardJWTSecret: "dashboard-jwt-secret", DashboardJWTIssuer: "dashboard-issuer", DashboardJWTAudience: "dashboard-audience", DashboardJWTPrefix: "dashboard_", DashboardJWTTTL: 1,
	}
	if err := cfg.ValidateIdentityRealms(); err == nil || !strings.Contains(err.Error(), "MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE") {
		t.Fatalf("missing Dashboard MFA file was accepted: %v", err)
	}
	cfg.DashboardMFAEncryptionKey = "dashboard-file-loaded-key"
	if err := cfg.ValidateIdentityRealms(); err == nil || !strings.Contains(err.Error(), "MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID") {
		t.Fatalf("missing Dashboard MFA key id was accepted: %v", err)
	}
}

func TestDashboardMFAUsesProtectedSecretFileInDeploymentContract(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	composeText := string(compose)
	if !strings.Contains(composeText, `file: "${MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE:?`) || !strings.Contains(composeText, `MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE: "/run/secrets/`) {
		t.Fatal("Compose must require and pass the Dashboard MFA secret-file path")
	}
	if !strings.Contains(composeText, `MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID: "${MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID:?`) {
		t.Fatal("Compose must require the Dashboard MFA key id without a default")
	}
	envExample, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(envExample), "MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_FILE=") || !strings.Contains(string(envExample), "MOCHAT_DASHBOARD_MFA_ENCRYPTION_KEY_ID=") {
		t.Fatal(".env.example must document the protected Dashboard MFA file and key id")
	}
}
