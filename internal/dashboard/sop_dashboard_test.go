package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSOPDashboardContactIndexReturnsLaravelPage(t *testing.T) {
	store := &fakeSOPDashboardStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		contactPage: ContactSOPDashboardPage{
			Page:    2,
			PerPage: 1,
			Total:   2,
			Items: []ContactSOPDashboardItem{{
				ID:             11,
				CorpID:         7,
				CreatorID:      1,
				CreatorName:    "管理员",
				Name:           "欢迎SOP",
				SettingRaw:     `[{"name":"首条规则"}]`,
				EmployeeIDsRaw: `[31,32]`,
				ContactIDsRaw:  `[88]`,
				State:          1,
				CreatedAt:      "2026-07-05 10:00:00",
			}},
		},
	}
	handler := NewSOPDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/contactSop/index?page=2&perPage=1&name=欢迎", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req.Host = "api.example.com"
	rec := httptest.NewRecorder()

	handler.ContactIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			CurrentPage int              `json:"current_page"`
			Total       int              `json:"total"`
			Data        []map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 200 || envelope.Data.CurrentPage != 2 || envelope.Data.Total != 2 || len(envelope.Data.Data) != 1 {
		t.Fatalf("envelope = %#v", envelope)
	}
	item := envelope.Data.Data[0]
	if item["contactSopId"].(float64) != 11 || item["employeeNum"].(float64) != 2 || item["contactNum"].(float64) != 1 {
		t.Fatalf("item = %#v", item)
	}
	if store.lastContactFilter.CorpID != 7 || store.lastContactFilter.Name != "欢迎" || store.lastContactFilter.Page != 2 {
		t.Fatalf("filter = %#v", store.lastContactFilter)
	}
}

func TestSOPDashboardContactStorePreservesJSONRules(t *testing.T) {
	store := &fakeSOPDashboardStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, createContactID: 66}
	handler := NewSOPDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/contactSop/store", strings.NewReader(`{"name":"欢迎SOP","setting":[{"name":"首条规则","content":[{"type":"text","value":"你好"}]}],"employeeIds":[31,32],"contactIds":"88,89"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ContactStore(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.createdContact.CorpID != 7 || store.createdContact.CreatorID != 1 || store.createdContact.Name != "欢迎SOP" {
		t.Fatalf("created = %#v", store.createdContact)
	}
	if !strings.Contains(store.createdContact.SettingRaw, "首条规则") || store.createdContact.EmployeeIDsRaw != `[31,32]` || store.createdContact.ContactIDsRaw != `[88,89]` {
		t.Fatalf("raw json = setting:%s employees:%s contacts:%s", store.createdContact.SettingRaw, store.createdContact.EmployeeIDsRaw, store.createdContact.ContactIDsRaw)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricContactSOPs {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestSOPDashboardContactStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeSOPDashboardStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricContactSOPs,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewSOPDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/contactSop/store", strings.NewReader(`{"name":"额度外个人SOP"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ContactStore(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "套餐额度已达上限：个人SOP规则数 1/1") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if store.createContactCalls != 0 {
		t.Fatalf("create contact calls = %d", store.createContactCalls)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricContactSOPs || store.quotaAdditional != 1 {
		t.Fatalf("quota = tenant %d metric %s additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.refreshMetric != "" {
		t.Fatalf("refresh should not run, got %s", store.refreshMetric)
	}
}

func TestSOPDashboardContactDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeSOPDashboardStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}}
	handler := NewSOPDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodDelete, "/dashboard/contactSop/destroy", strings.NewReader(`{"contactSopId":66}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ContactDestroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedContactID != 66 {
		t.Fatalf("deleted contact id = %d", store.deletedContactID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricContactSOPs {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestSOPDashboardRoomSetRoomUpdatesRoomIDs(t *testing.T) {
	store := &fakeSOPDashboardStore{user: User{ID: 1, IsSuperAdmin: 1}}
	handler := NewSOPDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/roomSop/setRoom", strings.NewReader(`{"roomSopId":88,"roomIds":[900001,900002]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RoomSetRoom(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.updatedRoomID != 88 || store.updatedRoomCorpID != 7 || store.updatedRoomIDsRaw != `[900001,900002]` {
		t.Fatalf("room update id=%d corp=%d raw=%s", store.updatedRoomID, store.updatedRoomCorpID, store.updatedRoomIDsRaw)
	}
}

func TestSOPDashboardRoomStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeSOPDashboardStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricRoomSOPs,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewSOPDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/roomSop/store", strings.NewReader(`{"name":"额度外群SOP"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RoomStore(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "套餐额度已达上限：群SOP规则数 1/1") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if store.createRoomCalls != 0 {
		t.Fatalf("create room calls = %d", store.createRoomCalls)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricRoomSOPs || store.quotaAdditional != 1 {
		t.Fatalf("quota = tenant %d metric %s additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.refreshMetric != "" {
		t.Fatalf("refresh should not run, got %s", store.refreshMetric)
	}
}

func TestSOPDashboardRoomDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeSOPDashboardStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}}
	handler := NewSOPDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodDelete, "/dashboard/roomSop/destroy", strings.NewReader(`{"roomSopId":88}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.RoomDestroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedRoomID != 88 {
		t.Fatalf("deleted room id = %d", store.deletedRoomID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomSOPs {
		t.Fatalf("refresh = tenant %d metric %s", store.refreshTenantID, store.refreshMetric)
	}
}

func TestSOPDashboardContactUpdateDoesNotDefaultState(t *testing.T) {
	store := &fakeSOPDashboardStore{user: User{ID: 1, IsSuperAdmin: 1}}
	handler := NewSOPDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/contactSop/update", strings.NewReader(`{"contactSopId":66,"name":"只改名称"}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ContactUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !store.updatedContact.HasName || store.updatedContact.Name != "只改名称" {
		t.Fatalf("updated = %#v", store.updatedContact)
	}
	if store.updatedContact.HasState {
		t.Fatalf("state should not be updated by default: %#v", store.updatedContact)
	}
}

