package dashboard

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	SaaSPaymentSettlementSyncCronTaskName = "cron-saas-payment-settlement-sync"

	SaaSPaymentSettlementSyncSourceCron   = "cron"
	SaaSPaymentSettlementSyncSourceManual = "manual"

	SaaSPaymentSettlementSyncStatusRunning   = "running"
	SaaSPaymentSettlementSyncStatusSucceeded = "succeeded"
	SaaSPaymentSettlementSyncStatusPreviewed = "previewed"
	SaaSPaymentSettlementSyncStatusFailed    = "failed"

	SaaSAdminOperationActionSettlementSync = "payment.settlement.sync"
	SaaSAdminOperationTargetSettlementSync = "payment_settlement_sync"

	SaaSAlertTypePaymentSettlementSyncFailed = "payment_settlement_sync_failed"
	SaaSAlertTypePaymentSettlementIssue      = "payment_settlement_issue"
	SaaSMetricPaymentSettlementSync          = "payment_settlement_sync"

	saasPaymentSettlementBridgePath             = "/v1/payment-settlements"
	saasPaymentSettlementBridgeMaxResponseBytes = 32 << 20
	saasPaymentSettlementSyncDefaultMaxPages    = 20
	saasPaymentSettlementSyncStaleAfterSeconds  = 15 * 60
)

type SaaSPaymentSettlementSyncState struct {
	ID            int64
	Provider      string
	Cursor        string
	ActiveRunID   int64
	LastAttemptAt string
	LastSuccessAt string
	LastError     string
	Version       int
	CreatedAt     string
	UpdatedAt     string
}

type SaaSPaymentSettlementSyncRun struct {
	ID                   int64
	RunNo                string
	Provider             string
	Source               string
	Status               string
	DryRun               bool
	CursorBefore         string
	CursorAfter          string
	FetchedPageCount     int
	FetchedBatchCount    int
	ImportedBatchCount   int
	IdempotentBatchCount int
	EntryCount           int
	IssueCount           int
	OpenIssueCount       int
	StartedAt            string
	FinishedAt           string
	ErrorMessage         string
	ActorUserID          int
	ActorTenantID        int
	OperationID          int64
	CreatedAt            string
	UpdatedAt            string
}

type SaaSPaymentSettlementSyncRunOptions struct {
	Provider string
	Source   string
	Status   string
	Limit    int
}

type SaaSPaymentSettlementSyncSummary struct {
	ProviderCount   int
	ActiveCount     int
	RunCount        int
	SucceededCount  int
	PreviewedCount  int
	FailedCount     int
	ImportedCount   int
	IdempotentCount int
	EntryCount      int
	IssueCount      int
	OpenIssueCount  int
}

type SaaSPaymentSettlementSyncReport struct {
	Options SaaSPaymentSettlementSyncRunOptions
	Summary SaaSPaymentSettlementSyncSummary
	States  []SaaSPaymentSettlementSyncState
	Runs    []SaaSPaymentSettlementSyncRun
}

type SaaSPaymentSettlementSyncBegin struct {
	RunNo             string
	Provider          string
	Source            string
	DryRun            bool
	ActorUserID       int
	ActorTenantID     int
	StaleAfterSeconds int
}

type SaaSPaymentSettlementSyncFinish struct {
	RunID                int64
	Provider             string
	Status               string
	DryRun               bool
	CursorAfter          string
	FetchedPageCount     int
	FetchedBatchCount    int
	ImportedBatchCount   int
	IdempotentBatchCount int
	EntryCount           int
	IssueCount           int
	OpenIssueCount       int
	ActorUserID          int
	ActorTenantID        int
}

type SaaSPaymentSettlementSyncFailure struct {
	RunID                int64
	Provider             string
	ErrorMessage         string
	FetchedPageCount     int
	FetchedBatchCount    int
	ImportedBatchCount   int
	IdempotentBatchCount int
	EntryCount           int
	IssueCount           int
	OpenIssueCount       int
	ActorUserID          int
	ActorTenantID        int
}

