package dashboard

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

const batchSendSubmitChunkSize = 10000

type contactMessageBatchSendSubmitStore interface {
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	ContactMessageBatchSendByID(ctx context.Context, batchID int) (ContactMessageBatchSendItem, bool, error)
	CreateContactMessageBatchSendTasks(ctx context.Context, batchID int) ([]ContactMessageBatchSendSendTarget, error)
	MarkContactMessageBatchSendSubmitted(ctx context.Context, batchID int, results []ContactMessageBatchSendMessageResult) error
}

type roomMessageBatchSendSubmitStore interface {
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	RoomMessageBatchSendByID(ctx context.Context, batchID int) (RoomMessageBatchSendItem, bool, error)
	CreateRoomMessageBatchSendTasks(ctx context.Context, batchID int) ([]RoomMessageBatchSendTarget, error)
	MarkRoomMessageBatchSendSubmitted(ctx context.Context, batchID int, results []RoomMessageBatchSendMessageResult) error
}

type contactMessageBatchSendSubmitClient interface {
	UploadTemporaryImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error)
	SubmitContactMessageBatchSend(ctx context.Context, credential RoomWelcomeCorpCredential, payload ContactMessageBatchSendMessagePayload) (ContactMessageBatchSendMessageResult, error)
}

type roomMessageBatchSendSubmitClient interface {
	UploadTemporaryImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error)
	SubmitRoomMessageBatchSend(ctx context.Context, credential RoomWelcomeCorpCredential, payload RoomMessageBatchSendMessagePayload) (RoomMessageBatchSendMessageResult, error)
}

type ContactBatchSendScheduleStore interface {
	contactMessageBatchSendSubmitStore
	DueContactMessageBatchSendIDs(ctx context.Context, now time.Time) ([]int, error)
}

type RoomBatchSendScheduleStore interface {
	roomMessageBatchSendSubmitStore
	DueRoomMessageBatchSendIDs(ctx context.Context, now time.Time) ([]int, error)
}

type ContactBatchSendScheduleCron struct {
	store           ContactBatchSendScheduleStore
	client          contactMessageBatchSendSubmitClient
	fileStorageRoot string
	logger          *log.Logger
	now             func() time.Time
}

type RoomBatchSendScheduleCron struct {
	store           RoomBatchSendScheduleStore
	client          roomMessageBatchSendSubmitClient
	fileStorageRoot string
	logger          *log.Logger
	now             func() time.Time
}

type batchSendScheduleSummary struct {
	Scanned int
	Sent    int
	Skipped int
	Failed  int
}

func NewContactBatchSendScheduleCron(store ContactBatchSendScheduleStore, client contactMessageBatchSendSubmitClient, fileStorageRoot string, logger *log.Logger) *ContactBatchSendScheduleCron {
	if logger == nil {
		logger = log.Default()
	}
	if strings.TrimSpace(fileStorageRoot) == "" {
		fileStorageRoot = defaultRoomTagPullFileStorageRoot
	}
	return &ContactBatchSendScheduleCron{store: store, client: client, fileStorageRoot: fileStorageRoot, logger: logger}
}

func NewRoomBatchSendScheduleCron(store RoomBatchSendScheduleStore, client roomMessageBatchSendSubmitClient, fileStorageRoot string, logger *log.Logger) *RoomBatchSendScheduleCron {
	if logger == nil {
		logger = log.Default()
	}
	if strings.TrimSpace(fileStorageRoot) == "" {
		fileStorageRoot = defaultRoomTagPullFileStorageRoot
	}
	return &RoomBatchSendScheduleCron{store: store, client: client, fileStorageRoot: fileStorageRoot, logger: logger}
}

