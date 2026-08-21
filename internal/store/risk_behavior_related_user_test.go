package store

import "testing"

func TestParseRiskRelatedUserArray(t *testing.T) {
	result := parseRiskRelatedUser([]byte(`[{"userId":1006,"userName":"赵磊","role":"employee"},{"userId":2010,"userName":"罗敏","role":"customer"}]`))
	if result["employeeName"] != "赵磊" || result["customerName"] != "罗敏" {
		t.Fatalf("unexpected related user mapping: %#v", result)
	}
}
