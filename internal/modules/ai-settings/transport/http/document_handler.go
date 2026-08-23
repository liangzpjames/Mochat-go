package http

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

const (
	maxDocumentUploadBytes int64 = 20 << 20
	maxMultipartOverhead   int64 = 1 << 20
)

type documentParser func(path, filename string) (ports.ParsedDocument, error)

type DocumentHandler struct {
	repo           ports.DocumentRepository
	knowledgeBases ports.KnowledgeBaseRepository
	storage        ports.DocumentStorage
	parse          documentParser
	principal      PrincipalResolver
	authorize      Authorizer
	generate       func() string
}

func NewDocumentHandler(repo ports.DocumentRepository, knowledgeBases ports.KnowledgeBaseRepository, storage ports.DocumentStorage, parse documentParser, principal PrincipalResolver, authorizer Authorizer, generate func() string) *DocumentHandler {
	return &DocumentHandler{repo: repo, knowledgeBases: knowledgeBases, storage: storage, parse: parse, principal: principal, authorize: authorizer, generate: generate}
}

func (h *DocumentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	principal, ok := resolvePrincipal(w, r, h.principal)
	if !ok {
		return
	}
	knowledgeBaseID, documentID, pathOK := documentPathIDs(r.URL.Path)
	if !pathOK {
		writeEnvelope(w, http.StatusBadRequest, machineCodeIDRequired, nil)
		return
	}
	permission := "/ai-settings/knowledge-base#get"
	if r.Method != http.MethodGet {
		permission = "/ai-settings/knowledge-base@edit#put"
	}
	if !authorize(w, r, h.authorize, principal, permission) {
		return
	}
	parentExists, err := h.parentExists(r.Context(), principal, knowledgeBaseID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
		return
	}
	if !parentExists {
		writeEnvelope(w, http.StatusNotFound, machineCodeNotFound, nil)
		return
	}
	switch r.Method {
	case http.MethodGet:
		if documentID != "" {
			writeEnvelope(w, http.StatusMethodNotAllowed, "method not allowed", nil)
			return
		}
		h.list(w, r, principal, knowledgeBaseID)
	case http.MethodPost:
		if documentID != "" {
			writeEnvelope(w, http.StatusMethodNotAllowed, "method not allowed", nil)
			return
		}
		h.upload(w, r, principal, knowledgeBaseID)
	case http.MethodDelete:
		if documentID == "" {
			writeEnvelope(w, http.StatusBadRequest, machineCodeIDRequired, nil)
			return
		}
		h.delete(w, r, principal, knowledgeBaseID, documentID)
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (h *DocumentHandler) parentExists(ctx context.Context, principal Principal, knowledgeBaseID string) (bool, error) {
	if h.knowledgeBases == nil {
		return false, errors.New("knowledge base repository is unavailable")
	}
	items, err := h.knowledgeBases.GetByIDs(ctx, principal.TenantID, principal.CorpID, []string{knowledgeBaseID})
	if err != nil {
		return false, err
	}
	return len(items) == 1 && items[0].ID == knowledgeBaseID, nil
}

func (h *DocumentHandler) list(w http.ResponseWriter, r *http.Request, principal Principal, knowledgeBaseID string) {
	if h.repo == nil {
		writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
		return
	}
	items, err := h.repo.List(r.Context(), principal.TenantID, principal.CorpID, knowledgeBaseID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
		return
	}
	writeEnvelope(w, http.StatusOK, "success", items)
}

func (h *DocumentHandler) upload(w http.ResponseWriter, r *http.Request, principal Principal, knowledgeBaseID string) {
	if h.repo == nil || h.storage == nil || h.parse == nil || h.generate == nil {
		writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxDocumentUploadBytes+maxMultipartOverhead)
	if err := r.ParseMultipartForm(maxDocumentUploadBytes); err != nil {
		writeEnvelope(w, http.StatusRequestEntityTooLarge, machineCodeDocumentTooLarge, nil)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, machineCodeDocumentUnreadable, nil)
		return
	}
	defer file.Close()
	filename := filepath.Base(strings.TrimSpace(header.Filename))
	if filename == "" || filename == "." {
		writeEnvelope(w, http.StatusBadRequest, machineCodeDocumentUnreadable, nil)
		return
	}
	stagedPath, sizeBytes, stagedChecksum, err := h.storage.Stage(file)
	if err != nil {
		h.writeDocumentInputError(w, err)
		return
	}
	defer h.storage.RemoveStaged(stagedPath)

	parsed, parseErr := h.parse(stagedPath, filename)
	if parseErr != nil && !errors.Is(parseErr, ports.ErrDocumentUnreadable) {
		h.writeDocumentInputError(w, parseErr)
		return
	}
	documentID := h.generate()
	extension := parsed.Extension
	if extension == "" {
		extension = strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	}
	mimeType := parsed.MIMEType
	if mimeType == "" {
		mimeType = safeMIMEType(header)
	}
	objectKey := fmt.Sprintf("tenant-%d/corp-%d/%s/%s.%s", principal.TenantID, principal.CorpID, knowledgeBaseID, documentID, extension)
	if _, err := h.storage.Commit(stagedPath, objectKey); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
		return
	}

	status := ports.DocumentStatusReady
	errorSummary := ""
	chunks := []ports.KnowledgeChunk{}
	if parseErr != nil {
		status = ports.DocumentStatusFailed
		errorSummary = "无法提取可用文本，请确认文件未加密且包含文字层。"
	} else {
		chunks = chunkDocument(parsed.Text, principal, knowledgeBaseID, documentID)
	}
	checksum := parsed.SHA256
	if checksum == "" {
		checksum = stagedChecksum
	}
	document := ports.KnowledgeDocument{
		ID: documentID, TenantID: principal.TenantID, CorpID: principal.CorpID, KnowledgeBaseID: knowledgeBaseID,
		Filename: filename, Extension: extension, MIMEType: mimeType, ObjectKey: objectKey,
		SizeBytes: sizeBytes, SHA256: checksum, Status: status, ErrorSummary: errorSummary,
		CharacterCount: parsed.CharacterCount, ChunkCount: len(chunks), CreatedBy: principal.UserID, UpdatedBy: principal.UserID,
	}
	created, err := h.repo.Create(r.Context(), document, chunks)
	if err != nil {
		_ = h.storage.Delete(objectKey)
		writeMutationError(w, err)
		return
	}
	writeEnvelope(w, http.StatusCreated, "success", created)
}

