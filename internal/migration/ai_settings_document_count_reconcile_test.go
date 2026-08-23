package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAISettingsDocumentCountReconcileMigrationUsesLiveDocuments(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	body, err := os.ReadFile(filepath.Join(root, "0157_ai_settings_document_count_reconcile.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, required := range []string{
		"UPDATE `mochat_go_ai_knowledge_bases` knowledge_base",
		"FROM `mochat_go_ai_knowledge_documents`",
		"WHERE `deleted_at` IS NULL",
		"documents.`knowledge_base_id` COLLATE utf8mb4_general_ci = knowledge_base.`id`",
		"knowledge_base.`document_count` = COALESCE(documents.`document_count`, 0)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("0157 migration missing %q", required)
		}
	}
}
