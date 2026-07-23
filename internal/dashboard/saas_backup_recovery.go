package dashboard

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"jiyi/mochat-go/internal/saasbackup"
)

func (h *SaaSAdminHandler) WithBackupManager(manager *saasbackup.Manager) *SaaSAdminHandler {
	h.backupManager = manager
	return h
}

func (h *SaaSAdminHandler) BackupOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	manager, ok := h.saasBackupManager(w)
	if !ok {
		return
	}
	limit := saasAdminQueryInt(r, "limit", 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	overview, err := manager.Overview(r.Context(), limit)
	if err != nil {
		writeSaaSBackupError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasBackupOverviewPayload(overview))
}

func (h *SaaSAdminHandler) BackupPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasBackupManager(w)
	if !ok {
		return
	}
	var input saasbackup.PolicyUpdate
	if err := decodeSaaSBackupJSON(r, &input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input, err := manager.ValidatePolicyUpdate(input)
	if err != nil {
		writeSaaSBackupError(w, err)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionBackupPolicyUpdate, 0) {
		return
	}
	input.Actor = saasbackup.Actor{UserID: user.ID, TenantID: user.TenantID}
	policy, err := manager.UpdatePolicy(r.Context(), input)
	if err != nil {
		writeSaaSBackupError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"policy": saasBackupPolicyPayload(policy)})
}

func (h *SaaSAdminHandler) BackupRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasBackupManager(w)
	if !ok {
		return
	}
	var request struct {
		Action       string `json:"action"`
		BackupRunID  int64  `json:"backupRunId"`
		CleanupRunID int64  `json:"cleanupRunId"`
	}
	if err := decodeSaaSBackupJSON(r, &request); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	action := strings.ToLower(strings.TrimSpace(request.Action))
	actor := saasbackup.Actor{UserID: user.ID, TenantID: user.TenantID}
	switch action {
	case "create":
		run, err := manager.Create(r.Context(), saasbackup.TriggerManual, actor)
		if err != nil {
			writeSaaSBackupError(w, err)
			return
		}
		writeEnvelope(w, http.StatusCreated, http.StatusCreated, "success", map[string]any{"action": action, "run": saasBackupRunPayload(run)})
	case "verify":
		if request.BackupRunID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "backupRunId 必须是正整数", nil)
			return
		}
		run, err := manager.Verify(r.Context(), request.BackupRunID, actor)
		if err != nil {
			writeSaaSBackupError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"action": action, "run": saasBackupRunPayload(run)})
	case "replicate":
		if request.BackupRunID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "backupRunId 必须是正整数", nil)
			return
		}
		run, err := manager.Replicate(r.Context(), request.BackupRunID, actor)
		if err != nil {
			writeSaaSBackupError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"action": action, "run": saasBackupRunPayload(run)})
	case "cleanup":
		if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionBackupRetentionCleanup, 0) {
			return
		}
		plan, err := manager.PlanCleanup(r.Context())
		if err != nil {
			writeSaaSBackupError(w, err)
			return
		}
		cleanupRun, err := manager.ScheduleCleanup(r.Context(), plan, actor, 0, 0)
		if err != nil {
			writeSaaSBackupError(w, err)
			return
		}
		writeEnvelope(w, http.StatusCreated, http.StatusCreated, "success", map[string]any{"action": action, "cleanupRun": saasBackupCleanupRunPayload(cleanupRun)})
	case "cleanup-retry":
		if request.CleanupRunID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "cleanupRunId 必须是正整数", nil)
			return
		}
		cleanupRun, err := manager.RetryCleanup(r.Context(), request.CleanupRunID, actor)
		if err != nil {
			writeSaaSBackupError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"action": action, "cleanupRun": saasBackupCleanupRunPayload(cleanupRun)})
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "action 必须是 create、verify、replicate、cleanup 或 cleanup-retry", nil)
	}
}

func (h *SaaSAdminHandler) RestoreDrill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasBackupManager(w)
	if !ok {
		return
	}
	var request struct {
		BackupRunID int64 `json:"backupRunId"`
	}
	if err := decodeSaaSBackupJSON(r, &request); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if request.BackupRunID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "backupRunId 必须是正整数", nil)
		return
	}
	drill, err := manager.RestoreDrill(r.Context(), request.BackupRunID, saasbackup.TriggerManual, saasbackup.Actor{UserID: user.ID, TenantID: user.TenantID})
	if err != nil {
		writeSaaSBackupError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"drill": saasRestoreDrillPayload(drill)})
}

func (h *SaaSAdminHandler) saasBackupManager(w http.ResponseWriter) (*saasbackup.Manager, bool) {
	if h.backupManager == nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "SaaS 备份管理器未配置", nil)
		return nil, false
	}
	return h.backupManager, true
}

func decodeSaaSBackupJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("请求体不能为空")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("请求 JSON 无效")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("请求 JSON 只能包含一个对象")
	}
	return nil
}

