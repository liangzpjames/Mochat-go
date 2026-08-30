package dashboard

import (
	"strings"
	"testing"
)

func TestValidateRiskRuleRequiresExactlyOneSupportedStrategy(t *testing.T) {
	for _, strategies := range [][]RiskRuleStrategy{
		nil,
		{{Behavior: "private_transaction", Pattern: "私下", RiskLevel: "high"}, {Behavior: "sensitive_word", Pattern: "电话", RiskLevel: "high"}},
	} {
		err := ValidateRiskRule(RiskRule{Name: "敏感信息", Status: RiskRuleEnabled, Subject: RiskSubjectEmployee, Strategies: strategies})
		if err == nil || !strings.Contains(err.Error(), "恰好配置 1 条") {
			t.Fatalf("strategies=%d err=%v", len(strategies), err)
		}
	}
}

func TestValidateRiskRuleAcceptsArcRiskRuleShape(t *testing.T) {
	err := ValidateRiskRule(RiskRule{Name: "外部敏感信息", Status: RiskRuleEnabled, Subject: RiskSubjectBoth, Strategies: []RiskRuleStrategy{{Behavior: "sensitive_word", Pattern: "报价", NotifyType: "none", RiskLevel: "medium"}}})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateRiskRuleRejectsUnknownBehavior(t *testing.T) {
	base := RiskRule{Name: "风险行为", Status: RiskRuleEnabled, Subject: RiskSubjectBoth}
	base.Strategies = []RiskRuleStrategy{{Behavior: "unknown", Pattern: "报价", RiskLevel: "medium"}}
	if err := ValidateRiskRule(base); err == nil || !strings.Contains(err.Error(), "行为") {
		t.Fatalf("unknown behavior err=%v", err)
	}
}
