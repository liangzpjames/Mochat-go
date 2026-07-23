package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/saasbackup"
)

func (s *MySQLStore) BackupPolicy(ctx context.Context) (saasbackup.Policy, error) {
	return scanSaaSBackupPolicy(s.db.QueryRowContext(ctx, saasBackupPolicySelect+` WHERE id = 1`))
}

func (s *MySQLStore) UpdateBackupPolicy(ctx context.Context, input saasbackup.PolicyUpdate) (saasbackup.Policy, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.Policy{}, err
	}
	defer rollbackQuietly(tx)
	before, err := scanSaaSBackupPolicy(tx.QueryRowContext(ctx, saasBackupPolicySelect+` WHERE id = 1 FOR UPDATE`))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.Policy{}, saasbackup.NotFound("备份策略不存在")
	}
	if err != nil {
		return saasbackup.Policy{}, err
	}
	if before.Version != input.ExpectedVersion {
		return saasbackup.Policy{}, saasbackup.Conflict("备份策略版本已变化，请刷新后重试")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_policies
		SET status = ?, interval_minutes = ?, retention_days = ?, min_successful_backups = ?,
			max_backup_age_minutes = ?, restore_drill_interval_days = ?, require_encryption = ?, require_offsite_replica = ?,
			version = version + 1, updated_by = ?, updated_at = NOW()
		WHERE id = 1 AND version = ?
	`, input.Status, input.IntervalMinutes, input.RetentionDays, input.MinSuccessfulBackups, input.MaxBackupAgeMinutes,
		input.RestoreDrillIntervalDays, boolInt(input.RequireEncryption), boolInt(input.RequireOffsiteReplica),
		input.Actor.UserID, input.ExpectedVersion)
	if err != nil {
		return saasbackup.Policy{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saasbackup.Policy{}, saasbackup.Conflict("备份策略版本已变化，请刷新后重试")
	}
	after, err := scanSaaSBackupPolicy(tx.QueryRowContext(ctx, saasBackupPolicySelect+` WHERE id = 1`))
	if err != nil {
		return saasbackup.Policy{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.Actor.UserID, ActorTenantID: input.Actor.TenantID,
		Action: saasbackup.OperationActionPolicyUpdate, TargetType: "saas_backup_policy", TargetID: "1", TargetName: "平台数据库备份策略",
		BeforeJSON: saasBackupPolicyAuditJSON(before), AfterJSON: saasBackupPolicyAuditJSON(after), Remark: "update SaaS backup policy",
	})
	if err != nil {
		return saasbackup.Policy{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.Actor.UserID, operationID); err != nil {
		return saasbackup.Policy{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.Policy{}, err
	}
	return after, nil
}

func (s *MySQLStore) BackupSourceMetadata(ctx context.Context) (saasbackup.SourceMetadata, error) {
	var metadata saasbackup.SourceMetadata
	if err := s.db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&metadata.DatabaseName); err != nil {
		return metadata, err
	}
	if strings.TrimSpace(metadata.DatabaseName) == "" {
		return metadata, saasbackup.Invalid("备份源 DSN 必须指定数据库名")
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`, metadata.DatabaseName).Scan(&metadata.TableCount); err != nil {
		return metadata, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_schema_migrations`).Scan(&metadata.MigrationCount); err != nil {
		return metadata, err
	}
	err := s.db.QueryRowContext(ctx, `SELECT version FROM mochat_go_schema_migrations ORDER BY applied_at DESC, version DESC LIMIT 1`).Scan(&metadata.MigrationVersion)
	return metadata, err
}

func (s *MySQLStore) StartBackupRun(ctx context.Context, input saasbackup.BackupStart) (saasbackup.BackupRun, error) {
	if input.ReplicaStatus != saasbackup.ReplicaStatusDisabled && input.ReplicaStatus != saasbackup.ReplicaStatusPending {
		return saasbackup.BackupRun{}, saasbackup.Invalid("备份异地副本初始状态无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	defer rollbackQuietly(tx)
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_runs
		SET status = 'failed', active_slot = NULL, error_message = 'backup execution lease expired',
			finished_at = NOW(), version = version + 1, updated_at = NOW()
		WHERE status = 'running' AND active_slot = 1 AND started_at <= DATE_SUB(NOW(), INTERVAL 24 HOUR)
	`); err != nil {
		return saasbackup.BackupRun{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_backup_runs
			(backup_no, trigger_type, status, active_slot, artifact_name, artifact_format, encrypted, encryption_key_id,
			 sha256, size_bytes, replica_status, replica_provider, database_name, migration_version, migration_count, table_count, verification_status,
			 verified_at, verification_error, error_message, started_at, finished_at, created_by, actor_tenant_id,
			 operation_id, version, created_at, updated_at, deleted_at)
		VALUES (?, ?, 'running', 1, ?, ?, ?, ?, '', 0, ?, ?, ?, ?, ?, ?, 'pending', NULL, '', '', ?, NULL, ?, ?, 0, 1, NOW(), NOW(), NULL)
	`, input.BackupNo, input.TriggerType, input.ArtifactName, input.ArtifactFormat, boolInt(input.Encrypted), input.EncryptionKeyID,
		input.ReplicaStatus, input.ReplicaProvider, input.Metadata.DatabaseName, input.Metadata.MigrationVersion,
		input.Metadata.MigrationCount, input.Metadata.TableCount,
		input.StartedAt, input.Actor.UserID, input.Actor.TenantID)
	if isMySQLDuplicateKeyError(err) {
		return saasbackup.BackupRun{}, saasbackup.Conflict("已有备份正在执行，请稍后重试")
	}
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	id, _ := result.LastInsertId()
	if err := tx.Commit(); err != nil {
		return saasbackup.BackupRun{}, err
	}
	return s.BackupRun(ctx, id)
}

func (s *MySQLStore) CompleteBackupRun(ctx context.Context, input saasbackup.BackupCompletion) (saasbackup.BackupRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	defer rollbackQuietly(tx)
	before, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ? FOR UPDATE`, input.RunID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.BackupRun{}, saasbackup.NotFound("备份运行不存在")
	}
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	if before.Status != saasbackup.RunStatusRunning {
		return before, saasbackup.Conflict("备份运行已经结束")
	}
	if input.Status != saasbackup.RunStatusSucceeded && input.Status != saasbackup.RunStatusFailed {
		return before, saasbackup.Invalid("备份完成状态无效")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_runs
		SET status = ?, active_slot = NULL, sha256 = ?, size_bytes = ?, error_message = ?, finished_at = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND status = 'running' AND active_slot = 1
	`, input.Status, input.SHA256, input.SizeBytes, truncateRunes(input.ErrorMessage, 1000), input.FinishedAt, input.RunID)
	if err != nil {
		return before, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return before, saasbackup.Conflict("备份运行状态已变化")
	}
	after, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ?`, input.RunID))
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.Actor.UserID, ActorTenantID: input.Actor.TenantID,
		Action: saasbackup.OperationActionCreate, TargetType: "saas_backup_run", TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.BackupNo,
		BeforeJSON: saasBackupRunAuditJSON(before), AfterJSON: saasBackupRunAuditJSON(after), Remark: "complete SaaS database backup",
	})
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_backup_runs SET operation_id = ? WHERE id = ?`, operationID, after.ID); err != nil {
		return saasbackup.BackupRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.BackupRun{}, err
	}
	return s.BackupRun(ctx, after.ID)
}

func (s *MySQLStore) RecordBackupVerification(ctx context.Context, input saasbackup.BackupVerification) (saasbackup.BackupRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	defer rollbackQuietly(tx)
	before, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ? FOR UPDATE`, input.RunID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.BackupRun{}, saasbackup.NotFound("备份运行不存在")
	}
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	if before.Status != saasbackup.RunStatusSucceeded {
		return before, saasbackup.Conflict("只有成功备份可以记录校验结果")
	}
	if before.CleanupRunID > 0 {
		return before, saasbackup.Conflict("备份已绑定保留清理任务，不能记录新的校验结果")
	}
	if input.Status != saasbackup.VerificationPassed && input.Status != saasbackup.VerificationFailed {
		return before, saasbackup.Invalid("备份校验状态无效")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_runs
		SET verification_status = ?, verified_at = ?, verification_error = ?, version = version + 1, updated_at = NOW()
		WHERE id = ?
	`, input.Status, input.VerifiedAt, truncateRunes(input.ErrorMessage, 1000), input.RunID); err != nil {
		return before, err
	}
	after, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ?`, input.RunID))
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.Actor.UserID, ActorTenantID: input.Actor.TenantID,
		Action: saasbackup.OperationActionVerify, TargetType: "saas_backup_run", TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.BackupNo,
		BeforeJSON: saasBackupRunAuditJSON(before), AfterJSON: saasBackupRunAuditJSON(after), Remark: "verify SaaS database backup",
	})
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_backup_runs SET operation_id = ? WHERE id = ?`, operationID, after.ID); err != nil {
		return saasbackup.BackupRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.BackupRun{}, err
	}
	return s.BackupRun(ctx, after.ID)
}

func (s *MySQLStore) StartBackupReplica(ctx context.Context, input saasbackup.BackupReplicaStart) (saasbackup.BackupRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	defer rollbackQuietly(tx)
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_runs
		SET replica_status = 'failed', replica_error = 'offsite replication lease expired',
			version = version + 1, updated_at = NOW()
		WHERE replica_status = 'uploading' AND updated_at <= DATE_SUB(NOW(), INTERVAL 24 HOUR)
	`); err != nil {
		return saasbackup.BackupRun{}, err
	}
	before, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ? FOR UPDATE`, input.RunID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.BackupRun{}, saasbackup.NotFound("备份运行不存在")
	}
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	if before.Status != saasbackup.RunStatusSucceeded {
		return before, saasbackup.Conflict("只有成功备份可以复制到异地对象存储")
	}
	if before.CleanupRunID > 0 {
		return before, saasbackup.Conflict("备份已绑定保留清理任务，不能复制异地副本")
	}
	if before.ReplicaStatus == saasbackup.ReplicaStatusUploading {
		return before, saasbackup.Conflict("异地副本正在上传，请稍后重试")
	}
	if strings.TrimSpace(input.Provider) == "" || strings.TrimSpace(input.Bucket) == "" || strings.TrimSpace(input.ObjectKey) == "" {
		return before, saasbackup.Invalid("异地副本提供方、存储桶和对象键不能为空")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_runs
		SET replica_status = 'uploading', replica_provider = ?, replica_bucket = ?, replica_object_key = ?,
			replica_etag = '', replica_version_id = '', replica_sha256 = '', replica_size_bytes = 0,
			replicated_at = NULL, replica_verified_at = NULL, replica_error = '', version = version + 1, updated_at = ?
		WHERE id = ? AND replica_status <> 'uploading'
	`, strings.TrimSpace(input.Provider), strings.TrimSpace(input.Bucket), strings.TrimSpace(input.ObjectKey), input.StartedAt, input.RunID); err != nil {
		return before, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.BackupRun{}, err
	}
	return s.BackupRun(ctx, input.RunID)
}

