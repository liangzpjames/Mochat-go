package dashboardauth

import (
	"context"
	"errors"
	"strings"

	"jiyi/mochat-go/internal/authpassword"
	"jiyi/mochat-go/internal/authrealm"
	"time"
)

const (
	DashboardIdentityStatusActive   = 1
	DashboardIdentityStatusDisabled = 2

	DashboardMFAStatusPending  = 0
	DashboardMFAStatusActive   = 1
	DashboardMFAStatusDisabled = 2

	DashboardMFAChallengeEnrollment     = "enrollment"
	DashboardMFAChallengeLogin          = "login_mfa"
	DashboardMFAChallengePasswordChange = "password_change"
)

var (
	ErrIdentityNotFound    = errors.New("dashboard identity not found")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrIdentityUnavailable = errors.New("dashboard identity unavailable")
	ErrActivationInvalid   = errors.New("dashboard activation invalid")
	ErrSessionInvalid      = errors.New("dashboard session invalid")
	ErrInvalidPassword     = errors.New("invalid password")
	ErrMFAChallengeInvalid = errors.New("dashboard MFA challenge invalid")
)

type DashboardIdentity struct {
	UserID             int
	LoginIdentifier    string
	PasswordHash       string
	Status             int
	MustRotatePassword int
	AuthVersion        uint64
	MFARequired        int
}

type ActivationStatusValue string

const (
	ActivationStatusValid               ActivationStatusValue = "valid"
	ActivationStatusExpired             ActivationStatusValue = "expired"
	ActivationStatusActivated           ActivationStatusValue = "activated"
	ActivationStatusRevoked             ActivationStatusValue = "revoked"
	ActivationStatusInvalid             ActivationStatusValue = "invalid"
	ActivationPrimaryActionActivate                           = "activate"
	ActivationPrimaryActionLogin                              = "login"
	ActivationPrimaryActionContactAdmin                       = "contact_admin"
)

type DashboardActivationStatus struct {
	Status        ActivationStatusValue `json:"status"`
	TenantName    string                `json:"tenantName"`
	AccountHint   string                `json:"accountHint"`
	ExpiresAt     int64                 `json:"expiresAt"`
	PrimaryAction string                `json:"primaryAction"`
}

type DashboardActivationStatusStore interface {
	DashboardActivationStatus(context.Context, [32]byte, time.Time) (DashboardActivationStatus, error)
}

type DashboardIdentityStore interface {
	Authenticate(ctx context.Context, loginIdentifier string) (DashboardIdentity, error)
	Activate(ctx context.Context, tokenDigest [32]byte, passwordHash string) error
	CheckSession(ctx context.Context, userID int, authVersion uint64) error
}

// DashboardMFAChallenge is intentionally a digest-addressed, one-time
// server-side object. Raw challenge tokens never cross this boundary.
type DashboardMFAChallenge struct {
	UserID           int
	AuthVersion      uint64
	ChallengeType    string
	Status           int
	Attempts         int
	MaxAttempts      int
	ExpiresAt        time.Time
	SecretCiphertext string
	EncryptionKeyID  string
}

// DashboardAuthPersistence is the durable extension used by the Dashboard
// authentication HTTP boundary. The original narrow interface remains intact
// for callers that only need password lookup, activation, or legacy version
// checks; production authentication requires this complete extension.
type DashboardAuthPersistence interface {
	DashboardIdentityStore
	MFAStatus(ctx context.Context, userID int) (int, error)
	BeginMFAEnrollment(ctx context.Context, userID int, authVersion uint64, tokenDigest [32]byte, expiresAt time.Time, secretCiphertext, keyID string) error
	CreateMFAChallenge(ctx context.Context, userID int, authVersion uint64, challengeType string, tokenDigest [32]byte, expiresAt time.Time) error
	FindMFAChallenge(ctx context.Context, tokenDigest [32]byte) (DashboardMFAChallenge, error)
	RecordMFAFailure(ctx context.Context, tokenDigest [32]byte) error
	CompleteMFAChallenge(ctx context.Context, tokenDigest [32]byte, userID int, authVersion uint64, challengeType string, totpStep int64) (DashboardIdentity, error)
	CompletePasswordChange(ctx context.Context, tokenDigest [32]byte, passwordHash string) (DashboardIdentity, error)
	CreateSession(ctx context.Context, userID int, authVersion uint64, jtiDigest [32]byte, issuedAt, expiresAt time.Time) error
	CheckSessionToken(ctx context.Context, claims authrealm.Claims) error
	RevokeSession(ctx context.Context, claims authrealm.Claims) error
	CreatePasswordReset(ctx context.Context, userID int, authVersion uint64, tokenDigest [32]byte, expiresAt time.Time) error
	CompletePasswordReset(ctx context.Context, tokenDigest [32]byte, passwordHash string) (DashboardIdentity, error)
}

type Service struct {
	store DashboardIdentityStore
}

func NewService(store DashboardIdentityStore) *Service {
	return &Service{store: store}
}

func HashPassword(password string) (string, error) {
	return authpassword.Hash(password)
}

func VerifyPassword(hash string, password string) bool {
	return authpassword.Verify(hash, password)
}

func (service *Service) Authenticate(ctx context.Context, loginIdentifier string, password string) (DashboardIdentity, error) {
	if service == nil || service.store == nil || strings.TrimSpace(loginIdentifier) == "" || strings.TrimSpace(password) == "" {
		return DashboardIdentity{}, ErrInvalidCredentials
	}
	identity, err := service.store.Authenticate(ctx, strings.TrimSpace(loginIdentifier))
	if err != nil {
		authpassword.Verify("", password)
		if errors.Is(err, ErrIdentityNotFound) {
			return DashboardIdentity{}, ErrInvalidCredentials
		}
		return DashboardIdentity{}, ErrIdentityUnavailable
	}
	passwordOK := authpassword.Verify(identity.PasswordHash, password)
	if identity.Status != DashboardIdentityStatusActive || !passwordOK {
		return DashboardIdentity{}, ErrInvalidCredentials
	}
	identity.PasswordHash = ""
	return identity, nil
}

func (service *Service) Activate(ctx context.Context, tokenDigest [32]byte, password string) error {
	if service == nil || service.store == nil || tokenDigest == ([32]byte{}) || strings.TrimSpace(password) == "" {
		return ErrInvalidPassword
	}
	hash, err := authpassword.Hash(password)
	if err != nil {
		return ErrInvalidPassword
	}
	if err := service.store.Activate(ctx, tokenDigest, hash); err != nil {
		return ErrActivationInvalid
	}
	return nil
}

func (service *Service) CheckSession(ctx context.Context, userID int, authVersion uint64) error {
	if service == nil || service.store == nil || userID <= 0 || authVersion == 0 {
		return ErrSessionInvalid
	}
	if err := service.store.CheckSession(ctx, userID, authVersion); err != nil {
		return ErrSessionInvalid
	}
	return nil
}

func (service *Service) CheckTokenSession(ctx context.Context, claims authrealm.Claims) error {
	if service == nil || service.store == nil || claims.UserID <= 0 || claims.AuthVersion == 0 || strings.TrimSpace(claims.JWTID) == "" {
		return ErrSessionInvalid
	}
	persistence, ok := service.store.(DashboardAuthPersistence)
	if !ok {
		return ErrSessionInvalid
	}
	if err := persistence.CheckSessionToken(ctx, claims); err != nil {
		return ErrSessionInvalid
	}
	return nil
}
