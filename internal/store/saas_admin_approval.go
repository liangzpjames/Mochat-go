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

const saasAdminApprovalExecutionLease = 15 * time.Minute

func (s *MySQLStore) SaaSAdminApprovals(ctx context.Context, options dashboard.SaaSAdminApprovalOptions) (dashboard.SaaSAdminApprovalReport, error) {
	if err := s.expireSaaSAdminApprovals(ctx); err != nil {
		return dashboard.SaaSAdminApprovalReport{}, err
	}
	where, args := saasAdminApprovalWhere(options)
	var report dashboard.SaaSAdminApprovalReport
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(a.status = 'pending'), 0), COALESCE(SUM(a.status = 'approved'), 0),
			COALESCE(SUM(a.status = 'rejected'), 0), COALESCE(SUM(a.status = 'canceled'), 0),
			COALESCE(SUM(a.status = 'expired'), 0), COALESCE(SUM(a.status = 'executing'), 0),
			COALESCE(SUM(a.status = 'executed'), 0), COALESCE(SUM(a.risk_level = 'critical'), 0)
		FROM mochat_go_saas_admin_approvals a
		WHERE `+where, args...).Scan(
		&report.Summary.Total, &report.Summary.Pending, &report.Summary.Approved, &report.Summary.Rejected,
		&report.Summary.Canceled, &report.Summary.Expired, &report.Summary.Executing, &report.Summary.Executed, &report.Summary.Critical,
	)
	if err != nil {
		return dashboard.SaaSAdminApprovalReport{}, err
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, saasAdminApprovalSelect+`
		WHERE `+where+`
		ORDER BY a.id DESC
		LIMIT ?`, append(args, limit)...)
	if err != nil {
		return dashboard.SaaSAdminApprovalReport{}, err
	}
	defer rows.Close()
	report.Items = make([]dashboard.SaaSAdminApproval, 0)
	for rows.Next() {
		item, err := scanSaaSAdminApproval(rows)
		if err != nil {
			return dashboard.SaaSAdminApprovalReport{}, err
		}
		report.Items = append(report.Items, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminApprovalReport{}, err
	}
	return report, nil
}

func (s *MySQLStore) SaaSAdminApprovalEvents(ctx context.Context, approvalID int64, limit int) ([]dashboard.SaaSAdminApprovalEvent, error) {
	if approvalID <= 0 {
		return nil, dashboard.NewSaaSAdminBadRequest("approvalId 无效")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.approval_id, e.event_type, e.from_status, e.to_status,
			e.actor_user_id, e.actor_tenant_id, COALESCE(u.name, ''), e.reason,
			COALESCE(CAST(e.context_json AS CHAR), ''), e.created_at
		FROM mochat_go_saas_admin_approval_events e
		LEFT JOIN mc_user u ON u.id = e.actor_user_id
		WHERE e.approval_id = ?
		ORDER BY e.id ASC
		LIMIT ?
	`, approvalID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSAdminApprovalEvent, 0)
	for rows.Next() {
		var item dashboard.SaaSAdminApprovalEvent
		var createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.ApprovalID, &item.EventType, &item.FromStatus, &item.ToStatus,
			&item.ActorUserID, &item.ActorTenantID, &item.ActorName, &item.Reason, &item.ContextJSON, &createdAt); err != nil {
			return nil, err
		}
		item.CreatedAt = formatTime(createdAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) CreateSaaSAdminApproval(ctx context.Context, input dashboard.SaaSAdminApprovalCreate) (dashboard.SaaSAdminApprovalCreateResult, error) {
	if strings.TrimSpace(input.RequestNo) == "" || strings.TrimSpace(input.ActionType) == "" || input.RequesterUserID <= 0 || input.RequesterTenantID <= 0 ||
		strings.TrimSpace(input.IdempotencyKey) == "" || strings.TrimSpace(input.RequestSHA256) == "" || strings.TrimSpace(input.RequestJSON) == "" || strings.TrimSpace(input.Reason) == "" ||
		input.ExpiresAt.IsZero() || input.PolicyVersion <= 0 || input.RequiredApprovals < 1 || input.RequiredApprovals > 5 || input.ReminderMinutes < 1 || input.SLADueAt.IsZero() || input.NextReminderAt.IsZero() {
		return dashboard.SaaSAdminApprovalCreateResult{}, dashboard.NewSaaSAdminBadRequest("审批申请字段不完整")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminApprovalCreateResult{}, err
	}
	defer rollbackQuietly(tx)
	existing, found, err := saasAdminApprovalByIdempotencyTx(ctx, tx, input.RequesterUserID, input.IdempotencyKey, true)
	if err != nil {
		return dashboard.SaaSAdminApprovalCreateResult{}, err
	}
	if found {
		if existing.ActionType != input.ActionType || existing.RequestSHA256 != input.RequestSHA256 {
			return dashboard.SaaSAdminApprovalCreateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批幂等键已被不同请求使用"}
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSAdminApprovalCreateResult{}, err
		}
		return dashboard.SaaSAdminApprovalCreateResult{Approval: existing, Idempotent: true}, nil
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_approvals
			(request_no, action_type, risk_level, status, required_permission, policy_version, required_approvals, approval_count, reminder_minutes, target_type, target_id, target_name,
			 requester_user_id, requester_tenant_id, idempotency_key, request_sha256, request_json, reason,
			 sla_due_at, next_reminder_at, expires_at, version, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', ?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, NOW(), NOW())
	`, input.RequestNo, input.ActionType, input.RiskLevel, input.RequiredPermission, input.PolicyVersion, input.RequiredApprovals, input.ReminderMinutes,
		input.TargetType, input.TargetID, input.TargetName, input.RequesterUserID, input.RequesterTenantID, input.IdempotencyKey,
		input.RequestSHA256, input.RequestJSON, truncateRunes(input.Reason, 255), input.SLADueAt, input.NextReminderAt, input.ExpiresAt)
	if isMySQLDuplicateKeyError(err) {
		return dashboard.SaaSAdminApprovalCreateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批单号或幂等键已存在"}
	}
	if err != nil {
		return dashboard.SaaSAdminApprovalCreateResult{}, err
	}
	approvalID, _ := result.LastInsertId()
	if err := insertSaaSAdminApprovalEventTx(ctx, tx, approvalID, "requested", "", dashboard.SaaSAdminApprovalStatusPending, input.RequesterUserID, input.RequesterTenantID, input.Reason, map[string]any{
		"requestSha256": input.RequestSHA256, "policyVersion": input.PolicyVersion, "requiredApprovals": input.RequiredApprovals,
		"slaDueAt": input.SLADueAt, "nextReminderAt": input.NextReminderAt,
	}); err != nil {
		return dashboard.SaaSAdminApprovalCreateResult{}, err
	}
	after, found, err := saasAdminApprovalByIDTx(ctx, tx, approvalID, false)
	if err != nil || !found {
		if err == nil {
			err = errors.New("审批申请写入后无法读取")
		}
		return dashboard.SaaSAdminApprovalCreateResult{}, err
	}
	operationID, err := insertSaaSAdminApprovalAuditTx(ctx, tx, dashboard.SaaSAdminOperationActionApprovalRequest, input.RequesterUserID, input.RequesterTenantID, after, dashboard.SaaSAdminApproval{}, input.Reason)
	if err != nil {
		return dashboard.SaaSAdminApprovalCreateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminApprovalCreateResult{}, err
	}
	return dashboard.SaaSAdminApprovalCreateResult{Approval: after, OperationID: operationID}, nil
}

func (s *MySQLStore) DecideSaaSAdminApproval(ctx context.Context, input dashboard.SaaSAdminApprovalDecision) (dashboard.SaaSAdminApproval, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	defer rollbackQuietly(tx)
	before, found, err := saasAdminApprovalByIDTx(ctx, tx, input.ApprovalID, true)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if !found {
		return dashboard.SaaSAdminApproval{}, dashboard.NewSaaSAdminNotFound("审批不存在")
	}
	if expired, err := expireSaaSAdminApprovalLocked(ctx, tx, &before); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	} else if expired {
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSAdminApproval{}, err
		}
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批已过期"}
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSAdminApproval{}, approvalVersionConflict(before.Version)
	}
	if before.RequesterUserID == input.ActorUserID || (input.DelegatedFromUserID > 0 && before.RequesterUserID == input.DelegatedFromUserID) {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 403, Message: "发起人不能复核自己的审批"}
	}
	if !before.EffectAppliedAtValue.IsZero() {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "业务副作用已提交，不能再变更审批决定"}
	}
	if before.TargetType == dashboard.SaaSAdminOperationTargetAccessAssignment &&
		(before.TargetID == strconv.Itoa(input.ActorUserID) || (input.DelegatedFromUserID > 0 && before.TargetID == strconv.Itoa(input.DelegatedFromUserID))) {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 403, Message: "不能复核授予自己权限的审批"}
	}
	var existingDecisionID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM mochat_go_saas_admin_approval_decisions
		WHERE approval_id = ? AND reviewer_user_id = ? LIMIT 1
	`, before.ID, input.ActorUserID).Scan(&existingDecisionID)
	if err == nil {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "同一审批人不能重复投票"}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminApproval{}, err
	}
	nextStatus := ""
	approvalCount := before.ApprovalCount
	switch input.Decision {
	case dashboard.SaaSAdminApprovalDecisionApprove:
		if before.Status != dashboard.SaaSAdminApprovalStatusPending {
			return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "只有待审批申请可以批准"}
		}
		approvalCount++
		nextStatus = dashboard.SaaSAdminApprovalStatusPending
		if approvalCount >= before.RequiredApprovals {
			nextStatus = dashboard.SaaSAdminApprovalStatusApproved
		}
	case dashboard.SaaSAdminApprovalDecisionReject:
		if before.Status != dashboard.SaaSAdminApprovalStatusPending && before.Status != dashboard.SaaSAdminApprovalStatusApproved {
			return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "当前审批状态不能驳回"}
		}
		nextStatus = dashboard.SaaSAdminApprovalStatusRejected
	default:
		return dashboard.SaaSAdminApproval{}, dashboard.NewSaaSAdminBadRequest("审批决定无效")
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_approval_decisions
			(approval_id, reviewer_user_id, reviewer_tenant_id, delegated_from_user_id, decision, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW())
	`, before.ID, input.ActorUserID, input.ActorTenantID, input.DelegatedFromUserID, input.Decision, truncateRunes(input.Reason, 255))
	if isMySQLDuplicateKeyError(err) {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "同一审批人不能重复投票"}
	}
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	nextReminderAt := any(nil)
	if nextStatus == dashboard.SaaSAdminApprovalStatusPending {
		if before.NextReminderAtValue.IsZero() {
			nextReminderAt = time.Now().Add(time.Duration(before.ReminderMinutes) * time.Minute)
		} else {
			nextReminderAt = before.NextReminderAtValue
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_approvals
		SET status = ?, approval_count = ?, reviewer_user_id = ?, reviewed_at = NOW(), decision_reason = ?,
			next_reminder_at = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, nextStatus, approvalCount, input.ActorUserID, truncateRunes(input.Reason, 255), nextReminderAt, before.ID, before.Version)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return dashboard.SaaSAdminApproval{}, approvalVersionConflict(before.Version)
	}
	if err := insertSaaSAdminApprovalEventTx(ctx, tx, before.ID, "decision_"+input.Decision, before.Status, nextStatus, input.ActorUserID, input.ActorTenantID, input.Reason, map[string]any{
		"approvalCount": approvalCount, "requiredApprovals": before.RequiredApprovals, "delegatedFromUserId": input.DelegatedFromUserID,
	}); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	after, _, err := saasAdminApprovalByIDTx(ctx, tx, before.ID, false)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if _, err := insertSaaSAdminApprovalAuditTx(ctx, tx, dashboard.SaaSAdminOperationActionApprovalDecision, input.ActorUserID, input.ActorTenantID, after, before, input.Reason); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	return after, nil
}

func (s *MySQLStore) CancelSaaSAdminApproval(ctx context.Context, input dashboard.SaaSAdminApprovalCancel) (dashboard.SaaSAdminApproval, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	defer rollbackQuietly(tx)
	before, found, err := saasAdminApprovalByIDTx(ctx, tx, input.ApprovalID, true)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if !found {
		return dashboard.SaaSAdminApproval{}, dashboard.NewSaaSAdminNotFound("审批不存在")
	}
	if before.RequesterUserID != input.ActorUserID {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 403, Message: "只有发起人可以撤回审批"}
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSAdminApproval{}, approvalVersionConflict(before.Version)
	}
	if !before.EffectAppliedAtValue.IsZero() {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "业务副作用已提交，不能撤回审批"}
	}
	if before.Status != dashboard.SaaSAdminApprovalStatusPending && before.Status != dashboard.SaaSAdminApprovalStatusApproved {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "当前审批状态不能撤回"}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_approvals
		SET status = 'canceled', decision_reason = ?, next_reminder_at = NULL, version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, truncateRunes(input.Reason, 255), before.ID, before.Version)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return dashboard.SaaSAdminApproval{}, approvalVersionConflict(before.Version)
	}
	if err := insertSaaSAdminApprovalEventTx(ctx, tx, before.ID, "canceled", before.Status, dashboard.SaaSAdminApprovalStatusCanceled, input.ActorUserID, input.ActorTenantID, input.Reason, nil); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	after, _, err := saasAdminApprovalByIDTx(ctx, tx, before.ID, false)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if _, err := insertSaaSAdminApprovalAuditTx(ctx, tx, dashboard.SaaSAdminOperationActionApprovalCancel, input.ActorUserID, input.ActorTenantID, after, before, input.Reason); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	return after, nil
}

func (s *MySQLStore) BeginSaaSAdminApprovalExecution(ctx context.Context, input dashboard.SaaSAdminApprovalExecutionStart) (dashboard.SaaSAdminApproval, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	defer rollbackQuietly(tx)
	before, found, err := saasAdminApprovalByIDTx(ctx, tx, input.ApprovalID, true)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if !found {
		return dashboard.SaaSAdminApproval{}, dashboard.NewSaaSAdminNotFound("审批不存在")
	}
	if expired, err := expireSaaSAdminApprovalLocked(ctx, tx, &before); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	} else if expired {
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSAdminApproval{}, err
		}
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批已过期"}
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSAdminApproval{}, approvalVersionConflict(before.Version)
	}
	if before.RequesterUserID == input.ActorUserID {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 403, Message: "发起人不能执行自己的审批"}
	}
	if before.ReviewerUserID <= 0 || before.ReviewerUserID == before.RequesterUserID {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批尚未由独立复核人批准"}
	}
	if before.RequiredApprovals < 1 || before.ApprovalCount < before.RequiredApprovals {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批会签人数尚未达到策略要求"}
	}
	if before.TargetType == dashboard.SaaSAdminOperationTargetAccessAssignment && before.TargetID == strconv.Itoa(input.ActorUserID) {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 403, Message: "不能执行授予自己权限的审批"}
	}
	recovered := false
	if before.Status == dashboard.SaaSAdminApprovalStatusExecuting {
		if before.ExecutionStartedValue.IsZero() || time.Since(before.ExecutionStartedValue) < saasAdminApprovalExecutionLease {
			return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批正在执行"}
		}
		recovered = true
	} else if before.Status != dashboard.SaaSAdminApprovalStatusApproved {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "只有已批准审批可以执行"}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_approvals
		SET status = 'executing', execution_user_id = ?, execution_started_at = NOW(), executed_at = NULL,
			execution_attempts = execution_attempts + 1, last_error = NULL, version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, input.ActorUserID, before.ID, before.Version)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return dashboard.SaaSAdminApproval{}, approvalVersionConflict(before.Version)
	}
	eventType := "execution_started"
	if recovered {
		eventType = "execution_recovered"
	}
	if err := insertSaaSAdminApprovalEventTx(ctx, tx, before.ID, eventType, before.Status, dashboard.SaaSAdminApprovalStatusExecuting, input.ActorUserID, input.ActorTenantID, "执行已批准操作", map[string]any{"recovered": recovered}); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	after, _, err := saasAdminApprovalByIDTx(ctx, tx, before.ID, false)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if _, err := insertSaaSAdminApprovalAuditTx(ctx, tx, dashboard.SaaSAdminOperationActionApprovalExecuteStart, input.ActorUserID, input.ActorTenantID, after, before, "start approved action"); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	return after, nil
}

func (s *MySQLStore) FinishSaaSAdminApprovalExecution(ctx context.Context, input dashboard.SaaSAdminApprovalExecutionFinish) (dashboard.SaaSAdminApproval, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	defer rollbackQuietly(tx)
	before, found, err := saasAdminApprovalByIDTx(ctx, tx, input.ApprovalID, true)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if !found {
		return dashboard.SaaSAdminApproval{}, dashboard.NewSaaSAdminNotFound("审批不存在")
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSAdminApproval{}, approvalVersionConflict(before.Version)
	}
	if before.Status != dashboard.SaaSAdminApprovalStatusExecuting || before.ExecutionUserID != input.ActorUserID {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批执行租约不属于当前用户"}
	}
	if input.Success && before.EffectAppliedAtValue.IsZero() {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "业务副作用提交标记缺失，审批不能完成"}
	}
	if !input.Success && !before.EffectAppliedAtValue.IsZero() {
		return dashboard.SaaSAdminApproval{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "业务副作用已提交，审批只能完成为已执行"}
	}
	nextStatus := dashboard.SaaSAdminApprovalStatusApproved
	eventType := "execution_failed"
	resultJSON := any(nil)
	lastError := truncateRunes(input.ErrorMessage, 2000)
	executedAt := any(nil)
	if input.Success {
		nextStatus = dashboard.SaaSAdminApprovalStatusExecuted
		eventType = "execution_succeeded"
		resultJSON = nullableJSONText(input.ResultJSON)
		lastError = ""
		executedAt = time.Now()
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_approvals
		SET status = ?, result_json = ?, last_error = ?, executed_at = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, nextStatus, resultJSON, nullableTrimmedString(lastError), executedAt, before.ID, before.Version)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return dashboard.SaaSAdminApproval{}, approvalVersionConflict(before.Version)
	}
	contextPayload := map[string]any{"success": input.Success}
	if !input.Success {
		contextPayload["error"] = lastError
	}
	if err := insertSaaSAdminApprovalEventTx(ctx, tx, before.ID, eventType, before.Status, nextStatus, input.ActorUserID, input.ActorTenantID, lastError, contextPayload); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	after, _, err := saasAdminApprovalByIDTx(ctx, tx, before.ID, false)
	if err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	remark := "approved action failed"
	if input.Success {
		remark = "approved action executed"
	}
	if _, err := insertSaaSAdminApprovalAuditTx(ctx, tx, dashboard.SaaSAdminOperationActionApprovalExecuteFinish, input.ActorUserID, input.ActorTenantID, after, before, remark); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminApproval{}, err
	}
	return after, nil
}

const saasAdminApprovalSelect = `
	SELECT a.id, a.request_no, a.action_type, a.risk_level, a.status, a.required_permission,
		a.policy_version, a.required_approvals, a.approval_count, a.reminder_minutes,
		a.target_type, a.target_id, a.target_name,
		a.requester_user_id, a.requester_tenant_id, COALESCE(requester.name, ''),
		a.idempotency_key, a.request_sha256, COALESCE(CAST(a.request_json AS CHAR), ''), a.reason,
		a.reviewer_user_id, COALESCE(reviewer.name, ''), a.reviewed_at, a.decision_reason,
		a.sla_due_at, a.next_reminder_at, a.last_reminded_at, a.reminder_count,
			a.execution_user_id, COALESCE(executor.name, ''), a.execution_started_at,
			a.effect_applied_at, a.effect_operation_id, a.executed_at,
		a.execution_attempts, COALESCE(CAST(a.result_json AS CHAR), ''), COALESCE(a.last_error, ''),
		a.expires_at, a.version, a.created_at, a.updated_at
	FROM mochat_go_saas_admin_approvals a
	LEFT JOIN mc_user requester ON requester.id = a.requester_user_id
	LEFT JOIN mc_user reviewer ON reviewer.id = a.reviewer_user_id
	LEFT JOIN mc_user executor ON executor.id = a.execution_user_id
