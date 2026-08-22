package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkEmployeeSearchConditionReturnsEnumsAndSyncTime(t *testing.T) {
	store := &fakeWorkReadStore{
		users:    map[int]User{1: {ID: 1}},
		syncTime: "2026-07-02 10:30:00",
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workEmployee/searchCondition", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.SearchCondition(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.lastSyncCorpIDs) != 1 || store.lastSyncCorpIDs[0] != 7 {
		t.Fatalf("corpIDs = %#v", store.lastSyncCorpIDs)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	status := data["status"].([]any)
	if first := status[0].(map[string]any); int(first["id"].(float64)) != 1 || first["name"] != "已激活" {
		t.Fatalf("status = %#v", status)
	}
	contactAuth := data["contactAuth"].([]any)
	if second := contactAuth[1].(map[string]any); int(second["id"].(float64)) != 2 || second["name"] != "否" {
		t.Fatalf("contactAuth = %#v", contactAuth)
	}
	if data["syncTime"] != "2026-07-02 10:30:00" {
		t.Fatalf("syncTime = %#v", data["syncTime"])
	}
}

func TestWorkEmployeeSearchConditionIgnoresCrossTenantCachedCorp(t *testing.T) {
	store := &fakeWorkReadStore{
		users:           map[int]User{1: {ID: 1}},
		corpIDsByUser:   []int{7},
		firstCorpID:     7,
		firstEmployeeID: 99,
		syncTime:        "2026-07-02 10:30:00",
	}
	handler := NewWorkReadHandler(store, staticAdminCache("8-0"), HeaderUserIDResolver{}, "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workEmployee/searchCondition", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.SearchCondition(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.lastSyncCorpIDs) != 1 || store.lastSyncCorpIDs[0] != 7 {
		t.Fatalf("corpIDs = %#v", store.lastSyncCorpIDs)
	}
}

func TestWorkDepartmentIndexReturnsTreeAndEmployees(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		departments: []WorkDepartment{
			{ID: 10, CorpID: 7, Name: "总部", ParentID: 0, WXDepartmentID: 1, WXParentID: 0, Order: 10, Path: "#10#", Level: 1},
			{ID: 11, CorpID: 7, Name: "销售部", ParentID: 10, WXDepartmentID: 2, WXParentID: 1, Order: 9, Path: "#10#-#11#", Level: 2},
		},
		employees: []WorkDepartmentEmployee{
			{ID: 21, Name: "张三", WXUserID: "zhangsan", Avatar: "avatar/a.png"},
		},
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com/")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workDepartment/index?searchKeyWords=销售", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.DepartmentIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastDepartmentCorpID != 7 || store.lastEmployeeCorpID != 7 || store.lastDepartmentSearch != "销售" || store.lastEmployeeSearch != "销售" {
		t.Fatalf("store calls = %#v", store)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	departments := data["department"].([]any)
	root := departments[0].(map[string]any)
	if int(root["id"].(float64)) != 10 || root["name"] != "总部" {
		t.Fatalf("root = %#v", root)
	}
	child := root["son"].([]any)[0].(map[string]any)
	if int(child["id"].(float64)) != 11 || child["name"] != "销售部" || int(child["parentId"].(float64)) != 10 {
		t.Fatalf("child = %#v", child)
	}

	employees := data["employee"].([]any)
	employee := employees[0].(map[string]any)
	if int(employee["id"].(float64)) != 21 || int(employee["employeeId"].(float64)) != 21 || employee["avatar"] != "http://api.example.com/static/avatar/a.png" {
		t.Fatalf("employee = %#v", employee)
	}
}

func TestWorkDepartmentIndexRequiresDashboardPrincipal(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandler(store, staticAdminCache(""), HeaderUserIDResolver{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/workDepartment/index", nil)
	rec := httptest.NewRecorder()
	handler.DepartmentIndex(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkEmployeeDepartmentMemberIndexReturnsMembers(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		members: []WorkDepartmentMember{
			{EmployeeID: 21, DepartmentID: 10, DepartmentName: "总部", EmployeeName: "张三"},
			{EmployeeID: 22, DepartmentID: 11, DepartmentName: "销售部", EmployeeName: "李四"},
		},
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workEmployeeDepartment/memberIndex?departmentIds=10,11,10,abc", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.MemberIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastMemberCorpID != 7 || len(store.lastMemberDepartmentIDs) != 2 || store.lastMemberDepartmentIDs[0] != 10 || store.lastMemberDepartmentIDs[1] != 11 {
		t.Fatalf("member call = corp %d departments %#v", store.lastMemberCorpID, store.lastMemberDepartmentIDs)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	member := data[0].(map[string]any)
	if int(member["employeeId"].(float64)) != 21 || int(member["departmentId"].(float64)) != 10 || member["departmentName"] != "总部" || member["employeeName"] != "张三" {
		t.Fatalf("member = %#v", member)
	}
}

func TestWorkEmployeeDepartmentMemberIndexRequiresDepartmentIDs(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workEmployeeDepartment/memberIndex", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.MemberIndex(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "部门id必传" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestWorkDepartmentSelectByPhoneReturnsDepartments(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		phoneDepartments: []WorkDepartmentPhoneOption{
			{CorpID: 7, WorkDepartmentID: 10, WorkDepartmentName: "总部"},
		},
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workDepartment/selectByPhone?phone=13800000000&type=2", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.SelectByPhone(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastPhoneCorpID != 7 || store.lastPhone != "13800000000" {
		t.Fatalf("phone call = corp %d phone %q", store.lastPhoneCorpID, store.lastPhone)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	department := data[0].(map[string]any)
	if int(department["corpId"].(float64)) != 7 || int(department["workDepartmentId"].(float64)) != 10 || department["workDepartmentName"] != "总部" {
		t.Fatalf("department = %#v", department)
	}
}

func TestWorkDepartmentSelectByPhoneValidatesPhone(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workDepartment/selectByPhone?phone=138&type=1", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.SelectByPhone(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "手机号码 字符串长度为固定值：11" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestWorkDepartmentPageIndexReturnsTreeWithAuthorization(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		departments: []WorkDepartment{
			{ID: 10, CorpID: 7, Name: "总部", ParentID: 0, Level: 1, Path: "#10#"},
			{ID: 11, CorpID: 7, Name: "销售部", ParentID: 10, Level: 2, Path: "#10#-#11#"},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workDepartment/pageIndex?page=1&perPage=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.DepartmentPageIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workDepartment/pageIndex#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if store.lastDepartmentCorpID != 7 || store.lastDepartmentSearch != "" {
		t.Fatalf("department call = corp %d search %q", store.lastDepartmentCorpID, store.lastDepartmentSearch)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if int(page["total"].(float64)) != 1 || int(page["totalPage"].(float64)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	root := data["list"].([]any)[0].(map[string]any)
	if int(root["departmentId"].(float64)) != 10 || root["level"] != "一级部门" || root["departmentPath"] != "1" {
		t.Fatalf("root = %#v", root)
	}
	child := root["children"].([]any)[0].(map[string]any)
	if int(child["departmentId"].(float64)) != 11 || child["level"] != "二级部门" || child["departmentPath"] != "1-1" {
		t.Fatalf("child = %#v", child)
	}
}

func TestWorkDepartmentPagePayloadIncludesEnterpriseRoot(t *testing.T) {
	payload := workDepartmentPagePayload([]WorkDepartment{
		{ID: 2, CorpID: 7, Name: "测试公司", ParentID: 0, Level: 0, Path: "#2#"},
	}, 1, 10)

	page := payload["page"].(map[string]any)
	if page["total"] != 1 || page["totalPage"] != 1 {
		t.Fatalf("page = %#v", page)
	}
	list := payload["list"].([]map[string]any)
	if len(list) != 1 {
		t.Fatalf("list = %#v", list)
	}
	root := list[0]
	if root["departmentId"] != 2 || root["name"] != "测试公司" || root["level"] != "企业根部门" || root["departmentPath"] != "1" {
		t.Fatalf("root = %#v", root)
	}
}

func TestWorkDepartmentShowEmployeeReturnsPagedEmployees(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		employeePage: WorkDepartmentEmployeePage{
			Items: []WorkDepartmentEmployeeListItem{
				{EmployeeID: 21, EmployeeName: "张三", Phone: "13800000000", RoleName: "管理员"},
			},
			Total:     1,
			TotalPage: 1,
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workDepartment/showEmployee?departmentId=10&page=1&perPage=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.DepartmentShowEmployee(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workDepartment/showEmployee#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if store.lastEmployeePageFilter.CorpID != 7 || store.lastEmployeePageFilter.DepartmentID != 10 || store.lastEmployeePageFilter.PerPage != 20 {
		t.Fatalf("filter = %#v", store.lastEmployeePageFilter)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if int(page["total"].(float64)) != 1 || int(page["perPage"].(float64)) != 20 {
		t.Fatalf("page = %#v", page)
	}
	employee := data["list"].([]any)[0].(map[string]any)
	if int(employee["employeeId"].(float64)) != 21 || employee["employeeName"] != "张三" || employee["phone"] != "13800000000" || employee["roleName"] != "管理员" {
		t.Fatalf("employee = %#v", employee)
	}
}

func TestWorkEmployeeIndexReturnsPagedEmployees(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		workEmployeeIndexPage: WorkEmployeeIndexPage{
			Items: []WorkEmployeeIndexItem{
				{
					ID:                21,
					Name:              "张三",
					ThumbAvatar:       "avatar/a.png",
					Status:            1,
					ContactAuth:       2,
					WXUserID:          "zhangsan",
					CorpID:            7,
					Gender:            1,
					MessageNums:       8,
					SendMessageNums:   5,
					ReplyMessageRatio: "0.75",
					AddNums:           3,
					ApplyNums:         2,
					InvalidContact:    1,
					AverageReply:      10,
				},
			},
			Total:     1,
			TotalPage: 1,
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workEmployee/index?status=1&contactAuth=2&page=1&perPage=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkEmployeeIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workEmployee/index#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if len(store.lastWorkEmployeeIndexFilter.CorpIDs) != 1 || store.lastWorkEmployeeIndexFilter.CorpIDs[0] != 7 || store.lastWorkEmployeeIndexFilter.Status != 1 || store.lastWorkEmployeeIndexFilter.ContactAuth != "2" {
		t.Fatalf("filter = %#v", store.lastWorkEmployeeIndexFilter)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	employee := data["list"].([]any)[0].(map[string]any)
	if int(employee["id"].(float64)) != 21 || employee["statusName"] != "已激活" || employee["gender"] != "男" || employee["contactAuthName"] != "否" || employee["thumbAvatar"] != "http://api.example.com/static/avatar/a.png" {
		t.Fatalf("employee = %#v", employee)
	}
	if employee["replyMessageRatio"] != "0.75" || int(employee["addNums"].(float64)) != 3 {
		t.Fatalf("employee stats = %#v", employee)
	}
}

func TestWorkEmployeeIndexAppliesDataPermission(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                 map[int]User{1: {ID: 1}},
		workEmployeeIndexPage: WorkEmployeeIndexPage{Items: []WorkEmployeeIndexItem{}, Total: 0, TotalPage: 0},
	}
	authorizer := &recordingAuthorizer{
		accessSet: true,
		access: AccessContext{
			DataPermission:  DataPermissionSelf,
			DeptEmployeeIDs: []int{22},
		},
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workEmployee/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkEmployeeIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	filter := store.lastWorkEmployeeIndexFilter
	if !filter.RestrictEmployeeIDs || len(filter.EmployeeIDs) != 1 || filter.EmployeeIDs[0] != 22 {
		t.Fatalf("filter = %#v", filter)
	}
}

func TestWorkEmployeeIndexRejectsInvalidQuery(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", &recordingAuthorizer{})

	tests := []struct {
		name string
		path string
		msg  string
	}{
		{name: "invalid status", path: "/dashboard/workEmployee/index?status=abc", msg: "成员状态必须为整数"},
		{name: "invalid corp id", path: "/dashboard/workEmployee/index?corpId=abc", msg: "企业微信不能为空"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := authenticatedDashboardRequestForTest(http.MethodGet, tt.path, nil)
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()
			handler.WorkEmployeeIndex(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			body := decodeBody(t, rec.Body.Bytes())
			if body["msg"] != tt.msg {
				t.Fatalf("msg = %#v, want %q", body["msg"], tt.msg)
			}
		})
	}
}

func TestContactTagGroupIndexReturnsGroups(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		contactTagGroups: []WorkContactTagGroup{
			{ID: 11, GroupName: "重点客户"},
		},
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContactTagGroup/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagGroupIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.lastContactTagGroupCorpIDs) != 1 || store.lastContactTagGroupCorpIDs[0] != 7 {
		t.Fatalf("corp ids = %#v", store.lastContactTagGroupCorpIDs)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	first := data[0].(map[string]any)
	last := data[1].(map[string]any)
	if int(first["groupId"].(float64)) != 11 || first["groupName"] != "重点客户" {
		t.Fatalf("first = %#v", first)
	}
	if int(last["groupId"].(float64)) != 0 || last["groupName"] != "未分组" {
		t.Fatalf("last = %#v", last)
	}
}

func TestSidebarContactTagGroupIndexUsesEmployeeCorp(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		contactTagGroups: []WorkContactTagGroup{
			{ID: 11, GroupName: "重点客户"},
		},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workContactTagGroup/index", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarContactTagGroupIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSidebarEmployeeID != 5 {
		t.Fatalf("lastSidebarEmployeeID = %d", store.lastSidebarEmployeeID)
	}
	if len(store.lastContactTagGroupCorpIDs) != 1 || store.lastContactTagGroupCorpIDs[0] != 7 {
		t.Fatalf("corp ids = %#v", store.lastContactTagGroupCorpIDs)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	first := data[0].(map[string]any)
	last := data[1].(map[string]any)
	if int(first["groupId"].(float64)) != 11 || first["groupName"] != "重点客户" {
		t.Fatalf("first = %#v", first)
	}
	if int(last["groupId"].(float64)) != 0 || last["groupName"] != "未分组" {
		t.Fatalf("last = %#v", last)
	}
}

func TestContactTagGroupDetailReturnsGroup(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                map[int]User{1: {ID: 1}},
		contactTagGroup:      WorkContactTagGroup{ID: 11, GroupName: "重点客户"},
		contactTagGroupFound: true,
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContactTagGroup/detail?groupId=11", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagGroupDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastContactTagGroupID != 11 {
		t.Fatalf("group id = %d", store.lastContactTagGroupID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["id"].(float64)) != 11 || data["groupName"] != "重点客户" {
		t.Fatalf("data = %#v", data)
	}
}

func TestContactTagGroupDetailRequiresGroupID(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContactTagGroup/detail", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagGroupDetail(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "分组id必传" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestContactTagGroupStoreCreatesGroup(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workContactTagGroup/store", strings.NewReader(`{"groupName":"重点客户"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagGroupStore(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastContactTagGroupNameCorpID != 7 || store.lastContactTagGroupName != "重点客户" || store.lastContactTagGroupNameExcludeID != 0 {
		t.Fatalf("name check = corp %d name %q exclude %d", store.lastContactTagGroupNameCorpID, store.lastContactTagGroupName, store.lastContactTagGroupNameExcludeID)
	}
	if store.createdContactTagGroup.CorpID != 7 || store.createdContactTagGroup.GroupName != "重点客户" {
		t.Fatalf("created group = %#v", store.createdContactTagGroup)
	}
}

func TestContactTagGroupDestroyCascades(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                map[int]User{1: {ID: 1}},
		contactTagGroup:      WorkContactTagGroup{ID: 11, GroupName: "重点客户"},
		contactTagGroupFound: true,
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/workContactTagGroup/destroy", strings.NewReader(`{"groupId":11}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagGroupDestroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedContactTagGroupCorpID != 7 || store.deletedContactTagGroupID != 11 {
		t.Fatalf("deleted group = corp %d id %d", store.deletedContactTagGroupCorpID, store.deletedContactTagGroupID)
	}
}

func TestContactTagIndexReturnsPagedTags(t *testing.T) {
	store := &fakeWorkReadStore{
		users:              map[int]User{1: {ID: 1}},
		contactTagSyncTime: "2026-07-03 10:00:00",
		contactTagPage: WorkContactTagPage{
			Items:     []WorkContactTagItem{{ID: 21, Name: "高意向", ContactNum: 3}},
			Total:     1,
			TotalPage: 1,
			PerPage:   5,
		},
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContactTag/index?groupId=11&page=1&perPage=5", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.lastContactTagFilter.CorpIDs) != 1 || store.lastContactTagFilter.CorpIDs[0] != 7 || store.lastContactTagFilter.GroupID == nil || *store.lastContactTagFilter.GroupID != 11 {
		t.Fatalf("filter = %#v", store.lastContactTagFilter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if data["syncTagTime"] != "2026-07-03 10:00:00" {
		t.Fatalf("syncTagTime = %#v", data["syncTagTime"])
	}
	page := data["page"].(map[string]any)
	if int(page["perPage"].(float64)) != 5 || int(page["total"].(float64)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	tag := data["list"].([]any)[0].(map[string]any)
	if int(tag["id"].(float64)) != 21 || tag["name"] != "高意向" || int(tag["contactNum"].(float64)) != 3 {
		t.Fatalf("tag = %#v", tag)
	}
}

func TestContactTagDetailReturnsTag(t *testing.T) {
	store := &fakeWorkReadStore{
		users:           map[int]User{1: {ID: 1}},
		contactTag:      WorkContactTagDetail{TagID: 21, TagName: "高意向", GroupID: 11},
		contactTagFound: true,
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContactTag/detail?tagId=21", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastContactTagID != 21 {
		t.Fatalf("tag id = %d", store.lastContactTagID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["tagId"].(float64)) != 21 || data["tagName"] != "高意向" || int(data["groupId"].(float64)) != 11 {
		t.Fatalf("data = %#v", data)
	}
}

func TestContactTagDetailRequiresTagID(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContactTag/detail", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagDetail(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "标签id必传" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestContactTagListReturnsGroupsWithTags(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		contactTagList: []WorkContactTagListGroup{
			{
				ID:        11,
				WXGroupID: "wx-group-11",
				GroupName: "重点客户",
				Tags: []WorkContactTagListTag{
					{ID: 21, WXContactTagID: "wx-tag-21", Name: "高意向", ContactTagGroupID: 11},
				},
			},
		},
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContactTag/contactTagList?name=高", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.lastContactTagListCorpIDs) != 1 || store.lastContactTagListCorpIDs[0] != 7 || store.lastContactTagListName != "高" {
		t.Fatalf("list call = corpIDs %#v name %q", store.lastContactTagListCorpIDs, store.lastContactTagListName)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	group := data[0].(map[string]any)
	if int(group["id"].(float64)) != 11 || group["wxGroupId"] != "wx-group-11" || group["groupName"] != "重点客户" {
		t.Fatalf("group = %#v", group)
	}
	tags := group["tags"].([]any)
	tag := tags[0].(map[string]any)
	if int(tag["id"].(float64)) != 21 || tag["wxContactTagId"] != "wx-tag-21" || tag["name"] != "高意向" || int(tag["contactTagGroupId"].(float64)) != 11 {
		t.Fatalf("tag = %#v", tag)
	}
}

func TestContactTagAllReturnsTags(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		contactTagOptions: []WorkContactTagOption{
			{ID: 21, Name: "高意向"},
		},
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContactTag/allTag?groupId=11", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagAll(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.lastContactTagsCorpIDs) != 1 || store.lastContactTagsCorpIDs[0] != 7 || store.lastContactTagsGroupID == nil || *store.lastContactTagsGroupID != 11 {
		t.Fatalf("tag call = corpIDs %#v groupID %#v", store.lastContactTagsCorpIDs, store.lastContactTagsGroupID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	tag := data[0].(map[string]any)
	if int(tag["id"].(float64)) != 21 || tag["name"] != "高意向" {
		t.Fatalf("tag = %#v", tag)
	}
}

func TestContactTagStoreCreatesMultipleTags(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workContactTag/store", strings.NewReader(`{"groupId":11,"tagName":["高意向","复购"]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagStore(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastContactTagNameCheckCorpID != 7 || store.lastContactTagNameCheckGroupID != 11 || len(store.lastContactTagNameCheckNames) != 2 {
		t.Fatalf("name check = corp %d group %d names %#v", store.lastContactTagNameCheckCorpID, store.lastContactTagNameCheckGroupID, store.lastContactTagNameCheckNames)
	}
	if store.createdContactTags.CorpID != 7 || store.createdContactTags.GroupID != 11 || len(store.createdContactTags.TagNames) != 2 || store.createdContactTags.TagNames[0] != "高意向" || store.createdContactTags.TagNames[1] != "复购" {
		t.Fatalf("created tags = %#v", store.createdContactTags)
	}
}

func TestContactTagStoreSyncsRemoteCorpTags(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                      map[int]User{1: {ID: 1}},
		contactTagGroup:            WorkContactTagGroup{ID: 11, GroupName: "重点客户"},
		contactTagGroupFound:       true,
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "secret"},
		roomWelcomeCredentialFound: true,
	}
	client := &fakeContactTagWriteClient{addResult: WorkContactTagSyncGroup{
		WXGroupID: "wx-group-new",
		Tags: []WorkContactTagSyncTag{
			{WXContactTagID: "wx-tag-1", Name: "高意向"},
			{WXContactTagID: "wx-tag-2", Name: "复购"},
		},
	}}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "").
		WithWorkContactTagWriteClient(client)

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workContactTag/store", strings.NewReader(`{"groupId":11,"tagName":["高意向","复购"]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagStore(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(client.addRequests) != 1 || client.addRequests[0].GroupName != "重点客户" || len(client.addRequests[0].TagNames) != 2 {
		t.Fatalf("add requests = %#v", client.addRequests)
	}
	if store.updatedContactTagGroupWXIDCorpID != 7 || store.updatedContactTagGroupWXIDGroupID != 11 || store.updatedContactTagGroupWXID != "wx-group-new" {
		t.Fatalf("group wx update = corp %d group %d wx %q", store.updatedContactTagGroupWXIDCorpID, store.updatedContactTagGroupWXIDGroupID, store.updatedContactTagGroupWXID)
	}
	if store.updatedContactTagWXIDsCorpID != 7 || store.updatedContactTagWXIDsGroupID != 11 || store.updatedContactTagWXIDs["高意向"] != "wx-tag-1" || store.updatedContactTagWXIDs["复购"] != "wx-tag-2" {
		t.Fatalf("tag wx updates = corp %d group %d map %#v", store.updatedContactTagWXIDsCorpID, store.updatedContactTagWXIDsGroupID, store.updatedContactTagWXIDs)
	}
}

func TestContactTagUpdateSyncsRemoteRename(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                      map[int]User{1: {ID: 1}},
		contactTag:                 WorkContactTagDetail{TagID: 21, WXContactTagID: "wx-tag-21", TagName: "高意向", GroupID: 11},
		contactTagFound:            true,
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "secret"},
		roomWelcomeCredentialFound: true,
	}
	client := &fakeContactTagWriteClient{}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "").
		WithWorkContactTagWriteClient(client)

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workContactTag/update", strings.NewReader(`{"tagId":21,"groupId":11,"tagName":"高意向Plus","isUpdate":1}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if client.updatedTagID != "wx-tag-21" || client.updatedTagName != "高意向Plus" {
		t.Fatalf("remote update = id %q name %q", client.updatedTagID, client.updatedTagName)
	}
	if store.updatedContactTagTargetGroupID != 11 || store.updatedContactTagName != "高意向Plus" {
		t.Fatalf("local update = group %d name %q", store.updatedContactTagTargetGroupID, store.updatedContactTagName)
	}
}

func TestContactTagMoveSyncsRemoteDeleteAndAdd(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                      map[int]User{1: {ID: 1}},
		contactTag:                 WorkContactTagDetail{TagID: 21, WXContactTagID: "wx-old-tag", TagName: "高意向", GroupID: 10},
		contactTagFound:            true,
		contactTagGroup:            WorkContactTagGroup{ID: 11, WXGroupID: "wx-new-group", GroupName: "新分组"},
		contactTagGroupFound:       true,
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "secret"},
		roomWelcomeCredentialFound: true,
	}
	client := &fakeContactTagWriteClient{addResult: WorkContactTagSyncGroup{
		WXGroupID: "wx-new-group",
		Tags:      []WorkContactTagSyncTag{{WXContactTagID: "wx-new-tag", Name: "高意向"}},
	}}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "").
		WithWorkContactTagWriteClient(client)

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workContactTag/move", strings.NewReader(`{"tagId":"21","groupId":11}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagMove(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(client.deletedTagIDs) != 1 || client.deletedTagIDs[0] != "wx-old-tag" {
		t.Fatalf("deleted tag ids = %#v", client.deletedTagIDs)
	}
	if len(client.addRequests) != 1 || client.addRequests[0].WXGroupID != "wx-new-group" || client.addRequests[0].TagNames[0] != "高意向" {
		t.Fatalf("add requests = %#v", client.addRequests)
	}
	if store.updatedContactTagWXIDs["高意向"] != "wx-new-tag" {
		t.Fatalf("tag wx updates = %#v", store.updatedContactTagWXIDs)
	}
}

func TestContactTagGroupDestroySyncsRemoteGroupDelete(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                      map[int]User{1: {ID: 1}},
		contactTagGroup:            WorkContactTagGroup{ID: 11, WXGroupID: "wx-group-11", GroupName: "重点客户"},
		contactTagGroupFound:       true,
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "secret"},
		roomWelcomeCredentialFound: true,
	}
	client := &fakeContactTagWriteClient{}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "").
		WithWorkContactTagWriteClient(client)

	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/workContactTagGroup/destroy", strings.NewReader(`{"groupId":11}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagGroupDestroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(client.deletedGroupIDs) != 1 || client.deletedGroupIDs[0] != "wx-group-11" {
		t.Fatalf("deleted group ids = %#v", client.deletedGroupIDs)
	}
}

