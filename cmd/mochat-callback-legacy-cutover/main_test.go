package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseCutoverOptionsRequiresTrafficStoppedMaintenanceToken(t *testing.T) {
	env := map[string]string{
		"MOCHAT_GO_ALLOW_WEWORK_CALLBACK_LEGACY_CUTOVER": "1",
		"MOCHAT_GO_MYSQL_DSN":                            "user:secret@tcp(mysql:3306)/mochat",
		"MOCHAT_REDIS_ADDR":                              "redis:6379",
	}
	getenv := func(key string) string { return env[key] }
	if _, err := parseCutoverOptions([]string{"-confirm-legacy-traffic-stopped"}, getenv, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "producers, consumers, and retry writers") {
		t.Fatalf("missing maintenance token error=%v", err)
	}
	env["MOCHAT_GO_WEWORK_CALLBACK_LEGACY_TRAFFIC_STOPPED"] = "1"
	if _, err := parseCutoverOptions(nil, getenv, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "confirm-legacy-traffic-stopped") {
		t.Fatalf("missing explicit confirmation error=%v", err)
	}
	options, err := parseCutoverOptions([]string{"-confirm-legacy-traffic-stopped"}, getenv, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if len(options.ownerToken) != 64 {
		t.Fatalf("owner token length=%d", len(options.ownerToken))
	}
}

func TestCutoverCLIHasNoSensitiveArgvFlags(t *testing.T) {
	env := map[string]string{
		"MOCHAT_GO_ALLOW_WEWORK_CALLBACK_LEGACY_CUTOVER":   "1",
		"MOCHAT_GO_WEWORK_CALLBACK_LEGACY_TRAFFIC_STOPPED": "1",
		"MOCHAT_GO_MYSQL_DSN":                              "user:secret@tcp(mysql:3306)/mochat",
		"MOCHAT_REDIS_ADDR":                                "redis:6379",
	}
	getenv := func(key string) string { return env[key] }
	for _, forbidden := range []string{"-dsn", "-redis-password", "-redis-addr"} {
		if _, err := parseCutoverOptions([]string{forbidden, "secret", "-confirm-legacy-traffic-stopped"}, getenv, &bytes.Buffer{}); err == nil {
			t.Fatalf("sensitive argv flag %q was accepted", forbidden)
		}
	}
	help := &bytes.Buffer{}
	_, _ = parseCutoverOptions([]string{"-help"}, getenv, help)
	for _, forbidden := range []string{"-dsn", "-redis-password", "-redis-addr"} {
		if strings.Contains(help.String(), forbidden) {
			t.Fatalf("help exposes sensitive argv flag %q: %s", forbidden, help.String())
		}
	}
}

func TestLegacySourceFingerprintNormalizesAddressAndExcludesPassword(t *testing.T) {
	a, err := legacySourceFingerprint(" REDIS.EXAMPLE.COM:6379 ", 2)
	if err != nil {
		t.Fatal(err)
	}
	b, err := legacySourceFingerprint("redis.example.com:6379", 2)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || len(a) != 64 || strings.Contains(a, "redis.example.com") {
		t.Fatalf("fingerprints a=%q b=%q", a, b)
	}
	c, _ := legacySourceFingerprint("redis.example.com:6379", 3)
	if c == a {
		t.Fatal("Redis DB was not bound into the source fingerprint")
	}
}

func TestNewCutoverOwnerTokenIsUniqueAndOpaque(t *testing.T) {
	a, err := newCutoverOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := newCutoverOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 64 || len(b) != 64 || a == b {
		t.Fatalf("owner tokens a=%q b=%q", a, b)
	}
}
