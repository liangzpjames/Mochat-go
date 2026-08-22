package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/dashboard"
)

func TestSidebarWorkbenchSummaryBindsEmployeeAndCorpAcrossMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)

	mock.ExpectQuery(`(?s)SELECT employee.id.*FROM mc_work_contact_employee AS rel.*rel.employee_id = employee.id.*rel.corp_id = employee.corp_id.*mc_work_contact_tag_pivot AS pivot.*pivot.employee_id = employee.id.*FROM mc_work_room AS room.*room.owner_id = employee.id.*FROM mc_contact_sop_log AS contact_log.*contact_log.employee = employee.wx_user_id.*FROM mc_room_sop_log AS room_log.*room_log.employee = employee.wx_user_id.*FROM mc_contact_batch_add_import AS batch.*batch.employee_id = employee.id.*FROM mc_work_employee AS employee.*JOIN mc_corp AS corp.*employee.id = \?.*employee.corp_id = \?`).
		WithArgs(7, 9).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "avatar", "corp_name", "department_names",
			"customer_total", "added_today", "tagged_total", "owned_room_total",
			"contact_sop_pending", "room_sop_pending", "batch_add_pending",
		}).AddRow(7, "员工甲", "avatar.png", "示例企业", "销售部\x1f华东区", 12, 2, 4, 1, 3, 2, 1))

	summary, err := store.SidebarWorkbenchSummary(context.Background(), dashboard.SidebarEmployee{ID: 7, CorpID: 9})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Employee.Name != "员工甲" || len(summary.Employee.DepartmentNames) != 2 || summary.Customers.Total != 12 || summary.Tasks.BatchAddPending != 1 {
		t.Fatalf("summary=%+v", summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSidebarContactsBindsEmployeeCorpAndKeywordForCountAndPage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)

	base := `(?s)FROM mc_work_contact_employee AS rel.*JOIN mc_work_contact AS contact.*contact.corp_id = rel.corp_id.*rel.employee_id = \?.*rel.corp_id = \?.*rel.deleted_at IS NULL.*contact.deleted_at IS NULL.*contact.name LIKE \?.*rel.remark LIKE \?`
	mock.ExpectQuery(base).WithArgs(7, 9, "%客户%", "%客户%").WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	mock.ExpectQuery(base+`.*ORDER BY rel.create_time DESC.*LIMIT \? OFFSET \?`).
		WithArgs(7, 9, "%客户%", "%客户%", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "wx_external_userid", "name", "avatar", "remark", "status", "added_at", "tags"}).
			AddRow(11, "external-11", "客户甲", "", "重点", 1, "2026-08-23 09:00:00", "高意向\x1f已沟通"))

	page, err := store.SidebarContacts(context.Background(), dashboard.SidebarEmployee{ID: 7, CorpID: 9}, dashboard.SidebarContactFilter{Keyword: "客户", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.TotalPage != 1 || len(page.Items) != 1 || len(page.Items[0].Tags) != 2 || page.Items[0].WXExternalUserID != "external-11" {
		t.Fatalf("page=%+v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSidebarTasksUsesEmployeeCorpForRoomSOP(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)

	base := `(?s)FROM mc_room_sop_log AS log.*JOIN mc_work_employee AS employee.*employee.id = \?.*employee.corp_id = \?.*employee.wx_user_id = log.employee.*JOIN mc_work_room AS room.*room.corp_id = log.corp_id.*log.state = 0`
	mock.ExpectQuery(regexp.MustCompile(base).String()).WithArgs(7, 9).WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	mock.ExpectQuery(regexp.MustCompile(base+`.*ORDER BY scheduled_at DESC.*LIMIT \? OFFSET \?`).String()).
		WithArgs(7, 9, 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "kind", "title", "subject_name", "scheduled_at", "state"}).
			AddRow(31, "roomSop", "群 SOP", "客户群甲", "2026-08-23 10:00:00", "pending"))

	page, err := store.SidebarTasks(context.Background(), dashboard.SidebarEmployee{ID: 7, CorpID: 9}, dashboard.SidebarTaskFilter{Kind: "roomSop", State: "pending", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Kind != "roomSop" || page.Items[0].SubjectName != "客户群甲" {
		t.Fatalf("page=%+v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSidebarContactSOPDoneIsAnHonestEmptyState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)

	page, err := store.SidebarTasks(context.Background(), dashboard.SidebarEmployee{ID: 7, CorpID: 9}, dashboard.SidebarTaskFilter{Kind: "contactSop", State: "done", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("page=%+v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
