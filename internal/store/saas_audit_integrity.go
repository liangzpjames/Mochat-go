package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

const (
	saasAuditIntegrityVersion = 1
	saasAuditGenesisHash      = "0000000000000000000000000000000000000000000000000000000000000000"
	saasAuditTimeLayout       = "2006-01-02 15:04:05"
)

type saasAuditStructuralLog struct {
	ID            int64  `json:"id"`
	TenantID      int    `json:"tenantId"`
	ActorUserID   int    `json:"actorUserId"`
	ActorTenantID int    `json:"actorTenantId"`
	Action        string `json:"action"`
	TargetType    string `json:"targetType"`
	TargetID      string `json:"targetId"`
	CreatedAt     string `json:"createdAt"`
}

type saasAuditChainState struct {
	TenantID               int
	AnchorLogID            int64
	AnchorHash             string
	LegacyLogCount         int64
	LastLogID              int64
	LastHash               string
	SignedLogCount         int64
	Status                 string
	SealedAt               sql.NullTime
	LastVerifiedAt         sql.NullTime
	LastVerificationStatus string
	LastFailedLogID        int64
	LastVerificationError  string
	Version                int
}

type saasAuditVerificationOutcome struct {
	Chain       dashboard.SaaSAdminAuditIntegrityChain
	Sealed      bool
	Healthy     bool
	Verified    int64
	StartedAt   time.Time
	FinishedAt  time.Time
	FailedLogID int64
	Failure     string
}

