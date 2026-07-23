package dashboard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

const (
	SaaSMetricCorps                 = "corps"
	SaaSMetricUsers                 = "users"
	SaaSMetricContacts              = "contacts"
	SaaSMetricRooms                 = "rooms"
	SaaSMetricAgents                = "agents"
	SaaSMetricChannelCodes          = "channel_codes"
	SaaSMetricShopCodes             = "shop_codes"
	SaaSMetricRadars                = "radars"
	SaaSMetricLotteries             = "lotteries"
	SaaSMetricRoomInfinitePulls     = "room_infinite_pulls"
	SaaSMetricRoomFissions          = "room_fissions"
	SaaSMetricRoomClockIns          = "room_clock_ins"
	SaaSMetricRoomQualities         = "room_qualities"
	SaaSMetricRoomCalendars         = "room_calendars"
	SaaSMetricRoomReminds           = "room_reminds"
	SaaSMetricContactSOPs           = "contact_sops"
	SaaSMetricRoomSOPs              = "room_sops"
	SaaSMetricSensitiveWords        = "sensitive_words"
	SaaSMetricStorage               = "storage_mb"
	SaaSMetricContactMessageBatches = "contact_message_batches"
	SaaSMetricRoomMessageBatches    = "room_message_batches"
	SaaSMetricRoomTagPulls          = "room_tag_pulls"
	SaaSMetricWorkRoomAutoPulls     = "work_room_auto_pulls"
	SaaSMetricWorkFissions          = "work_fissions"
	SaaSMetricOfficialAccounts      = "official_accounts"
	SaaSMetricAsyncExecutions       = "async_executions"
)

type SaaSQuotaStatus struct {
	Metric     string
	TenantID   int
	Current    int64
	Limit      int64
	Additional int64
}

func (s SaaSQuotaStatus) Exceeded() bool {
	return s.Limit > 0 && s.Current+s.Additional > s.Limit
}

type SaaSQuotaStore interface {
	SaaSQuotaStatus(ctx context.Context, tenantID int, metric string, additional int64) (SaaSQuotaStatus, error)
	RefreshSaaSUsageCounter(ctx context.Context, tenantID int, metric string) error
}

const (
	SaaSAlertTypeQuotaExceeded                  = "quota_exceeded"
	SaaSAlertTypeTenantRenewal                  = "tenant_renewal_reminder"
	SaaSAlertTypeAdminTaskSLA                   = "admin_task_sla_reminder"
	SaaSAlertTypeOperationQueueAssign           = "operation_queue_assignment_reminder"
	SaaSAlertTypeNotificationPolicyTest         = "notification_policy_test"
	SaaSAlertTypePaymentFailed                  = "payment_failed_reminder"
	SaaSAlertTypeApprovalSLA                    = "approval_sla_reminder"
	SaaSAlertTypeSystemHealthIncident           = "system_health_incident"
	SaaSAlertTypeServiceAccountUsageWarning     = "service_account_usage_warning"
	SaaSAlertTypeServiceAccountRejectionWarning = "service_account_rejection_warning"
	SaaSAlertSeverityWarning                    = "warning"
	SaaSAlertSeverityCritical                   = "critical"
	SaaSAlertStatusOpen                         = "open"
	SaaSAlertStatusResolved                     = "resolved"
	SaaSAlertPeriodLifetime                     = "lifetime"
)

type SaaSQuotaAlert struct {
	Status    SaaSQuotaStatus
	AlertType string
	Severity  string
	PeriodKey string
	Source    string
	Message   string
	Context   map[string]any
}

type SaaSAlertStore interface {
	RecordSaaSQuotaAlert(ctx context.Context, alert SaaSQuotaAlert) error
}

type SaaSAlertRecord struct {
	ID              int64
	AlertKey        string
	TenantID        int
	AlertType       string
	Severity        string
	Status          string
	Metric          string
	PeriodKey       string
	CurrentValue    int64
	LimitValue      int64
	AdditionalValue int64
	OccurrenceCount int64
	Source          string
	Message         string
	ContextJSON     string
	FirstSeenAt     string
	LastSeenAt      string
	ResolvedAt      string
	CreatedAt       string
	UpdatedAt       string
}

type SaaSAlertListOptions struct {
	TenantID         int
	ExcludedTenantID int
	Status           string
	Metric           string
	AlertType        string
	Page             int
	PerPage          int
}

type SaaSAlertListPage struct {
	Items     []SaaSAlertRecord
	Total     int
	TotalPage int
}

type SaaSQuotaExceededError struct {
	Status SaaSQuotaStatus
}

func NewSaaSQuotaExceededError(status SaaSQuotaStatus) error {
	return &SaaSQuotaExceededError{Status: status}
}

