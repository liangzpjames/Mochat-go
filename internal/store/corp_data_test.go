package store

import (
	"context"
	"database/sql"
	"fmt"
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

func TestCorpDataTrendQueryUsesSupportedTimezoneInsteadOfRangeStartOffset(t *testing.T) {
	dstLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, dstLocation)
	to := time.Date(2026, 4, 1, 0, 0, 0, 0, dstLocation)
	_, args := corpDataTrendQuery(dashboard.CorpDataScope{TenantID: 11, CorpID: 7}, from, to)

	timezoneArgs := 0
	for _, arg := range args {
		if value, ok := arg.(string); ok {
			timezoneArgs++
			if value != "+08:00" {
				t.Fatalf("trend timezone arg = %q", value)
			}
		}
	}
	if timezoneArgs != 4 {
		t.Fatalf("trend timezone arg count = %d", timezoneArgs)
	}
}

func TestCorpDataQuitQueriesUseIndexedTimestampColumn(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	scope := dashboard.CorpDataScope{TenantID: 11, CorpID: 7}
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, location)

	summary := strings.Join(strings.Fields(corpDataSummaryRoomMembersQuery(scope, []any{1, 2, 3, 4, 5, 6}).query), " ")
	trend, _ := corpDataTrendQuery(scope, now.AddDate(0, 0, -1), now.AddDate(0, 0, 1))
	trend = strings.Join(strings.Fields(trend), " ")
	for name, query := range map[string]string{"summary": summary, "trend": trend} {
		if strings.Contains(query, "STR_TO_DATE(contact_room.out_time") {
			t.Fatalf("%s query still wraps out_time in STR_TO_DATE: %s", name, query)
		}
		if !strings.Contains(query, "contact_room.updated_at") {
			t.Fatalf("%s query does not use the indexed quit timestamp: %s", name, query)
		}
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

func TestCorpDataSummaryQueryPlanUsesAtMostFourScopedResourceQueries(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	scope := dashboard.CorpDataScope{
		TenantID: 21, CorpID: 8, EmployeeIDs: []int{3}, DepartmentIDs: []int{6}, EmployeeScopeRestricted: true,
	}

	queries := corpDataSummaryQuerySpecs(scope, time.Date(2026, 8, 1, 12, 0, 0, 0, location))

	if len(queries) != 4 {
		t.Fatalf("summary query count = %d, want 4", len(queries))
	}
	if len(queries)+1 > 5 {
		t.Fatalf("index query count = %d, want <= 5", len(queries)+1)
	}
	for _, item := range queries {
		normalized := strings.Join(strings.Fields(item.query), " ")
		for _, fragment := range []string{
			"scoped_corp.tenant_id = ?",
			"scoped_employee.id IN (?)",
			"mc_work_employee_department",
			"department.corp_id = ?",
			"department.id IN (?)",
			"MAX(",
		} {
			if !strings.Contains(normalized, fragment) {
				t.Fatalf("%s query missing %q: %s", item.domain, fragment, normalized)
			}
		}
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
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("SET time_zone = '+00:00'"); err != nil {
		t.Fatal(err)
	}
	fixture := seedCorpDataIntegrationFixture(t, db)
	counter := &countingCorpDataQueryExecutor{db: db}
	store := NewMySQLStore(db)
	store.corpDataExecutor = counter
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, location)
	allScope := dashboard.CorpDataScope{TenantID: fixture.tenantA, CorpID: fixture.corpA}
	a1Scope := dashboard.CorpDataScope{
		TenantID: fixture.tenantA, CorpID: fixture.corpA, EmployeeIDs: []int{fixture.employeeA1}, EmployeeScopeRestricted: true,
	}
	departmentA1Scope := dashboard.CorpDataScope{
		TenantID: fixture.tenantA, CorpID: fixture.corpA, DepartmentIDs: []int{fixture.departmentA1},
	}
	emptyScope := dashboard.CorpDataScope{
		TenantID: fixture.tenantA, CorpID: fixture.corpA, EmployeeScopeRestricted: true,
	}
	assertCorpDataExplainPlans(t, db, a1Scope, now)

	summaryQueryRowsBefore := counter.queryRowCalls
	summaryQueriesBefore := counter.queryCalls
	allSummary, err := store.CorpDataSummary(context.Background(), allScope, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := counter.queryRowCalls - summaryQueryRowsBefore; got != 4 {
		t.Fatalf("CorpDataSummary QueryRowContext calls = %d, want 4", got)
	}
	if got := counter.queryCalls - summaryQueriesBefore; got != 0 {
		t.Fatalf("CorpDataSummary hidden QueryContext calls = %d, want 0", got)
	}
	t.Logf("QUERY_BUDGET CorpDataSummary QueryRowContext=%d QueryContext=%d", counter.queryRowCalls-summaryQueryRowsBefore, counter.queryCalls-summaryQueriesBefore)
	assertCorpDataSummary(t, allSummary, dashboard.CorpDataSummary{
		WeChatContactNum: 4, WeChatRoomNum: 2, RoomMemberNum: 2, CorpMemberNum: 2,
		AddContactNum: 2, LastAddContactNum: 1, AddIntoRoomNum: 1, LastAddIntoRoomNum: 1,
		LossContactNum: 1, QuitRoomNum: 1, AddFriendsNum: 3, LastAddFriendsNum: 1,
		MonthAddRoomNum: 2, MonthAddRoomMemberNum: 2, MonthLossContactNum: 1,
		UpdateTime: "2026-08-02 09:00:00",
	})

	a1Summary, err := store.CorpDataSummary(context.Background(), a1Scope, now)
	if err != nil {
		t.Fatal(err)
	}
	assertCorpDataSummary(t, a1Summary, dashboard.CorpDataSummary{
		WeChatContactNum: 3, WeChatRoomNum: 1, RoomMemberNum: 1, CorpMemberNum: 1,
		AddContactNum: 1, LastAddContactNum: 1, LastAddIntoRoomNum: 1,
		LossContactNum: 1, QuitRoomNum: 1, AddFriendsNum: 2, LastAddFriendsNum: 1,
		MonthAddRoomNum: 1, MonthAddRoomMemberNum: 1, MonthLossContactNum: 1,
		UpdateTime: "2026-08-02 03:00:00",
	})

	departmentSummary, err := store.CorpDataSummary(context.Background(), departmentA1Scope, now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(departmentSummary, a1Summary) {
		t.Fatalf("duplicate department relations changed counts: department=%#v employee=%#v", departmentSummary, a1Summary)
	}

	emptySummary, err := store.CorpDataSummary(context.Background(), emptyScope, now)
	if err != nil {
		t.Fatal(err)
	}
	if emptySummary != (dashboard.CorpDataSummary{}) {
		t.Fatalf("restricted empty summary = %#v", emptySummary)
	}

	crossTenantSummary, err := store.CorpDataSummary(context.Background(), dashboard.CorpDataScope{
		TenantID: fixture.tenantA, CorpID: fixture.corpB,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if crossTenantSummary != (dashboard.CorpDataSummary{}) {
		t.Fatalf("cross-tenant summary = %#v", crossTenantSummary)
	}
	tenantBSummary, err := store.CorpDataSummary(context.Background(), dashboard.CorpDataScope{
		TenantID: fixture.tenantB, CorpID: fixture.corpB,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if tenantBSummary.WeChatContactNum != 1 || tenantBSummary.WeChatRoomNum != 1 || tenantBSummary.RoomMemberNum != 1 || tenantBSummary.CorpMemberNum != 1 {
		t.Fatalf("tenant B fixture was not independently queryable: %#v", tenantBSummary)
	}

	from := time.Date(2026, 7, 31, 0, 0, 0, 0, location)
	to := time.Date(2026, 8, 2, 0, 0, 0, 0, location)
	trendQueryRowsBefore := counter.queryRowCalls
	trendQueriesBefore := counter.queryCalls
	points, err := store.CorpDataLineChat(context.Background(), a1Scope, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if got := counter.queryCalls - trendQueriesBefore; got != 1 {
		t.Fatalf("CorpDataLineChat QueryContext calls = %d, want 1", got)
	}
	if got := counter.queryRowCalls - trendQueryRowsBefore; got != 0 {
		t.Fatalf("CorpDataLineChat hidden QueryRowContext calls = %d, want 0", got)
	}
	t.Logf("QUERY_BUDGET CorpDataLineChat QueryRowContext=%d QueryContext=%d", counter.queryRowCalls-trendQueryRowsBefore, counter.queryCalls-trendQueriesBefore)
	assertCorpDataPoints(t, points, []dashboard.CorpDataPoint{
		{Date: "2026-07-31", AddContactNum: 1},
		{Date: "2026-08-01", AddContactNum: 1, AddIntoRoomNum: 1},
		{Date: "2026-08-02", AddContactNum: 1, LossContactNum: 1, QuitRoomNum: 1},
	})
	monthly := dashboard.AggregateCorpDataPeriod(points, "month")
	assertCorpDataPoints(t, monthly, []dashboard.CorpDataPoint{
		{Date: "2026-07", AddContactNum: 1},
		{Date: "2026-08", AddContactNum: 2, AddIntoRoomNum: 1, LossContactNum: 1, QuitRoomNum: 1},
	})
}

type countingCorpDataQueryExecutor struct {
	db            *sql.DB
	queryRowCalls int
	queryCalls    int
}

func (e *countingCorpDataQueryExecutor) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	e.queryRowCalls++
	return e.db.QueryRowContext(ctx, query, args...)
}

func (e *countingCorpDataQueryExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	e.queryCalls++
	return e.db.QueryContext(ctx, query, args...)
}

func assertCorpDataExplainPlans(t *testing.T, db *sql.DB, scope dashboard.CorpDataScope, now time.Time) {
	t.Helper()
	type plan struct {
		name    string
		query   string
		args    []any
		targets []string
	}
	plans := make([]plan, 0, 5)
	for _, spec := range corpDataSummaryQuerySpecs(scope, now) {
		target := map[string]string{
			"contacts": "contact_employee", "rooms": "room", "room_members": "contact_room", "employees": "employee",
		}[spec.domain]
		plans = append(plans, plan{name: "summary_" + spec.domain, query: spec.query, args: spec.args, targets: []string{target}})
	}
	trendQuery, trendArgs := corpDataTrendQuery(scope, now.AddDate(0, 0, -2), now.AddDate(0, 0, 1))
	plans = append(plans, plan{
		name: "trend", query: trendQuery, args: trendArgs,
		targets: []string{"contact_employee", "contact_room"},
	})

	for _, item := range plans {
		rows, err := db.Query("EXPLAIN "+item.query, item.args...)
		if err != nil {
			t.Fatalf("EXPLAIN %s: %v", item.name, err)
		}
		seen := make(map[string]bool, len(item.targets))
		for rows.Next() {
			var id sql.NullInt64
			var selectType, table, accessType, possibleKeys, key, keyLen, ref, rowEstimate, extra sql.NullString
			if err := rows.Scan(&id, &selectType, &table, &accessType, &possibleKeys, &key, &keyLen, &ref, &rowEstimate, &extra); err != nil {
				rows.Close()
				t.Fatalf("scan EXPLAIN %s: %v", item.name, err)
			}
			for _, target := range item.targets {
				if table.String != target {
					continue
				}
				seen[target] = true
				t.Logf("EXPLAIN %s table=%s type=%s key=%s rows=%s extra=%s", item.name, table.String, accessType.String, key.String, rowEstimate.String, extra.String)
				if strings.EqualFold(accessType.String, "ALL") {
					rows.Close()
					t.Fatalf("EXPLAIN %s performs ALL scan on %s (possible_keys=%q key=%q rows=%q extra=%q)", item.name, table.String, possibleKeys.String, key.String, rowEstimate.String, extra.String)
				}
			}
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close EXPLAIN %s: %v", item.name, err)
		}
		for _, target := range item.targets {
			if !seen[target] {
				t.Fatalf("EXPLAIN %s did not expose target fact table %s", item.name, target)
			}
		}
	}
}

type corpDataIntegrationFixture struct {
	tenantA      int
	tenantB      int
	corpA        int
	corpB        int
	employeeA1   int
	employeeA2   int
	employeeB1   int
	departmentA1 int
	departmentA2 int
	ids          map[string][]int
}

func seedCorpDataIntegrationFixture(t *testing.T, db *sql.DB) *corpDataIntegrationFixture {
	t.Helper()
	fixture := &corpDataIntegrationFixture{ids: map[string][]int{}}
	t.Cleanup(func() { fixture.cleanup(t, db) })
	prefix := fmt.Sprintf("task3_corp_data_%d", time.Now().UnixNano())
	fixture.tenantA = fixture.insert(t, db, "mc_tenant", `INSERT INTO mc_tenant (name, status) VALUES (?, 1)`, prefix+"_tenant_a")
	fixture.tenantB = fixture.insert(t, db, "mc_tenant", `INSERT INTO mc_tenant (name, status) VALUES (?, 1)`, prefix+"_tenant_b")
	fixture.corpA = fixture.insert(t, db, "mc_corp", `INSERT INTO mc_corp (name, wx_corpid, tenant_id, created_at, updated_at) VALUES (?, ?, ?, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, prefix+"_corp_a", prefix+"_wx_a", fixture.tenantA)
	fixture.corpB = fixture.insert(t, db, "mc_corp", `INSERT INTO mc_corp (name, wx_corpid, tenant_id, created_at, updated_at) VALUES (?, ?, ?, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, prefix+"_corp_b", prefix+"_wx_b", fixture.tenantB)
	fixture.departmentA1 = fixture.insert(t, db, "mc_work_department", `INSERT INTO mc_work_department (wx_department_id, corp_id, name, wx_parentid, created_at, updated_at) VALUES (101, ?, ?, 0, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, fixture.corpA, prefix+"_department_a1")
	fixture.departmentA2 = fixture.insert(t, db, "mc_work_department", `INSERT INTO mc_work_department (wx_department_id, corp_id, name, wx_parentid, created_at, updated_at) VALUES (102, ?, ?, 0, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, fixture.corpA, prefix+"_department_a2")
	departmentB := fixture.insert(t, db, "mc_work_department", `INSERT INTO mc_work_department (wx_department_id, corp_id, name, wx_parentid, created_at, updated_at) VALUES (201, ?, ?, 0, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, fixture.corpB, prefix+"_department_b")
	fixture.employeeA1 = fixture.insert(t, db, "mc_work_employee", `INSERT INTO mc_work_employee (wx_user_id, corp_id, name, status, created_at, updated_at) VALUES (?, ?, ?, 1, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, prefix+"_employee_a1", fixture.corpA, prefix+"_employee_a1")
	fixture.employeeA2 = fixture.insert(t, db, "mc_work_employee", `INSERT INTO mc_work_employee (wx_user_id, corp_id, name, status, created_at, updated_at) VALUES (?, ?, ?, 1, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, prefix+"_employee_a2", fixture.corpA, prefix+"_employee_a2")
	fixture.employeeB1 = fixture.insert(t, db, "mc_work_employee", `INSERT INTO mc_work_employee (wx_user_id, corp_id, name, status, created_at, updated_at) VALUES (?, ?, ?, 1, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, prefix+"_employee_b1", fixture.corpB, prefix+"_employee_b1")
	fixture.insert(t, db, "mc_work_employee_department", `INSERT INTO mc_work_employee_department (employee_id, department_id, created_at) VALUES (?, ?, '2026-07-01 00:00:00')`, fixture.employeeA1, fixture.departmentA1)
	fixture.insert(t, db, "mc_work_employee_department", `INSERT INTO mc_work_employee_department (employee_id, department_id, created_at) VALUES (?, ?, '2026-07-01 00:00:00')`, fixture.employeeA1, fixture.departmentA1)
	fixture.insert(t, db, "mc_work_employee_department", `INSERT INTO mc_work_employee_department (employee_id, department_id, created_at) VALUES (?, ?, '2026-07-01 00:00:00')`, fixture.employeeA2, fixture.departmentA2)
	fixture.insert(t, db, "mc_work_employee_department", `INSERT INTO mc_work_employee_department (employee_id, department_id, created_at) VALUES (?, ?, '2026-07-01 00:00:00')`, fixture.employeeB1, departmentB)

	fixture.addContact(t, db, prefix+"_a1_july", fixture.corpA, fixture.employeeA1, "2026-07-31 15:59:59", 1, nil)
	fixture.addContact(t, db, prefix+"_a1_august", fixture.corpA, fixture.employeeA1, "2026-07-31 16:00:00", 1, nil)
	fixture.addContact(t, db, prefix+"_a1_today", fixture.corpA, fixture.employeeA1, "2026-08-01 16:30:00", 1, nil)
	fixture.addContact(t, db, prefix+"_a1_loss", fixture.corpA, fixture.employeeA1, "2026-07-01 00:00:00", 2, ptr("2026-08-01 18:00:00"))
	fixture.addContact(t, db, prefix+"_a2_today", fixture.corpA, fixture.employeeA2, "2026-08-02 00:00:00", 1, nil)
	fixture.addContact(t, db, prefix+"_b_today", fixture.corpB, fixture.employeeB1, "2026-08-02 02:00:00", 1, nil)

	roomA1 := fixture.addRoom(t, db, prefix+"_room_a1", fixture.corpA, fixture.employeeA1, "2026-07-31 16:15:00")
	roomA2 := fixture.addRoom(t, db, prefix+"_room_a2", fixture.corpA, fixture.employeeA2, "2026-08-02 00:10:00")
	roomB := fixture.addRoom(t, db, prefix+"_room_b", fixture.corpB, fixture.employeeB1, "2026-08-02 02:10:00")
	fixture.addRoomMember(t, db, roomA1, fixture.employeeA1, 1, "2026-07-31 16:20:00", "")
	fixture.addRoomMember(t, db, roomA1, fixture.employeeA1, 2, "2026-07-01 00:00:00", "2026-08-01 19:00:00")
	fixture.addRoomMember(t, db, roomA2, fixture.employeeA2, 1, "2026-08-02 01:00:00", "")
	fixture.addRoomMember(t, db, roomB, fixture.employeeB1, 1, "2026-08-02 02:20:00", "")
	fixture.insert(t, db, "mc_corp_day_data", `INSERT INTO mc_corp_day_data (corp_id, add_contact_num, add_room_num, add_into_room_num, loss_contact_num, quit_room_num, date) VALUES (?, 999, 999, 999, 999, 999, '2026-08-02 00:00:00')`, fixture.corpA)
	fixture.insert(t, db, "mc_corp_day_data", `INSERT INTO mc_corp_day_data (corp_id, add_contact_num, add_room_num, add_into_room_num, loss_contact_num, quit_room_num, date) VALUES (?, 777, 777, 777, 777, 777, '2026-08-02 00:00:00')`, fixture.corpB)
	fixture.insert(t, db, "mc_work_update_time", `INSERT INTO mc_work_update_time (corp_id, type, last_update_time) VALUES (?, 6, '2026-08-31 00:00:00')`, fixture.corpA)
	noiseCorp := fixture.insert(t, db, "mc_corp", `INSERT INTO mc_corp (name, wx_corpid, tenant_id, created_at, updated_at) VALUES (?, ?, ?, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, prefix+"_corp_plan_noise", prefix+"_wx_plan_noise", fixture.tenantB)
	noiseDepartment := fixture.insert(t, db, "mc_work_department", `INSERT INTO mc_work_department (wx_department_id, corp_id, name, wx_parentid, created_at, updated_at) VALUES (301, ?, ?, 0, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, noiseCorp, prefix+"_department_plan_noise")
	noiseEmployee := fixture.insert(t, db, "mc_work_employee", `INSERT INTO mc_work_employee (wx_user_id, corp_id, name, status, created_at, updated_at) VALUES (?, ?, ?, 1, '2026-07-01 00:00:00', '2026-07-01 00:00:00')`, prefix+"_employee_plan_noise", noiseCorp, prefix+"_employee_plan_noise")
	noiseRoom := fixture.addRoom(t, db, prefix+"_room_plan_noise", noiseCorp, noiseEmployee, "2026-07-01 00:00:00")
	fixture.addExplainPlanNoise(t, db, prefix, noiseCorp, noiseEmployee, noiseDepartment, noiseRoom)
	return fixture
}

func (f *corpDataIntegrationFixture) insert(t *testing.T, db *sql.DB, table string, query string, args ...any) int {
	t.Helper()
	result, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("insert %s: %v", table, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("insert %s id: %v", table, err)
	}
	f.ids[table] = append(f.ids[table], int(id))
	return int(id)
}

func (f *corpDataIntegrationFixture) addExplainPlanNoise(t *testing.T, db *sql.DB, prefix string, corpID int, employeeID int, departmentID int, roomID int) {
	t.Helper()
	const rowCount = 512
	employees := make([][]any, 0, rowCount)
	contacts := make([][]any, 0, rowCount)
	rooms := make([][]any, 0, rowCount)
	members := make([][]any, 0, rowCount)
	departments := make([][]any, 0, rowCount)
	for index := 0; index < rowCount; index++ {
		name := fmt.Sprintf("%s_plan_noise_%03d", prefix, index)
		employees = append(employees, []any{name, corpID, name, 1, "2026-07-01 00:00:00", "2026-07-01 00:00:00"})
		contacts = append(contacts, []any{employeeID, 0, 0, corpID, 1, "2026-07-01 00:00:00", "2026-07-01 00:00:00", "2026-07-01 00:00:00"})
		rooms = append(rooms, []any{corpID, name, name, employeeID, "", 0, "2026-07-01 00:00:00", "2026-07-01 00:00:00", "2026-07-01 00:00:00"})
		members = append(members, []any{name, employeeID, roomID, 1, "2026-07-01 00:00:00", "", "2026-07-01 00:00:00", "2026-07-01 00:00:00"})
		departments = append(departments, []any{employeeID, departmentID, "2026-07-01 00:00:00", "2026-07-01 00:00:00"})
	}
	f.insertRows(t, db, "mc_work_employee", "wx_user_id, corp_id, name, status, created_at, updated_at", 6, employees)
	f.insertRows(t, db, "mc_work_contact_employee", "employee_id, contact_id, add_way, corp_id, status, create_time, created_at, updated_at", 8, contacts)
	f.insertRows(t, db, "mc_work_room", "corp_id, wx_chat_id, name, owner_id, notice, status, create_time, created_at, updated_at", 9, rooms)
	f.insertRows(t, db, "mc_work_contact_room", "wx_user_id, employee_id, room_id, status, join_time, out_time, created_at, updated_at", 8, members)
	f.insertRows(t, db, "mc_work_employee_department", "employee_id, department_id, created_at, updated_at", 4, departments)
	for _, table := range []string{"mc_work_contact_employee", "mc_work_room", "mc_work_contact_room", "mc_work_employee", "mc_work_employee_department"} {
		if _, err := db.Exec("ANALYZE TABLE " + table); err != nil {
			t.Fatalf("analyze %s: %v", table, err)
		}
	}
}

func (f *corpDataIntegrationFixture) insertRows(t *testing.T, db *sql.DB, table string, columns string, width int, rows [][]any) {
	t.Helper()
	rowPlaceholder := "(" + strings.TrimSuffix(strings.Repeat("?,", width), ",") + ")"
	placeholders := make([]string, len(rows))
	args := make([]any, 0, len(rows)*width)
	for index, row := range rows {
		if len(row) != width {
			t.Fatalf("insert %s row %d width = %d, want %d", table, index, len(row), width)
		}
		placeholders[index] = rowPlaceholder
		args = append(args, row...)
	}
	result, err := db.Exec("INSERT INTO "+table+" ("+columns+") VALUES "+strings.Join(placeholders, ","), args...)
	if err != nil {
		t.Fatalf("insert %s plan noise: %v", table, err)
	}
	firstID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("insert %s plan noise first id: %v", table, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("insert %s plan noise rows: %v", table, err)
	}
	for offset := int64(0); offset < affected; offset++ {
		f.ids[table] = append(f.ids[table], int(firstID+offset))
	}
}

func (f *corpDataIntegrationFixture) addContact(t *testing.T, db *sql.DB, name string, corpID int, employeeID int, createTime string, status int, deletedAt *string) {
	t.Helper()
	contactID := f.insert(t, db, "mc_work_contact", `INSERT INTO mc_work_contact (corp_id, wx_external_userid, name) VALUES (?, ?, ?)`, corpID, name, name)
	f.insert(t, db, "mc_work_contact_employee", `INSERT INTO mc_work_contact_employee (employee_id, contact_id, add_way, corp_id, status, create_time, created_at, updated_at, deleted_at) VALUES (?, ?, 0, ?, ?, ?, ?, ?, ?)`, employeeID, contactID, corpID, status, createTime, createTime, createTime, deletedAt)
}

func (f *corpDataIntegrationFixture) addRoom(t *testing.T, db *sql.DB, name string, corpID int, employeeID int, createdAt string) int {
	t.Helper()
	return f.insert(t, db, "mc_work_room", `INSERT INTO mc_work_room (corp_id, wx_chat_id, name, owner_id, notice, status, create_time, created_at, updated_at) VALUES (?, ?, ?, ?, '', 0, ?, ?, ?)`, corpID, name, name, employeeID, createdAt, createdAt, createdAt)
}

func (f *corpDataIntegrationFixture) addRoomMember(t *testing.T, db *sql.DB, roomID int, employeeID int, status int, joinTime string, outTime string) {
	t.Helper()
	updatedAt := joinTime
	if status == 2 && outTime != "" {
		updatedAt = outTime
	}
	f.insert(t, db, "mc_work_contact_room", `INSERT INTO mc_work_contact_room (wx_user_id, employee_id, room_id, status, join_time, out_time, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, fmt.Sprintf("member_%d_%d", roomID, len(f.ids["mc_work_contact_room"])), employeeID, roomID, status, joinTime, outTime, joinTime, updatedAt)
}

func (f *corpDataIntegrationFixture) cleanup(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{
		"mc_work_update_time", "mc_corp_day_data", "mc_work_contact_room", "mc_work_room",
		"mc_work_contact_employee", "mc_work_contact", "mc_work_employee_department",
		"mc_work_employee", "mc_work_department", "mc_corp", "mc_tenant",
	} {
		ids := f.ids[table]
		if len(ids) == 0 {
			continue
		}
		args := make([]any, 0, len(ids))
		for _, id := range ids {
			args = append(args, id)
		}
		if _, err := db.Exec("DELETE FROM "+table+" WHERE id IN ("+placeholders(len(ids))+")", args...); err != nil {
			t.Errorf("cleanup %s: %v", table, err)
		}
	}
}

func assertCorpDataSummary(t *testing.T, got dashboard.CorpDataSummary, want dashboard.CorpDataSummary) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("summary = %#v, want %#v", got, want)
	}
}

func assertCorpDataPoints(t *testing.T, got []dashboard.CorpDataPoint, want []dashboard.CorpDataPoint) {
	t.Helper()
	withoutIDs := make([]dashboard.CorpDataPoint, len(got))
	copy(withoutIDs, got)
	for index := range withoutIDs {
		withoutIDs[index].ID = 0
	}
	if !reflect.DeepEqual(withoutIDs, want) {
		t.Fatalf("points = %#v, want %#v", withoutIDs, want)
	}
}

func ptr(value string) *string {
	return &value
}
