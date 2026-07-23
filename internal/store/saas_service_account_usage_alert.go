package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

type saasServiceAccountUsageAlertAccount struct {
	ID                      int64
	TenantID                int
	TenantName              string
	TenantStatus            int
	Code                    string
	Name                    string
	Status                  string
	ExpiresAt               sql.NullTime
	DailyRequestLimit       int64
	DailyRequestCount       int64
	DailyRejectedCount      int64
	UsageAlertEnabled       bool
	UsageWarningPercent     int
	RejectionWarningCount   int
	CooldownMinutes         int
	UsageLastNotifiedAt     sql.NullTime
	RejectionLastNotifiedAt sql.NullTime
}

type saasServiceAccountUsageAlertOutcome struct {
	usageWarning        bool
	rejectionWarning    bool
	resolved            int
	notifications       int
	notificationsClosed int
	usageNotified       bool
	rejectionNotified   bool
}

func (s *MySQLStore) EvaluateSaaSServiceAccountUsageAlerts(ctx context.Context, options dashboard.SaaSServiceAccountUsageAlertEvaluateOptions) (dashboard.SaaSServiceAccountUsageAlertEvaluateResult, error) {
	if options.Limit <= 0 || options.Limit > 500 {
		options.Limit = 100
	}
	if options.TenantID < 0 || options.ServiceAccountID < 0 {
		return dashboard.SaaSServiceAccountUsageAlertEvaluateResult{}, dashboard.NewSaaSAdminBadRequest("tenantId 和 serviceAccountId 不能小于 0")
	}
	source := strings.TrimSpace(options.Source)
	if source == "" {
		source = "maintenance"
	}
	now := time.Now()
	result := dashboard.SaaSServiceAccountUsageAlertEvaluateResult{EvaluatedAt: now.Format("2006-01-02 15:04:05")}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer rollbackQuietly(tx)

	where := []string{"t.deleted_at IS NULL"}
	args := make([]any, 0, 3)
	if options.TenantID > 0 {
		where = append(where, "a.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if options.ServiceAccountID > 0 {
		where = append(where, "a.id = ?")
		args = append(args, options.ServiceAccountID)
	}
	args = append(args, options.Limit)
	rows, err := tx.QueryContext(ctx, `
		SELECT a.id
		FROM mochat_go_saas_service_accounts a
		INNER JOIN mc_tenant t ON t.id = a.tenant_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY a.usage_alert_last_evaluated_at IS NULL DESC, a.usage_alert_last_evaluated_at ASC, a.id ASC
		LIMIT ?
	`, args...)
	if err != nil {
		return result, err
	}
	accountIDs := make([]int64, 0, options.Limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return result, err
		}
		accountIDs = append(accountIDs, id)
	}
	if err := rows.Close(); err != nil {
		return result, err
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	for _, accountID := range accountIDs {
		account, err := loadSaaSServiceAccountUsageAlertAccount(ctx, tx, accountID)
		if err != nil {
			return result, err
		}
		result.ScannedAccounts++
		eligible := account.UsageAlertEnabled && account.Status == dashboard.SaaSServiceAccountStatusActive && account.TenantStatus == 1
		if account.ExpiresAt.Valid && !now.Before(account.ExpiresAt.Time) {
			eligible = false
		}
		if !account.UsageAlertEnabled {
			result.DisabledPolicies++
		}
		if eligible {
			result.EligibleAccounts++
		}
		outcome, err := evaluateSaaSServiceAccountUsageAlertAccount(ctx, tx, account, eligible, source, now)
		if err != nil {
			return result, err
		}
		if outcome.usageWarning {
			result.UsageWarningAccounts++
		}
		if outcome.rejectionWarning {
			result.RejectionWarningAccounts++
		}
		result.ResolvedAlerts += outcome.resolved
		result.NotificationsQueued += outcome.notifications
		result.NotificationsClosed += outcome.notificationsClosed
	}

	if options.ActorUserID > 0 {
		afterJSON, _ := json.Marshal(map[string]any{
			"tenantId": options.TenantID, "serviceAccountId": options.ServiceAccountID, "limit": options.Limit,
			"scannedAccounts": result.ScannedAccounts, "eligibleAccounts": result.EligibleAccounts,
			"usageWarningAccounts": result.UsageWarningAccounts, "rejectionWarningAccounts": result.RejectionWarningAccounts,
			"resolvedAlerts": result.ResolvedAlerts, "notificationsQueued": result.NotificationsQueued,
			"notificationsClosed": result.NotificationsClosed,
		})
		targetID := "all"
		if options.ServiceAccountID > 0 {
			targetID = strconv.FormatInt(options.ServiceAccountID, 10)
		} else if options.TenantID > 0 {
			targetID = "tenant:" + strconv.Itoa(options.TenantID)
		}
		operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
			TenantID: options.TenantID, ActorUserID: options.ActorUserID, ActorTenantID: options.ActorTenantID,
			Action:     dashboard.SaaSAdminOperationActionServiceAccountUsageAlertEvaluate,
			TargetType: dashboard.SaaSAdminOperationTargetServiceAccountUsageAlert, TargetID: targetID,
			TargetName: "OpenAPI usage alert evaluation", AfterJSON: string(afterJSON), Remark: "evaluate SaaS service account usage alerts",
		})
		if err != nil {
			return result, err
		}
		result.OperationID = operationID
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func loadSaaSServiceAccountUsageAlertAccount(ctx context.Context, tx *sql.Tx, accountID int64) (saasServiceAccountUsageAlertAccount, error) {
	var account saasServiceAccountUsageAlertAccount
	err := tx.QueryRowContext(ctx, `
		SELECT a.id, a.tenant_id, COALESCE(t.name, ''), t.status, a.code, a.name, a.status, a.expires_at,
			a.daily_request_limit,
			IF(a.daily_window_date = CURDATE(), a.daily_request_count, 0),
			IF(a.daily_window_date = CURDATE(), a.daily_rejected_count, 0),
			a.usage_alert_enabled, a.usage_warning_percent, a.rejection_warning_count, a.usage_alert_cooldown_minutes,
			a.usage_alert_last_notified_at, a.rejection_alert_last_notified_at
		FROM mochat_go_saas_service_accounts a
		INNER JOIN mc_tenant t ON t.id = a.tenant_id AND t.deleted_at IS NULL
		WHERE a.id = ?
		LIMIT 1
		FOR UPDATE
	`, accountID).Scan(
		&account.ID, &account.TenantID, &account.TenantName, &account.TenantStatus, &account.Code, &account.Name, &account.Status, &account.ExpiresAt,
		&account.DailyRequestLimit, &account.DailyRequestCount, &account.DailyRejectedCount,
		&account.UsageAlertEnabled, &account.UsageWarningPercent, &account.RejectionWarningCount, &account.CooldownMinutes,
		&account.UsageLastNotifiedAt, &account.RejectionLastNotifiedAt,
	)
	return account, err
}

func evaluateSaaSServiceAccountUsageAlertAccount(ctx context.Context, tx *sql.Tx, account saasServiceAccountUsageAlertAccount, eligible bool, source string, now time.Time) (saasServiceAccountUsageAlertOutcome, error) {
	outcome := saasServiceAccountUsageAlertOutcome{}
	usageThreshold := int64(0)
	if account.DailyRequestLimit > 0 {
		usageThreshold = (account.DailyRequestLimit*int64(account.UsageWarningPercent) + 99) / 100
		if usageThreshold < 1 {
			usageThreshold = 1
		}
	}
	usageCondition := eligible && usageThreshold > 0 && account.DailyRequestCount >= usageThreshold
	rejectionThreshold := int64(account.RejectionWarningCount)
	rejectionCondition := eligible && rejectionThreshold > 0 && account.DailyRejectedCount >= rejectionThreshold
	periodKey := dashboard.SaaSServiceAccountUsageAlertPeriodKey(account.ID)
	contextPayload := map[string]any{
		"serviceAccountId": account.ID, "serviceAccountCode": account.Code, "serviceAccountName": account.Name,
		"tenantId": account.TenantID, "tenantName": account.TenantName, "usageDate": now.Format("2006-01-02"),
		"requestCount": account.DailyRequestCount, "rejectedCount": account.DailyRejectedCount,
		"dailyRequestLimit": account.DailyRequestLimit, "usageWarningPercent": account.UsageWarningPercent,
		"rejectionWarningCount": account.RejectionWarningCount, "cooldownMinutes": account.CooldownMinutes,
	}

	usageAlert := dashboard.SaaSQuotaAlert{
		Status:    dashboard.SaaSQuotaStatus{TenantID: account.TenantID, Metric: dashboard.SaaSEventMetricServiceAccountUsage, Current: account.DailyRequestCount, Limit: usageThreshold},
		AlertType: dashboard.SaaSAlertTypeServiceAccountUsageWarning, Severity: dashboard.SaaSAlertSeverityWarning,
		PeriodKey: periodKey, Source: source,
		Message: fmt.Sprintf("OpenAPI 服务账号 %s 当日用量达到 %d/%d（%d%% 预警线）", account.Name, account.DailyRequestCount, account.DailyRequestLimit, account.UsageWarningPercent),
		Context: contextPayload,
	}
	if account.DailyRequestLimit > 0 && account.DailyRequestCount >= account.DailyRequestLimit {
		usageAlert.Severity = dashboard.SaaSAlertSeverityCritical
	}
	usageNotify := usageCondition && notificationCooldownElapsed(account.UsageLastNotifiedAt, account.CooldownMinutes, now)
	usageResolved, usageQueued, usageClosed, err := reconcileSaaSServiceAccountUsageAlert(ctx, tx, usageAlert, usageCondition, usageNotify, now)
	if err != nil {
		return outcome, err
	}
	outcome.usageWarning = usageCondition
	outcome.resolved += usageResolved
	outcome.notifications += usageQueued
	outcome.notificationsClosed += usageClosed
	outcome.usageNotified = usageQueued > 0

	rejectionAlert := dashboard.SaaSQuotaAlert{
		Status:    dashboard.SaaSQuotaStatus{TenantID: account.TenantID, Metric: dashboard.SaaSEventMetricServiceAccountUsage, Current: account.DailyRejectedCount, Limit: rejectionThreshold},
		AlertType: dashboard.SaaSAlertTypeServiceAccountRejectionWarning, Severity: dashboard.SaaSAlertSeverityWarning,
		PeriodKey: periodKey, Source: source,
		Message: fmt.Sprintf("OpenAPI 服务账号 %s 当日限流拒绝 %d 次（预警线 %d）", account.Name, account.DailyRejectedCount, rejectionThreshold),
		Context: contextPayload,
	}
	criticalRejections := rejectionThreshold * 5
	if criticalRejections < 10 {
		criticalRejections = 10
	}
	if rejectionThreshold > 0 && account.DailyRejectedCount >= criticalRejections {
		rejectionAlert.Severity = dashboard.SaaSAlertSeverityCritical
	}
	rejectionNotify := rejectionCondition && notificationCooldownElapsed(account.RejectionLastNotifiedAt, account.CooldownMinutes, now)
	rejectionResolved, rejectionQueued, rejectionClosed, err := reconcileSaaSServiceAccountUsageAlert(ctx, tx, rejectionAlert, rejectionCondition, rejectionNotify, now)
	if err != nil {
		return outcome, err
	}
	outcome.rejectionWarning = rejectionCondition
	outcome.resolved += rejectionResolved
	outcome.notifications += rejectionQueued
	outcome.notificationsClosed += rejectionClosed
	outcome.rejectionNotified = rejectionQueued > 0

	_, err = tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_service_accounts
		SET usage_alert_last_evaluated_at = ?,
			usage_alert_last_notified_at = CASE WHEN ? = 1 THEN ? WHEN ? = 0 THEN NULL ELSE usage_alert_last_notified_at END,
			rejection_alert_last_notified_at = CASE WHEN ? = 1 THEN ? WHEN ? = 0 THEN NULL ELSE rejection_alert_last_notified_at END
		WHERE id = ?
	`, now, boolTinyInt(outcome.usageNotified), now, boolTinyInt(usageCondition), boolTinyInt(outcome.rejectionNotified), now, boolTinyInt(rejectionCondition), account.ID)
	return outcome, err
}

func notificationCooldownElapsed(last sql.NullTime, cooldownMinutes int, now time.Time) bool {
	if !last.Valid {
		return true
	}
	if cooldownMinutes < 5 {
		cooldownMinutes = dashboard.DefaultSaaSServiceAccountUsageAlertCooldownMinutes
	}
	return !now.Before(last.Time.Add(time.Duration(cooldownMinutes) * time.Minute))
}

func reconcileSaaSServiceAccountUsageAlert(ctx context.Context, tx *sql.Tx, alert dashboard.SaaSQuotaAlert, condition, notify bool, now time.Time) (int, int, int, error) {
	alertKey := saasAlertKey(alert.Status.TenantID, alert.Status.Metric, alert.AlertType, alert.PeriodKey)
	if !condition {
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_alerts
			SET status = ?, resolved_at = ?, updated_at = ?
			WHERE alert_key = ? AND status = ? AND deleted_at IS NULL
		`, dashboard.SaaSAlertStatusResolved, now, now, alertKey, dashboard.SaaSAlertStatusOpen)
		if err != nil {
			return 0, 0, 0, err
		}
		resolved, _ := result.RowsAffected()
		closedResult, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_alert_notifications
			SET status = ?, last_error = ?, next_retry_at = NULL, updated_at = ?
			WHERE alert_key = ? AND status IN (?, ?) AND deleted_at IS NULL
		`, dashboard.SaaSAlertNotificationStatusClosed, "alert condition recovered before delivery", now, alertKey,
			dashboard.SaaSAlertNotificationStatusPending, dashboard.SaaSAlertNotificationStatusFailed)
		if err != nil {
			return 0, 0, 0, err
		}
		closed, _ := closedResult.RowsAffected()
		return int(resolved), 0, int(closed), nil
	}
	contextJSON, err := json.Marshal(alert.Context)
	if err != nil {
		return 0, 0, 0, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_alerts (
			alert_key, tenant_id, alert_type, severity, status, metric, period_key,
			current_value, limit_value, additional_value, occurrence_count,
			source, message, context_json, first_seen_at, last_seen_at, resolved_at, created_at, updated_at, deleted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1, ?, ?, ?, ?, ?, NULL, ?, ?, NULL)
		ON DUPLICATE KEY UPDATE
			status = VALUES(status), severity = VALUES(severity), current_value = VALUES(current_value), limit_value = VALUES(limit_value),
			additional_value = 0, occurrence_count = occurrence_count + 1, source = VALUES(source), message = VALUES(message),
			context_json = VALUES(context_json), last_seen_at = VALUES(last_seen_at), resolved_at = NULL, updated_at = VALUES(updated_at), deleted_at = NULL
	`, alertKey, alert.Status.TenantID, alert.AlertType, alert.Severity, dashboard.SaaSAlertStatusOpen, alert.Status.Metric, alert.PeriodKey,
		nonNegativeInt64(alert.Status.Current), nonNegativeInt64(alert.Status.Limit), truncateRunes(alert.Source, 64), truncateRunes(alert.Message, 255), string(contextJSON), now, now, now, now)
	if err != nil {
		return 0, 0, 0, err
	}
	if !notify {
		return 0, 0, 0, nil
	}
	var maxAttempts int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(notification_max_attempts), 3)
		FROM mochat_go_saas_alert_settings
		WHERE tenant_id = ? AND channel = ? AND deleted_at IS NULL
	`, alert.Status.TenantID, dashboard.SaaSAlertNotificationChannelWebhook).Scan(&maxAttempts); err != nil {
		return 0, 0, 0, err
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	alertJSON, err := json.Marshal(alert)
	if err != nil {
		return 0, 0, 0, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_alert_notifications (
			notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts,
			alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at
		) VALUES (?, ?, ?, ?, ?, 0, ?, ?, '', ?, NULL, ?, ?, NULL)
		ON DUPLICATE KEY UPDATE alert_key = VALUES(alert_key), tenant_id = VALUES(tenant_id), channel = VALUES(channel),
			status = VALUES(status), attempts = 0, max_attempts = VALUES(max_attempts), alert_json = VALUES(alert_json),
			last_error = '', next_retry_at = VALUES(next_retry_at), delivered_at = NULL, updated_at = VALUES(updated_at), deleted_at = NULL
	`, dashboard.SaaSAlertNotificationKey(alert, dashboard.SaaSAlertNotificationChannelWebhook), alertKey, alert.Status.TenantID,
		dashboard.SaaSAlertNotificationChannelWebhook, dashboard.SaaSAlertNotificationStatusPending, maxAttempts, string(alertJSON), now, now, now)
	if err != nil {
		return 0, 0, 0, err
	}
	return 0, 1, 0, nil
}

var _ dashboard.SaaSServiceAccountUsageAlertEvaluator = (*MySQLStore)(nil)
