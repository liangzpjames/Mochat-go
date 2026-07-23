package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/saasbackup"
)

const saasBackupCleanupRunSelect = `
	SELECT id, cleanup_no, status, active_slot, policy_version, cutoff_at, scanned_count, candidate_count,
		preserved_count, deleted_count, failed_count, replicas_deleted_count, missing_files_count, attempts,
		approval_id, lease_expires_at, started_at, finished_at, last_error, created_by, actor_tenant_id,
		operation_id, version, created_at, updated_at
	FROM mochat_go_saas_backup_cleanup_runs
`

const saasBackupCleanupItemSelect = `
	SELECT id, cleanup_run_id, backup_run_id, backup_run_version, backup_no, artifact_name,
		replica_object_key, replica_version_id, status, replica_status, local_status, record_status,
		attempts, last_error, replica_deleted_at, local_deleted_at, record_deleted_at, operation_id,
		created_at, updated_at
	FROM mochat_go_saas_backup_cleanup_items
`

func (s *MySQLStore) ScheduleBackupCleanup(ctx context.Context, input saasbackup.CleanupSchedule) (saasbackup.CleanupRun, error) {
	if strings.TrimSpace(input.CleanupNo) == "" || input.Plan.PolicyVersion <= 0 || input.Plan.CutoffAt.IsZero() || len(input.Plan.Items) == 0 {
		return saasbackup.CleanupRun{}, saasbackup.Invalid("备份清理计划无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.CleanupRun{}, err
	}
	defer rollbackQuietly(tx)

	policy, err := scanSaaSBackupPolicy(tx.QueryRowContext(ctx, saasBackupPolicySelect+` WHERE id = 1 FOR UPDATE`))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.CleanupRun{}, saasbackup.NotFound("备份策略不存在")
	}
	if err != nil {
		return saasbackup.CleanupRun{}, err
	}
	if policy.Version != input.Plan.PolicyVersion {
		return saasbackup.CleanupRun{}, saasbackup.Conflict("备份策略版本已变化，请重新提交清理审批")
	}

	planItems := append([]saasbackup.CleanupPlanItem(nil), input.Plan.Items...)
	sort.Slice(planItems, func(i, j int) bool { return planItems[i].BackupRunID < planItems[j].BackupRunID })
	lockedRuns := make([]saasbackup.BackupRun, 0, len(planItems))
	for _, planned := range planItems {
		run, scanErr := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ? FOR UPDATE`, planned.BackupRunID))
		if errors.Is(scanErr, sql.ErrNoRows) {
			return saasbackup.CleanupRun{}, saasbackup.Conflict("清理候选备份已不存在，请重新提交审批")
		}
		if scanErr != nil {
			return saasbackup.CleanupRun{}, scanErr
		}
		if !sameBackupCleanupSnapshot(run, planned) || run.CleanupRunID != 0 {
			return saasbackup.CleanupRun{}, saasbackup.Conflict("清理候选备份已变化，请重新提交审批")
		}
		lockedRuns = append(lockedRuns, run)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_backup_cleanup_runs
			(cleanup_no, status, active_slot, policy_version, cutoff_at, scanned_count, candidate_count,
			 preserved_count, deleted_count, failed_count, replicas_deleted_count, missing_files_count,
			 attempts, approval_id, lease_expires_at, started_at, finished_at, last_error, created_by,
			 actor_tenant_id, operation_id, version, created_at, updated_at)
		VALUES (?, 'pending', 1, ?, ?, ?, ?, ?, 0, 0, 0, 0, 0, ?, NULL, NULL, NULL, '', ?, ?, 0, 1, NOW(), NOW())
	`, strings.TrimSpace(input.CleanupNo), input.Plan.PolicyVersion, input.Plan.CutoffAt, input.Plan.Scanned,
		len(planItems), input.Plan.Preserved, nullablePositiveInt64(input.ApprovalExecutionID), input.Actor.UserID, input.Actor.TenantID)
	if isMySQLDuplicateKeyError(err) {
		return saasbackup.CleanupRun{}, saasbackup.Conflict("已有备份清理任务待执行，请先完成或处理该任务")
	}
	if err != nil {
		return saasbackup.CleanupRun{}, err
	}
	cleanupRunID, _ := result.LastInsertId()

	for index, run := range lockedRuns {
		boundVersion := run.Version + 1
		update, updateErr := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_backup_runs
			SET cleanup_run_id = ?, version = version + 1, updated_at = NOW()
			WHERE id = ? AND version = ? AND cleanup_run_id IS NULL
		`, cleanupRunID, run.ID, run.Version)
		if updateErr != nil {
			return saasbackup.CleanupRun{}, updateErr
		}
		if affected, _ := update.RowsAffected(); affected != 1 {
			return saasbackup.CleanupRun{}, saasbackup.Conflict("清理候选备份已变化，请重新提交审批")
		}
		replicaStatus := saasbackup.CleanupStepPending
		if strings.TrimSpace(run.ReplicaObjectKey) == "" || run.ReplicaStatus == saasbackup.ReplicaStatusDeleted {
			replicaStatus = saasbackup.CleanupStepNotRequired
		}
		localStatus := saasbackup.CleanupStepPending
		if strings.TrimSpace(run.ArtifactName) == "" {
			localStatus = saasbackup.CleanupStepNotRequired
		}
		planned := planItems[index]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_backup_cleanup_items
				(cleanup_run_id, backup_run_id, backup_run_version, backup_no, artifact_name,
				 replica_object_key, replica_version_id, status, replica_status, local_status,
				 record_status, attempts, last_error, operation_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?, 'pending', 0, '', 0, NOW(), NOW())
		`, cleanupRunID, run.ID, boundVersion, planned.BackupNo, planned.ArtifactName,
			planned.ReplicaObjectKey, planned.ReplicaVersionID, replicaStatus, localStatus); err != nil {
			return saasbackup.CleanupRun{}, err
		}
	}

	created, err := scanSaaSBackupCleanupRun(tx.QueryRowContext(ctx, saasBackupCleanupRunSelect+` WHERE id = ?`, cleanupRunID))
	if err != nil {
		return saasbackup.CleanupRun{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.Actor.UserID, ActorTenantID: input.Actor.TenantID,
		Action: saasbackup.OperationActionCleanupSchedule, TargetType: "saas_backup_cleanup_run",
		TargetID: strconv.FormatInt(cleanupRunID, 10), TargetName: created.CleanupNo,
		BeforeJSON: "{}", AfterJSON: saasBackupCleanupRunAuditJSON(created), Remark: "schedule approved SaaS backup retention cleanup",
	})
	if err != nil {
		return saasbackup.CleanupRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_backup_cleanup_runs SET operation_id = ? WHERE id = ?`, operationID, cleanupRunID); err != nil {
		return saasbackup.CleanupRun{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.Actor.UserID, operationID); err != nil {
		return saasbackup.CleanupRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.CleanupRun{}, err
	}
	return s.BackupCleanupRun(ctx, cleanupRunID)
}

func (s *MySQLStore) BackupCleanupRun(ctx context.Context, id int64) (saasbackup.CleanupRun, error) {
	item, err := scanSaaSBackupCleanupRun(s.db.QueryRowContext(ctx, saasBackupCleanupRunSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return item, saasbackup.NotFound("备份清理任务不存在")
	}
	return item, err
}

func (s *MySQLStore) BackupCleanupRuns(ctx context.Context, limit int) ([]saasbackup.CleanupRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, saasBackupCleanupRunSelect+` ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saasbackup.CleanupRun, 0)
	for rows.Next() {
		item, scanErr := scanSaaSBackupCleanupRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) BackupCleanupItems(ctx context.Context, cleanupRunID int64) ([]saasbackup.CleanupItem, error) {
	rows, err := s.db.QueryContext(ctx, saasBackupCleanupItemSelect+` WHERE cleanup_run_id = ? ORDER BY id ASC`, cleanupRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saasbackup.CleanupItem, 0)
	for rows.Next() {
		item, scanErr := scanSaaSBackupCleanupItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) ClaimBackupCleanup(ctx context.Context, cleanupRunID int64, retry bool, actor saasbackup.Actor) (saasbackup.CleanupRun, []saasbackup.CleanupItem, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.CleanupRun{}, nil, err
	}
	defer rollbackQuietly(tx)
	run, err := claimBackupCleanupTx(ctx, tx, cleanupRunID, retry, actor)
	if err != nil {
		return saasbackup.CleanupRun{}, nil, err
	}
	items, err := backupCleanupItemsTx(ctx, tx, run.ID)
	if err != nil {
		return saasbackup.CleanupRun{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.CleanupRun{}, nil, err
	}
	return run, items, nil
}

func (s *MySQLStore) ClaimNextBackupCleanup(ctx context.Context, actor saasbackup.Actor) (saasbackup.CleanupRun, []saasbackup.CleanupItem, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.CleanupRun{}, nil, false, err
	}
	defer rollbackQuietly(tx)
	var id int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM mochat_go_saas_backup_cleanup_runs
		WHERE active_slot = 1 AND (status = 'pending' OR (status = 'running' AND lease_expires_at <= NOW()))
		ORDER BY id ASC LIMIT 1 FOR UPDATE
	`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return saasbackup.CleanupRun{}, nil, false, err
		}
		return saasbackup.CleanupRun{}, []saasbackup.CleanupItem{}, false, nil
	}
	if err != nil {
		return saasbackup.CleanupRun{}, nil, false, err
	}
	run, err := claimBackupCleanupTx(ctx, tx, id, false, actor)
	if err != nil {
		return saasbackup.CleanupRun{}, nil, false, err
	}
	items, err := backupCleanupItemsTx(ctx, tx, run.ID)
	if err != nil {
		return saasbackup.CleanupRun{}, nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.CleanupRun{}, nil, false, err
	}
	return run, items, true, nil
}

func (s *MySQLStore) CheckpointBackupCleanupItem(ctx context.Context, input saasbackup.CleanupCheckpoint) (saasbackup.CleanupItem, error) {
	if input.Step != saasbackup.CleanupStepReplica && input.Step != saasbackup.CleanupStepLocal {
		return saasbackup.CleanupItem{}, saasbackup.Invalid("备份清理步骤无效")
	}
	if input.Status != saasbackup.CleanupStepDeleted && input.Status != saasbackup.CleanupStepMissing &&
		input.Status != saasbackup.CleanupStepNotRequired && input.Status != saasbackup.CleanupStepFailed {
		return saasbackup.CleanupItem{}, saasbackup.Invalid("备份清理步骤状态无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	defer rollbackQuietly(tx)
	run, err := lockBackupCleanupExecution(ctx, tx, input.CleanupRunID, input.CleanupAttempt)
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	item, err := scanSaaSBackupCleanupItem(tx.QueryRowContext(ctx, saasBackupCleanupItemSelect+` WHERE id = ? AND cleanup_run_id = ? FOR UPDATE`, input.ItemID, run.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.CleanupItem{}, saasbackup.NotFound("备份清理步骤不存在")
	}
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	itemStatus := saasbackup.CleanupItemStatusRunning
	lastError := ""
	if input.Status == saasbackup.CleanupStepFailed {
		itemStatus = saasbackup.CleanupItemStatusFailed
		lastError = truncateRunes(input.ErrorMessage, 1000)
	}
	column := "replica_status"
	timeColumn := "replica_deleted_at"
	if input.Step == saasbackup.CleanupStepLocal {
		column, timeColumn = "local_status", "local_deleted_at"
	}
	query := fmt.Sprintf(`
		UPDATE mochat_go_saas_backup_cleanup_items
		SET %s = ?, %s = IF(? IN ('deleted', 'missing'), NOW(), %s), status = ?,
			attempts = attempts + IF(status IN ('pending', 'failed'), 1, 0), last_error = ?, updated_at = NOW()
		WHERE id = ? AND cleanup_run_id = ?
	`, column, timeColumn, timeColumn)
	if _, err := tx.ExecContext(ctx, query, input.Status, input.Status, itemStatus, lastError, item.ID, run.ID); err != nil {
		return saasbackup.CleanupItem{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_backup_cleanup_runs SET lease_expires_at = DATE_ADD(NOW(), INTERVAL 5 MINUTE), last_error = ?, updated_at = NOW() WHERE id = ?`, lastError, run.ID); err != nil {
		return saasbackup.CleanupItem{}, err
	}
	updated, err := scanSaaSBackupCleanupItem(tx.QueryRowContext(ctx, saasBackupCleanupItemSelect+` WHERE id = ?`, item.ID))
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.CleanupItem{}, err
	}
	return updated, nil
}

