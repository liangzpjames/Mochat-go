package mysql

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
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

func TestAgentRepositoryRejectsMalformedKnowledgeBaseIDs(t *testing.T) {
	tests := []struct {
		name        string
		expectQuery string
		call        func(*AgentRepository) error
	}{
		{
			name:        "list",
			expectQuery: "SELECT " + agentColumns + " FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND deleted_at IS NULL ORDER BY updated_at DESC",
			call: func(repository *AgentRepository) error {
				_, err := repository.List(context.Background(), 1, 2)
				return err
			},
		},
		{
			name:        "knowledge base reference list",
			expectQuery: "SELECT " + agentColumns + " FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND JSON_CONTAINS(knowledge_base_ids, JSON_QUOTE(?)) AND deleted_at IS NULL ORDER BY updated_at DESC",
			call: func(repository *AgentRepository) error {
				_, err := repository.ListReferencingKnowledgeBase(context.Background(), 1, 2, "kb-1")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
			expectation := mock.ExpectQuery(regexp.QuoteMeta(test.expectQuery))
			if test.name == "list" {
				expectation.WithArgs(int64(1), int64(2))
			} else {
				expectation.WithArgs(int64(1), int64(2), "kb-1")
			}
			expectation.WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "name", "description", "knowledge_base_ids", "status", "created_by", "updated_by", "created_at", "updated_at"}).
				AddRow("agent-1", 1, 2, "客服助手", "", `not-json`, 1, 7, 7, now, now))

			repository, err := NewAgentRepository(db)
			if err != nil {
				t.Fatal(err)
			}
			if err := test.call(repository); err == nil {
				t.Fatal("error = nil, want malformed knowledge_base_ids error")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestKnowledgeBaseRepositoryClassifiesMissingAndStorageFailures(t *testing.T) {
	tests := []struct {
		name    string
		missing bool
		expect  func(sqlmock.Sqlmock)
		call    func(*KnowledgeBaseRepository) error
	}{
		{
			name:    "update missing",
			missing: true,
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_knowledge_bases SET name=?, description=?, document_count=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs("售后库", "", 0, 1, int64(7), sqlmock.AnyArg(), "kb-1", int64(1), int64(2)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			call: func(repository *KnowledgeBaseRepository) error {
				_, err := repository.Update(context.Background(), ports.KnowledgeBase{ID: "kb-1", TenantID: 1, CorpID: 2, Name: "售后库", Status: 1, UpdatedBy: 7})
				return err
			},
		},
		{
			name:    "update storage failure",
			missing: false,
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_knowledge_bases SET name=?, description=?, document_count=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs("售后库", "", 0, 1, int64(7), sqlmock.AnyArg(), "kb-1", int64(1), int64(2)).
					WillReturnError(errors.New("database unavailable"))
			},
			call: func(repository *KnowledgeBaseRepository) error {
				_, err := repository.Update(context.Background(), ports.KnowledgeBase{ID: "kb-1", TenantID: 1, CorpID: 2, Name: "售后库", Status: 1, UpdatedBy: 7})
				return err
			},
		},
		{
			name:    "delete missing",
			missing: true,
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_knowledge_bases SET deleted_at=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "kb-1", int64(1), int64(2)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			call: func(repository *KnowledgeBaseRepository) error {
				return repository.Delete(context.Background(), 1, 2, "kb-1")
			},
		},
		{
			name:    "delete storage failure",
			missing: false,
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_knowledge_bases SET deleted_at=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "kb-1", int64(1), int64(2)).
					WillReturnError(errors.New("database unavailable"))
			},
			call: func(repository *KnowledgeBaseRepository) error {
				return repository.Delete(context.Background(), 1, 2, "kb-1")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			test.expect(mock)
			repository, err := NewKnowledgeBaseRepository(db)
			if err != nil {
				t.Fatal(err)
			}
			err = test.call(repository)
			if test.missing && !errors.Is(err, ports.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
			if !test.missing && (err == nil || errors.Is(err, ports.ErrNotFound)) {
				t.Fatalf("error = %v, want storage error", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAgentRepositoryClassifiesMissingAndStorageFailures(t *testing.T) {
	tests := []struct {
		name    string
		missing bool
		expect  func(sqlmock.Sqlmock)
		call    func(*AgentRepository) error
	}{
		{
			name:    "update missing",
			missing: true,
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_agents SET name=?, description=?, knowledge_base_ids=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs("客服助手", "", `[]`, 1, int64(7), sqlmock.AnyArg(), "agent-1", int64(1), int64(2)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			call: func(repository *AgentRepository) error {
				_, err := repository.Update(context.Background(), ports.Agent{ID: "agent-1", TenantID: 1, CorpID: 2, Name: "客服助手", KnowledgeBaseIDs: []string{}, Status: 1, UpdatedBy: 7})
				return err
			},
		},
		{
			name:    "update storage failure",
			missing: false,
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_agents SET name=?, description=?, knowledge_base_ids=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs("客服助手", "", `[]`, 1, int64(7), sqlmock.AnyArg(), "agent-1", int64(1), int64(2)).
					WillReturnError(errors.New("database unavailable"))
			},
			call: func(repository *AgentRepository) error {
				_, err := repository.Update(context.Background(), ports.Agent{ID: "agent-1", TenantID: 1, CorpID: 2, Name: "客服助手", KnowledgeBaseIDs: []string{}, Status: 1, UpdatedBy: 7})
				return err
			},
		},
		{
			name:    "delete missing",
			missing: true,
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_agents SET deleted_at=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "agent-1", int64(1), int64(2)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			call: func(repository *AgentRepository) error {
				return repository.Delete(context.Background(), 1, 2, "agent-1")
			},
		},
		{
			name:    "delete storage failure",
			missing: false,
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_agents SET deleted_at=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "agent-1", int64(1), int64(2)).
					WillReturnError(errors.New("database unavailable"))
			},
			call: func(repository *AgentRepository) error {
				return repository.Delete(context.Background(), 1, 2, "agent-1")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			test.expect(mock)
			repository, err := NewAgentRepository(db)
			if err != nil {
				t.Fatal(err)
			}
			err = test.call(repository)
			if test.missing && !errors.Is(err, ports.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
			if !test.missing && (err == nil || errors.Is(err, ports.ErrNotFound)) {
				t.Fatalf("error = %v, want storage error", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
