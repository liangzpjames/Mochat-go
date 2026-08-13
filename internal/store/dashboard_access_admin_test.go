package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

type fakeDashboardAccessAdminResult struct{ id, affected int64 }

func (result fakeDashboardAccessAdminResult) LastInsertId() (int64, error) { return result.id, nil }
func (result fakeDashboardAccessAdminResult) RowsAffected() (int64, error) {
	return result.affected, nil
}

type fakeDashboardAccessAdminTx struct {
	actorRow  func(string, ...any) dashboardAccessAdminRow
	row       func(string, ...any) dashboardAccessAdminRow
	rows      func(string, ...any) (dashboardAccessRows, error)
	exec      func(string, ...any) (sql.Result, error)
	queries   []string
	queryArgs [][]any
	execs     []string
	execArgs  [][]any
	commits   int
	rollbacks int
}

func (tx *fakeDashboardAccessAdminTx) QueryRowContext(_ context.Context, query string, args ...any) dashboardAccessAdminRow {
	tx.queries = append(tx.queries, query)
	tx.queryArgs = append(tx.queryArgs, append([]any(nil), args...))
	if strings.Contains(query, "dashboard_access_actor_id") {
		if tx.actorRow != nil {
			return tx.actorRow(query, args...)
		}
		return fakeDashboardTenantAccessRow{values: []any{args[1]}}
	}
	return tx.row(query, args...)
}

func (tx *fakeDashboardAccessAdminTx) QueryContext(_ context.Context, query string, args ...any) (dashboardAccessRows, error) {
	tx.queries = append(tx.queries, query)
	tx.queryArgs = append(tx.queryArgs, append([]any(nil), args...))
	if tx.rows == nil {
		return &fakeDashboardAccessRows{}, nil
	}
	return tx.rows(query, args...)
}

func (tx *fakeDashboardAccessAdminTx) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	tx.execs = append(tx.execs, query)
	tx.execArgs = append(tx.execArgs, append([]any(nil), args...))
	if tx.exec != nil {
		return tx.exec(query, args...)
	}
	return fakeDashboardAccessAdminResult{id: 8, affected: 1}, nil
}

func (tx *fakeDashboardAccessAdminTx) Commit() error   { tx.commits++; return nil }
func (tx *fakeDashboardAccessAdminTx) Rollback() error { tx.rollbacks++; return nil }

func dashboardAccessAdminStoreWithTx(tx *fakeDashboardAccessAdminTx) *MySQLStore {
	return &MySQLStore{dashboardAccessAdminBegin: func(context.Context) (dashboardAccessAdminTx, error) { return tx, nil }}
}

func TestDashboardEmployeeAccountsUseActiveTenantCorpBindingStatus(t *testing.T) {
	if dashboardAccessActiveBindingStatus != 2 {
		t.Fatalf("employee account binding status = %d, want active status 2", dashboardAccessActiveBindingStatus)
	}
}

func TestDashboardAccessRolesLoadsPermissionsForEveryListedRole(t *testing.T) {
	queries := make([]string, 0, 2)
	queryArgs := make([][]any, 0, 2)
	store := &MySQLStore{
		dashboardAccessQueryRow: func(_ context.Context, query string, args ...any) dashboardTenantAccessRow {
			if !strings.Contains(query, "COUNT(*) FROM mc_rbac_role") || !reflect.DeepEqual(args, []any{9}) {
				return fakeDashboardTenantAccessRow{err: errors.New("unexpected count query")}
			}
			return fakeDashboardTenantAccessRow{values: []any{2}}
		},
		dashboardAccessQuery: func(_ context.Context, query string, args ...any) (dashboardAccessRows, error) {
			queries = append(queries, query)
			queryArgs = append(queryArgs, append([]any(nil), args...))
			switch {
			case strings.Contains(query, "FROM mc_rbac_role role"):
				return &fakeDashboardAccessRows{rows: [][]any{
					{8, 9, "销售", "普通角色", 1, uint64(4), 2},
					{7, 9, "空权限", "普通角色", 2, uint64(3), 0},
				}}, nil
			case strings.Contains(query, "mochat_go_dashboard_role_permissions"):
				return &fakeDashboardAccessRows{rows: [][]any{
					{8, "dashboard.index", dashboard.DataScopeDepartment},
					{8, "dashboard.chat.v2_all", dashboard.DataScopeTenant},
				}}, nil
			default:
				return nil, errors.New("unexpected roles query")
			}
		},
	}

	page, err := store.DashboardAccessRoles(context.Background(), 9, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.List) != 2 || len(page.List[0].Permissions) != 2 {
		t.Fatalf("roles=%+v", page.List)
	}
	if page.List[0].Permissions[0].Code != "dashboard.index" || page.List[0].Permissions[0].Scope != dashboard.DataScopeDepartment {
		t.Fatalf("first role permissions=%+v", page.List[0].Permissions)
	}
	if page.List[1].Permissions == nil || len(page.List[1].Permissions) != 0 {
		t.Fatalf("empty role permissions must encode as []: %#v", page.List[1].Permissions)
	}
	if len(queries) != 2 || !strings.Contains(queries[1], "relation.role_id IN (?,?)") {
		t.Fatalf("queries=%v", queries)
	}
	if !reflect.DeepEqual(queryArgs[1], []any{9, 8, 7}) {
		t.Fatalf("permission args=%v", queryArgs[1])
	}
}

