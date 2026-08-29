package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContactBatchTitleMigrationIsScopedReversibleAndMySQL57Compatible(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0175_contact_batch_title.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0175_contact_batch_title.down.sql"))
	if err != nil {
		t.Fatal(err)
	}

	upSource := string(up)
	for _, required := range []string{
		"information_schema.columns", "table_name = 'mc_contact_message_batch_send'", "column_name = 'batch_title'",
		"ALTER TABLE `mc_contact_message_batch_send` ADD COLUMN `batch_title` varchar(100)",
	} {
		if !strings.Contains(upSource, required) {
			t.Errorf("0175 up migration missing %q", required)
		}
	}
	downSource := string(down)
	for _, required := range []string{
		"information_schema.columns", "table_name = 'mc_contact_message_batch_send'", "column_name = 'batch_title'",
		"ALTER TABLE `mc_contact_message_batch_send` DROP COLUMN `batch_title`",
	} {
		if !strings.Contains(downSource, required) {
			t.Errorf("0175 down migration missing %q", required)
		}
	}
	for _, source := range []string{upSource, downSource} {
		for _, incompatible := range []string{"ADD COLUMN IF NOT EXISTS", "DROP COLUMN IF EXISTS", "RETURNING", "SKIP LOCKED"} {
			if strings.Contains(strings.ToUpper(source), incompatible) {
				t.Errorf("0175 uses MySQL 5.7 incompatible syntax %q", incompatible)
			}
		}
	}
}