func (s *MySQLStore) CompleteBackupReplica(ctx context.Context, input saasbackup.BackupReplicaCompletion) (saasbackup.BackupRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	defer rollbackQuietly(tx)
	before, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ? FOR UPDATE`, input.RunID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.BackupRun{}, saasbackup.NotFound("备份运行不存在")
	}
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	if before.ReplicaStatus != saasbackup.ReplicaStatusUploading {
		return before, saasbackup.Conflict("异地副本上传状态已变化")
	}
	if before.CleanupRunID > 0 {
		return before, saasbackup.Conflict("备份已绑定保留清理任务，不能完成异地副本上传")
	}
	if input.Status != saasbackup.ReplicaStatusSucceeded && input.Status != saasbackup.ReplicaStatusFailed {
		return before, saasbackup.Invalid("异地副本完成状态无效")
	}
	if input.Status == saasbackup.ReplicaStatusSucceeded && (input.SizeBytes <= 0 || len(strings.TrimSpace(input.SHA256)) != 64) {
		return before, saasbackup.Invalid("异地副本完整性元数据无效")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_runs
		SET replica_status = ?, replica_etag = ?, replica_version_id = ?, replica_sha256 = ?, replica_size_bytes = ?,
			replicated_at = ?, replica_verified_at = ?, replica_error = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND replica_status = 'uploading'
	`, input.Status, strings.TrimSpace(input.ETag), strings.TrimSpace(input.VersionID), strings.ToLower(strings.TrimSpace(input.SHA256)),
		input.SizeBytes, nullableBackupTime(input.ReplicatedAt), nullableBackupTime(input.VerifiedAt),
		truncateRunes(input.ErrorMessage, 1000), input.RunID); err != nil {
		return before, err
	}
	after, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ?`, input.RunID))
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.Actor.UserID, ActorTenantID: input.Actor.TenantID,
		Action: saasbackup.OperationActionReplicate, TargetType: "saas_backup_run", TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.BackupNo,
		BeforeJSON: saasBackupRunAuditJSON(before), AfterJSON: saasBackupRunAuditJSON(after), Remark: "replicate SaaS database backup to offsite object storage",
	})
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_backup_runs SET operation_id = ? WHERE id = ?`, operationID, after.ID); err != nil {
		return saasbackup.BackupRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.BackupRun{}, err
	}
	return s.BackupRun(ctx, after.ID)
}