func TestDashboardAccessAdminWritesRevalidateActorBeforeTargetOrMutation(t *testing.T) {
	writes := []struct {
		name string
		run  func(*MySQLStore) error
	}{
		{name: "replace user access", run: func(store *MySQLStore) error {
			_, err := store.ReplaceUserDashboardAccess(context.Background(), dashboard.ReplaceUserDashboardAccessCommand{TenantID: 9, ActorUserID: 1, TargetUserID: 7, ExpectedVersion: 3})
			return err
		}},
		{name: "create role", run: func(store *MySQLStore) error {
			_, err := store.CreateDashboardRole(context.Background(), dashboard.CreateDashboardRoleCommand{TenantID: 9, ActorUserID: 1, Name: "销售", Status: 1})
			return err
		}},
		{name: "update role", run: func(store *MySQLStore) error {
			_, err := store.UpdateDashboardRole(context.Background(), dashboard.UpdateDashboardRoleCommand{TenantID: 9, ActorUserID: 1, RoleID: 8, Name: "销售", ExpectedVersion: 4})
			return err
		}},
		{name: "update role status", run: func(store *MySQLStore) error {
			_, err := store.UpdateDashboardRoleStatus(context.Background(), dashboard.UpdateDashboardRoleStatusCommand{TenantID: 9, ActorUserID: 1, RoleID: 8, Status: 2, ExpectedVersion: 4})
			return err
		}},
		{name: "delete role", run: func(store *MySQLStore) error {
			return store.DeleteDashboardRole(context.Background(), dashboard.DeleteDashboardRoleCommand{TenantID: 9, ActorUserID: 1, RoleID: 8, ExpectedVersion: 4})
		}},
	}
	actorStates := []string{"wrong tenant", "disabled", "not superadmin"}
	for _, write := range writes {
		for _, actorState := range actorStates {
			t.Run(write.name+"/"+actorState, func(t *testing.T) {
				tx := &fakeDashboardAccessAdminTx{
					actorRow: func(string, ...any) dashboardAccessAdminRow {
						return fakeDashboardTenantAccessRow{err: sql.ErrNoRows}
					},
					row: func(string, ...any) dashboardAccessAdminRow {
						return fakeDashboardTenantAccessRow{err: errors.New("target query must not execute")}
					},
				}
				err := write.run(dashboardAccessAdminStoreWithTx(tx))
				if !errors.Is(err, dashboard.ErrDashboardAccessAdminForbidden) {
					t.Fatalf("error=%v", err)
				}
				if len(tx.queries) != 1 || len(tx.execs) != 0 || tx.commits != 0 {
					t.Fatalf("queries=%v execs=%v commits=%d", tx.queries, tx.execs, tx.commits)
				}
				query := tx.queries[0]
				for _, contract := range []string{"tenant_id=?", "id=?", "status=1", "isSuperAdmin", "deleted_at IS NULL", "FOR UPDATE"} {
					if !strings.Contains(query, contract) {
						t.Fatalf("actor query missing %q: %s", contract, query)
					}
				}
				if !reflect.DeepEqual(tx.queryArgs[0], []any{9, 1}) {
					t.Fatalf("actor args=%v", tx.queryArgs[0])
				}
			})
		}
	}
}

