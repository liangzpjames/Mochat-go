package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) SaaSReleaseEvidence(ctx context.Context) ([]dashboard.SaaSReleaseEvidence, error) {
	rows, err := s.db.QueryContext(ctx, saasReleaseEvidenceSelect+` ORDER BY required DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSReleaseEvidence, 0, dashboard.SaaSReleaseEvidenceRequiredCount)
	for rows.Next() {
		item, err := scanSaaSReleaseEvidence(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) UpdateSaaSReleaseEvidence(ctx context.Context, input dashboard.SaaSReleaseEvidenceUpdate) (dashboard.SaaSReleaseEvidenceUpdateResult, error) {
	if input.Status == dashboard.SaaSReleaseEvidenceStatusPassed && !saasReleaseVerificationMatchesUpdate(input) {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusUnprocessableEntity, Message: "发布证据缺少有效的远端工件校验结果"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, err
	}
	defer tx.Rollback()
	before, found, err := saasReleaseEvidenceByKey(ctx, tx, input.Key, true)
	if err != nil {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, err
	}
	if !found {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, dashboard.NewSaaSAdminNotFound("发布证据项不存在")
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "发布证据版本已变化，请刷新后重试"}
	}
	checkedAt := "NOW()"
	checkedBy := input.ActorUserID
	if input.Status == dashboard.SaaSReleaseEvidenceStatusMissing {
		checkedAt = "NULL"
		checkedBy = 0
	}
	query := `
		UPDATE mochat_go_saas_release_evidence
		SET status = ?, evidence_url = ?, environment = ?, source_fingerprint = ?, artifact_sha256 = ?, artifact_size_bytes = ?, note = ?,
			checked_at = ` + checkedAt + `, checked_by = ?, version = version + 1, updated_at = NOW()
		WHERE id = ? AND version = ?`
	result, err := tx.ExecContext(ctx, query, input.Status, input.EvidenceURL, input.Environment, input.SourceFingerprint, input.ArtifactSHA256, input.ArtifactSizeBytes, input.Note, checkedBy, before.ID, before.Version)
	if err != nil {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, err
	}
	if affected != 1 {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "发布证据版本已变化，请刷新后重试"}
	}
	after, found, err := saasReleaseEvidenceByKey(ctx, tx, input.Key, false)
	if err != nil {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, err
	}
	if !found {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, fmt.Errorf("updated release evidence %s was not persisted", input.Key)
	}
	beforeJSON, _ := json.Marshal(saasReleaseEvidenceAuditPayload(before))
	afterAudit := saasReleaseEvidenceAuditPayload(after)
	if input.ArtifactVerification != nil {
		afterAudit["artifactVerification"] = input.ArtifactVerification
	}
	afterJSON, _ := json.Marshal(afterAudit)
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionEvidenceSave, TargetType: dashboard.SaaSAdminOperationTargetEvidence,
		TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.Title,
		BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), Remark: "update production release evidence",
	})
	if err != nil {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSReleaseEvidenceUpdateResult{}, err
	}
	return dashboard.SaaSReleaseEvidenceUpdateResult{Evidence: after, ArtifactVerification: input.ArtifactVerification, OperationID: operationID}, nil
}

func (s *MySQLStore) SaaSReleaseEvidenceActions(ctx context.Context, platformTenantID int) ([]dashboard.SaaSReleaseEvidenceAction, error) {
	if platformTenantID <= 0 {
		return nil, dashboard.NewSaaSAdminBadRequest("platformTenantId 无效")
	}
	rows, err := s.db.QueryContext(ctx, saasReleaseEvidenceActionSelect+` ORDER BY e.required DESC, e.id ASC`, platformTenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSReleaseEvidenceAction, 0, dashboard.SaaSReleaseEvidenceRequiredCount)
	for rows.Next() {
		item, err := scanSaaSReleaseEvidenceAction(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) UpdateSaaSReleaseEvidenceAction(ctx context.Context, input dashboard.SaaSReleaseEvidenceActionUpdate) (dashboard.SaaSReleaseEvidenceActionUpdateResult, error) {
	if input.ActorUserID <= 0 || input.ActorTenantID <= 0 {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, dashboard.NewSaaSAdminBadRequest("操作人或平台租户无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, err
	}
	defer tx.Rollback()
	before, found, err := saasReleaseEvidenceActionByKey(ctx, tx, input.Key, input.ActorTenantID, true)
	if err != nil {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, err
	}
	if !found {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, dashboard.NewSaaSAdminNotFound("发布补证行动不存在")
	}
	if before.Version != input.ExpectedVersion {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "发布补证行动版本已变化，请刷新后重试"}
	}
	if input.OwnerUserID > 0 {
		var ownerID int
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM mc_user
			WHERE id = ? AND tenant_id = ? AND status = 1 AND deleted_at IS NULL
			LIMIT 1 FOR UPDATE
		`, input.OwnerUserID, input.ActorTenantID).Scan(&ownerID)
		if errors.Is(err, sql.ErrNoRows) {
			return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, dashboard.NewSaaSAdminBadRequest("负责人必须是当前平台管理租户的有效用户")
		}
		if err != nil {
			return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, err
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_release_evidence_actions
		SET owner_user_id = ?, due_at = NULLIF(?, ''), next_action = ?, note = ?, version = version + 1,
			updated_by = ?, updated_at = NOW()
		WHERE id = ? AND version = ?
	`, input.OwnerUserID, input.DueAt, input.NextAction, input.Note, input.ActorUserID, before.ID, before.Version)
	if err != nil {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, err
	}
	if affected != 1 {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "发布补证行动版本已变化，请刷新后重试"}
	}
	after, found, err := saasReleaseEvidenceActionByKey(ctx, tx, input.Key, input.ActorTenantID, false)
	if err != nil {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, err
	}
	if !found {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, fmt.Errorf("updated release evidence action %s was not persisted", input.Key)
	}
	beforeJSON, _ := json.Marshal(saasReleaseEvidenceActionAuditPayload(before))
	afterJSON, _ := json.Marshal(saasReleaseEvidenceActionAuditPayload(after))
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionEvidenceActionSave, TargetType: dashboard.SaaSAdminOperationTargetEvidenceAction,
		TargetID: strconv.FormatInt(after.ID, 10), TargetName: after.Title,
		BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), Remark: "update production release evidence action",
	})
	if err != nil {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSReleaseEvidenceActionUpdateResult{}, err
	}
	return dashboard.SaaSReleaseEvidenceActionUpdateResult{Action: after, OperationID: operationID}, nil
}

func (s *MySQLStore) SaaSReleaseCandidates(ctx context.Context, limit int) ([]dashboard.SaaSReleaseCandidate, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, saasReleaseCandidateSelect+` ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dashboard.SaaSReleaseCandidate, 0, limit)
	for rows.Next() {
		item, err := scanSaaSReleaseCandidate(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) CreateSaaSReleaseCandidate(ctx context.Context, input dashboard.SaaSReleaseCandidateCreate) (dashboard.SaaSReleaseCandidateCreateResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, saasReleaseEvidenceSelect+` ORDER BY required DESC, id ASC FOR UPDATE`)
	if err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	items := make([]dashboard.SaaSReleaseEvidence, 0, dashboard.SaaSReleaseEvidenceRequiredCount)
	for rows.Next() {
		item, scanErr := scanSaaSReleaseEvidence(rows)
		if scanErr != nil {
			rows.Close()
			return dashboard.SaaSReleaseCandidateCreateResult{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	verifications := make(map[string]dashboard.SaaSReleaseArtifactVerification, len(input.ArtifactVerifications))
	for key, verification := range input.ArtifactVerifications {
		verifications[key] = verification
	}
	required, passed, matched := 0, 0, 0
	failedKeys := make([]string, 0, dashboard.SaaSReleaseEvidenceRequiredCount)
	for _, item := range items {
		if !item.Required {
			continue
		}
		required++
		verification, found := verifications[item.Key]
		if !found {
			verification = dashboard.SaaSReleaseArtifactVerification{
				EvidenceKey: item.Key, EvidenceVersion: item.Version, EvidenceURL: item.EvidenceURL,
				ExpectedSHA256: item.ArtifactSHA256, ExpectedSizeBytes: item.ArtifactSizeBytes,
				Error: "未收到远端工件校验结果",
			}
			verifications[item.Key] = verification
		} else if !dashboard.SaaSReleaseArtifactVerificationMatchesEvidence(item, verification) {
			return dashboard.SaaSReleaseCandidateCreateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "发布证据在远端校验期间已变化，请刷新后重试"}
		}
		if dashboard.SaaSReleaseEvidenceComplete(item) && dashboard.SaaSReleaseArtifactVerificationPassed(item, verification) {
			passed++
			if item.SourceFingerprint == input.SourceFingerprint {
				matched++
			}
		} else {
			failedKeys = append(failedKeys, item.Key)
		}
	}
	status := dashboard.SaaSReleaseCandidateStatusBlocked
	message := fmt.Sprintf("发布门禁未通过：必需 %d，远端校验通过 %d，指纹匹配 %d", required, passed, matched)
	if len(failedKeys) > 0 {
		message += "；失败项：" + strings.Join(failedKeys, ", ")
	}
	if required == dashboard.SaaSReleaseEvidenceRequiredCount && passed == required && matched == required {
		status = dashboard.SaaSReleaseCandidateStatusReady
		message = "发布门禁通过：六类生产证据已远端校验且源码指纹一致"
	}
	snapshotJSON, err := json.Marshal(saasReleaseEvidenceSnapshot(items, verifications))
	if err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_release_candidates
			(candidate_no, release_version, source_fingerprint, status, required_count, passed_count, matched_count,
			 snapshot_json, gate_message, created_by, operation_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, NOW())
	`, input.CandidateNo, input.ReleaseVersion, input.SourceFingerprint, status, required, passed, matched, string(snapshotJSON), message, input.ActorUserID)
	if isMySQLDuplicateKeyError(err) {
		return dashboard.SaaSReleaseCandidateCreateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "发布候选编号冲突，请重试"}
	}
	if err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	candidate, found, err := saasReleaseCandidateByID(ctx, tx, id)
	if err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	if !found {
		return dashboard.SaaSReleaseCandidateCreateResult{}, fmt.Errorf("release candidate %d was not persisted", id)
	}
	afterJSON, _ := json.Marshal(saasReleaseCandidateAuditPayload(candidate))
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: 0, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionCandidateGate, TargetType: dashboard.SaaSAdminOperationTargetCandidate,
		TargetID: candidate.CandidateNo, TargetName: candidate.ReleaseVersion,
		BeforeJSON: `{}`, AfterJSON: string(afterJSON), Remark: message,
	})
	if err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_release_candidates SET operation_id = ? WHERE id = ?`, operationID, candidate.ID); err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.ActorUserID, operationID); err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	candidate.OperationID = operationID
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSReleaseCandidateCreateResult{}, err
	}
	return dashboard.SaaSReleaseCandidateCreateResult{Candidate: candidate, OperationID: operationID}, nil
}

