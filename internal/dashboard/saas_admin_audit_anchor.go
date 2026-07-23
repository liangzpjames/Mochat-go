package dashboard

import (
	"errors"
	"net/http"
	"strings"

	"jiyi/mochat-go/internal/saasauditanchor"
)

func (h *SaaSAdminHandler) WithAuditAnchorManager(manager *saasauditanchor.Manager) *SaaSAdminHandler {
	h.auditAnchorManager = manager
	return h
}

func (h *SaaSAdminHandler) AuditAnchors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	manager, ok := h.saasAuditAnchorManager(w)
	if !ok {
		return
	}
	tenantID := saasAdminQueryInt(r, "tenantId", 0)
	limit := saasAdminQueryInt(r, "limit", 50)
	overview, err := manager.Overview(r.Context(), tenantID, limit)
	if err != nil {
		writeSaaSAuditAnchorError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", saasAuditAnchorOverviewPayload(overview))
}

func (h *SaaSAdminHandler) AuditAnchor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasAuditAnchorManager(w)
	if !ok {
		return
	}
	var body struct {
		Action   string `json:"action"`
		TenantID int    `json:"tenantId"`
		Limit    int    `json:"limit"`
	}
	if !decodeSaaSServiceAccountJSON(w, r, &body) {
		return
	}
	if body.Limit == 0 {
		body.Limit = 50
	}
	actor := saasauditanchor.Actor{UserID: user.ID, TenantID: user.TenantID}
	switch strings.ToLower(strings.TrimSpace(body.Action)) {
	case "create":
		result, err := manager.Create(r.Context(), saasauditanchor.CreateOptions{
			TenantID: body.TenantID, Limit: body.Limit, Source: saasauditanchor.TriggerManual, Actor: actor,
		})
		if err != nil {
			writeSaaSAuditAnchorError(w, err)
			return
		}
		writeEnvelope(w, http.StatusCreated, http.StatusCreated, "ok", map[string]any{
			"action": "create", "result": saasAuditAnchorCreateResultPayload(result),
		})
	case "verify":
		result, err := manager.Verify(r.Context(), saasauditanchor.VerifyOptions{
			TenantID: body.TenantID, Limit: body.Limit, Source: saasauditanchor.TriggerManual, Actor: actor,
		})
		if err != nil {
			writeSaaSAuditAnchorError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{
			"action": "verify", "result": saasAuditAnchorVerifyResultPayload(result),
		})
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "action 必须是 create 或 verify", nil)
	}
}

func (h *SaaSAdminHandler) saasAuditAnchorManager(w http.ResponseWriter) (*saasauditanchor.Manager, bool) {
	if h.auditAnchorManager == nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "audit anchor manager is not configured", nil)
		return nil, false
	}
	return h.auditAnchorManager, true
}

