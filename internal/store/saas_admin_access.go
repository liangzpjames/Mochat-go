package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

type saasAdminAccessQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *MySQLStore) SaaSAdminAccessProfile(ctx context.Context, userID int, platformTenantID int) (dashboard.SaaSAdminAccessProfile, error) {
	if userID <= 0 || platformTenantID <= 0 {
		return dashboard.SaaSAdminAccessProfile{}, dashboard.NewSaaSAdminBadRequest("userId 或 platformTenantId 无效")
	}
	var profile dashboard.SaaSAdminAccessProfile
	var isSuperAdmin int
	var accessVersion sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, COALESCE(u.name, ''), COALESCE(u.phone, ''),
			CASE WHEN EXISTS (
				SELECT 1
				FROM mochat_go_saas_admin_user_roles ur
				INNER JOIN mochat_go_saas_admin_roles r ON r.id = ur.role_id
				INNER JOIN mochat_go_saas_admin_role_permissions rp ON rp.role_id = r.id
				WHERE ur.user_id = u.id
				  AND r.code = 'platform_root'
				  AND r.status = 1
				  AND r.is_system = 1
				  AND rp.permission_code = '*'
			) THEN 1 ELSE 0 END AS is_platform_super_admin,
			ua.version
		FROM mochat_go_saas_admin_users u
		LEFT JOIN mochat_go_saas_admin_user_access ua ON ua.user_id = u.id
		WHERE u.id = ? AND u.status = 1
		LIMIT 1
	`, userID).Scan(&profile.UserID, &profile.UserName, &profile.Phone, &isSuperAdmin, &accessVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminAccessProfile{}, dashboard.NewSaaSAdminNotFound("平台用户不存在")
	}
	if err != nil {
		return dashboard.SaaSAdminAccessProfile{}, err
	}
	profile.TenantID = platformTenantID
	if profile.TenantID != platformTenantID {
		return dashboard.SaaSAdminAccessProfile{}, &dashboard.SaaSAdminOperationError{Status: 403, Message: "用户不属于平台管理租户"}
	}
	profile.IsPlatformSuperAdmin = isSuperAdmin == 1
	if accessVersion.Valid {
		profile.Version = int(accessVersion.Int64)
	}
	if profile.IsPlatformSuperAdmin {
		profile.Permissions = []string{"*"}
		return profile, nil
	}
	rolesByUser, err := loadSaaSAdminAccessRolesForUsers(ctx, s.db, []int{userID}, true)
	if err != nil {
		return dashboard.SaaSAdminAccessProfile{}, err
	}
	profile.Roles = rolesByUser[userID]
	profile.Permissions = mergeSaaSAdminAccessPermissions(profile.Roles)
	return profile, nil
}

func (s *MySQLStore) SaaSAdminAccessRoles(ctx context.Context) ([]dashboard.SaaSAdminAccessRole, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.code, r.name, r.description, r.status, r.is_system, r.version,
			r.created_by, r.updated_by, r.created_at, r.updated_at, COUNT(DISTINCT ur.user_id)
		FROM mochat_go_saas_admin_roles r
		LEFT JOIN mochat_go_saas_admin_user_roles ur ON ur.role_id = r.id
		GROUP BY r.id, r.code, r.name, r.description, r.status, r.is_system, r.version,
			r.created_by, r.updated_by, r.created_at, r.updated_at
		ORDER BY r.is_system DESC, r.status ASC, r.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := make([]dashboard.SaaSAdminAccessRole, 0)
	roleIDs := make([]int64, 0)
	for rows.Next() {
		role, err := scanSaaSAdminAccessRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
		roleIDs = append(roleIDs, role.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	permissions, err := loadSaaSAdminRolePermissions(ctx, s.db, roleIDs)
	if err != nil {
		return nil, err
	}
	for index := range roles {
		roles[index].Permissions = permissions[roles[index].ID]
	}
	return roles, nil
}

func (s *MySQLStore) UpsertSaaSAdminAccessRole(ctx context.Context, input dashboard.SaaSAdminAccessRoleUpsert) (dashboard.SaaSAdminAccessRoleUpsertResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
	}
	defer tx.Rollback()

	var before dashboard.SaaSAdminAccessRole
	created := input.ID == 0
	roleID := input.ID
	if created {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_admin_roles
				(code, name, description, status, is_system, version, created_by, updated_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, 0, 1, ?, ?, NOW(), NOW())
		`, input.Code, input.Name, input.Description, input.Status, input.ActorUserID, input.ActorUserID)
		if isMySQLDuplicateKeyError(err) {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "平台岗位编码已存在"}
		}
		if err != nil {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
		}
		roleID, _ = result.LastInsertId()
	} else {
		before, err = saasAdminAccessRoleByID(ctx, tx, roleID, true)
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, dashboard.NewSaaSAdminNotFound("平台岗位不存在")
		}
		if err != nil {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
		}
		if before.IsSystem {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "内置平台岗位不可修改"}
		}
		if before.Version != input.ExpectedVersion {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "平台岗位版本已变化，请刷新后重试"}
		}
		beforePermissions, err := loadSaaSAdminRolePermissions(ctx, tx, []int64{roleID})
		if err != nil {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
		}
		before.Permissions = beforePermissions[roleID]
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_admin_roles
			SET code = ?, name = ?, description = ?, status = ?, version = version + 1, updated_by = ?, updated_at = NOW()
			WHERE id = ? AND version = ? AND is_system = 0
		`, input.Code, input.Name, input.Description, input.Status, input.ActorUserID, roleID, input.ExpectedVersion)
		if isMySQLDuplicateKeyError(err) {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "平台岗位编码已存在"}
		}
		if err != nil {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "平台岗位版本已变化，请刷新后重试"}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_saas_admin_role_permissions WHERE role_id = ?`, roleID); err != nil {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
		}
	}
	for _, permission := range input.Permissions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_admin_role_permissions (role_id, permission_code, created_at)
			VALUES (?, ?, NOW())
		`, roleID, permission); err != nil {
			return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
		}
	}
	after, err := saasAdminAccessRoleByID(ctx, tx, roleID, false)
	if err != nil {
		return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
	}
	after.Permissions = append([]string(nil), input.Permissions...)
	beforeJSON, _ := json.Marshal(saasAdminAccessRoleAuditPayload(before))
	afterJSON, _ := json.Marshal(saasAdminAccessRoleAuditPayload(after))
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionAccessRoleSave, TargetType: dashboard.SaaSAdminOperationTargetAccessRole,
		TargetID: strconv.FormatInt(roleID, 10), TargetName: after.Name, BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON),
		Remark: "save SaaS admin access role",
	})
	if err != nil {
		return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminAccessRoleUpsertResult{}, err
	}
	return dashboard.SaaSAdminAccessRoleUpsertResult{Role: after, OperationID: operationID, Created: created}, nil
}

func (s *MySQLStore) SaaSAdminAccessAssignments(ctx context.Context, platformTenantID int, options dashboard.SaaSAdminAccessAssignmentOptions) ([]dashboard.SaaSAdminAccessAssignment, error) {
	if options.Limit <= 0 || options.Limit > 100 {
		options.Limit = 100
	}
	if platformTenantID <= 0 {
		return nil, dashboard.NewSaaSAdminBadRequest("platformTenantId 无效")
	}
	where := []string{"1 = 1"}
	args := []any{}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		where = append(where, "(u.name LIKE ? OR u.phone LIKE ? OR CAST(u.id AS CHAR) = ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, keyword)
	}
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, COALESCE(u.name, ''), COALESCE(u.phone, ''), u.status,
			CASE WHEN EXISTS (
				SELECT 1
				FROM mochat_go_saas_admin_user_roles ur
				INNER JOIN mochat_go_saas_admin_roles r ON r.id = ur.role_id
				INNER JOIN mochat_go_saas_admin_role_permissions rp ON rp.role_id = r.id
				WHERE ur.user_id = u.id
				  AND r.code = 'platform_root'
				  AND r.status = 1
				  AND r.is_system = 1
				  AND rp.permission_code = '*'
			) THEN 1 ELSE 0 END AS is_platform_super_admin,
			COALESCE(ua.version, 0), COALESCE(ua.updated_by, 0), ua.updated_at
		FROM mochat_go_saas_admin_users u
		LEFT JOIN mochat_go_saas_admin_user_access ua ON ua.user_id = u.id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY is_platform_super_admin DESC, u.status ASC, u.id ASC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSAdminAccessAssignment, 0)
	userIDs := make([]int, 0)
	for rows.Next() {
		var item dashboard.SaaSAdminAccessAssignment
		var isSuperAdmin int
		var updatedAt sql.NullTime
		if err := rows.Scan(&item.UserID, &item.UserName, &item.Phone, &item.Status, &isSuperAdmin, &item.Version, &item.UpdatedBy, &updatedAt); err != nil {
			return nil, err
		}
		item.IsSuperAdmin = isSuperAdmin == 1
		item.UpdatedAt = formatTime(updatedAt)
		items = append(items, item)
		userIDs = append(userIDs, item.UserID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rolesByUser, err := loadSaaSAdminAccessRolesForUsers(ctx, s.db, userIDs, false)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Roles = rolesByUser[items[index].UserID]
		if items[index].IsSuperAdmin {
			items[index].Permissions = []string{"*"}
		} else {
			items[index].Permissions = mergeSaaSAdminAccessPermissions(items[index].Roles)
		}
	}
	return items, nil
}

func (s *MySQLStore) UpdateSaaSAdminAccessAssignment(ctx context.Context, platformTenantID int, input dashboard.SaaSAdminAccessAssignmentUpdate) (dashboard.SaaSAdminAccessAssignmentUpdateResult, error) {
	if input.UserID == input.ActorUserID {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, dashboard.NewSaaSAdminBadRequest("不能修改自己的平台授权")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
	}
	defer tx.Rollback()

	var targetName, targetPhone string
	var targetStatus, targetSuper int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(u.name, ''), COALESCE(u.phone, ''), u.status,
			CASE WHEN EXISTS (
				SELECT 1
				FROM mochat_go_saas_admin_user_roles ur
				INNER JOIN mochat_go_saas_admin_roles r ON r.id = ur.role_id
				INNER JOIN mochat_go_saas_admin_role_permissions rp ON rp.role_id = r.id
				WHERE ur.user_id = u.id
				  AND r.code = 'platform_root'
				  AND r.status = 1
				  AND r.is_system = 1
				  AND rp.permission_code = '*'
			) THEN 1 ELSE 0 END
		FROM mochat_go_saas_admin_users u
		WHERE u.id = ?
		LIMIT 1 FOR UPDATE
	`, input.UserID).Scan(&targetName, &targetPhone, &targetStatus, &targetSuper)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, dashboard.NewSaaSAdminNotFound("平台用户不存在")
	}
	if err != nil {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
	}
	if platformTenantID <= 0 {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 403, Message: "目标用户不属于平台管理租户"}
	}
	if targetSuper == 1 {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "平台超级管理员使用隐式全权限，无需分配岗位"}
	}

	var currentVersion, currentUpdatedBy int
	var currentUpdatedAt sql.NullTime
	stateExists := true
	err = tx.QueryRowContext(ctx, `
		SELECT version, updated_by, updated_at
		FROM mochat_go_saas_admin_user_access WHERE user_id = ? LIMIT 1 FOR UPDATE
	`, input.UserID).Scan(&currentVersion, &currentUpdatedBy, &currentUpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		stateExists = false
		currentVersion = 0
	} else if err != nil {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
	}
	if currentVersion != input.ExpectedVersion {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "人员授权版本已变化，请刷新后重试"}
	}
	beforeRolesByUser, err := loadSaaSAdminAccessRolesForUsers(ctx, tx, []int{input.UserID}, false)
	if err != nil {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
	}
	before := dashboard.SaaSAdminAccessAssignment{
		UserID: input.UserID, UserName: targetName, Phone: targetPhone, Status: targetStatus,
		Version: currentVersion, Roles: beforeRolesByUser[input.UserID], UpdatedBy: currentUpdatedBy, UpdatedAt: formatTime(currentUpdatedAt),
	}

	roles, err := saasAdminAccessRolesByIDs(ctx, tx, input.RoleIDs, true)
	if err != nil {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
	}
	if len(roles) != len(input.RoleIDs) {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, dashboard.NewSaaSAdminBadRequest("roleIds 包含不存在或已停用的平台岗位")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM mochat_go_saas_admin_user_roles WHERE user_id = ?`, input.UserID); err != nil {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
	}
	for _, roleID := range input.RoleIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
			VALUES (?, ?, ?, NOW())
		`, input.UserID, roleID, input.ActorUserID); err != nil {
			return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
		}
	}
	newVersion := currentVersion + 1
	if !stateExists {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())
		`, input.UserID, newVersion, input.ActorUserID); err != nil {
			return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
		}
	} else {
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_admin_user_access
			SET version = ?, updated_by = ?, updated_at = NOW()
			WHERE user_id = ? AND version = ?
		`, newVersion, input.ActorUserID, input.UserID, currentVersion)
		if err != nil {
			return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "人员授权版本已变化，请刷新后重试"}
		}
	}
	after := dashboard.SaaSAdminAccessAssignment{
		UserID: input.UserID, UserName: targetName, Phone: targetPhone, Status: targetStatus,
		Version: newVersion, Roles: roles, Permissions: mergeSaaSAdminAccessPermissions(roles), UpdatedBy: input.ActorUserID,
	}
	beforeJSON, _ := json.Marshal(saasAdminAccessAssignmentAuditPayload(before))
	afterJSON, _ := json.Marshal(saasAdminAccessAssignmentAuditPayload(after))
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionAccessAssignmentSave, TargetType: dashboard.SaaSAdminOperationTargetAccessAssignment,
		TargetID: strconv.Itoa(input.UserID), TargetName: targetName, BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON),
		Remark: "save SaaS admin access assignment",
	})
	if err != nil {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminAccessAssignmentUpdateResult{}, err
	}
	return dashboard.SaaSAdminAccessAssignmentUpdateResult{Assignment: after, OperationID: operationID}, nil
}

