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
		"sub_part is null",
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

func TestArchiveSourceMigrationGuardNormalizesMariaDBColumnDefaults(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0138_archive_source_sync.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(body))
	for _, required := range []string{
		"column_default = '' or column_default = concat(char(39),char(39)) or column_default = concat(char(34),char(34))",
		"upper(trim(coalesce(column_default,''))) = 'null'",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("0138 guard missing MariaDB column_default normalization contract %q", required)
		}
	}
}
