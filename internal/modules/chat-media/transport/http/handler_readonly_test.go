package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMediaHandlerRejectsManualWriteMethods(t *testing.T) {
	handler, _, _ := newTestHandler(t)
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		req := httptest.NewRequest(method, "/dashboard/chat/media", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("method=%s status=%d body=%s", method, rec.Code, rec.Body.String())
		}
	}
}

func TestMediaListPassesSynchronizedRecordingFilters(t *testing.T) {
	store := &filterCaptureStore{}
	storage, err := newTestAudioStorage(t)
	handler, err := NewMediaHandler(store, storage, fakeResolver{}, fakeAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media?page=2&perPage=20&sender=%E5%BC%A0%E4%BC%9F&receiver=%E9%99%88%E7%BB%8F%E7%90%86&from=2026-08-01&to=2026-08-20", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastFilter.Sender != "张伟" || store.lastFilter.Receiver != "陈经理" || store.lastFilter.SyncedFrom != "2026-08-01" || store.lastFilter.SyncedTo != "2026-08-20" {
		t.Fatalf("filter=%+v", store.lastFilter)
	}
}

func TestMediaListRejectsInvalidSynchronizedDateRange(t *testing.T) {
	storage, err := newTestAudioStorage(t)
	handler, err := NewMediaHandler(&filterCaptureStore{}, storage, fakeResolver{}, fakeAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media?from=2026-08-20&to=2026-08-01", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}
