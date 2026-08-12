package store

import (
	"reflect"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestSaaSAdminTenantScopeUsesExplicitTenantOnly(t *testing.T) {
	where := saasAdminTenantWhere("tenant_id", 1, 0)
	args := saasAdminTenantArgs(1, 0)
	if where != " AND tenant_id = ?" || !reflect.DeepEqual(args, []any{1}) {
		t.Fatalf("tenant one scope where=%q args=%v", where, args)
	}

	where = saasAdminTenantWhere("tenant_id", 12, 0)
	args = saasAdminTenantArgs(12, 0)
	if where != " AND tenant_id = ?" || !reflect.DeepEqual(args, []any{12}) {
		t.Fatalf("cross-tenant scope where=%q args=%v", where, args)
	}
}

func TestSaaSAdminBusinessQueriesDoNotExcludeSyntheticPlatformID(t *testing.T) {
	tests := []struct {
		name string
		call func() (string, []any)
	}{
		{
			name: "operation logs",
			call: func() (string, []any) {
				return saasAdminOperationLogWhere(dashboard.SaaSAdminOperationLogOptions{})
			},
		},
		{
			name: "billing events",
			call: func() (string, []any) {
				return saasAdminBillingEventWhere(dashboard.SaaSAdminBillingEventOptions{})
			},
		},
		{
			name: "admin tasks",
			call: func() (string, []any) {
				return saasAdminTaskWhere(dashboard.SaaSAdminTaskOptions{})
			},
		},
		{
			name: "alerts",
			call: func() (string, []any) {
				return saasAlertWhereSQL(dashboard.SaaSAlertListOptions{})
			},
		},
		{
			name: "notifications",
			call: func() (string, []any) {
				return saasAdminAlertNotificationWhereSQL(dashboard.SaaSAdminAlertNotificationOptions{})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			where, args := test.call()
			if strings.Contains(where, "tenant_id <> ?") {
				t.Fatalf("where=%q args=%v", where, args)
			}
		})
	}
}
