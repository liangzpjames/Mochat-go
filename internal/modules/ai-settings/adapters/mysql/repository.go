package mysql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

type KnowledgeBaseRepository struct {
	db *sql.DB
}

func NewKnowledgeBaseRepository(db *sql.DB) (*KnowledgeBaseRepository, error) {
	if db == nil {
		return nil, errors.New("AI settings database is required")
	}
	return &KnowledgeBaseRepository{db: db}, nil
}

const kbColumns = "id, tenant_id, corp_id, name, description, document_count, status, created_by, updated_by, created_at, updated_at"

func scanKnowledgeBase(row interface{ Scan(...any) error }) (ports.KnowledgeBase, error) {
	var v ports.KnowledgeBase
	var createdAt, updatedAt time.Time
	err := row.Scan(&v.ID, &v.TenantID, &v.CorpID, &v.Name, &v.Description, &v.DocumentCount, &v.Status, &v.CreatedBy, &v.UpdatedBy, &createdAt, &updatedAt)
	if err != nil {
		return ports.KnowledgeBase{}, err
	}
	v.CreatedAt = createdAt.Format(time.RFC3339)
	v.UpdatedAt = updatedAt.Format(time.RFC3339)
	return v, nil
}

func (r *KnowledgeBaseRepository) List(ctx context.Context, tenantID, corpID int64) ([]ports.KnowledgeBase, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+kbColumns+" FROM mochat_go_ai_knowledge_bases WHERE tenant_id=? AND corp_id=? AND deleted_at IS NULL ORDER BY updated_at DESC", tenantID, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ports.KnowledgeBase, 0)
	for rows.Next() {
		v, err := scanKnowledgeBase(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (r *KnowledgeBaseRepository) GetByIDs(ctx context.Context, tenantID, corpID int64, ids []string) ([]ports.KnowledgeBase, error) {
	if len(ids) == 0 {
		return []ports.KnowledgeBase{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+2)
	args = append(args, tenantID, corpID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, "SELECT "+kbColumns+" FROM mochat_go_ai_knowledge_bases WHERE tenant_id=? AND corp_id=? AND id IN ("+placeholders+") AND deleted_at IS NULL", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ports.KnowledgeBase, 0, len(ids))
	for rows.Next() {
		v, err := scanKnowledgeBase(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (r *KnowledgeBaseRepository) Create(ctx context.Context, v ports.KnowledgeBase) (ports.KnowledgeBase, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.KnowledgeBase{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx,
		"INSERT INTO mochat_go_ai_knowledge_bases (id, tenant_id, corp_id, name, description, document_count, status, created_by, updated_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
		v.ID, v.TenantID, v.CorpID, v.Name, v.Description, v.DocumentCount, v.Status, v.CreatedBy, v.UpdatedBy, now, now)
	if err != nil {
		return ports.KnowledgeBase{}, err
	}
	if err := insertAudit(ctx, tx, v.TenantID, v.CorpID, v.CreatedBy, "knowledge_base", v.ID, "create", []string{"name", "description", "document_count", "status"}); err != nil {
		return ports.KnowledgeBase{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.KnowledgeBase{}, err
	}
	return r.getByID(ctx, v.TenantID, v.CorpID, v.ID)
}

func (r *KnowledgeBaseRepository) Update(ctx context.Context, v ports.KnowledgeBase) (ports.KnowledgeBase, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.KnowledgeBase{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		"UPDATE mochat_go_ai_knowledge_bases SET name=?, description=?, document_count=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL",
		v.Name, v.Description, v.DocumentCount, v.Status, v.UpdatedBy, now, v.ID, v.TenantID, v.CorpID)
	if err != nil {
		return ports.KnowledgeBase{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return ports.KnowledgeBase{}, err
	}
	if affected == 0 {
		return ports.KnowledgeBase{}, ports.ErrNotFound
	}
	if err := insertAudit(ctx, tx, v.TenantID, v.CorpID, v.UpdatedBy, "knowledge_base", v.ID, "update", []string{"name", "description", "document_count", "status"}); err != nil {
		return ports.KnowledgeBase{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.KnowledgeBase{}, err
	}
	return r.getByID(ctx, v.TenantID, v.CorpID, v.ID)
}

func (r *KnowledgeBaseRepository) Delete(ctx context.Context, tenantID, corpID, actorUserID int64, id string) error {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var lockedID string
	if err := tx.QueryRowContext(ctx,
		"SELECT id FROM mochat_go_ai_knowledge_bases WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL FOR UPDATE",
		id, tenantID, corpID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.ErrNotFound
		}
		return err
	}
	var referenceCount int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND JSON_CONTAINS(knowledge_base_ids, JSON_QUOTE(?)) AND deleted_at IS NULL",
		tenantID, corpID, id).Scan(&referenceCount); err != nil {
		return err
	}
	if referenceCount > 0 {
		return &ports.KnowledgeBaseReferencedError{Count: referenceCount}
	}
	res, err := tx.ExecContext(ctx,
		"UPDATE mochat_go_ai_knowledge_bases SET deleted_at=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL",
		now, actorUserID, now, id, tenantID, corpID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ports.ErrNotFound
	}
	if err := insertAudit(ctx, tx, tenantID, corpID, actorUserID, "knowledge_base", id, "delete", []string{"deleted_at", "updated_by"}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *KnowledgeBaseRepository) getByID(ctx context.Context, tenantID, corpID int64, id string) (ports.KnowledgeBase, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+kbColumns+" FROM mochat_go_ai_knowledge_bases WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL", id, tenantID, corpID)
	return scanKnowledgeBase(row)
}

type AgentRepository struct {
	db *sql.DB
}

func NewAgentRepository(db *sql.DB) (*AgentRepository, error) {
	if db == nil {
		return nil, errors.New("AI settings database is required")
	}
	return &AgentRepository{db: db}, nil
}

const agentColumns = "id, tenant_id, corp_id, name, description, knowledge_base_ids, status, created_by, updated_by, created_at, updated_at"

func scanAgent(row interface{ Scan(...any) error }) (ports.Agent, error) {
	var v ports.Agent
	var kbIDs string
	var createdAt, updatedAt time.Time
	err := row.Scan(&v.ID, &v.TenantID, &v.CorpID, &v.Name, &v.Description, &kbIDs, &v.Status, &v.CreatedBy, &v.UpdatedBy, &createdAt, &updatedAt)
	if err != nil {
		return ports.Agent{}, err
	}
	if err := json.Unmarshal([]byte(kbIDs), &v.KnowledgeBaseIDs); err != nil {
		return ports.Agent{}, err
	}
	if v.KnowledgeBaseIDs == nil {
		v.KnowledgeBaseIDs = []string{}
	}
	v.CreatedAt = createdAt.Format(time.RFC3339)
	v.UpdatedAt = updatedAt.Format(time.RFC3339)
	return v, nil
}

func (r *AgentRepository) List(ctx context.Context, tenantID, corpID int64) ([]ports.Agent, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+agentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND deleted_at IS NULL ORDER BY updated_at DESC", tenantID, corpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ports.Agent, 0)
	for rows.Next() {
		v, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (r *AgentRepository) ListReferencingKnowledgeBase(ctx context.Context, tenantID, corpID int64, knowledgeBaseID string) ([]ports.Agent, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+agentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND JSON_CONTAINS(knowledge_base_ids, JSON_QUOTE(?)) AND deleted_at IS NULL ORDER BY updated_at DESC", tenantID, corpID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ports.Agent, 0)
	for rows.Next() {
		v, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (r *AgentRepository) Create(ctx context.Context, v ports.Agent) (ports.Agent, error) {
	now := time.Now().UTC()
	kbJSON, err := json.Marshal(nonNilStrings(v.KnowledgeBaseIDs))
	if err != nil {
		return ports.Agent{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.Agent{}, err
	}
	defer tx.Rollback()
	if err := lockKnowledgeBases(ctx, tx, v.TenantID, v.CorpID, v.KnowledgeBaseIDs); err != nil {
		return ports.Agent{}, err
	}
	_, err = tx.ExecContext(ctx,
		"INSERT INTO mochat_go_ai_agents (id, tenant_id, corp_id, name, description, knowledge_base_ids, status, created_by, updated_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
		v.ID, v.TenantID, v.CorpID, v.Name, v.Description, string(kbJSON), v.Status, v.CreatedBy, v.UpdatedBy, now, now)
	if err != nil {
		return ports.Agent{}, err
	}
	if err := insertAudit(ctx, tx, v.TenantID, v.CorpID, v.CreatedBy, "agent", v.ID, "create", []string{"name", "description", "knowledge_base_ids", "status"}); err != nil {
		return ports.Agent{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.Agent{}, err
	}
	return r.getByID(ctx, v.TenantID, v.CorpID, v.ID)
}

func (r *AgentRepository) Update(ctx context.Context, v ports.Agent) (ports.Agent, error) {
	now := time.Now().UTC()
	kbJSON, err := json.Marshal(nonNilStrings(v.KnowledgeBaseIDs))
	if err != nil {
		return ports.Agent{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.Agent{}, err
	}
	defer tx.Rollback()
	if err := lockKnowledgeBases(ctx, tx, v.TenantID, v.CorpID, v.KnowledgeBaseIDs); err != nil {
		return ports.Agent{}, err
	}
	res, err := tx.ExecContext(ctx,
		"UPDATE mochat_go_ai_agents SET name=?, description=?, knowledge_base_ids=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL",
		v.Name, v.Description, string(kbJSON), v.Status, v.UpdatedBy, now, v.ID, v.TenantID, v.CorpID)
	if err != nil {
		return ports.Agent{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return ports.Agent{}, err
	}
	if affected == 0 {
		return ports.Agent{}, ports.ErrNotFound
	}
	if err := insertAudit(ctx, tx, v.TenantID, v.CorpID, v.UpdatedBy, "agent", v.ID, "update", []string{"name", "description", "knowledge_base_ids", "status"}); err != nil {
		return ports.Agent{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.Agent{}, err
	}
	return r.getByID(ctx, v.TenantID, v.CorpID, v.ID)
}

func (r *AgentRepository) Delete(ctx context.Context, tenantID, corpID, actorUserID int64, id string) error {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		"UPDATE mochat_go_ai_agents SET deleted_at=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL",
		now, actorUserID, now, id, tenantID, corpID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ports.ErrNotFound
	}
	if err := insertAudit(ctx, tx, tenantID, corpID, actorUserID, "agent", id, "delete", []string{"deleted_at", "updated_by"}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *AgentRepository) getByID(ctx context.Context, tenantID, corpID int64, id string) (ports.Agent, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+agentColumns+" FROM mochat_go_ai_agents WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL", id, tenantID, corpID)
	return scanAgent(row)
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func lockKnowledgeBases(ctx context.Context, tx *sql.Tx, tenantID, corpID int64, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	unique := make(map[string]struct{}, len(ids))
	ordered := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ordered)), ",")
	args := make([]any, 0, len(ordered)+2)
	args = append(args, tenantID, corpID)
	for _, id := range ordered {
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx,
		"SELECT id FROM mochat_go_ai_knowledge_bases WHERE tenant_id=? AND corp_id=? AND id IN ("+placeholders+") AND deleted_at IS NULL ORDER BY id FOR UPDATE",
		args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != len(ordered) {
		return ports.ErrKnowledgeBaseInvalid
	}
	return nil
}

func insertAudit(ctx context.Context, tx *sql.Tx, tenantID, corpID, actorUserID int64, entityType, entityID, action string, changedFields []string) error {
	encoded, err := json.Marshal(changedFields)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		"INSERT INTO mochat_go_ai_settings_audits (tenant_id, corp_id, actor_user_id, entity_type, entity_id, action, changed_fields) VALUES (?,?,?,?,?,?,?)",
		tenantID, corpID, actorUserID, entityType, entityID, action, string(encoded))
	return err
}

func NewIDGenerator() func() string {
	return func() string {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			return fmt.Sprintf("%d", time.Now().UnixNano())
		}
		return hex.EncodeToString(buf)
	}
}
