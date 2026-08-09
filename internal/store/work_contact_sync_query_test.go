package store

import (
	"strings"
	"testing"
)

func TestWorkContactSyncEmployeesQueryOnlySelectsAuthorizedCustomerContactEmployees(t *testing.T) {
	query := strings.Join(strings.Fields(workContactSyncEmployeesQuery()), " ")
	for _, fragment := range []string{
		"corp_id = ?", "contact_auth = 1", "wx_user_id <> ''", "deleted_at IS NULL",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q: %s", fragment, query)
		}
	}
}
