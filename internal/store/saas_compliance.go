package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/saascompliance"
)

var complianceIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type complianceDatasetRows struct {
	rows    *sql.Rows
	columns []string
}

func (r *complianceDatasetRows) Columns() []string { return append([]string(nil), r.columns...) }
func (r *complianceDatasetRows) Next() bool        { return r.rows.Next() }
func (r *complianceDatasetRows) Err() error        { return r.rows.Err() }
func (r *complianceDatasetRows) Close() error      { return r.rows.Close() }

func (r *complianceDatasetRows) Values() ([]any, error) {
	values := make([]any, len(r.columns))
	targets := make([]any, len(values))
	for index := range values {
		targets[index] = &values[index]
	}
	if err := r.rows.Scan(targets...); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *MySQLStore) CompliancePolicy(ctx context.Context) (saascompliance.Policy, error) {
	return compliancePolicyByID(ctx, s.db, false)
}

func (s *MySQLStore) UpdateCompliancePolicy(ctx context.Context, input saascompliance.PolicyUpdate) (saascompliance.Policy, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.Policy{}, err
	}
	defer tx.Rollback()
	before, err := compliancePolicyByID(ctx, tx, true)
	if err != nil {
		return saascompliance.Policy{}, err
	}
	if before.Version != input.ExpectedVersion {
		return saascompliance.Policy{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "合规策略版本已变化，请刷新后重试"}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_compliance_policies
		SET status = ?, export_retention_days = ?, erasure_grace_days = ?, require_recent_export = ?,
			recent_export_max_age_days = ?, billing_retention_days = ?, audit_retention_days = ?, service_account_usage_retention_days = ?,
			version = version + 1, updated_by = ?, updated_at = NOW()
		WHERE id = 1 AND version = ?
	`, input.Status, input.ExportRetentionDays, input.ErasureGraceDays, boolToInt(input.RequireRecentExport),
		input.RecentExportMaxAgeDays, input.BillingRetentionDays, input.AuditRetentionDays, input.ServiceAccountUsageRetentionDays,
		input.Actor.UserID, input.ExpectedVersion)
	if err != nil {
		return saascompliance.Policy{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saascompliance.Policy{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "合规策略版本已变化，请刷新后重试"}
	}
	after, err := compliancePolicyByID(ctx, tx, false)
	if err != nil {
		return saascompliance.Policy{}, err
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, 0, input.Actor, saascompliance.OperationActionPolicyUpdate,
		"saas_compliance_policy", "1", "租户数据合规策略", before, after, "更新租户数据生命周期策略")
	if err != nil {
		return saascompliance.Policy{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.Actor.UserID, operationID); err != nil {
		return saascompliance.Policy{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.Policy{}, err
	}
	return after, nil
}

func compliancePolicyByID(ctx context.Context, queryer saasAdminAccessQueryer, forUpdate bool) (saascompliance.Policy, error) {
	query := `
		SELECT id, status, export_retention_days, erasure_grace_days, require_recent_export,
			recent_export_max_age_days, billing_retention_days, audit_retention_days, service_account_usage_retention_days,
			version, updated_by, created_at, updated_at
		FROM mochat_go_saas_compliance_policies WHERE id = 1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var item saascompliance.Policy
	var requireRecent int
	var createdAt, updatedAt sql.NullTime
	err := queryer.QueryRowContext(ctx, query).Scan(
		&item.ID, &item.Status, &item.ExportRetentionDays, &item.ErasureGraceDays, &requireRecent,
		&item.RecentExportMaxAgeDays, &item.BillingRetentionDays, &item.AuditRetentionDays, &item.ServiceAccountUsageRetentionDays,
		&item.Version, &item.UpdatedBy, &createdAt, &updatedAt,
	)
	if err != nil {
		return saascompliance.Policy{}, err
	}
	item.RequireRecentExport = requireRecent == 1
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, nil
}

func (s *MySQLStore) ComplianceTenant(ctx context.Context, tenantID int) (saascompliance.Tenant, error) {
	var item saascompliance.Tenant
	var createdAt, updatedAt, deletedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, status, created_at, updated_at, deleted_at
		FROM mc_tenant WHERE id = ? LIMIT 1
	`, tenantID).Scan(&item.ID, &item.Name, &item.Status, &createdAt, &updatedAt, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.Tenant{}, saascompliance.NotFound("业务租户不存在")
	}
	if err != nil {
		return saascompliance.Tenant{}, err
	}
	item.CreatedAt, item.UpdatedAt, item.DeletedAt = formatTime(createdAt), formatTime(updatedAt), formatTime(deletedAt)
	return item, nil
}

func (s *MySQLStore) ComplianceInventoryCoverage(ctx context.Context, specs []saascompliance.DatasetSpec) (saascompliance.InventoryCoverage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT TABLE_NAME
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND COLUMN_NAME IN ('tenant_id', 'corp_id', 'matched_tenant_id')
		ORDER BY TABLE_NAME
	`)
	if err != nil {
		return saascompliance.InventoryCoverage{}, err
	}
	defer rows.Close()
	result := saascompliance.InventoryCoverage{OwnedTables: []string{}, CoveredTables: []string{}, UnknownTables: []string{}}
	covered := saascompliance.InventoryCoveredTables()
	for _, spec := range specs {
		covered[spec.Table] = struct{}{}
	}
	for table := range covered {
		result.CoveredTables = append(result.CoveredTables, table)
	}
	sort.Strings(result.CoveredTables)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return saascompliance.InventoryCoverage{}, err
		}
		result.OwnedTables = append(result.OwnedTables, table)
		if _, ok := covered[table]; !ok {
			result.UnknownTables = append(result.UnknownTables, table)
		}
	}
	return result, rows.Err()
}

func (s *MySQLStore) OpenComplianceDataset(ctx context.Context, spec saascompliance.DatasetSpec, tenantID int) (saascompliance.DatasetRowIterator, error) {
	orderBy := strings.TrimSpace(spec.OrderBy)
	if orderBy == "" {
		orderBy = "id"
	}
	if !complianceIdentifierPattern.MatchString(spec.Table) || !complianceIdentifierPattern.MatchString(orderBy) || strings.TrimSpace(spec.Predicate) == "" {
		return nil, saascompliance.Invalid("租户数据清单定义无效")
	}
	args := complianceTenantArgs(spec.Predicate, tenantID)
	query := fmt.Sprintf("SELECT * FROM `%s` WHERE (%s) ORDER BY `%s` ASC", spec.Table, spec.Predicate, orderBy)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	columns, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, err
	}
	return &complianceDatasetRows{rows: rows, columns: columns}, nil
}