`

func saasAdminApprovalWhere(options dashboard.SaaSAdminApprovalOptions) (string, []any) {
	clauses := []string{"1 = 1"}
	args := make([]any, 0)
	if options.ApprovalID > 0 {
		clauses = append(clauses, "a.id = ?")
		args = append(args, options.ApprovalID)
	}
	if options.Status != "" && options.Status != dashboard.SaaSAdminApprovalStatusAll {
		clauses = append(clauses, "a.status = ?")
		args = append(args, options.Status)
	}
	if options.ActionType != "" {
		clauses = append(clauses, "a.action_type = ?")
		args = append(args, options.ActionType)
	}
	if options.RiskLevel != "" && options.RiskLevel != dashboard.SaaSAdminApprovalRiskAll {
		clauses = append(clauses, "a.risk_level = ?")
		args = append(args, options.RiskLevel)
	}
	if options.RequesterID > 0 {
		clauses = append(clauses, "a.requester_user_id = ?")
		args = append(args, options.RequesterID)
	}
	if options.ReviewerID > 0 {
		clauses = append(clauses, "a.reviewer_user_id = ?")
		args = append(args, options.ReviewerID)
	}
	if options.ExecutionID > 0 {
		clauses = append(clauses, "a.execution_user_id = ?")
		args = append(args, options.ExecutionID)
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		clauses = append(clauses, "(a.request_no LIKE ? OR a.action_type LIKE ? OR a.target_id LIKE ? OR a.target_name LIKE ? OR a.reason LIKE ? OR requester.name LIKE ?)")
		args = append(args, like, like, like, like, like, like)
	}
	return strings.Join(clauses, " AND "), args
}

func scanSaaSAdminApproval(scanner sqlScanner) (dashboard.SaaSAdminApproval, error) {
	var item dashboard.SaaSAdminApproval
	var reviewedAt, slaDueAt, nextReminderAt, lastRemindedAt, executionStartedAt, effectAppliedAt, executedAt, expiresAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.RequestNo, &item.ActionType, &item.RiskLevel, &item.Status, &item.RequiredPermission,
		&item.PolicyVersion, &item.RequiredApprovals, &item.ApprovalCount, &item.ReminderMinutes,
		&item.TargetType, &item.TargetID, &item.TargetName,
		&item.RequesterUserID, &item.RequesterTenantID, &item.RequesterName,
		&item.IdempotencyKey, &item.RequestSHA256, &item.RequestJSON, &item.Reason,
		&item.ReviewerUserID, &item.ReviewerName, &reviewedAt, &item.DecisionReason,
		&slaDueAt, &nextReminderAt, &lastRemindedAt, &item.ReminderCount,
		&item.ExecutionUserID, &item.ExecutionUserName, &executionStartedAt,
		&effectAppliedAt, &item.EffectOperationID, &executedAt,
		&item.ExecutionAttempts, &item.ResultJSON, &item.LastError,
		&expiresAt, &item.Version, &createdAt, &updatedAt,
	)
	item.ReviewedAt = formatTime(reviewedAt)
	item.SLADueAt = formatTime(slaDueAt)
	item.NextReminderAt = formatTime(nextReminderAt)
	item.LastRemindedAt = formatTime(lastRemindedAt)
	item.ExecutionStartedAt = formatTime(executionStartedAt)
	item.EffectAppliedAt = formatTime(effectAppliedAt)
	item.ExecutedAt = formatTime(executedAt)
	item.ExpiresAt = formatTime(expiresAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	if executionStartedAt.Valid {
		item.ExecutionStartedValue = executionStartedAt.Time
	}
	if slaDueAt.Valid {
		item.SLADueAtValue = slaDueAt.Time
	}
	if nextReminderAt.Valid {
		item.NextReminderAtValue = nextReminderAt.Time
	}
	if effectAppliedAt.Valid {
		item.EffectAppliedAtValue = effectAppliedAt.Time
	}
	if expiresAt.Valid {
		item.ExpiresAtValue = expiresAt.Time
	}
	return item, err
}

func saasAdminApprovalByIDTx(ctx context.Context, tx *sql.Tx, approvalID int64, forUpdate bool) (dashboard.SaaSAdminApproval, bool, error) {
	query := saasAdminApprovalSelect + " WHERE a.id = ?"
	if forUpdate {
		query += " FOR UPDATE"
	}
	item, err := scanSaaSAdminApproval(tx.QueryRowContext(ctx, query, approvalID))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminApproval{}, false, nil
	}
	return item, err == nil, err
}

func saasAdminApprovalByIdempotencyTx(ctx context.Context, tx *sql.Tx, requesterID int, key string, forUpdate bool) (dashboard.SaaSAdminApproval, bool, error) {
	query := saasAdminApprovalSelect + " WHERE a.requester_user_id = ? AND a.idempotency_key = ?"
	if forUpdate {
		query += " FOR UPDATE"
	}
	item, err := scanSaaSAdminApproval(tx.QueryRowContext(ctx, query, requesterID, key))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminApproval{}, false, nil
	}
	return item, err == nil, err
}

func (s *MySQLStore) expireSaaSAdminApprovals(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, status FROM mochat_go_saas_admin_approvals
			WHERE status IN ('pending', 'approved') AND effect_applied_at IS NULL AND expires_at <= NOW()
		ORDER BY id ASC LIMIT 1000 FOR UPDATE
	`)
	if err != nil {
		return err
	}
	type expiredRow struct {
		id     int64
		status string
	}
	items := make([]expiredRow, 0)
	for rows.Next() {
		var item expiredRow
		if err := rows.Scan(&item.id, &item.status); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_admin_approvals SET status = 'expired', next_reminder_at = NULL, version = version + 1, updated_at = NOW() WHERE id = ?`, item.id); err != nil {
			return err
		}
		if err := insertSaaSAdminApprovalEventTx(ctx, tx, item.id, "expired", item.status, dashboard.SaaSAdminApprovalStatusExpired, 0, 0, "审批有效期已结束", nil); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func expireSaaSAdminApprovalLocked(ctx context.Context, tx *sql.Tx, item *dashboard.SaaSAdminApproval) (bool, error) {
	if item == nil || item.ExpiresAtValue.IsZero() || time.Now().Before(item.ExpiresAtValue) ||
		!item.EffectAppliedAtValue.IsZero() ||
		(item.Status != dashboard.SaaSAdminApprovalStatusPending && item.Status != dashboard.SaaSAdminApprovalStatusApproved) {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_admin_approvals SET status = 'expired', next_reminder_at = NULL, version = version + 1, updated_at = NOW() WHERE id = ? AND version = ?`, item.ID, item.Version); err != nil {
		return false, err
	}
	if err := insertSaaSAdminApprovalEventTx(ctx, tx, item.ID, "expired", item.Status, dashboard.SaaSAdminApprovalStatusExpired, 0, 0, "审批有效期已结束", nil); err != nil {
		return false, err
	}
	item.Status = dashboard.SaaSAdminApprovalStatusExpired
	item.Version++
	return true, nil
}

