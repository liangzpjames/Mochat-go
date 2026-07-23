package dashboard

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const (
	SaaSAdminAuditIntegrityStatusUnsealed = "unsealed"
	SaaSAdminAuditIntegrityStatusHealthy  = "healthy"
	SaaSAdminAuditIntegrityStatusFailed   = "failed"

	SaaSAdminOperationActionAuditIntegrityVerify = "saas.admin.audit.integrity.verify"
	SaaSAdminOperationTargetAuditIntegrity       = "saas_admin_audit_integrity"
)

type SaaSAdminAuditIntegrityOptions struct {
	TenantID          int    `json:"tenantId"`
	Limit             int    `json:"limit"`
	VerificationLimit int    `json:"verificationLimit"`
	Source            string `json:"-"`
	ActorUserID       int    `json:"-"`
	ActorTenantID     int    `json:"-"`
	RecordOperation   bool   `json:"-"`
}

type SaaSAdminAuditIntegrityChain struct {
	TenantID               int
	TenantName             string
	AnchorLogID            int64
	AnchorHash             string
	LegacyLogCount         int64
	LastLogID              int64
	LastHash               string
	SignedLogCount         int64
	Status                 string
	SealedAt               string
	LastVerifiedAt         string
	LastVerificationStatus string
	LastFailedLogID        int64
	LastVerificationError  string
	Version                int
}

type SaaSAdminAuditIntegrityVerification struct {
	ID               int64
	TenantID         int
	TenantName       string
	Source           string
	Status           string
	AnchorLogID      int64
	AnchorHash       string
	LegacyLogCount   int64
	SignedLogCount   int64
	VerifiedLogCount int64
	ChainHeadLogID   int64
	ChainHeadHash    string
	FailedLogID      int64
	ErrorMessage     string
	ActorUserID      int
	ActorTenantID    int
	StartedAt        string
	FinishedAt       string
}

type SaaSAdminAuditIntegritySummary struct {
	ChainCount                int
	UnsealedTenantCount       int
	HealthyChainCount         int
	FailedChainCount          int
	LegacyLogCount            int64
	SignedLogCount            int64
	RetentionDays             int
	RetentionCutoff           string
	RetentionEligibleLogCount int64
}

type SaaSAdminAuditIntegrityOverview struct {
	Summary       SaaSAdminAuditIntegritySummary
	Chains        []SaaSAdminAuditIntegrityChain
	Verifications []SaaSAdminAuditIntegrityVerification
}

type SaaSAdminAuditIntegrityVerifyResult struct {
	ScannedChains int
	SealedChains  int
	HealthyChains int
	FailedChains  int
	LegacyLogs    int64
	SignedLogs    int64
	VerifiedLogs  int64
	OperationID   int64
	VerifiedAt    string
	Chains        []SaaSAdminAuditIntegrityChain
}

type SaaSAdminAuditIntegrityStore interface {
	SaaSAdminAuditIntegrityOverview(ctx context.Context, options SaaSAdminAuditIntegrityOptions) (SaaSAdminAuditIntegrityOverview, error)
	VerifySaaSAdminAuditIntegrity(ctx context.Context, options SaaSAdminAuditIntegrityOptions) (SaaSAdminAuditIntegrityVerifyResult, error)
}

func (h *SaaSAdminHandler) AuditIntegrity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminAuditIntegrityStore(w)
	if !ok {
		return
	}
	options, err := parseSaaSAdminAuditIntegrityQuery(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	overview, err := store.SaaSAdminAuditIntegrityOverview(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", saasAdminAuditIntegrityOverviewPayload(overview))
}

func (h *SaaSAdminHandler) AuditIntegrityVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminAuditIntegrityStore(w)
	if !ok {
		return
	}
	var options SaaSAdminAuditIntegrityOptions
	if !decodeSaaSServiceAccountJSON(w, r, &options) {
		return
	}
	if err := normalizeSaaSAdminAuditIntegrityOptions(&options); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	options.Source = "admin_manual"
	options.ActorUserID = user.ID
	options.ActorTenantID = user.TenantID
	options.RecordOperation = true
	result, err := store.VerifySaaSAdminAuditIntegrity(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", saasAdminAuditIntegrityVerifyResultPayload(result))
}

func parseSaaSAdminAuditIntegrityQuery(r *http.Request) (SaaSAdminAuditIntegrityOptions, error) {
	options := SaaSAdminAuditIntegrityOptions{Limit: 50, VerificationLimit: 20}
	if r == nil {
		return options, nil
	}
	var err error
	if value := strings.TrimSpace(r.URL.Query().Get("tenantId")); value != "" {
		options.TenantID, err = strconv.Atoi(value)
		if err != nil {
			return options, errors.New("tenantId 必须是非负整数")
		}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		options.Limit, err = strconv.Atoi(value)
		if err != nil {
			return options, errors.New("limit 必须是整数")
		}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("verificationLimit")); value != "" {
		options.VerificationLimit, err = strconv.Atoi(value)
		if err != nil {
			return options, errors.New("verificationLimit 必须是整数")
		}
	}
	if err := normalizeSaaSAdminAuditIntegrityOptions(&options); err != nil {
		return options, err
	}
	return options, nil
}

func normalizeSaaSAdminAuditIntegrityOptions(options *SaaSAdminAuditIntegrityOptions) error {
	if options == nil || options.TenantID < 0 {
		return errors.New("tenantId 必须是非负整数")
	}
	if options.Limit == 0 {
		options.Limit = 50
	}
	if options.Limit < 1 || options.Limit > 500 {
		return errors.New("limit 必须在 1 至 500 之间")
	}
	if options.VerificationLimit == 0 {
		options.VerificationLimit = 20
	}
	if options.VerificationLimit < 1 || options.VerificationLimit > 200 {
		return errors.New("verificationLimit 必须在 1 至 200 之间")
	}
	return nil
}

func (h *SaaSAdminHandler) saasAdminAuditIntegrityStore(w http.ResponseWriter) (SaaSAdminAuditIntegrityStore, bool) {
	store, ok := h.store.(SaaSAdminAuditIntegrityStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "audit integrity store is not configured", nil)
		return nil, false
	}
	return store, true
}

