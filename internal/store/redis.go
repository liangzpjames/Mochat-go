package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboard"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type RedisStore struct {
	client                      *redis.Client
	weWorkCallbackWakeupTestKey string
}

const (
	legacyWeWorkCallbackPendingKey    = "mochat-go:wework-callback"
	legacyWeWorkCallbackProcessingKey = "mochat-go:wework-callback:processing"
	legacyWeWorkCallbackDeadKey       = "mochat-go:wework-callback:dead"
	weWorkCallbackWakeupKey           = "mochat-go:wework-callback:wakeup"
)

func NewRedisStore(cfg RedisConfig) *RedisStore {
	return &RedisStore{client: redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})}
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}

func (s *RedisStore) UserCorpCache(ctx context.Context, userID int) (string, error) {
	value, err := s.client.Get(ctx, fmt.Sprintf("mc:user.%d", userID)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

func (s *RedisStore) SetUserCorpCache(ctx context.Context, userID int, value string) error {
	return s.client.Set(ctx, fmt.Sprintf("mc:user.%d", userID), value, 0).Err()
}

func (s *RedisStore) DeleteUserCorpCache(ctx context.Context, userID int) error {
	return s.client.Del(ctx, fmt.Sprintf("mc:user.%d", userID)).Err()
}

func (s *RedisStore) ContactTransferStateLogID(ctx context.Context) (int, error) {
	value, err := s.client.Get(ctx, "log_id").Int()
	if err == redis.Nil {
		return 0, nil
	}
	return value, err
}

func (s *RedisStore) SetContactTransferStateLogID(ctx context.Context, logID int, ttl time.Duration) error {
	return s.client.Set(ctx, "log_id", logID, ttl).Err()
}

func (s *RedisStore) EmployeeStatisticApplied(ctx context.Context, corpID int, employeeID int, startUnix int64) (bool, error) {
	count, err := s.client.Exists(ctx, employeeStatisticRedisKey(corpID, employeeID, startUnix)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *RedisStore) SetEmployeeStatisticApplied(ctx context.Context, corpID int, employeeID int, startUnix int64, ttl time.Duration) error {
	return s.client.Set(ctx, employeeStatisticRedisKey(corpID, employeeID, startUnix), startUnix, ttl).Err()
}

func (s *RedisStore) JWTBlacklisted(ctx context.Context, key string) (bool, error) {
	count, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *RedisStore) AddJWTBlacklist(ctx context.Context, key string, ttl time.Duration) error {
	return s.client.Set(ctx, key, time.Now().Unix(), ttl).Err()
}

func (s *RedisStore) WakeWeWorkCallback(ctx context.Context) error {
	wakeupKey := s.weWorkCallbackWakeupKey()
	pipe := s.client.TxPipeline()
	pipe.LPush(ctx, wakeupKey, "pending")
	pipe.LTrim(ctx, wakeupKey, 0, 63)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) WaitWeWorkCallbackWakeup(ctx context.Context, timeout time.Duration) error {
	if s == nil || s.client == nil {
		return errors.New("wework callback wakeup store is not configured")
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	_, err := s.client.BRPop(ctx, timeout, s.weWorkCallbackWakeupKey()).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	return err
}

func (s *RedisStore) weWorkCallbackWakeupKey() string {
	if s != nil && s.weWorkCallbackWakeupTestKey != "" {
		return s.weWorkCallbackWakeupTestKey
	}
	return weWorkCallbackWakeupKey
}

func (s *RedisStore) PreflightLegacyWeWorkCallbackBacklog(ctx context.Context) (dashboard.LegacyWeWorkCallbackBacklogStats, error) {
	pipe := s.client.Pipeline()
	pending := pipe.LLen(ctx, legacyWeWorkCallbackPendingKey)
	processing := pipe.LLen(ctx, legacyWeWorkCallbackProcessingKey)
	dead := pipe.LLen(ctx, legacyWeWorkCallbackDeadKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return dashboard.LegacyWeWorkCallbackBacklogStats{}, err
	}
	return dashboard.LegacyWeWorkCallbackBacklogStats{Pending: pending.Val(), Processing: processing.Val(), Dead: dead.Val()}, nil
}

func (s *RedisStore) NextLegacyWeWorkCallback(ctx context.Context) (dashboard.LegacyWeWorkCallbackDelivery, bool, error) {
	raw, err := s.client.LIndex(ctx, legacyWeWorkCallbackProcessingKey, 0).Result()
	if err == redis.Nil {
		raw, err = s.client.LMove(ctx, legacyWeWorkCallbackPendingKey, legacyWeWorkCallbackProcessingKey, "LEFT", "RIGHT").Result()
	}
	if err == redis.Nil {
		return dashboard.LegacyWeWorkCallbackDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.LegacyWeWorkCallbackDelivery{}, false, err
	}
	var event dashboard.WeWorkCallbackEvent
	if _, err := decodeReliableQueuePayload(raw, &event); err != nil {
		return dashboard.LegacyWeWorkCallbackDelivery{}, false, fmt.Errorf("decode legacy wework callback delivery: %w", err)
	}
	return dashboard.LegacyWeWorkCallbackDelivery{Event: event, Raw: raw}, true, nil
}

func (s *RedisStore) AckLegacyWeWorkCallback(ctx context.Context, delivery dashboard.LegacyWeWorkCallbackDelivery) error {
	return s.ackReliableQueueItem(ctx, legacyWeWorkCallbackProcessingKey, delivery.Raw)
}

func (s *RedisStore) EnqueueContactWelcome(ctx context.Context, event dashboard.ContactWelcomeEvent) error {
	return s.enqueueReliableQueueItem(ctx, dashboard.ContactWelcomeQueueDescriptor(), event, dashboard.ContactWelcomeIdempotencyKey(event))
}

func (s *RedisStore) DequeueContactWelcome(ctx context.Context, timeout time.Duration) (dashboard.ContactWelcomeDelivery, bool, error) {
	descriptor := dashboard.ContactWelcomeQueueDescriptor()
	raw, err := s.client.BLMove(ctx, descriptor.SourceKey, descriptor.ProcessingKey, "LEFT", "RIGHT", timeout).Result()
	if err == redis.Nil {
		return dashboard.ContactWelcomeDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.ContactWelcomeDelivery{}, false, err
	}
	var event dashboard.ContactWelcomeEvent
	attempts, err := decodeReliableQueuePayload(raw, &event)
	if err != nil {
		_ = s.moveMalformedQueueItem(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, raw, err.Error())
		return dashboard.ContactWelcomeDelivery{}, false, err
	}
	markedRaw, err := s.markReliableQueueProcessing(ctx, descriptor.ProcessingKey, raw, event, attempts)
	if err != nil {
		return dashboard.ContactWelcomeDelivery{}, false, err
	}
	return dashboard.ContactWelcomeDelivery{Event: event, Raw: markedRaw, Attempts: attempts}, true, nil
}

func (s *RedisStore) AckContactWelcome(ctx context.Context, delivery dashboard.ContactWelcomeDelivery) error {
	return s.ackReliableQueueItem(ctx, dashboard.ContactWelcomeQueueDescriptor().ProcessingKey, delivery.Raw)
}

func (s *RedisStore) RetryContactWelcome(ctx context.Context, delivery dashboard.ContactWelcomeDelivery, reason string, maxAttempts int) (bool, error) {
	descriptor := dashboard.ContactWelcomeQueueDescriptor()
	return s.retryReliableQueueItem(ctx, reliableQueueRetryOptions{
		SourceKey:      descriptor.SourceKey,
		ProcessingKey:  descriptor.ProcessingKey,
		DeadLetterKey:  descriptor.DeadLetterKey,
		Raw:            delivery.Raw,
		Event:          delivery.Event,
		CurrentAttempt: delivery.Attempts,
		Reason:         reason,
		MaxAttempts:    maxAttempts,
	})
}

func (s *RedisStore) RecoverContactWelcomeProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	descriptor := dashboard.ContactWelcomeQueueDescriptor()
	return s.recoverReliableQueueProcessing(ctx, reliableQueueRecoveryOptions{
		SourceKey:     descriptor.SourceKey,
		ProcessingKey: descriptor.ProcessingKey,
		DeadLetterKey: descriptor.DeadLetterKey,
		StaleAfter:    staleAfter,
		MaxAttempts:   maxAttempts,
	})
}

func (s *RedisStore) WorkContactWelcomeStatus(ctx context.Context, contactID int) (int, error) {
	status, err := s.client.Get(ctx, workContactWelcomeStatusRedisKey(contactID)).Int()
	if err == redis.Nil {
		return 0, nil
	}
	return status, err
}

func (s *RedisStore) SetWorkContactWelcomeStatus(ctx context.Context, contactID int, status int, ttl time.Duration) error {
	return s.client.Set(ctx, workContactWelcomeStatusRedisKey(contactID), status, ttl).Err()
}

const employeeApplyTicketKey = "mochat-go:queue-ticket:employee-apply"

const employeeApplyEnqueueScript = `
local now = redis.call("TIME")
local sourceType = redis.call("TYPE", KEYS[1]).ok
local claimType = redis.call("TYPE", KEYS[2]).ok
local ticketType = redis.call("TYPE", KEYS[3]).ok
if sourceType ~= "none" and sourceType ~= "list" then
  return redis.error_reply("employee apply source key has wrong type")
end
if claimType ~= "none" and claimType ~= "string" then
  return redis.error_reply("employee apply idempotency key has wrong type")
end
if ticketType ~= "none" and ticketType ~= "string" then
  return redis.error_reply("employee apply ticket key has wrong type")
end
local existing = redis.call("GET", KEYS[2])
if existing then
  return {0, existing, now[1], now[2]}
end
local ticket = tostring(redis.call("INCR", KEYS[3]))
local ok = redis.call("SET", KEYS[2], ticket, "NX", "EX", ARGV[2])
if not ok then
  existing = redis.call("GET", KEYS[2])
  return {0, existing or "", now[1], now[2]}
end
local raw = string.gsub(ARGV[1], '"queueTicket":"__QUEUE_TICKET__"', '"queueTicket":"' .. ticket .. '"')
local appended, appendResult = pcall(redis.call, "RPUSH", KEYS[1], raw)
if not appended then
  local claimed = redis.call("GET", KEYS[2])
  if claimed == ticket then
    redis.call("DEL", KEYS[2])
  end
  return redis.error_reply("employee apply enqueue append failed")
end
return {1, ticket, now[1], now[2]}
`

func employeeApplyEnqueueReceiptFromRedis(values []interface{}) (companyprofile.EmployeeSyncEnqueueReceipt, error) {
	if len(values) < 4 {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, errors.New("employee apply enqueue receipt is malformed")
	}
	inserted, err := redisResultInt64(values[0])
	if err != nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, err
	}
	if inserted != 0 && inserted != 1 {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, errors.New("employee apply enqueue insertion flag is invalid")
	}
	ticket, err := redisResultString(values[1])
	if err != nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, err
	}
	seconds, err := redisResultInt64(values[2])
	if err != nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, err
	}
	micros, err := redisResultInt64(values[3])
	if err != nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, err
	}
	if ticket == "" {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, errors.New("employee apply enqueue ticket is empty")
	}
	return companyprofile.EmployeeSyncEnqueueReceipt{
		Cursor:      dashboard.CompanyEmployeeSyncCursor,
		Ticket:      ticket,
		RequestedAt: time.Unix(seconds, micros*1000).UTC(),
	}, nil
}

