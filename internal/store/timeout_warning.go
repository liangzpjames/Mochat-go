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

func normalizeTimeoutPage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return page, perPage
}
func timeoutScopeWhere(tenantID, corpID int) (string, []any) {
	if tenantID > 0 {
		return " WHERE tenant_id = ? AND corp_id = ?", []any{tenantID, corpID}
	}
	return " WHERE corp_id = ?", []any{corpID}
}

func (s *MySQLStore) TimeoutRulePage(ctx context.Context, f dashboard.TimeoutRuleFilter) (dashboard.TimeoutRulePage, error) {
	f.Page, f.PerPage = normalizeTimeoutPage(f.Page, f.PerPage)
	where, args := timeoutScopeWhere(f.TenantID, f.CorpID)
	if strings.TrimSpace(f.Name) != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+strings.TrimSpace(f.Name)+"%")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_timeout_rules"+where, args...).Scan(&total); err != nil {
		return dashboard.TimeoutRulePage{}, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,tenant_id,corp_id,name,status,monitor_target,monitor_target_ids_json,conversation_scopes_json,ai_insight_enabled,trigger_count,created_at,updated_at FROM mochat_go_timeout_rules"+where+" ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?", append(args, f.PerPage, (f.Page-1)*f.PerPage)...)
	if err != nil {
		return dashboard.TimeoutRulePage{}, err
	}
	defer rows.Close()
	items := []dashboard.TimeoutRule{}
	for rows.Next() {
		var v dashboard.TimeoutRule
		var targetIDs, scopes []byte
		var ai int
		var created, updated time.Time
		if err = rows.Scan(&v.ID, &v.TenantID, &v.CorpID, &v.Name, &v.Status, &v.MonitorTarget, &targetIDs, &scopes, &ai, &v.TriggerCount, &created, &updated); err != nil {
			return dashboard.TimeoutRulePage{}, err
		}
		_ = json.Unmarshal(targetIDs, &v.MonitorTargetIDs)
		_ = json.Unmarshal(scopes, &v.ConversationScopes)
		v.AIInsightEnabled = ai != 0
		v.CreatedAt = created.Format(time.RFC3339)
		v.UpdatedAt = updated.Format(time.RFC3339)
		if err = s.loadTimeoutRuleRelations(ctx, &v); err != nil {
			return dashboard.TimeoutRulePage{}, err
		}
		items = append(items, v)
	}
	return dashboard.TimeoutRulePage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, rows.Err()
}

