package store

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"
)

func newRedisIntegrationStore(t *testing.T, addr string) *RedisStore {
	t.Helper()
	database := 0
	if raw := strings.TrimSpace(os.Getenv("MOCHAT_REDIS_DB")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			t.Fatalf("invalid MOCHAT_REDIS_DB=%q", raw)
		}
		database = parsed
	}
	return NewRedisStore(RedisConfig{Addr: addr, DB: database})
}

func TestRedisStoreQueueIdempotencyIntegration(t *testing.T) {
	addr := os.Getenv("MOCHAT_REDIS_ADDR")
	if addr == "" {
		t.Skip("MOCHAT_REDIS_ADDR is not set")
	}
	store := newRedisIntegrationStore(t, addr)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	descriptor := dashboard.EmployeeApplyQueueDescriptor()
	event := dashboard.EmployeeApplyEvent{BindingID: 7, Source: "queue-idempotency-integration"}
	idempotencyKey := dashboard.EmployeeApplyIdempotencyKey(event)
	if err := store.client.Del(ctx, descriptor.SourceKey, descriptor.ProcessingKey, descriptor.DeadLetterKey, idempotencyKey).Err(); err != nil {
		t.Fatal(err)
	}

	if err := store.EnqueueEmployeeApply(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueEmployeeApply(ctx, event); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, descriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("source queue length = %d err=%v", length, err)
	}
	if ttl, err := store.client.TTL(ctx, idempotencyKey).Result(); err != nil || ttl <= 0 {
		t.Fatalf("idempotency ttl = %s err=%v", ttl, err)
	}

	raw, err := store.client.LIndex(ctx, descriptor.SourceKey, 0).Result()
	if err != nil {
		t.Fatal(err)
	}
	var envelope reliableQueueEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Queue != dashboard.QueueNameEmployeeApply || envelope.PayloadType != dashboard.QueuePayloadTypeEmployeeApply || envelope.IdempotencyKey != idempotencyKey || envelope.EnqueuedAt == "" || envelope.QueueTicket == "" {
		t.Fatalf("envelope = %+v", envelope)
	}
	var queuedEvent dashboard.EmployeeApplyEvent
	if err := json.Unmarshal(envelope.Payload, &queuedEvent); err != nil {
		t.Fatal(err)
	}
	if queuedEvent.QueueTicket != envelope.QueueTicket {
		t.Fatalf("event/envelope ticket mismatch: event=%q envelope=%q", queuedEvent.QueueTicket, envelope.QueueTicket)
	}

	delivery, ok, err := store.DequeueEmployeeApply(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected delivery")
	}
	if delivery.Event.BindingID != 7 || delivery.Event.Source != "queue-idempotency-integration" {
		t.Fatalf("delivery = %+v", delivery)
	}
	if err := store.EnqueueEmployeeApply(ctx, event); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, descriptor.SourceKey).Result(); err != nil || length != 0 {
		t.Fatalf("duplicate while processing source queue length = %d err=%v", length, err)
	}
	if length, err := store.client.LLen(ctx, descriptor.ProcessingKey).Result(); err != nil || length != 1 {
		t.Fatalf("processing queue length after duplicate = %d err=%v", length, err)
	}
	if err := store.AckEmployeeApply(ctx, delivery); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, descriptor.ProcessingKey).Result(); err != nil || length != 0 {
		t.Fatalf("processing queue length = %d err=%v", length, err)
	}
	if exists, err := store.client.Exists(ctx, idempotencyKey).Result(); err != nil || exists != 0 {
		t.Fatalf("idempotency key still exists after ack: exists=%d err=%v", exists, err)
	}
	if err := store.EnqueueEmployeeApply(ctx, event); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, descriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("source queue length after ack and re-enqueue = %d err=%v", length, err)
	}
	secondDelivery, ok, err := store.DequeueEmployeeApply(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || secondDelivery.Event.BindingID != delivery.Event.BindingID || secondDelivery.Event.Source != delivery.Event.Source {
		t.Fatalf("second delivery = %+v ok=%v", secondDelivery, ok)
	}
	if secondDelivery.Event.QueueTicket == delivery.Event.QueueTicket || secondDelivery.Event.QueueTicket == "" {
		t.Fatalf("second delivery ticket=%q, first ticket=%q; re-enqueue should claim a new ticket", secondDelivery.Event.QueueTicket, delivery.Event.QueueTicket)
	}
	if err := store.AckEmployeeApply(ctx, secondDelivery); err != nil {
		t.Fatal(err)
	}
	if err := store.client.RPush(ctx, descriptor.ProcessingKey, "legacy-employee-raw").Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.AckEmployeeApply(ctx, dashboard.EmployeeApplyDelivery{Raw: "legacy-employee-raw"}); err != nil {
		t.Fatal(err)
	}

	welcomeDescriptor := dashboard.ContactWelcomeQueueDescriptor()
	welcomeEvent := dashboard.ContactWelcomeEvent{
		CorpID:      7,
		ContactID:   101,
		EmployeeID:  3,
		ContactName: "客户A",
		WelcomeCode: "welcome-code",
		Content:     dashboard.ContactWelcomeContent{Text: "你好，##客户名称##"},
	}
	welcomeDuplicate := welcomeEvent
	welcomeDuplicate.ContactName = "客户A改名"
	welcomeIDKey := dashboard.ContactWelcomeIdempotencyKey(welcomeEvent)
	if err := store.client.Del(ctx, welcomeDescriptor.SourceKey, welcomeDescriptor.ProcessingKey, welcomeDescriptor.DeadLetterKey, welcomeIDKey).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueContactWelcome(ctx, welcomeEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueContactWelcome(ctx, welcomeDuplicate); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, welcomeDescriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("welcome source queue length = %d err=%v", length, err)
	}
	if ttl, err := store.client.TTL(ctx, welcomeIDKey).Result(); err != nil || ttl <= 0 {
		t.Fatalf("welcome idempotency ttl = %s err=%v", ttl, err)
	}
	welcomeDelivery, ok, err := store.DequeueContactWelcome(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected welcome delivery")
	}
	if welcomeDelivery.Event.ContactID != 101 || welcomeDelivery.Event.WelcomeCode != "welcome-code" || welcomeDelivery.Event.Content.Text == "" {
		t.Fatalf("welcome delivery = %+v", welcomeDelivery)
	}
	if err := store.AckContactWelcome(ctx, welcomeDelivery); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, welcomeDescriptor.ProcessingKey).Result(); err != nil || length != 0 {
		t.Fatalf("welcome processing queue length = %d err=%v", length, err)
	}

	fileDescriptor := dashboard.AsyncFileUploadQueueDescriptor()
	fileEvent := dashboard.AsyncFileUploadEvent{
		Files: []dashboard.AsyncFileUploadFile{
			{SourcePath: "/tmp/source-a.txt", TargetPath: "queued/source-a.txt", DeleteSource: true},
			{SourcePath: "https://example.com/source-b.txt", TargetPath: "queued/source-b.txt"},
		},
		Source: "queue-idempotency-integration",
	}
	fileDuplicate := dashboard.AsyncFileUploadEvent{
		Files: []dashboard.AsyncFileUploadFile{
			{SourcePath: "https://example.com/source-b.txt", TargetPath: "queued/source-b.txt"},
			{SourcePath: " /tmp/source-a.txt ", TargetPath: " queued/source-a.txt ", DeleteSource: true},
		},
		Source: " queue-idempotency-integration ",
	}
	fileIDKey := dashboard.AsyncFileUploadIdempotencyKey(fileEvent)
	if err := store.client.Del(ctx, fileDescriptor.SourceKey, fileDescriptor.ProcessingKey, fileDescriptor.DeadLetterKey, fileIDKey).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueAsyncFileUpload(ctx, fileEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueAsyncFileUpload(ctx, fileDuplicate); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, fileDescriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("async file source queue length = %d err=%v", length, err)
	}
	if ttl, err := store.client.TTL(ctx, fileIDKey).Result(); err != nil || ttl <= 0 {
		t.Fatalf("async file idempotency ttl = %s err=%v", ttl, err)
	}
	fileDelivery, ok, err := store.DequeueAsyncFileUpload(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected async file delivery")
	}
	if len(fileDelivery.Event.Files) != 2 || fileDelivery.Event.Files[0].TargetPath != "queued/source-a.txt" || !fileDelivery.Event.Files[0].DeleteSource {
		t.Fatalf("async file delivery = %+v", fileDelivery)
	}
	if err := store.AckAsyncFileUpload(ctx, fileDelivery); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, fileDescriptor.ProcessingKey).Result(); err != nil || length != 0 {
		t.Fatalf("async file processing queue length = %d err=%v", length, err)
	}

	markDescriptor := dashboard.MarkTagsQueueDescriptor()
	markEvent := dashboard.MarkTagsEvent{CorpID: 7, ContactID: 101, EmployeeID: 3, TagIDs: []int{9, 2, 9}, Source: "queue-idempotency-integration"}
	markDuplicate := dashboard.MarkTagsEvent{CorpID: 7, ContactID: 101, EmployeeID: 3, TagIDs: []int{2, 9}, Source: " queue-idempotency-integration "}
	markIDKey := dashboard.MarkTagsIdempotencyKey(markEvent)
	if err := store.client.Del(ctx, markDescriptor.SourceKey, markDescriptor.ProcessingKey, markDescriptor.DeadLetterKey, markIDKey).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueMarkTags(ctx, markEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueMarkTags(ctx, markDuplicate); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, markDescriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("mark tags source queue length = %d err=%v", length, err)
	}
	if ttl, err := store.client.TTL(ctx, markIDKey).Result(); err != nil || ttl <= 0 {
		t.Fatalf("mark tags idempotency ttl = %s err=%v", ttl, err)
	}
	markDelivery, ok, err := store.DequeueMarkTags(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected mark tags delivery")
	}
	if markDelivery.Event.CorpID != 7 || markDelivery.Event.ContactID != 101 || markDelivery.Event.EmployeeID != 3 || len(markDelivery.Event.TagIDs) != 3 {
		t.Fatalf("mark tags delivery = %+v", markDelivery)
	}
	if err := store.AckMarkTags(ctx, markDelivery); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, markDescriptor.ProcessingKey).Result(); err != nil || length != 0 {
		t.Fatalf("mark tags processing queue length = %d err=%v", length, err)
	}

	messageDescriptor := dashboard.MessageRemindQueueDescriptor()
	messageEvent := dashboard.MessageRemindEvent{
		CorpID:  7,
		ToUser:  dashboard.MessageRemindRecipients{"go-b", "go-a", "go-a"},
		MsgType: "text",
		Content: "提醒",
		Extra:   map[string]any{"safe": 1},
		Source:  "queue-idempotency-integration",
	}
	messageDuplicate := dashboard.MessageRemindEvent{
		CorpID:  7,
		ToUser:  dashboard.MessageRemindRecipients{" go-a ", "go-b"},
		MsgType: "text",
		Content: "提醒",
		Extra:   map[string]any{"safe": 1},
		Source:  " queue-idempotency-integration ",
	}
	messageIDKey := dashboard.MessageRemindIdempotencyKey(messageEvent)
	if err := store.client.Del(ctx, messageDescriptor.SourceKey, messageDescriptor.ProcessingKey, messageDescriptor.DeadLetterKey, messageIDKey).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueMessageRemind(ctx, messageEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueMessageRemind(ctx, messageDuplicate); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, messageDescriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("message remind source queue length = %d err=%v", length, err)
	}
	if ttl, err := store.client.TTL(ctx, messageIDKey).Result(); err != nil || ttl <= 0 {
		t.Fatalf("message remind idempotency ttl = %s err=%v", ttl, err)
	}
	messageDelivery, ok, err := store.DequeueMessageRemind(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected message remind delivery")
	}
	if messageDelivery.Event.CorpID != 7 || messageDelivery.Event.MsgType != "text" || len(messageDelivery.Event.ToUser) != 3 || messageDelivery.Event.Content != "提醒" {
		t.Fatalf("message remind delivery = %+v", messageDelivery)
	}
	if err := store.AckMessageRemind(ctx, messageDelivery); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, messageDescriptor.ProcessingKey).Result(); err != nil || length != 0 {
		t.Fatalf("message remind processing queue length = %d err=%v", length, err)
	}

	roomDescriptor := dashboard.WorkRoomSyncQueueDescriptor()
	roomEvent := dashboard.WorkRoomSyncEvent{CorpID: 7, ChatID: "chat-1", Source: "queue-idempotency-integration"}
	roomDuplicate := dashboard.WorkRoomSyncEvent{CorpID: 7, ChatID: " chat-1 ", Source: " queue-idempotency-integration "}
	roomIDKey := dashboard.WorkRoomSyncIdempotencyKey(roomEvent)
	if err := store.client.Del(ctx, roomDescriptor.SourceKey, roomDescriptor.ProcessingKey, roomDescriptor.DeadLetterKey, roomIDKey).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueWorkRoomSync(ctx, roomEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueWorkRoomSync(ctx, roomDuplicate); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, roomDescriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("work room sync source queue length = %d err=%v", length, err)
	}
	if ttl, err := store.client.TTL(ctx, roomIDKey).Result(); err != nil || ttl <= 0 {
		t.Fatalf("work room sync idempotency ttl = %s err=%v", ttl, err)
	}
	roomDelivery, ok, err := store.DequeueWorkRoomSync(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected work room sync delivery")
	}
	if roomDelivery.Event.CorpID != 7 || roomDelivery.Event.ChatID != "chat-1" || roomDelivery.Event.Source != "queue-idempotency-integration" {
		t.Fatalf("work room sync delivery = %+v", roomDelivery)
	}
	if err := store.AckWorkRoomSync(ctx, roomDelivery); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, roomDescriptor.ProcessingKey).Result(); err != nil || length != 0 {
		t.Fatalf("work room sync processing queue length = %d err=%v", length, err)
	}

	contactDescriptor := dashboard.WorkContactSyncQueueDescriptor()
	contactEvent := dashboard.WorkContactSyncEvent{
		CorpID: 7,
		Employees: []dashboard.WorkContactSyncEmployee{
			{ID: 12, WXUserID: "lisi"},
			{ID: 11, WXUserID: "zhangsan"},
		},
		ExternalUserIDs: []string{"external-1", "external-2"},
		Source:          "queue-idempotency-integration",
	}
	contactDuplicate := dashboard.WorkContactSyncEvent{
		CorpID: 7,
		Employees: []dashboard.WorkContactSyncEmployee{
			{ID: 11, WXUserID: " zhangsan "},
			{ID: 12, WXUserID: "lisi"},
		},
		ExternalUserIDs: []string{" external-2 ", "external-1", "external-1"},
		Source:          " queue-idempotency-integration ",
	}
	contactIDKey := dashboard.WorkContactSyncIdempotencyKey(contactEvent)
	if err := store.client.Del(ctx, contactDescriptor.SourceKey, contactDescriptor.ProcessingKey, contactDescriptor.DeadLetterKey, contactIDKey).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueWorkContactSync(ctx, contactEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueWorkContactSync(ctx, contactDuplicate); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, contactDescriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("work contact sync source queue length = %d err=%v", length, err)
	}
	if ttl, err := store.client.TTL(ctx, contactIDKey).Result(); err != nil || ttl <= 0 {
		t.Fatalf("work contact sync idempotency ttl = %s err=%v", ttl, err)
	}
	contactDelivery, ok, err := store.DequeueWorkContactSync(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected work contact sync delivery")
	}
	if contactDelivery.Event.CorpID != 7 || len(contactDelivery.Event.Employees) != 2 || len(contactDelivery.Event.ExternalUserIDs) != 2 || contactDelivery.Event.Source != "queue-idempotency-integration" {
		t.Fatalf("work contact sync delivery = %+v", contactDelivery)
	}
	if err := store.AckWorkContactSync(ctx, contactDelivery); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, contactDescriptor.ProcessingKey).Result(); err != nil || length != 0 {
		t.Fatalf("work contact sync processing queue length = %d err=%v", length, err)
	}

	departmentDescriptor := dashboard.WorkDepartmentListQueueDescriptor()
	departmentEvent := dashboard.WorkDepartmentListEvent{CorpIDs: []int{7, 2, 7}, UserID: 5, TenantID: 11, Source: "queue-idempotency-integration"}
	departmentDuplicate := dashboard.WorkDepartmentListEvent{CorpIDs: []int{2, 7}, UserID: 5, TenantID: 11, Source: " queue-idempotency-integration "}
	departmentIDKey := dashboard.WorkDepartmentListIdempotencyKey(departmentEvent)
	if err := store.client.Del(ctx, departmentDescriptor.SourceKey, departmentDescriptor.ProcessingKey, departmentDescriptor.DeadLetterKey, departmentIDKey).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueWorkDepartmentList(ctx, departmentEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueWorkDepartmentList(ctx, departmentDuplicate); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, departmentDescriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("work department list source queue length = %d err=%v", length, err)
	}
	if ttl, err := store.client.TTL(ctx, departmentIDKey).Result(); err != nil || ttl <= 0 {
		t.Fatalf("work department list idempotency ttl = %s err=%v", ttl, err)
	}
	departmentDelivery, ok, err := store.DequeueWorkDepartmentList(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected work department list delivery")
	}
	if len(departmentDelivery.Event.CorpIDs) != 3 || departmentDelivery.Event.UserID != 5 || departmentDelivery.Event.TenantID != 11 || departmentDelivery.Event.Source != "queue-idempotency-integration" {
		t.Fatalf("work department list delivery = %+v", departmentDelivery)
	}
	if err := store.AckWorkDepartmentList(ctx, departmentDelivery); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, departmentDescriptor.ProcessingKey).Result(); err != nil || length != 0 {
		t.Fatalf("work department list processing queue length = %d err=%v", length, err)
	}

	mediaDescriptor := dashboard.MediumMediaIDUpdateQueueDescriptor()
	mediaEvent := dashboard.MediumMediaIDUpdateEvent{CorpID: 7, MediumIDs: []int{21, 9, 21}, Source: "queue-idempotency-integration"}
	mediaDuplicate := dashboard.MediumMediaIDUpdateEvent{CorpID: 7, MediumIDs: []int{9, 21}, Source: " queue-idempotency-integration "}
	mediaIDKey := dashboard.MediumMediaIDUpdateIdempotencyKey(mediaEvent)
	if err := store.client.Del(ctx, mediaDescriptor.SourceKey, mediaDescriptor.ProcessingKey, mediaDescriptor.DeadLetterKey, mediaIDKey).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueMediumMediaIDUpdate(ctx, mediaEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueMediumMediaIDUpdate(ctx, mediaDuplicate); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, mediaDescriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("medium media id update source queue length = %d err=%v", length, err)
	}
	if ttl, err := store.client.TTL(ctx, mediaIDKey).Result(); err != nil || ttl <= 0 {
		t.Fatalf("medium media id update idempotency ttl = %s err=%v", ttl, err)
	}
	mediaDelivery, ok, err := store.DequeueMediumMediaIDUpdate(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected medium media id update delivery")
	}
	if mediaDelivery.Event.CorpID != 7 || len(mediaDelivery.Event.MediumIDs) != 3 || mediaDelivery.Event.Source != "queue-idempotency-integration" {
		t.Fatalf("medium media id update delivery = %+v", mediaDelivery)
	}
	if err := store.AckMediumMediaIDUpdate(ctx, mediaDelivery); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, mediaDescriptor.ProcessingKey).Result(); err != nil || length != 0 {
		t.Fatalf("medium media id update processing queue length = %d err=%v", length, err)
	}

	statisticDescriptor := dashboard.EmployeeStatisticApplyQueueDescriptor()
	statisticEvent := dashboard.EmployeeStatisticApplyEvent{Source: "queue-idempotency-integration"}
	statisticDuplicate := dashboard.EmployeeStatisticApplyEvent{Source: " queue-idempotency-integration "}
	statisticIDKey := dashboard.EmployeeStatisticApplyIdempotencyKey(statisticEvent)
	if err := store.client.Del(ctx, statisticDescriptor.SourceKey, statisticDescriptor.ProcessingKey, statisticDescriptor.DeadLetterKey, statisticIDKey).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueEmployeeStatisticApply(ctx, statisticEvent); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueEmployeeStatisticApply(ctx, statisticDuplicate); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, statisticDescriptor.SourceKey).Result(); err != nil || length != 1 {
		t.Fatalf("employee statistic apply source queue length = %d err=%v", length, err)
	}
	if ttl, err := store.client.TTL(ctx, statisticIDKey).Result(); err != nil || ttl <= 0 {
		t.Fatalf("employee statistic apply idempotency ttl = %s err=%v", ttl, err)
	}
	statisticDelivery, ok, err := store.DequeueEmployeeStatisticApply(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected employee statistic apply delivery")
	}
	if statisticDelivery.Event.Source != "queue-idempotency-integration" {
		t.Fatalf("employee statistic apply delivery = %+v", statisticDelivery)
	}
	if err := store.AckEmployeeStatisticApply(ctx, statisticDelivery); err != nil {
		t.Fatal(err)
	}
	if length, err := store.client.LLen(ctx, statisticDescriptor.ProcessingKey).Result(); err != nil || length != 0 {
		t.Fatalf("employee statistic apply processing queue length = %d err=%v", length, err)
	}
}

