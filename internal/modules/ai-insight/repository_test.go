package aiinsight

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

var archiveMessageColumns = []string{
	"id", "msgid", "content_text", "work_employee_id", "to_user_type", "target_id", "sender_type", "seq", "msg_data_time",
	"employee_name", "employee_avatar", "target_name", "target_avatar",
}

func expectArchiveShard(mock sqlmock.Sqlmock, tableIndex int, args []driver.Value, rows *sqlmock.Rows, err error) {
	expectation := mock.ExpectQuery(regexp.QuoteMeta("FROM mc_work_message_" + strconv.Itoa(tableIndex) + " wm"))
	if len(args) > 0 {
		expectation.WithArgs(args...)
	}
	if err != nil {
		expectation.WillReturnError(err)
		return
	}
	expectation.WillReturnRows(rows)
}

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

func TestConversationCandidateWindowCanReadBackItsSourceMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewSQLRepository(db)
	oldest := time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)
	newest := oldest.Add(5 * time.Minute)
	queryStart, queryEnd := oldest.Add(-time.Hour), newest.Add(time.Hour)
	conversationKey := "7:1:99"

	candidateRows := sqlmock.NewRows(archiveMessageColumns).
		AddRow("new", "msg-new", "新消息", 7, 1, "99", 1, 2, newest, "员工", "", "客户", "").
		AddRow("old", "msg-old", "旧消息", 7, 1, "99", 0, 1, oldest, "员工", "", "客户", "")
	expectArchiveShard(mock, 1, []driver.Value{int64(42), queryStart, queryEnd}, candidateRows, nil)
	for tableIndex := 2; tableIndex <= 10; tableIndex++ {
		expectArchiveShard(mock, tableIndex, []driver.Value{int64(42), queryStart, queryEnd}, nil, &mysql.MySQLError{Number: 1146, Message: "table does not exist"})
	}

	candidates, err := repo.ConversationCandidates(context.Background(), CandidateQuery{CorpID: 42, StartAt: queryStart, EndAt: queryEnd})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v", candidates)
	}
	candidate := candidates[0]
	if candidate.SourceStartedAt.After(candidate.SourceEndedAt) {
		t.Fatalf("candidate window is reversed: started=%s ended=%s", candidate.SourceStartedAt, candidate.SourceEndedAt)
	}
	if !candidate.SourceStartedAt.Equal(oldest) || !candidate.SourceEndedAt.Equal(newest) {
		t.Fatalf("candidate window = %s..%s, want %s..%s", candidate.SourceStartedAt, candidate.SourceEndedAt, oldest, newest)
	}

	messageRows := sqlmock.NewRows(archiveMessageColumns).
		AddRow("new", "msg-new", "新消息", 7, 1, "99", 1, 2, newest, "员工", "", "客户", "").
		AddRow("old", "msg-old", "旧消息", 7, 1, "99", 0, 1, oldest, "员工", "", "客户", "")
	expectArchiveShard(mock, 1, []driver.Value{int64(42), oldest, newest, conversationKey}, messageRows, nil)
	for tableIndex := 2; tableIndex <= 10; tableIndex++ {
		expectArchiveShard(mock, tableIndex, []driver.Value{int64(42), oldest, newest, conversationKey}, nil, &mysql.MySQLError{Number: 1146, Message: "table does not exist"})
	}
	messages, err := repo.ConversationMessages(context.Background(), ConversationWindowQuery{
		CorpID: 42, ConversationKey: conversationKey, StartAt: candidate.SourceStartedAt, EndAt: candidate.SourceEndedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].ID != "new" || messages[1].ID != "old" {
		t.Fatalf("messages = %#v", messages)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveMessagesPropagatesNonMissingTableError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewSQLRepository(db)
	wantErr := errors.New("connection reset")
	expectArchiveShard(mock, 1, []driver.Value{int64(42)}, nil, wantErr)

	_, err = repo.archiveMessages(context.Background(), 42, time.Time{}, time.Time{}, nil, false, "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveMessagesIgnoresOnlyMissingTableError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewSQLRepository(db)
	expectArchiveShard(mock, 1, []driver.Value{int64(42)}, nil, &mysql.MySQLError{Number: 1146, Message: "table does not exist"})
	expectArchiveShard(mock, 2, []driver.Value{int64(42)}, nil, &mysql.MySQLError{Number: 1142, Message: "permission denied"})

	_, err = repo.archiveMessages(context.Background(), 42, time.Time{}, time.Time{}, nil, false, "")
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1142 {
		t.Fatalf("error = %v, want MySQL 1142", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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

func TestSaveInsightUsesTwentyFiveBusinessPlaceholders(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	generatedAt := time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC)
	insight := ConversationInsight{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, RuleID: 9, RuleVersionID: 10,
		ConversationKey: "11:1:customer", EmployeeID: 11, EmployeeName: "员工", EmployeeAvatar: "employee-avatar",
		TargetType: "1", TargetID: "customer", TargetName: "客户", TargetAvatar: "customer-avatar",
		SourceStartedAt: generatedAt.Add(-time.Minute), SourceEndedAt: generatedAt,
		SourceMessageCount: 2, SourceFingerprint: "source-fingerprint", Status: AnalysisStatusSucceeded,
		Summary: "摘要", ResultJSON: []byte(`{"quality":"high"}`), Provider: "provider", Model: "model", PromptVersion: "v1", GeneratedAt: &generatedAt,
	}

	const saveInsightSQL = "INSERT INTO mochat_go_ai_conversation_insights " +
		"(tenant_id,corp_id,analysis_type,rule_id,rule_version_id,conversation_key,employee_id,employee_name,employee_avatar,target_type,target_id,target_name,target_avatar,source_started_at,source_ended_at,source_message_count,source_fingerprint,status,summary,result_json,error_summary,provider,model,prompt_version,generated_at,created_at,updated_at) " +
		"VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NOW(),NOW()) " +
		"ON DUPLICATE KEY UPDATE status=VALUES(status),summary=VALUES(summary),result_json=VALUES(result_json),error_summary=VALUES(error_summary),provider=VALUES(provider),model=VALUES(model),prompt_version=VALUES(prompt_version),generated_at=VALUES(generated_at),updated_at=NOW()"
	mock.ExpectExec(regexp.QuoteMeta(saveInsightSQL)).
		WithArgs(
			insight.TenantID, insight.CorpID, insight.AnalysisType, insight.RuleID, insight.RuleVersionID,
			insight.ConversationKey, insight.EmployeeID, insight.EmployeeName, insight.EmployeeAvatar,
			insight.TargetType, insight.TargetID, insight.TargetName, insight.TargetAvatar,
			insight.SourceStartedAt, insight.SourceEndedAt, insight.SourceMessageCount, insight.SourceFingerprint,
			insight.Status, insight.Summary, string(insight.ResultJSON), insight.ErrorSummary, insight.Provider,
			insight.Model, insight.PromptVersion, generatedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := NewSQLRepository(db).SaveInsight(context.Background(), insight); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