func (s *MySQLStore) ComplianceStorageFiles(ctx context.Context, tenantID int) ([]saascompliance.StorageFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, relative_path, original_name, size_bytes
		FROM mochat_go_saas_storage_objects
		WHERE tenant_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saascompliance.StorageFile, 0)
	for rows.Next() {
		var item saascompliance.StorageFile
		if err := rows.Scan(&item.ID, &item.RelativePath, &item.OriginalName, &item.SizeBytes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) ComplianceLegalHolds(ctx context.Context, tenantID, limit int) ([]saascompliance.LegalHold, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_legal_holds
		SET status = 'expired', active_slot = NULL, version = version + 1, updated_at = NOW()
		WHERE status = 'active' AND expires_at IS NOT NULL AND expires_at <= NOW()
	`)
	query := `
		SELECT h.id, h.hold_no, h.tenant_id, COALESCE(t.name, ''), h.status, h.reason,
			h.starts_at, h.expires_at, h.released_at, h.released_by, h.release_reason,
			h.version, h.created_by, h.updated_by, h.created_at, h.updated_at
		FROM mochat_go_saas_legal_holds h
		LEFT JOIN mc_tenant t ON t.id = h.tenant_id
		WHERE (? = 0 OR h.tenant_id = ?)
		ORDER BY h.id DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, tenantID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saascompliance.LegalHold, 0)
	for rows.Next() {
		item, err := scanComplianceLegalHold(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) CreateComplianceLegalHold(ctx context.Context, input saascompliance.LegalHoldCreate) (saascompliance.LegalHold, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	defer tx.Rollback()
	var tenantName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM mc_tenant WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, input.TenantID).Scan(&tenantName); errors.Is(err, sql.ErrNoRows) {
		return saascompliance.LegalHold{}, saascompliance.NotFound("业务租户不存在")
	} else if err != nil {
		return saascompliance.LegalHold{}, err
	}
	var expires any
	if !input.ExpiresAt.IsZero() {
		expires = input.ExpiresAt
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_legal_holds
			(hold_no, tenant_id, status, active_slot, reason, starts_at, expires_at, version,
			 created_by, updated_by, created_at, updated_at)
		VALUES (?, ?, 'active', ?, ?, ?, ?, 1, ?, ?, NOW(), NOW())
	`, input.HoldNo, input.TenantID, input.TenantID, input.Reason, input.StartsAt, expires, input.Actor.UserID, input.Actor.UserID)
	if isMySQLDuplicateKeyError(err) {
		return saascompliance.LegalHold{}, saascompliance.Conflict("该租户已经存在活动法律保留")
	}
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	id, _ := result.LastInsertId()
	item, err := complianceLegalHoldByID(ctx, tx, id, false)
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, input.TenantID, input.Actor, saascompliance.OperationActionHoldCreate,
		"saas_legal_hold", input.HoldNo, tenantName, nil, item, input.Reason)
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	item.OperationID = operationID
	if err := tx.Commit(); err != nil {
		return saascompliance.LegalHold{}, err
	}
	return item, nil
}

func (s *MySQLStore) ReleaseComplianceLegalHold(ctx context.Context, input saascompliance.LegalHoldRelease) (saascompliance.LegalHold, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	defer tx.Rollback()
	before, err := complianceLegalHoldByID(ctx, tx, input.HoldID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.LegalHold{}, saascompliance.NotFound("法律保留不存在")
	}
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	if input.ApprovalExecutionID > 0 {
		plan := input.ApprovalPlan
		if plan == nil || before.ID != plan.HoldID || before.HoldNo != plan.HoldNo || before.TenantID != plan.TenantID ||
			before.TenantName != plan.TenantName || before.Status != plan.Status || before.Reason != plan.HoldReason ||
			before.StartsAt != plan.StartsAt || before.ExpiresAt != plan.ExpiresAt || before.Version != plan.ExpectedVersion {
			return saascompliance.LegalHold{}, saascompliance.Conflict("法律保留与审批冻结快照不一致，请重新发起审批")
		}
	}
	if before.Status != saascompliance.HoldStatusActive || before.Version != input.ExpectedVersion {
		return saascompliance.LegalHold{}, saascompliance.Conflict("法律保留已变化，请刷新后重试")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_legal_holds
		SET status = 'released', active_slot = NULL, released_at = NOW(), released_by = ?, release_reason = ?,
			version = version + 1, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND status = 'active' AND version = ?
	`, input.Actor.UserID, input.Reason, input.Actor.UserID, input.HoldID, input.ExpectedVersion)
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saascompliance.LegalHold{}, saascompliance.Conflict("法律保留已变化，请刷新后重试")
	}
	after, err := complianceLegalHoldByID(ctx, tx, input.HoldID, false)
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, before.TenantID, input.Actor, saascompliance.OperationActionHoldRelease,
		"saas_legal_hold", before.HoldNo, before.TenantName, before, after, input.Reason)
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	after.OperationID = operationID
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.Actor.UserID, operationID); err != nil {
		return saascompliance.LegalHold{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.LegalHold{}, err
	}
	return after, nil
}

func (s *MySQLStore) ComplianceLegalHold(ctx context.Context, id int64) (saascompliance.LegalHold, error) {
	item, err := complianceLegalHoldByID(ctx, s.db, id, false)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.LegalHold{}, saascompliance.NotFound("法律保留不存在")
	}
	return item, err
}

func (s *MySQLStore) ActiveComplianceLegalHold(ctx context.Context, tenantID int, now time.Time) (saascompliance.LegalHold, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.id, h.hold_no, h.tenant_id, COALESCE(t.name, ''), h.status, h.reason,
			h.starts_at, h.expires_at, h.released_at, h.released_by, h.release_reason,
			h.version, h.created_by, h.updated_by, h.created_at, h.updated_at
		FROM mochat_go_saas_legal_holds h
		LEFT JOIN mc_tenant t ON t.id = h.tenant_id
		WHERE h.tenant_id = ? AND h.status = 'active' AND h.starts_at <= ?
			AND (h.expires_at IS NULL OR h.expires_at > ?)
		ORDER BY h.id DESC LIMIT 1
	`, tenantID, now, now)
	if err != nil {
		return saascompliance.LegalHold{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return saascompliance.LegalHold{}, false, rows.Err()
	}
	item, err := scanComplianceLegalHold(rows)
	return item, err == nil, err
}

func complianceLegalHoldByID(ctx context.Context, queryer saasAdminAccessQueryer, id int64, forUpdate bool) (saascompliance.LegalHold, error) {
	query := `
		SELECT h.id, h.hold_no, h.tenant_id, COALESCE(t.name, ''), h.status, h.reason,
			h.starts_at, h.expires_at, h.released_at, h.released_by, h.release_reason,
			h.version, h.created_by, h.updated_by, h.created_at, h.updated_at
		FROM mochat_go_saas_legal_holds h LEFT JOIN mc_tenant t ON t.id = h.tenant_id
		WHERE h.id = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := queryer.QueryContext(ctx, query, id)
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return saascompliance.LegalHold{}, err
		}
		return saascompliance.LegalHold{}, sql.ErrNoRows
	}
	return scanComplianceLegalHold(rows)
}

func scanComplianceLegalHold(scanner interface{ Scan(...any) error }) (saascompliance.LegalHold, error) {
	var item saascompliance.LegalHold
	var startsAt time.Time
	var expiresAt, releasedAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.HoldNo, &item.TenantID, &item.TenantName, &item.Status, &item.Reason,
		&startsAt, &expiresAt, &releasedAt, &item.ReleasedBy, &item.ReleaseReason,
		&item.Version, &item.CreatedBy, &item.UpdatedBy, &createdAt, &updatedAt)
	if err != nil {
		return saascompliance.LegalHold{}, err
	}
	item.StartsAtValue, item.StartsAt = startsAt, startsAt.Format("2006-01-02 15:04:05")
	item.ExpiresValue, item.ExpiresAt = nullableTimeValue(expiresAt), formatTime(expiresAt)
	item.ReleasedAt, item.CreatedAt, item.UpdatedAt = formatTime(releasedAt), formatTime(createdAt), formatTime(updatedAt)
	return item, nil
}