func TestRedisStoreEmployeeApplyDeadLetterReleasesIdempotencyIntegration(t *testing.T) {
	addr := os.Getenv("MOCHAT_REDIS_ADDR")
	if addr == "" {
		t.Skip("MOCHAT_REDIS_ADDR is not set")
	}
	store := newRedisIntegrationStore(t, addr)
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	descriptor := dashboard.EmployeeApplyQueueDescriptor()
	event := dashboard.EmployeeApplyEvent{BindingID: 7001, Source: "dead-letter-release-integration"}
	idempotencyKey := dashboard.EmployeeApplyIdempotencyKey(event)
	cleanup := func() {
		_ = store.client.Del(ctx, descriptor.SourceKey, descriptor.ProcessingKey, descriptor.DeadLetterKey, idempotencyKey).Err()
	}
	cleanup()
	defer cleanup()

	if err := store.EnqueueEmployeeApply(ctx, event); err != nil {
		t.Fatal(err)
	}
	delivery, ok, err := store.DequeueEmployeeApply(ctx, time.Second)
	if err != nil || !ok {
		t.Fatalf("dequeue ok=%v err=%v", ok, err)
	}
	if delivery.Event.QueueTicket == "" {
		t.Fatalf("dequeued employee apply event lost queue ticket: %+v", delivery.Event)
	}
	deadLettered, err := store.RetryEmployeeApply(ctx, delivery, "SYNC_FAILED", 1)
	if err != nil || !deadLettered {
		t.Fatalf("dead-lettered=%v err=%v", deadLettered, err)
	}
	if exists, err := store.client.Exists(ctx, idempotencyKey).Result(); err != nil || exists != 0 {
		t.Fatalf("dead-letter retained idempotency key: exists=%d err=%v", exists, err)
	}
	if source, err := store.client.LLen(ctx, descriptor.SourceKey).Result(); err != nil || source != 0 {
		t.Fatalf("source after dead-letter=%d err=%v", source, err)
	}
	if dead, err := store.client.LLen(ctx, descriptor.DeadLetterKey).Result(); err != nil || dead != 1 {
		t.Fatalf("dead-letter queue=%d err=%v", dead, err)
	}
	if err := store.EnqueueEmployeeApply(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueEmployeeApply(ctx, event); err != nil {
		t.Fatal(err)
	}
	if source, err := store.client.LLen(ctx, descriptor.SourceKey).Result(); err != nil || source != 1 {
		t.Fatalf("source after immediate re-enqueue duplicate=%d err=%v", source, err)
	}
}

func TestRedisStoreEmployeeApplyCleanupDoesNotDeleteNewerClaimIntegration(t *testing.T) {
	addr := os.Getenv("MOCHAT_REDIS_ADDR")
	if addr == "" {
		t.Skip("MOCHAT_REDIS_ADDR is not set; an isolated Redis DSN is required")
	}
	store := newRedisIntegrationStore(t, addr)
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	descriptor := dashboard.EmployeeApplyQueueDescriptor()
	key := "mochat-go:test:employee-apply:newer-claim"
	deadKey := key + ":dead"
	cleanup := func() {
		_ = store.client.Del(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, key, deadKey).Err()
	}
	cleanup()
	defer cleanup()

	oldRaw, err := json.Marshal(reliableQueueEnvelope{
		Queue: descriptor.Name, PayloadType: descriptor.PayloadType, IdempotencyKey: key, QueueTicket: "1",
		Payload: json.RawMessage(`{"bindingId":7002,"source":"cleanup-ticket-test","queueTicket":"1"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.client.Set(ctx, key, "2", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.client.RPush(ctx, descriptor.ProcessingKey, oldRaw).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.AckEmployeeApply(ctx, dashboard.EmployeeApplyDelivery{Raw: string(oldRaw)}); err != nil {
		t.Fatal(err)
	}
	if got, err := store.client.Get(ctx, key).Result(); err != nil || got != "2" {
		t.Fatalf("newer claim after old ack=%q err=%v, want 2", got, err)
	}

	newRaw, err := json.Marshal(reliableQueueEnvelope{
		Queue: descriptor.Name, PayloadType: descriptor.PayloadType, IdempotencyKey: key, QueueTicket: "2",
		Payload: json.RawMessage(`{"bindingId":7002,"source":"cleanup-ticket-test","queueTicket":"2"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.client.RPush(ctx, descriptor.ProcessingKey, newRaw).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.AckEmployeeApply(ctx, dashboard.EmployeeApplyDelivery{Raw: string(newRaw)}); err != nil {
		t.Fatal(err)
	}
	if exists, err := store.client.Exists(ctx, key).Result(); err != nil || exists != 0 {
		t.Fatalf("matching ticket did not release idempotency key: exists=%d err=%v", exists, err)
	}

	oldDeadRaw := strings.Replace(string(oldRaw), `"cleanup-ticket-test"`, `"cleanup-ticket-dead-test"`, 1)
	oldDeadRaw = strings.Replace(oldDeadRaw, `"`+key+`"`, `"`+deadKey+`"`, 1)
	if err := store.client.Set(ctx, deadKey, "2", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.client.RPush(ctx, descriptor.ProcessingKey, oldDeadRaw).Err(); err != nil {
		t.Fatal(err)
	}
	deadLettered, err := store.RetryEmployeeApply(ctx, dashboard.EmployeeApplyDelivery{Raw: oldDeadRaw}, "SYNC_FAILED", 1)
	if err != nil || !deadLettered {
		t.Fatalf("old-ticket dead-lettered=%v err=%v", deadLettered, err)
	}
	if got, err := store.client.Get(ctx, deadKey).Result(); err != nil || got != "2" {
		t.Fatalf("newer claim after old dead-letter=%q err=%v, want 2", got, err)
	}
	newDeadRaw := strings.Replace(oldDeadRaw, `"queueTicket":"1"`, `"queueTicket":"2"`, 1)
	if err := store.client.RPush(ctx, descriptor.ProcessingKey, newDeadRaw).Err(); err != nil {
		t.Fatal(err)
	}
	deadLettered, err = store.RetryEmployeeApply(ctx, dashboard.EmployeeApplyDelivery{Raw: newDeadRaw}, "SYNC_FAILED", 1)
	if err != nil || !deadLettered {
		t.Fatalf("matching-ticket dead-lettered=%v err=%v", deadLettered, err)
	}
	if exists, err := store.client.Exists(ctx, deadKey).Result(); err != nil || exists != 0 {
		t.Fatalf("matching dead-letter ticket did not release idempotency key: exists=%d err=%v", exists, err)
	}
}

func TestRedisStoreEmployeeApplyEnqueueWrongTypeRollsBackClaimIntegration(t *testing.T) {
	addr := os.Getenv("MOCHAT_REDIS_ADDR")
	if addr == "" {
		t.Skip("MOCHAT_REDIS_ADDR is not set; an isolated Redis DSN is required")
	}
	store := newRedisIntegrationStore(t, addr)
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	descriptor := dashboard.EmployeeApplyQueueDescriptor()
	event := dashboard.EmployeeApplyEvent{BindingID: 7003, Source: "enqueue-wrong-type-test"}
	key := dashboard.EmployeeApplyIdempotencyKey(event)
	cleanup := func() {
		_ = store.client.Del(ctx, descriptor.SourceKey, key).Err()
	}
	cleanup()
	defer cleanup()
	if err := store.client.Set(ctx, descriptor.SourceKey, "wrong-type", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnqueueEmployeeApplyWithReceipt(ctx, event); err == nil {
		t.Fatal("enqueue unexpectedly succeeded against a string source key")
	}
	if exists, err := store.client.Exists(ctx, key).Result(); err != nil || exists != 0 {
		t.Fatalf("failed enqueue left idempotency claim: exists=%d err=%v", exists, err)
	}
}
