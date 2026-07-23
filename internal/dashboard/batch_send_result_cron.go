package dashboard

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	batchSendStatusSent       = 1
	batchSendMessageNotSent   = 0
	batchSendMessageDelivered = 1
	batchSendPageLimit        = 500
)

type BatchSendGroupTask struct {
	UserID   string `json:"userid"`
	Status   int    `json:"status"`
	SendTime int64  `json:"send_time"`
}

type BatchSendGroupResult struct {
	UserID         string `json:"userid"`
	ExternalUserID string `json:"external_userid"`
	ChatID         string `json:"chat_id"`
	Status         int    `json:"status"`
	SendTime       int64  `json:"send_time"`
}

type BatchSendGroupTaskPage struct {
	ErrCode    int
	ErrMsg     string
	TaskList   []BatchSendGroupTask
	NextCursor string
}

type BatchSendGroupResultPage struct {
	ErrCode    int
	ErrMsg     string
	SendList   []BatchSendGroupResult
	NextCursor string
}

type ContactBatchSendSyncTarget struct {
	ID                int
	BatchID           int
	Status            int
	MsgID             string
	SendEmployeeTotal int
	SendContactTotal  int
	Credential        RoomWelcomeCorpCredential
}

type RoomBatchSendSyncTarget struct {
	ID                int
	BatchID           int
	Status            int
	MsgID             string
	SendEmployeeTotal int
	SendRoomTotal     int
	Credential        RoomWelcomeCorpCredential
}

type ContactBatchSendResultCronStore interface {
	ContactBatchSendEmployeeIDsForSync(ctx context.Context, since time.Time) ([]int, error)
	ContactBatchSendSyncTarget(ctx context.Context, batchEmployeeID int) (ContactBatchSendSyncTarget, bool, error)
	UpdateContactBatchSendEmployeeSent(ctx context.Context, batchEmployeeID int, errCode int, errMsg string, sendTime int64) error
	UpdateContactBatchSendResult(ctx context.Context, batchID int, externalUserID string, userID string, status int, sendTime int64) error
	RefreshContactBatchSendTotals(ctx context.Context, batchID int, sendEmployeeTotal int, sendContactTotal int) error
}

type RoomBatchSendResultCronStore interface {
	RoomBatchSendEmployeeIDsForSync(ctx context.Context, since time.Time) ([]int, error)
	RoomBatchSendSyncTarget(ctx context.Context, batchEmployeeID int) (RoomBatchSendSyncTarget, bool, error)
	UpdateRoomBatchSendEmployeeSent(ctx context.Context, batchEmployeeID int, errCode int, errMsg string, sendTime int64) error
	UpdateRoomBatchSendResult(ctx context.Context, batchID int, chatID string, userID string, status int, sendTime int64) error
	RefreshRoomBatchSendTotals(ctx context.Context, batchID int, sendEmployeeTotal int, sendRoomTotal int) error
}

type BatchSendResultCronClient interface {
	GroupMessageTasks(ctx context.Context, credential RoomWelcomeCorpCredential, msgID string, limit int, cursor string) (BatchSendGroupTaskPage, error)
	GroupMessageSendResults(ctx context.Context, credential RoomWelcomeCorpCredential, msgID string, userID string, limit int, cursor string) (BatchSendGroupResultPage, error)
}

type ContactBatchSendResultCron struct {
	store  ContactBatchSendResultCronStore
	client BatchSendResultCronClient
	logger *log.Logger
	now    func() time.Time
}

type RoomBatchSendResultCron struct {
	store  RoomBatchSendResultCronStore
	client BatchSendResultCronClient
	logger *log.Logger
	now    func() time.Time
}

func NewContactBatchSendResultCron(store ContactBatchSendResultCronStore, client BatchSendResultCronClient, logger *log.Logger) *ContactBatchSendResultCron {
	if logger == nil {
		logger = log.Default()
	}
	return &ContactBatchSendResultCron{store: store, client: client, logger: logger}
}

func NewRoomBatchSendResultCron(store RoomBatchSendResultCronStore, client BatchSendResultCronClient, logger *log.Logger) *RoomBatchSendResultCron {
	if logger == nil {
		logger = log.Default()
	}
	return &RoomBatchSendResultCron{store: store, client: client, logger: logger}
}

func (c *ContactBatchSendResultCron) RunOnce(ctx context.Context) error {
	if c.store == nil || c.client == nil {
		return fmt.Errorf("ContactSyncSendResultTask cron dependencies are not configured")
	}
	ids, err := c.store.ContactBatchSendEmployeeIDsForSync(ctx, c.since())
	if err != nil {
		return err
	}
	var firstErr error
	result := batchSendResultSummary{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		result.Scanned++
		updated, err := c.syncOne(ctx, id)
		if err != nil {
			c.logger.Printf("ContactSyncSendResultTask cron sync failed: batch_employee=%d err=%v", id, err)
			result.Failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("batch_employee %d: %w", id, err)
			}
			continue
		}
		if updated {
			result.Updated++
			continue
		}
		result.Skipped++
	}
	c.logger.Printf("ContactSyncSendResultTask cron finished: scanned=%d updated=%d skipped=%d failed=%d", result.Scanned, result.Updated, result.Skipped, result.Failed)
	return firstErr
}

