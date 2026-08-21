package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestChannelCodeDrainageEmployeeIDsCollectsNestedEmployeeIDs(t *testing.T) {
	value := map[string]any{
		"employees": []any{
			map[string]any{"employeeId": 1004},
			map[string]any{"timeSlot": []any{
				map[string]any{"employeeId": []any{1005, 1006}},
			}},
		},
		"unrelated": 7,
	}
	got := channelCodeDrainageEmployeeIDs(value)
	if len(got) != 3 || got[0] != 1004 || got[1] != 1005 || got[2] != 1006 {
		t.Fatalf("employee ids = %#v", got)
	}
}

func TestChannelCodeWorkspaceWhereExcludesSimulationAndScopesEmployee(t *testing.T) {
	where, args := channelCodeWorkspaceWhere(dashboard.ChannelCodeWorkspaceFilter{
		CorpIDs: []int{7}, Name: "展会", Creator: "李娜", EmployeeID: 1004, State: "active",
	})
	joined := strings.Join(where, " AND ")
	for _, required := range []string{
		"cc.corp_id IN (?)",
		"cc.data_source <> 'simulation'",
		"creator_filter.name LIKE ?",
		"JSON_SEARCH(cc.drainage_employee",
		"cc.lifecycle_state = ?",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("where missing %q: %s", required, joined)
		}
	}
	if len(args) != 5 {
		t.Fatalf("args = %#v", args)
	}
}
