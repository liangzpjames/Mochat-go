package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	WorkMessageArchiveCursorType        = 40
	WorkMessageArchiveMessageTableCount = 10
	WorkMessageArchiveDefaultSyncLimit  = 100
)

type WorkMessageArchiveCorp struct {
	CorpID        int
	WXCorpID      string
	ChatSecret    string
	RSAPublicKey  string
	RSAPrivateKey string
}

type WorkMessageArchiveMessage struct {
	Seq         int64
	MsgID       string
	Action      string
	From        string
	ToList      []string
	RoomID      string
	MsgType     string
	MsgTime     time.Time
	ContentRaw  string
	ContentText string
	RawJSON     string
}

type WorkMessageArchiveUpsertResult struct {
	Inserted bool
	Skipped  bool
	Resolved bool
}

type WorkMessageArchiveSyncResult struct {
	CorpsScanned     int
	MessagesFetched  int
	MessagesInserted int
	ItemsSkipped     int
	ItemsFailed      int
	LastSeq          int64
	FailedCorpID     int
}

type WorkMessageArchiveSyncStore interface {
	WorkMessageArchiveEnabledCorps(ctx context.Context) ([]WorkMessageArchiveCorp, error)
	WorkMessageArchiveCursor(ctx context.Context, corpID int) (int64, error)
	UpsertWorkMessageArchive(ctx context.Context, corpID int, message WorkMessageArchiveMessage) (WorkMessageArchiveUpsertResult, error)
	UpdateWorkMessageArchiveCursor(ctx context.Context, corpID int, lastSeq int64) error
}

type WorkMessageArchiveSyncClient interface {
	FetchWorkMessageArchive(ctx context.Context, corp WorkMessageArchiveCorp, seq int64, limit int) ([]WorkMessageArchiveMessage, error)
}

type WorkMessageArchiveSyncCron struct {
	mu                  sync.Mutex
	store               WorkMessageArchiveSyncStore
	client              WorkMessageArchiveSyncClient
	logger              *slog.Logger
	limit               int
	consecutiveFailures int
	lastFailureLog      time.Time
}

func NewWorkMessageArchiveSyncCron(store WorkMessageArchiveSyncStore, client WorkMessageArchiveSyncClient, logger *slog.Logger) *WorkMessageArchiveSyncCron {
	if logger == nil {
		logger = slog.Default()
	}
	return &WorkMessageArchiveSyncCron{
		store:  store,
		client: client,
		logger: logger,
		limit:  WorkMessageArchiveDefaultSyncLimit,
	}
}

func (c *WorkMessageArchiveSyncCron) WithLimit(limit int) *WorkMessageArchiveSyncCron {
	if limit > 0 {
		c.limit = limit
	}
	return c
}

