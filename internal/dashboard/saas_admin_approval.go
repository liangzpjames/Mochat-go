package dashboard

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/authjwt"
	"jiyi/mochat-go/internal/dashboardadmin"
	"jiyi/mochat-go/internal/identitysecurity"
	"jiyi/mochat-go/internal/saasbackup"
	"jiyi/mochat-go/internal/saascompliance"
)

const (
	SaaSAdminApprovalActionTenantDisable              = "tenant.disable"
	SaaSAdminApprovalActionTenantEnable               = "tenant.enable"
	SaaSAdminApprovalActionTenantProvision            = "tenant.provision"
	SaaSAdminApprovalActionTenantRenewal              = "tenant.renewal"
	SaaSAdminApprovalActionSubscriptionTransition     = "tenant.subscription.transition"
	SaaSAdminApprovalActionPackageUpsert              = "package.upsert"
	SaaSAdminApprovalActionTenantPackageUpdate        = "tenant.package.update"
	SaaSAdminApprovalActionTenantDomainCreate         = "tenant.domain.create"
	SaaSAdminApprovalActionTenantDomainCommand        = "tenant.domain.command"
	SaaSAdminApprovalActionPaymentOrderCreate         = "payment.order.create"
	SaaSAdminApprovalActionPaymentRefundCreate        = "payment.refund.create"
	SaaSAdminApprovalActionInvoiceIssue               = "billing.invoice.issue"
	SaaSAdminApprovalActionPaymentSettlementClose     = "payment.settlement.close"
	SaaSAdminApprovalActionPaymentSettlementReopen    = "payment.settlement.reopen"
	SaaSAdminApprovalActionPaymentSettlementResolve   = "payment.settlement.resolve"
	SaaSAdminApprovalActionAccessRoleSave             = "access.role.save"
	SaaSAdminApprovalActionAccessAssignmentSave       = "access.assignment.save"
	SaaSAdminApprovalActionTenantDataErase            = "tenant.data.erase"
	SaaSAdminApprovalActionReleaseCandidateGate       = "release.candidate.gate"
	SaaSAdminApprovalActionApprovalPolicyUpdate       = "approval.policy.update"
	SaaSAdminApprovalActionBackupPolicyUpdate         = "backup.policy.update"
	SaaSAdminApprovalActionBackupRetentionCleanup     = "backup.retention.cleanup"
	SaaSAdminApprovalActionCompliancePolicyUpdate     = "compliance.policy.update"
	SaaSAdminApprovalActionComplianceExportDelete     = "compliance.export.delete"
	SaaSAdminApprovalActionComplianceLegalHoldRelease = "compliance.legal_hold.release"
	SaaSAdminApprovalActionIdentityPolicyUpdate       = "identity.policy.update"
	SaaSAdminApprovalActionIdentityMFAReset           = "identity.mfa.reset"
	SaaSAdminApprovalActionServiceAccountCreate       = "service_account.create"
	SaaSAdminApprovalActionServiceAccountUpdate       = "service_account.update"
	SaaSAdminApprovalActionServiceAccountKeyRotate    = "service_account.key.rotate"
	SaaSAdminApprovalActionServiceAccountKeyRevoke    = "service_account.key.revoke"

	SaaSAdminApprovalActionDashboardTenantProvision   = dashboardadmin.ApprovalActionTenantProvision
	SaaSAdminApprovalActionDashboardActivationResend  = dashboardadmin.ApprovalActionActivationResend
	SaaSAdminApprovalActionDashboardSuperAdminReplace = dashboardadmin.ApprovalActionSuperAdminReplace
	SaaSAdminApprovalActionDashboardSuperAdminStatus  = dashboardadmin.ApprovalActionSuperAdminStatus

	SaaSAdminApprovalStatusAll       = "all"
	SaaSAdminApprovalStatusPending   = "pending"
	SaaSAdminApprovalStatusApproved  = "approved"
	SaaSAdminApprovalStatusRejected  = "rejected"
	SaaSAdminApprovalStatusCanceled  = "canceled"
	SaaSAdminApprovalStatusExpired   = "expired"
	SaaSAdminApprovalStatusExecuting = "executing"
	SaaSAdminApprovalStatusExecuted  = "executed"

	SaaSAdminApprovalDecisionApprove = "approve"
	SaaSAdminApprovalDecisionReject  = "reject"

	SaaSAdminApprovalRiskAll      = "all"
	SaaSAdminApprovalRiskHigh     = "high"
	SaaSAdminApprovalRiskCritical = "critical"

	SaaSAdminOperationActionApprovalRequest        = "saas.admin.approval.request"
	SaaSAdminOperationActionApprovalDecision       = "saas.admin.approval.decision"
	SaaSAdminOperationActionApprovalCancel         = "saas.admin.approval.cancel"
	SaaSAdminOperationActionApprovalExecuteStart   = "saas.admin.approval.execute_start"
	SaaSAdminOperationActionApprovalExecuteFinish  = "saas.admin.approval.execute_finish"
	SaaSAdminOperationActionApprovalPolicySave     = "saas.admin.approval.policy.save"
	SaaSAdminOperationActionApprovalDelegationSave = "saas.admin.approval.delegation.save"
	SaaSAdminOperationActionApprovalReminder       = "saas.admin.approval.reminder"
	SaaSAdminOperationTargetApproval               = "saas_admin_approval"
	SaaSAdminOperationTargetApprovalPolicy         = "saas_admin_approval_policy"
	SaaSAdminOperationTargetApprovalDelegation     = "saas_admin_approval_delegation"

	saasAdminApprovalActionTimeout = 2 * time.Minute
	saasAdminApprovalFinishTimeout = 15 * time.Second
)

type SaaSAdminApprovalPolicy struct {
	ActionType           string
	Name                 string
	RiskLevel            string
	RequiredPermission   string
	TargetType           string
	Description          string
	Enabled              bool
	AmountThresholdCents int64
	RequiredApprovals    int
	SLAMinutes           int
	ReminderMinutes      int
	ExpiryHours          int
	Version              int
	UpdatedBy            int
	UpdatedAt            string
}

type SaaSAdminApproval struct {
	ID                    int64
	RequestNo             string
	ActionType            string
	RiskLevel             string
	Status                string
	RequiredPermission    string
	PolicyVersion         int
	RequiredApprovals     int
	ApprovalCount         int
	ReminderMinutes       int
	TargetType            string
	TargetID              string
	TargetName            string
	RequesterUserID       int
	RequesterTenantID     int
	RequesterName         string
	IdempotencyKey        string
	RequestSHA256         string
	RequestJSON           string
	Reason                string
	ReviewerUserID        int
	ReviewerName          string
	ReviewedAt            string
	DecisionReason        string
	SLADueAt              string
	SLADueAtValue         time.Time
	NextReminderAt        string
	NextReminderAtValue   time.Time
	LastRemindedAt        string
	ReminderCount         int
	ExecutionUserID       int
	ExecutionUserName     string
	ExecutionStartedAt    string
	ExecutionStartedValue time.Time
	EffectAppliedAt       string
	EffectAppliedAtValue  time.Time
	EffectOperationID     int64
	ExecutedAt            string
	ExecutionAttempts     int
	ResultJSON            string
	LastError             string
	ExpiresAt             string
	ExpiresAtValue        time.Time
	Version               int
	CreatedAt             string
	UpdatedAt             string
}

type SaaSAdminApprovalEvent struct {
	ID            int64
	ApprovalID    int64
	EventType     string
	FromStatus    string
	ToStatus      string
	ActorUserID   int
	ActorTenantID int
	ActorName     string
	Reason        string
	ContextJSON   string
	CreatedAt     string
}

type SaaSAdminApprovalOptions struct {
	ApprovalID  int64
	Status      string
	ActionType  string
	RiskLevel   string
	RequesterID int
	ReviewerID  int
	ExecutionID int
	Keyword     string
	Limit       int
}

type SaaSAdminApprovalSummary struct {
	Total     int
	Pending   int
	Approved  int
	Rejected  int
	Canceled  int
	Expired   int
	Executing int
	Executed  int
	Critical  int
}

type SaaSAdminApprovalReport struct {
	Summary SaaSAdminApprovalSummary
	Items   []SaaSAdminApproval
}

type SaaSAdminApprovalCreate struct {
	RequestNo          string
	ActionType         string
	RiskLevel          string
	RequiredPermission string
	TargetType         string
	TargetID           string
	TargetName         string
	RequesterUserID    int
	RequesterTenantID  int
	IdempotencyKey     string
	RequestSHA256      string
	RequestJSON        string
	Reason             string
	ExpiresAt          time.Time
	PolicyVersion      int
	RequiredApprovals  int
	ReminderMinutes    int
	SLADueAt           time.Time
	NextReminderAt     time.Time
}

type SaaSAdminApprovalCreateResult struct {
	Approval    SaaSAdminApproval
	OperationID int64
	Idempotent  bool
}

type SaaSAdminApprovalDecision struct {
	ApprovalID          int64
	Decision            string
	Reason              string
	ExpectedVersion     int
	ActorUserID         int
	ActorTenantID       int
	DelegatedFromUserID int
}

type SaaSAdminApprovalDecisionRecord struct {
	ID                  int64
	ApprovalID          int64
	ReviewerUserID      int
	ReviewerTenantID    int
	ReviewerName        string
	DelegatedFromUserID int
	DelegatedFromName   string
	Decision            string
	Reason              string
	CreatedAt           string
}

type SaaSAdminApprovalPolicyUpdate struct {
	ActionType               string
	Enabled                  bool
	AmountThresholdCents     int64
	RequiredApprovals        int
	SLAMinutes               int
	ReminderMinutes          int
	ExpiryHours              int
	ExpectedVersion          int
	ActorUserID              int
	ActorTenantID            int
	ApprovalExecutionID      int64
	ApprovalExecutionVersion int
}

type SaaSAdminApprovalPolicyUpdateResult struct {
	Policy      SaaSAdminApprovalPolicy
	OperationID int64
}

type SaaSAdminApprovalDelegation struct {
	ID              int64
	DelegatorUserID int
	DelegatorName   string
	DelegateUserID  int
	DelegateName    string
	StartsAt        string
	StartsAtValue   time.Time
	EndsAt          string
	EndsAtValue     time.Time
	Status          int
	Reason          string
	Version         int
	CreatedBy       int
	UpdatedBy       int
	CreatedAt       string
	UpdatedAt       string
}

type SaaSAdminApprovalDelegationOptions struct {
	DelegatorUserID int
	DelegateUserID  int
	ActiveOnly      bool
	Limit           int
}

type SaaSAdminApprovalDelegationUpsert struct {
	ID               int64
	DelegatorUserID  int
	DelegateUserID   int
	StartsAt         time.Time
	EndsAt           time.Time
	Status           int
	Reason           string
	ExpectedVersion  int
	ActorUserID      int
	ActorTenantID    int
	PlatformTenantID int
}

type SaaSAdminApprovalDelegationUpsertResult struct {
	Delegation  SaaSAdminApprovalDelegation
	OperationID int64
	Created     bool
}

type SaaSAdminApprovalReminderResult struct {
	Scanned          int
	Enqueued         int
	Skipped          int
	NotificationKeys []string
}

type SaaSAdminApprovalReminderCreate struct {
	Limit            int
	MaxAttempts      int
	ActorUserID      int
	ActorTenantID    int
	PlatformTenantID int
}

type SaaSAdminApprovalCancel struct {
	ApprovalID      int64
	Reason          string
	ExpectedVersion int
	ActorUserID     int
	ActorTenantID   int
}

type SaaSAdminApprovalExecutionStart struct {
	ApprovalID      int64
	ExpectedVersion int
	ActorUserID     int
	ActorTenantID   int
}

type SaaSAdminApprovalExecutionFinish struct {
	ApprovalID      int64
	ExpectedVersion int
	ActorUserID     int
	ActorTenantID   int
	Success         bool
	ResultJSON      string
	ErrorMessage    string
}

type SaaSAdminApprovalStore interface {
	SaaSAdminApprovals(context.Context, SaaSAdminApprovalOptions) (SaaSAdminApprovalReport, error)
	SaaSAdminApprovalEvents(context.Context, int64, int) ([]SaaSAdminApprovalEvent, error)
	CreateSaaSAdminApproval(context.Context, SaaSAdminApprovalCreate) (SaaSAdminApprovalCreateResult, error)
	DecideSaaSAdminApproval(context.Context, SaaSAdminApprovalDecision) (SaaSAdminApproval, error)
	CancelSaaSAdminApproval(context.Context, SaaSAdminApprovalCancel) (SaaSAdminApproval, error)
	BeginSaaSAdminApprovalExecution(context.Context, SaaSAdminApprovalExecutionStart) (SaaSAdminApproval, error)
	FinishSaaSAdminApprovalExecution(context.Context, SaaSAdminApprovalExecutionFinish) (SaaSAdminApproval, error)
}

