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
	defaultSmartAnalysisObjective      = "识别客户意向、沟通质量、风险信号和建议跟进动作"
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
	_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_ai_analysis_rules
        (tenant_id,corp_id,system_key,name,objective,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,status,current_version,created_by,updated_by,created_at,updated_at)
        VALUES (?,?,?,?,?,?,'all','[]',?,?,'enabled',1,?,?,?,?)`,
		tenantID, corpID, ports.DefaultSmartAnalysisSystemKey, ports.DefaultSmartAnalysisRuleName,
		defaultSmartAnalysisObjective, `["direct","group"]`, 30, 2, actorUserID, actorUserID, now, now)
	if err != nil {
		return ports.Agent{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_ai_analysis_rule_versions
        (tenant_id,corp_id,rule_id,version,objective,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,created_by,created_at)
        SELECT tenant_id,corp_id,id,current_version,objective,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,updated_by,NOW()
        FROM mochat_go_ai_analysis_rules WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL`, tenantID, corpID, ports.DefaultSmartAnalysisSystemKey)
	if err != nil {
		return ports.Agent{}, err
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
	if err != nil {
		return ports.Agent{}, err
	}
	rule, err := loadDefaultSmartAnalysisRule(ctx, r.db, tenantID, corpID)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.Agent{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.Agent{}, err
	}
	value.SmartAnalysisRule = &rule
	return value, nil
}

func loadDefaultSmartAnalysisRule(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, tenantID, corpID int64) (ports.SmartAnalysisRule, error) {
	var value ports.SmartAnalysisRule
	var conversationTypesJSON string
	var updatedAt time.Time
	err := queryer.QueryRowContext(ctx, `SELECT id,name,objective,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at
        FROM mochat_go_ai_analysis_rules
        WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL`, tenantID, corpID, ports.DefaultSmartAnalysisSystemKey).
		Scan(&value.ID, &value.Name, &value.Objective, &conversationTypesJSON, &value.LookbackDays, &value.MinimumMessages, &value.CurrentVersion, &updatedAt)
	if err != nil {
		return ports.SmartAnalysisRule{}, err
	}
	if err := json.Unmarshal([]byte(conversationTypesJSON), &value.ConversationTypes); err != nil {
		return ports.SmartAnalysisRule{}, err
	}
	if value.ConversationTypes == nil {
		value.ConversationTypes = []string{}
	}
	value.UpdatedAt = updatedAt.Format(time.RFC3339)
	return value, nil
}

func (r *AgentRepository) UpdateSessionAssistant(ctx context.Context, value ports.Agent) (ports.Agent, error) {
	value.Name = ports.SessionAnalysisAssistantName
	value.SystemKey = ports.SessionAnalysisSystemKey
	if value.SmartAnalysisRule == nil {
		return ports.Agent{}, errors.New("default smart analysis rule is required")
	}
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
	var ruleID int64
	var currentObjective, currentTypesJSON string
	var currentLookbackDays, currentMinimumMessages, currentVersion int
	err = tx.QueryRowContext(ctx, `SELECT id,objective,conversation_types_json,lookback_days,minimum_messages,current_version
        FROM mochat_go_ai_analysis_rules
        WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL FOR UPDATE`, value.TenantID, value.CorpID, ports.DefaultSmartAnalysisSystemKey).
		Scan(&ruleID, &currentObjective, &currentTypesJSON, &currentLookbackDays, &currentMinimumMessages, &currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.Agent{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.Agent{}, err
	}
	typesJSON, err := json.Marshal(nonNilStrings(value.SmartAnalysisRule.ConversationTypes))
	if err != nil {
		return ports.Agent{}, err
	}
	ruleChanged := currentObjective != value.SmartAnalysisRule.Objective || currentTypesJSON != string(typesJSON) || currentLookbackDays != value.SmartAnalysisRule.LookbackDays || currentMinimumMessages != value.SmartAnalysisRule.MinimumMessages
	if ruleChanged {
		nextVersion := currentVersion + 1
		result, err := tx.ExecContext(ctx, `UPDATE mochat_go_ai_analysis_rules
            SET objective=?,conversation_types_json=?,lookback_days=?,minimum_messages=?,current_version=?,updated_by=?,updated_at=?
            WHERE tenant_id=? AND corp_id=? AND id=? AND system_key=? AND deleted_at IS NULL`,
			value.SmartAnalysisRule.Objective, string(typesJSON), value.SmartAnalysisRule.LookbackDays, value.SmartAnalysisRule.MinimumMessages,
			nextVersion, value.UpdatedBy, now, value.TenantID, value.CorpID, ruleID, ports.DefaultSmartAnalysisSystemKey)
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
		_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_ai_analysis_rule_versions
            (tenant_id,corp_id,rule_id,version,objective,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,created_by,created_at)
            VALUES (?,?,?,?,?,?,'all','[]',?,?,?,?)`, value.TenantID, value.CorpID, ruleID, nextVersion,
			value.SmartAnalysisRule.Objective, string(typesJSON), value.SmartAnalysisRule.LookbackDays, value.SmartAnalysisRule.MinimumMessages, value.UpdatedBy, now)
		if err != nil {
			return ports.Agent{}, err
		}
	}
	if err := insertAudit(ctx, tx, value.TenantID, value.CorpID, value.UpdatedBy, "agent", value.ID, "update", []string{"description", "knowledge_base_ids", "status"}); err != nil {
		return ports.Agent{}, err
	}
	if ruleChanged {
		if err := insertAudit(ctx, tx, value.TenantID, value.CorpID, value.UpdatedBy, "analysis_rule", fmt.Sprintf("%d", ruleID), "update", []string{"objective", "conversation_types", "lookback_days", "minimum_messages", "current_version"}); err != nil {
			return ports.Agent{}, err
		}
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
