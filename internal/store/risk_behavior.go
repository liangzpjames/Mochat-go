package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"jiyi/mochat-go/internal/dashboard"
	"strings"
	"time"
)

func parseRiskRelatedUser(raw []byte) map[string]any {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	if object, ok := payload.(map[string]any); ok {
		return object
	}
	items, ok := payload.([]any)
	if !ok {
		return map[string]any{}
	}
	result := map[string]any{}
	for _, item := range items {
		person, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := person["role"].(string)
		name, _ := person["userName"].(string)
		if name == "" {
			name, _ = person["name"].(string)
		}
		id, _ := person["userId"].(float64)
		if id == 0 {
			id, _ = person["id"].(float64)
		}
		switch role {
		case "employee":
			if name != "" {
				result["employeeName"] = name
			}
			if id > 0 {
				result["employeeId"] = id
			}
		case "customer":
			if name != "" {
				result["customerName"] = name
			}
			if id > 0 {
				result["customerId"] = id
			}
		case "room":
			if name != "" {
				result["roomName"] = name
			}
		}
	}
	return result
}

func (s *MySQLStore) RiskRulePage(ctx context.Context, f dashboard.RiskRuleFilter) (dashboard.RiskRulePage, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 || f.PerPage > 100 {
		f.PerPage = 20
	}
	where := " WHERE corp_id = ?"
	args := []any{f.CorpID}
	if f.TenantID > 0 {
		where = " WHERE tenant_id = ? AND corp_id = ?"
		args = []any{f.TenantID, f.CorpID}
	}
	if strings.TrimSpace(f.Name) != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(f.Name)+"%")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_risk_rules"+where, args...).Scan(&total); err != nil {
		return dashboard.RiskRulePage{}, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,tenant_id,corp_id,name,status,subject,whitelist_json,ai_insight_enabled,trigger_count FROM mochat_go_risk_rules"+where+" ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?", append(args, f.PerPage, (f.Page-1)*f.PerPage)...)
	if err != nil {
		return dashboard.RiskRulePage{}, err
	}
	defer rows.Close()
	items := []dashboard.RiskRule{}
	for rows.Next() {
		var v dashboard.RiskRule
		var whitelist []byte
		var ai int
		if err := rows.Scan(&v.ID, &v.TenantID, &v.CorpID, &v.Name, &v.Status, &v.Subject, &whitelist, &ai, &v.TriggerCount); err != nil {
			return dashboard.RiskRulePage{}, err
		}
		_ = json.Unmarshal(whitelist, &v.Whitelist)
		v.AIInsightEnabled = ai != 0
		strategyRows, strategyErr := s.db.QueryContext(ctx, `SELECT id,behavior,pattern,notify_type,risk_level FROM mochat_go_risk_rule_strategies WHERE rule_id=? ORDER BY id`, v.ID)
		if strategyErr != nil {
			return dashboard.RiskRulePage{}, strategyErr
		}
		for strategyRows.Next() {
			var strategy dashboard.RiskRuleStrategy
			if scanErr := strategyRows.Scan(&strategy.ID, &strategy.Behavior, &strategy.Pattern, &strategy.NotifyType, &strategy.RiskLevel); scanErr != nil {
				strategyRows.Close()
				return dashboard.RiskRulePage{}, scanErr
			}
			v.Strategies = append(v.Strategies, strategy)
		}
		strategyRows.Close()
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RiskRulePage{}, err
	}
	return dashboard.RiskRulePage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, nil
}

