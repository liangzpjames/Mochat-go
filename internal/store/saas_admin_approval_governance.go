package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

type saasAdminApprovalGovernanceQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const saasAdminApprovalPolicySelect = `
	SELECT action_type, enabled, amount_threshold_cents, required_approvals,
		sla_minutes, reminder_minutes, expiry_hours, version, updated_by, updated_at
	FROM mochat_go_saas_admin_approval_policies
`

func (s *MySQLStore) SaaSAdminApprovalPolicies(ctx context.Context) ([]dashboard.SaaSAdminApprovalPolicy, error) {
	rows, err := s.db.QueryContext(ctx, saasAdminApprovalPolicySelect+` ORDER BY action_type ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSAdminApprovalPolicy, 0)
	for rows.Next() {
		item, err := scanSaaSAdminApprovalPolicy(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) SaaSAdminApprovalPolicy(ctx context.Context, actionType string) (dashboard.SaaSAdminApprovalPolicy, bool, error) {
	return saasAdminApprovalPolicyByAction(ctx, s.db, actionType, false)
}

func (s *MySQLStore) UpdateSaaSAdminApprovalPolicy(ctx context.Context, input dashboard.SaaSAdminApprovalPolicyUpdate) (dashboard.SaaSAdminApprovalPolicyUpdateResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminApprovalPolicyUpdateResult{}, err
	}
	defer rollbackQuietly(tx)
	before, found, err := saasAdminApprovalPolicyByAction(ctx, tx, input.ActionType, true)
	if err != nil {
		return dashboard.SaaSAdminApprovalPolicyUpdateResult{}, err
	}
	if !found {
		return dashboard.SaaSAdminApprovalPolicyUpdateResult{}, dashboard.NewSaaSAdminNotFound("审批策略不存在")
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSAdminApprovalPolicyUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批策略版本已变化，请刷新后重试"}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_approval_policies
		SET enabled = ?, amount_threshold_cents = ?, required_approvals = ?, sla_minutes = ?,
			reminder_minutes = ?, expiry_hours = ?, version = version + 1, updated_by = ?, updated_at = NOW()
		WHERE action_type = ? AND version = ?
	`, boolTinyInt(input.Enabled), input.AmountThresholdCents, input.RequiredApprovals, input.SLAMinutes,
		input.ReminderMinutes, input.ExpiryHours, input.ActorUserID, input.ActionType, input.ExpectedVersion)
	if err != nil {
		return dashboard.SaaSAdminApprovalPolicyUpdateResult{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return dashboard.SaaSAdminApprovalPolicyUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批策略版本已变化，请刷新后重试"}
	}
	after, _, err := saasAdminApprovalPolicyByAction(ctx, tx, input.ActionType, false)
	if err != nil {
		return dashboard.SaaSAdminApprovalPolicyUpdateResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionApprovalPolicySave, TargetType: dashboard.SaaSAdminOperationTargetApprovalPolicy,
		TargetID: input.ActionType, TargetName: input.ActionType, BeforeJSON: saasAdminMarshalJSON(approvalPolicyAuditPayload(before)),
		AfterJSON: saasAdminMarshalJSON(approvalPolicyAuditPayload(after)), Remark: "save SaaS admin approval policy",
	})
	if err != nil {
		return dashboard.SaaSAdminApprovalPolicyUpdateResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSAdminApprovalPolicyUpdateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminApprovalPolicyUpdateResult{}, err
	}
	return dashboard.SaaSAdminApprovalPolicyUpdateResult{Policy: after, OperationID: operationID}, nil
}

