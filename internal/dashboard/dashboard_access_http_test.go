package dashboard

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

func TestDashboardAccessHTTPRoutes(t *testing.T) {
	service, store := newDashboardAccessAdminFixture()
	store.users = DashboardAccessUserPage{List: []DashboardAccessUserSummary{{ID: 7}}, Page: DashboardAccessPage{Page: 1, PerPage: 20, Total: 1, TotalPage: 1}}
	store.roles = DashboardAccessRolePage{List: []DashboardAccessRoleDetail{store.role}, Page: DashboardAccessPage{Page: 1, PerPage: 20, Total: 1, TotalPage: 1}}
	store.audits = DashboardPermissionAuditPage{List: []DashboardPermissionAudit{{ID: 1}}, Page: DashboardAccessPage{Page: 1, PerPage: 20, Total: 1, TotalPage: 1}}
	handler := NewDashboardAccessHTTP(service)
	for _, test := range []struct {
		name, method, path, body string
		wantStatus               int
	}{
		{name: "profile", method: http.MethodGet, path: "/dashboard/access/profile", wantStatus: http.StatusOK},
		{name: "catalog", method: http.MethodGet, path: "/dashboard/access/catalog", wantStatus: http.StatusOK},
		{name: "users", method: http.MethodGet, path: "/dashboard/access/users?page=1&perPage=20", wantStatus: http.StatusOK},
		{name: "user", method: http.MethodGet, path: "/dashboard/access/users/7", wantStatus: http.StatusOK},
		{name: "replace user", method: http.MethodPut, path: "/dashboard/access/users/7", body: `{"roleIds":[8],"directPermissions":[{"code":"dashboard.index","scope":"self"}],"expectedVersion":3}`, wantStatus: http.StatusOK},
		{name: "roles", method: http.MethodGet, path: "/dashboard/access/roles", wantStatus: http.StatusOK},
		{name: "create role", method: http.MethodPost, path: "/dashboard/access/roles", body: `{"name":"销售","remark":"普通角色","status":1,"permissions":[{"code":"dashboard.index","scope":"department"}]}`, wantStatus: http.StatusCreated},
		{name: "update role", method: http.MethodPut, path: "/dashboard/access/roles/8", body: `{"name":"销售","remark":"普通角色","permissions":[{"code":"dashboard.index","scope":"department"}],"expectedVersion":4}`, wantStatus: http.StatusOK},
		{name: "role status", method: http.MethodPut, path: "/dashboard/access/roles/8/status", body: `{"status":2,"expectedVersion":4}`, wantStatus: http.StatusOK},
		{name: "delete role", method: http.MethodDelete, path: "/dashboard/access/roles/8", body: `{"expectedVersion":4}`, wantStatus: http.StatusOK},
		{name: "audits", method: http.MethodGet, path: "/dashboard/access/audits?page=1&perPage=20", wantStatus: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := dashboardAccessHTTPTestRequest(test.method, test.path, bytes.NewBufferString(test.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			var envelope struct {
				Code int             `json:"code"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Code != 200 || len(envelope.Data) == 0 {
				t.Fatalf("envelope=%+v error=%v body=%s", envelope, err, response.Body.String())
			}
		})
	}
}

func TestDashboardAccessHTTPProfilePublishesBindingStatus(t *testing.T) {
	service, _ := newDashboardAccessAdminFixture()
	handler := NewDashboardAccessHTTP(service)
	request := dashboardAccessHTTPTestRequest(http.MethodGet, "/dashboard/access/profile", nil)
	principal, err := dashboardprincipal.DashboardPrincipalFromContext(request.Context())
	if err != nil {
		t.Fatal(err)
	}
	principal.CorpStatus = dashboardprincipal.CorpBindingStatusPending
	request = request.WithContext(dashboardprincipal.WithPrincipal(request.Context(), principal))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data struct {
			CorpBindingStatus string `json:"corpBindingStatus"`
			CorpName          string `json:"corpName"`
			UserName          string `json:"userName"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.CorpBindingStatus != "pending" || envelope.Data.CorpName != "极义科技" || envelope.Data.UserName != "管理员" {
		t.Fatalf("profile=%+v body=%s", envelope.Data, response.Body.String())
	}
}

func TestDashboardAccessHTTPUsesDashboardPrincipalInsteadOfLegacyResolver(t *testing.T) {
	service, _ := newDashboardAccessAdminFixture()
	request := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/access/catalog", nil, 1, 9, 12, 81)
	request.Header.Set("X-Mochat-Go-User-ID", "2")
	handler := NewDashboardAccessHTTP(service)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDashboardAccessHTTPRejectsActorAndTenantFields(t *testing.T) {
	service, _ := newDashboardAccessAdminFixture()
	handler := NewDashboardAccessHTTP(service)
	for _, field := range []string{"tenantId", "tenant_id", "operateId", "operateName", "actorUserId"} {
		body := `{"name":"销售","status":1,"permissions":[],"` + field + `":99}`
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, dashboardAccessHTTPTestRequest(http.MethodPost, "/dashboard/access/roles", bytes.NewBufferString(body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("field=%s status=%d body=%s", field, response.Code, response.Body.String())
		}
	}
}

func TestDashboardAccessHTTPStrictlyDecodesEveryWriteShape(t *testing.T) {
	service, _ := newDashboardAccessAdminFixture()
	handler := NewDashboardAccessHTTP(service)
	for _, test := range []struct{ method, path, body string }{
		{http.MethodPut, "/dashboard/access/users/7", `{"expectedVersion":3,"tenantId":9}`},
		{http.MethodPost, "/dashboard/access/roles", `{"name":"sales","status":1,"actorUserId":1}`},
		{http.MethodPut, "/dashboard/access/roles/8", `{"name":"sales","expectedVersion":4,"operateName":"admin"}`},
		{http.MethodPut, "/dashboard/access/roles/8/status", `{"status":2,"expectedVersion":4,"tenant_id":9}`},
		{http.MethodDelete, "/dashboard/access/roles/8", `{"expectedVersion":4,"operateId":1}`},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, dashboardAccessHTTPTestRequest(test.method, test.path, bytes.NewBufferString(test.body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
	}
}

func TestDashboardAccessHTTPMapsPermissionNotFoundAndConflicts(t *testing.T) {
	service, store := newDashboardAccessAdminFixture()
	t.Run("ordinary management", func(t *testing.T) {
		handler := NewDashboardAccessHTTP(service)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, dashboardAccessHTTPTestRequestAs(http.MethodGet, "/dashboard/access/catalog", nil, 2))
		if response.Code != http.StatusForbidden || machineCode(t, response) != DashboardPermissionDeniedCode {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})
	t.Run("cross tenant hidden", func(t *testing.T) {
		store.userFound = false
		handler := NewDashboardAccessHTTP(service)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, dashboardAccessHTTPTestRequest(http.MethodGet, "/dashboard/access/users/7", nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		store.userFound = true
	})
	t.Run("cross tenant role hidden", func(t *testing.T) {
		store.writeErr = ErrDashboardAccessAdminNotFound
		handler := NewDashboardAccessHTTP(service)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, dashboardAccessHTTPTestRequest(http.MethodPut, "/dashboard/access/roles/8", bytes.NewBufferString(`{"name":"sales","expectedVersion":4}`)))
		if response.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		store.writeErr = nil
	})
	t.Run("version conflict", func(t *testing.T) {
		store.writeErr = ErrDashboardAccessAdminConflict
		handler := NewDashboardAccessHTTP(service)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, dashboardAccessHTTPTestRequest(http.MethodPut, "/dashboard/access/users/7", bytes.NewBufferString(`{"expectedVersion":3}`)))
		if response.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})
	t.Run("role has members", func(t *testing.T) {
		store.writeErr = ErrDashboardAccessRoleHasMembers
		handler := NewDashboardAccessHTTP(service)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, dashboardAccessHTTPTestRequest(http.MethodDelete, "/dashboard/access/roles/8", bytes.NewBufferString(`{"expectedVersion":4}`)))
		if response.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})
}

func TestDashboardAccessHTTPStableUnauthorizedAndInternalErrors(t *testing.T) {
	service, store := newDashboardAccessAdminFixture()
	response := httptest.NewRecorder()
	NewDashboardAccessHTTP(service).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/dashboard/access/profile", nil),
	)
	if response.Code != http.StatusUnauthorized || machineCode(t, response) != "UNAUTHORIZED" {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	store.writeErr = errors.New("database unavailable")
	response = httptest.NewRecorder()
	NewDashboardAccessHTTP(service).ServeHTTP(
		response,
		dashboardAccessHTTPTestRequest(http.MethodPut, "/dashboard/access/users/7", bytes.NewBufferString(`{"expectedVersion":3}`)),
	)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDashboardAccessHTTPWriteResponsesExposeLatestVersionWithoutConflictOverwrite(t *testing.T) {
	service, store := newDashboardAccessAdminFixture()
	store.user.Version = 9
	handler := NewDashboardAccessHTTP(service)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, dashboardAccessHTTPTestRequest(http.MethodPut, "/dashboard/access/users/7", bytes.NewBufferString(`{"expectedVersion":3}`)))
	var success struct {
		Data struct {
			Version uint64 `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &success); err != nil || response.Code != http.StatusOK || success.Data.Version != 9 {
		t.Fatalf("status=%d response=%+v err=%v body=%s", response.Code, success, err, response.Body.String())
	}

	store.writeErr = ErrDashboardAccessAdminConflict
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, dashboardAccessHTTPTestRequest(http.MethodPut, "/dashboard/access/users/7", bytes.NewBufferString(`{"expectedVersion":3}`)))
	var conflict struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &conflict); err != nil || response.Code != http.StatusConflict || string(conflict.Data) != "null" {
		t.Fatalf("status=%d data=%s err=%v body=%s", response.Code, conflict.Data, err, response.Body.String())
	}
}

func TestDashboardAccessHTTPRejectsUnknownPathsMethodsAndBadIDs(t *testing.T) {
	service, _ := newDashboardAccessAdminFixture()
	handler := NewDashboardAccessHTTP(service)
	for _, request := range []*http.Request{
		dashboardAccessHTTPTestRequest(http.MethodPost, "/dashboard/access/catalog", nil),
		dashboardAccessHTTPTestRequest(http.MethodGet, "/dashboard/access/profile/", nil),
		dashboardAccessHTTPTestRequest(http.MethodGet, "/dashboard/access/users/not-a-number", nil),
		dashboardAccessHTTPTestRequest(http.MethodGet, "/dashboard/access/users/7/extra", nil),
		dashboardAccessHTTPTestRequest(http.MethodGet, "/dashboard/access/unknown", nil),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound && response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s status=%d body=%s", request.Method, request.URL.Path, response.Code, response.Body.String())
		}
	}
}

func dashboardAccessHTTPTestRequest(method, target string, body io.Reader) *http.Request {
	return dashboardAccessHTTPTestRequestAs(method, target, body, 1)
}

func dashboardAccessHTTPTestRequestAs(method, target string, body io.Reader, userID int) *http.Request {
	return authenticatedDashboardRequestForTestAs(method, target, body, userID, 9, 12, 81)
}

func TestRequireTenantSuperAdmin(t *testing.T) {
	response := httptest.NewRecorder()
	if requireTenantSuperAdmin(response, User{ID: 7, TenantID: 9}) {
		t.Fatal("ordinary user accepted")
	}
	if response.Code != http.StatusForbidden || machineCode(t, response) != DashboardPermissionDeniedCode {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !requireTenantSuperAdmin(httptest.NewRecorder(), User{ID: 1, TenantID: 9, IsSuperAdmin: 1}) {
		t.Fatal("superadmin rejected")
	}
}
