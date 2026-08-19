package store

import (
	"reflect"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestStaffDirectoryWhereUsesPermissionDepartmentAndMode(t *testing.T) {
	where, args := staffDirectoryWhere(dashboard.WorkMessageStaffDirectoryFilter{
		TenantID: 1, CorpID: 7, UserID: 42, Mode: dashboard.WorkMessageStaffModeFocused,
		Keyword: "张%", DepartmentID: 10, RestrictEmployeeIDs: true, EmployeeIDs: []int{9, 10},
	})
	for _, fragment := range []string{"e.corp_id = ?", "e.id IN (?,?)", "e.name LIKE ? ESCAPE", "wed.department_id = ?", "focus.user_id = ?"} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("missing %q in %s", fragment, where)
		}
	}
	if !reflect.DeepEqual(args, []any{7, 9, 10, `%张\%%`, 10, 1, 7, 42}) {
		t.Fatalf("args=%#v", args)
	}
}

func TestStaffDirectoryWhereRestrictsEmptyPermissionScope(t *testing.T) {
	where, args := staffDirectoryWhere(dashboard.WorkMessageStaffDirectoryFilter{
		CorpID: 7, RestrictEmployeeIDs: true,
	})
	if where != "1 = 0" || len(args) != 0 {
		t.Fatalf("where=%q args=%#v", where, args)
	}
}

func TestStaffMessageWhereIncludesStableBeforeCursor(t *testing.T) {
	cursor := dashboard.EncodeWorkMessageStaffCursor(dashboard.WorkMessageStaffCursor{
		SentAt: "2026-08-16 10:00:00", TableIndex: 2, Seq: 88, ID: 9,
	})
	where, args, err := staffMessageWhere(dashboard.WorkMessageStaffDetailFilter{
		EmployeeID: 9, ToUserType: 1, ToUserID: 31, Keyword: "报价%", Date: "2026-08-16",
		MessageTypes: []int{2, 5}, Before: cursor,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"wm.work_employee_id = ?", "wm.to_user_type = ?", "wm.to_user_id = ?", "wm.msg_type IN (?,?)",
		"wm.content_text LIKE ? ESCAPE", "wm.msg_data_time >= ?", "wm.msg_data_time < ?", "wm.table_index < ?",
	} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("missing %q in %s", fragment, where)
		}
	}
	if len(args) < 12 {
		t.Fatalf("args=%#v", args)
	}
}

func TestBuildStaffDepartmentTreeKeepsFullOrganizationAndCountsVisibleEmployees(t *testing.T) {
	rows := []staffDepartmentRow{
		{ID: 1, ParentID: 0, Name: "总部", Order: 1},
		{ID: 2, ParentID: 1, Name: "销售部", Order: 2},
		{ID: 3, ParentID: 1, Name: "研发部", Order: 3},
	}
	memberships := map[int][]int{9: {2}, 10: {2}, 11: {3}}
	tree := buildStaffDepartmentTree(rows, memberships, map[int]struct{}{9: {}, 10: {}})
	if len(tree) != 1 || tree[0].EmployeeCount != 2 || len(tree[0].Children) != 2 || tree[0].Children[0].ID != 2 || tree[0].Children[0].EmployeeCount != 2 || tree[0].Children[1].ID != 3 || tree[0].Children[1].EmployeeCount != 0 {
		t.Fatalf("tree=%#v", tree)
	}
}

func TestStaffDepartmentDescendantIDsIncludesSelectedDepartmentAndChildren(t *testing.T) {
	rows := []staffDepartmentRow{
		{ID: 101, ParentID: 0, Name: "总公司"},
		{ID: 102, ParentID: 101, Name: "销售部"},
		{ID: 103, ParentID: 101, Name: "市场部"},
		{ID: 104, ParentID: 102, Name: "客服部"},
	}

	got := staffDepartmentDescendantIDs(rows, 101)
	for _, id := range []int{101, 102, 103, 104} {
		if _, ok := got[id]; !ok {
			t.Fatalf("department %d is missing from descendants: %#v", id, got)
		}
	}
	if _, ok := staffDepartmentDescendantIDs(rows, 102)[103]; ok {
		t.Fatalf("sibling department leaked into descendants: %#v", staffDepartmentDescendantIDs(rows, 102))
	}
}

func TestStaffEmployeeMatchesSelectedDepartmentDescendants(t *testing.T) {
	selected := map[int]struct{}{101: {}, 102: {}, 104: {}}
	if !staffEmployeeMatchesDepartment([]int{104}, selected) {
		t.Fatal("employee in a child department should match")
	}
	if staffEmployeeMatchesDepartment([]int{103}, selected) {
		t.Fatal("employee in a sibling department should not match")
	}
}

func TestStaffMessageContentAlwaysReturnsAnObject(t *testing.T) {
	for _, raw := range []string{`"hello"`, `42`, `null`, `not-json`} {
		if _, ok := staffMessageContent(raw).(map[string]any); !ok {
			t.Fatalf("raw=%q content=%#v", raw, staffMessageContent(raw))
		}
	}
}
