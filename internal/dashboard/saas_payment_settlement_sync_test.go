package dashboard

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSaaSPaymentSettlementHTTPBridgeClientSendsContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != saasPaymentSettlementBridgePath || r.URL.Query().Get("provider") != "gateway" || r.URL.Query().Get("cursor") != "cursor-1" || r.URL.Query().Get("limit") != "25" {
			t.Fatalf("request = %s", r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer bridge-secret" || r.Header.Get("Accept") != "application/json" {
			t.Fatalf("headers = %+v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"provider":"gateway","nextCursor":"cursor-2","hasMore":false,"batches":[]}`)
	}))
	defer server.Close()
	client, err := NewSaaSPaymentSettlementHTTPBridgeClient(server.URL, "bridge-secret", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.FetchPaymentSettlements(context.Background(), "gateway", "cursor-1", 25)
	if err != nil {
		t.Fatal(err)
	}
	if page.Provider != "gateway" || page.NextCursor != "cursor-2" || page.HasMore || len(page.Batches) != 0 {
		t.Fatalf("page = %+v", page)
	}
}

func TestSaaSPaymentSettlementSyncServiceImportsPagesAndAlertsIssues(t *testing.T) {
	store := &fakePaymentSettlementSyncStore{cursor: "cursor-0", importResults: []SaaSPaymentSettlementImportResult{
		{Batch: SaaSPaymentSettlementBatch{BatchNo: "SET-1", IssueCount: 2, OpenIssueCount: 2}},
		{Batch: SaaSPaymentSettlementBatch{BatchNo: "SET-2"}, Idempotent: true},
	}}
	client := &fakePaymentSettlementBridgeClient{pages: []SaaSPaymentSettlementBridgePage{
		{Provider: "gateway", NextCursor: "cursor-1", HasMore: true, Batches: []saasPaymentSettlementImportRequest{paymentSettlementBridgeBatch("GW-SET-1", "GW-TXN-1", "PAY-1")}},
		{Provider: "gateway", NextCursor: "cursor-2", Batches: []saasPaymentSettlementImportRequest{paymentSettlementBridgeBatch("GW-SET-2", "GW-TXN-2", "PAY-2")}},
	}}
	service, err := NewSaaSPaymentSettlementSyncService(store, client, []string{"gateway"}, 100, 1, 3, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.Run(context.Background(), "gateway", false, SaaSPaymentSettlementSyncSourceCron, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != SaaSPaymentSettlementSyncStatusSucceeded || run.CursorAfter != "cursor-2" || run.FetchedPageCount != 2 || run.FetchedBatchCount != 2 || run.ImportedBatchCount != 1 || run.IdempotentBatchCount != 1 || run.EntryCount != 2 || run.IssueCount != 2 || run.OpenIssueCount != 2 {
		t.Fatalf("run = %+v", run)
	}
	if len(store.imports) != 2 || store.imports[0].SourceSHA256 == "" || store.imports[0].ActorTenantID != 1 {
		t.Fatalf("imports = %+v", store.imports)
	}
	if len(store.alerts) != 1 || store.alerts[0].AlertType != SaaSAlertTypePaymentSettlementIssue || store.alerts[0].PeriodKey != run.RunNo {
		t.Fatalf("alerts = %+v", store.alerts)
	}
}

func TestSaaSPaymentSettlementSyncServicePreviewDoesNotImportOrAdvanceCursor(t *testing.T) {
	store := &fakePaymentSettlementSyncStore{cursor: "cursor-0"}
	client := &fakePaymentSettlementBridgeClient{pages: []SaaSPaymentSettlementBridgePage{{
		Provider: "gateway", NextCursor: "cursor-1", Batches: []saasPaymentSettlementImportRequest{paymentSettlementBridgeBatch("GW-SET-1", "GW-TXN-1", "PAY-1")},
	}}}
	service, err := NewSaaSPaymentSettlementSyncService(store, client, []string{"gateway"}, 100, 1, 3, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.Run(context.Background(), "gateway", true, SaaSPaymentSettlementSyncSourceManual, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != SaaSPaymentSettlementSyncStatusPreviewed || run.CursorAfter != "cursor-0" || len(store.imports) != 0 || run.FetchedBatchCount != 1 || run.EntryCount != 1 {
		t.Fatalf("run=%+v imports=%d", run, len(store.imports))
	}
}

func TestSaaSPaymentSettlementSyncServiceFailurePreservesProgress(t *testing.T) {
	store := &fakePaymentSettlementSyncStore{cursor: "cursor-0", importResults: []SaaSPaymentSettlementImportResult{{Batch: SaaSPaymentSettlementBatch{BatchNo: "SET-1", IssueCount: 1, OpenIssueCount: 1}}}}
	client := &fakePaymentSettlementBridgeClient{
		pages: []SaaSPaymentSettlementBridgePage{{Provider: "gateway", NextCursor: "cursor-1", HasMore: true, Batches: []saasPaymentSettlementImportRequest{paymentSettlementBridgeBatch("GW-SET-1", "GW-TXN-1", "PAY-1")}}},
		err:   errors.New("bridge unavailable"),
	}
	service, err := NewSaaSPaymentSettlementSyncService(store, client, []string{"gateway"}, 100, 1, 3, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.Run(context.Background(), "gateway", false, SaaSPaymentSettlementSyncSourceCron, 0, 1)
	if err == nil || !strings.Contains(err.Error(), "bridge unavailable") {
		t.Fatalf("err = %v", err)
	}
	if run.Status != SaaSPaymentSettlementSyncStatusFailed || run.FetchedPageCount != 1 || run.FetchedBatchCount != 1 || run.ImportedBatchCount != 1 || run.OpenIssueCount != 1 {
		t.Fatalf("run = %+v", run)
	}
	if store.failure.FetchedPageCount != 1 || store.failure.ImportedBatchCount != 1 || len(store.alerts) != 1 || store.alerts[0].AlertType != SaaSAlertTypePaymentSettlementSyncFailed {
		t.Fatalf("failure=%+v alerts=%+v", store.failure, store.alerts)
	}
}

func paymentSettlementBridgeBatch(settlementNo, transactionNo, orderNo string) saasPaymentSettlementImportRequest {
	netAmount := int64(9940)
	return saasPaymentSettlementImportRequest{
		Provider: "gateway", ProviderSettlementNo: settlementNo, Currency: "CNY",
		Entries: []saasPaymentSettlementImportEntryRequest{{
			ProviderTransactionNo: transactionNo, TransactionType: SaaSPaymentSettlementTransactionPayment,
			OrderNo: orderNo, AmountCents: 10000, FeeCents: -60, NetAmountCents: &netAmount,
		}},
	}
}

type fakePaymentSettlementBridgeClient struct {
	pages []SaaSPaymentSettlementBridgePage
	err   error
	calls int
}

func (c *fakePaymentSettlementBridgeClient) FetchPaymentSettlements(_ context.Context, _ string, _ string, _ int) (SaaSPaymentSettlementBridgePage, error) {
	if c.calls < len(c.pages) {
		page := c.pages[c.calls]
		c.calls++
		return page, nil
	}
	c.calls++
	if c.err != nil {
		return SaaSPaymentSettlementBridgePage{}, c.err
	}
	return SaaSPaymentSettlementBridgePage{}, nil
}

type fakePaymentSettlementSyncStore struct {
	cursor        string
	run           SaaSPaymentSettlementSyncRun
	imports       []SaaSPaymentSettlementImport
	importResults []SaaSPaymentSettlementImportResult
	failure       SaaSPaymentSettlementSyncFailure
	alerts        []SaaSQuotaAlert
}

func (s *fakePaymentSettlementSyncStore) BeginSaaSPaymentSettlementSync(_ context.Context, input SaaSPaymentSettlementSyncBegin) (SaaSPaymentSettlementSyncRun, SaaSPaymentSettlementSyncState, error) {
	s.run = SaaSPaymentSettlementSyncRun{ID: 1, RunNo: input.RunNo, Provider: input.Provider, Source: input.Source, Status: SaaSPaymentSettlementSyncStatusRunning, DryRun: input.DryRun, CursorBefore: s.cursor, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID}
	return s.run, SaaSPaymentSettlementSyncState{ID: 1, Provider: input.Provider, Cursor: s.cursor, ActiveRunID: 1}, nil
}

func (s *fakePaymentSettlementSyncStore) FinishSaaSPaymentSettlementSync(_ context.Context, input SaaSPaymentSettlementSyncFinish) (SaaSPaymentSettlementSyncRun, SaaSPaymentSettlementSyncState, error) {
	s.run.Status, s.run.CursorAfter, s.run.DryRun = input.Status, input.CursorAfter, input.DryRun
	s.run.FetchedPageCount, s.run.FetchedBatchCount = input.FetchedPageCount, input.FetchedBatchCount
	s.run.ImportedBatchCount, s.run.IdempotentBatchCount = input.ImportedBatchCount, input.IdempotentBatchCount
	s.run.EntryCount, s.run.IssueCount, s.run.OpenIssueCount = input.EntryCount, input.IssueCount, input.OpenIssueCount
	return s.run, SaaSPaymentSettlementSyncState{ID: 1, Provider: input.Provider, Cursor: input.CursorAfter}, nil
}

func (s *fakePaymentSettlementSyncStore) FailSaaSPaymentSettlementSync(_ context.Context, input SaaSPaymentSettlementSyncFailure) (SaaSPaymentSettlementSyncRun, SaaSPaymentSettlementSyncState, error) {
	s.failure = input
	s.run.Status, s.run.ErrorMessage = SaaSPaymentSettlementSyncStatusFailed, input.ErrorMessage
	s.run.FetchedPageCount, s.run.FetchedBatchCount = input.FetchedPageCount, input.FetchedBatchCount
	s.run.ImportedBatchCount, s.run.IdempotentBatchCount = input.ImportedBatchCount, input.IdempotentBatchCount
	s.run.EntryCount, s.run.IssueCount, s.run.OpenIssueCount = input.EntryCount, input.IssueCount, input.OpenIssueCount
	return s.run, SaaSPaymentSettlementSyncState{ID: 1, Provider: input.Provider, Cursor: s.cursor, LastError: input.ErrorMessage}, nil
}

func (s *fakePaymentSettlementSyncStore) SaaSAdminPaymentSettlementSyncRuns(_ context.Context, options SaaSPaymentSettlementSyncRunOptions) (SaaSPaymentSettlementSyncReport, error) {
	return SaaSPaymentSettlementSyncReport{Options: options, Runs: []SaaSPaymentSettlementSyncRun{s.run}}, nil
}

func (s *fakePaymentSettlementSyncStore) ImportSaaSAdminPaymentSettlement(_ context.Context, input SaaSPaymentSettlementImport) (SaaSPaymentSettlementImportResult, error) {
	s.imports = append(s.imports, input)
	index := len(s.imports) - 1
	if index < len(s.importResults) {
		return s.importResults[index], nil
	}
	return SaaSPaymentSettlementImportResult{Batch: SaaSPaymentSettlementBatch{BatchNo: input.BatchNo}}, nil
}

func (s *fakePaymentSettlementSyncStore) EnqueueSaaSAlertNotification(_ context.Context, alert SaaSQuotaAlert, _ string, _ int) (SaaSAlertNotification, error) {
	s.alerts = append(s.alerts, alert)
	return SaaSAlertNotification{}, nil
}
