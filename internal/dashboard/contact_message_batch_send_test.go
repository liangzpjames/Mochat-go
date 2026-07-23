package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestContactMessageBatchSendIndexAndShow(t *testing.T) {
	store := &fakeContactMessageBatchSendStore{
		users: map[int]User{1: {ID: 1, Name: "管理员"}},
		page: ContactMessageBatchSendPage{
			PerPage:   10,
			Total:     1,
			TotalPage: 1,
			Items: []ContactMessageBatchSendItem{{
				ID: 800001, UserID: 1, UserName: "管理员", SendWay: 1, SendStatus: 1, CreatedAt: "2026-07-04 10:00:00",
				Content: []ContactMessageBatchSendContent{{MsgType: "image", PicURL: "image/a.jpg"}},
			}},
		},
		batches: map[int]ContactMessageBatchSendItem{
			800001: {
				ID: 800001, UserID: 1, UserName: "管理员", CreatedAt: "2026-07-04 10:00:00",
				Content:      []ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}},
				FilterParams: ContactMessageBatchSendFilterParams{Rooms: []int{11}},
				FilterParamsDetail: ContactMessageBatchSendFilterDetail{
					Rooms: []ContactMessageBatchSendNameID{{ID: 11, Name: "客户群A"}},
					Tags:  []ContactMessageBatchSendNameID{},
				},
			},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewContactMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "http://api.example.com", t.TempDir(), nil)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/contactMessageBatchSend/index?page=1&perPage=10", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("index status=%d body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/contactMessageBatchSend/index#get" || store.filter.UserID != 1 {
		t.Fatalf("auth=%#v filter=%#v", authorizer, store.filter)
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	item := data["list"].([]any)[0].(map[string]any)
	content := item["content"].([]any)[0].(map[string]any)
	if content["pic_url"] != "http://api.example.com/static/image/a.jpg" {
		t.Fatalf("content=%#v", content)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/contactMessageBatchSend/show?batchId=800001", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.Show(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("show status=%d body=%s", rec.Code, rec.Body.String())
	}
	show := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	if show["creator"] != "管理员" || show["filterParamsDetail"].(map[string]any)["rooms"].([]any)[0].(map[string]any)["name"] != "客户群A" {
		t.Fatalf("show=%#v", show)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/contactMessageBatchSend/messageShow?batchId=800001", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.MessageShow(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("messageShow status=%d body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/contactMessageBatchSend/show#get" {
		t.Fatalf("messageShow permission=%q", authorizer.permissionKey)
	}
	preview := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	message := preview["message"].([]any)[0].(map[string]any)
	if message["msgType"] != "text" || message["content"] != "hello" {
		t.Fatalf("messageShow=%#v", preview)
	}
}

