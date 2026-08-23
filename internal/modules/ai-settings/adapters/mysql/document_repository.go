package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

const maxKnowledgeBaseDocuments = 100

const documentColumns = "id, tenant_id, corp_id, knowledge_base_id, filename, extension, mime_type, object_key, size_bytes, sha256, status, error_summary, character_count, chunk_count, created_by, updated_by, created_at, updated_at"

type DocumentRepository struct {
	db *sql.DB
}

func NewDocumentRepository(db *sql.DB) (*DocumentRepository, error) {
	if db == nil {
		return nil, errors.New("AI settings document database is required")
	}
	return &DocumentRepository{db: db}, nil
}

func documentColumnNames() []string {
	return []string{"id", "tenant_id", "corp_id", "knowledge_base_id", "filename", "extension", "mime_type", "object_key", "size_bytes", "sha256", "status", "error_summary", "character_count", "chunk_count", "created_by", "updated_by", "created_at", "updated_at"}
}

func scanKnowledgeDocument(row interface{ Scan(...any) error }) (ports.KnowledgeDocument, error) {
	var value ports.KnowledgeDocument
	var createdAt, updatedAt time.Time
	if err := row.Scan(
		&value.ID, &value.TenantID, &value.CorpID, &value.KnowledgeBaseID,
		&value.Filename, &value.Extension, &value.MIMEType, &value.ObjectKey,
		&value.SizeBytes, &value.SHA256, &value.Status, &value.ErrorSummary,
		&value.CharacterCount, &value.ChunkCount, &value.CreatedBy, &value.UpdatedBy,
		&createdAt, &updatedAt,
	); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	value.CreatedAt = createdAt.Format(time.RFC3339)
	value.UpdatedAt = updatedAt.Format(time.RFC3339)
	return value, nil
}

func (r *DocumentRepository) List(ctx context.Context, tenantID, corpID int64, knowledgeBaseID string) ([]ports.KnowledgeDocument, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+documentColumns+" FROM mochat_go_ai_knowledge_documents WHERE tenant_id=? AND corp_id=? AND knowledge_base_id=? AND deleted_at IS NULL ORDER BY created_at DESC, id DESC", tenantID, corpID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ports.KnowledgeDocument, 0)
	for rows.Next() {
		value, err := scanKnowledgeDocument(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (r *DocumentRepository) Get(ctx context.Context, tenantID, corpID int64, knowledgeBaseID, documentID string) (ports.KnowledgeDocument, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+documentColumns+" FROM mochat_go_ai_knowledge_documents WHERE id=? AND tenant_id=? AND corp_id=? AND knowledge_base_id=? AND deleted_at IS NULL", documentID, tenantID, corpID, knowledgeBaseID)
	value, err := scanKnowledgeDocument(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.KnowledgeDocument{}, ports.ErrNotFound
	}
	return value, err
}

func (r *DocumentRepository) Count(ctx context.Context, tenantID, corpID int64, knowledgeBaseID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_ai_knowledge_documents WHERE tenant_id=? AND corp_id=? AND knowledge_base_id=? AND deleted_at IS NULL", tenantID, corpID, knowledgeBaseID).Scan(&count)
	return count, err
}

func (r *DocumentRepository) Create(ctx context.Context, document ports.KnowledgeDocument, chunks []ports.KnowledgeChunk) (ports.KnowledgeDocument, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.KnowledgeDocument{}, err
	}
	defer tx.Rollback()
	var lockedID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM mochat_go_ai_knowledge_bases WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL FOR UPDATE", document.KnowledgeBaseID, document.TenantID, document.CorpID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.KnowledgeDocument{}, ports.ErrNotFound
		}
		return ports.KnowledgeDocument{}, err
	}
	var documentCount int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_ai_knowledge_documents WHERE tenant_id=? AND corp_id=? AND knowledge_base_id=? AND deleted_at IS NULL", document.TenantID, document.CorpID, document.KnowledgeBaseID).Scan(&documentCount); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	if documentCount >= maxKnowledgeBaseDocuments {
		return ports.KnowledgeDocument{}, ports.ErrDocumentLimit
	}
	var duplicateCount int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_ai_knowledge_documents WHERE tenant_id=? AND corp_id=? AND knowledge_base_id=? AND sha256=? AND deleted_at IS NULL", document.TenantID, document.CorpID, document.KnowledgeBaseID, document.SHA256).Scan(&duplicateCount); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	if duplicateCount > 0 {
		return ports.KnowledgeDocument{}, ports.ErrDocumentDuplicate
	}
	document.ChunkCount = len(chunks)
	if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_ai_knowledge_documents
        (id,tenant_id,corp_id,knowledge_base_id,filename,extension,mime_type,object_key,size_bytes,sha256,status,error_summary,character_count,chunk_count,created_by,updated_by,created_at,updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		document.ID, document.TenantID, document.CorpID, document.KnowledgeBaseID, document.Filename, document.Extension, document.MIMEType, document.ObjectKey,
		document.SizeBytes, document.SHA256, document.Status, document.ErrorSummary, document.CharacterCount, document.ChunkCount, document.CreatedBy, document.UpdatedBy, now, now); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	for _, chunk := range chunks {
		if _, err := tx.ExecContext(ctx, "INSERT INTO mochat_go_ai_knowledge_chunks (tenant_id,corp_id,knowledge_base_id,document_id,ordinal,content,character_count) VALUES (?,?,?,?,?,?,?)", document.TenantID, document.CorpID, document.KnowledgeBaseID, document.ID, chunk.Ordinal, chunk.Content, chunk.CharacterCount); err != nil {
			return ports.KnowledgeDocument{}, err
		}
	}
	if err := updateKnowledgeBaseDocumentCount(ctx, tx, document.TenantID, document.CorpID, document.KnowledgeBaseID, document.UpdatedBy, now); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	if err := insertAudit(ctx, tx, document.TenantID, document.CorpID, document.CreatedBy, "knowledge_document", document.ID, "create", []string{"filename", "status", "size_bytes", "sha256", "character_count", "chunk_count"}); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	document.CreatedAt = now.Format(time.RFC3339)
	document.UpdatedAt = now.Format(time.RFC3339)
	return document, nil
}

