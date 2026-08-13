package companyprofile

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"regexp"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

var weComAgentIDPattern = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

type Service struct {
	store         Store
	verifier      WeComVerifier
	syncScheduler EmployeeSyncScheduler
}

func NewService(store Store, verifier WeComVerifier) *Service {
	return &Service{store: store, verifier: verifier}
}

func (s *Service) WithEmployeeSyncScheduler(scheduler EmployeeSyncScheduler) *Service {
	if s != nil {
		s.syncScheduler = scheduler
	}
	return s
}

func (s *Service) authorize(principal dashboardprincipal.DashboardPrincipal, allowPending bool) error {
	if principal.UserID <= 0 || principal.TenantID <= 0 || principal.CorpID <= 0 || principal.AuthVersion == 0 || !principal.IsSuperAdmin {
		return ErrPermissionDenied
	}
	if principal.CorpStatus == dashboardprincipal.CorpBindingStatusSuspended {
		return ErrTenantAccessDenied
	}
	if principal.CorpStatus != dashboardprincipal.CorpBindingStatusActive &&
		(!allowPending || principal.CorpStatus != dashboardprincipal.CorpBindingStatusPending) {
		return ErrTenantAccessDenied
	}
	return nil
}

func (s *Service) requireStore() error {
	if s == nil || s.store == nil {
		return ErrStoreUnavailable
	}
	return nil
}

