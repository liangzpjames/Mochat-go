package companyprofile

import (
	"context"
	"errors"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

type companyProfileTestScheduler struct {
	calls         int
	bindingID     int
	cursor        string
	err           error
	beforeEnqueue func()
}

func (s *companyProfileTestScheduler) EnqueueEmployeeSync(_ context.Context, bindingID int) (string, error) {
	if s.beforeEnqueue != nil {
		s.beforeEnqueue()
	}
	s.calls++
	s.bindingID = bindingID
	if s.err != nil {
		return "", s.err
	}
	return s.cursor, nil
}

func TestServiceEmployeeSyncEnqueuesTenantBindingWithoutProviderCall(t *testing.T) {
	store := &companyProfileContractStore{verificationSnapshot: VerificationSnapshot{
		Verified: true, WXCorpID: "ww-authoritative", BindingVersion: 1,
	}}
	scheduler := &companyProfileTestScheduler{cursor: "company-sync", beforeEnqueue: func() {
		if store.queueCalls != 0 {
			t.Fatalf("queued marker written before Redis enqueue: %d", store.queueCalls)
		}
	}}
	service := NewService(store, &companyProfileTestVerifier{}).WithEmployeeSyncScheduler(scheduler)

	result, err := service.StartEmployeeSync(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "queued" || result.Cursor != "company-sync" || scheduler.calls != 1 || scheduler.bindingID != 202 {
		t.Fatalf("result=%+v scheduler=%+v", result, scheduler)
	}
}

func TestServiceEmployeeSyncRecordsSafeFailureWhenQueueUnavailable(t *testing.T) {
	store := &companyProfileContractStore{verificationSnapshot: VerificationSnapshot{
		Verified: true, WXCorpID: "ww-authoritative", BindingVersion: 1,
	}}
	scheduler := &companyProfileTestScheduler{err: errors.New("redis failure with employee-secret")}
	service := NewService(store, &companyProfileTestVerifier{}).WithEmployeeSyncScheduler(scheduler)

	result, err := service.StartEmployeeSync(context.Background(), companyProfileTestPrincipal(true, dashboardprincipal.CorpBindingStatusActive))
	if !errors.Is(err, ErrStoreUnavailable) || result.Status != "failed" || result.ErrorCode != "SYNC_FAILED" || store.queueCalls != 0 || store.failureCalls != 0 {
		t.Fatalf("result=%+v err=%v queueCalls=%d failureCalls=%d", result, err, store.queueCalls, store.failureCalls)
	}
	if strings.Contains(err.Error(), "employee-secret") {
		t.Fatalf("queue error leaked provider material: %v", err)
	}
}
