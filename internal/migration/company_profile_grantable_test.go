package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompanyProfileGrantable0162DownPreservesPreexistingResources(t *testing.T) {
	downBytes, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0162_company_profile_grantable.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down := string(downBytes)
	upperDown := strings.ToUpper(down)
	if strings.Contains(upperDown, "DELETE") || strings.Contains(upperDown, "UPDATE") {
		t.Fatal("0162 down must not mutate a permission or resource whose pre-migration ownership/state is unknown")
	}
	if !strings.Contains(down, "ownership and previous restriction are unknown") || !strings.Contains(upperDown, "DO 0") {
		t.Fatal("0162 down must document and execute its conservative no-op rollback")
	}
}
