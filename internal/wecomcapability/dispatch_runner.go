package wecomcapability

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrDispatchAlreadyClaimed      = errors.New("dispatch is already claimed")
	ErrDispatchAuthorizationDenied = errors.New("dispatch authorization denied")
	ErrDispatchCredentialStale     = errors.New("dispatch credential generation is stale")
	ErrDispatchInvalidState        = errors.New("dispatch state is invalid")
	ErrDispatchContract            = errors.New("dispatch provider contract is invalid")
	ErrDispatchReconcileRequired   = errors.New("dispatch requires external reconciliation")
)

const (
	dispatchAuthorizationPreclaim = "preclaim"
	dispatchAuthorizationClaim    = "claim"
)

// DispatchPrincipal is the minimal authenticated, tenant-scoped identity
// needed by a dispatch runner. It deliberately contains no credential or
// provider payload.
type DispatchPrincipal struct {
	UserID       int
	TenantID     int
	CorpID       int
	IsSuperAdmin bool
	AuthVersion  uint64
}

type DispatchClaimRequest struct {
	Principal                 DispatchPrincipal
	DispatchID                int64
	ExpectedCredentialVersion uint64
	LeaseDuration             time.Duration
}

type DispatchTransitionRequest struct {
	Principal         DispatchPrincipal
	DispatchID        int64
	Status            string
	LeaseToken        string
	Attempt           int
	ProviderRequestID string
	ProviderMessageID string
	ProviderObjectID  string
	NextPollAt        *time.Time
	NextPollDelay     time.Duration
	LastErrorCode     string
}

type DispatchReconcileRequest struct {
	Principal         DispatchPrincipal
	DispatchID        int64
	LeaseToken        string
	Attempt           int
	ProviderRequestID string
	ProviderMessageID string
	ProviderObjectID  string
	ErrorCode         string
	NextPollAt        *time.Time
	NextPollDelay     time.Duration
}

type DispatchResultRequest struct {
	Principal        DispatchPrincipal
	DispatchID       int64
	LeaseToken       string
	Attempt          int
	TargetKind       string
	TargetID         string
	Status           string
	ProviderTargetID string
	ErrorCode        string
	ErrorMessageSafe string
}

type DispatchKind string

const (
	DispatchKindContactBatch DispatchKind = "contact_batch"
	DispatchKindRoomBatch    DispatchKind = "room_batch"
)

type OperationAggregateRequest struct {
	Principal                 DispatchPrincipal
	OperationID               int64
	Capability                string
	ExpectedCredentialVersion uint64
}

type OperationAggregate struct {
	OperationID int64
	Capability  string
	Status      string
	Dispatches  []Dispatch
	Results     []OperationResult
}

// DispatchLedger is the durable boundary. Implementations must perform
// scope, generation, lease, idempotency, and audit/event checks in the same
// transaction as each mutation.
type DispatchLedger interface {
	ClaimDispatch(context.Context, DispatchClaimRequest) (Dispatch, error)
	TransitionDispatch(context.Context, DispatchTransitionRequest) (Dispatch, error)
	PersistDispatchReconcile(context.Context, DispatchReconcileRequest) (Dispatch, error)
	RecordDispatchResult(context.Context, DispatchResultRequest) (OperationResult, error)
	AggregateOperation(context.Context, OperationAggregateRequest) (OperationAggregate, error)
}

type DispatchAuthorizationRequest struct {
	Principal                 DispatchPrincipal
	Capability                string
	DispatchID                int64
	OperationID               int64
	TargetID                  string
	ExpectedCredentialVersion uint64
	Stage                     string
}

// DispatchAuthorizer is called both before and after the durable claim. The
// second call is intentional: SaaS package/quota, tenant-corp ownership,
// dashboard scope, and target ownership may change while a claim is pending.
type DispatchAuthorizer interface {
	AuthorizeDispatch(context.Context, DispatchAuthorizationRequest) error
}

