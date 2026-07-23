package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCorpSelectReturnsTenantCorpsForSuperAdmin(t *testing.T) {
	store := &fakeCorpStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 10, IsSuperAdmin: 1},
		},
		tenantCorpIDs: map[int][]int{10: {3, 4}},
		corps: map[int]Corp{
			3: {ID: 3, Name: "华泰汽车"},
			4: {ID: 4, Name: "迁移测试企业"},
		},
	}
	handler := NewCorpSelectHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corp/select?corpName=迁移", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	data := body.Data.([]any)
	if len(data) != 1 {
		t.Fatalf("len(data) = %d, body=%s", len(data), rec.Body.String())
	}
	corp := data[0].(map[string]any)
	if int(corp["corpId"].(float64)) != 4 || corp["corpName"] != "迁移测试企业" {
		t.Fatalf("corp = %#v", corp)
	}
}

func TestCorpSelectReturnsBoundCorpsForNormalUser(t *testing.T) {
	store := &fakeCorpStore{
		users: map[int]User{
			2: {ID: 2, TenantID: 10, IsSuperAdmin: 0},
		},
		userCorpIDs: map[int][]int{2: {5}},
		corps:       map[int]Corp{5: {ID: 5, Name: "普通用户企业"}},
	}
	handler := NewCorpSelectHandler(store, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corp/select", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	data := body.Data.([]any)
	if len(data) != 1 {
		t.Fatalf("len(data) = %d, body=%s", len(data), rec.Body.String())
	}
}

func TestCorpBindWritesSuperAdminCache(t *testing.T) {
	store := &fakeCorpStore{
		users:         map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		tenantCorpIDs: map[int][]int{10: {3}},
	}
	cache := &fakeCorpCache{}
	handler := NewCorpBindHandler(store, cache, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/corp/bind", bytes.NewBufferString(`{"corpId":3}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if cache.userID != 1 || cache.value != "3-0" {
		t.Fatalf("cache = userID %d value %q", cache.userID, cache.value)
	}
}

func TestCorpBindRejectsSuperAdminCrossTenantCorp(t *testing.T) {
	store := &fakeCorpStore{
		users:         map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		tenantCorpIDs: map[int][]int{10: {3}},
	}
	cache := &fakeCorpCache{}
	handler := NewCorpBindHandler(store, cache, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/corp/bind", bytes.NewBufferString(`{"corpId":7}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if cache.value != "" {
		t.Fatalf("cache value = %q", cache.value)
	}
}

func TestCorpBindRequiresNormalUserMembership(t *testing.T) {
	store := &fakeCorpStore{
		users:       map[int]User{2: {ID: 2, IsSuperAdmin: 0}},
		userCorpIDs: map[int][]int{2: {5}},
		employeeIDs: map[[2]int]int{
			{2, 5}: 9,
		},
	}
	cache := &fakeCorpCache{}
	handler := NewCorpBindHandler(store, cache, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/corp/bind", bytes.NewBufferString("corpId=7"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	req = httptest.NewRequest(http.MethodPost, "/dashboard/corp/bind", bytes.NewBufferString("corpId=5"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if cache.value != "5-9" {
		t.Fatalf("cache value = %q", cache.value)
	}
}

func TestCorpBindReportsCacheFailure(t *testing.T) {
	store := &fakeCorpStore{
		users:         map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		tenantCorpIDs: map[int][]int{10: {3}},
	}
	handler := NewCorpBindHandler(store, &fakeCorpCache{err: errors.New("redis down")}, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/corp/bind", bytes.NewBufferString(`{"corpId":3}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

type fakeCorpStore struct {
	users         map[int]User
	tenantCorpIDs map[int][]int
	userCorpIDs   map[int][]int
	corps         map[int]Corp
	employeeIDs   map[[2]int]int
}

func (s *fakeCorpStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeCorpStore) CorpIDsByTenant(_ context.Context, tenantID int) ([]int, error) {
	return append([]int{}, s.tenantCorpIDs[tenantID]...), nil
}

func (s *fakeCorpStore) CorpIDsByUser(_ context.Context, userID int) ([]int, error) {
	return append([]int{}, s.userCorpIDs[userID]...), nil
}

func (s *fakeCorpStore) CorpsByIDsName(_ context.Context, corpIDs []int, name string) ([]Corp, error) {
	var out []Corp
	for _, corpID := range corpIDs {
		corp, ok := s.corps[corpID]
		if !ok {
			continue
		}
		if name != "" && !strings.Contains(corp.Name, name) {
			continue
		}
		out = append(out, corp)
	}
	return out, nil
}

func (s *fakeCorpStore) EmployeeIDByUserCorp(_ context.Context, userID int, corpID int) (int, error) {
	return s.employeeIDs[[2]int{userID, corpID}], nil
}

type fakeCorpCache struct {
	userID int
	value  string
	err    error
}

func (c *fakeCorpCache) SetUserCorpCache(_ context.Context, userID int, value string) error {
	c.userID = userID
	c.value = value
	return c.err
}