func redisResultString(value interface{}) (string, error) {
	switch value := value.(type) {
	case string:
		return value, nil
	case []byte:
		return string(value), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case int:
		return strconv.Itoa(value), nil
	default:
		return "", fmt.Errorf("unexpected redis result type %T", value)
	}
}

func redisResultInt64(value interface{}) (int64, error) {
	raw, err := redisResultString(value)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(raw, 10, 64)
}

func (s *RedisStore) EnqueueEmployeeApply(ctx context.Context, event dashboard.EmployeeApplyEvent) error {
	_, err := s.EnqueueEmployeeApplyWithReceipt(ctx, event)
	return err
}

func (s *RedisStore) EnqueueEmployeeApplyWithReceipt(ctx context.Context, event dashboard.EmployeeApplyEvent) (companyprofile.EmployeeSyncEnqueueReceipt, error) {
	descriptor := dashboard.EmployeeApplyQueueDescriptor()
	const queueTicketPlaceholder = "__QUEUE_TICKET__"
	event.QueueTicket = queueTicketPlaceholder
	payload, err := json.Marshal(event)
	if err != nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, err
	}
	idempotencyKey := dashboard.EmployeeApplyIdempotencyKey(event)
	if idempotencyKey == "" {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, fmt.Errorf("employee apply idempotency key unavailable")
	}
	serverNow, err := s.client.Time(ctx).Result()
	if err != nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, err
	}
	envelope := reliableQueueEnvelope{
		Queue: descriptor.Name, PayloadType: descriptor.PayloadType, IdempotencyKey: idempotencyKey,
		EnqueuedAt: serverNow.UTC().Format(time.RFC3339Nano), QueueTicket: queueTicketPlaceholder, Payload: payload,
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, err
	}
	ttlSeconds := int(descriptor.IdempotencyTTL.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = int((10 * time.Minute).Seconds())
	}
	result, err := s.client.Eval(ctx, employeeApplyEnqueueScript, []string{descriptor.SourceKey, idempotencyKey, employeeApplyTicketKey}, string(raw), ttlSeconds).Slice()
	if err != nil {
		return companyprofile.EmployeeSyncEnqueueReceipt{}, err
	}
	return employeeApplyEnqueueReceiptFromRedis(result)
}

