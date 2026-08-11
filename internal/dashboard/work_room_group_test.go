package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkRoomGroupIndexReturnsPagedGroups(t *testing.T) {
	store := &fakeWorkRoomGroupStore{
		users: map[int]User{1: {ID: 1}},
		page: WorkRoomGroupPage{
			Items: []WorkRoomGroupItem{{ID: 11, CorpID: 7, Name: "潜在群", CreatedAt: "2026-07-03 10:00:00"}},
			Total: 1, TotalPage: 1, PerPage: 10,
		},
	}
	handler := NewWorkRoomGroupHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoomGroup/index?page=1&perPage=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastPageCorpID != 7 || store.lastPage != 1 || store.lastPerPage != 10 {
		t.Fatalf("page call = corp %d page %d perPage %d", store.lastPageCorpID, store.lastPage, store.lastPerPage)
	}
	body := decodeBody(t, rec.Body.Bytes())
	list := body["data"].(map[string]any)["list"].([]any)
	item := list[0].(map[string]any)
	if int(item["workRoomGroupId"].(float64)) != 11 || item["workRoomGroupName"] != "潜在群" || int(item["corpId"].(float64)) != 7 {
		t.Fatalf("item = %#v", item)
	}
}

func TestWorkRoomGroupStoreCreatesGroup(t *testing.T) {
	store := &fakeWorkRoomGroupStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkRoomGroupHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workRoomGroup/store", strings.NewReader(`{"corpId":7,"workRoomGroupName":"成交群"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastNameCorpID != 7 || store.lastName != "成交群" || store.lastExcludeID != 0 {
		t.Fatalf("name check = corp %d name %q exclude %d", store.lastNameCorpID, store.lastName, store.lastExcludeID)
	}
	if store.created.CorpID != 7 || store.created.Name != "成交群" {
		t.Fatalf("created = %#v", store.created)
	}
}

func TestWorkRoomGroupUpdateRejectsDuplicateName(t *testing.T) {
	store := &fakeWorkRoomGroupStore{
		users:      map[int]User{1: {ID: 1}},
		group:      WorkRoomGroupItem{ID: 11, CorpID: 7, Name: "旧名称"},
		groupFound: true,
		nameExists: true,
	}
	handler := NewWorkRoomGroupHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workRoomGroup/update", strings.NewReader(`{"workRoomGroupId":11,"workRoomGroupName":"重复群"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "该客户群分组名称已存在，不可更新" {
		t.Fatalf("msg = %#v", body["msg"])
	}
	if store.updatedGroupID != 0 {
		t.Fatalf("updated = %d", store.updatedGroupID)
	}
}

func TestWorkRoomGroupDestroyReassignsRooms(t *testing.T) {
	store := &fakeWorkRoomGroupStore{
		users:      map[int]User{1: {ID: 1}},
		group:      WorkRoomGroupItem{ID: 11, CorpID: 7, Name: "旧名称"},
		groupFound: true,
	}
	handler := NewWorkRoomGroupHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/workRoomGroup/destroy", strings.NewReader(`{"workRoomGroupId":11}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedGroupID != 11 {
		t.Fatalf("deleted group id = %d", store.deletedGroupID)
	}
}

type fakeWorkRoomGroupStore struct {
	users          map[int]User
	page           WorkRoomGroupPage
	group          WorkRoomGroupItem
	groupFound     bool
	nameExists     bool
	lastPageCorpID int
	lastPage       int
	lastPerPage    int
	lastGroupID    int
	lastNameCorpID int
	lastName       string
	lastExcludeID  int
	created        WorkRoomGroupWrite
	updatedGroupID int
	updatedName    string
	deletedGroupID int
}

func (s *fakeWorkRoomGroupStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeWorkRoomGroupStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 0, nil
}

func (s *fakeWorkRoomGroupStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 0, 0, false, nil
}

func (s *fakeWorkRoomGroupStore) WorkRoomGroupPage(_ context.Context, corpID int, page int, perPage int) (WorkRoomGroupPage, error) {
	s.lastPageCorpID = corpID
	s.lastPage = page
	s.lastPerPage = perPage
	return s.page, nil
}

func (s *fakeWorkRoomGroupStore) WorkRoomGroupByID(_ context.Context, groupID int) (WorkRoomGroupItem, bool, error) {
	s.lastGroupID = groupID
	return s.group, s.groupFound, nil
}

func (s *fakeWorkRoomGroupStore) WorkRoomGroupNameExists(_ context.Context, corpID int, name string, excludeGroupID int) (bool, error) {
	s.lastNameCorpID = corpID
	s.lastName = name
	s.lastExcludeID = excludeGroupID
	return s.nameExists, nil
}

func (s *fakeWorkRoomGroupStore) CreateWorkRoomGroup(_ context.Context, values WorkRoomGroupWrite) (int, error) {
	s.created = values
	return 1, nil
}

func (s *fakeWorkRoomGroupStore) UpdateWorkRoomGroup(_ context.Context, groupID int, name string) (bool, error) {
	s.updatedGroupID = groupID
	s.updatedName = name
	return true, nil
}

func (s *fakeWorkRoomGroupStore) DeleteWorkRoomGroupReassignRooms(_ context.Context, groupID int) (bool, error) {
	s.deletedGroupID = groupID
	return true, nil
}
