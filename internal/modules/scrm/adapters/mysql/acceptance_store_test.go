package mysql

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestAcceptanceResourceIDsAlwaysCarryCleanupPrefix(t *testing.T) {
	for i := 0; i < 20; i++ {
		id := acceptanceID()
		if len(id) > 36 || !strings.HasPrefix(id, ports.AcceptancePrefix) {
			t.Fatalf("unsafe acceptance id %q", id)
		}
	}
}
