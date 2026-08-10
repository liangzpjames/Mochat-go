package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

type dashboardAccessAdminRow interface{ Scan(...any) error }

type dashboardAccessAdminTx interface {
	QueryRowContext(context.Context, string, ...any) dashboardAccessAdminRow
	QueryContext(context.Context, string, ...any) (dashboardAccessRows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Commit() error
	Rollback() error
}

type dashboardAccessAdminBeginFunc func(context.Context) (dashboardAccessAdminTx, error)

type sqlDashboardAccessAdminTx struct{ tx *sql.Tx }

func (tx sqlDashboardAccessAdminTx) QueryRowContext(ctx context.Context, query string, args ...any) dashboardAccessAdminRow {
	return tx.tx.QueryRowContext(ctx, query, args...)
}
func (tx sqlDashboardAccessAdminTx) QueryContext(ctx context.Context, query string, args ...any) (dashboardAccessRows, error) {
	return tx.tx.QueryContext(ctx, query, args...)
}
func (tx sqlDashboardAccessAdminTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.tx.ExecContext(ctx, query, args...)
}
func (tx sqlDashboardAccessAdminTx) Commit() error   { return tx.tx.Commit() }
func (tx sqlDashboardAccessAdminTx) Rollback() error { return tx.tx.Rollback() }

func (s *MySQLStore) beginDashboardAccessAdmin(ctx context.Context) (dashboardAccessAdminTx, error) {
	if s.dashboardAccessAdminBegin != nil {
		return s.dashboardAccessAdminBegin(ctx)
	}
	if s.db == nil {
		return nil, errors.New("dashboard access administration store is unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return sqlDashboardAccessAdminTx{tx: tx}, nil
}

func (s *MySQLStore) DashboardAccessUsers(ctx context.Context, tenantID, page, perPage int) (dashboard.DashboardAccessUserPage, error) {
	if s.db == nil {
		return dashboard.DashboardAccessUserPage{}, errors.New("dashboard access administration store is unavailable")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_user WHERE tenant_id=? AND deleted_at IS NULL`, tenantID).Scan(&total); err != nil {
		return dashboard.DashboardAccessUserPage{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,COALESCE(name,''),COALESCE(phone,''),status,COALESCE(isSuperAdmin,0),dashboard_access_version
		FROM mc_user WHERE tenant_id=? AND deleted_at IS NULL
		ORDER BY id DESC LIMIT ? OFFSET ?
	`, tenantID, perPage, (page-1)*perPage)
	if err != nil {
		return dashboard.DashboardAccessUserPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.DashboardAccessUserSummary, 0)
	for rows.Next() {
		var item dashboard.DashboardAccessUserSummary
		var superadmin int
		if err := rows.Scan(&item.ID, &item.Name, &item.Phone, &item.Status, &superadmin, &item.Version); err != nil {
			return dashboard.DashboardAccessUserPage{}, err
		}
		item.IsSuperAdmin = superadmin == 1
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.DashboardAccessUserPage{}, err
	}
	return dashboard.DashboardAccessUserPage{List: items, Page: dashboardAdminPage(page, perPage, total)}, nil
}

func (s *MySQLStore) DashboardAccessUser(ctx context.Context, tenantID, userID int) (dashboard.DashboardAccessUserDetail, bool, error) {
	if s.db == nil {
		return dashboard.DashboardAccessUserDetail{}, false, errors.New("dashboard access administration store is unavailable")
	}
	var item dashboard.DashboardAccessUserDetail
	var superadmin int
	err := s.db.QueryRowContext(ctx, `
		SELECT id,tenant_id,COALESCE(name,''),COALESCE(phone,''),status,COALESCE(isSuperAdmin,0),dashboard_access_version
		FROM mc_user WHERE tenant_id=? AND id=? AND deleted_at IS NULL LIMIT 1
	`, tenantID, userID).Scan(&item.ID, &item.TenantID, &item.Name, &item.Phone, &item.Status, &superadmin, &item.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.DashboardAccessUserDetail{}, false, nil
	}
	if err != nil {
		return dashboard.DashboardAccessUserDetail{}, false, err
	}
	item.IsSuperAdmin = superadmin == 1
	item.Roles, err = s.dashboardAccessUserRoles(ctx, tenantID, userID)
	if err != nil {
		return dashboard.DashboardAccessUserDetail{}, false, err
	}
	item.DirectPermissions, err = s.dashboardAccessUserDirectPermissions(ctx, tenantID, userID)
	if err != nil {
		return dashboard.DashboardAccessUserDetail{}, false, err
	}
	profile, err := dashboard.NewDashboardAccessService(s).Resolve(ctx, userID, 0)
	if err != nil {
		return dashboard.DashboardAccessUserDetail{}, false, err
	}
	item.EffectivePermissions = profile.EffectivePermissions
	for _, permission := range profile.EffectivePermissions {
		for _, source := range permission.Sources {
			if source.Type == dashboard.PermissionSourceRole {
				item.InheritedPermissions = append(item.InheritedPermissions, permission)
				break
			}
		}
	}
	return item, true, nil
}

func (s *MySQLStore) dashboardAccessUserRoles(ctx context.Context, tenantID, userID int) ([]dashboard.DashboardAccessRoleSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT role.id,role.name,role.status,role.dashboard_access_version
		FROM mochat_go_dashboard_user_roles relation
		INNER JOIN mc_rbac_role role ON role.tenant_id=relation.tenant_id AND role.id=relation.role_id AND role.deleted_at IS NULL
		WHERE relation.tenant_id=? AND relation.user_id=? ORDER BY role.id ASC
	`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.DashboardAccessRoleSummary, 0)
	for rows.Next() {
		var item dashboard.DashboardAccessRoleSummary
		if err := rows.Scan(&item.ID, &item.Name, &item.Status, &item.Version); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) dashboardAccessUserDirectPermissions(ctx context.Context, tenantID, userID int) ([]dashboard.DashboardPermissionAssignment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT permission.code,relation.data_scope
		FROM mochat_go_dashboard_user_permissions relation
		INNER JOIN mochat_go_dashboard_permissions permission ON permission.id=relation.permission_id
		WHERE relation.tenant_id=? AND relation.user_id=? AND relation.effect='allow'
		ORDER BY permission.code ASC
	`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.DashboardPermissionAssignment, 0)
	for rows.Next() {
		var item dashboard.DashboardPermissionAssignment
		if err := rows.Scan(&item.Code, &item.Scope); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) DashboardAccessRoles(ctx context.Context, tenantID, page, perPage int) (dashboard.DashboardAccessRolePage, error) {
	if s.db == nil {
		return dashboard.DashboardAccessRolePage{}, errors.New("dashboard access administration store is unavailable")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_rbac_role WHERE tenant_id=? AND deleted_at IS NULL`, tenantID).Scan(&total); err != nil {
		return dashboard.DashboardAccessRolePage{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT role.id,role.tenant_id,role.name,COALESCE(role.remarks,''),role.status,role.dashboard_access_version,
			(SELECT COUNT(*) FROM mochat_go_dashboard_user_roles member WHERE member.tenant_id=role.tenant_id AND member.role_id=role.id)
		FROM mc_rbac_role role WHERE role.tenant_id=? AND role.deleted_at IS NULL
		ORDER BY role.id DESC LIMIT ? OFFSET ?
	`, tenantID, perPage, (page-1)*perPage)
	if err != nil {
		return dashboard.DashboardAccessRolePage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.DashboardAccessRoleDetail, 0)
	for rows.Next() {
		var item dashboard.DashboardAccessRoleDetail
		if err := rows.Scan(&item.ID, &item.TenantID, &item.Name, &item.Remark, &item.Status, &item.Version, &item.MemberCount); err != nil {
			return dashboard.DashboardAccessRolePage{}, err
		}
		item.IsSystem = dashboardAccessSystemRole(item.Remark)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.DashboardAccessRolePage{}, err
	}
	return dashboard.DashboardAccessRolePage{List: items, Page: dashboardAdminPage(page, perPage, total)}, nil
}

func (s *MySQLStore) DashboardAccessRole(ctx context.Context, tenantID, roleID int) (dashboard.DashboardAccessRoleDetail, bool, error) {
	if s.db == nil {
		return dashboard.DashboardAccessRoleDetail{}, false, errors.New("dashboard access administration store is unavailable")
	}
	var item dashboard.DashboardAccessRoleDetail
	err := s.db.QueryRowContext(ctx, `
		SELECT role.id,role.tenant_id,role.name,COALESCE(role.remarks,''),role.status,role.dashboard_access_version,
			(SELECT COUNT(*) FROM mochat_go_dashboard_user_roles member WHERE member.tenant_id=role.tenant_id AND member.role_id=role.id)
		FROM mc_rbac_role role WHERE role.tenant_id=? AND role.id=? AND role.deleted_at IS NULL LIMIT 1
	`, tenantID, roleID).Scan(&item.ID, &item.TenantID, &item.Name, &item.Remark, &item.Status, &item.Version, &item.MemberCount)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.DashboardAccessRoleDetail{}, false, nil
	}
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, false, err
	}
	item.IsSystem = dashboardAccessSystemRole(item.Remark)
	rows, err := s.db.QueryContext(ctx, `
		SELECT permission.code,relation.data_scope
		FROM mochat_go_dashboard_role_permissions relation
		INNER JOIN mochat_go_dashboard_permissions permission ON permission.id=relation.permission_id
		WHERE relation.tenant_id=? AND relation.role_id=? ORDER BY permission.code ASC
	`, tenantID, roleID)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var assignment dashboard.DashboardPermissionAssignment
		if err := rows.Scan(&assignment.Code, &assignment.Scope); err != nil {
			return dashboard.DashboardAccessRoleDetail{}, false, err
		}
		item.Permissions = append(item.Permissions, assignment)
	}
	return item, true, rows.Err()
}

