package archive

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type DurableArchiveBinding struct {
	Scope    Scope
	WXCorpID string
}

type DurableBridgeStore interface {
	SyncStore
	DurableArchiveBindings(context.Context) ([]DurableArchiveBinding, error)
	LatestArchiveSyncCursor(context.Context, Scope, string) (Cursor, error)
}

type DurableBridgeRunner struct {
	store  DurableBridgeStore
	client *BridgeArchiveClient
	limit  int
}

func NewDurableBridgeRunner(store DurableBridgeStore, client *BridgeArchiveClient, limit int) *DurableBridgeRunner {
	if limit <= 0 {
		limit = DefaultFetchLimit
	}
	return &DurableBridgeRunner{store: store, client: client, limit: limit}
}

func (r *DurableBridgeRunner) RunOnce(ctx context.Context) error {
	if r == nil || r.store == nil || r.client == nil {
		return errors.New("durable archive bridge runner is not configured")
	}
	bindings, err := r.store.DurableArchiveBindings(ctx)
	if err != nil {
		return err
	}
	var firstErr error
	for _, binding := range bindings {
		source, err := NewBridgeSource(r.client, binding.Scope, binding.WXCorpID)
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
		_, err = NewSyncService(r.store).Sync(ctx, source, SyncRequest{
			Scope: binding.Scope, StartCursor: cursor, Limit: r.limit, RetryFailed: true,
			IdempotencyKey: DurableArchiveIdempotencyKey(binding.Scope, source.SourceID(), cursor),
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func DurableArchiveIdempotencyKey(scope Scope, sourceID string, cursor Cursor) string {
	return fmt.Sprintf("archive:%d:%d:%s:%d", scope.TenantID, scope.CorpID, strings.TrimSpace(sourceID), cursor.Sequence)
}

func ValidateArchivePipelineFlags(legacyEnabled, durableEnabled bool) error {
	if legacyEnabled && durableEnabled {
		return errors.New("legacy and durable work message archive pipelines are mutually exclusive")
	}
	return nil
}
