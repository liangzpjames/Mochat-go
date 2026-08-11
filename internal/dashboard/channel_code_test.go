package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChannelCodeGroupIndexReturnsPHPCompatibleGroups(t *testing.T) {
	store := &fakeChannelCodeStore{
		users: map[int]User{1: {ID: 1}},
		groups: []ChannelCodeGroup{
			{ID: 900001, Name: "Go迁移渠道分组"},
		},
	}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/channelCodeGroup/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.GroupIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.lastCorpIDs) != 1 || store.lastCorpIDs[0] != 7 {
		t.Fatalf("corp ids = %#v", store.lastCorpIDs)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	first := data[0].(map[string]any)
	second := data[1].(map[string]any)
	if int(first["groupId"].(float64)) != 900001 || first["name"] != "Go迁移渠道分组" {
		t.Fatalf("first = %#v", first)
	}
	if int(second["groupId"].(float64)) != 0 || second["name"] != "未分组" {
		t.Fatalf("second = %#v", second)
	}
}

func TestChannelCodeGroupDetailReturnsPHPCompatibleDetail(t *testing.T) {
	store := &fakeChannelCodeStore{
		users: map[int]User{1: {ID: 1}},
		group: ChannelCodeGroup{ID: 900001, Name: "Go迁移渠道分组"},
		found: true,
	}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/channelCodeGroup/detail?groupId=900001", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.GroupDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastGroupID != 900001 {
		t.Fatalf("group id = %d", store.lastGroupID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["id"].(float64)) != 900001 || int(data["groupId"].(float64)) != 900001 || data["name"] != "Go迁移渠道分组" {
		t.Fatalf("data = %#v", data)
	}
}

func TestChannelCodeGroupDetailRequiresGroupID(t *testing.T) {
	store := &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/channelCodeGroup/detail", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.GroupDetail(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "分组id必传" {
		t.Fatalf("body = %#v", body)
	}
}

func TestChannelCodeGroupStoreCreatesGroups(t *testing.T) {
	store := &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/channelCodeGroup/store", strings.NewReader(`{"name":["渠道A","渠道B"]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.GroupStore(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastNameExists[0] != "渠道A" || store.lastNameExists[1] != "渠道B" {
		t.Fatalf("name check = %#v", store.lastNameExists)
	}
	if store.createdCorpID != 7 || len(store.createdNames) != 2 || store.createdNames[0] != "渠道A" || store.createdNames[1] != "渠道B" {
		t.Fatalf("created = corp %d names %#v", store.createdCorpID, store.createdNames)
	}
}

func TestChannelCodeGroupStoreRequiresDashboardPrincipal(t *testing.T) {
	store := &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99,8-100"), HeaderUserIDResolver{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/channelCodeGroup/store", strings.NewReader(`{"name":["test"]}`))
	rec := httptest.NewRecorder()
	handler.GroupStore(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestChannelCodeGroupStoreRejectsDuplicateName(t *testing.T) {
	store := &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}, nameExists: true}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/channelCodeGroup/store", strings.NewReader(`{"name":["渠道A"]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.GroupStore(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "已存在相同分组名" {
		t.Fatalf("body = %#v", body)
	}
}

func TestChannelCodeGroupUpdateRenamesGroup(t *testing.T) {
	store := &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/channelCodeGroup/update", strings.NewReader(`{"groupId":900001,"name":"新渠道分组"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.GroupUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.updatedGroupID != 900001 || store.updatedName != "新渠道分组" {
		t.Fatalf("updated = id %d name %q", store.updatedGroupID, store.updatedName)
	}
}

func TestChannelCodeGroupUpdateRejectsUngrouped(t *testing.T) {
	store := &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/channelCodeGroup/update", strings.NewReader(`{"groupId":0,"name":"未分组"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.GroupUpdate(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "【未分组】不能修改" {
		t.Fatalf("body = %#v", body)
	}
}

