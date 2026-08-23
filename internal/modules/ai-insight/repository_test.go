package aiinsight

import (
	"strings"
	"testing"
	"time"
)

func TestConversationCandidatesMergeShardsByGlobalMessageTime(t *testing.T) {
	base := time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)
	rows := []archiveMessageRow{
		{ID: "shard-1-old", MessageTime: base.Add(-2 * time.Minute), Sequence: 1, TableIndex: 1},
		{ID: "shard-2-new", MessageTime: base, Sequence: 1, TableIndex: 2},
		{ID: "shard-1-new", MessageTime: base.Add(-time.Minute), Sequence: 2, TableIndex: 1},
	}
	sortArchiveRows(rows)
	got := []string{rows[0].ID, rows[1].ID, rows[2].ID}
	want := []string{"shard-2-new", "shard-1-new", "shard-1-old"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("global order = %#v, want %#v", got, want)
		}
	}
}

func TestSaveInsightSourceFingerprintIsStable(t *testing.T) {
	rows := []archiveMessageRow{{ID: "m1", MsgID: "msg:1", Content: "你好", Sequence: 1, TableIndex: 1, MessageTime: time.Unix(10, 0)}}
	first := fingerprintArchiveRows(rows)
	second := fingerprintArchiveRows(rows)
	if first == "" || first != second || len(first) != 64 {
		t.Fatalf("fingerprint = %q, second = %q", first, second)
	}
}

func TestInsightPageFilterIsTenantCorpAndEmployeeScoped(t *testing.T) {
	where, args := insightWhere(InsightFilter{TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, Restricted: true, AllowedEmployeeIDs: []int64{1002, 1001}})
	joined := strings.Join(where, " AND ")
	for _, fragment := range []string{"i.tenant_id=?", "i.corp_id=?", "i.analysis_type=?", "i.employee_id IN (?,?)"} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("where = %s, missing %s", joined, fragment)
		}
	}
	if len(args) != 5 || args[0] != int64(7) || args[1] != int64(8) {
		t.Fatalf("args = %#v", args)
	}
}

func TestRuleWriteValidation(t *testing.T) {
	valid := RuleWrite{Name: "采购意向", Objective: "识别明确询价", ConversationTypes: []string{"direct"}, TargetScope: "all", LookbackDays: 30, MinimumMessages: 2, Status: "enabled"}
	if err := validateRuleWrite(valid); err != nil {
		t.Fatal(err)
	}
	valid.MinimumMessages = 1
	if err := validateRuleWrite(valid); err == nil {
		t.Fatal("expected minimum message validation")
	}
}

func TestEnabledRuleVersionsQuerySelectsOnlySystemDefault(t *testing.T) {
	query, args := enabledRuleVersionsQuery(7, 8)
	if !strings.Contains(query, "r.system_key=?") || !strings.Contains(query, "v.version=r.current_version") {
		t.Fatalf("query does not select the fixed current rule: %s", query)
	}
	if len(args) != 3 || args[0] != int64(7) || args[1] != int64(8) || args[2] != DefaultSmartAnalysisRuleSystemKey {
		t.Fatalf("args = %#v", args)
	}
}
