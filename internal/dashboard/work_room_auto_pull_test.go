package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestWorkRoomAutoPullIndexReturnsPHPCompatiblePage(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users: map[int]User{1: {ID: 1, Name: "管理员"}},
		page: WorkRoomAutoPullPage{
			PerPage:   10,
			Total:     1,
			TotalPage: 1,
			Items: []WorkRoomAutoPullItem{{
				WorkRoomAutoPullID: 900001,
				QRCodeName:         "Go迁移自动拉群",
				QRCodeURL:          "qrcode/auto-pull.png",
				LeadingWords:       "欢迎入群",
				Tags:               []string{"Go迁移标签"},
				Employees:          []string{"Go迁移员工"},
				Rooms:              []WorkRoomAutoPullListRoom{{RoomName: "Go迁移客户群", StateText: "拉人中"}},
				ContactNum:         1,
				CreatedAt:          "2026-07-03 10:00:00",
			}},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkRoomAutoPullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoomAutoPull/index?qrcodeName=Go&page=1&perPage=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workRoomAutoPull/index#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if len(store.filter.CorpIDs) != 1 || store.filter.CorpIDs[0] != 7 || store.filter.QRCodeName != "Go" || store.filter.Page != 1 || store.filter.PerPage != 10 {
		t.Fatalf("filter = %#v", store.filter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if page["perPage"] != "10" || int(page["total"].(float64)) != 1 || int(page["totalPage"].(float64)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	item := data["list"].([]any)[0].(map[string]any)
	if int(item["workRoomAutoPullId"].(float64)) != 900001 || item["qrcodeUrl"] != "http://api.example.com/static/qrcode/auto-pull.png" || item["leadingWords"] != "欢迎入群" {
		t.Fatalf("item = %#v", item)
	}
	if tag := item["tags"].([]any)[0]; tag != "Go迁移标签" {
		t.Fatalf("tags = %#v", item["tags"])
	}
	room := item["rooms"].([]any)[0].(map[string]any)
	if room["roomName"] != "Go迁移客户群" || room["stateText"] != "拉人中" {
		t.Fatalf("room = %#v", room)
	}
}

func TestWorkRoomAutoPullIndexAppliesDataPermission(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users:       map[int]User{1: {ID: 1}},
		businessIDs: []int{900001},
		page:        WorkRoomAutoPullPage{PerPage: 10, Items: []WorkRoomAutoPullItem{}},
	}
	authorizer := &recordingAuthorizer{
		accessSet: true,
		access: AccessContext{
			DataPermission:  DataPermissionDepartment,
			DeptEmployeeIDs: []int{99, 100},
		},
	}
	handler := NewWorkRoomAutoPullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoomAutoPull/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.businessOperators) != 2 || store.businessOperators[0] != 99 || store.businessOperators[1] != 100 {
		t.Fatalf("operators = %#v", store.businessOperators)
	}
	if !store.filter.RestrictBusinessIDs || len(store.filter.BusinessIDs) != 1 || store.filter.BusinessIDs[0] != 900001 {
		t.Fatalf("filter = %#v", store.filter)
	}
}

func TestWorkRoomAutoPullShowReturnsDetail(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users:     map[int]User{1: {ID: 1}},
		showFound: true,
		show: WorkRoomAutoPullShow{
			WorkRoomAutoPullID: 900001,
			QRCodeName:         "Go迁移自动拉群",
			QRCodeURL:          "qrcode/auto-pull.png",
			IsVerified:         2,
			RoomNum:            1,
			LeadingWords:       "欢迎入群",
			CreatedAt:          "2026-07-03 10:00:00",
			Employees: []WorkRoomAutoPullEmployee{{
				ID: 1, EmployeeID: 1, Name: "Go迁移员工", Avatar: "avatar.png", WXUserID: "go-user", Select: true,
			}},
			Tags: []ChannelCodeTagGroup{{
				GroupID:   0,
				GroupName: "未分组",
				List:      []ChannelCodeTagItem{{TagID: 900001, TagName: "Go迁移标签", IsSelected: 1}},
			}},
			SelectedTags: []int{900001},
			Rooms: []WorkRoomAutoPullShowRoom{{
				RoomID: 900001, RoomName: "Go迁移客户群", RoomMax: 200, Num: 1, MaxNum: 50, RoomQRCodeURL: "qrcode/room.png", State: 2,
			}},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkRoomAutoPullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoomAutoPull/show?workRoomAutoPullId=900001", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workRoomAutoPull/show#get" || store.showID != 900001 {
		t.Fatalf("authorizer = %#v showID=%d", authorizer, store.showID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["workRoomAutoPullId"].(float64)) != 900001 || data["qrcodeUrl"] != "http://api.example.com/static/qrcode/auto-pull.png" || int(data["roomNum"].(float64)) != 1 {
		t.Fatalf("data = %#v", data)
	}
	employee := data["employees"].([]any)[0].(map[string]any)
	if int(employee["employeeId"].(float64)) != 1 || employee["select"] != true {
		t.Fatalf("employee = %#v", employee)
	}
	room := data["rooms"].([]any)[0].(map[string]any)
	if room["longRoomQrcodeUrl"] != "http://api.example.com/static/qrcode/room.png" || int(room["state"].(float64)) != 2 {
		t.Fatalf("room = %#v", room)
	}
}

func TestWorkRoomAutoPullShowValidatesID(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkRoomAutoPullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	for _, tc := range []struct {
		query string
		msg   string
	}{
		{query: "", msg: "自动拉群ID 必填"},
		{query: "?workRoomAutoPullId=abc", msg: "自动拉群ID 必需为整数"},
		{query: "?workRoomAutoPullId=0", msg: "自动拉群ID 不可小于1"},
	} {
		req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workRoomAutoPull/show"+tc.query, nil)
		req.Header.Set("X-Mochat-Go-User-ID", "1")
		rec := httptest.NewRecorder()
		handler.Show(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body=%s", tc.query, rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec.Body.Bytes())
		if body["msg"] != tc.msg {
			t.Fatalf("%s msg = %#v", tc.query, body["msg"])
		}
	}
}

func TestWorkRoomAutoPullStoreCreatesRecordAndQRCode(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users:      map[int]User{1: {ID: 1}},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		createID:   910001,
	}
	client := &fakeWorkRoomAutoPullContactWayClient{
		qrcode: WorkRoomAutoPullQRCode{ConfigID: "config-910001", QRCodeURL: "https://wecom.example/qrcode.png"},
	}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{DataPermission: DataPermissionAll, WorkEmployeeID: 99}}
	handler := NewWorkRoomAutoPullHandlerWithContactWayClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "", client)

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workRoomAutoPull/store", strings.NewReader(`{"corpId":7,"qrcodeName":"新自动拉群","isVerified":2,"leadingWords":"欢迎","mediumId":45,"employees":"1","tags":"900001","rooms":"[{\"roomId\":900001,\"maxNum\":50}]"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workRoomAutoPull/store#post" {
		t.Fatalf("permission key = %q", authorizer.permissionKey)
	}
	if store.created.CorpID != 7 || store.created.QRCodeName != "新自动拉群" || store.created.MediumID != 45 || store.created.Employees != `["1"]` || store.created.Tags != `["900001"]` {
		t.Fatalf("created = %#v", store.created)
	}
	if store.createOperationID != 99 {
		t.Fatalf("operation id = %d", store.createOperationID)
	}
	if client.createState != "workRoomAutoPullId-910001" || client.createSkipVerify != true || len(client.createUsers) != 1 || client.createUsers[0] != "go-migrate-user" {
		t.Fatalf("client = %#v", client)
	}
	if store.qrcodeID != 910001 || store.qrcodeURL != "https://wecom.example/qrcode.png" || store.qrcodeConfigID != "config-910001" {
		t.Fatalf("qrcode update = id %d url %q config %q", store.qrcodeID, store.qrcodeURL, store.qrcodeConfigID)
	}
}

func TestWorkRoomAutoPullStoreUsesSelectedCorpInsteadOfClientCorp(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users:      map[int]User{1: {ID: 1}},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		createID:   910002,
	}
	client := &fakeWorkRoomAutoPullContactWayClient{qrcode: WorkRoomAutoPullQRCode{ConfigID: "config-910002", QRCodeURL: "https://wecom.example/qrcode.png"}}
	handler := NewWorkRoomAutoPullHandlerWithContactWayClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{accessSet: true, access: AccessContext{DataPermission: DataPermissionAll, WorkEmployeeID: 99}}, "", client)

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workRoomAutoPull/store", strings.NewReader(`{"corpId":999,"qrcodeName":"跨企业请求","isVerified":2,"leadingWords":"欢迎","employees":"1","tags":"900001","rooms":"[{\"roomId\":900001,\"maxNum\":50}]"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK || store.created.CorpID != 7 || store.credentialLookupCorpID != 7 {
		t.Fatalf("status=%d createdCorp=%d credentialCorp=%d body=%s", rec.Code, store.created.CorpID, store.credentialLookupCorpID, rec.Body.String())
	}
}

func TestWorkRoomAutoPullStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users: map[int]User{1: {ID: 1, TenantID: 8}},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricWorkRoomAutoPulls,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	client := &fakeWorkRoomAutoPullContactWayClient{
		qrcode: WorkRoomAutoPullQRCode{ConfigID: "config-910001", QRCodeURL: "https://wecom.example/qrcode.png"},
	}
	handler := NewWorkRoomAutoPullHandlerWithContactWayClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{accessSet: true, access: AccessContext{DataPermission: DataPermissionAll, WorkEmployeeID: 99}}, "", client)

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workRoomAutoPull/store", strings.NewReader(`{"corpId":7,"qrcodeName":"新自动拉群","isVerified":2,"leadingWords":"欢迎","employees":"1","tags":"900001","rooms":"[{\"roomId\":900001,\"maxNum\":50}]"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.quotaMetric != SaaSMetricWorkRoomAutoPulls || store.quotaTenantID != 8 || store.createCalls != 0 || store.refreshMetric != "" {
		t.Fatalf("quota metric=%q tenant=%d createCalls=%d refresh=%q", store.quotaMetric, store.quotaTenantID, store.createCalls, store.refreshMetric)
	}
	if client.createState != "" || len(client.createUsers) != 0 {
		t.Fatalf("client = %#v", client)
	}
}

func TestWorkRoomAutoPullUpdateUpdatesRecordAndQRCode(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users:        map[int]User{1: {ID: 1}},
		credential:   RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		updateFound:  true,
		updateTarget: WorkRoomAutoPullUpdateTarget{CorpID: 7, WXConfigID: "config-old"},
	}
	client := &fakeWorkRoomAutoPullContactWayClient{}
	handler := NewWorkRoomAutoPullHandlerWithContactWayClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{accessSet: true, access: AccessContext{WorkEmployeeID: 99}}, "", client)

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workRoomAutoPull/update", strings.NewReader(`{"workRoomAutoPullId":900001,"isVerified":1,"employees":"1","tags":"900001","rooms":"[{\"roomId\":900001,\"maxNum\":40}]"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.updateID != 900001 || store.updated.IsVerified != 1 || store.updateOperationID != 99 {
		t.Fatalf("updated = id %d values %#v op %d", store.updateID, store.updated, store.updateOperationID)
	}
	if client.updateConfigID != "config-old" || client.updateState != "workRoomAutoPullId-900001" || client.updateSkipVerify != false {
		t.Fatalf("client = %#v", client)
	}
}

func TestWorkRoomAutoPullMoveUsesUpdatePermissionAndNoops(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{users: map[int]User{1: {ID: 1}}}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{WorkEmployeeID: 99}}
	handler := NewWorkRoomAutoPullHandlerWithContactWayClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "", &fakeWorkRoomAutoPullContactWayClient{})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workRoomAutoPull/move", strings.NewReader(`{"workRoomAutoPullId":900001,"groupId":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Move(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workRoomAutoPull/update#put" {
		t.Fatalf("permission key = %q", authorizer.permissionKey)
	}
	if store.updateID != 0 {
		t.Fatalf("move should not update storage, updateID=%d", store.updateID)
	}
}