type DispatchSubmitRequest struct {
	Principal                 DispatchPrincipal
	Capability                string
	Dispatch                  Dispatch
	ExpectedCredentialVersion uint64
}

type DispatchSubmitResult struct {
	Submitted         bool
	ProviderRequestID string
	ProviderMessageID string
	ProviderObjectID  string
}

type DispatchSender interface {
	Submit(context.Context, DispatchSubmitRequest) (DispatchSubmitResult, error)
}

type DispatchPollRequest struct {
	Principal                 DispatchPrincipal
	Capability                string
	Dispatch                  Dispatch
	ExpectedCredentialVersion uint64
}

type DispatchPollResult struct {
	Terminal          bool
	Status            string
	ProviderRequestID string
	ProviderMessageID string
	ProviderObjectID  string
	ErrorCode         string
	NextPollAt        *time.Time
	Results           []DispatchResultRequest
}

type DispatchPoller interface {
	Poll(context.Context, DispatchPollRequest) (DispatchPollResult, error)
}

type DispatchProviderErrorCategory string

const (
	DispatchErrorRateLimit    DispatchProviderErrorCategory = "rate_limit"
	DispatchErrorServer       DispatchProviderErrorCategory = "server"
	DispatchErrorTimeout      DispatchProviderErrorCategory = "timeout"
	DispatchErrorUnauthorized DispatchProviderErrorCategory = "unauthorized"
	DispatchErrorForbidden    DispatchProviderErrorCategory = "forbidden"
	DispatchErrorContract     DispatchProviderErrorCategory = "contract"
)

type DispatchProviderError struct {
	Code     string
	Category DispatchProviderErrorCategory
	// SubmissionNotAccepted is only a provider contract assertion that the
	// submit endpoint definitely did not accept the request. Without it, a
	// submit-side 429 is treated as ambiguous and reconciled conservatively.
	SubmissionNotAccepted bool
	// Retryable is retained for callers that classify a provider-specific
	// error, but the default classifier only honors it for unknown codes.
	Retryable bool
	Cause     error
}

func (e *DispatchProviderError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Code
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Cause)
}

