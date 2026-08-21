package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"jiyi/mochat-go/internal/dashboard"
)

func TestMySQLStoreRefuseArchivePageUsesSubjectEmployeeAndDateFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := &MySQLStore{db: db}
	where := " WHERE tenant_id=? AND corp_id=? AND subject_type=? AND employee_id=? AND refused_at >= ? AND refused_at < DATE_ADD(?, INTERVAL 1 DAY)"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM mochat_go_refuse_archive_records" + where)).
		WithArgs(23, 5, "room", int64(9), "2026-08-01 00:00:00", "2026-08-20 00:00:00").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,subject_type,subject_id,subject_name,employee_id,employee_name,authorization_status,source,refused_at,authorized_at,last_follow_up_at,follow_up_status,follow_up_note,updated_at FROM mochat_go_refuse_archive_records"+where+" ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?")).
		WithArgs(23, 5, "room", int64(9), "2026-08-01 00:00:00", "2026-08-20 00:00:00", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "subject_type", "subject_id", "subject_name", "employee_id", "employee_name", "authorization_status", "source", "refused_at", "authorized_at", "last_follow_up_at", "follow_up_status", "follow_up_note", "updated_at"}))

	page, err := store.RefuseArchivePage(context.Background(), dashboard.RefuseArchiveFilter{TenantID: 23, CorpID: 5, SubjectType: "room", EmployeeID: 9, RefusedFrom: "2026-08-01", RefusedTo: "2026-08-20", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || page.Page != 1 || page.PerPage != 20 {
		t.Fatalf("page=%+v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
