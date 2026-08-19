package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestCustomerDirectoryModePredicateIsMutuallyExclusive(t *testing.T) {
	cases := map[dashboard.WorkMessageCustomerMode]string{
		dashboard.WorkMessageCustomerModeFocused: "focused_conversation_count > 0",
		dashboard.WorkMessageCustomerModeActive:  "active_relation_count > 0",
		dashboard.WorkMessageCustomerModeLost:    "active_relation_count = 0 AND lost_relation_count > 0",
	}
	for mode, fragment := range cases {
		where, _ := customerDirectoryOuterWhere(dashboard.WorkMessageCustomerDirectoryFilter{Mode: mode})
		if !strings.Contains(where, fragment) {
			t.Fatalf("mode=%s where=%s", mode, where)
		}
	}
}

func TestCustomerDirectoryOuterWhereEscapesKeywordAndDoesNotBroadenMode(t *testing.T) {
	where, args := customerDirectoryOuterWhere(dashboard.WorkMessageCustomerDirectoryFilter{
		Mode: dashboard.WorkMessageCustomerModeLost, Keyword: "甲%",
	})
	for _, fragment := range []string{"active_relation_count = 0", "lost_relation_count > 0", "name LIKE ? ESCAPE", "archive_name LIKE ? ESCAPE", "external_userid LIKE ? ESCAPE"} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("where=%q missing=%q", where, fragment)
		}
	}
	if len(args) != 3 || args[0] != `%甲\%%` || args[1] != `%甲\%%` || args[2] != `%甲\%%` {
		t.Fatalf("args=%#v", args)
	}
}

func TestEmptyCustomerDirectoryPreservesFixedPageContract(t *testing.T) {
	page := emptyCustomerDirectory(dashboard.WorkMessageCustomerDirectoryFilter{Page: 0})
	if page.Page != 1 || page.PageSize != 50 || page.Customers == nil || page.Limitations == nil || page.Capabilities == nil {
		t.Fatalf("page=%#v", page)
	}
}
