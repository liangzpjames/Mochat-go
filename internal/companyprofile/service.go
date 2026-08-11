package companyprofile

import (
	"context"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

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

func (s *Service) RotateArchiveCredentials(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input ArchiveCredentialsInput) (Profile, error) {
	if err := s.authorize(principal, true); err != nil {
		return Profile{}, err
	}
	if input.ExpectedVersion == 0 || input.ChatSecret == nil {
		return Profile{}, ErrInvalidRequest
	}
	if err := s.requireStore(); err != nil {
		return Profile{}, err
	}
	return s.store.RotateArchiveCredentials(ctx, principal, input)
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
