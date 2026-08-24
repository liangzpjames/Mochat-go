package aiinsight

import (
	"reflect"
	"strings"
	"testing"
)

func TestProjectionEmployeeScoreRequiresRealIntegerBeforeCast(t *testing.T) {
	zero := 0
	where, args := insightWhere(InsightFilter{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession,
		View: "employee-score", MinScore: &zero, MaxScore: &zero,
		Restricted: true, AllowedEmployeeIDs: []int64{1002, 1001},
	})
	joined := strings.Join(where, " AND ")
	guard := "JSON_TYPE(JSON_EXTRACT(i.result_json,'$.employeeQa.score'))='INTEGER'"
	minimum := "CAST(JSON_UNQUOTE(JSON_EXTRACT(i.result_json,'$.employeeQa.score')) AS SIGNED)>=?"
	maximum := "CAST(JSON_UNQUOTE(JSON_EXTRACT(i.result_json,'$.employeeQa.score')) AS SIGNED)<=?"
	guardIndex, minimumIndex, maximumIndex := strings.Index(joined, guard), strings.Index(joined, minimum), strings.Index(joined, maximum)
	if guardIndex < 0 {
		t.Fatalf("where=%s, missing MariaDB integer JSON_TYPE guard", joined)
	}
	if minimumIndex < 0 || maximumIndex < 0 || guardIndex > minimumIndex || guardIndex > maximumIndex {
		t.Fatalf("where=%s, integer guard must precede both CAST comparisons", joined)
	}
	wantArgs := []any{int64(7), int64(8), AnalysisTypeSession, 0, 0, int64(1001), int64(1002)}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args=%#v, want stable projection/scope order %#v", args, wantArgs)
	}
}