func (s *Service) GetProfile(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (Profile, error) {
	if err := s.authorize(principal, true); err != nil {
		return Profile{}, err
	}
	if err := s.requireStore(); err != nil {
		return Profile{}, err
	}
	return s.store.GetProfile(ctx, principal)
}

func (s *Service) UpdateProfile(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input UpdateProfileInput) (Profile, error) {
	if err := s.authorize(principal, true); err != nil {
		return Profile{}, err
	}
	if input.ExpectedVersion == 0 || strings.TrimSpace(input.DisplayName) == "" || len([]rune(strings.TrimSpace(input.DisplayName))) > 255 {
		return Profile{}, ErrInvalidRequest
	}
	if err := s.requireStore(); err != nil {
		return Profile{}, err
	}
	return s.store.UpdateProfile(ctx, principal, input)
}

func (s *Service) RotateWeComCredentials(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input WeComCredentialsInput) (Profile, error) {
	if err := s.authorize(principal, true); err != nil {
		return Profile{}, err
	}
	if input.WXCorpID != nil {
		return Profile{}, ErrCorpIDImmutable
	}
	if input.ExpectedVersion == 0 || !hasWeComCredentialInput(input) {
		return Profile{}, ErrInvalidRequest
	}
	if err := s.requireStore(); err != nil {
		return Profile{}, err
	}
	return s.store.RotateWeComCredentials(ctx, principal, input)
}

func (s *Service) RotateAgentCredentials(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input AgentCredentialsInput) (Profile, error) {
	if err := s.authorize(principal, true); err != nil {
		return Profile{}, err
	}
	if input.ExpectedVersion == 0 || (input.AgentID <= 0 && strings.TrimSpace(input.WXAgentID) == "") {
		return Profile{}, ErrInvalidRequest
	}
	if input.WXSecret != nil && strings.TrimSpace(*input.WXSecret) == "" {
		input.WXSecret = nil
	}
	if err := s.requireStore(); err != nil {
		return Profile{}, err
	}
	return s.store.RotateAgentCredentials(ctx, principal, input)
}

func (s *Service) ConfigureApplication(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input ApplicationCredentialsInput) (Profile, error) {
	if err := s.authorize(principal, true); err != nil {
		return Profile{}, err
	}
	input.WXAgentID = strings.TrimSpace(input.WXAgentID)
	input.Secret = strings.TrimSpace(input.Secret)
	if input.ExpectedVersion == 0 || !weComAgentIDPattern.MatchString(input.WXAgentID) || input.Secret == "" {
		return Profile{}, ErrInvalidRequest
	}
	if err := s.requireStore(); err != nil {
		return Profile{}, err
	}
	var err error
	input.CallbackToken, input.EncodingAESKey, err = GenerateCallbackConfiguration()
	if err != nil {
		return Profile{}, ErrStoreUnavailable
	}
	return s.store.ConfigureApplication(ctx, principal, input)
}

func (s *Service) RotateArchiveCredentials(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input ArchiveCredentialsInput) (Profile, error) {
	if err := s.authorize(principal, true); err != nil {
		return Profile{}, err
	}
	if input.ExpectedVersion == 0 || (input.ChatSecret == nil && input.RSAPublicKey == nil && input.RSAPrivateKey == nil) {
		return Profile{}, ErrInvalidRequest
	}
	if input.ChatSecret != nil {
		value := strings.TrimSpace(*input.ChatSecret)
		if value == "" {
			input.ChatSecret = nil
		} else {
			input.ChatSecret = &value
		}
	}
	if input.RSAPublicKey != nil || input.RSAPrivateKey != nil {
		if input.RSAPublicKey == nil || input.RSAPrivateKey == nil {
			return Profile{}, ErrInvalidRequest
		}
		publicKey := strings.TrimSpace(*input.RSAPublicKey)
		privateKey := strings.TrimSpace(*input.RSAPrivateKey)
		if !matchingRSAKeyPair(publicKey, privateKey) {
			return Profile{}, ErrInvalidRequest
		}
		input.RSAPublicKey = &publicKey
		input.RSAPrivateKey = &privateKey
	}
	if input.ChatSecret == nil && input.RSAPublicKey == nil {
		return Profile{}, ErrInvalidRequest
	}
	if err := s.requireStore(); err != nil {
		return Profile{}, err
	}
	return s.store.RotateArchiveCredentials(ctx, principal, input)
}

func (s *Service) GetCallbackConfiguration(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (CallbackConfiguration, error) {
	if err := s.authorize(principal, true); err != nil {
		return CallbackConfiguration{}, err
	}
	if err := s.requireStore(); err != nil {
		return CallbackConfiguration{}, err
	}
	return s.store.GetCallbackConfiguration(ctx, principal)
}

func (s *Service) RegenerateCallbackConfiguration(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input CallbackConfigurationInput) (CallbackConfiguration, error) {
	if err := s.authorize(principal, true); err != nil {
		return CallbackConfiguration{}, err
	}
	if input.ExpectedVersion == 0 {
		return CallbackConfiguration{}, ErrInvalidRequest
	}
	var err error
	input.Token, input.EncodingAESKey, err = GenerateCallbackConfiguration()
	if err != nil {
		return CallbackConfiguration{}, ErrStoreUnavailable
	}
	if err := s.requireStore(); err != nil {
		return CallbackConfiguration{}, err
	}
	return s.store.RegenerateCallbackConfiguration(ctx, principal, input)
}

func matchingRSAKeyPair(publicPEM, privatePEM string) bool {
	publicBlock, _ := pem.Decode([]byte(publicPEM))
	privateBlock, _ := pem.Decode([]byte(privatePEM))
	if publicBlock == nil || privateBlock == nil {
		return false
	}
	var publicKey *rsa.PublicKey
	if parsed, err := x509.ParsePKIXPublicKey(publicBlock.Bytes); err == nil {
		publicKey, _ = parsed.(*rsa.PublicKey)
	} else if parsed, parseErr := x509.ParsePKCS1PublicKey(publicBlock.Bytes); parseErr == nil {
		publicKey = parsed
	}
	var privateKey *rsa.PrivateKey
	if parsed, err := x509.ParsePKCS1PrivateKey(privateBlock.Bytes); err == nil {
		privateKey = parsed
	} else if parsed, parseErr := x509.ParsePKCS8PrivateKey(privateBlock.Bytes); parseErr == nil {
		privateKey, _ = parsed.(*rsa.PrivateKey)
	}
	return publicKey != nil && privateKey != nil && publicKey.E == privateKey.PublicKey.E && publicKey.N.Cmp(privateKey.PublicKey.N) == 0 && publicKey.N.Sign() > 0
}

func GenerateCallbackConfiguration() (string, string, error) {
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", err
	}
	encodingAESKey, err := randomAlphanumeric(43)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(tokenBytes), encodingAESKey, nil
}

func randomAlphanumeric(length int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 0, length)
	random := make([]byte, length)
	for len(result) < length {
		if _, err := rand.Read(random); err != nil {
			return "", err
		}
		for _, value := range random {
			// 248 is the largest multiple of 62 below 256. Rejecting the
			// remaining values avoids modulo bias while keeping the key
			// compatible with WeCom's alphanumeric input constraint.
			if value >= 248 {
				continue
			}
			result = append(result, alphabet[int(value)%len(alphabet)])
			if len(result) == length {
				break
			}
		}
	}
	return string(result), nil
}