func (s *MySQLStore) CreateComplianceExport(ctx context.Context, input saascompliance.DataExportCreate) (saascompliance.DataExport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	defer tx.Rollback()
	var tenantName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM mc_tenant WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, input.TenantID).Scan(&tenantName); errors.Is(err, sql.ErrNoRows) {
		return saascompliance.DataExport{}, saascompliance.NotFound("业务租户不存在")
	} else if err != nil {
		return saascompliance.DataExport{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_data_exports
			(export_no, tenant_id, tenant_name_snapshot, status, active_slot, artifact_name, artifact_format,
			 encrypted, encryption_key_id, inventory_version, request_reason, requested_by, actor_tenant_id,
			 operation_id, version, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', ?, ?, ?, 1, ?, ?, ?, ?, ?, 0, 1, NOW(), NOW())
	`, input.ExportNo, input.TenantID, tenantName, input.TenantID, input.ArtifactName, saascompliance.ArtifactFormat,
		input.EncryptionKeyID, input.InventoryVersion, input.Reason, input.Actor.UserID, input.Actor.TenantID)
	if isMySQLDuplicateKeyError(err) {
		return saascompliance.DataExport{}, saascompliance.Conflict("该租户已有待处理或运行中的数据导出")
	}
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	id, _ := result.LastInsertId()
	item, err := complianceExportByID(ctx, tx, id, false)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, input.TenantID, input.Actor, saascompliance.OperationActionExportRequest,
		"saas_data_export", input.ExportNo, tenantName, nil, item, input.Reason)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_data_exports SET operation_id = ? WHERE id = ?`, operationID, id); err != nil {
		return saascompliance.DataExport{}, err
	}
	item.OperationID = operationID
	if err := tx.Commit(); err != nil {
		return saascompliance.DataExport{}, err
	}
	return item, nil
}

func (s *MySQLStore) ClaimComplianceExport(ctx context.Context, exportID int64, now time.Time) (saascompliance.DataExport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	defer tx.Rollback()
	before, err := complianceExportByID(ctx, tx, exportID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.DataExport{}, saascompliance.NotFound("租户数据导出不存在")
	}
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if before.DeletionStatus != "" {
		return saascompliance.DataExport{}, saascompliance.Conflict("租户数据导出已进入审批删除流程")
	}
	if before.Status != saascompliance.ExportStatusPending && before.Status != saascompliance.ExportStatusFailed {
		return saascompliance.DataExport{}, saascompliance.Conflict("租户数据导出状态不允许处理")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_data_exports
		SET status = 'running', active_slot = tenant_id, started_at = ?, finished_at = NULL, expires_at = NULL,
			error_message = '', version = version + 1, updated_at = NOW()
		WHERE id = ? AND status IN ('pending', 'failed') AND version = ?
	`, now, exportID, before.Version)
	if isMySQLDuplicateKeyError(err) {
		return saascompliance.DataExport{}, saascompliance.Conflict("该租户已有运行中的数据导出")
	}
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saascompliance.DataExport{}, saascompliance.Conflict("租户数据导出状态已变化")
	}
	after, err := complianceExportByID(ctx, tx, exportID, false)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.DataExport{}, err
	}
	return after, nil
}

func (s *MySQLStore) CompleteComplianceExport(ctx context.Context, input saascompliance.DataExportCompletion) (saascompliance.DataExport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	defer tx.Rollback()
	before, err := complianceExportByID(ctx, tx, input.ExportID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.DataExport{}, saascompliance.NotFound("租户数据导出不存在")
	}
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if before.Status != saascompliance.ExportStatusRunning {
		return saascompliance.DataExport{}, saascompliance.Conflict("租户数据导出不在运行状态")
	}
	if input.Status != saascompliance.ExportStatusSucceeded && input.Status != saascompliance.ExportStatusFailed {
		return saascompliance.DataExport{}, saascompliance.Invalid("导出完成状态无效")
	}
	var expires any
	if !input.ExpiresAt.IsZero() {
		expires = input.ExpiresAt
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_data_exports
		SET status = ?, active_slot = NULL, sha256 = ?, size_bytes = ?, manifest_sha256 = ?,
			table_count = ?, row_count = ?, file_count = ?, file_size_bytes = ?, error_message = ?,
			finished_at = ?, expires_at = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND status = 'running' AND version = ?
	`, input.Status, input.SHA256, input.SizeBytes, input.ManifestSHA256, input.TableCount, input.RowCount,
		input.FileCount, input.FileSizeBytes, input.ErrorMessage, input.FinishedAt, expires, input.ExportID, before.Version)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saascompliance.DataExport{}, saascompliance.Conflict("租户数据导出状态已变化")
	}
	after, err := complianceExportByID(ctx, tx, input.ExportID, false)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	remark := "租户数据导出失败"
	if input.Status == saascompliance.ExportStatusSucceeded {
		remark = "租户数据导出完成"
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, before.TenantID, input.Actor, saascompliance.OperationActionExportComplete,
		"saas_data_export", before.ExportNo, before.TenantName, before, after, remark)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_data_exports SET operation_id = ? WHERE id = ?`, operationID, input.ExportID); err != nil {
		return saascompliance.DataExport{}, err
	}
	after.OperationID = operationID
	if err := tx.Commit(); err != nil {
		return saascompliance.DataExport{}, err
	}
	return after, nil
}

func (s *MySQLStore) ComplianceExport(ctx context.Context, id int64) (saascompliance.DataExport, error) {
	item, err := complianceExportByID(ctx, s.db, id, false)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.DataExport{}, saascompliance.NotFound("租户数据导出不存在")
	}
	return item, err
}

func (s *MySQLStore) ComplianceExports(ctx context.Context, tenantID, limit int) ([]saascompliance.DataExport, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, complianceExportSelect+`
		WHERE (? = 0 OR e.tenant_id = ?)
		ORDER BY e.id DESC LIMIT ?
	`, tenantID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saascompliance.DataExport, 0)
	for rows.Next() {
		item, err := scanComplianceExport(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) NextPendingComplianceExport(ctx context.Context) (saascompliance.DataExport, bool, error) {
	rows, err := s.db.QueryContext(ctx, complianceExportSelect+`
		WHERE e.status = 'pending' ORDER BY e.id ASC LIMIT 1
	`)
	if err != nil {
		return saascompliance.DataExport{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return saascompliance.DataExport{}, false, rows.Err()
	}
	item, err := scanComplianceExport(rows)
	return item, err == nil, err
}

func (s *MySQLStore) RunnableComplianceExportDeletions(ctx context.Context, limit int) ([]saascompliance.DataExport, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, complianceExportSelect+`
		WHERE e.deletion_status = 'pending'
			OR (e.deletion_status = 'running' AND
				(e.deletion_lease_expires_at IS NULL OR e.deletion_lease_expires_at <= NOW()))
		ORDER BY e.deletion_requested_at ASC, e.id ASC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saascompliance.DataExport, 0)
	for rows.Next() {
		item, err := scanComplianceExport(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) RecordComplianceExportDownload(ctx context.Context, exportID int64, actor saascompliance.Actor) (saascompliance.DataExport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	defer tx.Rollback()
	before, err := complianceExportByID(ctx, tx, exportID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.DataExport{}, saascompliance.NotFound("租户数据导出不存在")
	}
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if before.Status != saascompliance.ExportStatusSucceeded || before.DeletedAt != "" || before.DeletionStatus != "" {
		return saascompliance.DataExport{}, saascompliance.Conflict("租户数据导出尚不可下载")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_data_exports
		SET download_count = download_count + 1, last_downloaded_at = NOW(), last_downloaded_by = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ?
	`, actor.UserID, exportID); err != nil {
		return saascompliance.DataExport{}, err
	}
	after, err := complianceExportByID(ctx, tx, exportID, false)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if _, err := insertComplianceOperationLog(ctx, tx, before.TenantID, actor, saascompliance.OperationActionExportDownload,
		"saas_data_export", before.ExportNo, before.TenantName, nil, map[string]any{"downloadCount": after.DownloadCount}, "下载加密租户数据导出"); err != nil {
		return saascompliance.DataExport{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.DataExport{}, err
	}
	return after, nil
}

func (s *MySQLStore) MarkComplianceExportDeleted(ctx context.Context, exportID int64, actor saascompliance.Actor) (saascompliance.DataExport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	defer tx.Rollback()
	before, err := complianceExportByID(ctx, tx, exportID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.DataExport{}, saascompliance.NotFound("租户数据导出不存在")
	}
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if before.DeletionStatus != "" {
		return saascompliance.DataExport{}, saascompliance.Conflict("租户数据导出已进入审批删除流程")
	}
	if before.Status == saascompliance.ExportStatusRunning || before.Status == saascompliance.ExportStatusPending {
		return saascompliance.DataExport{}, saascompliance.Conflict("运行中的租户数据导出不能删除")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_data_exports
		SET status = 'deleted', active_slot = NULL, deleted_at = COALESCE(deleted_at, NOW()),
			version = version + 1, updated_at = NOW()
		WHERE id = ?
	`, exportID); err != nil {
		return saascompliance.DataExport{}, err
	}
	after, err := complianceExportByID(ctx, tx, exportID, false)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	if _, err := insertComplianceOperationLog(ctx, tx, before.TenantID, actor, saascompliance.OperationActionExportDelete,
		"saas_data_export", before.ExportNo, before.TenantName, before, after, "删除租户数据导出工件"); err != nil {
		return saascompliance.DataExport{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.DataExport{}, err
	}
	return after, nil
}

const complianceExportSelect = `
	SELECT e.id, e.export_no, e.tenant_id, e.tenant_name_snapshot, e.status, e.artifact_name,
		e.artifact_format, e.encrypted, e.encryption_key_id, e.sha256, e.size_bytes, e.manifest_sha256,
		e.inventory_version, e.table_count, e.row_count, e.file_count, e.file_size_bytes,
		e.request_reason, e.requested_by, e.actor_tenant_id, e.started_at, e.finished_at, e.expires_at,
		e.error_message, e.download_count, e.last_downloaded_at, e.last_downloaded_by,
		e.operation_id, e.deletion_status, e.deletion_artifact_status, e.deletion_record_status,
		e.deletion_attempts, e.deletion_approval_id, e.deletion_lease_expires_at, e.deletion_requested_at,
		e.deletion_started_at, e.deletion_finished_at, e.deletion_last_error, e.deletion_requested_by,
		e.deletion_actor_tenant_id, e.version, e.created_at, e.updated_at, e.deleted_at
	FROM mochat_go_saas_data_exports e
`

func complianceExportByID(ctx context.Context, queryer saasAdminAccessQueryer, id int64, forUpdate bool) (saascompliance.DataExport, error) {
	query := complianceExportSelect + " WHERE e.id = ?"
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := queryer.QueryContext(ctx, query, id)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return saascompliance.DataExport{}, err
		}
		return saascompliance.DataExport{}, sql.ErrNoRows
	}
	return scanComplianceExport(rows)
}

