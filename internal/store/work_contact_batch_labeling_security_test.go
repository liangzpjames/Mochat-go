package store

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"jiyi/mochat-go/internal/dashboard"
)

func TestBatchLabelWorkContactsRejectsCrossCorpTagAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM mc_work_employee.*id = \? AND corp_id = \?`).
		WithArgs(5, 7).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(`(?s)FROM mc_work_contact AS contact.*relation.employee_id = \?.*relation.corp_id = contact.corp_id.*WHERE contact.corp_id = \?.*contact.id IN \(\?,\?\)`).
		WithArgs(5, 7, 31, 32).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(DISTINCT contact.id)"}).AddRow(2))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag.*corp_id = \?.*id IN \(\?,\?\)`).
		WithArgs(7, 11, 99).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectRollback()

	inserted, err := store.BatchLabelWorkContacts(context.Background(), []int{31, 32}, []int{11, 99}, 5, 7)
	if inserted != 0 || !errors.Is(err, dashboard.ErrWorkContactBatchLabelScope) {
		t.Fatalf("inserted=%d err=%v", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBatchLabelWorkContactsScopesExistingPivotToEmployee(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM mc_work_employee.*id = \? AND corp_id = \?`).
		WithArgs(5, 7).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(`(?s)FROM mc_work_contact AS contact.*relation.employee_id = \?.*WHERE contact.corp_id = \?.*contact.id IN \(\?\)`).
		WithArgs(5, 7, 31).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(DISTINCT contact.id)"}).AddRow(1))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag.*corp_id = \?.*id IN \(\?\)`).
		WithArgs(7, 11).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag_pivot.*contact_id IN \(\?\).*contact_tag_id IN \(\?\).*employee_id = \?`).
		WithArgs(31, 11, 5).
		WillReturnRows(sqlmock.NewRows([]string{"contact_id", "contact_tag_id"}))
	mock.ExpectExec(`(?s)INSERT INTO mc_work_contact_tag_pivot.*VALUES \(\?, \?, \?, NOW\(\), NOW\(\)\)`).
		WithArgs(31, 5, 11).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	inserted, err := store.BatchLabelWorkContacts(context.Background(), []int{31}, []int{11}, 5, 7)
	if err != nil || inserted != 1 {
		t.Fatalf("inserted=%d err=%v", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