func (s *MySQLStore) DashboardPermissionAudits(ctx context.Context, tenantID int, filter dashboard.DashboardPermissionAuditFilter) (dashboard.DashboardPermissionAuditPage, error) {
	if s.db == nil {
		return dashboard.DashboardPermissionAuditPage{}, errors.New("dashboard access administration store is unavailable")
	}
	where := []string{"audit.tenant_id=?"}
	args := []any{tenantID}
	if filter.ActorUserID > 0 {
		where, args = append(where, "audit.actor_user_id=?"), append(args, filter.ActorUserID)
	}
	for value, column := range map[string]string{filter.TargetType: "audit.target_type", filter.TargetID: "audit.target_id", filter.Action: "audit.action"} {
		if strings.TrimSpace(value) != "" {
			where, args = append(where, column+"=?"), append(args, strings.TrimSpace(value))
		}
	}
	if !filter.StartedAt.IsZero() {
		where, args = append(where, "audit.created_at>=?"), append(args, filter.StartedAt)
	}
	if !filter.EndedAt.IsZero() {
		where, args = append(where, "audit.created_at<?"), append(args, filter.EndedAt)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits audit WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return dashboard.DashboardPermissionAuditPage{}, err
	}
	queryArgs := append(append([]any(nil), args...), filter.PerPage, (filter.Page-1)*filter.PerPage)
	rows, err := s.db.QueryContext(ctx, `
		SELECT audit.id,audit.actor_user_id,COALESCE(actor.name,''),audit.action,audit.target_type,audit.target_id,
			COALESCE(audit.before_json,''),COALESCE(audit.after_json,''),audit.expected_version,audit.result_version,audit.request_id,audit.created_at
		FROM mochat_go_dashboard_permission_audits audit
		LEFT JOIN mc_user actor ON actor.tenant_id=audit.tenant_id AND actor.id=audit.actor_user_id
		WHERE `+whereSQL+` ORDER BY audit.id DESC LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return dashboard.DashboardPermissionAuditPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.DashboardPermissionAudit, 0)
	for rows.Next() {
		var item dashboard.DashboardPermissionAudit
		var actor sql.NullInt64
		var expected, result sql.NullInt64
		var beforeJSON, afterJSON []byte
		if err := rows.Scan(&item.ID, &actor, &item.ActorName, &item.Action, &item.TargetType, &item.TargetID, &beforeJSON, &afterJSON, &expected, &result, &item.RequestID, &item.CreatedAt); err != nil {
			return dashboard.DashboardPermissionAuditPage{}, err
		}
		item.BeforeJSON = append([]byte(nil), beforeJSON...)
		item.AfterJSON = append([]byte(nil), afterJSON...)
		if actor.Valid {
			value := int(actor.Int64)
			item.ActorUserID = &value
		}
		if expected.Valid {
			value := uint64(expected.Int64)
			item.ExpectedVersion = &value
		}
		if result.Valid {
			value := uint64(result.Int64)
			item.ResultVersion = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.DashboardPermissionAuditPage{}, err
	}
	return dashboard.DashboardPermissionAuditPage{List: items, Page: dashboardAdminPage(filter.Page, filter.PerPage, total)}, nil
}

func (s *MySQLStore) ReplaceUserDashboardAccess(ctx context.Context, command dashboard.ReplaceUserDashboardAccessCommand) (dashboard.DashboardAccessUserDetail, error) {
	tx, err := s.beginDashboardAccessAdmin(ctx)
	if err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	defer tx.Rollback()
	if err = validateDashboardAccessActorTx(ctx, tx, command.TenantID, command.ActorUserID); err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	var version uint64
	var name, phone string
	var status, superadmin int
	err = tx.QueryRowContext(ctx, `SELECT dashboard_access_version,COALESCE(name,''),COALESCE(phone,''),status,COALESCE(isSuperAdmin,0) FROM mc_user WHERE tenant_id=? AND id=? AND deleted_at IS NULL LIMIT 1 FOR UPDATE`, command.TenantID, command.TargetUserID).Scan(&version, &name, &phone, &status, &superadmin)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.DashboardAccessUserDetail{}, dashboard.ErrDashboardAccessAdminNotFound
	}
	if err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	if version != command.ExpectedVersion {
		return dashboard.DashboardAccessUserDetail{}, dashboard.ErrDashboardAccessAdminConflict
	}
	if err := validateDashboardRoleIDsTx(ctx, tx, command.TenantID, command.RoleIDs); err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	permissionIDs, err := validateDashboardAssignmentsTx(ctx, tx, command.DirectPermissions)
	if err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	before, err := dashboardUserRelationsSnapshotTx(ctx, tx, command.TenantID, command.TargetUserID)
	if err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM mochat_go_dashboard_user_roles WHERE tenant_id=? AND user_id=?`, command.TenantID, command.TargetUserID); err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	for _, roleID := range uniqueSortedInts(command.RoleIDs) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_user_roles (tenant_id,user_id,role_id,created_at,updated_at) VALUES (?,?,?,NOW(),NOW())`, command.TenantID, command.TargetUserID, roleID); err != nil {
			return dashboard.DashboardAccessUserDetail{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM mochat_go_dashboard_user_permissions WHERE tenant_id=? AND user_id=?`, command.TenantID, command.TargetUserID); err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	for _, assignment := range command.DirectPermissions {
		if _, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_user_permissions (tenant_id,user_id,permission_id,effect,data_scope,created_at,updated_at) VALUES (?,?,?,'allow',?,NOW(),NOW())`, command.TenantID, command.TargetUserID, permissionIDs[assignment.Code], assignment.Scope); err != nil {
			return dashboard.DashboardAccessUserDetail{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE mc_user SET dashboard_access_version = dashboard_access_version + 1,updated_at=NOW() WHERE tenant_id=? AND id=? AND dashboard_access_version=?`, command.TenantID, command.TargetUserID, version); err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	after := map[string]any{"roleIds": uniqueSortedInts(command.RoleIDs), "directPermissions": command.DirectPermissions}
	if err = insertDashboardAccessAuditTx(ctx, tx, command.TenantID, command.ActorUserID, "user_access.replace", "user", strconv.Itoa(command.TargetUserID), before, after, &version, version+1, command.RequestID); err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	if err = tx.Commit(); err != nil {
		return dashboard.DashboardAccessUserDetail{}, err
	}
	return dashboard.DashboardAccessUserDetail{ID: command.TargetUserID, TenantID: command.TenantID, Name: name, Phone: phone, Status: status, IsSuperAdmin: superadmin == 1, DirectPermissions: command.DirectPermissions, Version: version + 1}, nil
}

