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

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

// CreateRoomBatchDispatch is the only production create entry for the
// durable room-batch path. It deliberately does not call the legacy
// CreateRoomMessageBatchSend method: the business row, operation,
// dispatch chunks, and their create audit/event are one transaction.
func (s *MySQLStore) CreateRoomBatchDispatch(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, access dashboard.DashboardAccessContext, input dashboard.RoomBatchDispatchInput) (dashboard.RoomBatchDispatchResult, error) {
	if s == nil || s.db == nil || principal.TenantID <= 0 || principal.CorpID <= 0 || principal.UserID <= 0 || input.Batch.UserID != principal.UserID || input.Batch.CorpID != principal.CorpID || strings.TrimSpace(input.IdempotencyKey) == "" {
		return dashboard.RoomBatchDispatchResult{}, companyprofile.ErrInvalidRequest
	}
	if !principal.IsSuperAdmin && !dashboardPermissionCodeContains(access.PermissionCodes, "dashboard.acquisition.precise_group_send") {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchPermissionDenied
	}
	if !contactBatchAccessIdentityMatches(access, principal) {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchBodyScope
	}
	if access.ScopeRequired && access.Scope != dashboard.DataScopeTenant && len(access.AllowedEmployeeIDs) == 0 {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchTargetNotOwned
	}
	if len(input.Batch.EmployeeIDs) == 0 || len(input.RoomTargets) == 0 || input.SenderOwnerEmployeeID <= 0 {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchTargetNotOwned
	}
	if !validContactBatchStoreToken(input.IdempotencyKey, 128) || !validContactBatchStoreToken(input.RequestID, 255) {
		return dashboard.RoomBatchDispatchResult{}, companyprofile.ErrInvalidRequest
	}
	if access.ScopeRequired && access.Scope != dashboard.DataScopeTenant && !dashboardEmployeeIDsWithinScope(input.Batch.EmployeeIDs, access.AllowedEmployeeIDs) {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchTargetNotOwned
	}
	if access.ScopeRequired && access.Scope != dashboard.DataScopeTenant && !dashboardEmployeeIDsWithinScope([]int{input.SenderOwnerEmployeeID}, access.AllowedEmployeeIDs) {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchTargetNotOwned
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, companyprofile.ErrStoreUnavailable
	}
	defer rollbackQuietly(tx)
	tenantAccess, err := s.dashboardTenantAccessTx(ctx, tx, principal.TenantID, s.nowForContactBatch())
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	if !tenantAccess.Allowed {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchTenantDenied
	}
	quota, err := s.roomBatchQuotaStatusTx(ctx, tx, principal.TenantID, 1)
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	if quota.Limit > 0 && quota.Current+quota.Additional > quota.Limit {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchQuotaExceeded
	}
	actorFacts, err := s.checkContactBatchActor(ctx, tx, principal, true)
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	currentScope, err := s.contactBatchCurrentScopeTx(ctx, tx, dashboardprincipal.DashboardPrincipal{
		UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID,
		AuthVersion: principal.AuthVersion, IsSuperAdmin: actorFacts.IsSuperAdmin == 1,
	})
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	if !currentScope.Allowed {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchPermissionDenied
	}
	if currentScope.Scope != dashboard.DataScopeTenant && (!dashboardEmployeeIDsWithinScope(input.Batch.EmployeeIDs, currentScope.AllowedEmployeeIDs) || !dashboardEmployeeIDsWithinScope([]int{input.SenderOwnerEmployeeID}, currentScope.AllowedEmployeeIDs)) {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchTargetNotOwned
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	generation := credentialGenerationForCapability(binding, wecomcapability.RoomBatchSend)
	if generation == 0 || binding.Status != 2 || strings.TrimSpace(binding.VerifiedWXCorpID) == "" {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchCapabilityLimited
	}
	credential, found, err := loadEncryptedCorpCredentialByID(ctx, tx, binding.CorpID, true)
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	if !found {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchCapabilityLimited
	}
	secret, err := s.decodeEncryptedCorpCredential(credential)
	if err != nil || strings.TrimSpace(secret.ContactSecret) == "" {
		return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchCapabilityLimited
	}
	ownedRooms, err := validateRoomBatchTargetsTx(ctx, tx, principal.CorpID, input.Batch.EmployeeIDs, input.RoomTargets)
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	input.Batch.UserName = ""
	if err := tx.QueryRowContext(ctx, `SELECT name FROM mc_user WHERE tenant_id=? AND id=? AND deleted_at IS NULL`, principal.TenantID, principal.UserID).Scan(&input.Batch.UserName); err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}

	actor := principal.UserID
	actorUserID, actorSource := &actor, capabilityActorUser
	targetTotal := totalRoomBatchTargets(ownedRooms)
	operationID, operation, created, err := createRoomBatchOperationTx(ctx, tx, principal, input, generation, targetTotal, actorUserID, actorSource)
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	if !created {
		batchID, ok := roomBatchIDFromRequestID(operation.RequestID)
		if !ok {
			return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchConflict
		}
		if err := tx.Commit(); err != nil {
			return dashboard.RoomBatchDispatchResult{}, err
		}
		return dashboard.RoomBatchDispatchResult{OperationID: operationID, BatchID: int64(batchID), Status: operation.Status, Duplicate: true}, nil
	}

	batchID, err := insertRoomBatchBusinessRowTx(ctx, tx, principal, input.Batch)
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_capability_operations SET request_id=? WHERE tenant_id=? AND corp_id=? AND id=?`, roomBatchRequestID(batchID), principal.TenantID, principal.CorpID, operationID); err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	operation, err = queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, true)
	if err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, "", "create", actorUserID, actorSource, input.RequestID, nil); err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	if err := insertRoomBatchTargetsTx(ctx, tx, batchID, principal.TenantID, principal.CorpID, input.Batch.EmployeeIDs, input.SenderOwnerEmployeeID, ownedRooms); err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	for _, employeeID := range input.Batch.EmployeeIDs {
		for chunkNo, chunk := range contactBatchStringChunks(roomBatchChatIDs(ownedRooms[employeeID]), contactBatchDispatchChunkSize) {
			if len(chunk) == 0 {
				continue
			}
			targetID := fmt.Sprintf("room_batch:%d:owner:%d:chunk:%d", batchID, employeeID, chunkNo)
			idempotencyKey := roomBatchDispatchKey(input.IdempotencyKey, employeeID, chunkNo, chunk)
			result, err := tx.ExecContext(ctx, `
				INSERT INTO mochat_go_wecom_capability_dispatches
				(tenant_id,corp_id,operation_id,dispatch_kind,chunk_no,target_id,idempotency_key,status,credential_generation)
				VALUES (?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`,
				principal.TenantID, principal.CorpID, operationID, string(wecomcapability.DispatchKindRoomBatch), chunkNo, targetID, idempotencyKey, wecomcapability.DispatchQueued, generation)
			if err != nil {
				return dashboard.RoomBatchDispatchResult{}, err
			}
			dispatchID, err := result.LastInsertId()
			if err != nil || dispatchID <= 0 {
				return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchConflict
			}
			dispatch, err := queryCapabilityDispatchTx(ctx, tx, principal.TenantID, principal.CorpID, dispatchID, true)
			if err != nil {
				return dashboard.RoomBatchDispatchResult{}, err
			}
			if dispatch.OperationID != operationID || dispatch.DispatchKind != string(wecomcapability.DispatchKindRoomBatch) || dispatch.CredentialVersion != generation {
				return dashboard.RoomBatchDispatchResult{}, dashboard.ErrRoomBatchConflict
			}
			rowsAffected, rowsErr := result.RowsAffected()
			if rowsErr == nil && rowsAffected == 1 {
				if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, operation.Status, "dispatch_enqueue", actorUserID, actorSource, input.RequestID, &dispatchID); err != nil {
					return dashboard.RoomBatchDispatchResult{}, err
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return dashboard.RoomBatchDispatchResult{}, err
	}
	return dashboard.RoomBatchDispatchResult{OperationID: operationID, BatchID: int64(batchID), Status: wecomcapability.OperationPending}, nil
}

func createRoomBatchOperationTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, input dashboard.RoomBatchDispatchInput, generation uint64, targetTotal int, actorUserID *int, actorSource string) (int64, wecomcapability.Operation, bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_capability_operations
		(tenant_id,corp_id,capability,action,credential_group,credential_generation,idempotency_key,status,target_total,actor_user_id,actor_source,request_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`,
		principal.TenantID, principal.CorpID, wecomcapability.RoomBatchSend, wecomcapability.ActionSend,
		wecomcapability.CredentialGroupForCapability(wecomcapability.RoomBatchSend), generation, input.IdempotencyKey,
		wecomcapability.OperationPending, targetTotal, actorUserID, actorSource, input.RequestID)
	if err != nil {
		return 0, wecomcapability.Operation{}, false, err
	}
	operationID, err := result.LastInsertId()
	if err != nil || operationID <= 0 {
		return 0, wecomcapability.Operation{}, false, dashboard.ErrRoomBatchConflict
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, true)
	if err != nil {
		return 0, wecomcapability.Operation{}, false, err
	}
	if operation.Capability != wecomcapability.RoomBatchSend || operation.Action != wecomcapability.ActionSend || operation.CredentialVersion != generation || operation.IdempotencyKey != input.IdempotencyKey {
		return 0, wecomcapability.Operation{}, false, dashboard.ErrRoomBatchConflict
	}
	rowsAffected, rowsErr := result.RowsAffected()
	return operationID, operation, rowsErr == nil && rowsAffected == 1, nil
}

func insertRoomBatchBusinessRowTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, values dashboard.RoomMessageBatchSendWrite) (int, error) {
	var definite any
	if strings.TrimSpace(values.DefiniteTime) != "" {
		definite = values.DefiniteTime
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_room_message_batch_send
		(tenant_id,corp_id,user_id,user_name,batch_title,medium_id,employee_ids,content,send_way,send_status,definite_time,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,NOW(),NOW())`,
		principal.TenantID, principal.CorpID, principal.UserID, values.UserName, values.BatchTitle, values.MediumID, mustJSONStore(values.EmployeeIDs), values.ContentJSON, values.SendWay, 1, definite)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil || id <= 0 || id > int64(^uint(0)>>1) {
		return 0, dashboard.ErrRoomBatchConflict
	}
	return int(id), nil
}

type roomBatchOwnedRoom struct {
	RoomID     int
	Name       string
	WXChatID   string
	CreateTime sql.NullTime
	MemberNum  int
}

// validateRoomBatchTargetsTx re-verifies every explicit room target inside
// the create transaction: the owner employees must exist and be active, and
// each (owner employee, room) pair must resolve to a live mc_work_room row
// owned by that employee under the corp.
func validateRoomBatchTargetsTx(ctx context.Context, tx *sql.Tx, corpID int, employeeIDs []int, targets []dashboard.RoomBatchTarget) (map[int][]roomBatchOwnedRoom, error) {
	employeeIDs = uniquePositiveInts(employeeIDs)
	if len(employeeIDs) == 0 {
		return nil, dashboard.ErrRoomBatchTargetNotOwned
	}
	placeholders := make([]string, len(employeeIDs))
	args := make([]any, 0, len(employeeIDs)+2)
	for index, employeeID := range employeeIDs {
		placeholders[index] = "?"
		args = append(args, employeeID)
	}
	args = append(args, corpID)
	var employeeCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_employee WHERE id IN (`+strings.Join(placeholders, ",")+`) AND corp_id=? AND deleted_at IS NULL AND status <> 5`, args...).Scan(&employeeCount); err != nil {
		return nil, err
	}
	if employeeCount != len(employeeIDs) {
		return nil, dashboard.ErrRoomBatchTargetNotOwned
	}
	if len(targets) == 0 {
		return nil, dashboard.ErrRoomBatchTargetNotOwned
	}
	type roomBatchPair struct {
		OwnerEmployeeID int
		RoomID          int
	}
	seen := make(map[roomBatchPair]struct{}, len(targets))
	pairs := make([]roomBatchPair, 0, len(targets))
	roomIDs := make([]int, 0, len(targets))
	for _, target := range targets {
		if target.OwnerEmployeeID <= 0 || target.RoomID <= 0 || !containsContactBatchInt(employeeIDs, target.OwnerEmployeeID) {
			return nil, dashboard.ErrRoomBatchTargetNotOwned
		}
		pair := roomBatchPair{OwnerEmployeeID: target.OwnerEmployeeID, RoomID: target.RoomID}
		if _, exists := seen[pair]; exists {
			continue
		}
		seen[pair] = struct{}{}
		pairs = append(pairs, pair)
		roomIDs = appendUniqueInt(roomIDs, target.RoomID)
	}
	if len(pairs) == 0 {
		return nil, dashboard.ErrRoomBatchTargetNotOwned
	}
	roomPlaceholders := make([]string, len(roomIDs))
	roomArgs := make([]any, 0, len(roomIDs)+2)
	for index, roomID := range roomIDs {
		roomPlaceholders[index] = "?"
		roomArgs = append(roomArgs, roomID)
	}
	roomArgs = append(roomArgs, corpID)
	rows, err := tx.QueryContext(ctx, `
		SELECT r.id, COALESCE(r.name,''), COALESCE(r.wx_chat_id,''), r.create_time, r.owner_id,
		       (SELECT COUNT(*) FROM mc_work_contact_room member WHERE member.room_id=r.id AND member.deleted_at IS NULL) AS member_num
		FROM mc_work_room r
		WHERE r.id IN (`+strings.Join(roomPlaceholders, ",")+`) AND r.corp_id=? AND r.deleted_at IS NULL`, roomArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resolved := make(map[int]roomBatchOwnedRoom, len(roomIDs))
	owners := make(map[int]int, len(roomIDs))
	for rows.Next() {
		var room roomBatchOwnedRoom
		var name, wxChatID sql.NullString
		var ownerID int
		if err := rows.Scan(&room.RoomID, &name, &wxChatID, &room.CreateTime, &ownerID, &room.MemberNum); err != nil {
			return nil, err
		}
		room.Name = nullString(name)
		room.WXChatID = nullString(wxChatID)
		resolved[room.RoomID] = room
		owners[room.RoomID] = ownerID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	owned := make(map[int][]roomBatchOwnedRoom, len(employeeIDs))
	for _, employeeID := range employeeIDs {
		owned[employeeID] = []roomBatchOwnedRoom{}
	}
	for _, pair := range pairs {
		room, ok := resolved[pair.RoomID]
		if !ok || owners[pair.RoomID] != pair.OwnerEmployeeID {
			return nil, dashboard.ErrRoomBatchTargetNotOwned
		}
		owned[pair.OwnerEmployeeID] = append(owned[pair.OwnerEmployeeID], room)
	}
	return owned, nil
}

// insertRoomBatchTargetsTx persists the resolved result rows and the per-owner
// employee rows, then updates the business-row totals, all inside the create
// transaction. The sender owner employee must have a wx_user_id.
func insertRoomBatchTargetsTx(ctx context.Context, tx *sql.Tx, batchID, tenantID, corpID int, employeeIDs []int, senderEmployeeID int, owned map[int][]roomBatchOwnedRoom) error {
	for _, employeeID := range employeeIDs {
		var wxUserID string
		if err := tx.QueryRow(`SELECT wx_user_id FROM mc_work_employee WHERE id=? AND corp_id=? AND deleted_at IS NULL`, employeeID, corpID).Scan(&wxUserID); err != nil {
			return err
		}
		rooms := owned[employeeID]
		for _, room := range rooms {
			if _, err := tx.Exec(`
				INSERT INTO mc_room_message_batch_send_result (batch_id,employee_id,room_id,room_name,room_employee_num,room_create_time,chat_id,created_at,updated_at)
				VALUES (?,?,?,?,?,?,?,NOW(),NOW())`, batchID, employeeID, room.RoomID, room.Name, room.MemberNum, nullTimeArg(room.CreateTime), room.WXChatID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`
			INSERT INTO mc_room_message_batch_send_employee (batch_id,employee_id,wx_user_id,send_room_total,created_at,updated_at,last_sync_time)
			VALUES (?,?,?,?,NOW(),NOW(),NOW())`, batchID, employeeID, wxUserID, len(rooms)); err != nil {
			return err
		}
		if employeeID == senderEmployeeID && strings.TrimSpace(wxUserID) == "" {
			return dashboard.ErrRoomBatchTargetNotOwned
		}
	}
	roomTotal := totalRoomBatchTargets(owned)
	_, err := tx.Exec(`UPDATE mc_room_message_batch_send SET send_employee_total=?,send_room_total=?,not_send_total=?,not_received_total=?,updated_at=NOW() WHERE tenant_id=? AND corp_id=? AND id=?`, len(employeeIDs), roomTotal, roomTotal, roomTotal, tenantID, corpID, batchID)
	return err
}

// roomBatchQuotaStatusTx mirrors contactBatchQuotaStatusTx against the room
// batch business table and the room_message_batches SaaS metric.
func (s *MySQLStore) roomBatchQuotaStatusTx(ctx context.Context, tx *sql.Tx, tenantID int, additional int64) (dashboard.SaaSQuotaStatus, error) {
	status := dashboard.SaaSQuotaStatus{Metric: dashboard.SaaSMetricRoomMessageBatches, TenantID: tenantID, Additional: additional}
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mc_room_message_batch_send batch
		JOIN mc_corp corp ON corp.id=batch.corp_id AND corp.tenant_id=?
		WHERE batch.tenant_id=? AND corp.deleted_at IS NULL AND batch.deleted_at IS NULL`, tenantID, tenantID).Scan(&status.Current); err != nil {
		return dashboard.SaaSQuotaStatus{}, err
	}
	err := tx.QueryRowContext(ctx, `
		SELECT limit_value FROM mochat_go_saas_usage_counters
		WHERE tenant_id=? AND metric=? AND period_key='lifetime' AND deleted_at IS NULL
		LIMIT 1 FOR UPDATE`, tenantID, dashboard.SaaSMetricRoomMessageBatches).Scan(&status.Limit)
	if errors.Is(err, sql.ErrNoRows) || isMissingSaaSTableError(err) {
		return status, nil
	}
	if err != nil {
		return dashboard.SaaSQuotaStatus{}, err
	}
	return status, nil
}

func roomBatchChatIDs(rooms []roomBatchOwnedRoom) []string {
	result := make([]string, 0, len(rooms))
	for _, room := range rooms {
		if value := strings.TrimSpace(room.WXChatID); value != "" {
			result = appendUniqueString(result, value)
		}
	}
	return result
}

func totalRoomBatchTargets(values map[int][]roomBatchOwnedRoom) int {
	total := 0
	for _, rooms := range values {
		total += len(rooms)
	}
	return total
}

func roomBatchRequestID(batchID int) string { return "room-batch:" + strconv.Itoa(batchID) }

func roomBatchIDFromRequestID(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "room-batch:") {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(value, "room-batch:"))
	return id, err == nil && id > 0
}

func roomBatchDispatchKey(seed string, employeeID, chunkNo int, values []string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(seed))
	_, _ = hash.Write([]byte("\x00" + strconv.Itoa(employeeID) + "\x00" + strconv.Itoa(chunkNo) + "\x00" + strings.Join(values, "\x00")))
	return "room-" + hex.EncodeToString(hash.Sum(nil))
}
