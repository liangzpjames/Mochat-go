package wecomcapability

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type dispatchRunnerFakeLedger struct {
	mu                 sync.Mutex
	claimed            Dispatch
	claimCount         int
	transitioned       []DispatchTransitionRequest
	transitionErr      error
	transitionErrAfter int
	aggregate          OperationAggregate
	aggregateError     error
}

func (f *dispatchRunnerFakeLedger) ClaimDispatch(_ context.Context, _ DispatchClaimRequest) (Dispatch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimCount++
	if f.claimCount > 1 {
		return Dispatch{}, ErrDispatchAlreadyClaimed
	}
	return f.claimed, nil
}

func (f *dispatchRunnerFakeLedger) TransitionDispatch(_ context.Context, request DispatchTransitionRequest) (Dispatch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transitioned = append(f.transitioned, request)
	if f.transitionErr != nil && (f.transitionErrAfter == 0 || len(f.transitioned) >= f.transitionErrAfter) {
		return Dispatch{}, f.transitionErr
	}
	if !CanTransitionDispatch(f.claimed.Status, request.Status) {
		return Dispatch{}, ErrDispatchInvalidState
	}
	f.claimed.Status = request.Status
	if request.ProviderRequestID != "" {
		f.claimed.ProviderRequestID = request.ProviderRequestID
	}
	if request.ProviderMessageID != "" {
		f.claimed.ProviderMessageID = request.ProviderMessageID
	}
	if request.ProviderObjectID != "" {
		f.claimed.ProviderObjectID = request.ProviderObjectID
	}
	return f.claimed, nil
}

func (f *dispatchRunnerFakeLedger) RecordDispatchResult(context.Context, DispatchResultRequest) (OperationResult, error) {
	return OperationResult{}, nil
}

func (f *dispatchRunnerFakeLedger) AggregateOperation(context.Context, OperationAggregateRequest) (OperationAggregate, error) {
	return f.aggregate, f.aggregateError
}

type dispatchRunnerFakeAuthorizer struct {
	mu    sync.Mutex
	calls []DispatchAuthorizationRequest
	err   error
}

func (f *dispatchRunnerFakeAuthorizer) AuthorizeDispatch(_ context.Context, request DispatchAuthorizationRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, request)
	return f.err
}

type dispatchRunnerFakeSender struct {
	calls  int
	result DispatchSubmitResult
	err    error
}

func (f *dispatchRunnerFakeSender) Submit(context.Context, DispatchSubmitRequest) (DispatchSubmitResult, error) {
	f.calls++
	return f.result, f.err
}

type dispatchRunnerFakePoller struct {
	calls  int
	result DispatchPollResult
	err    error
}

func (f *dispatchRunnerFakePoller) Poll(context.Context, DispatchPollRequest) (DispatchPollResult, error) {
	f.calls++
	return f.result, f.err
}

type dispatchRunnerFixedClock struct{ now time.Time }

func (f dispatchRunnerFixedClock) Now() time.Time { return f.now }

func TestDispatchRunnerDoesNotSubmitAlreadySubmittedDispatch(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 11, TenantID: 7, CorpID: 9, OperationID: 21, DispatchKind: "contact", TargetID: "external-1",
		Status: DispatchSubmitted, CredentialVersion: 4, Attempt: 2, LeaseToken: "lease-2", ProviderRequestID: "task-1",
	}, aggregate: OperationAggregate{Status: OperationSucceeded}}
	sender := &dispatchRunnerFakeSender{result: DispatchSubmitResult{ProviderRequestID: "must-not-submit"}}
	poller := &dispatchRunnerFakePoller{result: DispatchPollResult{Terminal: true, Status: DispatchSucceeded, ProviderRequestID: "task-1"}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, poller)

	result, err := runner.Run(context.Background(), DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: ContactBatchSend, ExpectedCredentialVersion: 4, DispatchID: 11, LeaseDuration: time.Minute,
	})
	if err != nil {
		t.Fatalf("run submitted dispatch: %v", err)
	}
	if sender.calls != 0 || poller.calls != 1 || !result.Polled || result.Submitted || result.ParentStatus != OperationSucceeded {
		t.Fatalf("submitted dispatch was submitted again: sender=%d poller=%d result=%+v", sender.calls, poller.calls, result)
	}
}

func TestDispatchRunnerFailsClosedBeforeSubmitOnGenerationOrAuthorization(t *testing.T) {
	for _, test := range []struct {
		name       string
		authorizer *dispatchRunnerFakeAuthorizer
		version    uint64
	}{
		{name: "generation mismatch", authorizer: &dispatchRunnerFakeAuthorizer{}, version: 3},
		{name: "authorization denied", authorizer: &dispatchRunnerFakeAuthorizer{err: ErrDispatchAuthorizationDenied}, version: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
				ID: 12, TenantID: 7, CorpID: 9, OperationID: 22, DispatchKind: "room", TargetID: "room-1",
				Status: DispatchQueued, CredentialVersion: 4, Attempt: 1, LeaseToken: "lease-1",
			}}
			sender := &dispatchRunnerFakeSender{}
			runner := NewDispatchRunner(ledger, test.authorizer, sender, nil)
			_, err := runner.Run(context.Background(), DispatchRunRequest{
				Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
				Capability: RoomBatchSend, ExpectedCredentialVersion: test.version, DispatchID: 12, LeaseDuration: time.Minute,
			})
			if err == nil || sender.calls != 0 {
				t.Fatalf("runner did not fail closed before submit: err=%v sender=%d", err, sender.calls)
			}
		})
	}
}