func scanComplianceExport(scanner interface{ Scan(...any) error }) (saascompliance.DataExport, error) {
	var item saascompliance.DataExport
	var encrypted int
	var deletionApprovalID sql.NullInt64
	var startedAt, finishedAt, expiresAt, lastDownloadedAt, deletionLeaseExpiresAt, deletionRequestedAt sql.NullTime
	var deletionStartedAt, deletionFinishedAt, createdAt, updatedAt, deletedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.ExportNo, &item.TenantID, &item.TenantName, &item.Status, &item.ArtifactName,
		&item.ArtifactFormat, &encrypted, &item.EncryptionKeyID, &item.SHA256, &item.SizeBytes, &item.ManifestSHA256,
		&item.InventoryVersion, &item.TableCount, &item.RowCount, &item.FileCount, &item.FileSizeBytes,
		&item.RequestReason, &item.RequestedBy, &item.ActorTenantID, &startedAt, &finishedAt, &expiresAt,
		&item.ErrorMessage, &item.DownloadCount, &lastDownloadedAt, &item.LastDownloadedBy,
		&item.OperationID, &item.DeletionStatus, &item.DeletionArtifactStatus, &item.DeletionRecordStatus,
		&item.DeletionAttempts, &deletionApprovalID, &deletionLeaseExpiresAt, &deletionRequestedAt,
		&deletionStartedAt, &deletionFinishedAt, &item.DeletionLastError, &item.DeletionRequestedBy,
		&item.DeletionActorTenantID, &item.Version, &createdAt, &updatedAt, &deletedAt)
	if err != nil {
		return saascompliance.DataExport{}, err
	}
	item.Encrypted = encrypted == 1
	item.StartedAt, item.FinishedAt, item.ExpiresAt = formatTime(startedAt), formatTime(finishedAt), formatTime(expiresAt)
	item.FinishedAtValue, item.ExpiresAtValue = nullableTimeValue(finishedAt), nullableTimeValue(expiresAt)
	item.LastDownloadedAt = formatTime(lastDownloadedAt)
	if deletionApprovalID.Valid {
		item.DeletionApprovalID = deletionApprovalID.Int64
	}
	item.DeletionLeaseExpiresAt = formatTime(deletionLeaseExpiresAt)
	item.DeletionLeaseValue = nullableTimeValue(deletionLeaseExpiresAt)
	item.DeletionRequestedAt = formatTime(deletionRequestedAt)
	item.DeletionStartedAt = formatTime(deletionStartedAt)
	item.DeletionFinishedAt = formatTime(deletionFinishedAt)
	item.CreatedAt, item.UpdatedAt, item.DeletedAt = formatTime(createdAt), formatTime(updatedAt), formatTime(deletedAt)
	return item, nil
}

