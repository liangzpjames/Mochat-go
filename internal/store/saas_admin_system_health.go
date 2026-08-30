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
	"jiyi/mochat-go/internal/migrationhistory"
	"jiyi/mochat-go/internal/saasbackup"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func (s *MySQLStore) SaaSAdminSystemHealthChecks(ctx context.Context, options dashboard.SaaSAdminSystemHealthOptions) ([]dashboard.SaaSAdminSystemHealthCheck, error) {
	checks := make([]dashboard.SaaSAdminSystemHealthCheck, 0, 20)
	checks = append(checks, dashboard.SaaSAdminSystemHealthCheck{
		Code: "database_connection", Name: "MySQL 连接", Category: "runtime", Status: dashboard.SaaSAdminSystemHealthStateHealthy,
		Severity: dashboard.SaaSAdminSystemHealthStateCritical, Detail: "数据库查询正常",
	})

	migrationCheck, err := saasMigrationHealthCheck(ctx, s.db)
	if err != nil {
		return nil, err
	}
	checks = append(checks, migrationCheck)

	appendCountCheck := func(code, name, category, query string, args []any, warningAt, criticalAt int64, detail string) error {
		var count int64
		if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
			return err
		}
		status := dashboard.SaaSAdminSystemHealthStateHealthy
		if criticalAt > 0 && count >= criticalAt {
			status = dashboard.SaaSAdminSystemHealthStateCritical
		} else if warningAt > 0 && count >= warningAt {
			status = dashboard.SaaSAdminSystemHealthStateWarning
		}
		checks = append(checks, dashboard.SaaSAdminSystemHealthCheck{
			Code: code, Name: name, Category: category, Status: status, Severity: dashboard.SaaSAdminSystemHealthStateWarning,
			Current: count, Threshold: warningAt, Detail: fmt.Sprintf(detail, count),
		})
		if status == dashboard.SaaSAdminSystemHealthStateCritical {
			checks[len(checks)-1].Severity = dashboard.SaaSAdminSystemHealthStateCritical
		}
		return nil
	}

	queries := []struct {
		code, name, category, query, detail string
		args                                []any
		warningAt, criticalAt               int64
	}{
		{"background_tasks_failed", "后台任务状态", "background_tasks", `SELECT COUNT(*) FROM mochat_go_background_tasks WHERE status = 'failed'`, "失败任务 %d 个", nil, 1, 1},
		{"background_executions_failed", "后台任务执行", "background_tasks", `SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE status = 'failed' AND started_at >= DATE_SUB(NOW(), INTERVAL ? HOUR)`, "统计窗口内失败执行 %d 次", []any{options.FailureWindowHours}, 1, 5},
		{"notification_dead", "通知耗尽", "notifications", `SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE status = 'dead' AND deleted_at IS NULL`, "耗尽通知 %d 条", nil, 1, 1},
		{"notification_retry_due", "通知重试到期", "notifications", `SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE status = 'failed' AND deleted_at IS NULL AND COALESCE(next_retry_at, updated_at) <= NOW()`, "到期失败通知 %d 条", nil, 1, 20},
		{"notification_stale_pending", "通知待发积压", "notifications", `SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE status = 'pending' AND deleted_at IS NULL AND created_at <= DATE_SUB(NOW(), INTERVAL ? MINUTE) AND (next_retry_at IS NULL OR next_retry_at <= NOW())`, "超过积压阈值的待发通知 %d 条", []any{options.NotificationStaleMins}, 1, 50},
		{"approval_sla_overdue", "审批 SLA", "approvals", `SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE status = 'pending' AND effect_applied_at IS NULL AND expires_at > NOW() AND sla_due_at IS NOT NULL AND sla_due_at <= NOW()`, "超过 SLA 的待审批单 %d 条", nil, 1, 1},
		{"admin_tasks_failed", "运营任务失败", "operations", `SELECT COUNT(*) FROM mochat_go_saas_admin_tasks WHERE status = 'failed' AND deleted_at IS NULL`, "失败运营任务 %d 条", nil, 1, 1},
		{"admin_tasks_blocked", "运营任务阻断", "operations", `SELECT COUNT(*) FROM mochat_go_saas_admin_tasks WHERE status = 'blocked' AND deleted_at IS NULL`, "阻断运营任务 %d 条", nil, 1, 20},
	}
	for _, item := range queries {
		if err := appendCountCheck(item.code, item.name, item.category, item.query, item.args, item.warningAt, item.criticalAt, item.detail); err != nil {
			return nil, err
		}
	}

	var settlementStateErrors, settlementRunErrors int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_sync_states WHERE last_error <> ''`).Scan(&settlementStateErrors); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_sync_runs WHERE status = 'failed' AND started_at >= DATE_SUB(NOW(), INTERVAL ? HOUR)`, options.FailureWindowHours).Scan(&settlementRunErrors); err != nil {
		return nil, err
	}
	settlementCount := settlementStateErrors + settlementRunErrors
	settlementStatus := dashboard.SaaSAdminSystemHealthStateHealthy
	if settlementCount > 0 {
		settlementStatus = dashboard.SaaSAdminSystemHealthStateCritical
	}
	checks = append(checks, dashboard.SaaSAdminSystemHealthCheck{
		Code: "settlement_sync_failed", Name: "结算自动同步", Category: "finance", Status: settlementStatus,
		Severity: dashboard.SaaSAdminSystemHealthStateCritical, Current: settlementCount, Threshold: 1,
		Detail:   fmt.Sprintf("异常渠道 %d 个，窗口内失败运行 %d 次", settlementStateErrors, settlementRunErrors),
		Metadata: map[string]any{"stateErrorCount": settlementStateErrors, "failedRunCount": settlementRunErrors},
	})
	backupChecks, err := s.saasBackupSystemHealthChecks(ctx, options)
	if err != nil {
		return nil, err
	}
	checks = append(checks, backupChecks...)
	complianceChecks, err := s.saasComplianceSystemHealthChecks(ctx, options)
	if err != nil {
		return nil, err
	}
	checks = append(checks, complianceChecks...)
	domainDeliveryChecks, err := s.saasTenantDomainDeliverySystemHealthChecks(ctx, options)
	if err != nil {
		return nil, err
	}
	checks = append(checks, domainDeliveryChecks...)
	return checks, nil
}

