package store

import (
	"reflect"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestWorkMessageUserWhereAppliesConversationFiltersAndPermissionScope(t *testing.T) {
	where, args := workMessageUserWhere(dashboard.WorkMessageUserFilter{
		WorkEmployeeID:      9,
		ToUserType:          1,
		ToUserID:            31,
		Keyword:             "报价",
		DateTimeStart:       "2026-07-01 00:00:00",
		DateTimeEnd:         "2026-07-31 23:59:59",
		RestrictEmployeeIDs: true,
		EmployeeIDs:         []int{9, 10},
	})

	for _, fragment := range []string{
		"work_employee_id = ?",
		"to_user_type = ?",
		"to_user_id = ?",
		"(employee_name LIKE ? OR sender_name LIKE ? OR target_name LIKE ? OR content_text LIKE ?)",
		"msg_data_time >= ?",
		"msg_data_time <= ?",
		"work_employee_id IN (?,?)",
	} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("where %q missing %q", where, fragment)
		}
	}
	want := []any{
		9, 1, 31,
		"%报价%", "%报价%", "%报价%", "%报价%",
		"2026-07-01 00:00:00", "2026-07-31 23:59:59",
		9, 10,
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestWorkMessageUserWhereRestrictsEmptyPermissionScope(t *testing.T) {
	where, args := workMessageUserWhere(dashboard.WorkMessageUserFilter{
		ToUserType:          -1,
		RestrictEmployeeIDs: true,
	})

	if where != "1 = 0" {
		t.Fatalf("where = %q", where)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v", args)
	}
}

func TestWorkMessageUserWherePreservesLegacyEmployeeRequirement(t *testing.T) {
	where, args := workMessageUserWhere(dashboard.WorkMessageUserFilter{
		WorkEmployeeID: 0,
		ToUserType:     1,
	})

	if where != "work_employee_id = ? AND to_user_type = ?" {
		t.Fatalf("where = %q", where)
	}
	if !reflect.DeepEqual(args, []any{0, 1}) {
		t.Fatalf("args = %#v", args)
	}
}

func TestWorkMessageUnionSQLScopesEveryArchiveTableToCorp(t *testing.T) {
	query, args := workMessageUnionSQL(7)

	if count := strings.Count(query, "WHERE wm.corp_id = ?"); count != 10 {
		t.Fatalf("corp predicates = %d", count)
	}
	if len(args) != 10 {
		t.Fatalf("corp args = %#v", args)
	}
	for _, arg := range args {
		if arg != 7 {
			t.Fatalf("cross-corp arg = %#v", arg)
		}
	}
	if !strings.Contains(query, "WHEN COALESCE(wm.to_user_type, 0) = 2 THEN '群成员'") {
		t.Fatalf("group sender must use a neutral label when participant identity is unavailable")
	}
}

func TestWorkMessageConversationGroupingKeepsEmployeesSeparate(t *testing.T) {
	columns, key := workMessageConversationGrouping()

	if columns != "work_employee_id, to_user_type, to_user_id" {
		t.Fatalf("columns = %q", columns)
	}
	if key != "CONCAT(wm.work_employee_id, ':', wm.to_user_type, ':', wm.to_user_id)" {
		t.Fatalf("key = %q", key)
	}
}

func TestWorkMessagePageWindowReadsLatestWithoutLargeOffset(t *testing.T) {
	order, offset, reverse := workMessagePageWindow(dashboard.WorkMessageFilter{
		Page: 1, PerPage: 200, Latest: true,
	})
	if order != "msg_data_time DESC, seq DESC, table_index DESC, id DESC" || offset != 0 || !reverse {
		t.Fatalf("latest window = order %q, offset %d, reverse %v", order, offset, reverse)
	}

	order, offset, reverse = workMessagePageWindow(dashboard.WorkMessageFilter{
		Page: 2, PerPage: 200,
	})
	if order != "msg_data_time ASC, seq ASC, table_index ASC, id ASC" || offset != 200 || reverse {
		t.Fatalf("paged window = order %q, offset %d, reverse %v", order, offset, reverse)
	}
}

func TestReverseWorkMessageItemsRestoresChronologicalDisplay(t *testing.T) {
	items := []dashboard.WorkMessageItem{{ID: 3}, {ID: 2}, {ID: 1}}
	reverseWorkMessageItems(items)
	if items[0].ID != 1 || items[1].ID != 2 || items[2].ID != 3 {
		t.Fatalf("items = %#v", items)
	}
}
