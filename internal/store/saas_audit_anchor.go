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
	"jiyi/mochat-go/internal/saasauditanchor"
)

func (s *MySQLStore) VerifyAuditAnchorChains(ctx context.Context, options saasauditanchor.CreateOptions) (saasauditanchor.AuditVerification, error) {
	verified, err := s.VerifySaaSAdminAuditIntegrity(ctx, dashboard.SaaSAdminAuditIntegrityOptions{
		TenantID: options.TenantID, Limit: options.Limit, Source: options.Source,
		ActorUserID: options.Actor.UserID, ActorTenantID: options.Actor.TenantID,
	})
	if err != nil {
		return saasauditanchor.AuditVerification{}, err
	}
	result := saasauditanchor.AuditVerification{
		ScannedChains: verified.ScannedChains, HealthyChains: verified.HealthyChains,
		FailedChains: verified.FailedChains, Chains: make([]saasauditanchor.Chain, 0, len(verified.Chains)),
	}
	for _, chain := range verified.Chains {
		result.Chains = append(result.Chains, saasauditanchor.Chain{
			TenantID: chain.TenantID, TenantName: chain.TenantName, ChainVersion: saasAuditIntegrityVersion,
			AnchorLogID: chain.AnchorLogID, AnchorHash: chain.AnchorHash, LegacyLogCount: chain.LegacyLogCount,
			ChainHeadLogID: chain.LastLogID, ChainHeadHash: chain.LastHash,
			SignedLogCount: chain.SignedLogCount, Status: chain.Status,
		})
	}
	return result, nil
}

