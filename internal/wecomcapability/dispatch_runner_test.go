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
	reconcileCalls     int
	reconcileErr       error
	recordedResults    []DispatchResultRequest
	allowReclaim       bool
	aggregate          OperationAggregate
	aggregateError     error
}

func (f *dispatchRunnerFakeLedger) ClaimDispatch(_ context.Context, _ DispatchClaimRequest) (Dispatch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimCount++
	if f.claimCount > 1 && !f.allowReclaim {
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

func (f *dispatchRunnerFakeLedger) PersistDispatchReconcile(_ context.Context, request DispatchReconcileRequest) (Dispatch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reconcileCalls++
	if f.reconcileErr != nil {
		return Dispatch{}, f.reconcileErr
	}
	if request.ProviderRequestID != "" {
		f.claimed.ProviderRequestID = request.ProviderRequestID
	}
	if request.ProviderMessageID != "" {
		f.claimed.ProviderMessageID = request.ProviderMessageID
	}
	if request.ProviderObjectID != "" {
		f.claimed.ProviderObjectID = request.ProviderObjectID
	}
	f.claimed.LastErrorCode = DispatchReconcileRequiredCode
	f.claimed.NextPollAt = request.NextPollAt
	if f.claimed.Status == DispatchSubmitting && (f.claimed.ProviderRequestID != "" || f.claimed.ProviderMessageID != "" || f.claimed.ProviderObjectID != "") {
		f.claimed.Status = DispatchSubmitted
	}
	return f.claimed, nil
}

func (f *dispatchRunnerFakeLedger) RecordDispatchResult(_ context.Context, request DispatchResultRequest) (OperationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordedResults = append(f.recordedResults, request)
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
		ID: 11, TenantID: 7, CorpID: 9, OperationID: 21, DispatchKind: string(DispatchKindContactBatch), TargetID: "external-1",
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

func TestDispatchRunnerPollsPersistedSubmittingReconcileWithoutResubmit(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 111, TenantID: 7, CorpID: 9, OperationID: 211,
		DispatchKind: string(DispatchKindContactBatch), TargetID: "external-reconcile",
		Status: DispatchSubmitting, CredentialVersion: 4, Attempt: 2, LeaseToken: "lease-reconcile",
		ProviderRequestID: "task-reconcile", LastErrorCode: "wecom.dispatch_reconcile_required",
	}, aggregate: OperationAggregate{Status: OperationPolling}}
	sender := &dispatchRunnerFakeSender{result: DispatchSubmitResult{Submitted: true, ProviderRequestID: "must-not-submit"}}
	poller := &dispatchRunnerFakePoller{result: DispatchPollResult{Terminal: true, Status: DispatchSucceeded, ProviderRequestID: "task-reconcile"}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, poller)

	result, err := runner.Run(context.Background(), DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: ContactBatchSend, ExpectedCredentialVersion: 4, DispatchID: 111, LeaseDuration: time.Minute,
	})
	if err != nil || sender.calls != 0 || poller.calls != 1 || !result.Polled {
		t.Fatalf("persisted submitting reconcile was resubmitted or not polled: sender=%d poller=%d result=%+v err=%v", sender.calls, poller.calls, result, err)
	}
}

func TestDispatchRunnerRepairsSubmittingCrashMarkerBeforePolling(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{allowReclaim: true, claimed: Dispatch{
		ID: 112, TenantID: 7, CorpID: 9, OperationID: 212,
		DispatchKind: string(DispatchKindContactBatch), TargetID: "external-crash-window",
		Status: DispatchSubmitting, CredentialVersion: 4, Attempt: 2, LeaseToken: "lease-crash-window",
	}}
	sender := &dispatchRunnerFakeSender{result: DispatchSubmitResult{Submitted: true, ProviderRequestID: "must-not-submit"}}
	poller := &dispatchRunnerFakePoller{result: DispatchPollResult{Terminal: true, Status: DispatchSucceeded}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, poller)
	request := DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: ContactBatchSend, ExpectedCredentialVersion: 4, DispatchID: 112, LeaseDuration: time.Minute,
	}

	first, firstErr := runner.Run(context.Background(), request)
	if !errors.Is(firstErr, ErrDispatchReconcileRequired) || !first.ReconcileRequired || sender.calls != 0 || poller.calls != 0 || ledger.reconcileCalls != 1 || ledger.claimed.LastErrorCode != DispatchReconcileRequiredCode {
		t.Fatalf("submitting crash window was not durably fenced for reconciliation: sender=%d poller=%d reconcile=%d dispatch=%+v result=%+v err=%v", sender.calls, poller.calls, ledger.reconcileCalls, ledger.claimed, first, firstErr)
	}
	second, secondErr := runner.Run(context.Background(), request)
	if secondErr != nil || sender.calls != 0 || poller.calls != 1 || !second.Polled {
		t.Fatalf("submitting crash window did not recover through poll-only path: sender=%d poller=%d result=%+v err=%v", sender.calls, poller.calls, second, secondErr)
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
				ID: 12, TenantID: 7, CorpID: 9, OperationID: 22, DispatchKind: string(DispatchKindRoomBatch), TargetID: "room-1",
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
		claimed:            Dispatch{ID: 13, TenantID: 7, CorpID: 9, OperationID: 23, DispatchKind: string(DispatchKindContactBatch), TargetID: "external-2", Status: DispatchClaimed, CredentialVersion: 4, Attempt: 1, LeaseToken: "lease-1"},
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
	if ledger.reconcileCalls != 1 || ledger.claimed.Status != DispatchSubmitted || ledger.claimed.ProviderRequestID != "task-after-db-failure" {
		t.Fatalf("external success was not durably reconciled: calls=%d dispatch=%+v", ledger.reconcileCalls, ledger.claimed)
	}
}

