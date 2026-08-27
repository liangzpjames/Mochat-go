package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
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

const (
	companySyncStateQueued    = "SYNC_QUEUED"
	companySyncStateRunning   = "SYNC_RUNNING"
	companySyncStateFailed    = "SYNC_FAILED"
	companySyncStateCompleted = "SYNC_COMPLETED"
)

type companySyncStateMarker struct {
	Code              string `json:"code"`
	Cursor            string `json:"cursor,omitempty"`
	ErrorCode         string `json:"errorCode,omitempty"`
	CredentialVersion uint64 `json:"credentialVersion,omitempty"`
	QueueTicket       string `json:"queueTicket,omitempty"`
}

func companySyncStateJSON(code string) string {
	return companySyncStateJSONWithError(code, "")
}

func companySyncStateJSONWithError(code, errorCode string) string {
	return companySyncStateJSONWithVersion(code, errorCode, 0)
}

func companySyncStateJSONWithVersion(code, errorCode string, credentialVersion uint64) string {
	return companySyncStateJSONWithVersionAndTicket(code, errorCode, credentialVersion, "")
}

func companySyncStateJSONWithVersionAndTicket(code, errorCode string, credentialVersion uint64, queueTicket string) string {
	marker := companySyncStateMarker{Code: code, CredentialVersion: credentialVersion, QueueTicket: strings.TrimSpace(queueTicket)}
	if code == companySyncStateQueued || code == companySyncStateRunning || code == companySyncStateFailed || code == companySyncStateCompleted {
		marker.Cursor = dashboard.CompanyEmployeeSyncCursor
	}
	if errorCode == "SYNC_FAILED" {
		marker.ErrorCode = errorCode
	}
	raw, _ := json.Marshal(marker)
	return string(raw)
}

func decodeCompanySyncState(raw string) companySyncStateMarker {
	var marker companySyncStateMarker
	if err := json.Unmarshal([]byte(raw), &marker); err != nil {
		return companySyncStateMarker{Code: companySyncStateFailed}
	}
	if marker.Code != companySyncStateQueued && marker.Code != companySyncStateRunning && marker.Code != companySyncStateFailed && marker.Code != companySyncStateCompleted {
		return companySyncStateMarker{Code: companySyncStateFailed}
	}
	if marker.ErrorCode != "" && marker.ErrorCode != "SYNC_FAILED" {
		marker.ErrorCode = ""
	}
	if marker.Cursor != dashboard.CompanyEmployeeSyncCursor {
		marker.Cursor = ""
	}
	return marker
}

