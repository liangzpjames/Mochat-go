package dashboard

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MediumMediaCronStore interface {
	ActiveCorpIDs(ctx context.Context) ([]int, error)
	MediumMediaForUpdateByCorp(ctx context.Context, corpID int, olderThan int64) ([]MediumMediaUpdateItem, error)
	MediumCorpCredentialByID(ctx context.Context, corpID int) (MediumCorpCredential, bool, error)
	UpdateMediumMediaID(ctx context.Context, mediumID int, mediaID string, lastUploadTime int64) (bool, error)
}

type MediumMediaCronResult struct {
	CorpsScanned int
	ItemsScanned int
	ItemsUpdated int
	ItemsSkipped int
}

type MediumMediaCron struct {
	store           MediumMediaCronStore
	client          MediumMediaClient
	fileStorageRoot string
	logger          *log.Logger
	now             func() time.Time
}

func NewMediumMediaCron(store MediumMediaCronStore, client MediumMediaClient, fileStorageRoot string, logger *log.Logger) *MediumMediaCron {
	if logger == nil {
		logger = log.Default()
	}
	fileStorageRoot = strings.TrimSpace(fileStorageRoot)
	if fileStorageRoot == "" {
		fileStorageRoot = defaultMediumFileStorageRoot
	}
	return &MediumMediaCron{store: store, client: client, fileStorageRoot: fileStorageRoot, logger: logger}
}

func (c *MediumMediaCron) RunOnce(ctx context.Context) error {
	if c.store == nil || c.client == nil {
		return fmt.Errorf("mediaIdUpdate cron dependencies are not configured")
	}
	now := c.currentTime()
	olderThan := now.Unix() - mediumTemporaryMediaFreshLimit
	corpIDs, err := c.store.ActiveCorpIDs(ctx)
	if err != nil {
		return err
	}
	result := MediumMediaCronResult{}
	var firstErr error
	for _, corpID := range corpIDs {
		if corpID <= 0 {
			continue
		}
		result.CorpsScanned++
		items, err := c.store.MediumMediaForUpdateByCorp(ctx, corpID, olderThan)
		if err != nil {
			c.logger.Printf("mediaIdUpdate cron query failed: corp=%d err=%v", corpID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("corp %d: %w", corpID, err)
			}
			continue
		}
		if len(items) == 0 {
			continue
		}
		credential, found, err := c.store.MediumCorpCredentialByID(ctx, corpID)
		if err != nil {
			c.logger.Printf("mediaIdUpdate cron credential query failed: corp=%d err=%v", corpID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("corp %d credential: %w", corpID, err)
			}
			continue
		}
		if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.EmployeeSecret) == "" {
			c.logger.Printf("mediaIdUpdate cron skipped: corp=%d credential missing", corpID)
			result.ItemsSkipped += len(items)
			continue
		}
		for _, item := range items {
			result.ItemsScanned++
			updated, err := c.refreshItem(ctx, credential, item, now)
			if err != nil {
				c.logger.Printf("mediaIdUpdate cron refresh failed: corp=%d medium=%d err=%v", corpID, item.ID, err)
				if firstErr == nil {
					firstErr = fmt.Errorf("corp %d medium %d: %w", corpID, item.ID, err)
				}
				continue
			}
			if updated {
				result.ItemsUpdated++
				continue
			}
			result.ItemsSkipped++
		}
	}
	c.logger.Printf("mediaIdUpdate cron finished: corps=%d scanned=%d updated=%d skipped=%d", result.CorpsScanned, result.ItemsScanned, result.ItemsUpdated, result.ItemsSkipped)
	return firstErr
}

func (c *MediumMediaCron) refreshItem(ctx context.Context, credential MediumCorpCredential, item MediumMediaUpdateItem, now time.Time) (bool, error) {
	if now.Unix()-item.LastUploadTime <= mediumTemporaryMediaFreshLimit {
		return false, nil
	}
	mediaType := mediumWXMediaType(item.Type)
	if mediaType == "" {
		return false, nil
	}
	filePath := strings.TrimSpace(toString(item.Content[mediaType+"Path"]))
	if filePath == "" {
		return false, nil
	}
	localPath := mediumLocalFilePath(c.fileStorageRoot, filePath)
	info, err := os.Stat(localPath)
	if err != nil || info.IsDir() {
		return false, nil
	}
	mediaID, err := c.client.UploadTemporaryMedia(ctx, credential, mediaType, localPath)
	if err != nil {
		return false, err
	}
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return false, fmt.Errorf("企业微信接口未返回 media_id")
	}
	updated, err := c.store.UpdateMediumMediaID(ctx, item.ID, mediaID, now.Unix())
	if err != nil {
		return false, err
	}
	if !updated {
		return false, fmt.Errorf("素材 media_id 写回失败")
	}
	return true, nil
}

func (c *MediumMediaCron) currentTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func mediumLocalFilePath(root string, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	root = strings.TrimSpace(root)
	if root == "" {
		root = defaultMediumFileStorageRoot
	}
	return filepath.Join(root, strings.TrimLeft(path, "/"))
}
