package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
)

var (
	errCompanyBindingNotFound    = errors.New("company binding not found")
	errCompanyBindingNotUnique   = errors.New("company binding is not unique")
	errCompanyBindingNotVerified = errors.New("company binding is not verified")
)

type companySyncBinding struct {
	TenantID int
	CorpID   int
	Status   int
	WXCorpID string
}

func requireSingleCompanySyncBinding(bindings []companySyncBinding) (companySyncBinding, error) {
	if len(bindings) == 0 {
		return companySyncBinding{}, errCompanyBindingNotFound
	}
	if len(bindings) != 1 {
		return companySyncBinding{}, errCompanyBindingNotUnique
	}
	binding := bindings[0]
	if binding.TenantID <= 0 || binding.CorpID <= 0 {
		return companySyncBinding{}, errCompanyBindingNotUnique
	}
	if binding.Status != 2 || strings.TrimSpace(binding.WXCorpID) == "" {
		return companySyncBinding{}, errCompanyBindingNotVerified
	}
	binding.WXCorpID = strings.TrimSpace(binding.WXCorpID)
	return binding, nil
}

func loadSingleCompanySyncBinding(ctx context.Context, queryer companyProfileQueryer, tenantID int, forUpdate bool) (companySyncBinding, error) {
	if tenantID <= 0 {
		return companySyncBinding{}, errCompanyBindingNotFound
	}
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT tenant_id, corp_id, status, COALESCE(verified_wx_corpid, '')
		FROM mochat_go_tenant_corp_bindings
		WHERE tenant_id = ?`+suffix, tenantID)
	if err != nil {
		return companySyncBinding{}, err
	}
	bindings := make([]companySyncBinding, 0, 2)
	for rows.Next() {
		var binding companySyncBinding
		if err := rows.Scan(&binding.TenantID, &binding.CorpID, &binding.Status, &binding.WXCorpID); err != nil {
			_ = rows.Close()
			return companySyncBinding{}, err
		}
		bindings = append(bindings, binding)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return companySyncBinding{}, err
	}
	if err := rows.Close(); err != nil {
		return companySyncBinding{}, err
	}
	return requireSingleCompanySyncBinding(bindings)
}

func (s *MySQLStore) TenantIDByBindingID(ctx context.Context, bindingID int) (int, error) {
	if s == nil || s.db == nil || bindingID <= 0 {
		return 0, errors.New("company binding unavailable")
	}
	binding, err := loadSingleCompanySyncBinding(ctx, s.db, bindingID, false)
	if err != nil {
		return 0, err
	}
	return binding.TenantID, nil
}

func (s *MySQLStore) CompanyEmployeeSyncCredentials(ctx context.Context, bindingID int) ([]dashboard.WorkEmployeeSyncCredential, error) {
	if s == nil || s.db == nil || bindingID <= 0 {
		return nil, errors.New("company binding unavailable")
	}
	binding, err := loadSingleCompanySyncBinding(ctx, s.db, bindingID, false)
	if errors.Is(err, errCompanyBindingNotFound) || errors.Is(err, errCompanyBindingNotVerified) {
		return []dashboard.WorkEmployeeSyncCredential{}, nil
	}
	if err != nil {
		return nil, err
	}
	item, found, err := loadEncryptedCorpCredentialByID(ctx, s.db, binding.CorpID, false)
	if err != nil {
		return nil, err
	}
	if !found || item.TenantID != binding.TenantID || strings.TrimSpace(item.WXCorpID) != binding.WXCorpID {
		return []dashboard.WorkEmployeeSyncCredential{}, nil
	}
	secret, err := s.decodeEncryptedCorpCredential(item)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(secret.EmployeeSecret) == "" {
		return []dashboard.WorkEmployeeSyncCredential{}, nil
	}
	return []dashboard.WorkEmployeeSyncCredential{{
		CorpID: item.ID, TenantID: binding.TenantID, WXCorpID: binding.WXCorpID,
		EmployeeSecret: secret.EmployeeSecret, ContactSecret: secret.ContactSecret,
	}}, nil
}

func (s *MySQLStore) SyncCompanyEmployees(ctx context.Context, bindingID int, departments []dashboard.WorkEmployeeSyncDepartment, employees []dashboard.WorkEmployeeSyncEmployee) (dashboard.WorkEmployeeSyncResult, error) {
	return s.syncCompanyEmployeesTx(ctx, bindingID, 0, 0, departments, employees)
}

func (s *MySQLStore) SyncEmployeeData(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, data companyprofile.EmployeeSyncData) (companyprofile.SyncResult, error) {
	if principal.TenantID <= 0 || principal.CorpID <= 0 || principal.UserID <= 0 {
		return companyprofile.SyncResult{}, companyprofile.ErrPermissionDenied
	}
	departments := make([]dashboard.WorkEmployeeSyncDepartment, 0, len(data.Departments))
	for _, department := range data.Departments {
		departments = append(departments, dashboard.WorkEmployeeSyncDepartment{
			WXDepartmentID: department.WXDepartmentID, Name: department.Name,
			WXParentID: department.WXParentID, Order: department.Order,
		})
	}
	employees := make([]dashboard.WorkEmployeeSyncEmployee, 0, len(data.Employees))
	for _, employee := range data.Employees {
		employees = append(employees, dashboard.WorkEmployeeSyncEmployee{
			WXUserID: employee.WXUserID, Name: employee.Name, Mobile: employee.Mobile,
			Position: employee.Position, Gender: employee.Gender, Email: employee.Email,
			Avatar: employee.Avatar, ThumbAvatar: employee.ThumbAvatar, Telephone: employee.Telephone,
			Alias: employee.Alias, Status: employee.Status, QRCode: employee.QRCode,
			Address: employee.Address, OpenUserID: employee.OpenUserID,
			WXMainDepartmentID: employee.WXMainDepartmentID, DepartmentIDs: append([]int(nil), employee.DepartmentIDs...),
			IsLeaderInDepartment: append([]int(nil), employee.IsLeaderInDepartment...), DepartmentOrders: append([]int(nil), employee.DepartmentOrders...),
		})
	}
	started := time.Now().UTC()
	result, err := s.syncCompanyEmployeesTx(ctx, principal.TenantID, principal.UserID, principal.AuthVersion, departments, employees)
	if err != nil {
		_ = s.recordCompanySyncFailure(ctx, principal)
		return companyprofile.SyncResult{Status: "failed", StartedAt: started, FinishedAt: time.Now().UTC(), ErrorCode: "SYNC_FAILED"}, err
	}
	return companyprofile.SyncResult{
		Status: "completed", DepartmentsCreated: result.DepartmentsCreated, DepartmentsUpdated: result.DepartmentsUpdated,
		EmployeesCreated: result.EmployeesCreated, EmployeesUpdated: result.EmployeesUpdated,
		StartedAt: started, FinishedAt: time.Now().UTC(),
	}, nil
}

func (s *MySQLStore) GetSyncStatus(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (companyprofile.SyncStatus, error) {
	if s == nil || s.db == nil {
		return companyprofile.SyncStatus{}, companyprofile.ErrStoreUnavailable
	}
	if err := s.checkCompanyActor(ctx, s.db, principal, false); err != nil {
		return companyprofile.SyncStatus{}, err
	}
	var corpID int
	var status int
	var verified string
	if err := s.db.QueryRowContext(ctx, `
		SELECT b.corp_id, b.status, COALESCE(b.verified_wx_corpid,'')
		FROM mochat_go_tenant_corp_bindings b
		WHERE b.tenant_id = ? AND b.corp_id = ? LIMIT 1`, principal.TenantID, principal.CorpID).Scan(&corpID, &status, &verified); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return companyprofile.SyncStatus{}, companyprofile.ErrNotFound
		}
		return companyprofile.SyncStatus{}, err
	}
	if status != 2 || strings.TrimSpace(verified) == "" {
		return companyprofile.SyncStatus{}, companyprofile.ErrTenantAccessDenied
	}
	var lastUpdate sql.NullTime
	var errorMessage sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT last_update_time, COALESCE(CAST(error_msg AS CHAR), '')
		FROM mc_work_update_time
		WHERE corp_id = ? AND type = 1
		ORDER BY id DESC LIMIT 1`, corpID).Scan(&lastUpdate, &errorMessage)
	if errors.Is(err, sql.ErrNoRows) {
		return companyprofile.SyncStatus{Status: "idle"}, nil
	}
	if err != nil {
		return companyprofile.SyncStatus{}, err
	}
	var departments, employees int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_department WHERE corp_id = ? AND deleted_at IS NULL`, corpID).Scan(&departments); err != nil {
		return companyprofile.SyncStatus{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_employee WHERE corp_id = ? AND deleted_at IS NULL`, corpID).Scan(&employees); err != nil {
		return companyprofile.SyncStatus{}, err
	}
	statusResult := companySyncStatusFromRecord(lastUpdate, errorMessage, departments, employees)
	if lastUpdate.Valid {
		value := lastUpdate.Time
		statusResult.FinishedAt = &value
	}
	return statusResult, nil
}

