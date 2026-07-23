package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMenuAdminIndexReturnsPagedTreeAndAuthorizes(t *testing.T) {
	store := &fakeMenuAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 10, IsSuperAdmin: 1},
		},
		items: []MenuListItem{
			{ID: 1, Name: "企微管理", Level: 1, ParentID: 0, Icon: "server", Status: 1, OperateName: "admin", UpdatedAt: "2026-07-02 10:00:00"},
			{ID: 2, Name: "菜单管理", Level: 2, ParentID: 1, Icon: "menu", Status: 1, OperateName: "admin", UpdatedAt: "2026-07-02 10:01:00"},
			{ID: 3, Name: "系统设置", Level: 1, ParentID: 0, Icon: "settings", Status: 2, OperateName: "admin", UpdatedAt: "2026-07-02 10:02:00"},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewMenuAdminHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, authorizer)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/menu/index?name=菜单&page=1&perPage=1", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/menu/index#get" {
		t.Fatalf("permission key = %q", authorizer.permissionKey)
	}
	if authorizer.corpID != 7 || authorizer.workEmployeeID != 9 {
		t.Fatalf("auth context = corp %d employee %d", authorizer.corpID, authorizer.workEmployeeID)
	}
	if store.lastFilter.Name != "菜单" {
		t.Fatalf("filter = %+v", store.lastFilter)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if int(page["total"].(float64)) != 2 || int(page["totalPage"].(float64)) != 2 || int(page["perPage"].(float64)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	list := data["list"].([]any)
	if len(list) != 1 {
		t.Fatalf("list len = %d", len(list))
	}
	first := list[0].(map[string]any)
	if first["menuPath"] != "1" || first["levelName"] != "一级菜单" {
		t.Fatalf("first = %#v", first)
	}
	children := first["children"].([]any)
	if len(children) != 1 {
		t.Fatalf("children = %#v", children)
	}
	child := children[0].(map[string]any)
	if child["menuPath"] != "1-1" || child["levelName"] != "二级菜单" {
		t.Fatalf("child = %#v", child)
	}
}

func TestMenuAdminShowReturnsPHPCompatibleDetail(t *testing.T) {
	store := &fakeMenuAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		details: map[int]MenuDetail{
			4: {
				ID:             4,
				Name:           "菜单详情",
				Level:          3,
				Status:         1,
				Icon:           "menu",
				LinkURL:        "/dashboard/menu/show#get",
				IsPageMenu:     1,
				LinkType:       1,
				DataPermission: 2,
				Path:           "#1#-#2#-#4#",
			},
		},
	}
	handler := NewMenuAdminHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/menu/show?menuId=4", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["menuId"].(float64)) != 4 || data["levelName"] != "三级菜单" {
		t.Fatalf("data = %#v", data)
	}
	if int(data["firstMenuId"].(float64)) != 1 || int(data["secondMenuId"].(float64)) != 2 || int(data["thirdMenuId"].(float64)) != 4 || data["fourthMenuId"] != "" {
		t.Fatalf("path fields = %#v", data)
	}
	if int(data["linkType"].(float64)) != 1 || int(data["dataPermission"].(float64)) != 2 {
		t.Fatalf("permission fields = %#v", data)
	}
}

func TestMenuAdminShowRejectsPermissionDenied(t *testing.T) {
	store := &fakeMenuAdminStore{
		users:   map[int]User{2: {ID: 2, TenantID: 10, IsSuperAdmin: 0}},
		details: map[int]MenuDetail{4: {ID: 4}},
	}
	handler := NewMenuAdminHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, &recordingAuthorizer{err: ErrPermissionDenied})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/menu/show?menuId=4", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.detailLookups != 0 {
		t.Fatalf("detail should not be queried after permission denied")
	}
}

func TestMenuAdminStoreCreatesThirdLevelMenu(t *testing.T) {
	store := &fakeMenuAdminStore{
		users: map[int]User{1: {ID: 1, Name: "管理员", TenantID: 10, IsSuperAdmin: 1}},
	}
	handler := NewMenuAdminHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/menu/store", strings.NewReader(`{"name":"客户列表","level":3,"firstMenuId":1,"secondMenuId":2,"linkType":1,"linkUrl":"/dashboard/workContact/index","dataPermission":2}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.createdMenu.Name != "客户列表" || store.createdMenu.Level != 3 || store.createdMenu.ParentID != 2 || store.createdMenu.Path != "#1#-#2#" {
		t.Fatalf("created menu = %+v", store.createdMenu)
	}
	if store.createdMenu.LinkURL != "/dashboard/workContact/index" || store.createdMenu.DataPermission != 2 {
		t.Fatalf("created menu link fields = %+v", store.createdMenu)
	}
}

func TestMenuAdminStatusUpdateDisablesCascade(t *testing.T) {
	store := &fakeMenuAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
	}
	handler := NewMenuAdminHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodPut, "/dashboard/menu/statusUpdate", strings.NewReader(`{"menuId":4,"status":2}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.StatusUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.statusMenuID != 4 || store.statusValue != 2 || !store.statusCascade {
		t.Fatalf("status update = menu %d status %d cascade %v", store.statusMenuID, store.statusValue, store.statusCascade)
	}
}

type fakeMenuAdminStore struct {
	users         map[int]User
	items         []MenuListItem
	details       map[int]MenuDetail
	lastFilter    MenuListFilter
	detailLookups int
	linkExists    bool
	createdMenu   MenuCreateValues
	updatedMenuID int
	updatedMenu   MenuUpdateValues
	statusMenuID  int
	statusValue   int
	statusCascade bool
	deletedMenuID int
}

func (s *fakeMenuAdminStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeMenuAdminStore) EmployeeIDByUserCorp(_ context.Context, userID int, corpID int) (int, error) {
	return 0, nil
}

func (s *fakeMenuAdminStore) FirstEmployeeByUser(_ context.Context, userID int) (int, int, bool, error) {
	return 0, 0, false, nil
}

func (s *fakeMenuAdminStore) MenuList(_ context.Context, filter MenuListFilter) ([]MenuListItem, error) {
	s.lastFilter = filter
	return s.items, nil
}

func (s *fakeMenuAdminStore) MenuDetailByID(_ context.Context, menuID int) (MenuDetail, bool, error) {
	s.detailLookups++
	menu, ok := s.details[menuID]
	return menu, ok, nil
}

func (s *fakeMenuAdminStore) MenuLinkURLExists(_ context.Context, linkURL string, excludeMenuID int) (bool, error) {
	return s.linkExists, nil
}

func (s *fakeMenuAdminStore) CreateMenu(_ context.Context, values MenuCreateValues) (int, error) {
	s.createdMenu = values
	return 101, nil
}

func (s *fakeMenuAdminStore) UpdateMenu(_ context.Context, menuID int, values MenuUpdateValues) (bool, error) {
	s.updatedMenuID = menuID
	s.updatedMenu = values
	return true, nil
}

func (s *fakeMenuAdminStore) UpdateMenuStatus(_ context.Context, menuID int, status int, cascade bool) (bool, error) {
	s.statusMenuID = menuID
	s.statusValue = status
	s.statusCascade = cascade
	return true, nil
}

func (s *fakeMenuAdminStore) DeleteMenuCascade(_ context.Context, menuID int) (bool, error) {
	s.deletedMenuID = menuID
	return true, nil
}