func (s *MySQLStore) CreateDashboardRole(ctx context.Context, command dashboard.CreateDashboardRoleCommand) (dashboard.DashboardAccessRoleDetail, error) {
	if command.TenantID <= 0 || command.ActorUserID <= 0 || strings.TrimSpace(command.Name) == "" || (command.Status != 1 && command.Status != 2) || dashboardAccessSystemRole(command.Remark) {
		return dashboard.DashboardAccessRoleDetail{}, dashboard.ErrDashboardAccessAdminInvalid
	}
	tx, err := s.beginDashboardAccessAdmin(ctx)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	defer tx.Rollback()
	if err = validateDashboardAccessActorTx(ctx, tx, command.TenantID, command.ActorUserID); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	permissionIDs, err := validateDashboardAssignmentsTx(ctx, tx, command.Permissions)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO mc_rbac_role (tenant_id,name,remarks,status,operate_id,operate_name,data_permission,dashboard_access_version,created_at,updated_at) VALUES (?,?,?,?,?,?,'[]',1,NOW(),NOW())`, command.TenantID, command.Name, command.Remark, command.Status, command.ActorUserID, command.ActorName)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	roleID64, err := result.LastInsertId()
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	roleID := int(roleID64)
	if err = replaceDashboardRolePermissionsTx(ctx, tx, command.TenantID, roleID, command.Permissions, permissionIDs); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	after := map[string]any{"name": command.Name, "remark": command.Remark, "status": command.Status, "permissions": command.Permissions}
	if err = insertDashboardAccessAuditTx(ctx, tx, command.TenantID, command.ActorUserID, "role.create", "role", strconv.Itoa(roleID), nil, after, nil, 1, command.RequestID); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	if err = tx.Commit(); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	return dashboard.DashboardAccessRoleDetail{ID: roleID, TenantID: command.TenantID, Name: command.Name, Remark: command.Remark, Status: command.Status, Permissions: command.Permissions, Version: 1}, nil
}

func (s *MySQLStore) UpdateDashboardRole(ctx context.Context, command dashboard.UpdateDashboardRoleCommand) (dashboard.DashboardAccessRoleDetail, error) {
	return s.updateDashboardRole(ctx, command, nil)
}

func (s *MySQLStore) updateDashboardRole(ctx context.Context, command dashboard.UpdateDashboardRoleCommand, status *int) (dashboard.DashboardAccessRoleDetail, error) {
	if dashboardAccessSystemRole(command.Remark) {
		return dashboard.DashboardAccessRoleDetail{}, dashboard.ErrDashboardAccessAdminInvalid
	}
	tx, err := s.beginDashboardAccessAdmin(ctx)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	defer tx.Rollback()
	if err = validateDashboardAccessActorTx(ctx, tx, command.TenantID, command.ActorUserID); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	version, oldName, oldRemark, oldStatus, err := lockDashboardRoleTx(ctx, tx, command.TenantID, command.RoleID)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	if version != command.ExpectedVersion {
		return dashboard.DashboardAccessRoleDetail{}, dashboard.ErrDashboardAccessAdminConflict
	}
	if dashboardAccessSystemRole(oldRemark) {
		return dashboard.DashboardAccessRoleDetail{}, dashboard.ErrDashboardAccessAdminInvalid
	}
	permissionIDs, err := validateDashboardAssignmentsTx(ctx, tx, command.Permissions)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	beforePermissions, err := dashboardRolePermissionsSnapshotTx(ctx, tx, command.TenantID, command.RoleID)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	newStatus := oldStatus
	if status != nil {
		newStatus = *status
	}
	if _, err = tx.ExecContext(ctx, `UPDATE mc_rbac_role SET name=?,remarks=?,status=?,operate_id=?,operate_name=?,dashboard_access_version=dashboard_access_version+1,updated_at=NOW() WHERE tenant_id=? AND id=? AND dashboard_access_version=? AND deleted_at IS NULL`, command.Name, command.Remark, newStatus, command.ActorUserID, command.ActorName, command.TenantID, command.RoleID, version); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	if status == nil {
		if err = replaceDashboardRolePermissionsTx(ctx, tx, command.TenantID, command.RoleID, command.Permissions, permissionIDs); err != nil {
			return dashboard.DashboardAccessRoleDetail{}, err
		}
	}
	before := map[string]any{"name": oldName, "remark": oldRemark, "status": oldStatus, "permissions": beforePermissions}
	afterPermissions := command.Permissions
	if status != nil {
		afterPermissions = beforePermissions
	}
	after := map[string]any{"name": command.Name, "remark": command.Remark, "status": newStatus, "permissions": afterPermissions}
	action := "role.update"
	if status != nil {
		action = "role.status"
	}
	if err = insertDashboardAccessAuditTx(ctx, tx, command.TenantID, command.ActorUserID, action, "role", strconv.Itoa(command.RoleID), before, after, &version, version+1, command.RequestID); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	if err = tx.Commit(); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	return dashboard.DashboardAccessRoleDetail{ID: command.RoleID, TenantID: command.TenantID, Name: command.Name, Remark: command.Remark, Status: newStatus, Permissions: afterPermissions, Version: version + 1}, nil
}

func (s *MySQLStore) UpdateDashboardRoleStatus(ctx context.Context, command dashboard.UpdateDashboardRoleStatusCommand) (dashboard.DashboardAccessRoleDetail, error) {
	if command.Status != 1 && command.Status != 2 {
		return dashboard.DashboardAccessRoleDetail{}, dashboard.ErrDashboardAccessAdminInvalid
	}
	// updateDashboardRole obtains the authoritative existing name and remark.
	tx, err := s.beginDashboardAccessAdmin(ctx)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	defer tx.Rollback()
	if err = validateDashboardAccessActorTx(ctx, tx, command.TenantID, command.ActorUserID); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	version, name, remark, oldStatus, err := lockDashboardRoleTx(ctx, tx, command.TenantID, command.RoleID)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	if version != command.ExpectedVersion {
		return dashboard.DashboardAccessRoleDetail{}, dashboard.ErrDashboardAccessAdminConflict
	}
	if dashboardAccessSystemRole(remark) {
		return dashboard.DashboardAccessRoleDetail{}, dashboard.ErrDashboardAccessAdminInvalid
	}
	permissions, err := dashboardRolePermissionsSnapshotTx(ctx, tx, command.TenantID, command.RoleID)
	if err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE mc_rbac_role SET status=?,operate_id=?,operate_name=?,dashboard_access_version=dashboard_access_version+1,updated_at=NOW() WHERE tenant_id=? AND id=? AND dashboard_access_version=? AND deleted_at IS NULL`, command.Status, command.ActorUserID, command.ActorName, command.TenantID, command.RoleID, version); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	before := map[string]any{"name": name, "remark": remark, "status": oldStatus, "permissions": permissions}
	after := map[string]any{"name": name, "remark": remark, "status": command.Status, "permissions": permissions}
	if err = insertDashboardAccessAuditTx(ctx, tx, command.TenantID, command.ActorUserID, "role.status", "role", strconv.Itoa(command.RoleID), before, after, &version, version+1, command.RequestID); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	if err = tx.Commit(); err != nil {
		return dashboard.DashboardAccessRoleDetail{}, err
	}
	return dashboard.DashboardAccessRoleDetail{ID: command.RoleID, TenantID: command.TenantID, Name: name, Remark: remark, Status: command.Status, Permissions: permissions, Version: version + 1}, nil
}