type SaaSAdminApprovalGovernanceStore interface {
	SaaSAdminApprovalPolicies(context.Context) ([]SaaSAdminApprovalPolicy, error)
	SaaSAdminApprovalPolicy(context.Context, string) (SaaSAdminApprovalPolicy, bool, error)
	UpdateSaaSAdminApprovalPolicy(context.Context, SaaSAdminApprovalPolicyUpdate) (SaaSAdminApprovalPolicyUpdateResult, error)
	SaaSAdminApprovalDecisions(context.Context, int64, int) ([]SaaSAdminApprovalDecisionRecord, error)
	SaaSAdminApprovalDelegations(context.Context, SaaSAdminApprovalDelegationOptions) ([]SaaSAdminApprovalDelegation, error)
	UpsertSaaSAdminApprovalDelegation(context.Context, SaaSAdminApprovalDelegationUpsert) (SaaSAdminApprovalDelegationUpsertResult, error)
	ResolveSaaSAdminApprovalDelegation(context.Context, int, int, time.Time) (SaaSAdminApprovalDelegation, bool, error)
	CreateSaaSAdminApprovalReminders(context.Context, SaaSAdminApprovalReminderCreate) (SaaSAdminApprovalReminderResult, error)
}

func SaaSAdminApprovalPolicies() []SaaSAdminApprovalPolicy {
	policies := []SaaSAdminApprovalPolicy{
		{ActionType: SaaSAdminApprovalActionDashboardTenantProvision, Name: "开通 Dashboard 管理员租户", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionTenantsManage, TargetType: "dashboard_tenant", Description: "双人复核后原子创建 SaaS 租户、绑定和首个 Dashboard 管理员", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionDashboardActivationResend, Name: "重发 Dashboard 激活", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionTenantsManage, TargetType: "dashboard_identity_activation", Description: "双人复核后重新生成一次性激活凭据", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionDashboardSuperAdminReplace, Name: "替换 Dashboard 超级管理员", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionTenantsManage, TargetType: "dashboard_superadmin", Description: "新主体激活后经双人复核原子替换旧超管", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionDashboardSuperAdminStatus, Name: "变更 Dashboard 超级管理员状态", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionTenantsManage, TargetType: "dashboard_superadmin", Description: "双人复核后停用或恢复 SaaS 治理的 Dashboard 超管", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionTenantDisable, Name: "停用业务租户", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionTenantsManage, TargetType: "tenant", Description: "停用后会立即阻断该租户新登录和旧 token 访问", Enabled: true, RequiredApprovals: 2, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, Version: 1},
		{ActionType: SaaSAdminApprovalActionTenantEnable, Name: "启用业务租户", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionTenantsManage, TargetType: "tenant", Description: "冻结停用租户和订阅快照，双人复核后恢复登录、访问与订阅状态", Enabled: true, RequiredApprovals: 2, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, Version: 1},
		{ActionType: SaaSAdminApprovalActionTenantProvision, Name: "开通业务租户", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionTenantsManage, TargetType: "tenant", Description: "冻结管理员凭据哈希、运营任务版本、套餐定义版本和开户预览，双人复核后原子创建租户、管理员、权限、套餐与订阅", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionTenantRenewal, Name: "租户续费生效", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionTenantsManage, TargetType: "saas_tenant_renewal", Description: "冻结租户状态、套餐绑定、目标套餐、订阅和运营任务版本，双人复核后原子提交权益、订阅、账单、任务与审计", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionSubscriptionTransition, Name: "迁移租户订阅状态", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionFinanceManage, TargetType: SaaSAdminOperationTargetSubscription, Description: "冻结租户状态、订阅版本与完整状态快照，双人复核后原子提交订阅、事件、操作审计和审批效果", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionPackageUpsert, Name: "变更平台套餐定义", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionTenantsManage, TargetType: "saas_package", Description: "冻结套餐版本、状态、说明、全部额度和租户影响，双人复核后创建或更新平台套餐定义", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionTenantPackageUpdate, Name: "变更租户套餐权益", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionTenantsManage, TargetType: "saas_tenant_package", Description: "冻结租户状态、当前分配版本、目标套餐版本、到期时间和降额影响，双人复核后变更租户权益", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionTenantDomainCreate, Name: "新增租户域名绑定", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionDomainsManage, TargetType: SaaSAdminOperationTargetTenantDomain, Description: "冻结租户与标准化域名，双人复核后生成 DNS 校验令牌并原子创建待验证绑定", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionTenantDomainCommand, Name: "变更租户域名路由", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionDomainsManage, TargetType: SaaSAdminOperationTargetTenantDomain, Description: "冻结目标域名和租户完整路由快照，双人复核后切换主域名、启停、轮换校验令牌或删除绑定", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionPaymentOrderCreate, Name: "创建收款订单", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionFinanceManage, TargetType: SaaSAdminOperationTargetPaymentOrder, Description: "冻结正常租户、套餐定义版本、完整权益额度、服务期和应收金额，双人复核后原子创建收款订单", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionPaymentRefundCreate, Name: "创建退款", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionFinanceManage, TargetType: SaaSAdminOperationTargetPaymentRefund, Description: "预占退款金额并可能联动暂停或取消订阅权益", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 24, Version: 1},
		{ActionType: SaaSAdminApprovalActionInvoiceIssue, Name: "正式开具发票单据", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionFinanceManage, TargetType: SaaSAdminOperationTargetInvoiceDocument, Description: "冻结蓝票或红票、支付订单版本和金额台账，双人复核后原子开具并提交财务审计", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionPaymentSettlementClose, Name: "结算关账", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionFinanceManage, TargetType: SaaSAdminOperationTargetSettlementBatch, Description: "冻结渠道结算批次金额、差异和版本，双人复核后原子关账并阻断后续对账修改", Enabled: true, RequiredApprovals: 2, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, Version: 1},
		{ActionType: SaaSAdminApprovalActionPaymentSettlementReopen, Name: "重开结算批次", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionFinanceManage, TargetType: SaaSAdminOperationTargetSettlementBatch, Description: "冻结已关闭批次的完整账务与关账审计快照，双人复核后原子重开并恢复对账修改能力", Enabled: true, RequiredApprovals: 2, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, Version: 1},
		{ActionType: SaaSAdminApprovalActionPaymentSettlementResolve, Name: "处理结算差异", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionFinanceManage, TargetType: SaaSAdminOperationTargetSettlementEntry, Description: "冻结结算差异明细和所属批次完整账务快照，双人复核后原子解决、忽略或重新打开差异", Enabled: true, RequiredApprovals: 2, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, Version: 1},
		{ActionType: SaaSAdminApprovalActionAccessRoleSave, Name: "变更平台岗位", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionAccessManage, TargetType: SaaSAdminOperationTargetAccessRole, Description: "创建或修改平台岗位及其有效权限", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionAccessAssignmentSave, Name: "变更人员授权", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionAccessManage, TargetType: SaaSAdminOperationTargetAccessAssignment, Description: "覆盖平台成员岗位并立即改变总后台访问权", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionTenantDataErase, Name: "授权租户数据擦除", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionComplianceManage, TargetType: "saas_erasure_request", Description: "授权已停用业务租户在宽限期和保留门禁满足后进入不可逆擦除队列", Enabled: true, RequiredApprovals: 2, SLAMinutes: 1440, ReminderMinutes: 240, ExpiryHours: 72, Version: 1},
		{ActionType: SaaSAdminApprovalActionReleaseCandidateGate, Name: "运行发布候选门禁", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionReleaseManage, TargetType: SaaSAdminOperationTargetCandidate, Description: "重新远端校验六类生产证据并生成不可变发布候选", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 6, Version: 1},
		{ActionType: SaaSAdminApprovalActionApprovalPolicyUpdate, Name: "变更审批策略", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionApprovalsManage, TargetType: SaaSAdminOperationTargetApprovalPolicy, Description: "变更高风险动作的开关、会签人数、SLA、提醒或有效期", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionBackupPolicyUpdate, Name: "变更备份策略", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionBackupsManage, TargetType: "saas_backup_policy", Description: "变更平台数据库备份启停、加密、异地副本、频率与保留策略", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionBackupRetentionCleanup, Name: "执行备份保留清理", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionBackupsManage, TargetType: "saas_backup_cleanup_run", Description: "冻结过期备份范围并按异地副本、本地工件、数据库记录的顺序执行可重试清理", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionCompliancePolicyUpdate, Name: "变更合规生命周期策略", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionComplianceManage, TargetType: "saas_compliance_policy", Description: "变更租户导出、擦除、账务、审计和服务账号用量数据的保留边界与执行门禁", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionComplianceExportDelete, Name: "删除合规导出工件", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionComplianceManage, TargetType: "saas_data_export", Description: "冻结合规导出快照并按本地加密工件、数据库记录的顺序执行可重试删除", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionComplianceLegalHoldRelease, Name: "解除法律保留", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionComplianceManage, TargetType: "saas_legal_hold", Description: "冻结法律保留快照并在双人复核后解除租户删除门禁", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionIdentityPolicyUpdate, Name: "变更身份安全策略", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionIdentityManage, TargetType: "saas_identity_policy", Description: "变更租户密码锁定、会话、MFA、登录 IP 白名单与身份数据保留策略", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionIdentityMFAReset, Name: "重置用户 MFA", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionIdentityManage, TargetType: "saas_identity_mfa", Description: "冻结用户、租户、MFA 凭据版本与重置原因，双人复核后清除二次认证并撤销全部活动会话", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionServiceAccountCreate, Name: "创建服务账号", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionIntegrationsManage, TargetType: SaaSAdminOperationTargetServiceAccount, Description: "冻结服务账号配置与首个 Key 参数，双人复核后创建账号并一次性交付首个 Key", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionServiceAccountUpdate, Name: "变更服务账号", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionIntegrationsManage, TargetType: SaaSAdminOperationTargetServiceAccount, Description: "冻结服务账号版本并在双人复核后变更状态、作用域、网络边界、额度和有效期", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionServiceAccountKeyRotate, Name: "轮换服务账号 API Key", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionIntegrationsManage, TargetType: SaaSAdminOperationTargetServiceAccountKey, Description: "冻结服务账号版本与轮换参数，双人复核后生成一次性新 Key 并切换旧 Key 宽限状态", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
		{ActionType: SaaSAdminApprovalActionServiceAccountKeyRevoke, Name: "吊销服务账号 API Key", RiskLevel: SaaSAdminApprovalRiskCritical, RequiredPermission: SaaSAdminPermissionIntegrationsManage, TargetType: SaaSAdminOperationTargetServiceAccountKey, Description: "冻结 API Key 版本并在双人复核后立即阻断该密钥访问", Enabled: true, RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, Version: 1},
	}
	for index := range policies {
		policies[index] = enforceSaaSAdminApprovalPolicyGovernance(policies[index])
	}
	return policies
}

func SaaSAdminApprovalPolicyByAction(actionType string) (SaaSAdminApprovalPolicy, bool) {
	actionType = strings.TrimSpace(actionType)
	for _, policy := range SaaSAdminApprovalPolicies() {
		if policy.ActionType == actionType {
			return policy, true
		}
	}
	return SaaSAdminApprovalPolicy{}, false
}

func saasAdminApprovalPolicyGovernanceLocked(policy SaaSAdminApprovalPolicy) bool {
	return policy.RiskLevel == SaaSAdminApprovalRiskCritical
}

func saasAdminApprovalPolicyMinimumApprovals(policy SaaSAdminApprovalPolicy) int {
	if saasAdminApprovalPolicyGovernanceLocked(policy) {
		return 2
	}
	return 1
}

func saasAdminApprovalPolicyAmountThresholdLocked(policy SaaSAdminApprovalPolicy) bool {
	return saasAdminApprovalPolicyGovernanceLocked(policy)
}

func enforceSaaSAdminApprovalPolicyGovernance(policy SaaSAdminApprovalPolicy) SaaSAdminApprovalPolicy {
	if !saasAdminApprovalPolicyGovernanceLocked(policy) {
		return policy
	}
	policy.Enabled = true
	policy.AmountThresholdCents = 0
	if policy.RequiredApprovals < saasAdminApprovalPolicyMinimumApprovals(policy) {
		policy.RequiredApprovals = saasAdminApprovalPolicyMinimumApprovals(policy)
	}
	return policy
}

func (h *SaaSAdminHandler) ApprovalPolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	policies, err := h.effectiveSaaSAdminApprovalPolicies(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"required": h.highRiskApproval, "policies": saasAdminApprovalPolicyPayloads(policies),
	})
}

