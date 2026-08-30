package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/httpresponse"
	"jiyi/mochat-go/internal/moduleprincipal"
	"jiyi/mochat-go/internal/modules/chat-media/ports"
	"jiyi/mochat-go/internal/modules/providers"
)

const mediaPermission = "/chat/file-audio#get"

type PrincipalResolver = moduleprincipal.Resolver
type Authorizer = moduleprincipal.Authorizer
type Principal = moduleprincipal.Principal
type AudioObject = ports.AudioObject
type ListResult = ports.ListResult
type MediaListFilter = ports.MediaListFilter
type MediaStore = ports.MediaStore
type DurationUpdater = ports.DurationUpdater

type MediaHandler struct {
	store     MediaStore
	storage   providers.AudioProvider
	principal PrincipalResolver
	authorize Authorizer
}

func NewMediaHandler(store MediaStore, storage providers.AudioProvider, principal PrincipalResolver, authorize Authorizer) (*MediaHandler, error) {
	if store == nil {
		return nil, errors.New("chat media store is required")
	}
	if storage == nil {
		return nil, errors.New("chat media audio provider is required")
	}
	return &MediaHandler{store: store, storage: storage, principal: principal, authorize: authorize}, nil
}

func RegisterRoutes(registrar interface {
	Handle(method, pattern string, handler http.Handler) error
}, handler http.Handler) error {
	if err := registrar.Handle(http.MethodGet, "/dashboard/chat/media", handler); err != nil {
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
	if principal.CorpID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, "principal unauthorized", nil)
		return
	}
	corpID := principal.CorpID
	if h.authorize != nil {
		if err := h.authorize.Authorize(r.Context(), principal, corpID, mediaPermission); err != nil {
			writeEnvelope(w, http.StatusForbidden, "forbidden", nil)
			return
		}
	}
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	h.list(w, r, corpID)
}

func (h *MediaHandler) list(w http.ResponseWriter, r *http.Request, corpID int64) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("perPage"))
	syncedFrom, syncedTo, err := mediaSyncDateRange(r.URL.Query())
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	result, err := h.store.List(r.Context(), MediaListFilter{
		CorpID: corpID, Page: page, PerPage: perPage,
		Sender: strings.TrimSpace(r.URL.Query().Get("sender")), Receiver: strings.TrimSpace(r.URL.Query().Get("receiver")),
		SyncedFrom: syncedFrom, SyncedTo: syncedTo,
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	for index := range result.List {
		result.List[index].PlayURL = fmt.Sprintf("/dashboard/chat/media/%d/content", result.List[index].ID)
		h.hydrateDuration(r.Context(), &result.List[index])
	}
	writeEnvelope(w, http.StatusOK, "success", result)
}

func mediaSyncDateRange(q interface{ Get(string) string }) (string, string, error) {
	from, to := strings.TrimSpace(q.Get("from")), strings.TrimSpace(q.Get("to"))
	for _, value := range []string{from, to} {
		if value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return "", "", errors.New("发送日期格式必须为 YYYY-MM-DD")
		}
	}
	if from != "" && to != "" && from > to {
		return "", "", errors.New("发送日期开始不能晚于结束日期")
	}
	return from, to, nil
}

func (h *MediaHandler) hydrateDuration(ctx context.Context, object *AudioObject) {
	if object == nil || object.DurationSeconds > 0 || strings.TrimSpace(object.RelativePath) == "" {
		return
	}
	reader, _, err := h.storage.Open(ctx, object.RelativePath)
	if err != nil {
		return
	}
	defer reader.Close()
	duration, err := probeAudioDuration(reader, object.ContentType)
	if err != nil || duration <= 0 {
		return
	}
	object.DurationSeconds = duration
	if updater, ok := h.store.(DurationUpdater); ok {
		_ = updater.UpdateDuration(ctx, object.ID, duration)
	}
}

func (h *MediaHandler) serveContent(w http.ResponseWriter, r *http.Request, principal moduleprincipal.Principal) {
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
	if object == nil || object.Source != "wecom_sync" {
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
	httpresponse.AllowLongWrite(w)
	w.Header().Set("Content-Type", object.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Accept-Ranges", "bytes")
	if seeker, ok := reader.(io.ReadSeeker); ok {
		http.ServeContent(w, r, object.OriginalName, object.CreatedAt, seeker)
		return
	}
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
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

func writeEnvelope(w http.ResponseWriter, code int, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": message, "data": data})
}