func TestDispatchRunnerExternalSuccessWithStoreFailureRequiresReconcile(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{
		claimed:            Dispatch{ID: 13, TenantID: 7, CorpID: 9, OperationID: 23, DispatchKind: "contact", TargetID: "external-2", Status: DispatchClaimed, CredentialVersion: 4, Attempt: 1, LeaseToken: "lease-1"},
		transitionErr:      errors.New("db unavailable after external submit"),
		transitionErrAfter: 2,
	}
	sender := &dispatchRunnerFakeSender{result: DispatchSubmitResult{Submitted: true, ProviderRequestID: "task-after-db-failure", ProviderMessageID: "msg-after-db-failure"}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, nil)

	result, err := runner.Run(context.Background(), DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: ContactBatchSend, ExpectedCredentialVersion: 4, DispatchID: 13, LeaseDuration: time.Minute,
	})
	if !errors.Is(err, ErrDispatchReconcileRequired) || !result.ReconcileRequired || result.ProviderRequestID != "task-after-db-failure" {
		t.Fatalf("external success was not surfaced for reconciliation: result=%+v err=%v", result, err)
	}
}

func TestDispatchRunnerRetryClassifierSchedulesRetryWithoutFakeSuccess(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 14, TenantID: 7, CorpID: 9, OperationID: 24, DispatchKind: "room", TargetID: "room-2", Status: DispatchClaimed, CredentialVersion: 4, Attempt: 1, LeaseToken: "lease-1",
	}}
	sender := &dispatchRunnerFakeSender{err: &DispatchProviderError{Code: "wecom.http_429", Retryable: true}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, nil)
	runner = runner.WithClock(dispatchRunnerFixedClock{now: time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC)}).WithBackoff(FixedBackoff(time.Minute))

	result, err := runner.Run(context.Background(), DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: RoomBatchSend, ExpectedCredentialVersion: 4, DispatchID: 14, LeaseDuration: time.Minute,
	})
	if err == nil || result.Submitted || !result.RetryScheduled || result.NextAttemptAt.IsZero() || result.ErrorCode != "wecom.http_429" {
		t.Fatalf("retryable error became success or lost retry schedule: result=%+v err=%v", result, err)
	}
}

func TestDefaultDispatchRetryClassifierUsesStableProviderCategories(t *testing.T) {
	classifier := defaultDispatchRetryClassifier{}
	for _, test := range []struct {
		code      string
		retryable bool
	}{
		{code: "wecom.http_429", retryable: true},
		{code: "wecom.http_503", retryable: true},
		{code: "wecom.timeout", retryable: true},
		{code: "wecom.http_401", retryable: false},
		{code: "wecom.http_403", retryable: false},
		{code: "wecom.dispatch_contract_invalid", retryable: false},
		{code: "wecom.unclassified", retryable: false},
	} {
		decision := classifier.Classify(&DispatchProviderError{Code: test.code, Retryable: !test.retryable})
		if decision.Retryable != test.retryable || decision.ErrorCode != test.code {
			t.Fatalf("classify(%q)=%+v, want retryable=%v and stable code", test.code, decision, test.retryable)
		}
	}
}

func TestDispatchRunnerPollRetryKeepsSubmittedWorkInPollingState(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 15, TenantID: 7, CorpID: 9, OperationID: 25, DispatchKind: "contact", TargetID: "external-3", Status: DispatchPolling, CredentialVersion: 4, Attempt: 3, LeaseToken: "lease-3",
	}}
	sender := &dispatchRunnerFakeSender{}
	poller := &dispatchRunnerFakePoller{err: &DispatchProviderError{Code: "wecom.http_503", Retryable: true}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, poller).
		WithClock(dispatchRunnerFixedClock{now: time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC)}).
		WithBackoff(FixedBackoff(time.Minute))

	result, err := runner.Run(context.Background(), DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: ContactBatchSend, ExpectedCredentialVersion: 4, DispatchID: 15, LeaseDuration: time.Minute,
	})
	if err == nil || sender.calls != 0 || !result.RetryScheduled || result.Status != DispatchPolling || result.ErrorCode != "wecom.http_503" {
		t.Fatalf("poll retry was allowed to become resubmittable: sender=%d result=%+v err=%v", sender.calls, result, err)
	}
}

