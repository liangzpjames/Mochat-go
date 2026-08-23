package store

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"jiyi/mochat-go/internal/dashboard"
)

func TestSidebarWorkContactByExternalUserIDBindsDuplicateIDToCorpAndEmployee(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectQuery(`(?s)FROM mc_work_contact AS contact.*JOIN mc_work_contact_employee AS pivot.*pivot.employee_id = \?.*pivot.corp_id = contact.corp_id.*JOIN mc_work_employee AS employee.*employee.corp_id = contact.corp_id.*WHERE contact.wx_external_userid = \? AND contact.corp_id = \?`).
		WithArgs(5, "duplicate-external-id", 7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "avatar", "corp_id"}).
			AddRow(31, "当前员工客户", "avatar.png", 7))

	contact, found, err := store.SidebarWorkContactByExternalUserID(context.Background(), "duplicate-external-id", 7, 5)
	if err != nil || !found || contact.ID != 31 || contact.CorpID != 7 {
		t.Fatalf("contact=%#v found=%v err=%v", contact, found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSidebarWorkContactQueriesBindEmployeeAndCorp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectQuery(`(?s)FROM mc_work_contact AS contact.*JOIN mc_work_contact_employee AS pivot.*pivot.employee_id = \?.*pivot.corp_id = contact.corp_id.*employee.corp_id = contact.corp_id.*WHERE contact.id = \? AND contact.corp_id = \?`).
		WithArgs(5, 31, 7).
		WillReturnRows(sqlmock.NewRows([]string{"name", "avatar", "gender", "business_no", "remark", "description"}))
	if _, found, err := store.WorkContactShowByID(context.Background(), 31, 5, 7); err != nil || found {
		t.Fatalf("show found=%v err=%v", found, err)
	}

	mock.ExpectQuery(`(?s)FROM mc_contact_employee_track AS track.*contact.corp_id = \?.*pivot.employee_id = \?.*employee.corp_id = contact.corp_id.*WHERE track.contact_id = \?`).
		WithArgs(7, 5, 31).
		WillReturnRows(sqlmock.NewRows([]string{"id", "content", "created_at"}))
	if tracks, err := store.SidebarContactEmployeeTracksByContactID(context.Background(), 31, 5, 7); err != nil || len(tracks) != 0 {
		t.Fatalf("tracks=%#v err=%v", tracks, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWorkContactShowTagsExcludeOtherEmployeeAndCorpNoise(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectQuery(`(?s)FROM mc_work_contact AS contact.*JOIN mc_work_contact_employee AS pivot.*pivot.employee_id = \?.*pivot.corp_id = contact.corp_id.*WHERE contact.id = \? AND contact.corp_id = \?`).
		WithArgs(5, 31, 7).
		WillReturnRows(sqlmock.NewRows([]string{"name", "avatar", "gender", "business_no", "remark", "description"}).
			AddRow("客户", "avatar.png", 1, "NO-31", "备注", "描述"))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag_pivot AS pivot.*JOIN mc_work_contact AS contact.*contact.corp_id = \?.*JOIN mc_work_contact_tag AS tag.*tag.corp_id = \?.*WHERE pivot.contact_id = \?\s+AND pivot.employee_id = \?\s+AND pivot.deleted_at IS NULL`).
		WithArgs(7, 7, 31, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(11, "当前员工当前企业标签"))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_room AS contact_room.*WHERE contact_room.contact_id = \?`).
		WithArgs(31).
		WillReturnRows(sqlmock.NewRows([]string{"name"}))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_employee AS contact_employee.*WHERE contact_employee.contact_id = \?`).
		WithArgs(31).
		WillReturnRows(sqlmock.NewRows([]string{"name", "corp_name"}))

	info, found, err := store.WorkContactShowByID(context.Background(), 31, 5, 7)
	if err != nil || !found {
		t.Fatalf("info=%#v found=%v err=%v", info, found, err)
	}
	if len(info.Tags) != 1 || info.Tags[0].TagID != 11 || info.Tags[0].TagName != "当前员工当前企业标签" {
		t.Fatalf("tags=%#v", info.Tags)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSidebarMediumReadAndWriteBindCorpVisibilityAndStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectQuery(`(?s)FROM mc_medium.*WHERE id = \? AND corp_id = \? AND sidebar_visible = 1 AND status = 'available' AND deleted_at IS NULL`).
		WithArgs(21, 7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "media_id", "last_upload_time", "type", "content"}))
	if _, found, err := store.SidebarMediumMediaForUpdateByID(context.Background(), 7, 21); err != nil || found {
		t.Fatalf("medium found=%v err=%v", found, err)
	}

	mock.ExpectExec(`(?s)UPDATE mc_medium.*WHERE id = \? AND corp_id = \? AND sidebar_visible = 1 AND status = 'available' AND deleted_at IS NULL`).
		WithArgs("new-media", int64(123), 21, 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if updated, err := store.UpdateSidebarMediumMediaID(context.Background(), 7, 21, "new-media", 123); err != nil || !updated {
		t.Fatalf("updated=%v err=%v", updated, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestContactSOPInfoBindsCorpAcrossBothLookupBranches(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	columns := []string{"id", "contact_sop_id", "creator", "task", "created_at", "contact_id", "contact_name", "avatar", "wx_external_userid", "updated_at"}
	base := `(?s)LEFT JOIN mc_contact_sop AS sop ON sop.id = log.contact_sop_id AND sop.corp_id = log.corp_id.*LEFT JOIN mc_work_contact AS contact ON contact.wx_external_userid = log.contact AND contact.corp_id = log.corp_id`

	mock.ExpectQuery(base+`.*WHERE log.corp_id = \? AND log.id = \?.*employee.corp_id = log.corp_id`).
		WithArgs(9, 55, 7).
		WillReturnRows(sqlmock.NewRows(columns))
	mock.ExpectQuery(base+`.*WHERE log.corp_id = \? AND log.contact_sop_id = \?.*employee.corp_id = log.corp_id`).
		WithArgs(9, 55, 7).
		WillReturnRows(sqlmock.NewRows(columns))
	if _, found, err := store.ContactSOPInfo(context.Background(), 7, 9, 55); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyWorkContactTagsRejectsTagIDsOutsideCurrentCorp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM mc_work_contact.*WHERE id = \? AND corp_id = \? AND deleted_at IS NULL`).
		WithArgs(31, 7).
		WillReturnRows(sqlmock.NewRows([]string{"wx_external_userid"}).AddRow("external-31"))
	mock.ExpectQuery(`(?s)FROM mc_work_employee.*WHERE id = \? AND corp_id = \? AND deleted_at IS NULL`).
		WithArgs(5, 7).
		WillReturnRows(sqlmock.NewRows([]string{"wx_user_id"}).AddRow("employee-5"))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag.*WHERE corp_id = \?.*id IN \(\?,\?\).*deleted_at IS NULL`).
		WithArgs(7, 11, 99).
		WillReturnRows(sqlmock.NewRows([]string{"id", "wx_contact_tag_id", "name"}).
			AddRow(11, "wx-tag-11", "当前企业标签"))
	mock.ExpectRollback()

	result, found, err := store.ApplyWorkContactTags(context.Background(), dashboard.MarkTagsApplyValues{
		CorpID: 7, ContactID: 31, EmployeeID: 5, TagIDs: []int{11, 99},
	})
	if found || !errors.Is(err, errWorkContactTagScope) {
		t.Fatalf("result=%#v found=%v err=%v", result, found, err)
	}
	if len(result.AddedWXTagIDs) != 0 || len(result.AddedTagNames) != 0 {
		t.Fatalf("rejected request returned added tags: %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyWorkContactTagsKeepsCurrentCorpTagWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM mc_work_contact.*WHERE id = \? AND corp_id = \? AND deleted_at IS NULL`).
		WithArgs(31, 7).
		WillReturnRows(sqlmock.NewRows([]string{"wx_external_userid"}).AddRow("external-31"))
	mock.ExpectQuery(`(?s)FROM mc_work_employee.*WHERE id = \? AND corp_id = \? AND deleted_at IS NULL`).
		WithArgs(5, 7).
		WillReturnRows(sqlmock.NewRows([]string{"wx_user_id"}).AddRow("employee-5"))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag.*WHERE corp_id = \?.*id IN \(\?\).*deleted_at IS NULL`).
		WithArgs(7, 11).
		WillReturnRows(sqlmock.NewRows([]string{"id", "wx_contact_tag_id", "name"}).
			AddRow(11, "wx-tag-11", "当前企业标签"))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag_pivot AS pivot.*JOIN mc_work_contact_tag AS tag.*tag.corp_id = \?.*pivot.contact_id = \?.*pivot.employee_id = \?`).
		WithArgs(7, 31, 5).
		WillReturnRows(sqlmock.NewRows([]string{"contact_tag_id"}))
	mock.ExpectExec(`(?s)INSERT INTO mc_work_contact_tag_pivot.*VALUES \(\?, \?, \?, 1, NOW\(\), NOW\(\)\)`).
		WithArgs(31, 5, 11).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`(?s)INSERT INTO mc_contact_employee_track`).
		WithArgs(5, 31, "系统对该客户打标签【当前企业标签】", 7, 2).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, found, err := store.ApplyWorkContactTags(context.Background(), dashboard.MarkTagsApplyValues{
		CorpID: 7, ContactID: 31, EmployeeID: 5, TagIDs: []int{11},
	})
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if len(result.AddedWXTagIDs) != 1 || result.AddedWXTagIDs[0] != "wx-tag-11" {
		t.Fatalf("added wx tag ids = %#v", result.AddedWXTagIDs)
	}
	if len(result.AddedTagNames) != 1 || result.AddedTagNames[0] != "当前企业标签" {
		t.Fatalf("added tag names = %#v", result.AddedTagNames)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateWorkContactProfileReportsTagWithoutWeComMapping(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM mc_work_contact_employee AS ce.*employee.corp_id = ce.corp_id.*contact.corp_id = ce.corp_id.*WHERE ce.employee_id = \? AND ce.contact_id = \? AND ce.corp_id = \?.*FOR UPDATE`).
		WithArgs(5, 31, 7).
		WillReturnRows(sqlmock.NewRows([]string{"wx_external_userid", "wx_user_id", "remark", "description", "business_no"}).
			AddRow("external-31", "employee-5", "", "", ""))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag.*corp_id = \?.*id IN \(\?\)`).
		WithArgs(7, 11).
		WillReturnRows(sqlmock.NewRows([]string{"id", "wx_contact_tag_id", "name"}).AddRow(11, "", "尚未同步标签"))
	mock.ExpectQuery(`(?s)FROM mc_work_contact_tag_pivot AS pivot.*tag.corp_id = \?.*pivot.contact_id = \?.*pivot.employee_id = \?`).
		WithArgs(7, 31, 5).
		WillReturnRows(sqlmock.NewRows([]string{"contact_tag_id"}))
	mock.ExpectExec(`(?s)INSERT INTO mc_work_contact_tag_pivot.*VALUES \(\?, \?, \?, 1, NOW\(\), NOW\(\)\)`).
		WithArgs(31, 5, 11).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`(?s)INSERT INTO mc_contact_employee_track`).
		WithArgs(5, 31, "系统对该客户打标签【尚未同步标签】", 7, 2).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, found, err := store.UpdateWorkContactProfile(context.Background(), dashboard.WorkContactUpdateValues{
		CorpID: 7, ContactID: 31, EmployeeID: 5, HasTag: true, TagIDs: []int{11},
	})
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if !result.TagSyncRequested || len(result.UnsyncableTagIDs) != 1 || result.UnsyncableTagIDs[0] != 11 || len(result.AddedWXTagIDs) != 0 {
		t.Fatalf("result=%#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
