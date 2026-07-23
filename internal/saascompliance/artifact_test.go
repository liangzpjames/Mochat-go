package saascompliance

import (
	"bytes"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestComplianceArtifactRoundTripAndTamperDetection(t *testing.T) {
	master := make([]byte, 32)
	if _, err := rand.Read(master); err != nil {
		t.Fatal(err)
	}
	key := deriveComplianceKey(master)
	payload := bytes.Repeat([]byte("tenant-export-payload\n"), complianceEncryptionChunkSize/8)
	var encrypted bytes.Buffer
	writer, err := newEncryptedWriter(&encrypted, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := newEncryptedReader(bytes.NewReader(encrypted.Bytes()), key)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatal("decrypted compliance artifact differs from source")
	}

	tampered := append([]byte(nil), encrypted.Bytes()...)
	tampered[len(tampered)-1] ^= 0xff
	reader, err = newEncryptedReader(bytes.NewReader(tampered), key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(reader); err == nil {
		t.Fatal("tampered compliance artifact unexpectedly authenticated")
	}
}

func TestComplianceKeyDerivationAndSafeStoragePath(t *testing.T) {
	master := bytes.Repeat([]byte{7}, 32)
	derived := deriveComplianceKey(master)
	if len(derived) != 32 || bytes.Equal(derived, master) {
		t.Fatalf("derived key is not domain separated: %x", derived)
	}
	root := t.TempDir()
	path, err := safeRootJoin(root, "tenant/asset.txt")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(root, "tenant", "asset.txt") {
		t.Fatalf("safe path = %q", path)
	}
	for _, value := range []string{"../secret", "tenant/../../secret", "", "."} {
		if _, err := safeRootJoin(root, value); err == nil {
			t.Fatalf("unsafe path %q was accepted", value)
		}
	}
	if _, err := safeRootJoin("", "asset.txt"); err == nil {
		t.Fatal("empty storage root was accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "asset.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestComplianceInventoryHasStableUniqueCoverage(t *testing.T) {
	items := Inventory()
	if len(items) != 158 {
		t.Fatalf("inventory item count = %d, want 158", len(items))
	}
	keys := make(map[string]struct{}, len(items))
	orderBy := make(map[string]string, len(items))
	covered := InventoryCoveredTables()
	for index, item := range items {
		if item.Key == "" || item.Table == "" || item.Predicate == "" || !item.Export {
			t.Fatalf("invalid inventory item: %+v", item)
		}
		if index > 0 && items[index-1].DeleteOrder > item.DeleteOrder {
			t.Fatalf("inventory order is unstable at %s", item.Key)
		}
		if _, exists := keys[item.Key]; exists {
			t.Fatalf("duplicate inventory key: %s", item.Key)
		}
		keys[item.Key] = struct{}{}
		orderBy[item.Table] = item.OrderBy
		if _, ok := covered[item.Table]; !ok {
			t.Fatalf("inventory table is not covered: %s", item.Table)
		}
	}
	if orderBy["mochat_go_saas_identity_policies"] != "tenant_id" || orderBy["mochat_go_saas_identity_user_states"] != "user_id" {
		t.Fatalf("identity dataset order keys = %+v", orderBy)
	}
	if orderBy["mochat_go_saas_branding_profiles"] != "tenant_id" || orderBy["mochat_go_saas_admin_audit_chains"] != "tenant_id" {
		t.Fatalf("branding dataset order key = %q", orderBy["mochat_go_saas_branding_profiles"])
	}
	for _, required := range []string{"mc_tenant", "mc_user", "mc_corp", "mc_work_contact", "mc_work_contact_room", "mochat_go_saas_branding_profiles", "mochat_go_saas_payment_orders", "mochat_go_saas_service_account_keys", "mochat_go_saas_service_account_usage_daily", "mochat_go_saas_tenant_domain_deliveries", "mochat_go_saas_tenant_domain_delivery_jobs", "mochat_go_saas_tenant_domain_delivery_events", "mochat_go_saas_admin_audit_chains", "mochat_go_saas_admin_audit_verifications"} {
		if _, ok := covered[required]; !ok {
			t.Fatalf("required table missing from inventory: %s", required)
		}
	}
}
