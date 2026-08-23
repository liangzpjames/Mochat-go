package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

const auditInsertSQL = "INSERT INTO mochat_go_ai_settings_audits (tenant_id, corp_id, actor_user_id, entity_type, entity_id, action, changed_fields) VALUES (?,?,?,?,?,?,?)"

func TestAISettingsRepositoriesCommitEveryMutationWithAudit(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		entityType     string
		entityID       string
		action         string
		actor          int64
		changedFields  []string
		expectBusiness func(sqlmock.Sqlmock)
		expectRead     func(sqlmock.Sqlmock)
		call           func(*sql.DB) error
	}{
		{
			name:          "knowledge base create",
			entityType:    "knowledge_base",
			entityID:      "kb-1",
			action:        "create",
			actor:         17,
			changedFields: []string{"name", "description", "document_count", "status"},
			expectBusiness: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_ai_knowledge_bases (id, tenant_id, corp_id, name, description, document_count, status, created_by, updated_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)")).
					WithArgs("kb-1", int64(1), int64(2), "售后库", "仅元数据", 3, 1, int64(17), int64(17), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectRead: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT "+kbColumns+" FROM mochat_go_ai_knowledge_bases WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs("kb-1", int64(1), int64(2)).
					WillReturnRows(knowledgeBaseRows().AddRow("kb-1", 1, 2, "售后库", "仅元数据", 3, 1, 17, 17, now, now))
			},
			call: func(db *sql.DB) error {
				repo, err := NewKnowledgeBaseRepository(db)
				if err != nil {
					return err
				}
				_, err = repo.Create(context.Background(), ports.KnowledgeBase{ID: "kb-1", TenantID: 1, CorpID: 2, Name: "售后库", Description: "仅元数据", DocumentCount: 3, Status: 1, CreatedBy: 17, UpdatedBy: 17})
				return err
			},
		},
		{
			name:          "knowledge base update",
			entityType:    "knowledge_base",
			entityID:      "kb-1",
			action:        "update",
			actor:         17,
			changedFields: []string{"name", "description", "document_count", "status"},
			expectBusiness: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_knowledge_bases SET name=?, description=?, document_count=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs("售后库", "仅元数据", 3, 0, int64(17), sqlmock.AnyArg(), "kb-1", int64(1), int64(2)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectRead: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT "+kbColumns+" FROM mochat_go_ai_knowledge_bases WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs("kb-1", int64(1), int64(2)).
					WillReturnRows(knowledgeBaseRows().AddRow("kb-1", 1, 2, "售后库", "仅元数据", 3, 0, 17, 17, now, now))
			},
			call: func(db *sql.DB) error {
				repo, err := NewKnowledgeBaseRepository(db)
				if err != nil {
					return err
				}
				_, err = repo.Update(context.Background(), ports.KnowledgeBase{ID: "kb-1", TenantID: 1, CorpID: 2, Name: "售后库", Description: "仅元数据", DocumentCount: 3, Status: 0, UpdatedBy: 17})
				return err
			},
		},
		{
			name:          "knowledge base delete",
			entityType:    "knowledge_base",
			entityID:      "kb-1",
			action:        "delete",
			actor:         17,
			changedFields: []string{"deleted_at", "updated_by"},
			expectBusiness: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_knowledge_bases SET deleted_at=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs(sqlmock.AnyArg(), int64(17), sqlmock.AnyArg(), "kb-1", int64(1), int64(2)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			call: func(db *sql.DB) error {
				repo, err := NewKnowledgeBaseRepository(db)
				if err != nil {
					return err
				}
				return repo.Delete(context.Background(), 1, 2, 17, "kb-1")
			},
		},
		{
			name:          "agent create",
			entityType:    "agent",
			entityID:      "agent-1",
			action:        "create",
			actor:         17,
			changedFields: []string{"name", "description", "knowledge_base_ids", "status"},
			expectBusiness: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_ai_agents (id, tenant_id, corp_id, name, description, knowledge_base_ids, status, created_by, updated_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)")).
					WithArgs("agent-1", int64(1), int64(2), "客服助手", "仅配置", `["kb-1"]`, 1, int64(17), int64(17), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectRead: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT "+agentColumns+" FROM mochat_go_ai_agents WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs("agent-1", int64(1), int64(2)).
					WillReturnRows(agentRows().AddRow("agent-1", 1, 2, "客服助手", "仅配置", `["kb-1"]`, 1, 17, 17, now, now))
			},
			call: func(db *sql.DB) error {
				repo, err := NewAgentRepository(db)
				if err != nil {
					return err
				}
				_, err = repo.Create(context.Background(), ports.Agent{ID: "agent-1", TenantID: 1, CorpID: 2, Name: "客服助手", Description: "仅配置", KnowledgeBaseIDs: []string{"kb-1"}, Status: 1, CreatedBy: 17, UpdatedBy: 17})
				return err
			},
		},
		{
			name:          "agent update",
			entityType:    "agent",
			entityID:      "agent-1",
			action:        "update",
			actor:         17,
			changedFields: []string{"name", "description", "knowledge_base_ids", "status"},
			expectBusiness: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_agents SET name=?, description=?, knowledge_base_ids=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs("客服助手", "仅配置", `["kb-1"]`, 0, int64(17), sqlmock.AnyArg(), "agent-1", int64(1), int64(2)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectRead: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT "+agentColumns+" FROM mochat_go_ai_agents WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs("agent-1", int64(1), int64(2)).
					WillReturnRows(agentRows().AddRow("agent-1", 1, 2, "客服助手", "仅配置", `["kb-1"]`, 0, 17, 17, now, now))
			},
			call: func(db *sql.DB) error {
				repo, err := NewAgentRepository(db)
				if err != nil {
					return err
				}
				_, err = repo.Update(context.Background(), ports.Agent{ID: "agent-1", TenantID: 1, CorpID: 2, Name: "客服助手", Description: "仅配置", KnowledgeBaseIDs: []string{"kb-1"}, Status: 0, UpdatedBy: 17})
				return err
			},
		},
		{
			name:          "agent delete",
			entityType:    "agent",
			entityID:      "agent-1",
			action:        "delete",
			actor:         17,
			changedFields: []string{"deleted_at", "updated_by"},
			expectBusiness: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_agents SET deleted_at=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WithArgs(sqlmock.AnyArg(), int64(17), sqlmock.AnyArg(), "agent-1", int64(1), int64(2)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			call: func(db *sql.DB) error {
				repo, err := NewAgentRepository(db)
				if err != nil {
					return err
				}
				return repo.Delete(context.Background(), 1, 2, 17, "agent-1")
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

			mock.ExpectBegin()
			test.expectBusiness(mock)
			mock.ExpectExec(regexp.QuoteMeta(auditInsertSQL)).
				WithArgs(int64(1), int64(2), test.actor, test.entityType, test.entityID, test.action, exactJSONStrings{want: test.changedFields}).
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectCommit()
			if test.expectRead != nil {
				test.expectRead(mock)
			}

			if err := test.call(db); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAISettingsRepositoriesRollbackAtomicMutationFailures(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(sqlmock.Sqlmock)
		call        func(*sql.DB) error
		expectError string
	}{
		{
			name: "knowledge base create business write failure rolls back",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_ai_knowledge_bases (id, tenant_id, corp_id, name, description, document_count, status, created_by, updated_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)")).
					WillReturnError(errors.New("knowledge base unavailable"))
				mock.ExpectRollback()
			},
			call: func(db *sql.DB) error {
				repo, _ := NewKnowledgeBaseRepository(db)
				_, err := repo.Create(context.Background(), ports.KnowledgeBase{ID: "kb-1", TenantID: 1, CorpID: 2, Name: "售后库", Status: 1, CreatedBy: 17, UpdatedBy: 17})
				return err
			},
			expectError: "knowledge base unavailable",
		},
		{
			name: "agent create business write failure rolls back",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_ai_agents (id, tenant_id, corp_id, name, description, knowledge_base_ids, status, created_by, updated_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)")).
					WillReturnError(errors.New("agent unavailable"))
				mock.ExpectRollback()
			},
			call: func(db *sql.DB) error {
				repo, _ := NewAgentRepository(db)
				_, err := repo.Create(context.Background(), ports.Agent{ID: "agent-1", TenantID: 1, CorpID: 2, Name: "客服助手", Status: 1, CreatedBy: 17, UpdatedBy: 17})
				return err
			},
			expectError: "agent unavailable",
		},
		{
			name: "knowledge base audit write failure rolls back",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_knowledge_bases SET name=?, description=?, document_count=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta(auditInsertSQL)).WillReturnError(errors.New("knowledge base audit unavailable"))
				mock.ExpectRollback()
			},
			call: func(db *sql.DB) error {
				repo, _ := NewKnowledgeBaseRepository(db)
				_, err := repo.Update(context.Background(), ports.KnowledgeBase{ID: "kb-1", TenantID: 1, CorpID: 2, Name: "售后库", Status: 1, UpdatedBy: 17})
				return err
			},
			expectError: "knowledge base audit unavailable",
		},
		{
			name: "agent audit write failure rolls back",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_agents SET name=?, description=?, knowledge_base_ids=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta(auditInsertSQL)).WillReturnError(errors.New("agent audit unavailable"))
				mock.ExpectRollback()
			},
			call: func(db *sql.DB) error {
				repo, _ := NewAgentRepository(db)
				_, err := repo.Update(context.Background(), ports.Agent{ID: "agent-1", TenantID: 1, CorpID: 2, Name: "客服助手", Status: 1, UpdatedBy: 17})
				return err
			},
			expectError: "agent audit unavailable",
		},
		{
			name: "knowledge base commit failure is returned",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_knowledge_bases SET deleted_at=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta(auditInsertSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit().WillReturnError(errors.New("knowledge base commit unavailable"))
			},
			call: func(db *sql.DB) error {
				repo, _ := NewKnowledgeBaseRepository(db)
				return repo.Delete(context.Background(), 1, 2, 17, "kb-1")
			},
			expectError: "knowledge base commit unavailable",
		},
		{
			name: "agent commit failure is returned",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_agents SET deleted_at=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta(auditInsertSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit().WillReturnError(errors.New("agent commit unavailable"))
			},
			call: func(db *sql.DB) error {
				repo, _ := NewAgentRepository(db)
				return repo.Delete(context.Background(), 1, 2, 17, "agent-1")
			},
			expectError: "agent commit unavailable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			test.setup(mock)

			err = test.call(db)
			if err == nil || err.Error() != test.expectError {
				t.Fatalf("error = %v, want %q", err, test.expectError)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAISettingsRepositoriesRollbackMissingUpdatesAndDeletes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(sqlmock.Sqlmock)
		call  func(*sql.DB) error
	}{
		{
			name: "knowledge base update",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_knowledge_bases SET name=?, description=?, document_count=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
			},
			call: func(db *sql.DB) error {
				repo, _ := NewKnowledgeBaseRepository(db)
				_, err := repo.Update(context.Background(), ports.KnowledgeBase{ID: "kb-missing", TenantID: 1, CorpID: 2, Name: "售后库", Status: 1, UpdatedBy: 17})
				return err
			},
		},
		{
			name: "knowledge base delete",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_knowledge_bases SET deleted_at=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
			},
			call: func(db *sql.DB) error {
				repo, _ := NewKnowledgeBaseRepository(db)
				return repo.Delete(context.Background(), 1, 2, 17, "kb-missing")
			},
		},
		{
			name: "agent update",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_agents SET name=?, description=?, knowledge_base_ids=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
			},
			call: func(db *sql.DB) error {
				repo, _ := NewAgentRepository(db)
				_, err := repo.Update(context.Background(), ports.Agent{ID: "agent-missing", TenantID: 1, CorpID: 2, Name: "客服助手", Status: 1, UpdatedBy: 17})
				return err
			},
		},
		{
			name: "agent delete",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_ai_agents SET deleted_at=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL")).
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
			},
			call: func(db *sql.DB) error {
				repo, _ := NewAgentRepository(db)
				return repo.Delete(context.Background(), 1, 2, 17, "agent-missing")
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
			test.setup(mock)

			if err := test.call(db); !errors.Is(err, ports.ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type exactJSONStrings struct {
	want []string
}

func (a exactJSONStrings) Match(value driver.Value) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	var got []string
	return json.Unmarshal([]byte(text), &got) == nil && reflect.DeepEqual(got, a.want)
}
