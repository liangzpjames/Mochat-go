package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRoomMessageBatchSendIndexAndShow(t *testing.T) {
	store := &fakeRoomMessageBatchSendStore{
		users: map[int]User{1: {ID: 1, Name: "管理员"}},
		page: RoomMessageBatchSendPage{
			PerPage:   10,
			Total:     1,
			TotalPage: 1,
			Items: []RoomMessageBatchSendItem{{
				ID: 810001, UserID: 1, UserName: "管理员", BatchTitle: "群发A", SendWay: 1, SendStatus: 1, CreatedAt: "2026-07-04 10:00:00",
				Content: []ContactMessageBatchSendContent{{MsgType: "image", PicURL: "image/a.jpg"}},
			}},
		},
		batches: map[int]RoomMessageBatchSendItem{
			810001: {
				ID: 810001, UserID: 1, UserName: "管理员", BatchTitle: "群发A", CreatedAt: "2026-07-04 10:00:00",
				Content: []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}},
			},
		},
		seedRooms: []ContactMessageBatchSendNameID{{ID: 11, Name: "客户群A"}},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewRoomMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "http://api.example.com", t.TempDir(), nil)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/roomMessageBatchSend/index?batchTitle=%E7%BE%A4%E5%8F%91&page=1&perPage=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("index status=%d body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/roomMessageBatchSend/index#get" || store.filter.UserID != 1 || store.filter.BatchTitle != "群发" {
		t.Fatalf("auth=%#v filter=%#v", authorizer, store.filter)
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	item := data["list"].([]any)[0].(map[string]any)
	content := item["content"].([]any)[0].(map[string]any)
	if item["batchTitle"] != "群发A" || content["pic_url"] != "http://api.example.com/static/image/a.jpg" {
		t.Fatalf("item=%#v content=%#v", item, content)
	}

	req = authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/roomMessageBatchSend/show?batchId=810001", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.Show(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("show status=%d body=%s", rec.Code, rec.Body.String())
	}
	show := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	if show["creator"] != "管理员" || show["seedRooms"].([]any)[0].(map[string]any)["name"] != "客户群A" {
		t.Fatalf("show=%#v", show)
	}
}

func TestRoomMessageBatchSendStoreCreatesTasksAndSubmitsWeCom(t *testing.T) {
	store := &fakeRoomMessageBatchSendStore{
		users:           map[int]User{1: {ID: 1, Name: "管理员"}},
		batches:         map[int]RoomMessageBatchSendItem{},
		createdID:       810002,
		employeesValid:  true,
		credentialFound: true,
		credential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wx-corp", ContactSecret: "secret"},
		sendTargets: []RoomMessageBatchSendTarget{{
			EmployeeID: 21,
			WXUserID:   "wx-user-21",
			ChatIDs:    []string{"chat-1", "chat-2"},
		}},
	}
	client := &fakeRoomMessageBatchSendClient{mediaID: "media-image", msgID: "msg-810002"}
	handler := NewRoomMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "/tmp/mochat-go-test", client)

	body := `{"batchTitle":"Go 客户群群发","employeeIds":[21],"content":[{"msgType":"text","content":"hello"},{"msgType":"image","pic_url":"image/a.jpg"}],"mediumId":45,"sendWay":1}`
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomMessageBatchSend/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.UserID != 1 || store.created.MediumID != 45 || store.created.BatchTitle != "Go 客户群群发" || len(store.created.EmployeeIDs) != 1 {
		t.Fatalf("created=%#v", store.created)
	}
	if client.uploadPath != filepath.Join("/tmp/mochat-go-test", "image/a.jpg") || len(client.submits) != 1 {
		t.Fatalf("uploads=%#v submits=%#v", client.uploadPaths, client.submits)
	}
	submit := client.submits[0]
	if submit.Sender != "wx-user-21" || submit.ChatIDs[0] != "chat-1" || submit.Content[1].MediaID != "media-image" {
		t.Fatalf("submit=%#v", submit)
	}
	if len(store.markedResults) != 1 || store.markedResults[0].EmployeeID != 21 || store.markedResults[0].MsgID != "msg-810002" {
		t.Fatalf("marked=%#v", store.markedResults)
	}
}

func TestRoomMessageBatchSendStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeRoomMessageBatchSendStore{
		users:          map[int]User{1: {ID: 1, Name: "管理员", TenantID: 8}},
		employeesValid: true,
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricRoomMessageBatches,
			Current:    2,
			Limit:      2,
			Additional: 1,
		},
	}
	handler := NewRoomMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "/tmp/mochat-go-test", nil)

	body := `{"batchTitle":"Go 客户群群发","employeeIds":[21],"content":[{"msgType":"text","content":"hello"}],"sendWay":2,"definiteTime":"2026-07-04 10:00:00"}`
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomMessageBatchSend/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.quotaMetric != SaaSMetricRoomMessageBatches || store.quotaTenantID != 8 || store.createCalls != 0 || store.refreshMetric != "" {
		t.Fatalf("quota metric=%q tenant=%d createCalls=%d refresh=%q", store.quotaMetric, store.quotaTenantID, store.createCalls, store.refreshMetric)
	}
}

func TestRoomMessageBatchSendDetailListsRemindAndDestroy(t *testing.T) {
	store := &fakeRoomMessageBatchSendStore{
		users: map[int]User{1: {ID: 1}},
		batches: map[int]RoomMessageBatchSendItem{
			810003: {ID: 810003, CorpID: 7, UserID: 1, BatchTitle: "群发A", CreatedAt: "2026-07-04 10:00:00"},
		},
		ownerPage: RoomMessageBatchSendOwnerPage{
			PerPage: 15, Total: 1, TotalPage: 1,
			Items: []RoomMessageBatchSendOwnerItem{{ID: 501, EmployeeID: 21, EmployeeName: "员工A", EmployeeAvatar: "avatar/a.jpg"}},
		},
		roomPage: RoomMessageBatchSendRoomPage{
			PerPage: 15, Total: 1, TotalPage: 1,
			Items: []RoomMessageBatchSendRoomItem{{ID: 601, RoomID: 31, RoomName: "客户群A", EmployeeID: 21, EmployeeName: "员工A", SendTime: 1783159200}},
		},
		remindAgent:      RoomTagPullAgentCredential{CorpID: 7, WXCorpID: "wx-corp", WXAgentID: "1000001", WXSecret: "agent-secret"},
		remindAgentFound: true,
		deleteFound:      true,
	}
	client := &fakeRoomMessageBatchSendClient{}
	handler := NewRoomMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", t.TempDir(), client)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/roomMessageBatchSend/roomOwnerSendIndex?batchId=810003&sendStatus=0&page=1&perPage=15", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.RoomOwnerSendIndex(rec, req)
	if rec.Code != http.StatusOK || store.ownerFilter.BatchID != 810003 || store.ownerFilter.SendStatus == nil || *store.ownerFilter.SendStatus != 0 {
		t.Fatalf("owner status=%d filter=%#v body=%s", rec.Code, store.ownerFilter, rec.Body.String())
	}
	owner := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if owner["employeeAvatar"] != "http://api.example.com/static/avatar/a.jpg" {
		t.Fatalf("owner=%#v", owner)
	}

	req = authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/roomMessageBatchSend/roomReceiveIndex?batchId=810003&sendStatus=0&keyWords=%E5%AE%A2%E6%88%B7&page=1&perPage=15", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.RoomReceiveIndex(rec, req)
	if rec.Code != http.StatusOK || store.roomFilter.BatchID != 810003 || store.roomFilter.KeyWords != "客户" {
		t.Fatalf("room status=%d filter=%#v body=%s", rec.Code, store.roomFilter, rec.Body.String())
	}
	room := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if room["roomName"] != "客户群A" || room["sendTime"] == "" {
		t.Fatalf("room=%#v", room)
	}

	req = authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/roomMessageBatchSend/remind?batchId=810003&batchEmployId=21", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.Remind(rec, req)
	if rec.Code != http.StatusOK || len(client.agentMessages) != 1 || client.agentMessages[0].ToUser != "wx-user-21" || !strings.Contains(client.agentMessages[0].Content, "客户群群发任务") {
		t.Fatalf("remind status=%d messages=%#v body=%s", rec.Code, client.agentMessages, rec.Body.String())
	}

	req = authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/roomMessageBatchSend/destroy", strings.NewReader(`{"batchId":810003}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.Destroy(rec, req)
	if rec.Code != http.StatusOK || store.deletedID != 810003 {
		t.Fatalf("destroy status=%d deleted=%d body=%s", rec.Code, store.deletedID, rec.Body.String())
	}
}

