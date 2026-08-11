package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomFissionShowContactKeepsZeroStatusFilters(t *testing.T) {
	store := &fakeRoomFissionStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		contactPage: RoomFissionContactPage{
			Page:    1,
			PerPage: 15,
			Total:   1,
			Items: []RoomFissionContactItem{{
				ID:         9,
				FissionID:  12,
				Nickname:   "未完成客户",
				Status:     0,
				WriteOff:   0,
				JoinStatus: 0,
			}},
		},
	}
	handler := NewRoomFissionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "")
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/roomFission/showContact?fissionId=12&status=0&writeOff=0&joinStatus=0&receiveStatus=0&page=1&perPage=15", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ShowContact(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastContactFilter.CorpID != 7 || store.lastContactFilter.FissionID != 12 || store.lastContactFilter.Status != 0 || store.lastContactFilter.WriteOff != 0 || store.lastContactFilter.JoinStatus != 0 || store.lastContactFilter.ReceiveStatus != 0 {
		t.Fatalf("filter = %#v", store.lastContactFilter)
	}
	var envelope struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.List) != 1 || envelope.Data.List[0]["nickname"] != "未完成客户" {
		t.Fatalf("list = %#v", envelope.Data.List)
	}
}

func TestRoomFissionStorePreservesNestedJSON(t *testing.T) {
	store := &fakeRoomFissionStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, createID: 88}
	handler := NewRoomFissionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "https://op.example")
	body := `{
		"fission":{"official_account_id":3,"active_name":"群裂变活动","end_time":"2026-08-01 12:00:00","target_count":5,"new_friend":1,"delete_invalid":0,"receive_employees":[11,12],"auto_pass":true},
		"poster":{"cover_pic":"/upload/poster.png","avatar_show":1,"nickname_show":1,"nickname_color":"#ff0000"},
		"rooms":[{"room_qrcode":"/upload/room.png","room_max":200,"room":{"id":31,"name":"活动群"}}],
		"welcome":{"text":"欢迎入群","link_title":"领取奖品","link_desc":"完成任务","link_pic":"/upload/welcome.png"},
		"invite":{"type":1,"employees":[11],"choose_contact":{"is_all":0,"gender":3},"text":"邀请文案","link_title":"参与活动","link_desc":"快来参加","link_pic":"/upload/invite.png"}
	}`
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomFission/store", strings.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.Fission.CorpID != 7 || store.created.Fission.CreateUserID != 1 || store.created.Fission.ActiveName != "群裂变活动" || store.created.Fission.AutoPass != 1 {
		t.Fatalf("created fission = %#v", store.created.Fission)
	}
	if !strings.Contains(store.created.Fission.ReceiveEmployeesRaw, "11") || !strings.Contains(store.created.Rooms[0].RoomRaw, "活动群") || !strings.Contains(store.created.Invite.ChooseContactRaw, "gender") {
		t.Fatalf("json raw = %#v", store.created)
	}
	if !store.created.RoomsTouched || len(store.created.Rooms) != 1 || store.created.Poster.CoverPic == "" || store.created.Welcome.Text == "" || !store.created.Invite.Touched {
		t.Fatalf("nested values = %#v", store.created)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomFissions {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestRoomFissionStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeRoomFissionStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricRoomFissions,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewRoomFissionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "")
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/roomFission/store", strings.NewReader(`{"fission":{"active_name":"额度外群裂变"}}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Msg != "套餐额度已达上限：群裂变数 1/1" {
		t.Fatalf("envelope = %#v", envelope)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricRoomFissions || store.quotaAdditional != 1 {
		t.Fatalf("quota = tenant %d metric %q additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.createCalls != 0 || store.refreshMetric != "" {
		t.Fatalf("side effects = createCalls %d refresh %q", store.createCalls, store.refreshMetric)
	}
}

func TestRoomFissionDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeRoomFissionStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, deleteOK: true}
	handler := NewRoomFissionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "")
	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/roomFission/destroy", strings.NewReader(`{"fissionId":88}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedID != 88 {
		t.Fatalf("deleted id = %d", store.deletedID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricRoomFissions {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

type fakeRoomFissionStore struct {
	user              User
	page              RoomFissionPage
	bundle            RoomFissionBundle
	bundleFound       bool
	contactPage       RoomFissionContactPage
	roomPage          RoomFissionRoomPage
	lastFilter        RoomFissionFilter
	lastContactFilter RoomFissionContactFilter
	lastRoomFilter    RoomFissionRoomFilter
	created           RoomFissionWrite
	updated           RoomFissionWrite
	invite            RoomFissionInviteWrite
	createID          int
	createCalls       int
	quota             SaaSQuotaStatus
	quotaTenantID     int
	quotaMetric       string
	quotaAdditional   int64
	refreshTenantID   int
	refreshMetric     string
	deleteOK          bool
	deletedID         int
	overview          RoomFissionOverview
}

func (s *fakeRoomFissionStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeRoomFissionStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeRoomFissionStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeRoomFissionStore) RoomFissionPage(_ context.Context, filter RoomFissionFilter) (RoomFissionPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeRoomFissionStore) RoomFissionBundleByID(context.Context, int, int) (RoomFissionBundle, bool, error) {
	return s.bundle, s.bundleFound, nil
}

func (s *fakeRoomFissionStore) CreateRoomFission(_ context.Context, values RoomFissionWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeRoomFissionStore) UpdateRoomFission(_ context.Context, _ int, _ int, values RoomFissionWrite) (bool, error) {
	s.updated = values
	return true, nil
}

func (s *fakeRoomFissionStore) DeleteRoomFission(_ context.Context, _ int, id int) (bool, error) {
	s.deletedID = id
	if s.deleteOK {
		return true, nil
	}
	return true, nil
}

func (s *fakeRoomFissionStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" {
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeRoomFissionStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}

func (s *fakeRoomFissionStore) UpsertRoomFissionInvite(_ context.Context, _ int, _ int, values RoomFissionInviteWrite) (bool, error) {
	s.invite = values
	return true, nil
}

func (s *fakeRoomFissionStore) RoomFissionContactPage(_ context.Context, filter RoomFissionContactFilter) (RoomFissionContactPage, error) {
	s.lastContactFilter = filter
	return s.contactPage, nil
}

func (s *fakeRoomFissionStore) RoomFissionRoomPage(_ context.Context, filter RoomFissionRoomFilter) (RoomFissionRoomPage, error) {
	s.lastRoomFilter = filter
	return s.roomPage, nil
}

func (s *fakeRoomFissionStore) RoomFissionOverview(context.Context, int, int) (RoomFissionOverview, error) {
	return s.overview, nil
}

func (s *fakeRoomFissionStore) WriteOffRoomFissionContact(context.Context, int, int, int) (bool, error) {
	return true, nil
}
