package authjwt

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestParserExtractsUserIDFromBearerToken(t *testing.T) {
	now := time.Unix(1000, 0)
	token := testToken(t, "secret", map[string]any{
		"uid": 7,
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"jti": "jti-7",
	}, true)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/user/loginShow", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	userID, err := Parser{
		Secret:        "secret",
		SkipBlacklist: true,
		Now:           func() time.Time { return now },
	}.UserID(req)
	if err != nil {
		t.Fatal(err)
	}
	if userID != 7 {
		t.Fatalf("userID = %d", userID)
	}
}

func TestParserExtractsUserIDFromTokenParameter(t *testing.T) {
	now := time.Unix(1000, 0)
	token := testToken(t, "secret", map[string]any{
		"uid": 8,
		"exp": now.Add(time.Hour).Unix(),
		"nbf": now.Unix(),
		"jti": "jti-8",
	}, false)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/user/loginShow?token="+token, nil)

	userID, err := Parser{
		Secret:        "secret",
		SkipBlacklist: true,
		Now:           func() time.Time { return now },
	}.UserID(req)
	if err != nil {
		t.Fatal(err)
	}
	if userID != 8 {
		t.Fatalf("userID = %d", userID)
	}
}

func TestParserRejectsBadSignature(t *testing.T) {
	now := time.Unix(1000, 0)
	token := testToken(t, "secret", map[string]any{
		"uid": 7,
		"exp": now.Add(time.Hour).Unix(),
		"nbf": now.Unix(),
		"jti": "jti-7",
	}, false)

	_, err := Parser{
		Secret:        "other-secret",
		SkipBlacklist: true,
		Now:           func() time.Time { return now },
	}.Parse(context.Background(), token)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidSignature)
	}
}

func TestParserRejectsExpiredToken(t *testing.T) {
	now := time.Unix(1000, 0)
	token := testToken(t, "secret", map[string]any{
		"uid": 7,
		"exp": now.Add(-time.Second).Unix(),
		"nbf": now.Add(-time.Hour).Unix(),
		"jti": "jti-7",
	}, false)

	_, err := Parser{
		Secret:        "secret",
		SkipBlacklist: true,
		Now:           func() time.Time { return now },
	}.Parse(context.Background(), token)
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("err = %v, want %v", err, ErrTokenExpired)
	}
}

func TestParserRejectsBlacklistedToken(t *testing.T) {
	now := time.Unix(1000, 0)
	payload := map[string]any{
		"uid": 7,
		"exp": now.Add(time.Hour).Unix(),
		"nbf": now.Unix(),
		"jti": "jti-7",
	}
	token := testToken(t, "secret", payload, false)
	key := BlacklistKey("default", payload, token)

	_, err := Parser{
		Secret:    "secret",
		Prefix:    "default",
		Blacklist: staticBlacklist(key),
		Now:       func() time.Time { return now },
	}.Parse(context.Background(), token)
	if !errors.Is(err, ErrTokenBlacklisted) {
		t.Fatalf("err = %v, want %v", err, ErrTokenBlacklisted)
	}
}

func TestBlacklistKeyMatchesPHPDoctrineRedisCacheKey(t *testing.T) {
	key := BlacklistKey("mc_jwt_", map[string]any{"jti": "php-jti"}, "token-value")
	want := "[jwt:blacklist:mc_jwt_:php-jti][1]"
	if key != want {
		t.Fatalf("key = %q, want %q", key, want)
	}

	key = BlacklistKey("", map[string]any{}, "token-value")
	md5Token := testMD5Hex("token-value")
	want = "[jwt:blacklist:default:" + md5Token + "][1]"
	if key != want {
		t.Fatalf("fallback key = %q, want %q", key, want)
	}
}