func (s *MySQLStore) loadTimeoutRuleRelations(ctx context.Context, rule *dashboard.TimeoutRule) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id,timeout_minutes,notify_type,risk_level,sort_order FROM mochat_go_timeout_rule_strategies WHERE tenant_id=? AND corp_id=? AND rule_id=? ORDER BY sort_order,id`, rule.TenantID, rule.CorpID, rule.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v dashboard.TimeoutStrategy
		if err = rows.Scan(&v.ID, &v.TimeoutMinutes, &v.NotifyType, &v.RiskLevel, &v.SortOrder); err != nil {
			rows.Close()
			return err
		}
		rule.Strategies = append(rule.Strategies, v)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT id,weekday,TIME_FORMAT(start_time,'%H:%i'),TIME_FORMAT(end_time,'%H:%i') FROM mochat_go_timeout_rule_quiet_periods WHERE tenant_id=? AND corp_id=? AND rule_id=? ORDER BY id`, rule.TenantID, rule.CorpID, rule.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v dashboard.TimeoutQuietPeriod
		if err = rows.Scan(&v.ID, &v.Weekday, &v.StartTime, &v.EndTime); err != nil {
			rows.Close()
			return err
		}
		rule.QuietPeriods = append(rule.QuietPeriods, v)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT id,target_type,target_id FROM mochat_go_timeout_rule_notify_targets WHERE tenant_id=? AND corp_id=? AND rule_id=? ORDER BY id`, rule.TenantID, rule.CorpID, rule.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v dashboard.TimeoutNotifyTarget
		if err = rows.Scan(&v.ID, &v.TargetType, &v.TargetID); err != nil {
			rows.Close()
			return err
		}
		rule.NotifyTargets = append(rule.NotifyTargets, v)
	}
	return rows.Close()
}

func (s *MySQLStore) CreateTimeoutRule(ctx context.Context, rule dashboard.TimeoutRule) (int64, error) {
	if err := dashboard.ValidateTimeoutRule(rule); err != nil {
		return 0, err
	}
	if rule.TenantID <= 0 {
		tenant, err := s.tenantIDByCorpID(ctx, int(rule.CorpID))
		if err != nil {
			return 0, err
		}
		rule.TenantID = int64(tenant)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now()
	ids, _ := json.Marshal(rule.MonitorTargetIDs)
	scopes, _ := json.Marshal(rule.ConversationScopes)
	result, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_timeout_rules (tenant_id,corp_id,name,status,monitor_target,monitor_target_ids_json,conversation_scopes_json,ai_insight_enabled,trigger_count,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,0,?,?)`, rule.TenantID, rule.CorpID, strings.TrimSpace(rule.Name), rule.Status, rule.MonitorTarget, ids, scopes, rule.AIInsightEnabled, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err = s.replaceTimeoutRuleRelations(ctx, tx, rule, id, now); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) UpdateTimeoutRule(ctx context.Context, rule dashboard.TimeoutRule) (bool, error) {
	if rule.ID <= 0 {
		return false, fmt.Errorf("规则 ID 无效")
	}
	if err := dashboard.ValidateTimeoutRule(rule); err != nil {
		return false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	ids, _ := json.Marshal(rule.MonitorTargetIDs)
	scopes, _ := json.Marshal(rule.ConversationScopes)
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_timeout_rules SET name=?,monitor_target=?,monitor_target_ids_json=?,conversation_scopes_json=?,ai_insight_enabled=?,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=?`, strings.TrimSpace(rule.Name), rule.MonitorTarget, ids, scopes, rule.AIInsightEnabled, time.Now(), rule.ID, rule.TenantID, rule.CorpID)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return false, nil
	}
	for _, table := range []string{"mochat_go_timeout_rule_strategies", "mochat_go_timeout_rule_quiet_periods", "mochat_go_timeout_rule_notify_targets"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE rule_id=? AND tenant_id=? AND corp_id=?", rule.ID, rule.TenantID, rule.CorpID); err != nil {
			return false, err
		}
	}
	if err = s.replaceTimeoutRuleRelations(ctx, tx, rule, rule.ID, time.Now()); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MySQLStore) replaceTimeoutRuleRelations(ctx context.Context, tx *sql.Tx, rule dashboard.TimeoutRule, id int64, now time.Time) error {
	for i, v := range rule.Strategies {
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_timeout_rule_strategies (tenant_id,corp_id,rule_id,timeout_minutes,notify_type,risk_level,sort_order,created_at) VALUES (?,?,?,?,?,?,?,?)`, rule.TenantID, rule.CorpID, id, v.TimeoutMinutes, v.NotifyType, v.RiskLevel, i, now); err != nil {
			return err
		}
	}
	for _, v := range rule.QuietPeriods {
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_timeout_rule_quiet_periods (tenant_id,corp_id,rule_id,weekday,start_time,end_time) VALUES (?,?,?,?,?,?)`, rule.TenantID, rule.CorpID, id, v.Weekday, v.StartTime, v.EndTime); err != nil {
			return err
		}
	}
	for _, v := range rule.NotifyTargets {
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_timeout_rule_notify_targets (tenant_id,corp_id,rule_id,target_type,target_id) VALUES (?,?,?,?,?)`, rule.TenantID, rule.CorpID, id, v.TargetType, v.TargetID); err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLStore) SetTimeoutRuleStatus(ctx context.Context, tenantID, corpID int, id int64, status dashboard.TimeoutRuleStatus) (bool, error) {
	if status != dashboard.TimeoutRuleEnabled && status != dashboard.TimeoutRuleDisabled {
		return false, fmt.Errorf("规则状态无效")
	}
	r, err := s.db.ExecContext(ctx, `UPDATE mochat_go_timeout_rules SET status=?,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=?`, status, time.Now(), id, tenantID, corpID)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}