func (s *MySQLStore) BackupRun(ctx context.Context, id int64) (saasbackup.BackupRun, error) {
	item, err := scanSaaSBackupRun(s.db.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return item, saasbackup.NotFound("备份运行不存在")
	}
	return item, err
}

func (s *MySQLStore) BackupRuns(ctx context.Context, limit int) ([]saasbackup.BackupRun, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, saasBackupRunSelect+` ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saasbackup.BackupRun, 0)
	for rows.Next() {
		item, err := scanSaaSBackupRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) StartRestoreDrill(ctx context.Context, input saasbackup.RestoreDrillStart) (saasbackup.RestoreDrill, error) {
	if input.TargetLifecycle != saasbackup.RestoreTargetLifecyclePreconfigured && input.TargetLifecycle != saasbackup.RestoreTargetLifecycleEphemeral {
		return saasbackup.RestoreDrill{}, saasbackup.Invalid("恢复目标生命周期无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	defer rollbackQuietly(tx)
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_restore_drills
		SET status = 'failed', active_slot = NULL, error_message = 'restore drill execution lease expired',
			finished_at = NOW(), duration_ms = TIMESTAMPDIFF(MICROSECOND, started_at, NOW()) DIV 1000
		WHERE status = 'running' AND active_slot = 1 AND started_at <= DATE_SUB(NOW(), INTERVAL 24 HOUR)
	`); err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	backupRun, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ? FOR UPDATE`, input.BackupRun.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.RestoreDrill{}, saasbackup.NotFound("备份运行不存在")
	}
	if err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	if backupRun.Version != input.BackupRun.Version || backupRun.Status != saasbackup.RunStatusSucceeded || backupRun.CleanupRunID > 0 {
		return saasbackup.RestoreDrill{}, saasbackup.Conflict("备份运行已变化或已进入保留清理任务")
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_restore_drills
			(drill_no, backup_run_id, trigger_type, status, active_slot, target_fingerprint, target_database,
			 target_lifecycle, target_cleanup_status,
			 expected_migration_version, actual_migration_version, expected_migration_count, actual_migration_count,
			 expected_table_count, actual_table_count, checks_json, error_message, started_at, finished_at, duration_ms,
			 created_by, actor_tenant_id, operation_id, created_at)
		VALUES (?, ?, ?, 'running', 1, ?, ?, ?, ?, ?, '', ?, 0, ?, 0, NULL, '', ?, NULL, 0, ?, ?, 0, NOW())
	`, input.DrillNo, input.BackupRun.ID, input.TriggerType, input.TargetFingerprint, input.TargetDatabase,
		input.TargetLifecycle, input.TargetCleanupStatus, input.BackupRun.MigrationVersion,
		input.BackupRun.MigrationCount, input.BackupRun.TableCount, input.StartedAt,
		input.Actor.UserID, input.Actor.TenantID)
	if isMySQLDuplicateKeyError(err) {
		return saasbackup.RestoreDrill{}, saasbackup.Conflict("已有恢复演练正在执行，请稍后重试")
	}
	if err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	id, _ := result.LastInsertId()
	if err := tx.Commit(); err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	return s.restoreDrill(ctx, id)
}

