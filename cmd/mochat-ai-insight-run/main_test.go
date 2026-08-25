package main

import "testing"

func TestParseScopeRejectsUnscopedAndBackfillArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"--tenant-id", "1"}, {"--tenant-id", "1", "--corp-id", "2", "--lookback-days", "7"}} {
		if _, _, err := parseScope(args); err == nil {
			t.Fatalf("parseScope(%q) error=nil", args)
		}
	}
	if tenantID, corpID, err := parseScope([]string{"--tenant-id", "7", "--corp-id", "9"}); err != nil || tenantID != 7 || corpID != 9 {
		t.Fatalf("scope=%d/%d err=%v", tenantID, corpID, err)
	}
}
