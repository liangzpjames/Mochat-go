package dashboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type ArchiveMediaContentFilter struct {
	ID                  string
	TenantID            int
	CorpID              int
	RestrictEmployeeIDs bool
	AllowedEmployeeIDs  []int
}

type ArchiveMediaContentObject struct {
	ID          string
	MediaType   string
	Name        string
	MIMEType    string
	Size        int64
	Status      string
	StoragePath string
	SHA256      string
}

type ArchiveMediaContentStore interface {
	ArchiveMediaContent(context.Context, ArchiveMediaContentFilter) (ArchiveMediaContentObject, bool, error)
}

type ArchiveMediaContentHandler struct {
	store     ArchiveMediaContentStore
	root      string
	hashSlots chan struct{}
	hashFile  func(io.Reader) ([sha256.Size]byte, error)
}

const archiveMediaHashConcurrency = 4

func NewArchiveMediaContentHandler(store ArchiveMediaContentStore, storageRoot string) *ArchiveMediaContentHandler {
	return &ArchiveMediaContentHandler{
		store: store, root: strings.TrimSpace(storageRoot), hashSlots: make(chan struct{}, archiveMediaHashConcurrency), hashFile: archiveMediaSHA256,
	}
}

func (handler *ArchiveMediaContentHandler) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if request == nil || (request.Method != http.MethodGet && request.Method != http.MethodHead) {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	setArchiveMediaSecurityHeaders(w.Header())
	id, ok := archiveMediaIDFromPath(request.URL.Path)
	if !ok || handler == nil || handler.store == nil || handler.root == "" {
		http.NotFound(w, request)
		return
	}
	principal, err := DashboardPrincipalFromContext(request.Context())
	if err != nil {
		http.NotFound(w, request)
		return
	}
	access, ok := DashboardAccessFromContext(request.Context())
	if !ok || access.UserID != principal.UserID || access.TenantID != principal.TenantID || access.CorpID != principal.CorpID || !access.ScopeRequired {
		http.NotFound(w, request)
		return
	}
	filter := ArchiveMediaContentFilter{ID: id, TenantID: principal.TenantID, CorpID: principal.CorpID}
	if access.Scope != DataScopeTenant {
		filter.RestrictEmployeeIDs = true
		filter.AllowedEmployeeIDs = uniquePositiveInts(access.AllowedEmployeeIDs)
		if len(filter.AllowedEmployeeIDs) == 0 {
			http.NotFound(w, request)
			return
		}
	}
	object, found, err := handler.store.ArchiveMediaContent(request.Context(), filter)
	if err != nil || !found || object.ID != id || object.Status != "ready" || object.Size <= 0 {
		http.NotFound(w, request)
		return
	}
	root, expectedRootInfo, expectedArchiveInfo, expectedInfo, ok := handler.safeObjectPath(object)
	if !ok {
		http.NotFound(w, request)
		return
	}
	storageRoot, err := os.OpenRoot(root)
	if err != nil {
		http.NotFound(w, request)
		return
	}
	defer storageRoot.Close()
	openedRootInfo, err := storageRoot.Stat(".")
	if err != nil || !os.SameFile(expectedRootInfo, openedRootInfo) || archiveMediaIsReparsePoint(openedRootInfo) {
		http.NotFound(w, request)
		return
	}
	confinedRoot, err := storageRoot.OpenRoot("archive-media")
	if err != nil {
		http.NotFound(w, request)
		return
	}
	defer confinedRoot.Close()
	openedArchiveInfo, err := confinedRoot.Stat(".")
	if err != nil || !os.SameFile(expectedArchiveInfo, openedArchiveInfo) || archiveMediaIsReparsePoint(openedArchiveInfo) {
		http.NotFound(w, request)
		return
	}
	file, err := confinedRoot.Open(id)
	if err != nil {
		http.NotFound(w, request)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || archiveMediaIsReparsePoint(info) || !os.SameFile(expectedInfo, info) || info.Size() != object.Size {
		http.NotFound(w, request)
		return
	}
	if !handler.archiveMediaFileHashMatches(request.Context(), file, info, object.SHA256) {
		http.NotFound(w, request)
		return
	}
	mimeType, inline := archiveMediaMIME(object.MediaType, object.MIMEType)
	disposition := "attachment"
	if inline && request.URL.Query().Get("download") != "1" {
		disposition = "inline"
	}
	name := safeArchiveMediaFilename(object.Name, id)
	if value := mime.FormatMediaType(disposition, map[string]string{"filename": name}); value != "" {
		w.Header().Set("Content-Disposition", value)
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Accept-Ranges", "bytes")
	serveArchiveMediaRange(w, request, file, info.Size())
}

func (handler *ArchiveMediaContentHandler) archiveMediaFileHashMatches(ctx context.Context, file *os.File, info os.FileInfo, expected string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	decoded, err := hex.DecodeString(expected)
	if err != nil || len(decoded) != sha256.Size {
		return false
	}
	if handler.hashSlots == nil {
		return false
	}
	select {
	case handler.hashSlots <- struct{}{}:
		defer func() { <-handler.hashSlots }()
	case <-ctx.Done():
		return false
	}
	return handler.hashOpenedArchiveMedia(file, info, decoded)
}

func (handler *ArchiveMediaContentHandler) hashOpenedArchiveMedia(file *os.File, info os.FileInfo, expected []byte) bool {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false
	}
	hashFile := handler.hashFile
	if hashFile == nil {
		hashFile = archiveMediaSHA256
	}
	actual, err := hashFile(file)
	if err != nil {
		return false
	}
	after, err := file.Stat()
	return err == nil && os.SameFile(info, after) && info.Size() == after.Size() && info.ModTime().Equal(after.ModTime()) && string(actual[:]) == string(expected)
}

func archiveMediaSHA256(reader io.Reader) ([sha256.Size]byte, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return [sha256.Size]byte{}, err
	}
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func (handler *ArchiveMediaContentHandler) safeObjectPath(object ArchiveMediaContentObject) (string, os.FileInfo, os.FileInfo, os.FileInfo, bool) {
	root, err := filepath.Abs(handler.root)
	if err != nil {
		return "", nil, nil, nil, false
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || archiveMediaIsReparsePoint(rootInfo) || !rootInfo.IsDir() {
		return "", nil, nil, nil, false
	}
	archiveRoot := filepath.Join(root, "archive-media")
	archiveInfo, err := os.Lstat(archiveRoot)
	if err != nil || archiveInfo.Mode()&os.ModeSymlink != 0 || archiveMediaIsReparsePoint(archiveInfo) || !archiveInfo.IsDir() {
		return "", nil, nil, nil, false
	}
	expected := filepath.Join(archiveRoot, object.ID)
	databasePath, err := filepath.Abs(strings.TrimSpace(object.StoragePath))
	if err != nil || filepath.Clean(databasePath) != filepath.Clean(expected) {
		return "", nil, nil, nil, false
	}
	relative, err := filepath.Rel(archiveRoot, expected)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", nil, nil, nil, false
	}
	info, err := os.Lstat(expected)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || archiveMediaIsReparsePoint(info) || !info.Mode().IsRegular() {
		return "", nil, nil, nil, false
	}
	return root, rootInfo, archiveInfo, info, true
}

func archiveMediaIDFromPath(path string) (string, bool) {
	const prefix, suffix = "/dashboard/archive/media/", "/content"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if strings.Contains(raw, "/") {
		return "", false
	}
	parsed, err := uuid.Parse(raw)
	return parsed.String(), err == nil && parsed.String() == strings.ToLower(raw)
}

func setArchiveMediaSecurityHeaders(header http.Header) {
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", "private, no-store")
	header.Set("Pragma", "no-cache")
}

func archiveMediaMIME(mediaType, supplied string) (string, bool) {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	supplied = strings.ToLower(strings.TrimSpace(strings.Split(supplied, ";")[0]))
	allowed := map[string]map[string]bool{
		"image": {"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true},
		"voice": {"audio/mpeg": true, "audio/mp4": true, "audio/ogg": true, "audio/wav": true, "audio/aac": true},
		"audio": {"audio/mpeg": true, "audio/mp4": true, "audio/ogg": true, "audio/wav": true, "audio/aac": true},
		"video": {"video/mp4": true, "video/webm": true, "video/quicktime": true},
		"file": {
			"application/pdf": true, "text/plain": true, "text/csv": true, "application/zip": true,
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
		},
	}
	if allowed[mediaType][supplied] {
		return supplied, mediaType != "file"
	}
	return "application/octet-stream", false
}

func safeArchiveMediaFilename(raw, id string) string {
	raw = filepath.Base(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"))
	raw = strings.Map(func(value rune) rune {
		if value < 0x20 || value == 0x7f || value == '"' || value == '\\' || value == '/' {
			return -1
		}
		return value
	}, raw)
	if raw == "" || raw == "." {
		return "archive-media-" + id
	}
	runes := []rune(raw)
	if len(runes) > 180 {
		raw = string(runes[:180])
	}
	return raw
}

func serveArchiveMediaRange(w http.ResponseWriter, request *http.Request, file *os.File, size int64) {
	rawRange := strings.TrimSpace(request.Header.Get("Range"))
	start, end, partial, err := parseArchiveMediaRange(rawRange, size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if !partial {
		start, end = 0, size-1
	}
	length := end - start + 1
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	if partial {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	if request.Method == http.MethodHead {
		return
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return
	}
	_, _ = io.CopyN(w, file, length)
}

func parseArchiveMediaRange(raw string, size int64) (start, end int64, partial bool, err error) {
	if raw == "" {
		return 0, size - 1, false, nil
	}
	if size <= 0 || !strings.HasPrefix(raw, "bytes=") || strings.Contains(raw, ",") {
		return 0, 0, false, errors.New("invalid range")
	}
	value := strings.TrimSpace(strings.TrimPrefix(raw, "bytes="))
	left, right, ok := strings.Cut(value, "-")
	if !ok || (left == "" && right == "") {
		return 0, 0, false, errors.New("invalid range")
	}
	if left == "" {
		suffix, parseErr := strconv.ParseInt(right, 10, 64)
		if parseErr != nil || suffix <= 0 {
			return 0, 0, false, errors.New("invalid range")
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, true, nil
	}
	start, err = strconv.ParseInt(left, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false, errors.New("invalid range")
	}
	if right == "" {
		return start, size - 1, true, nil
	}
	end, err = strconv.ParseInt(right, 10, 64)
	if err != nil || end < start {
		return 0, 0, false, errors.New("invalid range")
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true, nil
}
