package dashboard

import (
	"context"
	"net/http"
	"time"

	"jiyi/mochat-go/internal/authjwt"
)

type LogoutStore interface {
	DeleteUserCorpCache(ctx context.Context, userID int) error
	AddJWTBlacklist(ctx context.Context, key string, ttl time.Duration) error
}

type LogoutHandler struct {
	store        LogoutStore
	parser       authjwt.Parser
	blacklistTTL time.Duration
	sessions     interface {
		RevokeCurrentSession(ctx context.Context, jti string, userID int, reason string) error
	}
}

func (h *LogoutHandler) WithIdentitySessions(sessions interface {
	RevokeCurrentSession(ctx context.Context, jti string, userID int, reason string) error
}) *LogoutHandler {
	h.sessions = sessions
	return h
}

func NewLogoutHandler(store LogoutStore, parser authjwt.Parser, blacklistTTL time.Duration) *LogoutHandler {
	return &LogoutHandler{store: store, parser: parser, blacklistTTL: blacklistTTL}
}

func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	token := authjwt.TokenFromRequest(r)
	if token == "" {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	payload, err := h.parser.Parse(r.Context(), token)
	if err != nil {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	userID, ok := authjwt.UserIDFromPayload(payload)
	if !ok || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	if h.sessions != nil {
		jti, _ := payload["jti"].(string)
		if err := h.sessions.RevokeCurrentSession(r.Context(), jti, userID, "user logout"); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}

	if err := h.store.DeleteUserCorpCache(r.Context(), userID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	ttl := h.blacklistTTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	key := authjwt.BlacklistKey(h.parser.Prefix, payload, token)
	if err := h.store.AddJWTBlacklist(r.Context(), key, ttl); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}