func (s *MySQLStore) DeleteTimeoutRule(ctx context.Context, tenantID, corpID int, id int64) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_timeout_records WHERE tenant_id=? AND corp_id=? AND rule_id=?`, tenantID, corpID, id).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return false, fmt.Errorf("已有超时记录的规则不能删除，请停用规则")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	for _, table := range []string{"mochat_go_timeout_rule_strategies", "mochat_go_timeout_rule_quiet_periods", "mochat_go_timeout_rule_notify_targets"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE rule_id=? AND tenant_id=? AND corp_id=?", id, tenantID, corpID); err != nil {
			return false, err
		}
	}
	r, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_timeout_rules WHERE id=? AND tenant_id=? AND corp_id=?`, id, tenantID, corpID)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *MySQLStore) TimeoutSettings(ctx context.Context, tenantID, corpID int) (dashboard.TimeoutSettings, error) {
	var v dashboard.TimeoutSettings
	var groups, types []byte
	var updated time.Time
	err := s.db.QueryRowContext(ctx, `SELECT tenant_id,corp_id,closing_phrases_json,whitelist_message_types_json,updated_at FROM mochat_go_timeout_settings WHERE tenant_id=? AND corp_id=?`, tenantID, corpID).Scan(&v.TenantID, &v.CorpID, &groups, &types, &updated)
	if err == sql.ErrNoRows {
		return dashboard.TimeoutSettings{TenantID: int64(tenantID), CorpID: int64(corpID), ClosingPhraseGroups: [][]string{}, WhitelistMessageTypes: []string{}}, nil
	}
	if err != nil {
		return v, err
	}
	_ = json.Unmarshal(groups, &v.ClosingPhraseGroups)
	_ = json.Unmarshal(types, &v.WhitelistMessageTypes)
	v.UpdatedAt = updated.Format(time.RFC3339)
	return v, nil
}
func (s *MySQLStore) SaveTimeoutSettings(ctx context.Context, v dashboard.TimeoutSettings) error {
	if err := dashboard.ValidateTimeoutSettings(v); err != nil {
		return err
	}
	groups, _ := json.Marshal(v.ClosingPhraseGroups)
	types, _ := json.Marshal(v.WhitelistMessageTypes)
	_, err := s.db.ExecContext(ctx, `INSERT INTO mochat_go_timeout_settings (tenant_id,corp_id,closing_phrases_json,whitelist_message_types_json,updated_at) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE closing_phrases_json=VALUES(closing_phrases_json),whitelist_message_types_json=VALUES(whitelist_message_types_json),updated_at=VALUES(updated_at)`, v.TenantID, v.CorpID, groups, types, time.Now())
	return err
}

