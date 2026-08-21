package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	SensitiveWordMonitorMessageTableCount      = 10
	SensitiveWordMonitorCursorTypeBase         = 20
	SensitiveWordMonitorDefaultMessagesPerTick = 200
)

type SensitiveWordCronWord struct {
	ID     int
	CorpID int
	Name   string
}

type SensitiveWordArchivedMessage struct {
	TableIndex     int
	ID             int
	CorpID         int
	WorkEmployeeID int
	ToUserType     int
	ToUserID       int
	SenderType     int
	MsgType        int
	ContentRaw     string
	ContentText    string
	RoomID         int
	MsgDataTime    time.Time
	SenderName     string
	TargetName     string
	RoomName       string
}

type SensitiveWordMonitorCreate struct {
	CorpID            int
	SensitiveWordID   int
	SensitiveWordName string
	Source            int
	TriggerUserID     int
	TriggerName       string
	WorkRoomID        int
	TriggerScenario   string
	Sender            string
	MsgType           int
	SendTime          time.Time
	ContentRaw        string
	ConversationJSON  string
}

type SensitiveWordMonitorCronResult struct {
	WordsScanned     int
	CorpsScanned     int
	TablesScanned    int
	MessagesScanned  int
	MonitorsInserted int
	ItemsSkipped     int
	ItemsFailed      int
}

type SensitiveWordMonitorCronStore interface {
	ActiveSensitiveWords(ctx context.Context) ([]SensitiveWordCronWord, error)
	SensitiveWordMessageCursor(ctx context.Context, corpID int, tableIndex int) (int, error)
	PendingSensitiveWordMessages(ctx context.Context, corpID int, tableIndex int, afterID int, limit int) ([]SensitiveWordArchivedMessage, error)
	InsertSensitiveWordMonitor(ctx context.Context, item SensitiveWordMonitorCreate) (bool, error)
	UpdateSensitiveWordMessageCursor(ctx context.Context, corpID int, tableIndex int, lastID int) error
}

type SensitiveWordMonitorStatusWriter interface {
	RecordSensitiveWordScanStarted(context.Context, int) error
	RecordSensitiveWordScanFinished(context.Context, int, error) error
}

type SensitiveWordMonitorCron struct {
	store           SensitiveWordMonitorCronStore
	logger          *log.Logger
	now             func() time.Time
	messagesPerTick int
}

func NewSensitiveWordMonitorCron(store SensitiveWordMonitorCronStore, logger *log.Logger) *SensitiveWordMonitorCron {
	if logger == nil {
		logger = log.Default()
	}
	return &SensitiveWordMonitorCron{
		store:           store,
		logger:          logger,
		now:             time.Now,
		messagesPerTick: SensitiveWordMonitorDefaultMessagesPerTick,
	}
}

