package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardAgentStoreCreatesWorkAgentFromWeComDetail(t *testing.T) {
	store := &fakeDashboardAgentStore{
		user:       User{ID: 1, TenantID: 1, Status: 1},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wx-corp"},
	}
	wecom := &fakeDashboardAgentWeCom{
		detail: WorkAgentDetail{
			Name:               "Go迁移应用",
			SquareLogoURL:      "https://example.com/logo.png",
			Description:        "应用描述",
			Close:              0,
			RedirectDomain:     "app.example.com",
			ReportLocationFlag: 1,
			IsReportEnter:      1,
			HomeURL:            "https://app.example.com/home",
		},
	}
	handler := NewDashboardAgentHandler(store, staticDashboardAgentCache("7-11"), HeaderUserIDResolver{}, wecom)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/agent/store", strings.NewReader(`{"wxAgentId":"1000003","wxSecret":"agent-secret-created","type":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 200 {
		t.Fatalf("body = %+v", body)
	}
	if store.created.CorpID != 7 || store.created.WXAgentID != "1000003" || store.created.WXSecret != "agent-secret-created" || store.created.Type != 1 {
		t.Fatalf("created values = %+v", store.created)
	}
	if store.createdDetail.Name != "Go迁移应用" || store.createdDetail.RedirectDomain != "app.example.com" {
		t.Fatalf("created detail = %+v", store.createdDetail)
	}
	if wecom.wxSecret != "agent-secret-created" || wecom.wxAgentID != "1000003" || wecom.credential.WXCorpID != "wx-corp" {
		t.Fatalf("wecom call = credential %+v secret %q agent %q", wecom.credential, wecom.wxSecret, wecom.wxAgentID)
	}
}

func TestDashboardAgentStoreRejectsSaaSAgentQuotaExceeded(t *testing.T) {
	base := &fakeDashboardAgentStore{
		user:       User{ID: 1, TenantID: 1, Status: 1},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wx-corp"},
	}
	store := &fakeDashboardAgentQuotaStore{
		fakeDashboardAgentStore: base,
		quota:                   SaaSQuotaStatus{Metric: SaaSMetricAgents, TenantID: 1, Current: 3, Limit: 3, Additional: 1},
	}
	wecom := &fakeDashboardAgentWeCom{}
	handler := NewDashboardAgentHandler(store, staticDashboardAgentCache("7-11"), HeaderUserIDResolver{}, wecom)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/agent/store", strings.NewReader(`{"wxAgentId":"1000003","wxSecret":"agent-secret-created","type":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Msg != "套餐额度已达上限：应用数 3/3" {
		t.Fatalf("body = %+v", body)
	}
	if store.created.WXAgentID != "" {
		t.Fatalf("CreateWorkAgent should not run")
	}
	if wecom.wxAgentID != "" {
		t.Fatalf("wecom should not be called")
	}
}

func TestDashboardAgentStoreRefreshesSaaSAgentUsage(t *testing.T) {
	base := &fakeDashboardAgentStore{
		user:       User{ID: 1, TenantID: 1, Status: 1},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wx-corp"},
	}
	store := &fakeDashboardAgentQuotaStore{
		fakeDashboardAgentStore: base,
		quota:                   SaaSQuotaStatus{Metric: SaaSMetricAgents, TenantID: 1, Current: 2, Limit: 3, Additional: 1},
	}
	handler := NewDashboardAgentHandler(store, staticDashboardAgentCache("7-11"), HeaderUserIDResolver{}, &fakeDashboardAgentWeCom{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/agent/store", strings.NewReader(`{"wxAgentId":"1000003","wxSecret":"agent-secret-created","type":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.quotaMetric != SaaSMetricAgents || store.refreshMetric != SaaSMetricAgents {
		t.Fatalf("quota metric=%q refresh=%q", store.quotaMetric, store.refreshMetric)
	}
}

func TestDashboardAgentStoreRequiresSelectedCorp(t *testing.T) {
	store := &fakeDashboardAgentStore{user: User{ID: 1, TenantID: 1, Status: 1}}
	handler := NewDashboardAgentHandler(store, staticDashboardAgentCache(""), HeaderUserIDResolver{}, &fakeDashboardAgentWeCom{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/agent/store", strings.NewReader(`{"wxAgentId":"1000003","wxSecret":"agent-secret-created","type":1}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.created.WXAgentID != "" {
		t.Fatalf("unexpected create: %+v", store.created)
	}
}

func TestDashboardAgentStoreDoesNotCreateWhenWeComFails(t *testing.T) {
	store := &fakeDashboardAgentStore{
		user:       User{ID: 1, TenantID: 1, Status: 1},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wx-corp"},
	}
	handler := NewDashboardAgentHandler(store, staticDashboardAgentCache("7-11"), HeaderUserIDResolver{}, &fakeDashboardAgentWeCom{err: fmt.Errorf("not found")})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/agent/store", strings.NewReader(`{"wxAgentId":"bad-agent","wxSecret":"bad-secret","type":1}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.created.WXAgentID != "" {
		t.Fatalf("unexpected create: %+v", store.created)
	}
}

type fakeDashboardAgentStore struct {
	user          User
	credential    RoomWelcomeCorpCredential
	created       WorkAgentWriteValues
	createdDetail WorkAgentDetail
}

func (s *fakeDashboardAgentStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if s.user.ID == userID {
		return s.user, true, nil
	}
	return User{}, false, nil
}

func (s *fakeDashboardAgentStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 11, nil
}

func (s *fakeDashboardAgentStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 0, 0, false, nil
}

func (s *fakeDashboardAgentStore) RoomWelcomeCorpCredentialByID(_ context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error) {
	if s.credential.CorpID == corpID {
		return s.credential, true, nil
	}
	return RoomWelcomeCorpCredential{}, false, nil
}

func (s *fakeDashboardAgentStore) CreateWorkAgent(_ context.Context, values WorkAgentWriteValues, detail WorkAgentDetail) (int, error) {
	s.created = values
	s.createdDetail = detail
	return 123, nil
}

type fakeDashboardAgentQuotaStore struct {
	*fakeDashboardAgentStore
	quota         SaaSQuotaStatus
	quotaMetric   string
	refreshMetric string
}

func (s *fakeDashboardAgentQuotaStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaMetric = metric
	status := s.quota
	status.TenantID = tenantID
	status.Metric = metric
	status.Additional = additional
	return status, nil
}

func (s *fakeDashboardAgentQuotaStore) RefreshSaaSUsageCounter(_ context.Context, _ int, metric string) error {
	s.refreshMetric = metric
	return nil
}

type fakeDashboardAgentWeCom struct {
	detail     WorkAgentDetail
	err        error
	credential RoomWelcomeCorpCredential
	wxSecret   string
	wxAgentID  string
}

func (c *fakeDashboardAgentWeCom) AgentDetail(_ context.Context, credential RoomWelcomeCorpCredential, wxSecret string, wxAgentID string) (WorkAgentDetail, error) {
	c.credential = credential
	c.wxSecret = wxSecret
	c.wxAgentID = wxAgentID
	return c.detail, c.err
}

type staticDashboardAgentCache string

func (c staticDashboardAgentCache) UserCorpCache(context.Context, int) (string, error) {
	return string(c), nil
}
