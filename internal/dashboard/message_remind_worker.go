package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

type MessageRemindRecipients []string

func (r *MessageRemindRecipients) UnmarshalJSON(raw []byte) error {
	recipients, err := decodeMessageRemindRecipients(raw)
	if err != nil {
		return err
	}
	*r = recipients
	return nil
}

type MessageRemindEvent struct {
	CorpID  int                     `json:"corpId"`
	ToType  string                  `json:"toType,omitempty"`
	To      MessageRemindRecipients `json:"to,omitempty"`
	ToUser  MessageRemindRecipients `json:"toUser,omitempty"`
	ToParty MessageRemindRecipients `json:"toParty,omitempty"`
	ToTag   MessageRemindRecipients `json:"toTag,omitempty"`
	MsgType string                  `json:"msgType"`
	Content any                     `json:"content"`
	Extra   map[string]any          `json:"extra,omitempty"`
	Source  string                  `json:"source,omitempty"`
}

func (e *MessageRemindEvent) UnmarshalJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
		return e.unmarshalMessageRemindObject(raw)
	}
	return e.unmarshalMessageRemindLegacyArray(raw)
}

func (e *MessageRemindEvent) unmarshalMessageRemindObject(raw []byte) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if rawValue, ok := firstJSONField(payload, "corpId", "corp_id"); ok {
		corpID, err := decodeMessageRemindInt(rawValue)
		if err != nil {
			return fmt.Errorf("corpId: %w", err)
		}
		e.CorpID = corpID
	}
	if err := decodeOptionalJSONField(payload, &e.MsgType, "msgType", "msg_type"); err != nil {
		return fmt.Errorf("msgType: %w", err)
	}
	if err := decodeOptionalJSONField(payload, &e.ToType, "toType", "to_type"); err != nil {
		return fmt.Errorf("toType: %w", err)
	}
	if err := decodeOptionalJSONField(payload, &e.Source, "source"); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if rawValue, ok := firstJSONField(payload, "to"); ok {
		recipients, err := decodeMessageRemindRecipients(rawValue)
		if err != nil {
			return fmt.Errorf("to: %w", err)
		}
		e.To = recipients
	}
	if rawValue, ok := firstJSONField(payload, "toUser", "to_user"); ok {
		recipients, err := decodeMessageRemindRecipients(rawValue)
		if err != nil {
			return fmt.Errorf("toUser: %w", err)
		}
		e.ToUser = recipients
	}
	if rawValue, ok := firstJSONField(payload, "toParty", "to_party"); ok {
		recipients, err := decodeMessageRemindRecipients(rawValue)
		if err != nil {
			return fmt.Errorf("toParty: %w", err)
		}
		e.ToParty = recipients
	}
	if rawValue, ok := firstJSONField(payload, "toTag", "to_tag"); ok {
		recipients, err := decodeMessageRemindRecipients(rawValue)
		if err != nil {
			return fmt.Errorf("toTag: %w", err)
		}
		e.ToTag = recipients
	}
	if rawValue, ok := firstJSONField(payload, "content"); ok {
		content, err := decodeMessageRemindAny(rawValue)
		if err != nil {
			return fmt.Errorf("content: %w", err)
		}
		e.Content = content
	}
	if rawValue, ok := firstJSONField(payload, "extra"); ok && strings.TrimSpace(string(rawValue)) != "null" {
		extra, err := decodeMessageRemindAny(rawValue)
		if err != nil {
			return fmt.Errorf("extra: %w", err)
		}
		if extraMap, ok := extra.(map[string]any); ok {
			e.Extra = extraMap
		} else {
			return fmt.Errorf("extra must be object")
		}
	}
	return nil
}