type fakeRoomMessageBatchSendStore struct {
	users            map[int]User
	page             RoomMessageBatchSendPage
	filter           RoomMessageBatchSendFilter
	batches          map[int]RoomMessageBatchSendItem
	seedRooms        []ContactMessageBatchSendNameID
	created          RoomMessageBatchSendWrite
	createdID        int
	createCalls      int
	employeesValid   bool
	credential       RoomWelcomeCorpCredential
	credentialFound  bool
	sendTargets      []RoomMessageBatchSendTarget
	markedResults    []RoomMessageBatchSendMessageResult
	ownerPage        RoomMessageBatchSendOwnerPage
	ownerFilter      RoomMessageBatchSendOwnerFilter
	roomPage         RoomMessageBatchSendRoomPage
	roomFilter       RoomMessageBatchSendRoomFilter
	employees        []RoomMessageBatchSendEmployeeRef
	remindAgent      RoomTagPullAgentCredential
	remindAgentFound bool
	deleteFound      bool
	deletedID        int
	quota            SaaSQuotaStatus
	quotaTenantID    int
	quotaMetric      string
	refreshTenantID  int
	refreshMetric    string
}

func (s *fakeRoomMessageBatchSendStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeRoomMessageBatchSendStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 99, nil
}

func (s *fakeRoomMessageBatchSendStore) MediumAvailableToUser(context.Context, int, int, int) (bool, error) {
	return true, nil
}

func (s *fakeRoomMessageBatchSendStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeRoomMessageBatchSendStore) RoomWelcomeCorpCredentialByID(_ context.Context, _ int) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, s.credentialFound, nil
}

func (s *fakeRoomMessageBatchSendStore) RoomTagPullRemindAgentByCorpID(_ context.Context, _ int) (RoomTagPullAgentCredential, bool, error) {
	return s.remindAgent, s.remindAgentFound, nil
}

func (s *fakeRoomMessageBatchSendStore) RoomMessageBatchSendPage(_ context.Context, filter RoomMessageBatchSendFilter) (RoomMessageBatchSendPage, error) {
	s.filter = filter
	return s.page, nil
}

func (s *fakeRoomMessageBatchSendStore) RoomMessageBatchSendByID(_ context.Context, batchID int) (RoomMessageBatchSendItem, bool, error) {
	item, ok := s.batches[batchID]
	return item, ok, nil
}

func (s *fakeRoomMessageBatchSendStore) RoomMessageBatchSendSeedRooms(_ context.Context, _ int, _ int) ([]ContactMessageBatchSendNameID, error) {
	return append([]ContactMessageBatchSendNameID{}, s.seedRooms...), nil
}

func (s *fakeRoomMessageBatchSendStore) RoomMessageBatchSendValidateEmployees(_ context.Context, _ int, _ []int) (bool, error) {
	return s.employeesValid, nil
}

