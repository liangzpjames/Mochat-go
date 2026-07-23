package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestWeComCredentialStorageEncryptsAndDecrypts(t *testing.T) {
	manager := testWeComCredentialManager(t, wecomcredentials.Config{
		EncryptionKey:       "3131313131313131313131313131313131313131313131313131313131313131",
		EncryptionKeyID:     "wecom-q3",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	store := &MySQLStore{weComCredentialCipher: manager}
	corpCredential := wecomcredentials.CorpCredential{
		EmployeeSecret: "employee-secret", ContactSecret: "contact-secret", CallbackToken: "callback-token",
		EncodingAESKey: "encoding-key", ChatSecret: "chat-secret",
	}
	corpStorage, err := store.encodeCorpCredential(10, "wx-corp-10", corpCredential)
	if err != nil {
		t.Fatal(err)
	}
	if corpStorage.EmployeeSecret != "" || corpStorage.ContactSecret != "" || corpStorage.CallbackToken != "" ||
		corpStorage.EncodingAESKey != "" || corpStorage.ChatSecret != "" || corpStorage.KeyID != "wecom-q3" ||
		corpStorage.Ciphertext == "" || strings.Contains(corpStorage.Ciphertext, "employee-secret") {
		t.Fatalf("corp storage=%+v", corpStorage)
	}
	decodedCorp, err := store.decodeCorpCredential(corpCredentialRecord{
		TenantID: 10, WXCorpID: "wx-corp-10", Ciphertext: corpStorage.Ciphertext, KeyID: corpStorage.KeyID,
	})
	if err != nil || decodedCorp != corpCredential {
		t.Fatalf("decoded corp=%+v err=%v", decodedCorp, err)
	}

	agentCredential := wecomcredentials.AgentCredential{WXSecret: "agent-secret"}
	agentStorage, err := store.encodeAgentCredential(25, "1000010", agentCredential)
	if err != nil {
		t.Fatal(err)
	}
	if agentStorage.WXSecret != "" || agentStorage.KeyID != "wecom-q3" || agentStorage.Ciphertext == "" || strings.Contains(agentStorage.Ciphertext, "agent-secret") {
		t.Fatalf("agent storage=%+v", agentStorage)
	}
	decodedAgent, err := store.decodeAgentCredential(agentCredentialRecord{
		CorpID: 25, WXAgentID: "1000010", Ciphertext: agentStorage.Ciphertext, KeyID: agentStorage.KeyID,
	})
	if err != nil || decodedAgent != agentCredential {
		t.Fatalf("decoded agent=%+v err=%v", decodedAgent, err)
	}
}

func TestWeComCredentialStorageReadsHistoricalKeyAndLegacyPlaintext(t *testing.T) {
	oldManager := testWeComCredentialManager(t, wecomcredentials.Config{
		EncryptionKey:   "3232323232323232323232323232323232323232323232323232323232323232",
		EncryptionKeyID: "old",
	})
	ciphertext, keyID, err := oldManager.EncryptCorp(10, "wx-corp-10", wecomcredentials.CorpCredential{EmployeeSecret: "old-secret"})
	if err != nil {
		t.Fatal(err)
	}
	manager := testWeComCredentialManager(t, wecomcredentials.Config{
		EncryptionKeys:  `{"old":"3232323232323232323232323232323232323232323232323232323232323232","new":"3333333333333333333333333333333333333333333333333333333333333333"}`,
		EncryptionKeyID: "new",
	})
	store := &MySQLStore{weComCredentialCipher: manager}
	decoded, err := store.decodeCorpCredential(corpCredentialRecord{TenantID: 10, WXCorpID: "wx-corp-10", Ciphertext: ciphertext, KeyID: keyID})
	if err != nil || decoded.EmployeeSecret != "old-secret" {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	legacy, err := store.decodeCorpCredential(corpCredentialRecord{
		EmployeeSecret: "employee", ContactSecret: "contact", CallbackToken: "token", EncodingAESKey: "aes", ChatSecret: "chat",
	})
	if err != nil || legacy.EmployeeSecret != "employee" || legacy.ContactSecret != "contact" || legacy.CallbackToken != "token" || legacy.EncodingAESKey != "aes" || legacy.ChatSecret != "chat" {
		t.Fatalf("legacy=%+v err=%v", legacy, err)
	}
	legacyAgent, err := store.decodeAgentCredential(agentCredentialRecord{WXSecret: "legacy-agent"})
	if err != nil || legacyAgent.WXSecret != "legacy-agent" {
		t.Fatalf("legacy agent=%+v err=%v", legacyAgent, err)
	}
}

func TestWeComCredentialStorageRetainsCompatibilityWithoutKey(t *testing.T) {
	store := &MySQLStore{}
	corp, err := store.encodeCorpCredential(10, "wx-corp-10", wecomcredentials.CorpCredential{EmployeeSecret: " employee ", ChatSecret: " chat "})
	if err != nil || corp.EmployeeSecret != "employee" || corp.ChatSecret != "chat" || corp.Ciphertext != "" || corp.KeyID != "" {
		t.Fatalf("corp=%+v err=%v", corp, err)
	}
	agent, err := store.encodeAgentCredential(25, "1000010", wecomcredentials.AgentCredential{WXSecret: " agent "})
	if err != nil || agent.WXSecret != "agent" || agent.Ciphertext != "" || agent.KeyID != "" {
		t.Fatalf("agent=%+v err=%v", agent, err)
	}
}

func TestWeComCredentialRotationAndMissingKeyStatus(t *testing.T) {
	legacyCorp := corpCredentialRecord{EmployeeSecret: "legacy"}
	activeCorp := corpCredentialRecord{Ciphertext: "ciphertext", KeyID: "active"}
	staleCorp := corpCredentialRecord{Ciphertext: "ciphertext", KeyID: "old"}
	if !corpCredentialNeedsRotation(legacyCorp, "active") || corpCredentialNeedsRotation(activeCorp, "active") || !corpCredentialNeedsRotation(staleCorp, "active") {
		t.Fatalf("corp rotation legacy=%t active=%t stale=%t", corpCredentialNeedsRotation(legacyCorp, "active"), corpCredentialNeedsRotation(activeCorp, "active"), corpCredentialNeedsRotation(staleCorp, "active"))
	}
	if !agentCredentialNeedsRotation(agentCredentialRecord{WXSecret: "legacy"}, "active") ||
		agentCredentialNeedsRotation(agentCredentialRecord{Ciphertext: "ciphertext", KeyID: "active"}, "active") {
		t.Fatal("agent rotation classification mismatch")
	}

	status := dashboard.SaaSWeComCredentialProtectionStatus{ActiveKeyID: "active", KeyCount: 1}
	unavailable := map[string]struct{}{}
	updateWeComProtectionStatus(&status, unavailable, "ciphertext", "", false, nil)
	if _, found := unavailable["(missing)"]; !found || status.EncryptedCredentialCount != 1 || status.RotationRequiredCount != 1 {
		t.Fatalf("status=%+v unavailable=%+v", status, unavailable)
	}
}

func testWeComCredentialManager(t *testing.T, config wecomcredentials.Config) *wecomcredentials.Manager {
	t.Helper()
	manager, err := wecomcredentials.NewManager(config)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}
