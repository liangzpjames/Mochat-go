package mysql

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestKnowledgeBaseRepositoryGetByIDsScopesAndExcludesDeleted(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+kbColumns+" FROM mochat_go_ai_knowledge_bases WHERE tenant_id=? AND corp_id=? AND id IN (?,?) AND deleted_at IS NULL")).
		WithArgs(int64(1), int64(2), "kb-1", "kb-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "name", "description", "document_count", "status", "created_by", "updated_by", "created_at", "updated_at"}).
			AddRow("kb-1", 1, 2, "售后库", "", 0, 1, 7, 7, now, now))

	repository, err := NewKnowledgeBaseRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	items, err := repository.GetByIDs(context.Background(), 1, 2, []string{"kb-1", "kb-2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "kb-1" || items[0].TenantID != 1 || items[0].CorpID != 2 {
		t.Fatalf("items = %#v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRepositoryListsKnowledgeBaseReferencesWithinScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT "+agentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND JSON_CONTAINS(knowledge_base_ids, JSON_QUOTE(?)) AND deleted_at IS NULL ORDER BY updated_at DESC")).
		WithArgs(int64(1), int64(2), "kb-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "name", "description", "knowledge_base_ids", "status", "created_by", "updated_by", "created_at", "updated_at"}).
			AddRow("agent-1", 1, 2, "客服助手", "", `["kb-1"]`, 1, 7, 7, now, now))

	repository, err := NewAgentRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	items, err := repository.ListReferencingKnowledgeBase(context.Background(), 1, 2, "kb-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "agent-1" || len(items[0].KnowledgeBaseIDs) != 1 || items[0].KnowledgeBaseIDs[0] != "kb-1" {
		t.Fatalf("items = %#v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