type companySyncBinding struct {
	TenantID int
	CorpID   int
	Status   int
	WXCorpID string
	Version  uint64
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
		SELECT tenant_id, corp_id, status, COALESCE(verified_wx_corpid, ''), version
		FROM mochat_go_tenant_corp_bindings
		WHERE tenant_id = ?`+suffix, tenantID)
	if err != nil {
		return companySyncBinding{}, err
	}
	bindings := make([]companySyncBinding, 0, 2)
	for rows.Next() {
		var binding companySyncBinding
		if err := rows.Scan(&binding.TenantID, &binding.CorpID, &binding.Status, &binding.WXCorpID, &binding.Version); err != nil {
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
		CorpID: item.ID, TenantID: binding.TenantID, CredentialVersion: binding.Version, WXCorpID: binding.WXCorpID,
		EmployeeSecret: secret.EmployeeSecret, ContactSecret: secret.ContactSecret,
	}}, nil
}

func (s *MySQLStore) SyncCompanyEmployees(ctx context.Context, bindingID int, departments []dashboard.WorkEmployeeSyncDepartment, employees []dashboard.WorkEmployeeSyncEmployee) (dashboard.WorkEmployeeSyncResult, error) {
	return s.syncCompanyEmployeesTx(ctx, bindingID, 0, 0, 0, "", departments, employees)
}

func (s *MySQLStore) SyncCompanyEmployeesAtVersion(ctx context.Context, bindingID int, credentialVersion uint64, queueTicket string, departments []dashboard.WorkEmployeeSyncDepartment, employees []dashboard.WorkEmployeeSyncEmployee) (dashboard.WorkEmployeeSyncResult, error) {
	return s.syncCompanyEmployeesTx(ctx, bindingID, 0, 0, credentialVersion, queueTicket, departments, employees)
}

func (s *MySQLStore) QueueEmployeeSync(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, receipt companyprofile.EmployeeSyncEnqueueReceipt) (companyprofile.EmployeeSyncQueueResult, error) {
	if s == nil || s.db == nil || principal.TenantID <= 0 || principal.CorpID <= 0 || principal.UserID <= 0 {
		return companyprofile.EmployeeSyncQueueResult{}, companyprofile.ErrPermissionDenied
	}
	if strings.TrimSpace(receipt.Ticket) == "" {
		return companyprofile.EmployeeSyncQueueResult{}, errors.New("company sync queue ticket unavailable")
	}
	if _, err := parseCompanySyncQueueTicket(receipt.Ticket); err != nil {
		return companyprofile.EmployeeSyncQueueResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return companyprofile.EmployeeSyncQueueResult{}, err
	}
	defer rollbackQuietly(tx)
	if err := s.checkCompanyActor(ctx, tx, principal, true); err != nil {
		return companyprofile.EmployeeSyncQueueResult{}, err
	}
	binding, err := loadSingleCompanySyncBinding(ctx, tx, principal.TenantID, true)
	if errors.Is(err, errCompanyBindingNotFound) {
		return companyprofile.EmployeeSyncQueueResult{}, companyprofile.ErrNotFound
	}
	if errors.Is(err, errCompanyBindingNotVerified) || errors.Is(err, errCompanyBindingNotUnique) {
		return companyprofile.EmployeeSyncQueueResult{}, companyprofile.ErrTenantAccessDenied
	}
	if err != nil {
		return companyprofile.EmployeeSyncQueueResult{}, err
	}
	if binding.CorpID != principal.CorpID {
		return companyprofile.EmployeeSyncQueueResult{}, companyprofile.ErrNotFound
	}
	result, err := queueCompanySyncStateTx(ctx, tx, binding.CorpID, binding.Version, receipt)
	if err != nil {
		return companyprofile.EmployeeSyncQueueResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return companyprofile.EmployeeSyncQueueResult{}, err
	}
	return result, nil
}

func (s *MySQLStore) BeginCompanyEmployeeSync(ctx context.Context, bindingID int) error {
	return s.updateCompanyEmployeeSyncState(ctx, bindingID, 0, "", companySyncStateRunning, "")
}

func (s *MySQLStore) BeginCompanyEmployeeSyncAtVersion(ctx context.Context, bindingID int, credentialVersion uint64, queueTicket string) error {
	if credentialVersion == 0 {
		return errors.New("company credential version unavailable")
	}
	return s.updateCompanyEmployeeSyncState(ctx, bindingID, credentialVersion, queueTicket, companySyncStateRunning, "")
}

func (s *MySQLStore) MarkCompanyEmployeeSyncQueuedAtVersion(ctx context.Context, bindingID int, credentialVersion uint64, queueTicket string, errorCode string) error {
	if credentialVersion == 0 {
		return errors.New("company credential version unavailable")
	}
	return s.updateCompanyEmployeeSyncState(ctx, bindingID, credentialVersion, queueTicket, companySyncStateQueued, errorCode)
}

func (s *MySQLStore) RecordCompanyEmployeeSyncFailureAtVersion(ctx context.Context, bindingID int, credentialVersion uint64, queueTicket string) error {
	if credentialVersion == 0 {
		return errors.New("company credential version unavailable")
	}
	return s.updateCompanyEmployeeSyncState(ctx, bindingID, credentialVersion, queueTicket, companySyncStateFailed, "")
}

func (s *MySQLStore) MarkCompanyEmployeeSyncQueued(ctx context.Context, bindingID int, errorCode string) error {
	return s.updateCompanyEmployeeSyncState(ctx, bindingID, 0, "", companySyncStateQueued, errorCode)
}

func (s *MySQLStore) RecordCompanyEmployeeSyncFailure(ctx context.Context, bindingID int) error {
	return s.updateCompanyEmployeeSyncState(ctx, bindingID, 0, "", companySyncStateFailed, "")
}

func (s *MySQLStore) updateCompanyEmployeeSyncState(ctx context.Context, bindingID int, expectedVersion uint64, queueTicket string, state, errorCode string) error {
	if s == nil || s.db == nil || bindingID <= 0 {
		return errors.New("company binding unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	binding, err := loadSingleCompanySyncBinding(ctx, tx, bindingID, true)
	if err != nil {
		return err
	}
	if expectedVersion != 0 && binding.Version != expectedVersion {
		return errors.New("company credential version stale")
	}
	if expectedVersion != 0 && strings.TrimSpace(queueTicket) == "" {
		return errors.New("company queue ticket unavailable")
	}
	if strings.TrimSpace(queueTicket) != "" {
		if _, err := parseCompanySyncQueueTicket(queueTicket); err != nil {
			return err
		}
	}
	if err := setCompanySyncStateTx(ctx, tx, binding.CorpID, binding.Version, queueTicket, state, errorCode); err != nil {
		return err
	}
	return tx.Commit()
}

func queueCompanySyncStateTx(ctx context.Context, tx *sql.Tx, corpID int, credentialVersion uint64, receipt companyprofile.EmployeeSyncEnqueueReceipt) (companyprofile.EmployeeSyncQueueResult, error) {
	if corpID <= 0 {
		return companyprofile.EmployeeSyncQueueResult{}, errors.New("company binding unavailable")
	}
	if strings.TrimSpace(receipt.Ticket) == "" {
		return companyprofile.EmployeeSyncQueueResult{}, errors.New("company sync queue ticket unavailable")
	}
	if _, err := parseCompanySyncQueueTicket(receipt.Ticket); err != nil {
		return companyprofile.EmployeeSyncQueueResult{}, err
	}
	var updateTimeID int
	var errorMessage sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(CAST(error_msg AS CHAR), '')
		FROM mc_work_update_time
		WHERE corp_id = ? AND type = 1
		ORDER BY id DESC LIMIT 1 FOR UPDATE`, corpID).Scan(&updateTimeID, &errorMessage)
	if errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO mc_work_update_time (corp_id, type, last_update_time, error_msg, created_at, updated_at)
			VALUES (?, 1, NULL, ?, NOW(), NOW())`, corpID, companySyncStateJSONWithVersionAndTicket(companySyncStateQueued, "", credentialVersion, receipt.Ticket))
		if insertErr != nil {
			return companyprofile.EmployeeSyncQueueResult{}, insertErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return companyprofile.EmployeeSyncQueueResult{}, errors.New("company sync queue insert affected unexpected rows")
		}
		return companyprofile.EmployeeSyncQueueResult{Cursor: dashboard.CompanyEmployeeSyncCursor}, nil
	}
	if err != nil {
		return companyprofile.EmployeeSyncQueueResult{}, err
	}
	marker := decodeCompanySyncState(errorMessage.String)
	ticketRelation, err := compareCompanySyncQueueTickets(marker.QueueTicket, receipt.Ticket)
	if err != nil {
		return companyprofile.EmployeeSyncQueueResult{}, err
	}
	if ticketRelation > 0 {
		// A delayed request must never replace a newer Redis claim, regardless
		// of whether the newer worker is queued, running, completed, or failed.
		return companyprofile.EmployeeSyncQueueResult{Cursor: marker.Cursor, AlreadyQueued: true}, nil
	}
	if companySyncMarkerAlreadyQueued(marker, credentialVersion, receipt.Ticket) {
		return companyprofile.EmployeeSyncQueueResult{Cursor: marker.Cursor, AlreadyQueued: true}, nil
	}
	if shouldPreserveCompletedCompanySyncMarker(marker, receipt.Ticket, credentialVersion) {
		// Redis is authoritative. Preserve only the completion written by this
		// exact enqueue ticket; old/legacy markers are safe to replace.
		return companyprofile.EmployeeSyncQueueResult{Cursor: dashboard.CompanyEmployeeSyncCursor, AlreadyQueued: true}, nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_update_time
		SET error_msg = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND type = 1`, companySyncStateJSONWithVersionAndTicket(companySyncStateQueued, "", credentialVersion, receipt.Ticket), updateTimeID, corpID)
	if err != nil {
		return companyprofile.EmployeeSyncQueueResult{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return companyprofile.EmployeeSyncQueueResult{}, errors.New("company sync queue update affected unexpected rows")
	}
	return companyprofile.EmployeeSyncQueueResult{Cursor: dashboard.CompanyEmployeeSyncCursor}, nil
}

