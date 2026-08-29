package dashboard

import (
	"strings"
	"testing"
)

func TestValidateRiskRuleRejectsDuplicateBehavior(t *testing.T) {
	err := ValidateRiskRule(RiskRule{Name: "敏感信息", Status: RiskRuleEnabled, Subject: RiskSubjectEmployee, Strategies: []RiskRuleStrategy{{Behavior: "sensitive_word", Pattern: "手机号", RiskLevel: "high"}, {Behavior: "sensitive_word", Pattern: "电话", RiskLevel: "high"}}})
	if err == nil {
		t.Fatal("expected duplicate behavior to be rejected")
	}
}

func TestValidateRiskRuleAcceptsArcRiskRuleShape(t *testing.T) {
	err := ValidateRiskRule(RiskRule{Name: "外部敏感信息", Status: RiskRuleEnabled, Subject: RiskSubjectBoth, Strategies: []RiskRuleStrategy{{Behavior: "sensitive_word", Pattern: "报价", NotifyType: "none", RiskLevel: "medium"}}})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateRiskRuleRejectsUnknownBehaviorAndUnboundedStrategies(t *testing.T) {
	base := RiskRule{Name: "风险行为", Status: RiskRuleEnabled, Subject: RiskSubjectBoth}
	base.Strategies = []RiskRuleStrategy{{Behavior: "unknown", Pattern: "报价", RiskLevel: "medium"}}
	if err := ValidateRiskRule(base); err == nil || !strings.Contains(err.Error(), "行为") {
		t.Fatalf("unknown behavior err=%v", err)
	}
	base.Strategies = []RiskRuleStrategy{
		{Behavior: "private_transaction", Pattern: "a", RiskLevel: "low"},
		{Behavior: "promise_rebate", Pattern: "b", RiskLevel: "medium"},
		{Behavior: "sensitive_word", Pattern: "c", RiskLevel: "high"},
		{Behavior: "unknown", Pattern: "d", RiskLevel: "low"},
	}
	if err := ValidateRiskRule(base); err == nil || !strings.Contains(err.Error(), "最多") {
		t.Fatalf("unbounded strategies err=%v", err)
	}
}
