package archive

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type DurableArchiveBinding struct {
	Scope           Scope
	WXCorpID        string
	IntegrationMode string
}

type DurableArchivePendingRun struct {
	Binding        DurableArchiveBinding
	Cursor         Cursor
	IdempotencyKey string
}

type DurableBridgeStore interface {
	SyncStore
	DurableArchiveBindings(context.Context) ([]DurableArchiveBinding, error)
	DurableArchiveBindingForScope(context.Context, Scope) (DurableArchiveBinding, bool, error)
	PendingDurableArchiveRuns(context.Context, int) ([]DurableArchivePendingRun, error)
	LatestArchiveSyncCursor(context.Context, Scope, string) (Cursor, error)
}

var ErrDurableArchiveBindingUnavailable = errors.New("durable archive binding unavailable")

type DurableBridgeRunner struct {
	store  DurableBridgeStore
	client *BridgeArchiveClient
	limit  int
	now    func() time.Time
}

func NewDurableBridgeRunner(store DurableBridgeStore, client *BridgeArchiveClient, limit int) *DurableBridgeRunner {
	if limit <= 0 {
		limit = DefaultFetchLimit
	}
	return &DurableBridgeRunner{store: store, client: client, limit: limit, now: time.Now}
}

func (r *DurableBridgeRunner) WithClock(now func() time.Time) *DurableBridgeRunner {
	if r != nil && now != nil {
		r.now = now
	}
	return r
}

// Enqueue records a manual run in the same durable ledger used by periodic
// archive synchronization. No bridge call is made on the request path.
func (r *DurableBridgeRunner) Enqueue(ctx context.Context, binding DurableArchiveBinding, requestID string) (SyncRun, error) {
	if r == nil || r.store == nil || r.client == nil {
		return SyncRun{}, errors.New("durable archive bridge runner is not configured")
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 96 {
		return SyncRun{}, errors.New("archive manual request id is invalid")
	}
	source, err := NewBridgeSource(r.client, binding.Scope, binding.WXCorpID, binding.IntegrationMode)
	if err != nil {
		return SyncRun{}, err
	}
	cursor, err := r.store.LatestArchiveSyncCursor(ctx, binding.Scope, source.SourceID())
	if err != nil {
		return SyncRun{}, err
	}
	return NewSyncService(r.store).Enqueue(ctx, source, SyncRequest{
		Scope: binding.Scope, StartCursor: cursor, Limit: r.limit, RetryFailed: true,
		IdempotencyKey: "archive:manual:" + requestID,
	})
}

func (r *DurableBridgeRunner) EnqueueScope(ctx context.Context, scope Scope, requestID string) (SyncRun, error) {
	if r == nil || r.store == nil {
		return SyncRun{}, errors.New("durable archive bridge runner is not configured")
	}
	binding, ok, err := r.store.DurableArchiveBindingForScope(ctx, scope)
	if err != nil {
		return SyncRun{}, err
	}
	if !ok {
		return SyncRun{}, ErrDurableArchiveBindingUnavailable
	}
	return r.Enqueue(ctx, binding, requestID)
}

func (r *DurableBridgeRunner) RunOnce(ctx context.Context) error {
	if r == nil || r.store == nil || r.client == nil {
		return errors.New("durable archive bridge runner is not configured")
	}
	pending, err := r.store.PendingDurableArchiveRuns(ctx, r.limit)
	if err != nil {
		return err
	}
	var firstErr error
	processedScopes := make(map[Scope]struct{}, len(pending))
	for _, item := range pending {
		processedScopes[item.Binding.Scope] = struct{}{}
		if err := r.syncBinding(ctx, item.Binding, item.Cursor, item.IdempotencyKey); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	bindings, err := r.store.DurableArchiveBindings(ctx)
	if err != nil {
		if firstErr != nil {
			return firstErr
		}
		return err
	}
	for _, binding := range bindings {
		if _, pendingForScope := processedScopes[binding.Scope]; pendingForScope {
			continue
		}
		source, err := NewBridgeSource(r.client, binding.Scope, binding.WXCorpID, binding.IntegrationMode)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		cursor, err := r.store.LatestArchiveSyncCursor(ctx, binding.Scope, source.SourceID())
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		err = r.syncBinding(ctx, binding, cursor, DurableArchivePollIdempotencyKey(binding.Scope, source.SourceID(), cursor, r.now()))
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *DurableBridgeRunner) syncBinding(ctx context.Context, binding DurableArchiveBinding, cursor Cursor, key string) error {
	source, err := NewBridgeSource(r.client, binding.Scope, binding.WXCorpID, binding.IntegrationMode)
	if err != nil {
		return err
	}
	_, err = NewSyncService(r.store).Sync(ctx, source, SyncRequest{
		Scope: binding.Scope, StartCursor: cursor, Limit: r.limit, RetryFailed: true, IdempotencyKey: key,
	})
	return err
}

func DurableArchiveIdempotencyKey(scope Scope, sourceID string, cursor Cursor) string {
	return fmt.Sprintf("archive:%d:%d:%s:%d", scope.TenantID, scope.CorpID, strings.TrimSpace(sourceID), cursor.Sequence)
}

func DurableArchivePollIdempotencyKey(scope Scope, sourceID string, cursor Cursor, at time.Time) string {
	return fmt.Sprintf("archive:poll:%d:%d:%s:%d:%s", scope.TenantID, scope.CorpID, strings.TrimSpace(sourceID), cursor.Sequence, at.UTC().Format("200601021504"))
}

func ValidateArchivePipelineFlags(legacyEnabled, durableEnabled bool) error {
	if legacyEnabled && durableEnabled {
		return errors.New("legacy and durable work message archive pipelines are mutually exclusive")
	}
	return nil
}