func (s *MySQLStore) SaaSAdminApprovalDecisions(ctx context.Context, approvalID int64, limit int) ([]dashboard.SaaSAdminApprovalDecisionRecord, error) {
	if approvalID <= 0 {
		return nil, dashboard.NewSaaSAdminBadRequest("approvalId 无效")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.approval_id, d.reviewer_user_id, d.reviewer_tenant_id, COALESCE(reviewer.name, ''),
			d.delegated_from_user_id, COALESCE(delegator.name, ''), d.decision, d.reason, d.created_at
		FROM mochat_go_saas_admin_approval_decisions d
		LEFT JOIN mc_user reviewer ON reviewer.id = d.reviewer_user_id
		LEFT JOIN mc_user delegator ON delegator.id = d.delegated_from_user_id
		WHERE d.approval_id = ?
		ORDER BY d.id ASC LIMIT ?
	`, approvalID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSAdminApprovalDecisionRecord, 0)
	for rows.Next() {
		var item dashboard.SaaSAdminApprovalDecisionRecord
		var createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.ApprovalID, &item.ReviewerUserID, &item.ReviewerTenantID, &item.ReviewerName,
			&item.DelegatedFromUserID, &item.DelegatedFromName, &item.Decision, &item.Reason, &createdAt); err != nil {
			return nil, err
		}
		item.CreatedAt = formatTime(createdAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) SaaSAdminApprovalDelegations(ctx context.Context, options dashboard.SaaSAdminApprovalDelegationOptions) ([]dashboard.SaaSAdminApprovalDelegation, error) {
	if options.Limit <= 0 || options.Limit > 500 {
		options.Limit = 100
	}
	where := []string{"1 = 1"}
	args := make([]any, 0)
	if options.DelegatorUserID > 0 {
		where = append(where, "d.delegator_user_id = ?")
		args = append(args, options.DelegatorUserID)
	}
	if options.DelegateUserID > 0 {
		where = append(where, "d.delegate_user_id = ?")
		args = append(args, options.DelegateUserID)
	}
	if options.ActiveOnly {
		where = append(where, "d.status = 1", "d.starts_at <= NOW()", "d.ends_at > NOW()")
	}
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, saasAdminApprovalDelegationSelect+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY d.status ASC, d.ends_at DESC, d.id DESC LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSAdminApprovalDelegation, 0)
	for rows.Next() {
		item, err := scanSaaSAdminApprovalDelegation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) UpsertSaaSAdminApprovalDelegation(ctx context.Context, input dashboard.SaaSAdminApprovalDelegationUpsert) (dashboard.SaaSAdminApprovalDelegationUpsertResult, error) {
	if input.DelegatorUserID <= 0 || input.DelegateUserID <= 0 || input.DelegatorUserID == input.DelegateUserID || input.PlatformTenantID <= 0 ||
		input.StartsAt.IsZero() || !input.EndsAt.After(input.StartsAt) || (input.Status != 1 && input.Status != 2) || strings.TrimSpace(input.Reason) == "" {
		return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, dashboard.NewSaaSAdminBadRequest("审批委托字段无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, err
	}
	defer rollbackQuietly(tx)
	eligible, err := saasAdminApprovalReviewerEligible(ctx, tx, input.DelegatorUserID, input.PlatformTenantID)
	if err != nil {
		return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, err
	}
	if !eligible {
		return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "委托人当前没有审批复核权限"}
	}
	if valid, err := saasAdminApprovalPlatformUser(ctx, tx, input.DelegateUserID, input.PlatformTenantID); err != nil {
		return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, err
	} else if !valid {
		return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "受托人不是有效平台成员"}
	}
	created := input.ID == 0
	delegationID := input.ID
	var before dashboard.SaaSAdminApprovalDelegation
	if !created {
		before, err = saasAdminApprovalDelegationByID(ctx, tx, input.ID, true)
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, dashboard.NewSaaSAdminNotFound("审批委托不存在")
		}
		if err != nil {
			return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, err
		}
		if before.Version != input.ExpectedVersion {
			return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批委托版本已变化，请刷新后重试"}
		}
	}
	if input.Status == 1 {
		var conflictID int64
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM mochat_go_saas_admin_approval_delegations
			WHERE id <> ? AND status = 1 AND starts_at < ? AND ends_at > ?
				AND (delegator_user_id IN (?, ?) OR delegate_user_id IN (?, ?))
			LIMIT 1 FOR UPDATE
		`, input.ID, input.EndsAt, input.StartsAt, input.DelegatorUserID, input.DelegateUserID, input.DelegatorUserID, input.DelegateUserID).Scan(&conflictID)
		if err == nil {
			return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "委托时间与现有有效委托重叠"}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, err
		}
	}
	if created {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_admin_approval_delegations
				(delegator_user_id, delegate_user_id, starts_at, ends_at, status, reason, version, created_by, updated_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, NOW(), NOW())
		`, input.DelegatorUserID, input.DelegateUserID, input.StartsAt, input.EndsAt, input.Status, truncateRunes(input.Reason, 255), input.ActorUserID, input.ActorUserID)
		if err != nil {
			return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, err
		}
		delegationID, _ = result.LastInsertId()
	} else {
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_admin_approval_delegations
			SET delegator_user_id = ?, delegate_user_id = ?, starts_at = ?, ends_at = ?, status = ?, reason = ?,
				version = version + 1, updated_by = ?, updated_at = NOW()
			WHERE id = ? AND version = ?
		`, input.DelegatorUserID, input.DelegateUserID, input.StartsAt, input.EndsAt, input.Status, truncateRunes(input.Reason, 255),
			input.ActorUserID, input.ID, input.ExpectedVersion)
		if err != nil {
			return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批委托版本已变化，请刷新后重试"}
		}
	}
	after, err := saasAdminApprovalDelegationByID(ctx, tx, delegationID, false)
	if err != nil {
		return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionApprovalDelegationSave, TargetType: dashboard.SaaSAdminOperationTargetApprovalDelegation,
		TargetID: strconv.FormatInt(delegationID, 10), TargetName: after.DelegatorName + " -> " + after.DelegateName,
		BeforeJSON: saasAdminMarshalJSON(approvalDelegationAuditPayload(before)), AfterJSON: saasAdminMarshalJSON(approvalDelegationAuditPayload(after)),
		Remark: "save SaaS admin approval delegation",
	})
	if err != nil {
		return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminApprovalDelegationUpsertResult{}, err
	}
	return dashboard.SaaSAdminApprovalDelegationUpsertResult{Delegation: after, OperationID: operationID, Created: created}, nil
}

