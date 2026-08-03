package dashboard

import (
	"context"
	"strings"
	"time"
)

type TimeoutEvaluation struct {
	TenantID         int       `json:"tenantId"`
	CorpID           int       `json:"corpId"`
	CustomerMessage  bool      `json:"customerMessage"`
	Replied          bool      `json:"replied"`
	ConversationType string    `json:"conversationType"`
	ConversationID   string    `json:"conversationId"`
	CustomerID       string    `json:"customerId"`
	CustomerName     string    `json:"customerName"`
	EmployeeID       int64     `json:"employeeId"`
	EmployeeName     string    `json:"employeeName"`
	DepartmentIDs    []int64   `json:"departmentIds"`
	MessageID        string    `json:"messageId"`
	Message          string    `json:"message"`
	MessageType      string    `json:"messageType"`
	MessageAt        time.Time `json:"messageAt"`
	EvaluatedAt      time.Time `json:"evaluatedAt"`
}

type TimeoutEvaluatorProvider interface {
	EvaluateTimeoutMessage(context.Context, TimeoutEvaluation) (int, error)
}

func MatchTimeoutStrategies(e TimeoutEvaluation, rules []TimeoutRule, settings TimeoutSettings) []TimeoutRecord {
	if !e.CustomerMessage || e.Replied || e.EvaluatedAt.Before(e.MessageAt) || timeoutMessageExcluded(e, settings) {
		return []TimeoutRecord{}
	}
	seconds := int64(e.EvaluatedAt.Sub(e.MessageAt).Seconds())
	result := []TimeoutRecord{}
	for _, rule := range rules {
		if rule.Status != TimeoutRuleEnabled || !containsString(rule.ConversationScopes, e.ConversationType) || !timeoutTargetMatches(rule, e) || inTimeoutQuietPeriod(rule.QuietPeriods, e.MessageAt) {
			continue
		}
		for _, strategy := range rule.Strategies {
			if seconds < int64(strategy.TimeoutMinutes*60) {
				continue
			}
			result = append(result, TimeoutRecord{TenantID: int64(e.TenantID), CorpID: int64(e.CorpID), RuleID: rule.ID, StrategyID: strategy.ID, RuleName: rule.Name, ConversationType: e.ConversationType, ConversationID: e.ConversationID, CustomerID: e.CustomerID, CustomerName: e.CustomerName, EmployeeID: e.EmployeeID, EmployeeName: e.EmployeeName, TriggerMessageID: e.MessageID, TriggerMessage: e.Message, MessageType: e.MessageType, TimeoutSeconds: seconds, RiskLevel: strategy.RiskLevel, AuditStatus: "pending", OccurredAt: e.EvaluatedAt.Format(time.RFC3339)})
		}
	}
	return result
}

func timeoutMessageExcluded(e TimeoutEvaluation, settings TimeoutSettings) bool {
	if containsString(settings.WhitelistMessageTypes, e.MessageType) {
		return true
	}
	content := strings.ToLower(strings.TrimSpace(e.Message))
	for _, group := range settings.ClosingPhraseGroups {
		for _, phrase := range group {
			if p := strings.ToLower(strings.TrimSpace(phrase)); p != "" && strings.Contains(content, p) {
				return true
			}
		}
	}
	return false
}
func timeoutTargetMatches(rule TimeoutRule, e TimeoutEvaluation) bool {
	if rule.MonitorTarget == TimeoutMonitorAll {
		return true
	}
	if rule.MonitorTarget == TimeoutMonitorEmployee {
		return containsInt64(rule.MonitorTargetIDs, e.EmployeeID)
	}
	for _, departmentID := range e.DepartmentIDs {
		if containsInt64(rule.MonitorTargetIDs, departmentID) {
			return true
		}
	}
	return false
}
func inTimeoutQuietPeriod(periods []TimeoutQuietPeriod, at time.Time) bool {
	clock := at.Format("15:04")
	weekday := int(at.Weekday())
	for _, period := range periods {
		if (period.Weekday == 0 || period.Weekday == weekday) && period.StartTime <= clock && clock <= period.EndTime {
			return true
		}
	}
	return false
}
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
func containsInt64(values []int64, want int64) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
