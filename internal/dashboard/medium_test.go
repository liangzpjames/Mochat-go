package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMediumIndexReturnsPagedItems(t *testing.T) {
	groupID := 11
	store := &fakeMediumStore{
		users: map[int]User{1: {ID: 1, Name: "管理员"}},
		page: MediumPage{
			Items: []MediumItem{
				{ID: 21, Type: 2, MediaID: "media-21", Content: map[string]any{"imagePath": "image/a.png"}, CorpID: 7, MediumGroupID: 11, MediumGroupName: "图片素材", UserID: 1, UserName: "管理员", CreatedAt: "2026-07-03 10:00:00"},
			},
			Total: 1, TotalPage: 1, PerPage: 10,
		},
	}
	handler := NewMediumHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/medium/index?mediumGroupId=11&type=2&searchStr=image", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastFilter.CorpID != 7 || store.lastFilter.Type != 2 || store.lastFilter.Search != "image" || store.lastFilter.MediumGroupID == nil || *store.lastFilter.MediumGroupID != groupID {
		t.Fatalf("filter = %#v", store.lastFilter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	list := body["data"].(map[string]any)["list"].([]any)
	item := list[0].(map[string]any)
	content := item["content"].(map[string]any)
	if item["type"] != "图片" || content["imageFullPath"] != "http://api.example.com/static/image/a.png" {
		t.Fatalf("item = %#v", item)
	}
}

func TestMediumIndexAppliesPersonalScopeFromSession(t *testing.T) {
	store := &fakeMediumStore{users: map[int]User{1: {ID: 1, Name: "管理员"}}, page: MediumPage{PerPage: 10}}
	handler := NewMediumHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/medium/index?scopeType=personal&scopeId=999", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastFilter.ScopeType != "personal" || store.lastFilter.ScopeID != 1 {
		t.Fatalf("scope filter = %#v", store.lastFilter)
	}
}

func TestMediumIndexAppliesSidebarVisibilityQuery(t *testing.T) {
	store := &fakeMediumStore{users: map[int]User{1: {ID: 1, Name: "管理员"}}, page: MediumPage{PerPage: 10}}
	handler := NewMediumHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/medium/index?scopeType=public&sidebarVisible=1", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK || store.lastFilter.SidebarVisible == nil || !*store.lastFilter.SidebarVisible {
		t.Fatalf("status=%d filter=%#v body=%s", rec.Code, store.lastFilter, rec.Body.String())
	}
}

