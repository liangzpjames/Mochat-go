package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/mysqlconn"
)

func TestWorkMessageArchivePredicateSeparatesSimulationFromRealArchive(t *testing.T) {
	realPredicate, ok := workMessageArchivePredicate(workMessageArchiveReal)
	if !ok || realPredicate != "msgid NOT LIKE 'MOCHAT-SIM:%'" {
		t.Fatalf("real archive predicate = %q, %v", realPredicate, ok)
	}
	simulationPredicate, ok := workMessageArchivePredicate(workMessageArchiveSimulation)
	if !ok || simulationPredicate != "msgid LIKE 'MOCHAT-SIM:%'" {
		t.Fatalf("simulation archive predicate = %q, %v", simulationPredicate, ok)
	}
	if predicate, ok := workMessageArchivePredicate(workMessageArchiveUnavailable); ok || predicate != "" {
		t.Fatalf("unavailable archive predicate = %q, %v", predicate, ok)
	}
}

func TestWorkMessageUserWhereAppliesConversationFiltersAndPermissionScope(t *testing.T) {
	where, args := workMessageUserWhere(dashboard.WorkMessageUserFilter{
		WorkEmployeeID:      9,
		ToUserType:          1,
		ToUserID:            31,
		Keyword:             "报价",
		DateTimeStart:       "2026-07-01 00:00:00",
		DateTimeEnd:         "2026-07-31 23:59:59",
		RestrictEmployeeIDs: true,
		EmployeeIDs:         []int{9, 10},
	})

	for _, fragment := range []string{
		"work_employee_id = ?",
		"to_user_type = ?",
		"to_user_id = ?",
		`(employee_name LIKE ? ESCAPE '\\' OR sender_name LIKE ? ESCAPE '\\' OR target_name LIKE ? ESCAPE '\\' OR content_text LIKE ? ESCAPE '\\')`,
		"msg_data_time >= ?",
		"msg_data_time < ?",
		"work_employee_id IN (?,?)",
	} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("where %q missing %q", where, fragment)
		}
	}
	want := []any{
		9, 1, 31,
		"2026-07-01 00:00:00", "2026-07-31 23:59:59",
		9, 10,
		"%报价%", "%报价%", "%报价%", "%报价%",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestWorkMessageUserWhereUsesRequestedEmployeeSetWithoutSingleEmployeeFallback(t *testing.T) {
	where, args := workMessageUserWhere(dashboard.WorkMessageUserFilter{
		AllowAllEmployees:   true,
		ToUserType:          0,
		RestrictEmployeeIDs: true,
		EmployeeIDs:         []int{12, 9, 12},
	})

	if strings.Contains(where, "work_employee_id = ?") {
		t.Fatalf("where unexpectedly narrows to one employee: %q", where)
	}
	if !strings.Contains(where, "work_employee_id IN (?,?)") {
		t.Fatalf("where = %q", where)
	}
	if !reflect.DeepEqual(args, []any{0, 12, 9}) {
		t.Fatalf("args = %#v", args)
	}
}

func TestWorkMessageUserWhereTreatsKeywordWildcardsAsLiterals(t *testing.T) {
	where, args := workMessageUserWhere(dashboard.WorkMessageUserFilter{
		AllowAllEmployees: true,
		ToUserType:        -1,
		Keyword:           `报价_100%\确认`,
	})

	if strings.Count(where, `LIKE ? ESCAPE '\\'`) != 4 {
		t.Fatalf("where = %q", where)
	}
	want := `%报价\_100\%\\确认%`
	if !reflect.DeepEqual(args, []any{want, want, want, want}) {
		t.Fatalf("args = %#v", args)
	}
}

func TestWorkMessageUserWhereRestrictsEmptyPermissionScope(t *testing.T) {
	where, args := workMessageUserWhere(dashboard.WorkMessageUserFilter{
		ToUserType:          -1,
		RestrictEmployeeIDs: true,
	})

	if where != "1 = 0" {
		t.Fatalf("where = %q", where)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v", args)
	}
}

func TestWorkMessageUserWherePreservesLegacyEmployeeRequirement(t *testing.T) {
	where, args := workMessageUserWhere(dashboard.WorkMessageUserFilter{
		WorkEmployeeID: 0,
		ToUserType:     1,
	})

	if where != "work_employee_id = ? AND to_user_type = ?" {
		t.Fatalf("where = %q", where)
	}
	if !reflect.DeepEqual(args, []any{0, 1}) {
		t.Fatalf("args = %#v", args)
	}
}

func TestWorkMessageUnionSQLScopesEveryArchiveTableToCorp(t *testing.T) {
	query, args := workMessageUnionSQL(7)

	if count := strings.Count(query, "WHERE wm.corp_id = ?"); count != 10 {
		t.Fatalf("corp predicates = %d", count)
	}
	if len(args) != 10 {
		t.Fatalf("corp args = %#v", args)
	}
	for _, arg := range args {
		if arg != 7 {
			t.Fatalf("cross-corp arg = %#v", arg)
		}
	}
	if !strings.Contains(query, "WHEN COALESCE(wm.to_user_type, 0) = 2 THEN '群成员'") {
		t.Fatalf("group sender must use a neutral label when participant identity is unavailable")
	}
	for _, join := range []string{
		"sender.corp_id = wm.corp_id",
		"target_employee.corp_id = wm.corp_id",
		"target_contact.corp_id = wm.corp_id",
		"target_room.corp_id = wm.corp_id",
	} {
		if count := strings.Count(query, join); count != 10 {
			t.Fatalf("%s predicates = %d", join, count)
		}
	}
}

func TestWorkMessageFilteredUnionPushesIndexableScopeIntoEveryShard(t *testing.T) {
	query, args := workMessageFilteredUnionSQL(dashboard.WorkMessageUserFilter{
		CorpID:              7,
		AllowAllEmployees:   true,
		ToUserType:          1,
		DateTimeStart:       "2026-07-01 00:00:00",
		DateTimeEnd:         "2026-08-01 00:00:00",
		RestrictEmployeeIDs: true,
		EmployeeIDs:         []int{9, 10},
	})

	for _, fragment := range []string{
		"wm.to_user_type = ?",
		"wm.msg_data_time >= ?",
		"wm.msg_data_time < ?",
		"wm.work_employee_id IN (?,?)",
	} {
		if count := strings.Count(query, fragment); count != 10 {
			t.Fatalf("%s predicates = %d", fragment, count)
		}
	}
	wantShardArgs := []any{7, 1, "2026-07-01 00:00:00", "2026-08-01 00:00:00", 9, 10}
	if len(args) != len(wantShardArgs)*10 {
		t.Fatalf("args = %#v", args)
	}
	for shard := 0; shard < 10; shard++ {
		got := args[shard*len(wantShardArgs) : (shard+1)*len(wantShardArgs)]
		if !reflect.DeepEqual(got, wantShardArgs) {
			t.Fatalf("shard %d args = %#v", shard+1, got)
		}
	}
}

func TestWorkMessageArchiveSourcePushesIdentifiersIntoShardQueries(t *testing.T) {
	query, args, where, whereArgs, ok := workMessageArchiveSource(7, "msg:archive-42")
	if !ok || strings.Count(query, "wm.msgid = ?") != 10 || where != "1 = 1" || len(whereArgs) != 0 {
		t.Fatalf("message source ok=%v where=%q query=%q", ok, where, query)
	}
	if len(args) != 20 {
		t.Fatalf("message args = %#v", args)
	}
	for shard := 0; shard < 10; shard++ {
		if !reflect.DeepEqual(args[shard*2:(shard+1)*2], []any{7, "archive-42"}) {
			t.Fatalf("message shard %d args = %#v", shard+1, args[shard*2:(shard+1)*2])
		}
	}

	query, args, where, whereArgs, ok = workMessageArchiveSource(7, "seq:102")
	if !ok || !strings.Contains(query, "FROM mc_work_message_2 wm") || strings.Contains(query, "UNION ALL") || !reflect.DeepEqual(args, []any{7, int64(102)}) || where != "1 = 1" || len(whereArgs) != 0 {
		t.Fatalf("seq source ok=%v args=%#v where=%q query=%q", ok, args, where, query)
	}

	query, args, where, whereArgs, ok = workMessageArchiveSource(7, "table:3:99")
	if !ok || !strings.Contains(query, "FROM mc_work_message_3 wm") || !strings.Contains(query, "wm.id = ?") || !reflect.DeepEqual(args, []any{7, 99}) || where != "1 = 1" || len(whereArgs) != 0 {
		t.Fatalf("table source ok=%v args=%#v where=%q query=%q", ok, args, where, query)
	}
}

func TestWorkMessageConversationGroupingKeepsEmployeesSeparate(t *testing.T) {
	columns, key := workMessageConversationGrouping()

	if columns != "work_employee_id, to_user_type, to_user_id" {
		t.Fatalf("columns = %q", columns)
	}
	if key != "CONCAT(wm.work_employee_id, ':', wm.to_user_type, ':', wm.to_user_id)" {
		t.Fatalf("key = %q", key)
	}
}

func TestWorkMessagePageWindowReadsLatestWithoutLargeOffset(t *testing.T) {
	order, offset, reverse := workMessagePageWindow(dashboard.WorkMessageFilter{
		Page: 1, PerPage: 200, Latest: true,
	})
	if order != "msg_data_time DESC, seq DESC, table_index DESC, id DESC" || offset != 0 || !reverse {
		t.Fatalf("latest window = order %q, offset %d, reverse %v", order, offset, reverse)
	}

	order, offset, reverse = workMessagePageWindow(dashboard.WorkMessageFilter{
		Page: 2, PerPage: 200,
	})
	if order != "msg_data_time ASC, seq ASC, table_index ASC, id ASC" || offset != 200 || reverse {
		t.Fatalf("paged window = order %q, offset %d, reverse %v", order, offset, reverse)
	}
}

func TestReverseWorkMessageItemsRestoresChronologicalDisplay(t *testing.T) {
	items := []dashboard.WorkMessageItem{{ID: 3}, {ID: 2}, {ID: 1}}
	reverseWorkMessageItems(items)
	if items[0].ID != 1 || items[1].ID != 2 || items[2].ID != 3 {
		t.Fatalf("items = %#v", items)
	}
}

func TestIntegrationWorkMessageQueriesEnforceScopeFiltersPaginationAndArchiveLookup(t *testing.T) {
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

	fixture := seedWorkMessageIntegrationFixture(t, db)
	store := NewMySQLStore(db)
	ctx := context.Background()

	assertWorkMessageArchiveAuthorization(t, ctx, store, fixture)
	assertWorkMessageConversationFilters(t, ctx, store, fixture)
	assertWorkMessageEmployeeScopes(t, ctx, store, fixture)
	assertWorkMessageArchiveLookup(t, ctx, store, fixture)
	assertWorkMessageExplainPlans(t, db, fixture)
}

type workMessageIntegrationFixture struct {
	tenantA    int
	tenantB    int
	corpA      int
	corpB      int
	corpOff    int
	employeeA1 int
	employeeA2 int
	employeeB1 int
	messageA1  int
	ids        map[string][]int
}

func seedWorkMessageIntegrationFixture(t *testing.T, db *sql.DB) *workMessageIntegrationFixture {
	t.Helper()
	f := &workMessageIntegrationFixture{ids: map[string][]int{}}
	t.Cleanup(func() { f.cleanup(t, db) })
	prefix := fmt.Sprintf("task4_work_message_%d", time.Now().UnixNano())
	f.tenantA = f.insert(t, db, "mc_tenant", `INSERT INTO mc_tenant (name, status) VALUES (?, 1)`, prefix+"_tenant_a")
	f.tenantB = f.insert(t, db, "mc_tenant", `INSERT INTO mc_tenant (name, status) VALUES (?, 1)`, prefix+"_tenant_b")
	f.corpA = f.insert(t, db, "mc_corp", `INSERT INTO mc_corp (name, wx_corpid, tenant_id, chat_status, created_at, updated_at) VALUES (?, ?, ?, 1, NOW(), NOW())`, prefix+"_corp_a", prefix+"_wx_a", f.tenantA)
	f.corpB = f.insert(t, db, "mc_corp", `INSERT INTO mc_corp (name, wx_corpid, tenant_id, chat_status, created_at, updated_at) VALUES (?, ?, ?, 1, NOW(), NOW())`, prefix+"_corp_b", prefix+"_wx_b", f.tenantB)
	f.corpOff = f.insert(t, db, "mc_corp", `INSERT INTO mc_corp (name, wx_corpid, tenant_id, chat_status, created_at, updated_at) VALUES (?, ?, ?, 0, NOW(), NOW())`, prefix+"_corp_off", prefix+"_wx_off", f.tenantA)
	f.employeeA1 = f.insert(t, db, "mc_work_employee", `INSERT INTO mc_work_employee (wx_user_id, corp_id, name, status, audit_status, created_at, updated_at) VALUES (?, ?, ?, 1, 1, NOW(), NOW())`, prefix+"_employee_a1", f.corpA, prefix+"_employee_a1")
	f.employeeA2 = f.insert(t, db, "mc_work_employee", `INSERT INTO mc_work_employee (wx_user_id, corp_id, name, status, audit_status, created_at, updated_at) VALUES (?, ?, ?, 1, 1, NOW(), NOW())`, prefix+"_employee_a2", f.corpA, prefix+"_employee_a2")
	f.employeeB1 = f.insert(t, db, "mc_work_employee", `INSERT INTO mc_work_employee (wx_user_id, corp_id, name, status, audit_status, created_at, updated_at) VALUES (?, ?, ?, 1, 1, NOW(), NOW())`, prefix+"_employee_b1", f.corpB, prefix+"_employee_b1")

	f.messageA1 = f.addMessage(t, db, 1, f.corpA, "task4-a1-old", 101, f.employeeA1, 1, 501, "旧报价", "2026-07-01 10:00:00")
	f.addMessage(t, db, 2, f.corpA, "task4-a1-quote", 102, f.employeeA1, 1, 501, "请确认报价_100%", "2026-07-02 10:00:00")
	f.addMessage(t, db, 3, f.corpA, "task4-a1-boundary", 103, f.employeeA1, 1, 501, "边界报价", "2026-07-03 00:00:00")
	f.addMessage(t, db, 4, f.corpA, "task4-a2-room", 104, f.employeeA2, 2, 601, "群聊进展", "2026-07-03 11:00:00")
	f.addMessage(t, db, 5, f.corpA, "task4-a2-employee", 105, f.employeeA2, 0, 602, "内部同步", "2026-07-04 11:00:00")
	f.addMessage(t, db, 6, f.corpB, "task4-a1-quote", 102, f.employeeB1, 1, 501, "跨企业报价", "2026-07-05 11:00:00")
	f.addMessage(t, db, 7, f.corpA, "task4-a1-wildcard-decoy", 106, f.employeeA1, 1, 503, "报价X100anything", "2026-07-01 09:00:00")

	for tableIndex := 1; tableIndex <= 10; tableIndex++ {
		for index := 0; index < 128; index++ {
			f.addMessage(t, db, tableIndex, f.corpA,
				fmt.Sprintf("%s-noise-%d-%d", prefix, tableIndex, index),
				int64(10000+tableIndex*1000+index), f.employeeA1, 1, 501,
				"执行计划噪声", "2026-06-01 00:00:00")
			f.addMessage(t, db, tableIndex, f.corpB,
				fmt.Sprintf("%s-foreign-noise-%d-%d", prefix, tableIndex, index),
				int64(20000+tableIndex*1000+index), f.employeeB1, 1, 501,
				"执行计划跨企业噪声", "2026-07-02 09:00:00")
		}
		if _, err := db.Exec("ANALYZE TABLE mc_work_message_" + strconv.Itoa(tableIndex)); err != nil {
			t.Fatalf("analyze message table %d: %v", tableIndex, err)
		}
	}
	return f
}

func assertWorkMessageArchiveAuthorization(t *testing.T, ctx context.Context, store *MySQLStore, f *workMessageIntegrationFixture) {
	t.Helper()
	for _, tc := range []struct {
		name     string
		tenantID int
		corpID   int
		want     bool
	}{
		{name: "same tenant and enabled corp", tenantID: f.tenantA, corpID: f.corpA, want: true},
		{name: "cross tenant", tenantID: f.tenantA, corpID: f.corpB, want: false},
		{name: "archive disabled", tenantID: f.tenantA, corpID: f.corpOff, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.WorkMessageArchiveAuthorized(ctx, tc.tenantID, tc.corpID)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("authorized = %v, want %v", got, tc.want)
			}
		})
	}
}

