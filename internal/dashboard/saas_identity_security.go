package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/identitysecurity"
)

func (h *SaaSAdminHandler) WithIdentitySecurityManager(manager *identitysecurity.Manager) *SaaSAdminHandler {
	h.identitySecurityManager = manager
	return h
}

func (h *SaaSAdminHandler) IdentityOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	manager, ok := h.saasIdentitySecurityManager(w)
	if !ok {
		return
	}
	tenantID, ok := identityQueryInt(w, r, "tenantId", h.platformAdminTenantID, 1, 0)
	if !ok {
		return
	}
	limit, ok := identityQueryInt(w, r, "limit", 50, 1, 200)
	if !ok {
		return
	}
	overview, err := manager.Overview(r.Context(), tenantID, limit)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", overview)
}

func (h *SaaSAdminHandler) IdentityPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasIdentitySecurityManager(w)
	if !ok {
		return
	}
	var input identitysecurity.PolicyUpdate
	if err := decodeIdentitySecurityJSON(r, &input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input, _, err := manager.PlanPolicyUpdate(r.Context(), input)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	input.Actor = identitysecurity.Actor{UserID: user.ID, TenantID: user.TenantID}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionIdentityPolicyUpdate, 0) {
		return
	}
	policy, err := manager.UpdatePolicy(r.Context(), input)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"policy": policy})
}

func (h *SaaSAdminHandler) IdentitySessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	manager, ok := h.saasIdentitySecurityManager(w)
	if !ok {
		return
	}
	options, valid := identitySessionOptions(w, r)
	if !valid {
		return
	}
	items, err := manager.Sessions(r.Context(), options)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"sessions": items})
}

func (h *SaaSAdminHandler) IdentitySession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasIdentitySecurityManager(w)
	if !ok {
		return
	}
	var input identitysecurity.SessionRevoke
	if err := decodeIdentitySecurityJSON(r, &input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input.Actor = identitysecurity.Actor{UserID: user.ID, TenantID: user.TenantID}
	item, err := manager.RevokeSession(r.Context(), input)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"session": item})
}

func (h *SaaSAdminHandler) IdentityLoginEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	manager, ok := h.saasIdentitySecurityManager(w)
	if !ok {
		return
	}
	options, valid := identityEventOptions(w, r)
	if !valid {
		return
	}
	items, err := manager.LoginEvents(r.Context(), options)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"events": items})
}

func (h *SaaSAdminHandler) IdentityIncidents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	manager, ok := h.saasIdentitySecurityManager(w)
	if !ok {
		return
	}
	options, valid := identityIncidentOptions(w, r)
	if !valid {
		return
	}
	items, err := manager.Incidents(r.Context(), options)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"incidents": items})
}

func (h *SaaSAdminHandler) IdentityIncident(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasIdentitySecurityManager(w)
	if !ok {
		return
	}
	var input identitysecurity.IncidentUpdate
	if err := decodeIdentitySecurityJSON(r, &input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input.Actor = identitysecurity.Actor{UserID: user.ID, TenantID: user.TenantID}
	item, err := manager.UpdateIncident(r.Context(), input)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"incident": item})
}

func (h *SaaSAdminHandler) IdentityUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasIdentitySecurityManager(w)
	if !ok {
		return
	}
	var body struct {
		UserID          int    `json:"userId"`
		ExpectedVersion int    `json:"expectedVersion"`
		Reason          string `json:"reason"`
	}
	if err := decodeIdentitySecurityJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	state, err := manager.UnlockUser(r.Context(), body.UserID, body.ExpectedVersion, body.Reason, identitysecurity.Actor{UserID: user.ID, TenantID: user.TenantID})
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"user": state})
}

func (h *SaaSAdminHandler) IdentityMFA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	manager, ok := h.saasIdentitySecurityManager(w)
	if !ok {
		return
	}
	var input identitysecurity.MFAReset
	if err := decodeIdentitySecurityJSON(r, &input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input, _, _, err := manager.PlanMFAReset(r.Context(), input)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	input.Actor = identitysecurity.Actor{UserID: user.ID, TenantID: user.TenantID}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionIdentityMFAReset, 0) {
		return
	}
	result, err := manager.ResetMFA(r.Context(), input)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"mfa": safeIdentityMFAPayload(result.Credential), "revokedSessions": result.RevokedSessions, "operationId": result.OperationID,
	})
}

