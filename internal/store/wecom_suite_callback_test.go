package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestRotateSuiteAuthorizationCredentialDoesNotRequireRetiredKey(t *testing.T) {
	oldManager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey: strings.Repeat("31", 32), EncryptionKeyID: "retired", RequireEncryption: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	oldCiphertext, oldKeyID, err := oldManager.EncryptAuthorization(4, "integration-4", wecomcredentials.AuthorizationCredential{
		Mode: "third_party_delegated", ProviderAppID: "suite-old", PermanentCode: "permanent-old",
	})
	if err != nil {
		t.Fatal(err)
	}
	activeManager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey: strings.Repeat("32", 32), EncryptionKeyID: "active", RequireEncryption: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	ciphertext, keyID, err := rotateSuiteAuthorizationCredential(activeManager, 4, "integration-4", oldCiphertext, oldKeyID, "suite-current", "permanent-current")
	if err != nil {
		t.Fatalf("reauthorization with retired prior key: %v", err)
	}
	credential, err := activeManager.DecryptAuthorization(4, "integration-4", keyID, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Mode != "third_party_delegated" || credential.ProviderAppID != "suite-current" || credential.PermanentCode != "permanent-current" {
		t.Fatalf("rotated credential=%#v", credential)
	}
	if credential.EmployeeSecret != "" || credential.ContactSecret != "" || credential.AgentSecret != "" || credential.ChatSecret != "" {
		t.Fatalf("delegated authorization retained self-built secrets: %#v", credential)
	}
}