func (e *DispatchProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type DispatchRetryDecision struct {
	Retryable bool
	Reconcile bool
	ErrorCode string
}

type DispatchRetryClassifier interface {
	Classify(error) DispatchRetryDecision
}

type defaultDispatchRetryClassifier struct{}

func (defaultDispatchRetryClassifier) Classify(err error) DispatchRetryDecision {
	var providerErr *DispatchProviderError
	if errors.As(err, &providerErr) {
		code := safeDispatchErrorCode(providerErr.Code)
		switch providerErr.Category {
		case DispatchErrorRateLimit, DispatchErrorServer:
			return DispatchRetryDecision{Retryable: true, ErrorCode: code}
		case DispatchErrorTimeout:
			return DispatchRetryDecision{Reconcile: true, ErrorCode: code}
		case DispatchErrorUnauthorized, DispatchErrorForbidden, DispatchErrorContract:
			return DispatchRetryDecision{ErrorCode: code}
		}
		if code == "wecom.http_429" || isDispatchHTTP5xx(code) {
			return DispatchRetryDecision{Retryable: true, ErrorCode: code}
		}
		if code == "wecom.timeout" {
			return DispatchRetryDecision{Reconcile: true, ErrorCode: code}
		}
		if code == "wecom.http_401" || code == "wecom.http_403" || code == "wecom.dispatch_contract_invalid" || code == "wecom.contract_error" {
			return DispatchRetryDecision{ErrorCode: code}
		}
		// Unknown provider categories fail closed. A custom classifier may use
		// Retryable explicitly, but the default must not create an unbounded
		// retry loop from an unclassified external error.
		return DispatchRetryDecision{ErrorCode: code}
	}
	return DispatchRetryDecision{ErrorCode: "wecom.provider_error"}
}

func isDispatchHTTP5xx(code string) bool {
	const prefix = "wecom.http_"
	if !strings.HasPrefix(code, prefix) {
		return false
	}
	status, err := strconv.Atoi(strings.TrimPrefix(code, prefix))
	return err == nil && status >= 500 && status <= 599
}

type DispatchClock interface{ Now() time.Time }

type wallDispatchClock struct{}

func (wallDispatchClock) Now() time.Time { return time.Now().UTC() }

type DispatchBackoff interface{ Duration(int) time.Duration }

type fixedDispatchBackoff time.Duration

func (b fixedDispatchBackoff) Duration(int) time.Duration { return time.Duration(b) }

func FixedBackoff(duration time.Duration) DispatchBackoff { return fixedDispatchBackoff(duration) }

type DispatchRunRequest struct {
	Principal                 DispatchPrincipal
	Capability                string
	ExpectedCredentialVersion uint64
	DispatchID                int64
	LeaseDuration             time.Duration
}

type DispatchRunResult struct {
	Dispatch          Dispatch
	Status            string
	ParentStatus      string
	Submitted         bool
	Polled            bool
	RetryScheduled    bool
	NextAttemptAt     time.Time
	ErrorCode         string
	ProviderRequestID string
	ProviderMessageID string
	ReconcileRequired bool
}

type DispatchRunner struct {
	ledger     DispatchLedger
	authorizer DispatchAuthorizer
	sender     DispatchSender
	poller     DispatchPoller
	clock      DispatchClock
	backoff    DispatchBackoff
	classifier DispatchRetryClassifier
}

func NewDispatchRunner(ledger DispatchLedger, authorizer DispatchAuthorizer, sender DispatchSender, poller DispatchPoller) *DispatchRunner {
	return &DispatchRunner{
		ledger: ledger, authorizer: authorizer, sender: sender, poller: poller,
		clock: wallDispatchClock{}, backoff: fixedDispatchBackoff(time.Minute), classifier: defaultDispatchRetryClassifier{},
	}
}

func (r *DispatchRunner) WithClock(clock DispatchClock) *DispatchRunner {
	if clock != nil {
		r.clock = clock
	}
	return r
}

func (r *DispatchRunner) WithBackoff(backoff DispatchBackoff) *DispatchRunner {
	if backoff != nil {
		r.backoff = backoff
	}
	return r
}

func (r *DispatchRunner) WithRetryClassifier(classifier DispatchRetryClassifier) *DispatchRunner {
	if classifier != nil {
		r.classifier = classifier
	}
	return r
}

func (r *DispatchRunner) Run(ctx context.Context, request DispatchRunRequest) (DispatchRunResult, error) {
	if r == nil || r.ledger == nil || r.authorizer == nil || request.DispatchID <= 0 || request.Principal.TenantID <= 0 || request.Principal.CorpID <= 0 || request.Principal.AuthVersion == 0 || request.ExpectedCredentialVersion == 0 || request.LeaseDuration <= 0 || !isDispatchCapability(request.Capability) {
		return DispatchRunResult{}, ErrDispatchInvalidState
	}
	preclaim := DispatchAuthorizationRequest{
		Principal: request.Principal, Capability: request.Capability, DispatchID: request.DispatchID,
		ExpectedCredentialVersion: request.ExpectedCredentialVersion, Stage: dispatchAuthorizationPreclaim,
	}
	if err := r.authorizer.AuthorizeDispatch(ctx, preclaim); err != nil {
		return DispatchRunResult{}, err
	}
	dispatch, err := r.ledger.ClaimDispatch(ctx, DispatchClaimRequest{
		Principal: request.Principal, DispatchID: request.DispatchID,
		ExpectedCredentialVersion: request.ExpectedCredentialVersion, LeaseDuration: request.LeaseDuration,
	})
	if err != nil {
		return DispatchRunResult{}, err
	}
	result := DispatchRunResult{Dispatch: dispatch, Status: dispatch.Status, ProviderRequestID: dispatch.ProviderRequestID, ProviderMessageID: dispatch.ProviderMessageID}
	if dispatch.CredentialVersion != request.ExpectedCredentialVersion {
		return result, ErrDispatchCredentialStale
	}
	if dispatch.TenantID != request.Principal.TenantID || dispatch.CorpID != request.Principal.CorpID || dispatch.OperationID <= 0 || strings.TrimSpace(dispatch.TargetID) == "" || dispatch.Attempt <= 0 || strings.TrimSpace(dispatch.LeaseToken) == "" {
		return result, ErrDispatchInvalidState
	}
	if !DispatchKindMatchesCapability(request.Capability, DispatchKind(dispatch.DispatchKind)) {
		return result, ErrDispatchInvalidState
	}
	if isTerminalDispatchStatus(dispatch.Status) {
		return result, nil
	}
	if err := r.authorizer.AuthorizeDispatch(ctx, DispatchAuthorizationRequest{
		Principal: request.Principal, Capability: request.Capability, DispatchID: dispatch.ID, OperationID: dispatch.OperationID,
		TargetID: dispatch.TargetID, ExpectedCredentialVersion: request.ExpectedCredentialVersion, Stage: dispatchAuthorizationClaim,
	}); err != nil {
		return result, err
	}

	if dispatch.Status == DispatchSubmitted || dispatch.Status == DispatchPolling || (dispatch.Status == DispatchSubmitting && dispatch.LastErrorCode == DispatchReconcileRequiredCode) {
		return r.poll(ctx, request, dispatch, result)
	}
	if dispatch.Status == DispatchSubmitting {
		result.ReconcileRequired = true
		result.ErrorCode = DispatchReconcileRequiredCode
		return r.persistReconcile(ctx, request, dispatch, result, DispatchSubmitResult{})
	}
	if dispatch.Status != DispatchClaimed {
		return result, ErrDispatchInvalidState
	}
	if r.sender == nil {
		return result, ErrDispatchInvalidState
	}
	if _, err := r.ledger.TransitionDispatch(ctx, DispatchTransitionRequest{
		Principal: request.Principal, DispatchID: dispatch.ID, Status: DispatchSubmitting,
		LeaseToken: dispatch.LeaseToken, Attempt: dispatch.Attempt,
	}); err != nil {
		return result, err
	}
	submit, submitErr := r.sender.Submit(ctx, DispatchSubmitRequest{
		Principal: request.Principal, Capability: request.Capability, Dispatch: dispatch,
		ExpectedCredentialVersion: request.ExpectedCredentialVersion,
	})
	if submitErr != nil {
		return r.handleProviderError(ctx, request, dispatch, result, submitErr, true)
	}
	if submit.Submitted && strings.TrimSpace(submit.ProviderRequestID) == "" && strings.TrimSpace(submit.ProviderMessageID) == "" && strings.TrimSpace(submit.ProviderObjectID) == "" {
		result.ErrorCode = "wecom.dispatch_contract_ambiguous"
		return r.persistReconcile(ctx, request, dispatch, result, DispatchSubmitResult{})
	}
	if !submit.Submitted || strings.TrimSpace(submit.ProviderRequestID) == "" && strings.TrimSpace(submit.ProviderMessageID) == "" && strings.TrimSpace(submit.ProviderObjectID) == "" {
		return r.failContract(ctx, request, dispatch, result)
	}
	updated, transitionErr := r.ledger.TransitionDispatch(ctx, DispatchTransitionRequest{
		Principal: request.Principal, DispatchID: dispatch.ID, Status: DispatchSubmitted,
		LeaseToken: dispatch.LeaseToken, Attempt: dispatch.Attempt,
		ProviderRequestID: submit.ProviderRequestID, ProviderMessageID: submit.ProviderMessageID, ProviderObjectID: submit.ProviderObjectID,
	})
	result.Submitted = true
	result.ProviderRequestID, result.ProviderMessageID = submit.ProviderRequestID, submit.ProviderMessageID
	if transitionErr != nil {
		return r.persistReconcile(ctx, request, dispatch, result, submit)
	}
	result.Dispatch, result.Status = updated, updated.Status
	return r.attachAggregate(ctx, request, result)
}

func (r *DispatchRunner) poll(ctx context.Context, request DispatchRunRequest, dispatch Dispatch, result DispatchRunResult) (DispatchRunResult, error) {
	if r.poller == nil {
		return result, ErrDispatchInvalidState
	}
	result.Polled = true
	poll, err := r.poller.Poll(ctx, DispatchPollRequest{
		Principal: request.Principal, Capability: request.Capability, Dispatch: dispatch,
		ExpectedCredentialVersion: request.ExpectedCredentialVersion,
	})
	if err != nil {
		return r.handleProviderError(ctx, request, dispatch, result, err, false)
	}
	providerRequestID := firstDispatchValue(poll.ProviderRequestID, dispatch.ProviderRequestID)
	providerMessageID := firstDispatchValue(poll.ProviderMessageID, dispatch.ProviderMessageID)
	providerObjectID := firstDispatchValue(poll.ProviderObjectID, dispatch.ProviderObjectID)
	if !poll.Terminal {
		next := poll.NextPollAt
		nextDelay := time.Duration(0)
		if next == nil {
			nextDelay = r.backoff.Duration(dispatch.Attempt)
			nextValue := r.clock.Now().Add(nextDelay)
			next = &nextValue
		}
		status := DispatchPolling
		updated, transitionErr := r.ledger.TransitionDispatch(ctx, DispatchTransitionRequest{
			Principal: request.Principal, DispatchID: dispatch.ID, Status: status, LeaseToken: dispatch.LeaseToken, Attempt: dispatch.Attempt,
			ProviderRequestID: providerRequestID, ProviderMessageID: providerMessageID, ProviderObjectID: providerObjectID, NextPollAt: next, NextPollDelay: nextDelay,
		})
		if transitionErr != nil {
			return r.persistReconcile(ctx, request, dispatch, result, DispatchSubmitResult{ProviderRequestID: providerRequestID, ProviderMessageID: providerMessageID, ProviderObjectID: providerObjectID})
		}
		result.Dispatch, result.Status, result.NextAttemptAt = updated, updated.Status, *next
		result.RetryScheduled = true
		return r.attachAggregate(ctx, request, result)
	}
	if !isTerminalDispatchStatus(poll.Status) || poll.Status == DispatchQueued || poll.Status == DispatchClaimed || poll.Status == DispatchSubmitting || poll.Status == DispatchSubmitted || poll.Status == DispatchPolling {
		return r.failContract(ctx, request, dispatch, result)
	}
	if (poll.Status == DispatchFailed || poll.Status == DispatchPartialFailed) && safeDispatchErrorCode(poll.ErrorCode) == "" {
		return r.failContract(ctx, request, dispatch, result)
	}
	if dispatch.Status == DispatchSubmitted || (dispatch.Status == DispatchSubmitting && dispatch.LastErrorCode == DispatchReconcileRequiredCode) {
		updated, transitionErr := r.ledger.TransitionDispatch(ctx, DispatchTransitionRequest{
			Principal: request.Principal, DispatchID: dispatch.ID, Status: DispatchPolling, LeaseToken: dispatch.LeaseToken, Attempt: dispatch.Attempt,
			ProviderRequestID: providerRequestID, ProviderMessageID: providerMessageID, ProviderObjectID: providerObjectID,
		})
		if transitionErr != nil {
			return r.persistReconcile(ctx, request, dispatch, result, DispatchSubmitResult{ProviderRequestID: providerRequestID, ProviderMessageID: providerMessageID, ProviderObjectID: providerObjectID})
		}
		dispatch = updated
	}
	for _, targetResult := range poll.Results {
		if !IsTerminalOperationResultStatus(targetResult.Status) {
			return r.failContract(ctx, request, dispatch, result)
		}
		targetResult.Principal = request.Principal
		targetResult.DispatchID = dispatch.ID
		targetResult.LeaseToken = dispatch.LeaseToken
		targetResult.Attempt = dispatch.Attempt
		if _, err := r.ledger.RecordDispatchResult(ctx, targetResult); err != nil {
			return r.persistReconcile(ctx, request, dispatch, result, DispatchSubmitResult{ProviderRequestID: providerRequestID, ProviderMessageID: providerMessageID, ProviderObjectID: providerObjectID})
		}
	}
	updated, transitionErr := r.ledger.TransitionDispatch(ctx, DispatchTransitionRequest{
		Principal: request.Principal, DispatchID: dispatch.ID, Status: poll.Status, LeaseToken: dispatch.LeaseToken, Attempt: dispatch.Attempt,
		ProviderRequestID: providerRequestID, ProviderMessageID: providerMessageID, ProviderObjectID: providerObjectID, LastErrorCode: safeDispatchErrorCode(poll.ErrorCode),
	})
	if transitionErr != nil {
		return r.persistReconcile(ctx, request, dispatch, result, DispatchSubmitResult{ProviderRequestID: providerRequestID, ProviderMessageID: providerMessageID, ProviderObjectID: providerObjectID})
	}
	result.Dispatch, result.Status = updated, updated.Status
	return r.attachAggregate(ctx, request, result)
}

func (r *DispatchRunner) handleProviderError(ctx context.Context, request DispatchRunRequest, dispatch Dispatch, result DispatchRunResult, providerErr error, submitPhase bool) (DispatchRunResult, error) {
	decision := r.classifier.Classify(providerErr)
	if decision.ErrorCode == "" {
		decision.ErrorCode = "wecom.provider_error"
	}
	if submitPhase {
		if !dispatchProviderErrorSubmissionNotAccepted(providerErr) && dispatchProviderErrorMayHaveBeenAccepted(providerErr, decision.ErrorCode) {
			decision.Reconcile = true
			decision.Retryable = false
		}
	} else if decision.Reconcile && dispatchProviderErrorMayHaveBeenAccepted(providerErr, decision.ErrorCode) {
		decision.Reconcile = false
		decision.Retryable = true
	}
	if decision.Reconcile {
		result.ErrorCode = decision.ErrorCode
		return r.persistReconcile(ctx, request, dispatch, result, DispatchSubmitResult{})
	}
	next := (*time.Time)(nil)
	nextDelay := time.Duration(0)
	if decision.Retryable {
		nextDelay = r.backoff.Duration(dispatch.Attempt)
		nextValue := r.clock.Now().Add(nextDelay)
		next = &nextValue
	}
	status := DispatchFailed
	if decision.Retryable && (dispatch.Status == DispatchSubmitted || dispatch.Status == DispatchPolling) {
		status = DispatchPolling
	}
	updated, transitionErr := r.ledger.TransitionDispatch(ctx, DispatchTransitionRequest{
		Principal: request.Principal, DispatchID: dispatch.ID, Status: status, LeaseToken: dispatch.LeaseToken, Attempt: dispatch.Attempt,
		NextPollAt: next, NextPollDelay: nextDelay, LastErrorCode: decision.ErrorCode,
	})
	result.ErrorCode = decision.ErrorCode
	result.Dispatch, result.Status = updated, updated.Status
	result.RetryScheduled = decision.Retryable
	if next != nil {
		result.NextAttemptAt = *next
	}
	if transitionErr != nil {
		result.ReconcileRequired = true
		return result, ErrDispatchReconcileRequired
	}
	resultWithAggregate, aggregateErr := r.attachAggregate(ctx, request, result)
	if aggregateErr != nil {
		resultWithAggregate.ReconcileRequired = true
		return resultWithAggregate, ErrDispatchReconcileRequired
	}
	return resultWithAggregate, providerErr
}

func (r *DispatchRunner) persistReconcile(ctx context.Context, request DispatchRunRequest, dispatch Dispatch, result DispatchRunResult, provider DispatchSubmitResult) (DispatchRunResult, error) {
	result.ReconcileRequired = true
	result.ProviderRequestID = firstDispatchValue(provider.ProviderRequestID, result.ProviderRequestID)
	result.ProviderMessageID = firstDispatchValue(provider.ProviderMessageID, result.ProviderMessageID)
	errorCode := result.ErrorCode
	if errorCode == "" {
		errorCode = "wecom.dispatch_transition_failed"
	}
	updated, err := r.ledger.PersistDispatchReconcile(ctx, DispatchReconcileRequest{
		Principal: request.Principal, DispatchID: dispatch.ID, LeaseToken: dispatch.LeaseToken, Attempt: dispatch.Attempt,
		ProviderRequestID: provider.ProviderRequestID, ProviderMessageID: provider.ProviderMessageID, ProviderObjectID: provider.ProviderObjectID,
		ErrorCode:  errorCode,
		NextPollAt: nil,
	})
	if err != nil {
		return result, ErrDispatchReconcileRequired
	}
	result.Dispatch, result.Status = updated, updated.Status
	return result, ErrDispatchReconcileRequired
}

func dispatchProviderErrorSubmissionNotAccepted(err error) bool {
	var providerErr *DispatchProviderError
	return errors.As(err, &providerErr) && providerErr.SubmissionNotAccepted
}

func dispatchProviderErrorMayHaveBeenAccepted(err error, code string) bool {
	if code == "wecom.http_429" || code == "wecom.timeout" || isDispatchHTTP5xx(code) {
		return true
	}
	var providerErr *DispatchProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Category == DispatchErrorRateLimit || providerErr.Category == DispatchErrorServer || providerErr.Category == DispatchErrorTimeout
	}
	return false
}

