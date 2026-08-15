package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

type roomBatchDurableSnapshot struct {
	Batch      dashboard.RoomMessageBatchSendItem
	Operation  wecomcapability.Operation
	Dispatches []wecomcapability.Dispatch
	Results    []wecomcapability.OperationResult
}

func roomMessageBatchSendByScopedIDTx(ctx context.Context, tx *sql.Tx, tenantID, corpID, batchID int, forUpdate bool) (dashboard.RoomMessageBatchSendItem, bool, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	row := tx.QueryRowContext(ctx, `
		SELECT id, corp_id, user_id, medium_id, user_name, employee_ids, batch_title, content,
		       send_way, definite_time, send_time, send_room_total, send_employee_total, send_total,
		       not_send_total, received_total, not_received_total, send_status, created_at
		FROM mc_room_message_batch_send
		WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL
		LIMIT 1`+suffix, tenantID, corpID, batchID)
	item, err := scanRoomMessageBatchSendRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.RoomMessageBatchSendItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomMessageBatchSendItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) RoomBatchDurableView(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID int) (dashboard.RoomBatchDurableView, bool, error) {
	if s == nil || s.db == nil || batchID <= 0 {
		return dashboard.RoomBatchDurableView{}, false, dashboard.ErrRoomBatchBodyScope
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.RoomBatchDurableView{}, false, err
	}
	defer rollbackQuietly(tx)
	snapshot, found, err := s.roomBatchDurableSnapshotTx(ctx, tx, principal, batchID, false)
	if err != nil || !found {
		return dashboard.RoomBatchDurableView{}, found, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.RoomBatchDurableView{}, false, err
	}
	return dashboard.RoomBatchDurableView{
		Batch: snapshot.Batch, Operation: snapshot.Operation,
		Dispatches: snapshot.Dispatches, Results: snapshot.Results,
	}, true, nil
}

func (s *MySQLStore) roomBatchDurableSnapshotTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, batchID int, forUpdate bool) (roomBatchDurableSnapshot, bool, error) {
	if _, err := s.authorizeRoomBatchReadTx(ctx, tx, principal, batchID, forUpdate); err != nil {
		return roomBatchDurableSnapshot{}, false, err
	}
	batch, found, err := roomMessageBatchSendByScopedIDTx(ctx, tx, principal.TenantID, principal.CorpID, batchID, forUpdate)
	if err != nil || !found {
		return roomBatchDurableSnapshot{}, found, err
	}
	operationID, found, err := roomBatchOperationIDTx(ctx, tx, principal.TenantID, principal.CorpID, batchID, forUpdate)
	if err != nil || !found {
		return roomBatchDurableSnapshot{}, false, err
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, forUpdate)
	if err != nil {
		return roomBatchDurableSnapshot{}, false, err
	}
	dispatches, err := queryContactBatchDispatchesTx(ctx, tx, principal.TenantID, principal.CorpID, operationID)
	if err != nil {
		return roomBatchDurableSnapshot{}, false, err
	}
	results, err := queryCapabilityResultsDB(ctx, tx, principal.TenantID, principal.CorpID, operationID)
	if err != nil {
		return roomBatchDurableSnapshot{}, false, err
	}
	operation.Status = wecomcapability.AggregateOperationStatusWithResults(operation.Capability, dispatches, results)
	batch.SendStatus = roomBatchDurableBatchStatus(operation.Status)
	batch.SendTotal = operation.SuccessTotal
	batch.ReceivedTotal = operation.SuccessTotal
	batch.NotReceivedTotal = operation.FailureTotal
	batch.SendRoomTotal = operation.TargetTotal
	if operation.FinishedAt != nil {
		batch.SendTime = operation.FinishedAt.Format("2006-01-02 15:04:05")
	}
	return roomBatchDurableSnapshot{Batch: batch, Operation: operation, Dispatches: dispatches, Results: results}, true, nil
}