func (h *DocumentHandler) delete(w http.ResponseWriter, r *http.Request, principal Principal, knowledgeBaseID, documentID string) {
	if h.repo == nil || h.storage == nil {
		writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
		return
	}
	document, err := h.repo.Get(r.Context(), principal.TenantID, principal.CorpID, knowledgeBaseID, documentID)
	if err != nil {
		writeMutationError(w, err)
		return
	}
	quarantine, err := h.storage.Quarantine(document.ObjectKey)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
		return
	}
	deleted, err := h.repo.Delete(r.Context(), principal.TenantID, principal.CorpID, principal.UserID, knowledgeBaseID, documentID)
	if err != nil {
		_ = h.storage.Restore(quarantine, document.ObjectKey)
		writeMutationError(w, err)
		return
	}
	_ = h.storage.PurgeQuarantine(quarantine)
	writeEnvelope(w, http.StatusOK, "success", map[string]any{"id": deleted.ID})
}

func (h *DocumentHandler) writeDocumentInputError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ports.ErrUnsupportedDocumentType):
		writeEnvelope(w, http.StatusUnsupportedMediaType, machineCodeDocumentTypeUnsupported, nil)
	case errors.Is(err, ports.ErrDocumentTooLarge):
		writeEnvelope(w, http.StatusRequestEntityTooLarge, machineCodeDocumentTooLarge, nil)
	case errors.Is(err, ports.ErrDocumentTextTooLarge):
		writeEnvelope(w, http.StatusUnprocessableEntity, machineCodeDocumentTextTooLarge, nil)
	case errors.Is(err, ports.ErrDocumentUnreadable):
		writeEnvelope(w, http.StatusUnprocessableEntity, machineCodeDocumentUnreadable, nil)
	default:
		writeEnvelope(w, http.StatusInternalServerError, machineCodeStorageFailure, nil)
	}
}

func documentPathIDs(path string) (knowledgeBaseID, documentID string, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for index := range parts {
		if parts[index] != "knowledge-bases" || index+2 >= len(parts) || parts[index+2] != "documents" {
			continue
		}
		knowledgeBaseID = strings.TrimSpace(parts[index+1])
		if index+3 < len(parts) {
			documentID = strings.TrimSpace(parts[index+3])
		}
		return knowledgeBaseID, documentID, knowledgeBaseID != "" && (index+3 == len(parts) || index+4 == len(parts))
	}
	return "", "", false
}

func safeMIMEType(header *multipart.FileHeader) string {
	if header == nil {
		return "application/octet-stream"
	}
	value := strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0])
	if value == "" {
		return "application/octet-stream"
	}
	return value
}

func chunkDocument(text string, principal Principal, knowledgeBaseID, documentID string) []ports.KnowledgeChunk {
	runes := []rune(text)
	if len(runes) == 0 {
		return []ports.KnowledgeChunk{}
	}
	const chunkSize = 1200
	const overlap = 100
	chunks := make([]ports.KnowledgeChunk, 0, (len(runes)+chunkSize-1)/chunkSize)
	for start := 0; start < len(runes); {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		content := strings.TrimSpace(string(runes[start:end]))
		if content != "" {
			chunks = append(chunks, ports.KnowledgeChunk{TenantID: principal.TenantID, CorpID: principal.CorpID, KnowledgeBaseID: knowledgeBaseID, DocumentID: documentID, Ordinal: len(chunks), Content: content, CharacterCount: len([]rune(content))})
		}
		if end == len(runes) {
			break
		}
		start = end - overlap
	}
	return chunks
}