func (s *MySQLStore) CreateComplianceErasure(ctx context.Context, input saascompliance.ErasureCreate) (saascompliance.ErasureRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	defer tx.Rollback()
	var tenantName string
	var tenantStatus int
	if err := tx.QueryRowContext(ctx, `SELECT name, status FROM mc_tenant WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, input.TenantID).Scan(&tenantName, &tenantStatus); errors.Is(err, sql.ErrNoRows) {
		return saascompliance.ErasureRequest{}, saascompliance.NotFound("业务租户不存在")
	} else if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if tenantStatus != 2 {
		return saascompliance.ErasureRequest{}, saascompliance.Conflict("擦除前必须先停用业务租户")
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_erasure_requests
			(request_no, tenant_id, tenant_name_snapshot, status, active_slot, reason, confirmation_sha256,
			 eligible_at, latest_export_id, inventory_version, requested_by, actor_tenant_id,
			 operation_id, version, created_at, updated_at)
		VALUES (?, ?, ?, 'pending_approval', ?, ?, ?, ?, ?, ?, ?, ?, 0, 1, NOW(), NOW())
	`, input.RequestNo, input.TenantID, tenantName, input.TenantID, input.Reason, input.ConfirmationSHA,
		input.EligibleAt, input.LatestExportID, input.InventoryVersion, input.Actor.UserID, input.Actor.TenantID)
	if isMySQLDuplicateKeyError(err) {
		return saascompliance.ErasureRequest{}, saascompliance.Conflict("该租户已有未终结的擦除请求")
	}
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	id, _ := result.LastInsertId()
	item, err := complianceErasureByID(ctx, tx, id, false)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, input.TenantID, input.Actor, saascompliance.OperationActionErasureRequest,
		"saas_erasure_request", input.RequestNo, tenantName, nil, item, input.Reason)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_erasure_requests SET operation_id = ? WHERE id = ?`, operationID, id); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	item.OperationID = operationID
	if err := tx.Commit(); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	return item, nil
}

func (s *MySQLStore) AuthorizeComplianceErasure(ctx context.Context, input saascompliance.ErasureAuthorize) (saascompliance.ErasureRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	defer tx.Rollback()
	before, err := complianceErasureByID(ctx, tx, input.RequestID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.ErasureRequest{}, saascompliance.NotFound("租户数据擦除请求不存在")
	}
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if before.ApprovalID == input.ApprovalID && before.Status != saascompliance.ErasureStatusPendingApproval {
		if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalID, input.ApprovalVersion, input.Actor.UserID, before.OperationID); err != nil {
			return saascompliance.ErasureRequest{}, err
		}
		if err := tx.Commit(); err != nil {
			return saascompliance.ErasureRequest{}, err
		}
		return before, nil
	}
	if before.Status != saascompliance.ErasureStatusPendingApproval || before.ApprovalID != 0 {
		return saascompliance.ErasureRequest{}, saascompliance.Conflict("租户数据擦除请求已授权或状态已变化")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_requests
		SET status = CASE WHEN eligible_at <= NOW() THEN 'approved' ELSE 'waiting' END,
			approval_id = ?, approved_at = NOW(), approved_by = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND status = 'pending_approval' AND approval_id = 0 AND version = ?
	`, input.ApprovalID, input.ApprovalUserID, input.RequestID, before.Version)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saascompliance.ErasureRequest{}, saascompliance.Conflict("租户数据擦除请求状态已变化")
	}
	after, err := complianceErasureByID(ctx, tx, input.RequestID, false)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, before.TenantID, input.Actor, saascompliance.OperationActionErasureAuthorize,
		"saas_erasure_request", before.RequestNo, before.TenantName, before, after, "双人审批完成，授权进入擦除队列")
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_erasure_requests SET operation_id = ? WHERE id = ?`, operationID, input.RequestID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalID, input.ApprovalVersion, input.Actor.UserID, operationID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	after.OperationID = operationID
	if err := tx.Commit(); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	return after, nil
}

func (s *MySQLStore) CancelComplianceErasure(ctx context.Context, requestID int64, reason string, actor saascompliance.Actor) (saascompliance.ErasureRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	defer tx.Rollback()
	before, err := complianceErasureByID(ctx, tx, requestID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.ErasureRequest{}, saascompliance.NotFound("租户数据擦除请求不存在")
	}
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if before.Status == saascompliance.ErasureStatusRunning || before.Status == saascompliance.ErasureStatusSucceeded || before.Status == saascompliance.ErasureStatusCanceled {
		return saascompliance.ErasureRequest{}, saascompliance.Conflict("租户数据擦除请求当前状态不能取消")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_requests
		SET status = 'canceled', active_slot = NULL, last_error = ?, finished_at = NOW(),
			version = version + 1, updated_at = NOW()
		WHERE id = ?
	`, reason, requestID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	after, err := complianceErasureByID(ctx, tx, requestID, false)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, before.TenantID, actor, saascompliance.OperationActionErasureCancel,
		"saas_erasure_request", before.RequestNo, before.TenantName, before, after, reason)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_erasure_requests SET operation_id = ? WHERE id = ?`, operationID, requestID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	after.OperationID = operationID
	if err := tx.Commit(); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	return after, nil
}

func (s *MySQLStore) ComplianceErasure(ctx context.Context, id int64) (saascompliance.ErasureRequest, error) {
	item, err := complianceErasureByID(ctx, s.db, id, false)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.ErasureRequest{}, saascompliance.NotFound("租户数据擦除请求不存在")
	}
	return item, err
}

func (s *MySQLStore) ComplianceErasures(ctx context.Context, tenantID, limit int) ([]saascompliance.ErasureRequest, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, complianceErasureSelect+`
		WHERE (? = 0 OR r.tenant_id = ?)
		ORDER BY r.id DESC LIMIT ?
	`, tenantID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saascompliance.ErasureRequest, 0)
	for rows.Next() {
		item, err := scanComplianceErasure(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) NextDueComplianceErasure(ctx context.Context, now time.Time) (saascompliance.ErasureRequest, bool, error) {
	rows, err := s.db.QueryContext(ctx, complianceErasureSelect+`
		WHERE r.status IN ('approved', 'waiting') AND r.eligible_at <= ?
		ORDER BY r.id ASC LIMIT 1
	`, now)
	if err != nil {
		return saascompliance.ErasureRequest{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return saascompliance.ErasureRequest{}, false, rows.Err()
	}
	item, err := scanComplianceErasure(rows)
	return item, err == nil, err
}

func (s *MySQLStore) ComplianceRetentionBlocks(ctx context.Context, tenantID int, policy saascompliance.Policy, now time.Time) ([]saascompliance.RetentionBlock, error) {
	if policy.BillingRetentionDays <= 0 {
		return []saascompliance.RetentionBlock{}, nil
	}
	cutoff := now.Add(-time.Duration(policy.BillingRetentionDays) * 24 * time.Hour)
	seen := make(map[string]struct{})
	blocks := make([]saascompliance.RetentionBlock, 0)
	for _, spec := range saascompliance.Inventory() {
		if spec.RetentionClass != saascompliance.RetentionFinancial {
			continue
		}
		if _, ok := seen[spec.Table]; ok {
			continue
		}
		seen[spec.Table] = struct{}{}
		if !complianceIdentifierPattern.MatchString(spec.Table) {
			return nil, saascompliance.Invalid("租户数据清单定义无效")
		}
		args := complianceTenantArgs(spec.Predicate, tenantID)
		args = append(args, cutoff)
		query := fmt.Sprintf("SELECT COUNT(*), MAX(created_at) FROM `%s` WHERE (%s) AND created_at > ?", spec.Table, spec.Predicate)
		var count int64
		var latest sql.NullTime
		if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count, &latest); err != nil {
			return nil, err
		}
		if count == 0 || !latest.Valid {
			continue
		}
		blocks = append(blocks, saascompliance.RetentionBlock{
			Category: saascompliance.RetentionFinancial, Table: spec.Table, RecordCount: count,
			RetainUntil: latest.Time.Add(time.Duration(policy.BillingRetentionDays) * 24 * time.Hour).Format("2006-01-02 15:04:05"),
		})
	}
	return blocks, nil
}

func (s *MySQLStore) PrepareComplianceErasure(ctx context.Context, requestID int64, specs []saascompliance.DatasetSpec, now time.Time) (saascompliance.ErasureRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	defer tx.Rollback()
	before, err := complianceErasureByID(ctx, tx, requestID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.ErasureRequest{}, saascompliance.NotFound("租户数据擦除请求不存在")
	}
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	allowed := before.Status == saascompliance.ErasureStatusApproved || before.Status == saascompliance.ErasureStatusWaiting ||
		before.Status == saascompliance.ErasureStatusRunning || before.Status == saascompliance.ErasureStatusFailed || before.Status == saascompliance.ErasureStatusBlocked
	if !allowed || before.ApprovalID <= 0 || now.Before(before.EligibleAtValue) {
		return saascompliance.ErasureRequest{}, saascompliance.Conflict("租户数据擦除请求尚未满足执行条件")
	}
	for _, spec := range specs {
		if _, err := tx.ExecContext(ctx, `
			INSERT IGNORE INTO mochat_go_saas_erasure_steps
				(request_id, step_order, step_key, table_name, action, status, affected_rows, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'pending', 0, NOW(), NOW())
		`, requestID, spec.DeleteOrder, spec.Key, spec.Table, spec.Action); err != nil {
			return saascompliance.ErasureRequest{}, err
		}
	}
	for _, special := range []struct {
		order  int
		key    string
		action string
	}{
		{250, "tenant_storage_files", saascompliance.DatasetActionFiles},
		{950, "compliance_export_artifacts", saascompliance.DatasetActionArtifacts},
		{975, "verify_erasure", saascompliance.DatasetActionVerify},
	} {
		if _, err := tx.ExecContext(ctx, `
			INSERT IGNORE INTO mochat_go_saas_erasure_steps
				(request_id, step_order, step_key, table_name, action, status, affected_rows, created_at, updated_at)
			VALUES (?, ?, ?, '', ?, 'pending', 0, NOW(), NOW())
		`, requestID, special.order, special.key, special.action); err != nil {
			return saascompliance.ErasureRequest{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_steps
		SET status = 'pending', error_message = '', started_at = NULL, finished_at = NULL, updated_at = NOW()
		WHERE request_id = ? AND status = 'failed'
	`, requestID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	var total, completed int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*), SUM(CASE WHEN status = 'succeeded' THEN 1 ELSE 0 END)
		FROM mochat_go_saas_erasure_steps WHERE request_id = ?
	`, requestID).Scan(&total, &completed); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_requests
		SET status = 'running', started_at = COALESCE(started_at, ?), finished_at = NULL,
			total_steps = ?, completed_steps = ?, last_error = '', version = version + 1, updated_at = NOW()
		WHERE id = ?
	`, now, total, completed, requestID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	after, err := complianceErasureByID(ctx, tx, requestID, false)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	return after, nil
}

