package dashboard

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type saasAdminApprovalPolicyBody struct {
	ActionType           string `json:"actionType"`
	Enabled              bool   `json:"enabled"`
	AmountThresholdCents int64  `json:"amountThresholdCents"`
	RequiredApprovals    int    `json:"requiredApprovals"`
	SLAMinutes           int    `json:"slaMinutes"`
	ReminderMinutes      int    `json:"reminderMinutes"`
	ExpiryHours          int    `json:"expiryHours"`
	ExpectedVersion      int    `json:"expectedVersion"`
}

type saasAdminApprovalDelegationBody struct {
	ID              int64  `json:"id"`
	DelegatorUserID int    `json:"delegatorUserId"`
	DelegateUserID  int    `json:"delegateUserId"`
	StartsAt        string `json:"startsAt"`
	EndsAt          string `json:"endsAt"`
	Status          int    `json:"status"`
	Reason          string `json:"reason"`
	ExpectedVersion int    `json:"expectedVersion"`
}

type saasAdminApprovalReminderBody struct {
	Limit       int `json:"limit"`
	MaxAttempts int `json:"maxAttempts"`
}

func validateSaaSAdminApprovalPolicyBody(body *saasAdminApprovalPolicyBody) error {
	body.ActionType = strings.TrimSpace(body.ActionType)
	definition, found := SaaSAdminApprovalPolicyByAction(body.ActionType)
	if !found {
		return NewSaaSAdminBadRequest("actionType 无效")
	}
	if body.ExpectedVersion <= 0 || body.AmountThresholdCents < 0 || body.RequiredApprovals < 1 || body.RequiredApprovals > 5 ||
		body.SLAMinutes < 15 || body.SLAMinutes > 10080 || body.ReminderMinutes < 5 || body.ReminderMinutes > body.SLAMinutes ||
		body.ExpiryHours < 1 || body.ExpiryHours > 168 {
		return NewSaaSAdminBadRequest("策略版本、阈值、会签人数、SLA、提醒间隔或有效期无效")
	}
	if body.ActionType != SaaSAdminApprovalActionPaymentRefundCreate && body.AmountThresholdCents != 0 {
		return NewSaaSAdminBadRequest("金额阈值仅适用于退款审批")
	}
	if saasAdminApprovalPolicyGovernanceLocked(definition) &&
		(!body.Enabled || body.AmountThresholdCents != 0 || body.RequiredApprovals < saasAdminApprovalPolicyMinimumApprovals(definition)) {
		return NewSaaSAdminBadRequest("严重风险门禁必须启用、不得设置绕过阈值且至少需要两人会签")
	}
	return nil
}

func (h *SaaSAdminHandler) effectiveSaaSAdminApprovalPolicies(ctx context.Context) ([]SaaSAdminApprovalPolicy, error) {
	defaults := SaaSAdminApprovalPolicies()
	store, ok := h.store.(SaaSAdminApprovalGovernanceStore)
	if !ok || store == nil {
		return defaults, nil
	}
	stored, err := store.SaaSAdminApprovalPolicies(ctx)
	if err != nil {
		return nil, err
	}
	byAction := make(map[string]SaaSAdminApprovalPolicy, len(stored))
	for _, item := range stored {
		byAction[item.ActionType] = item
	}
	for index := range defaults {
		configured, found := byAction[defaults[index].ActionType]
		if !found {
			continue
		}
		defaults[index].Enabled = configured.Enabled
		defaults[index].AmountThresholdCents = configured.AmountThresholdCents
		defaults[index].RequiredApprovals = configured.RequiredApprovals
		defaults[index].SLAMinutes = configured.SLAMinutes
		defaults[index].ReminderMinutes = configured.ReminderMinutes
		defaults[index].ExpiryHours = configured.ExpiryHours
		defaults[index].Version = configured.Version
		defaults[index].UpdatedBy = configured.UpdatedBy
		defaults[index].UpdatedAt = configured.UpdatedAt
		defaults[index] = enforceSaaSAdminApprovalPolicyGovernance(defaults[index])
	}
	return defaults, nil
}

