package saasauth

import (
	"context"
	"errors"
	"time"

	"jiyi/mochat-go/internal/authrealm"
)

const (
	SaaSMFAStatusPending  = 0
	SaaSMFAStatusActive   = 1
	SaaSMFAStatusDisabled = 2

	SaaSMFAChallengeEnrollment     = "enrollment"
	SaaSMFAChallengeLogin          = "login_mfa"
	SaaSMFAChallengePasswordChange = "password_change"
)

var (
	ErrMFAChallengeInvalid   = errors.New("saas MFA challenge invalid")
	ErrMFAEnrollmentRequired = errors.New("saas MFA enrollment required")
)

type SaaSMFAChallenge struct {
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

type SaaSAuthPersistence interface {
	MFAStatus(context.Context, int) (int, error)
	BeginMFAEnrollment(context.Context, int, uint64, [32]byte, time.Time, string, string) error
	CreateMFAChallenge(context.Context, int, uint64, string, [32]byte, time.Time) error
	FindMFAChallenge(context.Context, [32]byte) (SaaSMFAChallenge, error)
	RecordMFAFailure(context.Context, [32]byte) error
	CompleteMFAChallenge(context.Context, [32]byte, int, uint64, string, int64) (SaaSIdentity, error)
	CompletePasswordChange(context.Context, [32]byte, string) (SaaSIdentity, error)
	CreateSession(context.Context, int, uint64, [32]byte, time.Time, time.Time) error
	CheckSessionToken(context.Context, authrealm.Claims) error
	RevokeSession(context.Context, authrealm.Claims) error
}