func saasAdminAuditIntegrityOverviewPayload(overview SaaSAdminAuditIntegrityOverview) map[string]any {
	return map[string]any{
		"summary":       saasAdminAuditIntegritySummaryPayload(overview.Summary),
		"chains":        saasAdminAuditIntegrityChainPayloads(overview.Chains),
		"verifications": saasAdminAuditIntegrityVerificationPayloads(overview.Verifications),
	}
}

func saasAdminAuditIntegrityVerifyResultPayload(result SaaSAdminAuditIntegrityVerifyResult) map[string]any {
	return map[string]any{
		"scannedChains": result.ScannedChains,
		"sealedChains":  result.SealedChains,
		"healthyChains": result.HealthyChains,
		"failedChains":  result.FailedChains,
		"legacyLogs":    result.LegacyLogs,
		"signedLogs":    result.SignedLogs,
		"verifiedLogs":  result.VerifiedLogs,
		"operationId":   result.OperationID,
		"verifiedAt":    result.VerifiedAt,
		"chains":        saasAdminAuditIntegrityChainPayloads(result.Chains),
	}
}

func saasAdminAuditIntegritySummaryPayload(summary SaaSAdminAuditIntegritySummary) map[string]any {
	return map[string]any{
		"chainCount":                summary.ChainCount,
		"unsealedTenantCount":       summary.UnsealedTenantCount,
		"healthyChainCount":         summary.HealthyChainCount,
		"failedChainCount":          summary.FailedChainCount,
		"legacyLogCount":            summary.LegacyLogCount,
		"signedLogCount":            summary.SignedLogCount,
		"retentionDays":             summary.RetentionDays,
		"retentionCutoff":           summary.RetentionCutoff,
		"retentionEligibleLogCount": summary.RetentionEligibleLogCount,
	}
}

func saasAdminAuditIntegrityChainPayloads(chains []SaaSAdminAuditIntegrityChain) []map[string]any {
	items := make([]map[string]any, 0, len(chains))
	for _, item := range chains {
		items = append(items, map[string]any{
			"tenantId": item.TenantID, "tenantName": item.TenantName,
			"anchorLogId": item.AnchorLogID, "anchorHash": item.AnchorHash, "legacyLogCount": item.LegacyLogCount,
			"lastLogId": item.LastLogID, "lastHash": item.LastHash, "signedLogCount": item.SignedLogCount,
			"status": item.Status, "sealedAt": item.SealedAt, "lastVerifiedAt": item.LastVerifiedAt,
			"lastVerificationStatus": item.LastVerificationStatus, "lastFailedLogId": item.LastFailedLogID,
			"lastVerificationError": item.LastVerificationError, "version": item.Version,
		})
	}
	return items
}

func saasAdminAuditIntegrityVerificationPayloads(verifications []SaaSAdminAuditIntegrityVerification) []map[string]any {
	items := make([]map[string]any, 0, len(verifications))
	for _, item := range verifications {
		items = append(items, map[string]any{
			"id": item.ID, "tenantId": item.TenantID, "tenantName": item.TenantName,
			"source": item.Source, "status": item.Status,
			"anchorLogId": item.AnchorLogID, "anchorHash": item.AnchorHash, "legacyLogCount": item.LegacyLogCount,
			"signedLogCount": item.SignedLogCount, "verifiedLogCount": item.VerifiedLogCount,
			"chainHeadLogId": item.ChainHeadLogID, "chainHeadHash": item.ChainHeadHash,
			"failedLogId": item.FailedLogID, "errorMessage": item.ErrorMessage,
			"actorUserId": item.ActorUserID, "actorTenantId": item.ActorTenantID,
			"startedAt": item.StartedAt, "finishedAt": item.FinishedAt,
		})
	}
	return items
}
