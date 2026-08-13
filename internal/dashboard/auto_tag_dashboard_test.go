package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

func withAutoTagTestPrincipal(request *http.Request, userID, tenantID, corpID int) *http.Request {
	ctx := dashboardprincipal.WithPrincipal(request.Context(), dashboardprincipal.DashboardPrincipal{
		UserID: userID, TenantID: tenantID, CorpID: corpID,
		CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 1,
	})
	return request.WithContext(WithDashboardAccessContext(ctx, DashboardAccessContext{
		UserID: userID, TenantID: tenantID, CorpID: corpID,
		WorkEmployeeID: 99, Scope: DataScopeTenant,
	}))
}

func TestAutoTagIndexReturnsListAndPagination(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		page: AutoTagPage{
			Page:    1,
			PerPage: 15,
			Total:   1,
			Items: []AutoTagItem{{
				ID:                   31,
				Type:                 1,
				Name:                 "关键词规则",
				FuzzyMatchKeywordRaw: `["报价"]`,
				ExactMatchKeywordRaw: `["下单"]`,
				TagRuleRaw:           `[{"tags":[{"tagid":7,"tagname":"意向"}]}]`,
				TagsRaw:              `["意向"]`,
				OnOff:                1,
				MarkTagCount:         3,
				CreatedAt:            "2026-07-05 10:00:00",
			}},
		},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/autoTag/index?type=1&name=关键&tags=7&page=1&perPage=15", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data struct {
			Total int              `json:"total"`
			List  []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Total != 1 || len(envelope.Data.List) != 1 {
		t.Fatalf("data = %#v", envelope.Data)
	}
	item := envelope.Data.List[0]
	if item["id"].(float64) != 31 || item["name"] != "关键词规则" || item["mark_tag_count"].(float64) != 3 {
		t.Fatalf("item = %#v", item)
	}
	if store.lastFilter.CorpID != 7 || store.lastFilter.Type != 1 || store.lastFilter.Name != "关键" || len(store.lastFilter.Tags) != 1 || store.lastFilter.Tags[0] != 7 {
		t.Fatalf("filter = %#v", store.lastFilter)
	}
}

func TestAutoTagStorePreservesRuleJSONAndFlattensTags(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, IsSuperAdmin: 1}, createID: 88}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/autoTag/store", strings.NewReader(`{"type":1,"name":"关键词规则","employees":["zhangsan"],"fuzzy_match_keyword":["报价"],"exact_match_keyword":["下单"],"tag_rule":[{"time_type":1,"trigger_count":2,"tags":[{"tagid":7,"tagname":"意向"}]}]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.Type != 1 || store.created.Name != "关键词规则" {
		t.Fatalf("created = %#v", store.created)
	}
	if !strings.Contains(store.created.TagRuleRaw, "trigger_count") || !strings.Contains(store.created.EmployeesRaw, "zhangsan") || !strings.Contains(store.created.FuzzyMatchKeywordRaw, "报价") {
		t.Fatalf("json raw = %#v", store.created)
	}
	if !strings.Contains(store.created.TagsRaw, "意向") {
		t.Fatalf("flattened tags = %s", store.created.TagsRaw)
	}
}

