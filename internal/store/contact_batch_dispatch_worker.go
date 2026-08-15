package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

const contactBatchDispatchPageCode = "dashboard.acquisition.precise_group_send"

// ContactBatchDispatchDue returns only contact dispatches that the durable
// ledger says may be claimed now. The operation actor and current identity
// version are read with the same snapshot as the dispatch; a later authorizer
// transaction is still mandatory before an external request.
func (s *MySQLStore) ContactBatchDispatchDue(ctx context.Context, limit int) ([]dashboard.ContactBatchDispatchWorkItem, error) {
	if s == nil || s.db == nil || limit <= 0 {
		return nil, errors.New("contact batch dispatch store is unavailable")
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id,d.tenant_id,d.corp_id,d.operation_id,d.dispatch_kind,d.chunk_no,d.target_id,d.idempotency_key,d.status,
		       d.provider_request_id,d.provider_message_id,d.provider_object_id,d.credential_generation,d.lease_token,
		       d.lease_expires_at,d.attempt,d.next_poll_at,d.last_error_code,d.created_at,d.updated_at,
		       COALESCE(o.actor_user_id,0),COALESCE(identity.auth_version,0),COALESCE(actor.isSuperAdmin,0)
		FROM mochat_go_wecom_capability_dispatches d
		JOIN mochat_go_wecom_capability_operations o
		  ON o.tenant_id=d.tenant_id AND o.corp_id=d.corp_id AND o.id=d.operation_id
		JOIN mc_contact_message_batch_send batch
		  ON batch.tenant_id=d.tenant_id AND batch.corp_id=d.corp_id
		 AND batch.id=CAST(SUBSTRING_INDEX(o.request_id, ':', -1) AS UNSIGNED)
		 AND o.request_id LIKE 'contact-batch:%' AND batch.deleted_at IS NULL
		LEFT JOIN mochat_go_dashboard_identities identity
		  ON identity.user_id=o.actor_user_id AND identity.status=1
		LEFT JOIN mc_user actor
		  ON actor.tenant_id=o.tenant_id AND actor.id=o.actor_user_id AND actor.deleted_at IS NULL
		WHERE d.dispatch_kind=?
		  AND (batch.send_way=1 OR (batch.send_way=2 AND batch.definite_time IS NOT NULL AND batch.definite_time <= NOW(6)))
		  AND (
			d.status=?
			OR (d.status IN (?,?) AND d.last_error_code IN (?,?) AND d.next_poll_at IS NOT NULL AND d.next_poll_at <= NOW(6))
			OR (d.status IN (?,?,?,?) AND (d.lease_expires_at IS NULL OR d.lease_expires_at <= NOW(6))
				AND (d.next_poll_at IS NULL OR d.next_poll_at <= NOW(6)))
		)
		ORDER BY d.id ASC
		LIMIT ?`,
		string(wecomcapability.DispatchKindContactBatch), wecomcapability.DispatchQueued,
		wecomcapability.DispatchFailed, wecomcapability.DispatchPartialFailed, "wecom.http_429", "wecom.http_500",
		wecomcapability.DispatchClaimed, wecomcapability.DispatchSubmitting, wecomcapability.DispatchSubmitted, wecomcapability.DispatchPolling,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactBatchDispatchWorkItem, 0)
	for rows.Next() {
		item, actorID, authVersion, superadmin, err := scanContactBatchDue(rows)
		if err != nil {
			return nil, err
		}
		item.Principal = dashboardprincipal.DashboardPrincipal{
			UserID: actorID, TenantID: item.Dispatch.TenantID, CorpID: item.Dispatch.CorpID,
			CorpStatus: dashboardprincipal.CorpBindingStatusActive, IsSuperAdmin: superadmin == 1,
			AuthVersion: uint64(authVersion),
		}
		if item.Principal.UserID <= 0 {
			item.Principal.AuthVersion = 1
		}
		item.CredentialVersion = item.Dispatch.CredentialVersion
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func scanContactBatchDue(scanner interface{ Scan(...any) error }) (dashboard.ContactBatchDispatchWorkItem, int, int64, int, error) {
	var item dashboard.ContactBatchDispatchWorkItem
	var dispatch wecomcapability.Dispatch
	var leaseExpires, nextPoll, created, updated sql.NullTime
	var actorID, superadmin int
	var authVersion int64
	if err := scanner.Scan(
		&dispatch.ID, &dispatch.TenantID, &dispatch.CorpID, &dispatch.OperationID, &dispatch.DispatchKind, &dispatch.ChunkNo,
		&dispatch.TargetID, &dispatch.IdempotencyKey, &dispatch.Status, &dispatch.ProviderRequestID, &dispatch.ProviderMessageID,
		&dispatch.ProviderObjectID, &dispatch.CredentialVersion, &dispatch.LeaseToken, &leaseExpires, &dispatch.Attempt,
		&nextPoll, &dispatch.LastErrorCode, &created, &updated, &actorID, &authVersion, &superadmin,
	); err != nil {
		return dashboard.ContactBatchDispatchWorkItem{}, 0, 0, 0, err
	}
	dispatch.LeaseExpiresAt, dispatch.NextPollAt = nullableTimePtr(leaseExpires), nullableTimePtr(nextPoll)
	dispatch.CreatedAt, dispatch.UpdatedAt = nullableTimePtr(created), nullableTimePtr(updated)
	item.Dispatch = dispatch
	return item, actorID, authVersion, superadmin, nil
}

// AuthorizeDispatch is deliberately implemented by the production store,
// not by a creation-time snapshot. Every preclaim and postclaim therefore
// rechecks the current binding, actor identity, page grant, data scope and
// persisted customer ownership before the sender is reached.
func (s *MySQLStore) AuthorizeDispatch(ctx context.Context, request wecomcapability.DispatchAuthorizationRequest) error {
	if s == nil || s.db == nil || request.Principal.UserID <= 0 || request.Principal.TenantID <= 0 || request.Principal.CorpID <= 0 || request.Principal.AuthVersion == 0 || request.DispatchID <= 0 {
		return dashboard.ErrContactBatchCapabilityLimited
	}
	if request.Capability == wecomcapability.RoomBatchSend {
		return s.authorizeRoomBatchDispatch(ctx, request)
	}
	if request.Capability != wecomcapability.ContactBatchSend {
		return dashboard.ErrContactBatchCapabilityLimited
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	dispatch, err := queryCapabilityDispatchTx(ctx, tx, request.Principal.TenantID, request.Principal.CorpID, request.DispatchID, true)
	if err != nil {
		return err
	}
	if dispatch.DispatchKind != string(wecomcapability.DispatchKindContactBatch) || !wecomcapability.DispatchKindMatchesCapability(request.Capability, wecomcapability.DispatchKind(dispatch.DispatchKind)) {
		return dashboard.ErrContactBatchCapabilityLimited
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, request.Principal.TenantID, request.Principal.CorpID, dispatch.OperationID, true)
	if err != nil {
		return err
	}
	if operation.Capability != wecomcapability.ContactBatchSend || operation.ActorUserID != request.Principal.UserID || !capabilityOperationAllowsDispatch(operation.Status) {
		return dashboard.ErrContactBatchCapabilityLimited
	}
	tenantAccess, err := s.dashboardTenantAccessTx(ctx, tx, request.Principal.TenantID, s.nowForContactBatch())
	if err != nil {
		return err
	}
	if !tenantAccess.Allowed {
		return dashboard.ErrContactBatchTenantDenied
	}
	quota, err := s.contactBatchQuotaStatusTx(ctx, tx, request.Principal.TenantID, 0)
	if err != nil {
		return err
	}
	if quota.Limit > 0 && quota.Current > quota.Limit {
		return dashboard.ErrContactBatchQuotaExceeded
	}
	facts, err := s.checkContactBatchActor(ctx, tx, dashboardprincipal.DashboardPrincipal{
		UserID: request.Principal.UserID, TenantID: request.Principal.TenantID, CorpID: request.Principal.CorpID,
		AuthVersion: request.Principal.AuthVersion, IsSuperAdmin: request.Principal.IsSuperAdmin,
	}, true)
	if err != nil {
		return err
	}
	principal := dashboardprincipal.DashboardPrincipal{
		UserID: request.Principal.UserID, TenantID: request.Principal.TenantID, CorpID: request.Principal.CorpID,
		AuthVersion: request.Principal.AuthVersion, IsSuperAdmin: facts.IsSuperAdmin == 1,
	}
	scope, err := s.contactBatchCurrentScopeTx(ctx, tx, principal)
	if err != nil {
		return err
	}
	if !scope.Allowed {
		return dashboard.ErrContactBatchPermissionDenied
	}
	if err := s.authorizeContactBatchDispatchTargetTx(ctx, tx, principal, dispatch, scope); err != nil {
		return err
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return err
	}
	generation := credentialGenerationForCapability(binding, operation.Capability)
	credential, found, err := loadEncryptedCorpCredentialByID(ctx, tx, binding.CorpID, true)
	if err != nil {
		return err
	}
	if !found {
		return dashboard.ErrContactBatchCapabilityLimited
	}
	secret, err := s.decodeEncryptedCorpCredential(credential)
	if err != nil {
		return dashboard.ErrContactBatchCapabilityLimited
	}
	if generation == 0 || dispatch.CredentialVersion != generation || operation.CredentialVersion != generation || request.ExpectedCredentialVersion != generation || binding.Status != 2 || strings.TrimSpace(binding.VerifiedWXCorpID) == "" || strings.TrimSpace(secret.ContactSecret) == "" {
		return dashboard.ErrContactBatchCapabilityLimited
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

type contactBatchCurrentScope struct {
	Allowed            bool
	Scope              dashboard.DataScope
	AllowedEmployeeIDs []int
}

func (s *MySQLStore) contactBatchCurrentScopeTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal) (contactBatchCurrentScope, error) {
	var isSuperadmin int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(isSuperAdmin,0) FROM mc_user WHERE tenant_id=? AND id=? AND deleted_at IS NULL`, principal.TenantID, principal.UserID).Scan(&isSuperadmin); err != nil {
		return contactBatchCurrentScope{}, err
	}
	if isSuperadmin == 1 {
		return contactBatchCurrentScope{Allowed: true, Scope: dashboard.DataScopeTenant}, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT direct_permission.data_scope
		FROM mochat_go_dashboard_user_permissions direct_permission
		JOIN mochat_go_dashboard_permissions permission ON permission.id=direct_permission.permission_id
		WHERE direct_permission.tenant_id=? AND direct_permission.user_id=? AND direct_permission.effect='allow'
		  AND permission.code=? AND permission.status=1 AND permission.deleted_at IS NULL AND permission.superadmin_only=0
		UNION ALL
		SELECT role_permission.data_scope
		FROM mochat_go_dashboard_user_roles user_role
		JOIN mc_rbac_role role ON role.tenant_id=user_role.tenant_id AND role.id=user_role.role_id AND role.status=1 AND role.deleted_at IS NULL
		JOIN mochat_go_dashboard_role_permissions role_permission ON role_permission.tenant_id=user_role.tenant_id AND role_permission.role_id=user_role.role_id
		JOIN mochat_go_dashboard_permissions permission ON permission.id=role_permission.permission_id
		WHERE user_role.tenant_id=? AND user_role.user_id=? AND permission.code=? AND permission.status=1 AND permission.deleted_at IS NULL AND permission.superadmin_only=0`,
		principal.TenantID, principal.UserID, contactBatchDispatchPageCode, principal.TenantID, principal.UserID, contactBatchDispatchPageCode)
	if err != nil {
		return contactBatchCurrentScope{}, err
	}
	defer rows.Close()
	scope := dashboard.DataScope("")
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return contactBatchCurrentScope{}, err
		}
		scope = mergeContactBatchScope(scope, dashboard.DataScope(strings.TrimSpace(raw)))
	}
	if err := rows.Err(); err != nil {
		return contactBatchCurrentScope{}, err
	}
	if scope == "" {
		return contactBatchCurrentScope{}, nil
	}
	result := contactBatchCurrentScope{Allowed: true, Scope: scope}
	if scope == dashboard.DataScopeTenant {
		return result, nil
	}
	if scope == dashboard.DataScopeSelf {
		employeeRows, err := tx.QueryContext(ctx, `SELECT id FROM mc_work_employee WHERE corp_id=? AND log_user_id=? AND deleted_at IS NULL`, principal.CorpID, principal.UserID)
		if err != nil {
			return contactBatchCurrentScope{}, err
		}
		defer employeeRows.Close()
		for employeeRows.Next() {
			var employeeID int
			if err := employeeRows.Scan(&employeeID); err != nil {
				return contactBatchCurrentScope{}, err
			}
			result.AllowedEmployeeIDs = append(result.AllowedEmployeeIDs, employeeID)
		}
		return result, employeeRows.Err()
	}
	rows, err = tx.QueryContext(ctx, `
		SELECT DISTINCT scoped_employee.id
		FROM mc_work_employee owner_employee
		JOIN mc_work_employee_department owner_membership ON owner_membership.employee_id=owner_employee.id AND owner_membership.deleted_at IS NULL
		JOIN mc_work_department owner_department ON owner_department.id=owner_membership.department_id AND owner_department.corp_id=? AND owner_department.deleted_at IS NULL
		JOIN mc_work_department scoped_department ON scoped_department.corp_id=? AND scoped_department.deleted_at IS NULL AND scoped_department.path LIKE CONCAT(owner_department.path,'%')
		JOIN mc_work_employee_department scoped_membership ON scoped_membership.department_id=scoped_department.id AND scoped_membership.deleted_at IS NULL
		JOIN mc_work_employee scoped_employee ON scoped_employee.id=scoped_membership.employee_id AND scoped_employee.corp_id=? AND scoped_employee.deleted_at IS NULL
		WHERE owner_employee.corp_id=? AND owner_employee.log_user_id=? AND owner_employee.deleted_at IS NULL`,
		principal.CorpID, principal.CorpID, principal.CorpID, principal.CorpID, principal.UserID)
	if err != nil {
		return contactBatchCurrentScope{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var employeeID int
		if err := rows.Scan(&employeeID); err != nil {
			return contactBatchCurrentScope{}, err
		}
		result.AllowedEmployeeIDs = appendUniqueInt(result.AllowedEmployeeIDs, employeeID)
	}
	return result, rows.Err()
}

func mergeContactBatchScope(current, candidate dashboard.DataScope) dashboard.DataScope {
	if candidate != dashboard.DataScopeSelf && candidate != dashboard.DataScopeDepartment && candidate != dashboard.DataScopeTenant {
		return current
	}
	if current == dashboard.DataScopeTenant || candidate == dashboard.DataScopeTenant {
		return dashboard.DataScopeTenant
	}
	if current == dashboard.DataScopeDepartment || candidate == dashboard.DataScopeDepartment {
		return dashboard.DataScopeDepartment
	}
	return candidate
}

func appendUniqueInt(values []int, value int) []int {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (s *MySQLStore) authorizeContactBatchDispatchTargetTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, dispatch wecomcapability.Dispatch, scope contactBatchCurrentScope) error {
	target, ok := parseContactBatchDispatchTarget(dispatch.TargetID)
	if !ok {
		return dashboard.ErrContactBatchTargetNotOwned
	}
	if scope.Scope != dashboard.DataScopeTenant && !containsContactBatchEmployeeInt(scope.AllowedEmployeeIDs, target.EmployeeID) {
		return dashboard.ErrContactBatchTargetNotOwned
	}
	var employeeRows int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mc_contact_message_batch_send_employee employee_batch
		JOIN mc_contact_message_batch_send batch ON batch.id=employee_batch.batch_id AND batch.tenant_id=? AND batch.corp_id=? AND batch.deleted_at IS NULL
		WHERE employee_batch.batch_id=? AND employee_batch.employee_id=?`, principal.TenantID, principal.CorpID, target.BatchID, target.EmployeeID).Scan(&employeeRows); err != nil {
		return err
	}
	if employeeRows != 1 {
		return dashboard.ErrContactBatchTargetNotOwned
	}
	var unowned int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_contact_message_batch_send_result result
		JOIN mc_contact_message_batch_send batch ON batch.id=result.batch_id AND batch.tenant_id=? AND batch.corp_id=? AND batch.deleted_at IS NULL
		WHERE result.batch_id=? AND result.employee_id=?
		  AND NOT EXISTS (
			SELECT 1 FROM mc_work_contact contact
			JOIN mc_work_contact_employee membership ON membership.contact_id=contact.id AND membership.employee_id=? AND membership.corp_id=? AND membership.deleted_at IS NULL
			WHERE contact.corp_id=? AND contact.wx_external_userid=result.external_user_id AND contact.deleted_at IS NULL
		  )`, principal.TenantID, principal.CorpID, target.BatchID, target.EmployeeID, target.EmployeeID, principal.CorpID, principal.CorpID).Scan(&unowned); err != nil {
		return err
	}
	if unowned != 0 {
		return dashboard.ErrContactBatchTargetNotOwned
	}
	return nil
}

func containsContactBatchEmployeeInt(values []int, wanted int) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
