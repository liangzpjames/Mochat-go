package dashboard

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
)

func TestAgentTxtVerifyUploadWritesTextFile(t *testing.T) {
	root := t.TempDir()
	handler := NewAgentTxtVerifyUploadHandler(root)
	body, contentType := multipartBody(t, "WW_verify_ABCDEF1234567890.txt", "text/plain", "ABCDEF1234567890")

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/agent/txtVerifyUpload", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	content, err := os.ReadFile(filepath.Join(root, "wx_txt_verify", "WW_verify_ABCDEF1234567890.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "ABCDEF1234567890" {
		t.Fatalf("content = %q", content)
	}
	bodyMap := decodeBody(t, rec.Body.Bytes())
	if bodyMap["code"].(float64) != 200 {
		t.Fatalf("body = %#v", bodyMap)
	}
}

func TestAgentTxtVerifyUploadRejectsNonTextFile(t *testing.T) {
	handler := NewAgentTxtVerifyUploadHandler(t.TempDir())
	body, contentType := multipartBody(t, "WW_verify_ABCDEF1234567890.txt", "application/octet-stream", "ABCDEF1234567890")

	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/agent/txtVerifyUpload", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	bodyMap := decodeBody(t, rec.Body.Bytes())
	if bodyMap["msg"] != "文件类型错误" {
		t.Fatalf("msg = %#v", bodyMap["msg"])
	}
}

func multipartBody(t *testing.T, filename string, contentType string, content string) (io.Reader, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
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