func TestWorkRoomAutoPullStoreValidatesRequiredName(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkRoomAutoPullHandlerWithContactWayClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", &fakeWorkRoomAutoPullContactWayClient{})

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workRoomAutoPull/store", strings.NewReader(`{"corpId":7,"isVerified":2,"leadingWords":"欢迎","employees":"1","tags":"900001","rooms":"[]"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "扫码名称 必填" {
		t.Fatalf("body = %#v", body)
	}
}

func TestWorkRoomAutoPullStoreCreatesDirectJoinQRCodeWithoutLegacyFields(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users: map[int]User{1: {ID: 1}}, credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"}, createID: 920001,
		roomChatIDs: []string{"chat-22", "chat-11"},
	}
	client := &fakeWorkRoomAutoPullJoinWayClient{qrcode: WorkRoomAutoPullQRCode{ConfigID: "join-920001", QRCodeURL: "https://wecom.example/join.png"}}
	handler := NewWorkRoomAutoPullHandlerWithJoinWayClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{accessSet: true, access: AccessContext{DataPermission: DataPermissionAll, WorkEmployeeID: 99}}, "", client)
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workRoomAutoPull/store", strings.NewReader(`{"corpId":7,"qrcodeName":"售后群活码","rooms":[22,11],"autoCreateRoom":true,"roomBaseName":"售后服务群","roomBaseId":8}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.roomLookupCorpID != 7 || !reflect.DeepEqual(store.roomLookupIDs, []int{22, 11}) {
		t.Fatalf("room lookup corp=%d ids=%#v", store.roomLookupCorpID, store.roomLookupIDs)
	}
	if !reflect.DeepEqual(client.payload.ChatIDs, []string{"chat-22", "chat-11"}) || !client.payload.AutoCreateRoom || client.payload.RoomBaseName != "售后服务群" || client.payload.RoomBaseID != 8 {
		t.Fatalf("payload=%#v", client.payload)
	}
	if store.created.ProviderKind != "join_way" || store.created.Employees != "[]" || store.created.Tags != "[]" || store.created.LeadingWords != "" {
		t.Fatalf("created=%#v", store.created)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["workRoomAutoPullId"].(float64)) != 920001 || data["qrcodeUrl"] != "https://wecom.example/join.png" {
		t.Fatalf("data=%#v", data)
	}
}