func (s *MySQLStore) ComplianceErasureSteps(ctx context.Context, requestID int64) ([]saascompliance.ErasureStep, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, request_id, step_order, step_key, table_name, action, status, affected_rows,
			started_at, finished_at, error_message
		FROM mochat_go_saas_erasure_steps
		WHERE request_id = ? ORDER BY step_order ASC, id ASC
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saascompliance.ErasureStep, 0)
	for rows.Next() {
		item, err := scanComplianceErasureStep(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) ApplyComplianceErasureStep(ctx context.Context, request saascompliance.ErasureRequest, spec saascompliance.DatasetSpec, now time.Time) (saascompliance.ErasureStep, error) {
	if !complianceIdentifierPattern.MatchString(spec.Table) || strings.TrimSpace(spec.Predicate) == "" {
		return saascompliance.ErasureStep{}, saascompliance.Invalid("租户数据清单定义无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	defer tx.Rollback()
	step, err := complianceErasureStepByKey(ctx, tx, request.ID, spec.Key, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.ErasureStep{}, saascompliance.NotFound("租户数据擦除步骤不存在")
	}
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	if step.Status == saascompliance.StepStatusSucceeded {
		if err := tx.Commit(); err != nil {
			return saascompliance.ErasureStep{}, err
		}
		return step, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_steps
		SET status = 'running', started_at = COALESCE(started_at, ?), error_message = '', updated_at = NOW()
		WHERE id = ?
	`, now, step.ID); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	args := complianceTenantArgs(spec.Predicate, request.TenantID)
	var result sql.Result
	switch spec.Action {
	case saascompliance.DatasetActionDelete:
		query := fmt.Sprintf("DELETE FROM `%s` WHERE (%s)", spec.Table, spec.Predicate)
		result, err = tx.ExecContext(ctx, query, args...)
	case saascompliance.DatasetActionRedact:
		switch spec.Table {
		case "mochat_go_saas_admin_operation_logs":
			query := fmt.Sprintf(`
				UPDATE %s SET target_name = '', before_json = NULL, after_json = NULL,
					remark = '[redacted by tenant erasure]', updated_at = NOW()
				WHERE (%s)
			`, "`"+spec.Table+"`", spec.Predicate)
			result, err = tx.ExecContext(ctx, query, args...)
		case "mochat_go_saas_payment_settlement_entries":
			query := fmt.Sprintf(`
				UPDATE %s SET matched_payment_order_id = NULL, matched_refund_id = NULL, matched_tenant_id = 0,
					matched_internal_no = '', raw_json = NULL, issue_message = '', handled_by_user_id = 0,
					handled_by_tenant_id = 0, updated_at = NOW()
				WHERE (%s)
			`, "`"+spec.Table+"`", spec.Predicate)
			result, err = tx.ExecContext(ctx, query, args...)
		case "mochat_go_saas_identity_login_events":
			query := fmt.Sprintf(`
				UPDATE %s SET user_id = 0, tenant_id = 0, phone_sha256 = '', reason_code = 'tenant_erased',
					ip_address = '', user_agent = '', metadata_json = JSON_OBJECT()
				WHERE (%s)
			`, "`"+spec.Table+"`", spec.Predicate)
			result, err = tx.ExecContext(ctx, query, args...)
		case "mochat_go_saas_identity_security_incidents":
			query := fmt.Sprintf(`
				UPDATE %s SET stable_key = CONCAT('erased:', id), tenant_id = 0, user_id = 0,
					title = '[redacted by tenant erasure]', latest_detail = '', assigned_to = '',
					acknowledged_by = 0, resolved_by = 0,
					resolution = CASE WHEN resolution = '' THEN '' ELSE '[redacted by tenant erasure]' END,
					version = version + 1, updated_at = NOW()
				WHERE (%s)
			`, "`"+spec.Table+"`", spec.Predicate)
			result, err = tx.ExecContext(ctx, query, args...)
		default:
			return saascompliance.ErasureStep{}, saascompliance.Invalid("未知审计脱敏步骤")
		}
	case saascompliance.DatasetActionRetain:
		result = complianceStaticResult(0)
	default:
		return saascompliance.ErasureStep{}, saascompliance.Invalid("租户数据擦除步骤动作无效")
	}
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	affected, _ := result.RowsAffected()
	if spec.Action == saascompliance.DatasetActionDelete {
		query := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE (%s)", spec.Table, spec.Predicate)
		var remaining int64
		if err := tx.QueryRowContext(ctx, query, args...).Scan(&remaining); err != nil {
			return saascompliance.ErasureStep{}, err
		}
		if remaining != 0 {
			return saascompliance.ErasureStep{}, fmt.Errorf("erasure verification failed for %s: %d rows remain", spec.Table, remaining)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_steps
		SET status = 'succeeded', affected_rows = ?, finished_at = ?, error_message = '', updated_at = NOW()
		WHERE id = ?
	`, affected, now, step.ID); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	deletedIncrement, redactedIncrement := int64(0), int64(0)
	if spec.Action == saascompliance.DatasetActionDelete {
		deletedIncrement = affected
	} else if spec.Action == saascompliance.DatasetActionRedact {
		redactedIncrement = affected
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_requests
		SET completed_steps = (SELECT COUNT(*) FROM mochat_go_saas_erasure_steps WHERE request_id = ? AND status = 'succeeded'),
			deleted_rows = deleted_rows + ?, redacted_rows = redacted_rows + ?, updated_at = NOW()
		WHERE id = ?
	`, request.ID, deletedIncrement, redactedIncrement, request.ID); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	after, err := complianceErasureStepByKey(ctx, tx, request.ID, spec.Key, false)
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	return after, nil
}

func (s *MySQLStore) ApplyComplianceErasureArtifacts(ctx context.Context, request saascompliance.ErasureRequest, count int, now time.Time) (saascompliance.ErasureStep, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	defer tx.Rollback()
	step, err := complianceErasureStepByKey(ctx, tx, request.ID, "compliance_export_artifacts", true)
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	if step.Status == saascompliance.StepStatusSucceeded {
		if err := tx.Commit(); err != nil {
			return saascompliance.ErasureStep{}, err
		}
		return step, nil
	}
	var active int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_data_exports
		WHERE tenant_id = ? AND status IN ('pending', 'running')
	`, request.TenantID).Scan(&active); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	if active > 0 {
		return saascompliance.ErasureStep{}, saascompliance.Conflict("租户仍有运行中的数据导出")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_data_exports
		SET status = 'deleted', active_slot = NULL, deleted_at = COALESCE(deleted_at, ?), version = version + 1, updated_at = NOW()
		WHERE tenant_id = ? AND status <> 'deleted'
	`, now, request.TenantID); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_steps
		SET status = 'succeeded', affected_rows = ?, started_at = COALESCE(started_at, ?), finished_at = ?, error_message = '', updated_at = NOW()
		WHERE id = ?
	`, count, now, now, step.ID); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_requests
		SET completed_steps = (SELECT COUNT(*) FROM mochat_go_saas_erasure_steps WHERE request_id = ? AND status = 'succeeded'), updated_at = NOW()
		WHERE id = ?
	`, request.ID, request.ID); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	after, err := complianceErasureStepByKey(ctx, tx, request.ID, "compliance_export_artifacts", false)
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	return after, nil
}

func (s *MySQLStore) ApplyComplianceErasureStorageFiles(ctx context.Context, request saascompliance.ErasureRequest, count int, now time.Time) (saascompliance.ErasureStep, error) {
	return s.completeComplianceSpecialStep(ctx, request, "tenant_storage_files", count, now)
}

func (s *MySQLStore) completeComplianceSpecialStep(ctx context.Context, request saascompliance.ErasureRequest, stepKey string, count int, now time.Time) (saascompliance.ErasureStep, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	defer tx.Rollback()
	step, err := complianceErasureStepByKey(ctx, tx, request.ID, stepKey, true)
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	if step.Status == saascompliance.StepStatusSucceeded {
		if err := tx.Commit(); err != nil {
			return saascompliance.ErasureStep{}, err
		}
		return step, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_steps
		SET status = 'succeeded', affected_rows = ?, started_at = COALESCE(started_at, ?), finished_at = ?, error_message = '', updated_at = NOW()
		WHERE id = ?
	`, count, now, now, step.ID); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_requests
		SET completed_steps = (SELECT COUNT(*) FROM mochat_go_saas_erasure_steps WHERE request_id = ? AND status = 'succeeded'), updated_at = NOW()
		WHERE id = ?
	`, request.ID, request.ID); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	after, err := complianceErasureStepByKey(ctx, tx, request.ID, stepKey, false)
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	if err := tx.Commit(); err != nil {
		return saascompliance.ErasureStep{}, err
	}
	return after, nil
}

func (s *MySQLStore) FinalizeComplianceErasure(ctx context.Context, request saascompliance.ErasureRequest, anonymousRef, originalNameSHA, verificationSHA, reportJSON string, actor saascompliance.Actor, now time.Time) (saascompliance.ErasureRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	defer tx.Rollback()
	current, err := complianceErasureByID(ctx, tx, request.ID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return saascompliance.ErasureRequest{}, saascompliance.NotFound("租户数据擦除请求不存在")
	}
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if current.Status == saascompliance.ErasureStatusSucceeded {
		if err := tx.Commit(); err != nil {
			return saascompliance.ErasureRequest{}, err
		}
		return current, nil
	}
	var incomplete int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_saas_erasure_steps
		WHERE request_id = ? AND action NOT IN ('verify', 'tombstone') AND status <> 'succeeded'
	`, request.ID).Scan(&incomplete); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if incomplete != 0 {
		return saascompliance.ErasureRequest{}, saascompliance.Conflict("租户数据擦除仍有未完成步骤")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mc_tenant
		SET name = ?, status = 2, logo = '', login_background = '', url = '', copyright = '', server_ips = NULL,
			deleted_at = COALESCE(deleted_at, ?), updated_at = ?
		WHERE id = ?
	`, anonymousRef, now, now, request.TenantID)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return saascompliance.ErasureRequest{}, saascompliance.NotFound("待擦除租户不存在")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_tenant_tombstones
			(tenant_id, anonymous_ref, original_name_sha256, erasure_request_id, verification_sha256, erased_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE anonymous_ref = VALUES(anonymous_ref), verification_sha256 = VALUES(verification_sha256)
	`, request.TenantID, anonymousRef, originalNameSHA, request.ID, verificationSHA, now); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_legal_holds
		SET reason = '[redacted by tenant erasure]', release_reason = CASE WHEN release_reason = '' THEN '' ELSE '[redacted by tenant erasure]' END,
			active_slot = NULL, status = CASE WHEN status = 'active' THEN 'released' ELSE status END,
			updated_at = NOW()
		WHERE tenant_id = ?
	`, request.TenantID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_data_exports
		SET tenant_name_snapshot = ?, request_reason = '[redacted by tenant erasure]', status = 'deleted',
			active_slot = NULL, deleted_at = COALESCE(deleted_at, ?), updated_at = NOW()
		WHERE tenant_id = ?
	`, anonymousRef, now, request.TenantID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_requests
		SET tenant_name_snapshot = ?, reason = '[redacted by tenant erasure]'
		WHERE tenant_id = ?
	`, anonymousRef, request.TenantID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_steps
		SET status = 'succeeded', affected_rows = CASE WHEN action = 'tombstone' THEN 1 ELSE affected_rows END,
			started_at = COALESCE(started_at, ?), finished_at = ?, error_message = '', updated_at = NOW()
		WHERE request_id = ? AND action IN ('verify', 'tombstone')
	`, now, now, request.ID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_requests
		SET status = 'succeeded', active_slot = NULL, completed_steps = total_steps,
			verification_sha256 = ?, report_json = ?, last_error = '', finished_at = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ?
	`, verificationSHA, reportJSON, now, request.ID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	operationID, err := insertComplianceOperationLog(ctx, tx, request.TenantID, actor, saascompliance.OperationActionErasureComplete,
		"saas_erasure_request", request.RequestNo, anonymousRef, nil,
		map[string]any{"tenantId": request.TenantID, "anonymousRef": anonymousRef, "verificationSha256": verificationSHA},
		"租户业务数据已擦除并完成逐步校验")
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_erasure_requests SET operation_id = ? WHERE id = ?`, operationID, request.ID); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	after, err := complianceErasureByID(ctx, tx, request.ID, false)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	after.OperationID = operationID
	if err := tx.Commit(); err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	return after, nil
}

