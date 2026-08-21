package store

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"jiyi/mochat-go/internal/dashboard"
)

func TestUpsertWorkMessageArchiveWritesParticipantIdentityInSameTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id\n\t\tFROM mc_work_employee")).
		WithArgs(7, "wmKh001").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, COALESCE(owner_id, 0)\n\t\tFROM mc_work_room")).
		WithArgs(7, "wrRoom001").
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(3001, 9))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_work_message_participant_identity")).
		WithArgs(7, "msg-room-1", int64(88), "wmKh001", "wrRoom001").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mc_work_message_8")).
		WithArgs(7, "msg-room-1", int64(88), 9, 2, 3001, 0, 0, 1, 1,
			`{"content":"hello"}`, "hello", 3001, sqlmock.AnyArg(), 7, "msg-room-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := NewMySQLStore(db).UpsertWorkMessageArchive(context.Background(), 7, dashboard.WorkMessageArchiveMessage{
		Seq:         88,
		MsgID:       "msg-room-1",
		From:        "wmKh001",
		RoomID:      "wrRoom001",
		MsgType:     "text",
		ContentRaw:  `{"content":"hello"}`,
		ContentText: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Inserted || !result.Resolved {
		t.Fatalf("result=%#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpsertWorkMessageArchiveRollsBackWhenParticipantIdentityWriteFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	expectArchiveRoomResolution(mock)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_work_message_participant_identity")).
		WithArgs(7, "msg-room-1", int64(88), "wmKh001", "wrRoom001").
		WillReturnError(errors.New("identity write failed"))
	mock.ExpectRollback()

	_, err = NewMySQLStore(db).UpsertWorkMessageArchive(context.Background(), 7, archiveRoomMessage())
	if err == nil {
		t.Fatal("expected participant identity write error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpsertWorkMessageArchiveRollsBackWhenMessageWriteFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	expectArchiveRoomResolution(mock)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_work_message_participant_identity")).
		WithArgs(7, "msg-room-1", int64(88), "wmKh001", "wrRoom001").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mc_work_message_8")).
		WithArgs(7, "msg-room-1", int64(88), 9, 2, 3001, 0, 0, 1, 1,
			`{"content":"hello"}`, "hello", 3001, sqlmock.AnyArg(), 7, "msg-room-1").
		WillReturnError(errors.New("message write failed"))
	mock.ExpectRollback()

	_, err = NewMySQLStore(db).UpsertWorkMessageArchive(context.Background(), 7, archiveRoomMessage())
	if err == nil {
		t.Fatal("expected message write error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectArchiveRoomResolution(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id\n\t\tFROM mc_work_employee")).
		WithArgs(7, "wmKh001").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, COALESCE(owner_id, 0)\n\t\tFROM mc_work_room")).
		WithArgs(7, "wrRoom001").
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(3001, 9))
}

func archiveRoomMessage() dashboard.WorkMessageArchiveMessage {
	return dashboard.WorkMessageArchiveMessage{
		Seq:         88,
		MsgID:       "msg-room-1",
		From:        "wmKh001",
		RoomID:      "wrRoom001",
		MsgType:     "text",
		ContentRaw:  `{"content":"hello"}`,
		ContentText: "hello",
	}
}
