package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) DashboardPermissionResources(ctx context.Context, method string) ([]dashboard.DashboardPermissionResource, error) {
	query := s.dashboardAccessQuery
	if query == nil && s.db != nil {
		query = func(ctx context.Context, statement string, args ...any) (dashboardAccessRows, error) {
			return s.db.QueryContext(ctx, statement, args...)
		}
	}
	if query == nil {
		return nil, errors.New("dashboard permission resource store is unavailable")
	}
	rows, err := query(ctx, `
		SELECT permission.code, resource.http_method, resource.path_pattern, resource.scope_required
		FROM mochat_go_dashboard_permission_resources resource
		INNER JOIN mochat_go_dashboard_permissions permission
			ON permission.id = resource.permission_id
			AND permission.status = 1 AND permission.deleted_at IS NULL
		WHERE resource.http_method = ? AND resource.status = 1 AND resource.deleted_at IS NULL
		ORDER BY resource.path_pattern ASC, permission.code ASC
	`, strings.ToUpper(strings.TrimSpace(method)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resources := make([]dashboard.DashboardPermissionResource, 0)
	for rows.Next() {
		var resource dashboard.DashboardPermissionResource
		var scopeRequired int
		if err := rows.Scan(&resource.PermissionCode, &resource.Method, &resource.PathPattern, &scopeRequired); err != nil {
			return nil, err
		}
		resource.ScopeRequired = scopeRequired == 1
		resources = append(resources, resource)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return resources, nil
}

type dashboardAccessRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type dashboardAccessQueryRowFunc func(ctx context.Context, query string, args ...any) dashboardTenantAccessRow
type dashboardAccessQueryFunc func(ctx context.Context, query string, args ...any) (dashboardAccessRows, error)

func (s *MySQLStore) DashboardAccessIdentity(ctx context.Context, userID int) (dashboard.DashboardAccessIdentity, bool, error) {
	queryRow := s.dashboardAccessQueryRow
	if queryRow == nil && s.db != nil {
		queryRow = func(ctx context.Context, query string, args ...any) dashboardTenantAccessRow {
			return s.db.QueryRowContext(ctx, query, args...)
		}
	}
	if queryRow == nil {
		return dashboard.DashboardAccessIdentity{}, false, errors.New("dashboard access identity store is unavailable")
	}
	var identity dashboard.DashboardAccessIdentity
	var isSuperAdmin int
	err := queryRow(ctx, `
		SELECT id, tenant_id, COALESCE(name, ''), status, COALESCE(isSuperAdmin, 0)
		FROM mc_user
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`, userID).Scan(&identity.UserID, &identity.TenantID, &identity.UserName, &identity.Status, &isSuperAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.DashboardAccessIdentity{}, false, nil
	}
	if err != nil {
		return dashboard.DashboardAccessIdentity{}, false, err
	}
	identity.IsSuperAdmin = isSuperAdmin == 1
	return identity, true, nil
}

func (s *MySQLStore) DashboardPermissionCatalog(ctx context.Context) ([]dashboard.DashboardPermissionDefinition, error) {
	query := s.dashboardAccessQuery
	if query == nil && s.db != nil {
		query = func(ctx context.Context, statement string, args ...any) (dashboardAccessRows, error) {
			return s.db.QueryContext(ctx, statement, args...)
		}
	}
	if query == nil {
		return nil, errors.New("dashboard permission catalog store is unavailable")
	}
	rows, err := query(ctx, `
		SELECT
			permission.id, permission.code, permission.path, permission.name,
			COALESCE(permission.group_code, ''), permission.sort, permission.superadmin_only,
			MAX(COALESCE(resource.scope_required, 0))
		FROM mochat_go_dashboard_permissions permission
		LEFT JOIN mochat_go_dashboard_permission_resources resource
			ON resource.permission_id = permission.id
			AND resource.status = 1 AND resource.deleted_at IS NULL
		WHERE permission.status = 1 AND permission.deleted_at IS NULL
		GROUP BY permission.id, permission.code, permission.path, permission.name,
			permission.group_code, permission.sort, permission.superadmin_only
		ORDER BY permission.sort ASC, permission.code ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	catalog := make([]dashboard.DashboardPermissionDefinition, 0, 53)
	for rows.Next() {
		var permission dashboard.DashboardPermissionDefinition
		var superadminOnly, scopeRequired int
		if err := rows.Scan(
			&permission.ID, &permission.Code, &permission.Path, &permission.Name,
			&permission.GroupCode, &permission.Sort, &superadminOnly, &scopeRequired,
		); err != nil {
			return nil, err
		}
		permission.SuperadminOnly = superadminOnly == 1
		permission.ScopeRequired = scopeRequired == 1
		catalog = append(catalog, permission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return catalog, nil
}

func (s *MySQLStore) DashboardPermissionGrants(ctx context.Context, tenantID, userID int) ([]dashboard.DashboardPermissionGrantFact, error) {
	query := s.dashboardAccessQuery
	if query == nil && s.db != nil {
		query = func(ctx context.Context, statement string, args ...any) (dashboardAccessRows, error) {
			return s.db.QueryContext(ctx, statement, args...)
		}
	}
	if query == nil {
		return nil, errors.New("dashboard permission grant store is unavailable")
	}
	rows, err := query(ctx, `
		SELECT
			direct_permission.tenant_id, direct_permission.permission_id,
			'direct' AS source_type, direct_permission.user_id AS source_id,
			'直接授权' AS source_name, direct_user.status AS source_status,
			0 AS source_deleted, direct_permission.data_scope
		FROM mochat_go_dashboard_user_permissions direct_permission
		INNER JOIN mc_user direct_user
			ON direct_user.tenant_id = direct_permission.tenant_id
			AND direct_user.id = direct_permission.user_id
			AND direct_user.deleted_at IS NULL
		INNER JOIN mochat_go_dashboard_permissions permission
			ON permission.id = direct_permission.permission_id
			AND permission.status = 1 AND permission.deleted_at IS NULL
			AND permission.superadmin_only = 0
		WHERE direct_permission.tenant_id = ? AND direct_permission.user_id = ?
			AND direct_permission.effect = 'allow'
		UNION ALL
		SELECT
			user_role.tenant_id, role_permission.permission_id,
			'role' AS source_type, role.id AS source_id,
			role.name AS source_name, role.status AS source_status,
			0 AS source_deleted, role_permission.data_scope
		FROM mochat_go_dashboard_user_roles user_role
		INNER JOIN mc_rbac_role role
			ON role.tenant_id = user_role.tenant_id AND role.id = user_role.role_id
			AND role.status = 1 AND role.deleted_at IS NULL
		INNER JOIN mochat_go_dashboard_role_permissions role_permission
			ON role_permission.tenant_id = user_role.tenant_id
			AND role_permission.role_id = user_role.role_id
		INNER JOIN mochat_go_dashboard_permissions permission
			ON permission.id = role_permission.permission_id
			AND permission.status = 1 AND permission.deleted_at IS NULL
			AND permission.superadmin_only = 0
		WHERE user_role.tenant_id = ? AND user_role.user_id = ?
		ORDER BY permission_id ASC, source_type ASC, source_id ASC
	`, tenantID, userID, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := make([]dashboard.DashboardPermissionGrantFact, 0)
	for rows.Next() {
		var grant dashboard.DashboardPermissionGrantFact
		var sourceDeleted int
		if err := rows.Scan(
			&grant.TenantID, &grant.PermissionID, &grant.SourceType, &grant.SourceID,
			&grant.SourceName, &grant.SourceStatus, &sourceDeleted, &grant.Scope,
		); err != nil {
			return nil, err
		}
		grant.SourceDeleted = sourceDeleted == 1
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return grants, nil
}

func (s *MySQLStore) DashboardEmployeeScope(ctx context.Context, tenantID, userID, corpID int) (dashboard.DashboardEmployeeScope, bool, error) {
	queryRow := s.dashboardAccessQueryRow
	if queryRow == nil && s.db != nil {
		queryRow = func(ctx context.Context, statement string, args ...any) dashboardTenantAccessRow {
			return s.db.QueryRowContext(ctx, statement, args...)
		}
	}
	query := s.dashboardAccessQuery
	if query == nil && s.db != nil {
		query = func(ctx context.Context, statement string, args ...any) (dashboardAccessRows, error) {
			return s.db.QueryContext(ctx, statement, args...)
		}
	}
	if queryRow == nil || query == nil {
		return dashboard.DashboardEmployeeScope{}, false, errors.New("dashboard employee scope store is unavailable")
	}
	var scope dashboard.DashboardEmployeeScope
	err := queryRow(ctx, `
		SELECT employee.id
		FROM mc_corp corp
		INNER JOIN mc_work_employee employee
			ON employee.corp_id = corp.id AND employee.deleted_at IS NULL
		WHERE corp.tenant_id = ? AND corp.id = ? AND employee.log_user_id = ?
			AND corp.deleted_at IS NULL
		ORDER BY employee.id ASC
		LIMIT 1
	`, tenantID, corpID, userID).Scan(&scope.EmployeeID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.DashboardEmployeeScope{}, false, nil
	}
	if err != nil {
		return dashboard.DashboardEmployeeScope{}, false, err
	}
	rows, err := query(ctx, `
		SELECT DISTINCT scoped_department.id, scoped_employee.id
		FROM mc_corp corp
		INNER JOIN mc_work_employee employee
			ON employee.corp_id = corp.id AND employee.deleted_at IS NULL
		INNER JOIN mc_work_employee_department source_membership
			ON source_membership.employee_id = employee.id AND source_membership.deleted_at IS NULL
		INNER JOIN mc_work_department source_department
			ON source_department.id = source_membership.department_id
			AND source_department.corp_id = corp.id AND source_department.deleted_at IS NULL
		INNER JOIN mc_work_department scoped_department
			ON scoped_department.corp_id = corp.id AND scoped_department.deleted_at IS NULL
			AND scoped_department.path LIKE CONCAT(source_department.path, '%')
		INNER JOIN mc_work_employee_department scoped_membership
			ON scoped_membership.department_id = scoped_department.id AND scoped_membership.deleted_at IS NULL
		INNER JOIN mc_work_employee scoped_employee
			ON scoped_employee.id = scoped_membership.employee_id
			AND scoped_employee.corp_id = corp.id AND scoped_employee.deleted_at IS NULL
		WHERE corp.tenant_id = ? AND corp.id = ? AND employee.log_user_id = ?
			AND corp.deleted_at IS NULL
		ORDER BY source_department.id ASC, scoped_employee.id ASC
	`, tenantID, corpID, userID)
	if err != nil {
		return dashboard.DashboardEmployeeScope{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var departmentID, employeeID int
		if err := rows.Scan(&departmentID, &employeeID); err != nil {
			return dashboard.DashboardEmployeeScope{}, false, err
		}
		scope.DepartmentIDs = append(scope.DepartmentIDs, departmentID)
		scope.DepartmentEmployeeIDs = append(scope.DepartmentEmployeeIDs, employeeID)
	}
	if err := rows.Err(); err != nil {
		return dashboard.DashboardEmployeeScope{}, false, err
	}
	scope.DepartmentIDs = uniquePositiveInts(scope.DepartmentIDs)
	scope.DepartmentEmployeeIDs = uniquePositiveInts(scope.DepartmentEmployeeIDs)
	if len(scope.DepartmentEmployeeIDs) == 0 && scope.EmployeeID > 0 {
		scope.DepartmentEmployeeIDs = []int{scope.EmployeeID}
	}
	return scope, true, nil
}