func (s *MySQLStore) BlockComplianceErasure(ctx context.Context, requestID int64, message string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_requests
		SET status = 'blocked', last_error = ?, finished_at = NULL, version = version + 1, updated_at = ?
		WHERE id = ? AND status NOT IN ('succeeded', 'canceled')
	`, truncateComplianceError(message), now, requestID)
	return err
}

func (s *MySQLStore) FailComplianceErasure(ctx context.Context, requestID int64, message string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_steps
		SET status = 'failed', error_message = ?, finished_at = ?, updated_at = NOW()
		WHERE request_id = ? AND status = 'running'
	`, truncateComplianceError(message), now, requestID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_erasure_requests
		SET status = 'failed', last_error = ?, finished_at = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND status NOT IN ('succeeded', 'canceled')
	`, truncateComplianceError(message), now, requestID); err != nil {
		return err
	}
	return tx.Commit()
}

const complianceErasureSelect = `
	SELECT r.id, r.request_no, r.tenant_id, r.tenant_name_snapshot, r.status, r.reason,
		r.eligible_at, r.latest_export_id, r.approval_id, r.approved_at, r.approved_by,
		r.inventory_version, r.total_steps, r.completed_steps, r.deleted_rows, r.redacted_rows,
		r.verification_sha256, COALESCE(CAST(r.report_json AS CHAR), ''), r.last_error,
		r.requested_by, r.actor_tenant_id, r.started_at, r.finished_at, r.operation_id,
		r.version, r.created_at, r.updated_at
	FROM mochat_go_saas_erasure_requests r
`

func complianceErasureByID(ctx context.Context, queryer saasAdminAccessQueryer, id int64, forUpdate bool) (saascompliance.ErasureRequest, error) {
	query := complianceErasureSelect + " WHERE r.id = ?"
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := queryer.QueryContext(ctx, query, id)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return saascompliance.ErasureRequest{}, err
		}
		return saascompliance.ErasureRequest{}, sql.ErrNoRows
	}
	return scanComplianceErasure(rows)
}

func scanComplianceErasure(scanner interface{ Scan(...any) error }) (saascompliance.ErasureRequest, error) {
	var item saascompliance.ErasureRequest
	var eligibleAt time.Time
	var approvedAt, startedAt, finishedAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.RequestNo, &item.TenantID, &item.TenantName, &item.Status, &item.Reason,
		&eligibleAt, &item.LatestExportID, &item.ApprovalID, &approvedAt, &item.ApprovedBy,
		&item.InventoryVersion, &item.TotalSteps, &item.CompletedSteps, &item.DeletedRows, &item.RedactedRows,
		&item.VerificationSHA256, &item.ReportJSON, &item.LastError,
		&item.RequestedBy, &item.ActorTenantID, &startedAt, &finishedAt, &item.OperationID,
		&item.Version, &createdAt, &updatedAt)
	if err != nil {
		return saascompliance.ErasureRequest{}, err
	}
	item.EligibleAtValue, item.EligibleAt = eligibleAt, eligibleAt.Format("2006-01-02 15:04:05")
	item.ApprovedAt, item.StartedAt, item.FinishedAt = formatTime(approvedAt), formatTime(startedAt), formatTime(finishedAt)
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, nil
}

func complianceErasureStepByKey(ctx context.Context, queryer saasAdminAccessQueryer, requestID int64, key string, forUpdate bool) (saascompliance.ErasureStep, error) {
	query := `
		SELECT id, request_id, step_order, step_key, table_name, action, status, affected_rows,
			started_at, finished_at, error_message
		FROM mochat_go_saas_erasure_steps WHERE request_id = ? AND step_key = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := queryer.QueryContext(ctx, query, requestID, key)
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return saascompliance.ErasureStep{}, err
		}
		return saascompliance.ErasureStep{}, sql.ErrNoRows
	}
	return scanComplianceErasureStep(rows)
}