func (e *MessageRemindEvent) unmarshalMessageRemindLegacyArray(raw []byte) error {
	var legacy []json.RawMessage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if len(legacy) < 4 {
		return fmt.Errorf("message remind legacy payload requires at least 4 fields")
	}
	corpID, err := decodeMessageRemindInt(legacy[0])
	if err != nil {
		return fmt.Errorf("corpId: %w", err)
	}
	e.CorpID = corpID
	offset := 1
	if len(legacy) >= 6 {
		var toType string
		if err := json.Unmarshal(legacy[1], &toType); err == nil && validMessageRemindToType(toType) {
			e.ToType = toType
			offset = 2
		}
	}
	recipients, err := decodeMessageRemindRecipients(legacy[offset])
	if err != nil {
		return fmt.Errorf("to: %w", err)
	}
	if offset == 1 {
		e.ToUser = recipients
	} else {
		e.To = recipients
	}
	if len(legacy) <= offset+2 {
		return fmt.Errorf("message remind legacy payload requires msgType and content")
	}
	if err := json.Unmarshal(legacy[offset+1], &e.MsgType); err != nil {
		return fmt.Errorf("msgType: %w", err)
	}
	e.Content, err = decodeMessageRemindAny(legacy[offset+2])
	if err != nil {
		return fmt.Errorf("content: %w", err)
	}
	if len(legacy) > offset+3 && strings.TrimSpace(string(legacy[offset+3])) != "null" {
		extra, err := decodeMessageRemindAny(legacy[offset+3])
		if err != nil {
			return fmt.Errorf("extra: %w", err)
		}
		if extraMap, ok := extra.(map[string]any); ok {
			e.Extra = extraMap
		} else {
			return fmt.Errorf("extra must be object")
		}
	}
	return nil
}

func (e MessageRemindEvent) recipientTarget() (string, []string, error) {
	if recipients := normalizeMessageRemindRecipients(e.ToUser); len(recipients) > 0 {
		return "user", recipients, nil
	}
	if recipients := normalizeMessageRemindRecipients(e.ToParty); len(recipients) > 0 {
		return "party", recipients, nil
	}
	if recipients := normalizeMessageRemindRecipients(e.ToTag); len(recipients) > 0 {
		return "tag", recipients, nil
	}
	toType := strings.ToLower(strings.TrimSpace(e.ToType))
	if toType == "" {
		return "", nil, fmt.Errorf("missing recipient")
	}
	if !validMessageRemindToType(toType) {
		return "", nil, fmt.Errorf("unsupported recipient type %q", e.ToType)
	}
	recipients := normalizeMessageRemindRecipients(e.To)
	if len(recipients) == 0 {
		return "", nil, fmt.Errorf("missing recipient")
	}
	return toType, recipients, nil
}

type MessageRemindDelivery struct {
	Event    MessageRemindEvent
	Raw      string
	Attempts int
}

type WorkAgentMessagePayload struct {
	ToUser  string
	ToParty string
	ToTag   string
	MsgType string
	Content any
	Extra   map[string]any
}

type MessageRemindWorkerQueue interface {
	DequeueMessageRemind(ctx context.Context, timeout time.Duration) (MessageRemindDelivery, bool, error)
	AckMessageRemind(ctx context.Context, delivery MessageRemindDelivery) error
	RetryMessageRemind(ctx context.Context, delivery MessageRemindDelivery, reason string, maxAttempts int) (bool, error)
	RecoverMessageRemindProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error)
}

type MessageRemindWorkerStore interface {
	RoomTagPullRemindAgentByCorpID(ctx context.Context, corpID int) (RoomTagPullAgentCredential, bool, error)
	MediumCorpCredentialByID(ctx context.Context, corpID int) (MediumCorpCredential, bool, error)
}

type MessageRemindWorkerClient interface {
	SendAgentMessage(ctx context.Context, credential RoomTagPullAgentCredential, payload WorkAgentMessagePayload) error
	UploadTemporaryMedia(ctx context.Context, credential MediumCorpCredential, mediaType string, filePath string) (string, error)
}

type MessageRemindWorker struct {
	queue             MessageRemindWorkerQueue
	store             MessageRemindWorkerStore
	client            MessageRemindWorkerClient
	fileStorageRoot   string
	pollTimeout       time.Duration
	maxAttempts       int
	processingTimeout time.Duration
	recoveryInterval  time.Duration
	alertNotifier     SaaSAlertNotifier
	logger            *log.Logger
}

func NewMessageRemindWorker(queue MessageRemindWorkerQueue, store MessageRemindWorkerStore, client MessageRemindWorkerClient, fileStorageRoot string, logger *log.Logger) *MessageRemindWorker {
	if logger == nil {
		logger = log.Default()
	}
	if client == nil {
		client = NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL)
	}
	fileStorageRoot = strings.TrimSpace(fileStorageRoot)
	if fileStorageRoot == "" {
		fileStorageRoot = defaultMediumFileStorageRoot
	}
	return &MessageRemindWorker{
		queue:             queue,
		store:             store,
		client:            client,
		fileStorageRoot:   fileStorageRoot,
		pollTimeout:       5 * time.Second,
		maxAttempts:       3,
		processingTimeout: 5 * time.Minute,
		recoveryInterval:  time.Minute,
		logger:            logger,
	}
}

