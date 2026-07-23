package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

const saasSubscriptionQueryMaxLimit = 5000

type saasSubscriptionScanner interface {
	Scan(dest ...any) error
}

func (s *MySQLStore) saasTenantSubscriptionAccess(ctx context.Context, tenantID int) (managed bool, status string, allowed bool, graceEndsAt string, err error) {
	if tenantID <= 0 {
		return false, "", true, "", nil
	}
	var item dashboard.SaaSAdminSubscription
	var cancelAtPeriodEnd int
	var now time.Time
	err = s.db.QueryRowContext(ctx, `
		SELECT s.status, COALESCE(t.status, 1),
			COALESCE(DATE_FORMAT(s.trial_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(s.current_period_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(s.grace_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			s.cancel_at_period_end, NOW()
		FROM mochat_go_saas_subscriptions s
		LEFT JOIN mc_tenant t ON t.id = s.tenant_id AND t.deleted_at IS NULL
		WHERE s.tenant_id = ? AND s.deleted_at IS NULL
		LIMIT 1
	`, tenantID).Scan(
		&item.Status, &item.TenantStatus, &item.TrialEndsAt, &item.CurrentPeriodEndsAt,
		&item.GraceEndsAt, &cancelAtPeriodEnd, &now,
	)
	if errors.Is(err, sql.ErrNoRows) || isMissingSaaSTableError(err) {
		return false, "", true, "", nil
	}
	if err != nil {
		return false, "", false, "", err
	}
	item.CancelAtPeriodEnd = cancelAtPeriodEnd == 1
	status = dashboard.SaaSAdminEffectiveSubscriptionStatus(item, now)
	return true, status, dashboard.SaaSAdminSubscriptionAllowsAccess(status), item.GraceEndsAt, nil
}

