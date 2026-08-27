package main

import "testing"

func TestLoadConfigRequiresLongIndependentTokensForFixtureMode(t *testing.T) {
	env := map[string]string{
		"MOCHAT_ARCHIVE_BRIDGE_BEARER":        "bridge-0123456789012345678901234567890123456789",
		"MOCHAT_ARCHIVE_FIXTURE_ENABLED":      "true",
		"MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER": "admin-0123456789012345678901234567890123456789",
		"MOCHAT_ARCHIVE_FIXTURE_STATE_PATH":   "/tmp/fixture/state.json",
	}
	config, err := loadConfig(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if !config.fixtureEnabled || config.bridgeBearer == config.fixtureAdminBearer || config.address != ":8083" {
		t.Fatalf("config=%+v", config)
	}
	delete(env, "MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER")
	if _, err := loadConfig(func(key string) string { return env[key] }); err == nil {
		t.Fatal("expected missing admin token to fail closed")
	}
}