func (w *MessageRemindWorker) WithProcessingTimeout(timeout time.Duration) *MessageRemindWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
		w.recoveryInterval = timeout
		if w.recoveryInterval > time.Minute {
			w.recoveryInterval = time.Minute
		}
	}
	return w
}

func (w *MessageRemindWorker) WithFileStorageRoot(fileStorageRoot string) *MessageRemindWorker {
	if strings.TrimSpace(fileStorageRoot) != "" {
		w.fileStorageRoot = strings.TrimSpace(fileStorageRoot)
	}
	return w
}

func (w *MessageRemindWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *MessageRemindWorker {
	w.alertNotifier = notifier
	return w
}

func (w *MessageRemindWorker) Run(ctx context.Context) error {
	if w.queue == nil || w.store == nil || w.client == nil {
		return fmt.Errorf("message remind worker dependencies are not configured")
	}
	nextRecovery := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !time.Now().Before(nextRecovery) {
			w.recoverProcessing(ctx)
			nextRecovery = time.Now().Add(w.recoveryInterval)
		}
		delivery, ok, err := w.queue.DequeueMessageRemind(ctx, w.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logger.Printf("message remind dequeue failed: %v", err)
			continue
		}
		if !ok {
			continue
		}
		w.handleDelivery(ctx, delivery)
	}
}

func (w *MessageRemindWorker) recoverProcessing(ctx context.Context) {
	recovered, err := w.queue.RecoverMessageRemindProcessing(ctx, w.processingTimeout, w.maxAttempts)
	if err != nil {
		w.logger.Printf("message remind processing recovery failed: %v", err)
		return
	}
	if recovered > 0 {
		w.logger.Printf("message remind recovered processing jobs: %d", recovered)
	}
}

