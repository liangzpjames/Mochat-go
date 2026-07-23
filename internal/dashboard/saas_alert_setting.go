package dashboard

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"jiyi/mochat-go/internal/outboundhttp"
)

const (
	defaultSaaSAlertWebhookTimeoutSeconds           = 5
	defaultSaaSAlertWebhookRetryAttempts            = 1
	defaultSaaSAlertWebhookRetryDelayMS             = 250
	defaultSaaSAlertNotificationMaxAttempts         = 3
	defaultSaaSAlertNotificationRetryDelaySeconds   = 300
	maxSaaSAlertWebhookTimeoutSeconds               = 300
	maxSaaSAlertWebhookRetryAttempts                = 10
	maxSaaSAlertWebhookRetryDelayMS                 = 600000
	maxSaaSAlertNotificationMaxAttempts             = 20
	maxSaaSAlertNotificationRetryDelaySeconds       = 86400
	defaultSaaSAlertMinimumSeverity                 = SaaSAlertSeverityWarning
	defaultSaaSAlertQuietHoursStart                 = "22:00"
	defaultSaaSAlertQuietHoursEnd                   = "08:00"
	defaultSaaSAlertTimezone                        = "Asia/Shanghai"
	maxSaaSAlertAllowedTypes                        = 20
	maxSaaSAlertHourlyLimit                         = 10000
	saaSAlertWebhookNotConfiguredErrorMessagePrefix = "SaaS alert webhook is not configured"
	SaaSAlertCredentialProtectionEmpty              = "empty"
	SaaSAlertCredentialProtectionEncrypted          = "encrypted"
	SaaSAlertCredentialProtectionLegacyPlaintext    = "legacy_plaintext"
)

type SaaSAlertSetting struct {
	ID                            int64
	TenantID                      int
	Channel                       string
	Enabled                       bool
	WebhookURL                    string
	WebhookSecret                 string
	WebhookCredentialProtection   string
	WebhookCredentialKeyID        string
	WebhookTimeoutSeconds         int
	WebhookRetryAttempts          int
	WebhookRetryDelayMS           int
	WebhookTitleTemplate          string
	WebhookBodyTemplate           string
	NotificationMaxAttempts       int
	NotificationRetryDelaySeconds int
	MinimumSeverity               string
	AllowedAlertTypes             []string
	QuietHoursEnabled             bool
	QuietHoursStart               string
	QuietHoursEnd                 string
	Timezone                      string
	HourlyLimit                   int
	CreatedAt                     string
	UpdatedAt                     string
}

type SaaSAlertSettingStore interface {
	GetSaaSAlertSetting(ctx context.Context, tenantID int, channel string) (SaaSAlertSetting, bool, error)
	SaveSaaSAlertSetting(ctx context.Context, setting SaaSAlertSetting) (SaaSAlertSetting, error)
}

type SaaSAlertSettingReader interface {
	GetSaaSAlertSetting(ctx context.Context, tenantID int, channel string) (SaaSAlertSetting, bool, error)
}

type SaaSAlertCredentialProtectionStatus struct {
	EncryptionConfigured      bool     `json:"encryptionConfigured"`
	RequireEncryption         bool     `json:"requireEncryption"`
	DedicatedConfigured       bool     `json:"dedicatedConfigured"`
	ActiveKeyID               string   `json:"activeKeyId"`
	KeyCount                  int      `json:"keyCount"`
	ConfiguredCredentialCount int      `json:"configuredCredentialCount"`
	EncryptedCredentialCount  int      `json:"encryptedCredentialCount"`
	LegacyPlaintextCount      int      `json:"legacyPlaintextCount"`
	ActiveKeyCredentialCount  int      `json:"activeKeyCredentialCount"`
	RotationRequiredCount     int      `json:"rotationRequiredCount"`
	UnavailableKeyCount       int      `json:"unavailableKeyCount"`
	DecryptFailureCount       int      `json:"decryptFailureCount"`
	UnavailableKeyIDs         []string `json:"unavailableKeyIds"`
	Healthy                   bool     `json:"healthy"`
}

type SaaSAlertCredentialRotationResult struct {
	TenantID         int    `json:"tenantId"`
	Limit            int    `json:"limit"`
	ScannedCount     int    `json:"scannedCount"`
	RotatedCount     int    `json:"rotatedCount"`
	LegacyCount      int    `json:"legacyCount"`
	ReencryptedCount int    `json:"reencryptedCount"`
	ActiveKeyID      string `json:"activeKeyId"`
}

