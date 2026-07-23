package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGreetingIndexAppliesDataPermissionAndReturnsState(t *testing.T) {
	store := &fakeGreetingStore{
		users: map[int]User{1: {ID: 1}},
		page: GreetingPage{
			Items: []GreetingItem{{ID: 11, CorpID: 7, Type: "-1-2-", Words: "你好", MediumID: 21, RangeType: 2, EmployeeIDs: []int{31}, CreatedAt: "2026-07-03 10:00:00"}},
			Total: 1, TotalPage: 1, PerPage: 10,
		},
		allGreetings: []GreetingItem{
			{ID: 10, CorpID: 7, RangeType: 1},
			{ID: 11, CorpID: 7, RangeType: 2, EmployeeIDs: []int{31}},
		},
		employees: map[int]GreetingEmployee{31: {ID: 31, Name: "张三"}},
		media:     map[int]GreetingMedium{21: {ID: 21, Type: 2, Content: map[string]any{"imagePath": "image/a.png"}}},
	}
	authz := &recordingAuthorizer{accessSet: true, access: AccessContext{DataPermission: DataPermissionSelf, WorkEmployeeID: 31, DeptEmployeeIDs: []int{31}}}
	handler := NewGreetingHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authz, "http://api.example.com")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/greeting/index", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !store.lastFilter.RestrictOperationIDs || len(store.lastFilter.OperationIDs) != 1 || store.lastFilter.OperationIDs[0] != 31 {
		t.Fatalf("filter = %#v", store.lastFilter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["hadGeneral"].(float64)) != 1 {
		t.Fatalf("hadGeneral = %#v", data["hadGeneral"])
	}
	list := data["list"].([]any)
	item := list[0].(map[string]any)
	if item["typeText"] != "文本+图片" || item["rangeTypeText"] != "指定企业成员" {
		t.Fatalf("item = %#v", item)
	}
}

func TestGreetingStoreNormalizesTypeAndEmployees(t *testing.T) {
	store := &fakeGreetingStore{users: map[int]User{1: {ID: 1}}}
	authz := &recordingAuthorizer{accessSet: true, access: AccessContext{DataPermission: DataPermissionAll, WorkEmployeeID: 31}}
	handler := NewGreetingHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authz, "http://api.example.com")

	req := httptest.NewRequest(http.MethodPost, "/dashboard/greeting/store", strings.NewReader(`{"rangeType":2,"type":"1,2,1","employees":"31,32","words":"你好","mediumId":21}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.Type != "-1-2-" || store.created.RangeType != 2 || len(store.created.EmployeeIDs) != 2 || store.created.EmployeeIDs[0] != 31 || store.created.EmployeeIDs[1] != 32 || store.createdOperationID != 31 {
		t.Fatalf("created = %#v operation=%d", store.created, store.createdOperationID)
	}
}

func TestGreetingDestroyRejectsForeignCorp(t *testing.T) {
	store := &fakeGreetingStore{
		users:         map[int]User{1: {ID: 1}},
		greeting:      GreetingItem{ID: 11, CorpID: 8},
		greetingFound: true,
	}
	handler := NewGreetingHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "http://api.example.com")

	req := httptest.NewRequest(http.MethodDelete, "/dashboard/greeting/destroy", strings.NewReader(`{"greetingId":11}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Destroy(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "此欢迎语不归属当前企业，不可操作" {
		t.Fatalf("msg = %#v", body["msg"])
	}
	if store.deletedID != 0 {
		t.Fatalf("deleted = %d", store.deletedID)
	}
}

type fakeGreetingStore struct {
	users              map[int]User
	page               GreetingPage
	allGreetings       []GreetingItem
	greeting           GreetingItem
	greetingFound      bool
	employees          map[int]GreetingEmployee
	media              map[int]GreetingMedium
	lastFilter         GreetingFilter
	created            GreetingWrite
	createdOperationID int
	updatedID          int
	updated            GreetingWrite
	updatedOperationID int
	deletedID          int
}

func (s *fakeGreetingStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeGreetingStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 0, nil
}

func (s *fakeGreetingStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 0, 0, false, nil
}

func (s *fakeGreetingStore) GreetingPage(_ context.Context, filter GreetingFilter) (GreetingPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeGreetingStore) GreetingsByCorp(_ context.Context, _ int) ([]GreetingItem, error) {
	return s.allGreetings, nil
}

func (s *fakeGreetingStore) GreetingByID(_ context.Context, _ int) (GreetingItem, bool, error) {
	return s.greeting, s.greetingFound, nil
}

func (s *fakeGreetingStore) GreetingEmployeesByIDs(_ context.Context, _ []int) (map[int]GreetingEmployee, error) {
	return s.employees, nil
}

func (s *fakeGreetingStore) GreetingMediaByIDs(_ context.Context, _ []int) (map[int]GreetingMedium, error) {
	return s.media, nil
}

func (s *fakeGreetingStore) CreateGreetingWithLog(_ context.Context, values GreetingWrite, operationID int) (int, error) {
	s.created = values
	s.createdOperationID = operationID
	return 1, nil
}

func (s *fakeGreetingStore) UpdateGreetingWithLog(_ context.Context, greetingID int, values GreetingWrite, operationID int) (bool, error) {
	s.updatedID = greetingID
	s.updated = values
	s.updatedOperationID = operationID
	return true, nil
}

func (s *fakeGreetingStore) DeleteGreeting(_ context.Context, greetingID int) (bool, error) {
	s.deletedID = greetingID
	return true, nil
}
