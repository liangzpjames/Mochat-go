package dashboard

import (
	"reflect"
	"testing"
)

func TestEmployeeApplyEventCarriesOnlyAuthoritativeBindingReference(t *testing.T) {
	typ := reflect.TypeOf(EmployeeApplyEvent{})
	if _, ok := typ.FieldByName("CorpIDs"); ok {
		t.Fatalf("employee apply payload still accepts client corp ids")
	}
	if _, ok := typ.FieldByName("TenantID"); ok {
		t.Fatalf("employee apply payload still accepts tenant id")
	}
	if _, ok := typ.FieldByName("BindingID"); !ok {
		t.Fatalf("employee apply payload must carry binding id")
	}
}
