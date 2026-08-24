package aiinsight

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
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

func TestConversationMessagesUsesGlobalEvidenceIDsAndCorrectSenderNamesAcrossShards(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewSQLRepository(db)
	messageTime := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)

	expectArchiveShard(mock, 1, []driver.Value{int64(42)}, sqlmock.NewRows(archiveMessageColumns).
		AddRow("7", "wx-global-7", "员工回复", 11, 1, "customer-1", 0, 1, messageTime.Add(time.Minute), "员工甲", "", "客户乙", ""), nil)
	expectArchiveShard(mock, 2, []driver.Value{int64(42)}, sqlmock.NewRows(archiveMessageColumns).
		AddRow("7", "", "客户追问", 11, 1, "customer-1", 1, 2, messageTime, "员工甲", "", "客户乙", ""), nil)
	for tableIndex := 3; tableIndex <= 10; tableIndex++ {
		expectArchiveShard(mock, tableIndex, []driver.Value{int64(42)}, nil, &mysql.MySQLError{Number: 1146, Message: "table does not exist"})
	}

	messages, err := repo.ConversationMessages(context.Background(), ConversationWindowQuery{CorpID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages=%#v", messages)
	}
	if messages[0].ID != "msgid:wx-global-7" || messages[0].LegacyID != "7" || messages[0].Direction != "outbound" || messages[0].SenderName != "员工甲" {
		t.Fatalf("outbound=%#v", messages[0])
	}
	if messages[1].ID != "shard:2:7" || messages[1].LegacyID != "7" || messages[1].Direction != "inbound" || messages[1].SenderName != "客户乙" {
		t.Fatalf("inbound=%#v", messages[1])
	}
	if messages[0].ID == messages[1].ID {
		t.Fatal("cross-shard evidence IDs collided")
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

func setProjectionFilterField(t *testing.T, filter *InsightFilter, name string, value any) {
	t.Helper()
	field := reflect.ValueOf(filter).Elem().FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("InsightFilter 缺少投影字段 %s", name)
	}
	if !field.CanSet() {
		t.Fatalf("InsightFilter.%s 不可写", name)
	}
	want := reflect.ValueOf(value)
	if !want.Type().AssignableTo(field.Type()) {
		t.Fatalf("InsightFilter.%s 类型=%s，不能赋值 %s", name, field.Type(), want.Type())
	}
	field.Set(want)
}

func TestProjectionInsightFilterDeclaresTypedDerivedFields(t *testing.T) {
	typ := reflect.TypeOf(InsightFilter{})
	for _, want := range []struct {
		name string
		typ  reflect.Type
	}{
		{name: "View", typ: reflect.TypeOf("")},
		{name: "Emotion", typ: reflect.TypeOf("")},
		{name: "MinScore", typ: reflect.TypeOf((*int)(nil))},
		{name: "MaxScore", typ: reflect.TypeOf((*int)(nil))},
	} {
		field, ok := typ.FieldByName(want.name)
		if !ok {
			t.Errorf("InsightFilter 缺少字段 %s", want.name)
			continue
		}
		if field.Type != want.typ {
			t.Errorf("InsightFilter.%s 类型=%s，want %s", want.name, field.Type, want.typ)
		}
	}
}

func TestProjectionInsightWhereUsesParameterizedMariaDBJSONAndStableScopeOrder(t *testing.T) {
	zero, hundred := 0, 100
	tests := []struct {
		name     string
		view     string
		emotion  string
		minScore *int
		maxScore *int
		keyword  string
		fragment string
		tailArgs []any
	}{
		{
			name: "emotion exact customer label", view: "emotion", emotion: "negative",
			fragment: "JSON_UNQUOTE(JSON_EXTRACT(i.result_json,'$.customer.emotion.label'))=?", tailArgs: []any{"negative", int64(1001), int64(1002)},
		},
		{
			name: "score keeps zero and one hundred", view: "employee-score", minScore: &zero, maxScore: &hundred,
			fragment: "CAST(JSON_UNQUOTE(JSON_EXTRACT(i.result_json,'$.employeeQa.score')) AS SIGNED)>=?", tailArgs: []any{0, 100, int64(1001), int64(1002)},
		},
		{
			name: "keyword searches customer keyword array with escaped like characters", view: "communication-keyword", keyword: `50%_\采购`,
			fragment: "JSON_SEARCH(JSON_EXTRACT(i.result_json,'$.customer.keywords'),'one',?,'\\\\','$[*]') IS NOT NULL", tailArgs: []any{`%50\%\_\\采购%`, int64(1001), int64(1002)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter := InsightFilter{TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, Keyword: test.keyword, Restricted: true, AllowedEmployeeIDs: []int64{1002, 1001}}
			setProjectionFilterField(t, &filter, "View", test.view)
			setProjectionFilterField(t, &filter, "Emotion", test.emotion)
			setProjectionFilterField(t, &filter, "MinScore", test.minScore)
			setProjectionFilterField(t, &filter, "MaxScore", test.maxScore)
			where, args := insightWhere(filter)
			joined := strings.Join(where, " AND ")
			for _, fragment := range []string{"i.tenant_id=?", "i.corp_id=?", "i.analysis_type=?", test.fragment, "i.employee_id IN (?,?)"} {
				if !strings.Contains(joined, fragment) {
					t.Fatalf("where=%s，missing %s", joined, fragment)
				}
			}
			if test.view == "employee-score" && !strings.Contains(joined, "CAST(JSON_UNQUOTE(JSON_EXTRACT(i.result_json,'$.employeeQa.score')) AS SIGNED)<=?") {
				t.Fatalf("where=%s，missing score max condition", joined)
			}
			wantArgs := append([]any{int64(7), int64(8), AnalysisTypeSession}, test.tailArgs...)
			if !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("args=%#v，want stable tenant/corp/type/projection/scope order %#v", args, wantArgs)
			}
		})
	}
}

