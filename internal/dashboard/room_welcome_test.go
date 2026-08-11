package dashboard

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomWelcomeIndexFiltersCreatorAndReturnsFullPicture(t *testing.T) {
	store := &fakeRoomWelcomeStore{
		users: map[int]User{1: {ID: 1, IsSuperAdmin: 0}},
		page: RoomWelcomePage{
			Items: []RoomWelcomeItem{{
				ID:           11,
				CorpID:       7,
				MsgText:      "欢迎",
				ComplexType:  "image",
				MsgComplex:   `{"pic":"image/roomWelcome/a.jpg","pic_url":"https://wx.example.com/a.jpg"}`,
				CreateUserID: 1,
				CreatedAt:    "2026-07-03 10:00:00",
			}},
			Total:     1,
			TotalPage: 1,
			PerPage:   10,
		},
		creators: map[int]string{1: "管理员"},
	}
	handler := NewRoomWelcomeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, "http://api.example.com", t.TempDir(), nil)

	rec := performAuthenticatedDashboardRequest(handler.Index, "GET", "/dashboard/roomWelcome/index?text=%E6%AC%A2", nil, map[string]string{"X-Mochat-Go-User-ID": "1"})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !store.lastFilter.RestrictCreateUserID || store.lastFilter.CreateUserID != 1 || store.lastFilter.Text != "欢" {
		t.Fatalf("filter = %+v", store.lastFilter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	list := data["list"].([]any)
	item := list[0].(map[string]any)
	if item["create_user"] != "管理员" {
		t.Fatalf("create_user = %v", item["create_user"])
	}
	if !strings.Contains(item["msg_complex"].(string), "http://api.example.com/static/image/roomWelcome/a.jpg") {
		t.Fatalf("msg_complex = %v", item["msg_complex"])
	}
}

func TestRoomWelcomeStoreCreatesWeComTemplateAndLocalRecord(t *testing.T) {
	store := &fakeRoomWelcomeStore{
		users:      map[int]User{1: {ID: 1, Name: "管理员", IsSuperAdmin: 1}},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
	}
	client := &fakeRoomWelcomeTemplateClient{templateID: "tpl_123"}
	handler := NewRoomWelcomeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", t.TempDir(), client)

	body := `{"msg_text":"你好[用户昵称]","notice":1,"msg_complex":{"type":"link","link":{"title":"官网","desc":"介绍","url":"https://example.com"}}}`
	rec := performAuthenticatedDashboardRequest(handler.Store, "POST", "/dashboard/roomWelcome/store", strings.NewReader(body), map[string]string{
		"Content-Type":        "application/json",
		"X-Mochat-Go-User-ID": "1",
	})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if client.created.TextContent != "你好%NICKNAME%" || client.created.Notify != 1 || client.created.Link == nil || client.created.Link.URL != "https://example.com" {
		t.Fatalf("created payload = %+v", client.created)
	}
	if store.created.ComplexTemplateID != "tpl_123" || store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.ComplexType != "link" {
		t.Fatalf("created record = %+v", store.created)
	}
	if !strings.Contains(store.created.MsgComplex, "https://example.com") {
		t.Fatalf("msg complex = %s", store.created.MsgComplex)
	}
}

func TestRoomWelcomeDestroyDeletesWeComTemplateThenLocalRecord(t *testing.T) {
	store := &fakeRoomWelcomeStore{
		users:      map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		item:       RoomWelcomeItem{ID: 11, CorpID: 7, ComplexTemplateID: "tpl_123"},
		itemFound:  true,
	}
	client := &fakeRoomWelcomeTemplateClient{}
	handler := NewRoomWelcomeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com", t.TempDir(), client)

	rec := performAuthenticatedDashboardRequest(handler.Destroy, "DELETE", "/dashboard/roomWelcome/destroy", strings.NewReader(`{"id":11}`), map[string]string{
		"Content-Type":        "application/json",
		"X-Mochat-Go-User-ID": "1",
	})
	if rec.Code != 200 {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if client.deletedTemplateID != "tpl_123" || store.deletedID != 11 {
		t.Fatalf("deleted template=%q local=%d", client.deletedTemplateID, store.deletedID)
	}
}

func performAuthenticatedDashboardRequest(handler http.HandlerFunc, method string, target string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	req := authenticatedDashboardRequestForTest(method, target, body)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

type fakeRoomWelcomeStore struct {
	users        map[int]User
	page         RoomWelcomePage
	item         RoomWelcomeItem
	itemFound    bool
	creators     map[int]string
	credential   RoomWelcomeCorpCredential
	lastFilter   RoomWelcomeFilter
	created      RoomWelcomeWrite
	updated      RoomWelcomeWrite
	updatedID    int
	deletedID    int
	createdID    int
	credentialOK bool
}

func (s *fakeRoomWelcomeStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeRoomWelcomeStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 99, nil
}

func (s *fakeRoomWelcomeStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeRoomWelcomeStore) RoomWelcomePage(_ context.Context, filter RoomWelcomeFilter) (RoomWelcomePage, error) {
	s.lastFilter = filter
	if s.page.PerPage == 0 {
		s.page.PerPage = filter.PerPage
	}
	return s.page, nil
}

func (s *fakeRoomWelcomeStore) RoomWelcomeByID(_ context.Context, _ int) (RoomWelcomeItem, bool, error) {
	return s.item, s.itemFound, nil
}

func (s *fakeRoomWelcomeStore) RoomWelcomeCreatorNamesByIDs(_ context.Context, _ []int) (map[int]string, error) {
	return s.creators, nil
}

func (s *fakeRoomWelcomeStore) RoomWelcomeCorpCredentialByID(_ context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error) {
	if s.credential.CorpID == 0 {
		s.credential.CorpID = corpID
	}
	if s.credential.WXCorpID == "" {
		return s.credential, s.credentialOK, nil
	}
	return s.credential, true, nil
}

func (s *fakeRoomWelcomeStore) CreateRoomWelcome(_ context.Context, values RoomWelcomeWrite) (int, error) {
	s.created = values
	s.createdID = 101
	return s.createdID, nil
}

func (s *fakeRoomWelcomeStore) UpdateRoomWelcome(_ context.Context, roomWelcomeID int, values RoomWelcomeWrite) (bool, error) {
	s.updatedID = roomWelcomeID
	s.updated = values
	return true, nil
}

func (s *fakeRoomWelcomeStore) DeleteRoomWelcome(_ context.Context, roomWelcomeID int) (bool, error) {
	s.deletedID = roomWelcomeID
	return true, nil
}

type fakeRoomWelcomeTemplateClient struct {
	templateID        string
	created           RoomWelcomeTemplatePayload
	updated           RoomWelcomeTemplatePayload
	updatedTemplateID string
	deletedTemplateID string
}

func (c *fakeRoomWelcomeTemplateClient) UploadImage(_ context.Context, _ RoomWelcomeCorpCredential, _ string) (string, error) {
	return "https://wx.example.com/image.jpg", nil
}

func (c *fakeRoomWelcomeTemplateClient) UploadTemporaryImage(_ context.Context, _ RoomWelcomeCorpCredential, _ string) (string, error) {
	return "media_123", nil
}

func (c *fakeRoomWelcomeTemplateClient) CreateGroupWelcomeTemplate(_ context.Context, _ RoomWelcomeCorpCredential, payload RoomWelcomeTemplatePayload) (string, error) {
	c.created = payload
	if c.templateID == "" {
		c.templateID = "tpl"
	}
	return c.templateID, nil
}

func (c *fakeRoomWelcomeTemplateClient) UpdateGroupWelcomeTemplate(_ context.Context, _ RoomWelcomeCorpCredential, templateID string, payload RoomWelcomeTemplatePayload) error {
	c.updatedTemplateID = templateID
	c.updated = payload
	return nil
}

func (c *fakeRoomWelcomeTemplateClient) DeleteGroupWelcomeTemplate(_ context.Context, _ RoomWelcomeCorpCredential, templateID string) error {
	c.deletedTemplateID = templateID
	return nil
}