func (s *MySQLStore) ResolveSaaSAdminApprovalDelegation(ctx context.Context, delegateUserID int, platformTenantID int, at time.Time) (dashboard.SaaSAdminApprovalDelegation, bool, error) {
	if delegateUserID <= 0 || platformTenantID <= 0 {
		return dashboard.SaaSAdminApprovalDelegation{}, false, nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	item, err := scanSaaSAdminApprovalDelegation(s.db.QueryRowContext(ctx, saasAdminApprovalDelegationSelect+`
		WHERE d.delegate_user_id = ? AND d.status = 1 AND d.starts_at <= ? AND d.ends_at > ?
			AND delegate.tenant_id = ? AND delegate.deleted_at IS NULL
			AND delegator.tenant_id = ? AND delegator.deleted_at IS NULL
			AND (delegator.isSuperAdmin = 1 OR EXISTS (
				SELECT 1 FROM mochat_go_saas_admin_user_roles ur
				INNER JOIN mochat_go_saas_admin_roles r ON r.id = ur.role_id AND r.status = 1
				INNER JOIN mochat_go_saas_admin_role_permissions rp ON rp.role_id = r.id
				WHERE ur.user_id = delegator.id AND rp.permission_code = ?
			))
		ORDER BY d.ends_at ASC, d.id ASC LIMIT 1
	`, delegateUserID, at, at, platformTenantID, platformTenantID, dashboard.SaaSAdminPermissionApprovalsReview))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminApprovalDelegation{}, false, nil
	}
	return item, err == nil, err
}

