package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultInventoryUsesDiscoveredRegistryAndCanonicalChecksums(t *testing.T) {
	root := t.TempDir()
	migrationDir := filepath.Join(root, "deploy", "standalone", "migrations")
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "deploy", "standalone", "schema"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "deploy", "standalone", "schema", "mochat.sql"), []byte("CREATE TABLE initial (id int);"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "0002_second.up.sql"), []byte("CREATE TABLE second (id int);"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "0002_second.down.sql"), []byte("DROP TABLE second;"), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := DefaultInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("inventory length = %d, want 2: %#v", len(items), items)
	}
	if items[0].Version != "0001_initial_schema" || items[1].Version != "0002_second" {
		t.Fatalf("inventory order = %#v", items)
	}
	for _, item := range items {
		if len(item.Checksum) != 64 {
			t.Fatalf("checksum for %s = %q", item.Version, item.Checksum)
		}
	}
	if items[1].Kind != MigrationAutomatic {
		t.Fatalf("kind = %q, want automatic", items[1].Kind)
	}
}
