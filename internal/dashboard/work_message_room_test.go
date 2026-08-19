package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type roomHandlerTestStore struct {
	*fakeAutoTagStore
	directory       WorkMessageRoomDirectoryPage
	directoryFilter WorkMessageRoomDirectoryFilter
	profile         WorkMessageRoomProfile
	profileFilter   WorkMessageRoomProfileFilter
	messages        WorkMessageRoomMessages
	messagesFilter  WorkMessageRoomMessagesFilter
	members         WorkMessageRoomMemberPage
	membersFilter   WorkMessageRoomMembersFilter
	options         WorkMessageRoomFilterOptions
}

func newRoomHandlerTestStore() *roomHandlerTestStore {
	return &roomHandlerTestStore{fakeAutoTagStore: &fakeAutoTagStore{user: User{ID: 42, TenantID: 1}}}
}

func (s *roomHandlerTestStore) RoomDirectory(_ context.Context, filter WorkMessageRoomDirectoryFilter) (WorkMessageRoomDirectoryPage, error) {
	s.directoryFilter = filter
	return s.directory, nil
}

func (s *roomHandlerTestStore) RoomProfile(_ context.Context, filter WorkMessageRoomProfileFilter) (WorkMessageRoomProfile, error) {
	s.profileFilter = filter
	return s.profile, nil
}

func (s *roomHandlerTestStore) RoomMessages(_ context.Context, filter WorkMessageRoomMessagesFilter) (WorkMessageRoomMessages, error) {
	s.messagesFilter = filter
	return s.messages, nil
}

func (s *roomHandlerTestStore) RoomMembers(_ context.Context, filter WorkMessageRoomMembersFilter) (WorkMessageRoomMemberPage, error) {
	s.membersFilter = filter
	return s.members, nil
}

func (s *roomHandlerTestStore) RoomFilterOptions(context.Context, WorkMessageRoomFilterOptionsFilter) (WorkMessageRoomFilterOptions, error) {
	return s.options, nil
}

func TestWorkMessageRoomDirectoryUsesPrincipalScopeAndFixedPageSize(t *testing.T) {
	store := newRoomHandlerTestStore()
	store.directory = WorkMessageRoomDirectoryPage{Items: []WorkMessageRoomDirectoryItem{}, Page: 2, PageSize: 50}
	authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
		CorpID: 7, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10},
	}}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/roomDirectory?corpId=999&roomMode=active&keyword=客户&page=2&pageSize=50", nil)
	req = withAutoTagTestPrincipal(req, 42, 1, 7)
	rec := httptest.NewRecorder()

	handler.WorkMessageRoomDirectory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	filter := store.directoryFilter
	if filter.TenantID != 1 || filter.CorpID != 7 || filter.UserID != 42 || filter.RoomMode != WorkMessageRoomModeActive ||
		filter.Keyword != "客户" || filter.Page != 2 || filter.PageSize != 50 || !filter.RestrictEmployeeIDs ||
		!reflect.DeepEqual(filter.EmployeeIDs, []int{9, 10}) {
		t.Fatalf("filter=%#v", filter)
	}
	var envelope struct {
		Data WorkMessageRoomDirectoryPage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Items == nil {
		t.Fatal("items must be an empty array instead of null")
	}
}

func TestWorkMessageRoomDirectoryRejectsInvalidModeAndPageSize(t *testing.T) {
	for _, query := range []string{"roomMode=unknown&pageSize=50", "roomMode=active&pageSize=20"} {
		t.Run(query, func(t *testing.T) {
			store := newRoomHandlerTestStore()
			handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
			req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/roomDirectory?"+query, nil)
			req = withAutoTagTestPrincipal(req, 42, 1, 7)
			rec := httptest.NewRecorder()
			handler.WorkMessageRoomDirectory(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestWorkMessageRoomMessagesRejectsInvalidRoomAndBeforeCursor(t *testing.T) {
	for _, query := range []string{"roomId=0&pageSize=50", "roomId=3001&pageSize=20", "roomId=3001&pageSize=50&before=bad"} {
		t.Run(query, func(t *testing.T) {
			store := newRoomHandlerTestStore()
			handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
			req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/roomMessages?"+query, nil)
			req = withAutoTagTestPrincipal(req, 42, 1, 7)
			rec := httptest.NewRecorder()
			handler.WorkMessageRoomMessages(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestWorkMessageRoomMembersParsesModeAndFixedPageSize(t *testing.T) {
	store := newRoomHandlerTestStore()
	store.members = WorkMessageRoomMemberPage{Items: []WorkMessageRoomMember{}, Page: 1, PageSize: 50}
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/roomMembers?roomId=3001&mode=left&keyword=李&page=1&pageSize=50", nil)
	req = withAutoTagTestPrincipal(req, 42, 1, 7)
	rec := httptest.NewRecorder()
	handler.WorkMessageRoomMembers(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := store.membersFilter; got.RoomID != 3001 || got.Mode != WorkMessageRoomMemberModeLeft || got.Keyword != "李" || got.PageSize != 50 {
		t.Fatalf("filter=%#v", got)
	}
}

func TestWorkMessageRoomProfileRequiresPositiveRoomID(t *testing.T) {
	store := newRoomHandlerTestStore()
	handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/roomProfile?roomId=nope", nil)
	req = withAutoTagTestPrincipal(req, 42, 1, 7)
	rec := httptest.NewRecorder()
	handler.WorkMessageRoomProfile(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
