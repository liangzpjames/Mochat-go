package aiinsight

import (
	"strings"
	"testing"
)

func validSessionJSON() string {
	return `{"schemaVersion":1,"summary":"客户关注交付时间","customer":{"qualityLevel":"high","qualityReason":"客户主动确认交付时间","purchaseIntent":{"level":"high","score":80,"reason":"明确询价并确认交付","evidenceMessageIds":["msg:inside"]},"churnRisk":{"level":"low","score":20,"reason":"沟通积极","evidenceMessageIds":[]},"keywords":["交付"],"explicitNeeds":["确认交付时间"],"implicitNeeds":[],"emotion":{"label":"neutral","reason":"语气平稳","evidenceMessageIds":[]},"recommendedReply":"确认排期并同步节点","actions":["跟进排期"],"notes":[]},"employeeQa":{"score":85,"dimensions":[{"name":"响应","score":90,"comment":"响应及时"}],"strengths":["响应及时"],"issues":[],"suggestions":[]}}`
}

func TestParseSessionAnalysisAcceptsValidJSONAndCodeFence(t *testing.T) {
	result, err := ParseSessionAnalysisResult("```json\n"+validSessionJSON()+"\n```", map[string]struct{}{"msg:inside": {}})
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 1 || result.Summary == "" || result.Customer.PurchaseIntent.Score == nil || *result.Customer.PurchaseIntent.Score != 80 {
		t.Fatalf("unexpected parsed result: %#v", result)
	}
}

func TestParseSessionAnalysisRejectsMissingSchemaVersion(t *testing.T) {
	raw := strings.Replace(validSessionJSON(), `{"schemaVersion":1,`, `{`, 1)
	if _, err := ParseSessionAnalysisResult(raw, map[string]struct{}{"msg:inside": {}}); err == nil {
		t.Fatal("expected missing schemaVersion error")
	}
}

func TestParseSessionAnalysisRejectsEvidenceOutsideSourceWindow(t *testing.T) {
	raw := strings.Replace(validSessionJSON(), `"msg:inside"`, `"msg:outside"`, 1)
	_, err := ParseSessionAnalysisResult(raw, map[string]struct{}{"msg:inside": {}})
	if err == nil || !strings.Contains(err.Error(), "msg:outside") {
		t.Fatalf("ParseSessionAnalysisResult error = %v", err)
	}
}

func TestParseSessionAnalysisRejectsInvalidEnumAndScore(t *testing.T) {
	raw := strings.Replace(validSessionJSON(), `"level":"high","score":80`, `"level":"unknown","score":101`, 1)
	if _, err := ParseSessionAnalysisResult(raw, map[string]struct{}{"msg:inside": {}}); err == nil {
		t.Fatal("expected invalid level/score error")
	}
}

func TestParseSessionAnalysisRejectsEmptySummary(t *testing.T) {
	raw := strings.Replace(validSessionJSON(), `"summary":"客户关注交付时间"`, `"summary":""`, 1)
	if _, err := ParseSessionAnalysisResult(raw, map[string]struct{}{"msg:inside": {}}); err == nil {
		t.Fatal("expected empty summary error")
	}
}

func TestParseSmartAnalysisRejectsInvalidConfidenceAndEvidence(t *testing.T) {
	raw := `{"schemaVersion":1,"conclusion":"客户有明确采购意向","matched":true,"confidence":1.2,"evidenceMessageIds":["msg:outside"],"recommendations":["安排演示"]}`
	if _, err := ParseSmartAnalysisResult(raw, map[string]struct{}{"msg:inside": {}}); err == nil {
		t.Fatal("expected confidence/evidence error")
	}
}

func TestParseSmartAnalysisAcceptsNullableConfidence(t *testing.T) {
	raw := `{"schemaVersion":1,"conclusion":"暂未命中规则","matched":false,"confidence":null,"evidenceMessageIds":[],"recommendations":[]}`
	result, err := ParseSmartAnalysisResult(raw, map[string]struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Matched || result.Conclusion == "" || result.Confidence != nil {
		t.Fatalf("unexpected parsed smart result: %#v", result)
	}
}