func TestProjectionInsightWhereUsesExclusiveEndBoundaryForWholeSelectedDay(t *testing.T) {
	start := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	endExclusive := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	where, args := insightWhere(InsightFilter{
		TenantID: 7, CorpID: 8, AnalysisType: AnalysisTypeSession, StartAt: &start, EndAt: &endExclusive,
	})
	joined := strings.Join(where, " AND ")
	if !strings.Contains(joined, "i.source_ended_at>=?") {
		t.Fatalf("where=%s，startDate must be inclusive", joined)
	}
	if !strings.Contains(joined, "i.source_started_at<?") || strings.Contains(joined, "i.source_started_at<=?") {
		t.Fatalf("where=%s，endDate must use next-day exclusive '<' boundary", joined)
	}
	if len(args) != 5 || args[3] != start || args[4] != endExclusive {
		t.Fatalf("args=%#v", args)
	}
}

func TestProjectionInsightDetailIsSingleTenantCorpSessionEmployeeScopedQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT i\.id.*FROM mochat_go_ai_conversation_insights i.*WHERE i\.tenant_id=\? AND i\.corp_id=\? AND i\.analysis_type=\? AND i\.id=\? AND i\.employee_id IN \(\?,\?\)`).
		WithArgs(int64(11), int64(22), AnalysisTypeSession, int64(91), int64(1001), int64(1002)).
		WillReturnRows(sqlmock.NewRows(insightColumns()))

	_, err = NewSQLRepository(db).InsightDetail(context.Background(), InsightDetailFilter{
		TenantID: 11, CorpID: 22, AnalysisType: AnalysisTypeSession, ID: 91,
		Restricted: true, AllowedEmployeeIDs: []int64{1002, 1001},
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-scope detail error=%v，want sql.ErrNoRows for handler 404", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("detail must not issue an unscoped fallback query: %v", err)
	}
}
