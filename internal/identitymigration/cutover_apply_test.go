package identitymigration

import (
	"context"
	"strings"
	"testing"
)

func TestApplyCutoverRequiresDatabaseAndSessionBoundRequest(t *testing.T) {
	_, err := ApplyCutover(context.Background(), nil, DatabaseOptions{}, "missing.sql")
	if err == nil || !strings.Contains(err.Error(), "database") {
		t.Fatalf("ApplyCutover() error=%v, want database requirement", err)
	}
}
