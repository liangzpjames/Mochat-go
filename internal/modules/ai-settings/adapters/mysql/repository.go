package mysql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO mochat_go_ai_knowledge_bases (id, tenant_id, corp_id, name, description, document_count, status, created_by, updated_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
		v.ID, v.TenantID, v.CorpID, v.Name, v.Description, v.DocumentCount, v.Status, v.CreatedBy, v.UpdatedBy, now, now)
	if err != nil {
		return ports.KnowledgeBase{}, err
	}
	return r.getByID(ctx, v.TenantID, v.CorpID, v.ID)
}

func (r *KnowledgeBaseRepository) Update(ctx context.Context, v ports.KnowledgeBase) (ports.KnowledgeBase, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		"UPDATE mochat_go_ai_knowledge_bases SET name=?, description=?, document_count=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL",
		v.Name, v.Description, v.DocumentCount, v.Status, v.UpdatedBy, now, v.ID, v.TenantID, v.CorpID)
	if err != nil {
		return ports.KnowledgeBase{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ports.KnowledgeBase{}, errors.New("knowledge base not found or not scoped to this tenant/corp")
	}
	return r.getByID(ctx, v.TenantID, v.CorpID, v.ID)
}

func (r *KnowledgeBaseRepository) Delete(ctx context.Context, tenantID, corpID int64, id string) error {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		"UPDATE mochat_go_ai_knowledge_bases SET deleted_at=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL",
		now, now, id, tenantID, corpID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("knowledge base not found or already deleted")
	}
	return nil
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
	_ = json.Unmarshal([]byte(kbIDs), &v.KnowledgeBaseIDs)
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
	_, err = r.db.ExecContext(ctx,
		"INSERT INTO mochat_go_ai_agents (id, tenant_id, corp_id, name, description, knowledge_base_ids, status, created_by, updated_by, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
		v.ID, v.TenantID, v.CorpID, v.Name, v.Description, string(kbJSON), v.Status, v.CreatedBy, v.UpdatedBy, now, now)
	if err != nil {
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
	res, err := r.db.ExecContext(ctx,
		"UPDATE mochat_go_ai_agents SET name=?, description=?, knowledge_base_ids=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL",
		v.Name, v.Description, string(kbJSON), v.Status, v.UpdatedBy, now, v.ID, v.TenantID, v.CorpID)
	if err != nil {
		return ports.Agent{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ports.Agent{}, errors.New("agent not found or not scoped to this tenant/corp")
	}
	return r.getByID(ctx, v.TenantID, v.CorpID, v.ID)
}

func (r *AgentRepository) Delete(ctx context.Context, tenantID, corpID int64, id string) error {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		"UPDATE mochat_go_ai_agents SET deleted_at=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND deleted_at IS NULL",
		now, now, id, tenantID, corpID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("agent not found or already deleted")
	}
	return nil
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

func NewIDGenerator() func() string {
	return func() string {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			return fmt.Sprintf("%d", time.Now().UnixNano())
		}
		return hex.EncodeToString(buf)
	}
}