func (s *MySQLStore) AuditAnchorCheckpointSummary(ctx context.Context, tenantID int) (saasauditanchor.CheckpointSummary, error) {
	var result saasauditanchor.CheckpointSummary
	var latestSignedAt sql.NullString
	var latestVerifiedAt, latestRemoteVerifiedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(artifact_status = 'exported'), 0),
			COALESCE(SUM(artifact_status = 'failed'), 0),
			COALESCE(SUM(verification_status = 'passed'), 0),
				COALESCE(SUM(verification_status = 'failed'), 0),
				COALESCE(SUM(verification_status = 'pending'), 0),
				COALESCE(SUM(remote_status = 'exported'), 0),
				COALESCE(SUM(remote_status = 'failed'), 0),
				COALESCE(SUM(remote_status = 'pending'), 0),
				MAX(signed_at), MAX(last_verified_at), MAX(remote_verified_at)
		FROM mochat_go_saas_admin_audit_anchor_checkpoints
		WHERE (? = 0 OR tenant_id = ?)
	`, tenantID, tenantID).Scan(
		&result.CheckpointCount, &result.ExportedCount, &result.ArtifactFailedCount,
		&result.VerificationPassedCount, &result.VerificationFailedCount,
		&result.PendingVerificationCount, &result.RemoteExportedCount, &result.RemoteFailedCount,
		&result.RemotePendingCount, &latestSignedAt, &latestVerifiedAt, &latestRemoteVerifiedAt,
	)
	if err != nil {
		return saasauditanchor.CheckpointSummary{}, err
	}
	if latestSignedAt.Valid {
		result.LatestSignedAt = latestSignedAt.String
	}
	result.LatestVerifiedAt = formatTime(latestVerifiedAt)
	result.LatestRemoteVerifiedAt = formatTime(latestRemoteVerifiedAt)
	return result, nil
}

func (s *MySQLStore) AuditAnchorCheckpoints(ctx context.Context, tenantID, limit int) ([]saasauditanchor.Checkpoint, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, auditAnchorCheckpointSelect+`
		WHERE (? = 0 OR checkpoints.tenant_id = ?)
		ORDER BY checkpoints.id DESC
		LIMIT ?
	`, tenantID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saasauditanchor.Checkpoint, 0, limit)
	for rows.Next() {
		item, err := scanAuditAnchorCheckpoint(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) AuditAnchorCheckpointsPendingRemote(ctx context.Context, tenantID, limit int) ([]saasauditanchor.Checkpoint, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, auditAnchorCheckpointSelect+`
		WHERE (? = 0 OR checkpoints.tenant_id = ?)
			AND (checkpoints.remote_status <> 'exported' OR checkpoints.remote_object_key = '')
		ORDER BY checkpoints.id ASC
		LIMIT ?
	`, tenantID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]saasauditanchor.Checkpoint, 0, limit)
	for rows.Next() {
		item, err := scanAuditAnchorCheckpoint(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) AuditAnchorCheckpointKeyIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT key_id
		FROM mochat_go_saas_admin_audit_anchor_checkpoints
		ORDER BY key_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}

func (s *MySQLStore) AuditAnchorCheckpointNos(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT checkpoint_no
		FROM mochat_go_saas_admin_audit_anchor_checkpoints
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}

func (s *MySQLStore) AuditAnchorRemoteObjectKeys(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT remote_object_key
		FROM mochat_go_saas_admin_audit_anchor_checkpoints
		WHERE remote_object_key <> ''
		ORDER BY remote_object_key ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}

func (s *MySQLStore) AuditAnchorCheckpointByFingerprint(ctx context.Context, fingerprint string) (saasauditanchor.Checkpoint, bool, error) {
	item, err := scanAuditAnchorCheckpoint(s.db.QueryRowContext(ctx, auditAnchorCheckpointSelect+`
		WHERE checkpoints.checkpoint_fingerprint = ?
		LIMIT 1
	`, strings.TrimSpace(fingerprint)))
	if errors.Is(err, sql.ErrNoRows) {
		return saasauditanchor.Checkpoint{}, false, nil
	}
	return item, err == nil, err
}

func (s *MySQLStore) CreateAuditAnchorCheckpoint(ctx context.Context, input saasauditanchor.CheckpointCreate) (saasauditanchor.Checkpoint, bool, error) {
	item := input.Checkpoint
	result, err := s.db.ExecContext(ctx, `
		INSERT IGNORE INTO mochat_go_saas_admin_audit_anchor_checkpoints
			(checkpoint_no, checkpoint_fingerprint, tenant_id, chain_version,
			 anchor_log_id, anchor_hash, legacy_log_count, chain_head_log_id, chain_head_hash,
			 signed_log_count, signature_algorithm, key_id, payload_sha256, signature,
			 artifact_name, artifact_status, remote_status, verification_status, source,
			 actor_user_id, actor_tenant_id, signed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, item.CheckpointNo, item.Fingerprint, item.TenantID, item.ChainVersion,
		item.AnchorLogID, item.AnchorHash, item.LegacyLogCount, item.ChainHeadLogID, item.ChainHeadHash,
		item.SignedLogCount, item.SignatureAlgorithm, item.KeyID, item.PayloadSHA256, item.Signature,
		item.ArtifactName, saasauditanchor.ArtifactStatusPending, normalizeAuditAnchorRemoteStatus(item.RemoteStatus),
		saasauditanchor.VerifyStatusPending,
		item.Source, item.ActorUserID, item.ActorTenantID, item.SignedAt)
	if err != nil {
		return saasauditanchor.Checkpoint{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return saasauditanchor.Checkpoint{}, false, err
	}
	stored, ok, err := s.AuditAnchorCheckpointByFingerprint(ctx, item.Fingerprint)
	if err != nil {
		return saasauditanchor.Checkpoint{}, false, err
	}
	if !ok {
		return saasauditanchor.Checkpoint{}, false, fmt.Errorf("audit anchor checkpoint was not persisted")
	}
	return stored, affected > 0, nil
}

func normalizeAuditAnchorRemoteStatus(value string) string {
	switch strings.TrimSpace(value) {
	case saasauditanchor.RemoteStatusPending, saasauditanchor.RemoteStatusExported, saasauditanchor.RemoteStatusFailed:
		return strings.TrimSpace(value)
	default:
		return saasauditanchor.RemoteStatusDisabled
	}
}

