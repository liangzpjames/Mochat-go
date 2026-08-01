package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	SaaSAdminExpectedMigrationVersion = "0106_work_message_global_search_indexes"
	SaaSAdminExpectedMigrationCount   = 106

	SaaSAdminSystemHealthStateHealthy  = "healthy"
	SaaSAdminSystemHealthStateWarning  = "warning"
	SaaSAdminSystemHealthStateCritical = "critical"

	SaaSAdminSystemIncidentStatusAll          = "all"
	SaaSAdminSystemIncidentStatusActive       = "active"
	SaaSAdminSystemIncidentStatusOpen         = "open"
	SaaSAdminSystemIncidentStatusAcknowledged = "acknowledged"
	SaaSAdminSystemIncidentStatusResolved     = "resolved"

	SaaSAdminSystemIncidentActionAcknowledge = "acknowledge"
	SaaSAdminSystemIncidentActionResolve     = "resolve"
	SaaSAdminSystemIncidentActionReopen      = "reopen"
	SaaSAdminSystemIncidentActionAssign      = "assign"

	SaaSAdminSystemHealthTriggerManual = "manual"
	SaaSAdminSystemHealthTriggerCron   = "cron"

	SaaSAdminOperationActionSystemHealthScan     = "saas.admin.system_health.scan"
	SaaSAdminOperationActionSystemIncidentUpdate = "saas.admin.system_incident.update"
	SaaSAdminOperationTargetSystemHealthScan     = "saas_admin_health_scan"
	SaaSAdminOperationTargetSystemIncident       = "saas_admin_system_incident"
)

type SaaSAdminSystemHealthOptions struct {
	FailureWindowHours    int `json:"failureWindowHours"`
	NotificationStaleMins int `json:"notificationStaleMinutes"`
}

type SaaSAdminSystemHealthCheck struct {
	Code      string
	Name      string
	Category  string
	Status    string
	Severity  string
	Current   int64
	Threshold int64
	Detail    string
	Metadata  map[string]any
}

type SaaSAdminSystemHealthSummary struct {
	HealthState   string
	CheckCount    int
	HealthyCount  int
	WarningCount  int
	CriticalCount int
	IssueCount    int
}

type SaaSAdminSystemHealthScan struct {
	ID                       int64
	ScanNo                   string
	TriggerType              string
	Status                   string
	HealthState              string
	CheckCount               int
	IssueCount               int
	CriticalCount            int
	WarningCount             int
	OpenedCount              int
	ReopenedCount            int
	RecoveredCount           int
	NotificationCount        int
	FailureWindowHours       int
	NotificationStaleMinutes int
	ActorUserID              int
	ActorTenantID            int
	StartedAt                string
	FinishedAt               string
	SnapshotJSON             string
	ErrorMessage             string
	OperationID              int64
	CreatedAt                string
}

type SaaSAdminSystemIncident struct {
	ID              int64
	IncidentKey     string
	Source          string
	Category        string
	Severity        string
	Status          string
	Title           string
	Detail          string
	CurrentValue    int64
	ThresholdValue  int64
	OccurrenceCount int64
	FirstDetectedAt string
	LastDetectedAt  string
	LastScanID      int64
	AcknowledgedAt  string
	AcknowledgedBy  int
	ResolvedAt      string
	ResolvedBy      int
	Owner           string
	ResolutionNote  string
	MetadataJSON    string
	Version         int
	CreatedAt       string
	UpdatedAt       string
}

type SaaSAdminSystemIncidentOptions struct {
	Status   string
	Severity string
	Source   string
	Owner    string
	Keyword  string
	Limit    int
}

type SaaSAdminSystemHealthScanInput struct {
	ScanNo           string
	TriggerType      string
	Options          SaaSAdminSystemHealthOptions
	Checks           []SaaSAdminSystemHealthCheck
	Summary          SaaSAdminSystemHealthSummary
	Notify           bool
	MaxAttempts      int
	PlatformTenantID int
	ActorUserID      int
	ActorTenantID    int
	StartedAt        time.Time
	FinishedAt       time.Time
}

