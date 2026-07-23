package dashboard

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

const EmployeeStatisticCacheTTL = 24*time.Hour + 100*time.Second

type EmployeeStatisticTarget struct {
	ID            int
	CorpID        int
	WXUserID      string
	WXCorpID      string
	ContactSecret string
}

type EmployeeStatisticRecord struct {
	CorpID              int
	EmployeeID          int
	NewApplyCnt         int
	NewContactCnt       int
	ChatCnt             int
	MessageCnt          int
	ReplyPercentage     int
	AvgReplyTime        int
	NegativeFeedbackCnt int
	SynTime             time.Time
}

type EmployeeStatisticStore interface {
	EmployeeStatisticTargets(ctx context.Context) ([]EmployeeStatisticTarget, error)
	InsertEmployeeStatistic(ctx context.Context, record EmployeeStatisticRecord) error
}

type EmployeeStatisticCache interface {
	EmployeeStatisticApplied(ctx context.Context, corpID int, employeeID int, startUnix int64) (bool, error)
	SetEmployeeStatisticApplied(ctx context.Context, corpID int, employeeID int, startUnix int64, ttl time.Duration) error
}

type EmployeeStatisticClient interface {
	UserBehavior(ctx context.Context, credential RoomWelcomeCorpCredential, userIDs []string, start time.Time, end time.Time) ([]StatisticBehaviorData, error)
}

type EmployeeStatisticCronResult struct {
	ItemsScanned  int
	ItemsInserted int
	ItemsSkipped  int
	ItemsFailed   int
}

type EmployeeStatisticCron struct {
	store  EmployeeStatisticStore
	cache  EmployeeStatisticCache
	client EmployeeStatisticClient
	logger *log.Logger
	now    func() time.Time
}

func NewEmployeeStatisticCron(store EmployeeStatisticStore, cache EmployeeStatisticCache, client EmployeeStatisticClient, logger *log.Logger) *EmployeeStatisticCron {
	if logger == nil {
		logger = log.Default()
	}
	return &EmployeeStatisticCron{store: store, cache: cache, client: client, logger: logger}
}

func (c *EmployeeStatisticCron) RunOnce(ctx context.Context) error {
	return c.runOnce(ctx, nil, false)
}

func (c *EmployeeStatisticCron) RunOnceForCorpIDs(ctx context.Context, corpIDs []int) error {
	return c.runOnce(ctx, corpIDs, true)
}

func (c *EmployeeStatisticCron) runOnce(ctx context.Context, corpIDs []int, scoped bool) error {
	if c.store == nil || c.cache == nil || c.client == nil {
		return fmt.Errorf("employeeStatistic cron dependencies are not configured")
	}
	start, end := c.yesterdayRange()
	targets, err := c.store.EmployeeStatisticTargets(ctx)
	if err != nil {
		return err
	}
	if scoped {
		targets = filterEmployeeStatisticTargetsByCorpIDs(targets, corpIDs)
	}
	result := EmployeeStatisticCronResult{}
	var firstErr error
	for _, target := range targets {
		if target.ID <= 0 {
			continue
		}
		result.ItemsScanned++
		if strings.TrimSpace(target.WXUserID) == "" || strings.TrimSpace(target.WXCorpID) == "" || strings.TrimSpace(target.ContactSecret) == "" {
			c.logger.Printf("employeeStatistic cron skipped: employee=%d corp=%d credential missing", target.ID, target.CorpID)
			result.ItemsSkipped++
			continue
		}
		applied, err := c.cache.EmployeeStatisticApplied(ctx, target.CorpID, target.ID, start.Unix())
		if err != nil {
			return err
		}
		if applied {
			result.ItemsSkipped++
			continue
		}
		if err := c.cache.SetEmployeeStatisticApplied(ctx, target.CorpID, target.ID, start.Unix(), EmployeeStatisticCacheTTL); err != nil {
			return err
		}
		behavior, err := c.client.UserBehavior(ctx, RoomWelcomeCorpCredential{WXCorpID: target.WXCorpID, ContactSecret: target.ContactSecret}, []string{target.WXUserID}, start, end)
		if err != nil {
			c.logger.Printf("employeeStatistic cron fetch failed: employee=%d corp=%d err=%v", target.ID, target.CorpID, err)
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("employee %d: %w", target.ID, err)
			}
			continue
		}
		if len(behavior) == 0 {
			result.ItemsSkipped++
			continue
		}
		for _, item := range behavior {
			record := employeeStatisticRecordFromBehavior(target, item, start)
			if err := c.store.InsertEmployeeStatistic(ctx, record); err != nil {
				c.logger.Printf("employeeStatistic cron insert failed: employee=%d corp=%d err=%v", target.ID, target.CorpID, err)
				result.ItemsFailed++
				if firstErr == nil {
					firstErr = fmt.Errorf("employee %d insert: %w", target.ID, err)
				}
				continue
			}
			result.ItemsInserted++
		}
	}
	c.logger.Printf("employeeStatistic cron finished: date=%s scanned=%d inserted=%d skipped=%d failed=%d", start.Format("2006-01-02"), result.ItemsScanned, result.ItemsInserted, result.ItemsSkipped, result.ItemsFailed)
	return firstErr
}

func filterEmployeeStatisticTargetsByCorpIDs(targets []EmployeeStatisticTarget, corpIDs []int) []EmployeeStatisticTarget {
	allowed := make(map[int]struct{}, len(corpIDs))
	for _, corpID := range corpIDs {
		if corpID > 0 {
			allowed[corpID] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	filtered := make([]EmployeeStatisticTarget, 0, len(targets))
	for _, target := range targets {
		if _, ok := allowed[target.CorpID]; ok {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func (c *EmployeeStatisticCron) yesterdayRange() (time.Time, time.Time) {
	now := time.Now()
	if c.now != nil {
		now = c.now()
	}
	local := now.In(time.Local).AddDate(0, 0, -1)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	end := start.Add(24*time.Hour - time.Second)
	return start, end
}

func employeeStatisticRecordFromBehavior(target EmployeeStatisticTarget, item StatisticBehaviorData, synTime time.Time) EmployeeStatisticRecord {
	replyPercentage := 0
	if item.ReplyPercentage != 0 {
		replyPercentage = int(item.ReplyPercentage * 100)
	}
	return EmployeeStatisticRecord{
		CorpID:              target.CorpID,
		EmployeeID:          target.ID,
		NewApplyCnt:         item.NewApplyCnt,
		NewContactCnt:       item.NewApplyCnt,
		ChatCnt:             item.ChatCnt,
		MessageCnt:          item.MessageCnt,
		ReplyPercentage:     replyPercentage,
		AvgReplyTime:        int(item.AvgReplyTime),
		NegativeFeedbackCnt: item.NegativeFeedbackCnt,
		SynTime:             synTime,
	}
}
