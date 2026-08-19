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