func TestDashboardAccessAdminStoreRejectsReservedRoleRemarksBeforeMutation(t *testing.T) {
	for _, remark := range []string{"系统预置全权限角色", "bootstrap full-access role"} {
		t.Run(remark, func(t *testing.T) {
			createTx := &fakeDashboardAccessAdminTx{row: func(string, ...any) dashboardAccessAdminRow {
				return fakeDashboardTenantAccessRow{err: errors.New("unexpected target query")}
			}}
			store := dashboardAccessAdminStoreWithTx(createTx)
			_, err := store.CreateDashboardRole(context.Background(), dashboard.CreateDashboardRoleCommand{
				TenantID: 9, ActorUserID: 1, Name: "伪装角色", Remark: remark, Status: 1,
			})
			if !errors.Is(err, dashboard.ErrDashboardAccessAdminInvalid) || len(createTx.execs) != 0 || createTx.commits != 0 {
				t.Fatalf("create error=%v execs=%v commits=%d", err, createTx.execs, createTx.commits)
			}

			updateTx := &fakeDashboardAccessAdminTx{row: func(query string, _ ...any) dashboardAccessAdminRow {
				if strings.Contains(query, "FROM mc_rbac_role") {
					return fakeDashboardTenantAccessRow{values: []any{uint64(4), "销售", "普通角色", 1}}
				}
				return fakeDashboardTenantAccessRow{err: errors.New("unexpected row query")}
			}}
			store = dashboardAccessAdminStoreWithTx(updateTx)
			_, err = store.UpdateDashboardRole(context.Background(), dashboard.UpdateDashboardRoleCommand{
				TenantID: 9, ActorUserID: 1, RoleID: 8, Name: "伪装角色", Remark: remark, ExpectedVersion: 4,
			})
			if !errors.Is(err, dashboard.ErrDashboardAccessAdminInvalid) || len(updateTx.execs) != 0 || updateTx.commits != 0 {
				t.Fatalf("update error=%v execs=%v commits=%d", err, updateTx.execs, updateTx.commits)
			}
		})
	}
}

