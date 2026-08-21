package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestWorkMessageRoomDirectorySQLRejectsZeroEffectiveRoomID(t *testing.T) {
	sql := normalizeSQL(workMessageRoomDirectoryBaseSQL())
	for _, required := range []string{
		"wm.to_user_type=2",
		"wm.to_user_id>0",
		"mc_work_room room",
		"group by room.id",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("room directory SQL missing %q: %s", required, sql)
		}
	}
}

func TestWorkMessageRoomDirectoryKeywordIsEscaped(t *testing.T) {
	where, args := workMessageRoomDirectoryWhere(dashboard.WorkMessageRoomDirectoryFilter{Keyword: `100%_客户`})
	if !strings.Contains(where, "room.name LIKE ?") || len(args) != 3 || args[0] != `%100\%\_客户%` || args[1] != args[0] || args[2] != args[0] {
		t.Fatalf("where=%q args=%#v", where, args)
	}
}

func TestWorkMessageRoomMembersModeScopesRealMemberTypes(t *testing.T) {
	for mode, want := range map[dashboard.WorkMessageRoomMemberMode]string{
		dashboard.WorkMessageRoomMemberModeEmployee: "membership.type = 1",
		dashboard.WorkMessageRoomMemberModeCustomer: "membership.type = 2",
		dashboard.WorkMessageRoomMemberModeLeft:     "membership.status = 2",
	} {
		where, _ := workMessageRoomMembersWhere(dashboard.WorkMessageRoomMembersFilter{RoomID: 3001, Mode: mode})
		if !strings.Contains(where, want) {
			t.Fatalf("mode=%s where=%q missing %q", mode, where, want)
		}
	}
}

func TestWorkMessageRoomFilterOptionsUsesMoChatDirectoryTables(t *testing.T) {
	queries := workMessageRoomFilterOptionSQL()
	for _, query := range queries {
		sql := normalizeSQL(query)
		if !strings.Contains(sql, "corp_id = ?") || !strings.Contains(sql, "deleted_at is null") {
			t.Fatalf("option query is not corp/deleted scoped: %s", sql)
		}
	}
}

func TestWorkMessageRoomMessagesCursorUsesStableArchiveOrder(t *testing.T) {
	cursor := dashboard.EncodeWorkMessageStaffCursor(dashboard.WorkMessageStaffCursor{SentAt: "2026-08-20 10:00:00", TableIndex: 3, Seq: 88, ID: 9})
	where, args, err := workMessageRoomMessagesWhere(dashboard.WorkMessageRoomMessagesFilter{Before: cursor}, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"message.sent_at < ?", "message.seq < ?", "message.table_index < ?", "message.row_id < ?"} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("cursor where=%q missing %q", where, fragment)
		}
	}
	if len(args) != 7 {
		t.Fatalf("cursor args=%#v", args)
	}
}

func TestWorkMessageRoomMessagesJoinParticipantIdentity(t *testing.T) {
	sql := normalizeSQL(workMessageRoomMessagesBaseSQL("SELECT * FROM room_source"))
	for _, required := range []string{
		"mochat_go_work_message_participant_identity identity_row",
		"participant_employee.wx_user_id=identity_row.sender_wx_id",
		"participant_contact.wx_external_userid=identity_row.sender_wx_id",
		"else 'member'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("message SQL missing %q: %s", required, sql)
		}
	}
}

func normalizeSQL(query string) string {
	return strings.ToLower(strings.Join(strings.Fields(query), " "))
}
