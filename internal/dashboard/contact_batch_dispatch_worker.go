package dashboard

import (
	"context"
	"errors"
	"log"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

// ContactBatchDispatchWorkItem is the database-owned unit handed to the
// contact-only runner. The actor and credential generation are read from the
// operation/identity rows; the worker never trusts the create request again.
type ContactBatchDispatchWorkItem struct {
	Principal         dashboardprincipal.DashboardPrincipal
	Dispatch          wecomcapability.Dispatch
	CredentialVersion uint64
}

type ContactBatchDispatchDueStore interface {
	ContactBatchDispatchDue(context.Context, int) ([]ContactBatchDispatchWorkItem, error)
}

// ContactBatchDispatchCron is the production lifecycle boundary for durable
// contact dispatches. It deliberately has no room path and no legacy batch
// sender fallback.
type ContactBatchDispatchCron struct {
	store  ContactBatchDispatchDueStore
	runner *wecomcapability.DispatchRunner
	lease  time.Duration
	limit  int
	logger *log.Logger
}

func NewContactBatchDispatchCron(store ContactBatchDispatchDueStore, runner *wecomcapability.DispatchRunner, lease time.Duration, limit int, logger *log.Logger) *ContactBatchDispatchCron {
	if lease <= 0 {
		lease = 5 * time.Minute
	}
	if limit <= 0 {
		limit = 50
	}
	if logger == nil {
		logger = log.Default()
	}
	return &ContactBatchDispatchCron{store: store, runner: runner, lease: lease, limit: limit, logger: logger}
}

func (c *ContactBatchDispatchCron) RunOnce(ctx context.Context) error {
	if c == nil || c.store == nil || c.runner == nil {
		return errors.New("contact batch dispatch runner is not configured")
	}
	items, err := c.store.ContactBatchDispatchDue(ctx, c.limit)
	if err != nil {
		return err
	}
	var firstErr error
	for _, item := range items {
		if item.Dispatch.DispatchKind != string(wecomcapability.DispatchKindContactBatch) || item.CredentialVersion == 0 || item.Principal.TenantID <= 0 || item.Principal.CorpID <= 0 || item.Principal.AuthVersion == 0 {
			if firstErr == nil {
				firstErr = errors.New("contact batch dispatch work item is invalid")
			}
			continue
		}
		_, runErr := c.runner.Run(ctx, wecomcapability.DispatchRunRequest{
			Principal: wecomcapability.DispatchPrincipal{
				UserID: item.Principal.UserID, TenantID: item.Principal.TenantID, CorpID: item.Principal.CorpID,
				IsSuperAdmin: item.Principal.IsSuperAdmin, AuthVersion: item.Principal.AuthVersion,
			}, Capability: wecomcapability.ContactBatchSend,
			ExpectedCredentialVersion: item.CredentialVersion, DispatchID: item.Dispatch.ID, LeaseDuration: c.lease,
		})
		if runErr != nil {
			if firstErr == nil {
				firstErr = runErr
			}
			c.logger.Printf("contact batch dispatch id=%d failed: %v", item.Dispatch.ID, runErr)
		}
	}
	return firstErr
}