func (c *SensitiveWordMonitorCron) RunOnce(ctx context.Context) error {
	if c.store == nil {
		return fmt.Errorf("sensitiveWordsMonitor cron dependencies are not configured")
	}
	words, err := c.store.ActiveSensitiveWords(ctx)
	if err != nil {
		return err
	}
	grouped := sensitiveWordsByCorp(words)
	result := SensitiveWordMonitorCronResult{WordsScanned: len(words), CorpsScanned: len(grouped)}
	limit := c.messagesPerTick
	if limit <= 0 {
		limit = SensitiveWordMonitorDefaultMessagesPerTick
	}
	var firstErr error
	for corpID, corpWords := range grouped {
		statusWriter, hasStatus := c.store.(SensitiveWordMonitorStatusWriter)
		if hasStatus {
			if statusErr := statusWriter.RecordSensitiveWordScanStarted(ctx, corpID); statusErr != nil && firstErr == nil {
				firstErr = statusErr
			}
		}
		var corpErr error
		for tableIndex := 1; tableIndex <= SensitiveWordMonitorMessageTableCount; tableIndex++ {
			tableResult, err := c.scanTable(ctx, corpID, tableIndex, corpWords, limit)
			mergeSensitiveWordMonitorCronResult(&result, tableResult)
			if err != nil {
				if corpErr == nil {
					corpErr = err
				}
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		if hasStatus {
			if statusErr := statusWriter.RecordSensitiveWordScanFinished(ctx, corpID, corpErr); statusErr != nil && firstErr == nil {
				firstErr = statusErr
			}
		}
	}
	c.logger.Printf("sensitiveWordsMonitor cron finished: words=%d corps=%d tables=%d messages=%d monitors_inserted=%d skipped=%d failed=%d", result.WordsScanned, result.CorpsScanned, result.TablesScanned, result.MessagesScanned, result.MonitorsInserted, result.ItemsSkipped, result.ItemsFailed)
	return firstErr
}

func (c *SensitiveWordMonitorCron) scanTable(ctx context.Context, corpID int, tableIndex int, words []SensitiveWordCronWord, limit int) (SensitiveWordMonitorCronResult, error) {
	result := SensitiveWordMonitorCronResult{TablesScanned: 1}
	cursor, err := c.store.SensitiveWordMessageCursor(ctx, corpID, tableIndex)
	if err != nil {
		result.ItemsFailed++
		return result, fmt.Errorf("sensitive words cursor corp=%d table=%d: %w", corpID, tableIndex, err)
	}
	messages, err := c.store.PendingSensitiveWordMessages(ctx, corpID, tableIndex, cursor, limit)
	if err != nil {
		result.ItemsFailed++
		return result, fmt.Errorf("sensitive words messages corp=%d table=%d: %w", corpID, tableIndex, err)
	}
	if len(messages) == 0 {
		result.ItemsSkipped++
		return result, nil
	}
	maxProcessedID := cursor
	var firstErr error
	for _, message := range messages {
		if message.ID <= cursor {
			result.ItemsSkipped++
			continue
		}
		result.MessagesScanned++
		text := sensitiveWordArchivedMessageText(message)
		if strings.TrimSpace(text) == "" {
			maxProcessedID = message.ID
			result.ItemsSkipped++
			continue
		}
		matches := sensitiveWordMatches(text, words)
		if len(matches) == 0 {
			maxProcessedID = message.ID
			result.ItemsSkipped++
			continue
		}
		messageHadError := false
		for _, word := range matches {
			item := sensitiveWordMonitorCreate(word, message, text, c.now())
			inserted, err := c.store.InsertSensitiveWordMonitor(ctx, item)
			if err != nil {
				result.ItemsFailed++
				messageHadError = true
				if firstErr == nil {
					firstErr = fmt.Errorf("sensitive words monitor insert corp=%d table=%d message=%d word=%d: %w", corpID, tableIndex, message.ID, word.ID, err)
				}
				continue
			}
			if inserted {
				result.MonitorsInserted++
			}
		}
		if messageHadError {
			break
		}
		maxProcessedID = message.ID
	}
	if maxProcessedID > cursor {
		if err := c.store.UpdateSensitiveWordMessageCursor(ctx, corpID, tableIndex, maxProcessedID); err != nil {
			result.ItemsFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("sensitive words cursor update corp=%d table=%d last_id=%d: %w", corpID, tableIndex, maxProcessedID, err)
			}
		}
	}
	return result, firstErr
}

func sensitiveWordsByCorp(words []SensitiveWordCronWord) map[int][]SensitiveWordCronWord {
	grouped := map[int][]SensitiveWordCronWord{}
	for _, word := range words {
		word.Name = strings.TrimSpace(word.Name)
		if word.ID <= 0 || word.CorpID <= 0 || word.Name == "" {
			continue
		}
		grouped[word.CorpID] = append(grouped[word.CorpID], word)
	}
	return grouped
}

func sensitiveWordMatches(text string, words []SensitiveWordCronWord) []SensitiveWordCronWord {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}
	matches := make([]SensitiveWordCronWord, 0)
	seen := map[int]struct{}{}
	for _, word := range words {
		name := strings.ToLower(strings.TrimSpace(word.Name))
		if name == "" {
			continue
		}
		if strings.Contains(text, name) {
			if _, ok := seen[word.ID]; ok {
				continue
			}
			seen[word.ID] = struct{}{}
			matches = append(matches, word)
		}
	}
	return matches
}

func sensitiveWordArchivedMessageText(message SensitiveWordArchivedMessage) string {
	if strings.TrimSpace(message.ContentText) != "" {
		return strings.TrimSpace(message.ContentText)
	}
	return strings.TrimSpace(sensitiveWordTextFromJSON(message.ContentRaw))
}

func sensitiveWordTextFromJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return raw
	}
	return strings.TrimSpace(sensitiveWordFlattenText(payload))
}

