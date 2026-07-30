package authjwt

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrMissingSecret      = errors.New("jwt secret is required")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenNotActive     = errors.New("token not active")
	ErrTokenBlacklisted   = errors.New("token blacklisted")
	ErrSessionInvalid     = errors.New("token session invalid")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrBackendUnavailable = errors.New("authentication backend unavailable")
)

type BlacklistChecker interface {
	JWTBlacklisted(ctx context.Context, key string) (bool, error)
}

type SessionChecker interface {
	ValidateJWTSession(ctx context.Context, jti string, userID int, issuedAt time.Time, expiresAt time.Time) error
}

type Parser struct {
	Secret        string
	Prefix        string
	Blacklist     BlacklistChecker
	Sessions      SessionChecker
	SkipBlacklist bool
	Now           func() time.Time
}

type TokenOptions struct {
	Secret string
	TTL    time.Duration
	Now    time.Time
	UID    int
	Issuer string
}

func (p Parser) UserID(r *http.Request) (int, error) {
	token := TokenFromRequest(r)
	if token == "" {
		return 0, ErrUnauthorized
	}

	payload, err := p.Parse(r.Context(), token)
	if err != nil {
		return 0, err
	}

	userID, ok := intFromPayload(payload, "uid")
	if !ok || userID <= 0 {
		return 0, ErrUnauthorized
	}
	return userID, nil
}

func (p Parser) Parse(ctx context.Context, token string) (map[string]any, error) {
	if p.Secret == "" {
		return nil, ErrMissingSecret
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, ErrInvalidToken
	}

	headerRaw, err := decodeBase64URL(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: header", ErrInvalidToken)
	}
	payloadRaw, err := decodeBase64URL(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: payload", ErrInvalidToken)
	}
	signatureRaw, err := decodeBase64URL(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: signature", ErrInvalidToken)
	}

	if _, err := decodeJSONObject(headerRaw); err != nil {
		return nil, fmt.Errorf("%w: header", ErrInvalidToken)
	}
	payload, err := decodeJSONObject(payloadRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: payload", ErrInvalidToken)
	}

	signingInput := parts[0] + "." + parts[1]
	password := md5Hex(signingInput + p.Secret)
	if !checkPHPPasswordHash(string(signatureRaw), password) {
		return nil, ErrInvalidSignature
	}

	now := time.Now()
	if p.Now != nil {
		now = p.Now()
	}
	nowUnix := now.Unix()

	if exp, ok := int64FromPayload(payload, "exp"); ok && exp <= nowUnix {
		return nil, ErrTokenExpired
	}
	if nbf, ok := int64FromPayload(payload, "nbf"); ok && nbf > nowUnix {
		return nil, ErrTokenNotActive
	}

	if !p.SkipBlacklist {
		if p.Blacklist == nil {
			return nil, ErrBackendUnavailable
		}
		key := BlacklistKey(p.Prefix, payload, token)
		blacklisted, err := p.Blacklist.JWTBlacklisted(ctx, key)
		if err != nil {
			return nil, ErrBackendUnavailable
		}
		if blacklisted {
			return nil, ErrTokenBlacklisted
		}
	}

	if p.Sessions != nil {
		jti, jtiOK := stringFromPayload(payload, "jti")
		userID, userOK := intFromPayload(payload, "uid")
		iat, iatOK := int64FromPayload(payload, "iat")
		exp, expOK := int64FromPayload(payload, "exp")
		if !jtiOK || jti == "" || !userOK || userID <= 0 || !iatOK || !expOK || exp <= iat {
			return nil, ErrSessionInvalid
		}
		if err := p.Sessions.ValidateJWTSession(ctx, jti, userID, time.Unix(iat, 0), time.Unix(exp, 0)); err != nil {
			var invalid interface {
				InvalidSession() bool
			}
			if errors.As(err, &invalid) && invalid.InvalidSession() {
				return nil, ErrSessionInvalid
			}
			return nil, ErrBackendUnavailable
		}
	}

	return payload, nil
}

func TokenFromRequest(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	if token := r.FormValue("token"); token != "" {
		return token
	}
	return ""
}