func (s *MySQLStore) CreateSaaSAdminApprovalReminders(ctx context.Context, input dashboard.SaaSAdminApprovalReminderCreate) (dashboard.SaaSAdminApprovalReminderResult, error) {
	if input.PlatformTenantID <= 0 {
		return dashboard.SaaSAdminApprovalReminderResult{}, dashboard.NewSaaSAdminBadRequest("platformTenantId 无效")
	}
	if input.Limit <= 0 || input.Limit > 500 {
		input.Limit = 100
	}
	if input.MaxAttempts <= 0 || input.MaxAttempts > 20 {
		input.MaxAttempts = 3
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminApprovalReminderResult{}, err
	}
	defer rollbackQuietly(tx)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, request_no, action_type, target_name, requester_user_id, approval_count, required_approvals,
			reminder_minutes, sla_due_at, next_reminder_at, reminder_count, created_at, status
		FROM mochat_go_saas_admin_approvals
		WHERE status = 'pending' AND effect_applied_at IS NULL AND expires_at > NOW()
			AND next_reminder_at IS NOT NULL AND next_reminder_at <= NOW()
		ORDER BY next_reminder_at ASC, id ASC LIMIT ? FOR UPDATE
	`, input.Limit)
	if err != nil {
		return dashboard.SaaSAdminApprovalReminderResult{}, err
	}
	type candidate struct {
		id                                                                                int64
		requestNo, actionType, targetName, status                                         string
		requesterUserID, approvalCount, requiredApprovals, reminderMinutes, reminderCount int
		slaDueAt, nextReminderAt, createdAt                                               time.Time
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.requestNo, &item.actionType, &item.targetName, &item.requesterUserID,
			&item.approvalCount, &item.requiredApprovals, &item.reminderMinutes, &item.slaDueAt, &item.nextReminderAt,
			&item.reminderCount, &item.createdAt, &item.status); err != nil {
			rows.Close()
			return dashboard.SaaSAdminApprovalReminderResult{}, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Close(); err != nil {
		return dashboard.SaaSAdminApprovalReminderResult{}, err
	}
	result := dashboard.SaaSAdminApprovalReminderResult{Scanned: len(candidates), NotificationKeys: make([]string, 0, len(candidates))}
	now := time.Now()
	for _, item := range candidates {
		severity := dashboard.SaaSAlertSeverityWarning
		if !now.Before(item.slaDueAt) {
			severity = dashboard.SaaSAlertSeverityCritical
		}
		sequence := item.reminderCount + 1
		periodKey := fmt.Sprintf("approval_%d_reminder_%d", item.id, sequence)
		message := fmt.Sprintf("审批单 %s 等待会签，当前 %d/%d，请审批人处理。", item.requestNo, item.approvalCount, item.requiredApprovals)
		if severity == dashboard.SaaSAlertSeverityCritical {
			message = fmt.Sprintf("审批单 %s 已超过 SLA，当前会签 %d/%d，请立即处理。", item.requestNo, item.approvalCount, item.requiredApprovals)
		}
		alert := dashboard.SaaSQuotaAlert{
			Status: dashboard.SaaSQuotaStatus{TenantID: input.PlatformTenantID, Metric: dashboard.SaaSEventMetricApprovalSLA,
				Current: int64(now.Sub(item.createdAt) / time.Minute), Limit: int64(item.slaDueAt.Sub(item.createdAt) / time.Minute)},
			AlertType: dashboard.SaaSAlertTypeApprovalSLA, Severity: severity, PeriodKey: periodKey,
			Source: "saas_admin.approval.sla_reminder", Message: message,
			Context: map[string]any{"approvalId": item.id, "requestNo": item.requestNo, "actionType": item.actionType,
				"targetName": item.targetName, "requesterUserId": item.requesterUserID, "approvalCount": item.approvalCount,
				"requiredApprovals": item.requiredApprovals, "slaDueAt": item.slaDueAt, "reminderSequence": sequence},
		}
		raw, err := json.Marshal(alert)
		if err != nil {
			return dashboard.SaaSAdminApprovalReminderResult{}, err
		}
		notificationKey := dashboard.SaaSAlertNotificationKey(alert, dashboard.SaaSAlertNotificationChannelWebhook)
		alertKey := fmt.Sprintf("%d:%s:%s:%s", input.PlatformTenantID, dashboard.SaaSEventMetricApprovalSLA, dashboard.SaaSAlertTypeApprovalSLA, periodKey)
		insert, err := tx.ExecContext(ctx, `
			INSERT IGNORE INTO mochat_go_saas_alert_notifications
				(notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json,
				 last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, ?, ?, 0, ?, ?, '', NOW(), NULL, NOW(), NOW(), NULL)
		`, notificationKey, alertKey, input.PlatformTenantID, dashboard.SaaSAlertNotificationChannelWebhook,
			dashboard.SaaSAlertNotificationStatusPending, input.MaxAttempts, string(raw))
		if err != nil {
			return dashboard.SaaSAdminApprovalReminderResult{}, err
		}
		if affected, _ := insert.RowsAffected(); affected == 1 {
			result.Enqueued++
			result.NotificationKeys = append(result.NotificationKeys, notificationKey)
		} else {
			result.Skipped++
		}
		nextReminderAt := now.Add(time.Duration(item.reminderMinutes) * time.Minute)
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_admin_approvals
			SET last_reminded_at = ?, reminder_count = reminder_count + 1, next_reminder_at = ?, updated_at = NOW()
			WHERE id = ? AND status = 'pending' AND reminder_count = ?
		`, now, nextReminderAt, item.id, item.reminderCount); err != nil {
			return dashboard.SaaSAdminApprovalReminderResult{}, err
		}
		if err := insertSaaSAdminApprovalEventTx(ctx, tx, item.id, "sla_reminder_enqueued", item.status, item.status,
			input.ActorUserID, input.ActorTenantID, "审批 SLA 提醒已入队", map[string]any{
				"notificationKey": notificationKey, "reminderSequence": sequence, "severity": severity, "nextReminderAt": nextReminderAt,
			}); err != nil {
			return dashboard.SaaSAdminApprovalReminderResult{}, err
		}
	}
	if len(candidates) > 0 {
		_, err = insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
			TenantID: 0, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
			Action: dashboard.SaaSAdminOperationActionApprovalReminder, TargetType: dashboard.SaaSAdminOperationTargetApproval,
			TargetID: "sla-reminders", TargetName: "审批 SLA 提醒", AfterJSON: saasAdminMarshalJSON(result), Remark: "enqueue SaaS admin approval SLA reminders",
		})
		if err != nil {
			return dashboard.SaaSAdminApprovalReminderResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminApprovalReminderResult{}, err
	}
	return result, nil
}

