package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	SaaSAlertNotificationChannelWebhook   = "webhook"
	SaaSAlertNotificationStatusPending    = "pending"
	SaaSAlertNotificationStatusFailed     = "failed"
	SaaSAlertNotificationStatusDelivered  = "delivered"
	SaaSAlertNotificationStatusDead       = "dead"
	SaaSAlertNotificationStatusClosed     = "closed"
	SaaSAlertNotificationStatusSuppressed = "suppressed"
)

type SaaSAlertNotification struct {
	ID              int64
	NotificationKey string
	AlertKey        string
	TenantID        int
	Channel         string
	Status          string
	Attempts        int
	MaxAttempts     int
	Alert           SaaSQuotaAlert
	LastError       string
	NextRetryAt     string
	DeliveredAt     string
	CreatedAt       string
	UpdatedAt       string
}

type SaaSAlertNotificationStore interface {
	EnqueueSaaSAlertNotification(ctx context.Context, alert SaaSQuotaAlert, channel string, maxAttempts int) (SaaSAlertNotification, error)
	ListDueSaaSAlertNotifications(ctx context.Context, limit int) ([]SaaSAlertNotification, error)
	MarkSaaSAlertNotificationDelivered(ctx context.Context, id int64) error
	MarkSaaSAlertNotificationFailed(ctx context.Context, id int64, retryDelay time.Duration, lastError string) (string, error)
	DeferSaaSAlertNotification(ctx context.Context, id int64, nextRetryAt time.Time, reason string) error
	SuppressSaaSAlertNotification(ctx context.Context, id int64, reason string) error
	SaaSAlertNotificationDeliveryWindow(ctx context.Context, tenantID int, channel string, window time.Duration) (SaaSAlertNotificationDeliveryWindow, error)
}

type SaaSAlertNotificationKeyReader interface {
	SaaSAlertNotificationByKey(ctx context.Context, notificationKey string) (SaaSAlertNotification, error)
}

type PersistentSaaSAlertNotifier struct {
	store       SaaSAlertNotificationStore
	inner       SaaSAlertNotifier
	channel     string
	maxAttempts int
	retryDelay  time.Duration
}

func NewPersistentSaaSAlertNotifier(store SaaSAlertNotificationStore, inner SaaSAlertNotifier, channel string, maxAttempts int, retryDelay time.Duration) *PersistentSaaSAlertNotifier {
	if inner == nil {
		return nil
	}
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if retryDelay < 0 {
		retryDelay = 0
	}
	return &PersistentSaaSAlertNotifier{
		store:       store,
		inner:       inner,
		channel:     channel,
		maxAttempts: maxAttempts,
		retryDelay:  retryDelay,
	}
}

func (n *PersistentSaaSAlertNotifier) NotifySaaSQuotaAlert(ctx context.Context, alert SaaSQuotaAlert) error {
	if n == nil || n.inner == nil {
		return nil
	}
	channel := n.channel
	maxAttempts := n.maxAttempts
	retryDelay := n.retryDelay
	setting, settingFound, err := notificationTenantSetting(ctx, n.store, alert.Status.TenantID, channel)
	if err != nil {
		return err
	}
	if settingFound {
		maxAttempts = setting.NotificationMaxAttempts
		retryDelay = setting.NotificationRetryDelay()
	}
	var notification SaaSAlertNotification
	var enqueueErr error
	if n.store != nil {
		notification, enqueueErr = n.store.EnqueueSaaSAlertNotification(ctx, alert, channel, maxAttempts)
	}
	if settingFound {
		decision, err := evaluateSaaSAlertNotificationPolicy(ctx, n.store, setting, alert, time.Now())
		if err != nil {
			return err
		}
		if handled, err := applySaaSAlertNotificationPolicyDecision(ctx, n.store, notification.ID, decision); handled {
			if err != nil {
				return err
			}
			return enqueueErr
		}
	}
	notifyErr := n.inner.NotifySaaSQuotaAlert(ctx, alert)
	if notification.ID > 0 {
		if notifyErr == nil {
			if err := n.store.MarkSaaSAlertNotificationDelivered(ctx, notification.ID); err != nil {
				return err
			}
		} else {
			if _, err := n.store.MarkSaaSAlertNotificationFailed(ctx, notification.ID, retryDelay, notifyErr.Error()); err != nil {
				return fmt.Errorf("%w; mark SaaS alert notification failed: %v", notifyErr, err)
			}
		}
	}
	if notifyErr != nil {
		return notifyErr
	}
	return enqueueErr
}