func (s *MySQLStore) authorizeRoomBatchReadTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, batchID int, forUpdate bool) (dashboard.RoomMessageBatchSendItem, error) {
	facts, err := s.checkContactBatchActor(ctx, tx, principal, forUpdate)
	if err != nil {
		return dashboard.RoomMessageBatchSendItem{}, err
	}
	principal.IsSuperAdmin = facts.IsSuperAdmin == 1
	scope, err := s.contactBatchCurrentScopeTx(ctx, tx, principal)
	if err != nil {
		return dashboard.RoomMessageBatchSendItem{}, err
	}
	if !scope.Allowed {
		return dashboard.RoomMessageBatchSendItem{}, dashboard.ErrRoomBatchPermissionDenied
	}
	batch, found, err := roomMessageBatchSendByScopedIDTx(ctx, tx, principal.TenantID, principal.CorpID, batchID, forUpdate)
	if err != nil {
		return dashboard.RoomMessageBatchSendItem{}, err
	}
	if !found {
		return dashboard.RoomMessageBatchSendItem{}, dashboard.ErrRoomBatchNotFound
	}
	if batch.UserID != principal.UserID {
		return dashboard.RoomMessageBatchSendItem{}, dashboard.ErrRoomBatchTargetNotOwned
	}
	if scope.Scope != dashboard.DataScopeTenant && !dashboardEmployeeIDsWithinScope(batch.EmployeeIDs, scope.AllowedEmployeeIDs) {
		return dashboard.RoomMessageBatchSendItem{}, dashboard.ErrRoomBatchTargetNotOwned
	}
	return batch, nil
}

