package dashboard

import (
	"context"
	"fmt"
	"log"
	"time"
)

type CorpDataCronStore interface {
	ActiveCorpIDs(ctx context.Context) ([]int, error)
	RefreshCorpDayData(ctx context.Context, corpID int, now time.Time) (CorpDataCronResult, error)
}

type CorpDataCronResult struct {
	CorpID         int
	AddContactNum  int
	AddRoomNum     int
	AddIntoRoomNum int
	LossContactNum int
	QuitRoomNum    int
	Date           string
}

type CorpDataCron struct {
	store  CorpDataCronStore
	logger *log.Logger
	now    func() time.Time
}

func NewCorpDataCron(store CorpDataCronStore, logger *log.Logger) *CorpDataCron {
	if logger == nil {
		logger = log.Default()
	}
	return &CorpDataCron{store: store, logger: logger}
}

func (c *CorpDataCron) RunOnce(ctx context.Context) error {
	if c.store == nil {
		return fmt.Errorf("corpData cron store is not configured")
	}
	corpIDs, err := c.store.ActiveCorpIDs(ctx)
	if err != nil {
		return err
	}
	now := c.currentTime()
	var firstErr error
	for _, corpID := range corpIDs {
		if corpID <= 0 {
			continue
		}
		result, err := c.store.RefreshCorpDayData(ctx, corpID, now)
		if err != nil {
			c.logger.Printf("corpData cron refresh failed: corp=%d err=%v", corpID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("corp %d: %w", corpID, err)
			}
			continue
		}
		c.logger.Printf(
			"corpData cron refreshed: corp=%d date=%s add_contact=%d add_room=%d add_into_room=%d loss_contact=%d quit_room=%d",
			result.CorpID,
			result.Date,
			result.AddContactNum,
			result.AddRoomNum,
			result.AddIntoRoomNum,
			result.LossContactNum,
			result.QuitRoomNum,
		)
	}
	return firstErr
}

func (c *CorpDataCron) currentTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}