func BlacklistKey(prefix string, payload map[string]any, token string) string {
	if prefix == "" {
		prefix = "default"
	}

	jti, ok := stringFromPayload(payload, "jti")
	if !ok || jti == "" {
		jti = md5Hex(token)
	}

	id := "jwt:blacklist:" + prefix + ":" + jti
	return "[" + id + "][1]"
}

func UserIDFromPayload(payload map[string]any) (int, bool) {
	return intFromPayload(payload, "uid")
}

func CheckPasswordHash(secret string, password string, hash string) bool {
	if secret == "" || hash == "" {
		return false
	}
	return checkPHPPasswordHash(hash, md5Hex(password+secret))
}

func GeneratePasswordHash(secret string, password string) (string, error) {
	if secret == "" {
		return "", ErrMissingSecret
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(md5Hex(password+secret)), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return normalizeBcryptForPHP(string(hash)), nil
}

func MakeToken(options TokenOptions) (string, map[string]any, error) {
	if options.Secret == "" {
		return "", nil, ErrMissingSecret
	}
	if options.UID <= 0 {
		return "", nil, ErrUnauthorized
	}
	ttl := options.TTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	timestamp := now.Unix()
	issuer := options.Issuer
	if issuer == "" {
		issuer = "http://:"
	}

	nonce, err := randomHex(16)
	if err != nil {
		return "", nil, err
	}
	payload := map[string]any{
		"sub": "1",
		"iss": issuer,
		"exp": timestamp + int64(ttl/time.Second),
		"iat": timestamp,
		"nbf": timestamp,
		"uid": options.UID,
		"s":   nonce,
	}

	jtiSeed, err := randomHex(16)
	if err != nil {
		return "", nil, err
	}
	payload["jti"] = md5Hex(strconv.FormatInt(timestamp, 10) + "-" + strconv.Itoa(options.UID) + "-" + jtiSeed)

	headerRaw := []byte(`{"typ":"jwt"}`)
	payloadRaw, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	header := base64.RawURLEncoding.EncodeToString(headerRaw)
	body := base64.RawURLEncoding.EncodeToString(payloadRaw)
	signingInput := header + "." + body
	signature, err := GeneratePasswordHash(options.Secret, signingInput)
	if err != nil {
		return "", nil, err
	}
	token := signingInput + "." + base64.RawURLEncoding.EncodeToString([]byte(signature))
	return token, payload, nil
}

func decodeBase64URL(value string) ([]byte, error) {
	if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}

	padded := value
	if rem := len(padded) % 4; rem != 0 {
		padded += strings.Repeat("=", 4-rem)
	}
	return base64.URLEncoding.DecodeString(padded)
}

func decodeJSONObject(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var out map[string]any
	if err := decoder.Decode(&out); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, errors.New("not an object")
	}
	return out, nil
}

func checkPHPPasswordHash(signature string, password string) bool {
	if bcrypt.CompareHashAndPassword([]byte(signature), []byte(password)) == nil {
		return true
	}

	if strings.HasPrefix(signature, "$2y$") {
		normalized := "$2a$" + strings.TrimPrefix(signature, "$2y$")
		return bcrypt.CompareHashAndPassword([]byte(normalized), []byte(password)) == nil
	}

	return false
}

func normalizeBcryptForPHP(hash string) string {
	if strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") {
		return "$2y$" + hash[4:]
	}
	return hash
}

func intFromPayload(payload map[string]any, key string) (int, bool) {
	value, ok := int64FromPayload(payload, key)
	if !ok || value > int64(^uint(0)>>1) {
		return 0, false
	}
	return int(value), true
}

func int64FromPayload(payload map[string]any, key string) (int64, bool) {
	switch value := payload[key].(type) {
	case json.Number:
		n, err := value.Int64()
		return n, err == nil
	case float64:
		return int64(value), true
	case int64:
		return value, true
	case int:
		return int64(value), true
	case string:
		n, err := strconv.ParseInt(value, 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func stringFromPayload(payload map[string]any, key string) (string, bool) {
	switch value := payload[key].(type) {
	case string:
		return value, true
	case json.Number:
		return value.String(), true
	default:
		return "", false
	}
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
