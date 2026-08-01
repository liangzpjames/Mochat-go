package store

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/mysqlconn"
)

func TestCorpDataTrendQueryScopesTenantCorpEmployeesDepartmentsAndInclusiveDateRange(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	scope := dashboard.CorpDataScope{
		TenantID: 11, CorpID: 7, EmployeeIDs: []int{3, 5}, DepartmentIDs: []int{13}, EmployeeScopeRestricted: true,
	}
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, location)
	to := time.Date(2026, 8, 1, 0, 0, 0, 0, location)
	query, args := corpDataTrendQuery(scope, from, to)
	normalized := strings.Join(strings.Fields(query), " ")

	for _, fragment := range []string{
		"INNER JOIN mc_corp AS scoped_corp",
		"scoped_corp.tenant_id = ?",
		"corp_id = ?",
		"scoped_employee.id IN (?,?)",
		"mc_work_employee_department",
		"mc_work_department",
		"department.corp_id = ?",
		"department.id IN (?)",
	} {
		if !strings.Contains(normalized, fragment) {
			t.Fatalf("query missing %q: %s", fragment, normalized)
		}
	}
	if !strings.Contains(normalized, ">= FROM_UNIXTIME(?)") || !strings.Contains(normalized, "< FROM_UNIXTIME(?)") {
		t.Fatalf("query is not sargable and inclusive: %s", normalized)
	}
	if !strings.Contains(normalized, "ORDER BY date ASC LIMIT 31") {
		t.Fatalf("query does not preserve deterministic ordering/window: %s", normalized)
	}
	if len(args) == 0 || !reflect.DeepEqual(args[len(args)-2:], []any{from.Unix(), to.Unix()}) {
		t.Fatalf("range args = %#v", args)
	}
}

func TestCorpDataTrendQueryMakesRestrictedEmptyScopeMatchZeroRows(t *testing.T) {
	query, _ := corpDataTrendQuery(dashboard.CorpDataScope{
		TenantID: 11, CorpID: 7, EmployeeIDs: []int{}, EmployeeScopeRestricted: true,
	}, time.Unix(0, 0).UTC(), time.Unix(3600, 0).UTC())
	if !strings.Contains(strings.Join(strings.Fields(query), " "), "1 = 0") {
		t.Fatalf("restricted empty query can become unfiltered: %s", query)
	}
}

func TestCorpDataMetricQueryScopesTenantCorpAndDepartment(t *testing.T) {
	query, args := corpDataMetricCountQuery(corpDataMetricContacts, dashboard.CorpDataScope{
		TenantID: 21, CorpID: 8, DepartmentIDs: []int{6},
	}, time.Time{}, time.Time{})
	normalized := strings.Join(strings.Fields(query), " ")
	for _, fragment := range []string{"scoped_corp.tenant_id = ?", "contact_employee.corp_id = ?", "department.corp_id = ?", "department.id IN (?)"} {
		if !strings.Contains(normalized, fragment) {
			t.Fatalf("query missing %q: %s", fragment, normalized)
		}
	}
	if !reflect.DeepEqual(args[:2], []any{21, 8}) {
		t.Fatalf("tenant/corp args = %#v", args)
	}
}

func TestIntegrationCorpDataScopedQueriesExecute(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_MYSQL_DSN"))
	if dsn == "" {
		if os.Getenv("MOCHAT_REQUIRE_MYSQL_INTEGRATION") == "1" {
			t.Fatal("MOCHAT_MYSQL_DSN is required")
		}
		t.Skip("MOCHAT_MYSQL_DSN is required for MySQL integration tests")
	}
	db, err := mysqlconn.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	scope := dashboard.CorpDataScope{
		TenantID: 999999, CorpID: 999999, EmployeeIDs: []int{999999}, DepartmentIDs: []int{999999}, EmployeeScopeRestricted: true,
	}

	if _, err := store.CorpDataSummary(context.Background(), scope, time.Date(2026, 8, 1, 12, 0, 0, 0, location)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CorpDataLineChat(context.Background(), scope, time.Date(2026, 7, 1, 0, 0, 0, 0, location), time.Date(2026, 7, 31, 0, 0, 0, 0, location)); err != nil {
		t.Fatal(err)
	}
}
