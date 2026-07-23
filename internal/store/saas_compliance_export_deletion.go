package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/saascompliance"
)

func (s *MySQLStore) ScheduleComplianceExportDeletion(ctx context.Context, input saascompliance.DataExportDeletionSchedule) (saascompliance.DataExport, error) {
	if input.Plan.ExportID <= 0 || strings.TrimSpace(input.Plan.ExportNo) == "" || input.ApprovalExecutionID <= 0 || input.ApprovalExecutionVersion <= 0 {
		return saascompliance.DataExport{}, saascompliance.Invalid("合规导出删除计划无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	defer rollbackQuietly(tx)

	before, err := complianceExportByID(ctx, tx, input.Plan.ExportID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.DataExport{}, saascompliance.NotFound("租户数据导出不存在")
	}
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if !sameComplianceExportDeletionSnapshot(before, input.Plan) || before.DeletionStatus != "" {
		return saascompliance.DataExport{}, saascompliance.Conflict("租户数据导出已变化，请重新提交删除审批")
	}
	if err := ensureComplianceExportDeletionNotBlockedTx(ctx, tx, before); err != nil {
		return saascompliance.DataExport{}, err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_data_exports
		SET deletion_status = 'pending', deletion_artifact_status = 'pending', deletion_record_status = 'pending',
			deletion_attempts = 0, deletion_approval_id = ?, deletion_lease_expires_at = NULL,
			deletion_requested_at = NOW(), deletion_started_at = NULL, deletion_finished_at = NULL,
			deletion_last_error = '', deletion_requested_by = ?, deletion_actor_tenant_id = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND deletion_status = '' AND version = ?
	`, input.ApprovalExecutionID, input.Actor.UserID, input.Actor.TenantID, before.ID, before.Version)
	if isMySQLDuplicateKeyError(err) {
		return saascompliance.DataExport{}, saascompliance.Conflict("该审批已绑定其他合规导出删除任务")
	}
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saascompliance.DataExport{}, saascompliance.Conflict("租户数据导出已变化，请重新提交删除审批")
	}
	after, err := complianceExportByID(ctx, tx, before.ID, false)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, before.TenantID, input.Actor,
		saascompliance.OperationActionExportDeletePlan, "saas_data_export", before.ExportNo, before.TenantName,
		before, after, "冻结合规导出工件并启动已审批删除任务")
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_data_exports SET operation_id = ? WHERE id = ?`, operationID, before.ID); err != nil {
		return saascompliance.DataExport{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.Actor.UserID, operationID); err != nil {
		return saascompliance.DataExport{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.DataExport{}, err
	}
	after.OperationID = operationID
	return after, nil
}

func (s *MySQLStore) ClaimComplianceExportDeletion(ctx context.Context, exportID int64, retry bool, actor saascompliance.Actor) (saascompliance.DataExport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	defer rollbackQuietly(tx)
	before, err := complianceExportByID(ctx, tx, exportID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.DataExport{}, saascompliance.NotFound("租户数据导出不存在")
	}
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if before.DeletionStatus == saascompliance.ExportDeletionStatusSucceeded {
		return before, saascompliance.Conflict("合规导出工件已经删除")
	}
	if before.DeletionStatus == saascompliance.ExportDeletionStatusFailed {
		if !retry {
			return before, saascompliance.Conflict("失败的合规导出删除任务需要人工重试")
		}
	} else if before.DeletionStatus == saascompliance.ExportDeletionStatusPending {
		if retry {
			return before, saascompliance.Conflict("待执行的合规导出删除任务不需要重试")
		}
	} else if before.DeletionStatus == saascompliance.ExportDeletionStatusRunning {
		if !before.DeletionLeaseValue.IsZero() && before.DeletionLeaseValue.After(time.Now()) {
			return before, saascompliance.Conflict("合规导出删除任务正在执行")
		}
	} else {
		return before, saascompliance.Conflict("合规导出未绑定已批准的删除任务")
	}
	if err := ensureComplianceExportDeletionNotBlockedTx(ctx, tx, before); err != nil {
		return saascompliance.DataExport{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_data_exports
		SET deletion_status = 'running', deletion_attempts = deletion_attempts + 1,
			deletion_lease_expires_at = DATE_ADD(NOW(), INTERVAL 5 MINUTE),
			deletion_started_at = COALESCE(deletion_started_at, NOW()), deletion_finished_at = NULL,
			deletion_last_error = '', version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, before.ID, before.Version)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return before, saascompliance.Conflict("合规导出删除任务状态已变化")
	}
	after, err := complianceExportByID(ctx, tx, before.ID, false)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if retry {
		if _, err := insertComplianceOperationLog(ctx, tx, before.TenantID, actor,
			saascompliance.OperationActionExportDeleteRetry, "saas_data_export", before.ExportNo, before.TenantName,
			before, after, "重试已审批的合规导出工件删除任务"); err != nil {
			return saascompliance.DataExport{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.DataExport{}, err
	}
	return after, nil
}

func (s *MySQLStore) CheckpointComplianceExportDeletion(ctx context.Context, input saascompliance.DataExportDeletionCheckpoint) (saascompliance.DataExport, error) {
	if input.Status != saascompliance.ExportDeletionStepDeleted && input.Status != saascompliance.ExportDeletionStepMissing && input.Status != saascompliance.ExportDeletionStepFailed {
		return saascompliance.DataExport{}, saascompliance.Invalid("合规导出工件删除步骤状态无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	defer rollbackQuietly(tx)
	before, err := lockComplianceExportDeletion(ctx, tx, input.ExportID, input.Attempt)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	deletionStatus := saascompliance.ExportDeletionStatusRunning
	lease := "DATE_ADD(NOW(), INTERVAL 5 MINUTE)"
	finished := "NULL"
	lastError := ""
	if input.Status == saascompliance.ExportDeletionStepFailed {
		deletionStatus = saascompliance.ExportDeletionStatusFailed
		lease = "NULL"
		finished = "NOW()"
		lastError = truncateRunes(input.ErrorMessage, 1000)
	}
	query := `UPDATE mochat_go_saas_data_exports
		SET deletion_status = ?, deletion_artifact_status = ?, deletion_lease_expires_at = ` + lease + `,
			deletion_finished_at = ` + finished + `, deletion_last_error = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?`
	result, err := tx.ExecContext(ctx, query, deletionStatus, input.Status, lastError, before.ID, before.Version)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return before, saascompliance.Conflict("合规导出删除执行租约已被新的执行取代")
	}
	after, err := complianceExportByID(ctx, tx, before.ID, false)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.DataExport{}, err
	}
	return after, nil
}

func (s *MySQLStore) CompleteComplianceExportDeletion(ctx context.Context, input saascompliance.DataExportDeletionCompletion) (saascompliance.DataExport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	defer rollbackQuietly(tx)
	before, err := lockComplianceExportDeletion(ctx, tx, input.ExportID, input.Attempt)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if before.DeletionArtifactStatus != saascompliance.ExportDeletionStepDeleted && before.DeletionArtifactStatus != saascompliance.ExportDeletionStepMissing {
		return before, saascompliance.Conflict("合规导出工件删除步骤尚未完成")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_data_exports
		SET status = 'deleted', active_slot = NULL, deleted_at = COALESCE(deleted_at, NOW()),
			deletion_status = 'succeeded', deletion_record_status = 'deleted', deletion_lease_expires_at = NULL,
			deletion_finished_at = NOW(), deletion_last_error = '', version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, before.ID, before.Version)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return before, saascompliance.Conflict("合规导出删除任务状态已变化")
	}
	after, err := complianceExportByID(ctx, tx, before.ID, false)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, before.TenantID, input.Actor,
		saascompliance.OperationActionExportDelete, "saas_data_export", before.ExportNo, before.TenantName,
		before, after, "完成已审批的合规导出工件删除任务")
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_data_exports SET operation_id = ? WHERE id = ?`, operationID, before.ID); err != nil {
		return saascompliance.DataExport{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.DataExport{}, err
	}
	after.OperationID = operationID
	return after, nil
}

func lockComplianceExportDeletion(ctx context.Context, tx *sql.Tx, exportID int64, attempt int) (saascompliance.DataExport, error) {
	item, err := complianceExportByID(ctx, tx, exportID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return item, saascompliance.NotFound("租户数据导出不存在")
	}
	if err != nil {
		return item, err
	}
	if item.DeletionStatus != saascompliance.ExportDeletionStatusRunning || item.DeletionAttempts != attempt {
		return item, saascompliance.Conflict("合规导出删除执行租约已被新的执行取代")
	}
	return item, nil
}

func ensureComplianceExportDeletionNotBlockedTx(ctx context.Context, tx *sql.Tx, item saascompliance.DataExport) error {
	if item.Status == saascompliance.ExportStatusPending || item.Status == saascompliance.ExportStatusRunning || item.Status == saascompliance.ExportStatusDeleted || item.DeletedAt != "" {
		return saascompliance.Conflict("当前租户数据导出状态不允许删除")
	}
	var activeHolds int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_legal_holds
		WHERE tenant_id = ? AND status = 'active' AND starts_at <= NOW()
			AND (expires_at IS NULL OR expires_at > NOW())
	`, item.TenantID).Scan(&activeHolds); err != nil {
		return err
	}
	if activeHolds > 0 {
		return saascompliance.Conflict("租户存在生效中的法律保留，不能删除合规导出工件")
	}
	var erasureReferences int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_erasure_requests
		WHERE latest_export_id = ? AND status IN ('pending_approval', 'approved', 'waiting', 'running', 'failed', 'blocked')
	`, item.ID).Scan(&erasureReferences); err != nil {
		return err
	}
	if erasureReferences > 0 {
		return saascompliance.Conflict("合规导出仍被未终结的数据擦除请求引用，不能删除")
	}
	return nil
}

func sameComplianceExportDeletionSnapshot(item saascompliance.DataExport, plan saascompliance.DataExportDeletionPlan) bool {
	return item.ID == plan.ExportID && item.ExportNo == plan.ExportNo && item.TenantID == plan.TenantID &&
		item.TenantName == plan.TenantName && item.Status == plan.Status && item.ArtifactName == plan.ArtifactName &&
		item.SHA256 == plan.SHA256 && item.SizeBytes == plan.SizeBytes && item.FinishedAt == plan.FinishedAt && item.ExpiresAt == plan.ExpiresAt
}