func scanSaaSAdminApprovalPolicy(scanner sqlScanner) (dashboard.SaaSAdminApprovalPolicy, error) {
	var item dashboard.SaaSAdminApprovalPolicy
	var enabled int
	var updatedAt sql.NullTime
	err := scanner.Scan(&item.ActionType, &enabled, &item.AmountThresholdCents, &item.RequiredApprovals,
		&item.SLAMinutes, &item.ReminderMinutes, &item.ExpiryHours, &item.Version, &item.UpdatedBy, &updatedAt)
	item.Enabled = enabled == 1
	item.UpdatedAt = formatTime(updatedAt)
	return item, err
}

func saasAdminApprovalPolicyByAction(ctx context.Context, queryer saasAdminApprovalGovernanceQueryer, actionType string, forUpdate bool) (dashboard.SaaSAdminApprovalPolicy, bool, error) {
	query := saasAdminApprovalPolicySelect + ` WHERE action_type = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	item, err := scanSaaSAdminApprovalPolicy(queryer.QueryRowContext(ctx, query, strings.TrimSpace(actionType)))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminApprovalPolicy{}, false, nil
	}
	return item, err == nil, err
}

const saasAdminApprovalDelegationSelect = `
	SELECT d.id, d.delegator_user_id, COALESCE(delegator.name, ''), d.delegate_user_id, COALESCE(delegate.name, ''),
		d.starts_at, d.ends_at, d.status, d.reason, d.version, d.created_by, d.updated_by, d.created_at, d.updated_at
	FROM mochat_go_saas_admin_approval_delegations d
	LEFT JOIN mc_user delegator ON delegator.id = d.delegator_user_id
	LEFT JOIN mc_user delegate ON delegate.id = d.delegate_user_id
`

func scanSaaSAdminApprovalDelegation(scanner sqlScanner) (dashboard.SaaSAdminApprovalDelegation, error) {
	var item dashboard.SaaSAdminApprovalDelegation
	var startsAt, endsAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.DelegatorUserID, &item.DelegatorName, &item.DelegateUserID, &item.DelegateName,
		&startsAt, &endsAt, &item.Status, &item.Reason, &item.Version, &item.CreatedBy, &item.UpdatedBy, &createdAt, &updatedAt)
	item.StartsAt, item.EndsAt = formatTime(startsAt), formatTime(endsAt)
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	if startsAt.Valid {
		item.StartsAtValue = startsAt.Time
	}
	if endsAt.Valid {
		item.EndsAtValue = endsAt.Time
	}
	return item, err
}

func saasAdminApprovalDelegationByID(ctx context.Context, queryer saasAdminApprovalGovernanceQueryer, id int64, forUpdate bool) (dashboard.SaaSAdminApprovalDelegation, error) {
	query := saasAdminApprovalDelegationSelect + ` WHERE d.id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanSaaSAdminApprovalDelegation(queryer.QueryRowContext(ctx, query, id))
}