func (s *MySQLStore) EvaluateRiskMessage(ctx context.Context, message dashboard.RiskMessage) (int, error) {
	if message.TenantID <= 0 || message.CorpID <= 0 || strings.TrimSpace(message.MessageID) == "" || strings.TrimSpace(message.Content) == "" {
		return 0, fmt.Errorf("风险消息参数无效")
	}
	occurredAt := time.Now()
	if strings.TrimSpace(message.OccurredAt) != "" {
		parsed, err := time.Parse(time.RFC3339, message.OccurredAt)
		if err != nil {
			return 0, fmt.Errorf("风险消息发生时间无效: %w", err)
		}
		occurredAt = parsed
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var highWater int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0) FROM mochat_go_risk_rules WHERE tenant_id=? AND corp_id=?`, message.TenantID, message.CorpID).Scan(&highWater); err != nil {
		return 0, err
	}
	const batchSize = 100
	var cursor int64
	totalCreated := 0
	for cursor < highWater {
		rules, nextCursor, err := riskRuleBatchForEvaluation(ctx, tx, message.TenantID, message.CorpID, cursor, highWater, batchSize)
		if err != nil {
			return 0, err
		}
		if len(rules) == 0 {
			break
		}
		for _, record := range dashboard.MatchRiskStrategies(message, rules) {
			related, marshalErr := json.Marshal(record.RelatedUser)
			if marshalErr != nil {
				return 0, marshalErr
			}
			result, execErr := tx.ExecContext(ctx, `INSERT INTO mochat_go_risk_records (tenant_id,corp_id,rule_id,strategy_id,behavior,risk_level,conversation_type,conversation_id,message_id,trigger_message,related_user_json,ai_summary,audit_status,occurred_at,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, record.TenantID, record.CorpID, record.RuleID, record.StrategyID, record.Behavior, record.RiskLevel, record.ConversationType, record.ConversationID, record.MessageID, record.TriggerMessage, related, nil, "pending", occurredAt, time.Now())
			if execErr != nil {
				if isMySQLDuplicateKeyError(execErr) {
					continue
				}
				return 0, execErr
			}
			n, affectedErr := result.RowsAffected()
			if affectedErr != nil {
				return 0, affectedErr
			}
			if n != 1 {
				return 0, fmt.Errorf("风险记录保存失败")
			}
			countResult, countErr := tx.ExecContext(ctx, `UPDATE mochat_go_risk_rules SET trigger_count=trigger_count+1,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=?`, time.Now(), record.RuleID, message.TenantID, message.CorpID)
			if countErr != nil {
				return 0, countErr
			}
			countAffected, affectedErr := countResult.RowsAffected()
			if affectedErr != nil {
				return 0, affectedErr
			}
			if countAffected != 1 {
				return 0, fmt.Errorf("风险规则计数更新失败")
			}
			totalCreated++
		}
		if nextCursor <= cursor {
			return 0, fmt.Errorf("风险规则游标未推进")
		}
		cursor = nextCursor
		if len(rules) < batchSize {
			break
		}
		rules = nil
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return totalCreated, nil
}

