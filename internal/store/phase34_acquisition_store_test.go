package store

import (
	"reflect"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestPhase34AcquisitionLinkWhereAlwaysScopesCorpAndFiltersName(t *testing.T) {
	where, args := phase34AcquisitionLinkWhere(dashboard.Phase34AcquisitionLinkFilter{CorpID: 7, Name: "官网", Status: "draft"}, "l")
	wantWhere := []string{"l.corp_id = ?", "l.deleted_at IS NULL", "l.name LIKE ?", "l.status = ?"}
	wantArgs := []any{7, "%官网%", "draft"}
	if !reflect.DeepEqual(where, wantWhere) || !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("where=%#v args=%#v", where, args)
	}
}

func TestPhase34ShortLinkWhereDoesNotAcceptMissingCorpScope(t *testing.T) {
	where, args := phase34ShortLinkWhere(dashboard.Phase34ShortLinkFilter{Name: "活动"}, "s")
	if len(where) == 0 || where[0] != "s.corp_id = ?" || len(args) == 0 || args[0] != 0 {
		t.Fatalf("where=%#v args=%#v", where, args)
	}
}
