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
	defaultSmartAssistantDescription   = "根据企业微信会话生成可追溯的智能分析结论。"
	defaultSessionAnalysisObjective    = "分析客户会话并提供客户经营与员工服务改进建议。"
	defaultCustomerAnalysisPrompt      = "请基于会话内容分析客户购买意向、流失风险和核心需求，说明证据并给出下一步跟进建议。"
	defaultEmployeeQAPrompt            = "请基于会话内容完成员工服务质检，识别客户异议并给出可执行的改进建议。"
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

type systemAssistantSpec struct {
	id          string
	systemKey   string
	name        string
	description string
}

func (r *AgentRepository) EnsureSystemAssistants(ctx context.Context, tenantID, corpID, actorUserID int64, sessionID, smartID string) ([]ports.Agent, error) {
	return r.ensureSystemAssistants(ctx, tenantID, corpID, actorUserID, []systemAssistantSpec{
		{id: sessionID, systemKey: ports.SessionAnalysisSystemKey, name: ports.SessionAnalysisAssistantName, description: defaultSessionAssistantDescription},
		{id: smartID, systemKey: ports.SmartAnalysisSystemKey, name: ports.SmartAnalysisAssistantName, description: defaultSmartAssistantDescription},
	})
}

// EnsureSessionAssistant preserves the ai-insight contract while the runner is
// migrated to select a system assistant explicitly.
func (r *AgentRepository) EnsureSessionAssistant(ctx context.Context, tenantID, corpID, actorUserID int64, id string) (ports.Agent, error) {
	values, err := r.ensureSystemAssistants(ctx, tenantID, corpID, actorUserID, []systemAssistantSpec{{id: id, systemKey: ports.SessionAnalysisSystemKey, name: ports.SessionAnalysisAssistantName, description: defaultSessionAssistantDescription}})
	if err != nil {
		return ports.Agent{}, err
	}
	return values[0], nil
}