func (h *SaaSAdminHandler) Approvals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminApprovalStore(w)
	if !ok {
		return
	}
	options, err := parseSaaSAdminApprovalOptions(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	report, err := store.SaaSAdminApprovals(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasAdminApprovalReportPayload(report))
}

func (h *SaaSAdminHandler) ApprovalEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminApprovalStore(w)
	if !ok {
		return
	}
	approvalID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("approvalId")), 10, 64)
	limit := positiveQueryInt(r, "limit", 100)
	if approvalID <= 0 || limit > 500 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "approvalId 必填且 limit 不能超过 500", nil)
		return
	}
	events, err := store.SaaSAdminApprovalEvents(r.Context(), approvalID, limit)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"events": saasAdminApprovalEventPayloads(events)})
}

type saasAdminApprovalRequestBody struct {
	ActionType     string          `json:"actionType"`
	Payload        json.RawMessage `json:"payload"`
	Reason         string          `json:"reason"`
	ExpiresInHours int             `json:"expiresInHours"`
	IdempotencyKey string          `json:"idempotencyKey"`
}

func (h *SaaSAdminHandler) ApprovalRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminApprovalStore(w)
	if !ok {
		return
	}
	var body saasAdminApprovalRequestBody
	if err := decodeSaaSAdminAccessJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	reason := strings.TrimSpace(body.Reason)
	if reason == "" || len([]rune(reason)) > 255 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "reason 必填且最多 255 个字符", nil)
		return
	}
	requestNo, err := newSaaSAdminApprovalRequestNo()
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	idempotencyKey := strings.TrimSpace(body.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = requestNo
	}
	if len(idempotencyKey) > 128 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "idempotencyKey 最多 128 个字符", nil)
		return
	}
	policy, normalized, targetID, targetName, err := h.normalizeSaaSAdminApprovalPayload(r.Context(), user, strings.TrimSpace(body.ActionType), body.Payload, idempotencyKey)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	policy, err = h.effectiveSaaSAdminApprovalPolicy(r.Context(), policy.ActionType)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	amountCents := saasAdminApprovalAmountCents(policy.ActionType, normalized)
	if !saasAdminApprovalPolicyRequires(policy, amountCents) {
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, "当前策略不要求该操作审批，可直接执行", map[string]any{
			"actionType": policy.ActionType, "enabled": policy.Enabled, "amountThresholdCents": policy.AmountThresholdCents,
		})
		return
	}
	expiresInHours := body.ExpiresInHours
	if expiresInHours == 0 {
		expiresInHours = policy.ExpiryHours
	}
	if expiresInHours < 1 || expiresInHours > policy.ExpiryHours {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "expiresInHours 必须在 1 至策略有效期之间", nil)
		return
	}
	profile, err := h.saasAdminAccessProfile(r.Context(), user)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if !SaaSAdminAccessHasPermission(profile, policy.RequiredPermission) {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "缺少高风险动作权限 "+policy.RequiredPermission, nil)
		return
	}
	digest, err := saasAdminApprovalRequestDigest(policy.ActionType, normalized)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	now := time.Now()
	result, err := store.CreateSaaSAdminApproval(r.Context(), SaaSAdminApprovalCreate{
		RequestNo: requestNo, ActionType: policy.ActionType, RiskLevel: policy.RiskLevel,
		RequiredPermission: policy.RequiredPermission, TargetType: policy.TargetType, TargetID: targetID, TargetName: targetName,
		RequesterUserID: user.ID, RequesterTenantID: user.TenantID, IdempotencyKey: idempotencyKey,
		RequestSHA256: hex.EncodeToString(digest[:]), RequestJSON: string(normalized), Reason: reason,
		ExpiresAt: now.Add(time.Duration(expiresInHours) * time.Hour), PolicyVersion: policy.Version,
		RequiredApprovals: policy.RequiredApprovals, ReminderMinutes: policy.ReminderMinutes,
		SLADueAt: now.Add(time.Duration(policy.SLAMinutes) * time.Minute), NextReminderAt: now.Add(time.Duration(policy.ReminderMinutes) * time.Minute),
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"approval": saasAdminApprovalPayload(result.Approval), "operationId": result.OperationID, "idempotent": result.Idempotent,
	})
}

type saasAdminApprovalDecisionBody struct {
	ApprovalID      int64  `json:"approvalId"`
	Decision        string `json:"decision"`
	Reason          string `json:"reason"`
	ExpectedVersion int    `json:"expectedVersion"`
}

func (h *SaaSAdminHandler) ApprovalDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminApprovalStore(w)
	if !ok {
		return
	}
	var body saasAdminApprovalDecisionBody
	if err := decodeSaaSAdminAccessJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	body.Decision = strings.ToLower(strings.TrimSpace(body.Decision))
	body.Reason = strings.TrimSpace(body.Reason)
	if body.ApprovalID <= 0 || body.ExpectedVersion <= 0 || (body.Decision != SaaSAdminApprovalDecisionApprove && body.Decision != SaaSAdminApprovalDecisionReject) || body.Reason == "" || len([]rune(body.Reason)) > 255 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "approvalId、expectedVersion、decision=approve/reject 和 reason 必填", nil)
		return
	}
	delegatedFromUserID, err := h.approvalDelegatedFromUserID(r.Context(), user)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	approval, err := store.DecideSaaSAdminApproval(r.Context(), SaaSAdminApprovalDecision{
		ApprovalID: body.ApprovalID, Decision: body.Decision, Reason: body.Reason, ExpectedVersion: body.ExpectedVersion,
		ActorUserID: user.ID, ActorTenantID: user.TenantID, DelegatedFromUserID: delegatedFromUserID,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"approval": saasAdminApprovalPayload(approval)})
}

type saasAdminApprovalCancelBody struct {
	ApprovalID      int64  `json:"approvalId"`
	Reason          string `json:"reason"`
	ExpectedVersion int    `json:"expectedVersion"`
}

func (h *SaaSAdminHandler) ApprovalCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminApprovalStore(w)
	if !ok {
		return
	}
	var body saasAdminApprovalCancelBody
	if err := decodeSaaSAdminAccessJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if body.ApprovalID <= 0 || body.ExpectedVersion <= 0 || body.Reason == "" || len([]rune(body.Reason)) > 255 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "approvalId、expectedVersion 和 reason 必填", nil)
		return
	}
	approval, err := store.CancelSaaSAdminApproval(r.Context(), SaaSAdminApprovalCancel{
		ApprovalID: body.ApprovalID, Reason: body.Reason, ExpectedVersion: body.ExpectedVersion,
		ActorUserID: user.ID, ActorTenantID: user.TenantID,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"approval": saasAdminApprovalPayload(approval)})
}

type saasAdminApprovalExecuteBody struct {
	ApprovalID      int64 `json:"approvalId"`
	ExpectedVersion int   `json:"expectedVersion"`
}

func (h *SaaSAdminHandler) ApprovalExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminApprovalStore(w)
	if !ok {
		return
	}
	var body saasAdminApprovalExecuteBody
	if err := decodeSaaSAdminAccessJSON(r, &body); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if body.ApprovalID <= 0 || body.ExpectedVersion <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "approvalId 和 expectedVersion 必填", nil)
		return
	}
	approval, err := store.BeginSaaSAdminApprovalExecution(r.Context(), SaaSAdminApprovalExecutionStart{
		ApprovalID: body.ApprovalID, ExpectedVersion: body.ExpectedVersion, ActorUserID: user.ID, ActorTenantID: user.TenantID,
	})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	actionContext, cancelAction := context.WithTimeout(context.WithoutCancel(r.Context()), saasAdminApprovalActionTimeout)
	var resultPayload map[string]any
	var executionErr error
	if approval.EffectAppliedAtValue.IsZero() {
		resultPayload, executionErr = h.executeSaaSAdminApproval(actionContext, approval, user)
	} else {
		resultPayload = map[string]any{
			"recovered": true, "effectAppliedAt": approval.EffectAppliedAt, "effectOperationId": approval.EffectOperationID,
		}
	}
	cancelAction()
	resultJSON := ""
	if executionErr == nil {
		raw, marshalErr := json.Marshal(saasAdminApprovalPersistentResult(approval.ActionType, resultPayload))
		if marshalErr != nil {
			executionErr = marshalErr
		} else {
			resultJSON = string(raw)
		}
	}
	finishContext, cancelFinish := context.WithTimeout(context.WithoutCancel(r.Context()), saasAdminApprovalFinishTimeout)
	defer cancelFinish()
	finish, finishErr := store.FinishSaaSAdminApprovalExecution(finishContext, SaaSAdminApprovalExecutionFinish{
		ApprovalID: approval.ID, ExpectedVersion: approval.Version, ActorUserID: user.ID, ActorTenantID: user.TenantID,
		Success: executionErr == nil, ResultJSON: resultJSON, ErrorMessage: approvalExecutionError(executionErr),
	})
	if finishErr != nil {
		writeSaaSAdminError(w, finishErr)
		return
	}
	if executionErr != nil {
		writeSaaSAdminError(w, executionErr)
		return
	}
	response := map[string]any{"approval": saasAdminApprovalPayload(finish), "result": resultPayload}
	if activationToken, ok := resultPayload["activationToken"].(string); ok && strings.TrimSpace(activationToken) != "" {
		writeOneTimeSecretEnvelope(w, http.StatusOK, "激活令牌仅在本次响应中展示，请立即通过受控渠道交付", response)
		return
	}
	if plainTextKey, ok := resultPayload["plainTextKey"].(string); ok && strings.TrimSpace(plainTextKey) != "" {
		writeOneTimeServiceAccountKey(w, http.StatusOK, response)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", response)
}

func writeOneTimeSecretEnvelope(w http.ResponseWriter, status int, message string, payload map[string]any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeEnvelope(w, status, status, message, payload)
}

func saasAdminApprovalPersistentResult(actionType string, result map[string]any) map[string]any {
	if result == nil {
		return result
	}
	if actionType == SaaSAdminApprovalActionDashboardTenantProvision || actionType == SaaSAdminApprovalActionDashboardActivationResend {
		persistent := make(map[string]any, len(result)+1)
		delivered := false
		for key, value := range result {
			if key != "activationToken" && key != "activationPath" {
				persistent[key] = value
				continue
			}
			if key == "activationPath" {
				continue
			}
			if token, ok := value.(string); ok && strings.TrimSpace(token) != "" {
				delivered = true
			}
		}
		persistent["activationTokenDelivered"] = delivered
		return persistent
	}
	if actionType == SaaSAdminApprovalActionTenantDomainCreate {
		persistent := make(map[string]any, len(result))
		for key, value := range result {
			persistent[key] = value
		}
		if domain, ok := result["domain"].(map[string]any); ok {
			redacted := make(map[string]any, len(domain))
			for key, value := range domain {
				if key != "verificationToken" && key != "verificationRecordValue" {
					redacted[key] = value
				}
			}
			redacted["verificationTokenDelivered"] = true
			persistent["domain"] = redacted
		}
		return persistent
	}
	if actionType != SaaSAdminApprovalActionServiceAccountCreate && actionType != SaaSAdminApprovalActionServiceAccountKeyRotate {
		return result
	}
	persistent := make(map[string]any, len(result))
	for key, value := range result {
		if key != "plainTextKey" {
			persistent[key] = value
		}
	}
	persistent["plainTextKeyDelivered"] = true
	return persistent
}

func saasAdminApprovalRequestDigest(actionType string, normalized []byte) ([32]byte, error) {
	if actionType != SaaSAdminApprovalActionTenantProvision {
		return sha256.Sum256(normalized), nil
	}
	var plan SaaSAdminTenantProvisionApprovalPlan
	if err := json.Unmarshal(normalized, &plan); err != nil {
		return [32]byte{}, err
	}
	plan.Provision.AdminPasswordHash = ""
	redacted, err := json.Marshal(plan)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(redacted), nil
}