func (c *WorkMessageArchiveSyncCron) RunOnce(ctx context.Context) error {
	startedAt := time.Now()
	if c.store == nil || c.client == nil {
		return fmt.Errorf("workMessageArchive sync cron dependencies are not configured")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	corps, err := c.store.WorkMessageArchiveEnabledCorps(ctx)
	if err != nil {
		c.logArchiveSyncResult(WorkMessageArchiveSyncResult{}, err, time.Since(startedAt), true)
		return err
	}
	result := WorkMessageArchiveSyncResult{CorpsScanned: len(corps)}
	limit := c.limit
	if limit <= 0 {
		limit = WorkMessageArchiveDefaultSyncLimit
	}
	var firstErr error
	for _, corp := range corps {
		corpResult, err := c.syncCorp(ctx, corp, limit)
		mergeWorkMessageArchiveSyncResult(&result, corpResult)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	c.logArchiveSyncResult(result, firstErr, time.Since(startedAt), true)
	return firstErr
}

func (c *WorkMessageArchiveSyncCron) RunCorp(ctx context.Context, corpID int) error {
	if c.store == nil || c.client == nil {
		return fmt.Errorf("workMessageArchive sync cron dependencies are not configured")
	}
	if corpID <= 0 {
		return fmt.Errorf("workMessageArchive sync corp id is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	corps, err := c.store.WorkMessageArchiveEnabledCorps(ctx)
	if err != nil {
		c.logArchiveSyncResult(WorkMessageArchiveSyncResult{FailedCorpID: corpID}, err, 0, false)
		return err
	}
	limit := c.limit
	if limit <= 0 {
		limit = WorkMessageArchiveDefaultSyncLimit
	}
	for _, corp := range corps {
		if corp.CorpID != corpID {
			continue
		}
		result, err := c.syncCorp(ctx, corp, limit)
		c.logArchiveSyncResult(result, err, 0, false)
		return err
	}
	c.logger.Debug("企业未启用会话存档，本次主动同步跳过", "event", "archive_sync_skipped", "component", "archive", "corp_id", corpID,
		"step", "select_corp", "result", "skipped", "error_code", "ARCHIVE_NOT_ENABLED")
	return nil
}

func (c *WorkMessageArchiveSyncCron) logArchiveSyncResult(result WorkMessageArchiveSyncResult, runErr error, elapsed time.Duration, periodic bool) {
	attrs := []any{
		"component", "archive", "step", "sync", "corps_scanned", result.CorpsScanned,
		"messages_fetched", result.MessagesFetched, "messages_inserted", result.MessagesInserted,
		"items_skipped", result.ItemsSkipped, "items_failed", result.ItemsFailed, "last_seq", result.LastSeq,
		"duration_ms", elapsed.Milliseconds(),
	}
	if result.FailedCorpID > 0 {
		attrs = append(attrs, "corp_id", result.FailedCorpID)
	}
	if runErr != nil || result.ItemsFailed > 0 {
		if !periodic {
			c.logger.Error("会话存档主动同步失败；请检查 bridge、企业凭据、游标和数据库",
				append([]any{"event", "archive_sync_failed", "result", "failed", "error_code", "ARCHIVE_SYNC_FAILED"}, attrs...)...)
			return
		}
		c.consecutiveFailures++
		now := time.Now()
		if c.lastFailureLog.IsZero() || now.Sub(c.lastFailureLog) >= time.Minute {
			c.logger.Error("会话存档同步持续失败并将自动重试；请检查 bridge、企业凭据、游标和数据库，重复日志已限流",
				append([]any{"event", "archive_sync_failed", "result", "retrying", "error_code", "ARCHIVE_SYNC_FAILED", "retry_count", c.consecutiveFailures}, attrs...)...)
			c.lastFailureLog = now
		}
		return
	}
	if periodic && c.consecutiveFailures > 0 {
		c.logger.Info("会话存档同步已从连续失败中恢复",
			append([]any{"event", "archive_sync_recovered", "result", "success", "retry_count", c.consecutiveFailures}, attrs...)...)
		c.consecutiveFailures = 0
		c.lastFailureLog = time.Time{}
	}
	if result.MessagesFetched == 0 && result.MessagesInserted == 0 {
		return
	}
	c.logger.Info("会话存档同步完成；如写入数低于拉取数请检查跳过项和数据去重",
		append([]any{"event", "archive_sync_completed", "result", "success"}, attrs...)...)
}

func (c *WorkMessageArchiveSyncCron) syncCorp(ctx context.Context, corp WorkMessageArchiveCorp, limit int) (WorkMessageArchiveSyncResult, error) {
	result := WorkMessageArchiveSyncResult{}
	if corp.CorpID <= 0 || strings.TrimSpace(corp.WXCorpID) == "" || strings.TrimSpace(corp.ChatSecret) == "" ||
		strings.TrimSpace(corp.RSAPublicKey) == "" || strings.TrimSpace(corp.RSAPrivateKey) == "" {
		result.ItemsFailed++
		result.FailedCorpID = corp.CorpID
		return result, fmt.Errorf("work message archive configuration is incomplete for corp %d", corp.CorpID)
	}
	cursor, err := c.store.WorkMessageArchiveCursor(ctx, corp.CorpID)
	if err != nil {
		result.ItemsFailed++
		result.FailedCorpID = corp.CorpID
		return result, fmt.Errorf("work message archive cursor corp=%d: %w", corp.CorpID, err)
	}
	messages, err := c.client.FetchWorkMessageArchive(ctx, corp, cursor, limit)
	if err != nil {
		result.ItemsFailed++
		result.FailedCorpID = corp.CorpID
		return result, fmt.Errorf("work message archive fetch corp=%d: %w", corp.CorpID, err)
	}
	if len(messages) == 0 {
		result.ItemsSkipped++
		return result, nil
	}
	maxSeq := cursor
	var firstErr error
	for _, message := range messages {
		if message.Seq <= cursor || message.Seq <= 0 || strings.TrimSpace(message.MsgID) == "" {
			result.ItemsSkipped++
			continue
		}
		result.MessagesFetched++
		upsert, err := c.store.UpsertWorkMessageArchive(ctx, corp.CorpID, message)
		if err != nil {
			result.ItemsFailed++
			result.FailedCorpID = corp.CorpID
			if firstErr == nil {
				firstErr = fmt.Errorf("work message archive upsert corp=%d seq=%d msgid=%s: %w", corp.CorpID, message.Seq, message.MsgID, err)
			}
			break
		}
		if upsert.Inserted {
			result.MessagesInserted++
		}
		if upsert.Skipped || !upsert.Inserted {
			result.ItemsSkipped++
		}
		if message.Seq > maxSeq {
			maxSeq = message.Seq
		}
	}
	if maxSeq > cursor {
		if err := c.store.UpdateWorkMessageArchiveCursor(ctx, corp.CorpID, maxSeq); err != nil {
			result.ItemsFailed++
			result.FailedCorpID = corp.CorpID
			if firstErr == nil {
				firstErr = fmt.Errorf("work message archive cursor update corp=%d seq=%d: %w", corp.CorpID, maxSeq, err)
			}
		}
		result.LastSeq = maxSeq
	}
	return result, firstErr
}

type WorkMessageArchiveBridgeClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewWorkMessageArchiveBridgeClient(baseURL string, token string) *WorkMessageArchiveBridgeClient {
	return &WorkMessageArchiveBridgeClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *WorkMessageArchiveBridgeClient) FetchWorkMessageArchive(ctx context.Context, corp WorkMessageArchiveCorp, seq int64, limit int) ([]WorkMessageArchiveMessage, error) {
	if strings.TrimSpace(c.baseURL) == "" {
		return nil, fmt.Errorf("work message archive bridge base url is empty")
	}
	if limit <= 0 {
		limit = WorkMessageArchiveDefaultSyncLimit
	}
	request := map[string]any{
		"corp_id":   corp.CorpID,
		"wx_corpid": corp.WXCorpID,
		"seq":       seq,
		"limit":     limit,
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/work-message/archive/messages", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("work message archive bridge returned HTTP %d", resp.StatusCode)
	}
	var response struct {
		weComBaseResponse
		Messages []json.RawMessage `json:"messages"`
		ChatData []json.RawMessage `json:"chatdata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	if err := response.Err(); err != nil {
		return nil, err
	}
	items := response.Messages
	if len(items) == 0 {
		items = response.ChatData
	}
	messages := make([]WorkMessageArchiveMessage, 0, len(items))
	for _, item := range items {
		message, err := parseWorkMessageArchiveBridgeMessage(item)
		if err != nil {
			return nil, err
		}
		if message.Seq <= seq || strings.TrimSpace(message.MsgID) == "" {
			continue
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func parseWorkMessageArchiveBridgeMessage(raw json.RawMessage) (WorkMessageArchiveMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return WorkMessageArchiveMessage{}, err
	}
	message := WorkMessageArchiveMessage{
		Seq:     jsonInt64(object, "seq"),
		MsgID:   jsonString(object, "msgid", "msg_id"),
		Action:  jsonString(object, "action"),
		From:    jsonString(object, "from", "msgfrom"),
		RoomID:  jsonString(object, "roomid", "room_id"),
		MsgType: jsonString(object, "msgtype", "msg_type"),
		RawJSON: string(raw),
	}
	message.ToList = jsonStringSlice(object, "tolist", "to_list")
	message.MsgTime = jsonTime(object, "msgtime", "msg_time")
	message.ContentRaw = workMessageArchiveContentRaw(object)
	message.ContentText = workMessageArchiveContentText(object, message.ContentRaw)
	if message.MsgID == "" {
		return WorkMessageArchiveMessage{}, fmt.Errorf("work message archive message is missing msgid")
	}
	return message, nil
}

func workMessageArchiveContentRaw(object map[string]json.RawMessage) string {
	for _, key := range []string{"content", "text", "image", "voice", "video", "file", "link", "markdown", "mixed", "location", "emotion", "meeting_voice_call", "voip_doc_share", "docmsg", "calendar", "vote", "collect", "redpacket", "card"} {
		if raw, ok := object[key]; ok && len(raw) > 0 {
			return string(raw)
		}
	}
	if raw, ok := object["message"]; ok && len(raw) > 0 {
		return string(raw)
	}
	return "{}"
}

func workMessageArchiveContentText(object map[string]json.RawMessage, contentRaw string) string {
	for _, key := range []string{"content_text", "contentText"} {
		if text := jsonString(object, key); strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	if strings.TrimSpace(contentRaw) == "" {
		return ""
	}
	var payload any
	if err := json.Unmarshal([]byte(contentRaw), &payload); err != nil {
		return strings.TrimSpace(contentRaw)
	}
	return strings.TrimSpace(sensitiveWordFlattenText(payload))
}

func jsonString(object map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := object[key]
		if !ok || len(raw) == 0 {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			return strings.TrimSpace(value)
		}
		var number json.Number
		if err := json.Unmarshal(raw, &number); err == nil {
			return number.String()
		}
	}
	return ""
}

func jsonStringSlice(object map[string]json.RawMessage, keys ...string) []string {
	for _, key := range keys {
		raw, ok := object[key]
		if !ok || len(raw) == 0 {
			continue
		}
		var values []string
		if err := json.Unmarshal(raw, &values); err == nil {
			return compactStrings(values)
		}
		value := jsonString(object, key)
		if value != "" {
			return []string{value}
		}
	}
	return nil
}

func jsonInt64(object map[string]json.RawMessage, key string) int64 {
	raw, ok := object[key]
	if !ok || len(raw) == 0 {
		return 0
	}
	var value int64
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0
	}
	parsed, _ := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	return parsed
}

func jsonTime(object map[string]json.RawMessage, keys ...string) time.Time {
	for _, key := range keys {
		raw, ok := object[key]
		if !ok || len(raw) == 0 {
			continue
		}
		var number int64
		if err := json.Unmarshal(raw, &number); err == nil {
			return unixMessageTime(number)
		}
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
				return unixMessageTime(parsed)
			}
			for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
				if parsed, err := time.ParseInLocation(layout, text, time.Local); err == nil {
					return parsed
				}
			}
		}
	}
	return time.Time{}
}

func unixMessageTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	if value > 1000000000000 {
		return time.UnixMilli(value)
	}
	return time.Unix(value, 0)
}

func compactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			compacted = append(compacted, value)
		}
	}
	return compacted
}

func mergeWorkMessageArchiveSyncResult(target *WorkMessageArchiveSyncResult, source WorkMessageArchiveSyncResult) {
	target.CorpsScanned += source.CorpsScanned
	target.MessagesFetched += source.MessagesFetched
	target.MessagesInserted += source.MessagesInserted
	target.ItemsSkipped += source.ItemsSkipped
	target.ItemsFailed += source.ItemsFailed
	if source.LastSeq > target.LastSeq {
		target.LastSeq = source.LastSeq
	}
	if target.FailedCorpID == 0 && source.FailedCorpID > 0 {
		target.FailedCorpID = source.FailedCorpID
	}
}