func (s *MySQLStore) TimeoutRecordPage(ctx context.Context, f dashboard.TimeoutRecordFilter) (dashboard.TimeoutRecordPage, error) {
	f.Page, f.PerPage = normalizeTimeoutPage(f.Page, f.PerPage)
	where := " WHERE r.corp_id = ?"
	args := []any{f.CorpID}
	if f.TenantID > 0 {
		where = " WHERE r.tenant_id = ? AND r.corp_id = ?"
		args = []any{f.TenantID, f.CorpID}
	}
	if f.Customer != "" {
		where += " AND (r.customer_id=? OR r.customer_name LIKE ?)"
		args = append(args, f.Customer, "%"+f.Customer+"%")
	}
	for column, value := range map[string]string{"risk_level": f.RiskLevel, "conversation_type": f.ConversationType, "audit_status": f.AuditStatus} {
		if value != "" {
			where += " AND r." + column + "=?"
			args = append(args, value)
		}
	}
	if f.RuleID > 0 {
		where += " AND r.rule_id=?"
		args = append(args, f.RuleID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_timeout_records r"+where, args...).Scan(&total); err != nil {
		return dashboard.TimeoutRecordPage{}, err
	}
	query := `SELECT r.id,r.tenant_id,r.corp_id,r.rule_id,r.strategy_id,COALESCE(q.name,''),r.conversation_type,r.conversation_id,r.customer_id,r.customer_name,r.employee_id,r.employee_name,r.trigger_message_id,r.trigger_message,r.message_type,r.timeout_seconds,r.risk_level,COALESCE(r.ai_summary,''),r.audit_status,r.assigned_employee_id,r.occurred_at,r.created_at FROM mochat_go_timeout_records r LEFT JOIN mochat_go_timeout_rules q ON q.id=r.rule_id` + where + ` ORDER BY r.occurred_at DESC,r.id DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, append(args, f.PerPage, (f.Page-1)*f.PerPage)...)
	if err != nil {
		return dashboard.TimeoutRecordPage{}, err
	}
	defer rows.Close()
	items := []dashboard.TimeoutRecord{}
	for rows.Next() {
		var v dashboard.TimeoutRecord
		var occurred, created time.Time
		if err = rows.Scan(&v.ID, &v.TenantID, &v.CorpID, &v.RuleID, &v.StrategyID, &v.RuleName, &v.ConversationType, &v.ConversationID, &v.CustomerID, &v.CustomerName, &v.EmployeeID, &v.EmployeeName, &v.TriggerMessageID, &v.TriggerMessage, &v.MessageType, &v.TimeoutSeconds, &v.RiskLevel, &v.AISummary, &v.AuditStatus, &v.AssignedEmployeeID, &occurred, &created); err != nil {
			return dashboard.TimeoutRecordPage{}, err
		}
		v.OccurredAt = occurred.Format(time.RFC3339)
		v.CreatedAt = created.Format(time.RFC3339)
		items = append(items, v)
	}
	return dashboard.TimeoutRecordPage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, rows.Err()
}

func (s *MySQLStore) AuditTimeoutRecords(ctx context.Context, tenantID, corpID, actorID int, ids []int64, action, remark string) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("请选择超时记录")
	}
	if action != "confirmed" && action != "ignored" && action != "closed" {
		return 0, fmt.Errorf("审计操作无效")
	}
	return s.updateTimeoutRecords(ctx, tenantID, corpID, actorID, ids, action, 0, remark)
}
func (s *MySQLStore) AssignTimeoutRecords(ctx context.Context, tenantID, corpID, actorID int, ids []int64, employeeID int64, remark string) (int64, error) {
	if len(ids) == 0 || employeeID <= 0 {
		return 0, fmt.Errorf("分派参数无效")
	}
	return s.updateTimeoutRecords(ctx, tenantID, corpID, actorID, ids, "assigned", employeeID, remark)
}
func (s *MySQLStore) updateTimeoutRecords(ctx context.Context, tenantID, corpID, actorID int, ids []int64, action string, employeeID int64, remark string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var total int64
	for _, id := range ids {
		query := `UPDATE mochat_go_timeout_records SET audit_status=? WHERE id=? AND tenant_id=? AND corp_id=?`
		args := []any{action, id, tenantID, corpID}
		if action == "assigned" {
			query = `UPDATE mochat_go_timeout_records SET assigned_employee_id=? WHERE id=? AND tenant_id=? AND corp_id=?`
			args = []any{employeeID, id, tenantID, corpID}
		}
		r, e := tx.ExecContext(ctx, query, args...)
		if e != nil {
			return 0, e
		}
		n, _ := r.RowsAffected()
		if n == 0 {
			continue
		}
		total += n
		if _, e = tx.ExecContext(ctx, `INSERT INTO mochat_go_timeout_record_audits (record_id,tenant_id,corp_id,actor_id,action,assigned_employee_id,remark,created_at) VALUES (?,?,?,?,?,?,?,?)`, id, tenantID, corpID, actorID, action, employeeID, strings.TrimSpace(remark), time.Now()); e != nil {
			return 0, e
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *MySQLStore) EvaluateTimeoutMessage(ctx context.Context, e dashboard.TimeoutEvaluation) (int, error) {
	if e.CorpID <= 0 || strings.TrimSpace(e.MessageID) == "" {
		return 0, fmt.Errorf("超时评估参数无效")
	}
	rules, err := s.TimeoutRulePage(ctx, dashboard.TimeoutRuleFilter{TenantID: e.TenantID, CorpID: e.CorpID, Page: 1, PerPage: 100})
	if err != nil {
		return 0, err
	}
	settings, err := s.TimeoutSettings(ctx, e.TenantID, e.CorpID)
	if err != nil {
		return 0, err
	}
	matches := dashboard.MatchTimeoutStrategies(e, rules.Items, settings)
	created := 0
	for _, v := range matches {
		occurred := e.EvaluatedAt
		if occurred.IsZero() {
			occurred = time.Now()
		}
		r, execErr := s.db.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_timeout_records (tenant_id,corp_id,rule_id,strategy_id,conversation_type,conversation_id,customer_id,customer_name,employee_id,employee_name,trigger_message_id,trigger_message,message_type,timeout_seconds,risk_level,ai_summary,audit_status,assigned_employee_id,occurred_at,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, v.TenantID, v.CorpID, v.RuleID, v.StrategyID, v.ConversationType, v.ConversationID, v.CustomerID, v.CustomerName, v.EmployeeID, v.EmployeeName, v.TriggerMessageID, v.TriggerMessage, v.MessageType, v.TimeoutSeconds, v.RiskLevel, nil, "pending", 0, occurred, time.Now())
		if execErr != nil {
			return created, execErr
		}
		n, _ := r.RowsAffected()
		if n == 0 {
			continue
		}
		created++
		recordID, _ := r.LastInsertId()
		_, _ = s.db.ExecContext(ctx, `UPDATE mochat_go_timeout_rules SET trigger_count=trigger_count+1,updated_at=? WHERE id=? AND tenant_id=? AND corp_id=?`, time.Now(), v.RuleID, v.TenantID, v.CorpID)
		strategy := timeoutStrategyByID(rules.Items, v.RuleID, v.StrategyID)
		if strategy.NotifyType != dashboard.TimeoutNotifyNone {
			targetType := "owner"
			targetID := v.EmployeeID
			if strategy.NotifyType == dashboard.TimeoutNotifyExtra {
				targetType = "extra"
				targetID = 0
			}
			_, _ = s.db.ExecContext(ctx, `INSERT IGNORE INTO mochat_go_timeout_notification_intents (record_id,strategy_id,tenant_id,corp_id,notify_type,target_type,target_id,status,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, recordID, v.StrategyID, v.TenantID, v.CorpID, strategy.NotifyType, targetType, targetID, "pending", time.Now())
		}
	}
	return created, nil
}
func timeoutStrategyByID(rules []dashboard.TimeoutRule, ruleID, strategyID int64) dashboard.TimeoutStrategy {
	for _, rule := range rules {
		if rule.ID == ruleID {
			for _, v := range rule.Strategies {
				if v.ID == strategyID {
					return v
				}
			}
		}
	}
	return dashboard.TimeoutStrategy{NotifyType: dashboard.TimeoutNotifyNone}
}

var _ dashboard.TimeoutWarningProvider = (*MySQLStore)(nil)
var _ dashboard.TimeoutRuleProviderWriter = (*MySQLStore)(nil)
var _ dashboard.TimeoutRecordProviderWriter = (*MySQLStore)(nil)
var _ dashboard.TimeoutSettingsProviderWriter = (*MySQLStore)(nil)
var _ dashboard.TimeoutEvaluatorProvider = (*MySQLStore)(nil)
