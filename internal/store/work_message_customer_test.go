package store

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/dashboard"
)

func TestCustomerDirectoryModePredicateIsMutuallyExclusive(t *testing.T) {
	cases := map[dashboard.WorkMessageCustomerMode]string{
		dashboard.WorkMessageCustomerModeFocused: "focused_conversation_count > 0",
		dashboard.WorkMessageCustomerModeActive:  "active_relation_count > 0",
		dashboard.WorkMessageCustomerModeLost:    "active_relation_count = 0 AND lost_relation_count > 0",
	}
	for mode, fragment := range cases {
		where, _ := customerDirectoryOuterWhere(dashboard.WorkMessageCustomerDirectoryFilter{Mode: mode})
		if !strings.Contains(where, fragment) {
			t.Fatalf("mode=%s where=%s", mode, where)
		}
	}
}

func TestCustomerDirectoryOuterWhereEscapesKeywordAndDoesNotBroadenMode(t *testing.T) {
	where, args := customerDirectoryOuterWhere(dashboard.WorkMessageCustomerDirectoryFilter{
		Mode: dashboard.WorkMessageCustomerModeLost, Keyword: "甲%",
	})
	for _, fragment := range []string{"active_relation_count = 0", "lost_relation_count > 0", "name LIKE ? ESCAPE", "archive_name LIKE ? ESCAPE", "external_userid LIKE ? ESCAPE"} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("where=%q missing=%q", where, fragment)
		}
	}
	if len(args) != 3 || args[0] != `%甲\%%` || args[1] != `%甲\%%` || args[2] != `%甲\%%` {
		t.Fatalf("args=%#v", args)
	}
}

func TestEmptyCustomerDirectoryPreservesFixedPageContract(t *testing.T) {
	page := emptyCustomerDirectory(dashboard.WorkMessageCustomerDirectoryFilter{Page: 0})
	if page.Page != 1 || page.PageSize != 50 || page.Customers == nil || page.Limitations == nil || page.Capabilities == nil {
		t.Fatalf("page=%#v", page)
	}
}

func TestCustomerDirectorySourceScopesGroupMembershipToActiveCorpRoom(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(corp.chat_status),0)`)).
		WithArgs(27, 11, 11).
		WillReturnRows(sqlmock.NewRows([]string{"chat_status"}).AddRow(1))
	expectCustomerDirectoryRegistryState(mock, false)
	expectCustomerDirectoryFocusTable(mock, true)

	sourceSQL, sourceArgs, available, err := NewMySQLStore(db).customerDirectorySource(context.Background(), dashboard.WorkMessageCustomerDirectoryFilter{TenantID: 11, CorpID: 27, UserID: 77})
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatalf("source unavailable")
	}
	if !strings.Contains(sourceSQL, "JOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=? AND room.deleted_at IS NULL") {
		t.Fatalf("group membership must be constrained to the active room in the requested corp: %s", sourceSQL)
	}
	if len(sourceArgs) != 26 || sourceArgs[10] != 27 {
		t.Fatalf("room corp placeholder must follow first archive args; args=%#v", sourceArgs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerDirectorySourceUsesRealArchiveWithoutSimulationBatchTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(corp.chat_status),0)`)).
		WithArgs(27, 11, 11).
		WillReturnRows(sqlmock.NewRows([]string{"chat_status"}).AddRow(1))
	expectCustomerDirectoryRegistryState(mock, false)
	expectCustomerDirectoryFocusTable(mock, true)

	sourceSQL, _, available, err := NewMySQLStore(db).customerDirectorySource(context.Background(), dashboard.WorkMessageCustomerDirectoryFilter{TenantID: 11, CorpID: 27, UserID: 77})
	if err != nil || !available {
		t.Fatalf("real archive must remain available without simulation tables: available=%t err=%v", available, err)
	}
	if strings.Contains(sourceSQL, "mochat_go_archive_simulation_batches") {
		t.Fatalf("real archive SQL unexpectedly references missing simulation table: %s", sourceSQL)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerDirectoryReportsFocusTableUnavailableWithoutQueryingIt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(corp.chat_status),0)`)).
		WithArgs(27, 11, 11).
		WillReturnRows(sqlmock.NewRows([]string{"chat_status"}).AddRow(1))
	expectCustomerDirectoryRegistryState(mock, false)
	expectCustomerDirectoryFocusTable(mock, false)

	page, err := NewMySQLStore(db).WorkMessageCustomerDirectory(context.Background(), dashboard.WorkMessageCustomerDirectoryFilter{TenantID: 11, CorpID: 27, UserID: 77})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Capabilities) == 0 || len(page.Limitations) != 1 || page.Limitations[0].Key != "focusUnavailable" {
		t.Fatalf("missing focus table must return an explicit non-zero capability/limitation response: %#v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectCustomerDirectoryRegistryState(mock sqlmock.Sqlmock, legacySimulation bool) {
	rows := sqlmock.NewRows([]string{"table_name"})
	legacyCount := 0
	if legacySimulation {
		rows.AddRow("mochat_go_archive_simulation_batches").AddRow("mochat_go_archive_simulation_messages")
		legacyCount = 2
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT table_name
		FROM information_schema.tables`)).WillReturnRows(rows)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM information_schema.tables`)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(legacyCount))
}

func expectCustomerDirectoryFocusTable(mock sqlmock.Sqlmock, exists bool) {
	count := 0
	if exists {
		count = 1
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`)).
		WithArgs("mochat_go_work_message_focus").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
}

