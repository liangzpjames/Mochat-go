package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/outboundhttp"
	"jiyi/mochat-go/internal/saasauth"
)

type SaaSAlertDashboardStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	ListSaaSAlerts(ctx context.Context, options SaaSAlertListOptions) (SaaSAlertListPage, error)
	ResolveSaaSAlert(ctx context.Context, tenantID int, metric string, alertType string) (bool, error)
	GetSaaSAlertSetting(ctx context.Context, tenantID int, channel string) (SaaSAlertSetting, bool, error)
	SaveSaaSAlertSetting(ctx context.Context, setting SaaSAlertSetting) (SaaSAlertSetting, error)
}

type SaaSAlertHandler struct {
	store                 SaaSAlertDashboardStore
	resolver              UserIDResolver
	platformAdminTenantID int
	webhookGuard          *outboundhttp.Guard
}

func NewSaaSAlertHandler(store SaaSAlertDashboardStore, resolver UserIDResolver, platformTenantID ...int) *SaaSAlertHandler {
	platformID := 1
	if len(platformTenantID) > 0 && platformTenantID[0] > 0 {
		platformID = platformTenantID[0]
	}
	return &SaaSAlertHandler{store: store, resolver: resolver, platformAdminTenantID: platformID, webhookGuard: defaultSaaSAlertWebhookGuard}
}

func (h *SaaSAlertHandler) WithWebhookGuard(guard *outboundhttp.Guard) *SaaSAlertHandler {
	h.webhookGuard = normalizeSaaSAlertWebhookGuard(guard)
	return h
}

func (h *SaaSAlertHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = SaaSAlertStatusOpen
	} else if strings.EqualFold(status, "all") {
		status = ""
	}
	pageNumber := positiveQueryInt(r, "page", 1)
	perPage := positiveQueryInt(r, "perPage", 20)
	if perPage > 500 {
		perPage = 500
	}
	options := SaaSAlertListOptions{
		TenantID:  user.TenantID,
		Status:    status,
		Metric:    strings.TrimSpace(r.URL.Query().Get("metric")),
		AlertType: strings.TrimSpace(r.URL.Query().Get("alertType")),
		Page:      pageNumber,
		PerPage:   perPage,
	}
	if options.AlertType == "" {
		options.AlertType = SaaSAlertTypeQuotaExceeded
	}
	page, err := h.store.ListSaaSAlerts(r.Context(), options)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	items := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, saasAlertPayload(item))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   perPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": items,
	})
}

func (h *SaaSAlertHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	metric := stringParam(params, "metric")
	if metric == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "metric 必传", nil)
		return
	}
	alertType := stringParam(params, "alertType")
	if alertType == "" {
		alertType = SaaSAlertTypeQuotaExceeded
	}
	resolved, err := h.store.ResolveSaaSAlert(r.Context(), user.TenantID, metric, alertType)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"resolved": resolved,
	})
}

func (h *SaaSAlertHandler) Setting(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.showSetting(w, r)
	case http.MethodPut, http.MethodPost:
		h.saveSetting(w, r)
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (h *SaaSAlertHandler) showSetting(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	channel := strings.TrimSpace(r.URL.Query().Get("channel"))
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	setting, exists, err := h.store.GetSaaSAlertSetting(r.Context(), user.TenantID, channel)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !exists {
		setting = DefaultSaaSAlertSetting(user.TenantID, channel)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"setting":         SaaSAlertSettingPayload(setting, exists),
		"webhookSecurity": saasAlertWebhookSecurityPayload(h.webhookGuard),
		"urlSecurity":     saasAlertWebhookURLSecurityPayload(h.webhookGuard, setting.WebhookURL),
	})
}