func (h *SaaSAdminHandler) normalizeSaaSAdminApprovalPayload(ctx context.Context, user User, actionType string, raw json.RawMessage, approvalIdempotencyKey string) (SaaSAdminApprovalPolicy, []byte, string, string, error) {
	policy, ok := SaaSAdminApprovalPolicyByAction(actionType)
	if !ok {
		return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("未知高风险动作: " + actionType)
	}
	if len(raw) == 0 || len(raw) > 1<<20 {
		return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("payload 必填且不能超过 1 MiB")
	}
	if actionType == SaaSAdminApprovalActionDashboardTenantProvision || actionType == SaaSAdminApprovalActionDashboardActivationResend || actionType == SaaSAdminApprovalActionDashboardSuperAdminReplace || actionType == SaaSAdminApprovalActionDashboardSuperAdminStatus {
		plan, err := dashboardadmin.NormalizeSaaSAdminApprovalPayload(actionType, raw)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("Dashboard 审批载荷无效")
		}
		return policy, plan.NormalizedJSON, plan.TargetID, plan.TargetName, nil
	}
	requestFromJSON := func() *http.Request {
		return &http.Request{Method: http.MethodPost, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(raw))}
	}
	var normalized any
	targetID := ""
	targetName := ""
	switch actionType {
	case SaaSAdminApprovalActionTenantDisable, SaaSAdminApprovalActionTenantEnable:
		input, err := parseSaaSAdminTenantStatusUpdate(requestFromJSON())
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		requestedStatus := 1
		currentStatus := 2
		message := "审批动作 tenant.enable 只能启用停用业务租户"
		if actionType == SaaSAdminApprovalActionTenantDisable {
			requestedStatus = 2
			currentStatus = 1
			message = "审批动作 tenant.disable 只能停用正常业务租户"
		}
		if input.Status != requestedStatus {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(message)
		}
		planner, ok := h.store.(SaaSAdminTenantStatusApprovalStore)
		if !ok || planner == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("tenant status approval store is not configured")
		}
		plan, err := planner.PlanSaaSAdminTenantStatusUpdate(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		if plan.SchemaVersion != SaaSAdminTenantStatusApprovalPlanSchemaVersion ||
			plan.Update.TenantID != input.TenantID || plan.Update.Status != requestedStatus ||
			plan.Snapshot.TenantID != input.TenantID || plan.Snapshot.TenantStatus != currentStatus ||
			plan.Update.ExpectedStatus != currentStatus {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(message)
		}
		normalized, targetID, targetName = plan, strconv.Itoa(input.TenantID), strings.TrimSpace(plan.Snapshot.TenantName)
		if targetName == "" {
			targetName = "租户 " + targetID
		}
	case SaaSAdminApprovalActionTenantProvision:
		var selector struct {
			TaskID              int64 `json:"taskId"`
			ExpectedTaskVersion int   `json:"expectedTaskVersion"`
		}
		if err := json.Unmarshal(raw, &selector); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("JSON 格式错误")
		}
		var plan SaaSAdminTenantProvisionApprovalPlan
		if selector.TaskID > 0 {
			if selector.ExpectedTaskVersion <= 0 {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("taskId 和 expectedTaskVersion 必须大于 0")
			}
			tasks, err := h.store.SaaSAdminTasks(ctx, SaaSAdminTaskOptions{TaskID: selector.TaskID, Limit: 1})
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", err
			}
			if len(tasks) == 0 {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminNotFound("task not found")
			}
			task := tasks[0]
			if task.TaskType != SaaSAdminTaskTypeTenantProvision {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("task type unsupported")
			}
			if task.Version != selector.ExpectedTaskVersion {
				return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务版本已变化，请刷新后重试"}
			}
			if task.Status == SaaSAdminTaskStatusApplied || task.Status == SaaSAdminTaskStatusCanceled {
				return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务状态不允许申请开户审批"}
			}
			provision, err := saasAdminTenantProvisionFromTaskRequest(task.RequestJSON)
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
			}
			preview, pkg, err := h.tenantProvisionPreviewWithPackage(ctx, provision)
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", err
			}
			if preview.Blocked {
				return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: preview.BlockReason}
			}
			taskDigest := sha256.Sum256([]byte(task.RequestJSON))
			taskRequestSHA256 := hex.EncodeToString(taskDigest[:])
			plan = SaaSAdminTenantProvisionApprovalPlan{
				Source:                    "task",
				TaskID:                    task.ID,
				ExpectedTaskVersion:       task.Version,
				ExpectedTaskRequestSHA256: taskRequestSHA256,
				CredentialFingerprint:     taskRequestSHA256,
				Provision:                 provision,
				Preview:                   preview,
				Package:                   pkg,
			}
		} else {
			provision, password, err := parseSaaSAdminTenantProvision(requestFromJSON())
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
			}
			if strings.TrimSpace(h.passwordSecret) == "" {
				return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("password secret not configured")
			}
			passwordHash, err := authjwt.GeneratePasswordHash(h.passwordSecret, password)
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", err
			}
			provision.AdminPasswordHash = passwordHash
			preview, pkg, err := h.tenantProvisionPreviewWithPackage(ctx, provision)
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", err
			}
			mac := hmac.New(sha256.New, []byte(h.passwordSecret))
			_, _ = mac.Write([]byte(approvalIdempotencyKey))
			_, _ = mac.Write([]byte{0})
			_, _ = mac.Write([]byte(password))
			plan = SaaSAdminTenantProvisionApprovalPlan{
				Source:                "direct",
				CredentialFingerprint: hex.EncodeToString(mac.Sum(nil)),
				Provision:             provision,
				Preview:               preview,
				Package:               pkg,
			}
		}
		normalized = plan
		targetID = plan.Preview.AdminPhone
		if plan.TaskID > 0 {
			targetID = "task:" + strconv.FormatInt(plan.TaskID, 10)
		} else if plan.Provision.TenantID > 0 {
			targetID = strconv.Itoa(plan.Provision.TenantID)
		}
		targetName = plan.Preview.TenantName + " / " + plan.Preview.AdminPhone
	case SaaSAdminApprovalActionTenantRenewal:
		var selector struct {
			TaskID              int64 `json:"taskId"`
			ExpectedTaskVersion int   `json:"expectedTaskVersion"`
		}
		if err := json.Unmarshal(raw, &selector); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("JSON 格式错误")
		}
		var plan SaaSAdminTenantRenewalApprovalPlan
		if selector.TaskID > 0 {
			if selector.ExpectedTaskVersion <= 0 {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("taskId 和 expectedTaskVersion 必须大于 0")
			}
			tasks, err := h.store.SaaSAdminTasks(ctx, SaaSAdminTaskOptions{TaskID: selector.TaskID, Limit: 1})
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", err
			}
			if len(tasks) == 0 {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminNotFound("task not found")
			}
			task := tasks[0]
			if task.TaskType != SaaSAdminTaskTypeTenantRenewal {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("task type unsupported")
			}
			if task.Version != selector.ExpectedTaskVersion {
				return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务版本已变化，请刷新后重试"}
			}
			if task.Status == SaaSAdminTaskStatusApplied || task.Status == SaaSAdminTaskStatusCanceled {
				return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "任务状态不允许申请续费审批"}
			}
			renewal, err := saasAdminTenantRenewalFromTaskRequest(task.RequestJSON)
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
			}
			preview, resolved, state, err := h.tenantRenewalPreviewState(ctx, renewal)
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", err
			}
			if preview.Blocked {
				return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: preview.BlockReason}
			}
			taskDigest := sha256.Sum256([]byte(task.RequestJSON))
			plan = SaaSAdminTenantRenewalApprovalPlan{
				Source:                    "task",
				TaskID:                    task.ID,
				ExpectedTaskVersion:       task.Version,
				ExpectedTaskRequestSHA256: hex.EncodeToString(taskDigest[:]),
				ExpectedTenantStatus:      resolved.ExpectedTenantStatus,
				Renewal:                   resolved,
				Preview:                   preview,
				CurrentPackage:            state.CurrentPackage,
				TargetPackage:             state.TargetPackage,
				Subscription:              state.Subscription,
			}
		} else {
			renewal, err := parseSaaSAdminTenantRenewal(requestFromJSON())
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
			}
			preview, resolved, state, err := h.tenantRenewalPreviewState(ctx, renewal)
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", err
			}
			if preview.Blocked {
				return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: preview.BlockReason}
			}
			plan = SaaSAdminTenantRenewalApprovalPlan{
				Source:               "direct",
				ExpectedTenantStatus: resolved.ExpectedTenantStatus,
				Renewal:              resolved,
				Preview:              preview,
				CurrentPackage:       state.CurrentPackage,
				TargetPackage:        state.TargetPackage,
				Subscription:         state.Subscription,
			}
		}
		normalized = plan
		targetID = strconv.Itoa(plan.Renewal.TenantID)
		if plan.TaskID > 0 {
			targetID = "task:" + strconv.FormatInt(plan.TaskID, 10)
		}
		targetName = strings.TrimSpace(plan.Preview.TenantName + " / " + plan.TargetPackage.Name + " / " + plan.Renewal.ExpiresAt)
	case SaaSAdminApprovalActionSubscriptionTransition:
		transition, err := parseSaaSAdminSubscriptionTransition(requestFromJSON())
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		idempotencyDigest := sha256.Sum256([]byte("subscription-transition:" + approvalIdempotencyKey))
		transition.IdempotencyKey = "approval:" + hex.EncodeToString(idempotencyDigest[:])
		transition.Source = "approval"
		plan, err := h.planSaaSAdminSubscriptionTransition(ctx, transition)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		normalized = plan
		targetID = strconv.FormatInt(plan.Subscription.ID, 10)
		targetName = strings.TrimSpace(plan.Subscription.TenantName + " / " + plan.Subscription.Status + " -> " + plan.Transition.Status)
	case SaaSAdminApprovalActionPackageUpsert:
		var input SaaSAdminPackageUpsert
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if err := normalizeSaaSAdminPackageUpsert(&input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		plan, err := h.planPackageUpsert(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		normalized, targetID, targetName = plan, input.Code, input.Name
	case SaaSAdminApprovalActionTenantPackageUpdate:
		input, err := parseSaaSAdminTenantPackageUpdate(requestFromJSON())
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		plan, err := h.planTenantPackageUpdate(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		normalized = plan
		targetID = strconv.Itoa(input.TenantID)
		targetName = plan.TenantName + " / " + plan.TargetPackage.Name
	case SaaSAdminApprovalActionPaymentOrderCreate:
		input, err := parseSaaSAdminPaymentOrderCreate(requestFromJSON())
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if input.OrderNo == "" {
			digest := sha256.Sum256([]byte("payment-order:" + approvalIdempotencyKey))
			input.OrderNo = "APR-PAY-" + strings.ToUpper(hex.EncodeToString(digest[:12]))
		}
		if input.IdempotencyKey == "" {
			input.IdempotencyKey = "approval:" + approvalIdempotencyKey
		}
		plan, err := h.planSaaSAdminPaymentOrderCreate(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		normalized = plan
		targetID = input.OrderNo
		targetName = strings.TrimSpace(plan.Tenant.TenantName + " / " + plan.Package.Name + " / " + strconv.FormatInt(input.AmountCents, 10) + " " + input.Currency + " 分")
	case SaaSAdminApprovalActionPaymentRefundCreate:
		input, err := parseSaaSAdminPaymentRefundCreate(requestFromJSON())
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if input.RefundNo == "" {
			digest := sha256.Sum256([]byte("refund:" + approvalIdempotencyKey))
			input.RefundNo = "APR-REF-" + strings.ToUpper(hex.EncodeToString(digest[:12]))
		}
		if input.IdempotencyKey == "" {
			input.IdempotencyKey = "approval:" + approvalIdempotencyKey
		}
		normalized, targetID, targetName = input, input.RefundNo, input.OrderNo
	case SaaSAdminApprovalActionInvoiceIssue:
		input, err := parseSaaSInvoiceDocumentTransition(requestFromJSON())
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if input.Status != SaaSInvoiceStatusIssued {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("审批动作 billing.invoice.issue 只支持 issued 状态")
		}
		plan, err := h.planSaaSInvoiceIssue(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		kindName := "蓝票"
		if plan.Document.Kind == SaaSInvoiceKindCreditNote {
			kindName = "红票"
		}
		normalized = plan
		targetID = plan.Document.DocumentNo
		targetName = strings.TrimSpace(plan.Document.TenantName + " / " + kindName + " / " + plan.Document.DocumentNo)
	case SaaSAdminApprovalActionPaymentSettlementClose:
		input, err := parseSaaSPaymentSettlementTransition(requestFromJSON())
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if input.Action != SaaSPaymentSettlementTransitionClose {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("审批动作 payment.settlement.close 只支持 close")
		}
		plan, err := h.planSaaSPaymentSettlementClose(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		normalized = plan
		targetID = plan.Batch.BatchNo
		targetName = strings.TrimSpace(plan.Batch.Provider + " / " + plan.Batch.ProviderSettlementNo + " / " + plan.Batch.BatchNo)
	case SaaSAdminApprovalActionPaymentSettlementReopen:
		input, err := parseSaaSPaymentSettlementTransition(requestFromJSON())
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if input.Action != SaaSPaymentSettlementTransitionReopen {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("审批动作 payment.settlement.reopen 只支持 reopen")
		}
		plan, err := h.planSaaSPaymentSettlementReopen(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		normalized = plan
		targetID = plan.Batch.BatchNo
		targetName = strings.TrimSpace(plan.Batch.Provider + " / " + plan.Batch.ProviderSettlementNo + " / " + plan.Batch.BatchNo)
	case SaaSAdminApprovalActionPaymentSettlementResolve:
		input, err := parseSaaSPaymentSettlementEntryResolve(requestFromJSON())
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		plan, err := h.planSaaSPaymentSettlementResolve(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		normalized = plan
		targetID = strconv.FormatInt(plan.Entry.ID, 10)
		targetName = strings.TrimSpace(plan.Entry.Provider + " / " + plan.Entry.ProviderTransactionNo + " / " + plan.Entry.BatchNo)
	case SaaSAdminApprovalActionTenantDomainCreate:
		var request SaaSAdminTenantDomainRequest
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &request); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if err := normalizeAndValidateSaaSAdminTenantDomainRequest(&request); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if request.Action != SaaSTenantDomainActionCreate {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("审批动作 tenant.domain.create 只支持新增域名")
		}
		store, ok := h.store.(SaaSAdminTenantDomainCreateApprovalStore)
		if !ok || store == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("tenant domain create approval store is not configured")
		}
		target, err := store.SaaSAdminTenantDomainCreateTarget(ctx, request.TenantID, request.Hostname)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		normalized = request
		targetID = strconv.Itoa(request.TenantID) + ":" + request.Hostname
		targetName = strings.TrimSpace(target.TenantName)
		if targetName == "" {
			targetName = "租户 #" + strconv.Itoa(request.TenantID)
		}
		targetName += " / " + request.Hostname
	case SaaSAdminApprovalActionTenantDomainCommand:
		var request SaaSAdminTenantDomainRequest
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &request); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if err := normalizeAndValidateSaaSAdminTenantDomainRequest(&request); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if !SaaSTenantDomainCommandRequiresApproval(request.Action) {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("审批动作 tenant.domain.command 只支持路由生命周期变更")
		}
		planner, ok := h.store.(SaaSAdminTenantDomainApprovalStore)
		if !ok || planner == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("SaaS tenant domain approval store is not configured")
		}
		plan, err := planner.PlanSaaSAdminTenantDomainCommand(ctx, request)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		normalized = plan
		targetID = strconv.FormatInt(plan.Domain.ID, 10)
		targetName = strings.TrimSpace(plan.Domain.Hostname + " / " + request.Action)
	case SaaSAdminApprovalActionAccessRoleSave:
		var input SaaSAdminAccessRoleUpsert
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if err := validateSaaSAdminAccessRoleUpsert(&input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if user.IsSuperAdmin != 1 && input.ID > 0 {
			profile, err := h.saasAdminAccessProfile(ctx, user)
			if err != nil {
				return SaaSAdminApprovalPolicy{}, nil, "", "", err
			}
			for _, role := range profile.Roles {
				if role.ID == input.ID {
					return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("不能申请修改自己持有的平台岗位")
				}
			}
		}
		targetID = input.Code
		if input.ID > 0 {
			targetID = strconv.FormatInt(input.ID, 10)
		}
		normalized, targetName = input, input.Name
	case SaaSAdminApprovalActionAccessAssignmentSave:
		var input SaaSAdminAccessAssignmentUpdate
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		input.RoleIDs = uniquePositiveInt64s(input.RoleIDs)
		if input.UserID <= 0 || input.ExpectedVersion < 0 || len(input.RoleIDs) > 20 {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("userId、expectedVersion 或 roleIds 格式错误")
		}
		if input.UserID == user.ID {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("不能申请修改自己的平台授权")
		}
		normalized, targetID, targetName = input, strconv.Itoa(input.UserID), "平台用户 "+strconv.Itoa(input.UserID)
	case SaaSAdminApprovalActionTenantDataErase:
		var input struct {
			RequestID int64 `json:"requestId"`
		}
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if h.complianceManager == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("SaaS compliance manager is not configured")
		}
		request, err := h.complianceManager.Erasure(ctx, input.RequestID)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", saasComplianceAdminError(err)
		}
		if request.Status != saascompliance.ErasureStatusPendingApproval || request.ApprovalID != 0 {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("租户数据擦除请求已授权或状态不允许审批")
		}
		normalized, targetID, targetName = input, strconv.FormatInt(input.RequestID, 10), request.TenantName
	case SaaSAdminApprovalActionReleaseCandidateGate:
		var input SaaSReleaseCandidateApprovalPayload
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		candidate := SaaSReleaseCandidateCreate{ReleaseVersion: input.ReleaseVersion, SourceFingerprint: input.SourceFingerprint}
		if err := h.normalizeSaaSReleaseCandidateRequest(&candidate); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		store, ok := h.store.(SaaSReleaseReadinessStore)
		if !ok || store == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("SaaS release readiness store is not configured")
		}
		items, err := store.SaaSReleaseEvidence(ctx)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		if !saasReleaseEvidenceMetadataReady(items, candidate.SourceFingerprint) {
			return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "六类生产证据元数据尚未全部通过或源码指纹不一致"}
		}
		input.ReleaseVersion, input.SourceFingerprint = candidate.ReleaseVersion, candidate.SourceFingerprint
		normalized, targetID, targetName = input, candidate.SourceFingerprint, candidate.ReleaseVersion
	case SaaSAdminApprovalActionApprovalPolicyUpdate:
		var input saasAdminApprovalPolicyBody
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if err := validateSaaSAdminApprovalPolicyBody(&input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		target, _ := SaaSAdminApprovalPolicyByAction(input.ActionType)
		normalized, targetID, targetName = input, input.ActionType, target.Name
	case SaaSAdminApprovalActionBackupPolicyUpdate:
		if h.backupManager == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("SaaS backup manager is not configured")
		}
		var input saasbackup.PolicyUpdate
		if err := decodeSaaSBackupJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		input, err := h.backupManager.ValidatePolicyUpdate(input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", saasBackupAdminError(err)
		}
		normalized, targetID, targetName = input, "1", "平台数据库备份策略"
	case SaaSAdminApprovalActionBackupRetentionCleanup:
		if h.backupManager == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("SaaS backup manager is not configured")
		}
		var input struct{}
		if err := decodeSaaSBackupJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("备份清理审批 payload 必须是空对象")
		}
		plan, err := h.backupManager.PlanCleanup(ctx)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", saasBackupAdminError(err)
		}
		normalized = plan
		targetID = fmt.Sprintf("policy:%d:%d", plan.PolicyVersion, plan.CutoffAt.Unix())
		targetName = fmt.Sprintf("%d 个过期备份", len(plan.Items))
	case SaaSAdminApprovalActionCompliancePolicyUpdate:
		if h.complianceManager == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("SaaS compliance manager is not configured")
		}
		var input saascompliance.PolicyUpdate
		if err := decodeSaaSComplianceJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		input, err := h.complianceManager.PlanPolicyUpdate(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", saasComplianceAdminError(err)
		}
		normalized, targetID, targetName = input, "1", "租户数据合规策略"
	case SaaSAdminApprovalActionComplianceExportDelete:
		if h.complianceManager == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("SaaS compliance manager is not configured")
		}
		var input struct {
			ExportID int64 `json:"exportId"`
		}
		if err := decodeSaaSComplianceJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		plan, err := h.complianceManager.PlanExportDeletion(ctx, input.ExportID)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", saasComplianceAdminError(err)
		}
		normalized, targetID, targetName = plan, strconv.FormatInt(plan.ExportID, 10), plan.ExportNo
	case SaaSAdminApprovalActionComplianceLegalHoldRelease:
		if h.complianceManager == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("SaaS compliance manager is not configured")
		}
		var input struct {
			HoldID          int64  `json:"holdId"`
			Reason          string `json:"reason"`
			ExpectedVersion int    `json:"expectedVersion"`
		}
		if err := decodeSaaSComplianceJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		plan, err := h.complianceManager.PlanLegalHoldRelease(ctx, input.HoldID, input.ExpectedVersion, input.Reason)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", saasComplianceAdminError(err)
		}
		normalized, targetID, targetName = plan, strconv.FormatInt(plan.HoldID, 10), plan.HoldNo
	case SaaSAdminApprovalActionIdentityPolicyUpdate:
		if h.identitySecurityManager == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("SaaS identity security manager is not configured")
		}
		var input identitysecurity.PolicyUpdate
		if err := decodeIdentitySecurityJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		input, current, err := h.identitySecurityManager.PlanPolicyUpdate(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", identitySecurityAdminError(err)
		}
		targetID = strconv.Itoa(input.TenantID)
		targetName = strings.TrimSpace(current.TenantName)
		if targetName == "" {
			targetName = "租户 " + targetID + " 身份安全策略"
		}
		normalized = input
	case SaaSAdminApprovalActionIdentityMFAReset:
		if h.identitySecurityManager == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("SaaS identity security manager is not configured")
		}
		var input identitysecurity.MFAReset
		if err := decodeIdentitySecurityJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		input, state, _, err := h.identitySecurityManager.PlanMFAReset(ctx, input)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", identitySecurityAdminError(err)
		}
		targetID = strconv.Itoa(input.UserID)
		targetName = strings.TrimSpace(state.UserName)
		if targetName == "" {
			targetName = "身份安全账号 #" + targetID
		}
		targetName += " / 租户 " + strconv.Itoa(input.TenantID)
		normalized = input
	case SaaSAdminApprovalActionServiceAccountCreate:
		var input SaaSServiceAccountCreate
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if err := normalizeSaaSServiceAccountCreate(&input, time.Now()); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		store, ok := h.store.(SaaSServiceAccountCreateApprovalStore)
		if !ok || store == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("service account create approval store is not configured")
		}
		target, err := store.SaaSServiceAccountCreateTarget(ctx, input.TenantID, input.Code)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		if target.TenantStatus != 1 {
			return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "只能为正常租户创建服务账号"}
		}
		targetName = strings.TrimSpace(target.TenantName)
		if targetName == "" {
			targetName = "租户 #" + strconv.Itoa(input.TenantID)
		}
		targetName += " / " + input.Name
		normalized = input
		targetID = strconv.Itoa(input.TenantID) + ":" + input.Code
	case SaaSAdminApprovalActionServiceAccountUpdate:
		var input SaaSServiceAccountUpdate
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if err := normalizeSaaSServiceAccountUpdate(&input, time.Now()); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		store, ok := h.store.(SaaSServiceAccountUpdateApprovalStore)
		if !ok || store == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("service account update approval store is not configured")
		}
		account, err := store.SaaSServiceAccountForApproval(ctx, input.ID)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		if account.Version != input.ExpectedVersion {
			return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "服务账号版本已变化，请刷新后重试"}
		}
		targetName = strings.TrimSpace(account.Name)
		if targetName == "" {
			targetName = "服务账号 #" + strconv.FormatInt(account.ID, 10)
		}
		normalized, targetID = input, strconv.FormatInt(account.ID, 10)
	case SaaSAdminApprovalActionServiceAccountKeyRotate:
		var input SaaSServiceAccountKeyRotate
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if err := normalizeSaaSServiceAccountKeyRotate(&input, time.Now()); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		store, ok := h.store.(SaaSServiceAccountUpdateApprovalStore)
		if !ok || store == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("service account rotate approval store is not configured")
		}
		account, err := store.SaaSServiceAccountForApproval(ctx, input.ServiceAccountID)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		if account.Version != input.ExpectedVersion {
			return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "服务账号版本已变化，请刷新后重试"}
		}
		if account.Status != SaaSServiceAccountStatusActive || account.TenantStatus != 1 {
			return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "只能为启用且租户正常的服务账号申请轮换密钥"}
		}
		now := time.Now()
		if accountExpiry, ok := parseSaaSAdminNormalizedDateTime(account.ExpiresAt); ok {
			if !now.Before(accountExpiry) {
				return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "服务账号已过期"}
			}
			keyExpiry, _ := parseSaaSAdminNormalizedDateTime(input.ExpiresAt)
			if keyExpiry.After(accountExpiry) {
				return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("新密钥过期时间不能晚于服务账号过期时间")
			}
		}
		accountName := strings.TrimSpace(account.Name)
		if accountName == "" {
			accountName = "服务账号 #" + strconv.FormatInt(account.ID, 10)
		}
		targetName = accountName + " / " + input.Name
		normalized, targetID = input, strconv.FormatInt(account.ID, 10)
	case SaaSAdminApprovalActionServiceAccountKeyRevoke:
		var input SaaSServiceAccountKeyRevoke
		if err := decodeSaaSAdminAccessJSON(requestFromJSON(), &input); err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest(err.Error())
		}
		if input.ServiceAccountID <= 0 || input.KeyID <= 0 || input.ExpectedVersion <= 0 {
			return SaaSAdminApprovalPolicy{}, nil, "", "", NewSaaSAdminBadRequest("serviceAccountId、keyId 和 expectedVersion 必须大于 0")
		}
		store, ok := h.store.(SaaSServiceAccountApprovalStore)
		if !ok || store == nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", errors.New("service account approval store is not configured")
		}
		account, key, err := store.SaaSServiceAccountKeyForApproval(ctx, input.ServiceAccountID, input.KeyID)
		if err != nil {
			return SaaSAdminApprovalPolicy{}, nil, "", "", err
		}
		if key.Version != input.ExpectedVersion {
			return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "API Key 版本已变化，请刷新后重试"}
		}
		if key.Status == SaaSServiceAccountKeyStatusRevoked {
			return SaaSAdminApprovalPolicy{}, nil, "", "", &SaaSAdminOperationError{Status: http.StatusConflict, Message: "API Key 已吊销"}
		}
		targetName = strings.TrimSpace(account.Name) + " / " + strings.TrimSpace(key.Name)
		if strings.TrimSpace(targetName) == "/" {
			targetName = "API Key #" + strconv.FormatInt(key.ID, 10)
		}
		normalized, targetID = input, strconv.FormatInt(key.ID, 10)
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return SaaSAdminApprovalPolicy{}, nil, "", "", err
	}
	return policy, encoded, targetID, targetName, nil
}