func TestWorkRoomAutoPullStoreDoesNotPersistWhenJoinWayCreationFails(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users: map[int]User{1: {ID: 1}}, credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		roomChatIDs: []string{"chat-22"},
	}
	client := &fakeWorkRoomAutoPullJoinWayClient{createErr: fmt.Errorf("provider unavailable")}
	handler := NewWorkRoomAutoPullHandlerWithJoinWayClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{accessSet: true, access: AccessContext{DataPermission: DataPermissionAll, WorkEmployeeID: 99}}, "", client)
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workRoomAutoPull/store", strings.NewReader(`{"qrcodeName":"售后群活码","rooms":[22],"autoCreateRoom":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest || store.createCalls != 0 {
		t.Fatalf("status=%d createCalls=%d body=%s", rec.Code, store.createCalls, rec.Body.String())
	}
}

func TestWorkRoomAutoPullStoreCleansRemoteJoinWayWhenDatabaseSaveFails(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users: map[int]User{1: {ID: 1}}, credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		roomChatIDs: []string{"chat-22"}, createErr: fmt.Errorf("database unavailable"),
	}
	client := &fakeWorkRoomAutoPullJoinWayClient{qrcode: WorkRoomAutoPullQRCode{ConfigID: "join-920002", QRCodeURL: "https://wecom.example/join.png"}}
	handler := NewWorkRoomAutoPullHandlerWithJoinWayClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{accessSet: true, access: AccessContext{DataPermission: DataPermissionAll, WorkEmployeeID: 99}}, "", client)
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/workRoomAutoPull/store", strings.NewReader(`{"qrcodeName":"售后群活码","rooms":[22],"autoCreateRoom":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusInternalServerError || client.deletedConfigID != "join-920002" {
		t.Fatalf("status=%d deleted=%q body=%s", rec.Code, client.deletedConfigID, rec.Body.String())
	}
	if body := decodeBody(t, rec.Body.Bytes()); body["msg"] != "群活码保存失败，已清理企业微信配置" {
		t.Fatalf("body=%#v", body)
	}
}