func saasMigrationHealthCheck(ctx context.Context, db *sql.DB) (dashboard.SaaSAdminSystemHealthCheck, error) {
	check := dashboard.SaaSAdminSystemHealthCheck{
		Code: "schema_migration", Name: "数据库迁移", Category: "database", Status: dashboard.SaaSAdminSystemHealthStateHealthy,
		Severity: dashboard.SaaSAdminSystemHealthStateCritical, Threshold: dashboard.SaaSAdminExpectedMigrationCount,
		Metadata: map[string]any{"currentVersion": "", "expectedVersion": dashboard.SaaSAdminExpectedMigrationVersion, "migrationCount": int64(0), "ledgerAvailable": true},
	}
	rows, err := db.QueryContext(ctx, `SELECT version, checksum FROM mochat_go_schema_migrations`)
	if err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1146 {
			check.Status = dashboard.SaaSAdminSystemHealthStateCritical
			check.Detail = "迁移账本不存在；数据库初始化不完整"
			check.Metadata["ledgerAvailable"] = false
			return check, nil
		}
		return dashboard.SaaSAdminSystemHealthCheck{}, err
	}
	defer rows.Close()
	type ledgerEntry struct{ version, checksum string }
	entries := make([]ledgerEntry, 0, dashboard.SaaSAdminExpectedMigrationCount+1)
	var currentMigration string
	var supersedingMigrationValid bool
	for rows.Next() {
		var entry ledgerEntry
		if err := rows.Scan(&entry.version, &entry.checksum); err != nil {
			return dashboard.SaaSAdminSystemHealthCheck{}, err
		}
		entries = append(entries, entry)
		if entry.version > currentMigration {
			currentMigration = entry.version
		}
		if entry.version == migrationhistory.LiveCodeWorkspaceVersion && migrationhistory.IsValidLiveCodeWorkspaceChecksum(entry.checksum) {
			supersedingMigrationValid = true
		}
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminSystemHealthCheck{}, err
	}
	rawMigrationCount := int64(len(entries))
	var supersededMigrationCount int64
	for _, entry := range entries {
		if migrationhistory.IsAuditedSupersededLiveCode(entry.version, entry.checksum, supersedingMigrationValid) {
			supersededMigrationCount++
		}
	}
	migrationCount := rawMigrationCount - supersededMigrationCount
	check.Current = migrationCount
	check.Detail = fmt.Sprintf("当前 %s，共 %d 个版本", currentMigration, migrationCount)
	check.Metadata["currentVersion"] = currentMigration
	check.Metadata["migrationCount"] = migrationCount
	check.Metadata["rawMigrationCount"] = rawMigrationCount
	check.Metadata["supersededMigrationCount"] = supersededMigrationCount
	if currentMigration != dashboard.SaaSAdminExpectedMigrationVersion || migrationCount != dashboard.SaaSAdminExpectedMigrationCount {
		check.Status = dashboard.SaaSAdminSystemHealthStateCritical
		check.Detail = fmt.Sprintf("迁移未对齐：当前 %s/%d，期望 %s/%d", currentMigration, migrationCount, dashboard.SaaSAdminExpectedMigrationVersion, dashboard.SaaSAdminExpectedMigrationCount)
	}
	return check, nil
}

func (s *MySQLStore) saasTenantDomainDeliverySystemHealthChecks(ctx context.Context, options dashboard.SaaSAdminSystemHealthOptions) ([]dashboard.SaaSAdminSystemHealthCheck, error) {
	checks := make([]dashboard.SaaSAdminSystemHealthCheck, 0, 2)
	var failedJobs, expiredLeases, overdueCallbacks, stalePending int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(status = 'failed' AND updated_at >= DATE_SUB(NOW(), INTERVAL ? HOUR)), 0),
			COALESCE(SUM(status = 'processing' AND lease_expires_at IS NOT NULL AND lease_expires_at <= NOW()), 0),
			COALESCE(SUM(status = 'waiting' AND next_attempt_at IS NOT NULL AND next_attempt_at <= NOW()), 0),
			COALESCE(SUM(status = 'pending' AND COALESCE(next_attempt_at, created_at) <= DATE_SUB(NOW(), INTERVAL 15 MINUTE)), 0)
		FROM mochat_go_saas_tenant_domain_delivery_jobs
	`, options.FailureWindowHours).Scan(&failedJobs, &expiredLeases, &overdueCallbacks, &stalePending); err != nil {
		return nil, err
	}
	queueStatus := dashboard.SaaSAdminSystemHealthStateHealthy
	queueSeverity := dashboard.SaaSAdminSystemHealthStateWarning
	if failedJobs > 0 || expiredLeases > 0 {
		queueStatus = dashboard.SaaSAdminSystemHealthStateCritical
		queueSeverity = dashboard.SaaSAdminSystemHealthStateCritical
	} else if overdueCallbacks > 0 || stalePending > 0 {
		queueStatus = dashboard.SaaSAdminSystemHealthStateWarning
	}
	checks = append(checks, dashboard.SaaSAdminSystemHealthCheck{
		Code: "domain_delivery_queue", Name: "域名交付队列", Category: "domains", Status: queueStatus,
		Severity: queueSeverity, Current: failedJobs + expiredLeases + overdueCallbacks + stalePending, Threshold: 1,
		Detail:   fmt.Sprintf("统计窗口失败 %d 个，租约过期 %d 个，回调超时 %d 个，待处理积压 %d 个", failedJobs, expiredLeases, overdueCallbacks, stalePending),
		Metadata: map[string]any{"failedJobCount": failedJobs, "expiredLeaseCount": expiredLeases, "overdueCallbackCount": overdueCallbacks, "stalePendingCount": stalePending},
	})

	var certificateFailures, expiringCertificates, unconfiguredDomains int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(dd.delivery_status = 'failed' OR dd.routing_status = 'failed' OR dd.certificate_status = 'failed'
				OR (dd.certificate_expires_at IS NOT NULL AND dd.certificate_expires_at <= NOW())), 0),
			COALESCE(SUM(dd.delivery_status = 'degraded' OR dd.certificate_status = 'expiring'
				OR (dd.certificate_expires_at IS NOT NULL AND dd.certificate_expires_at > NOW()
					AND dd.certificate_expires_at <= DATE_ADD(NOW(), INTERVAL 30 DAY))), 0),
			COALESCE(SUM(dd.id IS NULL OR dd.delivery_status = 'unconfigured'), 0)
		FROM mochat_go_saas_tenant_domains d
		LEFT JOIN mochat_go_saas_tenant_domain_deliveries dd ON dd.domain_id = d.id
		WHERE d.status = 'active' AND d.verified_at IS NOT NULL AND d.deleted_at IS NULL
	`).Scan(&certificateFailures, &expiringCertificates, &unconfiguredDomains); err != nil {
		return nil, err
	}
	certificateStatus := dashboard.SaaSAdminSystemHealthStateHealthy
	certificateSeverity := dashboard.SaaSAdminSystemHealthStateWarning
	if certificateFailures > 0 {
		certificateStatus = dashboard.SaaSAdminSystemHealthStateCritical
		certificateSeverity = dashboard.SaaSAdminSystemHealthStateCritical
	} else if expiringCertificates > 0 || unconfiguredDomains > 0 {
		certificateStatus = dashboard.SaaSAdminSystemHealthStateWarning
	}
	checks = append(checks, dashboard.SaaSAdminSystemHealthCheck{
		Code: "domain_certificate_lifecycle", Name: "域名路由与 TLS", Category: "domains", Status: certificateStatus,
		Severity: certificateSeverity, Current: certificateFailures + expiringCertificates + unconfiguredDomains, Threshold: 1,
		Detail:   fmt.Sprintf("交付或证书失败 %d 个，30 天内到期 %d 个，尚未配置交付 %d 个", certificateFailures, expiringCertificates, unconfiguredDomains),
		Metadata: map[string]any{"failureCount": certificateFailures, "expiringCount": expiringCertificates, "unconfiguredCount": unconfiguredDomains, "expiryWarningDays": 30},
	})
	return checks, nil
}

