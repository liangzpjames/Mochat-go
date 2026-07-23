package dashboard

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"jiyi/mochat-go/internal/authjwt"
)

func TestLogoutDeletesUserCacheAndBlacklistsToken(t *testing.T) {
	now := time.Unix(1000, 0)
	payload := map[string]any{
		"uid": 7,
		"exp": now.Add(time.Hour).Unix(),
		"nbf": now.Unix(),
		"jti": "logout-jti",
	}
	token := dashboardTestToken(t, "secret", payload)
	store := &fakeLogoutStore{}
	handler := NewLogoutHandler(store, authjwt.Parser{
		Secret:    "secret",
		Prefix:    "default",
		Blacklist: store,
		Now:       func() time.Time { return now },
	}, time.Hour)

	req := httptest.NewRequest(http.MethodPut, "/dashboard/user/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedUserID != 7 {
		t.Fatalf("deletedUserID = %d", store.deletedUserID)
	}
	wantKey := authjwt.BlacklistKey("default", payload, token)
	if store.blacklistKey != wantKey {
		t.Fatalf("blacklistKey = %q, want %q", store.blacklistKey, wantKey)
	}
	if store.blacklistTTL != time.Hour {
		t.Fatalf("blacklistTTL = %s", store.blacklistTTL)
	}
}

func TestLogoutRejectsMissingToken(t *testing.T) {
	handler := NewLogoutHandler(&fakeLogoutStore{}, authjwt.Parser{Secret: "secret"}, time.Hour)

	req := httptest.NewRequest(http.MethodPut, "/dashboard/user/logout", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogoutReportsStoreFailure(t *testing.T) {
	now := time.Unix(1000, 0)
	token := dashboardTestToken(t, "secret", map[string]any{
		"uid": 7,
		"exp": now.Add(time.Hour).Unix(),
		"nbf": now.Unix(),
		"jti": "logout-jti",
	})
	store := &fakeLogoutStore{deleteErr: errors.New("redis down")}
	handler := NewLogoutHandler(store, authjwt.Parser{
		Secret:        "secret",
		SkipBlacklist: true,
		Now:           func() time.Time { return now },
	}, time.Hour)

	req := httptest.NewRequest(http.MethodPut, "/dashboard/user/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

type fakeLogoutStore struct {
	deletedUserID int
	blacklistKey  string
	blacklistTTL  time.Duration
	deleteErr     error
	blacklisted   map[string]bool
}

func (s *fakeLogoutStore) DeleteUserCorpCache(_ context.Context, userID int) error {
	s.deletedUserID = userID
	return s.deleteErr
}

func (s *fakeLogoutStore) AddJWTBlacklist(_ context.Context, key string, ttl time.Duration) error {
	s.blacklistKey = key
	s.blacklistTTL = ttl
	return nil
}

func (s *fakeLogoutStore) JWTBlacklisted(_ context.Context, key string) (bool, error) {
	return s.blacklisted != nil && s.blacklisted[key], nil
}

func dashboardTestToken(t *testing.T, secret string, payload map[string]any) string {
	t.Helper()

	headerJSON := []byte(`{"typ":"jwt"}`)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	body := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + body
	sum := md5.Sum([]byte(signingInput + secret))
	password := hex.EncodeToString(sum[:])
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(hash)
}