func riskRuleBatchForEvaluation(ctx context.Context, tx *sql.Tx, tenantID, corpID int, cursor, highWater int64, limit int) ([]dashboard.RiskRule, int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,tenant_id,corp_id,name,status,subject,whitelist_json,ai_insight_enabled,trigger_count
		FROM mochat_go_risk_rules
		WHERE tenant_id=? AND corp_id=? AND status='enabled' AND id>? AND id<=?
		ORDER BY id LIMIT ? FOR UPDATE`, tenantID, corpID, cursor, highWater, limit)
	if err != nil {
		return nil, cursor, err
	}
	rules := make([]dashboard.RiskRule, 0, limit)
	ruleIDs := make([]int64, 0, limit)
	for rows.Next() {
		var rule dashboard.RiskRule
		var whitelist []byte
		var ai int
		if err := rows.Scan(&rule.ID, &rule.TenantID, &rule.CorpID, &rule.Name, &rule.Status, &rule.Subject, &whitelist, &ai, &rule.TriggerCount); err != nil {
			rows.Close()
			return nil, cursor, err
		}
		if err := json.Unmarshal(whitelist, &rule.Whitelist); err != nil {
			rows.Close()
			return nil, cursor, err
		}
		rule.AIInsightEnabled = ai != 0
		rules = append(rules, rule)
		ruleIDs = append(ruleIDs, rule.ID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, cursor, err
	}
	if err := rows.Close(); err != nil {
		return nil, cursor, err
	}
	if len(rules) == 0 {
		return rules, cursor, nil
	}
	args := make([]any, 0, len(ruleIDs))
	index := make(map[int64]int, len(ruleIDs))
	for i, id := range ruleIDs {
		args = append(args, id)
		index[id] = i
	}
	strategyRows, err := tx.QueryContext(ctx, `SELECT rule_id,id,behavior,pattern,notify_type,risk_level
		FROM mochat_go_risk_rule_strategies
		WHERE rule_id IN (`+placeholders(len(ruleIDs))+`)
		ORDER BY rule_id,id FOR UPDATE`, args...)
	if err != nil {
		return nil, cursor, err
	}
	defer strategyRows.Close()
	for strategyRows.Next() {
		var ruleID int64
		var strategy dashboard.RiskRuleStrategy
		if err := strategyRows.Scan(&ruleID, &strategy.ID, &strategy.Behavior, &strategy.Pattern, &strategy.NotifyType, &strategy.RiskLevel); err != nil {
			return nil, cursor, err
		}
		if i, ok := index[ruleID]; ok {
			rules[i].Strategies = append(rules[i].Strategies, strategy)
		}
	}
	if err := strategyRows.Err(); err != nil {
		return nil, cursor, err
	}
	return rules, ruleIDs[len(ruleIDs)-1], nil
}

func (s *MySQLStore) RiskRecordPage(ctx context.Context, f dashboard.RiskRecordFilter) (dashboard.RiskRecordPage, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	f.PerPage = 20
	where := " WHERE corp_id = ?"
	args := []any{f.CorpID}
	if f.TenantID > 0 {
		where = " WHERE tenant_id = ? AND corp_id = ?"
		args = []any{f.TenantID, f.CorpID}
	}
	if f.RiskLevel != "" {
		where += " AND risk_level = ?"
		args = append(args, f.RiskLevel)
	}
	if f.Behavior != "" {
		where += " AND behavior = ?"
		args = append(args, f.Behavior)
	}
	if f.ConversationType != "" {
		where += " AND conversation_type = ?"
		args = append(args, f.ConversationType)
	}
	if f.AuditStatus != "" {
		where += " AND audit_status = ?"
		args = append(args, f.AuditStatus)
	}
	if strings.TrimSpace(f.OccurredFrom) != "" {
		where += " AND occurred_at >= ?"
		args = append(args, strings.TrimSpace(f.OccurredFrom))
	}
	if strings.TrimSpace(f.OccurredTo) != "" {
		where += " AND occurred_at <= ?"
		args = append(args, strings.TrimSpace(f.OccurredTo))
	}
	if f.RuleID > 0 {
		where += " AND rule_id = ?"
		args = append(args, f.RuleID)
	}
	if len(f.EmployeeIDs) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(f.EmployeeIDs)), ",")
		where += " AND CAST(JSON_UNQUOTE(JSON_EXTRACT(related_user_json, '$.employeeId')) AS UNSIGNED) IN (" + placeholders + ")"
		for _, id := range f.EmployeeIDs {
			args = append(args, id)
		}
	}
	if f.RestrictEmployeeIDs {
		if len(f.AllowedEmployeeIDs) == 0 {
			where += " AND 1=0"
		} else {
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(f.AllowedEmployeeIDs)), ",")
			where += " AND CAST(JSON_UNQUOTE(JSON_EXTRACT(related_user_json, '$.employeeId')) AS UNSIGNED) IN (" + placeholders + ")"
			for _, id := range f.AllowedEmployeeIDs {
				args = append(args, id)
			}
		}
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_risk_records"+where, args...).Scan(&total); err != nil {
		return dashboard.RiskRecordPage{}, err
	}
	var pending, highRisk, processed int
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(CASE WHEN audit_status='pending' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN risk_level='high' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN audit_status IN ('confirmed','ignored','reviewed') THEN 1 ELSE 0 END),0) FROM mochat_go_risk_records"+where, args...).Scan(&pending, &highRisk, &processed); err != nil {
		return dashboard.RiskRecordPage{}, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,tenant_id,corp_id,rule_id,strategy_id,behavior,risk_level,conversation_type,conversation_id,message_id,trigger_message,related_user_json,COALESCE(ai_summary,''),audit_status,occurred_at FROM mochat_go_risk_records"+where+" ORDER BY occurred_at DESC,id DESC LIMIT ? OFFSET ?", append(args, f.PerPage, (f.Page-1)*f.PerPage)...)
	if err != nil {
		return dashboard.RiskRecordPage{}, err
	}
	defer rows.Close()
	items := []dashboard.RiskRecord{}
	for rows.Next() {
		var v dashboard.RiskRecord
		var user []byte
		var occurred time.Time
		if err := rows.Scan(&v.ID, &v.TenantID, &v.CorpID, &v.RuleID, &v.StrategyID, &v.Behavior, &v.RiskLevel, &v.ConversationType, &v.ConversationID, &v.MessageID, &v.TriggerMessage, &user, &v.AISummary, &v.AuditStatus, &occurred); err != nil {
			return dashboard.RiskRecordPage{}, err
		}
		v.RelatedUser = parseRiskRelatedUser(user)
		v.OccurredAt = occurred.Format(time.RFC3339)
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RiskRecordPage{}, err
	}
	return dashboard.RiskRecordPage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage, Summary: &dashboard.RiskRecordSummary{Total: total, Pending: pending, HighRisk: highRisk, Processed: processed}}, nil
}

func (s *MySQLStore) RiskRecordDetail(ctx context.Context, f dashboard.RiskRecordDetailFilter) (dashboard.RiskRecordDetail, error) {
	if f.ID <= 0 {
		return dashboard.RiskRecordDetail{}, fmt.Errorf("风险记录不存在")
	}
	where := "tenant_id=? AND corp_id=? AND id=?"
	args := []any{f.TenantID, f.CorpID, f.ID}
	if f.RestrictEmployeeIDs {
		if len(f.AllowedEmployeeIDs) == 0 {
			return dashboard.RiskRecordDetail{}, fmt.Errorf("风险记录不存在")
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(f.AllowedEmployeeIDs)), ",")
		where += " AND CAST(JSON_UNQUOTE(JSON_EXTRACT(related_user_json, '$.employeeId')) AS UNSIGNED) IN (" + placeholders + ")"
		for _, id := range f.AllowedEmployeeIDs {
			args = append(args, id)
		}
	}
	var record dashboard.RiskRecord
	var user []byte
	var occurred time.Time
	err := s.db.QueryRowContext(ctx, `SELECT id,tenant_id,corp_id,rule_id,strategy_id,behavior,risk_level,conversation_type,conversation_id,message_id,trigger_message,related_user_json,COALESCE(ai_summary,''),audit_status,occurred_at FROM mochat_go_risk_records WHERE `+where, args...).Scan(&record.ID, &record.TenantID, &record.CorpID, &record.RuleID, &record.StrategyID, &record.Behavior, &record.RiskLevel, &record.ConversationType, &record.ConversationID, &record.MessageID, &record.TriggerMessage, &user, &record.AISummary, &record.AuditStatus, &occurred)
	if err == sql.ErrNoRows {
		return dashboard.RiskRecordDetail{}, fmt.Errorf("风险记录不存在")
	}
	if err != nil {
		return dashboard.RiskRecordDetail{}, err
	}
	record.RelatedUser = parseRiskRelatedUser(user)
	record.OccurredAt = occurred.Format(time.RFC3339)
	auditRows, err := s.db.QueryContext(ctx, `SELECT id,actor_id,action,remark,created_at FROM mochat_go_risk_record_audits WHERE tenant_id=? AND corp_id=? AND record_id=? ORDER BY created_at DESC,id DESC`, f.TenantID, f.CorpID, f.ID)
	if err != nil {
		return dashboard.RiskRecordDetail{}, err
	}
	defer auditRows.Close()
	audits := []dashboard.RiskRecordAudit{}
	for auditRows.Next() {
		var audit dashboard.RiskRecordAudit
		var created time.Time
		if err := auditRows.Scan(&audit.ID, &audit.ActorID, &audit.Action, &audit.Remark, &created); err != nil {
			return dashboard.RiskRecordDetail{}, err
		}
		audit.CreatedAt = created.Format(time.RFC3339)
		audits = append(audits, audit)
	}
	conversationAvailable := false
	if record.MessageID != "" {
		employeeIDs := f.AllowedEmployeeIDs
		if value, ok := record.RelatedUser["employeeId"].(float64); ok && value > 0 {
			employeeIDs = []int{int(value)}
		}
		// Session positioning is optional enrichment. A missing archive source,
		// legacy simulation row, or unavailable archive index must not hide the
		// risk record and its audit history; the UI reports the positioning gap.
		if _, found, lookupErr := s.WorkMessageByArchiveID(ctx, dashboard.WorkMessageArchiveFilter{CorpID: f.CorpID, ArchiveMessageID: record.MessageID, EmployeeIDs: employeeIDs, RestrictEmployeeIDs: f.RestrictEmployeeIDs}); lookupErr == nil {
			conversationAvailable = found
		}
	}
	return dashboard.RiskRecordDetail{Record: record, Audits: audits, ConversationAvailable: conversationAvailable}, nil
}

func (s *MySQLStore) CreateRiskRule(ctx context.Context, rule dashboard.RiskRule) (int64, error) {
	if err := dashboard.ValidateRiskRule(rule); err != nil {
		return 0, err
	}
	if rule.TenantID <= 0 {
		tenant, err := s.tenantIDByCorpID(ctx, int(rule.CorpID))
		if err != nil {
			return 0, err
		}
		rule.TenantID = int64(tenant)
	}
	now := time.Now()
	whitelist, err := json.Marshal(rule.Whitelist)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_risk_rules (tenant_id,corp_id,name,status,subject,whitelist_json,ai_insight_enabled,created_by,updated_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, rule.TenantID, rule.CorpID, strings.TrimSpace(rule.Name), rule.Status, rule.Subject, whitelist, rule.AIInsightEnabled, 0, 0, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, strategy := range rule.Strategies {
		if _, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_risk_rule_strategies (rule_id,behavior,pattern,notify_type,risk_level,created_at) VALUES (?,?,?,?,?,?)`, id, strings.TrimSpace(strategy.Behavior), strings.TrimSpace(strategy.Pattern), strategy.NotifyType, strategy.RiskLevel, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) UpdateRiskRule(ctx context.Context, rule dashboard.RiskRule) (bool, error) {
	if rule.ID <= 0 {
		return false, fmt.Errorf("规则 ID 无效")
	}
	if err := dashboard.ValidateRiskRule(rule); err != nil {
		return false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var lockedID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_risk_rules WHERE id=? AND tenant_id=? AND corp_id=? FOR UPDATE`, rule.ID, rule.TenantID, rule.CorpID).Scan(&lockedID); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	whitelist, err := json.Marshal(rule.Whitelist)
	if err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_risk_rules SET name=?,status=?,subject=?,whitelist_json=?,ai_insight_enabled=?,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=?`, strings.TrimSpace(rule.Name), rule.Status, rule.Subject, whitelist, rule.AIInsightEnabled, time.Now(), rule.ID, rule.TenantID, rule.CorpID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected > 1 {
		return false, fmt.Errorf("风险规则更新影响了意外的行数")
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,behavior FROM mochat_go_risk_rule_strategies WHERE rule_id=? ORDER BY id FOR UPDATE`, rule.ID)
	if err != nil {
		return false, err
	}
	existing := make(map[string]int64, len(rule.Strategies))
	for rows.Next() {
		var id int64
		var behavior string
		if err := rows.Scan(&id, &behavior); err != nil {
			rows.Close()
			return false, err
		}
		existing[behavior] = id
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	if err := rows.Close(); err != nil {
		return false, err
	}
	retained := make(map[string]struct{}, len(rule.Strategies))
	for _, strategy := range rule.Strategies {
		behavior := strings.TrimSpace(strategy.Behavior)
		retained[behavior] = struct{}{}
		if id, ok := existing[behavior]; ok {
			strategyResult, err := tx.ExecContext(ctx, `UPDATE mochat_go_risk_rule_strategies SET pattern=?,notify_type=?,risk_level=? WHERE id=? AND rule_id=?`, strings.TrimSpace(strategy.Pattern), strategy.NotifyType, strategy.RiskLevel, id, rule.ID)
			if err != nil {
				return false, err
			}
			n, err := strategyResult.RowsAffected()
			if err != nil {
				return false, err
			}
			if n > 1 {
				return false, fmt.Errorf("风险策略更新影响了意外的行数")
			}
			continue
		}
		strategyResult, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_risk_rule_strategies (rule_id,behavior,pattern,notify_type,risk_level,created_at) VALUES (?,?,?,?,?,?)`, rule.ID, behavior, strings.TrimSpace(strategy.Pattern), strategy.NotifyType, strategy.RiskLevel, time.Now())
		if err != nil {
			return false, err
		}
		n, err := strategyResult.RowsAffected()
		if err != nil {
			return false, err
		}
		if n != 1 {
			return false, fmt.Errorf("风险策略新增失败")
		}
	}
	for behavior, id := range existing {
		if _, ok := retained[behavior]; ok {
			continue
		}
		deleteResult, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_risk_rule_strategies WHERE id=? AND rule_id=?`, id, rule.ID)
		if err != nil {
			return false, err
		}
		n, err := deleteResult.RowsAffected()
		if err != nil {
			return false, err
		}
		if n != 1 {
			return false, fmt.Errorf("风险策略删除影响了意外的行数")
		}
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) SetRiskRuleStatus(ctx context.Context, tenantID, corpID int, id int64, status dashboard.RiskRuleStatus) (bool, error) {
	if status != dashboard.RiskRuleEnabled && status != dashboard.RiskRuleDisabled {
		return false, fmt.Errorf("规则状态无效")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE mochat_go_risk_rules SET status=?,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=?`, status, time.Now(), id, tenantID, corpID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *MySQLStore) DeleteRiskRule(ctx context.Context, tenantID, corpID int, id int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var lockedID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_risk_rules WHERE id=? AND tenant_id=? AND corp_id=? FOR UPDATE`, id, tenantID, corpID).Scan(&lockedID); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_risk_records WHERE tenant_id=? AND corp_id=? AND rule_id=?`, tenantID, corpID, id).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return false, fmt.Errorf("已有风险记录的规则不能删除，请停用规则")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM mochat_go_risk_rule_strategies WHERE rule_id=?`, id); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_risk_rules WHERE id=? AND tenant_id=? AND corp_id=?`, id, tenantID, corpID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *MySQLStore) AuditRiskRecords(ctx context.Context, tenantID int, corpID int, actorID int, ids []int64, action string, remark string) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("请选择风险记录")
	}
	if action != "confirmed" && action != "ignored" {
		return 0, fmt.Errorf("审计操作无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var total int64
	for _, id := range ids {
		result, execErr := tx.ExecContext(ctx, `UPDATE mochat_go_risk_records SET audit_status=? WHERE id=? AND tenant_id=? AND corp_id=?`, action, id, tenantID, corpID)
		if execErr != nil {
			return 0, execErr
		}
		n, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return 0, affectedErr
		}
		if n == 0 {
			continue
		}
		total += n
		if _, execErr = tx.ExecContext(ctx, `INSERT INTO mochat_go_risk_record_audits (record_id,tenant_id,corp_id,actor_id,action,remark,created_at) VALUES (?,?,?,?,?,?,?)`, id, tenantID, corpID, actorID, action, strings.TrimSpace(remark), time.Now()); execErr != nil {
			return 0, execErr
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}

var _ dashboard.RiskBehaviorProvider = (*MySQLStore)(nil)
var _ = sql.ErrNoRows