func (s *MySQLStore) UpdateAuditAnchorArtifact(ctx context.Context, update saasauditanchor.ArtifactUpdate) (saasauditanchor.Checkpoint, error) {
	var exportedAt any
	if !update.ExportedAt.IsZero() {
		exportedAt = update.ExportedAt.Format(saasAuditTimeLayout)
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_audit_anchor_checkpoints
		SET artifact_status = ?, artifact_name = ?, artifact_sha256 = ?, artifact_error = ?,
			exported_at = ?, updated_at = NOW()
		WHERE id = ?
	`, update.Status, truncateRunes(strings.TrimSpace(update.ArtifactName), 255),
		strings.ToLower(strings.TrimSpace(update.ArtifactSHA256)), truncateRunes(update.ErrorMessage, 255),
		exportedAt, update.ID)
	if err != nil {
		return saasauditanchor.Checkpoint{}, err
	}
	return s.auditAnchorCheckpointByID(ctx, update.ID)
}

func (s *MySQLStore) UpdateAuditAnchorRemoteArtifact(ctx context.Context, update saasauditanchor.RemoteArtifactUpdate) (saasauditanchor.Checkpoint, error) {
	current, err := s.auditAnchorCheckpointByID(ctx, update.ID)
	if err != nil {
		return saasauditanchor.Checkpoint{}, err
	}
	artifact := update.Artifact
	if artifact.Provider == "" {
		artifact.Provider = current.RemoteProvider
	}
	if artifact.Bucket == "" {
		artifact.Bucket = current.RemoteBucket
	}
	if artifact.ObjectKey == "" {
		artifact.ObjectKey = current.RemoteObjectKey
	}
	if artifact.ETag == "" {
		artifact.ETag = current.RemoteETag
	}
	if artifact.VersionID == "" {
		artifact.VersionID = current.RemoteVersionID
	}
	if artifact.SHA256 == "" {
		artifact.SHA256 = current.RemoteSHA256
	}
	if artifact.SizeBytes == 0 {
		artifact.SizeBytes = current.RemoteSizeBytes
	}
	if artifact.RetentionMode == "" {
		artifact.RetentionMode = current.RemoteRetentionMode
	}
	if artifact.RetainUntil == "" {
		artifact.RetainUntil = current.RemoteRetainUntil
	}
	var exportedAt, verifiedAt any
	if !artifact.ExportedAt.IsZero() {
		exportedAt = artifact.ExportedAt.Format(saasAuditTimeLayout)
	} else if current.RemoteExportedAt != "" {
		if value, parseErr := time.Parse(time.RFC3339, current.RemoteExportedAt); parseErr == nil {
			exportedAt = value.Format(saasAuditTimeLayout)
		}
	}
	if !artifact.VerifiedAt.IsZero() {
		verifiedAt = artifact.VerifiedAt.Format(saasAuditTimeLayout)
	} else if current.RemoteVerifiedAt != "" {
		if value, parseErr := time.Parse(time.RFC3339, current.RemoteVerifiedAt); parseErr == nil {
			verifiedAt = value.Format(saasAuditTimeLayout)
		}
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_audit_anchor_checkpoints
		SET remote_status = ?, remote_provider = ?, remote_bucket = ?, remote_object_key = ?,
			remote_etag = ?, remote_version_id = ?, remote_sha256 = ?, remote_size_bytes = ?,
			remote_retention_mode = ?, remote_retain_until = ?, remote_error = ?,
			remote_exported_at = ?, remote_verified_at = ?, updated_at = NOW()
		WHERE id = ?
	`, normalizeAuditAnchorRemoteStatus(update.Status), truncateRunes(strings.TrimSpace(artifact.Provider), 24),
		truncateRunes(strings.TrimSpace(artifact.Bucket), 255), truncateRunes(strings.TrimSpace(artifact.ObjectKey), 512),
		truncateRunes(strings.TrimSpace(artifact.ETag), 128), truncateRunes(strings.TrimSpace(artifact.VersionID), 255),
		strings.ToLower(strings.TrimSpace(artifact.SHA256)), artifact.SizeBytes,
		truncateRunes(strings.ToLower(strings.TrimSpace(artifact.RetentionMode)), 16),
		truncateRunes(strings.TrimSpace(artifact.RetainUntil), 20), truncateRunes(update.ErrorMessage, 255),
		exportedAt, verifiedAt, update.ID)
	if err != nil {
		return saasauditanchor.Checkpoint{}, err
	}
	return s.auditAnchorCheckpointByID(ctx, update.ID)
}