func shouldPreserveCompletedCompanySyncMarker(marker companySyncStateMarker, queueTicket string, currentVersion uint64) bool {
	if marker.Code != companySyncStateCompleted || currentVersion == 0 || marker.CredentialVersion != currentVersion || strings.TrimSpace(queueTicket) == "" {
		return false
	}
	relation, err := compareCompanySyncQueueTickets(marker.QueueTicket, queueTicket)
	return err == nil && relation == 0
}

func companySyncMarkerAlreadyQueued(marker companySyncStateMarker, currentVersion uint64, queueTicket string) bool {
	if currentVersion == 0 || marker.CredentialVersion != currentVersion {
		return false
	}
	if marker.Code != companySyncStateQueued && marker.Code != companySyncStateRunning {
		return false
	}
	relation, err := compareCompanySyncQueueTickets(marker.QueueTicket, queueTicket)
	return err == nil && relation == 0
}

func parseCompanySyncQueueTicket(ticket string) (uint64, error) {
	trimmed := strings.TrimSpace(ticket)
	if trimmed == "" {
		return 0, errors.New("company sync queue ticket unavailable")
	}
	value, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("company sync queue ticket invalid")
	}
	return value, nil
}

// compareCompanySyncQueueTickets reports the relation of existing to incoming:
// -1 means the existing marker is older, 0 means the same claim, and 1 means
// the marker is newer. Empty existing markers are legacy and older than a
// numeric queue claim; a legacy empty receipt may not replace a numeric claim.
func compareCompanySyncQueueTickets(existing, incoming string) (int, error) {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if existing == "" && incoming == "" {
		return 0, nil
	}
	if existing == "" {
		if _, err := parseCompanySyncQueueTicket(incoming); err != nil {
			return 0, err
		}
		return -1, nil
	}
	if incoming == "" {
		return 0, errors.New("company sync queue ticket stale")
	}
	existingValue, err := parseCompanySyncQueueTicket(existing)
	if err != nil {
		return 0, err
	}
	incomingValue, err := parseCompanySyncQueueTicket(incoming)
	if err != nil {
		return 0, err
	}
	switch {
	case existingValue > incomingValue:
		return 1, nil
	case existingValue < incomingValue:
		return -1, nil
	default:
		return 0, nil
	}
}

