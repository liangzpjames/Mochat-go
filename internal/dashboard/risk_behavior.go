package dashboard

import (
	"context"
	"fmt"
	"strings"
)

type RiskRuleFilter struct {
	TenantID int
	CorpID   int
	Name     string
	Page     int
	PerPage  int
}
type RiskRulePage struct {
	Items   []RiskRule `json:"items"`
	Total   int        `json:"total"`
	Page    int        `json:"page"`
	PerPage int        `json:"perPage"`
}
type RiskRecordFilter struct {
	TenantID            int
	CorpID              int
	RiskLevel           string
	Behavior            string
	ConversationType    string
	RuleID              int64
	Page                int
	PerPage             int
	AllowedEmployeeIDs  []int
	RestrictEmployeeIDs bool
}
type RiskRecordPage struct {
	Items   []RiskRecord `json:"items"`
	Total   int          `json:"total"`
	Page    int          `json:"page"`
	PerPage int          `json:"perPage"`
}
type RiskBehaviorProvider interface {
	RiskRulePage(context.Context, RiskRuleFilter) (RiskRulePage, error)
	RiskRecordPage(context.Context, RiskRecordFilter) (RiskRecordPage, error)
}

type RiskTenantResolver interface {
	TenantIDByCorpID(context.Context, int) (int, error)
}

type RiskRuleStatus string

const (
	RiskRuleEnabled  RiskRuleStatus = "enabled"
	RiskRuleDisabled RiskRuleStatus = "disabled"
)

type RiskSubject string

const (
	RiskSubjectEmployee RiskSubject = "employee"
	RiskSubjectCustomer RiskSubject = "customer"
	RiskSubjectBoth     RiskSubject = "both"
)

type RiskRuleStrategy struct {
	ID         int64  `json:"id"`
	Behavior   string `json:"behavior"`
	Pattern    string `json:"pattern"`
	NotifyType string `json:"notifyType"`
	RiskLevel  string `json:"riskLevel"`
}

type RiskRule struct {
	ID               int64              `json:"id"`
	TenantID         int64              `json:"tenantId"`
	CorpID           int64              `json:"corpId"`
	Name             string             `json:"name"`
	Status           RiskRuleStatus     `json:"status"`
	Subject          RiskSubject        `json:"subject"`
	Whitelist        []string           `json:"whitelist"`
	AIInsightEnabled bool               `json:"aiInsightEnabled"`
	TriggerCount     int64              `json:"triggerCount"`
	Strategies       []RiskRuleStrategy `json:"strategies"`
}

type RiskRuleProviderWriter interface {
	CreateRiskRule(context.Context, RiskRule) (int64, error)
	UpdateRiskRule(context.Context, RiskRule) (bool, error)
	SetRiskRuleStatus(context.Context, int, int64, RiskRuleStatus) (bool, error)
	DeleteRiskRule(context.Context, int, int64) (bool, error)
}

type RiskRecordProviderWriter interface {
	AuditRiskRecords(context.Context, int, int, int, []int64, string, string) (int64, error)
}

type RiskRecord struct {
	ID               int64          `json:"id"`
	TenantID         int64          `json:"tenantId"`
	CorpID           int64          `json:"corpId"`
	RuleID           int64          `json:"ruleId"`
	StrategyID       int64          `json:"strategyId"`
	Behavior         string         `json:"behavior"`
	RiskLevel        string         `json:"riskLevel"`
	ConversationType string         `json:"conversationType"`
	ConversationID   string         `json:"conversationId"`
	MessageID        string         `json:"messageId"`
	TriggerMessage   string         `json:"triggerMessage"`
	RelatedUser      map[string]any `json:"relatedUser"`
	AISummary        string         `json:"aiSummary"`
	AuditStatus      string         `json:"auditStatus"`
	OccurredAt       string         `json:"occurredAt"`
}

func ValidateRiskRule(rule RiskRule) error {
	if strings.TrimSpace(rule.Name) == "" || len([]rune(rule.Name)) > 80 {
		return fmt.Errorf("规则名称不能为空且不能超过 80 个字符")
	}
	if rule.Status != RiskRuleEnabled && rule.Status != RiskRuleDisabled {
		return fmt.Errorf("规则状态无效")
	}
	if rule.Subject != RiskSubjectEmployee && rule.Subject != RiskSubjectCustomer && rule.Subject != RiskSubjectBoth {
		return fmt.Errorf("监听主体无效")
	}
	seen := map[string]struct{}{}
	for _, strategy := range rule.Strategies {
		behavior := strings.TrimSpace(strategy.Behavior)
		if behavior == "" || strings.TrimSpace(strategy.Pattern) == "" {
			return fmt.Errorf("风险策略必须包含行为和匹配内容")
		}
		if _, ok := seen[behavior]; ok {
			return fmt.Errorf("同一风险行为只能配置一条策略")
		}
		seen[behavior] = struct{}{}
		if strategy.RiskLevel != "low" && strategy.RiskLevel != "medium" && strategy.RiskLevel != "high" {
			return fmt.Errorf("风险等级无效")
		}
	}
	return nil
}