func insertSaaSAdminApprovalEventTx(ctx context.Context, tx *sql.Tx, approvalID int64, eventType, fromStatus, toStatus string, actorUserID, actorTenantID int, reason string, contextPayload any) error {
	contextJSON := any(nil)
	if contextPayload != nil {
		raw, err := json.Marshal(contextPayload)
		if err != nil {
			return err
		}
		contextJSON = string(raw)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_approval_events
			(approval_id, event_type, from_status, to_status, actor_user_id, actor_tenant_id, reason, context_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW())
	`, approvalID, eventType, fromStatus, toStatus, actorUserID, actorTenantID, truncateRunes(reason, 255), contextJSON)
	return err
}

func markSaaSAdminApprovalEffectTx(ctx context.Context, tx *sql.Tx, approvalID int64, approvalVersion int, actorUserID int, operationID int64) error {
	if approvalID <= 0 {
		return nil
	}
	if approvalVersion <= 0 || actorUserID <= 0 {
		return dashboard.NewSaaSAdminBadRequest("审批执行引用无效")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_approvals
		SET effect_applied_at = NOW(), effect_operation_id = ?, updated_at = NOW()
		WHERE id = ? AND status = 'executing' AND execution_user_id = ? AND version = ? AND effect_applied_at IS NULL
	`, operationID, approvalID, actorUserID, approvalVersion)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return &dashboard.SaaSAdminOperationError{Status: 409, Message: "审批执行租约已变化，业务事务未提交"}
	}
	return nil
}