func (s *MySQLStore) DeleteDashboardRole(ctx context.Context, command dashboard.DeleteDashboardRoleCommand) error {
	tx, err := s.beginDashboardAccessAdmin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = validateDashboardAccessActorTx(ctx, tx, command.TenantID, command.ActorUserID); err != nil {
		return err
	}
	version, name, remark, status, err := lockDashboardRoleTx(ctx, tx, command.TenantID, command.RoleID)
	if err != nil {
		return err
	}
	if version != command.ExpectedVersion {
		return dashboard.ErrDashboardAccessAdminConflict
	}
	if dashboardAccessSystemRole(remark) {
		return dashboard.ErrDashboardAccessAdminInvalid
	}
	var members int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_dashboard_user_roles WHERE tenant_id=? AND role_id=?`, command.TenantID, command.RoleID).Scan(&members); err != nil {
		return err
	}
	if members > 0 {
		return dashboard.ErrDashboardAccessRoleHasMembers
	}
	permissions, err := dashboardRolePermissionsSnapshotTx(ctx, tx, command.TenantID, command.RoleID)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM mochat_go_dashboard_role_permissions WHERE tenant_id=? AND role_id=?`, command.TenantID, command.RoleID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE mc_rbac_role SET status=2,deleted_at=NOW(),operate_id=?,operate_name=?,dashboard_access_version=dashboard_access_version+1,updated_at=NOW() WHERE tenant_id=? AND id=? AND dashboard_access_version=? AND deleted_at IS NULL`, command.ActorUserID, command.ActorName, command.TenantID, command.RoleID, version); err != nil {
		return err
	}
	before := map[string]any{"name": name, "remark": remark, "status": status, "permissions": permissions}
	if err = insertDashboardAccessAuditTx(ctx, tx, command.TenantID, command.ActorUserID, "role.delete", "role", strconv.Itoa(command.RoleID), before, nil, &version, version+1, command.RequestID); err != nil {
		return err
	}
	return tx.Commit()
}

func lockDashboardRoleTx(ctx context.Context, tx dashboardAccessAdminTx, tenantID, roleID int) (uint64, string, string, int, error) {
	var version uint64
	var name, remark string
	var status int
	err := tx.QueryRowContext(ctx, `SELECT dashboard_access_version,COALESCE(name,''),COALESCE(remarks,''),status FROM mc_rbac_role WHERE tenant_id=? AND id=? AND deleted_at IS NULL LIMIT 1 FOR UPDATE`, tenantID, roleID).Scan(&version, &name, &remark, &status)
	if errors.Is(err, sql.ErrNoRows) {
		err = dashboard.ErrDashboardAccessAdminNotFound
	}
	return version, name, remark, status, err
}

func validateDashboardAccessActorTx(ctx context.Context, tx dashboardAccessAdminTx, tenantID, actorUserID int) error {
	if tenantID <= 0 || actorUserID <= 0 {
		return dashboard.ErrDashboardAccessAdminForbidden
	}
	var lockedActorID int
	err := tx.QueryRowContext(ctx, `SELECT id AS dashboard_access_actor_id FROM mc_user WHERE tenant_id=? AND id=? AND status=1 AND COALESCE(isSuperAdmin,0)=1 AND deleted_at IS NULL LIMIT 1 FOR UPDATE`, tenantID, actorUserID).Scan(&lockedActorID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ErrDashboardAccessAdminForbidden
	}
	if err != nil {
		return err
	}
	if lockedActorID != actorUserID {
		return dashboard.ErrDashboardAccessAdminForbidden
	}
	return nil
}

func validateDashboardRoleIDsTx(ctx context.Context, tx dashboardAccessAdminTx, tenantID int, roleIDs []int) error {
	ids := uniqueSortedInts(roleIDs)
	if len(ids) == 0 {
		return nil
	}
	args := []any{tenantID}
	for _, id := range ids {
		args = append(args, id)
	}
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_rbac_role WHERE tenant_id=? AND id IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL`, args...).Scan(&count)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return dashboard.ErrDashboardAccessAdminNotFound
	}
	return nil
}