func (s *MySQLStore) SaaSAdminSubscriptions(ctx context.Context, options dashboard.SaaSAdminSubscriptionOptions) (dashboard.SaaSAdminSubscriptionReport, error) {
	if options.Limit <= 0 {
		options.Limit = 100
	}
	if options.Limit > saasSubscriptionQueryMaxLimit {
		options.Limit = saasSubscriptionQueryMaxLimit
	}
	where := []string{"s.deleted_at IS NULL"}
	args := []any{}
	if options.TenantID > 0 {
		where = append(where, "s.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if strings.TrimSpace(options.PackageCode) != "" {
		where = append(where, "s.package_code = ?")
		args = append(args, strings.TrimSpace(options.PackageCode))
	}
	if strings.TrimSpace(options.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(options.Keyword) + "%"
		where = append(where, "(CAST(s.tenant_id AS CHAR) LIKE ? OR COALESCE(t.name, '') LIKE ? OR s.package_code LIKE ? OR s.package_name LIKE ? OR s.state_reason LIKE ?)")
		args = append(args, keyword, keyword, keyword, keyword, keyword)
	}
	args = append(args, saasSubscriptionQueryMaxLimit)
	rows, err := s.db.QueryContext(ctx, saasAdminSubscriptionSelectSQL()+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY
			CASE s.status
				WHEN 'past_due' THEN 1 WHEN 'grace' THEN 2 WHEN 'suspended' THEN 3
				WHEN 'trialing' THEN 4 WHEN 'active' THEN 5 WHEN 'canceled' THEN 6 ELSE 7 END,
			COALESCE(s.grace_ends_at, s.current_period_ends_at, s.trial_ends_at) ASC,
			s.tenant_id ASC
		LIMIT ?
	`, args...)
	if err != nil {
		return dashboard.SaaSAdminSubscriptionReport{}, err
	}
	defer rows.Close()

	now := time.Now()
	all := make([]dashboard.SaaSAdminSubscription, 0)
	for rows.Next() {
		item, err := scanSaaSAdminSubscription(rows)
		if err != nil {
			return dashboard.SaaSAdminSubscriptionReport{}, err
		}
		enrichSaaSAdminSubscription(&item, now)
		if options.Status != "" && options.Status != dashboard.SaaSAdminSubscriptionStatusAll && item.EffectiveStatus != options.Status {
			continue
		}
		if options.Access == dashboard.SaaSAdminSubscriptionAccessAllowed && !item.AccessAllowed {
			continue
		}
		if options.Access == dashboard.SaaSAdminSubscriptionAccessBlocked && item.AccessAllowed {
			continue
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminSubscriptionReport{}, err
	}
	report := dashboard.SaaSAdminSubscriptionReport{Options: options}
	report.Summary = summarizeSaaSAdminSubscriptions(all)
	if len(all) > options.Limit {
		all = all[:options.Limit]
	}
	report.Subscriptions = all
	return report, nil
}

func (s *MySQLStore) SaaSAdminSubscriptionEvents(ctx context.Context, options dashboard.SaaSAdminSubscriptionEventOptions) ([]dashboard.SaaSAdminSubscriptionEvent, error) {
	if options.Limit <= 0 {
		options.Limit = 50
	}
	if options.Limit > saasSubscriptionQueryMaxLimit {
		options.Limit = saasSubscriptionQueryMaxLimit
	}
	where := []string{"1 = 1"}
	args := []any{}
	if options.TenantID > 0 {
		where = append(where, "e.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if strings.TrimSpace(options.Status) != "" {
		where = append(where, "e.to_status = ?")
		args = append(args, strings.TrimSpace(options.Status))
	}
	if strings.TrimSpace(options.Source) != "" {
		where = append(where, "e.source = ?")
		args = append(args, strings.TrimSpace(options.Source))
	}
	if strings.TrimSpace(options.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(options.Keyword) + "%"
		where = append(where, "(CAST(e.tenant_id AS CHAR) LIKE ? OR COALESCE(t.name, '') LIKE ? OR e.event_type LIKE ? OR e.reason LIKE ? OR COALESCE(e.idempotency_key, '') LIKE ?)")
		args = append(args, keyword, keyword, keyword, keyword, keyword)
	}
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			e.id, e.subscription_id, e.tenant_id, COALESCE(t.name, ''), e.event_type,
			e.from_status, e.to_status,
			COALESCE(DATE_FORMAT(e.effective_at, '%Y-%m-%d %H:%i:%s'), ''),
			e.actor_user_id, e.actor_tenant_id, e.source, COALESCE(e.idempotency_key, ''),
			e.reason, COALESCE(CAST(e.payload_json AS CHAR), ''),
			COALESCE(DATE_FORMAT(e.created_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_subscription_events e
		LEFT JOIN mc_tenant t ON t.id = e.tenant_id AND t.deleted_at IS NULL
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY e.id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSAdminSubscriptionEvent, 0)
	for rows.Next() {
		var item dashboard.SaaSAdminSubscriptionEvent
		if err := rows.Scan(
			&item.ID, &item.SubscriptionID, &item.TenantID, &item.TenantName, &item.EventType,
			&item.FromStatus, &item.ToStatus, &item.EffectiveAt, &item.ActorUserID,
			&item.ActorTenantID, &item.Source, &item.IdempotencyKey, &item.Reason,
			&item.PayloadJSON, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) TransitionSaaSAdminSubscription(ctx context.Context, transition dashboard.SaaSAdminSubscriptionTransition) (dashboard.SaaSAdminSubscriptionTransitionResult, error) {
	if transition.TenantID <= 0 {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, dashboard.NewSaaSAdminBadRequest("tenantId required")
	}
	transition.Status = strings.ToLower(strings.TrimSpace(transition.Status))
	if !dashboard.SaaSAdminSubscriptionStatusValid(transition.Status) {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, dashboard.NewSaaSAdminBadRequest("subscription status invalid")
	}
	approvedExecution := transition.ApprovalExecutionID > 0
	if approvedExecution != (transition.ApprovalExecutionVersion > 0) {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, dashboard.NewSaaSAdminBadRequest("审批执行引用无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
	}
	defer rollbackQuietly(tx)

	now, err := saasSubscriptionDBNowTx(ctx, tx)
	if err != nil {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
	}
	current, err := ensureSaaSAdminSubscriptionTx(ctx, tx, transition.TenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, dashboard.NewSaaSAdminNotFound("tenant subscription not found")
	}
	if err != nil {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
	}
	enrichSaaSAdminSubscription(&current, now)

	if approvedExecution {
		if transition.ExpectedSubscriptionID <= 0 || transition.ExpectedVersion <= 0 || transition.ExpectedTenantStatus <= 0 {
			return dashboard.SaaSAdminSubscriptionTransitionResult{}, dashboard.NewSaaSAdminBadRequest("订阅审批冻结快照无效")
		}
		if current.ID != transition.ExpectedSubscriptionID {
			return dashboard.SaaSAdminSubscriptionTransitionResult{}, &dashboard.SaaSAdminOperationError{
				Status: 409, Message: "租户订阅记录已变化，请刷新后重新申请审批",
			}
		}
		if current.TenantStatus != transition.ExpectedTenantStatus {
			return dashboard.SaaSAdminSubscriptionTransitionResult{}, &dashboard.SaaSAdminOperationError{
				Status: 409, Message: "租户状态已变化，请刷新后重新申请审批",
			}
		}
		if current.Version != transition.ExpectedVersion {
			return dashboard.SaaSAdminSubscriptionTransitionResult{}, &dashboard.SaaSAdminOperationError{
				Status: 409, Message: "租户订阅版本已变化，请刷新后重新申请审批",
			}
		}
	}
	if transition.IdempotencyKey != "" {
		var eventID int64
		err := tx.QueryRowContext(ctx, `
			SELECT id
			FROM mochat_go_saas_subscription_events
			WHERE tenant_id = ? AND idempotency_key = ?
			LIMIT 1
		`, transition.TenantID, transition.IdempotencyKey).Scan(&eventID)
		if err == nil {
			if err := tx.Commit(); err != nil {
				return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
			}
			return dashboard.SaaSAdminSubscriptionTransitionResult{
				Subscription: current, PreviousStatus: current.Status, Idempotent: true, EventID: eventID,
			}, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
		}
	}
	if !approvedExecution && transition.ExpectedVersion > 0 && current.Version != transition.ExpectedVersion {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, dashboard.NewSaaSAdminBadRequest(fmt.Sprintf("subscription version conflict: current=%d", current.Version))
	}
	if !dashboard.SaaSAdminSubscriptionTransitionAllowed(current.Status, transition.Status) {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, dashboard.NewSaaSAdminBadRequest("subscription transition not allowed: " + current.Status + " -> " + transition.Status)
	}

	next := current
	if err := applySaaSAdminSubscriptionTransitionTx(ctx, tx, &next, transition, now); err != nil {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
	}
	if dashboard.SaaSAdminSubscriptionStateEqual(current, next) {
		if approvedExecution {
			return dashboard.SaaSAdminSubscriptionTransitionResult{}, &dashboard.SaaSAdminOperationError{
				Status: 409, Message: "目标订阅状态与当前状态一致，审批事务未提交",
			}
		}
		if err := tx.Commit(); err != nil {
			return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
		}
		return dashboard.SaaSAdminSubscriptionTransitionResult{Subscription: current, PreviousStatus: current.Status}, nil
	}

	next.Version = current.Version + 1
	if next.Version <= 0 {
		next.Version = 1
	}
	updateResult, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_subscriptions
		SET package_code = ?, package_name = ?, status = ?, billing_cycle = ?,
			trial_starts_at = ?, trial_ends_at = ?, current_period_starts_at = ?, current_period_ends_at = ?,
			grace_ends_at = ?, cancel_at_period_end = ?, canceled_at = ?, suspended_at = ?,
			version = ?, state_reason = ?, updated_at = NOW(), deleted_at = NULL
		WHERE id = ? AND tenant_id = ? AND version = ? AND deleted_at IS NULL
	`, next.PackageCode, next.PackageName, next.Status, next.BillingCycle,
		saasSubscriptionNullableTime(next.TrialStartsAt), saasSubscriptionNullableTime(next.TrialEndsAt),
		saasSubscriptionNullableTime(next.CurrentPeriodStartsAt), saasSubscriptionNullableTime(next.CurrentPeriodEndsAt),
		saasSubscriptionNullableTime(next.GraceEndsAt), boolToInt(next.CancelAtPeriodEnd),
		saasSubscriptionNullableTime(next.CanceledAt), saasSubscriptionNullableTime(next.SuspendedAt),
		next.Version, next.StateReason, next.ID, next.TenantID, current.Version,
	)
	if err != nil {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
	}
	if affected, _ := updateResult.RowsAffected(); affected != 1 {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, &dashboard.SaaSAdminOperationError{
			Status: 409, Message: "租户订阅版本已变化，审批事务未提交",
		}
	}
	eventType := "transition"
	if transition.Source == "reconcile" {
		eventType = "reconciled"
	}
	payload := saasAdminMarshalJSON(map[string]any{
		"before": saasSubscriptionStatePayload(current),
		"after":  saasSubscriptionStatePayload(next),
	})
	eventID, err := insertSaaSSubscriptionEventTx(ctx, tx, dashboard.SaaSAdminSubscriptionEvent{
		SubscriptionID: next.ID, TenantID: next.TenantID, EventType: eventType,
		FromStatus: current.Status, ToStatus: next.Status, EffectiveAt: now.Format("2006-01-02 15:04:05"),
		ActorUserID: transition.ActorUserID, ActorTenantID: transition.ActorTenantID,
		Source: saasSubscriptionSource(transition.Source), IdempotencyKey: transition.IdempotencyKey,
		Reason: next.StateReason, PayloadJSON: payload,
	})
	if err != nil {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
	}
	action := dashboard.SaaSAdminOperationActionSubscriptionTransition
	if transition.Source == "reconcile" {
		action = dashboard.SaaSAdminOperationActionSubscriptionReconcile
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: next.TenantID, ActorUserID: transition.ActorUserID, ActorTenantID: transition.ActorTenantID,
		Action: action, TargetType: dashboard.SaaSAdminOperationTargetSubscription,
		TargetID: strconv.FormatInt(next.ID, 10), TargetName: next.TenantName,
		BeforeJSON: saasAdminMarshalJSON(saasSubscriptionStatePayload(current)),
		AfterJSON:  saasAdminMarshalJSON(saasSubscriptionStatePayload(next)), Remark: next.StateReason,
	})
	if err != nil && !isMissingSaaSTableError(err) {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(
		ctx, tx, transition.ApprovalExecutionID, transition.ApprovalExecutionVersion, transition.ActorUserID, operationID,
	); err != nil {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminSubscriptionTransitionResult{}, err
	}
	next.UpdatedAt = now.Format("2006-01-02 15:04:05")
	enrichSaaSAdminSubscription(&next, now)
	return dashboard.SaaSAdminSubscriptionTransitionResult{
		Subscription: next, PreviousStatus: current.Status, Changed: true,
		EventID: eventID, OperationID: operationID,
	}, nil
}

func (s *MySQLStore) ReconcileSaaSAdminSubscriptions(ctx context.Context, reconcile dashboard.SaaSAdminSubscriptionReconcile) (dashboard.SaaSAdminSubscriptionReconcileResult, error) {
	if reconcile.Limit <= 0 {
		reconcile.Limit = 500
	}
	if reconcile.Limit > saasSubscriptionQueryMaxLimit {
		reconcile.Limit = saasSubscriptionQueryMaxLimit
	}
	report, err := s.SaaSAdminSubscriptions(ctx, dashboard.SaaSAdminSubscriptionOptions{
		TenantID: reconcile.TenantID, Status: dashboard.SaaSAdminSubscriptionStatusAll,
		Access: dashboard.SaaSAdminSubscriptionAccessAll, Limit: reconcile.Limit,
	})
	if err != nil {
		return dashboard.SaaSAdminSubscriptionReconcileResult{}, err
	}
	result := dashboard.SaaSAdminSubscriptionReconcileResult{DryRun: reconcile.DryRun}
	for _, item := range report.Subscriptions {
		result.ScannedCount++
		if item.TenantID == reconcile.ExcludedTenantID || !item.NeedsReconciliation {
			result.SkippedCount++
			continue
		}
		result.ReconciliationDue++
		if reconcile.DryRun {
			result.Transitions = append(result.Transitions, dashboard.SaaSAdminSubscriptionTransitionResult{
				Subscription: item, PreviousStatus: item.Status,
			})
			continue
		}
		transition, err := s.TransitionSaaSAdminSubscription(ctx, dashboard.SaaSAdminSubscriptionTransition{
			TenantID: item.TenantID, Status: item.EffectiveStatus, ExpectedVersion: item.Version,
			IdempotencyKey: fmt.Sprintf("reconcile:%d:%d:%s", item.TenantID, item.Version, item.EffectiveStatus),
			Reason:         fmt.Sprintf("自动订阅对账：%s -> %s", item.Status, item.EffectiveStatus), Source: "reconcile",
			ActorUserID: reconcile.ActorUserID, ActorTenantID: reconcile.ActorTenantID,
		})
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, dashboard.SaaSAdminSubscriptionReconcileError{TenantID: item.TenantID, Error: err.Error()})
			continue
		}
		if transition.Changed {
			result.ChangedCount++
		} else {
			result.SkippedCount++
		}
		result.Transitions = append(result.Transitions, transition)
	}
	return result, nil
}