func (s *MySQLStore) saasComplianceSystemHealthChecks(ctx context.Context, options dashboard.SaaSAdminSystemHealthOptions) ([]dashboard.SaaSAdminSystemHealthCheck, error) {
	checks := make([]dashboard.SaaSAdminSystemHealthCheck, 0, 2)
	var failedExports, staleExports, failedDeletions, staleDeletions int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(status = 'failed' AND updated_at >= DATE_SUB(NOW(), INTERVAL ? HOUR)), 0),
			COALESCE(SUM((status = 'running' AND started_at <= DATE_SUB(NOW(), INTERVAL 2 HOUR))
				OR (status = 'pending' AND created_at <= DATE_SUB(NOW(), INTERVAL 30 MINUTE))), 0),
			COALESCE(SUM(deletion_status = 'failed'), 0),
			COALESCE(SUM((deletion_status = 'running' AND
				(deletion_lease_expires_at IS NULL OR deletion_lease_expires_at <= NOW()))
				OR (deletion_status = 'pending' AND deletion_requested_at <= DATE_SUB(NOW(), INTERVAL 5 MINUTE))), 0)
		FROM mochat_go_saas_data_exports
	`, options.FailureWindowHours).Scan(&failedExports, &staleExports, &failedDeletions, &staleDeletions); err != nil {
		return nil, err
	}
	exportStatus := dashboard.SaaSAdminSystemHealthStateHealthy
	exportSeverity := dashboard.SaaSAdminSystemHealthStateWarning
	if staleExports > 0 || failedExports >= 3 || failedDeletions > 0 || staleDeletions > 0 {
		exportStatus = dashboard.SaaSAdminSystemHealthStateCritical
		exportSeverity = dashboard.SaaSAdminSystemHealthStateCritical
	} else if failedExports > 0 {
		exportStatus = dashboard.SaaSAdminSystemHealthStateWarning
	}
	checks = append(checks, dashboard.SaaSAdminSystemHealthCheck{
		Code: "compliance_export_queue", Name: "合规导出队列", Category: "compliance", Status: exportStatus,
		Severity: exportSeverity, Current: failedExports + staleExports + failedDeletions + staleDeletions, Threshold: 1,
		Detail: fmt.Sprintf("统计窗口导出失败 %d 个，超时待处理或运行 %d 个，删除失败 %d 个，删除待处理或租约过期 %d 个",
			failedExports, staleExports, failedDeletions, staleDeletions),
		Metadata: map[string]any{"failedExportCount": failedExports, "staleExportCount": staleExports,
			"failedDeletionCount": failedDeletions, "staleDeletionCount": staleDeletions},
	})

	var failedErasures, blockedErasures, staleErasures int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(status = 'failed' AND updated_at >= DATE_SUB(NOW(), INTERVAL ? HOUR)), 0),
			COALESCE(SUM(status = 'blocked'), 0),
			COALESCE(SUM(status = 'running' AND started_at <= DATE_SUB(NOW(), INTERVAL 2 HOUR)), 0)
		FROM mochat_go_saas_erasure_requests
	`, options.FailureWindowHours).Scan(&failedErasures, &blockedErasures, &staleErasures); err != nil {
		return nil, err
	}
	erasureStatus := dashboard.SaaSAdminSystemHealthStateHealthy
	erasureSeverity := dashboard.SaaSAdminSystemHealthStateWarning
	if failedErasures > 0 || staleErasures > 0 {
		erasureStatus = dashboard.SaaSAdminSystemHealthStateCritical
		erasureSeverity = dashboard.SaaSAdminSystemHealthStateCritical
	} else if blockedErasures > 0 {
		erasureStatus = dashboard.SaaSAdminSystemHealthStateWarning
	}
	checks = append(checks, dashboard.SaaSAdminSystemHealthCheck{
		Code: "compliance_erasure_queue", Name: "租户数据擦除队列", Category: "compliance", Status: erasureStatus,
		Severity: erasureSeverity, Current: failedErasures + blockedErasures + staleErasures, Threshold: 1,
		Detail:   fmt.Sprintf("失败 %d 个，门禁阻断 %d 个，超时运行 %d 个", failedErasures, blockedErasures, staleErasures),
		Metadata: map[string]any{"failedErasureCount": failedErasures, "blockedErasureCount": blockedErasures, "staleErasureCount": staleErasures},
	})
	return checks, nil
}

