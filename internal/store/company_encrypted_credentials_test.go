package store

import (
	"testing"

	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestCompanyCredentialReencryptsWhenAuthoritativeCorpIDChanges(t *testing.T) {
	manager := testWeComCredentialManager(t, wecomcredentials.Config{
		EncryptionKey:       testCompanyCredentialKey(21),
		EncryptionKeyID:     "task10-rebind-key",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	ciphertext, keyID, err := manager.EncryptCorp(1, "ww-candidate", wecomcredentials.CorpCredential{
		EmployeeSecret: "employee-secret",
		ChatSecret:     "archive-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := &MySQLStore{weComCredentialCipher: manager}
	storage, err := reencryptCompanyCorpCredential(store, corpCredentialRecord{
		ID: 100, TenantID: 1, WXCorpID: "ww-candidate", Ciphertext: ciphertext, KeyID: keyID,
	}, "ww-authoritative")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := manager.DecryptCorp(1, "ww-authoritative", storage.KeyID, storage.Ciphertext)
	if err != nil || decoded.EmployeeSecret != "employee-secret" || decoded.ChatSecret != "archive-secret" {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
}

func TestCompanyCredentialRotationCanInitializeEmptyEncryptedSlot(t *testing.T) {
	credential, err := companyCredentialForRotation(&MySQLStore{}, corpCredentialRecord{})
	if err != nil {
		t.Fatal(err)
	}
	if credential != (wecomcredentials.CorpCredential{}) {
		t.Fatalf("credential=%+v, want empty initial slot", credential)
	}
}

func TestCompanyCredentialCanBeEncryptedBeforeCandidateCorpIDIsVerified(t *testing.T) {
	manager := testWeComCredentialManager(t, wecomcredentials.Config{
		EncryptionKey:       testCompanyCredentialKey(22),
		EncryptionKeyID:     "task10-pending-key",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	store := &MySQLStore{weComCredentialCipher: manager}
	storage, err := store.encodeCorpCredential(1, "", wecomcredentials.CorpCredential{EmployeeSecret: "pending-secret"})
	if err != nil || storage.Ciphertext == "" || storage.KeyID == "" {
		t.Fatalf("storage=%+v err=%v", storage, err)
	}
	decoded, err := manager.DecryptCorp(1, "", storage.KeyID, storage.Ciphertext)
	if err != nil || decoded.EmployeeSecret != "pending-secret" {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
}