func (h *SaaSAdminHandler) executeSaaSAdminApproval(ctx context.Context, approval SaaSAdminApproval, actor User) (map[string]any, error) {
	if approval.ActionType == SaaSAdminApprovalActionDashboardTenantProvision || approval.ActionType == SaaSAdminApprovalActionDashboardActivationResend || approval.ActionType == SaaSAdminApprovalActionDashboardSuperAdminReplace || approval.ActionType == SaaSAdminApprovalActionDashboardSuperAdminStatus {
		if h.dashboardAdminApprovalExecutor == nil {
			return nil, errors.New("Dashboard 管理审批执行器未配置")
		}
		return h.dashboardAdminApprovalExecutor(ctx, actor.ID, approval.ID, approval.Version, approval.ActionType, json.RawMessage(approval.RequestJSON))
	}
	switch approval.ActionType {
	case SaaSAdminApprovalActionTenantDisable, SaaSAdminApprovalActionTenantEnable:
		var plan SaaSAdminTenantStatusApprovalPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		var input SaaSAdminTenantStatusUpdate
		if plan.SchemaVersion == SaaSAdminTenantStatusApprovalPlanSchemaVersion {
			input = plan.Update
			expectedStatus := 1
			if approval.ActionType == SaaSAdminApprovalActionTenantEnable {
				expectedStatus = 2
			}
			if plan.Snapshot.TenantID != input.TenantID || plan.Snapshot.TenantStatus != expectedStatus ||
				input.ExpectedStatus != expectedStatus || (approval.ActionType == SaaSAdminApprovalActionTenantEnable && input.Status != 1) ||
				(approval.ActionType == SaaSAdminApprovalActionTenantDisable && input.Status != 2) {
				return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
			}
			input.ApprovalPlan = &plan
		} else if approval.ActionType == SaaSAdminApprovalActionTenantDisable {
			if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil || input.Status != 2 {
				return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
			}
		} else {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := h.store.UpdateSaaSAdminTenantStatus(ctx, input)
		if err != nil {
			return nil, err
		}
		return saasAdminTenantStatusUpdatePayload(result), nil
	case SaaSAdminApprovalActionTenantProvision:
		var plan SaaSAdminTenantProvisionApprovalPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if (plan.Source != "direct" && plan.Source != "task") ||
			strings.TrimSpace(plan.CredentialFingerprint) == "" ||
			strings.TrimSpace(plan.Provision.AdminPasswordHash) == "" ||
			strings.TrimSpace(plan.Provision.PackageCode) == "" ||
			plan.Package.Version <= 0 ||
			plan.Package.Code != plan.Provision.PackageCode {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if plan.Source == "task" && (plan.TaskID <= 0 || plan.ExpectedTaskVersion <= 0 || strings.TrimSpace(plan.ExpectedTaskRequestSHA256) == "") {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input := plan.Provision
		input.ExpectedPackageVersion = plan.Package.Version
		input.TaskID = plan.TaskID
		input.ExpectedTaskVersion = plan.ExpectedTaskVersion
		input.ExpectedTaskRequestSHA256 = plan.ExpectedTaskRequestSHA256
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := h.store.ProvisionSaaSAdminTenant(ctx, input)
		if err != nil {
			return nil, err
		}
		payload := saasAdminTenantProvisionPayload(result)
		payload["source"] = plan.Source
		if plan.TaskID > 0 {
			payload["taskId"] = plan.TaskID
		}
		return payload, nil
	case SaaSAdminApprovalActionTenantRenewal:
		var plan SaaSAdminTenantRenewalApprovalPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if (plan.Source != "direct" && plan.Source != "task") ||
			plan.Renewal.TenantID <= 0 ||
			strings.TrimSpace(plan.Renewal.PackageCode) == "" ||
			strings.TrimSpace(plan.Renewal.ExpiresAt) == "" ||
			plan.ExpectedTenantStatus <= 0 ||
			plan.TargetPackage.Version <= 0 ||
			plan.TargetPackage.Code != plan.Renewal.PackageCode ||
			(plan.CurrentPackage != nil && plan.CurrentPackage.Version <= 0) ||
			(plan.Subscription.Exists && plan.Subscription.Version <= 0) ||
			(!plan.Subscription.Exists && plan.Subscription.Version != 0) {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if plan.Source == "task" && (plan.TaskID <= 0 || plan.ExpectedTaskVersion <= 0 || len(strings.TrimSpace(plan.ExpectedTaskRequestSHA256)) != 64) {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input := plan.Renewal
		input.ExpectedTenantStatus = plan.ExpectedTenantStatus
		input.ExpectedPackageVersion = plan.TargetPackage.Version
		if plan.CurrentPackage != nil {
			input.ExpectedPackageAssignmentVersion = plan.CurrentPackage.Version
		}
		input.ExpectedSubscriptionExists = plan.Subscription.Exists
		input.ExpectedSubscriptionVersion = plan.Subscription.Version
		input.TaskID = plan.TaskID
		input.ExpectedTaskVersion = plan.ExpectedTaskVersion
		input.ExpectedTaskRequestSHA256 = plan.ExpectedTaskRequestSHA256
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := h.store.RenewSaaSAdminTenant(ctx, input)
		if err != nil {
			return nil, err
		}
		payload := saasAdminTenantRenewalPayload(result)
		payload["source"] = plan.Source
		if plan.TaskID > 0 {
			payload["taskId"] = plan.TaskID
		}
		return payload, nil
	case SaaSAdminApprovalActionSubscriptionTransition:
		var plan SaaSAdminSubscriptionTransitionApprovalPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if plan.Subscription.ID <= 0 ||
			plan.Subscription.TenantID <= 0 ||
			plan.Subscription.TenantStatus <= 0 ||
			plan.Subscription.Version <= 0 ||
			plan.Transition.TenantID != plan.Subscription.TenantID ||
			plan.Transition.ExpectedVersion != plan.Subscription.Version ||
			!SaaSAdminSubscriptionStatusValid(plan.Subscription.Status) ||
			!SaaSAdminSubscriptionStatusValid(plan.Transition.Status) ||
			strings.TrimSpace(plan.Transition.IdempotencyKey) == "" {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input := plan.Transition
		input.ExpectedSubscriptionID = plan.Subscription.ID
		input.ExpectedTenantStatus = plan.Subscription.TenantStatus
		input.ExpectedVersion = plan.Subscription.Version
		input.Source = "approval"
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := h.store.TransitionSaaSAdminSubscription(ctx, input)
		if err != nil {
			return nil, err
		}
		return saasAdminSubscriptionTransitionResultPayload(result), nil
	case SaaSAdminApprovalActionPackageUpsert:
		var plan SaaSAdminPackageUpsertPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if err := normalizeSaaSAdminPackageUpsert(&plan.Update); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏: " + err.Error())
		}
		plan.Update.ActorUserID, plan.Update.ActorTenantID = actor.ID, actor.TenantID
		plan.Update.ApprovalExecutionID, plan.Update.ApprovalExecutionVersion = approval.ID, approval.Version
		pkg, err := h.store.UpsertSaaSAdminPackage(ctx, plan.Update)
		if err != nil {
			return nil, err
		}
		payload := saasAdminPackagePayload(pkg)
		payload["impact"] = saasAdminPackageImpactPayload(plan.Impact)
		return payload, nil
	case SaaSAdminApprovalActionTenantPackageUpdate:
		var plan SaaSAdminTenantPackageUpdatePlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if plan.Update.TenantID <= 0 || strings.TrimSpace(plan.Update.PackageCode) == "" ||
			plan.Update.ExpectedVersion < 0 || plan.Update.ExpectedPackageVersion <= 0 || plan.Update.ExpectedTenantStatus <= 0 {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		plan.Update.ActorUserID, plan.Update.ActorTenantID = actor.ID, actor.TenantID
		plan.Update.ApprovalExecutionID, plan.Update.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := h.store.UpdateSaaSAdminTenantPackage(ctx, plan.Update)
		if err != nil {
			return nil, err
		}
		payload := saasAdminTenantPackageUpdatePayload(result)
		payload["impact"] = saasAdminPackageImpactPayload(plan.Impact)
		return payload, nil
	case SaaSAdminApprovalActionPaymentOrderCreate:
		store, ok := h.store.(SaaSAdminPaymentStore)
		if !ok || store == nil {
			return nil, errors.New("payment store is not configured")
		}
		var plan SaaSAdminPaymentOrderCreateApprovalPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if plan.Create.OrderNo == "" || plan.Create.TenantID <= 0 || plan.Create.AmountCents <= 0 ||
			plan.Tenant.TenantID != plan.Create.TenantID || plan.Tenant.TenantStatus != 1 ||
			plan.Create.ExpectedTenantStatus != plan.Tenant.TenantStatus ||
			plan.Package.Code == "" || plan.Package.Code != plan.Create.PackageCode || plan.Package.Status != 1 || plan.Package.Version <= 0 ||
			plan.Create.ExpectedPackageVersion != plan.Package.Version {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input := plan.Create
		input.ExpectedTenantStatus = plan.Tenant.TenantStatus
		input.ExpectedPackageVersion = plan.Package.Version
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.CreateSaaSAdminPaymentOrder(ctx, input)
		if err != nil {
			return nil, err
		}
		return saasAdminPaymentOrderCreateResultPayload(result), nil
	case SaaSAdminApprovalActionPaymentRefundCreate:
		store, ok := h.store.(SaaSAdminPaymentRefundStore)
		if !ok {
			return nil, errors.New("payment refund store is not configured")
		}
		var input SaaSAdminPaymentRefundCreate
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.CreateSaaSAdminPaymentRefund(ctx, input)
		if err != nil {
			return nil, err
		}
		return saasAdminPaymentRefundCreateResultPayload(result), nil
	case SaaSAdminApprovalActionInvoiceIssue:
		store, ok := h.store.(SaaSInvoiceStore)
		if !ok || store == nil {
			return nil, errors.New("invoice store is not configured")
		}
		var plan SaaSInvoiceIssueApprovalPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if plan.Document.ID <= 0 || plan.Document.DocumentNo == "" || plan.Document.TenantID <= 0 ||
			plan.Document.PaymentOrderID <= 0 || plan.Document.OrderNo == "" ||
			plan.Document.Version <= 0 || plan.Document.AmountCents <= 0 ||
			plan.Order.ID != plan.Document.PaymentOrderID || plan.Order.OrderNo != plan.Document.OrderNo ||
			plan.Order.TenantID != plan.Document.TenantID || plan.Order.Version <= 0 ||
			plan.Order.Status != SaaSPaymentOrderStatusPaid ||
			plan.Transition.DocumentNo != plan.Document.DocumentNo ||
			plan.Transition.ExpectedVersion != plan.Document.Version ||
			plan.Transition.Status != SaaSInvoiceStatusIssued {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input := plan.Transition
		input.ApprovalPlan = &plan
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.TransitionSaaSInvoiceDocument(ctx, input)
		if err != nil {
			return nil, err
		}
		return saasInvoiceDocumentTransitionResultPayload(result), nil
	case SaaSAdminApprovalActionPaymentSettlementClose, SaaSAdminApprovalActionPaymentSettlementReopen:
		store, ok := h.store.(SaaSAdminPaymentSettlementStore)
		if !ok {
			return nil, errors.New("payment settlement store is not configured")
		}
		var plan SaaSPaymentSettlementTransitionApprovalPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		valid := plan.Batch.ID > 0 && plan.Batch.BatchNo != "" && plan.Batch.Version > 0 &&
			plan.Transition.BatchNo == plan.Batch.BatchNo && plan.Transition.ExpectedVersion == plan.Batch.Version
		if approval.ActionType == SaaSAdminApprovalActionPaymentSettlementClose {
			valid = valid && (plan.SchemaVersion == 0 || plan.SchemaVersion == SaaSPaymentSettlementApprovalPlanSchemaVersion) &&
				plan.Batch.Status == SaaSPaymentSettlementBatchStatusReconciled && plan.Batch.OpenIssueCount == 0 &&
				plan.Transition.Action == SaaSPaymentSettlementTransitionClose
		} else {
			valid = valid && plan.SchemaVersion == SaaSPaymentSettlementApprovalPlanSchemaVersion &&
				plan.Batch.Status == SaaSPaymentSettlementBatchStatusClosed && plan.Batch.ClosedByUserID > 0 &&
				plan.Batch.ClosedAt != "" && plan.Batch.CloseReason != "" &&
				plan.Transition.Action == SaaSPaymentSettlementTransitionReopen
		}
		if !valid {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input := plan.Transition
		input.ApprovalPlan = &plan
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.TransitionSaaSAdminPaymentSettlement(ctx, input)
		if err != nil {
			return nil, err
		}
		return map[string]any{"batch": saasPaymentSettlementBatchPayload(result.Batch), "previousStatus": result.PreviousStatus, "operationId": result.OperationID}, nil
	case SaaSAdminApprovalActionPaymentSettlementResolve:
		store, ok := h.store.(SaaSAdminPaymentSettlementStore)
		if !ok || store == nil {
			return nil, errors.New("payment settlement store is not configured")
		}
		var plan SaaSPaymentSettlementResolveApprovalPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		valid := plan.SchemaVersion == SaaSPaymentSettlementResolveApprovalPlanSchemaVersion &&
			plan.Entry.ID > 0 && plan.Entry.BatchID > 0 && plan.Entry.Version > 0 &&
			plan.Entry.BatchID == plan.Batch.ID && plan.Entry.BatchNo == plan.Batch.BatchNo &&
			plan.Entry.BatchStatus == plan.Batch.Status && plan.Batch.Status == SaaSPaymentSettlementBatchStatusReconciled &&
			plan.Resolve.EntryID == plan.Entry.ID && plan.Resolve.ExpectedVersion == plan.Entry.Version &&
			plan.Entry.ReconciliationStatus != SaaSPaymentSettlementReconciliationMatched &&
			saasPaymentSettlementResolveTransitionValid(plan.Entry.HandlingStatus, plan.Resolve.HandlingStatus) &&
			strings.TrimSpace(plan.Resolve.Reason) != ""
		if !valid {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input := plan.Resolve
		input.ApprovalPlan = &plan
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.ResolveSaaSAdminPaymentSettlementEntry(ctx, input)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"entry": saasPaymentSettlementEntryPayload(result.Entry), "batch": saasPaymentSettlementBatchPayload(result.Batch), "operationId": result.OperationID,
		}, nil
	case SaaSAdminApprovalActionTenantDomainCreate:
		store, ok := h.store.(SaaSAdminTenantDomainStore)
		if !ok || store == nil {
			return nil, errors.New("tenant domain store is not configured")
		}
		var request SaaSAdminTenantDomainRequest
		if err := json.Unmarshal([]byte(approval.RequestJSON), &request); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if err := normalizeAndValidateSaaSAdminTenantDomainRequest(&request); err != nil || request.Action != SaaSTenantDomainActionCreate {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		token, err := generateSaaSTenantDomainToken()
		if err != nil {
			return nil, errors.New("生成域名校验令牌失败")
		}
		result, err := store.CreateSaaSAdminTenantDomain(ctx, SaaSAdminTenantDomainCreate{
			TenantID: request.TenantID, Hostname: request.Hostname, Token: token,
			ActorUserID: actor.ID, ActorTenantID: actor.TenantID,
			ApprovalExecutionID: approval.ID, ApprovalExecutionVersion: approval.Version,
		})
		if err != nil {
			return nil, err
		}
		return saasTenantDomainResultPayload(result), nil
	case SaaSAdminApprovalActionTenantDomainCommand:
		store, ok := h.store.(SaaSAdminTenantDomainStore)
		if !ok || store == nil {
			return nil, errors.New("SaaS tenant domain store is not configured")
		}
		var plan SaaSTenantDomainCommandApprovalPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		valid := plan.SchemaVersion == SaaSTenantDomainApprovalPlanSchemaVersion &&
			plan.Command.ID > 0 && plan.Command.ID == plan.Domain.ID && plan.Command.ExpectedVersion == plan.Domain.Version &&
			plan.Domain.TenantID > 0 && plan.Domain.Hostname != "" && len(plan.RoutingDomains) > 0 && len(plan.RoutingSHA256) == sha256.Size*2 &&
			SaaSTenantDomainCommandRequiresApproval(plan.Command.Action)
		if !valid {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input := plan.Command
		if input.Action == SaaSTenantDomainActionRotateToken {
			token, err := generateSaaSTenantDomainToken()
			if err != nil {
				return nil, errors.New("生成域名校验令牌失败")
			}
			input.Token = token
		}
		input.ApprovalPlan = &plan
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.ApplySaaSAdminTenantDomainCommand(ctx, input)
		if err != nil {
			return nil, err
		}
		return saasTenantDomainResultPayload(result), nil
	case SaaSAdminApprovalActionAccessRoleSave:
		store, ok := h.store.(SaaSAdminAccessStore)
		if !ok {
			return nil, errors.New("SaaS admin access store is not configured")
		}
		var input SaaSAdminAccessRoleUpsert
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.UpsertSaaSAdminAccessRole(ctx, input)
		if err != nil {
			return nil, err
		}
		return map[string]any{"role": saasAdminAccessRolePayload(result.Role), "operationId": result.OperationID, "created": result.Created}, nil
	case SaaSAdminApprovalActionAccessAssignmentSave:
		store, ok := h.store.(SaaSAdminAccessStore)
		if !ok {
			return nil, errors.New("SaaS admin access store is not configured")
		}
		var input SaaSAdminAccessAssignmentUpdate
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.UpdateSaaSAdminAccessAssignment(ctx, h.platformAdminTenantID, input)
		if err != nil {
			return nil, err
		}
		return map[string]any{"assignment": saasAdminAccessAssignmentPayload(result.Assignment), "operationId": result.OperationID}, nil
	case SaaSAdminApprovalActionTenantDataErase:
		if h.complianceManager == nil {
			return nil, errors.New("SaaS compliance manager is not configured")
		}
		var input struct {
			RequestID int64 `json:"requestId"`
		}
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		request, err := h.complianceManager.AuthorizeErasure(ctx, saascompliance.ErasureAuthorize{
			RequestID: input.RequestID, ApprovalID: approval.ID, ApprovalVersion: approval.Version, ApprovalUserID: actor.ID,
			Actor: saascompliance.Actor{UserID: actor.ID, TenantID: actor.TenantID},
		})
		if err != nil {
			return nil, saasComplianceAdminError(err)
		}
		return map[string]any{"requestId": request.ID, "requestNo": request.RequestNo, "status": request.Status, "eligibleAt": request.EligibleAt}, nil
	case SaaSAdminApprovalActionReleaseCandidateGate:
		var payload SaaSReleaseCandidateApprovalPayload
		if err := json.Unmarshal([]byte(approval.RequestJSON), &payload); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		result, err := h.createSaaSReleaseCandidate(ctx, SaaSReleaseCandidateCreate{
			ReleaseVersion: payload.ReleaseVersion, SourceFingerprint: payload.SourceFingerprint,
			ApprovalExecutionID: approval.ID, ApprovalExecutionVersion: approval.Version,
		}, actor)
		if err != nil {
			return nil, err
		}
		return map[string]any{"candidate": saasReleaseCandidatePayload(result.Candidate), "operationId": result.OperationID}, nil
	case SaaSAdminApprovalActionApprovalPolicyUpdate:
		store, ok := h.store.(SaaSAdminApprovalGovernanceStore)
		if !ok {
			return nil, errors.New("SaaS admin approval governance store is not configured")
		}
		var input saasAdminApprovalPolicyBody
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		if err := validateSaaSAdminApprovalPolicyBody(&input); err != nil {
			return nil, err
		}
		result, err := store.UpdateSaaSAdminApprovalPolicy(ctx, SaaSAdminApprovalPolicyUpdate{
			ActionType: input.ActionType, Enabled: input.Enabled, AmountThresholdCents: input.AmountThresholdCents,
			RequiredApprovals: input.RequiredApprovals, SLAMinutes: input.SLAMinutes, ReminderMinutes: input.ReminderMinutes,
			ExpiryHours: input.ExpiryHours, ExpectedVersion: input.ExpectedVersion, ActorUserID: actor.ID, ActorTenantID: actor.TenantID,
			ApprovalExecutionID: approval.ID, ApprovalExecutionVersion: approval.Version,
		})
		if err != nil {
			return nil, err
		}
		policy, err := h.effectiveSaaSAdminApprovalPolicy(ctx, input.ActionType)
		if err != nil {
			return nil, err
		}
		policy.Version, policy.UpdatedBy, policy.UpdatedAt = result.Policy.Version, result.Policy.UpdatedBy, result.Policy.UpdatedAt
		return map[string]any{"policy": saasAdminApprovalPolicyPayload(policy), "operationId": result.OperationID}, nil
	case SaaSAdminApprovalActionBackupPolicyUpdate:
		if h.backupManager == nil {
			return nil, errors.New("SaaS backup manager is not configured")
		}
		var input saasbackup.PolicyUpdate
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input.Actor = saasbackup.Actor{UserID: actor.ID, TenantID: actor.TenantID}
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		policy, err := h.backupManager.UpdatePolicy(ctx, input)
		if err != nil {
			return nil, saasBackupAdminError(err)
		}
		return map[string]any{"policy": saasBackupPolicyPayload(policy)}, nil
	case SaaSAdminApprovalActionBackupRetentionCleanup:
		if h.backupManager == nil {
			return nil, errors.New("SaaS backup manager is not configured")
		}
		var plan saasbackup.CleanupPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		run, err := h.backupManager.ScheduleCleanup(ctx, plan, saasbackup.Actor{UserID: actor.ID, TenantID: actor.TenantID}, approval.ID, approval.Version)
		if err != nil {
			return nil, saasBackupAdminError(err)
		}
		return map[string]any{"cleanupRun": saasBackupCleanupRunPayload(run)}, nil
	case SaaSAdminApprovalActionCompliancePolicyUpdate:
		if h.complianceManager == nil {
			return nil, errors.New("SaaS compliance manager is not configured")
		}
		var input saascompliance.PolicyUpdate
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input.Actor = saascompliance.Actor{UserID: actor.ID, TenantID: actor.TenantID}
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		policy, err := h.complianceManager.UpdatePolicy(ctx, input)
		if err != nil {
			return nil, saasComplianceAdminError(err)
		}
		return map[string]any{"policy": saasCompliancePolicyPayload(policy)}, nil
	case SaaSAdminApprovalActionComplianceExportDelete:
		if h.complianceManager == nil {
			return nil, errors.New("SaaS compliance manager is not configured")
		}
		var plan saascompliance.DataExportDeletionPlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		item, err := h.complianceManager.ScheduleExportDeletion(ctx, plan,
			saascompliance.Actor{UserID: actor.ID, TenantID: actor.TenantID}, approval.ID, approval.Version)
		if err != nil {
			return nil, saasComplianceAdminError(err)
		}
		return map[string]any{"export": saasComplianceExportPayload(item)}, nil
	case SaaSAdminApprovalActionComplianceLegalHoldRelease:
		if h.complianceManager == nil {
			return nil, errors.New("SaaS compliance manager is not configured")
		}
		var plan saascompliance.LegalHoldReleasePlan
		if err := json.Unmarshal([]byte(approval.RequestJSON), &plan); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		hold, err := h.complianceManager.ReleaseLegalHoldPlan(ctx, plan,
			saascompliance.Actor{UserID: actor.ID, TenantID: actor.TenantID}, approval.ID, approval.Version)
		if err != nil {
			return nil, saasComplianceAdminError(err)
		}
		return map[string]any{"hold": saasComplianceLegalHoldPayload(hold)}, nil
	case SaaSAdminApprovalActionIdentityPolicyUpdate:
		if h.identitySecurityManager == nil {
			return nil, errors.New("SaaS identity security manager is not configured")
		}
		var input identitysecurity.PolicyUpdate
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input.Actor = identitysecurity.Actor{UserID: actor.ID, TenantID: actor.TenantID}
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		policy, err := h.identitySecurityManager.UpdatePolicy(ctx, input)
		if err != nil {
			return nil, identitySecurityAdminError(err)
		}
		return map[string]any{"policy": policy}, nil
	case SaaSAdminApprovalActionIdentityMFAReset:
		if h.identitySecurityManager == nil {
			return nil, errors.New("SaaS identity security manager is not configured")
		}
		var input identitysecurity.MFAReset
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input.Actor = identitysecurity.Actor{UserID: actor.ID, TenantID: actor.TenantID}
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := h.identitySecurityManager.ResetMFA(ctx, input)
		if err != nil {
			return nil, identitySecurityAdminError(err)
		}
		return map[string]any{
			"mfa": safeIdentityMFAPayload(result.Credential), "revokedSessions": result.RevokedSessions, "operationId": result.OperationID,
		}, nil
	case SaaSAdminApprovalActionServiceAccountCreate:
		store, ok := h.store.(SaaSServiceAccountStore)
		if !ok || store == nil {
			return nil, errors.New("service account store is not configured")
		}
		var input SaaSServiceAccountCreate
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		key, err := h.newSaaSServiceAccountKey()
		if err != nil {
			return nil, err
		}
		plainTextKey := key.PlainText
		key.PlainText = ""
		input.Key = key
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.CreateSaaSServiceAccount(ctx, input)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"account": saasServiceAccountPayload(result.Account), "key": saasServiceAccountKeyPayload(result.Key),
			"plainTextKey": plainTextKey, "initialKey": true, "operationId": result.OperationID,
		}, nil
	case SaaSAdminApprovalActionServiceAccountUpdate:
		store, ok := h.store.(SaaSServiceAccountStore)
		if !ok || store == nil {
			return nil, errors.New("service account store is not configured")
		}
		var input SaaSServiceAccountUpdate
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.UpdateSaaSServiceAccount(ctx, input)
		if err != nil {
			return nil, err
		}
		return map[string]any{"account": saasServiceAccountPayload(result.Account), "operationId": result.OperationID}, nil
	case SaaSAdminApprovalActionServiceAccountKeyRotate:
		store, ok := h.store.(SaaSServiceAccountStore)
		if !ok || store == nil {
			return nil, errors.New("service account store is not configured")
		}
		var input SaaSServiceAccountKeyRotate
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		key, err := h.newSaaSServiceAccountKey()
		if err != nil {
			return nil, err
		}
		plainTextKey := key.PlainText
		key.PlainText = ""
		input.Key = key
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.RotateSaaSServiceAccountKey(ctx, input)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"account": saasServiceAccountPayload(result.Account), "key": saasServiceAccountKeyPayload(result.Key),
			"plainTextKey": plainTextKey, "retiringKeys": result.RetiringKeys, "graceMinutes": input.GraceMinutes,
			"operationId": result.OperationID,
		}, nil
	case SaaSAdminApprovalActionServiceAccountKeyRevoke:
		store, ok := h.store.(SaaSServiceAccountStore)
		if !ok || store == nil {
			return nil, errors.New("service account store is not configured")
		}
		var input SaaSServiceAccountKeyRevoke
		if err := json.Unmarshal([]byte(approval.RequestJSON), &input); err != nil {
			return nil, NewSaaSAdminBadRequest("审批请求载荷损坏")
		}
		input.ActorUserID, input.ActorTenantID = actor.ID, actor.TenantID
		input.ApprovalExecutionID, input.ApprovalExecutionVersion = approval.ID, approval.Version
		result, err := store.RevokeSaaSServiceAccountKey(ctx, input)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"account": saasServiceAccountPayload(result.Account), "key": saasServiceAccountKeyPayload(result.Key), "operationId": result.OperationID,
		}, nil
	default:
		return nil, NewSaaSAdminBadRequest("未知审批动作")
	}
}

func (h *SaaSAdminHandler) rejectDirectHighRiskAction(ctx context.Context, w http.ResponseWriter, actionType string, amountCents int64) bool {
	if !h.highRiskApproval {
		return false
	}
	policy, err := h.effectiveSaaSAdminApprovalPolicy(ctx, actionType)
	if err != nil {
		writeSaaSAdminError(w, err)
		return true
	}
	if !saasAdminApprovalPolicyRequires(policy, amountCents) {
		return false
	}
	writeEnvelope(w, http.StatusPreconditionRequired, http.StatusPreconditionRequired, "高风险操作必须通过审批中心发起、复核并执行", map[string]any{
		"actionType": actionType, "requestPath": "/dashboard/saasAdmin/approvalRequest", "required": true,
		"requiredApprovals": policy.RequiredApprovals, "amountThresholdCents": policy.AmountThresholdCents,
	})
	return true
}

func saasAdminApprovalPolicyRequires(policy SaaSAdminApprovalPolicy, amountCents int64) bool {
	if !policy.Enabled {
		return false
	}
	if policy.ActionType == SaaSAdminApprovalActionPaymentRefundCreate && policy.AmountThresholdCents > 0 {
		return amountCents >= policy.AmountThresholdCents
	}
	return true
}

func saasAdminApprovalAmountCents(actionType string, normalized []byte) int64 {
	if actionType != SaaSAdminApprovalActionPaymentRefundCreate {
		return 0
	}
	var input SaaSAdminPaymentRefundCreate
	if json.Unmarshal(normalized, &input) != nil {
		return 0
	}
	return input.AmountCents
}

func (h *SaaSAdminHandler) saasAdminApprovalStore(w http.ResponseWriter) (SaaSAdminApprovalStore, bool) {
	store, ok := h.store.(SaaSAdminApprovalStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "SaaS admin approval store is not configured", nil)
		return nil, false
	}
	return store, true
}

func parseSaaSAdminApprovalOptions(r *http.Request) (SaaSAdminApprovalOptions, error) {
	approvalID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("approvalId")), 10, 64)
	requesterID, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("requesterUserId")))
	reviewerID, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("reviewerUserId")))
	executionID, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("executionUserId")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = SaaSAdminApprovalStatusAll
	}
	if !saasAdminApprovalStatusValid(status, true) {
		return SaaSAdminApprovalOptions{}, errors.New("status 无效")
	}
	risk := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("riskLevel")))
	if risk == "" {
		risk = SaaSAdminApprovalRiskAll
	}
	if risk != SaaSAdminApprovalRiskAll && risk != SaaSAdminApprovalRiskHigh && risk != SaaSAdminApprovalRiskCritical {
		return SaaSAdminApprovalOptions{}, errors.New("riskLevel 必须是 all、high 或 critical")
	}
	action := strings.TrimSpace(r.URL.Query().Get("actionType"))
	if action != "" {
		if _, ok := SaaSAdminApprovalPolicyByAction(action); !ok {
			return SaaSAdminApprovalOptions{}, errors.New("actionType 无效")
		}
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		return SaaSAdminApprovalOptions{}, errors.New("keyword 最多 80 个字符")
	}
	limit := positiveQueryInt(r, "limit", 100)
	if limit > 500 {
		limit = 500
	}
	return SaaSAdminApprovalOptions{ApprovalID: approvalID, Status: status, ActionType: action, RiskLevel: risk, RequesterID: requesterID, ReviewerID: reviewerID, ExecutionID: executionID, Keyword: keyword, Limit: limit}, nil
}

