package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/httpresponse"
	"jiyi/mochat-go/internal/saascompliance"
)

type complianceExportOpener interface {
	OpenExport(context.Context, int64, saascompliance.Actor) (saascompliance.ExportDownload, error)
}

func (h *SaaSAdminHandler) WithComplianceManager(manager *saascompliance.Manager) *SaaSAdminHandler {
	h.complianceManager = manager
	h.complianceExportOpener = manager
	return h
}

func (h *SaaSAdminHandler) withComplianceExportOpener(opener complianceExportOpener) *SaaSAdminHandler {
	h.complianceExportOpener = opener
	return h
}

func (h *SaaSAdminHandler) ComplianceOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	manager, ok := h.saasComplianceManager(w)
	if !ok {
		return
	}
	tenantID := saasAdminQueryInt(r, "tenantId", 0)
	limit := saasAdminQueryInt(r, "limit", 50)
	if tenantID < 0 || limit <= 0 || limit > 200 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId 或 limit 无效", nil)
		return
	}
	overview, err := manager.Overview(r.Context(), tenantID, limit)
	if err != nil {
		writeSaaSComplianceError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasComplianceOverviewPayload(overview))
}

func (h *SaaSAdminHandler) CompliancePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasComplianceManager(w)
	if !ok {
		return
	}
	var input saascompliance.PolicyUpdate
	if err := decodeSaaSComplianceJSON(r, &input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input, err := manager.ValidatePolicyUpdate(input)
	if err != nil {
		writeSaaSComplianceError(w, err)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionCompliancePolicyUpdate, 0) {
		return
	}
	input.Actor = saascompliance.Actor{UserID: user.ID, TenantID: user.TenantID}
	policy, err := manager.UpdatePolicy(r.Context(), input)
	if err != nil {
		writeSaaSComplianceError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"policy": saasCompliancePolicyPayload(policy)})
}

func (h *SaaSAdminHandler) ComplianceLegalHold(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasComplianceManager(w)
	if !ok {
		return
	}
	var body struct {
		Action          string `json:"action"`
		HoldID          int64  `json:"holdId"`
		TenantID        int    `json:"tenantId"`
		Reason          string `json:"reason"`
		StartsAt        string `json:"startsAt"`
		ExpiresAt       string `json:"expiresAt"`
		ExpectedVersion int    `json:"expectedVersion"`
	}
	if err := decodeSaaSComplianceJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	actor := saascompliance.Actor{UserID: user.ID, TenantID: user.TenantID}
	switch strings.ToLower(strings.TrimSpace(body.Action)) {
	case "create":
		startsAt, err := parseOptionalComplianceTime(body.StartsAt)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "startsAt 必须是 RFC3339 时间", nil)
			return
		}
		expiresAt, err := parseOptionalComplianceTime(body.ExpiresAt)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "expiresAt 必须是 RFC3339 时间", nil)
			return
		}
		hold, err := manager.CreateLegalHold(r.Context(), saascompliance.LegalHoldCreate{
			TenantID: body.TenantID, Reason: body.Reason, StartsAt: startsAt, ExpiresAt: expiresAt, Actor: actor,
		})
		if err != nil {
			writeSaaSComplianceError(w, err)
			return
		}
		writeEnvelope(w, http.StatusCreated, http.StatusCreated, "success", map[string]any{"hold": saasComplianceLegalHoldPayload(hold)})
	case "release":
		if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionComplianceLegalHoldRelease, 0) {
			return
		}
		hold, err := manager.ReleaseLegalHold(r.Context(), saascompliance.LegalHoldRelease{
			HoldID: body.HoldID, Reason: body.Reason, ExpectedVersion: body.ExpectedVersion, Actor: actor,
		})
		if err != nil {
			writeSaaSComplianceError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"hold": saasComplianceLegalHoldPayload(hold)})
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "action 必须是 create 或 release", nil)
	}
}

