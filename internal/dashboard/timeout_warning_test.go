package dashboard

import "testing"

func TestValidateTimeoutRuleAcceptsCompleteRule(t *testing.T) {
	rule := TimeoutRule{Name: "客户回复超时", Status: TimeoutRuleEnabled, MonitorTarget: TimeoutMonitorAll, ConversationScopes: []string{"single"}, Strategies: []TimeoutStrategy{{TimeoutMinutes: 3, NotifyType: TimeoutNotifyOwner, RiskLevel: "low"}}}
	if err := ValidateTimeoutRule(rule); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTimeoutRuleRejectsInvalidStrategy(t *testing.T) {
	for _, strategies := range [][]TimeoutStrategy{
		{},
		{{TimeoutMinutes: 2, NotifyType: TimeoutNotifyNone, RiskLevel: "low"}},
		{{TimeoutMinutes: 3, NotifyType: TimeoutNotifyNone, RiskLevel: "low"}, {TimeoutMinutes: 3, NotifyType: TimeoutNotifyNone, RiskLevel: "medium"}},
	} {
		rule := TimeoutRule{Name: "规则", Status: TimeoutRuleEnabled, MonitorTarget: TimeoutMonitorAll, ConversationScopes: []string{"single"}, Strategies: strategies}
		if err := ValidateTimeoutRule(rule); err == nil {
			t.Fatalf("expected strategies to fail: %#v", strategies)
		}
	}
}

func TestValidateTimeoutSettingsLimitsPhraseGroups(t *testing.T) {
	settings := TimeoutSettings{ClosingPhraseGroups: [][]string{{"好的"}, {"谢谢"}, {"再见"}, {"收到"}, {"明白"}, {"结束"}}}
	if err := ValidateTimeoutSettings(settings); err == nil {
		t.Fatal("expected too many phrase groups to fail")
	}
}
