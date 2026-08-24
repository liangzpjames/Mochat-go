package aiinsight

import (
	"encoding/json"
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

func validSessionV2JSON() string {
	return `{"schemaVersion":2,"summary":"客户关注交付时间","customer":{"qualityLevel":"high","qualityScore":82.5,"qualityReason":"客户主动确认交付时间","purchaseIntent":{"level":"high","score":80,"reason":"明确询价并确认交付","evidenceMessageIds":["msg:inside"],"dimensions":[{"name":"询价明确度","weight":0.6,"score":90,"reason":"主动询价","evidenceMessageIds":["msg:inside"]}]},"churnRisk":{"level":"insufficient","score":null,"reason":"证据不足","evidenceMessageIds":[],"dimensions":[]},"keywords":["交付"],"explicitNeeds":["确认交付时间"],"implicitNeeds":[],"emotion":{"label":"neutral","reason":"语气平稳","evidenceMessageIds":[]},"recommendedReply":"确认排期并同步节点","actions":["跟进排期"],"notes":[]},"employeeQa":{"score":85,"dimensions":[{"name":"响应","score":90,"comment":"响应及时"}],"strengths":["响应及时"],"issues":[],"suggestions":[],"unresolvedCustomerIssues":[{"title":"交付日期未确认","reason":"员工尚未给出日期","evidenceMessageIds":["msg:inside"]}],"unresolvedObjections":[]}}`
}

func TestParseSessionAnalysisAcceptsV1AndV2WithoutConvertingNullableScores(t *testing.T) {
	allowed := map[string]struct{}{"msg:inside": {}}
	if _, err := ParseSessionAnalysisResult(validSessionJSON(), allowed); err != nil {
		t.Fatalf("v1 error = %v", err)
	}
	result, err := ParseSessionAnalysisResult(validSessionV2JSON(), allowed)
	if err != nil {
		t.Fatalf("v2 error = %v", err)
	}
	if result.SchemaVersion != 2 || result.Customer.QualityScore == nil || *result.Customer.QualityScore != 82.5 {
		t.Fatalf("v2 result = %#v", result)
	}
	if result.Customer.ChurnRisk.Score != nil {
		t.Fatalf("nullable score was converted: %#v", result.Customer.ChurnRisk.Score)
	}
}

func TestParseSessionAnalysisRejectsInvalidV2Quantification(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
	}{
		{name: "quality score", old: `"qualityScore":82.5`, new: `"qualityScore":101`},
		{name: "dimension name", old: `"name":"询价明确度"`, new: `"name":" "`},
		{name: "dimension weight", old: `"weight":0.6`, new: `"weight":1.1`},
		{name: "dimension score", old: `"score":90`, new: `"score":-1`},
		{name: "dimension evidence", old: `"evidenceMessageIds":["msg:inside"]`, new: `"evidenceMessageIds":["msg:outside"]`},
		{name: "unresolved title", old: `"title":"交付日期未确认"`, new: `"title":""`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := strings.Replace(validSessionV2JSON(), test.old, test.new, 1)
			if _, err := ParseSessionAnalysisResult(raw, map[string]struct{}{"msg:inside": {}}); err == nil {
				t.Fatalf("expected invalid v2 %s", test.name)
			}
		})
	}
}

func validSmartV2JSON() string {
	return `{"schemaVersion":2,"conclusion":"发现退款风险","matched":true,"matchScore":88,"confidenceScore":76.5,"evidenceCoverageScore":50,"priorityScore":91,"priorityLevel":"high","dimensions":[{"name":"退款意向","weight":1,"score":88,"reason":"客户明确提出退款","evidenceMessageIds":["msg:inside"]}],"evidenceMessageIds":["msg:inside"],"recommendations":["核对审批"]}`
}

