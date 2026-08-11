package dashboard

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/clientip"
	"jiyi/mochat-go/internal/serviceaccountkey"
)

const (
	SaaSServiceAccountStatusAll      = "all"
	SaaSServiceAccountStatusActive   = "active"
	SaaSServiceAccountStatusDisabled = "disabled"

	SaaSServiceAccountKeyStatusAll      = "all"
	SaaSServiceAccountKeyStatusActive   = "active"
	SaaSServiceAccountKeyStatusRetiring = "retiring"
	SaaSServiceAccountKeyStatusRevoked  = "revoked"

	SaaSServiceAccountScopeProfileRead = "tenant.profile.read"
	SaaSServiceAccountScopeUsageRead   = "tenant.usage.read"
	SaaSServiceAccountScopeAlertsRead  = "tenant.alerts.read"

	SaaSAdminOperationActionServiceAccountCreate = "saas.admin.service_account.create"
	SaaSAdminOperationActionServiceAccountUpdate = "saas.admin.service_account.update"
	SaaSAdminOperationActionServiceAccountRotate = "saas.admin.service_account.rotate"
	SaaSAdminOperationActionServiceAccountRevoke = "saas.admin.service_account.revoke"
	SaaSAdminOperationTargetServiceAccount       = "saas_service_account"
	SaaSAdminOperationTargetServiceAccountKey    = "saas_service_account_key"

	DefaultSaaSServiceAccountRateLimitPerMinute        = 60
	DefaultSaaSServiceAccountDailyRequestLimit         = 10000
	DefaultSaaSServiceAccountUsageHistoryDays          = 7
	DefaultSaaSServiceAccountUsageRetentionDays        = 90
	DefaultSaaSServiceAccountUsageWarningPercent       = 80
	DefaultSaaSServiceAccountRejectionWarningCount     = 1
	DefaultSaaSServiceAccountUsageAlertCooldownMinutes = 60

	saasServiceAccountDefaultKeyTTL                = 90 * 24 * time.Hour
	saasServiceAccountMaxRateLimitPerMinute        = 60000
	saasServiceAccountMaxDailyRequestLimit         = 100000000
	saasServiceAccountMaxRejectionWarningCount     = 100000000
	saasServiceAccountMaxUsageAlertCooldownMinutes = 10080
	saasServiceAccountUsageRouteKeyMaxLength       = 128
)

