package dashboard

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/authjwt"
	"jiyi/mochat-go/internal/clientip"
	"jiyi/mochat-go/internal/identitysecurity"
	"jiyi/mochat-go/internal/outboundhttp"
	"jiyi/mochat-go/internal/saasauditanchor"
	"jiyi/mochat-go/internal/saasauth"
	"jiyi/mochat-go/internal/saasbackup"
	"jiyi/mochat-go/internal/saascompliance"
	"jiyi/mochat-go/internal/serviceaccountkey"
)

const (
	SaaSAdminScopeTenant   = "tenant"
	SaaSAdminScopePlatform = "platform"

	SaaSAdminTenantPopulationSelected = "selected_tenant"
	SaaSAdminTenantPopulationBusiness = "business_tenants"

	SaaSAdminDueStateAll       = "all"
	SaaSAdminDueStateNormal    = "normal"
	SaaSAdminDueStateExpiring  = "expiring"
	SaaSAdminDueStateExpired   = "expired"
	SaaSAdminDueStateNoPackage = "no_package"

	SaaSAdminExportKindTenants            = "tenants"
	SaaSAdminExportKindTenantLifecycle    = "tenantLifecycle"
	SaaSAdminExportKindAlerts             = "alerts"
	SaaSAdminExportKindNotifications      = "notifications"
	SaaSAdminExportKindNotificationHealth = "notificationHealth"
	SaaSAdminExportKindNotificationSLO    = "notificationSlo"
	SaaSAdminExportKindOperations         = "operations"
	SaaSAdminExportKindBillingEvents      = "billingEvents"
	SaaSAdminExportKindBillingReconcile   = "billingReconciliation"
	SaaSAdminExportKindBillingFollowUps   = "billingReconciliationFollowUps"
	SaaSAdminExportKindDailyReport        = "dailyReport"
	SaaSAdminExportKindBusinessMetrics    = "businessMetrics"
	SaaSAdminExportKindBusinessTrends     = "businessTrends"
	SaaSAdminExportKindRisk               = "risk"
	SaaSAdminExportKindOperationQueue     = "operationQueue"
	SaaSAdminExportKindOperationOwners    = "operationQueueOwners"
	SaaSAdminExportKindOperationAssigns   = "operationQueueAssignments"
	SaaSAdminExportKindCustomerSuccess    = "customerSuccess"
	SaaSAdminExportKindCustomerOwners     = "customerSuccessOwners"
	SaaSAdminExportKindRenewalForecast    = "renewalForecast"
	SaaSAdminExportKindRenewalOwners      = "renewalForecastOwners"
	SaaSAdminExportKindRiskFollowUps      = "riskFollowUps"
	SaaSAdminExportKindRiskOwners         = "riskFollowUpOwners"
	SaaSAdminExportKindTasks              = "tasks"
	SaaSAdminExportKindTaskSLA            = "taskSla"
	SaaSAdminExportKindUsage              = "usage"
	SaaSAdminExportKindPackages           = "packages"
	SaaSAdminExportKindBillingOwners      = "billingReconciliationFollowUpOwners"

	saasAdminListMaxLimit               = 100
	saasAdminExportMaxLimit             = 5000
	saasAdminDefaultHighUsageRatio      = 0.8
	saasAdminDefaultTaskSLAWarningHours = 4
	saasAdminDefaultTaskSLAOverdueHours = 24

	SaaSAdminRiskFollowUpStatusPending        = "pending"
	SaaSAdminRiskFollowUpStatusContacted      = "contacted"
	SaaSAdminRiskFollowUpStatusRenewalPending = "renewal_pending"
	SaaSAdminRiskFollowUpStatusResolved       = "resolved"
	SaaSAdminRiskFollowUpStatusIgnored        = "ignored"

	SaaSAdminRiskFollowUpDueStateAll     = "all"
	SaaSAdminRiskFollowUpDueStateOverdue = "overdue"
	SaaSAdminRiskFollowUpDueStateDueSoon = "due_soon"
	SaaSAdminRiskFollowUpDueStateFuture  = "future"
	SaaSAdminRiskFollowUpDueStateNoDate  = "no_date"
	SaaSAdminRiskFollowUpDueStateClosed  = "closed"

	SaaSAdminRenewalForecastFilterAll         = "all"
	SaaSAdminRenewalForecastPriceStatePriced  = "priced"
	SaaSAdminRenewalForecastPriceStateUnknown = "unknown"
	SaaSAdminRenewalForecastTaskStatusNone    = "none"

	SaaSAdminCustomerSuccessPriorityCritical = "critical"
	SaaSAdminCustomerSuccessPriorityHigh     = "high"
	SaaSAdminCustomerSuccessPriorityMedium   = "medium"
	SaaSAdminCustomerSuccessPriorityNormal   = "normal"

	SaaSAdminOperationQueueSourceAll                = "all"
	SaaSAdminOperationQueueSourceCustomerSuccess    = "customer_success"
	SaaSAdminOperationQueueSourceTaskSLA            = "task_sla"
	SaaSAdminOperationQueueSourceBillingFollowUp    = "billing_follow_up"
	SaaSAdminOperationQueueSourceNotification       = "notification"
	SaaSAdminOperationQueueSourceClosedNotification = "closed_notification"
	SaaSAdminOperationQueueSourceNotificationHealth = "notification_health"

	SaaSAdminTaskTypePackageSync     = "package_sync"
	SaaSAdminTaskTypeTenantRenewal   = "tenant_renewal"
	SaaSAdminTaskTypeTenantProvision = "tenant_provision"

	SaaSAdminTaskStatusPending  = "pending"
	SaaSAdminTaskStatusBlocked  = "blocked"
	SaaSAdminTaskStatusApplied  = "applied"
	SaaSAdminTaskStatusFailed   = "failed"
	SaaSAdminTaskStatusCanceled = "canceled"

	SaaSAdminOperationActionTaskCreate                     = "saas.admin.task.create"
	SaaSAdminOperationActionTaskApply                      = "saas.admin.task.apply"
	SaaSAdminOperationActionTaskCancel                     = "saas.admin.task.cancel"
	SaaSAdminOperationActionTaskBulkCancel                 = "saas.admin.task.bulk_cancel"
	SaaSAdminOperationActionTaskReset                      = "saas.admin.task.reset"
	SaaSAdminOperationActionTaskBulkReset                  = "saas.admin.task.bulk_reset"
	SaaSAdminOperationActionTaskBlock                      = "saas.admin.task.block"
	SaaSAdminOperationActionTaskSLANotify                  = "saas.admin.task.sla_notify"
	SaaSAdminOperationActionOperationQueueAssign           = "saas.admin.operation_queue.assign"
	SaaSAdminOperationActionOperationQueueAssignmentClose  = "saas.admin.operation_queue.assignment_close"
	SaaSAdminOperationActionOperationQueueAssignmentNotify = "saas.admin.operation_queue.assignment_notify"
	SaaSAdminOperationActionBillingReconciliationFollowUp  = "billing.reconciliation.follow_up"
	SaaSAdminOperationActionNotificationClose              = "tenant.notification.close"
	SaaSAdminOperationActionNotificationPolicyUpdate       = "tenant.notification_policy.update"
	SaaSAdminOperationActionNotificationPolicyTest         = "tenant.notification_policy.test"
	SaaSAdminOperationActionNotificationCredentialRotate   = "saas.admin.notification_credential.rotate"
	SaaSAdminOperationActionWeComCredentialRotate          = "saas.admin.wecom_credential.rotate"
	SaaSAdminOperationActionWeChatOpenCredentialRotate     = "saas.admin.wechat_open_credential.rotate"
	SaaSAdminOperationActionTenantRenewalNotify            = "tenant.renewal.notify"
	SaaSAdminOperationTargetAdminTask                      = "admin_task"
	SaaSAdminOperationTargetBillingEvent                   = "billing_event"
	SaaSAdminOperationTargetAlertNotification              = "alert_notification"
	SaaSAdminOperationTargetNotificationPolicy             = "notification_policy"
	SaaSAdminOperationTargetNotificationCredential         = "notification_credential"
	SaaSAdminOperationTargetWeComCredential                = "wecom_credential"
	SaaSAdminOperationTargetWeChatOpenCredential           = "wechat_open_credential"
	SaaSAdminOperationTargetNotificationHealth             = "notification_health"
)

type SaaSAdminOverviewStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	SaaSAdminOverview(ctx context.Context, options SaaSAdminOverviewOptions) (SaaSAdminOverview, error)
	SaaSAdminPackages(ctx context.Context) ([]SaaSAdminPackage, error)
	SaaSAdminTenantUsage(ctx context.Context, tenantID int) ([]SaaSAdminUsageMetric, error)
	SaaSAdminAlertSummary(ctx context.Context, options SaaSAlertListOptions) (SaaSAdminAlertSummary, error)
	ListSaaSAlerts(ctx context.Context, options SaaSAlertListOptions) (SaaSAlertListPage, error)
	ResolveSaaSAdminAlert(ctx context.Context, resolve SaaSAdminAlertResolve) (SaaSAdminAlertResolveResult, error)
	BulkResolveSaaSAdminAlerts(ctx context.Context, resolve SaaSAdminAlertBulkResolve) (SaaSAdminAlertBulkResolveResult, error)
	SaaSAdminAlertNotificationSummary(ctx context.Context, options SaaSAdminAlertNotificationOptions) (SaaSAdminAlertNotificationSummary, error)
	SaaSAdminAlertNotifications(ctx context.Context, options SaaSAdminAlertNotificationOptions) ([]SaaSAlertNotification, error)
	SaaSAdminNotificationHealth(ctx context.Context, options SaaSAdminNotificationHealthOptions) (SaaSAdminNotificationHealthSource, error)
	SaaSAdminNotificationSLO(ctx context.Context, options SaaSAdminNotificationSLOOptions) (SaaSAdminNotificationSLOSource, error)
	SaaSAdminNotificationPolicies(ctx context.Context, options SaaSAdminNotificationPolicyOptions) (SaaSAdminNotificationPolicyReport, error)
	GetSaaSAlertSetting(ctx context.Context, tenantID int, channel string) (SaaSAlertSetting, bool, error)
	SaveSaaSAlertSetting(ctx context.Context, setting SaaSAlertSetting) (SaaSAlertSetting, error)
	EnqueueSaaSAlertNotification(ctx context.Context, alert SaaSQuotaAlert, channel string, maxAttempts int) (SaaSAlertNotification, error)
	RetrySaaSAdminAlertNotification(ctx context.Context, retry SaaSAdminAlertNotificationRetry) (SaaSAdminAlertNotificationRetryResult, error)
	BulkRetrySaaSAdminAlertNotifications(ctx context.Context, retry SaaSAdminAlertNotificationBulkRetry) (SaaSAdminAlertNotificationBulkRetryResult, error)
	CloseSaaSAdminAlertNotification(ctx context.Context, close SaaSAdminAlertNotificationClose) (SaaSAdminAlertNotificationCloseResult, error)
	BulkCloseSaaSAdminAlertNotifications(ctx context.Context, close SaaSAdminAlertNotificationBulkClose) (SaaSAdminAlertNotificationBulkCloseResult, error)
	SaaSAdminOperationLogSummary(ctx context.Context, options SaaSAdminOperationLogOptions) (SaaSAdminOperationLogSummary, error)
	SaaSAdminOperationLogs(ctx context.Context, options SaaSAdminOperationLogOptions) ([]SaaSAdminOperationLog, error)
	RecordSaaSAdminOperationLog(ctx context.Context, item SaaSAdminOperationLog) (int64, error)
	SaaSAdminBillingEventSummary(ctx context.Context, options SaaSAdminBillingEventOptions) (SaaSAdminDailyBillingSummary, error)
	SaaSAdminBillingEvents(ctx context.Context, options SaaSAdminBillingEventOptions) ([]SaaSAdminBillingEvent, error)
	SaaSAdminBillingEventByID(ctx context.Context, id int64) (SaaSAdminBillingEvent, bool, error)
	SaaSAdminBillingReconciliation(ctx context.Context, options SaaSAdminBillingReconciliationOptions) (SaaSAdminBillingReconciliationReport, error)
	SaaSAdminBillingReconciliationFollowUpSnapshots(ctx context.Context, options SaaSAdminBillingReconciliationFollowUpOptions) ([]SaaSAdminBillingReconciliationFollowUpSnapshot, error)
	SaaSAdminTaskSummary(ctx context.Context, options SaaSAdminTaskOptions) (SaaSAdminTaskSummary, error)
	SaaSAdminTasks(ctx context.Context, options SaaSAdminTaskOptions) ([]SaaSAdminTask, error)
	SaaSAdminLatestRiskFollowUps(ctx context.Context, tenantIDs []int) (map[int]SaaSAdminRiskFollowUpSnapshot, error)
	SaaSAdminRiskFollowUpSnapshots(ctx context.Context, options SaaSAdminRiskFollowUpTaskOptions) ([]SaaSAdminRiskFollowUpSnapshot, error)
	RecordSaaSAdminRiskFollowUp(ctx context.Context, followUp SaaSAdminRiskFollowUp) (SaaSAdminRiskFollowUpResult, error)
	UpsertSaaSAdminPackage(ctx context.Context, update SaaSAdminPackageUpsert) (SaaSAdminPackage, error)
	CreateSaaSAdminTask(ctx context.Context, task SaaSAdminTaskCreate) (SaaSAdminTask, error)
	UpdateSaaSAdminTaskStatus(ctx context.Context, update SaaSAdminTaskStatusUpdate) (SaaSAdminTask, error)
	UpdateSaaSAdminTenantStatus(ctx context.Context, update SaaSAdminTenantStatusUpdate) (SaaSAdminTenantStatusUpdateResult, error)
	UpdateSaaSAdminTenantPackage(ctx context.Context, update SaaSAdminTenantPackageUpdate) (SaaSAdminTenantPackageUpdateResult, error)
	RenewSaaSAdminTenant(ctx context.Context, renewal SaaSAdminTenantRenewal) (SaaSAdminTenantRenewalResult, error)
	ProvisionSaaSAdminTenant(ctx context.Context, provision SaaSAdminTenantProvision) (SaaSAdminTenantProvisionResult, error)
	SaaSAdminSubscriptions(ctx context.Context, options SaaSAdminSubscriptionOptions) (SaaSAdminSubscriptionReport, error)
	SaaSAdminSubscriptionEvents(ctx context.Context, options SaaSAdminSubscriptionEventOptions) ([]SaaSAdminSubscriptionEvent, error)
	TransitionSaaSAdminSubscription(ctx context.Context, transition SaaSAdminSubscriptionTransition) (SaaSAdminSubscriptionTransitionResult, error)
	ReconcileSaaSAdminSubscriptions(ctx context.Context, reconcile SaaSAdminSubscriptionReconcile) (SaaSAdminSubscriptionReconcileResult, error)
}

type SaaSAdminTenantStatusApprovalStore interface {
	PlanSaaSAdminTenantStatusUpdate(context.Context, SaaSAdminTenantStatusUpdate) (SaaSAdminTenantStatusApprovalPlan, error)
}

type SaaSAdminOverviewOptions struct {
	Scope            string
	TenantID         int
	ExcludedTenantID int
	Limit            int
	ExpiringDays     int
	Keyword          string
	TenantStatus     int
	PackageCode      string
	DueState         string
}

type SaaSAdminOverview struct {
	Summary SaaSAdminSummary
	Tenants []SaaSAdminTenantOverview
	Metrics []SaaSAdminMetricOverview
}

type SaaSAdminTenantDetail struct {
	CanPlatformScope      bool
	PlatformAdminTenantID int
	Summary               SaaSAdminSummary
	Tenant                SaaSAdminTenantOverview
	Metrics               []SaaSAdminMetricOverview
	Operations            []SaaSAdminOperationLog
}

type SaaSAdminTenantLifecycle struct {
	PlatformAdminTenantID int
	Tenant                SaaSAdminTenantOverview
	Filter                SaaSAdminTenantLifecycleFilter
	Operations            []SaaSAdminOperationLog
	BillingEvents         []SaaSAdminBillingEvent
	Tasks                 []SaaSAdminTask
	Alerts                []SaaSAlertRecord
	Notifications         []SaaSAlertNotification
}

type SaaSAdminTenantLifecycleFilter struct {
	Source    string
	EventType string
	Status    string
	Keyword   string
}

type SaaSAdminTenantLifecycleEvent struct {
	Source      string
	EventType   string
	Title       string
	Status      string
	OccurredAt  string
	ReferenceID string
	ActorUserID int
	Remark      string
	Payload     any
}

type SaaSAdminTenantUsageDetail struct {
	CanPlatformScope      bool
	PlatformAdminTenantID int
	Tenant                SaaSAdminTenantOverview
	Summary               SaaSAdminUsageSummary
	Metrics               []SaaSAdminUsageMetric
}

type SaaSAdminRiskOptions struct {
	SaaSAdminOverviewOptions
	HighUsageRatio float64
}

type SaaSAdminRiskReport struct {
	Summary SaaSAdminRiskSummary
	Items   []SaaSAdminRiskTenant
}

type SaaSAdminRiskSummary struct {
	TotalTenantCount         int
	EvaluatedTenantCount     int
	RiskTenantCount          int
	CriticalRiskTenantCount  int
	HighRiskTenantCount      int
	MediumRiskTenantCount    int
	DisabledTenantCount      int
	ExpiredTenantCount       int
	ExpiringSoonTenantCount  int
	NoPackageTenantCount     int
	OpenAlertTenantCount     int
	HighUsageTenantCount     int
	ExceededUsageTenantCount int
	WarningUsageTenantCount  int
	FollowUpTenantCount      int
	PendingFollowUpCount     int
	OverdueFollowUpCount     int
	RenewalPendingCount      int
}

type SaaSAdminRiskTenant struct {
	Tenant          SaaSAdminTenantOverview
	RiskLevel       string
	RiskScore       int
	Reasons         []string
	SuggestedAction string
	HighUsageRatio  float64
	TopUsageMetrics []SaaSAdminUsageMetric
	FollowUp        SaaSAdminRiskFollowUpSnapshot
}

type SaaSAdminBusinessMetricsOptions struct {
	TenantLimit    int
	ExpiringDays   int
	HighUsageRatio float64
	BillingLimit   int
}

type SaaSAdminBusinessMetricsReport struct {
	Options       SaaSAdminBusinessMetricsOptions
	Summary       SaaSAdminBusinessMetricsSummary
	Packages      []SaaSAdminBusinessPackageMetric
	BillingEvents []SaaSAdminBillingEvent
}

type SaaSAdminBusinessMetricsSummary struct {
	TenantCount                int
	ActiveTenantPackageCount   int
	PricedTenantCount          int
	UnknownPriceTenantCount    int
	EstimatedMRRCents          int64
	EstimatedARRCents          int64
	EstimatedARPACents         int64
	AtRiskTenantCount          int
	AtRiskMRRCents             int64
	ExpiringSoonTenantCount    int
	ExpiringSoonMRRCents       int64
	ExpiredTenantCount         int
	ExpiredMRRCents            int64
	RecentBillingEventCount    int
	RecentRenewalCount         int
	RecentRefundCount          int
	RecentGrossAmountCents     int64
	RecentRefundAmountCents    int64
	RecentBillingAmountCents   int64
	BillingPricePackageCount   int
	MissingBillingPackageCount int
}

type SaaSAdminBusinessPackageMetric struct {
	PackageCode             string
	PackageName             string
	PackageStatus           int
	TenantCount             int
	ActiveTenantCount       int
	PricedTenantCount       int
	UnknownPriceTenantCount int
	EstimatedMRRCents       int64
	EstimatedARRCents       int64
	AtRiskTenantCount       int
	AtRiskMRRCents          int64
	ExpiringSoonTenantCount int
	ExpiringSoonMRRCents    int64
	ExpiredTenantCount      int
	ExpiredMRRCents         int64
	LatestAmountCents       int64
	LatestBillingEventID    int64
	LatestBillingAt         string
	Estimated               bool
}

type SaaSAdminBusinessTrendOptions struct {
	Months       int
	BillingLimit int
	TaskLimit    int
}

type SaaSAdminBusinessTrendReport struct {
	Options       SaaSAdminBusinessTrendOptions
	Summary       SaaSAdminBusinessTrendSummary
	Months        []SaaSAdminBusinessTrendMonth
	RenewalFunnel SaaSAdminBusinessRenewalFunnel
}

type SaaSAdminBusinessTrendSummary struct {
	MonthCount          int
	BillingEventCount   int
	RenewalCount        int
	RefundCount         int
	GrossAmountCents    int64
	RefundAmountCents   int64
	BillingAmountCents  int64
	TenantCount         int
	PackageCount        int
	TaskCount           int
	PendingTaskCount    int
	BlockedTaskCount    int
	FailedTaskCount     int
	AppliedTaskCount    int
	CanceledTaskCount   int
	ActionableTaskCount int
}

type SaaSAdminBusinessTrendMonth struct {
	Month              string
	EventCount         int
	RenewalCount       int
	RefundCount        int
	GrossAmountCents   int64
	RefundAmountCents  int64
	AmountCents        int64
	TenantCount        int
	PackageCount       int
	Packages           []SaaSAdminBusinessTrendPackage
	tenantIDs          map[int]struct{}
	packageCodes       map[string]struct{}
	packageMetricsByID map[string]*SaaSAdminBusinessTrendPackage
}

type SaaSAdminBusinessTrendPackage struct {
	PackageCode       string
	PackageName       string
	EventCount        int
	RenewalCount      int
	RefundCount       int
	GrossAmountCents  int64
	RefundAmountCents int64
	AmountCents       int64
}

type SaaSAdminBusinessRenewalFunnel struct {
	Summary     SaaSAdminTaskSummary
	RecentTasks []SaaSAdminTask
}

type SaaSAdminRenewalForecastOptions struct {
	TenantLimit  int
	Days         int
	BillingLimit int
	TaskLimit    int
	PackageCode  string
	Bucket       string
	PriceState   string
	Owner        string
	TaskStatus   string
}

type SaaSAdminRenewalForecastReport struct {
	Options       SaaSAdminRenewalForecastOptions
	Summary       SaaSAdminRenewalForecastSummary
	Buckets       []SaaSAdminRenewalForecastBucket
	Owners        []SaaSAdminRenewalForecastOwnerSummary
	Items         []SaaSAdminRenewalForecastTenant
	TaskSummary   SaaSAdminTaskSummary
	RecentTasks   []SaaSAdminTask
	BillingEvents []SaaSAdminBillingEvent
}

type SaaSAdminRenewalForecastSummary struct {
	TenantCount             int
	ForecastTenantCount     int
	PricedTenantCount       int
	UnknownPriceTenantCount int
	RenewalAmountCents      int64
	EstimatedMRRCents       int64
	ExpiredTenantCount      int
	ExpiredAmountCents      int64
	DueWithin30TenantCount  int
	DueWithin30AmountCents  int64
	Due31To60TenantCount    int
	Due31To60AmountCents    int64
	Due61To90TenantCount    int
	Due61To90AmountCents    int64
	DueLaterTenantCount     int
	DueLaterAmountCents     int64
	ActionableTaskCount     int
	PendingTaskCount        int
	BlockedTaskCount        int
	FailedTaskCount         int
	AppliedTaskCount        int
	CanceledTaskCount       int
}

type SaaSAdminRenewalForecastBucket struct {
	Bucket                  string
	Label                   string
	TenantCount             int
	PricedTenantCount       int
	UnknownPriceTenantCount int
	RenewalAmountCents      int64
	EstimatedMRRCents       int64
	ActionableTaskCount     int
}

type SaaSAdminRenewalForecastOwnerSummary struct {
	Owner                   string
	TenantCount             int
	PricedTenantCount       int
	UnknownPriceTenantCount int
	RenewalAmountCents      int64
	EstimatedMRRCents       int64
	ExpiredTenantCount      int
	DueWithin30TenantCount  int
	Due31To60TenantCount    int
	Due61To90TenantCount    int
	DueLaterTenantCount     int
	ActionableTaskCount     int
	PendingTaskCount        int
	BlockedTaskCount        int
	FailedTaskCount         int
	AppliedTaskCount        int
	CanceledTaskCount       int
	NextFollowUpAt          string
	TopTenants              []SaaSAdminRenewalForecastTenant
}

type SaaSAdminRenewalForecastTenant struct {
	Tenant               SaaSAdminTenantOverview
	Bucket               string
	BucketLabel          string
	DaysUntil            int
	Owner                string
	RenewalAmountCents   int64
	EstimatedMRRCents    int64
	Priced               bool
	LatestBillingEventID int64
	LatestBillingAt      string
	RiskFollowUp         SaaSAdminRiskFollowUpSnapshot
	HasRiskFollowUp      bool
	TaskSummary          SaaSAdminTaskSummary
	LatestTask           SaaSAdminTask
	HasLatestTask        bool
}

type SaaSAdminRenewalForecastTasks struct {
	Options               SaaSAdminRenewalForecastOptions
	PackageCode           string
	ExpiresAt             string
	Months                int
	AmountCents           int64
	Currency              string
	PaidAt                string
	PaymentMethod         string
	ExternalOrderNoPrefix string
	Remark                string
	ForceCreate           bool
	ActorUserID           int
	ActorTenantID         int
}

type SaaSAdminRenewalForecastTasksResult struct {
	Options              SaaSAdminRenewalForecastOptions
	MatchedCount         int
	CreatedCount         int
	PendingCount         int
	BlockedCount         int
	SkippedExistingCount int
	SkippedInvalidCount  int
	PackageCode          string
	ExpiresAt            string
	Months               int
	AmountCents          int64
	Currency             string
	PaidAt               string
	PaymentMethod        string
	Remark               string
	ForceCreate          bool
	Tasks                []SaaSAdminTask
	Skipped              []SaaSAdminCustomerSuccessRenewalTaskSkipped
}

type SaaSAdminRenewalForecastAssign struct {
	Options        SaaSAdminRenewalForecastOptions
	Status         string
	Owner          string
	NextFollowUpAt string
	Remark         string
	ActorUserID    int
	ActorTenantID  int
}

type SaaSAdminRenewalForecastAssignResult struct {
	Options        SaaSAdminRenewalForecastOptions
	MatchedCount   int
	AssignedCount  int
	Status         string
	Owner          string
	NextFollowUpAt string
	Remark         string
	FollowUps      []SaaSAdminRiskFollowUpResult
}

type SaaSAdminCustomerSuccessOptions struct {
	TenantLimit    int
	Limit          int
	ExpiringDays   int
	HighUsageRatio float64
	Owner          string
	Priority       string
}

type SaaSAdminCustomerSuccessReport struct {
	Options SaaSAdminCustomerSuccessOptions
	Summary SaaSAdminCustomerSuccessSummary
	Items   []SaaSAdminCustomerSuccessQueueItem
}

type SaaSAdminCustomerSuccessSummary struct {
	TenantCount                int
	QueueCount                 int
	ReturnedCount              int
	CriticalCount              int
	HighCount                  int
	MediumCount                int
	NormalCount                int
	OverdueCount               int
	DueSoonCount               int
	UnassignedCount            int
	RiskTenantCount            int
	BillingFollowUpCount       int
	ActionableTaskCount        int
	RetryableNotificationCount int
}

type SaaSAdminCustomerSuccessQueueItem struct {
	Tenant                     SaaSAdminTenantOverview
	Risk                       SaaSAdminRiskTenant
	Priority                   string
	HealthScore                int
	Owner                      string
	DueState                   string
	Reasons                    []string
	NextAction                 string
	RiskFollowUp               SaaSAdminRiskFollowUpTask
	BillingFollowUps           []SaaSAdminBillingReconciliationFollowUpTask
	BillingFollowUpCount       int
	AdminTaskSummary           SaaSAdminTaskSummary
	RetryableNotificationCount int
	FailedNotificationCount    int
	DeadNotificationCount      int
}

type SaaSAdminOperationQueueOptions struct {
	TenantLimit        int
	Limit              int
	ExpiringDays       int
	HighUsageRatio     float64
	Source             string
	Priority           string
	Owner              string
	Keyword            string
	WarningHours       int
	OverdueHours       int
	HealthWindowHours  int
	HealthStaleMinutes int
}

type SaaSAdminOperationQueueReport struct {
	Options SaaSAdminOperationQueueOptions
	Summary SaaSAdminOperationQueueSummary
	Items   []SaaSAdminOperationQueueItem
}

type SaaSAdminOperationQueueOwnerReport struct {
	Options            SaaSAdminOperationQueueOptions
	Summary            SaaSAdminOperationQueueSummary
	TotalOwnerCount    int
	ReturnedOwnerCount int
	ScannedQueueCount  int
	Owners             []SaaSAdminOperationQueueOwnerSummary
}

type SaaSAdminOperationQueueAssignmentOptions struct {
	Source      string
	Owner       string
	Status      string
	DueState    string
	ObjectType  string
	ObjectID    string
	TenantID    int
	Keyword     string
	Limit       int
	CurrentOnly bool
}

type SaaSAdminOperationQueueAssignmentReport struct {
	Options       SaaSAdminOperationQueueAssignmentOptions
	Summary       SaaSAdminOperationQueueAssignmentSummary
	ReturnedCount int
	Assignments   []SaaSAdminOperationQueueAssignment
}

type SaaSAdminOperationQueueAssignmentSummary struct {
	AssignmentCount         int
	TenantCount             int
	OwnerCount              int
	SourceCount             int
	TaskSLACount            int
	NotificationCount       int
	ClosedNotificationCount int
	NotificationHealthCount int
	OverdueCount            int
	DueSoonCount            int
	FutureCount             int
	NoDateCount             int
	ClosedCount             int
	NextFollowUpAt          string
}

type SaaSAdminOperationQueueSummary struct {
	QueueCount              int
	ReturnedCount           int
	TenantCount             int
	SourceCount             int
	CriticalCount           int
	HighCount               int
	MediumCount             int
	NormalCount             int
	CustomerSuccessCount    int
	TaskSLACount            int
	BillingFollowUpCount    int
	NotificationCount       int
	ClosedNotificationCount int
	NotificationHealthCount int
	UnassignedCount         int
}

type SaaSAdminOperationQueueItem struct {
	ID         string
	Source     string
	Priority   string
	TenantID   int
	TenantName string
	Owner      string
	Title      string
	Reason     string
	NextAction string
	DueState   string
	Status     string
	ObjectType string
	ObjectID   string
	CreatedAt  string
	UpdatedAt  string
	AgeHours   int
	Score      int
	Remark     string
	Assignment *SaaSAdminOperationQueueAssignment
	Reference  map[string]any
}

type SaaSAdminOperationQueueOwnerSummary struct {
	Owner                   string
	QueueCount              int
	TenantCount             int
	SourceCount             int
	CriticalCount           int
	HighCount               int
	MediumCount             int
	NormalCount             int
	CustomerSuccessCount    int
	TaskSLACount            int
	BillingFollowUpCount    int
	NotificationCount       int
	ClosedNotificationCount int
	NotificationHealthCount int
	UnassignedCount         int
	MaxAgeHours             int
	TopTenants              []SaaSAdminOperationQueueOwnerTenant
	TopItems                []SaaSAdminOperationQueueItem
}

type SaaSAdminOperationQueueOwnerTenant struct {
	TenantID      int
	TenantName    string
	QueueCount    int
	CriticalCount int
	HighCount     int
	Sources       []string
}

type SaaSAdminOperationQueueAssign struct {
	Options        SaaSAdminOperationQueueOptions
	Status         string
	Owner          string
	NextFollowUpAt string
	Remark         string
	ActorUserID    int
	ActorTenantID  int
}

type SaaSAdminOperationQueueAssignResult struct {
	Options                         SaaSAdminOperationQueueOptions
	MatchedCount                    int
	AssignableCount                 int
	AssignedCount                   int
	SkippedCount                    int
	UnsupportedCount                int
	CustomerSuccessAssignedCount    int
	BillingFollowUpAssignedCount    int
	QueueAssignmentAssignedCount    int
	TaskSLAAssignedCount            int
	NotificationAssignedCount       int
	ClosedNotificationAssignedCount int
	NotificationHealthAssignedCount int
	Status                          string
	Owner                           string
	NextFollowUpAt                  string
	Remark                          string
	Items                           []SaaSAdminOperationQueueAssignItem
}

type SaaSAdminOperationQueueAssignItem struct {
	QueueItem                SaaSAdminOperationQueueItem
	RiskFollowUp             *SaaSAdminRiskFollowUpResult
	BillingFollowUp          *SaaSAdminBillingReconciliationFollowUpResult
	OperationQueueAssignment *SaaSAdminOperationQueueAssignment
	SkippedReason            string
}

type SaaSAdminOperationQueueAssignment struct {
	TenantID       int
	Source         string
	ObjectType     string
	ObjectID       string
	TargetName     string
	Owner          string
	Status         string
	DueState       string
	NextFollowUpAt string
	Remark         string
	OperationID    int64
	ActorUserID    int
	ActorTenantID  int
	AssignedAt     string
}

type SaaSAdminOperationQueueAssignmentNotifications struct {
	Options       SaaSAdminOperationQueueAssignmentOptions
	Channel       string
	MaxAttempts   int
	Remark        string
	ForceCreate   bool
	ActorUserID   int
	ActorTenantID int
}

type SaaSAdminOperationQueueAssignmentNotificationsResult struct {
	Options              SaaSAdminOperationQueueAssignmentOptions
	Summary              SaaSAdminOperationQueueAssignmentSummary
	MatchedCount         int
	EligibleCount        int
	EnqueuedCount        int
	SkippedExistingCount int
	SkippedInvalidCount  int
	SkippedStatusCount   int
	Channel              string
	MaxAttempts          int
	Remark               string
	ForceCreate          bool
	Notifications        []SaaSAlertNotification
	Skipped              []SaaSAdminOperationQueueAssignmentNotificationSkipped
}

type SaaSAdminOperationQueueAssignmentNotificationSkipped struct {
	OperationID int64
	TenantID    int
	Source      string
	ObjectID    string
	DueState    string
	Reason      string
}

type SaaSAdminOperationQueueAssignmentClose struct {
	OperationID   int64
	Status        string
	Remark        string
	ActorUserID   int
	ActorTenantID int
	Context       map[string]any
}

type SaaSAdminOperationQueueAssignmentCloseResult struct {
	Closed        bool
	AlreadyClosed bool
	Previous      SaaSAdminOperationQueueAssignment
	Assignment    SaaSAdminOperationQueueAssignment
}

type SaaSAdminCustomerSuccessOwnerReport struct {
	Options         SaaSAdminCustomerSuccessOptions
	Summary         SaaSAdminCustomerSuccessSummary
	TotalOwnerCount int
	Owners          []SaaSAdminCustomerSuccessOwnerSummary
}

type SaaSAdminCustomerSuccessOwnerSummary struct {
	Owner                      string
	TenantCount                int
	CriticalCount              int
	HighCount                  int
	MediumCount                int
	NormalCount                int
	OverdueCount               int
	DueSoonCount               int
	BlockedCount               int
	BillingFollowUpCount       int
	ActionableTaskCount        int
	RetryableNotificationCount int
	FailedNotificationCount    int
	DeadNotificationCount      int
	MaxHealthScore             int
	TotalHealthScore           int
	AverageHealthScore         int
	NextFollowUpAt             string
	TopTenants                 []SaaSAdminCustomerSuccessQueueItem
}

type SaaSAdminCustomerSuccessAssign struct {
	Options        SaaSAdminCustomerSuccessOptions
	Status         string
	Owner          string
	NextFollowUpAt string
	Remark         string
	ActorUserID    int
	ActorTenantID  int
}

type SaaSAdminCustomerSuccessAssignResult struct {
	Options        SaaSAdminCustomerSuccessOptions
	MatchedCount   int
	AssignedCount  int
	Status         string
	Owner          string
	NextFollowUpAt string
	Remark         string
	FollowUps      []SaaSAdminRiskFollowUpResult
}

type SaaSAdminCustomerSuccessRenewalTasks struct {
	Options               SaaSAdminCustomerSuccessOptions
	PackageCode           string
	ExpiresAt             string
	Months                int
	AmountCents           int64
	Currency              string
	PaidAt                string
	PaymentMethod         string
	ExternalOrderNoPrefix string
	Remark                string
	ForceCreate           bool
	ActorUserID           int
	ActorTenantID         int
}

type SaaSAdminCustomerSuccessRenewalTasksResult struct {
	Options              SaaSAdminCustomerSuccessOptions
	MatchedCount         int
	CreatedCount         int
	PendingCount         int
	BlockedCount         int
	SkippedExistingCount int
	SkippedInvalidCount  int
	PackageCode          string
	ExpiresAt            string
	Months               int
	AmountCents          int64
	Currency             string
	PaidAt               string
	PaymentMethod        string
	Remark               string
	ForceCreate          bool
	Tasks                []SaaSAdminTask
	Skipped              []SaaSAdminCustomerSuccessRenewalTaskSkipped
}

type SaaSAdminCustomerSuccessRenewalNotifications struct {
	Options       SaaSAdminCustomerSuccessOptions
	Channel       string
	MaxAttempts   int
	ReminderDays  int
	Remark        string
	ForceCreate   bool
	ActorUserID   int
	ActorTenantID int
}

type SaaSAdminCustomerSuccessRenewalNotificationsResult struct {
	Options              SaaSAdminCustomerSuccessOptions
	MatchedCount         int
	EnqueuedCount        int
	SkippedExistingCount int
	SkippedInvalidCount  int
	Channel              string
	MaxAttempts          int
	ReminderDays         int
	Remark               string
	ForceCreate          bool
	Notifications        []SaaSAlertNotification
	Skipped              []SaaSAdminCustomerSuccessRenewalTaskSkipped
}

type SaaSAdminRenewalForecastNotifications struct {
	Options       SaaSAdminRenewalForecastOptions
	Channel       string
	MaxAttempts   int
	ReminderDays  int
	Remark        string
	ForceCreate   bool
	ActorUserID   int
	ActorTenantID int
}

type SaaSAdminRenewalForecastNotificationsResult struct {
	Options              SaaSAdminRenewalForecastOptions
	MatchedCount         int
	EnqueuedCount        int
	SkippedExistingCount int
	SkippedInvalidCount  int
	Channel              string
	MaxAttempts          int
	ReminderDays         int
	Remark               string
	ForceCreate          bool
	Notifications        []SaaSAlertNotification
	Skipped              []SaaSAdminCustomerSuccessRenewalTaskSkipped
}

type SaaSAdminCustomerSuccessRenewalTaskSkipped struct {
	TenantID   int
	TenantName string
	Reason     string
}

type SaaSAdminRiskFollowUp struct {
	TenantID       int
	Status         string
	Owner          string
	NextFollowUpAt string
	Remark         string
	ActorUserID    int
	ActorTenantID  int
}

type SaaSAdminRiskFollowUpSnapshot struct {
	TenantID       int
	TenantName     string
	Status         string
	Owner          string
	NextFollowUpAt string
	Remark         string
	OperationID    int64
	CreatedAt      string
}

type SaaSAdminRiskFollowUpTaskOptions struct {
	TenantID         int
	ExcludedTenantID int
	Status           string
	Owner            string
	DueState         string
	Keyword          string
	Limit            int
}

type SaaSAdminRiskFollowUpTask struct {
	SaaSAdminRiskFollowUpSnapshot
	DueState  string
	Overdue   bool
	DaysUntil int
}

type SaaSAdminRiskFollowUpTaskSummary struct {
	TotalCount          int
	PendingCount        int
	ContactedCount      int
	RenewalPendingCount int
	ResolvedCount       int
	IgnoredCount        int
	OverdueCount        int
	DueSoonCount        int
	NoDateCount         int
	ClosedCount         int
}

type SaaSAdminRiskFollowUpOwnerSummary struct {
	Owner               string
	TotalCount          int
	OpenCount           int
	PendingCount        int
	ContactedCount      int
	RenewalPendingCount int
	ResolvedCount       int
	IgnoredCount        int
	OverdueCount        int
	DueSoonCount        int
	FutureCount         int
	NoDateCount         int
	ClosedCount         int
	LatestFollowUpAt    string
	NextFollowUpAt      string
}

type SaaSAdminRiskFollowUpResult struct {
	TenantID       int
	TenantName     string
	Status         string
	Owner          string
	NextFollowUpAt string
	Remark         string
	OperationID    int64
}

type SaaSAdminRiskFollowUpBulkClose struct {
	Options       SaaSAdminRiskFollowUpTaskOptions
	Status        string
	Remark        string
	ActorUserID   int
	ActorTenantID int
}

type SaaSAdminRiskFollowUpBulkCloseResult struct {
	ClosedCount int
	Status      string
	Remark      string
	FollowUps   []SaaSAdminRiskFollowUpResult
}

type SaaSAdminDailyReportOptions struct {
	ReportDate     string
	Days           int
	WindowStart    time.Time
	WindowEnd      time.Time
	ExpiringDays   int
	HighUsageRatio float64
	TenantLimit    int
	ItemLimit      int
}

type SaaSAdminDailyReportData struct {
	Options                SaaSAdminDailyReportOptions
	Overview               SaaSAdminOverview
	RiskReport             SaaSAdminRiskReport
	RiskTaskSummary        SaaSAdminRiskFollowUpTaskSummary
	RiskOwners             []SaaSAdminRiskFollowUpOwnerSummary
	TaskSLAReport          SaaSAdminTaskSLAReport
	AlertPage              SaaSAlertListPage
	NotificationSummary    SaaSAdminDailyNotificationSummary
	RetryableNotifications []SaaSAlertNotification
	ClosedNotifications    []SaaSAlertNotification
	QueueAssignmentReport  SaaSAdminOperationQueueAssignmentReport
	OperationActionSummary []SaaSAdminDailyOperationActionSummary
	WindowOperations       []SaaSAdminOperationLog
	BillingSummary         SaaSAdminDailyBillingSummary
	WindowBillingEvents    []SaaSAdminBillingEvent
	Summary                SaaSAdminDailyReportSummary
}

type SaaSAdminDailyReportSummary struct {
	TenantCount                         int
	ActiveTenantPackageCount            int
	UserCount                           int
	CorpCount                           int
	ExpiringSoonTenantCount             int
	ExpiredTenantCount                  int
	EvaluatedRiskTenantCount            int
	RiskTenantCount                     int
	CriticalRiskTenantCount             int
	HighRiskTenantCount                 int
	OpenRiskFollowUpCount               int
	OverdueRiskFollowUpCount            int
	DueSoonRiskFollowUpCount            int
	RiskFollowUpOwnerCount              int
	OpenAlertCount                      int
	TaskSLAActiveCount                  int
	TaskSLAWarningCount                 int
	TaskSLAOverdueCount                 int
	TaskSLAOwnerCount                   int
	TaskSLAMaxAgeHours                  int
	PendingNotificationCount            int
	FailedNotificationCount             int
	DeadNotificationCount               int
	ClosedNotificationCount             int
	RetryableNotificationCount          int
	WindowQueueAssignmentCount          int
	WindowTaskSLAAssignCount            int
	WindowNotificationAssignCount       int
	WindowClosedNotificationAssignCount int
	WindowNotificationHealthAssignCount int
	WindowOperationCount                int
	WindowBillingEventCount             int
	WindowBillingAmountCents            int64
	WindowRenewalCount                  int
	WindowRefundCount                   int
	WindowGrossBillingAmountCents       int64
	WindowRefundAmountCents             int64
	WindowRiskFollowUpCount             int
	WindowAlertResolveCount             int
	WindowNotificationRetryCount        int
	WindowNotificationCloseCount        int
}

type SaaSAdminDailyNotificationSummary struct {
	PendingCount    int
	FailedCount     int
	DeadCount       int
	ClosedCount     int
	SuppressedCount int
	DeliveredCount  int
	RetryableCount  int
}

type SaaSAdminDailyBillingSummary struct {
	EventCount        int
	RenewalCount      int
	RefundCount       int
	GrossAmountCents  int64
	RefundAmountCents int64
	AmountCents       int64
}

type SaaSAdminDailyOperationActionSummary struct {
	Action string
	Count  int
}

type SaaSAdminSummary struct {
	TenantCount              int
	ActiveTenantPackageCount int
	EnabledPackageCount      int
	UserCount                int
	CorpCount                int
	OpenAlertCount           int
	PendingNotificationCount int
	ExpiringSoonTenantCount  int
	ExpiredTenantCount       int
}

type SaaSAdminTenantOverview struct {
	TenantID        int
	TenantName      string
	TenantStatus    int
	PackageCode     string
	PackageName     string
	PackageStatus   int
	PackageVersion  int
	PackageLimits   SaaSAdminPackageLimits
	ExpiresAt       string
	Expired         bool
	ExpiringSoon    bool
	OpenAlertCount  int
	MaxUsageMetric  string
	MaxUsageCurrent int64
	MaxUsageLimit   int64
	MaxUsageRatio   float64
}

type SaaSAdminMetricOverview struct {
	Metric         string
	Current        int64
	Limit          int64
	UsageRatio     float64
	OpenAlertCount int
}

type SaaSAdminUsageSummary struct {
	MetricCount          int
	LimitedMetricCount   int
	UnlimitedMetricCount int
	OpenAlertMetricCount int
	ExceededMetricCount  int
	WarningMetricCount   int
	HighestUsageMetric   string
	HighestUsageRatio    float64
}

type SaaSAdminUsageMetric struct {
	Metric         string
	PeriodKey      string
	Current        int64
	Limit          int64
	Remaining      int64
	Unlimited      bool
	UsageRatio     float64
	Status         string
	OpenAlertCount int
	UpdatedBy      string
	UpdatedAt      string
}

type SaaSAdminAlertResolve struct {
	TenantID      int
	Metric        string
	AlertType     string
	PeriodKey     string
	Remark        string
	ActorUserID   int
	ActorTenantID int
}

type SaaSAdminAlertResolveResult struct {
	Resolved       bool
	TenantID       int
	TenantName     string
	AlertID        int64
	AlertKey       string
	Metric         string
	AlertType      string
	PeriodKey      string
	PreviousStatus string
	Status         string
	Remark         string
	OperationID    int64
}

type SaaSAdminAlertBulkResolve struct {
	TenantID        int
	AllowedTenantID int
	Metric          string
	AlertType       string
	PeriodKey       string
	Limit           int
	Remark          string
	ActorUserID     int
	ActorTenantID   int
}

type SaaSAdminAlertBulkResolveResult struct {
	ResolvedCount int
	TenantID      int
	Metric        string
	AlertType     string
	PeriodKey     string
	Limit         int
	Remark        string
	Alerts        []SaaSAdminAlertResolveResult
}

type SaaSAdminAlertSummary struct {
	AlertCount    int
	OpenCount     int
	ResolvedCount int
	WarningCount  int
	CriticalCount int
	MetricCount   int
	TenantCount   int
}

type SaaSAdminAlertNotificationOptions struct {
	TenantID         int
	ExcludedTenantID int
	Status           string
	Channel          string
	Keyword          string
	Limit            int
}

type SaaSAdminAlertNotificationSummary struct {
	NotificationCount int
	PendingCount      int
	FailedCount       int
	DeliveredCount    int
	DeadCount         int
	ClosedCount       int
	SuppressedCount   int
	RetryableCount    int
	TenantCount       int
	ChannelCount      int
}

type SaaSAdminAlertNotificationRetry struct {
	NotificationID  int64
	AllowedTenantID int
	Remark          string
	ActorUserID     int
	ActorTenantID   int
}

type SaaSAdminAlertNotificationRetryResult struct {
	Retried         bool
	NotificationID  int64
	NotificationKey string
	TenantID        int
	AlertKey        string
	Channel         string
	PreviousStatus  string
	Status          string
	Attempts        int
	MaxAttempts     int
	Remark          string
	OperationID     int64
}

type SaaSAdminAlertNotificationBulkRetry struct {
	TenantID        int
	AllowedTenantID int
	Status          string
	Channel         string
	Keyword         string
	Limit           int
	Remark          string
	ActorUserID     int
	ActorTenantID   int
}

type SaaSAdminAlertNotificationBulkRetryResult struct {
	RetriedCount  int
	TenantID      int
	Status        string
	Channel       string
	Keyword       string
	Limit         int
	Remark        string
	Notifications []SaaSAdminAlertNotificationRetryResult
}

type SaaSAdminAlertNotificationClose struct {
	NotificationID  int64
	AllowedTenantID int
	Remark          string
	ActorUserID     int
	ActorTenantID   int
}

type SaaSAdminAlertNotificationCloseResult struct {
	Closed          bool
	NotificationID  int64
	NotificationKey string
	TenantID        int
	AlertKey        string
	Channel         string
	PreviousStatus  string
	Status          string
	Attempts        int
	MaxAttempts     int
	Remark          string
	OperationID     int64
}

type SaaSAdminAlertNotificationBulkClose struct {
	TenantID        int
	AllowedTenantID int
	Status          string
	Channel         string
	Keyword         string
	Limit           int
	Remark          string
	ActorUserID     int
	ActorTenantID   int
}

type SaaSAdminAlertNotificationBulkCloseResult struct {
	ClosedCount   int
	TenantID      int
	Status        string
	Channel       string
	Keyword       string
	Limit         int
	Remark        string
	Notifications []SaaSAdminAlertNotificationCloseResult
}

type SaaSAdminPackage struct {
	ID          int                    `json:"id"`
	Code        string                 `json:"code"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      int                    `json:"status"`
	Version     int                    `json:"version"`
	Limits      SaaSAdminPackageLimits `json:"limits"`
}

type SaaSAdminPackageLimits struct {
	MaxCorps              int64 `json:"maxCorps"`
	MaxUsers              int64 `json:"maxUsers"`
	MaxContacts           int64 `json:"maxContacts"`
	MaxRooms              int64 `json:"maxRooms"`
	MaxAgents             int64 `json:"maxAgents"`
	ChannelCodes          int64 `json:"channelCodes"`
	ShopCodes             int64 `json:"shopCodes"`
	Radars                int64 `json:"radars"`
	Lotteries             int64 `json:"lotteries"`
	RoomInfinitePulls     int64 `json:"roomInfinitePulls"`
	RoomFissions          int64 `json:"roomFissions"`
	RoomClockIns          int64 `json:"roomClockIns"`
	RoomQualities         int64 `json:"roomQualities"`
	RoomCalendars         int64 `json:"roomCalendars"`
	RoomReminds           int64 `json:"roomReminds"`
	ContactSOPs           int64 `json:"contactSops"`
	RoomSOPs              int64 `json:"roomSops"`
	SensitiveWords        int64 `json:"sensitiveWords"`
	StorageMB             int64 `json:"storageMb"`
	ContactMessageBatches int64 `json:"contactMessageBatches"`
	RoomMessageBatches    int64 `json:"roomMessageBatches"`
	RoomTagPulls          int64 `json:"roomTagPulls"`
	WorkRoomAutoPulls     int64 `json:"workRoomAutoPulls"`
	WorkFissions          int64 `json:"workFissions"`
	OfficialAccounts      int64 `json:"officialAccounts"`
	AsyncExecutions       int64 `json:"asyncExecutions"`
}

type SaaSAdminPackageUpsert struct {
	Code                     string                 `json:"code"`
	Name                     string                 `json:"name"`
	Description              string                 `json:"description"`
	Status                   int                    `json:"status"`
	Limits                   SaaSAdminPackageLimits `json:"limits"`
	ExpectedVersion          int                    `json:"expectedVersion"`
	ActorUserID              int                    `json:"-"`
	ActorTenantID            int                    `json:"-"`
	ApprovalExecutionID      int64                  `json:"-"`
	ApprovalExecutionVersion int                    `json:"-"`
}

type SaaSAdminPackageUpsertPlan struct {
	Update  SaaSAdminPackageUpsert `json:"update"`
	Current *SaaSAdminPackage      `json:"current,omitempty"`
	Impact  SaaSAdminPackageImpact `json:"impact"`
}

type SaaSAdminPackageLimitChange struct {
	Field     string `json:"field"`
	Metric    string `json:"metric"`
	Label     string `json:"label"`
	Before    int64  `json:"before"`
	After     int64  `json:"after"`
	Delta     int64  `json:"delta"`
	Direction string `json:"direction"`
}

type SaaSAdminPackageImpactTenant struct {
	TenantID   int    `json:"tenantId"`
	TenantName string `json:"tenantName"`
	Metric     string `json:"metric"`
	Label      string `json:"label"`
	Current    int64  `json:"current"`
	Limit      int64  `json:"limit"`
}

type SaaSAdminPackageImpact struct {
	Existing               bool                           `json:"existing"`
	AssignedTenantCount    int                            `json:"assignedTenantCount"`
	CheckedTenantCount     int                            `json:"checkedTenantCount"`
	OverLimitTenantCount   int                            `json:"overLimitTenantCount"`
	OverLimitTenants       []SaaSAdminPackageImpactTenant `json:"overLimitTenants"`
	StatusChanged          bool                           `json:"statusChanged"`
	BeforeStatus           int                            `json:"beforeStatus"`
	AfterStatus            int                            `json:"afterStatus"`
	ChangedLimitCount      int                            `json:"changedLimitCount"`
	IncreasedLimitCount    int                            `json:"increasedLimitCount"`
	DecreasedLimitCount    int                            `json:"decreasedLimitCount"`
	NewlyLimitedCount      int                            `json:"newlyLimitedCount"`
	NewlyUnlimitedCount    int                            `json:"newlyUnlimitedCount"`
	Changes                []SaaSAdminPackageLimitChange  `json:"changes"`
	TenantSnapshotsUpdated bool                           `json:"tenantSnapshotsUpdated"`
}

type SaaSAdminPackageTenantSnapshotSync struct {
	PackageCode    string
	TenantID       int
	Limit          int
	DryRun         bool
	AllowOverLimit bool
	Remark         string
	ActorUserID    int
	ActorTenantID  int
}

type SaaSAdminPackageTenantSnapshotSyncTenant struct {
	TenantID         int
	TenantName       string
	TenantStatus     int
	Version          int
	ExpiresAt        string
	Synced           bool
	Skipped          bool
	MetricsRefreshed int
	OverLimitMetrics []SaaSAdminPackageImpactTenant
}

type SaaSAdminPackageTenantSnapshotSyncResult struct {
	PackageCode            string
	PackageName            string
	PackageVersion         int
	Limit                  int
	DryRun                 bool
	AllowOverLimit         bool
	TenantSnapshotsUpdated bool
	MatchedTenantCount     int
	CheckedTenantCount     int
	SyncedTenantCount      int
	OverLimitTenantCount   int
	MetricsRefreshed       int
	Blocked                bool
	BlockReason            string
	Tenants                []SaaSAdminPackageTenantSnapshotSyncTenant
	OverLimitTenants       []SaaSAdminPackageImpactTenant
}

type SaaSAdminOperationLogOptions struct {
	TenantID         int
	ExcludedTenantID int
	Limit            int
	Action           string
	TargetType       string
	Keyword          string
}

type SaaSAdminOperationLogSummary struct {
	OperationCount  int
	TenantCount     int
	ActorUserCount  int
	ActionCount     int
	TargetTypeCount int
}

type SaaSAdminOperationLog struct {
	ID            int64
	TenantID      int
	ActorUserID   int
	ActorTenantID int
	Action        string
	TargetType    string
	TargetID      string
	TargetName    string
	BeforeJSON    string
	AfterJSON     string
	Remark        string
	CreatedAt     string
}

type SaaSAdminBillingEventOptions struct {
	TenantID         int
	ExcludedTenantID int
	Limit            int
	EventType        string
	PackageCode      string
	Keyword          string
}

type SaaSAdminBillingEvent struct {
	ID                int64
	TenantID          int
	EventType         string
	PackageCode       string
	PackageName       string
	PreviousExpiresAt string
	NewExpiresAt      string
	AmountCents       int64
	Currency          string
	PaidAt            string
	PaymentMethod     string
	ExternalOrderNo   string
	ActorUserID       int
	ActorTenantID     int
	Remark            string
	MetadataJSON      string
	CreatedAt         string
}

type SaaSAdminBillingReconciliationOptions struct {
	SaaSAdminBillingEventOptions
	MismatchOnly bool
}

type SaaSAdminBillingReconciliationReport struct {
	Summary SaaSAdminBillingReconciliationSummary
	Items   []SaaSAdminBillingReconciliationItem
}

type SaaSAdminBillingReconciliationSummary struct {
	CheckedCount         int
	MatchedCount         int
	MismatchedCount      int
	MissingPackageCount  int
	InactivePackageCount int
	PackageMismatchCount int
	ExpiresMismatchCount int
}

type SaaSAdminBillingReconciliationItem struct {
	BillingEvent         SaaSAdminBillingEvent
	TenantName           string
	CurrentPackageFound  bool
	CurrentPackageCode   string
	CurrentPackageName   string
	CurrentExpiresAt     string
	CurrentPackageStatus int
	Status               string
	Reasons              []string
}

type SaaSAdminBillingReconciliationFollowUp struct {
	BillingEventID int64
	Status         string
	Owner          string
	NextFollowUpAt string
	Remark         string
	ActorUserID    int
	ActorTenantID  int
}

type SaaSAdminBillingReconciliationFollowUpResult struct {
	OperationID    int64
	BillingEvent   SaaSAdminBillingEvent
	Status         string
	Owner          string
	NextFollowUpAt string
	Remark         string
	ActorUserID    int
	ActorTenantID  int
	FollowedUpAt   string
}

type SaaSAdminBillingReconciliationFollowUpBulkClose struct {
	Options       SaaSAdminBillingReconciliationFollowUpOptions
	Status        string
	Remark        string
	ActorUserID   int
	ActorTenantID int
}

type SaaSAdminBillingReconciliationFollowUpBulkCloseResult struct {
	ClosedCount int
	Status      string
	Remark      string
	FollowUps   []SaaSAdminBillingReconciliationFollowUpResult
}

type SaaSAdminBillingReconciliationFollowUpOptions struct {
	TenantID         int
	ExcludedTenantID int
	Status           string
	Owner            string
	DueState         string
	Keyword          string
	Limit            int
}

type SaaSAdminBillingReconciliationFollowUpSnapshot struct {
	OperationID     int64
	BillingEventID  int64
	TenantID        int
	TenantName      string
	Status          string
	Owner           string
	NextFollowUpAt  string
	Remark          string
	PackageCode     string
	PackageName     string
	NewExpiresAt    string
	AmountCents     int64
	Currency        string
	ExternalOrderNo string
	CreatedAt       string
}

type SaaSAdminBillingReconciliationFollowUpTask struct {
	SaaSAdminBillingReconciliationFollowUpSnapshot
	DueState  string
	Overdue   bool
	DaysUntil int
}

type SaaSAdminTaskOptions struct {
	TaskID           int64
	TaskType         string
	Status           string
	TenantID         int
	ExcludedTenantID int
	PackageCode      string
	Limit            int
}

type SaaSAdminTask struct {
	ID            int64
	TaskType      string
	Status        string
	Version       int
	TenantID      int
	PackageCode   string
	ActorUserID   int
	ActorTenantID int
	RequestJSON   string
	PreviewJSON   string
	ResultJSON    string
	Remark        string
	LastError     string
	AppliedAt     string
	CreatedAt     string
	UpdatedAt     string
}

type SaaSAdminTaskSummary struct {
	TaskCount            int
	PendingCount         int
	BlockedCount         int
	FailedCount          int
	AppliedCount         int
	CanceledCount        int
	ActionableCount      int
	PackageSyncCount     int
	TenantRenewalCount   int
	TenantProvisionCount int
	TenantCount          int
	ActorUserCount       int
}

type SaaSAdminTaskOwnerSummary struct {
	Owner         string
	ActorUserID   int
	ActorTenantID int
	Summary       SaaSAdminTaskSummary
	LastTaskAt    string
	LastAppliedAt string
	LastError     string
	RecentTasks   []SaaSAdminTask
}

type SaaSAdminTaskOwnerReport struct {
	Options          SaaSAdminTaskOptions
	Summary          SaaSAdminTaskSummary
	OwnerCount       int
	ScannedTaskCount int
	Partial          bool
	Owners           []SaaSAdminTaskOwnerSummary
}

type SaaSAdminTaskSLAOptions struct {
	SaaSAdminTaskOptions
	WarningHours int
	OverdueHours int
}

type SaaSAdminTaskSLASummary struct {
	TaskCount            int
	FreshCount           int
	WarningCount         int
	OverdueCount         int
	UnknownAgeCount      int
	PendingCount         int
	BlockedCount         int
	FailedCount          int
	PackageSyncCount     int
	TenantRenewalCount   int
	TenantProvisionCount int
	TenantCount          int
	ActorUserCount       int
	MaxAgeHours          int
}

type SaaSAdminTaskSLAItem struct {
	Task        SaaSAdminTask
	Owner       string
	AgeHours    int
	SLAStatus   string
	BreachHours int
}

type SaaSAdminTaskSLAOwnerSummary struct {
	Owner         string
	ActorUserID   int
	ActorTenantID int
	Summary       SaaSAdminTaskSLASummary
	OldestTaskAt  string
	Tasks         []SaaSAdminTaskSLAItem
}

type SaaSAdminTaskSLAReport struct {
	Options          SaaSAdminTaskSLAOptions
	TaskSummary      SaaSAdminTaskSummary
	Summary          SaaSAdminTaskSLASummary
	OwnerCount       int
	ScannedTaskCount int
	Partial          bool
	Owners           []SaaSAdminTaskSLAOwnerSummary
	Tasks            []SaaSAdminTaskSLAItem
}

type SaaSAdminTaskSLANotifications struct {
	Options       SaaSAdminTaskSLAOptions
	Channel       string
	MaxAttempts   int
	SLAStatus     string
	Remark        string
	ForceCreate   bool
	ActorUserID   int
	ActorTenantID int
}

type SaaSAdminTaskSLANotificationsResult struct {
	Options              SaaSAdminTaskSLAOptions
	Summary              SaaSAdminTaskSLASummary
	MatchedCount         int
	EligibleCount        int
	EnqueuedCount        int
	SkippedExistingCount int
	SkippedInvalidCount  int
	SkippedStatusCount   int
	Channel              string
	MaxAttempts          int
	SLAStatus            string
	Remark               string
	ForceCreate          bool
	Notifications        []SaaSAlertNotification
	Skipped              []SaaSAdminTaskSLANotificationSkipped
}

type SaaSAdminTaskSLANotificationSkipped struct {
	TaskID   int64
	TenantID int
	Reason   string
}

type SaaSAdminTaskCreate struct {
	TaskType      string
	Status        string
	TenantID      int
	PackageCode   string
	ActorUserID   int
	ActorTenantID int
	RequestJSON   string
	PreviewJSON   string
	Remark        string
}

type SaaSAdminTaskStatusUpdate struct {
	TaskID          int64
	Status          string
	ResultJSON      string
	LastError       string
	Applied         bool
	ExpectedVersion int
}

type SaaSAdminTaskBulkCancel struct {
	Options       SaaSAdminTaskOptions
	Remark        string
	ActorUserID   int
	ActorTenantID int
}

type SaaSAdminTaskBulkCancelResult struct {
	MatchedCount           int
	CanceledCount          int
	SkippedAppliedCount    int
	SkippedCanceledCount   int
	SkippedIneligibleCount int
	Options                SaaSAdminTaskOptions
	Remark                 string
	Tasks                  []SaaSAdminTask
}

type SaaSAdminTaskBulkResetResult struct {
	MatchedCount           int
	ResetCount             int
	SkippedPendingCount    int
	SkippedAppliedCount    int
	SkippedCanceledCount   int
	SkippedIneligibleCount int
	Options                SaaSAdminTaskOptions
	Remark                 string
	Tasks                  []SaaSAdminTask
}

type SaaSAdminTaskBulkApplyResult struct {
	MatchedCount            int
	AppliedCount            int
	BlockedCount            int
	FailedCount             int
	SkippedAppliedCount     int
	SkippedCanceledCount    int
	SkippedUnsupportedCount int
	Options                 SaaSAdminTaskOptions
	Remark                  string
	Tasks                   []SaaSAdminTask
	Errors                  []SaaSAdminTaskBulkApplyError
}

type SaaSAdminTaskBulkApplyError struct {
	TaskID   int64
	TenantID int
	Error    string
}

type SaaSAdminTenantStatusUpdate struct {
	TenantID                 int                                `json:"tenantId"`
	Status                   int                                `json:"status"`
	Remark                   string                             `json:"remark"`
	ExpectedStatus           int                                `json:"expectedStatus,omitempty"`
	ActorUserID              int                                `json:"-"`
	ActorTenantID            int                                `json:"-"`
	ApprovalPlan             *SaaSAdminTenantStatusApprovalPlan `json:"-"`
	ApprovalExecutionID      int64                              `json:"-"`
	ApprovalExecutionVersion int                                `json:"-"`
}

const SaaSAdminTenantStatusApprovalPlanSchemaVersion = 1

type SaaSAdminTenantStatusApprovalSnapshot struct {
	TenantID            int    `json:"tenantId"`
	TenantName          string `json:"tenantName"`
	TenantStatus        int    `json:"tenantStatus"`
	SubscriptionPresent bool   `json:"subscriptionPresent"`
	SubscriptionID      int64  `json:"subscriptionId,omitempty"`
	SubscriptionStatus  string `json:"subscriptionStatus,omitempty"`
	SubscriptionVersion int    `json:"subscriptionVersion,omitempty"`
}

type SaaSAdminTenantStatusApprovalPlan struct {
	SchemaVersion int                                   `json:"schemaVersion"`
	Update        SaaSAdminTenantStatusUpdate           `json:"update"`
	Snapshot      SaaSAdminTenantStatusApprovalSnapshot `json:"snapshot"`
}

type SaaSAdminTenantStatusUpdateResult struct {
	TenantID       int
	TenantName     string
	PreviousStatus int
	Status         int
	Remark         string
	OperationID    int64
}

type SaaSAdminTenantPackageUpdate struct {
	TenantID                 int    `json:"tenantId"`
	PackageCode              string `json:"packageCode"`
	ExpiresAt                string `json:"expiresAt"`
	Remark                   string `json:"remark"`
	ExpectedVersion          int    `json:"expectedVersion"`
	ExpectedPackageVersion   int    `json:"expectedPackageVersion"`
	ExpectedTenantStatus     int    `json:"expectedTenantStatus"`
	ActorUserID              int    `json:"-"`
	ActorTenantID            int    `json:"-"`
	ApprovalExecutionID      int64  `json:"-"`
	ApprovalExecutionVersion int    `json:"-"`
}

type SaaSAdminTenantPackageSnapshot struct {
	TenantID     int                    `json:"tenantId"`
	TenantName   string                 `json:"tenantName"`
	TenantStatus int                    `json:"tenantStatus"`
	PackageCode  string                 `json:"packageCode"`
	PackageName  string                 `json:"packageName"`
	ExpiresAt    string                 `json:"expiresAt"`
	Status       int                    `json:"status"`
	Version      int                    `json:"version"`
	Limits       SaaSAdminPackageLimits `json:"limits"`
}

type SaaSAdminTenantPackageUpdatePlan struct {
	Update        SaaSAdminTenantPackageUpdate    `json:"update"`
	TenantName    string                          `json:"tenantName"`
	Current       *SaaSAdminTenantPackageSnapshot `json:"current,omitempty"`
	TargetPackage SaaSAdminPackage                `json:"targetPackage"`
	Impact        SaaSAdminPackageImpact          `json:"impact"`
}

type SaaSAdminTenantPackageUpdateResult struct {
	TenantID              int
	TenantName            string
	PackageCode           string
	PackageName           string
	ExpiresAt             string
	Status                int
	Version               int
	OperationID           int64
	MetricsRefreshed      int
	MetricsRefreshPending bool
	MetricsRefreshError   string
}

type SaaSAdminTenantRenewal struct {
	TenantID                         int    `json:"tenantId"`
	PackageCode                      string `json:"packageCode"`
	ExpiresAt                        string `json:"expiresAt"`
	AmountCents                      int64  `json:"amountCents"`
	Currency                         string `json:"currency"`
	PaidAt                           string `json:"paidAt"`
	PaymentMethod                    string `json:"paymentMethod"`
	ExternalOrderNo                  string `json:"externalOrderNo"`
	Remark                           string `json:"remark"`
	ExpectedPackageAssignmentVersion int    `json:"-"`
	ExpectedPackageVersion           int    `json:"-"`
	ExpectedTenantStatus             int    `json:"-"`
	ExpectedSubscriptionExists       bool   `json:"-"`
	ExpectedSubscriptionVersion      int    `json:"-"`
	TaskID                           int64  `json:"-"`
	ExpectedTaskVersion              int    `json:"-"`
	ExpectedTaskRequestSHA256        string `json:"-"`
	ActorUserID                      int    `json:"-"`
	ActorTenantID                    int    `json:"-"`
	ApprovalExecutionID              int64  `json:"-"`
	ApprovalExecutionVersion         int    `json:"-"`
}

type SaaSAdminTenantRenewalResult struct {
	TenantID              int
	TenantName            string
	PackageCode           string
	PackageName           string
	PreviousExpiresAt     string
	ExpiresAt             string
	AmountCents           int64
	Currency              string
	BillingEventID        int64
	OperationID           int64
	MetricsRefreshed      int
	MetricsRefreshPending bool
	MetricsRefreshError   string
}

type SaaSAdminTenantRenewalPreview struct {
	TenantID                      int    `json:"tenantId"`
	TenantName                    string `json:"tenantName"`
	TenantStatus                  int    `json:"tenantStatus"`
	PreviousPackageCode           string `json:"previousPackageCode"`
	PreviousPackageName           string `json:"previousPackageName"`
	PreviousPackageStatus         int    `json:"previousPackageStatus"`
	PreviousPackageVersion        int    `json:"previousPackageVersion"`
	PreviousExpiresAt             string `json:"previousExpiresAt"`
	PackageCode                   string `json:"packageCode"`
	PackageName                   string `json:"packageName"`
	PackageVersion                int    `json:"packageVersion"`
	ExpiresAt                     string `json:"expiresAt"`
	AmountCents                   int64  `json:"amountCents"`
	Currency                      string `json:"currency"`
	PaidAt                        string `json:"paidAt"`
	PaymentMethod                 string `json:"paymentMethod"`
	ExternalOrderNo               string `json:"externalOrderNo"`
	Remark                        string `json:"remark"`
	SubscriptionExists            bool   `json:"subscriptionExists"`
	SubscriptionStatus            string `json:"subscriptionStatus"`
	SubscriptionVersion           int    `json:"subscriptionVersion"`
	SubscriptionCurrentPeriodEnds string `json:"subscriptionCurrentPeriodEnds"`
	Blocked                       bool   `json:"blocked"`
	BlockReason                   string `json:"blockReason"`
}

type SaaSAdminTenantRenewalSubscriptionSnapshot struct {
	Exists               bool   `json:"exists"`
	Status               string `json:"status"`
	Version              int    `json:"version"`
	PackageCode          string `json:"packageCode"`
	PackageName          string `json:"packageName"`
	CurrentPeriodEndsAt  string `json:"currentPeriodEndsAt"`
	LatestBillingEventID int64  `json:"latestBillingEventId"`
	CancelAtPeriodEnd    bool   `json:"cancelAtPeriodEnd"`
}

type SaaSAdminTenantRenewalApprovalPlan struct {
	Source                    string                                     `json:"source"`
	TaskID                    int64                                      `json:"taskId,omitempty"`
	ExpectedTaskVersion       int                                        `json:"expectedTaskVersion,omitempty"`
	ExpectedTaskRequestSHA256 string                                     `json:"expectedTaskRequestSha256,omitempty"`
	ExpectedTenantStatus      int                                        `json:"expectedTenantStatus"`
	Renewal                   SaaSAdminTenantRenewal                     `json:"renewal"`
	Preview                   SaaSAdminTenantRenewalPreview              `json:"preview"`
	CurrentPackage            *SaaSAdminTenantPackageSnapshot            `json:"currentPackage,omitempty"`
	TargetPackage             SaaSAdminPackage                           `json:"targetPackage"`
	Subscription              SaaSAdminTenantRenewalSubscriptionSnapshot `json:"subscription"`
}

type SaaSAdminTenantProvision struct {
	TenantID                  int    `json:"tenantId"`
	TenantName                string `json:"tenantName"`
	AdminPhone                string `json:"adminPhone"`
	AdminName                 string `json:"adminName"`
	AdminPasswordHash         string `json:"adminPasswordHash"`
	RoleName                  string `json:"roleName"`
	PackageCode               string `json:"packageCode"`
	ExpiresAt                 string `json:"expiresAt"`
	ConfigCopyMode            string `json:"configCopyMode"`
	Remark                    string `json:"remark"`
	ExpectedPackageVersion    int    `json:"-"`
	TaskID                    int64  `json:"-"`
	ExpectedTaskVersion       int    `json:"-"`
	ExpectedTaskRequestSHA256 string `json:"-"`
	ActorUserID               int    `json:"-"`
	ActorTenantID             int    `json:"-"`
	ApprovalExecutionID       int64  `json:"-"`
	ApprovalExecutionVersion  int    `json:"-"`
}

type SaaSAdminTenantProvisionResult struct {
	TenantID              int
	TenantName            string
	AdminUserID           int
	AdminPhone            string
	AdminName             string
	RoleID                int
	RoleName              string
	PackageCode           string
	PackageName           string
	ExpiresAt             string
	MenuCount             int
	ConfigCopyCount       int
	MetricsRefreshed      int
	MetricsRefreshPending bool
	MetricsRefreshError   string
	OperationID           int64
}

type SaaSAdminTenantProvisionPreview struct {
	TenantID       int    `json:"tenantId"`
	TenantName     string `json:"tenantName"`
	AdminPhone     string `json:"adminPhone"`
	AdminName      string `json:"adminName"`
	RoleName       string `json:"roleName"`
	PackageCode    string `json:"packageCode"`
	PackageName    string `json:"packageName"`
	ExpiresAt      string `json:"expiresAt"`
	ConfigCopyMode string `json:"configCopyMode"`
	Remark         string `json:"remark"`
	Blocked        bool   `json:"blocked"`
	BlockReason    string `json:"blockReason"`
}

type SaaSAdminTenantProvisionApprovalPlan struct {
	Source                    string                          `json:"source"`
	TaskID                    int64                           `json:"taskId,omitempty"`
	ExpectedTaskVersion       int                             `json:"expectedTaskVersion,omitempty"`
	ExpectedTaskRequestSHA256 string                          `json:"expectedTaskRequestSha256,omitempty"`
	CredentialFingerprint     string                          `json:"credentialFingerprint"`
	Provision                 SaaSAdminTenantProvision        `json:"provision"`
	Preview                   SaaSAdminTenantProvisionPreview `json:"preview"`
	Package                   SaaSAdminPackage                `json:"package"`
}

type SaaSAdminOperationError struct {
	Status  int
	Message string
}

func (e *SaaSAdminOperationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func NewSaaSAdminBadRequest(message string) error {
	return &SaaSAdminOperationError{Status: http.StatusBadRequest, Message: message}
}

func NewSaaSAdminNotFound(message string) error {
	return &SaaSAdminOperationError{Status: http.StatusNotFound, Message: message}
}

type SaaSAdminHandler struct {
	store                               SaaSAdminOverviewStore
	resolver                            UserIDResolver
	platformAdminTenantID               int
	passwordSecret                      string
	paymentSettlementSync               *SaaSPaymentSettlementSyncService
	highRiskApproval                    bool
	systemHealthProbes                  []SaaSAdminSystemHealthProbe
	systemHealthNotificationMaxAttempts int
	serviceAccountKeyManager            *serviceaccountkey.Manager
	serviceAccountClientIPResolver      *clientip.Resolver
	backupManager                       *saasbackup.Manager
	auditAnchorManager                  *saasauditanchor.Manager
	complianceManager                   *saascompliance.Manager
	identitySecurityManager             *identitysecurity.Manager
	dashboardAdminApprovalExecutor      func(context.Context, int, int64, int, string, json.RawMessage) (map[string]any, error)
	tenantDomainVerifier                SaaSTenantDomainOwnershipVerifier
	releaseSourceFingerprint            string
	releaseSourceFingerprintSource      string
	releaseEvidenceVerifier             SaaSReleaseArtifactVerifier
	webhookGuard                        *outboundhttp.Guard
}

func (h *SaaSAdminHandler) WithHighRiskApprovalRequired(required bool) *SaaSAdminHandler {
	h.highRiskApproval = required
	return h
}

func (h *SaaSAdminHandler) WithDashboardAdminApprovalExecutor(executor func(context.Context, int, int64, int, string, json.RawMessage) (map[string]any, error)) *SaaSAdminHandler {
	h.dashboardAdminApprovalExecutor = executor
	return h
}

func (h *SaaSAdminHandler) WithReleaseSourceFingerprint(value string, source ...string) *SaaSAdminHandler {
	h.releaseSourceFingerprint = strings.ToLower(strings.TrimSpace(value))
	h.releaseSourceFingerprintSource = ""
	if len(source) > 0 {
		h.releaseSourceFingerprintSource = strings.ToLower(strings.TrimSpace(source[0]))
	}
	return h
}

func (h *SaaSAdminHandler) WithReleaseEvidenceVerifier(verifier SaaSReleaseArtifactVerifier) *SaaSAdminHandler {
	h.releaseEvidenceVerifier = verifier
	return h
}

func (h *SaaSAdminHandler) WithWebhookGuard(guard *outboundhttp.Guard) *SaaSAdminHandler {
	h.webhookGuard = normalizeSaaSAlertWebhookGuard(guard)
	return h
}

func NewSaaSAdminHandler(store SaaSAdminOverviewStore, resolver UserIDResolver, platformAdminTenantID int, passwordSecret ...string) *SaaSAdminHandler {
	if platformAdminTenantID <= 0 {
		platformAdminTenantID = 1
	}
	secret := ""
	if len(passwordSecret) > 0 {
		secret = passwordSecret[0]
	}
	return &SaaSAdminHandler{
		store:                               store,
		resolver:                            resolver,
		platformAdminTenantID:               platformAdminTenantID,
		passwordSecret:                      secret,
		systemHealthNotificationMaxAttempts: 3,
		serviceAccountKeyManager:            serviceaccountkey.NewLegacyManager(secret),
		serviceAccountClientIPResolver:      clientip.DirectResolver(),
		webhookGuard:                        defaultSaaSAlertWebhookGuard,
	}
}

func (h *SaaSAdminHandler) saasAdminOverview(ctx context.Context, options SaaSAdminOverviewOptions) (SaaSAdminOverview, error) {
	if options.Scope == SaaSAdminScopePlatform {
		options.ExcludedTenantID = h.platformAdminTenantID
	}
	return h.store.SaaSAdminOverview(ctx, options)
}

func saasAdminTenantPopulation(scope string) string {
	if scope == SaaSAdminScopePlatform {
		return SaaSAdminTenantPopulationBusiness
	}
	return SaaSAdminTenantPopulationSelected
}

func (h *SaaSAdminHandler) Overview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	options, canPlatformScope, ok := h.overviewOptions(w, r, user)
	if !ok {
		return
	}
	accessPayload := map[string]any{"isPlatformSuperAdmin": false, "roles": []any{}, "permissions": []string{}}
	canReadTenantDetails := true
	if user.TenantID == h.platformAdminTenantID {
		profile, err := h.saasAdminAccessProfile(r.Context(), user)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		accessPayload = saasAdminAccessProfilePayload(profile)
		canReadTenantDetails = SaaSAdminAccessHasPermission(profile, SaaSAdminPermissionTenantsRead)
		if !canReadTenantDetails {
			hasTenantFilter := strings.TrimSpace(r.URL.Query().Get("tenantId")) != "" ||
				strings.TrimSpace(r.URL.Query().Get("keyword")) != "" ||
				strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("packageCode"), r.URL.Query().Get("package_code"))) != "" ||
				strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("tenantStatus"), r.URL.Query().Get("tenant_status"))) != "" ||
				(strings.TrimSpace(r.URL.Query().Get("dueState")) != "" && strings.TrimSpace(r.URL.Query().Get("dueState")) != SaaSAdminDueStateAll)
			if hasTenantFilter {
				writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "缺少平台权限 "+SaaSAdminPermissionTenantsRead, nil)
				return
			}
			options.Scope = SaaSAdminScopePlatform
			options.TenantID = 0
			options.Keyword = ""
			options.TenantStatus = 0
			options.PackageCode = ""
			options.DueState = SaaSAdminDueStateAll
		}
	}
	overview, err := h.saasAdminOverview(r.Context(), options)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !canReadTenantDetails {
		overview.Tenants = nil
		overview.Metrics = nil
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"scope":                 options.Scope,
		"tenantId":              options.TenantID,
		"tenantPopulation":      saasAdminTenantPopulation(options.Scope),
		"canPlatformScope":      canPlatformScope,
		"platformAdminTenantId": h.platformAdminTenantID,
		"access":                accessPayload,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminOverviewFiltersPayload(options),
		"summary":               saasAdminSummaryPayload(overview.Summary),
		"tenants":               saasAdminTenantPayloads(overview.Tenants),
		"metrics":               saasAdminMetricPayloads(overview.Metrics),
	})
}

func (h *SaaSAdminHandler) Packages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	packages, err := h.store.SaaSAdminPackages(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"packages": saasAdminPackagePayloads(packages),
	})
}

func (h *SaaSAdminHandler) OperationLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.operationLogOptions(w, r, positiveQueryInt(r, "limit", 20), saasAdminListMaxLimit)
	if !ok {
		return
	}
	summary, err := h.store.SaaSAdminOperationLogSummary(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	logs, err := h.store.SaaSAdminOperationLogs(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"filters":       saasAdminOperationLogFiltersPayload(options),
		"summary":       saasAdminOperationLogSummaryPayload(summary),
		"returnedCount": len(logs),
		"operations":    saasAdminOperationLogPayloads(logs),
	})
}

func (h *SaaSAdminHandler) recordTaskOperation(ctx context.Context, action string, before SaaSAdminTask, after SaaSAdminTask, user User, remark string, extra map[string]any) error {
	task := after
	if task.ID <= 0 {
		task = before
	}
	if task.ID <= 0 {
		return nil
	}
	beforeJSON := ""
	if before.ID > 0 {
		beforeJSON = saasAdminPayloadJSON(saasAdminTaskPayload(before))
	}
	afterJSON := ""
	if after.ID > 0 {
		payload := saasAdminTaskPayload(after)
		for key, value := range extra {
			payload[key] = value
		}
		afterJSON = saasAdminPayloadJSON(payload)
	}
	if strings.TrimSpace(remark) == "" {
		remark = action
	}
	_, err := h.store.RecordSaaSAdminOperationLog(ctx, SaaSAdminOperationLog{
		TenantID:      task.TenantID,
		ActorUserID:   user.ID,
		ActorTenantID: user.TenantID,
		Action:        action,
		TargetType:    SaaSAdminOperationTargetAdminTask,
		TargetID:      strconv.FormatInt(task.ID, 10),
		TargetName:    task.TaskType,
		BeforeJSON:    beforeJSON,
		AfterJSON:     afterJSON,
		Remark:        strings.TrimSpace(remark),
	})
	return err
}

func (h *SaaSAdminHandler) BillingEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.billingEventOptions(w, r, positiveQueryInt(r, "limit", 20), saasAdminListMaxLimit)
	if !ok {
		return
	}
	summary, err := h.store.SaaSAdminBillingEventSummary(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	events, err := h.store.SaaSAdminBillingEvents(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"filters":       saasAdminBillingEventFiltersPayload(options),
		"summary":       saasAdminDailyBillingSummaryPayload(summary),
		"returnedCount": len(events),
		"billingEvents": saasAdminBillingEventPayloads(events),
	})
}

func (h *SaaSAdminHandler) BillingReconciliation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.billingReconciliationOptions(w, r, positiveQueryInt(r, "limit", 20), saasAdminListMaxLimit)
	if !ok {
		return
	}
	report, err := h.store.SaaSAdminBillingReconciliation(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	report = saasAdminNormalizeBillingReconciliationReport(report)
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"filters":       saasAdminBillingReconciliationFiltersPayload(options),
		"summary":       saasAdminBillingReconciliationSummaryPayload(report.Summary),
		"returnedCount": len(report.Items),
		"items":         saasAdminBillingReconciliationPayloads(report.Items),
	})
}

func (h *SaaSAdminHandler) BillingReconciliationFollowUp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	followUp, err := parseSaaSAdminBillingReconciliationFollowUp(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	event, found, err := h.store.SaaSAdminBillingEventByID(r.Context(), followUp.BillingEventID)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "billing event not found", nil)
		return
	}

	followUp.ActorUserID = user.ID
	followUp.ActorTenantID = user.TenantID
	result, err := h.recordBillingReconciliationFollowUpOperation(r.Context(), event, followUp)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminBillingReconciliationFollowUpPayload(result))
}

func (h *SaaSAdminHandler) recordBillingReconciliationFollowUpOperation(ctx context.Context, event SaaSAdminBillingEvent, followUp SaaSAdminBillingReconciliationFollowUp) (SaaSAdminBillingReconciliationFollowUpResult, error) {
	followedUpAt := time.Now().Format("2006-01-02 15:04:05")
	afterPayload := map[string]any{
		"billingEventId":  event.ID,
		"tenantId":        event.TenantID,
		"status":          followUp.Status,
		"owner":           followUp.Owner,
		"nextFollowUpAt":  followUp.NextFollowUpAt,
		"remark":          followUp.Remark,
		"packageCode":     event.PackageCode,
		"externalOrderNo": event.ExternalOrderNo,
		"actorUserId":     followUp.ActorUserID,
		"actorTenantId":   followUp.ActorTenantID,
		"followedUpAt":    followedUpAt,
	}
	targetName := strings.TrimSpace(event.ExternalOrderNo)
	if targetName == "" {
		targetName = strings.TrimSpace(event.PackageCode)
	}
	if targetName == "" {
		targetName = strings.TrimSpace(event.EventType)
	}
	operationID, err := h.store.RecordSaaSAdminOperationLog(ctx, SaaSAdminOperationLog{
		TenantID:      event.TenantID,
		ActorUserID:   followUp.ActorUserID,
		ActorTenantID: followUp.ActorTenantID,
		Action:        SaaSAdminOperationActionBillingReconciliationFollowUp,
		TargetType:    SaaSAdminOperationTargetBillingEvent,
		TargetID:      strconv.FormatInt(event.ID, 10),
		TargetName:    targetName,
		BeforeJSON:    saasAdminPayloadJSON(saasAdminBillingEventPayload(event)),
		AfterJSON:     saasAdminPayloadJSON(afterPayload),
		Remark:        followUp.Remark,
	})
	if err != nil {
		return SaaSAdminBillingReconciliationFollowUpResult{}, err
	}
	return SaaSAdminBillingReconciliationFollowUpResult{
		OperationID:    operationID,
		BillingEvent:   event,
		Status:         followUp.Status,
		Owner:          followUp.Owner,
		NextFollowUpAt: followUp.NextFollowUpAt,
		Remark:         followUp.Remark,
		ActorUserID:    followUp.ActorUserID,
		ActorTenantID:  followUp.ActorTenantID,
		FollowedUpAt:   followedUpAt,
	}, nil
}

func (h *SaaSAdminHandler) BillingReconciliationFollowUps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.billingReconciliationFollowUpOptions(w, r, positiveQueryInt(r, "limit", 50), saasAdminListMaxLimit)
	if !ok {
		return
	}
	storeOptions := options
	storeOptions.DueState = SaaSAdminRiskFollowUpDueStateAll
	storeOptions.Limit = saasAdminExportMaxLimit
	snapshots, err := h.store.SaaSAdminBillingReconciliationFollowUpSnapshots(r.Context(), storeOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	tasks, summary := saasAdminBuildBillingReconciliationFollowUpTasks(snapshots, options, time.Now())
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"canPlatformScope":      true,
		"platformAdminTenantId": h.platformAdminTenantID,
		"filters":               saasAdminBillingReconciliationFollowUpFiltersPayload(options),
		"summary":               saasAdminRiskFollowUpTaskSummaryPayload(summary),
		"followUps":             saasAdminBillingReconciliationFollowUpTaskPayloads(tasks),
	})
}

func (h *SaaSAdminHandler) BillingReconciliationFollowUpOwners(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.billingReconciliationFollowUpOptions(w, r, saasAdminExportMaxLimit, saasAdminExportMaxLimit)
	if !ok {
		return
	}
	storeOptions := options
	storeOptions.DueState = SaaSAdminRiskFollowUpDueStateAll
	storeOptions.Limit = saasAdminExportMaxLimit
	snapshots, err := h.store.SaaSAdminBillingReconciliationFollowUpSnapshots(r.Context(), storeOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	tasks, summary := saasAdminBuildBillingReconciliationFollowUpTasks(snapshots, options, time.Now())
	owners := saasAdminBuildBillingReconciliationFollowUpOwnerSummaries(tasks)
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"canPlatformScope":      true,
		"platformAdminTenantId": h.platformAdminTenantID,
		"filters":               saasAdminBillingReconciliationFollowUpFiltersPayload(options),
		"summary":               saasAdminRiskFollowUpTaskSummaryPayload(summary),
		"owners":                saasAdminRiskFollowUpOwnerSummaryPayloads(owners),
	})
}

func (h *SaaSAdminHandler) BillingReconciliationFollowUpBulkClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	bulkClose, err := parseSaaSAdminBillingReconciliationFollowUpBulkClose(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	bulkClose.ActorUserID = user.ID
	bulkClose.ActorTenantID = user.TenantID

	storeOptions := bulkClose.Options
	storeOptions.DueState = SaaSAdminRiskFollowUpDueStateAll
	storeOptions.Limit = saasAdminExportMaxLimit
	snapshots, err := h.store.SaaSAdminBillingReconciliationFollowUpSnapshots(r.Context(), storeOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	tasks, _ := saasAdminBuildBillingReconciliationFollowUpTasks(snapshots, bulkClose.Options, time.Now())
	result := SaaSAdminBillingReconciliationFollowUpBulkCloseResult{
		Status: bulkClose.Status,
		Remark: bulkClose.Remark,
	}
	for _, task := range tasks {
		if task.Status == SaaSAdminRiskFollowUpStatusResolved || task.Status == SaaSAdminRiskFollowUpStatusIgnored {
			continue
		}
		event, found, err := h.store.SaaSAdminBillingEventByID(r.Context(), task.BillingEventID)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if !found {
			continue
		}
		followUp := SaaSAdminBillingReconciliationFollowUp{
			BillingEventID: task.BillingEventID,
			Status:         bulkClose.Status,
			Owner:          task.Owner,
			Remark:         bulkClose.Remark,
			ActorUserID:    bulkClose.ActorUserID,
			ActorTenantID:  bulkClose.ActorTenantID,
		}
		closed, err := h.recordBillingReconciliationFollowUpOperation(r.Context(), event, followUp)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		result.FollowUps = append(result.FollowUps, closed)
	}
	result.ClosedCount = len(result.FollowUps)
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminBillingReconciliationFollowUpBulkClosePayload(result))
}

func (h *SaaSAdminHandler) Tasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.taskOptions(w, r, positiveQueryInt(r, "limit", 20), saasAdminListMaxLimit)
	if !ok {
		return
	}
	summary, err := h.store.SaaSAdminTaskSummary(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	tasks, err := h.store.SaaSAdminTasks(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"filters":       saasAdminTaskFiltersPayload(options),
		"summary":       saasAdminTaskSummaryPayload(summary),
		"returnedCount": len(tasks),
		"tasks":         saasAdminTaskPayloads(tasks),
	})
}

func (h *SaaSAdminHandler) TaskOwners(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.taskOptions(w, r, positiveQueryInt(r, "limit", 20), saasAdminListMaxLimit)
	if !ok {
		return
	}
	summary, err := h.store.SaaSAdminTaskSummary(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	fetchOptions := options
	fetchOptions.Limit = saasAdminExportMaxLimit
	tasks, err := h.store.SaaSAdminTasks(r.Context(), fetchOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	report := saasAdminBuildTaskOwnerReport(options, summary, tasks, 3)
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTaskOwnerReportPayload(report))
}

func (h *SaaSAdminHandler) TaskSLA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.taskSLAOptions(w, r, positiveQueryInt(r, "limit", 20), saasAdminListMaxLimit)
	if !ok {
		return
	}
	summary, err := h.store.SaaSAdminTaskSummary(r.Context(), options.SaaSAdminTaskOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	fetchOptions := options.SaaSAdminTaskOptions
	fetchOptions.Limit = saasAdminExportMaxLimit
	tasks, err := h.store.SaaSAdminTasks(r.Context(), fetchOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	report := saasAdminBuildTaskSLAReport(options, summary, tasks, time.Now(), 3)
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTaskSLAReportPayload(report))
}

func (h *SaaSAdminHandler) TaskSLANotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.taskSLAOptions(w, r, positiveQueryInt(r, "limit", 50), saasAdminListMaxLimit)
	if !ok {
		return
	}
	notify, err := parseSaaSAdminTaskSLANotifications(r, options)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	notify.Options = options
	notify.ActorUserID = user.ID
	notify.ActorTenantID = user.TenantID
	summary, err := h.store.SaaSAdminTaskSummary(r.Context(), options.SaaSAdminTaskOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	fetchOptions := options.SaaSAdminTaskOptions
	fetchOptions.Limit = saasAdminExportMaxLimit
	tasks, err := h.store.SaaSAdminTasks(r.Context(), fetchOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	report := saasAdminBuildTaskSLAReport(options, summary, tasks, time.Now(), 3)
	existingKeys := map[string]struct{}{}
	if !notify.ForceCreate {
		existing, err := h.store.SaaSAdminAlertNotifications(r.Context(), SaaSAdminAlertNotificationOptions{
			Channel: notify.Channel,
			Keyword: SaaSAlertTypeAdminTaskSLA,
			Limit:   saasAdminExportMaxLimit,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		for _, notification := range existing {
			existingKeys[notification.NotificationKey] = struct{}{}
		}
	}
	result := SaaSAdminTaskSLANotificationsResult{
		Options:       options,
		Summary:       report.Summary,
		MatchedCount:  len(report.Tasks),
		Channel:       notify.Channel,
		MaxAttempts:   notify.MaxAttempts,
		SLAStatus:     notify.SLAStatus,
		Remark:        notify.Remark,
		ForceCreate:   notify.ForceCreate,
		Notifications: make([]SaaSAlertNotification, 0, len(report.Tasks)),
		Skipped:       make([]SaaSAdminTaskSLANotificationSkipped, 0),
	}
	for _, item := range report.Tasks {
		if !saasAdminTaskSLANotificationEligible(item, notify.SLAStatus) {
			result.SkippedStatusCount++
			result.Skipped = append(result.Skipped, SaaSAdminTaskSLANotificationSkipped{
				TaskID:   item.Task.ID,
				TenantID: item.Task.TenantID,
				Reason:   "task sla status below notification threshold",
			})
			continue
		}
		result.EligibleCount++
		alert, notificationKey, err := saasAdminTaskSLANotificationAlert(item, notify, h.platformAdminTenantID)
		if err != nil {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminTaskSLANotificationSkipped{
				TaskID:   item.Task.ID,
				TenantID: item.Task.TenantID,
				Reason:   err.Error(),
			})
			continue
		}
		if _, exists := existingKeys[notificationKey]; exists && !notify.ForceCreate {
			result.SkippedExistingCount++
			result.Skipped = append(result.Skipped, SaaSAdminTaskSLANotificationSkipped{
				TaskID:   item.Task.ID,
				TenantID: item.Task.TenantID,
				Reason:   "task sla notification already exists",
			})
			continue
		}
		notification, err := h.store.EnqueueSaaSAlertNotification(r.Context(), alert, notify.Channel, notify.MaxAttempts)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if notification.ID <= 0 {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminTaskSLANotificationSkipped{
				TaskID:   item.Task.ID,
				TenantID: item.Task.TenantID,
				Reason:   "notification outbox unavailable",
			})
			continue
		}
		if _, err := h.store.RecordSaaSAdminOperationLog(r.Context(), SaaSAdminOperationLog{
			TenantID:      notification.TenantID,
			ActorUserID:   notify.ActorUserID,
			ActorTenantID: notify.ActorTenantID,
			Action:        SaaSAdminOperationActionTaskSLANotify,
			TargetType:    SaaSAdminOperationTargetAlertNotification,
			TargetID:      strconv.FormatInt(notification.ID, 10),
			TargetName:    notification.NotificationKey,
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"notification": saasAdminAlertNotificationPayload(notification),
				"source":       "admin_task_sla",
				"filters":      saasAdminTaskSLAFiltersPayload(options),
				"task":         saasAdminTaskSLAItemPayload(item),
				"slaStatus":    notify.SLAStatus,
				"forceCreate":  notify.ForceCreate,
			}),
			Remark: notify.Remark,
		}); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		existingKeys[notificationKey] = struct{}{}
		result.EnqueuedCount++
		result.Notifications = append(result.Notifications, notification)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTaskSLANotificationsPayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) TaskCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	cancel, err := parseSaaSAdminTaskCancel(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	tasks, err := h.store.SaaSAdminTasks(r.Context(), SaaSAdminTaskOptions{TaskID: cancel.TaskID, Limit: 1})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if len(tasks) == 0 {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "task not found", nil)
		return
	}
	task := tasks[0]
	if task.Status == SaaSAdminTaskStatusApplied {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task already applied", nil)
		return
	}
	beforeTask := task
	result := map[string]any{
		"canceled":      true,
		"canceledAt":    time.Now().Format("2006-01-02 15:04:05"),
		"actorUserId":   user.ID,
		"actorTenantId": user.TenantID,
		"remark":        cancel.Remark,
	}
	if task.Status != SaaSAdminTaskStatusCanceled {
		task, err = h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
			TaskID:     task.ID,
			Status:     SaaSAdminTaskStatusCanceled,
			ResultJSON: saasAdminPayloadJSON(result),
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
	}
	if beforeTask.Status != SaaSAdminTaskStatusCanceled {
		if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskCancel, beforeTask, task, user, cancel.Remark, nil); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"task":   saasAdminTaskPayload(task),
		"result": result,
	})
}

func (h *SaaSAdminHandler) TaskReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	reset, err := parseSaaSAdminTaskCancel(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	tasks, err := h.store.SaaSAdminTasks(r.Context(), SaaSAdminTaskOptions{TaskID: reset.TaskID, Limit: 1})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if len(tasks) == 0 {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "task not found", nil)
		return
	}
	task := tasks[0]
	switch task.Status {
	case SaaSAdminTaskStatusFailed, SaaSAdminTaskStatusBlocked:
	case SaaSAdminTaskStatusPending:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task already pending", nil)
		return
	case SaaSAdminTaskStatusApplied:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task already applied", nil)
		return
	case SaaSAdminTaskStatusCanceled:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task canceled", nil)
		return
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task status unsupported", nil)
		return
	}
	beforeTask := task
	resetAt := time.Now().Format("2006-01-02 15:04:05")
	task, err = h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
		TaskID: task.ID,
		Status: SaaSAdminTaskStatusPending,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := map[string]any{
		"reset":          true,
		"resetAt":        resetAt,
		"previousStatus": beforeTask.Status,
		"actorUserId":    user.ID,
		"actorTenantId":  user.TenantID,
		"remark":         reset.Remark,
	}
	if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskReset, beforeTask, task, user, reset.Remark, map[string]any{
		"reset":          true,
		"resetAt":        resetAt,
		"previousStatus": beforeTask.Status,
	}); err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"task":   saasAdminTaskPayload(task),
		"result": result,
	})
}

func (h *SaaSAdminHandler) TaskBulkCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	cancel, err := parseSaaSAdminTaskBulkCancel(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	cancel.ActorUserID = user.ID
	cancel.ActorTenantID = user.TenantID
	tasks, err := h.store.SaaSAdminTasks(r.Context(), cancel.Options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := SaaSAdminTaskBulkCancelResult{
		MatchedCount: len(tasks),
		Options:      cancel.Options,
		Remark:       cancel.Remark,
		Tasks:        make([]SaaSAdminTask, 0, len(tasks)),
	}
	canceledAt := time.Now().Format("2006-01-02 15:04:05")
	for _, task := range tasks {
		switch task.Status {
		case SaaSAdminTaskStatusApplied:
			result.SkippedAppliedCount++
			continue
		case SaaSAdminTaskStatusCanceled:
			result.SkippedCanceledCount++
			continue
		case SaaSAdminTaskStatusPending, SaaSAdminTaskStatusBlocked, SaaSAdminTaskStatusFailed:
			payload := map[string]any{
				"canceled":      true,
				"bulkCancel":    true,
				"canceledAt":    canceledAt,
				"actorUserId":   cancel.ActorUserID,
				"actorTenantId": cancel.ActorTenantID,
				"remark":        cancel.Remark,
				"filters":       saasAdminTaskFiltersPayload(cancel.Options),
			}
			updated, err := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:     task.ID,
				Status:     SaaSAdminTaskStatusCanceled,
				ResultJSON: saasAdminPayloadJSON(payload),
			})
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskBulkCancel, task, updated, user, cancel.Remark, map[string]any{
				"bulkCancel": true,
				"filters":    saasAdminTaskFiltersPayload(cancel.Options),
			}); err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			result.CanceledCount++
			result.Tasks = append(result.Tasks, updated)
		default:
			result.SkippedIneligibleCount++
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTaskBulkCancelPayload(result))
}

func (h *SaaSAdminHandler) TaskBulkReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	reset, err := parseSaaSAdminTaskBulkReset(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	reset.ActorUserID = user.ID
	reset.ActorTenantID = user.TenantID
	tasks, err := h.store.SaaSAdminTasks(r.Context(), reset.Options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := SaaSAdminTaskBulkResetResult{
		MatchedCount: len(tasks),
		Options:      reset.Options,
		Remark:       reset.Remark,
		Tasks:        make([]SaaSAdminTask, 0, len(tasks)),
	}
	resetAt := time.Now().Format("2006-01-02 15:04:05")
	for _, task := range tasks {
		switch task.Status {
		case SaaSAdminTaskStatusPending:
			result.SkippedPendingCount++
			continue
		case SaaSAdminTaskStatusApplied:
			result.SkippedAppliedCount++
			continue
		case SaaSAdminTaskStatusCanceled:
			result.SkippedCanceledCount++
			continue
		case SaaSAdminTaskStatusBlocked, SaaSAdminTaskStatusFailed:
			updated, err := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID: task.ID,
				Status: SaaSAdminTaskStatusPending,
			})
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskBulkReset, task, updated, user, reset.Remark, map[string]any{
				"reset":          true,
				"bulkReset":      true,
				"resetAt":        resetAt,
				"previousStatus": task.Status,
				"filters":        saasAdminTaskFiltersPayload(reset.Options),
			}); err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			result.ResetCount++
			result.Tasks = append(result.Tasks, updated)
		default:
			result.SkippedIneligibleCount++
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTaskBulkResetPayload(result))
}

func (h *SaaSAdminHandler) TenantRenewalTaskBulkApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantRenewal, 0) {
		return
	}
	apply, err := parseSaaSAdminTenantRenewalTaskBulkApply(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	apply.ActorUserID = user.ID
	apply.ActorTenantID = user.TenantID
	tasks, err := h.store.SaaSAdminTasks(r.Context(), apply.Options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := SaaSAdminTaskBulkApplyResult{
		MatchedCount: len(tasks),
		Options:      apply.Options,
		Remark:       apply.Remark,
		Tasks:        make([]SaaSAdminTask, 0, len(tasks)),
		Errors:       make([]SaaSAdminTaskBulkApplyError, 0),
	}
	for _, task := range tasks {
		if task.TaskType != SaaSAdminTaskTypeTenantRenewal {
			result.SkippedUnsupportedCount++
			continue
		}
		switch task.Status {
		case SaaSAdminTaskStatusApplied:
			result.SkippedAppliedCount++
			continue
		case SaaSAdminTaskStatusCanceled:
			result.SkippedCanceledCount++
			continue
		}
		beforeTask := task
		renewal, err := saasAdminTenantRenewalFromTaskRequest(task.RequestJSON)
		if err != nil {
			updated, updateErr := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:    task.ID,
				Status:    SaaSAdminTaskStatusFailed,
				LastError: err.Error(),
			})
			if updateErr != nil {
				writeSaaSAdminError(w, updateErr)
				return
			}
			result.FailedCount++
			result.Tasks = append(result.Tasks, updated)
			result.Errors = append(result.Errors, SaaSAdminTaskBulkApplyError{TaskID: task.ID, TenantID: task.TenantID, Error: err.Error()})
			continue
		}
		renewal.ActorUserID = user.ID
		renewal.ActorTenantID = user.TenantID
		preview, resolvedRenewal, err := h.tenantRenewalPreview(r.Context(), renewal)
		if err != nil {
			updated, updateErr := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:    task.ID,
				Status:    SaaSAdminTaskStatusFailed,
				LastError: err.Error(),
			})
			if updateErr != nil {
				writeSaaSAdminError(w, updateErr)
				return
			}
			result.FailedCount++
			result.Tasks = append(result.Tasks, updated)
			result.Errors = append(result.Errors, SaaSAdminTaskBulkApplyError{TaskID: task.ID, TenantID: task.TenantID, Error: err.Error()})
			continue
		}
		if preview.Blocked {
			updated, err := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:     task.ID,
				Status:     SaaSAdminTaskStatusBlocked,
				ResultJSON: saasAdminPayloadJSON(saasAdminTenantRenewalPreviewPayload(preview)),
				LastError:  preview.BlockReason,
			})
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskBlock, beforeTask, updated, user, preview.BlockReason, map[string]any{
				"bulkApply": true,
				"filters":   saasAdminTaskFiltersPayload(apply.Options),
			}); err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			result.BlockedCount++
			result.Tasks = append(result.Tasks, updated)
			continue
		}
		taskDigest := sha256.Sum256([]byte(task.RequestJSON))
		resolvedRenewal.TaskID = task.ID
		resolvedRenewal.ExpectedTaskVersion = task.Version
		resolvedRenewal.ExpectedTaskRequestSHA256 = fmt.Sprintf("%x", taskDigest)
		renewalResult, err := h.store.RenewSaaSAdminTenant(r.Context(), resolvedRenewal)
		if err != nil {
			updated, updateErr := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:    task.ID,
				Status:    SaaSAdminTaskStatusFailed,
				LastError: err.Error(),
			})
			if updateErr != nil {
				writeSaaSAdminError(w, updateErr)
				return
			}
			result.FailedCount++
			result.Tasks = append(result.Tasks, updated)
			result.Errors = append(result.Errors, SaaSAdminTaskBulkApplyError{TaskID: task.ID, TenantID: task.TenantID, Error: err.Error()})
			continue
		}
		updated := task
		updated.Status = SaaSAdminTaskStatusApplied
		updated.Version++
		updated.TenantID = renewalResult.TenantID
		updated.PackageCode = renewalResult.PackageCode
		updated.ResultJSON = saasAdminPayloadJSON(saasAdminTenantRenewalPayload(renewalResult))
		updated.LastError = ""
		updated.AppliedAt = time.Now().Format("2006-01-02 15:04:05")
		result.AppliedCount++
		result.Tasks = append(result.Tasks, updated)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTaskBulkApplyPayload(result))
}

func (h *SaaSAdminHandler) DailyReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.dailyReportOptions(w, r, time.Now())
	if !ok {
		return
	}
	report, err := h.buildDailyReport(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminDailyReportPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) OperationQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.operationQueueOptions(w, r, 50, saasAdminListMaxLimit)
	if !ok {
		return
	}
	report, err := h.buildOperationQueue(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminOperationQueuePayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) OperationQueueOwners(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.operationQueueOptions(w, r, 50, saasAdminListMaxLimit)
	if !ok {
		return
	}
	report, err := h.buildOperationQueueOwnerReport(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminOperationQueueOwnerReportPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) OperationQueueAssignments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.operationQueueAssignmentOptions(w, r, 50, saasAdminListMaxLimit)
	if !ok {
		return
	}
	report, err := h.buildOperationQueueAssignmentReport(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminOperationQueueAssignmentReportPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) OperationQueueAssignmentClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	closeAssignment, err := parseSaaSAdminOperationQueueAssignmentClose(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	closeAssignment.ActorUserID = user.ID
	closeAssignment.ActorTenantID = user.TenantID

	assignments, err := h.latestOperationQueueAssignments(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	var previous SaaSAdminOperationQueueAssignment
	for _, assignment := range assignments {
		if assignment.OperationID == closeAssignment.OperationID {
			previous = assignment
			break
		}
	}
	if previous.OperationID <= 0 {
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, "operation queue assignment is no longer current", nil)
		return
	}
	result, err := h.closeOperationQueueAssignment(r.Context(), previous, closeAssignment)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminOperationQueueAssignmentClosePayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) closeOperationQueueAssignment(ctx context.Context, previous SaaSAdminOperationQueueAssignment, closeAssignment SaaSAdminOperationQueueAssignmentClose) (SaaSAdminOperationQueueAssignmentCloseResult, error) {
	previous.DueState = saasAdminOperationQueueAssignmentDueState(previous, time.Now())
	if previous.DueState == SaaSAdminRiskFollowUpDueStateClosed {
		return SaaSAdminOperationQueueAssignmentCloseResult{
			AlreadyClosed: true,
			Previous:      previous,
			Assignment:    previous,
		}, nil
	}

	closed := previous
	closed.Status = closeAssignment.Status
	closed.DueState = SaaSAdminRiskFollowUpDueStateClosed
	closed.NextFollowUpAt = ""
	closed.Remark = closeAssignment.Remark
	closed.ActorUserID = closeAssignment.ActorUserID
	closed.ActorTenantID = closeAssignment.ActorTenantID
	closed.AssignedAt = time.Now().Format("2006-01-02 15:04:05")
	closed.OperationID = 0
	after := saasAdminOperationQueueAssignmentPayload(closed)
	after["previousOperationId"] = previous.OperationID
	if len(closeAssignment.Context) > 0 {
		after["closeContext"] = closeAssignment.Context
	}
	operationID, err := h.store.RecordSaaSAdminOperationLog(ctx, SaaSAdminOperationLog{
		TenantID:      closed.TenantID,
		ActorUserID:   closeAssignment.ActorUserID,
		ActorTenantID: closeAssignment.ActorTenantID,
		Action:        SaaSAdminOperationActionOperationQueueAssignmentClose,
		TargetType:    closed.ObjectType,
		TargetID:      closed.ObjectID,
		TargetName:    closed.TargetName,
		BeforeJSON:    saasAdminPayloadJSON(saasAdminOperationQueueAssignmentPayload(previous)),
		AfterJSON:     saasAdminPayloadJSON(after),
		Remark:        closeAssignment.Remark,
	})
	if err != nil {
		return SaaSAdminOperationQueueAssignmentCloseResult{}, err
	}
	closed.OperationID = operationID
	return SaaSAdminOperationQueueAssignmentCloseResult{
		Closed:     true,
		Previous:   previous,
		Assignment: closed,
	}, nil
}

func (h *SaaSAdminHandler) OperationQueueAssignmentNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.operationQueueAssignmentOptions(w, r, 50, saasAdminListMaxLimit)
	if !ok {
		return
	}
	if options.DueState == "" || options.DueState == SaaSAdminRiskFollowUpDueStateAll {
		options.DueState = SaaSAdminRiskFollowUpDueStateOverdue
	}
	if options.DueState != SaaSAdminRiskFollowUpDueStateOverdue && options.DueState != SaaSAdminRiskFollowUpDueStateDueSoon {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "dueState must be overdue or due_soon", nil)
		return
	}
	options.CurrentOnly = true
	notify, err := parseSaaSAdminOperationQueueAssignmentNotifications(r, options)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	notify.Options = options
	notify.ActorUserID = user.ID
	notify.ActorTenantID = user.TenantID
	result, err := h.createOperationQueueAssignmentNotifications(r.Context(), notify)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminOperationQueueAssignmentNotificationsPayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) createOperationQueueAssignmentNotifications(ctx context.Context, notify SaaSAdminOperationQueueAssignmentNotifications) (SaaSAdminOperationQueueAssignmentNotificationsResult, error) {
	options := notify.Options
	options.CurrentOnly = true
	notify.Options = options
	report, err := h.buildOperationQueueAssignmentReport(ctx, options)
	if err != nil {
		return SaaSAdminOperationQueueAssignmentNotificationsResult{}, err
	}
	existingKeys := map[string]struct{}{}
	if !notify.ForceCreate {
		existing, err := h.store.SaaSAdminAlertNotifications(ctx, SaaSAdminAlertNotificationOptions{
			Channel: notify.Channel,
			Keyword: SaaSAlertTypeOperationQueueAssign,
			Limit:   saasAdminExportMaxLimit,
		})
		if err != nil {
			return SaaSAdminOperationQueueAssignmentNotificationsResult{}, err
		}
		for _, notification := range existing {
			existingKeys[notification.NotificationKey] = struct{}{}
		}
	}
	result := SaaSAdminOperationQueueAssignmentNotificationsResult{
		Options:       options,
		Summary:       report.Summary,
		MatchedCount:  len(report.Assignments),
		Channel:       notify.Channel,
		MaxAttempts:   notify.MaxAttempts,
		Remark:        notify.Remark,
		ForceCreate:   notify.ForceCreate,
		Notifications: make([]SaaSAlertNotification, 0, len(report.Assignments)),
		Skipped:       make([]SaaSAdminOperationQueueAssignmentNotificationSkipped, 0),
	}
	for _, assignment := range report.Assignments {
		if !saasAdminOperationQueueAssignmentNotificationEligible(assignment) {
			result.SkippedStatusCount++
			result.Skipped = append(result.Skipped, SaaSAdminOperationQueueAssignmentNotificationSkipped{
				OperationID: assignment.OperationID,
				TenantID:    assignment.TenantID,
				Source:      assignment.Source,
				ObjectID:    assignment.ObjectID,
				DueState:    assignment.DueState,
				Reason:      "operation queue assignment is not due",
			})
			continue
		}
		result.EligibleCount++
		alert, notificationKey, err := saasAdminOperationQueueAssignmentNotificationAlert(assignment, notify, h.platformAdminTenantID)
		if err != nil {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminOperationQueueAssignmentNotificationSkipped{
				OperationID: assignment.OperationID,
				TenantID:    assignment.TenantID,
				Source:      assignment.Source,
				ObjectID:    assignment.ObjectID,
				DueState:    assignment.DueState,
				Reason:      err.Error(),
			})
			continue
		}
		_, exists := existingKeys[notificationKey]
		if !exists && !notify.ForceCreate {
			if reader, ok := h.store.(SaaSAlertNotificationKeyReader); ok && reader != nil {
				existing, err := reader.SaaSAlertNotificationByKey(ctx, notificationKey)
				if err != nil {
					return SaaSAdminOperationQueueAssignmentNotificationsResult{}, err
				}
				exists = existing.ID > 0
			}
		}
		if exists && !notify.ForceCreate {
			result.SkippedExistingCount++
			result.Skipped = append(result.Skipped, SaaSAdminOperationQueueAssignmentNotificationSkipped{
				OperationID: assignment.OperationID,
				TenantID:    assignment.TenantID,
				Source:      assignment.Source,
				ObjectID:    assignment.ObjectID,
				DueState:    assignment.DueState,
				Reason:      "operation queue assignment notification already exists",
			})
			continue
		}
		notification, err := h.store.EnqueueSaaSAlertNotification(ctx, alert, notify.Channel, notify.MaxAttempts)
		if err != nil {
			return SaaSAdminOperationQueueAssignmentNotificationsResult{}, err
		}
		if notification.ID <= 0 {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminOperationQueueAssignmentNotificationSkipped{
				OperationID: assignment.OperationID,
				TenantID:    assignment.TenantID,
				Source:      assignment.Source,
				ObjectID:    assignment.ObjectID,
				DueState:    assignment.DueState,
				Reason:      "notification outbox unavailable",
			})
			continue
		}
		if _, err := h.store.RecordSaaSAdminOperationLog(ctx, SaaSAdminOperationLog{
			TenantID:      notification.TenantID,
			ActorUserID:   notify.ActorUserID,
			ActorTenantID: notify.ActorTenantID,
			Action:        SaaSAdminOperationActionOperationQueueAssignmentNotify,
			TargetType:    SaaSAdminOperationTargetAlertNotification,
			TargetID:      strconv.FormatInt(notification.ID, 10),
			TargetName:    notification.NotificationKey,
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"notification": saasAdminAlertNotificationPayload(notification),
				"source":       "operation_queue_assignment",
				"filters":      saasAdminOperationQueueAssignmentFiltersPayload(options),
				"assignment":   saasAdminOperationQueueAssignmentPayload(assignment),
				"forceCreate":  notify.ForceCreate,
			}),
			Remark: notify.Remark,
		}); err != nil {
			return SaaSAdminOperationQueueAssignmentNotificationsResult{}, err
		}
		existingKeys[notificationKey] = struct{}{}
		result.EnqueuedCount++
		result.Notifications = append(result.Notifications, notification)
	}
	return result, nil
}

func (h *SaaSAdminHandler) OperationQueueAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.operationQueueOptions(w, r, 50, saasAdminListMaxLimit)
	if !ok {
		return
	}
	assign, err := parseSaaSAdminOperationQueueAssign(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	assign.Options = options
	assign.ActorUserID = user.ID
	assign.ActorTenantID = user.TenantID

	report, err := h.buildOperationQueue(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := SaaSAdminOperationQueueAssignResult{
		Options:        options,
		MatchedCount:   len(report.Items),
		Status:         assign.Status,
		Owner:          assign.Owner,
		NextFollowUpAt: assign.NextFollowUpAt,
		Remark:         assign.Remark,
		Items:          make([]SaaSAdminOperationQueueAssignItem, 0, len(report.Items)),
	}
	for _, queueItem := range report.Items {
		assignItem := SaaSAdminOperationQueueAssignItem{QueueItem: queueItem}
		switch queueItem.Source {
		case SaaSAdminOperationQueueSourceCustomerSuccess:
			if queueItem.TenantID <= 0 || queueItem.TenantID == h.platformAdminTenantID {
				assignItem.SkippedReason = "invalid_tenant"
				result.SkippedCount++
				result.Items = append(result.Items, assignItem)
				continue
			}
			result.AssignableCount++
			followUp := SaaSAdminRiskFollowUp{
				TenantID:       queueItem.TenantID,
				Status:         assign.Status,
				Owner:          assign.Owner,
				NextFollowUpAt: assign.NextFollowUpAt,
				Remark:         assign.Remark,
				ActorUserID:    assign.ActorUserID,
				ActorTenantID:  assign.ActorTenantID,
			}
			assigned, err := h.store.RecordSaaSAdminRiskFollowUp(r.Context(), followUp)
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			assignItem.RiskFollowUp = &assigned
			result.AssignedCount++
			result.CustomerSuccessAssignedCount++
		case SaaSAdminOperationQueueSourceBillingFollowUp:
			billingEventID, err := strconv.ParseInt(strings.TrimSpace(queueItem.ObjectID), 10, 64)
			if err != nil || billingEventID <= 0 {
				assignItem.SkippedReason = "invalid_billing_event"
				result.SkippedCount++
				result.Items = append(result.Items, assignItem)
				continue
			}
			event, found, err := h.store.SaaSAdminBillingEventByID(r.Context(), billingEventID)
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			if !found {
				assignItem.SkippedReason = "billing_event_not_found"
				result.SkippedCount++
				result.Items = append(result.Items, assignItem)
				continue
			}
			result.AssignableCount++
			followUp := SaaSAdminBillingReconciliationFollowUp{
				BillingEventID: billingEventID,
				Status:         assign.Status,
				Owner:          assign.Owner,
				NextFollowUpAt: assign.NextFollowUpAt,
				Remark:         assign.Remark,
				ActorUserID:    assign.ActorUserID,
				ActorTenantID:  assign.ActorTenantID,
			}
			assigned, err := h.recordBillingReconciliationFollowUpOperation(r.Context(), event, followUp)
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			assignItem.BillingFollowUp = &assigned
			result.AssignedCount++
			result.BillingFollowUpAssignedCount++
		case SaaSAdminOperationQueueSourceTaskSLA, SaaSAdminOperationQueueSourceNotification, SaaSAdminOperationQueueSourceClosedNotification, SaaSAdminOperationQueueSourceNotificationHealth:
			if strings.TrimSpace(queueItem.ObjectID) == "" || strings.TrimSpace(queueItem.ObjectType) == "" {
				assignItem.SkippedReason = "invalid_object"
				result.SkippedCount++
				result.Items = append(result.Items, assignItem)
				continue
			}
			result.AssignableCount++
			assigned, err := h.recordOperationQueueAssignment(r.Context(), queueItem, assign)
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			assignItem.OperationQueueAssignment = &assigned
			result.AssignedCount++
			result.QueueAssignmentAssignedCount++
			switch queueItem.Source {
			case SaaSAdminOperationQueueSourceTaskSLA:
				result.TaskSLAAssignedCount++
			case SaaSAdminOperationQueueSourceNotification:
				result.NotificationAssignedCount++
			case SaaSAdminOperationQueueSourceClosedNotification:
				result.ClosedNotificationAssignedCount++
			case SaaSAdminOperationQueueSourceNotificationHealth:
				result.NotificationHealthAssignedCount++
			}
		default:
			assignItem.SkippedReason = "unsupported_source"
			result.SkippedCount++
			result.UnsupportedCount++
		}
		result.Items = append(result.Items, assignItem)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminOperationQueueAssignPayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) recordOperationQueueAssignment(ctx context.Context, queueItem SaaSAdminOperationQueueItem, assign SaaSAdminOperationQueueAssign) (SaaSAdminOperationQueueAssignment, error) {
	assignment := SaaSAdminOperationQueueAssignment{
		TenantID:       queueItem.TenantID,
		Source:         queueItem.Source,
		ObjectType:     queueItem.ObjectType,
		ObjectID:       queueItem.ObjectID,
		TargetName:     queueItem.Title,
		Owner:          assign.Owner,
		Status:         assign.Status,
		NextFollowUpAt: assign.NextFollowUpAt,
		Remark:         assign.Remark,
		ActorUserID:    assign.ActorUserID,
		ActorTenantID:  assign.ActorTenantID,
		AssignedAt:     time.Now().Format("2006-01-02 15:04:05"),
	}
	after := saasAdminOperationQueueAssignmentPayload(assignment)
	after["queueItem"] = saasAdminOperationQueueItemPayload(queueItem)
	operationID, err := h.store.RecordSaaSAdminOperationLog(ctx, SaaSAdminOperationLog{
		TenantID:      queueItem.TenantID,
		ActorUserID:   assign.ActorUserID,
		ActorTenantID: assign.ActorTenantID,
		Action:        SaaSAdminOperationActionOperationQueueAssign,
		TargetType:    queueItem.ObjectType,
		TargetID:      queueItem.ObjectID,
		TargetName:    queueItem.Title,
		BeforeJSON:    saasAdminPayloadJSON(map[string]any{"queueItem": saasAdminOperationQueueItemPayload(queueItem)}),
		AfterJSON:     saasAdminPayloadJSON(after),
		Remark:        assign.Remark,
	})
	if err != nil {
		return SaaSAdminOperationQueueAssignment{}, err
	}
	assignment.OperationID = operationID
	return assignment, nil
}

func (h *SaaSAdminHandler) buildOperationQueueAssignmentReport(ctx context.Context, options SaaSAdminOperationQueueAssignmentOptions) (SaaSAdminOperationQueueAssignmentReport, error) {
	logs, err := h.operationQueueAssignmentLogs(ctx, options.TenantID)
	if err != nil {
		return SaaSAdminOperationQueueAssignmentReport{}, err
	}
	return saasAdminBuildOperationQueueAssignmentReport(options, logs), nil
}

func (h *SaaSAdminHandler) operationQueueAssignmentLogs(ctx context.Context, tenantID int) ([]SaaSAdminOperationLog, error) {
	logs := make([]SaaSAdminOperationLog, 0)
	for _, action := range []string{
		SaaSAdminOperationActionOperationQueueAssignmentClose,
		SaaSAdminOperationActionOperationQueueAssign,
	} {
		items, err := h.store.SaaSAdminOperationLogs(ctx, SaaSAdminOperationLogOptions{
			TenantID: tenantID,
			Action:   action,
			Limit:    saasAdminExportMaxLimit,
		})
		if err != nil {
			return nil, err
		}
		logs = append(logs, items...)
	}
	return logs, nil
}

func (h *SaaSAdminHandler) buildOperationQueue(ctx context.Context, options SaaSAdminOperationQueueOptions) (SaaSAdminOperationQueueReport, error) {
	customerOptions := SaaSAdminCustomerSuccessOptions{
		TenantLimit:    options.TenantLimit,
		Limit:          saasAdminExportMaxLimit,
		ExpiringDays:   options.ExpiringDays,
		HighUsageRatio: options.HighUsageRatio,
		Owner:          options.Owner,
		Priority:       options.Priority,
	}
	customerReport, err := h.buildCustomerSuccessReport(ctx, customerOptions)
	if err != nil {
		return SaaSAdminOperationQueueReport{}, err
	}

	taskOptions := SaaSAdminTaskOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            saasAdminExportMaxLimit,
	}
	taskSummary, err := h.store.SaaSAdminTaskSummary(ctx, taskOptions)
	if err != nil {
		return SaaSAdminOperationQueueReport{}, err
	}
	tasks, err := h.store.SaaSAdminTasks(ctx, taskOptions)
	if err != nil {
		return SaaSAdminOperationQueueReport{}, err
	}
	taskSLAReport := saasAdminBuildTaskSLAReport(SaaSAdminTaskSLAOptions{
		SaaSAdminTaskOptions: taskOptions,
		WarningHours:         options.WarningHours,
		OverdueHours:         options.OverdueHours,
	}, taskSummary, tasks, time.Now(), 3)

	billingSnapshots, err := h.store.SaaSAdminBillingReconciliationFollowUpSnapshots(ctx, SaaSAdminBillingReconciliationFollowUpOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		DueState:         SaaSAdminRiskFollowUpDueStateAll,
		Limit:            saasAdminExportMaxLimit,
	})
	if err != nil {
		return SaaSAdminOperationQueueReport{}, err
	}
	billingTasks, _ := saasAdminBuildBillingReconciliationFollowUpTasks(billingSnapshots, SaaSAdminBillingReconciliationFollowUpOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		DueState:         SaaSAdminRiskFollowUpDueStateAll,
		Limit:            saasAdminExportMaxLimit,
	}, time.Now())

	notifications, err := h.store.SaaSAdminAlertNotifications(ctx, SaaSAdminAlertNotificationOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            saasAdminExportMaxLimit,
	})
	if err != nil {
		return SaaSAdminOperationQueueReport{}, err
	}
	notificationHealth := []SaaSAdminNotificationHealthTenant{}
	if options.Source == "" || options.Source == SaaSAdminOperationQueueSourceNotificationHealth {
		healthOptions := SaaSAdminNotificationHealthOptions{
			ExcludedTenantID: h.platformAdminTenantID,
			Channel:          SaaSAlertNotificationChannelWebhook,
			State:            SaaSAdminNotificationHealthStateAll,
			WindowHours:      options.HealthWindowHours,
			StaleMinutes:     options.HealthStaleMinutes,
			Limit:            saasAdminExportMaxLimit,
		}
		healthSource, err := h.store.SaaSAdminNotificationHealth(ctx, healthOptions)
		if err != nil {
			return SaaSAdminOperationQueueReport{}, err
		}
		healthReport := saasAdminBuildNotificationHealthReport(healthSource, healthOptions, time.Now())
		notificationHealth = healthReport.Tenants
	}
	assignments, err := h.latestOperationQueueAssignments(ctx)
	if err != nil {
		return SaaSAdminOperationQueueReport{}, err
	}

	items, summary := saasAdminBuildOperationQueue(saasAdminOperationQueueBuildInput{
		CustomerSuccess:    customerReport.Items,
		TaskSLA:            taskSLAReport.Tasks,
		BillingTasks:       billingTasks,
		Notifications:      notifications,
		NotificationHealth: notificationHealth,
		Assignments:        assignments,
		Options:            options,
	})
	return SaaSAdminOperationQueueReport{
		Options: options,
		Summary: summary,
		Items:   items,
	}, nil
}

func (h *SaaSAdminHandler) latestOperationQueueAssignments(ctx context.Context) (map[string]SaaSAdminOperationQueueAssignment, error) {
	logs, err := h.operationQueueAssignmentLogs(ctx, 0)
	if err != nil {
		return nil, err
	}
	assignments := map[string]SaaSAdminOperationQueueAssignment{}
	for _, log := range logs {
		assignment, ok := saasAdminOperationQueueAssignmentFromLog(log)
		if !ok {
			continue
		}
		key := saasAdminOperationQueueAssignmentKey(assignment.Source, assignment.ObjectID)
		if key == "" {
			continue
		}
		existing, exists := assignments[key]
		if !exists || saasAdminOperationQueueAssignmentNewer(assignment, existing) {
			assignments[key] = assignment
		}
	}
	return assignments, nil
}

func (h *SaaSAdminHandler) buildOperationQueueOwnerReport(ctx context.Context, options SaaSAdminOperationQueueOptions) (SaaSAdminOperationQueueOwnerReport, error) {
	queueOptions := options
	queueOptions.Limit = saasAdminExportMaxLimit
	queueReport, err := h.buildOperationQueue(ctx, queueOptions)
	if err != nil {
		return SaaSAdminOperationQueueOwnerReport{}, err
	}
	owners := saasAdminBuildOperationQueueOwnerSummaries(queueReport.Items, 3, 3)
	totalOwnerCount := len(owners)
	if options.Limit > 0 && len(owners) > options.Limit {
		owners = owners[:options.Limit]
	}
	return SaaSAdminOperationQueueOwnerReport{
		Options:            options,
		Summary:            queueReport.Summary,
		TotalOwnerCount:    totalOwnerCount,
		ReturnedOwnerCount: len(owners),
		ScannedQueueCount:  len(queueReport.Items),
		Owners:             owners,
	}, nil
}

func (h *SaaSAdminHandler) buildDailyReport(ctx context.Context, options SaaSAdminDailyReportOptions) (SaaSAdminDailyReportData, error) {
	overviewOptions := SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopePlatform,
		Limit:        options.TenantLimit,
		ExpiringDays: options.ExpiringDays,
		DueState:     SaaSAdminDueStateAll,
	}
	overview, err := h.saasAdminOverview(ctx, overviewOptions)
	if err != nil {
		return SaaSAdminDailyReportData{}, err
	}
	usageByTenant := make(map[int][]SaaSAdminUsageMetric, len(overview.Tenants))
	for _, tenant := range overview.Tenants {
		if tenant.TenantID <= 0 {
			continue
		}
		metrics, err := h.store.SaaSAdminTenantUsage(ctx, tenant.TenantID)
		if err != nil {
			return SaaSAdminDailyReportData{}, err
		}
		usageByTenant[tenant.TenantID] = metrics
	}
	riskReport := saasAdminBuildRiskReport(overview, usageByTenant, options.HighUsageRatio)
	if err := h.attachRiskFollowUps(ctx, &riskReport); err != nil {
		return SaaSAdminDailyReportData{}, err
	}
	snapshots, err := h.store.SaaSAdminRiskFollowUpSnapshots(ctx, SaaSAdminRiskFollowUpTaskOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		DueState:         SaaSAdminRiskFollowUpDueStateAll,
		Limit:            saasAdminExportMaxLimit,
	})
	if err != nil {
		return SaaSAdminDailyReportData{}, err
	}
	riskTasks, riskTaskSummary := saasAdminBuildRiskFollowUpTasks(snapshots, SaaSAdminRiskFollowUpTaskOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		DueState:         SaaSAdminRiskFollowUpDueStateAll,
		Limit:            saasAdminExportMaxLimit,
	}, time.Now())
	riskOwners := saasAdminBuildRiskFollowUpOwnerSummaries(riskTasks)
	taskSLAOptions := SaaSAdminTaskSLAOptions{
		SaaSAdminTaskOptions: SaaSAdminTaskOptions{
			ExcludedTenantID: h.platformAdminTenantID,
			Limit:            options.ItemLimit,
		},
		WarningHours: saasAdminDefaultTaskSLAWarningHours,
		OverdueHours: saasAdminDefaultTaskSLAOverdueHours,
	}
	taskSummary, err := h.store.SaaSAdminTaskSummary(ctx, taskSLAOptions.SaaSAdminTaskOptions)
	if err != nil {
		return SaaSAdminDailyReportData{}, err
	}
	taskFetchOptions := taskSLAOptions.SaaSAdminTaskOptions
	taskFetchOptions.Limit = saasAdminExportMaxLimit
	tasks, err := h.store.SaaSAdminTasks(ctx, taskFetchOptions)
	if err != nil {
		return SaaSAdminDailyReportData{}, err
	}
	taskSLAReport := saasAdminBuildTaskSLAReport(taskSLAOptions, taskSummary, tasks, time.Now(), 3)
	alertPage, err := h.store.ListSaaSAlerts(ctx, SaaSAlertListOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		Status:           SaaSAlertStatusOpen,
		Page:             1,
		PerPage:          options.ItemLimit,
	})
	if err != nil {
		return SaaSAdminDailyReportData{}, err
	}
	notifications, err := h.store.SaaSAdminAlertNotifications(ctx, SaaSAdminAlertNotificationOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            saasAdminExportMaxLimit,
	})
	if err != nil {
		return SaaSAdminDailyReportData{}, err
	}
	operations, err := h.store.SaaSAdminOperationLogs(ctx, SaaSAdminOperationLogOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            saasAdminExportMaxLimit,
	})
	if err != nil {
		return SaaSAdminDailyReportData{}, err
	}
	billingEvents, err := h.store.SaaSAdminBillingEvents(ctx, SaaSAdminBillingEventOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            saasAdminExportMaxLimit,
	})
	if err != nil {
		return SaaSAdminDailyReportData{}, err
	}
	windowOperations := saasAdminFilterOperationLogsByWindow(operations, options.WindowStart, options.WindowEnd)
	windowBillingEvents := saasAdminFilterBillingEventsByWindow(billingEvents, options.WindowStart, options.WindowEnd)
	queueAssignmentReport := saasAdminBuildOperationQueueAssignmentReport(SaaSAdminOperationQueueAssignmentOptions{
		Limit: options.ItemLimit,
	}, saasAdminOperationQueueAssignmentLogs(windowOperations))
	notificationSummary := saasAdminDailyNotificationSummary(notifications)
	closedNotifications := saasAdminClosedNotificationsInWindow(notifications, options.WindowStart, options.WindowEnd)
	billingSummary := saasAdminDailyBillingSummary(windowBillingEvents)
	actionSummary := saasAdminDailyOperationActionSummary(windowOperations)
	summary := saasAdminDailyReportSummary(overview, riskReport, riskTaskSummary, riskOwners, taskSLAReport, alertPage.Total, notificationSummary, queueAssignmentReport.Summary, billingSummary, windowOperations)
	return SaaSAdminDailyReportData{
		Options:                options,
		Overview:               overview,
		RiskReport:             riskReport,
		RiskTaskSummary:        riskTaskSummary,
		RiskOwners:             riskOwners,
		TaskSLAReport:          taskSLAReport,
		AlertPage:              alertPage,
		NotificationSummary:    notificationSummary,
		RetryableNotifications: saasAdminRetryableNotifications(notifications),
		ClosedNotifications:    closedNotifications,
		QueueAssignmentReport:  queueAssignmentReport,
		OperationActionSummary: actionSummary,
		WindowOperations:       windowOperations,
		BillingSummary:         billingSummary,
		WindowBillingEvents:    windowBillingEvents,
		Summary:                summary,
	}, nil
}

func (h *SaaSAdminHandler) buildBusinessMetrics(ctx context.Context, options SaaSAdminBusinessMetricsOptions) (SaaSAdminBusinessMetricsReport, error) {
	overviewOptions := SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopePlatform,
		Limit:        options.TenantLimit,
		ExpiringDays: options.ExpiringDays,
		DueState:     SaaSAdminDueStateAll,
	}
	overview, err := h.saasAdminOverview(ctx, overviewOptions)
	if err != nil {
		return SaaSAdminBusinessMetricsReport{}, err
	}
	usageByTenant := make(map[int][]SaaSAdminUsageMetric, len(overview.Tenants))
	for _, tenant := range overview.Tenants {
		if tenant.TenantID <= 0 {
			continue
		}
		metrics, err := h.store.SaaSAdminTenantUsage(ctx, tenant.TenantID)
		if err != nil {
			return SaaSAdminBusinessMetricsReport{}, err
		}
		usageByTenant[tenant.TenantID] = metrics
	}
	riskReport := saasAdminBuildRiskReport(overview, usageByTenant, options.HighUsageRatio)
	packages, err := h.store.SaaSAdminPackages(ctx)
	if err != nil {
		return SaaSAdminBusinessMetricsReport{}, err
	}
	billingEvents, err := h.store.SaaSAdminBillingEvents(ctx, SaaSAdminBillingEventOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            options.BillingLimit,
	})
	if err != nil {
		return SaaSAdminBusinessMetricsReport{}, err
	}
	return saasAdminBuildBusinessMetricsReport(options, overview, riskReport, packages, billingEvents), nil
}

func (h *SaaSAdminHandler) buildBusinessTrends(ctx context.Context, options SaaSAdminBusinessTrendOptions) (SaaSAdminBusinessTrendReport, error) {
	billingEvents, err := h.store.SaaSAdminBillingEvents(ctx, SaaSAdminBillingEventOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            options.BillingLimit,
	})
	if err != nil {
		return SaaSAdminBusinessTrendReport{}, err
	}
	taskOptions := SaaSAdminTaskOptions{
		TaskType:         SaaSAdminTaskTypeTenantRenewal,
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            options.TaskLimit,
	}
	taskSummary, err := h.store.SaaSAdminTaskSummary(ctx, taskOptions)
	if err != nil {
		return SaaSAdminBusinessTrendReport{}, err
	}
	tasks, err := h.store.SaaSAdminTasks(ctx, taskOptions)
	if err != nil {
		return SaaSAdminBusinessTrendReport{}, err
	}
	return saasAdminBuildBusinessTrendReport(options, billingEvents, taskSummary, tasks, time.Now()), nil
}

func (h *SaaSAdminHandler) buildRenewalForecast(ctx context.Context, options SaaSAdminRenewalForecastOptions) (SaaSAdminRenewalForecastReport, error) {
	overview, err := h.saasAdminOverview(ctx, SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopePlatform,
		Limit:        options.TenantLimit,
		ExpiringDays: options.Days,
		PackageCode:  options.PackageCode,
		DueState:     SaaSAdminDueStateAll,
	})
	if err != nil {
		return SaaSAdminRenewalForecastReport{}, err
	}
	riskFollowUps := map[int]SaaSAdminRiskFollowUpSnapshot{}
	if tenantIDs := saasAdminTenantOverviewIDs(overview.Tenants); len(tenantIDs) > 0 {
		riskFollowUps, err = h.store.SaaSAdminLatestRiskFollowUps(ctx, tenantIDs)
		if err != nil {
			return SaaSAdminRenewalForecastReport{}, err
		}
	}
	billingEvents, err := h.store.SaaSAdminBillingEvents(ctx, SaaSAdminBillingEventOptions{
		EventType:        "renewal",
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            options.BillingLimit,
	})
	if err != nil {
		return SaaSAdminRenewalForecastReport{}, err
	}
	taskOptions := SaaSAdminTaskOptions{
		TaskType:         SaaSAdminTaskTypeTenantRenewal,
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            options.TaskLimit,
	}
	taskSummary, err := h.store.SaaSAdminTaskSummary(ctx, taskOptions)
	if err != nil {
		return SaaSAdminRenewalForecastReport{}, err
	}
	tasks, err := h.store.SaaSAdminTasks(ctx, taskOptions)
	if err != nil {
		return SaaSAdminRenewalForecastReport{}, err
	}
	return saasAdminBuildRenewalForecastReport(options, overview, billingEvents, taskSummary, tasks, riskFollowUps, time.Now()), nil
}

func saasAdminDailyReportPayload(report SaaSAdminDailyReportData, platformAdminTenantID int) map[string]any {
	options := report.Options
	return map[string]any{
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"window":                saasAdminDailyReportWindowPayload(options),
		"filters":               saasAdminDailyReportFiltersPayload(options),
		"summary":               saasAdminDailyReportSummaryPayload(report.Summary),
		"risk": map[string]any{
			"summary":     saasAdminRiskSummaryPayload(report.RiskReport.Summary),
			"riskTenants": saasAdminRiskTenantPayloads(saasAdminLimitRiskTenants(report.RiskReport.Items, options.ItemLimit)),
		},
		"riskFollowUps": map[string]any{
			"summary": saasAdminRiskFollowUpTaskSummaryPayload(report.RiskTaskSummary),
			"owners":  saasAdminRiskFollowUpOwnerSummaryPayloads(saasAdminLimitRiskFollowUpOwnerSummaries(report.RiskOwners, options.ItemLimit)),
		},
		"taskSla": map[string]any{
			"filters":       saasAdminTaskSLAFiltersPayload(report.TaskSLAReport.Options),
			"taskSummary":   saasAdminTaskSummaryPayload(report.TaskSLAReport.TaskSummary),
			"summary":       saasAdminTaskSLASummaryPayload(report.TaskSLAReport.Summary),
			"ownerCount":    report.TaskSLAReport.OwnerCount,
			"returnedCount": len(report.TaskSLAReport.Tasks),
			"owners":        saasAdminTaskSLAOwnerSummaryPayloads(report.TaskSLAReport.Owners),
			"tasks":         saasAdminTaskSLAItemPayloads(report.TaskSLAReport.Tasks),
		},
		"alerts": map[string]any{
			"openCount": report.AlertPage.Total,
			"items":     saasAdminAlertPayloads(report.AlertPage.Items),
		},
		"notifications": map[string]any{
			"summary":     saasAdminDailyNotificationSummaryPayload(report.NotificationSummary),
			"items":       saasAdminAlertNotificationPayloads(saasAdminLimitNotifications(report.RetryableNotifications, options.ItemLimit)),
			"closedItems": saasAdminAlertNotificationPayloads(saasAdminLimitNotifications(report.ClosedNotifications, options.ItemLimit)),
		},
		"operationQueueAssignments": map[string]any{
			"filters":         saasAdminOperationQueueAssignmentFiltersPayload(report.QueueAssignmentReport.Options),
			"summary":         saasAdminOperationQueueAssignmentSummaryPayload(report.QueueAssignmentReport.Summary),
			"assignmentCount": report.QueueAssignmentReport.Summary.AssignmentCount,
			"returnedCount":   report.QueueAssignmentReport.ReturnedCount,
			"assignments":     saasAdminOperationQueueAssignmentPayloads(report.QueueAssignmentReport.Assignments),
		},
		"operations": map[string]any{
			"summary":    saasAdminDailyOperationActionSummaryPayloads(report.OperationActionSummary),
			"operations": saasAdminOperationLogPayloads(saasAdminLimitOperationLogs(report.WindowOperations, options.ItemLimit)),
		},
		"billing": map[string]any{
			"summary":       saasAdminDailyBillingSummaryPayload(report.BillingSummary),
			"billingEvents": saasAdminBillingEventPayloads(saasAdminLimitBillingEvents(report.WindowBillingEvents, options.ItemLimit)),
		},
	}
}

func (h *SaaSAdminHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	kind, ok := saasAdminExportKind(w, r)
	if !ok {
		return
	}
	limit := positiveQueryInt(r, "limit", 1000)
	if limit > saasAdminExportMaxLimit {
		limit = saasAdminExportMaxLimit
	}

	var writeCSV func(*csv.Writer)
	switch kind {
	case SaaSAdminExportKindTenants:
		options, ok := h.exportTenantOptions(w, r, limit)
		if !ok {
			return
		}
		overview, err := h.saasAdminOverview(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminTenantCSV(writer, overview.Tenants)
		}
	case SaaSAdminExportKindTenantLifecycle:
		tenantID := saasAdminQueryInt(r, "tenantId", 0)
		if tenantID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId required", nil)
			return
		}
		filter, err := parseSaaSAdminTenantLifecycleFilter(r)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		lifecycle, err := h.buildTenantLifecycle(r.Context(), tenantID, limit, positiveQueryInt(r, "expiringDays", 30), filter)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		timeline, _, _ := saasAdminTenantLifecycleTimeline(lifecycle, limit)
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminTenantLifecycleCSV(writer, lifecycle, timeline)
		}
	case SaaSAdminExportKindUsage:
		options, ok := h.exportTenantOptions(w, r, limit)
		if !ok {
			return
		}
		overview, err := h.saasAdminOverview(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		usageByTenant := make(map[int][]SaaSAdminUsageMetric, len(overview.Tenants))
		for _, tenant := range overview.Tenants {
			if tenant.TenantID <= 0 {
				continue
			}
			metrics, err := h.store.SaaSAdminTenantUsage(r.Context(), tenant.TenantID)
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			usageByTenant[tenant.TenantID] = metrics
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminUsageCSV(writer, overview.Tenants, usageByTenant)
		}
	case SaaSAdminExportKindPackages:
		packages, err := h.store.SaaSAdminPackages(r.Context())
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminPackageCSV(writer, packages)
		}
	case SaaSAdminExportKindSubscriptions:
		options, err := parseSaaSAdminSubscriptionOptions(r, saasAdminExportMaxLimit)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		options.Limit = limit
		report, err := h.store.SaaSAdminSubscriptions(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminSubscriptionCSV(writer, report)
		}
	case SaaSAdminExportKindPaymentOrders:
		store, ok := h.paymentStore(w)
		if !ok {
			return
		}
		options, err := parseSaaSAdminPaymentOrderOptions(r, saasAdminExportMaxLimit)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		options.Limit = limit
		report, err := store.SaaSAdminPaymentOrders(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminPaymentOrderCSV(writer, report)
		}
	case SaaSAdminExportKindPaymentSettlementBatches:
		store, ok := h.paymentSettlementStore(w)
		if !ok {
			return
		}
		options, err := parseSaaSPaymentSettlementBatchOptions(r, saasAdminExportMaxLimit)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		options.Limit = limit
		report, err := store.SaaSAdminPaymentSettlementBatches(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) { writeSaaSPaymentSettlementBatchCSV(writer, report) }
	case SaaSAdminExportKindPaymentSettlementEntries:
		store, ok := h.paymentSettlementStore(w)
		if !ok {
			return
		}
		options, err := parseSaaSPaymentSettlementEntryOptions(r, saasAdminExportMaxLimit)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		options.Limit = limit
		report, err := store.SaaSAdminPaymentSettlementEntries(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) { writeSaaSPaymentSettlementEntryCSV(writer, report) }
	case SaaSAdminExportKindPaymentRefunds:
		store, ok := h.paymentRefundStore(w)
		if !ok {
			return
		}
		options, err := parseSaaSAdminPaymentRefundOptions(r, saasAdminExportMaxLimit)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		options.Limit = limit
		report, err := store.SaaSAdminPaymentRefunds(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminPaymentRefundCSV(writer, report)
		}
	case SaaSAdminExportKindInvoiceDocuments:
		store, ok := h.invoiceStore(w)
		if !ok {
			return
		}
		options, err := parseSaaSInvoiceDocumentOptions(r, saasAdminExportMaxLimit)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		options.Limit = limit
		report, err := store.SaaSInvoiceDocuments(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSInvoiceDocumentsCSV(writer, report)
		}
	case SaaSAdminExportKindRisk:
		options, ok := h.exportRiskOptions(w, r, user, limit)
		if !ok {
			return
		}
		overview, err := h.saasAdminOverview(r.Context(), options.SaaSAdminOverviewOptions)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		usageByTenant := make(map[int][]SaaSAdminUsageMetric, len(overview.Tenants))
		for _, tenant := range overview.Tenants {
			if tenant.TenantID <= 0 {
				continue
			}
			metrics, err := h.store.SaaSAdminTenantUsage(r.Context(), tenant.TenantID)
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			usageByTenant[tenant.TenantID] = metrics
		}
		report := saasAdminBuildRiskReport(overview, usageByTenant, options.HighUsageRatio)
		if err := h.attachRiskFollowUps(r.Context(), &report); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminRiskCSV(writer, report.Items)
		}
	case SaaSAdminExportKindOperationQueue:
		options, ok := h.operationQueueOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		report, err := h.buildOperationQueue(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminOperationQueueCSV(writer, report.Items)
		}
	case SaaSAdminExportKindOperationOwners:
		options, ok := h.operationQueueOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		report, err := h.buildOperationQueueOwnerReport(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminOperationQueueOwnerCSV(writer, report.Owners)
		}
	case SaaSAdminExportKindOperationAssigns:
		options, ok := h.operationQueueAssignmentOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		report, err := h.buildOperationQueueAssignmentReport(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminOperationQueueAssignmentCSV(writer, report.Assignments)
		}
	case SaaSAdminExportKindCustomerSuccess:
		options, ok := h.customerSuccessOptionsWithLimit(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		report, err := h.buildCustomerSuccessReport(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminCustomerSuccessCSV(writer, report.Items)
		}
	case SaaSAdminExportKindCustomerOwners:
		options, ok := h.customerSuccessOptionsWithLimit(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		report, err := h.buildCustomerSuccessOwnerReport(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminCustomerSuccessOwnerCSV(writer, report.Owners)
		}
	case SaaSAdminExportKindRenewalForecast:
		options, ok := h.renewalForecastOptions(w, r)
		if !ok {
			return
		}
		report, err := h.buildRenewalForecast(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminRenewalForecastCSV(writer, report.Items)
		}
	case SaaSAdminExportKindRenewalOwners:
		options, ok := h.renewalForecastOptions(w, r)
		if !ok {
			return
		}
		report, err := h.buildRenewalForecast(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminRenewalForecastOwnerCSV(writer, report.Owners)
		}
	case SaaSAdminExportKindRiskFollowUps:
		options, ok := h.riskFollowUpTaskOptionsWithLimit(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		storeOptions := options
		storeOptions.DueState = SaaSAdminRiskFollowUpDueStateAll
		storeOptions.Limit = saasAdminExportMaxLimit
		snapshots, err := h.store.SaaSAdminRiskFollowUpSnapshots(r.Context(), storeOptions)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		tasks, _ := saasAdminBuildRiskFollowUpTasks(snapshots, options, time.Now())
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminRiskFollowUpCSV(writer, tasks)
		}
	case SaaSAdminExportKindRiskOwners:
		options, ok := h.riskFollowUpTaskOptionsWithLimit(w, r, saasAdminExportMaxLimit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		storeOptions := options
		storeOptions.DueState = SaaSAdminRiskFollowUpDueStateAll
		storeOptions.Limit = saasAdminExportMaxLimit
		snapshots, err := h.store.SaaSAdminRiskFollowUpSnapshots(r.Context(), storeOptions)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		tasks, _ := saasAdminBuildRiskFollowUpTasks(snapshots, options, time.Now())
		owners := saasAdminBuildRiskFollowUpOwnerSummaries(tasks)
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminFollowUpOwnerCSV(writer, owners)
		}
	case SaaSAdminExportKindTasks:
		options, ok := h.taskOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		tasks, err := h.store.SaaSAdminTasks(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminTaskCSV(writer, tasks)
		}
	case SaaSAdminExportKindTaskSLA:
		options, ok := h.taskSLAOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		summary, err := h.store.SaaSAdminTaskSummary(r.Context(), options.SaaSAdminTaskOptions)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		fetchOptions := options.SaaSAdminTaskOptions
		fetchOptions.Limit = saasAdminExportMaxLimit
		tasks, err := h.store.SaaSAdminTasks(r.Context(), fetchOptions)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		report := saasAdminBuildTaskSLAReport(options, summary, tasks, time.Now(), 3)
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminTaskSLACSV(writer, report.Tasks)
		}
	case SaaSAdminExportKindAlerts:
		options, _, ok := h.alertOptions(w, r, user)
		if !ok {
			return
		}
		options.Page = 1
		options.PerPage = limit
		page, err := h.store.ListSaaSAlerts(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminAlertCSV(writer, page.Items)
		}
	case SaaSAdminExportKindNotifications:
		options, _, ok := h.notificationOptions(w, r, user)
		if !ok {
			return
		}
		options.Limit = limit
		notifications, err := h.store.SaaSAdminAlertNotifications(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminAlertNotificationCSV(writer, notifications)
		}
	case SaaSAdminExportKindNotificationHealth:
		options, ok := h.notificationHealthOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		source, err := h.store.SaaSAdminNotificationHealth(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		report := saasAdminBuildNotificationHealthReport(source, options, time.Now())
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminNotificationHealthCSV(writer, report)
		}
	case SaaSAdminExportKindNotificationSLO:
		options, ok := h.notificationSLOOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		source, err := h.store.SaaSAdminNotificationSLO(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		report := saasAdminBuildNotificationSLOReport(source, options, time.Now())
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminNotificationSLOCSV(writer, report)
		}
	case SaaSAdminExportKindDailyReport:
		options, ok := h.dailyReportOptions(w, r, time.Now())
		if !ok {
			return
		}
		report, err := h.buildDailyReport(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminDailyReportCSV(writer, report)
		}
	case SaaSAdminExportKindBusinessMetrics:
		options, ok := h.businessMetricsOptions(w, r)
		if !ok {
			return
		}
		report, err := h.buildBusinessMetrics(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminBusinessMetricsCSV(writer, report)
		}
	case SaaSAdminExportKindBusinessTrends:
		options := h.businessTrendOptions(r)
		report, err := h.buildBusinessTrends(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminBusinessTrendsCSV(writer, report)
		}
	case SaaSAdminExportKindOperations:
		options, ok := h.operationLogOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		logs, err := h.store.SaaSAdminOperationLogs(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminOperationCSV(writer, logs)
		}
	case SaaSAdminExportKindBillingEvents:
		options, ok := h.billingEventOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		events, err := h.store.SaaSAdminBillingEvents(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminBillingCSV(writer, events)
		}
	case SaaSAdminExportKindBillingFollowUps:
		options, ok := h.billingReconciliationFollowUpOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		storeOptions := options
		storeOptions.DueState = SaaSAdminRiskFollowUpDueStateAll
		storeOptions.Limit = saasAdminExportMaxLimit
		snapshots, err := h.store.SaaSAdminBillingReconciliationFollowUpSnapshots(r.Context(), storeOptions)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		tasks, _ := saasAdminBuildBillingReconciliationFollowUpTasks(snapshots, options, time.Now())
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminBillingReconciliationFollowUpCSV(writer, tasks)
		}
	case SaaSAdminExportKindBillingOwners:
		options, ok := h.billingReconciliationFollowUpOptions(w, r, saasAdminExportMaxLimit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		storeOptions := options
		storeOptions.DueState = SaaSAdminRiskFollowUpDueStateAll
		storeOptions.Limit = saasAdminExportMaxLimit
		snapshots, err := h.store.SaaSAdminBillingReconciliationFollowUpSnapshots(r.Context(), storeOptions)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		tasks, _ := saasAdminBuildBillingReconciliationFollowUpTasks(snapshots, options, time.Now())
		owners := saasAdminBuildBillingReconciliationFollowUpOwnerSummaries(tasks)
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminFollowUpOwnerCSV(writer, owners)
		}
	case SaaSAdminExportKindBillingReconcile:
		options, ok := h.billingReconciliationOptions(w, r, limit, saasAdminExportMaxLimit)
		if !ok {
			return
		}
		report, err := h.store.SaaSAdminBillingReconciliation(r.Context(), options)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		report = saasAdminNormalizeBillingReconciliationReport(report)
		writeCSV = func(writer *csv.Writer) {
			writeSaaSAdminBillingReconciliationCSV(writer, report.Items)
		}
	}
	filenameKind := kind
	if kind == SaaSAdminExportKindPaymentOrders {
		filenameKind = "payment-orders"
	} else if kind == SaaSAdminExportKindPaymentRefunds {
		filenameKind = "payment-refunds"
	} else if kind == SaaSAdminExportKindInvoiceDocuments {
		filenameKind = "invoice-documents"
	} else if kind == SaaSAdminExportKindPaymentSettlementBatches {
		filenameKind = "payment-settlement-batches"
	} else if kind == SaaSAdminExportKindPaymentSettlementEntries {
		filenameKind = "payment-settlement-entries"
	}
	filename := "mochat-saas-" + filenameKind + "-" + time.Now().Format("20060102150405") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(w)
	writeCSV(writer)
	writer.Flush()
}

func (h *SaaSAdminHandler) TenantDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	tenantID := saasAdminQueryInt(r, "tenantId", 0)
	if tenantID <= 0 {
		tenantID = user.TenantID
	}
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	if tenantID != user.TenantID && !canPlatformScope {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return
	}
	expiringDays := positiveQueryInt(r, "expiringDays", 30)
	if expiringDays > 365 {
		expiringDays = 365
	}
	operationLimit := positiveQueryInt(r, "operationLimit", positiveQueryInt(r, "limit", 10))
	if operationLimit > 50 {
		operationLimit = 50
	}
	overview, err := h.saasAdminOverview(r.Context(), SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopeTenant,
		TenantID:     tenantID,
		Limit:        1,
		ExpiringDays: expiringDays,
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(overview.Tenants) == 0 {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "tenant not found", nil)
		return
	}
	operations, err := h.store.SaaSAdminOperationLogs(r.Context(), SaaSAdminOperationLogOptions{
		TenantID: tenantID,
		Limit:    operationLimit,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	detail := SaaSAdminTenantDetail{
		CanPlatformScope:      canPlatformScope,
		PlatformAdminTenantID: h.platformAdminTenantID,
		Summary:               overview.Summary,
		Tenant:                overview.Tenants[0],
		Metrics:               overview.Metrics,
		Operations:            operations,
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTenantDetailPayload(detail))
}

func (h *SaaSAdminHandler) TenantLifecycle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	tenantID := saasAdminQueryInt(r, "tenantId", 0)
	if tenantID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantId required", nil)
		return
	}
	if tenantID == h.platformAdminTenantID {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "平台管理租户不需要生命周期审计", nil)
		return
	}
	limit := positiveQueryInt(r, "limit", 30)
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	filter, err := parseSaaSAdminTenantLifecycleFilter(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	lifecycle, err := h.buildTenantLifecycle(r.Context(), tenantID, limit, positiveQueryInt(r, "expiringDays", 30), filter)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTenantLifecyclePayload(lifecycle, limit))
}

func (h *SaaSAdminHandler) buildTenantLifecycle(ctx context.Context, tenantID int, limit int, expiringDays int, filter SaaSAdminTenantLifecycleFilter) (SaaSAdminTenantLifecycle, error) {
	if tenantID <= 0 {
		return SaaSAdminTenantLifecycle{}, NewSaaSAdminBadRequest("tenantId required")
	}
	if tenantID == h.platformAdminTenantID {
		return SaaSAdminTenantLifecycle{}, NewSaaSAdminBadRequest("平台管理租户不需要生命周期审计")
	}
	if limit <= 0 {
		limit = 30
	}
	if expiringDays <= 0 {
		expiringDays = 30
	}
	overview, err := h.saasAdminOverview(ctx, SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopeTenant,
		TenantID:     tenantID,
		Limit:        1,
		ExpiringDays: expiringDays,
	})
	if err != nil {
		return SaaSAdminTenantLifecycle{}, err
	}
	if len(overview.Tenants) == 0 {
		return SaaSAdminTenantLifecycle{}, NewSaaSAdminNotFound("tenant not found")
	}
	operations, err := h.store.SaaSAdminOperationLogs(ctx, SaaSAdminOperationLogOptions{
		TenantID: tenantID,
		Limit:    limit,
	})
	if err != nil {
		return SaaSAdminTenantLifecycle{}, err
	}
	billingEvents, err := h.store.SaaSAdminBillingEvents(ctx, SaaSAdminBillingEventOptions{
		TenantID: tenantID,
		Limit:    limit,
	})
	if err != nil {
		return SaaSAdminTenantLifecycle{}, err
	}
	tasks, err := h.store.SaaSAdminTasks(ctx, SaaSAdminTaskOptions{
		TenantID: tenantID,
		Limit:    limit,
	})
	if err != nil {
		return SaaSAdminTenantLifecycle{}, err
	}
	alertPage, err := h.store.ListSaaSAlerts(ctx, SaaSAlertListOptions{
		TenantID: tenantID,
		Status:   "",
		Page:     1,
		PerPage:  limit,
	})
	if err != nil {
		return SaaSAdminTenantLifecycle{}, err
	}
	notifications, err := h.store.SaaSAdminAlertNotifications(ctx, SaaSAdminAlertNotificationOptions{
		TenantID: tenantID,
		Limit:    limit,
	})
	if err != nil {
		return SaaSAdminTenantLifecycle{}, err
	}
	lifecycle := SaaSAdminTenantLifecycle{
		PlatformAdminTenantID: h.platformAdminTenantID,
		Tenant:                overview.Tenants[0],
		Filter:                filter,
		Operations:            operations,
		BillingEvents:         billingEvents,
		Tasks:                 tasks,
		Alerts:                alertPage.Items,
		Notifications:         notifications,
	}
	return lifecycle, nil
}

func parseSaaSAdminTenantLifecycleFilter(r *http.Request) (SaaSAdminTenantLifecycleFilter, error) {
	source := normalizeSaaSAdminTenantLifecycleSource(r.URL.Query().Get("source"))
	if source == "__invalid__" {
		return SaaSAdminTenantLifecycleFilter{}, fmt.Errorf("source 必须是 all、operation、billing、task、alert 或 notification")
	}
	return SaaSAdminTenantLifecycleFilter{
		Source:    source,
		EventType: strings.TrimSpace(r.URL.Query().Get("eventType")),
		Status:    strings.TrimSpace(r.URL.Query().Get("status")),
		Keyword:   strings.TrimSpace(r.URL.Query().Get("keyword")),
	}, nil
}

func normalizeSaaSAdminTenantLifecycleSource(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "all", "*":
		return ""
	case "operation", "operations":
		return "operation"
	case "billing", "billingevent", "billingevents", "billing_event", "billing_events":
		return "billing"
	case "task", "tasks":
		return "task"
	case "alert", "alerts":
		return "alert"
	case "notification", "notifications", "alertnotification", "alertnotifications", "alert_notification", "alert_notifications":
		return "notification"
	default:
		return "__invalid__"
	}
}

func (h *SaaSAdminHandler) Usage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	tenantID := saasAdminQueryInt(r, "tenantId", 0)
	if tenantID <= 0 {
		tenantID = user.TenantID
	}
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	if tenantID != user.TenantID && !canPlatformScope {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return
	}
	expiringDays := positiveQueryInt(r, "expiringDays", 30)
	if expiringDays > 365 {
		expiringDays = 365
	}
	overview, err := h.saasAdminOverview(r.Context(), SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopeTenant,
		TenantID:     tenantID,
		Limit:        1,
		ExpiringDays: expiringDays,
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(overview.Tenants) == 0 {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "tenant not found", nil)
		return
	}
	metrics, err := h.store.SaaSAdminTenantUsage(r.Context(), tenantID)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	detail := SaaSAdminTenantUsageDetail{
		CanPlatformScope:      canPlatformScope,
		PlatformAdminTenantID: h.platformAdminTenantID,
		Tenant:                overview.Tenants[0],
		Summary:               saasAdminUsageSummary(metrics),
		Metrics:               metrics,
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTenantUsagePayload(detail))
}

func (h *SaaSAdminHandler) Risk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	options, canPlatformScope, ok := h.riskOptions(w, r, user)
	if !ok {
		return
	}
	overview, err := h.saasAdminOverview(r.Context(), options.SaaSAdminOverviewOptions)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	usageByTenant := make(map[int][]SaaSAdminUsageMetric, len(overview.Tenants))
	for _, tenant := range overview.Tenants {
		if tenant.TenantID <= 0 {
			continue
		}
		metrics, err := h.store.SaaSAdminTenantUsage(r.Context(), tenant.TenantID)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		usageByTenant[tenant.TenantID] = metrics
	}
	report := saasAdminBuildRiskReport(overview, usageByTenant, options.HighUsageRatio)
	if canPlatformScope {
		if err := h.attachRiskFollowUps(r.Context(), &report); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminRiskReportPayload(report, options, canPlatformScope, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) BusinessMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.businessMetricsOptions(w, r)
	if !ok {
		return
	}
	report, err := h.buildBusinessMetrics(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminBusinessMetricsReportPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) BusinessTrends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options := h.businessTrendOptions(r)
	report, err := h.buildBusinessTrends(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminBusinessTrendReportPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) RenewalForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.renewalForecastOptions(w, r)
	if !ok {
		return
	}
	report, err := h.buildRenewalForecast(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminRenewalForecastReportPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) RenewalForecastTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.renewalForecastOptions(w, r)
	if !ok {
		return
	}
	renewalTasks, err := parseSaaSAdminRenewalForecastTasks(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	renewalTasks.Options = options
	renewalTasks.ActorUserID = user.ID
	renewalTasks.ActorTenantID = user.TenantID
	report, err := h.buildRenewalForecast(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := SaaSAdminRenewalForecastTasksResult{
		Options:       options,
		MatchedCount:  len(report.Items),
		PackageCode:   renewalTasks.PackageCode,
		ExpiresAt:     renewalTasks.ExpiresAt,
		Months:        renewalTasks.Months,
		AmountCents:   renewalTasks.AmountCents,
		Currency:      renewalTasks.Currency,
		PaidAt:        renewalTasks.PaidAt,
		PaymentMethod: renewalTasks.PaymentMethod,
		Remark:        renewalTasks.Remark,
		ForceCreate:   renewalTasks.ForceCreate,
		Tasks:         make([]SaaSAdminTask, 0, len(report.Items)),
		Skipped:       make([]SaaSAdminCustomerSuccessRenewalTaskSkipped, 0),
	}
	now := time.Now()
	for _, item := range report.Items {
		tenantID := item.Tenant.TenantID
		if tenantID <= 0 || tenantID == h.platformAdminTenantID {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     "tenant invalid",
			})
			continue
		}
		if !renewalTasks.ForceCreate && item.TaskSummary.ActionableCount > 0 {
			result.SkippedExistingCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     "tenant renewal task already actionable",
			})
			continue
		}
		amountCents := renewalTasks.AmountCents
		if amountCents <= 0 {
			amountCents = item.RenewalAmountCents
		}
		renewal := SaaSAdminTenantRenewal{
			TenantID:        tenantID,
			PackageCode:     saasAdminFirstNonEmpty(renewalTasks.PackageCode, item.Tenant.PackageCode),
			ExpiresAt:       renewalTasks.ExpiresAt,
			AmountCents:     amountCents,
			Currency:        renewalTasks.Currency,
			PaidAt:          renewalTasks.PaidAt,
			PaymentMethod:   renewalTasks.PaymentMethod,
			ExternalOrderNo: saasAdminCustomerSuccessRenewalExternalOrderNo(renewalTasks.ExternalOrderNoPrefix, tenantID),
			Remark:          renewalTasks.Remark,
			ActorUserID:     renewalTasks.ActorUserID,
			ActorTenantID:   renewalTasks.ActorTenantID,
		}
		if renewal.ExpiresAt == "" {
			renewal.ExpiresAt = saasAdminCustomerSuccessRenewalExpiresAt(item.Tenant.ExpiresAt, renewalTasks.Months, now)
		}
		preview, resolvedRenewal, err := h.tenantRenewalPreview(r.Context(), renewal)
		if err != nil {
			var opErr *SaaSAdminOperationError
			if errors.As(err, &opErr) && opErr != nil {
				result.SkippedInvalidCount++
				result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
					TenantID:   tenantID,
					TenantName: item.Tenant.TenantName,
					Reason:     opErr.Message,
				})
				continue
			}
			writeSaaSAdminError(w, err)
			return
		}
		status := SaaSAdminTaskStatusPending
		if preview.Blocked {
			status = SaaSAdminTaskStatusBlocked
			result.BlockedCount++
		} else {
			result.PendingCount++
		}
		task, err := h.store.CreateSaaSAdminTask(r.Context(), SaaSAdminTaskCreate{
			TaskType:      SaaSAdminTaskTypeTenantRenewal,
			Status:        status,
			TenantID:      resolvedRenewal.TenantID,
			PackageCode:   resolvedRenewal.PackageCode,
			ActorUserID:   user.ID,
			ActorTenantID: user.TenantID,
			RequestJSON:   saasAdminPayloadJSON(saasAdminTenantRenewalTaskRequestPayload(resolvedRenewal)),
			PreviewJSON:   saasAdminPayloadJSON(saasAdminTenantRenewalPreviewPayload(preview)),
			Remark:        resolvedRenewal.Remark,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskCreate, SaaSAdminTask{}, task, user, task.Remark, map[string]any{
			"source":  "renewal_forecast",
			"filters": saasAdminRenewalForecastFiltersPayload(options),
		}); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		result.CreatedCount++
		result.Tasks = append(result.Tasks, task)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminRenewalForecastTasksPayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) RenewalForecastNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.renewalForecastOptions(w, r)
	if !ok {
		return
	}
	notify, err := parseSaaSAdminRenewalForecastNotifications(r, options)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	notify.Options = options
	notify.ActorUserID = user.ID
	notify.ActorTenantID = user.TenantID
	report, err := h.buildRenewalForecast(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	existingKeys := map[string]struct{}{}
	if !notify.ForceCreate {
		existing, err := h.store.SaaSAdminAlertNotifications(r.Context(), SaaSAdminAlertNotificationOptions{
			Channel: notify.Channel,
			Keyword: SaaSAlertTypeTenantRenewal,
			Limit:   saasAdminExportMaxLimit,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		for _, notification := range existing {
			existingKeys[notification.NotificationKey] = struct{}{}
		}
	}
	result := SaaSAdminRenewalForecastNotificationsResult{
		Options:       options,
		MatchedCount:  len(report.Items),
		Channel:       notify.Channel,
		MaxAttempts:   notify.MaxAttempts,
		ReminderDays:  notify.ReminderDays,
		Remark:        notify.Remark,
		ForceCreate:   notify.ForceCreate,
		Notifications: make([]SaaSAlertNotification, 0, len(report.Items)),
		Skipped:       make([]SaaSAdminCustomerSuccessRenewalTaskSkipped, 0),
	}
	now := time.Now()
	for _, item := range report.Items {
		tenantID := item.Tenant.TenantID
		if tenantID <= 0 || tenantID == h.platformAdminTenantID {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     "tenant invalid",
			})
			continue
		}
		alert, notificationKey, err := saasAdminRenewalForecastNotificationAlert(item, notify, now)
		if err != nil {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     err.Error(),
			})
			continue
		}
		if _, exists := existingKeys[notificationKey]; exists && !notify.ForceCreate {
			result.SkippedExistingCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     "tenant renewal notification already exists",
			})
			continue
		}
		notification, err := h.store.EnqueueSaaSAlertNotification(r.Context(), alert, notify.Channel, notify.MaxAttempts)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if notification.ID <= 0 {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     "notification outbox unavailable",
			})
			continue
		}
		if _, err := h.store.RecordSaaSAdminOperationLog(r.Context(), SaaSAdminOperationLog{
			TenantID:      tenantID,
			ActorUserID:   notify.ActorUserID,
			ActorTenantID: notify.ActorTenantID,
			Action:        SaaSAdminOperationActionTenantRenewalNotify,
			TargetType:    SaaSAdminOperationTargetAlertNotification,
			TargetID:      strconv.FormatInt(notification.ID, 10),
			TargetName:    notification.NotificationKey,
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"notification": saasAdminAlertNotificationPayload(notification),
				"source":       "renewal_forecast",
				"filters":      saasAdminRenewalForecastFiltersPayload(options),
				"forceCreate":  notify.ForceCreate,
			}),
			Remark: notify.Remark,
		}); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		existingKeys[notificationKey] = struct{}{}
		result.EnqueuedCount++
		result.Notifications = append(result.Notifications, notification)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminRenewalForecastNotificationsPayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) RenewalForecastAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.renewalForecastOptions(w, r)
	if !ok {
		return
	}
	assign, err := parseSaaSAdminRenewalForecastAssign(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	assign.Options = options
	assign.ActorUserID = user.ID
	assign.ActorTenantID = user.TenantID
	report, err := h.buildRenewalForecast(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := SaaSAdminRenewalForecastAssignResult{
		Options:        options,
		MatchedCount:   len(report.Items),
		Status:         assign.Status,
		Owner:          assign.Owner,
		NextFollowUpAt: assign.NextFollowUpAt,
		Remark:         assign.Remark,
		FollowUps:      make([]SaaSAdminRiskFollowUpResult, 0, len(report.Items)),
	}
	for _, item := range report.Items {
		if item.Tenant.TenantID <= 0 || item.Tenant.TenantID == h.platformAdminTenantID {
			continue
		}
		followUp := SaaSAdminRiskFollowUp{
			TenantID:       item.Tenant.TenantID,
			Status:         assign.Status,
			Owner:          assign.Owner,
			NextFollowUpAt: assign.NextFollowUpAt,
			Remark:         assign.Remark,
			ActorUserID:    assign.ActorUserID,
			ActorTenantID:  assign.ActorTenantID,
		}
		assigned, err := h.store.RecordSaaSAdminRiskFollowUp(r.Context(), followUp)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		result.FollowUps = append(result.FollowUps, assigned)
	}
	result.AssignedCount = len(result.FollowUps)
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminRenewalForecastAssignPayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) CustomerSuccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.customerSuccessOptions(w, r)
	if !ok {
		return
	}
	report, err := h.buildCustomerSuccessReport(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminCustomerSuccessReportPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) CustomerSuccessOwners(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.customerSuccessOptionsWithLimit(w, r, 50, saasAdminListMaxLimit)
	if !ok {
		return
	}
	report, err := h.buildCustomerSuccessOwnerReport(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminCustomerSuccessOwnerReportPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) CustomerSuccessAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.customerSuccessOptionsWithLimit(w, r, 50, saasAdminListMaxLimit)
	if !ok {
		return
	}
	assign, err := parseSaaSAdminCustomerSuccessAssign(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	assign.Options = options
	assign.ActorUserID = user.ID
	assign.ActorTenantID = user.TenantID
	report, err := h.buildCustomerSuccessReport(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := SaaSAdminCustomerSuccessAssignResult{
		Options:        options,
		MatchedCount:   len(report.Items),
		Status:         assign.Status,
		Owner:          assign.Owner,
		NextFollowUpAt: assign.NextFollowUpAt,
		Remark:         assign.Remark,
	}
	for _, item := range report.Items {
		if item.Tenant.TenantID <= 0 || item.Tenant.TenantID == h.platformAdminTenantID {
			continue
		}
		followUp := SaaSAdminRiskFollowUp{
			TenantID:       item.Tenant.TenantID,
			Status:         assign.Status,
			Owner:          assign.Owner,
			NextFollowUpAt: assign.NextFollowUpAt,
			Remark:         assign.Remark,
			ActorUserID:    assign.ActorUserID,
			ActorTenantID:  assign.ActorTenantID,
		}
		assigned, err := h.store.RecordSaaSAdminRiskFollowUp(r.Context(), followUp)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		result.FollowUps = append(result.FollowUps, assigned)
	}
	result.AssignedCount = len(result.FollowUps)
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminCustomerSuccessAssignPayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) CustomerSuccessRenewalTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.customerSuccessOptionsWithLimit(w, r, 50, saasAdminListMaxLimit)
	if !ok {
		return
	}
	renewalTasks, err := parseSaaSAdminCustomerSuccessRenewalTasks(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	renewalTasks.Options = options
	renewalTasks.ActorUserID = user.ID
	renewalTasks.ActorTenantID = user.TenantID
	report, err := h.buildCustomerSuccessReport(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := SaaSAdminCustomerSuccessRenewalTasksResult{
		Options:       options,
		MatchedCount:  len(report.Items),
		PackageCode:   renewalTasks.PackageCode,
		ExpiresAt:     renewalTasks.ExpiresAt,
		Months:        renewalTasks.Months,
		AmountCents:   renewalTasks.AmountCents,
		Currency:      renewalTasks.Currency,
		PaidAt:        renewalTasks.PaidAt,
		PaymentMethod: renewalTasks.PaymentMethod,
		Remark:        renewalTasks.Remark,
		ForceCreate:   renewalTasks.ForceCreate,
		Tasks:         make([]SaaSAdminTask, 0, len(report.Items)),
		Skipped:       make([]SaaSAdminCustomerSuccessRenewalTaskSkipped, 0),
	}
	now := time.Now()
	for _, item := range report.Items {
		tenantID := item.Tenant.TenantID
		if tenantID <= 0 || tenantID == h.platformAdminTenantID {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     "tenant invalid",
			})
			continue
		}
		if !renewalTasks.ForceCreate && item.AdminTaskSummary.TenantRenewalCount > 0 {
			result.SkippedExistingCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     "tenant renewal task already actionable",
			})
			continue
		}
		renewal := SaaSAdminTenantRenewal{
			TenantID:        tenantID,
			PackageCode:     saasAdminFirstNonEmpty(renewalTasks.PackageCode, item.Tenant.PackageCode),
			ExpiresAt:       renewalTasks.ExpiresAt,
			AmountCents:     renewalTasks.AmountCents,
			Currency:        renewalTasks.Currency,
			PaidAt:          renewalTasks.PaidAt,
			PaymentMethod:   renewalTasks.PaymentMethod,
			ExternalOrderNo: saasAdminCustomerSuccessRenewalExternalOrderNo(renewalTasks.ExternalOrderNoPrefix, tenantID),
			Remark:          renewalTasks.Remark,
			ActorUserID:     renewalTasks.ActorUserID,
			ActorTenantID:   renewalTasks.ActorTenantID,
		}
		if renewal.ExpiresAt == "" {
			renewal.ExpiresAt = saasAdminCustomerSuccessRenewalExpiresAt(item.Tenant.ExpiresAt, renewalTasks.Months, now)
		}
		preview, resolvedRenewal, err := h.tenantRenewalPreview(r.Context(), renewal)
		if err != nil {
			var opErr *SaaSAdminOperationError
			if errors.As(err, &opErr) && opErr != nil {
				result.SkippedInvalidCount++
				result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
					TenantID:   tenantID,
					TenantName: item.Tenant.TenantName,
					Reason:     opErr.Message,
				})
				continue
			}
			writeSaaSAdminError(w, err)
			return
		}
		status := SaaSAdminTaskStatusPending
		if preview.Blocked {
			status = SaaSAdminTaskStatusBlocked
			result.BlockedCount++
		} else {
			result.PendingCount++
		}
		task, err := h.store.CreateSaaSAdminTask(r.Context(), SaaSAdminTaskCreate{
			TaskType:      SaaSAdminTaskTypeTenantRenewal,
			Status:        status,
			TenantID:      resolvedRenewal.TenantID,
			PackageCode:   resolvedRenewal.PackageCode,
			ActorUserID:   user.ID,
			ActorTenantID: user.TenantID,
			RequestJSON:   saasAdminPayloadJSON(saasAdminTenantRenewalTaskRequestPayload(resolvedRenewal)),
			PreviewJSON:   saasAdminPayloadJSON(saasAdminTenantRenewalPreviewPayload(preview)),
			Remark:        resolvedRenewal.Remark,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskCreate, SaaSAdminTask{}, task, user, task.Remark, map[string]any{
			"source":  "customer_success_queue",
			"filters": saasAdminCustomerSuccessFiltersPayload(options),
		}); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		result.CreatedCount++
		result.Tasks = append(result.Tasks, task)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminCustomerSuccessRenewalTasksPayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) CustomerSuccessRenewalNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.customerSuccessOptionsWithLimit(w, r, 50, saasAdminListMaxLimit)
	if !ok {
		return
	}
	notify, err := parseSaaSAdminCustomerSuccessRenewalNotifications(r, options)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	notify.Options = options
	notify.ActorUserID = user.ID
	notify.ActorTenantID = user.TenantID
	report, err := h.buildCustomerSuccessReport(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	existingKeys := map[string]struct{}{}
	if !notify.ForceCreate {
		existing, err := h.store.SaaSAdminAlertNotifications(r.Context(), SaaSAdminAlertNotificationOptions{
			Channel: notify.Channel,
			Keyword: SaaSAlertTypeTenantRenewal,
			Limit:   saasAdminExportMaxLimit,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		for _, notification := range existing {
			existingKeys[notification.NotificationKey] = struct{}{}
		}
	}
	result := SaaSAdminCustomerSuccessRenewalNotificationsResult{
		Options:       options,
		MatchedCount:  len(report.Items),
		Channel:       notify.Channel,
		MaxAttempts:   notify.MaxAttempts,
		ReminderDays:  notify.ReminderDays,
		Remark:        notify.Remark,
		ForceCreate:   notify.ForceCreate,
		Notifications: make([]SaaSAlertNotification, 0, len(report.Items)),
		Skipped:       make([]SaaSAdminCustomerSuccessRenewalTaskSkipped, 0),
	}
	now := time.Now()
	for _, item := range report.Items {
		tenantID := item.Tenant.TenantID
		if tenantID <= 0 || tenantID == h.platformAdminTenantID {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     "tenant invalid",
			})
			continue
		}
		alert, notificationKey, err := saasAdminCustomerSuccessRenewalNotificationAlert(item, notify, now)
		if err != nil {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     err.Error(),
			})
			continue
		}
		if _, exists := existingKeys[notificationKey]; exists && !notify.ForceCreate {
			result.SkippedExistingCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     "tenant renewal notification already exists",
			})
			continue
		}
		notification, err := h.store.EnqueueSaaSAlertNotification(r.Context(), alert, notify.Channel, notify.MaxAttempts)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if notification.ID <= 0 {
			result.SkippedInvalidCount++
			result.Skipped = append(result.Skipped, SaaSAdminCustomerSuccessRenewalTaskSkipped{
				TenantID:   tenantID,
				TenantName: item.Tenant.TenantName,
				Reason:     "notification outbox unavailable",
			})
			continue
		}
		if _, err := h.store.RecordSaaSAdminOperationLog(r.Context(), SaaSAdminOperationLog{
			TenantID:      tenantID,
			ActorUserID:   notify.ActorUserID,
			ActorTenantID: notify.ActorTenantID,
			Action:        SaaSAdminOperationActionTenantRenewalNotify,
			TargetType:    SaaSAdminOperationTargetAlertNotification,
			TargetID:      strconv.FormatInt(notification.ID, 10),
			TargetName:    notification.NotificationKey,
			AfterJSON: saasAdminPayloadJSON(map[string]any{
				"notification": saasAdminAlertNotificationPayload(notification),
				"source":       "customer_success_queue",
				"filters":      saasAdminCustomerSuccessFiltersPayload(options),
				"forceCreate":  notify.ForceCreate,
			}),
			Remark: notify.Remark,
		}); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		existingKeys[notificationKey] = struct{}{}
		result.EnqueuedCount++
		result.Notifications = append(result.Notifications, notification)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminCustomerSuccessRenewalNotificationsPayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) buildCustomerSuccessReport(ctx context.Context, options SaaSAdminCustomerSuccessOptions) (SaaSAdminCustomerSuccessReport, error) {
	overview, err := h.saasAdminOverview(ctx, SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopePlatform,
		Limit:        options.TenantLimit,
		ExpiringDays: options.ExpiringDays,
		DueState:     SaaSAdminDueStateAll,
	})
	if err != nil {
		return SaaSAdminCustomerSuccessReport{}, err
	}
	usageByTenant := make(map[int][]SaaSAdminUsageMetric, len(overview.Tenants))
	for _, tenant := range overview.Tenants {
		if tenant.TenantID <= 0 || tenant.TenantID == h.platformAdminTenantID {
			continue
		}
		metrics, err := h.store.SaaSAdminTenantUsage(ctx, tenant.TenantID)
		if err != nil {
			return SaaSAdminCustomerSuccessReport{}, err
		}
		usageByTenant[tenant.TenantID] = metrics
	}
	riskReport := saasAdminBuildRiskReport(overview, usageByTenant, options.HighUsageRatio)
	if err := h.attachRiskFollowUps(ctx, &riskReport); err != nil {
		return SaaSAdminCustomerSuccessReport{}, err
	}
	riskSnapshots, err := h.store.SaaSAdminRiskFollowUpSnapshots(ctx, SaaSAdminRiskFollowUpTaskOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		DueState:         SaaSAdminRiskFollowUpDueStateAll,
		Limit:            saasAdminExportMaxLimit,
	})
	if err != nil {
		return SaaSAdminCustomerSuccessReport{}, err
	}
	riskTasks, _ := saasAdminBuildRiskFollowUpTasks(riskSnapshots, SaaSAdminRiskFollowUpTaskOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		DueState:         SaaSAdminRiskFollowUpDueStateAll,
		Limit:            saasAdminExportMaxLimit,
	}, time.Now())
	billingSnapshots, err := h.store.SaaSAdminBillingReconciliationFollowUpSnapshots(ctx, SaaSAdminBillingReconciliationFollowUpOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		DueState:         SaaSAdminRiskFollowUpDueStateAll,
		Limit:            saasAdminExportMaxLimit,
	})
	if err != nil {
		return SaaSAdminCustomerSuccessReport{}, err
	}
	billingTasks, _ := saasAdminBuildBillingReconciliationFollowUpTasks(billingSnapshots, SaaSAdminBillingReconciliationFollowUpOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		DueState:         SaaSAdminRiskFollowUpDueStateAll,
		Limit:            saasAdminExportMaxLimit,
	}, time.Now())
	adminTasks, err := h.store.SaaSAdminTasks(ctx, SaaSAdminTaskOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            saasAdminExportMaxLimit,
	})
	if err != nil {
		return SaaSAdminCustomerSuccessReport{}, err
	}
	notifications, err := h.store.SaaSAdminAlertNotifications(ctx, SaaSAdminAlertNotificationOptions{
		ExcludedTenantID: h.platformAdminTenantID,
		Limit:            saasAdminExportMaxLimit,
	})
	if err != nil {
		return SaaSAdminCustomerSuccessReport{}, err
	}
	items, summary := saasAdminBuildCustomerSuccessQueue(saasAdminCustomerSuccessBuildInput{
		PlatformAdminTenantID: h.platformAdminTenantID,
		RiskReport:            riskReport,
		RiskTasks:             riskTasks,
		BillingTasks:          billingTasks,
		AdminTasks:            adminTasks,
		Notifications:         notifications,
		Options:               options,
	})
	return SaaSAdminCustomerSuccessReport{
		Options: options,
		Summary: summary,
		Items:   items,
	}, nil
}

func (h *SaaSAdminHandler) buildCustomerSuccessOwnerReport(ctx context.Context, options SaaSAdminCustomerSuccessOptions) (SaaSAdminCustomerSuccessOwnerReport, error) {
	queueOptions := options
	queueOptions.Limit = saasAdminExportMaxLimit
	report, err := h.buildCustomerSuccessReport(ctx, queueOptions)
	if err != nil {
		return SaaSAdminCustomerSuccessOwnerReport{}, err
	}
	owners := saasAdminBuildCustomerSuccessOwnerSummaries(report.Items, 3)
	totalOwnerCount := len(owners)
	if options.Limit > 0 && len(owners) > options.Limit {
		owners = owners[:options.Limit]
	}
	return SaaSAdminCustomerSuccessOwnerReport{
		Options:         options,
		Summary:         report.Summary,
		TotalOwnerCount: totalOwnerCount,
		Owners:          owners,
	}, nil
}

func (h *SaaSAdminHandler) RiskFollowUp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	followUp, err := parseSaaSAdminRiskFollowUp(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	followUp.ActorUserID = user.ID
	followUp.ActorTenantID = user.TenantID
	result, err := h.store.RecordSaaSAdminRiskFollowUp(r.Context(), followUp)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminRiskFollowUpPayload(result))
}

func (h *SaaSAdminHandler) RiskFollowUps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.riskFollowUpTaskOptions(w, r)
	if !ok {
		return
	}
	storeOptions := options
	storeOptions.DueState = SaaSAdminRiskFollowUpDueStateAll
	storeOptions.Limit = saasAdminExportMaxLimit
	snapshots, err := h.store.SaaSAdminRiskFollowUpSnapshots(r.Context(), storeOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	tasks, summary := saasAdminBuildRiskFollowUpTasks(snapshots, options, time.Now())
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"canPlatformScope":      true,
		"platformAdminTenantId": h.platformAdminTenantID,
		"filters":               saasAdminRiskFollowUpTaskFiltersPayload(options),
		"summary":               saasAdminRiskFollowUpTaskSummaryPayload(summary),
		"followUps":             saasAdminRiskFollowUpTaskPayloads(tasks),
	})
}

func (h *SaaSAdminHandler) RiskFollowUpOwners(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.riskFollowUpTaskOptionsWithLimit(w, r, saasAdminExportMaxLimit, saasAdminExportMaxLimit)
	if !ok {
		return
	}
	storeOptions := options
	storeOptions.DueState = SaaSAdminRiskFollowUpDueStateAll
	storeOptions.Limit = saasAdminExportMaxLimit
	snapshots, err := h.store.SaaSAdminRiskFollowUpSnapshots(r.Context(), storeOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	tasks, summary := saasAdminBuildRiskFollowUpTasks(snapshots, options, time.Now())
	owners := saasAdminBuildRiskFollowUpOwnerSummaries(tasks)
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"canPlatformScope":      true,
		"platformAdminTenantId": h.platformAdminTenantID,
		"filters":               saasAdminRiskFollowUpTaskFiltersPayload(options),
		"summary":               saasAdminRiskFollowUpTaskSummaryPayload(summary),
		"owners":                saasAdminRiskFollowUpOwnerSummaryPayloads(owners),
	})
}

func (h *SaaSAdminHandler) RiskFollowUpBulkClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	bulkClose, err := parseSaaSAdminRiskFollowUpBulkClose(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	bulkClose.ActorUserID = user.ID
	bulkClose.ActorTenantID = user.TenantID

	storeOptions := bulkClose.Options
	storeOptions.DueState = SaaSAdminRiskFollowUpDueStateAll
	storeOptions.Limit = saasAdminExportMaxLimit
	snapshots, err := h.store.SaaSAdminRiskFollowUpSnapshots(r.Context(), storeOptions)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	tasks, _ := saasAdminBuildRiskFollowUpTasks(snapshots, bulkClose.Options, time.Now())
	result := SaaSAdminRiskFollowUpBulkCloseResult{
		Status: bulkClose.Status,
		Remark: bulkClose.Remark,
	}
	for _, task := range tasks {
		if task.Status == SaaSAdminRiskFollowUpStatusResolved || task.Status == SaaSAdminRiskFollowUpStatusIgnored {
			continue
		}
		followUp := SaaSAdminRiskFollowUp{
			TenantID:       task.TenantID,
			Status:         bulkClose.Status,
			Owner:          task.Owner,
			NextFollowUpAt: "",
			Remark:         bulkClose.Remark,
			ActorUserID:    bulkClose.ActorUserID,
			ActorTenantID:  bulkClose.ActorTenantID,
		}
		closed, err := h.store.RecordSaaSAdminRiskFollowUp(r.Context(), followUp)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		result.FollowUps = append(result.FollowUps, closed)
	}
	result.ClosedCount = len(result.FollowUps)
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminRiskFollowUpBulkClosePayload(result))
}

func (h *SaaSAdminHandler) attachRiskFollowUps(ctx context.Context, report *SaaSAdminRiskReport) error {
	if report == nil || len(report.Items) == 0 {
		return nil
	}
	tenantIDs := saasAdminRiskTenantIDs(report.Items)
	if len(tenantIDs) == 0 {
		return nil
	}
	followUps, err := h.store.SaaSAdminLatestRiskFollowUps(ctx, tenantIDs)
	if err != nil {
		return err
	}
	saasAdminAttachRiskFollowUps(report, followUps, time.Now())
	return nil
}

func (h *SaaSAdminHandler) Alerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	options, canPlatformScope, ok := h.alertOptions(w, r, user)
	if !ok {
		return
	}
	summary, err := h.store.SaaSAdminAlertSummary(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	page, err := h.store.ListSaaSAlerts(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"tenantId":              options.TenantID,
		"canPlatformScope":      canPlatformScope,
		"platformAdminTenantId": h.platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminAlertFiltersPayload(options),
		"page": map[string]any{
			"page":      options.Page,
			"perPage":   options.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"summary":       saasAdminAlertSummaryPayload(summary),
		"returnedCount": len(page.Items),
		"alerts":        saasAdminAlertPayloads(page.Items),
	})
}

func (h *SaaSAdminHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	resolve, err := parseSaaSAdminAlertResolve(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if resolve.TenantID <= 0 {
		resolve.TenantID = user.TenantID
	}
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	if resolve.TenantID != user.TenantID && !canPlatformScope {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return
	}
	resolve.ActorUserID = user.ID
	resolve.ActorTenantID = user.TenantID
	result, err := h.store.ResolveSaaSAdminAlert(r.Context(), resolve)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminAlertResolvePayload(result))
}

func (h *SaaSAdminHandler) BulkResolveAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	resolve, err := parseSaaSAdminAlertBulkResolve(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	if resolve.TenantID > 0 && resolve.TenantID != user.TenantID && !canPlatformScope {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return
	}
	if !canPlatformScope {
		resolve.TenantID = user.TenantID
		resolve.AllowedTenantID = user.TenantID
	}
	resolve.ActorUserID = user.ID
	resolve.ActorTenantID = user.TenantID
	result, err := h.store.BulkResolveSaaSAdminAlerts(r.Context(), resolve)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminAlertBulkResolvePayload(result))
}

func (h *SaaSAdminHandler) Notifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	options, canPlatformScope, ok := h.notificationOptions(w, r, user)
	if !ok {
		return
	}
	summary, err := h.store.SaaSAdminAlertNotificationSummary(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	notifications, err := h.store.SaaSAdminAlertNotifications(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"tenantId":              options.TenantID,
		"canPlatformScope":      canPlatformScope,
		"platformAdminTenantId": h.platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminNotificationFiltersPayload(options),
		"summary":               saasAdminNotificationSummaryPayload(summary),
		"returnedCount":         len(notifications),
		"notifications":         saasAdminAlertNotificationPayloads(notifications),
	})
}

func (h *SaaSAdminHandler) RetryNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	retry, err := parseSaaSAdminNotificationRetry(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	if !canPlatformScope {
		retry.AllowedTenantID = user.TenantID
	}
	retry.ActorUserID = user.ID
	retry.ActorTenantID = user.TenantID
	result, err := h.store.RetrySaaSAdminAlertNotification(r.Context(), retry)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if result.TenantID != user.TenantID && !canPlatformScope {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminNotificationRetryPayload(result))
}

func (h *SaaSAdminHandler) BulkRetryNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	retry, err := parseSaaSAdminNotificationBulkRetry(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	if retry.TenantID > 0 && retry.TenantID != user.TenantID && !canPlatformScope {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return
	}
	if !canPlatformScope {
		retry.TenantID = user.TenantID
		retry.AllowedTenantID = user.TenantID
	}
	retry.ActorUserID = user.ID
	retry.ActorTenantID = user.TenantID
	result, err := h.store.BulkRetrySaaSAdminAlertNotifications(r.Context(), retry)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminNotificationBulkRetryPayload(result))
}

func (h *SaaSAdminHandler) CloseNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	closeReq, err := parseSaaSAdminNotificationClose(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	if !canPlatformScope {
		closeReq.AllowedTenantID = user.TenantID
	}
	closeReq.ActorUserID = user.ID
	closeReq.ActorTenantID = user.TenantID
	result, err := h.store.CloseSaaSAdminAlertNotification(r.Context(), closeReq)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if result.TenantID != user.TenantID && !canPlatformScope {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminNotificationClosePayload(result))
}

func (h *SaaSAdminHandler) BulkCloseNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return
	}
	closeReq, err := parseSaaSAdminNotificationBulkClose(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	if closeReq.TenantID > 0 && closeReq.TenantID != user.TenantID && !canPlatformScope {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return
	}
	if !canPlatformScope {
		closeReq.TenantID = user.TenantID
		closeReq.AllowedTenantID = user.TenantID
	}
	closeReq.ActorUserID = user.ID
	closeReq.ActorTenantID = user.TenantID
	result, err := h.store.BulkCloseSaaSAdminAlertNotifications(r.Context(), closeReq)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminNotificationBulkClosePayload(result))
}

func (h *SaaSAdminHandler) UpsertPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	update, err := parseSaaSAdminPackageUpsert(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionPackageUpsert, 0) {
		return
	}
	plan, err := h.planPackageUpsert(r.Context(), update)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	plan.Update.ActorUserID = user.ID
	plan.Update.ActorTenantID = user.TenantID
	pkg, err := h.store.UpsertSaaSAdminPackage(r.Context(), plan.Update)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	payload := saasAdminPackagePayload(pkg)
	payload["impact"] = saasAdminPackageImpactPayload(plan.Impact)
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *SaaSAdminHandler) packageUpsertImpact(ctx context.Context, update SaaSAdminPackageUpsert) (SaaSAdminPackageImpact, error) {
	plan, err := h.planPackageUpsert(ctx, update)
	if err != nil {
		return SaaSAdminPackageImpact{}, err
	}
	return plan.Impact, nil
}

func (h *SaaSAdminHandler) planPackageUpsert(ctx context.Context, update SaaSAdminPackageUpsert) (SaaSAdminPackageUpsertPlan, error) {
	packages, err := h.store.SaaSAdminPackages(ctx)
	if err != nil {
		return SaaSAdminPackageUpsertPlan{}, err
	}
	var before SaaSAdminPackage
	existing := false
	for _, item := range packages {
		if item.Code == update.Code {
			before = item
			existing = true
			break
		}
	}
	if existing {
		if update.ExpectedVersion != before.Version {
			return SaaSAdminPackageUpsertPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "套餐版本已变化，请刷新后重试"}
		}
	} else if update.ExpectedVersion != 0 {
		return SaaSAdminPackageUpsertPlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "套餐状态已变化，请刷新后重试"}
	}
	after := SaaSAdminPackage{
		Code:        update.Code,
		Name:        update.Name,
		Description: update.Description,
		Status:      update.Status,
		Version:     update.ExpectedVersion + 1,
		Limits:      update.Limits,
	}
	impact := saasAdminBuildPackageImpact(existing, before, after)
	overview, err := h.saasAdminOverview(ctx, SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopePlatform,
		PackageCode:  update.Code,
		DueState:     SaaSAdminDueStateAll,
		Limit:        saasAdminExportMaxLimit,
		ExpiringDays: 30,
	})
	if err != nil {
		return SaaSAdminPackageUpsertPlan{}, err
	}
	impact.AssignedTenantCount = len(overview.Tenants)
	if impact.DecreasedLimitCount == 0 && impact.NewlyLimitedCount == 0 {
		return newSaaSAdminPackageUpsertPlan(update, existing, before, impact), nil
	}
	changedLimitedMetrics := saasAdminLimitedPackageChangeMetrics(impact.Changes)
	if len(changedLimitedMetrics) == 0 {
		return newSaaSAdminPackageUpsertPlan(update, existing, before, impact), nil
	}
	seenOverLimitTenant := map[int]bool{}
	for _, tenant := range overview.Tenants {
		if tenant.TenantID <= 0 {
			continue
		}
		impact.CheckedTenantCount++
		metrics, err := h.store.SaaSAdminTenantUsage(ctx, tenant.TenantID)
		if err != nil {
			return SaaSAdminPackageUpsertPlan{}, err
		}
		for _, metric := range metrics {
			limit, ok := changedLimitedMetrics[metric.Metric]
			if !ok || limit <= 0 || metric.Current <= limit {
				continue
			}
			if !seenOverLimitTenant[tenant.TenantID] {
				seenOverLimitTenant[tenant.TenantID] = true
				impact.OverLimitTenantCount++
			}
			if len(impact.OverLimitTenants) < 10 {
				impact.OverLimitTenants = append(impact.OverLimitTenants, SaaSAdminPackageImpactTenant{
					TenantID:   tenant.TenantID,
					TenantName: tenant.TenantName,
					Metric:     metric.Metric,
					Label:      saasMetricLabel(metric.Metric),
					Current:    metric.Current,
					Limit:      limit,
				})
			}
		}
	}
	return newSaaSAdminPackageUpsertPlan(update, existing, before, impact), nil
}

func newSaaSAdminPackageUpsertPlan(update SaaSAdminPackageUpsert, existing bool, before SaaSAdminPackage, impact SaaSAdminPackageImpact) SaaSAdminPackageUpsertPlan {
	plan := SaaSAdminPackageUpsertPlan{Update: update, Impact: impact}
	if existing {
		current := before
		plan.Current = &current
	}
	return plan
}

func (h *SaaSAdminHandler) planTenantPackageUpdate(ctx context.Context, update SaaSAdminTenantPackageUpdate) (SaaSAdminTenantPackageUpdatePlan, error) {
	overview, err := h.saasAdminOverview(ctx, SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopeTenant,
		TenantID:     update.TenantID,
		DueState:     SaaSAdminDueStateAll,
		Limit:        1,
		ExpiringDays: 30,
	})
	if err != nil {
		return SaaSAdminTenantPackageUpdatePlan{}, err
	}
	if len(overview.Tenants) == 0 {
		return SaaSAdminTenantPackageUpdatePlan{}, NewSaaSAdminNotFound("tenant not found")
	}
	tenant := overview.Tenants[0]
	existing := tenant.PackageVersion > 0
	if existing {
		if tenant.PackageVersion != update.ExpectedVersion {
			return SaaSAdminTenantPackageUpdatePlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐版本已变化，请刷新后重试"}
		}
	} else if update.ExpectedVersion != 0 {
		return SaaSAdminTenantPackageUpdatePlan{}, &SaaSAdminOperationError{Status: http.StatusConflict, Message: "租户套餐状态已变化，请刷新后重试"}
	}

	packages, err := h.store.SaaSAdminPackages(ctx)
	if err != nil {
		return SaaSAdminTenantPackageUpdatePlan{}, err
	}
	var target SaaSAdminPackage
	found := false
	for _, item := range packages {
		if item.Code == update.PackageCode {
			target = item
			found = true
			break
		}
	}
	if !found {
		return SaaSAdminTenantPackageUpdatePlan{}, NewSaaSAdminNotFound("package not found")
	}
	if target.Status != 1 {
		return SaaSAdminTenantPackageUpdatePlan{}, NewSaaSAdminBadRequest("package disabled")
	}

	update.ExpectedPackageVersion = target.Version
	update.ExpectedTenantStatus = tenant.TenantStatus
	var current *SaaSAdminTenantPackageSnapshot
	var before SaaSAdminPackage
	if existing {
		snapshot := SaaSAdminTenantPackageSnapshot{
			TenantID:     tenant.TenantID,
			TenantName:   tenant.TenantName,
			TenantStatus: tenant.TenantStatus,
			PackageCode:  tenant.PackageCode,
			PackageName:  tenant.PackageName,
			ExpiresAt:    tenant.ExpiresAt,
			Status:       tenant.PackageStatus,
			Version:      tenant.PackageVersion,
			Limits:       tenant.PackageLimits,
		}
		current = &snapshot
		before = SaaSAdminPackage{
			Code:   snapshot.PackageCode,
			Name:   snapshot.PackageName,
			Status: snapshot.Status,
			Limits: snapshot.Limits,
		}
	}
	impact := saasAdminBuildPackageImpact(existing, before, target)
	impact.AssignedTenantCount = 1
	impact.CheckedTenantCount = 1
	limitedMetrics := saasAdminLimitedPackageChangeMetrics(impact.Changes)
	if len(limitedMetrics) > 0 {
		metrics, err := h.store.SaaSAdminTenantUsage(ctx, tenant.TenantID)
		if err != nil {
			return SaaSAdminTenantPackageUpdatePlan{}, err
		}
		overLimit := false
		for _, metric := range metrics {
			limit, ok := limitedMetrics[metric.Metric]
			if !ok || limit <= 0 || metric.Current <= limit {
				continue
			}
			overLimit = true
			if len(impact.OverLimitTenants) < 10 {
				impact.OverLimitTenants = append(impact.OverLimitTenants, SaaSAdminPackageImpactTenant{
					TenantID:   tenant.TenantID,
					TenantName: tenant.TenantName,
					Metric:     metric.Metric,
					Label:      saasMetricLabel(metric.Metric),
					Current:    metric.Current,
					Limit:      limit,
				})
			}
		}
		if overLimit {
			impact.OverLimitTenantCount = 1
		}
	}
	return SaaSAdminTenantPackageUpdatePlan{
		Update:        update,
		TenantName:    tenant.TenantName,
		Current:       current,
		TargetPackage: target,
		Impact:        impact,
	}, nil
}

func (h *SaaSAdminHandler) UpdateTenantStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	update, err := parseSaaSAdminTenantStatusUpdate(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if update.TenantID == h.platformAdminTenantID && update.Status != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "platform tenant cannot be disabled", nil)
		return
	}
	if update.Status == 2 && h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantDisable, 0) {
		return
	}
	if update.Status == 1 && update.TenantID != h.platformAdminTenantID && h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantEnable, 0) {
		return
	}
	update.ActorUserID = user.ID
	update.ActorTenantID = user.TenantID
	result, err := h.store.UpdateSaaSAdminTenantStatus(r.Context(), update)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTenantStatusUpdatePayload(result))
}

func (h *SaaSAdminHandler) UpdateTenantPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	update, err := parseSaaSAdminTenantPackageUpdate(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantPackageUpdate, 0) {
		return
	}
	plan, err := h.planTenantPackageUpdate(r.Context(), update)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	plan.Update.ActorUserID = user.ID
	plan.Update.ActorTenantID = user.TenantID
	result, err := h.store.UpdateSaaSAdminTenantPackage(r.Context(), plan.Update)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	payload := saasAdminTenantPackageUpdatePayload(result)
	payload["impact"] = saasAdminPackageImpactPayload(plan.Impact)
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *SaaSAdminHandler) PackageSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	sync, err := parseSaaSAdminPackageTenantSnapshotSync(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	sync.ActorUserID = user.ID
	sync.ActorTenantID = user.TenantID
	result, err := h.packageTenantSnapshotSyncPlan(r.Context(), sync)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if sync.DryRun {
		writeEnvelope(w, http.StatusOK, 200, "success", saasAdminPackageTenantSnapshotSyncPayload(result))
		return
	}
	if result.OverLimitTenantCount > 0 && !sync.AllowOverLimit {
		result.Blocked = true
		result.BlockReason = "存在超出新套餐额度的租户，需先处理或设置 allowOverLimit"
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "package sync blocked by over-limit tenants", saasAdminPackageTenantSnapshotSyncPayload(result))
		return
	}
	result, err = h.applyPackageTenantSnapshotSync(r.Context(), sync, result)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminPackageTenantSnapshotSyncPayload(result))
}

func (h *SaaSAdminHandler) PackageSyncTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	sync, err := parseSaaSAdminPackageTenantSnapshotSync(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	sync.DryRun = true
	sync.ActorUserID = user.ID
	sync.ActorTenantID = user.TenantID
	preview, err := h.packageTenantSnapshotSyncPlan(r.Context(), sync)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	status := SaaSAdminTaskStatusPending
	if preview.OverLimitTenantCount > 0 && !sync.AllowOverLimit {
		status = SaaSAdminTaskStatusBlocked
		preview.Blocked = true
		preview.BlockReason = "存在超出新套餐额度的租户，需先处理或设置 allowOverLimit"
	}
	task, err := h.store.CreateSaaSAdminTask(r.Context(), SaaSAdminTaskCreate{
		TaskType:      SaaSAdminTaskTypePackageSync,
		Status:        status,
		TenantID:      sync.TenantID,
		PackageCode:   sync.PackageCode,
		ActorUserID:   user.ID,
		ActorTenantID: user.TenantID,
		RequestJSON:   saasAdminPayloadJSON(saasAdminPackageSyncTaskRequestPayload(sync)),
		PreviewJSON:   saasAdminPayloadJSON(saasAdminPackageTenantSnapshotSyncPayload(preview)),
		Remark:        sync.Remark,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskCreate, SaaSAdminTask{}, task, user, task.Remark, nil); err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"task":   saasAdminTaskPayload(task),
		"result": saasAdminPackageTenantSnapshotSyncPayload(preview),
	})
}

func (h *SaaSAdminHandler) PackageSyncTaskApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	taskID, err := parseSaaSAdminTaskID(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	tasks, err := h.store.SaaSAdminTasks(r.Context(), SaaSAdminTaskOptions{TaskID: taskID, Limit: 1})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if len(tasks) == 0 {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "task not found", nil)
		return
	}
	task := tasks[0]
	if task.TaskType != SaaSAdminTaskTypePackageSync {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task type unsupported", nil)
		return
	}
	if task.Status == SaaSAdminTaskStatusApplied {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task already applied", nil)
		return
	}
	if task.Status == SaaSAdminTaskStatusCanceled {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task canceled", nil)
		return
	}
	beforeTask := task
	sync, err := saasAdminPackageSyncFromTaskRequest(task.RequestJSON)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	sync.DryRun = false
	sync.ActorUserID = user.ID
	sync.ActorTenantID = user.TenantID
	result, err := h.packageTenantSnapshotSyncPlan(r.Context(), sync)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if result.OverLimitTenantCount > 0 && !sync.AllowOverLimit {
		result.Blocked = true
		result.BlockReason = "存在超出新套餐额度的租户，需先处理或设置 allowOverLimit"
		task, err = h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
			TaskID:     task.ID,
			Status:     SaaSAdminTaskStatusBlocked,
			ResultJSON: saasAdminPayloadJSON(saasAdminPackageTenantSnapshotSyncPayload(result)),
			LastError:  result.BlockReason,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskBlock, beforeTask, task, user, result.BlockReason, nil); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "package sync task blocked by over-limit tenants", map[string]any{
			"task":   saasAdminTaskPayload(task),
			"result": saasAdminPackageTenantSnapshotSyncPayload(result),
		})
		return
	}
	result, err = h.applyPackageTenantSnapshotSync(r.Context(), sync, result)
	if err != nil {
		_, _ = h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
			TaskID:    task.ID,
			Status:    SaaSAdminTaskStatusFailed,
			LastError: err.Error(),
		})
		writeSaaSAdminError(w, err)
		return
	}
	task, err = h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
		TaskID:     task.ID,
		Status:     SaaSAdminTaskStatusApplied,
		ResultJSON: saasAdminPayloadJSON(saasAdminPackageTenantSnapshotSyncPayload(result)),
		Applied:    true,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskApply, beforeTask, task, user, task.Remark, nil); err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"task":   saasAdminTaskPayload(task),
		"result": saasAdminPackageTenantSnapshotSyncPayload(result),
	})
}

func (h *SaaSAdminHandler) PackageSyncTaskBulkApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	apply, err := parseSaaSAdminPackageSyncTaskBulkApply(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	apply.ActorUserID = user.ID
	apply.ActorTenantID = user.TenantID
	tasks, err := h.store.SaaSAdminTasks(r.Context(), apply.Options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := SaaSAdminTaskBulkApplyResult{
		MatchedCount: len(tasks),
		Options:      apply.Options,
		Remark:       apply.Remark,
		Tasks:        make([]SaaSAdminTask, 0, len(tasks)),
		Errors:       make([]SaaSAdminTaskBulkApplyError, 0),
	}
	for _, task := range tasks {
		if task.TaskType != SaaSAdminTaskTypePackageSync {
			result.SkippedUnsupportedCount++
			continue
		}
		switch task.Status {
		case SaaSAdminTaskStatusApplied:
			result.SkippedAppliedCount++
			continue
		case SaaSAdminTaskStatusCanceled:
			result.SkippedCanceledCount++
			continue
		}
		beforeTask := task
		sync, err := saasAdminPackageSyncFromTaskRequest(task.RequestJSON)
		if err != nil {
			updated, updateErr := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:    task.ID,
				Status:    SaaSAdminTaskStatusFailed,
				LastError: err.Error(),
			})
			if updateErr != nil {
				writeSaaSAdminError(w, updateErr)
				return
			}
			result.FailedCount++
			result.Tasks = append(result.Tasks, updated)
			result.Errors = append(result.Errors, SaaSAdminTaskBulkApplyError{TaskID: task.ID, TenantID: task.TenantID, Error: err.Error()})
			continue
		}
		sync.DryRun = false
		sync.ActorUserID = user.ID
		sync.ActorTenantID = user.TenantID
		plan, err := h.packageTenantSnapshotSyncPlan(r.Context(), sync)
		if err != nil {
			updated, updateErr := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:    task.ID,
				Status:    SaaSAdminTaskStatusFailed,
				LastError: err.Error(),
			})
			if updateErr != nil {
				writeSaaSAdminError(w, updateErr)
				return
			}
			result.FailedCount++
			result.Tasks = append(result.Tasks, updated)
			result.Errors = append(result.Errors, SaaSAdminTaskBulkApplyError{TaskID: task.ID, TenantID: task.TenantID, Error: err.Error()})
			continue
		}
		if plan.OverLimitTenantCount > 0 && !sync.AllowOverLimit {
			plan.Blocked = true
			plan.BlockReason = "存在超出新套餐额度的租户，需先处理或设置 allowOverLimit"
			updated, err := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:     task.ID,
				Status:     SaaSAdminTaskStatusBlocked,
				ResultJSON: saasAdminPayloadJSON(saasAdminPackageTenantSnapshotSyncPayload(plan)),
				LastError:  plan.BlockReason,
			})
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskBlock, beforeTask, updated, user, plan.BlockReason, map[string]any{
				"bulkApply": true,
				"filters":   saasAdminTaskFiltersPayload(apply.Options),
			}); err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			result.BlockedCount++
			result.Tasks = append(result.Tasks, updated)
			continue
		}
		applied, err := h.applyPackageTenantSnapshotSync(r.Context(), sync, plan)
		if err != nil {
			updated, updateErr := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:    task.ID,
				Status:    SaaSAdminTaskStatusFailed,
				LastError: err.Error(),
			})
			if updateErr != nil {
				writeSaaSAdminError(w, updateErr)
				return
			}
			result.FailedCount++
			result.Tasks = append(result.Tasks, updated)
			result.Errors = append(result.Errors, SaaSAdminTaskBulkApplyError{TaskID: task.ID, TenantID: task.TenantID, Error: err.Error()})
			continue
		}
		updated, err := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
			TaskID:     task.ID,
			Status:     SaaSAdminTaskStatusApplied,
			ResultJSON: saasAdminPayloadJSON(saasAdminPackageTenantSnapshotSyncPayload(applied)),
			Applied:    true,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskApply, beforeTask, updated, user, apply.Remark, map[string]any{
			"bulkApply": true,
			"filters":   saasAdminTaskFiltersPayload(apply.Options),
		}); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		result.AppliedCount++
		result.Tasks = append(result.Tasks, updated)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTaskBulkApplyPayload(result))
}

func (h *SaaSAdminHandler) TenantRenewalTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	renewal, err := parseSaaSAdminTenantRenewal(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	renewal.ActorUserID = user.ID
	renewal.ActorTenantID = user.TenantID
	preview, resolvedRenewal, err := h.tenantRenewalPreview(r.Context(), renewal)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	status := SaaSAdminTaskStatusPending
	if preview.Blocked {
		status = SaaSAdminTaskStatusBlocked
	}
	task, err := h.store.CreateSaaSAdminTask(r.Context(), SaaSAdminTaskCreate{
		TaskType:      SaaSAdminTaskTypeTenantRenewal,
		Status:        status,
		TenantID:      resolvedRenewal.TenantID,
		PackageCode:   resolvedRenewal.PackageCode,
		ActorUserID:   user.ID,
		ActorTenantID: user.TenantID,
		RequestJSON:   saasAdminPayloadJSON(saasAdminTenantRenewalTaskRequestPayload(resolvedRenewal)),
		PreviewJSON:   saasAdminPayloadJSON(saasAdminTenantRenewalPreviewPayload(preview)),
		Remark:        resolvedRenewal.Remark,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskCreate, SaaSAdminTask{}, task, user, task.Remark, nil); err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"task":   saasAdminTaskPayload(task),
		"result": saasAdminTenantRenewalPreviewPayload(preview),
	})
}

func (h *SaaSAdminHandler) TenantRenewalTaskApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantRenewal, 0) {
		return
	}
	taskID, err := parseSaaSAdminTaskID(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	tasks, err := h.store.SaaSAdminTasks(r.Context(), SaaSAdminTaskOptions{TaskID: taskID, Limit: 1})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if len(tasks) == 0 {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "task not found", nil)
		return
	}
	task := tasks[0]
	if task.TaskType != SaaSAdminTaskTypeTenantRenewal {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task type unsupported", nil)
		return
	}
	if task.Status == SaaSAdminTaskStatusApplied {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task already applied", nil)
		return
	}
	if task.Status == SaaSAdminTaskStatusCanceled {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task canceled", nil)
		return
	}
	beforeTask := task
	renewal, err := saasAdminTenantRenewalFromTaskRequest(task.RequestJSON)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	renewal.ActorUserID = user.ID
	renewal.ActorTenantID = user.TenantID
	preview, resolvedRenewal, err := h.tenantRenewalPreview(r.Context(), renewal)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if preview.Blocked {
		task, err = h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
			TaskID:     task.ID,
			Status:     SaaSAdminTaskStatusBlocked,
			ResultJSON: saasAdminPayloadJSON(saasAdminTenantRenewalPreviewPayload(preview)),
			LastError:  preview.BlockReason,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskBlock, beforeTask, task, user, preview.BlockReason, nil); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenant renewal task blocked", map[string]any{
			"task":   saasAdminTaskPayload(task),
			"result": saasAdminTenantRenewalPreviewPayload(preview),
		})
		return
	}
	taskDigest := sha256.Sum256([]byte(task.RequestJSON))
	resolvedRenewal.TaskID = task.ID
	resolvedRenewal.ExpectedTaskVersion = task.Version
	resolvedRenewal.ExpectedTaskRequestSHA256 = fmt.Sprintf("%x", taskDigest)
	result, err := h.store.RenewSaaSAdminTenant(r.Context(), resolvedRenewal)
	if err != nil {
		_, _ = h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
			TaskID:    task.ID,
			Status:    SaaSAdminTaskStatusFailed,
			LastError: err.Error(),
		})
		writeSaaSAdminError(w, err)
		return
	}
	task.Status = SaaSAdminTaskStatusApplied
	task.Version++
	task.TenantID = result.TenantID
	task.PackageCode = result.PackageCode
	task.ResultJSON = saasAdminPayloadJSON(saasAdminTenantRenewalPayload(result))
	task.LastError = ""
	task.AppliedAt = time.Now().Format("2006-01-02 15:04:05")
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"task":   saasAdminTaskPayload(task),
		"result": saasAdminTenantRenewalPayload(result),
	})
}

func (h *SaaSAdminHandler) TenantProvisionTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	provision, password, err := parseSaaSAdminTenantProvision(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if strings.TrimSpace(h.passwordSecret) == "" {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "password secret not configured", nil)
		return
	}
	passwordHash, err := authjwt.GeneratePasswordHash(h.passwordSecret, password)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	provision.AdminPasswordHash = passwordHash
	provision.ActorUserID = user.ID
	provision.ActorTenantID = user.TenantID
	preview, err := h.tenantProvisionPreview(r.Context(), provision)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	status := SaaSAdminTaskStatusPending
	if preview.Blocked {
		status = SaaSAdminTaskStatusBlocked
	}
	task, err := h.store.CreateSaaSAdminTask(r.Context(), SaaSAdminTaskCreate{
		TaskType:      SaaSAdminTaskTypeTenantProvision,
		Status:        status,
		TenantID:      provision.TenantID,
		PackageCode:   provision.PackageCode,
		ActorUserID:   user.ID,
		ActorTenantID: user.TenantID,
		RequestJSON:   saasAdminPayloadJSON(saasAdminTenantProvisionTaskRequestPayload(provision)),
		PreviewJSON:   saasAdminPayloadJSON(saasAdminTenantProvisionPreviewPayload(preview)),
		Remark:        provision.Remark,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskCreate, SaaSAdminTask{}, task, user, task.Remark, nil); err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"task":   saasAdminTaskPayload(task),
		"result": saasAdminTenantProvisionPreviewPayload(preview),
	})
}

func (h *SaaSAdminHandler) TenantProvisionTaskApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantProvision, 0) {
		return
	}
	taskID, err := parseSaaSAdminTaskID(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	tasks, err := h.store.SaaSAdminTasks(r.Context(), SaaSAdminTaskOptions{TaskID: taskID, Limit: 1})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if len(tasks) == 0 {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "task not found", nil)
		return
	}
	task := tasks[0]
	if task.TaskType != SaaSAdminTaskTypeTenantProvision {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task type unsupported", nil)
		return
	}
	if task.Status == SaaSAdminTaskStatusApplied {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task already applied", nil)
		return
	}
	if task.Status == SaaSAdminTaskStatusCanceled {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "task canceled", nil)
		return
	}
	beforeTask := task
	provision, err := saasAdminTenantProvisionFromTaskRequest(task.RequestJSON)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	provision.ActorUserID = user.ID
	provision.ActorTenantID = user.TenantID
	preview, err := h.tenantProvisionPreview(r.Context(), provision)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if preview.Blocked {
		task, err = h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
			TaskID:     task.ID,
			Status:     SaaSAdminTaskStatusBlocked,
			ResultJSON: saasAdminPayloadJSON(saasAdminTenantProvisionPreviewPayload(preview)),
			LastError:  preview.BlockReason,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskBlock, beforeTask, task, user, preview.BlockReason, nil); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenant provision task blocked", map[string]any{
			"task":   saasAdminTaskPayload(task),
			"result": saasAdminTenantProvisionPreviewPayload(preview),
		})
		return
	}
	result, err := h.store.ProvisionSaaSAdminTenant(r.Context(), provision)
	if err != nil {
		_, _ = h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
			TaskID:    task.ID,
			Status:    SaaSAdminTaskStatusFailed,
			LastError: err.Error(),
		})
		writeSaaSAdminError(w, err)
		return
	}
	task, err = h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
		TaskID:     task.ID,
		Status:     SaaSAdminTaskStatusApplied,
		ResultJSON: saasAdminPayloadJSON(saasAdminTenantProvisionPayload(result)),
		Applied:    true,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskApply, beforeTask, task, user, task.Remark, nil); err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"task":   saasAdminTaskPayload(task),
		"result": saasAdminTenantProvisionPayload(result),
	})
}

func (h *SaaSAdminHandler) TenantProvisionTaskBulkApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantProvision, 0) {
		return
	}
	apply, err := parseSaaSAdminTenantProvisionTaskBulkApply(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	apply.ActorUserID = user.ID
	apply.ActorTenantID = user.TenantID
	tasks, err := h.store.SaaSAdminTasks(r.Context(), apply.Options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	result := SaaSAdminTaskBulkApplyResult{
		MatchedCount: len(tasks),
		Options:      apply.Options,
		Remark:       apply.Remark,
		Tasks:        make([]SaaSAdminTask, 0, len(tasks)),
		Errors:       make([]SaaSAdminTaskBulkApplyError, 0),
	}
	for _, task := range tasks {
		if task.TaskType != SaaSAdminTaskTypeTenantProvision {
			result.SkippedUnsupportedCount++
			continue
		}
		switch task.Status {
		case SaaSAdminTaskStatusApplied:
			result.SkippedAppliedCount++
			continue
		case SaaSAdminTaskStatusCanceled:
			result.SkippedCanceledCount++
			continue
		}
		beforeTask := task
		provision, err := saasAdminTenantProvisionFromTaskRequest(task.RequestJSON)
		if err != nil {
			updated, updateErr := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:    task.ID,
				Status:    SaaSAdminTaskStatusFailed,
				LastError: err.Error(),
			})
			if updateErr != nil {
				writeSaaSAdminError(w, updateErr)
				return
			}
			result.FailedCount++
			result.Tasks = append(result.Tasks, updated)
			result.Errors = append(result.Errors, SaaSAdminTaskBulkApplyError{TaskID: task.ID, TenantID: task.TenantID, Error: err.Error()})
			continue
		}
		provision.ActorUserID = user.ID
		provision.ActorTenantID = user.TenantID
		preview, err := h.tenantProvisionPreview(r.Context(), provision)
		if err != nil {
			updated, updateErr := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:    task.ID,
				Status:    SaaSAdminTaskStatusFailed,
				LastError: err.Error(),
			})
			if updateErr != nil {
				writeSaaSAdminError(w, updateErr)
				return
			}
			result.FailedCount++
			result.Tasks = append(result.Tasks, updated)
			result.Errors = append(result.Errors, SaaSAdminTaskBulkApplyError{TaskID: task.ID, TenantID: task.TenantID, Error: err.Error()})
			continue
		}
		if preview.Blocked {
			updated, err := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:     task.ID,
				Status:     SaaSAdminTaskStatusBlocked,
				ResultJSON: saasAdminPayloadJSON(saasAdminTenantProvisionPreviewPayload(preview)),
				LastError:  preview.BlockReason,
			})
			if err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskBlock, beforeTask, updated, user, preview.BlockReason, map[string]any{
				"bulkApply": true,
				"filters":   saasAdminTaskFiltersPayload(apply.Options),
			}); err != nil {
				writeSaaSAdminError(w, err)
				return
			}
			result.BlockedCount++
			result.Tasks = append(result.Tasks, updated)
			continue
		}
		provisionResult, err := h.store.ProvisionSaaSAdminTenant(r.Context(), provision)
		if err != nil {
			updated, updateErr := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
				TaskID:    task.ID,
				Status:    SaaSAdminTaskStatusFailed,
				LastError: err.Error(),
			})
			if updateErr != nil {
				writeSaaSAdminError(w, updateErr)
				return
			}
			result.FailedCount++
			result.Tasks = append(result.Tasks, updated)
			result.Errors = append(result.Errors, SaaSAdminTaskBulkApplyError{TaskID: task.ID, TenantID: task.TenantID, Error: err.Error()})
			continue
		}
		updated, err := h.store.UpdateSaaSAdminTaskStatus(r.Context(), SaaSAdminTaskStatusUpdate{
			TaskID:     task.ID,
			Status:     SaaSAdminTaskStatusApplied,
			ResultJSON: saasAdminPayloadJSON(saasAdminTenantProvisionPayload(provisionResult)),
			Applied:    true,
		})
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		if err := h.recordTaskOperation(r.Context(), SaaSAdminOperationActionTaskApply, beforeTask, updated, user, apply.Remark, map[string]any{
			"bulkApply": true,
			"filters":   saasAdminTaskFiltersPayload(apply.Options),
		}); err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		result.AppliedCount++
		result.Tasks = append(result.Tasks, updated)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTaskBulkApplyPayload(result))
}

func (h *SaaSAdminHandler) tenantProvisionPreview(ctx context.Context, provision SaaSAdminTenantProvision) (SaaSAdminTenantProvisionPreview, error) {
	preview, _, err := h.tenantProvisionPreviewWithPackage(ctx, provision)
	return preview, err
}

func (h *SaaSAdminHandler) tenantProvisionPreviewWithPackage(ctx context.Context, provision SaaSAdminTenantProvision) (SaaSAdminTenantProvisionPreview, SaaSAdminPackage, error) {
	packages, err := h.store.SaaSAdminPackages(ctx)
	if err != nil {
		return SaaSAdminTenantProvisionPreview{}, SaaSAdminPackage{}, err
	}
	var pkg SaaSAdminPackage
	found := false
	for _, item := range packages {
		if item.Code == provision.PackageCode {
			pkg = item
			found = true
			break
		}
	}
	if !found {
		return SaaSAdminTenantProvisionPreview{}, SaaSAdminPackage{}, NewSaaSAdminNotFound("package not found")
	}
	if pkg.Status != 1 {
		return SaaSAdminTenantProvisionPreview{}, SaaSAdminPackage{}, NewSaaSAdminBadRequest("package disabled")
	}
	return SaaSAdminTenantProvisionPreview{
		TenantID:       provision.TenantID,
		TenantName:     provision.TenantName,
		AdminPhone:     provision.AdminPhone,
		AdminName:      provision.AdminName,
		RoleName:       provision.RoleName,
		PackageCode:    pkg.Code,
		PackageName:    pkg.Name,
		ExpiresAt:      provision.ExpiresAt,
		ConfigCopyMode: provision.ConfigCopyMode,
		Remark:         provision.Remark,
	}, pkg, nil
}

type saasAdminTenantRenewalPlanState struct {
	CurrentPackage *SaaSAdminTenantPackageSnapshot
	TargetPackage  SaaSAdminPackage
	Subscription   SaaSAdminTenantRenewalSubscriptionSnapshot
}

func (h *SaaSAdminHandler) tenantRenewalPreview(ctx context.Context, renewal SaaSAdminTenantRenewal) (SaaSAdminTenantRenewalPreview, SaaSAdminTenantRenewal, error) {
	preview, resolved, _, err := h.tenantRenewalPreviewState(ctx, renewal)
	return preview, resolved, err
}

func (h *SaaSAdminHandler) tenantRenewalPreviewState(ctx context.Context, renewal SaaSAdminTenantRenewal) (SaaSAdminTenantRenewalPreview, SaaSAdminTenantRenewal, saasAdminTenantRenewalPlanState, error) {
	overview, err := h.saasAdminOverview(ctx, SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopeTenant,
		TenantID:     renewal.TenantID,
		Limit:        1,
		DueState:     SaaSAdminDueStateAll,
		ExpiringDays: 30,
	})
	if err != nil {
		return SaaSAdminTenantRenewalPreview{}, SaaSAdminTenantRenewal{}, saasAdminTenantRenewalPlanState{}, err
	}
	if len(overview.Tenants) == 0 {
		return SaaSAdminTenantRenewalPreview{}, SaaSAdminTenantRenewal{}, saasAdminTenantRenewalPlanState{}, NewSaaSAdminNotFound("tenant not found")
	}
	tenant := overview.Tenants[0]
	packageCode := strings.TrimSpace(renewal.PackageCode)
	if packageCode == "" {
		packageCode = strings.TrimSpace(tenant.PackageCode)
	}
	if packageCode == "" {
		return SaaSAdminTenantRenewalPreview{}, SaaSAdminTenantRenewal{}, saasAdminTenantRenewalPlanState{}, NewSaaSAdminBadRequest("packageCode required")
	}
	packages, err := h.store.SaaSAdminPackages(ctx)
	if err != nil {
		return SaaSAdminTenantRenewalPreview{}, SaaSAdminTenantRenewal{}, saasAdminTenantRenewalPlanState{}, err
	}
	var pkg SaaSAdminPackage
	found := false
	for _, item := range packages {
		if item.Code == packageCode {
			pkg = item
			found = true
			break
		}
	}
	if !found {
		return SaaSAdminTenantRenewalPreview{}, SaaSAdminTenantRenewal{}, saasAdminTenantRenewalPlanState{}, NewSaaSAdminNotFound("package not found")
	}
	if pkg.Status != 1 {
		return SaaSAdminTenantRenewalPreview{}, SaaSAdminTenantRenewal{}, saasAdminTenantRenewalPlanState{}, NewSaaSAdminBadRequest("package disabled")
	}
	subscriptionReport, err := h.store.SaaSAdminSubscriptions(ctx, SaaSAdminSubscriptionOptions{
		TenantID: renewal.TenantID,
		Status:   SaaSAdminSubscriptionStatusAll,
		Access:   SaaSAdminSubscriptionAccessAll,
		Limit:    1,
	})
	if err != nil {
		return SaaSAdminTenantRenewalPreview{}, SaaSAdminTenantRenewal{}, saasAdminTenantRenewalPlanState{}, err
	}
	state := saasAdminTenantRenewalPlanState{TargetPackage: pkg}
	if tenant.PackageVersion > 0 {
		state.CurrentPackage = &SaaSAdminTenantPackageSnapshot{
			TenantID:     tenant.TenantID,
			TenantName:   tenant.TenantName,
			TenantStatus: tenant.TenantStatus,
			PackageCode:  tenant.PackageCode,
			PackageName:  tenant.PackageName,
			ExpiresAt:    tenant.ExpiresAt,
			Status:       tenant.PackageStatus,
			Version:      tenant.PackageVersion,
			Limits:       tenant.PackageLimits,
		}
	}
	if len(subscriptionReport.Subscriptions) > 0 {
		subscription := subscriptionReport.Subscriptions[0]
		state.Subscription = SaaSAdminTenantRenewalSubscriptionSnapshot{
			Exists:               true,
			Status:               subscription.Status,
			Version:              subscription.Version,
			PackageCode:          subscription.PackageCode,
			PackageName:          subscription.PackageName,
			CurrentPeriodEndsAt:  subscription.CurrentPeriodEndsAt,
			LatestBillingEventID: subscription.LatestBillingEventID,
			CancelAtPeriodEnd:    subscription.CancelAtPeriodEnd,
		}
	}
	renewal.PackageCode = pkg.Code
	renewal.ExpectedPackageAssignmentVersion = tenant.PackageVersion
	renewal.ExpectedPackageVersion = pkg.Version
	renewal.ExpectedTenantStatus = tenant.TenantStatus
	renewal.ExpectedSubscriptionExists = state.Subscription.Exists
	renewal.ExpectedSubscriptionVersion = state.Subscription.Version
	preview := SaaSAdminTenantRenewalPreview{
		TenantID:                      renewal.TenantID,
		TenantName:                    tenant.TenantName,
		TenantStatus:                  tenant.TenantStatus,
		PreviousPackageCode:           tenant.PackageCode,
		PreviousPackageName:           tenant.PackageName,
		PreviousPackageStatus:         tenant.PackageStatus,
		PreviousPackageVersion:        tenant.PackageVersion,
		PreviousExpiresAt:             tenant.ExpiresAt,
		PackageCode:                   pkg.Code,
		PackageName:                   pkg.Name,
		PackageVersion:                pkg.Version,
		ExpiresAt:                     renewal.ExpiresAt,
		AmountCents:                   renewal.AmountCents,
		Currency:                      renewal.Currency,
		PaidAt:                        renewal.PaidAt,
		PaymentMethod:                 renewal.PaymentMethod,
		ExternalOrderNo:               renewal.ExternalOrderNo,
		Remark:                        renewal.Remark,
		SubscriptionExists:            state.Subscription.Exists,
		SubscriptionStatus:            state.Subscription.Status,
		SubscriptionVersion:           state.Subscription.Version,
		SubscriptionCurrentPeriodEnds: state.Subscription.CurrentPeriodEndsAt,
	}
	if saasAdminDateTimeBefore(renewal.ExpiresAt, tenant.ExpiresAt) {
		preview.Blocked = true
		preview.BlockReason = "新到期时间早于当前到期时间"
	}
	return preview, renewal, state, nil
}

func (h *SaaSAdminHandler) packageTenantSnapshotSyncPlan(ctx context.Context, sync SaaSAdminPackageTenantSnapshotSync) (SaaSAdminPackageTenantSnapshotSyncResult, error) {
	packages, err := h.store.SaaSAdminPackages(ctx)
	if err != nil {
		return SaaSAdminPackageTenantSnapshotSyncResult{}, err
	}
	var pkg SaaSAdminPackage
	found := false
	for _, item := range packages {
		if item.Code == sync.PackageCode {
			pkg = item
			found = true
			break
		}
	}
	if !found {
		return SaaSAdminPackageTenantSnapshotSyncResult{}, NewSaaSAdminNotFound("package not found")
	}
	if pkg.Status != 1 {
		return SaaSAdminPackageTenantSnapshotSyncResult{}, NewSaaSAdminBadRequest("package disabled")
	}
	options := SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopePlatform,
		TenantID:     sync.TenantID,
		PackageCode:  sync.PackageCode,
		DueState:     SaaSAdminDueStateAll,
		Limit:        sync.Limit,
		ExpiringDays: 30,
	}
	overview, err := h.saasAdminOverview(ctx, options)
	if err != nil {
		return SaaSAdminPackageTenantSnapshotSyncResult{}, err
	}
	result := SaaSAdminPackageTenantSnapshotSyncResult{
		PackageCode:    pkg.Code,
		PackageName:    pkg.Name,
		PackageVersion: pkg.Version,
		Limit:          sync.Limit,
		DryRun:         sync.DryRun,
		AllowOverLimit: sync.AllowOverLimit,
		Tenants:        make([]SaaSAdminPackageTenantSnapshotSyncTenant, 0, len(overview.Tenants)),
	}
	result.MatchedTenantCount = len(overview.Tenants)
	limits := saasAdminPackageLimitMetricMap(pkg.Limits)
	for _, tenant := range overview.Tenants {
		item := SaaSAdminPackageTenantSnapshotSyncTenant{
			TenantID:     tenant.TenantID,
			TenantName:   tenant.TenantName,
			TenantStatus: tenant.TenantStatus,
			Version:      tenant.PackageVersion,
			ExpiresAt:    tenant.ExpiresAt,
			Skipped:      tenant.TenantID <= 0,
		}
		if tenant.TenantID <= 0 {
			result.Tenants = append(result.Tenants, item)
			continue
		}
		result.CheckedTenantCount++
		metrics, err := h.store.SaaSAdminTenantUsage(ctx, tenant.TenantID)
		if err != nil {
			return SaaSAdminPackageTenantSnapshotSyncResult{}, err
		}
		tenantOverLimit := false
		for _, metric := range metrics {
			limit, ok := limits[metric.Metric]
			if !ok || limit <= 0 || metric.Current <= limit {
				continue
			}
			overLimit := SaaSAdminPackageImpactTenant{
				TenantID:   tenant.TenantID,
				TenantName: tenant.TenantName,
				Metric:     metric.Metric,
				Label:      saasMetricLabel(metric.Metric),
				Current:    metric.Current,
				Limit:      limit,
			}
			item.OverLimitMetrics = append(item.OverLimitMetrics, overLimit)
			if len(result.OverLimitTenants) < 10 {
				result.OverLimitTenants = append(result.OverLimitTenants, overLimit)
			}
			tenantOverLimit = true
		}
		if tenantOverLimit {
			result.OverLimitTenantCount++
		}
		result.Tenants = append(result.Tenants, item)
	}
	return result, nil
}

func (h *SaaSAdminHandler) applyPackageTenantSnapshotSync(ctx context.Context, sync SaaSAdminPackageTenantSnapshotSync, result SaaSAdminPackageTenantSnapshotSyncResult) (SaaSAdminPackageTenantSnapshotSyncResult, error) {
	for i := range result.Tenants {
		tenant := &result.Tenants[i]
		if tenant.TenantID <= 0 {
			tenant.Skipped = true
			continue
		}
		updated, err := h.store.UpdateSaaSAdminTenantPackage(ctx, SaaSAdminTenantPackageUpdate{
			TenantID:               tenant.TenantID,
			PackageCode:            sync.PackageCode,
			ExpiresAt:              tenant.ExpiresAt,
			Remark:                 sync.Remark,
			ExpectedVersion:        tenant.Version,
			ExpectedPackageVersion: result.PackageVersion,
			ExpectedTenantStatus:   tenant.TenantStatus,
			ActorUserID:            sync.ActorUserID,
			ActorTenantID:          sync.ActorTenantID,
		})
		if err != nil {
			return SaaSAdminPackageTenantSnapshotSyncResult{}, err
		}
		tenant.Synced = true
		tenant.Skipped = false
		tenant.MetricsRefreshed = updated.MetricsRefreshed
		tenant.Version = updated.Version
		if updated.TenantName != "" {
			tenant.TenantName = updated.TenantName
		}
		if updated.ExpiresAt != "" {
			tenant.ExpiresAt = updated.ExpiresAt
		}
		result.SyncedTenantCount++
		result.MetricsRefreshed += updated.MetricsRefreshed
	}
	result.TenantSnapshotsUpdated = result.SyncedTenantCount > 0
	return result, nil
}

func (h *SaaSAdminHandler) RenewTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantRenewal, 0) {
		return
	}
	renewal, err := parseSaaSAdminTenantRenewal(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	renewal.ActorUserID = user.ID
	renewal.ActorTenantID = user.TenantID
	result, err := h.store.RenewSaaSAdminTenant(r.Context(), renewal)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTenantRenewalPayload(result))
}

func (h *SaaSAdminHandler) ProvisionTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionTenantProvision, 0) {
		return
	}
	provision, password, err := parseSaaSAdminTenantProvision(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if strings.TrimSpace(h.passwordSecret) == "" {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "password secret not configured", nil)
		return
	}
	passwordHash, err := authjwt.GeneratePasswordHash(h.passwordSecret, password)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	provision.AdminPasswordHash = passwordHash
	provision.ActorUserID = user.ID
	provision.ActorTenantID = user.TenantID
	result, err := h.store.ProvisionSaaSAdminTenant(r.Context(), provision)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminTenantProvisionPayload(result))
}

func (h *SaaSAdminHandler) overviewOptions(w http.ResponseWriter, r *http.Request, user User) (SaaSAdminOverviewOptions, bool, bool) {
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	scope := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if scope == "" {
		scope = SaaSAdminScopeTenant
	}
	if scope == "all" {
		scope = SaaSAdminScopePlatform
	}
	if scope != SaaSAdminScopeTenant && scope != SaaSAdminScopePlatform {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "scope 必须是 tenant 或 platform", nil)
		return SaaSAdminOverviewOptions{}, canPlatformScope, false
	}
	tenantID := saasAdminQueryInt(r, "tenantId", 0)
	if scope == SaaSAdminScopePlatform {
		if !canPlatformScope {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
			return SaaSAdminOverviewOptions{}, canPlatformScope, false
		}
	} else {
		if tenantID <= 0 {
			tenantID = user.TenantID
		}
		if tenantID != user.TenantID && !canPlatformScope {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
			return SaaSAdminOverviewOptions{}, canPlatformScope, false
		}
	}
	limit := positiveQueryInt(r, "limit", 20)
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	expiringDays := positiveQueryInt(r, "expiringDays", 30)
	if expiringDays > 365 {
		expiringDays = 365
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "keyword 最多 80 个字符", nil)
		return SaaSAdminOverviewOptions{}, canPlatformScope, false
	}
	packageCode := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("packageCode"), r.URL.Query().Get("package_code")))
	if len([]rune(packageCode)) > 64 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "packageCode 最多 64 个字符", nil)
		return SaaSAdminOverviewOptions{}, canPlatformScope, false
	}
	tenantStatus, ok := saasAdminOptionalTenantStatus(w, r)
	if !ok {
		return SaaSAdminOverviewOptions{}, canPlatformScope, false
	}
	dueState, ok := saasAdminOverviewDueState(w, r)
	if !ok {
		return SaaSAdminOverviewOptions{}, canPlatformScope, false
	}
	return SaaSAdminOverviewOptions{
		Scope:        scope,
		TenantID:     tenantID,
		Limit:        limit,
		ExpiringDays: expiringDays,
		Keyword:      keyword,
		TenantStatus: tenantStatus,
		PackageCode:  packageCode,
		DueState:     dueState,
	}, canPlatformScope, true
}

func (h *SaaSAdminHandler) riskOptions(w http.ResponseWriter, r *http.Request, user User) (SaaSAdminRiskOptions, bool, bool) {
	overviewOptions, canPlatformScope, ok := h.overviewOptions(w, r, user)
	if !ok {
		return SaaSAdminRiskOptions{}, canPlatformScope, false
	}
	highUsageRatio, ok := saasAdminHighUsageRatio(w, r)
	if !ok {
		return SaaSAdminRiskOptions{}, canPlatformScope, false
	}
	return SaaSAdminRiskOptions{
		SaaSAdminOverviewOptions: overviewOptions,
		HighUsageRatio:           highUsageRatio,
	}, canPlatformScope, true
}

func (h *SaaSAdminHandler) businessMetricsOptions(w http.ResponseWriter, r *http.Request) (SaaSAdminBusinessMetricsOptions, bool) {
	tenantLimit := positiveQueryInt(r, "tenantLimit", 500)
	if tenantLimit <= 0 {
		tenantLimit = 500
	}
	if tenantLimit > saasAdminExportMaxLimit {
		tenantLimit = saasAdminExportMaxLimit
	}
	expiringDays := positiveQueryInt(r, "expiringDays", 30)
	if expiringDays > 365 {
		expiringDays = 365
	}
	highUsageRatio, ok := saasAdminHighUsageRatio(w, r)
	if !ok {
		return SaaSAdminBusinessMetricsOptions{}, false
	}
	billingLimit := positiveQueryInt(r, "billingLimit", saasAdminExportMaxLimit)
	if billingLimit <= 0 {
		billingLimit = saasAdminExportMaxLimit
	}
	if billingLimit > saasAdminExportMaxLimit {
		billingLimit = saasAdminExportMaxLimit
	}
	return SaaSAdminBusinessMetricsOptions{
		TenantLimit:    tenantLimit,
		ExpiringDays:   expiringDays,
		HighUsageRatio: highUsageRatio,
		BillingLimit:   billingLimit,
	}, true
}

func (h *SaaSAdminHandler) businessTrendOptions(r *http.Request) SaaSAdminBusinessTrendOptions {
	months := positiveQueryInt(r, "months", 6)
	if months <= 0 {
		months = 6
	}
	if months > 24 {
		months = 24
	}
	billingLimit := positiveQueryInt(r, "billingLimit", saasAdminExportMaxLimit)
	if billingLimit <= 0 {
		billingLimit = saasAdminExportMaxLimit
	}
	if billingLimit > saasAdminExportMaxLimit {
		billingLimit = saasAdminExportMaxLimit
	}
	taskLimit := positiveQueryInt(r, "taskLimit", saasAdminExportMaxLimit)
	if taskLimit <= 0 {
		taskLimit = saasAdminExportMaxLimit
	}
	if taskLimit > saasAdminExportMaxLimit {
		taskLimit = saasAdminExportMaxLimit
	}
	return SaaSAdminBusinessTrendOptions{
		Months:       months,
		BillingLimit: billingLimit,
		TaskLimit:    taskLimit,
	}
}

func (h *SaaSAdminHandler) renewalForecastOptions(w http.ResponseWriter, r *http.Request) (SaaSAdminRenewalForecastOptions, bool) {
	tenantLimit := positiveQueryInt(r, "tenantLimit", 500)
	if tenantLimit <= 0 {
		tenantLimit = 500
	}
	if tenantLimit > saasAdminExportMaxLimit {
		tenantLimit = saasAdminExportMaxLimit
	}
	days := positiveQueryInt(r, "days", 90)
	if days <= 0 {
		days = 90
	}
	if days > 365 {
		days = 365
	}
	billingLimit := positiveQueryInt(r, "billingLimit", saasAdminExportMaxLimit)
	if billingLimit <= 0 {
		billingLimit = saasAdminExportMaxLimit
	}
	if billingLimit > saasAdminExportMaxLimit {
		billingLimit = saasAdminExportMaxLimit
	}
	taskLimit := positiveQueryInt(r, "taskLimit", saasAdminExportMaxLimit)
	if taskLimit <= 0 {
		taskLimit = saasAdminExportMaxLimit
	}
	if taskLimit > saasAdminExportMaxLimit {
		taskLimit = saasAdminExportMaxLimit
	}
	packageCode := strings.TrimSpace(r.URL.Query().Get("packageCode"))
	if len([]rune(packageCode)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "packageCode 最多 80 个字符", nil)
		return SaaSAdminRenewalForecastOptions{}, false
	}
	bucket, ok := saasAdminNormalizeRenewalForecastBucketFilter(r.URL.Query().Get("bucket"))
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "bucket 必须是 all、expired、due_0_30、due_31_60、due_61_90 或 due_later", nil)
		return SaaSAdminRenewalForecastOptions{}, false
	}
	priceState, ok := saasAdminNormalizeRenewalForecastPriceState(r.URL.Query().Get("priced"))
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "priced 必须是 all、true、false、priced 或 unknown", nil)
		return SaaSAdminRenewalForecastOptions{}, false
	}
	owner := strings.TrimSpace(r.URL.Query().Get("owner"))
	if len([]rune(owner)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "owner 最多 80 个字符", nil)
		return SaaSAdminRenewalForecastOptions{}, false
	}
	taskStatus, ok := saasAdminNormalizeRenewalForecastTaskStatus(r.URL.Query().Get("taskStatus"))
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "taskStatus 必须是 all、none、pending、blocked、failed、applied 或 canceled", nil)
		return SaaSAdminRenewalForecastOptions{}, false
	}
	return SaaSAdminRenewalForecastOptions{
		TenantLimit:  tenantLimit,
		Days:         days,
		BillingLimit: billingLimit,
		TaskLimit:    taskLimit,
		PackageCode:  packageCode,
		Bucket:       bucket,
		PriceState:   priceState,
		Owner:        owner,
		TaskStatus:   taskStatus,
	}, true
}

func saasAdminNormalizeRenewalForecastBucketFilter(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", SaaSAdminRenewalForecastFilterAll:
		return SaaSAdminRenewalForecastFilterAll, true
	case "expired":
		return "expired", true
	case "due_0_30", "due0_30", "0_30", "30":
		return "due_0_30", true
	case "due_31_60", "due31_60", "31_60", "60":
		return "due_31_60", true
	case "due_61_90", "due61_90", "61_90", "90":
		return "due_61_90", true
	case "due_later", "later", "after_90":
		return "due_later", true
	default:
		return "", false
	}
}

func saasAdminNormalizeRenewalForecastPriceState(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", SaaSAdminRenewalForecastFilterAll:
		return SaaSAdminRenewalForecastFilterAll, true
	case "1", "true", "yes", SaaSAdminRenewalForecastPriceStatePriced:
		return SaaSAdminRenewalForecastPriceStatePriced, true
	case "0", "false", "no", SaaSAdminRenewalForecastPriceStateUnknown, "unpriced":
		return SaaSAdminRenewalForecastPriceStateUnknown, true
	default:
		return "", false
	}
}

func saasAdminNormalizeRenewalForecastTaskStatus(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", SaaSAdminRenewalForecastFilterAll:
		return SaaSAdminRenewalForecastFilterAll, true
	case SaaSAdminRenewalForecastTaskStatusNone:
		return SaaSAdminRenewalForecastTaskStatusNone, true
	case SaaSAdminTaskStatusPending, SaaSAdminTaskStatusBlocked, SaaSAdminTaskStatusFailed, SaaSAdminTaskStatusApplied, SaaSAdminTaskStatusCanceled:
		return strings.ToLower(strings.TrimSpace(value)), true
	default:
		return "", false
	}
}

func (h *SaaSAdminHandler) customerSuccessOptions(w http.ResponseWriter, r *http.Request) (SaaSAdminCustomerSuccessOptions, bool) {
	return h.customerSuccessOptionsWithLimit(w, r, 50, saasAdminListMaxLimit)
}

func (h *SaaSAdminHandler) customerSuccessOptionsWithLimit(w http.ResponseWriter, r *http.Request, defaultLimit int, maxLimit int) (SaaSAdminCustomerSuccessOptions, bool) {
	tenantLimit := positiveQueryInt(r, "tenantLimit", 200)
	if tenantLimit <= 0 {
		tenantLimit = 200
	}
	if tenantLimit > 500 {
		tenantLimit = 500
	}
	limit := positiveQueryInt(r, "limit", defaultLimit)
	if limit <= 0 {
		limit = defaultLimit
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	expiringDays := positiveQueryInt(r, "expiringDays", 30)
	if expiringDays > 365 {
		expiringDays = 365
	}
	highUsageRatio, ok := saasAdminHighUsageRatio(w, r)
	if !ok {
		return SaaSAdminCustomerSuccessOptions{}, false
	}
	owner := strings.TrimSpace(r.URL.Query().Get("owner"))
	if len([]rune(owner)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "owner 最多 80 个字符", nil)
		return SaaSAdminCustomerSuccessOptions{}, false
	}
	priority, err := normalizeSaaSAdminCustomerSuccessPriority(r.URL.Query().Get("priority"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return SaaSAdminCustomerSuccessOptions{}, false
	}
	return SaaSAdminCustomerSuccessOptions{
		TenantLimit:    tenantLimit,
		Limit:          limit,
		ExpiringDays:   expiringDays,
		HighUsageRatio: highUsageRatio,
		Owner:          owner,
		Priority:       priority,
	}, true
}

func (h *SaaSAdminHandler) operationQueueOptions(w http.ResponseWriter, r *http.Request, defaultLimit int, maxLimit int) (SaaSAdminOperationQueueOptions, bool) {
	tenantLimit := positiveQueryInt(r, "tenantLimit", 200)
	if tenantLimit <= 0 {
		tenantLimit = 200
	}
	if tenantLimit > 500 {
		tenantLimit = 500
	}
	limit := positiveQueryInt(r, "limit", defaultLimit)
	if limit <= 0 {
		limit = defaultLimit
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	expiringDays := positiveQueryInt(r, "expiringDays", 30)
	if expiringDays > 365 {
		expiringDays = 365
	}
	highUsageRatio, ok := saasAdminHighUsageRatio(w, r)
	if !ok {
		return SaaSAdminOperationQueueOptions{}, false
	}
	source, err := normalizeSaaSAdminOperationQueueSource(r.URL.Query().Get("source"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return SaaSAdminOperationQueueOptions{}, false
	}
	priority, err := normalizeSaaSAdminCustomerSuccessPriority(r.URL.Query().Get("priority"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return SaaSAdminOperationQueueOptions{}, false
	}
	owner := strings.TrimSpace(r.URL.Query().Get("owner"))
	if len([]rune(owner)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "owner 最多 80 个字符", nil)
		return SaaSAdminOperationQueueOptions{}, false
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "keyword 最多 80 个字符", nil)
		return SaaSAdminOperationQueueOptions{}, false
	}
	warningHours := positiveQueryInt(r, "warningHours", saasAdminDefaultTaskSLAWarningHours)
	overdueHours := positiveQueryInt(r, "overdueHours", saasAdminDefaultTaskSLAOverdueHours)
	if warningHours > 24*365 || overdueHours > 24*365 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "warningHours 和 overdueHours 必须小于等于 8760", nil)
		return SaaSAdminOperationQueueOptions{}, false
	}
	if warningHours >= overdueHours {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "warningHours 必须小于 overdueHours", nil)
		return SaaSAdminOperationQueueOptions{}, false
	}
	healthWindowHours := positiveQueryInt(r, "healthWindowHours", saasAdminNotificationHealthDefaultWindowHours)
	if healthWindowHours > saasAdminNotificationHealthMaxWindowHours {
		healthWindowHours = saasAdminNotificationHealthMaxWindowHours
	}
	healthStaleMinutes := positiveQueryInt(r, "healthStaleMinutes", saasAdminNotificationHealthDefaultStaleMinutes)
	if healthStaleMinutes > saasAdminNotificationHealthMaxStaleMinutes {
		healthStaleMinutes = saasAdminNotificationHealthMaxStaleMinutes
	}
	return SaaSAdminOperationQueueOptions{
		TenantLimit:        tenantLimit,
		Limit:              limit,
		ExpiringDays:       expiringDays,
		HighUsageRatio:     highUsageRatio,
		Source:             source,
		Priority:           priority,
		Owner:              owner,
		Keyword:            keyword,
		WarningHours:       warningHours,
		OverdueHours:       overdueHours,
		HealthWindowHours:  healthWindowHours,
		HealthStaleMinutes: healthStaleMinutes,
	}, true
}

func (h *SaaSAdminHandler) operationQueueAssignmentOptions(w http.ResponseWriter, r *http.Request, defaultLimit int, maxLimit int) (SaaSAdminOperationQueueAssignmentOptions, bool) {
	limit := positiveQueryInt(r, "limit", defaultLimit)
	if limit <= 0 {
		limit = defaultLimit
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	source, err := normalizeSaaSAdminOperationQueueSource(r.URL.Query().Get("source"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return SaaSAdminOperationQueueAssignmentOptions{}, false
	}
	owner := strings.TrimSpace(r.URL.Query().Get("owner"))
	if len([]rune(owner)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "owner 最多 80 个字符", nil)
		return SaaSAdminOperationQueueAssignmentOptions{}, false
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if len([]rune(status)) > 32 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "status 最多 32 个字符", nil)
		return SaaSAdminOperationQueueAssignmentOptions{}, false
	}
	dueState, err := normalizeSaaSAdminRiskFollowUpDueState(r.URL.Query().Get("dueState"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return SaaSAdminOperationQueueAssignmentOptions{}, false
	}
	objectType := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("objectType"), r.URL.Query().Get("targetType")))
	if len([]rune(objectType)) > 64 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "objectType 最多 64 个字符", nil)
		return SaaSAdminOperationQueueAssignmentOptions{}, false
	}
	objectID := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("objectId"), r.URL.Query().Get("objectID"), r.URL.Query().Get("targetId"), r.URL.Query().Get("targetID")))
	if len([]rune(objectID)) > 64 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "objectId 最多 64 个字符", nil)
		return SaaSAdminOperationQueueAssignmentOptions{}, false
	}
	tenantID := positiveQueryInt(r, "tenantId", 0)
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "keyword 最多 80 个字符", nil)
		return SaaSAdminOperationQueueAssignmentOptions{}, false
	}
	currentOnly := false
	if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("currentOnly"), r.URL.Query().Get("current_only"))); raw != "" {
		currentOnly, err = parseSaaSAdminRequestBool(raw, "currentOnly")
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return SaaSAdminOperationQueueAssignmentOptions{}, false
		}
	}
	return SaaSAdminOperationQueueAssignmentOptions{
		Source:      source,
		Owner:       owner,
		Status:      status,
		DueState:    dueState,
		ObjectType:  objectType,
		ObjectID:    objectID,
		TenantID:    tenantID,
		Keyword:     keyword,
		Limit:       limit,
		CurrentOnly: currentOnly,
	}, true
}

func (h *SaaSAdminHandler) dailyReportOptions(w http.ResponseWriter, r *http.Request, now time.Time) (SaaSAdminDailyReportOptions, bool) {
	reportDate := strings.TrimSpace(r.URL.Query().Get("date"))
	if reportDate == "" {
		reportDate = now.Format("2006-01-02")
	}
	day, err := time.ParseInLocation("2006-01-02", reportDate, time.Local)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "date 必须是 YYYY-MM-DD", nil)
		return SaaSAdminDailyReportOptions{}, false
	}
	days := positiveQueryInt(r, "days", 1)
	if days <= 0 {
		days = 1
	}
	if days > 31 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "days 必须在 1 到 31 之间", nil)
		return SaaSAdminDailyReportOptions{}, false
	}
	expiringDays := positiveQueryInt(r, "expiringDays", 30)
	if expiringDays > 365 {
		expiringDays = 365
	}
	tenantLimit := positiveQueryInt(r, "tenantLimit", 200)
	if tenantLimit <= 0 {
		tenantLimit = 200
	}
	if tenantLimit > 500 {
		tenantLimit = 500
	}
	itemLimit := positiveQueryInt(r, "limit", 20)
	if itemLimit <= 0 {
		itemLimit = 20
	}
	if itemLimit > saasAdminListMaxLimit {
		itemLimit = saasAdminListMaxLimit
	}
	highUsageRatio, ok := saasAdminHighUsageRatio(w, r)
	if !ok {
		return SaaSAdminDailyReportOptions{}, false
	}
	windowStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
	return SaaSAdminDailyReportOptions{
		ReportDate:     windowStart.Format("2006-01-02"),
		Days:           days,
		WindowStart:    windowStart,
		WindowEnd:      windowStart.AddDate(0, 0, days),
		ExpiringDays:   expiringDays,
		HighUsageRatio: highUsageRatio,
		TenantLimit:    tenantLimit,
		ItemLimit:      itemLimit,
	}, true
}

func (h *SaaSAdminHandler) exportTenantOptions(w http.ResponseWriter, r *http.Request, limit int) (SaaSAdminOverviewOptions, bool) {
	expiringDays := positiveQueryInt(r, "expiringDays", 30)
	if expiringDays > 365 {
		expiringDays = 365
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "keyword 最多 80 个字符", nil)
		return SaaSAdminOverviewOptions{}, false
	}
	packageCode := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("packageCode"), r.URL.Query().Get("package_code")))
	if len([]rune(packageCode)) > 64 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "packageCode 最多 64 个字符", nil)
		return SaaSAdminOverviewOptions{}, false
	}
	tenantStatus, ok := saasAdminOptionalTenantStatus(w, r)
	if !ok {
		return SaaSAdminOverviewOptions{}, false
	}
	dueState, ok := saasAdminOverviewDueState(w, r)
	if !ok {
		return SaaSAdminOverviewOptions{}, false
	}
	return SaaSAdminOverviewOptions{
		Scope:        SaaSAdminScopePlatform,
		TenantID:     saasAdminQueryInt(r, "tenantId", 0),
		Limit:        limit,
		ExpiringDays: expiringDays,
		Keyword:      keyword,
		TenantStatus: tenantStatus,
		PackageCode:  packageCode,
		DueState:     dueState,
	}, true
}

func (h *SaaSAdminHandler) exportRiskOptions(w http.ResponseWriter, r *http.Request, user User, limit int) (SaaSAdminRiskOptions, bool) {
	overviewOptions, ok := h.exportTenantOptions(w, r, limit)
	if !ok {
		return SaaSAdminRiskOptions{}, false
	}
	if user.TenantID != h.platformAdminTenantID {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return SaaSAdminRiskOptions{}, false
	}
	highUsageRatio, ok := saasAdminHighUsageRatio(w, r)
	if !ok {
		return SaaSAdminRiskOptions{}, false
	}
	return SaaSAdminRiskOptions{
		SaaSAdminOverviewOptions: overviewOptions,
		HighUsageRatio:           highUsageRatio,
	}, true
}

func saasAdminHighUsageRatio(w http.ResponseWriter, r *http.Request) (float64, bool) {
	highUsageRatio := saasAdminDefaultHighUsageRatio
	rawRatio := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("highUsageRatio"), r.URL.Query().Get("usageThreshold")))
	if rawRatio == "" {
		return highUsageRatio, true
	}
	value, err := strconv.ParseFloat(rawRatio, 64)
	if err != nil || value < 0.5 || value > 1.5 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "highUsageRatio 必须在 0.5 到 1.5 之间", nil)
		return 0, false
	}
	return value, true
}

func (h *SaaSAdminHandler) operationLogOptions(w http.ResponseWriter, r *http.Request, limit int, maxLimit int) (SaaSAdminOperationLogOptions, bool) {
	if limit <= 0 {
		limit = 20
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	options := SaaSAdminOperationLogOptions{
		TenantID: saasAdminQueryInt(r, "tenantId", 0),
		Limit:    limit,
	}
	var ok bool
	if options.Action, ok = saasAdminQueryString(w, "action", 64, r.URL.Query().Get("action")); !ok {
		return SaaSAdminOperationLogOptions{}, false
	}
	if options.TargetType, ok = saasAdminQueryString(w, "targetType", 32, r.URL.Query().Get("targetType"), r.URL.Query().Get("target_type")); !ok {
		return SaaSAdminOperationLogOptions{}, false
	}
	if options.Keyword, ok = saasAdminQueryString(w, "keyword", 80, r.URL.Query().Get("keyword")); !ok {
		return SaaSAdminOperationLogOptions{}, false
	}
	return options, true
}

func (h *SaaSAdminHandler) riskFollowUpTaskOptions(w http.ResponseWriter, r *http.Request) (SaaSAdminRiskFollowUpTaskOptions, bool) {
	return h.riskFollowUpTaskOptionsWithLimit(w, r, 50, saasAdminListMaxLimit)
}

func (h *SaaSAdminHandler) riskFollowUpTaskOptionsWithLimit(w http.ResponseWriter, r *http.Request, defaultLimit int, maxLimit int) (SaaSAdminRiskFollowUpTaskOptions, bool) {
	limit := positiveQueryInt(r, "limit", defaultLimit)
	if limit <= 0 {
		limit = 50
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	status, ok := saasAdminOptionalRiskFollowUpStatus(w, r)
	if !ok {
		return SaaSAdminRiskFollowUpTaskOptions{}, false
	}
	dueState, ok := saasAdminRiskFollowUpDueState(w, r)
	if !ok {
		return SaaSAdminRiskFollowUpTaskOptions{}, false
	}
	owner := strings.TrimSpace(r.URL.Query().Get("owner"))
	if len([]rune(owner)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "owner 最多 80 个字符", nil)
		return SaaSAdminRiskFollowUpTaskOptions{}, false
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "keyword 最多 80 个字符", nil)
		return SaaSAdminRiskFollowUpTaskOptions{}, false
	}
	return SaaSAdminRiskFollowUpTaskOptions{
		TenantID: saasAdminQueryInt(r, "tenantId", 0),
		Status:   status,
		Owner:    owner,
		DueState: dueState,
		Keyword:  keyword,
		Limit:    limit,
	}, true
}

func (h *SaaSAdminHandler) billingReconciliationFollowUpOptions(w http.ResponseWriter, r *http.Request, defaultLimit int, maxLimit int) (SaaSAdminBillingReconciliationFollowUpOptions, bool) {
	options, ok := h.riskFollowUpTaskOptionsWithLimit(w, r, defaultLimit, maxLimit)
	if !ok {
		return SaaSAdminBillingReconciliationFollowUpOptions{}, false
	}
	return SaaSAdminBillingReconciliationFollowUpOptions{
		TenantID: options.TenantID,
		Status:   options.Status,
		Owner:    options.Owner,
		DueState: options.DueState,
		Keyword:  options.Keyword,
		Limit:    options.Limit,
	}, true
}

func (h *SaaSAdminHandler) billingEventOptions(w http.ResponseWriter, r *http.Request, limit int, maxLimit int) (SaaSAdminBillingEventOptions, bool) {
	if limit <= 0 {
		limit = 20
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	options := SaaSAdminBillingEventOptions{
		TenantID: saasAdminQueryInt(r, "tenantId", 0),
		Limit:    limit,
	}
	var ok bool
	if options.EventType, ok = saasAdminQueryString(w, "eventType", 32, r.URL.Query().Get("eventType"), r.URL.Query().Get("event_type")); !ok {
		return SaaSAdminBillingEventOptions{}, false
	}
	if options.PackageCode, ok = saasAdminQueryString(w, "packageCode", 64, r.URL.Query().Get("packageCode"), r.URL.Query().Get("package_code")); !ok {
		return SaaSAdminBillingEventOptions{}, false
	}
	if options.Keyword, ok = saasAdminQueryString(w, "keyword", 80, r.URL.Query().Get("keyword")); !ok {
		return SaaSAdminBillingEventOptions{}, false
	}
	return options, true
}

func (h *SaaSAdminHandler) billingReconciliationOptions(w http.ResponseWriter, r *http.Request, limit int, maxLimit int) (SaaSAdminBillingReconciliationOptions, bool) {
	eventOptions, ok := h.billingEventOptions(w, r, limit, maxLimit)
	if !ok {
		return SaaSAdminBillingReconciliationOptions{}, false
	}
	options := SaaSAdminBillingReconciliationOptions{SaaSAdminBillingEventOptions: eventOptions}
	raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("mismatchOnly"), r.URL.Query().Get("mismatch_only")))
	if raw != "" {
		value, err := parseSaaSAdminRequestBool(raw, "mismatchOnly")
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return SaaSAdminBillingReconciliationOptions{}, false
		}
		options.MismatchOnly = value
	}
	return options, true
}

func (h *SaaSAdminHandler) taskOptions(w http.ResponseWriter, r *http.Request, limit int, maxLimit int) (SaaSAdminTaskOptions, bool) {
	if limit <= 0 {
		limit = 20
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	options := SaaSAdminTaskOptions{
		TenantID: saasAdminQueryInt(r, "tenantId", 0),
		Limit:    limit,
	}
	rawTaskID := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("taskId"), r.URL.Query().Get("task_id"), r.URL.Query().Get("id")))
	if rawTaskID != "" {
		taskID, err := strconv.ParseInt(rawTaskID, 10, 64)
		if err != nil || taskID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "taskId invalid", nil)
			return SaaSAdminTaskOptions{}, false
		}
		options.TaskID = taskID
	}
	taskType, ok := saasAdminQueryString(w, "taskType", 64, r.URL.Query().Get("taskType"), r.URL.Query().Get("task_type"), r.URL.Query().Get("type"))
	if !ok {
		return SaaSAdminTaskOptions{}, false
	}
	taskType = strings.ToLower(strings.TrimSpace(taskType))
	if taskType == "all" {
		taskType = ""
	}
	if taskType != "" && !validSaaSAdminTaskType(taskType) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "taskType 必须是 package_sync、tenant_renewal、tenant_provision 或 all", nil)
		return SaaSAdminTaskOptions{}, false
	}
	status, ok := saasAdminTaskStatus(w, r)
	if !ok {
		return SaaSAdminTaskOptions{}, false
	}
	packageCode, ok := saasAdminQueryString(w, "packageCode", 64, r.URL.Query().Get("packageCode"), r.URL.Query().Get("package_code"))
	if !ok {
		return SaaSAdminTaskOptions{}, false
	}
	packageCode = strings.TrimSpace(packageCode)
	if packageCode != "" && !validSaaSAdminPackageCode(packageCode) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "packageCode 仅支持字母、数字、下划线和中划线", nil)
		return SaaSAdminTaskOptions{}, false
	}
	options.TaskType = taskType
	options.Status = status
	options.PackageCode = packageCode
	return options, true
}

func (h *SaaSAdminHandler) taskSLAOptions(w http.ResponseWriter, r *http.Request, limit int, maxLimit int) (SaaSAdminTaskSLAOptions, bool) {
	taskOptions, ok := h.taskOptions(w, r, limit, maxLimit)
	if !ok {
		return SaaSAdminTaskSLAOptions{}, false
	}
	warningHours := positiveQueryInt(r, "warningHours", saasAdminDefaultTaskSLAWarningHours)
	overdueHours := positiveQueryInt(r, "overdueHours", saasAdminDefaultTaskSLAOverdueHours)
	if warningHours > 24*365 || overdueHours > 24*365 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "warningHours 和 overdueHours 必须小于等于 8760", nil)
		return SaaSAdminTaskSLAOptions{}, false
	}
	if warningHours >= overdueHours {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "warningHours 必须小于 overdueHours", nil)
		return SaaSAdminTaskSLAOptions{}, false
	}
	return SaaSAdminTaskSLAOptions{
		SaaSAdminTaskOptions: taskOptions,
		WarningHours:         warningHours,
		OverdueHours:         overdueHours,
	}, true
}

func (h *SaaSAdminHandler) alertOptions(w http.ResponseWriter, r *http.Request, user User) (SaaSAlertListOptions, bool, bool) {
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	tenantID := saasAdminQueryInt(r, "tenantId", 0)
	if tenantID > 0 && tenantID != user.TenantID && !canPlatformScope {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return SaaSAlertListOptions{}, canPlatformScope, false
	}
	if !canPlatformScope {
		tenantID = user.TenantID
	}
	status, ok := saasAdminAlertStatus(w, r)
	if !ok {
		return SaaSAlertListOptions{}, canPlatformScope, false
	}
	page := positiveQueryInt(r, "page", 1)
	perPage := positiveQueryInt(r, "perPage", positiveQueryInt(r, "limit", 20))
	if perPage > saasAdminListMaxLimit {
		perPage = saasAdminListMaxLimit
	}
	metric, ok := saasAdminQueryString(w, "metric", 64, r.URL.Query().Get("metric"))
	if !ok {
		return SaaSAlertListOptions{}, canPlatformScope, false
	}
	alertType, ok := saasAdminQueryString(w, "alertType", 64, r.URL.Query().Get("alertType"), r.URL.Query().Get("alert_type"))
	if !ok {
		return SaaSAlertListOptions{}, canPlatformScope, false
	}
	return SaaSAlertListOptions{
		TenantID:  tenantID,
		Status:    status,
		Metric:    metric,
		AlertType: alertType,
		Page:      page,
		PerPage:   perPage,
	}, canPlatformScope, true
}

func (h *SaaSAdminHandler) notificationOptions(w http.ResponseWriter, r *http.Request, user User) (SaaSAdminAlertNotificationOptions, bool, bool) {
	canPlatformScope := user.TenantID == h.platformAdminTenantID
	tenantID := saasAdminQueryInt(r, "tenantId", 0)
	if tenantID > 0 && tenantID != user.TenantID && !canPlatformScope {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return SaaSAdminAlertNotificationOptions{}, canPlatformScope, false
	}
	if !canPlatformScope {
		tenantID = user.TenantID
	}
	limit := positiveQueryInt(r, "limit", 20)
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	status, ok := saasAdminNotificationStatus(w, r)
	if !ok {
		return SaaSAdminAlertNotificationOptions{}, canPlatformScope, false
	}
	channel, ok := saasAdminQueryString(w, "channel", 32, r.URL.Query().Get("channel"))
	if !ok {
		return SaaSAdminAlertNotificationOptions{}, canPlatformScope, false
	}
	keyword, ok := saasAdminQueryString(w, "keyword", 80, r.URL.Query().Get("keyword"))
	if !ok {
		return SaaSAdminAlertNotificationOptions{}, canPlatformScope, false
	}
	return SaaSAdminAlertNotificationOptions{
		TenantID: tenantID,
		Status:   status,
		Channel:  channel,
		Keyword:  keyword,
		Limit:    limit,
	}, canPlatformScope, true
}

func (h *SaaSAdminHandler) resolveSuperAdmin(w http.ResponseWriter, r *http.Request) (User, bool) {
	var user User
	if _, principalErr := saasauth.PrincipalFromContext(r.Context()); principalErr == nil {
		resolved, found, err := resolveSaaSAdminActor(r.Context(), h.store, h.platformAdminTenantID)
		if errors.Is(err, ErrSaaSAdminActorStoreUnavailable) {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, err.Error(), nil)
			return User{}, false
		}
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return User{}, false
		}
		if !found || resolved.Status != 1 {
			writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
			return User{}, false
		}
		user = resolved
	} else {
		// The production SaaS route is constructed with a nil resolver. This
		// compatibility branch keeps direct legacy unit-test construction
		// isolated from the verified SaaS principal path.
		if h.resolver == nil {
			writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
			return User{}, false
		}
		userID, err := h.resolver.UserID(r)
		if err != nil || userID <= 0 {
			writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
			return User{}, false
		}
		resolved, found, err := h.store.UserByID(r.Context(), userID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return User{}, false
		}
		if !found {
			writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
			return User{}, false
		}
		user = resolved
	}
	if user.TenantID <= 0 {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return User{}, false
	}
	if user.IsSuperAdmin != 1 {
		if user.TenantID != h.platformAdminTenantID {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
			return User{}, false
		}
		required := SaaSAdminRequiredPermission(r)
		if required == "" {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
			return User{}, false
		}
		profile, accessErr := h.saasAdminAccessProfile(r.Context(), user)
		if accessErr != nil {
			writeSaaSAdminError(w, accessErr)
			return User{}, false
		}
		hasPermission := SaaSAdminAccessHasPermission(profile, required)
		if !hasPermission && required == SaaSAdminPermissionApprovalsReview {
			_, delegated, delegationErr := h.activeSaaSAdminApprovalDelegation(r.Context(), user.ID)
			if delegationErr != nil {
				writeSaaSAdminError(w, delegationErr)
				return User{}, false
			}
			hasPermission = delegated
		}
		if !hasPermission {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "缺少平台权限 "+required, nil)
			return User{}, false
		}
	}
	return user, true
}

func (h *SaaSAdminHandler) resolvePlatformSuperAdmin(w http.ResponseWriter, r *http.Request) (User, bool) {
	user, ok := h.resolveSuperAdmin(w, r)
	if !ok {
		return User{}, false
	}
	if user.TenantID != h.platformAdminTenantID {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, ErrPermissionDenied.Error(), nil)
		return User{}, false
	}
	return user, true
}

func saasAdminQueryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func saasAdminQueryString(w http.ResponseWriter, field string, maxRunes int, values ...string) (string, bool) {
	value := strings.TrimSpace(saasAdminFirstNonEmpty(values...))
	if maxRunes > 0 && len([]rune(value)) > maxRunes {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, field+" 最多 "+strconv.Itoa(maxRunes)+" 个字符", nil)
		return "", false
	}
	return value, true
}

func saasAdminOptionalTenantStatus(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("tenantStatus"), r.URL.Query().Get("tenant_status")))
	if raw == "" {
		return 0, true
	}
	status, err := strconv.Atoi(raw)
	if err != nil || (status != 1 && status != 2) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tenantStatus 必须是 1 或 2", nil)
		return 0, false
	}
	return status, true
}

func saasAdminOverviewDueState(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.ToLower(strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("dueState"), r.URL.Query().Get("due_state"))))
	switch raw {
	case "", SaaSAdminDueStateAll:
		return SaaSAdminDueStateAll, true
	case SaaSAdminDueStateNormal:
		return SaaSAdminDueStateNormal, true
	case SaaSAdminDueStateExpiring, "expiring_soon", "soon":
		return SaaSAdminDueStateExpiring, true
	case SaaSAdminDueStateExpired:
		return SaaSAdminDueStateExpired, true
	case SaaSAdminDueStateNoPackage, "no-package", "none":
		return SaaSAdminDueStateNoPackage, true
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "dueState 必须是 all、normal、expiring、expired 或 no_package", nil)
		return "", false
	}
}

func saasAdminAlertStatus(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	switch raw {
	case "":
		return SaaSAlertStatusOpen, true
	case "all":
		return "", true
	case SaaSAlertStatusOpen, SaaSAlertStatusResolved:
		return raw, true
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "status 必须是 open、resolved 或 all", nil)
		return "", false
	}
}

func saasAdminNotificationStatus(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	switch raw {
	case "":
		return "", true
	case "all":
		return "", true
	case SaaSAlertNotificationStatusPending, SaaSAlertNotificationStatusFailed, SaaSAlertNotificationStatusDelivered, SaaSAlertNotificationStatusDead, SaaSAlertNotificationStatusClosed, SaaSAlertNotificationStatusSuppressed:
		return raw, true
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "status 必须是 pending、failed、delivered、dead、closed、suppressed 或 all", nil)
		return "", false
	}
}

func saasAdminTaskStatus(w http.ResponseWriter, r *http.Request) (string, bool) {
	status, err := normalizeSaaSAdminTaskStatus(r.URL.Query().Get("status"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "status 必须是 pending、blocked、applied、failed、canceled 或 all", nil)
		return "", false
	}
	return status, true
}

func normalizeSaaSAdminTaskStatus(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "":
		return "", nil
	case "all":
		return "", nil
	case SaaSAdminTaskStatusPending, SaaSAdminTaskStatusBlocked, SaaSAdminTaskStatusApplied, SaaSAdminTaskStatusFailed, SaaSAdminTaskStatusCanceled:
		return value, nil
	default:
		return "", errors.New("status 必须是 pending、blocked、applied、failed、canceled 或 all")
	}
}

func validSaaSAdminTaskType(taskType string) bool {
	switch strings.TrimSpace(taskType) {
	case SaaSAdminTaskTypePackageSync, SaaSAdminTaskTypeTenantRenewal, SaaSAdminTaskTypeTenantProvision:
		return true
	default:
		return false
	}
}

func saasAdminExportKind(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.ToLower(strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("type"), r.URL.Query().Get("resource"))))
	switch raw {
	case "", "tenant", "tenants":
		return SaaSAdminExportKindTenants, true
	case "tenantlifecycle", "tenantlifecycles", "tenant_lifecycle", "tenant_lifecycles", "lifecycle", "lifecycles":
		return SaaSAdminExportKindTenantLifecycle, true
	case "alert", "alerts", "saasalert", "saasalerts", "saas_alert", "saas_alerts":
		return SaaSAdminExportKindAlerts, true
	case "notification", "notifications", "alertnotification", "alertnotifications", "alert_notification", "alert_notifications":
		return SaaSAdminExportKindNotifications, true
	case "notificationhealth", "notification_health", "notificationdeliveryhealth", "notification_delivery_health", "deliveryhealth", "delivery_health":
		return SaaSAdminExportKindNotificationHealth, true
	case "notificationslo", "notification_slo", "notificationdeliveryslo", "notification_delivery_slo", "deliveryslo", "delivery_slo":
		return SaaSAdminExportKindNotificationSLO, true
	case "risk", "risks", "risktenant", "risktenants", "risk_tenant", "risk_tenants":
		return SaaSAdminExportKindRisk, true
	case "operationqueue", "operation_queue", "opsqueue", "ops_queue", "adminqueue", "admin_queue":
		return SaaSAdminExportKindOperationQueue, true
	case "operationqueueowner", "operationqueueowners", "operation_queue_owner", "operation_queue_owners", "opsqueueowner", "opsqueueowners", "ops_queue_owner", "ops_queue_owners", "adminqueueowner", "adminqueueowners", "admin_queue_owner", "admin_queue_owners":
		return SaaSAdminExportKindOperationOwners, true
	case "operationqueueassignment", "operationqueueassignments", "operation_queue_assignment", "operation_queue_assignments", "operationqueueassign", "operationqueueassigns", "operation_queue_assign", "operation_queue_assigns", "opsqueueassignment", "opsqueueassignments", "ops_queue_assignment", "ops_queue_assignments", "adminqueueassignment", "adminqueueassignments", "admin_queue_assignment", "admin_queue_assignments":
		return SaaSAdminExportKindOperationAssigns, true
	case "customersuccess", "customer_success", "customer_success_queue", "customersuccessqueue", "customer_successes", "cs":
		return SaaSAdminExportKindCustomerSuccess, true
	case "customersuccessowner", "customersuccessowners", "customer_success_owner", "customer_success_owners", "csowner", "csowners", "cs_owner", "cs_owners":
		return SaaSAdminExportKindCustomerOwners, true
	case "renewalforecast", "renewal_forecast", "renewalforecasts", "renewal_forecasts", "forecast", "forecasts":
		return SaaSAdminExportKindRenewalForecast, true
	case "renewalforecastowner", "renewalforecastowners", "renewal_forecast_owner", "renewal_forecast_owners", "forecastowner", "forecastowners", "forecast_owner", "forecast_owners":
		return SaaSAdminExportKindRenewalOwners, true
	case "riskfollowup", "riskfollowups", "risk_follow_up", "risk_follow_ups", "followup", "followups":
		return SaaSAdminExportKindRiskFollowUps, true
	case "riskfollowupowner", "riskfollowupowners", "risk_follow_up_owner", "risk_follow_up_owners", "riskowners", "risk_owners":
		return SaaSAdminExportKindRiskOwners, true
	case "task", "tasks", "admin_task", "admin_tasks":
		return SaaSAdminExportKindTasks, true
	case "tasksla", "task_sla", "admin_task_sla", "admin_tasks_sla", "sla_tasks", "task_slas":
		return SaaSAdminExportKindTaskSLA, true
	case "daily", "report", "dailyreport", "daily_report":
		return SaaSAdminExportKindDailyReport, true
	case "businessmetrics", "business_metrics", "metrics", "business":
		return SaaSAdminExportKindBusinessMetrics, true
	case "businesstrends", "business_trends", "trends", "trend":
		return SaaSAdminExportKindBusinessTrends, true
	case "usage", "usages", "usagemetrics", "usage_metrics":
		return SaaSAdminExportKindUsage, true
	case "package", "packages", "plans", "plan":
		return SaaSAdminExportKindPackages, true
	case "subscription", "subscriptions", "saas_subscription", "saas_subscriptions":
		return SaaSAdminExportKindSubscriptions, true
	case "paymentorder", "paymentorders", "payment_order", "payment_orders", "collection", "collections":
		return SaaSAdminExportKindPaymentOrders, true
	case "paymentrefund", "paymentrefunds", "payment_refund", "payment_refunds", "refund", "refunds":
		return SaaSAdminExportKindPaymentRefunds, true
	case "paymentsettlement", "paymentsettlements", "payment_settlement", "payment_settlements", "paymentsettlementbatch", "paymentsettlementbatches", "payment_settlement_batch", "payment_settlement_batches", "settlement", "settlements":
		return SaaSAdminExportKindPaymentSettlementBatches, true
	case "paymentsettlemententry", "paymentsettlemententries", "payment_settlement_entry", "payment_settlement_entries", "settlemententry", "settlemententries", "settlement_entry", "settlement_entries":
		return SaaSAdminExportKindPaymentSettlementEntries, true
	case "invoice", "invoices", "invoicedocument", "invoicedocuments", "invoice_document", "invoice_documents", "creditnote", "creditnotes":
		return SaaSAdminExportKindInvoiceDocuments, true
	case "operation", "operations", "operationlogs", "operation_logs":
		return SaaSAdminExportKindOperations, true
	case "billing", "billingevents", "billing_events":
		return SaaSAdminExportKindBillingEvents, true
	case "billingreconciliationfollowups", "billing_reconciliation_followups", "billing_reconciliation_follow_ups", "billingfollowups", "billing_followups", "billing_follow_ups":
		return SaaSAdminExportKindBillingFollowUps, true
	case "billingreconciliationfollowupowners", "billing_reconciliation_followup_owners", "billing_reconciliation_follow_up_owners", "billingfollowupowners", "billing_followup_owners", "billing_follow_up_owners", "billingowners", "billing_owners":
		return SaaSAdminExportKindBillingOwners, true
	case "billingreconciliation", "billing_reconciliation", "billingreconcile", "billing_reconcile", "reconciliation", "reconcile":
		return SaaSAdminExportKindBillingReconcile, true
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "type 必须是 tenants、tenantLifecycle、usage、risk、operationQueue、operationQueueOwners、operationQueueAssignments、businessMetrics、businessTrends、customerSuccess、customerSuccessOwners、renewalForecast、renewalForecastOwners、riskFollowUps、riskFollowUpOwners、alerts、notifications、notificationHealth、notificationSlo、tasks、taskSla、dailyReport、packages、subscriptions、paymentOrders、paymentRefunds、paymentSettlementBatches、paymentSettlementEntries、invoiceDocuments、operations、billingEvents、billingReconciliation、billingReconciliationFollowUps 或 billingReconciliationFollowUpOwners", nil)
		return "", false
	}
}

func writeSaaSAdminTenantCSV(writer *csv.Writer, tenants []SaaSAdminTenantOverview) {
	_ = writer.Write([]string{
		"tenantId",
		"tenantName",
		"tenantStatus",
		"packageCode",
		"packageName",
		"packageStatus",
		"expiresAt",
		"expired",
		"expiringSoon",
		"openAlertCount",
		"maxUsageMetric",
		"maxUsageCurrent",
		"maxUsageLimit",
		"maxUsageRatio",
	})
	for _, item := range tenants {
		_ = writer.Write([]string{
			strconv.Itoa(item.TenantID),
			item.TenantName,
			strconv.Itoa(item.TenantStatus),
			item.PackageCode,
			item.PackageName,
			strconv.Itoa(item.PackageStatus),
			item.ExpiresAt,
			strconv.FormatBool(item.Expired),
			strconv.FormatBool(item.ExpiringSoon),
			strconv.Itoa(item.OpenAlertCount),
			item.MaxUsageMetric,
			strconv.FormatInt(item.MaxUsageCurrent, 10),
			strconv.FormatInt(item.MaxUsageLimit, 10),
			strconv.FormatFloat(item.MaxUsageRatio, 'f', 6, 64),
		})
	}
}

func writeSaaSAdminTenantLifecycleCSV(writer *csv.Writer, lifecycle SaaSAdminTenantLifecycle, events []SaaSAdminTenantLifecycleEvent) {
	_ = writer.Write([]string{
		"tenantId",
		"tenantName",
		"packageCode",
		"packageName",
		"source",
		"eventType",
		"title",
		"status",
		"occurredAt",
		"referenceId",
		"actorUserId",
		"remark",
		"payload",
	})
	tenant := lifecycle.Tenant
	for _, item := range events {
		_ = writer.Write([]string{
			strconv.Itoa(tenant.TenantID),
			tenant.TenantName,
			tenant.PackageCode,
			tenant.PackageName,
			item.Source,
			item.EventType,
			item.Title,
			item.Status,
			item.OccurredAt,
			item.ReferenceID,
			strconv.Itoa(item.ActorUserID),
			item.Remark,
			saasAdminJSONForCSV(item.Payload),
		})
	}
}

func saasAdminJSONForCSV(value any) string {
	if value == nil {
		return ""
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(payload)
}

func writeSaaSAdminRiskCSV(writer *csv.Writer, items []SaaSAdminRiskTenant) {
	_ = writer.Write([]string{
		"tenantId",
		"tenantName",
		"riskLevel",
		"riskScore",
		"riskReasons",
		"suggestedAction",
		"tenantStatus",
		"packageCode",
		"packageName",
		"expiresAt",
		"expired",
		"expiringSoon",
		"openAlertCount",
		"highestUsageMetric",
		"highestUsageRatio",
		"followUpStatus",
		"followUpOwner",
		"nextFollowUpAt",
		"followUpRemark",
		"followUpCreatedAt",
		"topUsageMetrics",
	})
	for _, item := range items {
		_ = writer.Write([]string{
			strconv.Itoa(item.Tenant.TenantID),
			item.Tenant.TenantName,
			item.RiskLevel,
			strconv.Itoa(item.RiskScore),
			strings.Join(item.Reasons, "；"),
			item.SuggestedAction,
			strconv.Itoa(item.Tenant.TenantStatus),
			item.Tenant.PackageCode,
			item.Tenant.PackageName,
			item.Tenant.ExpiresAt,
			strconv.FormatBool(item.Tenant.Expired),
			strconv.FormatBool(item.Tenant.ExpiringSoon),
			strconv.Itoa(item.Tenant.OpenAlertCount),
			item.Tenant.MaxUsageMetric,
			strconv.FormatFloat(item.HighUsageRatio, 'f', 6, 64),
			item.FollowUp.Status,
			item.FollowUp.Owner,
			item.FollowUp.NextFollowUpAt,
			item.FollowUp.Remark,
			item.FollowUp.CreatedAt,
			saasAdminRiskMetricCSV(item.TopUsageMetrics),
		})
	}
}

func writeSaaSAdminOperationQueueCSV(writer *csv.Writer, items []SaaSAdminOperationQueueItem) {
	_ = writer.Write([]string{
		"source",
		"priority",
		"tenantId",
		"tenantName",
		"owner",
		"title",
		"reason",
		"nextAction",
		"dueState",
		"status",
		"objectType",
		"objectId",
		"ageHours",
		"createdAt",
		"updatedAt",
		"remark",
	})
	for _, item := range items {
		_ = writer.Write([]string{
			item.Source,
			item.Priority,
			strconv.Itoa(item.TenantID),
			item.TenantName,
			item.Owner,
			item.Title,
			item.Reason,
			item.NextAction,
			item.DueState,
			item.Status,
			item.ObjectType,
			item.ObjectID,
			strconv.Itoa(item.AgeHours),
			item.CreatedAt,
			item.UpdatedAt,
			item.Remark,
		})
	}
}

func writeSaaSAdminOperationQueueOwnerCSV(writer *csv.Writer, owners []SaaSAdminOperationQueueOwnerSummary) {
	_ = writer.Write([]string{
		"owner",
		"queueCount",
		"tenantCount",
		"sourceCount",
		"criticalCount",
		"highCount",
		"mediumCount",
		"normalCount",
		"customerSuccessCount",
		"taskSlaCount",
		"billingFollowUpCount",
		"notificationCount",
		"closedNotificationCount",
		"notificationHealthCount",
		"unassignedCount",
		"maxAgeHours",
		"topTenants",
		"topItems",
	})
	for _, owner := range owners {
		_ = writer.Write([]string{
			owner.Owner,
			strconv.Itoa(owner.QueueCount),
			strconv.Itoa(owner.TenantCount),
			strconv.Itoa(owner.SourceCount),
			strconv.Itoa(owner.CriticalCount),
			strconv.Itoa(owner.HighCount),
			strconv.Itoa(owner.MediumCount),
			strconv.Itoa(owner.NormalCount),
			strconv.Itoa(owner.CustomerSuccessCount),
			strconv.Itoa(owner.TaskSLACount),
			strconv.Itoa(owner.BillingFollowUpCount),
			strconv.Itoa(owner.NotificationCount),
			strconv.Itoa(owner.ClosedNotificationCount),
			strconv.Itoa(owner.NotificationHealthCount),
			strconv.Itoa(owner.UnassignedCount),
			strconv.Itoa(owner.MaxAgeHours),
			saasAdminOperationQueueOwnerTenantCSV(owner.TopTenants),
			saasAdminOperationQueueOwnerTopItemCSV(owner.TopItems),
		})
	}
}

func writeSaaSAdminOperationQueueAssignmentCSV(writer *csv.Writer, assignments []SaaSAdminOperationQueueAssignment) {
	_ = writer.Write([]string{
		"operationId",
		"tenantId",
		"source",
		"objectType",
		"objectId",
		"targetName",
		"owner",
		"status",
		"dueState",
		"nextFollowUpAt",
		"remark",
		"actorUserId",
		"actorTenantId",
		"assignedAt",
	})
	for _, item := range assignments {
		_ = writer.Write([]string{
			strconv.FormatInt(item.OperationID, 10),
			strconv.Itoa(item.TenantID),
			item.Source,
			item.ObjectType,
			item.ObjectID,
			item.TargetName,
			item.Owner,
			item.Status,
			item.DueState,
			item.NextFollowUpAt,
			item.Remark,
			strconv.Itoa(item.ActorUserID),
			strconv.Itoa(item.ActorTenantID),
			item.AssignedAt,
		})
	}
}

func writeSaaSAdminCustomerSuccessCSV(writer *csv.Writer, items []SaaSAdminCustomerSuccessQueueItem) {
	_ = writer.Write([]string{
		"tenantId",
		"tenantName",
		"priority",
		"healthScore",
		"owner",
		"dueState",
		"reasons",
		"nextAction",
		"riskLevel",
		"riskScore",
		"suggestedAction",
		"tenantStatus",
		"packageCode",
		"packageName",
		"expiresAt",
		"expired",
		"expiringSoon",
		"openAlertCount",
		"highestUsageMetric",
		"highestUsageRatio",
		"riskFollowUpStatus",
		"riskFollowUpOwner",
		"riskNextFollowUpAt",
		"riskFollowUpRemark",
		"billingFollowUpCount",
		"actionableTaskCount",
		"retryableNotificationCount",
		"failedNotificationCount",
		"deadNotificationCount",
		"topUsageMetrics",
	})
	for _, item := range items {
		_ = writer.Write([]string{
			strconv.Itoa(item.Tenant.TenantID),
			item.Tenant.TenantName,
			item.Priority,
			strconv.Itoa(item.HealthScore),
			item.Owner,
			item.DueState,
			strings.Join(item.Reasons, "；"),
			item.NextAction,
			item.Risk.RiskLevel,
			strconv.Itoa(item.Risk.RiskScore),
			item.Risk.SuggestedAction,
			strconv.Itoa(item.Tenant.TenantStatus),
			item.Tenant.PackageCode,
			item.Tenant.PackageName,
			item.Tenant.ExpiresAt,
			strconv.FormatBool(item.Tenant.Expired),
			strconv.FormatBool(item.Tenant.ExpiringSoon),
			strconv.Itoa(item.Tenant.OpenAlertCount),
			item.Tenant.MaxUsageMetric,
			strconv.FormatFloat(item.Risk.HighUsageRatio, 'f', 6, 64),
			item.RiskFollowUp.Status,
			item.RiskFollowUp.Owner,
			item.RiskFollowUp.NextFollowUpAt,
			item.RiskFollowUp.Remark,
			strconv.Itoa(item.BillingFollowUpCount),
			strconv.Itoa(item.AdminTaskSummary.ActionableCount),
			strconv.Itoa(item.RetryableNotificationCount),
			strconv.Itoa(item.FailedNotificationCount),
			strconv.Itoa(item.DeadNotificationCount),
			saasAdminRiskMetricCSV(item.Risk.TopUsageMetrics),
		})
	}
}

func writeSaaSAdminCustomerSuccessOwnerCSV(writer *csv.Writer, owners []SaaSAdminCustomerSuccessOwnerSummary) {
	_ = writer.Write([]string{
		"owner",
		"tenantCount",
		"criticalCount",
		"highCount",
		"mediumCount",
		"normalCount",
		"overdueCount",
		"dueSoonCount",
		"blockedCount",
		"billingFollowUpCount",
		"actionableTaskCount",
		"retryableNotificationCount",
		"failedNotificationCount",
		"deadNotificationCount",
		"maxHealthScore",
		"averageHealthScore",
		"nextFollowUpAt",
		"topTenants",
	})
	for _, owner := range owners {
		topTenants := make([]string, 0, len(owner.TopTenants))
		for _, tenant := range owner.TopTenants {
			topTenants = append(topTenants, tenant.Tenant.TenantName+"#"+strconv.Itoa(tenant.Tenant.TenantID)+"("+tenant.Priority+")")
		}
		_ = writer.Write([]string{
			owner.Owner,
			strconv.Itoa(owner.TenantCount),
			strconv.Itoa(owner.CriticalCount),
			strconv.Itoa(owner.HighCount),
			strconv.Itoa(owner.MediumCount),
			strconv.Itoa(owner.NormalCount),
			strconv.Itoa(owner.OverdueCount),
			strconv.Itoa(owner.DueSoonCount),
			strconv.Itoa(owner.BlockedCount),
			strconv.Itoa(owner.BillingFollowUpCount),
			strconv.Itoa(owner.ActionableTaskCount),
			strconv.Itoa(owner.RetryableNotificationCount),
			strconv.Itoa(owner.FailedNotificationCount),
			strconv.Itoa(owner.DeadNotificationCount),
			strconv.Itoa(owner.MaxHealthScore),
			strconv.Itoa(owner.AverageHealthScore),
			owner.NextFollowUpAt,
			strings.Join(topTenants, "；"),
		})
	}
}

func writeSaaSAdminRenewalForecastCSV(writer *csv.Writer, items []SaaSAdminRenewalForecastTenant) {
	_ = writer.Write([]string{
		"tenantId",
		"tenantName",
		"tenantStatus",
		"packageCode",
		"packageName",
		"expiresAt",
		"bucket",
		"bucketLabel",
		"daysUntil",
		"renewalAmountCents",
		"estimatedMrrCents",
		"priced",
		"latestBillingEventId",
		"latestBillingAt",
		"taskCount",
		"actionableTaskCount",
		"pendingTaskCount",
		"blockedTaskCount",
		"failedTaskCount",
		"appliedTaskCount",
		"latestTaskId",
		"latestTaskStatus",
		"latestTaskCreatedAt",
		"owner",
		"riskFollowUpStatus",
		"riskFollowUpNextAt",
	})
	for _, item := range items {
		latestTaskID := ""
		latestTaskStatus := ""
		latestTaskCreatedAt := ""
		if item.HasLatestTask {
			latestTaskID = strconv.FormatInt(item.LatestTask.ID, 10)
			latestTaskStatus = item.LatestTask.Status
			latestTaskCreatedAt = item.LatestTask.CreatedAt
		}
		riskFollowUpStatus := ""
		riskFollowUpNextAt := ""
		if item.HasRiskFollowUp {
			riskFollowUpStatus = item.RiskFollowUp.Status
			riskFollowUpNextAt = item.RiskFollowUp.NextFollowUpAt
		}
		_ = writer.Write([]string{
			strconv.Itoa(item.Tenant.TenantID),
			item.Tenant.TenantName,
			strconv.Itoa(item.Tenant.TenantStatus),
			item.Tenant.PackageCode,
			item.Tenant.PackageName,
			item.Tenant.ExpiresAt,
			item.Bucket,
			item.BucketLabel,
			strconv.Itoa(item.DaysUntil),
			strconv.FormatInt(item.RenewalAmountCents, 10),
			strconv.FormatInt(item.EstimatedMRRCents, 10),
			strconv.FormatBool(item.Priced),
			strconv.FormatInt(item.LatestBillingEventID, 10),
			item.LatestBillingAt,
			strconv.Itoa(item.TaskSummary.TaskCount),
			strconv.Itoa(item.TaskSummary.ActionableCount),
			strconv.Itoa(item.TaskSummary.PendingCount),
			strconv.Itoa(item.TaskSummary.BlockedCount),
			strconv.Itoa(item.TaskSummary.FailedCount),
			strconv.Itoa(item.TaskSummary.AppliedCount),
			latestTaskID,
			latestTaskStatus,
			latestTaskCreatedAt,
			item.Owner,
			riskFollowUpStatus,
			riskFollowUpNextAt,
		})
	}
}

func writeSaaSAdminRenewalForecastOwnerCSV(writer *csv.Writer, owners []SaaSAdminRenewalForecastOwnerSummary) {
	_ = writer.Write([]string{
		"owner",
		"tenantCount",
		"pricedTenantCount",
		"unknownPriceTenantCount",
		"renewalAmountCents",
		"estimatedMrrCents",
		"expiredTenantCount",
		"dueWithin30TenantCount",
		"due31To60TenantCount",
		"due61To90TenantCount",
		"dueLaterTenantCount",
		"actionableTaskCount",
		"pendingTaskCount",
		"blockedTaskCount",
		"failedTaskCount",
		"appliedTaskCount",
		"canceledTaskCount",
		"nextFollowUpAt",
		"topTenants",
	})
	for _, owner := range owners {
		topTenants := make([]string, 0, len(owner.TopTenants))
		for _, tenant := range owner.TopTenants {
			price := "unknown"
			if tenant.Priced {
				price = strconv.FormatInt(tenant.RenewalAmountCents, 10)
			}
			topTenants = append(topTenants, tenant.Tenant.TenantName+"#"+strconv.Itoa(tenant.Tenant.TenantID)+"("+tenant.Bucket+","+price+")")
		}
		_ = writer.Write([]string{
			owner.Owner,
			strconv.Itoa(owner.TenantCount),
			strconv.Itoa(owner.PricedTenantCount),
			strconv.Itoa(owner.UnknownPriceTenantCount),
			strconv.FormatInt(owner.RenewalAmountCents, 10),
			strconv.FormatInt(owner.EstimatedMRRCents, 10),
			strconv.Itoa(owner.ExpiredTenantCount),
			strconv.Itoa(owner.DueWithin30TenantCount),
			strconv.Itoa(owner.Due31To60TenantCount),
			strconv.Itoa(owner.Due61To90TenantCount),
			strconv.Itoa(owner.DueLaterTenantCount),
			strconv.Itoa(owner.ActionableTaskCount),
			strconv.Itoa(owner.PendingTaskCount),
			strconv.Itoa(owner.BlockedTaskCount),
			strconv.Itoa(owner.FailedTaskCount),
			strconv.Itoa(owner.AppliedTaskCount),
			strconv.Itoa(owner.CanceledTaskCount),
			owner.NextFollowUpAt,
			strings.Join(topTenants, "；"),
		})
	}
}

func writeSaaSAdminUsageCSV(writer *csv.Writer, tenants []SaaSAdminTenantOverview, usageByTenant map[int][]SaaSAdminUsageMetric) {
	_ = writer.Write([]string{
		"tenantId",
		"tenantName",
		"tenantStatus",
		"packageCode",
		"packageName",
		"expiresAt",
		"metric",
		"metricLabel",
		"periodKey",
		"current",
		"usageLimit",
		"remaining",
		"unlimited",
		"usageRatio",
		"status",
		"openAlertCount",
		"updatedBy",
		"updatedAt",
	})
	for _, tenant := range tenants {
		for _, metric := range usageByTenant[tenant.TenantID] {
			_ = writer.Write([]string{
				strconv.Itoa(tenant.TenantID),
				tenant.TenantName,
				strconv.Itoa(tenant.TenantStatus),
				tenant.PackageCode,
				tenant.PackageName,
				tenant.ExpiresAt,
				metric.Metric,
				saasMetricLabel(metric.Metric),
				metric.PeriodKey,
				strconv.FormatInt(metric.Current, 10),
				strconv.FormatInt(metric.Limit, 10),
				strconv.FormatInt(metric.Remaining, 10),
				strconv.FormatBool(metric.Unlimited),
				strconv.FormatFloat(metric.UsageRatio, 'f', 6, 64),
				metric.Status,
				strconv.Itoa(metric.OpenAlertCount),
				metric.UpdatedBy,
				metric.UpdatedAt,
			})
		}
	}
}

func writeSaaSAdminPackageCSV(writer *csv.Writer, packages []SaaSAdminPackage) {
	_ = writer.Write([]string{
		"code",
		"name",
		"description",
		"status",
		"maxCorps",
		"maxUsers",
		"maxContacts",
		"maxRooms",
		"maxAgents",
		"channelCodes",
		"shopCodes",
		"radars",
		"lotteries",
		"roomInfinitePulls",
		"roomFissions",
		"roomClockIns",
		"roomQualities",
		"roomCalendars",
		"roomReminds",
		"contactSops",
		"roomSops",
		"sensitiveWords",
		"storageMb",
		"contactMessageBatches",
		"roomMessageBatches",
		"roomTagPulls",
		"workRoomAutoPulls",
		"workFissions",
		"officialAccounts",
		"asyncExecutions",
	})
	for _, item := range packages {
		limits := item.Limits
		_ = writer.Write([]string{
			item.Code,
			item.Name,
			item.Description,
			strconv.Itoa(item.Status),
			strconv.FormatInt(limits.MaxCorps, 10),
			strconv.FormatInt(limits.MaxUsers, 10),
			strconv.FormatInt(limits.MaxContacts, 10),
			strconv.FormatInt(limits.MaxRooms, 10),
			strconv.FormatInt(limits.MaxAgents, 10),
			strconv.FormatInt(limits.ChannelCodes, 10),
			strconv.FormatInt(limits.ShopCodes, 10),
			strconv.FormatInt(limits.Radars, 10),
			strconv.FormatInt(limits.Lotteries, 10),
			strconv.FormatInt(limits.RoomInfinitePulls, 10),
			strconv.FormatInt(limits.RoomFissions, 10),
			strconv.FormatInt(limits.RoomClockIns, 10),
			strconv.FormatInt(limits.RoomQualities, 10),
			strconv.FormatInt(limits.RoomCalendars, 10),
			strconv.FormatInt(limits.RoomReminds, 10),
			strconv.FormatInt(limits.ContactSOPs, 10),
			strconv.FormatInt(limits.RoomSOPs, 10),
			strconv.FormatInt(limits.SensitiveWords, 10),
			strconv.FormatInt(limits.StorageMB, 10),
			strconv.FormatInt(limits.ContactMessageBatches, 10),
			strconv.FormatInt(limits.RoomMessageBatches, 10),
			strconv.FormatInt(limits.RoomTagPulls, 10),
			strconv.FormatInt(limits.WorkRoomAutoPulls, 10),
			strconv.FormatInt(limits.WorkFissions, 10),
			strconv.FormatInt(limits.OfficialAccounts, 10),
			strconv.FormatInt(limits.AsyncExecutions, 10),
		})
	}
}

func saasAdminRiskMetricCSV(metrics []SaaSAdminUsageMetric) string {
	parts := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		limit := "unlimited"
		if metric.Limit > 0 {
			limit = strconv.FormatInt(metric.Limit, 10)
		}
		parts = append(parts, metric.Metric+"="+strconv.FormatInt(metric.Current, 10)+"/"+limit+" ratio="+strconv.FormatFloat(metric.UsageRatio, 'f', 6, 64)+" status="+metric.Status+" alerts="+strconv.Itoa(metric.OpenAlertCount))
	}
	return strings.Join(parts, " | ")
}

func writeSaaSAdminRiskFollowUpCSV(writer *csv.Writer, tasks []SaaSAdminRiskFollowUpTask) {
	_ = writer.Write([]string{
		"operationId",
		"tenantId",
		"tenantName",
		"status",
		"owner",
		"nextFollowUpAt",
		"dueState",
		"overdue",
		"daysUntil",
		"remark",
		"createdAt",
	})
	for _, item := range tasks {
		_ = writer.Write([]string{
			strconv.FormatInt(item.OperationID, 10),
			strconv.Itoa(item.TenantID),
			item.TenantName,
			item.Status,
			item.Owner,
			item.NextFollowUpAt,
			item.DueState,
			strconv.FormatBool(item.Overdue),
			strconv.Itoa(item.DaysUntil),
			item.Remark,
			item.CreatedAt,
		})
	}
}

func writeSaaSAdminFollowUpOwnerCSV(writer *csv.Writer, owners []SaaSAdminRiskFollowUpOwnerSummary) {
	_ = writer.Write([]string{
		"owner",
		"totalCount",
		"openCount",
		"pendingCount",
		"contactedCount",
		"renewalPendingCount",
		"resolvedCount",
		"ignoredCount",
		"overdueCount",
		"dueSoonCount",
		"futureCount",
		"noDateCount",
		"closedCount",
		"latestFollowUpAt",
		"nextFollowUpAt",
	})
	for _, owner := range owners {
		_ = writer.Write([]string{
			owner.Owner,
			strconv.Itoa(owner.TotalCount),
			strconv.Itoa(owner.OpenCount),
			strconv.Itoa(owner.PendingCount),
			strconv.Itoa(owner.ContactedCount),
			strconv.Itoa(owner.RenewalPendingCount),
			strconv.Itoa(owner.ResolvedCount),
			strconv.Itoa(owner.IgnoredCount),
			strconv.Itoa(owner.OverdueCount),
			strconv.Itoa(owner.DueSoonCount),
			strconv.Itoa(owner.FutureCount),
			strconv.Itoa(owner.NoDateCount),
			strconv.Itoa(owner.ClosedCount),
			owner.LatestFollowUpAt,
			owner.NextFollowUpAt,
		})
	}
}

func writeSaaSAdminBillingReconciliationFollowUpCSV(writer *csv.Writer, tasks []SaaSAdminBillingReconciliationFollowUpTask) {
	_ = writer.Write([]string{
		"operationId",
		"billingEventId",
		"tenantId",
		"tenantName",
		"status",
		"owner",
		"nextFollowUpAt",
		"dueState",
		"overdue",
		"daysUntil",
		"remark",
		"packageCode",
		"packageName",
		"newExpiresAt",
		"amountCents",
		"currency",
		"externalOrderNo",
		"createdAt",
	})
	for _, item := range tasks {
		_ = writer.Write([]string{
			strconv.FormatInt(item.OperationID, 10),
			strconv.FormatInt(item.BillingEventID, 10),
			strconv.Itoa(item.TenantID),
			item.TenantName,
			item.Status,
			item.Owner,
			item.NextFollowUpAt,
			item.DueState,
			strconv.FormatBool(item.Overdue),
			strconv.Itoa(item.DaysUntil),
			item.Remark,
			item.PackageCode,
			item.PackageName,
			item.NewExpiresAt,
			strconv.FormatInt(item.AmountCents, 10),
			item.Currency,
			item.ExternalOrderNo,
			item.CreatedAt,
		})
	}
}

func writeSaaSAdminTaskCSV(writer *csv.Writer, tasks []SaaSAdminTask) {
	_ = writer.Write([]string{
		"id",
		"taskType",
		"status",
		"tenantId",
		"packageCode",
		"actorUserId",
		"actorTenantId",
		"remark",
		"lastError",
		"appliedAt",
		"createdAt",
		"updatedAt",
		"request",
		"preview",
		"result",
	})
	for _, item := range tasks {
		_ = writer.Write([]string{
			strconv.FormatInt(item.ID, 10),
			item.TaskType,
			item.Status,
			strconv.Itoa(item.TenantID),
			item.PackageCode,
			strconv.Itoa(item.ActorUserID),
			strconv.Itoa(item.ActorTenantID),
			item.Remark,
			item.LastError,
			item.AppliedAt,
			item.CreatedAt,
			item.UpdatedAt,
			saasAdminCSVJSONField(saasAdminTaskRequestPayload(item)),
			saasAdminCSVJSONField(saasAdminJSONPayload(item.PreviewJSON)),
			saasAdminCSVJSONField(saasAdminJSONPayload(item.ResultJSON)),
		})
	}
}

func writeSaaSAdminTaskSLACSV(writer *csv.Writer, tasks []SaaSAdminTaskSLAItem) {
	_ = writer.Write([]string{
		"taskId",
		"taskType",
		"status",
		"tenantId",
		"packageCode",
		"owner",
		"actorUserId",
		"actorTenantId",
		"slaStatus",
		"ageHours",
		"breachHours",
		"remark",
		"lastError",
		"appliedAt",
		"createdAt",
		"updatedAt",
		"request",
		"preview",
		"result",
	})
	for _, item := range tasks {
		task := item.Task
		_ = writer.Write([]string{
			strconv.FormatInt(task.ID, 10),
			task.TaskType,
			task.Status,
			strconv.Itoa(task.TenantID),
			task.PackageCode,
			item.Owner,
			strconv.Itoa(task.ActorUserID),
			strconv.Itoa(task.ActorTenantID),
			item.SLAStatus,
			strconv.Itoa(item.AgeHours),
			strconv.Itoa(item.BreachHours),
			task.Remark,
			task.LastError,
			task.AppliedAt,
			task.CreatedAt,
			task.UpdatedAt,
			saasAdminCSVJSONField(saasAdminTaskRequestPayload(task)),
			saasAdminCSVJSONField(saasAdminJSONPayload(task.PreviewJSON)),
			saasAdminCSVJSONField(saasAdminJSONPayload(task.ResultJSON)),
		})
	}
}

func writeSaaSAdminAlertCSV(writer *csv.Writer, alerts []SaaSAlertRecord) {
	_ = writer.Write([]string{
		"id",
		"alertKey",
		"tenantId",
		"alertType",
		"severity",
		"status",
		"metric",
		"metricLabel",
		"periodKey",
		"currentValue",
		"limitValue",
		"additionalValue",
		"occurrenceCount",
		"source",
		"message",
		"context",
		"firstSeenAt",
		"lastSeenAt",
		"resolvedAt",
		"createdAt",
		"updatedAt",
	})
	for _, item := range alerts {
		_ = writer.Write([]string{
			strconv.FormatInt(item.ID, 10),
			item.AlertKey,
			strconv.Itoa(item.TenantID),
			item.AlertType,
			item.Severity,
			item.Status,
			item.Metric,
			saasMetricLabel(item.Metric),
			item.PeriodKey,
			strconv.FormatInt(item.CurrentValue, 10),
			strconv.FormatInt(item.LimitValue, 10),
			strconv.FormatInt(item.AdditionalValue, 10),
			strconv.FormatInt(item.OccurrenceCount, 10),
			item.Source,
			item.Message,
			saasAdminCSVJSONField(saasAdminJSONPayload(item.ContextJSON)),
			item.FirstSeenAt,
			item.LastSeenAt,
			item.ResolvedAt,
			item.CreatedAt,
			item.UpdatedAt,
		})
	}
}

func writeSaaSAdminAlertNotificationCSV(writer *csv.Writer, notifications []SaaSAlertNotification) {
	_ = writer.Write([]string{
		"id",
		"notificationKey",
		"alertKey",
		"tenantId",
		"channel",
		"status",
		"attempts",
		"maxAttempts",
		"metric",
		"metricLabel",
		"alertType",
		"periodKey",
		"currentValue",
		"limitValue",
		"message",
		"lastError",
		"nextRetryAt",
		"deliveredAt",
		"createdAt",
		"updatedAt",
	})
	for _, item := range notifications {
		metric := item.Alert.Status.Metric
		alertType := item.Alert.AlertType
		periodKey := item.Alert.PeriodKey
		if alertType == "" {
			alertType = SaaSAlertTypeQuotaExceeded
		}
		if periodKey == "" {
			periodKey = SaaSAlertPeriodLifetime
		}
		_ = writer.Write([]string{
			strconv.FormatInt(item.ID, 10),
			item.NotificationKey,
			item.AlertKey,
			strconv.Itoa(item.TenantID),
			item.Channel,
			item.Status,
			strconv.Itoa(item.Attempts),
			strconv.Itoa(item.MaxAttempts),
			metric,
			saasMetricLabel(metric),
			alertType,
			periodKey,
			strconv.FormatInt(item.Alert.Status.Current, 10),
			strconv.FormatInt(item.Alert.Status.Limit, 10),
			item.Alert.Message,
			item.LastError,
			item.NextRetryAt,
			item.DeliveredAt,
			item.CreatedAt,
			item.UpdatedAt,
		})
	}
}

func writeSaaSAdminDailyReportCSV(writer *csv.Writer, report SaaSAdminDailyReportData) {
	_ = writer.Write([]string{"section", "metric", "value", "remark"})
	options := report.Options
	writeSaaSAdminDailyReportCSVRow(writer, "window", "date", options.ReportDate, "")
	writeSaaSAdminDailyReportCSVRow(writer, "window", "days", strconv.Itoa(options.Days), "")
	writeSaaSAdminDailyReportCSVRow(writer, "window", "startAt", options.WindowStart.Format("2006-01-02 15:04:05"), "")
	writeSaaSAdminDailyReportCSVRow(writer, "window", "endAt", options.WindowEnd.Format("2006-01-02 15:04:05"), "")
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "expiringDays", strconv.Itoa(options.ExpiringDays), "")
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "highUsageRatio", strconv.FormatFloat(options.HighUsageRatio, 'f', 6, 64), "")
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "tenantLimit", strconv.Itoa(options.TenantLimit), "")
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "limit", strconv.Itoa(options.ItemLimit), "")

	summary := report.Summary
	summaryRows := []struct {
		metric string
		value  string
	}{
		{"tenantCount", strconv.Itoa(summary.TenantCount)},
		{"activeTenantPackageCount", strconv.Itoa(summary.ActiveTenantPackageCount)},
		{"userCount", strconv.Itoa(summary.UserCount)},
		{"corpCount", strconv.Itoa(summary.CorpCount)},
		{"expiringSoonTenantCount", strconv.Itoa(summary.ExpiringSoonTenantCount)},
		{"expiredTenantCount", strconv.Itoa(summary.ExpiredTenantCount)},
		{"evaluatedRiskTenantCount", strconv.Itoa(summary.EvaluatedRiskTenantCount)},
		{"riskTenantCount", strconv.Itoa(summary.RiskTenantCount)},
		{"criticalRiskTenantCount", strconv.Itoa(summary.CriticalRiskTenantCount)},
		{"highRiskTenantCount", strconv.Itoa(summary.HighRiskTenantCount)},
		{"openRiskFollowUpCount", strconv.Itoa(summary.OpenRiskFollowUpCount)},
		{"overdueRiskFollowUpCount", strconv.Itoa(summary.OverdueRiskFollowUpCount)},
		{"dueSoonRiskFollowUpCount", strconv.Itoa(summary.DueSoonRiskFollowUpCount)},
		{"riskFollowUpOwnerCount", strconv.Itoa(summary.RiskFollowUpOwnerCount)},
		{"openAlertCount", strconv.Itoa(summary.OpenAlertCount)},
		{"taskSlaActiveCount", strconv.Itoa(summary.TaskSLAActiveCount)},
		{"taskSlaWarningCount", strconv.Itoa(summary.TaskSLAWarningCount)},
		{"taskSlaOverdueCount", strconv.Itoa(summary.TaskSLAOverdueCount)},
		{"taskSlaOwnerCount", strconv.Itoa(summary.TaskSLAOwnerCount)},
		{"taskSlaMaxAgeHours", strconv.Itoa(summary.TaskSLAMaxAgeHours)},
		{"pendingNotificationCount", strconv.Itoa(summary.PendingNotificationCount)},
		{"failedNotificationCount", strconv.Itoa(summary.FailedNotificationCount)},
		{"deadNotificationCount", strconv.Itoa(summary.DeadNotificationCount)},
		{"closedNotificationCount", strconv.Itoa(summary.ClosedNotificationCount)},
		{"retryableNotificationCount", strconv.Itoa(summary.RetryableNotificationCount)},
		{"windowQueueAssignmentCount", strconv.Itoa(summary.WindowQueueAssignmentCount)},
		{"windowTaskSlaAssignCount", strconv.Itoa(summary.WindowTaskSLAAssignCount)},
		{"windowNotificationAssignCount", strconv.Itoa(summary.WindowNotificationAssignCount)},
		{"windowClosedNotificationAssignCount", strconv.Itoa(summary.WindowClosedNotificationAssignCount)},
		{"windowNotificationHealthAssignCount", strconv.Itoa(summary.WindowNotificationHealthAssignCount)},
		{"windowOperationCount", strconv.Itoa(summary.WindowOperationCount)},
		{"windowBillingEventCount", strconv.Itoa(summary.WindowBillingEventCount)},
		{"windowBillingAmountCents", strconv.FormatInt(summary.WindowBillingAmountCents, 10)},
		{"windowRenewalCount", strconv.Itoa(summary.WindowRenewalCount)},
		{"windowRefundCount", strconv.Itoa(summary.WindowRefundCount)},
		{"windowGrossBillingAmountCents", strconv.FormatInt(summary.WindowGrossBillingAmountCents, 10)},
		{"windowRefundAmountCents", strconv.FormatInt(summary.WindowRefundAmountCents, 10)},
		{"windowRiskFollowUpCount", strconv.Itoa(summary.WindowRiskFollowUpCount)},
		{"windowAlertResolveCount", strconv.Itoa(summary.WindowAlertResolveCount)},
		{"windowNotificationRetryCount", strconv.Itoa(summary.WindowNotificationRetryCount)},
		{"windowNotificationCloseCount", strconv.Itoa(summary.WindowNotificationCloseCount)},
	}
	for _, row := range summaryRows {
		writeSaaSAdminDailyReportCSVRow(writer, "summary", row.metric, row.value, "")
	}

	for _, owner := range saasAdminLimitRiskFollowUpOwnerSummaries(report.RiskOwners, options.ItemLimit) {
		remark := "total=" + strconv.Itoa(owner.TotalCount) +
			" overdue=" + strconv.Itoa(owner.OverdueCount) +
			" dueSoon=" + strconv.Itoa(owner.DueSoonCount) +
			" closed=" + strconv.Itoa(owner.ClosedCount) +
			" nextFollowUpAt=" + owner.NextFollowUpAt
		writeSaaSAdminDailyReportCSVRow(writer, "riskOwner", owner.Owner, strconv.Itoa(owner.OpenCount), remark)
	}
	for _, tenant := range saasAdminLimitRiskTenants(report.RiskReport.Items, options.ItemLimit) {
		remark := "tenantId=" + strconv.Itoa(tenant.Tenant.TenantID) +
			" riskScore=" + strconv.Itoa(tenant.RiskScore) +
			" reasons=" + strings.Join(tenant.Reasons, "；")
		writeSaaSAdminDailyReportCSVRow(writer, "riskTenant", tenant.Tenant.TenantName, tenant.RiskLevel, remark)
	}
	for _, owner := range report.TaskSLAReport.Owners {
		remark := "warning=" + strconv.Itoa(owner.Summary.WarningCount) +
			" overdue=" + strconv.Itoa(owner.Summary.OverdueCount) +
			" blocked=" + strconv.Itoa(owner.Summary.BlockedCount) +
			" failed=" + strconv.Itoa(owner.Summary.FailedCount) +
			" maxAgeHours=" + strconv.Itoa(owner.Summary.MaxAgeHours) +
			" oldestTaskAt=" + owner.OldestTaskAt
		writeSaaSAdminDailyReportCSVRow(writer, "taskSlaOwner", owner.Owner, strconv.Itoa(owner.Summary.TaskCount), remark)
	}
	for _, item := range report.TaskSLAReport.Tasks {
		task := item.Task
		metric := strings.TrimSpace(task.TaskType + " " + item.SLAStatus)
		remark := "taskId=" + strconv.FormatInt(task.ID, 10) +
			" tenantId=" + strconv.Itoa(task.TenantID) +
			" packageCode=" + task.PackageCode +
			" owner=" + item.Owner +
			" status=" + task.Status +
			" ageHours=" + strconv.Itoa(item.AgeHours) +
			" breachHours=" + strconv.Itoa(item.BreachHours) +
			" lastError=" + task.LastError +
			" remark=" + task.Remark
		writeSaaSAdminDailyReportCSVRow(writer, "taskSlaTask", metric, strconv.FormatInt(task.ID, 10), remark)
	}
	for _, notification := range saasAdminLimitNotifications(report.RetryableNotifications, options.ItemLimit) {
		metric := strings.TrimSpace(notification.Status + " " + notification.NotificationKey)
		remark := "channel=" + notification.Channel +
			" attempts=" + strconv.Itoa(notification.Attempts) + "/" + strconv.Itoa(notification.MaxAttempts) +
			" lastError=" + notification.LastError
		writeSaaSAdminDailyReportCSVRow(writer, "notification", metric, strconv.Itoa(notification.TenantID), remark)
	}
	for _, notification := range saasAdminLimitNotifications(report.ClosedNotifications, options.ItemLimit) {
		metric := strings.TrimSpace(notification.Status + " " + notification.NotificationKey)
		remark := "channel=" + notification.Channel +
			" attempts=" + strconv.Itoa(notification.Attempts) + "/" + strconv.Itoa(notification.MaxAttempts) +
			" lastError=" + notification.LastError +
			" updatedAt=" + notification.UpdatedAt
		writeSaaSAdminDailyReportCSVRow(writer, "closedNotification", metric, strconv.Itoa(notification.TenantID), remark)
	}
	for _, assignment := range report.QueueAssignmentReport.Assignments {
		metric := strings.TrimSpace(assignment.Source + " " + assignment.Status)
		remark := "operationId=" + strconv.FormatInt(assignment.OperationID, 10) +
			" tenantId=" + strconv.Itoa(assignment.TenantID) +
			" objectType=" + assignment.ObjectType +
			" objectId=" + assignment.ObjectID +
			" targetName=" + assignment.TargetName +
			" owner=" + assignment.Owner +
			" dueState=" + assignment.DueState +
			" nextFollowUpAt=" + assignment.NextFollowUpAt +
			" assignedAt=" + assignment.AssignedAt +
			" remark=" + assignment.Remark
		writeSaaSAdminDailyReportCSVRow(writer, "operationQueueAssignment", metric, assignment.Owner, remark)
	}
	for _, action := range report.OperationActionSummary {
		writeSaaSAdminDailyReportCSVRow(writer, "operationAction", action.Action, strconv.Itoa(action.Count), "")
	}
	for _, operation := range saasAdminLimitOperationLogs(report.WindowOperations, options.ItemLimit) {
		remark := "id=" + strconv.FormatInt(operation.ID, 10) +
			" targetType=" + operation.TargetType +
			" targetId=" + operation.TargetID +
			" createdAt=" + operation.CreatedAt +
			" remark=" + operation.Remark
		writeSaaSAdminDailyReportCSVRow(writer, "operation", operation.Action, operation.TargetName, remark)
	}
	for _, event := range saasAdminLimitBillingEvents(report.WindowBillingEvents, options.ItemLimit) {
		metric := strings.TrimSpace(event.EventType + " " + event.PackageCode)
		remark := "id=" + strconv.FormatInt(event.ID, 10) +
			" tenantId=" + strconv.Itoa(event.TenantID) +
			" packageName=" + event.PackageName +
			" currency=" + event.Currency +
			" externalOrderNo=" + event.ExternalOrderNo +
			" createdAt=" + event.CreatedAt
		writeSaaSAdminDailyReportCSVRow(writer, "billing", metric, strconv.FormatInt(event.AmountCents, 10), remark)
	}
}

func writeSaaSAdminDailyReportCSVRow(writer *csv.Writer, section, metric, value, remark string) {
	_ = writer.Write([]string{section, metric, value, remark})
}

func writeSaaSAdminBusinessMetricsCSV(writer *csv.Writer, report SaaSAdminBusinessMetricsReport) {
	_ = writer.Write([]string{"section", "metric", "value", "remark"})
	options := report.Options
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "tenantLimit", strconv.Itoa(options.TenantLimit), "")
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "expiringDays", strconv.Itoa(options.ExpiringDays), "")
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "highUsageRatio", strconv.FormatFloat(options.HighUsageRatio, 'f', 6, 64), "")
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "billingLimit", strconv.Itoa(options.BillingLimit), "")

	summary := report.Summary
	summaryRows := []struct {
		metric string
		value  string
	}{
		{"tenantCount", strconv.Itoa(summary.TenantCount)},
		{"activeTenantPackageCount", strconv.Itoa(summary.ActiveTenantPackageCount)},
		{"pricedTenantCount", strconv.Itoa(summary.PricedTenantCount)},
		{"unknownPriceTenantCount", strconv.Itoa(summary.UnknownPriceTenantCount)},
		{"estimatedMrrCents", strconv.FormatInt(summary.EstimatedMRRCents, 10)},
		{"estimatedArrCents", strconv.FormatInt(summary.EstimatedARRCents, 10)},
		{"estimatedArpaCents", strconv.FormatInt(summary.EstimatedARPACents, 10)},
		{"atRiskTenantCount", strconv.Itoa(summary.AtRiskTenantCount)},
		{"atRiskMrrCents", strconv.FormatInt(summary.AtRiskMRRCents, 10)},
		{"expiringSoonTenantCount", strconv.Itoa(summary.ExpiringSoonTenantCount)},
		{"expiringSoonMrrCents", strconv.FormatInt(summary.ExpiringSoonMRRCents, 10)},
		{"expiredTenantCount", strconv.Itoa(summary.ExpiredTenantCount)},
		{"expiredMrrCents", strconv.FormatInt(summary.ExpiredMRRCents, 10)},
		{"recentBillingEventCount", strconv.Itoa(summary.RecentBillingEventCount)},
		{"recentRenewalCount", strconv.Itoa(summary.RecentRenewalCount)},
		{"recentRefundCount", strconv.Itoa(summary.RecentRefundCount)},
		{"recentGrossAmountCents", strconv.FormatInt(summary.RecentGrossAmountCents, 10)},
		{"recentRefundAmountCents", strconv.FormatInt(summary.RecentRefundAmountCents, 10)},
		{"recentBillingAmountCents", strconv.FormatInt(summary.RecentBillingAmountCents, 10)},
		{"billingPricePackageCount", strconv.Itoa(summary.BillingPricePackageCount)},
		{"missingBillingPackageCount", strconv.Itoa(summary.MissingBillingPackageCount)},
	}
	for _, row := range summaryRows {
		writeSaaSAdminDailyReportCSVRow(writer, "summary", row.metric, row.value, "")
	}

	for _, item := range report.Packages {
		remark := "packageName=" + item.PackageName +
			" packageStatus=" + strconv.Itoa(item.PackageStatus) +
			" tenantCount=" + strconv.Itoa(item.TenantCount) +
			" activeTenantCount=" + strconv.Itoa(item.ActiveTenantCount) +
			" pricedTenantCount=" + strconv.Itoa(item.PricedTenantCount) +
			" unknownPriceTenantCount=" + strconv.Itoa(item.UnknownPriceTenantCount) +
			" estimatedArrCents=" + strconv.FormatInt(item.EstimatedARRCents, 10) +
			" atRiskTenantCount=" + strconv.Itoa(item.AtRiskTenantCount) +
			" atRiskMrrCents=" + strconv.FormatInt(item.AtRiskMRRCents, 10) +
			" expiringSoonTenantCount=" + strconv.Itoa(item.ExpiringSoonTenantCount) +
			" expiringSoonMrrCents=" + strconv.FormatInt(item.ExpiringSoonMRRCents, 10) +
			" expiredTenantCount=" + strconv.Itoa(item.ExpiredTenantCount) +
			" expiredMrrCents=" + strconv.FormatInt(item.ExpiredMRRCents, 10) +
			" latestAmountCents=" + strconv.FormatInt(item.LatestAmountCents, 10) +
			" latestBillingEventId=" + strconv.FormatInt(item.LatestBillingEventID, 10) +
			" latestBillingAt=" + item.LatestBillingAt +
			" estimated=" + strconv.FormatBool(item.Estimated)
		writeSaaSAdminDailyReportCSVRow(writer, "package", item.PackageCode, strconv.FormatInt(item.EstimatedMRRCents, 10), remark)
	}
	for _, event := range report.BillingEvents {
		metric := strings.TrimSpace(event.EventType + " " + event.PackageCode)
		remark := "id=" + strconv.FormatInt(event.ID, 10) +
			" tenantId=" + strconv.Itoa(event.TenantID) +
			" packageName=" + event.PackageName +
			" currency=" + event.Currency +
			" paidAt=" + event.PaidAt +
			" externalOrderNo=" + event.ExternalOrderNo +
			" createdAt=" + event.CreatedAt
		writeSaaSAdminDailyReportCSVRow(writer, "recentBilling", metric, strconv.FormatInt(event.AmountCents, 10), remark)
	}
}

func writeSaaSAdminBusinessTrendsCSV(writer *csv.Writer, report SaaSAdminBusinessTrendReport) {
	_ = writer.Write([]string{"section", "metric", "value", "remark"})
	options := report.Options
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "months", strconv.Itoa(options.Months), "")
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "billingLimit", strconv.Itoa(options.BillingLimit), "")
	writeSaaSAdminDailyReportCSVRow(writer, "filter", "taskLimit", strconv.Itoa(options.TaskLimit), "")

	summary := report.Summary
	summaryRows := []struct {
		metric string
		value  string
	}{
		{"monthCount", strconv.Itoa(summary.MonthCount)},
		{"billingEventCount", strconv.Itoa(summary.BillingEventCount)},
		{"renewalCount", strconv.Itoa(summary.RenewalCount)},
		{"refundCount", strconv.Itoa(summary.RefundCount)},
		{"grossAmountCents", strconv.FormatInt(summary.GrossAmountCents, 10)},
		{"refundAmountCents", strconv.FormatInt(summary.RefundAmountCents, 10)},
		{"billingAmountCents", strconv.FormatInt(summary.BillingAmountCents, 10)},
		{"tenantCount", strconv.Itoa(summary.TenantCount)},
		{"packageCount", strconv.Itoa(summary.PackageCount)},
		{"taskCount", strconv.Itoa(summary.TaskCount)},
		{"pendingTaskCount", strconv.Itoa(summary.PendingTaskCount)},
		{"blockedTaskCount", strconv.Itoa(summary.BlockedTaskCount)},
		{"failedTaskCount", strconv.Itoa(summary.FailedTaskCount)},
		{"appliedTaskCount", strconv.Itoa(summary.AppliedTaskCount)},
		{"canceledTaskCount", strconv.Itoa(summary.CanceledTaskCount)},
		{"actionableTaskCount", strconv.Itoa(summary.ActionableTaskCount)},
	}
	for _, row := range summaryRows {
		writeSaaSAdminDailyReportCSVRow(writer, "summary", row.metric, row.value, "")
	}

	for _, item := range report.Months {
		remark := "eventCount=" + strconv.Itoa(item.EventCount) +
			" renewalCount=" + strconv.Itoa(item.RenewalCount) +
			" refundCount=" + strconv.Itoa(item.RefundCount) +
			" grossAmountCents=" + strconv.FormatInt(item.GrossAmountCents, 10) +
			" refundAmountCents=" + strconv.FormatInt(item.RefundAmountCents, 10) +
			" tenantCount=" + strconv.Itoa(item.TenantCount) +
			" packageCount=" + strconv.Itoa(item.PackageCount)
		writeSaaSAdminDailyReportCSVRow(writer, "month", item.Month, strconv.FormatInt(item.AmountCents, 10), remark)
		for _, pkg := range item.Packages {
			metric := strings.TrimSpace(item.Month + " " + pkg.PackageCode)
			remark := "packageName=" + pkg.PackageName +
				" eventCount=" + strconv.Itoa(pkg.EventCount) +
				" renewalCount=" + strconv.Itoa(pkg.RenewalCount) +
				" refundCount=" + strconv.Itoa(pkg.RefundCount) +
				" grossAmountCents=" + strconv.FormatInt(pkg.GrossAmountCents, 10) +
				" refundAmountCents=" + strconv.FormatInt(pkg.RefundAmountCents, 10)
			writeSaaSAdminDailyReportCSVRow(writer, "trendPackage", metric, strconv.FormatInt(pkg.AmountCents, 10), remark)
		}
	}

	funnel := report.RenewalFunnel.Summary
	funnelRows := []struct {
		metric string
		value  string
	}{
		{"taskCount", strconv.Itoa(funnel.TaskCount)},
		{"pendingCount", strconv.Itoa(funnel.PendingCount)},
		{"blockedCount", strconv.Itoa(funnel.BlockedCount)},
		{"failedCount", strconv.Itoa(funnel.FailedCount)},
		{"appliedCount", strconv.Itoa(funnel.AppliedCount)},
		{"canceledCount", strconv.Itoa(funnel.CanceledCount)},
		{"actionableCount", strconv.Itoa(funnel.ActionableCount)},
		{"tenantRenewalCount", strconv.Itoa(funnel.TenantRenewalCount)},
		{"tenantCount", strconv.Itoa(funnel.TenantCount)},
		{"actorUserCount", strconv.Itoa(funnel.ActorUserCount)},
	}
	for _, row := range funnelRows {
		writeSaaSAdminDailyReportCSVRow(writer, "renewalFunnel", row.metric, row.value, "")
	}
	for _, task := range report.RenewalFunnel.RecentTasks {
		metric := strconv.FormatInt(task.ID, 10)
		remark := "taskType=" + task.TaskType +
			" tenantId=" + strconv.Itoa(task.TenantID) +
			" packageCode=" + task.PackageCode +
			" actorUserId=" + strconv.Itoa(task.ActorUserID) +
			" lastError=" + task.LastError +
			" appliedAt=" + task.AppliedAt +
			" createdAt=" + task.CreatedAt +
			" updatedAt=" + task.UpdatedAt +
			" remark=" + task.Remark
		writeSaaSAdminDailyReportCSVRow(writer, "renewalTask", metric, task.Status, remark)
	}
}

func writeSaaSAdminOperationCSV(writer *csv.Writer, logs []SaaSAdminOperationLog) {
	_ = writer.Write([]string{
		"id",
		"tenantId",
		"action",
		"targetType",
		"targetId",
		"targetName",
		"actorUserId",
		"actorTenantId",
		"remark",
		"createdAt",
		"before",
		"after",
	})
	for _, item := range logs {
		_ = writer.Write([]string{
			strconv.FormatInt(item.ID, 10),
			strconv.Itoa(item.TenantID),
			item.Action,
			item.TargetType,
			item.TargetID,
			item.TargetName,
			strconv.Itoa(item.ActorUserID),
			strconv.Itoa(item.ActorTenantID),
			item.Remark,
			item.CreatedAt,
			item.BeforeJSON,
			item.AfterJSON,
		})
	}
}

func writeSaaSAdminBillingCSV(writer *csv.Writer, events []SaaSAdminBillingEvent) {
	_ = writer.Write([]string{
		"id",
		"tenantId",
		"eventType",
		"packageCode",
		"packageName",
		"previousExpiresAt",
		"newExpiresAt",
		"amountCents",
		"currency",
		"paidAt",
		"paymentMethod",
		"externalOrderNo",
		"actorUserId",
		"actorTenantId",
		"remark",
		"metadata",
		"createdAt",
	})
	for _, item := range events {
		_ = writer.Write([]string{
			strconv.FormatInt(item.ID, 10),
			strconv.Itoa(item.TenantID),
			item.EventType,
			item.PackageCode,
			item.PackageName,
			item.PreviousExpiresAt,
			item.NewExpiresAt,
			strconv.FormatInt(item.AmountCents, 10),
			item.Currency,
			item.PaidAt,
			item.PaymentMethod,
			item.ExternalOrderNo,
			strconv.Itoa(item.ActorUserID),
			strconv.Itoa(item.ActorTenantID),
			item.Remark,
			item.MetadataJSON,
			item.CreatedAt,
		})
	}
}

func writeSaaSAdminBillingReconciliationCSV(writer *csv.Writer, items []SaaSAdminBillingReconciliationItem) {
	_ = writer.Write([]string{
		"id",
		"tenantId",
		"tenantName",
		"eventType",
		"packageCode",
		"packageName",
		"previousExpiresAt",
		"newExpiresAt",
		"amountCents",
		"currency",
		"paidAt",
		"paymentMethod",
		"externalOrderNo",
		"currentPackageFound",
		"currentPackageCode",
		"currentPackageName",
		"currentExpiresAt",
		"currentPackageStatus",
		"reconcileStatus",
		"mismatchReasons",
		"actorUserId",
		"actorTenantId",
		"remark",
		"metadata",
		"createdAt",
	})
	for _, item := range items {
		item = saasAdminBillingReconciliationItemWithStatus(item)
		event := item.BillingEvent
		_ = writer.Write([]string{
			strconv.FormatInt(event.ID, 10),
			strconv.Itoa(event.TenantID),
			item.TenantName,
			event.EventType,
			event.PackageCode,
			event.PackageName,
			event.PreviousExpiresAt,
			event.NewExpiresAt,
			strconv.FormatInt(event.AmountCents, 10),
			event.Currency,
			event.PaidAt,
			event.PaymentMethod,
			event.ExternalOrderNo,
			strconv.FormatBool(item.CurrentPackageFound),
			item.CurrentPackageCode,
			item.CurrentPackageName,
			item.CurrentExpiresAt,
			strconv.Itoa(item.CurrentPackageStatus),
			item.Status,
			strings.Join(item.Reasons, ";"),
			strconv.Itoa(event.ActorUserID),
			strconv.Itoa(event.ActorTenantID),
			event.Remark,
			event.MetadataJSON,
			event.CreatedAt,
		})
	}
}

type saasAdminPackageUpsertRequest struct {
	Code            string                 `json:"code"`
	PackageCode     string                 `json:"packageCode"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Status          int                    `json:"status"`
	Limits          SaaSAdminPackageLimits `json:"limits"`
	ExpectedVersion int                    `json:"expectedVersion"`
}

func parseSaaSAdminPackageUpsert(r *http.Request) (SaaSAdminPackageUpsert, error) {
	var req saasAdminPackageUpsertRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminPackageUpsert{}, err
		}
		req.Code = saasAdminFirstNonEmpty(r.FormValue("code"), r.FormValue("packageCode"), r.FormValue("package_code"))
		req.Name = r.FormValue("name")
		req.Description = r.FormValue("description")
		req.Status, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("status")))
		req.ExpectedVersion, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("expectedVersion")))
		limitsRaw := strings.TrimSpace(r.FormValue("limits"))
		if limitsRaw != "" {
			if err := json.Unmarshal([]byte(limitsRaw), &req.Limits); err != nil {
				return SaaSAdminPackageUpsert{}, errors.New("limits JSON 格式错误")
			}
		}
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminPackageUpsert{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminPackageUpsert{}, errors.New("JSON 格式错误")
			}
		}
	}
	update := SaaSAdminPackageUpsert{
		Code:            saasAdminFirstNonEmpty(req.Code, req.PackageCode),
		Name:            req.Name,
		Description:     req.Description,
		Status:          req.Status,
		Limits:          req.Limits,
		ExpectedVersion: req.ExpectedVersion,
	}
	if err := normalizeSaaSAdminPackageUpsert(&update); err != nil {
		return SaaSAdminPackageUpsert{}, err
	}
	return update, nil
}

func normalizeSaaSAdminPackageUpsert(update *SaaSAdminPackageUpsert) error {
	update.Code = strings.TrimSpace(update.Code)
	update.Name = strings.TrimSpace(update.Name)
	update.Description = strings.TrimSpace(update.Description)
	if update.Code == "" {
		return errors.New("code required")
	}
	if !validSaaSAdminPackageCode(update.Code) {
		return errors.New("code 仅支持字母、数字、下划线和中划线")
	}
	if len([]rune(update.Code)) > 64 {
		return errors.New("code too long")
	}
	if update.Name == "" {
		return errors.New("name required")
	}
	if len([]rune(update.Name)) > 100 {
		return errors.New("name too long")
	}
	if len([]rune(update.Description)) > 255 {
		return errors.New("description too long")
	}
	if update.Status == 0 {
		update.Status = 1
	}
	if update.Status != 1 && update.Status != 2 {
		return errors.New("status must be 1 or 2")
	}
	if update.ExpectedVersion < 0 {
		return errors.New("expectedVersion must not be negative")
	}
	if !saasAdminPackageLimitsValid(update.Limits) {
		return errors.New("limits must not be negative")
	}
	return nil
}

func validSaaSAdminPackageCode(code string) bool {
	for _, r := range code {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func saasAdminPackageLimitsValid(limits SaaSAdminPackageLimits) bool {
	for _, value := range []int64{
		limits.MaxCorps,
		limits.MaxUsers,
		limits.MaxContacts,
		limits.MaxRooms,
		limits.MaxAgents,
		limits.ChannelCodes,
		limits.ShopCodes,
		limits.Radars,
		limits.Lotteries,
		limits.RoomInfinitePulls,
		limits.RoomFissions,
		limits.RoomClockIns,
		limits.RoomQualities,
		limits.RoomCalendars,
		limits.RoomReminds,
		limits.ContactSOPs,
		limits.RoomSOPs,
		limits.SensitiveWords,
		limits.StorageMB,
		limits.ContactMessageBatches,
		limits.RoomMessageBatches,
		limits.RoomTagPulls,
		limits.WorkRoomAutoPulls,
		limits.WorkFissions,
		limits.OfficialAccounts,
		limits.AsyncExecutions,
	} {
		if value < 0 {
			return false
		}
	}
	return true
}

type saasAdminTenantPackageUpdateRequest struct {
	TenantID        int    `json:"tenantId"`
	PackageCode     string `json:"packageCode"`
	ExpiresAt       string `json:"expiresAt"`
	Remark          string `json:"remark"`
	ExpectedVersion int    `json:"expectedVersion"`
}

func parseSaaSAdminTenantPackageUpdate(r *http.Request) (SaaSAdminTenantPackageUpdate, error) {
	var req saasAdminTenantPackageUpdateRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminTenantPackageUpdate{}, err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))))
		req.PackageCode = saasAdminFirstNonEmpty(r.FormValue("packageCode"), r.FormValue("package_code"))
		req.ExpiresAt = saasAdminFirstNonEmpty(r.FormValue("expiresAt"), r.FormValue("expires_at"))
		req.Remark = r.FormValue("remark")
		req.ExpectedVersion, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("expectedVersion"), r.FormValue("expected_version"))))
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminTenantPackageUpdate{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminTenantPackageUpdate{}, errors.New("JSON 格式错误")
			}
		}
	}
	req.PackageCode = strings.TrimSpace(req.PackageCode)
	req.ExpiresAt = strings.TrimSpace(req.ExpiresAt)
	req.Remark = strings.TrimSpace(req.Remark)
	if req.TenantID <= 0 {
		return SaaSAdminTenantPackageUpdate{}, errors.New("tenantId required")
	}
	if req.PackageCode == "" {
		return SaaSAdminTenantPackageUpdate{}, errors.New("packageCode required")
	}
	if req.ExpectedVersion < 0 {
		return SaaSAdminTenantPackageUpdate{}, errors.New("expectedVersion must not be negative")
	}
	if len([]rune(req.Remark)) > 255 {
		return SaaSAdminTenantPackageUpdate{}, errors.New("remark too long")
	}
	expiresAt, err := normalizeSaaSAdminExpiresAt(req.ExpiresAt)
	if err != nil {
		return SaaSAdminTenantPackageUpdate{}, err
	}
	return SaaSAdminTenantPackageUpdate{
		TenantID:        req.TenantID,
		PackageCode:     req.PackageCode,
		ExpiresAt:       expiresAt,
		Remark:          req.Remark,
		ExpectedVersion: req.ExpectedVersion,
	}, nil
}

type saasAdminPackageTenantSnapshotSyncRequest struct {
	Code                string `json:"code"`
	PackageCode         string `json:"packageCode"`
	PackageCodeSnake    string `json:"package_code"`
	TenantID            int    `json:"tenantId"`
	TenantIDSnake       int    `json:"tenant_id"`
	Limit               int    `json:"limit"`
	DryRun              *bool  `json:"dryRun"`
	DryRunSnake         *bool  `json:"dry_run"`
	AllowOverLimit      *bool  `json:"allowOverLimit"`
	AllowOverLimitSnake *bool  `json:"allow_over_limit"`
	Remark              string `json:"remark"`
}

func parseSaaSAdminPackageTenantSnapshotSync(r *http.Request) (SaaSAdminPackageTenantSnapshotSync, error) {
	dryRun := true
	allowOverLimit := false
	var req saasAdminPackageTenantSnapshotSyncRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminPackageTenantSnapshotSync{}, err
		}
		req.Code = saasAdminFirstNonEmpty(r.FormValue("packageCode"), r.FormValue("package_code"), r.FormValue("code"))
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminPackageTenantSnapshotSync{}, errors.New("tenantId invalid")
			}
			req.TenantID = value
		}
		if raw := strings.TrimSpace(r.FormValue("limit")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminPackageTenantSnapshotSync{}, errors.New("limit invalid")
			}
			req.Limit = value
		}
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("dryRun"), r.FormValue("dry_run"))); raw != "" {
			value, err := parseSaaSAdminRequestBool(raw, "dryRun")
			if err != nil {
				return SaaSAdminPackageTenantSnapshotSync{}, err
			}
			dryRun = value
		}
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("allowOverLimit"), r.FormValue("allow_over_limit"))); raw != "" {
			value, err := parseSaaSAdminRequestBool(raw, "allowOverLimit")
			if err != nil {
				return SaaSAdminPackageTenantSnapshotSync{}, err
			}
			allowOverLimit = value
		}
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminPackageTenantSnapshotSync{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminPackageTenantSnapshotSync{}, errors.New("JSON 格式错误")
			}
		}
		if req.TenantID == 0 && req.TenantIDSnake > 0 {
			req.TenantID = req.TenantIDSnake
		}
		if req.DryRun != nil {
			dryRun = *req.DryRun
		} else if req.DryRunSnake != nil {
			dryRun = *req.DryRunSnake
		}
		if req.AllowOverLimit != nil {
			allowOverLimit = *req.AllowOverLimit
		} else if req.AllowOverLimitSnake != nil {
			allowOverLimit = *req.AllowOverLimitSnake
		}
	}
	packageCode := strings.TrimSpace(saasAdminFirstNonEmpty(req.PackageCode, req.PackageCodeSnake, req.Code))
	remark := strings.TrimSpace(req.Remark)
	if packageCode == "" {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("packageCode required")
	}
	if !validSaaSAdminPackageCode(packageCode) {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("packageCode 仅支持字母、数字、下划线和中划线")
	}
	if len([]rune(packageCode)) > 64 {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("packageCode too long")
	}
	if req.TenantID < 0 {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("tenantId invalid")
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("remark too long")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = saasAdminListMaxLimit
	}
	if limit > saasAdminExportMaxLimit {
		limit = saasAdminExportMaxLimit
	}
	return SaaSAdminPackageTenantSnapshotSync{
		PackageCode:    packageCode,
		TenantID:       req.TenantID,
		Limit:          limit,
		DryRun:         dryRun,
		AllowOverLimit: allowOverLimit,
		Remark:         remark,
	}, nil
}

func parseSaaSAdminTaskID(r *http.Request) (int64, error) {
	var req struct {
		TaskID      int64 `json:"taskId"`
		TaskIDSnake int64 `json:"task_id"`
		ID          int64 `json:"id"`
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return 0, err
		}
		raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("taskId"), r.FormValue("task_id"), r.FormValue("id")))
		if raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return 0, errors.New("taskId invalid")
			}
			req.TaskID = value
		}
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return 0, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return 0, errors.New("JSON 格式错误")
			}
		}
	}
	taskID := req.TaskID
	if taskID == 0 {
		taskID = req.TaskIDSnake
	}
	if taskID == 0 {
		taskID = req.ID
	}
	if taskID <= 0 {
		return 0, errors.New("taskId required")
	}
	return taskID, nil
}

type saasAdminTaskCancelRequest struct {
	TaskID int64
	Remark string
}

func parseSaaSAdminTaskCancel(r *http.Request) (saasAdminTaskCancelRequest, error) {
	var req struct {
		TaskID      int64  `json:"taskId"`
		TaskIDSnake int64  `json:"task_id"`
		ID          int64  `json:"id"`
		Remark      string `json:"remark"`
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return saasAdminTaskCancelRequest{}, err
		}
		raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("taskId"), r.FormValue("task_id"), r.FormValue("id")))
		if raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return saasAdminTaskCancelRequest{}, errors.New("taskId invalid")
			}
			req.TaskID = value
		}
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return saasAdminTaskCancelRequest{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return saasAdminTaskCancelRequest{}, errors.New("JSON 格式错误")
			}
		}
	}
	taskID := req.TaskID
	if taskID == 0 {
		taskID = req.TaskIDSnake
	}
	if taskID == 0 {
		taskID = req.ID
	}
	if taskID <= 0 {
		return saasAdminTaskCancelRequest{}, errors.New("taskId required")
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return saasAdminTaskCancelRequest{}, errors.New("remark 最多 255 个字符")
	}
	return saasAdminTaskCancelRequest{
		TaskID: taskID,
		Remark: remark,
	}, nil
}

type saasAdminTaskBulkCancelRequest struct {
	TaskID           int64  `json:"taskId"`
	TaskIDSnake      int64  `json:"task_id"`
	ID               int64  `json:"id"`
	TaskType         string `json:"taskType"`
	TaskTypeSnake    string `json:"task_type"`
	Type             string `json:"type"`
	Status           string `json:"status"`
	TenantID         int    `json:"tenantId"`
	TenantIDSnake    int    `json:"tenant_id"`
	PackageCode      string `json:"packageCode"`
	PackageCodeSnake string `json:"package_code"`
	Limit            int    `json:"limit"`
	Remark           string `json:"remark"`
}

func parseSaaSAdminTaskBulkCancel(r *http.Request) (SaaSAdminTaskBulkCancel, error) {
	return parseSaaSAdminTaskBulkRequest(r, "批量取消运营任务")
}

func parseSaaSAdminTaskBulkReset(r *http.Request) (SaaSAdminTaskBulkCancel, error) {
	return parseSaaSAdminTaskBulkRequest(r, "批量重置运营任务")
}

func parseSaaSAdminPackageSyncTaskBulkApply(r *http.Request) (SaaSAdminTaskBulkCancel, error) {
	apply, err := parseSaaSAdminTaskBulkRequest(r, "批量应用套餐同步任务")
	if err != nil {
		return SaaSAdminTaskBulkCancel{}, err
	}
	if apply.Options.TaskType == "" {
		apply.Options.TaskType = SaaSAdminTaskTypePackageSync
	}
	if apply.Options.TaskType != SaaSAdminTaskTypePackageSync {
		return SaaSAdminTaskBulkCancel{}, errors.New("taskType 必须是 package_sync")
	}
	return apply, nil
}

func parseSaaSAdminTenantRenewalTaskBulkApply(r *http.Request) (SaaSAdminTaskBulkCancel, error) {
	apply, err := parseSaaSAdminTaskBulkRequest(r, "批量应用续费任务")
	if err != nil {
		return SaaSAdminTaskBulkCancel{}, err
	}
	if apply.Options.TaskType == "" {
		apply.Options.TaskType = SaaSAdminTaskTypeTenantRenewal
	}
	if apply.Options.TaskType != SaaSAdminTaskTypeTenantRenewal {
		return SaaSAdminTaskBulkCancel{}, errors.New("taskType 必须是 tenant_renewal")
	}
	return apply, nil
}

func parseSaaSAdminTenantProvisionTaskBulkApply(r *http.Request) (SaaSAdminTaskBulkCancel, error) {
	apply, err := parseSaaSAdminTaskBulkRequest(r, "批量应用平台开户任务")
	if err != nil {
		return SaaSAdminTaskBulkCancel{}, err
	}
	if apply.Options.TaskType == "" {
		apply.Options.TaskType = SaaSAdminTaskTypeTenantProvision
	}
	if apply.Options.TaskType != SaaSAdminTaskTypeTenantProvision {
		return SaaSAdminTaskBulkCancel{}, errors.New("taskType 必须是 tenant_provision")
	}
	return apply, nil
}

func parseSaaSAdminTaskBulkRequest(r *http.Request, defaultRemark string) (SaaSAdminTaskBulkCancel, error) {
	req := saasAdminTaskBulkCancelRequest{
		TaskType:    saasAdminFirstNonEmpty(r.URL.Query().Get("taskType"), r.URL.Query().Get("task_type"), r.URL.Query().Get("type")),
		Status:      r.URL.Query().Get("status"),
		TenantID:    saasAdminQueryInt(r, "tenantId", 0),
		PackageCode: saasAdminFirstNonEmpty(r.URL.Query().Get("packageCode"), r.URL.Query().Get("package_code")),
		Limit:       positiveQueryInt(r, "limit", 50),
	}
	if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("taskId"), r.URL.Query().Get("task_id"), r.URL.Query().Get("id"))); raw != "" {
		taskID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return SaaSAdminTaskBulkCancel{}, errors.New("taskId invalid")
		}
		req.TaskID = taskID
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminTaskBulkCancel{}, err
		}
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("taskId"), r.FormValue("task_id"), r.FormValue("id"))); raw != "" {
			taskID, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return SaaSAdminTaskBulkCancel{}, errors.New("taskId invalid")
			}
			req.TaskID = taskID
		}
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))); raw != "" {
			tenantID, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminTaskBulkCancel{}, errors.New("tenantId invalid")
			}
			req.TenantID = tenantID
		}
		if raw := strings.TrimSpace(r.FormValue("limit")); raw != "" {
			limit, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminTaskBulkCancel{}, errors.New("limit invalid")
			}
			req.Limit = limit
		}
		req.TaskType = saasAdminFirstNonEmpty(r.FormValue("taskType"), r.FormValue("task_type"), r.FormValue("type"), req.TaskType)
		req.Status = saasAdminFirstNonEmpty(r.FormValue("status"), req.Status)
		req.PackageCode = saasAdminFirstNonEmpty(r.FormValue("packageCode"), r.FormValue("package_code"), req.PackageCode)
		req.Remark = saasAdminFirstNonEmpty(r.FormValue("remark"), req.Remark)
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminTaskBulkCancel{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminTaskBulkCancel{}, errors.New("JSON 格式错误")
			}
		}
	}
	taskID := req.TaskID
	if taskID == 0 {
		taskID = req.TaskIDSnake
	}
	if taskID == 0 {
		taskID = req.ID
	}
	if taskID < 0 {
		return SaaSAdminTaskBulkCancel{}, errors.New("taskId invalid")
	}
	tenantID := req.TenantID
	if tenantID == 0 {
		tenantID = req.TenantIDSnake
	}
	if tenantID < 0 {
		return SaaSAdminTaskBulkCancel{}, errors.New("tenantId invalid")
	}
	taskType := strings.ToLower(strings.TrimSpace(saasAdminFirstNonEmpty(req.TaskType, req.TaskTypeSnake, req.Type)))
	if taskType == "all" {
		taskType = ""
	}
	if taskType != "" && !validSaaSAdminTaskType(taskType) {
		return SaaSAdminTaskBulkCancel{}, errors.New("taskType 必须是 package_sync、tenant_renewal、tenant_provision 或 all")
	}
	status, err := normalizeSaaSAdminTaskStatus(req.Status)
	if err != nil {
		return SaaSAdminTaskBulkCancel{}, err
	}
	packageCode := strings.TrimSpace(saasAdminFirstNonEmpty(req.PackageCode, req.PackageCodeSnake))
	if packageCode != "" && !validSaaSAdminPackageCode(packageCode) {
		return SaaSAdminTaskBulkCancel{}, errors.New("packageCode 仅支持字母、数字、下划线和中划线")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = defaultRemark
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminTaskBulkCancel{}, errors.New("remark too long")
	}
	return SaaSAdminTaskBulkCancel{
		Options: SaaSAdminTaskOptions{
			TaskID:      taskID,
			TaskType:    taskType,
			Status:      status,
			TenantID:    tenantID,
			PackageCode: packageCode,
			Limit:       limit,
		},
		Remark: remark,
	}, nil
}

func saasAdminPackageSyncTaskRequestPayload(sync SaaSAdminPackageTenantSnapshotSync) map[string]any {
	return map[string]any{
		"packageCode":    sync.PackageCode,
		"tenantId":       sync.TenantID,
		"limit":          sync.Limit,
		"dryRun":         false,
		"allowOverLimit": sync.AllowOverLimit,
		"remark":         sync.Remark,
	}
}

func saasAdminPackageSyncFromTaskRequest(raw string) (SaaSAdminPackageTenantSnapshotSync, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("task request missing")
	}
	var req saasAdminPackageTenantSnapshotSyncRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("task request JSON 格式错误")
	}
	if req.TenantID == 0 && req.TenantIDSnake > 0 {
		req.TenantID = req.TenantIDSnake
	}
	allowOverLimit := false
	if req.AllowOverLimit != nil {
		allowOverLimit = *req.AllowOverLimit
	} else if req.AllowOverLimitSnake != nil {
		allowOverLimit = *req.AllowOverLimitSnake
	}
	packageCode := strings.TrimSpace(saasAdminFirstNonEmpty(req.PackageCode, req.PackageCodeSnake, req.Code))
	remark := strings.TrimSpace(req.Remark)
	if packageCode == "" {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("packageCode required")
	}
	if !validSaaSAdminPackageCode(packageCode) {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("packageCode 仅支持字母、数字、下划线和中划线")
	}
	if len([]rune(packageCode)) > 64 {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("packageCode too long")
	}
	if req.TenantID < 0 {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("tenantId invalid")
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminPackageTenantSnapshotSync{}, errors.New("remark too long")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = saasAdminListMaxLimit
	}
	if limit > saasAdminExportMaxLimit {
		limit = saasAdminExportMaxLimit
	}
	return SaaSAdminPackageTenantSnapshotSync{
		PackageCode:    packageCode,
		TenantID:       req.TenantID,
		Limit:          limit,
		DryRun:         false,
		AllowOverLimit: allowOverLimit,
		Remark:         remark,
	}, nil
}

func saasAdminTenantRenewalTaskRequestPayload(renewal SaaSAdminTenantRenewal) map[string]any {
	return map[string]any{
		"tenantId":        renewal.TenantID,
		"packageCode":     renewal.PackageCode,
		"expiresAt":       renewal.ExpiresAt,
		"amountCents":     renewal.AmountCents,
		"currency":        renewal.Currency,
		"paidAt":          renewal.PaidAt,
		"paymentMethod":   renewal.PaymentMethod,
		"externalOrderNo": renewal.ExternalOrderNo,
		"remark":          renewal.Remark,
	}
}

func saasAdminTenantRenewalFromTaskRequest(raw string) (SaaSAdminTenantRenewal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SaaSAdminTenantRenewal{}, errors.New("task request missing")
	}
	var req saasAdminTenantRenewalRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return SaaSAdminTenantRenewal{}, errors.New("task request JSON 格式错误")
	}
	return normalizeSaaSAdminTenantRenewalRequest(req)
}

func saasAdminTenantProvisionTaskRequestPayload(provision SaaSAdminTenantProvision) map[string]any {
	return map[string]any{
		"tenantId":          provision.TenantID,
		"tenantName":        provision.TenantName,
		"adminPhone":        provision.AdminPhone,
		"adminName":         provision.AdminName,
		"adminPasswordHash": provision.AdminPasswordHash,
		"roleName":          provision.RoleName,
		"packageCode":       provision.PackageCode,
		"expiresAt":         provision.ExpiresAt,
		"configCopyMode":    provision.ConfigCopyMode,
		"remark":            provision.Remark,
	}
}

func saasAdminTenantProvisionFromTaskRequest(raw string) (SaaSAdminTenantProvision, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SaaSAdminTenantProvision{}, errors.New("task request missing")
	}
	var req saasAdminTenantProvisionRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return SaaSAdminTenantProvision{}, errors.New("task request JSON 格式错误")
	}
	if req.TenantID < 0 {
		return SaaSAdminTenantProvision{}, errors.New("tenantId invalid")
	}
	tenantName := strings.TrimSpace(req.TenantName)
	if tenantName == "" {
		return SaaSAdminTenantProvision{}, errors.New("tenantName required")
	}
	if len([]rune(tenantName)) > 255 {
		return SaaSAdminTenantProvision{}, errors.New("tenantName too long")
	}
	adminPhone := strings.TrimSpace(saasAdminFirstNonEmpty(req.AdminPhone, req.Phone))
	if len(adminPhone) != 11 || !validPhone(adminPhone) {
		return SaaSAdminTenantProvision{}, errors.New("adminPhone invalid")
	}
	adminPasswordHash := strings.TrimSpace(req.AdminPasswordHash)
	if adminPasswordHash == "" {
		return SaaSAdminTenantProvision{}, errors.New("adminPasswordHash required")
	}
	if len([]rune(adminPasswordHash)) > 255 {
		return SaaSAdminTenantProvision{}, errors.New("adminPasswordHash too long")
	}
	adminName := strings.TrimSpace(saasAdminFirstNonEmpty(req.AdminName, req.UserName))
	if adminName == "" {
		adminName = "超级管理员"
	}
	if len([]rune(adminName)) > 255 {
		return SaaSAdminTenantProvision{}, errors.New("adminName too long")
	}
	roleName := strings.TrimSpace(req.RoleName)
	if roleName == "" {
		roleName = "超级管理员"
	}
	if len([]rune(roleName)) > 255 {
		return SaaSAdminTenantProvision{}, errors.New("roleName too long")
	}
	packageCode := strings.TrimSpace(req.PackageCode)
	if packageCode == "" {
		return SaaSAdminTenantProvision{}, errors.New("packageCode required")
	}
	if !validSaaSAdminPackageCode(packageCode) {
		return SaaSAdminTenantProvision{}, errors.New("packageCode 仅支持字母、数字、下划线和中划线")
	}
	if len([]rune(packageCode)) > 64 {
		return SaaSAdminTenantProvision{}, errors.New("packageCode too long")
	}
	expiresAt, err := normalizeSaaSAdminExpiresAt(req.ExpiresAt)
	if err != nil {
		return SaaSAdminTenantProvision{}, err
	}
	configCopyMode := strings.ToLower(strings.TrimSpace(req.ConfigCopyMode))
	if configCopyMode == "" {
		configCopyMode = "missing"
	}
	if configCopyMode != "missing" && configCopyMode != "overwrite" && configCopyMode != "skip" {
		return SaaSAdminTenantProvision{}, errors.New("configCopyMode must be missing, overwrite, or skip")
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return SaaSAdminTenantProvision{}, errors.New("remark too long")
	}
	return SaaSAdminTenantProvision{
		TenantID:          req.TenantID,
		TenantName:        tenantName,
		AdminPhone:        adminPhone,
		AdminName:         adminName,
		AdminPasswordHash: adminPasswordHash,
		RoleName:          roleName,
		PackageCode:       packageCode,
		ExpiresAt:         expiresAt,
		ConfigCopyMode:    configCopyMode,
		Remark:            remark,
	}, nil
}

type saasAdminTenantStatusUpdateRequest struct {
	TenantID int    `json:"tenantId"`
	Status   int    `json:"status"`
	Remark   string `json:"remark"`
}

func parseSaaSAdminTenantStatusUpdate(r *http.Request) (SaaSAdminTenantStatusUpdate, error) {
	var req saasAdminTenantStatusUpdateRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminTenantStatusUpdate{}, err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))))
		req.Status, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("status")))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminTenantStatusUpdate{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminTenantStatusUpdate{}, errors.New("JSON 格式错误")
			}
		}
	}
	if req.TenantID <= 0 {
		return SaaSAdminTenantStatusUpdate{}, errors.New("tenantId required")
	}
	if req.Status != 1 && req.Status != 2 {
		return SaaSAdminTenantStatusUpdate{}, errors.New("status must be 1 or 2")
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return SaaSAdminTenantStatusUpdate{}, errors.New("remark too long")
	}
	return SaaSAdminTenantStatusUpdate{
		TenantID: req.TenantID,
		Status:   req.Status,
		Remark:   remark,
	}, nil
}

type saasAdminRiskFollowUpRequest struct {
	TenantID       int    `json:"tenantId"`
	Status         string `json:"status"`
	Owner          string `json:"owner"`
	NextFollowUpAt string `json:"nextFollowUpAt"`
	Remark         string `json:"remark"`
}

type saasAdminCustomerSuccessAssignRequest struct {
	Status              string `json:"status"`
	Owner               string `json:"owner"`
	AssignOwner         string `json:"assignOwner"`
	Assignee            string `json:"assignee"`
	NextFollowUpAt      string `json:"nextFollowUpAt"`
	NextFollowUpAtSnake string `json:"next_follow_up_at"`
	Remark              string `json:"remark"`
}

func parseSaaSAdminCustomerSuccessAssign(r *http.Request) (SaaSAdminCustomerSuccessAssign, error) {
	req := saasAdminCustomerSuccessAssignRequest{Status: SaaSAdminRiskFollowUpStatusPending}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminCustomerSuccessAssign{}, err
		}
		req.Status = saasAdminFirstNonEmpty(r.FormValue("status"), req.Status)
		req.Owner = saasAdminFirstNonEmpty(r.FormValue("assignOwner"), r.FormValue("assign_owner"), r.FormValue("assignee"), r.FormValue("owner"))
		req.NextFollowUpAt = saasAdminFirstNonEmpty(r.FormValue("nextFollowUpAt"), r.FormValue("next_follow_up_at"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminCustomerSuccessAssign{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminCustomerSuccessAssign{}, errors.New("JSON 格式错误")
			}
		}
	}
	status, err := normalizeSaaSAdminRiskFollowUpStatus(req.Status)
	if err != nil {
		return SaaSAdminCustomerSuccessAssign{}, err
	}
	if status == SaaSAdminRiskFollowUpStatusResolved || status == SaaSAdminRiskFollowUpStatusIgnored {
		return SaaSAdminCustomerSuccessAssign{}, errors.New("status 必须是 pending、contacted 或 renewal_pending")
	}
	owner := strings.TrimSpace(saasAdminFirstNonEmpty(req.AssignOwner, req.Assignee, req.Owner))
	if owner == "" {
		return SaaSAdminCustomerSuccessAssign{}, errors.New("owner required")
	}
	if len([]rune(owner)) > 80 {
		return SaaSAdminCustomerSuccessAssign{}, errors.New("owner too long")
	}
	nextFollowUpAt, err := normalizeSaaSAdminOptionalDateTime(saasAdminFirstNonEmpty(req.NextFollowUpAt, req.NextFollowUpAtSnake))
	if err != nil {
		return SaaSAdminCustomerSuccessAssign{}, errors.New("nextFollowUpAt format invalid")
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "客户成功队列批量分派"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminCustomerSuccessAssign{}, errors.New("remark too long")
	}
	return SaaSAdminCustomerSuccessAssign{
		Status:         status,
		Owner:          owner,
		NextFollowUpAt: nextFollowUpAt,
		Remark:         remark,
	}, nil
}

func parseSaaSAdminOperationQueueAssign(r *http.Request) (SaaSAdminOperationQueueAssign, error) {
	req := saasAdminCustomerSuccessAssignRequest{Status: SaaSAdminRiskFollowUpStatusPending}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminOperationQueueAssign{}, err
		}
		req.Status = saasAdminFirstNonEmpty(r.FormValue("status"), req.Status)
		req.Owner = saasAdminFirstNonEmpty(r.FormValue("assignOwner"), r.FormValue("assign_owner"), r.FormValue("assignee"), r.FormValue("owner"))
		req.NextFollowUpAt = saasAdminFirstNonEmpty(r.FormValue("nextFollowUpAt"), r.FormValue("next_follow_up_at"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminOperationQueueAssign{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminOperationQueueAssign{}, errors.New("JSON 格式错误")
			}
		}
	}
	status, err := normalizeSaaSAdminRiskFollowUpStatus(req.Status)
	if err != nil {
		return SaaSAdminOperationQueueAssign{}, err
	}
	if status == SaaSAdminRiskFollowUpStatusResolved || status == SaaSAdminRiskFollowUpStatusIgnored {
		return SaaSAdminOperationQueueAssign{}, errors.New("status 必须是 pending、contacted 或 renewal_pending")
	}
	owner := strings.TrimSpace(saasAdminFirstNonEmpty(req.AssignOwner, req.Assignee, req.Owner))
	if owner == "" {
		return SaaSAdminOperationQueueAssign{}, errors.New("owner required")
	}
	if len([]rune(owner)) > 80 {
		return SaaSAdminOperationQueueAssign{}, errors.New("owner too long")
	}
	nextFollowUpAt, err := normalizeSaaSAdminOptionalDateTime(saasAdminFirstNonEmpty(req.NextFollowUpAt, req.NextFollowUpAtSnake))
	if err != nil {
		return SaaSAdminOperationQueueAssign{}, errors.New("nextFollowUpAt format invalid")
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "运营待办批量分派"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminOperationQueueAssign{}, errors.New("remark too long")
	}
	return SaaSAdminOperationQueueAssign{
		Status:         status,
		Owner:          owner,
		NextFollowUpAt: nextFollowUpAt,
		Remark:         remark,
	}, nil
}

type saasAdminOperationQueueAssignmentCloseRequest struct {
	OperationID      int64  `json:"operationId"`
	OperationIDSnake int64  `json:"operation_id"`
	Status           string `json:"status"`
	CloseStatus      string `json:"closeStatus"`
	CloseStatusSnake string `json:"close_status"`
	Remark           string `json:"remark"`
}

func parseSaaSAdminOperationQueueAssignmentClose(r *http.Request) (SaaSAdminOperationQueueAssignmentClose, error) {
	var req saasAdminOperationQueueAssignmentCloseRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminOperationQueueAssignmentClose{}, err
		}
		req.OperationID, _ = strconv.ParseInt(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("operationId"), r.FormValue("operation_id"))), 10, 64)
		req.Status = r.FormValue("status")
		req.CloseStatus = saasAdminFirstNonEmpty(r.FormValue("closeStatus"), r.FormValue("close_status"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminOperationQueueAssignmentClose{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminOperationQueueAssignmentClose{}, errors.New("JSON 格式错误")
			}
		}
	}
	operationID := req.OperationID
	if operationID <= 0 {
		operationID = req.OperationIDSnake
	}
	if operationID <= 0 {
		return SaaSAdminOperationQueueAssignmentClose{}, errors.New("operationId required")
	}
	status, err := normalizeSaaSAdminRiskFollowUpStatus(saasAdminFirstNonEmpty(req.CloseStatus, req.CloseStatusSnake, req.Status, SaaSAdminRiskFollowUpStatusResolved))
	if err != nil {
		return SaaSAdminOperationQueueAssignmentClose{}, err
	}
	if status != SaaSAdminRiskFollowUpStatusResolved && status != SaaSAdminRiskFollowUpStatusIgnored {
		return SaaSAdminOperationQueueAssignmentClose{}, errors.New("closeStatus 必须是 resolved 或 ignored")
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "关闭运营待办认领"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminOperationQueueAssignmentClose{}, errors.New("remark too long")
	}
	return SaaSAdminOperationQueueAssignmentClose{
		OperationID: operationID,
		Status:      status,
		Remark:      remark,
	}, nil
}

type saasAdminOperationQueueAssignmentNotificationsRequest struct {
	Channel          string `json:"channel"`
	MaxAttempts      int    `json:"maxAttempts"`
	MaxAttemptsSnake int    `json:"max_attempts"`
	Remark           string `json:"remark"`
	ForceCreate      *bool  `json:"forceCreate"`
	ForceCreateSnake *bool  `json:"force_create"`
}

func parseSaaSAdminOperationQueueAssignmentNotifications(r *http.Request, options SaaSAdminOperationQueueAssignmentOptions) (SaaSAdminOperationQueueAssignmentNotifications, error) {
	req := saasAdminOperationQueueAssignmentNotificationsRequest{
		Channel:     SaaSAlertNotificationChannelWebhook,
		MaxAttempts: 3,
	}
	forceCreate := false
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminOperationQueueAssignmentNotifications{}, err
		}
		req.Channel = saasAdminFirstNonEmpty(r.FormValue("channel"), req.Channel)
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("maxAttempts"), r.FormValue("max_attempts"))); raw != "" {
			maxAttempts, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminOperationQueueAssignmentNotifications{}, errors.New("maxAttempts invalid")
			}
			req.MaxAttempts = maxAttempts
		}
		req.Remark = r.FormValue("remark")
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("forceCreate"), r.FormValue("force_create"))); raw != "" {
			value, err := parseSaaSAdminRequestBool(raw, "forceCreate")
			if err != nil {
				return SaaSAdminOperationQueueAssignmentNotifications{}, err
			}
			forceCreate = value
		}
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminOperationQueueAssignmentNotifications{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminOperationQueueAssignmentNotifications{}, errors.New("JSON 格式错误")
			}
		}
		if req.MaxAttemptsSnake != 0 {
			req.MaxAttempts = req.MaxAttemptsSnake
		}
		if req.ForceCreate != nil {
			forceCreate = *req.ForceCreate
		} else if req.ForceCreateSnake != nil {
			forceCreate = *req.ForceCreateSnake
		}
	}
	channel := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("channel"), req.Channel))
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	if channel != SaaSAlertNotificationChannelWebhook {
		return SaaSAdminOperationQueueAssignmentNotifications{}, errors.New("channel must be webhook")
	}
	maxAttempts := req.MaxAttempts
	if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("maxAttempts"), r.URL.Query().Get("max_attempts"))); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return SaaSAdminOperationQueueAssignmentNotifications{}, errors.New("maxAttempts invalid")
		}
		maxAttempts = parsed
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if maxAttempts > 20 {
		return SaaSAdminOperationQueueAssignmentNotifications{}, errors.New("maxAttempts must be <= 20")
	}
	if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("forceCreate"), r.URL.Query().Get("force_create"))); raw != "" {
		value, err := parseSaaSAdminRequestBool(raw, "forceCreate")
		if err != nil {
			return SaaSAdminOperationQueueAssignmentNotifications{}, err
		}
		forceCreate = value
	}
	remark := strings.TrimSpace(saasAdminFirstNonEmpty(r.URL.Query().Get("remark"), req.Remark))
	if remark == "" {
		remark = "运营待办认领到期提醒"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminOperationQueueAssignmentNotifications{}, errors.New("remark too long")
	}
	return SaaSAdminOperationQueueAssignmentNotifications{
		Options:     options,
		Channel:     channel,
		MaxAttempts: maxAttempts,
		Remark:      remark,
		ForceCreate: forceCreate,
	}, nil
}

func parseSaaSAdminRenewalForecastAssign(r *http.Request) (SaaSAdminRenewalForecastAssign, error) {
	req := saasAdminCustomerSuccessAssignRequest{Status: SaaSAdminRiskFollowUpStatusRenewalPending}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminRenewalForecastAssign{}, err
		}
		req.Status = saasAdminFirstNonEmpty(r.FormValue("status"), req.Status)
		req.Owner = saasAdminFirstNonEmpty(r.FormValue("assignOwner"), r.FormValue("assign_owner"), r.FormValue("assignee"), r.FormValue("owner"))
		req.NextFollowUpAt = saasAdminFirstNonEmpty(r.FormValue("nextFollowUpAt"), r.FormValue("next_follow_up_at"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminRenewalForecastAssign{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminRenewalForecastAssign{}, errors.New("JSON 格式错误")
			}
		}
	}
	status, err := normalizeSaaSAdminRiskFollowUpStatus(req.Status)
	if err != nil {
		return SaaSAdminRenewalForecastAssign{}, err
	}
	if status == SaaSAdminRiskFollowUpStatusResolved || status == SaaSAdminRiskFollowUpStatusIgnored {
		return SaaSAdminRenewalForecastAssign{}, errors.New("status 必须是 pending、contacted 或 renewal_pending")
	}
	owner := strings.TrimSpace(saasAdminFirstNonEmpty(req.AssignOwner, req.Assignee, req.Owner))
	if owner == "" {
		return SaaSAdminRenewalForecastAssign{}, errors.New("owner required")
	}
	if len([]rune(owner)) > 80 {
		return SaaSAdminRenewalForecastAssign{}, errors.New("owner too long")
	}
	nextFollowUpAt, err := normalizeSaaSAdminOptionalDateTime(saasAdminFirstNonEmpty(req.NextFollowUpAt, req.NextFollowUpAtSnake))
	if err != nil {
		return SaaSAdminRenewalForecastAssign{}, errors.New("nextFollowUpAt format invalid")
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "续费预测批量分派"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminRenewalForecastAssign{}, errors.New("remark too long")
	}
	return SaaSAdminRenewalForecastAssign{
		Status:         status,
		Owner:          owner,
		NextFollowUpAt: nextFollowUpAt,
		Remark:         remark,
	}, nil
}

type saasAdminCustomerSuccessRenewalTasksRequest struct {
	PackageCode                string `json:"packageCode"`
	PackageCodeSnake           string `json:"package_code"`
	ExpiresAt                  string `json:"expiresAt"`
	ExpiresAtSnake             string `json:"expires_at"`
	Months                     int    `json:"months"`
	RenewMonths                int    `json:"renewMonths"`
	RenewMonthsSnake           int    `json:"renew_months"`
	Amount                     string `json:"amount"`
	AmountCents                int64  `json:"amountCents"`
	AmountCentsSnake           int64  `json:"amount_cents"`
	Currency                   string `json:"currency"`
	PaidAt                     string `json:"paidAt"`
	PaidAtSnake                string `json:"paid_at"`
	PaymentMethod              string `json:"paymentMethod"`
	PaymentMethodSnake         string `json:"payment_method"`
	ExternalOrderNoPrefix      string `json:"externalOrderNoPrefix"`
	ExternalOrderNoPrefixSnake string `json:"external_order_no_prefix"`
	Remark                     string `json:"remark"`
	ForceCreate                *bool  `json:"forceCreate"`
	ForceCreateSnake           *bool  `json:"force_create"`
}

func parseSaaSAdminCustomerSuccessRenewalTasks(r *http.Request) (SaaSAdminCustomerSuccessRenewalTasks, error) {
	req := saasAdminCustomerSuccessRenewalTasksRequest{Months: 12}
	forceCreate := false
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminCustomerSuccessRenewalTasks{}, err
		}
		req.PackageCode = saasAdminFirstNonEmpty(r.FormValue("packageCode"), r.FormValue("package_code"))
		req.ExpiresAt = saasAdminFirstNonEmpty(r.FormValue("expiresAt"), r.FormValue("expires_at"))
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("months"), r.FormValue("renewMonths"), r.FormValue("renew_months"))); raw != "" {
			months, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("months invalid")
			}
			req.Months = months
		}
		req.Amount = r.FormValue("amount")
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("amountCents"), r.FormValue("amount_cents"))); raw != "" {
			amountCents, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("amountCents invalid")
			}
			req.AmountCents = amountCents
		}
		req.Currency = r.FormValue("currency")
		req.PaidAt = saasAdminFirstNonEmpty(r.FormValue("paidAt"), r.FormValue("paid_at"))
		req.PaymentMethod = saasAdminFirstNonEmpty(r.FormValue("paymentMethod"), r.FormValue("payment_method"))
		req.ExternalOrderNoPrefix = saasAdminFirstNonEmpty(r.FormValue("externalOrderNoPrefix"), r.FormValue("external_order_no_prefix"))
		req.Remark = r.FormValue("remark")
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("forceCreate"), r.FormValue("force_create"))); raw != "" {
			value, err := parseSaaSAdminRequestBool(raw, "forceCreate")
			if err != nil {
				return SaaSAdminCustomerSuccessRenewalTasks{}, err
			}
			forceCreate = value
		}
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminCustomerSuccessRenewalTasks{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("JSON 格式错误")
			}
		}
		if req.RenewMonths > 0 {
			req.Months = req.RenewMonths
		}
		if req.RenewMonthsSnake > 0 {
			req.Months = req.RenewMonthsSnake
		}
		if req.AmountCents == 0 && req.AmountCentsSnake != 0 {
			req.AmountCents = req.AmountCentsSnake
		}
		if req.ForceCreate != nil {
			forceCreate = *req.ForceCreate
		} else if req.ForceCreateSnake != nil {
			forceCreate = *req.ForceCreateSnake
		}
	}
	packageCode := strings.TrimSpace(saasAdminFirstNonEmpty(req.PackageCode, req.PackageCodeSnake))
	if packageCode != "" {
		if !validSaaSAdminPackageCode(packageCode) {
			return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("packageCode 仅支持字母、数字、下划线和中划线")
		}
		if len([]rune(packageCode)) > 64 {
			return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("packageCode too long")
		}
	}
	expiresAt, err := normalizeSaaSAdminExpiresAt(saasAdminFirstNonEmpty(req.ExpiresAt, req.ExpiresAtSnake))
	if err != nil {
		return SaaSAdminCustomerSuccessRenewalTasks{}, err
	}
	months := req.Months
	if months <= 0 {
		if expiresAt != "" {
			months = 0
		} else {
			return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("months must be positive")
		}
	}
	if months > 120 {
		return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("months must be <= 120")
	}
	amountCents := req.AmountCents
	if strings.TrimSpace(req.Amount) != "" {
		amountCents, err = parseSaaSAdminAmountCents(req.Amount)
		if err != nil {
			return SaaSAdminCustomerSuccessRenewalTasks{}, err
		}
	}
	if amountCents < 0 {
		return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("amountCents must not be negative")
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "CNY"
	}
	if len(currency) != 3 {
		return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("currency must be 3 letters")
	}
	paidAt, err := normalizeSaaSAdminOptionalDateTime(saasAdminFirstNonEmpty(req.PaidAt, req.PaidAtSnake))
	if err != nil {
		return SaaSAdminCustomerSuccessRenewalTasks{}, err
	}
	paymentMethod := strings.TrimSpace(saasAdminFirstNonEmpty(req.PaymentMethod, req.PaymentMethodSnake))
	if len([]rune(paymentMethod)) > 64 {
		return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("paymentMethod too long")
	}
	externalOrderNoPrefix := strings.TrimSpace(saasAdminFirstNonEmpty(req.ExternalOrderNoPrefix, req.ExternalOrderNoPrefixSnake))
	if len([]rune(externalOrderNoPrefix)) > 100 {
		return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("externalOrderNoPrefix too long")
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "客户成功队列批量生成续费任务"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminCustomerSuccessRenewalTasks{}, errors.New("remark too long")
	}
	return SaaSAdminCustomerSuccessRenewalTasks{
		PackageCode:           packageCode,
		ExpiresAt:             expiresAt,
		Months:                months,
		AmountCents:           amountCents,
		Currency:              currency,
		PaidAt:                paidAt,
		PaymentMethod:         paymentMethod,
		ExternalOrderNoPrefix: externalOrderNoPrefix,
		Remark:                remark,
		ForceCreate:           forceCreate,
	}, nil
}

func parseSaaSAdminRenewalForecastTasks(r *http.Request) (SaaSAdminRenewalForecastTasks, error) {
	parsed, err := parseSaaSAdminCustomerSuccessRenewalTasks(r)
	if err != nil {
		return SaaSAdminRenewalForecastTasks{}, err
	}
	remark := parsed.Remark
	if remark == "客户成功队列批量生成续费任务" {
		remark = "续费预测批量生成续费任务"
	}
	return SaaSAdminRenewalForecastTasks{
		PackageCode:           parsed.PackageCode,
		ExpiresAt:             parsed.ExpiresAt,
		Months:                parsed.Months,
		AmountCents:           parsed.AmountCents,
		Currency:              parsed.Currency,
		PaidAt:                parsed.PaidAt,
		PaymentMethod:         parsed.PaymentMethod,
		ExternalOrderNoPrefix: parsed.ExternalOrderNoPrefix,
		Remark:                remark,
		ForceCreate:           parsed.ForceCreate,
	}, nil
}

type saasAdminCustomerSuccessRenewalNotificationsRequest struct {
	Channel           string `json:"channel"`
	ReminderDays      int    `json:"reminderDays"`
	ReminderDaysSnake int    `json:"reminder_days"`
	MaxAttempts       int    `json:"maxAttempts"`
	MaxAttemptsSnake  int    `json:"max_attempts"`
	Remark            string `json:"remark"`
	ForceCreate       *bool  `json:"forceCreate"`
	ForceCreateSnake  *bool  `json:"force_create"`
}

type saasAdminTaskSLANotificationsRequest struct {
	Channel          string `json:"channel"`
	MaxAttempts      int    `json:"maxAttempts"`
	MaxAttemptsSnake int    `json:"max_attempts"`
	SLAStatus        string `json:"slaStatus"`
	SLAStatusSnake   string `json:"sla_status"`
	Remark           string `json:"remark"`
	ForceCreate      *bool  `json:"forceCreate"`
	ForceCreateSnake *bool  `json:"force_create"`
}

func parseSaaSAdminTaskSLANotifications(r *http.Request, options SaaSAdminTaskSLAOptions) (SaaSAdminTaskSLANotifications, error) {
	req := saasAdminTaskSLANotificationsRequest{
		Channel:     SaaSAlertNotificationChannelWebhook,
		MaxAttempts: 3,
		SLAStatus:   "warning",
	}
	forceCreate := false
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminTaskSLANotifications{}, err
		}
		req.Channel = saasAdminFirstNonEmpty(r.FormValue("channel"), req.Channel)
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("maxAttempts"), r.FormValue("max_attempts"))); raw != "" {
			maxAttempts, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminTaskSLANotifications{}, errors.New("maxAttempts invalid")
			}
			req.MaxAttempts = maxAttempts
		}
		req.SLAStatus = saasAdminFirstNonEmpty(r.FormValue("slaStatus"), r.FormValue("sla_status"), req.SLAStatus)
		req.Remark = r.FormValue("remark")
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("forceCreate"), r.FormValue("force_create"))); raw != "" {
			value, err := parseSaaSAdminRequestBool(raw, "forceCreate")
			if err != nil {
				return SaaSAdminTaskSLANotifications{}, err
			}
			forceCreate = value
		}
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminTaskSLANotifications{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminTaskSLANotifications{}, errors.New("JSON 格式错误")
			}
		}
		if req.MaxAttemptsSnake != 0 {
			req.MaxAttempts = req.MaxAttemptsSnake
		}
		if req.SLAStatusSnake != "" {
			req.SLAStatus = req.SLAStatusSnake
		}
		if req.ForceCreate != nil {
			forceCreate = *req.ForceCreate
		} else if req.ForceCreateSnake != nil {
			forceCreate = *req.ForceCreateSnake
		}
	}
	req.SLAStatus = saasAdminFirstNonEmpty(r.URL.Query().Get("slaStatus"), r.URL.Query().Get("sla_status"), req.SLAStatus)
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	if channel != SaaSAlertNotificationChannelWebhook {
		return SaaSAdminTaskSLANotifications{}, errors.New("channel must be webhook")
	}
	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if maxAttempts > 20 {
		return SaaSAdminTaskSLANotifications{}, errors.New("maxAttempts must be <= 20")
	}
	slaStatus, err := normalizeSaaSAdminTaskSLANotificationStatus(req.SLAStatus)
	if err != nil {
		return SaaSAdminTaskSLANotifications{}, err
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "运营任务SLA催办"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminTaskSLANotifications{}, errors.New("remark too long")
	}
	return SaaSAdminTaskSLANotifications{
		Options:     options,
		Channel:     channel,
		MaxAttempts: maxAttempts,
		SLAStatus:   slaStatus,
		Remark:      remark,
		ForceCreate: forceCreate,
	}, nil
}

func normalizeSaaSAdminTaskSLANotificationStatus(raw string) (string, error) {
	status := strings.ToLower(strings.TrimSpace(raw))
	status = strings.ReplaceAll(status, "-", "_")
	switch status {
	case "", "warning", "warn", "warning_or_overdue", "actionable":
		return "warning", nil
	case "overdue":
		return "overdue", nil
	case "fresh":
		return "fresh", nil
	case "unknown":
		return "unknown", nil
	case "all", "active":
		return "all", nil
	default:
		return "", errors.New("slaStatus must be warning、overdue、fresh、unknown 或 all")
	}
}

func parseSaaSAdminCustomerSuccessRenewalNotifications(r *http.Request, options SaaSAdminCustomerSuccessOptions) (SaaSAdminCustomerSuccessRenewalNotifications, error) {
	req := saasAdminCustomerSuccessRenewalNotificationsRequest{
		Channel:      SaaSAlertNotificationChannelWebhook,
		MaxAttempts:  3,
		ReminderDays: options.ExpiringDays,
	}
	forceCreate := false
	if req.ReminderDays <= 0 {
		req.ReminderDays = 30
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminCustomerSuccessRenewalNotifications{}, err
		}
		req.Channel = saasAdminFirstNonEmpty(r.FormValue("channel"), req.Channel)
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("reminderDays"), r.FormValue("reminder_days"))); raw != "" {
			reminderDays, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminCustomerSuccessRenewalNotifications{}, errors.New("reminderDays invalid")
			}
			req.ReminderDays = reminderDays
		}
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("maxAttempts"), r.FormValue("max_attempts"))); raw != "" {
			maxAttempts, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminCustomerSuccessRenewalNotifications{}, errors.New("maxAttempts invalid")
			}
			req.MaxAttempts = maxAttempts
		}
		req.Remark = r.FormValue("remark")
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("forceCreate"), r.FormValue("force_create"))); raw != "" {
			value, err := parseSaaSAdminRequestBool(raw, "forceCreate")
			if err != nil {
				return SaaSAdminCustomerSuccessRenewalNotifications{}, err
			}
			forceCreate = value
		}
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminCustomerSuccessRenewalNotifications{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminCustomerSuccessRenewalNotifications{}, errors.New("JSON 格式错误")
			}
		}
		if req.ReminderDaysSnake != 0 {
			req.ReminderDays = req.ReminderDaysSnake
		}
		if req.MaxAttemptsSnake != 0 {
			req.MaxAttempts = req.MaxAttemptsSnake
		}
		if req.ForceCreate != nil {
			forceCreate = *req.ForceCreate
		} else if req.ForceCreateSnake != nil {
			forceCreate = *req.ForceCreateSnake
		}
	}
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	if channel != SaaSAlertNotificationChannelWebhook {
		return SaaSAdminCustomerSuccessRenewalNotifications{}, errors.New("channel must be webhook")
	}
	reminderDays := req.ReminderDays
	if reminderDays <= 0 {
		return SaaSAdminCustomerSuccessRenewalNotifications{}, errors.New("reminderDays must be positive")
	}
	if reminderDays > 3660 {
		return SaaSAdminCustomerSuccessRenewalNotifications{}, errors.New("reminderDays must be <= 3660")
	}
	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if maxAttempts > 20 {
		return SaaSAdminCustomerSuccessRenewalNotifications{}, errors.New("maxAttempts must be <= 20")
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "客户成功队列批量生成续费提醒"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminCustomerSuccessRenewalNotifications{}, errors.New("remark too long")
	}
	return SaaSAdminCustomerSuccessRenewalNotifications{
		Channel:      channel,
		MaxAttempts:  maxAttempts,
		ReminderDays: reminderDays,
		Remark:       remark,
		ForceCreate:  forceCreate,
	}, nil
}

func parseSaaSAdminRenewalForecastNotifications(r *http.Request, options SaaSAdminRenewalForecastOptions) (SaaSAdminRenewalForecastNotifications, error) {
	req := saasAdminCustomerSuccessRenewalNotificationsRequest{
		Channel:      SaaSAlertNotificationChannelWebhook,
		MaxAttempts:  3,
		ReminderDays: options.Days,
	}
	forceCreate := false
	if req.ReminderDays <= 0 {
		req.ReminderDays = 30
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminRenewalForecastNotifications{}, err
		}
		req.Channel = saasAdminFirstNonEmpty(r.FormValue("channel"), req.Channel)
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("reminderDays"), r.FormValue("reminder_days"))); raw != "" {
			reminderDays, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminRenewalForecastNotifications{}, errors.New("reminderDays invalid")
			}
			req.ReminderDays = reminderDays
		}
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("maxAttempts"), r.FormValue("max_attempts"))); raw != "" {
			maxAttempts, err := strconv.Atoi(raw)
			if err != nil {
				return SaaSAdminRenewalForecastNotifications{}, errors.New("maxAttempts invalid")
			}
			req.MaxAttempts = maxAttempts
		}
		req.Remark = r.FormValue("remark")
		if raw := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("forceCreate"), r.FormValue("force_create"))); raw != "" {
			value, err := parseSaaSAdminRequestBool(raw, "forceCreate")
			if err != nil {
				return SaaSAdminRenewalForecastNotifications{}, err
			}
			forceCreate = value
		}
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminRenewalForecastNotifications{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminRenewalForecastNotifications{}, errors.New("JSON 格式错误")
			}
		}
		if req.ReminderDaysSnake != 0 {
			req.ReminderDays = req.ReminderDaysSnake
		}
		if req.MaxAttemptsSnake != 0 {
			req.MaxAttempts = req.MaxAttemptsSnake
		}
		if req.ForceCreate != nil {
			forceCreate = *req.ForceCreate
		} else if req.ForceCreateSnake != nil {
			forceCreate = *req.ForceCreateSnake
		}
	}
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = SaaSAlertNotificationChannelWebhook
	}
	if channel != SaaSAlertNotificationChannelWebhook {
		return SaaSAdminRenewalForecastNotifications{}, errors.New("channel must be webhook")
	}
	reminderDays := req.ReminderDays
	if reminderDays <= 0 {
		return SaaSAdminRenewalForecastNotifications{}, errors.New("reminderDays must be positive")
	}
	if reminderDays > 3660 {
		return SaaSAdminRenewalForecastNotifications{}, errors.New("reminderDays must be <= 3660")
	}
	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if maxAttempts > 20 {
		return SaaSAdminRenewalForecastNotifications{}, errors.New("maxAttempts must be <= 20")
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "续费预测批量生成续费提醒"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminRenewalForecastNotifications{}, errors.New("remark too long")
	}
	return SaaSAdminRenewalForecastNotifications{
		Channel:      channel,
		MaxAttempts:  maxAttempts,
		ReminderDays: reminderDays,
		Remark:       remark,
		ForceCreate:  forceCreate,
	}, nil
}

func parseSaaSAdminRiskFollowUp(r *http.Request) (SaaSAdminRiskFollowUp, error) {
	var req saasAdminRiskFollowUpRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminRiskFollowUp{}, err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))))
		req.Status = r.FormValue("status")
		req.Owner = r.FormValue("owner")
		req.NextFollowUpAt = saasAdminFirstNonEmpty(r.FormValue("nextFollowUpAt"), r.FormValue("next_follow_up_at"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminRiskFollowUp{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminRiskFollowUp{}, errors.New("JSON 格式错误")
			}
		}
	}
	if req.TenantID <= 0 {
		return SaaSAdminRiskFollowUp{}, errors.New("tenantId required")
	}
	status, err := normalizeSaaSAdminRiskFollowUpStatus(req.Status)
	if err != nil {
		return SaaSAdminRiskFollowUp{}, err
	}
	owner := strings.TrimSpace(req.Owner)
	if len([]rune(owner)) > 80 {
		return SaaSAdminRiskFollowUp{}, errors.New("owner too long")
	}
	nextFollowUpAt, err := normalizeSaaSAdminOptionalDateTime(req.NextFollowUpAt)
	if err != nil {
		return SaaSAdminRiskFollowUp{}, errors.New("nextFollowUpAt format invalid")
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return SaaSAdminRiskFollowUp{}, errors.New("remark too long")
	}
	return SaaSAdminRiskFollowUp{
		TenantID:       req.TenantID,
		Status:         status,
		Owner:          owner,
		NextFollowUpAt: nextFollowUpAt,
		Remark:         remark,
	}, nil
}

type saasAdminBillingReconciliationFollowUpRequest struct {
	BillingEventID      int64  `json:"billingEventId"`
	BillingEventIDSnake int64  `json:"billing_event_id"`
	EventID             int64  `json:"eventId"`
	ID                  int64  `json:"id"`
	Status              string `json:"status"`
	Owner               string `json:"owner"`
	NextFollowUpAt      string `json:"nextFollowUpAt"`
	Remark              string `json:"remark"`
}

func parseSaaSAdminBillingReconciliationFollowUp(r *http.Request) (SaaSAdminBillingReconciliationFollowUp, error) {
	var req saasAdminBillingReconciliationFollowUpRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminBillingReconciliationFollowUp{}, err
		}
		req.BillingEventID, _ = strconv.ParseInt(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("billingEventId"), r.FormValue("billing_event_id"), r.FormValue("eventId"), r.FormValue("id"))), 10, 64)
		req.Status = r.FormValue("status")
		req.Owner = r.FormValue("owner")
		req.NextFollowUpAt = saasAdminFirstNonEmpty(r.FormValue("nextFollowUpAt"), r.FormValue("next_follow_up_at"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminBillingReconciliationFollowUp{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminBillingReconciliationFollowUp{}, errors.New("JSON 格式错误")
			}
		}
		if req.BillingEventID <= 0 {
			req.BillingEventID = req.BillingEventIDSnake
		}
		if req.BillingEventID <= 0 {
			req.BillingEventID = req.EventID
		}
		if req.BillingEventID <= 0 {
			req.BillingEventID = req.ID
		}
	}
	if req.BillingEventID <= 0 {
		return SaaSAdminBillingReconciliationFollowUp{}, errors.New("billingEventId required")
	}
	statusRaw := req.Status
	if strings.TrimSpace(statusRaw) == "" {
		statusRaw = SaaSAdminRiskFollowUpStatusContacted
	}
	status, err := normalizeSaaSAdminRiskFollowUpStatus(statusRaw)
	if err != nil {
		return SaaSAdminBillingReconciliationFollowUp{}, err
	}
	owner := strings.TrimSpace(req.Owner)
	if len([]rune(owner)) > 80 {
		return SaaSAdminBillingReconciliationFollowUp{}, errors.New("owner too long")
	}
	nextFollowUpAt, err := normalizeSaaSAdminOptionalDateTime(req.NextFollowUpAt)
	if err != nil {
		return SaaSAdminBillingReconciliationFollowUp{}, errors.New("nextFollowUpAt format invalid")
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "账单对账跟进"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminBillingReconciliationFollowUp{}, errors.New("remark too long")
	}
	return SaaSAdminBillingReconciliationFollowUp{
		BillingEventID: req.BillingEventID,
		Status:         status,
		Owner:          owner,
		NextFollowUpAt: nextFollowUpAt,
		Remark:         remark,
	}, nil
}

type saasAdminBillingReconciliationFollowUpBulkCloseRequest struct {
	TenantID     int    `json:"tenantId"`
	FilterStatus string `json:"filterStatus"`
	Owner        string `json:"owner"`
	DueState     string `json:"dueState"`
	Keyword      string `json:"keyword"`
	Limit        int    `json:"limit"`
	Status       string `json:"status"`
	CloseStatus  string `json:"closeStatus"`
	Remark       string `json:"remark"`
}

func parseSaaSAdminBillingReconciliationFollowUpBulkClose(r *http.Request) (SaaSAdminBillingReconciliationFollowUpBulkClose, error) {
	req := saasAdminBillingReconciliationFollowUpBulkCloseRequest{
		TenantID:     saasAdminQueryInt(r, "tenantId", 0),
		FilterStatus: r.URL.Query().Get("status"),
		Owner:        r.URL.Query().Get("owner"),
		DueState:     r.URL.Query().Get("dueState"),
		Keyword:      r.URL.Query().Get("keyword"),
		Limit:        positiveQueryInt(r, "limit", 50),
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminBillingReconciliationFollowUpBulkClose{}, err
		}
		if value := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))); value != "" {
			req.TenantID, _ = strconv.Atoi(value)
		}
		req.FilterStatus = saasAdminFirstNonEmpty(r.FormValue("filterStatus"), r.FormValue("filter_status"), r.FormValue("taskStatus"), r.FormValue("task_status"), req.FilterStatus)
		req.Owner = saasAdminFirstNonEmpty(r.FormValue("owner"), req.Owner)
		req.DueState = saasAdminFirstNonEmpty(r.FormValue("dueState"), r.FormValue("due_state"), req.DueState)
		req.Keyword = saasAdminFirstNonEmpty(r.FormValue("keyword"), req.Keyword)
		if value := strings.TrimSpace(r.FormValue("limit")); value != "" {
			req.Limit, _ = strconv.Atoi(value)
		}
		req.Status = r.FormValue("status")
		req.CloseStatus = saasAdminFirstNonEmpty(r.FormValue("closeStatus"), r.FormValue("close_status"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminBillingReconciliationFollowUpBulkClose{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminBillingReconciliationFollowUpBulkClose{}, errors.New("JSON 格式错误")
			}
		}
	}

	filterStatus := strings.TrimSpace(req.FilterStatus)
	if filterStatus != "" && strings.EqualFold(filterStatus, "all") {
		filterStatus = ""
	}
	if filterStatus != "" {
		normalized, err := normalizeSaaSAdminRiskFollowUpStatus(filterStatus)
		if err != nil {
			return SaaSAdminBillingReconciliationFollowUpBulkClose{}, err
		}
		filterStatus = normalized
	}
	dueState, err := normalizeSaaSAdminRiskFollowUpDueState(req.DueState)
	if err != nil {
		return SaaSAdminBillingReconciliationFollowUpBulkClose{}, err
	}
	owner := strings.TrimSpace(req.Owner)
	if len([]rune(owner)) > 80 {
		return SaaSAdminBillingReconciliationFollowUpBulkClose{}, errors.New("owner too long")
	}
	keyword := strings.TrimSpace(req.Keyword)
	if len([]rune(keyword)) > 80 {
		return SaaSAdminBillingReconciliationFollowUpBulkClose{}, errors.New("keyword too long")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	statusRaw := saasAdminFirstNonEmpty(req.CloseStatus, req.Status, SaaSAdminRiskFollowUpStatusResolved)
	status, err := normalizeSaaSAdminRiskFollowUpStatus(statusRaw)
	if err != nil {
		return SaaSAdminBillingReconciliationFollowUpBulkClose{}, err
	}
	if status != SaaSAdminRiskFollowUpStatusResolved && status != SaaSAdminRiskFollowUpStatusIgnored {
		return SaaSAdminBillingReconciliationFollowUpBulkClose{}, errors.New("closeStatus 必须是 resolved 或 ignored")
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "批量关闭账单跟进任务"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminBillingReconciliationFollowUpBulkClose{}, errors.New("remark too long")
	}
	return SaaSAdminBillingReconciliationFollowUpBulkClose{
		Options: SaaSAdminBillingReconciliationFollowUpOptions{
			TenantID: req.TenantID,
			Status:   filterStatus,
			Owner:    owner,
			DueState: dueState,
			Keyword:  keyword,
			Limit:    limit,
		},
		Status: status,
		Remark: remark,
	}, nil
}

type saasAdminRiskFollowUpBulkCloseRequest struct {
	TenantID     int    `json:"tenantId"`
	FilterStatus string `json:"filterStatus"`
	Owner        string `json:"owner"`
	DueState     string `json:"dueState"`
	Keyword      string `json:"keyword"`
	Limit        int    `json:"limit"`
	Status       string `json:"status"`
	CloseStatus  string `json:"closeStatus"`
	Remark       string `json:"remark"`
}

func parseSaaSAdminRiskFollowUpBulkClose(r *http.Request) (SaaSAdminRiskFollowUpBulkClose, error) {
	req := saasAdminRiskFollowUpBulkCloseRequest{
		TenantID:     saasAdminQueryInt(r, "tenantId", 0),
		FilterStatus: r.URL.Query().Get("status"),
		Owner:        r.URL.Query().Get("owner"),
		DueState:     r.URL.Query().Get("dueState"),
		Keyword:      r.URL.Query().Get("keyword"),
		Limit:        positiveQueryInt(r, "limit", 50),
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminRiskFollowUpBulkClose{}, err
		}
		if value := strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))); value != "" {
			req.TenantID, _ = strconv.Atoi(value)
		}
		req.FilterStatus = saasAdminFirstNonEmpty(r.FormValue("filterStatus"), r.FormValue("filter_status"), r.FormValue("taskStatus"), r.FormValue("task_status"), req.FilterStatus)
		req.Owner = saasAdminFirstNonEmpty(r.FormValue("owner"), req.Owner)
		req.DueState = saasAdminFirstNonEmpty(r.FormValue("dueState"), r.FormValue("due_state"), req.DueState)
		req.Keyword = saasAdminFirstNonEmpty(r.FormValue("keyword"), req.Keyword)
		if value := strings.TrimSpace(r.FormValue("limit")); value != "" {
			req.Limit, _ = strconv.Atoi(value)
		}
		req.Status = r.FormValue("status")
		req.CloseStatus = saasAdminFirstNonEmpty(r.FormValue("closeStatus"), r.FormValue("close_status"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminRiskFollowUpBulkClose{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminRiskFollowUpBulkClose{}, errors.New("JSON 格式错误")
			}
		}
	}

	filterStatus := strings.TrimSpace(req.FilterStatus)
	if filterStatus != "" && strings.EqualFold(filterStatus, "all") {
		filterStatus = ""
	}
	if filterStatus != "" {
		normalized, err := normalizeSaaSAdminRiskFollowUpStatus(filterStatus)
		if err != nil {
			return SaaSAdminRiskFollowUpBulkClose{}, err
		}
		filterStatus = normalized
	}
	dueState, err := normalizeSaaSAdminRiskFollowUpDueState(req.DueState)
	if err != nil {
		return SaaSAdminRiskFollowUpBulkClose{}, err
	}
	owner := strings.TrimSpace(req.Owner)
	if len([]rune(owner)) > 80 {
		return SaaSAdminRiskFollowUpBulkClose{}, errors.New("owner too long")
	}
	keyword := strings.TrimSpace(req.Keyword)
	if len([]rune(keyword)) > 80 {
		return SaaSAdminRiskFollowUpBulkClose{}, errors.New("keyword too long")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	statusRaw := saasAdminFirstNonEmpty(req.CloseStatus, req.Status, SaaSAdminRiskFollowUpStatusResolved)
	status, err := normalizeSaaSAdminRiskFollowUpStatus(statusRaw)
	if err != nil {
		return SaaSAdminRiskFollowUpBulkClose{}, err
	}
	if status != SaaSAdminRiskFollowUpStatusResolved && status != SaaSAdminRiskFollowUpStatusIgnored {
		return SaaSAdminRiskFollowUpBulkClose{}, errors.New("closeStatus 必须是 resolved 或 ignored")
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "批量关闭风险跟进任务"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminRiskFollowUpBulkClose{}, errors.New("remark too long")
	}
	return SaaSAdminRiskFollowUpBulkClose{
		Options: SaaSAdminRiskFollowUpTaskOptions{
			TenantID: req.TenantID,
			Status:   filterStatus,
			Owner:    owner,
			DueState: dueState,
			Keyword:  keyword,
			Limit:    limit,
		},
		Status: status,
		Remark: remark,
	}, nil
}

type saasAdminTenantRenewalRequest struct {
	TenantID        int    `json:"tenantId"`
	PackageCode     string `json:"packageCode"`
	ExpiresAt       string `json:"expiresAt"`
	Amount          string `json:"amount"`
	AmountCents     int64  `json:"amountCents"`
	Currency        string `json:"currency"`
	PaidAt          string `json:"paidAt"`
	PaymentMethod   string `json:"paymentMethod"`
	ExternalOrderNo string `json:"externalOrderNo"`
	Remark          string `json:"remark"`
}

func parseSaaSAdminTenantRenewal(r *http.Request) (SaaSAdminTenantRenewal, error) {
	var req saasAdminTenantRenewalRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminTenantRenewal{}, err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))))
		req.PackageCode = saasAdminFirstNonEmpty(r.FormValue("packageCode"), r.FormValue("package_code"))
		req.ExpiresAt = saasAdminFirstNonEmpty(r.FormValue("expiresAt"), r.FormValue("expires_at"))
		req.Amount = r.FormValue("amount")
		req.AmountCents, _ = strconv.ParseInt(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("amountCents"), r.FormValue("amount_cents"))), 10, 64)
		req.Currency = r.FormValue("currency")
		req.PaidAt = saasAdminFirstNonEmpty(r.FormValue("paidAt"), r.FormValue("paid_at"))
		req.PaymentMethod = saasAdminFirstNonEmpty(r.FormValue("paymentMethod"), r.FormValue("payment_method"))
		req.ExternalOrderNo = saasAdminFirstNonEmpty(r.FormValue("externalOrderNo"), r.FormValue("external_order_no"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminTenantRenewal{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminTenantRenewal{}, errors.New("JSON 格式错误")
			}
		}
	}
	return normalizeSaaSAdminTenantRenewalRequest(req)
}

func normalizeSaaSAdminTenantRenewalRequest(req saasAdminTenantRenewalRequest) (SaaSAdminTenantRenewal, error) {
	if req.TenantID <= 0 {
		return SaaSAdminTenantRenewal{}, errors.New("tenantId required")
	}
	req.PackageCode = strings.TrimSpace(req.PackageCode)
	expiresAt, err := normalizeSaaSAdminExpiresAt(req.ExpiresAt)
	if err != nil {
		return SaaSAdminTenantRenewal{}, err
	}
	if expiresAt == "" {
		return SaaSAdminTenantRenewal{}, errors.New("expiresAt required")
	}
	amountCents := req.AmountCents
	if strings.TrimSpace(req.Amount) != "" {
		amountCents, err = parseSaaSAdminAmountCents(req.Amount)
		if err != nil {
			return SaaSAdminTenantRenewal{}, err
		}
	}
	if amountCents < 0 {
		return SaaSAdminTenantRenewal{}, errors.New("amountCents must not be negative")
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "CNY"
	}
	if len(currency) != 3 {
		return SaaSAdminTenantRenewal{}, errors.New("currency must be 3 letters")
	}
	paidAt, err := normalizeSaaSAdminOptionalDateTime(req.PaidAt)
	if err != nil {
		return SaaSAdminTenantRenewal{}, err
	}
	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	if len([]rune(paymentMethod)) > 64 {
		return SaaSAdminTenantRenewal{}, errors.New("paymentMethod too long")
	}
	externalOrderNo := strings.TrimSpace(req.ExternalOrderNo)
	if len([]rune(externalOrderNo)) > 128 {
		return SaaSAdminTenantRenewal{}, errors.New("externalOrderNo too long")
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return SaaSAdminTenantRenewal{}, errors.New("remark too long")
	}
	return SaaSAdminTenantRenewal{
		TenantID:        req.TenantID,
		PackageCode:     req.PackageCode,
		ExpiresAt:       expiresAt,
		AmountCents:     amountCents,
		Currency:        currency,
		PaidAt:          paidAt,
		PaymentMethod:   paymentMethod,
		ExternalOrderNo: externalOrderNo,
		Remark:          remark,
	}, nil
}

type saasAdminTenantProvisionRequest struct {
	TenantID          int    `json:"tenantId"`
	TenantName        string `json:"tenantName"`
	AdminPhone        string `json:"adminPhone"`
	Phone             string `json:"phone"`
	AdminName         string `json:"adminName"`
	UserName          string `json:"userName"`
	Password          string `json:"password"`
	AdminPasswordHash string `json:"adminPasswordHash"`
	RoleName          string `json:"roleName"`
	PackageCode       string `json:"packageCode"`
	ExpiresAt         string `json:"expiresAt"`
	ConfigCopyMode    string `json:"configCopyMode"`
	Remark            string `json:"remark"`
}

func parseSaaSAdminTenantProvision(r *http.Request) (SaaSAdminTenantProvision, string, error) {
	var req saasAdminTenantProvisionRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminTenantProvision{}, "", err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))))
		req.TenantName = saasAdminFirstNonEmpty(r.FormValue("tenantName"), r.FormValue("tenant_name"), r.FormValue("name"))
		req.AdminPhone = saasAdminFirstNonEmpty(r.FormValue("adminPhone"), r.FormValue("admin_phone"), r.FormValue("phone"))
		req.AdminName = saasAdminFirstNonEmpty(r.FormValue("adminName"), r.FormValue("admin_name"), r.FormValue("userName"), r.FormValue("user_name"))
		req.Password = r.FormValue("password")
		req.RoleName = saasAdminFirstNonEmpty(r.FormValue("roleName"), r.FormValue("role_name"))
		req.PackageCode = saasAdminFirstNonEmpty(r.FormValue("packageCode"), r.FormValue("package_code"))
		req.ExpiresAt = saasAdminFirstNonEmpty(r.FormValue("expiresAt"), r.FormValue("expires_at"))
		req.ConfigCopyMode = saasAdminFirstNonEmpty(r.FormValue("configCopyMode"), r.FormValue("config_copy_mode"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminTenantProvision{}, "", err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminTenantProvision{}, "", errors.New("JSON 格式错误")
			}
		}
	}
	tenantName := strings.TrimSpace(req.TenantName)
	if tenantName == "" {
		return SaaSAdminTenantProvision{}, "", errors.New("tenantName required")
	}
	if len([]rune(tenantName)) > 255 {
		return SaaSAdminTenantProvision{}, "", errors.New("tenantName too long")
	}
	adminPhone := strings.TrimSpace(saasAdminFirstNonEmpty(req.AdminPhone, req.Phone))
	if len(adminPhone) != 11 || !validPhone(adminPhone) {
		return SaaSAdminTenantProvision{}, "", errors.New("adminPhone invalid")
	}
	password := strings.TrimSpace(req.Password)
	if !validAlphaNumPassword(password) {
		return SaaSAdminTenantProvision{}, "", errors.New("password must be letters or numbers")
	}
	adminName := strings.TrimSpace(saasAdminFirstNonEmpty(req.AdminName, req.UserName))
	if adminName == "" {
		adminName = "超级管理员"
	}
	if len([]rune(adminName)) > 255 {
		return SaaSAdminTenantProvision{}, "", errors.New("adminName too long")
	}
	roleName := strings.TrimSpace(req.RoleName)
	if roleName == "" {
		roleName = "超级管理员"
	}
	if len([]rune(roleName)) > 255 {
		return SaaSAdminTenantProvision{}, "", errors.New("roleName too long")
	}
	packageCode := strings.TrimSpace(req.PackageCode)
	if packageCode == "" {
		return SaaSAdminTenantProvision{}, "", errors.New("packageCode required")
	}
	expiresAt, err := normalizeSaaSAdminExpiresAt(req.ExpiresAt)
	if err != nil {
		return SaaSAdminTenantProvision{}, "", err
	}
	configCopyMode := strings.ToLower(strings.TrimSpace(req.ConfigCopyMode))
	if configCopyMode == "" {
		configCopyMode = "missing"
	}
	if configCopyMode != "missing" && configCopyMode != "overwrite" && configCopyMode != "skip" {
		return SaaSAdminTenantProvision{}, "", errors.New("configCopyMode must be missing, overwrite, or skip")
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return SaaSAdminTenantProvision{}, "", errors.New("remark too long")
	}
	return SaaSAdminTenantProvision{
		TenantID:       req.TenantID,
		TenantName:     tenantName,
		AdminPhone:     adminPhone,
		AdminName:      adminName,
		RoleName:       roleName,
		PackageCode:    packageCode,
		ExpiresAt:      expiresAt,
		ConfigCopyMode: configCopyMode,
		Remark:         remark,
	}, password, nil
}

type saasAdminAlertResolveRequest struct {
	TenantID  int    `json:"tenantId"`
	Metric    string `json:"metric"`
	AlertType string `json:"alertType"`
	PeriodKey string `json:"periodKey"`
	Remark    string `json:"remark"`
}

func parseSaaSAdminAlertResolve(r *http.Request) (SaaSAdminAlertResolve, error) {
	var req saasAdminAlertResolveRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminAlertResolve{}, err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))))
		req.Metric = r.FormValue("metric")
		req.AlertType = saasAdminFirstNonEmpty(r.FormValue("alertType"), r.FormValue("alert_type"))
		req.PeriodKey = saasAdminFirstNonEmpty(r.FormValue("periodKey"), r.FormValue("period_key"))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminAlertResolve{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminAlertResolve{}, errors.New("JSON 格式错误")
			}
		}
	}
	metric := strings.TrimSpace(req.Metric)
	if metric == "" {
		return SaaSAdminAlertResolve{}, errors.New("metric required")
	}
	if len([]rune(metric)) > 64 {
		return SaaSAdminAlertResolve{}, errors.New("metric too long")
	}
	alertType := strings.TrimSpace(req.AlertType)
	if alertType == "" {
		alertType = SaaSAlertTypeQuotaExceeded
	}
	if len([]rune(alertType)) > 64 {
		return SaaSAdminAlertResolve{}, errors.New("alertType too long")
	}
	periodKey := strings.TrimSpace(req.PeriodKey)
	if periodKey == "" {
		periodKey = SaaSAlertPeriodLifetime
	}
	if len([]rune(periodKey)) > 32 {
		return SaaSAdminAlertResolve{}, errors.New("periodKey too long")
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return SaaSAdminAlertResolve{}, errors.New("remark too long")
	}
	return SaaSAdminAlertResolve{
		TenantID:  req.TenantID,
		Metric:    metric,
		AlertType: alertType,
		PeriodKey: periodKey,
		Remark:    remark,
	}, nil
}

type saasAdminAlertBulkResolveRequest struct {
	TenantID  int    `json:"tenantId"`
	Metric    string `json:"metric"`
	AlertType string `json:"alertType"`
	PeriodKey string `json:"periodKey"`
	Limit     int    `json:"limit"`
	Remark    string `json:"remark"`
}

func parseSaaSAdminAlertBulkResolve(r *http.Request) (SaaSAdminAlertBulkResolve, error) {
	var req saasAdminAlertBulkResolveRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminAlertBulkResolve{}, err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))))
		req.Metric = r.FormValue("metric")
		req.AlertType = saasAdminFirstNonEmpty(r.FormValue("alertType"), r.FormValue("alert_type"))
		req.PeriodKey = saasAdminFirstNonEmpty(r.FormValue("periodKey"), r.FormValue("period_key"))
		req.Limit, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("limit")))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminAlertBulkResolve{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminAlertBulkResolve{}, errors.New("JSON 格式错误")
			}
		}
	}
	if req.TenantID < 0 {
		return SaaSAdminAlertBulkResolve{}, errors.New("tenantId invalid")
	}
	metric := strings.TrimSpace(req.Metric)
	if len([]rune(metric)) > 64 {
		return SaaSAdminAlertBulkResolve{}, errors.New("metric too long")
	}
	alertType := strings.TrimSpace(req.AlertType)
	if len([]rune(alertType)) > 64 {
		return SaaSAdminAlertBulkResolve{}, errors.New("alertType too long")
	}
	periodKey := strings.TrimSpace(req.PeriodKey)
	if len([]rune(periodKey)) > 32 {
		return SaaSAdminAlertBulkResolve{}, errors.New("periodKey too long")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return SaaSAdminAlertBulkResolve{}, errors.New("remark too long")
	}
	return SaaSAdminAlertBulkResolve{
		TenantID:  req.TenantID,
		Metric:    metric,
		AlertType: alertType,
		PeriodKey: periodKey,
		Limit:     limit,
		Remark:    remark,
	}, nil
}

type saasAdminNotificationRetryRequest struct {
	NotificationID int64  `json:"notificationId"`
	Remark         string `json:"remark"`
}

func parseSaaSAdminNotificationRetry(r *http.Request) (SaaSAdminAlertNotificationRetry, error) {
	var req saasAdminNotificationRetryRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminAlertNotificationRetry{}, err
		}
		req.NotificationID, _ = strconv.ParseInt(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("notificationId"), r.FormValue("notification_id"), r.FormValue("id"))), 10, 64)
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminAlertNotificationRetry{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminAlertNotificationRetry{}, errors.New("JSON 格式错误")
			}
		}
	}
	if req.NotificationID <= 0 {
		return SaaSAdminAlertNotificationRetry{}, errors.New("notificationId required")
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return SaaSAdminAlertNotificationRetry{}, errors.New("remark too long")
	}
	return SaaSAdminAlertNotificationRetry{
		NotificationID: req.NotificationID,
		Remark:         remark,
	}, nil
}

func parseSaaSAdminNotificationClose(r *http.Request) (SaaSAdminAlertNotificationClose, error) {
	retry, err := parseSaaSAdminNotificationRetry(r)
	if err != nil {
		return SaaSAdminAlertNotificationClose{}, err
	}
	return SaaSAdminAlertNotificationClose{
		NotificationID: retry.NotificationID,
		Remark:         retry.Remark,
	}, nil
}

type saasAdminNotificationBulkRetryRequest struct {
	TenantID int    `json:"tenantId"`
	Status   string `json:"status"`
	Channel  string `json:"channel"`
	Keyword  string `json:"keyword"`
	Limit    int    `json:"limit"`
	Remark   string `json:"remark"`
}

func parseSaaSAdminNotificationBulkRetry(r *http.Request) (SaaSAdminAlertNotificationBulkRetry, error) {
	var req saasAdminNotificationBulkRetryRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminAlertNotificationBulkRetry{}, err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))))
		req.Status = r.FormValue("status")
		req.Channel = r.FormValue("channel")
		req.Keyword = r.FormValue("keyword")
		req.Limit, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("limit")))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminAlertNotificationBulkRetry{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminAlertNotificationBulkRetry{}, errors.New("JSON 格式错误")
			}
		}
	}
	if req.TenantID < 0 {
		return SaaSAdminAlertNotificationBulkRetry{}, errors.New("tenantId invalid")
	}
	status, err := normalizeSaaSAdminNotificationBulkRetryStatus(req.Status)
	if err != nil {
		return SaaSAdminAlertNotificationBulkRetry{}, err
	}
	channel := strings.TrimSpace(req.Channel)
	if len([]rune(channel)) > 32 {
		return SaaSAdminAlertNotificationBulkRetry{}, errors.New("channel too long")
	}
	keyword := strings.TrimSpace(req.Keyword)
	if len([]rune(keyword)) > 80 {
		return SaaSAdminAlertNotificationBulkRetry{}, errors.New("keyword too long")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return SaaSAdminAlertNotificationBulkRetry{}, errors.New("remark too long")
	}
	return SaaSAdminAlertNotificationBulkRetry{
		TenantID: req.TenantID,
		Status:   status,
		Channel:  channel,
		Keyword:  keyword,
		Limit:    limit,
		Remark:   remark,
	}, nil
}

func normalizeSaaSAdminNotificationBulkRetryStatus(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "all":
		return "", nil
	case SaaSAlertNotificationStatusFailed, SaaSAlertNotificationStatusDead:
		return value, nil
	default:
		return "", errors.New("status 必须是 failed、dead 或 all")
	}
}

func parseSaaSAdminNotificationBulkClose(r *http.Request) (SaaSAdminAlertNotificationBulkClose, error) {
	var req saasAdminNotificationBulkRetryRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminAlertNotificationBulkClose{}, err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))))
		req.Status = r.FormValue("status")
		req.Channel = r.FormValue("channel")
		req.Keyword = r.FormValue("keyword")
		req.Limit, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("limit")))
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminAlertNotificationBulkClose{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminAlertNotificationBulkClose{}, errors.New("JSON 格式错误")
			}
		}
	}
	if req.TenantID < 0 {
		return SaaSAdminAlertNotificationBulkClose{}, errors.New("tenantId invalid")
	}
	status, err := normalizeSaaSAdminNotificationBulkCloseStatus(req.Status)
	if err != nil {
		return SaaSAdminAlertNotificationBulkClose{}, err
	}
	channel := strings.TrimSpace(req.Channel)
	if len([]rune(channel)) > 32 {
		return SaaSAdminAlertNotificationBulkClose{}, errors.New("channel too long")
	}
	keyword := strings.TrimSpace(req.Keyword)
	if len([]rune(keyword)) > 80 {
		return SaaSAdminAlertNotificationBulkClose{}, errors.New("keyword too long")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "关闭通知"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminAlertNotificationBulkClose{}, errors.New("remark too long")
	}
	return SaaSAdminAlertNotificationBulkClose{
		TenantID: req.TenantID,
		Status:   status,
		Channel:  channel,
		Keyword:  keyword,
		Limit:    limit,
		Remark:   remark,
	}, nil
}

func normalizeSaaSAdminNotificationBulkCloseStatus(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "all":
		return "", nil
	case SaaSAlertNotificationStatusPending, SaaSAlertNotificationStatusFailed:
		return value, nil
	default:
		return "", errors.New("status 必须是 pending、failed 或 all")
	}
}

func normalizeSaaSAdminRiskFollowUpStatus(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case SaaSAdminRiskFollowUpStatusPending,
		SaaSAdminRiskFollowUpStatusContacted,
		SaaSAdminRiskFollowUpStatusRenewalPending,
		SaaSAdminRiskFollowUpStatusResolved,
		SaaSAdminRiskFollowUpStatusIgnored:
		return value, nil
	default:
		return "", errors.New("status 必须是 pending、contacted、renewal_pending、resolved 或 ignored")
	}
}

func normalizeSaaSAdminCustomerSuccessPriority(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "", "all":
		return "", nil
	case SaaSAdminCustomerSuccessPriorityCritical,
		SaaSAdminCustomerSuccessPriorityHigh,
		SaaSAdminCustomerSuccessPriorityMedium,
		SaaSAdminCustomerSuccessPriorityNormal:
		return value, nil
	default:
		return "", errors.New("priority 必须是 critical、high、medium、normal 或 all")
	}
}

func normalizeSaaSAdminOperationQueueSource(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "", SaaSAdminOperationQueueSourceAll:
		return "", nil
	case SaaSAdminOperationQueueSourceCustomerSuccess,
		SaaSAdminOperationQueueSourceTaskSLA,
		SaaSAdminOperationQueueSourceBillingFollowUp,
		SaaSAdminOperationQueueSourceNotification,
		SaaSAdminOperationQueueSourceClosedNotification,
		SaaSAdminOperationQueueSourceNotificationHealth:
		return value, nil
	default:
		return "", errors.New("source 必须是 customer_success、task_sla、billing_follow_up、notification、closed_notification、notification_health 或 all")
	}
}

func saasAdminOptionalRiskFollowUpStatus(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	raw = strings.ReplaceAll(raw, "-", "_")
	if raw == "" || raw == "all" {
		return "", true
	}
	status, err := normalizeSaaSAdminRiskFollowUpStatus(raw)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return "", false
	}
	return status, true
}

func saasAdminRiskFollowUpDueState(w http.ResponseWriter, r *http.Request) (string, bool) {
	dueState, err := normalizeSaaSAdminRiskFollowUpDueState(r.URL.Query().Get("dueState"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return "", false
	}
	return dueState, true
}

func normalizeSaaSAdminRiskFollowUpDueState(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "-", "_")
	if value == "" || value == "all" {
		return SaaSAdminRiskFollowUpDueStateAll, nil
	}
	switch value {
	case SaaSAdminRiskFollowUpDueStateOverdue,
		SaaSAdminRiskFollowUpDueStateDueSoon,
		SaaSAdminRiskFollowUpDueStateFuture,
		SaaSAdminRiskFollowUpDueStateNoDate,
		SaaSAdminRiskFollowUpDueStateClosed:
		return value, nil
	default:
		return "", errors.New("dueState 必须是 all、overdue、due_soon、future、no_date 或 closed")
	}
}

const saasAdminMySQLTimestampMax = "2038-01-18 23:59:59"

var errSaaSAdminMySQLTimestampRange = errors.New("date exceeds MySQL TIMESTAMP range")

func normalizeSaaSAdminExpiresAt(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "null") || raw == "-" {
		return "", nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		parsed, err := time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			normalized := parsed.Format("2006-01-02 15:04:05")
			if layout == "2006-01-02" {
				normalized = parsed.Format("2006-01-02 00:00:00")
			}
			if normalized > saasAdminMySQLTimestampMax {
				return "", fmt.Errorf("%w (max %s)", errSaaSAdminMySQLTimestampRange, saasAdminMySQLTimestampMax)
			}
			return normalized, nil
		}
	}
	return "", errors.New("expiresAt format invalid")
}

func normalizeSaaSAdminOptionalDateTime(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "null") || raw == "-" {
		return "", nil
	}
	return normalizeSaaSAdminExpiresAt(raw)
}

func saasAdminDateTimeBefore(left string, right string) bool {
	leftTime, ok := parseSaaSAdminNormalizedDateTime(left)
	if !ok {
		return false
	}
	rightTime, ok := parseSaaSAdminNormalizedDateTime(right)
	if !ok {
		return false
	}
	return leftTime.Before(rightTime)
}

func parseSaaSAdminNormalizedDateTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", time.RFC3339} {
		parsed, err := time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func saasAdminCustomerSuccessRenewalExpiresAt(currentExpiresAt string, months int, now time.Time) string {
	if months <= 0 {
		months = 12
	}
	base := now
	if parsed, ok := parseSaaSAdminNormalizedDateTime(currentExpiresAt); ok && parsed.After(base) {
		base = parsed
	}
	return base.AddDate(0, months, 0).Format("2006-01-02 15:04:05")
}

func saasAdminCustomerSuccessRenewalExternalOrderNo(prefix string, tenantID int) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || tenantID <= 0 {
		return ""
	}
	value := fmt.Sprintf("%s-%d", prefix, tenantID)
	if len([]rune(value)) > 128 {
		runes := []rune(value)
		value = string(runes[:128])
	}
	return value
}

func saasAdminCustomerSuccessRenewalNotificationAlert(item SaaSAdminCustomerSuccessQueueItem, notify SaaSAdminCustomerSuccessRenewalNotifications, now time.Time) (SaaSQuotaAlert, string, error) {
	expiresAt := strings.TrimSpace(item.Tenant.ExpiresAt)
	if expiresAt == "" {
		return SaaSQuotaAlert{}, "", errors.New("tenant expiresAt missing")
	}
	expiresAtTime, ok := parseSaaSAdminNormalizedDateTime(expiresAt)
	if !ok {
		return SaaSQuotaAlert{}, "", errors.New("tenant expiresAt invalid")
	}
	today := time.Date(now.In(time.Local).Year(), now.In(time.Local).Month(), now.In(time.Local).Day(), 0, 0, 0, 0, time.Local)
	expiresDate := time.Date(expiresAtTime.In(time.Local).Year(), expiresAtTime.In(time.Local).Month(), expiresAtTime.In(time.Local).Day(), 0, 0, 0, 0, time.Local)
	daysUntil := int(expiresDate.Sub(today).Hours() / 24)
	tenantName := strings.TrimSpace(item.Tenant.TenantName)
	if tenantName == "" {
		tenantName = "租户 " + strconv.Itoa(item.Tenant.TenantID)
	}
	packageName := strings.TrimSpace(saasAdminFirstNonEmpty(item.Tenant.PackageName, item.Tenant.PackageCode, "未开套餐"))
	message := ""
	severity := SaaSAlertSeverityWarning
	switch {
	case daysUntil < 0:
		severity = SaaSAlertSeverityCritical
		message = fmt.Sprintf("%s 的 %s 已于 %s 到期，逾期 %d 天，请立即催收或续费。", tenantName, packageName, expiresDate.Format("2006-01-02"), -daysUntil)
	case daysUntil == 0:
		severity = SaaSAlertSeverityCritical
		message = fmt.Sprintf("%s 的 %s 今日到期，请立即确认续费。", tenantName, packageName)
	default:
		if daysUntil <= 3 {
			severity = SaaSAlertSeverityCritical
		}
		message = fmt.Sprintf("%s 的 %s 将于 %s 到期，剩余 %d 天，请跟进续费。", tenantName, packageName, expiresDate.Format("2006-01-02"), daysUntil)
	}
	alert := SaaSQuotaAlert{
		Status: SaaSQuotaStatus{
			Metric:   SaaSEventMetricTenantRenewal,
			TenantID: item.Tenant.TenantID,
			Current:  int64(daysUntil),
			Limit:    int64(notify.ReminderDays),
		},
		AlertType: SaaSAlertTypeTenantRenewal,
		Severity:  severity,
		PeriodKey: "renewal_" + expiresDate.Format("20060102"),
		Source:    "saas_admin.customer_success.renewal_notification",
		Message:   message,
		Context: map[string]any{
			"tenantId":     item.Tenant.TenantID,
			"tenantName":   tenantName,
			"packageCode":  item.Tenant.PackageCode,
			"packageName":  packageName,
			"expiresAt":    expiresDate.Format("2006-01-02"),
			"daysUntil":    daysUntil,
			"reminderDays": notify.ReminderDays,
			"priority":     item.Priority,
			"healthScore":  item.HealthScore,
			"owner":        item.Owner,
			"dueState":     item.DueState,
			"reasons":      item.Reasons,
			"remark":       notify.Remark,
		},
	}
	return alert, SaaSAlertNotificationKey(alert, notify.Channel), nil
}

func saasAdminRenewalForecastNotificationAlert(item SaaSAdminRenewalForecastTenant, notify SaaSAdminRenewalForecastNotifications, now time.Time) (SaaSQuotaAlert, string, error) {
	expiresAt := strings.TrimSpace(item.Tenant.ExpiresAt)
	if expiresAt == "" {
		return SaaSQuotaAlert{}, "", errors.New("tenant expiresAt missing")
	}
	expiresAtTime, ok := parseSaaSAdminNormalizedDateTime(expiresAt)
	if !ok {
		return SaaSQuotaAlert{}, "", errors.New("tenant expiresAt invalid")
	}
	today := time.Date(now.In(time.Local).Year(), now.In(time.Local).Month(), now.In(time.Local).Day(), 0, 0, 0, 0, time.Local)
	expiresDate := time.Date(expiresAtTime.In(time.Local).Year(), expiresAtTime.In(time.Local).Month(), expiresAtTime.In(time.Local).Day(), 0, 0, 0, 0, time.Local)
	daysUntil := int(expiresDate.Sub(today).Hours() / 24)
	tenantName := strings.TrimSpace(item.Tenant.TenantName)
	if tenantName == "" {
		tenantName = "租户 " + strconv.Itoa(item.Tenant.TenantID)
	}
	packageName := strings.TrimSpace(saasAdminFirstNonEmpty(item.Tenant.PackageName, item.Tenant.PackageCode, "未开套餐"))
	message := ""
	severity := SaaSAlertSeverityWarning
	switch {
	case daysUntil < 0:
		severity = SaaSAlertSeverityCritical
		message = fmt.Sprintf("%s 的 %s 已于 %s 到期，逾期 %d 天，请立即催收或续费。", tenantName, packageName, expiresDate.Format("2006-01-02"), -daysUntil)
	case daysUntil == 0:
		severity = SaaSAlertSeverityCritical
		message = fmt.Sprintf("%s 的 %s 今日到期，请立即确认续费。", tenantName, packageName)
	default:
		if daysUntil <= 3 {
			severity = SaaSAlertSeverityCritical
		}
		message = fmt.Sprintf("%s 的 %s 将于 %s 到期，剩余 %d 天，请跟进续费。", tenantName, packageName, expiresDate.Format("2006-01-02"), daysUntil)
	}
	alert := SaaSQuotaAlert{
		Status: SaaSQuotaStatus{
			Metric:   SaaSEventMetricTenantRenewal,
			TenantID: item.Tenant.TenantID,
			Current:  int64(daysUntil),
			Limit:    int64(notify.ReminderDays),
		},
		AlertType: SaaSAlertTypeTenantRenewal,
		Severity:  severity,
		PeriodKey: "renewal_" + expiresDate.Format("20060102"),
		Source:    "saas_admin.renewal_forecast.notification",
		Message:   message,
		Context: map[string]any{
			"tenantId":             item.Tenant.TenantID,
			"tenantName":           tenantName,
			"packageCode":          item.Tenant.PackageCode,
			"packageName":          packageName,
			"expiresAt":            expiresDate.Format("2006-01-02"),
			"daysUntil":            daysUntil,
			"reminderDays":         notify.ReminderDays,
			"bucket":               item.Bucket,
			"bucketLabel":          item.BucketLabel,
			"renewalAmountCents":   item.RenewalAmountCents,
			"estimatedMrrCents":    item.EstimatedMRRCents,
			"priced":               item.Priced,
			"owner":                item.Owner,
			"latestBillingEventId": item.LatestBillingEventID,
			"remark":               notify.Remark,
		},
	}
	return alert, SaaSAlertNotificationKey(alert, notify.Channel), nil
}

func saasAdminTaskSLANotificationEligible(item SaaSAdminTaskSLAItem, slaStatus string) bool {
	switch slaStatus {
	case "all":
		return true
	case "overdue":
		return item.SLAStatus == "overdue"
	case "fresh":
		return item.SLAStatus == "fresh"
	case "unknown":
		return item.SLAStatus == "unknown"
	default:
		return item.SLAStatus == "warning" || item.SLAStatus == "overdue"
	}
}

func saasAdminTaskSLANotificationAlert(item SaaSAdminTaskSLAItem, notify SaaSAdminTaskSLANotifications, platformAdminTenantID int) (SaaSQuotaAlert, string, error) {
	task := item.Task
	if task.ID <= 0 {
		return SaaSQuotaAlert{}, "", errors.New("task id missing")
	}
	notificationTenantID := task.TenantID
	if notificationTenantID <= 0 {
		notificationTenantID = platformAdminTenantID
	}
	if notificationTenantID <= 0 {
		return SaaSQuotaAlert{}, "", errors.New("notification tenant missing")
	}
	severity := SaaSAlertSeverityWarning
	if item.SLAStatus == "overdue" {
		severity = SaaSAlertSeverityCritical
	}
	taskTypeLabel := saasAdminTaskTypeLabel(task.TaskType)
	statusLabel := saasAdminTaskStatusLabel(task.Status)
	owner := strings.TrimSpace(item.Owner)
	if owner == "" {
		owner = saasAdminTaskOwnerLabel(task.ActorUserID, task.ActorTenantID)
	}
	message := fmt.Sprintf("运营任务 #%d（%s/%s）已进入 %s，已等待 %d 小时，请负责人 %s 跟进。", task.ID, taskTypeLabel, statusLabel, saasAdminTaskSLAStatusLabel(item.SLAStatus), item.AgeHours, owner)
	if item.BreachHours > 0 {
		message = fmt.Sprintf("运营任务 #%d（%s/%s）已逾期 %d 小时，累计等待 %d 小时，请负责人 %s 立即处理。", task.ID, taskTypeLabel, statusLabel, item.BreachHours, item.AgeHours, owner)
	}
	periodKey := fmt.Sprintf("admin_task_sla_%d_%s_%dh_%dh", task.ID, item.SLAStatus, notify.Options.WarningHours, notify.Options.OverdueHours)
	alert := SaaSQuotaAlert{
		Status: SaaSQuotaStatus{
			Metric:   SaaSEventMetricAdminTaskSLA,
			TenantID: notificationTenantID,
			Current:  int64(item.AgeHours),
			Limit:    int64(notify.Options.OverdueHours),
		},
		AlertType: SaaSAlertTypeAdminTaskSLA,
		Severity:  severity,
		PeriodKey: periodKey,
		Source:    "saas_admin.task_sla.notification",
		Message:   message,
		Context: map[string]any{
			"taskId":               task.ID,
			"taskType":             task.TaskType,
			"taskTypeLabel":        taskTypeLabel,
			"taskStatus":           task.Status,
			"taskStatusLabel":      statusLabel,
			"tenantId":             task.TenantID,
			"notificationTenantId": notificationTenantID,
			"packageCode":          task.PackageCode,
			"owner":                owner,
			"actorUserId":          task.ActorUserID,
			"actorTenantId":        task.ActorTenantID,
			"slaStatus":            item.SLAStatus,
			"slaStatusLabel":       saasAdminTaskSLAStatusLabel(item.SLAStatus),
			"ageHours":             item.AgeHours,
			"breachHours":          item.BreachHours,
			"warningHours":         notify.Options.WarningHours,
			"overdueHours":         notify.Options.OverdueHours,
			"createdAt":            task.CreatedAt,
			"updatedAt":            task.UpdatedAt,
			"remark":               task.Remark,
			"lastError":            task.LastError,
			"notificationRemark":   notify.Remark,
		},
	}
	return alert, SaaSAlertNotificationKey(alert, notify.Channel), nil
}

func saasAdminOperationQueueAssignmentNotificationEligible(assignment SaaSAdminOperationQueueAssignment) bool {
	switch assignment.DueState {
	case SaaSAdminRiskFollowUpDueStateOverdue, SaaSAdminRiskFollowUpDueStateDueSoon:
		return true
	default:
		return false
	}
}

func saasAdminOperationQueueAssignmentNotificationAlert(assignment SaaSAdminOperationQueueAssignment, notify SaaSAdminOperationQueueAssignmentNotifications, platformAdminTenantID int) (SaaSQuotaAlert, string, error) {
	if assignment.OperationID <= 0 {
		return SaaSQuotaAlert{}, "", errors.New("operation id missing")
	}
	if strings.TrimSpace(assignment.Source) == "" {
		return SaaSQuotaAlert{}, "", errors.New("assignment source missing")
	}
	if strings.TrimSpace(assignment.ObjectID) == "" {
		return SaaSQuotaAlert{}, "", errors.New("assignment object id missing")
	}
	notificationTenantID := assignment.TenantID
	if notificationTenantID <= 0 {
		notificationTenantID = platformAdminTenantID
	}
	if notificationTenantID <= 0 {
		return SaaSQuotaAlert{}, "", errors.New("notification tenant missing")
	}
	dueAt, ok := saasAdminParseRiskFollowUpTime(assignment.NextFollowUpAt)
	if !ok {
		return SaaSQuotaAlert{}, "", errors.New("nextFollowUpAt invalid")
	}
	now := time.Now()
	today := time.Date(now.In(time.Local).Year(), now.In(time.Local).Month(), now.In(time.Local).Day(), 0, 0, 0, 0, time.Local)
	dueDate := time.Date(dueAt.In(time.Local).Year(), dueAt.In(time.Local).Month(), dueAt.In(time.Local).Day(), 0, 0, 0, 0, time.Local)
	daysUntil := int(dueDate.Sub(today).Hours() / 24)
	targetName := strings.TrimSpace(assignment.TargetName)
	if targetName == "" {
		targetName = strings.TrimSpace(assignment.ObjectType + " " + assignment.ObjectID)
	}
	if targetName == "" {
		targetName = assignment.ObjectID
	}
	owner := strings.TrimSpace(assignment.Owner)
	if owner == "" {
		owner = "未分配"
	}
	sourceLabel := saasAdminOperationQueueSourceLabel(assignment.Source)
	dueStateLabel := saasAdminOperationQueueAssignmentDueStateLabel(assignment.DueState)
	severity := SaaSAlertSeverityWarning
	message := fmt.Sprintf("运营待办认领 #%d（%s/%s）将在 %s 到期，请负责人 %s 跟进。", assignment.OperationID, sourceLabel, targetName, assignment.NextFollowUpAt, owner)
	if assignment.DueState == SaaSAdminRiskFollowUpDueStateOverdue {
		severity = SaaSAlertSeverityCritical
		overdueDays := -daysUntil
		if overdueDays < 1 {
			overdueDays = 1
		}
		message = fmt.Sprintf("运营待办认领 #%d（%s/%s）已逾期 %d 天，计划跟进时间 %s，请负责人 %s 立即处理。", assignment.OperationID, sourceLabel, targetName, overdueDays, assignment.NextFollowUpAt, owner)
	}
	periodKey := fmt.Sprintf("operation_queue_assignment_%d_%s", assignment.OperationID, assignment.DueState)
	alert := SaaSQuotaAlert{
		Status: SaaSQuotaStatus{
			Metric:   SaaSEventMetricOperationQueueAssignment,
			TenantID: notificationTenantID,
			Current:  int64(daysUntil),
			Limit:    7,
		},
		AlertType: SaaSAlertTypeOperationQueueAssign,
		Severity:  severity,
		PeriodKey: periodKey,
		Source:    "saas_admin.operation_queue.assignment_notification",
		Message:   message,
		Context: map[string]any{
			"operationId":          assignment.OperationID,
			"tenantId":             assignment.TenantID,
			"notificationTenantId": notificationTenantID,
			"source":               assignment.Source,
			"sourceLabel":          sourceLabel,
			"objectType":           assignment.ObjectType,
			"objectId":             assignment.ObjectID,
			"targetName":           targetName,
			"owner":                owner,
			"status":               assignment.Status,
			"dueState":             assignment.DueState,
			"dueStateLabel":        dueStateLabel,
			"nextFollowUpAt":       assignment.NextFollowUpAt,
			"daysUntil":            daysUntil,
			"assignmentRemark":     assignment.Remark,
			"notificationRemark":   notify.Remark,
			"assignedAt":           assignment.AssignedAt,
			"actorUserId":          assignment.ActorUserID,
			"actorTenantId":        assignment.ActorTenantID,
			"filters":              saasAdminOperationQueueAssignmentFiltersPayload(notify.Options),
		},
	}
	return alert, SaaSAlertNotificationKey(alert, notify.Channel), nil
}

func saasAdminOperationQueueSourceLabel(source string) string {
	switch source {
	case SaaSAdminOperationQueueSourceCustomerSuccess:
		return "客户成功"
	case SaaSAdminOperationQueueSourceTaskSLA:
		return "任务SLA"
	case SaaSAdminOperationQueueSourceBillingFollowUp:
		return "账单跟进"
	case SaaSAdminOperationQueueSourceNotification:
		return "失败通知"
	case SaaSAdminOperationQueueSourceClosedNotification:
		return "关闭通知"
	case SaaSAdminOperationQueueSourceNotificationHealth:
		return "通知健康"
	default:
		return strings.TrimSpace(source)
	}
}

func saasAdminOperationQueueAssignmentDueStateLabel(state string) string {
	switch state {
	case SaaSAdminRiskFollowUpDueStateOverdue:
		return "已逾期"
	case SaaSAdminRiskFollowUpDueStateDueSoon:
		return "7天内"
	case SaaSAdminRiskFollowUpDueStateFuture:
		return "未来"
	case SaaSAdminRiskFollowUpDueStateNoDate:
		return "无日期"
	case SaaSAdminRiskFollowUpDueStateClosed:
		return "已关闭"
	default:
		return strings.TrimSpace(state)
	}
}

func saasAdminTaskTypeLabel(taskType string) string {
	switch taskType {
	case SaaSAdminTaskTypePackageSync:
		return "套餐同步"
	case SaaSAdminTaskTypeTenantRenewal:
		return "租户续费"
	case SaaSAdminTaskTypeTenantProvision:
		return "平台开户"
	default:
		return taskType
	}
}

func saasAdminTaskStatusLabel(status string) string {
	switch status {
	case SaaSAdminTaskStatusPending:
		return "待应用"
	case SaaSAdminTaskStatusBlocked:
		return "阻断"
	case SaaSAdminTaskStatusFailed:
		return "失败"
	case SaaSAdminTaskStatusApplied:
		return "已应用"
	case SaaSAdminTaskStatusCanceled:
		return "已取消"
	default:
		return status
	}
}

func saasAdminTaskSLAStatusLabel(status string) string {
	switch status {
	case "overdue":
		return "逾期"
	case "warning":
		return "预警"
	case "fresh":
		return "正常"
	case "unknown":
		return "时间未知"
	default:
		return status
	}
}

func parseSaaSAdminAmountCents(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	if strings.HasPrefix(raw, "-") {
		return 0, errors.New("amount must not be negative")
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 {
		return 0, errors.New("amount format invalid")
	}
	yuan := strings.TrimSpace(parts[0])
	if yuan == "" {
		yuan = "0"
	}
	cents := ""
	if len(parts) == 2 {
		cents = strings.TrimSpace(parts[1])
	}
	if len(cents) > 2 {
		return 0, errors.New("amount supports at most 2 decimal places")
	}
	for _, r := range yuan + cents {
		if r < '0' || r > '9' {
			return 0, errors.New("amount format invalid")
		}
	}
	for len(cents) < 2 {
		cents += "0"
	}
	yuanValue, err := strconv.ParseInt(yuan, 10, 64)
	if err != nil {
		return 0, errors.New("amount format invalid")
	}
	centsValue, err := strconv.ParseInt(cents, 10, 64)
	if err != nil {
		return 0, errors.New("amount format invalid")
	}
	return yuanValue*100 + centsValue, nil
}

func saasAdminFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseSaaSAdminRequestBool(raw string, field string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true, nil
	case "0", "false", "no", "n", "off":
		return false, nil
	default:
		return false, errors.New(field + " invalid")
	}
}

func saasAdminSummaryPayload(summary SaaSAdminSummary) map[string]any {
	return map[string]any{
		"tenantCount":              summary.TenantCount,
		"activeTenantPackageCount": summary.ActiveTenantPackageCount,
		"enabledPackageCount":      summary.EnabledPackageCount,
		"userCount":                summary.UserCount,
		"corpCount":                summary.CorpCount,
		"openAlertCount":           summary.OpenAlertCount,
		"pendingNotificationCount": summary.PendingNotificationCount,
		"expiringSoonTenantCount":  summary.ExpiringSoonTenantCount,
		"expiredTenantCount":       summary.ExpiredTenantCount,
	}
}

func saasAdminOverviewFiltersPayload(options SaaSAdminOverviewOptions) map[string]any {
	dueState := strings.TrimSpace(options.DueState)
	if dueState == "" {
		dueState = SaaSAdminDueStateAll
	}
	return map[string]any{
		"keyword":      options.Keyword,
		"tenantStatus": options.TenantStatus,
		"packageCode":  options.PackageCode,
		"dueState":     dueState,
	}
}

func saasAdminRiskFiltersPayload(options SaaSAdminRiskOptions) map[string]any {
	filters := saasAdminOverviewFiltersPayload(options.SaaSAdminOverviewOptions)
	filters["tenantId"] = options.TenantID
	filters["limit"] = options.Limit
	filters["expiringDays"] = options.ExpiringDays
	filters["highUsageRatio"] = options.HighUsageRatio
	return filters
}

func saasAdminBusinessMetricsFiltersPayload(options SaaSAdminBusinessMetricsOptions) map[string]any {
	return map[string]any{
		"tenantLimit":    options.TenantLimit,
		"expiringDays":   options.ExpiringDays,
		"highUsageRatio": options.HighUsageRatio,
		"billingLimit":   options.BillingLimit,
	}
}

func saasAdminBusinessTrendFiltersPayload(options SaaSAdminBusinessTrendOptions) map[string]any {
	return map[string]any{
		"months":       options.Months,
		"billingLimit": options.BillingLimit,
		"taskLimit":    options.TaskLimit,
	}
}

func saasAdminRenewalForecastFiltersPayload(options SaaSAdminRenewalForecastOptions) map[string]any {
	return map[string]any{
		"tenantLimit":  options.TenantLimit,
		"days":         options.Days,
		"billingLimit": options.BillingLimit,
		"taskLimit":    options.TaskLimit,
		"packageCode":  options.PackageCode,
		"bucket":       options.Bucket,
		"priced":       options.PriceState,
		"owner":        options.Owner,
		"taskStatus":   options.TaskStatus,
	}
}

func saasAdminOperationLogFiltersPayload(options SaaSAdminOperationLogOptions) map[string]any {
	return map[string]any{
		"tenantId":   options.TenantID,
		"limit":      options.Limit,
		"action":     options.Action,
		"targetType": options.TargetType,
		"keyword":    options.Keyword,
	}
}

func saasAdminOperationLogSummaryPayload(summary SaaSAdminOperationLogSummary) map[string]any {
	return map[string]any{
		"operationCount":  summary.OperationCount,
		"tenantCount":     summary.TenantCount,
		"actorUserCount":  summary.ActorUserCount,
		"actionCount":     summary.ActionCount,
		"targetTypeCount": summary.TargetTypeCount,
	}
}

func saasAdminBillingEventFiltersPayload(options SaaSAdminBillingEventOptions) map[string]any {
	return map[string]any{
		"tenantId":    options.TenantID,
		"limit":       options.Limit,
		"eventType":   options.EventType,
		"packageCode": options.PackageCode,
		"keyword":     options.Keyword,
	}
}

func saasAdminBillingReconciliationFiltersPayload(options SaaSAdminBillingReconciliationOptions) map[string]any {
	filters := saasAdminBillingEventFiltersPayload(options.SaaSAdminBillingEventOptions)
	filters["mismatchOnly"] = options.MismatchOnly
	return filters
}

func saasAdminTaskFiltersPayload(options SaaSAdminTaskOptions) map[string]any {
	return map[string]any{
		"taskId":      options.TaskID,
		"taskType":    options.TaskType,
		"status":      options.Status,
		"tenantId":    options.TenantID,
		"packageCode": options.PackageCode,
		"limit":       options.Limit,
	}
}

func saasAdminTaskSummaryPayload(summary SaaSAdminTaskSummary) map[string]any {
	return map[string]any{
		"taskCount":            summary.TaskCount,
		"pendingCount":         summary.PendingCount,
		"blockedCount":         summary.BlockedCount,
		"failedCount":          summary.FailedCount,
		"appliedCount":         summary.AppliedCount,
		"canceledCount":        summary.CanceledCount,
		"actionableCount":      summary.ActionableCount,
		"packageSyncCount":     summary.PackageSyncCount,
		"tenantRenewalCount":   summary.TenantRenewalCount,
		"tenantProvisionCount": summary.TenantProvisionCount,
		"tenantCount":          summary.TenantCount,
		"actorUserCount":       summary.ActorUserCount,
	}
}

func saasAdminTaskOwnerReportPayload(report SaaSAdminTaskOwnerReport) map[string]any {
	return map[string]any{
		"filters":          saasAdminTaskFiltersPayload(report.Options),
		"summary":          saasAdminTaskSummaryPayload(report.Summary),
		"ownerCount":       report.OwnerCount,
		"returnedCount":    len(report.Owners),
		"scannedTaskCount": report.ScannedTaskCount,
		"partial":          report.Partial,
		"owners":           saasAdminTaskOwnerSummaryPayloads(report.Owners),
	}
}

func saasAdminTaskOwnerSummaryPayloads(owners []SaaSAdminTaskOwnerSummary) []map[string]any {
	items := make([]map[string]any, 0, len(owners))
	for _, owner := range owners {
		items = append(items, saasAdminTaskOwnerSummaryPayload(owner))
	}
	return items
}

func saasAdminTaskOwnerSummaryPayload(owner SaaSAdminTaskOwnerSummary) map[string]any {
	return map[string]any{
		"owner":         owner.Owner,
		"actorUserId":   owner.ActorUserID,
		"actorTenantId": owner.ActorTenantID,
		"summary":       saasAdminTaskSummaryPayload(owner.Summary),
		"lastTaskAt":    owner.LastTaskAt,
		"lastAppliedAt": owner.LastAppliedAt,
		"lastError":     owner.LastError,
		"recentTasks":   saasAdminTaskPayloads(owner.RecentTasks),
	}
}

func saasAdminTaskSLAReportPayload(report SaaSAdminTaskSLAReport) map[string]any {
	return map[string]any{
		"filters":          saasAdminTaskSLAFiltersPayload(report.Options),
		"taskSummary":      saasAdminTaskSummaryPayload(report.TaskSummary),
		"summary":          saasAdminTaskSLASummaryPayload(report.Summary),
		"ownerCount":       report.OwnerCount,
		"returnedCount":    len(report.Tasks),
		"returnedOwners":   len(report.Owners),
		"scannedTaskCount": report.ScannedTaskCount,
		"partial":          report.Partial,
		"owners":           saasAdminTaskSLAOwnerSummaryPayloads(report.Owners),
		"tasks":            saasAdminTaskSLAItemPayloads(report.Tasks),
	}
}

func saasAdminTaskSLAFiltersPayload(options SaaSAdminTaskSLAOptions) map[string]any {
	filters := saasAdminTaskFiltersPayload(options.SaaSAdminTaskOptions)
	filters["warningHours"] = options.WarningHours
	filters["overdueHours"] = options.OverdueHours
	return filters
}

func saasAdminTaskSLASummaryPayload(summary SaaSAdminTaskSLASummary) map[string]any {
	return map[string]any{
		"taskCount":            summary.TaskCount,
		"freshCount":           summary.FreshCount,
		"warningCount":         summary.WarningCount,
		"overdueCount":         summary.OverdueCount,
		"unknownAgeCount":      summary.UnknownAgeCount,
		"pendingCount":         summary.PendingCount,
		"blockedCount":         summary.BlockedCount,
		"failedCount":          summary.FailedCount,
		"packageSyncCount":     summary.PackageSyncCount,
		"tenantRenewalCount":   summary.TenantRenewalCount,
		"tenantProvisionCount": summary.TenantProvisionCount,
		"tenantCount":          summary.TenantCount,
		"actorUserCount":       summary.ActorUserCount,
		"maxAgeHours":          summary.MaxAgeHours,
	}
}

func saasAdminTaskSLAOwnerSummaryPayloads(owners []SaaSAdminTaskSLAOwnerSummary) []map[string]any {
	items := make([]map[string]any, 0, len(owners))
	for _, owner := range owners {
		items = append(items, saasAdminTaskSLAOwnerSummaryPayload(owner))
	}
	return items
}

func saasAdminTaskSLAOwnerSummaryPayload(owner SaaSAdminTaskSLAOwnerSummary) map[string]any {
	return map[string]any{
		"owner":         owner.Owner,
		"actorUserId":   owner.ActorUserID,
		"actorTenantId": owner.ActorTenantID,
		"summary":       saasAdminTaskSLASummaryPayload(owner.Summary),
		"oldestTaskAt":  owner.OldestTaskAt,
		"tasks":         saasAdminTaskSLAItemPayloads(owner.Tasks),
	}
}

func saasAdminTaskSLAItemPayloads(items []SaaSAdminTaskSLAItem) []map[string]any {
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload = append(payload, saasAdminTaskSLAItemPayload(item))
	}
	return payload
}

func saasAdminTaskSLAItemPayload(item SaaSAdminTaskSLAItem) map[string]any {
	return map[string]any{
		"task":        saasAdminTaskPayload(item.Task),
		"owner":       item.Owner,
		"ageHours":    item.AgeHours,
		"slaStatus":   item.SLAStatus,
		"breachHours": item.BreachHours,
	}
}

func saasAdminTaskSLANotificationsPayload(result SaaSAdminTaskSLANotificationsResult, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":       true,
		"platformAdminTenantId":  platformAdminTenantID,
		"generatedAt":            time.Now().Format("2006-01-02 15:04:05"),
		"filters":                saasAdminTaskSLAFiltersPayload(result.Options),
		"summary":                saasAdminTaskSLASummaryPayload(result.Summary),
		"matchedCount":           result.MatchedCount,
		"eligibleCount":          result.EligibleCount,
		"enqueuedCount":          result.EnqueuedCount,
		"skippedExistingCount":   result.SkippedExistingCount,
		"skippedInvalidCount":    result.SkippedInvalidCount,
		"skippedStatusCount":     result.SkippedStatusCount,
		"channel":                result.Channel,
		"maxAttempts":            result.MaxAttempts,
		"slaStatus":              result.SLAStatus,
		"remark":                 result.Remark,
		"forceCreate":            result.ForceCreate,
		"notifications":          saasAdminAlertNotificationPayloads(result.Notifications),
		"skipped":                saasAdminTaskSLANotificationSkippedPayloads(result.Skipped),
		"taskSLANotificationKey": SaaSAlertTypeAdminTaskSLA,
	}
}

func saasAdminTaskSLANotificationSkippedPayloads(items []SaaSAdminTaskSLANotificationSkipped) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"taskId":   item.TaskID,
			"tenantId": item.TenantID,
			"reason":   item.Reason,
		})
	}
	return payloads
}

func saasAdminRiskFollowUpTaskFiltersPayload(options SaaSAdminRiskFollowUpTaskOptions) map[string]any {
	dueState := strings.TrimSpace(options.DueState)
	if dueState == "" {
		dueState = SaaSAdminRiskFollowUpDueStateAll
	}
	return map[string]any{
		"tenantId": options.TenantID,
		"status":   options.Status,
		"owner":    options.Owner,
		"dueState": dueState,
		"keyword":  options.Keyword,
		"limit":    options.Limit,
	}
}

func saasAdminBillingReconciliationFollowUpFiltersPayload(options SaaSAdminBillingReconciliationFollowUpOptions) map[string]any {
	dueState := strings.TrimSpace(options.DueState)
	if dueState == "" {
		dueState = SaaSAdminRiskFollowUpDueStateAll
	}
	return map[string]any{
		"tenantId": options.TenantID,
		"status":   options.Status,
		"owner":    options.Owner,
		"dueState": dueState,
		"keyword":  options.Keyword,
		"limit":    options.Limit,
	}
}

func saasAdminAlertFiltersPayload(options SaaSAlertListOptions) map[string]any {
	return map[string]any{
		"tenantId":  options.TenantID,
		"status":    options.Status,
		"metric":    options.Metric,
		"alertType": options.AlertType,
		"page":      options.Page,
		"perPage":   options.PerPage,
	}
}

func saasAdminAlertSummaryPayload(summary SaaSAdminAlertSummary) map[string]any {
	return map[string]any{
		"alertCount":    summary.AlertCount,
		"openCount":     summary.OpenCount,
		"resolvedCount": summary.ResolvedCount,
		"warningCount":  summary.WarningCount,
		"criticalCount": summary.CriticalCount,
		"metricCount":   summary.MetricCount,
		"tenantCount":   summary.TenantCount,
	}
}

func saasAdminNotificationFiltersPayload(options SaaSAdminAlertNotificationOptions) map[string]any {
	return map[string]any{
		"tenantId": options.TenantID,
		"status":   options.Status,
		"channel":  options.Channel,
		"keyword":  options.Keyword,
		"limit":    options.Limit,
	}
}

func saasAdminNotificationSummaryPayload(summary SaaSAdminAlertNotificationSummary) map[string]any {
	return map[string]any{
		"notificationCount": summary.NotificationCount,
		"pendingCount":      summary.PendingCount,
		"failedCount":       summary.FailedCount,
		"deliveredCount":    summary.DeliveredCount,
		"deadCount":         summary.DeadCount,
		"closedCount":       summary.ClosedCount,
		"suppressedCount":   summary.SuppressedCount,
		"retryableCount":    summary.RetryableCount,
		"tenantCount":       summary.TenantCount,
		"channelCount":      summary.ChannelCount,
	}
}

func saasAdminPackagePayloads(packages []SaaSAdminPackage) []map[string]any {
	items := make([]map[string]any, 0, len(packages))
	for _, item := range packages {
		items = append(items, saasAdminPackagePayload(item))
	}
	return items
}

func saasAdminPackagePayload(item SaaSAdminPackage) map[string]any {
	return map[string]any{
		"code":        item.Code,
		"name":        item.Name,
		"description": item.Description,
		"status":      item.Status,
		"version":     item.Version,
		"limits":      saasAdminPackageLimitsPayload(item.Limits),
	}
}

func saasAdminBuildPackageImpact(existing bool, before SaaSAdminPackage, after SaaSAdminPackage) SaaSAdminPackageImpact {
	impact := SaaSAdminPackageImpact{
		Existing:               existing,
		AfterStatus:            after.Status,
		TenantSnapshotsUpdated: false,
	}
	if existing {
		impact.BeforeStatus = before.Status
		impact.StatusChanged = before.Status != after.Status
	}
	beforeLimits := SaaSAdminPackageLimits{}
	if existing {
		beforeLimits = before.Limits
	}
	impact.Changes = saasAdminPackageLimitChanges(beforeLimits, after.Limits)
	impact.ChangedLimitCount = len(impact.Changes)
	for _, change := range impact.Changes {
		switch change.Direction {
		case "increase":
			impact.IncreasedLimitCount++
		case "decrease":
			impact.DecreasedLimitCount++
		case "newly_limited":
			impact.NewlyLimitedCount++
		case "newly_unlimited":
			impact.NewlyUnlimitedCount++
		}
	}
	return impact
}

func saasAdminLimitedPackageChangeMetrics(changes []SaaSAdminPackageLimitChange) map[string]int64 {
	metrics := map[string]int64{}
	for _, change := range changes {
		if change.After <= 0 {
			continue
		}
		if change.Direction != "decrease" && change.Direction != "newly_limited" {
			continue
		}
		metrics[change.Metric] = change.After
	}
	return metrics
}

func saasAdminPackageLimitChanges(before, after SaaSAdminPackageLimits) []SaaSAdminPackageLimitChange {
	beforeValues := saasAdminPackageLimitValues(before)
	afterValues := saasAdminPackageLimitValues(after)
	changes := make([]SaaSAdminPackageLimitChange, 0, len(afterValues))
	for i, afterValue := range afterValues {
		beforeValue := beforeValues[i]
		if beforeValue.Value == afterValue.Value {
			continue
		}
		changes = append(changes, SaaSAdminPackageLimitChange{
			Field:     afterValue.Field,
			Metric:    afterValue.Metric,
			Label:     afterValue.Label,
			Before:    beforeValue.Value,
			After:     afterValue.Value,
			Delta:     afterValue.Value - beforeValue.Value,
			Direction: saasAdminPackageLimitChangeDirection(beforeValue.Value, afterValue.Value),
		})
	}
	return changes
}

func saasAdminPackageLimitChangeDirection(before, after int64) string {
	switch {
	case before <= 0 && after > 0:
		return "newly_limited"
	case before > 0 && after <= 0:
		return "newly_unlimited"
	case after > before:
		return "increase"
	default:
		return "decrease"
	}
}

type saasAdminPackageLimitValue struct {
	Field  string
	Metric string
	Label  string
	Value  int64
}

func saasAdminPackageLimitValues(limits SaaSAdminPackageLimits) []saasAdminPackageLimitValue {
	return []saasAdminPackageLimitValue{
		{Field: "maxCorps", Metric: SaaSMetricCorps, Label: saasMetricLabel(SaaSMetricCorps), Value: limits.MaxCorps},
		{Field: "maxUsers", Metric: SaaSMetricUsers, Label: saasMetricLabel(SaaSMetricUsers), Value: limits.MaxUsers},
		{Field: "maxContacts", Metric: SaaSMetricContacts, Label: saasMetricLabel(SaaSMetricContacts), Value: limits.MaxContacts},
		{Field: "maxRooms", Metric: SaaSMetricRooms, Label: saasMetricLabel(SaaSMetricRooms), Value: limits.MaxRooms},
		{Field: "maxAgents", Metric: SaaSMetricAgents, Label: saasMetricLabel(SaaSMetricAgents), Value: limits.MaxAgents},
		{Field: "channelCodes", Metric: SaaSMetricChannelCodes, Label: saasMetricLabel(SaaSMetricChannelCodes), Value: limits.ChannelCodes},
		{Field: "shopCodes", Metric: SaaSMetricShopCodes, Label: saasMetricLabel(SaaSMetricShopCodes), Value: limits.ShopCodes},
		{Field: "radars", Metric: SaaSMetricRadars, Label: saasMetricLabel(SaaSMetricRadars), Value: limits.Radars},
		{Field: "lotteries", Metric: SaaSMetricLotteries, Label: saasMetricLabel(SaaSMetricLotteries), Value: limits.Lotteries},
		{Field: "roomInfinitePulls", Metric: SaaSMetricRoomInfinitePulls, Label: saasMetricLabel(SaaSMetricRoomInfinitePulls), Value: limits.RoomInfinitePulls},
		{Field: "roomFissions", Metric: SaaSMetricRoomFissions, Label: saasMetricLabel(SaaSMetricRoomFissions), Value: limits.RoomFissions},
		{Field: "roomClockIns", Metric: SaaSMetricRoomClockIns, Label: saasMetricLabel(SaaSMetricRoomClockIns), Value: limits.RoomClockIns},
		{Field: "roomQualities", Metric: SaaSMetricRoomQualities, Label: saasMetricLabel(SaaSMetricRoomQualities), Value: limits.RoomQualities},
		{Field: "roomCalendars", Metric: SaaSMetricRoomCalendars, Label: saasMetricLabel(SaaSMetricRoomCalendars), Value: limits.RoomCalendars},
		{Field: "roomReminds", Metric: SaaSMetricRoomReminds, Label: saasMetricLabel(SaaSMetricRoomReminds), Value: limits.RoomReminds},
		{Field: "contactSops", Metric: SaaSMetricContactSOPs, Label: saasMetricLabel(SaaSMetricContactSOPs), Value: limits.ContactSOPs},
		{Field: "roomSops", Metric: SaaSMetricRoomSOPs, Label: saasMetricLabel(SaaSMetricRoomSOPs), Value: limits.RoomSOPs},
		{Field: "sensitiveWords", Metric: SaaSMetricSensitiveWords, Label: saasMetricLabel(SaaSMetricSensitiveWords), Value: limits.SensitiveWords},
		{Field: "storageMb", Metric: SaaSMetricStorage, Label: saasMetricLabel(SaaSMetricStorage), Value: limits.StorageMB},
		{Field: "contactMessageBatches", Metric: SaaSMetricContactMessageBatches, Label: saasMetricLabel(SaaSMetricContactMessageBatches), Value: limits.ContactMessageBatches},
		{Field: "roomMessageBatches", Metric: SaaSMetricRoomMessageBatches, Label: saasMetricLabel(SaaSMetricRoomMessageBatches), Value: limits.RoomMessageBatches},
		{Field: "roomTagPulls", Metric: SaaSMetricRoomTagPulls, Label: saasMetricLabel(SaaSMetricRoomTagPulls), Value: limits.RoomTagPulls},
		{Field: "workRoomAutoPulls", Metric: SaaSMetricWorkRoomAutoPulls, Label: saasMetricLabel(SaaSMetricWorkRoomAutoPulls), Value: limits.WorkRoomAutoPulls},
		{Field: "workFissions", Metric: SaaSMetricWorkFissions, Label: saasMetricLabel(SaaSMetricWorkFissions), Value: limits.WorkFissions},
		{Field: "officialAccounts", Metric: SaaSMetricOfficialAccounts, Label: saasMetricLabel(SaaSMetricOfficialAccounts), Value: limits.OfficialAccounts},
		{Field: "asyncExecutions", Metric: SaaSMetricAsyncExecutions, Label: saasMetricLabel(SaaSMetricAsyncExecutions), Value: limits.AsyncExecutions},
	}
}

func saasAdminPackageLimitMetricMap(limits SaaSAdminPackageLimits) map[string]int64 {
	values := saasAdminPackageLimitValues(limits)
	metrics := make(map[string]int64, len(values))
	for _, value := range values {
		if value.Value <= 0 {
			continue
		}
		metrics[value.Metric] = value.Value
	}
	return metrics
}

func saasAdminPackageImpactPayload(impact SaaSAdminPackageImpact) map[string]any {
	return map[string]any{
		"existing":               impact.Existing,
		"assignedTenantCount":    impact.AssignedTenantCount,
		"checkedTenantCount":     impact.CheckedTenantCount,
		"overLimitTenantCount":   impact.OverLimitTenantCount,
		"overLimitTenants":       saasAdminPackageImpactTenantPayloads(impact.OverLimitTenants),
		"statusChanged":          impact.StatusChanged,
		"beforeStatus":           impact.BeforeStatus,
		"afterStatus":            impact.AfterStatus,
		"changedLimitCount":      impact.ChangedLimitCount,
		"increasedLimitCount":    impact.IncreasedLimitCount,
		"decreasedLimitCount":    impact.DecreasedLimitCount,
		"newlyLimitedCount":      impact.NewlyLimitedCount,
		"newlyUnlimitedCount":    impact.NewlyUnlimitedCount,
		"tenantSnapshotsUpdated": impact.TenantSnapshotsUpdated,
		"changes":                saasAdminPackageLimitChangePayloads(impact.Changes),
	}
}

func saasAdminPackageLimitChangePayloads(changes []SaaSAdminPackageLimitChange) []map[string]any {
	items := make([]map[string]any, 0, len(changes))
	for _, item := range changes {
		items = append(items, map[string]any{
			"field":     item.Field,
			"metric":    item.Metric,
			"label":     item.Label,
			"before":    item.Before,
			"after":     item.After,
			"delta":     item.Delta,
			"direction": item.Direction,
		})
	}
	return items
}

func saasAdminPackageImpactTenantPayloads(tenants []SaaSAdminPackageImpactTenant) []map[string]any {
	items := make([]map[string]any, 0, len(tenants))
	for _, item := range tenants {
		items = append(items, map[string]any{
			"tenantId":   item.TenantID,
			"tenantName": item.TenantName,
			"metric":     item.Metric,
			"label":      item.Label,
			"current":    item.Current,
			"limit":      item.Limit,
		})
	}
	return items
}

func saasAdminOperationLogPayloads(logs []SaaSAdminOperationLog) []map[string]any {
	items := make([]map[string]any, 0, len(logs))
	for _, item := range logs {
		items = append(items, saasAdminOperationLogPayload(item))
	}
	return items
}

func saasAdminOperationLogPayload(item SaaSAdminOperationLog) map[string]any {
	return map[string]any{
		"id":            item.ID,
		"tenantId":      item.TenantID,
		"actorUserId":   item.ActorUserID,
		"actorTenantId": item.ActorTenantID,
		"action":        item.Action,
		"targetType":    item.TargetType,
		"targetId":      item.TargetID,
		"targetName":    item.TargetName,
		"before":        saasAdminJSONPayload(item.BeforeJSON),
		"after":         saasAdminJSONPayload(item.AfterJSON),
		"remark":        item.Remark,
		"createdAt":     item.CreatedAt,
	}
}

func saasAdminBillingEventPayloads(events []SaaSAdminBillingEvent) []map[string]any {
	items := make([]map[string]any, 0, len(events))
	for _, item := range events {
		items = append(items, saasAdminBillingEventPayload(item))
	}
	return items
}

func saasAdminBillingEventPayload(item SaaSAdminBillingEvent) map[string]any {
	return map[string]any{
		"id":                item.ID,
		"tenantId":          item.TenantID,
		"eventType":         item.EventType,
		"packageCode":       item.PackageCode,
		"packageName":       item.PackageName,
		"previousExpiresAt": item.PreviousExpiresAt,
		"newExpiresAt":      item.NewExpiresAt,
		"amountCents":       item.AmountCents,
		"currency":          item.Currency,
		"paidAt":            item.PaidAt,
		"paymentMethod":     item.PaymentMethod,
		"externalOrderNo":   item.ExternalOrderNo,
		"actorUserId":       item.ActorUserID,
		"actorTenantId":     item.ActorTenantID,
		"remark":            item.Remark,
		"metadata":          saasAdminJSONPayload(item.MetadataJSON),
		"createdAt":         item.CreatedAt,
	}
}

func saasAdminBillingReconciliationSummaryPayload(summary SaaSAdminBillingReconciliationSummary) map[string]any {
	return map[string]any{
		"checkedCount":         summary.CheckedCount,
		"matchedCount":         summary.MatchedCount,
		"mismatchedCount":      summary.MismatchedCount,
		"missingPackageCount":  summary.MissingPackageCount,
		"inactivePackageCount": summary.InactivePackageCount,
		"packageMismatchCount": summary.PackageMismatchCount,
		"expiresMismatchCount": summary.ExpiresMismatchCount,
	}
}

func saasAdminBillingReconciliationPayloads(items []SaaSAdminBillingReconciliationItem) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		item = saasAdminBillingReconciliationItemWithStatus(item)
		payload := saasAdminBillingEventPayload(item.BillingEvent)
		payload["tenantName"] = item.TenantName
		payload["currentPackageFound"] = item.CurrentPackageFound
		payload["currentPackageCode"] = item.CurrentPackageCode
		payload["currentPackageName"] = item.CurrentPackageName
		payload["currentExpiresAt"] = item.CurrentExpiresAt
		payload["currentPackageStatus"] = item.CurrentPackageStatus
		payload["reconcileStatus"] = item.Status
		payload["mismatchReasons"] = item.Reasons
		payloads = append(payloads, payload)
	}
	return payloads
}

func saasAdminBillingReconciliationFollowUpPayload(result SaaSAdminBillingReconciliationFollowUpResult) map[string]any {
	return map[string]any{
		"operationId":    result.OperationID,
		"billingEventId": result.BillingEvent.ID,
		"tenantId":       result.BillingEvent.TenantID,
		"billingEvent":   saasAdminBillingEventPayload(result.BillingEvent),
		"status":         result.Status,
		"owner":          result.Owner,
		"nextFollowUpAt": result.NextFollowUpAt,
		"remark":         result.Remark,
		"actorUserId":    result.ActorUserID,
		"actorTenantId":  result.ActorTenantID,
		"followedUpAt":   result.FollowedUpAt,
	}
}

func saasAdminBillingReconciliationFollowUpBulkClosePayload(result SaaSAdminBillingReconciliationFollowUpBulkCloseResult) map[string]any {
	items := make([]map[string]any, 0, len(result.FollowUps))
	for _, followUp := range result.FollowUps {
		items = append(items, saasAdminBillingReconciliationFollowUpPayload(followUp))
	}
	return map[string]any{
		"closedCount": result.ClosedCount,
		"status":      result.Status,
		"remark":      result.Remark,
		"followUps":   items,
	}
}

func saasAdminBillingReconciliationFollowUpSnapshotPayload(item SaaSAdminBillingReconciliationFollowUpSnapshot) map[string]any {
	return map[string]any{
		"operationId":     item.OperationID,
		"billingEventId":  item.BillingEventID,
		"tenantId":        item.TenantID,
		"tenantName":      item.TenantName,
		"status":          item.Status,
		"owner":           item.Owner,
		"nextFollowUpAt":  item.NextFollowUpAt,
		"remark":          item.Remark,
		"packageCode":     item.PackageCode,
		"packageName":     item.PackageName,
		"newExpiresAt":    item.NewExpiresAt,
		"amountCents":     item.AmountCents,
		"currency":        item.Currency,
		"externalOrderNo": item.ExternalOrderNo,
		"createdAt":       item.CreatedAt,
	}
}

func saasAdminBillingReconciliationFollowUpTaskPayloads(tasks []SaaSAdminBillingReconciliationFollowUpTask) []map[string]any {
	items := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, saasAdminBillingReconciliationFollowUpTaskPayload(task))
	}
	return items
}

func saasAdminBillingReconciliationFollowUpTaskPayload(task SaaSAdminBillingReconciliationFollowUpTask) map[string]any {
	item := saasAdminBillingReconciliationFollowUpSnapshotPayload(task.SaaSAdminBillingReconciliationFollowUpSnapshot)
	item["dueState"] = task.DueState
	item["overdue"] = task.Overdue
	item["daysUntil"] = task.DaysUntil
	return item
}

func saasAdminNormalizeBillingReconciliationReport(report SaaSAdminBillingReconciliationReport) SaaSAdminBillingReconciliationReport {
	for i := range report.Items {
		report.Items[i] = saasAdminBillingReconciliationItemWithStatus(report.Items[i])
	}
	return report
}

func saasAdminBillingReconciliationItemWithStatus(item SaaSAdminBillingReconciliationItem) SaaSAdminBillingReconciliationItem {
	reasons := make([]string, 0, 4)
	if !item.CurrentPackageFound {
		reasons = append(reasons, "missing_package")
	} else {
		if item.CurrentPackageStatus != 1 {
			reasons = append(reasons, "inactive_package")
		}
		if strings.TrimSpace(item.BillingEvent.PackageCode) != "" && strings.TrimSpace(item.CurrentPackageCode) != strings.TrimSpace(item.BillingEvent.PackageCode) {
			reasons = append(reasons, "package_mismatch")
		}
		if strings.TrimSpace(item.BillingEvent.NewExpiresAt) != "" && strings.TrimSpace(item.CurrentExpiresAt) != "" && saasAdminDateTimeBefore(item.CurrentExpiresAt, item.BillingEvent.NewExpiresAt) {
			reasons = append(reasons, "expires_mismatch")
		}
	}
	item.Reasons = reasons
	if len(reasons) == 0 {
		item.Status = "matched"
	} else {
		item.Status = "mismatch"
	}
	return item
}

func saasAdminTaskPayloads(tasks []SaaSAdminTask) []map[string]any {
	items := make([]map[string]any, 0, len(tasks))
	for _, item := range tasks {
		items = append(items, saasAdminTaskPayload(item))
	}
	return items
}

func saasAdminTaskPayload(item SaaSAdminTask) map[string]any {
	canApply := item.ID > 0 && item.Status != SaaSAdminTaskStatusApplied && item.Status != SaaSAdminTaskStatusCanceled
	canCancel := item.ID > 0 && item.Status != SaaSAdminTaskStatusApplied && item.Status != SaaSAdminTaskStatusCanceled
	return map[string]any{
		"id":            item.ID,
		"taskType":      item.TaskType,
		"status":        item.Status,
		"version":       item.Version,
		"tenantId":      item.TenantID,
		"packageCode":   item.PackageCode,
		"actorUserId":   item.ActorUserID,
		"actorTenantId": item.ActorTenantID,
		"request":       saasAdminTaskRequestPayload(item),
		"preview":       saasAdminJSONPayload(item.PreviewJSON),
		"result":        saasAdminJSONPayload(item.ResultJSON),
		"remark":        item.Remark,
		"lastError":     item.LastError,
		"appliedAt":     item.AppliedAt,
		"createdAt":     item.CreatedAt,
		"updatedAt":     item.UpdatedAt,
		"canApply":      canApply,
		"canCancel":     canCancel,
	}
}

func saasAdminTaskRequestPayload(item SaaSAdminTask) any {
	request := saasAdminJSONPayload(item.RequestJSON)
	if item.TaskType != SaaSAdminTaskTypeTenantProvision {
		return request
	}
	payload, ok := request.(map[string]any)
	if !ok {
		return map[string]any{"redacted": true}
	}
	hasPasswordHash := false
	for _, key := range []string{"adminPasswordHash", "admin_password_hash", "passwordHash", "password_hash"} {
		if _, exists := payload[key]; exists {
			hasPasswordHash = true
			delete(payload, key)
		}
	}
	delete(payload, "password")
	if hasPasswordHash {
		payload["hasAdminPasswordHash"] = true
	}
	return payload
}

type saasAdminTaskOwnerKey struct {
	actorUserID   int
	actorTenantID int
}

type saasAdminTaskOwnerAccumulator struct {
	item      SaaSAdminTaskOwnerSummary
	tenantIDs map[int]struct{}
	actorIDs  map[int]struct{}
}

func saasAdminBuildTaskOwnerReport(options SaaSAdminTaskOptions, summary SaaSAdminTaskSummary, tasks []SaaSAdminTask, recentLimit int) SaaSAdminTaskOwnerReport {
	if recentLimit <= 0 {
		recentLimit = 3
	}
	byOwner := map[saasAdminTaskOwnerKey]*saasAdminTaskOwnerAccumulator{}
	for _, task := range tasks {
		key := saasAdminTaskOwnerKey{actorUserID: task.ActorUserID, actorTenantID: task.ActorTenantID}
		acc, exists := byOwner[key]
		if !exists {
			acc = &saasAdminTaskOwnerAccumulator{
				item: SaaSAdminTaskOwnerSummary{
					Owner:         saasAdminTaskOwnerLabel(task.ActorUserID, task.ActorTenantID),
					ActorUserID:   task.ActorUserID,
					ActorTenantID: task.ActorTenantID,
				},
				tenantIDs: map[int]struct{}{},
				actorIDs:  map[int]struct{}{},
			}
			byOwner[key] = acc
		}
		saasAdminAddTaskToSummary(&acc.item.Summary, task)
		if task.TenantID > 0 {
			acc.tenantIDs[task.TenantID] = struct{}{}
		}
		if task.ActorUserID > 0 {
			acc.actorIDs[task.ActorUserID] = struct{}{}
		}
		if len(acc.item.RecentTasks) < recentLimit {
			acc.item.RecentTasks = append(acc.item.RecentTasks, task)
		}
		if task.LastError != "" && acc.item.LastError == "" {
			acc.item.LastError = task.LastError
		}
		if task.AppliedAt > acc.item.LastAppliedAt {
			acc.item.LastAppliedAt = task.AppliedAt
		}
		if lastTaskAt := saasAdminTaskLastActivityAt(task); lastTaskAt > acc.item.LastTaskAt {
			acc.item.LastTaskAt = lastTaskAt
		}
	}

	owners := make([]SaaSAdminTaskOwnerSummary, 0, len(byOwner))
	for _, acc := range byOwner {
		acc.item.Summary.TenantCount = len(acc.tenantIDs)
		acc.item.Summary.ActorUserCount = len(acc.actorIDs)
		owners = append(owners, acc.item)
	}
	sort.Slice(owners, func(i, j int) bool {
		left := owners[i].Summary
		right := owners[j].Summary
		if left.ActionableCount != right.ActionableCount {
			return left.ActionableCount > right.ActionableCount
		}
		leftRisk := left.BlockedCount + left.FailedCount
		rightRisk := right.BlockedCount + right.FailedCount
		if leftRisk != rightRisk {
			return leftRisk > rightRisk
		}
		if left.TaskCount != right.TaskCount {
			return left.TaskCount > right.TaskCount
		}
		if owners[i].LastTaskAt != owners[j].LastTaskAt {
			return owners[i].LastTaskAt > owners[j].LastTaskAt
		}
		if owners[i].ActorTenantID != owners[j].ActorTenantID {
			return owners[i].ActorTenantID < owners[j].ActorTenantID
		}
		return owners[i].ActorUserID < owners[j].ActorUserID
	})

	ownerCount := len(owners)
	if options.Limit > 0 && len(owners) > options.Limit {
		owners = owners[:options.Limit]
	}
	return SaaSAdminTaskOwnerReport{
		Options:          options,
		Summary:          summary,
		OwnerCount:       ownerCount,
		ScannedTaskCount: len(tasks),
		Partial:          summary.TaskCount > len(tasks),
		Owners:           owners,
	}
}

func saasAdminTaskOwnerLabel(actorUserID int, actorTenantID int) string {
	switch {
	case actorUserID > 0 && actorTenantID > 0:
		return fmt.Sprintf("用户 #%d / 租户 #%d", actorUserID, actorTenantID)
	case actorUserID > 0:
		return fmt.Sprintf("用户 #%d", actorUserID)
	case actorTenantID > 0:
		return fmt.Sprintf("租户 #%d 未记录用户", actorTenantID)
	default:
		return "未分配"
	}
}

func saasAdminAddTaskToSummary(summary *SaaSAdminTaskSummary, task SaaSAdminTask) {
	summary.TaskCount++
	switch task.Status {
	case SaaSAdminTaskStatusPending:
		summary.PendingCount++
		summary.ActionableCount++
	case SaaSAdminTaskStatusBlocked:
		summary.BlockedCount++
		summary.ActionableCount++
	case SaaSAdminTaskStatusFailed:
		summary.FailedCount++
		summary.ActionableCount++
	case SaaSAdminTaskStatusApplied:
		summary.AppliedCount++
	case SaaSAdminTaskStatusCanceled:
		summary.CanceledCount++
	}
	switch task.TaskType {
	case SaaSAdminTaskTypePackageSync:
		summary.PackageSyncCount++
	case SaaSAdminTaskTypeTenantRenewal:
		summary.TenantRenewalCount++
	case SaaSAdminTaskTypeTenantProvision:
		summary.TenantProvisionCount++
	}
}

func saasAdminTaskLastActivityAt(task SaaSAdminTask) string {
	latest := task.CreatedAt
	if task.UpdatedAt > latest {
		latest = task.UpdatedAt
	}
	if task.AppliedAt > latest {
		latest = task.AppliedAt
	}
	return latest
}

type saasAdminTaskSLAOwnerAccumulator struct {
	item      SaaSAdminTaskSLAOwnerSummary
	tenantIDs map[int]struct{}
	actorIDs  map[int]struct{}
}

func saasAdminBuildTaskSLAReport(options SaaSAdminTaskSLAOptions, taskSummary SaaSAdminTaskSummary, tasks []SaaSAdminTask, now time.Time, ownerTaskLimit int) SaaSAdminTaskSLAReport {
	if ownerTaskLimit <= 0 {
		ownerTaskLimit = 3
	}
	slaItems := make([]SaaSAdminTaskSLAItem, 0, len(tasks))
	for _, task := range tasks {
		if !saasAdminTaskSLAActive(task) {
			continue
		}
		slaItems = append(slaItems, saasAdminTaskSLAItem(task, options, now))
	}
	sort.Slice(slaItems, func(i, j int) bool {
		if rankI, rankJ := saasAdminTaskSLAStatusRank(slaItems[i].SLAStatus), saasAdminTaskSLAStatusRank(slaItems[j].SLAStatus); rankI != rankJ {
			return rankI > rankJ
		}
		if slaItems[i].AgeHours != slaItems[j].AgeHours {
			return slaItems[i].AgeHours > slaItems[j].AgeHours
		}
		return slaItems[i].Task.ID > slaItems[j].Task.ID
	})

	report := SaaSAdminTaskSLAReport{
		Options:          options,
		TaskSummary:      taskSummary,
		ScannedTaskCount: len(tasks),
		Partial:          taskSummary.TaskCount > len(tasks),
	}
	tenantIDs := map[int]struct{}{}
	actorIDs := map[int]struct{}{}
	byOwner := map[saasAdminTaskOwnerKey]*saasAdminTaskSLAOwnerAccumulator{}
	for _, item := range slaItems {
		saasAdminAddTaskSLAItemToSummary(&report.Summary, item)
		if item.Task.TenantID > 0 {
			tenantIDs[item.Task.TenantID] = struct{}{}
		}
		if item.Task.ActorUserID > 0 {
			actorIDs[item.Task.ActorUserID] = struct{}{}
		}
		key := saasAdminTaskOwnerKey{actorUserID: item.Task.ActorUserID, actorTenantID: item.Task.ActorTenantID}
		acc, exists := byOwner[key]
		if !exists {
			acc = &saasAdminTaskSLAOwnerAccumulator{
				item: SaaSAdminTaskSLAOwnerSummary{
					Owner:         item.Owner,
					ActorUserID:   item.Task.ActorUserID,
					ActorTenantID: item.Task.ActorTenantID,
				},
				tenantIDs: map[int]struct{}{},
				actorIDs:  map[int]struct{}{},
			}
			byOwner[key] = acc
		}
		saasAdminAddTaskSLAItemToSummary(&acc.item.Summary, item)
		if item.Task.TenantID > 0 {
			acc.tenantIDs[item.Task.TenantID] = struct{}{}
		}
		if item.Task.ActorUserID > 0 {
			acc.actorIDs[item.Task.ActorUserID] = struct{}{}
		}
		if acc.item.OldestTaskAt == "" || item.Task.CreatedAt < acc.item.OldestTaskAt {
			acc.item.OldestTaskAt = item.Task.CreatedAt
		}
		if len(acc.item.Tasks) < ownerTaskLimit {
			acc.item.Tasks = append(acc.item.Tasks, item)
		}
	}
	report.Summary.TenantCount = len(tenantIDs)
	report.Summary.ActorUserCount = len(actorIDs)

	owners := make([]SaaSAdminTaskSLAOwnerSummary, 0, len(byOwner))
	for _, acc := range byOwner {
		acc.item.Summary.TenantCount = len(acc.tenantIDs)
		acc.item.Summary.ActorUserCount = len(acc.actorIDs)
		owners = append(owners, acc.item)
	}
	sort.Slice(owners, func(i, j int) bool {
		left := owners[i].Summary
		right := owners[j].Summary
		if left.OverdueCount != right.OverdueCount {
			return left.OverdueCount > right.OverdueCount
		}
		if left.WarningCount != right.WarningCount {
			return left.WarningCount > right.WarningCount
		}
		if left.MaxAgeHours != right.MaxAgeHours {
			return left.MaxAgeHours > right.MaxAgeHours
		}
		if left.TaskCount != right.TaskCount {
			return left.TaskCount > right.TaskCount
		}
		if owners[i].ActorTenantID != owners[j].ActorTenantID {
			return owners[i].ActorTenantID < owners[j].ActorTenantID
		}
		return owners[i].ActorUserID < owners[j].ActorUserID
	})
	report.OwnerCount = len(owners)
	if options.Limit > 0 && len(owners) > options.Limit {
		owners = owners[:options.Limit]
	}
	if options.Limit > 0 && len(slaItems) > options.Limit {
		slaItems = slaItems[:options.Limit]
	}
	report.Owners = owners
	report.Tasks = slaItems
	return report
}

func saasAdminTaskSLAActive(task SaaSAdminTask) bool {
	switch task.Status {
	case SaaSAdminTaskStatusPending, SaaSAdminTaskStatusBlocked, SaaSAdminTaskStatusFailed:
		return true
	default:
		return false
	}
}

func saasAdminTaskSLAItem(task SaaSAdminTask, options SaaSAdminTaskSLAOptions, now time.Time) SaaSAdminTaskSLAItem {
	item := SaaSAdminTaskSLAItem{
		Task:  task,
		Owner: saasAdminTaskOwnerLabel(task.ActorUserID, task.ActorTenantID),
	}
	createdAt, ok := parseSaaSAdminNormalizedDateTime(task.CreatedAt)
	if !ok {
		item.SLAStatus = "unknown"
		return item
	}
	if createdAt.After(now) {
		createdAt = now
	}
	ageHours := int(now.Sub(createdAt).Hours())
	item.AgeHours = ageHours
	switch {
	case ageHours >= options.OverdueHours:
		item.SLAStatus = "overdue"
		item.BreachHours = ageHours - options.OverdueHours
	case ageHours >= options.WarningHours:
		item.SLAStatus = "warning"
	default:
		item.SLAStatus = "fresh"
	}
	return item
}

func saasAdminAddTaskSLAItemToSummary(summary *SaaSAdminTaskSLASummary, item SaaSAdminTaskSLAItem) {
	summary.TaskCount++
	switch item.SLAStatus {
	case "overdue":
		summary.OverdueCount++
	case "warning":
		summary.WarningCount++
	case "fresh":
		summary.FreshCount++
	default:
		summary.UnknownAgeCount++
	}
	switch item.Task.Status {
	case SaaSAdminTaskStatusPending:
		summary.PendingCount++
	case SaaSAdminTaskStatusBlocked:
		summary.BlockedCount++
	case SaaSAdminTaskStatusFailed:
		summary.FailedCount++
	}
	switch item.Task.TaskType {
	case SaaSAdminTaskTypePackageSync:
		summary.PackageSyncCount++
	case SaaSAdminTaskTypeTenantRenewal:
		summary.TenantRenewalCount++
	case SaaSAdminTaskTypeTenantProvision:
		summary.TenantProvisionCount++
	}
	if item.AgeHours > summary.MaxAgeHours {
		summary.MaxAgeHours = item.AgeHours
	}
}

func saasAdminTaskSLAStatusRank(status string) int {
	switch status {
	case "overdue":
		return 4
	case "warning":
		return 3
	case "unknown":
		return 2
	case "fresh":
		return 1
	default:
		return 0
	}
}

func saasAdminTaskBulkCancelPayload(result SaaSAdminTaskBulkCancelResult) map[string]any {
	return map[string]any{
		"canceled":               result.CanceledCount > 0,
		"matchedCount":           result.MatchedCount,
		"canceledCount":          result.CanceledCount,
		"skippedAppliedCount":    result.SkippedAppliedCount,
		"skippedCanceledCount":   result.SkippedCanceledCount,
		"skippedIneligibleCount": result.SkippedIneligibleCount,
		"filters":                saasAdminTaskFiltersPayload(result.Options),
		"remark":                 result.Remark,
		"tasks":                  saasAdminTaskPayloads(result.Tasks),
	}
}

func saasAdminTaskBulkResetPayload(result SaaSAdminTaskBulkResetResult) map[string]any {
	return map[string]any{
		"reset":                  result.ResetCount > 0,
		"matchedCount":           result.MatchedCount,
		"resetCount":             result.ResetCount,
		"skippedPendingCount":    result.SkippedPendingCount,
		"skippedAppliedCount":    result.SkippedAppliedCount,
		"skippedCanceledCount":   result.SkippedCanceledCount,
		"skippedIneligibleCount": result.SkippedIneligibleCount,
		"filters":                saasAdminTaskFiltersPayload(result.Options),
		"remark":                 result.Remark,
		"tasks":                  saasAdminTaskPayloads(result.Tasks),
	}
}

func saasAdminTaskBulkApplyPayload(result SaaSAdminTaskBulkApplyResult) map[string]any {
	return map[string]any{
		"applied":                 result.AppliedCount > 0,
		"matchedCount":            result.MatchedCount,
		"appliedCount":            result.AppliedCount,
		"blockedCount":            result.BlockedCount,
		"failedCount":             result.FailedCount,
		"skippedAppliedCount":     result.SkippedAppliedCount,
		"skippedCanceledCount":    result.SkippedCanceledCount,
		"skippedUnsupportedCount": result.SkippedUnsupportedCount,
		"filters":                 saasAdminTaskFiltersPayload(result.Options),
		"remark":                  result.Remark,
		"tasks":                   saasAdminTaskPayloads(result.Tasks),
		"errors":                  saasAdminTaskBulkApplyErrorPayloads(result.Errors),
	}
}

func saasAdminTaskBulkApplyErrorPayloads(errors []SaaSAdminTaskBulkApplyError) []map[string]any {
	payloads := make([]map[string]any, 0, len(errors))
	for _, item := range errors {
		payloads = append(payloads, map[string]any{
			"taskId":   item.TaskID,
			"tenantId": item.TenantID,
			"error":    item.Error,
		})
	}
	return payloads
}

func saasAdminCSVJSONField(value any) string {
	if value == nil {
		return ""
	}
	if raw, ok := value.(string); ok {
		return raw
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func saasAdminJSONPayload(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	return value
}

func saasAdminPayloadJSON(value any) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func saasAdminPackageLimitsPayload(limits SaaSAdminPackageLimits) map[string]any {
	return map[string]any{
		"maxCorps":              limits.MaxCorps,
		"maxUsers":              limits.MaxUsers,
		"maxContacts":           limits.MaxContacts,
		"maxRooms":              limits.MaxRooms,
		"maxAgents":             limits.MaxAgents,
		"channelCodes":          limits.ChannelCodes,
		"shopCodes":             limits.ShopCodes,
		"radars":                limits.Radars,
		"lotteries":             limits.Lotteries,
		"roomInfinitePulls":     limits.RoomInfinitePulls,
		"roomFissions":          limits.RoomFissions,
		"roomClockIns":          limits.RoomClockIns,
		"roomQualities":         limits.RoomQualities,
		"roomCalendars":         limits.RoomCalendars,
		"roomReminds":           limits.RoomReminds,
		"contactSops":           limits.ContactSOPs,
		"roomSops":              limits.RoomSOPs,
		"sensitiveWords":        limits.SensitiveWords,
		"storageMb":             limits.StorageMB,
		"contactMessageBatches": limits.ContactMessageBatches,
		"roomMessageBatches":    limits.RoomMessageBatches,
		"roomTagPulls":          limits.RoomTagPulls,
		"workRoomAutoPulls":     limits.WorkRoomAutoPulls,
		"workFissions":          limits.WorkFissions,
		"officialAccounts":      limits.OfficialAccounts,
		"asyncExecutions":       limits.AsyncExecutions,
	}
}

func saasAdminTenantPayloads(tenants []SaaSAdminTenantOverview) []map[string]any {
	items := make([]map[string]any, 0, len(tenants))
	for _, tenant := range tenants {
		items = append(items, saasAdminTenantPayload(tenant))
	}
	return items
}

func saasAdminTenantPayload(tenant SaaSAdminTenantOverview) map[string]any {
	return map[string]any{
		"tenantId":        tenant.TenantID,
		"tenantName":      tenant.TenantName,
		"tenantStatus":    tenant.TenantStatus,
		"packageCode":     tenant.PackageCode,
		"packageName":     tenant.PackageName,
		"packageStatus":   tenant.PackageStatus,
		"packageVersion":  tenant.PackageVersion,
		"expiresAt":       tenant.ExpiresAt,
		"expired":         tenant.Expired,
		"expiringSoon":    tenant.ExpiringSoon,
		"openAlertCount":  tenant.OpenAlertCount,
		"maxUsageMetric":  tenant.MaxUsageMetric,
		"maxUsageLabel":   saasMetricLabel(tenant.MaxUsageMetric),
		"maxUsageCurrent": tenant.MaxUsageCurrent,
		"maxUsageLimit":   tenant.MaxUsageLimit,
		"maxUsageRatio":   tenant.MaxUsageRatio,
	}
}

func saasAdminTenantDetailPayload(detail SaaSAdminTenantDetail) map[string]any {
	return map[string]any{
		"tenantId":              detail.Tenant.TenantID,
		"canPlatformScope":      detail.CanPlatformScope,
		"platformAdminTenantId": detail.PlatformAdminTenantID,
		"summary":               saasAdminSummaryPayload(detail.Summary),
		"tenant":                saasAdminTenantPayload(detail.Tenant),
		"metrics":               saasAdminMetricPayloads(detail.Metrics),
		"operations":            saasAdminOperationLogPayloads(detail.Operations),
	}
}

func saasAdminTenantLifecyclePayload(lifecycle SaaSAdminTenantLifecycle, limit int) map[string]any {
	timeline, timelineCount, rawTimelineCount := saasAdminTenantLifecycleTimeline(lifecycle, limit)
	return map[string]any{
		"tenantId":              lifecycle.Tenant.TenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"canPlatformScope":      true,
		"platformAdminTenantId": lifecycle.PlatformAdminTenantID,
		"filters": map[string]any{
			"tenantId":  lifecycle.Tenant.TenantID,
			"limit":     limit,
			"source":    lifecycle.Filter.Source,
			"eventType": lifecycle.Filter.EventType,
			"status":    lifecycle.Filter.Status,
			"keyword":   lifecycle.Filter.Keyword,
		},
		"summary": map[string]any{
			"operationCount":     len(lifecycle.Operations),
			"billingEventCount":  len(lifecycle.BillingEvents),
			"taskCount":          len(lifecycle.Tasks),
			"alertCount":         len(lifecycle.Alerts),
			"notificationCount":  len(lifecycle.Notifications),
			"timelineCount":      timelineCount,
			"rawTimelineCount":   rawTimelineCount,
			"returnedEventCount": len(timeline),
			"filterActive":       saasAdminTenantLifecycleFilterActive(lifecycle.Filter),
		},
		"tenant":        saasAdminTenantPayload(lifecycle.Tenant),
		"timeline":      saasAdminTenantLifecycleEventPayloads(timeline),
		"operations":    saasAdminOperationLogPayloads(lifecycle.Operations),
		"billingEvents": saasAdminBillingEventPayloads(lifecycle.BillingEvents),
		"tasks":         saasAdminTaskPayloads(lifecycle.Tasks),
		"alerts":        saasAdminAlertPayloads(lifecycle.Alerts),
		"notifications": saasAdminAlertNotificationPayloads(lifecycle.Notifications),
	}
}

func saasAdminTenantLifecycleTimeline(lifecycle SaaSAdminTenantLifecycle, limit int) ([]SaaSAdminTenantLifecycleEvent, int, int) {
	events := make([]SaaSAdminTenantLifecycleEvent, 0, len(lifecycle.Operations)+len(lifecycle.BillingEvents)+len(lifecycle.Tasks)+len(lifecycle.Alerts)+len(lifecycle.Notifications))
	for _, item := range lifecycle.Operations {
		title := item.TargetName
		if title == "" {
			title = item.Action
		}
		events = append(events, SaaSAdminTenantLifecycleEvent{
			Source:      "operation",
			EventType:   item.Action,
			Title:       title,
			Status:      saasAdminTenantLifecycleOperationStatus(item),
			OccurredAt:  item.CreatedAt,
			ReferenceID: strconv.FormatInt(item.ID, 10),
			ActorUserID: item.ActorUserID,
			Remark:      item.Remark,
			Payload:     saasAdminOperationLogPayload(item),
		})
	}
	for _, item := range lifecycle.BillingEvents {
		events = append(events, SaaSAdminTenantLifecycleEvent{
			Source:      "billing",
			EventType:   item.EventType,
			Title:       item.PackageName,
			Status:      item.Currency,
			OccurredAt:  item.CreatedAt,
			ReferenceID: strconv.FormatInt(item.ID, 10),
			ActorUserID: item.ActorUserID,
			Remark:      item.Remark,
			Payload:     saasAdminBillingEventPayload(item),
		})
	}
	for _, item := range lifecycle.Tasks {
		occurredAt := saasAdminFirstNonEmpty(item.AppliedAt, item.UpdatedAt, item.CreatedAt)
		events = append(events, SaaSAdminTenantLifecycleEvent{
			Source:      "task",
			EventType:   item.TaskType,
			Title:       item.TaskType,
			Status:      item.Status,
			OccurredAt:  occurredAt,
			ReferenceID: strconv.FormatInt(item.ID, 10),
			ActorUserID: item.ActorUserID,
			Remark:      item.Remark,
			Payload:     saasAdminTaskPayload(item),
		})
	}
	for _, item := range lifecycle.Alerts {
		payload := saasAlertPayload(item)
		payload["metricLabel"] = saasMetricLabel(item.Metric)
		events = append(events, SaaSAdminTenantLifecycleEvent{
			Source:      "alert",
			EventType:   item.AlertType,
			Title:       saasMetricLabel(item.Metric),
			Status:      item.Status,
			OccurredAt:  saasAdminFirstNonEmpty(item.ResolvedAt, item.LastSeenAt, item.CreatedAt),
			ReferenceID: strconv.FormatInt(item.ID, 10),
			Remark:      item.Message,
			Payload:     payload,
		})
	}
	for _, item := range lifecycle.Notifications {
		eventType := item.Alert.AlertType
		if eventType == "" {
			eventType = item.Channel
		}
		events = append(events, SaaSAdminTenantLifecycleEvent{
			Source:      "notification",
			EventType:   eventType,
			Title:       item.NotificationKey,
			Status:      item.Status,
			OccurredAt:  saasAdminFirstNonEmpty(item.UpdatedAt, item.CreatedAt),
			ReferenceID: strconv.FormatInt(item.ID, 10),
			Remark:      item.LastError,
			Payload:     saasAdminAlertNotificationPayload(item),
		})
	}
	sort.SliceStable(events, func(i, j int) bool {
		left, leftOK := parseSaaSAdminNormalizedDateTime(events[i].OccurredAt)
		right, rightOK := parseSaaSAdminNormalizedDateTime(events[j].OccurredAt)
		if leftOK && rightOK && !left.Equal(right) {
			return left.After(right)
		}
		if events[i].OccurredAt != events[j].OccurredAt {
			return events[i].OccurredAt > events[j].OccurredAt
		}
		if events[i].Source != events[j].Source {
			return events[i].Source < events[j].Source
		}
		return events[i].ReferenceID > events[j].ReferenceID
	})
	rawTotal := len(events)
	if saasAdminTenantLifecycleFilterActive(lifecycle.Filter) {
		events = filterSaaSAdminTenantLifecycleEvents(events, lifecycle.Filter)
	}
	total := len(events)
	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}
	return events, total, rawTotal
}

func saasAdminTenantLifecycleOperationStatus(item SaaSAdminOperationLog) string {
	if item.Action != SaaSAdminOperationActionOperationQueueAssign {
		return ""
	}
	assignment, ok := saasAdminOperationQueueAssignmentFromLog(item)
	if !ok {
		return ""
	}
	return assignment.Status
}

func saasAdminTenantLifecycleFilterActive(filter SaaSAdminTenantLifecycleFilter) bool {
	return filter.Source != "" || filter.EventType != "" || filter.Status != "" || filter.Keyword != ""
}

func filterSaaSAdminTenantLifecycleEvents(events []SaaSAdminTenantLifecycleEvent, filter SaaSAdminTenantLifecycleFilter) []SaaSAdminTenantLifecycleEvent {
	filtered := make([]SaaSAdminTenantLifecycleEvent, 0, len(events))
	eventType := strings.ToLower(strings.TrimSpace(filter.EventType))
	status := strings.ToLower(strings.TrimSpace(filter.Status))
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	for _, item := range events {
		if filter.Source != "" && item.Source != filter.Source {
			continue
		}
		if eventType != "" && !strings.Contains(strings.ToLower(item.EventType), eventType) {
			continue
		}
		if status != "" && !strings.Contains(strings.ToLower(item.Status), status) {
			continue
		}
		if keyword != "" && !saasAdminTenantLifecycleEventContains(item, keyword) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func saasAdminTenantLifecycleEventContains(item SaaSAdminTenantLifecycleEvent, keyword string) bool {
	fields := []string{
		item.Source,
		item.EventType,
		item.Title,
		item.Status,
		item.OccurredAt,
		item.ReferenceID,
		strconv.Itoa(item.ActorUserID),
		item.Remark,
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), keyword) {
			return true
		}
	}
	if item.Payload == nil {
		return false
	}
	payload, err := json.Marshal(item.Payload)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(payload)), keyword)
}

func saasAdminTenantLifecycleEventPayloads(events []SaaSAdminTenantLifecycleEvent) []map[string]any {
	items := make([]map[string]any, 0, len(events))
	for _, item := range events {
		items = append(items, map[string]any{
			"source":      item.Source,
			"eventType":   item.EventType,
			"title":       item.Title,
			"status":      item.Status,
			"occurredAt":  item.OccurredAt,
			"referenceId": item.ReferenceID,
			"actorUserId": item.ActorUserID,
			"remark":      item.Remark,
			"payload":     item.Payload,
		})
	}
	return items
}

func saasAdminTenantUsagePayload(detail SaaSAdminTenantUsageDetail) map[string]any {
	return map[string]any{
		"tenantId":              detail.Tenant.TenantID,
		"canPlatformScope":      detail.CanPlatformScope,
		"platformAdminTenantId": detail.PlatformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"tenant":                saasAdminTenantPayload(detail.Tenant),
		"summary":               saasAdminUsageSummaryPayload(detail.Summary),
		"usageMetrics":          saasAdminUsageMetricPayloads(detail.Metrics),
	}
}

func saasAdminRiskReportPayload(report SaaSAdminRiskReport, options SaaSAdminRiskOptions, canPlatformScope bool, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"scope":                 options.Scope,
		"tenantId":              options.TenantID,
		"canPlatformScope":      canPlatformScope,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminRiskFiltersPayload(options),
		"summary":               saasAdminRiskSummaryPayload(report.Summary),
		"riskTenants":           saasAdminRiskTenantPayloads(report.Items),
	}
}

func saasAdminBusinessMetricsReportPayload(report SaaSAdminBusinessMetricsReport, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"estimated":             true,
		"filters":               saasAdminBusinessMetricsFiltersPayload(report.Options),
		"summary":               saasAdminBusinessMetricsSummaryPayload(report.Summary),
		"packageCount":          len(report.Packages),
		"packages":              saasAdminBusinessPackageMetricPayloads(report.Packages),
		"recentBillingEvents":   saasAdminBillingEventPayloads(saasAdminLimitBillingEvents(report.BillingEvents, 10)),
	}
}

func saasAdminBusinessMetricsSummaryPayload(summary SaaSAdminBusinessMetricsSummary) map[string]any {
	return map[string]any{
		"tenantCount":                summary.TenantCount,
		"activeTenantPackageCount":   summary.ActiveTenantPackageCount,
		"pricedTenantCount":          summary.PricedTenantCount,
		"unknownPriceTenantCount":    summary.UnknownPriceTenantCount,
		"estimatedMrrCents":          summary.EstimatedMRRCents,
		"estimatedArrCents":          summary.EstimatedARRCents,
		"estimatedArpaCents":         summary.EstimatedARPACents,
		"atRiskTenantCount":          summary.AtRiskTenantCount,
		"atRiskMrrCents":             summary.AtRiskMRRCents,
		"expiringSoonTenantCount":    summary.ExpiringSoonTenantCount,
		"expiringSoonMrrCents":       summary.ExpiringSoonMRRCents,
		"expiredTenantCount":         summary.ExpiredTenantCount,
		"expiredMrrCents":            summary.ExpiredMRRCents,
		"recentBillingEventCount":    summary.RecentBillingEventCount,
		"recentRenewalCount":         summary.RecentRenewalCount,
		"recentRefundCount":          summary.RecentRefundCount,
		"recentGrossAmountCents":     summary.RecentGrossAmountCents,
		"recentRefundAmountCents":    summary.RecentRefundAmountCents,
		"recentBillingAmountCents":   summary.RecentBillingAmountCents,
		"billingPricePackageCount":   summary.BillingPricePackageCount,
		"missingBillingPackageCount": summary.MissingBillingPackageCount,
	}
}

func saasAdminBusinessTrendReportPayload(report SaaSAdminBusinessTrendReport, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminBusinessTrendFiltersPayload(report.Options),
		"summary":               saasAdminBusinessTrendSummaryPayload(report.Summary),
		"months":                saasAdminBusinessTrendMonthPayloads(report.Months),
		"renewalFunnel": map[string]any{
			"summary":     saasAdminTaskSummaryPayload(report.RenewalFunnel.Summary),
			"recentTasks": saasAdminTaskPayloads(saasAdminLimitTasks(report.RenewalFunnel.RecentTasks, 10)),
		},
	}
}

func saasAdminBusinessTrendSummaryPayload(summary SaaSAdminBusinessTrendSummary) map[string]any {
	return map[string]any{
		"monthCount":          summary.MonthCount,
		"billingEventCount":   summary.BillingEventCount,
		"renewalCount":        summary.RenewalCount,
		"refundCount":         summary.RefundCount,
		"grossAmountCents":    summary.GrossAmountCents,
		"refundAmountCents":   summary.RefundAmountCents,
		"billingAmountCents":  summary.BillingAmountCents,
		"tenantCount":         summary.TenantCount,
		"packageCount":        summary.PackageCount,
		"taskCount":           summary.TaskCount,
		"pendingTaskCount":    summary.PendingTaskCount,
		"blockedTaskCount":    summary.BlockedTaskCount,
		"failedTaskCount":     summary.FailedTaskCount,
		"appliedTaskCount":    summary.AppliedTaskCount,
		"canceledTaskCount":   summary.CanceledTaskCount,
		"actionableTaskCount": summary.ActionableTaskCount,
	}
}

func saasAdminBusinessTrendMonthPayloads(items []SaaSAdminBusinessTrendMonth) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"month":             item.Month,
			"eventCount":        item.EventCount,
			"renewalCount":      item.RenewalCount,
			"refundCount":       item.RefundCount,
			"grossAmountCents":  item.GrossAmountCents,
			"refundAmountCents": item.RefundAmountCents,
			"amountCents":       item.AmountCents,
			"tenantCount":       item.TenantCount,
			"packageCount":      item.PackageCount,
			"packages":          saasAdminBusinessTrendPackagePayloads(item.Packages),
		})
	}
	return payloads
}

func saasAdminBusinessTrendPackagePayloads(items []SaaSAdminBusinessTrendPackage) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"packageCode":       item.PackageCode,
			"packageName":       item.PackageName,
			"eventCount":        item.EventCount,
			"renewalCount":      item.RenewalCount,
			"refundCount":       item.RefundCount,
			"grossAmountCents":  item.GrossAmountCents,
			"refundAmountCents": item.RefundAmountCents,
			"amountCents":       item.AmountCents,
		})
	}
	return payloads
}

func saasAdminRenewalForecastReportPayload(report SaaSAdminRenewalForecastReport, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"estimated":             true,
		"filters":               saasAdminRenewalForecastFiltersPayload(report.Options),
		"summary":               saasAdminRenewalForecastSummaryPayload(report.Summary),
		"buckets":               saasAdminRenewalForecastBucketPayloads(report.Buckets),
		"owners":                saasAdminRenewalForecastOwnerPayloads(report.Owners),
		"tenants":               saasAdminRenewalForecastTenantPayloads(report.Items),
		"renewalTasks": map[string]any{
			"summary":     saasAdminTaskSummaryPayload(report.TaskSummary),
			"recentTasks": saasAdminTaskPayloads(saasAdminLimitTasks(report.RecentTasks, 10)),
		},
		"recentBillingEvents": saasAdminBillingEventPayloads(saasAdminLimitBillingEvents(report.BillingEvents, 10)),
	}
}

func saasAdminRenewalForecastSummaryPayload(summary SaaSAdminRenewalForecastSummary) map[string]any {
	return map[string]any{
		"tenantCount":             summary.TenantCount,
		"forecastTenantCount":     summary.ForecastTenantCount,
		"pricedTenantCount":       summary.PricedTenantCount,
		"unknownPriceTenantCount": summary.UnknownPriceTenantCount,
		"renewalAmountCents":      summary.RenewalAmountCents,
		"estimatedMrrCents":       summary.EstimatedMRRCents,
		"expiredTenantCount":      summary.ExpiredTenantCount,
		"expiredAmountCents":      summary.ExpiredAmountCents,
		"dueWithin30TenantCount":  summary.DueWithin30TenantCount,
		"dueWithin30AmountCents":  summary.DueWithin30AmountCents,
		"due31To60TenantCount":    summary.Due31To60TenantCount,
		"due31To60AmountCents":    summary.Due31To60AmountCents,
		"due61To90TenantCount":    summary.Due61To90TenantCount,
		"due61To90AmountCents":    summary.Due61To90AmountCents,
		"dueLaterTenantCount":     summary.DueLaterTenantCount,
		"dueLaterAmountCents":     summary.DueLaterAmountCents,
		"actionableTaskCount":     summary.ActionableTaskCount,
		"pendingTaskCount":        summary.PendingTaskCount,
		"blockedTaskCount":        summary.BlockedTaskCount,
		"failedTaskCount":         summary.FailedTaskCount,
		"appliedTaskCount":        summary.AppliedTaskCount,
		"canceledTaskCount":       summary.CanceledTaskCount,
	}
}

func saasAdminRenewalForecastBucketPayloads(items []SaaSAdminRenewalForecastBucket) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"bucket":                  item.Bucket,
			"label":                   item.Label,
			"tenantCount":             item.TenantCount,
			"pricedTenantCount":       item.PricedTenantCount,
			"unknownPriceTenantCount": item.UnknownPriceTenantCount,
			"renewalAmountCents":      item.RenewalAmountCents,
			"estimatedMrrCents":       item.EstimatedMRRCents,
			"actionableTaskCount":     item.ActionableTaskCount,
		})
	}
	return payloads
}

func saasAdminRenewalForecastOwnerPayloads(items []SaaSAdminRenewalForecastOwnerSummary) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"owner":                   item.Owner,
			"tenantCount":             item.TenantCount,
			"pricedTenantCount":       item.PricedTenantCount,
			"unknownPriceTenantCount": item.UnknownPriceTenantCount,
			"renewalAmountCents":      item.RenewalAmountCents,
			"estimatedMrrCents":       item.EstimatedMRRCents,
			"expiredTenantCount":      item.ExpiredTenantCount,
			"dueWithin30TenantCount":  item.DueWithin30TenantCount,
			"due31To60TenantCount":    item.Due31To60TenantCount,
			"due61To90TenantCount":    item.Due61To90TenantCount,
			"dueLaterTenantCount":     item.DueLaterTenantCount,
			"actionableTaskCount":     item.ActionableTaskCount,
			"pendingTaskCount":        item.PendingTaskCount,
			"blockedTaskCount":        item.BlockedTaskCount,
			"failedTaskCount":         item.FailedTaskCount,
			"appliedTaskCount":        item.AppliedTaskCount,
			"canceledTaskCount":       item.CanceledTaskCount,
			"nextFollowUpAt":          item.NextFollowUpAt,
			"topTenants":              saasAdminRenewalForecastTenantPayloads(item.TopTenants),
		})
	}
	return payloads
}

func saasAdminRenewalForecastTenantPayloads(items []SaaSAdminRenewalForecastTenant) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := saasAdminTenantPayload(item.Tenant)
		payload["bucket"] = item.Bucket
		payload["bucketLabel"] = item.BucketLabel
		payload["daysUntil"] = item.DaysUntil
		payload["owner"] = item.Owner
		payload["renewalAmountCents"] = item.RenewalAmountCents
		payload["estimatedMrrCents"] = item.EstimatedMRRCents
		payload["priced"] = item.Priced
		payload["latestBillingEventId"] = item.LatestBillingEventID
		payload["latestBillingAt"] = item.LatestBillingAt
		if item.HasRiskFollowUp {
			payload["riskFollowUp"] = saasAdminRiskFollowUpSnapshotPayload(item.RiskFollowUp)
		} else {
			payload["riskFollowUp"] = nil
		}
		payload["taskSummary"] = saasAdminTaskSummaryPayload(item.TaskSummary)
		if item.HasLatestTask {
			payload["latestTask"] = saasAdminTaskPayload(item.LatestTask)
		} else {
			payload["latestTask"] = nil
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func saasAdminRenewalForecastTasksPayload(result SaaSAdminRenewalForecastTasksResult, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminRenewalForecastFiltersPayload(result.Options),
		"matchedCount":          result.MatchedCount,
		"createdCount":          result.CreatedCount,
		"pendingCount":          result.PendingCount,
		"blockedCount":          result.BlockedCount,
		"skippedExistingCount":  result.SkippedExistingCount,
		"skippedInvalidCount":   result.SkippedInvalidCount,
		"packageCode":           result.PackageCode,
		"expiresAt":             result.ExpiresAt,
		"months":                result.Months,
		"amountCents":           result.AmountCents,
		"currency":              result.Currency,
		"paidAt":                result.PaidAt,
		"paymentMethod":         result.PaymentMethod,
		"remark":                result.Remark,
		"forceCreate":           result.ForceCreate,
		"tasks":                 saasAdminTaskPayloads(result.Tasks),
		"skipped":               saasAdminCustomerSuccessRenewalTaskSkippedPayloads(result.Skipped),
	}
}

func saasAdminBusinessPackageMetricPayloads(items []SaaSAdminBusinessPackageMetric) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"packageCode":             item.PackageCode,
			"packageName":             item.PackageName,
			"packageStatus":           item.PackageStatus,
			"tenantCount":             item.TenantCount,
			"activeTenantCount":       item.ActiveTenantCount,
			"pricedTenantCount":       item.PricedTenantCount,
			"unknownPriceTenantCount": item.UnknownPriceTenantCount,
			"estimatedMrrCents":       item.EstimatedMRRCents,
			"estimatedArrCents":       item.EstimatedARRCents,
			"atRiskTenantCount":       item.AtRiskTenantCount,
			"atRiskMrrCents":          item.AtRiskMRRCents,
			"expiringSoonTenantCount": item.ExpiringSoonTenantCount,
			"expiringSoonMrrCents":    item.ExpiringSoonMRRCents,
			"expiredTenantCount":      item.ExpiredTenantCount,
			"expiredMrrCents":         item.ExpiredMRRCents,
			"latestAmountCents":       item.LatestAmountCents,
			"latestBillingEventId":    item.LatestBillingEventID,
			"latestBillingAt":         item.LatestBillingAt,
			"estimated":               item.Estimated,
		})
	}
	return payloads
}

func saasAdminCustomerSuccessReportPayload(report SaaSAdminCustomerSuccessReport, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminCustomerSuccessFiltersPayload(report.Options),
		"summary":               saasAdminCustomerSuccessSummaryPayload(report.Summary),
		"returnedCount":         len(report.Items),
		"items":                 saasAdminCustomerSuccessQueueItemPayloads(report.Items),
	}
}

func saasAdminOperationQueuePayload(report SaaSAdminOperationQueueReport, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminOperationQueueFiltersPayload(report.Options),
		"summary":               saasAdminOperationQueueSummaryPayload(report.Summary),
		"returnedCount":         len(report.Items),
		"items":                 saasAdminOperationQueueItemPayloads(report.Items),
	}
}

func saasAdminOperationQueueOwnerReportPayload(report SaaSAdminOperationQueueOwnerReport, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminOperationQueueFiltersPayload(report.Options),
		"summary":               saasAdminOperationQueueSummaryPayload(report.Summary),
		"ownerCount":            report.TotalOwnerCount,
		"returnedCount":         report.ReturnedOwnerCount,
		"scannedQueueCount":     report.ScannedQueueCount,
		"owners":                saasAdminOperationQueueOwnerSummaryPayloads(report.Owners),
	}
}

func saasAdminOperationQueueAssignmentReportPayload(report SaaSAdminOperationQueueAssignmentReport, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminOperationQueueAssignmentFiltersPayload(report.Options),
		"summary":               saasAdminOperationQueueAssignmentSummaryPayload(report.Summary),
		"assignmentCount":       report.Summary.AssignmentCount,
		"returnedCount":         report.ReturnedCount,
		"assignments":           saasAdminOperationQueueAssignmentPayloads(report.Assignments),
	}
}

func saasAdminOperationQueueAssignmentFiltersPayload(options SaaSAdminOperationQueueAssignmentOptions) map[string]any {
	return map[string]any{
		"source":      options.Source,
		"owner":       options.Owner,
		"status":      options.Status,
		"dueState":    options.DueState,
		"objectType":  options.ObjectType,
		"objectId":    options.ObjectID,
		"tenantId":    options.TenantID,
		"keyword":     options.Keyword,
		"limit":       options.Limit,
		"currentOnly": options.CurrentOnly,
	}
}

func saasAdminOperationQueueAssignmentSummaryPayload(summary SaaSAdminOperationQueueAssignmentSummary) map[string]any {
	return map[string]any{
		"assignmentCount":         summary.AssignmentCount,
		"tenantCount":             summary.TenantCount,
		"ownerCount":              summary.OwnerCount,
		"sourceCount":             summary.SourceCount,
		"taskSlaCount":            summary.TaskSLACount,
		"notificationCount":       summary.NotificationCount,
		"closedNotificationCount": summary.ClosedNotificationCount,
		"notificationHealthCount": summary.NotificationHealthCount,
		"overdueCount":            summary.OverdueCount,
		"dueSoonCount":            summary.DueSoonCount,
		"futureCount":             summary.FutureCount,
		"noDateCount":             summary.NoDateCount,
		"closedCount":             summary.ClosedCount,
		"nextFollowUpAt":          summary.NextFollowUpAt,
	}
}

func saasAdminOperationQueueAssignmentPayloads(items []SaaSAdminOperationQueueAssignment) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, saasAdminOperationQueueAssignmentPayload(item))
	}
	return payloads
}

func saasAdminOperationQueueFiltersPayload(options SaaSAdminOperationQueueOptions) map[string]any {
	return map[string]any{
		"tenantLimit":        options.TenantLimit,
		"limit":              options.Limit,
		"expiringDays":       options.ExpiringDays,
		"highUsageRatio":     options.HighUsageRatio,
		"source":             options.Source,
		"priority":           options.Priority,
		"owner":              options.Owner,
		"keyword":            options.Keyword,
		"warningHours":       options.WarningHours,
		"overdueHours":       options.OverdueHours,
		"healthWindowHours":  options.HealthWindowHours,
		"healthStaleMinutes": options.HealthStaleMinutes,
	}
}

func saasAdminOperationQueueOwnerSummaryPayloads(owners []SaaSAdminOperationQueueOwnerSummary) []map[string]any {
	payloads := make([]map[string]any, 0, len(owners))
	for _, owner := range owners {
		payloads = append(payloads, saasAdminOperationQueueOwnerSummaryPayload(owner))
	}
	return payloads
}

func saasAdminOperationQueueOwnerSummaryPayload(owner SaaSAdminOperationQueueOwnerSummary) map[string]any {
	return map[string]any{
		"owner":                   owner.Owner,
		"queueCount":              owner.QueueCount,
		"tenantCount":             owner.TenantCount,
		"sourceCount":             owner.SourceCount,
		"criticalCount":           owner.CriticalCount,
		"highCount":               owner.HighCount,
		"mediumCount":             owner.MediumCount,
		"normalCount":             owner.NormalCount,
		"customerSuccessCount":    owner.CustomerSuccessCount,
		"taskSlaCount":            owner.TaskSLACount,
		"billingFollowUpCount":    owner.BillingFollowUpCount,
		"notificationCount":       owner.NotificationCount,
		"closedNotificationCount": owner.ClosedNotificationCount,
		"notificationHealthCount": owner.NotificationHealthCount,
		"unassignedCount":         owner.UnassignedCount,
		"maxAgeHours":             owner.MaxAgeHours,
		"topTenants":              saasAdminOperationQueueOwnerTenantPayloads(owner.TopTenants),
		"topItems":                saasAdminOperationQueueItemPayloads(owner.TopItems),
	}
}

func saasAdminOperationQueueOwnerTenantPayloads(items []SaaSAdminOperationQueueOwnerTenant) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"tenantId":      item.TenantID,
			"tenantName":    item.TenantName,
			"queueCount":    item.QueueCount,
			"criticalCount": item.CriticalCount,
			"highCount":     item.HighCount,
			"sources":       item.Sources,
		})
	}
	return payloads
}

func saasAdminOperationQueueSummaryPayload(summary SaaSAdminOperationQueueSummary) map[string]any {
	return map[string]any{
		"queueCount":              summary.QueueCount,
		"returnedCount":           summary.ReturnedCount,
		"tenantCount":             summary.TenantCount,
		"sourceCount":             summary.SourceCount,
		"criticalCount":           summary.CriticalCount,
		"highCount":               summary.HighCount,
		"mediumCount":             summary.MediumCount,
		"normalCount":             summary.NormalCount,
		"customerSuccessCount":    summary.CustomerSuccessCount,
		"taskSlaCount":            summary.TaskSLACount,
		"billingFollowUpCount":    summary.BillingFollowUpCount,
		"notificationCount":       summary.NotificationCount,
		"closedNotificationCount": summary.ClosedNotificationCount,
		"notificationHealthCount": summary.NotificationHealthCount,
		"unassignedCount":         summary.UnassignedCount,
	}
}

func saasAdminOperationQueueItemPayloads(items []SaaSAdminOperationQueueItem) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, saasAdminOperationQueueItemPayload(item))
	}
	return payloads
}

func saasAdminOperationQueueItemPayload(item SaaSAdminOperationQueueItem) map[string]any {
	payload := map[string]any{
		"id":         item.ID,
		"source":     item.Source,
		"priority":   item.Priority,
		"tenantId":   item.TenantID,
		"tenantName": item.TenantName,
		"owner":      item.Owner,
		"title":      item.Title,
		"reason":     item.Reason,
		"nextAction": item.NextAction,
		"dueState":   item.DueState,
		"status":     item.Status,
		"objectType": item.ObjectType,
		"objectId":   item.ObjectID,
		"createdAt":  item.CreatedAt,
		"updatedAt":  item.UpdatedAt,
		"ageHours":   item.AgeHours,
		"score":      item.Score,
		"remark":     item.Remark,
		"reference":  item.Reference,
	}
	if item.Assignment != nil {
		payload["assignment"] = saasAdminOperationQueueAssignmentPayload(*item.Assignment)
	}
	return payload
}

func saasAdminOperationQueueAssignmentPayload(assignment SaaSAdminOperationQueueAssignment) map[string]any {
	return map[string]any{
		"tenantId":       assignment.TenantID,
		"source":         assignment.Source,
		"objectType":     assignment.ObjectType,
		"objectId":       assignment.ObjectID,
		"targetName":     assignment.TargetName,
		"owner":          assignment.Owner,
		"status":         assignment.Status,
		"dueState":       assignment.DueState,
		"nextFollowUpAt": assignment.NextFollowUpAt,
		"remark":         assignment.Remark,
		"operationId":    assignment.OperationID,
		"actorUserId":    assignment.ActorUserID,
		"actorTenantId":  assignment.ActorTenantID,
		"assignedAt":     assignment.AssignedAt,
	}
}

func saasAdminOperationQueueAssignmentNotificationsPayload(result SaaSAdminOperationQueueAssignmentNotificationsResult, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminOperationQueueAssignmentFiltersPayload(result.Options),
		"summary":               saasAdminOperationQueueAssignmentSummaryPayload(result.Summary),
		"matchedCount":          result.MatchedCount,
		"eligibleCount":         result.EligibleCount,
		"enqueuedCount":         result.EnqueuedCount,
		"skippedExistingCount":  result.SkippedExistingCount,
		"skippedInvalidCount":   result.SkippedInvalidCount,
		"skippedStatusCount":    result.SkippedStatusCount,
		"channel":               result.Channel,
		"maxAttempts":           result.MaxAttempts,
		"remark":                result.Remark,
		"forceCreate":           result.ForceCreate,
		"notifications":         saasAdminAlertNotificationPayloads(result.Notifications),
		"skipped":               saasAdminOperationQueueAssignmentNotificationSkippedPayloads(result.Skipped),
		"operationQueueAssignmentNotificationKey": SaaSAlertTypeOperationQueueAssign,
	}
}

func saasAdminOperationQueueAssignmentClosePayload(result SaaSAdminOperationQueueAssignmentCloseResult, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"closed":                result.Closed,
		"alreadyClosed":         result.AlreadyClosed,
		"previous":              saasAdminOperationQueueAssignmentPayload(result.Previous),
		"assignment":            saasAdminOperationQueueAssignmentPayload(result.Assignment),
	}
}

func saasAdminOperationQueueAssignmentNotificationSkippedPayloads(items []SaaSAdminOperationQueueAssignmentNotificationSkipped) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"operationId": item.OperationID,
			"tenantId":    item.TenantID,
			"source":      item.Source,
			"objectId":    item.ObjectID,
			"dueState":    item.DueState,
			"reason":      item.Reason,
		})
	}
	return payloads
}

func saasAdminOperationQueueAssignPayload(result SaaSAdminOperationQueueAssignResult, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":                true,
		"platformAdminTenantId":           platformAdminTenantID,
		"generatedAt":                     time.Now().Format("2006-01-02 15:04:05"),
		"filters":                         saasAdminOperationQueueFiltersPayload(result.Options),
		"matchedCount":                    result.MatchedCount,
		"assignableCount":                 result.AssignableCount,
		"assignedCount":                   result.AssignedCount,
		"skippedCount":                    result.SkippedCount,
		"unsupportedCount":                result.UnsupportedCount,
		"customerSuccessAssignedCount":    result.CustomerSuccessAssignedCount,
		"billingFollowUpAssignedCount":    result.BillingFollowUpAssignedCount,
		"queueAssignmentAssignedCount":    result.QueueAssignmentAssignedCount,
		"taskSlaAssignedCount":            result.TaskSLAAssignedCount,
		"notificationAssignedCount":       result.NotificationAssignedCount,
		"closedNotificationAssignedCount": result.ClosedNotificationAssignedCount,
		"notificationHealthAssignedCount": result.NotificationHealthAssignedCount,
		"status":                          result.Status,
		"owner":                           result.Owner,
		"nextFollowUpAt":                  result.NextFollowUpAt,
		"remark":                          result.Remark,
		"items":                           saasAdminOperationQueueAssignItemPayloads(result.Items),
	}
}

func saasAdminOperationQueueAssignItemPayloads(items []SaaSAdminOperationQueueAssignItem) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, saasAdminOperationQueueAssignItemPayload(item))
	}
	return payloads
}

func saasAdminOperationQueueAssignItemPayload(item SaaSAdminOperationQueueAssignItem) map[string]any {
	payload := map[string]any{
		"queueItem":     saasAdminOperationQueueItemPayload(item.QueueItem),
		"skippedReason": item.SkippedReason,
	}
	if item.RiskFollowUp != nil {
		payload["riskFollowUp"] = saasAdminRiskFollowUpPayload(*item.RiskFollowUp)
	}
	if item.BillingFollowUp != nil {
		payload["billingFollowUp"] = saasAdminBillingReconciliationFollowUpPayload(*item.BillingFollowUp)
	}
	if item.OperationQueueAssignment != nil {
		payload["operationQueueAssignment"] = saasAdminOperationQueueAssignmentPayload(*item.OperationQueueAssignment)
	}
	return payload
}

func saasAdminCustomerSuccessOwnerReportPayload(report SaaSAdminCustomerSuccessOwnerReport, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminCustomerSuccessFiltersPayload(report.Options),
		"summary":               saasAdminCustomerSuccessSummaryPayload(report.Summary),
		"ownerCount":            report.TotalOwnerCount,
		"returnedCount":         len(report.Owners),
		"owners":                saasAdminCustomerSuccessOwnerSummaryPayloads(report.Owners),
	}
}

func saasAdminCustomerSuccessFiltersPayload(options SaaSAdminCustomerSuccessOptions) map[string]any {
	return map[string]any{
		"tenantLimit":    options.TenantLimit,
		"limit":          options.Limit,
		"expiringDays":   options.ExpiringDays,
		"highUsageRatio": options.HighUsageRatio,
		"owner":          options.Owner,
		"priority":       options.Priority,
	}
}

func saasAdminCustomerSuccessSummaryPayload(summary SaaSAdminCustomerSuccessSummary) map[string]any {
	return map[string]any{
		"tenantCount":                summary.TenantCount,
		"queueCount":                 summary.QueueCount,
		"returnedCount":              summary.ReturnedCount,
		"criticalCount":              summary.CriticalCount,
		"highCount":                  summary.HighCount,
		"mediumCount":                summary.MediumCount,
		"normalCount":                summary.NormalCount,
		"overdueCount":               summary.OverdueCount,
		"dueSoonCount":               summary.DueSoonCount,
		"unassignedCount":            summary.UnassignedCount,
		"riskTenantCount":            summary.RiskTenantCount,
		"billingFollowUpCount":       summary.BillingFollowUpCount,
		"actionableTaskCount":        summary.ActionableTaskCount,
		"retryableNotificationCount": summary.RetryableNotificationCount,
	}
}

func saasAdminCustomerSuccessQueueItemPayloads(items []SaaSAdminCustomerSuccessQueueItem) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, saasAdminCustomerSuccessQueueItemPayload(item))
	}
	return payloads
}

func saasAdminCustomerSuccessOwnerSummaryPayloads(owners []SaaSAdminCustomerSuccessOwnerSummary) []map[string]any {
	payloads := make([]map[string]any, 0, len(owners))
	for _, owner := range owners {
		payloads = append(payloads, map[string]any{
			"owner":                      owner.Owner,
			"tenantCount":                owner.TenantCount,
			"criticalCount":              owner.CriticalCount,
			"highCount":                  owner.HighCount,
			"mediumCount":                owner.MediumCount,
			"normalCount":                owner.NormalCount,
			"overdueCount":               owner.OverdueCount,
			"dueSoonCount":               owner.DueSoonCount,
			"blockedCount":               owner.BlockedCount,
			"billingFollowUpCount":       owner.BillingFollowUpCount,
			"actionableTaskCount":        owner.ActionableTaskCount,
			"retryableNotificationCount": owner.RetryableNotificationCount,
			"failedNotificationCount":    owner.FailedNotificationCount,
			"deadNotificationCount":      owner.DeadNotificationCount,
			"maxHealthScore":             owner.MaxHealthScore,
			"averageHealthScore":         owner.AverageHealthScore,
			"nextFollowUpAt":             owner.NextFollowUpAt,
			"topTenants":                 saasAdminCustomerSuccessQueueItemPayloads(owner.TopTenants),
		})
	}
	return payloads
}

func saasAdminCustomerSuccessQueueItemPayload(item SaaSAdminCustomerSuccessQueueItem) map[string]any {
	var riskFollowUp any
	if item.RiskFollowUp.OperationID > 0 {
		payloads := saasAdminRiskFollowUpTaskPayloads([]SaaSAdminRiskFollowUpTask{item.RiskFollowUp})
		if len(payloads) > 0 {
			riskFollowUp = payloads[0]
		}
	}
	billingFollowUps := item.BillingFollowUps
	if len(billingFollowUps) > 3 {
		billingFollowUps = billingFollowUps[:3]
	}
	return map[string]any{
		"tenantId":                   item.Tenant.TenantID,
		"tenantName":                 item.Tenant.TenantName,
		"priority":                   item.Priority,
		"healthScore":                item.HealthScore,
		"owner":                      item.Owner,
		"dueState":                   item.DueState,
		"reasons":                    item.Reasons,
		"nextAction":                 item.NextAction,
		"riskLevel":                  item.Risk.RiskLevel,
		"riskScore":                  item.Risk.RiskScore,
		"suggestedAction":            item.Risk.SuggestedAction,
		"openAlertCount":             item.Tenant.OpenAlertCount,
		"billingFollowUpCount":       item.BillingFollowUpCount,
		"retryableNotificationCount": item.RetryableNotificationCount,
		"failedNotificationCount":    item.FailedNotificationCount,
		"deadNotificationCount":      item.DeadNotificationCount,
		"adminTaskSummary":           saasAdminTaskSummaryPayload(item.AdminTaskSummary),
		"tenant":                     saasAdminTenantPayload(item.Tenant),
		"risk":                       saasAdminRiskTenantPayload(item.Risk),
		"riskFollowUp":               riskFollowUp,
		"billingFollowUps":           saasAdminBillingReconciliationFollowUpTaskPayloads(billingFollowUps),
	}
}

func saasAdminCustomerSuccessAssignPayload(result SaaSAdminCustomerSuccessAssignResult, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminCustomerSuccessFiltersPayload(result.Options),
		"matchedCount":          result.MatchedCount,
		"assignedCount":         result.AssignedCount,
		"status":                result.Status,
		"owner":                 result.Owner,
		"nextFollowUpAt":        result.NextFollowUpAt,
		"remark":                result.Remark,
		"followUps":             saasAdminRiskFollowUpPayloads(result.FollowUps),
	}
}

func saasAdminRenewalForecastAssignPayload(result SaaSAdminRenewalForecastAssignResult, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminRenewalForecastFiltersPayload(result.Options),
		"matchedCount":          result.MatchedCount,
		"assignedCount":         result.AssignedCount,
		"status":                result.Status,
		"owner":                 result.Owner,
		"nextFollowUpAt":        result.NextFollowUpAt,
		"remark":                result.Remark,
		"followUps":             saasAdminRiskFollowUpPayloads(result.FollowUps),
	}
}

func saasAdminCustomerSuccessRenewalTasksPayload(result SaaSAdminCustomerSuccessRenewalTasksResult, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters":               saasAdminCustomerSuccessFiltersPayload(result.Options),
		"matchedCount":          result.MatchedCount,
		"createdCount":          result.CreatedCount,
		"pendingCount":          result.PendingCount,
		"blockedCount":          result.BlockedCount,
		"skippedExistingCount":  result.SkippedExistingCount,
		"skippedInvalidCount":   result.SkippedInvalidCount,
		"packageCode":           result.PackageCode,
		"expiresAt":             result.ExpiresAt,
		"months":                result.Months,
		"amountCents":           result.AmountCents,
		"currency":              result.Currency,
		"paidAt":                result.PaidAt,
		"paymentMethod":         result.PaymentMethod,
		"remark":                result.Remark,
		"forceCreate":           result.ForceCreate,
		"tasks":                 saasAdminTaskPayloads(result.Tasks),
		"skipped":               saasAdminCustomerSuccessRenewalTaskSkippedPayloads(result.Skipped),
	}
}

func saasAdminCustomerSuccessRenewalNotificationsPayload(result SaaSAdminCustomerSuccessRenewalNotificationsResult, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":       true,
		"platformAdminTenantId":  platformAdminTenantID,
		"generatedAt":            time.Now().Format("2006-01-02 15:04:05"),
		"filters":                saasAdminCustomerSuccessFiltersPayload(result.Options),
		"matchedCount":           result.MatchedCount,
		"enqueuedCount":          result.EnqueuedCount,
		"skippedExistingCount":   result.SkippedExistingCount,
		"skippedInvalidCount":    result.SkippedInvalidCount,
		"channel":                result.Channel,
		"maxAttempts":            result.MaxAttempts,
		"reminderDays":           result.ReminderDays,
		"remark":                 result.Remark,
		"forceCreate":            result.ForceCreate,
		"notifications":          saasAdminAlertNotificationPayloads(result.Notifications),
		"skipped":                saasAdminCustomerSuccessRenewalTaskSkippedPayloads(result.Skipped),
		"renewalNotificationKey": SaaSAlertTypeTenantRenewal,
	}
}

func saasAdminRenewalForecastNotificationsPayload(result SaaSAdminRenewalForecastNotificationsResult, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"canPlatformScope":       true,
		"platformAdminTenantId":  platformAdminTenantID,
		"generatedAt":            time.Now().Format("2006-01-02 15:04:05"),
		"filters":                saasAdminRenewalForecastFiltersPayload(result.Options),
		"matchedCount":           result.MatchedCount,
		"enqueuedCount":          result.EnqueuedCount,
		"skippedExistingCount":   result.SkippedExistingCount,
		"skippedInvalidCount":    result.SkippedInvalidCount,
		"channel":                result.Channel,
		"maxAttempts":            result.MaxAttempts,
		"reminderDays":           result.ReminderDays,
		"remark":                 result.Remark,
		"forceCreate":            result.ForceCreate,
		"notifications":          saasAdminAlertNotificationPayloads(result.Notifications),
		"skipped":                saasAdminCustomerSuccessRenewalTaskSkippedPayloads(result.Skipped),
		"renewalNotificationKey": SaaSAlertTypeTenantRenewal,
	}
}

func saasAdminCustomerSuccessRenewalTaskSkippedPayloads(items []SaaSAdminCustomerSuccessRenewalTaskSkipped) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"tenantId":   item.TenantID,
			"tenantName": item.TenantName,
			"reason":     item.Reason,
		})
	}
	return payloads
}

func saasAdminDailyReportWindowPayload(options SaaSAdminDailyReportOptions) map[string]any {
	return map[string]any{
		"date":      options.ReportDate,
		"days":      options.Days,
		"startAt":   options.WindowStart.Format("2006-01-02 15:04:05"),
		"endAt":     options.WindowEnd.Format("2006-01-02 15:04:05"),
		"startDate": options.WindowStart.Format("2006-01-02"),
		"endDate":   options.WindowEnd.Add(-time.Second).Format("2006-01-02"),
	}
}

func saasAdminDailyReportFiltersPayload(options SaaSAdminDailyReportOptions) map[string]any {
	return map[string]any{
		"date":           options.ReportDate,
		"days":           options.Days,
		"expiringDays":   options.ExpiringDays,
		"highUsageRatio": options.HighUsageRatio,
		"tenantLimit":    options.TenantLimit,
		"limit":          options.ItemLimit,
	}
}

func saasAdminDailyReportSummaryPayload(summary SaaSAdminDailyReportSummary) map[string]any {
	return map[string]any{
		"tenantCount":                         summary.TenantCount,
		"activeTenantPackageCount":            summary.ActiveTenantPackageCount,
		"userCount":                           summary.UserCount,
		"corpCount":                           summary.CorpCount,
		"expiringSoonTenantCount":             summary.ExpiringSoonTenantCount,
		"expiredTenantCount":                  summary.ExpiredTenantCount,
		"evaluatedRiskTenantCount":            summary.EvaluatedRiskTenantCount,
		"riskTenantCount":                     summary.RiskTenantCount,
		"criticalRiskTenantCount":             summary.CriticalRiskTenantCount,
		"highRiskTenantCount":                 summary.HighRiskTenantCount,
		"openRiskFollowUpCount":               summary.OpenRiskFollowUpCount,
		"overdueRiskFollowUpCount":            summary.OverdueRiskFollowUpCount,
		"dueSoonRiskFollowUpCount":            summary.DueSoonRiskFollowUpCount,
		"riskFollowUpOwnerCount":              summary.RiskFollowUpOwnerCount,
		"openAlertCount":                      summary.OpenAlertCount,
		"taskSlaActiveCount":                  summary.TaskSLAActiveCount,
		"taskSlaWarningCount":                 summary.TaskSLAWarningCount,
		"taskSlaOverdueCount":                 summary.TaskSLAOverdueCount,
		"taskSlaOwnerCount":                   summary.TaskSLAOwnerCount,
		"taskSlaMaxAgeHours":                  summary.TaskSLAMaxAgeHours,
		"pendingNotificationCount":            summary.PendingNotificationCount,
		"failedNotificationCount":             summary.FailedNotificationCount,
		"deadNotificationCount":               summary.DeadNotificationCount,
		"closedNotificationCount":             summary.ClosedNotificationCount,
		"retryableNotificationCount":          summary.RetryableNotificationCount,
		"windowQueueAssignmentCount":          summary.WindowQueueAssignmentCount,
		"windowTaskSlaAssignCount":            summary.WindowTaskSLAAssignCount,
		"windowNotificationAssignCount":       summary.WindowNotificationAssignCount,
		"windowClosedNotificationAssignCount": summary.WindowClosedNotificationAssignCount,
		"windowNotificationHealthAssignCount": summary.WindowNotificationHealthAssignCount,
		"windowOperationCount":                summary.WindowOperationCount,
		"windowBillingEventCount":             summary.WindowBillingEventCount,
		"windowBillingAmountCents":            summary.WindowBillingAmountCents,
		"windowRenewalCount":                  summary.WindowRenewalCount,
		"windowRefundCount":                   summary.WindowRefundCount,
		"windowGrossBillingAmountCents":       summary.WindowGrossBillingAmountCents,
		"windowRefundAmountCents":             summary.WindowRefundAmountCents,
		"windowRiskFollowUpCount":             summary.WindowRiskFollowUpCount,
		"windowAlertResolveCount":             summary.WindowAlertResolveCount,
		"windowNotificationRetryCount":        summary.WindowNotificationRetryCount,
		"windowNotificationCloseCount":        summary.WindowNotificationCloseCount,
	}
}

func saasAdminDailyNotificationSummaryPayload(summary SaaSAdminDailyNotificationSummary) map[string]any {
	return map[string]any{
		"pendingCount":    summary.PendingCount,
		"failedCount":     summary.FailedCount,
		"deadCount":       summary.DeadCount,
		"closedCount":     summary.ClosedCount,
		"suppressedCount": summary.SuppressedCount,
		"deliveredCount":  summary.DeliveredCount,
		"retryableCount":  summary.RetryableCount,
	}
}

func saasAdminDailyBillingSummaryPayload(summary SaaSAdminDailyBillingSummary) map[string]any {
	return map[string]any{
		"eventCount":        summary.EventCount,
		"renewalCount":      summary.RenewalCount,
		"refundCount":       summary.RefundCount,
		"grossAmountCents":  summary.GrossAmountCents,
		"refundAmountCents": summary.RefundAmountCents,
		"amountCents":       summary.AmountCents,
	}
}

func saasAdminDailyOperationActionSummaryPayloads(summaries []SaaSAdminDailyOperationActionSummary) []map[string]any {
	items := make([]map[string]any, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, map[string]any{
			"action": summary.Action,
			"count":  summary.Count,
		})
	}
	return items
}

func saasAdminRiskSummaryPayload(summary SaaSAdminRiskSummary) map[string]any {
	return map[string]any{
		"totalTenantCount":         summary.TotalTenantCount,
		"evaluatedTenantCount":     summary.EvaluatedTenantCount,
		"riskTenantCount":          summary.RiskTenantCount,
		"criticalRiskTenantCount":  summary.CriticalRiskTenantCount,
		"highRiskTenantCount":      summary.HighRiskTenantCount,
		"mediumRiskTenantCount":    summary.MediumRiskTenantCount,
		"disabledTenantCount":      summary.DisabledTenantCount,
		"expiredTenantCount":       summary.ExpiredTenantCount,
		"expiringSoonTenantCount":  summary.ExpiringSoonTenantCount,
		"noPackageTenantCount":     summary.NoPackageTenantCount,
		"openAlertTenantCount":     summary.OpenAlertTenantCount,
		"highUsageTenantCount":     summary.HighUsageTenantCount,
		"exceededUsageTenantCount": summary.ExceededUsageTenantCount,
		"warningUsageTenantCount":  summary.WarningUsageTenantCount,
		"followUpTenantCount":      summary.FollowUpTenantCount,
		"pendingFollowUpCount":     summary.PendingFollowUpCount,
		"overdueFollowUpCount":     summary.OverdueFollowUpCount,
		"renewalPendingCount":      summary.RenewalPendingCount,
	}
}

func saasAdminRiskTenantPayloads(items []SaaSAdminRiskTenant) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, saasAdminRiskTenantPayload(item))
	}
	return payloads
}

func saasAdminRiskTenantPayload(item SaaSAdminRiskTenant) map[string]any {
	tenant := saasAdminTenantPayload(item.Tenant)
	tenant["riskLevel"] = item.RiskLevel
	tenant["riskScore"] = item.RiskScore
	tenant["riskReasons"] = item.Reasons
	tenant["suggestedAction"] = item.SuggestedAction
	tenant["highestUsageRatio"] = item.HighUsageRatio
	tenant["topUsageMetrics"] = saasAdminUsageMetricPayloads(item.TopUsageMetrics)
	if item.FollowUp.OperationID > 0 {
		tenant["followUp"] = saasAdminRiskFollowUpSnapshotPayload(item.FollowUp)
	} else {
		tenant["followUp"] = nil
	}
	return tenant
}

func saasAdminRiskFollowUpSnapshotPayload(item SaaSAdminRiskFollowUpSnapshot) map[string]any {
	return map[string]any{
		"tenantId":       item.TenantID,
		"tenantName":     item.TenantName,
		"status":         item.Status,
		"owner":          item.Owner,
		"nextFollowUpAt": item.NextFollowUpAt,
		"remark":         item.Remark,
		"operationId":    item.OperationID,
		"createdAt":      item.CreatedAt,
	}
}

func saasAdminRiskFollowUpTaskPayloads(tasks []SaaSAdminRiskFollowUpTask) []map[string]any {
	items := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		item := saasAdminRiskFollowUpSnapshotPayload(task.SaaSAdminRiskFollowUpSnapshot)
		item["dueState"] = task.DueState
		item["overdue"] = task.Overdue
		item["daysUntil"] = task.DaysUntil
		items = append(items, item)
	}
	return items
}

func saasAdminRiskFollowUpTaskSummaryPayload(summary SaaSAdminRiskFollowUpTaskSummary) map[string]any {
	return map[string]any{
		"totalCount":          summary.TotalCount,
		"pendingCount":        summary.PendingCount,
		"contactedCount":      summary.ContactedCount,
		"renewalPendingCount": summary.RenewalPendingCount,
		"resolvedCount":       summary.ResolvedCount,
		"ignoredCount":        summary.IgnoredCount,
		"overdueCount":        summary.OverdueCount,
		"dueSoonCount":        summary.DueSoonCount,
		"noDateCount":         summary.NoDateCount,
		"closedCount":         summary.ClosedCount,
	}
}

func saasAdminRiskFollowUpOwnerSummaryPayloads(owners []SaaSAdminRiskFollowUpOwnerSummary) []map[string]any {
	items := make([]map[string]any, 0, len(owners))
	for _, owner := range owners {
		items = append(items, map[string]any{
			"owner":               owner.Owner,
			"totalCount":          owner.TotalCount,
			"openCount":           owner.OpenCount,
			"pendingCount":        owner.PendingCount,
			"contactedCount":      owner.ContactedCount,
			"renewalPendingCount": owner.RenewalPendingCount,
			"resolvedCount":       owner.ResolvedCount,
			"ignoredCount":        owner.IgnoredCount,
			"overdueCount":        owner.OverdueCount,
			"dueSoonCount":        owner.DueSoonCount,
			"futureCount":         owner.FutureCount,
			"noDateCount":         owner.NoDateCount,
			"closedCount":         owner.ClosedCount,
			"latestFollowUpAt":    owner.LatestFollowUpAt,
			"nextFollowUpAt":      owner.NextFollowUpAt,
		})
	}
	return items
}

func saasAdminUsageSummaryPayload(summary SaaSAdminUsageSummary) map[string]any {
	return map[string]any{
		"metricCount":          summary.MetricCount,
		"limitedMetricCount":   summary.LimitedMetricCount,
		"unlimitedMetricCount": summary.UnlimitedMetricCount,
		"openAlertMetricCount": summary.OpenAlertMetricCount,
		"exceededMetricCount":  summary.ExceededMetricCount,
		"warningMetricCount":   summary.WarningMetricCount,
		"highestUsageMetric":   summary.HighestUsageMetric,
		"highestUsageLabel":    saasMetricLabel(summary.HighestUsageMetric),
		"highestUsageRatio":    summary.HighestUsageRatio,
	}
}

func saasAdminTenantPackageUpdatePayload(result SaaSAdminTenantPackageUpdateResult) map[string]any {
	return map[string]any{
		"tenantId":              result.TenantID,
		"tenantName":            result.TenantName,
		"packageCode":           result.PackageCode,
		"packageName":           result.PackageName,
		"expiresAt":             result.ExpiresAt,
		"status":                result.Status,
		"version":               result.Version,
		"operationId":           result.OperationID,
		"metricsRefreshed":      result.MetricsRefreshed,
		"metricsRefreshPending": result.MetricsRefreshPending,
		"metricsRefreshError":   result.MetricsRefreshError,
	}
}

func saasAdminPackageTenantSnapshotSyncPayload(result SaaSAdminPackageTenantSnapshotSyncResult) map[string]any {
	return map[string]any{
		"packageCode":            result.PackageCode,
		"packageName":            result.PackageName,
		"packageVersion":         result.PackageVersion,
		"limit":                  result.Limit,
		"dryRun":                 result.DryRun,
		"allowOverLimit":         result.AllowOverLimit,
		"tenantSnapshotsUpdated": result.TenantSnapshotsUpdated,
		"matchedTenantCount":     result.MatchedTenantCount,
		"checkedTenantCount":     result.CheckedTenantCount,
		"syncedTenantCount":      result.SyncedTenantCount,
		"overLimitTenantCount":   result.OverLimitTenantCount,
		"metricsRefreshed":       result.MetricsRefreshed,
		"blocked":                result.Blocked,
		"blockReason":            result.BlockReason,
		"tenants":                saasAdminPackageTenantSnapshotSyncTenantPayloads(result.Tenants),
		"overLimitTenants":       saasAdminPackageImpactTenantPayloads(result.OverLimitTenants),
	}
}

func saasAdminPackageTenantSnapshotSyncTenantPayloads(tenants []SaaSAdminPackageTenantSnapshotSyncTenant) []map[string]any {
	items := make([]map[string]any, 0, len(tenants))
	for _, item := range tenants {
		items = append(items, map[string]any{
			"tenantId":         item.TenantID,
			"tenantName":       item.TenantName,
			"tenantStatus":     item.TenantStatus,
			"version":          item.Version,
			"expiresAt":        item.ExpiresAt,
			"synced":           item.Synced,
			"skipped":          item.Skipped,
			"metricsRefreshed": item.MetricsRefreshed,
			"overLimitMetrics": saasAdminPackageImpactTenantPayloads(item.OverLimitMetrics),
		})
	}
	return items
}

func saasAdminTenantStatusUpdatePayload(result SaaSAdminTenantStatusUpdateResult) map[string]any {
	return map[string]any{
		"tenantId":       result.TenantID,
		"tenantName":     result.TenantName,
		"previousStatus": result.PreviousStatus,
		"status":         result.Status,
		"remark":         result.Remark,
		"operationId":    result.OperationID,
	}
}

func saasAdminRiskFollowUpPayload(result SaaSAdminRiskFollowUpResult) map[string]any {
	return map[string]any{
		"tenantId":       result.TenantID,
		"tenantName":     result.TenantName,
		"status":         result.Status,
		"owner":          result.Owner,
		"nextFollowUpAt": result.NextFollowUpAt,
		"remark":         result.Remark,
		"operationId":    result.OperationID,
	}
}

func saasAdminRiskFollowUpPayloads(results []SaaSAdminRiskFollowUpResult) []map[string]any {
	payloads := make([]map[string]any, 0, len(results))
	for _, result := range results {
		payloads = append(payloads, saasAdminRiskFollowUpPayload(result))
	}
	return payloads
}

func saasAdminRiskFollowUpBulkClosePayload(result SaaSAdminRiskFollowUpBulkCloseResult) map[string]any {
	items := make([]map[string]any, 0, len(result.FollowUps))
	for _, followUp := range result.FollowUps {
		items = append(items, saasAdminRiskFollowUpPayload(followUp))
	}
	return map[string]any{
		"closedCount": result.ClosedCount,
		"status":      result.Status,
		"remark":      result.Remark,
		"followUps":   items,
	}
}

func saasAdminTenantRenewalPayload(result SaaSAdminTenantRenewalResult) map[string]any {
	return map[string]any{
		"tenantId":              result.TenantID,
		"tenantName":            result.TenantName,
		"packageCode":           result.PackageCode,
		"packageName":           result.PackageName,
		"previousExpiresAt":     result.PreviousExpiresAt,
		"expiresAt":             result.ExpiresAt,
		"amountCents":           result.AmountCents,
		"currency":              result.Currency,
		"billingEventId":        result.BillingEventID,
		"operationId":           result.OperationID,
		"metricsRefreshed":      result.MetricsRefreshed,
		"metricsRefreshPending": result.MetricsRefreshPending,
		"metricsRefreshError":   result.MetricsRefreshError,
	}
}

func saasAdminTenantRenewalPreviewPayload(preview SaaSAdminTenantRenewalPreview) map[string]any {
	return map[string]any{
		"tenantId":                      preview.TenantID,
		"tenantName":                    preview.TenantName,
		"tenantStatus":                  preview.TenantStatus,
		"previousPackageCode":           preview.PreviousPackageCode,
		"previousPackageName":           preview.PreviousPackageName,
		"previousPackageStatus":         preview.PreviousPackageStatus,
		"previousPackageVersion":        preview.PreviousPackageVersion,
		"previousExpiresAt":             preview.PreviousExpiresAt,
		"packageCode":                   preview.PackageCode,
		"packageName":                   preview.PackageName,
		"packageVersion":                preview.PackageVersion,
		"expiresAt":                     preview.ExpiresAt,
		"amountCents":                   preview.AmountCents,
		"currency":                      preview.Currency,
		"paidAt":                        preview.PaidAt,
		"paymentMethod":                 preview.PaymentMethod,
		"externalOrderNo":               preview.ExternalOrderNo,
		"remark":                        preview.Remark,
		"subscriptionExists":            preview.SubscriptionExists,
		"subscriptionStatus":            preview.SubscriptionStatus,
		"subscriptionVersion":           preview.SubscriptionVersion,
		"subscriptionCurrentPeriodEnds": preview.SubscriptionCurrentPeriodEnds,
		"blocked":                       preview.Blocked,
		"blockReason":                   preview.BlockReason,
	}
}

func saasAdminTenantProvisionPayload(result SaaSAdminTenantProvisionResult) map[string]any {
	return map[string]any{
		"tenantId":              result.TenantID,
		"tenantName":            result.TenantName,
		"adminUserId":           result.AdminUserID,
		"adminPhone":            result.AdminPhone,
		"adminName":             result.AdminName,
		"roleId":                result.RoleID,
		"roleName":              result.RoleName,
		"packageCode":           result.PackageCode,
		"packageName":           result.PackageName,
		"expiresAt":             result.ExpiresAt,
		"menuCount":             result.MenuCount,
		"configCopyCount":       result.ConfigCopyCount,
		"metricsRefreshed":      result.MetricsRefreshed,
		"metricsRefreshPending": result.MetricsRefreshPending,
		"metricsRefreshError":   result.MetricsRefreshError,
		"operationId":           result.OperationID,
	}
}

func saasAdminTenantProvisionPreviewPayload(preview SaaSAdminTenantProvisionPreview) map[string]any {
	return map[string]any{
		"tenantId":       preview.TenantID,
		"tenantName":     preview.TenantName,
		"adminPhone":     preview.AdminPhone,
		"adminName":      preview.AdminName,
		"roleName":       preview.RoleName,
		"packageCode":    preview.PackageCode,
		"packageName":    preview.PackageName,
		"expiresAt":      preview.ExpiresAt,
		"configCopyMode": preview.ConfigCopyMode,
		"remark":         preview.Remark,
		"blocked":        preview.Blocked,
		"blockReason":    preview.BlockReason,
	}
}

func writeSaaSAdminError(w http.ResponseWriter, err error) {
	var opErr *SaaSAdminOperationError
	if errors.As(err, &opErr) && opErr != nil {
		status := opErr.Status
		if status <= 0 {
			status = http.StatusBadRequest
		}
		writeEnvelope(w, status, status, opErr.Message, nil)
		return
	}
	writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
}

func saasAdminMetricPayloads(metrics []SaaSAdminMetricOverview) []map[string]any {
	items := make([]map[string]any, 0, len(metrics))
	for _, metric := range metrics {
		items = append(items, map[string]any{
			"metric":         metric.Metric,
			"label":          saasMetricLabel(metric.Metric),
			"current":        metric.Current,
			"limit":          metric.Limit,
			"usageRatio":     metric.UsageRatio,
			"openAlertCount": metric.OpenAlertCount,
		})
	}
	return items
}

func saasAdminUsageMetricPayloads(metrics []SaaSAdminUsageMetric) []map[string]any {
	items := make([]map[string]any, 0, len(metrics))
	for _, metric := range metrics {
		items = append(items, map[string]any{
			"metric":         metric.Metric,
			"label":          saasMetricLabel(metric.Metric),
			"periodKey":      metric.PeriodKey,
			"current":        metric.Current,
			"limit":          metric.Limit,
			"remaining":      metric.Remaining,
			"unlimited":      metric.Unlimited,
			"usageRatio":     metric.UsageRatio,
			"status":         metric.Status,
			"openAlertCount": metric.OpenAlertCount,
			"updatedBy":      metric.UpdatedBy,
			"updatedAt":      metric.UpdatedAt,
		})
	}
	return items
}

func saasAdminAlertPayloads(alerts []SaaSAlertRecord) []map[string]any {
	items := make([]map[string]any, 0, len(alerts))
	for _, alert := range alerts {
		item := saasAlertPayload(alert)
		item["metricLabel"] = saasMetricLabel(alert.Metric)
		items = append(items, item)
	}
	return items
}

func saasAdminAlertResolvePayload(result SaaSAdminAlertResolveResult) map[string]any {
	return map[string]any{
		"resolved":       result.Resolved,
		"tenantId":       result.TenantID,
		"tenantName":     result.TenantName,
		"alertId":        result.AlertID,
		"alertKey":       result.AlertKey,
		"metric":         result.Metric,
		"metricLabel":    saasMetricLabel(result.Metric),
		"alertType":      result.AlertType,
		"periodKey":      result.PeriodKey,
		"previousStatus": result.PreviousStatus,
		"status":         result.Status,
		"remark":         result.Remark,
		"operationId":    result.OperationID,
	}
}

func saasAdminAlertBulkResolvePayload(result SaaSAdminAlertBulkResolveResult) map[string]any {
	items := make([]map[string]any, 0, len(result.Alerts))
	for _, item := range result.Alerts {
		items = append(items, saasAdminAlertResolvePayload(item))
	}
	return map[string]any{
		"resolved":      result.ResolvedCount > 0,
		"resolvedCount": result.ResolvedCount,
		"tenantId":      result.TenantID,
		"filters": map[string]any{
			"tenantId":  result.TenantID,
			"metric":    result.Metric,
			"alertType": result.AlertType,
			"periodKey": result.PeriodKey,
			"limit":     result.Limit,
		},
		"remark": result.Remark,
		"alerts": items,
	}
}

func saasAdminAlertNotificationPayloads(notifications []SaaSAlertNotification) []map[string]any {
	items := make([]map[string]any, 0, len(notifications))
	for _, item := range notifications {
		items = append(items, saasAdminAlertNotificationPayload(item))
	}
	return items
}

func saasAdminAlertNotificationPayload(item SaaSAlertNotification) map[string]any {
	metric := item.Alert.Status.Metric
	alertType := item.Alert.AlertType
	periodKey := item.Alert.PeriodKey
	current := item.Alert.Status.Current
	limit := item.Alert.Status.Limit
	if alertType == "" {
		alertType = SaaSAlertTypeQuotaExceeded
	}
	if periodKey == "" {
		periodKey = SaaSAlertPeriodLifetime
	}
	return map[string]any{
		"id":              item.ID,
		"notificationKey": item.NotificationKey,
		"alertKey":        item.AlertKey,
		"tenantId":        item.TenantID,
		"channel":         item.Channel,
		"status":          item.Status,
		"attempts":        item.Attempts,
		"maxAttempts":     item.MaxAttempts,
		"metric":          metric,
		"metricLabel":     saasMetricLabel(metric),
		"alertType":       alertType,
		"periodKey":       periodKey,
		"currentValue":    current,
		"limitValue":      limit,
		"message":         item.Alert.Message,
		"lastError":       item.LastError,
		"nextRetryAt":     item.NextRetryAt,
		"deliveredAt":     item.DeliveredAt,
		"createdAt":       item.CreatedAt,
		"updatedAt":       item.UpdatedAt,
	}
}

func saasAdminNotificationRetryPayload(result SaaSAdminAlertNotificationRetryResult) map[string]any {
	return map[string]any{
		"retried":         result.Retried,
		"notificationId":  result.NotificationID,
		"notificationKey": result.NotificationKey,
		"tenantId":        result.TenantID,
		"alertKey":        result.AlertKey,
		"channel":         result.Channel,
		"previousStatus":  result.PreviousStatus,
		"status":          result.Status,
		"attempts":        result.Attempts,
		"maxAttempts":     result.MaxAttempts,
		"remark":          result.Remark,
		"operationId":     result.OperationID,
	}
}

func saasAdminNotificationBulkRetryPayload(result SaaSAdminAlertNotificationBulkRetryResult) map[string]any {
	items := make([]map[string]any, 0, len(result.Notifications))
	for _, item := range result.Notifications {
		items = append(items, saasAdminNotificationRetryPayload(item))
	}
	return map[string]any{
		"retried":      result.RetriedCount > 0,
		"retriedCount": result.RetriedCount,
		"tenantId":     result.TenantID,
		"filters": saasAdminNotificationFiltersPayload(SaaSAdminAlertNotificationOptions{
			TenantID: result.TenantID,
			Status:   result.Status,
			Channel:  result.Channel,
			Keyword:  result.Keyword,
			Limit:    result.Limit,
		}),
		"remark":        result.Remark,
		"notifications": items,
	}
}

func saasAdminNotificationClosePayload(result SaaSAdminAlertNotificationCloseResult) map[string]any {
	return map[string]any{
		"closed":          result.Closed,
		"notificationId":  result.NotificationID,
		"notificationKey": result.NotificationKey,
		"tenantId":        result.TenantID,
		"alertKey":        result.AlertKey,
		"channel":         result.Channel,
		"previousStatus":  result.PreviousStatus,
		"status":          result.Status,
		"attempts":        result.Attempts,
		"maxAttempts":     result.MaxAttempts,
		"remark":          result.Remark,
		"operationId":     result.OperationID,
	}
}

func saasAdminNotificationBulkClosePayload(result SaaSAdminAlertNotificationBulkCloseResult) map[string]any {
	items := make([]map[string]any, 0, len(result.Notifications))
	for _, item := range result.Notifications {
		items = append(items, saasAdminNotificationClosePayload(item))
	}
	return map[string]any{
		"closed":      result.ClosedCount > 0,
		"closedCount": result.ClosedCount,
		"tenantId":    result.TenantID,
		"filters": saasAdminNotificationFiltersPayload(SaaSAdminAlertNotificationOptions{
			TenantID: result.TenantID,
			Status:   result.Status,
			Channel:  result.Channel,
			Keyword:  result.Keyword,
			Limit:    result.Limit,
		}),
		"remark":        result.Remark,
		"notifications": items,
	}
}

func saasAdminBuildBusinessMetricsReport(options SaaSAdminBusinessMetricsOptions, overview SaaSAdminOverview, riskReport SaaSAdminRiskReport, packages []SaaSAdminPackage, billingEvents []SaaSAdminBillingEvent) SaaSAdminBusinessMetricsReport {
	latestAmounts := saasAdminBusinessLatestPackageAmounts(billingEvents)
	riskByTenant := make(map[int]SaaSAdminRiskTenant, len(riskReport.Items))
	for _, item := range riskReport.Items {
		riskByTenant[item.Tenant.TenantID] = item
	}
	packageMetrics := make(map[string]*SaaSAdminBusinessPackageMetric, len(packages)+len(overview.Tenants))
	for _, item := range packages {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		packageMetrics[code] = &SaaSAdminBusinessPackageMetric{
			PackageCode:   code,
			PackageName:   strings.TrimSpace(item.Name),
			PackageStatus: item.Status,
		}
	}
	summary := SaaSAdminBusinessMetricsSummary{
		TenantCount:              overview.Summary.TenantCount,
		ActiveTenantPackageCount: overview.Summary.ActiveTenantPackageCount,
		RecentBillingEventCount:  len(billingEvents),
	}
	for _, event := range billingEvents {
		if event.EventType == "renewal" {
			summary.RecentRenewalCount++
		}
		if event.EventType == "refund" {
			summary.RecentRefundCount++
			summary.RecentRefundAmountCents += event.AmountCents
		} else {
			summary.RecentGrossAmountCents += event.AmountCents
		}
		summary.RecentBillingAmountCents += saasAdminBillingSignedAmount(event)
	}
	for _, tenant := range overview.Tenants {
		code := strings.TrimSpace(tenant.PackageCode)
		if code == "" {
			continue
		}
		metric := packageMetrics[code]
		if metric == nil {
			metric = &SaaSAdminBusinessPackageMetric{
				PackageCode:   code,
				PackageName:   strings.TrimSpace(tenant.PackageName),
				PackageStatus: tenant.PackageStatus,
			}
			packageMetrics[code] = metric
		}
		if metric.PackageName == "" {
			metric.PackageName = strings.TrimSpace(tenant.PackageName)
		}
		if metric.PackageStatus == 0 {
			metric.PackageStatus = tenant.PackageStatus
		}
		metric.TenantCount++
		activeTenant := tenant.TenantStatus != 2 && tenant.PackageStatus == 1
		if activeTenant {
			metric.ActiveTenantCount++
		}
		amount := latestAmounts[code]
		if amount.AmountCents > 0 {
			metric.LatestAmountCents = amount.AmountCents
			metric.LatestBillingEventID = amount.BillingEventID
			metric.LatestBillingAt = amount.BillingAt
			metric.Estimated = true
		}
		monthlyCents := saasAdminBusinessMonthlyAmount(amount.AmountCents)
		if activeTenant && !tenant.Expired {
			if monthlyCents > 0 {
				metric.PricedTenantCount++
				metric.EstimatedMRRCents += monthlyCents
				summary.PricedTenantCount++
				summary.EstimatedMRRCents += monthlyCents
			} else {
				metric.UnknownPriceTenantCount++
				summary.UnknownPriceTenantCount++
			}
		}
		risk := riskByTenant[tenant.TenantID]
		if risk.RiskLevel != "" && risk.RiskLevel != "normal" {
			metric.AtRiskTenantCount++
			summary.AtRiskTenantCount++
			if monthlyCents > 0 {
				metric.AtRiskMRRCents += monthlyCents
				summary.AtRiskMRRCents += monthlyCents
			}
		}
		if tenant.ExpiringSoon {
			metric.ExpiringSoonTenantCount++
			summary.ExpiringSoonTenantCount++
			if monthlyCents > 0 {
				metric.ExpiringSoonMRRCents += monthlyCents
				summary.ExpiringSoonMRRCents += monthlyCents
			}
		}
		if tenant.Expired {
			metric.ExpiredTenantCount++
			summary.ExpiredTenantCount++
			if monthlyCents > 0 {
				metric.ExpiredMRRCents += monthlyCents
				summary.ExpiredMRRCents += monthlyCents
			}
		}
	}
	items := make([]SaaSAdminBusinessPackageMetric, 0, len(packageMetrics))
	for _, metric := range packageMetrics {
		metric.EstimatedARRCents = metric.EstimatedMRRCents * 12
		if metric.TenantCount > 0 && metric.LatestAmountCents <= 0 {
			summary.MissingBillingPackageCount++
		}
		if metric.LatestAmountCents > 0 {
			summary.BillingPricePackageCount++
		}
		items = append(items, *metric)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.EstimatedMRRCents != right.EstimatedMRRCents {
			return left.EstimatedMRRCents > right.EstimatedMRRCents
		}
		if left.AtRiskMRRCents != right.AtRiskMRRCents {
			return left.AtRiskMRRCents > right.AtRiskMRRCents
		}
		if left.TenantCount != right.TenantCount {
			return left.TenantCount > right.TenantCount
		}
		return left.PackageCode < right.PackageCode
	})
	summary.EstimatedARRCents = summary.EstimatedMRRCents * 12
	if summary.PricedTenantCount > 0 {
		summary.EstimatedARPACents = summary.EstimatedMRRCents / int64(summary.PricedTenantCount)
	}
	return SaaSAdminBusinessMetricsReport{
		Options:       options,
		Summary:       summary,
		Packages:      items,
		BillingEvents: billingEvents,
	}
}

type saasAdminBusinessPackageAmount struct {
	AmountCents    int64
	BillingEventID int64
	BillingAt      string
}

func saasAdminBusinessLatestPackageAmounts(events []SaaSAdminBillingEvent) map[string]saasAdminBusinessPackageAmount {
	amounts := make(map[string]saasAdminBusinessPackageAmount)
	for _, event := range events {
		code := strings.TrimSpace(event.PackageCode)
		if code == "" || event.AmountCents <= 0 || event.EventType != "renewal" {
			continue
		}
		if _, ok := amounts[code]; ok {
			continue
		}
		amounts[code] = saasAdminBusinessPackageAmount{
			AmountCents:    event.AmountCents,
			BillingEventID: event.ID,
			BillingAt:      saasAdminFirstNonEmpty(event.PaidAt, event.CreatedAt),
		}
	}
	return amounts
}

func saasAdminBusinessMonthlyAmount(annualAmountCents int64) int64 {
	if annualAmountCents <= 0 {
		return 0
	}
	return annualAmountCents / 12
}

func saasAdminBuildBusinessTrendReport(options SaaSAdminBusinessTrendOptions, billingEvents []SaaSAdminBillingEvent, taskSummary SaaSAdminTaskSummary, tasks []SaaSAdminTask, now time.Time) SaaSAdminBusinessTrendReport {
	if options.Months <= 0 {
		options.Months = 6
	}
	if options.Months > 24 {
		options.Months = 24
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	start := monthStart.AddDate(0, -options.Months+1, 0)
	months := make([]SaaSAdminBusinessTrendMonth, 0, options.Months)
	monthIndexes := make(map[string]int, options.Months)
	for i := 0; i < options.Months; i++ {
		month := start.AddDate(0, i, 0).Format("2006-01")
		monthIndexes[month] = i
		months = append(months, SaaSAdminBusinessTrendMonth{
			Month:              month,
			tenantIDs:          map[int]struct{}{},
			packageCodes:       map[string]struct{}{},
			packageMetricsByID: map[string]*SaaSAdminBusinessTrendPackage{},
		})
	}
	summary := SaaSAdminBusinessTrendSummary{
		MonthCount:          len(months),
		TaskCount:           taskSummary.TaskCount,
		PendingTaskCount:    taskSummary.PendingCount,
		BlockedTaskCount:    taskSummary.BlockedCount,
		FailedTaskCount:     taskSummary.FailedCount,
		AppliedTaskCount:    taskSummary.AppliedCount,
		CanceledTaskCount:   taskSummary.CanceledCount,
		ActionableTaskCount: taskSummary.ActionableCount,
	}
	globalTenants := map[int]struct{}{}
	globalPackages := map[string]struct{}{}
	for _, event := range billingEvents {
		eventTime, ok := saasAdminBusinessTrendEventTime(event)
		if !ok {
			continue
		}
		month := eventTime.Format("2006-01")
		index, ok := monthIndexes[month]
		if !ok {
			continue
		}
		bucket := &months[index]
		bucket.EventCount++
		bucket.AmountCents += saasAdminBillingSignedAmount(event)
		summary.BillingEventCount++
		summary.BillingAmountCents += saasAdminBillingSignedAmount(event)
		if event.EventType == "renewal" {
			bucket.RenewalCount++
			summary.RenewalCount++
		}
		if event.EventType == "refund" {
			bucket.RefundCount++
			bucket.RefundAmountCents += event.AmountCents
			summary.RefundCount++
			summary.RefundAmountCents += event.AmountCents
		} else {
			bucket.GrossAmountCents += event.AmountCents
			summary.GrossAmountCents += event.AmountCents
		}
		if event.TenantID > 0 {
			bucket.tenantIDs[event.TenantID] = struct{}{}
			globalTenants[event.TenantID] = struct{}{}
		}
		packageID, packageCode, packageName, ok := saasAdminBusinessTrendPackageIdentity(event)
		if !ok {
			continue
		}
		bucket.packageCodes[packageID] = struct{}{}
		globalPackages[packageID] = struct{}{}
		metric := bucket.packageMetricsByID[packageID]
		if metric == nil {
			metric = &SaaSAdminBusinessTrendPackage{
				PackageCode: packageCode,
				PackageName: packageName,
			}
			bucket.packageMetricsByID[packageID] = metric
		}
		metric.EventCount++
		metric.AmountCents += saasAdminBillingSignedAmount(event)
		if event.EventType == "renewal" {
			metric.RenewalCount++
		}
		if event.EventType == "refund" {
			metric.RefundCount++
			metric.RefundAmountCents += event.AmountCents
		} else {
			metric.GrossAmountCents += event.AmountCents
		}
	}
	for i := range months {
		months[i].TenantCount = len(months[i].tenantIDs)
		months[i].PackageCount = len(months[i].packageCodes)
		months[i].Packages = saasAdminBusinessTrendPackages(months[i].packageMetricsByID)
		months[i].tenantIDs = nil
		months[i].packageCodes = nil
		months[i].packageMetricsByID = nil
	}
	summary.TenantCount = len(globalTenants)
	summary.PackageCount = len(globalPackages)
	return SaaSAdminBusinessTrendReport{
		Options: options,
		Summary: summary,
		Months:  months,
		RenewalFunnel: SaaSAdminBusinessRenewalFunnel{
			Summary:     taskSummary,
			RecentTasks: tasks,
		},
	}
}

func saasAdminBusinessTrendEventTime(event SaaSAdminBillingEvent) (time.Time, bool) {
	if parsed, ok := parseSaaSAdminNormalizedDateTime(event.PaidAt); ok {
		return parsed, true
	}
	return parseSaaSAdminNormalizedDateTime(event.CreatedAt)
}

func saasAdminBillingSignedAmount(event SaaSAdminBillingEvent) int64 {
	if event.EventType == "refund" {
		return -event.AmountCents
	}
	return event.AmountCents
}

func saasAdminBusinessTrendPackageIdentity(event SaaSAdminBillingEvent) (string, string, string, bool) {
	code := strings.TrimSpace(event.PackageCode)
	name := strings.TrimSpace(event.PackageName)
	if code == "" && name == "" {
		return "", "", "", false
	}
	if code == "" {
		code = "unknown"
	}
	if name == "" {
		name = code
	}
	return code + "\x00" + name, code, name, true
}

func saasAdminBusinessTrendPackages(items map[string]*SaaSAdminBusinessTrendPackage) []SaaSAdminBusinessTrendPackage {
	packages := make([]SaaSAdminBusinessTrendPackage, 0, len(items))
	for _, item := range items {
		packages = append(packages, *item)
	}
	sort.SliceStable(packages, func(i, j int) bool {
		left := packages[i]
		right := packages[j]
		if left.AmountCents != right.AmountCents {
			return left.AmountCents > right.AmountCents
		}
		if left.RenewalCount != right.RenewalCount {
			return left.RenewalCount > right.RenewalCount
		}
		return left.PackageCode < right.PackageCode
	})
	return packages
}

func saasAdminLimitTasks(tasks []SaaSAdminTask, limit int) []SaaSAdminTask {
	if limit <= 0 || len(tasks) <= limit {
		return tasks
	}
	return tasks[:limit]
}

func saasAdminBuildRenewalForecastReport(options SaaSAdminRenewalForecastOptions, overview SaaSAdminOverview, billingEvents []SaaSAdminBillingEvent, taskSummary SaaSAdminTaskSummary, tasks []SaaSAdminTask, riskFollowUps map[int]SaaSAdminRiskFollowUpSnapshot, now time.Time) SaaSAdminRenewalForecastReport {
	if options.Days <= 0 {
		options.Days = 90
	}
	if options.Days > 365 {
		options.Days = 365
	}
	if strings.TrimSpace(options.Bucket) == "" {
		options.Bucket = SaaSAdminRenewalForecastFilterAll
	}
	if strings.TrimSpace(options.PriceState) == "" {
		options.PriceState = SaaSAdminRenewalForecastFilterAll
	}
	if strings.TrimSpace(options.TaskStatus) == "" {
		options.TaskStatus = SaaSAdminRenewalForecastFilterAll
	}
	latestAmounts := saasAdminBusinessLatestPackageAmounts(billingEvents)
	taskSummaryByTenant := saasAdminRenewalForecastTaskSummaryByTenant(tasks)
	latestTaskByTenant := saasAdminRenewalForecastLatestTaskByTenant(tasks)
	ownerFilter := strings.ToLower(strings.TrimSpace(options.Owner))
	packageFilter := strings.ToLower(strings.TrimSpace(options.PackageCode))
	buckets := saasAdminRenewalForecastInitialBuckets()
	bucketByName := map[string]*SaaSAdminRenewalForecastBucket{}
	for i := range buckets {
		bucketByName[buckets[i].Bucket] = &buckets[i]
	}
	report := SaaSAdminRenewalForecastReport{
		Options:       options,
		TaskSummary:   taskSummary,
		RecentTasks:   tasks,
		BillingEvents: billingEvents,
		Summary: SaaSAdminRenewalForecastSummary{
			TenantCount:         overview.Summary.TenantCount,
			ActionableTaskCount: taskSummary.ActionableCount,
			PendingTaskCount:    taskSummary.PendingCount,
			BlockedTaskCount:    taskSummary.BlockedCount,
			FailedTaskCount:     taskSummary.FailedCount,
			AppliedTaskCount:    taskSummary.AppliedCount,
			CanceledTaskCount:   taskSummary.CanceledCount,
		},
		Buckets: buckets,
		Items:   make([]SaaSAdminRenewalForecastTenant, 0, len(overview.Tenants)),
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	for _, tenant := range overview.Tenants {
		if tenant.TenantStatus == 2 || strings.TrimSpace(tenant.PackageCode) == "" {
			continue
		}
		if packageFilter != "" && strings.ToLower(strings.TrimSpace(tenant.PackageCode)) != packageFilter {
			continue
		}
		expiresAt, ok := parseSaaSAdminNormalizedDateTime(tenant.ExpiresAt)
		if !ok {
			continue
		}
		expireDay := time.Date(expiresAt.Year(), expiresAt.Month(), expiresAt.Day(), 0, 0, 0, 0, time.Local)
		daysUntil := int(expireDay.Sub(today).Hours() / 24)
		if daysUntil > options.Days {
			continue
		}
		bucketName, bucketLabel := saasAdminRenewalForecastBucket(daysUntil)
		if options.Bucket != SaaSAdminRenewalForecastFilterAll && options.Bucket != bucketName {
			continue
		}
		amount := latestAmounts[strings.TrimSpace(tenant.PackageCode)]
		renewalAmountCents := amount.AmountCents
		estimatedMRR := saasAdminBusinessMonthlyAmount(renewalAmountCents)
		item := SaaSAdminRenewalForecastTenant{
			Tenant:               tenant,
			Bucket:               bucketName,
			BucketLabel:          bucketLabel,
			DaysUntil:            daysUntil,
			RenewalAmountCents:   renewalAmountCents,
			EstimatedMRRCents:    estimatedMRR,
			Priced:               renewalAmountCents > 0,
			LatestBillingEventID: amount.BillingEventID,
			LatestBillingAt:      amount.BillingAt,
			TaskSummary:          taskSummaryByTenant[tenant.TenantID],
			Owner:                "未分配",
		}
		if followUp, ok := riskFollowUps[tenant.TenantID]; ok && followUp.OperationID > 0 {
			item.RiskFollowUp = followUp
			item.HasRiskFollowUp = true
			if owner := strings.TrimSpace(followUp.Owner); owner != "" {
				item.Owner = owner
			}
		}
		if task, ok := latestTaskByTenant[tenant.TenantID]; ok {
			item.LatestTask = task
			item.HasLatestTask = true
		}
		if options.PriceState == SaaSAdminRenewalForecastPriceStatePriced && !item.Priced {
			continue
		}
		if options.PriceState == SaaSAdminRenewalForecastPriceStateUnknown && item.Priced {
			continue
		}
		latestTaskStatus := SaaSAdminRenewalForecastTaskStatusNone
		if item.HasLatestTask {
			latestTaskStatus = item.LatestTask.Status
		}
		if options.TaskStatus != SaaSAdminRenewalForecastFilterAll && options.TaskStatus != latestTaskStatus {
			continue
		}
		if ownerFilter != "" && !strings.Contains(strings.ToLower(item.Owner), ownerFilter) {
			continue
		}
		report.Items = append(report.Items, item)
		saasAdminRenewalForecastCountItem(&report.Summary, bucketByName[bucketName], item)
	}
	sort.SliceStable(report.Items, func(i, j int) bool {
		left := report.Items[i]
		right := report.Items[j]
		if left.DaysUntil != right.DaysUntil {
			return left.DaysUntil < right.DaysUntil
		}
		if left.RenewalAmountCents != right.RenewalAmountCents {
			return left.RenewalAmountCents > right.RenewalAmountCents
		}
		return left.Tenant.TenantID < right.Tenant.TenantID
	})
	report.Summary.ForecastTenantCount = len(report.Items)
	report.Owners = saasAdminRenewalForecastOwnerSummaries(report.Items)
	report.Buckets = buckets
	return report
}

func saasAdminRenewalForecastInitialBuckets() []SaaSAdminRenewalForecastBucket {
	return []SaaSAdminRenewalForecastBucket{
		{Bucket: "expired", Label: "已到期"},
		{Bucket: "due_0_30", Label: "30 天内"},
		{Bucket: "due_31_60", Label: "31-60 天"},
		{Bucket: "due_61_90", Label: "61-90 天"},
		{Bucket: "due_later", Label: "90 天以上"},
	}
}

func saasAdminRenewalForecastBucket(daysUntil int) (string, string) {
	switch {
	case daysUntil < 0:
		return "expired", "已到期"
	case daysUntil <= 30:
		return "due_0_30", "30 天内"
	case daysUntil <= 60:
		return "due_31_60", "31-60 天"
	case daysUntil <= 90:
		return "due_61_90", "61-90 天"
	default:
		return "due_later", "90 天以上"
	}
}

func saasAdminRenewalForecastCountItem(summary *SaaSAdminRenewalForecastSummary, bucket *SaaSAdminRenewalForecastBucket, item SaaSAdminRenewalForecastTenant) {
	if summary == nil || bucket == nil {
		return
	}
	summary.RenewalAmountCents += item.RenewalAmountCents
	summary.EstimatedMRRCents += item.EstimatedMRRCents
	bucket.TenantCount++
	bucket.RenewalAmountCents += item.RenewalAmountCents
	bucket.EstimatedMRRCents += item.EstimatedMRRCents
	if item.Priced {
		summary.PricedTenantCount++
		bucket.PricedTenantCount++
	} else {
		summary.UnknownPriceTenantCount++
		bucket.UnknownPriceTenantCount++
	}
	bucket.ActionableTaskCount += item.TaskSummary.ActionableCount
	switch item.Bucket {
	case "expired":
		summary.ExpiredTenantCount++
		summary.ExpiredAmountCents += item.RenewalAmountCents
	case "due_0_30":
		summary.DueWithin30TenantCount++
		summary.DueWithin30AmountCents += item.RenewalAmountCents
	case "due_31_60":
		summary.Due31To60TenantCount++
		summary.Due31To60AmountCents += item.RenewalAmountCents
	case "due_61_90":
		summary.Due61To90TenantCount++
		summary.Due61To90AmountCents += item.RenewalAmountCents
	case "due_later":
		summary.DueLaterTenantCount++
		summary.DueLaterAmountCents += item.RenewalAmountCents
	}
}

func saasAdminRenewalForecastOwnerSummaries(items []SaaSAdminRenewalForecastTenant) []SaaSAdminRenewalForecastOwnerSummary {
	ownerMap := map[string]*SaaSAdminRenewalForecastOwnerSummary{}
	for _, item := range items {
		owner := strings.TrimSpace(item.Owner)
		if owner == "" {
			owner = "未分配"
		}
		summary := ownerMap[owner]
		if summary == nil {
			summary = &SaaSAdminRenewalForecastOwnerSummary{Owner: owner}
			ownerMap[owner] = summary
		}
		summary.TenantCount++
		summary.RenewalAmountCents += item.RenewalAmountCents
		summary.EstimatedMRRCents += item.EstimatedMRRCents
		if item.Priced {
			summary.PricedTenantCount++
		} else {
			summary.UnknownPriceTenantCount++
		}
		switch item.Bucket {
		case "expired":
			summary.ExpiredTenantCount++
		case "due_0_30":
			summary.DueWithin30TenantCount++
		case "due_31_60":
			summary.Due31To60TenantCount++
		case "due_61_90":
			summary.Due61To90TenantCount++
		case "due_later":
			summary.DueLaterTenantCount++
		}
		summary.ActionableTaskCount += item.TaskSummary.ActionableCount
		summary.PendingTaskCount += item.TaskSummary.PendingCount
		summary.BlockedTaskCount += item.TaskSummary.BlockedCount
		summary.FailedTaskCount += item.TaskSummary.FailedCount
		summary.AppliedTaskCount += item.TaskSummary.AppliedCount
		summary.CanceledTaskCount += item.TaskSummary.CanceledCount
		if item.HasRiskFollowUp && strings.TrimSpace(item.RiskFollowUp.NextFollowUpAt) != "" {
			nextFollowUpAt := strings.TrimSpace(item.RiskFollowUp.NextFollowUpAt)
			if summary.NextFollowUpAt == "" || nextFollowUpAt < summary.NextFollowUpAt {
				summary.NextFollowUpAt = nextFollowUpAt
			}
		}
		if len(summary.TopTenants) < 3 {
			summary.TopTenants = append(summary.TopTenants, item)
		}
	}
	owners := make([]SaaSAdminRenewalForecastOwnerSummary, 0, len(ownerMap))
	for _, summary := range ownerMap {
		owners = append(owners, *summary)
	}
	sort.SliceStable(owners, func(i, j int) bool {
		left := owners[i]
		right := owners[j]
		if left.ExpiredTenantCount != right.ExpiredTenantCount {
			return left.ExpiredTenantCount > right.ExpiredTenantCount
		}
		if left.DueWithin30TenantCount != right.DueWithin30TenantCount {
			return left.DueWithin30TenantCount > right.DueWithin30TenantCount
		}
		if left.RenewalAmountCents != right.RenewalAmountCents {
			return left.RenewalAmountCents > right.RenewalAmountCents
		}
		if left.ActionableTaskCount != right.ActionableTaskCount {
			return left.ActionableTaskCount > right.ActionableTaskCount
		}
		return left.Owner < right.Owner
	})
	return owners
}

func saasAdminRenewalForecastTaskSummaryByTenant(tasks []SaaSAdminTask) map[int]SaaSAdminTaskSummary {
	byTenant := map[int]SaaSAdminTaskSummary{}
	for _, task := range tasks {
		if task.TenantID <= 0 || task.TaskType != SaaSAdminTaskTypeTenantRenewal {
			continue
		}
		summary := byTenant[task.TenantID]
		summary.TaskCount++
		switch task.Status {
		case SaaSAdminTaskStatusPending:
			summary.PendingCount++
			summary.ActionableCount++
		case SaaSAdminTaskStatusBlocked:
			summary.BlockedCount++
			summary.ActionableCount++
		case SaaSAdminTaskStatusFailed:
			summary.FailedCount++
			summary.ActionableCount++
		case SaaSAdminTaskStatusApplied:
			summary.AppliedCount++
		case SaaSAdminTaskStatusCanceled:
			summary.CanceledCount++
		}
		summary.TenantRenewalCount++
		summary.TenantCount = 1
		if task.ActorUserID > 0 {
			summary.ActorUserCount = 1
		}
		byTenant[task.TenantID] = summary
	}
	return byTenant
}

func saasAdminRenewalForecastLatestTaskByTenant(tasks []SaaSAdminTask) map[int]SaaSAdminTask {
	byTenant := map[int]SaaSAdminTask{}
	for _, task := range tasks {
		if task.TenantID <= 0 || task.TaskType != SaaSAdminTaskTypeTenantRenewal {
			continue
		}
		existing, ok := byTenant[task.TenantID]
		if !ok || saasAdminDateTimeBefore(existing.CreatedAt, task.CreatedAt) {
			byTenant[task.TenantID] = task
		}
	}
	return byTenant
}

func saasAdminBuildRiskReport(overview SaaSAdminOverview, usageByTenant map[int][]SaaSAdminUsageMetric, highUsageRatio float64) SaaSAdminRiskReport {
	if highUsageRatio <= 0 {
		highUsageRatio = saasAdminDefaultHighUsageRatio
	}
	report := SaaSAdminRiskReport{
		Summary: SaaSAdminRiskSummary{
			TotalTenantCount:     overview.Summary.TenantCount,
			EvaluatedTenantCount: len(overview.Tenants),
		},
		Items: make([]SaaSAdminRiskTenant, 0, len(overview.Tenants)),
	}
	for _, tenant := range overview.Tenants {
		item := saasAdminBuildRiskTenant(tenant, usageByTenant[tenant.TenantID], highUsageRatio)
		report.Items = append(report.Items, item)
		saasAdminCountRiskTenant(&report.Summary, item)
	}
	sort.SliceStable(report.Items, func(i, j int) bool {
		left := report.Items[i]
		right := report.Items[j]
		if left.RiskScore != right.RiskScore {
			return left.RiskScore > right.RiskScore
		}
		if left.Tenant.OpenAlertCount != right.Tenant.OpenAlertCount {
			return left.Tenant.OpenAlertCount > right.Tenant.OpenAlertCount
		}
		if left.HighUsageRatio != right.HighUsageRatio {
			return left.HighUsageRatio > right.HighUsageRatio
		}
		return left.Tenant.TenantID < right.Tenant.TenantID
	})
	return report
}

func saasAdminRiskTenantIDs(items []SaaSAdminRiskTenant) []int {
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(items))
	for _, item := range items {
		tenantID := item.Tenant.TenantID
		if tenantID <= 0 {
			continue
		}
		if _, ok := seen[tenantID]; ok {
			continue
		}
		seen[tenantID] = struct{}{}
		ids = append(ids, tenantID)
	}
	return ids
}

func saasAdminTenantOverviewIDs(items []SaaSAdminTenantOverview) []int {
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(items))
	for _, item := range items {
		tenantID := item.TenantID
		if tenantID <= 0 {
			continue
		}
		if _, ok := seen[tenantID]; ok {
			continue
		}
		seen[tenantID] = struct{}{}
		ids = append(ids, tenantID)
	}
	return ids
}

func saasAdminAttachRiskFollowUps(report *SaaSAdminRiskReport, followUps map[int]SaaSAdminRiskFollowUpSnapshot, now time.Time) {
	if report == nil || len(followUps) == 0 {
		return
	}
	for i := range report.Items {
		followUp, ok := followUps[report.Items[i].Tenant.TenantID]
		if !ok || followUp.OperationID <= 0 {
			continue
		}
		report.Items[i].FollowUp = followUp
		saasAdminCountRiskFollowUp(&report.Summary, followUp, now)
	}
}

func saasAdminCountRiskFollowUp(summary *SaaSAdminRiskSummary, followUp SaaSAdminRiskFollowUpSnapshot, now time.Time) {
	if summary == nil || followUp.OperationID <= 0 {
		return
	}
	summary.FollowUpTenantCount++
	switch followUp.Status {
	case SaaSAdminRiskFollowUpStatusPending, SaaSAdminRiskFollowUpStatusContacted:
		summary.PendingFollowUpCount++
	case SaaSAdminRiskFollowUpStatusRenewalPending:
		summary.PendingFollowUpCount++
		summary.RenewalPendingCount++
	}
	if saasAdminRiskFollowUpOverdue(followUp, now) {
		summary.OverdueFollowUpCount++
	}
}

func saasAdminRiskFollowUpOverdue(followUp SaaSAdminRiskFollowUpSnapshot, now time.Time) bool {
	if strings.TrimSpace(followUp.NextFollowUpAt) == "" {
		return false
	}
	if followUp.Status == SaaSAdminRiskFollowUpStatusResolved || followUp.Status == SaaSAdminRiskFollowUpStatusIgnored {
		return false
	}
	dueAt, ok := saasAdminParseRiskFollowUpTime(followUp.NextFollowUpAt)
	if !ok {
		return false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return dueAt.Before(today)
}

func saasAdminParseRiskFollowUpTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		parsed, err := time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func saasAdminRiskFollowUpDueStateRank(state string) int {
	switch state {
	case SaaSAdminRiskFollowUpDueStateOverdue:
		return 0
	case SaaSAdminRiskFollowUpDueStateDueSoon:
		return 1
	case SaaSAdminRiskFollowUpDueStateFuture:
		return 2
	case SaaSAdminRiskFollowUpDueStateNoDate:
		return 3
	case SaaSAdminRiskFollowUpDueStateClosed:
		return 4
	default:
		return 5
	}
}

func saasAdminBuildRiskFollowUpTasks(snapshots []SaaSAdminRiskFollowUpSnapshot, options SaaSAdminRiskFollowUpTaskOptions, now time.Time) ([]SaaSAdminRiskFollowUpTask, SaaSAdminRiskFollowUpTaskSummary) {
	tasks := make([]SaaSAdminRiskFollowUpTask, 0, len(snapshots))
	for _, snapshot := range snapshots {
		task := saasAdminRiskFollowUpTask(snapshot, now)
		if options.DueState != "" && options.DueState != SaaSAdminRiskFollowUpDueStateAll && task.DueState != options.DueState {
			continue
		}
		tasks = append(tasks, task)
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		left := tasks[i]
		right := tasks[j]
		if left.Overdue != right.Overdue {
			return left.Overdue
		}
		leftHasDate := strings.TrimSpace(left.NextFollowUpAt) != ""
		rightHasDate := strings.TrimSpace(right.NextFollowUpAt) != ""
		if leftHasDate != rightHasDate {
			return leftHasDate
		}
		if leftHasDate && rightHasDate && left.NextFollowUpAt != right.NextFollowUpAt {
			return left.NextFollowUpAt < right.NextFollowUpAt
		}
		if left.CreatedAt != right.CreatedAt {
			return left.CreatedAt > right.CreatedAt
		}
		return left.TenantID < right.TenantID
	})
	summary := SaaSAdminRiskFollowUpTaskSummary{}
	for _, task := range tasks {
		saasAdminCountRiskFollowUpTask(&summary, task)
	}
	if options.Limit > 0 && len(tasks) > options.Limit {
		tasks = tasks[:options.Limit]
	}
	return tasks, summary
}

func saasAdminRiskFollowUpTask(snapshot SaaSAdminRiskFollowUpSnapshot, now time.Time) SaaSAdminRiskFollowUpTask {
	task := SaaSAdminRiskFollowUpTask{
		SaaSAdminRiskFollowUpSnapshot: snapshot,
		DueState:                      SaaSAdminRiskFollowUpDueStateNoDate,
	}
	if snapshot.Status == SaaSAdminRiskFollowUpStatusResolved || snapshot.Status == SaaSAdminRiskFollowUpStatusIgnored {
		task.DueState = SaaSAdminRiskFollowUpDueStateClosed
		return task
	}
	dueAt, ok := saasAdminParseRiskFollowUpTime(snapshot.NextFollowUpAt)
	if !ok {
		return task
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	daysUntil := int(dueAt.Sub(today).Hours() / 24)
	task.DaysUntil = daysUntil
	switch {
	case dueAt.Before(today):
		task.DueState = SaaSAdminRiskFollowUpDueStateOverdue
		task.Overdue = true
	case !dueAt.After(today.AddDate(0, 0, 7)):
		task.DueState = SaaSAdminRiskFollowUpDueStateDueSoon
	default:
		task.DueState = SaaSAdminRiskFollowUpDueStateFuture
	}
	return task
}

func saasAdminCountRiskFollowUpTask(summary *SaaSAdminRiskFollowUpTaskSummary, task SaaSAdminRiskFollowUpTask) {
	if summary == nil {
		return
	}
	summary.TotalCount++
	switch task.Status {
	case SaaSAdminRiskFollowUpStatusPending:
		summary.PendingCount++
	case SaaSAdminRiskFollowUpStatusContacted:
		summary.ContactedCount++
	case SaaSAdminRiskFollowUpStatusRenewalPending:
		summary.RenewalPendingCount++
	case SaaSAdminRiskFollowUpStatusResolved:
		summary.ResolvedCount++
	case SaaSAdminRiskFollowUpStatusIgnored:
		summary.IgnoredCount++
	}
	switch task.DueState {
	case SaaSAdminRiskFollowUpDueStateOverdue:
		summary.OverdueCount++
	case SaaSAdminRiskFollowUpDueStateDueSoon:
		summary.DueSoonCount++
	case SaaSAdminRiskFollowUpDueStateNoDate:
		summary.NoDateCount++
	case SaaSAdminRiskFollowUpDueStateClosed:
		summary.ClosedCount++
	}
}

func saasAdminBuildRiskFollowUpOwnerSummaries(tasks []SaaSAdminRiskFollowUpTask) []SaaSAdminRiskFollowUpOwnerSummary {
	byOwner := map[string]*SaaSAdminRiskFollowUpOwnerSummary{}
	for _, task := range tasks {
		ownerName := strings.TrimSpace(task.Owner)
		if ownerName == "" {
			ownerName = "未分配"
		}
		summary := byOwner[ownerName]
		if summary == nil {
			summary = &SaaSAdminRiskFollowUpOwnerSummary{Owner: ownerName}
			byOwner[ownerName] = summary
		}
		summary.TotalCount++
		if task.Status != SaaSAdminRiskFollowUpStatusResolved && task.Status != SaaSAdminRiskFollowUpStatusIgnored {
			summary.OpenCount++
		}
		switch task.Status {
		case SaaSAdminRiskFollowUpStatusPending:
			summary.PendingCount++
		case SaaSAdminRiskFollowUpStatusContacted:
			summary.ContactedCount++
		case SaaSAdminRiskFollowUpStatusRenewalPending:
			summary.RenewalPendingCount++
		case SaaSAdminRiskFollowUpStatusResolved:
			summary.ResolvedCount++
		case SaaSAdminRiskFollowUpStatusIgnored:
			summary.IgnoredCount++
		}
		switch task.DueState {
		case SaaSAdminRiskFollowUpDueStateOverdue:
			summary.OverdueCount++
		case SaaSAdminRiskFollowUpDueStateDueSoon:
			summary.DueSoonCount++
		case SaaSAdminRiskFollowUpDueStateFuture:
			summary.FutureCount++
		case SaaSAdminRiskFollowUpDueStateNoDate:
			summary.NoDateCount++
		case SaaSAdminRiskFollowUpDueStateClosed:
			summary.ClosedCount++
		}
		if task.CreatedAt > summary.LatestFollowUpAt {
			summary.LatestFollowUpAt = task.CreatedAt
		}
		if task.NextFollowUpAt != "" && task.DueState != SaaSAdminRiskFollowUpDueStateClosed && (summary.NextFollowUpAt == "" || task.NextFollowUpAt < summary.NextFollowUpAt) {
			summary.NextFollowUpAt = task.NextFollowUpAt
		}
	}
	owners := make([]SaaSAdminRiskFollowUpOwnerSummary, 0, len(byOwner))
	for _, summary := range byOwner {
		owners = append(owners, *summary)
	}
	sort.SliceStable(owners, func(i, j int) bool {
		left := owners[i]
		right := owners[j]
		if left.OverdueCount != right.OverdueCount {
			return left.OverdueCount > right.OverdueCount
		}
		if left.DueSoonCount != right.DueSoonCount {
			return left.DueSoonCount > right.DueSoonCount
		}
		if left.OpenCount != right.OpenCount {
			return left.OpenCount > right.OpenCount
		}
		if left.TotalCount != right.TotalCount {
			return left.TotalCount > right.TotalCount
		}
		return left.Owner < right.Owner
	})
	return owners
}

func saasAdminBuildBillingReconciliationFollowUpTasks(snapshots []SaaSAdminBillingReconciliationFollowUpSnapshot, options SaaSAdminBillingReconciliationFollowUpOptions, now time.Time) ([]SaaSAdminBillingReconciliationFollowUpTask, SaaSAdminRiskFollowUpTaskSummary) {
	tasks := make([]SaaSAdminBillingReconciliationFollowUpTask, 0, len(snapshots))
	status := strings.TrimSpace(options.Status)
	owner := strings.ToLower(strings.TrimSpace(options.Owner))
	keyword := strings.ToLower(strings.TrimSpace(options.Keyword))
	for _, snapshot := range snapshots {
		if status != "" && snapshot.Status != status {
			continue
		}
		if owner != "" && !strings.Contains(strings.ToLower(snapshot.Owner), owner) {
			continue
		}
		if keyword != "" && !saasAdminBillingReconciliationFollowUpSnapshotMatchesKeyword(snapshot, keyword) {
			continue
		}
		task := saasAdminBillingReconciliationFollowUpTask(snapshot, now)
		if options.DueState != "" && options.DueState != SaaSAdminRiskFollowUpDueStateAll && task.DueState != options.DueState {
			continue
		}
		tasks = append(tasks, task)
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		left := tasks[i]
		right := tasks[j]
		leftRank := saasAdminRiskFollowUpDueStateRank(left.DueState)
		rightRank := saasAdminRiskFollowUpDueStateRank(right.DueState)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		leftHasDate := strings.TrimSpace(left.NextFollowUpAt) != ""
		rightHasDate := strings.TrimSpace(right.NextFollowUpAt) != ""
		if leftHasDate != rightHasDate {
			return leftHasDate
		}
		if leftHasDate && rightHasDate && left.NextFollowUpAt != right.NextFollowUpAt {
			return left.NextFollowUpAt < right.NextFollowUpAt
		}
		if left.CreatedAt != right.CreatedAt {
			return left.CreatedAt > right.CreatedAt
		}
		return left.BillingEventID < right.BillingEventID
	})
	summary := SaaSAdminRiskFollowUpTaskSummary{}
	for _, task := range tasks {
		saasAdminCountBillingReconciliationFollowUpTask(&summary, task)
	}
	if options.Limit > 0 && len(tasks) > options.Limit {
		tasks = tasks[:options.Limit]
	}
	return tasks, summary
}

func saasAdminBillingReconciliationFollowUpTask(snapshot SaaSAdminBillingReconciliationFollowUpSnapshot, now time.Time) SaaSAdminBillingReconciliationFollowUpTask {
	task := SaaSAdminBillingReconciliationFollowUpTask{
		SaaSAdminBillingReconciliationFollowUpSnapshot: snapshot,
		DueState: SaaSAdminRiskFollowUpDueStateNoDate,
	}
	if snapshot.Status == SaaSAdminRiskFollowUpStatusResolved || snapshot.Status == SaaSAdminRiskFollowUpStatusIgnored {
		task.DueState = SaaSAdminRiskFollowUpDueStateClosed
		return task
	}
	dueAt, ok := saasAdminParseRiskFollowUpTime(snapshot.NextFollowUpAt)
	if !ok {
		return task
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	daysUntil := int(dueAt.Sub(today).Hours() / 24)
	task.DaysUntil = daysUntil
	switch {
	case dueAt.Before(today):
		task.DueState = SaaSAdminRiskFollowUpDueStateOverdue
		task.Overdue = true
	case !dueAt.After(today.AddDate(0, 0, 7)):
		task.DueState = SaaSAdminRiskFollowUpDueStateDueSoon
	default:
		task.DueState = SaaSAdminRiskFollowUpDueStateFuture
	}
	return task
}

func saasAdminCountBillingReconciliationFollowUpTask(summary *SaaSAdminRiskFollowUpTaskSummary, task SaaSAdminBillingReconciliationFollowUpTask) {
	if summary == nil {
		return
	}
	summary.TotalCount++
	switch task.Status {
	case SaaSAdminRiskFollowUpStatusPending:
		summary.PendingCount++
	case SaaSAdminRiskFollowUpStatusContacted:
		summary.ContactedCount++
	case SaaSAdminRiskFollowUpStatusRenewalPending:
		summary.RenewalPendingCount++
	case SaaSAdminRiskFollowUpStatusResolved:
		summary.ResolvedCount++
	case SaaSAdminRiskFollowUpStatusIgnored:
		summary.IgnoredCount++
	}
	switch task.DueState {
	case SaaSAdminRiskFollowUpDueStateOverdue:
		summary.OverdueCount++
	case SaaSAdminRiskFollowUpDueStateDueSoon:
		summary.DueSoonCount++
	case SaaSAdminRiskFollowUpDueStateNoDate:
		summary.NoDateCount++
	case SaaSAdminRiskFollowUpDueStateClosed:
		summary.ClosedCount++
	}
}

func saasAdminBuildBillingReconciliationFollowUpOwnerSummaries(tasks []SaaSAdminBillingReconciliationFollowUpTask) []SaaSAdminRiskFollowUpOwnerSummary {
	byOwner := map[string]*SaaSAdminRiskFollowUpOwnerSummary{}
	for _, task := range tasks {
		ownerName := strings.TrimSpace(task.Owner)
		if ownerName == "" {
			ownerName = "未分配"
		}
		summary := byOwner[ownerName]
		if summary == nil {
			summary = &SaaSAdminRiskFollowUpOwnerSummary{Owner: ownerName}
			byOwner[ownerName] = summary
		}
		summary.TotalCount++
		if task.Status != SaaSAdminRiskFollowUpStatusResolved && task.Status != SaaSAdminRiskFollowUpStatusIgnored {
			summary.OpenCount++
		}
		switch task.Status {
		case SaaSAdminRiskFollowUpStatusPending:
			summary.PendingCount++
		case SaaSAdminRiskFollowUpStatusContacted:
			summary.ContactedCount++
		case SaaSAdminRiskFollowUpStatusRenewalPending:
			summary.RenewalPendingCount++
		case SaaSAdminRiskFollowUpStatusResolved:
			summary.ResolvedCount++
		case SaaSAdminRiskFollowUpStatusIgnored:
			summary.IgnoredCount++
		}
		switch task.DueState {
		case SaaSAdminRiskFollowUpDueStateOverdue:
			summary.OverdueCount++
		case SaaSAdminRiskFollowUpDueStateDueSoon:
			summary.DueSoonCount++
		case SaaSAdminRiskFollowUpDueStateFuture:
			summary.FutureCount++
		case SaaSAdminRiskFollowUpDueStateNoDate:
			summary.NoDateCount++
		case SaaSAdminRiskFollowUpDueStateClosed:
			summary.ClosedCount++
		}
		if task.CreatedAt > summary.LatestFollowUpAt {
			summary.LatestFollowUpAt = task.CreatedAt
		}
		if task.NextFollowUpAt != "" && task.DueState != SaaSAdminRiskFollowUpDueStateClosed && (summary.NextFollowUpAt == "" || task.NextFollowUpAt < summary.NextFollowUpAt) {
			summary.NextFollowUpAt = task.NextFollowUpAt
		}
	}
	owners := make([]SaaSAdminRiskFollowUpOwnerSummary, 0, len(byOwner))
	for _, summary := range byOwner {
		owners = append(owners, *summary)
	}
	sort.SliceStable(owners, func(i, j int) bool {
		left := owners[i]
		right := owners[j]
		if left.OverdueCount != right.OverdueCount {
			return left.OverdueCount > right.OverdueCount
		}
		if left.DueSoonCount != right.DueSoonCount {
			return left.DueSoonCount > right.DueSoonCount
		}
		if left.OpenCount != right.OpenCount {
			return left.OpenCount > right.OpenCount
		}
		if left.TotalCount != right.TotalCount {
			return left.TotalCount > right.TotalCount
		}
		return left.Owner < right.Owner
	})
	return owners
}

func saasAdminBillingReconciliationFollowUpSnapshotMatchesKeyword(item SaaSAdminBillingReconciliationFollowUpSnapshot, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	fields := []string{
		strconv.FormatInt(item.BillingEventID, 10),
		strconv.Itoa(item.TenantID),
		item.TenantName,
		item.Status,
		item.Owner,
		item.NextFollowUpAt,
		item.Remark,
		item.PackageCode,
		item.PackageName,
		item.NewExpiresAt,
		item.Currency,
		item.ExternalOrderNo,
		item.CreatedAt,
		strconv.FormatInt(item.OperationID, 10),
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), keyword) {
			return true
		}
	}
	return false
}

type saasAdminCustomerSuccessBuildInput struct {
	PlatformAdminTenantID int
	RiskReport            SaaSAdminRiskReport
	RiskTasks             []SaaSAdminRiskFollowUpTask
	BillingTasks          []SaaSAdminBillingReconciliationFollowUpTask
	AdminTasks            []SaaSAdminTask
	Notifications         []SaaSAlertNotification
	Options               SaaSAdminCustomerSuccessOptions
}

type saasAdminOperationQueueBuildInput struct {
	CustomerSuccess    []SaaSAdminCustomerSuccessQueueItem
	TaskSLA            []SaaSAdminTaskSLAItem
	BillingTasks       []SaaSAdminBillingReconciliationFollowUpTask
	Notifications      []SaaSAlertNotification
	NotificationHealth []SaaSAdminNotificationHealthTenant
	Assignments        map[string]SaaSAdminOperationQueueAssignment
	Options            SaaSAdminOperationQueueOptions
}

type saasAdminCustomerSuccessNotificationCounts struct {
	Failed int
	Dead   int
}

func saasAdminBuildOperationQueue(input saasAdminOperationQueueBuildInput) ([]SaaSAdminOperationQueueItem, SaaSAdminOperationQueueSummary) {
	options := input.Options
	sourceFilter := strings.TrimSpace(options.Source)
	priorityFilter := strings.TrimSpace(options.Priority)
	ownerFilter := strings.ToLower(strings.TrimSpace(options.Owner))
	keywordFilter := strings.ToLower(strings.TrimSpace(options.Keyword))
	now := time.Now()

	tenantNames := map[int]string{}
	for _, item := range input.CustomerSuccess {
		if item.Tenant.TenantID > 0 && strings.TrimSpace(item.Tenant.TenantName) != "" {
			tenantNames[item.Tenant.TenantID] = item.Tenant.TenantName
		}
	}
	for _, item := range input.BillingTasks {
		if item.TenantID > 0 && strings.TrimSpace(item.TenantName) != "" {
			tenantNames[item.TenantID] = item.TenantName
		}
	}

	items := make([]SaaSAdminOperationQueueItem, 0)
	summary := SaaSAdminOperationQueueSummary{}
	tenantIDs := map[int]struct{}{}
	sourceNames := map[string]struct{}{}
	add := func(item SaaSAdminOperationQueueItem) {
		saasAdminApplyOperationQueueAssignment(&item, input.Assignments)
		if sourceFilter != "" && item.Source != sourceFilter {
			return
		}
		if priorityFilter != "" && item.Priority != priorityFilter {
			return
		}
		if ownerFilter != "" && !strings.Contains(strings.ToLower(strings.TrimSpace(item.Owner)), ownerFilter) {
			return
		}
		if keywordFilter != "" && !saasAdminOperationQueueItemMatchesKeyword(item, keywordFilter) {
			return
		}
		if item.AgeHours == 0 {
			if base, ok := parseSaaSAdminNormalizedDateTime(saasAdminFirstNonEmpty(item.CreatedAt, item.UpdatedAt)); ok {
				if base.After(now) {
					base = now
				}
				item.AgeHours = int(now.Sub(base).Hours())
			}
		}
		items = append(items, item)
		saasAdminCountOperationQueueItem(&summary, item)
		if item.TenantID > 0 {
			tenantIDs[item.TenantID] = struct{}{}
		}
		if strings.TrimSpace(item.Source) != "" {
			sourceNames[item.Source] = struct{}{}
		}
	}

	for _, item := range input.CustomerSuccess {
		add(SaaSAdminOperationQueueItem{
			ID:         fmt.Sprintf("%s:%d", SaaSAdminOperationQueueSourceCustomerSuccess, item.Tenant.TenantID),
			Source:     SaaSAdminOperationQueueSourceCustomerSuccess,
			Priority:   item.Priority,
			TenantID:   item.Tenant.TenantID,
			TenantName: item.Tenant.TenantName,
			Owner:      item.Owner,
			Title:      "客户成功待处理：" + item.Tenant.TenantName,
			Reason:     strings.Join(item.Reasons, "；"),
			NextAction: item.NextAction,
			DueState:   item.DueState,
			Status:     item.Priority,
			ObjectType: "tenant",
			ObjectID:   strconv.Itoa(item.Tenant.TenantID),
			Score:      item.HealthScore,
			Reference:  saasAdminCustomerSuccessQueueItemPayload(item),
		})
	}

	for _, item := range input.TaskSLA {
		task := item.Task
		if item.SLAStatus == "fresh" && task.Status == SaaSAdminTaskStatusPending {
			continue
		}
		priority := saasAdminOperationQueueTaskSLAPriority(item)
		title := fmt.Sprintf("运营任务 SLA：%s #%d", task.TaskType, task.ID)
		reason := fmt.Sprintf("状态=%s，SLA=%s，已等待 %d 小时", task.Status, saasAdminTaskSLAStatusLabel(item.SLAStatus), item.AgeHours)
		if strings.TrimSpace(task.LastError) != "" {
			reason += "，错误=" + strings.TrimSpace(task.LastError)
		}
		add(SaaSAdminOperationQueueItem{
			ID:         fmt.Sprintf("%s:%d", SaaSAdminOperationQueueSourceTaskSLA, task.ID),
			Source:     SaaSAdminOperationQueueSourceTaskSLA,
			Priority:   priority,
			TenantID:   task.TenantID,
			TenantName: tenantNames[task.TenantID],
			Owner:      item.Owner,
			Title:      title,
			Reason:     reason,
			NextAction: saasAdminOperationQueueTaskSLANextAction(item),
			DueState:   item.SLAStatus,
			Status:     task.Status,
			ObjectType: SaaSAdminOperationTargetAdminTask,
			ObjectID:   strconv.FormatInt(task.ID, 10),
			CreatedAt:  task.CreatedAt,
			UpdatedAt:  task.UpdatedAt,
			AgeHours:   item.AgeHours,
			Score:      item.AgeHours,
			Remark:     task.Remark,
			Reference:  saasAdminTaskPayload(task),
		})
	}

	for _, task := range input.BillingTasks {
		if !saasAdminCustomerSuccessBillingTaskOpen(task) {
			continue
		}
		reason := strings.TrimSpace(strings.Join([]string{
			task.PackageCode,
			task.ExternalOrderNo,
			task.Remark,
		}, " "))
		add(SaaSAdminOperationQueueItem{
			ID:         fmt.Sprintf("%s:%d", SaaSAdminOperationQueueSourceBillingFollowUp, task.OperationID),
			Source:     SaaSAdminOperationQueueSourceBillingFollowUp,
			Priority:   saasAdminOperationQueueDuePriority(task.DueState),
			TenantID:   task.TenantID,
			TenantName: task.TenantName,
			Owner:      task.Owner,
			Title:      "账单异常跟进：" + task.TenantName,
			Reason:     reason,
			NextAction: "处理账单对账异常",
			DueState:   task.DueState,
			Status:     task.Status,
			ObjectType: SaaSAdminOperationTargetBillingEvent,
			ObjectID:   strconv.FormatInt(task.BillingEventID, 10),
			CreatedAt:  task.CreatedAt,
			Score:      saasAdminOperationQueueDueScore(task.DueState),
			Remark:     task.Remark,
			Reference:  saasAdminBillingReconciliationFollowUpTaskPayload(task),
		})
	}

	for _, notification := range input.Notifications {
		source := ""
		priority := SaaSAdminCustomerSuccessPriorityNormal
		nextAction := ""
		switch notification.Status {
		case SaaSAlertNotificationStatusFailed:
			source = SaaSAdminOperationQueueSourceNotification
			priority = SaaSAdminCustomerSuccessPriorityMedium
			nextAction = "重试失败通知"
		case SaaSAlertNotificationStatusDead:
			source = SaaSAdminOperationQueueSourceNotification
			priority = SaaSAdminCustomerSuccessPriorityHigh
			nextAction = "复查耗尽通知并重试"
		case SaaSAlertNotificationStatusClosed:
			source = SaaSAdminOperationQueueSourceClosedNotification
			priority = SaaSAdminCustomerSuccessPriorityNormal
			nextAction = "复盘已关闭通知"
		default:
			continue
		}
		add(SaaSAdminOperationQueueItem{
			ID:         fmt.Sprintf("%s:%d", source, notification.ID),
			Source:     source,
			Priority:   priority,
			TenantID:   notification.TenantID,
			TenantName: tenantNames[notification.TenantID],
			Title:      "通知 outbox：" + notification.NotificationKey,
			Reason:     strings.TrimSpace(notification.LastError),
			NextAction: nextAction,
			DueState:   notification.Status,
			Status:     notification.Status,
			ObjectType: SaaSAdminOperationTargetAlertNotification,
			ObjectID:   strconv.FormatInt(notification.ID, 10),
			CreatedAt:  notification.CreatedAt,
			UpdatedAt:  notification.UpdatedAt,
			Score:      notification.Attempts,
			Remark:     notification.LastError,
			Reference:  saasAdminAlertNotificationPayload(notification),
		})
	}

	for _, health := range input.NotificationHealth {
		if health.HealthState != SaaSAdminNotificationHealthStateCritical && health.HealthState != SaaSAdminNotificationHealthStateWarning {
			continue
		}
		priority := SaaSAdminCustomerSuccessPriorityHigh
		if health.HealthState == SaaSAdminNotificationHealthStateCritical {
			priority = SaaSAdminCustomerSuccessPriorityCritical
		}
		tenantName := strings.TrimSpace(health.TenantName)
		if tenantName == "" {
			tenantName = "租户 #" + strconv.Itoa(health.TenantID)
		}
		add(SaaSAdminOperationQueueItem{
			ID:         fmt.Sprintf("%s:%d", SaaSAdminOperationQueueSourceNotificationHealth, health.TenantID),
			Source:     SaaSAdminOperationQueueSourceNotificationHealth,
			Priority:   priority,
			TenantID:   health.TenantID,
			TenantName: tenantName,
			Title:      "通知送达健康异常：" + tenantName,
			Reason:     strings.Join(health.Reasons, "；"),
			NextAction: health.SuggestedAction,
			DueState:   health.HealthState,
			Status:     health.HealthState,
			ObjectType: SaaSAdminOperationTargetNotificationHealth,
			ObjectID:   strconv.Itoa(health.TenantID),
			CreatedAt:  health.OldestPendingAt,
			UpdatedAt:  saasAdminFirstNonEmpty(health.LastFailureAt, health.LatestNotificationAt),
			Score:      saasAdminNotificationHealthQueueScore(health),
			Reference:  saasAdminNotificationHealthTenantPayload(health),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if rankI, rankJ := saasAdminCustomerSuccessPriorityRank(left.Priority), saasAdminCustomerSuccessPriorityRank(right.Priority); rankI != rankJ {
			return rankI < rankJ
		}
		if dueI, dueJ := saasAdminOperationQueueDueRank(left.DueState), saasAdminOperationQueueDueRank(right.DueState); dueI != dueJ {
			return dueI < dueJ
		}
		if left.AgeHours != right.AgeHours {
			return left.AgeHours > right.AgeHours
		}
		if left.UpdatedAt != right.UpdatedAt {
			return left.UpdatedAt > right.UpdatedAt
		}
		if left.CreatedAt != right.CreatedAt {
			return left.CreatedAt > right.CreatedAt
		}
		return left.ID < right.ID
	})
	summary.QueueCount = len(items)
	summary.TenantCount = len(tenantIDs)
	summary.SourceCount = len(sourceNames)
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	summary.ReturnedCount = len(items)
	return items, summary
}

func saasAdminNotificationHealthQueueScore(item SaaSAdminNotificationHealthTenant) int {
	return item.DeadCount*1000 + item.StalePendingCount*100 + item.FailedCount*10 + item.ReadyPendingCount
}

func saasAdminApplyOperationQueueAssignment(item *SaaSAdminOperationQueueItem, assignments map[string]SaaSAdminOperationQueueAssignment) {
	if item == nil || len(assignments) == 0 {
		return
	}
	assignment, ok := assignments[saasAdminOperationQueueItemAssignmentKey(*item)]
	if !ok || strings.TrimSpace(assignment.Owner) == "" || saasAdminOperationQueueAssignmentDueState(assignment, time.Now()) == SaaSAdminRiskFollowUpDueStateClosed {
		return
	}
	item.Owner = assignment.Owner
	if strings.TrimSpace(assignment.Remark) != "" {
		item.Remark = assignment.Remark
	}
	item.Assignment = &assignment
}

func saasAdminOperationQueueItemAssignmentKey(item SaaSAdminOperationQueueItem) string {
	return saasAdminOperationQueueAssignmentKey(item.Source, item.ObjectID)
}

func saasAdminOperationQueueAssignmentKey(source string, objectID string) string {
	source = strings.TrimSpace(source)
	objectID = strings.TrimSpace(objectID)
	if source == "" || objectID == "" {
		return ""
	}
	return source + ":" + objectID
}

func saasAdminOperationQueueAssignmentLogs(logs []SaaSAdminOperationLog) []SaaSAdminOperationLog {
	items := make([]SaaSAdminOperationLog, 0, len(logs))
	for _, log := range logs {
		if log.Action == SaaSAdminOperationActionOperationQueueAssign {
			items = append(items, log)
		}
	}
	return items
}

func saasAdminOperationQueueAssignmentFromLog(log SaaSAdminOperationLog) (SaaSAdminOperationQueueAssignment, bool) {
	payload, ok := saasAdminJSONPayload(log.AfterJSON).(map[string]any)
	if !ok {
		return SaaSAdminOperationQueueAssignment{}, false
	}
	assignment := SaaSAdminOperationQueueAssignment{
		TenantID:       log.TenantID,
		Source:         strings.TrimSpace(toString(payload["source"])),
		ObjectType:     strings.TrimSpace(toString(payload["objectType"])),
		ObjectID:       strings.TrimSpace(toString(payload["objectId"])),
		TargetName:     strings.TrimSpace(toString(payload["targetName"])),
		Owner:          strings.TrimSpace(toString(payload["owner"])),
		Status:         strings.TrimSpace(toString(payload["status"])),
		NextFollowUpAt: strings.TrimSpace(toString(payload["nextFollowUpAt"])),
		Remark:         strings.TrimSpace(toString(payload["remark"])),
		OperationID:    log.ID,
		ActorUserID:    log.ActorUserID,
		ActorTenantID:  log.ActorTenantID,
		AssignedAt:     log.CreatedAt,
	}
	if assignment.ObjectType == "" {
		assignment.ObjectType = log.TargetType
	}
	if assignment.ObjectID == "" {
		assignment.ObjectID = log.TargetID
	}
	if assignment.Source == "" || assignment.ObjectID == "" || assignment.Owner == "" {
		return SaaSAdminOperationQueueAssignment{}, false
	}
	if assignment.TargetName == "" {
		assignment.TargetName = log.TargetName
	}
	if assignment.Remark == "" {
		assignment.Remark = log.Remark
	}
	return assignment, true
}

func saasAdminBuildOperationQueueAssignmentReport(options SaaSAdminOperationQueueAssignmentOptions, logs []SaaSAdminOperationLog) SaaSAdminOperationQueueAssignmentReport {
	if options.DueState == "" {
		options.DueState = SaaSAdminRiskFollowUpDueStateAll
	}
	report := SaaSAdminOperationQueueAssignmentReport{
		Options:     options,
		Assignments: make([]SaaSAdminOperationQueueAssignment, 0, len(logs)),
	}
	ownerFilter := strings.ToLower(strings.TrimSpace(options.Owner))
	statusFilter := strings.ToLower(strings.TrimSpace(options.Status))
	dueStateFilter := strings.TrimSpace(options.DueState)
	objectTypeFilter := strings.ToLower(strings.TrimSpace(options.ObjectType))
	objectIDFilter := strings.TrimSpace(options.ObjectID)
	keywordFilter := strings.ToLower(strings.TrimSpace(options.Keyword))
	tenantIDs := map[int]struct{}{}
	owners := map[string]struct{}{}
	sources := map[string]struct{}{}
	assignments := make([]SaaSAdminOperationQueueAssignment, 0, len(logs))
	for _, log := range logs {
		assignment, ok := saasAdminOperationQueueAssignmentFromLog(log)
		if !ok {
			continue
		}
		assignments = append(assignments, assignment)
	}
	if options.CurrentOnly {
		latest := map[string]SaaSAdminOperationQueueAssignment{}
		for _, assignment := range assignments {
			key := saasAdminOperationQueueAssignmentKey(assignment.Source, assignment.ObjectID)
			if key == "" {
				continue
			}
			existing, exists := latest[key]
			if !exists || saasAdminOperationQueueAssignmentNewer(assignment, existing) {
				latest[key] = assignment
			}
		}
		assignments = assignments[:0]
		for _, assignment := range latest {
			assignments = append(assignments, assignment)
		}
	}
	sort.SliceStable(assignments, func(i, j int) bool {
		if assignments[i].OperationID != assignments[j].OperationID {
			return assignments[i].OperationID > assignments[j].OperationID
		}
		return assignments[i].AssignedAt > assignments[j].AssignedAt
	})
	for _, assignment := range assignments {
		assignment.DueState = saasAdminOperationQueueAssignmentDueState(assignment, time.Now())
		if options.Source != "" && assignment.Source != options.Source {
			continue
		}
		if ownerFilter != "" && !strings.Contains(strings.ToLower(assignment.Owner), ownerFilter) {
			continue
		}
		if statusFilter != "" && strings.ToLower(assignment.Status) != statusFilter {
			continue
		}
		if dueStateFilter != "" && dueStateFilter != SaaSAdminRiskFollowUpDueStateAll && assignment.DueState != dueStateFilter {
			continue
		}
		if objectTypeFilter != "" && strings.ToLower(assignment.ObjectType) != objectTypeFilter {
			continue
		}
		if objectIDFilter != "" && assignment.ObjectID != objectIDFilter {
			continue
		}
		if options.TenantID > 0 && assignment.TenantID != options.TenantID {
			continue
		}
		if keywordFilter != "" && !saasAdminOperationQueueAssignmentContains(assignment, keywordFilter) {
			continue
		}
		report.Summary.AssignmentCount++
		switch assignment.Source {
		case SaaSAdminOperationQueueSourceTaskSLA:
			report.Summary.TaskSLACount++
		case SaaSAdminOperationQueueSourceNotification:
			report.Summary.NotificationCount++
		case SaaSAdminOperationQueueSourceClosedNotification:
			report.Summary.ClosedNotificationCount++
		case SaaSAdminOperationQueueSourceNotificationHealth:
			report.Summary.NotificationHealthCount++
		}
		switch assignment.DueState {
		case SaaSAdminRiskFollowUpDueStateOverdue:
			report.Summary.OverdueCount++
		case SaaSAdminRiskFollowUpDueStateDueSoon:
			report.Summary.DueSoonCount++
		case SaaSAdminRiskFollowUpDueStateFuture:
			report.Summary.FutureCount++
		case SaaSAdminRiskFollowUpDueStateNoDate:
			report.Summary.NoDateCount++
		case SaaSAdminRiskFollowUpDueStateClosed:
			report.Summary.ClosedCount++
		}
		if assignment.NextFollowUpAt != "" && assignment.DueState != SaaSAdminRiskFollowUpDueStateClosed && (report.Summary.NextFollowUpAt == "" || assignment.NextFollowUpAt < report.Summary.NextFollowUpAt) {
			report.Summary.NextFollowUpAt = assignment.NextFollowUpAt
		}
		if assignment.TenantID > 0 {
			tenantIDs[assignment.TenantID] = struct{}{}
		}
		if strings.TrimSpace(assignment.Owner) != "" {
			owners[assignment.Owner] = struct{}{}
		}
		if strings.TrimSpace(assignment.Source) != "" {
			sources[assignment.Source] = struct{}{}
		}
		if options.Limit <= 0 || len(report.Assignments) < options.Limit {
			report.Assignments = append(report.Assignments, assignment)
		}
	}
	report.Summary.TenantCount = len(tenantIDs)
	report.Summary.OwnerCount = len(owners)
	report.Summary.SourceCount = len(sources)
	report.ReturnedCount = len(report.Assignments)
	return report
}

func saasAdminOperationQueueAssignmentContains(assignment SaaSAdminOperationQueueAssignment, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		strconv.Itoa(assignment.TenantID),
		assignment.Source,
		assignment.ObjectType,
		assignment.ObjectID,
		assignment.TargetName,
		assignment.Owner,
		assignment.Status,
		assignment.DueState,
		assignment.NextFollowUpAt,
		assignment.Remark,
		strconv.FormatInt(assignment.OperationID, 10),
		strconv.Itoa(assignment.ActorUserID),
		strconv.Itoa(assignment.ActorTenantID),
		assignment.AssignedAt,
	}, "\n"))
	return strings.Contains(haystack, keyword)
}

func saasAdminOperationQueueAssignmentNewer(candidate SaaSAdminOperationQueueAssignment, existing SaaSAdminOperationQueueAssignment) bool {
	if candidate.OperationID > 0 && existing.OperationID > 0 && candidate.OperationID != existing.OperationID {
		return candidate.OperationID > existing.OperationID
	}
	return candidate.AssignedAt > existing.AssignedAt
}

func saasAdminOperationQueueAssignmentDueState(assignment SaaSAdminOperationQueueAssignment, now time.Time) string {
	status := strings.ToLower(strings.TrimSpace(assignment.Status))
	switch status {
	case SaaSAdminRiskFollowUpStatusResolved, SaaSAdminRiskFollowUpStatusIgnored, "closed", "done":
		return SaaSAdminRiskFollowUpDueStateClosed
	}
	dueAt, ok := saasAdminParseRiskFollowUpTime(assignment.NextFollowUpAt)
	if !ok {
		return SaaSAdminRiskFollowUpDueStateNoDate
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case dueAt.Before(today):
		return SaaSAdminRiskFollowUpDueStateOverdue
	case !dueAt.After(today.AddDate(0, 0, 7)):
		return SaaSAdminRiskFollowUpDueStateDueSoon
	default:
		return SaaSAdminRiskFollowUpDueStateFuture
	}
}

func saasAdminBuildOperationQueueOwnerSummaries(items []SaaSAdminOperationQueueItem, topTenantLimit int, topItemLimit int) []SaaSAdminOperationQueueOwnerSummary {
	type tenantAccumulator struct {
		item    SaaSAdminOperationQueueOwnerTenant
		sources map[string]struct{}
	}
	type ownerAccumulator struct {
		item      SaaSAdminOperationQueueOwnerSummary
		tenants   map[int]*tenantAccumulator
		sourceSet map[string]struct{}
	}
	byOwner := map[string]*ownerAccumulator{}
	for _, queueItem := range items {
		ownerName := strings.TrimSpace(queueItem.Owner)
		if ownerName == "" {
			ownerName = "未分配"
		}
		acc := byOwner[ownerName]
		if acc == nil {
			acc = &ownerAccumulator{
				item: SaaSAdminOperationQueueOwnerSummary{
					Owner: ownerName,
				},
				tenants:   map[int]*tenantAccumulator{},
				sourceSet: map[string]struct{}{},
			}
			byOwner[ownerName] = acc
		}
		acc.item.QueueCount++
		switch queueItem.Priority {
		case SaaSAdminCustomerSuccessPriorityCritical:
			acc.item.CriticalCount++
		case SaaSAdminCustomerSuccessPriorityHigh:
			acc.item.HighCount++
		case SaaSAdminCustomerSuccessPriorityMedium:
			acc.item.MediumCount++
		case SaaSAdminCustomerSuccessPriorityNormal:
			acc.item.NormalCount++
		}
		switch queueItem.Source {
		case SaaSAdminOperationQueueSourceCustomerSuccess:
			acc.item.CustomerSuccessCount++
		case SaaSAdminOperationQueueSourceTaskSLA:
			acc.item.TaskSLACount++
		case SaaSAdminOperationQueueSourceBillingFollowUp:
			acc.item.BillingFollowUpCount++
		case SaaSAdminOperationQueueSourceNotification:
			acc.item.NotificationCount++
		case SaaSAdminOperationQueueSourceClosedNotification:
			acc.item.ClosedNotificationCount++
		case SaaSAdminOperationQueueSourceNotificationHealth:
			acc.item.NotificationHealthCount++
		}
		if strings.TrimSpace(queueItem.Owner) == "" || queueItem.Owner == "未分配" {
			acc.item.UnassignedCount++
		}
		if queueItem.AgeHours > acc.item.MaxAgeHours {
			acc.item.MaxAgeHours = queueItem.AgeHours
		}
		if strings.TrimSpace(queueItem.Source) != "" {
			acc.sourceSet[queueItem.Source] = struct{}{}
		}
		if topItemLimit <= 0 || len(acc.item.TopItems) < topItemLimit {
			acc.item.TopItems = append(acc.item.TopItems, queueItem)
		}
		if queueItem.TenantID > 0 {
			tenant := acc.tenants[queueItem.TenantID]
			if tenant == nil {
				tenant = &tenantAccumulator{
					item: SaaSAdminOperationQueueOwnerTenant{
						TenantID:   queueItem.TenantID,
						TenantName: queueItem.TenantName,
					},
					sources: map[string]struct{}{},
				}
				acc.tenants[queueItem.TenantID] = tenant
			}
			tenant.item.QueueCount++
			if queueItem.Priority == SaaSAdminCustomerSuccessPriorityCritical {
				tenant.item.CriticalCount++
			}
			if queueItem.Priority == SaaSAdminCustomerSuccessPriorityHigh {
				tenant.item.HighCount++
			}
			if strings.TrimSpace(queueItem.Source) != "" {
				tenant.sources[queueItem.Source] = struct{}{}
			}
		}
	}

	owners := make([]SaaSAdminOperationQueueOwnerSummary, 0, len(byOwner))
	for _, acc := range byOwner {
		acc.item.SourceCount = len(acc.sourceSet)
		acc.item.TenantCount = len(acc.tenants)
		tenants := make([]SaaSAdminOperationQueueOwnerTenant, 0, len(acc.tenants))
		for _, tenant := range acc.tenants {
			tenant.item.Sources = saasAdminSortedStringSet(tenant.sources)
			tenants = append(tenants, tenant.item)
		}
		sort.SliceStable(tenants, func(i, j int) bool {
			left := tenants[i]
			right := tenants[j]
			if left.CriticalCount != right.CriticalCount {
				return left.CriticalCount > right.CriticalCount
			}
			if left.HighCount != right.HighCount {
				return left.HighCount > right.HighCount
			}
			if left.QueueCount != right.QueueCount {
				return left.QueueCount > right.QueueCount
			}
			if left.TenantName != right.TenantName {
				return left.TenantName < right.TenantName
			}
			return left.TenantID < right.TenantID
		})
		if topTenantLimit > 0 && len(tenants) > topTenantLimit {
			tenants = tenants[:topTenantLimit]
		}
		acc.item.TopTenants = tenants
		owners = append(owners, acc.item)
	}
	sort.SliceStable(owners, func(i, j int) bool {
		left := owners[i]
		right := owners[j]
		if left.CriticalCount != right.CriticalCount {
			return left.CriticalCount > right.CriticalCount
		}
		if left.HighCount != right.HighCount {
			return left.HighCount > right.HighCount
		}
		if left.QueueCount != right.QueueCount {
			return left.QueueCount > right.QueueCount
		}
		if left.MaxAgeHours != right.MaxAgeHours {
			return left.MaxAgeHours > right.MaxAgeHours
		}
		return left.Owner < right.Owner
	})
	return owners
}

func saasAdminSortedStringSet(items map[string]struct{}) []string {
	values := make([]string, 0, len(items))
	for value := range items {
		if strings.TrimSpace(value) != "" {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}

func saasAdminOperationQueueOwnerTenantCSV(items []SaaSAdminOperationQueueOwnerTenant) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.TenantName)
		if name == "" {
			name = strconv.Itoa(item.TenantID)
		}
		parts = append(parts, fmt.Sprintf("%s#%d:%d", name, item.TenantID, item.QueueCount))
	}
	return strings.Join(parts, "；")
}

func saasAdminOperationQueueOwnerTopItemCSV(items []SaaSAdminOperationQueueItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = item.ID
		}
		parts = append(parts, fmt.Sprintf("%s/%s/%s", item.Source, item.Priority, title))
	}
	return strings.Join(parts, "；")
}

func saasAdminOperationQueueTaskSLAPriority(item SaaSAdminTaskSLAItem) string {
	switch {
	case item.SLAStatus == "overdue",
		item.Task.Status == SaaSAdminTaskStatusBlocked,
		item.Task.Status == SaaSAdminTaskStatusFailed:
		return SaaSAdminCustomerSuccessPriorityCritical
	case item.SLAStatus == "warning":
		return SaaSAdminCustomerSuccessPriorityHigh
	case item.SLAStatus == "unknown":
		return SaaSAdminCustomerSuccessPriorityMedium
	default:
		return SaaSAdminCustomerSuccessPriorityMedium
	}
}

func saasAdminOperationQueueTaskSLANextAction(item SaaSAdminTaskSLAItem) string {
	if item.Task.Status == SaaSAdminTaskStatusBlocked {
		return "解除阻断后重新应用任务"
	}
	if item.Task.Status == SaaSAdminTaskStatusFailed {
		return "复查失败原因并重置任务"
	}
	if item.SLAStatus == "overdue" {
		return "立即处理逾期运营任务"
	}
	if item.SLAStatus == "warning" {
		return "优先跟进预警运营任务"
	}
	return "确认任务创建时间并跟进"
}

func saasAdminOperationQueueDuePriority(state string) string {
	switch state {
	case SaaSAdminRiskFollowUpDueStateOverdue:
		return SaaSAdminCustomerSuccessPriorityCritical
	case SaaSAdminRiskFollowUpDueStateDueSoon, "warning":
		return SaaSAdminCustomerSuccessPriorityHigh
	case SaaSAdminRiskFollowUpDueStateNoDate, "unknown":
		return SaaSAdminCustomerSuccessPriorityMedium
	default:
		return SaaSAdminCustomerSuccessPriorityMedium
	}
}

func saasAdminOperationQueueDueScore(state string) int {
	switch state {
	case SaaSAdminRiskFollowUpDueStateOverdue:
		return 100
	case SaaSAdminRiskFollowUpDueStateDueSoon, "warning":
		return 80
	case SaaSAdminRiskFollowUpDueStateNoDate, "unknown":
		return 60
	default:
		return 40
	}
}

func saasAdminOperationQueueDueRank(state string) int {
	switch state {
	case SaaSAdminRiskFollowUpDueStateOverdue:
		return 0
	case "blocked":
		return 1
	case SaaSAdminRiskFollowUpDueStateDueSoon, "warning":
		return 2
	case SaaSAdminRiskFollowUpDueStateNoDate, "unknown":
		return 3
	case SaaSAdminRiskFollowUpDueStateFuture:
		return 4
	case SaaSAlertNotificationStatusClosed:
		return 5
	case SaaSAdminCustomerSuccessPriorityNormal, "fresh":
		return 6
	default:
		return 7
	}
}

func saasAdminOperationQueueItemMatchesKeyword(item SaaSAdminOperationQueueItem, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	fields := []string{
		item.ID,
		item.Source,
		item.Priority,
		strconv.Itoa(item.TenantID),
		item.TenantName,
		item.Owner,
		item.Title,
		item.Reason,
		item.NextAction,
		item.DueState,
		item.Status,
		item.ObjectType,
		item.ObjectID,
		item.Remark,
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), keyword) {
			return true
		}
	}
	return false
}

func saasAdminCountOperationQueueItem(summary *SaaSAdminOperationQueueSummary, item SaaSAdminOperationQueueItem) {
	if summary == nil {
		return
	}
	switch item.Priority {
	case SaaSAdminCustomerSuccessPriorityCritical:
		summary.CriticalCount++
	case SaaSAdminCustomerSuccessPriorityHigh:
		summary.HighCount++
	case SaaSAdminCustomerSuccessPriorityMedium:
		summary.MediumCount++
	case SaaSAdminCustomerSuccessPriorityNormal:
		summary.NormalCount++
	}
	switch item.Source {
	case SaaSAdminOperationQueueSourceCustomerSuccess:
		summary.CustomerSuccessCount++
	case SaaSAdminOperationQueueSourceTaskSLA:
		summary.TaskSLACount++
	case SaaSAdminOperationQueueSourceBillingFollowUp:
		summary.BillingFollowUpCount++
	case SaaSAdminOperationQueueSourceNotification:
		summary.NotificationCount++
	case SaaSAdminOperationQueueSourceClosedNotification:
		summary.ClosedNotificationCount++
	case SaaSAdminOperationQueueSourceNotificationHealth:
		summary.NotificationHealthCount++
	}
	if strings.TrimSpace(item.Owner) == "" || item.Owner == "未分配" {
		summary.UnassignedCount++
	}
}

func saasAdminBuildCustomerSuccessQueue(input saasAdminCustomerSuccessBuildInput) ([]SaaSAdminCustomerSuccessQueueItem, SaaSAdminCustomerSuccessSummary) {
	riskTasksByTenant := saasAdminCustomerSuccessRiskTasksByTenant(input.RiskTasks)
	billingTasksByTenant := saasAdminCustomerSuccessBillingTasksByTenant(input.BillingTasks)
	adminTaskSummaryByTenant := saasAdminCustomerSuccessAdminTaskSummaryByTenant(input.AdminTasks)
	notificationsByTenant := saasAdminCustomerSuccessNotificationsByTenant(input.Notifications)
	ownerFilter := strings.ToLower(strings.TrimSpace(input.Options.Owner))
	priorityFilter := strings.TrimSpace(input.Options.Priority)
	now := time.Now()

	items := make([]SaaSAdminCustomerSuccessQueueItem, 0, len(input.RiskReport.Items))
	summary := SaaSAdminCustomerSuccessSummary{
		TenantCount:     input.RiskReport.Summary.EvaluatedTenantCount,
		RiskTenantCount: input.RiskReport.Summary.RiskTenantCount,
	}
	for _, risk := range input.RiskReport.Items {
		tenantID := risk.Tenant.TenantID
		if tenantID <= 0 || tenantID == input.PlatformAdminTenantID {
			continue
		}
		riskTask := riskTasksByTenant[tenantID]
		if riskTask.OperationID <= 0 && saasAdminCustomerSuccessRiskSnapshotOpen(risk.FollowUp) {
			riskTask = saasAdminRiskFollowUpTask(risk.FollowUp, now)
		}
		item := SaaSAdminCustomerSuccessQueueItem{
			Tenant:                     risk.Tenant,
			Risk:                       risk,
			RiskFollowUp:               riskTask,
			BillingFollowUps:           billingTasksByTenant[tenantID],
			BillingFollowUpCount:       len(billingTasksByTenant[tenantID]),
			AdminTaskSummary:           adminTaskSummaryByTenant[tenantID],
			FailedNotificationCount:    notificationsByTenant[tenantID].Failed,
			DeadNotificationCount:      notificationsByTenant[tenantID].Dead,
			RetryableNotificationCount: notificationsByTenant[tenantID].Failed + notificationsByTenant[tenantID].Dead,
		}
		item.HealthScore = saasAdminCustomerSuccessScore(item)
		item.DueState = saasAdminCustomerSuccessDueState(item)
		item.Priority = saasAdminCustomerSuccessPriority(item)
		item.Owner = saasAdminCustomerSuccessOwner(item)
		item.NextAction = saasAdminCustomerSuccessNextAction(item)
		item.Reasons = saasAdminCustomerSuccessReasons(item)
		if priorityFilter == "" && item.Priority == SaaSAdminCustomerSuccessPriorityNormal {
			continue
		}
		if priorityFilter != "" && priorityFilter != "all" && item.Priority != priorityFilter {
			continue
		}
		if ownerFilter != "" && !strings.Contains(strings.ToLower(item.Owner), ownerFilter) {
			continue
		}
		items = append(items, item)
		saasAdminCountCustomerSuccessQueueItem(&summary, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		leftPriority := saasAdminCustomerSuccessPriorityRank(left.Priority)
		rightPriority := saasAdminCustomerSuccessPriorityRank(right.Priority)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		if left.HealthScore != right.HealthScore {
			return left.HealthScore > right.HealthScore
		}
		leftDue := saasAdminCustomerSuccessDueStateRank(left.DueState)
		rightDue := saasAdminCustomerSuccessDueStateRank(right.DueState)
		if leftDue != rightDue {
			return leftDue < rightDue
		}
		return left.Tenant.TenantID < right.Tenant.TenantID
	})
	summary.QueueCount = len(items)
	if input.Options.Limit > 0 && len(items) > input.Options.Limit {
		items = items[:input.Options.Limit]
	}
	summary.ReturnedCount = len(items)
	return items, summary
}

func saasAdminBuildCustomerSuccessOwnerSummaries(items []SaaSAdminCustomerSuccessQueueItem, topTenantLimit int) []SaaSAdminCustomerSuccessOwnerSummary {
	if topTenantLimit <= 0 {
		topTenantLimit = 3
	}
	byOwner := map[string]*SaaSAdminCustomerSuccessOwnerSummary{}
	for _, item := range items {
		ownerName := strings.TrimSpace(item.Owner)
		if ownerName == "" {
			ownerName = "未分配"
		}
		summary := byOwner[ownerName]
		if summary == nil {
			summary = &SaaSAdminCustomerSuccessOwnerSummary{Owner: ownerName}
			byOwner[ownerName] = summary
		}
		summary.TenantCount++
		switch item.Priority {
		case SaaSAdminCustomerSuccessPriorityCritical:
			summary.CriticalCount++
		case SaaSAdminCustomerSuccessPriorityHigh:
			summary.HighCount++
		case SaaSAdminCustomerSuccessPriorityMedium:
			summary.MediumCount++
		case SaaSAdminCustomerSuccessPriorityNormal:
			summary.NormalCount++
		}
		switch item.DueState {
		case SaaSAdminRiskFollowUpDueStateOverdue:
			summary.OverdueCount++
		case SaaSAdminRiskFollowUpDueStateDueSoon:
			summary.DueSoonCount++
		case "blocked":
			summary.BlockedCount++
		}
		summary.BillingFollowUpCount += item.BillingFollowUpCount
		summary.ActionableTaskCount += item.AdminTaskSummary.ActionableCount
		summary.RetryableNotificationCount += item.RetryableNotificationCount
		summary.FailedNotificationCount += item.FailedNotificationCount
		summary.DeadNotificationCount += item.DeadNotificationCount
		summary.TotalHealthScore += item.HealthScore
		if item.HealthScore > summary.MaxHealthScore {
			summary.MaxHealthScore = item.HealthScore
		}
		if next := saasAdminCustomerSuccessEarliestNextFollowUpAt(item); next != "" && (summary.NextFollowUpAt == "" || next < summary.NextFollowUpAt) {
			summary.NextFollowUpAt = next
		}
		summary.TopTenants = append(summary.TopTenants, item)
	}
	owners := make([]SaaSAdminCustomerSuccessOwnerSummary, 0, len(byOwner))
	for _, summary := range byOwner {
		if summary.TenantCount > 0 {
			summary.AverageHealthScore = int(float64(summary.TotalHealthScore) / float64(summary.TenantCount))
		}
		if len(summary.TopTenants) > topTenantLimit {
			summary.TopTenants = summary.TopTenants[:topTenantLimit]
		}
		owners = append(owners, *summary)
	}
	sort.SliceStable(owners, func(i, j int) bool {
		left := owners[i]
		right := owners[j]
		if left.CriticalCount != right.CriticalCount {
			return left.CriticalCount > right.CriticalCount
		}
		if left.OverdueCount != right.OverdueCount {
			return left.OverdueCount > right.OverdueCount
		}
		if left.HighCount != right.HighCount {
			return left.HighCount > right.HighCount
		}
		if left.RetryableNotificationCount != right.RetryableNotificationCount {
			return left.RetryableNotificationCount > right.RetryableNotificationCount
		}
		if left.TenantCount != right.TenantCount {
			return left.TenantCount > right.TenantCount
		}
		if left.MaxHealthScore != right.MaxHealthScore {
			return left.MaxHealthScore > right.MaxHealthScore
		}
		return left.Owner < right.Owner
	})
	return owners
}

func saasAdminCustomerSuccessEarliestNextFollowUpAt(item SaaSAdminCustomerSuccessQueueItem) string {
	next := strings.TrimSpace(item.RiskFollowUp.NextFollowUpAt)
	for _, task := range item.BillingFollowUps {
		candidate := strings.TrimSpace(task.NextFollowUpAt)
		if candidate != "" && (next == "" || candidate < next) {
			next = candidate
		}
	}
	return next
}

func saasAdminCustomerSuccessRiskTasksByTenant(tasks []SaaSAdminRiskFollowUpTask) map[int]SaaSAdminRiskFollowUpTask {
	byTenant := map[int]SaaSAdminRiskFollowUpTask{}
	for _, task := range tasks {
		if task.TenantID <= 0 || !saasAdminCustomerSuccessRiskTaskOpen(task) {
			continue
		}
		if existing, ok := byTenant[task.TenantID]; ok && saasAdminCustomerSuccessRiskTaskBefore(existing, task) {
			continue
		}
		byTenant[task.TenantID] = task
	}
	return byTenant
}

func saasAdminCustomerSuccessBillingTasksByTenant(tasks []SaaSAdminBillingReconciliationFollowUpTask) map[int][]SaaSAdminBillingReconciliationFollowUpTask {
	byTenant := map[int][]SaaSAdminBillingReconciliationFollowUpTask{}
	for _, task := range tasks {
		if task.TenantID <= 0 || !saasAdminCustomerSuccessBillingTaskOpen(task) {
			continue
		}
		byTenant[task.TenantID] = append(byTenant[task.TenantID], task)
	}
	return byTenant
}

func saasAdminCustomerSuccessAdminTaskSummaryByTenant(tasks []SaaSAdminTask) map[int]SaaSAdminTaskSummary {
	byTenant := map[int]SaaSAdminTaskSummary{}
	for _, task := range tasks {
		if task.TenantID <= 0 || !saasAdminCustomerSuccessTaskActionable(task.Status) {
			continue
		}
		summary := byTenant[task.TenantID]
		summary.TaskCount++
		summary.ActionableCount++
		switch task.Status {
		case SaaSAdminTaskStatusPending:
			summary.PendingCount++
		case SaaSAdminTaskStatusBlocked:
			summary.BlockedCount++
		case SaaSAdminTaskStatusFailed:
			summary.FailedCount++
		}
		switch task.TaskType {
		case SaaSAdminTaskTypePackageSync:
			summary.PackageSyncCount++
		case SaaSAdminTaskTypeTenantRenewal:
			summary.TenantRenewalCount++
		case SaaSAdminTaskTypeTenantProvision:
			summary.TenantProvisionCount++
		}
		summary.TenantCount = 1
		if task.ActorUserID > 0 {
			summary.ActorUserCount = 1
		}
		byTenant[task.TenantID] = summary
	}
	return byTenant
}

func saasAdminCustomerSuccessNotificationsByTenant(notifications []SaaSAlertNotification) map[int]saasAdminCustomerSuccessNotificationCounts {
	byTenant := map[int]saasAdminCustomerSuccessNotificationCounts{}
	for _, notification := range notifications {
		if notification.TenantID <= 0 {
			continue
		}
		counts := byTenant[notification.TenantID]
		switch notification.Status {
		case SaaSAlertNotificationStatusFailed:
			counts.Failed++
		case SaaSAlertNotificationStatusDead:
			counts.Dead++
		}
		byTenant[notification.TenantID] = counts
	}
	return byTenant
}

func saasAdminCustomerSuccessScore(item SaaSAdminCustomerSuccessQueueItem) int {
	score := item.Risk.RiskScore
	if item.RiskFollowUp.OperationID > 0 {
		switch item.RiskFollowUp.DueState {
		case SaaSAdminRiskFollowUpDueStateOverdue:
			score += 30
		case SaaSAdminRiskFollowUpDueStateDueSoon:
			score += 15
		case SaaSAdminRiskFollowUpDueStateNoDate:
			score += 5
		case SaaSAdminRiskFollowUpDueStateFuture:
			score += 3
		}
		if item.RiskFollowUp.Status == SaaSAdminRiskFollowUpStatusRenewalPending {
			score += 10
		}
	}
	billingScore := len(item.BillingFollowUps) * 5
	for _, task := range item.BillingFollowUps {
		switch task.DueState {
		case SaaSAdminRiskFollowUpDueStateOverdue:
			billingScore += 25
		case SaaSAdminRiskFollowUpDueStateDueSoon:
			billingScore += 12
		case SaaSAdminRiskFollowUpDueStateNoDate:
			billingScore += 5
		}
	}
	if billingScore > 45 {
		billingScore = 45
	}
	score += billingScore
	score += item.AdminTaskSummary.BlockedCount*20 + item.AdminTaskSummary.FailedCount*20 + item.AdminTaskSummary.PendingCount*8
	notificationScore := item.RetryableNotificationCount * 5
	if notificationScore > 25 {
		notificationScore = 25
	}
	score += notificationScore
	if score > 0 && saasAdminCustomerSuccessOwner(item) == "未分配" {
		score += 5
	}
	return score
}

func saasAdminCustomerSuccessPriority(item SaaSAdminCustomerSuccessQueueItem) string {
	switch {
	case item.HealthScore >= 100 ||
		item.Risk.RiskLevel == SaaSAdminCustomerSuccessPriorityCritical ||
		item.DueState == SaaSAdminRiskFollowUpDueStateOverdue ||
		item.AdminTaskSummary.BlockedCount > 0 ||
		item.AdminTaskSummary.FailedCount > 0:
		return SaaSAdminCustomerSuccessPriorityCritical
	case item.HealthScore >= 70 ||
		item.Risk.RiskLevel == SaaSAdminCustomerSuccessPriorityHigh ||
		item.BillingFollowUpCount > 0 ||
		item.RetryableNotificationCount >= 3:
		return SaaSAdminCustomerSuccessPriorityHigh
	case item.HealthScore >= 30 ||
		item.Risk.RiskLevel == SaaSAdminCustomerSuccessPriorityMedium ||
		item.AdminTaskSummary.PendingCount > 0 ||
		item.RetryableNotificationCount > 0:
		return SaaSAdminCustomerSuccessPriorityMedium
	default:
		return SaaSAdminCustomerSuccessPriorityNormal
	}
}

func saasAdminCustomerSuccessDueState(item SaaSAdminCustomerSuccessQueueItem) string {
	if item.RiskFollowUp.DueState == SaaSAdminRiskFollowUpDueStateOverdue {
		return SaaSAdminRiskFollowUpDueStateOverdue
	}
	for _, task := range item.BillingFollowUps {
		if task.DueState == SaaSAdminRiskFollowUpDueStateOverdue {
			return SaaSAdminRiskFollowUpDueStateOverdue
		}
	}
	if item.RiskFollowUp.DueState == SaaSAdminRiskFollowUpDueStateDueSoon {
		return SaaSAdminRiskFollowUpDueStateDueSoon
	}
	for _, task := range item.BillingFollowUps {
		if task.DueState == SaaSAdminRiskFollowUpDueStateDueSoon {
			return SaaSAdminRiskFollowUpDueStateDueSoon
		}
	}
	if item.AdminTaskSummary.BlockedCount > 0 || item.AdminTaskSummary.FailedCount > 0 {
		return "blocked"
	}
	if item.RiskFollowUp.DueState == SaaSAdminRiskFollowUpDueStateNoDate {
		return SaaSAdminRiskFollowUpDueStateNoDate
	}
	for _, task := range item.BillingFollowUps {
		if task.DueState == SaaSAdminRiskFollowUpDueStateNoDate {
			return SaaSAdminRiskFollowUpDueStateNoDate
		}
	}
	return SaaSAdminCustomerSuccessPriorityNormal
}

func saasAdminCustomerSuccessOwner(item SaaSAdminCustomerSuccessQueueItem) string {
	if owner := strings.TrimSpace(item.RiskFollowUp.Owner); owner != "" {
		return owner
	}
	for _, task := range item.BillingFollowUps {
		if owner := strings.TrimSpace(task.Owner); owner != "" {
			return owner
		}
	}
	return "未分配"
}

func saasAdminCustomerSuccessNextAction(item SaaSAdminCustomerSuccessQueueItem) string {
	if item.RiskFollowUp.DueState == SaaSAdminRiskFollowUpDueStateOverdue {
		return "立即跟进逾期风险任务"
	}
	for _, task := range item.BillingFollowUps {
		if task.DueState == SaaSAdminRiskFollowUpDueStateOverdue {
			return "立即处理逾期账单跟进"
		}
	}
	if item.AdminTaskSummary.BlockedCount > 0 {
		return "处理阻断运营任务"
	}
	if item.AdminTaskSummary.FailedCount > 0 {
		return "重置或复查失败运营任务"
	}
	if strings.TrimSpace(item.Risk.SuggestedAction) != "" && item.Risk.RiskScore > 0 {
		return item.Risk.SuggestedAction
	}
	if item.RetryableNotificationCount > 0 {
		return "重试失败通知"
	}
	return "保持观察"
}

func saasAdminCustomerSuccessReasons(item SaaSAdminCustomerSuccessQueueItem) []string {
	reasons := make([]string, 0, len(item.Risk.Reasons)+6)
	seen := map[string]struct{}{}
	add := func(reason string) {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			return
		}
		key := strings.ToLower(reason)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		reasons = append(reasons, reason)
	}
	for _, reason := range item.Risk.Reasons {
		add(reason)
	}
	if item.RiskFollowUp.OperationID > 0 {
		switch item.RiskFollowUp.DueState {
		case SaaSAdminRiskFollowUpDueStateOverdue:
			add("风险跟进已逾期")
		case SaaSAdminRiskFollowUpDueStateDueSoon:
			add("风险跟进 7 天内到期")
		case SaaSAdminRiskFollowUpDueStateNoDate:
			add("风险跟进未设置下次时间")
		}
	}
	if item.BillingFollowUpCount > 0 {
		add("有未关闭账单跟进")
	}
	if item.AdminTaskSummary.BlockedCount > 0 {
		add("运营任务已阻断")
	}
	if item.AdminTaskSummary.FailedCount > 0 {
		add("运营任务失败")
	}
	if item.AdminTaskSummary.PendingCount > 0 {
		add("有待应用运营任务")
	}
	if item.RetryableNotificationCount > 0 {
		add("有失败或耗尽通知")
	}
	if item.HealthScore > 0 && len(reasons) == 0 {
		add("存在待处理信号")
	}
	return reasons
}

func saasAdminCountCustomerSuccessQueueItem(summary *SaaSAdminCustomerSuccessSummary, item SaaSAdminCustomerSuccessQueueItem) {
	if summary == nil {
		return
	}
	switch item.Priority {
	case SaaSAdminCustomerSuccessPriorityCritical:
		summary.CriticalCount++
	case SaaSAdminCustomerSuccessPriorityHigh:
		summary.HighCount++
	case SaaSAdminCustomerSuccessPriorityMedium:
		summary.MediumCount++
	case SaaSAdminCustomerSuccessPriorityNormal:
		summary.NormalCount++
	}
	switch item.DueState {
	case SaaSAdminRiskFollowUpDueStateOverdue:
		summary.OverdueCount++
	case SaaSAdminRiskFollowUpDueStateDueSoon:
		summary.DueSoonCount++
	}
	if item.Owner == "未分配" {
		summary.UnassignedCount++
	}
	summary.BillingFollowUpCount += item.BillingFollowUpCount
	summary.ActionableTaskCount += item.AdminTaskSummary.ActionableCount
	summary.RetryableNotificationCount += item.RetryableNotificationCount
}

func saasAdminCustomerSuccessRiskSnapshotOpen(snapshot SaaSAdminRiskFollowUpSnapshot) bool {
	return snapshot.OperationID > 0 && snapshot.Status != SaaSAdminRiskFollowUpStatusResolved && snapshot.Status != SaaSAdminRiskFollowUpStatusIgnored
}

func saasAdminCustomerSuccessRiskTaskOpen(task SaaSAdminRiskFollowUpTask) bool {
	return task.OperationID > 0 && task.Status != SaaSAdminRiskFollowUpStatusResolved && task.Status != SaaSAdminRiskFollowUpStatusIgnored
}

func saasAdminCustomerSuccessBillingTaskOpen(task SaaSAdminBillingReconciliationFollowUpTask) bool {
	return task.OperationID > 0 && task.Status != SaaSAdminRiskFollowUpStatusResolved && task.Status != SaaSAdminRiskFollowUpStatusIgnored
}

func saasAdminCustomerSuccessTaskActionable(status string) bool {
	switch status {
	case SaaSAdminTaskStatusPending, SaaSAdminTaskStatusBlocked, SaaSAdminTaskStatusFailed:
		return true
	default:
		return false
	}
}

func saasAdminCustomerSuccessRiskTaskBefore(existing SaaSAdminRiskFollowUpTask, candidate SaaSAdminRiskFollowUpTask) bool {
	existingRank := saasAdminRiskFollowUpDueStateRank(existing.DueState)
	candidateRank := saasAdminRiskFollowUpDueStateRank(candidate.DueState)
	if existingRank != candidateRank {
		return existingRank < candidateRank
	}
	if existing.NextFollowUpAt != "" && candidate.NextFollowUpAt != "" && existing.NextFollowUpAt != candidate.NextFollowUpAt {
		return existing.NextFollowUpAt < candidate.NextFollowUpAt
	}
	if existing.NextFollowUpAt != candidate.NextFollowUpAt {
		return existing.NextFollowUpAt != ""
	}
	if existing.CreatedAt != candidate.CreatedAt {
		return existing.CreatedAt > candidate.CreatedAt
	}
	return existing.OperationID < candidate.OperationID
}

func saasAdminCustomerSuccessPriorityRank(priority string) int {
	switch priority {
	case SaaSAdminCustomerSuccessPriorityCritical:
		return 0
	case SaaSAdminCustomerSuccessPriorityHigh:
		return 1
	case SaaSAdminCustomerSuccessPriorityMedium:
		return 2
	case SaaSAdminCustomerSuccessPriorityNormal:
		return 3
	default:
		return 4
	}
}

func saasAdminCustomerSuccessDueStateRank(state string) int {
	switch state {
	case SaaSAdminRiskFollowUpDueStateOverdue:
		return 0
	case SaaSAdminRiskFollowUpDueStateDueSoon:
		return 1
	case "blocked":
		return 2
	case SaaSAdminRiskFollowUpDueStateNoDate:
		return 3
	case SaaSAdminCustomerSuccessPriorityNormal:
		return 4
	default:
		return 5
	}
}

func saasAdminDailyReportSummary(overview SaaSAdminOverview, riskReport SaaSAdminRiskReport, riskTaskSummary SaaSAdminRiskFollowUpTaskSummary, owners []SaaSAdminRiskFollowUpOwnerSummary, taskSLAReport SaaSAdminTaskSLAReport, openAlertCount int, notificationSummary SaaSAdminDailyNotificationSummary, queueAssignmentSummary SaaSAdminOperationQueueAssignmentSummary, billingSummary SaaSAdminDailyBillingSummary, operations []SaaSAdminOperationLog) SaaSAdminDailyReportSummary {
	summary := SaaSAdminDailyReportSummary{
		TenantCount:                         overview.Summary.TenantCount,
		ActiveTenantPackageCount:            overview.Summary.ActiveTenantPackageCount,
		UserCount:                           overview.Summary.UserCount,
		CorpCount:                           overview.Summary.CorpCount,
		ExpiringSoonTenantCount:             overview.Summary.ExpiringSoonTenantCount,
		ExpiredTenantCount:                  overview.Summary.ExpiredTenantCount,
		EvaluatedRiskTenantCount:            riskReport.Summary.EvaluatedTenantCount,
		RiskTenantCount:                     riskReport.Summary.RiskTenantCount,
		CriticalRiskTenantCount:             riskReport.Summary.CriticalRiskTenantCount,
		HighRiskTenantCount:                 riskReport.Summary.HighRiskTenantCount,
		OpenRiskFollowUpCount:               riskTaskSummary.TotalCount - riskTaskSummary.ClosedCount,
		OverdueRiskFollowUpCount:            riskTaskSummary.OverdueCount,
		DueSoonRiskFollowUpCount:            riskTaskSummary.DueSoonCount,
		RiskFollowUpOwnerCount:              len(owners),
		OpenAlertCount:                      openAlertCount,
		TaskSLAActiveCount:                  taskSLAReport.Summary.TaskCount,
		TaskSLAWarningCount:                 taskSLAReport.Summary.WarningCount,
		TaskSLAOverdueCount:                 taskSLAReport.Summary.OverdueCount,
		TaskSLAOwnerCount:                   taskSLAReport.OwnerCount,
		TaskSLAMaxAgeHours:                  taskSLAReport.Summary.MaxAgeHours,
		PendingNotificationCount:            notificationSummary.PendingCount,
		FailedNotificationCount:             notificationSummary.FailedCount,
		DeadNotificationCount:               notificationSummary.DeadCount,
		ClosedNotificationCount:             notificationSummary.ClosedCount,
		RetryableNotificationCount:          notificationSummary.RetryableCount,
		WindowQueueAssignmentCount:          queueAssignmentSummary.AssignmentCount,
		WindowTaskSLAAssignCount:            queueAssignmentSummary.TaskSLACount,
		WindowNotificationAssignCount:       queueAssignmentSummary.NotificationCount,
		WindowClosedNotificationAssignCount: queueAssignmentSummary.ClosedNotificationCount,
		WindowNotificationHealthAssignCount: queueAssignmentSummary.NotificationHealthCount,
		WindowOperationCount:                len(operations),
		WindowBillingEventCount:             billingSummary.EventCount,
		WindowBillingAmountCents:            billingSummary.AmountCents,
		WindowRenewalCount:                  billingSummary.RenewalCount,
		WindowRefundCount:                   billingSummary.RefundCount,
		WindowGrossBillingAmountCents:       billingSummary.GrossAmountCents,
		WindowRefundAmountCents:             billingSummary.RefundAmountCents,
	}
	if summary.OpenRiskFollowUpCount < 0 {
		summary.OpenRiskFollowUpCount = 0
	}
	for _, operation := range operations {
		switch operation.Action {
		case "tenant.risk.follow_up":
			summary.WindowRiskFollowUpCount++
		case "tenant.alert.resolve":
			summary.WindowAlertResolveCount++
		case "tenant.notification.retry":
			summary.WindowNotificationRetryCount++
		case SaaSAdminOperationActionNotificationClose:
			summary.WindowNotificationCloseCount++
		}
	}
	return summary
}

func saasAdminDailyNotificationSummary(notifications []SaaSAlertNotification) SaaSAdminDailyNotificationSummary {
	summary := SaaSAdminDailyNotificationSummary{}
	for _, notification := range notifications {
		switch notification.Status {
		case SaaSAlertNotificationStatusPending:
			summary.PendingCount++
		case SaaSAlertNotificationStatusFailed:
			summary.FailedCount++
			summary.RetryableCount++
		case SaaSAlertNotificationStatusDead:
			summary.DeadCount++
			summary.RetryableCount++
		case SaaSAlertNotificationStatusClosed:
			summary.ClosedCount++
		case SaaSAlertNotificationStatusSuppressed:
			summary.SuppressedCount++
		case SaaSAlertNotificationStatusDelivered:
			summary.DeliveredCount++
		}
	}
	return summary
}

func saasAdminDailyBillingSummary(events []SaaSAdminBillingEvent) SaaSAdminDailyBillingSummary {
	summary := SaaSAdminDailyBillingSummary{EventCount: len(events)}
	for _, event := range events {
		summary.AmountCents += saasAdminBillingSignedAmount(event)
		if event.EventType == "renewal" {
			summary.RenewalCount++
		}
		if event.EventType == "refund" {
			summary.RefundCount++
			summary.RefundAmountCents += event.AmountCents
		} else {
			summary.GrossAmountCents += event.AmountCents
		}
	}
	return summary
}

func saasAdminDailyOperationActionSummary(operations []SaaSAdminOperationLog) []SaaSAdminDailyOperationActionSummary {
	byAction := map[string]int{}
	for _, operation := range operations {
		action := strings.TrimSpace(operation.Action)
		if action == "" {
			action = "unknown"
		}
		byAction[action]++
	}
	summaries := make([]SaaSAdminDailyOperationActionSummary, 0, len(byAction))
	for action, count := range byAction {
		summaries = append(summaries, SaaSAdminDailyOperationActionSummary{Action: action, Count: count})
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].Count != summaries[j].Count {
			return summaries[i].Count > summaries[j].Count
		}
		return summaries[i].Action < summaries[j].Action
	})
	return summaries
}

func saasAdminFilterOperationLogsByWindow(logs []SaaSAdminOperationLog, start time.Time, end time.Time) []SaaSAdminOperationLog {
	items := make([]SaaSAdminOperationLog, 0, len(logs))
	for _, item := range logs {
		if saasAdminTimeInWindow(item.CreatedAt, start, end) {
			items = append(items, item)
		}
	}
	return items
}

func saasAdminFilterBillingEventsByWindow(events []SaaSAdminBillingEvent, start time.Time, end time.Time) []SaaSAdminBillingEvent {
	items := make([]SaaSAdminBillingEvent, 0, len(events))
	for _, item := range events {
		if saasAdminTimeInWindow(item.CreatedAt, start, end) {
			items = append(items, item)
		}
	}
	return items
}

func saasAdminTimeInWindow(raw string, start time.Time, end time.Time) bool {
	parsed, ok := saasAdminParseRiskFollowUpTime(raw)
	if !ok {
		return false
	}
	return !parsed.Before(start) && parsed.Before(end)
}

func saasAdminRetryableNotifications(notifications []SaaSAlertNotification) []SaaSAlertNotification {
	items := make([]SaaSAlertNotification, 0, len(notifications))
	for _, item := range notifications {
		if item.Status == SaaSAlertNotificationStatusFailed || item.Status == SaaSAlertNotificationStatusDead {
			items = append(items, item)
		}
	}
	return items
}

func saasAdminClosedNotificationsInWindow(notifications []SaaSAlertNotification, start time.Time, end time.Time) []SaaSAlertNotification {
	items := make([]SaaSAlertNotification, 0)
	for _, item := range notifications {
		if item.Status != SaaSAlertNotificationStatusClosed {
			continue
		}
		if !saasAdminTimeInWindow(item.UpdatedAt, start, end) {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].UpdatedAt != items[j].UpdatedAt {
			return items[i].UpdatedAt > items[j].UpdatedAt
		}
		return items[i].ID > items[j].ID
	})
	return items
}

func saasAdminLimitRiskTenants(items []SaaSAdminRiskTenant, limit int) []SaaSAdminRiskTenant {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func saasAdminLimitRiskFollowUpOwnerSummaries(items []SaaSAdminRiskFollowUpOwnerSummary, limit int) []SaaSAdminRiskFollowUpOwnerSummary {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func saasAdminLimitNotifications(items []SaaSAlertNotification, limit int) []SaaSAlertNotification {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func saasAdminLimitOperationLogs(items []SaaSAdminOperationLog, limit int) []SaaSAdminOperationLog {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func saasAdminLimitBillingEvents(items []SaaSAdminBillingEvent, limit int) []SaaSAdminBillingEvent {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func saasAdminCountRiskTenant(summary *SaaSAdminRiskSummary, item SaaSAdminRiskTenant) {
	if item.RiskLevel != "normal" {
		summary.RiskTenantCount++
	}
	switch item.RiskLevel {
	case "critical":
		summary.CriticalRiskTenantCount++
	case "high":
		summary.HighRiskTenantCount++
	case "medium":
		summary.MediumRiskTenantCount++
	}
	if item.Tenant.TenantStatus == 2 {
		summary.DisabledTenantCount++
	}
	if item.Tenant.Expired {
		summary.ExpiredTenantCount++
	}
	if item.Tenant.ExpiringSoon {
		summary.ExpiringSoonTenantCount++
	}
	if strings.TrimSpace(item.Tenant.PackageCode) == "" {
		summary.NoPackageTenantCount++
	}
	if item.Tenant.OpenAlertCount > 0 {
		summary.OpenAlertTenantCount++
	}
	var hasHighUsage bool
	var hasExceeded bool
	var hasWarning bool
	for _, metric := range item.TopUsageMetrics {
		hasHighUsage = true
		if metric.Limit > 0 && metric.Current > metric.Limit || metric.Status == "exceeded" {
			hasExceeded = true
			continue
		}
		hasWarning = true
	}
	if hasHighUsage {
		summary.HighUsageTenantCount++
	}
	if hasExceeded {
		summary.ExceededUsageTenantCount++
	} else if hasWarning {
		summary.WarningUsageTenantCount++
	}
}

func saasAdminBuildRiskTenant(tenant SaaSAdminTenantOverview, metrics []SaaSAdminUsageMetric, highUsageRatio float64) SaaSAdminRiskTenant {
	var score int
	reasons := []string{}
	if tenant.TenantStatus == 2 {
		score += 60
		reasons = append(reasons, "租户已停用")
	}
	if strings.TrimSpace(tenant.PackageCode) == "" {
		score += 45
		reasons = append(reasons, "未开通套餐")
	}
	if tenant.Expired {
		score += 60
		reasons = append(reasons, "套餐已到期")
	} else if tenant.ExpiringSoon {
		score += 25
		reasons = append(reasons, "套餐即将到期")
	}
	if tenant.OpenAlertCount > 0 {
		score += 20 + minInt(tenant.OpenAlertCount*5, 30)
		reasons = append(reasons, "存在 "+strconv.Itoa(tenant.OpenAlertCount)+" 条打开告警")
	}
	topMetrics, exceededCount, warningCount, highestRatio := saasAdminRiskTopMetrics(metrics, highUsageRatio)
	if highestRatio <= 0 {
		highestRatio = tenant.MaxUsageRatio
	}
	if len(topMetrics) == 0 && tenant.MaxUsageLimit > 0 && tenant.MaxUsageRatio >= highUsageRatio {
		warningCount++
		topMetrics = append(topMetrics, SaaSAdminUsageMetric{
			Metric:         tenant.MaxUsageMetric,
			PeriodKey:      SaaSAlertPeriodLifetime,
			Current:        tenant.MaxUsageCurrent,
			Limit:          tenant.MaxUsageLimit,
			Remaining:      SaaSAdminUsageMetricRemaining(tenant.MaxUsageCurrent, tenant.MaxUsageLimit),
			UsageRatio:     tenant.MaxUsageRatio,
			Status:         SaaSAdminUsageMetricStatus(tenant.MaxUsageCurrent, tenant.MaxUsageLimit, tenant.OpenAlertCount),
			OpenAlertCount: tenant.OpenAlertCount,
		})
	}
	if exceededCount > 0 {
		score += 45
		reasons = append(reasons, strconv.Itoa(exceededCount)+" 项额度已超限")
	} else if warningCount > 0 {
		score += 25
		reasons = append(reasons, strconv.Itoa(warningCount)+" 项额度接近上限")
	} else if tenant.MaxUsageLimit > 0 && tenant.MaxUsageRatio >= highUsageRatio {
		score += 20
		reasons = append(reasons, "最高用量达到 "+strconv.Itoa(int(highUsageRatio*100))+"% 阈值")
	}
	level := "normal"
	switch {
	case score >= 90:
		level = "critical"
	case score >= 60:
		level = "high"
	case score >= 25:
		level = "medium"
	}
	return SaaSAdminRiskTenant{
		Tenant:          tenant,
		RiskLevel:       level,
		RiskScore:       score,
		Reasons:         reasons,
		SuggestedAction: saasAdminRiskSuggestedAction(tenant, exceededCount, warningCount, highUsageRatio),
		HighUsageRatio:  highestRatio,
		TopUsageMetrics: topMetrics,
	}
}

func saasAdminRiskTopMetrics(metrics []SaaSAdminUsageMetric, highUsageRatio float64) ([]SaaSAdminUsageMetric, int, int, float64) {
	candidates := make([]SaaSAdminUsageMetric, 0, len(metrics))
	var exceededCount int
	var warningCount int
	var highestRatio float64
	for _, metric := range metrics {
		if metric.UsageRatio > highestRatio {
			highestRatio = metric.UsageRatio
		}
		exceeded := metric.Limit > 0 && metric.Current > metric.Limit || metric.Status == "exceeded"
		warning := metric.OpenAlertCount > 0 || metric.Status == "warning" || metric.UsageRatio >= highUsageRatio
		if exceeded {
			exceededCount++
		} else if warning {
			warningCount++
		}
		if exceeded || warning {
			candidates = append(candidates, metric)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		leftSeverity := saasAdminRiskMetricSeverity(left, highUsageRatio)
		rightSeverity := saasAdminRiskMetricSeverity(right, highUsageRatio)
		if leftSeverity != rightSeverity {
			return leftSeverity > rightSeverity
		}
		if left.OpenAlertCount != right.OpenAlertCount {
			return left.OpenAlertCount > right.OpenAlertCount
		}
		if left.UsageRatio != right.UsageRatio {
			return left.UsageRatio > right.UsageRatio
		}
		return left.Metric < right.Metric
	})
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return candidates, exceededCount, warningCount, highestRatio
}

func saasAdminRiskMetricSeverity(metric SaaSAdminUsageMetric, highUsageRatio float64) int {
	if metric.Limit > 0 && metric.Current > metric.Limit || metric.Status == "exceeded" {
		return 4
	}
	if metric.OpenAlertCount > 0 {
		return 3
	}
	if metric.Status == "warning" || metric.UsageRatio >= highUsageRatio {
		return 2
	}
	return 1
}

func saasAdminRiskSuggestedAction(tenant SaaSAdminTenantOverview, exceededCount int, warningCount int, highUsageRatio float64) string {
	switch {
	case tenant.TenantStatus == 2:
		return "确认停用原因，必要时恢复或归档租户"
	case tenant.Expired:
		return "联系客户续费，或按规则停用服务"
	case tenant.OpenAlertCount > 0:
		return "处理打开告警，评估扩容或清理用量"
	case strings.TrimSpace(tenant.PackageCode) == "":
		return "补齐套餐配置并刷新用量额度"
	case exceededCount > 0:
		return "升级套餐或降低超额资源"
	case warningCount > 0 || tenant.MaxUsageRatio >= highUsageRatio && tenant.MaxUsageLimit > 0:
		return "提前沟通扩容，避免触发额度告警"
	case tenant.ExpiringSoon:
		return "跟进续费排期"
	default:
		return "保持观察"
	}
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func saasAdminUsageSummary(metrics []SaaSAdminUsageMetric) SaaSAdminUsageSummary {
	var summary SaaSAdminUsageSummary
	for _, metric := range metrics {
		summary.MetricCount++
		if metric.Unlimited {
			summary.UnlimitedMetricCount++
		} else {
			summary.LimitedMetricCount++
		}
		if metric.OpenAlertCount > 0 {
			summary.OpenAlertMetricCount++
		}
		switch metric.Status {
		case "exceeded":
			summary.ExceededMetricCount++
		case "warning":
			summary.WarningMetricCount++
		}
		if metric.UsageRatio > summary.HighestUsageRatio {
			summary.HighestUsageRatio = metric.UsageRatio
			summary.HighestUsageMetric = metric.Metric
		}
	}
	return summary
}

func saasUsageRatio(current int64, limit int64) float64 {
	if current <= 0 || limit <= 0 {
		return 0
	}
	return float64(current) / float64(limit)
}

func SaaSAdminMetricUsageRatio(current int64, limit int64) float64 {
	return saasUsageRatio(current, limit)
}

func SaaSAdminUsageMetricRemaining(current int64, limit int64) int64 {
	if limit <= 0 || current >= limit {
		return 0
	}
	return limit - current
}

func SaaSAdminUsageMetricStatus(current int64, limit int64, openAlertCount int) string {
	if limit <= 0 {
		return "unlimited"
	}
	if current > limit {
		return "exceeded"
	}
	if openAlertCount > 0 || saasUsageRatio(current, limit) >= 0.8 {
		return "warning"
	}
	return "normal"
}
