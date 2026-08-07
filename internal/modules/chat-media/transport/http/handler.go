package http

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
	audiolocal "jiyi/mochat-go/internal/modules/providers/audio/local"
	scrmhttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

const (
	maxAudioBytes   = 50 << 20
	mediaPermission = "/chat/file-audio#get"
)

type PrincipalResolver interface {
	Resolve(*http.Request) (scrmhttp.Principal, error)
}

type Authorizer interface {
	Authorize(context.Context, scrmhttp.Principal, int64, string) error
}

type MediaHandler struct {
	store     MediaStore
	storage   providers.AudioProvider
	principal PrincipalResolver
	authorize Authorizer
}

func NewMediaHandler(store MediaStore, fileStorageRoot string, principal PrincipalResolver, authorize Authorizer) (*MediaHandler, error) {
	if store == nil {
		return nil, errors.New("chat media store is required")
	}
	storage, err := audiolocal.New(audiolocal.Config{Root: fileStorageRoot})
	if err != nil {
		return nil, err
	}
	return &MediaHandler{store: store, storage: storage, principal: principal, authorize: authorize}, nil
}

func RegisterRoutes(registrar interface {
	Handle(method, pattern string, handler http.Handler) error
}, handler http.Handler) error {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if err := registrar.Handle(method, "/dashboard/chat/media", handler); err != nil {
			return err
		}
	}
	if err := registrar.Handle(http.MethodDelete, "/dashboard/chat/media/{id}", handler); err != nil {
		return err
	}
	return registrar.Handle(http.MethodGet, "/dashboard/chat/media/{id}/content", handler)
}

func (h *MediaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	principal, err := h.principal.Resolve(r)
	if err != nil {
		writeEnvelope(w, http.StatusUnauthorized, "principal unauthorized", nil)
		return
	}
	// Content URLs are tenant-scoped by the object itself, so they do not
	// require a corpId query parameter (the audio element would otherwise be
	// unable to load the URL).
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/content") {
		h.serveContent(w, r, principal)
		return
	}
	corpID, err := parseCorpID(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if h.authorize != nil {
		if err := h.authorize.Authorize(r.Context(), principal, corpID, mediaPermission); err != nil {
			writeEnvelope(w, http.StatusForbidden, "forbidden", nil)
			return
		}
	}
	switch {
	case r.Method == http.MethodGet:
		h.list(w, r, corpID)
	case r.Method == http.MethodPost:
		h.upload(w, r, principal, corpID)
	case r.Method == http.MethodDelete:
		h.remove(w, r, principal, corpID)
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (h *MediaHandler) list(w http.ResponseWriter, r *http.Request, corpID int64) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("perPage"))
	result, err := h.store.List(r.Context(), corpID, page, perPage, r.URL.Query().Get("keyword"))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	for index := range result.List {
		result.List[index].PlayURL = fmt.Sprintf("/dashboard/chat/media/%d/content", result.List[index].ID)
	}
	writeEnvelope(w, http.StatusOK, "success", result)
}

func (h *MediaHandler) upload(w http.ResponseWriter, r *http.Request, principal scrmhttp.Principal, corpID int64) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAudioBytes+1<<20)
	if err := r.ParseMultipartForm(maxAudioBytes); err != nil {
		writeEnvelope(w, http.StatusBadRequest, "上传文件过大或格式错误："+err.Error(), nil)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, "缺少上传文件（字段 file）", nil)
		return
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, maxAudioBytes+1))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, "读取上传文件失败", nil)
		return
	}
	if len(payload) == 0 {
		writeEnvelope(w, http.StatusBadRequest, "上传文件为空", nil)
		return
	}
	if len(payload) > maxAudioBytes {
		writeEnvelope(w, http.StatusBadRequest, "上传文件超过 50MB 限制", nil)
		return
	}
	detected, ok := detectAudioFormat(payload)
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, "仅支持 WAV/MP3/OGG/FLAC/M4A/AAC/AMR/WebM 音频文件（按文件内容识别）", nil)
		return
	}
	contentType := detected.contentType
	now := time.Now()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, "生成文件标识失败", nil)
		return
	}
	extension := detected.extension
	key := fmt.Sprintf("audio/%d/%04d/%02d/%s%s", corpID, now.Year(), int(now.Month()), hex.EncodeToString(random[:]), extension)
	sha256Hex, err := sha256HexOf(payload)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, "计算文件校验失败", nil)
		return
	}
	if err := h.storage.Put(r.Context(), key, bytes.NewReader(payload), providers.PutOptions{ContentType: contentType, SizeBytes: int64(len(payload))}); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, "保存文件失败："+err.Error(), nil)
		return
	}
	id, err := h.store.Create(r.Context(), AudioObject{
		TenantID: principal.TenantID, UserID: principal.UserID, CorpID: corpID,
		OriginalName: filepath.Base(header.Filename), RelativePath: key,
		ContentType: contentType, SizeBytes: int64(len(payload)), SHA256: sha256Hex,
		CreatedAt: now,
	})
	if err != nil {
		_ = h.storage.Delete(r.Context(), key)
		writeEnvelope(w, http.StatusInternalServerError, "记录文件失败："+err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, "success", map[string]any{
		"id": id, "originalName": filepath.Base(header.Filename), "contentType": contentType,
		"sizeBytes": len(payload), "createdAt": now.Format(time.RFC3339),
		"playUrl": fmt.Sprintf("/dashboard/chat/media/%d/content", id),
	})
}