func TestDispatchRunnerPollDoesNotErasePersistedProviderIdentity(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 151, TenantID: 7, CorpID: 9, OperationID: 251, DispatchKind: "contact", TargetID: "external-identity", Status: DispatchSubmitted, CredentialVersion: 4, Attempt: 3, LeaseToken: "lease-3", ProviderRequestID: "task-existing", ProviderMessageID: "msg-existing",
	}}
	poller := &dispatchRunnerFakePoller{result: DispatchPollResult{Terminal: true, Status: DispatchSucceeded}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, &dispatchRunnerFakeSender{}, poller)

	result, err := runner.Run(context.Background(), DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: ContactBatchSend, ExpectedCredentialVersion: 4, DispatchID: 151, LeaseDuration: time.Minute,
	})
	if err != nil || result.Dispatch.ProviderRequestID != "task-existing" || result.Dispatch.ProviderMessageID != "msg-existing" {
		t.Fatalf("poll erased provider identity: result=%+v err=%v", result, err)
	}
}

func TestDispatchRunnerDuplicateWorkerCannotSubmitSameClaimTwice(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 16, TenantID: 7, CorpID: 9, OperationID: 26, DispatchKind: "room", TargetID: "room-4", Status: DispatchClaimed, CredentialVersion: 4, Attempt: 1, LeaseToken: "lease-1",
	}}
	sender := &dispatchRunnerFakeSender{result: DispatchSubmitResult{Submitted: true, ProviderRequestID: "task-16"}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, nil)
	request := DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: RoomBatchSend, ExpectedCredentialVersion: 4, DispatchID: 16, LeaseDuration: time.Minute,
	}
	if _, err := runner.Run(context.Background(), request); err != nil {
		t.Fatalf("first worker failed: %v", err)
	}
	if _, err := runner.Run(context.Background(), request); !errors.Is(err, ErrDispatchAlreadyClaimed) {
		t.Fatalf("duplicate worker err=%v, want already claimed", err)
	}
	if sender.calls != 1 {
		t.Fatalf("duplicate worker submitted %d times", sender.calls)
	}
}

func TestDispatchRunnerRejectsEmptyOrCrossCapabilityDispatchBeforeSubmit(t *testing.T) {
	for _, test := range []struct {
		name         string
		capability   string
		dispatchKind string
		targetID     string
	}{
		{name: "empty target", capability: ContactBatchSend, dispatchKind: "contact", targetID: ""},
		{name: "cross capability kind", capability: ContactBatchSend, dispatchKind: "room", targetID: "room-5"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
				ID: 17, TenantID: 7, CorpID: 9, OperationID: 27, DispatchKind: test.dispatchKind, TargetID: test.targetID, Status: DispatchClaimed, CredentialVersion: 4, Attempt: 1, LeaseToken: "lease-1",
			}}
			sender := &dispatchRunnerFakeSender{result: DispatchSubmitResult{Submitted: true, ProviderRequestID: "must-not-submit"}}
			runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, nil)
			_, err := runner.Run(context.Background(), DispatchRunRequest{
				Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
				Capability: test.capability, ExpectedCredentialVersion: 4, DispatchID: 17, LeaseDuration: time.Minute,
			})
			if err == nil || sender.calls != 0 {
				t.Fatalf("invalid dispatch was sent: err=%v calls=%d", err, sender.calls)
			}
		})
	}
}

func TestAggregateOperationStatusUsesDispatchesAndSeparatesCapability(t *testing.T) {
	if got := AggregateOperationStatus(ContactBatchSend, []Dispatch{
		{Status: DispatchSucceeded}, {Status: DispatchPartialFailed},
	}); got != OperationPartialFailed {
		t.Fatalf("contact aggregate=%q, want partial_failed", got)
	}
	if got := AggregateOperationStatus(RoomBatchSend, []Dispatch{{Status: DispatchSucceeded}}); got != OperationSucceeded {
		t.Fatalf("room aggregate=%q, want succeeded", got)
	}
	if got := AggregateOperationStatus(ContactBatchSend, nil); got != OperationPending {
		t.Fatalf("empty aggregate=%q, want pending", got)
	}
	if got := AggregateOperationStatus("employee_sync", []Dispatch{{Status: DispatchSucceeded}}); got != OperationFailed {
		t.Fatalf("cross-capability aggregate=%q, want fail closed", got)
	}
	if got := AggregateOperationStatusWithResults(ContactBatchSend, []Dispatch{{Status: DispatchSucceeded}}, []OperationResult{{Status: DispatchFailed}}); got != OperationFailed {
		t.Fatalf("failed target result was ignored: aggregate=%q", got)
	}
	if got := AggregateOperationStatusWithResults(ContactBatchSend, []Dispatch{{Status: DispatchSucceeded}}, []OperationResult{{Status: DispatchSucceeded}, {Status: DispatchFailed}}); got != OperationPartialFailed {
		t.Fatalf("mixed target results were not partial: aggregate=%q", got)
	}
}
