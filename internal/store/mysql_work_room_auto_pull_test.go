package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestWorkRoomAutoPullRoomWXChatIDsKeepsSelectionOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, wx_chat_id
		FROM mc_work_room
		WHERE corp_id = ? AND id IN (?,?) AND wx_chat_id <> '' AND deleted_at IS NULL`)).
		WithArgs(7, 22, 11).
		WillReturnRows(sqlmock.NewRows([]string{"id", "wx_chat_id"}).AddRow(11, "chat-11").AddRow(22, "chat-22"))

	got, err := store.WorkRoomAutoPullRoomWXChatIDs(context.Background(), 7, []int{22, 11})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "chat-22" || got[1] != "chat-11" {
		t.Fatalf("got=%#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWorkRoomAutoPullRoomWXChatIDsRejectsMissingRoom(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	mock.ExpectQuery("SELECT id, wx_chat_id").WithArgs(7, 11, 22).
		WillReturnRows(sqlmock.NewRows([]string{"id", "wx_chat_id"}).AddRow(11, "chat-11"))

	if _, err := store.WorkRoomAutoPullRoomWXChatIDs(context.Background(), 7, []int{11, 22}); err == nil {
		t.Fatal("expected missing room error")
	}
}