func (h *SaaSAdminHandler) ComplianceExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasComplianceManager(w)
	if !ok {
		return
	}
	var body struct {
		Action   string `json:"action"`
		ExportID int64  `json:"exportId"`
		TenantID int    `json:"tenantId"`
		Reason   string `json:"reason"`
	}
	if err := decodeSaaSComplianceJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	actor := saascompliance.Actor{UserID: user.ID, TenantID: user.TenantID}
	switch strings.ToLower(strings.TrimSpace(body.Action)) {
	case "request":
		item, err := manager.RequestExport(r.Context(), body.TenantID, body.Reason, actor)
		if err != nil {
			writeSaaSComplianceError(w, err)
			return
		}
		writeEnvelope(w, http.StatusCreated, http.StatusCreated, "success", map[string]any{"export": saasComplianceExportPayload(item)})
	case "process":
		item, err := manager.ProcessExport(r.Context(), body.ExportID, actor)
		if err != nil {
			writeSaaSComplianceError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"export": saasComplianceExportPayload(item)})
	case "delete":
		if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionComplianceExportDelete, 0) {
			return
		}
		item, err := manager.DeleteExport(r.Context(), body.ExportID, actor)
		if err != nil {
			writeSaaSComplianceError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"export": saasComplianceExportPayload(item)})
	case "delete-retry":
		item, err := manager.RetryExportDeletion(r.Context(), body.ExportID, actor)
		if err != nil {
			writeSaaSComplianceError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"export": saasComplianceExportPayload(item)})
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "action 必须是 request、process、delete 或 delete-retry", nil)
	}
}

func (h *SaaSAdminHandler) ComplianceExportDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	opener := h.complianceExportOpener
	if opener == nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "租户数据合规能力未配置", nil)
		return
	}
	exportID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("exportId")), 10, 64)
	download, err := opener.OpenExport(r.Context(), exportID, saascompliance.Actor{UserID: user.ID, TenantID: user.TenantID})
	if err != nil {
		writeSaaSComplianceError(w, err)
		return
	}
	defer download.Reader.Close()
	httpresponse.AllowLongWrite(w)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, download.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, download.Reader); err != nil {
		return
	}
}

func (h *SaaSAdminHandler) ComplianceErasure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasComplianceManager(w)
	if !ok {
		return
	}
	var body struct {
		Action       string `json:"action"`
		RequestID    int64  `json:"requestId"`
		TenantID     int    `json:"tenantId"`
		Confirmation string `json:"confirmation"`
		Reason       string `json:"reason"`
	}
	if err := decodeSaaSComplianceJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	actor := saascompliance.Actor{UserID: user.ID, TenantID: user.TenantID}
	switch strings.ToLower(strings.TrimSpace(body.Action)) {
	case "request":
		item, err := manager.RequestErasure(r.Context(), body.TenantID, body.Confirmation, body.Reason, actor)
		if err != nil {
			writeSaaSComplianceError(w, err)
			return
		}
		writeEnvelope(w, http.StatusCreated, http.StatusCreated, "success", map[string]any{
			"erasure":  saasComplianceErasurePayload(item),
			"approval": map[string]any{"actionType": SaaSAdminApprovalActionTenantDataErase, "payload": map[string]any{"requestId": item.ID}},
		})
	case "process":
		item, err := manager.ProcessErasure(r.Context(), body.RequestID, actor)
		if err != nil {
			writeSaaSComplianceError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"erasure": saasComplianceErasurePayload(item)})
	case "cancel":
		item, err := manager.CancelErasure(r.Context(), body.RequestID, body.Reason, actor)
		if err != nil {
			writeSaaSComplianceError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"erasure": saasComplianceErasurePayload(item)})
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "action 必须是 request、process 或 cancel", nil)
	}
}

