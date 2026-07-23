package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoleSelectReturnsTenantRoles(t *testing.T) {
	store := &fakeRoleSelectStore{
		users: map[int]User{1: {ID: 1, TenantID: 10}},
		rolesByTenant: map[int][]RoleOption{
			10: {
				{ID: 3, Name: "管理员"},
				{ID: 4, Name: "客服"},
			},
		},
	}
	handler := NewRoleSelectHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/select", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastTenantID != 10 {
		t.Fatalf("lastTenantID = %d", store.lastTenantID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("data = %#v", data)
	}
	first := data[0].(map[string]any)
	if int(first["roleId"].(float64)) != 3 || first["name"] != "管理员" {
		t.Fatalf("first = %#v", first)
	}
}

func TestRoleSelectReturnsEmptyArrayWhenNoRoles(t *testing.T) {
	store := &fakeRoleSelectStore{users: map[int]User{1: {ID: 1, TenantID: 10}}}
	handler := NewRoleSelectHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/select", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if data := body["data"].([]any); len(data) != 0 {
		t.Fatalf("data = %#v", data)
	}
}

type fakeRoleSelectStore struct {
	users         map[int]User
	rolesByTenant map[int][]RoleOption
	lastTenantID  int
}

func (s *fakeRoleSelectStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeRoleSelectStore) RolesByTenantID(_ context.Context, tenantID int) ([]RoleOption, error) {
	s.lastTenantID = tenantID
	return s.rolesByTenant[tenantID], nil
}
