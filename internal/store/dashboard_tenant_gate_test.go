package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

type fakeDashboardTenantAccessRow struct {
	values []any
	err    error
}

func (row fakeDashboardTenantAccessRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(dest) != len(row.values) {
		return errors.New("unexpected scan destination count")
	}
	for index := range dest {
		target := reflect.ValueOf(dest[index])
		value := reflect.ValueOf(row.values[index])
		if !target.IsValid() || target.Kind() != reflect.Pointer || !value.IsValid() || !value.Type().AssignableTo(target.Elem().Type()) {
			return errors.New("unexpected scan destination type")
		}
		target.Elem().Set(value)
	}
	return nil
}

func dashboardTenantAccessLimitsJSON(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(dashboard.SaaSAdminPackageLimits{})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func validDashboardTenantAccessRow(t *testing.T) fakeDashboardTenantAccessRow {
	t.Helper()
	return fakeDashboardTenantAccessRow{values: []any{
		9, 1,
		int64(21), "pro", 1, "2026-08-01 00:00:00", "2026-09-01 00:00:00", dashboardTenantAccessLimitsJSON(t),
		int64(31), dashboard.SaaSAdminSubscriptionStatusActive, "", "2026-09-01 00:00:00", "", 0,
	}}
}

func TestMySQLStoreDashboardTenantAccessUsesAuthenticatedTenant(t *testing.T) {
	var query string
	var args []any
	store := &MySQLStore{dashboardTenantAccessQueryRow: func(_ context.Context, gotQuery string, gotArgs ...any) dashboardTenantAccessRow {
		query, args = gotQuery, gotArgs
		return validDashboardTenantAccessRow(t)
	}}
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.Local)
	access, err := store.DashboardTenantAccess(context.Background(), 9, now)
	if err != nil {
		t.Fatal(err)
	}
	if !access.Allowed || access.TenantID != 9 {
		t.Fatalf("access = %+v", access)
	}
	if !strings.Contains(query, "WHERE t.id = ?") || !reflect.DeepEqual(args, []any{9}) {
		t.Fatalf("query=%q args=%v", query, args)
	}
}

func TestMySQLStoreDashboardTenantAccessFailsClosedOnMissingOrInvalidData(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.Local)
	tests := []struct {
		name   string
		row    dashboardTenantAccessRow
		reason string
	}{
		{name: "tenant missing", row: fakeDashboardTenantAccessRow{err: sql.ErrNoRows}, reason: dashboard.DashboardTenantAccessReasonTenantMissing},
		{name: "limits invalid", row: func() dashboardTenantAccessRow {
			row := validDashboardTenantAccessRow(t)
			row.values[7] = "{}"
			return row
		}(), reason: dashboard.DashboardTenantAccessReasonLimitsInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &MySQLStore{dashboardTenantAccessQueryRow: func(context.Context, string, ...any) dashboardTenantAccessRow { return test.row }}
			access, err := store.DashboardTenantAccess(context.Background(), 9, now)
			if err != nil {
				t.Fatal(err)
			}
			if access.Allowed || access.TenantID != 9 || access.Reason != test.reason {
				t.Fatalf("access = %+v", access)
			}
		})
	}
}

func TestMySQLStoreDashboardTenantAccessReturnsStorageError(t *testing.T) {
	want := errors.New("database unavailable")
	store := &MySQLStore{dashboardTenantAccessQueryRow: func(context.Context, string, ...any) dashboardTenantAccessRow {
		return fakeDashboardTenantAccessRow{err: want}
	}}
	access, err := store.DashboardTenantAccess(context.Background(), 9, time.Now())
	if !errors.Is(err, want) || access.Allowed {
		t.Fatalf("access=%+v err=%v", access, err)
	}
}
