package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"jiyi/mochat-go/internal/dashboard"
)

func TestContactTransferUnassignedContactsSearchesCustomerDisplayFieldsAndSupportsPartialDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := &MySQLStore{db: db}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT created_at\n\t\tFROM mc_work_unassigned")).WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"created_at"}))
	mock.ExpectQuery(`(?s)ce\.remark LIKE \?.*c\.name LIKE \?.*c\.wx_external_userid LIKE \?.*ce\.create_time >= \?`).WithArgs(7, "%待分配%", "%待分配%", "%待分配%", "2026-08-01").WillReturnRows(sqlmock.NewRows([]string{"contact_id", "employee_id", "contact_wx_id", "employee_wx_id", "contact_name", "nick_name", "corp_name", "employee_name", "create_time", "add_way"}))

	_, err = store.ContactTransferUnassignedContacts(context.Background(), dashboard.ContactTransferUnassignedFilter{
		CorpID: 7, ContactName: "待分配", AddTimeStart: "2026-08-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
