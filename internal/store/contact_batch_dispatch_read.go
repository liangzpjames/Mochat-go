package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

type contactBatchDurableSnapshot struct {
	Batch      dashboard.ContactMessageBatchSendItem
	Operation  wecomcapability.Operation
	Dispatches []wecomcapability.Dispatch
	Results    []wecomcapability.OperationResult
}

func contactMessageBatchSendByScopedIDTx(ctx context.Context, tx *sql.Tx, tenantID, corpID, batchID int, forUpdate bool) (dashboard.ContactMessageBatchSendItem, bool, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	row := tx.QueryRowContext(ctx, `
		SELECT id, corp_id, user_id, medium_id, batch_title, user_name, employee_ids, filter_params, filter_params_detail, content,
		       send_way, definite_time, send_time, send_employee_total, send_contact_total, send_total,
		       not_send_total, received_total, not_received_total, receive_limit_total, not_friend_total,
		       send_status, created_at
		FROM mc_contact_message_batch_send
		WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL
		LIMIT 1`+suffix, tenantID, corpID, batchID)
	item, err := scanContactMessageBatchSendRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ContactMessageBatchSendItem{}, false, nil
	}
	if err != nil {
		return dashboard.ContactMessageBatchSendItem{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) ContactBatchDurableView(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID int) (dashboard.ContactBatchDurableView, bool, error) {
	if s == nil || s.db == nil || batchID <= 0 {
		return dashboard.ContactBatchDurableView{}, false, dashboard.ErrContactBatchBodyScope
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.ContactBatchDurableView{}, false, err
	}
	defer rollbackQuietly(tx)
	snapshot, found, err := s.contactBatchDurableSnapshotTx(ctx, tx, principal, batchID, false)
	if err != nil || !found {
		return dashboard.ContactBatchDurableView{}, found, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.ContactBatchDurableView{}, false, err
	}
	return dashboard.ContactBatchDurableView{
		Batch: snapshot.Batch, Operation: snapshot.Operation,
		Dispatches: snapshot.Dispatches, Results: snapshot.Results,
	}, true, nil
}

func (s *MySQLStore) contactBatchDurableSnapshotTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, batchID int, forUpdate bool) (contactBatchDurableSnapshot, bool, error) {
	if _, err := s.authorizeContactBatchReadTx(ctx, tx, principal, batchID, forUpdate); err != nil {
		return contactBatchDurableSnapshot{}, false, err
	}
	batch, found, err := contactMessageBatchSendByScopedIDTx(ctx, tx, principal.TenantID, principal.CorpID, batchID, forUpdate)
	if err != nil || !found {
		return contactBatchDurableSnapshot{}, found, err
	}
	operationID, found, err := contactBatchOperationIDTx(ctx, tx, principal.TenantID, principal.CorpID, batchID, forUpdate)
	if err != nil || !found {
		return contactBatchDurableSnapshot{}, false, err
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, forUpdate)
	if err != nil {
		return contactBatchDurableSnapshot{}, false, err
	}
	dispatches, err := queryContactBatchDispatchesTx(ctx, tx, principal.TenantID, principal.CorpID, operationID)
	if err != nil {
		return contactBatchDurableSnapshot{}, false, err
	}
	results, err := queryCapabilityResultsDB(ctx, tx, principal.TenantID, principal.CorpID, operationID)
	if err != nil {
		return contactBatchDurableSnapshot{}, false, err
	}
	operation.Status = wecomcapability.AggregateOperationStatusWithResults(operation.Capability, dispatches, results)
	batch.SendStatus = contactBatchDurableBatchStatus(operation.Status)
	batch.SendTotal = operation.SuccessTotal
	batch.ReceivedTotal = operation.SuccessTotal
	batch.NotReceivedTotal = operation.FailureTotal
	batch.SendContactTotal = operation.TargetTotal
	if operation.FinishedAt != nil {
		batch.SendTime = operation.FinishedAt.Format("2006-01-02 15:04:05")
	}
	return contactBatchDurableSnapshot{Batch: batch, Operation: operation, Dispatches: dispatches, Results: results}, true, nil
}