func (c *ContactBatchSendScheduleCron) RunOnce(ctx context.Context) error {
	if c.store == nil || c.client == nil {
		return fmt.Errorf("ContactMessageBatchSend scheduled cron dependencies are not configured")
	}
	ids, err := c.store.DueContactMessageBatchSendIDs(ctx, c.currentTime())
	if err != nil {
		return err
	}
	result := batchSendScheduleSummary{}
	var firstErr error
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		result.Scanned++
		sent, err := submitContactMessageBatchSend(ctx, c.store, c.client, c.fileStorageRoot, id)
		if err != nil {
			c.logger.Printf("ContactMessageBatchSend scheduled cron failed: batch=%d err=%v", id, err)
			result.Failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("batch %d: %w", id, err)
			}
			continue
		}
		if sent {
			result.Sent++
			continue
		}
		result.Skipped++
	}
	c.logger.Printf("ContactMessageBatchSend scheduled cron finished: scanned=%d sent=%d skipped=%d failed=%d", result.Scanned, result.Sent, result.Skipped, result.Failed)
	return firstErr
}

func (c *RoomBatchSendScheduleCron) RunOnce(ctx context.Context) error {
	if c.store == nil || c.client == nil {
		return fmt.Errorf("RoomMessageBatchSend scheduled cron dependencies are not configured")
	}
	ids, err := c.store.DueRoomMessageBatchSendIDs(ctx, c.currentTime())
	if err != nil {
		return err
	}
	result := batchSendScheduleSummary{}
	var firstErr error
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		result.Scanned++
		sent, err := submitRoomMessageBatchSend(ctx, c.store, c.client, c.fileStorageRoot, id)
		if err != nil {
			c.logger.Printf("RoomMessageBatchSend scheduled cron failed: batch=%d err=%v", id, err)
			result.Failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("batch %d: %w", id, err)
			}
			continue
		}
		if sent {
			result.Sent++
			continue
		}
		result.Skipped++
	}
	c.logger.Printf("RoomMessageBatchSend scheduled cron finished: scanned=%d sent=%d skipped=%d failed=%d", result.Scanned, result.Sent, result.Skipped, result.Failed)
	return firstErr
}

func (c *ContactBatchSendScheduleCron) currentTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *RoomBatchSendScheduleCron) currentTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func submitContactMessageBatchSend(ctx context.Context, store contactMessageBatchSendSubmitStore, client contactMessageBatchSendSubmitClient, fileStorageRoot string, batchID int) (bool, error) {
	if store == nil || client == nil {
		return false, fmt.Errorf("客户群发依赖未配置")
	}
	batch, found, err := store.ContactMessageBatchSendByID(ctx, batchID)
	if err != nil {
		return false, err
	}
	if !found || batch.SendStatus != 0 {
		return false, nil
	}
	credential, found, err := store.RoomWelcomeCorpCredentialByID(ctx, batch.CorpID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("企业授信信息不存在")
	}
	content, err := prepareContactMessageBatchSendContent(ctx, client, credential, fileStorageRoot, batch.Content)
	if err != nil {
		return false, err
	}
	targets, err := store.CreateContactMessageBatchSendTasks(ctx, batchID)
	if err != nil {
		return false, fmt.Errorf("客户群发消息创建失败: %w", err)
	}
	results := make([]ContactMessageBatchSendMessageResult, 0, len(targets))
	for _, target := range targets {
		if strings.TrimSpace(target.WXUserID) == "" || len(target.ExternalUserIDs) == 0 {
			continue
		}
		for _, externalUserIDs := range chunkStrings(target.ExternalUserIDs, batchSendSubmitChunkSize) {
			payload := ContactMessageBatchSendMessagePayload{
				Content:        content,
				ExternalUserID: externalUserIDs,
				Sender:         target.WXUserID,
			}
			result, err := client.SubmitContactMessageBatchSend(ctx, credential, payload)
			if err != nil {
				return false, fmt.Errorf("客户群发发送失败: %w", err)
			}
			result.EmployeeID = target.EmployeeID
			results = append(results, result)
		}
	}
	if err := store.MarkContactMessageBatchSendSubmitted(ctx, batchID, results); err != nil {
		return false, fmt.Errorf("客户群发状态更新失败: %w", err)
	}
	return true, nil
}