func TestContactTagMoveRejectsDuplicateName(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                map[int]User{1: {ID: 1}},
		contactTag:           WorkContactTagDetail{TagID: 21, TagName: "高意向", GroupID: 10},
		contactTagFound:      true,
		contactTagNamesExist: true,
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workContactTag/move", strings.NewReader(`{"tagId":"21","groupId":11}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagMove(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "该分组下已有相同标签" {
		t.Fatalf("msg = %#v", body["msg"])
	}
	if len(store.movedContactTagIDs) != 0 {
		t.Fatalf("moved ids = %#v", store.movedContactTagIDs)
	}
	if store.lastContactTagNameCheckCorpID != 7 || store.lastContactTagNameCheckGroupID != 11 || len(store.lastContactTagNameCheckExcludeIDs) != 1 || store.lastContactTagNameCheckExcludeIDs[0] != 21 {
		t.Fatalf("name check = corp %d group %d exclude %#v", store.lastContactTagNameCheckCorpID, store.lastContactTagNameCheckGroupID, store.lastContactTagNameCheckExcludeIDs)
	}
}

func TestSidebarContactTagAllUsesEmployeeCorp(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		contactTagOptions: []WorkContactTagOption{
			{ID: 21, Name: "高意向"},
		},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workContactTag/allTag?groupId=11", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarContactTagAll(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSidebarEmployeeID != 5 {
		t.Fatalf("lastSidebarEmployeeID = %d", store.lastSidebarEmployeeID)
	}
	if len(store.lastContactTagsCorpIDs) != 1 || store.lastContactTagsCorpIDs[0] != 7 || store.lastContactTagsGroupID == nil || *store.lastContactTagsGroupID != 11 {
		t.Fatalf("tag call = corpIDs %#v groupID %#v", store.lastContactTagsCorpIDs, store.lastContactTagsGroupID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	tag := data[0].(map[string]any)
	if int(tag["id"].(float64)) != 21 || tag["name"] != "高意向" {
		t.Fatalf("tag = %#v", tag)
	}
}

func TestSidebarWorkContactDetailReturnsContact(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		workContactDetail: WorkContactDetail{
			ID:     900001,
			Name:   "Go迁移客户",
			Avatar: "avatars/customer.png",
			CorpID: 7,
		},
		workContactFound: true,
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "http://api.example.com").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workContact/detail?wxExternalUserid=external-user-900001", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkContactDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSidebarEmployeeID != 5 {
		t.Fatalf("lastSidebarEmployeeID = %d", store.lastSidebarEmployeeID)
	}
	if store.lastWorkContactExternalUserID != "external-user-900001" {
		t.Fatalf("external user id = %q", store.lastWorkContactExternalUserID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["id"].(float64)) != 900001 || data["name"] != "Go迁移客户" || data["avatar"] != "http://api.example.com/static/avatars/customer.png" || int(data["corpId"].(float64)) != 7 {
		t.Fatalf("data = %#v", data)
	}
}