func writeSaaSBackupError(w http.ResponseWriter, err error) {
	writeSaaSAdminError(w, saasBackupAdminError(err))
}

func saasBackupAdminError(err error) error {
	var opErr *saasbackup.OperationError
	if errors.As(err, &opErr) {
		status := http.StatusInternalServerError
		switch opErr.Code {
		case "invalid":
			status = http.StatusBadRequest
		case "not_found":
			status = http.StatusNotFound
		case "conflict":
			status = http.StatusConflict
		case "unavailable":
			status = http.StatusServiceUnavailable
		}
		return &SaaSAdminOperationError{Status: status, Message: opErr.Message}
	}
	return err
}

func saasBackupOverviewPayload(overview saasbackup.Overview) map[string]any {
	summary := map[string]any{
		"runCount": len(overview.Runs), "successfulCount": 0, "failedCount": 0, "runningCount": 0,
		"verifiedCount": 0, "replicaSucceededCount": 0, "replicaFailedCount": 0,
		"drillCount": len(overview.Drills), "successfulDrillCount": 0,
		"cleanupCount": len(overview.CleanupRuns), "cleanupFailedCount": 0, "cleanupRunningCount": 0,
	}
	for _, run := range overview.Runs {
		switch run.Status {
		case saasbackup.RunStatusSucceeded:
			summary["successfulCount"] = summary["successfulCount"].(int) + 1
		case saasbackup.RunStatusFailed:
			summary["failedCount"] = summary["failedCount"].(int) + 1
		case saasbackup.RunStatusRunning:
			summary["runningCount"] = summary["runningCount"].(int) + 1
		}
		if run.Status != saasbackup.RunStatusDeleted && run.VerificationStatus == saasbackup.VerificationPassed {
			summary["verifiedCount"] = summary["verifiedCount"].(int) + 1
		}
		if run.Status != saasbackup.RunStatusDeleted && run.ReplicaStatus == saasbackup.ReplicaStatusSucceeded {
			summary["replicaSucceededCount"] = summary["replicaSucceededCount"].(int) + 1
		}
		if run.Status != saasbackup.RunStatusDeleted && run.ReplicaStatus == saasbackup.ReplicaStatusFailed {
			summary["replicaFailedCount"] = summary["replicaFailedCount"].(int) + 1
		}
	}
	for _, drill := range overview.Drills {
		if drill.Status == saasbackup.DrillStatusSucceeded {
			summary["successfulDrillCount"] = summary["successfulDrillCount"].(int) + 1
		}
	}
	for _, cleanupRun := range overview.CleanupRuns {
		if cleanupRun.Status == saasbackup.CleanupStatusFailed || cleanupRun.Status == saasbackup.CleanupStatusPartial {
			summary["cleanupFailedCount"] = summary["cleanupFailedCount"].(int) + 1
		}
		if cleanupRun.Status == saasbackup.CleanupStatusRunning || cleanupRun.Status == saasbackup.CleanupStatusPending {
			summary["cleanupRunningCount"] = summary["cleanupRunningCount"].(int) + 1
		}
	}
	runs := make([]map[string]any, 0, len(overview.Runs))
	for _, run := range overview.Runs {
		runs = append(runs, saasBackupRunPayload(run))
	}
	drills := make([]map[string]any, 0, len(overview.Drills))
	for _, drill := range overview.Drills {
		drills = append(drills, saasRestoreDrillPayload(drill))
	}
	cleanupRuns := make([]map[string]any, 0, len(overview.CleanupRuns))
	for _, cleanupRun := range overview.CleanupRuns {
		cleanupRuns = append(cleanupRuns, saasBackupCleanupRunPayload(cleanupRun))
	}
	return map[string]any{
		"policy": saasBackupPolicyPayload(overview.Policy), "summary": summary, "runs": runs, "drills": drills, "cleanupRuns": cleanupRuns,
		"config": map[string]any{
			"sourceConfigured": overview.Config.SourceConfigured, "backupRootConfigured": overview.Config.BackupRootConfigured,
			"encryptionConfigured": overview.Config.EncryptionConfigured, "restoreConfigured": overview.Config.RestoreConfigured,
			"cronEnabled": overview.Config.CronEnabled, "cronIntervalSeconds": overview.Config.CronIntervalSeconds,
			"cronRunOnStart": overview.Config.CronRunOnStart,
			"dumpToolReady":  overview.Config.DumpToolReady, "restoreToolReady": overview.Config.RestoreToolReady,
			"encryptionKeyId": overview.Config.EncryptionKeyID, "encryptionKeyCount": overview.Config.EncryptionKeyCount,
			"restoreAutoProvision": overview.Config.RestoreAutoProvision, "restoreAdminConfigured": overview.Config.RestoreAdminConfigured,
			"replicaConfigured": overview.Config.ReplicaConfigured, "replicaProvider": overview.Config.ReplicaProvider,
			"replicaBucket": overview.Config.ReplicaBucket, "restoreDatabasePrefix": overview.Config.RestoreDatabasePrefix,
		},
	}
}