type SaaSPaymentSettlementSyncStore interface {
	BeginSaaSPaymentSettlementSync(context.Context, SaaSPaymentSettlementSyncBegin) (SaaSPaymentSettlementSyncRun, SaaSPaymentSettlementSyncState, error)
	FinishSaaSPaymentSettlementSync(context.Context, SaaSPaymentSettlementSyncFinish) (SaaSPaymentSettlementSyncRun, SaaSPaymentSettlementSyncState, error)
	FailSaaSPaymentSettlementSync(context.Context, SaaSPaymentSettlementSyncFailure) (SaaSPaymentSettlementSyncRun, SaaSPaymentSettlementSyncState, error)
	SaaSAdminPaymentSettlementSyncRuns(context.Context, SaaSPaymentSettlementSyncRunOptions) (SaaSPaymentSettlementSyncReport, error)
	ImportSaaSAdminPaymentSettlement(context.Context, SaaSPaymentSettlementImport) (SaaSPaymentSettlementImportResult, error)
}

type SaaSPaymentSettlementSyncAlertStore interface {
	EnqueueSaaSAlertNotification(context.Context, SaaSQuotaAlert, string, int) (SaaSAlertNotification, error)
}

type SaaSPaymentSettlementBridgePage struct {
	Provider   string                               `json:"provider"`
	NextCursor string                               `json:"nextCursor"`
	HasMore    bool                                 `json:"hasMore"`
	Batches    []saasPaymentSettlementImportRequest `json:"batches"`
}

type SaaSPaymentSettlementBridgeClient interface {
	FetchPaymentSettlements(context.Context, string, string, int) (SaaSPaymentSettlementBridgePage, error)
}

type SaaSPaymentSettlementHTTPBridgeClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewSaaSPaymentSettlementHTTPBridgeClient(baseURL, token string, timeout time.Duration) (*SaaSPaymentSettlementHTTPBridgeClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("payment settlement bridge base URL must be an absolute HTTP(S) URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("payment settlement bridge base URL must not include credentials, query, or fragment")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &SaaSPaymentSettlementHTTPBridgeClient{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		client:  &http.Client{Timeout: timeout},
	}, nil
}

