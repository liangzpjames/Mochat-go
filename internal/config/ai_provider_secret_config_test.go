package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIProviderComposeOverrideUsesProtectedSecretFile(t *testing.T) {
	path := filepath.Join("..", "..", "deploy", "standalone", "docker-compose.ai.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{
		`MOCHAT_GO_AI_PROVIDER_KEY_FILE`,
		`/run/secrets/mochat-ai-provider-key`,
		`MOCHAT_GO_AI_PROVIDER_KEY: ""`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("AI compose override missing %q", required)
		}
	}
	if strings.Contains(text, `MOCHAT_GO_AI_PROVIDER_KEY: "${`) {
		t.Fatal("AI compose override must not inject key material into container environment")
	}
}