func roomBatchOperationIDTx(ctx context.Context, tx *sql.Tx, tenantID, corpID, batchID int, forUpdate bool) (int64, bool, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	var operationID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_wecom_capability_operations WHERE tenant_id=? AND corp_id=? AND capability=? AND request_id=? ORDER BY id DESC LIMIT 1`+suffix, tenantID, corpID, wecomcapability.RoomBatchSend, roomBatchRequestID(batchID)).Scan(&operationID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return operationID, err == nil, err
}

func roomBatchDurableBatchStatus(status string) int {
	switch status {
	case wecomcapability.OperationSucceeded:
		return 2
	case wecomcapability.OperationPartialFailed, wecomcapability.OperationFailed, wecomcapability.OperationCancelled:
		return 3
	case wecomcapability.OperationPending:
		return 0
	default:
		return 1
	}
}

func roomBatchResultStatusMap(results []wecomcapability.OperationResult) map[string]string {
	result := make(map[string]string, len(results))
	for _, item := range results {
		if item.TargetKind != "employee_room_chat_id" || strings.TrimSpace(item.TargetID) == "" {
			continue
		}
		if current, exists := result[item.TargetID]; !exists || current == wecomcapability.DispatchSucceeded && item.Status != wecomcapability.DispatchSucceeded {
			result[item.TargetID] = item.Status
		}
	}
	return result
}

func roomBatchDurableResultStatus(operationStatus string, results map[string]string, employeeID int, chatID string) int {
	if status, ok := results[fmt.Sprintf("%d:%s", employeeID, strings.TrimSpace(chatID))]; ok {
		switch status {
		case wecomcapability.DispatchSucceeded:
			return 2
		case wecomcapability.DispatchFailed, wecomcapability.DispatchPartialFailed:
			return 3
		}
	}
	return roomBatchDurableBatchStatus(operationStatus)
}

func (s *MySQLStore) RoomBatchDurableOwnerPage(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID int, filter dashboard.RoomMessageBatchSendOwnerFilter) (dashboard.RoomMessageBatchSendOwnerPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.RoomMessageBatchSendOwnerPage{}, err
	}
	defer rollbackQuietly(tx)
	snapshot, found, err := s.roomBatchDurableSnapshotTx(ctx, tx, principal, batchID, false)
	if err != nil {
		return dashboard.RoomMessageBatchSendOwnerPage{}, err
	}
	if !found {
		return dashboard.RoomMessageBatchSendOwnerPage{}, sql.ErrNoRows
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT a.id,a.send_room_total,e.id,e.name,e.alias,e.avatar,e.thumb_avatar
		FROM mc_room_message_batch_send_employee a
		JOIN mc_room_message_batch_send b ON b.tenant_id=? AND b.corp_id=? AND b.id=a.batch_id AND b.deleted_at IS NULL
		JOIN mc_work_employee e ON e.id=a.employee_id AND e.corp_id=b.corp_id AND e.deleted_at IS NULL
		WHERE a.batch_id=? ORDER BY a.id DESC`, principal.TenantID, principal.CorpID, batchID)
	if err != nil {
		return dashboard.RoomMessageBatchSendOwnerPage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.RoomMessageBatchSendOwnerItem, 0)
	status := roomBatchDurableBatchStatus(snapshot.Operation.Status)
	sendTime := ""
	if snapshot.Operation.FinishedAt != nil {
		sendTime = snapshot.Operation.FinishedAt.Format("2006-01-02 15:04:05")
	}
	for rows.Next() {
		var item dashboard.RoomMessageBatchSendOwnerItem
		var name, alias, avatar, thumb sql.NullString
		if err := rows.Scan(&item.ID, &item.SendRoomTotal, &item.EmployeeID, &name, &alias, &avatar, &thumb); err != nil {
			return dashboard.RoomMessageBatchSendOwnerPage{}, err
		}
		item.Status, item.SendTime = status, sendTime
		item.EmployeeName, item.EmployeeAlias = nullString(name), nullString(alias)
		item.EmployeeAvatar, item.EmployeeThumbAvatar = nullString(avatar), nullString(thumb)
		if filter.SendStatus != nil && item.Status != *filter.SendStatus {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomMessageBatchSendOwnerPage{}, err
	}
	total := len(items)
	start := (filter.Page - 1) * filter.PerPage
	if start > total {
		start = total
	}
	end := start + filter.PerPage
	if end > total {
		end = total
	}
	pageItems := items[start:end]
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	if err := tx.Commit(); err != nil {
		return dashboard.RoomMessageBatchSendOwnerPage{}, err
	}
	return dashboard.RoomMessageBatchSendOwnerPage{Items: pageItems, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) RoomBatchDurableReceivePage(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID int, filter dashboard.RoomMessageBatchSendRoomFilter) (dashboard.RoomMessageBatchSendRoomPage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.RoomMessageBatchSendRoomPage{}, err
	}
	defer rollbackQuietly(tx)
	snapshot, found, err := s.roomBatchDurableSnapshotTx(ctx, tx, principal, batchID, false)
	if err != nil {
		return dashboard.RoomMessageBatchSendRoomPage{}, err
	}
	if !found {
		return dashboard.RoomMessageBatchSendRoomPage{}, sql.ErrNoRows
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT a.id,a.room_id,a.room_name,a.chat_id,a.employee_id,a.room_employee_num,a.room_create_time,
		       e.name,e.alias
		FROM mc_room_message_batch_send_result a
		JOIN mc_room_message_batch_send b ON b.tenant_id=? AND b.corp_id=? AND b.id=a.batch_id AND b.deleted_at IS NULL
		JOIN mc_work_employee e ON e.id=a.employee_id AND e.corp_id=b.corp_id AND e.deleted_at IS NULL
		WHERE a.batch_id=? ORDER BY a.id DESC`, principal.TenantID, principal.CorpID, batchID)
	if err != nil {
		return dashboard.RoomMessageBatchSendRoomPage{}, err
	}
	defer rows.Close()
	resultStatuses := roomBatchResultStatusMap(snapshot.Results)
	status := roomBatchDurableBatchStatus(snapshot.Operation.Status)
	sendTime := int64(0)
	if snapshot.Operation.FinishedAt != nil {
		sendTime = snapshot.Operation.FinishedAt.Unix()
	}
	keyword := strings.ToLower(strings.TrimSpace(filter.KeyWords))
	items := make([]dashboard.RoomMessageBatchSendRoomItem, 0)
	for rows.Next() {
		var item dashboard.RoomMessageBatchSendRoomItem
		var roomName, employeeName, employeeAlias sql.NullString
		var chatID string
		var roomCreateTime sql.NullTime
		if err := rows.Scan(&item.ID, &item.RoomID, &roomName, &chatID, &item.EmployeeID, &item.RoomEmployeeNum, &roomCreateTime, &employeeName, &employeeAlias); err != nil {
			return dashboard.RoomMessageBatchSendRoomPage{}, err
		}
		item.RoomName = nullString(roomName)
		item.RoomCreateTime = formatTime(roomCreateTime)
		item.EmployeeName, item.EmployeeAlias = nullString(employeeName), nullString(employeeAlias)
		item.Status = roomBatchDurableResultStatus(snapshot.Operation.Status, resultStatuses, item.EmployeeID, chatID)
		if status == 3 && item.Status == 1 {
			item.Status = status
		}
		item.SendTime = int(sendTime)
		if keyword != "" && !strings.Contains(strings.ToLower(item.RoomName), keyword) {
			continue
		}
		if filter.SendStatus != nil && item.Status != *filter.SendStatus {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.RoomMessageBatchSendRoomPage{}, err
	}
	total := len(items)
	start := (filter.Page - 1) * filter.PerPage
	if start > total {
		start = total
	}
	end := start + filter.PerPage
	if end > total {
		end = total
	}
	pageItems := items[start:end]
	totalPage := 0
	if total > 0 {
		totalPage = (total + filter.PerPage - 1) / filter.PerPage
	}
	if err := tx.Commit(); err != nil {
		return dashboard.RoomMessageBatchSendRoomPage{}, err
	}
	return dashboard.RoomMessageBatchSendRoomPage{Items: pageItems, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

// RoomMessageBatchSendByIDForPrincipal is the principal-scoped legacy fallback
// read used by the dashboard ownership check when the durable store is bound.
func (s *MySQLStore) RoomMessageBatchSendByIDForPrincipal(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID int) (dashboard.RoomMessageBatchSendItem, bool, error) {
	if s == nil || s.db == nil || principal.TenantID <= 0 || principal.CorpID <= 0 || principal.UserID <= 0 || batchID <= 0 {
		return dashboard.RoomMessageBatchSendItem{}, false, dashboard.ErrRoomBatchBodyScope
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, user_id, medium_id, user_name, employee_ids, batch_title, content,
		       send_way, definite_time, send_time, send_room_total, send_employee_total, send_total,
		       not_send_total, received_total, not_received_total, send_status, created_at
		FROM mc_room_message_batch_send
		WHERE tenant_id=? AND corp_id=? AND id=? AND user_id=? AND deleted_at IS NULL
		LIMIT 1`, principal.TenantID, principal.CorpID, batchID, principal.UserID)
	item, err := scanRoomMessageBatchSendRow(row)
	if err == sql.ErrNoRows {
		return dashboard.RoomMessageBatchSendItem{}, false, nil
	}
	if err != nil {
		return dashboard.RoomMessageBatchSendItem{}, false, err
	}
	return item, true, nil
}

