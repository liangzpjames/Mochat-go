package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLotteryShowContactKeepsZeroStatusFilters(t *testing.T) {
	store := &fakeLotteryStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		contactPage: LotteryContactPage{
			Page:    1,
			PerPage: 15,
			Total:   1,
			Items: []LotteryContactItem{{
				ID:             9,
				LotteryID:      12,
				Nickname:       "未完成客户",
				ContactTagsRaw: `[]`,
				Status:         0,
				WriteOff:       0,
			}},
		},
	}
	handler := NewLotteryHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "")
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/lottery/showContact?lotteryId=12&status=0&writeOff=0&page=1&perPage=15", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ShowContact(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastContactFilter.CorpID != 7 || store.lastContactFilter.LotteryID != 12 || store.lastContactFilter.Status != 0 || store.lastContactFilter.WriteOff != 0 {
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

func TestLotteryStorePreservesPrizeJSON(t *testing.T) {
	store := &fakeLotteryStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, createID: 88}
	handler := NewLotteryHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "https://op.example")
	body := `{"name":"抽奖活动","description":"活动说明","contactTags":[7,8],"prizeSet":[{"name":"一等奖"}],"exchangeSet":{"type":1},"drawSet":{"daily":1},"winSet":{"limit":1},"corpCard":{"name":"企业名片"},"isShow":1}`
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/lottery/store", strings.NewReader(body))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.Name != "抽奖活动" || store.created.IsShow != 1 {
		t.Fatalf("created = %#v", store.created)
	}
	if !strings.Contains(store.created.ContactTagsRaw, "7") || !strings.Contains(store.created.PrizeSetRaw, "一等奖") || !strings.Contains(store.created.ExchangeSetRaw, "type") || !strings.Contains(store.created.CorpCardRaw, "企业名片") {
		t.Fatalf("json raw = %#v", store.created)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricLotteries {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

func TestLotteryStoreRejectsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeLotteryStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricLotteries,
			TenantID:   8,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewLotteryHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "https://op.example")
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/lottery/store", strings.NewReader(`{"name":"额度外抽奖活动"}`))
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
	if envelope.Msg != "套餐额度已达上限：抽奖活动数 1/1" {
		t.Fatalf("envelope = %#v", envelope)
	}
	if store.quotaTenantID != 8 || store.quotaMetric != SaaSMetricLotteries || store.quotaAdditional != 1 {
		t.Fatalf("quota = tenant %d metric %q additional %d", store.quotaTenantID, store.quotaMetric, store.quotaAdditional)
	}
	if store.createCalls != 0 || store.refreshMetric != "" {
		t.Fatalf("side effects = createCalls %d refresh %q", store.createCalls, store.refreshMetric)
	}
}

func TestLotteryDestroyRefreshesSaaSUsage(t *testing.T) {
	store := &fakeLotteryStore{user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1}, deleteOK: true}
	handler := NewLotteryHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil, "https://op.example")
	req := authenticatedDashboardRequestForTest(http.MethodDelete, "/dashboard/lottery/destroy", strings.NewReader(`{"lotteryId":88}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Destroy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedID != 88 {
		t.Fatalf("deleted id = %d", store.deletedID)
	}
	if store.refreshTenantID != 8 || store.refreshMetric != SaaSMetricLotteries {
		t.Fatalf("refresh = tenant %d metric %q", store.refreshTenantID, store.refreshMetric)
	}
}

type fakeLotteryStore struct {
	user              User
	page              LotteryPage
	item              LotteryItem
	prize             LotteryPrizeItem
	itemFound         bool
	contactPage       LotteryContactPage
	lastFilter        LotteryFilter
	lastContactFilter LotteryContactFilter
	created           LotteryWrite
	updated           LotteryWrite
	createID          int
	createCalls       int
	batchLotteryID    int
	batchContactIDs   []int
	batchTagIDs       []int
	quota             SaaSQuotaStatus
	quotaTenantID     int
	quotaMetric       string
	quotaAdditional   int64
	refreshTenantID   int
	refreshMetric     string
	deleteOK          bool
	deletedID         int
}

func (s *fakeLotteryStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeLotteryStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeLotteryStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeLotteryStore) LotteryPage(_ context.Context, filter LotteryFilter) (LotteryPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeLotteryStore) LotteryByID(context.Context, int, int) (LotteryItem, LotteryPrizeItem, bool, error) {
	return s.item, s.prize, s.itemFound, nil
}

func (s *fakeLotteryStore) CreateLottery(_ context.Context, values LotteryWrite) (int, error) {
	s.createCalls++
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeLotteryStore) UpdateLottery(_ context.Context, _ int, _ int, values LotteryWrite) (bool, error) {
	s.updated = values
	return true, nil
}

func (s *fakeLotteryStore) DeleteLottery(_ context.Context, _ int, id int) (bool, error) {
	s.deletedID = id
	if s.deleteOK {
		return true, nil
	}
	return true, nil
}

func (s *fakeLotteryStore) LotteryContactPage(_ context.Context, filter LotteryContactFilter) (LotteryContactPage, error) {
	s.lastContactFilter = filter
	return s.contactPage, nil
}

func (s *fakeLotteryStore) WriteOffLotteryContact(context.Context, int, int, int) (bool, error) {
	return true, nil
}

func (s *fakeLotteryStore) BatchTagLotteryContacts(_ context.Context, _ int, lotteryID int, contactIDs []int, tagIDs []int) (int, error) {
	s.batchLotteryID = lotteryID
	s.batchContactIDs = append([]int{}, contactIDs...)
	s.batchTagIDs = append([]int{}, tagIDs...)
	return len(tagIDs), nil
}

func (s *fakeLotteryStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaMetric = metric
	s.quotaAdditional = additional
	if s.quota.Metric != "" {
		return s.quota, nil
	}
	return SaaSQuotaStatus{Metric: metric, TenantID: tenantID, Additional: additional}, nil
}

func (s *fakeLotteryStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