func TestContactMessageBatchSendStoreCreatesTasksAndSubmitsWeCom(t *testing.T) {
	store := &fakeContactMessageBatchSendStore{
		users:           map[int]User{1: {ID: 1, Name: "管理员"}},
		batches:         map[int]ContactMessageBatchSendItem{},
		createdID:       800002,
		credentialFound: true,
		credential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wx-corp", ContactSecret: "secret"},
		filterDetail: ContactMessageBatchSendFilterDetail{
			Rooms: []ContactMessageBatchSendNameID{{ID: 11, Name: "客户群A"}},
			Tags:  []ContactMessageBatchSendNameID{{ID: 31, Name: "标签A"}},
		},
		sendTargets: []ContactMessageBatchSendSendTarget{{
			EmployeeID:      21,
			WXUserID:        "wx-user-21",
			ExternalUserIDs: []string{"external-1", "external-2"},
		}},
	}
	client := &fakeContactMessageBatchSendClient{mediaID: "media-image", msgID: "msg-800002"}
	handler := NewContactMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "/tmp/mochat-go-test", client)

	body := `{"employeeIds":[21],"filterParams":{"gender":1,"rooms":[11],"tags":[31],"excludeContacts":[41],"addTimeStart":"2026-07-01","addTimeEnd":"2026-07-02"},"content":[{"msgType":"text","content":"hello"},{"msgType":"image","pic_url":"image/a.jpg"}],"sendWay":1}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/contactMessageBatchSend/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.UserID != 1 || len(store.created.EmployeeIDs) != 1 || store.created.FilterParams.Gender == nil || *store.created.FilterParams.Gender != 1 {
		t.Fatalf("created=%#v", store.created)
	}
	if client.uploadPath != "/tmp/mochat-go-test/image/a.jpg" || len(client.uploadPaths) != 1 || len(client.submits) != 1 {
		t.Fatalf("uploads=%#v submits=%#v", client.uploadPaths, client.submits)
	}
	submit := client.submits[0]
	if submit.Sender != "wx-user-21" || submit.ExternalUserID[0] != "external-1" || submit.Content[1].MediaID != "media-image" {
		t.Fatalf("submit=%#v", submit)
	}
	if len(store.markedResults) != 1 || store.markedResults[0].EmployeeID != 21 || store.markedResults[0].MsgID != "msg-800002" {
		t.Fatalf("marked=%#v", store.markedResults)
	}
}

func TestContactMessageBatchSendStoreUploadsMiniProgramCoverBeforeSubmit(t *testing.T) {
	store := &fakeContactMessageBatchSendStore{
		users:           map[int]User{1: {ID: 1, Name: "管理员"}},
		batches:         map[int]ContactMessageBatchSendItem{},
		createdID:       800004,
		credentialFound: true,
		credential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wx-corp", ContactSecret: "secret"},
		sendTargets: []ContactMessageBatchSendSendTarget{{
			EmployeeID:      21,
			WXUserID:        "wx-user-21",
			ExternalUserIDs: []string{"external-1"},
		}},
	}
	client := &fakeContactMessageBatchSendClient{mediaID: "media-mini-cover", msgID: "msg-800004"}
	handler := NewContactMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "/tmp/mochat-go-test", client)

	body := `{"employeeIds":[21],"filterParams":{},"content":[{"msgType":"miniprogram","title":"小程序","pic_media_id":"image/mini.jpg","appid":"wx123","page":"pages/index"}],"sendWay":1}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/contactMessageBatchSend/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(client.uploadPaths) != 1 || client.uploadPaths[0] != "/tmp/mochat-go-test/image/mini.jpg" {
		t.Fatalf("upload paths=%#v", client.uploadPaths)
	}
	if len(client.submits) != 1 || client.submits[0].Content[0].PicMediaID != "media-mini-cover" {
		t.Fatalf("submits=%#v", client.submits)
	}
	if store.created.Content[0].PicURL != "image/mini.jpg" {
		t.Fatalf("created content=%#v", store.created.Content[0])
	}
}

func TestContactMessageBatchSendStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeContactMessageBatchSendStore{
		users: map[int]User{1: {ID: 1, Name: "管理员", TenantID: 8}},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricContactMessageBatches,
			Current:    3,
			Limit:      3,
			Additional: 1,
		},
	}
	handler := NewContactMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", "/tmp/mochat-go-test", nil)

	body := `{"employeeIds":[21],"filterParams":{},"content":[{"msgType":"text","content":"hello"}],"sendWay":2,"definiteTime":"2026-07-04 10:00:00"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/contactMessageBatchSend/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.quotaMetric != SaaSMetricContactMessageBatches || store.quotaTenantID != 8 || store.createCalls != 0 || store.refreshMetric != "" {
		t.Fatalf("quota metric=%q tenant=%d createCalls=%d refresh=%q", store.quotaMetric, store.quotaTenantID, store.createCalls, store.refreshMetric)
	}
}

func TestContactMessageBatchSendDetailListsRemindAndDestroy(t *testing.T) {
	store := &fakeContactMessageBatchSendStore{
		users: map[int]User{1: {ID: 1}},
		batches: map[int]ContactMessageBatchSendItem{
			800003: {ID: 800003, CorpID: 7, UserID: 1, CreatedAt: "2026-07-04 10:00:00"},
		},
		employeePage: ContactMessageBatchSendEmployeePage{
			PerPage: 15, Total: 1, TotalPage: 1,
			Items: []ContactMessageBatchSendEmployeeItem{{ID: 501, EmployeeID: 21, EmployeeName: "员工A", EmployeeAvatar: "avatar/a.jpg"}},
		},
		receivePage: ContactMessageBatchSendReceivePage{
			PerPage: 15, Total: 1, TotalPage: 1,
			Items: []ContactMessageBatchSendReceiveItem{{ID: 601, ContactID: 31, ContactName: "客户A", ContactAvatar: "avatar/c.jpg", SendTime: 1783159200}},
		},
		employees:        []ContactMessageBatchSendEmployeeRef{{ID: 21, WXUserID: "wx-user-21"}},
		remindAgent:      RoomTagPullAgentCredential{CorpID: 7, WXCorpID: "wx-corp", WXAgentID: "1000001", WXSecret: "agent-secret"},
		remindAgentFound: true,
		deleteFound:      true,
	}
	client := &fakeContactMessageBatchSendClient{}
	handler := NewContactMessageBatchSendHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", t.TempDir(), client)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/contactMessageBatchSend/employeeSendIndex?batchId=800003&sendStatus=0&keyWords=%E5%91%98%E5%B7%A5", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.EmployeeSendIndex(rec, req)
	if rec.Code != http.StatusOK || store.employeeFilter.BatchID != 800003 || store.employeeFilter.SendStatus == nil || *store.employeeFilter.SendStatus != 0 {
		t.Fatalf("employee status=%d filter=%#v body=%s", rec.Code, store.employeeFilter, rec.Body.String())
	}
	employee := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if employee["employeeAvatar"] != "http://api.example.com/static/avatar/a.jpg" {
		t.Fatalf("employee=%#v", employee)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/contactMessageBatchSend/contactReceiveIndex?batchId=800003", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.ContactReceiveIndex(rec, req)
	if rec.Code != http.StatusOK || store.receiveFilter.BatchID != 800003 {
		t.Fatalf("receive status=%d filter=%#v body=%s", rec.Code, store.receiveFilter, rec.Body.String())
	}
	contact := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if contact["contactAvatar"] != "http://api.example.com/static/avatar/c.jpg" || contact["sendTime"] == "" {
		t.Fatalf("contact=%#v", contact)
	}

	req = httptest.NewRequest(http.MethodPost, "/dashboard/contactMessageBatchSend/remind", strings.NewReader(`{"batchId":800003}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.Remind(rec, req)
	if rec.Code != http.StatusOK || len(client.agentMessages) != 1 || !strings.Contains(client.agentMessages[0].Content, "客户群发任务") {
		t.Fatalf("remind status=%d messages=%#v body=%s", rec.Code, client.agentMessages, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/dashboard/contactMessageBatchSend/destroy", strings.NewReader(`{"batchId":800003}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.Destroy(rec, req)
	if rec.Code != http.StatusOK || store.deletedID != 800003 {
		t.Fatalf("destroy status=%d deleted=%d body=%s", rec.Code, store.deletedID, rec.Body.String())
	}
}

type fakeContactMessageBatchSendStore struct {
	users            map[int]User
	page             ContactMessageBatchSendPage
	filter           ContactMessageBatchSendFilter
	batches          map[int]ContactMessageBatchSendItem
	created          ContactMessageBatchSendWrite
	createdID        int
	createCalls      int
	credential       RoomWelcomeCorpCredential
	credentialFound  bool
	filterDetail     ContactMessageBatchSendFilterDetail
	sendTargets      []ContactMessageBatchSendSendTarget
	markedResults    []ContactMessageBatchSendMessageResult
	employeePage     ContactMessageBatchSendEmployeePage
	employeeFilter   ContactMessageBatchSendEmployeeFilter
	receivePage      ContactMessageBatchSendReceivePage
	receiveFilter    ContactMessageBatchSendReceiveFilter
	roomInfo         ContactMessageBatchSendRoomInfo
	roomFound        bool
	employees        []ContactMessageBatchSendEmployeeRef
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

func (s *fakeContactMessageBatchSendStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeContactMessageBatchSendStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 99, nil
}

func (s *fakeContactMessageBatchSendStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeContactMessageBatchSendStore) RoomWelcomeCorpCredentialByID(_ context.Context, _ int) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, s.credentialFound, nil
}

func (s *fakeContactMessageBatchSendStore) RoomTagPullRemindAgentByCorpID(_ context.Context, _ int) (RoomTagPullAgentCredential, bool, error) {
	return s.remindAgent, s.remindAgentFound, nil
}

func (s *fakeContactMessageBatchSendStore) ContactMessageBatchSendPage(_ context.Context, filter ContactMessageBatchSendFilter) (ContactMessageBatchSendPage, error) {
	s.filter = filter
	return s.page, nil
}

func (s *fakeContactMessageBatchSendStore) ContactMessageBatchSendByID(_ context.Context, batchID int) (ContactMessageBatchSendItem, bool, error) {
	item, ok := s.batches[batchID]
	return item, ok, nil
}

func (s *fakeContactMessageBatchSendStore) CreateContactMessageBatchSend(_ context.Context, values ContactMessageBatchSendWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.batches == nil {
		s.batches = map[int]ContactMessageBatchSendItem{}
	}
	s.batches[s.createdID] = ContactMessageBatchSendItem{
		ID:           s.createdID,
		CorpID:       values.CorpID,
		UserID:       values.UserID,
		UserName:     values.UserName,
		EmployeeIDs:  values.EmployeeIDs,
		FilterParams: values.FilterParams,
		Content:      values.Content,
		SendWay:      values.SendWay,
		CreatedAt:    "2026-07-04 10:00:00",
	}
	return s.createdID, nil
}

func (s *fakeContactMessageBatchSendStore) CreateContactMessageBatchSendTasks(_ context.Context, _ int) ([]ContactMessageBatchSendSendTarget, error) {
	return append([]ContactMessageBatchSendSendTarget{}, s.sendTargets...), nil
}

func (s *fakeContactMessageBatchSendStore) MarkContactMessageBatchSendSubmitted(_ context.Context, _ int, results []ContactMessageBatchSendMessageResult) error {
	s.markedResults = append([]ContactMessageBatchSendMessageResult{}, results...)
	return nil
}

func (s *fakeContactMessageBatchSendStore) ContactMessageBatchSendEmployeePage(_ context.Context, filter ContactMessageBatchSendEmployeeFilter) (ContactMessageBatchSendEmployeePage, error) {
	s.employeeFilter = filter
	return s.employeePage, nil
}

func (s *fakeContactMessageBatchSendStore) ContactMessageBatchSendReceivePage(_ context.Context, filter ContactMessageBatchSendReceiveFilter) (ContactMessageBatchSendReceivePage, error) {
	s.receiveFilter = filter
	return s.receivePage, nil
}

func (s *fakeContactMessageBatchSendStore) DeleteContactMessageBatchSend(_ context.Context, batchID int) (bool, error) {
	s.deletedID = batchID
	return s.deleteFound, nil
}

func (s *fakeContactMessageBatchSendStore) ContactMessageBatchSendRoomInfo(_ context.Context, _ int) (ContactMessageBatchSendRoomInfo, bool, error) {
	return s.roomInfo, s.roomFound, nil
}

func (s *fakeContactMessageBatchSendStore) ContactMessageBatchSendEmployees(_ context.Context, _ int, _ int) ([]ContactMessageBatchSendEmployeeRef, error) {
	return append([]ContactMessageBatchSendEmployeeRef{}, s.employees...), nil
}

func (s *fakeContactMessageBatchSendStore) ContactMessageBatchSendEmployeeWXUserID(_ context.Context, employeeID int) (string, bool, error) {
	return "wx-user-" + strconv.Itoa(employeeID), true, nil
}

func (s *fakeContactMessageBatchSendStore) ContactMessageBatchSendFilterDetail(_ context.Context, _ ContactMessageBatchSendFilterParams) (ContactMessageBatchSendFilterDetail, error) {
	return s.filterDetail, nil
}

func (s *fakeContactMessageBatchSendStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	status := s.quota
	status.TenantID = tenantID
	status.Metric = metric
	status.Additional = additional
	return status, nil
}

func (s *fakeContactMessageBatchSendStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}

type fakeContactMessageBatchSendClient struct {
	mediaID       string
	msgID         string
	uploadPath    string
	uploadPaths   []string
	submits       []ContactMessageBatchSendMessagePayload
	agentMessages []fakeContactMessageBatchSendAgentMessage
}

type fakeContactMessageBatchSendAgentMessage struct {
	ToUser  string
	Content string
}

func (c *fakeContactMessageBatchSendClient) UploadTemporaryImage(_ context.Context, _ RoomWelcomeCorpCredential, filePath string) (string, error) {
	c.uploadPath = filePath
	c.uploadPaths = append(c.uploadPaths, filePath)
	return c.mediaID, nil
}

func (c *fakeContactMessageBatchSendClient) SubmitContactMessageBatchSend(_ context.Context, _ RoomWelcomeCorpCredential, payload ContactMessageBatchSendMessagePayload) (ContactMessageBatchSendMessageResult, error) {
	c.submits = append(c.submits, payload)
	return ContactMessageBatchSendMessageResult{ErrCode: 0, ErrMsg: "ok", MsgID: c.msgID}, nil
}

func (c *fakeContactMessageBatchSendClient) SendAgentTextMessage(_ context.Context, _ RoomTagPullAgentCredential, toUser string, content string) error {
	c.agentMessages = append(c.agentMessages, fakeContactMessageBatchSendAgentMessage{ToUser: toUser, Content: content})
	return nil
}