func scanSaaSAdminAccessRole(scanner sqlScanner) (dashboard.SaaSAdminAccessRole, error) {
	var role dashboard.SaaSAdminAccessRole
	var isSystem int
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(
		&role.ID, &role.Code, &role.Name, &role.Description, &role.Status, &isSystem, &role.Version,
		&role.CreatedBy, &role.UpdatedBy, &createdAt, &updatedAt, &role.AssignmentCount,
	)
	role.IsSystem = isSystem == 1
	role.CreatedAt = formatTime(createdAt)
	role.UpdatedAt = formatTime(updatedAt)
	return role, err
}

func saasAdminAccessRoleByID(ctx context.Context, queryer saasAdminAccessQueryer, roleID int64, forUpdate bool) (dashboard.SaaSAdminAccessRole, error) {
	query := `
		SELECT r.id, r.code, r.name, r.description, r.status, r.is_system, r.version,
			r.created_by, r.updated_by, r.created_at, r.updated_at,
			(SELECT COUNT(*) FROM mochat_go_saas_admin_user_roles ur WHERE ur.role_id = r.id)
		FROM mochat_go_saas_admin_roles r
		WHERE r.id = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	return scanSaaSAdminAccessRole(queryer.QueryRowContext(ctx, query, roleID))
}

func saasAdminAccessRolesByIDs(ctx context.Context, queryer saasAdminAccessQueryer, roleIDs []int64, activeOnly bool) ([]dashboard.SaaSAdminAccessRole, error) {
	if len(roleIDs) == 0 {
		return []dashboard.SaaSAdminAccessRole{}, nil
	}
	args := make([]any, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		args = append(args, roleID)
	}
	where := "r.id IN (" + placeholders(len(roleIDs)) + ")"
	if activeOnly {
		where += " AND r.status = 1"
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT r.id, r.code, r.name, r.description, r.status, r.is_system, r.version,
			r.created_by, r.updated_by, r.created_at, r.updated_at, COUNT(DISTINCT ur.user_id)
		FROM mochat_go_saas_admin_roles r
		LEFT JOIN mochat_go_saas_admin_user_roles ur ON ur.role_id = r.id
		WHERE `+where+`
		GROUP BY r.id, r.code, r.name, r.description, r.status, r.is_system, r.version,
			r.created_by, r.updated_by, r.created_at, r.updated_at
		ORDER BY r.id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := make([]dashboard.SaaSAdminAccessRole, 0, len(roleIDs))
	loadedIDs := make([]int64, 0, len(roleIDs))
	for rows.Next() {
		role, err := scanSaaSAdminAccessRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
		loadedIDs = append(loadedIDs, role.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	permissions, err := loadSaaSAdminRolePermissions(ctx, queryer, loadedIDs)
	if err != nil {
		return nil, err
	}
	for index := range roles {
		roles[index].Permissions = permissions[roles[index].ID]
	}
	return roles, nil
}

func loadSaaSAdminRolePermissions(ctx context.Context, queryer saasAdminAccessQueryer, roleIDs []int64) (map[int64][]string, error) {
	result := make(map[int64][]string, len(roleIDs))
	if len(roleIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		args = append(args, roleID)
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT role_id, permission_code
		FROM mochat_go_saas_admin_role_permissions
		WHERE role_id IN (`+placeholders(len(roleIDs))+`)
		ORDER BY role_id ASC, permission_code ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var roleID int64
		var permission string
		if err := rows.Scan(&roleID, &permission); err != nil {
			return nil, err
		}
		result[roleID] = append(result[roleID], permission)
	}
	return result, rows.Err()
}

func loadSaaSAdminAccessRolesForUsers(ctx context.Context, queryer saasAdminAccessQueryer, userIDs []int, activeOnly bool) (map[int][]dashboard.SaaSAdminAccessRole, error) {
	result := make(map[int][]dashboard.SaaSAdminAccessRole, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	args := make([]any, 0, len(userIDs))
	for _, userID := range userIDs {
		args = append(args, userID)
	}
	activeFilter := ""
	if activeOnly {
		activeFilter = " AND r.status = 1"
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT ur.user_id, r.id, r.code, r.name, r.description, r.status, r.is_system, r.version,
			r.created_by, r.updated_by, r.created_at, r.updated_at, rp.permission_code
		FROM mochat_go_saas_admin_user_roles ur
		INNER JOIN mochat_go_saas_admin_roles r ON r.id = ur.role_id`+activeFilter+`
		LEFT JOIN mochat_go_saas_admin_role_permissions rp ON rp.role_id = r.id
		WHERE ur.user_id IN (`+placeholders(len(userIDs))+`)
		ORDER BY ur.user_id ASC, r.id ASC, rp.permission_code ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type roleKey struct {
		userID int
		roleID int64
	}
	roleIndexes := make(map[roleKey]int)
	for rows.Next() {
		var userID int
		var role dashboard.SaaSAdminAccessRole
		var isSystem int
		var createdAt, updatedAt sql.NullTime
		var permission sql.NullString
		if err := rows.Scan(
			&userID, &role.ID, &role.Code, &role.Name, &role.Description, &role.Status, &isSystem, &role.Version,
			&role.CreatedBy, &role.UpdatedBy, &createdAt, &updatedAt, &permission,
		); err != nil {
			return nil, err
		}
		key := roleKey{userID: userID, roleID: role.ID}
		index, exists := roleIndexes[key]
		if !exists {
			role.IsSystem = isSystem == 1
			role.CreatedAt = formatTime(createdAt)
			role.UpdatedAt = formatTime(updatedAt)
			result[userID] = append(result[userID], role)
			index = len(result[userID]) - 1
			roleIndexes[key] = index
		}
		if permission.Valid && permission.String != "" {
			roles := result[userID]
			roles[index].Permissions = append(roles[index].Permissions, permission.String)
			result[userID] = roles
		}
	}
	return result, rows.Err()
}

func mergeSaaSAdminAccessPermissions(roles []dashboard.SaaSAdminAccessRole) []string {
	seen := make(map[string]struct{})
	for _, role := range roles {
		if role.Status != 1 {
			continue
		}
		for _, permission := range role.Permissions {
			if dashboard.SaaSAdminPermissionValid(permission) {
				seen[permission] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(seen))
	for _, definition := range dashboard.SaaSAdminPermissionCatalog() {
		if _, exists := seen[definition.Code]; exists {
			result = append(result, definition.Code)
		}
	}
	return result
}

func saasAdminAccessRoleAuditPayload(role dashboard.SaaSAdminAccessRole) map[string]any {
	if role.ID == 0 {
		return map[string]any{}
	}
	return map[string]any{
		"id": role.ID, "code": role.Code, "name": role.Name, "description": role.Description,
		"status": role.Status, "isSystem": role.IsSystem, "version": role.Version, "permissions": role.Permissions,
	}
}

func saasAdminAccessAssignmentAuditPayload(item dashboard.SaaSAdminAccessAssignment) map[string]any {
	roleIDs := make([]int64, 0, len(item.Roles))
	roleCodes := make([]string, 0, len(item.Roles))
	for _, role := range item.Roles {
		roleIDs = append(roleIDs, role.ID)
		roleCodes = append(roleCodes, role.Code)
	}
	return map[string]any{
		"userId": item.UserID, "userName": item.UserName, "version": item.Version,
		"roleIds": roleIDs, "roleCodes": roleCodes, "permissions": item.Permissions,
	}
}

var _ dashboard.SaaSAdminAccessStore = (*MySQLStore)(nil)
