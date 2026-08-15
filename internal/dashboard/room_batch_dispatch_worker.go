package dashboard

import (
	"context"
	"errors"
	"log"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

// RoomBatchDispatchWorkItem is the database-owned unit handed to the
// room-only runner. The actor and credential generation are read from the
// operation/identity rows; the worker never trusts the create request again.
type RoomBatchDispatchWorkItem struct {
	Principal         dashboardprincipal.DashboardPrincipal
	Dispatch          wecomcapability.Dispatch
	CredentialVersion uint64
}

type RoomBatchDispatchDueStore interface {
	RoomBatchDispatchDue(context.Context, int) ([]RoomBatchDispatchWorkItem, error)
}

// RoomBatchDispatchCron is the production lifecycle boundary for durable room
// dispatches. It deliberately has no contact path and no legacy batch sender
// fallback.
type RoomBatchDispatchCron struct {
	store  RoomBatchDispatchDueStore
	runner *wecomcapability.DispatchRunner
	lease  time.Duration
	limit  int
	logger *log.Logger
}

func NewRoomBatchDispatchCron(store RoomBatchDispatchDueStore, runner *wecomcapability.DispatchRunner, lease time.Duration, limit int, logger *log.Logger) *RoomBatchDispatchCron {
	if lease <= 0 {
		lease = 5 * time.Minute
	}
	if limit <= 0 {
		limit = 50
	}
	if logger == nil {
		logger = log.Default()
	}
	return &RoomBatchDispatchCron{store: store, runner: runner, lease: lease, limit: limit, logger: logger}
}

func (c *RoomBatchDispatchCron) RunOnce(ctx context.Context) error {
	if c == nil || c.store == nil || c.runner == nil {
		return errors.New("room batch dispatch runner is not configured")
	}
	items, err := c.store.RoomBatchDispatchDue(ctx, c.limit)
	if err != nil {
		return err
	}
	var firstErr error
	for _, item := range items {
		if item.Dispatch.DispatchKind != string(wecomcapability.DispatchKindRoomBatch) || item.CredentialVersion == 0 || item.Principal.TenantID <= 0 || item.Principal.CorpID <= 0 || item.Principal.AuthVersion == 0 {
			if firstErr == nil {
				firstErr = errors.New("room batch dispatch work item is invalid")
			}
			continue
		}
		_, runErr := c.runner.Run(ctx, wecomcapability.DispatchRunRequest{
			Principal: wecomcapability.DispatchPrincipal{
				UserID: item.Principal.UserID, TenantID: item.Principal.TenantID, CorpID: item.Principal.CorpID,
				IsSuperAdmin: item.Principal.IsSuperAdmin, AuthVersion: item.Principal.AuthVersion,
			}, Capability: wecomcapability.RoomBatchSend,
			ExpectedCredentialVersion: item.CredentialVersion, DispatchID: item.Dispatch.ID, LeaseDuration: c.lease,
		})
		if runErr != nil {
			if firstErr == nil {
				firstErr = runErr
			}
			c.logger.Printf("room batch dispatch id=%d failed: %v", item.Dispatch.ID, runErr)
		}
	}
	return firstErr
}