func (s *MySQLStore) SaaSAdminAuditIntegrityOverview(ctx context.Context, options dashboard.SaaSAdminAuditIntegrityOptions) (dashboard.SaaSAdminAuditIntegrityOverview, error) {
	if options.Limit <= 0 {
		options.Limit = 50
	}
	if options.VerificationLimit <= 0 {
		options.VerificationLimit = 20
	}
	result := dashboard.SaaSAdminAuditIntegrityOverview{
		Chains:        []dashboard.SaaSAdminAuditIntegrityChain{},
		Verifications: []dashboard.SaaSAdminAuditIntegrityVerification{},
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(status = 'healthy'), 0),
			COALESCE(SUM(status = 'failed'), 0),
			COALESCE(SUM(legacy_log_count), 0),
			COALESCE(SUM(signed_log_count), 0)
		FROM mochat_go_saas_admin_audit_chains
		WHERE (? = 0 OR tenant_id = ?)
	`, options.TenantID, options.TenantID).Scan(
		&result.Summary.ChainCount,
		&result.Summary.HealthyChainCount,
		&result.Summary.FailedChainCount,
		&result.Summary.LegacyLogCount,
		&result.Summary.SignedLogCount,
	); err != nil {
		return dashboard.SaaSAdminAuditIntegrityOverview{}, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT DISTINCT tenant_id
			FROM mochat_go_saas_admin_operation_logs
			WHERE deleted_at IS NULL AND (? = 0 OR tenant_id = ?)
		) logs
		LEFT JOIN mochat_go_saas_admin_audit_chains chains ON chains.tenant_id = logs.tenant_id
		WHERE chains.tenant_id IS NULL OR chains.status = 'unsealed'
	`, options.TenantID, options.TenantID).Scan(&result.Summary.UnsealedTenantCount); err != nil {
		return dashboard.SaaSAdminAuditIntegrityOverview{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT chains.tenant_id, COALESCE(tenants.name, ''), chains.anchor_log_id, chains.anchor_hash,
			chains.legacy_log_count, chains.last_log_id, chains.last_hash, chains.signed_log_count,
			chains.status, chains.sealed_at, chains.last_verified_at, chains.last_verification_status,
			chains.last_failed_log_id, chains.last_verification_error, chains.version
		FROM mochat_go_saas_admin_audit_chains chains
		LEFT JOIN mc_tenant tenants ON tenants.id = chains.tenant_id
		WHERE (? = 0 OR chains.tenant_id = ?)
		ORDER BY (chains.status = 'failed') DESC, chains.last_verified_at ASC, chains.tenant_id ASC
		LIMIT ?
	`, options.TenantID, options.TenantID, options.Limit)
	if err != nil {
		return dashboard.SaaSAdminAuditIntegrityOverview{}, err
	}
	for rows.Next() {
		item, err := scanSaaSAuditChain(rows)
		if err != nil {
			rows.Close()
			return dashboard.SaaSAdminAuditIntegrityOverview{}, err
		}
		result.Chains = append(result.Chains, item)
	}
	if err := rows.Close(); err != nil {
		return dashboard.SaaSAdminAuditIntegrityOverview{}, err
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminAuditIntegrityOverview{}, err
	}

	verificationRows, err := s.db.QueryContext(ctx, `
		SELECT verifications.id, verifications.tenant_id, COALESCE(tenants.name, ''), verifications.source,
			verifications.status, verifications.anchor_log_id, verifications.anchor_hash,
			verifications.legacy_log_count, verifications.signed_log_count, verifications.verified_log_count,
			verifications.chain_head_log_id, verifications.chain_head_hash, verifications.failed_log_id,
			verifications.error_message, verifications.actor_user_id, verifications.actor_tenant_id,
			verifications.started_at, verifications.finished_at
		FROM mochat_go_saas_admin_audit_verifications verifications
		LEFT JOIN mc_tenant tenants ON tenants.id = verifications.tenant_id
		WHERE (? = 0 OR verifications.tenant_id = ?)
		ORDER BY verifications.id DESC
		LIMIT ?
	`, options.TenantID, options.TenantID, options.VerificationLimit)
	if err != nil {
		return dashboard.SaaSAdminAuditIntegrityOverview{}, err
	}
	for verificationRows.Next() {
		var item dashboard.SaaSAdminAuditIntegrityVerification
		var startedAt, finishedAt sql.NullTime
		if err := verificationRows.Scan(
			&item.ID, &item.TenantID, &item.TenantName, &item.Source, &item.Status,
			&item.AnchorLogID, &item.AnchorHash, &item.LegacyLogCount, &item.SignedLogCount,
			&item.VerifiedLogCount, &item.ChainHeadLogID, &item.ChainHeadHash, &item.FailedLogID,
			&item.ErrorMessage, &item.ActorUserID, &item.ActorTenantID, &startedAt, &finishedAt,
		); err != nil {
			verificationRows.Close()
			return dashboard.SaaSAdminAuditIntegrityOverview{}, err
		}
		item.TenantName = saasAuditTenantName(item.TenantID, item.TenantName)
		item.StartedAt, item.FinishedAt = formatTime(startedAt), formatTime(finishedAt)
		result.Verifications = append(result.Verifications, item)
	}
	if err := verificationRows.Close(); err != nil {
		return dashboard.SaaSAdminAuditIntegrityOverview{}, err
	}
	if err := verificationRows.Err(); err != nil {
		return dashboard.SaaSAdminAuditIntegrityOverview{}, err
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT audit_retention_days
		FROM mochat_go_saas_compliance_policies
		WHERE id = 1
	`).Scan(&result.Summary.RetentionDays); err != nil {
		return dashboard.SaaSAdminAuditIntegrityOverview{}, err
	}
	cutoff := time.Now().AddDate(0, 0, -result.Summary.RetentionDays).Truncate(time.Second)
	result.Summary.RetentionCutoff = cutoff.Format(saasAuditTimeLayout)
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_saas_admin_operation_logs
		WHERE deleted_at IS NULL AND created_at < ? AND (? = 0 OR tenant_id = ?)
	`, result.Summary.RetentionCutoff, options.TenantID, options.TenantID).Scan(&result.Summary.RetentionEligibleLogCount); err != nil {
		return dashboard.SaaSAdminAuditIntegrityOverview{}, err
	}
	return result, nil
}

func (s *MySQLStore) VerifySaaSAdminAuditIntegrity(ctx context.Context, options dashboard.SaaSAdminAuditIntegrityOptions) (dashboard.SaaSAdminAuditIntegrityVerifyResult, error) {
	if options.Limit <= 0 {
		options.Limit = 100
	}
	if options.Source == "" {
		options.Source = "manual"
	}
	result := dashboard.SaaSAdminAuditIntegrityVerifyResult{Chains: []dashboard.SaaSAdminAuditIntegrityChain{}}
	if options.RecordOperation {
		operationID, err := s.recordSaaSAuditVerificationOperation(ctx, options)
		if err != nil {
			return dashboard.SaaSAdminAuditIntegrityVerifyResult{}, err
		}
		result.OperationID = operationID
	}
	tenantIDs, err := s.saasAuditIntegrityTenantIDs(ctx, options.TenantID, options.Limit)
	if err != nil {
		return dashboard.SaaSAdminAuditIntegrityVerifyResult{}, err
	}
	for _, tenantID := range tenantIDs {
		outcome, err := s.verifySaaSAuditIntegrityTenant(ctx, tenantID, options)
		if err != nil {
			return dashboard.SaaSAdminAuditIntegrityVerifyResult{}, err
		}
		result.ScannedChains++
		if outcome.Sealed {
			result.SealedChains++
		}
		if outcome.Healthy {
			result.HealthyChains++
		} else {
			result.FailedChains++
		}
		result.LegacyLogs += outcome.Chain.LegacyLogCount
		result.SignedLogs += outcome.Chain.SignedLogCount
		result.VerifiedLogs += outcome.Verified
		result.VerifiedAt = outcome.FinishedAt.Format(saasAuditTimeLayout)
		result.Chains = append(result.Chains, outcome.Chain)
	}
	if result.VerifiedAt == "" {
		result.VerifiedAt = time.Now().Format(saasAuditTimeLayout)
	}
	return result, nil
}

func (s *MySQLStore) recordSaaSAuditVerificationOperation(ctx context.Context, options dashboard.SaaSAdminAuditIntegrityOptions) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	targetID := "all"
	if options.TenantID > 0 {
		targetID = strconv.Itoa(options.TenantID)
	}
	after, _ := json.Marshal(map[string]any{
		"tenantId": options.TenantID,
		"limit":    options.Limit,
		"source":   options.Source,
	})
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID:      options.TenantID,
		ActorUserID:   options.ActorUserID,
		ActorTenantID: options.ActorTenantID,
		Action:        dashboard.SaaSAdminOperationActionAuditIntegrityVerify,
		TargetType:    dashboard.SaaSAdminOperationTargetAuditIntegrity,
		TargetID:      targetID,
		TargetName:    "审计完整性校验",
		AfterJSON:     string(after),
		Remark:        "执行 SaaS 审计完整性封存和校验",
	})
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return operationID, nil
}

func (s *MySQLStore) saasAuditIntegrityTenantIDs(ctx context.Context, tenantID int, limit int) ([]int, error) {
	if tenantID > 0 {
		return []int{tenantID}, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT tenant_id
		FROM (
			SELECT 0 AS tenant_id
			UNION
			SELECT tenant_id FROM mochat_go_saas_admin_operation_logs WHERE deleted_at IS NULL
			UNION
			SELECT tenant_id FROM mochat_go_saas_admin_audit_chains
		) tenants
		ORDER BY tenant_id ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]int, 0, limit)
	for rows.Next() {
		var value int
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}

func (s *MySQLStore) verifySaaSAuditIntegrityTenant(ctx context.Context, tenantID int, options dashboard.SaaSAdminAuditIntegrityOptions) (saasAuditVerificationOutcome, error) {
	startedAt := time.Now().Truncate(time.Second)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return saasAuditVerificationOutcome{}, err
	}
	defer tx.Rollback()
	state, sealed, err := ensureSaaSAuditChainTx(ctx, tx, tenantID)
	if err != nil {
		return saasAuditVerificationOutcome{}, err
	}
	healthy, verified, failedLogID, failure, err := verifySaaSAuditChainStateTx(ctx, tx, state)
	if err != nil {
		return saasAuditVerificationOutcome{}, err
	}
	finishedAt := time.Now().Truncate(time.Second)
	status := dashboard.SaaSAdminAuditIntegrityStatusHealthy
	if !healthy {
		status = dashboard.SaaSAdminAuditIntegrityStatusFailed
	}
	failure = truncateRunes(strings.TrimSpace(failure), 255)
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_audit_chains
		SET status = ?, last_verified_at = ?, last_verification_status = ?,
			last_failed_log_id = ?, last_verification_error = ?, version = version + 1, updated_at = NOW()
		WHERE tenant_id = ?
	`, status, finishedAt.Format(saasAuditTimeLayout), status, failedLogID, failure, tenantID); err != nil {
		return saasAuditVerificationOutcome{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_audit_verifications
			(tenant_id, source, status, anchor_log_id, anchor_hash, legacy_log_count,
			 signed_log_count, verified_log_count, chain_head_log_id, chain_head_hash,
			 failed_log_id, error_message, actor_user_id, actor_tenant_id, started_at, finished_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
	`, tenantID, truncateRunes(strings.TrimSpace(options.Source), 24), status,
		state.AnchorLogID, state.AnchorHash, state.LegacyLogCount, state.SignedLogCount, verified,
		state.LastLogID, state.LastHash, failedLogID, failure, options.ActorUserID,
		options.ActorTenantID, startedAt.Format(saasAuditTimeLayout), finishedAt.Format(saasAuditTimeLayout)); err != nil {
		return saasAuditVerificationOutcome{}, err
	}
	if err := tx.Commit(); err != nil {
		return saasAuditVerificationOutcome{}, err
	}
	state.Status = status
	state.LastVerifiedAt = sql.NullTime{Time: finishedAt, Valid: true}
	state.LastVerificationStatus = status
	state.LastFailedLogID = failedLogID
	state.LastVerificationError = failure
	state.Version++
	return saasAuditVerificationOutcome{
		Chain:       saasAuditChainPayload(state, ""),
		Sealed:      sealed,
		Healthy:     healthy,
		Verified:    verified,
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
		FailedLogID: failedLogID,
		Failure:     failure,
	}, nil
}

func verifySaaSAuditChainStateTx(ctx context.Context, tx *sql.Tx, state saasAuditChainState) (bool, int64, int64, string, error) {
	legacyLogID, legacyHash, legacyCount, err := saasAuditLegacyAnchorTx(ctx, tx, state.TenantID)
	if err != nil {
		return false, 0, 0, "", err
	}
	if legacyLogID != state.AnchorLogID || legacyHash != state.AnchorHash || legacyCount != state.LegacyLogCount {
		failedID := legacyLogID
		if failedID == 0 {
			failedID = state.AnchorLogID
		}
		return false, 0, failedID, "legacy 审计锚点与当前结构日志不一致", nil
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, tenant_id, actor_user_id, actor_tenant_id, action, target_type, target_id,
			COALESCE(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), ''), integrity_prev_hash,
			integrity_hash, integrity_version
		FROM mochat_go_saas_admin_operation_logs
		WHERE tenant_id = ? AND deleted_at IS NULL AND integrity_version > 0
		ORDER BY id ASC
	`, state.TenantID)
	if err != nil {
		return false, 0, 0, "", err
	}
	defer rows.Close()
	expectedPrevious := state.AnchorHash
	expectedLastID := state.AnchorLogID
	expectedLastHash := state.AnchorHash
	var verified int64
	for rows.Next() {
		var item saasAuditStructuralLog
		var previousHash, currentHash string
		var version int
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.ActorUserID, &item.ActorTenantID,
			&item.Action, &item.TargetType, &item.TargetID, &item.CreatedAt,
			&previousHash, &currentHash, &version,
		); err != nil {
			return false, verified, 0, "", err
		}
		if version != saasAuditIntegrityVersion {
			return false, verified, item.ID, fmt.Sprintf("不支持的审计完整性版本 %d", version), nil
		}
		if item.ID <= state.AnchorLogID {
			return false, verified, item.ID, "结构摘要日志位于 legacy 锚点之前", nil
		}
		if previousHash != expectedPrevious {
			return false, verified, item.ID, "审计结构摘要链前序摘要不一致", nil
		}
		expectedHash, err := saasAuditVersionedHash(previousHash, item)
		if err != nil {
			return false, verified, 0, "", err
		}
		if currentHash != expectedHash {
			return false, verified, item.ID, "审计结构摘要与日志内容不一致", nil
		}
		expectedPrevious = currentHash
		expectedLastID = item.ID
		expectedLastHash = currentHash
		verified++
	}
	if err := rows.Err(); err != nil {
		return false, verified, 0, "", err
	}
	if verified != state.SignedLogCount || expectedLastID != state.LastLogID || expectedLastHash != state.LastHash {
		return false, verified, expectedLastID, "审计结构摘要链头或计数不一致", nil
	}
	return true, verified, 0, "", nil
}