func scanComplianceErasureStep(scanner interface{ Scan(...any) error }) (saascompliance.ErasureStep, error) {
	var item saascompliance.ErasureStep
	var startedAt, finishedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.RequestID, &item.StepOrder, &item.StepKey, &item.TableName,
		&item.Action, &item.Status, &item.AffectedRows, &startedAt, &finishedAt, &item.ErrorMessage)
	if err != nil {
		return saascompliance.ErasureStep{}, err
	}
	item.StartedAt, item.FinishedAt = formatTime(startedAt), formatTime(finishedAt)
	return item, nil
}

type complianceRowsAffected int64

func (r complianceRowsAffected) LastInsertId() (int64, error) { return 0, nil }
func (r complianceRowsAffected) RowsAffected() (int64, error) { return int64(r), nil }
func complianceStaticResult(value int64) sql.Result           { return complianceRowsAffected(value) }

func complianceTenantArgs(predicate string, tenantID int) []any {
	count := strings.Count(predicate, "?")
	args := make([]any, count)
	for index := range args {
		args[index] = tenantID
	}
	return args
}

func nullableTimeValue(value sql.NullTime) time.Time {
	if value.Valid {
		return value.Time
	}
	return time.Time{}
}

func insertComplianceOperationLog(ctx context.Context, tx *sql.Tx, tenantID int, actor saascompliance.Actor, action, targetType, targetID, targetName string, before, after any, remark string) (int64, error) {
	beforeJSON, err := marshalComplianceSnapshot(before)
	if err != nil {
		return 0, err
	}
	afterJSON, err := marshalComplianceSnapshot(after)
	if err != nil {
		return 0, err
	}
	return insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: tenantID, ActorUserID: actor.UserID, ActorTenantID: actor.TenantID,
		Action: action, TargetType: targetType, TargetID: targetID, TargetName: targetName,
		BeforeJSON: beforeJSON, AfterJSON: afterJSON, Remark: remark,
	})
}

func marshalComplianceSnapshot(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func truncateComplianceError(value string) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 1000 {
		return string([]rune(value)[:1000])
	}
	return value
}

var _ saascompliance.Store = (*MySQLStore)(nil)