func (s *MySQLStore) CompleteRestoreDrill(ctx context.Context, input saasbackup.RestoreDrillCompletion) (saasbackup.RestoreDrill, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	defer rollbackQuietly(tx)
	before, err := scanSaaSRestoreDrill(tx.QueryRowContext(ctx, saasRestoreDrillSelect+` WHERE id = ? FOR UPDATE`, input.DrillID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.RestoreDrill{}, saasbackup.NotFound("恢复演练不存在")
	}
	if err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	if before.Status != saasbackup.DrillStatusRunning {
		return before, saasbackup.Conflict("恢复演练已经结束")
	}
	if input.Status != saasbackup.DrillStatusSucceeded && input.Status != saasbackup.DrillStatusFailed {
		return before, saasbackup.Invalid("恢复演练完成状态无效")
	}
	switch input.TargetCleanupStatus {
	case saasbackup.RestoreCleanupNotRequired, saasbackup.RestoreCleanupSucceeded,
		saasbackup.RestoreCleanupFailed, saasbackup.RestoreCleanupRetained:
	default:
		return before, saasbackup.Invalid("恢复目标清理状态无效")
	}
	checks, _ := json.Marshal(input.Checks)
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_restore_drills
		SET status = ?, active_slot = NULL, actual_migration_version = ?, actual_migration_count = ?, actual_table_count = ?,
			checks_json = ?, error_message = ?, finished_at = ?, duration_ms = ?, target_cleanup_status = ?,
			target_cleaned_at = ?, target_cleanup_error = ?
		WHERE id = ? AND status = 'running' AND active_slot = 1
	`, input.Status, input.ActualMigrationVersion, input.ActualMigrationCount, input.ActualTableCount, nullableJSON(checks),
		truncateRunes(input.ErrorMessage, 1000), input.FinishedAt, input.DurationMS, input.TargetCleanupStatus,
		nullableBackupTime(input.TargetCleanedAt), truncateRunes(input.TargetCleanupError, 1000), input.DrillID)
	if err != nil {
		return before, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return before, saasbackup.Conflict("恢复演练状态已变化")
	}
	after, err := scanSaaSRestoreDrill(tx.QueryRowContext(ctx, saasRestoreDrillSelect+` WHERE id = ?`, input.DrillID))
	if err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.Actor.UserID, ActorTenantID: input.Actor.TenantID,
		Action: saasbackup.OperationActionRestoreDrill, TargetType: "saas_restore_drill", TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.DrillNo,
		BeforeJSON: saasRestoreDrillAuditJSON(before), AfterJSON: saasRestoreDrillAuditJSON(after), Remark: "complete isolated SaaS restore drill",
	})
	if err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_restore_drills SET operation_id = ? WHERE id = ?`, operationID, after.ID); err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.RestoreDrill{}, err
	}
	return s.restoreDrill(ctx, after.ID)
}