func (h *SaaSAdminHandler) effectiveSaaSAdminApprovalPolicy(ctx context.Context, actionType string) (SaaSAdminApprovalPolicy, error) {
	definition, found := SaaSAdminApprovalPolicyByAction(actionType)
	if !found {
		return SaaSAdminApprovalPolicy{}, NewSaaSAdminBadRequest("未知高风险动作: " + actionType)
	}
	store, ok := h.store.(SaaSAdminApprovalGovernanceStore)
	if !ok || store == nil {
		return definition, nil
	}
	configured, found, err := store.SaaSAdminApprovalPolicy(ctx, actionType)
	if err != nil {
		return SaaSAdminApprovalPolicy{}, err
	}
	if !found {
		return definition, nil
	}
	definition.Enabled = configured.Enabled
	definition.AmountThresholdCents = configured.AmountThresholdCents
	definition.RequiredApprovals = configured.RequiredApprovals
	definition.SLAMinutes = configured.SLAMinutes
	definition.ReminderMinutes = configured.ReminderMinutes
	definition.ExpiryHours = configured.ExpiryHours
	definition.Version = configured.Version
	definition.UpdatedBy = configured.UpdatedBy
	definition.UpdatedAt = configured.UpdatedAt
	return enforceSaaSAdminApprovalPolicyGovernance(definition), nil
}

// SaaSAdminApprovalPolicyForAction is used by the Dashboard governance HTTP
// adapter so direct writes and the generic approval center consult the same
// effective catalog and persisted policy version.
func (h *SaaSAdminHandler) SaaSAdminApprovalPolicyForAction(ctx context.Context, actionType string) (SaaSAdminApprovalPolicy, error) {
	return h.effectiveSaaSAdminApprovalPolicy(ctx, actionType)
}

func (h *SaaSAdminHandler) DirectSaaSAdminApprovalRequired(ctx context.Context, actionType string) (SaaSAdminApprovalPolicy, bool, error) {
	policy, err := h.effectiveSaaSAdminApprovalPolicy(ctx, actionType)
	if err != nil {
		return SaaSAdminApprovalPolicy{}, false, err
	}
	return policy, h.highRiskApproval && saasAdminApprovalPolicyRequires(policy, 0), nil
}

func (h *SaaSAdminHandler) ApprovalPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminApprovalGovernanceStore(w)
	if !ok {
		return
	}
	var body saasAdminApprovalPolicyBody
	if err := decodeSaaSAdminAccessJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := validateSaaSAdminApprovalPolicyBody(&body); err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionApprovalPolicyUpdate, 0) {
		return
	}
	result, err := store.UpdateSaaSAdminApprovalPolicy(r.Context(), SaaSAdminApprovalPolicyUpdate{
		ActionType: body.ActionType, Enabled: body.Enabled, AmountThresholdCents: body.AmountThresholdCents,
		RequiredApprovals: body.RequiredApprovals, SLAMinutes: body.SLAMinutes, ReminderMinutes: body.ReminderMinutes,
		ExpiryHours: body.ExpiryHours, ExpectedVersion: body.ExpectedVersion, ActorUserID: user.ID, ActorTenantID: user.TenantID,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	policy, err := h.effectiveSaaSAdminApprovalPolicy(r.Context(), body.ActionType)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	policy.Version, policy.UpdatedBy, policy.UpdatedAt = result.Policy.Version, result.Policy.UpdatedBy, result.Policy.UpdatedAt
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"policy": saasAdminApprovalPolicyPayload(policy), "operationId": result.OperationID})
}

