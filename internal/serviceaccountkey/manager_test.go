package serviceaccountkey

import (
	"context"
	"strings"
	"testing"
)

func TestManagerDedicatedRingAndLegacyCompatibility(t *testing.T) {
	oldKey := strings.Repeat("07", 32)
	newKey := strings.Repeat("08", 32)
	manager, err := NewManager(Config{
		ActiveKeyID: "2026-q3", ActiveKey: newKey,
		Keys:            `{"2026-q2":"` + oldKey + `"}`,
		LegacyJWTSecret: "legacy-dashboard-secret", AllowLegacyJWT: true, RequireDedicated: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	status := manager.ConfigStatus()
	if status.ActiveKeyID != "2026-q3" || !status.DedicatedConfigured || !status.LegacyJWTEnabled || status.KeyCount != 3 {
		t.Fatalf("status = %+v", status)
	}
	keyID, digest, err := manager.HashActive("mch_live_example_secret")
	if err != nil || keyID != "2026-q3" || len(digest) != 64 {
		t.Fatalf("active hash = key_id %q digest %q err %v", keyID, digest, err)
	}
	if legacyDigest, ok := manager.Hash("", "mch_live_example_secret"); !ok || len(legacyDigest) != 64 || legacyDigest == digest {
		t.Fatalf("legacy digest = %q ok=%v", legacyDigest, ok)
	}
	if missing := manager.MissingKeyIDs([]string{"2026-q2", "retired", "retired"}); len(missing) != 1 || missing[0] != "retired" {
		t.Fatalf("missing = %#v", missing)
	}
	if err := manager.CheckConfiguration(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestManagerRequiresDedicatedPepper(t *testing.T) {
	if _, err := NewManager(Config{LegacyJWTSecret: "legacy", AllowLegacyJWT: true, RequireDedicated: true}); err == nil {
		t.Fatal("expected dedicated pepper requirement error")
	}
	manager := NewLegacyManager("legacy")
	if manager == nil || manager.ConfigStatus().ActiveKeyID != LegacyJWTKeyID {
		t.Fatalf("legacy manager = %#v", manager)
	}
}

func TestManagerRejectsMissingAndReservedKeyIDs(t *testing.T) {
	key := strings.Repeat("09", 32)
	for _, config := range []Config{
		{ActiveKeyID: "missing", Keys: `{"present":"` + key + `"}`, AllowLegacyJWT: false},
		{ActiveKeyID: LegacyJWTKeyID, ActiveKey: key, AllowLegacyJWT: false},
		{ActiveKeyID: "primary", Keys: `{"legacy-jwt":"` + key + `"}`, AllowLegacyJWT: false},
	} {
		if _, err := NewManager(config); err == nil {
			t.Fatalf("expected invalid config error for %+v", config)
		}
	}
}