const saasReleaseEvidenceSelect = `
	SELECT id, evidence_key, title, category, required, status, evidence_url, environment,
		source_fingerprint, artifact_sha256, artifact_size_bytes, checked_at, checked_by, note, version, created_at, updated_at
	FROM mochat_go_saas_release_evidence`

const saasReleaseEvidenceActionSelect = `
	SELECT a.id, a.evidence_key, e.title, a.owner_user_id, COALESCE(u.name, ''), COALESCE(u.phone, ''),
		CASE WHEN u.id IS NOT NULL AND u.tenant_id = ? AND u.status = 1 AND u.deleted_at IS NULL THEN 1 ELSE 0 END,
		a.due_at, a.next_action, a.note, a.version, a.created_by, a.updated_by, a.created_at, a.updated_at
	FROM mochat_go_saas_release_evidence_actions a
	INNER JOIN mochat_go_saas_release_evidence e ON e.evidence_key = a.evidence_key
	LEFT JOIN mc_user u ON u.id = a.owner_user_id`

const saasReleaseCandidateSelect = `
	SELECT id, candidate_no, release_version, source_fingerprint, status, required_count, passed_count,
		matched_count, snapshot_json, gate_message, created_by, operation_id, created_at
	FROM mochat_go_saas_release_candidates`