func TestDispatchRunnerAmbiguousSubmitEntersReconcileWithoutFakeSuccess(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 131, TenantID: 7, CorpID: 9, OperationID: 231, DispatchKind: string(DispatchKindContactBatch),
		TargetID: "external-ambiguous", Status: DispatchClaimed, CredentialVersion: 4, Attempt: 1, LeaseToken: "lease-1",
	}}
	sender := &dispatchRunnerFakeSender{result: DispatchSubmitResult{Submitted: true}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, nil)

	result, err := runner.Run(context.Background(), DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: ContactBatchSend, ExpectedCredentialVersion: 4, DispatchID: 131, LeaseDuration: time.Minute,
	})
	if !errors.Is(err, ErrDispatchReconcileRequired) || !result.ReconcileRequired || sender.calls != 1 || ledger.reconcileCalls != 1 || ledger.claimed.Status != DispatchSubmitting {
		t.Fatalf("ambiguous submit was treated as a normal failure/success: sender=%d reconcile=%d dispatch=%+v result=%+v err=%v", sender.calls, ledger.reconcileCalls, ledger.claimed, result, err)
	}
}

func TestDispatchRunnerSubmitServerErrorReconcilesAndNextRunDoesNotResubmit(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{allowReclaim: true, claimed: Dispatch{
		ID: 132, TenantID: 7, CorpID: 9, OperationID: 232, DispatchKind: string(DispatchKindContactBatch),
		TargetID: "external-503", Status: DispatchClaimed, CredentialVersion: 4, Attempt: 1, LeaseToken: "lease-1",
	}}
	sender := &dispatchRunnerFakeSender{err: &DispatchProviderError{Code: "wecom.http_503", Retryable: true}}
	poller := &dispatchRunnerFakePoller{result: DispatchPollResult{Terminal: true, Status: DispatchSucceeded, ProviderRequestID: "task-503"}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, poller)
	request := DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: ContactBatchSend, ExpectedCredentialVersion: 4, DispatchID: 132, LeaseDuration: time.Minute,
	}

	first, firstErr := runner.Run(context.Background(), request)
	if !errors.Is(firstErr, ErrDispatchReconcileRequired) || !first.ReconcileRequired || ledger.reconcileCalls != 1 {
		t.Fatalf("submit 5xx was not durably reconciled: result=%+v err=%v reconcile=%d", first, firstErr, ledger.reconcileCalls)
	}
	second, secondErr := runner.Run(context.Background(), request)
	if secondErr != nil || sender.calls != 1 || poller.calls != 1 || !second.Polled {
		t.Fatalf("recovered submit 5xx was resubmitted: sender=%d poller=%d result=%+v err=%v", sender.calls, poller.calls, second, secondErr)
	}
}

func TestDispatchRunnerPollServerErrorRetriesPollWithoutSubmit(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{allowReclaim: true, claimed: Dispatch{
		ID: 133, TenantID: 7, CorpID: 9, OperationID: 233, DispatchKind: string(DispatchKindContactBatch),
		TargetID: "external-poll-503", Status: DispatchPolling, CredentialVersion: 4, Attempt: 2, LeaseToken: "lease-2", ProviderRequestID: "task-poll-503",
	}}
	sender := &dispatchRunnerFakeSender{}
	poller := &dispatchRunnerFakePoller{err: &DispatchProviderError{Code: "wecom.http_503", Retryable: true}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, sender, poller)
	request := DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: ContactBatchSend, ExpectedCredentialVersion: 4, DispatchID: 133, LeaseDuration: time.Minute,
	}

	if _, err := runner.Run(context.Background(), request); err == nil {
		t.Fatal("poll 5xx unexpectedly became success")
	}
	if len(ledger.transitioned) != 1 || ledger.transitioned[0].NextPollDelay != time.Minute || ledger.transitioned[0].NextPollAt == nil {
		t.Fatalf("poll 5xx did not carry a database-relative backoff: transitions=%+v", ledger.transitioned)
	}
	poller.err = nil
	poller.result = DispatchPollResult{Terminal: true, Status: DispatchSucceeded, ProviderRequestID: "task-poll-503"}
	if result, err := runner.Run(context.Background(), request); err != nil || sender.calls != 0 || poller.calls != 2 || !result.Polled {
		t.Fatalf("poll 5xx was not retried as poll-only: sender=%d poller=%d result=%+v err=%v", sender.calls, poller.calls, result, err)
	}
}