func companySyncStatusMarkerStale(status string, markerVersion, currentVersion uint64) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "completed", "succeeded", "success", "queued", "syncing", "running", "failed":
		return markerVersion == 0 || currentVersion == 0 || markerVersion != currentVersion
	case "stale":
		return true
	default:
		return false
	}
}

func setCompanySyncStateTx(ctx context.Context, tx *sql.Tx, corpID int, credentialVersion uint64, queueTicket string, state, errorCode string) error {
	if corpID <= 0 || (state != companySyncStateQueued && state != companySyncStateRunning && state != companySyncStateFailed) {
		return errors.New("company sync state unavailable")
	}
	if errorCode != "" && errorCode != "SYNC_FAILED" {
		return errors.New("company sync error code unavailable")
	}
	if strings.TrimSpace(queueTicket) != "" {
		if _, err := parseCompanySyncQueueTicket(queueTicket); err != nil {
			return err
		}
	}
	var updateTimeID int
	var existingMessage sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(CAST(error_msg AS CHAR), '')
		FROM mc_work_update_time
		WHERE corp_id = ? AND type = 1
		ORDER BY id DESC LIMIT 1 FOR UPDATE`, corpID).Scan(&updateTimeID, &existingMessage)
	if errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO mc_work_update_time (corp_id, type, last_update_time, error_msg, created_at, updated_at)
			VALUES (?, 1, NULL, ?, NOW(), NOW())`, corpID, companySyncStateJSONWithVersionAndTicket(state, errorCode, credentialVersion, queueTicket))
		if insertErr != nil {
			return insertErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return errors.New("company sync state insert affected unexpected rows")
		}
		return nil
	}
	if err != nil {
		return err
	}
	existingMarker := decodeCompanySyncState(existingMessage.String)
	ticketRelation, err := compareCompanySyncQueueTickets(existingMarker.QueueTicket, queueTicket)
	if err != nil {
		return err
	}
	if ticketRelation > 0 {
		return errors.New("company sync queue ticket stale")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_update_time
		SET error_msg = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND type = 1`, companySyncStateJSONWithVersionAndTicket(state, errorCode, credentialVersion, queueTicket), updateTimeID, corpID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return errors.New("company sync state update affected unexpected rows")
	}
	return nil
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
	result, err := s.syncCompanyEmployeesTx(ctx, principal.TenantID, principal.UserID, principal.AuthVersion, 0, "", departments, employees)
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
	var bindingVersion uint64
	if err := s.db.QueryRowContext(ctx, `
		SELECT b.corp_id, b.status, COALESCE(b.verified_wx_corpid,''), b.version
		FROM mochat_go_tenant_corp_bindings b
		WHERE b.tenant_id = ? AND b.corp_id = ? LIMIT 1`, principal.TenantID, principal.CorpID).Scan(&corpID, &status, &verified, &bindingVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return companyprofile.SyncStatus{}, companyprofile.ErrNotFound
		}
		return companyprofile.SyncStatus{}, err
	}
	// A newly provisioned tenant owns a pending binding before any WeCom
	// credentials are configured. Reading its sync status is a harmless
	// Dashboard query and must remain available as an empty state; only the
	// actual sync mutation requires a verified active binding.
	if status == 1 && strings.TrimSpace(verified) == "" {
		return companyprofile.SyncStatus{Status: "idle"}, nil
	}
	if status != 2 || strings.TrimSpace(verified) == "" {
		return companyprofile.SyncStatus{}, companyprofile.ErrTenantAccessDenied
	}
	var lastUpdate sql.NullTime
	var errorMessage sql.NullString
	var updatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT last_update_time, COALESCE(CAST(error_msg AS CHAR), ''), updated_at
		FROM mc_work_update_time
		WHERE corp_id = ? AND type = 1
		ORDER BY id DESC LIMIT 1`, corpID).Scan(&lastUpdate, &errorMessage, &updatedAt)
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
	if companySyncStatusMarkerStale(statusResult.Status, statusResult.CredentialVersion, bindingVersion) {
		statusResult.Status = "stale"
		statusResult.ErrorCode = "wecom.sync_stale"
		statusResult.CredentialVersion = 0
	}
	if updatedAt.Valid && (statusResult.Status == "queued" || statusResult.Status == "syncing" || statusResult.Status == "failed") {
		value := updatedAt.Time
		statusResult.StartedAt = &value
	}
	if lastUpdate.Valid {
		value := lastUpdate.Time
		statusResult.FinishedAt = &value
	}
	return statusResult, nil
}

