package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

const contactBatchDispatchChunkSize = 10000

// CreateContactBatchDispatch is the only production create entry for the
// durable contact-batch path. It deliberately does not call the legacy
// CreateContactMessageBatchSend method: the business row, operation,
// dispatch chunks, and their create audit/event are one transaction.
func (s *MySQLStore) CreateContactBatchDispatch(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, access dashboard.DashboardAccessContext, input dashboard.ContactBatchDispatchInput) (dashboard.ContactBatchDispatchResult, error) {
	if s == nil || s.db == nil || principal.TenantID <= 0 || principal.CorpID <= 0 || principal.UserID <= 0 || input.Batch.UserID != principal.UserID || input.Batch.CorpID != principal.CorpID || strings.TrimSpace(input.IdempotencyKey) == "" {
		return dashboard.ContactBatchDispatchResult{}, companyprofile.ErrInvalidRequest
	}
	if !principal.IsSuperAdmin && !dashboardPermissionCodeContains(access.PermissionCodes, "dashboard.acquisition.precise_group_send") {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchPermissionDenied
	}
	if !contactBatchAccessIdentityMatches(access, principal) {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchBodyScope
	}
	if access.ScopeRequired && access.Scope != dashboard.DataScopeTenant && len(access.AllowedEmployeeIDs) == 0 {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchTargetNotOwned
	}
	if len(input.Batch.EmployeeIDs) == 0 || len(input.ContactTargets) == 0 || input.SenderEmployeeID <= 0 {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchTargetNotOwned
	}
	if !validContactBatchStoreToken(input.IdempotencyKey, 128) || !validContactBatchStoreToken(input.RequestID, 255) {
		return dashboard.ContactBatchDispatchResult{}, companyprofile.ErrInvalidRequest
	}
	if access.ScopeRequired && access.Scope != dashboard.DataScopeTenant && !dashboardEmployeeIDsWithinScope(input.Batch.EmployeeIDs, access.AllowedEmployeeIDs) {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchTargetNotOwned
	}
	if access.ScopeRequired && access.Scope != dashboard.DataScopeTenant && !dashboardEmployeeIDsWithinScope([]int{input.SenderEmployeeID}, access.AllowedEmployeeIDs) {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchTargetNotOwned
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, companyprofile.ErrStoreUnavailable
	}
	defer rollbackQuietly(tx)
	tenantAccess, err := s.dashboardTenantAccessTx(ctx, tx, principal.TenantID, s.nowForContactBatch())
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	if !tenantAccess.Allowed {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchTenantDenied
	}
	quota, err := s.contactBatchQuotaStatusTx(ctx, tx, principal.TenantID, 1)
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	if quota.Limit > 0 && quota.Current+quota.Additional > quota.Limit {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchQuotaExceeded
	}
	actorFacts, err := s.checkContactBatchActor(ctx, tx, principal, true)
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	currentScope, err := s.contactBatchCurrentScopeTx(ctx, tx, dashboardprincipal.DashboardPrincipal{
		UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID,
		AuthVersion: principal.AuthVersion, IsSuperAdmin: actorFacts.IsSuperAdmin == 1,
	})
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	if !currentScope.Allowed {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchPermissionDenied
	}
	if currentScope.Scope != dashboard.DataScopeTenant && (!dashboardEmployeeIDsWithinScope(input.Batch.EmployeeIDs, currentScope.AllowedEmployeeIDs) || !dashboardEmployeeIDsWithinScope([]int{input.SenderEmployeeID}, currentScope.AllowedEmployeeIDs)) {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchTargetNotOwned
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	generation := credentialGenerationForCapability(binding, wecomcapability.ContactBatchSend)
	if generation == 0 || binding.Status != 2 || strings.TrimSpace(binding.VerifiedWXCorpID) == "" {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchCapabilityLimited
	}
	credential, found, err := loadEncryptedCorpCredentialByID(ctx, tx, binding.CorpID, true)
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	if !found {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchCapabilityLimited
	}
	secret, err := s.decodeEncryptedCorpCredential(credential)
	if err != nil || strings.TrimSpace(secret.ContactSecret) == "" {
		return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchCapabilityLimited
	}
	ownedExternalIDs, err := validateContactBatchTargetsTx(ctx, tx, principal.CorpID, input.Batch.EmployeeIDs, input.SenderEmployeeID, input.ContactTargets)
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	input.Batch.UserName = ""
	if err := tx.QueryRowContext(ctx, `SELECT name FROM mc_user WHERE tenant_id=? AND id=? AND deleted_at IS NULL`, principal.TenantID, principal.UserID).Scan(&input.Batch.UserName); err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}

	actor := principal.UserID
	actorUserID, actorSource := &actor, capabilityActorUser
	targetTotal := totalContactBatchTargets(ownedExternalIDs)
	operationID, operation, created, err := createContactBatchOperationTx(ctx, tx, principal, input, generation, targetTotal, actorUserID, actorSource)
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	if !created {
		batchID, ok := contactBatchIDFromRequestID(operation.RequestID)
		if !ok {
			return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchConflict
		}
		if err := tx.Commit(); err != nil {
			return dashboard.ContactBatchDispatchResult{}, err
		}
		return dashboard.ContactBatchDispatchResult{OperationID: operationID, BatchID: int64(batchID), Status: operation.Status, Duplicate: true}, nil
	}

	batchID, err := insertContactBatchBusinessRowTx(ctx, tx, principal, input.Batch)
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_capability_operations SET request_id=? WHERE tenant_id=? AND corp_id=? AND id=?`, contactBatchRequestID(batchID), principal.TenantID, principal.CorpID, operationID); err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	operation, err = queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, true)
	if err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, "", "create", actorUserID, actorSource, input.RequestID, nil); err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	if err := insertContactBatchTargetsTx(ctx, tx, batchID, principal.TenantID, principal.CorpID, input.Batch.EmployeeIDs, input.SenderEmployeeID, ownedExternalIDs); err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	for employeeIndex, employeeID := range input.Batch.EmployeeIDs {
		_ = employeeIndex
		for chunkNo, chunk := range contactBatchStringChunks(ownedExternalIDs[employeeID], contactBatchDispatchChunkSize) {
			if len(chunk) == 0 {
				continue
			}
			targetID := fmt.Sprintf("contact_batch:%d:employee:%d:chunk:%d", batchID, employeeID, chunkNo)
			idempotencyKey := contactBatchDispatchKey(input.IdempotencyKey, employeeID, chunkNo, chunk)
			result, err := tx.ExecContext(ctx, `
				INSERT INTO mochat_go_wecom_capability_dispatches
				(tenant_id,corp_id,operation_id,dispatch_kind,chunk_no,target_id,idempotency_key,status,credential_generation)
				VALUES (?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`,
				principal.TenantID, principal.CorpID, operationID, string(wecomcapability.DispatchKindContactBatch), chunkNo, targetID, idempotencyKey, wecomcapability.DispatchQueued, generation)
			if err != nil {
				return dashboard.ContactBatchDispatchResult{}, err
			}
			dispatchID, err := result.LastInsertId()
			if err != nil || dispatchID <= 0 {
				return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchConflict
			}
			dispatch, err := queryCapabilityDispatchTx(ctx, tx, principal.TenantID, principal.CorpID, dispatchID, true)
			if err != nil {
				return dashboard.ContactBatchDispatchResult{}, err
			}
			if dispatch.OperationID != operationID || dispatch.DispatchKind != string(wecomcapability.DispatchKindContactBatch) || dispatch.CredentialVersion != generation {
				return dashboard.ContactBatchDispatchResult{}, dashboard.ErrContactBatchConflict
			}
			rowsAffected, rowsErr := result.RowsAffected()
			if rowsErr == nil && rowsAffected == 1 {
				if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, operation.Status, "dispatch_enqueue", actorUserID, actorSource, input.RequestID, &dispatchID); err != nil {
					return dashboard.ContactBatchDispatchResult{}, err
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return dashboard.ContactBatchDispatchResult{}, err
	}
	return dashboard.ContactBatchDispatchResult{OperationID: operationID, BatchID: int64(batchID), Status: wecomcapability.OperationPending}, nil
}

func createContactBatchOperationTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, input dashboard.ContactBatchDispatchInput, generation uint64, targetTotal int, actorUserID *int, actorSource string) (int64, wecomcapability.Operation, bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_capability_operations
		(tenant_id,corp_id,capability,action,credential_group,credential_generation,idempotency_key,status,target_total,actor_user_id,actor_source,request_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`,
		principal.TenantID, principal.CorpID, wecomcapability.ContactBatchSend, wecomcapability.ActionSend,
		wecomcapability.CredentialGroupForCapability(wecomcapability.ContactBatchSend), generation, input.IdempotencyKey,
		wecomcapability.OperationPending, targetTotal, actorUserID, actorSource, input.RequestID)
	if err != nil {
		return 0, wecomcapability.Operation{}, false, err
	}
	operationID, err := result.LastInsertId()
	if err != nil || operationID <= 0 {
		return 0, wecomcapability.Operation{}, false, dashboard.ErrContactBatchConflict
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, true)
	if err != nil {
		return 0, wecomcapability.Operation{}, false, err
	}
	if operation.Capability != wecomcapability.ContactBatchSend || operation.Action != wecomcapability.ActionSend || operation.CredentialVersion != generation || operation.IdempotencyKey != input.IdempotencyKey {
		return 0, wecomcapability.Operation{}, false, dashboard.ErrContactBatchConflict
	}
	rowsAffected, rowsErr := result.RowsAffected()
	return operationID, operation, rowsErr == nil && rowsAffected == 1, nil
}

func insertContactBatchBusinessRowTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, values dashboard.ContactMessageBatchSendWrite) (int, error) {
	var definite any
	if strings.TrimSpace(values.DefiniteTime) != "" {
		definite = values.DefiniteTime
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_contact_message_batch_send
		(tenant_id,corp_id,user_id,user_name,batch_title,medium_id,employee_ids,filter_params,filter_params_detail,content,send_way,send_status,definite_time,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,NOW(),NOW())`,
		principal.TenantID, principal.CorpID, principal.UserID, values.UserName, values.BatchTitle, values.MediumID, mustJSONStore(values.EmployeeIDs), values.FilterParamsJSON, values.FilterDetailJSON, values.ContentJSON, values.SendWay, 1, definite)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil || id <= 0 || id > int64(^uint(0)>>1) {
		return 0, dashboard.ErrContactBatchConflict
	}
	return int(id), nil
}

func insertContactBatchTargetsTx(ctx context.Context, tx *sql.Tx, batchID, tenantID, corpID int, employeeIDs []int, senderEmployeeID int, ownedExternalIDs map[int][]string) error {
	for _, employeeID := range employeeIDs {
		var wxUserID string
		if err := tx.QueryRow(`SELECT wx_user_id FROM mc_work_employee WHERE id=? AND corp_id=? AND deleted_at IS NULL`, employeeID, corpID).Scan(&wxUserID); err != nil {
			return err
		}
		count := 0
		externalUserIDs := ownedExternalIDs[employeeID]
		for _, externalUserID := range externalUserIDs {
			var contactID int
			err := tx.QueryRow(`
				SELECT c.id FROM mc_work_contact c
				JOIN mc_work_contact_employee ce ON ce.contact_id=c.id AND ce.employee_id=? AND ce.corp_id=? AND ce.deleted_at IS NULL
				WHERE c.corp_id=? AND c.wx_external_userid=? AND c.deleted_at IS NULL LIMIT 1`, employeeID, corpID, corpID, externalUserID).Scan(&contactID)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			count++
			if _, err := tx.Exec(`
				INSERT INTO mc_contact_message_batch_send_result (batch_id,employee_id,contact_id,external_user_id,created_at,updated_at)
				VALUES (?,?,?,?,NOW(),NOW())`, batchID, employeeID, contactID, externalUserID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`
			INSERT INTO mc_contact_message_batch_send_employee (batch_id,employee_id,wx_user_id,send_contact_total,created_at,updated_at,last_sync_time)
			VALUES (?,?,?,?,NOW(),NOW(),NOW())`, batchID, employeeID, wxUserID, count); err != nil {
			return err
		}
		if employeeID == senderEmployeeID && strings.TrimSpace(wxUserID) == "" {
			return dashboard.ErrContactBatchTargetNotOwned
		}
	}
	total := totalContactBatchTargets(ownedExternalIDs)
	_, err := tx.Exec(`UPDATE mc_contact_message_batch_send SET send_employee_total=?,send_contact_total=?,not_send_total=?,not_received_total=?,updated_at=NOW() WHERE tenant_id=? AND corp_id=? AND id=?`, len(employeeIDs), total, total, total, tenantID, corpID, batchID)
	return err
}

func validateContactBatchTargetsTx(ctx context.Context, tx *sql.Tx, corpID int, employeeIDs []int, senderEmployeeID int, targets []dashboard.ContactBatchTarget) (map[int][]string, error) {
	placeholders := make([]string, len(employeeIDs))
	args := make([]any, 0, len(employeeIDs)+2)
	for index, employeeID := range employeeIDs {
		if employeeID <= 0 {
			return nil, dashboard.ErrContactBatchTargetNotOwned
		}
		placeholders[index] = "?"
		args = append(args, employeeID)
	}
	args = append(args, corpID)
	var employeeCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_employee WHERE id IN (`+strings.Join(placeholders, ",")+`) AND corp_id=? AND deleted_at IS NULL AND status <> 5`, args...).Scan(&employeeCount); err != nil {
		return nil, err
	}
	if employeeCount != len(employeeIDs) || !containsContactBatchInt(employeeIDs, senderEmployeeID) {
		return nil, dashboard.ErrContactBatchTargetNotOwned
	}
	contactTargets := buildContactBatchTargetPairs(targets)
	if len(contactTargets) == 0 {
		return nil, dashboard.ErrContactBatchTargetNotOwned
	}
	contactIDs := make([]int, 0, len(targets))
	for _, target := range targets {
		if target.EmployeeID <= 0 || target.ContactID <= 0 || !containsContactBatchInt(employeeIDs, target.EmployeeID) {
			return nil, dashboard.ErrContactBatchTargetNotOwned
		}
		contactIDs = appendUniqueInt(contactIDs, target.ContactID)
	}
	contactPlaceholders := make([]string, len(contactIDs))
	contactArgs := make([]any, 0, len(contactIDs)+len(employeeIDs)+2)
	for index, contactID := range contactIDs {
		contactPlaceholders[index] = "?"
		contactArgs = append(contactArgs, contactID)
	}
	employeePlaceholders := make([]string, len(employeeIDs))
	for index, employeeID := range employeeIDs {
		employeePlaceholders[index] = "?"
		contactArgs = append(contactArgs, employeeID)
	}
	contactArgs = append(contactArgs, corpID)
	rows, err := tx.QueryContext(ctx, `
		SELECT ce.employee_id,c.id,c.wx_external_userid
		FROM mc_work_contact c JOIN mc_work_contact_employee ce ON ce.contact_id=c.id
		WHERE c.id IN (`+strings.Join(contactPlaceholders, ",")+
		`) AND ce.employee_id IN (`+strings.Join(employeePlaceholders, ",")+
		`) AND ce.corp_id=? AND ce.deleted_at IS NULL AND c.corp_id=? AND c.deleted_at IS NULL
		ORDER BY ce.employee_id,c.id`, append(contactArgs, corpID)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resolved := make(map[[2]int]string, len(targets))
	for rows.Next() {
		var employeeID int
		var contactID int
		var externalID string
		if err := rows.Scan(&employeeID, &contactID, &externalID); err != nil {
			return nil, err
		}
		resolved[[2]int{employeeID, contactID}] = strings.TrimSpace(externalID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ownedRows := make([]contactBatchOwnedTarget, 0, len(targets))
	for _, target := range targets {
		externalID, ok := resolved[[2]int{target.EmployeeID, target.ContactID}]
		if !ok || externalID == "" {
			return nil, dashboard.ErrContactBatchTargetNotOwned
		}
		ownedRows = append(ownedRows, contactBatchOwnedTarget{EmployeeID: target.EmployeeID, ExternalUserID: externalID})
	}
	owned := buildContactBatchOwnedTargets(employeeIDs, ownedRows)
	if len(owned[senderEmployeeID]) == 0 {
		return nil, dashboard.ErrContactBatchTargetNotOwned
	}
	return owned, nil
}

func buildContactBatchTargetPairs(values []dashboard.ContactBatchTarget) map[int][]contactBatchOwnedTarget {
	result := make(map[int][]contactBatchOwnedTarget)
	seen := make(map[[2]int]struct{}, len(values))
	for _, value := range values {
		if value.EmployeeID <= 0 || value.ContactID <= 0 {
			continue
		}
		key := [2]int{value.EmployeeID, value.ContactID}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result[value.EmployeeID] = append(result[value.EmployeeID], contactBatchOwnedTarget{EmployeeID: value.EmployeeID, ContactID: value.ContactID})
	}
	return result
}

type contactBatchOwnedTarget struct {
	EmployeeID     int
	ContactID      int
	ExternalUserID string
}

func buildContactBatchOwnedTargets(employeeIDs []int, rows []contactBatchOwnedTarget) map[int][]string {
	owned := make(map[int][]string, len(employeeIDs))
	for _, employeeID := range employeeIDs {
		owned[employeeID] = []string{}
	}
	for _, row := range rows {
		if _, selected := owned[row.EmployeeID]; !selected {
			continue
		}
		if value := strings.TrimSpace(row.ExternalUserID); value != "" {
			owned[row.EmployeeID] = appendUniqueString(owned[row.EmployeeID], value)
		}
	}
	return owned
}

func contactBatchAccessIdentityMatches(access dashboard.DashboardAccessContext, principal dashboardprincipal.DashboardPrincipal) bool {
	return access.UserID > 0 && access.TenantID > 0 && access.CorpID > 0 &&
		access.UserID == principal.UserID && access.TenantID == principal.TenantID && access.CorpID == principal.CorpID
}

func (s *MySQLStore) checkContactBatchActor(ctx context.Context, queryer companyProfileQueryer, principal dashboardprincipal.DashboardPrincipal, forUpdate bool) (companyActorFacts, error) {
	if principal.UserID <= 0 || principal.TenantID <= 0 || principal.CorpID <= 0 {
		return companyActorFacts{}, dashboard.ErrContactBatchPermissionDenied
	}
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	var facts companyActorFacts
	var bindingActive int
	err := queryer.QueryRowContext(ctx, `
		SELECT u.status, COALESCE(u.isSuperAdmin, 0), d.status, u.tenant_id,
		       d.auth_version, d.activated_at IS NOT NULL,
		       CASE WHEN EXISTS (
				SELECT 1 FROM mochat_go_tenant_corp_bindings b
				WHERE b.tenant_id = u.tenant_id AND b.corp_id = ? AND b.status IN (1,2)
			) THEN 1 ELSE 0 END
		FROM mc_user u
		JOIN mochat_go_dashboard_identities d ON d.user_id = u.id
		WHERE u.id = ? AND u.tenant_id = ? AND u.deleted_at IS NULL
		LIMIT 1`+suffix, principal.CorpID, principal.UserID, principal.TenantID).Scan(
		&facts.UserStatus, &facts.IsSuperAdmin, &facts.IdentityStatus, &facts.TenantID, &facts.AuthVersion, &facts.Activated, &bindingActive)
	if errors.Is(err, sql.ErrNoRows) {
		return companyActorFacts{}, dashboard.ErrContactBatchPermissionDenied
	}
	if err != nil {
		return companyActorFacts{}, companyprofile.ErrStoreUnavailable
	}
	if !businessActorFactsAllowed(facts, principal, bindingActive == 1) {
		return companyActorFacts{}, dashboard.ErrContactBatchPermissionDenied
	}
	return facts, nil
}

func (s *MySQLStore) contactBatchPagePermissionActiveTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal) bool {
	var granted int
	err := tx.QueryRowContext(ctx, `
		SELECT CASE WHEN EXISTS (
			SELECT 1
			FROM mochat_go_dashboard_user_permissions direct_permission
			JOIN mochat_go_dashboard_permissions permission ON permission.id=direct_permission.permission_id
			WHERE direct_permission.tenant_id=? AND direct_permission.user_id=? AND direct_permission.effect='allow'
			  AND permission.code='dashboard.acquisition.precise_group_send'
			  AND permission.status=1 AND permission.deleted_at IS NULL
		) OR EXISTS (
			SELECT 1
			FROM mochat_go_dashboard_user_roles user_role
			JOIN mc_rbac_role role ON role.tenant_id=user_role.tenant_id AND role.id=user_role.role_id
			JOIN mochat_go_dashboard_role_permissions role_permission ON role_permission.tenant_id=user_role.tenant_id AND role_permission.role_id=user_role.role_id
			JOIN mochat_go_dashboard_permissions permission ON permission.id=role_permission.permission_id
			WHERE user_role.tenant_id=? AND user_role.user_id=? AND role.status=1 AND role.deleted_at IS NULL
			  AND permission.code='dashboard.acquisition.precise_group_send'
			  AND permission.status=1 AND permission.deleted_at IS NULL
		) THEN 1 ELSE 0 END`, principal.TenantID, principal.UserID, principal.TenantID, principal.UserID).Scan(&granted)
	return err == nil && granted == 1
}

func (s *MySQLStore) contactBatchQuotaStatusTx(ctx context.Context, tx *sql.Tx, tenantID int, additional int64) (dashboard.SaaSQuotaStatus, error) {
	status := dashboard.SaaSQuotaStatus{Metric: dashboard.SaaSMetricContactMessageBatches, TenantID: tenantID, Additional: additional}
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mc_contact_message_batch_send batch
		JOIN mc_corp corp ON corp.id=batch.corp_id AND corp.tenant_id=?
		WHERE batch.tenant_id=? AND corp.deleted_at IS NULL AND batch.deleted_at IS NULL`, tenantID, tenantID).Scan(&status.Current); err != nil {
		return dashboard.SaaSQuotaStatus{}, err
	}
	err := tx.QueryRowContext(ctx, `
		SELECT limit_value FROM mochat_go_saas_usage_counters
		WHERE tenant_id=? AND metric=? AND period_key='lifetime' AND deleted_at IS NULL
		LIMIT 1 FOR UPDATE`, tenantID, dashboard.SaaSMetricContactMessageBatches).Scan(&status.Limit)
	if errors.Is(err, sql.ErrNoRows) || isMissingSaaSTableError(err) {
		return status, nil
	}
	if err != nil {
		return dashboard.SaaSQuotaStatus{}, err
	}
	return status, nil
}

func dashboardPermissionCodeContains(codes []string, wanted string) bool {
	for _, code := range codes {
		if strings.TrimSpace(code) == wanted {
			return true
		}
	}
	return false
}

func dashboardEmployeeIDsWithinScope(ids, allowed []int) bool {
	allowedSet := make(map[int]struct{}, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := allowedSet[id]; !ok {
			return false
		}
	}
	return true
}

func validContactBatchStoreToken(value string, limit int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > limit || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	return true
}

func contactBatchRequestID(batchID int) string { return "contact-batch:" + strconv.Itoa(batchID) }

func contactBatchIDFromRequestID(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "contact-batch:") {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(value, "contact-batch:"))
	return id, err == nil && id > 0
}

func contactBatchDispatchKey(seed string, employeeID, chunkNo int, values []string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(seed))
	_, _ = hash.Write([]byte("\x00" + strconv.Itoa(employeeID) + "\x00" + strconv.Itoa(chunkNo) + "\x00" + strings.Join(values, "\x00")))
	return "contact-" + hex.EncodeToString(hash.Sum(nil))
}

func contactBatchStringChunks(values []string, size int) [][]string {
	if size <= 0 {
		size = contactBatchDispatchChunkSize
	}
	result := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		result = append(result, append([]string(nil), values[start:end]...))
	}
	return result
}

func containsContactBatchInt(values []int, wanted int) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func appendUniqueString(values []string, wanted string) []string {
	for _, value := range values {
		if value == wanted {
			return values
		}
	}
	return append(values, wanted)
}

func totalContactBatchTargets(values map[int][]string) int {
	total := 0
	for _, targets := range values {
		total += len(targets)
	}
	return total
}

func (s *MySQLStore) nowForContactBatch() time.Time { return time.Now() }