func writeSaaSAuditAnchorError(w http.ResponseWriter, err error) {
	var opErr *saasauditanchor.OperationError
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

func saasAuditAnchorOverviewPayload(overview saasauditanchor.Overview) map[string]any {
	return map[string]any{
		"config": map[string]any{
			"artifactRoot":               overview.Config.ArtifactRoot,
			"artifactRootConfigured":     overview.Config.ArtifactRootConfigured,
			"hmacConfigured":             overview.Config.HMACConfigured,
			"hmacKeyId":                  overview.Config.HMACKeyID,
			"hmacKeyCount":               overview.Config.HMACKeyCount,
			"missingKeyIds":              overview.Config.MissingKeyIDs,
			"independentStorageRequired": overview.Config.IndependentStorageRequired,
			"remoteConfigured":           overview.Config.RemoteConfigured,
			"remoteRequired":             overview.Config.RemoteRequired,
			"remoteProvider":             overview.Config.RemoteProvider,
			"remoteBucket":               overview.Config.RemoteBucket,
			"remotePrefix":               overview.Config.RemotePrefix,
			"remoteRetentionMode":        overview.Config.RemoteRetentionMode,
			"remoteRetentionDays":        overview.Config.RemoteRetentionDays,
		},
		"summary": map[string]any{
			"checkpointCount":          overview.Summary.CheckpointCount,
			"exportedCount":            overview.Summary.ExportedCount,
			"artifactFailedCount":      overview.Summary.ArtifactFailedCount,
			"verificationPassedCount":  overview.Summary.VerificationPassedCount,
			"verificationFailedCount":  overview.Summary.VerificationFailedCount,
			"pendingVerificationCount": overview.Summary.PendingVerificationCount,
			"orphanArtifactCount":      overview.Summary.OrphanArtifactCount,
			"remoteExportedCount":      overview.Summary.RemoteExportedCount,
			"remoteFailedCount":        overview.Summary.RemoteFailedCount,
			"remotePendingCount":       overview.Summary.RemotePendingCount,
			"orphanRemoteCount":        overview.Summary.OrphanRemoteCount,
			"latestSignedAt":           overview.Summary.LatestSignedAt,
			"latestVerifiedAt":         overview.Summary.LatestVerifiedAt,
			"latestRemoteVerifiedAt":   overview.Summary.LatestRemoteVerifiedAt,
		},
		"checkpoints": saasAuditAnchorCheckpointPayloads(overview.Checkpoints),
	}
}

func saasAuditAnchorCreateResultPayload(result saasauditanchor.CreateResult) map[string]any {
	return map[string]any{
		"scannedChains": result.ScannedChains, "createdCheckpoints": result.CreatedCheckpoints,
		"existingCheckpoints": result.ExistingCheckpoints, "backfilledCheckpoints": result.BackfilledCheckpoints,
		"exportedArtifacts": result.ExportedArtifacts,
		"failedArtifacts":   result.FailedArtifacts, "operationId": result.OperationID,
		"remoteExportedArtifacts": result.RemoteExportedArtifacts,
		"remoteFailedArtifacts":   result.RemoteFailedArtifacts,
		"createdAt":               result.CreatedAt, "checkpoints": saasAuditAnchorCheckpointPayloads(result.Checkpoints),
	}
}

func saasAuditAnchorVerifyResultPayload(result saasauditanchor.VerifyResult) map[string]any {
	return map[string]any{
		"scannedCheckpoints": result.ScannedCheckpoints, "passedCheckpoints": result.PassedCheckpoints,
		"failedCheckpoints": result.FailedCheckpoints, "missingKeyCount": result.MissingKeyCount,
		"missingArtifactCount": result.MissingArtifactCount, "orphanArtifactCount": result.OrphanArtifactCount,
		"missingRemoteCount": result.MissingRemoteCount, "orphanRemoteCount": result.OrphanRemoteCount,
		"operationId": result.OperationID, "verifiedAt": result.VerifiedAt,
		"checkpoints": saasAuditAnchorCheckpointPayloads(result.Checkpoints),
	}
}

func saasAuditAnchorCheckpointPayloads(items []saasauditanchor.Checkpoint) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id": item.ID, "checkpointNo": item.CheckpointNo, "tenantId": item.TenantID,
			"tenantName": item.TenantName, "chainVersion": item.ChainVersion,
			"anchorLogId": item.AnchorLogID, "anchorHash": item.AnchorHash,
			"legacyLogCount": item.LegacyLogCount, "chainHeadLogId": item.ChainHeadLogID,
			"chainHeadHash": item.ChainHeadHash, "signedLogCount": item.SignedLogCount,
			"signatureAlgorithm": item.SignatureAlgorithm, "keyId": item.KeyID,
			"payloadSha256": item.PayloadSHA256, "signature": item.Signature,
			"artifactName": item.ArtifactName, "artifactSha256": item.ArtifactSHA256,
			"artifactStatus": item.ArtifactStatus, "artifactError": item.ArtifactError,
			"remoteStatus": item.RemoteStatus, "remoteProvider": item.RemoteProvider,
			"remoteBucket": item.RemoteBucket, "remoteObjectKey": item.RemoteObjectKey,
			"remoteEtag": item.RemoteETag, "remoteVersionId": item.RemoteVersionID,
			"remoteSha256": item.RemoteSHA256, "remoteSizeBytes": item.RemoteSizeBytes,
			"remoteRetentionMode": item.RemoteRetentionMode, "remoteRetainUntil": item.RemoteRetainUntil,
			"remoteError": item.RemoteError, "remoteExportedAt": item.RemoteExportedAt,
			"remoteVerifiedAt":   item.RemoteVerifiedAt,
			"verificationStatus": item.VerificationStatus, "verificationError": item.VerificationError,
			"source": item.Source, "actorUserId": item.ActorUserID, "actorTenantId": item.ActorTenantID,
			"signedAt": item.SignedAt, "exportedAt": item.ExportedAt,
			"lastVerifiedAt": item.LastVerifiedAt, "createdAt": item.CreatedAt,
		})
	}
	return result
}
