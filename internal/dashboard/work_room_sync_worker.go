package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

type WorkRoomSyncEvent struct {
	CorpID   int    `json:"corpId,omitempty"`
	WXCorpID string `json:"wxCorpId,omitempty"`
	ChatID   string `json:"chatId,omitempty"`
	Source   string `json:"source,omitempty"`
}

func (e *WorkRoomSyncEvent) UnmarshalJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
		return e.unmarshalWorkRoomSyncObject(raw)
	}
	return e.unmarshalWorkRoomSyncLegacyArray(raw)
}

func (e *WorkRoomSyncEvent) unmarshalWorkRoomSyncObject(raw []byte) error {
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
	if err := decodeOptionalJSONField(payload, &e.WXCorpID, "wxCorpId", "wx_corpid", "ToUserName", "toUserName", "to_user_name"); err != nil {
		return fmt.Errorf("wxCorpId: %w", err)
	}
	if err := decodeOptionalJSONField(payload, &e.ChatID, "chatId", "chat_id", "ChatId", "ChatID"); err != nil {
		return fmt.Errorf("chatId: %w", err)
	}
	if err := decodeOptionalJSONField(payload, &e.Source, "source"); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	return nil
}

func (e *WorkRoomSyncEvent) unmarshalWorkRoomSyncLegacyArray(raw []byte) error {
	var legacy []json.RawMessage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if len(legacy) < 1 {
		return fmt.Errorf("work room sync legacy payload requires at least 1 field")
	}
	trimmed := strings.TrimSpace(string(legacy[0]))
	if strings.HasPrefix(trimmed, "{") {
		return e.unmarshalWorkRoomSyncObject(legacy[0])
	}
	corpID, err := decodeMessageRemindInt(legacy[0])
	if err != nil {
		return fmt.Errorf("corpId: %w", err)
	}
	e.CorpID = corpID
	if len(legacy) > 1 {
		if err := json.Unmarshal(legacy[1], &e.ChatID); err != nil {
			return fmt.Errorf("chatId: %w", err)
		}
	}
	return nil
}

type WorkRoomSyncDelivery struct {
	Event    WorkRoomSyncEvent
	Raw      string
	Attempts int
}

type WorkRoomSyncWorkerQueue interface {
	DequeueWorkRoomSync(ctx context.Context, timeout time.Duration) (WorkRoomSyncDelivery, bool, error)
	AckWorkRoomSync(ctx context.Context, delivery WorkRoomSyncDelivery) error
	RetryWorkRoomSync(ctx context.Context, delivery WorkRoomSyncDelivery, reason string, maxAttempts int) (bool, error)
	RecoverWorkRoomSyncProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error)
}

type WorkRoomSyncWorkerStore interface {
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	RoomWelcomeCorpCredentialByWXCorpID(ctx context.Context, wxCorpID string) (RoomWelcomeCorpCredential, bool, error)
	SyncWorkRooms(ctx context.Context, corpID int, rooms []WorkRoomSyncRoom) (WorkRoomSyncResult, error)
	SyncWorkRoom(ctx context.Context, corpID int, room WorkRoomSyncRoom) (WorkRoomSyncResult, error)
}

type WorkRoomSyncWorker struct {
	queue             WorkRoomSyncWorkerQueue
	store             WorkRoomSyncWorkerStore
	client            WorkRoomSyncClient
	pollTimeout       time.Duration
	maxAttempts       int
	processingTimeout time.Duration
	recoveryInterval  time.Duration
	alertNotifier     SaaSAlertNotifier
	logger            *log.Logger
}

func NewWorkRoomSyncWorker(queue WorkRoomSyncWorkerQueue, store WorkRoomSyncWorkerStore, client WorkRoomSyncClient, logger *log.Logger) *WorkRoomSyncWorker {
	if logger == nil {
		logger = log.Default()
	}
	if client == nil {
		client = NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL)
	}
	return &WorkRoomSyncWorker{
		queue:             queue,
		store:             store,
		client:            client,
		pollTimeout:       5 * time.Second,
		maxAttempts:       3,
		processingTimeout: 5 * time.Minute,
		recoveryInterval:  time.Minute,
		logger:            logger,
	}
}

func (w *WorkRoomSyncWorker) WithProcessingTimeout(timeout time.Duration) *WorkRoomSyncWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
		w.recoveryInterval = timeout
		if w.recoveryInterval > time.Minute {
			w.recoveryInterval = time.Minute
		}
	}
	return w
}