func submitRoomMessageBatchSend(ctx context.Context, store roomMessageBatchSendSubmitStore, client roomMessageBatchSendSubmitClient, fileStorageRoot string, batchID int) (bool, error) {
	if store == nil || client == nil {
		return false, fmt.Errorf("客户群群发依赖未配置")
	}
	batch, found, err := store.RoomMessageBatchSendByID(ctx, batchID)
	if err != nil {
		return false, err
	}
	if !found || batch.SendStatus != 0 {
		return false, nil
	}
	credential, found, err := store.RoomWelcomeCorpCredentialByID(ctx, batch.CorpID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("企业授信信息不存在")
	}
	content, err := prepareRoomMessageBatchSendContent(ctx, client, credential, fileStorageRoot, batch.Content)
	if err != nil {
		return false, err
	}
	targets, err := store.CreateRoomMessageBatchSendTasks(ctx, batchID)
	if err != nil {
		return false, fmt.Errorf("客户群群发消息创建失败: %w", err)
	}
	results := make([]RoomMessageBatchSendMessageResult, 0, len(targets))
	for _, target := range targets {
		if strings.TrimSpace(target.WXUserID) == "" || len(target.ChatIDs) == 0 {
			continue
		}
		for _, chatIDs := range chunkStrings(target.ChatIDs, batchSendSubmitChunkSize) {
			result, err := client.SubmitRoomMessageBatchSend(ctx, credential, RoomMessageBatchSendMessagePayload{
				Content: content,
				ChatIDs: chatIDs,
				Sender:  target.WXUserID,
			})
			if err != nil {
				return false, fmt.Errorf("客户群群发发送失败: %w", err)
			}
			result.EmployeeID = target.EmployeeID
			results = append(results, result)
		}
	}
	if err := store.MarkRoomMessageBatchSendSubmitted(ctx, batchID, results); err != nil {
		return false, fmt.Errorf("客户群群发状态更新失败: %w", err)
	}
	return true, nil
}

func prepareContactMessageBatchSendContent(ctx context.Context, client contactMessageBatchSendSubmitClient, credential RoomWelcomeCorpCredential, fileStorageRoot string, content []ContactMessageBatchSendContent) ([]ContactMessageBatchSendContent, error) {
	return prepareBatchSendContent(ctx, client.UploadTemporaryImage, credential, fileStorageRoot, content)
}

func prepareRoomMessageBatchSendContent(ctx context.Context, client roomMessageBatchSendSubmitClient, credential RoomWelcomeCorpCredential, fileStorageRoot string, content []ContactMessageBatchSendContent) ([]ContactMessageBatchSendContent, error) {
	return prepareBatchSendContent(ctx, client.UploadTemporaryImage, credential, fileStorageRoot, content)
}

func prepareBatchSendContent(ctx context.Context, upload func(context.Context, RoomWelcomeCorpCredential, string) (string, error), credential RoomWelcomeCorpCredential, fileStorageRoot string, content []ContactMessageBatchSendContent) ([]ContactMessageBatchSendContent, error) {
	prepared := make([]ContactMessageBatchSendContent, 0, len(content))
	for _, item := range content {
		switch item.MsgType {
		case "image":
			if item.PicURL != "" {
				localPath, filePath, err := contactMessageBatchSendMediaFilePath(fileStorageRoot, item.PicURL)
				if err != nil {
					return nil, fmt.Errorf("图片处理失败: %w", err)
				}
				mediaID, err := upload(ctx, credential, filePath)
				if err != nil {
					return nil, fmt.Errorf("上传图片失败: %w", err)
				}
				item.PicURL = localPath
				item.MediaID = mediaID
			}
		case "miniprogram":
			if item.PicURL != "" {
				localPath, filePath, err := contactMessageBatchSendMediaFilePath(fileStorageRoot, item.PicURL)
				if err != nil {
					return nil, fmt.Errorf("图片处理失败: %w", err)
				}
				mediaID, err := upload(ctx, credential, filePath)
				if err != nil {
					return nil, fmt.Errorf("上传图片失败: %w", err)
				}
				item.PicURL = localPath
				item.PicMediaID = mediaID
			}
		}
		prepared = append(prepared, item)
	}
	return prepared, nil
}

func chunkStrings(values []string, size int) [][]string {
	if size <= 0 {
		size = len(values)
	}
	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunk := append([]string{}, values[start:end]...)
		chunks = append(chunks, chunk)
	}
	return chunks
}
