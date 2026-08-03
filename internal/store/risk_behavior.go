package store

import (
	"context"
	"database/sql"
	"encoding/json"
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
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RiskRulePage{}, err
	}
	return dashboard.RiskRulePage{Items: items, Total: total, Page: f.Page, PerPage: f.PerPage}, nil
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

var _ dashboard.RiskBehaviorProvider = (*MySQLStore)(nil)
var _ = sql.ErrNoRows
