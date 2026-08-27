package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ArchiveComponentFilter struct {
	ID                 string
	TenantID           int
	CorpID             int
	ConversationScopes []ArchiveMediaConversationScope
}

type ArchiveComponentObject struct {
	ID                 string
	TenantID           int
	CorpID             int
	WXCorpID           string
	MessageID          string
	SourceIdentity     string
	PublicKeyVersion   uint32
	EncryptedSecretKey string
}

type ArchiveComponentContent struct {
	Type     string
	MIMEType string
	FileName string
	Body     []byte
}

type ArchiveComponentStore interface {
	ArchiveComponentByID(context.Context, ArchiveComponentFilter) (ArchiveComponentObject, bool, error)
}

type ArchiveComponentBridge interface {
	FetchArchiveComponent(context.Context, ArchiveComponentObject) (ArchiveComponentContent, error)
}

type archiveComponentSession struct {
	UserID      int
	TenantID    int
	CorpID      int
	AuthVersion uint64
	ExpiresAt   time.Time
	Object      ArchiveComponentObject
}

type ArchiveComponentHandler struct {
	store    ArchiveComponentStore
	bridge   ArchiveComponentBridge
	mu       sync.Mutex
	sessions map[string]archiveComponentSession
	now      func() time.Time
}

func NewArchiveComponentHandler(store ArchiveComponentStore, bridge ArchiveComponentBridge) *ArchiveComponentHandler {
	return &ArchiveComponentHandler{store: store, bridge: bridge, sessions: map[string]archiveComponentSession{}, now: time.Now}
}

func (handler *ArchiveComponentHandler) WithClock(now func() time.Time) *ArchiveComponentHandler {
	if handler != nil && now != nil {
		handler.mu.Lock()
		handler.now = now
		handler.mu.Unlock()
	}
	return handler
}

