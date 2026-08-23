package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

type fakeDocumentRepo struct {
	items     []ports.KnowledgeDocument
	chunks    map[string][]ports.KnowledgeChunk
	createErr error
	deleteErr error
	deletedID string
}

func (f *fakeDocumentRepo) List(_ context.Context, tenantID, corpID int64, kbID string) ([]ports.KnowledgeDocument, error) {
	result := make([]ports.KnowledgeDocument, 0)
	for _, item := range f.items {
		if item.TenantID == tenantID && item.CorpID == corpID && item.KnowledgeBaseID == kbID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *fakeDocumentRepo) Get(_ context.Context, tenantID, corpID int64, kbID, documentID string) (ports.KnowledgeDocument, error) {
	for _, item := range f.items {
		if item.TenantID == tenantID && item.CorpID == corpID && item.KnowledgeBaseID == kbID && item.ID == documentID {
			return item, nil
		}
	}
	return ports.KnowledgeDocument{}, ports.ErrNotFound
}

func (f *fakeDocumentRepo) Count(_ context.Context, tenantID, corpID int64, kbID string) (int, error) {
	items, _ := f.List(context.Background(), tenantID, corpID, kbID)
	return len(items), nil
}

func (f *fakeDocumentRepo) Create(_ context.Context, item ports.KnowledgeDocument, chunks []ports.KnowledgeChunk) (ports.KnowledgeDocument, error) {
	if f.createErr != nil {
		return ports.KnowledgeDocument{}, f.createErr
	}
	if f.chunks == nil {
		f.chunks = map[string][]ports.KnowledgeChunk{}
	}
	item.ChunkCount = len(chunks)
	f.items = append(f.items, item)
	f.chunks[item.ID] = chunks
	return item, nil
}

func (f *fakeDocumentRepo) Delete(_ context.Context, tenantID, corpID, _ int64, kbID, documentID string) (ports.KnowledgeDocument, error) {
	if f.deleteErr != nil {
		return ports.KnowledgeDocument{}, f.deleteErr
	}
	item, err := f.Get(context.Background(), tenantID, corpID, kbID, documentID)
	if err != nil {
		return ports.KnowledgeDocument{}, err
	}
	f.deletedID = documentID
	for index := range f.items {
		if f.items[index].ID == documentID {
			f.items = append(f.items[:index], f.items[index+1:]...)
			break
		}
	}
	return item, nil
}

type fakeDocumentStorage struct {
	root       string
	objects    map[string]string
	quarantine map[string]string
}

func newFakeDocumentStorage(t *testing.T) *fakeDocumentStorage {
	t.Helper()
	return &fakeDocumentStorage{root: t.TempDir(), objects: map[string]string{}, quarantine: map[string]string{}}
}

func (f *fakeDocumentStorage) Stage(reader io.Reader) (string, int64, string, error) {
	file, err := os.CreateTemp(f.root, "stage-*")
	if err != nil {
		return "", 0, "", err
	}
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(file, hash), reader)
	_ = file.Close()
	return file.Name(), size, hex.EncodeToString(hash.Sum(nil)), err
}

func (f *fakeDocumentStorage) Commit(staged, key string) (string, error) {
	target := filepath.Join(f.root, strings.ReplaceAll(key, "/", "_"))
	if err := os.Rename(staged, target); err != nil {
		return "", err
	}
	f.objects[key] = target
	return target, nil
}

func (f *fakeDocumentStorage) Quarantine(key string) (string, error) {
	path, ok := f.objects[key]
	if !ok {
		return "", os.ErrNotExist
	}
	quarantine := path + ".trash"
	if err := os.Rename(path, quarantine); err != nil {
		return "", err
	}
	delete(f.objects, key)
	f.quarantine[quarantine] = key
	return quarantine, nil
}

func (f *fakeDocumentStorage) Restore(path, key string) error {
	target := strings.TrimSuffix(path, ".trash")
	if err := os.Rename(path, target); err != nil {
		return err
	}
	f.objects[key] = target
	delete(f.quarantine, path)
	return nil
}

func (f *fakeDocumentStorage) PurgeQuarantine(path string) error {
	delete(f.quarantine, path)
	return os.Remove(path)
}

func (f *fakeDocumentStorage) Delete(key string) error {
	path := f.objects[key]
	delete(f.objects, key)
	return os.Remove(path)
}

func (f *fakeDocumentStorage) RemoveStaged(path string) { _ = os.Remove(path) }

func fakeParseDocument(path, filename string) (ports.ParsedDocument, error) {
	extension := strings.ToLower(filepath.Ext(filename))
	if extension == ".doc" {
		return ports.ParsedDocument{}, ports.ErrUnsupportedDocumentType
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ports.ParsedDocument{}, err
	}
	return ports.ParsedDocument{Text: string(content), CharacterCount: len([]rune(string(content))), Extension: strings.TrimPrefix(extension, "."), MIMEType: "text/plain"}, nil
}

func newDocumentHandlerForTest(t *testing.T, kbRepo *fakeKBRepo, documentRepo *fakeDocumentRepo) (*DocumentHandler, *fakeDocumentStorage) {
	t.Helper()
	storage := newFakeDocumentStorage(t)
	return NewDocumentHandler(documentRepo, kbRepo, storage, fakeParseDocument, fakeResolver{principal: Principal{UserID: 7, TenantID: 1, CorpID: 2}}, nil, func() string { return "doc-1" }), storage
}