type SaaSAlertCredentialProtectionStore interface {
	SaaSAlertCredentialProtection(ctx context.Context) (SaaSAlertCredentialProtectionStatus, error)
	RotateSaaSAlertCredentials(ctx context.Context, tenantID int, limit int) (SaaSAlertCredentialRotationResult, error)
}

func DefaultSaaSAlertSetting(tenantID int, channel string) SaaSAlertSetting {
	return NormalizeSaaSAlertSetting(SaaSAlertSetting{
		TenantID:                      tenantID,
		Channel:                       channel,
		WebhookRetryDelayMS:           defaultSaaSAlertWebhookRetryDelayMS,
		NotificationRetryDelaySeconds: defaultSaaSAlertNotificationRetryDelaySeconds,
		MinimumSeverity:               defaultSaaSAlertMinimumSeverity,
		QuietHoursStart:               defaultSaaSAlertQuietHoursStart,
		QuietHoursEnd:                 defaultSaaSAlertQuietHoursEnd,
		Timezone:                      defaultSaaSAlertTimezone,
	})
}

func NormalizeSaaSAlertSetting(setting SaaSAlertSetting) SaaSAlertSetting {
	setting.Channel = strings.TrimSpace(setting.Channel)
	if setting.Channel == "" {
		setting.Channel = SaaSAlertNotificationChannelWebhook
	}
	setting.WebhookURL = strings.TrimSpace(setting.WebhookURL)
	setting.WebhookSecret = strings.TrimSpace(setting.WebhookSecret)
	setting.WebhookCredentialProtection = strings.TrimSpace(setting.WebhookCredentialProtection)
	setting.WebhookCredentialKeyID = strings.TrimSpace(setting.WebhookCredentialKeyID)
	if setting.WebhookTimeoutSeconds <= 0 {
		setting.WebhookTimeoutSeconds = defaultSaaSAlertWebhookTimeoutSeconds
	}
	if setting.WebhookRetryAttempts <= 0 {
		setting.WebhookRetryAttempts = defaultSaaSAlertWebhookRetryAttempts
	}
	if setting.WebhookRetryDelayMS < 0 {
		setting.WebhookRetryDelayMS = defaultSaaSAlertWebhookRetryDelayMS
	}
	setting.WebhookTitleTemplate = strings.TrimSpace(setting.WebhookTitleTemplate)
	if setting.WebhookTitleTemplate == "" {
		setting.WebhookTitleTemplate = defaultSaaSAlertWebhookTitleTemplate
	}
	setting.WebhookBodyTemplate = strings.TrimSpace(setting.WebhookBodyTemplate)
	if setting.WebhookBodyTemplate == "" {
		setting.WebhookBodyTemplate = defaultSaaSAlertWebhookBodyTemplate
	}
	if setting.NotificationMaxAttempts <= 0 {
		setting.NotificationMaxAttempts = defaultSaaSAlertNotificationMaxAttempts
	}
	if setting.NotificationRetryDelaySeconds < 0 {
		setting.NotificationRetryDelaySeconds = defaultSaaSAlertNotificationRetryDelaySeconds
	}
	setting.MinimumSeverity = strings.ToLower(strings.TrimSpace(setting.MinimumSeverity))
	if setting.MinimumSeverity == "" {
		setting.MinimumSeverity = defaultSaaSAlertMinimumSeverity
	}
	setting.AllowedAlertTypes = normalizeSaaSAlertTypes(setting.AllowedAlertTypes)
	setting.QuietHoursStart = strings.TrimSpace(setting.QuietHoursStart)
	if setting.QuietHoursStart == "" {
		setting.QuietHoursStart = defaultSaaSAlertQuietHoursStart
	}
	setting.QuietHoursEnd = strings.TrimSpace(setting.QuietHoursEnd)
	if setting.QuietHoursEnd == "" {
		setting.QuietHoursEnd = defaultSaaSAlertQuietHoursEnd
	}
	setting.Timezone = strings.TrimSpace(setting.Timezone)
	if setting.Timezone == "" {
		setting.Timezone = defaultSaaSAlertTimezone
	}
	if setting.HourlyLimit < 0 {
		setting.HourlyLimit = 0
	}
	return setting
}