func (r *DispatchRunner) failContract(ctx context.Context, request DispatchRunRequest, dispatch Dispatch, result DispatchRunResult) (DispatchRunResult, error) {
	updated, err := r.ledger.TransitionDispatch(ctx, DispatchTransitionRequest{
		Principal: request.Principal, DispatchID: dispatch.ID, Status: DispatchFailed, LeaseToken: dispatch.LeaseToken, Attempt: dispatch.Attempt,
		LastErrorCode: "wecom.dispatch_contract_invalid",
	})
	result.ErrorCode = "wecom.dispatch_contract_invalid"
	result.Dispatch, result.Status = updated, updated.Status
	if err != nil {
		result.ReconcileRequired = true
		return result, ErrDispatchReconcileRequired
	}
	resultWithAggregate, aggregateErr := r.attachAggregate(ctx, request, result)
	if aggregateErr != nil {
		resultWithAggregate.ReconcileRequired = true
		return resultWithAggregate, ErrDispatchReconcileRequired
	}
	return resultWithAggregate, ErrDispatchContract
}

func (r *DispatchRunner) attachAggregate(ctx context.Context, request DispatchRunRequest, result DispatchRunResult) (DispatchRunResult, error) {
	aggregate, err := r.ledger.AggregateOperation(ctx, OperationAggregateRequest{
		Principal: request.Principal, OperationID: result.Dispatch.OperationID, Capability: request.Capability,
		ExpectedCredentialVersion: request.ExpectedCredentialVersion,
	})
	if err != nil {
		return result, err
	}
	result.ParentStatus = aggregate.Status
	return result, nil
}