func sensitiveWordFlattenText(payload any) string {
	switch value := payload.(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := sensitiveWordFlattenText(item); strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	case map[string]any:
		preferredKeys := []string{"content", "text", "title", "description", "desc", "name", "file_name", "filename"}
		parts := make([]string, 0, len(value))
		for _, key := range preferredKeys {
			if item, ok := value[key]; ok {
				if text := sensitiveWordFlattenText(item); strings.TrimSpace(text) != "" {
					parts = append(parts, text)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
		for _, item := range value {
			if text := sensitiveWordFlattenText(item); strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

func sensitiveWordMonitorCreate(word SensitiveWordCronWord, message SensitiveWordArchivedMessage, text string, now time.Time) SensitiveWordMonitorCreate {
	source, triggerUserID, triggerName, sender := sensitiveWordTriggerIdentity(message)
	roomID := message.RoomID
	if roomID <= 0 && message.ToUserType == 2 {
		roomID = message.ToUserID
	}
	scenario := sensitiveWordTriggerScenario(message, roomID)
	sendTime := message.MsgDataTime
	if sendTime.IsZero() {
		sendTime = now
	}
	contentRaw := strings.TrimSpace(message.ContentRaw)
	if contentRaw == "" {
		contentRaw = sensitiveWordContentJSON(text)
	}
	conversationJSON := sensitiveWordConversationJSON(message, sender, sendTime, contentRaw, text)
	return SensitiveWordMonitorCreate{
		CorpID:            message.CorpID,
		SensitiveWordID:   word.ID,
		SensitiveWordName: word.Name,
		Source:            source,
		TriggerUserID:     triggerUserID,
		TriggerName:       triggerName,
		WorkRoomID:        roomID,
		TriggerScenario:   scenario,
		Sender:            sender,
		MsgType:           message.MsgType,
		SendTime:          sendTime,
		ContentRaw:        contentRaw,
		ConversationJSON:  conversationJSON,
	}
}

func sensitiveWordTriggerIdentity(message SensitiveWordArchivedMessage) (source int, triggerUserID int, triggerName string, sender string) {
	if message.SenderType == 0 {
		triggerName = firstNonBlank(message.SenderName, "员工")
		return 2, message.WorkEmployeeID, triggerName, triggerName
	}
	switch message.ToUserType {
	case 0:
		triggerName = firstNonBlank(message.TargetName, "员工")
		return 2, message.ToUserID, triggerName, triggerName
	case 1:
		triggerName = firstNonBlank(message.TargetName, "客户")
		return 1, message.ToUserID, triggerName, triggerName
	case 2:
		triggerName = firstNonBlank(message.TargetName, message.RoomName, "客户群成员")
		return 1, 0, triggerName, triggerName
	default:
		triggerName = firstNonBlank(message.TargetName, "客户")
		return 1, message.ToUserID, triggerName, triggerName
	}
}

func sensitiveWordTriggerScenario(message SensitiveWordArchivedMessage, roomID int) string {
	if roomID > 0 {
		return firstNonBlank(message.RoomName, message.TargetName, "客户群")
	}
	switch message.ToUserType {
	case 0:
		return "内部会话"
	case 1:
		return "客户会话"
	case 2:
		return firstNonBlank(message.TargetName, "客户群")
	default:
		return "会话存档"
	}
}

func sensitiveWordConversationJSON(message SensitiveWordArchivedMessage, sender string, sendTime time.Time, contentRaw string, text string) string {
	payload := []map[string]any{{
		"sender":                sender,
		"msgType":               message.MsgType,
		"sendTime":              formatCronTime(sendTime),
		"isTrigger":             1,
		"msgContent":            sensitiveWordMessageContentPayload(contentRaw, text),
		"workMessageTableIndex": message.TableIndex,
		"workMessageId":         message.ID,
	}}
	data, err := json.Marshal(payload)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func sensitiveWordMessageContentPayload(raw string, fallbackText string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{"content": fallbackText}
	}
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err == nil {
		return payload
	}
	return map[string]any{"content": raw}
}

func sensitiveWordContentJSON(text string) string {
	data, err := json.Marshal(map[string]string{"content": text})
	if err != nil {
		return `{"content":""}`
	}
	return string(data)
}

func formatCronTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mergeSensitiveWordMonitorCronResult(target *SensitiveWordMonitorCronResult, source SensitiveWordMonitorCronResult) {
	target.WordsScanned += source.WordsScanned
	target.CorpsScanned += source.CorpsScanned
	target.TablesScanned += source.TablesScanned
	target.MessagesScanned += source.MessagesScanned
	target.MonitorsInserted += source.MonitorsInserted
	target.ItemsSkipped += source.ItemsSkipped
	target.ItemsFailed += source.ItemsFailed
}