func (s *RedisStore) DequeueEmployeeApply(ctx context.Context, timeout time.Duration) (dashboard.EmployeeApplyDelivery, bool, error) {
	descriptor := dashboard.EmployeeApplyQueueDescriptor()
	raw, err := s.client.BLMove(ctx, descriptor.SourceKey, descriptor.ProcessingKey, "LEFT", "RIGHT", timeout).Result()
	if err == redis.Nil {
		return dashboard.EmployeeApplyDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.EmployeeApplyDelivery{}, false, err
	}
	var event dashboard.EmployeeApplyEvent
	attempts, err := decodeReliableQueuePayload(raw, &event)
	if err != nil {
		_ = s.moveMalformedQueueItem(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, raw, err.Error())
		return dashboard.EmployeeApplyDelivery{}, false, err
	}
	markedRaw, err := s.markReliableQueueProcessing(ctx, descriptor.ProcessingKey, raw, event, attempts)
	if err != nil {
		return dashboard.EmployeeApplyDelivery{}, false, err
	}
	return dashboard.EmployeeApplyDelivery{Event: event, Raw: markedRaw, Attempts: attempts}, true, nil
}

func (s *RedisStore) AckEmployeeApply(ctx context.Context, delivery dashboard.EmployeeApplyDelivery) error {
	return s.ackEmployeeApplyQueueItem(ctx, dashboard.EmployeeApplyQueueDescriptor().ProcessingKey, delivery.Raw)
}

func (s *RedisStore) RetryEmployeeApply(ctx context.Context, delivery dashboard.EmployeeApplyDelivery, reason string, maxAttempts int) (bool, error) {
	descriptor := dashboard.EmployeeApplyQueueDescriptor()
	return s.retryReliableQueueItem(ctx, reliableQueueRetryOptions{
		SourceKey:      descriptor.SourceKey,
		ProcessingKey:  descriptor.ProcessingKey,
		DeadLetterKey:  descriptor.DeadLetterKey,
		Raw:            delivery.Raw,
		Event:          delivery.Event,
		CurrentAttempt: delivery.Attempts,
		Reason:         reason,
		MaxAttempts:    maxAttempts,
	})
}

func (s *RedisStore) RecoverEmployeeApplyProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	descriptor := dashboard.EmployeeApplyQueueDescriptor()
	return s.recoverReliableQueueProcessing(ctx, reliableQueueRecoveryOptions{
		SourceKey:     descriptor.SourceKey,
		ProcessingKey: descriptor.ProcessingKey,
		DeadLetterKey: descriptor.DeadLetterKey,
		StaleAfter:    staleAfter,
		MaxAttempts:   maxAttempts,
	})
}

func (s *RedisStore) EnqueueAsyncFileUpload(ctx context.Context, event dashboard.AsyncFileUploadEvent) error {
	return s.enqueueReliableQueueItem(ctx, dashboard.AsyncFileUploadQueueDescriptor(), event, dashboard.AsyncFileUploadIdempotencyKey(event))
}

func (s *RedisStore) DequeueAsyncFileUpload(ctx context.Context, timeout time.Duration) (dashboard.AsyncFileUploadDelivery, bool, error) {
	descriptor := dashboard.AsyncFileUploadQueueDescriptor()
	raw, err := s.client.BLMove(ctx, descriptor.SourceKey, descriptor.ProcessingKey, "LEFT", "RIGHT", timeout).Result()
	if err == redis.Nil {
		return dashboard.AsyncFileUploadDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.AsyncFileUploadDelivery{}, false, err
	}
	var event dashboard.AsyncFileUploadEvent
	attempts, err := decodeReliableQueuePayload(raw, &event)
	if err != nil {
		_ = s.moveMalformedQueueItem(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, raw, err.Error())
		return dashboard.AsyncFileUploadDelivery{}, false, err
	}
	markedRaw, err := s.markReliableQueueProcessing(ctx, descriptor.ProcessingKey, raw, event, attempts)
	if err != nil {
		return dashboard.AsyncFileUploadDelivery{}, false, err
	}
	return dashboard.AsyncFileUploadDelivery{Event: event, Raw: markedRaw, Attempts: attempts}, true, nil
}

func (s *RedisStore) AckAsyncFileUpload(ctx context.Context, delivery dashboard.AsyncFileUploadDelivery) error {
	return s.ackReliableQueueItem(ctx, dashboard.AsyncFileUploadQueueDescriptor().ProcessingKey, delivery.Raw)
}

func (s *RedisStore) RetryAsyncFileUpload(ctx context.Context, delivery dashboard.AsyncFileUploadDelivery, reason string, maxAttempts int) (bool, error) {
	descriptor := dashboard.AsyncFileUploadQueueDescriptor()
	return s.retryReliableQueueItem(ctx, reliableQueueRetryOptions{
		SourceKey:      descriptor.SourceKey,
		ProcessingKey:  descriptor.ProcessingKey,
		DeadLetterKey:  descriptor.DeadLetterKey,
		Raw:            delivery.Raw,
		Event:          delivery.Event,
		CurrentAttempt: delivery.Attempts,
		Reason:         reason,
		MaxAttempts:    maxAttempts,
	})
}

func (s *RedisStore) RecoverAsyncFileUploadProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	descriptor := dashboard.AsyncFileUploadQueueDescriptor()
	return s.recoverReliableQueueProcessing(ctx, reliableQueueRecoveryOptions{
		SourceKey:     descriptor.SourceKey,
		ProcessingKey: descriptor.ProcessingKey,
		DeadLetterKey: descriptor.DeadLetterKey,
		StaleAfter:    staleAfter,
		MaxAttempts:   maxAttempts,
	})
}

func (s *RedisStore) EnqueueMarkTags(ctx context.Context, event dashboard.MarkTagsEvent) error {
	return s.enqueueReliableQueueItem(ctx, dashboard.MarkTagsQueueDescriptor(), event, dashboard.MarkTagsIdempotencyKey(event))
}

