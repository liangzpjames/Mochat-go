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
