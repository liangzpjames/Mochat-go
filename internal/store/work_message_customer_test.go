package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/dashboard"
)

func TestCustomerDetailAssociationRules(t *testing.T) {
	if err := validateCustomerConversationAssociation(31, 1, 99, false); !errors.Is(err, dashboard.ErrWorkMessageConversationNotFound) {
		t.Fatalf("err=%v", err)
	}
	if err := validateCustomerConversationAssociation(31, 1, 31, false); err != nil {
		t.Fatal(err)
	}
	if err := validateCustomerConversationAssociation(31, 2, 44, false); !errors.Is(err, dashboard.ErrWorkMessageConversationNotFound) {
		t.Fatalf("err=%v", err)
	}
	if err := validateCustomerConversationAssociation(31, 2, 44, true); err != nil {
		t.Fatal(err)
	}
	if err := validateCustomerConversationAssociation(31, 3, 44, true); !errors.Is(err, dashboard.ErrWorkMessageConversationNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestCustomerDetailRoomMembershipScopesCustomerCorpAndKeepsHistoricalMembership(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS(
		SELECT 1 FROM mc_work_contact_room membership
		JOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=? AND room.deleted_at IS NULL
		JOIN mc_work_contact contact_scope ON contact_scope.id=membership.contact_id AND contact_scope.corp_id=?
		WHERE membership.contact_id=? AND membership.room_id=?
	)`)).
		WithArgs(27, 27, 31, 44).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	related, err := NewMySQLStore(db).customerRoomMembershipExists(context.Background(), 27, 31, 44)
	if err != nil {
		t.Fatal(err)
	}
	if !related {
		t.Fatal("historical same-corp membership must authorize the room")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerDetailRejectsEmployeeOutsideRestrictedScope(t *testing.T) {
	_, err := NewMySQLStore(nil).WorkMessageCustomerDetail(context.Background(), dashboard.WorkMessageCustomerDetailFilter{
		CustomerID: 31, EmployeeID: 9, ToUserType: 1, ToUserID: 31,
		RestrictEmployeeIDs: true, EmployeeIDs: []int{10},
	})
	if !errors.Is(err, dashboard.ErrWorkMessageConversationNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestCustomerDetailFromStaffIncludesAvailableProfileInJSONContract(t *testing.T) {
	staff := dashboard.WorkMessageStaffDetail{
		ConversationID: "9:1:31", EmployeeID: 9, TargetID: 31, EmployeeName: "员工", TargetType: "customer", TargetName: "归档客户",
		Stats:      dashboard.WorkMessageStaffStats{CommunicationDays: 2, MessageTotal: 3, InboundTotal: 2, OutboundTotal: 1},
		Messages:   []dashboard.WorkMessageStaffMessage{{ID: "msg:1", Direction: "inbound", SenderName: "群成员"}},
		NextBefore: "cursor", HasMore: true, Capabilities: []dashboard.WorkMessageCapability{{Key: "archive", Available: true}},
	}
	profile := dashboard.WorkMessageCustomerProfile{ID: 31, Name: "客户资料", Avatar: "https://example.test/avatar.png", ProfileStatus: "available"}
	got := customerDetailFromStaff(profile, staff)
	if got.CustomerID != 31 || got.CustomerName != "客户资料" || !reflect.DeepEqual(got.WorkMessageStaffDetail, staff) {
		t.Fatalf("detail=%#v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if gotProfile, ok := payload["profile"].(map[string]any); !ok || !reflect.DeepEqual(gotProfile, map[string]any{
		"id": float64(31), "name": "客户资料", "avatar": "https://example.test/avatar.png", "profileStatus": "available",
	}) {
		t.Fatalf("profile=%#v payload=%s", payload["profile"], encoded)
	}
}

func TestCustomerDetailFromStaffUsesCustomerArchiveNameForMissingProfile(t *testing.T) {
	got := customerDetailFromStaff(
		dashboard.WorkMessageCustomerProfile{ID: 31, Avatar: "", ProfileStatus: "missing"},
		dashboard.WorkMessageStaffDetail{TargetType: "customer", TargetName: "归档客户"},
	)
	if got.CustomerName != "归档客户" || got.Profile.ID != 31 || got.Profile.Name != "归档客户" || got.Profile.Avatar != "" || got.Profile.ProfileStatus != "missing" {
		t.Fatalf("detail=%#v", got)
	}
}

func TestCustomerDetailFromStaffUsesStableNameWhenArchiveDoesNotRepresentCustomer(t *testing.T) {
	got := customerDetailFromStaff(
		dashboard.WorkMessageCustomerProfile{ID: 31, ProfileStatus: "missing"},
		dashboard.WorkMessageStaffDetail{TargetType: "room", TargetName: "群聊"},
	)
	if got.CustomerName != "客户 #31" {
		t.Fatalf("customerName=%q", got.CustomerName)
	}
}

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
	if !strings.Contains(sourceSQL, "AS archive_name") {
		t.Fatalf("customer directory keyword filtering requires archive_name in the derived projection: %s", sourceSQL)
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

func TestCustomerConversationGroupRoomsScopeContactCorpAndBindEachSource(t *testing.T) {
	filter := dashboard.WorkMessageCustomerConversationFilter{
		CorpID: 27, CustomerID: 31, Mode: dashboard.WorkMessageCustomerConversationModeGroup,
	}
	sqlText, args, err := customerConversationBaseSQL(filter, "SELECT * FROM archive WHERE source=?", []any{"archive-source"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(sqlText, "JOIN mc_work_contact contact_scope ON contact_scope.id=membership.contact_id AND contact_scope.corp_id=?"); got != 2 {
		t.Fatalf("group source must scope contact corp twice, got %d in %s", got, sqlText)
	}
	if got := strings.Count(sqlText, "SELECT * FROM archive WHERE source=?"); got != 3 {
		t.Fatalf("group source must bind latest, anti-newer, and count sources, got %d in %s", got, sqlText)
	}
	want := []any{"archive-source", 27, 27, 31, "archive-source", "archive-source", 27, 27, 31, 27, 27, 27, 27}
	if len(args) != len(want) {
		t.Fatalf("args=%#v want=%#v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d]=%#v want=%#v; args=%#v", i, args[i], want[i], args)
		}
	}
}

func TestCustomerConversationWorkspaceProfileRestrictedGroupRequiresAllowedEmployeeMember(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("FROM mc_work_contact WHERE id=? AND corp_id=? LIMIT 1")).
		WithArgs(31, 27).
		WillReturnRows(sqlmock.NewRows([]string{"name", "avatar", "profile_status"}).AddRow("客户", "", "available"))
	mock.ExpectQuery(regexp.QuoteMeta("FROM mc_work_contact_employee relation")).
		WithArgs(27, 31, 9).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(regexp.QuoteMeta("FROM mc_work_contact_room membership\n\t\tJOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=? AND room.deleted_at IS NULL\n\t\tJOIN mc_work_contact contact_scope ON contact_scope.id=membership.contact_id AND contact_scope.corp_id=?\n\t\tWHERE membership.contact_id=? AND membership.employee_id IN (?)")).
		WithArgs(27, 27, 31, 9).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	_, visible, err := NewMySQLStore(db).customerWorkspaceProfile(context.Background(), 27, 31, true, []int{9})
	if err != nil {
		t.Fatal(err)
	}
	if visible {
		t.Fatal("a group member attached only to another employee must not authorize this scope")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerConversationWorkspaceProfileMissingRestrictedCustomerRequiresAuthorizedRelationship(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("FROM mc_work_contact WHERE id=? AND corp_id=? LIMIT 1")).
		WithArgs(103, 27).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("FROM mc_work_contact_employee relation")).
		WithArgs(27, 103, 9).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(regexp.QuoteMeta("FROM mc_work_contact_room membership\n\t\tJOIN mc_work_room room ON room.id=membership.room_id AND room.corp_id=? AND room.deleted_at IS NULL\n\t\tJOIN mc_work_contact contact_scope ON contact_scope.id=membership.contact_id AND contact_scope.corp_id=?\n\t\tWHERE membership.contact_id=? AND membership.employee_id IN (?)")).
		WithArgs(27, 27, 103, 9).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	_, visible, err := NewMySQLStore(db).customerWorkspaceProfile(context.Background(), 27, 103, true, []int{9})
	if err != nil {
		t.Fatal(err)
	}
	if visible {
		t.Fatal("a missing profile without an allowed relationship must be hidden")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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

func TestCustomerConversationBaseSQLUsesArchiveAvatarForRooms(t *testing.T) {
	sqlText, _, err := customerConversationBaseSQLWithSource(
		dashboard.WorkMessageCustomerConversationFilter{CorpID: 27, CustomerID: 31, Mode: dashboard.WorkMessageCustomerConversationModeDirect},
		"SELECT 1 AS work_employee_id, 1 AS to_user_id",
		[]any{},
		"archive_messages",
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sqlText, "room.avatar") {
		t.Fatalf("customer conversation SQL must not depend on missing mc_work_room.avatar column: %s", sqlText)
	}
	if !strings.Contains(sqlText, "NULLIF(latest.target_avatar,'')") {
		t.Fatalf("room avatar must fall back to archived target_avatar: %s", sqlText)
	}
	if strings.Contains(sqlText, "latest.current_member") {
		t.Fatalf("direct customer conversation SQL must not depend on group-only current_member: %s", sqlText)
	}
	if strings.Contains(sqlText, "'none'") || !strings.Contains(sqlText, "'unknown'") {
		t.Fatalf("direct relationship status must stay inside the Dashboard contract: %s", sqlText)
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

func TestCustomerConversationDecorationReportsCapabilitiesForEmptyPage(t *testing.T) {
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
	capabilities, err := NewMySQLStore(db).decorateCustomerConversationFlags(context.Background(), dashboard.WorkMessageCustomerConversationFilter{}, []dashboard.WorkMessageCustomerConversation{})
	if err != nil {
		t.Fatal(err)
	}
	availability := map[string]bool{}
	for _, capability := range capabilities {
		availability[capability.Key] = capability.Available
	}
	for _, key := range []string{"focus", "riskRecords", "timeoutRecords"} {
		if availability[key] {
			t.Fatalf("empty page must report missing %s table: %#v", key, capabilities)
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