func (s *Service) Verify(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input VerifyInput) (Profile, error) {
	if err := s.authorize(principal, true); err != nil {
		return Profile{}, err
	}
	if input.ExpectedVersion == 0 {
		return Profile{}, ErrInvalidRequest
	}
	if err := s.requireStore(); err != nil {
		return Profile{}, err
	}
	snapshot, err := s.store.GetVerificationSnapshot(ctx, principal)
	if err != nil {
		return Profile{}, err
	}
	if snapshot.BindingVersion != input.ExpectedVersion {
		return Profile{}, ErrVersionConflict
	}
	wxCorpID := strings.TrimSpace(input.WXCorpID)
	if snapshot.Verified {
		if wxCorpID != "" {
			return Profile{}, ErrCorpIDImmutable
		}
		wxCorpID = strings.TrimSpace(snapshot.WXCorpID)
	}
	if wxCorpID == "" || s.verifier == nil {
		if s.verifier == nil {
			return Profile{}, ErrVerifierUnavailable
		}
		return Profile{}, ErrInvalidRequest
	}
	verification, err := s.verifier.Verify(ctx, VerificationRequest{TenantID: principal.TenantID, CorpID: principal.CorpID, WXCorpID: wxCorpID, Credentials: snapshot.Credentials})
	if err != nil {
		return Profile{}, ErrCredentialInvalid
	}
	verification.WXCorpID = strings.TrimSpace(verification.WXCorpID)
	if verification.WXCorpID == "" {
		verification.WXCorpID = wxCorpID
	}
	if snapshot.Verified && verification.WXCorpID != strings.TrimSpace(snapshot.WXCorpID) {
		return Profile{}, ErrCorpIDImmutable
	}
	if strings.TrimSpace(verification.CorpName) == "" {
		return Profile{}, ErrCredentialInvalid
	}
	return s.store.CommitVerification(ctx, principal, input, verification)
}

func (s *Service) StartEmployeeSync(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (SyncResult, error) {
	if err := s.authorize(principal, false); err != nil {
		return SyncResult{}, err
	}
	if s == nil || s.store == nil || s.syncScheduler == nil {
		return SyncResult{}, ErrStoreUnavailable
	}
	snapshot, err := s.store.GetVerificationSnapshot(ctx, principal)
	if err != nil {
		return SyncResult{}, err
	}
	if !snapshot.Verified || strings.TrimSpace(snapshot.WXCorpID) == "" {
		return SyncResult{}, ErrTenantAccessDenied
	}
	syncStore, ok := s.store.(EmployeeSyncQueueStore)
	if !ok {
		return SyncResult{}, ErrStoreUnavailable
	}
	result := SyncResult{Status: "queued", StartedAt: time.Now().UTC()}
	// Redis is the queue authority. Do not write SYNC_QUEUED before this
	// call succeeds; an enqueue failure must not leave a durable queued marker.
	cursor, err := s.syncScheduler.EnqueueEmployeeSync(ctx, principal.TenantID)
	if err != nil {
		result.Status = "failed"
		result.FinishedAt = time.Now().UTC()
		result.ErrorCode = "SYNC_FAILED"
		return result, ErrStoreUnavailable
	}
	queueResult, err := syncStore.QueueEmployeeSync(ctx, principal)
	if err != nil {
		// Redis is already the durable queue authority. The worker owns the
		// running/completed/failed lifecycle; a request-side marker failure must
		// not overwrite a worker that completed concurrently. Return the safe
		// queued acknowledgement and let the worker reconcile the marker.
		result.Cursor = strings.TrimSpace(cursor)
		return result, nil
	}
	result.Cursor = queueResult.Cursor
	if strings.TrimSpace(cursor) != "" {
		result.Cursor = strings.TrimSpace(cursor)
	}
	return result, nil
}

func (s *Service) GetSyncStatus(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (SyncStatus, error) {
	if err := s.authorize(principal, false); err != nil {
		return SyncStatus{}, err
	}
	if s == nil || s.store == nil {
		return SyncStatus{}, ErrStoreUnavailable
	}
	syncStore, ok := s.store.(SyncStore)
	if !ok {
		return SyncStatus{}, ErrStoreUnavailable
	}
	return syncStore.GetSyncStatus(ctx, principal)
}

func (s *Service) ListAudits(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, filter AuditFilter) (AuditPage, error) {
	if err := s.authorize(principal, true); err != nil {
		return AuditPage{}, err
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 || filter.PerPage > 100 {
		filter.PerPage = 20
	}
	if err := s.requireStore(); err != nil {
		return AuditPage{}, err
	}
	return s.store.ListAudits(ctx, principal, filter)
}

func hasWeComCredentialInput(input WeComCredentialsInput) bool {
	return input.EmployeeSecret != nil || input.ContactSecret != nil || input.CallbackToken != nil || input.EncodingAESKey != nil || input.ChatSecret != nil
}

func IsKnownError(err error) bool {
	return errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrTenantAccessDenied) ||
		errors.Is(err, ErrNotFound) || errors.Is(err, ErrVersionConflict) || errors.Is(err, ErrCorpIDImmutable) ||
		errors.Is(err, ErrCredentialInvalid) || errors.Is(err, ErrStoreUnavailable) || errors.Is(err, ErrVerifierUnavailable)
}