func (s *MySQLStore) saasBackupSystemHealthChecks(ctx context.Context, options dashboard.SaaSAdminSystemHealthOptions) ([]dashboard.SaaSAdminSystemHealthCheck, error) {
	policy, err := s.BackupPolicy(ctx)
	if err != nil {
		return nil, err
	}
	checks := make([]dashboard.SaaSAdminSystemHealthCheck, 0, 5)

	freshness := dashboard.SaaSAdminSystemHealthCheck{
		Code: "backup_freshness", Name: "数据库备份新鲜度", Category: "disaster_recovery",
		Status: dashboard.SaaSAdminSystemHealthStateHealthy, Severity: dashboard.SaaSAdminSystemHealthStateCritical,
		Threshold: int64(policy.MaxBackupAgeMinutes), Metadata: map[string]any{"policyStatus": policy.Status},
	}
	var latestBackupAt sql.NullTime
	var latestBackupNo string
	err = s.db.QueryRowContext(ctx, `
		SELECT backup_no, finished_at FROM mochat_go_saas_backup_runs
		WHERE status = 'succeeded' AND verification_status = 'passed' AND deleted_at IS NULL
		ORDER BY finished_at DESC, id DESC LIMIT 1
	`).Scan(&latestBackupNo, &latestBackupAt)
	if errors.Is(err, sql.ErrNoRows) {
		freshness.Status = dashboard.SaaSAdminSystemHealthStateCritical
		freshness.Current = int64(policy.MaxBackupAgeMinutes) + 1
		freshness.Detail = "尚无通过完整性校验的成功备份"
	} else if err != nil {
		return nil, err
	} else {
		ageMinutes := int64(time.Since(latestBackupAt.Time) / time.Minute)
		if ageMinutes < 0 {
			ageMinutes = 0
		}
		freshness.Current = ageMinutes
		freshness.Detail = fmt.Sprintf("最近备份 %s，距今 %d 分钟", latestBackupNo, ageMinutes)
		freshness.Metadata["latestBackupNo"] = latestBackupNo
		if ageMinutes > int64(policy.MaxBackupAgeMinutes) {
			freshness.Status = dashboard.SaaSAdminSystemHealthStateCritical
		} else if ageMinutes*100 >= int64(policy.MaxBackupAgeMinutes)*80 {
			freshness.Status = dashboard.SaaSAdminSystemHealthStateWarning
			freshness.Severity = dashboard.SaaSAdminSystemHealthStateWarning
		}
	}
	if policy.Status != saasbackup.PolicyStatusActive {
		freshness.Status = dashboard.SaaSAdminSystemHealthStateCritical
		freshness.Detail = "数据库备份策略已停用"
	}
	checks = append(checks, freshness)

	var failedRuns, staleRuns int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_backup_runs
		WHERE status = 'failed' AND finished_at >= DATE_SUB(NOW(), INTERVAL ? HOUR)
	`, options.FailureWindowHours).Scan(&failedRuns); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_backup_runs
		WHERE status = 'running' AND started_at <= DATE_SUB(NOW(), INTERVAL 24 HOUR)
	`).Scan(&staleRuns); err != nil {
		return nil, err
	}
	failureTotal := failedRuns + staleRuns
	failureStatus := dashboard.SaaSAdminSystemHealthStateHealthy
	failureSeverity := dashboard.SaaSAdminSystemHealthStateWarning
	if failureTotal >= 3 || staleRuns > 0 {
		failureStatus = dashboard.SaaSAdminSystemHealthStateCritical
		failureSeverity = dashboard.SaaSAdminSystemHealthStateCritical
	} else if failureTotal > 0 {
		failureStatus = dashboard.SaaSAdminSystemHealthStateWarning
	}
	checks = append(checks, dashboard.SaaSAdminSystemHealthCheck{
		Code: "backup_failures", Name: "数据库备份失败", Category: "disaster_recovery", Status: failureStatus,
		Severity: failureSeverity, Current: failureTotal, Threshold: 1,
		Detail:   fmt.Sprintf("统计窗口失败 %d 次，租约过期运行 %d 个", failedRuns, staleRuns),
		Metadata: map[string]any{"failedRunCount": failedRuns, "staleRunCount": staleRuns},
	})

	var unencrypted int64
	if policy.RequireEncryption {
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM mochat_go_saas_backup_runs
			WHERE status = 'succeeded' AND deleted_at IS NULL AND encrypted = 0
		`).Scan(&unencrypted); err != nil {
			return nil, err
		}
	}
	encryptionStatus := dashboard.SaaSAdminSystemHealthStateHealthy
	if unencrypted > 0 {
		encryptionStatus = dashboard.SaaSAdminSystemHealthStateCritical
	}
	checks = append(checks, dashboard.SaaSAdminSystemHealthCheck{
		Code: "backup_encryption", Name: "备份加密合规", Category: "disaster_recovery", Status: encryptionStatus,
		Severity: dashboard.SaaSAdminSystemHealthStateCritical, Current: unencrypted, Threshold: 1,
		Detail:   fmt.Sprintf("策略要求加密=%t，未加密成功备份 %d 个", policy.RequireEncryption, unencrypted),
		Metadata: map[string]any{"requireEncryption": policy.RequireEncryption},
	})

	var replicaNonCompliant, replicaFailures int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_backup_runs
		WHERE status = 'succeeded' AND deleted_at IS NULL AND replica_status <> 'succeeded'
	`).Scan(&replicaNonCompliant); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_backup_runs
		WHERE status = 'succeeded' AND deleted_at IS NULL AND replica_status IN ('failed', 'uploading')
	`).Scan(&replicaFailures); err != nil {
		return nil, err
	}
	replicaStatus := dashboard.SaaSAdminSystemHealthStateHealthy
	replicaSeverity := dashboard.SaaSAdminSystemHealthStateWarning
	if policy.RequireOffsiteReplica && replicaNonCompliant > 0 {
		replicaStatus = dashboard.SaaSAdminSystemHealthStateCritical
		replicaSeverity = dashboard.SaaSAdminSystemHealthStateCritical
	} else if replicaFailures > 0 {
		replicaStatus = dashboard.SaaSAdminSystemHealthStateWarning
	}
	checks = append(checks, dashboard.SaaSAdminSystemHealthCheck{
		Code: "backup_replica", Name: "备份异地副本", Category: "disaster_recovery", Status: replicaStatus,
		Severity: replicaSeverity, Current: replicaNonCompliant, Threshold: 1,
		Detail:   fmt.Sprintf("策略要求异地副本=%t，未完成副本 %d 个，上传异常 %d 个", policy.RequireOffsiteReplica, replicaNonCompliant, replicaFailures),
		Metadata: map[string]any{"requireOffsiteReplica": policy.RequireOffsiteReplica, "replicaFailureCount": replicaFailures},
	})

	drill := dashboard.SaaSAdminSystemHealthCheck{
		Code: "restore_drill_freshness", Name: "隔离恢复演练", Category: "disaster_recovery",
		Status: dashboard.SaaSAdminSystemHealthStateHealthy, Severity: dashboard.SaaSAdminSystemHealthStateCritical,
		Threshold: int64(policy.RestoreDrillIntervalDays * 1440),
	}
	var latestDrillAt sql.NullTime
	var latestDrillNo string
	err = s.db.QueryRowContext(ctx, `
		SELECT drill_no, finished_at FROM mochat_go_saas_restore_drills
		WHERE status = 'succeeded' ORDER BY finished_at DESC, id DESC LIMIT 1
	`).Scan(&latestDrillNo, &latestDrillAt)
	if errors.Is(err, sql.ErrNoRows) {
		drill.Status = dashboard.SaaSAdminSystemHealthStateCritical
		drill.Current = drill.Threshold + 1
		drill.Detail = "尚无成功的隔离恢复演练"
	} else if err != nil {
		return nil, err
	} else {
		ageMinutes := int64(time.Since(latestDrillAt.Time) / time.Minute)
		if ageMinutes < 0 {
			ageMinutes = 0
		}
		drill.Current = ageMinutes
		drill.Detail = fmt.Sprintf("最近演练 %s，距今 %d 分钟", latestDrillNo, ageMinutes)
		drill.Metadata = map[string]any{"latestDrillNo": latestDrillNo}
		if ageMinutes > drill.Threshold {
			drill.Status = dashboard.SaaSAdminSystemHealthStateCritical
		} else if ageMinutes*100 >= drill.Threshold*80 {
			drill.Status = dashboard.SaaSAdminSystemHealthStateWarning
			drill.Severity = dashboard.SaaSAdminSystemHealthStateWarning
		}
	}
	var cleanupFailures, staleCleanup, retainedTargets int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(target_cleanup_status = 'failed'), 0),
			COALESCE(SUM(target_cleanup_status = 'pending' AND started_at <= DATE_SUB(NOW(), INTERVAL 24 HOUR)), 0),
			COALESCE(SUM(target_cleanup_status = 'retained'), 0)
		FROM mochat_go_saas_restore_drills
		WHERE target_lifecycle = 'ephemeral'
	`).Scan(&cleanupFailures, &staleCleanup, &retainedTargets); err != nil {
		return nil, err
	}
	if drill.Metadata == nil {
		drill.Metadata = map[string]any{}
	}
	drill.Metadata["cleanupFailureCount"] = cleanupFailures
	drill.Metadata["staleCleanupCount"] = staleCleanup
	drill.Metadata["retainedTargetCount"] = retainedTargets
	if cleanupFailures > 0 || staleCleanup > 0 {
		drill.Status = dashboard.SaaSAdminSystemHealthStateCritical
		drill.Severity = dashboard.SaaSAdminSystemHealthStateCritical
		drill.Detail += fmt.Sprintf("；临时库清理失败 %d 个，超时待清理 %d 个", cleanupFailures, staleCleanup)
	} else if retainedTargets > 0 && drill.Status == dashboard.SaaSAdminSystemHealthStateHealthy {
		drill.Status = dashboard.SaaSAdminSystemHealthStateWarning
		drill.Severity = dashboard.SaaSAdminSystemHealthStateWarning
		drill.Detail += fmt.Sprintf("；排障保留临时库 %d 个", retainedTargets)
	}
	checks = append(checks, drill)
	return checks, nil
}

