package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSaaSAdminPaymentSettlementRequiresPlatformAdminAndParsesFilters(t *testing.T) {
	store := newFakeSaaSPaymentSettlementStore()
	store.batchReport = SaaSPaymentSettlementBatchReport{Batches: []SaaSPaymentSettlementBatch{{BatchNo: "SET-1", Provider: "gateway", Status: SaaSPaymentSettlementBatchStatusReconciled}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	tenantReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/paymentSettlementBatches", nil)
	tenantReq.Header.Set("X-Mochat-Go-User-ID", "2")
	tenantRec := httptest.NewRecorder()
	handler.PaymentSettlementBatches(tenantRec, tenantReq)
	if tenantRec.Code != http.StatusForbidden {
		t.Fatalf("tenant status=%d body=%s", tenantRec.Code, tenantRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/paymentSettlementBatches?provider=gateway&status=reconciled&currency=cny&keyword=SET&limit=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.PaymentSettlementBatches(rec, req)
	if rec.Code != http.StatusOK || decodeBody(t, rec.Body.Bytes())["code"].(float64) != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastBatchOptions.Provider != "gateway" || store.lastBatchOptions.Status != SaaSPaymentSettlementBatchStatusReconciled || store.lastBatchOptions.Currency != "CNY" || store.lastBatchOptions.Keyword != "SET" || store.lastBatchOptions.Limit != 10 {
		t.Fatalf("options=%+v", store.lastBatchOptions)
	}
}

func TestSaaSAdminPaymentSettlementImportJSONAndCSV(t *testing.T) {
	store := newFakeSaaSPaymentSettlementStore()
	store.importResult = SaaSPaymentSettlementImportResult{Batch: SaaSPaymentSettlementBatch{BatchNo: "SET-JSON", Version: 1}, Entries: SaaSPaymentSettlementEntryReport{Summary: SaaSPaymentSettlementEntrySummary{EntryCount: 2}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	body := `{"batchNo":"SET-JSON","provider":"gateway","providerSettlementNo":"GW-SET-1","periodStart":"2026-07-01 00:00:00","periodEnd":"2026-07-02 00:00:00","currency":"cny","remark":"日结单","entries":[{"lineNo":1,"providerTransactionNo":"GW-PAY-1","transactionType":"payment","orderNo":"PAY-1","providerOrderNo":"GW-ORDER-1","amountCents":10000,"feeCents":-60,"netAmountCents":9940,"occurredAt":"2026-07-01 12:00:00"},{"lineNo":2,"providerTransactionNo":"GW-REF-1","transactionType":"refund","refundNo":"REF-1","providerRefundNo":"GW-REFUND-1","amountCents":-3000,"feeCents":0,"netAmountCents":-3000}]}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentSettlementImport", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ImportPaymentSettlement(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	input := store.lastImport
	if input.BatchNo != "SET-JSON" || input.Provider != "gateway" || input.ProviderSettlementNo != "GW-SET-1" || input.Currency != "CNY" || input.ActorUserID != 1 || input.ActorTenantID != 1 || len(input.Entries) != 2 {
		t.Fatalf("input=%+v", input)
	}
	if input.Entries[0].NetAmountCents != 9940 || input.Entries[1].AmountCents != -3000 || input.SourceSHA256 == "" {
		t.Fatalf("entries=%+v source=%q", input.Entries, input.SourceSHA256)
	}

	csvBody := "batchNo,provider,providerSettlementNo,periodStart,periodEnd,currency,lineNo,providerTransactionNo,transactionType,orderNo,providerOrderNo,refundNo,providerRefundNo,amountCents,feeCents,netAmountCents,occurredAt,remark\n" +
		"SET-CSV,gateway,GW-SET-CSV,2026-07-02 00:00:00,2026-07-03 00:00:00,CNY,1,GW-PAY-CSV,payment,PAY-CSV,GW-ORDER-CSV,, ,8800,-50,8750,2026-07-02 08:00:00,CSV 导入\n"
	csvReq := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentSettlementImport", strings.NewReader(csvBody))
	csvReq.Header.Set("Content-Type", "text/csv")
	csvReq.Header.Set("X-Mochat-Go-User-ID", "1")
	csvRec := httptest.NewRecorder()
	handler.ImportPaymentSettlement(csvRec, csvReq)
	if csvRec.Code != http.StatusOK {
		t.Fatalf("csv status=%d body=%s", csvRec.Code, csvRec.Body.String())
	}
	if store.lastImport.BatchNo != "SET-CSV" || len(store.lastImport.Entries) != 1 || store.lastImport.Entries[0].AmountCents != 8800 || store.lastImport.Entries[0].RawJSON == "" {
		t.Fatalf("csv input=%+v", store.lastImport)
	}
}

func TestSaaSPaymentSettlementImportRejectsInvalidSignedAmountsAndNet(t *testing.T) {
	for name, body := range map[string]string{
		"positive refund": `{"provider":"gateway","providerSettlementNo":"GW-1","currency":"CNY","entries":[{"providerTransactionNo":"T-1","transactionType":"refund","refundNo":"REF-1","amountCents":100}]}`,
		"wrong net":       `{"provider":"gateway","providerSettlementNo":"GW-1","currency":"CNY","entries":[{"providerTransactionNo":"T-1","transactionType":"payment","orderNo":"PAY-1","amountCents":100,"feeCents":-2,"netAmountCents":99}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/paymentSettlementImport", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if _, err := parseSaaSPaymentSettlementImport(req); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestSaaSAdminPaymentSettlementActionsCarryActorAndVersion(t *testing.T) {
	store := newFakeSaaSPaymentSettlementStore()
	store.reconcileResult = SaaSPaymentSettlementReconcileResult{Batch: SaaSPaymentSettlementBatch{BatchNo: "SET-1", Version: 4}}
	store.resolveResult = SaaSPaymentSettlementEntryResolveResult{Entry: SaaSPaymentSettlementEntry{ID: 7, Version: 3}, Batch: SaaSPaymentSettlementBatch{BatchNo: "SET-1", Version: 5}}
	store.transitionResult = SaaSPaymentSettlementTransitionResult{Batch: SaaSPaymentSettlementBatch{BatchNo: "SET-1", Status: SaaSPaymentSettlementBatchStatusClosed, Version: 6}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	requests := []struct {
		path string
		body string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"/dashboard/saasAdmin/paymentSettlementReconcile", `{"batchNo":"SET-1","expectedVersion":3,"dryRun":false}`, handler.ReconcilePaymentSettlement},
		{"/dashboard/saasAdmin/paymentSettlementResolve", `{"entryId":7,"expectedVersion":2,"handlingStatus":"resolved","reason":"财务已核实"}`, handler.ResolvePaymentSettlementEntry},
		{"/dashboard/saasAdmin/paymentSettlementTransition", `{"batchNo":"SET-1","expectedVersion":5,"action":"close","reason":"差异已处理"}`, handler.TransitionPaymentSettlement},
	}
	for _, item := range requests {
		req := httptest.NewRequest(http.MethodPost, item.path, strings.NewReader(item.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()
		item.call(rec, req)
		if rec.Code != http.StatusOK || decodeBody(t, rec.Body.Bytes())["code"].(float64) != http.StatusOK {
			t.Fatalf("path=%s status=%d body=%s", item.path, rec.Code, rec.Body.String())
		}
	}
	if store.lastReconcile.BatchNo != "SET-1" || store.lastReconcile.ExpectedVersion != 3 || store.lastReconcile.DryRun || store.lastReconcile.ActorUserID != 1 {
		t.Fatalf("reconcile=%+v", store.lastReconcile)
	}
	if store.lastResolve.EntryID != 7 || store.lastResolve.ExpectedVersion != 2 || store.lastResolve.HandlingStatus != SaaSPaymentSettlementHandlingResolved || store.lastResolve.ActorTenantID != 1 {
		t.Fatalf("resolve=%+v", store.lastResolve)
	}
	if store.lastTransition.BatchNo != "SET-1" || store.lastTransition.ExpectedVersion != 5 || store.lastTransition.Action != SaaSPaymentSettlementTransitionClose || store.lastTransition.ActorUserID != 1 {
		t.Fatalf("transition=%+v", store.lastTransition)
	}
}

func TestSaaSAdminPaymentSettlementCSVExports(t *testing.T) {
	store := newFakeSaaSPaymentSettlementStore()
	store.batchReport = SaaSPaymentSettlementBatchReport{Batches: []SaaSPaymentSettlementBatch{{BatchNo: "SET-CSV", Provider: "gateway", ProviderSettlementNo: "GW-SET", Status: SaaSPaymentSettlementBatchStatusClosed, Currency: "CNY"}}}
	store.entryReport = SaaSPaymentSettlementEntryReport{Entries: []SaaSPaymentSettlementEntry{{BatchNo: "SET-CSV", LineNo: 1, Provider: "gateway", ProviderTransactionNo: "GW-TXN", TransactionType: SaaSPaymentSettlementTransactionPayment, AmountCents: 1000, NetAmountCents: 990, Currency: "CNY", ReconciliationStatus: SaaSPaymentSettlementReconciliationMatched, HandlingStatus: SaaSPaymentSettlementHandlingNone}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	for exportType, token := range map[string]string{"paymentSettlementBatches": "SET-CSV,gateway,GW-SET", "paymentSettlementEntries": "SET-CSV,,1,gateway,GW-TXN"} {
		req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/export?type="+exportType, nil)
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()
		handler.ExportCSV(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), token) {
			t.Fatalf("type=%s status=%d body=%s", exportType, rec.Code, rec.Body.String())
		}
	}
}

type fakeSaaSPaymentSettlementStore struct {
	*fakeSaaSAdminRefundStore
	batchReport      SaaSPaymentSettlementBatchReport
	entryReport      SaaSPaymentSettlementEntryReport
	importResult     SaaSPaymentSettlementImportResult
	reconcileResult  SaaSPaymentSettlementReconcileResult
	resolveResult    SaaSPaymentSettlementEntryResolveResult
	transitionResult SaaSPaymentSettlementTransitionResult
	lastBatchOptions SaaSPaymentSettlementBatchOptions
	lastEntryOptions SaaSPaymentSettlementEntryOptions
	lastImport       SaaSPaymentSettlementImport
	lastReconcile    SaaSPaymentSettlementReconcile
	lastResolve      SaaSPaymentSettlementEntryResolve
	lastTransition   SaaSPaymentSettlementTransition
}

func newFakeSaaSPaymentSettlementStore() *fakeSaaSPaymentSettlementStore {
	return &fakeSaaSPaymentSettlementStore{fakeSaaSAdminRefundStore: newFakeSaaSAdminRefundStore()}
}

func (s *fakeSaaSPaymentSettlementStore) SaaSAdminPaymentSettlementBatches(_ context.Context, options SaaSPaymentSettlementBatchOptions) (SaaSPaymentSettlementBatchReport, error) {
	s.lastBatchOptions = options
	s.batchReport.Options = options
	return s.batchReport, nil
}

func (s *fakeSaaSPaymentSettlementStore) SaaSAdminPaymentSettlementEntries(_ context.Context, options SaaSPaymentSettlementEntryOptions) (SaaSPaymentSettlementEntryReport, error) {
	s.lastEntryOptions = options
	s.entryReport.Options = options
	return s.entryReport, nil
}

func (s *fakeSaaSPaymentSettlementStore) ImportSaaSAdminPaymentSettlement(_ context.Context, input SaaSPaymentSettlementImport) (SaaSPaymentSettlementImportResult, error) {
	s.lastImport = input
	return s.importResult, nil
}

func (s *fakeSaaSPaymentSettlementStore) ReconcileSaaSAdminPaymentSettlement(_ context.Context, input SaaSPaymentSettlementReconcile) (SaaSPaymentSettlementReconcileResult, error) {
	s.lastReconcile = input
	return s.reconcileResult, nil
}

func (s *fakeSaaSPaymentSettlementStore) ResolveSaaSAdminPaymentSettlementEntry(_ context.Context, input SaaSPaymentSettlementEntryResolve) (SaaSPaymentSettlementEntryResolveResult, error) {
	s.lastResolve = input
	return s.resolveResult, nil
}

func (s *fakeSaaSPaymentSettlementStore) TransitionSaaSAdminPaymentSettlement(_ context.Context, input SaaSPaymentSettlementTransition) (SaaSPaymentSettlementTransitionResult, error) {
	s.lastTransition = input
	return s.transitionResult, nil
}