func ensureSaaSAuditChainTx(ctx context.Context, tx *sql.Tx, tenantID int) (saasAuditChainState, bool, error) {
	if tenantID < 0 {
		tenantID = 0
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO mochat_go_saas_admin_audit_chains
			(tenant_id, anchor_hash, last_hash, status, version, created_at, updated_at)
		VALUES (?, ?, ?, 'unsealed', 1, NOW(), NOW())
	`, tenantID, saasAuditGenesisHash, saasAuditGenesisHash); err != nil {
		return saasAuditChainState{}, false, err
	}
	state, err := selectSaaSAuditChainStateTx(ctx, tx, tenantID)
	if err != nil {
		return saasAuditChainState{}, false, err
	}
	if state.Status != dashboard.SaaSAdminAuditIntegrityStatusUnsealed && state.SealedAt.Valid {
		return state, false, nil
	}
	anchorLogID, anchorHash, legacyCount, err := saasAuditLegacyAnchorTx(ctx, tx, tenantID)
	if err != nil {
		return saasAuditChainState{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_audit_chains
		SET anchor_log_id = ?, anchor_hash = ?, legacy_log_count = ?,
			last_log_id = ?, last_hash = ?, signed_log_count = 0,
			status = 'healthy', sealed_at = NOW(), last_failed_log_id = 0,
			last_verification_error = '', version = version + 1, updated_at = NOW()
		WHERE tenant_id = ?
	`, anchorLogID, anchorHash, legacyCount, anchorLogID, anchorHash, tenantID); err != nil {
		return saasAuditChainState{}, false, err
	}
	state, err = selectSaaSAuditChainStateTx(ctx, tx, tenantID)
	return state, err == nil, err
}