func saasAdminSubscriptionSelectSQL() string {
	return `
		SELECT
			s.id, s.tenant_id, COALESCE(t.name, ''), COALESCE(t.status, 1),
			s.package_code, s.package_name, s.status, s.billing_cycle,
			COALESCE(DATE_FORMAT(s.trial_starts_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(s.trial_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(s.current_period_starts_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(s.current_period_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(s.grace_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			s.cancel_at_period_end,
			COALESCE(DATE_FORMAT(s.canceled_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(s.suspended_at, '%Y-%m-%d %H:%i:%s'), ''),
			s.latest_billing_event_id, s.version, s.state_reason,
			COALESCE(CAST(s.metadata_json AS CHAR), ''),
			COALESCE(DATE_FORMAT(s.created_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(s.updated_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_subscriptions s
		LEFT JOIN mc_tenant t ON t.id = s.tenant_id AND t.deleted_at IS NULL
	`
}

func scanSaaSAdminSubscription(scanner saasSubscriptionScanner) (dashboard.SaaSAdminSubscription, error) {
	var item dashboard.SaaSAdminSubscription
	var cancelAtPeriodEnd int
	err := scanner.Scan(
		&item.ID, &item.TenantID, &item.TenantName, &item.TenantStatus,
		&item.PackageCode, &item.PackageName, &item.Status, &item.BillingCycle,
		&item.TrialStartsAt, &item.TrialEndsAt, &item.CurrentPeriodStartsAt,
		&item.CurrentPeriodEndsAt, &item.GraceEndsAt, &cancelAtPeriodEnd,
		&item.CanceledAt, &item.SuspendedAt, &item.LatestBillingEventID,
		&item.Version, &item.StateReason, &item.MetadataJSON, &item.CreatedAt, &item.UpdatedAt,
	)
	item.CancelAtPeriodEnd = cancelAtPeriodEnd == 1
	return item, err
}