func saasBackupPolicyPayload(item saasbackup.Policy) map[string]any {
	return map[string]any{
		"id": item.ID, "status": item.Status, "intervalMinutes": item.IntervalMinutes,
		"retentionDays": item.RetentionDays, "minSuccessfulBackups": item.MinSuccessfulBackups,
		"maxBackupAgeMinutes": item.MaxBackupAgeMinutes, "restoreDrillIntervalDays": item.RestoreDrillIntervalDays,
		"requireEncryption": item.RequireEncryption, "requireOffsiteReplica": item.RequireOffsiteReplica,
		"version": item.Version, "updatedBy": item.UpdatedBy,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasBackupRunPayload(item saasbackup.BackupRun) map[string]any {
	return map[string]any{
		"id": item.ID, "backupNo": item.BackupNo, "triggerType": item.TriggerType, "status": item.Status,
		"artifactName": item.ArtifactName, "artifactFormat": item.ArtifactFormat, "encrypted": item.Encrypted,
		"encryptionKeyId": item.EncryptionKeyID, "sha256": item.SHA256, "sizeBytes": item.SizeBytes,
		"encryptionKeyReady": item.EncryptionKeyReady, "replicaStatus": item.ReplicaStatus,
		"replicaProvider": item.ReplicaProvider, "replicaBucket": item.ReplicaBucket,
		"replicaObjectKey": item.ReplicaObjectKey, "replicaETag": item.ReplicaETag,
		"replicaVersionId": item.ReplicaVersionID, "replicaSha256": item.ReplicaSHA256,
		"replicaSizeBytes": item.ReplicaSizeBytes, "replicatedAt": item.ReplicatedAt,
		"replicaVerifiedAt": item.ReplicaVerifiedAt, "replicaError": item.ReplicaError,
		"migrationVersion": item.MigrationVersion, "migrationCount": item.MigrationCount, "tableCount": item.TableCount,
		"verificationStatus": item.VerificationStatus, "verifiedAt": item.VerifiedAt,
		"verificationError": item.VerificationError, "errorMessage": item.ErrorMessage,
		"startedAt": item.StartedAt, "finishedAt": item.FinishedAt, "createdBy": item.CreatedBy,
		"operationId": item.OperationID, "cleanupRunId": item.CleanupRunID, "version": item.Version, "deletedAt": item.DeletedAt,
	}
}

func saasBackupCleanupRunPayload(item saasbackup.CleanupRun) map[string]any {
	return map[string]any{
		"id": item.ID, "cleanupNo": item.CleanupNo, "status": item.Status, "policyVersion": item.PolicyVersion,
		"cutoffAt": item.CutoffAt, "scannedCount": item.ScannedCount, "candidateCount": item.CandidateCount,
		"preservedCount": item.PreservedCount, "deletedCount": item.DeletedCount, "failedCount": item.FailedCount,
		"replicasDeletedCount": item.ReplicasDeletedCount, "missingFilesCount": item.MissingFilesCount,
		"attempts": item.Attempts, "approvalId": item.ApprovalID, "leaseExpiresAt": item.LeaseExpiresAt,
		"startedAt": item.StartedAt, "finishedAt": item.FinishedAt, "lastError": item.LastError,
		"createdBy": item.CreatedBy, "operationId": item.OperationID, "version": item.Version,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasRestoreDrillPayload(item saasbackup.RestoreDrill) map[string]any {
	checks := any(map[string]any{})
	if strings.TrimSpace(item.ChecksJSON) != "" {
		var decoded any
		if json.Unmarshal([]byte(item.ChecksJSON), &decoded) == nil {
			checks = decoded
		}
	}
	return map[string]any{
		"id": item.ID, "drillNo": item.DrillNo, "backupRunId": item.BackupRunID,
		"triggerType": item.TriggerType, "status": item.Status, "targetFingerprint": item.TargetFingerprint,
		"targetDatabase": item.TargetDatabase, "targetLifecycle": item.TargetLifecycle,
		"targetCleanupStatus": item.TargetCleanupStatus, "targetCleanedAt": item.TargetCleanedAt,
		"targetCleanupError": item.TargetCleanupError, "expectedMigrationVersion": item.ExpectedMigrationVersion,
		"actualMigrationVersion": item.ActualMigrationVersion, "expectedMigrationCount": item.ExpectedMigrationCount,
		"actualMigrationCount": item.ActualMigrationCount, "expectedTableCount": item.ExpectedTableCount,
		"actualTableCount": item.ActualTableCount, "checks": checks, "errorMessage": item.ErrorMessage,
		"startedAt": item.StartedAt, "finishedAt": item.FinishedAt, "durationMs": item.DurationMS,
		"createdBy": item.CreatedBy, "operationId": item.OperationID,
	}
}
