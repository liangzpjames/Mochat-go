package identitymigration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestReadSecretFileRejectsEmptyAndReturnsOnlyTrimmedMaterial(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSecretFile(empty); err == nil {
		t.Fatal("empty secret file unexpectedly accepted")
	}
	key := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(key, []byte("  0707070707070707070707070707070707070707070707070707070707070707  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := ReadSecretFile(key)
	if err != nil || string(value) != "0707070707070707070707070707070707070707070707070707070707070707" {
		t.Fatalf("secret file value=%q err=%v", value, err)
	}
}

func TestCredentialManagerUsesDedicatedKeyFileContract(t *testing.T) {
	key := filepath.Join(t.TempDir(), "wecom.key")
	if err := os.WriteFile(key, []byte(strings.Repeat("07", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewCredentialManagerFromFile(key, "task8-wecom-v1")
	if err != nil || manager == nil || !manager.ConfigStatus().DedicatedConfigured || manager.ConfigStatus().ActiveKeyID != "task8-wecom-v1" {
		t.Fatalf("manager=%v status=%+v err=%v", manager, manager.ConfigStatus(), err)
	}
}

func TestControlledMigrationPathIsExplicitAndNotTheGenericRunner(t *testing.T) {
	path, err := ControlledMigrationPath("D:/workspace/mochat-go/mochat-go", "backfill")
	if err != nil || !strings.HasSuffix(filepath.ToSlash(path), "deploy/standalone/migrations/0130_identity_realms_single_corp_backfill.up.sql") {
		t.Fatalf("path=%q err=%v", path, err)
	}
	if _, err := ControlledMigrationPath(".", "unknown"); err == nil {
		t.Fatal("unknown controlled migration action unexpectedly accepted")
	}
}

func TestCredentialManagerKeyRingDecryptsHistoricalKeyAndRejectsMissingKey(t *testing.T) {
	oldKey := strings.Repeat("07", 32)
	currentKey := strings.Repeat("08", 32)
	legacyManager, err := wecomcredentials.NewManager(wecomcredentials.Config{EncryptionKey: oldKey, EncryptionKeyID: "legacy-v1", RequireEncryption: true, DedicatedConfigured: true})
	if err != nil {
		t.Fatal(err)
	}
	want := wecomcredentials.CorpCredential{EmployeeSecret: "employee-value", ContactSecret: "contact-value", CallbackToken: "callback-value", EncodingAESKey: "encoding-value", ChatSecret: "chat-value"}
	ciphertext, keyID, err := legacyManager.EncryptCorp(7, "wx-corp-7", want)
	if err != nil || keyID != "legacy-v1" {
		t.Fatalf("encrypt legacy credential key=%q err=%v", keyID, err)
	}
	ringFile := filepath.Join(t.TempDir(), "credential-ring.json")
	ringData, _ := json.Marshal(map[string]string{"legacy-v1": oldKey, "current-v1": currentKey})
	if err := os.WriteFile(ringFile, ringData, 0o600); err != nil {
		t.Fatal(err)
	}
	ringManager, err := NewCredentialManagerFromFile(ringFile, "current-v1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ringManager.DecryptCorp(7, "wx-corp-7", "legacy-v1", ciphertext)
	if err != nil || got != want {
		t.Fatalf("historical credential decrypt got=%+v err=%v", got, err)
	}
	if _, err := ringManager.DecryptCorp(7, "wx-corp-7", "missing-v1", ciphertext); err == nil {
		t.Fatal("missing historical key unexpectedly decrypted")
	}
}
