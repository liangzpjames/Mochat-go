package dashboard

import (
	"context"
	"strings"
)

type RiskMessage struct {
	TenantID         int            `json:"tenantId"`
	CorpID           int            `json:"corpId"`
	MessageID        string         `json:"messageId"`
	ConversationID   string         `json:"conversationId"`
	ConversationType string         `json:"conversationType"`
	Content          string         `json:"content"`
	RelatedUser      map[string]any `json:"relatedUser"`
	OccurredAt       string         `json:"occurredAt"`
}

type RiskEvaluatorProvider interface {
	EvaluateRiskMessage(context.Context, RiskMessage) (int, error)
}

func MatchRiskStrategies(message RiskMessage, rules []RiskRule) []RiskRecord {
	result := []RiskRecord{}
	content := strings.ToLower(message.Content)
	for _, rule := range rules {
		if rule.Status != RiskRuleEnabled {
			continue
		}
		for _, strategy := range rule.Strategies {
			pattern := strings.ToLower(strings.TrimSpace(strategy.Pattern))
			if pattern == "" || !strings.Contains(content, pattern) {
				continue
			}
			result = append(result, RiskRecord{TenantID: int64(message.TenantID), CorpID: int64(message.CorpID), RuleID: rule.ID, StrategyID: strategy.ID, Behavior: strategy.Behavior, RiskLevel: strategy.RiskLevel, ConversationType: message.ConversationType, ConversationID: message.ConversationID, MessageID: message.MessageID, TriggerMessage: message.Content, RelatedUser: message.RelatedUser, AuditStatus: "pending", OccurredAt: message.OccurredAt})
		}
	}
	return result
}
