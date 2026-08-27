package wecomcredentials

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestManagerEncryptsCorpAndAgentCredentialsWithBoundContext(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(1), EncryptionKeyID: "wecom-v1", RequireEncryption: true, DedicatedConfigured: true})
	if err != nil {
		t.Fatal(err)
	}
	corpCiphertext, keyID, err := manager.EncryptCorp(7, "ww-corp", CorpCredential{
		EmployeeSecret: "employee", ContactSecret: "contact", CallbackToken: "token", EncodingAESKey: "aes", ChatSecret: "archive",
		ArchiveRSAPublicKey: "public-pem", ArchiveRSAPrivateKey: "private-pem",
	})
	if err != nil {
		t.Fatal(err)
	}
	if keyID != "wecom-v1" || strings.Contains(corpCiphertext, "employee") || strings.Contains(corpCiphertext, "archive") {
		t.Fatalf("corp ciphertext leaked plaintext or key mismatch: key=%q value=%q", keyID, corpCiphertext)
	}
	corp, err := manager.DecryptCorp(7, "ww-corp", keyID, corpCiphertext)
	if err != nil || corp.ContactSecret != "contact" || corp.ChatSecret != "archive" || corp.ArchiveRSAPublicKey != "public-pem" || corp.ArchiveRSAPrivateKey != "private-pem" {
		t.Fatalf("corp credential=%+v err=%v", corp, err)
	}
	if _, err := manager.DecryptCorp(8, "ww-corp", keyID, corpCiphertext); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("cross-tenant decrypt error=%v", err)
	}
	if _, err := manager.DecryptCorp(7, "ww-other", keyID, corpCiphertext); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("cross-corp decrypt error=%v", err)
	}

	agentCiphertext, agentKeyID, err := manager.EncryptAgent(13, "100001", AgentCredential{WXSecret: "agent-secret"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := manager.DecryptAgent(13, "100001", agentKeyID, agentCiphertext)
	if err != nil || agent.WXSecret != "agent-secret" {
		t.Fatalf("agent credential=%+v err=%v", agent, err)
	}
	if _, err := manager.DecryptAgent(14, "100001", agentKeyID, agentCiphertext); err == nil {
		t.Fatal("expected cross-corp agent decrypt failure")
	}
}

func TestAuthorizationCredentialIsTenantAndIntegrationBound(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(3), EncryptionKeyID: "wecom-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}
	value := AuthorizationCredential{
		Mode:          "third_party_delegated",
		PermanentCode: "local-permanent-code",
		ProviderAppID: "provider-a",
	}
	ciphertext, keyID, err := manager.EncryptAuthorization(7, "integration-a", value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ciphertext, value.PermanentCode) {
		t.Fatal("authorization ciphertext contains plaintext")
	}
	got, err := manager.DecryptAuthorization(7, "integration-a", keyID, ciphertext)
	if err != nil || got != value {
		t.Fatalf("authorization round trip failed: got=%+v err=%v", got, err)
	}
	if _, err := manager.DecryptAuthorization(8, "integration-a", keyID, ciphertext); err == nil {
		t.Fatal("cross-tenant authorization decrypt succeeded")
	}
	if _, err := manager.DecryptAuthorization(7, "integration-b", keyID, ciphertext); err == nil {
		t.Fatal("cross-integration authorization decrypt succeeded")
	}
	if _, err := manager.DecryptAuthorization(7, "integration-a", "unknown", ciphertext); err == nil {
		t.Fatal("unknown-key authorization decrypt succeeded")
	}
}

func TestSuiteTicketCredentialIsEncryptedAndSuiteBound(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(6), EncryptionKeyID: "wecom-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, err := manager.EncryptSuiteTicket("suite-a", "sensitive-ticket")
	if err != nil || strings.Contains(ciphertext, "sensitive-ticket") {
		t.Fatalf("ciphertext=%q key=%q err=%v", ciphertext, keyID, err)
	}
	ticket, err := manager.DecryptSuiteTicket("suite-a", keyID, ciphertext)
	if err != nil || ticket != "sensitive-ticket" {
		t.Fatalf("ticket=%q err=%v", ticket, err)
	}
	if _, err := manager.DecryptSuiteTicket("suite-b", keyID, ciphertext); err == nil {
		t.Fatal("cross-suite ticket decrypt succeeded")
	}
}

func TestArchiveMediaCredentialIsTenantAndObjectBound(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(4), EncryptionKeyID: "wecom-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, err := manager.EncryptArchiveMedia(7, "014c1da7-1b2e-4aa1-90aa-a6a0d6f53380", ArchiveMediaCredential{SDKFileID: "private-sdk-file-id"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ciphertext, "private-sdk-file-id") {
		t.Fatal("archive media locator leaked in ciphertext envelope")
	}
	got, err := manager.DecryptArchiveMedia(7, "014c1da7-1b2e-4aa1-90aa-a6a0d6f53380", keyID, ciphertext)
	if err != nil || got.SDKFileID != "private-sdk-file-id" {
		t.Fatalf("credential=%#v err=%v", got, err)
	}
	if _, err := manager.DecryptArchiveMedia(8, "014c1da7-1b2e-4aa1-90aa-a6a0d6f53380", keyID, ciphertext); err == nil {
		t.Fatal("cross-tenant media locator decrypt unexpectedly succeeded")
	}
	if _, err := manager.DecryptArchiveMedia(7, "different-object", keyID, ciphertext); err == nil {
		t.Fatal("cross-object media locator decrypt unexpectedly succeeded")
	}
}