// CancelRoomBatchDurable fences the durable room cancel against the worker
// claim: only a still-pending operation with every dispatch queued may be
// cancelled, and the dispatches, operation, and business row all move in one
// transaction with audit/event transitions.
func (s *MySQLStore) CancelRoomBatchDurable(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	snapshot, found, err := s.roomBatchDurableSnapshotTx(ctx, tx, principal, batchID, true)
	if err != nil {
		return err
	}
	if !found {
		return sql.ErrNoRows
	}
	if snapshot.Operation.Status != wecomcapability.OperationPending {
		return dashboard.ErrRoomBatchConflict
	}
	actor := principal.UserID
	for index := range snapshot.Dispatches {
		dispatch := snapshot.Dispatches[index]
		if dispatch.Status != wecomcapability.DispatchQueued {
			return dashboard.ErrRoomBatchConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_capability_dispatches SET status=?,last_error_code=?,lease_token='',lease_expires_at=NULL,next_poll_at=NULL,updated_at=NOW(6) WHERE tenant_id=? AND corp_id=? AND id=? AND status=?`, wecomcapability.DispatchCancelled, "wecom.room_batch_cancelled", principal.TenantID, principal.CorpID, dispatch.ID, wecomcapability.DispatchQueued); err != nil {
			return err
		}
		auditOperation := snapshot.Operation
		auditOperation.Status = wecomcapability.DispatchCancelled
		auditOperation.ErrorCode = "wecom.room_batch_cancelled"
		if err := appendCapabilityLedgerTransitionTx(ctx, tx, auditOperation, dispatch.Status, "dispatch_cancel", &actor, capabilityActorUser, snapshot.Operation.RequestID, &dispatch.ID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_capability_operations SET status=?,error_code=?,lease_token='',lease_expires_at=NULL,finished_at=NOW(6),updated_at=NOW(6) WHERE tenant_id=? AND corp_id=? AND id=? AND status=?`, wecomcapability.OperationCancelled, "wecom.room_batch_cancelled", principal.TenantID, principal.CorpID, snapshot.Operation.ID, wecomcapability.OperationPending); err != nil {
		return err
	}
	snapshot.Operation.Status = wecomcapability.OperationCancelled
	snapshot.Operation.ErrorCode = "wecom.room_batch_cancelled"
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, snapshot.Operation, wecomcapability.OperationPending, "cancel", &actor, capabilityActorUser, snapshot.Operation.RequestID, nil); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_room_message_batch_send SET deleted_at=NOW(6),updated_at=NOW(6) WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL`, principal.TenantID, principal.CorpID, batchID); err != nil {
		return err
	}
	return tx.Commit()
}