func TestWorkRoomAutoPullUpdateRejectsDirectJoinWayBeforeMutation(t *testing.T) {
	store := &fakeWorkRoomAutoPullStore{
		users: map[int]User{1: {ID: 1}}, updateFound: true,
		updateTarget: WorkRoomAutoPullUpdateTarget{CorpID: 7, WXConfigID: "join-920001", ProviderKind: "join_way"},
	}
	client := &fakeWorkRoomAutoPullContactWayClient{}
	handler := NewWorkRoomAutoPullHandlerWithContactWayClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{accessSet: true, access: AccessContext{WorkEmployeeID: 99}}, "", client)
	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workRoomAutoPull/update", strings.NewReader(`{"workRoomAutoPullId":920001,"isVerified":1,"employees":"1","tags":"900001","rooms":"[{\"roomId\":900001,\"maxNum\":40}]"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Update(rec, req)

	if rec.Code != http.StatusConflict || store.updateID != 0 || client.updateConfigID != "" {
		t.Fatalf("status=%d updateID=%d providerConfig=%q body=%s", rec.Code, store.updateID, client.updateConfigID, rec.Body.String())
	}
}

type fakeWorkRoomAutoPullStore struct {
	users                  map[int]User
	credential             RoomWelcomeCorpCredential
	page                   WorkRoomAutoPullPage
	filter                 WorkRoomAutoPullFilter
	businessOperators      []int
	businessIDs            []int
	show                   WorkRoomAutoPullShow
	showFound              bool
	showID                 int
	createID               int
	createCalls            int
	createErr              error
	created                WorkRoomAutoPullWrite
	createOperationID      int
	updated                WorkRoomAutoPullWrite
	updateID               int
	updateOperationID      int
	updateTarget           WorkRoomAutoPullUpdateTarget
	updateFound            bool
	qrcodeID               int
	qrcodeURL              string
	qrcodeConfigID         string
	deletedID              int
	quota                  SaaSQuotaStatus
	credentialLookupCorpID int
	quotaTenantID          int
	quotaMetric            string
	refreshTenantID        int
	refreshMetric          string
	roomChatIDs            []string
	roomLookupCorpID       int
	roomLookupIDs          []int
}

func (s *fakeWorkRoomAutoPullStore) WorkRoomAutoPullRoomWXChatIDs(_ context.Context, corpID int, roomIDs []int) ([]string, error) {
	s.roomLookupCorpID = corpID
	s.roomLookupIDs = append([]int{}, roomIDs...)
	return append([]string{}, s.roomChatIDs...), nil
}

func (s *fakeWorkRoomAutoPullStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeWorkRoomAutoPullStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 99, nil
}

func (s *fakeWorkRoomAutoPullStore) MediumAvailableToUser(context.Context, int, int, int) (bool, error) {
	return true, nil
}

func (s *fakeWorkRoomAutoPullStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeWorkRoomAutoPullStore) RoomWelcomeCorpCredentialByID(_ context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error) {
	s.credentialLookupCorpID = corpID
	if s.credential.CorpID == 0 {
		return RoomWelcomeCorpCredential{}, false, nil
	}
	return s.credential, s.credential.CorpID == corpID, nil
}

func (s *fakeWorkRoomAutoPullStore) WorkRoomAutoPullPage(_ context.Context, filter WorkRoomAutoPullFilter) (WorkRoomAutoPullPage, error) {
	s.filter = filter
	return s.page, nil
}

func (s *fakeWorkRoomAutoPullStore) WorkRoomAutoPullBusinessIDsByOperators(_ context.Context, operationIDs []int) ([]int, error) {
	s.businessOperators = append([]int{}, operationIDs...)
	return append([]int{}, s.businessIDs...), nil
}

func (s *fakeWorkRoomAutoPullStore) WorkRoomAutoPullShowByID(_ context.Context, id int) (WorkRoomAutoPullShow, bool, error) {
	s.showID = id
	return s.show, s.showFound, nil
}

func (s *fakeWorkRoomAutoPullStore) WorkRoomAutoPullEmployeeWXUserIDs(_ context.Context, employeeIDs []int) ([]string, error) {
	if len(employeeIDs) == 0 {
		return []string{}, nil
	}
	return []string{"go-migrate-user"}, nil
}

func (s *fakeWorkRoomAutoPullStore) CreateWorkRoomAutoPullWithLog(_ context.Context, values WorkRoomAutoPullWrite, operationID int) (int, error) {
	s.createCalls++
	s.created = values
	s.createOperationID = operationID
	if s.createID == 0 {
		s.createID = 900001
	}
	return s.createID, s.createErr
}

func (s *fakeWorkRoomAutoPullStore) WorkRoomAutoPullUpdateTargetByID(_ context.Context, _ int) (WorkRoomAutoPullUpdateTarget, bool, error) {
	return s.updateTarget, s.updateFound, nil
}

func (s *fakeWorkRoomAutoPullStore) UpdateWorkRoomAutoPullWithLog(_ context.Context, id int, values WorkRoomAutoPullWrite, operationID int) (WorkRoomAutoPullUpdateTarget, bool, error) {
	s.updateID = id
	s.updated = values
	s.updateOperationID = operationID
	return s.updateTarget, s.updateFound, nil
}

func (s *fakeWorkRoomAutoPullStore) UpdateWorkRoomAutoPullQRCode(_ context.Context, id int, qrcodeURL string, configID string) error {
	s.qrcodeID = id
	s.qrcodeURL = qrcodeURL
	s.qrcodeConfigID = configID
	return nil
}

func (s *fakeWorkRoomAutoPullStore) DeleteWorkRoomAutoPull(_ context.Context, id int) error {
	s.deletedID = id
	return nil
}

func (s *fakeWorkRoomAutoPullStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	status := s.quota
	status.TenantID = tenantID
	status.Metric = metric
	status.Additional = additional
	return status, nil
}

func (s *fakeWorkRoomAutoPullStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}

type fakeWorkRoomAutoPullContactWayClient struct {
	qrcode           WorkRoomAutoPullQRCode
	createUsers      []string
	createSkipVerify bool
	createState      string
	updateConfigID   string
	updateUsers      []string
	updateSkipVerify bool
	updateState      string
}

func (c *fakeWorkRoomAutoPullContactWayClient) CreateContactWay(_ context.Context, _ RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, state string) (WorkRoomAutoPullQRCode, error) {
	c.createUsers = append([]string{}, userIDs...)
	c.createSkipVerify = skipVerify
	c.createState = state
	return c.qrcode, nil
}

func (c *fakeWorkRoomAutoPullContactWayClient) UpdateContactWay(_ context.Context, _ RoomWelcomeCorpCredential, configID string, userIDs []string, skipVerify bool, state string) error {
	c.updateConfigID = configID
	c.updateUsers = append([]string{}, userIDs...)
	c.updateSkipVerify = skipVerify
	c.updateState = state
	return nil
}

type fakeWorkRoomAutoPullJoinWayClient struct {
	qrcode          WorkRoomAutoPullQRCode
	payload         WorkRoomAutoPullJoinWayPayload
	deletedConfigID string
	createErr       error
	deleteErr       error
}

func (c *fakeWorkRoomAutoPullJoinWayClient) CreateJoinWay(_ context.Context, _ RoomWelcomeCorpCredential, payload WorkRoomAutoPullJoinWayPayload) (WorkRoomAutoPullQRCode, error) {
	c.payload = payload
	return c.qrcode, c.createErr
}

func (c *fakeWorkRoomAutoPullJoinWayClient) UpdateJoinWay(_ context.Context, _ RoomWelcomeCorpCredential, _ string, payload WorkRoomAutoPullJoinWayPayload) error {
	c.payload = payload
	return nil
}

func (c *fakeWorkRoomAutoPullJoinWayClient) DeleteJoinWay(_ context.Context, _ RoomWelcomeCorpCredential, configID string) error {
	c.deletedConfigID = configID
	return c.deleteErr
}
