package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOfficialAccountIndexListsAccounts(t *testing.T) {
	store := &fakeOfficialAccountStore{
		users: map[int]User{1: {ID: 1}},
		accounts: []OfficialAccountItem{
			{ID: 2, Nickname: "公众号B", Avatar: "avatar/b.jpg"},
			{ID: 1, Nickname: "公众号A", Avatar: "avatar/a.jpg"},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewOfficialAccountHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "http://api.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/officialAccount/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/officialAccount/index#get" {
		t.Fatalf("permission=%s", authorizer.permissionKey)
	}
	list := decodeBody(t, rec.Body.Bytes())["data"].([]any)
	first := list[0].(map[string]any)
	if first["nickname"] != "公众号B" || first["avatar"] != "http://api.example.com/static/avatar/b.jpg" {
		t.Fatalf("first=%#v", first)
	}
}

func TestOfficialAccountIndexFindsOrCreatesModuleSet(t *testing.T) {
	store := &fakeOfficialAccountStore{
		users:    map[int]User{1: {ID: 1}},
		accounts: []OfficialAccountItem{{ID: 7, Nickname: "默认公众号", Avatar: "avatar/default.jpg"}},
		sets: map[int]OfficialAccountSetItem{
			2: {ID: 200, OfficialAccountID: 7, Type: 2, CorpID: 7},
		},
	}
	handler := NewOfficialAccountHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/officialAccount/index?type=2", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeBody(t, rec.Body.Bytes())["data"].(map[string]any)
	if data["id"].(float64) != 7 || store.upsertType != 0 {
		t.Fatalf("data=%#v upsertType=%d", data, store.upsertType)
	}

	delete(store.sets, 2)
	req = httptest.NewRequest(http.MethodGet, "/dashboard/officialAccount/index?type=2", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.Index(rec, req)
	if rec.Code != http.StatusOK || store.upsertType != 2 || store.upsertAccountID != 7 {
		t.Fatalf("status=%d upsert=%d/%d body=%s", rec.Code, store.upsertType, store.upsertAccountID, rec.Body.String())
	}
}

func TestOfficialAccountSetUpsertsModuleAccount(t *testing.T) {
	store := &fakeOfficialAccountStore{
		users: map[int]User{1: {ID: 1}},
		sets:  map[int]OfficialAccountSetItem{},
	}
	handler := NewOfficialAccountHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/officialAccount/set?type=3&official_account_id=9", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Set(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.upsertCorpID != 7 || store.upsertUserID != 1 || store.upsertType != 3 || store.upsertAccountID != 9 {
		t.Fatalf("upsert=%d/%d/%d/%d", store.upsertCorpID, store.upsertUserID, store.upsertType, store.upsertAccountID)
	}
}

type fakeOfficialAccountStore struct {
	users           map[int]User
	accounts        []OfficialAccountItem
	sets            map[int]OfficialAccountSetItem
	upsertCorpID    int
	upsertType      int
	upsertAccountID int
	upsertUserID    int
}

func (s *fakeOfficialAccountStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeOfficialAccountStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 99, nil
}

func (s *fakeOfficialAccountStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeOfficialAccountStore) OfficialAccountsByCorpID(_ context.Context, _ int) ([]OfficialAccountItem, error) {
	return append([]OfficialAccountItem{}, s.accounts...), nil
}

func (s *fakeOfficialAccountStore) OfficialAccountFirstByCorpID(_ context.Context, _ int) (OfficialAccountItem, bool, error) {
	if len(s.accounts) == 0 {
		return OfficialAccountItem{}, false, nil
	}
	return s.accounts[0], true, nil
}

func (s *fakeOfficialAccountStore) OfficialAccountByID(_ context.Context, id int) (OfficialAccountItem, bool, error) {
	for _, item := range s.accounts {
		if item.ID == id {
			return item, true, nil
		}
	}
	return OfficialAccountItem{}, false, nil
}

func (s *fakeOfficialAccountStore) OfficialAccountSetByCorpIDType(_ context.Context, _ int, accountType int) (OfficialAccountSetItem, bool, error) {
	item, ok := s.sets[accountType]
	return item, ok, nil
}

func (s *fakeOfficialAccountStore) UpsertOfficialAccountSet(_ context.Context, corpID int, accountType int, officialAccountID int, userID int) error {
	s.upsertCorpID = corpID
	s.upsertType = accountType
	s.upsertAccountID = officialAccountID
	s.upsertUserID = userID
	if s.sets == nil {
		s.sets = map[int]OfficialAccountSetItem{}
	}
	s.sets[accountType] = OfficialAccountSetItem{OfficialAccountID: officialAccountID, Type: accountType, CorpID: corpID}
	return nil
}
