package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaterialBatchDestroyRejectsReferencedMaterial(t *testing.T) {
	store := &fakeMaterialFoundationStore{
		fakeMediumStore: fakeMediumStore{users: map[int]User{1: {ID: 1, Name: "管理员"}}},
		references:      []MaterialReference{{SourceType: "greeting", SourceID: 8, SourceName: "新客欢迎语"}},
	}
	handler := NewMediumHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")
	req := httptest.NewRequest(http.MethodPost, "/dashboard/medium/batchDestroy", strings.NewReader(`{"ids":[21]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BatchDestroy(rec, req)

	if rec.Code != http.StatusConflict || store.batchDeleted {
		t.Fatalf("status=%d deleted=%v body=%s", rec.Code, store.batchDeleted, rec.Body.String())
	}
}

func TestMaterialSelectorReturnsOnlyVisibleAvailableItems(t *testing.T) {
	store := &fakeMaterialFoundationStore{fakeMediumStore: fakeMediumStore{
		users: map[int]User{1: {ID: 1, Name: "管理员"}},
		page:  MediumPage{Items: []MediumItem{{ID: 21, Type: 1, Content: map[string]any{"title": "问候", "content": "你好"}, ScopeType: "public", SidebarVisible: true, Status: "available"}}, Total: 1, PerPage: 20},
	}}
	handler := NewMediumHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")
	req := httptest.NewRequest(http.MethodGet, "/dashboard/materialSelector/index?scene=chat&scopeType=public", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Selector(rec, req)

	if rec.Code != http.StatusOK || store.lastFilter.Status != "available" || store.lastFilter.SidebarVisible == nil || !*store.lastFilter.SidebarVisible {
		t.Fatalf("status=%d filter=%#v body=%s", rec.Code, store.lastFilter, rec.Body.String())
	}
}

func TestMaterialSelectorUsesCurrentUserVisibleScopes(t *testing.T) {
	store := &fakeMaterialFoundationStore{fakeMediumStore: fakeMediumStore{
		users: map[int]User{1: {ID: 1, Name: "管理员"}},
		page:  MediumPage{PerPage: 20},
	}}
	handler := NewMediumHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")
	req := httptest.NewRequest(http.MethodGet, "/dashboard/materialSelector/index?scene=friends_circle&scopeType=personal&scopeId=999", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Selector(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastFilter.ScopeType != "" || store.lastFilter.ScopeID != 0 || !store.lastFilter.SelectorVisible || store.lastFilter.UserID != 1 {
		t.Fatalf("selector visibility filter=%#v", store.lastFilter)
	}
}

func TestMaterialBatchMoveIsAtomicWithinCorp(t *testing.T) {
	store := &fakeMaterialFoundationStore{fakeMediumStore: fakeMediumStore{users: map[int]User{1: {ID: 1, Name: "管理员"}}}}
	handler := NewMediumHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")
	req := httptest.NewRequest(http.MethodPost, "/dashboard/medium/batchGroupUpdate", strings.NewReader(`{"ids":[21,22],"mediumGroupId":9}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.BatchGroupUpdate(rec, req)

	if rec.Code != http.StatusOK || store.batchMoveCorpID != 7 || store.batchMoveGroupID != 9 || len(store.batchMoveIDs) != 2 {
		t.Fatalf("status=%d corp=%d group=%d ids=%v body=%s", rec.Code, store.batchMoveCorpID, store.batchMoveGroupID, store.batchMoveIDs, rec.Body.String())
	}
}

type fakeMaterialFoundationStore struct {
	fakeMediumStore
	references       []MaterialReference
	batchDeleted     bool
	batchMoveCorpID  int
	batchMoveGroupID int
	batchMoveIDs     []int
}

func (s *fakeMaterialFoundationStore) MaterialReferences(_ context.Context, _ int, _ []int) ([]MaterialReference, error) {
	return s.references, nil
}

func (s *fakeMaterialFoundationStore) BatchDeleteMedium(_ context.Context, _ int, _ []int) (bool, error) {
	s.batchDeleted = true
	return true, nil
}

func (s *fakeMaterialFoundationStore) BatchUpdateMediumGroupID(_ context.Context, corpID int, ids []int, groupID int) (bool, error) {
	s.batchMoveCorpID, s.batchMoveIDs, s.batchMoveGroupID = corpID, append([]int(nil), ids...), groupID
	return true, nil
}