type fakeSOPDashboardStore struct {
	user        User
	contactPage ContactSOPDashboardPage
	contactItem ContactSOPDashboardItem
	roomPage    RoomSOPDashboardPage
	roomItem    RoomSOPDashboardItem

	lastContactFilter  ContactSOPDashboardFilter
	lastRoomFilter     RoomSOPDashboardFilter
	createdContact     ContactSOPDashboardWrite
	updatedContact     ContactSOPDashboardWrite
	createdRoom        RoomSOPDashboardWrite
	updatedRoom        RoomSOPDashboardWrite
	createContactID    int
	createRoomID       int
	createContactCalls int
	createRoomCalls    int
	quota              SaaSQuotaStatus
	quotaTenantID      int
	quotaMetric        string
	quotaAdditional    int64
	refreshTenantID    int
	refreshMetric      string
	deletedContactID   int
	deletedRoomID      int
	updatedRoomCorpID  int
	updatedRoomID      int
	updatedRoomIDsRaw  string
}

func (s *fakeSOPDashboardStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeSOPDashboardStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeSOPDashboardStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeSOPDashboardStore) ContactSOPDashboardPage(_ context.Context, filter ContactSOPDashboardFilter) (ContactSOPDashboardPage, error) {
	s.lastContactFilter = filter
	return s.contactPage, nil
}

func (s *fakeSOPDashboardStore) ContactSOPDashboardByID(context.Context, int, int) (ContactSOPDashboardItem, bool, error) {
	return s.contactItem, s.contactItem.ID > 0, nil
}

func (s *fakeSOPDashboardStore) CreateContactSOPDashboard(_ context.Context, values ContactSOPDashboardWrite) (int, error) {
	s.createContactCalls++
	s.createdContact = values
	if s.createContactID > 0 {
		return s.createContactID, nil
	}
	return 1, nil
}

func (s *fakeSOPDashboardStore) UpdateContactSOPDashboard(_ context.Context, _ int, _ int, values ContactSOPDashboardWrite) (bool, error) {
	s.updatedContact = values
	return true, nil
}

func (s *fakeSOPDashboardStore) UpdateContactSOPDashboardEmployees(context.Context, int, int, string) (bool, error) {
	return true, nil
}

func (s *fakeSOPDashboardStore) UpdateContactSOPDashboardState(context.Context, int, int, int) (bool, error) {
	return true, nil
}

func (s *fakeSOPDashboardStore) DeleteContactSOPDashboard(_ context.Context, _ int, id int) (bool, error) {
	s.deletedContactID = id
	return true, nil
}

func (s *fakeSOPDashboardStore) RoomSOPDashboardPage(_ context.Context, filter RoomSOPDashboardFilter) (RoomSOPDashboardPage, error) {
	s.lastRoomFilter = filter
	return s.roomPage, nil
}

func (s *fakeSOPDashboardStore) RoomSOPDashboardByID(context.Context, int, int) (RoomSOPDashboardItem, bool, error) {
	return s.roomItem, s.roomItem.ID > 0, nil
}

func (s *fakeSOPDashboardStore) CreateRoomSOPDashboard(_ context.Context, values RoomSOPDashboardWrite) (int, error) {
	s.createRoomCalls++
	s.createdRoom = values
	if s.createRoomID > 0 {
		return s.createRoomID, nil
	}
	return 1, nil
}

func (s *fakeSOPDashboardStore) UpdateRoomSOPDashboard(_ context.Context, _ int, _ int, values RoomSOPDashboardWrite) (bool, error) {
	s.updatedRoom = values
	return true, nil
}

func (s *fakeSOPDashboardStore) UpdateRoomSOPDashboardRooms(_ context.Context, corpID int, id int, roomIDsRaw string) (bool, error) {
	s.updatedRoomCorpID = corpID
	s.updatedRoomID = id
	s.updatedRoomIDsRaw = roomIDsRaw
	return true, nil
}

func (s *fakeSOPDashboardStore) UpdateRoomSOPDashboardState(context.Context, int, int, int) (bool, error) {
	return true, nil
}

func (s *fakeSOPDashboardStore) DeleteRoomSOPDashboard(_ context.Context, _ int, id int) (bool, error) {
	s.deletedRoomID = id
	return true, nil
}

func (s *fakeSOPDashboardStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" {
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeSOPDashboardStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