type SaaSAdminSystemHealthScanResult struct {
	Scan             SaaSAdminSystemHealthScan
	Incidents        []SaaSAdminSystemIncident
	OpenedCount      int
	ReopenedCount    int
	RecoveredCount   int
	NotificationKeys []string
}

type SaaSAdminSystemIncidentUpdate struct {
	IncidentID      int64  `json:"incidentId"`
	Action          string `json:"action"`
	ExpectedVersion int    `json:"expectedVersion"`
	Owner           string `json:"owner"`
	Note            string `json:"note"`
	ActorUserID     int    `json:"-"`
	ActorTenantID   int    `json:"-"`
}

type SaaSAdminSystemIncidentUpdateResult struct {
	Incident    SaaSAdminSystemIncident
	OperationID int64
}

type SaaSAdminSystemHealthStore interface {
	SaaSAdminSystemHealthChecks(ctx context.Context, options SaaSAdminSystemHealthOptions) ([]SaaSAdminSystemHealthCheck, error)
	PersistSaaSAdminSystemHealthScan(ctx context.Context, input SaaSAdminSystemHealthScanInput) (SaaSAdminSystemHealthScanResult, error)
	SaaSAdminSystemHealthScans(ctx context.Context, limit int) ([]SaaSAdminSystemHealthScan, error)
	SaaSAdminSystemIncidents(ctx context.Context, options SaaSAdminSystemIncidentOptions) ([]SaaSAdminSystemIncident, error)
	UpdateSaaSAdminSystemIncident(ctx context.Context, input SaaSAdminSystemIncidentUpdate) (SaaSAdminSystemIncidentUpdateResult, error)
}

type SaaSAdminSystemHealthProbe struct {
	Code     string
	Name     string
	Category string
	Severity string
	Check    func(context.Context) error
}

func (h *SaaSAdminHandler) WithSystemHealthProbe(probe SaaSAdminSystemHealthProbe) *SaaSAdminHandler {
	probe.Code = strings.TrimSpace(probe.Code)
	probe.Name = strings.TrimSpace(probe.Name)
	probe.Category = strings.TrimSpace(probe.Category)
	if probe.Code != "" && probe.Name != "" && probe.Check != nil {
		if probe.Category == "" {
			probe.Category = "runtime"
		}
		if probe.Severity != SaaSAdminSystemHealthStateWarning && probe.Severity != SaaSAdminSystemHealthStateCritical {
			probe.Severity = SaaSAdminSystemHealthStateCritical
		}
		h.systemHealthProbes = append(h.systemHealthProbes, probe)
	}
	return h
}

func (h *SaaSAdminHandler) WithSystemHealthNotificationMaxAttempts(maxAttempts int) *SaaSAdminHandler {
	if maxAttempts > 0 && maxAttempts <= 20 {
		h.systemHealthNotificationMaxAttempts = maxAttempts
	}
	return h
}

func (h *SaaSAdminHandler) SystemHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminSystemHealthStore(w)
	if !ok {
		return
	}
	options, err := saasAdminSystemHealthOptionsFromQuery(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	checks, err := h.collectSaaSAdminSystemHealthChecks(r.Context(), store, options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	incidents, err := store.SaaSAdminSystemIncidents(r.Context(), SaaSAdminSystemIncidentOptions{Status: SaaSAdminSystemIncidentStatusActive, Severity: SaaSAdminSystemIncidentStatusAll, Limit: 100})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	scans, err := store.SaaSAdminSystemHealthScans(r.Context(), 10)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{
		"summary":   saasAdminSystemHealthSummaryPayload(summarizeSaaSAdminSystemHealth(checks)),
		"options":   map[string]any{"failureWindowHours": options.FailureWindowHours, "notificationStaleMinutes": options.NotificationStaleMins},
		"checks":    saasAdminSystemHealthChecksPayload(checks),
		"incidents": saasAdminSystemIncidentsPayload(incidents),
		"scans":     saasAdminSystemHealthScansPayload(scans),
	})
}

