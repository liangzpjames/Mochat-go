package dashboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	SaaSTenantDomainDeliveryStatusUnconfigured = "unconfigured"
	SaaSTenantDomainDeliveryStatusPending      = "pending"
	SaaSTenantDomainDeliveryStatusProvisioning = "provisioning"
	SaaSTenantDomainDeliveryStatusReady        = "ready"
	SaaSTenantDomainDeliveryStatusDegraded     = "degraded"
	SaaSTenantDomainDeliveryStatusFailed       = "failed"
	SaaSTenantDomainDeliveryStatusDisabled     = "disabled"
	SaaSTenantDomainDeliveryStatusDeleted      = "deleted"

	SaaSTenantDomainRoutingStatusPending      = "pending"
	SaaSTenantDomainRoutingStatusProvisioning = "provisioning"
	SaaSTenantDomainRoutingStatusReady        = "ready"
	SaaSTenantDomainRoutingStatusFailed       = "failed"
	SaaSTenantDomainRoutingStatusDisabled     = "disabled"
	SaaSTenantDomainRoutingStatusDeleted      = "deleted"

	SaaSTenantDomainCertificateStatusPending      = "pending"
	SaaSTenantDomainCertificateStatusProvisioning = "provisioning"
	SaaSTenantDomainCertificateStatusActive       = "active"
	SaaSTenantDomainCertificateStatusExpiring     = "expiring"
	SaaSTenantDomainCertificateStatusFailed       = "failed"
	SaaSTenantDomainCertificateStatusRevoked      = "revoked"
	SaaSTenantDomainCertificateStatusDisabled     = "disabled"
	SaaSTenantDomainCertificateStatusDeleted      = "deleted"

	SaaSTenantDomainDeliveryActionProvision = "provision"
	SaaSTenantDomainDeliveryActionRefresh   = "refresh"
	SaaSTenantDomainDeliveryActionDisable   = "disable"
	SaaSTenantDomainDeliveryActionDelete    = "delete"

	SaaSTenantDomainDeliveryJobStatusAll        = "all"
	SaaSTenantDomainDeliveryJobStatusPending    = "pending"
	SaaSTenantDomainDeliveryJobStatusProcessing = "processing"
	SaaSTenantDomainDeliveryJobStatusWaiting    = "waiting"
	SaaSTenantDomainDeliveryJobStatusSucceeded  = "succeeded"
	SaaSTenantDomainDeliveryJobStatusFailed     = "failed"
	SaaSTenantDomainDeliveryJobStatusCanceled   = "canceled"

	SaaSTenantDomainDeliveryBridgeStatusAccepted = "accepted"
	SaaSTenantDomainDeliveryBridgeStatusReady    = "ready"
	SaaSTenantDomainDeliveryBridgeStatusFailed   = "failed"

	SaaSTenantDomainDeliveryDefaultMaxAttempts = 5
	SaaSTenantDomainDeliveryMaxListLimit       = 500
	SaaSTenantDomainDeliveryCronTaskName       = "cron-saas-tenant-domain-delivery"

	SaaSAdminOperationTargetTenantDomainDelivery = "saas_tenant_domain_delivery"
)

