package dashboard

import (
	"testing"
	"time"
)

func TestMatchTimeoutStrategiesCreatesReachedLevels(t *testing.T) {
	messageAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.Local)
	rules := []TimeoutRule{{ID: 9, Status: TimeoutRuleEnabled, MonitorTarget: TimeoutMonitorAll, ConversationScopes: []string{"single"}, Strategies: []TimeoutStrategy{{ID: 11, TimeoutMinutes: 3, NotifyType: TimeoutNotifyOwner, RiskLevel: "low"}, {ID: 12, TimeoutMinutes: 10, NotifyType: TimeoutNotifyExtra, RiskLevel: "high"}}}}
	evaluation := TimeoutEvaluation{TenantID: 3, CorpID: 2, CustomerMessage: true, ConversationType: "single", ConversationID: "c-1", MessageID: "m-1", Message: "请帮我处理", MessageType: "text", CustomerID: "customer-1", EmployeeID: 7, MessageAt: messageAt, EvaluatedAt: messageAt.Add(11 * time.Minute)}

	records := MatchTimeoutStrategies(evaluation, rules, TimeoutSettings{})
	if len(records) != 2 || records[0].TimeoutSeconds != 660 || records[1].StrategyID != 12 {
		t.Fatalf("records=%#v", records)
	}
}

func TestMatchTimeoutStrategiesSkipsClosingPhraseWhitelistAndQuietPeriod(t *testing.T) {
	messageAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.Local)
	rule := TimeoutRule{ID: 9, Status: TimeoutRuleEnabled, MonitorTarget: TimeoutMonitorAll, ConversationScopes: []string{"single"}, Strategies: []TimeoutStrategy{{ID: 11, TimeoutMinutes: 3, NotifyType: TimeoutNotifyNone, RiskLevel: "low"}}}
	base := TimeoutEvaluation{CustomerMessage: true, ConversationType: "single", MessageID: "m-1", Message: "好的，谢谢", MessageType: "text", MessageAt: messageAt, EvaluatedAt: messageAt.Add(5 * time.Minute)}
	if got := MatchTimeoutStrategies(base, []TimeoutRule{rule}, TimeoutSettings{ClosingPhraseGroups: [][]string{{"好的"}}}); len(got) != 0 {
		t.Fatalf("closing phrase records=%#v", got)
	}
	base.Message = "图片"
	base.MessageType = "image"
	if got := MatchTimeoutStrategies(base, []TimeoutRule{rule}, TimeoutSettings{WhitelistMessageTypes: []string{"image"}}); len(got) != 0 {
		t.Fatalf("whitelist records=%#v", got)
	}
	rule.QuietPeriods = []TimeoutQuietPeriod{{Weekday: int(messageAt.Weekday()), StartTime: "09:00", EndTime: "11:00"}}
	base.MessageType = "text"
	if got := MatchTimeoutStrategies(base, []TimeoutRule{rule}, TimeoutSettings{}); len(got) != 0 {
		t.Fatalf("quiet records=%#v", got)
	}
}

func TestMatchTimeoutStrategiesRespectsMonitorTarget(t *testing.T) {
	now := time.Now()
	rule := TimeoutRule{ID: 1, Status: TimeoutRuleEnabled, MonitorTarget: TimeoutMonitorEmployee, MonitorTargetIDs: []int64{8}, ConversationScopes: []string{"single"}, Strategies: []TimeoutStrategy{{ID: 1, TimeoutMinutes: 3, NotifyType: TimeoutNotifyNone, RiskLevel: "low"}}}
	evaluation := TimeoutEvaluation{CustomerMessage: true, EmployeeID: 7, ConversationType: "single", MessageAt: now.Add(-time.Hour), EvaluatedAt: now}
	if got := MatchTimeoutStrategies(evaluation, []TimeoutRule{rule}, TimeoutSettings{}); len(got) != 0 {
		t.Fatalf("records=%#v", got)
	}
}