func (handler *ArchiveComponentHandler) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	setArchiveComponentSecurityHeaders(w.Header())
	if handler == nil || handler.store == nil || handler.bridge == nil || request == nil {
		http.NotFound(w, request)
		return
	}
	switch request.Method {
	case http.MethodPost:
		handler.createSession(w, request)
	case http.MethodGet:
		handler.serveSession(w, request)
	default:
		w.Header().Set("Allow", "GET, POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (handler *ArchiveComponentHandler) createSession(w http.ResponseWriter, request *http.Request) {
	id, ok := archiveComponentIDFromCreatePath(request.URL.Path)
	principal, principalErr := DashboardPrincipalFromContext(request.Context())
	access, accessOK := DashboardAccessFromContext(request.Context())
	if !ok || principalErr != nil || !accessOK || access.UserID != principal.UserID || access.TenantID != principal.TenantID || access.CorpID != principal.CorpID || !access.ScopeRequired {
		http.NotFound(w, request)
		return
	}
	scopes := archiveMediaConversationScopes(access)
	if len(scopes) == 0 {
		http.NotFound(w, request)
		return
	}
	object, found, err := handler.store.ArchiveComponentByID(request.Context(), ArchiveComponentFilter{
		ID: id, TenantID: principal.TenantID, CorpID: principal.CorpID, ConversationScopes: scopes,
	})
	if err != nil || !found || object.ID != id || object.TenantID != principal.TenantID || object.CorpID != principal.CorpID || object.MessageID == "" || object.PublicKeyVersion == 0 || object.EncryptedSecretKey == "" {
		http.NotFound(w, request)
		return
	}
	token, err := newArchiveComponentToken()
	if err != nil {
		http.NotFound(w, request)
		return
	}
	handler.mu.Lock()
	now := handler.currentTimeLocked()
	handler.deleteExpiredSessionsLocked(now)
	handler.sessions[token] = archiveComponentSession{UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID, AuthVersion: principal.AuthVersion, ExpiresAt: now.Add(60 * time.Second), Object: object}
	handler.mu.Unlock()
	writeArchiveComponentJSON(w, http.StatusOK, map[string]any{"sessionUrl": "/dashboard/archive/components/session/" + token, "expiresIn": 60})
}

func (handler *ArchiveComponentHandler) serveSession(w http.ResponseWriter, request *http.Request) {
	token, ok := archiveComponentTokenFromPath(request.URL.Path)
	principal, principalErr := DashboardPrincipalFromContext(request.Context())
	access, accessOK := DashboardAccessFromContext(request.Context())
	if !ok || principalErr != nil || !accessOK || access.UserID != principal.UserID || access.TenantID != principal.TenantID || access.CorpID != principal.CorpID {
		http.NotFound(w, request)
		return
	}
	handler.mu.Lock()
	session, found := handler.sessions[token]
	now := handler.currentTimeLocked()
	if found && !now.Before(session.ExpiresAt) {
		delete(handler.sessions, token)
		found = false
	}
	if found && (session.UserID != principal.UserID || session.TenantID != principal.TenantID || session.CorpID != principal.CorpID || session.AuthVersion != principal.AuthVersion) {
		handler.mu.Unlock()
		http.NotFound(w, request)
		return
	}
	if found {
		delete(handler.sessions, token)
	}
	handler.mu.Unlock()
	if !found {
		http.NotFound(w, request)
		return
	}
	content, err := handler.bridge.FetchArchiveComponent(request.Context(), session.Object)
	if err != nil || len(content.Body) == 0 {
		http.NotFound(w, request)
		return
	}
	mimeType := safeArchiveComponentMIME(content.Type, content.MIMEType)
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", formatPositiveLength(len(content.Body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content.Body)
}

func (handler *ArchiveComponentHandler) deleteExpiredSessionsLocked(now time.Time) {
	for token, session := range handler.sessions {
		if !now.Before(session.ExpiresAt) {
			delete(handler.sessions, token)
		}
	}
}

func (handler *ArchiveComponentHandler) currentTimeLocked() time.Time {
	if handler.now == nil {
		return time.Now()
	}
	return handler.now()
}

func newArchiveComponentToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func archiveComponentIDFromCreatePath(path string) (string, bool) {
	const prefix, suffix = "/dashboard/archive/components/", "/session"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	parsed, err := uuid.Parse(value)
	return parsed.String(), err == nil && parsed.String() == strings.ToLower(value) && !strings.Contains(value, "/")
}

func archiveComponentTokenFromPath(path string) (string, bool) {
	const prefix = "/dashboard/archive/components/session/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	value := strings.TrimPrefix(path, prefix)
	if len(value) != 43 || strings.Contains(value, "/") {
		return "", false
	}
	_, err := base64.RawURLEncoding.DecodeString(value)
	return value, err == nil
}

func safeArchiveComponentMIME(kind, supplied string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	supplied = strings.ToLower(strings.TrimSpace(strings.Split(supplied, ";")[0]))
	allowed := map[string]map[string]bool{
		"text":  {"text/plain": true},
		"image": {"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true},
		"voice": {"audio/mpeg": true, "audio/mp4": true, "audio/ogg": true, "audio/wav": true, "audio/aac": true},
		"video": {"video/mp4": true, "video/webm": true, "video/quicktime": true},
		"file":  {"application/pdf": true, "text/plain": true, "text/csv": true, "application/zip": true},
	}
	if allowed[kind][supplied] {
		if supplied == "text/plain" {
			return "text/plain; charset=utf-8"
		}
		return supplied
	}
	return "application/octet-stream"
}

func formatPositiveLength(value int) string {
	if value <= 0 {
		return "0"
	}
	return strconv.Itoa(value)
}

func setArchiveComponentSecurityHeaders(header http.Header) {
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", "private, no-store")
	header.Set("Content-Security-Policy", "default-src 'none'; media-src 'self' blob:; img-src 'self' blob:; sandbox")
	header.Set("X-Frame-Options", "SAMEORIGIN")
}

func writeArchiveComponentJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
