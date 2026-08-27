package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveFixtureDatasetLedgerOwnsOneDatasetPerSource(t *testing.T) {
	path := filepath.Join("..", "..", "deploy", "standalone", "migrations", "0171_archive_fixture_dataset_ledger.up.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(body))
	for _, required := range []string{"mochat_go_archive_fixture_datasets", "primary key (`dataset`)", "unique key `uk_archive_fixture_source`", "`tenant_id`,`corp_id`,`source_identity`", "'seeding','ready','cleaning'"} {
		if !strings.Contains(text, required) {
			t.Fatalf("0171 missing %q", required)
		}
	}
}