func (s *MySQLStore) authorizeContactBatchReadTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, batchID int, forUpdate bool) (dashboard.ContactMessageBatchSendItem, error) {
	facts, err := s.checkContactBatchActor(ctx, tx, principal, forUpdate)
	if err != nil {
		return dashboard.ContactMessageBatchSendItem{}, err
	}
	principal.IsSuperAdmin = facts.IsSuperAdmin == 1
	scope, err := s.contactBatchCurrentScopeTx(ctx, tx, principal)
	if err != nil {
		return dashboard.ContactMessageBatchSendItem{}, err
	}
	if !scope.Allowed {
		return dashboard.ContactMessageBatchSendItem{}, dashboard.ErrContactBatchPermissionDenied
	}
	batch, found, err := contactMessageBatchSendByScopedIDTx(ctx, tx, principal.TenantID, principal.CorpID, batchID, forUpdate)
	if err != nil {
		return dashboard.ContactMessageBatchSendItem{}, err
	}
	if !found {
		return dashboard.ContactMessageBatchSendItem{}, dashboard.ErrContactBatchNotFound
	}
	if batch.UserID != principal.UserID {
		return dashboard.ContactMessageBatchSendItem{}, dashboard.ErrContactBatchTargetNotOwned
	}
	if scope.Scope != dashboard.DataScopeTenant && !dashboardEmployeeIDsWithinScope(batch.EmployeeIDs, scope.AllowedEmployeeIDs) {
		return dashboard.ContactMessageBatchSendItem{}, dashboard.ErrContactBatchTargetNotOwned
	}
	return batch, nil
}

func contactBatchOperationIDTx(ctx context.Context, tx *sql.Tx, tenantID, corpID, batchID int, forUpdate bool) (int64, bool, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	var operationID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM mochat_go_wecom_capability_operations WHERE tenant_id=? AND corp_id=? AND capability=? AND request_id=? ORDER BY id DESC LIMIT 1`+suffix, tenantID, corpID, wecomcapability.ContactBatchSend, contactBatchRequestID(batchID)).Scan(&operationID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return operationID, err == nil, err
}

func queryContactBatchDispatchesTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int, operationID int64) ([]wecomcapability.Dispatch, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id,tenant_id,corp_id,operation_id,dispatch_kind,chunk_no,target_id,idempotency_key,status,
		       provider_request_id,provider_message_id,provider_object_id,credential_generation,lease_token,
		       lease_expires_at,attempt,next_poll_at,last_error_code,created_at,updated_at
		FROM mochat_go_wecom_capability_dispatches
		WHERE tenant_id=? AND corp_id=? AND operation_id=? ORDER BY id ASC`, tenantID, corpID, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]wecomcapability.Dispatch, 0)
	for rows.Next() {
		item, err := scanCapabilityDispatchRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanCapabilityDispatchRow(scanner capabilityOperationScanner) (wecomcapability.Dispatch, error) {
	var item wecomcapability.Dispatch
	var leaseExpires, nextPoll, created, updated sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.TenantID, &item.CorpID, &item.OperationID, &item.DispatchKind, &item.ChunkNo,
		&item.TargetID, &item.IdempotencyKey, &item.Status, &item.ProviderRequestID, &item.ProviderMessageID,
		&item.ProviderObjectID, &item.CredentialVersion, &item.LeaseToken, &leaseExpires, &item.Attempt,
		&nextPoll, &item.LastErrorCode, &created, &updated,
	); err != nil {
		return wecomcapability.Dispatch{}, err
	}
	item.LeaseExpiresAt, item.NextPollAt = nullableTimePtr(leaseExpires), nullableTimePtr(nextPoll)
	item.CreatedAt, item.UpdatedAt = nullableTimePtr(created), nullableTimePtr(updated)
	return item, nil
}