func TestAutoTagShowReturnsDetailAndStatistics(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		item: AutoTagItem{
			ID:           31,
			Type:         3,
			Name:         "分时段规则",
			EmployeesRaw: `["zhangsan"]`,
			TagRuleRaw:   `[{"time_type":1,"tags":[{"tagid":8,"tagname":"新客"}]}]`,
			TagsRaw:      `["新客"]`,
			OnOff:        1,
			CreatedAt:    "2026-07-05 10:00:00",
		},
		stats: AutoTagStatistics{TotalCount: 12, TodayCount: 2},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/autoTag/show?id=31", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data struct {
			AutoTag    map[string]any `json:"auto_tag"`
			Statistics map[string]any `json:"statistics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.AutoTag["name"] != "分时段规则" || envelope.Data.Statistics["total_count"].(float64) != 12 || envelope.Data.Statistics["today_count"].(float64) != 2 {
		t.Fatalf("data = %#v", envelope.Data)
	}
}

func TestWorkMessageIndexReturnsMessageList(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		messagePage: WorkMessagePage{
			Page:    1,
			PerPage: 100,
			Total:   1,
			Items: []WorkMessageItem{{
				ID:            9,
				Action:        0,
				Name:          "张三",
				IsCurrentUser: 1,
				Type:          1,
				ContentRaw:    `{"content":"你好"}`,
				MsgDataTime:   "2026-07-05 11:00:00",
			}},
		},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/index?workEmployeeId=99&toUserType=1&toUserId=31&page=1&perPage=100", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.List) != 1 || envelope.Data.List[0]["name"] != "张三" {
		t.Fatalf("list = %#v", envelope.Data.List)
	}
	content := envelope.Data.List[0]["content"].(map[string]any)
	if content["content"] != "你好" {
		t.Fatalf("content = %#v", content)
	}
	if store.lastMessageFilter.WorkEmployeeID != 99 || store.lastMessageFilter.ToUserType != 1 || store.lastMessageFilter.ToUserID != 31 {
		t.Fatalf("message filter = %#v", store.lastMessageFilter)
	}
}

func TestWorkMessageGlobalSearchReturnsExplicitPageAndForwardsAllFilters(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 1, TenantID: 10, IsSuperAdmin: 1},
		toUserPage: WorkMessageToUserPage{
			Page:      2,
			PerPage:   20,
			Total:     1,
			TotalPage: 1,
			Items: []WorkMessageToUser{{
				ID:             17,
				TableIndex:     1,
				MsgID:          "archive-31",
				WorkEmployeeID: 9,
				ToUserType:     1,
				ToUserID:       31,
				Name:           "星河科技",
				Content:        "请确认报价",
				MsgDataTime:    "2026-07-05 11:00:00",
			}},
		},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/toUsers?view=global&corpId=7&keyword=报价&employeeId=9&customerId=31&from=2026-07-01&to=2026-07-31&page=2&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageToUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data struct {
			List     []map[string]any `json:"list"`
			Total    int              `json:"total"`
			Page     int              `json:"page"`
			PageSize int              `json:"pageSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Total != 1 || envelope.Data.Page != 2 || envelope.Data.PageSize != 20 {
		t.Fatalf("page = %#v", envelope.Data)
	}
	if len(envelope.Data.List) != 1 || envelope.Data.List[0]["id"] != "msg:archive-31" {
		t.Fatalf("list = %#v", envelope.Data.List)
	}
	assertStructField(t, store.lastToUserFilter, "Keyword", "报价")
	assertStructField(t, store.lastToUserFilter, "ToUserID", 31)
	assertStructField(t, store.lastToUserFilter, "DateTimeStart", "2026-07-01 00:00:00")
	assertStructField(t, store.lastToUserFilter, "DateTimeEnd", "2026-08-01 00:00:00")
}

func TestWorkMessageToUsersAllowsGuardedUnboundSuperadmin(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10, IsSuperAdmin: 1}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, &recordingAuthorizer{accessSet: true, access: AccessContext{CorpID: 7, DataPermission: DataPermissionAll}})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/toUsers?page=1&perPage=15", nil)
	ctx := dashboardprincipal.WithPrincipal(req.Context(), dashboardprincipal.DashboardPrincipal{UserID: 1, TenantID: 10, CorpID: 7, IsSuperAdmin: true, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 1})
	req = req.WithContext(WithDashboardAccessContext(ctx, DashboardAccessContext{UserID: 1, TenantID: 10, CorpID: 7, IsSuperAdmin: true, ScopeRequired: true, Scope: DataScopeTenant}))
	rec := httptest.NewRecorder()
	handler.WorkMessageToUsers(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.employeeLookupCalls != 0 {
		t.Fatalf("legacy employee lookups=%d want 0", store.employeeLookupCalls)
	}
}

func TestWorkMessageGlobalSearchIgnoresClientCorpScope(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10, IsSuperAdmin: 1}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/toUsers?view=global&corpId=99&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageToUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.toUserCalls != 1 || store.lastToUserFilter.CorpID != 7 {
		t.Fatalf("storage calls = %d, filter=%#v", store.toUserCalls, store.lastToUserFilter)
	}
}