type saasReleaseScanner interface {
	Scan(...any) error
}

func scanSaaSReleaseEvidence(scanner saasReleaseScanner) (dashboard.SaaSReleaseEvidence, error) {
	var item dashboard.SaaSReleaseEvidence
	var required int
	var checkedAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Key, &item.Title, &item.Category, &required, &item.Status,
		&item.EvidenceURL, &item.Environment, &item.SourceFingerprint, &item.ArtifactSHA256, &item.ArtifactSizeBytes, &checkedAt, &item.CheckedBy,
		&item.Note, &item.Version, &createdAt, &updatedAt)
	item.Required = required == 1
	item.CheckedAt, item.CreatedAt, item.UpdatedAt = formatTime(checkedAt), formatTime(createdAt), formatTime(updatedAt)
	return item, err
}

func scanSaaSReleaseEvidenceAction(scanner saasReleaseScanner) (dashboard.SaaSReleaseEvidenceAction, error) {
	var item dashboard.SaaSReleaseEvidenceAction
	var ownerActive int
	var dueAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Key, &item.Title, &item.OwnerUserID, &item.OwnerName, &item.OwnerPhone, &ownerActive,
		&dueAt, &item.NextAction, &item.Note, &item.Version, &item.CreatedBy, &item.UpdatedBy, &createdAt, &updatedAt)
	item.OwnerActive = ownerActive == 1
	item.DueAt, item.CreatedAt, item.UpdatedAt = formatTime(dueAt), formatTime(createdAt), formatTime(updatedAt)
	return item, err
}

