package companyprofile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

type companyProfileArchiveScheduler struct {
	calls     int
	principal dashboardprincipal.DashboardPrincipal
	requestID string
	result    ArchiveSyncStatus
	err       error
}

func (s *companyProfileArchiveScheduler) EnqueueArchiveSync(_ context.Context, principal dashboardprincipal.DashboardPrincipal, requestID string) (ArchiveSyncStatus, error) {
	s.calls++
	s.principal = principal
	s.requestID = requestID
	return s.result, s.err
}

func TestServiceArchiveSyncUsesAuthenticatedScopeAndReturnsQueuedStatus(t *testing.T) {
	store := &companyProfileContractStore{}
	scheduler := &companyProfileArchiveScheduler{result: ArchiveSyncStatus{Status: "queued", RunID: "91"}}
	service := NewService(store, &companyProfileTestVerifier{}).WithArchiveSyncScheduler(scheduler)
	principal := companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive)

	result, err := service.StartArchiveSync(context.Background(), principal, ArchiveSyncInput{RequestID: "manual-browser-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "queued" || scheduler.calls != 1 || scheduler.principal.TenantID != principal.TenantID || scheduler.principal.CorpID != principal.CorpID || scheduler.requestID != "manual-browser-1" {
		t.Fatalf("result=%+v scheduler=%+v", result, scheduler)
	}
}

func TestServiceArchiveSyncAllowsAuthorizedGlobalConversationOperator(t *testing.T) {
	store := &companyProfileContractStore{}
	scheduler := &companyProfileArchiveScheduler{result: ArchiveSyncStatus{Status: "queued", Available: true}}
	service := NewService(store, &companyProfileTestVerifier{}).WithArchiveSyncScheduler(scheduler)
	principal := companyProfileTestPrincipal(false, dashboardprincipal.CorpBindingStatusActive)
	ctx := dashboardprincipal.WithCapabilityAccess(context.Background(), false, []string{"dashboard.chat.v2_all"})

	if _, err := service.StartArchiveSync(ctx, principal, ArchiveSyncInput{RequestID: "conversation-sync-1"}); err != nil {
		t.Fatal(err)
	}
	if scheduler.calls != 1 {
		t.Fatalf("scheduler calls=%d, want 1", scheduler.calls)
	}
}

func TestServiceArchiveSyncStatusAllowsEmptyPendingBinding(t *testing.T) {
	store := &companyProfileContractStore{archiveSyncStatus: ArchiveSyncStatus{Status: "idle", Available: false, UnavailableReason: "需要管理员先完成会话存档授权"}}
	service := NewService(store, &companyProfileTestVerifier{})
	status, err := service.GetArchiveSyncStatus(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusPending))
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "idle" || status.Available || status.UnavailableReason == "" || store.archiveSyncStatusCalls != 1 {
		t.Fatalf("status=%+v calls=%d", status, store.archiveSyncStatusCalls)
	}
}

func TestHTTPArchiveSyncRejectsRealmSelectorsAndNeverReturnsTechnicalFields(t *testing.T) {
	finishedAt := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	store := &companyProfileContractStore{archiveSyncStatus: ArchiveSyncStatus{
		Status: "completed", Available: true, Processed: 7, Failed: 1, FinishedAt: &finishedAt,
	}}
	scheduler := &companyProfileArchiveScheduler{result: ArchiveSyncStatus{Status: "queued", Available: true, RunID: "92"}}
	handler := NewHTTPHandler(NewService(store, &companyProfileTestVerifier{}).WithArchiveSyncScheduler(scheduler))
	principalContext := dashboardprincipal.WithPrincipal(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive))

	request := httptest.NewRequest(http.MethodPost, "/dashboard/company/archive-sync", strings.NewReader(`{"requestId":"manual-browser-2"}`)).WithContext(principalContext)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || scheduler.calls != 1 || !strings.Contains(response.Body.String(), `"status":"queued"`) {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, scheduler.calls, response.Body.String())
	}

	badRequest := httptest.NewRequest(http.MethodPost, "/dashboard/company/archive-sync", strings.NewReader(`{"requestId":"manual-browser-3","tenantId":999,"corpId":888}`)).WithContext(principalContext)
	badResponse := httptest.NewRecorder()
	handler.ServeHTTP(badResponse, badRequest)
	if badResponse.Code != http.StatusBadRequest || scheduler.calls != 1 {
		t.Fatalf("bad status=%d calls=%d body=%s", badResponse.Code, scheduler.calls, badResponse.Body.String())
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/dashboard/company/archive-sync-status", nil).WithContext(principalContext)
	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("status response=%d body=%s", statusResponse.Code, statusResponse.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	serialized := strings.ToLower(statusResponse.Body.String())
	for _, forbidden := range []string{"archive.", "source", "cursor", "lease", "idempotency"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("status response leaked %q: %s", forbidden, serialized)
		}
	}
}