func TestWorkMessageGlobalSearchSupportsRoomFilterAndEmptyResults(t *testing.T) {
	store := &fakeAutoTagStore{
		user:       User{ID: 1, TenantID: 10, IsSuperAdmin: 1},
		toUserPage: WorkMessageToUserPage{Page: 1, PerPage: 20, Items: []WorkMessageToUser{}},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/toUsers?view=global&corpId=7&roomId=44&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageToUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastToUserFilter.ToUserType != 2 {
		t.Fatalf("target type = %d", store.lastToUserFilter.ToUserType)
	}
	assertStructField(t, store.lastToUserFilter, "ToUserID", 44)
	var envelope struct {
		Data struct {
			List  []any `json:"list"`
			Total int   `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.List == nil || len(envelope.Data.List) != 0 || envelope.Data.Total != 0 {
		t.Fatalf("data = %#v", envelope.Data)
	}
}

func TestWorkMessageGlobalSearchCapsPageSize(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10, IsSuperAdmin: 1}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/toUsers?view=global&corpId=7&page=1&pageSize=1000000", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageToUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastToUserFilter.PerPage != 100 {
		t.Fatalf("per page = %d", store.lastToUserFilter.PerPage)
	}
}

func TestWorkMessageGlobalSearchRejectsMalformedNumericFilters(t *testing.T) {
	for _, query := range []string{
		"employeeId=abc",
		"customerId=-1",
		"roomId=0",
		"page=abc",
		"pageSize=-20",
	} {
		t.Run(query, func(t *testing.T) {
			store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10, IsSuperAdmin: 1}}
			handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
			req := httptest.NewRequest(http.MethodGet,
				"/dashboard/workMessage/toUsers?view=global&corpId=7&"+query, nil)
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			req = withAutoTagTestPrincipal(req, 1, 10, 7)
			rec := httptest.NewRecorder()

			handler.WorkMessageToUsers(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			if store.toUserCalls != 0 {
				t.Fatalf("storage calls = %d", store.toUserCalls)
			}
		})
	}
}

func TestWorkMessageGlobalSearchAppliesEmployeeDataPermission(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10}}
	authorizer := &recordingAuthorizer{
		accessSet: true,
		access: AccessContext{
			CorpID:          7,
			WorkEmployeeID:  9,
			DataPermission:  DataPermissionDepartment,
			DeptEmployeeIDs: []int{9, 10},
		},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/toUsers?view=global&corpId=7&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageToUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	assertStructField(t, store.lastToUserFilter, "RestrictEmployeeIDs", true)
	assertStructField(t, store.lastToUserFilter, "EmployeeIDs", []int{9, 10})
}

func TestWorkMessageGlobalSearchIntersectsRequestedEmployeesAndUsesExclusiveEndBoundary(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10}}
	authorizer := &recordingAuthorizer{
		accessSet: true,
		access: AccessContext{
			CorpID:          7,
			WorkEmployeeID:  9,
			DataPermission:  DataPermissionDepartment,
			DeptEmployeeIDs: []int{9, 10},
		},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/toUsers?view=global&keyword=报价&conversationType=customer&employeeIds=9&employeeIds=999&startAt=2026-07-01&endAt=2026-07-31&page=2&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageToUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	assertStructField(t, store.lastToUserFilter, "ToUserType", 1)
	assertStructField(t, store.lastToUserFilter, "EmployeeIDs", []int{9})
	assertStructField(t, store.lastToUserFilter, "RestrictEmployeeIDs", true)
	assertStructField(t, store.lastToUserFilter, "DateTimeStart", "2026-07-01 00:00:00")
	assertStructField(t, store.lastToUserFilter, "DateTimeEnd", "2026-08-01 00:00:00")
	if store.archiveTenantID != 10 || store.archiveCorpID != 7 {
		t.Fatalf("archive scope tenant=%d corp=%d", store.archiveTenantID, store.archiveCorpID)
	}
}

func TestWorkMessageGlobalSearchReturnsZeroForEmptyEmployeeIntersection(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10}}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
		CorpID: 7, WorkEmployeeID: 9, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9},
	}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/toUsers?view=global&employeeIds=999&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageToUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	assertStructField(t, store.lastToUserFilter, "RestrictEmployeeIDs", true)
	assertStructField(t, store.lastToUserFilter, "EmployeeIDs", []int{})
}

func TestWorkMessageGlobalSearchRejectsInvalidConversationType(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10, IsSuperAdmin: 1}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/toUsers?view=global&conversationType=channel&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageToUsers(rec, req)

	if rec.Code != http.StatusBadRequest || store.toUserCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.toUserCalls, rec.Body.String())
	}
}

func TestWorkMessageGlobalSearchReturnsArchiveUnauthorized(t *testing.T) {
	store := &fakeAutoTagStore{
		user:                    User{ID: 1, TenantID: 10, IsSuperAdmin: 1},
		archiveAuthorizationSet: true,
		archiveAuthorized:       false,
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/toUsers?view=global&page=1&pageSize=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageToUsers(rec, req)

	var envelope struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusForbidden || envelope.Code != 40301 || store.toUserCalls != 0 {
		t.Fatalf("status=%d code=%d calls=%d body=%s", rec.Code, envelope.Code, store.toUserCalls, rec.Body.String())
	}
}

func TestWorkMessageGlobalListAndDetailUseSameRBACPermissionKey(t *testing.T) {
	const expectedPermissionKey = "/dashboard/workMessage/toUsers#get"
	for _, tc := range []struct {
		name string
		url  string
		call func(*AutoTagHandler, http.ResponseWriter, *http.Request)
	}{
		{
			name: "list",
			url:  "/dashboard/workMessage/toUsers?view=global&page=1&pageSize=20",
			call: (*AutoTagHandler).WorkMessageToUsers,
		},
		{
			name: "detail",
			url:  "/dashboard/workMessage/detail?id=msg%3Aarchive-31",
			call: (*AutoTagHandler).WorkMessageIndex,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeAutoTagStore{
				user: User{ID: 1, TenantID: 10},
				archiveMessage: WorkMessageItem{
					ID: 17, TableIndex: 1, MsgID: "archive-31", WorkEmployeeID: 9, ToUserType: 1, ToUserID: 31,
				},
				archiveMessageFound: true,
				messagePage: WorkMessagePage{Page: 1, PerPage: 200, Total: 1, Items: []WorkMessageItem{{
					ID: 17, TableIndex: 1, MsgID: "archive-31", WorkEmployeeID: 9, ToUserType: 1, ToUserID: 31,
				}}},
			}
			authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
				CorpID: 7, WorkEmployeeID: 9, DataPermission: DataPermissionAll,
			}}
			handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			req = withAutoTagTestPrincipal(req, 1, 10, 7)
			rec := httptest.NewRecorder()

			tc.call(handler, rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if authorizer.permissionKey != expectedPermissionKey {
				t.Fatalf("permission key = %q, want %q", authorizer.permissionKey, expectedPermissionKey)
			}
		})
	}
}

func TestWorkMessageGlobalDetailReturnsConversationMessages(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 1, TenantID: 10, IsSuperAdmin: 1},
		archiveMessage: WorkMessageItem{
			ID: 17, TableIndex: 1, MsgID: "archive-31", WorkEmployeeID: 9, ToUserType: 1, ToUserID: 31,
		},
		archiveMessageFound: true,
		messagePage: WorkMessagePage{
			Page: 1, PerPage: 200, Total: 1,
			Items: []WorkMessageItem{{
				ID: 17, TableIndex: 1, Name: "张三", ContentRaw: `{"content":"你好"}`,
				MsgDataTime: "2026-07-05 11:00:00",
			}},
		},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/detail?id=msg%3Aarchive-31", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data struct {
			ID           string           `json:"id"`
			MessageTotal int              `json:"messageTotal"`
			Truncated    bool             `json:"truncated"`
			Window       string           `json:"window"`
			Messages     []map[string]any `json:"messages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ID != "msg:archive-31" || len(envelope.Data.Messages) != 1 {
		t.Fatalf("detail = %#v", envelope.Data)
	}
	if envelope.Data.MessageTotal != 1 || envelope.Data.Truncated || envelope.Data.Window != "latest" {
		t.Fatalf("window = %#v", envelope.Data)
	}
	if envelope.Data.Messages[0]["id"] != "table:1:17" {
		t.Fatalf("message = %#v", envelope.Data.Messages[0])
	}
	if store.lastMessageFilter.WorkEmployeeID != 9 || store.lastMessageFilter.ToUserType != 1 || store.lastMessageFilter.ToUserID != 31 {
		t.Fatalf("filter = %#v", store.lastMessageFilter)
	}
}

func TestWorkMessageGlobalDetailReturnsNotFoundWithoutCrossCorpData(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10, IsSuperAdmin: 1}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet,
		"/dashboard/workMessage/detail?id=msg%3Amissing", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageIndex(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastArchiveFilter.CorpID != 7 {
		t.Fatalf("filter = %#v", store.lastArchiveFilter)
	}
}

func TestWorkMessageGlobalDetailUsesSameEmployeeScopeAndReturnsNotFound(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10}}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
		CorpID: 7, WorkEmployeeID: 9, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10},
	}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/detail?id=msg%3Arestricted", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageIndex(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !store.lastArchiveFilter.RestrictEmployeeIDs || !reflect.DeepEqual(store.lastArchiveFilter.EmployeeIDs, []int{9, 10}) {
		t.Fatalf("archive filter = %#v", store.lastArchiveFilter)
	}
}

