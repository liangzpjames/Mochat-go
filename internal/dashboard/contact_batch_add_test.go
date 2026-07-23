package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContactBatchAddDetailReturnsSidebarPayload(t *testing.T) {
	store := &fakeContactBatchAddStore{
		employee: SidebarEmployee{ID: 7, CorpID: 9, LogUserID: 3},
		detail: ContactBatchAddDetail{
			EmployeeName: "员工A",
			List: []ContactBatchAddContact{
				{ID: 1, Phone: "13800000001", Status: 1},
				{ID: 2, Phone: "13800000002", Status: 2},
			},
		},
	}
	handler := NewContactBatchAddHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})
	req := httptest.NewRequest(http.MethodGet, "/sidebar/contactBatchAdd/detail?batchId=55&status=4", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "7")
	rec := httptest.NewRecorder()

	handler.Detail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			EmployeeName string           `json:"employeeName"`
			List         []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 200 || envelope.Data.EmployeeName != "员工A" || len(envelope.Data.List) != 2 {
		t.Fatalf("envelope = %#v", envelope)
	}
	if envelope.Data.List[0]["status"] != "待添加" || envelope.Data.List[1]["status"] != "待通过" {
		t.Fatalf("list = %#v", envelope.Data.List)
	}
	if store.employeeID != 7 || store.batchID != 55 || store.status != 4 {
		t.Fatalf("store args employee=%d batch=%d status=%d", store.employeeID, store.batchID, store.status)
	}
}

func TestContactBatchAddDashboardIndexReturnsLaravelPage(t *testing.T) {
	store := &fakeContactBatchAddDashboardStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		page: ContactBatchAddPage{
			Page:    2,
			PerPage: 1,
			Total:   2,
			Items: []ContactBatchAddItem{{
				ID:         11,
				RecordID:   7,
				Phone:      "13800000001",
				Status:     1,
				EmployeeID: 31,
				Remark:     "张三",
				TagIDs:     []int{5},
			}},
		},
		employees: map[int]ContactBatchAddEmployee{31: {ID: 31, Name: "员工A"}},
		tags:      map[int]ContactBatchAddTag{5: {ID: 5, Name: "重点客户"}},
	}
	handler := NewContactBatchAddDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/contactBatchAdd/index?page=2&perPage=1&status=1&searchKey=138", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	req.Host = "api.example.com"
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

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
	if item["phone"] != "13800000001" || item["statusText"] != "待添加" {
		t.Fatalf("item = %#v", item)
	}
	if store.lastFilter.CorpID != 7 || store.lastFilter.Status != 1 || !store.lastFilter.HasStatus || store.lastFilter.SearchKey != "138" {
		t.Fatalf("filter = %#v", store.lastFilter)
	}
}

func TestContactBatchAddDashboardImportStoreAcceptsJSONContacts(t *testing.T) {
	store := &fakeContactBatchAddDashboardStore{user: User{ID: 1, IsSuperAdmin: 1}}
	handler := NewContactBatchAddDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/contactBatchAdd/importStore", strings.NewReader(`{"title":"导入任务","allotEmployee":[31],"tags":[5],"contacts":[{"phone":"13800000001","remark":"张三"},{"phone":"bad"}]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ImportStore(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			SuccessNum int `json:"successNum"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 200 || envelope.Data.SuccessNum != 1 {
		t.Fatalf("envelope = %#v", envelope)
	}
	if store.created.CorpID != 7 || store.created.Title != "导入任务" || len(store.created.Rows) != 1 || store.created.Rows[0].Remark != "张三" {
		t.Fatalf("created = %#v", store.created)
	}
}

