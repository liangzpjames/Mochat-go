package dashboard

import (
	"context"
	"net/http"
	"strconv"
)

func (h *SaaSAdminHandler) WeComCredentialProtection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	payload, err := h.saasWeComCredentialProtectionPayload(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"platformAdminTenantId": h.platformAdminTenantID,
		"credentialProtection":  payload,
	})
}

func (h *SaaSAdminHandler) WeComCredentialRotation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	credentialStore, ok := h.store.(SaaSWeComCredentialProtectionStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "WeCom credential protection is not available", nil)
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	tenantID, tenantIDFound, err := intParam(params, "tenantId")
	if err != nil || (tenantIDFound && tenantID < 0) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId 必须是非负整数", nil)
		return
	}
	limit := 100
	if value, found, parseErr := intParam(params, "limit"); parseErr != nil || (found && (value <= 0 || value > 1000)) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "limit 必须是 1 到 1000 的整数", nil)
		return
	} else if found {
		limit = value
	}
	remark, ok := saasAdminPolicyRemark(w, stringParam(params, "remark"), "平台轮换企业微信凭据密钥")
	if !ok {
		return
	}
	before, err := credentialStore.SaaSWeComCredentialProtection(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result, err := credentialStore.RotateSaaSWeComCredentials(r.Context(), tenantID, limit)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	after, err := credentialStore.SaaSWeComCredentialProtection(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	targetID := "all"
	logTenantID := h.platformAdminTenantID
	if tenantID > 0 {
		targetID = strconv.Itoa(tenantID)
		logTenantID = tenantID
	}
	operationID, err := h.store.RecordSaaSAdminOperationLog(r.Context(), SaaSAdminOperationLog{
		TenantID:      logTenantID,
		ActorUserID:   user.ID,
		ActorTenantID: user.TenantID,
		Action:        SaaSAdminOperationActionWeComCredentialRotate,
		TargetType:    SaaSAdminOperationTargetWeComCredential,
		TargetID:      targetID,
		TargetName:    "企业微信凭据",
		BeforeJSON:    saasAdminPayloadJSON(saasWeComCredentialProtectionStatusPayload(before, true)),
		AfterJSON: saasAdminPayloadJSON(map[string]any{
			"rotation":   result,
			"protection": saasWeComCredentialProtectionStatusPayload(after, true),
		}),
		Remark: remark,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"platformAdminTenantId": h.platformAdminTenantID,
		"operationId":           operationID,
		"rotation":              result,
		"credentialProtection":  saasWeComCredentialProtectionStatusPayload(after, true),
	})
}

func (h *SaaSAdminHandler) saasWeComCredentialProtectionPayload(ctx context.Context) (map[string]any, error) {
	credentialStore, ok := h.store.(SaaSWeComCredentialProtectionStore)
	if !ok {
		return saasWeComCredentialProtectionStatusPayload(SaaSWeComCredentialProtectionStatus{}, false), nil
	}
	status, err := credentialStore.SaaSWeComCredentialProtection(ctx)
	if err != nil {
		return nil, err
	}
	return saasWeComCredentialProtectionStatusPayload(status, true), nil
}

func saasWeComCredentialProtectionStatusPayload(status SaaSWeComCredentialProtectionStatus, supported bool) map[string]any {
	unavailableKeyIDs := append([]string(nil), status.UnavailableKeyIDs...)
	if unavailableKeyIDs == nil {
		unavailableKeyIDs = []string{}
	}
	return map[string]any{
		"supported":                 supported,
		"encryptionConfigured":      status.EncryptionConfigured,
		"requireEncryption":         status.RequireEncryption,
		"dedicatedConfigured":       status.DedicatedConfigured,
		"activeKeyId":               status.ActiveKeyID,
		"keyCount":                  status.KeyCount,
		"configuredCredentialCount": status.ConfiguredCredentialCount,
		"corpCredentialCount":       status.CorpCredentialCount,
		"agentCredentialCount":      status.AgentCredentialCount,
		"encryptedCredentialCount":  status.EncryptedCredentialCount,
		"legacyPlaintextCount":      status.LegacyPlaintextCount,
		"activeKeyCredentialCount":  status.ActiveKeyCredentialCount,
		"rotationRequiredCount":     status.RotationRequiredCount,
		"unavailableKeyCount":       status.UnavailableKeyCount,
		"decryptFailureCount":       status.DecryptFailureCount,
		"unavailableKeyIds":         unavailableKeyIDs,
		"healthy":                   status.Healthy,
		"rotationAvailable":         supported && status.EncryptionConfigured,
	}
}