func (h *SaaSAdminHandler) ApprovalDecisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminApprovalGovernanceStore(w)
	if !ok {
		return
	}
	approvalID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("approvalId")), 10, 64)
	limit := positiveQueryInt(r, "limit", 100)
	if approvalID <= 0 || limit > 500 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "approvalId 必填且 limit 不能超过 500", nil)
		return
	}
	items, err := store.SaaSAdminApprovalDecisions(r.Context(), approvalID, limit)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"decisions": saasAdminApprovalDecisionPayloads(items)})
}

func (h *SaaSAdminHandler) ApprovalDelegations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminApprovalGovernanceStore(w)
	if !ok {
		return
	}
	options := SaaSAdminApprovalDelegationOptions{
		DelegatorUserID: saasAdminQueryInt(r, "delegatorUserId", 0), DelegateUserID: saasAdminQueryInt(r, "delegateUserId", 0),
		ActiveOnly: strings.TrimSpace(r.URL.Query().Get("activeOnly")) == "1", Limit: positiveQueryInt(r, "limit", 100),
	}
	if options.Limit > 500 {
		options.Limit = 500
	}
	items, err := store.SaaSAdminApprovalDelegations(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"delegations": saasAdminApprovalDelegationPayloads(items)})
}

func (h *SaaSAdminHandler) ApprovalDelegation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminApprovalGovernanceStore(w)
	if !ok {
		return
	}
	var body saasAdminApprovalDelegationBody
	if err := decodeSaaSAdminAccessJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	startsAt, startsOK := parseSaaSAdminApprovalTime(body.StartsAt)
	endsAt, endsOK := parseSaaSAdminApprovalTime(body.EndsAt)
	if body.DelegatorUserID <= 0 || body.DelegateUserID <= 0 || body.DelegatorUserID == body.DelegateUserID || !startsOK || !endsOK || !endsAt.After(startsAt) ||
		(body.Status != 1 && body.Status != 2) || body.Reason == "" || len([]rune(body.Reason)) > 255 || (body.ID > 0 && body.ExpectedVersion <= 0) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "委托人、受托人、起止时间、状态、原因或版本无效", nil)
		return
	}
	if endsAt.Sub(startsAt) > 90*24*time.Hour {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "单次审批委托不能超过 90 天", nil)
		return
	}
	result, err := store.UpsertSaaSAdminApprovalDelegation(r.Context(), SaaSAdminApprovalDelegationUpsert{
		ID: body.ID, DelegatorUserID: body.DelegatorUserID, DelegateUserID: body.DelegateUserID, StartsAt: startsAt, EndsAt: endsAt,
		Status: body.Status, Reason: body.Reason, ExpectedVersion: body.ExpectedVersion, ActorUserID: user.ID,
		ActorTenantID: user.TenantID, PlatformTenantID: h.platformAdminTenantID,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"delegation": saasAdminApprovalDelegationPayload(result.Delegation), "operationId": result.OperationID, "created": result.Created,
	})
}

func (h *SaaSAdminHandler) ApprovalReminders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminApprovalGovernanceStore(w)
	if !ok {
		return
	}
	body := saasAdminApprovalReminderBody{Limit: 100, MaxAttempts: 3}
	if err := decodeSaaSAdminAccessJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if body.Limit <= 0 || body.Limit > 500 || body.MaxAttempts <= 0 || body.MaxAttempts > 20 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "limit 必须在 1 至 500，maxAttempts 必须在 1 至 20", nil)
		return
	}
	result, err := store.CreateSaaSAdminApprovalReminders(r.Context(), SaaSAdminApprovalReminderCreate{
		Limit: body.Limit, MaxAttempts: body.MaxAttempts, ActorUserID: user.ID, ActorTenantID: user.TenantID, PlatformTenantID: h.platformAdminTenantID,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"scanned": result.Scanned, "enqueued": result.Enqueued, "skipped": result.Skipped, "notificationKeys": result.NotificationKeys,
	})
}