func (h *SaaSAdminHandler) ComplianceErasureSteps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	manager, ok := h.saasComplianceManager(w)
	if !ok {
		return
	}
	requestID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("requestId")), 10, 64)
	steps, err := manager.ErasureSteps(r.Context(), requestID)
	if err != nil {
		writeSaaSComplianceError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"requestId": requestID, "steps": saasComplianceErasureStepPayloads(steps)})
}

func (h *SaaSAdminHandler) saasComplianceManager(w http.ResponseWriter) (*saascompliance.Manager, bool) {
	if h.complianceManager == nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "SaaS 合规管理器未配置", nil)
		return nil, false
	}
	return h.complianceManager, true
}

func decodeSaaSComplianceJSON(r *http.Request, target any) error {
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

func parseOptionalComplianceTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func writeSaaSComplianceError(w http.ResponseWriter, err error) {
	var opErr *saascompliance.OperationError
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
		writeEnvelope(w, status, status, opErr.Message, nil)
		return
	}
	writeSaaSAdminError(w, err)
}

func saasComplianceAdminError(err error) error {
	if err == nil {
		return nil
	}
	var adminErr *SaaSAdminOperationError
	if errors.As(err, &adminErr) {
		return err
	}
	var opErr *saascompliance.OperationError
	if !errors.As(err, &opErr) || opErr == nil {
		return err
	}
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

func saasComplianceOverviewPayload(overview saascompliance.Overview) map[string]any {
	return map[string]any{
		"policy": saasCompliancePolicyPayload(overview.Policy), "config": saasComplianceConfigPayload(overview.Config),
		"holds": saasComplianceLegalHoldPayloads(overview.Holds), "exports": saasComplianceExportPayloads(overview.Exports),
		"erasures": saasComplianceErasurePayloads(overview.Erasures),
	}
}

func saasCompliancePolicyPayload(item saascompliance.Policy) map[string]any {
	return map[string]any{
		"id": item.ID, "status": item.Status, "exportRetentionDays": item.ExportRetentionDays,
		"erasureGraceDays": item.ErasureGraceDays, "requireRecentExport": item.RequireRecentExport,
		"recentExportMaxAgeDays": item.RecentExportMaxAgeDays, "billingRetentionDays": item.BillingRetentionDays,
		"auditRetentionDays": item.AuditRetentionDays, "serviceAccountUsageRetentionDays": item.ServiceAccountUsageRetentionDays,
		"version": item.Version, "updatedBy": item.UpdatedBy,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasComplianceConfigPayload(item saascompliance.ConfigStatus) map[string]any {
	return map[string]any{
		"artifactRootConfigured": item.ArtifactRootConfigured, "fileStorageConfigured": item.FileStorageConfigured,
		"encryptionConfigured": item.EncryptionConfigured, "encryptionKeyId": item.EncryptionKeyID,
		"encryptionKeyCount": item.EncryptionKeyCount, "inventoryVersion": item.InventoryVersion,
		"inventoryTableCount": item.InventoryTableCount, "inventoryUnknownTables": item.InventoryUnknownTables,
	}
}

func saasComplianceLegalHoldPayload(item saascompliance.LegalHold) map[string]any {
	return map[string]any{
		"id": item.ID, "holdNo": item.HoldNo, "tenantId": item.TenantID, "tenantName": item.TenantName,
		"status": item.Status, "reason": item.Reason, "startsAt": item.StartsAt, "expiresAt": item.ExpiresAt,
		"releasedAt": item.ReleasedAt, "releasedBy": item.ReleasedBy, "releaseReason": item.ReleaseReason,
		"version": item.Version, "createdBy": item.CreatedBy, "updatedBy": item.UpdatedBy,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt, "operationId": item.OperationID,
	}
}

func saasComplianceLegalHoldPayloads(items []saascompliance.LegalHold) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasComplianceLegalHoldPayload(item))
	}
	return result
}