func TestParseSmartAnalysisAcceptsV1AndV2AndLeavesV1ScoresNil(t *testing.T) {
	v1, err := ParseSmartAnalysisResult(`{"schemaVersion":1,"conclusion":"暂未命中","matched":false,"confidence":null,"evidenceMessageIds":[],"recommendations":[]}`, map[string]struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if v1.MatchScore != nil || v1.ConfidenceScore != nil || v1.EvidenceCoverageScore != nil || v1.PriorityScore != nil {
		t.Fatalf("v1 aggregate scores must stay nil: %#v", v1)
	}
	v2, err := ParseSmartAnalysisResult(validSmartV2JSON(), map[string]struct{}{"msg:inside": {}})
	if err != nil || v2.PriorityLevel != "high" || v2.MatchScore == nil || *v2.MatchScore != 88 {
		t.Fatalf("v2 result=%#v error=%v", v2, err)
	}
}

func TestParseSmartAnalysisRejectsInvalidV2Quantification(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
	}{
		{name: "match score", old: `"matchScore":88`, new: `"matchScore":101`},
		{name: "confidence score", old: `"confidenceScore":76.5`, new: `"confidenceScore":-1`},
		{name: "coverage score", old: `"evidenceCoverageScore":50`, new: `"evidenceCoverageScore":101`},
		{name: "priority score", old: `"priorityScore":91`, new: `"priorityScore":101`},
		{name: "priority", old: `"priorityLevel":"high"`, new: `"priorityLevel":"urgent"`},
		{name: "dimension weight", old: `"weight":1`, new: `"weight":1.01`},
		{name: "dimension evidence", old: `"evidenceMessageIds":["msg:inside"]`, new: `"evidenceMessageIds":["msg:outside"]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := strings.Replace(validSmartV2JSON(), test.old, test.new, 1)
			if _, err := ParseSmartAnalysisResult(raw, map[string]struct{}{"msg:inside": {}}); err == nil {
				t.Fatalf("expected invalid v2 %s", test.name)
			}
		})
	}
}

func TestParseSmartAnalysisV2RequiresPreciseScoreFieldsAndRejectsLegacyConfidence(t *testing.T) {
	missing := strings.Replace(validSmartV2JSON(), `"priorityScore":91,`, "", 1)
	if _, err := ParseSmartAnalysisResult(missing, map[string]struct{}{"msg:inside": {}}); err == nil {
		t.Fatal("expected missing priorityScore error")
	}
	legacy := strings.Replace(validSmartV2JSON(), `"matched":true,`, `"matched":true,"confidence":0.9,`, 1)
	if _, err := ParseSmartAnalysisResult(legacy, map[string]struct{}{"msg:inside": {}}); err == nil {
		t.Fatal("expected v2 legacy confidence error")
	}
}

func TestParseAnalysisV1RejectsV2OnlyFields(t *testing.T) {
	allowed := map[string]struct{}{"msg:inside": {}}
	tests := []struct {
		name  string
		parse func(string) error
		raw   string
	}{
		{
			name: "session quality score",
			parse: func(raw string) error {
				_, err := ParseSessionAnalysisResult(raw, allowed)
				return err
			},
			raw: strings.Replace(validSessionJSON(), `"qualityLevel":"high",`, `"qualityLevel":"high","qualityScore":80,`, 1),
		},
		{
			name: "session quantified dimensions",
			parse: func(raw string) error {
				_, err := ParseSessionAnalysisResult(raw, allowed)
				return err
			},
			raw: strings.Replace(validSessionJSON(), `"evidenceMessageIds":["msg:inside"]`, `"evidenceMessageIds":["msg:inside"],"dimensions":[]`, 1),
		},
		{
			name: "session unresolved issues",
			parse: func(raw string) error {
				_, err := ParseSessionAnalysisResult(raw, allowed)
				return err
			},
			raw: strings.Replace(validSessionJSON(), `"suggestions":[]`, `"suggestions":[],"unresolvedCustomerIssues":[]`, 1),
		},
		{
			name: "smart quantified scores",
			parse: func(raw string) error {
				_, err := ParseSmartAnalysisResult(raw, allowed)
				return err
			},
			raw: `{"schemaVersion":1,"conclusion":"暂未命中","matched":false,"confidence":null,"matchScore":null,"evidenceMessageIds":[],"recommendations":[]}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.parse(test.raw); err == nil {
				t.Fatal("expected schema-version field pollution error")
			}
		})
	}
}

func TestParseAnalysisV2RequiresNullableScoresAndDimensionFields(t *testing.T) {
	allowed := map[string]struct{}{"msg:inside": {}}
	tests := []struct {
		name  string
		parse func(string) error
		raw   string
	}{
		{
			name: "session qualityScore",
			parse: func(raw string) error {
				_, err := ParseSessionAnalysisResult(raw, allowed)
				return err
			},
			raw: strings.Replace(validSessionV2JSON(), `"qualityScore":82.5,`, "", 1),
		},
		{
			name: "session dimension weight",
			parse: func(raw string) error {
				_, err := ParseSessionAnalysisResult(raw, allowed)
				return err
			},
			raw: strings.Replace(validSessionV2JSON(), `"weight":0.6,`, "", 1),
		},
		{
			name: "session dimension score",
			parse: func(raw string) error {
				_, err := ParseSessionAnalysisResult(raw, allowed)
				return err
			},
			raw: strings.Replace(validSessionV2JSON(), `"score":90,`, "", 1),
		},
		{
			name: "session dimension null weight",
			parse: func(raw string) error {
				_, err := ParseSessionAnalysisResult(raw, allowed)
				return err
			},
			raw: strings.Replace(validSessionV2JSON(), `"weight":0.6`, `"weight":null`, 1),
		},
		{
			name: "smart dimension weight",
			parse: func(raw string) error {
				_, err := ParseSmartAnalysisResult(raw, allowed)
				return err
			},
			raw: strings.Replace(validSmartV2JSON(), `"weight":1,`, "", 1),
		},
		{
			name: "smart dimension score",
			parse: func(raw string) error {
				_, err := ParseSmartAnalysisResult(raw, allowed)
				return err
			},
			raw: strings.Replace(validSmartV2JSON(), `"score":88,`, "", 1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.parse(test.raw); err == nil {
				t.Fatal("expected required v2 field error")
			}
		})
	}
}