func (e *SaaSQuotaExceededError) Error() string {
	if e == nil {
		return "SaaS quota exceeded"
	}
	return saasQuotaExceededMessage(e.Status)
}

func enforceSaaSQuota(ctx context.Context, w http.ResponseWriter, store any, tenantID int, metric string, additional int64) bool {
	if err := requireSaaSQuota(ctx, store, tenantID, metric, additional); err != nil {
		var quotaErr *SaaSQuotaExceededError
		if errors.As(err, &quotaErr) && quotaErr != nil {
			writeSaaSQuotaExceeded(w, quotaErr.Status)
			return false
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return false
	}
	return true
}

func requireSaaSQuota(ctx context.Context, store any, tenantID int, metric string, additional int64) error {
	quotaStore, ok := store.(SaaSQuotaStore)
	if !ok {
		return nil
	}
	status, err := quotaStore.SaaSQuotaStatus(ctx, tenantID, metric, additional)
	if err != nil {
		return err
	}
	if status.Exceeded() {
		return NewSaaSQuotaExceededError(status)
	}
	return nil
}

func refreshSaaSUsageCounter(ctx context.Context, store any, tenantID int, metric string) error {
	quotaStore, ok := store.(SaaSQuotaStore)
	if !ok {
		return nil
	}
	return quotaStore.RefreshSaaSUsageCounter(ctx, tenantID, metric)
}

func writeSaaSQuotaError(w http.ResponseWriter, err error) bool {
	var quotaErr *SaaSQuotaExceededError
	if !errors.As(err, &quotaErr) || quotaErr == nil {
		return false
	}
	writeSaaSQuotaExceeded(w, quotaErr.Status)
	return true
}

func writeSaaSQuotaExceeded(w http.ResponseWriter, status SaaSQuotaStatus) {
	writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, saasQuotaExceededMessage(status), map[string]any{
		"metric":     status.Metric,
		"current":    status.Current,
		"limit":      status.Limit,
		"additional": status.Additional,
	})
}

func saasQuotaExceededMessage(status SaaSQuotaStatus) string {
	return fmt.Sprintf("套餐额度已达上限：%s %d/%d", saasMetricLabel(status.Metric), status.Current, status.Limit)
}

func saasMetricLabel(metric string) string {
	switch metric {
	case SaaSMetricCorps:
		return "企业数"
	case SaaSMetricUsers:
		return "子账号数"
	case SaaSMetricContacts:
		return "客户数"
	case SaaSMetricRooms:
		return "客户群数"
	case SaaSMetricAgents:
		return "应用数"
	case SaaSMetricChannelCodes:
		return "渠道活码数"
	case SaaSMetricShopCodes:
		return "门店活码数"
	case SaaSMetricRadars:
		return "互动雷达数"
	case SaaSMetricLotteries:
		return "抽奖活动数"
	case SaaSMetricRoomInfinitePulls:
		return "无限拉群数"
	case SaaSMetricRoomFissions:
		return "群裂变数"
	case SaaSMetricRoomClockIns:
		return "群打卡数"
	case SaaSMetricRoomQualities:
		return "群质检规则数"
	case SaaSMetricRoomCalendars:
		return "群日历数"
	case SaaSMetricRoomReminds:
		return "客户群提醒数"
	case SaaSMetricContactSOPs:
		return "个人SOP规则数"
	case SaaSMetricRoomSOPs:
		return "群SOP规则数"
	case SaaSMetricSensitiveWords:
		return "敏感词词库数"
	case SaaSMetricStorage:
		return "素材存储"
	case SaaSMetricContactMessageBatches:
		return "客户群发任务数"
	case SaaSMetricRoomMessageBatches:
		return "客户群群发任务数"
	case SaaSMetricRoomTagPulls:
		return "标签建群任务数"
	case SaaSMetricWorkRoomAutoPulls:
		return "自动拉群活码数"
	case SaaSMetricWorkFissions:
		return "裂变活动数"
	case SaaSMetricOfficialAccounts:
		return "公众号授权数"
	case SaaSMetricAsyncExecutions:
		return "异步执行量"
	case SaaSEventMetricTenantRenewal:
		return "租户续费"
	case SaaSEventMetricAdminTaskSLA:
		return "运营任务SLA"
	case SaaSEventMetricOperationQueueAssignment:
		return "运营待办认领"
	case SaaSEventMetricNotificationPolicy:
		return "通知策略"
	case SaaSEventMetricPaymentCollection:
		return "支付收款"
	case SaaSEventMetricApprovalSLA:
		return "审批SLA"
	case SaaSEventMetricSystemHealth:
		return "平台系统健康"
	case SaaSEventMetricServiceAccountUsage:
		return "OpenAPI 服务账号用量"
	default:
		return metric
	}
}
