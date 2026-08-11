package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMediumGroupIndexReturnsGroupsAndAuthorizes(t *testing.T) {
	store := &fakeMediumGroupStore{
		users: map[int]User{1: {ID: 1}},
		groups: []MediumGroup{
			{ID: 11, Name: "图片素材"},
		},
	}
	authz := &recordingAuthorizer{}
	handler := NewMediumGroupHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authz)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/mediumGroup/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authz.permissionKey != "/dashboard/mediumGroup/index#get" || authz.corpID != 7 || authz.workEmployeeID != 99 {
		t.Fatalf("authz = %#v", authz)
	}
	if store.lastCorpID != 7 {
		t.Fatalf("corp id = %d", store.lastCorpID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	first := data[0].(map[string]any)
	second := data[1].(map[string]any)
	if int(first["id"].(float64)) != 0 || first["name"] != "全部分组" || int(second["id"].(float64)) != 11 || second["name"] != "图片素材" {
		t.Fatalf("data = %#v", data)
	}
}

func TestMediumGroupStoreCreatesGroup(t *testing.T) {
	store := &fakeMediumGroupStore{users: map[int]User{1: {ID: 1}}, createID: 22}
	handler := NewMediumGroupHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/mediumGroup/store", strings.NewReader(`{"name":"图片素材"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastNameCheckCorpID != 7 || store.lastNameCheckName != "图片素材" || store.lastNameCheckExcludeID != 0 {
		t.Fatalf("name check = corp %d name %q exclude %d", store.lastNameCheckCorpID, store.lastNameCheckName, store.lastNameCheckExcludeID)
	}
	if store.created.CorpID != 7 || store.created.Name != "图片素材" {
		t.Fatalf("created = %#v", store.created)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["id"].(float64)) != 22 {
		t.Fatalf("data = %#v", data)
	}
}

func TestMediumGroupDestroyReassignsMedia(t *testing.T) {
	store := &fakeMediumGroupStore{users: map[int]User{1: {ID: 1}}}
	handler := NewMediumGroupHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/mediumGroup/destroy", strings.NewReader(`{"id":11}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedCorpID != 7 || store.deletedGroupID != 11 {
		t.Fatalf("deleted = corp %d group %d", store.deletedCorpID, store.deletedGroupID)
	}
}

func TestSidebarMediumGroupIndexUsesEmployeeCorp(t *testing.T) {
	store := &fakeMediumGroupStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		groups:           []MediumGroup{{ID: 11, Name: "文件素材"}},
	}
	handler := NewMediumGroupHandler(store, nil, HeaderUserIDResolver{}, nil).
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/mediumGroup/index", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSidebarEmployeeID != 5 || store.lastCorpID != 7 {
		t.Fatalf("sidebar = employee %d corp %d", store.lastSidebarEmployeeID, store.lastCorpID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	first := data[0].(map[string]any)
	if int(first["id"].(float64)) != 0 || first["name"] != "未分组" {
		t.Fatalf("data = %#v", data)
	}
}

type fakeMediumGroupStore struct {
	users                  map[int]User
	sidebarEmployees       map[int]SidebarEmployee
	groups                 []MediumGroup
	nameExists             bool
	createID               int
	lastCorpID             int
	lastSidebarEmployeeID  int
	lastNameCheckCorpID    int
	lastNameCheckName      string
	lastNameCheckExcludeID int
	created                MediumGroupWrite
	updatedGroupID         int
	updated                MediumGroupWrite
	deletedCorpID          int
	deletedGroupID         int
}

func (s *fakeMediumGroupStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeMediumGroupStore) SidebarEmployeeByID(_ context.Context, employeeID int) (SidebarEmployee, bool, error) {
	s.lastSidebarEmployeeID = employeeID
	employee, ok := s.sidebarEmployees[employeeID]
	return employee, ok, nil
}

func (s *fakeMediumGroupStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 0, nil
}

func (s *fakeMediumGroupStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 0, 0, false, nil
}

func (s *fakeMediumGroupStore) MediumGroupsByCorpID(_ context.Context, corpID int) ([]MediumGroup, error) {
	s.lastCorpID = corpID
	return s.groups, nil
}

func (s *fakeMediumGroupStore) MediumGroupNameExists(_ context.Context, corpID int, name string, excludeGroupID int) (bool, error) {
	s.lastNameCheckCorpID = corpID
	s.lastNameCheckName = name
	s.lastNameCheckExcludeID = excludeGroupID
	return s.nameExists, nil
}

func (s *fakeMediumGroupStore) CreateMediumGroup(_ context.Context, values MediumGroupWrite) (int, error) {
	s.created = values
	if s.createID == 0 {
		return 1, nil
	}
	return s.createID, nil
}

func (s *fakeMediumGroupStore) UpdateMediumGroup(_ context.Context, groupID int, values MediumGroupWrite) (bool, error) {
	s.updatedGroupID = groupID
	s.updated = values
	return true, nil
}

func (s *fakeMediumGroupStore) DeleteMediumGroupReassignMedia(_ context.Context, corpID int, groupID int) (bool, error) {
	s.deletedCorpID = corpID
	s.deletedGroupID = groupID
	return true, nil
}
