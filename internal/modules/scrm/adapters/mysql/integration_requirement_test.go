//go:build integration

package mysql

import "testing"

func TestResolveMySQLIntegrationDSNRequiresDSNWhenStrict(t *testing.T) {
	_, err := resolveMySQLIntegrationDSN("", true)
	if err == nil {
		t.Fatal("expected missing DSN error in strict integration mode")
	}
}

func TestResolveMySQLIntegrationDSNAllowsDeveloperSkip(t *testing.T) {
	dsn, err := resolveMySQLIntegrationDSN("", false)
	if err != nil {
		t.Fatal(err)
	}
	if dsn != "" {
		t.Fatalf("dsn = %q, want empty", dsn)
	}
}

func TestResolveMySQLIntegrationDSNUsesConfiguredDSN(t *testing.T) {
	const want = "user:pass@tcp(mysql:3306)/mochat"
	dsn, err := resolveMySQLIntegrationDSN(want, true)
	if err != nil {
		t.Fatal(err)
	}
	if dsn != want {
		t.Fatalf("dsn = %q, want %q", dsn, want)
	}
}
