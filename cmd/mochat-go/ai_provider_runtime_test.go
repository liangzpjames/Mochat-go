package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/modules/providers/catalog"
)

func TestBuildAIProviderPrefersProtectedKeyFile(t *testing.T) {
	const fileSecret = "file-secret-material"
	const environmentSecret = "environment-secret-material"
	dir := t.TempDir()
	path := filepath.Join(dir, "ai-provider-key")
	if err := os.WriteFile(path, []byte("  "+fileSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY_FILE", path)
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", environmentSecret)
	t.Setenv("MOCHAT_GO_AI_PROVIDER_MODEL", "test-model")

	provider, err := buildAIProvider()
	if err != nil {
		t.Fatal(err)
	}
	status := provider.Status()
	if status.State != providers.StateReady {
		t.Fatalf("status=%#v, protected file key must make provider ready", status)
	}
	encoded, _ := json.Marshal(status)
	if containsSecret(string(encoded), fileSecret) || containsSecret(string(encoded), environmentSecret) {
		t.Fatal("provider status leaked secret material")
	}
}

func TestBuildAIProviderLimitsUnavailableConfiguredKeyFileWithoutEnvFallback(t *testing.T) {
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", "must-not-be-used")

	provider, err := buildAIProvider()
	if err != nil {
		t.Fatal(err)
	}
	if status := provider.Status(); status.State != providers.StateLimited {
		t.Fatalf("status=%#v, configured missing key file must fail closed as limited", status)
	}
}

func TestBuildAIProviderLimitsEmptyConfiguredKeyFileWithoutEnvFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(path, []byte(" \r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY_FILE", path)
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", "must-not-be-used")

	provider, err := buildAIProvider()
	if err != nil {
		t.Fatal(err)
	}
	if status := provider.Status(); status.State != providers.StateLimited {
		t.Fatalf("status=%#v, configured empty key file must fail closed as limited", status)
	}
}

func TestDashboardAIStatusProviderUsesEnabledRuntime(t *testing.T) {
	const secret = "composition-test-secret"
	t.Setenv("MOCHAT_GO_AI_PROVIDER_BASE_URL", "http://127.0.0.1:9/v1")
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", secret)
	t.Setenv("MOCHAT_GO_AI_PROVIDER_MODEL", "test-model")

	runtime, err := buildDashboardAIStatusProvider(config.Config{EnableAIDebtClearance: true, EnableAIInsight: true})
	if err != nil {
		t.Fatal(err)
	}
	status := runtime.Status()
	if status.State != providers.StateReady {
		t.Fatalf("status=%#v, enabled runtime with key must be ready", status)
	}
	registry, err := catalog.NewRegistry(catalog.Dependencies{AI: runtime, AIEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, registered := range registry.Snapshot(nil) {
		if registered.Kind == "ai" && registered.Source != providers.SourceExternal {
			t.Fatalf("enabled AI source=%q, want external", registered.Source)
		}
	}
	encoded, _ := json.Marshal(status)
	if string(encoded) == "" || string(encoded) == secret || containsSecret(string(encoded), secret) {
		t.Fatalf("status leaked AI key: %s", encoded)
	}
}

func TestDashboardAIStatusProviderDisabledIsCodeOnlyEvenWhenKeyExists(t *testing.T) {
	const secret = "disabled-composition-secret"
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", secret)

	runtime, err := buildDashboardAIStatusProvider(config.Config{EnableAIInsight: false})
	if err != nil {
		t.Fatal(err)
	}
	status := runtime.Status()
	if status.State != providers.StateLimited || status.Code != "ai.disabled" || status.Source != providers.SourceCodeOnly {
		t.Fatalf("status=%#v, disabled AI must be explicit code-only limited", status)
	}
	registry, err := catalog.NewRegistry(catalog.Dependencies{AI: runtime, AIEnabled: false})
	if err != nil {
		t.Fatal(err)
	}
	for _, registered := range registry.Snapshot(nil) {
		if registered.Kind == "ai" && (registered.Source != providers.SourceCodeOnly || registered.Code != "ai.disabled") {
			t.Fatalf("disabled AI registration=%#v, want code-only disabled", registered)
		}
	}
	encoded, _ := json.Marshal(status)
	if containsSecret(string(encoded), secret) {
		t.Fatalf("status leaked disabled AI key: %s", encoded)
	}
}

func TestDashboardAIStatusProviderRequiresBothAIFlags(t *testing.T) {
	const secret = "dual-flag-composition-secret"
	t.Setenv("MOCHAT_GO_AI_PROVIDER_BASE_URL", "http://127.0.0.1:9/v1")
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", secret)
	t.Setenv("MOCHAT_GO_AI_PROVIDER_MODEL", "test-model")

	cases := []struct {
		name       string
		debt       bool
		insight    bool
		wantState  providers.State
		wantSource providers.Source
	}{
		{name: "debt disabled insight disabled", debt: false, insight: false, wantState: providers.StateLimited, wantSource: providers.SourceCodeOnly},
		{name: "debt disabled insight enabled", debt: false, insight: true, wantState: providers.StateLimited, wantSource: providers.SourceCodeOnly},
		{name: "debt enabled insight disabled", debt: true, insight: false, wantState: providers.StateLimited, wantSource: providers.SourceCodeOnly},
		{name: "both enabled", debt: true, insight: true, wantState: providers.StateReady, wantSource: providers.SourceExternal},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			runtime, err := buildDashboardAIStatusProvider(config.Config{EnableAIDebtClearance: test.debt, EnableAIInsight: test.insight})
			if err != nil {
				t.Fatal(err)
			}
			status := runtime.Status()
			if status.State != test.wantState {
				t.Fatalf("status=%#v, want state=%q", status, test.wantState)
			}
			registry, err := catalog.NewRegistry(catalog.Dependencies{AI: runtime, AIEnabled: test.debt && test.insight})
			if err != nil {
				t.Fatal(err)
			}
			registered := findRegisteredProvider(registry.Snapshot(nil), "ai")
			if registered.Source != test.wantSource || registered.State != test.wantState {
				t.Fatalf("registered=%#v, want state=%q source=%q", registered, test.wantState, test.wantSource)
			}
			encoded, err := json.Marshal(status)
			if err != nil {
				t.Fatal(err)
			}
			if containsSecret(string(encoded), secret) {
				t.Fatal("AI key leaked into status")
			}
		})
	}
}

func findRegisteredProvider(statuses []providers.Status, kind string) providers.Status {
	for _, status := range statuses {
		if status.Kind == kind {
			return status
		}
	}
	return providers.Status{}
}

func containsSecret(value, secret string) bool {
	return secret != "" && len(value) > 0 && len(secret) > 0 && strings.Contains(value, secret)
}
