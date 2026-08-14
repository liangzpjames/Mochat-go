package main

import (
	"encoding/json"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/modules/providers/catalog"
)

func TestDashboardAIStatusProviderUsesEnabledRuntime(t *testing.T) {
	const secret = "composition-test-secret"
	t.Setenv("MOCHAT_GO_AI_PROVIDER_BASE_URL", "http://127.0.0.1:9/v1")
	t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", secret)
	t.Setenv("MOCHAT_GO_AI_PROVIDER_MODEL", "test-model")

	runtime, err := buildDashboardAIStatusProvider(config.Config{EnableAIInsight: true})
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

func containsSecret(value, secret string) bool {
	return secret != "" && len(value) > 0 && len(secret) > 0 && strings.Contains(value, secret)
}
