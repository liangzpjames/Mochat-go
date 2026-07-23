package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/outboundhttp"
)

const (
	SaaSAdminNotificationPolicyStateAll          = "all"
	SaaSAdminNotificationPolicyStateEnabled      = "enabled"
	SaaSAdminNotificationPolicyStateDisabled     = "disabled"
	SaaSAdminNotificationPolicyStateUnconfigured = "unconfigured"
)

type SaaSAdminNotificationPolicyOptions struct {
	TenantID int
	Channel  string
	State    string
	Keyword  string
	Limit    int
}

type SaaSAdminNotificationPolicy struct {
	TenantID     int
	TenantName   string
	TenantStatus int
	PackageCode  string
	PackageName  string
	Configured   bool
	Setting      SaaSAlertSetting
}

type SaaSAdminNotificationPolicySummary struct {
	TenantCount       int
	ConfiguredCount   int
	EnabledCount      int
	DisabledCount     int
	UnconfiguredCount int
	MatchedCount      int
}

type SaaSAdminNotificationPolicyReport struct {
	Options  SaaSAdminNotificationPolicyOptions
	Summary  SaaSAdminNotificationPolicySummary
	Policies []SaaSAdminNotificationPolicy
}

func (h *SaaSAdminHandler) NotificationPolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := saasAdminNotificationPolicyOptions(w, r)
	if !ok {
		return
	}
	report, err := h.store.SaaSAdminNotificationPolicies(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	credentialProtection, err := h.saasAlertCredentialProtectionPayload(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	payload := saasAdminNotificationPolicyReportPayload(report, h.platformAdminTenantID, h.webhookGuard)
	payload["credentialProtection"] = credentialProtection
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *SaaSAdminHandler) NotificationPolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.showNotificationPolicy(w, r)
	case http.MethodPost, http.MethodPut:
		h.saveNotificationPolicy(w, r)
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (h *SaaSAdminHandler) showNotificationPolicy(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	tenantID := saasAdminQueryInt(r, "tenantId", 0)
	if tenantID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId 必传", nil)
		return
	}
	channel, ok := saasAdminNotificationPolicyChannel(w, r.URL.Query().Get("channel"))
	if !ok {
		return
	}
	policy, found, err := h.notificationPolicyByTenant(r, tenantID, channel)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "tenant not found", nil)
		return
	}
	credentialProtection, err := h.saasAlertCredentialProtectionPayload(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"platformAdminTenantId": h.platformAdminTenantID,
		"policy":                saasAdminNotificationPolicyPayload(policy, h.webhookGuard),
		"webhookSecurity":       saasAlertWebhookSecurityPayload(h.webhookGuard),
		"credentialProtection":  credentialProtection,
	})
}

func (h *SaaSAdminHandler) saveNotificationPolicy(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	tenantID, found, err := intParam(params, "tenantId")
	if err != nil || !found || tenantID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId 必须是正整数", nil)
		return
	}
	channel, ok := saasAdminNotificationPolicyChannel(w, stringParam(params, "channel"))
	if !ok {
		return
	}
	policy, exists, err := h.notificationPolicyByTenant(r, tenantID, channel)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if !exists {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "tenant not found", nil)
		return
	}
	setting := policy.Setting
	setting.TenantID = tenantID
	setting.Channel = channel
	before := SaaSAlertSettingPayload(setting, policy.Configured)
	setting, err = applySaaSAlertSettingParams(setting, params)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := validateSaaSAlertWebhookURLSecurity(h.webhookGuard, setting.WebhookURL); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	remark, ok := saasAdminPolicyRemark(w, stringParam(params, "remark"), "平台更新租户通知策略")
	if !ok {
		return
	}
	saved, err := h.store.SaveSaaSAlertSetting(r.Context(), setting)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	after := SaaSAlertSettingPayload(saved, true)
	operationID, err := h.store.RecordSaaSAdminOperationLog(r.Context(), SaaSAdminOperationLog{
		TenantID:      tenantID,
		ActorUserID:   user.ID,
		ActorTenantID: user.TenantID,
		Action:        SaaSAdminOperationActionNotificationPolicyUpdate,
		TargetType:    SaaSAdminOperationTargetNotificationPolicy,
		TargetID:      strconv.Itoa(tenantID) + ":" + channel,
		TargetName:    policy.TenantName,
		BeforeJSON:    saasAdminPayloadJSON(before),
		AfterJSON:     saasAdminPayloadJSON(after),
		Remark:        remark,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	policy.Configured = true
	policy.Setting = saved
	credentialProtection, err := h.saasAlertCredentialProtectionPayload(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"platformAdminTenantId": h.platformAdminTenantID,
		"operationId":           operationID,
		"policy":                saasAdminNotificationPolicyPayload(policy, h.webhookGuard),
		"webhookSecurity":       saasAlertWebhookSecurityPayload(h.webhookGuard),
		"credentialProtection":  credentialProtection,
	})
}