func (s *MySQLStore) PersistSaaSAdminSystemHealthScan(ctx context.Context, input dashboard.SaaSAdminSystemHealthScanInput) (dashboard.SaaSAdminSystemHealthScanResult, error) {
	if strings.TrimSpace(input.ScanNo) == "" || input.PlatformTenantID <= 0 || len(input.Checks) == 0 {
		return dashboard.SaaSAdminSystemHealthScanResult{}, dashboard.NewSaaSAdminBadRequest("健康扫描输入无效")
	}
	if input.MaxAttempts <= 0 || input.MaxAttempts > 20 {
		input.MaxAttempts = 3
	}
	if input.StartedAt.IsZero() {
		input.StartedAt = time.Now()
	}
	if input.FinishedAt.IsZero() {
		input.FinishedAt = time.Now()
	}
	snapshot, err := json.Marshal(map[string]any{"summary": input.Summary, "checks": input.Checks})
	if err != nil {
		return dashboard.SaaSAdminSystemHealthScanResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminSystemHealthScanResult{}, err
	}
	defer rollbackQuietly(tx)
	insert, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_health_scans
			(scan_no, trigger_type, status, health_state, check_count, issue_count, critical_count, warning_count,
			 opened_count, reopened_count, recovered_count, notification_count, failure_window_hours,
			 notification_stale_minutes, actor_user_id, actor_tenant_id, started_at, finished_at, snapshot_json,
			 error_message, operation_id, created_at)
		VALUES (?, ?, 'completed', ?, ?, ?, ?, ?, 0, 0, 0, 0, ?, ?, ?, ?, ?, ?, ?, '', 0, NOW())
	`, input.ScanNo, input.TriggerType, input.Summary.HealthState, input.Summary.CheckCount, input.Summary.IssueCount,
		input.Summary.CriticalCount, input.Summary.WarningCount, input.Options.FailureWindowHours, input.Options.NotificationStaleMins,
		input.ActorUserID, input.ActorTenantID, input.StartedAt, input.FinishedAt, snapshot)
	if err != nil {
		return dashboard.SaaSAdminSystemHealthScanResult{}, err
	}
	scanID, _ := insert.LastInsertId()

	existingRows, err := tx.QueryContext(ctx, saasAdminSystemIncidentSelect+` WHERE status IN ('open', 'acknowledged') FOR UPDATE`)
	if err != nil {
		return dashboard.SaaSAdminSystemHealthScanResult{}, err
	}
	existing := make(map[string]dashboard.SaaSAdminSystemIncident)
	for existingRows.Next() {
		item, err := scanSaaSAdminSystemIncident(existingRows)
		if err != nil {
			existingRows.Close()
			return dashboard.SaaSAdminSystemHealthScanResult{}, err
		}
		existing[item.IncidentKey] = item
	}
	if err := existingRows.Close(); err != nil {
		return dashboard.SaaSAdminSystemHealthScanResult{}, err
	}
	known := make(map[string]dashboard.SaaSAdminSystemHealthCheck, len(input.Checks))
	issues := make(map[string]dashboard.SaaSAdminSystemHealthCheck)
	for _, check := range input.Checks {
		known[check.Code] = check
		if check.Status == dashboard.SaaSAdminSystemHealthStateWarning || check.Status == dashboard.SaaSAdminSystemHealthStateCritical {
			issues[check.Code] = check
		}
	}

	result := dashboard.SaaSAdminSystemHealthScanResult{NotificationKeys: []string{}}
	now := input.FinishedAt
	for incidentKey, item := range existing {
		if _, stillUnhealthy := issues[incidentKey]; stillUnhealthy {
			continue
		}
		if _, checked := known[incidentKey]; !checked {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_admin_system_incidents
			SET status = 'resolved', resolved_at = ?, resolved_by = 0, resolution_note = 'auto_recovered',
				last_scan_id = ?, version = version + 1, updated_at = NOW()
			WHERE id = ? AND version = ?
		`, now, scanID, item.ID, item.Version); err != nil {
			return dashboard.SaaSAdminSystemHealthScanResult{}, err
		}
		result.RecoveredCount++
	}

	for _, check := range input.Checks {
		if check.Status != dashboard.SaaSAdminSystemHealthStateWarning && check.Status != dashboard.SaaSAdminSystemHealthStateCritical {
			continue
		}
		metadata, err := json.Marshal(check.Metadata)
		if err != nil {
			return dashboard.SaaSAdminSystemHealthScanResult{}, err
		}
		before, found, err := saasAdminSystemIncidentByKey(ctx, tx, check.Code, true)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return dashboard.SaaSAdminSystemHealthScanResult{}, err
		}
		opened, reopened, escalated := false, false, false
		occurrence := int64(1)
		if !found || errors.Is(err, sql.ErrNoRows) {
			insert, err := tx.ExecContext(ctx, `
				INSERT INTO mochat_go_saas_admin_system_incidents
					(incident_key, source, category, severity, status, title, detail, current_value, threshold_value,
					 occurrence_count, first_detected_at, last_detected_at, last_scan_id, acknowledged_at, acknowledged_by,
					 resolved_at, resolved_by, owner, resolution_note, metadata_json, version, created_at, updated_at)
				VALUES (?, ?, ?, ?, 'open', ?, ?, ?, ?, 1, ?, ?, ?, NULL, 0, NULL, 0, '', '', ?, 1, NOW(), NOW())
			`, check.Code, check.Code, check.Category, check.Status, truncateRunes(check.Name, 160), truncateRunes(check.Detail, 1000),
				check.Current, check.Threshold, now, now, scanID, metadata)
			if err != nil {
				return dashboard.SaaSAdminSystemHealthScanResult{}, err
			}
			before.ID, _ = insert.LastInsertId()
			opened = true
			result.OpenedCount++
		} else {
			occurrence = before.OccurrenceCount + 1
			reopened = before.Status == dashboard.SaaSAdminSystemIncidentStatusResolved
			escalated = before.Severity == dashboard.SaaSAdminSystemHealthStateWarning && check.Status == dashboard.SaaSAdminSystemHealthStateCritical
			status := before.Status
			acknowledgedAt := nullableTimeString(before.AcknowledgedAt)
			acknowledgedBy := before.AcknowledgedBy
			resolvedAt := nullableTimeString(before.ResolvedAt)
			resolvedBy := before.ResolvedBy
			resolutionNote := before.ResolutionNote
			if reopened || escalated {
				status = dashboard.SaaSAdminSystemIncidentStatusOpen
				acknowledgedAt = nil
				acknowledgedBy = 0
				resolvedAt = nil
				resolvedBy = 0
				resolutionNote = ""
				result.ReopenedCount++
			}
			update, err := tx.ExecContext(ctx, `
				UPDATE mochat_go_saas_admin_system_incidents
				SET source = ?, category = ?, severity = ?, status = ?, title = ?, detail = ?, current_value = ?,
					threshold_value = ?, occurrence_count = ?, last_detected_at = ?, last_scan_id = ?, acknowledged_at = ?,
					acknowledged_by = ?, resolved_at = ?, resolved_by = ?, resolution_note = ?, metadata_json = ?,
					version = version + 1, updated_at = NOW()
				WHERE id = ? AND version = ?
			`, check.Code, check.Category, check.Status, status, truncateRunes(check.Name, 160), truncateRunes(check.Detail, 1000),
				check.Current, check.Threshold, occurrence, now, scanID, acknowledgedAt, acknowledgedBy, resolvedAt, resolvedBy,
				resolutionNote, metadata, before.ID, before.Version)
			if err != nil {
				return dashboard.SaaSAdminSystemHealthScanResult{}, err
			}
			if affected, _ := update.RowsAffected(); affected != 1 {
				return dashboard.SaaSAdminSystemHealthScanResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "系统事故版本已变化，请重试健康扫描"}
			}
		}
		if input.Notify && (opened || reopened || escalated) {
			notificationKey, enqueued, err := insertSaaSAdminSystemHealthNotification(ctx, tx, input, check, occurrence)
			if err != nil {
				return dashboard.SaaSAdminSystemHealthScanResult{}, err
			}
			if enqueued {
				result.NotificationKeys = append(result.NotificationKeys, notificationKey)
			}
		}
	}

	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionSystemHealthScan, TargetType: dashboard.SaaSAdminOperationTargetSystemHealthScan,
		TargetID: strconv.FormatInt(scanID, 10), TargetName: input.ScanNo, BeforeJSON: "{}",
		AfterJSON: saasAdminMarshalJSON(map[string]any{"healthState": input.Summary.HealthState, "checkCount": input.Summary.CheckCount,
			"issueCount": input.Summary.IssueCount, "criticalCount": input.Summary.CriticalCount, "warningCount": input.Summary.WarningCount,
			"openedCount": result.OpenedCount, "reopenedCount": result.ReopenedCount, "recoveredCount": result.RecoveredCount,
			"notificationCount": len(result.NotificationKeys), "triggerType": input.TriggerType}),
		Remark: "scan SaaS admin system health",
	})
	if err != nil {
		return dashboard.SaaSAdminSystemHealthScanResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_health_scans
		SET opened_count = ?, reopened_count = ?, recovered_count = ?, notification_count = ?, operation_id = ?
		WHERE id = ?
	`, result.OpenedCount, result.ReopenedCount, result.RecoveredCount, len(result.NotificationKeys), operationID, scanID); err != nil {
		return dashboard.SaaSAdminSystemHealthScanResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminSystemHealthScanResult{}, err
	}
	result.Scan, err = s.saasAdminSystemHealthScanByID(ctx, scanID)
	if err != nil {
		return dashboard.SaaSAdminSystemHealthScanResult{}, err
	}
	result.Incidents, err = s.SaaSAdminSystemIncidents(ctx, dashboard.SaaSAdminSystemIncidentOptions{Status: dashboard.SaaSAdminSystemIncidentStatusActive, Severity: dashboard.SaaSAdminSystemIncidentStatusAll, Limit: 500})
	return result, err
}

func insertSaaSAdminSystemHealthNotification(ctx context.Context, tx *sql.Tx, input dashboard.SaaSAdminSystemHealthScanInput, check dashboard.SaaSAdminSystemHealthCheck, occurrence int64) (string, bool, error) {
	periodKey := fmt.Sprintf("incident_%s_%d", check.Code, occurrence)
	notificationKey := fmt.Sprintf("%d:%s:%s:%s:webhook", input.PlatformTenantID, dashboard.SaaSEventMetricSystemHealth, dashboard.SaaSAlertTypeSystemHealthIncident, periodKey)
	alert := dashboard.SaaSQuotaAlert{
		Status:    dashboard.SaaSQuotaStatus{TenantID: input.PlatformTenantID, Metric: dashboard.SaaSEventMetricSystemHealth, Current: check.Current, Limit: check.Threshold},
		AlertType: dashboard.SaaSAlertTypeSystemHealthIncident, Severity: check.Status, PeriodKey: periodKey,
		Source: "saas_admin.system_health", Message: check.Name + "：" + check.Detail,
		Context: map[string]any{"incidentKey": check.Code, "category": check.Category, "scanNo": input.ScanNo, "occurrence": occurrence},
	}
	raw, err := json.Marshal(alert)
	if err != nil {
		return "", false, err
	}
	alertKey := fmt.Sprintf("%d:%s:%s:%s", input.PlatformTenantID, dashboard.SaaSEventMetricSystemHealth, dashboard.SaaSAlertTypeSystemHealthIncident, periodKey)
	insert, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO mochat_go_saas_alert_notifications
			(notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json,
			 last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, 'webhook', 'pending', 0, ?, ?, '', NOW(), NULL, NOW(), NOW(), NULL)
	`, notificationKey, alertKey, input.PlatformTenantID, input.MaxAttempts, raw)
	if err != nil {
		return "", false, err
	}
	affected, _ := insert.RowsAffected()
	return notificationKey, affected == 1, nil
}

