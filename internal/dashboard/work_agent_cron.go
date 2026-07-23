package dashboard

import (
	"context"
	"fmt"
	"log"
	"strings"
)

type WorkAgentSyncItem struct {
	ID        int
	CorpID    int
	WXCorpID  string
	WXAgentID string
	WXSecret  string
}

type WorkAgentSyncStore interface {
	WorkAgentsForSync(ctx context.Context) ([]WorkAgentSyncItem, error)
	UpdateWorkAgentDetail(ctx context.Context, agentID int, detail WorkAgentDetail) (bool, error)
}

type WorkAgentSyncClient interface {
	AgentDetail(ctx context.Context, credential RoomWelcomeCorpCredential, wxSecret string, wxAgentID string) (WorkAgentDetail, error)
}

type WorkAgentSyncResult struct {
	ItemsScanned int
	ItemsUpdated int
	ItemsSkipped int
}

type WorkAgentSyncCron struct {
	store  WorkAgentSyncStore
	client WorkAgentSyncClient
	logger *log.Logger
}

func NewWorkAgentSyncCron(store WorkAgentSyncStore, client WorkAgentSyncClient, logger *log.Logger) *WorkAgentSyncCron {
	if logger == nil {
		logger = log.Default()
	}
	return &WorkAgentSyncCron{store: store, client: client, logger: logger}
}

func (c *WorkAgentSyncCron) RunOnce(ctx context.Context) error {
	if c.store == nil || c.client == nil {
		return fmt.Errorf("pullAgent cron dependencies are not configured")
	}
	agents, err := c.store.WorkAgentsForSync(ctx)
	if err != nil {
		return err
	}
	result := WorkAgentSyncResult{}
	var firstErr error
	for _, agent := range agents {
		if agent.ID <= 0 {
			continue
		}
		result.ItemsScanned++
		if strings.TrimSpace(agent.WXCorpID) == "" || strings.TrimSpace(agent.WXAgentID) == "" || strings.TrimSpace(agent.WXSecret) == "" {
			c.logger.Printf("pullAgent cron skipped: agent=%d corp=%d credential missing", agent.ID, agent.CorpID)
			result.ItemsSkipped++
			continue
		}
		detail, err := c.client.AgentDetail(ctx, RoomWelcomeCorpCredential{WXCorpID: agent.WXCorpID}, agent.WXSecret, agent.WXAgentID)
		if err != nil {
			c.logger.Printf("pullAgent cron refresh failed: agent=%d corp=%d err=%v", agent.ID, agent.CorpID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("agent %d: %w", agent.ID, err)
			}
			continue
		}
		updated, err := c.store.UpdateWorkAgentDetail(ctx, agent.ID, detail)
		if err != nil {
			c.logger.Printf("pullAgent cron update failed: agent=%d corp=%d err=%v", agent.ID, agent.CorpID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("agent %d update: %w", agent.ID, err)
			}
			continue
		}
		if !updated {
			c.logger.Printf("pullAgent cron skipped: agent=%d update affected no rows", agent.ID)
			result.ItemsSkipped++
			continue
		}
		result.ItemsUpdated++
	}
	c.logger.Printf("pullAgent cron finished: scanned=%d updated=%d skipped=%d", result.ItemsScanned, result.ItemsUpdated, result.ItemsSkipped)
	return firstErr
}
