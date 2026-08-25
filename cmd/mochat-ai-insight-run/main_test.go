package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRuntimeConfigRequiresProtectedKeyFile(t *testing.T) {
	getenv := func(name string) string {
		values := map[string]string{
			"MOCHAT_MYSQL_DSN":               "user:pass@tcp(db:3306)/mochat?parseTime=true",
			"MOCHAT_GO_AI_PROVIDER_BASE_URL": "https://api.deepseek.com",
			"MOCHAT_GO_AI_PROVIDER_MODEL":    "deepseek-v4-flash",
		}
		return values[name]
	}
	if _, err := loadRuntimeConfig(getenv); err == nil {
		t.Fatal("loadRuntimeConfig error=nil, want protected key file requirement")
	}
}

func TestLoadRuntimeConfigReadsKeyWithoutExposingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.key")
	if err := os.WriteFile(path, []byte("secret-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	getenv := func(name string) string {
		values := map[string]string{
			"MOCHAT_MYSQL_DSN":                      "user:pass@tcp(db:3306)/mochat?parseTime=true",
			"MOCHAT_GO_AI_PROVIDER_KEY_FILE":        path,
			"MOCHAT_GO_AI_PROVIDER_BASE_URL":        "https://api.deepseek.com/",
			"MOCHAT_GO_AI_PROVIDER_MODEL":           "deepseek-v4-flash",
			"MOCHAT_GO_AI_PROVIDER_TIMEOUT_SECONDS": "90",
			"MOCHAT_GO_AI_RUN_TIMEOUT_MINUTES":      "20",
		}
		return values[name]
	}
	config, err := loadRuntimeConfig(getenv)
	if err != nil {
		t.Fatal(err)
	}
	if config.APIKey != "secret-value" || config.BaseURL != "https://api.deepseek.com/" || config.Model != "deepseek-v4-flash" {
		t.Fatalf("unexpected runtime config metadata: base=%q model=%q keyLoaded=%t", config.BaseURL, config.Model, config.APIKey != "")
	}
	if config.ProviderTimeout.Seconds() != 90 || config.RunTimeout.Minutes() != 20 {
		t.Fatalf("unexpected timeouts: provider=%s run=%s", config.ProviderTimeout, config.RunTimeout)
	}
}