func uploadDocument(t *testing.T, handler http.Handler, target, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestDocumentUploadPersistsReadyDocumentAndChunks(t *testing.T) {
	kbs := &fakeKBRepo{items: []ports.KnowledgeBase{{ID: "kb-1", TenantID: 1, CorpID: 2, Status: 1}}}
	documents := &fakeDocumentRepo{}
	handler, _ := newDocumentHandlerForTest(t, kbs, documents)
	response := uploadDocument(t, handler, "/dashboard/ai-settings/knowledge-bases/kb-1/documents", "guide.txt", []byte("退款需要主管审批"))
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(documents.items) != 1 || documents.items[0].Status != ports.DocumentStatusReady || len(documents.chunks["doc-1"]) != 1 {
		t.Fatalf("documents=%#v chunks=%#v", documents.items, documents.chunks)
	}
	var envelope struct {
		Data ports.KnowledgeDocument `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Data.ObjectKey != "" {
		t.Fatalf("response leaked object key or failed decode: %#v err=%v", envelope, err)
	}
}

func TestDocumentUploadRejectsUnsupportedTypeAndCrossCorpParent(t *testing.T) {
	kbs := &fakeKBRepo{items: []ports.KnowledgeBase{
		{ID: "kb-1", TenantID: 1, CorpID: 2, Status: 1},
		{ID: "other", TenantID: 1, CorpID: 9, Status: 1},
	}}
	documents := &fakeDocumentRepo{}
	handler, _ := newDocumentHandlerForTest(t, kbs, documents)

	unsupported := uploadDocument(t, handler, "/dashboard/ai-settings/knowledge-bases/kb-1/documents", "legacy.doc", []byte("legacy"))
	if unsupported.Code != http.StatusUnsupportedMediaType || !bytes.Contains(unsupported.Body.Bytes(), []byte(machineCodeDocumentTypeUnsupported)) {
		t.Fatalf("unsupported status=%d body=%s", unsupported.Code, unsupported.Body.String())
	}
	crossCorp := uploadDocument(t, handler, "/dashboard/ai-settings/knowledge-bases/other/documents", "guide.txt", []byte("content"))
	if crossCorp.Code != http.StatusNotFound || len(documents.items) != 0 {
		t.Fatalf("cross corp status=%d items=%#v", crossCorp.Code, documents.items)
	}
}

func TestDocumentHandlerDoesNotMaskParentLookupFailureAsNotFound(t *testing.T) {
	kbs := &fakeKBRepo{getByIDsErr: errors.New("database unavailable")}
	handler, _ := newDocumentHandlerForTest(t, kbs, &fakeDocumentRepo{})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard/ai-settings/knowledge-bases/kb-1/documents", nil))

	if response.Code != http.StatusInternalServerError || !bytes.Contains(response.Body.Bytes(), []byte(machineCodeStorageFailure)) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDocumentListAndDeleteStayScopedAndRemovePrivateFile(t *testing.T) {
	kbs := &fakeKBRepo{items: []ports.KnowledgeBase{{ID: "kb-1", TenantID: 1, CorpID: 2, Status: 1}}}
	documents := &fakeDocumentRepo{}
	handler, storage := newDocumentHandlerForTest(t, kbs, documents)
	uploaded := uploadDocument(t, handler, "/dashboard/ai-settings/knowledge-bases/kb-1/documents", "guide.txt", []byte("content"))
	if uploaded.Code != http.StatusCreated {
		t.Fatalf("upload status=%d body=%s", uploaded.Code, uploaded.Body.String())
	}

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/dashboard/ai-settings/knowledge-bases/kb-1/documents", nil))
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte("guide.txt")) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	deleted := httptest.NewRecorder()
	handler.ServeHTTP(deleted, httptest.NewRequest(http.MethodDelete, "/dashboard/ai-settings/knowledge-bases/kb-1/documents/doc-1", nil))
	if deleted.Code != http.StatusOK || documents.deletedID != "doc-1" {
		t.Fatalf("delete status=%d body=%s deleted=%q", deleted.Code, deleted.Body.String(), documents.deletedID)
	}
	if len(storage.objects) != 0 || len(storage.quarantine) != 0 {
		t.Fatalf("storage not cleaned: objects=%#v quarantine=%#v", storage.objects, storage.quarantine)
	}
}

func TestDocumentUploadSurfacesLimitWithoutPersisting(t *testing.T) {
	kbs := &fakeKBRepo{items: []ports.KnowledgeBase{{ID: "kb-1", TenantID: 1, CorpID: 2, Status: 1}}}
	documents := &fakeDocumentRepo{createErr: ports.ErrDocumentLimit}
	handler, storage := newDocumentHandlerForTest(t, kbs, documents)
	response := uploadDocument(t, handler, "/dashboard/ai-settings/knowledge-bases/kb-1/documents", "guide.txt", []byte("content"))
	if response.Code != http.StatusConflict || !bytes.Contains(response.Body.Bytes(), []byte(machineCodeDocumentLimit)) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(storage.objects) != 0 {
		t.Fatalf("failed persistence left object: %#v", storage.objects)
	}
}