func (h *SaaSAlertHandler) saveSetting(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	channel := stringParam(params, "channel")
	if channel == "" {
		channel = strings.TrimSpace(r.URL.Query().Get("channel"))
	}
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	setting, exists, err := h.store.GetSaaSAlertSetting(r.Context(), user.TenantID, channel)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !exists {
		setting = DefaultSaaSAlertSetting(user.TenantID, channel)
	}
	setting.TenantID = user.TenantID
	setting.Channel = channel
	setting, err = applySaaSAlertSettingParams(setting, params)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := validateSaaSAlertWebhookURLSecurity(h.webhookGuard, setting.WebhookURL); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	saved, err := h.store.SaveSaaSAlertSetting(r.Context(), setting)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"setting":         SaaSAlertSettingPayload(saved, true),
		"webhookSecurity": saasAlertWebhookSecurityPayload(h.webhookGuard),
		"urlSecurity":     saasAlertWebhookURLSecurityPayload(h.webhookGuard, saved.WebhookURL),
	})
}

func (h *SaaSAlertHandler) resolveSuperAdmin(w http.ResponseWriter, r *http.Request) (User, bool) {
	if _, principalErr := saasauth.PrincipalFromContext(r.Context()); principalErr == nil {
		user, found, err := resolveSaaSAdminActor(r.Context(), h.store, h.platformAdminTenantID)
		if errors.Is(err, ErrSaaSAdminActorStoreUnavailable) {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, err.Error(), nil)
			return User{}, false
		}
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return User{}, false
		}
		if !found || user.Status != 1 {
			writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
			return User{}, false
		}
		if user.IsSuperAdmin != 1 {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
			return User{}, false
		}
		return user, true
	}
	if h.resolver == nil {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return User{}, false
	}
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return User{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return User{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return User{}, false
	}
	if user.IsSuperAdmin != 1 || user.TenantID <= 0 {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return User{}, false
	}
	return user, true
}

func boolParam(params map[string]any, key string) (bool, bool, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return false, false, nil
	}
	switch typed := value.(type) {
	case bool:
		return typed, true, nil
	case string:
		return parseBoolString(typed)
	case json.Number:
		integer, err := typed.Int64()
		if err != nil {
			return false, true, err
		}
		return integer != 0, true, nil
	case float64:
		return typed != 0, true, nil
	case int:
		return typed != 0, true, nil
	case []string:
		if len(typed) == 0 {
			return false, false, nil
		}
		return parseBoolString(typed[0])
	case []any:
		if len(typed) == 0 {
			return false, false, nil
		}
		return boolParam(map[string]any{key: typed[0]}, key)
	default:
		return parseBoolString(fmt.Sprint(value))
	}
}

func parseBoolString(raw string) (bool, bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false, false, nil
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true, true, nil
	case "0", "false", "no", "off":
		return false, true, nil
	default:
		integer, err := strconv.Atoi(value)
		if err != nil {
			return false, true, err
		}
		return integer != 0, true, nil
	}
}

func saasAlertPayload(alert SaaSAlertRecord) map[string]any {
	return map[string]any{
		"id":              alert.ID,
		"alertKey":        alert.AlertKey,
		"tenantId":        alert.TenantID,
		"alertType":       alert.AlertType,
		"severity":        alert.Severity,
		"status":          alert.Status,
		"metric":          alert.Metric,
		"periodKey":       alert.PeriodKey,
		"currentValue":    alert.CurrentValue,
		"limitValue":      alert.LimitValue,
		"additionalValue": alert.AdditionalValue,
		"occurrenceCount": alert.OccurrenceCount,
		"source":          alert.Source,
		"message":         alert.Message,
		"context":         parseSaaSAlertContext(alert.ContextJSON),
		"firstSeenAt":     alert.FirstSeenAt,
		"lastSeenAt":      alert.LastSeenAt,
		"resolvedAt":      alert.ResolvedAt,
		"createdAt":       alert.CreatedAt,
		"updatedAt":       alert.UpdatedAt,
	}
}

func parseSaaSAlertContext(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	return value
}