func ValidateSaaSAlertSetting(setting SaaSAlertSetting) error {
	setting = NormalizeSaaSAlertSetting(setting)
	if setting.TenantID <= 0 {
		return fieldError("tenantId 必传")
	}
	if setting.Channel != SaaSAlertNotificationChannelWebhook {
		return fieldError("channel 目前仅支持 webhook")
	}
	if setting.Enabled {
		if err := requireAbsoluteWebhookURL(setting.WebhookURL); err != nil {
			return err
		}
	}
	if setting.WebhookURL != "" {
		if err := requireAbsoluteWebhookURL(setting.WebhookURL); err != nil {
			return err
		}
	}
	if setting.WebhookTimeoutSeconds <= 0 || setting.WebhookTimeoutSeconds > maxSaaSAlertWebhookTimeoutSeconds {
		return fieldError("webhookTimeoutSeconds 必须在 1 到 300 之间")
	}
	if setting.WebhookRetryAttempts <= 0 || setting.WebhookRetryAttempts > maxSaaSAlertWebhookRetryAttempts {
		return fieldError("webhookRetryAttempts 必须在 1 到 10 之间")
	}
	if setting.WebhookRetryDelayMS < 0 || setting.WebhookRetryDelayMS > maxSaaSAlertWebhookRetryDelayMS {
		return fieldError("webhookRetryDelayMs 必须在 0 到 600000 之间")
	}
	if setting.NotificationMaxAttempts <= 0 || setting.NotificationMaxAttempts > maxSaaSAlertNotificationMaxAttempts {
		return fieldError("notificationMaxAttempts 必须在 1 到 20 之间")
	}
	if setting.NotificationRetryDelaySeconds < 0 || setting.NotificationRetryDelaySeconds > maxSaaSAlertNotificationRetryDelaySeconds {
		return fieldError("notificationRetryDelaySeconds 必须在 0 到 86400 之间")
	}
	if setting.MinimumSeverity != SaaSAlertSeverityWarning && setting.MinimumSeverity != SaaSAlertSeverityCritical {
		return fieldError("minimumSeverity 必须是 warning 或 critical")
	}
	if len(setting.AllowedAlertTypes) > maxSaaSAlertAllowedTypes {
		return fieldError("alertTypes 最多 20 项")
	}
	for _, alertType := range setting.AllowedAlertTypes {
		if len(alertType) > 64 || !validSaaSAlertType(alertType) {
			return fieldError("alertTypes 只能包含 1 到 64 位小写字母、数字、下划线、点或连字符")
		}
	}
	startMinutes, err := parseSaaSAlertClock(setting.QuietHoursStart)
	if err != nil {
		return fieldError("quietHoursStart 必须是 HH:MM")
	}
	endMinutes, err := parseSaaSAlertClock(setting.QuietHoursEnd)
	if err != nil {
		return fieldError("quietHoursEnd 必须是 HH:MM")
	}
	if setting.QuietHoursEnabled && startMinutes == endMinutes {
		return fieldError("quietHoursStart 和 quietHoursEnd 不能相同")
	}
	if _, err := time.LoadLocation(setting.Timezone); err != nil {
		return fieldError("timezone 必须是有效的 IANA 时区")
	}
	if setting.HourlyLimit < 0 || setting.HourlyLimit > maxSaaSAlertHourlyLimit {
		return fieldError("hourlyLimit 必须在 0 到 10000 之间")
	}
	if _, err := parseSaaSAlertWebhookTemplate("title", setting.WebhookTitleTemplate, defaultSaaSAlertWebhookTitleTemplate); err != nil {
		return fmt.Errorf("webhookTitleTemplate 必须是合法模板: %w", err)
	}
	if _, err := parseSaaSAlertWebhookTemplate("body", setting.WebhookBodyTemplate, defaultSaaSAlertWebhookBodyTemplate); err != nil {
		return fmt.Errorf("webhookBodyTemplate 必须是合法模板: %w", err)
	}
	return nil
}

