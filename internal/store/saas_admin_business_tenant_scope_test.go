package store

import (
	"reflect"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestSaaSAdminTenantScopeWhereExcludesControlPlaneTenant(t *testing.T) {
	where := saasAdminTenantWhere("tenant_id", 0, 1)
	args := saasAdminTenantArgs(0, 1)
	if where != " AND tenant_id <> ?" || !reflect.DeepEqual(args, []any{1}) {
		t.Fatalf("where=%q args=%v", where, args)
	}

	where = saasAdminTenantWhere("tenant_id", 12, 1)
	args = saasAdminTenantArgs(12, 1)
	if where != " AND tenant_id = ? AND tenant_id <> ?" || !reflect.DeepEqual(args, []any{12, 1}) {
		t.Fatalf("where=%q args=%v", where, args)
	}
}

func TestSaaSAdminBusinessQueriesCarryExcludedTenant(t *testing.T) {
	tests := []struct {
		name string
		call func() (string, []any)
	}{
		{
			name: "operation logs",
			call: func() (string, []any) {
				return saasAdminOperationLogWhere(dashboard.SaaSAdminOperationLogOptions{ExcludedTenantID: 1})
			},
		},
		{
			name: "billing events",
			call: func() (string, []any) {
				return saasAdminBillingEventWhere(dashboard.SaaSAdminBillingEventOptions{ExcludedTenantID: 1})
			},
		},
		{
			name: "admin tasks",
			call: func() (string, []any) {
				return saasAdminTaskWhere(dashboard.SaaSAdminTaskOptions{ExcludedTenantID: 1})
			},
		},
		{
			name: "alerts",
			call: func() (string, []any) {
				return saasAlertWhereSQL(dashboard.SaaSAlertListOptions{ExcludedTenantID: 1})
			},
		},
		{
			name: "notifications",
			call: func() (string, []any) {
				return saasAdminAlertNotificationWhereSQL(dashboard.SaaSAdminAlertNotificationOptions{ExcludedTenantID: 1})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			where, args := test.call()
			if !strings.Contains(where, "tenant_id <> ?") || !reflect.DeepEqual(args, []any{1}) {
				t.Fatalf("where=%q args=%v", where, args)
			}
		})
	}
}