func (s *MySQLStore) syncCompanyEmployeesTx(ctx context.Context, bindingID, actorUserID int, actorAuthVersion uint64, departments []dashboard.WorkEmployeeSyncDepartment, employees []dashboard.WorkEmployeeSyncEmployee) (dashboard.WorkEmployeeSyncResult, error) {
	if s == nil || s.db == nil || bindingID <= 0 {
		return dashboard.WorkEmployeeSyncResult{}, errors.New("company binding unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	defer rollbackQuietly(tx)
	binding, err := loadSingleCompanySyncBinding(ctx, tx, bindingID, true)
	if errors.Is(err, errCompanyBindingNotFound) {
		return dashboard.WorkEmployeeSyncResult{}, errors.New("company binding unavailable")
	}
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	if actorUserID > 0 {
		principal := dashboardprincipal.DashboardPrincipal{UserID: actorUserID, TenantID: binding.TenantID, CorpID: binding.CorpID, AuthVersion: actorAuthVersion, IsSuperAdmin: true, CorpStatus: dashboardprincipal.CorpBindingStatusActive}
		if err := s.checkCompanyActor(ctx, tx, principal, true); err != nil {
			return dashboard.WorkEmployeeSyncResult{}, err
		}
	}
	credential, found, err := loadEncryptedCorpCredentialByID(ctx, tx, binding.CorpID, true)
	if err != nil || !found {
		return dashboard.WorkEmployeeSyncResult{}, errors.New("company credential unavailable")
	}
	secret, err := s.decodeEncryptedCorpCredential(credential)
	if err != nil || strings.TrimSpace(secret.EmployeeSecret) == "" {
		return dashboard.WorkEmployeeSyncResult{}, errors.New("company credential unavailable")
	}
	syncCredential := dashboard.WorkEmployeeSyncCredential{CorpID: binding.CorpID, TenantID: binding.TenantID, WXCorpID: binding.WXCorpID, EmployeeSecret: secret.EmployeeSecret, ContactSecret: secret.ContactSecret}
	departmentMap, result, err := syncWorkEmployeeDepartmentsTx(ctx, tx, binding.CorpID, departments)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	employeeResult, err := syncWorkEmployeesWithoutIdentityTx(ctx, tx, syncCredential, departmentMap, employees)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	result.EmployeesCreated = employeeResult.EmployeesCreated
	result.EmployeesUpdated = employeeResult.EmployeesUpdated
	result.RelationsCreated = employeeResult.RelationsCreated
	result.RelationsUpdated = employeeResult.RelationsUpdated
	result.RelationsDeleted = employeeResult.RelationsDeleted
	if err := upsertCompanySyncUpdateTimeTx(ctx, tx, binding.CorpID, 1); err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	return result, nil
}

func companySyncStatusFromRecord(lastUpdate sql.NullTime, errorMessage sql.NullString, departments, employees int) companyprofile.SyncStatus {
	status := companyprofile.SyncStatus{Status: "completed", Departments: departments, Employees: employees}
	if strings.TrimSpace(errorMessage.String) != "" {
		status.Status = "failed"
		status.ErrorCode = "SYNC_FAILED"
	}
	return status
}

func upsertCompanySyncUpdateTimeTx(ctx context.Context, tx *sql.Tx, corpID int, updateType int) error {
	var updateTimeID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_update_time
		WHERE corp_id = ? AND type = ?
		ORDER BY id DESC
		LIMIT 1
	`, corpID, updateType).Scan(&updateTimeID)
	if errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO mc_work_update_time (corp_id, type, last_update_time, error_msg, created_at, updated_at)
			VALUES (?, ?, NOW(), NULL, NOW(), NOW())
		`, corpID, updateType)
		if insertErr != nil {
			return insertErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return errors.New("company sync status insert affected unexpected rows")
		}
		return nil
	}
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_update_time
		SET last_update_time = NOW(), error_msg = NULL, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND type = ?
	`, updateTimeID, corpID, updateType)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return errors.New("company sync status update affected unexpected rows")
	}
	return nil
}

func (s *MySQLStore) recordCompanySyncFailure(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) error {
	if s == nil || s.db == nil || principal.TenantID <= 0 || principal.CorpID <= 0 || principal.UserID <= 0 {
		return errors.New("company sync status unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	if err := s.checkCompanyActor(ctx, tx, principal, true); err != nil {
		return err
	}
	var bindingStatus int
	var verifiedCorpID string
	if err := tx.QueryRowContext(ctx, `
		SELECT status, COALESCE(verified_wx_corpid, '')
		FROM mochat_go_tenant_corp_bindings
		WHERE tenant_id = ? AND corp_id = ?
		LIMIT 1 FOR UPDATE`, principal.TenantID, principal.CorpID).Scan(&bindingStatus, &verifiedCorpID); err != nil {
		return err
	}
	if bindingStatus != 2 || strings.TrimSpace(verifiedCorpID) == "" {
		return errors.New("company binding is not verified")
	}
	const safeError = `{"code":"SYNC_FAILED"}`
	var updateTimeID int
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_work_update_time
		WHERE corp_id = ? AND type = 1
		ORDER BY id DESC
		LIMIT 1 FOR UPDATE`, principal.CorpID).Scan(&updateTimeID)
	if errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO mc_work_update_time (corp_id, type, last_update_time, error_msg, created_at, updated_at)
			VALUES (?, 1, NULL, ?, NOW(), NOW())`, principal.CorpID, safeError)
		if insertErr != nil {
			return insertErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return errors.New("company sync failure status insert affected unexpected rows")
		}
	} else if err != nil {
		return err
	} else {
		result, updateErr := tx.ExecContext(ctx, `
			UPDATE mc_work_update_time
			SET error_msg = ?, updated_at = NOW()
			WHERE id = ? AND corp_id = ? AND type = 1`, safeError, updateTimeID, principal.CorpID)
		if updateErr != nil {
			return updateErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return errors.New("company sync failure status update affected unexpected rows")
		}
	}
	return tx.Commit()
}

func syncWorkEmployeesWithoutIdentityTx(ctx context.Context, tx *sql.Tx, credential dashboard.WorkEmployeeSyncCredential, departments map[int]workEmployeeSyncDepartmentRow, employees []dashboard.WorkEmployeeSyncEmployee) (dashboard.WorkEmployeeSyncResult, error) {
	var result dashboard.WorkEmployeeSyncResult
	employeeByWX := map[string]dashboard.WorkEmployeeSyncEmployee{}
	order := make([]string, 0, len(employees))
	for _, employee := range employees {
		wxUserID := strings.TrimSpace(employee.WXUserID)
		if wxUserID == "" {
			continue
		}
		employee.WXUserID = wxUserID
		if _, exists := employeeByWX[wxUserID]; !exists {
			order = append(order, wxUserID)
		}
		employeeByWX[wxUserID] = employee
	}
	if len(order) == 0 {
		return result, nil
	}
	existing, err := workEmployeeRowsByWX(ctx, tx, credential.CorpID)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	userIDsByPhone, _, err := workEmployeeSyncUsersByPhone(ctx, tx, credential.TenantID, employeeByWX)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	employeeIDs := make(map[string]int, len(order))
	for _, wxUserID := range order {
		employee := employeeByWX[wxUserID]
		mainDepartmentID := syncMainDepartmentID(employee, departments)
		logUserID := userIDsByPhone[strings.TrimSpace(employee.Mobile)]
		values := []any{wxUserID, credential.CorpID, employee.Name, employee.Mobile, employee.Position, employee.Gender, employee.Email, employee.Avatar, employee.ThumbAvatar, employee.Telephone, employee.Alias, jsonRawOrEmptyArray(employee.ExtAttr), employee.Status, employee.QRCode, jsonRawOrEmptyArray(employee.ExternalProfile), jsonRawOrEmptyArray(employee.ExternalPosition), employee.Address, employee.OpenUserID, employee.WXMainDepartmentID, mainDepartmentID, logUserID, 2}
		if employeeID, ok := existing[wxUserID]; ok {
			updateArgs := append([]any{}, values[2:]...)
			updateArgs = append(updateArgs, employeeID, credential.CorpID)
			updated, execErr := tx.ExecContext(ctx, `
				UPDATE mc_work_employee
				SET name = ?, mobile = ?, position = ?, gender = ?, email = ?, avatar = ?, thumb_avatar = ?, telephone = ?, alias = ?,
					extattr = ?, status = ?, qr_code = ?, external_profile = ?, external_position = ?, address = ?, open_user_id = ?,
					wx_main_department_id = ?, main_department_id = ?, log_user_id = ?, contact_auth = ?, updated_at = NOW()
				WHERE id = ? AND corp_id = ? AND deleted_at IS NULL`, updateArgs...)
			if execErr != nil {
				return dashboard.WorkEmployeeSyncResult{}, execErr
			}
			if rows, rowsErr := updated.RowsAffected(); rowsErr != nil || rows != 1 {
				return dashboard.WorkEmployeeSyncResult{}, errors.New("employee update affected unexpected rows")
			}
			employeeIDs[wxUserID] = employeeID
			result.EmployeesUpdated++
			continue
		}
		inserted, execErr := tx.ExecContext(ctx, `
			INSERT INTO mc_work_employee (
				wx_user_id, corp_id, name, mobile, position, gender, email, avatar, thumb_avatar, telephone, alias,
				extattr, status, qr_code, external_profile, external_position, address, open_user_id,
				wx_main_department_id, main_department_id, log_user_id, contact_auth, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`, values...)
		if execErr != nil {
			return dashboard.WorkEmployeeSyncResult{}, execErr
		}
		id, execErr := inserted.LastInsertId()
		if execErr != nil {
			return dashboard.WorkEmployeeSyncResult{}, execErr
		}
		employeeIDs[wxUserID] = int(id)
		result.EmployeesCreated++
	}
	relations, err := syncWorkEmployeeRelationsTx(ctx, tx, credential.CorpID, order, employeeByWX, employeeIDs, departments)
	if err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	result.RelationsCreated = relations.RelationsCreated
	result.RelationsUpdated = relations.RelationsUpdated
	result.RelationsDeleted = relations.RelationsDeleted
	return result, nil
}
