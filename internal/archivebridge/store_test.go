package archivebridge

import "testing"

func TestStoreRejectsConflictingAndInvalidBindings(t *testing.T) {
	store := NewStore()
	binding := Binding{TenantID: 1, CorpID: 2, WXCorpID: "ww-a", IntegrationMode: ModeSelfBuilt}
	if err := store.RegisterFinance(binding, fakeFinanceDriver{}); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterFinance(binding, fakeFinanceDriver{}); err != nil {
		t.Fatalf("idempotent registration failed: %v", err)
	}
	conflict := binding
	conflict.IntegrationMode = ModeThirdPartyDelegated
	if err := store.RegisterFinance(conflict, fakeFinanceDriver{}); err == nil {
		t.Fatal("conflicting mode registration accepted")
	}
	if _, err := store.Resolve(Binding{TenantID: 1, CorpID: 2, WXCorpID: "ww-other", IntegrationMode: ModeSelfBuilt}); ErrorCode(err) != "ARCHIVE_BINDING_MISMATCH" {
		t.Fatalf("binding mismatch error=%v", err)
	}
}