func TestArchiveComponentCredentialIsTenantAndObjectBound(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(5), EncryptionKeyID: "wecom-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}
	value := ArchiveComponentCredential{MessageID: "dz-msg-1", PublicKeyVersion: 1, EncryptedSecretKey: "wrapped-private-key"}
	ciphertext, keyID, err := manager.EncryptArchiveComponent(7, "component-object", value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ciphertext, value.EncryptedSecretKey) {
		t.Fatal("archive component locator leaked in ciphertext envelope")
	}
	got, err := manager.DecryptArchiveComponent(7, "component-object", keyID, ciphertext)
	if err != nil || got != value {
		t.Fatalf("component credential=%#v err=%v", got, err)
	}
	if _, err := manager.DecryptArchiveComponent(8, "component-object", keyID, ciphertext); err == nil {
		t.Fatal("cross-tenant component decrypt unexpectedly succeeded")
	}
	if _, err := manager.DecryptArchiveComponent(7, "other-object", keyID, ciphertext); err == nil {
		t.Fatal("cross-object component decrypt unexpectedly succeeded")
	}
}

func TestManagerReadsHistoricalKeyAndWritesActiveKey(t *testing.T) {
	ring := `{"wecom-v1":"` + testKey(1) + `","wecom-v2":"` + testKey(2) + `"}`
	oldManager, err := NewManager(Config{EncryptionKeys: ring, EncryptionKeyID: "wecom-v1"})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, oldKeyID, err := oldManager.EncryptCorp(9, "ww-one", CorpCredential{ContactSecret: "old"})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(Config{EncryptionKeys: ring, EncryptionKeyID: "wecom-v2"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.DecryptCorp(9, "ww-one", oldKeyID, ciphertext); err != nil {
		t.Fatal(err)
	}
	_, activeKeyID, err := manager.EncryptCorp(9, "ww-one", CorpCredential{ContactSecret: "new"})
	if err != nil || activeKeyID != "wecom-v2" {
		t.Fatalf("active key=%q err=%v", activeKeyID, err)
	}
}

func TestManagerConfigurationAndSourceSelection(t *testing.T) {
	if _, err := NewManager(Config{RequireEncryption: true}); err == nil {
		t.Fatal("expected required encryption key error")
	}
	if _, err := NewManager(Config{EncryptionKeys: `{"old":"` + testKey(1) + `"}`, EncryptionKeyID: "missing"}); err == nil {
		t.Fatal("expected missing active key error")
	}
	selected, dedicated := SelectEncryptionSource(
		EncryptionSource{Key: testKey(1), KeyID: "wecom-v1"},
		EncryptionSource{Keys: `{"identity-v1":"` + testKey(2) + `"}`, KeyID: "identity-v1"},
	)
	if !dedicated || selected.Key == "" || selected.Keys != "" || selected.KeyID != "wecom-v1" {
		t.Fatalf("dedicated source=%+v dedicated=%t", selected, dedicated)
	}
	selected, dedicated = SelectEncryptionSource(EncryptionSource{}, EncryptionSource{Keys: `{"identity-v1":"` + testKey(2) + `"}`, KeyID: "identity-v1"})
	if dedicated || selected.Key != "" || selected.Keys == "" || selected.KeyID != "identity-v1" {
		t.Fatalf("fallback source=%+v dedicated=%t", selected, dedicated)
	}
}

func testKey(value byte) string {
	key := make([]byte, 32)
	for index := range key {
		key[index] = value
	}
	return base64.StdEncoding.EncodeToString(key)
}