func enrichSaaSAdminSubscription(item *dashboard.SaaSAdminSubscription, now time.Time) {
	if item == nil {
		return
	}
	item.EffectiveStatus = dashboard.SaaSAdminEffectiveSubscriptionStatus(*item, now)
	item.AccessAllowed = dashboard.SaaSAdminSubscriptionAllowsAccess(item.EffectiveStatus)
	item.AccessReason = dashboard.SaaSAdminSubscriptionAccessReason(item.EffectiveStatus)
	item.NeedsReconciliation = item.Status != item.EffectiveStatus
	item.NextActionAt = saasSubscriptionNextActionAt(*item)
}

func summarizeSaaSAdminSubscriptions(items []dashboard.SaaSAdminSubscription) dashboard.SaaSAdminSubscriptionSummary {
	var summary dashboard.SaaSAdminSubscriptionSummary
	tenants := map[int]struct{}{}
	packages := map[string]struct{}{}
	for _, item := range items {
		summary.SubscriptionCount++
		tenants[item.TenantID] = struct{}{}
		if item.PackageCode != "" {
			packages[item.PackageCode] = struct{}{}
		}
		switch item.EffectiveStatus {
		case dashboard.SaaSAdminSubscriptionStatusTrialing:
			summary.TrialingCount++
		case dashboard.SaaSAdminSubscriptionStatusActive:
			summary.ActiveCount++
		case dashboard.SaaSAdminSubscriptionStatusGrace:
			summary.GraceCount++
		case dashboard.SaaSAdminSubscriptionStatusPastDue:
			summary.PastDueCount++
		case dashboard.SaaSAdminSubscriptionStatusSuspended:
			summary.SuspendedCount++
		case dashboard.SaaSAdminSubscriptionStatusCanceled:
			summary.CanceledCount++
		}
		if item.AccessAllowed {
			summary.AccessAllowedCount++
		} else {
			summary.AccessBlockedCount++
		}
		if item.NeedsReconciliation {
			summary.ReconciliationDueCount++
		}
	}
	summary.TenantCount = len(tenants)
	summary.PackageCount = len(packages)
	return summary
}