func TestDashboardAccessAdminReplaceUserIsTenantScopedPhysicalAtomicAndAudited(t *testing.T) {
	tx := &fakeDashboardAccessAdminTx{}
	tx.row = func(query string, _ ...any) dashboardAccessAdminRow {
		switch {
		case strings.Contains(query, "FROM mc_user") && strings.Contains(query, "FOR UPDATE"):
			return fakeDashboardTenantAccessRow{values: []any{uint64(3), "目标用户", "13800000000", 1, 0}}
		case strings.Contains(query, "COUNT(*)") && strings.Contains(query, "mc_rbac_role"):
			return fakeDashboardTenantAccessRow{values: []any{1}}
		default:
			return fakeDashboardTenantAccessRow{err: errors.New("unexpected row query: " + query)}
		}
	}
	tx.rows = func(query string, _ ...any) (dashboardAccessRows, error) {
		switch {
		case strings.Contains(query, "mochat_go_dashboard_permissions"):
			return &fakeDashboardAccessRows{rows: [][]any{{int64(1), "dashboard.index", 0}}}, nil
		case strings.Contains(query, "mochat_go_dashboard_user_roles") || strings.Contains(query, "mochat_go_dashboard_user_permissions"):
			return &fakeDashboardAccessRows{}, nil
		default:
			return nil, errors.New("unexpected rows query: " + query)
		}
	}
	store := dashboardAccessAdminStoreWithTx(tx)
	result, err := store.ReplaceUserDashboardAccess(context.Background(), dashboard.ReplaceUserDashboardAccessCommand{
		TenantID: 9, ActorUserID: 1, ActorName: "管理员", TargetUserID: 7, RoleIDs: []int{8},
		DirectPermissions: []dashboard.DashboardPermissionAssignment{{Code: "dashboard.index", Scope: dashboard.DataScopeSelf}},
		ExpectedVersion:   3, RequestID: "request-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != 4 || tx.commits != 1 {
		t.Fatalf("result=%+v commits=%d", result, tx.commits)
	}
	for _, contract := range []string{
		"DELETE FROM mochat_go_dashboard_user_roles", "INSERT INTO mochat_go_dashboard_user_roles",
		"DELETE FROM mochat_go_dashboard_user_permissions", "INSERT INTO mochat_go_dashboard_user_permissions",
		"dashboard_access_version = dashboard_access_version + 1", "INSERT INTO mochat_go_dashboard_permission_audits",
	} {
		if !containsSQL(tx.execs, contract) {
			t.Fatalf("missing exec %q: %v", contract, tx.execs)
		}
	}
	if !queryWithArgs(tx.queries, tx.queryArgs, "FROM mc_user", []any{9, 7}) {
		t.Fatalf("user lock is not tenant scoped: queries=%v args=%v", tx.queries, tx.queryArgs)
	}
	if !containsAuditActor(tx.execs, tx.execArgs, 9, 1) {
		t.Fatalf("audit did not use authenticated tenant/actor: %v", tx.execArgs)
	}
}

func TestDashboardAccessAdminUserConflictAndCrossTenantHaveZeroWrites(t *testing.T) {
	for _, test := range []struct {
		name string
		row  dashboardAccessAdminRow
		want error
	}{
		{name: "cross tenant target", row: fakeDashboardTenantAccessRow{err: sql.ErrNoRows}, want: dashboard.ErrDashboardAccessAdminNotFound},
		{name: "version conflict", row: fakeDashboardTenantAccessRow{values: []any{uint64(4), "目标用户", "", 1, 0}}, want: dashboard.ErrDashboardAccessAdminConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx := &fakeDashboardAccessAdminTx{row: func(string, ...any) dashboardAccessAdminRow { return test.row }}
			store := dashboardAccessAdminStoreWithTx(tx)
			_, err := store.ReplaceUserDashboardAccess(context.Background(), dashboard.ReplaceUserDashboardAccessCommand{TenantID: 9, ActorUserID: 1, TargetUserID: 7, ExpectedVersion: 3})
			if !errors.Is(err, test.want) || len(tx.execs) != 0 || tx.commits != 0 {
				t.Fatalf("error=%v execs=%d commits=%d", err, len(tx.execs), tx.commits)
			}
		})
	}
}

func TestDashboardAccessAdminAuditFailureRollsBackWholeUserWrite(t *testing.T) {
	tx := successfulDashboardAccessAdminUserTx()
	tx.exec = func(query string, _ ...any) (sql.Result, error) {
		if strings.Contains(query, "permission_audits") {
			return nil, errors.New("audit unavailable")
		}
		return driver.RowsAffected(1), nil
	}
	store := dashboardAccessAdminStoreWithTx(tx)
	_, err := store.ReplaceUserDashboardAccess(context.Background(), dashboard.ReplaceUserDashboardAccessCommand{TenantID: 9, ActorUserID: 1, TargetUserID: 7, ExpectedVersion: 3})
	if err == nil || tx.commits != 0 || tx.rollbacks == 0 {
		t.Fatalf("error=%v commits=%d rollbacks=%d", err, tx.commits, tx.rollbacks)
	}
}

func TestDashboardAccessAdminRoleDeleteChecksMembersBeforeDelete(t *testing.T) {
	tx := &fakeDashboardAccessAdminTx{}
	tx.row = func(query string, _ ...any) dashboardAccessAdminRow {
		switch {
		case strings.Contains(query, "FROM mc_rbac_role") && strings.Contains(query, "FOR UPDATE"):
			return fakeDashboardTenantAccessRow{values: []any{uint64(4), "销售", "普通角色", 1}}
		case strings.Contains(query, "COUNT(*)") && strings.Contains(query, "user_roles"):
			return fakeDashboardTenantAccessRow{values: []any{2}}
		default:
			return fakeDashboardTenantAccessRow{err: errors.New("unexpected row query")}
		}
	}
	store := dashboardAccessAdminStoreWithTx(tx)
	err := store.DeleteDashboardRole(context.Background(), dashboard.DeleteDashboardRoleCommand{TenantID: 9, ActorUserID: 1, RoleID: 8, ExpectedVersion: 4})
	if !errors.Is(err, dashboard.ErrDashboardAccessRoleHasMembers) || len(tx.execs) != 0 || tx.commits != 0 {
		t.Fatalf("error=%v execs=%d commits=%d", err, len(tx.execs), tx.commits)
	}
	if !queryWithArgs(tx.queries, tx.queryArgs, "FROM mc_rbac_role", []any{9, 8}) || !queryWithArgs(tx.queries, tx.queryArgs, "user_roles", []any{9, 8}) {
		t.Fatalf("queries=%v args=%v", tx.queries, tx.queryArgs)
	}
}