func contactBatchDurableBatchStatus(status string) int {
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

func contactBatchDurableResultStatus(operationStatus string, results map[string]string, employeeID int, externalID string) int {
	if status, ok := results[contactBatchResultIdentity(employeeID, externalID)]; ok {
		switch status {
		case wecomcapability.DispatchSucceeded:
			return 2
		case wecomcapability.DispatchFailed, wecomcapability.DispatchPartialFailed:
			return 3
		}
	}
	return contactBatchDurableBatchStatus(operationStatus)
}

func contactBatchResultIdentity(employeeID int, externalID string) string {
	return fmt.Sprintf("%d:%s", employeeID, strings.TrimSpace(externalID))
}

func containsPositiveInt(values []int, wanted int) bool {
	for _, value := range values {
		if value == wanted && value > 0 {
			return true
		}
	}
	return false
}

func contactBatchResultStatusMap(results []wecomcapability.OperationResult) map[string]string {
	result := make(map[string]string, len(results))
	for _, item := range results {
		if item.TargetKind != "employee_external_userid" || strings.TrimSpace(item.TargetID) == "" {
			continue
		}
		if current, exists := result[item.TargetID]; !exists || current == wecomcapability.DispatchSucceeded && item.Status != wecomcapability.DispatchSucceeded {
			result[item.TargetID] = item.Status
		}
	}
	return result
}

func (s *MySQLStore) ContactBatchDurableEmployeePage(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID int, filter dashboard.ContactMessageBatchSendEmployeeFilter) (dashboard.ContactMessageBatchSendEmployeePage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.ContactMessageBatchSendEmployeePage{}, err
	}
	defer rollbackQuietly(tx)
	snapshot, found, err := s.contactBatchDurableSnapshotTx(ctx, tx, principal, batchID, false)
	if err != nil {
		return dashboard.ContactMessageBatchSendEmployeePage{}, err
	}
	if !found {
		return dashboard.ContactMessageBatchSendEmployeePage{}, sql.ErrNoRows
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT a.id,a.send_contact_total,e.id,e.name,e.alias,e.avatar,e.thumb_avatar
		FROM mc_contact_message_batch_send_employee a
		JOIN mc_contact_message_batch_send b ON b.tenant_id=? AND b.corp_id=? AND b.id=a.batch_id AND b.deleted_at IS NULL
		JOIN mc_work_employee e ON e.id=a.employee_id AND e.corp_id=b.corp_id AND e.deleted_at IS NULL
		WHERE a.batch_id=? ORDER BY a.id DESC`, principal.TenantID, principal.CorpID, batchID)
	if err != nil {
		return dashboard.ContactMessageBatchSendEmployeePage{}, err
	}
	defer rows.Close()
	items := make([]dashboard.ContactMessageBatchSendEmployeeItem, 0)
	status := contactBatchDurableBatchStatus(snapshot.Operation.Status)
	sendTime := ""
	if snapshot.Operation.FinishedAt != nil {
		sendTime = snapshot.Operation.FinishedAt.Format("2006-01-02 15:04:05")
	}
	keyword := strings.ToLower(strings.TrimSpace(filter.KeyWords))
	for rows.Next() {
		var item dashboard.ContactMessageBatchSendEmployeeItem
		var name, alias, avatar, thumb sql.NullString
		if err := rows.Scan(&item.ID, &item.SendContactTotal, &item.EmployeeID, &name, &alias, &avatar, &thumb); err != nil {
			return dashboard.ContactMessageBatchSendEmployeePage{}, err
		}
		item.Status, item.SendTime = status, sendTime
		item.EmployeeName, item.EmployeeAlias = nullString(name), nullString(alias)
		item.EmployeeAvatar, item.EmployeeThumbAvatar = nullString(avatar), nullString(thumb)
		if keyword != "" && !strings.Contains(strings.ToLower(item.EmployeeName), keyword) {
			continue
		}
		if filter.SendStatus != nil && item.Status != *filter.SendStatus {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactMessageBatchSendEmployeePage{}, err
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
		return dashboard.ContactMessageBatchSendEmployeePage{}, err
	}
	return dashboard.ContactMessageBatchSendEmployeePage{Items: pageItems, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) ContactBatchDurableReceivePage(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID int, filter dashboard.ContactMessageBatchSendReceiveFilter) (dashboard.ContactMessageBatchSendReceivePage, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 {
		filter.PerPage = 15
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.ContactMessageBatchSendReceivePage{}, err
	}
	defer rollbackQuietly(tx)
	snapshot, found, err := s.contactBatchDurableSnapshotTx(ctx, tx, principal, batchID, false)
	if err != nil {
		return dashboard.ContactMessageBatchSendReceivePage{}, err
	}
	if !found {
		return dashboard.ContactMessageBatchSendReceivePage{}, sql.ErrNoRows
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT a.id,a.external_user_id,a.employee_id,a.contact_id,
		       c.name,c.nick_name,c.avatar,e.name,e.alias
		FROM mc_contact_message_batch_send_result a
		JOIN mc_contact_message_batch_send b ON b.tenant_id=? AND b.corp_id=? AND b.id=a.batch_id AND b.deleted_at IS NULL
		JOIN mc_work_contact c ON c.id=a.contact_id AND c.corp_id=b.corp_id AND c.deleted_at IS NULL
		JOIN mc_work_employee e ON e.id=a.employee_id AND e.corp_id=b.corp_id AND e.deleted_at IS NULL
		WHERE a.batch_id=? ORDER BY a.id DESC`, principal.TenantID, principal.CorpID, batchID)
	if err != nil {
		return dashboard.ContactMessageBatchSendReceivePage{}, err
	}
	defer rows.Close()
	resultStatuses := contactBatchResultStatusMap(snapshot.Results)
	status := contactBatchDurableBatchStatus(snapshot.Operation.Status)
	sendTime := int64(0)
	if snapshot.Operation.FinishedAt != nil {
		sendTime = snapshot.Operation.FinishedAt.Unix()
	}
	keyword := strings.ToLower(strings.TrimSpace(filter.KeyWords))
	items := make([]dashboard.ContactMessageBatchSendReceiveItem, 0)
	for rows.Next() {
		var item dashboard.ContactMessageBatchSendReceiveItem
		var externalID string
		var name, nickname, avatar, employeeName, employeeAlias sql.NullString
		if err := rows.Scan(&item.ID, &externalID, &item.EmployeeID, &item.ContactID, &name, &nickname, &avatar, &employeeName, &employeeAlias); err != nil {
			return dashboard.ContactMessageBatchSendReceivePage{}, err
		}
		item.Status = contactBatchDurableResultStatus(snapshot.Operation.Status, resultStatuses, item.EmployeeID, externalID)
		if status == 3 && item.Status == 1 {
			item.Status = status
		}
		item.SendTime = int(sendTime)
		item.ContactName, item.ContactNickName, item.ContactAvatar = nullString(name), nullString(nickname), nullString(avatar)
		item.EmployeeName, item.EmployeeAlias = nullString(employeeName), nullString(employeeAlias)
		if keyword != "" && !strings.Contains(strings.ToLower(item.ContactName+" "+item.ContactNickName+" "+item.EmployeeName), keyword) {
			continue
		}
		if filter.SendStatus != nil && item.Status != *filter.SendStatus {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactMessageBatchSendReceivePage{}, err
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
		return dashboard.ContactMessageBatchSendReceivePage{}, err
	}
	return dashboard.ContactMessageBatchSendReceivePage{Items: pageItems, Total: total, TotalPage: totalPage, PerPage: filter.PerPage}, nil
}

func (s *MySQLStore) CancelContactBatchDurable(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	snapshot, found, err := s.contactBatchDurableSnapshotTx(ctx, tx, principal, batchID, true)
	if err != nil {
		return err
	}
	if !found {
		return sql.ErrNoRows
	}
	if snapshot.Operation.Status != wecomcapability.OperationPending {
		return dashboard.ErrContactBatchConflict
	}
	actor := principal.UserID
	for index := range snapshot.Dispatches {
		dispatch := snapshot.Dispatches[index]
		if dispatch.Status != wecomcapability.DispatchQueued {
			return dashboard.ErrContactBatchConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_capability_dispatches SET status=?,last_error_code=?,lease_token='',lease_expires_at=NULL,next_poll_at=NULL,updated_at=NOW(6) WHERE tenant_id=? AND corp_id=? AND id=? AND status=?`, wecomcapability.DispatchCancelled, "wecom.contact_batch_cancelled", principal.TenantID, principal.CorpID, dispatch.ID, wecomcapability.DispatchQueued); err != nil {
			return err
		}
		auditOperation := snapshot.Operation
		auditOperation.Status = wecomcapability.DispatchCancelled
		auditOperation.ErrorCode = "wecom.contact_batch_cancelled"
		if err := appendCapabilityLedgerTransitionTx(ctx, tx, auditOperation, dispatch.Status, "dispatch_cancel", &actor, capabilityActorUser, snapshot.Operation.RequestID, &dispatch.ID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_capability_operations SET status=?,error_code=?,lease_token='',lease_expires_at=NULL,finished_at=NOW(6),updated_at=NOW(6) WHERE tenant_id=? AND corp_id=? AND id=? AND status=?`, wecomcapability.OperationCancelled, "wecom.contact_batch_cancelled", principal.TenantID, principal.CorpID, snapshot.Operation.ID, wecomcapability.OperationPending); err != nil {
		return err
	}
	snapshot.Operation.Status = wecomcapability.OperationCancelled
	snapshot.Operation.ErrorCode = "wecom.contact_batch_cancelled"
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, snapshot.Operation, wecomcapability.OperationPending, "cancel", &actor, capabilityActorUser, snapshot.Operation.RequestID, nil); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_contact_message_batch_send SET deleted_at=NOW(6),updated_at=NOW(6) WHERE tenant_id=? AND corp_id=? AND id=? AND deleted_at IS NULL`, principal.TenantID, principal.CorpID, batchID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) PrepareContactBatchDurableReminder(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID, employeeID int, requestID string) (dashboard.ContactBatchDurableReminder, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	defer rollbackQuietly(tx)
	snapshot, found, err := s.contactBatchDurableSnapshotTx(ctx, tx, principal, batchID, true)
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	if !found {
		return dashboard.ContactBatchDurableReminder{}, sql.ErrNoRows
	}
	if snapshot.Operation.Status != wecomcapability.OperationSubmitted && snapshot.Operation.Status != wecomcapability.OperationPolling {
		return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchConflict
	}
	facts, err := s.checkContactBatchActor(ctx, tx, principal, true)
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	scope, err := s.contactBatchCurrentScopeTx(ctx, tx, dashboardprincipal.DashboardPrincipal{
		UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID,
		AuthVersion: principal.AuthVersion, IsSuperAdmin: facts.IsSuperAdmin == 1,
	})
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	if !scope.Allowed {
		return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchPermissionDenied
	}
	if employeeID > 0 {
		if !containsPositiveInt(snapshot.Batch.EmployeeIDs, employeeID) || (scope.Scope != dashboard.DataScopeTenant && !dashboardEmployeeIDsWithinScope([]int{employeeID}, scope.AllowedEmployeeIDs)) {
			return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchTargetNotOwned
		}
	} else if scope.Scope != dashboard.DataScopeTenant && !dashboardEmployeeIDsWithinScope(snapshot.Batch.EmployeeIDs, scope.AllowedEmployeeIDs) {
		return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchTargetNotOwned
	}
	recipients, err := contactBatchReminderRecipientsTx(ctx, tx, principal.TenantID, principal.CorpID, batchID, employeeID)
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	if len(recipients) == 0 {
		return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchTargetNotOwned
	}
	agent, found, err := s.contactBatchReminderAgentTx(ctx, tx, principal.TenantID, principal.CorpID)
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	if !found || strings.TrimSpace(agent.WXCorpID) == "" || strings.TrimSpace(agent.WXAgentID) == "" || strings.TrimSpace(agent.WXSecret) == "" {
		return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchCapabilityLimited
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	generation := credentialGenerationForCapability(binding, wecomcapability.AgentMessage)
	if generation == 0 {
		return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchCapabilityLimited
	}
	reminder, err := s.createContactBatchReminderAttemptTx(ctx, tx, principal, snapshot.Batch.CreatedAt, batchID, employeeID, requestID, agent, len(recipients), generation)
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	reminder.Agent = agent
	reminder.Recipients = recipients
	reminder.CreatedAt = snapshot.Batch.CreatedAt
	if reminder.AlreadyCompleted {
		if err := tx.Commit(); err != nil {
			return dashboard.ContactBatchDurableReminder{}, err
		}
		return reminder, nil
	}
	actor := principal.UserID
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, snapshot.Operation, snapshot.Operation.Status, "contact_batch_remind_requested", &actor, capabilityActorUser, snapshot.Operation.RequestID, nil); err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	return reminder, nil
}

func contactBatchReminderIdempotencyKey(batchID, employeeID int, requestID, createdAt string) string {
	if strings.TrimSpace(requestID) != "" {
		return "contact-remind:request:" + strings.TrimSpace(requestID)
	}
	return fmt.Sprintf("contact-remind:batch:%d:employee:%d:created:%s", batchID, employeeID, strings.TrimSpace(createdAt))
}

func (s *MySQLStore) createContactBatchReminderAttemptTx(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, createdAt string, batchID, employeeID int, requestID string, agent dashboard.RoomTagPullAgentCredential, targetTotal int, generation uint64) (dashboard.ContactBatchDurableReminder, error) {
	if targetTotal <= 0 || generation == 0 || principal.UserID <= 0 {
		return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchCapabilityLimited
	}
	idempotencyKey := contactBatchReminderIdempotencyKey(batchID, employeeID, requestID, createdAt)
	leaseToken, err := newCapabilityLeaseToken()
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_capability_operations
		(tenant_id,corp_id,capability,action,credential_group,credential_generation,idempotency_key,status,
		 provider_object_id,actual_agent_id,target_total,actor_user_id,actor_source,request_id,lease_token,
		 lease_expires_at,attempt,requested_at,started_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,DATE_ADD(NOW(6), INTERVAL 5 MINUTE),1,NOW(6),NOW(6))
		ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`,
		principal.TenantID, principal.CorpID, wecomcapability.AgentMessage, wecomcapability.ActionSend,
		wecomcapability.CredentialGroupAgent, generation, idempotencyKey, wecomcapability.OperationSubmitting,
		strings.TrimSpace(agent.WXAgentID), strings.TrimSpace(agent.WXAgentID), targetTotal, principal.UserID, capabilityActorUser,
		strings.TrimSpace(requestID), leaseToken)
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	operationID, err := result.LastInsertId()
	if err != nil || operationID <= 0 {
		return dashboard.ContactBatchDurableReminder{}, ErrCapabilityOperationConflict
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, operationID, true)
	if err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	if operation.Capability != wecomcapability.AgentMessage || operation.Action != wecomcapability.ActionSend || operation.CredentialVersion != generation || operation.IdempotencyKey != idempotencyKey {
		return dashboard.ContactBatchDurableReminder{}, ErrCapabilityOperationConflict
	}
	if operation.Status == wecomcapability.OperationSucceeded {
		return dashboard.ContactBatchDurableReminder{OperationID: operation.ID, IdempotencyKey: idempotencyKey, AlreadyCompleted: true}, nil
	}
	if operation.Status != wecomcapability.OperationSubmitting || operation.Attempt <= 0 || strings.TrimSpace(operation.LeaseToken) == "" {
		return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchReminderReconcileRequired
	}
	if operation.Attempt != 1 && operation.LeaseToken != leaseToken {
		return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchReminderReconcileRequired
	}
	if operation.LeaseToken != leaseToken {
		return dashboard.ContactBatchDurableReminder{}, dashboard.ErrContactBatchReminderReconcileRequired
	}
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, "", "contact_batch_remind_attempt", &principal.UserID, capabilityActorUser, requestID, nil); err != nil {
		return dashboard.ContactBatchDurableReminder{}, err
	}
	return dashboard.ContactBatchDurableReminder{OperationID: operation.ID, LeaseToken: operation.LeaseToken, Attempt: operation.Attempt, IdempotencyKey: idempotencyKey}, nil
}

