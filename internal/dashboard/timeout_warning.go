package dashboard

import (
	"context"
	"fmt"
	"strings"
)

type TimeoutRuleStatus string
type TimeoutMonitorTarget string
type TimeoutNotifyType string

const (
	TimeoutRuleEnabled       TimeoutRuleStatus    = "enabled"
	TimeoutRuleDisabled      TimeoutRuleStatus    = "disabled"
	TimeoutMonitorAll        TimeoutMonitorTarget = "all"
	TimeoutMonitorEmployee   TimeoutMonitorTarget = "employee"
	TimeoutMonitorDepartment TimeoutMonitorTarget = "department"
	TimeoutNotifyNone        TimeoutNotifyType    = "none"
	TimeoutNotifyOwner       TimeoutNotifyType    = "owner"
	TimeoutNotifyExtra       TimeoutNotifyType    = "extra"
)

type TimeoutStrategy struct {
	ID             int64             `json:"id"`
	TimeoutMinutes int               `json:"timeoutMinutes"`
	NotifyType     TimeoutNotifyType `json:"notifyType"`
	RiskLevel      string            `json:"riskLevel"`
	SortOrder      int               `json:"sortOrder"`
}

type TimeoutQuietPeriod struct {
	ID        int64  `json:"id"`
	Weekday   int    `json:"weekday"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type TimeoutNotifyTarget struct {
	ID         int64  `json:"id"`
	TargetType string `json:"targetType"`
	TargetID   int64  `json:"targetId"`
}

type TimeoutRule struct {
	ID                 int64                 `json:"id"`
	TenantID           int64                 `json:"tenantId"`
	CorpID             int64                 `json:"corpId"`
	Name               string                `json:"name"`
	Status             TimeoutRuleStatus     `json:"status"`
	MonitorTarget      TimeoutMonitorTarget  `json:"monitorTarget"`
	MonitorTargetIDs   []int64               `json:"monitorTargetIds"`
	ConversationScopes []string              `json:"conversationScopes"`
	AIInsightEnabled   bool                  `json:"aiInsightEnabled"`
	TriggerCount       int64                 `json:"triggerCount"`
	Strategies         []TimeoutStrategy     `json:"strategies"`
	QuietPeriods       []TimeoutQuietPeriod  `json:"quietPeriods"`
	NotifyTargets      []TimeoutNotifyTarget `json:"notifyTargets"`
	CreatedAt          string                `json:"createdAt"`
	UpdatedAt          string                `json:"updatedAt"`
}

type TimeoutSettings struct {
	TenantID              int64      `json:"tenantId"`
	CorpID                int64      `json:"corpId"`
	ClosingPhraseGroups   [][]string `json:"closingPhraseGroups"`
	WhitelistMessageTypes []string   `json:"whitelistMessageTypes"`
	UpdatedAt             string     `json:"updatedAt"`
}

type TimeoutRecord struct {
	ID                 int64  `json:"id"`
	TenantID           int64  `json:"tenantId"`
	CorpID             int64  `json:"corpId"`
	RuleID             int64  `json:"ruleId"`
	StrategyID         int64  `json:"strategyId"`
	RuleName           string `json:"ruleName"`
	ConversationType   string `json:"conversationType"`
	ConversationID     string `json:"conversationId"`
	CustomerID         string `json:"customerId"`
	CustomerName       string `json:"customerName"`
	EmployeeID         int64  `json:"employeeId"`
	EmployeeName       string `json:"employeeName"`
	TriggerMessageID   string `json:"triggerMessageId"`
	TriggerMessage     string `json:"triggerMessage"`
	MessageType        string `json:"messageType"`
	TimeoutSeconds     int64  `json:"timeoutSeconds"`
	RiskLevel          string `json:"riskLevel"`
	AISummary          string `json:"aiSummary"`
	AuditStatus        string `json:"auditStatus"`
	AssignedEmployeeID int64  `json:"assignedEmployeeId"`
	OccurredAt         string `json:"occurredAt"`
	CreatedAt          string `json:"createdAt"`
}

type TimeoutRuleFilter struct {
	TenantID, CorpID int
	Name             string
	Page, PerPage    int
}
type TimeoutRecordFilter struct {
	TenantID, CorpID                                   int
	Customer, RiskLevel, ConversationType, AuditStatus string
	RuleID                                             int64
	Page, PerPage                                      int
	AllowedEmployeeIDs                                 []int
	RestrictEmployeeIDs                                bool
}
type TimeoutRulePage struct {
	Items                []TimeoutRule `json:"items"`
	Total, Page, PerPage int
}
type TimeoutRecordPage struct {
	Items                []TimeoutRecord `json:"items"`
	Total, Page, PerPage int
}

type TimeoutWarningProvider interface {
	TimeoutRulePage(context.Context, TimeoutRuleFilter) (TimeoutRulePage, error)
	TimeoutRecordPage(context.Context, TimeoutRecordFilter) (TimeoutRecordPage, error)
	TimeoutSettings(context.Context, int, int) (TimeoutSettings, error)
}
type TimeoutRuleProviderWriter interface {
	CreateTimeoutRule(context.Context, TimeoutRule) (int64, error)
	UpdateTimeoutRule(context.Context, TimeoutRule) (bool, error)
	SetTimeoutRuleStatus(context.Context, int, int, int64, TimeoutRuleStatus) (bool, error)
	DeleteTimeoutRule(context.Context, int, int, int64) (bool, error)
}
type TimeoutRecordProviderWriter interface {
	AuditTimeoutRecords(context.Context, int, int, int, []int64, string, string) (int64, error)
	AssignTimeoutRecords(context.Context, int, int, int, []int64, int64, string) (int64, error)
}
type TimeoutSettingsProviderWriter interface {
	SaveTimeoutSettings(context.Context, TimeoutSettings) error
}

func ValidateTimeoutRule(rule TimeoutRule) error {
	if strings.TrimSpace(rule.Name) == "" || len([]rune(rule.Name)) > 80 {
		return fmt.Errorf("规则名称不能为空且不能超过 80 个字符")
	}
	if rule.Status != TimeoutRuleEnabled && rule.Status != TimeoutRuleDisabled {
		return fmt.Errorf("规则状态无效")
	}
	if rule.MonitorTarget != TimeoutMonitorAll && rule.MonitorTarget != TimeoutMonitorEmployee && rule.MonitorTarget != TimeoutMonitorDepartment {
		return fmt.Errorf("监听对象无效")
	}
	if rule.MonitorTarget != TimeoutMonitorAll && len(rule.MonitorTargetIDs) == 0 {
		return fmt.Errorf("请选择监听员工或部门")
	}
	if len(rule.ConversationScopes) == 0 {
		return fmt.Errorf("请选择监听范围")
	}
	for _, scope := range rule.ConversationScopes {
		if scope != "single" && scope != "group" {
			return fmt.Errorf("监听范围无效")
		}
	}
	if len(rule.Strategies) < 1 || len(rule.Strategies) > 5 {
		return fmt.Errorf("超时策略数量必须为 1 到 5 条")
	}
	seen := map[int]bool{}
	for _, strategy := range rule.Strategies {
		if strategy.TimeoutMinutes < 3 || strategy.TimeoutMinutes > 180 {
			return fmt.Errorf("超时时长必须为 3 到 180 分钟")
		}
		if seen[strategy.TimeoutMinutes] {
			return fmt.Errorf("超时时长不能重复")
		}
		seen[strategy.TimeoutMinutes] = true
		if strategy.NotifyType != TimeoutNotifyNone && strategy.NotifyType != TimeoutNotifyOwner && strategy.NotifyType != TimeoutNotifyExtra {
			return fmt.Errorf("通知类型无效")
		}
		if strategy.RiskLevel != "low" && strategy.RiskLevel != "medium" && strategy.RiskLevel != "high" {
			return fmt.Errorf("风险等级无效")
		}
	}
	if len(rule.QuietPeriods) > 5 {
		return fmt.Errorf("静音时段不能超过 5 条")
	}
	for _, period := range rule.QuietPeriods {
		if period.Weekday < 0 || period.Weekday > 7 || len(period.StartTime) != 5 || len(period.EndTime) != 5 {
			return fmt.Errorf("静音时段无效")
		}
	}
	return nil
}

func ValidateTimeoutSettings(settings TimeoutSettings) error {
	if len(settings.ClosingPhraseGroups) > 5 {
		return fmt.Errorf("结束语词表不能超过 5 个")
	}
	for _, group := range settings.ClosingPhraseGroups {
		if len(group) > 20 {
			return fmt.Errorf("单个结束语词表不能超过 20 个词")
		}
	}
	return nil
}
