package store

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"jiyi/mochat-go/internal/dashboard"
)

func TestUpdateContactFieldPivotsAtomicallyValidatesEveryExistingPivotBeforeWriting(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)

	mock.ExpectBegin()
	expectContactFieldPivotEmployeeAccess(mock, 66, 7, 5)
	mock.ExpectQuery(`(?s)SELECT id.*FROM mc_contact_field.*WHERE id IN \(\?,\?\).*deleted_at IS NULL.*FOR UPDATE`).
		WithArgs(31, 32).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(31).AddRow(32))
	mock.ExpectQuery(`(?s)FROM mc_contact_field_pivot.*WHERE id IN \(\?,\?\).*deleted_at IS NULL.*FOR UPDATE`).
		WithArgs(901, 902).
		WillReturnRows(sqlmock.NewRows([]string{"id", "contact_id", "contact_field_id", "value"}).
			AddRow(901, 66, 31, "旧备注").
			AddRow(902, 77, 32, "旧城市"))
	mock.ExpectRollback()

	err = store.UpdateContactFieldPivotsAtomically(context.Background(), dashboard.ContactFieldPivotBatchWrite{
		ContactID:             66,
		CorpID:                7,
		EmployeeID:            5,
		RequireEmployeeAccess: true,
		Items: []dashboard.ContactFieldPivotWrite{
			{PivotID: 901, ContactFieldID: 31, Name: "备注", Value: "新备注"},
			{PivotID: 902, ContactFieldID: 32, Name: "城市", Value: "新城市"},
		},
	})
	if !errors.Is(err, dashboard.ErrContactFieldPivotAccess) {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateContactFieldPivotsAtomicallyRollsBackEarlierUpdateWhenLaterUpdateFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)

	mock.ExpectBegin()
	expectContactFieldPivotEmployeeAccess(mock, 66, 7, 5)
	mock.ExpectQuery(`(?s)SELECT id.*FROM mc_contact_field.*WHERE id IN \(\?,\?\).*deleted_at IS NULL.*FOR UPDATE`).
		WithArgs(31, 32).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(31).AddRow(32))
	mock.ExpectQuery(`(?s)FROM mc_contact_field_pivot.*WHERE id IN \(\?,\?\).*deleted_at IS NULL.*FOR UPDATE`).
		WithArgs(901, 902).
		WillReturnRows(sqlmock.NewRows([]string{"id", "contact_id", "contact_field_id", "value"}).
			AddRow(901, 66, 31, "旧备注").
			AddRow(902, 66, 32, "旧城市"))
	mock.ExpectExec(`(?s)UPDATE mc_contact_field_pivot.*SET value = \?, updated_at = NOW\(\).*WHERE id = \? AND deleted_at IS NULL`).
		WithArgs("新备注", 901).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE mc_contact_field_pivot.*SET value = \?, updated_at = NOW\(\).*WHERE id = \? AND deleted_at IS NULL`).
		WithArgs("新城市", 902).
		WillReturnError(errors.New("second update failed"))
	mock.ExpectRollback()

	err = store.UpdateContactFieldPivotsAtomically(context.Background(), dashboard.ContactFieldPivotBatchWrite{
		ContactID:             66,
		CorpID:                7,
		EmployeeID:            5,
		RequireEmployeeAccess: true,
		Items: []dashboard.ContactFieldPivotWrite{
			{PivotID: 901, ContactFieldID: 31, Name: "备注", Value: "新备注"},
			{PivotID: 902, ContactFieldID: 32, Name: "城市", Value: "新城市"},
		},
	})
	if err == nil || err.Error() != "second update failed" {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateContactFieldPivotsAtomicallyRejectsAnUnknownFieldBeforeWriting(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)

	mock.ExpectBegin()
	expectContactFieldPivotEmployeeAccess(mock, 66, 7, 5)
	mock.ExpectQuery(`(?s)SELECT id.*FROM mc_contact_field.*WHERE id IN \(\?,\?\).*deleted_at IS NULL.*FOR UPDATE`).
		WithArgs(31, 99).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(31))
	mock.ExpectRollback()

	err = store.UpdateContactFieldPivotsAtomically(context.Background(), dashboard.ContactFieldPivotBatchWrite{
		ContactID:             66,
		CorpID:                7,
		EmployeeID:            5,
		RequireEmployeeAccess: true,
		Items: []dashboard.ContactFieldPivotWrite{
			{PivotID: 901, ContactFieldID: 31, Name: "备注", Value: "新备注"},
			{ContactFieldID: 99, Name: "未知字段", Value: "非法值"},
		},
	})
	if !errors.Is(err, dashboard.ErrContactFieldPivotFieldNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectContactFieldPivotEmployeeAccess(mock sqlmock.Sqlmock, contactID, corpID, employeeID int) {
	mock.ExpectQuery(`(?s)SELECT contact.id.*FROM mc_work_contact AS contact.*JOIN mc_work_contact_employee AS relation.*JOIN mc_work_employee AS employee.*WHERE contact.id = \? AND contact.corp_id = \?.*relation.employee_id = \?.*FOR UPDATE`).
		WithArgs(contactID, corpID, employeeID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(contactID))
}
