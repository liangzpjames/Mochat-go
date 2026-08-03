package dashboard

import "testing"

func TestMatchRiskStrategiesIsCaseInsensitiveAndSkipsDisabledRules(t *testing.T) {
	rules := []RiskRule{{ID: 1, Status: RiskRuleEnabled, Strategies: []RiskRuleStrategy{{ID: 2, Behavior: "keyword", Pattern: "报价", RiskLevel: "medium"}}}, {ID: 3, Status: RiskRuleDisabled, Strategies: []RiskRuleStrategy{{ID: 4, Behavior: "keyword", Pattern: "报价", RiskLevel: "high"}}}}
	got := MatchRiskStrategies(RiskMessage{CorpID: 7, MessageID: "m1", Content: "请发送报价单"}, rules)
	if len(got) != 1 || got[0].RuleID != 1 {
		t.Fatalf("unexpected matches: %+v", got)
	}
}
