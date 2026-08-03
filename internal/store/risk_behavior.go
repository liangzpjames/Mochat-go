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
	if message.CorpID <= 0 || strings.TrimSpace(message.MessageID) == "" || strings.TrimSpace(message.Content) == "" {
		return 0, fmt.Errorf("风险消息参数无效")
	}
	page, err := s.RiskRulePage(ctx, dashboard.RiskRuleFilter{TenantID: message.TenantID, CorpID: message.CorpID, Page: 1, PerPage: 100})
	if err != nil {
		return 0, err
	}
	matches := dashboard.MatchRiskStrategies(message, page.Items)
	created := 0
	for _, record := range matches {
		related, _ := json.Marshal(record.RelatedUser)
		occurred := time.Now()
		if parsed, parseErr := time.Parse(time.RFC3339, record.OccurredAt); parseErr == nil {
			occurred = parsed
		}
		result, execErr := s.db.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_risk_records (tenant_id,corp_id,rule_id,strategy_id,behavior,risk_level,conversation_type,conversation_id,message_id,trigger_message,related_user_json,ai_summary,audit_status,occurred_at,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, record.TenantID, record.CorpID, record.RuleID, record.StrategyID, record.Behavior, record.RiskLevel, record.ConversationType, record.ConversationID, record.MessageID, record.TriggerMessage, related, nil, "pending", occurred, time.Now())
		if execErr != nil {
			return created, execErr
		}
		n, _ := result.RowsAffected()
		if n > 0 {
			created++
			_, _ = s.db.ExecContext(ctx, `UPDATE mochat_go_risk_rules SET trigger_count=trigger_count+1,updated_at=? WHERE id=?`, time.Now(), record.RuleID)
		}
	}
	return created, nil
}

func (s *MySQLStore) RiskRecordPage(ctx context.Context, f dashboard.RiskRecordFilter) (dashboard.RiskRecordPage, error) {
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
	if f.RuleID > 0 {
		where += " AND rule_id = ?"
		args = append(args, f.RuleID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_risk_records"+where, args...).Scan(&total); err != nil {
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
		_ = json.Unmarshal(user, &v.RelatedUser)
		v.OccurredAt = occurred.Format(time.RFC3339)
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RiskRecordPage{}, err
	}
	return dashboard.RiskRecordPage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, nil
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
	whitelist, _ := json.Marshal(rule.Whitelist)
	result, err := s.db.ExecContext(ctx, `INSERT INTO mochat_go_risk_rules (tenant_id,corp_id,name,status,subject,whitelist_json,ai_insight_enabled,created_by,updated_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, rule.TenantID, rule.CorpID, strings.TrimSpace(rule.Name), rule.Status, rule.Subject, whitelist, rule.AIInsightEnabled, 0, 0, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, strategy := range rule.Strategies {
		if _, err = s.db.ExecContext(ctx, `INSERT INTO mochat_go_risk_rule_strategies (rule_id,behavior,pattern,notify_type,risk_level,created_at) VALUES (?,?,?,?,?,?)`, id, strings.TrimSpace(strategy.Behavior), strings.TrimSpace(strategy.Pattern), strategy.NotifyType, strategy.RiskLevel, now); err != nil {
			return 0, err
		}
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
	whitelist, _ := json.Marshal(rule.Whitelist)
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_risk_rules SET name=?,subject=?,whitelist_json=?,ai_insight_enabled=?,updated_at=? WHERE id=? AND corp_id=?`, strings.TrimSpace(rule.Name), rule.Subject, whitelist, rule.AIInsightEnabled, time.Now(), rule.ID, rule.CorpID)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return false, nil
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM mochat_go_risk_rule_strategies WHERE rule_id=?`, rule.ID); err != nil {
		return false, err
	}
	for _, strategy := range rule.Strategies {
		if _, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_risk_rule_strategies (rule_id,behavior,pattern,notify_type,risk_level,created_at) VALUES (?,?,?,?,?,?)`, rule.ID, strings.TrimSpace(strategy.Behavior), strings.TrimSpace(strategy.Pattern), strategy.NotifyType, strategy.RiskLevel, time.Now()); err != nil {
			return false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) SetRiskRuleStatus(ctx context.Context, corpID int, id int64, status dashboard.RiskRuleStatus) (bool, error) {
	if status != dashboard.RiskRuleEnabled && status != dashboard.RiskRuleDisabled {
		return false, fmt.Errorf("规则状态无效")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE mochat_go_risk_rules SET status=?,updated_at=? WHERE id=? AND corp_id=?`, status, time.Now(), id, corpID)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

func (s *MySQLStore) DeleteRiskRule(ctx context.Context, corpID int, id int64) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_risk_records WHERE corp_id=? AND rule_id=?`, corpID, id).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return false, fmt.Errorf("已有风险记录的规则不能删除，请停用规则")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM mochat_go_risk_rule_strategies WHERE rule_id=?`, id); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_risk_rules WHERE id=? AND corp_id=?`, id, corpID)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
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
		result, execErr := tx.ExecContext(ctx, `UPDATE mochat_go_risk_records SET audit_status=? WHERE id=? AND corp_id=?`, action, id, corpID)
		if execErr != nil {
			return 0, execErr
		}
		n, _ := result.RowsAffected()
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
