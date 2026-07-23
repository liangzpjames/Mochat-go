package dashboard

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const (
	SaaSAdminOperationActionServiceAccountUsageAlertEvaluate = "saas.admin.service_account.usage_alert.evaluate"
	SaaSAdminOperationTargetServiceAccountUsageAlert         = "saas_service_account_usage_alert"
)

type SaaSServiceAccountUsageAlertEvaluateOptions struct {
	TenantID         int    `json:"tenantId"`
	ServiceAccountID int64  `json:"serviceAccountId"`
	Limit            int    `json:"limit"`
	Source           string `json:"-"`
	ActorUserID      int    `json:"-"`
	ActorTenantID    int    `json:"-"`
}

type SaaSServiceAccountUsageAlertEvaluateResult struct {
	ScannedAccounts          int
	EligibleAccounts         int
	DisabledPolicies         int
	UsageWarningAccounts     int
	RejectionWarningAccounts int
	ResolvedAlerts           int
	NotificationsQueued      int
	NotificationsClosed      int
	OperationID              int64
	EvaluatedAt              string
}

type SaaSServiceAccountUsageAlertEvaluator interface {
	EvaluateSaaSServiceAccountUsageAlerts(ctx context.Context, options SaaSServiceAccountUsageAlertEvaluateOptions) (SaaSServiceAccountUsageAlertEvaluateResult, error)
}

func SaaSServiceAccountUsageAlertPeriodKey(serviceAccountID int64) string {
	return "account:" + strconv.FormatInt(serviceAccountID, 10)
}

func (h *SaaSAdminHandler) ServiceAccountUsageAlertEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	evaluator, ok := h.saasServiceAccountUsageAlertEvaluator(w)
	if !ok {
		return
	}
	var options SaaSServiceAccountUsageAlertEvaluateOptions
	if !decodeSaaSServiceAccountJSON(w, r, &options) {
		return
	}
	if err := normalizeSaaSServiceAccountUsageAlertEvaluateOptions(&options); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	options.Source = "admin_manual"
	options.ActorUserID = user.ID
	options.ActorTenantID = user.TenantID
	result, err := evaluator.EvaluateSaaSServiceAccountUsageAlerts(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", saasServiceAccountUsageAlertEvaluateResultPayload(result))
}

func normalizeSaaSServiceAccountUsageAlertEvaluateOptions(options *SaaSServiceAccountUsageAlertEvaluateOptions) error {
	if options == nil || options.TenantID < 0 || options.ServiceAccountID < 0 {
		return errors.New("tenantId 和 serviceAccountId 不能小于 0")
	}
	if options.Limit == 0 {
		options.Limit = 100
	}
	if options.Limit < 1 || options.Limit > 500 {
		return errors.New("limit 必须在 1 至 500 之间")
	}
	return nil
}

func (h *SaaSAdminHandler) saasServiceAccountUsageAlertEvaluator(w http.ResponseWriter) (SaaSServiceAccountUsageAlertEvaluator, bool) {
	evaluator, ok := h.store.(SaaSServiceAccountUsageAlertEvaluator)
	if !ok || evaluator == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "service account usage alert evaluator is not configured", nil)
		return nil, false
	}
	return evaluator, true
}

func saasServiceAccountUsageAlertEvaluateResultPayload(result SaaSServiceAccountUsageAlertEvaluateResult) map[string]any {
	return map[string]any{
		"scannedAccounts":          result.ScannedAccounts,
		"eligibleAccounts":         result.EligibleAccounts,
		"disabledPolicies":         result.DisabledPolicies,
		"usageWarningAccounts":     result.UsageWarningAccounts,
		"rejectionWarningAccounts": result.RejectionWarningAccounts,
		"resolvedAlerts":           result.ResolvedAlerts,
		"notificationsQueued":      result.NotificationsQueued,
		"notificationsClosed":      result.NotificationsClosed,
		"operationId":              result.OperationID,
		"evaluatedAt":              strings.TrimSpace(result.EvaluatedAt),
	}
}
