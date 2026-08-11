package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginShowReturnsPHPCompatibleEnvelope(t *testing.T) {
	store := &fakeStore{
		user: User{
			ID: 7, Phone: "13800138000", Name: "张三", Gender: 1, Department: "销售",
			Position: "主管", LoginTime: "2026-07-02 12:00:00", Status: 1, TenantID: 1,
		},
		employee: Employee{
			ID: 34, Name: "员工张三", Mobile: "13800138000", Position: "销售主管",
			Gender: 1, Email: "zhangsan@example.com", Avatar: "/avatar.png",
			ThumbAvatar: "/thumb.png", Telephone: "010-10000000", Alias: "zs",
			Status: 1, QRCode: "/qr.png", ExternalPosition: "顾问", Address: "北京",
		},
		corp: Corp{ID: 12, Name: "测试企业"},
	}
	handler := NewLoginShowHandler(store, staticCache("12-34"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/user/loginShow", nil, 7, 1, 12, 34)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 200 {
		t.Fatalf("code = %d", body.Code)
	}
	if body.Data["userName"] != "张三" {
		t.Fatalf("userName = %v", body.Data["userName"])
	}
	if body.Data["employeeName"] != "员工张三" {
		t.Fatalf("employeeName = %v", body.Data["employeeName"])
	}
	if body.Data["corpName"] != "测试企业" {
		t.Fatalf("corpName = %v", body.Data["corpName"])
	}
}

func TestLoginShowIgnoresCrossTenantCachedCorp(t *testing.T) {
	store := &fakeStore{
		user: User{
			ID: 7, Phone: "13800138000", Name: "张三", TenantID: 10, IsSuperAdmin: 1,
		},
		employee:      Employee{ID: 34, Name: "员工张三"},
		corp:          Corp{ID: 12, Name: "测试企业"},
		tenantCorpIDs: []int{12},
	}
	handler := NewLoginShowHandler(store, staticCache("99-0"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/user/loginShow", nil, 7, 10, 12, 34)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if int(body.Data["corpId"].(float64)) != 12 {
		t.Fatalf("corpId = %#v", body.Data["corpId"])
	}
	if int(body.Data["employeeId"].(float64)) != 34 {
		t.Fatalf("employeeId = %#v", body.Data["employeeId"])
	}
}

func TestLoginShowRejectsMissingUserID(t *testing.T) {
	handler := NewLoginShowHandler(&fakeStore{}, nil, HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/user/loginShow", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginShowRejectsDisabledTenant(t *testing.T) {
	store := &fakeStore{
		user: User{ID: 7, Phone: "13800138000", Name: "张三", Status: 1, TenantID: 902, TenantStatus: 2},
	}
	handler := NewLoginShowHandler(store, nil, HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/user/loginShow", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var body struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Msg != "租户已停用" {
		t.Fatalf("msg = %q", body.Msg)
	}
}

func TestLoginShowRejectsExpiredTenantPackage(t *testing.T) {
	store := &fakeStore{
		user: User{ID: 7, Phone: "13800138000", Name: "张三", Status: 1, TenantID: 902, TenantPackageExpired: true, TenantPackageExpiresAt: "2026-07-01 00:00:00"},
	}
	handler := NewLoginShowHandler(store, nil, HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/user/loginShow", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var body struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Msg != "租户套餐已到期" {
		t.Fatalf("msg = %q", body.Msg)
	}
}

func TestLoginShowRejectsBlockedManagedSubscription(t *testing.T) {
	store := &fakeStore{user: User{
		ID: 7, Status: 1, TenantID: 902, TenantSubscriptionManaged: true,
		TenantSubscriptionStatus:        SaaSAdminSubscriptionStatusSuspended,
		TenantSubscriptionAccessAllowed: false,
	}}
	handler := NewLoginShowHandler(store, nil, HeaderUserIDResolver{})
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/user/loginShow", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "租户订阅已暂停") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type staticCache string

func (c staticCache) UserCorpCache(context.Context, int) (string, error) {
	return string(c), nil
}

type fakeStore struct {
	user          User
	employee      Employee
	corp          Corp
	tenantCorpIDs []int
	userCorpIDs   []int
}

func (s *fakeStore) UserByID(context.Context, int) (User, bool, error) {
	return s.user, s.user.ID != 0, nil
}

func (s *fakeStore) EmployeeByID(_ context.Context, employeeID int) (Employee, bool, error) {
	return s.employee, s.employee.ID != 0 && s.employee.ID == employeeID, nil
}

func (s *fakeStore) CorpByID(_ context.Context, corpID int) (Corp, bool, error) {
	return s.corp, s.corp.ID != 0 && s.corp.ID == corpID, nil
}

func (s *fakeStore) EmployeeIDByUserCorp(_ context.Context, _ int, corpID int) (int, error) {
	if s.corp.ID != 0 && s.corp.ID == corpID {
		return s.employee.ID, nil
	}
	return 0, nil
}

func (s *fakeStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	if s.employee.ID == 0 || s.corp.ID == 0 {
		return 0, 0, false, nil
	}
	return s.corp.ID, s.employee.ID, true, nil
}

func (s *fakeStore) CorpIDsByTenant(context.Context, int) ([]int, error) {
	if s.tenantCorpIDs != nil {
		return append([]int{}, s.tenantCorpIDs...), nil
	}
	if s.corp.ID == 0 {
		return []int{}, nil
	}
	return []int{s.corp.ID}, nil
}

func (s *fakeStore) CorpIDsByUser(context.Context, int) ([]int, error) {
	if s.userCorpIDs != nil {
		return append([]int{}, s.userCorpIDs...), nil
	}
	if s.corp.ID == 0 {
		return []int{}, nil
	}
	return []int{s.corp.ID}, nil
}
