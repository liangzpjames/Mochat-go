package saasauth

import (
	"context"
	"errors"
	"strings"

	"jiyi/mochat-go/internal/authpassword"
)

const (
	SaaSIdentityStatusActive   = 1
	SaaSIdentityStatusDisabled = 2
)

var (
	ErrIdentityNotFound    = errors.New("saas identity not found")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrIdentityUnavailable = errors.New("saas identity unavailable")
	ErrInvalidBootstrap    = errors.New("invalid SaaS bootstrap input")
	ErrSessionInvalid      = errors.New("saas session invalid")
)

type SaaSIdentity struct {
	ID                 int
	LoginName          string
	Phone              string
	PasswordHash       string
	Name               string
	Status             int
	MustRotatePassword int
	AuthVersion        uint64
	MFARequired        int
}

type BootstrapSaaSAdmin struct {
	RequestKey   string
	LoginName    string
	Phone        string
	Name         string
	PasswordHash string
}

type SaaSIdentityStore interface {
	Authenticate(ctx context.Context, login string) (SaaSIdentity, error)
	Bootstrap(ctx context.Context, input BootstrapSaaSAdmin) (SaaSIdentity, error)
	CheckSession(ctx context.Context, userID int, authVersion uint64) error
}

type Service struct {
	store SaaSIdentityStore
}

func NewService(store SaaSIdentityStore) *Service {
	return &Service{store: store}
}

func HashPassword(password string) (string, error) {
	return authpassword.Hash(password)
}

func VerifyPassword(hash string, password string) bool {
	return authpassword.Verify(hash, password)
}

func (service *Service) Authenticate(ctx context.Context, login string, password string) (SaaSIdentity, error) {
	if service == nil || service.store == nil || strings.TrimSpace(login) == "" || strings.TrimSpace(password) == "" {
		return SaaSIdentity{}, ErrInvalidCredentials
	}
	identity, err := service.store.Authenticate(ctx, strings.ToLower(strings.TrimSpace(login)))
	if err != nil {
		// Always execute the same bcrypt comparison for an unknown login.
		authpassword.Verify("", password)
		if errors.Is(err, ErrIdentityNotFound) {
			return SaaSIdentity{}, ErrInvalidCredentials
		}
		return SaaSIdentity{}, ErrIdentityUnavailable
	}
	passwordOK := authpassword.Verify(identity.PasswordHash, password)
	if identity.Status != SaaSIdentityStatusActive || !passwordOK {
		return SaaSIdentity{}, ErrInvalidCredentials
	}
	identity.PasswordHash = ""
	return identity, nil
}

func (service *Service) Bootstrap(ctx context.Context, input BootstrapSaaSAdmin) (SaaSIdentity, error) {
	if service == nil || service.store == nil || strings.TrimSpace(input.RequestKey) == "" || strings.TrimSpace(input.LoginName) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.PasswordHash) == "" {
		return SaaSIdentity{}, ErrInvalidBootstrap
	}
	identity, err := service.store.Bootstrap(ctx, BootstrapSaaSAdmin{
		RequestKey:   strings.TrimSpace(input.RequestKey),
		LoginName:    strings.ToLower(strings.TrimSpace(input.LoginName)),
		Phone:        strings.TrimSpace(input.Phone),
		Name:         strings.TrimSpace(input.Name),
		PasswordHash: input.PasswordHash,
	})
	if err != nil {
		return SaaSIdentity{}, ErrIdentityUnavailable
	}
	identity.PasswordHash = ""
	return identity, nil
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
