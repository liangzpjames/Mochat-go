package companyprofile

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

const callbackRecoveryWakeupTimeout = 50 * time.Millisecond

var (
	callbackRecoveryEventKeyPattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	callbackRecoveryRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{15,127}$`)
	callbackRecoveryEvidencePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,39}$`)
)

func (s *Service) callbackRecoveryStore() (CallbackSideEffectRecoveryStore, error) {
	if s == nil || s.store == nil {
		return nil, ErrRecoveryUnavailable
	}
	store, ok := s.store.(CallbackSideEffectRecoveryStore)
	if !ok {
		return nil, ErrRecoveryUnavailable
	}
	return store, nil
}

func (s *Service) ListCallbackSideEffects(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input CallbackSideEffectListInput) (CallbackSideEffectPage, error) {
	if err := s.authorize(ctx, principal, false); err != nil {
		return CallbackSideEffectPage{}, err
	}
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = "unknown"
	}
	if input.Status != "unknown" || input.Limit < 0 || input.Limit > 100 {
		return CallbackSideEffectPage{}, ErrInvalidRequest
	}
	if input.Limit == 0 {
		input.Limit = 50
	}
	store, err := s.callbackRecoveryStore()
	if err != nil {
		return CallbackSideEffectPage{}, err
	}
	page, err := store.ListCallbackSideEffects(ctx, principal, input)
	return page, normalizeCallbackRecoveryError(err)
}

func (s *Service) GetCallbackSideEffect(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, eventKey, actionKey string) (CallbackSideEffectDetail, error) {
	if err := s.authorize(ctx, principal, false); err != nil {
		return CallbackSideEffectDetail{}, err
	}
	if !validCallbackRecoveryIdentity(eventKey, actionKey) {
		return CallbackSideEffectDetail{}, ErrRecoveryTargetNotFound
	}
	store, err := s.callbackRecoveryStore()
	if err != nil {
		return CallbackSideEffectDetail{}, err
	}
	detail, err := store.GetCallbackSideEffect(ctx, principal, eventKey, actionKey)
	return detail, normalizeCallbackRecoveryError(err)
}

func (s *Service) ReconcileCallbackSideEffect(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, eventKey, actionKey, requestID string, input CallbackSideEffectReconcileInput) (CallbackSideEffectReconcileResult, error) {
	if err := s.authorize(ctx, principal, false); err != nil {
		return CallbackSideEffectReconcileResult{}, err
	}
	requestID = strings.TrimSpace(requestID)
	input.Decision = strings.TrimSpace(input.Decision)
	input.Reason = strings.TrimSpace(input.Reason)
	input.EvidenceKind = strings.TrimSpace(input.EvidenceKind)
	input.EvidenceRef = strings.TrimSpace(input.EvidenceRef)
	if !validCallbackRecoveryIdentity(eventKey, actionKey) {
		return CallbackSideEffectReconcileResult{}, ErrRecoveryTargetNotFound
	}
	if !callbackRecoveryRequestIDPattern.MatchString(requestID) ||
		(input.Decision != CallbackSideEffectDecisionConfirmSent && input.Decision != CallbackSideEffectDecisionConfirmNotSentAndRetry) ||
		input.ExpectedVersion == 0 || input.ExpectedInboxLeaseFence == 0 || input.Reason == "" || utf8.RuneCountInString(input.Reason) > 255 ||
		!callbackRecoveryEvidencePattern.MatchString(input.EvidenceKind) || !validCallbackRecoveryEvidence(input.Decision, input.EvidenceKind) ||
		input.EvidenceRef == "" || utf8.RuneCountInString(input.EvidenceRef) > 255 {
		return CallbackSideEffectReconcileResult{}, ErrInvalidRequest
	}
	store, err := s.callbackRecoveryStore()
	if err != nil {
		return CallbackSideEffectReconcileResult{}, err
	}
	result, err := store.ReconcileCallbackSideEffect(ctx, principal, eventKey, actionKey, requestID, input)
	if err != nil {
		return CallbackSideEffectReconcileResult{}, normalizeCallbackRecoveryError(err)
	}
	if result.InboxReplayScheduled && s.callbackWakeup != nil {
		wakeupCtx, cancelWakeup := context.WithTimeout(context.WithoutCancel(ctx), callbackRecoveryWakeupTimeout)
		result.WakeupAccepted = s.callbackWakeup.WakeWeWorkCallback(wakeupCtx) == nil
		cancelWakeup()
	}
	return result, nil
}

func validCallbackRecoveryEvidence(decision, evidenceKind string) bool {
	switch decision {
	case CallbackSideEffectDecisionConfirmSent:
		switch evidenceKind {
		case "provider_message_id", "provider_delivery_query_sent", "provider_support_confirmed_sent":
			return true
		}
	case CallbackSideEffectDecisionConfirmNotSentAndRetry:
		switch evidenceKind {
		case "provider_delivery_query_absent", "provider_request_rejected", "provider_support_confirmed_not_sent":
			return true
		}
	}
	return false
}

func normalizeCallbackRecoveryError(err error) error {
	if err == nil {
		return nil
	}
	for _, known := range []error{ErrInvalidRequest, ErrPermissionDenied, ErrTenantAccessDenied, ErrRecoveryTargetNotFound, ErrNotFound, ErrIdempotencyConflict,
		ErrVersionConflict, ErrLeaseFenceConflict, ErrCallbackLeaseActive, ErrInboxStateConflict, ErrQuarantineActive, ErrSideEffectConflict, ErrUnsupportedAction} {
		if errors.Is(err, known) {
			return err
		}
	}
	return ErrRecoveryUnavailable
}

func validCallbackRecoveryIdentity(eventKey, actionKey string) bool {
	if !callbackRecoveryEventKeyPattern.MatchString(strings.TrimSpace(eventKey)) {
		return false
	}
	switch strings.TrimSpace(actionKey) {
	case "fission.employee_reminder", "fission.customer_push":
		return true
	default:
		return false
	}
}