func (w *WorkRoomSyncWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *WorkRoomSyncWorker {
	w.alertNotifier = notifier
	return w
}

func (w *WorkRoomSyncWorker) Run(ctx context.Context) error {
	if w.queue == nil || w.store == nil || w.client == nil {
		return fmt.Errorf("work room sync worker dependencies are not configured")
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
		delivery, ok, err := w.queue.DequeueWorkRoomSync(ctx, w.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logger.Printf("work room sync dequeue failed: %v", err)
			continue
		}
		if !ok {
			continue
		}
		w.handleDelivery(ctx, delivery)
	}
}

func (w *WorkRoomSyncWorker) recoverProcessing(ctx context.Context) {
	recovered, err := w.queue.RecoverWorkRoomSyncProcessing(ctx, w.processingTimeout, w.maxAttempts)
	if err != nil {
		w.logger.Printf("work room sync processing recovery failed: %v", err)
		return
	}
	if recovered > 0 {
		w.logger.Printf("work room sync recovered processing jobs: %d", recovered)
	}
}

func (w *WorkRoomSyncWorker) handleDelivery(ctx context.Context, delivery WorkRoomSyncDelivery) {
	ctx = WithSaaSAlertNotifier(ctx, w.alertNotifier)
	corpIDForExecution := delivery.Event.CorpID
	if corpIDForExecution <= 0 {
		if credential, err := w.credential(ctx, delivery.Event); err == nil {
			corpIDForExecution = credential.CorpID
		}
	}
	tenantID := tenantIDForQueueExecution(ctx, w.logger, w.store, corpIDForExecution)
	finishExecution := startQueueItemExecution(ctx, w.logger, QueueNameWorkRoomSync, w.store, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		deadLettered, retryErr := w.queue.RetryWorkRoomSync(ctx, delivery, err.Error(), w.maxAttempts)
		if retryErr != nil {
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; retry failed: %v", err, retryErr))
			w.logger.Printf("work room sync retry failed: corp_id=%d wx_corpid=%s chat_id=%s source=%s err=%v retry_err=%v", delivery.Event.CorpID, delivery.Event.WXCorpID, delivery.Event.ChatID, delivery.Event.Source, err, retryErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, err)
		if deadLettered {
			w.logger.Printf("work room sync moved to dead letter: corp_id=%d wx_corpid=%s chat_id=%s source=%s attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.WXCorpID, delivery.Event.ChatID, delivery.Event.Source, delivery.Attempts+1, err)
			return
		}
		w.logger.Printf("work room sync requeued: corp_id=%d wx_corpid=%s chat_id=%s source=%s attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.WXCorpID, delivery.Event.ChatID, delivery.Event.Source, delivery.Attempts+1, err)
		return
	}
	if err := w.queue.AckWorkRoomSync(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("ack failed: %w", err))
		w.logger.Printf("work room sync ack failed: corp_id=%d wx_corpid=%s chat_id=%s source=%s err=%v", delivery.Event.CorpID, delivery.Event.WXCorpID, delivery.Event.ChatID, delivery.Event.Source, err)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func (w *WorkRoomSyncWorker) Process(ctx context.Context, event WorkRoomSyncEvent) error {
	credential, err := w.credential(ctx, event)
	if err != nil {
		return err
	}
	if strings.TrimSpace(event.ChatID) != "" {
		return syncSingleWorkRoomFromWeCom(ctx, w.store, w.client, credential, strings.TrimSpace(event.ChatID))
	}
	return syncAllWorkRoomsFromWeCom(ctx, w.store, w.client, credential)
}

func (w *WorkRoomSyncWorker) credential(ctx context.Context, event WorkRoomSyncEvent) (RoomWelcomeCorpCredential, error) {
	if event.CorpID > 0 {
		credential, found, err := w.store.RoomWelcomeCorpCredentialByID(ctx, event.CorpID)
		if err != nil {
			return RoomWelcomeCorpCredential{}, err
		}
		if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
			return RoomWelcomeCorpCredential{}, fmt.Errorf("corp %d contact credential is not configured", event.CorpID)
		}
		return credential, nil
	}
	wxCorpID := strings.TrimSpace(event.WXCorpID)
	if wxCorpID == "" {
		return RoomWelcomeCorpCredential{}, fmt.Errorf("missing corp id or wx corpid")
	}
	credential, found, err := w.store.RoomWelcomeCorpCredentialByWXCorpID(ctx, wxCorpID)
	if err != nil {
		return RoomWelcomeCorpCredential{}, err
	}
	if !found || credential.CorpID <= 0 || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return RoomWelcomeCorpCredential{}, fmt.Errorf("wx corpid %s contact credential is not configured", wxCorpID)
	}
	return credential, nil
}

func syncAllWorkRoomsFromWeCom(ctx context.Context, store interface {
	SyncWorkRooms(context.Context, int, []WorkRoomSyncRoom) (WorkRoomSyncResult, error)
}, client WorkRoomSyncClient, credential RoomWelcomeCorpCredential) error {
	groupChats, err := client.GroupChats(ctx, credential)
	if err != nil {
		return err
	}
	groupChats = uniqueWorkRoomSyncGroupChats(groupChats)
	if len(groupChats) == 0 {
		return nil
	}
	rooms := make([]WorkRoomSyncRoom, 0, len(groupChats))
	for _, groupChat := range groupChats {
		room, err := client.GroupChatDetail(ctx, credential, groupChat.WXChatID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(room.WXChatID) == "" {
			room.WXChatID = groupChat.WXChatID
		}
		room.Status = groupChat.Status
		if strings.TrimSpace(room.WXChatID) != "" {
			rooms = append(rooms, room)
		}
	}
	_, err = store.SyncWorkRooms(ctx, credential.CorpID, rooms)
	return err
}

func syncSingleWorkRoomFromWeCom(ctx context.Context, store interface {
	SyncWorkRoom(context.Context, int, WorkRoomSyncRoom) (WorkRoomSyncResult, error)
}, client WorkRoomSyncClient, credential RoomWelcomeCorpCredential, wxChatID string) error {
	wxChatID = strings.TrimSpace(wxChatID)
	if wxChatID == "" {
		return nil
	}
	room, err := client.GroupChatDetail(ctx, credential, wxChatID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(room.WXChatID) == "" {
		room.WXChatID = wxChatID
	}
	groupChats, err := client.GroupChats(ctx, credential)
	if err != nil {
		return err
	}
	for _, groupChat := range groupChats {
		if strings.TrimSpace(groupChat.WXChatID) == wxChatID {
			room.Status = groupChat.Status
			break
		}
	}
	if strings.TrimSpace(room.WXChatID) == "" {
		return nil
	}
	_, err = store.SyncWorkRoom(ctx, credential.CorpID, room)
	return err
}
