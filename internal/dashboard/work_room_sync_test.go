package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkRoomSyncPullsGroupChatsAndStoresWithRBAC(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                      map[int]User{1: {ID: 1}},
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		roomWelcomeCredentialFound: true,
	}
	client := &fakeWorkRoomSyncClient{
		chats: []WorkRoomSyncGroupChat{
			{WXChatID: "chat-1", Status: 0},
			{WXChatID: "chat-2", Status: 1},
			{WXChatID: "chat-1", Status: 0},
		},
		details: map[string]WorkRoomSyncRoom{
			"chat-1": {
				WXChatID:   "chat-1",
				Name:       "客户群一",
				Owner:      "zhangsan",
				Status:     0,
				CreateTime: 1783300000,
				Members: []WorkRoomSyncMember{{
					WXUserID:  "zhangsan",
					Type:      1,
					JoinTime:  1783300100,
					JoinScene: 1,
				}},
			},
			"chat-2": {
				WXChatID: "chat-2",
				Name:     "客户群二",
				Owner:    "lisi",
				Status:   1,
			},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", authorizer).
		WithWorkRoomSyncClient(client)

	req := authenticatedDashboardRequestForTestAs(http.MethodPut, "/dashboard/workRoom/syn", nil, 1, 1, 7, 88)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomSync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workRoom/syn#put" || authorizer.corpID != 7 || authorizer.workEmployeeID != 88 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if client.credential.WXCorpID != "ww-go" || client.credential.ContactSecret != "contact-secret" {
		t.Fatalf("credential = %#v", client.credential)
	}
	if len(client.detailIDs) != 2 || client.detailIDs[0] != "chat-1" || client.detailIDs[1] != "chat-2" {
		t.Fatalf("detail ids = %#v", client.detailIDs)
	}
	if store.lastWorkRoomSyncCorpID != 7 || len(store.lastWorkRoomSyncRooms) != 2 {
		t.Fatalf("sync call = corp %d rooms %#v", store.lastWorkRoomSyncCorpID, store.lastWorkRoomSyncRooms)
	}
	if first := store.lastWorkRoomSyncRooms[0]; first.WXChatID != "chat-1" || len(first.Members) != 1 || first.Members[0].WXUserID != "zhangsan" {
		t.Fatalf("first room = %#v", first)
	}
	if second := store.lastWorkRoomSyncRooms[1]; second.WXChatID != "chat-2" || second.Status != 1 {
		t.Fatalf("second room = %#v", second)
	}
}

func TestWorkRoomSyncReturnsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                      map[int]User{1: {ID: 1, TenantID: 9}},
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		roomWelcomeCredentialFound: true,
		workRoomSyncErr: NewSaaSQuotaExceededError(SaaSQuotaStatus{
			Metric:     SaaSMetricRooms,
			TenantID:   9,
			Current:    5,
			Limit:      5,
			Additional: 1,
		}),
	}
	client := &fakeWorkRoomSyncClient{
		chats: []WorkRoomSyncGroupChat{{WXChatID: "chat-1", Status: 0}},
		details: map[string]WorkRoomSyncRoom{
			"chat-1": {WXChatID: "chat-1", Name: "客户群一"},
		},
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", &recordingAuthorizer{}).
		WithWorkRoomSyncClient(client)

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workRoom/syn", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomSync(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "套餐额度已达上限：客户群数 5/5" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestWorkRoomSyncRequiresDashboardPrincipal(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88,8-99"), HeaderUserIDResolver{}, "", &recordingAuthorizer{}).
		WithWorkRoomSyncClient(&fakeWorkRoomSyncClient{})

	req := httptest.NewRequest(http.MethodPut, "/dashboard/workRoom/syn", nil)
	rec := httptest.NewRecorder()
	handler.WorkRoomSync(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkRoomSyncRequiresCorpCredential(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", &recordingAuthorizer{}).
		WithWorkRoomSyncClient(&fakeWorkRoomSyncClient{})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workRoom/syn", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomSync(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "企业授权信息错误" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

type fakeWorkRoomSyncClient struct {
	chats      []WorkRoomSyncGroupChat
	details    map[string]WorkRoomSyncRoom
	credential RoomWelcomeCorpCredential
	detailIDs  []string
}

func (c *fakeWorkRoomSyncClient) GroupChats(_ context.Context, credential RoomWelcomeCorpCredential) ([]WorkRoomSyncGroupChat, error) {
	c.credential = credential
	return append([]WorkRoomSyncGroupChat{}, c.chats...), nil
}

func (c *fakeWorkRoomSyncClient) GroupChatDetail(_ context.Context, credential RoomWelcomeCorpCredential, wxChatID string) (WorkRoomSyncRoom, error) {
	c.credential = credential
	c.detailIDs = append(c.detailIDs, wxChatID)
	return c.details[wxChatID], nil
}

var _ WorkRoomSyncClient = (*fakeWorkRoomSyncClient)(nil)
