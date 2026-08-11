package dashboardauth

import (
	"context"
	"errors"
	"strings"

	"jiyi/mochat-go/internal/authpassword"
)

const (
	DashboardIdentityStatusActive   = 1
	DashboardIdentityStatusDisabled = 2
)

var (
	ErrIdentityNotFound    = errors.New("dashboard identity not found")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrIdentityUnavailable = errors.New("dashboard identity unavailable")
	ErrActivationInvalid   = errors.New("dashboard activation invalid")
	ErrSessionInvalid      = errors.New("dashboard session invalid")
	ErrInvalidPassword     = errors.New("invalid password")
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

type DashboardIdentityStore interface {
	Authenticate(ctx context.Context, loginIdentifier string) (DashboardIdentity, error)
	Activate(ctx context.Context, tokenDigest [32]byte, passwordHash string) error
	CheckSession(ctx context.Context, userID int, authVersion uint64) error
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
