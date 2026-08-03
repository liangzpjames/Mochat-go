package store

import "testing"

func TestNormalizeTimeoutPage(t *testing.T) {
	page, perPage := normalizeTimeoutPage(0, 0)
	if page != 1 || perPage != 20 {
		t.Fatalf("page=%d perPage=%d", page, perPage)
	}
	page, perPage = normalizeTimeoutPage(3, 101)
	if page != 3 || perPage != 20 {
		t.Fatalf("page=%d perPage=%d", page, perPage)
	}
}

func TestTimeoutScopeWhereRequiresTenantWhenPresent(t *testing.T) {
	where, args := timeoutScopeWhere(3, 2)
	if where != " WHERE tenant_id = ? AND corp_id = ?" || len(args) != 2 || args[0] != 3 || args[1] != 2 {
		t.Fatalf("where=%q args=%#v", where, args)
	}
}