func (r *DocumentRepository) Delete(ctx context.Context, tenantID, corpID, actorUserID int64, knowledgeBaseID, documentID string) (ports.KnowledgeDocument, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.KnowledgeDocument{}, err
	}
	defer tx.Rollback()
	document, err := scanKnowledgeDocument(tx.QueryRowContext(ctx, "SELECT "+documentColumns+" FROM mochat_go_ai_knowledge_documents WHERE id=? AND tenant_id=? AND corp_id=? AND knowledge_base_id=? AND deleted_at IS NULL FOR UPDATE", documentID, tenantID, corpID, knowledgeBaseID))
	if errors.Is(err, sql.ErrNoRows) {
		return ports.KnowledgeDocument{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.KnowledgeDocument{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM mochat_go_ai_knowledge_chunks WHERE tenant_id=? AND corp_id=? AND knowledge_base_id=? AND document_id=?", tenantID, corpID, knowledgeBaseID, documentID); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	result, err := tx.ExecContext(ctx, "UPDATE mochat_go_ai_knowledge_documents SET deleted_at=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND knowledge_base_id=? AND deleted_at IS NULL", now, actorUserID, now, documentID, tenantID, corpID, knowledgeBaseID)
	if err != nil {
		return ports.KnowledgeDocument{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ports.KnowledgeDocument{}, err
	}
	if affected == 0 {
		return ports.KnowledgeDocument{}, ports.ErrNotFound
	}
	if err := updateKnowledgeBaseDocumentCount(ctx, tx, tenantID, corpID, knowledgeBaseID, actorUserID, now); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	if err := insertAudit(ctx, tx, tenantID, corpID, actorUserID, "knowledge_document", documentID, "delete", []string{"deleted_at", "updated_by"}); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.KnowledgeDocument{}, err
	}
	return document, nil
}

func updateKnowledgeBaseDocumentCount(ctx context.Context, tx *sql.Tx, tenantID, corpID int64, knowledgeBaseID string, actorUserID int64, now time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE mochat_go_ai_knowledge_bases SET document_count=(SELECT COUNT(*) FROM mochat_go_ai_knowledge_documents WHERE tenant_id=? AND corp_id=? AND knowledge_base_id=? AND deleted_at IS NULL), updated_at=?, updated_by=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL`, tenantID, corpID, knowledgeBaseID, now, actorUserID, knowledgeBaseID, tenantID, corpID)
	return err
}
