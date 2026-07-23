package dashboard

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

func (h *SaaSAdminHandler) WithPaymentSettlementSyncService(service *SaaSPaymentSettlementSyncService) *SaaSAdminHandler {
	if h != nil {
		h.paymentSettlementSync = service
	}
	return h
}

func (h *SaaSAdminHandler) PaymentSettlementSyncRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.paymentSettlementSyncStore(w)
	if !ok {
		return
	}
	options, err := parseSaaSPaymentSettlementSyncRunOptions(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	report, err := store.SaaSAdminPaymentSettlementSyncRuns(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	providers := []string{}
	enabled := h.paymentSettlementSync != nil
	if enabled {
		providers = h.paymentSettlementSync.ConfiguredProviders()
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasPaymentSettlementSyncReportPayload(report, providers, enabled))
}

func (h *SaaSAdminHandler) PaymentSettlementSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	if h.paymentSettlementSync == nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "payment settlement bridge is not configured", nil)
		return
	}
	var request struct {
		Provider string `json:"provider"`
		DryRun   bool   `json:"dryRun"`
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4097))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if len(body) == 0 || len(body) > 4096 || json.Unmarshal(body, &request) != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "JSON 格式错误", nil)
		return
	}
	request.Provider = strings.ToLower(strings.TrimSpace(request.Provider))
	if request.Provider == "" {
		providers := h.paymentSettlementSync.ConfiguredProviders()
		if len(providers) == 1 {
			request.Provider = providers[0]
		}
	}
	result, err := h.paymentSettlementSync.Run(r.Context(), request.Provider, request.DryRun, SaaSPaymentSettlementSyncSourceManual, user.ID, user.TenantID)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"run": saasPaymentSettlementSyncRunPayload(result)})
}

func (h *SaaSAdminHandler) paymentSettlementSyncStore(w http.ResponseWriter) (SaaSPaymentSettlementSyncStore, bool) {
	store, ok := h.store.(SaaSPaymentSettlementSyncStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "payment settlement sync store is not configured", nil)
		return nil, false
	}
	return store, true
}

func parseSaaSPaymentSettlementSyncRunOptions(r *http.Request) (SaaSPaymentSettlementSyncRunOptions, error) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = "all"
	}
	if status != "all" && status != SaaSPaymentSettlementSyncStatusRunning && status != SaaSPaymentSettlementSyncStatusSucceeded && status != SaaSPaymentSettlementSyncStatusPreviewed && status != SaaSPaymentSettlementSyncStatusFailed {
		return SaaSPaymentSettlementSyncRunOptions{}, errors.New("status 必须是 all、running、succeeded、previewed 或 failed")
	}
	source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	if source == "" {
		source = "all"
	}
	if source != "all" && source != SaaSPaymentSettlementSyncSourceCron && source != SaaSPaymentSettlementSyncSourceManual {
		return SaaSPaymentSettlementSyncRunOptions{}, errors.New("source 必须是 all、cron 或 manual")
	}
	provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	if provider != "" && (!saasPaymentIdentifierPattern.MatchString(provider) || len(provider) > 32) {
		return SaaSPaymentSettlementSyncRunOptions{}, errors.New("provider 格式错误")
	}
	limit := positiveQueryInt(r, "limit", 100)
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	return SaaSPaymentSettlementSyncRunOptions{Provider: provider, Source: source, Status: status, Limit: limit}, nil
}

func saasPaymentSettlementSyncReportPayload(report SaaSPaymentSettlementSyncReport, providers []string, enabled bool) map[string]any {
	states := make([]map[string]any, 0, len(report.States))
	for _, state := range report.States {
		states = append(states, saasPaymentSettlementSyncStatePayload(state))
	}
	runs := make([]map[string]any, 0, len(report.Runs))
	for _, run := range report.Runs {
		runs = append(runs, saasPaymentSettlementSyncRunPayload(run))
	}
	return map[string]any{
		"enabled": enabled, "configuredProviders": providers,
		"options": map[string]any{"provider": report.Options.Provider, "source": report.Options.Source, "status": report.Options.Status, "limit": report.Options.Limit},
		"summary": map[string]any{
			"providerCount": report.Summary.ProviderCount, "activeCount": report.Summary.ActiveCount,
			"runCount": report.Summary.RunCount, "succeededCount": report.Summary.SucceededCount,
			"previewedCount": report.Summary.PreviewedCount, "failedCount": report.Summary.FailedCount,
			"importedCount": report.Summary.ImportedCount, "idempotentCount": report.Summary.IdempotentCount,
			"entryCount": report.Summary.EntryCount, "issueCount": report.Summary.IssueCount, "openIssueCount": report.Summary.OpenIssueCount,
		},
		"states": states, "runs": runs,
	}
}

func saasPaymentSettlementSyncStatePayload(state SaaSPaymentSettlementSyncState) map[string]any {
	return map[string]any{
		"id": state.ID, "provider": state.Provider, "cursor": state.Cursor, "activeRunId": state.ActiveRunID,
		"lastAttemptAt": state.LastAttemptAt, "lastSuccessAt": state.LastSuccessAt, "lastError": state.LastError,
		"version": state.Version, "createdAt": state.CreatedAt, "updatedAt": state.UpdatedAt,
	}
}

func saasPaymentSettlementSyncRunPayload(run SaaSPaymentSettlementSyncRun) map[string]any {
	return map[string]any{
		"id": run.ID, "runNo": run.RunNo, "provider": run.Provider, "source": run.Source, "status": run.Status, "dryRun": run.DryRun,
		"cursorBefore": run.CursorBefore, "cursorAfter": run.CursorAfter,
		"fetchedPageCount": run.FetchedPageCount, "fetchedBatchCount": run.FetchedBatchCount,
		"importedBatchCount": run.ImportedBatchCount, "idempotentBatchCount": run.IdempotentBatchCount,
		"entryCount": run.EntryCount, "issueCount": run.IssueCount, "openIssueCount": run.OpenIssueCount,
		"startedAt": run.StartedAt, "finishedAt": run.FinishedAt, "errorMessage": run.ErrorMessage,
		"actorUserId": run.ActorUserID, "actorTenantId": run.ActorTenantID, "operationId": run.OperationID,
		"createdAt": run.CreatedAt, "updatedAt": run.UpdatedAt,
	}
}