func selectSaaSAuditChainStateTx(ctx context.Context, tx *sql.Tx, tenantID int) (saasAuditChainState, error) {
	var state saasAuditChainState
	err := tx.QueryRowContext(ctx, `
		SELECT tenant_id, anchor_log_id, anchor_hash, legacy_log_count, last_log_id, last_hash,
			signed_log_count, status, sealed_at, last_verified_at, last_verification_status,
			last_failed_log_id, last_verification_error, version
		FROM mochat_go_saas_admin_audit_chains
		WHERE tenant_id = ?
		FOR UPDATE
	`, tenantID).Scan(
		&state.TenantID, &state.AnchorLogID, &state.AnchorHash, &state.LegacyLogCount,
		&state.LastLogID, &state.LastHash, &state.SignedLogCount, &state.Status,
		&state.SealedAt, &state.LastVerifiedAt, &state.LastVerificationStatus,
		&state.LastFailedLogID, &state.LastVerificationError, &state.Version,
	)
	return state, err
}

func insertSaaSAuditIntegrityLogTx(ctx context.Context, tx *sql.Tx, item dashboard.SaaSAdminOperationLog) (int64, error) {
	if item.TenantID < 0 {
		item.TenantID = 0
	}
	state, _, err := ensureSaaSAuditChainTx(ctx, tx, item.TenantID)
	if err != nil {
		return 0, err
	}
	createdAt := time.Now().Format(saasAuditTimeLayout)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_operation_logs
			(tenant_id, actor_user_id, actor_tenant_id, action, target_type, target_id, target_name,
			 before_json, after_json, remark, integrity_prev_hash, integrity_hash, integrity_version,
			 created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?, NULL)
	`, item.TenantID,
		item.ActorUserID,
		item.ActorTenantID,
		truncateRunes(strings.TrimSpace(item.Action), 64),
		truncateRunes(strings.TrimSpace(item.TargetType), 64),
		truncateRunes(strings.TrimSpace(item.TargetID), 128),
		truncateRunes(strings.TrimSpace(item.TargetName), 255),
		saasAdminJSONValue(item.BeforeJSON),
		saasAdminJSONValue(item.AfterJSON),
		truncateRunes(strings.TrimSpace(item.Remark), 255),
		state.LastHash,
		saasAuditIntegrityVersion,
		createdAt,
		createdAt,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	entry := saasAuditStructuralLog{
		ID:            id,
		TenantID:      item.TenantID,
		ActorUserID:   item.ActorUserID,
		ActorTenantID: item.ActorTenantID,
		Action:        truncateRunes(strings.TrimSpace(item.Action), 64),
		TargetType:    truncateRunes(strings.TrimSpace(item.TargetType), 64),
		TargetID:      truncateRunes(strings.TrimSpace(item.TargetID), 128),
		CreatedAt:     createdAt,
	}
	currentHash, err := saasAuditVersionedHash(state.LastHash, entry)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_operation_logs
		SET integrity_hash = ?
		WHERE id = ? AND tenant_id = ? AND integrity_version = ?
	`, currentHash, id, item.TenantID, saasAuditIntegrityVersion); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_admin_audit_chains
		SET last_log_id = ?, last_hash = ?, signed_log_count = signed_log_count + 1,
			version = version + 1, updated_at = NOW()
		WHERE tenant_id = ?
	`, id, currentHash, item.TenantID); err != nil {
		return 0, err
	}
	return id, nil
}

func insertLegacySaaSAdminOperationLogTx(ctx context.Context, tx *sql.Tx, item dashboard.SaaSAdminOperationLog) (int64, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_admin_operation_logs
			(tenant_id, actor_user_id, actor_tenant_id, action, target_type, target_id, target_name, before_json, after_json, remark, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL)
	`, item.TenantID,
		item.ActorUserID,
		item.ActorTenantID,
		truncateRunes(strings.TrimSpace(item.Action), 64),
		truncateRunes(strings.TrimSpace(item.TargetType), 64),
		truncateRunes(strings.TrimSpace(item.TargetID), 128),
		truncateRunes(strings.TrimSpace(item.TargetName), 255),
		saasAdminJSONValue(item.BeforeJSON),
		saasAdminJSONValue(item.AfterJSON),
		truncateRunes(strings.TrimSpace(item.Remark), 255),
	)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return id, nil
}

