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

func TestRealmMFARequiredConfigDefaultsOffAndParsesStrictly(t *testing.T) {
	cases := []struct {
		name           string
		saasValue      string
		dashboardValue string
		wantSaaS       bool
		wantDashboard  bool
		wantError      bool
	}{
		{name: "default", wantSaaS: false, wantDashboard: false},
		{name: "explicit zero", saasValue: "0", dashboardValue: "0", wantSaaS: false, wantDashboard: false},
		{name: "explicit one", saasValue: "1", dashboardValue: "1", wantSaaS: true, wantDashboard: true},
		{name: "SaaS only", saasValue: "1", dashboardValue: "0", wantSaaS: true, wantDashboard: false},
		{name: "Dashboard only", saasValue: "0", dashboardValue: "1", wantSaaS: false, wantDashboard: true},
		{name: "invalid SaaS value", saasValue: "maybe", wantError: true},
		{name: "invalid Dashboard value", dashboardValue: "maybe", wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			if tc.saasValue != "" {
				t.Setenv("MOCHAT_SAAS_ADMIN_MFA_REQUIRED", tc.saasValue)
			}
			if tc.dashboardValue != "" {
				t.Setenv("MOCHAT_DASHBOARD_MFA_REQUIRED", tc.dashboardValue)
			}
			cfg, err := FromEnv()
			if tc.wantError {
				if err == nil {
					t.Fatal("invalid MFA requirement value was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.SaaSAdminMFARequired != tc.wantSaaS || cfg.DashboardMFARequired != tc.wantDashboard {
				t.Fatalf("MFA requirements = SaaS %t Dashboard %t", cfg.SaaSAdminMFARequired, cfg.DashboardMFARequired)
			}
		})
	}
}

func TestStandaloneComposeDocumentsIndependentMFARequirementDefaults(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	composeText := string(compose)
	for _, expression := range []string{
		`MOCHAT_SAAS_ADMIN_MFA_REQUIRED: "${MOCHAT_SAAS_ADMIN_MFA_REQUIRED:-0}"`,
		`MOCHAT_DASHBOARD_MFA_REQUIRED: "${MOCHAT_DASHBOARD_MFA_REQUIRED:-0}"`,
	} {
		if !strings.Contains(composeText, expression) {
			t.Fatalf("Compose is missing independent MFA default %q", expression)
		}
	}
}

func TestIdentityRealmValidationRejectsSharedMFAKeyOrKeyID(t *testing.T) {
	base := Config{
		EnableSaaSAdminDashboard: true,
		MigrateAuth:              true,
		SaaSAdminJWTSecret:       "saas-jwt-secret", SaaSAdminJWTIssuer: "saas-issuer", SaaSAdminJWTAudience: "saas-audience", SaaSAdminJWTPrefix: "saas_", SaaSAdminJWTTTL: 1,
		DashboardJWTSecret: "dashboard-jwt-secret", DashboardJWTIssuer: "dashboard-issuer", DashboardJWTAudience: "dashboard-audience", DashboardJWTPrefix: "dashboard_", DashboardJWTTTL: 1,
		SaaSAdminMFAEncryptionKey: "same-mfa-key", SaaSAdminMFAEncryptionKeyID: "saas-mfa",
		DashboardMFAEncryptionKey: "same-mfa-key", DashboardMFAEncryptionKeyID: "dashboard-mfa",
	}
	if err := base.ValidateIdentityRealms(); err == nil || strings.Contains(err.Error(), "same-mfa-key") {
		t.Fatalf("shared MFA key was accepted or leaked in error: errorPresent=%t", err != nil)
	}

	base.SaaSAdminMFAEncryptionKey = "saas-mfa-key"
	base.DashboardMFAEncryptionKey = "dashboard-mfa-key"
	base.SaaSAdminMFAEncryptionKeyID = "shared-mfa-id"
	base.DashboardMFAEncryptionKeyID = "shared-mfa-id"
	if err := base.ValidateIdentityRealms(); err == nil || strings.Contains(err.Error(), "shared-mfa-id") {
		t.Fatalf("shared MFA key ID was accepted or leaked in error: errorPresent=%t", err != nil)
	}
}
