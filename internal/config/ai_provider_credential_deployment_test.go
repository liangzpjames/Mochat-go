package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandaloneDeploymentForwardsTenantAIProviderCredentialProtection(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone")
	compose, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	envExample, err := os.ReadFile(filepath.Join(root, ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"MOCHAT_GO_AI_PROVIDER_CREDENTIAL_ENCRYPTION_KEY",
		"MOCHAT_GO_AI_PROVIDER_CREDENTIAL_ENCRYPTION_KEYS",
		"MOCHAT_GO_AI_PROVIDER_CREDENTIAL_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_AI_PROVIDER_CREDENTIAL_REQUIRE_ENCRYPTION",
	} {
		if !strings.Contains(string(compose), name+": \"${"+name) {
			t.Fatalf("standalone compose does not forward %s", name)
		}
		if !strings.Contains(string(envExample), name+"=") {
			t.Fatalf("standalone .env.example does not document %s", name)
		}
	}
}