func saasAuditLegacyAnchorTx(ctx context.Context, tx *sql.Tx, tenantID int) (int64, string, int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, tenant_id, actor_user_id, actor_tenant_id, action, target_type, target_id,
			COALESCE(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM mochat_go_saas_admin_operation_logs
		WHERE tenant_id = ? AND deleted_at IS NULL AND integrity_version = 0
		ORDER BY id ASC
	`, tenantID)
	if err != nil {
		return 0, "", 0, err
	}
	defer rows.Close()
	hasher := sha256.New()
	_, _ = hasher.Write([]byte("mochat-go-saas-audit-legacy-v1\n"))
	var lastID, count int64
	for rows.Next() {
		var item saasAuditStructuralLog
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.ActorUserID, &item.ActorTenantID,
			&item.Action, &item.TargetType, &item.TargetID, &item.CreatedAt,
		); err != nil {
			return 0, "", 0, err
		}
		encoded, err := json.Marshal(item)
		if err != nil {
			return 0, "", 0, err
		}
		digest := sha256.Sum256(encoded)
		_, _ = hasher.Write([]byte(strconv.FormatInt(item.ID, 10)))
		_, _ = hasher.Write([]byte(":"))
		_, _ = hasher.Write([]byte(hex.EncodeToString(digest[:])))
		_, _ = hasher.Write([]byte("\n"))
		lastID = item.ID
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, "", 0, err
	}
	if count == 0 {
		return 0, saasAuditGenesisHash, 0, nil
	}
	return lastID, hex.EncodeToString(hasher.Sum(nil)), count, nil
}

func saasAuditVersionedHash(previousHash string, item saasAuditStructuralLog) (string, error) {
	payload := struct {
		Algorithm    int                    `json:"algorithm"`
		PreviousHash string                 `json:"previousHash"`
		Log          saasAuditStructuralLog `json:"log"`
	}{
		Algorithm:    saasAuditIntegrityVersion,
		PreviousHash: previousHash,
		Log:          item,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func scanSaaSAuditChain(scanner interface{ Scan(...any) error }) (dashboard.SaaSAdminAuditIntegrityChain, error) {
	var item dashboard.SaaSAdminAuditIntegrityChain
	var sealedAt, lastVerifiedAt sql.NullTime
	err := scanner.Scan(
		&item.TenantID, &item.TenantName, &item.AnchorLogID, &item.AnchorHash,
		&item.LegacyLogCount, &item.LastLogID, &item.LastHash, &item.SignedLogCount,
		&item.Status, &sealedAt, &lastVerifiedAt, &item.LastVerificationStatus,
		&item.LastFailedLogID, &item.LastVerificationError, &item.Version,
	)
	if err != nil {
		return dashboard.SaaSAdminAuditIntegrityChain{}, err
	}
	item.TenantName = saasAuditTenantName(item.TenantID, item.TenantName)
	item.SealedAt, item.LastVerifiedAt = formatTime(sealedAt), formatTime(lastVerifiedAt)
	return item, nil
}

func saasAuditChainPayload(state saasAuditChainState, tenantName string) dashboard.SaaSAdminAuditIntegrityChain {
	return dashboard.SaaSAdminAuditIntegrityChain{
		TenantID:               state.TenantID,
		TenantName:             saasAuditTenantName(state.TenantID, tenantName),
		AnchorLogID:            state.AnchorLogID,
		AnchorHash:             state.AnchorHash,
		LegacyLogCount:         state.LegacyLogCount,
		LastLogID:              state.LastLogID,
		LastHash:               state.LastHash,
		SignedLogCount:         state.SignedLogCount,
		Status:                 state.Status,
		SealedAt:               formatTime(state.SealedAt),
		LastVerifiedAt:         formatTime(state.LastVerifiedAt),
		LastVerificationStatus: state.LastVerificationStatus,
		LastFailedLogID:        state.LastFailedLogID,
		LastVerificationError:  state.LastVerificationError,
		Version:                state.Version,
	}
}

func saasAuditTenantName(tenantID int, tenantName string) string {
	if tenantID == 0 {
		return "平台级"
	}
	if strings.TrimSpace(tenantName) == "" {
		return fmt.Sprintf("租户 #%d", tenantID)
	}
	return tenantName
}

func isMissingSaaSAuditIntegritySchemaError(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlErr) || (mysqlErr.Number != 1054 && mysqlErr.Number != 1146) {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "mochat_go_saas_admin_audit_chains") ||
		strings.Contains(message, "integrity_prev_hash") ||
		strings.Contains(message, "integrity_hash") ||
		strings.Contains(message, "integrity_version")
}