func (s *RedisStore) DequeueMarkTags(ctx context.Context, timeout time.Duration) (dashboard.MarkTagsDelivery, bool, error) {
	descriptor := dashboard.MarkTagsQueueDescriptor()
	raw, err := s.client.BLMove(ctx, descriptor.SourceKey, descriptor.ProcessingKey, "LEFT", "RIGHT", timeout).Result()
	if err == redis.Nil {
		return dashboard.MarkTagsDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.MarkTagsDelivery{}, false, err
	}
	var event dashboard.MarkTagsEvent
	attempts, err := decodeReliableQueuePayload(raw, &event)
	if err != nil {
		_ = s.moveMalformedQueueItem(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, raw, err.Error())
		return dashboard.MarkTagsDelivery{}, false, err
	}
	markedRaw, err := s.markReliableQueueProcessing(ctx, descriptor.ProcessingKey, raw, event, attempts)
	if err != nil {
		return dashboard.MarkTagsDelivery{}, false, err
	}
	return dashboard.MarkTagsDelivery{Event: event, Raw: markedRaw, Attempts: attempts}, true, nil
}

func (s *RedisStore) AckMarkTags(ctx context.Context, delivery dashboard.MarkTagsDelivery) error {
	return s.ackReliableQueueItem(ctx, dashboard.MarkTagsQueueDescriptor().ProcessingKey, delivery.Raw)
}

func (s *RedisStore) RetryMarkTags(ctx context.Context, delivery dashboard.MarkTagsDelivery, reason string, maxAttempts int) (bool, error) {
	descriptor := dashboard.MarkTagsQueueDescriptor()
	return s.retryReliableQueueItem(ctx, reliableQueueRetryOptions{
		SourceKey:      descriptor.SourceKey,
		ProcessingKey:  descriptor.ProcessingKey,
		DeadLetterKey:  descriptor.DeadLetterKey,
		Raw:            delivery.Raw,
		Event:          delivery.Event,
		CurrentAttempt: delivery.Attempts,
		Reason:         reason,
		MaxAttempts:    maxAttempts,
	})
}

func (s *RedisStore) RecoverMarkTagsProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	descriptor := dashboard.MarkTagsQueueDescriptor()
	return s.recoverReliableQueueProcessing(ctx, reliableQueueRecoveryOptions{
		SourceKey:     descriptor.SourceKey,
		ProcessingKey: descriptor.ProcessingKey,
		DeadLetterKey: descriptor.DeadLetterKey,
		StaleAfter:    staleAfter,
		MaxAttempts:   maxAttempts,
	})
}

func (s *RedisStore) EnqueueMessageRemind(ctx context.Context, event dashboard.MessageRemindEvent) error {
	return s.enqueueReliableQueueItem(ctx, dashboard.MessageRemindQueueDescriptor(), event, dashboard.MessageRemindIdempotencyKey(event))
}

func (s *RedisStore) DequeueMessageRemind(ctx context.Context, timeout time.Duration) (dashboard.MessageRemindDelivery, bool, error) {
	descriptor := dashboard.MessageRemindQueueDescriptor()
	raw, err := s.client.BLMove(ctx, descriptor.SourceKey, descriptor.ProcessingKey, "LEFT", "RIGHT", timeout).Result()
	if err == redis.Nil {
		return dashboard.MessageRemindDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.MessageRemindDelivery{}, false, err
	}
	var event dashboard.MessageRemindEvent
	attempts, err := decodeReliableQueuePayload(raw, &event)
	if err != nil {
		_ = s.moveMalformedQueueItem(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, raw, err.Error())
		return dashboard.MessageRemindDelivery{}, false, err
	}
	markedRaw, err := s.markReliableQueueProcessing(ctx, descriptor.ProcessingKey, raw, event, attempts)
	if err != nil {
		return dashboard.MessageRemindDelivery{}, false, err
	}
	return dashboard.MessageRemindDelivery{Event: event, Raw: markedRaw, Attempts: attempts}, true, nil
}

func (s *RedisStore) AckMessageRemind(ctx context.Context, delivery dashboard.MessageRemindDelivery) error {
	return s.ackReliableQueueItem(ctx, dashboard.MessageRemindQueueDescriptor().ProcessingKey, delivery.Raw)
}

func (s *RedisStore) RetryMessageRemind(ctx context.Context, delivery dashboard.MessageRemindDelivery, reason string, maxAttempts int) (bool, error) {
	descriptor := dashboard.MessageRemindQueueDescriptor()
	return s.retryReliableQueueItem(ctx, reliableQueueRetryOptions{
		SourceKey:      descriptor.SourceKey,
		ProcessingKey:  descriptor.ProcessingKey,
		DeadLetterKey:  descriptor.DeadLetterKey,
		Raw:            delivery.Raw,
		Event:          delivery.Event,
		CurrentAttempt: delivery.Attempts,
		Reason:         reason,
		MaxAttempts:    maxAttempts,
	})
}

func (s *RedisStore) RecoverMessageRemindProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	descriptor := dashboard.MessageRemindQueueDescriptor()
	return s.recoverReliableQueueProcessing(ctx, reliableQueueRecoveryOptions{
		SourceKey:     descriptor.SourceKey,
		ProcessingKey: descriptor.ProcessingKey,
		DeadLetterKey: descriptor.DeadLetterKey,
		StaleAfter:    staleAfter,
		MaxAttempts:   maxAttempts,
	})
}

func (s *RedisStore) EnqueueWorkRoomSync(ctx context.Context, event dashboard.WorkRoomSyncEvent) error {
	return s.enqueueReliableQueueItem(ctx, dashboard.WorkRoomSyncQueueDescriptor(), event, dashboard.WorkRoomSyncIdempotencyKey(event))
}

func (s *RedisStore) DequeueWorkRoomSync(ctx context.Context, timeout time.Duration) (dashboard.WorkRoomSyncDelivery, bool, error) {
	descriptor := dashboard.WorkRoomSyncQueueDescriptor()
	raw, err := s.client.BLMove(ctx, descriptor.SourceKey, descriptor.ProcessingKey, "LEFT", "RIGHT", timeout).Result()
	if err == redis.Nil {
		return dashboard.WorkRoomSyncDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.WorkRoomSyncDelivery{}, false, err
	}
	var event dashboard.WorkRoomSyncEvent
	attempts, err := decodeReliableQueuePayload(raw, &event)
	if err != nil {
		_ = s.moveMalformedQueueItem(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, raw, err.Error())
		return dashboard.WorkRoomSyncDelivery{}, false, err
	}
	markedRaw, err := s.markReliableQueueProcessing(ctx, descriptor.ProcessingKey, raw, event, attempts)
	if err != nil {
		return dashboard.WorkRoomSyncDelivery{}, false, err
	}
	return dashboard.WorkRoomSyncDelivery{Event: event, Raw: markedRaw, Attempts: attempts}, true, nil
}

func (s *RedisStore) AckWorkRoomSync(ctx context.Context, delivery dashboard.WorkRoomSyncDelivery) error {
	return s.ackReliableQueueItem(ctx, dashboard.WorkRoomSyncQueueDescriptor().ProcessingKey, delivery.Raw)
}