func TestMediumStoreCreatesScopedTextMedium(t *testing.T) {
	store := &fakeMediumStore{users: map[int]User{1: {ID: 1, Name: "管理员"}}, createID: 32}
	handler := NewMediumHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodPost, "/dashboard/medium/store", strings.NewReader(`{"type":1,"mediumGroupId":0,"scopeType":"personal","scopeId":999,"sidebarVisible":true,"content":{"title":"问候","content":"你好"}}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.ScopeType != "personal" || store.created.ScopeID != 1 || !store.created.SidebarVisible || store.created.Status != "available" {
		t.Fatalf("created scope = %#v", store.created)
	}
}

func TestMediumStoreCreatesTextMedium(t *testing.T) {
	store := &fakeMediumStore{users: map[int]User{1: {ID: 1, Name: "管理员"}}, createID: 31}
	handler := NewMediumHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodPost, "/dashboard/medium/store", strings.NewReader(`{"type":1,"mediumGroupId":11,"content":{"title":"问候","content":"你好"}}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.Type != 1 || store.created.IsSync != 1 || store.created.MediumGroupID != 11 || store.created.UserID != 1 || store.created.UserName != "管理员" || !strings.Contains(store.created.Content, "你好") {
		t.Fatalf("created = %#v", store.created)
	}
	body := decodeBody(t, rec.Body.Bytes())
	if int(body["data"].(map[string]any)["id"].(float64)) != 31 {
		t.Fatalf("body = %#v", body)
	}
}

func TestMediumGroupUpdateMovesMedium(t *testing.T) {
	store := &fakeMediumStore{users: map[int]User{1: {ID: 1, Name: "管理员"}}}
	handler := NewMediumHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodPut, "/dashboard/medium/groupUpdate", strings.NewReader(`{"id":21,"mediumGroupId":0}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.GroupUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.movedCorpID != 7 || store.movedMediumID != 21 || store.movedGroupID != 0 {
		t.Fatalf("moved = corp %d medium %d group %d", store.movedCorpID, store.movedMediumID, store.movedGroupID)
	}
}

func TestSidebarMediumIndexFiltersUngrouped(t *testing.T) {
	zero := 0
	store := &fakeMediumStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		page:             MediumPage{PerPage: 10},
	}
	handler := NewMediumHandler(store, nil, HeaderUserIDResolver{}, nil, "http://api.example.com").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := httptest.NewRequest(http.MethodGet, "/sidebar/medium/index?mediumGroupId=0", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastSidebarEmployeeID != 5 || store.lastFilter.CorpID != 7 || store.lastFilter.MediumGroupID == nil || *store.lastFilter.MediumGroupID != zero {
		t.Fatalf("sidebar filter = employee %d filter %#v", store.lastSidebarEmployeeID, store.lastFilter)
	}
}

func TestSidebarMediumMediaIDUpdateUploadsExpiredMedium(t *testing.T) {
	root := t.TempDir()
	imagePath := filepath.Join(root, "image")
	if err := os.MkdirAll(imagePath, 0o755); err != nil {
		t.Fatal(err)
	}
	localFile := filepath.Join(imagePath, "a.png")
	if err := os.WriteFile(localFile, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := &fakeMediumStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		mediumMedia:      MediumMediaUpdateItem{ID: 21, MediaID: "old-media-id", Type: 2, Content: map[string]any{"imagePath": "image/a.png"}},
		mediumMediaFound: true,
		corpCredential:   MediumCorpCredential{CorpID: 7, WXCorpID: "wx-corp", EmployeeSecret: "employee-secret"},
		corpFound:        true,
	}
	client := &fakeMediumMediaClient{mediaID: "new-media-id"}
	handler := NewMediumHandlerWithMediaClient(store, nil, HeaderUserIDResolver{}, nil, "http://api.example.com", root, client).
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := httptest.NewRequest(http.MethodGet, "/sidebar/medium/mediaIdUpdate?mediumId=21", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.MediaIDUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if client.calls != 1 || client.lastMediaType != "image" || client.lastFilePath != localFile || client.lastCredential.EmployeeSecret != "employee-secret" {
		t.Fatalf("client = %#v", client)
	}
	if store.updatedMediaMediumID != 21 || store.updatedMediaID != "new-media-id" || store.updatedMediaLastUploadTime <= 0 {
		t.Fatalf("updated media = id %d media %q time %d", store.updatedMediaMediumID, store.updatedMediaID, store.updatedMediaLastUploadTime)
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["data"].(map[string]any)["mediaId"] != "new-media-id" {
		t.Fatalf("body = %#v", body)
	}
}

func TestSidebarMediumMediaIDUpdateKeepsFreshMedium(t *testing.T) {
	store := &fakeMediumStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		mediumMedia:      MediumMediaUpdateItem{ID: 21, MediaID: "fresh-media-id", LastUploadTime: time.Now().Unix(), Type: 2, Content: map[string]any{"imagePath": "image/a.png"}},
		mediumMediaFound: true,
		corpCredential:   MediumCorpCredential{CorpID: 7, WXCorpID: "wx-corp", EmployeeSecret: "employee-secret"},
		corpFound:        true,
	}
	client := &fakeMediumMediaClient{mediaID: "new-media-id"}
	handler := NewMediumHandlerWithMediaClient(store, nil, HeaderUserIDResolver{}, nil, "http://api.example.com", t.TempDir(), client).
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := httptest.NewRequest(http.MethodGet, "/sidebar/medium/mediaIdUpdate?mediumId=21", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.MediaIDUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if client.calls != 0 || store.updatedMediaMediumID != 0 {
		t.Fatalf("unexpected update: client calls %d medium %d", client.calls, store.updatedMediaMediumID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["data"].(map[string]any)["mediaId"] != "fresh-media-id" {
		t.Fatalf("body = %#v", body)
	}
}

type fakeMediumStore struct {
	users                      map[int]User
	sidebarEmployees           map[int]SidebarEmployee
	page                       MediumPage
	medium                     MediumItem
	mediumFound                bool
	mediumMedia                MediumMediaUpdateItem
	mediumMediaFound           bool
	corpCredential             MediumCorpCredential
	corpFound                  bool
	createID                   int
	lastFilter                 MediumFilter
	lastSidebarEmployeeID      int
	created                    MediumWrite
	updatedMediumID            int
	updated                    MediumWrite
	updatedMediaMediumID       int
	updatedMediaID             string
	updatedMediaLastUploadTime int64
	deletedCorpID              int
	deletedMediumID            int
	movedCorpID                int
	movedMediumID              int
	movedGroupID               int
}

func (s *fakeMediumStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeMediumStore) SidebarEmployeeByID(_ context.Context, employeeID int) (SidebarEmployee, bool, error) {
	s.lastSidebarEmployeeID = employeeID
	employee, ok := s.sidebarEmployees[employeeID]
	return employee, ok, nil
}

func (s *fakeMediumStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 0, nil
}

func (s *fakeMediumStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 0, 0, false, nil
}

func (s *fakeMediumStore) MediumPage(_ context.Context, filter MediumFilter) (MediumPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeMediumStore) MediumByID(_ context.Context, _ int, _ int) (MediumItem, bool, error) {
	return s.medium, s.mediumFound, nil
}

func (s *fakeMediumStore) CreateMedium(_ context.Context, values MediumWrite) (int, error) {
	s.created = values
	if s.createID == 0 {
		return 1, nil
	}
	return s.createID, nil
}

func (s *fakeMediumStore) UpdateMedium(_ context.Context, mediumID int, values MediumWrite) (bool, error) {
	s.updatedMediumID = mediumID
	s.updated = values
	return true, nil
}

func (s *fakeMediumStore) DeleteMedium(_ context.Context, corpID int, mediumID int) (bool, error) {
	s.deletedCorpID = corpID
	s.deletedMediumID = mediumID
	return true, nil
}

func (s *fakeMediumStore) UpdateMediumGroupID(_ context.Context, corpID int, mediumID int, groupID int) (bool, error) {
	s.movedCorpID = corpID
	s.movedMediumID = mediumID
	s.movedGroupID = groupID
	return true, nil
}

func (s *fakeMediumStore) MediumMediaForUpdateByID(_ context.Context, _ int) (MediumMediaUpdateItem, bool, error) {
	return s.mediumMedia, s.mediumMediaFound, nil
}

func (s *fakeMediumStore) UpdateMediumMediaID(_ context.Context, mediumID int, mediaID string, lastUploadTime int64) (bool, error) {
	s.updatedMediaMediumID = mediumID
	s.updatedMediaID = mediaID
	s.updatedMediaLastUploadTime = lastUploadTime
	return true, nil
}

func (s *fakeMediumStore) MediumCorpCredentialByID(_ context.Context, _ int) (MediumCorpCredential, bool, error) {
	return s.corpCredential, s.corpFound, nil
}

type fakeMediumMediaClient struct {
	mediaID        string
	calls          int
	lastCredential MediumCorpCredential
	lastMediaType  string
	lastFilePath   string
}

func (c *fakeMediumMediaClient) UploadTemporaryMedia(_ context.Context, credential MediumCorpCredential, mediaType string, filePath string) (string, error) {
	c.calls++
	c.lastCredential = credential
	c.lastMediaType = mediaType
	c.lastFilePath = filePath
	if c.mediaID == "" {
		return "new-media-id", nil
	}
	return c.mediaID, nil
}
