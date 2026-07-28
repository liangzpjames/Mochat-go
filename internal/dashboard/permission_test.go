package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPermissionByUserReturnsSuperAdminMenuTree(t *testing.T) {
	store := &fakePermissionStore{
		user: User{ID: 7, TenantID: 1, IsSuperAdmin: 1},
		menus: []Menu{
			{ID: 1, Name: "企微管理", ParentID: 0, Level: 1, LinkURL: "/dashboard_baseSysManager", IsPageMenu: 1, Sort: 1},
			{ID: 2, Name: "客户管理", ParentID: 1, Level: 2, LinkURL: "/dashboard_baseContact", IsPageMenu: 1, Sort: 2},
			{ID: 3, Name: "客户列表", ParentID: 2, Level: 3, LinkURL: "/dashboard/workContact/index", IsPageMenu: 1, Sort: 3},
			{ID: 4, Name: "删除客户", ParentID: 3, Level: 4, LinkURL: "/dashboard/workContact/index@delete", IsPageMenu: 2, Sort: 4},
		},
	}
	handler := NewPermissionByUserHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/permissionByUser", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodePermissionResponse(t, rec.Body.Bytes())
	if body.Code != 200 {
		t.Fatalf("code = %d", body.Code)
	}
	if len(body.Data) != 1 {
		t.Fatalf("root menu count = %d", len(body.Data))
	}
	if body.Data[0].MenuID != 1 {
		t.Fatalf("menuId = %d", body.Data[0].MenuID)
	}
	if body.Data[0].LinkURL != "_baseSysManager" {
		t.Fatalf("linkUrl = %q", body.Data[0].LinkURL)
	}
	if len(body.Data[0].Children) != 1 {
		t.Fatalf("child count = %d", len(body.Data[0].Children))
	}
	if len(body.Data[0].Children[0].Children) != 1 {
		t.Fatalf("section children = %+v", body.Data[0].Children[0].Children)
	}
	if body.Data[0].Children[0].Children[0].LinkURL != "/workContact/index" {
		t.Fatalf("page linkUrl = %q", body.Data[0].Children[0].Children[0].LinkURL)
	}
	page := body.Data[0].Children[0].Children[0]
	if len(page.Children) != 1 || page.Children[0].LinkURL != "/workContact/index@delete" {
		t.Fatalf("page actions = %+v", page.Children)
	}
}

func TestPermissionByUserReturnsRoleMenus(t *testing.T) {
	store := &fakePermissionStore{
		user:    User{ID: 7, TenantID: 1, IsSuperAdmin: 0},
		roleID:  12,
		menuIDs: []int{1, 2},
		menusByID: []Menu{
			{ID: 1, Name: "企微管理", ParentID: 0, LinkURL: "/dashboard_baseSysManager", IsPageMenu: 1},
			{ID: 2, Name: "客户管理", ParentID: 1, LinkURL: "/dashboard/workContact/index", IsPageMenu: 1},
		},
	}
	handler := NewPermissionByUserHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/permissionByUser", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.roleUserID != 7 || store.roleTenantID != 1 {
		t.Fatalf("role lookup = user %d tenant %d", store.roleUserID, store.roleTenantID)
	}
	if store.menuRoleID != 12 {
		t.Fatalf("menu roleID = %d", store.menuRoleID)
	}
	body := decodePermissionResponse(t, rec.Body.Bytes())
	if len(body.Data) != 1 || len(body.Data[0].Children) != 1 {
		t.Fatalf("unexpected tree: %+v", body.Data)
	}
}