func (s *RedisStore) RetryWorkRoomSync(ctx context.Context, delivery dashboard.WorkRoomSyncDelivery, reason string, maxAttempts int) (bool, error) {
	descriptor := dashboard.WorkRoomSyncQueueDescriptor()
	return s.retryReliableQueueItem(ctx, reliableQueueRetryOptions{
		SourceKey:      descriptor.SourceKey,
		ProcessingKey:  descriptor.ProcessingKey,
		DeadLetterKey:  descriptor.DeadLetterKey,
		Raw:            delivery.Raw,
		Event:          delivery.Event,
		CurrentAttempt: delivery.Attempts,
		Reason:         reason,
		MaxAttempts:    maxAttempts,
	})
}

func (s *RedisStore) RecoverWorkRoomSyncProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	descriptor := dashboard.WorkRoomSyncQueueDescriptor()
	return s.recoverReliableQueueProcessing(ctx, reliableQueueRecoveryOptions{
		SourceKey:     descriptor.SourceKey,
		ProcessingKey: descriptor.ProcessingKey,
		DeadLetterKey: descriptor.DeadLetterKey,
		StaleAfter:    staleAfter,
		MaxAttempts:   maxAttempts,
	})
}

func (s *RedisStore) EnqueueWorkContactSync(ctx context.Context, event dashboard.WorkContactSyncEvent) error {
	return s.enqueueReliableQueueItem(ctx, dashboard.WorkContactSyncQueueDescriptor(), event, dashboard.WorkContactSyncIdempotencyKey(event))
}

func (s *RedisStore) DequeueWorkContactSync(ctx context.Context, timeout time.Duration) (dashboard.WorkContactSyncDelivery, bool, error) {
	descriptor := dashboard.WorkContactSyncQueueDescriptor()
	raw, err := s.client.BLMove(ctx, descriptor.SourceKey, descriptor.ProcessingKey, "LEFT", "RIGHT", timeout).Result()
	if err == redis.Nil {
		return dashboard.WorkContactSyncDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.WorkContactSyncDelivery{}, false, err
	}
	var event dashboard.WorkContactSyncEvent
	attempts, err := decodeReliableQueuePayload(raw, &event)
	if err != nil {
		_ = s.moveMalformedQueueItem(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, raw, err.Error())
		return dashboard.WorkContactSyncDelivery{}, false, err
	}
	markedRaw, err := s.markReliableQueueProcessing(ctx, descriptor.ProcessingKey, raw, event, attempts)
	if err != nil {
		return dashboard.WorkContactSyncDelivery{}, false, err
	}
	return dashboard.WorkContactSyncDelivery{Event: event, Raw: markedRaw, Attempts: attempts}, true, nil
}

func (s *RedisStore) AckWorkContactSync(ctx context.Context, delivery dashboard.WorkContactSyncDelivery) error {
	return s.ackReliableQueueItem(ctx, dashboard.WorkContactSyncQueueDescriptor().ProcessingKey, delivery.Raw)
}

func (s *RedisStore) RetryWorkContactSync(ctx context.Context, delivery dashboard.WorkContactSyncDelivery, reason string, maxAttempts int) (bool, error) {
	descriptor := dashboard.WorkContactSyncQueueDescriptor()
	return s.retryReliableQueueItem(ctx, reliableQueueRetryOptions{
		SourceKey:      descriptor.SourceKey,
		ProcessingKey:  descriptor.ProcessingKey,
		DeadLetterKey:  descriptor.DeadLetterKey,
		Raw:            delivery.Raw,
		Event:          delivery.Event,
		CurrentAttempt: delivery.Attempts,
		Reason:         reason,
		MaxAttempts:    maxAttempts,
	})
}

func (s *RedisStore) RecoverWorkContactSyncProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	descriptor := dashboard.WorkContactSyncQueueDescriptor()
	return s.recoverReliableQueueProcessing(ctx, reliableQueueRecoveryOptions{
		SourceKey:     descriptor.SourceKey,
		ProcessingKey: descriptor.ProcessingKey,
		DeadLetterKey: descriptor.DeadLetterKey,
		StaleAfter:    staleAfter,
		MaxAttempts:   maxAttempts,
	})
}

func (s *RedisStore) EnqueueWorkDepartmentList(ctx context.Context, event dashboard.WorkDepartmentListEvent) error {
	return s.enqueueReliableQueueItem(ctx, dashboard.WorkDepartmentListQueueDescriptor(), event, dashboard.WorkDepartmentListIdempotencyKey(event))
}

func (s *RedisStore) DequeueWorkDepartmentList(ctx context.Context, timeout time.Duration) (dashboard.WorkDepartmentListDelivery, bool, error) {
	descriptor := dashboard.WorkDepartmentListQueueDescriptor()
	raw, err := s.client.BLMove(ctx, descriptor.SourceKey, descriptor.ProcessingKey, "LEFT", "RIGHT", timeout).Result()
	if err == redis.Nil {
		return dashboard.WorkDepartmentListDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.WorkDepartmentListDelivery{}, false, err
	}
	var event dashboard.WorkDepartmentListEvent
	attempts, err := decodeReliableQueuePayload(raw, &event)
	if err != nil {
		_ = s.moveMalformedQueueItem(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, raw, err.Error())
		return dashboard.WorkDepartmentListDelivery{}, false, err
	}
	markedRaw, err := s.markReliableQueueProcessing(ctx, descriptor.ProcessingKey, raw, event, attempts)
	if err != nil {
		return dashboard.WorkDepartmentListDelivery{}, false, err
	}
	return dashboard.WorkDepartmentListDelivery{Event: event, Raw: markedRaw, Attempts: attempts}, true, nil
}

func (s *RedisStore) AckWorkDepartmentList(ctx context.Context, delivery dashboard.WorkDepartmentListDelivery) error {
	return s.ackReliableQueueItem(ctx, dashboard.WorkDepartmentListQueueDescriptor().ProcessingKey, delivery.Raw)
}

func (s *RedisStore) RetryWorkDepartmentList(ctx context.Context, delivery dashboard.WorkDepartmentListDelivery, reason string, maxAttempts int) (bool, error) {
	descriptor := dashboard.WorkDepartmentListQueueDescriptor()
	return s.retryReliableQueueItem(ctx, reliableQueueRetryOptions{
		SourceKey:      descriptor.SourceKey,
		ProcessingKey:  descriptor.ProcessingKey,
		DeadLetterKey:  descriptor.DeadLetterKey,
		Raw:            delivery.Raw,
		Event:          delivery.Event,
		CurrentAttempt: delivery.Attempts,
		Reason:         reason,
		MaxAttempts:    maxAttempts,
	})
}