func (s *MySQLStore) syncCompanyEmployeesTx(ctx context.Context, bindingID, actorUserID int, actorAuthVersion, expectedCredentialVersion uint64, queueTicket string, departments []dashboard.WorkEmployeeSyncDepartment, employees []dashboard.WorkEmployeeSyncEmployee) (dashboard.WorkEmployeeSyncResult, error) {
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
	if expectedCredentialVersion != 0 && binding.Version != expectedCredentialVersion {
		return dashboard.WorkEmployeeSyncResult{}, errors.New("company credential version stale")
	}
	if expectedCredentialVersion != 0 && strings.TrimSpace(queueTicket) == "" {
		return dashboard.WorkEmployeeSyncResult{}, errors.New("company queue ticket unavailable")
	}
	if strings.TrimSpace(queueTicket) != "" {
		if _, err := parseCompanySyncQueueTicket(queueTicket); err != nil {
			return dashboard.WorkEmployeeSyncResult{}, err
		}
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
	if err := upsertCompanySyncUpdateTimeTx(ctx, tx, binding.CorpID, 1, binding.Version, queueTicket); err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WorkEmployeeSyncResult{}, err
	}
	return result, nil
}

func companySyncStatusFromRecord(lastUpdate sql.NullTime, errorMessage sql.NullString, departments, employees int) companyprofile.SyncStatus {
	status := companyprofile.SyncStatus{Status: "completed", Departments: departments, Employees: employees}
	if strings.TrimSpace(errorMessage.String) == "" {
		if !lastUpdate.Valid {
			status.Status = "idle"
		}
		return status
	}
	marker := decodeCompanySyncState(errorMessage.String)
	status.Cursor = marker.Cursor
	status.CredentialVersion = marker.CredentialVersion
	switch marker.Code {
	case companySyncStateQueued:
		status.Status = "queued"
		status.ErrorCode = marker.ErrorCode
	case companySyncStateRunning:
		status.Status = "syncing"
		status.ErrorCode = marker.ErrorCode
	case companySyncStateCompleted:
		status.Status = "completed"
	default:
		status.Status = "failed"
		status.ErrorCode = "SYNC_FAILED"
	}
	return status
}