func TestPermissionByUserAddsAutoTagIndexRegistrationChildren(t *testing.T) {
	store := &fakePermissionStore{
		user: User{ID: 7, TenantID: 1, IsSuperAdmin: 1},
		menus: []Menu{
			{ID: 1, Name: "企微管理", ParentID: 0, Level: 1, LinkURL: "/dashboard_baseSysManager", IsPageMenu: 1, Sort: 1},
			{ID: 14, Name: "客户运营", ParentID: 1, Level: 2, LinkURL: "/dashboard_baseCustomerOperation", IsPageMenu: 1, Sort: 2},
			{ID: 299, Name: "关键词打标签", ParentID: 14, Level: 3, LinkURL: "/dashboard/autoTag/keywordIndex", IsPageMenu: 1, Sort: 3},
			{ID: 302, Name: "创建项目", ParentID: 299, Level: 4, LinkURL: "/dashboard/autoTag/keywordCreate", IsPageMenu: 1, Sort: 4},
			{ID: 303, Name: "详情", ParentID: 299, Level: 4, LinkURL: "/dashboard/autoTag/keywordShow", IsPageMenu: 1, Sort: 5},
			{ID: 304, Name: "客户入群行为打标签", ParentID: 14, Level: 3, LinkURL: "/dashboard/autoTag/joinRoomIndex", IsPageMenu: 1, Sort: 6},
			{ID: 307, Name: "分时段打标签", ParentID: 14, Level: 3, LinkURL: "/dashboard/autoTag/dayPartIndex", IsPageMenu: 1, Sort: 7},
		},
	}
	handler := NewPermissionByUserHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/permissionByUser", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodePermissionResponse(t, rec.Body.Bytes())
	keywordMenu := findMenuByLink(body.Data, "/autoTag/keywordIndex")
	if keywordMenu == nil {
		t.Fatalf("missing keyword auto-tag menu: %+v", body.Data)
	}
	if len(keywordMenu.Children) < 3 {
		t.Fatalf("keyword auto-tag child count = %d, want synthetic index plus create/show", len(keywordMenu.Children))
	}
	if keywordMenu.Children[0].LinkURL != "/autoTag/keywordIndex" {
		t.Fatalf("first keyword child linkUrl = %q", keywordMenu.Children[0].LinkURL)
	}
	if keywordMenu.Children[0].ParentID != keywordMenu.ID {
		t.Fatalf("synthetic child parent = %d, want %d", keywordMenu.Children[0].ParentID, keywordMenu.ID)
	}
	if findDirectChildByLink(keywordMenu.Children, "/autoTag/keywordCreate") == nil {
		t.Fatalf("keyword create child was not preserved: %+v", keywordMenu.Children)
	}
	if findDirectChildByLink(keywordMenu.Children, "/autoTag/keywordShow") == nil {
		t.Fatalf("keyword show child was not preserved: %+v", keywordMenu.Children)
	}
	for _, link := range []string{"/autoTag/joinRoomIndex", "/autoTag/dayPartIndex"} {
		menu := findMenuByLink(body.Data, link)
		if menu == nil {
			t.Fatalf("missing auto-tag menu %s: %+v", link, body.Data)
		}
		if len(menu.Children) == 0 || menu.Children[0].LinkURL != link {
			t.Fatalf("auto-tag menu %s did not get self registration child: %+v", link, menu.Children)
		}
	}
}

func TestPermissionByUserRejectsRoleWithoutMenus(t *testing.T) {
	store := &fakePermissionStore{
		user:   User{ID: 7, TenantID: 1},
		roleID: 12,
	}
	handler := NewPermissionByUserHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/permissionByUser", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

type permissionResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data []Menu `json:"data"`
}

func decodePermissionResponse(t *testing.T, raw []byte) permissionResponse {
	t.Helper()
	var body permissionResponse
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func findMenuByLink(menus []Menu, link string) *Menu {
	for i := range menus {
		if menus[i].LinkURL == link {
			return &menus[i]
		}
		if found := findMenuByLink(menus[i].Children, link); found != nil {
			return found
		}
	}
	return nil
}

func findDirectChildByLink(menus []Menu, link string) *Menu {
	for i := range menus {
		if menus[i].LinkURL == link {
			return &menus[i]
		}
	}
	return nil
}

type fakePermissionStore struct {
	user         User
	roleID       int
	roleUserID   int
	roleTenantID int
	menuRoleID   int
	menuIDs      []int
	menus        []Menu
	menusByID    []Menu
}

func (s *fakePermissionStore) UserByID(context.Context, int) (User, bool, error) {
	return s.user, s.user.ID != 0, nil
}

func (s *fakePermissionStore) RoleIDByUserTenant(_ context.Context, userID int, tenantID int) (int, bool, error) {
	s.roleUserID = userID
	s.roleTenantID = tenantID
	return s.roleID, s.roleID != 0, nil
}

func (s *fakePermissionStore) MenuIDsByRole(_ context.Context, roleID int) ([]int, error) {
	s.menuRoleID = roleID
	return s.menuIDs, nil
}

func (s *fakePermissionStore) PageMenus(context.Context) ([]Menu, error) {
	return s.menus, nil
}

func (s *fakePermissionStore) MenusByIDs(context.Context, []int) ([]Menu, error) {
	return s.menusByID, nil
}
