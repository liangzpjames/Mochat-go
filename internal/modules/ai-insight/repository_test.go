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
	settingsports "jiyi/mochat-go/internal/modules/ai-settings/ports"
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

func TestInsightWhereCombinesScopedNameFiltersAndEscapesLikeMetacharacters(t *testing.T) {
	where, args := insightWhere(InsightFilter{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession,
		EmployeeID: 1001, Keyword: `50%_\摘要`, CustomerName: `客%_\户`,
		Restricted: true, AllowedEmployeeIDs: []int64{1002, 1001},
	})
	joined := strings.Join(where, " AND ")
	for _, fragment := range []string{
		"i.employee_id=?",
		"(i.summary LIKE ? ESCAPE '\\\\' OR i.target_name LIKE ? ESCAPE '\\\\' OR i.employee_name LIKE ? ESCAPE '\\\\')",
		"(i.target_type='1' AND i.target_name LIKE ? ESCAPE '\\\\')",
		"i.employee_id IN (?,?)",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("where = %s, missing %s", joined, fragment)
		}
	}
	wantLike := `%50\%\_\\摘要%`
	wantCustomerLike := `%客\%\_\\户%`
	if len(args) != 10 || args[4] != wantLike || args[5] != wantLike || args[6] != wantLike || args[7] != wantCustomerLike {
		t.Fatalf("args = %#v", args)
	}
}

func TestInsightWhereCustomerNameMatchesDirectConversationsOnly(t *testing.T) {
	where, args := insightWhere(InsightFilter{TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSmart, CustomerName: "客户甲"})
	joined := strings.Join(where, " AND ")
	if !strings.Contains(joined, "i.target_type='1'") || !strings.Contains(joined, "i.target_name LIKE ? ESCAPE '\\\\'") {
		t.Fatalf("where = %s", joined)
	}
	if len(args) != 4 || args[3] != "%客户甲%" {
		t.Fatalf("args = %#v", args)
	}
}

func TestInsightPageCountsBeforeSelectingWithIdenticalWhereAndArgs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	filter := InsightFilter{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, Page: 2, PageSize: 2,
		EmployeeID: 1001, Keyword: "摘要", CustomerName: "客户", Restricted: true, AllowedEmployeeIDs: []int64{1001},
	}
	where, args := insightWhere(filter)
	whereSQL := strings.Join(where, " AND ")
	driverArgs := make([]driver.Value, len(args))
	for i := range args {
		driverArgs[i] = args[i]
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM mochat_go_ai_conversation_insights i WHERE " + whereSQL)).
		WithArgs(driverArgs...).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	listArgs := append(append([]driver.Value(nil), driverArgs...), 2, 2)
	mock.ExpectQuery("SELECT i\\.id.*WHERE " + regexp.QuoteMeta(whereSQL) + ".*ORDER BY i\\.generated_at DESC, i\\.id DESC LIMIT \\? OFFSET \\?").
		WithArgs(listArgs...).WillReturnRows(sqlmock.NewRows(insightColumns()))

	page, err := NewSQLRepository(db).InsightPage(context.Background(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 2 || page.PageSize != 2 || page.Total != 5 || len(page.Items) != 0 {
		t.Fatalf("page = %#v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEmployeeOptionsQueryIsTenantCorpAnalysisAndEmployeeScoped(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query := EmployeeOptionFilter{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSmart, EmployeeKeyword: `王%_\`, Limit: 2,
		Restricted: true, AllowedEmployeeIDs: []int64{1002, 1001},
	}
	mock.ExpectQuery(`SELECT i\.employee_id.*FROM mochat_go_ai_conversation_insights i.*LEFT JOIN mc_work_employee e.*i\.tenant_id=\?.*i\.corp_id=\?.*i\.analysis_type=\?.*i\.employee_id IN \(\?,\?\).*LIKE \? ESCAPE.*GROUP BY i\.employee_id.*ORDER BY employee_name ASC, i\.employee_id ASC.*LIMIT \?`).
		WithArgs(int64(7), int64(8), AnalysisTypeSmart, int64(1001), int64(1002), `%王\%\_\\%`, 2).
		WillReturnRows(sqlmock.NewRows([]string{"employee_id", "employee_name", "employee_avatar"}).
			AddRow(1001, "王甲", "a1").AddRow(1002, "王乙", "a2"))

	options, err := NewSQLRepository(db).EmployeeOptions(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 || options[0].ID != 1001 || options[0].Name != "王甲" || options[1].ID != 1002 {
		t.Fatalf("options = %#v", options)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEmployeeOptionsRestrictedEmptyScopeReturnsEmptyWithoutQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	options, err := NewSQLRepository(db).EmployeeOptions(context.Background(), EmployeeOptionFilter{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, Restricted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 0 {
		t.Fatalf("options = %#v", options)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func insightColumns() []string {
	return []string{
		"id", "tenant_id", "corp_id", "analysis_type", "rule_id", "rule_version_id", "rule_name", "rule_version",
		"conversation_key", "employee_id", "employee_name", "employee_avatar", "target_type", "target_id", "target_name", "target_avatar",
		"source_started_at", "source_ended_at", "source_message_count", "source_fingerprint", "status", "summary", "result_json",
		"error_summary", "provider", "model", "prompt_version", "generated_at", "created_at",
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

func TestCurrentEnabledRuleVersionQuerySelectsRequestedSystemKeyAndPromptSnapshot(t *testing.T) {
	query, args := currentEnabledRuleVersionQuery(7, 8, settingsports.SessionAnalysisSystemKey)
	for _, fragment := range []string{"r.system_key=?", "v.version=r.current_version", "r.name", "v.customer_analysis_prompt", "v.employee_qa_prompt"} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q: %s", fragment, query)
		}
	}
	if len(args) != 3 || args[2] != settingsports.SessionAnalysisSystemKey {
		t.Fatalf("args = %#v", args)
	}
}