func saasAdminApprovalPlatformUser(ctx context.Context, queryer saasAdminApprovalGovernanceQueryer, userID, platformTenantID int) (bool, error) {
	var count int
	err := queryer.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mc_user WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, userID, platformTenantID).Scan(&count)
	return count == 1, err
}

func saasAdminApprovalReviewerEligible(ctx context.Context, queryer saasAdminApprovalGovernanceQueryer, userID, platformTenantID int) (bool, error) {
	var eligible int
	err := queryer.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM mc_user u
			WHERE u.id = ? AND u.tenant_id = ? AND u.deleted_at IS NULL
				AND (u.isSuperAdmin = 1 OR EXISTS (
					SELECT 1 FROM mochat_go_saas_admin_user_roles ur
					INNER JOIN mochat_go_saas_admin_roles r ON r.id = ur.role_id AND r.status = 1
					INNER JOIN mochat_go_saas_admin_role_permissions rp ON rp.role_id = r.id
					WHERE ur.user_id = u.id AND rp.permission_code = ?
				))
		)
	`, userID, platformTenantID, dashboard.SaaSAdminPermissionApprovalsReview).Scan(&eligible)
	return eligible == 1, err
}

func approvalPolicyAuditPayload(item dashboard.SaaSAdminApprovalPolicy) map[string]any {
	return map[string]any{"actionType": item.ActionType, "enabled": item.Enabled, "amountThresholdCents": item.AmountThresholdCents,
		"requiredApprovals": item.RequiredApprovals, "slaMinutes": item.SLAMinutes, "reminderMinutes": item.ReminderMinutes,
		"expiryHours": item.ExpiryHours, "version": item.Version, "updatedBy": item.UpdatedBy}
}

func approvalDelegationAuditPayload(item dashboard.SaaSAdminApprovalDelegation) map[string]any {
	if item.ID == 0 {
		return nil
	}
	return map[string]any{"id": item.ID, "delegatorUserId": item.DelegatorUserID, "delegateUserId": item.DelegateUserID,
		"startsAt": item.StartsAt, "endsAt": item.EndsAt, "status": item.Status, "reason": item.Reason, "version": item.Version}
}

var _ dashboard.SaaSAdminApprovalGovernanceStore = (*MySQLStore)(nil)
