package dashboard

import (
	"context"
	"fmt"
	"log"
	"strings"
)

type RoomTagPullCronCorp struct {
	CorpID        int
	WXCorpID      string
	ContactSecret string
}

type RoomTagPullCronActivity struct {
	ID     int
	WXTIDs []RoomTagPullWXTID
}

type RoomTagPullCronStore interface {
	RoomTagPullCronCorps(ctx context.Context) ([]RoomTagPullCronCorp, error)
	RoomTagPullCronActivitiesByCorp(ctx context.Context, corpID int) ([]RoomTagPullCronActivity, error)
	UpdateRoomTagPullWXTIDs(ctx context.Context, activityID int, tids []RoomTagPullWXTID) error
	RoomTagPullHasPendingContacts(ctx context.Context, activityID int) (bool, error)
	UpdateRoomTagPullContactSendStatus(ctx context.Context, activityID int, wxUserID string, externalUserID string, status int) error
}

type RoomTagPullCronResult struct {
	CorpsScanned      int
	ActivitiesScanned int
	TasksUpdated      int
	ContactsUpdated   int
	ItemsSkipped      int
	ItemsFailed       int
}

type RoomTagPullCron struct {
	store  RoomTagPullCronStore
	client BatchSendResultCronClient
	logger *log.Logger
}

func NewRoomTagPullCron(store RoomTagPullCronStore, client BatchSendResultCronClient, logger *log.Logger) *RoomTagPullCron {
	if logger == nil {
		logger = log.Default()
	}
	return &RoomTagPullCron{store: store, client: client, logger: logger}
}

func (c *RoomTagPullCron) RunOnce(ctx context.Context) error {
	if c.store == nil || c.client == nil {
		return fmt.Errorf("RoomTagPull cron dependencies are not configured")
	}
	corps, err := c.store.RoomTagPullCronCorps(ctx)
	if err != nil {
		return err
	}
	result := RoomTagPullCronResult{}
	var firstErr error
	for _, corp := range corps {
		if corp.CorpID <= 0 {
			continue
		}
		result.CorpsScanned++
		if strings.TrimSpace(corp.WXCorpID) == "" || strings.TrimSpace(corp.ContactSecret) == "" {
			result.ItemsSkipped++
			continue
		}
		corpResult, err := c.syncCorp(ctx, corp)
		result.ActivitiesScanned += corpResult.ActivitiesScanned
		result.TasksUpdated += corpResult.TasksUpdated
		result.ContactsUpdated += corpResult.ContactsUpdated
		result.ItemsSkipped += corpResult.ItemsSkipped
		result.ItemsFailed += corpResult.ItemsFailed
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("corp %d: %w", corp.CorpID, err)
		}
	}
	c.logger.Printf("RoomTagPull cron finished: corps=%d activities=%d tasks_updated=%d contacts_updated=%d skipped=%d failed=%d", result.CorpsScanned, result.ActivitiesScanned, result.TasksUpdated, result.ContactsUpdated, result.ItemsSkipped, result.ItemsFailed)
	return firstErr
}

func (c *RoomTagPullCron) syncCorp(ctx context.Context, corp RoomTagPullCronCorp) (RoomTagPullCronResult, error) {
	activities, err := c.store.RoomTagPullCronActivitiesByCorp(ctx, corp.CorpID)
	if err != nil {
		return RoomTagPullCronResult{}, err
	}
	result := RoomTagPullCronResult{}
	credential := RoomWelcomeCorpCredential{CorpID: corp.CorpID, WXCorpID: corp.WXCorpID, ContactSecret: corp.ContactSecret}
	var firstErr error
	for _, activity := range activities {
		if activity.ID <= 0 {
			continue
		}
		result.ActivitiesScanned++
		if len(activity.WXTIDs) == 0 {
			result.ItemsSkipped++
			continue
		}
		tasksUpdated, err := c.syncActivityTasks(ctx, credential, activity)
		if err != nil {
			c.logger.Printf("RoomTagPull cron task sync failed: activity=%d corp=%d err=%v", activity.ID, corp.CorpID, err)
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("activity %d task: %w", activity.ID, err)
			}
			continue
		}
		result.TasksUpdated += tasksUpdated
		pending, err := c.store.RoomTagPullHasPendingContacts(ctx, activity.ID)
		if err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("activity %d pending: %w", activity.ID, err)
			}
			continue
		}
		if !pending {
			result.ItemsSkipped++
			continue
		}
		contactsUpdated, err := c.syncActivityContacts(ctx, credential, activity)
		if err != nil {
			c.logger.Printf("RoomTagPull cron contact sync failed: activity=%d corp=%d err=%v", activity.ID, corp.CorpID, err)
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("activity %d contact: %w", activity.ID, err)
			}
			continue
		}
		result.ContactsUpdated += contactsUpdated
	}
	return result, firstErr
}

func (c *RoomTagPullCron) syncActivityTasks(ctx context.Context, credential RoomWelcomeCorpCredential, activity RoomTagPullCronActivity) (int, error) {
	tids := append([]RoomTagPullWXTID{}, activity.WXTIDs...)
	updated := 0
	for i, tid := range tids {
		if tid.Status == batchSendStatusSent || strings.TrimSpace(tid.TID) == "" {
			continue
		}
		page, err := c.client.GroupMessageTasks(ctx, credential, tid.TID, batchSendPageLimit, "")
		if err != nil {
			return updated, err
		}
		for _, task := range page.TaskList {
			if tids[i].Status != task.Status {
				updated++
			}
			tids[i].Status = task.Status
		}
	}
	if err := c.store.UpdateRoomTagPullWXTIDs(ctx, activity.ID, tids); err != nil {
		return updated, err
	}
	return updated, nil
}

func (c *RoomTagPullCron) syncActivityContacts(ctx context.Context, credential RoomWelcomeCorpCredential, activity RoomTagPullCronActivity) (int, error) {
	updated := 0
	for _, tid := range activity.WXTIDs {
		if strings.TrimSpace(tid.TID) == "" || strings.TrimSpace(tid.WXUserID) == "" {
			continue
		}
		count, err := c.syncActivityContactPage(ctx, credential, activity.ID, tid, "")
		if err != nil {
			return updated, err
		}
		updated += count
	}
	return updated, nil
}

func (c *RoomTagPullCron) syncActivityContactPage(ctx context.Context, credential RoomWelcomeCorpCredential, activityID int, tid RoomTagPullWXTID, cursor string) (int, error) {
	page, err := c.client.GroupMessageSendResults(ctx, credential, tid.TID, tid.WXUserID, batchSendPageLimit, cursor)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, send := range page.SendList {
		if strings.TrimSpace(send.ExternalUserID) == "" {
			continue
		}
		userID := strings.TrimSpace(send.UserID)
		if userID == "" {
			userID = tid.WXUserID
		}
		if err := c.store.UpdateRoomTagPullContactSendStatus(ctx, activityID, userID, send.ExternalUserID, send.Status); err != nil {
			return updated, err
		}
		updated++
	}
	if strings.TrimSpace(page.NextCursor) == "" {
		return updated, nil
	}
	nextUpdated, err := c.syncActivityContactPage(ctx, credential, activityID, tid, page.NextCursor)
	return updated + nextUpdated, err
}
