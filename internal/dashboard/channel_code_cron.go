package dashboard

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

type ChannelCodeCronItem struct {
	ID               int
	CorpID           int
	AutoAddFriend    int
	DrainageEmployee map[string]any
	WXConfigID       string
	ValidUntil       string
	LifecycleState   string
}

type ChannelCodeCronStore interface {
	ChannelCodesForCron(ctx context.Context) ([]ChannelCodeCronItem, error)
	ChannelCodeEmployeeWXUserIDs(ctx context.Context, employeeIDs []int) ([]string, error)
	ChannelCodeContactCountsByEmployee(ctx context.Context, employeeIDs []int) (map[int]int, error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	UpdateChannelCodeQRCode(ctx context.Context, channelCodeID int, qrCodeURL string, wxConfigID string) error
	DeleteChannelCode(ctx context.Context, channelCodeID int) error
}

type ChannelCodeCronClient interface {
	CreateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, state string) (qrCodeURL string, configID string, err error)
	UpdateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, configID string, userIDs []string, skipVerify bool, state string) error
}

type ChannelCodeCronResult struct {
	ItemsScanned int
	ItemsUpdated int
	ItemsSkipped int
	ItemsFailed  int
}

type ChannelCodeCron struct {
	store  ChannelCodeCronStore
	client ChannelCodeCronClient
	logger *log.Logger
	now    func() time.Time
}

func NewChannelCodeCron(store ChannelCodeCronStore, client ChannelCodeCronClient, logger *log.Logger) *ChannelCodeCron {
	if logger == nil {
		logger = log.Default()
	}
	return &ChannelCodeCron{store: store, client: client, logger: logger}
}

func (c *ChannelCodeCron) RunOnce(ctx context.Context) error {
	if c.store == nil || c.client == nil {
		return fmt.Errorf("channelCode cron dependencies are not configured")
	}
	items, err := c.store.ChannelCodesForCron(ctx)
	if err != nil {
		return err
	}
	result := ChannelCodeCronResult{}
	var firstErr error
	for _, item := range items {
		if item.ID <= 0 {
			continue
		}
		result.ItemsScanned++
		if item.LifecycleState != "" && item.LifecycleState != "active" {
			result.ItemsSkipped++
			continue
		}
		expired, err := c.expireIfNeeded(ctx, item)
		if err != nil {
			c.logger.Printf("channelCode cron expiry failed: channel=%d corp=%d err=%v", item.ID, item.CorpID, err)
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("channel %d expiry: %w", item.ID, err)
			}
			continue
		}
		if expired {
			result.ItemsUpdated++
			continue
		}
		updated, err := c.refreshItem(ctx, item)
		if err != nil {
			c.logger.Printf("channelCode cron refresh failed: channel=%d corp=%d err=%v", item.ID, item.CorpID, err)
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("channel %d: %w", item.ID, err)
			}
			continue
		}
		if updated {
			result.ItemsUpdated++
			continue
		}
		result.ItemsSkipped++
	}
	c.logger.Printf("channelCode cron finished: scanned=%d updated=%d skipped=%d failed=%d", result.ItemsScanned, result.ItemsUpdated, result.ItemsSkipped, result.ItemsFailed)
	return firstErr
}

func (c *ChannelCodeCron) expireIfNeeded(ctx context.Context, item ChannelCodeCronItem) (bool, error) {
	if strings.TrimSpace(item.ValidUntil) == "" {
		return false, nil
	}
	validUntil, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(item.ValidUntil), time.Local)
	if err != nil {
		return false, fmt.Errorf("invalid valid_until: %w", err)
	}
	if c.currentTime().Before(validUntil) {
		return false, nil
	}
	lifecycleStore, ok := c.store.(ChannelCodeLifecycleStore)
	if !ok {
		return false, fmt.Errorf("channelCode lifecycle store is not configured")
	}
	deleter, ok := c.client.(ChannelCodeContactWayDeleter)
	if !ok {
		return false, fmt.Errorf("channelCode contact way deleter is not configured")
	}
	credential, configID, found, err := lifecycleStore.ChannelCodeProviderConfig(ctx, item.ID, item.CorpID)
	if err != nil {
		return false, err
	}
	if !found || configID == "" {
		return false, fmt.Errorf("channelCode provider config is missing")
	}
	if err := deleter.DeleteContactWay(ctx, credential, configID); err != nil {
		return false, err
	}
	if err := lifecycleStore.SetChannelCodeLifecycle(ctx, item.ID, item.CorpID, "expired", "synced", ""); err != nil {
		return false, err
	}
	return true, nil
}

func (c *ChannelCodeCron) refreshItem(ctx context.Context, item ChannelCodeCronItem) (bool, error) {
	employeeIDs := channelCodeActiveEmployeeIDs(item.DrainageEmployee, c.currentTime())
	if len(employeeIDs) == 0 {
		return false, nil
	}
	if addMax, ok := channelCodeMap(item.DrainageEmployee["addMax"]); ok && channelCodeInt(addMax["status"]) == 1 {
		counts, err := c.store.ChannelCodeContactCountsByEmployee(ctx, employeeIDs)
		if err != nil {
			return false, err
		}
		employeeIDs = channelCodeApplyAddMax(employeeIDs, addMax, counts)
	}
	if len(employeeIDs) == 0 {
		return false, nil
	}
	wxUserIDs, err := c.store.ChannelCodeEmployeeWXUserIDs(ctx, employeeIDs)
	if err != nil {
		return false, err
	}
	if len(wxUserIDs) == 0 {
		return false, nil
	}
	credential, found, err := c.store.RoomWelcomeCorpCredentialByID(ctx, item.CorpID)
	if err != nil {
		return false, err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return false, nil
	}
	skipVerify := item.AutoAddFriend == 1
	state := fmt.Sprintf("channelCode-%d", item.ID)
	if strings.TrimSpace(item.WXConfigID) == "" {
		qrCodeURL, configID, err := c.client.CreateContactWay(ctx, credential, wxUserIDs, skipVerify, state)
		if err != nil {
			_ = c.store.DeleteChannelCode(ctx, item.ID)
			return false, err
		}
		if err := c.store.UpdateChannelCodeQRCode(ctx, item.ID, qrCodeURL, configID); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := c.client.UpdateContactWay(ctx, credential, item.WXConfigID, wxUserIDs, skipVerify, state); err != nil {
		return false, err
	}
	return true, nil
}

func (c *ChannelCodeCron) currentTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}