var (
	saasServiceAccountCodePattern      = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,63}$`)
	saasServiceAccountKeyPrefixPattern = regexp.MustCompile(`^[a-f0-9]{12}$`)
)

type SaaSServiceAccountScopeDefinition struct {
	Code        string
	Name        string
	Description string
}

type SaaSServiceAccount struct {
	ID                           int64
	TenantID                     int
	TenantName                   string
	TenantStatus                 int
	Code                         string
	Name                         string
	Description                  string
	Status                       string
	Scopes                       []string
	AllowedCIDRs                 []string
	RateLimitPerMinute           int
	DailyRequestLimit            int64
	UsageAlertEnabled            bool
	UsageWarningPercent          int
	RejectionWarningCount        int
	UsageAlertCooldownMinutes    int
	UsageAlertLastEvaluatedAt    string
	UsageAlertLastNotifiedAt     string
	RejectionAlertLastNotifiedAt string
	MinuteRequestCount           int64
	MinuteRejectedCount          int64
	DailyRequestCount            int64
	DailyRejectedCount           int64
	ExpiresAt                    string
	LastUsedAt                   string
	LastUsedIP                   string
	UseCount                     int64
	Version                      int
	CreatedBy                    int
	UpdatedBy                    int
	CreatedAt                    string
	UpdatedAt                    string
	Keys                         []SaaSServiceAccountKey
	TodayRoutes                  []SaaSServiceAccountRouteUsage
}

type SaaSServiceAccountRouteUsage struct {
	RouteKey      string
	RequestCount  int64
	RejectedCount int64
	LastUsedAt    string
	LastUsedIP    string
}

type SaaSServiceAccountUsageOptions struct {
	TenantID         int
	ServiceAccountID int64
	Days             int
	Limit            int
	DateFrom         string
	DateTo           string
}

type SaaSServiceAccountUsageDaily struct {
	UsageDate          string
	RequestCount       int64
	RejectedCount      int64
	ActiveAccountCount int
}

type SaaSServiceAccountUsageRoute struct {
	RouteKey           string
	RequestCount       int64
	RejectedCount      int64
	ActiveAccountCount int
	LastUsedAt         string
}

type SaaSServiceAccountUsageAccount struct {
	ServiceAccountID   int64
	ServiceAccountCode string
	ServiceAccountName string
	TenantID           int
	TenantName         string
	RequestCount       int64
	RejectedCount      int64
	RouteCount         int
	LastUsedAt         string
}

type SaaSServiceAccountUsageReport struct {
	DateFrom            string
	DateTo              string
	RetentionDays       int
	OldestUsageDate     string
	StoredRowCount      int64
	RequestCount        int64
	RejectedCount       int64
	ActiveAccountCount  int
	LimitedAccountCount int
	OpenAlertCount      int
	Daily               []SaaSServiceAccountUsageDaily
	Routes              []SaaSServiceAccountUsageRoute
	Accounts            []SaaSServiceAccountUsageAccount
}

type SaaSServiceAccountUsageCleanupResult struct {
	RetentionDays int
	CutoffDate    string
	EligibleRows  int64
	ProtectedRows int64
	DeletedRows   int64
	RemainingRows int64
}

type SaaSServiceAccountRateLimitState struct {
	RateLimitPerMinute  int
	MinuteRequestCount  int64
	MinuteRejectedCount int64
	MinuteResetAt       time.Time
	DailyRequestLimit   int64
	DailyRequestCount   int64
	DailyRejectedCount  int64
	DailyResetAt        time.Time
	LimitedBy           string
}

type SaaSServiceAccountKey struct {
	ID               int64
	ServiceAccountID int64
	Name             string
	Prefix           string
	HashKeyID        string
	LastFour         string
	Status           string
	ExpiresAt        string
	RetireAt         string
	LastUsedAt       string
	LastUsedIP       string
	UseCount         int64
	ReplacedByKeyID  int64
	RevokedAt        string
	RevokedBy        int
	Version          int
	CreatedBy        int
	CreatedAt        string
	UpdatedAt        string
}

type SaaSServiceAccountOptions struct {
	TenantID int
	Status   string
	Keyword  string
	Limit    int
}

type SaaSServiceAccountKeyMaterial struct {
	Prefix    string
	Hash      string
	HashKeyID string
	LastFour  string
	PlainText string
}

type SaaSServiceAccountCreate struct {
	TenantID                  int                           `json:"tenantId"`
	Code                      string                        `json:"code"`
	Name                      string                        `json:"name"`
	Description               string                        `json:"description"`
	Status                    string                        `json:"status"`
	Scopes                    []string                      `json:"scopes"`
	AllowedCIDRs              []string                      `json:"allowedCidrs"`
	RateLimitPerMinute        *int                          `json:"rateLimitPerMinute"`
	DailyRequestLimit         *int                          `json:"dailyRequestLimit"`
	UsageAlertEnabled         *bool                         `json:"usageAlertEnabled"`
	UsageWarningPercent       *int                          `json:"usageWarningPercent"`
	RejectionWarningCount     *int                          `json:"rejectionWarningCount"`
	UsageAlertCooldownMinutes *int                          `json:"usageAlertCooldownMinutes"`
	ExpiresAt                 string                        `json:"expiresAt"`
	KeyName                   string                        `json:"keyName"`
	KeyExpiresAt              string                        `json:"keyExpiresAt"`
	Key                       SaaSServiceAccountKeyMaterial `json:"-"`
	ActorUserID               int                           `json:"-"`
	ActorTenantID             int                           `json:"-"`
	ApprovalExecutionID       int64                         `json:"-"`
	ApprovalExecutionVersion  int                           `json:"-"`
}

type SaaSServiceAccountCreateResult struct {
	Account      SaaSServiceAccount
	Key          SaaSServiceAccountKey
	PlainTextKey string
	OperationID  int64
}

type SaaSServiceAccountCreateTarget struct {
	TenantID     int
	TenantName   string
	TenantStatus int
	Code         string
}

type SaaSServiceAccountUpdate struct {
	ID                        int64    `json:"id"`
	Name                      string   `json:"name"`
	Description               string   `json:"description"`
	Status                    string   `json:"status"`
	Scopes                    []string `json:"scopes"`
	AllowedCIDRs              []string `json:"allowedCidrs"`
	RateLimitPerMinute        *int     `json:"rateLimitPerMinute"`
	DailyRequestLimit         *int     `json:"dailyRequestLimit"`
	UsageAlertEnabled         *bool    `json:"usageAlertEnabled"`
	UsageWarningPercent       *int     `json:"usageWarningPercent"`
	RejectionWarningCount     *int     `json:"rejectionWarningCount"`
	UsageAlertCooldownMinutes *int     `json:"usageAlertCooldownMinutes"`
	ExpiresAt                 string   `json:"expiresAt"`
	ExpectedVersion           int      `json:"expectedVersion"`
	ActorUserID               int      `json:"-"`
	ActorTenantID             int      `json:"-"`
	ApprovalExecutionID       int64    `json:"-"`
	ApprovalExecutionVersion  int      `json:"-"`
}

type SaaSServiceAccountUpdateResult struct {
	Account     SaaSServiceAccount
	OperationID int64
}

type SaaSServiceAccountKeyRotate struct {
	ServiceAccountID         int64                         `json:"serviceAccountId"`
	ExpectedVersion          int                           `json:"expectedVersion"`
	Name                     string                        `json:"name"`
	ExpiresAt                string                        `json:"expiresAt"`
	GraceMinutes             int                           `json:"graceMinutes"`
	Key                      SaaSServiceAccountKeyMaterial `json:"-"`
	ActorUserID              int                           `json:"-"`
	ActorTenantID            int                           `json:"-"`
	ApprovalExecutionID      int64                         `json:"-"`
	ApprovalExecutionVersion int                           `json:"-"`
}

type SaaSServiceAccountKeyRotateResult struct {
	Account      SaaSServiceAccount
	Key          SaaSServiceAccountKey
	PlainTextKey string
	RetiringKeys int
	OperationID  int64
}

type SaaSServiceAccountKeyRevoke struct {
	ServiceAccountID         int64 `json:"serviceAccountId"`
	KeyID                    int64 `json:"keyId"`
	ExpectedVersion          int   `json:"expectedVersion"`
	ActorUserID              int   `json:"-"`
	ActorTenantID            int   `json:"-"`
	ApprovalExecutionID      int64 `json:"-"`
	ApprovalExecutionVersion int   `json:"-"`
}

type SaaSServiceAccountKeyRevokeResult struct {
	Account     SaaSServiceAccount
	Key         SaaSServiceAccountKey
	OperationID int64
}

type SaaSServiceAccountPrincipal struct {
	ServiceAccountID     int64
	ServiceAccountCode   string
	ServiceAccountName   string
	ServiceAccountStatus string
	TenantID             int
	TenantName           string
	TenantStatus         int
	Scopes               []string
	AllowedCIDRs         []string
	AccountExpiresAt     string
	KeyID                int64
	KeyName              string
	KeyPrefix            string
	KeyHash              string
	KeyHashKeyID         string
	KeyStatus            string
	KeyExpiresAt         string
	KeyRetireAt          string
	RateLimitPerMinute   int
	DailyRequestLimit    int64
	RateLimit            SaaSServiceAccountRateLimitState
}

type SaaSServiceAccountStore interface {
	SaaSServiceAccounts(ctx context.Context, options SaaSServiceAccountOptions) ([]SaaSServiceAccount, error)
	SaaSServiceAccountUsage(ctx context.Context, options SaaSServiceAccountUsageOptions) (SaaSServiceAccountUsageReport, error)
	CreateSaaSServiceAccount(ctx context.Context, input SaaSServiceAccountCreate) (SaaSServiceAccountCreateResult, error)
	UpdateSaaSServiceAccount(ctx context.Context, input SaaSServiceAccountUpdate) (SaaSServiceAccountUpdateResult, error)
	RotateSaaSServiceAccountKey(ctx context.Context, input SaaSServiceAccountKeyRotate) (SaaSServiceAccountKeyRotateResult, error)
	RevokeSaaSServiceAccountKey(ctx context.Context, input SaaSServiceAccountKeyRevoke) (SaaSServiceAccountKeyRevokeResult, error)
	SaaSServiceAccountPrincipalByPrefix(ctx context.Context, prefix string) (SaaSServiceAccountPrincipal, bool, error)
	ConsumeSaaSServiceAccountRequest(ctx context.Context, serviceAccountID int64, keyID int64, clientIP string, routeKey string) (SaaSServiceAccountRateLimitState, error)
}

type SaaSServiceAccountApprovalStore interface {
	SaaSServiceAccountKeyForApproval(ctx context.Context, serviceAccountID int64, keyID int64) (SaaSServiceAccount, SaaSServiceAccountKey, error)
}

type SaaSServiceAccountUpdateApprovalStore interface {
	SaaSServiceAccountForApproval(ctx context.Context, serviceAccountID int64) (SaaSServiceAccount, error)
}

type SaaSServiceAccountCreateApprovalStore interface {
	SaaSServiceAccountCreateTarget(ctx context.Context, tenantID int, code string) (SaaSServiceAccountCreateTarget, error)
}

type SaaSServiceAccountUsageCleaner interface {
	CleanupSaaSServiceAccountUsage(ctx context.Context, limit int) (SaaSServiceAccountUsageCleanupResult, error)
}

type SaaSServiceAccountKeyProtectionCount struct {
	HashKeyID      string
	KeyCount       int
	UsableKeyCount int
}

type SaaSServiceAccountKeyProtectionStore interface {
	SaaSServiceAccountKeyProtectionCounts(ctx context.Context) ([]SaaSServiceAccountKeyProtectionCount, error)
}

type SaaSServiceAccountKeyProtectionStatus struct {
	ActiveKeyID          string
	ConfiguredKeyIDs     []string
	KeyCount             int
	DedicatedConfigured  bool
	LegacyJWTEnabled     bool
	RequireDedicated     bool
	StoredKeyCount       int
	UsableKeyCount       int
	LegacyStoredKeyCount int
	LegacyUsableKeyCount int
	ActiveStoredKeyCount int
	ActiveUsableKeyCount int
	MissingKeyIDs        []string
}

type SaaSServiceAccountAuthError struct {
	Status  int
	Message string
}

func (e *SaaSServiceAccountAuthError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

type SaaSServiceAccountRateLimitError struct {
	State SaaSServiceAccountRateLimitState
}

func (e *SaaSServiceAccountRateLimitError) Error() string {
	return "API rate limit exceeded"
}

func SaaSServiceAccountScopeCatalog() []SaaSServiceAccountScopeDefinition {
	return []SaaSServiceAccountScopeDefinition{
		{Code: SaaSServiceAccountScopeProfileRead, Name: "租户身份", Description: "读取服务账号与绑定租户身份"},
		{Code: SaaSServiceAccountScopeUsageRead, Name: "租户用量", Description: "读取当前用量、额度和使用率"},
		{Code: SaaSServiceAccountScopeAlertsRead, Name: "租户告警", Description: "读取租户额度告警"},
	}
}

func SaaSServiceAccountScopeValid(scope string) bool {
	for _, item := range SaaSServiceAccountScopeCatalog() {
		if item.Code == strings.TrimSpace(scope) {
			return true
		}
	}
	return false
}

func (h *SaaSAdminHandler) WithServiceAccountKeyPepper(pepper string) *SaaSAdminHandler {
	h.serviceAccountKeyManager = serviceaccountkey.NewLegacyManager(strings.TrimSpace(pepper))
	return h
}

func (h *SaaSAdminHandler) WithServiceAccountKeyManager(manager *serviceaccountkey.Manager) *SaaSAdminHandler {
	h.serviceAccountKeyManager = manager
	return h
}

func (h *SaaSAdminHandler) WithServiceAccountClientIPResolver(resolver *clientip.Resolver) *SaaSAdminHandler {
	if resolver == nil {
		resolver = clientip.DirectResolver()
	}
	h.serviceAccountClientIPResolver = resolver
	return h
}

func (h *SaaSAdminHandler) ServiceAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasServiceAccountStore(w)
	if !ok {
		return
	}
	options, err := saasServiceAccountOptionsFromQuery(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	items, err := store.SaaSServiceAccounts(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	protection, err := h.saasServiceAccountKeyProtectionStatus(r.Context(), items)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{
		"items":              saasServiceAccountPayloads(items),
		"summary":            saasServiceAccountSummaryPayload(items),
		"scopes":             saasServiceAccountScopeCatalogPayload(),
		"keyProtection":      saasServiceAccountKeyProtectionPayload(protection),
		"clientIPResolution": h.serviceAccountClientIPResolver.Status(),
		"returnedCount":      len(items),
	})
}

func (h *SaaSAdminHandler) ServiceAccountUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasServiceAccountStore(w)
	if !ok {
		return
	}
	options, err := saasServiceAccountUsageOptionsFromQuery(r, time.Now())
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	report, err := store.SaaSServiceAccountUsage(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	report.Daily = fillSaaSServiceAccountUsageDays(report.Daily, options.DateFrom, options.Days)
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", saasServiceAccountUsageReportPayload(report))
}

func (h *SaaSAdminHandler) ServiceAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasServiceAccountStore(w)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		var input SaaSServiceAccountCreate
		if !decodeSaaSServiceAccountJSON(w, r, &input) {
			return
		}
		if err := normalizeSaaSServiceAccountCreate(&input, time.Now()); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionServiceAccountCreate, 0) {
			return
		}
		key, err := h.newSaaSServiceAccountKey()
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		plainTextKey := key.PlainText
		key.PlainText = ""
		input.Key = key
		input.ActorUserID = user.ID
		input.ActorTenantID = user.TenantID
		result, err := store.CreateSaaSServiceAccount(r.Context(), input)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		result.PlainTextKey = plainTextKey
		writeOneTimeServiceAccountKey(w, http.StatusCreated, map[string]any{
			"account":      saasServiceAccountPayload(result.Account),
			"key":          saasServiceAccountKeyPayload(result.Key),
			"plainTextKey": result.PlainTextKey,
			"operationId":  result.OperationID,
		})
		return
	}
	var input SaaSServiceAccountUpdate
	if !decodeSaaSServiceAccountJSON(w, r, &input) {
		return
	}
	if err := normalizeSaaSServiceAccountUpdate(&input, time.Now()); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionServiceAccountUpdate, 0) {
		return
	}
	input.ActorUserID = user.ID
	input.ActorTenantID = user.TenantID
	result, err := store.UpdateSaaSServiceAccount(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{"account": saasServiceAccountPayload(result.Account), "operationId": result.OperationID})
}

func (h *SaaSAdminHandler) ServiceAccountKeyRotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasServiceAccountStore(w)
	if !ok {
		return
	}
	var input SaaSServiceAccountKeyRotate
	if !decodeSaaSServiceAccountJSON(w, r, &input) {
		return
	}
	if err := normalizeSaaSServiceAccountKeyRotate(&input, time.Now()); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionServiceAccountKeyRotate, 0) {
		return
	}
	key, err := h.newSaaSServiceAccountKey()
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	plainTextKey := key.PlainText
	key.PlainText = ""
	input.Key = key
	input.ActorUserID = user.ID
	input.ActorTenantID = user.TenantID
	result, err := store.RotateSaaSServiceAccountKey(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result.PlainTextKey = plainTextKey
	writeOneTimeServiceAccountKey(w, http.StatusOK, map[string]any{
		"account":      saasServiceAccountPayload(result.Account),
		"key":          saasServiceAccountKeyPayload(result.Key),
		"plainTextKey": result.PlainTextKey,
		"retiringKeys": result.RetiringKeys,
		"operationId":  result.OperationID,
	})
}

func (h *SaaSAdminHandler) ServiceAccountKeyRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasServiceAccountStore(w)
	if !ok {
		return
	}
	var input SaaSServiceAccountKeyRevoke
	if !decodeSaaSServiceAccountJSON(w, r, &input) {
		return
	}
	if input.ServiceAccountID <= 0 || input.KeyID <= 0 || input.ExpectedVersion <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "serviceAccountId、keyId 和 expectedVersion 必须大于 0", nil)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionServiceAccountKeyRevoke, 0) {
		return
	}
	input.ActorUserID = user.ID
	input.ActorTenantID = user.TenantID
	result, err := store.RevokeSaaSServiceAccountKey(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{
		"account": saasServiceAccountPayload(result.Account), "key": saasServiceAccountKeyPayload(result.Key), "operationId": result.OperationID,
	})
}

func (h *SaaSAdminHandler) SaaSServiceAccountWhoAmI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	principal, ok := h.resolveSaaSServiceAccount(w, r, SaaSServiceAccountScopeProfileRead)
	if !ok {
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", saasServiceAccountPrincipalPayload(principal))
}

func (h *SaaSAdminHandler) SaaSServiceAccountUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	principal, ok := h.resolveSaaSServiceAccount(w, r, SaaSServiceAccountScopeUsageRead)
	if !ok {
		return
	}
	metrics, err := h.store.SaaSAdminTenantUsage(r.Context(), principal.TenantID)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	overview, err := h.saasAdminOverview(r.Context(), SaaSAdminOverviewOptions{Scope: SaaSAdminScopeTenant, TenantID: principal.TenantID, Limit: 1, ExpiringDays: 30})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	tenant := SaaSAdminTenantOverview{TenantID: principal.TenantID, TenantName: principal.TenantName, TenantStatus: principal.TenantStatus}
	if len(overview.Tenants) > 0 {
		tenant = overview.Tenants[0]
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{
		"tenant": saasAdminTenantPayload(tenant), "summary": saasAdminUsageSummaryPayload(saasAdminUsageSummary(metrics)), "metrics": saasAdminUsageMetricPayloads(metrics),
	})
}

func (h *SaaSAdminHandler) SaaSServiceAccountAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	principal, ok := h.resolveSaaSServiceAccount(w, r, SaaSServiceAccountScopeAlertsRead)
	if !ok {
		return
	}
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "all" {
		status = ""
	}
	if status != "" && status != SaaSAlertStatusOpen && status != SaaSAlertStatusResolved {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "status 必须是 open、resolved 或 all", nil)
		return
	}
	page := positiveQueryInt(r, "page", 1)
	perPage := positiveQueryInt(r, "perPage", 50)
	if perPage > 100 {
		perPage = 100
	}
	items, err := h.store.ListSaaSAlerts(r.Context(), SaaSAlertListOptions{
		TenantID: principal.TenantID, Status: status, Metric: strings.TrimSpace(r.URL.Query().Get("metric")), Page: page, PerPage: perPage,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{
		"items": saasAdminAlertPayloads(items.Items), "page": page, "perPage": perPage, "total": items.Total, "totalPage": items.TotalPage,
	})
}

func (h *SaaSAdminHandler) resolveSaaSServiceAccount(w http.ResponseWriter, r *http.Request, requiredScope string) (SaaSServiceAccountPrincipal, bool) {
	store, ok := h.store.(SaaSServiceAccountStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "service account store is not configured", nil)
		return SaaSServiceAccountPrincipal{}, false
	}
	token, prefix, ok := presentedSaaSServiceAccountKey(r)
	if !ok || h.serviceAccountKeyManager == nil {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "invalid API key", nil)
		return SaaSServiceAccountPrincipal{}, false
	}
	principal, found, err := store.SaaSServiceAccountPrincipalByPrefix(r.Context(), prefix)
	if err != nil {
		writeSaaSAdminError(w, err)
		return SaaSServiceAccountPrincipal{}, false
	}
	presentedHash, hashKeyFound := h.serviceAccountKeyManager.Hash(principal.KeyHashKeyID, token)
	if !found || !hashKeyFound || !constantTimeHexEqual(principal.KeyHash, presentedHash) || !saasServiceAccountPrincipalEnabled(principal, time.Now()) {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "invalid API key", nil)
		return SaaSServiceAccountPrincipal{}, false
	}
	clientIP := h.serviceAccountClientIPResolver.Resolve(r)
	if !saasServiceAccountIPAllowed(clientIP, principal.AllowedCIDRs) {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "client IP is not allowed", nil)
		return SaaSServiceAccountPrincipal{}, false
	}
	if !stringSliceContains(principal.Scopes, requiredScope) {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "missing API scope "+requiredScope, nil)
		return SaaSServiceAccountPrincipal{}, false
	}
	rateState, err := store.ConsumeSaaSServiceAccountRequest(r.Context(), principal.ServiceAccountID, principal.KeyID, clientIP, saasServiceAccountRouteKey(r))
	if err != nil {
		var authErr *SaaSServiceAccountAuthError
		if errors.As(err, &authErr) {
			writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "invalid API key", nil)
			return SaaSServiceAccountPrincipal{}, false
		}
		var rateErr *SaaSServiceAccountRateLimitError
		if errors.As(err, &rateErr) {
			applySaaSServiceAccountRateLimitHeaders(w, rateErr.State)
			writeEnvelope(w, http.StatusTooManyRequests, http.StatusTooManyRequests, rateErr.Error(), map[string]any{"rateLimit": saasServiceAccountRateLimitPayload(rateErr.State)})
			return SaaSServiceAccountPrincipal{}, false
		}
		writeSaaSAdminError(w, err)
		return SaaSServiceAccountPrincipal{}, false
	}
	principal.RateLimit = rateState
	applySaaSServiceAccountRateLimitHeaders(w, rateState)
	return principal, true
}

func saasServiceAccountRouteKey(r *http.Request) string {
	if r == nil {
		return "UNKNOWN /"
	}
	value := strings.ToUpper(strings.TrimSpace(r.Method)) + " " + strings.TrimSpace(r.URL.Path)
	if len(value) > saasServiceAccountUsageRouteKeyMaxLength {
		value = value[:saasServiceAccountUsageRouteKeyMaxLength]
	}
	return value
}

func applySaaSServiceAccountRateLimitHeaders(w http.ResponseWriter, state SaaSServiceAccountRateLimitState) {
	minuteRemaining := int64(state.RateLimitPerMinute) - state.MinuteRequestCount
	if minuteRemaining < 0 {
		minuteRemaining = 0
	}
	dailyRemaining := int64(-1)
	if state.DailyRequestLimit > 0 {
		dailyRemaining = state.DailyRequestLimit - state.DailyRequestCount
		if dailyRemaining < 0 {
			dailyRemaining = 0
		}
	}
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(state.RateLimitPerMinute))
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(minuteRemaining, 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(state.MinuteResetAt.Unix(), 10))
	w.Header().Set("X-RateLimit-Daily-Limit", strconv.FormatInt(state.DailyRequestLimit, 10))
	w.Header().Set("X-RateLimit-Daily-Remaining", strconv.FormatInt(dailyRemaining, 10))
	w.Header().Set("X-RateLimit-Daily-Reset", strconv.FormatInt(state.DailyResetAt.Unix(), 10))
	if state.LimitedBy != "" {
		resetAt := state.MinuteResetAt
		if state.LimitedBy == "daily" {
			resetAt = state.DailyResetAt
		}
		retryAfter := int64(time.Until(resetAt).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
	}
}

func (h *SaaSAdminHandler) saasServiceAccountStore(w http.ResponseWriter) (SaaSServiceAccountStore, bool) {
	store, ok := h.store.(SaaSServiceAccountStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "service account store is not configured", nil)
		return nil, false
	}
	return store, true
}

func (h *SaaSAdminHandler) CheckServiceAccountKeyProtection(ctx context.Context) error {
	if h == nil || h.serviceAccountKeyManager == nil {
		return errors.New("service account pepper manager is not configured")
	}
	if err := h.serviceAccountKeyManager.CheckConfiguration(ctx); err != nil {
		return err
	}
	status, err := h.saasServiceAccountKeyProtectionStatus(ctx, nil)
	if err != nil {
		return err
	}
	if len(status.MissingKeyIDs) > 0 {
		return fmt.Errorf("usable service account API keys reference unavailable peppers: %s", strings.Join(status.MissingKeyIDs, ", "))
	}
	return nil
}

func (h *SaaSAdminHandler) saasServiceAccountKeyProtectionStatus(ctx context.Context, items []SaaSServiceAccount) (SaaSServiceAccountKeyProtectionStatus, error) {
	status := SaaSServiceAccountKeyProtectionStatus{}
	if h != nil && h.serviceAccountKeyManager != nil {
		configStatus := h.serviceAccountKeyManager.ConfigStatus()
		status.ActiveKeyID = configStatus.ActiveKeyID
		status.ConfiguredKeyIDs = append([]string(nil), configStatus.KeyIDs...)
		status.KeyCount = configStatus.KeyCount
		status.DedicatedConfigured = configStatus.DedicatedConfigured
		status.LegacyJWTEnabled = configStatus.LegacyJWTEnabled
		status.RequireDedicated = configStatus.RequireDedicated
	}

	counts := []SaaSServiceAccountKeyProtectionCount{}
	if reporter, ok := h.store.(SaaSServiceAccountKeyProtectionStore); ok && reporter != nil {
		var err error
		counts, err = reporter.SaaSServiceAccountKeyProtectionCounts(ctx)
		if err != nil {
			return SaaSServiceAccountKeyProtectionStatus{}, err
		}
	} else {
		byKeyID := map[string]*SaaSServiceAccountKeyProtectionCount{}
		now := time.Now()
		for _, item := range items {
			for _, key := range item.Keys {
				keyID := strings.TrimSpace(key.HashKeyID)
				if keyID == "" {
					keyID = serviceaccountkey.LegacyJWTKeyID
				}
				count := byKeyID[keyID]
				if count == nil {
					count = &SaaSServiceAccountKeyProtectionCount{HashKeyID: keyID}
					byKeyID[keyID] = count
				}
				count.KeyCount++
				if saasServiceAccountKeyUsableForProtection(key, item, now) {
					count.UsableKeyCount++
				}
			}
		}
		for _, count := range byKeyID {
			counts = append(counts, *count)
		}
		sort.Slice(counts, func(i, j int) bool { return counts[i].HashKeyID < counts[j].HashKeyID })
	}

	usableKeyIDs := make([]string, 0, len(counts))
	for _, count := range counts {
		keyID := strings.TrimSpace(count.HashKeyID)
		if keyID == "" {
			keyID = serviceaccountkey.LegacyJWTKeyID
		}
		status.StoredKeyCount += count.KeyCount
		status.UsableKeyCount += count.UsableKeyCount
		if keyID == serviceaccountkey.LegacyJWTKeyID {
			status.LegacyStoredKeyCount += count.KeyCount
			status.LegacyUsableKeyCount += count.UsableKeyCount
		}
		if keyID == status.ActiveKeyID {
			status.ActiveStoredKeyCount += count.KeyCount
			status.ActiveUsableKeyCount += count.UsableKeyCount
		}
		if count.UsableKeyCount > 0 {
			usableKeyIDs = append(usableKeyIDs, keyID)
		}
	}
	if h != nil && h.serviceAccountKeyManager != nil {
		status.MissingKeyIDs = h.serviceAccountKeyManager.MissingKeyIDs(usableKeyIDs)
	} else {
		status.MissingKeyIDs = append([]string(nil), usableKeyIDs...)
		sort.Strings(status.MissingKeyIDs)
	}
	return status, nil
}

func saasServiceAccountKeyUsableForProtection(key SaaSServiceAccountKey, account SaaSServiceAccount, now time.Time) bool {
	if account.Status != SaaSServiceAccountStatusActive || account.TenantStatus != 1 {
		return false
	}
	if expiresAt, ok := parseSaaSAdminNormalizedDateTime(account.ExpiresAt); ok && !now.Before(expiresAt) {
		return false
	}
	if expiresAt, ok := parseSaaSAdminNormalizedDateTime(key.ExpiresAt); ok && !now.Before(expiresAt) {
		return false
	}
	if key.Status == SaaSServiceAccountKeyStatusActive {
		return true
	}
	if key.Status == SaaSServiceAccountKeyStatusRetiring {
		retireAt, ok := parseSaaSAdminNormalizedDateTime(key.RetireAt)
		return ok && now.Before(retireAt)
	}
	return false
}

func (h *SaaSAdminHandler) newSaaSServiceAccountKey() (SaaSServiceAccountKeyMaterial, error) {
	if h.serviceAccountKeyManager == nil {
		return SaaSServiceAccountKeyMaterial{}, errors.New("service account key pepper is not configured")
	}
	prefixBytes := make([]byte, 6)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(prefixBytes); err != nil {
		return SaaSServiceAccountKeyMaterial{}, err
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return SaaSServiceAccountKeyMaterial{}, err
	}
	prefix := hex.EncodeToString(prefixBytes)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	plainText := "mch_live_" + prefix + "_" + secret
	hashKeyID, digest, err := h.serviceAccountKeyManager.HashActive(plainText)
	if err != nil {
		return SaaSServiceAccountKeyMaterial{}, err
	}
	return SaaSServiceAccountKeyMaterial{
		Prefix: prefix, Hash: digest, HashKeyID: hashKeyID, LastFour: secret[len(secret)-4:], PlainText: plainText,
	}, nil
}

func hashSaaSServiceAccountKey(pepper string, plainText string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(plainText))
	return hex.EncodeToString(mac.Sum(nil))
}

func constantTimeHexEqual(left, right string) bool {
	leftBytes, leftErr := hex.DecodeString(strings.TrimSpace(left))
	rightBytes, rightErr := hex.DecodeString(strings.TrimSpace(right))
	return leftErr == nil && rightErr == nil && hmac.Equal(leftBytes, rightBytes)
}

func presentedSaaSServiceAccountKey(r *http.Request) (string, string, bool) {
	if r == nil {
		return "", "", false
	}
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(raw) >= 7 && strings.EqualFold(raw[:7], "Bearer ") {
		raw = strings.TrimSpace(raw[7:])
	} else if raw == "" {
		raw = strings.TrimSpace(r.Header.Get("X-API-Key"))
	} else {
		return "", "", false
	}
	parts := strings.SplitN(raw, "_", 4)
	if len(parts) != 4 || parts[0] != "mch" || parts[1] != "live" || !saasServiceAccountKeyPrefixPattern.MatchString(parts[2]) || len(parts[3]) < 40 {
		return "", "", false
	}
	return raw, parts[2], true
}

func saasServiceAccountPrincipalEnabled(principal SaaSServiceAccountPrincipal, now time.Time) bool {
	if principal.ServiceAccountStatus != SaaSServiceAccountStatusActive || principal.TenantStatus != 1 {
		return false
	}
	if expiresAt, ok := parseSaaSAdminNormalizedDateTime(principal.AccountExpiresAt); ok && !now.Before(expiresAt) {
		return false
	}
	if expiresAt, ok := parseSaaSAdminNormalizedDateTime(principal.KeyExpiresAt); ok && !now.Before(expiresAt) {
		return false
	}
	switch principal.KeyStatus {
	case SaaSServiceAccountKeyStatusActive:
		return true
	case SaaSServiceAccountKeyStatusRetiring:
		retireAt, ok := parseSaaSAdminNormalizedDateTime(principal.KeyRetireAt)
		return ok && now.Before(retireAt)
	default:
		return false
	}
}

func saasServiceAccountIPAllowed(clientIP string, allowedCIDRs []string) bool {
	if len(allowedCIDRs) == 0 {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return false
	}
	for _, raw := range allowedCIDRs {
		_, network, err := net.ParseCIDR(raw)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func saasServiceAccountOptionsFromQuery(r *http.Request) (SaaSServiceAccountOptions, error) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = SaaSServiceAccountStatusAll
	}
	if status != SaaSServiceAccountStatusAll && status != SaaSServiceAccountStatusActive && status != SaaSServiceAccountStatusDisabled {
		return SaaSServiceAccountOptions{}, errors.New("status 必须是 all、active 或 disabled")
	}
	limit := positiveQueryInt(r, "limit", 100)
	if limit > 500 {
		limit = 500
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 120 {
		return SaaSServiceAccountOptions{}, errors.New("keyword 最多 120 个字符")
	}
	return SaaSServiceAccountOptions{TenantID: positiveQueryInt(r, "tenantId", 0), Status: status, Keyword: keyword, Limit: limit}, nil
}

func saasServiceAccountUsageOptionsFromQuery(r *http.Request, now time.Time) (SaaSServiceAccountUsageOptions, error) {
	days := positiveQueryInt(r, "days", DefaultSaaSServiceAccountUsageHistoryDays)
	if days < 1 || days > DefaultSaaSServiceAccountUsageRetentionDays {
		return SaaSServiceAccountUsageOptions{}, errors.New("days 必须在 1 到 90 之间")
	}
	limit := positiveQueryInt(r, "limit", 10)
	if limit > 100 {
		limit = 100
	}
	serviceAccountID := int64(positiveQueryInt(r, "serviceAccountId", 0))
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return SaaSServiceAccountUsageOptions{
		TenantID: positiveQueryInt(r, "tenantId", 0), ServiceAccountID: serviceAccountID,
		Days: days, Limit: limit, DateFrom: today.AddDate(0, 0, -days+1).Format("2006-01-02"), DateTo: today.Format("2006-01-02"),
	}, nil
}

func fillSaaSServiceAccountUsageDays(items []SaaSServiceAccountUsageDaily, dateFrom string, days int) []SaaSServiceAccountUsageDaily {
	start, err := time.ParseInLocation("2006-01-02", dateFrom, time.Local)
	if err != nil || days <= 0 {
		return items
	}
	byDate := make(map[string]SaaSServiceAccountUsageDaily, len(items))
	for _, item := range items {
		byDate[item.UsageDate] = item
	}
	result := make([]SaaSServiceAccountUsageDaily, 0, days)
	for offset := 0; offset < days; offset++ {
		date := start.AddDate(0, 0, offset).Format("2006-01-02")
		item := byDate[date]
		item.UsageDate = date
		result = append(result, item)
	}
	return result
}

func normalizeSaaSServiceAccountCreate(input *SaaSServiceAccountCreate, now time.Time) error {
	if input == nil || input.TenantID <= 0 {
		return errors.New("tenantId 必须大于 0")
	}
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.KeyName = strings.TrimSpace(input.KeyName)
	if input.Status == "" {
		input.Status = SaaSServiceAccountStatusActive
	}
	if input.KeyName == "" {
		input.KeyName = "初始密钥"
	}
	if !saasServiceAccountCodePattern.MatchString(input.Code) {
		return errors.New("code 必须以小写字母开头，且只能包含小写字母、数字、下划线或连字符，长度 3 至 64")
	}
	if err := validateSaaSServiceAccountCommon(input.Name, input.Description, input.Status, &input.Scopes, &input.AllowedCIDRs); err != nil {
		return err
	}
	if input.RateLimitPerMinute == nil {
		value := DefaultSaaSServiceAccountRateLimitPerMinute
		input.RateLimitPerMinute = &value
	}
	if input.DailyRequestLimit == nil {
		value := DefaultSaaSServiceAccountDailyRequestLimit
		input.DailyRequestLimit = &value
	}
	if input.UsageAlertEnabled == nil {
		value := true
		input.UsageAlertEnabled = &value
	}
	if input.UsageWarningPercent == nil {
		value := DefaultSaaSServiceAccountUsageWarningPercent
		input.UsageWarningPercent = &value
	}
	if input.RejectionWarningCount == nil {
		value := DefaultSaaSServiceAccountRejectionWarningCount
		input.RejectionWarningCount = &value
	}
	if input.UsageAlertCooldownMinutes == nil {
		value := DefaultSaaSServiceAccountUsageAlertCooldownMinutes
		input.UsageAlertCooldownMinutes = &value
	}
	if err := validateSaaSServiceAccountRequestLimits(input.RateLimitPerMinute, input.DailyRequestLimit); err != nil {
		return err
	}
	if err := validateSaaSServiceAccountUsageAlertPolicy(input.UsageWarningPercent, input.RejectionWarningCount, input.UsageAlertCooldownMinutes); err != nil {
		return err
	}
	if len([]rune(input.KeyName)) > 80 {
		return errors.New("keyName 最多 80 个字符")
	}
	accountExpiry, err := normalizeSaaSServiceAccountExpiry(input.ExpiresAt, now, false)
	if err != nil {
		return fmt.Errorf("expiresAt %w", err)
	}
	keyRaw := input.KeyExpiresAt
	if strings.TrimSpace(keyRaw) == "" {
		keyRaw = now.Add(saasServiceAccountDefaultKeyTTL).Format("2006-01-02 15:04:05")
	}
	keyExpiry, err := normalizeSaaSServiceAccountExpiry(keyRaw, now, true)
	if err != nil {
		return fmt.Errorf("keyExpiresAt %w", err)
	}
	if accountExpiry != "" {
		accountTime, _ := parseSaaSAdminNormalizedDateTime(accountExpiry)
		keyTime, _ := parseSaaSAdminNormalizedDateTime(keyExpiry)
		if keyTime.After(accountTime) {
			return errors.New("keyExpiresAt 不能晚于服务账号 expiresAt")
		}
	}
	input.ExpiresAt = accountExpiry
	input.KeyExpiresAt = keyExpiry
	return nil
}

func normalizeSaaSServiceAccountUpdate(input *SaaSServiceAccountUpdate, now time.Time) error {
	if input == nil || input.ID <= 0 || input.ExpectedVersion <= 0 {
		return errors.New("id 和 expectedVersion 必须大于 0")
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if err := validateSaaSServiceAccountCommon(input.Name, input.Description, input.Status, &input.Scopes, &input.AllowedCIDRs); err != nil {
		return err
	}
	if err := validateSaaSServiceAccountRequestLimits(input.RateLimitPerMinute, input.DailyRequestLimit); err != nil {
		return err
	}
	if err := validateSaaSServiceAccountUsageAlertPolicy(input.UsageWarningPercent, input.RejectionWarningCount, input.UsageAlertCooldownMinutes); err != nil {
		return err
	}
	expiresAt, err := normalizeSaaSServiceAccountExpiry(input.ExpiresAt, now, false)
	if err != nil {
		return fmt.Errorf("expiresAt %w", err)
	}
	input.ExpiresAt = expiresAt
	return nil
}

func validateSaaSServiceAccountRequestLimits(rateLimitPerMinute, dailyRequestLimit *int) error {
	if rateLimitPerMinute != nil && (*rateLimitPerMinute < 1 || *rateLimitPerMinute > saasServiceAccountMaxRateLimitPerMinute) {
		return fmt.Errorf("rateLimitPerMinute 必须在 1 至 %d 之间", saasServiceAccountMaxRateLimitPerMinute)
	}
	if dailyRequestLimit != nil && (*dailyRequestLimit < 0 || *dailyRequestLimit > saasServiceAccountMaxDailyRequestLimit) {
		return fmt.Errorf("dailyRequestLimit 必须在 0 至 %d 之间，0 表示不限", saasServiceAccountMaxDailyRequestLimit)
	}
	return nil
}

func validateSaaSServiceAccountUsageAlertPolicy(usageWarningPercent, rejectionWarningCount, cooldownMinutes *int) error {
	if usageWarningPercent != nil && (*usageWarningPercent < 1 || *usageWarningPercent > 100) {
		return errors.New("usageWarningPercent 必须在 1 至 100 之间")
	}
	if rejectionWarningCount != nil && (*rejectionWarningCount < 0 || *rejectionWarningCount > saasServiceAccountMaxRejectionWarningCount) {
		return fmt.Errorf("rejectionWarningCount 必须在 0 至 %d 之间，0 表示关闭", saasServiceAccountMaxRejectionWarningCount)
	}
	if cooldownMinutes != nil && (*cooldownMinutes < 5 || *cooldownMinutes > saasServiceAccountMaxUsageAlertCooldownMinutes) {
		return fmt.Errorf("usageAlertCooldownMinutes 必须在 5 至 %d 之间", saasServiceAccountMaxUsageAlertCooldownMinutes)
	}
	return nil
}

func normalizeSaaSServiceAccountKeyRotate(input *SaaSServiceAccountKeyRotate, now time.Time) error {
	if input == nil || input.ServiceAccountID <= 0 || input.ExpectedVersion <= 0 {
		return errors.New("serviceAccountId 和 expectedVersion 必须大于 0")
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = "轮换密钥"
	}
	if len([]rune(input.Name)) > 80 {
		return errors.New("name 最多 80 个字符")
	}
	if input.GraceMinutes < 0 || input.GraceMinutes > 10080 {
		return errors.New("graceMinutes 必须在 0 至 10080 之间")
	}
	if strings.TrimSpace(input.ExpiresAt) == "" {
		input.ExpiresAt = now.Add(saasServiceAccountDefaultKeyTTL).Format("2006-01-02 15:04:05")
	}
	expiresAt, err := normalizeSaaSServiceAccountExpiry(input.ExpiresAt, now, true)
	if err != nil {
		return fmt.Errorf("expiresAt %w", err)
	}
	input.ExpiresAt = expiresAt
	return nil
}

func validateSaaSServiceAccountCommon(name, description, status string, scopes *[]string, cidrs *[]string) error {
	if name == "" || len([]rune(name)) > 120 {
		return errors.New("name 必填且最多 120 个字符")
	}
	if len([]rune(description)) > 255 {
		return errors.New("description 最多 255 个字符")
	}
	if status != SaaSServiceAccountStatusActive && status != SaaSServiceAccountStatusDisabled {
		return errors.New("status 必须是 active 或 disabled")
	}
	normalizedScopes := uniqueSortedStrings(*scopes)
	if len(normalizedScopes) == 0 {
		return errors.New("scopes 至少包含一项")
	}
	for _, scope := range normalizedScopes {
		if !SaaSServiceAccountScopeValid(scope) {
			return fmt.Errorf("不支持的 scope %s", scope)
		}
	}
	if len(normalizedScopes) > len(SaaSServiceAccountScopeCatalog()) {
		return errors.New("scopes 数量过多")
	}
	normalizedCIDRs, err := normalizeSaaSServiceAccountCIDRs(*cidrs)
	if err != nil {
		return err
	}
	*scopes = normalizedScopes
	*cidrs = normalizedCIDRs
	return nil
}

func normalizeSaaSServiceAccountCIDRs(values []string) ([]string, error) {
	if len(values) > 20 {
		return nil, errors.New("allowedCidrs 最多 20 项")
	}
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if ip := net.ParseIP(raw); ip != nil {
			if ip.To4() != nil {
				raw = ip.String() + "/32"
			} else {
				raw = ip.String() + "/128"
			}
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("allowedCidrs 包含无效值 %s", raw)
		}
		normalized := network.String()
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result, nil
}

func normalizeSaaSServiceAccountExpiry(raw string, now time.Time, required bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if required {
			return "", errors.New("必填")
		}
		return "", nil
	}
	parsed, ok := parseSaaSAdminNormalizedDateTime(raw)
	if !ok {
		return "", errors.New("格式必须是 YYYY-MM-DD HH:MM:SS、YYYY-MM-DD 或 RFC3339")
	}
	if !parsed.After(now) {
		return "", errors.New("必须晚于当前时间")
	}
	return parsed.Format("2006-01-02 15:04:05"), nil
}

func decodeSaaSServiceAccountJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid JSON body", nil)
		return false
	}
	return true
}

func writeOneTimeServiceAccountKey(w http.ResponseWriter, status int, payload map[string]any) {
	writeOneTimeSecretEnvelope(w, status, "API key 仅在本次响应中展示，请立即存入密钥管理器", payload)
}

func saasServiceAccountPayloads(items []SaaSServiceAccount) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasServiceAccountPayload(item))
	}
	return result
}

func saasServiceAccountPayload(item SaaSServiceAccount) map[string]any {
	routes := make([]map[string]any, 0, len(item.TodayRoutes))
	for _, route := range item.TodayRoutes {
		routes = append(routes, map[string]any{
			"routeKey": route.RouteKey, "requestCount": route.RequestCount, "rejectedCount": route.RejectedCount,
			"lastUsedAt": route.LastUsedAt, "lastUsedIp": route.LastUsedIP,
		})
	}
	return map[string]any{
		"id": item.ID, "tenantId": item.TenantID, "tenantName": item.TenantName, "tenantStatus": item.TenantStatus,
		"code": item.Code, "name": item.Name, "description": item.Description, "status": item.Status,
		"scopes": item.Scopes, "allowedCidrs": item.AllowedCIDRs, "expiresAt": item.ExpiresAt,
		"rateLimitPerMinute": item.RateLimitPerMinute, "dailyRequestLimit": item.DailyRequestLimit,
		"usageAlertEnabled": item.UsageAlertEnabled, "usageWarningPercent": item.UsageWarningPercent,
		"rejectionWarningCount": item.RejectionWarningCount, "usageAlertCooldownMinutes": item.UsageAlertCooldownMinutes,
		"usageAlertLastEvaluatedAt": item.UsageAlertLastEvaluatedAt, "usageAlertLastNotifiedAt": item.UsageAlertLastNotifiedAt,
		"rejectionAlertLastNotifiedAt": item.RejectionAlertLastNotifiedAt,
		"minuteRequestCount":           item.MinuteRequestCount, "minuteRejectedCount": item.MinuteRejectedCount,
		"dailyRequestCount": item.DailyRequestCount, "dailyRejectedCount": item.DailyRejectedCount, "todayRoutes": routes,
		"lastUsedAt": item.LastUsedAt, "lastUsedIp": item.LastUsedIP, "useCount": item.UseCount, "version": item.Version,
		"createdBy": item.CreatedBy, "updatedBy": item.UpdatedBy, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
		"keys": saasServiceAccountKeyPayloads(item.Keys),
	}
}

func saasServiceAccountKeyPayloads(items []SaaSServiceAccountKey) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasServiceAccountKeyPayload(item))
	}
	return result
}

func saasServiceAccountKeyPayload(item SaaSServiceAccountKey) map[string]any {
	return map[string]any{
		"id": item.ID, "serviceAccountId": item.ServiceAccountID, "name": item.Name, "prefix": item.Prefix,
		"hashKeyId": item.HashKeyID, "legacyPepper": item.HashKeyID == "" || item.HashKeyID == serviceaccountkey.LegacyJWTKeyID,
		"lastFour": item.LastFour, "display": "mch_live_" + item.Prefix + "_..." + item.LastFour, "status": item.Status,
		"expiresAt": item.ExpiresAt, "retireAt": item.RetireAt, "lastUsedAt": item.LastUsedAt, "lastUsedIp": item.LastUsedIP,
		"useCount": item.UseCount, "replacedByKeyId": item.ReplacedByKeyID, "revokedAt": item.RevokedAt,
		"revokedBy": item.RevokedBy, "version": item.Version, "createdBy": item.CreatedBy, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasServiceAccountKeyProtectionPayload(status SaaSServiceAccountKeyProtectionStatus) map[string]any {
	return map[string]any{
		"activeKeyId": status.ActiveKeyID, "configuredKeyIds": status.ConfiguredKeyIDs, "keyCount": status.KeyCount,
		"dedicatedConfigured": status.DedicatedConfigured, "legacyJwtEnabled": status.LegacyJWTEnabled,
		"requireDedicated": status.RequireDedicated, "storedKeyCount": status.StoredKeyCount, "usableKeyCount": status.UsableKeyCount,
		"legacyStoredKeyCount": status.LegacyStoredKeyCount, "legacyUsableKeyCount": status.LegacyUsableKeyCount,
		"activeStoredKeyCount": status.ActiveStoredKeyCount, "activeUsableKeyCount": status.ActiveUsableKeyCount,
		"missingKeyIds": status.MissingKeyIDs,
		"healthy":       len(status.MissingKeyIDs) == 0 && (!status.RequireDedicated || status.DedicatedConfigured),
	}
}

func saasServiceAccountSummaryPayload(items []SaaSServiceAccount) map[string]any {
	now := time.Now()
	activeAccounts, disabledAccounts, expiredAccounts, usageAlertAccounts := 0, 0, 0, 0
	keyCount, activeKeys, retiringKeys, revokedKeys := 0, 0, 0, 0
	todayRequests, todayRejected, currentMinuteRequests := int64(0), int64(0), int64(0)
	limitedAccounts := 0
	for _, item := range items {
		if item.UsageAlertEnabled {
			usageAlertAccounts++
		}
		todayRequests += item.DailyRequestCount
		todayRejected += item.DailyRejectedCount
		currentMinuteRequests += item.MinuteRequestCount
		if item.MinuteRejectedCount > 0 || item.DailyRejectedCount > 0 {
			limitedAccounts++
		}
		if expiresAt, ok := parseSaaSAdminNormalizedDateTime(item.ExpiresAt); ok && !now.Before(expiresAt) {
			expiredAccounts++
		} else if item.Status == SaaSServiceAccountStatusActive {
			activeAccounts++
		} else {
			disabledAccounts++
		}
		for _, key := range item.Keys {
			keyCount++
			switch key.Status {
			case SaaSServiceAccountKeyStatusActive:
				activeKeys++
			case SaaSServiceAccountKeyStatusRetiring:
				retiringKeys++
			case SaaSServiceAccountKeyStatusRevoked:
				revokedKeys++
			}
		}
	}
	return map[string]any{
		"accountCount": len(items), "activeAccountCount": activeAccounts, "disabledAccountCount": disabledAccounts,
		"expiredAccountCount": expiredAccounts, "keyCount": keyCount, "activeKeyCount": activeKeys,
		"retiringKeyCount": retiringKeys, "revokedKeyCount": revokedKeys,
		"todayRequestCount": todayRequests, "todayRejectedCount": todayRejected,
		"currentMinuteRequestCount": currentMinuteRequests, "limitedAccountCount": limitedAccounts,
		"usageAlertAccountCount": usageAlertAccounts,
	}
}

func saasServiceAccountUsageReportPayload(report SaaSServiceAccountUsageReport) map[string]any {
	daily := make([]map[string]any, 0, len(report.Daily))
	for _, item := range report.Daily {
		daily = append(daily, map[string]any{
			"usageDate": item.UsageDate, "requestCount": item.RequestCount, "rejectedCount": item.RejectedCount,
			"activeAccountCount": item.ActiveAccountCount,
		})
	}
	routes := make([]map[string]any, 0, len(report.Routes))
	for _, item := range report.Routes {
		routes = append(routes, map[string]any{
			"routeKey": item.RouteKey, "requestCount": item.RequestCount, "rejectedCount": item.RejectedCount,
			"activeAccountCount": item.ActiveAccountCount, "lastUsedAt": item.LastUsedAt,
		})
	}
	accounts := make([]map[string]any, 0, len(report.Accounts))
	for _, item := range report.Accounts {
		accounts = append(accounts, map[string]any{
			"serviceAccountId": item.ServiceAccountID, "serviceAccountCode": item.ServiceAccountCode,
			"serviceAccountName": item.ServiceAccountName, "tenantId": item.TenantID, "tenantName": item.TenantName,
			"requestCount": item.RequestCount, "rejectedCount": item.RejectedCount, "routeCount": item.RouteCount,
			"lastUsedAt": item.LastUsedAt,
		})
	}
	return map[string]any{
		"dateFrom": report.DateFrom, "dateTo": report.DateTo, "retentionDays": report.RetentionDays,
		"oldestUsageDate": report.OldestUsageDate, "storedRowCount": report.StoredRowCount,
		"requestCount": report.RequestCount, "rejectedCount": report.RejectedCount,
		"activeAccountCount": report.ActiveAccountCount, "limitedAccountCount": report.LimitedAccountCount,
		"openAlertCount": report.OpenAlertCount,
		"daily":          daily, "routes": routes, "accounts": accounts,
	}
}

func saasServiceAccountScopeCatalogPayload() []map[string]any {
	items := SaaSServiceAccountScopeCatalog()
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{"code": item.Code, "name": item.Name, "description": item.Description})
	}
	return result
}

func saasServiceAccountPrincipalPayload(item SaaSServiceAccountPrincipal) map[string]any {
	return map[string]any{
		"serviceAccountId": item.ServiceAccountID, "serviceAccountCode": item.ServiceAccountCode,
		"serviceAccountName": item.ServiceAccountName, "tenantId": item.TenantID, "tenantName": item.TenantName,
		"scopes": item.Scopes, "keyId": item.KeyID, "keyName": item.KeyName, "keyPrefix": item.KeyPrefix,
		"accountExpiresAt": item.AccountExpiresAt, "keyExpiresAt": item.KeyExpiresAt, "keyRetireAt": item.KeyRetireAt,
		"rateLimit": saasServiceAccountRateLimitPayload(item.RateLimit),
	}
}

func saasServiceAccountRateLimitPayload(state SaaSServiceAccountRateLimitState) map[string]any {
	minuteRemaining := int64(state.RateLimitPerMinute) - state.MinuteRequestCount
	if minuteRemaining < 0 {
		minuteRemaining = 0
	}
	dailyRemaining := int64(-1)
	if state.DailyRequestLimit > 0 {
		dailyRemaining = state.DailyRequestLimit - state.DailyRequestCount
		if dailyRemaining < 0 {
			dailyRemaining = 0
		}
	}
	return map[string]any{
		"rateLimitPerMinute": state.RateLimitPerMinute, "minuteRequestCount": state.MinuteRequestCount,
		"minuteRejectedCount": state.MinuteRejectedCount, "minuteRemaining": minuteRemaining,
		"minuteResetAt": state.MinuteResetAt.Format(time.RFC3339), "dailyRequestLimit": state.DailyRequestLimit,
		"dailyRequestCount": state.DailyRequestCount, "dailyRejectedCount": state.DailyRejectedCount,
		"dailyRemaining": dailyRemaining, "dailyUnlimited": state.DailyRequestLimit == 0,
		"dailyResetAt": state.DailyResetAt.Format(time.RFC3339), "limitedBy": state.LimitedBy,
	}
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func serviceAccountTargetID(id int64) string {
	return strconv.FormatInt(id, 10)
}