func assertWorkMessageConversationFilters(t *testing.T, ctx context.Context, store *MySQLStore, f *workMessageIntegrationFixture) {
	t.Helper()
	filtered, err := store.WorkMessageToUsers(ctx, dashboard.WorkMessageUserFilter{
		CorpID: f.corpA, AllowAllEmployees: true, ToUserType: 1, Keyword: "报价_100%",
		DateTimeStart: "2026-07-01 00:00:00", DateTimeEnd: "2026-07-03 00:00:00",
		RestrictEmployeeIDs: true, EmployeeIDs: []int{f.employeeA1}, Page: 1, PerPage: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].MsgID != "task4-a1-quote" {
		t.Fatalf("filtered page = %#v", filtered)
	}

	pageOne, err := store.WorkMessageToUsers(ctx, dashboard.WorkMessageUserFilter{
		CorpID: f.corpA, AllowAllEmployees: true, ToUserType: -1, Page: 1, PerPage: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	pageTwo, err := store.WorkMessageToUsers(ctx, dashboard.WorkMessageUserFilter{
		CorpID: f.corpA, AllowAllEmployees: true, ToUserType: -1, Page: 2, PerPage: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pageOne.Total != 4 || pageOne.TotalPage != 4 || len(pageOne.Items) != 1 || pageOne.Items[0].MsgID != "task4-a2-employee" {
		t.Fatalf("page one = %#v", pageOne)
	}
	if len(pageTwo.Items) != 1 || pageTwo.Items[0].MsgID != "task4-a2-room" {
		t.Fatalf("page two = %#v", pageTwo)
	}
}

func assertWorkMessageEmployeeScopes(t *testing.T, ctx context.Context, store *MySQLStore, f *workMessageIntegrationFixture) {
	t.Helper()
	for _, employeeIDs := range [][]int{{}, {f.employeeB1}} {
		page, err := store.WorkMessageToUsers(ctx, dashboard.WorkMessageUserFilter{
			CorpID: f.corpA, AllowAllEmployees: true, ToUserType: -1,
			RestrictEmployeeIDs: true, EmployeeIDs: employeeIDs, Page: 1, PerPage: 20,
		})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 0 || len(page.Items) != 0 {
			t.Fatalf("employee scope %v leaked data: %#v", employeeIDs, page)
		}
	}
}

func assertWorkMessageArchiveLookup(t *testing.T, ctx context.Context, store *MySQLStore, f *workMessageIntegrationFixture) {
	t.Helper()
	item, found, err := store.WorkMessageByArchiveID(ctx, dashboard.WorkMessageArchiveFilter{
		CorpID: f.corpA, ArchiveMessageID: "msg:task4-a1-quote",
		RestrictEmployeeIDs: true, EmployeeIDs: []int{f.employeeA1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found || item.MsgID != "task4-a1-quote" || item.WorkEmployeeID != f.employeeA1 {
		t.Fatalf("archive item = %#v, found=%v", item, found)
	}
	if _, found, err = store.WorkMessageByArchiveID(ctx, dashboard.WorkMessageArchiveFilter{
		CorpID: f.corpA, ArchiveMessageID: "msg:task4-a1-quote",
		RestrictEmployeeIDs: true, EmployeeIDs: []int{f.employeeA2},
	}); err != nil || found {
		t.Fatalf("restricted archive found=%v err=%v", found, err)
	}
	byTable, found, err := store.WorkMessageByArchiveID(ctx, dashboard.WorkMessageArchiveFilter{
		CorpID: f.corpA, ArchiveMessageID: fmt.Sprintf("table:1:%d", f.messageA1),
	})
	if err != nil || !found || byTable.ID != f.messageA1 {
		t.Fatalf("table archive = %#v, found=%v err=%v", byTable, found, err)
	}
}

func assertWorkMessageExplainPlans(t *testing.T, db *sql.DB, f *workMessageIntegrationFixture) {
	t.Helper()
	assertWorkMessageRequiredIndexes(t, db)
	filter := dashboard.WorkMessageUserFilter{
		CorpID:            f.corpA,
		AllowAllEmployees: true, ToUserType: 1, Keyword: "报价_100%",
		DateTimeStart: "2026-07-01 00:00:00", DateTimeEnd: "2026-07-03 00:00:00",
		RestrictEmployeeIDs: true, EmployeeIDs: []int{f.employeeA1},
	}
	query, args := workMessageFilteredUnionSQL(filter)
	where, whereArgs := workMessageUserWhere(filter)
	rows, err := db.Query("EXPLAIN SELECT * FROM ("+query+") wm WHERE "+where, append(args, whereArgs...)...)
	if err != nil {
		t.Fatalf("EXPLAIN work message list: %v", err)
	}
	assertWorkMessageExplainRows(t, rows, "list")

	sourceSQL, sourceArgs, idWhere, idArgs, ok := workMessageArchiveSource(f.corpA, "msg:task4-a1-quote")
	if !ok {
		t.Fatal("archive source rejected valid message ID")
	}
	rows, err = db.Query("EXPLAIN SELECT * FROM ("+sourceSQL+") wm WHERE "+idWhere, append(sourceArgs, idArgs...)...)
	if err != nil {
		t.Fatalf("EXPLAIN work message detail: %v", err)
	}
	assertWorkMessageExplainRows(t, rows, "detail")
}

func assertWorkMessageRequiredIndexes(t *testing.T, db *sql.DB) {
	t.Helper()
	requiredPrefixes := []string{"corp_id,work_employee_id", "corp_id,msgid", "corp_id,seq"}
	for tableIndex := 1; tableIndex <= dashboard.WorkMessageArchiveMessageTableCount; tableIndex++ {
		table := "mc_work_message_" + strconv.Itoa(tableIndex)
		rows, err := db.Query(`
			SELECT index_name, GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')
			FROM information_schema.statistics
			WHERE table_schema = DATABASE() AND table_name = ?
			GROUP BY index_name
		`, table)
		if err != nil {
			t.Fatalf("read indexes for %s: %v", table, err)
		}
		indexColumns := make([]string, 0)
		for rows.Next() {
			var name string
			var columns string
			if err := rows.Scan(&name, &columns); err != nil {
				rows.Close()
				t.Fatalf("scan indexes for %s: %v", table, err)
			}
			indexColumns = append(indexColumns, columns)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatalf("iterate indexes for %s: %v", table, err)
		}
		rows.Close()
		for _, prefix := range requiredPrefixes {
			found := false
			for _, columns := range indexColumns {
				if columns == prefix || strings.HasPrefix(columns, prefix+",") {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s is missing required index prefix %q; indexes=%v", table, prefix, indexColumns)
			}
		}
	}
}

func assertWorkMessageExplainRows(t *testing.T, rows *sql.Rows, planName string) {
	t.Helper()
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	tableColumn, typeColumn, keyColumn, rowsColumn := -1, -1, -1, -1
	for index, name := range columns {
		switch strings.ToLower(name) {
		case "table":
			tableColumn = index
		case "type":
			typeColumn = index
		case "key":
			keyColumn = index
		case "rows":
			rowsColumn = index
		}
	}
	if tableColumn < 0 || typeColumn < 0 || keyColumn < 0 || rowsColumn < 0 {
		t.Fatalf("unexpected EXPLAIN columns: %v", columns)
	}
	messagePlanRows := 0
	for rows.Next() {
		values := make([]sql.RawBytes, len(columns))
		dest := make([]any, len(columns))
		for index := range values {
			dest[index] = &values[index]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		table := string(values[tableColumn])
		accessType := string(values[typeColumn])
		key := string(values[keyColumn])
		estimatedRows, parseErr := strconv.Atoi(string(values[rowsColumn]))
		if table != "wm" {
			continue
		}
		if parseErr != nil {
			t.Fatalf("EXPLAIN %s has invalid row estimate %q", planName, string(values[rowsColumn]))
		}
		messagePlanRows++
		t.Logf("EXPLAIN %s source=%d type=%s key=%s rows=%d", planName, messagePlanRows, accessType, key, estimatedRows)
		usesFullScan := strings.EqualFold(accessType, "ALL")
		if estimatedRows > 512 || (!usesFullScan && key == "") {
			t.Fatalf("unacceptable message scan plan=%s source=%d type=%s key=%s rows=%d", planName, messagePlanRows, accessType, key, estimatedRows)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if messagePlanRows != 10 {
		t.Fatalf("EXPLAIN covered %d message sources, want 10", messagePlanRows)
	}
}

func (f *workMessageIntegrationFixture) insert(t *testing.T, db *sql.DB, table string, query string, args ...any) int {
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

func (f *workMessageIntegrationFixture) addMessage(t *testing.T, db *sql.DB, tableIndex int, corpID int, msgID string, seq int64, employeeID int, toUserType int, toUserID int, content string, sentAt string) int {
	t.Helper()
	table := "mc_work_message_" + strconv.Itoa(tableIndex)
	return f.insert(t, db, table, "INSERT INTO "+table+` (corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id, sender_type, action, type, msg_type, content, content_text, status, msg_data_time, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, 0, 1, 1, JSON_OBJECT('content', ?), ?, 1, ?, ?, ?)`, corpID, msgID, seq, employeeID, toUserType, toUserID, content, content, sentAt, sentAt, sentAt)
}

func (f *workMessageIntegrationFixture) cleanup(t *testing.T, db *sql.DB) {
	t.Helper()
	for tableIndex := 10; tableIndex >= 1; tableIndex-- {
		f.deleteIDs(t, db, "mc_work_message_"+strconv.Itoa(tableIndex))
	}
	for _, table := range []string{"mc_work_employee", "mc_corp", "mc_tenant"} {
		f.deleteIDs(t, db, table)
	}
}

func (f *workMessageIntegrationFixture) deleteIDs(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	ids := f.ids[table]
	if len(ids) == 0 {
		return
	}
	args := make([]any, len(ids))
	for index, id := range ids {
		args[index] = id
	}
	if _, err := db.Exec("DELETE FROM "+table+" WHERE id IN ("+placeholders(len(ids))+")", args...); err != nil {
		t.Errorf("cleanup %s: %v", table, err)
	}
}
