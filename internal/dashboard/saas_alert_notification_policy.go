package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	SaaSAlertNotificationPolicyActionDeliver  = "deliver"
	SaaSAlertNotificationPolicyActionDefer    = "defer"
	SaaSAlertNotificationPolicyActionSuppress = "suppress"
)

type SaaSAlertNotificationDeliveryWindow struct {
	DeliveredCount  int
	NextAvailableAt time.Time
}

type SaaSAlertNotificationPolicyDecision struct {
	Action      string
	Reason      string
	NextRetryAt time.Time
}

func evaluateSaaSAlertNotificationPolicy(ctx context.Context, store SaaSAlertNotificationStore, setting SaaSAlertSetting, alert SaaSQuotaAlert, now time.Time) (SaaSAlertNotificationPolicyDecision, error) {
	setting = NormalizeSaaSAlertSetting(setting)
	if !setting.Enabled {
		return SaaSAlertNotificationPolicyDecision{
			Action: SaaSAlertNotificationPolicyActionSuppress,
			Reason: "policy suppressed: notification policy is disabled",
		}, nil
	}
	if strings.TrimSpace(alert.AlertType) == SaaSAlertTypeNotificationPolicyTest {
		return SaaSAlertNotificationPolicyDecision{Action: SaaSAlertNotificationPolicyActionDeliver}, nil
	}
	if !saasAlertSettingAllowsType(setting, alert.AlertType) {
		return SaaSAlertNotificationPolicyDecision{
			Action: SaaSAlertNotificationPolicyActionSuppress,
			Reason: "policy suppressed: alert type is not subscribed",
		}, nil
	}
	if !saasAlertSettingAllowsSeverity(setting, alert.Severity) {
		return SaaSAlertNotificationPolicyDecision{
			Action: SaaSAlertNotificationPolicyActionSuppress,
			Reason: "policy suppressed: alert severity is below minimum",
		}, nil
	}
	quiet, quietEnd, err := saasAlertSettingQuietHoursEnd(setting, now)
	if err != nil {
		return SaaSAlertNotificationPolicyDecision{}, err
	}
	if quiet {
		return SaaSAlertNotificationPolicyDecision{
			Action:      SaaSAlertNotificationPolicyActionDefer,
			Reason:      "policy deferred: quiet hours",
			NextRetryAt: quietEnd,
		}, nil
	}
	if setting.HourlyLimit > 0 && store != nil {
		window, err := store.SaaSAlertNotificationDeliveryWindow(ctx, setting.TenantID, setting.Channel, time.Hour)
		if err != nil {
			return SaaSAlertNotificationPolicyDecision{}, err
		}
		if window.DeliveredCount >= setting.HourlyLimit {
			nextRetryAt := window.NextAvailableAt
			if nextRetryAt.IsZero() || !nextRetryAt.After(now) {
				nextRetryAt = now.Add(time.Minute)
			}
			return SaaSAlertNotificationPolicyDecision{
				Action:      SaaSAlertNotificationPolicyActionDefer,
				Reason:      fmt.Sprintf("policy deferred: hourly limit %d reached", setting.HourlyLimit),
				NextRetryAt: nextRetryAt,
			}, nil
		}
	}
	return SaaSAlertNotificationPolicyDecision{Action: SaaSAlertNotificationPolicyActionDeliver}, nil
}

func applySaaSAlertNotificationPolicyDecision(ctx context.Context, store SaaSAlertNotificationStore, notificationID int64, decision SaaSAlertNotificationPolicyDecision) (bool, error) {
	if decision.Action == "" || decision.Action == SaaSAlertNotificationPolicyActionDeliver {
		return false, nil
	}
	if store == nil || notificationID <= 0 {
		return true, nil
	}
	switch decision.Action {
	case SaaSAlertNotificationPolicyActionSuppress:
		return true, store.SuppressSaaSAlertNotification(ctx, notificationID, decision.Reason)
	case SaaSAlertNotificationPolicyActionDefer:
		return true, store.DeferSaaSAlertNotification(ctx, notificationID, decision.NextRetryAt, decision.Reason)
	default:
		return false, fmt.Errorf("unsupported SaaS alert notification policy action %q", decision.Action)
	}
}

func saasAlertSettingAllowsType(setting SaaSAlertSetting, alertType string) bool {
	if len(setting.AllowedAlertTypes) == 0 {
		return true
	}
	alertType = strings.ToLower(strings.TrimSpace(alertType))
	if alertType == "" {
		alertType = SaaSAlertTypeQuotaExceeded
	}
	for _, allowed := range setting.AllowedAlertTypes {
		if allowed == alertType {
			return true
		}
	}
	return false
}

func saasAlertSettingAllowsSeverity(setting SaaSAlertSetting, severity string) bool {
	severity = strings.ToLower(strings.TrimSpace(severity))
	if severity == "" {
		severity = SaaSAlertSeverityWarning
	}
	severityRank := map[string]int{
		SaaSAlertSeverityWarning:  1,
		SaaSAlertSeverityCritical: 2,
	}
	current, found := severityRank[severity]
	if !found {
		current = severityRank[SaaSAlertSeverityWarning]
	}
	minimum := severityRank[setting.MinimumSeverity]
	if minimum == 0 {
		minimum = severityRank[SaaSAlertSeverityWarning]
	}
	return current >= minimum
}

func saasAlertSettingQuietHoursEnd(setting SaaSAlertSetting, now time.Time) (bool, time.Time, error) {
	if !setting.QuietHoursEnabled {
		return false, time.Time{}, nil
	}
	startMinutes, err := parseSaaSAlertClock(setting.QuietHoursStart)
	if err != nil {
		return false, time.Time{}, err
	}
	endMinutes, err := parseSaaSAlertClock(setting.QuietHoursEnd)
	if err != nil {
		return false, time.Time{}, err
	}
	location, err := time.LoadLocation(setting.Timezone)
	if err != nil {
		return false, time.Time{}, err
	}
	localNow := now.In(location)
	currentMinutes := localNow.Hour()*60 + localNow.Minute()
	inQuietHours := false
	endDayOffset := 0
	if startMinutes < endMinutes {
		inQuietHours = currentMinutes >= startMinutes && currentMinutes < endMinutes
	} else {
		inQuietHours = currentMinutes >= startMinutes || currentMinutes < endMinutes
		if currentMinutes >= startMinutes {
			endDayOffset = 1
		}
	}
	if !inQuietHours {
		return false, time.Time{}, nil
	}
	endHour := endMinutes / 60
	endMinute := endMinutes % 60
	end := time.Date(localNow.Year(), localNow.Month(), localNow.Day()+endDayOffset, endHour, endMinute, 0, 0, location)
	return true, end, nil
}
