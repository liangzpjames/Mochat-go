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
	ErrWeWorkCallbackConflict             = errors.New("wework callback event key conflicts with a different payload")
	ErrWeWorkCallbackLeaseLost            = errors.New("wework callback lease is no longer owned")
	ErrLegacyWeWorkCallbackDeadBacklog    = errors.New("legacy wework callback dead-letter backlog requires operator review")
	ErrLegacyWeWorkCallbackSourceMismatch = errors.New("legacy wework callback source does not match durable cutover marker")
	ErrLegacyWeWorkCallbackAlreadyRunning = errors.New("legacy wework callback cutover is already running")
	ErrLegacyWeWorkCallbackOwnerMismatch  = errors.New("legacy wework callback cutover owner does not match durable marker")
	weWorkCallbackURLQueryPattern         = regexp.MustCompile(`(?i)(https?://[^\s"'?]+)\?[^\s"']+`)
	weWorkCallbackQuotedSecretPattern     = regexp.MustCompile(`(?i)("(?:access[_-]?token|token|secret|signature|nonce|authorization|password)"\s*:\s*")[^"]*(")`)
	weWorkCallbackAuthorizationPattern    = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*["']?)(bearer|basic)\s+[^"'\s,;}]+`)
	weWorkCallbackCredentialSchemePattern = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[^"'\s,;}]+`)
	weWorkCallbackAssignedSecretPattern   = regexp.MustCompile(`(?i)((?:access[_-]?token|token|secret|signature|nonce|authorization|password)\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^,;}\r\n]+)`)
)

const LegacyWeWorkCallbackCutoverName = "legacy-redis-v1"

const (
	WeWorkCallbackSideEffectPending = "pending"
	WeWorkCallbackSideEffectUnknown = "unknown"
	WeWorkCallbackSideEffectSent    = "sent"

	WeWorkCallbackActionFissionEmployeeReminder = "fission.employee_reminder"
	WeWorkCallbackActionFissionCustomerPush     = "fission.customer_push"
)

type WeWorkCallbackInboxStore interface {
	AcceptWeWorkCallback(ctx context.Context, event WeWorkCallbackEvent, eventKey string, fingerprint string) (replayed bool, err error)
}

type WeWorkCallbackWakeup interface {
	WakeWeWorkCallback(ctx context.Context) error
}

type WeWorkCallbackClaim struct {
	ID                   int64
	EventKey             string
	PayloadFingerprint   string
	Event                WeWorkCallbackEvent
	LeaseToken           string
	LeaseFence           uint64
	Attempt              int
	DependencyDeferCount int
}

type WeWorkCallbackInbox interface {
	WeWorkCallbackInboxStore
	ClaimWeWorkCallback(ctx context.Context, leaseDuration time.Duration, maxAttempts int) (WeWorkCallbackClaim, bool, error)
	ValidateWeWorkCallbackClaim(ctx context.Context, claim WeWorkCallbackClaim) error
	CompleteWeWorkCallback(ctx context.Context, claim WeWorkCallbackClaim) error
	DeferWeWorkCallbackDependency(ctx context.Context, claim WeWorkCallbackClaim, reason string, retryDelay time.Duration) error
	FailWeWorkCallback(ctx context.Context, claim WeWorkCallbackClaim, reason string, maxAttempts int, retryDelay time.Duration) (deadLettered bool, err error)
}

type WeWorkCallbackSideEffectStore interface {
	BeginWeWorkCallbackSideEffect(ctx context.Context, execution WeWorkCallbackExecution, actionKey, payloadHash string) (execute bool, status string, err error)
	CompleteWeWorkCallbackSideEffect(ctx context.Context, execution WeWorkCallbackExecution, actionKey, payloadHash string) error
}

type LegacyWeWorkCallbackBacklogStats struct {
	Pending    int64
	Processing int64
	Dead       int64
}

type LegacyWeWorkCallbackDelivery struct {
	Event WeWorkCallbackEvent
	Raw   string
}

// LegacyWeWorkCallbackBacklog is a one-way cutover capability. HTTP callback
// ACK never uses it; it only imports already-ACKed Redis backlog into MySQL.
type LegacyWeWorkCallbackBacklog interface {
	PreflightLegacyWeWorkCallbackBacklog(ctx context.Context) (LegacyWeWorkCallbackBacklogStats, error)
	NextLegacyWeWorkCallback(ctx context.Context) (LegacyWeWorkCallbackDelivery, bool, error)
	AckLegacyWeWorkCallback(ctx context.Context, delivery LegacyWeWorkCallbackDelivery) error
}

type LegacyWeWorkCallbackImportStore interface {
	WeWorkCallbackInboxStore
	TenantIDByCorpID(ctx context.Context, corpID int) (int, error)
}

type LegacyWeWorkCallbackCutoverStore interface {
	LegacyWeWorkCallbackImportStore
	WeWorkCallbackLegacyCutover(ctx context.Context) (LegacyWeWorkCallbackCutover, error)
	BeginWeWorkCallbackLegacyCutover(ctx context.Context, sourceFingerprint, ownerToken string) error
	CompleteWeWorkCallbackLegacyCutover(ctx context.Context, sourceFingerprint, ownerToken string, imported int) error
	FailWeWorkCallbackLegacyCutover(ctx context.Context, sourceFingerprint, ownerToken string, imported int, reason string) error
}

type LegacyWeWorkCallbackCutover struct {
	Status            string
	SourceFingerprint string
	OwnerToken        string
	ImportedCount     int
}

type weWorkCallbackExecutionContextKey struct{}

type WeWorkCallbackExecution struct {
	TenantID   int
	CorpID     int
	EventKey   string
	LeaseToken string
	LeaseFence uint64
}

func withWeWorkCallbackExecution(ctx context.Context, claim WeWorkCallbackClaim) context.Context {
	return context.WithValue(ctx, weWorkCallbackExecutionContextKey{}, WeWorkCallbackExecution{
		TenantID: claim.Event.TenantID, CorpID: claim.Event.CorpID,
		EventKey: claim.EventKey, LeaseToken: claim.LeaseToken, LeaseFence: claim.LeaseFence,
	})
}

func WeWorkCallbackExecutionFromContext(ctx context.Context) (WeWorkCallbackExecution, bool) {
	if ctx == nil {
		return WeWorkCallbackExecution{}, false
	}
	execution, ok := ctx.Value(weWorkCallbackExecutionContextKey{}).(WeWorkCallbackExecution)
	return execution, ok && strings.TrimSpace(execution.EventKey) != "" && strings.TrimSpace(execution.LeaseToken) != "" && execution.LeaseFence > 0
}

func WeWorkCallbackSideEffectPayloadHash(actionKey string, payload any) (string, error) {
	actionKey = strings.TrimSpace(actionKey)
	if actionKey == "" || payload == nil {
		return "", errors.New("wework callback side effect payload is invalid")
	}
	raw, err := json.Marshal(struct {
		ActionKey string `json:"actionKey"`
		Payload   any    `json:"payload"`
	}{ActionKey: actionKey, Payload: payload})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// SanitizeWeWorkCallbackFailure prevents Provider URL query credentials from
// crossing the durable inbox, task execution, or logging boundary.
func SanitizeWeWorkCallbackFailure(value string) string {
	value = strings.TrimSpace(value)
	value = weWorkCallbackURLQueryPattern.ReplaceAllString(value, `${1}?[REDACTED]`)
	value = weWorkCallbackQuotedSecretPattern.ReplaceAllString(value, `${1}[REDACTED]${2}`)
	value = weWorkCallbackAuthorizationPattern.ReplaceAllString(value, `${1}${2} [REDACTED]`)
	value = weWorkCallbackCredentialSchemePattern.ReplaceAllString(value, `${1} [REDACTED]`)
	value = weWorkCallbackAssignedSecretPattern.ReplaceAllString(value, `${1}[REDACTED]`)
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