func isDispatchCapability(capability string) bool {
	return capability == ContactBatchSend || capability == RoomBatchSend
}

func DispatchKindMatchesCapability(capability string, dispatchKind DispatchKind) bool {
	switch capability {
	case ContactBatchSend:
		return DispatchKind(strings.TrimSpace(string(dispatchKind))) == DispatchKindContactBatch
	case RoomBatchSend:
		return DispatchKind(strings.TrimSpace(string(dispatchKind))) == DispatchKindRoomBatch
	default:
		return false
	}
}

func isTerminalDispatchStatus(status string) bool {
	switch status {
	case DispatchSucceeded, DispatchPartialFailed, DispatchFailed, DispatchCancelled:
		return true
	default:
		return false
	}
}

func safeDispatchErrorCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	for _, r := range code {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '.' && r != '_' && r != '-' {
			return "wecom.provider_error"
		}
	}
	if len(code) > 96 {
		return "wecom.provider_error"
	}
	return code
}

func firstDispatchValue(candidate, existing string) string {
	if strings.TrimSpace(candidate) != "" {
		return strings.TrimSpace(candidate)
	}
	return strings.TrimSpace(existing)
}

func AggregateOperationStatus(capability string, dispatches []Dispatch) string {
	return AggregateOperationStatusWithResults(capability, dispatches, nil)
}