func TestAnalysisResultRoundTripPreservesV2NullsWithoutPollutingV1(t *testing.T) {
	allowed := map[string]struct{}{"msg:inside": {}}
	sessionV2Raw := strings.NewReplacer(
		`"qualityScore":82.5`, `"qualityScore":null`,
		`"score":90`, `"score":null`,
	).Replace(validSessionV2JSON())
	sessionV2, err := ParseSessionAnalysisResult(sessionV2Raw, allowed)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONPathsAreNull(t, sessionV2, "customer.qualityScore", "customer.purchaseIntent.dimensions.0.score")

	smartV2Raw := strings.NewReplacer(
		`"matchScore":88`, `"matchScore":null`,
		`"confidenceScore":76.5`, `"confidenceScore":null`,
		`"evidenceCoverageScore":50`, `"evidenceCoverageScore":null`,
		`"priorityScore":91`, `"priorityScore":null`,
		`"score":88`, `"score":null`,
	).Replace(validSmartV2JSON())
	smartV2, err := ParseSmartAnalysisResult(smartV2Raw, allowed)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONPathsAreNull(t, smartV2, "matchScore", "confidenceScore", "evidenceCoverageScore", "priorityScore", "dimensions.0.score")

	sessionV1, err := ParseSessionAnalysisResult(validSessionJSON(), allowed)
	if err != nil {
		t.Fatal(err)
	}
	sessionV1JSON, err := json.Marshal(sessionV1)
	if err != nil {
		t.Fatal(err)
	}
	var sessionV1Object map[string]any
	if err := json.Unmarshal(sessionV1JSON, &sessionV1Object); err != nil {
		t.Fatal(err)
	}
	customer := sessionV1Object["customer"].(map[string]any)
	employeeQA := sessionV1Object["employeeQa"].(map[string]any)
	if _, ok := customer["qualityScore"]; ok {
		t.Fatalf("v1 session result injected qualityScore: %s", sessionV1JSON)
	}
	for _, assessmentName := range []string{"purchaseIntent", "churnRisk"} {
		assessment := customer[assessmentName].(map[string]any)
		if _, ok := assessment["dimensions"]; ok {
			t.Fatalf("v1 session result injected %s.dimensions: %s", assessmentName, sessionV1JSON)
		}
	}
	for _, field := range []string{"unresolvedCustomerIssues", "unresolvedObjections"} {
		if _, ok := employeeQA[field]; ok {
			t.Fatalf("v1 session result injected %s: %s", field, sessionV1JSON)
		}
	}

	smartV1, err := ParseSmartAnalysisResult(`{"schemaVersion":1,"conclusion":"暂未命中","matched":false,"confidence":null,"evidenceMessageIds":[],"recommendations":[]}`, allowed)
	if err != nil {
		t.Fatal(err)
	}
	smartV1JSON, err := json.Marshal(smartV1)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"matchScore", "confidenceScore", "evidenceCoverageScore", "priorityScore", "priorityLevel", "dimensions"} {
		if strings.Contains(string(smartV1JSON), `"`+field+`"`) {
			t.Fatalf("v1 smart result injected %s: %s", field, smartV1JSON)
		}
	}
}

func assertJSONPathsAreNull(t *testing.T, value any, paths ...string) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var root any
	if err := json.Unmarshal(encoded, &root); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		current := root
		for _, part := range strings.Split(path, ".") {
			switch node := current.(type) {
			case map[string]any:
				var ok bool
				current, ok = node[part]
				if !ok {
					t.Fatalf("JSON path %s missing from %s", path, encoded)
				}
			case []any:
				if part != "0" || len(node) == 0 {
					t.Fatalf("JSON path %s missing from %s", path, encoded)
				}
				current = node[0]
			default:
				t.Fatalf("JSON path %s missing from %s", path, encoded)
			}
		}
		if current != nil {
			t.Fatalf("JSON path %s = %#v, want null in %s", path, current, encoded)
		}
	}
}