type SaaSTenantDomainDelivery struct {
	ID                   int64  `json:"id"`
	DomainID             int64  `json:"domainId"`
	TenantID             int    `json:"tenantId"`
	DeliveryStatus       string `json:"deliveryStatus"`
	RoutingStatus        string `json:"routingStatus"`
	CertificateStatus    string `json:"certificateStatus"`
	Provider             string `json:"provider"`
	ProviderRequestID    string `json:"providerRequestId"`
	CertificateID        string `json:"certificateId"`
	CertificateNotBefore string `json:"certificateNotBefore"`
	CertificateExpiresAt string `json:"certificateExpiresAt"`
	LastEventAt          string `json:"lastEventAt"`
	LastReconciledAt     string `json:"lastReconciledAt"`
	LastError            string `json:"lastError"`
	Version              int    `json:"version"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

type SaaSTenantDomainDeliveryJob struct {
	ID                   int64                    `json:"id"`
	JobNo                string                   `json:"jobNo"`
	DomainID             int64                    `json:"domainId"`
	TenantID             int                      `json:"tenantId"`
	Action               string                   `json:"action"`
	Status               string                   `json:"status"`
	Attempts             int                      `json:"attempts"`
	MaxAttempts          int                      `json:"maxAttempts"`
	NextAttemptAt        string                   `json:"nextAttemptAt"`
	LeaseExpiresAt       string                   `json:"leaseExpiresAt"`
	Provider             string                   `json:"provider"`
	ProviderRequestID    string                   `json:"providerRequestId"`
	RequestPayloadSHA256 string                   `json:"requestPayloadSha256"`
	LastError            string                   `json:"lastError"`
	ActorUserID          int                      `json:"actorUserId"`
	ActorTenantID        int                      `json:"actorTenantId"`
	OperationID          int64                    `json:"operationId"`
	StartedAt            string                   `json:"startedAt"`
	FinishedAt           string                   `json:"finishedAt"`
	CreatedAt            string                   `json:"createdAt"`
	UpdatedAt            string                   `json:"updatedAt"`
	Domain               SaaSTenantDomain         `json:"domain"`
	Delivery             SaaSTenantDomainDelivery `json:"delivery"`
}

type SaaSTenantDomainDeliveryJobOptions struct {
	DomainID int64
	TenantID int
	Status   string
	Limit    int
}

type SaaSTenantDomainDeliveryRequest struct {
	DomainID      int64
	Action        string
	MaxAttempts   int
	ActorUserID   int
	ActorTenantID int
}

type SaaSTenantDomainDeliveryRequestResult struct {
	Delivery    SaaSTenantDomainDelivery
	Job         SaaSTenantDomainDeliveryJob
	OperationID int64
	Reused      bool
}

type SaaSTenantDomainDeliveryRetry struct {
	JobID         int64
	ActorUserID   int
	ActorTenantID int
}

type SaaSTenantDomainDeliveryClaimOptions struct {
	Limit         int
	LeaseDuration time.Duration
}

type SaaSTenantDomainDeliveryBridgeUpdate struct {
	Status               string `json:"status"`
	Provider             string `json:"provider"`
	ProviderRequestID    string `json:"requestId"`
	RoutingStatus        string `json:"routingStatus"`
	CertificateStatus    string `json:"certificateStatus"`
	CertificateID        string `json:"certificateId"`
	CertificateNotBefore string `json:"certificateNotBefore"`
	CertificateExpiresAt string `json:"certificateExpiresAt"`
	Error                string `json:"error"`
	OccurredAt           string `json:"occurredAt"`
}

type SaaSTenantDomainDeliveryCompletion struct {
	JobID            int64
	JobNo            string
	ExpectedAttempts int
	Update           SaaSTenantDomainDeliveryBridgeUpdate
	RequestHash      string
	CallError        string
	RetryDelay       time.Duration
	CallbackWait     time.Duration
}

type SaaSTenantDomainDeliveryCallbackEvent struct {
	EventID              string `json:"eventId"`
	EventType            string `json:"eventType"`
	JobNo                string `json:"jobNo"`
	DomainID             int64  `json:"domainId"`
	Hostname             string `json:"hostname"`
	Status               string `json:"status"`
	Provider             string `json:"provider"`
	ProviderRequestID    string `json:"requestId"`
	RoutingStatus        string `json:"routingStatus"`
	CertificateStatus    string `json:"certificateStatus"`
	CertificateID        string `json:"certificateId"`
	CertificateNotBefore string `json:"certificateNotBefore"`
	CertificateExpiresAt string `json:"certificateExpiresAt"`
	Error                string `json:"error"`
	OccurredAt           string `json:"occurredAt"`
	PayloadSHA256        string `json:"-"`
	SignatureTimestamp   int64  `json:"-"`
}

type SaaSTenantDomainDeliveryCallbackResult struct {
	Duplicate bool
	Ignored   bool
	Delivery  SaaSTenantDomainDelivery
	Job       SaaSTenantDomainDeliveryJob
}

type SaaSTenantDomainDeliveryStore interface {
	SaaSAdminTenantDomainDeliveryJobs(context.Context, SaaSTenantDomainDeliveryJobOptions) ([]SaaSTenantDomainDeliveryJob, error)
	RequestSaaSTenantDomainDelivery(context.Context, SaaSTenantDomainDeliveryRequest) (SaaSTenantDomainDeliveryRequestResult, error)
	RetrySaaSTenantDomainDelivery(context.Context, SaaSTenantDomainDeliveryRetry) (SaaSTenantDomainDeliveryRequestResult, error)
	ClaimSaaSTenantDomainDeliveryJobs(context.Context, SaaSTenantDomainDeliveryClaimOptions) ([]SaaSTenantDomainDeliveryJob, error)
	CompleteSaaSTenantDomainDeliveryJob(context.Context, SaaSTenantDomainDeliveryCompletion) (SaaSTenantDomainDeliveryJob, error)
	ApplySaaSTenantDomainDeliveryCallback(context.Context, SaaSTenantDomainDeliveryCallbackEvent) (SaaSTenantDomainDeliveryCallbackResult, error)
}

type SaaSTenantDomainDeliveryBridge interface {
	Deliver(context.Context, SaaSTenantDomainDeliveryJob, string) (SaaSTenantDomainDeliveryBridgeUpdate, string, error)
}

type SaaSTenantDomainDeliveryProcessor struct {
	store        SaaSTenantDomainDeliveryStore
	bridge       SaaSTenantDomainDeliveryBridge
	callbackURL  string
	limit        int
	lease        time.Duration
	retryDelay   time.Duration
	callbackWait time.Duration
	logger       *log.Logger
}

type SaaSTenantDomainDeliveryProcessorResult struct {
	Claimed   int
	Succeeded int
	Waiting   int
	Retried   int
	Failed    int
}

func NewSaaSTenantDomainDeliveryProcessor(store SaaSTenantDomainDeliveryStore, bridge SaaSTenantDomainDeliveryBridge, callbackURL string, limit int, lease, retryDelay, callbackWait time.Duration, logger *log.Logger) *SaaSTenantDomainDeliveryProcessor {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	if retryDelay <= 0 {
		retryDelay = time.Minute
	}
	if callbackWait <= 0 {
		callbackWait = 10 * time.Minute
	}
	if logger == nil {
		logger = log.Default()
	}
	return &SaaSTenantDomainDeliveryProcessor{store: store, bridge: bridge, callbackURL: strings.TrimSpace(callbackURL), limit: limit, lease: lease, retryDelay: retryDelay, callbackWait: callbackWait, logger: logger}
}

func (p *SaaSTenantDomainDeliveryProcessor) CheckConfiguration(context.Context) error {
	if p == nil || p.store == nil || p.bridge == nil {
		return errors.New("域名交付 Bridge 未配置")
	}
	return nil
}

func (p *SaaSTenantDomainDeliveryProcessor) RunOnce(ctx context.Context) error {
	_, err := p.Process(ctx)
	return err
}

func (p *SaaSTenantDomainDeliveryProcessor) Process(ctx context.Context) (SaaSTenantDomainDeliveryProcessorResult, error) {
	if err := p.CheckConfiguration(ctx); err != nil {
		return SaaSTenantDomainDeliveryProcessorResult{}, err
	}
	jobs, err := p.store.ClaimSaaSTenantDomainDeliveryJobs(ctx, SaaSTenantDomainDeliveryClaimOptions{Limit: p.limit, LeaseDuration: p.lease})
	if err != nil {
		return SaaSTenantDomainDeliveryProcessorResult{}, err
	}
	result := SaaSTenantDomainDeliveryProcessorResult{Claimed: len(jobs)}
	var processErrors []error
	for _, job := range jobs {
		update, requestHash, callErr := p.bridge.Deliver(ctx, job, p.callbackURL)
		completion := SaaSTenantDomainDeliveryCompletion{JobID: job.ID, JobNo: job.JobNo, ExpectedAttempts: job.Attempts, Update: update, RequestHash: requestHash, RetryDelay: p.retryDelay, CallbackWait: p.callbackWait}
		if callErr != nil {
			completion.CallError = truncateTenantDomainDeliveryText(callErr.Error(), 500)
		}
		updated, completeErr := p.store.CompleteSaaSTenantDomainDeliveryJob(ctx, completion)
		if completeErr != nil {
			processErrors = append(processErrors, fmt.Errorf("complete domain delivery job %s: %w", job.JobNo, completeErr))
			result.Failed++
			continue
		}
		switch updated.Status {
		case SaaSTenantDomainDeliveryJobStatusSucceeded:
			result.Succeeded++
		case SaaSTenantDomainDeliveryJobStatusWaiting:
			result.Waiting++
		case SaaSTenantDomainDeliveryJobStatusPending:
			result.Retried++
		case SaaSTenantDomainDeliveryJobStatusFailed:
			result.Failed++
		}
		if callErr != nil {
			p.logger.Printf("SaaS tenant domain delivery bridge call persisted: job=%s status=%s error=%s", job.JobNo, updated.Status, completion.CallError)
		}
	}
	p.logger.Printf("SaaS tenant domain delivery processed: claimed=%d succeeded=%d waiting=%d retried=%d failed=%d", result.Claimed, result.Succeeded, result.Waiting, result.Retried, result.Failed)
	return result, errors.Join(processErrors...)
}

func (h *SaaSAdminHandler) TenantDomainDeliveryJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasTenantDomainDeliveryStore(w)
	if !ok {
		return
	}
	options, err := saasTenantDomainDeliveryJobOptions(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	items, err := store.SaaSAdminTenantDomainDeliveryJobs(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"items": items, "returnedCount": len(items)})
}

func (h *SaaSAdminHandler) TenantDomainDelivery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasTenantDomainDeliveryStore(w)
	if !ok {
		return
	}
	var request struct {
		Action   string `json:"action"`
		DomainID int64  `json:"domainId"`
		JobID    int64  `json:"jobId"`
	}
	if err := decodeSaaSAdminAccessJSON(r, &request); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	var result SaaSTenantDomainDeliveryRequestResult
	var err error
	if request.Action == "retry" {
		if request.JobID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "jobId 必填", nil)
			return
		}
		result, err = store.RetrySaaSTenantDomainDelivery(r.Context(), SaaSTenantDomainDeliveryRetry{JobID: request.JobID, ActorUserID: user.ID, ActorTenantID: user.TenantID})
	} else {
		if request.DomainID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "domainId 必填", nil)
			return
		}
		if request.Action == "reconcile" {
			request.Action = SaaSTenantDomainDeliveryActionRefresh
		}
		if !saasTenantDomainDeliveryActionValid(request.Action) {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "action 必须是 reconcile、provision、refresh、disable、delete 或 retry", nil)
			return
		}
		result, err = store.RequestSaaSTenantDomainDelivery(r.Context(), SaaSTenantDomainDeliveryRequest{DomainID: request.DomainID, Action: request.Action, MaxAttempts: SaaSTenantDomainDeliveryDefaultMaxAttempts, ActorUserID: user.ID, ActorTenantID: user.TenantID})
	}
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Reused {
		status = http.StatusOK
	}
	writeEnvelope(w, status, status, "success", map[string]any{"delivery": result.Delivery, "job": result.Job, "operationId": result.OperationID, "reused": result.Reused})
}

func (h *SaaSAdminHandler) saasTenantDomainDeliveryStore(w http.ResponseWriter) (SaaSTenantDomainDeliveryStore, bool) {
	store, ok := h.store.(SaaSTenantDomainDeliveryStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "SaaS tenant domain delivery store is not configured", nil)
		return nil, false
	}
	return store, true
}

func saasTenantDomainDeliveryJobOptions(r *http.Request) (SaaSTenantDomainDeliveryJobOptions, error) {
	options := SaaSTenantDomainDeliveryJobOptions{Status: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))), Limit: positiveQueryInt(r, "limit", 100)}
	if options.Limit > SaaSTenantDomainDeliveryMaxListLimit {
		options.Limit = SaaSTenantDomainDeliveryMaxListLimit
	}
	if value := strings.TrimSpace(r.URL.Query().Get("domainId")); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return options, errors.New("domainId 无效")
		}
		options.DomainID = id
	}
	if value := strings.TrimSpace(r.URL.Query().Get("tenantId")); value != "" {
		id, err := strconv.Atoi(value)
		if err != nil || id <= 0 {
			return options, errors.New("tenantId 无效")
		}
		options.TenantID = id
	}
	if options.Status == "" {
		options.Status = SaaSTenantDomainDeliveryJobStatusAll
	}
	if options.Status != SaaSTenantDomainDeliveryJobStatusAll && !saasTenantDomainDeliveryJobStatusValid(options.Status) {
		return options, errors.New("status 无效")
	}
	return options, nil
}

func saasTenantDomainDeliveryActionValid(value string) bool {
	switch value {
	case SaaSTenantDomainDeliveryActionProvision, SaaSTenantDomainDeliveryActionRefresh, SaaSTenantDomainDeliveryActionDisable, SaaSTenantDomainDeliveryActionDelete:
		return true
	default:
		return false
	}
}

func saasTenantDomainDeliveryJobStatusValid(value string) bool {
	switch value {
	case SaaSTenantDomainDeliveryJobStatusPending, SaaSTenantDomainDeliveryJobStatusProcessing, SaaSTenantDomainDeliveryJobStatusWaiting, SaaSTenantDomainDeliveryJobStatusSucceeded, SaaSTenantDomainDeliveryJobStatusFailed, SaaSTenantDomainDeliveryJobStatusCanceled:
		return true
	default:
		return false
	}
}

func normalizeSaaSTenantDomainDeliveryBridgeUpdate(update SaaSTenantDomainDeliveryBridgeUpdate) (SaaSTenantDomainDeliveryBridgeUpdate, error) {
	update.Status = strings.ToLower(strings.TrimSpace(update.Status))
	update.Provider = truncateTenantDomainDeliveryText(update.Provider, 64)
	update.ProviderRequestID = truncateTenantDomainDeliveryText(update.ProviderRequestID, 128)
	update.RoutingStatus = strings.ToLower(strings.TrimSpace(update.RoutingStatus))
	update.CertificateStatus = strings.ToLower(strings.TrimSpace(update.CertificateStatus))
	update.CertificateID = truncateTenantDomainDeliveryText(update.CertificateID, 191)
	update.Error = truncateTenantDomainDeliveryText(update.Error, 500)
	update.OccurredAt = strings.TrimSpace(update.OccurredAt)
	if update.Status == "" {
		update.Status = SaaSTenantDomainDeliveryBridgeStatusAccepted
	}
	if update.Status != SaaSTenantDomainDeliveryBridgeStatusAccepted && update.Status != SaaSTenantDomainDeliveryBridgeStatusReady && update.Status != SaaSTenantDomainDeliveryBridgeStatusFailed {
		return update, errors.New("Bridge status 无效")
	}
	if update.RoutingStatus == "" {
		update.RoutingStatus = SaaSTenantDomainRoutingStatusProvisioning
	}
	if update.CertificateStatus == "" {
		update.CertificateStatus = SaaSTenantDomainCertificateStatusProvisioning
	}
	if !saasTenantDomainRoutingStatusValid(update.RoutingStatus) {
		return update, errors.New("routingStatus 无效")
	}
	if !saasTenantDomainCertificateStatusValid(update.CertificateStatus) {
		return update, errors.New("certificateStatus 无效")
	}
	for name, value := range map[string]string{"certificateNotBefore": update.CertificateNotBefore, "certificateExpiresAt": update.CertificateExpiresAt, "occurredAt": update.OccurredAt} {
		if value == "" {
			continue
		}
		if _, err := parseSaaSTenantDomainDeliveryTime(value); err != nil {
			return update, fmt.Errorf("%s 无效: %w", name, err)
		}
	}
	if update.Status == SaaSTenantDomainDeliveryBridgeStatusReady && (update.RoutingStatus != SaaSTenantDomainRoutingStatusReady || (update.CertificateStatus != SaaSTenantDomainCertificateStatusActive && update.CertificateStatus != SaaSTenantDomainCertificateStatusExpiring)) {
		return update, errors.New("ready 状态要求 routingStatus=ready 且证书为 active 或 expiring")
	}
	if update.Status == SaaSTenantDomainDeliveryBridgeStatusFailed && update.Error == "" {
		update.Error = "域名交付 Bridge 返回失败"
	}
	return update, nil
}

func saasTenantDomainDeliveryStatus(update SaaSTenantDomainDeliveryBridgeUpdate) string {
	if update.RoutingStatus == SaaSTenantDomainRoutingStatusDeleted || update.CertificateStatus == SaaSTenantDomainCertificateStatusDeleted {
		return SaaSTenantDomainDeliveryStatusDeleted
	}
	if update.RoutingStatus == SaaSTenantDomainRoutingStatusDisabled || update.CertificateStatus == SaaSTenantDomainCertificateStatusDisabled || update.CertificateStatus == SaaSTenantDomainCertificateStatusRevoked {
		return SaaSTenantDomainDeliveryStatusDisabled
	}
	if update.Status == SaaSTenantDomainDeliveryBridgeStatusFailed || update.RoutingStatus == SaaSTenantDomainRoutingStatusFailed || update.CertificateStatus == SaaSTenantDomainCertificateStatusFailed {
		return SaaSTenantDomainDeliveryStatusFailed
	}
	if update.RoutingStatus == SaaSTenantDomainRoutingStatusReady && update.CertificateStatus == SaaSTenantDomainCertificateStatusActive {
		return SaaSTenantDomainDeliveryStatusReady
	}
	if update.RoutingStatus == SaaSTenantDomainRoutingStatusReady && update.CertificateStatus == SaaSTenantDomainCertificateStatusExpiring {
		return SaaSTenantDomainDeliveryStatusDegraded
	}
	return SaaSTenantDomainDeliveryStatusProvisioning
}

func saasTenantDomainRoutingStatusValid(value string) bool {
	switch value {
	case SaaSTenantDomainRoutingStatusPending, SaaSTenantDomainRoutingStatusProvisioning, SaaSTenantDomainRoutingStatusReady, SaaSTenantDomainRoutingStatusFailed, SaaSTenantDomainRoutingStatusDisabled, SaaSTenantDomainRoutingStatusDeleted:
		return true
	default:
		return false
	}
}

func saasTenantDomainCertificateStatusValid(value string) bool {
	switch value {
	case SaaSTenantDomainCertificateStatusPending, SaaSTenantDomainCertificateStatusProvisioning, SaaSTenantDomainCertificateStatusActive, SaaSTenantDomainCertificateStatusExpiring, SaaSTenantDomainCertificateStatusFailed, SaaSTenantDomainCertificateStatusRevoked, SaaSTenantDomainCertificateStatusDisabled, SaaSTenantDomainCertificateStatusDeleted:
		return true
	default:
		return false
	}
}

func parseSaaSTenantDomainDeliveryTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("必须是 RFC3339 或 YYYY-MM-DD HH:MM:SS")
}

func truncateTenantDomainDeliveryText(value string, max int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > max {
		value = string(runes[:max])
	}
	return value
}

func readTenantDomainDeliveryBody(r *http.Request, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, errors.New("request body too large")
	}
	return body, nil
}
