package http

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/moduleprincipal"
	"jiyi/mochat-go/internal/modules/providers"
	audiolocal "jiyi/mochat-go/internal/modules/providers/audio/local"
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

func (s *fakeStore) List(_ context.Context, filter MediaListFilter) (ListResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := ListResult{Page: filter.Page, PerPage: filter.PerPage}
	for _, object := range s.objects {
		if object.CorpID != filter.CorpID || object.DeletedAt != nil || object.Source != "wecom_sync" {
			continue
		}
		result.Total++
		result.List = append(result.List, *object)
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

func (s *fakeStore) UpdateDuration(_ context.Context, id int64, durationSeconds int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if object := s.objects[id]; object != nil {
		object.DurationSeconds = durationSeconds
	}
	return nil
}

type fakeResolver struct{}

func (fakeResolver) Resolve(*http.Request) (moduleprincipal.Principal, error) {
	return moduleprincipal.Principal{UserID: 7, TenantID: 1, CorpID: 2}, nil
}

type fakeAuthorizer struct{}

func (fakeAuthorizer) Authorize(context.Context, moduleprincipal.Principal, int64, string) error {
	return nil
}

type deadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

func newTestHandler(t *testing.T) (*MediaHandler, *fakeStore, string) {
	t.Helper()
	root := t.TempDir()
	store := newFakeStore()
	storage, err := audiolocal.New(audiolocal.Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewMediaHandler(store, storage, fakeResolver{}, fakeAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	return handler, store, root
}

func newTestAudioStorage(t *testing.T) (providers.AudioProvider, error) {
	t.Helper()
	return audiolocal.New(audiolocal.Config{Root: t.TempDir()})
}

func devWAV(seconds int) []byte {
	const sampleRate = 8000
	const channels = 1
	const bits = 16
	dataSize := sampleRate * channels * bits / 8 * seconds
	payload := make([]byte, 44+dataSize)
	copy(payload[:4], "RIFF")
	binary.LittleEndian.PutUint32(payload[4:8], uint32(len(payload)-8))
	copy(payload[8:12], "WAVE")
	copy(payload[12:16], "fmt ")
	binary.LittleEndian.PutUint32(payload[16:20], 16)
	binary.LittleEndian.PutUint16(payload[20:22], 1)
	binary.LittleEndian.PutUint16(payload[22:24], channels)
	binary.LittleEndian.PutUint32(payload[24:28], sampleRate)
	binary.LittleEndian.PutUint32(payload[28:32], sampleRate*channels*bits/8)
	binary.LittleEndian.PutUint16(payload[32:34], channels*bits/8)
	binary.LittleEndian.PutUint16(payload[34:36], bits)
	copy(payload[36:40], "data")
	binary.LittleEndian.PutUint32(payload[40:44], uint32(dataSize))
	return payload
}

func TestReadOnlyListDownloadAndDurationBackfill(t *testing.T) {
	handler, store, root := newTestHandler(t)
	content := devWAV(3)
	key := "audio/1/2026/08/dev-001.wav"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(key))), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(key)), content, 0o644); err != nil {
		t.Fatal(err)
	}
	store.objects[1] = &AudioObject{ID: 1, CorpID: 2, Source: "wecom_sync", OriginalName: "企微同步-001.wav", RelativePath: key, ContentType: "audio/wav", SizeBytes: int64(len(content)), SyncedAt: ptrTime(time.Date(2026, 8, 20, 10, 0, 0, 0, time.Local))}

	listReq := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media?sender=张伟", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list code = %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listed struct {
		Data struct {
			List []AudioObject `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Data.List) != 1 || listed.Data.List[0].DurationSeconds != 3 {
		t.Fatalf("list = %#v", listed.Data.List)
	}

	contentReq := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media/1/content", nil)
	contentRec := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	handler.ServeHTTP(contentRec, contentReq)
	if contentRec.Code != http.StatusOK || !bytes.Equal(contentRec.Body.Bytes(), content) {
		t.Fatalf("content code=%d len=%d", contentRec.Code, contentRec.Body.Len())
	}
	if contentType := contentRec.Header().Get("Content-Type"); contentType != "audio/wav" {
		t.Fatalf("content type = %q", contentType)
	}
	if len(contentRec.deadlines) != 1 || !contentRec.deadlines[0].IsZero() {
		t.Fatalf("long response deadline = %+v", contentRec.deadlines)
	}
	rangeReq := httptest.NewRequest(http.MethodGet, "/dashboard/chat/media/1/content", nil)
	rangeReq.Header.Set("Range", "bytes=0-43")
	rangeRec := httptest.NewRecorder()
	handler.ServeHTTP(rangeRec, rangeReq)
	if rangeRec.Code != http.StatusPartialContent || !bytes.Equal(rangeRec.Body.Bytes(), content[:44]) {
		t.Fatalf("range code=%d len=%d", rangeRec.Code, rangeRec.Body.Len())
	}
}

func ptrTime(value time.Time) *time.Time { return &value }

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
