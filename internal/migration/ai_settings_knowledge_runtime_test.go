package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAISettingsKnowledgeRuntimeMigration(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0156_ai_settings_knowledge_runtime.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0156_ai_settings_knowledge_runtime.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"mochat_go_ai_knowledge_documents",
		"mochat_go_ai_knowledge_chunks",
		"system_key",
		"session-analysis",
		"uq_ai_agents_system_key",
		"/dashboard/ai-settings/knowledge-bases/{id}/documents",
		"/dashboard/ai-settings/knowledge-bases/{id}/documents/{documentId}",
	} {
		if !strings.Contains(string(up), fragment) {
			t.Fatalf("up migration missing %q", fragment)
		}
	}
	if !strings.Contains(string(down), "SIGNAL SQLSTATE '45000'") {
		t.Fatal("rollback must refuse to destroy uploaded knowledge data")
	}
	for _, destructive := range []string{"DELETE ", "DROP TABLE", "DROP COLUMN"} {
		if strings.Contains(strings.ToUpper(string(down)), destructive) {
			t.Fatalf("down migration contains destructive operation %q", destructive)
		}
	}
	if _, err := SplitSQLStatements(string(up)); err != nil {
		t.Fatalf("up migration is not executable by production splitter: %v", err)
	}
	if _, err := SplitSQLStatements(string(down)); err != nil {
		t.Fatalf("down migration is not executable by production splitter: %v", err)
	}
}