func TestChannelCodeGroupMoveUpdatesChannelCodeGroup(t *testing.T) {
	store := &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/channelCodeGroup/move", strings.NewReader(`{"channelCodeId":900003,"groupId":900001}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.GroupMove(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.movedChannelCodeID != 900003 || store.movedGroupID != 900001 {
		t.Fatalf("moved = channel %d group %d", store.movedChannelCodeID, store.movedGroupID)
	}
}

func TestChannelCodeGroupMoveRequiresGroupID(t *testing.T) {
	store := &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/channelCodeGroup/move", strings.NewReader(`{"channelCodeId":900003}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.GroupMove(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "分组id必传" {
		t.Fatalf("body = %#v", body)
	}
}

func TestChannelCodeStoreCreatesCodeAndContactWay(t *testing.T) {
	store := &fakeChannelCodeStore{
		users:                map[int]User{1: {ID: 1, TenantID: 8}},
		credential:           RoomWelcomeCorpCredential{WXCorpID: "wx-corp", ContactSecret: "contact-secret"},
		createdChannelCodeID: 900004,
		employeeWXUserIDs:    []string{"go-migrate-user"},
	}
	wecom := &fakeChannelCodeWeComClient{qrCodeURL: "https://wecom.example/channel-code.png", configID: "config-created"}
	handler := NewChannelCodeHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", wecom)

	body := `{
		"baseInfo":{"groupId":900001,"name":"Go迁移渠道码新增","autoAddFriend":1,"tags":[900001]},
		"drainageEmployee":{"type":1,"employees":[],"specialPeriod":{"status":1,"detail":[{"startDate":"2026-01-01","endDate":"2026-12-31","timeSlot":[{"startTime":"00:00","endTime":"00:00","employeeId":[21]}]}]},"addMax":{"status":2,"employees":[],"spareEmployeeIds":[]}},
		"welcomeMessage":{"scanCodePush":2,"messageDetail":[]}
	}`
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/channelCode/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.createdChannelCode.GroupID != 900001 || store.createdChannelCode.Name != "Go迁移渠道码新增" || store.createdChannelCode.OperationEmployee != 99 {
		t.Fatalf("created = %#v", store.createdChannelCode)
	}
	if wecom.createState != "channelCode-900004" || !wecom.createSkipVerify || len(wecom.createUsers) != 1 || wecom.createUsers[0] != "go-migrate-user" {
		t.Fatalf("wecom create = %#v", wecom)
	}
	if store.qrCodeID != 900004 || store.qrCodeURL != "https://wecom.example/channel-code.png" || store.qrConfigID != "config-created" {
		t.Fatalf("qr update = id %d url %q config %q", store.qrCodeID, store.qrCodeURL, store.qrConfigID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricChannelCodes {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestChannelCodeStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeChannelCodeStore{
		users: map[int]User{1: {ID: 1, TenantID: 8}},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricChannelCodes,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	wecom := &fakeChannelCodeWeComClient{}
	handler := NewChannelCodeHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", wecom)

	body := `{
		"baseInfo":{"groupId":900001,"name":"额度外渠道码","autoAddFriend":1,"tags":[900001]},
		"drainageEmployee":{"type":1,"employees":[],"specialPeriod":{"status":1,"detail":[{"startDate":"2026-01-01","endDate":"2026-12-31","timeSlot":[{"startTime":"00:00","endTime":"00:00","employeeId":[21]}]}]},"addMax":{"status":2,"employees":[],"spareEmployeeIds":[]}},
		"welcomeMessage":{"scanCodePush":2,"messageDetail":[]}
	}`
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/channelCode/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	response := decodeBody(t, rec.Body.Bytes())
	if response["msg"] != "套餐额度已达上限：渠道活码数 1/1" {
		t.Fatalf("response = %#v", response)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricChannelCodes || store.quotaAdditional != 1 {
		t.Fatalf("quota = tenant %d metric %q additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.createCalls != 0 || store.refreshMetric != "" || wecom.createState != "" {
		t.Fatalf("side effects = createCalls %d refresh %q wecom %q", store.createCalls, store.refreshMetric, wecom.createState)
	}
}

func TestChannelCodeUpdatePersistsAndUpdatesContactWay(t *testing.T) {
	store := &fakeChannelCodeStore{
		users:             map[int]User{1: {ID: 1}},
		credential:        RoomWelcomeCorpCredential{WXCorpID: "wx-corp", ContactSecret: "contact-secret"},
		updateWXConfigID:  "config-old",
		employeeWXUserIDs: []string{"go-migrate-user"},
	}
	wecom := &fakeChannelCodeWeComClient{}
	handler := NewChannelCodeHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", wecom)

	body := `{
		"channelCodeId":900003,
		"baseInfo":{"groupId":900001,"name":"Go迁移渠道码编辑","autoAddFriend":2,"tags":[900001]},
		"drainageEmployee":{"type":1,"employees":[],"specialPeriod":{"status":1,"detail":[{"startDate":"2026-01-01","endDate":"2026-12-31","timeSlot":[{"startTime":"00:00","endTime":"00:00","employeeId":[21]}]}]},"addMax":{"status":2,"employees":[],"spareEmployeeIds":[]}},
		"welcomeMessage":{"scanCodePush":2,"messageDetail":[]}
	}`
	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/channelCode/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.updatedChannelCodeID != 900003 || store.updatedChannelCode.Name != "Go迁移渠道码编辑" || store.updatedChannelCode.AutoAddFriend != 2 {
		t.Fatalf("updated = id %d values %#v", store.updatedChannelCodeID, store.updatedChannelCode)
	}
	if wecom.updateConfigID != "config-old" || wecom.updateState != "channelCode-900003" || wecom.updateSkipVerify {
		t.Fatalf("wecom update = %#v", wecom)
	}
}

func TestChannelCodeIndexReturnsPageAndAppliesDataPermission(t *testing.T) {
	store := &fakeChannelCodeStore{
		users:       map[int]User{1: {ID: 1, Name: "管理员", TenantID: 3}},
		businessIDs: []int{900003},
		channelPage: ChannelCodeListPage{
			PerPage:   5,
			Total:     1,
			TotalPage: 1,
			Items: []ChannelCodeListItem{{
				ID:            900003,
				GroupID:       900001,
				GroupName:     "Go迁移渠道分组",
				Name:          "Go迁移渠道码",
				QRCodeURL:     "qrcode/go.png",
				AutoAddFriend: 1,
				Tags:          []string{"重点客户"},
				Type:          1,
				ContactNum:    2,
			}},
		},
	}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{RoleID: 8, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{99, 100}}}
	handler := NewChannelCodeHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/channelCode/index?name=Go&type=1&groupId=900001&page=1&perPage=5", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/channelCode/index#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if len(store.businessOperators) != 2 || store.businessOperators[0] != 99 || store.businessOperators[1] != 100 {
		t.Fatalf("business operators = %#v", store.businessOperators)
	}
	if !store.channelFilter.RestrictBusinessIDs || len(store.channelFilter.BusinessIDs) != 1 || store.channelFilter.BusinessIDs[0] != 900003 {
		t.Fatalf("filter business ids = %#v", store.channelFilter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if int(page["perPage"].(float64)) != 5 || int(page["total"].(float64)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	item := data["list"].([]any)[0].(map[string]any)
	if int(item["channelCodeId"].(float64)) != 900003 || item["qrcodeUrl"] != "http://api.example.com/static/qrcode/go.png" || item["autoAddFriend"] != "开启" || item["type"] != "单人" {
		t.Fatalf("item = %#v", item)
	}
}

func TestChannelCodeShowReturnsBaseDrainageAndWelcome(t *testing.T) {
	store := &fakeChannelCodeStore{
		users:     map[int]User{1: {ID: 1}},
		showFound: true,
		show: ChannelCodeShow{
			GroupID:       900001,
			GroupName:     "Go迁移渠道分组",
			Name:          "Go迁移渠道码",
			AutoAddFriend: 1,
			TagGroups: []ChannelCodeTagGroup{{
				GroupID:   0,
				GroupName: "未分组",
				List:      []ChannelCodeTagItem{{TagID: 900001, TagName: "重点客户", IsSelected: 1}},
			}},
			SelectedTags:     []int{900001},
			DrainageEmployee: map[string]any{"employees": []any{}},
			WelcomeMessage:   map[string]any{"messageDetail": []any{}},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewChannelCodeHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/channelCode/show?channelCodeId=900003", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.showID != 900003 || store.showCorpID != 7 {
		t.Fatalf("show lookup = id %d corp %d", store.showID, store.showCorpID)
	}
	if authorizer.permissionKey != "/dashboard/channelCode/show#get" {
		t.Fatalf("permission key = %q", authorizer.permissionKey)
	}
	body := decodeBody(t, rec.Body.Bytes())
	base := body["data"].(map[string]any)["baseInfo"].(map[string]any)
	if base["name"] != "Go迁移渠道码" || int(base["groupId"].(float64)) != 900001 || len(base["selectedTags"].([]any)) != 1 {
		t.Fatalf("base = %#v", base)
	}
}

func TestChannelCodeContactReturnsPHPCompatiblePage(t *testing.T) {
	store := &fakeChannelCodeStore{
		users: map[int]User{1: {ID: 1}},
		contactPage: ChannelCodeContactPage{
			PerPage:   15,
			Total:     1,
			TotalPage: 1,
			Items: []ChannelCodeContactItem{{
				ContactID:  900001,
				EmployeeID: 99,
				CreateTime: "2026-07-03 10:00:00",
				Name:       "客户A",
				Employees:  "极义科技 张三",
			}},
		},
	}
	handler := NewChannelCodeHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/channelCode/contact?channelCodeId=900003", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Contact(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.contactFilter.ChannelCodeID != 900003 || store.contactFilter.PerPage != 15 {
		t.Fatalf("contact filter = %#v", store.contactFilter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	item := body["data"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if item["name"] != "客户A" || item["employees"] != "极义科技 张三" {
		t.Fatalf("item = %#v", item)
	}
}

func TestChannelCodeStatisticsReturnsRanges(t *testing.T) {
	store := &fakeChannelCodeStore{
		users: map[int]User{1: {ID: 1}},
		statContacts: []ChannelCodeStatContact{
			{Status: 1, CreateAt: "2026-07-01 09:00:00"},
			{Status: 3, CreateAt: "2026-07-02 09:00:00", DeletedAt: "2026-07-03 09:00:00"},
		},
	}
	handler := NewChannelCodeHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/channelCode/statistics?channelCodeId=900003&type=1&startTime=2026-07-01&endTime=2026-07-03", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Statistics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.statID != 900003 {
		t.Fatalf("stat id = %d", store.statID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["addNumLong"].(float64)) != 2 || int(data["defriendNumLong"].(float64)) != 1 || int(data["netNumLong"].(float64)) != 1 {
		t.Fatalf("data = %#v", data)
	}
	rows := data["list"].([]any)
	last := rows[2].(map[string]any)
	if last["time"] != "2026-07-03" || int(last["defriendNumRange"].(float64)) != 1 {
		t.Fatalf("last = %#v", last)
	}
}

func TestChannelCodeStatisticsIndexPaginatesRows(t *testing.T) {
	store := &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}}
	handler := NewChannelCodeHandlerWithAuthorizer(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/channelCode/statisticsIndex?channelCodeId=900003&type=1&startTime=2026-07-01&endTime=2026-07-03&page=2&perPage=2", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.StatisticsIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if page["perPage"] != "2" || int(page["total"].(float64)) != 3 || int(page["totalPage"].(float64)) != 2 {
		t.Fatalf("page = %#v", page)
	}
	rows := data["list"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["time"] != "2026-07-03" {
		t.Fatalf("rows = %#v", rows)
	}
}

type fakeChannelCodeStore struct {
	users                map[int]User
	groups               []ChannelCodeGroup
	group                ChannelCodeGroup
	found                bool
	nameExists           bool
	lastCorpIDs          []int
	lastGroupID          int
	lastNameExists       []string
	createdCorpID        int
	createdNames         []string
	updatedGroupID       int
	updatedName          string
	movedChannelCodeID   int
	movedGroupID         int
	credential           RoomWelcomeCorpCredential
	createdChannelCodeID int
	createdChannelCode   ChannelCodeWriteValues
	createCalls          int
	updatedChannelCodeID int
	updatedChannelCode   ChannelCodeWriteValues
	updateWXConfigID     string
	qrCodeID             int
	qrCodeURL            string
	qrConfigID           string
	deletedChannelCodeID int
	employeeWXUserIDs    []string
	contactCounts        map[int]int
	channelPage          ChannelCodeListPage
	channelFilter        ChannelCodeListFilter
	businessOperators    []int
	businessIDs          []int
	show                 ChannelCodeShow
	showFound            bool
	showID               int
	showCorpID           int
	contactPage          ChannelCodeContactPage
	contactFilter        ChannelCodeContactFilter
	statContacts         []ChannelCodeStatContact
	statID               int
	quota                SaaSQuotaStatus
	quotaTenantID        int
	quotaMetric          string
	quotaAdditional      int64
	refreshTenantID      int
	refreshMetric        string
}

func (s *fakeChannelCodeStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeChannelCodeStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 0, nil
}

func (s *fakeChannelCodeStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 0, 0, false, nil
}

func (s *fakeChannelCodeStore) ChannelCodeGroupsByCorpIDs(_ context.Context, corpIDs []int) ([]ChannelCodeGroup, error) {
	s.lastCorpIDs = append([]int{}, corpIDs...)
	return s.groups, nil
}

func (s *fakeChannelCodeStore) ChannelCodeGroupByID(_ context.Context, groupID int) (ChannelCodeGroup, bool, error) {
	s.lastGroupID = groupID
	return s.group, s.found, nil
}

func (s *fakeChannelCodeStore) ChannelCodeGroupNamesExist(_ context.Context, names []string) (bool, error) {
	s.lastNameExists = append([]string{}, names...)
	return s.nameExists, nil
}

func (s *fakeChannelCodeStore) CreateChannelCodeGroups(_ context.Context, corpID int, names []string) error {
	s.createdCorpID = corpID
	s.createdNames = append([]string{}, names...)
	return nil
}

func (s *fakeChannelCodeStore) UpdateChannelCodeGroupName(_ context.Context, groupID int, name string) error {
	s.updatedGroupID = groupID
	s.updatedName = name
	return nil
}

func (s *fakeChannelCodeStore) MoveChannelCodeToGroup(_ context.Context, channelCodeID int, groupID int) error {
	s.movedChannelCodeID = channelCodeID
	s.movedGroupID = groupID
	return nil
}

func (s *fakeChannelCodeStore) RoomWelcomeCorpCredentialByID(_ context.Context, _ int) (RoomWelcomeCorpCredential, bool, error) {
	if s.credential.WXCorpID == "" {
		return RoomWelcomeCorpCredential{}, false, nil
	}
	return s.credential, true, nil
}

func (s *fakeChannelCodeStore) CreateChannelCode(_ context.Context, values ChannelCodeWriteValues) (int, error) {
	s.createCalls++
	s.createdChannelCode = values
	if s.createdChannelCodeID == 0 {
		s.createdChannelCodeID = 900004
	}
	return s.createdChannelCodeID, nil
}

func (s *fakeChannelCodeStore) UpdateChannelCode(_ context.Context, channelCodeID int, values ChannelCodeWriteValues) (string, error) {
	s.updatedChannelCodeID = channelCodeID
	s.updatedChannelCode = values
	return s.updateWXConfigID, nil
}

func (s *fakeChannelCodeStore) UpdateChannelCodeQRCode(_ context.Context, channelCodeID int, qrCodeURL string, wxConfigID string) error {
	s.qrCodeID = channelCodeID
	s.qrCodeURL = qrCodeURL
	s.qrConfigID = wxConfigID
	return nil
}

func (s *fakeChannelCodeStore) DeleteChannelCode(_ context.Context, channelCodeID int) error {
	s.deletedChannelCodeID = channelCodeID
	return nil
}

func (s *fakeChannelCodeStore) ChannelCodeEmployeeWXUserIDs(_ context.Context, _ []int) ([]string, error) {
	return append([]string{}, s.employeeWXUserIDs...), nil
}

func (s *fakeChannelCodeStore) ChannelCodeContactCountsByEmployee(_ context.Context, _ []int) (map[int]int, error) {
	result := map[int]int{}
	for key, value := range s.contactCounts {
		result[key] = value
	}
	return result, nil
}

func (s *fakeChannelCodeStore) ChannelCodePage(_ context.Context, filter ChannelCodeListFilter) (ChannelCodeListPage, error) {
	s.channelFilter = filter
	return s.channelPage, nil
}

func (s *fakeChannelCodeStore) ChannelCodeBusinessIDsByOperators(_ context.Context, operationIDs []int) ([]int, error) {
	s.businessOperators = append([]int{}, operationIDs...)
	return append([]int{}, s.businessIDs...), nil
}

func (s *fakeChannelCodeStore) ChannelCodeShowByID(_ context.Context, channelCodeID int, corpID int) (ChannelCodeShow, bool, error) {
	s.showID = channelCodeID
	s.showCorpID = corpID
	return s.show, s.showFound, nil
}

func (s *fakeChannelCodeStore) ChannelCodeContactPage(_ context.Context, filter ChannelCodeContactFilter) (ChannelCodeContactPage, error) {
	s.contactFilter = filter
	return s.contactPage, nil
}

func (s *fakeChannelCodeStore) ChannelCodeStatContacts(_ context.Context, channelCodeID int) ([]ChannelCodeStatContact, error) {
	s.statID = channelCodeID
	return append([]ChannelCodeStatContact{}, s.statContacts...), nil
}

func (s *fakeChannelCodeStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	status := s.quota
	if status.Metric == "" {
		status.Metric = metric
	}
	if status.TenantID == 0 {
		status.TenantID = tenantID
	}
	if status.Additional == 0 {
		status.Additional = additional
	}
	return status, nil
}

func (s *fakeChannelCodeStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}

type fakeChannelCodeWeComClient struct {
	qrCodeURL        string
	configID         string
	createUsers      []string
	createSkipVerify bool
	createState      string
	updateConfigID   string
	updateUsers      []string
	updateSkipVerify bool
	updateState      string
}

func (c *fakeChannelCodeWeComClient) CreateContactWay(_ context.Context, _ RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, state string) (string, string, error) {
	c.createUsers = append([]string{}, userIDs...)
	c.createSkipVerify = skipVerify
	c.createState = state
	return c.qrCodeURL, c.configID, nil
}

func (c *fakeChannelCodeWeComClient) UpdateContactWay(_ context.Context, _ RoomWelcomeCorpCredential, configID string, userIDs []string, skipVerify bool, state string) error {
	c.updateConfigID = configID
	c.updateUsers = append([]string{}, userIDs...)
	c.updateSkipVerify = skipVerify
	c.updateState = state
	return nil
}