func ensureSaaSAdminSubscriptionTx(ctx context.Context, tx *sql.Tx, tenantID int) (dashboard.SaaSAdminSubscription, error) {
	item, err := saasAdminSubscriptionByTenantTx(ctx, tx, tenantID)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return item, err
	}
	var tenantName string
	var tenantStatus int
	var packageCode string
	var packageName string
	var startsAt string
	var expiresAt string
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(t.name, ''), COALESCE(t.status, 1), p.package_code, p.package_name,
			COALESCE(DATE_FORMAT(p.starts_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(p.expires_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mc_tenant t
		JOIN mochat_go_saas_tenant_packages p ON p.tenant_id = t.id AND p.deleted_at IS NULL
		WHERE t.id = ? AND t.deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, tenantID).Scan(&tenantName, &tenantStatus, &packageCode, &packageName, &startsAt, &expiresAt)
	if err != nil {
		return dashboard.SaaSAdminSubscription{}, err
	}
	now, err := saasSubscriptionDBNowTx(ctx, tx)
	if err != nil {
		return dashboard.SaaSAdminSubscription{}, err
	}
	item = dashboard.SaaSAdminSubscription{
		TenantID: tenantID, TenantName: tenantName, TenantStatus: tenantStatus,
		PackageCode: packageCode, PackageName: packageName, Status: dashboard.SaaSAdminSubscriptionStatusActive,
		BillingCycle: "custom", CurrentPeriodStartsAt: startsAt, CurrentPeriodEndsAt: expiresAt,
		GraceEndsAt: saasSubscriptionGraceEnd(expiresAt), Version: 1,
		StateReason: "按现有套餐初始化订阅", CreatedAt: now.Format("2006-01-02 15:04:05"), UpdatedAt: now.Format("2006-01-02 15:04:05"),
	}
	if expiresAt == "" {
		item.BillingCycle = "lifetime"
	}
	item.Status = dashboard.SaaSAdminEffectiveSubscriptionStatus(item, now)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_subscriptions
			(tenant_id, package_code, package_name, status, billing_cycle, current_period_starts_at,
			 current_period_ends_at, grace_ends_at, version, state_reason, metadata_json, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, JSON_OBJECT('source', 'lazy_init', 'defaultGraceDays', ?), NOW(), NOW(), NULL)
	`, tenantID, packageCode, packageName, item.Status, item.BillingCycle,
		saasSubscriptionNullableTime(startsAt), saasSubscriptionNullableTime(expiresAt),
		saasSubscriptionNullableTime(item.GraceEndsAt), item.StateReason, dashboard.SaaSAdminSubscriptionDefaultGraceDays)
	if err != nil {
		return dashboard.SaaSAdminSubscription{}, err
	}
	item.ID, _ = result.LastInsertId()
	_, err = insertSaaSSubscriptionEventTx(ctx, tx, dashboard.SaaSAdminSubscriptionEvent{
		SubscriptionID: item.ID, TenantID: tenantID, EventType: "initialized", ToStatus: item.Status,
		EffectiveAt: now.Format("2006-01-02 15:04:05"), Source: "system",
		IdempotencyKey: "lazy-init", Reason: item.StateReason,
		PayloadJSON: saasAdminMarshalJSON(saasSubscriptionStatePayload(item)),
	})
	if err != nil {
		return dashboard.SaaSAdminSubscription{}, err
	}
	return item, nil
}

func saasAdminSubscriptionByTenantTx(ctx context.Context, tx *sql.Tx, tenantID int) (dashboard.SaaSAdminSubscription, error) {
	row := tx.QueryRowContext(ctx, saasAdminSubscriptionSelectSQL()+`
		WHERE s.tenant_id = ? AND s.deleted_at IS NULL
		LIMIT 1
		FOR UPDATE
	`, tenantID)
	return scanSaaSAdminSubscription(row)
}

func applySaaSAdminSubscriptionTransitionTx(ctx context.Context, tx *sql.Tx, item *dashboard.SaaSAdminSubscription, transition dashboard.SaaSAdminSubscriptionTransition, now time.Time) error {
	if item == nil {
		return dashboard.NewSaaSAdminBadRequest("subscription missing")
	}
	if transition.PackageCode != "" {
		var code string
		var name string
		err := tx.QueryRowContext(ctx, `
			SELECT p.package_code, p.package_name
			FROM mochat_go_saas_tenant_packages p
			WHERE p.tenant_id = ? AND p.package_code = ? AND p.status = 1 AND p.deleted_at IS NULL
			LIMIT 1
		`, item.TenantID, transition.PackageCode).Scan(&code, &name)
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.NewSaaSAdminBadRequest("packageCode must match active tenant package")
		}
		if err != nil {
			return err
		}
		item.PackageCode = code
		item.PackageName = name
	}
	return dashboard.ApplySaaSAdminSubscriptionTransitionState(item, transition, now)
}

func insertSaaSSubscriptionEventTx(ctx context.Context, tx *sql.Tx, item dashboard.SaaSAdminSubscriptionEvent) (int64, error) {
	var idempotency any
	if strings.TrimSpace(item.IdempotencyKey) != "" {
		idempotency = strings.TrimSpace(item.IdempotencyKey)
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_subscription_events
			(subscription_id, tenant_id, event_type, from_status, to_status, effective_at,
			 actor_user_id, actor_tenant_id, source, idempotency_key, reason, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
	`, item.SubscriptionID, item.TenantID, item.EventType, item.FromStatus, item.ToStatus,
		saasSubscriptionNullableTime(item.EffectiveAt), item.ActorUserID, item.ActorTenantID,
		saasSubscriptionSource(item.Source), idempotency, truncateRunes(strings.TrimSpace(item.Reason), 255),
		saasAdminJSONValue(item.PayloadJSON))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func saasSubscriptionDBNowTx(ctx context.Context, tx *sql.Tx) (time.Time, error) {
	var now time.Time
	err := tx.QueryRowContext(ctx, "SELECT NOW()").Scan(&now)
	return now, err
}

func saasSubscriptionNullableTime(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func parseSaaSSubscriptionTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", time.RFC3339} {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func saasSubscriptionGraceEnd(expiresAt string) string {
	expires, ok := parseSaaSSubscriptionTime(expiresAt)
	if !ok {
		return ""
	}
	return expires.AddDate(0, 0, dashboard.SaaSAdminSubscriptionDefaultGraceDays).Format("2006-01-02 15:04:05")
}

func saasSubscriptionNextActionAt(item dashboard.SaaSAdminSubscription) string {
	if item.CancelAtPeriodEnd && item.CurrentPeriodEndsAt != "" {
		return item.CurrentPeriodEndsAt
	}
	switch item.EffectiveStatus {
	case dashboard.SaaSAdminSubscriptionStatusTrialing:
		return item.TrialEndsAt
	case dashboard.SaaSAdminSubscriptionStatusActive:
		return item.CurrentPeriodEndsAt
	case dashboard.SaaSAdminSubscriptionStatusGrace:
		return item.GraceEndsAt
	default:
		return ""
	}
}

func saasSubscriptionSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "system"
	}
	return truncateRunes(source, 32)
}

