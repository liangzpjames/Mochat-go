package mysql

import (
	"context"
	"crypto/sha256"
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

const (
	sessionAgentColumns                = "id, tenant_id, corp_id, system_key, name, description, knowledge_base_ids, status, created_by, updated_by, created_at, updated_at"
	defaultSessionAssistantDescription = "分析企业微信会话中的客户意向、流失风险与员工服务质量。"
	maxRuntimeKnowledgeChunks          = 2000
)

func scanSessionAgent(row interface{ Scan(...any) error }) (ports.Agent, error) {
	var value ports.Agent
	var knowledgeBaseJSON string
	var createdAt, updatedAt time.Time
	if err := row.Scan(&value.ID, &value.TenantID, &value.CorpID, &value.SystemKey, &value.Name, &value.Description, &knowledgeBaseJSON, &value.Status, &value.CreatedBy, &value.UpdatedBy, &createdAt, &updatedAt); err != nil {
		return ports.Agent{}, err
	}
	if err := json.Unmarshal([]byte(knowledgeBaseJSON), &value.KnowledgeBaseIDs); err != nil {
		return ports.Agent{}, err
	}
	if value.KnowledgeBaseIDs == nil {
		value.KnowledgeBaseIDs = []string{}
	}
	value.CreatedAt = createdAt.Format(time.RFC3339)
	value.UpdatedAt = updatedAt.Format(time.RFC3339)
	return value, nil
}

func (r *AgentRepository) EnsureSessionAssistant(ctx context.Context, tenantID, corpID, actorUserID int64, id string) (ports.Agent, error) {
	if r == nil || r.db == nil {
		return ports.Agent{}, errors.New("AI settings agent database is unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.Agent{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_ai_agents
        (id,tenant_id,corp_id,system_key,name,description,knowledge_base_ids,status,created_by,updated_by,created_at,updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, tenantID, corpID, ports.SessionAnalysisSystemKey, ports.SessionAnalysisAssistantName, defaultSessionAssistantDescription, "[]", 1, actorUserID, actorUserID, now, now)
	if err != nil {
		return ports.Agent{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ports.Agent{}, err
	}
	if affected > 0 {
		if err := insertAudit(ctx, tx, tenantID, corpID, actorUserID, "agent", id, "create", []string{"system_key", "name", "description", "status"}); err != nil {
			return ports.Agent{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ports.Agent{}, err
	}
	return r.GetSessionAssistant(ctx, tenantID, corpID)
}

func (r *AgentRepository) GetSessionAssistant(ctx context.Context, tenantID, corpID int64) (ports.Agent, error) {
	value, err := scanSessionAgent(r.db.QueryRowContext(ctx, "SELECT "+sessionAgentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL", tenantID, corpID, ports.SessionAnalysisSystemKey))
	if errors.Is(err, sql.ErrNoRows) {
		return ports.Agent{}, ports.ErrNotFound
	}
	return value, err
}

func (r *AgentRepository) UpdateSessionAssistant(ctx context.Context, value ports.Agent) (ports.Agent, error) {
	value.Name = ports.SessionAnalysisAssistantName
	value.SystemKey = ports.SessionAnalysisSystemKey
	encoded, err := json.Marshal(nonNilStrings(value.KnowledgeBaseIDs))
	if err != nil {
		return ports.Agent{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.Agent{}, err
	}
	defer tx.Rollback()
	if err := lockKnowledgeBases(ctx, tx, value.TenantID, value.CorpID, value.KnowledgeBaseIDs); err != nil {
		return ports.Agent{}, err
	}
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, "UPDATE mochat_go_ai_agents SET name=?, description=?, knowledge_base_ids=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL", value.Name, value.Description, string(encoded), value.Status, value.UpdatedBy, now, value.ID, value.TenantID, value.CorpID, ports.SessionAnalysisSystemKey)
	if err != nil {
		return ports.Agent{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ports.Agent{}, err
	}
	if affected == 0 {
		return ports.Agent{}, ports.ErrNotFound
	}
	if err := insertAudit(ctx, tx, value.TenantID, value.CorpID, value.UpdatedBy, "agent", value.ID, "update", []string{"description", "knowledge_base_ids", "status"}); err != nil {
		return ports.Agent{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.Agent{}, err
	}
	return r.GetSessionAssistant(ctx, value.TenantID, value.CorpID)
}

func (r *AgentRepository) LoadSessionAssistantContext(ctx context.Context, tenantID, corpID int64) (ports.SessionAssistantContext, error) {
	agent, err := r.GetSessionAssistant(ctx, tenantID, corpID)
	if err != nil {
		return ports.SessionAssistantContext{}, err
	}
	hash := sha256.New()
	fmt.Fprintf(hash, "agent=%s\nstatus=%d\ndescription=%s\nupdated=%s\n", agent.ID, agent.Status, agent.Description, agent.UpdatedAt)
	contextValue := ports.SessionAssistantContext{AgentID: agent.ID, Name: agent.Name, Instructions: agent.Description, Enabled: agent.Status == 1, UpdatedAt: agent.UpdatedAt, KnowledgeChunks: []ports.KnowledgeChunk{}}
	if len(agent.KnowledgeBaseIDs) == 0 {
		contextValue.SettingsFingerprint = hex.EncodeToString(hash.Sum(nil))
		return contextValue, nil
	}
	ids := append([]string{}, agent.KnowledgeBaseIDs...)
	sort.Strings(ids)
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+2)
	args = append(args, tenantID, corpID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, "SELECT id,status,updated_at FROM mochat_go_ai_knowledge_bases WHERE tenant_id=? AND corp_id=? AND id IN ("+placeholders+") AND deleted_at IS NULL ORDER BY id", args...)
	if err != nil {
		return ports.SessionAssistantContext{}, err
	}
	enabledIDs := make([]string, 0, len(ids))
	for rows.Next() {
		var id string
		var status int
		var updatedAt time.Time
		if err := rows.Scan(&id, &status, &updatedAt); err != nil {
			rows.Close()
			return ports.SessionAssistantContext{}, err
		}
		fmt.Fprintf(hash, "kb=%s:%d:%s\n", id, status, updatedAt.UTC().Format(time.RFC3339Nano))
		if status == 1 {
			enabledIDs = append(enabledIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ports.SessionAssistantContext{}, err
	}
	rows.Close()
	contextValue.KnowledgeBaseCount = len(enabledIDs)
	if len(enabledIDs) == 0 || !contextValue.Enabled {
		contextValue.SettingsFingerprint = hex.EncodeToString(hash.Sum(nil))
		return contextValue, nil
	}
	chunkArgs := make([]any, 0, len(enabledIDs)+3)
	chunkArgs = append(chunkArgs, tenantID, corpID)
	for _, id := range enabledIDs {
		chunkArgs = append(chunkArgs, id)
	}
	chunkArgs = append(chunkArgs, maxRuntimeKnowledgeChunks)
	chunkPlaceholders := strings.TrimRight(strings.Repeat("?,", len(enabledIDs)), ",")
	chunkRows, err := r.db.QueryContext(ctx, `SELECT chunk.id,chunk.knowledge_base_id,chunk.document_id,document.filename,chunk.ordinal,chunk.content,chunk.character_count,document.sha256,document.updated_at
        FROM mochat_go_ai_knowledge_chunks chunk
        INNER JOIN mochat_go_ai_knowledge_documents document ON document.id=chunk.document_id AND document.tenant_id=chunk.tenant_id AND document.corp_id=chunk.corp_id AND document.knowledge_base_id=chunk.knowledge_base_id
        WHERE chunk.tenant_id=? AND chunk.corp_id=? AND chunk.knowledge_base_id IN (`+chunkPlaceholders+`) AND document.status='ready' AND document.deleted_at IS NULL
        ORDER BY chunk.knowledge_base_id,chunk.document_id,chunk.ordinal LIMIT ?`, chunkArgs...)
	if err != nil {
		return ports.SessionAssistantContext{}, err
	}
	documents := make(map[string]struct{})
	for chunkRows.Next() {
		var chunk ports.KnowledgeChunk
		var documentSHA string
		var documentUpdatedAt time.Time
		if err := chunkRows.Scan(&chunk.ID, &chunk.KnowledgeBaseID, &chunk.DocumentID, &chunk.DocumentName, &chunk.Ordinal, &chunk.Content, &chunk.CharacterCount, &documentSHA, &documentUpdatedAt); err != nil {
			chunkRows.Close()
			return ports.SessionAssistantContext{}, err
		}
		chunk.TenantID = tenantID
		chunk.CorpID = corpID
		contextValue.KnowledgeChunks = append(contextValue.KnowledgeChunks, chunk)
		if _, seen := documents[chunk.DocumentID]; !seen {
			documents[chunk.DocumentID] = struct{}{}
			fmt.Fprintf(hash, "doc=%s:%s:%s\n", chunk.DocumentID, documentSHA, documentUpdatedAt.UTC().Format(time.RFC3339Nano))
		}
	}
	if err := chunkRows.Err(); err != nil {
		chunkRows.Close()
		return ports.SessionAssistantContext{}, err
	}
	chunkRows.Close()
	contextValue.ReadyDocumentCount = len(documents)
	contextValue.SettingsFingerprint = hex.EncodeToString(hash.Sum(nil))
	return contextValue, nil
}