func scanSaaSReleaseCandidate(scanner saasReleaseScanner) (dashboard.SaaSReleaseCandidate, error) {
	var item dashboard.SaaSReleaseCandidate
	var createdAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.CandidateNo, &item.ReleaseVersion, &item.SourceFingerprint, &item.Status,
		&item.RequiredCount, &item.PassedCount, &item.MatchedCount, &item.SnapshotJSON, &item.GateMessage,
		&item.CreatedBy, &item.OperationID, &createdAt)
	item.CreatedAt = formatTime(createdAt)
	return item, err
}

func saasReleaseEvidenceByKey(ctx context.Context, queryer saasAdminAccessQueryer, key string, forUpdate bool) (dashboard.SaaSReleaseEvidence, bool, error) {
	query := saasReleaseEvidenceSelect + ` WHERE evidence_key = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	item, err := scanSaaSReleaseEvidence(queryer.QueryRowContext(ctx, query, key))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSReleaseEvidence{}, false, nil
	}
	return item, err == nil, err
}

func saasReleaseEvidenceActionByKey(ctx context.Context, queryer saasAdminAccessQueryer, key string, platformTenantID int, forUpdate bool) (dashboard.SaaSReleaseEvidenceAction, bool, error) {
	query := saasReleaseEvidenceActionSelect + ` WHERE a.evidence_key = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	item, err := scanSaaSReleaseEvidenceAction(queryer.QueryRowContext(ctx, query, platformTenantID, key))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSReleaseEvidenceAction{}, false, nil
	}
	return item, err == nil, err
}

func saasReleaseCandidateByID(ctx context.Context, queryer saasAdminAccessQueryer, id int64) (dashboard.SaaSReleaseCandidate, bool, error) {
	item, err := scanSaaSReleaseCandidate(queryer.QueryRowContext(ctx, saasReleaseCandidateSelect+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSReleaseCandidate{}, false, nil
	}
	return item, err == nil, err
}

func saasReleaseEvidenceAuditPayload(item dashboard.SaaSReleaseEvidence) map[string]any {
	if item.ID == 0 {
		return map[string]any{}
	}
	return map[string]any{
		"id": item.ID, "key": item.Key, "status": item.Status, "environment": item.Environment,
		"evidenceUrl": item.EvidenceURL, "sourceFingerprint": item.SourceFingerprint,
		"artifactSha256": item.ArtifactSHA256, "artifactSizeBytes": item.ArtifactSizeBytes,
		"checkedAt": item.CheckedAt, "checkedBy": item.CheckedBy, "note": item.Note, "version": item.Version,
	}
}

func saasReleaseEvidenceActionAuditPayload(item dashboard.SaaSReleaseEvidenceAction) map[string]any {
	if item.ID == 0 {
		return map[string]any{}
	}
	return map[string]any{
		"id": item.ID, "key": item.Key, "ownerUserId": item.OwnerUserID, "ownerName": item.OwnerName,
		"ownerActive": item.OwnerActive, "dueAt": item.DueAt, "nextAction": item.NextAction,
		"note": item.Note, "version": item.Version, "updatedBy": item.UpdatedBy, "updatedAt": item.UpdatedAt,
	}
}

func saasReleaseEvidenceSnapshot(items []dashboard.SaaSReleaseEvidence, verifications map[string]dashboard.SaaSReleaseArtifactVerification) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := saasReleaseEvidenceAuditPayload(item)
		if verification, found := verifications[item.Key]; found {
			payload["artifactVerification"] = verification
		}
		result = append(result, payload)
	}
	return result
}

func saasReleaseVerificationMatchesUpdate(input dashboard.SaaSReleaseEvidenceUpdate) bool {
	verification := input.ArtifactVerification
	return verification != nil && verification.Verified && verification.Error == "" &&
		verification.EvidenceKey == input.Key && verification.EvidenceVersion == input.ExpectedVersion &&
		verification.EvidenceURL == input.EvidenceURL && verification.ExpectedSHA256 == input.ArtifactSHA256 &&
		verification.ExpectedSizeBytes == input.ArtifactSizeBytes && verification.ActualSHA256 == input.ArtifactSHA256 &&
		verification.ActualSizeBytes == input.ArtifactSizeBytes && verification.HTTPStatus == http.StatusOK &&
		strings.TrimSpace(verification.AttemptedAt) != ""
}

func saasReleaseCandidateAuditPayload(item dashboard.SaaSReleaseCandidate) map[string]any {
	return map[string]any{
		"id": item.ID, "candidateNo": item.CandidateNo, "releaseVersion": item.ReleaseVersion,
		"sourceFingerprint": item.SourceFingerprint, "status": item.Status, "requiredCount": item.RequiredCount,
		"passedCount": item.PassedCount, "matchedCount": item.MatchedCount, "gateMessage": item.GateMessage,
		"createdBy": item.CreatedBy, "createdAt": item.CreatedAt,
	}
}
