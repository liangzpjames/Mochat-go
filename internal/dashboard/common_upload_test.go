package dashboard

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommonDashboardUploadWritesGeneratedFile(t *testing.T) {
	root := t.TempDir()
	handler := NewCommonUploadHandler(root, "http://api.example.com", HeaderUserIDResolver{})
	handler.now = func() time.Time { return time.Date(2026, 7, 3, 14, 35, 0, 0, time.Local) }
	body, contentType := multipartBodyWithFields(t, "avatar.png", "image/png", "PNGDATA", map[string]string{"name": "头像.png"})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/common/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.DashboardUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	bodyMap := decodeBody(t, rec.Body.Bytes())
	data := bodyMap["data"].(map[string]any)
	if data["name"] != "头像.png" || data["type"] != "image/png" {
		t.Fatalf("data = %#v", data)
	}
	path := data["path"].(string)
	if !strings.HasPrefix(path, "2026/0703/1435/") || !strings.HasSuffix(path, ".png") {
		t.Fatalf("path = %q", path)
	}
	if data["fullPath"] != "http://api.example.com/static/"+path {
		t.Fatalf("fullPath = %v", data["fullPath"])
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "PNGDATA" {
		t.Fatalf("content = %q", content)
	}
}

func TestCommonSidebarUploadUsesPathAndHashesChineseName(t *testing.T) {
	root := t.TempDir()
	handler := NewCommonUploadHandler(root, "http://api.example.com", HeaderUserIDResolver{})
	body, contentType := multipartBodyWithFields(t, "原始.txt", "text/plain", "hello", map[string]string{"path": "chat/2026", "name": "中文文件名.txt"})

	req := httptest.NewRequest(http.MethodPost, "/sidebar/common/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.SidebarUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	hash := md5.Sum([]byte("中文文件名.txt"))
	wantName := hex.EncodeToString(hash[:])
	bodyMap := decodeBody(t, rec.Body.Bytes())
	data := bodyMap["data"].(map[string]any)
	if data["name"] != wantName || data["path"] != "chat/2026/"+wantName {
		t.Fatalf("data = %#v", data)
	}
	content, err := os.ReadFile(filepath.Join(root, "chat", "2026", wantName))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello" {
		t.Fatalf("content = %q", content)
	}
}

func TestCommonSidebarUploadRejectsDuplicateAndUnsafePath(t *testing.T) {
	root := t.TempDir()
	handler := NewCommonUploadHandler(root, "http://api.example.com", HeaderUserIDResolver{})
	if err := os.MkdirAll(filepath.Join(root, "chat"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "chat", "a.txt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	body, contentType := multipartBodyWithFields(t, "a.txt", "text/plain", "new", map[string]string{"path": "chat"})
	req := httptest.NewRequest(http.MethodPost, "/sidebar/common/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec := httptest.NewRecorder()
	handler.SidebarUpload(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate status = %d body = %s", rec.Code, rec.Body.String())
	}
	if decodeBody(t, rec.Body.Bytes())["msg"] != "上传失败:已经存在此文件名的文件" {
		t.Fatalf("duplicate body = %s", rec.Body.String())
	}

	body, contentType = multipartBodyWithFields(t, "a.txt", "text/plain", "new", map[string]string{"path": "../escape"})
	req = httptest.NewRequest(http.MethodPost, "/sidebar/common/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Mochat-Go-User-ID", "9")
	rec = httptest.NewRecorder()
	handler.SidebarUpload(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsafe status = %d body = %s", rec.Code, rec.Body.String())
	}
	if decodeBody(t, rec.Body.Bytes())["msg"] != "文件路径非法" {
		t.Fatalf("unsafe body = %s", rec.Body.String())
	}
}

func TestCommonUploadRejectsUnsupportedExtension(t *testing.T) {
	handler := NewCommonUploadHandler(t.TempDir(), "http://api.example.com", HeaderUserIDResolver{})
	body, contentType := multipartBodyWithFields(t, "script.exe", "application/octet-stream", "MZ", nil)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/common/uploadFile", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.DashboardUploadFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if decodeBody(t, rec.Body.Bytes())["msg"] != "文件类型不合法" {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestCommonUploadRecordsSaaSStorageUsage(t *testing.T) {
	root := t.TempDir()
	store := &fakeCommonUploadStore{
		users: map[int]User{1: {ID: 1, TenantID: 9}},
		quota: SaaSQuotaStatus{
			Metric:   SaaSMetricStorage,
			TenantID: 9,
			Current:  0,
			Limit:    2,
		},
	}
	handler := NewCommonUploadHandlerWithStore(root, "http://api.example.com", HeaderUserIDResolver{}, store)
	handler.now = func() time.Time { return time.Date(2026, 7, 3, 14, 35, 0, 0, time.Local) }
	body, contentType := multipartBodyWithFields(t, "avatar.png", "image/png", "PNGDATA", nil)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/common/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.DashboardUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if store.quotaTenantID != 9 || store.quotaAdditionalBytes != int64(len("PNGDATA")) {
		t.Fatalf("quota check tenant=%d additional=%d", store.quotaTenantID, store.quotaAdditionalBytes)
	}
	if store.refreshMetric != SaaSMetricStorage || store.refreshTenantID != 9 {
		t.Fatalf("refresh tenant=%d metric=%s", store.refreshTenantID, store.refreshMetric)
	}
	if store.record.TenantID != 9 || store.record.UserID != 1 || store.record.Source != "dashboard.common.upload" || store.record.SizeBytes != int64(len("PNGDATA")) {
		t.Fatalf("record = %#v", store.record)
	}
	if store.record.RelativePath == "" || store.record.ContentType != "image/png" || store.record.OriginalName != "avatar.png" {
		t.Fatalf("record = %#v", store.record)
	}
}

func TestCommonUploadRejectsSaaSStorageQuotaBeforeWrite(t *testing.T) {
	root := t.TempDir()
	store := &fakeCommonUploadStore{
		users: map[int]User{1: {ID: 1, TenantID: 9}},
		quota: SaaSQuotaStatus{
			Metric:     SaaSMetricStorage,
			TenantID:   9,
			Current:    1,
			Limit:      1,
			Additional: 1,
		},
	}
	handler := NewCommonUploadHandlerWithStore(root, "http://api.example.com", HeaderUserIDResolver{}, store)
	body, contentType := multipartBodyWithFields(t, "avatar.png", "image/png", "PNGDATA", nil)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/common/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.DashboardUpload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if decodeBody(t, rec.Body.Bytes())["msg"] != "套餐额度已达上限：素材存储 1/1" {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if store.record.RelativePath != "" {
		t.Fatalf("record should not be written: %#v", store.record)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("files were written before quota rejection: %#v", entries)
	}
}

func multipartBodyWithFields(t *testing.T, filename string, contentType string, content string, fields map[string]string) (io.Reader, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

type fakeCommonUploadStore struct {
	users                map[int]User
	employees            map[int]SidebarEmployee
	tenantByCorp         map[int]int
	quota                SaaSQuotaStatus
	quotaTenantID        int
	quotaAdditionalBytes int64
	record               CommonUploadStorageObject
	refreshTenantID      int
	refreshMetric        string
}

func (s *fakeCommonUploadStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeCommonUploadStore) SidebarEmployeeByID(_ context.Context, employeeID int) (SidebarEmployee, bool, error) {
	employee, ok := s.employees[employeeID]
	return employee, ok, nil
}

func (s *fakeCommonUploadStore) TenantIDByCorpID(_ context.Context, corpID int) (int, error) {
	return s.tenantByCorp[corpID], nil
}

func (s *fakeCommonUploadStore) SaaSStorageQuotaStatus(_ context.Context, tenantID int, additionalBytes int64) (SaaSQuotaStatus, error) {
	s.quotaTenantID = tenantID
	s.quotaAdditionalBytes = additionalBytes
	return s.quota, nil
}

func (s *fakeCommonUploadStore) RecordCommonUploadStorageObject(_ context.Context, object CommonUploadStorageObject) error {
	s.record = object
	return nil
}

func (s *fakeCommonUploadStore) RefreshSaaSUsageCounter(_ context.Context, tenantID int, metric string) error {
	s.refreshTenantID = tenantID
	s.refreshMetric = metric
	return nil
}