func validateDashboardAssignmentsTx(ctx context.Context, tx dashboardAccessAdminTx, assignments []dashboard.DashboardPermissionAssignment) (map[string]int64, error) {
	result := make(map[string]int64, len(assignments))
	if len(assignments) == 0 {
		return result, nil
	}
	codes := make([]string, 0, len(assignments))
	seen := make(map[string]bool, len(assignments))
	for _, item := range assignments {
		if item.Code == "" || seen[item.Code] || (item.Scope != dashboard.DataScopeSelf && item.Scope != dashboard.DataScopeDepartment && item.Scope != dashboard.DataScopeTenant) {
			return nil, dashboard.ErrDashboardAccessAdminInvalid
		}
		seen[item.Code] = true
		codes = append(codes, item.Code)
	}
	sort.Strings(codes)
	args := make([]any, len(codes))
	for i, code := range codes {
		args[i] = code
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,code,superadmin_only FROM mochat_go_dashboard_permissions permission WHERE permission.code IN (`+placeholders(len(codes))+`) AND permission.status=1 AND permission.deleted_at IS NULL`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var code string
		var superadmin int
		if err := rows.Scan(&id, &code, &superadmin); err != nil {
			return nil, err
		}
		if superadmin == 1 {
			return nil, dashboard.ErrDashboardAccessAdminInvalid
		}
		result[code] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) != len(assignments) {
		return nil, dashboard.ErrDashboardAccessAdminNotFound
	}
	return result, nil
}

func replaceDashboardRolePermissionsTx(ctx context.Context, tx dashboardAccessAdminTx, tenantID, roleID int, assignments []dashboard.DashboardPermissionAssignment, permissionIDs map[string]int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_dashboard_role_permissions WHERE tenant_id=? AND role_id=?`, tenantID, roleID); err != nil {
		return err
	}
	for _, item := range assignments {
		if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_role_permissions (tenant_id,role_id,permission_id,data_scope,created_at,updated_at) VALUES (?,?,?,?,NOW(),NOW())`, tenantID, roleID, permissionIDs[item.Code], item.Scope); err != nil {
			return err
		}
	}
	return nil
}

func dashboardUserRelationsSnapshotTx(ctx context.Context, tx dashboardAccessAdminTx, tenantID, userID int) (map[string]any, error) {
	roles := []int{}
	rows, err := tx.QueryContext(ctx, `SELECT role_id FROM mochat_go_dashboard_user_roles WHERE tenant_id=? AND user_id=? ORDER BY role_id`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		roles = append(roles, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	permissions := []map[string]any{}
	rows, err = tx.QueryContext(ctx, `SELECT permission_id,data_scope FROM mochat_go_dashboard_user_permissions WHERE tenant_id=? AND user_id=? ORDER BY permission_id`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var scope string
		if err := rows.Scan(&id, &scope); err != nil {
			rows.Close()
			return nil, err
		}
		permissions = append(permissions, map[string]any{"permissionId": id, "scope": scope})
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"roleIds": roles, "directPermissions": permissions}, nil
}

func dashboardRolePermissionsSnapshotTx(ctx context.Context, tx dashboardAccessAdminTx, tenantID, roleID int) ([]dashboard.DashboardPermissionAssignment, error) {
	rows, err := tx.QueryContext(ctx, `SELECT permission.code,relation.data_scope FROM mochat_go_dashboard_role_permissions relation INNER JOIN mochat_go_dashboard_permissions permission ON permission.id=relation.permission_id WHERE relation.tenant_id=? AND relation.role_id=? ORDER BY permission.code`, tenantID, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []dashboard.DashboardPermissionAssignment{}
	for rows.Next() {
		var item dashboard.DashboardPermissionAssignment
		if err := rows.Scan(&item.Code, &item.Scope); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func insertDashboardAccessAuditTx(ctx context.Context, tx dashboardAccessAdminTx, tenantID, actorUserID int, action, targetType, targetID string, before, after any, expected *uint64, result uint64, requestID string) error {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	var expectedValue any
	if expected != nil {
		expectedValue = *expected
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_permission_audits (tenant_id,actor_user_id,action,target_type,target_id,before_json,after_json,expected_version,result_version,request_id,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,NOW())`, tenantID, actorUserID, action, targetType, targetID, string(beforeJSON), string(afterJSON), expectedValue, result, strings.TrimSpace(requestID))
	return err
}

func uniqueSortedInts(values []int) []int {
	seen := map[int]bool{}
	result := []int{}
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Ints(result)
	return result
}
func dashboardAccessSystemRole(remark string) bool {
	return dashboard.IsReservedDashboardRoleRemark(remark)
}
func dashboardAdminPage(page, perPage, total int) dashboard.DashboardAccessPage {
	totalPage := 0
	if total > 0 && perPage > 0 {
		totalPage = (total + perPage - 1) / perPage
	}
	return dashboard.DashboardAccessPage{Page: page, PerPage: perPage, Total: total, TotalPage: totalPage}
}