func (s *MySQLStore) SaaSAdminSystemHealthScans(ctx context.Context, limit int) ([]dashboard.SaaSAdminSystemHealthScan, error) {
	if limit <= 0 || limit > 500 {
		limit = 30
	}
	rows, err := s.db.QueryContext(ctx, saasAdminSystemHealthScanSelect+` ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSAdminSystemHealthScan, 0)
	for rows.Next() {
		item, err := scanSaaSAdminSystemHealthScan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) SaaSAdminSystemIncidents(ctx context.Context, options dashboard.SaaSAdminSystemIncidentOptions) ([]dashboard.SaaSAdminSystemIncident, error) {
	if options.Limit <= 0 || options.Limit > 500 {
		options.Limit = 100
	}
	where := []string{"1 = 1"}
	args := make([]any, 0)
	switch options.Status {
	case dashboard.SaaSAdminSystemIncidentStatusActive:
		where = append(where, "status IN ('open', 'acknowledged')")
	case dashboard.SaaSAdminSystemIncidentStatusOpen, dashboard.SaaSAdminSystemIncidentStatusAcknowledged, dashboard.SaaSAdminSystemIncidentStatusResolved:
		where = append(where, "status = ?")
		args = append(args, options.Status)
	}
	if options.Severity == dashboard.SaaSAdminSystemHealthStateWarning || options.Severity == dashboard.SaaSAdminSystemHealthStateCritical {
		where = append(where, "severity = ?")
		args = append(args, options.Severity)
	}
	if strings.TrimSpace(options.Source) != "" {
		where = append(where, "source = ?")
		args = append(args, strings.TrimSpace(options.Source))
	}
	if strings.TrimSpace(options.Owner) != "" {
		where = append(where, "owner = ?")
		args = append(args, strings.TrimSpace(options.Owner))
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where = append(where, "(incident_key LIKE ? OR title LIKE ? OR detail LIKE ? OR owner LIKE ?)")
		args = append(args, like, like, like, like)
	}
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, saasAdminSystemIncidentSelect+` WHERE `+strings.Join(where, " AND ")+` ORDER BY FIELD(status, 'open', 'acknowledged', 'resolved'), FIELD(severity, 'critical', 'warning'), last_detected_at DESC, id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSAdminSystemIncident, 0)
	for rows.Next() {
		item, err := scanSaaSAdminSystemIncident(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) UpdateSaaSAdminSystemIncident(ctx context.Context, input dashboard.SaaSAdminSystemIncidentUpdate) (dashboard.SaaSAdminSystemIncidentUpdateResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminSystemIncidentUpdateResult{}, err
	}
	defer rollbackQuietly(tx)
	before, err := saasAdminSystemIncidentByID(ctx, tx, input.IncidentID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminSystemIncidentUpdateResult{}, dashboard.NewSaaSAdminNotFound("系统事故不存在")
	}
	if err != nil {
		return dashboard.SaaSAdminSystemIncidentUpdateResult{}, err
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSAdminSystemIncidentUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "系统事故版本已变化，请刷新后重试"}
	}
	status := before.Status
	owner := before.Owner
	note := before.ResolutionNote
	acknowledgedAt := nullableTimeString(before.AcknowledgedAt)
	acknowledgedBy := before.AcknowledgedBy
	resolvedAt := nullableTimeString(before.ResolvedAt)
	resolvedBy := before.ResolvedBy
	now := time.Now()
	switch input.Action {
	case dashboard.SaaSAdminSystemIncidentActionAcknowledge:
		if status == dashboard.SaaSAdminSystemIncidentStatusResolved {
			return dashboard.SaaSAdminSystemIncidentUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "已解决事故不能认领，请先重开"}
		}
		status, owner, note = dashboard.SaaSAdminSystemIncidentStatusAcknowledged, input.Owner, input.Note
		acknowledgedAt, acknowledgedBy = now, input.ActorUserID
	case dashboard.SaaSAdminSystemIncidentActionResolve:
		if status == dashboard.SaaSAdminSystemIncidentStatusResolved {
			return dashboard.SaaSAdminSystemIncidentUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "事故已经解决"}
		}
		status, note = dashboard.SaaSAdminSystemIncidentStatusResolved, input.Note
		if input.Owner != "" {
			owner = input.Owner
		}
		resolvedAt, resolvedBy = now, input.ActorUserID
	case dashboard.SaaSAdminSystemIncidentActionReopen:
		if status != dashboard.SaaSAdminSystemIncidentStatusResolved {
			return dashboard.SaaSAdminSystemIncidentUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "只有已解决事故可以重开"}
		}
		status, note = dashboard.SaaSAdminSystemIncidentStatusOpen, input.Note
		if input.Owner != "" {
			owner = input.Owner
		}
		resolvedAt, resolvedBy = nil, 0
	case dashboard.SaaSAdminSystemIncidentActionAssign:
		if status == dashboard.SaaSAdminSystemIncidentStatusResolved {
			return dashboard.SaaSAdminSystemIncidentUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "已解决事故不能分派"}
		}
		owner, note = input.Owner, input.Note
	default:
		return dashboard.SaaSAdminSystemIncidentUpdateResult{}, dashboard.NewSaaSAdminBadRequest("事故操作无效")
	}
	update, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_system_incidents
		SET status = ?, owner = ?, resolution_note = ?, acknowledged_at = ?, acknowledged_by = ?, resolved_at = ?, resolved_by = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, status, truncateRunes(owner, 80), truncateRunes(note, 255), acknowledgedAt, acknowledgedBy, resolvedAt, resolvedBy, before.ID, input.ExpectedVersion)
	if err != nil {
		return dashboard.SaaSAdminSystemIncidentUpdateResult{}, err
	}
	if affected, _ := update.RowsAffected(); affected != 1 {
		return dashboard.SaaSAdminSystemIncidentUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "系统事故版本已变化，请刷新后重试"}
	}
	after, err := saasAdminSystemIncidentByID(ctx, tx, before.ID, false)
	if err != nil {
		return dashboard.SaaSAdminSystemIncidentUpdateResult{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionSystemIncidentUpdate, TargetType: dashboard.SaaSAdminOperationTargetSystemIncident,
		TargetID: strconv.FormatInt(before.ID, 10), TargetName: before.Title,
		BeforeJSON: saasAdminMarshalJSON(systemIncidentAuditPayload(before)), AfterJSON: saasAdminMarshalJSON(systemIncidentAuditPayload(after)),
		Remark: input.Action + ": " + input.Note,
	})
	if err != nil {
		return dashboard.SaaSAdminSystemIncidentUpdateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminSystemIncidentUpdateResult{}, err
	}
	return dashboard.SaaSAdminSystemIncidentUpdateResult{Incident: after, OperationID: operationID}, nil
}