func saasAdminApprovalStatusValid(status string, allowAll bool) bool {
	switch strings.TrimSpace(status) {
	case SaaSAdminApprovalStatusPending, SaaSAdminApprovalStatusApproved, SaaSAdminApprovalStatusRejected, SaaSAdminApprovalStatusCanceled, SaaSAdminApprovalStatusExpired, SaaSAdminApprovalStatusExecuting, SaaSAdminApprovalStatusExecuted:
		return true
	case SaaSAdminApprovalStatusAll:
		return allowAll
	default:
		return false
	}
}

func newSaaSAdminApprovalRequestNo() (string, error) {
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("APR-%s-%s", time.Now().UTC().Format("20060102T150405"), strings.ToUpper(hex.EncodeToString(random[:]))), nil
}

func approvalExecutionError(err error) string {
	if err == nil {
		return ""
	}
	return truncateApprovalText(err.Error(), 2000)
}

func truncateApprovalText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func saasAdminApprovalPolicyPayloads(items []SaaSAdminApprovalPolicy) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasAdminApprovalPolicyPayload(item))
	}
	return result
}

func saasAdminApprovalPayload(item SaaSAdminApproval) map[string]any {
	return map[string]any{
		"id": item.ID, "requestNo": item.RequestNo, "actionType": item.ActionType, "riskLevel": item.RiskLevel, "status": item.Status,
		"requiredPermission": item.RequiredPermission, "policyVersion": item.PolicyVersion, "requiredApprovals": item.RequiredApprovals,
		"approvalCount": item.ApprovalCount, "reminderMinutes": item.ReminderMinutes, "targetType": item.TargetType, "targetId": item.TargetID, "targetName": item.TargetName,
		"requesterUserId": item.RequesterUserID, "requesterTenantId": item.RequesterTenantID, "requesterName": item.RequesterName,
		"idempotencyKey": item.IdempotencyKey, "requestSha256": item.RequestSHA256, "request": saasAdminApprovalRequestPayload(item), "reason": item.Reason,
		"reviewerUserId": item.ReviewerUserID, "reviewerName": item.ReviewerName, "reviewedAt": item.ReviewedAt, "decisionReason": item.DecisionReason,
		"slaDueAt": item.SLADueAt, "nextReminderAt": item.NextReminderAt, "lastRemindedAt": item.LastRemindedAt, "reminderCount": item.ReminderCount,
		"executionUserId": item.ExecutionUserID, "executionUserName": item.ExecutionUserName, "executionStartedAt": item.ExecutionStartedAt,
		"effectAppliedAt": item.EffectAppliedAt, "effectOperationId": item.EffectOperationID,
		"executedAt": item.ExecutedAt, "executionAttempts": item.ExecutionAttempts, "result": approvalJSONPayload(item.ResultJSON), "lastError": item.LastError,
		"expiresAt": item.ExpiresAt, "version": item.Version, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasAdminApprovalRequestPayload(item SaaSAdminApproval) any {
	if item.ActionType != SaaSAdminApprovalActionTenantProvision {
		return json.RawMessage(item.RequestJSON)
	}
	request := approvalJSONPayload(item.RequestJSON)
	plan, ok := request.(map[string]any)
	if !ok {
		return map[string]any{"redacted": true}
	}
	delete(plan, "credentialFingerprint")
	provision, ok := plan["provision"].(map[string]any)
	if !ok {
		plan["provision"] = map[string]any{"redacted": true}
		return plan
	}
	hasPasswordHash := false
	for _, key := range []string{"adminPasswordHash", "admin_password_hash", "passwordHash", "password_hash"} {
		if _, exists := provision[key]; exists {
			hasPasswordHash = true
			delete(provision, key)
		}
	}
	delete(provision, "password")
	if hasPasswordHash {
		provision["hasAdminPasswordHash"] = true
	}
	return plan
}

func saasAdminApprovalReportPayload(report SaaSAdminApprovalReport) map[string]any {
	items := make([]map[string]any, 0, len(report.Items))
	for _, item := range report.Items {
		items = append(items, saasAdminApprovalPayload(item))
	}
	return map[string]any{
		"summary": map[string]any{
			"total": report.Summary.Total, "pending": report.Summary.Pending, "approved": report.Summary.Approved,
			"rejected": report.Summary.Rejected, "canceled": report.Summary.Canceled, "expired": report.Summary.Expired,
			"executing": report.Summary.Executing, "executed": report.Summary.Executed, "critical": report.Summary.Critical,
		},
		"items": items, "returnedCount": len(items),
	}
}

func saasAdminApprovalEventPayloads(events []SaaSAdminApprovalEvent) []map[string]any {
	items := make([]map[string]any, 0, len(events))
	for _, event := range events {
		items = append(items, map[string]any{
			"id": event.ID, "approvalId": event.ApprovalID, "eventType": event.EventType,
			"fromStatus": event.FromStatus, "toStatus": event.ToStatus, "actorUserId": event.ActorUserID,
			"actorTenantId": event.ActorTenantID, "actorName": event.ActorName, "reason": event.Reason,
			"context": approvalJSONPayload(event.ContextJSON), "createdAt": event.CreatedAt,
		})
	}
	return items
}

func approvalJSONPayload(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return raw
	}
	return value
}
