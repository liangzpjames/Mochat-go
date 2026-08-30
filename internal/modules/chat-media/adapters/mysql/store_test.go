package mysql

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSQLMediaStoreListUsesSynchronizedRecordingFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewSQLMediaStore(db)
	countQuery := regexp.QuoteMeta("SELECT COUNT(*) FROM mochat_go_audio_objects WHERE corp_id = ? AND deleted_at IS NULL AND source = 'wecom_sync' AND sender_name LIKE ? AND receiver_name LIKE ? AND synced_at >= ? AND synced_at < DATE_ADD(?, INTERVAL 1 DAY)")
	mock.ExpectQuery(countQuery).
		WithArgs(int64(9), "%张三%", "%李四%", "2026-08-01 00:00:00", "2026-08-20 00:00:00").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	listQuery := regexp.QuoteMeta("SELECT " + mediaColumns + "\n\t\tFROM mochat_go_audio_objects\n\t\tWHERE corp_id = ? AND deleted_at IS NULL AND source = 'wecom_sync' AND sender_name LIKE ? AND receiver_name LIKE ? AND synced_at >= ? AND synced_at < DATE_ADD(?, INTERVAL 1 DAY)\n\t\tORDER BY synced_at DESC, id DESC\n\t\tLIMIT ? OFFSET ?")
	mock.ExpectQuery(listQuery).
		WithArgs(int64(9), "%张三%", "%李四%", "2026-08-01 00:00:00", "2026-08-20 00:00:00", 20, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "user_id", "employee_id", "corp_id", "original_name", "source", "message_id", "sender_name", "receiver_name", "relative_path", "content_type", "size_bytes", "duration_seconds", "sha256", "created_at", "synced_at"}))

	result, err := store.List(context.Background(), MediaListFilter{
		CorpID: 9, Page: 2, PerPage: 20, Sender: "张三", Receiver: "李四", SyncedFrom: "2026-08-01", SyncedTo: "2026-08-20",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Page != 2 || result.PerPage != 20 {
		t.Fatalf("result=%+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

var _ *sql.DB
