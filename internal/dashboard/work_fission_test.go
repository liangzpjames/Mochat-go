package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkFissionIndexReturnsCompatibleList(t *testing.T) {
	store := &fakeWorkFissionStore{
		users: map[int]User{1: {ID: 1, IsSuperAdmin: 0}},
		page: WorkFissionListPage{
			Items: []WorkFissionListItem{{
				ID:               9,
				ActiveName:       "裂变活动",
				ServiceEmployees: `[{"id":1,"name":"张三"}]`,
				ContactTags:      `[{"id":2,"name":"标签"}]`,
				Tasks:            `[{"count":3}]`,
				EndTime:          time.Now().Add(time.Hour).Format("2006-01-02 15:04:05"),
				CreatedAt:        "2026-07-03 10:00:00",
				EmployeeNum:      4,
			}},
			Total:     1,
			TotalPage: 1,
			PerPage:   20,
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkFissionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "http://api.example.com", "http://op.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/workFission/index?active_name=裂变&page=2&perPage=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workFission/index#get" {
		t.Fatalf("permission=%s", authorizer.permissionKey)
	}
	if store.listFilter.CorpID != 7 || !store.listFilter.RestrictCreateUser || store.listFilter.Page != 2 || store.listFilter.PerPage != 20 {
		t.Fatalf("filter=%+v", store.listFilter)
	}
	item := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if item["status"] != "进行中" || item["finance_tag"] != "1/2" || item["employeeNum"].(float64) != 4 {
		t.Fatalf("item=%#v", item)
	}
}

func TestWorkFissionShowAndInfoReturnPHPFieldNames(t *testing.T) {
	store := &fakeWorkFissionStore{
		users: map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		bundle: WorkFissionBundle{
			Fission: WorkFissionInfo{
				ID:                    9,
				ActiveName:            "活动",
				ServiceEmployees:      `[{"id":1,"name":"张三"}]`,
				AutoPass:              1,
				AutoAddTag:            0,
				ContactTags:           `[{"id":2}]`,
				EndTime:               "2026-07-10 10:00:00",
				QRCodeInvalid:         7,
				Tasks:                 `[{"count":3}]`,
				NewFriend:             1,
				DeleteInvalid:         0,
				ReceivePrize:          1,
				ReceivePrizeEmployees: `[{"id":3}]`,
				ReceiveLinks:          `[{"url":"https://example.com"}]`,
				CreatedAt:             "2026-07-03 10:00:00",
			},
			Poster: WorkFissionPoster{
				PosterType:   1,
				CoverPic:     "poster/bg.png",
				WXCoverPic:   "wx-bg",
				FowardText:   "转发",
				AvatarShow:   1,
				NicknameShow: 0,
				QRCodeURL:    "https://qr.example.com/a.png",
			},
			Welcome: WorkFissionWelcome{MsgText: "欢迎", LinkTitle: "标题", LinkDesc: "描述", LinkCoverURL: "welcome/cover.png"},
			Push:    WorkFissionPush{PushEmployee: 1, PushContact: 0, MsgText: "推送", MsgComplex: `{"image":"push/a.png","title":"图"}`, MsgComplexType: "image"},
			Invite:  WorkFissionInvite{Text: "邀请", LinkTitle: "邀标题", LinkDesc: "邀描述", LinkPic: "invite/pic.png"},
		},
		bundleFound: true,
	}
	handler := NewWorkFissionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "http://op.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/workFission/show?id=9", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("show status=%d body=%s", rec.Code, rec.Body.String())
	}
	show := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	if show["link"] != "http://op.example.com/auth/workFission?id=9&target=%2FworkFission%3Fid%3D9" || show["welcome_url"] != "welcome/cover.png" {
		t.Fatalf("show=%#v", show)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/workFission/info?id=9", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.Info(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("info status=%d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	fission := data["fission"].(map[string]any)
	poster := data["poster"].(map[string]any)
	push := data["push"].(map[string]any)
	invite := data["invite"].(map[string]any)
	if fission["auto_pass"] != "true" || fission["auto_add_tag"] != "false" || poster["cover_pic"] != "http://api.example.com/static/poster/bg.png" {
		t.Fatalf("data=%#v", data)
	}
	if push["msg_complex"].(map[string]any)["image"] != "http://api.example.com/static/push/a.png" || invite["link_pic"] != "http://api.example.com/static/invite/pic.png" {
		t.Fatalf("push/invite=%#v %#v", push, invite)
	}
}

func TestWorkFissionStatisticsDeduplicatesEmployeesAndRates(t *testing.T) {
	store := &fakeWorkFissionStore{
		users: map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		stats: WorkFissionStatistics{
			FirstUserCount:   1,
			UserCount:        4,
			LossCount:        1,
			NewIncreaseCount: 3,
			NewLossCount:     1,
			InviteCount:      8,
			FirstLevelCount:  2,
			SecondLevelCount: 1,
			ThirdLevelCount:  1,
			Active:           []WorkFissionName{{ID: 9, ActiveName: "活动"}},
			ServiceEmployees: []string{`[{"id":1,"name":"张三"},{"id":2,"name":"李四"}]`, `[{"id":2,"name":"李四更新"}]`},
		},
	}
	handler := NewWorkFissionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "http://op.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/workFission/statistics?fission_ids=[9,10,9]", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Statistics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	user := data["user"].(map[string]any)
	if user["fission_rate"] != "75.00%" || user["insert_rate"] != "75.00%" || user["share_rate"] != "200.00%" {
		t.Fatalf("user=%#v", user)
	}
	employees := data["employee"].([]any)
	if len(employees) != 2 || employees[1].(map[string]any)["name"] != "李四更新" {
		t.Fatalf("employees=%#v", employees)
	}
	if len(store.statsIDs) != 2 || store.statsIDs[0] != 9 || store.statsIDs[1] != 10 {
		t.Fatalf("statsIDs=%v", store.statsIDs)
	}
}

func TestWorkFissionInviteDataDetailAndDestroy(t *testing.T) {
	store := &fakeWorkFissionStore{
		users: map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		invitePage: WorkFissionInviteDataPage{
			Items: []WorkFissionInviteDataItem{{
				ID:           21,
				Nickname:     "客户",
				Avatar:       "avatar.png",
				ActiveName:   "活动",
				Employee:     "wx-1",
				EmployeeName: "员工",
				CreatedAt:    "2026-07-03 10:00:00",
				Loss:         1,
				Status:       1,
				InviteCount:  3,
				ContactID:    31,
				EmployeeID:   41,
			}},
			Total:     1,
			TotalPage: 1,
			PerPage:   10000,
		},
		detail: WorkFissionInviteDetail{
			TotalCount: 3,
			NewCount:   2,
			LossCount:  1,
			Children:   []WorkFissionInviteDetailChild{{ID: 22, Nickname: "下级", Avatar: "child.png", Loss: 0, CreatedAt: "2026-07-03 11:00:00"}},
		},
		detailFound: true,
		deleteOK:    true,
	}
	handler := NewWorkFissionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "http://op.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/workFission/inviteData?fission_ids=[9]&status=1&loss=1", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.InviteData(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("inviteData status=%d body=%s", rec.Code, rec.Body.String())
	}
	row := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if row["loss"] != "已流失" || row["status"] != "已完成" || row["employee_id"].(float64) != 41 {
		t.Fatalf("row=%#v", row)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/workFission/inviteDetail?id=21", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.InviteDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("inviteDetail status=%d body=%s", rec.Code, rec.Body.String())
	}
	detail := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	if detail["insert"].(float64) != 2 || detail["user_list"].([]any)[0].(map[string]any)["avatar"] != "http://api.example.com/static/child.png" {
		t.Fatalf("detail=%#v", detail)
	}

	req = httptest.NewRequest(http.MethodDelete, "/dashboard/workFission/destroy?id=9", strings.NewReader(""))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.Destroy(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("destroy status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedCorpID != 7 || store.deletedID != 9 {
		t.Fatalf("deleted=%d/%d", store.deletedCorpID, store.deletedID)
	}
}

func TestWorkFissionInviteSendsMessageAndUpsertsConfig(t *testing.T) {
	store := &fakeWorkFissionStore{
		users:            map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		inviteTargets:    []string{"external-1", "external-2"},
		credentialFound:  true,
		credential:       RoomWelcomeCorpCredential{WXCorpID: "wx-corp", ContactSecret: "contact-secret"},
		bundleFound:      true,
		workFissionFound: true,
	}
	client := &fakeWorkFissionInviteClient{}
	handler := NewWorkFissionHandlerWithMessageClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "http://op.example.com", "/tmp/mochat-go-test", client)

	body := `{"fission_id":9,"text":"邀请文案","link_title":"邀请标题","link_desc":"邀请描述","link_pic":"invite/pic.png","filter":{"employee_ids":[21],"is_all":1,"start_time":"2026-07-01","end_time":"2026-07-04","gender":1}}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/workFission/invite", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Invite(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.inviteTargetFilter.CorpID != 7 || store.inviteTargetFilter.EmployeeIDs[0] != 21 || store.inviteTargetFilter.Gender == nil || *store.inviteTargetFilter.Gender != 1 {
		t.Fatalf("filter=%+v", store.inviteTargetFilter)
	}
	if store.upsertInvite.FissionID != 9 || store.upsertInvite.Text != "邀请文案" || store.upsertInvite.LinkPic != "invite/pic.png" || store.upsertInvite.CreateUserID != 1 {
		t.Fatalf("upsert=%+v", store.upsertInvite)
	}
	if len(client.payloads) != 1 {
		t.Fatalf("payloads=%#v", client.payloads)
	}
	payload := client.payloads[0]
	if len(payload.ExternalUserID) != 2 || payload.ExternalUserID[1] != "external-2" {
		t.Fatalf("external=%#v", payload.ExternalUserID)
	}
	if payload.Content[1].URL != "http://op.example.com/auth/workFission?id=9&target=%2FworkFission%3Fid%3D9" || payload.Content[1].PicURL != "http://api.example.com/static/invite/pic.png" {
		t.Fatalf("content=%#v", payload.Content)
	}
}

func TestWorkFissionStoreCreatesActivityBundle(t *testing.T) {
	store := &fakeWorkFissionStore{
		users:           map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		credentialFound: true,
		credential:      RoomWelcomeCorpCredential{WXCorpID: "wx-corp", ContactSecret: "contact-secret"},
		createdID:       77,
	}
	client := &fakeWorkFissionInviteClient{
		uploadURLs: []string{
			"https://wecom.example/welcome.png",
			"https://wecom.example/push.png",
			"https://wecom.example/invite.png",
		},
		contactWayConfigID: "contact-way-config",
		contactWayQRCode:   "https://wecom.example/qrcode.png",
	}
	handler := NewWorkFissionHandlerWithMessageClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "http://op.example.com", "/tmp/mochat-go-test", client)

	body := `{
		"fission":{
			"active_name":"新增裂变","service_employees":[{"id":21,"name":"员工","wxUserId":"wx-21"}],
			"auto_pass":true,"auto_add_tag":true,"contact_tags":[{"id":2}],
			"end_time":"2037-01-01 00:00:00","qr_code_invalid":7,"tasks":[{"count":3}],
			"new_friend":true,"delete_invalid":false,"receive_prize":0,
			"receive_prize_employees":[{"id":21,"wxUserId":"wx-21"}],"receive_links":[{"url":"https://gift.example"}]
		},
		"welcome":{"msg_text":"欢迎","link_title":"欢迎标题","link_desc":"欢迎描述","link_cover_url":"welcome/cover.png"},
		"poster":{"poster_type":1,"cover_pic":"poster/bg.png","foward_text":"转发","avatar_show":true,"nickname_show":false,"nickname_color":"#333333","card_corp_image_name":"形象","card_corp_name":"企业","card_corp_logo":"poster/logo.png","qrcode_w":"120","qrcode_h":"120","qrcode_x":"10","qrcode_y":"20"},
		"push":{"push_employee":true,"push_contact":false,"msg_text":"推送","msg_complex":{"msg_complex_type":"image","image":"push/image.png"}},
		"invite":{"text":"邀请","link_title":"邀请标题","link_desc":"邀请描述","link_pic":"invite/pic.png"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/workFission/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].([]any)
	if data[0].(float64) != 77 {
		t.Fatalf("data=%#v", data)
	}
	if store.created.Fission.CorpID != 7 || store.created.Fission.ActiveName != "新增裂变" || store.created.Fission.AutoPass != 1 || store.created.Fission.ReceiveQRCode == "[]" {
		t.Fatalf("fission=%+v", store.created.Fission)
	}
	if store.created.Welcome.LinkWXURL != "https://wecom.example/welcome.png" || store.created.Push.MsgComplexType != "image" || store.created.Invite.WXLinkPic != "https://wecom.example/invite.png" {
		t.Fatalf("created=%+v", store.created)
	}
	if len(client.contactWayUsers) != 1 || client.contactWayUsers[0] != "wx-21" || !client.contactWaySkipVerify {
		t.Fatalf("contact way users=%v skip=%v", client.contactWayUsers, client.contactWaySkipVerify)
	}
	if len(client.uploadPaths) != 3 || client.uploadPaths[0] != "/tmp/mochat-go-test/welcome/cover.png" {
		t.Fatalf("uploads=%#v", client.uploadPaths)
	}
}

func TestWorkFissionStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeWorkFissionStore{
		users: map[int]User{1: {ID: 1, TenantID: 8, IsSuperAdmin: 1}},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricWorkFissions,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	client := &fakeWorkFissionInviteClient{
		uploadURLs:           []string{"https://wecom.example/welcome.png"},
		contactWayConfigID:   "contact-way-config",
		contactWayQRCode:     "https://wecom.example/qrcode.png",
		contactWaySkipVerify: true,
	}
	handler := NewWorkFissionHandlerWithMessageClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "http://op.example.com", "/tmp/mochat-go-test", client)

	body := `{"fission":{"active_name":"新增裂变"},"welcome":{},"poster":{},"push":{},"invite":{}}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/workFission/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.quotaMetric != SaaSMetricWorkFissions || store.quotaTenantID != 8 || store.createCalls != 0 || store.refreshMetric != "" {
		t.Fatalf("quota metric=%q tenant=%d createCalls=%d refresh=%q", store.quotaMetric, store.quotaTenantID, store.createCalls, store.refreshMetric)
	}
	if len(client.uploadPaths) != 0 || len(client.contactWayUsers) != 0 {
		t.Fatalf("client uploads=%#v contactWay=%#v", client.uploadPaths, client.contactWayUsers)
	}
}

func TestWorkFissionUpdateWritesExistingActivity(t *testing.T) {
	store := &fakeWorkFissionStore{
		users:           map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		credentialFound: true,
		credential:      RoomWelcomeCorpCredential{WXCorpID: "wx-corp", ContactSecret: "contact-secret"},
		updateOK:        true,
	}
	client := &fakeWorkFissionInviteClient{uploadURLs: []string{"https://wecom.example/welcome-updated.png"}}
	handler := NewWorkFissionHandlerWithMessageClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "http://op.example.com", "/tmp/mochat-go-test", client)

	body := `{
		"fission":{
			"id":77,"active_name":"更新裂变","service_employees":[{"id":21,"name":"员工","wxUserId":"wx-21"}],
			"auto_pass":false,"auto_add_tag":false,"contact_tags":[],
			"end_time":"2037-02-01 00:00:00","qr_code_invalid":3,"tasks":[],
			"receive_prize":1,"receive_prize_employees":[],"receive_links":[{"url":"https://gift.example/new"}]
		},
		"welcome":{"msg_text":"欢迎更新","link_title":"标题更新","link_desc":"描述更新","link_cover_url":"welcome/new.png"},
		"poster":{"poster_type":0,"cover_pic":"","foward_text":"转发更新","avatar_show":false,"nickname_show":true,"nickname_color":"#111111","card_corp_image_name":"","card_corp_name":"","card_corp_logo":"","qrcode_w":"90","qrcode_h":"90","qrcode_x":"1","qrcode_y":"2"},
		"push":{"push_employee":false,"push_contact":true,"msg_text":"推送更新","msg_complex_type":"","msg_complex":{}}
	}`
	req := httptest.NewRequest(http.MethodPut, "/dashboard/workFission/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.updatedCorpID != 7 || store.updatedID != 77 {
		t.Fatalf("updated target=%d/%d", store.updatedCorpID, store.updatedID)
	}
	if store.updated.Fission.ActiveName != "更新裂变" || store.updated.Fission.NewFriend != 0 || store.updated.Push.PushContact != 1 || store.updated.Invite.Text != "" {
		t.Fatalf("updated=%+v", store.updated)
	}
}

type fakeWorkFissionStore struct {
	users              map[int]User
	page               WorkFissionListPage
	listFilter         WorkFissionListFilter
	bundle             WorkFissionBundle
	bundleFound        bool
	stats              WorkFissionStatistics
	statsIDs           []int
	chooseCount        int
	invitePage         WorkFissionInviteDataPage
	inviteFilter       WorkFissionInviteDataFilter
	detail             WorkFissionInviteDetail
	detailFound        bool
	deleteOK           bool
	deletedCorpID      int
	deletedID          int
	inviteTargets      []string
	inviteTargetFilter WorkFissionChooseContactFilter
	upsertInvite       WorkFissionInviteWrite
	credential         RoomWelcomeCorpCredential
	credentialFound    bool
	workFissionFound   bool
	created            WorkFissionWrite
	createdID          int
	createCalls        int
	updated            WorkFissionWrite
	updatedCorpID      int
	updatedID          int
	updateOK           bool
	quota              SaaSQuotaStatus
	quotaTenantID      int
	quotaMetric        string
	refreshTenantID    int
	refreshMetric      string
}

func (s *fakeWorkFissionStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeWorkFissionStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 99, nil
}

func (s *fakeWorkFissionStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeWorkFissionStore) WorkFissionPage(_ context.Context, filter WorkFissionListFilter) (WorkFissionListPage, error) {
	s.listFilter = filter
	return s.page, nil
}

func (s *fakeWorkFissionStore) WorkFissionBundleByID(_ context.Context, _ int, _ int) (WorkFissionBundle, bool, error) {
	return s.bundle, s.bundleFound, nil
}

func (s *fakeWorkFissionStore) WorkFissionStatistics(_ context.Context, _ int, fissionIDs []int) (WorkFissionStatistics, error) {
	s.statsIDs = append([]int{}, fissionIDs...)
	return s.stats, nil
}

func (s *fakeWorkFissionStore) WorkFissionChooseContactCount(_ context.Context, _ WorkFissionChooseContactFilter) (int, error) {
	return s.chooseCount, nil
}

func (s *fakeWorkFissionStore) WorkFissionInviteDataPage(_ context.Context, filter WorkFissionInviteDataFilter) (WorkFissionInviteDataPage, error) {
	s.inviteFilter = filter
	return s.invitePage, nil
}

func (s *fakeWorkFissionStore) WorkFissionInviteDetail(_ context.Context, _ int, _ int) (WorkFissionInviteDetail, bool, error) {
	return s.detail, s.detailFound, nil
}

func (s *fakeWorkFissionStore) WorkFissionInviteTargets(_ context.Context, filter WorkFissionChooseContactFilter) ([]string, error) {
	s.inviteTargetFilter = filter
	return s.inviteTargets, nil
}

func (s *fakeWorkFissionStore) CreateWorkFission(_ context.Context, values WorkFissionWrite) (int, error) {
	s.createCalls++
	s.created = values
	return s.createdID, nil
}

func (s *fakeWorkFissionStore) UpdateWorkFission(_ context.Context, corpID int, id int, values WorkFissionWrite) (bool, error) {
	s.updatedCorpID = corpID
	s.updatedID = id
	s.updated = values
	return s.updateOK, nil
}

func (s *fakeWorkFissionStore) UpsertWorkFissionInvite(_ context.Context, values WorkFissionInviteWrite) error {
	s.upsertInvite = values
	return nil
}

func (s *fakeWorkFissionStore) DeleteWorkFissionCascade(_ context.Context, corpID int, id int) (bool, error) {
	s.deletedCorpID = corpID
	s.deletedID = id
	return s.deleteOK, nil
}

func (s *fakeWorkFissionStore) RoomWelcomeCorpCredentialByID(_ context.Context, _ int) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, s.credentialFound, nil
}

func (s *fakeWorkFissionStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	status := s.quota
	status.TenantID = tenantID
	status.Metric = metric
	status.Additional = additional
	return status, nil
}

func (s *fakeWorkFissionStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}

type fakeWorkFissionInviteClient struct {
	payloads             []ContactMessageBatchSendMessagePayload
	uploadPaths          []string
	uploadURLs           []string
	contactWayUsers      []string
	contactWaySkipVerify bool
	contactWayConfigID   string
	contactWayQRCode     string
}

func (c *fakeWorkFissionInviteClient) UploadImage(_ context.Context, _ RoomWelcomeCorpCredential, filePath string) (string, error) {
	c.uploadPaths = append(c.uploadPaths, filePath)
	if len(c.uploadURLs) == 0 {
		return "https://wecom.example/uploaded.png", nil
	}
	url := c.uploadURLs[0]
	c.uploadURLs = c.uploadURLs[1:]
	return url, nil
}

func (c *fakeWorkFissionInviteClient) CreateContactWay(_ context.Context, _ RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, _ string) (string, string, error) {
	c.contactWayUsers = append([]string{}, userIDs...)
	c.contactWaySkipVerify = skipVerify
	return c.contactWayQRCode, c.contactWayConfigID, nil
}

func (c *fakeWorkFissionInviteClient) SubmitContactMessageBatchSend(_ context.Context, _ RoomWelcomeCorpCredential, payload ContactMessageBatchSendMessagePayload) (ContactMessageBatchSendMessageResult, error) {
	c.payloads = append(c.payloads, payload)
	return ContactMessageBatchSendMessageResult{ErrCode: 0, ErrMsg: "ok", MsgID: "msg-work-fission"}, nil
}