func (r *AgentRepository) ensureSystemAssistants(ctx context.Context, tenantID, corpID, actorUserID int64, specs []systemAssistantSpec) ([]ports.Agent, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("AI settings agent database is unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	for _, spec := range specs {
		result, err := tx.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_ai_agents
            (id,tenant_id,corp_id,system_key,name,description,knowledge_base_ids,status,created_by,updated_by,created_at,updated_at)
            VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, spec.id, tenantID, corpID, spec.systemKey, spec.name, spec.description, "[]", 1, actorUserID, actorUserID, now, now)
		if err != nil {
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected > 0 {
			if err := insertAudit(ctx, tx, tenantID, corpID, actorUserID, "agent", spec.id, "create", []string{"system_key", "name", "description", "status"}); err != nil {
				return nil, err
			}
		}
	}
	for _, spec := range specs {
		if err := ensureAnalysisRule(ctx, tx, tenantID, corpID, actorUserID, spec.systemKey, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result := make([]ports.Agent, 0, len(specs))
	for _, spec := range specs {
		value, err := r.getSystemAssistantByKey(ctx, tenantID, corpID, spec.systemKey)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func ensureAnalysisRule(ctx context.Context, tx *sql.Tx, tenantID, corpID, actorUserID int64, assistantSystemKey string, now time.Time) error {
	ruleSystemKey := ports.DefaultSmartAnalysisSystemKey
	ruleName := ports.DefaultSmartAnalysisRuleName
	objective := defaultSmartAnalysisObjective
	customerPrompt, employeePrompt := any(nil), any(nil)
	if assistantSystemKey == ports.SessionAnalysisSystemKey {
		ruleSystemKey = ports.SessionAnalysisSystemKey
		ruleName = ports.SessionAnalysisRuleName
		objective = defaultSessionAnalysisObjective
		customerPrompt, employeePrompt = defaultCustomerAnalysisPrompt, defaultEmployeeQAPrompt
	}
	_, err := tx.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_ai_analysis_rules
        (tenant_id,corp_id,system_key,name,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,status,current_version,created_by,updated_by,created_at,updated_at)
        VALUES (?,?,?,?,?,?,?,?,'all','[]',?,?,'enabled',1,?,?,?,?)`, tenantID, corpID, ruleSystemKey, ruleName, objective, customerPrompt, employeePrompt, `["direct","group"]`, 30, 2, actorUserID, actorUserID, now, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_ai_analysis_rule_versions
        (tenant_id,corp_id,rule_id,version,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,created_by,created_at)
        SELECT tenant_id,corp_id,id,current_version,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,updated_by,NOW()
        FROM mochat_go_ai_analysis_rules WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL`, tenantID, corpID, ruleSystemKey)
	return err
}

func (r *AgentRepository) GetSystemAssistant(ctx context.Context, tenantID, corpID int64, id string) (ports.Agent, error) {
	value, err := scanSessionAgent(r.db.QueryRowContext(ctx, "SELECT "+sessionAgentColumns+" FROM mochat_go_ai_agents WHERE id=? AND tenant_id=? AND corp_id=? AND system_key IN (?,?) AND deleted_at IS NULL", id, tenantID, corpID, ports.SessionAnalysisSystemKey, ports.SmartAnalysisSystemKey))
	if errors.Is(err, sql.ErrNoRows) {
		return ports.Agent{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.Agent{}, err
	}
	return r.attachSystemAssistantRule(ctx, value)
}

func (r *AgentRepository) GetSessionAssistant(ctx context.Context, tenantID, corpID int64) (ports.Agent, error) {
	return r.getSystemAssistantByKey(ctx, tenantID, corpID, ports.SessionAnalysisSystemKey)
}

func (r *AgentRepository) getSystemAssistantByKey(ctx context.Context, tenantID, corpID int64, systemKey string) (ports.Agent, error) {
	value, err := scanSessionAgent(r.db.QueryRowContext(ctx, "SELECT "+sessionAgentColumns+" FROM mochat_go_ai_agents WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL", tenantID, corpID, systemKey))
	if errors.Is(err, sql.ErrNoRows) {
		return ports.Agent{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.Agent{}, err
	}
	if err := r.loadAgentKnowledgeStatistics(ctx, &value); err != nil {
		return ports.Agent{}, err
	}
	return r.attachSystemAssistantRule(ctx, value)
}

func (r *AgentRepository) attachSystemAssistantRule(ctx context.Context, value ports.Agent) (ports.Agent, error) {
	switch value.SystemKey {
	case ports.SessionAnalysisSystemKey:
		value.Name = ports.SessionAnalysisAssistantName
		rule, err := loadSessionAnalysisRule(ctx, r.db, value.TenantID, value.CorpID)
		if errors.Is(err, sql.ErrNoRows) {
			return ports.Agent{}, ports.ErrNotFound
		}
		if err != nil {
			return ports.Agent{}, err
		}
		value.SessionAnalysisRule = &rule
		value.SmartAnalysisRule = nil
	case ports.SmartAnalysisSystemKey:
		value.Name = ports.SmartAnalysisAssistantName
		rule, err := loadDefaultSmartAnalysisRule(ctx, r.db, value.TenantID, value.CorpID)
		if errors.Is(err, sql.ErrNoRows) {
			return ports.Agent{}, ports.ErrNotFound
		}
		if err != nil {
			return ports.Agent{}, err
		}
		value.SmartAnalysisRule = &rule
		value.SessionAnalysisRule = nil
	default:
		return ports.Agent{}, ports.ErrNotFound
	}
	return value, nil
}

func (r *AgentRepository) loadAgentKnowledgeStatistics(ctx context.Context, value *ports.Agent) error {
	if len(value.KnowledgeBaseIDs) == 0 {
		return nil
	}
	ids := append([]string{}, value.KnowledgeBaseIDs...)
	sort.Strings(ids)
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+2)
	args = append(args, value.TenantID, value.CorpID)
	for _, id := range ids {
		args = append(args, id)
	}
	return r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT knowledge_base.id), COUNT(DISTINCT document.id)
		FROM mochat_go_ai_knowledge_bases knowledge_base
		LEFT JOIN mochat_go_ai_knowledge_documents document
		  ON document.tenant_id=knowledge_base.tenant_id
		 AND document.corp_id=knowledge_base.corp_id
		 AND document.knowledge_base_id=knowledge_base.id
		 AND knowledge_base.status=1
		 AND document.status='ready'
		 AND document.deleted_at IS NULL
		WHERE knowledge_base.tenant_id=? AND knowledge_base.corp_id=?
		  AND knowledge_base.id IN (`+placeholders+`)
		  AND knowledge_base.deleted_at IS NULL`, args...).Scan(&value.KnowledgeBaseCount, &value.ReadyDocumentCount)
}

func loadSessionAnalysisRule(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, tenantID, corpID int64) (ports.SessionAnalysisRule, error) {
	var value ports.SessionAnalysisRule
	var conversationTypesJSON string
	var updatedAt time.Time
	err := queryer.QueryRowContext(ctx, `SELECT id,name,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version,updated_at
        FROM mochat_go_ai_analysis_rules
        WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL`, tenantID, corpID, ports.SessionAnalysisSystemKey).
		Scan(&value.ID, &value.Name, &value.CustomerAnalysisPrompt, &value.EmployeeQAPrompt, &conversationTypesJSON, &value.LookbackDays, &value.MinimumMessages, &value.CurrentVersion, &updatedAt)
	if err != nil {
		return ports.SessionAnalysisRule{}, err
	}
	if err := json.Unmarshal([]byte(conversationTypesJSON), &value.ConversationTypes); err != nil {
		return ports.SessionAnalysisRule{}, err
	}
	if value.ConversationTypes == nil {
		value.ConversationTypes = []string{}
	}
	value.Name = ports.SessionAnalysisRuleName
	value.UpdatedAt = updatedAt.Format(time.RFC3339)
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
	value.Name = ports.DefaultSmartAnalysisRuleName
	value.UpdatedAt = updatedAt.Format(time.RFC3339)
	return value, nil
}

func (r *AgentRepository) UpdateSessionAssistant(ctx context.Context, value ports.Agent) (ports.Agent, error) {
	value.SystemKey = ports.SessionAnalysisSystemKey
	return r.UpdateSystemAssistant(ctx, value)
}

func (r *AgentRepository) UpdateSystemAssistant(ctx context.Context, value ports.Agent) (ports.Agent, error) {
	if r == nil || r.db == nil {
		return ports.Agent{}, errors.New("AI settings agent database is unavailable")
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
	systemKey, existingKnowledgeBaseIDs, err := lockSystemAssistant(ctx, tx, value.TenantID, value.CorpID, value.ID)
	if err != nil {
		return ports.Agent{}, err
	}
	if len(value.KnowledgeBaseIDs) > 0 {
		if err := lockKnowledgeBases(ctx, tx, value.TenantID, value.CorpID, value.KnowledgeBaseIDs, existingKnowledgeBaseIDs); err != nil {
			return ports.Agent{}, err
		}
	}
	name := ports.SessionAnalysisAssistantName
	if systemKey == ports.SmartAnalysisSystemKey {
		name = ports.SmartAnalysisAssistantName
	}
	value.SystemKey = systemKey
	value.Name = name
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, "UPDATE mochat_go_ai_agents SET name=?, description=?, knowledge_base_ids=?, status=?, updated_by=?, updated_at=? WHERE id=? AND tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL", value.Name, value.Description, string(encoded), value.Status, value.UpdatedBy, now, value.ID, value.TenantID, value.CorpID, systemKey)
	if err != nil {
		return ports.Agent{}, err
	}
	var ruleID int64
	var ruleChanged bool
	if systemKey == ports.SessionAnalysisSystemKey {
		if value.SessionAnalysisRule == nil {
			return ports.Agent{}, errors.New("session analysis rule is required")
		}
		ruleID, ruleChanged, err = updateSessionAnalysisRule(ctx, tx, value, now)
		value.SmartAnalysisRule = nil
	} else {
		if value.SmartAnalysisRule == nil {
			return ports.Agent{}, errors.New("smart analysis rule is required")
		}
		ruleID, ruleChanged, err = updateSmartAnalysisRule(ctx, tx, value, now)
		value.SessionAnalysisRule = nil
	}
	if err != nil {
		return ports.Agent{}, err
	}
	if err := insertAudit(ctx, tx, value.TenantID, value.CorpID, value.UpdatedBy, "agent", value.ID, "update", []string{"description", "knowledge_base_ids", "status"}); err != nil {
		return ports.Agent{}, err
	}
	if ruleChanged {
		changedFields := []string{"objective", "conversation_types", "lookback_days", "minimum_messages", "current_version"}
		if systemKey == ports.SessionAnalysisSystemKey {
			changedFields = []string{"customer_analysis_prompt", "employee_qa_prompt", "conversation_types", "lookback_days", "minimum_messages", "current_version"}
		}
		if err := insertAudit(ctx, tx, value.TenantID, value.CorpID, value.UpdatedBy, "analysis_rule", fmt.Sprintf("%d", ruleID), "update", changedFields); err != nil {
			return ports.Agent{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ports.Agent{}, err
	}
	return r.getSystemAssistantByKey(ctx, value.TenantID, value.CorpID, systemKey)
}

func lockSystemAssistant(ctx context.Context, tx *sql.Tx, tenantID, corpID int64, id string) (string, map[string]struct{}, error) {
	var systemKey, knowledgeBaseJSON string
	err := tx.QueryRowContext(ctx, `SELECT system_key,knowledge_base_ids FROM mochat_go_ai_agents
        WHERE id=? AND tenant_id=? AND corp_id=? AND system_key IN (?,?) AND deleted_at IS NULL FOR UPDATE`, id, tenantID, corpID, ports.SessionAnalysisSystemKey, ports.SmartAnalysisSystemKey).Scan(&systemKey, &knowledgeBaseJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ports.ErrNotFound
	}
	if err != nil {
		return "", nil, err
	}
	var knowledgeBaseIDs []string
	if err := json.Unmarshal([]byte(knowledgeBaseJSON), &knowledgeBaseIDs); err != nil {
		return "", nil, err
	}
	existing := make(map[string]struct{}, len(knowledgeBaseIDs))
	for _, knowledgeBaseID := range knowledgeBaseIDs {
		existing[knowledgeBaseID] = struct{}{}
	}
	return systemKey, existing, nil
}

func updateSessionAnalysisRule(ctx context.Context, tx *sql.Tx, value ports.Agent, now time.Time) (int64, bool, error) {
	var ruleID int64
	var objective, customerPrompt, employeePrompt, currentTypesJSON string
	var lookbackDays, minimumMessages, currentVersion int
	err := tx.QueryRowContext(ctx, `SELECT id,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version
        FROM mochat_go_ai_analysis_rules
        WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL FOR UPDATE`, value.TenantID, value.CorpID, ports.SessionAnalysisSystemKey).
		Scan(&ruleID, &objective, &customerPrompt, &employeePrompt, &currentTypesJSON, &lookbackDays, &minimumMessages, &currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, ports.ErrNotFound
	}
	if err != nil {
		return 0, false, err
	}
	typesJSON, err := canonicalConversationTypesJSON(value.SessionAnalysisRule.ConversationTypes)
	if err != nil {
		return 0, false, err
	}
	currentCanonicalTypesJSON, err := canonicalStoredConversationTypesJSON(currentTypesJSON)
	if err != nil {
		return 0, false, err
	}
	rule := value.SessionAnalysisRule
	changed := customerPrompt != rule.CustomerAnalysisPrompt || employeePrompt != rule.EmployeeQAPrompt || currentCanonicalTypesJSON != typesJSON || lookbackDays != rule.LookbackDays || minimumMessages != rule.MinimumMessages
	if !changed {
		return ruleID, false, nil
	}
	nextVersion := currentVersion + 1
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_ai_analysis_rules
        SET customer_analysis_prompt=?,employee_qa_prompt=?,conversation_types_json=?,lookback_days=?,minimum_messages=?,current_version=?,updated_by=?,updated_at=?
        WHERE tenant_id=? AND corp_id=? AND id=? AND system_key=? AND deleted_at IS NULL`, rule.CustomerAnalysisPrompt, rule.EmployeeQAPrompt, typesJSON, rule.LookbackDays, rule.MinimumMessages, nextVersion, value.UpdatedBy, now, value.TenantID, value.CorpID, ruleID, ports.SessionAnalysisSystemKey)
	if err != nil {
		return 0, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, false, err
	}
	if affected == 0 {
		return 0, false, ports.ErrNotFound
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_ai_analysis_rule_versions
        (tenant_id,corp_id,rule_id,version,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,created_by,created_at)
        VALUES (?,?,?,?,?,?,?,?,'all','[]',?,?,?,?)`, value.TenantID, value.CorpID, ruleID, nextVersion, objective, rule.CustomerAnalysisPrompt, rule.EmployeeQAPrompt, typesJSON, rule.LookbackDays, rule.MinimumMessages, value.UpdatedBy, now)
	if err != nil {
		return 0, false, err
	}
	return ruleID, true, nil
}

func updateSmartAnalysisRule(ctx context.Context, tx *sql.Tx, value ports.Agent, now time.Time) (int64, bool, error) {
	var ruleID int64
	var objective, currentTypesJSON string
	var customerPrompt, employeePrompt sql.NullString
	var lookbackDays, minimumMessages, currentVersion int
	err := tx.QueryRowContext(ctx, `SELECT id,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,lookback_days,minimum_messages,current_version
        FROM mochat_go_ai_analysis_rules
        WHERE tenant_id=? AND corp_id=? AND system_key=? AND deleted_at IS NULL FOR UPDATE`, value.TenantID, value.CorpID, ports.DefaultSmartAnalysisSystemKey).
		Scan(&ruleID, &objective, &customerPrompt, &employeePrompt, &currentTypesJSON, &lookbackDays, &minimumMessages, &currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, ports.ErrNotFound
	}
	if err != nil {
		return 0, false, err
	}
	typesJSON, err := canonicalConversationTypesJSON(value.SmartAnalysisRule.ConversationTypes)
	if err != nil {
		return 0, false, err
	}
	currentCanonicalTypesJSON, err := canonicalStoredConversationTypesJSON(currentTypesJSON)
	if err != nil {
		return 0, false, err
	}
	rule := value.SmartAnalysisRule
	changed := objective != rule.Objective || currentCanonicalTypesJSON != typesJSON || lookbackDays != rule.LookbackDays || minimumMessages != rule.MinimumMessages
	if !changed {
		return ruleID, false, nil
	}
	nextVersion := currentVersion + 1
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_ai_analysis_rules
        SET objective=?,conversation_types_json=?,lookback_days=?,minimum_messages=?,current_version=?,updated_by=?,updated_at=?
        WHERE tenant_id=? AND corp_id=? AND id=? AND system_key=? AND deleted_at IS NULL`, rule.Objective, typesJSON, rule.LookbackDays, rule.MinimumMessages, nextVersion, value.UpdatedBy, now, value.TenantID, value.CorpID, ruleID, ports.DefaultSmartAnalysisSystemKey)
	if err != nil {
		return 0, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, false, err
	}
	if affected == 0 {
		return 0, false, ports.ErrNotFound
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_ai_analysis_rule_versions
        (tenant_id,corp_id,rule_id,version,objective,customer_analysis_prompt,employee_qa_prompt,conversation_types_json,target_scope,target_ids_json,lookback_days,minimum_messages,created_by,created_at)
        VALUES (?,?,?,?,?,?,?,?,'all','[]',?,?,?,?)`, value.TenantID, value.CorpID, ruleID, nextVersion, rule.Objective, nullStringValue(customerPrompt), nullStringValue(employeePrompt), typesJSON, rule.LookbackDays, rule.MinimumMessages, value.UpdatedBy, now)
	if err != nil {
		return 0, false, err
	}
	return ruleID, true, nil
}

func nullStringValue(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func canonicalConversationTypesJSON(values []string) (string, error) {
	canonical := append([]string(nil), nonNilStrings(values)...)
	sort.Strings(canonical)
	encoded, err := json.Marshal(canonical)
	return string(encoded), err
}

func canonicalStoredConversationTypesJSON(encoded string) (string, error) {
	var values []string
	if err := json.Unmarshal([]byte(encoded), &values); err != nil {
		return "", err
	}
	return canonicalConversationTypesJSON(values)
}

func (r *AgentRepository) LoadSessionAssistantContext(ctx context.Context, tenantID, corpID int64) (ports.SessionAssistantContext, error) {
	return r.LoadSystemAssistantContext(ctx, tenantID, corpID, ports.SessionAnalysisSystemKey)
}

func (r *AgentRepository) LoadSystemAssistantContext(ctx context.Context, tenantID, corpID int64, systemKey string) (ports.SystemAssistantContext, error) {
	if systemKey != ports.SessionAnalysisSystemKey && systemKey != ports.SmartAnalysisSystemKey {
		return ports.SystemAssistantContext{}, ports.ErrNotFound
	}
	agent, err := r.getSystemAssistantByKey(ctx, tenantID, corpID, systemKey)
	if err != nil {
		return ports.SystemAssistantContext{}, err
	}
	hash := sha256.New()
	fmt.Fprintf(hash, "agent=%s\nstatus=%d\ndescription=%s\nupdated=%s\n", agent.ID, agent.Status, agent.Description, agent.UpdatedAt)
	contextValue := ports.SystemAssistantContext{AgentID: agent.ID, Name: agent.Name, Instructions: agent.Description, Enabled: agent.Status == 1, UpdatedAt: agent.UpdatedAt, KnowledgeChunks: []ports.KnowledgeChunk{}}
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
