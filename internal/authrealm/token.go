package authrealm

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	jwtAlgorithm = "HS256"
	jwtType      = "JWT"
)

type Claims struct {
	Subject     string `json:"sub"`
	UserID      int    `json:"uid"`
	Realm       Realm  `json:"realm"`
	Issuer      string `json:"iss"`
	Audience    string `json:"aud"`
	JWTID       string `json:"jti"`
	IssuedAt    int64  `json:"iat"`
	NotBefore   int64  `json:"nbf"`
	ExpiresAt   int64  `json:"exp"`
	AuthVersion uint64 `json:"auth_version"`
}

type SessionValidator func(context.Context, Claims) error

type Parser struct {
	Config          TokenConfig
	Now             func() time.Time
	ValidateSession SessionValidator
}

func Sign(config TokenConfig, claims Claims, now time.Time) (string, error) {
	if err := config.Validate(); err != nil {
		return "", err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	claims, err := prepareClaims(config, claims, now)
	if err != nil {
		return "", err
	}

	header, err := json.Marshal(struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}{Algorithm: jwtAlgorithm, Type: jwtType})
	if err != nil {
		return "", fmt.Errorf("marshal token header: %w", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal token claims: %w", err)
	}
	headerPart := base64.RawURLEncoding.EncodeToString(header)
	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := headerPart + "." + payloadPart
	signature := signBytes(config.Secret, signingInput)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func prepareClaims(config TokenConfig, claims Claims, now time.Time) (Claims, error) {
	if claims.UserID <= 0 {
		return Claims{}, fmt.Errorf("user id must be positive")
	}
	if claims.AuthVersion == 0 {
		return Claims{}, fmt.Errorf("auth version must be positive")
	}
	expectedSubject := config.Realm.SubjectPrefix() + fmt.Sprint(claims.UserID)
	if claims.Subject == "" {
		claims.Subject = expectedSubject
	} else if claims.Subject != expectedSubject {
		return Claims{}, fmt.Errorf("subject does not match the configured realm")
	}
	if claims.Realm == "" {
		claims.Realm = config.Realm
	} else if claims.Realm != config.Realm {
		return Claims{}, fmt.Errorf("claims realm does not match the signer realm")
	}
	if claims.Issuer == "" {
		claims.Issuer = config.Issuer
	} else if claims.Issuer != config.Issuer {
		return Claims{}, fmt.Errorf("claims issuer does not match the signer issuer")
	}
	if claims.Audience == "" {
		claims.Audience = config.Audience
	} else if claims.Audience != config.Audience {
		return Claims{}, fmt.Errorf("claims audience does not match the signer audience")
	}
	if claims.IssuedAt == 0 {
		claims.IssuedAt = now.Unix()
	}
	if claims.NotBefore == 0 {
		claims.NotBefore = claims.IssuedAt
	}
	if claims.ExpiresAt == 0 {
		claims.ExpiresAt = now.Add(config.TTL).Unix()
	}
	if claims.JWTID == "" {
		jti, err := newJWTID()
		if err != nil {
			return Claims{}, fmt.Errorf("generate token id: %w", err)
		}
		claims.JWTID = jti
	}
	return claims, nil
}

func newJWTID() (string, error) {
	var raw [18]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func (p Parser) Parse(ctx context.Context, token string) (Claims, error) {
	if err := p.Config.Validate(); err != nil {
		return Claims{}, err
	}
	if p.ValidateSession == nil {
		return Claims{}, tokenError(CodeSessionInvalid, ErrSessionInvalid)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rawToken := strings.TrimSpace(token)
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Claims{}, tokenError(CodeSessionInvalid, ErrInvalidToken)
	}

	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != jwtAlgorithm || header.Type != jwtType {
		return Claims{}, tokenError(CodeSessionInvalid, ErrInvalidToken)
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, tokenError(CodeSessionInvalid, ErrInvalidToken)
	}
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return Claims{}, tokenError(CodeSessionInvalid, ErrInvalidToken)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, tokenError(CodeSessionInvalid, ErrInvalidSignature)
	}
	expectedSignature := signBytes(p.Config.Secret, parts[0]+"."+parts[1])
	if !hmac.Equal(signature, expectedSignature) {
		return Claims{}, tokenError(CodeSessionInvalid, ErrInvalidSignature)
	}

	if claims.Realm != p.Config.Realm {
		return Claims{}, tokenError(CodeRealmMismatch, ErrRealmMismatch)
	}
	if claims.Issuer != p.Config.Issuer {
		return Claims{}, tokenError(CodeSessionInvalid, ErrIssuerMismatch)
	}
	if claims.Audience != p.Config.Audience {
		return Claims{}, tokenError(CodeSessionInvalid, ErrAudienceMismatch)
	}
	if !claimsValidForRealm(claims) {
		return Claims{}, tokenError(CodeSessionInvalid, ErrInvalidToken)
	}

	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}
	nowUnix := now.Unix()
	if claims.ExpiresAt <= nowUnix {
		return Claims{}, tokenError(CodeSessionInvalid, ErrTokenExpired)
	}
	if claims.NotBefore > nowUnix {
		return Claims{}, tokenError(CodeSessionInvalid, ErrTokenNotActive)
	}
	if claims.ExpiresAt <= claims.IssuedAt || claims.NotBefore > claims.ExpiresAt {
		return Claims{}, tokenError(CodeSessionInvalid, ErrInvalidToken)
	}

	if err := p.ValidateSession(ctx, claims); err != nil {
		if errors.Is(err, ErrAuthVersionMismatch) {
			return Claims{}, tokenError(CodeSessionInvalid, ErrAuthVersionMismatch)
		}
		return Claims{}, tokenError(CodeSessionInvalid, ErrSessionInvalid)
	}
	return claims, nil
}

func (p Parser) ParseToken(token string) (Claims, error) {
	return p.Parse(context.Background(), token)
}

func (p Parser) ParseWithAuthVersion(ctx context.Context, token string, expected uint64) (Claims, error) {
	claims, err := p.Parse(ctx, token)
	if err != nil {
		return Claims{}, err
	}
	if expected == 0 || claims.AuthVersion != expected {
		return Claims{}, tokenError(CodeSessionInvalid, ErrAuthVersionMismatch)
	}
	return claims, nil
}

func claimsValidForRealm(claims Claims) bool {
	if claims.UserID <= 0 || claims.AuthVersion == 0 || claims.JWTID == "" || claims.IssuedAt <= 0 || claims.NotBefore <= 0 || claims.ExpiresAt <= 0 {
		return false
	}
	return claims.Subject == claims.Realm.SubjectPrefix()+fmt.Sprint(claims.UserID)
}

func signBytes(secret []byte, input string) []byte {
	hash := hmac.New(sha256.New, secret)
	_, _ = hash.Write([]byte(input))
	return hash.Sum(nil)
}