func saasComplianceExportPayload(item saascompliance.DataExport) map[string]any {
	return map[string]any{
		"id": item.ID, "exportNo": item.ExportNo, "tenantId": item.TenantID, "tenantName": item.TenantName,
		"status": item.Status, "artifactFormat": item.ArtifactFormat, "encrypted": item.Encrypted,
		"encryptionKeyId": item.EncryptionKeyID, "encryptionKeyReady": item.EncryptionKeyReady,
		"sha256": item.SHA256, "sizeBytes": item.SizeBytes, "manifestSha256": item.ManifestSHA256,
		"inventoryVersion": item.InventoryVersion, "tableCount": item.TableCount, "rowCount": item.RowCount,
		"fileCount": item.FileCount, "fileSizeBytes": item.FileSizeBytes, "requestReason": item.RequestReason,
		"requestedBy": item.RequestedBy, "startedAt": item.StartedAt, "finishedAt": item.FinishedAt,
		"expiresAt": item.ExpiresAt, "errorMessage": item.ErrorMessage, "downloadCount": item.DownloadCount,
		"lastDownloadedAt": item.LastDownloadedAt, "lastDownloadedBy": item.LastDownloadedBy,
		"operationId":    item.OperationID,
		"deletionStatus": item.DeletionStatus, "deletionArtifactStatus": item.DeletionArtifactStatus,
		"deletionRecordStatus": item.DeletionRecordStatus, "deletionAttempts": item.DeletionAttempts,
		"deletionApprovalId": item.DeletionApprovalID, "deletionLeaseExpiresAt": item.DeletionLeaseExpiresAt,
		"deletionRequestedAt": item.DeletionRequestedAt, "deletionStartedAt": item.DeletionStartedAt,
		"deletionFinishedAt": item.DeletionFinishedAt, "deletionLastError": item.DeletionLastError,
		"deletionRequestedBy": item.DeletionRequestedBy,
		"version":             item.Version, "createdAt": item.CreatedAt, "deletedAt": item.DeletedAt,
	}
}

func saasComplianceExportPayloads(items []saascompliance.DataExport) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasComplianceExportPayload(item))
	}
	return result
}

func saasComplianceErasurePayload(item saascompliance.ErasureRequest) map[string]any {
	return map[string]any{
		"id": item.ID, "requestNo": item.RequestNo, "tenantId": item.TenantID, "tenantName": item.TenantName,
		"status": item.Status, "reason": item.Reason, "eligibleAt": item.EligibleAt, "latestExportId": item.LatestExportID,
		"approvalId": item.ApprovalID, "approvedAt": item.ApprovedAt, "approvedBy": item.ApprovedBy,
		"inventoryVersion": item.InventoryVersion, "totalSteps": item.TotalSteps, "completedSteps": item.CompletedSteps,
		"deletedRows": item.DeletedRows, "redactedRows": item.RedactedRows, "verificationSha256": item.VerificationSHA256,
		"report": rawJSONPayload(item.ReportJSON), "lastError": item.LastError, "requestedBy": item.RequestedBy,
		"startedAt": item.StartedAt, "finishedAt": item.FinishedAt, "operationId": item.OperationID,
		"version": item.Version, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasComplianceErasurePayloads(items []saascompliance.ErasureRequest) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasComplianceErasurePayload(item))
	}
	return result
}

func saasComplianceErasureStepPayloads(items []saascompliance.ErasureStep) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id": item.ID, "requestId": item.RequestID, "stepOrder": item.StepOrder, "stepKey": item.StepKey,
			"tableName": item.TableName, "action": item.Action, "status": item.Status, "affectedRows": item.AffectedRows,
			"startedAt": item.StartedAt, "finishedAt": item.FinishedAt, "errorMessage": item.ErrorMessage,
		})
	}
	return result
}

func rawJSONPayload(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var result any
	if json.Unmarshal([]byte(value), &result) != nil {
		return nil
	}
	return result
}
