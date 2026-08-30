package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type filterCaptureStore struct {
	lastFilter MediaListFilter
}

func (s *filterCaptureStore) Create(context.Context, AudioObject) (int64, error)   { return 1, nil }
func (s *filterCaptureStore) GetByID(context.Context, int64) (*AudioObject, error) { return nil, nil }
func (s *filterCaptureStore) List(_ context.Context, filter MediaListFilter) (ListResult, error) {
	s.lastFilter = filter
	return ListResult{Page: filter.Page, PerPage: filter.PerPage}, nil
}
func (s *filterCaptureStore) SoftDelete(context.Context, int64, int64) error { return nil }

func TestMediaListPassesValidatedDateFilter(t *testing.T) {
	store := &filterCaptureStore{}
	storage, err := newTestAudioStorage(t)
	handler, err := NewMediaHandler(store, storage, fakeResolver{}, fakeAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media?page=2&perPage=20&sender=%E5%BC%A0%E4%B8%89&receiver=%E6%9D%8E%E5%9B%9B&from=2026-08-01&to=2026-08-20", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastFilter.Sender != "张三" || store.lastFilter.Receiver != "李四" || store.lastFilter.SyncedFrom != "2026-08-01" || store.lastFilter.SyncedTo != "2026-08-20" {
		t.Fatalf("filter=%+v", store.lastFilter)
	}
}

func TestMediaListRejectsInvalidDateRange(t *testing.T) {
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
