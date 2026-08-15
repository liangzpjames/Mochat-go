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

// RoomBatchDispatchDue returns only room dispatches that the durable ledger
// says may be claimed now. The operation actor and current identity version
// are read with the same snapshot as the dispatch; a later authorizer
// transaction is still mandatory before an external request.
func (s *MySQLStore) RoomBatchDispatchDue(ctx context.Context, limit int) ([]dashboard.RoomBatchDispatchWorkItem, error) {
	if s == nil || s.db == nil || limit <= 0 {
		return nil, errors.New("room batch dispatch store is unavailable")
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
		JOIN mc_room_message_batch_send batch
		  ON batch.tenant_id=d.tenant_id AND batch.corp_id=d.corp_id
		 AND batch.id=CAST(SUBSTRING_INDEX(o.request_id, ':', -1) AS UNSIGNED)
		 AND o.request_id LIKE 'room-batch:%' AND batch.deleted_at IS NULL
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
		string(wecomcapability.DispatchKindRoomBatch), wecomcapability.DispatchQueued,
		wecomcapability.DispatchFailed, wecomcapability.DispatchPartialFailed, "wecom.http_429", "wecom.http_500",
		wecomcapability.DispatchClaimed, wecomcapability.DispatchSubmitting, wecomcapability.DispatchSubmitted, wecomcapability.DispatchPolling,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomBatchDispatchWorkItem, 0)
	for rows.Next() {
		item, actorID, authVersion, superadmin, err := scanRoomBatchDue(rows)
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

func scanRoomBatchDue(scanner interface{ Scan(...any) error }) (dashboard.RoomBatchDispatchWorkItem, int, int64, int, error) {
	var item dashboard.RoomBatchDispatchWorkItem
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
		return dashboard.RoomBatchDispatchWorkItem{}, 0, 0, 0, err
	}
	dispatch.LeaseExpiresAt, dispatch.NextPollAt = nullableTimePtr(leaseExpires), nullableTimePtr(nextPoll)
	dispatch.CreatedAt, dispatch.UpdatedAt = nullableTimePtr(created), nullableTimePtr(updated)
	item.Dispatch = dispatch
	return item, actorID, authVersion, superadmin, nil
}

// authorizeRoomBatchDispatch is the room branch of AuthorizeDispatch. It
// deliberately mirrors the contact branch: every preclaim and postclaim
// rechecks the current binding, actor identity, page grant, data scope and
// persisted room ownership before the sender is reached.
func (s *MySQLStore) authorizeRoomBatchDispatch(ctx context.Context, request wecomcapability.DispatchAuthorizationRequest) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	dispatch, err := queryCapabilityDispatchTx(ctx, tx, request.Principal.TenantID, request.Principal.CorpID, request.DispatchID, true)
	if err != nil {
		return err
	}
	if dispatch.DispatchKind != string(wecomcapability.DispatchKindRoomBatch) || !wecomcapability.DispatchKindMatchesCapability(request.Capability, wecomcapability.DispatchKind(dispatch.DispatchKind)) {
		return dashboard.ErrRoomBatchCapabilityLimited
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, request.Principal.TenantID, request.Principal.CorpID, dispatch.OperationID, true)
	if err != nil {
		return err
	}
	if operation.Capability != wecomcapability.RoomBatchSend || operation.ActorUserID != request.Principal.UserID || !capabilityOperationAllowsDispatch(operation.Status) {
		return dashboard.ErrRoomBatchCapabilityLimited
	}
	tenantAccess, err := s.dashboardTenantAccessTx(ctx, tx, request.Principal.TenantID, s.nowForContactBatch())
	if err != nil {
		return err
	}
	if !tenantAccess.Allowed {
		return dashboard.ErrRoomBatchTenantDenied
	}
	quota, err := s.roomBatchQuotaStatusTx(ctx, tx, request.Principal.TenantID, 0)
	if err != nil {
		return err
	}
	if quota.Limit > 0 && quota.Current > quota.Limit {
		return dashboard.ErrRoomBatchQuotaExceeded
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
		return dashboard.ErrRoomBatchPermissionDenied
	}
	if err := s.authorizeRoomBatchDispatchTargetTx(ctx, tx, principal, dispatch, scope); err != nil {
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
		return dashboard.ErrRoomBatchCapabilityLimited
	}
	secret, err := s.decodeEncryptedCorpCredential(credential)
	if err != nil {
		return dashboard.ErrRoomBatchCapabilityLimited
	}
	if generation == 0 || dispatch.CredentialVersion != generation || operation.CredentialVersion != generation || request.ExpectedCredentialVersion != generation || binding.Status != 2 || strings.TrimSpace(binding.VerifiedWXCorpID) == "" || strings.TrimSpace(secret.ContactSecret) == "" {
		return dashboard.ErrRoomBatchCapabilityLimited
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// authorizeRoomBatchDispatchTargetTx verifies that the dispatch's owner
// employee still owns every persisted room of this batch chunk under the
// current corp and data scope.
func (s *MySQLStore) authorizeRoomBatchDispatchTargetTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, dispatch wecomcapability.Dispatch, scope contactBatchCurrentScope) error {
	target, ok := parseRoomBatchDispatchTarget(dispatch.TargetID)
	if !ok {
		return dashboard.ErrRoomBatchTargetNotOwned
	}
	if scope.Scope != dashboard.DataScopeTenant && !containsContactBatchEmployeeInt(scope.AllowedEmployeeIDs, target.OwnerEmployeeID) {
		return dashboard.ErrRoomBatchTargetNotOwned
	}
	var employeeRows int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mc_room_message_batch_send_employee employee_batch
		JOIN mc_room_message_batch_send batch ON batch.id=employee_batch.batch_id AND batch.tenant_id=? AND batch.corp_id=? AND batch.deleted_at IS NULL
		WHERE employee_batch.batch_id=? AND employee_batch.employee_id=?`, principal.TenantID, principal.CorpID, target.BatchID, target.OwnerEmployeeID).Scan(&employeeRows); err != nil {
		return err
	}
	if employeeRows != 1 {
		return dashboard.ErrRoomBatchTargetNotOwned
	}
	var unowned int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_room_message_batch_send_result result
		JOIN mc_room_message_batch_send batch ON batch.id=result.batch_id AND batch.tenant_id=? AND batch.corp_id=? AND batch.deleted_at IS NULL
		WHERE result.batch_id=? AND result.employee_id=?
		  AND NOT EXISTS (
			SELECT 1 FROM mc_work_room room
			WHERE room.id=result.room_id AND room.corp_id=? AND room.owner_id=? AND room.deleted_at IS NULL
		  )`, principal.TenantID, principal.CorpID, target.BatchID, target.OwnerEmployeeID, principal.CorpID, target.OwnerEmployeeID).Scan(&unowned); err != nil {
		return err
	}
	if unowned != 0 {
		return dashboard.ErrRoomBatchTargetNotOwned
	}
	return nil
}
