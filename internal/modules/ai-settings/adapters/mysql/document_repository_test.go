package mysql

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

func TestDocumentRepositoryCreatesDocumentChunksCountAndAuditAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo, err := NewDocumentRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM mochat_go_ai_knowledge_bases WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL FOR UPDATE")).
		WithArgs("kb-1", int64(1), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("kb-1"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM mochat_go_ai_knowledge_documents WHERE tenant_id=? AND corp_id=? AND knowledge_base_id=? AND deleted_at IS NULL")).
		WithArgs(int64(1), int64(2), "kb-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM mochat_go_ai_knowledge_documents WHERE tenant_id=? AND corp_id=? AND knowledge_base_id=? AND sha256=? AND deleted_at IS NULL")).
		WithArgs(int64(1), int64(2), "kb-1", "checksum").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO mochat_go_ai_knowledge_documents").
		WithArgs("doc-1", int64(1), int64(2), "kb-1", "guide.txt", "txt", "text/plain", "private/key", int64(17), "checksum", "ready", "", 4, 1, int64(7), int64(7), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_knowledge_chunks").
		WithArgs(int64(1), int64(2), "kb-1", "doc-1", 0, "退款审批", 4).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE mochat_go_ai_knowledge_bases SET document_count=\\(SELECT COUNT\\(\\*\\)").
		WithArgs(int64(1), int64(2), "kb-1", sqlmock.AnyArg(), int64(7), "kb-1", int64(1), int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").
		WithArgs(int64(1), int64(2), int64(7), "knowledge_document", "doc-1", "create", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	document := ports.KnowledgeDocument{ID: "doc-1", TenantID: 1, CorpID: 2, KnowledgeBaseID: "kb-1", Filename: "guide.txt", Extension: "txt", MIMEType: "text/plain", ObjectKey: "private/key", SizeBytes: 17, SHA256: "checksum", Status: ports.DocumentStatusReady, CharacterCount: 4, ChunkCount: 1, CreatedBy: 7, UpdatedBy: 7}
	chunks := []ports.KnowledgeChunk{{TenantID: 1, CorpID: 2, KnowledgeBaseID: "kb-1", DocumentID: "doc-1", Ordinal: 0, Content: "退款审批", CharacterCount: 4}}
	created, err := repo.Create(context.Background(), document, chunks)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "doc-1" || created.CreatedAt == "" {
		t.Fatalf("created = %#v", created)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentRepositoryRejectsLimitAndRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo, _ := NewDocumentRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM mochat_go_ai_knowledge_bases").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("kb-1"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM mochat_go_ai_knowledge_documents").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	mock.ExpectRollback()
	_, err = repo.Create(context.Background(), ports.KnowledgeDocument{ID: "doc", TenantID: 1, CorpID: 2, KnowledgeBaseID: "kb-1"}, nil)
	if !errors.Is(err, ports.ErrDocumentLimit) {
		t.Fatalf("error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentRepositoryDeleteIsScopedAndTransactional(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo, _ := NewDocumentRepository(db)
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM mochat_go_ai_knowledge_documents WHERE id=\\? AND tenant_id=\\? AND corp_id=\\? AND knowledge_base_id=\\? AND deleted_at IS NULL FOR UPDATE").
		WithArgs("doc-1", int64(1), int64(2), "kb-1").
		WillReturnRows(sqlmock.NewRows(documentColumnNames()).AddRow("doc-1", 1, 2, "kb-1", "guide.txt", "txt", "text/plain", "private/key", 17, "checksum", "ready", "", 4, 1, 7, 7, now, now))
	mock.ExpectExec("DELETE FROM mochat_go_ai_knowledge_chunks").WithArgs(int64(1), int64(2), "kb-1", "doc-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE mochat_go_ai_knowledge_documents SET deleted_at=").WithArgs(sqlmock.AnyArg(), int64(7), sqlmock.AnyArg(), "doc-1", int64(1), int64(2), "kb-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE mochat_go_ai_knowledge_bases SET document_count=\\(SELECT COUNT\\(\\*\\)").WithArgs(int64(1), int64(2), "kb-1", sqlmock.AnyArg(), int64(7), "kb-1", int64(1), int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO mochat_go_ai_settings_audits").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	deleted, err := repo.Delete(context.Background(), 1, 2, 7, "kb-1", "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	if deleted.ObjectKey != "private/key" {
		t.Fatalf("deleted = %#v", deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentRepositoryMapsMissingDocument(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo, _ := NewDocumentRepository(db)
	mock.ExpectQuery("SELECT .* FROM mochat_go_ai_knowledge_documents").WillReturnError(sql.ErrNoRows)
	_, err := repo.Get(context.Background(), 1, 2, "kb-1", "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}