func TestDashboardAccessAdminRoleWriteRejectsSuperadminPermissionInsideTransaction(t *testing.T) {
	tx := &fakeDashboardAccessAdminTx{}
	tx.row = func(query string, _ ...any) dashboardAccessAdminRow {
		if strings.Contains(query, "FROM mc_rbac_role") {
			return fakeDashboardTenantAccessRow{values: []any{uint64(4), "销售", "普通角色", 1}}
		}
		return fakeDashboardTenantAccessRow{err: errors.New("unexpected row query")}
	}
	tx.rows = func(query string, _ ...any) (dashboardAccessRows, error) {
		if strings.Contains(query, "mochat_go_dashboard_permissions") {
			return &fakeDashboardAccessRows{rows: [][]any{{int64(50), "dashboard.company_setting.staff", 1}}}, nil
		}
		return &fakeDashboardAccessRows{}, nil
	}
	store := dashboardAccessAdminStoreWithTx(tx)
	_, err := store.UpdateDashboardRole(context.Background(), dashboard.UpdateDashboardRoleCommand{
		TenantID: 9, ActorUserID: 1, RoleID: 8, Name: "销售", ExpectedVersion: 4,
		Permissions: []dashboard.DashboardPermissionAssignment{{Code: "dashboard.company_setting.staff", Scope: dashboard.DataScopeTenant}},
	})
	if !errors.Is(err, dashboard.ErrDashboardAccessAdminInvalid) || len(tx.execs) != 0 {
		t.Fatalf("error=%v execs=%d", err, len(tx.execs))
	}
}

func TestDashboardAccessAdminRoleUpdateIsVersionedPhysicalAndAudited(t *testing.T) {
	tx := &fakeDashboardAccessAdminTx{}
	tx.row = func(query string, _ ...any) dashboardAccessAdminRow {
		if strings.Contains(query, "FROM mc_rbac_role") {
			return fakeDashboardTenantAccessRow{values: []any{uint64(4), "销售", "普通角色", 1}}
		}
		return fakeDashboardTenantAccessRow{err: errors.New("unexpected row query")}
	}
	tx.rows = func(query string, _ ...any) (dashboardAccessRows, error) {
		switch {
		case strings.Contains(query, "permission.code IN"):
			return &fakeDashboardAccessRows{rows: [][]any{{int64(1), "dashboard.index", 0}}}, nil
		case strings.Contains(query, "role_permissions"):
			return &fakeDashboardAccessRows{rows: [][]any{{"dashboard.old", dashboard.DataScopeSelf}}}, nil
		default:
			return nil, errors.New("unexpected rows query")
		}
	}
	store := dashboardAccessAdminStoreWithTx(tx)
	role, err := store.UpdateDashboardRole(context.Background(), dashboard.UpdateDashboardRoleCommand{
		TenantID: 9, ActorUserID: 1, ActorName: "管理员", RoleID: 8, Name: "销售组", Remark: "新备注",
		Permissions:     []dashboard.DashboardPermissionAssignment{{Code: "dashboard.index", Scope: dashboard.DataScopeDepartment}},
		ExpectedVersion: 4, RequestID: "request-role",
	})
	if err != nil {
		t.Fatal(err)
	}
	if role.Version != 5 || tx.commits != 1 {
		t.Fatalf("role=%+v commits=%d", role, tx.commits)
	}
	for _, contract := range []string{
		"dashboard_access_version=dashboard_access_version+1",
		"DELETE FROM mochat_go_dashboard_role_permissions",
		"INSERT INTO mochat_go_dashboard_role_permissions",
		"INSERT INTO mochat_go_dashboard_permission_audits",
	} {
		if !containsSQL(tx.execs, contract) {
			t.Fatalf("missing exec %q: %v", contract, tx.execs)
		}
	}
	if !queryWithArgs(tx.queries, tx.queryArgs, "FROM mc_rbac_role", []any{9, 8}) {
		t.Fatalf("role lock not tenant scoped: queries=%v args=%v", tx.queries, tx.queryArgs)
	}
}