func insertSaaSAdminApprovalAuditTx(ctx context.Context, tx *sql.Tx, action string, actorUserID, actorTenantID int, after, before dashboard.SaaSAdminApproval, remark string) (int64, error) {
	return insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: actorUserID, ActorTenantID: actorTenantID,
		Action: action, TargetType: dashboard.SaaSAdminOperationTargetApproval,
		TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.RequestNo,
		BeforeJSON: saasAdminMarshalJSON(saasAdminApprovalAuditPayload(before)), AfterJSON: saasAdminMarshalJSON(saasAdminApprovalAuditPayload(after)),
		Remark: truncateRunes(remark, 255),
	})
}

func saasAdminApprovalAuditPayload(item dashboard.SaaSAdminApproval) map[string]any {
	if item.ID == 0 {
		return nil
	}
	return map[string]any{
		"id": item.ID, "requestNo": item.RequestNo, "actionType": item.ActionType, "riskLevel": item.RiskLevel,
		"status": item.Status, "targetType": item.TargetType, "targetId": item.TargetID,
		"requesterUserId": item.RequesterUserID, "reviewerUserId": item.ReviewerUserID,
		"policyVersion": item.PolicyVersion, "requiredApprovals": item.RequiredApprovals, "approvalCount": item.ApprovalCount,
		"executionUserId": item.ExecutionUserID, "executionAttempts": item.ExecutionAttempts,
		"effectAppliedAt": item.EffectAppliedAt, "effectOperationId": item.EffectOperationID,
		"slaDueAt": item.SLADueAt, "reminderCount": item.ReminderCount, "expiresAt": item.ExpiresAt, "version": item.Version,
	}
}

func approvalVersionConflict(current int) error {
	return &dashboard.SaaSAdminOperationError{Status: 409, Message: fmt.Sprintf("审批版本已变化，请刷新后重试: current=%d", current)}
}

func nullableJSONText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

var _ dashboard.SaaSAdminApprovalStore = (*MySQLStore)(nil)
