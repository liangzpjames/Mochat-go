package saasalertcredentials

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestManagerEncryptsAndBindsCredentials(t *testing.T) {
	manager, err := NewManager(Config{
		EncryptionKey:       testKey(1),
		EncryptionKeyID:     "alert-v1",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, keyID, err := manager.Encrypt(7, "webhook", Credential{
		WebhookURL: "https://hooks.example.com/token", WebhookSecret: "signing-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if keyID != "alert-v1" || strings.Contains(encoded, "hooks.example.com") || strings.Contains(encoded, "signing-secret") {
		t.Fatalf("encrypted credential leaked plaintext or key id mismatch: key=%q value=%q", keyID, encoded)
	}
	credential, err := manager.Decrypt(7, "webhook", keyID, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if credential.WebhookURL != "https://hooks.example.com/token" || credential.WebhookSecret != "signing-secret" {
		t.Fatalf("decrypted credential = %+v", credential)
	}
	if _, err := manager.Decrypt(8, "webhook", keyID, encoded); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("cross-tenant decrypt error = %v", err)
	}
	if _, err := manager.Decrypt(7, "email", keyID, encoded); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("cross-channel decrypt error = %v", err)
	}
}

func TestManagerReadsHistoricalKeyAndWritesActiveKey(t *testing.T) {
	ring := `{"alert-v1":"` + testKey(1) + `","alert-v2":"` + testKey(2) + `"}`
	oldManager, err := NewManager(Config{EncryptionKeys: ring, EncryptionKeyID: "alert-v1"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, oldKeyID, err := oldManager.Encrypt(9, "webhook", Credential{WebhookURL: "https://hooks.example.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(Config{EncryptionKeys: ring, EncryptionKeyID: "alert-v2"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Decrypt(9, "webhook", oldKeyID, encoded); err != nil {
		t.Fatal(err)
	}
	_, activeKeyID, err := manager.Encrypt(9, "webhook", Credential{WebhookURL: "https://hooks.example.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	if activeKeyID != "alert-v2" {
		t.Fatalf("active key id = %q", activeKeyID)
	}
}

func TestManagerRejectsMissingOrConflictingKeyConfiguration(t *testing.T) {
	if _, err := NewManager(Config{RequireEncryption: true}); err == nil {
		t.Fatal("expected required encryption key error")
	}
	if _, err := NewManager(Config{EncryptionKeys: `{"old":"` + testKey(1) + `"}`, EncryptionKeyID: "missing"}); err == nil {
		t.Fatal("expected missing active key error")
	}
	if _, err := NewManager(Config{
		EncryptionKey: testKey(1), EncryptionKeys: `{"same":"` + testKey(2) + `"}`, EncryptionKeyID: "same",
	}); err == nil {
		t.Fatal("expected conflicting active key error")
	}
}

func TestSelectEncryptionSourceKeepsDomainsIsolated(t *testing.T) {
	dedicated, dedicatedConfigured := SelectEncryptionSource(
		EncryptionSource{Key: testKey(1), KeyID: "alert-v1"},
		EncryptionSource{Keys: `{"identity-v1":"` + testKey(2) + `"}`, KeyID: "identity-v1"},
	)
	if !dedicatedConfigured || dedicated.Key == "" || dedicated.Keys != "" || dedicated.KeyID != "alert-v1" {
		t.Fatalf("dedicated source = %+v, configured = %t", dedicated, dedicatedConfigured)
	}

	fallback, dedicatedConfigured := SelectEncryptionSource(
		EncryptionSource{},
		EncryptionSource{Key: testKey(3), KeyID: "identity-v1"},
		EncryptionSource{Keys: `{"compliance-v1":"` + testKey(4) + `"}`, KeyID: "compliance-v1"},
	)
	if dedicatedConfigured || fallback.Key == "" || fallback.Keys != "" || fallback.KeyID != "identity-v1" {
		t.Fatalf("fallback source = %+v, configured = %t", fallback, dedicatedConfigured)
	}
}

func testKey(value byte) string {
	key := make([]byte, 32)
	for index := range key {
		key[index] = value
	}
	return base64.StdEncoding.EncodeToString(key)
}