func applySaaSAlertSettingParams(setting SaaSAlertSetting, params map[string]any) (SaaSAlertSetting, error) {
	if enabled, found, err := boolParam(params, "enabled"); err != nil {
		return SaaSAlertSetting{}, fieldError("enabled 必须是布尔值")
	} else if found {
		setting.Enabled = enabled
	}
	if _, found := params["webhookUrl"]; found {
		setting.WebhookURL = stringParam(params, "webhookUrl")
	}
	if _, found := params["webhookSecret"]; found {
		setting.WebhookSecret = stringParam(params, "webhookSecret")
	}
	if clear, found, err := boolParam(params, "clearWebhookSecret"); err != nil {
		return SaaSAlertSetting{}, fieldError("clearWebhookSecret 必须是布尔值")
	} else if found && clear {
		setting.WebhookSecret = ""
	}
	if value, found, err := intParam(params, "webhookTimeoutSeconds"); err != nil {
		return SaaSAlertSetting{}, fieldError("webhookTimeoutSeconds 必须是整数")
	} else if found {
		setting.WebhookTimeoutSeconds = value
	}
	if value, found, err := intParam(params, "webhookRetryAttempts"); err != nil {
		return SaaSAlertSetting{}, fieldError("webhookRetryAttempts 必须是整数")
	} else if found {
		setting.WebhookRetryAttempts = value
	}
	if value, found, err := intParam(params, "webhookRetryDelayMs"); err != nil {
		return SaaSAlertSetting{}, fieldError("webhookRetryDelayMs 必须是整数")
	} else if found {
		setting.WebhookRetryDelayMS = value
	}
	if _, found := params["webhookTitleTemplate"]; found {
		setting.WebhookTitleTemplate = stringParam(params, "webhookTitleTemplate")
	}
	if _, found := params["webhookBodyTemplate"]; found {
		setting.WebhookBodyTemplate = stringParam(params, "webhookBodyTemplate")
	}
	if value, found, err := intParam(params, "notificationMaxAttempts"); err != nil {
		return SaaSAlertSetting{}, fieldError("notificationMaxAttempts 必须是整数")
	} else if found {
		setting.NotificationMaxAttempts = value
	}
	if value, found, err := intParam(params, "notificationRetryDelaySeconds"); err != nil {
		return SaaSAlertSetting{}, fieldError("notificationRetryDelaySeconds 必须是整数")
	} else if found {
		setting.NotificationRetryDelaySeconds = value
	}
	if _, found := params["minimumSeverity"]; found {
		setting.MinimumSeverity = stringParam(params, "minimumSeverity")
	}
	if raw, found := params["alertTypes"]; found {
		values, err := saasAlertTypeSliceParam(raw)
		if err != nil {
			return SaaSAlertSetting{}, fieldError("alertTypes 必须是字符串数组")
		}
		setting.AllowedAlertTypes = values
	}
	if enabled, found, err := boolParam(params, "quietHoursEnabled"); err != nil {
		return SaaSAlertSetting{}, fieldError("quietHoursEnabled 必须是布尔值")
	} else if found {
		setting.QuietHoursEnabled = enabled
	}
	if _, found := params["quietHoursStart"]; found {
		setting.QuietHoursStart = stringParam(params, "quietHoursStart")
	}
	if _, found := params["quietHoursEnd"]; found {
		setting.QuietHoursEnd = stringParam(params, "quietHoursEnd")
	}
	if _, found := params["timezone"]; found {
		setting.Timezone = stringParam(params, "timezone")
	}
	if value, found, err := intParam(params, "hourlyLimit"); err != nil {
		return SaaSAlertSetting{}, fieldError("hourlyLimit 必须是整数")
	} else if found {
		setting.HourlyLimit = value
	}
	setting = NormalizeSaaSAlertSetting(setting)
	if err := ValidateSaaSAlertSetting(setting); err != nil {
		return SaaSAlertSetting{}, err
	}
	return setting, nil
}

func (s SaaSAlertSetting) WebhookTimeout() time.Duration {
	s = NormalizeSaaSAlertSetting(s)
	return time.Duration(s.WebhookTimeoutSeconds) * time.Second
}

func (s SaaSAlertSetting) WebhookRetryDelay() time.Duration {
	s = NormalizeSaaSAlertSetting(s)
	return time.Duration(s.WebhookRetryDelayMS) * time.Millisecond
}

func (s SaaSAlertSetting) NotificationRetryDelay() time.Duration {
	s = NormalizeSaaSAlertSetting(s)
	return time.Duration(s.NotificationRetryDelaySeconds) * time.Second
}

func SaaSAlertSettingPayload(setting SaaSAlertSetting, exists bool) map[string]any {
	setting = NormalizeSaaSAlertSetting(setting)
	credentialProtection := setting.WebhookCredentialProtection
	if credentialProtection == "" {
		credentialProtection = SaaSAlertCredentialProtectionEmpty
		if setting.WebhookURL != "" || setting.WebhookSecret != "" {
			credentialProtection = SaaSAlertCredentialProtectionLegacyPlaintext
		}
	}
	return map[string]any{
		"id":                            setting.ID,
		"tenantId":                      setting.TenantID,
		"channel":                       setting.Channel,
		"enabled":                       setting.Enabled,
		"webhookUrl":                    setting.WebhookURL,
		"webhookSecretConfigured":       setting.WebhookSecret != "",
		"webhookCredentialProtection":   credentialProtection,
		"webhookCredentialKeyId":        setting.WebhookCredentialKeyID,
		"webhookTimeoutSeconds":         setting.WebhookTimeoutSeconds,
		"webhookRetryAttempts":          setting.WebhookRetryAttempts,
		"webhookRetryDelayMs":           setting.WebhookRetryDelayMS,
		"webhookTitleTemplate":          setting.WebhookTitleTemplate,
		"webhookBodyTemplate":           setting.WebhookBodyTemplate,
		"notificationMaxAttempts":       setting.NotificationMaxAttempts,
		"notificationRetryDelaySeconds": setting.NotificationRetryDelaySeconds,
		"minimumSeverity":               setting.MinimumSeverity,
		"alertTypes":                    append([]string{}, setting.AllowedAlertTypes...),
		"quietHoursEnabled":             setting.QuietHoursEnabled,
		"quietHoursStart":               setting.QuietHoursStart,
		"quietHoursEnd":                 setting.QuietHoursEnd,
		"timezone":                      setting.Timezone,
		"hourlyLimit":                   setting.HourlyLimit,
		"createdAt":                     setting.CreatedAt,
		"updatedAt":                     setting.UpdatedAt,
		"exists":                        exists,
	}
}