func (s *fakeRoomMessageBatchSendStore) CreateRoomMessageBatchSend(_ context.Context, values RoomMessageBatchSendWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.batches == nil {
		s.batches = map[int]RoomMessageBatchSendItem{}
	}
	s.batches[s.createdID] = RoomMessageBatchSendItem{
		ID:           s.createdID,
		CorpID:       values.CorpID,
		UserID:       values.UserID,
		UserName:     values.UserName,
		EmployeeIDs:  values.EmployeeIDs,
		BatchTitle:   values.BatchTitle,
		Content:      values.Content,
		SendWay:      values.SendWay,
		DefiniteTime: values.DefiniteTime,
		CreatedAt:    "2026-07-04 10:00:00",
	}
	return s.createdID, nil
}

func (s *fakeRoomMessageBatchSendStore) CreateRoomMessageBatchSendTasks(_ context.Context, _ int) ([]RoomMessageBatchSendTarget, error) {
	return append([]RoomMessageBatchSendTarget{}, s.sendTargets...), nil
}

func (s *fakeRoomMessageBatchSendStore) MarkRoomMessageBatchSendSubmitted(_ context.Context, _ int, results []RoomMessageBatchSendMessageResult) error {
	s.markedResults = append([]RoomMessageBatchSendMessageResult{}, results...)
	return nil
}

func (s *fakeRoomMessageBatchSendStore) RoomMessageBatchSendOwnerPage(_ context.Context, filter RoomMessageBatchSendOwnerFilter) (RoomMessageBatchSendOwnerPage, error) {
	s.ownerFilter = filter
	return s.ownerPage, nil
}

func (s *fakeRoomMessageBatchSendStore) RoomMessageBatchSendRoomPage(_ context.Context, filter RoomMessageBatchSendRoomFilter) (RoomMessageBatchSendRoomPage, error) {
	s.roomFilter = filter
	return s.roomPage, nil
}

func (s *fakeRoomMessageBatchSendStore) DeleteRoomMessageBatchSend(_ context.Context, batchID int) (bool, error) {
	s.deletedID = batchID
	return s.deleteFound, nil
}

func (s *fakeRoomMessageBatchSendStore) RoomMessageBatchSendEmployees(_ context.Context, _ int) ([]RoomMessageBatchSendEmployeeRef, error) {
	return append([]RoomMessageBatchSendEmployeeRef{}, s.employees...), nil
}

func (s *fakeRoomMessageBatchSendStore) RoomMessageBatchSendEmployeeWXUserID(_ context.Context, employeeID int) (string, bool, error) {
	return "wx-user-" + strconv.Itoa(employeeID), true, nil
}

func (s *fakeRoomMessageBatchSendStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	status := s.quota
	status.TenantID = tenantID
	status.Metric = metric
	status.Additional = additional
	return status, nil
}

func (s *fakeRoomMessageBatchSendStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}

type fakeRoomMessageBatchSendClient struct {
	mediaID       string
	msgID         string
	uploadPath    string
	uploadPaths   []string
	submits       []RoomMessageBatchSendMessagePayload
	agentMessages []fakeRoomMessageBatchSendAgentMessage
}

type fakeRoomMessageBatchSendAgentMessage struct {
	ToUser  string
	Content string
}

func (c *fakeRoomMessageBatchSendClient) UploadTemporaryImage(_ context.Context, _ RoomWelcomeCorpCredential, filePath string) (string, error) {
	c.uploadPath = filePath
	c.uploadPaths = append(c.uploadPaths, filePath)
	return c.mediaID, nil
}

func (c *fakeRoomMessageBatchSendClient) SubmitRoomMessageBatchSend(_ context.Context, _ RoomWelcomeCorpCredential, payload RoomMessageBatchSendMessagePayload) (RoomMessageBatchSendMessageResult, error) {
	c.submits = append(c.submits, payload)
	return RoomMessageBatchSendMessageResult{ErrCode: 0, ErrMsg: "ok", MsgID: c.msgID}, nil
}

func (c *fakeRoomMessageBatchSendClient) SendAgentTextMessage(_ context.Context, _ RoomTagPullAgentCredential, toUser string, content string) error {
	c.agentMessages = append(c.agentMessages, fakeRoomMessageBatchSendAgentMessage{ToUser: toUser, Content: content})
	return nil
}