func (h *SaaSAdminHandler) NotificationCredentialRotation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	credentialStore, ok := h.store.(SaaSAlertCredentialProtectionStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "notification credential protection is not available", nil)
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
	remark, ok := saasAdminPolicyRemark(w, stringParam(params, "remark"), "平台轮换通知 webhook 凭据密钥")
	if !ok {
		return
	}
	before, err := credentialStore.SaaSAlertCredentialProtection(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result, err := credentialStore.RotateSaaSAlertCredentials(r.Context(), tenantID, limit)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	after, err := credentialStore.SaaSAlertCredentialProtection(r.Context())
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
		Action:        SaaSAdminOperationActionNotificationCredentialRotate,
		TargetType:    SaaSAdminOperationTargetNotificationCredential,
		TargetID:      targetID,
		TargetName:    "通知 webhook 凭据",
		BeforeJSON:    saasAdminPayloadJSON(saasAlertCredentialProtectionStatusPayload(before, true)),
		AfterJSON: saasAdminPayloadJSON(map[string]any{
			"rotation":   result,
			"protection": saasAlertCredentialProtectionStatusPayload(after, true),
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
		"credentialProtection":  saasAlertCredentialProtectionStatusPayload(after, true),
	})
}

func (h *SaaSAdminHandler) saasAlertCredentialProtectionPayload(ctx context.Context) (map[string]any, error) {
	credentialStore, ok := h.store.(SaaSAlertCredentialProtectionStore)
	if !ok {
		return saasAlertCredentialProtectionStatusPayload(SaaSAlertCredentialProtectionStatus{}, false), nil
	}
	status, err := credentialStore.SaaSAlertCredentialProtection(ctx)
	if err != nil {
		return nil, err
	}
	return saasAlertCredentialProtectionStatusPayload(status, true), nil
}

func saasAlertCredentialProtectionStatusPayload(status SaaSAlertCredentialProtectionStatus, supported bool) map[string]any {
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

func (h *SaaSAdminHandler) NotificationPolicyTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	tenantID, found, err := intParam(params, "tenantId")
	if err != nil || !found || tenantID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId 必须是正整数", nil)
		return
	}
	channel, ok := saasAdminNotificationPolicyChannel(w, stringParam(params, "channel"))
	if !ok {
		return
	}
	policy, exists, err := h.notificationPolicyByTenant(r, tenantID, channel)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if !exists {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "tenant not found", nil)
		return
	}
	setting := NormalizeSaaSAlertSetting(policy.Setting)
	if !policy.Configured || !setting.Enabled || strings.TrimSpace(setting.WebhookURL) == "" {
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, "notification policy must be configured and enabled before testing", nil)
		return
	}
	if err := validateSaaSAlertWebhookURLSecurity(h.webhookGuard, setting.WebhookURL); err != nil {
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, err.Error(), nil)
		return
	}
	remark, ok := saasAdminPolicyRemark(w, stringParam(params, "remark"), "平台生成通知策略测试消息")
	if !ok {
		return
	}
	now := time.Now()
	alert := SaaSQuotaAlert{
		Status: SaaSQuotaStatus{
			Metric:   SaaSEventMetricNotificationPolicy,
			TenantID: tenantID,
		},
		AlertType: SaaSAlertTypeNotificationPolicyTest,
		Severity:  SaaSAlertSeverityWarning,
		PeriodKey: fmt.Sprintf("notification_policy_test_%d", now.UnixNano()),
		Source:    "saas_admin",
		Message:   "SaaS 通知策略测试消息",
		Context: map[string]any{
			"actorUserId":   user.ID,
			"actorTenantId": user.TenantID,
			"tenantName":    policy.TenantName,
			"remark":        remark,
		},
	}
	notification, err := h.store.EnqueueSaaSAlertNotification(r.Context(), alert, channel, setting.NotificationMaxAttempts)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	operationID, err := h.store.RecordSaaSAdminOperationLog(r.Context(), SaaSAdminOperationLog{
		TenantID:      tenantID,
		ActorUserID:   user.ID,
		ActorTenantID: user.TenantID,
		Action:        SaaSAdminOperationActionNotificationPolicyTest,
		TargetType:    SaaSAdminOperationTargetAlertNotification,
		TargetID:      strconv.FormatInt(notification.ID, 10),
		TargetName:    policy.TenantName,
		AfterJSON: saasAdminPayloadJSON(map[string]any{
			"notification": saasAdminAlertNotificationPayload(notification),
			"policy":       saasAdminNotificationPolicyPayload(policy, h.webhookGuard),
		}),
		Remark: remark,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"platformAdminTenantId": h.platformAdminTenantID,
		"operationId":           operationID,
		"notification":          saasAdminAlertNotificationPayload(notification),
	})
}