func (s *MySQLStore) UpdateAuditAnchorVerification(ctx context.Context, update saasauditanchor.VerificationUpdate) (saasauditanchor.Checkpoint, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_audit_anchor_checkpoints
		SET verification_status = ?, verification_error = ?, last_verified_at = ?, updated_at = NOW()
		WHERE id = ?
	`, update.Status, truncateRunes(update.ErrorMessage, 255), update.VerifiedAt.Format(saasAuditTimeLayout), update.ID)
	if err != nil {
		return saasauditanchor.Checkpoint{}, err
	}
	return s.auditAnchorCheckpointByID(ctx, update.ID)
}

func (s *MySQLStore) CheckAuditAnchorCheckpointChain(ctx context.Context, item saasauditanchor.Checkpoint) error {
	var anchorLogID, legacyLogCount, lastLogID, signedLogCount int64
	var anchorHash, lastHash, status string
	if err := s.db.QueryRowContext(ctx, `
		SELECT anchor_log_id, anchor_hash, legacy_log_count, last_log_id, last_hash, signed_log_count, status
		FROM mochat_go_saas_admin_audit_chains
		WHERE tenant_id = ?
	`, item.TenantID).Scan(&anchorLogID, &anchorHash, &legacyLogCount, &lastLogID, &lastHash, &signedLogCount, &status); errors.Is(err, sql.ErrNoRows) {
		return saasauditanchor.Conflict("审计摘要链状态不存在")
	} else if err != nil {
		return err
	}
	if status == dashboard.SaaSAdminAuditIntegrityStatusFailed {
		return saasauditanchor.Conflict("审计摘要链当前处于异常状态")
	}
	if anchorLogID != item.AnchorLogID || anchorHash != item.AnchorHash || legacyLogCount != item.LegacyLogCount {
		return saasauditanchor.Conflict("审计摘要链 legacy 锚点与签名检查点不一致")
	}
	if lastLogID < item.ChainHeadLogID || signedLogCount < item.SignedLogCount {
		return saasauditanchor.Conflict("审计摘要链发生回退或日志计数减少")
	}
	if item.SignedLogCount == 0 {
		if item.ChainHeadLogID != item.AnchorLogID || item.ChainHeadHash != item.AnchorHash {
			return saasauditanchor.Conflict("审计签名检查点链头与 legacy 锚点不一致")
		}
		return nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_saas_admin_operation_logs
		WHERE id = ? AND tenant_id = ? AND integrity_version = ? AND integrity_hash = ? AND deleted_at IS NULL
	`, item.ChainHeadLogID, item.TenantID, item.ChainVersion, item.ChainHeadHash).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return saasauditanchor.Conflict("审计签名检查点对应的链头日志不存在或摘要不一致")
	}
	return nil
}

