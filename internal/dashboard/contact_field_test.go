package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContactFieldIndexReturnsPagedFields(t *testing.T) {
	store := &fakeContactFieldStore{
		users: map[int]User{1: {ID: 1}},
		page: ContactFieldPage{
			Items: []ContactField{
				{
					ID:      10,
					Name:    "birthday",
					Label:   "生日",
					Type:    6,
					Options: []any{},
					Status:  1,
					Order:   9,
					IsSys:   1,
				},
			},
			Total:     1,
			TotalPage: 1,
			PerPage:   5,
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewContactFieldHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/contactField/index?status=1&page=2&perPage=5", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/contactField/index#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if store.lastFilter.Status != 1 || store.lastFilter.Page != 2 || store.lastFilter.PerPage != 5 {
		t.Fatalf("filter = %#v", store.lastFilter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if int(page["perPage"].(float64)) != 5 || int(page["total"].(float64)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	field := data["list"].([]any)[0].(map[string]any)
	if int(field["id"].(float64)) != 10 || field["name"] != "birthday" || field["label"] != "生日" || field["typeText"] != "日期" || int(field["isSys"].(float64)) != 1 {
		t.Fatalf("field = %#v", field)
	}
}

func TestContactFieldShowReturnsDetail(t *testing.T) {
	store := &fakeContactFieldStore{
		users: map[int]User{1: {ID: 1}},
		field: ContactField{
			ID:      12,
			Name:    "city",
			Label:   "城市",
			Type:    3,
			Options: []any{"上海", "杭州"},
			Status:  1,
			Order:   8,
			IsSys:   0,
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewContactFieldHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/contactField/show?id=12", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/contactField/show#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if store.lastID != 12 {
		t.Fatalf("lastID = %d", store.lastID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if int(data["id"].(float64)) != 12 || data["name"] != "city" || data["label"] != "城市" || data["typeText"] != "下拉" || int(data["isSys"].(float64)) != 0 {
		t.Fatalf("data = %#v", data)
	}
	options := data["options"].([]any)
	if len(options) != 2 || options[0] != "上海" || options[1] != "杭州" {
		t.Fatalf("options = %#v", options)
	}
}

func TestContactFieldShowMissingReturnsPHPMessage(t *testing.T) {
	store := &fakeContactFieldStore{users: map[int]User{1: {ID: 1}}}
	handler := NewContactFieldHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/contactField/show?id=404", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "无此条信息" {
		t.Fatalf("body = %#v", body)
	}
}

func TestContactFieldPortraitReturnsPHPCompatibleOptions(t *testing.T) {
	store := &fakeContactFieldStore{
		users: map[int]User{1: {ID: 1}},
		portraitFields: []ContactField{
			{
				ID:      21,
				Label:   "生日",
				Type:    6,
				Options: []any{},
			},
			{
				ID:      22,
				Label:   "头像",
				Type:    11,
				Options: []any{},
			},
		},
	}
	handler := NewContactFieldHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/contactField/portrait", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Portrait(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastStatus != 1 {
		t.Fatalf("lastStatus = %d", store.lastStatus)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("data = %#v", data)
	}
	all := data[0].(map[string]any)
	if int(all["fieldId"].(float64)) != 0 || all["name"] != "全部" {
		t.Fatalf("all = %#v", all)
	}
	field := data[1].(map[string]any)
	if int(field["fieldId"].(float64)) != 21 || field["name"] != "生日" || field["typeText"] != "日期" {
		t.Fatalf("field = %#v", field)
	}
	if tail := data[2].([]any); len(tail) != 0 {
		t.Fatalf("tail = %#v", tail)
	}
}

func TestContactFieldPivotIndexReturnsValues(t *testing.T) {
	store := &fakeContactFieldStore{
		users: map[int]User{1: {ID: 1}},
		portraitFields: []ContactField{
			{ID: 31, Label: "爱好", Type: 2, Options: []any{"跑步", "读书"}},
			{ID: 32, Label: "照片", Type: 11, Options: []any{}},
			{ID: 33, Label: "城市", Type: 3, Options: []any{"上海"}},
		},
		pivots: map[int]ContactFieldPivot{
			31: {ID: 901, ContactFieldID: 31, Value: "跑步,读书"},
			32: {ID: 902, ContactFieldID: 32, Value: "portrait/a.png"},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewContactFieldHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "http://api.example.com")

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/contactFieldPivot/index?contactId=55", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.FieldPivotIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/contactFieldPivot/index#get" || authorizer.corpID != 7 || authorizer.workEmployeeID != 99 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if store.lastContactID != 55 {
		t.Fatalf("lastContactID = %d", store.lastContactID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("data = %#v", data)
	}
	checkbox := data[0].(map[string]any)
	if int(checkbox["contactFieldId"].(float64)) != 31 || checkbox["name"] != "爱好" || checkbox["typeText"] != "多选" || int(checkbox["contactFieldPivotId"].(float64)) != 901 {
		t.Fatalf("checkbox = %#v", checkbox)
	}
	values := checkbox["value"].([]any)
	if len(values) != 2 || values[0] != "跑步" || values[1] != "读书" {
		t.Fatalf("values = %#v", values)
	}
	picture := data[1].(map[string]any)
	if picture["pictureFlag"] != "http://api.example.com/static/portrait/a.png" || picture["value"] != "portrait/a.png" {
		t.Fatalf("picture = %#v", picture)
	}
	empty := data[2].(map[string]any)
	if empty["contactFieldPivotId"] != "" || empty["value"] != "" {
		t.Fatalf("empty = %#v", empty)
	}
}

func TestSidebarContactFieldPivotIndexUsesEmployeeTokenWithoutRBAC(t *testing.T) {
	store := &fakeContactFieldStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		portraitFields: []ContactField{
			{ID: 31, Label: "爱好", Type: 2, Options: []any{"跑步", "读书"}},
			{ID: 32, Label: "照片", Type: 11, Options: []any{}},
		},
		pivots: map[int]ContactFieldPivot{
			31: {ID: 901, ContactFieldID: 31, Value: "跑步,读书"},
			32: {ID: 902, ContactFieldID: 32, Value: "portrait/a.png"},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewContactFieldHandler(store, nil, HeaderUserIDResolver{}, authorizer, "http://api.example.com").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/contactFieldPivot/index?contactId=55", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarFieldPivotIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "" {
		t.Fatalf("unexpected RBAC call: %#v", authorizer)
	}
	if store.lastSidebarEmployeeID != 5 || store.lastContactID != 55 {
		t.Fatalf("lastSidebarEmployeeID=%d lastContactID=%d", store.lastSidebarEmployeeID, store.lastContactID)
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].([]any)
	checkbox := data[0].(map[string]any)
	values := checkbox["value"].([]any)
	if len(values) != 2 || values[0] != "跑步" || values[1] != "读书" {
		t.Fatalf("values = %#v", values)
	}
	picture := data[1].(map[string]any)
	if picture["pictureFlag"] != "http://api.example.com/static/portrait/a.png" {
		t.Fatalf("picture = %#v", picture)
	}
}

func TestSidebarContactFieldPivotIndexRejectsAContactOutsideTheEmployeeScope(t *testing.T) {
	store := &fakeContactFieldStore{
		sidebarEmployees:  map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		denyContactAccess: true,
		portraitFields:    []ContactField{{ID: 31, Label: "城市", Type: 0}},
	}
	handler := NewContactFieldHandler(store, nil, HeaderUserIDResolver{}, nil, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/sidebar/contactFieldPivot/index?contactId=55", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()

	handler.SidebarFieldPivotIndex(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastContactID != 0 {
		t.Fatalf("portrait data was queried for forbidden contact %d", store.lastContactID)
	}
}

func TestContactFieldPivotUpdateCreatesUpdatesAndTracks(t *testing.T) {
	store := &fakeContactFieldStore{
		users:         map[int]User{1: {ID: 1}},
		pivotsByID:    map[int]ContactFieldPivot{901: {ID: 901, ContactID: 55, ContactFieldID: 31, Value: "旧备注"}},
		updatedPivots: map[int]string{},
		createdPivots: []ContactFieldPivotCreate{},
		createdTracks: []ContactEmployeeTrackCreate{},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewContactFieldHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "")

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/contactFieldPivot/update", strings.NewReader(`{
		"contactId":55,
		"userPortrait":"[{\"contactFieldPivotId\":901,\"contactFieldId\":31,\"name\":\"备注\",\"type\":0,\"value\":\"新备注\"},{\"contactFieldPivotId\":\"\",\"contactFieldId\":32,\"name\":\"爱好\",\"type\":2,\"value\":[\"跑步\",\"读书\"]}]"
	}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.FieldPivotUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "" {
		t.Fatalf("unexpected RBAC call: %#v", authorizer)
	}
	if store.updatedPivots[901] != "新备注" {
		t.Fatalf("updated pivots = %#v", store.updatedPivots)
	}
	if len(store.createdPivots) != 1 || store.createdPivots[0].ContactID != 55 || store.createdPivots[0].ContactFieldID != 32 || store.createdPivots[0].Value != "跑步,读书" {
		t.Fatalf("created pivots = %#v", store.createdPivots)
	}
	if len(store.createdTracks) != 1 {
		t.Fatalf("created tracks = %#v", store.createdTracks)
	}
	track := store.createdTracks[0]
	if track.EmployeeID != 99 || track.ContactID != 55 || track.CorpID != 7 || track.Event != 4 || track.Content != "编辑用户画像：备注 爱好 " {
		t.Fatalf("track = %#v", track)
	}
}

func TestSidebarContactFieldPivotUpdateUsesEmployeeContext(t *testing.T) {
	store := &fakeContactFieldStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		pivotsByID:       map[int]ContactFieldPivot{902: {ID: 902, ContactID: 66, ContactFieldID: 33, Value: "旧城市"}},
		updatedPivots:    map[int]string{},
		createdTracks:    []ContactEmployeeTrackCreate{},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewContactFieldHandler(store, nil, HeaderUserIDResolver{}, authorizer, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/sidebar/contactFieldPivot/update", strings.NewReader(`{
		"contactId":66,
		"userPortrait":[{"contactFieldPivotId":902,"contactFieldId":33,"name":"城市","type":3,"value":"新城市"}]
	}`))
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarFieldPivotUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "" {
		t.Fatalf("unexpected RBAC call: %#v", authorizer)
	}
	if store.updatedPivots[902] != "新城市" {
		t.Fatalf("updated pivots = %#v", store.updatedPivots)
	}
	if len(store.createdTracks) != 1 {
		t.Fatalf("created tracks = %#v", store.createdTracks)
	}
	track := store.createdTracks[0]
	if track.EmployeeID != 5 || track.ContactID != 66 || track.CorpID != 7 || track.Event != 4 || track.Content != "编辑用户画像：城市 " {
		t.Fatalf("track = %#v", track)
	}
}

func TestSidebarContactFieldPivotUpdateRejectsAPivotFromAnotherContact(t *testing.T) {
	store := &fakeContactFieldStore{
		sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		pivotsByID:       map[int]ContactFieldPivot{902: {ID: 902, ContactID: 77, ContactFieldID: 33, Value: "旧城市"}},
	}
	handler := NewContactFieldHandler(store, nil, HeaderUserIDResolver{}, nil, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})
	req := authenticatedDashboardRequestForTest(http.MethodPut, "/sidebar/contactFieldPivot/update", strings.NewReader(`{
		"contactId":66,
		"userPortrait":[{"contactFieldPivotId":902,"contactFieldId":33,"name":"城市","type":3,"value":"新城市"}]
	}`))
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()

	handler.SidebarFieldPivotUpdate(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.updatedPivots) != 0 {
		t.Fatalf("foreign pivot was updated: %#v", store.updatedPivots)
	}
}

func TestContactFieldStoreCreatesCustomField(t *testing.T) {
	store := &fakeContactFieldStore{users: map[int]User{1: {ID: 1}}}
	authorizer := &recordingAuthorizer{}
	handler := NewContactFieldHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, authorizer, "")

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/contactField/store", strings.NewReader(`{"label":"城市","type":3,"options":["上海","杭州"],"order":8,"status":1}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/contactField/store#post" {
		t.Fatalf("permission key = %q", authorizer.permissionKey)
	}
	if store.createdField.Label != "城市" || store.createdField.Type != 3 || store.createdField.Order != 8 || store.createdField.Status != 1 {
		t.Fatalf("created = %+v", store.createdField)
	}
	if store.createdField.Name == "" || !strings.Contains(store.createdField.Options, "上海") {
		t.Fatalf("created name/options = %+v", store.createdField)
	}
}

func TestContactFieldUpdateKeepsSystemFieldDefinition(t *testing.T) {
	store := &fakeContactFieldStore{
		users: map[int]User{1: {ID: 1}},
		field: ContactField{ID: 2, Name: "phone", Label: "手机号", Type: 7, Options: []any{}, Order: 1, Status: 1, IsSys: 1},
	}
	handler := NewContactFieldHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/contactField/update", strings.NewReader(`{"id":2,"label":"城市","type":3,"options":["上海"],"order":9,"status":0}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.updatedFieldID != 2 {
		t.Fatalf("updated id = %d", store.updatedFieldID)
	}
	if store.updatedField.Name != "phone" || store.updatedField.Label != "手机号" || store.updatedField.Type != 7 || store.updatedField.Order != 9 || store.updatedField.Status != 0 {
		t.Fatalf("updated field = %+v", store.updatedField)
	}
}

func TestContactFieldDestroyRejectsSystemField(t *testing.T) {
	store := &fakeContactFieldStore{
		users: map[int]User{1: {ID: 1}},
		field: ContactField{ID: 2, IsSys: 1},
	}
	handler := NewContactFieldHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/contactField/destroy", strings.NewReader(`{"id":2}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Destroy(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "系统初始化字段不可删除" {
		t.Fatalf("body = %#v", body)
	}
	if store.deletedFieldID != 0 {
		t.Fatalf("deleted id = %d", store.deletedFieldID)
	}
}

func TestContactFieldBatchUpdateUsesTransactionPayload(t *testing.T) {
	store := &fakeContactFieldStore{
		users:  map[int]User{1: {ID: 1}},
		fields: map[int]ContactField{2: {ID: 2, Label: "城市", Type: 3, Options: []any{"上海"}, Status: 1}},
	}
	handler := NewContactFieldHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, "")

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/contactField/batchUpdate", strings.NewReader(`{"update":[{"id":2,"label":"城市","type":3,"options":["北京"],"order":3,"status":1}],"destroy":[4,5]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.BatchUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(store.batchUpdates) != 1 || store.batchUpdates[0].ID != 2 || store.batchUpdates[0].Values.Order != 3 {
		t.Fatalf("batch updates = %+v", store.batchUpdates)
	}
	if len(store.batchDestroyIDs) != 2 || store.batchDestroyIDs[0] != 4 || store.batchDestroyIDs[1] != 5 {
		t.Fatalf("destroy ids = %+v", store.batchDestroyIDs)
	}
}

type fakeContactFieldStore struct {
	users                 map[int]User
	sidebarEmployees      map[int]SidebarEmployee
	page                  ContactFieldPage
	field                 ContactField
	fields                map[int]ContactField
	portraitFields        []ContactField
	pivots                map[int]ContactFieldPivot
	pivotsByID            map[int]ContactFieldPivot
	lastFilter            ContactFieldFilter
	lastID                int
	lastStatus            int
	lastContactID         int
	lastSidebarEmployeeID int
	labelExists           bool
	createdField          ContactFieldWriteValues
	updatedFieldID        int
	updatedField          ContactFieldWriteValues
	statusFieldID         int
	statusValue           int
	deletedFieldID        int
	batchUpdates          []ContactFieldBatchUpdate
	batchDestroyIDs       []int
	updatedPivots         map[int]string
	createdPivots         []ContactFieldPivotCreate
	createdTracks         []ContactEmployeeTrackCreate
	denyContactAccess     bool
}

func (s *fakeContactFieldStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeContactFieldStore) SidebarEmployeeByID(_ context.Context, employeeID int) (SidebarEmployee, bool, error) {
	s.lastSidebarEmployeeID = employeeID
	employee, ok := s.sidebarEmployees[employeeID]
	return employee, ok, nil
}

func (s *fakeContactFieldStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 0, nil
}

func (s *fakeContactFieldStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 0, 0, false, nil
}

func (s *fakeContactFieldStore) ContactAccessibleToEmployee(_ context.Context, _ int, _ int, _ int) (bool, error) {
	return !s.denyContactAccess, nil
}

func (s *fakeContactFieldStore) ContactFieldPage(_ context.Context, filter ContactFieldFilter) (ContactFieldPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeContactFieldStore) ContactFieldByID(_ context.Context, fieldID int) (ContactField, bool, error) {
	s.lastID = fieldID
	if s.fields != nil {
		field, ok := s.fields[fieldID]
		return field, ok, nil
	}
	if s.field.ID == 0 {
		return ContactField{}, false, nil
	}
	return s.field, true, nil
}

func (s *fakeContactFieldStore) ContactFieldsByStatusOrder(_ context.Context, status int) ([]ContactField, error) {
	s.lastStatus = status
	return s.portraitFields, nil
}

func (s *fakeContactFieldStore) ContactFieldPivotsByContactID(_ context.Context, contactID int, _ []int) (map[int]ContactFieldPivot, error) {
	s.lastContactID = contactID
	if s.pivots == nil {
		return map[int]ContactFieldPivot{}, nil
	}
	return s.pivots, nil
}

func (s *fakeContactFieldStore) ContactFieldPivotByID(_ context.Context, pivotID int) (ContactFieldPivot, bool, error) {
	pivot, ok := s.pivotsByID[pivotID]
	return pivot, ok, nil
}

func (s *fakeContactFieldStore) UpdateContactFieldPivotValue(_ context.Context, pivotID int, value string) (bool, error) {
	if s.updatedPivots == nil {
		s.updatedPivots = map[int]string{}
	}
	s.updatedPivots[pivotID] = value
	return true, nil
}

func (s *fakeContactFieldStore) CreateContactFieldPivots(_ context.Context, pivots []ContactFieldPivotCreate) error {
	s.createdPivots = append(s.createdPivots, pivots...)
	return nil
}

func (s *fakeContactFieldStore) CreateContactEmployeeTrack(_ context.Context, track ContactEmployeeTrackCreate) error {
	s.createdTracks = append(s.createdTracks, track)
	return nil
}

func (s *fakeContactFieldStore) ContactFieldLabelExists(_ context.Context, label string, excludeFieldID int) (bool, error) {
	return s.labelExists, nil
}

func (s *fakeContactFieldStore) CreateContactField(_ context.Context, values ContactFieldWriteValues) (int, error) {
	s.createdField = values
	return 101, nil
}

func (s *fakeContactFieldStore) UpdateContactField(_ context.Context, fieldID int, values ContactFieldWriteValues) (bool, error) {
	s.updatedFieldID = fieldID
	s.updatedField = values
	return true, nil
}

func (s *fakeContactFieldStore) UpdateContactFieldStatus(_ context.Context, fieldID int, status int) (bool, error) {
	s.statusFieldID = fieldID
	s.statusValue = status
	return true, nil
}

func (s *fakeContactFieldStore) DeleteContactField(_ context.Context, fieldID int) (bool, error) {
	s.deletedFieldID = fieldID
	return true, nil
}

func (s *fakeContactFieldStore) BatchUpdateContactFields(_ context.Context, updates []ContactFieldBatchUpdate, destroyIDs []int) error {
	s.batchUpdates = append([]ContactFieldBatchUpdate{}, updates...)
	s.batchDestroyIDs = append([]int{}, destroyIDs...)
	return nil
}
