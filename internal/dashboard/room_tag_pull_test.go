package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoomTagPullIndexReturnsPHPCompatiblePage(t *testing.T) {
	creatorID := 1
	store := &fakeRoomTagPullStore{
		users: map[int]User{1: {ID: 1, Name: "管理员", IsSuperAdmin: 0}},
		page: RoomTagPullPage{
			PerPage:   10000,
			Total:     1,
			TotalPage: 1,
			Items: []RoomTagPullItem{{
				ID: 900001, Name: "标签建群", Employees: []string{"员工A"}, Rooms: []string{"客户群A"},
				InviteNum: 2, JoinRoomNum: 1, NoSendNum: 1, NoInviteNum: 3, CreatedAt: "2026-07-03 10:00:00",
			}},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewRoomTagPullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/roomTagPull/index?name=%E6%A0%87%E7%AD%BE", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/roomTagPull/index#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if store.filter.CorpID != 7 || store.filter.Name != "标签" || store.filter.CreatorID == nil || *store.filter.CreatorID != creatorID {
		t.Fatalf("filter = %#v", store.filter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	item := data["list"].([]any)[0].(map[string]any)
	if int(item["id"].(float64)) != 900001 || item["name"] != "标签建群" || int(item["no_send_num"].(float64)) != 1 {
		t.Fatalf("item = %#v", item)
	}
	if item["employees"].([]any)[0] != "员工A" || item["rooms"].([]any)[0] != "客户群A" {
		t.Fatalf("item arrays = %#v", item)
	}
}

func TestRoomTagPullShowReturnsDetail(t *testing.T) {
	store := &fakeRoomTagPullStore{
		users:     map[int]User{1: {ID: 1}},
		showFound: true,
		show: RoomTagPullShow{
			Employees:   []RoomTagPullEmployee{{Name: "员工A", Avatar: "avatar/a.png", WXUserID: "wx-a"}},
			Rooms:       []RoomTagPullShowRoom{{ID: 11, Name: "客户群A", RoomMax: 200, ContactNum: 8}},
			JoinRoomNum: 3, NoJoinRoomNum: 5, InviteNum: 4, NoInviteNum: 2, SendNum: 1, NoSendNum: 1,
		},
	}
	handler := NewRoomTagPullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/roomTagPull/show?id=900001", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.showID != 900001 {
		t.Fatalf("show id = %d", store.showID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	employee := data["employees"].([]any)[0].(map[string]any)
	if employee["avatar"] != "http://api.example.com/static/avatar/a.png" || employee["wxUserId"] != "wx-a" {
		t.Fatalf("employee = %#v", employee)
	}
	room := data["rooms"].([]any)[0].(map[string]any)
	if int(room["room_max"].(float64)) != 200 || int(room["contact_num"].(float64)) != 8 {
		t.Fatalf("room = %#v", room)
	}
	if int(data["join_room_num"].(float64)) != 3 || int(data["no_send_num"].(float64)) != 1 {
		t.Fatalf("data = %#v", data)
	}
}

func TestRoomTagPullShowContactReturnsContactAndEmployeeViews(t *testing.T) {
	store := &fakeRoomTagPullStore{
		users: map[int]User{1: {ID: 1}},
		contactPage: RoomTagPullContactPage{
			PerPage: 10000, Total: 1, TotalPage: 1, ContactNum: 6,
			Items: []RoomTagPullContactItem{{Avatar: "avatar/c.png", ContactName: "客户A", EmployeeName: "员工A", SendStatus: 1, RoomName: "客户群A", IsJoinRoom: 1}},
		},
		employeeTasks:  []RoomTagPullEmployeeTask{{WXUserID: "wx-a", Status: 0, TaskNum: 2, Name: "员工A", Avatar: "avatar/a.png", ContactNum: 6, InviteNum: 2}},
		taskContactNum: 6,
	}
	handler := NewRoomTagPullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/roomTagPull/showContact?id=900001&type=1&contact_name=%E5%AE%A2%E6%88%B7&send_status=1", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ShowContact(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("contact status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.contactFilter.RoomTagPullID != 900001 || store.contactFilter.ContactName != "客户" || store.contactFilter.SendStatus == nil || *store.contactFilter.SendStatus != 1 {
		t.Fatalf("contact filter = %#v", store.contactFilter)
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	item := data["list"].([]any)[0].(map[string]any)
	if item["avatar"] != "http://api.example.com/static/avatar/c.png" || item["contact_name"] != "客户A" {
		t.Fatalf("contact item = %#v", item)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/roomTagPull/showContact?id=900001&type=2&wx_user_id=wx-a&is_send=0", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.ShowContact(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("employee status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.taskFilter.WXUserID != "wx-a" || store.taskFilter.IsSend == nil || *store.taskFilter.IsSend != 0 {
		t.Fatalf("task filter = %#v", store.taskFilter)
	}
	data = decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	task := data["list"].([]any)[0].(map[string]any)
	if task["avatar"] != "http://api.example.com/static/avatar/a.png" || int(task["task_num"].(float64)) != 2 {
		t.Fatalf("task = %#v", task)
	}
}

func TestRoomTagPullRoomListAndChooseContact(t *testing.T) {
	store := &fakeRoomTagPullStore{
		users:       map[int]User{1: {ID: 1}},
		rooms:       []RoomTagPullRoom{{ID: 11, WXChatID: "chat-a", Name: "客户群A", OwnerID: 21, RoomMax: 200, ContactNum: 8, AuditStatus: 1}},
		searchCount: 12,
	}
	handler := NewRoomTagPullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/roomTagPull/roomList?employees[]=21&type=2&name=%E5%AE%A2%E6%88%B7", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.RoomList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("room list status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.roomFilter.CorpID != 7 || len(store.roomFilter.EmployeeIDs) != 1 || store.roomFilter.EmployeeIDs[0] != 21 || store.roomFilter.Type != 2 {
		t.Fatalf("room filter = %#v", store.roomFilter)
	}
	room := decodeBody(t, rec.Body.Bytes())["data"].([]any)[0].(map[string]any)
	if room["wxChatId"] != "chat-a" || int(room["contact_num"].(float64)) != 8 {
		t.Fatalf("room = %#v", room)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/roomTagPull/chooseContact?employees[]=21&is_all=1&gender=2&tag_ids[]=31", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.ChooseContact(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("choose status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.search.CorpID != 7 || store.search.Search.Gender == nil || *store.search.Search.Gender != 2 || len(store.search.Search.TagIDs) != 1 || store.search.Search.TagIDs[0] != 31 {
		t.Fatalf("search = %#v", store.search)
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].([]any)
	if int(data[0].(float64)) != 12 {
		t.Fatalf("choose data = %#v", data)
	}
}

func TestRoomTagPullFilterContactCountsFilteredContacts(t *testing.T) {
	store := &fakeRoomTagPullStore{
		users:         map[int]User{1: {ID: 1}},
		filteredCount: 9,
	}
	handler := NewRoomTagPullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	req := httptest.NewRequest(http.MethodPost, "/dashboard/roomTagPull/filterContact", strings.NewReader(`{"employees":[21],"choose_contact":{"is_all":1,"gender":2,"tag_ids":[31]},"rooms":[{"id":11,"num":50}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.FilterContact(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.filtered.CorpID != 7 || store.filtered.Search.Gender == nil || *store.filtered.Search.Gender != 2 || store.filtered.Rooms[0].ID != 11 || store.filtered.FilterContact != 1 {
		t.Fatalf("filtered = %#v", store.filtered)
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].([]any)
	if int(data[0].(float64)) != 9 {
		t.Fatalf("data = %#v", data)
	}
}

func TestRoomTagPullStoreCreatesActivityAndSendsMessage(t *testing.T) {
	store := &fakeRoomTagPullStore{
		users:           map[int]User{1: {ID: 1}},
		credentialFound: true,
		credential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wx-corp", ContactSecret: "secret"},
		filteredCount:   2,
		createdID:       910001,
		sendTargets: []RoomTagPullSendTarget{{
			EmployeeID: 21,
			WXUserID:   "wx-user-21",
			Contacts: []RoomTagPullSendContact{
				{ContactID: 31, WXExternalUserID: "external-31", ContactName: "客户A", EmployeeID: 21, WXUserID: "wx-user-21"},
			},
		}},
	}
	client := &fakeRoomTagPullMessageClient{wxImage: "https://wecom.example/room.png", msgID: "msg-910001"}
	handler := NewRoomTagPullHandlerWithMessageClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", "/tmp/mochat-go-test", client)

	body := `{"name":"标签建群","employees":[21],"choose_contact":{"is_all":1,"gender":1,"tag_ids":[31]},"guide":"请扫码入群","rooms":[{"id":11,"name":"客户群A","num":50,"image":"image/local.jpg"}],"filter_contact":1}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/roomTagPull/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if client.uploadPath != filepath.Join("/tmp/mochat-go-test", "image/local.jpg") || len(client.messages) != 1 {
		t.Fatalf("client upload=%s messages=%#v", client.uploadPath, client.messages)
	}
	if client.messages[0].Sender != "wx-user-21" || client.messages[0].ExternalUserIDs[0] != "external-31" || client.messages[0].ImagePicURL != "https://wecom.example/room.png" {
		t.Fatalf("message = %#v", client.messages[0])
	}
	if store.created.Name != "标签建群" || store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.ContactNum != 2 || store.created.WXTIDJSON == "" {
		t.Fatalf("created = %#v", store.created)
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].([]any)
	if int(data[0].(float64)) != 910001 {
		t.Fatalf("data = %#v", data)
	}
}

func TestRoomTagPullStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeRoomTagPullStore{
		users: map[int]User{1: {ID: 1, TenantID: 8}},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricRoomTagPulls,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	client := &fakeRoomTagPullMessageClient{wxImage: "https://wecom.example/room.png", msgID: "msg-910001"}
	handler := NewRoomTagPullHandlerWithMessageClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", "/tmp/mochat-go-test", client)

	body := `{"name":"标签建群","employees":[21],"choose_contact":{"is_all":1},"guide":"请扫码入群","rooms":[{"id":11,"name":"客户群A","num":50,"image":"image/local.jpg"}],"filter_contact":1}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/roomTagPull/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.quotaMetric != SaaSMetricRoomTagPulls || store.quotaTenantID != 8 || store.createCalls != 0 || store.refreshMetric != "" {
		t.Fatalf("quota metric=%q tenant=%d createCalls=%d refresh=%q", store.quotaMetric, store.quotaTenantID, store.createCalls, store.refreshMetric)
	}
	if client.uploadPath != "" || len(client.messages) != 0 {
		t.Fatalf("client upload=%q messages=%#v", client.uploadPath, client.messages)
	}
}

func TestRoomTagPullRemindSendSendsAgentMessageForPendingTask(t *testing.T) {
	store := &fakeRoomTagPullStore{
		users: map[int]User{1: {ID: 1}},
		remindActivity: RoomTagPullRemindActivity{
			EmployeeIDs:   []int{21},
			ChooseContact: RoomTagPullContactSearch{Employees: []int{21}, IsAll: 1},
			ContactNum:    2,
			CreatedAt:     "2026-07-03 10:00:00",
			WXTIDs: []RoomTagPullWXTID{
				{WXUserID: "wx-user-21", TID: "msg-1", Status: 0},
				{WXUserID: "wx-user-22", TID: "msg-2", Status: 1},
			},
		},
		remindFound:      true,
		remindAgent:      RoomTagPullAgentCredential{CorpID: 7, WXCorpID: "wx-corp", WXAgentID: "1000001", WXSecret: "agent-secret"},
		remindAgentFound: true,
		sendTargets: []RoomTagPullSendTarget{{
			EmployeeID: 21,
			WXUserID:   "wx-user-21",
			Contacts: []RoomTagPullSendContact{
				{ContactID: 31, WXExternalUserID: "external-31", ContactName: "客户A", EmployeeID: 21, WXUserID: "wx-user-21"},
			},
		}},
	}
	client := &fakeRoomTagPullMessageClient{}
	handler := NewRoomTagPullHandlerWithMessageClient(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "", "/tmp/mochat-go-test", client)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/roomTagPull/remindSend?id=900001&wxUserId=wx-user-21", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.RemindSend(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.remindID != 900001 || store.remindAgentCorpID != 7 {
		t.Fatalf("remind id=%d agent corp=%d", store.remindID, store.remindAgentCorpID)
	}
	if len(client.agentMessages) != 1 {
		t.Fatalf("agent messages = %#v", client.agentMessages)
	}
	message := client.agentMessages[0]
	if message.ToUser != "wx-user-21" || message.Credential.WXAgentID != "1000001" {
		t.Fatalf("message target = %#v", message)
	}
	if !strings.Contains(message.Content, "任务创建于2026-07-03 10:00:00") || !strings.Contains(message.Content, "客户A等2个客户") {
		t.Fatalf("message content = %q", message.Content)
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].([]any)
	if len(data) != 0 {
		t.Fatalf("data = %#v", data)
	}
}

func TestRoomTagPullDestroyDeletesActivity(t *testing.T) {
	store := &fakeRoomTagPullStore{users: map[int]User{1: {ID: 1}}, deleteFound: true}
	authorizer := &recordingAuthorizer{}
	handler := NewRoomTagPullHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "")

	req := httptest.NewRequest(http.MethodDelete, "/dashboard/roomTagPull/destroy", strings.NewReader(`{"id":900001}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/roomTagPull/destroy#delete" || store.deletedID != 900001 {
		t.Fatalf("authorizer = %#v deleted=%d", authorizer, store.deletedID)
	}
}

type fakeRoomTagPullStore struct {
	users          map[int]User
	page           RoomTagPullPage
	filter         RoomTagPullFilter
	show           RoomTagPullShow
	showFound      bool
	showID         int
	contactPage    RoomTagPullContactPage
	contactFilter  RoomTagPullContactFilter
	employeeTasks  []RoomTagPullEmployeeTask
	taskContactNum int
	taskFilter     RoomTagPullEmployeeTaskFilter
	rooms          []RoomTagPullRoom
	roomFilter     RoomTagPullRoomFilter
	searchCount    int
	search         struct {
		CorpID int
		Search RoomTagPullContactSearch
	}
	filteredCount int
	filtered      struct {
		CorpID        int
		Search        RoomTagPullContactSearch
		Rooms         []RoomTagPullWriteRoom
		FilterContact int
	}
	credential        RoomWelcomeCorpCredential
	credentialFound   bool
	sendTargets       []RoomTagPullSendTarget
	created           RoomTagPullWrite
	createdID         int
	createCalls       int
	remindActivity    RoomTagPullRemindActivity
	remindFound       bool
	remindID          int
	remindAgent       RoomTagPullAgentCredential
	remindAgentFound  bool
	remindAgentCorpID int
	deleteFound       bool
	deletedID         int
	quota             SaaSQuotaStatus
	quotaTenantID     int
	quotaMetric       string
	refreshTenantID   int
	refreshMetric     string
}

func (s *fakeRoomTagPullStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeRoomTagPullStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 99, nil
}

func (s *fakeRoomTagPullStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeRoomTagPullStore) RoomWelcomeCorpCredentialByID(_ context.Context, _ int) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, s.credentialFound, nil
}

func (s *fakeRoomTagPullStore) RoomTagPullRemindAgentByCorpID(_ context.Context, corpID int) (RoomTagPullAgentCredential, bool, error) {
	s.remindAgentCorpID = corpID
	return s.remindAgent, s.remindAgentFound, nil
}

func (s *fakeRoomTagPullStore) RoomTagPullPage(_ context.Context, filter RoomTagPullFilter) (RoomTagPullPage, error) {
	s.filter = filter
	return s.page, nil
}

func (s *fakeRoomTagPullStore) RoomTagPullByID(_ context.Context, id int) (RoomTagPullShow, bool, error) {
	s.showID = id
	return s.show, s.showFound, nil
}

func (s *fakeRoomTagPullStore) RoomTagPullRemindByID(_ context.Context, id int) (RoomTagPullRemindActivity, bool, error) {
	s.remindID = id
	return s.remindActivity, s.remindFound, nil
}

func (s *fakeRoomTagPullStore) RoomTagPullContactPage(_ context.Context, filter RoomTagPullContactFilter) (RoomTagPullContactPage, error) {
	s.contactFilter = filter
	return s.contactPage, nil
}

func (s *fakeRoomTagPullStore) RoomTagPullEmployeeTasks(_ context.Context, filter RoomTagPullEmployeeTaskFilter) ([]RoomTagPullEmployeeTask, int, error) {
	s.taskFilter = filter
	return append([]RoomTagPullEmployeeTask{}, s.employeeTasks...), s.taskContactNum, nil
}

func (s *fakeRoomTagPullStore) RoomTagPullRooms(_ context.Context, filter RoomTagPullRoomFilter) ([]RoomTagPullRoom, error) {
	s.roomFilter = filter
	return append([]RoomTagPullRoom{}, s.rooms...), nil
}

func (s *fakeRoomTagPullStore) CountRoomTagPullSearchContacts(_ context.Context, corpID int, search RoomTagPullContactSearch) (int, error) {
	s.search.CorpID = corpID
	s.search.Search = search
	return s.searchCount, nil
}

func (s *fakeRoomTagPullStore) CountRoomTagPullFilteredContacts(_ context.Context, corpID int, search RoomTagPullContactSearch, rooms []RoomTagPullWriteRoom, filterContact int) (int, error) {
	s.filtered.CorpID = corpID
	s.filtered.Search = search
	s.filtered.Rooms = rooms
	s.filtered.FilterContact = filterContact
	return s.filteredCount, nil
}

func (s *fakeRoomTagPullStore) RoomTagPullSendTargets(_ context.Context, _ int, _ []int, _ RoomTagPullContactSearch) ([]RoomTagPullSendTarget, error) {
	return append([]RoomTagPullSendTarget{}, s.sendTargets...), nil
}

func (s *fakeRoomTagPullStore) CreateRoomTagPull(_ context.Context, values RoomTagPullWrite) (int, error) {
	s.createCalls++
	s.created = values
	return s.createdID, nil
}

func (s *fakeRoomTagPullStore) DeleteRoomTagPull(_ context.Context, id int) (bool, error) {
	s.deletedID = id
	return s.deleteFound, nil
}

func (s *fakeRoomTagPullStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	status := s.quota
	status.TenantID = tenantID
	status.Metric = metric
	status.Additional = additional
	return status, nil
}

func (s *fakeRoomTagPullStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}

type fakeRoomTagPullMessageClient struct {
	wxImage       string
	msgID         string
	uploadPath    string
	messages      []RoomTagPullMessagePayload
	agentMessages []fakeRoomTagPullAgentMessage
}

type fakeRoomTagPullAgentMessage struct {
	Credential RoomTagPullAgentCredential
	ToUser     string
	Content    string
}

func (c *fakeRoomTagPullMessageClient) UploadImage(_ context.Context, _ RoomWelcomeCorpCredential, filePath string) (string, error) {
	c.uploadPath = filePath
	return c.wxImage, nil
}

func (c *fakeRoomTagPullMessageClient) CreateExternalContactMessage(_ context.Context, _ RoomWelcomeCorpCredential, payload RoomTagPullMessagePayload) (string, error) {
	c.messages = append(c.messages, payload)
	return c.msgID, nil
}

func (c *fakeRoomTagPullMessageClient) SendAgentTextMessage(_ context.Context, credential RoomTagPullAgentCredential, toUser string, content string) error {
	c.agentMessages = append(c.agentMessages, fakeRoomTagPullAgentMessage{
		Credential: credential,
		ToUser:     toUser,
		Content:    content,
	})
	return nil
}