func (s *RedisStore) RecoverWorkDepartmentListProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	descriptor := dashboard.WorkDepartmentListQueueDescriptor()
	return s.recoverReliableQueueProcessing(ctx, reliableQueueRecoveryOptions{
		SourceKey:     descriptor.SourceKey,
		ProcessingKey: descriptor.ProcessingKey,
		DeadLetterKey: descriptor.DeadLetterKey,
		StaleAfter:    staleAfter,
		MaxAttempts:   maxAttempts,
	})
}

func (s *RedisStore) EnqueueMediumMediaIDUpdate(ctx context.Context, event dashboard.MediumMediaIDUpdateEvent) error {
	return s.enqueueReliableQueueItem(ctx, dashboard.MediumMediaIDUpdateQueueDescriptor(), event, dashboard.MediumMediaIDUpdateIdempotencyKey(event))
}

func (s *RedisStore) DequeueMediumMediaIDUpdate(ctx context.Context, timeout time.Duration) (dashboard.MediumMediaIDUpdateDelivery, bool, error) {
	descriptor := dashboard.MediumMediaIDUpdateQueueDescriptor()
	raw, err := s.client.BLMove(ctx, descriptor.SourceKey, descriptor.ProcessingKey, "LEFT", "RIGHT", timeout).Result()
	if err == redis.Nil {
		return dashboard.MediumMediaIDUpdateDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.MediumMediaIDUpdateDelivery{}, false, err
	}
	var event dashboard.MediumMediaIDUpdateEvent
	attempts, err := decodeReliableQueuePayload(raw, &event)
	if err != nil {
		_ = s.moveMalformedQueueItem(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, raw, err.Error())
		return dashboard.MediumMediaIDUpdateDelivery{}, false, err
	}
	markedRaw, err := s.markReliableQueueProcessing(ctx, descriptor.ProcessingKey, raw, event, attempts)
	if err != nil {
		return dashboard.MediumMediaIDUpdateDelivery{}, false, err
	}
	return dashboard.MediumMediaIDUpdateDelivery{Event: event, Raw: markedRaw, Attempts: attempts}, true, nil
}

func (s *RedisStore) AckMediumMediaIDUpdate(ctx context.Context, delivery dashboard.MediumMediaIDUpdateDelivery) error {
	return s.ackReliableQueueItem(ctx, dashboard.MediumMediaIDUpdateQueueDescriptor().ProcessingKey, delivery.Raw)
}

func (s *RedisStore) RetryMediumMediaIDUpdate(ctx context.Context, delivery dashboard.MediumMediaIDUpdateDelivery, reason string, maxAttempts int) (bool, error) {
	descriptor := dashboard.MediumMediaIDUpdateQueueDescriptor()
	return s.retryReliableQueueItem(ctx, reliableQueueRetryOptions{
		SourceKey:      descriptor.SourceKey,
		ProcessingKey:  descriptor.ProcessingKey,
		DeadLetterKey:  descriptor.DeadLetterKey,
		Raw:            delivery.Raw,
		Event:          delivery.Event,
		CurrentAttempt: delivery.Attempts,
		Reason:         reason,
		MaxAttempts:    maxAttempts,
	})
}

func (s *RedisStore) RecoverMediumMediaIDUpdateProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	descriptor := dashboard.MediumMediaIDUpdateQueueDescriptor()
	return s.recoverReliableQueueProcessing(ctx, reliableQueueRecoveryOptions{
		SourceKey:     descriptor.SourceKey,
		ProcessingKey: descriptor.ProcessingKey,
		DeadLetterKey: descriptor.DeadLetterKey,
		StaleAfter:    staleAfter,
		MaxAttempts:   maxAttempts,
	})
}

func (s *RedisStore) EnqueueEmployeeStatisticApply(ctx context.Context, event dashboard.EmployeeStatisticApplyEvent) error {
	return s.enqueueReliableQueueItem(ctx, dashboard.EmployeeStatisticApplyQueueDescriptor(), event, dashboard.EmployeeStatisticApplyIdempotencyKey(event))
}

func (s *RedisStore) DequeueEmployeeStatisticApply(ctx context.Context, timeout time.Duration) (dashboard.EmployeeStatisticApplyDelivery, bool, error) {
	descriptor := dashboard.EmployeeStatisticApplyQueueDescriptor()
	raw, err := s.client.BLMove(ctx, descriptor.SourceKey, descriptor.ProcessingKey, "LEFT", "RIGHT", timeout).Result()
	if err == redis.Nil {
		return dashboard.EmployeeStatisticApplyDelivery{}, false, nil
	}
	if err != nil {
		return dashboard.EmployeeStatisticApplyDelivery{}, false, err
	}
	var event dashboard.EmployeeStatisticApplyEvent
	attempts, err := decodeReliableQueuePayload(raw, &event)
	if err != nil {
		_ = s.moveMalformedQueueItem(ctx, descriptor.ProcessingKey, descriptor.DeadLetterKey, raw, err.Error())
		return dashboard.EmployeeStatisticApplyDelivery{}, false, err
	}
	markedRaw, err := s.markReliableQueueProcessing(ctx, descriptor.ProcessingKey, raw, event, attempts)
	if err != nil {
		return dashboard.EmployeeStatisticApplyDelivery{}, false, err
	}
	return dashboard.EmployeeStatisticApplyDelivery{Event: event, Raw: markedRaw, Attempts: attempts}, true, nil
}

func (s *RedisStore) AckEmployeeStatisticApply(ctx context.Context, delivery dashboard.EmployeeStatisticApplyDelivery) error {
	return s.ackReliableQueueItem(ctx, dashboard.EmployeeStatisticApplyQueueDescriptor().ProcessingKey, delivery.Raw)
}

func (s *RedisStore) RetryEmployeeStatisticApply(ctx context.Context, delivery dashboard.EmployeeStatisticApplyDelivery, reason string, maxAttempts int) (bool, error) {
	descriptor := dashboard.EmployeeStatisticApplyQueueDescriptor()
	return s.retryReliableQueueItem(ctx, reliableQueueRetryOptions{
		SourceKey:      descriptor.SourceKey,
		ProcessingKey:  descriptor.ProcessingKey,
		DeadLetterKey:  descriptor.DeadLetterKey,
		Raw:            delivery.Raw,
		Event:          delivery.Event,
		CurrentAttempt: delivery.Attempts,
		Reason:         reason,
		MaxAttempts:    maxAttempts,
	})
}