const saasAdminSystemHealthScanSelect = `
	SELECT id, scan_no, trigger_type, status, health_state, check_count, issue_count, critical_count, warning_count,
		opened_count, reopened_count, recovered_count, notification_count, failure_window_hours, notification_stale_minutes,
		actor_user_id, actor_tenant_id, started_at, finished_at, COALESCE(CAST(snapshot_json AS CHAR), ''), error_message,
		operation_id, created_at
	FROM mochat_go_saas_admin_health_scans
`

const saasAdminSystemIncidentSelect = `
	SELECT id, incident_key, source, category, severity, status, title, detail, current_value, threshold_value,
		occurrence_count, first_detected_at, last_detected_at, last_scan_id, acknowledged_at, acknowledged_by,
		resolved_at, resolved_by, owner, resolution_note, COALESCE(CAST(metadata_json AS CHAR), ''), version, created_at, updated_at
	FROM mochat_go_saas_admin_system_incidents
`

func (s *MySQLStore) saasAdminSystemHealthScanByID(ctx context.Context, id int64) (dashboard.SaaSAdminSystemHealthScan, error) {
	return scanSaaSAdminSystemHealthScan(s.db.QueryRowContext(ctx, saasAdminSystemHealthScanSelect+` WHERE id = ?`, id))
}