func TestDashboardAccessAdminRoleCrossTenantAndVersionConflictHaveZeroWrites(t *testing.T) {
	for _, test := range []struct {
		name string
		row  dashboardAccessAdminRow
		want error
	}{
		{name: "cross tenant target", row: fakeDashboardTenantAccessRow{err: sql.ErrNoRows}, want: dashboard.ErrDashboardAccessAdminNotFound},
		{name: "version conflict", row: fakeDashboardTenantAccessRow{values: []any{uint64(5), "销售", "普通角色", 1}}, want: dashboard.ErrDashboardAccessAdminConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx := &fakeDashboardAccessAdminTx{row: func(string, ...any) dashboardAccessAdminRow { return test.row }}
			store := dashboardAccessAdminStoreWithTx(tx)
			_, err := store.UpdateDashboardRole(context.Background(), dashboard.UpdateDashboardRoleCommand{TenantID: 9, ActorUserID: 1, RoleID: 8, Name: "销售", ExpectedVersion: 4})
			if !errors.Is(err, test.want) || len(tx.execs) != 0 || tx.commits != 0 {
				t.Fatalf("error=%v execs=%d commits=%d", err, len(tx.execs), tx.commits)
			}
		})
	}
}

func TestDashboardAccessAdminRoleAuditFailureRollsBackMetadataAndPermissions(t *testing.T) {
	tx := &fakeDashboardAccessAdminTx{}
	tx.row = func(string, ...any) dashboardAccessAdminRow {
		return fakeDashboardTenantAccessRow{values: []any{uint64(4), "销售", "普通角色", 1}}
	}
	tx.rows = func(query string, _ ...any) (dashboardAccessRows, error) {
		if strings.Contains(query, "permission.code IN") {
			return &fakeDashboardAccessRows{rows: [][]any{{int64(1), "dashboard.index", 0}}}, nil
		}
		return &fakeDashboardAccessRows{}, nil
	}
	tx.exec = func(query string, _ ...any) (sql.Result, error) {
		if strings.Contains(query, "permission_audits") {
			return nil, errors.New("audit unavailable")
		}
		return driver.RowsAffected(1), nil
	}
	store := dashboardAccessAdminStoreWithTx(tx)
	_, err := store.UpdateDashboardRole(context.Background(), dashboard.UpdateDashboardRoleCommand{
		TenantID: 9, ActorUserID: 1, RoleID: 8, Name: "销售", ExpectedVersion: 4,
		Permissions: []dashboard.DashboardPermissionAssignment{{Code: "dashboard.index", Scope: dashboard.DataScopeSelf}},
	})
	if err == nil || tx.commits != 0 || tx.rollbacks == 0 {
		t.Fatalf("error=%v commits=%d rollbacks=%d", err, tx.commits, tx.rollbacks)
	}
}

func successfulDashboardAccessAdminUserTx() *fakeDashboardAccessAdminTx {
	tx := &fakeDashboardAccessAdminTx{}
	tx.row = func(query string, _ ...any) dashboardAccessAdminRow {
		if strings.Contains(query, "FROM mc_user") {
			return fakeDashboardTenantAccessRow{values: []any{uint64(3), "目标用户", "", 1, 0}}
		}
		return fakeDashboardTenantAccessRow{err: errors.New("unexpected row query")}
	}
	tx.rows = func(string, ...any) (dashboardAccessRows, error) { return &fakeDashboardAccessRows{}, nil }
	return tx
}

func containsSQL(queries []string, contract string) bool {
	for _, query := range queries {
		if strings.Contains(query, contract) {
			return true
		}
	}
	return false
}

func queryWithArgs(queries []string, args [][]any, contract string, want []any) bool {
	for index, query := range queries {
		if strings.Contains(query, contract) && reflect.DeepEqual(args[index], want) {
			return true
		}
	}
	return false
}

func containsAuditActor(queries []string, args [][]any, tenantID, actorID int) bool {
	for index, query := range queries {
		if strings.Contains(query, "permission_audits") && len(args[index]) >= 2 && args[index][0] == tenantID && args[index][1] == actorID {
			return true
		}
	}
	return false
}