func (s *MySQLStore) RestoreDrills(ctx context.Context, limit int) ([]saasbackup.RestoreDrill, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, saasRestoreDrillSelect+` ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saasbackup.RestoreDrill, 0)
	for rows.Next() {
		item, err := scanSaaSRestoreDrill(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) MarkBackupDeleted(ctx context.Context, runID int64, actor saasbackup.Actor) (saasbackup.BackupRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	defer rollbackQuietly(tx)
	before, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ? FOR UPDATE`, runID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.BackupRun{}, saasbackup.NotFound("备份运行不存在")
	}
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	if before.Status == saasbackup.RunStatusRunning {
		return before, saasbackup.Conflict("运行中的备份不能删除")
	}
	if before.CleanupRunID > 0 {
		return before, saasbackup.Conflict("备份已绑定保留清理任务，必须由该任务完成删除")
	}
	if before.Status == saasbackup.RunStatusDeleted {
		return before, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_runs
		SET status = 'deleted', replica_status = IF(replica_object_key = '', replica_status, 'deleted'),
			deleted_at = NOW(), version = version + 1, updated_at = NOW() WHERE id = ?
	`, runID); err != nil {
		return before, err
	}
	after, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ?`, runID))
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: actor.UserID, ActorTenantID: actor.TenantID,
		Action: saasbackup.OperationActionDelete, TargetType: "saas_backup_run", TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.BackupNo,
		BeforeJSON: saasBackupRunAuditJSON(before), AfterJSON: saasBackupRunAuditJSON(after), Remark: "delete expired SaaS backup artifact",
	})
	if err != nil {
		return saasbackup.BackupRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_backup_runs SET operation_id = ? WHERE id = ?`, operationID, runID); err != nil {
		return saasbackup.BackupRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.BackupRun{}, err
	}
	return s.BackupRun(ctx, runID)
}

func (s *MySQLStore) restoreDrill(ctx context.Context, id int64) (saasbackup.RestoreDrill, error) {
	item, err := scanSaaSRestoreDrill(s.db.QueryRowContext(ctx, saasRestoreDrillSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return item, saasbackup.NotFound("恢复演练不存在")
	}
	return item, err
}

const saasBackupPolicySelect = `
	SELECT id, status, interval_minutes, retention_days, min_successful_backups, max_backup_age_minutes,
		restore_drill_interval_days, require_encryption, require_offsite_replica, version, updated_by, created_at, updated_at
	FROM mochat_go_saas_backup_policies
`

const saasBackupRunSelect = `
	SELECT id, backup_no, trigger_type, status, artifact_name, artifact_format, encrypted, encryption_key_id,
		sha256, size_bytes, replica_status, replica_provider, replica_bucket, replica_object_key, replica_etag,
		replica_version_id, replica_sha256, replica_size_bytes, replicated_at, replica_verified_at, replica_error,
		database_name, migration_version, migration_count, table_count, verification_status,
		verified_at, verification_error, error_message, started_at, finished_at, created_by, actor_tenant_id,
		operation_id, cleanup_run_id, version, created_at, updated_at, deleted_at
	FROM mochat_go_saas_backup_runs
`

const saasRestoreDrillSelect = `
	SELECT id, drill_no, backup_run_id, trigger_type, status, target_fingerprint, target_database,
		target_lifecycle, target_cleanup_status, target_cleaned_at, target_cleanup_error,
		expected_migration_version, actual_migration_version, expected_migration_count, actual_migration_count,
		expected_table_count, actual_table_count, COALESCE(CAST(checks_json AS CHAR), ''), error_message,
		started_at, finished_at, duration_ms, created_by, actor_tenant_id, operation_id, created_at
	FROM mochat_go_saas_restore_drills
`

func scanSaaSBackupPolicy(scanner interface{ Scan(...any) error }) (saasbackup.Policy, error) {
	var item saasbackup.Policy
	var encrypted, requireReplica int
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Status, &item.IntervalMinutes, &item.RetentionDays, &item.MinSuccessfulBackups,
		&item.MaxBackupAgeMinutes, &item.RestoreDrillIntervalDays, &encrypted, &requireReplica,
		&item.Version, &item.UpdatedBy, &createdAt, &updatedAt)
	item.RequireEncryption = encrypted == 1
	item.RequireOffsiteReplica = requireReplica == 1
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, err
}

func scanSaaSBackupRun(scanner interface{ Scan(...any) error }) (saasbackup.BackupRun, error) {
	var item saasbackup.BackupRun
	var encrypted int
	var cleanupRunID sql.NullInt64
	var replicatedAt, replicaVerifiedAt, verifiedAt, startedAt, finishedAt, createdAt, updatedAt, deletedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.BackupNo, &item.TriggerType, &item.Status, &item.ArtifactName, &item.ArtifactFormat,
		&encrypted, &item.EncryptionKeyID, &item.SHA256, &item.SizeBytes, &item.ReplicaStatus, &item.ReplicaProvider,
		&item.ReplicaBucket, &item.ReplicaObjectKey, &item.ReplicaETag, &item.ReplicaVersionID, &item.ReplicaSHA256,
		&item.ReplicaSizeBytes, &replicatedAt, &replicaVerifiedAt, &item.ReplicaError, &item.DatabaseName, &item.MigrationVersion,
		&item.MigrationCount, &item.TableCount, &item.VerificationStatus, &verifiedAt, &item.VerificationError,
		&item.ErrorMessage, &startedAt, &finishedAt, &item.CreatedBy, &item.ActorTenantID, &item.OperationID, &cleanupRunID,
		&item.Version, &createdAt, &updatedAt, &deletedAt)
	item.Encrypted = encrypted == 1
	if cleanupRunID.Valid {
		item.CleanupRunID = cleanupRunID.Int64
	}
	item.ReplicatedAt, item.ReplicaVerifiedAt = formatTime(replicatedAt), formatTime(replicaVerifiedAt)
	item.VerifiedAt, item.StartedAt, item.FinishedAt = formatTime(verifiedAt), formatTime(startedAt), formatTime(finishedAt)
	item.CreatedAt, item.UpdatedAt, item.DeletedAt = formatTime(createdAt), formatTime(updatedAt), formatTime(deletedAt)
	return item, err
}

func scanSaaSRestoreDrill(scanner interface{ Scan(...any) error }) (saasbackup.RestoreDrill, error) {
	var item saasbackup.RestoreDrill
	var targetCleanedAt, startedAt, finishedAt, createdAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.DrillNo, &item.BackupRunID, &item.TriggerType, &item.Status,
		&item.TargetFingerprint, &item.TargetDatabase, &item.TargetLifecycle, &item.TargetCleanupStatus,
		&targetCleanedAt, &item.TargetCleanupError, &item.ExpectedMigrationVersion, &item.ActualMigrationVersion,
		&item.ExpectedMigrationCount, &item.ActualMigrationCount, &item.ExpectedTableCount, &item.ActualTableCount,
		&item.ChecksJSON, &item.ErrorMessage, &startedAt, &finishedAt, &item.DurationMS, &item.CreatedBy,
		&item.ActorTenantID, &item.OperationID, &createdAt)
	item.TargetCleanedAt = formatTime(targetCleanedAt)
	item.StartedAt, item.FinishedAt, item.CreatedAt = formatTime(startedAt), formatTime(finishedAt), formatTime(createdAt)
	return item, err
}

func saasBackupPolicyAuditJSON(item saasbackup.Policy) string {
	return saasBackupMarshal(map[string]any{
		"status": item.Status, "intervalMinutes": item.IntervalMinutes, "retentionDays": item.RetentionDays,
		"minSuccessfulBackups": item.MinSuccessfulBackups, "maxBackupAgeMinutes": item.MaxBackupAgeMinutes,
		"restoreDrillIntervalDays": item.RestoreDrillIntervalDays, "requireEncryption": item.RequireEncryption,
		"requireOffsiteReplica": item.RequireOffsiteReplica,
		"version":               item.Version,
	})
}

func saasBackupRunAuditJSON(item saasbackup.BackupRun) string {
	return saasBackupMarshal(map[string]any{
		"id": item.ID, "backupNo": item.BackupNo, "triggerType": item.TriggerType, "status": item.Status,
		"artifactName": item.ArtifactName, "artifactFormat": item.ArtifactFormat, "encrypted": item.Encrypted,
		"encryptionKeyId": item.EncryptionKeyID, "sha256": item.SHA256, "sizeBytes": item.SizeBytes,
		"replicaStatus": item.ReplicaStatus, "replicaProvider": item.ReplicaProvider,
		"replicaBucket": item.ReplicaBucket, "replicaObjectKey": item.ReplicaObjectKey,
		"replicaETag": item.ReplicaETag, "replicaVersionId": item.ReplicaVersionID,
		"replicaSHA256": item.ReplicaSHA256, "replicaSizeBytes": item.ReplicaSizeBytes,
		"migrationVersion": item.MigrationVersion, "migrationCount": item.MigrationCount, "tableCount": item.TableCount,
		"verificationStatus": item.VerificationStatus, "errorMessage": item.ErrorMessage,
		"cleanupRunId": item.CleanupRunID, "version": item.Version,
	})
}

func saasRestoreDrillAuditJSON(item saasbackup.RestoreDrill) string {
	return saasBackupMarshal(map[string]any{
		"id": item.ID, "drillNo": item.DrillNo, "backupRunId": item.BackupRunID, "triggerType": item.TriggerType,
		"status": item.Status, "targetFingerprint": item.TargetFingerprint, "targetDatabase": item.TargetDatabase,
		"targetLifecycle": item.TargetLifecycle, "targetCleanupStatus": item.TargetCleanupStatus,
		"targetCleanedAt": item.TargetCleanedAt, "targetCleanupError": item.TargetCleanupError,
		"expectedMigrationVersion": item.ExpectedMigrationVersion, "actualMigrationVersion": item.ActualMigrationVersion,
		"expectedMigrationCount": item.ExpectedMigrationCount, "actualMigrationCount": item.ActualMigrationCount,
		"expectedTableCount": item.ExpectedTableCount, "actualTableCount": item.ActualTableCount,
		"durationMs": item.DurationMS, "errorMessage": item.ErrorMessage,
	})
}

func saasBackupMarshal(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func nullableJSON(value []byte) any {
	if len(value) == 0 || string(value) == "null" || string(value) == "{}" {
		return nil
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableBackupTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

var _ saasbackup.Store = (*MySQLStore)(nil)