func saasSubscriptionStatePayload(item dashboard.SaaSAdminSubscription) map[string]any {
	return map[string]any{
		"tenantId": item.TenantID, "packageCode": item.PackageCode, "packageName": item.PackageName,
		"status": item.Status, "billingCycle": item.BillingCycle,
		"trialStartsAt": item.TrialStartsAt, "trialEndsAt": item.TrialEndsAt,
		"currentPeriodStartsAt": item.CurrentPeriodStartsAt, "currentPeriodEndsAt": item.CurrentPeriodEndsAt,
		"graceEndsAt": item.GraceEndsAt, "cancelAtPeriodEnd": item.CancelAtPeriodEnd,
		"canceledAt": item.CanceledAt, "suspendedAt": item.SuspendedAt,
		"latestBillingEventId": item.LatestBillingEventID, "version": item.Version,
		"stateReason": item.StateReason,
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func syncSaaSSubscriptionPackageTx(
	ctx context.Context,
	tx *sql.Tx,
	tenantID int,
	packageCode string,
	packageName string,
	expiresAt string,
	billingCycle string,
	latestBillingEventID int64,
	eventType string,
	source string,
	idempotencyKey string,
	activate bool,
	actorUserID int,
	actorTenantID int,
	reason string,
) error {
	current, err := ensureSaaSAdminSubscriptionTx(ctx, tx, tenantID)
	if err != nil {
		if isMissingSaaSTableError(err) {
			return nil
		}
		return err
	}
	now, err := saasSubscriptionDBNowTx(ctx, tx)
	if err != nil {
		return err
	}
	next := current
	next.PackageCode = strings.TrimSpace(packageCode)
	next.PackageName = strings.TrimSpace(packageName)
	next.CurrentPeriodEndsAt = strings.TrimSpace(expiresAt)
	next.GraceEndsAt = saasSubscriptionGraceEnd(expiresAt)
	if normalizedCycle := strings.ToLower(strings.TrimSpace(billingCycle)); normalizedCycle == "monthly" || normalizedCycle == "yearly" || normalizedCycle == "custom" || normalizedCycle == "lifetime" {
		next.BillingCycle = normalizedCycle
	}
	if next.CurrentPeriodStartsAt == "" || activate {
		next.CurrentPeriodStartsAt = now.Format("2006-01-02 15:04:05")
	}
	if next.CurrentPeriodEndsAt == "" {
		next.BillingCycle = "lifetime"
		next.GraceEndsAt = ""
	} else if next.BillingCycle == "" || next.BillingCycle == "lifetime" {
		next.BillingCycle = "custom"
	}
	if latestBillingEventID > 0 {
		next.LatestBillingEventID = latestBillingEventID
	}
	if activate || (next.Status != dashboard.SaaSAdminSubscriptionStatusSuspended && next.Status != dashboard.SaaSAdminSubscriptionStatusCanceled) {
		next.Status = dashboard.SaaSAdminSubscriptionStatusActive
		next.TrialStartsAt = ""
		next.TrialEndsAt = ""
		next.CancelAtPeriodEnd = false
		next.CanceledAt = ""
		next.SuspendedAt = ""
		next.Status = dashboard.SaaSAdminEffectiveSubscriptionStatus(next, now)
		if next.Status == dashboard.SaaSAdminSubscriptionStatusSuspended {
			next.SuspendedAt = now.Format("2006-01-02 15:04:05")
		}
	}
	next.StateReason = strings.TrimSpace(reason)
	if next.StateReason == "" {
		next.StateReason = "同步租户套餐到订阅"
	}
	if dashboard.SaaSAdminSubscriptionStateEqual(current, next) && current.LatestBillingEventID == next.LatestBillingEventID {
		return nil
	}
	next.Version = current.Version + 1
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_subscriptions
		SET package_code = ?, package_name = ?, status = ?, billing_cycle = ?,
			trial_starts_at = ?, trial_ends_at = ?, current_period_starts_at = ?, current_period_ends_at = ?,
			grace_ends_at = ?, cancel_at_period_end = ?, canceled_at = ?, suspended_at = ?,
			latest_billing_event_id = ?, version = ?, state_reason = ?, updated_at = NOW(), deleted_at = NULL
		WHERE id = ? AND tenant_id = ? AND version = ? AND deleted_at IS NULL
	`, next.PackageCode, next.PackageName, next.Status, next.BillingCycle,
		saasSubscriptionNullableTime(next.TrialStartsAt), saasSubscriptionNullableTime(next.TrialEndsAt),
		saasSubscriptionNullableTime(next.CurrentPeriodStartsAt), saasSubscriptionNullableTime(next.CurrentPeriodEndsAt),
		saasSubscriptionNullableTime(next.GraceEndsAt), boolToInt(next.CancelAtPeriodEnd),
		saasSubscriptionNullableTime(next.CanceledAt), saasSubscriptionNullableTime(next.SuspendedAt),
		next.LatestBillingEventID, next.Version, next.StateReason, next.ID, next.TenantID, current.Version,
	); err != nil {
		if isMissingSaaSTableError(err) {
			return nil
		}
		return err
	}
	if eventType == "" {
		eventType = "package_synced"
	}
	_, err = insertSaaSSubscriptionEventTx(ctx, tx, dashboard.SaaSAdminSubscriptionEvent{
		SubscriptionID: next.ID, TenantID: next.TenantID, EventType: eventType,
		FromStatus: current.Status, ToStatus: next.Status, EffectiveAt: now.Format("2006-01-02 15:04:05"),
		ActorUserID: actorUserID, ActorTenantID: actorTenantID, Source: source,
		IdempotencyKey: idempotencyKey, Reason: next.StateReason,
		PayloadJSON: saasAdminMarshalJSON(map[string]any{
			"before": saasSubscriptionStatePayload(current), "after": saasSubscriptionStatePayload(next),
		}),
	})
	if isMissingSaaSTableError(err) {
		return nil
	}
	return err
}

func syncSaaSSubscriptionTenantStatusTx(ctx context.Context, tx *sql.Tx, tenantID int, tenantStatus int, actorUserID int, actorTenantID int, reason string) error {
	current, err := saasAdminSubscriptionByTenantTx(ctx, tx, tenantID)
	if errors.Is(err, sql.ErrNoRows) || isMissingSaaSTableError(err) {
		return nil
	}
	if err != nil {
		return err
	}
	now, err := saasSubscriptionDBNowTx(ctx, tx)
	if err != nil {
		return err
	}
	next := current
	next.TenantStatus = tenantStatus
	if tenantStatus == 2 {
		next.Status = dashboard.SaaSAdminSubscriptionStatusSuspended
		next.SuspendedAt = now.Format("2006-01-02 15:04:05")
		next.CanceledAt = ""
	} else if current.Status == dashboard.SaaSAdminSubscriptionStatusSuspended {
		previousStatus := dashboard.SaaSAdminSubscriptionStatusActive
		var storedPrevious string
		err := tx.QueryRowContext(ctx, `
			SELECT from_status
			FROM mochat_go_saas_subscription_events
			WHERE tenant_id = ? AND to_status = 'suspended' AND source = 'tenant_status'
			ORDER BY id DESC
			LIMIT 1
		`, tenantID).Scan(&storedPrevious)
		if err == nil && dashboard.SaaSAdminSubscriptionStatusValid(storedPrevious) && storedPrevious != dashboard.SaaSAdminSubscriptionStatusSuspended {
			previousStatus = storedPrevious
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		next.Status = previousStatus
		next.SuspendedAt = ""
		next.Status = dashboard.SaaSAdminEffectiveSubscriptionStatus(next, now)
	}
	next.StateReason = strings.TrimSpace(reason)
	if next.StateReason == "" {
		next.StateReason = "同步租户启停状态"
	}
	if dashboard.SaaSAdminSubscriptionStateEqual(current, next) {
		return nil
	}
	next.Version = current.Version + 1
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_subscriptions
		SET status = ?, suspended_at = ?, canceled_at = ?, version = ?, state_reason = ?, updated_at = NOW()
		WHERE id = ? AND tenant_id = ? AND version = ? AND deleted_at IS NULL
	`, next.Status, saasSubscriptionNullableTime(next.SuspendedAt), saasSubscriptionNullableTime(next.CanceledAt),
		next.Version, next.StateReason, next.ID, next.TenantID, current.Version); err != nil {
		return err
	}
	_, err = insertSaaSSubscriptionEventTx(ctx, tx, dashboard.SaaSAdminSubscriptionEvent{
		SubscriptionID: next.ID, TenantID: next.TenantID, EventType: "tenant_status_synced",
		FromStatus: current.Status, ToStatus: next.Status, EffectiveAt: now.Format("2006-01-02 15:04:05"),
		ActorUserID: actorUserID, ActorTenantID: actorTenantID, Source: "tenant_status",
		Reason: next.StateReason, PayloadJSON: saasAdminMarshalJSON(map[string]any{
			"tenantStatus": tenantStatus, "before": saasSubscriptionStatePayload(current), "after": saasSubscriptionStatePayload(next),
		}),
	})
	return err
}
