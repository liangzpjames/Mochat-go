package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMenuIconIndexReturnsNonEmptyIcons(t *testing.T) {
	store := &fakeMenuReadStore{
		users: map[int]User{1: {ID: 1}},
		icons: []string{"line-chart", "pie-chart"},
	}
	handler := NewMenuReadHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/menu/iconIndex", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.IconIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	if len(data) != 2 || data[0] != "line-chart" || data[1] != "pie-chart" {
		t.Fatalf("data = %#v", data)
	}
}

func TestMenuSelectReturnsTree(t *testing.T) {
	store := &fakeMenuReadStore{
		users: map[int]User{1: {ID: 1}},
		menus: []MenuOption{
			{ID: 1, Name: "企微管理", Level: 1, ParentID: 0, DataPermission: 2},
			{ID: 2, Name: "引流获客", Level: 2, ParentID: 1, DataPermission: 2},
			{ID: 3, Name: "渠道活码", Level: 3, ParentID: 2, DataPermission: 2},
			{ID: 4, Name: "系统设置", Level: 1, ParentID: 0, DataPermission: 0},
		},
	}
	handler := NewMenuReadHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/menu/select", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Select(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("data = %#v", data)
	}
	first := data[0].(map[string]any)
	if int(first["menuId"].(float64)) != 1 || first["name"] != "企微管理" {
		t.Fatalf("first = %#v", first)
	}
	children := first["children"].([]any)
	if len(children) != 1 {
		t.Fatalf("children = %#v", children)
	}
	child := children[0].(map[string]any)
	if int(child["parentId"].(float64)) != 1 || child["name"] != "引流获客" {
		t.Fatalf("child = %#v", child)
	}
}

type fakeMenuReadStore struct {
	users map[int]User
	menus []MenuOption
	icons []string
}

func (s *fakeMenuReadStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeMenuReadStore) MenuOptions(context.Context) ([]MenuOption, error) {
	return s.menus, nil
}

func (s *fakeMenuReadStore) MenuIcons(context.Context) ([]string, error) {
	return s.icons, nil
}
