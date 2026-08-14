package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveSourceMigrationGuardUsesPortableColumnMetadata(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0138_archive_source_sync.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(body))
	for _, required := range []string{
		"data_type",
		"numeric_precision",
		"default_generated",
		"lower(column_type) like '%unsigned'",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("0138 guard missing portable metadata contract %q", required)
		}
	}
	for _, forbidden := range []string{
		"column_type <> 'int(10) unsigned'",
		"column_type <> 'bigint(20) unsigned'",
		"column_type = 'int(10) unsigned'",
		"column_type = 'bigint(20) unsigned'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("0138 guard still depends on MySQL display width: %q", forbidden)
		}
	}
}