func (w *MessageRemindWorker) handleDelivery(ctx context.Context, delivery MessageRemindDelivery) {
	ctx = WithSaaSAlertNotifier(ctx, w.alertNotifier)
	tenantID := tenantIDForQueueExecution(ctx, w.logger, w.store, delivery.Event.CorpID)
	finishExecution := startQueueItemExecution(ctx, w.logger, QueueNameMessageRemind, w.store, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		deadLettered, retryErr := w.queue.RetryMessageRemind(ctx, delivery, err.Error(), w.maxAttempts)
		if retryErr != nil {
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; retry failed: %v", err, retryErr))
			w.logger.Printf("message remind retry failed: corp_id=%d msg_type=%s source=%s err=%v retry_err=%v", delivery.Event.CorpID, delivery.Event.MsgType, delivery.Event.Source, err, retryErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, err)
		if deadLettered {
			w.logger.Printf("message remind moved to dead letter: corp_id=%d msg_type=%s source=%s attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.MsgType, delivery.Event.Source, delivery.Attempts+1, err)
			return
		}
		w.logger.Printf("message remind requeued: corp_id=%d msg_type=%s source=%s attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.MsgType, delivery.Event.Source, delivery.Attempts+1, err)
		return
	}
	if err := w.queue.AckMessageRemind(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("ack failed: %w", err))
		w.logger.Printf("message remind ack failed: corp_id=%d msg_type=%s source=%s err=%v", delivery.Event.CorpID, delivery.Event.MsgType, delivery.Event.Source, err)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func (w *MessageRemindWorker) Process(ctx context.Context, event MessageRemindEvent) error {
	if event.CorpID <= 0 {
		return fmt.Errorf("missing corp id")
	}
	msgType := strings.TrimSpace(event.MsgType)
	if msgType == "" {
		return fmt.Errorf("missing message type")
	}
	toType, recipients, err := event.recipientTarget()
	if err != nil {
		return err
	}
	content, err := w.prepareContent(ctx, event.CorpID, msgType, event.Content)
	if err != nil {
		return err
	}
	credential, found, err := w.store.RoomTagPullRemindAgentByCorpID(ctx, event.CorpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.WXAgentID) == "" || strings.TrimSpace(credential.WXSecret) == "" {
		return fmt.Errorf("corp %d remind agent is not configured", event.CorpID)
	}
	payload := WorkAgentMessagePayload{
		MsgType: msgType,
		Content: content,
		Extra:   event.Extra,
	}
	switch toType {
	case "user":
		payload.ToUser = strings.Join(recipients, "|")
	case "party":
		payload.ToParty = strings.Join(recipients, "|")
	case "tag":
		payload.ToTag = strings.Join(recipients, "|")
	default:
		return fmt.Errorf("unsupported recipient type %q", toType)
	}
	return w.client.SendAgentMessage(ctx, credential, payload)
}

func (w *MessageRemindWorker) prepareContent(ctx context.Context, corpID int, msgType string, content any) (any, error) {
	if content == nil {
		return nil, fmt.Errorf("missing message content")
	}
	if raw, ok := content.(json.RawMessage); ok {
		decoded, err := decodeMessageRemindAny(raw)
		if err != nil {
			return nil, err
		}
		content = decoded
	}
	switch value := content.(type) {
	case map[string]any:
		return w.prepareMediaContent(ctx, corpID, msgType, value)
	default:
		return content, nil
	}
}

func (w *MessageRemindWorker) prepareMediaContent(ctx context.Context, corpID int, msgType string, content map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(content)+1)
	for key, value := range content {
		out[key] = value
	}
	if strings.TrimSpace(messageRemindStringValue(out["media_id"])) != "" {
		return out, nil
	}
	path := strings.TrimSpace(messageRemindStringValue(out["path"]))
	if path == "" {
		return out, nil
	}
	mediaType := strings.TrimSpace(msgType)
	if mediaType == "" {
		return nil, fmt.Errorf("missing media type")
	}
	localPath := mediumLocalFilePath(w.fileStorageRoot, path)
	info, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("message media path %q is not readable: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("message media path %q is a directory", path)
	}
	credential, found, err := w.store.MediumCorpCredentialByID(ctx, corpID)
	if err != nil {
		return nil, err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.EmployeeSecret) == "" {
		return nil, fmt.Errorf("corp %d employee credential is not configured", corpID)
	}
	mediaID, err := w.client.UploadTemporaryMedia(ctx, credential, mediaType, localPath)
	if err != nil {
		return nil, err
	}
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return nil, fmt.Errorf("企业微信接口未返回 media_id")
	}
	out["media_id"] = mediaID
	delete(out, "path")
	return out, nil
}

func decodeOptionalJSONField(payload map[string]json.RawMessage, target any, keys ...string) error {
	raw, ok := firstJSONField(payload, keys...)
	if !ok {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func firstJSONField(payload map[string]json.RawMessage, keys ...string) (json.RawMessage, bool) {
	for _, key := range keys {
		if raw, ok := payload[key]; ok {
			return raw, true
		}
	}
	return nil, false
}

func decodeMessageRemindAny(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func decodeMessageRemindInt(raw []byte) (int, error) {
	value, err := decodeMessageRemindAny(raw)
	if err != nil {
		return 0, err
	}
	switch typed := value.(type) {
	case json.Number:
		integer, err := typed.Int64()
		if err != nil {
			return 0, err
		}
		return int(integer), nil
	case string:
		integer, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, err
		}
		return integer, nil
	case float64:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("unsupported integer value %T", value)
	}
}

func decodeMessageRemindRecipients(raw []byte) (MessageRemindRecipients, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	value, err := decodeMessageRemindAny(raw)
	if err != nil {
		return nil, err
	}
	switch typed := value.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			value := strings.TrimSpace(messageRemindStringValue(item))
			if value != "" {
				out = append(out, value)
			}
		}
		return out, nil
	default:
		value := strings.TrimSpace(messageRemindStringValue(typed))
		if value == "" {
			return nil, nil
		}
		return MessageRemindRecipients{value}, nil
	}
}

func normalizeMessageRemindRecipients(recipients []string) []string {
	out := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		recipient = strings.TrimSpace(recipient)
		if recipient != "" {
			out = append(out, recipient)
		}
	}
	return out
}

func validMessageRemindToType(toType string) bool {
	switch strings.ToLower(strings.TrimSpace(toType)) {
	case "user", "party", "tag":
		return true
	default:
		return false
	}
}

func messageRemindStringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	default:
		return fmt.Sprintf("%v", typed)
	}
}
