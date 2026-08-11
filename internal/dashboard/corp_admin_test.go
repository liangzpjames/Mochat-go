package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCorpAdminIndexReturnsPHPCompatiblePage(t *testing.T) {
	store := &fakeCorpAdminStore{
		users: map[int]User{
			1: {ID: 1, TenantID: 10, IsSuperAdmin: 1},
		},
		listPage: CorpListPage{
			Items:     []CorpDetail{{ID: 7, Name: "迁移企业", WxCorpID: "wx-migrate", CreatedAt: "2026-07-02 10:00:00"}},
			Total:     1,
			TotalPage: 1,
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewCorpAdminHandler(store, staticAdminCache("7-0"), HeaderUserIDResolver{}, authorizer)

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/corp/index?corpName=迁移&page=2&perPage=5", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/corp/index#get" {
		t.Fatalf("permission key = %q", authorizer.permissionKey)
	}
	if store.lastFilter.TenantID != 10 || !store.lastFilter.SuperAdmin || store.lastFilter.CorpName != "迁移" || store.lastFilter.Page != 2 || store.lastFilter.PerPage != 5 {
		t.Fatalf("filter = %+v", store.lastFilter)
	}

	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	page := data["page"].(map[string]any)
	if int(page["total"].(float64)) != 1 || int(page["totalPage"].(float64)) != 1 || int(page["perPage"].(float64)) != 5 {
		t.Fatalf("page = %#v", page)
	}
	list := data["list"].([]any)
	row := list[0].(map[string]any)
	if int(row["corpId"].(float64)) != 7 || row["corpName"] != "迁移企业" || row["wxCorpId"] != "wx-migrate" {
		t.Fatalf("row = %#v", row)
	}
	if row["chatStatus"].(float64) != 0 || row["messageCreatedAt"] != "" {
		t.Fatalf("compat fields missing: %#v", row)
	}
}

func TestCorpAdminIndexRestrictsNormalUserToLoginCorpIDs(t *testing.T) {
	store := &fakeCorpAdminStore{
		users: map[int]User{
			2: {ID: 2, TenantID: 10, IsSuperAdmin: 0},
		},
		listPage: CorpListPage{Items: []CorpDetail{}, Total: 0, TotalPage: 0},
	}
	handler := NewCorpAdminHandler(store, staticAdminCache("5-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/corp/index", nil, 2, 10, 5, 99)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !reflect.DeepEqual(store.lastFilter.CorpIDs, []int{5}) {
		t.Fatalf("corpIDs = %#v", store.lastFilter.CorpIDs)
	}
	if store.lastFilter.SuperAdmin {
		t.Fatalf("normal user filter marked superadmin")
	}
}

func TestCorpAdminShowAppendsCallbackCID(t *testing.T) {
	store := &fakeCorpAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		details: map[int]CorpDetail{
			3: {
				ID:             3,
				Name:           "详情企业",
				WxCorpID:       "wx-detail",
				EmployeeSecret: "employee-secret",
				ContactSecret:  "contact-secret",
				EventCallback:  "https://api.example.com/weWork/callback",
				Token:          "token",
				EncodingAESKey: "aes-key",
				TenantID:       10,
			},
		},
	}
	handler := NewCorpAdminHandler(store, staticAdminCache("3-0"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/corp/show?corpId=3", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	if data["corpName"] != "详情企业" || data["wxCorpId"] != "wx-detail" {
		t.Fatalf("data = %#v", data)
	}
	if data["eventCallback"] != "https://api.example.com/weWork/callback?cid=3" {
		t.Fatalf("eventCallback = %v", data["eventCallback"])
	}
	if _, ok := data["createdAt"]; ok {
		t.Fatalf("createdAt should be omitted: %#v", data)
	}
}

func TestCorpAdminShowRejectsCorpOutsideCurrentUserScope(t *testing.T) {
	store := &fakeCorpAdminStore{
		users: map[int]User{2: {ID: 2, TenantID: 10}},
		details: map[int]CorpDetail{
			4: {ID: 4, Name: "鍏朵粬浼佷笟", TenantID: 10},
		},
	}
	handler := NewCorpAdminHandler(store, staticAdminCache("3-9"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/corp/show?corpId=4", nil, 2, 10, 3, 99)
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()
	handler.Show(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestCorpAdminUpdateWritesEditableFields(t *testing.T) {
	store := &fakeCorpAdminStore{
		users:   map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		details: map[int]CorpDetail{3: {ID: 3, Name: "旧企业", TenantID: 10}},
	}
	handler := NewCorpAdminHandler(store, staticAdminCache("3-0"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTestAs(http.MethodPut, "/dashboard/corp/update", bytes.NewBufferString(`{"corpId":3,"corpName":"新企业","wxCorpId":"wx-new","employeeSecret":"employee-secret","contactSecret":"contact-secret"}`), 1, 10, 3, 99)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.updatedCorpID != 3 {
		t.Fatalf("updatedCorpID = %d", store.updatedCorpID)
	}
	if store.updatedValues.Name != "新企业" || store.updatedValues.WxCorpID != "wx-new" {
		t.Fatalf("updatedValues = %+v", store.updatedValues)
	}
}

func TestCorpAdminUpdateRejectsCorpOutsideTenant(t *testing.T) {
	store := &fakeCorpAdminStore{
		users:   map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		details: map[int]CorpDetail{3: {ID: 3, Name: "other tenant corp", TenantID: 11}},
	}
	handler := NewCorpAdminHandler(store, staticAdminCache("3-0"), HeaderUserIDResolver{}, &recordingAuthorizer{})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/corp/update", bytes.NewBufferString(`{"corpId":3,"corpName":"updated corp","wxCorpId":"wx-new","employeeSecret":"employee-secret","contactSecret":"contact-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Update(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.updatedCorpID != 0 {
		t.Fatalf("update should not run, got corpID %d", store.updatedCorpID)
	}
}

func TestCorpAdminUpdateRejectsPermissionDenied(t *testing.T) {
	store := &fakeCorpAdminStore{
		users:   map[int]User{2: {ID: 2, TenantID: 10, IsSuperAdmin: 0}},
		details: map[int]CorpDetail{3: {ID: 3, Name: "旧企业"}},
	}
	handler := NewCorpAdminHandler(store, staticAdminCache("3-9"), HeaderUserIDResolver{}, &recordingAuthorizer{err: ErrPermissionDenied})

	req := authenticatedDashboardRequestForTestAs(http.MethodPut, "/dashboard/corp/update", bytes.NewBufferString(`{"corpId":3,"corpName":"新企业","wxCorpId":"wx-new","employeeSecret":"employee-secret","contactSecret":"contact-secret"}`), 2, 10, 3, 99)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "2")
	rec := httptest.NewRecorder()
	handler.Update(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.updatedCorpID != 0 {
		t.Fatalf("update should not run, got corpID %d", store.updatedCorpID)
	}
}

func TestCorpAdminStoreCreatesCorpAndCachesSelection(t *testing.T) {
	store := &fakeCorpAdminStore{
		users:     map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		createdID: 7,
	}
	cache := &fakeCorpCache{}
	employeeQueue := &fakeEmployeeApplyQueue{}
	wecom := &fakeCorpWeComValidator{}
	authorizer := &recordingAuthorizer{}
	handler := NewCorpAdminHandler(store, staticAdminCache(""), HeaderUserIDResolver{}, authorizer).
		WithCacheWriter(cache).
		WithEmployeeApplyQueue(employeeQueue).
		WithWeComValidator(wecom).
		WithAPIBaseURL("https://api.example.com/")

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/corp/store", bytes.NewBufferString(`{"corpName":" 新企业 ","wxCorpId":" wx-new ","employeeSecret":" employee-secret ","contactSecret":" contact-secret "}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/corp/store#post" {
		t.Fatalf("permission key = %q", authorizer.permissionKey)
	}
	if wecom.wxCorpID != "wx-new" || wecom.employeeSecret != "employee-secret" || wecom.contactSecret != "contact-secret" {
		t.Fatalf("wecom validator = %+v", wecom)
	}
	if store.createdValues.Name != "新企业" || store.createdValues.WxCorpID != "wx-new" {
		t.Fatalf("createdValues = %+v", store.createdValues)
	}
	if store.createdValues.EmployeeSecret != "employee-secret" || store.createdValues.ContactSecret != "contact-secret" {
		t.Fatalf("created secrets = %+v", store.createdValues)
	}
	if store.createdValues.EventCallback != "https://api.example.com/weWork/callback" {
		t.Fatalf("event callback = %q", store.createdValues.EventCallback)
	}
	if store.createdValues.TenantID != 10 {
		t.Fatalf("tenantID = %d", store.createdValues.TenantID)
	}
	if store.createdValues.Token == "" || len(store.createdValues.EncodingAESKey) != 43 {
		t.Fatalf("callback secrets = token:%q aes:%q", store.createdValues.Token, store.createdValues.EncodingAESKey)
	}
	if cache.userID != 1 || cache.value != "7-0" {
		t.Fatalf("cache = userID:%d value:%q", cache.userID, cache.value)
	}
	if employeeQueue.event.BindingID != 10 || employeeQueue.event.Source != "dashboard.corp.store" {
		t.Fatalf("employee queue event = %+v", employeeQueue.event)
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["code"].(float64) != 200 {
		t.Fatalf("body = %#v", body)
	}
}

func TestCorpAdminStoreRejectsExistingCorp(t *testing.T) {
	store := &fakeCorpAdminStore{
		users:     map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpCount: 1,
	}
	wecom := &fakeCorpWeComValidator{}
	handler := NewCorpAdminHandler(store, staticAdminCache(""), HeaderUserIDResolver{}, &recordingAuthorizer{}).
		WithWeComValidator(wecom)

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/corp/store", bytes.NewBufferString(`{"corpName":"新企业","wxCorpId":"wx-new","employeeSecret":"employee-secret","contactSecret":"contact-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "只能添加一个企业" {
		t.Fatalf("body = %#v", body)
	}
	if store.createCalled {
		t.Fatalf("CreateCorp should not run")
	}
	if wecom.calls != 0 {
		t.Fatalf("wecom calls = %d", wecom.calls)
	}
}

func TestCorpAdminStoreUsesTenantSaaSQuotaInsteadOfGlobalCorpLimit(t *testing.T) {
	base := &fakeCorpAdminStore{
		users:     map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
		corpCount: 99,
		createdID: 8,
	}
	store := &fakeCorpAdminQuotaStore{
		fakeCorpAdminStore: base,
		quota:              SaaSQuotaStatus{Metric: SaaSMetricCorps, TenantID: 10, Current: 1, Limit: 2, Additional: 1},
	}
	handler := NewCorpAdminHandler(store, staticAdminCache(""), HeaderUserIDResolver{}, &recordingAuthorizer{}).
		WithWeComValidator(&fakeCorpWeComValidator{})

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/corp/store", bytes.NewBufferString(`{"corpName":"第二企业","wxCorpId":"wx-new","employeeSecret":"employee-secret","contactSecret":"contact-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !store.createCalled {
		t.Fatalf("CreateCorp should run")
	}
	if store.quotaMetric != SaaSMetricCorps || store.refreshMetric != SaaSMetricCorps {
		t.Fatalf("quota metric=%q refresh=%q", store.quotaMetric, store.refreshMetric)
	}
}

func TestCorpAdminStoreRejectsSaaSCorpQuotaExceeded(t *testing.T) {
	base := &fakeCorpAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
	}
	store := &fakeCorpAdminQuotaStore{
		fakeCorpAdminStore: base,
		quota:              SaaSQuotaStatus{Metric: SaaSMetricCorps, TenantID: 10, Current: 2, Limit: 2, Additional: 1},
	}
	wecom := &fakeCorpWeComValidator{}
	handler := NewCorpAdminHandler(store, staticAdminCache(""), HeaderUserIDResolver{}, &recordingAuthorizer{}).
		WithWeComValidator(wecom)

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/corp/store", bytes.NewBufferString(`{"corpName":"第三企业","wxCorpId":"wx-new","employeeSecret":"employee-secret","contactSecret":"contact-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "套餐额度已达上限：企业数 2/2" {
		t.Fatalf("body = %#v", body)
	}
	if store.createCalled {
		t.Fatalf("CreateCorp should not run")
	}
	if wecom.calls != 0 {
		t.Fatalf("wecom calls = %d", wecom.calls)
	}
}

func TestCorpAdminStoreRejectsInvalidWeComSecrets(t *testing.T) {
	store := &fakeCorpAdminStore{
		users: map[int]User{1: {ID: 1, TenantID: 10, IsSuperAdmin: 1}},
	}
	handler := NewCorpAdminHandler(store, staticAdminCache(""), HeaderUserIDResolver{}, &recordingAuthorizer{}).
		WithWeComValidator(&fakeCorpWeComValidator{err: errors.New("通讯录管理secret或企业ID无效")})

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/corp/store", bytes.NewBufferString(`{"corpName":"新企业","wxCorpId":"wx-new","employeeSecret":"bad","contactSecret":"contact-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Store(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "通讯录管理secret或企业ID无效" {
		t.Fatalf("body = %#v", body)
	}
	if store.createCalled {
		t.Fatalf("CreateCorp should not run")
	}
}

type fakeCorpAdminStore struct {
	users         map[int]User
	details       map[int]CorpDetail
	listPage      CorpListPage
	lastFilter    CorpListFilter
	corpCount     int
	createdID     int
	createCalled  bool
	createdValues CorpCreateValues
	updatedCorpID int
	updatedValues CorpUpdateValues
}

func (s *fakeCorpAdminStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeCorpAdminStore) EmployeeIDByUserCorp(_ context.Context, userID int, corpID int) (int, error) {
	return 0, nil
}

func (s *fakeCorpAdminStore) FirstEmployeeByUser(_ context.Context, userID int) (int, int, bool, error) {
	return 0, 0, false, nil
}

func (s *fakeCorpAdminStore) CorpList(_ context.Context, filter CorpListFilter) (CorpListPage, error) {
	s.lastFilter = filter
	return s.listPage, nil
}

func (s *fakeCorpAdminStore) CorpDetailByID(_ context.Context, corpID int) (CorpDetail, bool, error) {
	corp, ok := s.details[corpID]
	return corp, ok, nil
}

func (s *fakeCorpAdminStore) CountCorps(context.Context) (int, error) {
	return s.corpCount, nil
}

func (s *fakeCorpAdminStore) CreateCorp(_ context.Context, values CorpCreateValues) (int, error) {
	s.createCalled = true
	s.createdValues = values
	if s.createdID > 0 {
		return s.createdID, nil
	}
	return 1, nil
}

func (s *fakeCorpAdminStore) UpdateCorp(_ context.Context, corpID int, values CorpUpdateValues) error {
	s.updatedCorpID = corpID
	s.updatedValues = values
	return nil
}

type fakeCorpAdminQuotaStore struct {
	*fakeCorpAdminStore
	quota         SaaSQuotaStatus
	quotaMetric   string
	refreshMetric string
}

func (s *fakeCorpAdminQuotaStore) SaaSQuotaStatus(_ context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error) {
	s.quotaMetric = metric
	status := s.quota
	status.TenantID = tenantID
	status.Metric = metric
	status.Additional = additional
	return status, nil
}

func (s *fakeCorpAdminQuotaStore) RefreshSaaSUsageCounter(_ context.Context, _ int, metric string) error {
	s.refreshMetric = metric
	return nil
}

type fakeCorpWeComValidator struct {
	calls          int
	wxCorpID       string
	employeeSecret string
	contactSecret  string
	err            error
}

func (v *fakeCorpWeComValidator) ValidateCorpSecrets(_ context.Context, wxCorpID string, employeeSecret string, contactSecret string) error {
	v.calls++
	v.wxCorpID = wxCorpID
	v.employeeSecret = employeeSecret
	v.contactSecret = contactSecret
	return v.err
}

type fakeEmployeeApplyQueue struct {
	event EmployeeApplyEvent
	err   error
}

func (q *fakeEmployeeApplyQueue) EnqueueEmployeeApply(_ context.Context, event EmployeeApplyEvent) error {
	q.event = event
	return q.err
}

type recordingAuthorizer struct {
	permissionKey  string
	corpID         int
	workEmployeeID int
	access         AccessContext
	accessSet      bool
	err            error
}

func (a *recordingAuthorizer) Resolve(_ context.Context, _ int, permissionKey string, corpID int, workEmployeeID int) (AccessContext, error) {
	a.permissionKey = permissionKey
	a.corpID = corpID
	a.workEmployeeID = workEmployeeID
	if a.accessSet {
		return a.access, a.err
	}
	return AccessContext{}, a.err
}

type staticAdminCache string

func (c staticAdminCache) UserCorpCache(context.Context, int) (string, error) {
	return string(c), nil
}

func decodeBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
