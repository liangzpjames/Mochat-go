package store

import "testing"

func TestSaaSTenantDefaultCorpValues(t *testing.T) {
	name, wxCorpID := saasTenantDefaultCorpValues(42, "  示例客户  ")
	if name != "示例客户" {
		t.Fatalf("name = %q, want 示例客户", name)
	}
	if wxCorpID != "fake_tenant_42" {
		t.Fatalf("wxCorpID = %q, want fake_tenant_42", wxCorpID)
	}
}