func (s *MySQLStore) CompleteBackupCleanupItem(ctx context.Context, input saasbackup.CleanupItemCompletion) (saasbackup.CleanupItem, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	defer rollbackQuietly(tx)
	run, err := lockBackupCleanupExecution(ctx, tx, input.CleanupRunID, input.CleanupAttempt)
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	item, err := scanSaaSBackupCleanupItem(tx.QueryRowContext(ctx, saasBackupCleanupItemSelect+` WHERE id = ? AND cleanup_run_id = ? FOR UPDATE`, input.ItemID, run.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.CleanupItem{}, saasbackup.NotFound("备份清理步骤不存在")
	}
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	if item.Status == saasbackup.CleanupItemStatusSucceeded {
		if err := tx.Commit(); err != nil {
			return saasbackup.CleanupItem{}, err
		}
		return item, nil
	}
	if !cleanupStepComplete(item.ReplicaStatus) || !cleanupStepComplete(item.LocalStatus) {
		return saasbackup.CleanupItem{}, saasbackup.Conflict("备份清理的外部删除步骤尚未完成")
	}
	before, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ? FOR UPDATE`, item.BackupRunID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.CleanupItem{}, saasbackup.NotFound("备份运行不存在")
	}
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	if before.CleanupRunID != run.ID || before.Version != item.BackupRunVersion || before.Status == saasbackup.RunStatusRunning {
		return saasbackup.CleanupItem{}, saasbackup.Conflict("备份运行已脱离冻结的清理快照")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_runs
		SET status = 'deleted', replica_status = IF(replica_object_key = '', replica_status, 'deleted'),
			cleanup_run_id = NULL, deleted_at = NOW(), version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ? AND cleanup_run_id = ?
	`, before.ID, before.Version, run.ID)
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saasbackup.CleanupItem{}, saasbackup.Conflict("备份运行已脱离冻结的清理快照")
	}
	after, err := scanSaaSBackupRun(tx.QueryRowContext(ctx, saasBackupRunSelect+` WHERE id = ?`, before.ID))
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.Actor.UserID, ActorTenantID: input.Actor.TenantID,
		Action: saasbackup.OperationActionDelete, TargetType: "saas_backup_run", TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.BackupNo,
		BeforeJSON: saasBackupRunAuditJSON(before), AfterJSON: saasBackupRunAuditJSON(after), Remark: "delete backup through approved retention cleanup " + run.CleanupNo,
	})
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_backup_runs SET operation_id = ? WHERE id = ?`, operationID, after.ID); err != nil {
		return saasbackup.CleanupItem{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_cleanup_items
		SET status = 'succeeded', record_status = 'deleted', record_deleted_at = NOW(), operation_id = ?, last_error = '', updated_at = NOW()
		WHERE id = ?
	`, operationID, item.ID); err != nil {
		return saasbackup.CleanupItem{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_backup_cleanup_runs SET lease_expires_at = DATE_ADD(NOW(), INTERVAL 5 MINUTE), last_error = '', updated_at = NOW() WHERE id = ?`, run.ID); err != nil {
		return saasbackup.CleanupItem{}, err
	}
	updated, err := scanSaaSBackupCleanupItem(tx.QueryRowContext(ctx, saasBackupCleanupItemSelect+` WHERE id = ?`, item.ID))
	if err != nil {
		return saasbackup.CleanupItem{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.CleanupItem{}, err
	}
	return updated, nil
}

func (s *MySQLStore) FinishBackupCleanup(ctx context.Context, input saasbackup.CleanupRunCompletion) (saasbackup.CleanupRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasbackup.CleanupRun{}, err
	}
	defer rollbackQuietly(tx)
	before, err := lockBackupCleanupExecution(ctx, tx, input.CleanupRunID, input.CleanupAttempt)
	if err != nil {
		return saasbackup.CleanupRun{}, err
	}
	var total, succeeded, failed, replicasDeleted, missingFiles int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(status = 'succeeded'), 0), COALESCE(SUM(status = 'failed'), 0),
			COALESCE(SUM(replica_status = 'deleted'), 0), COALESCE(SUM(local_status = 'missing'), 0)
		FROM mochat_go_saas_backup_cleanup_items WHERE cleanup_run_id = ?
	`, before.ID).Scan(&total, &succeeded, &failed, &replicasDeleted, &missingFiles); err != nil {
		return saasbackup.CleanupRun{}, err
	}
	pending := total - succeeded - failed
	status := saasbackup.CleanupStatusPending
	activeSlot := any(1)
	finishedAt := any(nil)
	if pending == 0 {
		activeSlot = nil
		finishedAt = time.Now()
		switch {
		case failed == 0 && succeeded == total:
			status = saasbackup.CleanupStatusSucceeded
		case succeeded > 0:
			status = saasbackup.CleanupStatusPartial
		default:
			status = saasbackup.CleanupStatusFailed
		}
	}
	lastError := ""
	if failed > 0 {
		_ = tx.QueryRowContext(ctx, `SELECT last_error FROM mochat_go_saas_backup_cleanup_items WHERE cleanup_run_id = ? AND status = 'failed' ORDER BY id DESC LIMIT 1`, before.ID).Scan(&lastError)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_cleanup_runs
		SET status = ?, active_slot = ?, deleted_count = ?, failed_count = ?, replicas_deleted_count = ?,
			missing_files_count = ?, lease_expires_at = NULL, finished_at = ?, last_error = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND status = 'running' AND attempts = ?
	`, status, activeSlot, succeeded, failed, replicasDeleted, missingFiles, finishedAt, truncateRunes(lastError, 1000), before.ID, input.CleanupAttempt); err != nil {
		return saasbackup.CleanupRun{}, err
	}
	after, err := scanSaaSBackupCleanupRun(tx.QueryRowContext(ctx, saasBackupCleanupRunSelect+` WHERE id = ?`, before.ID))
	if err != nil {
		return saasbackup.CleanupRun{}, err
	}
	if status != saasbackup.CleanupStatusPending {
		if _, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
			TenantID: 0, ActorUserID: input.Actor.UserID, ActorTenantID: input.Actor.TenantID,
			Action: saasbackup.OperationActionCleanupFinish, TargetType: "saas_backup_cleanup_run",
			TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.CleanupNo,
			BeforeJSON: saasBackupCleanupRunAuditJSON(before), AfterJSON: saasBackupCleanupRunAuditJSON(after), Remark: "finish SaaS backup retention cleanup",
		}); err != nil {
			return saasbackup.CleanupRun{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return saasbackup.CleanupRun{}, err
	}
	return after, nil
}

func claimBackupCleanupTx(ctx context.Context, tx *sql.Tx, cleanupRunID int64, retry bool, actor saasbackup.Actor) (saasbackup.CleanupRun, error) {
	before, err := scanSaaSBackupCleanupRun(tx.QueryRowContext(ctx, saasBackupCleanupRunSelect+` WHERE id = ? FOR UPDATE`, cleanupRunID))
	if errors.Is(err, sql.ErrNoRows) {
		return saasbackup.CleanupRun{}, saasbackup.NotFound("备份清理任务不存在")
	}
	if err != nil {
		return saasbackup.CleanupRun{}, err
	}
	if before.Status == saasbackup.CleanupStatusSucceeded {
		return before, saasbackup.Conflict("备份清理任务已经完成")
	}
	if before.Status == saasbackup.CleanupStatusPartial || before.Status == saasbackup.CleanupStatusFailed {
		if !retry {
			return before, saasbackup.Conflict("失败或部分成功的备份清理任务需要人工重试")
		}
	} else if retry && before.Status != saasbackup.CleanupStatusPending {
		return before, saasbackup.Conflict("当前备份清理任务不允许重试")
	}
	if before.Status == saasbackup.CleanupStatusRunning {
		lease, parseErr := parseOptionalStoreTime(before.LeaseExpiresAt)
		if parseErr == nil && lease.After(time.Now()) {
			return before, saasbackup.Conflict("备份清理任务正在执行")
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_backup_cleanup_runs
		SET status = 'running', active_slot = 1, attempts = attempts + 1,
			lease_expires_at = DATE_ADD(NOW(), INTERVAL 5 MINUTE), started_at = COALESCE(started_at, NOW()),
			finished_at = NULL, last_error = '', version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, before.ID, before.Version)
	if isMySQLDuplicateKeyError(err) {
		return before, saasbackup.Conflict("另一个备份清理任务正在执行")
	}
	if err != nil {
		return before, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return before, saasbackup.Conflict("备份清理任务状态已变化")
	}
	after, err := scanSaaSBackupCleanupRun(tx.QueryRowContext(ctx, saasBackupCleanupRunSelect+` WHERE id = ?`, before.ID))
	if err != nil {
		return saasbackup.CleanupRun{}, err
	}
	if retry {
		if _, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
			TenantID: 0, ActorUserID: actor.UserID, ActorTenantID: actor.TenantID,
			Action: saasbackup.OperationActionCleanupRetry, TargetType: "saas_backup_cleanup_run",
			TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.CleanupNo,
			BeforeJSON: saasBackupCleanupRunAuditJSON(before), AfterJSON: saasBackupCleanupRunAuditJSON(after), Remark: "retry approved SaaS backup retention cleanup",
		}); err != nil {
			return saasbackup.CleanupRun{}, err
		}
	}
	return after, nil
}

func lockBackupCleanupExecution(ctx context.Context, tx *sql.Tx, cleanupRunID int64, attempt int) (saasbackup.CleanupRun, error) {
	run, err := scanSaaSBackupCleanupRun(tx.QueryRowContext(ctx, saasBackupCleanupRunSelect+` WHERE id = ? FOR UPDATE`, cleanupRunID))
	if errors.Is(err, sql.ErrNoRows) {
		return run, saasbackup.NotFound("备份清理任务不存在")
	}
	if err != nil {
		return run, err
	}
	if run.Status != saasbackup.CleanupStatusRunning || run.Attempts != attempt {
		return run, saasbackup.Conflict("备份清理执行租约已被新的执行取代")
	}
	return run, nil
}

func backupCleanupItemsTx(ctx context.Context, tx *sql.Tx, cleanupRunID int64) ([]saasbackup.CleanupItem, error) {
	rows, err := tx.QueryContext(ctx, saasBackupCleanupItemSelect+` WHERE cleanup_run_id = ? ORDER BY id ASC`, cleanupRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saasbackup.CleanupItem, 0)
	for rows.Next() {
		item, scanErr := scanSaaSBackupCleanupItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanSaaSBackupCleanupRun(scanner interface{ Scan(...any) error }) (saasbackup.CleanupRun, error) {
	var item saasbackup.CleanupRun
	var activeSlot, approvalID sql.NullInt64
	var cutoffAt sql.NullTime
	var leaseExpiresAt, startedAt, finishedAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.CleanupNo, &item.Status, &activeSlot, &item.PolicyVersion, &cutoffAt,
		&item.ScannedCount, &item.CandidateCount, &item.PreservedCount, &item.DeletedCount, &item.FailedCount,
		&item.ReplicasDeletedCount, &item.MissingFilesCount, &item.Attempts, &approvalID, &leaseExpiresAt,
		&startedAt, &finishedAt, &item.LastError, &item.CreatedBy, &item.ActorTenantID, &item.OperationID,
		&item.Version, &createdAt, &updatedAt)
	if approvalID.Valid {
		item.ApprovalID = approvalID.Int64
	}
	item.CutoffAt = formatTime(cutoffAt)
	item.LeaseExpiresAt, item.StartedAt, item.FinishedAt = formatTime(leaseExpiresAt), formatTime(startedAt), formatTime(finishedAt)
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, err
}

func scanSaaSBackupCleanupItem(scanner interface{ Scan(...any) error }) (saasbackup.CleanupItem, error) {
	var item saasbackup.CleanupItem
	var replicaDeletedAt, localDeletedAt, recordDeletedAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.CleanupRunID, &item.BackupRunID, &item.BackupRunVersion,
		&item.BackupNo, &item.ArtifactName, &item.ReplicaObjectKey, &item.ReplicaVersionID, &item.Status,
		&item.ReplicaStatus, &item.LocalStatus, &item.RecordStatus, &item.Attempts, &item.LastError,
		&replicaDeletedAt, &localDeletedAt, &recordDeletedAt, &item.OperationID, &createdAt, &updatedAt)
	item.ReplicaDeletedAt, item.LocalDeletedAt, item.RecordDeletedAt = formatTime(replicaDeletedAt), formatTime(localDeletedAt), formatTime(recordDeletedAt)
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, err
}

func sameBackupCleanupSnapshot(run saasbackup.BackupRun, planned saasbackup.CleanupPlanItem) bool {
	return run.ID == planned.BackupRunID && run.Version == planned.BackupRunVersion && run.BackupNo == planned.BackupNo &&
		run.Status == planned.Status && run.FinishedAt == planned.FinishedAt && run.ArtifactName == planned.ArtifactName &&
		strings.EqualFold(run.SHA256, planned.SHA256) && run.SizeBytes == planned.SizeBytes &&
		run.ReplicaStatus == planned.ReplicaStatus && run.ReplicaProvider == planned.ReplicaProvider &&
		run.ReplicaBucket == planned.ReplicaBucket && run.ReplicaObjectKey == planned.ReplicaObjectKey &&
		run.ReplicaVersionID == planned.ReplicaVersionID
}

func cleanupStepComplete(status string) bool {
	return status == saasbackup.CleanupStepDeleted || status == saasbackup.CleanupStepMissing || status == saasbackup.CleanupStepNotRequired
}

func parseOptionalStoreTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(value), time.Local)
}

func saasBackupCleanupRunAuditJSON(item saasbackup.CleanupRun) string {
	return saasBackupMarshal(map[string]any{
		"id": item.ID, "cleanupNo": item.CleanupNo, "status": item.Status, "policyVersion": item.PolicyVersion,
		"cutoffAt": item.CutoffAt, "scannedCount": item.ScannedCount, "candidateCount": item.CandidateCount,
		"preservedCount": item.PreservedCount, "deletedCount": item.DeletedCount, "failedCount": item.FailedCount,
		"replicasDeletedCount": item.ReplicasDeletedCount, "missingFilesCount": item.MissingFilesCount,
		"attempts": item.Attempts, "approvalId": item.ApprovalID, "lastError": item.LastError, "version": item.Version,
	})
}