func saasAdminSystemIncidentByKey(ctx context.Context, queryer saasAdminApprovalGovernanceQueryer, key string, forUpdate bool) (dashboard.SaaSAdminSystemIncident, bool, error) {
	query := saasAdminSystemIncidentSelect + ` WHERE incident_key = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	item, err := scanSaaSAdminSystemIncident(queryer.QueryRowContext(ctx, query, key))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminSystemIncident{}, false, err
	}
	return item, err == nil, err
}

func saasAdminSystemIncidentByID(ctx context.Context, queryer saasAdminApprovalGovernanceQueryer, id int64, forUpdate bool) (dashboard.SaaSAdminSystemIncident, error) {
	query := saasAdminSystemIncidentSelect + ` WHERE id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanSaaSAdminSystemIncident(queryer.QueryRowContext(ctx, query, id))
}

func scanSaaSAdminSystemHealthScan(scanner interface{ Scan(...any) error }) (dashboard.SaaSAdminSystemHealthScan, error) {
	var item dashboard.SaaSAdminSystemHealthScan
	var startedAt, finishedAt, createdAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.ScanNo, &item.TriggerType, &item.Status, &item.HealthState, &item.CheckCount,
		&item.IssueCount, &item.CriticalCount, &item.WarningCount, &item.OpenedCount, &item.ReopenedCount,
		&item.RecoveredCount, &item.NotificationCount, &item.FailureWindowHours, &item.NotificationStaleMinutes,
		&item.ActorUserID, &item.ActorTenantID, &startedAt, &finishedAt, &item.SnapshotJSON, &item.ErrorMessage,
		&item.OperationID, &createdAt)
	item.StartedAt, item.FinishedAt, item.CreatedAt = formatTime(startedAt), formatTime(finishedAt), formatTime(createdAt)
	return item, err
}

func scanSaaSAdminSystemIncident(scanner interface{ Scan(...any) error }) (dashboard.SaaSAdminSystemIncident, error) {
	var item dashboard.SaaSAdminSystemIncident
	var firstDetectedAt, lastDetectedAt, acknowledgedAt, resolvedAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.IncidentKey, &item.Source, &item.Category, &item.Severity, &item.Status,
		&item.Title, &item.Detail, &item.CurrentValue, &item.ThresholdValue, &item.OccurrenceCount, &firstDetectedAt,
		&lastDetectedAt, &item.LastScanID, &acknowledgedAt, &item.AcknowledgedBy, &resolvedAt, &item.ResolvedBy,
		&item.Owner, &item.ResolutionNote, &item.MetadataJSON, &item.Version, &createdAt, &updatedAt)
	item.FirstDetectedAt, item.LastDetectedAt = formatTime(firstDetectedAt), formatTime(lastDetectedAt)
	item.AcknowledgedAt, item.ResolvedAt = formatTime(acknowledgedAt), formatTime(resolvedAt)
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, err
}

func nullableTimeString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	if err != nil {
		return nil
	}
	return parsed
}

func systemIncidentAuditPayload(item dashboard.SaaSAdminSystemIncident) map[string]any {
	return map[string]any{
		"id": item.ID, "incidentKey": item.IncidentKey, "severity": item.Severity, "status": item.Status,
		"owner": item.Owner, "resolutionNote": item.ResolutionNote, "version": item.Version,
		"acknowledgedAt": item.AcknowledgedAt, "acknowledgedBy": item.AcknowledgedBy,
		"resolvedAt": item.ResolvedAt, "resolvedBy": item.ResolvedBy,
	}
}
