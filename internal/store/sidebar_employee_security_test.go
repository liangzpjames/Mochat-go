package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSidebarWorkContactQueriesBindEmployeeAndCorp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}

	mock.ExpectQuery(`(?s)FROM mc_work_contact AS contact.*JOIN mc_work_contact_employee AS pivot.*pivot.employee_id = \?.*employee.corp_id = contact.corp_id.*WHERE contact.id = \?`).
		WithArgs(5, 31).
		WillReturnRows(sqlmock.NewRows([]string{"name", "avatar", "gender", "business_no", "remark", "description"}))
	if _, found, err := store.WorkContactShowByID(context.Background(), 31, 5); err != nil || found {
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