func (c *SaaSPaymentSettlementHTTPBridgeClient) FetchPaymentSettlements(ctx context.Context, provider, cursor string, limit int) (SaaSPaymentSettlementBridgePage, error) {
	if c == nil || c.client == nil || strings.TrimSpace(c.baseURL) == "" {
		return SaaSPaymentSettlementBridgePage{}, errors.New("payment settlement bridge client is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	endpoint, err := url.Parse(c.baseURL + saasPaymentSettlementBridgePath)
	if err != nil {
		return SaaSPaymentSettlementBridgePage{}, err
	}
	query := endpoint.Query()
	query.Set("provider", strings.TrimSpace(provider))
	query.Set("cursor", strings.TrimSpace(cursor))
	query.Set("limit", fmt.Sprintf("%d", limit))
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return SaaSPaymentSettlementBridgePage{}, err
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return SaaSPaymentSettlementBridgePage{}, fmt.Errorf("payment settlement bridge request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, saasPaymentSettlementBridgeMaxResponseBytes+1))
	if err != nil {
		return SaaSPaymentSettlementBridgePage{}, fmt.Errorf("read payment settlement bridge response: %w", err)
	}
	if len(body) > saasPaymentSettlementBridgeMaxResponseBytes {
		return SaaSPaymentSettlementBridgePage{}, errors.New("payment settlement bridge response exceeds 32MB")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(body))
		if len(message) > 500 {
			message = message[:500]
		}
		return SaaSPaymentSettlementBridgePage{}, fmt.Errorf("payment settlement bridge returned HTTP %d: %s", resp.StatusCode, message)
	}
	var page SaaSPaymentSettlementBridgePage
	if err := json.Unmarshal(body, &page); err != nil {
		return SaaSPaymentSettlementBridgePage{}, fmt.Errorf("decode payment settlement bridge response: %w", err)
	}
	return page, nil
}

type SaaSPaymentSettlementSyncService struct {
	store                   SaaSPaymentSettlementSyncStore
	alertStore              SaaSPaymentSettlementSyncAlertStore
	client                  SaaSPaymentSettlementBridgeClient
	providers               []string
	providerSet             map[string]struct{}
	limit                   int
	maxPages                int
	platformTenantID        int
	notificationMaxAttempts int
	logger                  *log.Logger
}

func NewSaaSPaymentSettlementSyncService(
	store SaaSPaymentSettlementSyncStore,
	client SaaSPaymentSettlementBridgeClient,
	providers []string,
	limit int,
	platformTenantID int,
	notificationMaxAttempts int,
	logger *log.Logger,
) (*SaaSPaymentSettlementSyncService, error) {
	normalized, err := normalizeSaaSPaymentSettlementProviders(providers)
	if err != nil {
		return nil, err
	}
	if store == nil || client == nil || len(normalized) == 0 || platformTenantID <= 0 {
		return nil, errors.New("payment settlement sync dependencies are not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > saasPaymentSettlementImportMaxEntries {
		limit = saasPaymentSettlementImportMaxEntries
	}
	if notificationMaxAttempts <= 0 {
		notificationMaxAttempts = 3
	}
	if logger == nil {
		logger = log.Default()
	}
	providerSet := make(map[string]struct{}, len(normalized))
	for _, provider := range normalized {
		providerSet[provider] = struct{}{}
	}
	alertStore, _ := store.(SaaSPaymentSettlementSyncAlertStore)
	return &SaaSPaymentSettlementSyncService{
		store: store, alertStore: alertStore, client: client, providers: normalized, providerSet: providerSet,
		limit: limit, maxPages: saasPaymentSettlementSyncDefaultMaxPages, platformTenantID: platformTenantID,
		notificationMaxAttempts: notificationMaxAttempts, logger: logger,
	}, nil
}

func normalizeSaaSPaymentSettlementProviders(providers []string) ([]string, error) {
	seen := make(map[string]struct{}, len(providers))
	normalized := make([]string, 0, len(providers))
	for _, raw := range providers {
		provider := strings.ToLower(strings.TrimSpace(raw))
		if provider == "" {
			continue
		}
		if !saasPaymentIdentifierPattern.MatchString(provider) || len(provider) > 32 {
			return nil, fmt.Errorf("payment settlement provider %q has invalid format", raw)
		}
		if _, ok := seen[provider]; ok {
			continue
		}
		seen[provider] = struct{}{}
		normalized = append(normalized, provider)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func (s *SaaSPaymentSettlementSyncService) ConfiguredProviders() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.providers...)
}

func (s *SaaSPaymentSettlementSyncService) RunAll(ctx context.Context) error {
	if s == nil {
		return errors.New("payment settlement sync service is not configured")
	}
	var allErr error
	for _, provider := range s.providers {
		result, err := s.Run(ctx, provider, false, SaaSPaymentSettlementSyncSourceCron, 0, s.platformTenantID)
		if err != nil {
			allErr = errors.Join(allErr, fmt.Errorf("provider %s: %w", provider, err))
			continue
		}
		s.logger.Printf("SaaS payment settlement sync finished: provider=%s run=%s pages=%d batches=%d imported=%d idempotent=%d entries=%d issues=%d open=%d",
			provider, result.RunNo, result.FetchedPageCount, result.FetchedBatchCount, result.ImportedBatchCount,
			result.IdempotentBatchCount, result.EntryCount, result.IssueCount, result.OpenIssueCount)
	}
	return allErr
}

func (s *SaaSPaymentSettlementSyncService) Run(ctx context.Context, provider string, dryRun bool, source string, actorUserID, actorTenantID int) (SaaSPaymentSettlementSyncRun, error) {
	if s == nil || s.store == nil || s.client == nil {
		return SaaSPaymentSettlementSyncRun{}, errors.New("payment settlement sync service is not configured")
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if _, ok := s.providerSet[provider]; !ok {
		return SaaSPaymentSettlementSyncRun{}, NewSaaSAdminBadRequest("provider 未配置为可同步渠道")
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if source != SaaSPaymentSettlementSyncSourceCron && source != SaaSPaymentSettlementSyncSourceManual {
		return SaaSPaymentSettlementSyncRun{}, NewSaaSAdminBadRequest("source 必须是 cron 或 manual")
	}
	if actorTenantID <= 0 {
		actorTenantID = s.platformTenantID
	}
	runNo, err := newSaaSPaymentSettlementSyncRunNo()
	if err != nil {
		return SaaSPaymentSettlementSyncRun{}, err
	}
	run, _, err := s.store.BeginSaaSPaymentSettlementSync(ctx, SaaSPaymentSettlementSyncBegin{
		RunNo: runNo, Provider: provider, Source: source, DryRun: dryRun,
		ActorUserID: actorUserID, ActorTenantID: actorTenantID, StaleAfterSeconds: saasPaymentSettlementSyncStaleAfterSeconds,
	})
	if err != nil {
		return SaaSPaymentSettlementSyncRun{}, err
	}
	result := run
	cursor := run.CursorBefore
	for pageNo := 1; pageNo <= s.maxPages; pageNo++ {
		page, fetchErr := s.client.FetchPaymentSettlements(ctx, provider, cursor, s.limit)
		if fetchErr != nil {
			return s.fail(ctx, result, fetchErr)
		}
		pageProvider := strings.ToLower(strings.TrimSpace(page.Provider))
		if pageProvider != "" && pageProvider != provider {
			return s.fail(ctx, result, fmt.Errorf("bridge provider %q does not match requested provider %q", pageProvider, provider))
		}
		if len(page.Batches) > s.limit {
			return s.fail(ctx, result, fmt.Errorf("bridge returned %d batches, limit is %d", len(page.Batches), s.limit))
		}
		result.FetchedPageCount++
		for batchIndex, batchRequest := range page.Batches {
			canonical, marshalErr := json.Marshal(batchRequest)
			if marshalErr != nil {
				return s.fail(ctx, result, fmt.Errorf("marshal bridge batch %d: %w", batchIndex, marshalErr))
			}
			digest := sha256.Sum256(canonical)
			input, normalizeErr := normalizeSaaSPaymentSettlementImport(batchRequest, hex.EncodeToString(digest[:]))
			if normalizeErr != nil {
				return s.fail(ctx, result, fmt.Errorf("bridge batch %d invalid: %w", batchIndex, normalizeErr))
			}
			if input.Provider != provider {
				return s.fail(ctx, result, fmt.Errorf("bridge batch %d provider %q does not match requested provider %q", batchIndex, input.Provider, provider))
			}
			if input.BatchNo == "" {
				input.BatchNo, err = newSaaSPaymentSettlementBatchNo()
				if err != nil {
					return s.fail(ctx, result, err)
				}
			}
			input.ActorUserID = actorUserID
			input.ActorTenantID = actorTenantID
			result.FetchedBatchCount++
			result.EntryCount += len(input.Entries)
			if dryRun {
				continue
			}
			imported, importErr := s.store.ImportSaaSAdminPaymentSettlement(ctx, input)
			if importErr != nil {
				return s.fail(ctx, result, fmt.Errorf("import settlement %s: %w", input.ProviderSettlementNo, importErr))
			}
			if imported.Idempotent {
				result.IdempotentBatchCount++
			} else {
				result.ImportedBatchCount++
			}
			result.IssueCount += imported.Batch.IssueCount
			result.OpenIssueCount += imported.Batch.OpenIssueCount
		}
		nextCursor := strings.TrimSpace(page.NextCursor)
		if len(nextCursor) > 512 {
			return s.fail(ctx, result, errors.New("bridge nextCursor exceeds 512 characters"))
		}
		if page.HasMore {
			if nextCursor == "" || nextCursor == cursor {
				return s.fail(ctx, result, errors.New("bridge hasMore requires a new non-empty nextCursor"))
			}
			cursor = nextCursor
			if pageNo == s.maxPages {
				return s.fail(ctx, result, fmt.Errorf("bridge pagination exceeds %d pages", s.maxPages))
			}
			continue
		}
		if nextCursor != "" {
			cursor = nextCursor
		}
		break
	}
	status := SaaSPaymentSettlementSyncStatusSucceeded
	if dryRun {
		status = SaaSPaymentSettlementSyncStatusPreviewed
		cursor = run.CursorBefore
	}
	finished, _, err := s.store.FinishSaaSPaymentSettlementSync(ctx, SaaSPaymentSettlementSyncFinish{
		RunID: result.ID, Provider: provider, Status: status, DryRun: dryRun, CursorAfter: cursor,
		FetchedPageCount: result.FetchedPageCount, FetchedBatchCount: result.FetchedBatchCount,
		ImportedBatchCount: result.ImportedBatchCount, IdempotentBatchCount: result.IdempotentBatchCount,
		EntryCount: result.EntryCount, IssueCount: result.IssueCount, OpenIssueCount: result.OpenIssueCount,
		ActorUserID: actorUserID, ActorTenantID: actorTenantID,
	})
	if err != nil {
		return result, err
	}
	if !dryRun && finished.OpenIssueCount > 0 {
		s.enqueueAlert(ctx, finished, SaaSAlertTypePaymentSettlementIssue, SaaSAlertSeverityWarning,
			fmt.Sprintf("渠道 %s 本次结算同步产生 %d 条待处理差异", provider, finished.OpenIssueCount), "")
	}
	return finished, nil
}

func (s *SaaSPaymentSettlementSyncService) fail(ctx context.Context, run SaaSPaymentSettlementSyncRun, cause error) (SaaSPaymentSettlementSyncRun, error) {
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	failed, _, persistErr := s.store.FailSaaSPaymentSettlementSync(persistCtx, SaaSPaymentSettlementSyncFailure{
		RunID: run.ID, Provider: run.Provider, ErrorMessage: cause.Error(),
		FetchedPageCount: run.FetchedPageCount, FetchedBatchCount: run.FetchedBatchCount,
		ImportedBatchCount: run.ImportedBatchCount, IdempotentBatchCount: run.IdempotentBatchCount,
		EntryCount: run.EntryCount, IssueCount: run.IssueCount, OpenIssueCount: run.OpenIssueCount,
		ActorUserID: run.ActorUserID, ActorTenantID: run.ActorTenantID,
	})
	if persistErr != nil {
		return run, errors.Join(cause, fmt.Errorf("persist settlement sync failure: %w", persistErr))
	}
	s.enqueueAlert(persistCtx, failed, SaaSAlertTypePaymentSettlementSyncFailed, SaaSAlertSeverityCritical,
		fmt.Sprintf("渠道 %s 结算同步失败：%s", failed.Provider, failed.ErrorMessage), failed.ErrorMessage)
	return failed, cause
}

func (s *SaaSPaymentSettlementSyncService) enqueueAlert(ctx context.Context, run SaaSPaymentSettlementSyncRun, alertType, severity, message, errorMessage string) {
	if s == nil || s.alertStore == nil {
		return
	}
	current := int64(run.OpenIssueCount)
	if alertType == SaaSAlertTypePaymentSettlementSyncFailed {
		current = 1
	}
	_, err := s.alertStore.EnqueueSaaSAlertNotification(ctx, SaaSQuotaAlert{
		Status:    SaaSQuotaStatus{TenantID: s.platformTenantID, Metric: SaaSMetricPaymentSettlementSync, Current: current},
		AlertType: alertType, Severity: severity, PeriodKey: run.RunNo, Source: "payment-settlement-sync", Message: message,
		Context: map[string]any{
			"runNo": run.RunNo, "provider": run.Provider, "source": run.Source, "status": run.Status,
			"openIssueCount": run.OpenIssueCount, "error": errorMessage,
		},
	}, SaaSAlertNotificationChannelWebhook, s.notificationMaxAttempts)
	if err != nil {
		s.logger.Printf("enqueue SaaS payment settlement sync alert failed: run=%s provider=%s err=%v", run.RunNo, run.Provider, err)
	}
}

type SaaSPaymentSettlementSyncCron struct {
	service *SaaSPaymentSettlementSyncService
}

func NewSaaSPaymentSettlementSyncCron(service *SaaSPaymentSettlementSyncService) *SaaSPaymentSettlementSyncCron {
	return &SaaSPaymentSettlementSyncCron{service: service}
}

func (c *SaaSPaymentSettlementSyncCron) RunOnce(ctx context.Context) error {
	if c == nil || c.service == nil {
		return errors.New("payment settlement sync cron dependencies are not configured")
	}
	return c.service.RunAll(ctx)
}

func newSaaSPaymentSettlementSyncRunNo() (string, error) {
	random := make([]byte, 5)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return fmt.Sprintf("PSS-%s-%s", time.Now().UTC().Format("20060102T150405"), strings.ToUpper(hex.EncodeToString(random))), nil
}