func TestCustomerConversationBaseUsesStableConversationID(t *testing.T) {
	sqlText, _, err := customerConversationBaseSQL(dashboard.WorkMessageCustomerConversationFilter{
		CustomerID: 31,
		Mode:       dashboard.WorkMessageCustomerConversationModeDirect,
	}, "SELECT * FROM archive", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"wm.to_user_type = 1",
		"wm.to_user_id = ?",
		"CONCAT(wm.work_employee_id, ':1:', wm.to_user_id)",
		"GROUP BY wm.work_employee_id, wm.to_user_id",
	} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("missing %q in %s", fragment, sqlText)
		}
	}
}

func TestCustomerConversationBaseKeepsHistoricalGroupMembership(t *testing.T) {
	sqlText, _, err := customerConversationBaseSQL(dashboard.WorkMessageCustomerConversationFilter{
		CorpID:     27,
		CustomerID: 31,
		Mode:       dashboard.WorkMessageCustomerConversationModeGroup,
	}, "SELECT * FROM archive", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"SELECT DISTINCT membership.room_id",
		"membership.contact_id = ?",
		"JOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=?",
		"wm.to_user_type = 2",
		"GROUP BY wm.work_employee_id, wm.to_user_id",
		"'left'",
	} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("missing %q in %s", fragment, sqlText)
		}
	}
	if strings.Contains(sqlText, "membership.deleted_at IS NULL") {
		t.Fatalf("historical membership must remain eligible: %s", sqlText)
	}
}

func TestCustomerConversationCountAndPageWrapTheSameBaseSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	baseSQL := "SELECT conversation_id, last_at FROM archive_conversations WHERE customer_id=?"
	baseArgs := []any{31}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM (" + baseSQL + ") customer_conversations")).
		WithArgs(31).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	if total, err := countCustomerConversations(context.Background(), db, baseSQL, baseArgs); err != nil || total != 1 {
		t.Fatalf("total=%d err=%v", total, err)
	}
	store := NewMySQLStore(db)
	mock.ExpectQuery(regexp.QuoteMeta("FROM ("+baseSQL+") customer_conversations")).
		WithArgs(31, 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"conversation_id", "work_employee_id", "employee_name", "employee_avatar", "to_user_type", "to_user_id", "target_name", "target_avatar", "content_text", "msg_type", "direction", "last_at", "message_total", "archive_source", "archive_source_id", "relation_status", "membership_status",
		}).AddRow("9:1:31", 9, "员工", "", 1, 31, "客户", "", "你好", 1, "outbound", nil, 2, "external", "msg:1", "active", ""))
	items, err := store.customerConversationPage(context.Background(), baseSQL, baseArgs, 1, 20)
	if err != nil || len(items) != 1 || items[0].ConversationID != "9:1:31" {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerConversationDecorationReportsUnavailableTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"mochat_go_work_message_focus", "mochat_go_risk_records", "mochat_go_timeout_records"} {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`)).
			WithArgs(table).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}
	list := []dashboard.WorkMessageCustomerConversation{{WorkMessageGlobalConversation: dashboard.WorkMessageGlobalConversation{ConversationID: "9:1:31"}}}
	capabilities, err := NewMySQLStore(db).decorateCustomerConversationFlags(context.Background(), dashboard.WorkMessageCustomerConversationFilter{TenantID: 11, CorpID: 27, UserID: 77}, list)
	if err != nil {
		t.Fatal(err)
	}
	availability := map[string]bool{}
	for _, capability := range capabilities {
		availability[capability.Key] = capability.Available
	}
	for _, key := range []string{"focus", "riskRecords", "timeoutRecords"} {
		if availability[key] {
			t.Fatalf("missing %s table must be explicitly unavailable: %#v", key, capabilities)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerConversationDecorationBatchesCurrentPageFlags(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(false)
	for _, table := range []string{"mochat_go_work_message_focus", "mochat_go_risk_records", "mochat_go_timeout_records"} {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`)).
			WithArgs(table).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	}
	mock.ExpectQuery("FROM mochat_go_work_message_focus").
		WithArgs(11, 27, 77, "9:1:31", "10:2:501").
		WillReturnRows(sqlmock.NewRows([]string{"conversation_id"}).AddRow("9:1:31"))
	mock.ExpectQuery("FROM mochat_go_risk_records").
		WithArgs(11, 27, "9:1:31", "10:2:501").
		WillReturnRows(sqlmock.NewRows([]string{"conversation_id", "count"}).AddRow("10:2:501", 2))
	mock.ExpectQuery("FROM mochat_go_timeout_records").
		WithArgs(11, 27, "9:1:31", "10:2:501").
		WillReturnRows(sqlmock.NewRows([]string{"conversation_id", "count"}).AddRow("9:1:31", 3))
	list := []dashboard.WorkMessageCustomerConversation{
		{WorkMessageGlobalConversation: dashboard.WorkMessageGlobalConversation{ConversationID: "9:1:31"}},
		{WorkMessageGlobalConversation: dashboard.WorkMessageGlobalConversation{ConversationID: "10:2:501"}},
	}
	_, err = NewMySQLStore(db).decorateCustomerConversationFlags(context.Background(), dashboard.WorkMessageCustomerConversationFilter{TenantID: 11, CorpID: 27, UserID: 77}, list)
	if err != nil {
		t.Fatal(err)
	}
	if !list[0].Focused || list[0].TimeoutCount != 3 || list[1].RiskCount != 2 {
		t.Fatalf("batched decoration=%#v", list)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