func normalizeSaaSAlertTypes(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized
}

func saasAlertTypeSliceParam(raw any) ([]string, error) {
	switch values := raw.(type) {
	case nil:
		return []string{}, nil
	case []string:
		return normalizeSaaSAlertTypes(values), nil
	case []any:
		items := make([]string, 0, len(values))
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("alert type must be string")
			}
			items = append(items, text)
		}
		return normalizeSaaSAlertTypes(items), nil
	case string:
		if strings.TrimSpace(values) == "" {
			return []string{}, nil
		}
		return normalizeSaaSAlertTypes(strings.Split(values, ",")), nil
	default:
		return nil, fmt.Errorf("unsupported alert types")
	}
}

func validSaaSAlertType(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' || char == '.' {
			continue
		}
		return false
	}
	return true
}

func parseSaaSAlertClock(value string) (int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, fmt.Errorf("invalid clock")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, fmt.Errorf("invalid hour")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("invalid minute")
	}
	return hour*60 + minute, nil
}

type StoreBackedSaaSAlertNotifier struct {
	store    SaaSAlertSettingReader
	fallback SaaSAlertNotifier
	channel  string
	guard    *outboundhttp.Guard
}

func NewStoreBackedSaaSAlertNotifier(store SaaSAlertSettingReader, fallback SaaSAlertNotifier, channel string, guards ...*outboundhttp.Guard) *StoreBackedSaaSAlertNotifier {
	if store == nil && fallback == nil {
		return nil
	}
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	guard := defaultSaaSAlertWebhookGuard
	if len(guards) > 0 && guards[0] != nil {
		guard = guards[0]
	}
	return &StoreBackedSaaSAlertNotifier{
		store:    store,
		fallback: fallback,
		channel:  channel,
		guard:    guard,
	}
}

func (n *StoreBackedSaaSAlertNotifier) NotifySaaSQuotaAlert(ctx context.Context, alert SaaSQuotaAlert) error {
	if n == nil {
		return nil
	}
	tenantID := alert.Status.TenantID
	if n.store != nil && tenantID > 0 {
		setting, found, err := n.store.GetSaaSAlertSetting(ctx, tenantID, n.channel)
		if err != nil {
			return err
		}
		if found {
			setting = NormalizeSaaSAlertSetting(setting)
			if !setting.Enabled {
				return nil
			}
			if strings.TrimSpace(setting.WebhookURL) == "" {
				return fmt.Errorf("%s for tenant %d", saaSAlertWebhookNotConfiguredErrorMessagePrefix, tenantID)
			}
			webhookNotifier, err := NewSaaSAlertWebhookNotifierWithTemplates(
				setting.WebhookURL,
				setting.WebhookTimeout(),
				setting.WebhookSecret,
				setting.WebhookTitleTemplate,
				setting.WebhookBodyTemplate,
				n.guard,
			)
			if err != nil {
				return err
			}
			notifier := NewRetryingSaaSAlertNotifier(webhookNotifier, setting.WebhookRetryAttempts, setting.WebhookRetryDelay())
			return notifier.NotifySaaSQuotaAlert(ctx, alert)
		}
	}
	if n.fallback != nil {
		return n.fallback.NotifySaaSQuotaAlert(ctx, alert)
	}
	return fmt.Errorf("%s for tenant %d", saaSAlertWebhookNotConfiguredErrorMessagePrefix, tenantID)
}

func requireAbsoluteWebhookURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fieldError("webhookUrl 必须是绝对 URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fieldError("webhookUrl 仅支持 http 或 https")
	}
	return nil
}
