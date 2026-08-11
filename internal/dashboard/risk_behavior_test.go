package dashboard

import "testing"

func TestValidateRiskRuleRejectsDuplicateBehavior(t *testing.T) {
	err := ValidateRiskRule(RiskRule{Name: "敏感信息", Status: RiskRuleEnabled, Subject: RiskSubjectEmployee, Strategies: []RiskRuleStrategy{{Behavior: "phone", Pattern: "手机号", RiskLevel: "high"}, {Behavior: "phone", Pattern: "电话", RiskLevel: "high"}}})
	if err == nil {
		t.Fatal("expected duplicate behavior to be rejected")
	}
}

func TestValidateRiskRuleAcceptsArcRiskRuleShape(t *testing.T) {
	err := ValidateRiskRule(RiskRule{Name: "外部敏感信息", Status: RiskRuleEnabled, Subject: RiskSubjectBoth, Strategies: []RiskRuleStrategy{{Behavior: "keyword", Pattern: "报价", NotifyType: "none", RiskLevel: "medium"}}})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