func (h *SaaSAdminHandler) saasIdentitySecurityManager(w http.ResponseWriter) (*identitysecurity.Manager, bool) {
	if h.identitySecurityManager == nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "身份安全管理器未配置", nil)
		return nil, false
	}
	return h.identitySecurityManager, true
}

type IdentitySelfHandler struct {
	store interface {
		UserByID(ctx context.Context, userID int) (User, bool, error)
	}
	resolver UserIDResolver
	manager  *identitysecurity.Manager
}

func NewIdentitySelfHandler(store interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
}, resolver UserIDResolver, manager *identitysecurity.Manager) *IdentitySelfHandler {
	return &IdentitySelfHandler{store: store, resolver: resolver, manager: manager}
}

func (h *IdentitySelfHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if h.manager == nil || h.store == nil || h.resolver == nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "身份安全服务未配置", nil)
		return
	}
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeIdentitySecurityError(w, err)
		return
	}
	if !found || user.Status != 1 || user.TenantStatus == 2 || user.TenantPackageExpired || (user.TenantSubscriptionManaged && !user.TenantSubscriptionAccessAllowed) {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "账户无法使用身份安全服务", nil)
		return
	}
	if r.Method == http.MethodGet {
		state, err := h.manager.UserState(r.Context(), user.ID)
		if err != nil {
			writeIdentitySecurityError(w, err)
			return
		}
		policy, err := h.manager.Policy(r.Context(), user.TenantID)
		if err != nil {
			writeIdentitySecurityError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"user": state, "policy": policy, "config": h.manager.ConfigStatus()})
		return
	}
	var body struct {
		Action          string `json:"action"`
		Code            string `json:"code"`
		ExpectedVersion int    `json:"expectedVersion"`
	}
	if err := decodeIdentitySecurityJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	principal := identitysecurity.Principal{UserID: user.ID, TenantID: user.TenantID, Phone: user.Phone, Name: user.Name}
	actor := identitysecurity.Actor{UserID: user.ID, TenantID: user.TenantID}
	switch strings.ToLower(strings.TrimSpace(body.Action)) {
	case "begin":
		enrollment, err := h.manager.BeginMFA(r.Context(), identitysecurity.MFABegin{Principal: principal, Actor: actor})
		if err != nil {
			writeIdentitySecurityError(w, err)
			return
		}
		writeEnvelope(w, http.StatusCreated, http.StatusCreated, "success", map[string]any{"enrollment": enrollment})
	case "verify":
		credential, err := h.manager.VerifyMFA(r.Context(), identitysecurity.MFAVerify{Principal: principal, Code: body.Code, ExpectedVersion: body.ExpectedVersion, Actor: actor})
		if err != nil {
			writeIdentitySecurityError(w, err)
			return
		}
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"mfa": safeIdentityMFAPayload(credential)})
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "action 必须是 begin 或 verify", nil)
	}
}