func (c *ContactBatchSendResultCron) syncOne(ctx context.Context, batchEmployeeID int) (bool, error) {
	target, found, err := c.store.ContactBatchSendSyncTarget(ctx, batchEmployeeID)
	if err != nil || !found {
		return false, err
	}
	if target.Status == batchSendStatusSent || strings.TrimSpace(target.MsgID) == "" || strings.TrimSpace(target.Credential.WXCorpID) == "" || strings.TrimSpace(target.Credential.ContactSecret) == "" {
		return false, nil
	}
	taskPage, err := c.client.GroupMessageTasks(ctx, target.Credential, target.MsgID, batchSendPageLimit, "")
	if err != nil {
		return false, err
	}
	updated := false
	for _, task := range taskPage.TaskList {
		if task.Status == batchSendMessageNotSent || strings.TrimSpace(task.UserID) == "" {
			continue
		}
		if err := c.store.UpdateContactBatchSendEmployeeSent(ctx, target.ID, taskPage.ErrCode, taskPage.ErrMsg, task.SendTime); err != nil {
			return false, err
		}
		if err := c.syncResults(ctx, target, task.UserID, ""); err != nil {
			return false, err
		}
		updated = true
	}
	if err := c.store.RefreshContactBatchSendTotals(ctx, target.BatchID, target.SendEmployeeTotal, target.SendContactTotal); err != nil {
		return false, err
	}
	return updated, nil
}

func (c *ContactBatchSendResultCron) syncResults(ctx context.Context, target ContactBatchSendSyncTarget, userID string, cursor string) error {
	page, err := c.client.GroupMessageSendResults(ctx, target.Credential, target.MsgID, userID, batchSendPageLimit, cursor)
	if err != nil {
		return err
	}
	for _, result := range page.SendList {
		if strings.TrimSpace(result.ExternalUserID) == "" {
			continue
		}
		if err := c.store.UpdateContactBatchSendResult(ctx, target.BatchID, result.ExternalUserID, result.UserID, result.Status, result.SendTime); err != nil {
			return err
		}
	}
	if strings.TrimSpace(page.NextCursor) == "" {
		return nil
	}
	return c.syncResults(ctx, target, userID, page.NextCursor)
}

func (c *ContactBatchSendResultCron) since() time.Time {
	now := time.Now()
	if c.now != nil {
		now = c.now()
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -7)
}

func (c *RoomBatchSendResultCron) RunOnce(ctx context.Context) error {
	if c.store == nil || c.client == nil {
		return fmt.Errorf("RoomSyncSendResultTask cron dependencies are not configured")
	}
	ids, err := c.store.RoomBatchSendEmployeeIDsForSync(ctx, c.since())
	if err != nil {
		return err
	}
	var firstErr error
	result := batchSendResultSummary{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		result.Scanned++
		updated, err := c.syncOne(ctx, id)
		if err != nil {
			c.logger.Printf("RoomSyncSendResultTask cron sync failed: batch_employee=%d err=%v", id, err)
			result.Failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("batch_employee %d: %w", id, err)
			}
			continue
		}
		if updated {
			result.Updated++
			continue
		}
		result.Skipped++
	}
	c.logger.Printf("RoomSyncSendResultTask cron finished: scanned=%d updated=%d skipped=%d failed=%d", result.Scanned, result.Updated, result.Skipped, result.Failed)
	return firstErr
}

func (c *RoomBatchSendResultCron) syncOne(ctx context.Context, batchEmployeeID int) (bool, error) {
	target, found, err := c.store.RoomBatchSendSyncTarget(ctx, batchEmployeeID)
	if err != nil || !found {
		return false, err
	}
	if target.Status == batchSendStatusSent || strings.TrimSpace(target.MsgID) == "" || strings.TrimSpace(target.Credential.WXCorpID) == "" || strings.TrimSpace(target.Credential.ContactSecret) == "" {
		return false, nil
	}
	taskPage, err := c.client.GroupMessageTasks(ctx, target.Credential, target.MsgID, batchSendPageLimit, "")
	if err != nil {
		return false, err
	}
	updated := false
	for _, task := range taskPage.TaskList {
		if task.Status == batchSendMessageNotSent || strings.TrimSpace(task.UserID) == "" {
			continue
		}
		if err := c.store.UpdateRoomBatchSendEmployeeSent(ctx, target.ID, taskPage.ErrCode, taskPage.ErrMsg, task.SendTime); err != nil {
			return false, err
		}
		if err := c.syncResults(ctx, target, task.UserID, ""); err != nil {
			return false, err
		}
		updated = true
	}
	if err := c.store.RefreshRoomBatchSendTotals(ctx, target.BatchID, target.SendEmployeeTotal, target.SendRoomTotal); err != nil {
		return false, err
	}
	return updated, nil
}

func (c *RoomBatchSendResultCron) syncResults(ctx context.Context, target RoomBatchSendSyncTarget, userID string, cursor string) error {
	page, err := c.client.GroupMessageSendResults(ctx, target.Credential, target.MsgID, userID, batchSendPageLimit, cursor)
	if err != nil {
		return err
	}
	for _, result := range page.SendList {
		if strings.TrimSpace(result.ChatID) == "" {
			continue
		}
		if err := c.store.UpdateRoomBatchSendResult(ctx, target.BatchID, result.ChatID, result.UserID, result.Status, result.SendTime); err != nil {
			return err
		}
	}
	if strings.TrimSpace(page.NextCursor) == "" {
		return nil
	}
	return c.syncResults(ctx, target, userID, page.NextCursor)
}

func (c *RoomBatchSendResultCron) since() time.Time {
	now := time.Now()
	if c.now != nil {
		now = c.now()
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -7)
}

type batchSendResultSummary struct {
	Scanned int
	Updated int
	Skipped int
	Failed  int
}