func (h *SaaSAdminHandler) notificationPolicyByTenant(r *http.Request, tenantID int, channel string) (SaaSAdminNotificationPolicy, bool, error) {
	report, err := h.store.SaaSAdminNotificationPolicies(r.Context(), SaaSAdminNotificationPolicyOptions{
		TenantID: tenantID,
		Channel:  channel,
		State:    SaaSAdminNotificationPolicyStateAll,
		Limit:    1,
	})
	if err != nil {
		return SaaSAdminNotificationPolicy{}, false, err
	}
	if len(report.Policies) == 0 {
		return SaaSAdminNotificationPolicy{}, false, nil
	}
	return report.Policies[0], true, nil
}

func saasAdminNotificationPolicyOptions(w http.ResponseWriter, r *http.Request) (SaaSAdminNotificationPolicyOptions, bool) {
	channel, ok := saasAdminNotificationPolicyChannel(w, r.URL.Query().Get("channel"))
	if !ok {
		return SaaSAdminNotificationPolicyOptions{}, false
	}
	state := strings.ToLower(strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("state"), r.URL.Query().Get("policyState"), r.URL.Query().Get("policy_state"))))
	switch state {
	case "", SaaSAdminNotificationPolicyStateAll:
		state = SaaSAdminNotificationPolicyStateAll
	case SaaSAdminNotificationPolicyStateEnabled, SaaSAdminNotificationPolicyStateDisabled, SaaSAdminNotificationPolicyStateUnconfigured:
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "state 必须是 all/enabled/disabled/unconfigured", nil)
		return SaaSAdminNotificationPolicyOptions{}, false
	}
	keyword, ok := saasAdminQueryString(w, "keyword", 80, r.URL.Query().Get("keyword"))
	if !ok {
		return SaaSAdminNotificationPolicyOptions{}, false
	}
	limit := positiveQueryInt(r, "limit", 50)
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	return SaaSAdminNotificationPolicyOptions{
		TenantID: saasAdminQueryInt(r, "tenantId", 0),
		Channel:  channel,
		State:    state,
		Keyword:  keyword,
		Limit:    limit,
	}, true
}

func saasAdminNotificationPolicyChannel(w http.ResponseWriter, raw string) (string, bool) {
	channel := strings.ToLower(strings.TrimSpace(raw))
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	if channel != SaaSAlertNotificationChannelWebhook {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "channel 目前仅支持 webhook", nil)
		return "", false
	}
	return channel, true
}

func saasAdminPolicyRemark(w http.ResponseWriter, raw string, fallback string) (string, bool) {
	remark := strings.TrimSpace(raw)
	if remark == "" {
		remark = fallback
	}
	if len([]rune(remark)) > 255 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "remark 最多 255 个字符", nil)
		return "", false
	}
	return remark, true
}

func saasAdminNotificationPolicyReportPayload(report SaaSAdminNotificationPolicyReport, platformAdminTenantID int, guard *outboundhttp.Guard) map[string]any {
	policies := make([]map[string]any, 0, len(report.Policies))
	for _, policy := range report.Policies {
		policies = append(policies, saasAdminNotificationPolicyPayload(policy, guard))
	}
	return map[string]any{
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters": map[string]any{
			"tenantId": report.Options.TenantID,
			"channel":  report.Options.Channel,
			"state":    report.Options.State,
			"keyword":  report.Options.Keyword,
			"limit":    report.Options.Limit,
		},
		"summary": map[string]any{
			"tenantCount":       report.Summary.TenantCount,
			"configuredCount":   report.Summary.ConfiguredCount,
			"enabledCount":      report.Summary.EnabledCount,
			"disabledCount":     report.Summary.DisabledCount,
			"unconfiguredCount": report.Summary.UnconfiguredCount,
			"matchedCount":      report.Summary.MatchedCount,
		},
		"returnedCount":   len(policies),
		"policies":        policies,
		"webhookSecurity": saasAlertWebhookSecurityPayload(guard),
	}
}

func saasAdminNotificationPolicyPayload(policy SaaSAdminNotificationPolicy, guard *outboundhttp.Guard) map[string]any {
	setting := SaaSAlertSettingPayload(policy.Setting, policy.Configured)
	setting["urlSecurity"] = saasAlertWebhookURLSecurityPayload(guard, policy.Setting.WebhookURL)
	return map[string]any{
		"tenantId":     policy.TenantID,
		"tenantName":   policy.TenantName,
		"tenantStatus": policy.TenantStatus,
		"packageCode":  policy.PackageCode,
		"packageName":  policy.PackageName,
		"configured":   policy.Configured,
		"setting":      setting,
	}
}
