package mysql

import (
	"os"
	"strings"
	"testing"
)

func TestSQLOrderRepositoryHasNoLegacyCreateEntryPoint(t *testing.T) {
	body, err := os.ReadFile("order_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, forbidden := range []string{
		"func (r *SQLOrderRepository) CreateContext(",
		"func (r *SQLOrderRepository) Create(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("legacy order creation entry point remains: %s", forbidden)
		}
	}
	if !strings.Contains(source, "func (r *SQLOrderRepository) CreateIdempotentContext(") {
		t.Fatal("explicit idempotent order creation entry point is missing")
	}
}
