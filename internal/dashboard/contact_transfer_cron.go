package dashboard

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

const ContactTransferStateCursorTTL = time.Hour

type ContactTransferStateLog struct {
	ID                 int
	CorpID             int
	ContactID          string
	HandoverEmployeeID string
	TakeoverEmployeeID string
	State              int
}

type ContactTransferStateResult struct {
	ExternalUserID string
	Status         int
}

type ContactTransferStateStore interface {
	NextContactTransferStateLog(ctx context.Context, afterID int) (ContactTransferStateLog, bool, error)
	UpdateContactTransferLogState(ctx context.Context, logID int, state int) (bool, error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
}

type ContactTransferStateCache interface {
	ContactTransferStateLogID(ctx context.Context) (int, error)
	SetContactTransferStateLogID(ctx context.Context, logID int, ttl time.Duration) error
}

type ContactTransferStateClient interface {
	TransferResult(ctx context.Context, credential RoomWelcomeCorpCredential, handoverUserID string, takeoverUserID string) ([]ContactTransferStateResult, error)
}

type ContactTransferStateCron struct {
	store  ContactTransferStateStore
	cache  ContactTransferStateCache
	client ContactTransferStateClient
	logger *log.Logger
}

func NewContactTransferStateCron(store ContactTransferStateStore, cache ContactTransferStateCache, client ContactTransferStateClient, logger *log.Logger) *ContactTransferStateCron {
	if logger == nil {
		logger = log.Default()
	}
	return &ContactTransferStateCron{store: store, cache: cache, client: client, logger: logger}
}

func (c *ContactTransferStateCron) RunOnce(ctx context.Context) error {
	if c.store == nil || c.cache == nil || c.client == nil {
		return fmt.Errorf("TransferStateRefresh cron dependencies are not configured")
	}
	cursor, err := c.cache.ContactTransferStateLogID(ctx)
	if err != nil {
		return err
	}
	logItem, found, err := c.store.NextContactTransferStateLog(ctx, cursor)
	if err != nil {
		return err
	}
	if !found {
		if err := c.cache.SetContactTransferStateLogID(ctx, 0, ContactTransferStateCursorTTL); err != nil {
			return err
		}
		c.logger.Printf("TransferStateRefresh cron finished: no log after id=%d, cursor reset", cursor)
		return nil
	}
	if err := c.cache.SetContactTransferStateLogID(ctx, logItem.ID, ContactTransferStateCursorTTL); err != nil {
		return err
	}
	if strings.TrimSpace(logItem.HandoverEmployeeID) == "" || strings.TrimSpace(logItem.TakeoverEmployeeID) == "" || strings.TrimSpace(logItem.ContactID) == "" {
		c.logger.Printf("TransferStateRefresh cron skipped: log=%d missing ids", logItem.ID)
		return nil
	}
	credential, found, err := c.store.RoomWelcomeCorpCredentialByID(ctx, logItem.CorpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		c.logger.Printf("TransferStateRefresh cron skipped: log=%d corp=%d credential missing", logItem.ID, logItem.CorpID)
		return nil
	}
	results, err := c.client.TransferResult(ctx, credential, logItem.HandoverEmployeeID, logItem.TakeoverEmployeeID)
	if err != nil {
		return err
	}
	for _, result := range results {
		if strings.TrimSpace(result.ExternalUserID) != logItem.ContactID {
			continue
		}
		updated, err := c.store.UpdateContactTransferLogState(ctx, logItem.ID, result.Status)
		if err != nil {
			return err
		}
		if !updated {
			return fmt.Errorf("transfer state log %d update affected no rows", logItem.ID)
		}
		c.logger.Printf("TransferStateRefresh cron refreshed: log=%d corp=%d contact=%s state=%d", logItem.ID, logItem.CorpID, logItem.ContactID, result.Status)
		return nil
	}
	c.logger.Printf("TransferStateRefresh cron finished: log=%d contact=%s no matching result", logItem.ID, logItem.ContactID)
	return nil
}
