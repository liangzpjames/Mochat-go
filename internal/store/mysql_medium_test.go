package store

import (
	"reflect"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestMediumWhereIncludesScopeStatusAndSidebarVisibility(t *testing.T) {
	visible := true
	where, args := mediumWhere(dashboard.MediumFilter{
		CorpID: 7, ScopeType: "personal", ScopeID: 3, Status: "available", SidebarVisible: &visible,
	}, "m")
	wantWhere := []string{
		"m.corp_id = ?", "m.is_sync = 1", "m.deleted_at IS NULL", "m.scope_type = ?", "m.scope_id = ?", "m.status = ?", "m.sidebar_visible = ?",
	}
	wantArgs := []any{7, "personal", 3, "available", true}
	if !reflect.DeepEqual(where, wantWhere) || !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("where=%#v args=%#v", where, args)
	}
}
