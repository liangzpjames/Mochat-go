package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	scrmhttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

type fakeStore struct {
	mu      sync.Mutex
	nextID  int64
	objects map[int64]*AudioObject
}

func newFakeStore() *fakeStore {
	return &fakeStore{nextID: 1, objects: map[int64]*AudioObject{}}
}

func (s *fakeStore) Create(_ context.Context, object AudioObject) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object.ID = s.nextID
	s.nextID++
	s.objects[object.ID] = &object
	return object.ID, nil
}

func (s *fakeStore) GetByID(_ context.Context, id int64) (*AudioObject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object := s.objects[id]
	if object == nil || object.DeletedAt != nil {
		return nil, nil
	}
	copy := *object
	return &copy, nil
}

func (s *fakeStore) List(_ context.Context, corpID int64, page int, perPage int, keyword string) (ListResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	result := ListResult{Page: page, PerPage: perPage}
	for _, object := range s.objects {
		if object.CorpID != corpID || object.DeletedAt != nil {
			continue
		}
		if keyword != "" && !strings.Contains(object.OriginalName, keyword) {
			continue
		}
		result.Total++
		copy := *object
		result.List = append(result.List, copy)
	}
	return result, nil
}

func (s *fakeStore) SoftDelete(_ context.Context, id int64, deletedBy int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	object := s.objects[id]
	if object == nil || object.DeletedAt != nil {
		return errors.New("not found")
	}
	now := time.Now()
	object.DeletedAt = &now
	object.DeletedBy = deletedBy
	return nil
}

type fakeResolver struct{}

func (fakeResolver) Resolve(*http.Request) (scrmhttp.Principal, error) {
	return scrmhttp.Principal{UserID: 7, TenantID: 1}, nil
}

type fakeAuthorizer struct{}

func (fakeAuthorizer) Authorize(context.Context, scrmhttp.Principal, int64, string) error {
	return nil
}

func newTestHandler(t *testing.T) (*MediaHandler, *fakeStore, string) {
	t.Helper()
	root := t.TempDir()
	store := newFakeStore()
	handler, err := NewMediaHandler(store, root, fakeResolver{}, fakeAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	return handler, store, root
}

func multipartUpload(t *testing.T, handler http.Handler, name string, contentType string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, name))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/chat/media?corpId=2", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestUploadListDownloadDelete(t *testing.T) {
	handler, _, root := newTestHandler(t)
	content := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")
	rec := multipartUpload(t, handler, "p35-accept.wav", "audio/wav", content)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload code = %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	found := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".wav") {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal("uploaded file not found on disk")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media?corpId=2", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list code = %d", listRec.Code)
	}
	var listed struct {
		Data struct {
			List  []AudioObject `json:"list"`
			Total int64         `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.Data.Total != 1 || len(listed.Data.List) != 1 || listed.Data.List[0].PlayURL == "" {
		t.Fatalf("list = %#v", listed.Data)
	}

	contentReq := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media/1/content", nil)
	contentRec := httptest.NewRecorder()
	handler.ServeHTTP(contentRec, contentReq)
	if contentRec.Code != http.StatusOK {
		t.Fatalf("content code = %d body=%s", contentRec.Code, contentRec.Body.String())
	}
	if !bytes.Equal(contentRec.Body.Bytes(), content) {
		t.Fatal("content mismatch")
	}
	if contentType := contentRec.Header().Get("Content-Type"); contentType != "audio/wav" {
		t.Fatalf("content type = %q", contentType)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/dashboard/chat/media/1?corpId=2", nil)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete code = %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	afterReq := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media/1/content", nil)
	afterRec := httptest.NewRecorder()
	handler.ServeHTTP(afterRec, afterReq)
	if afterRec.Code != http.StatusNotFound {
		t.Fatalf("content after delete code = %d, want 404", afterRec.Code)
	}
	foundAfter := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".wav") {
			foundAfter = true
		}
		return nil
	})
	if foundAfter {
		t.Fatal("disk file still present after soft delete")
	}
}

func TestUploadRejectsNonAudioAndMissingCorp(t *testing.T) {
	handler, _, _ := newTestHandler(t)
	rec := multipartUpload(t, handler, "notes.txt", "text/plain", []byte("hello"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-audio code = %d, want 400", rec.Code)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="a.wav"`)
	header.Set("Content-Type", "audio/wav")
	part, _ := writer.CreatePart(header)
	_, _ = part.Write([]byte("x"))
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/chat/media", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing corp code = %d, want 400", rec.Code)
	}
}

func TestUploadRejectsTextRenamedAsAudio(t *testing.T) {
	handler, _, _ := newTestHandler(t)
	rec := multipartUpload(t, handler, "notes.wav", "audio/wav", []byte("hello"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("renamed text code = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestUploadNormalizesExtensionFromContent(t *testing.T) {
	handler, store, root := newTestHandler(t)
	content := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")
	rec := multipartUpload(t, handler, "clip.mp3", "application/octet-stream", content)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload code = %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data struct {
			ID          int64  `json:"id"`
			ContentType string `json:"contentType"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.ContentType != "audio/wav" {
		t.Fatalf("content type = %q, want audio/wav", created.Data.ContentType)
	}
	object, err := store.GetByID(context.Background(), created.Data.ID)
	if err != nil || object == nil {
		t.Fatalf("stored object = %#v err=%v", object, err)
	}
	if !strings.HasSuffix(object.RelativePath, ".wav") {
		t.Fatalf("relative path = %q, want .wav suffix", object.RelativePath)
	}
	found := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".wav") {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal("normalized .wav file not found on disk")
	}
}

func TestUploadUnauthorized(t *testing.T) {
	store := newFakeStore()
	handler, err := NewMediaHandler(store, t.TempDir(), fakeResolver{}, fakeAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	// Replace the authorizer with a denying one through a small wrapper handler.
	deny := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.authorize = denyingAuthorizer{}
		handler.ServeHTTP(w, r)
	})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media?corpId=2", nil)
	rec := httptest.NewRecorder()
	deny.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", rec.Code)
	}
}

type denyingAuthorizer struct{}

func (denyingAuthorizer) Authorize(context.Context, scrmhttp.Principal, int64, string) error {
	return errors.New("denied")
}

func TestWriteEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	writeEnvelope(rec, http.StatusOK, "success", map[string]any{"ok": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	payload, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["code"].(float64) != 200 {
		t.Fatalf("envelope = %#v", decoded)
	}
}
