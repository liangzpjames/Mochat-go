package dashboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrWeWorkCallbackConflict     = errors.New("wework callback event key conflicts with a different payload")
	ErrWeWorkCallbackLeaseLost    = errors.New("wework callback lease is no longer owned")
	weWorkCallbackURLQueryPattern = regexp.MustCompile(`(?i)(https?://[^\s"'?]+)\?[^\s"']+`)
	weWorkCallbackSecretPattern   = regexp.MustCompile(`(?i)(access[_-]?token|token|secret|signature|nonce|authorization|password)(\s*[:=]\s*)("[^"]*"|'[^']*'|[^&\s,;}]+)`)
)

type WeWorkCallbackInboxStore interface {
	AcceptWeWorkCallback(ctx context.Context, event WeWorkCallbackEvent, eventKey string, fingerprint string) (replayed bool, err error)
}

type WeWorkCallbackWakeup interface {
	WakeWeWorkCallback(ctx context.Context) error
}

type WeWorkCallbackClaim struct {
	ID                 int64
	EventKey           string
	PayloadFingerprint string
	Event              WeWorkCallbackEvent
	LeaseToken         string
	LeaseFence         uint64
	Attempt            int
}

type WeWorkCallbackInbox interface {
	ClaimWeWorkCallback(ctx context.Context, leaseDuration time.Duration) (WeWorkCallbackClaim, bool, error)
	ValidateWeWorkCallbackClaim(ctx context.Context, claim WeWorkCallbackClaim) error
	CompleteWeWorkCallback(ctx context.Context, claim WeWorkCallbackClaim) error
	FailWeWorkCallback(ctx context.Context, claim WeWorkCallbackClaim, reason string, maxAttempts int, retryDelay time.Duration) (deadLettered bool, err error)
}

type weWorkCallbackExecutionContextKey struct{}

type WeWorkCallbackExecution struct {
	EventKey   string
	LeaseFence uint64
}

func withWeWorkCallbackExecution(ctx context.Context, claim WeWorkCallbackClaim) context.Context {
	return context.WithValue(ctx, weWorkCallbackExecutionContextKey{}, WeWorkCallbackExecution{EventKey: claim.EventKey, LeaseFence: claim.LeaseFence})
}

func WeWorkCallbackExecutionFromContext(ctx context.Context) (WeWorkCallbackExecution, bool) {
	if ctx == nil {
		return WeWorkCallbackExecution{}, false
	}
	execution, ok := ctx.Value(weWorkCallbackExecutionContextKey{}).(WeWorkCallbackExecution)
	return execution, ok && strings.TrimSpace(execution.EventKey) != "" && execution.LeaseFence > 0
}

// SanitizeWeWorkCallbackFailure prevents Provider URL query credentials from
// crossing the durable inbox, task execution, or logging boundary.
func SanitizeWeWorkCallbackFailure(value string) string {
	value = strings.TrimSpace(value)
	value = weWorkCallbackURLQueryPattern.ReplaceAllString(value, `${1}?[REDACTED]`)
	value = weWorkCallbackSecretPattern.ReplaceAllString(value, `${1}${2}[REDACTED]`)
	return value
}

func WeWorkCallbackEventKey(event WeWorkCallbackEvent) string {
	identity := map[string]string{}
	if providerEventID := queueMessageString(event.Message,
		"MsgId", "MsgID", "msgid", "msg_id",
		"EventId", "EventID", "eventid", "event_id",
	); providerEventID != "" {
		identity["providerEventId"] = providerEventID
	} else {
		// WeCom does not provide a unique event id for every callback family.
		// In that case the complete normalized payload is the only collision-safe
		// replay identity: exact retries stay stable while two legitimate changes
		// for the same resource and second remain distinct.
		identity = normalizedWeWorkCallbackMessage(event.Message)
	}
	return weWorkCallbackDigest(struct {
		TenantID  int               `json:"tenantId"`
		CorpID    int               `json:"corpId"`
		WxCorpID  string            `json:"wxCorpId"`
		EventPath string            `json:"eventPath"`
		Identity  map[string]string `json:"identity"`
	}{
		TenantID: event.TenantID, CorpID: event.CorpID, WxCorpID: strings.TrimSpace(event.WxCorpID),
		EventPath: strings.TrimSpace(event.EventPath), Identity: identity,
	})
}

func WeWorkCallbackPayloadFingerprint(event WeWorkCallbackEvent) string {
	return weWorkCallbackDigest(struct {
		TenantID  int               `json:"tenantId"`
		CorpID    int               `json:"corpId"`
		WxCorpID  string            `json:"wxCorpId"`
		EventPath string            `json:"eventPath"`
		Message   map[string]string `json:"message"`
	}{
		TenantID: event.TenantID, CorpID: event.CorpID, WxCorpID: strings.TrimSpace(event.WxCorpID),
		EventPath: strings.TrimSpace(event.EventPath), Message: normalizedWeWorkCallbackMessage(event.Message),
	})
}

func normalizedWeWorkCallbackMessage(message map[string]string) map[string]string {
	keys := make([]string, 0, len(message))
	for key := range message {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	normalized := make(map[string]string, len(keys))
	for _, key := range keys {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		normalized[name] = strings.TrimSpace(message[key])
	}
	return normalized
}

func weWorkCallbackDigest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