func TestContactBatchAddDashboardImportStoreRecordsMultipartFileStorage(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("title", "导入任务")
	_ = writer.WriteField("allotEmployee[]", "31")
	_ = writer.WriteField("tags[]", "5")
	part, err := writer.CreateFormFile("file", "contacts.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("13800000001,张三\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	store := &fakeContactBatchAddDashboardStore{
		user: User{ID: 1, TenantID: 8, IsSuperAdmin: 1},
		quota: SaaSQuotaStatus{
			Metric:   SaaSMetricStorage,
			TenantID: 8,
			Current:  0,
			Limit:    10,
		},
	}
	handler := NewContactBatchAddDashboardHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil).
		WithFileStorage(root, "http://api.example.com")
	handler.now = func() time.Time { return time.Date(2026, 7, 6, 19, 45, 0, 0, time.UTC) }
	req := httptest.NewRequest(http.MethodPost, "/dashboard/contactBatchAdd/importStore", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.ImportStore(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.FileName != "contacts.csv" || !strings.HasPrefix(store.created.FileURL, "contactBatchAdd/import/2026/0706/") || !strings.HasSuffix(store.created.FileURL, ".csv") {
		t.Fatalf("created file = name %q url %q", store.created.FileName, store.created.FileURL)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(store.created.FileURL)))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "13800000001,张三\n" {
		t.Fatalf("stored file = %q", string(raw))
	}
	if store.storageRecord.TenantID != 8 || store.storageRecord.UserID != 1 || store.storageRecord.CorpID != 7 || store.storageRecord.Source != "dashboard.contactBatchAdd.importStore" {
		t.Fatalf("storage owner = %+v", store.storageRecord)
	}
	if store.storageRecord.RelativePath != store.created.FileURL || store.storageRecord.OriginalName != "contacts.csv" || store.storageRecord.SizeBytes != int64(len(raw)) {
		t.Fatalf("storage record = %+v", store.storageRecord)
	}
	if store.storageRefreshTenantID != 8 || store.storageRefreshMetric != SaaSMetricStorage {
		t.Fatalf("refresh tenant=%d metric=%q", store.storageRefreshTenantID, store.storageRefreshMetric)
	}
}

type fakeContactBatchAddStore struct {
	employee SidebarEmployee
	detail   ContactBatchAddDetail

	employeeID int
	batchID    int
	status     int
}

func (s *fakeContactBatchAddStore) SidebarEmployeeByID(_ context.Context, employeeID int) (SidebarEmployee, bool, error) {
	if employeeID != s.employee.ID {
		return SidebarEmployee{}, false, nil
	}
	return s.employee, true, nil
}

func (s *fakeContactBatchAddStore) ContactBatchAddDetail(_ context.Context, employeeID int, batchID int, status int) (ContactBatchAddDetail, error) {
	s.employeeID = employeeID
	s.batchID = batchID
	s.status = status
	return s.detail, nil
}

type fakeContactBatchAddDashboardStore struct {
	user      User
	page      ContactBatchAddPage
	records   ContactBatchAddImportRecordPage
	employees map[int]ContactBatchAddEmployee
	tags      map[int]ContactBatchAddTag
	config    ContactBatchAddConfig
	dashboard ContactBatchAddDashboardData

	lastFilter ContactBatchAddFilter
	created    ContactBatchAddImportWrite

	quota                  SaaSQuotaStatus
	storageQuotaTenantID   int
	storageQuotaBytes      int64
	storageRecord          CommonUploadStorageObject
	storageRefreshTenantID int
	storageRefreshMetric   string
}

func (s *fakeContactBatchAddDashboardStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeContactBatchAddDashboardStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeContactBatchAddDashboardStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeContactBatchAddDashboardStore) ContactBatchAddPage(_ context.Context, filter ContactBatchAddFilter) (ContactBatchAddPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeContactBatchAddDashboardStore) ContactBatchAddImportRecordPage(context.Context, int, int, int) (ContactBatchAddImportRecordPage, error) {
	return s.records, nil
}

func (s *fakeContactBatchAddDashboardStore) ContactBatchAddEmployeesByIDs(context.Context, []int) (map[int]ContactBatchAddEmployee, error) {
	return s.employees, nil
}

func (s *fakeContactBatchAddDashboardStore) ContactBatchAddTagsByIDs(context.Context, int, []int) (map[int]ContactBatchAddTag, error) {
	return s.tags, nil
}

func (s *fakeContactBatchAddDashboardStore) ContactBatchAddConfig(context.Context, int) (ContactBatchAddConfig, bool, error) {
	return s.config, true, nil
}

func (s *fakeContactBatchAddDashboardStore) UpsertContactBatchAddConfig(context.Context, int, ContactBatchAddConfig) error {
	return nil
}

func (s *fakeContactBatchAddDashboardStore) CreateContactBatchAddImport(_ context.Context, values ContactBatchAddImportWrite) (int, error) {
	s.created = values
	return len(values.Rows), nil
}

func (s *fakeContactBatchAddDashboardStore) AllotContactBatchAddImports(context.Context, int, []int, []int, int) (int, error) {
	return 1, nil
}

func (s *fakeContactBatchAddDashboardStore) DeleteContactBatchAddImports(context.Context, int, []int) (int, error) {
	return 1, nil
}

func (s *fakeContactBatchAddDashboardStore) DeleteContactBatchAddImportRecords(context.Context, int, []int) (int, int, error) {
	return 1, 2, nil
}

func (s *fakeContactBatchAddDashboardStore) ContactBatchAddDashboard(context.Context, int, int, int) (ContactBatchAddDashboardData, error) {
	return s.dashboard, nil
}

func (s *fakeContactBatchAddDashboardStore) SaaSStorageQuotaStatus(_ context.Context, tenantID int, additionalBytes int64) (SaaSQuotaStatus, error) {
	s.storageQuotaTenantID = tenantID
	s.storageQuotaBytes = additionalBytes
	if s.quota.Metric == "" {
		return SaaSQuotaStatus{Metric: SaaSMetricStorage, TenantID: tenantID}, nil
	}
	return s.quota, nil
}

func (s *fakeContactBatchAddDashboardStore) RecordCommonUploadStorageObject(_ context.Context, object CommonUploadStorageObject) error {
	s.storageRecord = object
	return nil
}

func (s *fakeContactBatchAddDashboardStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.storageRefreshTenantID = tenantID
	s.storageRefreshMetric = metric
	return nil
}