func (s *MySQLStore) RecordAuditAnchorOperation(ctx context.Context, record saasauditanchor.OperationRecord) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	targetID := "all"
	if record.TenantID > 0 {
		targetID = strconv.Itoa(record.TenantID)
	}
	after, _ := json.Marshal(map[string]any{
		"tenantId": record.TenantID, "source": record.Source,
		"affected": record.Affected, "failed": record.Failed,
	})
	name, remark := "创建审计签名锚点", "创建 HMAC 签名检查点并写入独立证据文件"
	if record.Action == saasauditanchor.OperationActionVerify {
		name, remark = "校验审计签名锚点", "校验 HMAC 签名、独立证据文件和数据库摘要链"
	}
	id, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: record.TenantID, ActorUserID: record.Actor.UserID, ActorTenantID: record.Actor.TenantID,
		Action: record.Action, TargetType: saasauditanchor.OperationTarget, TargetID: targetID,
		TargetName: name, AfterJSON: string(after), Remark: remark,
	})
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *MySQLStore) auditAnchorCheckpointByID(ctx context.Context, id int64) (saasauditanchor.Checkpoint, error) {
	item, err := scanAuditAnchorCheckpoint(s.db.QueryRowContext(ctx, auditAnchorCheckpointSelect+`
		WHERE checkpoints.id = ?
		LIMIT 1
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return saasauditanchor.Checkpoint{}, saasauditanchor.NotFound("审计锚点不存在")
	}
	return item, err
}

const auditAnchorCheckpointSelect = `
	SELECT checkpoints.id, checkpoints.checkpoint_no, checkpoints.checkpoint_fingerprint,
		checkpoints.tenant_id, COALESCE(tenants.name, ''), checkpoints.chain_version,
		checkpoints.anchor_log_id, checkpoints.anchor_hash, checkpoints.legacy_log_count,
		checkpoints.chain_head_log_id, checkpoints.chain_head_hash, checkpoints.signed_log_count,
		checkpoints.signature_algorithm, checkpoints.key_id, checkpoints.payload_sha256,
			checkpoints.signature, checkpoints.artifact_name, checkpoints.artifact_sha256,
			checkpoints.artifact_status, checkpoints.artifact_error,
			checkpoints.remote_status, checkpoints.remote_provider, checkpoints.remote_bucket,
			checkpoints.remote_object_key, checkpoints.remote_etag, checkpoints.remote_version_id,
			checkpoints.remote_sha256, checkpoints.remote_size_bytes, checkpoints.remote_retention_mode,
			checkpoints.remote_retain_until, checkpoints.remote_error,
			checkpoints.remote_exported_at, checkpoints.remote_verified_at,
			checkpoints.verification_status, checkpoints.verification_error, checkpoints.source,
		checkpoints.actor_user_id, checkpoints.actor_tenant_id, checkpoints.signed_at,
		checkpoints.exported_at, checkpoints.last_verified_at, checkpoints.created_at
	FROM mochat_go_saas_admin_audit_anchor_checkpoints checkpoints
	LEFT JOIN mc_tenant tenants ON tenants.id = checkpoints.tenant_id
`

func scanAuditAnchorCheckpoint(scanner interface{ Scan(...any) error }) (saasauditanchor.Checkpoint, error) {
	var item saasauditanchor.Checkpoint
	var exportedAt, remoteExportedAt, remoteVerifiedAt, lastVerifiedAt, createdAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.CheckpointNo, &item.Fingerprint, &item.TenantID, &item.TenantName,
		&item.ChainVersion, &item.AnchorLogID, &item.AnchorHash, &item.LegacyLogCount,
		&item.ChainHeadLogID, &item.ChainHeadHash, &item.SignedLogCount,
		&item.SignatureAlgorithm, &item.KeyID, &item.PayloadSHA256, &item.Signature,
		&item.ArtifactName, &item.ArtifactSHA256, &item.ArtifactStatus, &item.ArtifactError,
		&item.RemoteStatus, &item.RemoteProvider, &item.RemoteBucket, &item.RemoteObjectKey,
		&item.RemoteETag, &item.RemoteVersionID, &item.RemoteSHA256, &item.RemoteSizeBytes,
		&item.RemoteRetentionMode, &item.RemoteRetainUntil, &item.RemoteError,
		&remoteExportedAt, &remoteVerifiedAt,
		&item.VerificationStatus, &item.VerificationError, &item.Source,
		&item.ActorUserID, &item.ActorTenantID, &item.SignedAt,
		&exportedAt, &lastVerifiedAt, &createdAt,
	)
	if err != nil {
		return saasauditanchor.Checkpoint{}, err
	}
	item.TenantName = saasAuditTenantName(item.TenantID, item.TenantName)
	item.ExportedAt = formatTime(exportedAt)
	item.RemoteExportedAt = formatTime(remoteExportedAt)
	item.RemoteVerifiedAt = formatTime(remoteVerifiedAt)
	item.LastVerifiedAt = formatTime(lastVerifiedAt)
	item.CreatedAt = formatTime(createdAt)
	return item, nil
}

var _ saasauditanchor.Store = (*MySQLStore)(nil)