func TestSidebarWorkContactDetailRejectsAContactOutsideTheEmployeeScope(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees:  map[int]SidebarEmployee{5: {ID: 5, CorpID: 7}},
		workContactDetail: WorkContactDetail{ID: 31, CorpID: 8, Name: "其他企业客户"},
		workContactFound:  true,
		denyContactAccess: true,
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workContact/detail?wxExternalUserid=external-other", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()

	handler.SidebarWorkContactDetail(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSidebarWorkContactDetailRequiresExternalUserID(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workContact/detail", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkContactDetail(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "微信userId必须" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestSidebarWorkContactShowReturnsBasicInfo(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		workContactShow: WorkContactShow{
			Name:        "Go迁移客户",
			Avatar:      "avatar/contact.png",
			Gender:      1,
			BusinessNo:  "GO-900001",
			Remark:      "重点客户",
			Description: "需要继续跟进",
			Tags: []WorkContactShowTag{
				{TagID: 900001, TagName: "Go迁移标签"},
			},
			RoomNames: []string{
				"Go迁移客户群",
			},
			EmployeeName: []string{
				"Go迁移企业 Go迁移员工",
			},
		},
		workContactShowFound: true,
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "http://api.example.com").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workContact/show?contactId=900001", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkContactShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSidebarEmployeeID != 5 {
		t.Fatalf("lastSidebarEmployeeID = %d", store.lastSidebarEmployeeID)
	}
	if store.lastWorkContactShowContactID != 900001 || store.lastWorkContactShowEmployeeID != 5 {
		t.Fatalf("show lookup = contact %d employee %d", store.lastWorkContactShowContactID, store.lastWorkContactShowEmployeeID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if data["name"] != "Go迁移客户" ||
		data["avatar"] != "http://api.example.com/static/avatar/contact.png" ||
		int(data["gender"].(float64)) != 1 ||
		data["genderText"] != "男" ||
		data["businessNo"] != "GO-900001" ||
		data["remark"] != "重点客户" ||
		data["description"] != "需要继续跟进" {
		t.Fatalf("data = %#v", data)
	}
	tag := data["tag"].([]any)[0].(map[string]any)
	if int(tag["tagId"].(float64)) != 900001 || tag["tagName"] != "Go迁移标签" {
		t.Fatalf("tag = %#v", tag)
	}
	roomNames := data["roomName"].([]any)
	if roomNames[0] != "Go迁移客户群" {
		t.Fatalf("roomName = %#v", roomNames)
	}
	employeeNames := data["employeeName"].([]any)
	if employeeNames[0] != "Go迁移企业 Go迁移员工" {
		t.Fatalf("employeeName = %#v", employeeNames)
	}
}

func TestSidebarWorkContactShowRequiresContactID(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workContact/show", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkContactShow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "客户id必传" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestWorkContactShowReturnsBasicInfoWithAuthorization(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		workContactShow: WorkContactShow{
			Name:        "Go迁移客户",
			Avatar:      "avatar/contact.png",
			Gender:      2,
			BusinessNo:  "GO-900001",
			Remark:      "后台备注",
			Description: "后台描述",
			Tags: []WorkContactShowTag{
				{TagID: 900001, TagName: "Go迁移标签"},
			},
			RoomNames: []string{
				"Go迁移客户群",
			},
			EmployeeName: []string{
				"Go迁移企业 Go迁移员工",
			},
		},
		workContactShowFound: true,
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContact/show?contactId=900001&employeeId=1", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workContact/show#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if store.lastWorkContactShowContactID != 900001 || store.lastWorkContactShowEmployeeID != 1 {
		t.Fatalf("show lookup = contact %d employee %d", store.lastWorkContactShowContactID, store.lastWorkContactShowEmployeeID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if data["name"] != "Go迁移客户" ||
		data["avatar"] != "http://api.example.com/static/avatar/contact.png" ||
		int(data["gender"].(float64)) != 2 ||
		data["genderText"] != "女" ||
		data["businessNo"] != "GO-900001" ||
		data["remark"] != "后台备注" ||
		data["description"] != "后台描述" {
		t.Fatalf("data = %#v", data)
	}
}

func TestWorkContactShowRequiresEmployeeID(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContact/show?contactId=900001", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactShow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "员工id必传" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestWorkContactIndexReturnsPHPCompatiblePage(t *testing.T) {
	addWay := 1
	gender := 1
	fieldID := 900001
	groupNum := 1
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1, Phone: "13800000000", Name: "Go迁移用户", Gender: 1, Department: "迁移组", Position: "管理员", LoginTime: "2026-07-03 10:00:00", Status: 1, TenantID: 1, IsSuperAdmin: 1}},
		workContactIndexPage: WorkContactIndexPage{
			PerPage:         20,
			Total:           1,
			TotalPage:       1,
			SyncContactTime: "2026-07-02 12:00:00",
			Items: []WorkContactIndexItem{
				{
					ID:           900001,
					EmployeeID:   99,
					ContactID:    900001,
					Remark:       "Go迁移备注",
					CreateTime:   "2026-07-03 08:00:00",
					AddWay:       addWay,
					BusinessNo:   "GO-900001",
					Name:         "Go迁移客户",
					Avatar:       "avatar/contact.png",
					Gender:       gender,
					RoomName:     []string{"Go迁移客户群"},
					EmployeeName: "Go迁移员工",
					Tag:          []string{"Go迁移标签"},
					IsContact:    1,
				},
			},
		},
	}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{RoleID: 8, DataPermission: DataPermissionAll}}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContact/index?keyWords=Go&remark=%E8%BF%81%E7%A7%BB&fieldId=900001&fieldValue=A&gender=1&addWay=1&roomId=900001&groupNum=1&employeeId=99&startTime=2026-07-01&endTime=2026-07-03&businessNo=GO-900001&page=1&perPage=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workContact/index#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	filter := store.lastWorkContactIndexFilter
	if filter.CorpID != 7 || filter.CurrentEmployeeID != 99 || !filter.RestrictEmployees || len(filter.EmployeeIDs) != 1 || filter.EmployeeIDs[0] != 99 || filter.Remark != "迁移" || filter.KeyWords != "Go" || filter.BusinessNo != "GO-900001" || filter.AddWay == nil || *filter.AddWay != addWay || filter.Gender == nil || *filter.Gender != gender || filter.FieldID == nil || *filter.FieldID != fieldID || filter.FieldValue != "A" || len(filter.RoomIDs) != 1 || filter.RoomIDs[0] != 900001 || filter.GroupNum == nil || *filter.GroupNum != groupNum || filter.StartTime != "2026-07-01" || filter.EndTime != "2026-07-03" || filter.Page != 1 || filter.PerPage != 20 {
		t.Fatalf("filter = %#v", filter)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if int(page["perPage"].(float64)) != 20 || int(page["total"].(float64)) != 1 || int(page["totalPage"].(float64)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	if data["syncContactTime"] != "2026-07-02 12:00:00" {
		t.Fatalf("sync = %#v", data["syncContactTime"])
	}
	item := data["list"].([]any)[0].(map[string]any)
	if int(item["id"].(float64)) != 900001 ||
		int(item["employeeId"].(float64)) != 99 ||
		int(item["contactId"].(float64)) != 900001 ||
		item["remark"] != "Go迁移备注" ||
		item["createTime"] != "2026-07-03 08:00:00" ||
		item["addWayText"] != "扫描二维码" ||
		item["genderText"] != "男" ||
		item["businessNo"] != "GO-900001" ||
		item["name"] != "Go迁移客户" ||
		item["avatar"] != "http://api.example.com/static/avatar/contact.png" ||
		item["employeeName"] != "Go迁移员工" ||
		int(item["isContact"].(float64)) != 1 {
		t.Fatalf("item = %#v", item)
	}
	if item["roomName"].([]any)[0] != "Go迁移客户群" || item["tag"].([]any)[0] != "Go迁移标签" {
		t.Fatalf("item = %#v", item)
	}
	user := item["user"].(map[string]any)
	if int(user["id"].(float64)) != 1 || int(user["roleId"].(float64)) != 8 || int(user["workEmployeeId"].(float64)) != 99 {
		t.Fatalf("user = %#v", user)
	}
}

func TestWorkContactIndexAppliesDepartmentDataPermission(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		workContactIndexPage: WorkContactIndexPage{
			PerPage: 20,
			Items:   []WorkContactIndexItem{},
		},
	}
	authorizer := &recordingAuthorizer{
		accessSet: true,
		access: AccessContext{
			DataPermission:  DataPermissionDepartment,
			DeptEmployeeIDs: []int{10, 20},
		},
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContact/index?employeeId=10,30", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	filter := store.lastWorkContactIndexFilter
	if !filter.RestrictEmployees || len(filter.EmployeeIDs) != 1 || filter.EmployeeIDs[0] != 10 {
		t.Fatalf("filter = %#v", filter)
	}
}

func TestWorkContactLossReturnsPHPCompatiblePage(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1, Phone: "13800000000", Name: "Go迁移用户", Gender: 1, Department: "迁移组", Position: "管理员", LoginTime: "2026-07-03 10:00:00", Status: 1, TenantID: 1, IsSuperAdmin: 1}},
		workContactLossPage: WorkContactLossPage{
			PerPage:   20,
			Total:     1,
			TotalPage: 1,
			Items: []WorkContactLossItem{
				{
					ID:           900002,
					EmployeeID:   99,
					ContactID:    900001,
					DeletedAt:    "2026-07-03 09:00:00",
					Avatar:       "avatar/loss.png",
					Name:         "Go迁移流失客户",
					Tag:          []string{"Go迁移标签"},
					EmployeeName: "Go迁移企业 Go迁移员工",
					Remark:       "Go迁移员工",
				},
			},
		},
	}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{RoleID: 8, DataPermission: DataPermissionAll}}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContact/lossContact?employeeId=99&page=1&perPage=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactLoss(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workContact/lossContact#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	filter := store.lastWorkContactLossFilter
	if filter.CorpID != 7 || !filter.RestrictEmployees || len(filter.EmployeeIDs) != 1 || filter.EmployeeIDs[0] != 99 || filter.Page != 1 || filter.PerPage != 20 {
		t.Fatalf("filter = %#v", filter)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if int(page["perPage"].(float64)) != 20 || int(page["total"].(float64)) != 1 || int(page["totalPage"].(float64)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	item := data["list"].([]any)[0].(map[string]any)
	if int(item["id"].(float64)) != 900002 ||
		int(item["employeeId"].(float64)) != 99 ||
		int(item["contactId"].(float64)) != 900001 ||
		item["deletedAt"] != "2026-07-03 09:00:00" ||
		item["avatar"] != "http://api.example.com/static/avatar/loss.png" ||
		item["name"] != "Go迁移流失客户" ||
		item["employeeName"] != "Go迁移企业 Go迁移员工" ||
		item["remark"] != "Go迁移员工" {
		t.Fatalf("item = %#v", item)
	}
	if item["tag"].([]any)[0] != "Go迁移标签" {
		t.Fatalf("item = %#v", item)
	}
	user := item["user"].(map[string]any)
	if int(user["id"].(float64)) != 1 || int(user["roleId"].(float64)) != 8 || int(user["workEmployeeId"].(float64)) != 99 {
		t.Fatalf("user = %#v", user)
	}
}

func TestWorkContactLossAppliesDepartmentDataPermission(t *testing.T) {
	store := &fakeWorkReadStore{
		users:               map[int]User{1: {ID: 1}},
		workContactLossPage: WorkContactLossPage{PerPage: 20, Items: []WorkContactLossItem{}},
	}
	authorizer := &recordingAuthorizer{
		accessSet: true,
		access: AccessContext{
			DataPermission:  DataPermissionDepartment,
			DeptEmployeeIDs: []int{10, 20},
		},
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContact/lossContact?employeeId=10,30", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactLoss(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	filter := store.lastWorkContactLossFilter
	if !filter.RestrictEmployees || len(filter.EmployeeIDs) != 1 || filter.EmployeeIDs[0] != 10 {
		t.Fatalf("filter = %#v", filter)
	}
}

func TestWorkContactRoomIndexReturnsMembersWithAuthorization(t *testing.T) {
	status := 1
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		workContactRoomPage: WorkContactRoomPage{
			MemberNum:  1,
			OutRoomNum: 0,
			PerPage:    10,
			Total:      1,
			TotalPage:  1,
			Items: []WorkContactRoomItem{
				{
					WorkContactRoomID: 900001,
					Name:              "Go迁移客户",
					Avatar:            "avatar/contact.png",
					IsOwner:           0,
					JoinTime:          "2026-07-03 09:00:00",
					OutRoomTime:       "",
					OtherRooms:        []string{"Go迁移其他群"},
					JoinScene:         3,
					Type:              2,
					ContactID:         900001,
					EmployeeID:        0,
					ContactEmployeeID: 1,
				},
			},
		},
		workContactRoomFound: true,
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "http://api.example.com", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContactRoom/index?workRoomId=900001&status=1&name=Go&page=1&perPage=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactRoomIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workContactRoom/index#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	filter := store.lastWorkContactRoomFilter
	if filter.WorkRoomID != 900001 || filter.CorpID != 7 || filter.Status == nil || *filter.Status != status || filter.Name != "Go" || filter.Page != 1 || filter.PerPage != 10 {
		t.Fatalf("filter = %#v", filter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["memberNum"].(float64)) != 1 || int(data["outRoomNum"].(float64)) != 0 {
		t.Fatalf("data = %#v", data)
	}
	page := data["page"].(map[string]any)
	if page["perPage"] != "10" {
		t.Fatalf("page = %#v", page)
	}
	item := data["list"].([]any)[0].(map[string]any)
	if int(item["workContactRoomId"].(float64)) != 900001 ||
		item["name"] != "Go迁移客户" ||
		item["avatar"] != "http://api.example.com/static/avatar/contact.png" ||
		item["joinSceneText"] != "通过扫描群二维码入群" ||
		int(item["contactId"].(float64)) != 900001 ||
		int(item["contactEmployeeId"].(float64)) != 1 {
		t.Fatalf("item = %#v", item)
	}
}

func TestWorkContactRoomIndexIgnoresZeroStatus(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		workContactRoomPage: WorkContactRoomPage{
			PerPage: 10,
			Items:   []WorkContactRoomItem{},
		},
		workContactRoomFound: true,
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContactRoom/index?workRoomId=900001&status=0", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactRoomIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastWorkContactRoomFilter.Status != nil {
		t.Fatalf("status filter = %#v", store.lastWorkContactRoomFilter.Status)
	}
}

func TestWorkRoomRoomIndexReturnsOptions(t *testing.T) {
	roomGroupID := 9
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		workRoomOptions: WorkRoomOptionPage{
			Total: 1,
			Items: []WorkRoomOption{
				{RoomID: 900001, RoomName: "Go迁移客户群", RoomMax: 500, CurrentNum: 3},
			},
		},
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoom/roomIndex?name=Go&roomGroupId=9", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomRoomIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	filter := store.lastWorkRoomOptionFilter
	if len(filter.CorpIDs) != 1 || filter.CorpIDs[0] != 7 || filter.Name != "Go" || filter.RoomGroupID == nil || *filter.RoomGroupID != roomGroupID {
		t.Fatalf("filter = %#v", filter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["total"].(float64)) != 1 {
		t.Fatalf("data = %#v", data)
	}
	item := data["list"].([]any)[0].(map[string]any)
	if int(item["roomId"].(float64)) != 900001 || item["roomName"] != "Go迁移客户群" || int(item["roomMax"].(float64)) != 500 || int(item["currentNum"].(float64)) != 3 {
		t.Fatalf("item = %#v", item)
	}
}

func TestWorkRoomIndexReturnsPHPCompatibleList(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		workRoomIndexPage: WorkRoomIndexPage{
			PerPage:   10,
			Total:     1,
			TotalPage: 1,
			Items: []WorkRoomIndexItem{
				{
					WorkRoomID: 900001,
					MemberNum:  2,
					RoomName:   "Go迁移客户群",
					OwnerName:  "Go迁移企业-Go迁移员工",
					RoomGroup:  "重点客户群",
					Status:     0,
					InRoomNum:  1,
					OutRoomNum: 1,
					Notice:     "群公告",
					CreateTime: "2026-07-03 09:00:00",
				},
			},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoom/index?workRoomName=Go&workRoomOwnerId=99&roomGroupId=0&workRoomStatus=0&startTime=2026-07-01&endTime=2026-07-03&page=1&perPage=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workRoom/index#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	filter := store.lastWorkRoomIndexFilter
	if filter.CorpID != 7 || !filter.RestrictOwner || len(filter.OwnerIDs) != 1 || filter.OwnerIDs[0] != 99 || filter.RoomGroupID == nil || *filter.RoomGroupID != 0 || filter.Status == nil || *filter.Status != 0 || filter.Name != "Go" || filter.StartTime != "2026-07-01" || filter.EndTime != "2026-07-03" || filter.Page != 1 || filter.PerPage != 10 {
		t.Fatalf("filter = %#v", filter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if page["perPage"] != "10" || int(page["total"].(float64)) != 1 || int(page["totalPage"].(float64)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	item := data["list"].([]any)[0].(map[string]any)
	if int(item["workRoomId"].(float64)) != 900001 ||
		int(item["memberNum"].(float64)) != 2 ||
		item["roomName"] != "Go迁移客户群" ||
		item["ownerName"] != "Go迁移企业-Go迁移员工" ||
		item["roomGroup"] != "重点客户群" ||
		int(item["status"].(float64)) != 0 ||
		item["statusText"] != "正常" ||
		int(item["inRoomNum"].(float64)) != 1 ||
		int(item["outRoomNum"].(float64)) != 1 ||
		item["notice"] != "群公告" ||
		item["createTime"] != "2026-07-03 09:00:00" {
		t.Fatalf("item = %#v", item)
	}
}

func TestWorkRoomIndexAppliesDataPermissionToOwners(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		workRoomIndexPage: WorkRoomIndexPage{
			PerPage: 10,
			Items:   []WorkRoomIndexItem{},
		},
	}
	authorizer := &recordingAuthorizer{
		accessSet: true,
		access: AccessContext{
			DataPermission:  DataPermissionDepartment,
			DeptEmployeeIDs: []int{10, 20},
		},
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoom/index?workRoomOwnerId=10,30", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	filter := store.lastWorkRoomIndexFilter
	if !filter.RestrictOwner || len(filter.OwnerIDs) != 1 || filter.OwnerIDs[0] != 10 {
		t.Fatalf("filter = %#v", filter)
	}
}

func TestSidebarWorkRoomManageReturnsEmptyWhenRoomExists(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees:     map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		workRoomManageExists: true,
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workRoom/roomManage?roomId=go-room-900001", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkRoomManage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSidebarEmployeeID != 5 || store.lastWorkRoomManageCorpID != 7 || store.lastWorkRoomManageWXChatID != "go-room-900001" {
		t.Fatalf("store = %#v", store)
	}
	body := decodeBody(t, rec.Body.Bytes())
	if len(body["data"].([]any)) != 0 {
		t.Fatalf("body = %#v", body)
	}
}

func TestSidebarWorkRoomManageRequiresRoomID(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workRoom/roomManage", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkRoomManage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "roomId 必传" {
		t.Fatalf("body = %#v", body)
	}
}

func TestSidebarWorkRoomManageRejectsMissingRoom(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workRoom/roomManage?roomId=missing-room", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkRoomManage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "群不存在" {
		t.Fatalf("body = %#v", body)
	}
}

func TestWorkRoomStatisticsReturnsPHPCompatibleSummary(t *testing.T) {
	today := time.Now()
	todayText := today.Format("2006-01-02")
	yesterdayText := today.AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysText := today.AddDate(0, 0, -2).Format("2006-01-02")
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		workRoomMemberStats: []WorkRoomMemberStat{
			{Status: 1, JoinTime: twoDaysText + " 08:00:00"},
			{Status: 2, JoinTime: yesterdayText + " 08:00:00", OutTime: todayText + " 09:00:00"},
			{Status: 1, JoinTime: todayText + " 08:00:00"},
		},
		workRoomMemberStatsFound: true,
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoom/statistics?workRoomId=900001&type=1&startTime="+twoDaysText+"&endTime="+todayText, nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomStatistics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workRoom/statistics#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if store.lastWorkRoomStatsID != 900001 {
		t.Fatalf("work room id = %d", store.lastWorkRoomStatsID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["total"].(float64)) != 2 || int(data["outTotal"].(float64)) != 1 || int(data["addNum"].(float64)) != 1 || int(data["outNum"].(float64)) != 1 || int(data["addNumRange"].(float64)) != 3 || int(data["outNumRange"].(float64)) != 1 {
		t.Fatalf("data = %#v", data)
	}
	list := data["list"].([]any)
	if len(list) != 3 {
		t.Fatalf("list = %#v", list)
	}
	first := list[0].(map[string]any)
	if first["time"] != twoDaysText || int(first["addNum"].(float64)) != 1 || int(first["outNum"].(float64)) != 0 {
		t.Fatalf("first = %#v", first)
	}
	last := list[2].(map[string]any)
	if last["time"] != todayText || int(last["addNum"].(float64)) != 1 || int(last["outNum"].(float64)) != 1 {
		t.Fatalf("last = %#v", last)
	}
}

func TestWorkRoomStatisticsIndexReturnsCumulativePage(t *testing.T) {
	today := time.Now()
	todayText := today.Format("2006-01-02")
	yesterdayText := today.AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysText := today.AddDate(0, 0, -2).Format("2006-01-02")
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		workRoomMemberStats: []WorkRoomMemberStat{
			{Status: 1, JoinTime: twoDaysText + " 08:00:00"},
			{Status: 2, JoinTime: yesterdayText + " 08:00:00", OutTime: todayText + " 09:00:00"},
			{Status: 1, JoinTime: todayText + " 08:00:00"},
		},
		workRoomMemberStatsFound: true,
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoom/statisticsIndex?workRoomId=900001&type=1&startTime="+twoDaysText+"&endTime="+todayText+"&page=1&perPage=2", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomStatisticsIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if page["perPage"] != "2" || int(page["total"].(float64)) != 3 || int(page["totalPage"].(float64)) != 2 {
		t.Fatalf("page = %#v", page)
	}
	list := data["list"].([]any)
	if len(list) != 2 {
		t.Fatalf("list = %#v", list)
	}
	first := list[0].(map[string]any)
	if first["time"] != twoDaysText || int(first["addNum"].(float64)) != 1 || int(first["total"].(float64)) != 1 || int(first["outTotal"].(float64)) != 0 {
		t.Fatalf("first = %#v", first)
	}
	second := list[1].(map[string]any)
	if second["time"] != yesterdayText || int(second["addNum"].(float64)) != 1 || int(second["total"].(float64)) != 2 || int(second["outTotal"].(float64)) != 0 {
		t.Fatalf("second = %#v", second)
	}
}

func TestWorkRoomStatisticsRequiresDayRange(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoom/statistics?workRoomId=900001&type=1", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomStatistics(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "按天统计开始和结束时间必传" {
		t.Fatalf("body = %#v", body)
	}
}

func TestSidebarWorkContactTrackReturnsTracks(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		contactEmployeeTracks: []ContactEmployeeTrack{
			{ID: 900001, Content: "添加了客户", CreatedAt: "2026-07-03 09:30:00"},
		},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workContact/track?contactId=900001", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkContactTrack(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSidebarEmployeeID != 5 {
		t.Fatalf("lastSidebarEmployeeID = %d", store.lastSidebarEmployeeID)
	}
	if store.lastContactTrackContactID != 900001 {
		t.Fatalf("contactID = %d", store.lastContactTrackContactID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	track := data[0].(map[string]any)
	if int(track["id"].(float64)) != 900001 || track["content"] != "添加了客户" || track["createdAt"] != "2026-07-03 09:30:00" {
		t.Fatalf("track = %#v", track)
	}
}

func TestWorkContactTrackReturnsTracksWithAuthorization(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
		contactEmployeeTracks: []ContactEmployeeTrack{
			{ID: 900001, Content: "添加了客户", CreatedAt: "2026-07-03 09:30:00"},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "", authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContact/track?contactId=900001", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactTrack(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workContact/track#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if store.lastContactTrackContactID != 900001 {
		t.Fatalf("contactID = %d", store.lastContactTrackContactID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	track := data[0].(map[string]any)
	if int(track["id"].(float64)) != 900001 || track["content"] != "添加了客户" || track["createdAt"] != "2026-07-03 09:30:00" {
		t.Fatalf("track = %#v", track)
	}
}

func TestWorkContactSourceReturnsPHPAddWayEnum(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workContact/source", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactSource(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	if len(data) != 15 {
		t.Fatalf("source count = %d, data=%#v", len(data), data)
	}
	first := data[0].(map[string]any)
	if int(first["addWay"].(float64)) != 0 || first["addWayText"] != "其他渠道" {
		t.Fatalf("first = %#v", first)
	}
	channelCode := data[2].(map[string]any)
	if int(channelCode["addWay"].(float64)) != 1001 || channelCode["addWayText"] != "渠道活码" {
		t.Fatalf("channel code = %#v", channelCode)
	}
	last := data[len(data)-1].(map[string]any)
	if int(last["addWay"].(float64)) != 202 || last["addWayText"] != "管理员/负责人分配" {
		t.Fatalf("last = %#v", last)
	}
}

func TestSidebarContactProcessStatusIndexReturnsExistingStatuses(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		processStatuses: []ContactProcessStatus{
			{ID: 11, Name: "新客户"},
			{ID: 12, Name: "初步沟通"},
		},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/contactProcessStatus/index", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarContactProcessStatusIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastProcessCorpID != 7 || store.defaultProcessesCreated {
		t.Fatalf("store = %#v", store)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	first := data[0].(map[string]any)
	if int(first["id"].(float64)) != 11 || first["name"] != "新客户" {
		t.Fatalf("first = %#v", first)
	}
}

func TestSidebarContactProcessStatusIndexCreatesDefaultsWhenEmpty(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/contactProcessStatus/index", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarContactProcessStatusIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastProcessCorpID != 7 || !store.defaultProcessesCreated {
		t.Fatalf("store = %#v", store)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	if len(data) != 5 {
		t.Fatalf("default status count = %d, data=%#v", len(data), data)
	}
	last := data[len(data)-1].(map[string]any)
	if int(last["id"].(float64)) != 5 || last["name"] != "无意向客户" {
		t.Fatalf("last = %#v", last)
	}
}

func TestSidebarContactProcessStatusUpdateWritesTrackAndStatus(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees:   map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		processStatus:      ContactProcessStatus{ID: 3, Name: "意向客户"},
		processStatusFound: true,
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/sidebar/contactProcessStatus/update", strings.NewReader(`{"contactId":21,"statusId":3}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarContactProcessStatusUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastProcessStatusID != 3 {
		t.Fatalf("status lookup = %d", store.lastProcessStatusID)
	}
	update := store.lastProcessUpdate
	if update.ContactID != 21 || update.StatusID != 3 || update.EmployeeID != 5 || update.CorpID != 7 || update.Event != 5 {
		t.Fatalf("update = %#v", update)
	}
	if update.Content != "编辑用户跟进状态：意向客户" {
		t.Fatalf("content = %q", update.Content)
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["code"].(float64) != 200 {
		t.Fatalf("body = %#v", body)
	}
}

func TestSidebarContactProcessStatusUpdateRequiresStatusID(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/sidebar/contactProcessStatus/update", strings.NewReader(`{"contactId":21}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarContactProcessStatusUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "跟进状态id必传" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestSidebarWorkContactTrackRequiresContactID(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
	}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/workContact/track", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkContactTrack(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "客户id必传" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

type fakeWorkReadStore struct {
	users                             map[int]User
	sidebarEmployees                  map[int]SidebarEmployee
	corpIDsByTenant                   []int
	corpIDsByUser                     []int
	firstCorpID                       int
	firstEmployeeID                   int
	syncTime                          string
	contactTagSyncTime                string
	departments                       []WorkDepartment
	employees                         []WorkDepartmentEmployee
	members                           []WorkDepartmentMember
	phoneDepartments                  []WorkDepartmentPhoneOption
	employeePage                      WorkDepartmentEmployeePage
	workEmployeeIndexPage             WorkEmployeeIndexPage
	employeeSyncCredentials           []WorkEmployeeSyncCredential
	employeeSyncResult                WorkEmployeeSyncResult
	workContactSyncEmployees          []WorkContactSyncEmployee
	workContactSyncResult             WorkContactSyncResult
	workContactSyncErr                error
	contactTagGroups                  []WorkContactTagGroup
	contactTagGroup                   WorkContactTagGroup
	contactTagGroupFound              bool
	contactTagPage                    WorkContactTagPage
	contactTag                        WorkContactTagDetail
	contactTagFound                   bool
	contactTagOptions                 []WorkContactTagOption
	contactTagList                    []WorkContactTagListGroup
	contactTagGroupNameExists         bool
	contactTagNamesExist              bool
	createdContactTagGroup            WorkContactTagGroupWrite
	updatedContactTagGroupCorpID      int
	updatedContactTagGroupID          int
	updatedContactTagGroupName        string
	updatedContactTagGroupWXIDCorpID  int
	updatedContactTagGroupWXIDGroupID int
	updatedContactTagGroupWXID        string
	deletedContactTagGroupCorpID      int
	deletedContactTagGroupID          int
	createdContactTags                WorkContactTagWrite
	updatedContactTagWXIDsCorpID      int
	updatedContactTagWXIDsGroupID     int
	updatedContactTagWXIDs            map[string]string
	updatedContactTagCorpID           int
	updatedContactTagID               int
	updatedContactTagTargetGroupID    int
	updatedContactTagName             string
	deletedContactTagCorpID           int
	deletedContactTagIDs              []int
	movedContactTagCorpID             int
	movedContactTagIDs                []int
	movedContactTagGroupID            int
	contactTagSyncResult              WorkContactTagSyncResult
	workContactDetail                 WorkContactDetail
	workContactFound                  bool
	workContactIndexPage              WorkContactIndexPage
	workContactLossPage               WorkContactLossPage
	workContactShow                   WorkContactShow
	workContactShowFound              bool
	denyContactAccess                 bool
	workContactUpdateResult           WorkContactUpdateResult
	workContactUpdateFound            bool
	batchLabelInserted                int
	roomWelcomeCredential             RoomWelcomeCorpCredential
	roomWelcomeCredentialFound        bool
	workContactRoomPage               WorkContactRoomPage
	workContactRoomFound              bool
	workRoomOptions                   WorkRoomOptionPage
	workRoomIndexPage                 WorkRoomIndexPage
	workRoomManageExists              bool
	workRoomMemberStats               []WorkRoomMemberStat
	workRoomMemberStatsFound          bool
	workRoomGroup                     WorkRoomGroupItem
	workRoomGroupFound                bool
	workRoomBatchUpdated              int
	workRoomSyncResult                WorkRoomSyncResult
	workRoomSyncErr                   error
	contactEmployeeTracks             []ContactEmployeeTrack
	processStatuses                   []ContactProcessStatus
	processStatus                     ContactProcessStatus
	processStatusFound                bool
	lastProcessUpdate                 ContactProcessStatusUpdate
	defaultProcessesCreated           bool
	lastCorpIDsByTenantID             int
	lastCorpIDsByUserID               int
	lastSyncCorpIDs                   []int
	lastContactTagSyncCorpIDs         []int
	lastDepartmentCorpID              int
	lastDepartmentSearch              string
	lastEmployeeCorpID                int
	lastEmployeeSearch                string
	lastMemberCorpID                  int
	lastMemberDepartmentIDs           []int
	lastPhoneCorpID                   int
	lastPhone                         string
	lastEmployeePageFilter            WorkDepartmentEmployeeListFilter
	lastWorkEmployeeIndexFilter       WorkEmployeeIndexFilter
	lastEmployeeSyncCredentialCorpIDs []int
	lastEmployeeSyncCredential        WorkEmployeeSyncCredential
	lastEmployeeSyncDepartments       []WorkEmployeeSyncDepartment
	lastEmployeeSyncEmployees         []WorkEmployeeSyncEmployee
	lastEmployeeSyncFollowUsers       []string
	lastEmployeeSyncPasswordHash      string
	lastWorkContactSyncCorpID         int
	lastWorkContactSyncBundles        []WorkContactSyncEmployeeContacts
	lastContactTagGroupCorpIDs        []int
	lastContactTagGroupID             int
	lastContactTagGroupNameCorpID     int
	lastContactTagGroupName           string
	lastContactTagGroupNameExcludeID  int
	lastContactTagFilter              WorkContactTagFilter
	lastContactTagID                  int
	lastContactTagsCorpIDs            []int
	lastContactTagsGroupID            *int
	lastContactTagNameCheckCorpID     int
	lastContactTagNameCheckGroupID    int
	lastContactTagNameCheckNames      []string
	lastContactTagNameCheckExcludeIDs []int
	lastContactTagSyncCorpID          int
	lastContactTagSyncGroups          []WorkContactTagSyncGroup
	lastSidebarEmployeeID             int
	lastContactTagListCorpIDs         []int
	lastContactTagListName            string
	lastWorkContactExternalUserID     string
	lastWorkContactIndexFilter        WorkContactIndexFilter
	lastWorkContactLossFilter         WorkContactLossFilter
	lastWorkContactShowContactID      int
	lastWorkContactShowEmployeeID     int
	lastWorkContactUpdate             WorkContactUpdateValues
	lastBatchLabelContactIDs          []int
	lastBatchLabelTagIDs              []int
	lastBatchLabelEmployeeID          int
	lastCredentialCorpID              int
	lastWorkContactRoomFilter         WorkContactRoomFilter
	lastWorkRoomOptionFilter          WorkRoomOptionFilter
	lastWorkRoomIndexFilter           WorkRoomIndexFilter
	lastWorkRoomManageCorpID          int
	lastWorkRoomManageWXChatID        string
	lastWorkRoomStatsID               int
	lastWorkRoomGroupID               int
	lastWorkRoomBatchUpdate           WorkRoomBatchUpdateValues
	lastWorkRoomSyncCorpID            int
	lastWorkRoomSyncRooms             []WorkRoomSyncRoom
	lastContactTrackContactID         int
	lastProcessCorpID                 int
	lastProcessStatusID               int
}

func (s *fakeWorkReadStore) ContactAccessibleToEmployee(_ context.Context, _ int, _ int, _ int) (bool, error) {
	return !s.denyContactAccess, nil
}

func (s *fakeWorkReadStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeWorkReadStore) SidebarEmployeeByID(_ context.Context, employeeID int) (SidebarEmployee, bool, error) {
	s.lastSidebarEmployeeID = employeeID
	employee, ok := s.sidebarEmployees[employeeID]
	return employee, ok, nil
}

func (s *fakeWorkReadStore) EmployeeIDByUserCorp(_ context.Context, userID int, _ int) (int, error) {
	if user := s.users[userID]; user.IsSuperAdmin == 1 {
		return 99, nil
	}
	return 0, nil
}

func (s *fakeWorkReadStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	if s.firstCorpID == 0 {
		return 0, 0, false, nil
	}
	return s.firstCorpID, s.firstEmployeeID, true, nil
}

func (s *fakeWorkReadStore) CorpIDsByTenant(_ context.Context, tenantID int) ([]int, error) {
	s.lastCorpIDsByTenantID = tenantID
	if s.corpIDsByTenant == nil {
		return []int{7}, nil
	}
	return append([]int{}, s.corpIDsByTenant...), nil
}

func (s *fakeWorkReadStore) CorpIDsByUser(_ context.Context, userID int) ([]int, error) {
	s.lastCorpIDsByUserID = userID
	if s.corpIDsByUser == nil {
		return []int{7}, nil
	}
	return append([]int{}, s.corpIDsByUser...), nil
}

func (s *fakeWorkReadStore) WorkEmployeeSyncTime(_ context.Context, corpIDs []int) (string, error) {
	s.lastSyncCorpIDs = append([]int{}, corpIDs...)
	return s.syncTime, nil
}

func (s *fakeWorkReadStore) WorkContactTagSyncTime(_ context.Context, corpIDs []int) (string, error) {
	s.lastContactTagSyncCorpIDs = append([]int{}, corpIDs...)
	return s.contactTagSyncTime, nil
}

func (s *fakeWorkReadStore) WorkDepartmentsByCorp(_ context.Context, corpID int, search string) ([]WorkDepartment, error) {
	s.lastDepartmentCorpID = corpID
	s.lastDepartmentSearch = search
	return s.departments, nil
}

func (s *fakeWorkReadStore) ActiveWorkEmployeesByCorp(_ context.Context, corpID int, search string) ([]WorkDepartmentEmployee, error) {
	s.lastEmployeeCorpID = corpID
	s.lastEmployeeSearch = search
	return s.employees, nil
}

func (s *fakeWorkReadStore) WorkDepartmentMembers(_ context.Context, corpID int, departmentIDs []int) ([]WorkDepartmentMember, error) {
	s.lastMemberCorpID = corpID
	s.lastMemberDepartmentIDs = append([]int{}, departmentIDs...)
	return s.members, nil
}

func (s *fakeWorkReadStore) WorkDepartmentsByEmployeeMobile(_ context.Context, corpID int, phone string) ([]WorkDepartmentPhoneOption, error) {
	s.lastPhoneCorpID = corpID
	s.lastPhone = phone
	return s.phoneDepartments, nil
}

func (s *fakeWorkReadStore) WorkDepartmentEmployeePage(_ context.Context, filter WorkDepartmentEmployeeListFilter) (WorkDepartmentEmployeePage, error) {
	s.lastEmployeePageFilter = filter
	return s.employeePage, nil
}

func (s *fakeWorkReadStore) WorkEmployeeIndexPage(_ context.Context, filter WorkEmployeeIndexFilter) (WorkEmployeeIndexPage, error) {
	s.lastWorkEmployeeIndexFilter = filter
	return s.workEmployeeIndexPage, nil
}

func (s *fakeWorkReadStore) WorkEmployeeSyncCredentials(_ context.Context, corpIDs []int) ([]WorkEmployeeSyncCredential, error) {
	s.lastEmployeeSyncCredentialCorpIDs = append([]int{}, corpIDs...)
	return s.employeeSyncCredentials, nil
}

func (s *fakeWorkReadStore) SyncWorkEmployees(_ context.Context, credential WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee, followUserIDs []string, defaultPasswordHash string) (WorkEmployeeSyncResult, error) {
	s.lastEmployeeSyncCredential = credential
	s.lastEmployeeSyncDepartments = append([]WorkEmployeeSyncDepartment{}, departments...)
	s.lastEmployeeSyncEmployees = append([]WorkEmployeeSyncEmployee{}, employees...)
	s.lastEmployeeSyncFollowUsers = append([]string{}, followUserIDs...)
	s.lastEmployeeSyncPasswordHash = defaultPasswordHash
	return s.employeeSyncResult, nil
}

func (s *fakeWorkReadStore) WorkContactSyncEmployees(_ context.Context, corpID int) ([]WorkContactSyncEmployee, error) {
	s.lastWorkContactSyncCorpID = corpID
	return append([]WorkContactSyncEmployee{}, s.workContactSyncEmployees...), nil
}

func (s *fakeWorkReadStore) SyncWorkContacts(_ context.Context, corpID int, bundles []WorkContactSyncEmployeeContacts) (WorkContactSyncResult, error) {
	s.lastWorkContactSyncCorpID = corpID
	s.lastWorkContactSyncBundles = append([]WorkContactSyncEmployeeContacts{}, bundles...)
	if s.workContactSyncErr != nil {
		return WorkContactSyncResult{}, s.workContactSyncErr
	}
	return s.workContactSyncResult, nil
}

func (s *fakeWorkReadStore) WorkContactTagGroupsByCorp(_ context.Context, corpIDs []int) ([]WorkContactTagGroup, error) {
	s.lastContactTagGroupCorpIDs = append([]int{}, corpIDs...)
	return s.contactTagGroups, nil
}

func (s *fakeWorkReadStore) WorkContactTagGroupByID(_ context.Context, groupID int) (WorkContactTagGroup, bool, error) {
	s.lastContactTagGroupID = groupID
	return s.contactTagGroup, s.contactTagGroupFound, nil
}

func (s *fakeWorkReadStore) WorkContactTagPage(_ context.Context, filter WorkContactTagFilter) (WorkContactTagPage, error) {
	s.lastContactTagFilter = filter
	return s.contactTagPage, nil
}

func (s *fakeWorkReadStore) WorkContactTagByID(_ context.Context, tagID int) (WorkContactTagDetail, bool, error) {
	s.lastContactTagID = tagID
	return s.contactTag, s.contactTagFound, nil
}

func (s *fakeWorkReadStore) WorkContactTags(_ context.Context, corpIDs []int, groupID *int) ([]WorkContactTagOption, error) {
	s.lastContactTagsCorpIDs = append([]int{}, corpIDs...)
	if groupID != nil {
		value := *groupID
		s.lastContactTagsGroupID = &value
	} else {
		s.lastContactTagsGroupID = nil
	}
	return s.contactTagOptions, nil
}

func (s *fakeWorkReadStore) WorkContactTagList(_ context.Context, corpIDs []int, name string) ([]WorkContactTagListGroup, error) {
	s.lastContactTagListCorpIDs = append([]int{}, corpIDs...)
	s.lastContactTagListName = name
	return s.contactTagList, nil
}

func (s *fakeWorkReadStore) WorkContactTagGroupNameExists(_ context.Context, corpID int, groupName string, excludeGroupID int) (bool, error) {
	s.lastContactTagGroupNameCorpID = corpID
	s.lastContactTagGroupName = groupName
	s.lastContactTagGroupNameExcludeID = excludeGroupID
	return s.contactTagGroupNameExists, nil
}

func (s *fakeWorkReadStore) CreateWorkContactTagGroup(_ context.Context, values WorkContactTagGroupWrite) (int, error) {
	s.createdContactTagGroup = values
	return 101, nil
}

func (s *fakeWorkReadStore) UpdateWorkContactTagGroup(_ context.Context, corpID int, groupID int, groupName string) (bool, error) {
	s.updatedContactTagGroupCorpID = corpID
	s.updatedContactTagGroupID = groupID
	s.updatedContactTagGroupName = groupName
	return true, nil
}

func (s *fakeWorkReadStore) UpdateWorkContactTagGroupWXID(_ context.Context, corpID int, groupID int, wxGroupID string) (bool, error) {
	s.updatedContactTagGroupWXIDCorpID = corpID
	s.updatedContactTagGroupWXIDGroupID = groupID
	s.updatedContactTagGroupWXID = wxGroupID
	return true, nil
}

func (s *fakeWorkReadStore) DeleteWorkContactTagGroupCascade(_ context.Context, corpID int, groupID int) (bool, error) {
	s.deletedContactTagGroupCorpID = corpID
	s.deletedContactTagGroupID = groupID
	return true, nil
}

func (s *fakeWorkReadStore) WorkContactTagNamesExist(_ context.Context, corpID int, groupID int, names []string, excludeTagIDs []int) (bool, error) {
	s.lastContactTagNameCheckCorpID = corpID
	s.lastContactTagNameCheckGroupID = groupID
	s.lastContactTagNameCheckNames = append([]string{}, names...)
	s.lastContactTagNameCheckExcludeIDs = append([]int{}, excludeTagIDs...)
	return s.contactTagNamesExist, nil
}

func (s *fakeWorkReadStore) CreateWorkContactTags(_ context.Context, values WorkContactTagWrite) error {
	s.createdContactTags = values
	return nil
}

func (s *fakeWorkReadStore) UpdateWorkContactTagWXIDsByName(_ context.Context, corpID int, groupID int, tagWXIDs map[string]string) error {
	s.updatedContactTagWXIDsCorpID = corpID
	s.updatedContactTagWXIDsGroupID = groupID
	s.updatedContactTagWXIDs = map[string]string{}
	for name, wxTagID := range tagWXIDs {
		s.updatedContactTagWXIDs[name] = wxTagID
	}
	return nil
}

func (s *fakeWorkReadStore) UpdateWorkContactTag(_ context.Context, corpID int, tagID int, groupID int, tagName string) (bool, error) {
	s.updatedContactTagCorpID = corpID
	s.updatedContactTagID = tagID
	s.updatedContactTagTargetGroupID = groupID
	s.updatedContactTagName = tagName
	return true, nil
}

func (s *fakeWorkReadStore) DeleteWorkContactTags(_ context.Context, corpID int, tagIDs []int) (bool, error) {
	s.deletedContactTagCorpID = corpID
	s.deletedContactTagIDs = append([]int{}, tagIDs...)
	return true, nil
}

func (s *fakeWorkReadStore) MoveWorkContactTags(_ context.Context, corpID int, tagIDs []int, groupID int) (bool, error) {
	s.movedContactTagCorpID = corpID
	s.movedContactTagIDs = append([]int{}, tagIDs...)
	s.movedContactTagGroupID = groupID
	return true, nil
}

func (s *fakeWorkReadStore) SyncWorkContactTags(_ context.Context, corpID int, groups []WorkContactTagSyncGroup) (WorkContactTagSyncResult, error) {
	s.lastContactTagSyncCorpID = corpID
	s.lastContactTagSyncGroups = append([]WorkContactTagSyncGroup{}, groups...)
	return s.contactTagSyncResult, nil
}

func (s *fakeWorkReadStore) WorkContactByExternalUserID(_ context.Context, externalUserID string) (WorkContactDetail, bool, error) {
	s.lastWorkContactExternalUserID = externalUserID
	return s.workContactDetail, s.workContactFound, nil
}

func (s *fakeWorkReadStore) WorkContactIndexPage(_ context.Context, filter WorkContactIndexFilter) (WorkContactIndexPage, error) {
	s.lastWorkContactIndexFilter = filter
	return s.workContactIndexPage, nil
}

func (s *fakeWorkReadStore) WorkContactLossPage(_ context.Context, filter WorkContactLossFilter) (WorkContactLossPage, error) {
	s.lastWorkContactLossFilter = filter
	return s.workContactLossPage, nil
}

func (s *fakeWorkReadStore) WorkContactShowByID(_ context.Context, contactID int, employeeID int) (WorkContactShow, bool, error) {
	s.lastWorkContactShowContactID = contactID
	s.lastWorkContactShowEmployeeID = employeeID
	return s.workContactShow, s.workContactShowFound, nil
}

func (s *fakeWorkReadStore) WorkContactRoomIndex(_ context.Context, filter WorkContactRoomFilter) (WorkContactRoomPage, bool, error) {
	s.lastWorkContactRoomFilter = filter
	return s.workContactRoomPage, s.workContactRoomFound, nil
}

func (s *fakeWorkReadStore) WorkRoomOptions(_ context.Context, filter WorkRoomOptionFilter) (WorkRoomOptionPage, error) {
	s.lastWorkRoomOptionFilter = filter
	return s.workRoomOptions, nil
}

func (s *fakeWorkReadStore) WorkRoomIndexPage(_ context.Context, filter WorkRoomIndexFilter) (WorkRoomIndexPage, error) {
	s.lastWorkRoomIndexFilter = filter
	return s.workRoomIndexPage, nil
}

func (s *fakeWorkReadStore) WorkRoomExistsByCorpWXChatID(_ context.Context, corpID int, wxChatID string) (bool, error) {
	s.lastWorkRoomManageCorpID = corpID
	s.lastWorkRoomManageWXChatID = wxChatID
	return s.workRoomManageExists, nil
}

func (s *fakeWorkReadStore) WorkRoomMemberStats(_ context.Context, workRoomID int) ([]WorkRoomMemberStat, bool, error) {
	s.lastWorkRoomStatsID = workRoomID
	return s.workRoomMemberStats, s.workRoomMemberStatsFound, nil
}

func (s *fakeWorkReadStore) WorkRoomGroupByID(_ context.Context, groupID int) (WorkRoomGroupItem, bool, error) {
	s.lastWorkRoomGroupID = groupID
	return s.workRoomGroup, s.workRoomGroupFound, nil
}

func (s *fakeWorkReadStore) UpdateWorkRoomsGroup(_ context.Context, values WorkRoomBatchUpdateValues) (int, error) {
	s.lastWorkRoomBatchUpdate = WorkRoomBatchUpdateValues{
		CorpID:          values.CorpID,
		WorkRoomGroupID: values.WorkRoomGroupID,
		WorkRoomIDs:     append([]int{}, values.WorkRoomIDs...),
	}
	return s.workRoomBatchUpdated, nil
}

func (s *fakeWorkReadStore) SyncWorkRooms(_ context.Context, corpID int, rooms []WorkRoomSyncRoom) (WorkRoomSyncResult, error) {
	s.lastWorkRoomSyncCorpID = corpID
	s.lastWorkRoomSyncRooms = append([]WorkRoomSyncRoom{}, rooms...)
	if s.workRoomSyncErr != nil {
		return WorkRoomSyncResult{}, s.workRoomSyncErr
	}
	return s.workRoomSyncResult, nil
}

func (s *fakeWorkReadStore) ContactEmployeeTracksByContactID(_ context.Context, contactID int) ([]ContactEmployeeTrack, error) {
	s.lastContactTrackContactID = contactID
	return s.contactEmployeeTracks, nil
}

func (s *fakeWorkReadStore) ContactProcessesByCorpID(_ context.Context, corpID int) ([]ContactProcessStatus, error) {
	s.lastProcessCorpID = corpID
	return s.processStatuses, nil
}

func (s *fakeWorkReadStore) CreateDefaultContactProcesses(_ context.Context, corpID int) error {
	s.defaultProcessesCreated = true
	s.processStatuses = []ContactProcessStatus{
		{ID: 1, Name: "新客户"},
		{ID: 2, Name: "初步沟通"},
		{ID: 3, Name: "意向客户"},
		{ID: 4, Name: "付款客户"},
		{ID: 5, Name: "无意向客户"},
	}
	s.lastProcessCorpID = corpID
	return nil
}

func (s *fakeWorkReadStore) ContactProcessByID(_ context.Context, statusID int) (ContactProcessStatus, bool, error) {
	s.lastProcessStatusID = statusID
	return s.processStatus, s.processStatusFound, nil
}

func (s *fakeWorkReadStore) UpdateContactProcessStatus(_ context.Context, update ContactProcessStatusUpdate) error {
	s.lastProcessUpdate = update
	return nil
}

func (s *fakeWorkReadStore) UpdateWorkContactProfile(_ context.Context, values WorkContactUpdateValues) (WorkContactUpdateResult, bool, error) {
	s.lastWorkContactUpdate = values
	return s.workContactUpdateResult, s.workContactUpdateFound, nil
}

func (s *fakeWorkReadStore) BatchLabelWorkContacts(_ context.Context, contactIDs []int, tagIDs []int, employeeID int) (int, error) {
	s.lastBatchLabelContactIDs = append([]int{}, contactIDs...)
	s.lastBatchLabelTagIDs = append([]int{}, tagIDs...)
	s.lastBatchLabelEmployeeID = employeeID
	return s.batchLabelInserted, nil
}

func (s *fakeWorkReadStore) RoomWelcomeCorpCredentialByID(_ context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error) {
	s.lastCredentialCorpID = corpID
	return s.roomWelcomeCredential, s.roomWelcomeCredentialFound, nil
}

type fakeContactTagWriteClient struct {
	addRequests     []WorkContactTagAddRequest
	addResult       WorkContactTagSyncGroup
	updatedTagID    string
	updatedTagName  string
	deletedTagIDs   []string
	deletedGroupIDs []string
}

func (c *fakeContactTagWriteClient) AddCorpTags(_ context.Context, _ RoomWelcomeCorpCredential, request WorkContactTagAddRequest) (WorkContactTagSyncGroup, error) {
	c.addRequests = append(c.addRequests, request)
	return c.addResult, nil
}

func (c *fakeContactTagWriteClient) UpdateCorpTag(_ context.Context, _ RoomWelcomeCorpCredential, wxContactTagID string, name string) error {
	c.updatedTagID = wxContactTagID
	c.updatedTagName = name
	return nil
}

func (c *fakeContactTagWriteClient) DeleteCorpTags(_ context.Context, _ RoomWelcomeCorpCredential, wxContactTagIDs []string, wxGroupIDs []string) error {
	c.deletedTagIDs = append(c.deletedTagIDs, wxContactTagIDs...)
	c.deletedGroupIDs = append(c.deletedGroupIDs, wxGroupIDs...)
	return nil
}