func (h *SaaSAdminHandler) activeSaaSAdminApprovalDelegation(ctx context.Context, delegateUserID int) (SaaSAdminApprovalDelegation, bool, error) {
	store, ok := h.store.(SaaSAdminApprovalGovernanceStore)
	if !ok || store == nil {
		return SaaSAdminApprovalDelegation{}, false, nil
	}
	return store.ResolveSaaSAdminApprovalDelegation(ctx, delegateUserID, h.platformAdminTenantID, time.Now())
}

func (h *SaaSAdminHandler) approvalDelegatedFromUserID(ctx context.Context, user User) (int, error) {
	if user.IsSuperAdmin == 1 {
		return 0, nil
	}
	profile, err := h.saasAdminAccessProfile(ctx, user)
	if err != nil {
		return 0, err
	}
	if SaaSAdminAccessHasPermission(profile, SaaSAdminPermissionApprovalsReview) {
		return 0, nil
	}
	delegation, found, err := h.activeSaaSAdminApprovalDelegation(ctx, user.ID)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, &SaaSAdminOperationError{Status: http.StatusForbidden, Message: "缺少平台权限 " + SaaSAdminPermissionApprovalsReview + " 且没有有效审批委托"}
	}
	return delegation.DelegatorUserID, nil
}

func (h *SaaSAdminHandler) saasAdminApprovalGovernanceStore(w http.ResponseWriter) (SaaSAdminApprovalGovernanceStore, bool) {
	store, ok := h.store.(SaaSAdminApprovalGovernanceStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "SaaS admin approval governance store is not configured", nil)
		return nil, false
	}
	return store, true
}

func parseSaaSAdminApprovalTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04"} {
		value, err := time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			return value, true
		}
	}
	return time.Time{}, false
}

func saasAdminApprovalPolicyPayload(item SaaSAdminApprovalPolicy) map[string]any {
	return map[string]any{
		"actionType": item.ActionType, "name": item.Name, "riskLevel": item.RiskLevel, "requiredPermission": item.RequiredPermission,
		"targetType": item.TargetType, "description": item.Description, "enabled": item.Enabled,
		"amountThresholdCents": item.AmountThresholdCents, "requiredApprovals": item.RequiredApprovals,
		"slaMinutes": item.SLAMinutes, "reminderMinutes": item.ReminderMinutes, "expiryHours": item.ExpiryHours,
		"governanceLocked":      saasAdminApprovalPolicyGovernanceLocked(item),
		"minimumApprovals":      saasAdminApprovalPolicyMinimumApprovals(item),
		"amountThresholdLocked": saasAdminApprovalPolicyAmountThresholdLocked(item),
		"version":               item.Version, "updatedBy": item.UpdatedBy, "updatedAt": item.UpdatedAt,
	}
}

func saasAdminApprovalDecisionPayloads(items []SaaSAdminApprovalDecisionRecord) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id": item.ID, "approvalId": item.ApprovalID, "reviewerUserId": item.ReviewerUserID, "reviewerTenantId": item.ReviewerTenantID,
			"reviewerName": item.ReviewerName, "delegatedFromUserId": item.DelegatedFromUserID, "delegatedFromName": item.DelegatedFromName,
			"decision": item.Decision, "reason": item.Reason, "createdAt": item.CreatedAt,
		})
	}
	return result
}

func saasAdminApprovalDelegationPayload(item SaaSAdminApprovalDelegation) map[string]any {
	return map[string]any{
		"id": item.ID, "delegatorUserId": item.DelegatorUserID, "delegatorName": item.DelegatorName,
		"delegateUserId": item.DelegateUserID, "delegateName": item.DelegateName, "startsAt": item.StartsAt, "endsAt": item.EndsAt,
		"status": item.Status, "reason": item.Reason, "version": item.Version, "createdBy": item.CreatedBy, "updatedBy": item.UpdatedBy,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasAdminApprovalDelegationPayloads(items []SaaSAdminApprovalDelegation) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasAdminApprovalDelegationPayload(item))
	}
	return result
}