type SaaSAlertNotificationDispatchResult struct {
	Scanned    int
	Delivered  int
	Deferred   int
	Suppressed int
	Failed     int
	Dead       int
}

func DispatchDueSaaSAlertNotifications(ctx context.Context, store SaaSAlertNotificationStore, notifier SaaSAlertNotifier, limit int, retryDelay time.Duration) (SaaSAlertNotificationDispatchResult, error) {
	return dispatchDueSaaSAlertNotificationsAt(ctx, store, notifier, limit, retryDelay, time.Now)
}

func dispatchDueSaaSAlertNotificationsAt(ctx context.Context, store SaaSAlertNotificationStore, notifier SaaSAlertNotifier, limit int, retryDelay time.Duration, now func() time.Time) (SaaSAlertNotificationDispatchResult, error) {
	result := SaaSAlertNotificationDispatchResult{}
	if store == nil {
		return result, nil
	}
	if notifier == nil {
		return result, fmt.Errorf("SaaS alert notifier is required")
	}
	notifications, err := store.ListDueSaaSAlertNotifications(ctx, limit)
	if err != nil {
		return result, err
	}
	result.Scanned = len(notifications)
	for _, notification := range notifications {
		notificationRetryDelay := retryDelay
		setting, found, err := notificationTenantSetting(ctx, store, notification.TenantID, notification.Channel)
		if err != nil {
			return result, err
		}
		if found {
			notificationRetryDelay = setting.NotificationRetryDelay()
			decision, err := evaluateSaaSAlertNotificationPolicy(ctx, store, setting, notification.Alert, now())
			if err != nil {
				return result, err
			}
			if handled, err := applySaaSAlertNotificationPolicyDecision(ctx, store, notification.ID, decision); handled {
				if err != nil {
					return result, err
				}
				if decision.Action == SaaSAlertNotificationPolicyActionSuppress {
					result.Suppressed++
				} else {
					result.Deferred++
				}
				continue
			}
		}
		if err := notifier.NotifySaaSQuotaAlert(ctx, notification.Alert); err != nil {
			status, markErr := store.MarkSaaSAlertNotificationFailed(ctx, notification.ID, notificationRetryDelay, err.Error())
			if markErr != nil {
				return result, markErr
			}
			if status == SaaSAlertNotificationStatusDead {
				result.Dead++
			} else {
				result.Failed++
			}
			continue
		}
		if err := store.MarkSaaSAlertNotificationDelivered(ctx, notification.ID); err != nil {
			return result, err
		}
		result.Delivered++
	}
	return result, nil
}

func notificationTenantSetting(ctx context.Context, store any, tenantID int, channel string) (SaaSAlertSetting, bool, error) {
	reader, ok := store.(SaaSAlertSettingReader)
	if !ok || reader == nil || tenantID <= 0 {
		return SaaSAlertSetting{}, false, nil
	}
	setting, found, err := reader.GetSaaSAlertSetting(ctx, tenantID, channel)
	if err != nil || !found {
		return setting, found, err
	}
	return NormalizeSaaSAlertSetting(setting), true, nil
}

func SaaSAlertNotificationKey(alert SaaSQuotaAlert, channel string) string {
	status := alert.Status
	alertType := strings.TrimSpace(alert.AlertType)
	if alertType == "" {
		alertType = SaaSAlertTypeQuotaExceeded
	}
	periodKey := strings.TrimSpace(alert.PeriodKey)
	if periodKey == "" {
		periodKey = SaaSAlertPeriodLifetime
	}
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	return fmt.Sprintf("%d:%s:%s:%s:%s", status.TenantID, strings.TrimSpace(status.Metric), alertType, periodKey, channel)
}
