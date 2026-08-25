package aiproviderconfig

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestManagerEncryptsTenantProviderCredentialWithoutPlaintext(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(1), EncryptionKeyID: "ai-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}

	ciphertext, keyID, hint, err := manager.Encrypt(7, "openai", "fixture-key-alpha-1234")
	if err != nil {
		t.Fatal(err)
	}
	if keyID != "ai-v1" || hint != "1234" || strings.Contains(ciphertext, "fixture-key-alpha-1234") {
		t.Fatal("encrypted credential leaked plaintext or metadata mismatch")
	}
	credential, err := manager.Decrypt(7, "openai", keyID, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if credential != "fixture-key-alpha-1234" {
		t.Fatal("decrypted credential did not match the configured fixture")
	}
}

func TestManagerRejectsCrossTenantCredentialDecrypt(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(2), EncryptionKeyID: "ai-v1"})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, _, err := manager.Encrypt(7, "openai", "fixture-key-beta-5678")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Decrypt(8, "openai", keyID, ciphertext); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("cross-tenant decrypt error = %v", err)
	}
}

func TestManagerReadsHistoricalKeyAndWritesActiveKey(t *testing.T) {
	ring := `{"ai-v1":"` + testKey(3) + `","ai-v2":"` + testKey(4) + `"}`
	oldManager, err := NewManager(Config{EncryptionKeys: ring, EncryptionKeyID: "ai-v1"})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, oldKeyID, _, err := oldManager.Encrypt(9, "anthropic", "fixture-key-old-9999")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(Config{EncryptionKeys: ring, EncryptionKeyID: "ai-v2"})
	if err != nil {
		t.Fatal(err)
	}
	if credential, err := manager.Decrypt(9, "anthropic", oldKeyID, ciphertext); err != nil || credential != "fixture-key-old-9999" {
		t.Fatalf("historical credential did not decrypt: %v", err)
	}
	_, activeKeyID, _, err := manager.Encrypt(9, "anthropic", "fixture-key-new-0000")
	if err != nil {
		t.Fatal(err)
	}
	if activeKeyID != "ai-v2" {
		t.Fatalf("active key id = %q", activeKeyID)
	}
}

func TestManagerRejectsEncryptionWithoutKey(t *testing.T) {
	manager, err := NewManager(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := manager.Encrypt(7, "openai", "fixture-key-none-0000"); err == nil {
		t.Fatal("expected encryption without a configured key to fail")
	}
	if _, err := NewManager(Config{RequireEncryption: true}); err == nil {
		t.Fatal("expected required encryption configuration to reject missing key")
	}
}

func TestManagerRejectsCredentialTooShortForSafeHint(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(6), EncryptionKeyID: "ai-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, hint, err := manager.Encrypt(7, "openai", "tiny"); err != nil || hint != "" {
		t.Fatal("short credential unexpectedly produced a hint or encryption error")
	}
}

func TestManagerDecryptStoredFailsClosedForInactiveOrExpiredConfig(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(5), EncryptionKeyID: "ai-v1"})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, hint, err := manager.Encrypt(7, "openai", "fixture-key-config-2468")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC)
	config := StoredConfig{TenantID: 7, Provider: "openai", Model: "fixture-model", CredentialCiphertext: ciphertext, KeyID: keyID, Hint: hint, Status: StatusActive, Version: 1, CreatedBy: 41, UpdatedBy: 42}
	if credential, err := manager.DecryptStored(config, now); err != nil || credential != "fixture-key-config-2468" {
		t.Fatalf("active stored config did not decrypt: %v", err)
	}
	config.Status = StatusDisabled
	if _, err := manager.DecryptStored(config, now); err == nil {
		t.Fatal("expected disabled stored config to fail closed")
	}
	config.Status = StatusActive
	expired := now.Add(-time.Second)
	config.ExpiresAt = &expired
	if _, err := manager.DecryptStored(config, now); err == nil {
		t.Fatal("expected expired stored config to fail closed")
	}
	config.ExpiresAt = nil
	notYetEffective := now.Add(time.Second)
	config.EffectiveAt = &notYetEffective
	if _, err := manager.DecryptStored(config, now); err == nil {
		t.Fatal("expected not-yet-effective stored config to fail closed")
	}
}

func testKey(value byte) string {
	key := make([]byte, 32)
	for index := range key {
		key[index] = value
	}
	return base64.StdEncoding.EncodeToString(key)
}
