package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkRoomBatchUpdateUpdatesSelectedCorpRooms(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                map[int]User{1: {ID: 1}},
		workRoomGroup:        WorkRoomGroupItem{ID: 9, CorpID: 7, Name: "重点群"},
		workRoomGroupFound:   true,
		workRoomBatchUpdated: 2,
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", authorizer)

	req := httptest.NewRequest(http.MethodPut, "/dashboard/workRoom/batchUpdate", strings.NewReader(`{"workRoomIds":"101,102,101,0","workRoomGroupId":9}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomBatchUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workRoom/batchUpdate#put" || authorizer.corpID != 7 || authorizer.workEmployeeID != 88 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if store.lastWorkRoomGroupID != 9 {
		t.Fatalf("group id = %d", store.lastWorkRoomGroupID)
	}
	values := store.lastWorkRoomBatchUpdate
	if values.CorpID != 7 || values.WorkRoomGroupID != 9 {
		t.Fatalf("values = %#v", values)
	}
	if len(values.WorkRoomIDs) != 2 || values.WorkRoomIDs[0] != 101 || values.WorkRoomIDs[1] != 102 {
		t.Fatalf("room ids = %#v", values.WorkRoomIDs)
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["code"].(float64) != 200 || len(body["data"].([]any)) != 0 {
		t.Fatalf("body = %#v", body)
	}
}

func TestWorkRoomBatchUpdateAllowsZeroGroupWithoutLookup(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                map[int]User{1: {ID: 1}},
		workRoomBatchUpdated: 1,
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", &recordingAuthorizer{})

	req := httptest.NewRequest(http.MethodPut, "/dashboard/workRoom/batchUpdate", strings.NewReader(`{"workRoomIds":"101","workRoomGroupId":0}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkRoomBatchUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastWorkRoomGroupID != 0 {
		t.Fatalf("unexpected group lookup = %d", store.lastWorkRoomGroupID)
	}
	if store.lastWorkRoomBatchUpdate.WorkRoomGroupID != 0 {
		t.Fatalf("group id = %d", store.lastWorkRoomBatchUpdate.WorkRoomGroupID)
	}
}

func TestWorkRoomBatchUpdateValidatesGroup(t *testing.T) {
	for _, tc := range []struct {
		name  string
		store *fakeWorkReadStore
		msg   string
	}{
		{
			name: "missing",
			store: &fakeWorkReadStore{
				users: map[int]User{1: {ID: 1}},
			},
			msg: "该分组信息不存在，不可操作",
		},
		{
			name: "wrong-corp",
			store: &fakeWorkReadStore{
				users:              map[int]User{1: {ID: 1}},
				workRoomGroup:      WorkRoomGroupItem{ID: 9, CorpID: 8},
				workRoomGroupFound: true,
			},
			msg: "该分组不归属当前登录企业，不可操作",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewWorkReadHandlerWithAuthorizer(tc.store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", &recordingAuthorizer{})
			req := httptest.NewRequest(http.MethodPut, "/dashboard/workRoom/batchUpdate", strings.NewReader(`{"workRoomIds":"101","workRoomGroupId":9}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()
			handler.WorkRoomBatchUpdate(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			body := decodeBody(t, rec.Body.Bytes())
			if body["msg"] != tc.msg {
				t.Fatalf("msg = %#v", body["msg"])
			}
			if tc.store.lastWorkRoomBatchUpdate.CorpID != 0 {
				t.Fatalf("unexpected update = %#v", tc.store.lastWorkRoomBatchUpdate)
			}
		})
	}
}

func TestWorkRoomBatchUpdateValidatesParamsAndSelectedCorp(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cache staticAdminCache
		body  string
		msg   string
	}{
		{name: "room-id-required", cache: staticAdminCache("7-88"), body: `{"workRoomGroupId":9}`, msg: "客户群ID 必填"},
		{name: "room-id-string", cache: staticAdminCache("7-88"), body: `{"workRoomIds":101,"workRoomGroupId":9}`, msg: "客户群ID 必需为字符串"},
		{name: "group-required", cache: staticAdminCache("7-88"), body: `{"workRoomIds":"101"}`, msg: "客户群分组ID 必填"},
		{name: "group-integer", cache: staticAdminCache("7-88"), body: `{"workRoomIds":"101","workRoomGroupId":"x"}`, msg: "客户群分组ID 必需为整数"},
		{name: "select-corp", cache: staticAdminCache("7-88,8-99"), body: `{"workRoomIds":"101","workRoomGroupId":0}`, msg: "未选择登录企业，不可操作"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewWorkReadHandlerWithAuthorizer(
				&fakeWorkReadStore{users: map[int]User{1: {ID: 1}}},
				tc.cache,
				HeaderUserIDResolver{},
				"",
				&recordingAuthorizer{},
			)
			req := httptest.NewRequest(http.MethodPut, "/dashboard/workRoom/batchUpdate", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()
			handler.WorkRoomBatchUpdate(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			body := decodeBody(t, rec.Body.Bytes())
			if body["msg"] != tc.msg {
				t.Fatalf("msg = %#v", body["msg"])
			}
		})
	}
}