// AggregateOperationStatusWithResults is a read-side projection. Dispatch
// rows are the chunk-level authority; result rows refine a successful
// projection when the provider returned per-target failures. It never writes
// the parent operation row.
func AggregateOperationStatusWithResults(capability string, dispatches []Dispatch, results []OperationResult) string {
	if !isDispatchCapability(capability) {
		return OperationFailed
	}
	if len(dispatches) == 0 {
		return OperationPending
	}
	allSucceeded, allCancelled := true, true
	hasSuccess, hasFailure, hasPartial, hasActive, hasQueued := false, false, false, false, false
	for _, dispatch := range dispatches {
		switch dispatch.Status {
		case DispatchSucceeded:
			hasSuccess, allCancelled = true, false
		case DispatchPartialFailed:
			hasPartial, allSucceeded, allCancelled = true, false, false
		case DispatchFailed:
			hasFailure, allSucceeded, allCancelled = true, false, false
		case DispatchCancelled:
			allSucceeded = false
		case DispatchQueued:
			hasQueued, allSucceeded, allCancelled = true, false, false
		case DispatchClaimed, DispatchSubmitting, DispatchSubmitted, DispatchPolling:
			hasActive, allSucceeded, allCancelled = true, false, false
		default:
			return OperationFailed
		}
	}
	if allCancelled {
		return OperationCancelled
	}
	if allSucceeded {
		resultStatus := aggregateResultStatus(results)
		switch resultStatus {
		case OperationPending, OperationFailed, OperationPartialFailed:
			return resultStatus
		default:
			return OperationSucceeded
		}
	}
	if hasActive {
		for _, dispatch := range dispatches {
			if dispatch.Status == DispatchSubmitted || dispatch.Status == DispatchPolling {
				return OperationPolling
			}
		}
		return OperationSubmitting
	}
	if hasQueued {
		return OperationPending
	}
	if hasPartial || (hasSuccess && hasFailure) || (hasSuccess && allCancelled) {
		return OperationPartialFailed
	}
	if hasFailure {
		return OperationFailed
	}
	return OperationPartialFailed
}

func aggregateResultStatus(results []OperationResult) string {
	if len(results) == 0 {
		return OperationSucceeded
	}
	hasQueued, hasSuccess, hasFailure, hasPartial := false, false, false, false
	for _, result := range results {
		switch result.Status {
		case DispatchQueued:
			hasQueued = true
		case DispatchSucceeded:
			hasSuccess = true
		case DispatchPartialFailed:
			hasPartial = true
		case DispatchFailed:
			hasFailure = true
		default:
			return OperationFailed
		}
	}
	if hasQueued {
		return OperationPending
	}
	if hasPartial || (hasSuccess && hasFailure) {
		return OperationPartialFailed
	}
	if hasFailure {
		return OperationFailed
	}
	return OperationSucceeded
}
