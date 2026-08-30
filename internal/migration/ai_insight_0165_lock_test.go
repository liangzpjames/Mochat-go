package migration

import "testing"

func TestAIInsight0165LockNameIsStableSchemaScopedAndBounded(t *testing.T) {
	checksum := "4575a0d89e59cf0b87059c0d60575be3e5cc566ee7338cc6fb6f616e8431520f"
	first, err := aiInsight0165LockName("tenant_schema_a", checksum)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := aiInsight0165LockName("tenant_schema_a", checksum)
	if err != nil {
		t.Fatal(err)
	}
	second, err := aiInsight0165LockName("tenant_schema_b", checksum)
	if err != nil {
		t.Fatal(err)
	}
	if first != repeated {
		t.Fatalf("lock name is not stable: %q != %q", first, repeated)
	}
	if first == second {
		t.Fatalf("different schemas share lock name %q", first)
	}
	if len(first) > 64 {
		t.Fatalf("lock name length = %d, exceeds MySQL limit: %q", len(first), first)
	}
	separatedA, err := aiInsight0165LockName("a", "bc")
	if err != nil {
		t.Fatal(err)
	}
	separatedB, err := aiInsight0165LockName("ab", "c")
	if err != nil {
		t.Fatal(err)
	}
	if separatedA == separatedB {
		t.Fatalf("schema/checksum boundary is ambiguous: %q", separatedA)
	}
	if _, err := aiInsight0165LockName("", checksum); err == nil {
		t.Fatal("empty selected schema unexpectedly produced a lock name")
	}
}
