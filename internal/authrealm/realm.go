package authrealm

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Realm string

const (
	RealmSaaSAdmin Realm = "saas_admin"
	RealmDashboard Realm = "dashboard"
)

var (
	ErrInvalidConfig       = errors.New("invalid authentication realm configuration")
	ErrInvalidToken        = errors.New("invalid authentication token")
	ErrInvalidSignature    = errors.New("invalid authentication signature")
	ErrRealmMismatch       = errors.New("authentication realm mismatch")
	ErrIssuerMismatch      = errors.New("authentication issuer mismatch")
	ErrAudienceMismatch    = errors.New("authentication audience mismatch")
	ErrTokenExpired        = errors.New("authentication token expired")
	ErrTokenNotActive      = errors.New("authentication token not active")
	ErrSessionInvalid      = errors.New("authentication session invalid")
	ErrAuthVersionMismatch = errors.New("authentication version mismatch")
)

const (
	CodeSessionInvalid = "SESSION_INVALID"
	CodeRealmMismatch  = "AUTH_REALM_MISMATCH"
)

// AuthError is the only error form returned for an invalid bearer token. Its
// text is a stable public code and never contains token, secret, or backend
// details.
type AuthError struct {
	code  string
	cause error
}

func (e *AuthError) Error() string {
	if e == nil || e.code == "" {
		return CodeSessionInvalid
	}
	return e.code
}

func (e *AuthError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *AuthError) StatusCode() int { return 401 }

func (e *AuthError) Code() string { return e.Error() }

func tokenError(code string, cause error) error {
	if cause == nil {
		cause = ErrInvalidToken
	}
	return &AuthError{code: code, cause: cause}
}

type TokenConfig struct {
	Secret   []byte
	Issuer   string
	Audience string
	TTL      time.Duration
	Realm    Realm
	Prefix   string
}

func (c TokenConfig) Validate() error {
	if len([]byte(strings.TrimSpace(string(c.Secret)))) == 0 {
		return fmt.Errorf("%w: secret is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Issuer) == "" {
		return fmt.Errorf("%w: issuer is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(c.Audience) == "" {
		return fmt.Errorf("%w: audience is required", ErrInvalidConfig)
	}
	if c.TTL <= 0 {
		return fmt.Errorf("%w: ttl must be positive", ErrInvalidConfig)
	}
	if !c.Realm.Valid() {
		return fmt.Errorf("%w: realm is invalid", ErrInvalidConfig)
	}
	return nil
}

func (r Realm) Valid() bool {
	return r == RealmSaaSAdmin || r == RealmDashboard
}

func (r Realm) SubjectPrefix() string {
	if r == RealmSaaSAdmin {
		return "saas-user:"
	}
	return "dashboard-user:"
}