func safeIdentityMFAPayload(item identitysecurity.MFACredential) map[string]any {
	return map[string]any{
		"userId": item.UserID, "tenantId": item.TenantID, "status": item.Status,
		"recoveryCodesRemaining": item.RecoveryCodesRemaining, "verifiedAt": item.VerifiedAt,
		"lastUsedAt": item.LastUsedAt, "disabledAt": item.DisabledAt, "disabledBy": item.DisabledBy,
		"disabledReason": item.DisabledReason, "version": item.Version, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func identitySessionOptions(w http.ResponseWriter, r *http.Request) (identitysecurity.SessionOptions, bool) {
	tenantID, ok := identityQueryInt(w, r, "tenantId", 0, 0, 0)
	if !ok {
		return identitysecurity.SessionOptions{}, false
	}
	userID, ok := identityQueryInt(w, r, "userId", 0, 0, 0)
	if !ok {
		return identitysecurity.SessionOptions{}, false
	}
	limit, ok := identityQueryInt(w, r, "limit", 50, 1, 200)
	if !ok {
		return identitysecurity.SessionOptions{}, false
	}
	status, ok := identityQueryEnum(w, r, "status", "", "", identitysecurity.SessionStatusActive, identitysecurity.SessionStatusRevoked, identitysecurity.SessionStatusExpired)
	if !ok {
		return identitysecurity.SessionOptions{}, false
	}
	keyword, ok := identityQueryText(w, r, "keyword", 100)
	return identitysecurity.SessionOptions{TenantID: tenantID, UserID: userID, Status: status, Keyword: keyword, Limit: limit}, ok
}

func identityEventOptions(w http.ResponseWriter, r *http.Request) (identitysecurity.EventOptions, bool) {
	tenantID, ok := identityQueryInt(w, r, "tenantId", 0, 0, 0)
	if !ok {
		return identitysecurity.EventOptions{}, false
	}
	userID, ok := identityQueryInt(w, r, "userId", 0, 0, 0)
	if !ok {
		return identitysecurity.EventOptions{}, false
	}
	limit, ok := identityQueryInt(w, r, "limit", 50, 1, 200)
	if !ok {
		return identitysecurity.EventOptions{}, false
	}
	risk, ok := identityQueryEnum(w, r, "risk", "", "", identitysecurity.RiskNormal, identitysecurity.RiskWarning, identitysecurity.RiskCritical)
	if !ok {
		return identitysecurity.EventOptions{}, false
	}
	result, ok := identityQueryEnum(w, r, "result", "", "", "succeeded", "failed", "blocked", "verified", "required", "revoked")
	if !ok {
		return identitysecurity.EventOptions{}, false
	}
	keyword, ok := identityQueryText(w, r, "keyword", 100)
	return identitysecurity.EventOptions{TenantID: tenantID, UserID: userID, Risk: risk, Result: result, Keyword: keyword, Limit: limit}, ok
}

func identityIncidentOptions(w http.ResponseWriter, r *http.Request) (identitysecurity.IncidentOptions, bool) {
	tenantID, ok := identityQueryInt(w, r, "tenantId", 0, 0, 0)
	if !ok {
		return identitysecurity.IncidentOptions{}, false
	}
	userID, ok := identityQueryInt(w, r, "userId", 0, 0, 0)
	if !ok {
		return identitysecurity.IncidentOptions{}, false
	}
	limit, ok := identityQueryInt(w, r, "limit", 50, 1, 200)
	if !ok {
		return identitysecurity.IncidentOptions{}, false
	}
	status, ok := identityQueryEnum(w, r, "status", "", "", identitysecurity.IncidentStatusOpen, identitysecurity.IncidentStatusAcknowledged, identitysecurity.IncidentStatusResolved)
	if !ok {
		return identitysecurity.IncidentOptions{}, false
	}
	severity, ok := identityQueryEnum(w, r, "severity", "", "", identitysecurity.RiskWarning, identitysecurity.RiskCritical)
	if !ok {
		return identitysecurity.IncidentOptions{}, false
	}
	keyword, ok := identityQueryText(w, r, "keyword", 100)
	return identitysecurity.IncidentOptions{TenantID: tenantID, UserID: userID, Status: status, Severity: severity, Keyword: keyword, Limit: limit}, ok
}

func identityQueryInt(w http.ResponseWriter, r *http.Request, key string, fallback, minimum, maximum int) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || (maximum > 0 && value > maximum) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, key+" 无效", nil)
		return 0, false
	}
	return value, true
}

func identityQueryText(w http.ResponseWriter, r *http.Request, key string, maxRunes int) (string, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if len([]rune(value)) > maxRunes {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, key+" 过长", nil)
		return "", false
	}
	return value, true
}

func identityQueryEnum(w http.ResponseWriter, r *http.Request, key, fallback string, allowed ...string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key)))
	if value == "" {
		return fallback, true
	}
	for _, item := range allowed {
		if value == item {
			return value, true
		}
	}
	writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, key+" 无效", nil)
	return "", false
}

func decodeIdentitySecurityJSON(r *http.Request, target any) error {
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

func writeIdentitySecurityError(w http.ResponseWriter, err error) {
	status := identitysecurity.StatusCode(err)
	message := err.Error()
	if status >= http.StatusInternalServerError {
		message = "身份安全服务暂不可用"
	}
	writeEnvelope(w, status, status, message, nil)
}

func identitySecurityAdminError(err error) error {
	if err == nil {
		return nil
	}
	var adminErr *SaaSAdminOperationError
	if errors.As(err, &adminErr) {
		return err
	}
	var identityErr *identitysecurity.Error
	if !errors.As(err, &identityErr) || identityErr == nil {
		return err
	}
	return &SaaSAdminOperationError{Status: identityErr.Status, Message: identityErr.Message}
}