func (h *MediaHandler) serveContent(w http.ResponseWriter, r *http.Request, principal scrmhttp.Principal) {
	id, ok := mediaIDFromPath(r.URL.Path)
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, "invalid media id", nil)
		return
	}
	object, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if object == nil {
		writeEnvelope(w, http.StatusNotFound, "media not found", nil)
		return
	}
	if h.authorize != nil {
		if err := h.authorize.Authorize(r.Context(), principal, object.CorpID, mediaPermission); err != nil {
			writeEnvelope(w, http.StatusForbidden, "forbidden", nil)
			return
		}
	}
	reader, size, err := h.storage.Open(r.Context(), object.RelativePath)
	if err != nil {
		writeEnvelope(w, http.StatusNotFound, "media content not found", nil)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", object.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

func (h *MediaHandler) remove(w http.ResponseWriter, r *http.Request, principal scrmhttp.Principal, corpID int64) {
	id, ok := mediaIDFromPath(r.URL.Path)
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, "invalid media id", nil)
		return
	}
	object, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if object == nil || object.CorpID != corpID {
		writeEnvelope(w, http.StatusNotFound, "media not found", nil)
		return
	}
	if err := h.store.SoftDelete(r.Context(), id, principal.UserID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	_ = h.storage.Delete(r.Context(), object.RelativePath)
	writeEnvelope(w, http.StatusOK, "success", map[string]any{"id": id})
}

type audioFormat struct {
	contentType string
	extension   string
}

func detectAudioFormat(payload []byte) (audioFormat, bool) {
	switch {
	case len(payload) >= 12 && bytes.Equal(payload[0:4], []byte("RIFF")) && bytes.Equal(payload[8:12], []byte("WAVE")):
		return audioFormat{contentType: "audio/wav", extension: ".wav"}, true
	case len(payload) >= 3 && bytes.Equal(payload[0:3], []byte("ID3")):
		return audioFormat{contentType: "audio/mpeg", extension: ".mp3"}, true
	case len(payload) >= 2 && payload[0] == 0xFF && payload[1]&0xE0 == 0xE0 && payload[1]&0x06 != 0x02:
		return audioFormat{contentType: "audio/mpeg", extension: ".mp3"}, true
	case len(payload) >= 4 && bytes.Equal(payload[0:4], []byte("OggS")):
		return audioFormat{contentType: "audio/ogg", extension: ".ogg"}, true
	case len(payload) >= 4 && bytes.Equal(payload[0:4], []byte("fLaC")):
		return audioFormat{contentType: "audio/flac", extension: ".flac"}, true
	case len(payload) >= 12 && bytes.Equal(payload[4:8], []byte("ftyp")):
		return audioFormat{contentType: "audio/mp4", extension: ".m4a"}, true
	case len(payload) >= 6 && bytes.Equal(payload[0:6], []byte("#!AMR")):
		return audioFormat{contentType: "audio/amr", extension: ".amr"}, true
	case len(payload) >= 4 && bytes.Equal(payload[0:4], []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return audioFormat{contentType: "audio/webm", extension: ".webm"}, true
	case len(payload) >= 2 && payload[0] == 0xFF && payload[1]&0xF6 == 0xF0:
		return audioFormat{contentType: "audio/aac", extension: ".aac"}, true
	}
	return audioFormat{}, false
}

func parseCorpID(r *http.Request) (int64, error) {
	corpID, err := strconv.ParseInt(r.URL.Query().Get("corpId"), 10, 64)
	if err != nil || corpID <= 0 {
		return 0, errors.New("corpId required")
	}
	return corpID, nil
}

func mediaIDFromPath(path string) (int64, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// /dashboard/chat/media/{id}[/content]
	if len(parts) < 4 || parts[0] != "dashboard" || parts[1] != "chat" || parts[2] != "media" {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[3], 10, 64)
	return id, err == nil && id > 0
}

func sha256HexOf(payload []byte) (string, error) {
	hash := sha256.New()
	_, err := hash.Write(payload)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeEnvelope(w http.ResponseWriter, code int, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": message, "data": data})
}