func TestWorkMessageGlobalDetailReturnsArchiveUnauthorized(t *testing.T) {
	store := &fakeAutoTagStore{
		user:                    User{ID: 1, TenantID: 10, IsSuperAdmin: 1},
		archiveAuthorizationSet: true, archiveAuthorized: false,
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/detail?id=msg%3Aarchive-31", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageIndex(rec, req)

	var envelope struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusForbidden || envelope.Code != 40301 || store.archiveLookupCalls != 0 {
		t.Fatalf("status=%d code=%d lookupCalls=%d body=%s", rec.Code, envelope.Code, store.archiveLookupCalls, rec.Body.String())
	}
}

func TestWorkMessageGlobalDetailReturnsRBACForbiddenBeforeArchiveAccess(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, TenantID: 10}}
	authorizer := &recordingAuthorizer{err: ErrPermissionDenied}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/detail?id=msg%3Aarchive-31", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageIndex(rec, req)

	if rec.Code != http.StatusForbidden || store.archiveAuthorizationCalls != 0 || store.archiveLookupCalls != 0 {
		t.Fatalf("status=%d authorizationCalls=%d lookupCalls=%d body=%s", rec.Code, store.archiveAuthorizationCalls, store.archiveLookupCalls, rec.Body.String())
	}
}