func (s *RedisStore) RecoverEmployeeStatisticApplyProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error) {
	descriptor := dashboard.EmployeeStatisticApplyQueueDescriptor()
	return s.recoverReliableQueueProcessing(ctx, reliableQueueRecoveryOptions{
		SourceKey:     descriptor.SourceKey,
		ProcessingKey: descriptor.ProcessingKey,
		DeadLetterKey: descriptor.DeadLetterKey,
		StaleAfter:    staleAfter,
		MaxAttempts:   maxAttempts,
	})
}

func (s *RedisStore) GetOperationSessionValue(ctx context.Context, sessionID string, key string) (map[string]any, bool, error) {
	raw, err := s.client.HGet(ctx, operationSessionRedisKey(sessionID), key).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (s *RedisStore) SetOperationSessionValue(ctx context.Context, sessionID string, key string, value map[string]any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	redisKey := operationSessionRedisKey(sessionID)
	if err := s.client.HSet(ctx, redisKey, key, raw).Err(); err != nil {
		return err
	}
	return s.client.Expire(ctx, redisKey, ttl).Err()
}

func operationSessionRedisKey(sessionID string) string {
	return "mochat-go:operation-session:" + sessionID
}

func employeeStatisticRedisKey(corpID int, employeeID int, startUnix int64) string {
	return fmt.Sprintf("EMPLOYEE_STATISTICS_APPLY_%d%d%d", corpID, employeeID, startUnix)
}

func workContactWelcomeStatusRedisKey(contactID int) string {
	return fmt.Sprintf("contact:welcome_status:%d", contactID)
}

type reliableQueueEnvelope struct {
	Queue               string          `json:"queue,omitempty"`
	PayloadType         string          `json:"payloadType,omitempty"`
	IdempotencyKey      string          `json:"idempotencyKey,omitempty"`
	QueueTicket         string          `json:"queueTicket,omitempty"`
	EnqueuedAt          string          `json:"enqueuedAt,omitempty"`
	Payload             json.RawMessage `json:"payload"`
	Attempts            int             `json:"attempts"`
	LastError           string          `json:"lastError,omitempty"`
	LastFailedAt        string          `json:"lastFailedAt,omitempty"`
	ProcessingStartedAt string          `json:"processingStartedAt,omitempty"`
}

type reliableQueueMalformedEnvelope struct {
	Raw            string `json:"raw"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	QueueTicket    string `json:"queueTicket,omitempty"`
	Attempts       int    `json:"attempts"`
	LastError      string `json:"lastError,omitempty"`
	LastFailedAt   string `json:"lastFailedAt,omitempty"`
}

type reliableQueueIdentity struct {
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	QueueTicket    string `json:"queueTicket,omitempty"`
}

type reliableQueueRetryOptions struct {
	SourceKey      string
	ProcessingKey  string
	DeadLetterKey  string
	Raw            string
	Event          any
	CurrentAttempt int
	Reason         string
	MaxAttempts    int
}

type reliableQueueRecoveryOptions struct {
	SourceKey     string
	ProcessingKey string
	DeadLetterKey string
	StaleAfter    time.Duration
	MaxAttempts   int
}

func (s *RedisStore) enqueueReliableQueueItem(ctx context.Context, descriptor dashboard.QueuePayloadDescriptor, event any, idempotencyKey string) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	envelope := reliableQueueEnvelope{
		Queue:          descriptor.Name,
		PayloadType:    descriptor.PayloadType,
		IdempotencyKey: idempotencyKey,
		EnqueuedAt:     time.Now().Format(time.RFC3339),
		Payload:        payload,
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	if idempotencyKey == "" {
		return s.client.RPush(ctx, descriptor.SourceKey, raw).Err()
	}
	ttlSeconds := int(descriptor.IdempotencyTTL.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = int((10 * time.Minute).Seconds())
	}
	const script = `
local ok = redis.call("SET", KEYS[2], "1", "NX", "EX", ARGV[2])
if ok then
  redis.call("RPUSH", KEYS[1], ARGV[1])
  return 1
end
return 0
`
	return s.client.Eval(ctx, script, []string{descriptor.SourceKey, idempotencyKey}, string(raw), ttlSeconds).Err()
}

func decodeReliableQueuePayload(raw string, out any) (int, error) {
	var envelope reliableQueueEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err == nil && len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, out); err != nil {
			return envelope.Attempts, err
		}
		return envelope.Attempts, nil
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return 0, err
	}
	return 0, nil
}

func decodeReliableQueueEnvelope(raw string) (reliableQueueEnvelope, bool) {
	var envelope reliableQueueEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || len(envelope.Payload) == 0 {
		return reliableQueueEnvelope{}, false
	}
	return envelope, true
}

func reliableQueuePayload(raw string, event any) (json.RawMessage, error) {
	if envelope, ok := decodeReliableQueueEnvelope(raw); ok && json.Valid(envelope.Payload) {
		return append(json.RawMessage(nil), envelope.Payload...), nil
	}
	if json.Valid([]byte(raw)) {
		return append(json.RawMessage(nil), []byte(raw)...), nil
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func reliableQueueEnvelopeForEvent(raw string, event any) (reliableQueueEnvelope, error) {
	if envelope, ok := decodeReliableQueueEnvelope(raw); ok && json.Valid(envelope.Payload) {
		return envelope, nil
	}
	payload, err := reliableQueuePayload(raw, event)
	if err != nil {
		return reliableQueueEnvelope{}, err
	}
	return reliableQueueEnvelope{Payload: payload}, nil
}

func (s *RedisStore) markReliableQueueProcessing(ctx context.Context, processingKey string, raw string, event any, attempts int) (string, error) {
	envelope, ok := decodeReliableQueueEnvelope(raw)
	if !ok {
		payload, err := reliableQueuePayload(raw, event)
		if err != nil {
			return "", err
		}
		envelope = reliableQueueEnvelope{Payload: payload, Attempts: attempts}
	}
	envelope.ProcessingStartedAt = time.Now().Format(time.RFC3339)
	nextRaw, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	if err := s.moveReliableQueueItem(ctx, processingKey, processingKey, raw, string(nextRaw)); err != nil {
		return "", err
	}
	return string(nextRaw), nil
}

func (s *RedisStore) ackReliableQueueItem(ctx context.Context, processingKey string, raw string) error {
	removed, err := s.client.LRem(ctx, processingKey, 1, raw).Result()
	if err != nil {
		return err
	}
	if removed == 0 {
		return fmt.Errorf("queue delivery is not in processing list")
	}
	return nil
}

const employeeApplyAckScript = `
local removed = redis.call("LREM", KEYS[1], 1, ARGV[1])
if removed > 0 and ARGV[2] ~= "" and ARGV[3] ~= "" then
  local current = redis.call("GET", ARGV[2])
  if current == ARGV[3] then
    redis.call("DEL", ARGV[2])
  end
end
return removed
`

func (s *RedisStore) ackEmployeeApplyQueueItem(ctx context.Context, processingKey string, raw string) error {
	idempotencyKey := ""
	queueTicket := ""
	if envelope, ok := decodeReliableQueueEnvelope(raw); ok {
		idempotencyKey = envelope.IdempotencyKey
		queueTicket = envelope.QueueTicket
	}
	removed, err := s.client.Eval(ctx, employeeApplyAckScript, []string{processingKey}, raw, idempotencyKey, queueTicket).Int()
	if err != nil {
		return err
	}
	if removed == 0 {
		return fmt.Errorf("queue delivery is not in processing list")
	}
	return nil
}

func (s *RedisStore) retryReliableQueueItem(ctx context.Context, opts reliableQueueRetryOptions) (bool, error) {
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	attempts := opts.CurrentAttempt + 1
	envelope, err := reliableQueueEnvelopeForEvent(opts.Raw, opts.Event)
	if err != nil {
		return false, err
	}
	envelope.Attempts = attempts
	envelope.LastError = opts.Reason
	envelope.LastFailedAt = time.Now().Format(time.RFC3339)
	envelope.ProcessingStartedAt = ""
	nextRaw, err := json.Marshal(envelope)
	if err != nil {
		return false, err
	}
	targetKey := opts.SourceKey
	deadLettered := false
	if attempts >= maxAttempts {
		targetKey = opts.DeadLetterKey
		deadLettered = true
	}
	var moveErr error
	if deadLettered && envelope.IdempotencyKey != "" {
		moveErr = s.moveReliableQueueItemAndReleaseIdempotency(ctx, opts.ProcessingKey, targetKey, opts.Raw, string(nextRaw), envelope.IdempotencyKey, envelope.QueueTicket)
	} else {
		moveErr = s.moveReliableQueueItem(ctx, opts.ProcessingKey, targetKey, opts.Raw, string(nextRaw))
	}
	if moveErr != nil {
		return false, moveErr
	}
	return deadLettered, nil
}

func (s *RedisStore) recoverReliableQueueProcessing(ctx context.Context, opts reliableQueueRecoveryOptions) (int, error) {
	staleAfter := opts.StaleAfter
	if staleAfter <= 0 {
		staleAfter = 5 * time.Minute
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	rawItems, err := s.client.LRange(ctx, opts.ProcessingKey, 0, -1).Result()
	if err != nil {
		return 0, err
	}
	recovered := 0
	now := time.Now()
	for _, raw := range rawItems {
		envelope, ok := decodeReliableQueueEnvelope(raw)
		if !ok {
			if !json.Valid([]byte(raw)) {
				if err := s.moveMalformedQueueItem(ctx, opts.ProcessingKey, opts.DeadLetterKey, raw, "processing item is not valid JSON"); err != nil {
					return recovered, err
				}
				recovered++
				continue
			}
			envelope = reliableQueueEnvelope{Payload: append(json.RawMessage(nil), []byte(raw)...)}
		}
		if envelope.ProcessingStartedAt != "" {
			startedAt, err := time.Parse(time.RFC3339, envelope.ProcessingStartedAt)
			if err == nil && now.Sub(startedAt) < staleAfter {
				continue
			}
		}
		envelope.Attempts++
		envelope.LastError = "processing timeout recovered"
		envelope.LastFailedAt = now.Format(time.RFC3339)
		envelope.ProcessingStartedAt = ""
		nextRaw, err := json.Marshal(envelope)
		if err != nil {
			return recovered, err
		}
		targetKey := opts.SourceKey
		if envelope.Attempts >= maxAttempts {
			targetKey = opts.DeadLetterKey
		}
		var moveErr error
		if envelope.Attempts >= maxAttempts && envelope.IdempotencyKey != "" {
			moveErr = s.moveReliableQueueItemAndReleaseIdempotency(ctx, opts.ProcessingKey, targetKey, raw, string(nextRaw), envelope.IdempotencyKey, envelope.QueueTicket)
		} else {
			moveErr = s.moveReliableQueueItem(ctx, opts.ProcessingKey, targetKey, raw, string(nextRaw))
		}
		if moveErr != nil {
			return recovered, moveErr
		}
		recovered++
	}
	return recovered, nil
}

func (s *RedisStore) moveMalformedQueueItem(ctx context.Context, processingKey string, deadLetterKey string, raw string, reason string) error {
	idempotencyKey := ""
	queueTicket := ""
	var identity reliableQueueIdentity
	if err := json.Unmarshal([]byte(raw), &identity); err == nil {
		idempotencyKey = identity.IdempotencyKey
		queueTicket = identity.QueueTicket
	}
	nextRaw, err := json.Marshal(reliableQueueMalformedEnvelope{
		Raw:            raw,
		IdempotencyKey: idempotencyKey,
		QueueTicket:    queueTicket,
		Attempts:       1,
		LastError:      reason,
		LastFailedAt:   time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	if idempotencyKey != "" {
		return s.moveReliableQueueItemAndReleaseIdempotency(ctx, processingKey, deadLetterKey, raw, string(nextRaw), idempotencyKey, queueTicket)
	}
	return s.moveReliableQueueItem(ctx, processingKey, deadLetterKey, raw, string(nextRaw))
}

func (s *RedisStore) moveReliableQueueItemAndReleaseIdempotency(ctx context.Context, processingKey string, targetKey string, raw string, nextRaw string, idempotencyKey string, queueTicket string) error {
	const script = `
local removed = redis.call("LREM", KEYS[1], 1, ARGV[1])
if removed > 0 then
  redis.call("RPUSH", KEYS[2], ARGV[2])
  if KEYS[3] ~= "" and ARGV[3] ~= "" then
    local current = redis.call("GET", KEYS[3])
    if current == ARGV[3] then
      redis.call("DEL", KEYS[3])
    end
  end
end
return removed
`
	removed, err := s.client.Eval(ctx, script, []string{processingKey, targetKey, idempotencyKey}, raw, nextRaw, queueTicket).Int()
	if err != nil {
		return err
	}
	if removed == 0 {
		return fmt.Errorf("queue delivery is not in processing list")
	}
	return nil
}

func (s *RedisStore) moveReliableQueueItem(ctx context.Context, processingKey string, targetKey string, raw string, nextRaw string) error {
	const script = `
local removed = redis.call("LREM", KEYS[1], 1, ARGV[1])
if removed > 0 then
  redis.call("RPUSH", KEYS[2], ARGV[2])
end
return removed
`
	removed, err := s.client.Eval(ctx, script, []string{processingKey, targetKey}, raw, nextRaw).Int()
	if err != nil {
		return err
	}
	if removed == 0 {
		return fmt.Errorf("queue delivery is not in processing list")
	}
	return nil
}
