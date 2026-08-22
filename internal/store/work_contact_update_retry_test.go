package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"jiyi/mochat-go/internal/dashboard"
)

func TestUpdateWorkContactProfileRetryDoesNotDuplicateRemarkTrack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	remark := "已保存备注"

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT COALESCE\(employee\.wx_user_id.*COALESCE\(ce\.remark.*COALESCE\(ce\.description.*COALESCE\(contact\.business_no`).
		WithArgs(5, 31, 7).
		WillReturnRows(sqlmock.NewRows([]string{"wx_user_id", "wx_external_userid", "remark", "description", "business_no"}).
			AddRow("employee-5", "external-31", remark, "", ""))
	mock.ExpectCommit()

	result, found, err := store.UpdateWorkContactProfile(context.Background(), dashboard.WorkContactUpdateValues{
		CorpID: 7, ContactID: 31, EmployeeID: 5, Remark: &remark,
	})
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if result.WXUserID != "employee-5" || result.WXExternalUserID != "external-31" {
		t.Fatalf("result=%#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateWorkContactProfileRetryStillReturnsExistingTagForWeComSync(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT COALESCE\(employee\.wx_user_id.*FROM mc_work_contact_employee`).
		WithArgs(5, 31, 7).
		WillReturnRows(sqlmock.NewRows([]string{"wx_user_id", "wx_external_userid", "remark", "description", "business_no"}).
			AddRow("employee-5", "external-31", "", "", ""))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag.*WHERE corp_id = \?.*id IN \(\?\)`).
		WithArgs(7, 11).
		WillReturnRows(sqlmock.NewRows([]string{"id", "wx_contact_tag_id", "name"}).
			AddRow(11, "wx-tag-11", "已保存标签"))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag_pivot AS pivot.*pivot.contact_id = \?.*pivot.employee_id = \?`).
		WithArgs(7, 31, 5).
		WillReturnRows(sqlmock.NewRows([]string{"contact_tag_id"}).AddRow(11))
	mock.ExpectCommit()

	result, found, err := store.UpdateWorkContactProfile(context.Background(), dashboard.WorkContactUpdateValues{
		CorpID: 7, ContactID: 31, EmployeeID: 5, HasTag: true, TagIDs: []int{11},
	})
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if len(result.AddedWXTagIDs) != 1 || result.AddedWXTagIDs[0] != "wx-tag-11" {
		t.Fatalf("sync wx tag ids=%#v", result.AddedWXTagIDs)
	}
	if len(result.AddedTagNames) != 0 {
		t.Fatalf("retry duplicated local tag names=%#v", result.AddedTagNames)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