func assertStructField(t *testing.T, value any, name string, want any) {
	t.Helper()
	field := reflect.ValueOf(value).FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("%T is missing field %s", value, name)
	}
	got := field.Interface()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}

func TestWorkMessageConfigStepCreateReturnsCorpConfig(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		config: WorkMessageConfigItem{
			ID:                 7,
			CorpID:             7,
			CorpName:           "测试企业",
			WXCorpID:           "wx123",
			ChatApplyStatus:    3,
			ChatWhitelistIPRaw: `["127.0.0.1"]`,
			ChatRSAKeyRaw:      `{"publicKey":"pub","privateKey":"pri"}`,
		},
		configFound: true,
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessageConfig/stepCreate", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req = withAutoTagTestPrincipal(req, 1, 10, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageConfigStepCreate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data["corpName"] != "测试企业" || envelope.Data["rsaPublicKey"] != "pub" {
		t.Fatalf("data = %#v", envelope.Data)
	}
}

type fakeAutoTagStore struct {
	user                      User
	page                      AutoTagPage
	item                      AutoTagItem
	stats                     AutoTagStatistics
	recordPage                AutoTagRecordPage
	created                   AutoTagWrite
	createID                  int
	lastFilter                AutoTagFilter
	messagePage               WorkMessagePage
	lastMessageFilter         WorkMessageFilter
	toUserPage                WorkMessageToUserPage
	lastToUserFilter          WorkMessageUserFilter
	toUserCalls               int
	config                    WorkMessageConfigItem
	configFound               bool
	archiveAuthorizationSet   bool
	archiveAuthorized         bool
	archiveTenantID           int
	archiveCorpID             int
	archiveMessage            WorkMessageItem
	archiveMessageFound       bool
	lastArchiveFilter         WorkMessageArchiveFilter
	archiveAuthorizationCalls int
	archiveLookupCalls        int
	employeeLookupCalls       int
}