func (s *MySQLStore) RecordContactBatchDurableReminder(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, reminder dashboard.ContactBatchDurableReminder, errorCode string, success bool, successTotal, failureTotal int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	if reminder.OperationID <= 0 || !validLedgerToken(reminder.LeaseToken, 128) || reminder.Attempt <= 0 || successTotal < 0 || failureTotal < 0 || successTotal+failureTotal <= 0 {
		return companyprofile.ErrInvalidRequest
	}
	operation, err := queryCapabilityOperationTx(ctx, tx, principal.TenantID, principal.CorpID, reminder.OperationID, true)
	if err != nil {
		return err
	}
	if operation.Capability != wecomcapability.AgentMessage || operation.Action != wecomcapability.ActionSend || operation.LeaseToken != reminder.LeaseToken || operation.Attempt != reminder.Attempt {
		return ErrCapabilityOperationStale
	}
	if operation.Status != wecomcapability.OperationSubmitting {
		if operation.Status == wecomcapability.OperationSucceeded && success && operation.SuccessTotal == successTotal && operation.FailureTotal == failureTotal {
			return tx.Commit()
		}
		if (operation.Status == wecomcapability.OperationFailed || operation.Status == wecomcapability.OperationPartialFailed) && !success && operation.SuccessTotal == successTotal && operation.FailureTotal == failureTotal {
			return tx.Commit()
		}
		return ErrCapabilityOperationStale
	}
	if !validCapabilityMachineCode(errorCode, 96) || success && (failureTotal != 0 || successTotal != operation.TargetTotal) || !success && failureTotal == 0 {
		return companyprofile.ErrInvalidRequest
	}
	if success {
		errorCode = ""
	}
	status := wecomcapability.OperationFailed
	if success {
		status = wecomcapability.OperationSucceeded
	} else if successTotal > 0 {
		status = wecomcapability.OperationPartialFailed
	}
	operation.ErrorCode = errorCode
	operation.Status = status
	operation.SuccessTotal = successTotal
	operation.FailureTotal = failureTotal
	updated, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_wecom_capability_operations
		SET status=?,external_success=?,success_total=?,failure_total=?,error_code=?,lease_token='',lease_expires_at=NULL,finished_at=NOW(6),updated_at=NOW(6)
		WHERE tenant_id=? AND corp_id=? AND id=? AND status=? AND lease_token=? AND attempt=?
		  AND lease_expires_at IS NOT NULL AND lease_expires_at > NOW(6)`,
		status, success, successTotal, failureTotal, errorCode, principal.TenantID, principal.CorpID, reminder.OperationID,
		wecomcapability.OperationSubmitting, reminder.LeaseToken, reminder.Attempt)
	if err != nil {
		return err
	}
	if err := requireCompanyRows(updated, 1); err != nil {
		return ErrCapabilityOperationStale
	}
	operation.LeaseToken = ""
	if err := appendCapabilityLedgerTransitionTx(ctx, tx, operation, wecomcapability.OperationSubmitting, "contact_batch_remind_result", &principal.UserID, capabilityActorUser, operation.RequestID, nil); err != nil {
		return err
	}
	return tx.Commit()
}

func contactBatchReminderRecipientsTx(ctx context.Context, tx *sql.Tx, tenantID, corpID, batchID, employeeID int) ([]string, error) {
	args := []any{tenantID, corpID, batchID}
	filter := ""
	if employeeID > 0 {
		filter = " AND a.employee_id=?"
		args = append(args, employeeID)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT a.employee_id, COALESCE(e.wx_user_id,'')
		FROM mc_contact_message_batch_send_employee a
		JOIN mc_contact_message_batch_send b ON b.tenant_id=? AND b.corp_id=? AND b.id=a.batch_id AND b.deleted_at IS NULL
		JOIN mc_work_employee e ON e.id=a.employee_id AND e.corp_id=b.corp_id AND e.deleted_at IS NULL
		WHERE a.batch_id=?`+filter+` ORDER BY a.employee_id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	recipients := make([]string, 0)
	for rows.Next() {
		var employeeID int
		var wxUserID string
		if err := rows.Scan(&employeeID, &wxUserID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(wxUserID) != "" {
			recipients = append(recipients, strings.TrimSpace(wxUserID))
		}
	}
	return recipients, rows.Err()
}

func (s *MySQLStore) contactBatchReminderAgentTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int) (dashboard.RoomTagPullAgentCredential, bool, error) {
	var agentID int
	if err := tx.QueryRowContext(ctx, `
		SELECT a.id FROM mc_work_agent a
		JOIN mc_corp c ON c.id=a.corp_id AND c.tenant_id=? AND c.deleted_at IS NULL
		WHERE a.corp_id=? AND `+authoritativeApplicationAgentSelectionSQL()+` LIMIT 1 FOR UPDATE`, tenantID, corpID).Scan(&agentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.RoomTagPullAgentCredential{}, false, nil
		}
		return dashboard.RoomTagPullAgentCredential{}, false, err
	}
	agent, found, err := s.loadAgentCredentialByID(ctx, tx, agentID, true)
	if err != nil || !found || agent.TenantID != tenantID || agent.CorpID != corpID {
		return dashboard.RoomTagPullAgentCredential{}, found, err
	}
	secret, err := s.decodeAgentCredential(agent)
	if err != nil {
		return dashboard.RoomTagPullAgentCredential{}, false, nil
	}
	return dashboard.RoomTagPullAgentCredential{CorpID: corpID, WXCorpID: agent.WXCorpID, WXAgentID: agent.WXAgentID, WXSecret: secret.WXSecret}, true, nil
}