func TestDispatchRunnerRetryClassifierSchedulesRetryWithoutFakeSuccess(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 14, TenantID: 7, CorpID: 9, OperationID: 24, DispatchKind: string(DispatchKindRoomBatch), TargetID: "room-2", Status: DispatchClaimed, CredentialVersion: 4, Attempt: 1, LeaseToken: "lease-1",
	}}
	sender := &dispatchRunnerFakeSender{err: &DispatchProviderError{Code: "wecom.http_429", Retryable: true, SubmissionNotAccepted: true}}
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
		reconcile bool
	}{
		{code: "wecom.http_429", retryable: true},
		{code: "wecom.http_503", retryable: true},
		{code: "wecom.timeout", reconcile: true},
		{code: "wecom.http_401"},
		{code: "wecom.http_403"},
		{code: "wecom.dispatch_contract_invalid"},
		{code: "wecom.unclassified"},
	} {
		decision := classifier.Classify(&DispatchProviderError{Code: test.code, Retryable: !test.retryable})
		if decision.Retryable != test.retryable || decision.Reconcile != test.reconcile || decision.ErrorCode != test.code {
			t.Fatalf("classify(%q)=%+v, want retryable=%v reconcile=%v and stable code", test.code, decision, test.retryable, test.reconcile)
		}
	}
}

func TestDispatchRunnerPollRetryKeepsSubmittedWorkInPollingState(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 15, TenantID: 7, CorpID: 9, OperationID: 25, DispatchKind: string(DispatchKindContactBatch), TargetID: "external-3", Status: DispatchPolling, CredentialVersion: 4, Attempt: 3, LeaseToken: "lease-3",
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
		ID: 151, TenantID: 7, CorpID: 9, OperationID: 251, DispatchKind: string(DispatchKindContactBatch), TargetID: "external-identity", Status: DispatchSubmitted, CredentialVersion: 4, Attempt: 3, LeaseToken: "lease-3", ProviderRequestID: "task-existing", ProviderMessageID: "msg-existing",
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

func TestDispatchRunnerRejectsNonTerminalTargetResultFromPoll(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 152, TenantID: 7, CorpID: 9, OperationID: 252, DispatchKind: string(DispatchKindContactBatch), TargetID: "external-queued-result",
		Status: DispatchSubmitted, CredentialVersion: 4, Attempt: 3, LeaseToken: "lease-3", ProviderRequestID: "task-queued-result",
	}}
	poller := &dispatchRunnerFakePoller{result: DispatchPollResult{
		Terminal: true, Status: DispatchSucceeded, ProviderRequestID: "task-queued-result",
		Results: []DispatchResultRequest{{TargetKind: "external_user", TargetID: "user-queued", Status: DispatchQueued}},
	}}
	runner := NewDispatchRunner(ledger, &dispatchRunnerFakeAuthorizer{}, &dispatchRunnerFakeSender{}, poller)

	result, err := runner.Run(context.Background(), DispatchRunRequest{
		Principal:  DispatchPrincipal{TenantID: 7, CorpID: 9, UserID: 100, AuthVersion: 1},
		Capability: ContactBatchSend, ExpectedCredentialVersion: 4, DispatchID: 152, LeaseDuration: time.Minute,
	})
	if !errors.Is(err, ErrDispatchContract) || len(ledger.recordedResults) != 0 || result.Status == DispatchSucceeded {
		t.Fatalf("queued target result was accepted as terminal: recorded=%d result=%+v err=%v", len(ledger.recordedResults), result, err)
	}
}

func TestDispatchRunnerDuplicateWorkerCannotSubmitSameClaimTwice(t *testing.T) {
	ledger := &dispatchRunnerFakeLedger{claimed: Dispatch{
		ID: 16, TenantID: 7, CorpID: 9, OperationID: 26, DispatchKind: string(DispatchKindRoomBatch), TargetID: "room-4", Status: DispatchClaimed, CredentialVersion: 4, Attempt: 1, LeaseToken: "lease-1",
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

func TestDispatchKindMatchesCapabilityUsesExactBatchKinds(t *testing.T) {
	if !DispatchKindMatchesCapability(ContactBatchSend, DispatchKindContactBatch) {
		t.Fatal("contact batch kind was rejected")
	}
	if !DispatchKindMatchesCapability(RoomBatchSend, DispatchKindRoomBatch) {
		t.Fatal("room batch kind was rejected")
	}
	for _, invalid := range []struct {
		capability string
		kind       string
	}{
		{capability: ContactBatchSend, kind: "contact"},
		{capability: ContactBatchSend, kind: "contact_batch_extra"},
		{capability: RoomBatchSend, kind: "room"},
		{capability: RoomBatchSend, kind: "room_batch_extra"},
		{capability: ContactBatchSend, kind: string(DispatchKindRoomBatch)},
	} {
		if DispatchKindMatchesCapability(invalid.capability, DispatchKind(invalid.kind)) {
			t.Fatalf("non-canonical kind %q matched capability %q", invalid.kind, invalid.capability)
		}
	}
}
