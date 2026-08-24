package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAISettingsKnowledgeCollationMigrationAlignsJoinKeys(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0160_ai_settings_knowledge_collation.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0160_ai_settings_knowledge_collation.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"ALTER TABLE `mochat_go_ai_knowledge_documents`",
		"MODIFY COLUMN `knowledge_base_id` varchar(36)",
		"COLLATE utf8mb4_general_ci NOT NULL",
		"ALTER TABLE `mochat_go_ai_knowledge_chunks`",
	} {
		if !strings.Contains(string(up), fragment) {
			t.Fatalf("up migration missing %q", fragment)
		}
	}
	if !strings.Contains(string(down), "COLLATE utf8mb4_unicode_ci NOT NULL") {
		t.Fatal("down migration must restore the previous knowledge join-key collation")
	}
	if _, err := SplitSQLStatements(string(up)); err != nil {
		t.Fatalf("up migration is not executable by production splitter: %v", err)
	}
	if _, err := SplitSQLStatements(string(down)); err != nil {
		t.Fatalf("down migration is not executable by production splitter: %v", err)
	}
}