func (s *fakeAutoTagStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	user := s.user
	if user.TenantID == 0 {
		user.TenantID = 10
	}
	return user, true, nil
}

func (s *fakeAutoTagStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	s.employeeLookupCalls++
	return 99, nil
}

func (s *fakeAutoTagStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeAutoTagStore) AutoTagPage(_ context.Context, filter AutoTagFilter) (AutoTagPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeAutoTagStore) AutoTagByID(context.Context, int, int) (AutoTagItem, bool, error) {
	if s.item.ID <= 0 {
		return AutoTagItem{}, false, nil
	}
	return s.item, true, nil
}

func (s *fakeAutoTagStore) CreateAutoTag(_ context.Context, values AutoTagWrite) (int, error) {
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeAutoTagStore) UpdateAutoTagOnOff(context.Context, int, int, int) (bool, error) {
	return true, nil
}

func (s *fakeAutoTagStore) DeleteAutoTag(context.Context, int, int) (bool, error) {
	return true, nil
}

func (s *fakeAutoTagStore) AutoTagStatistics(context.Context, int, int) (AutoTagStatistics, error) {
	return s.stats, nil
}

func (s *fakeAutoTagStore) AutoTagRecordPage(context.Context, AutoTagRecordFilter) (AutoTagRecordPage, error) {
	return s.recordPage, nil
}

func (s *fakeAutoTagStore) AutoTagKeywordTask(context.Context, int) (AutoTagKeywordTaskResult, error) {
	return AutoTagKeywordTaskResult{RuleCount: 1}, nil
}

func (s *fakeAutoTagStore) WorkMessageFromUsers(context.Context, WorkMessageFromUserFilter) ([]WorkMessageFromUser, error) {
	return []WorkMessageFromUser{{ID: 99, Name: "张三"}}, nil
}

func (s *fakeAutoTagStore) WorkMessageToUsers(_ context.Context, filter WorkMessageUserFilter) (WorkMessageToUserPage, error) {
	s.lastToUserFilter = filter
	s.toUserCalls++
	return s.toUserPage, nil
}

func (s *fakeAutoTagStore) WorkMessagePage(_ context.Context, filter WorkMessageFilter) (WorkMessagePage, error) {
	s.lastMessageFilter = filter
	return s.messagePage, nil
}

func (s *fakeAutoTagStore) WorkMessageArchiveAuthorized(_ context.Context, tenantID int, corpID int) (bool, error) {
	s.archiveAuthorizationCalls++
	s.archiveTenantID = tenantID
	s.archiveCorpID = corpID
	if !s.archiveAuthorizationSet {
		return true, nil
	}
	return s.archiveAuthorized, nil
}

func (s *fakeAutoTagStore) WorkMessageByArchiveID(_ context.Context, filter WorkMessageArchiveFilter) (WorkMessageItem, bool, error) {
	s.archiveLookupCalls++
	s.lastArchiveFilter = filter
	return s.archiveMessage, s.archiveMessageFound, nil
}

func (s *fakeAutoTagStore) WorkMessageConfigByCorp(context.Context, int) (WorkMessageConfigItem, bool, error) {
	return s.config, s.configFound, nil
}

func (s *fakeAutoTagStore) WorkMessageConfigPage(context.Context, int, string, int, int) (WorkMessageConfigPage, error) {
	return WorkMessageConfigPage{Items: []WorkMessageConfigItem{s.config}, Total: 1, Page: 1, PerPage: 10}, nil
}

func (s *fakeAutoTagStore) UpsertWorkMessageCorpConfig(context.Context, int, WorkMessageConfigItem) (int, error) {
	return 7, nil
}

func (s *fakeAutoTagStore) UpdateWorkMessageStepConfig(context.Context, int, WorkMessageConfigItem) (bool, error) {
	return true, nil
}
