package http

import (
	"strings"
	"testing"
)

func TestAcceptanceResourceIDsAlwaysCarryCleanupPrefix(t *testing.T) {
	for i := 0; i < 20; i++ {
		id := acceptanceID()
		if len(id) > 36 || !strings.HasPrefix(id, AcceptancePrefix) {
			t.Fatalf("unsafe acceptance id %q", id)
		}
	}
}