func (h *SaaSAdminHandler) SystemHealthScans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminSystemHealthStore(w)
	if !ok {
		return
	}
	limit := positiveQueryInt(r, "limit", 30)
	if limit > 500 {
		limit = 500
	}
	items, err := store.SaaSAdminSystemHealthScans(r.Context(), limit)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{"items": saasAdminSystemHealthScansPayload(items), "returnedCount": len(items)})
}

func (h *SaaSAdminHandler) SystemIncidents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminSystemHealthStore(w)
	if !ok {
		return
	}
	options, err := saasAdminSystemIncidentOptionsFromQuery(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	items, err := store.SaaSAdminSystemIncidents(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{"items": saasAdminSystemIncidentsPayload(items), "returnedCount": len(items)})
}

func (h *SaaSAdminHandler) SystemHealthScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	var request struct {
		FailureWindowHours    int   `json:"failureWindowHours"`
		NotificationStaleMins int   `json:"notificationStaleMinutes"`
		Notify                *bool `json:"notify"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid JSON body", nil)
		return
	}
	options, err := normalizeSaaSAdminSystemHealthOptions(SaaSAdminSystemHealthOptions{FailureWindowHours: request.FailureWindowHours, NotificationStaleMins: request.NotificationStaleMins})
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	notify := true
	if request.Notify != nil {
		notify = *request.Notify
	}
	result, err := h.RunSaaSAdminSystemHealthScan(r.Context(), SaaSAdminSystemHealthScanInput{
		TriggerType: SaaSAdminSystemHealthTriggerManual, Options: options, Notify: notify,
		ActorUserID: user.ID, ActorTenantID: user.TenantID, PlatformTenantID: h.platformAdminTenantID,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", saasAdminSystemHealthScanResultPayload(result))
}

func (h *SaaSAdminHandler) SystemIncident(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminSystemHealthStore(w)
	if !ok {
		return
	}
	var input SaaSAdminSystemIncidentUpdate
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid JSON body", nil)
		return
	}
	if err := validateSaaSAdminSystemIncidentUpdate(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input.ActorUserID = user.ID
	input.ActorTenantID = user.TenantID
	result, err := store.UpdateSaaSAdminSystemIncident(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{"incident": saasAdminSystemIncidentPayload(result.Incident), "operationId": result.OperationID})
}

func (h *SaaSAdminHandler) RunSaaSAdminSystemHealthScan(ctx context.Context, input SaaSAdminSystemHealthScanInput) (SaaSAdminSystemHealthScanResult, error) {
	store, ok := h.store.(SaaSAdminSystemHealthStore)
	if !ok || store == nil {
		return SaaSAdminSystemHealthScanResult{}, fmt.Errorf("SaaS system health store is not configured")
	}
	options, err := normalizeSaaSAdminSystemHealthOptions(input.Options)
	if err != nil {
		return SaaSAdminSystemHealthScanResult{}, err
	}
	startedAt := time.Now()
	checks, err := h.collectSaaSAdminSystemHealthChecks(ctx, store, options)
	if err != nil {
		return SaaSAdminSystemHealthScanResult{}, err
	}
	if input.ScanNo == "" {
		input.ScanNo, err = newSaaSAdminSystemHealthScanNo()
		if err != nil {
			return SaaSAdminSystemHealthScanResult{}, err
		}
	}
	if input.TriggerType != SaaSAdminSystemHealthTriggerCron {
		input.TriggerType = SaaSAdminSystemHealthTriggerManual
	}
	if input.PlatformTenantID <= 0 {
		input.PlatformTenantID = h.platformAdminTenantID
	}
	if input.MaxAttempts <= 0 || input.MaxAttempts > 20 {
		input.MaxAttempts = h.systemHealthNotificationMaxAttempts
	}
	if input.MaxAttempts <= 0 || input.MaxAttempts > 20 {
		input.MaxAttempts = 3
	}
	input.Options = options
	input.Checks = checks
	input.Summary = summarizeSaaSAdminSystemHealth(checks)
	input.StartedAt = startedAt
	input.FinishedAt = time.Now()
	return store.PersistSaaSAdminSystemHealthScan(ctx, input)
}

func (h *SaaSAdminHandler) collectSaaSAdminSystemHealthChecks(ctx context.Context, store SaaSAdminSystemHealthStore, options SaaSAdminSystemHealthOptions) ([]SaaSAdminSystemHealthCheck, error) {
	checks, err := store.SaaSAdminSystemHealthChecks(ctx, options)
	if err != nil {
		return nil, err
	}
	for _, probe := range h.systemHealthProbes {
		check := SaaSAdminSystemHealthCheck{Code: probe.Code, Name: probe.Name, Category: probe.Category, Status: SaaSAdminSystemHealthStateHealthy, Severity: probe.Severity, Detail: "连接正常"}
		if err := probe.Check(ctx); err != nil {
			check.Status = probe.Severity
			check.Current = 1
			check.Detail = truncateSystemHealthText(err.Error(), 500)
		}
		checks = append(checks, check)
	}
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].Category == checks[j].Category {
			return checks[i].Code < checks[j].Code
		}
		return checks[i].Category < checks[j].Category
	})
	return checks, nil
}

func (h *SaaSAdminHandler) saasAdminSystemHealthStore(w http.ResponseWriter) (SaaSAdminSystemHealthStore, bool) {
	store, ok := h.store.(SaaSAdminSystemHealthStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "SaaS system health store is not configured", nil)
		return nil, false
	}
	return store, true
}

func normalizeSaaSAdminSystemHealthOptions(options SaaSAdminSystemHealthOptions) (SaaSAdminSystemHealthOptions, error) {
	if options.FailureWindowHours == 0 {
		options.FailureWindowHours = 24
	}
	if options.NotificationStaleMins == 0 {
		options.NotificationStaleMins = 15
	}
	if options.FailureWindowHours < 1 || options.FailureWindowHours > 720 {
		return SaaSAdminSystemHealthOptions{}, errors.New("failureWindowHours 必须在 1 至 720 之间")
	}
	if options.NotificationStaleMins < 1 || options.NotificationStaleMins > 10080 {
		return SaaSAdminSystemHealthOptions{}, errors.New("notificationStaleMinutes 必须在 1 至 10080 之间")
	}
	return options, nil
}

func saasAdminSystemHealthOptionsFromQuery(r *http.Request) (SaaSAdminSystemHealthOptions, error) {
	window, err := optionalPositiveQueryInt(r, "failureWindowHours", 24)
	if err != nil {
		return SaaSAdminSystemHealthOptions{}, errors.New("failureWindowHours 无效")
	}
	stale, err := optionalPositiveQueryInt(r, "notificationStaleMinutes", 15)
	if err != nil {
		return SaaSAdminSystemHealthOptions{}, errors.New("notificationStaleMinutes 无效")
	}
	return normalizeSaaSAdminSystemHealthOptions(SaaSAdminSystemHealthOptions{FailureWindowHours: window, NotificationStaleMins: stale})
}

func optionalPositiveQueryInt(r *http.Request, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid positive integer")
	}
	return value, nil
}

func saasAdminSystemIncidentOptionsFromQuery(r *http.Request) (SaaSAdminSystemIncidentOptions, error) {
	options := SaaSAdminSystemIncidentOptions{
		Status: strings.TrimSpace(r.URL.Query().Get("status")), Severity: strings.TrimSpace(r.URL.Query().Get("severity")),
		Source: strings.TrimSpace(r.URL.Query().Get("source")), Owner: strings.TrimSpace(r.URL.Query().Get("owner")),
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Limit: positiveQueryInt(r, "limit", 100),
	}
	if options.Status == "" {
		options.Status = SaaSAdminSystemIncidentStatusActive
	}
	if options.Severity == "" {
		options.Severity = SaaSAdminSystemIncidentStatusAll
	}
	if options.Status != SaaSAdminSystemIncidentStatusAll && options.Status != SaaSAdminSystemIncidentStatusActive && options.Status != SaaSAdminSystemIncidentStatusOpen && options.Status != SaaSAdminSystemIncidentStatusAcknowledged && options.Status != SaaSAdminSystemIncidentStatusResolved {
		return SaaSAdminSystemIncidentOptions{}, errors.New("status 必须是 all、active、open、acknowledged 或 resolved")
	}
	if options.Severity != SaaSAdminSystemIncidentStatusAll && options.Severity != SaaSAdminSystemHealthStateWarning && options.Severity != SaaSAdminSystemHealthStateCritical {
		return SaaSAdminSystemIncidentOptions{}, errors.New("severity 必须是 all、warning 或 critical")
	}
	if len([]rune(options.Source)) > 64 || len([]rune(options.Owner)) > 80 || len([]rune(options.Keyword)) > 80 {
		return SaaSAdminSystemIncidentOptions{}, errors.New("事故筛选字段过长")
	}
	if options.Limit > 500 {
		options.Limit = 500
	}
	return options, nil
}

func validateSaaSAdminSystemIncidentUpdate(input *SaaSAdminSystemIncidentUpdate) error {
	input.Action = strings.TrimSpace(input.Action)
	input.Owner = strings.TrimSpace(input.Owner)
	input.Note = strings.TrimSpace(input.Note)
	if input.IncidentID <= 0 || input.ExpectedVersion <= 0 {
		return errors.New("incidentId 和 expectedVersion 必须大于 0")
	}
	if input.Action != SaaSAdminSystemIncidentActionAcknowledge && input.Action != SaaSAdminSystemIncidentActionResolve && input.Action != SaaSAdminSystemIncidentActionReopen && input.Action != SaaSAdminSystemIncidentActionAssign {
		return errors.New("action 必须是 acknowledge、resolve、reopen 或 assign")
	}
	if len([]rune(input.Owner)) > 80 || len([]rune(input.Note)) > 255 {
		return errors.New("owner 最多 80 个字符，note 最多 255 个字符")
	}
	if (input.Action == SaaSAdminSystemIncidentActionAcknowledge || input.Action == SaaSAdminSystemIncidentActionAssign) && input.Owner == "" {
		return errors.New("认领或分派事故时 owner 必填")
	}
	if (input.Action == SaaSAdminSystemIncidentActionResolve || input.Action == SaaSAdminSystemIncidentActionReopen) && input.Note == "" {
		return errors.New("解决或重开事故时 note 必填")
	}
	return nil
}

func summarizeSaaSAdminSystemHealth(checks []SaaSAdminSystemHealthCheck) SaaSAdminSystemHealthSummary {
	summary := SaaSAdminSystemHealthSummary{HealthState: SaaSAdminSystemHealthStateHealthy, CheckCount: len(checks)}
	for _, check := range checks {
		switch check.Status {
		case SaaSAdminSystemHealthStateCritical:
			summary.CriticalCount++
			summary.IssueCount++
		case SaaSAdminSystemHealthStateWarning:
			summary.WarningCount++
			summary.IssueCount++
		default:
			summary.HealthyCount++
		}
	}
	if summary.CriticalCount > 0 {
		summary.HealthState = SaaSAdminSystemHealthStateCritical
	} else if summary.WarningCount > 0 {
		summary.HealthState = SaaSAdminSystemHealthStateWarning
	}
	return summary
}

func newSaaSAdminSystemHealthScanNo() (string, error) {
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("HSC-%s-%s", time.Now().UTC().Format("20060102T150405"), strings.ToUpper(hex.EncodeToString(random[:]))), nil
}

func truncateSystemHealthText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func saasAdminSystemHealthSummaryPayload(summary SaaSAdminSystemHealthSummary) map[string]any {
	return map[string]any{"healthState": summary.HealthState, "checkCount": summary.CheckCount, "healthyCount": summary.HealthyCount, "warningCount": summary.WarningCount, "criticalCount": summary.CriticalCount, "issueCount": summary.IssueCount}
}

func saasAdminSystemHealthChecksPayload(items []SaaSAdminSystemHealthCheck) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		metadata := item.Metadata
		if metadata == nil {
			metadata = map[string]any{}
		}
		result = append(result, map[string]any{"code": item.Code, "name": item.Name, "category": item.Category, "status": item.Status, "severity": item.Severity, "current": item.Current, "threshold": item.Threshold, "detail": item.Detail, "metadata": metadata})
	}
	return result
}

func saasAdminSystemHealthScanPayload(item SaaSAdminSystemHealthScan) map[string]any {
	return map[string]any{
		"id": item.ID, "scanNo": item.ScanNo, "triggerType": item.TriggerType, "status": item.Status, "healthState": item.HealthState,
		"checkCount": item.CheckCount, "issueCount": item.IssueCount, "criticalCount": item.CriticalCount, "warningCount": item.WarningCount,
		"openedCount": item.OpenedCount, "reopenedCount": item.ReopenedCount, "recoveredCount": item.RecoveredCount, "notificationCount": item.NotificationCount,
		"failureWindowHours": item.FailureWindowHours, "notificationStaleMinutes": item.NotificationStaleMinutes,
		"actorUserId": item.ActorUserID, "actorTenantId": item.ActorTenantID, "startedAt": item.StartedAt, "finishedAt": item.FinishedAt,
		"errorMessage": item.ErrorMessage, "operationId": item.OperationID, "createdAt": item.CreatedAt,
	}
}

func saasAdminSystemHealthScansPayload(items []SaaSAdminSystemHealthScan) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasAdminSystemHealthScanPayload(item))
	}
	return result
}

func saasAdminSystemIncidentPayload(item SaaSAdminSystemIncident) map[string]any {
	metadata := map[string]any{}
	if strings.TrimSpace(item.MetadataJSON) != "" {
		_ = json.Unmarshal([]byte(item.MetadataJSON), &metadata)
	}
	return map[string]any{
		"id": item.ID, "incidentKey": item.IncidentKey, "source": item.Source, "category": item.Category, "severity": item.Severity,
		"status": item.Status, "title": item.Title, "detail": item.Detail, "currentValue": item.CurrentValue, "thresholdValue": item.ThresholdValue,
		"occurrenceCount": item.OccurrenceCount, "firstDetectedAt": item.FirstDetectedAt, "lastDetectedAt": item.LastDetectedAt, "lastScanId": item.LastScanID,
		"acknowledgedAt": item.AcknowledgedAt, "acknowledgedBy": item.AcknowledgedBy, "resolvedAt": item.ResolvedAt, "resolvedBy": item.ResolvedBy,
		"owner": item.Owner, "resolutionNote": item.ResolutionNote, "metadata": metadata, "version": item.Version,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasAdminSystemIncidentsPayload(items []SaaSAdminSystemIncident) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasAdminSystemIncidentPayload(item))
	}
	return result
}

func saasAdminSystemHealthScanResultPayload(result SaaSAdminSystemHealthScanResult) map[string]any {
	return map[string]any{
		"scan": saasAdminSystemHealthScanPayload(result.Scan), "incidents": saasAdminSystemIncidentsPayload(result.Incidents),
		"openedCount": result.OpenedCount, "reopenedCount": result.ReopenedCount, "recoveredCount": result.RecoveredCount,
		"notificationCount": len(result.NotificationKeys), "notificationKeys": result.NotificationKeys,
	}
}