func upsertCompanySyncUpdateTimeTx(ctx context.Context, tx *sql.Tx, corpID int, updateType int, credentialVersion uint64, queueTicket string) error {
	if strings.TrimSpace(queueTicket) != "" {
		if _, err := parseCompanySyncQueueTicket(queueTicket); err != nil {
			return err
		}
	}
	var updateTimeID int
	var existingMessage sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(CAST(error_msg AS CHAR), '')
		FROM mc_work_update_time
		WHERE corp_id = ? AND type = ?
		ORDER BY id DESC
		LIMIT 1
		FOR UPDATE
	`, corpID, updateType).Scan(&updateTimeID, &existingMessage)
	if errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO mc_work_update_time (corp_id, type, last_update_time, error_msg, created_at, updated_at)
			VALUES (?, ?, NOW(), ?, NOW(), NOW())
		`, corpID, updateType, companySyncStateJSONWithVersionAndTicket(companySyncStateCompleted, "", credentialVersion, queueTicket))
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
	existingMarker := decodeCompanySyncState(existingMessage.String)
	ticketRelation, err := compareCompanySyncQueueTickets(existingMarker.QueueTicket, queueTicket)
	if err != nil {
		return err
	}
	if ticketRelation > 0 {
		return errors.New("company sync queue ticket stale")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_work_update_time
		SET last_update_time = NOW(), error_msg = ?, updated_at = NOW()
		WHERE id = ? AND corp_id = ? AND type = ?
	`, companySyncStateJSONWithVersionAndTicket(companySyncStateCompleted, "", credentialVersion, queueTicket), updateTimeID, corpID, updateType)
	if err != nil {
		return err
	}
	return requireCompanySyncStatusRowsOrMatched(ctx, tx, result, updateTimeID, corpID, updateType)
}

func requireCompanySyncStatusRowsOrMatched(ctx context.Context, tx *sql.Tx, result sql.Result, updateTimeID, corpID, updateType int) error {
	if result == nil {
		return errors.New("company sync status update returned no result")
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 1 {
		return nil
	}
	if rows != 0 {
		return errors.New("company sync status update affected unexpected rows")
	}
	var matched int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_work_update_time
		WHERE id = ? AND corp_id = ? AND type = ?
		  AND last_update_time IS NOT NULL AND error_msg IS NULL`, updateTimeID, corpID, updateType).Scan(&matched); err == nil && matched == 1 {
		return nil
	}
	return errors.New("company sync status update affected unexpected rows")
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
	binding, err := loadSingleCompanySyncBinding(ctx, tx, principal.TenantID, true)
	if err != nil {
		return err
	}
	if binding.CorpID != principal.CorpID {
		return companyprofile.ErrNotFound
	}
	if err := setCompanySyncStateTx(ctx, tx, binding.CorpID, binding.Version, "", companySyncStateFailed, ""); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) RecordEmployeeSyncFailure(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) error {
	return s.recordCompanySyncFailure(ctx, principal)
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