func TestGeneratePasswordHashRoundTrips(t *testing.T) {
	hash, err := GeneratePasswordHash("secret", "123456")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$2y$") {
		t.Fatalf("hash prefix = %q", hash[:4])
	}
	if !CheckPasswordHash("secret", "123456", hash) {
		t.Fatalf("generated hash did not verify")
	}
	if CheckPasswordHash("secret", "bad-password", hash) {
		t.Fatalf("bad password verified")
	}
}

func TestMakeTokenRoundTripsThroughParser(t *testing.T) {
	now := time.Unix(1000, 0)
	token, payload, err := MakeToken(TokenOptions{
		Secret: "secret",
		TTL:    time.Hour,
		Now:    now,
		UID:    9,
		Issuer: "http://localhost/dashboard/user/auth",
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload["uid"].(int) != 9 {
		t.Fatalf("uid payload = %v", payload["uid"])
	}

	parsed, err := Parser{
		Secret:        "secret",
		SkipBlacklist: true,
		Now:           func() time.Time { return now.Add(time.Minute) },
	}.Parse(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	uid, ok := UserIDFromPayload(parsed)
	if !ok || uid != 9 {
		t.Fatalf("uid = %d ok=%v", uid, ok)
	}
}

func TestParserClassifiesBlacklistBackendFailureAsUnavailable(t *testing.T) {
	token := testToken(t, "secret", map[string]any{
		"uid": 9,
		"exp": time.Now().Add(time.Hour).Unix(),
	}, false)
	parser := Parser{
		Secret:    "secret",
		Blacklist: errorBlacklist{err: errors.New("redis password leaked")},
	}

	_, err := parser.Parse(context.Background(), token)
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("Parse() error = %v", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "redis") ||
		strings.Contains(strings.ToLower(err.Error()), "password") {
		t.Fatalf("Parse() leaked backend detail: %v", err)
	}
}

func TestParserDistinguishesInvalidSessionFromBackendFailure(t *testing.T) {
	now := time.Now()
	token := testToken(t, "secret", map[string]any{
		"uid": 9,
		"jti": "session-9",
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}, false)

	for _, tc := range []struct {
		name string
		err  error
		want error
	}{
		{name: "invalid session", err: invalidSessionTestError{}, want: ErrSessionInvalid},
		{name: "backend failure", err: errors.New("mysql DSN leaked"), want: ErrBackendUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parser := Parser{
				Secret:        "secret",
				SkipBlacklist: true,
				Sessions:      errorSessionChecker{err: tc.err},
			}
			_, err := parser.Parse(context.Background(), token)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Parse() error = %v, want %v", err, tc.want)
			}
			if strings.Contains(strings.ToLower(err.Error()), "mysql") ||
				strings.Contains(strings.ToLower(err.Error()), "dsn") {
				t.Fatalf("Parse() leaked backend detail: %v", err)
			}
		})
	}
}

type staticBlacklist string

func (b staticBlacklist) JWTBlacklisted(_ context.Context, key string) (bool, error) {
	return string(b) == key, nil
}

type errorBlacklist struct {
	err error
}

func (b errorBlacklist) JWTBlacklisted(context.Context, string) (bool, error) {
	return false, b.err
}

type errorSessionChecker struct {
	err error
}

func (c errorSessionChecker) ValidateJWTSession(context.Context, string, int, time.Time, time.Time) error {
	return c.err
}

type invalidSessionTestError struct{}

func (invalidSessionTestError) Error() string        { return "invalid session" }
func (invalidSessionTestError) InvalidSession() bool { return true }

func testToken(t *testing.T, secret string, payload map[string]any, php2y bool) string {
	t.Helper()

	headerJSON := []byte(`{"typ":"jwt"}`)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	body := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + body
	password := testMD5Hex(signingInput + secret)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	signature := string(hash)
	if php2y {
		if strings.HasPrefix(signature, "$2a$") || strings.HasPrefix(signature, "$2b$") {
			signature = "$2y$" + signature[4:]
		}
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString([]byte(signature))
}

func testMD5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
