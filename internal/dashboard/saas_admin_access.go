package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

const (
	SaaSAdminPermissionOverviewRead        = "platform.overview.read"
	SaaSAdminPermissionTenantsRead         = "platform.tenants.read"
	SaaSAdminPermissionTenantsManage       = "platform.tenants.manage"
	SaaSAdminPermissionOperationsRead      = "platform.operations.read"
	SaaSAdminPermissionOperationsManage    = "platform.operations.manage"
	SaaSAdminPermissionNotificationsRead   = "platform.notifications.read"
	SaaSAdminPermissionNotificationsManage = "platform.notifications.manage"
	SaaSAdminPermissionFinanceRead         = "platform.finance.read"
	SaaSAdminPermissionFinanceManage       = "platform.finance.manage"
	SaaSAdminPermissionAuditRead           = "platform.audit.read"
	SaaSAdminPermissionAuditManage         = "platform.audit.manage"
	SaaSAdminPermissionAccessManage        = "platform.access.manage"
	SaaSAdminPermissionApprovalsRead       = "platform.approvals.read"
	SaaSAdminPermissionApprovalsReview     = "platform.approvals.review"
	SaaSAdminPermissionApprovalsExecute    = "platform.approvals.execute"
	SaaSAdminPermissionApprovalsManage     = "platform.approvals.manage"
	SaaSAdminPermissionSystemRead          = "platform.system.read"
	SaaSAdminPermissionSystemManage        = "platform.system.manage"
	SaaSAdminPermissionIntegrationsRead    = "platform.integrations.read"
	SaaSAdminPermissionIntegrationsManage  = "platform.integrations.manage"
	SaaSAdminPermissionBackupsRead         = "platform.backups.read"
	SaaSAdminPermissionBackupsManage       = "platform.backups.manage"
	SaaSAdminPermissionComplianceRead      = "platform.compliance.read"
	SaaSAdminPermissionComplianceManage    = "platform.compliance.manage"
	SaaSAdminPermissionIdentityRead        = "platform.identity.read"
	SaaSAdminPermissionIdentityManage      = "platform.identity.manage"
	SaaSAdminPermissionBrandingRead        = "platform.branding.read"
	SaaSAdminPermissionBrandingManage      = "platform.branding.manage"
	SaaSAdminPermissionDomainsRead         = "platform.domains.read"
	SaaSAdminPermissionDomainsManage       = "platform.domains.manage"
	SaaSAdminPermissionReleaseRead         = "platform.release.read"
	SaaSAdminPermissionReleaseManage       = "platform.release.manage"

	SaaSAdminOperationActionAccessRoleSave       = "saas.admin.access.role.save"
	SaaSAdminOperationActionAccessAssignmentSave = "saas.admin.access.assignment.save"
	SaaSAdminOperationTargetAccessRole           = "saas_admin_role"
	SaaSAdminOperationTargetAccessAssignment     = "saas_admin_user_access"
)

var saasAdminAccessRoleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,47}$`)
var saasAdminTenantGovernanceRoutePattern = regexp.MustCompile(`^tenants/(?:\{tenantId\}|[1-9][0-9]*)/(?:activation/resend|super-admin/replace|super-admin/status|dashboard-admins)$`)

type SaaSAdminPermissionDefinition struct {
	Code        string
	Name        string
	Category    string
	Description string
	Write       bool
}

type SaaSAdminAccessRole struct {
	ID              int64
	Code            string
	Name            string
	Description     string
	Status          int
	IsSystem        bool
	Version         int
	Permissions     []string
	AssignmentCount int
	CreatedBy       int
	UpdatedBy       int
	CreatedAt       string
	UpdatedAt       string
}

type SaaSAdminAccessProfile struct {
	UserID               int
	UserName             string
	Phone                string
	TenantID             int
	IsPlatformSuperAdmin bool
	Version              int
	Roles                []SaaSAdminAccessRole
	Permissions          []string
}

type SaaSAdminAccessAssignment struct {
	UserID       int
	UserName     string
	Phone        string
	Status       int
	IsSuperAdmin bool
	Version      int
	Roles        []SaaSAdminAccessRole
	Permissions  []string
	UpdatedBy    int
	UpdatedAt    string
}

type SaaSAdminAccessAssignmentOptions struct {
	Keyword string
	Limit   int
}

type SaaSAdminAccessRoleUpsert struct {
	ID                       int64    `json:"id"`
	Code                     string   `json:"code"`
	Name                     string   `json:"name"`
	Description              string   `json:"description"`
	Status                   int      `json:"status"`
	ExpectedVersion          int      `json:"expectedVersion"`
	Permissions              []string `json:"permissions"`
	ActorUserID              int      `json:"-"`
	ActorTenantID            int      `json:"-"`
	ApprovalExecutionID      int64    `json:"-"`
	ApprovalExecutionVersion int      `json:"-"`
}

type SaaSAdminAccessRoleUpsertResult struct {
	Role        SaaSAdminAccessRole
	OperationID int64
	Created     bool
}

type SaaSAdminAccessAssignmentUpdate struct {
	UserID                   int     `json:"userId"`
	RoleIDs                  []int64 `json:"roleIds"`
	ExpectedVersion          int     `json:"expectedVersion"`
	ActorUserID              int     `json:"-"`
	ActorTenantID            int     `json:"-"`
	ApprovalExecutionID      int64   `json:"-"`
	ApprovalExecutionVersion int     `json:"-"`
}

type SaaSAdminAccessAssignmentUpdateResult struct {
	Assignment  SaaSAdminAccessAssignment
	OperationID int64
}

type SaaSAdminAccessStore interface {
	SaaSAdminAccessProfile(ctx context.Context, userID int, platformTenantID int) (SaaSAdminAccessProfile, error)
	SaaSAdminAccessRoles(ctx context.Context) ([]SaaSAdminAccessRole, error)
	UpsertSaaSAdminAccessRole(ctx context.Context, input SaaSAdminAccessRoleUpsert) (SaaSAdminAccessRoleUpsertResult, error)
	SaaSAdminAccessAssignments(ctx context.Context, platformTenantID int, options SaaSAdminAccessAssignmentOptions) ([]SaaSAdminAccessAssignment, error)
	UpdateSaaSAdminAccessAssignment(ctx context.Context, platformTenantID int, input SaaSAdminAccessAssignmentUpdate) (SaaSAdminAccessAssignmentUpdateResult, error)
}

func SaaSAdminPermissionCatalog() []SaaSAdminPermissionDefinition {
	return []SaaSAdminPermissionDefinition{
		{Code: SaaSAdminPermissionOverviewRead, Name: "平台总览", Category: "经营", Description: "查看经营指标、趋势和日报"},
		{Code: SaaSAdminPermissionTenantsRead, Name: "租户只读", Category: "租户", Description: "查看租户、套餐、用量和生命周期"},
		{Code: SaaSAdminPermissionTenantsManage, Name: "租户管理", Category: "租户", Description: "开户、停启、续费、套餐和订阅状态管理", Write: true},
		{Code: SaaSAdminPermissionOperationsRead, Name: "运营只读", Category: "运营", Description: "查看任务、风险、客户成功和待办"},
		{Code: SaaSAdminPermissionOperationsManage, Name: "运营处置", Category: "运营", Description: "分派、跟进、重置、关闭和生成提醒", Write: true},
		{Code: SaaSAdminPermissionNotificationsRead, Name: "通知只读", Category: "通知", Description: "查看告警、通知健康、SLO 和策略"},
		{Code: SaaSAdminPermissionNotificationsManage, Name: "通知处置", Category: "通知", Description: "解决告警、重试、关闭和调整通知策略", Write: true},
		{Code: SaaSAdminPermissionFinanceRead, Name: "财务只读", Category: "财务", Description: "查看订阅、收款、退款、发票和结算"},
		{Code: SaaSAdminPermissionFinanceManage, Name: "财务管理", Category: "财务", Description: "处理订单、退款、发票、对账和结算同步", Write: true},
		{Code: SaaSAdminPermissionAuditRead, Name: "审计查看", Category: "安全", Description: "查看并导出平台操作审计"},
		{Code: SaaSAdminPermissionAuditManage, Name: "审计治理", Category: "安全", Description: "执行审计完整性封存和校验", Write: true},
		{Code: SaaSAdminPermissionAccessManage, Name: "权限治理", Category: "安全", Description: "维护平台岗位和人员授权", Write: true},
		{Code: SaaSAdminPermissionApprovalsRead, Name: "审批查看", Category: "审批", Description: "查看高风险审批策略、申请和事件"},
		{Code: SaaSAdminPermissionApprovalsReview, Name: "审批复核", Category: "审批", Description: "批准或驳回他人发起的高风险操作", Write: true},
		{Code: SaaSAdminPermissionApprovalsExecute, Name: "审批执行", Category: "审批", Description: "执行已经批准的高风险操作", Write: true},
		{Code: SaaSAdminPermissionApprovalsManage, Name: "审批治理", Category: "审批", Description: "维护审批策略、委托和 SLA 提醒", Write: true},
		{Code: SaaSAdminPermissionSystemRead, Name: "系统健康", Category: "安全", Description: "查看平台健康检查、扫描和系统事故"},
		{Code: SaaSAdminPermissionSystemManage, Name: "事故处置", Category: "安全", Description: "执行健康扫描并认领、分派、解决或重开事故", Write: true},
		{Code: SaaSAdminPermissionIntegrationsRead, Name: "集成读取", Category: "安全", Description: "查看服务账号、API Key 状态和使用记录"},
		{Code: SaaSAdminPermissionIntegrationsManage, Name: "集成管理", Category: "安全", Description: "创建、更新服务账号并轮换或吊销 API Key", Write: true},
		{Code: SaaSAdminPermissionBackupsRead, Name: "灾备查看", Category: "安全", Description: "查看备份策略、工件校验和恢复演练结果"},
		{Code: SaaSAdminPermissionBackupsManage, Name: "灾备管理", Category: "安全", Description: "维护策略、创建和校验备份、执行隔离恢复演练", Write: true},
		{Code: SaaSAdminPermissionComplianceRead, Name: "合规查看", Category: "安全", Description: "查看租户数据策略、法律保留、导出和擦除证明"},
		{Code: SaaSAdminPermissionComplianceManage, Name: "合规管理", Category: "安全", Description: "维护法律保留、生成加密导出并发起或处理已批准的擦除", Write: true},
		{Code: SaaSAdminPermissionIdentityRead, Name: "身份安全查看", Category: "安全", Description: "查看登录策略、账号锁定、会话、登录事件和安全事故"},
		{Code: SaaSAdminPermissionIdentityManage, Name: "身份安全管理", Category: "安全", Description: "维护登录策略、解锁账号、撤销会话、重置 MFA 和处置安全事故", Write: true},
		{Code: SaaSAdminPermissionBrandingRead, Name: "品牌配置查看", Category: "平台", Description: "查看平台与租户白标品牌、登录页和支持入口"},
		{Code: SaaSAdminPermissionBrandingManage, Name: "品牌配置管理", Category: "平台", Description: "维护产品名称、同源品牌素材、颜色、文档和支持入口", Write: true},
		{Code: SaaSAdminPermissionDomainsRead, Name: "租户域名查看", Category: "平台", Description: "查看租户自定义域名、DNS 所有权校验和主域名状态"},
		{Code: SaaSAdminPermissionDomainsManage, Name: "租户域名管理", Category: "平台", Description: "创建、校验、启停、切换主域名和轮换 DNS 校验令牌", Write: true},
		{Code: SaaSAdminPermissionReleaseRead, Name: "发布准备查看", Category: "平台", Description: "查看生产证据、源码指纹和发布候选门禁快照"},
		{Code: SaaSAdminPermissionReleaseManage, Name: "发布准备管理", Category: "平台", Description: "维护生产证据并运行严格发布门禁", Write: true},
	}
}

func SaaSAdminPermissionValid(code string) bool {
	code = strings.TrimSpace(code)
	for _, item := range SaaSAdminPermissionCatalog() {
		if item.Code == code {
			return true
		}
	}
	return false
}

func SaaSAdminAccessHasPermission(profile SaaSAdminAccessProfile, permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false
	}
	if profile.IsPlatformSuperAdmin {
		return true
	}
	for _, code := range profile.Permissions {
		code = strings.TrimSpace(code)
		if code == permission || code == "*" {
			return true
		}
	}
	return false
}

func SaaSAdminRequiredPermission(r *http.Request) string {
	if r == nil {
		return ""
	}
	name := strings.TrimPrefix(r.URL.Path, "/dashboard/saasAdmin/")
	write := r.Method != http.MethodGet && r.Method != http.MethodHead
	if name == "export" {
		return saasAdminExportPermission(r.URL.Query().Get("type"))
	}
	if name == "tenants/provision" || saasAdminTenantGovernanceRoutePattern.MatchString(name) {
		return SaaSAdminPermissionTenantsManage
	}
	if name == "accessProfile" {
		return SaaSAdminPermissionOverviewRead
	}
	if name == "accessRoles" || name == "accessRole" || name == "accessAssignments" || name == "accessAssignment" {
		return SaaSAdminPermissionAccessManage
	}
	if name == "auditIntegrity" || name == "auditAnchors" {
		return SaaSAdminPermissionAuditRead
	}
	if name == "auditIntegrityVerify" || name == "auditAnchor" {
		return SaaSAdminPermissionAuditManage
	}
	if name == "approvalPolicies" || name == "approvals" || name == "approvalEvents" || name == "approvalDecisions" || name == "approvalDelegations" || name == "approvalRequest" || name == "approvalCancel" {
		return SaaSAdminPermissionApprovalsRead
	}
	if name == "approvalDecision" {
		return SaaSAdminPermissionApprovalsReview
	}
	if name == "approvalExecute" {
		return SaaSAdminPermissionApprovalsExecute
	}
	if name == "approvalPolicy" || name == "approvalDelegation" || name == "approvalReminders" {
		return SaaSAdminPermissionApprovalsManage
	}
	if name == "systemHealth" || name == "systemHealthScans" || name == "systemIncidents" {
		return SaaSAdminPermissionSystemRead
	}
	if name == "systemHealthScan" || name == "systemIncident" {
		return SaaSAdminPermissionSystemManage
	}
	if name == "serviceAccounts" || name == "serviceAccountUsage" || name == "wecomCredentialProtection" || name == "wechatOpenCredentialProtection" {
		return SaaSAdminPermissionIntegrationsRead
	}
	if name == "serviceAccount" || name == "serviceAccountKeyRotate" || name == "serviceAccountKeyRevoke" || name == "serviceAccountUsageAlertEvaluate" || name == "wecomCredentialRotation" || name == "wechatOpenCredentialRotation" {
		return SaaSAdminPermissionIntegrationsManage
	}
	if name == "backupOverview" {
		return SaaSAdminPermissionBackupsRead
	}
	if name == "backupPolicy" || name == "backupRun" || name == "restoreDrill" {
		return SaaSAdminPermissionBackupsManage
	}
	if name == "complianceOverview" || name == "complianceErasureSteps" {
		return SaaSAdminPermissionComplianceRead
	}
	if name == "compliancePolicy" || name == "complianceLegalHold" || name == "complianceExport" || name == "complianceExportDownload" || name == "complianceErasure" {
		return SaaSAdminPermissionComplianceManage
	}
	if name == "identityOverview" || name == "identitySessions" || name == "identityLoginEvents" || name == "identityIncidents" {
		return SaaSAdminPermissionIdentityRead
	}
	if name == "identityPolicy" || name == "identitySession" || name == "identityIncident" || name == "identityUser" || name == "identityMFA" {
		return SaaSAdminPermissionIdentityManage
	}
	if name == "brandingProfiles" || (name == "brandingProfile" && !write) {
		return SaaSAdminPermissionBrandingRead
	}
	if name == "brandingProfile" && write {
		return SaaSAdminPermissionBrandingManage
	}
	if name == "tenantDomains" || name == "tenantDomainDeliveryJobs" {
		return SaaSAdminPermissionDomainsRead
	}
	if name == "tenantDomain" || name == "tenantDomainDelivery" {
		return SaaSAdminPermissionDomainsManage
	}
	if name == "releaseReadiness" {
		return SaaSAdminPermissionReleaseRead
	}
	if name == "releaseEvidence" || name == "releaseEvidenceAction" || name == "releaseCandidate" {
		return SaaSAdminPermissionReleaseManage
	}
	if name == "overview" || name == "businessMetrics" || name == "businessTrends" || name == "dailyReport" {
		return SaaSAdminPermissionOverviewRead
	}
	if saasAdminRouteIn(name, "tenant", "tenantLifecycle", "tenantReadiness", "usage", "packages") {
		return SaaSAdminPermissionTenantsRead
	}
	if saasAdminRouteIn(name, "package", "packageSync", "packageSyncTask", "packageSyncTaskApply", "packageSyncTaskBulkApply", "tenantStatus", "tenantRenewal", "tenantRenewalTask", "tenantRenewalTaskApply", "tenantRenewalTaskBulkApply", "tenantProvision", "tenantProvisionTask", "tenantProvisionTaskApply", "tenantProvisionTaskBulkApply", "tenantPackage") {
		return SaaSAdminPermissionTenantsManage
	}
	if saasAdminRouteIn(name, "risk", "operationQueue", "operationQueueOwners", "operationQueueAssignments", "renewalForecast", "customerSuccess", "customerSuccessOwners", "riskFollowUps", "riskFollowUpOwners", "tasks", "taskOwners", "taskSla") {
		return SaaSAdminPermissionOperationsRead
	}
	if saasAdminRouteIn(name, "operationQueueAssignmentClose", "operationQueueAssignmentNotifications", "operationQueueAssign", "renewalForecastTasks", "renewalForecastAssign", "renewalForecastNotifications", "customerSuccessAssign", "customerSuccessRenewalTasks", "customerSuccessRenewalNotifications", "riskFollowUp", "riskFollowUpBulkClose", "taskSlaNotifications", "taskCancel", "taskBulkCancel", "taskBulkReset", "taskReset") {
		return SaaSAdminPermissionOperationsManage
	}
	if saasAdminRouteIn(name, "alerts", "notifications", "notificationHealth", "notificationSlo", "notificationPolicies") || (name == "notificationPolicy" && !write) {
		return SaaSAdminPermissionNotificationsRead
	}
	if saasAdminRouteIn(name, "alertResolve", "alertBulkResolve", "notificationHealthRecovery", "notificationPolicyTest", "notificationCredentialRotation", "notificationRetry", "notificationBulkRetry", "notificationClose", "notificationBulkClose") || (name == "notificationPolicy" && write) {
		return SaaSAdminPermissionNotificationsManage
	}
	if saasAdminRouteIn(name, "subscriptions", "subscriptionEvents", "paymentOrders", "paymentWebhookEvents", "paymentRefunds", "paymentSettlementBatches", "paymentSettlementEntries", "paymentSettlementSyncRuns", "invoiceDocuments", "billingEvents", "billingReconciliation", "billingReconciliationFollowUps", "billingReconciliationFollowUpOwners") || (name == "invoiceProfile" && !write) {
		return SaaSAdminPermissionFinanceRead
	}
	if saasAdminRouteIn(name, "subscriptionTransition", "subscriptionReconcile", "paymentOrder", "paymentOrderCancel", "paymentDunning", "paymentRefund", "paymentRefundCancel", "paymentSettlementImport", "paymentSettlementReconcile", "paymentSettlementResolve", "paymentSettlementTransition", "paymentSettlementSync", "invoice", "creditNote", "invoiceTransition", "billingReconciliationFollowUp", "billingReconciliationFollowUpBulkClose") || (name == "invoiceProfile" && write) {
		return SaaSAdminPermissionFinanceManage
	}
	if name == "operations" {
		return SaaSAdminPermissionAuditRead
	}
	return ""
}

func saasAdminExportPermission(kind string) string {
	switch strings.TrimSpace(kind) {
	case SaaSAdminExportKindTenants, SaaSAdminExportKindTenantLifecycle, SaaSAdminExportKindUsage, SaaSAdminExportKindPackages:
		return SaaSAdminPermissionTenantsRead
	case SaaSAdminExportKindBusinessMetrics, SaaSAdminExportKindBusinessTrends, SaaSAdminExportKindDailyReport:
		return SaaSAdminPermissionOverviewRead
	case SaaSAdminExportKindRisk, SaaSAdminExportKindOperationQueue, SaaSAdminExportKindOperationOwners, SaaSAdminExportKindOperationAssigns,
		SaaSAdminExportKindRenewalForecast, SaaSAdminExportKindRenewalOwners, SaaSAdminExportKindCustomerSuccess, SaaSAdminExportKindCustomerOwners,
		SaaSAdminExportKindRiskFollowUps, SaaSAdminExportKindRiskOwners, SaaSAdminExportKindTasks, SaaSAdminExportKindTaskSLA:
		return SaaSAdminPermissionOperationsRead
	case SaaSAdminExportKindAlerts, SaaSAdminExportKindNotifications, SaaSAdminExportKindNotificationHealth, SaaSAdminExportKindNotificationSLO:
		return SaaSAdminPermissionNotificationsRead
	case SaaSAdminExportKindBillingEvents, SaaSAdminExportKindBillingReconcile, SaaSAdminExportKindBillingFollowUps, SaaSAdminExportKindBillingOwners,
		SaaSAdminExportKindSubscriptions, SaaSAdminExportKindPaymentOrders, SaaSAdminExportKindPaymentRefunds, SaaSAdminExportKindInvoiceDocuments,
		SaaSAdminExportKindPaymentSettlementBatches, SaaSAdminExportKindPaymentSettlementEntries:
		return SaaSAdminPermissionFinanceRead
	case SaaSAdminExportKindOperations:
		return SaaSAdminPermissionAuditRead
	default:
		return ""
	}
}

func saasAdminRouteIn(value string, items ...string) bool {
	for _, item := range items {
		if value == item {
			return true
		}
	}
	return false
}

func (h *SaaSAdminHandler) AccessProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	profile, err := h.saasAdminAccessProfile(r.Context(), user)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"profile":     saasAdminAccessProfilePayload(profile),
		"permissions": saasAdminPermissionCatalogPayload(),
	})
}

func (h *SaaSAdminHandler) AccessRoles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminAccessStore(w)
	if !ok {
		return
	}
	roles, err := store.SaaSAdminAccessRoles(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"roles":       saasAdminAccessRolePayloads(roles),
		"permissions": saasAdminPermissionCatalogPayload(),
	})
}

func (h *SaaSAdminHandler) AccessRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminAccessStore(w)
	if !ok {
		return
	}
	var input SaaSAdminAccessRoleUpsert
	if err := decodeSaaSAdminAccessJSON(r, &input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := validateSaaSAdminAccessRoleUpsert(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if user.IsSuperAdmin != 1 && input.ID > 0 {
		profile, err := h.saasAdminAccessProfile(r.Context(), user)
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		for _, role := range profile.Roles {
			if role.ID == input.ID {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "非超级管理员不能修改自己持有的平台岗位", nil)
				return
			}
		}
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionAccessRoleSave, 0) {
		return
	}
	input.ActorUserID = user.ID
	input.ActorTenantID = user.TenantID
	result, err := store.UpsertSaaSAdminAccessRole(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"role": saasAdminAccessRolePayload(result.Role), "operationId": result.OperationID, "created": result.Created,
	})
}

func (h *SaaSAdminHandler) AccessAssignments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	store, ok := h.saasAdminAccessStore(w)
	if !ok {
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "keyword 最多 80 个字符", nil)
		return
	}
	limit := positiveQueryInt(r, "limit", 100)
	if limit > saasAdminListMaxLimit {
		limit = saasAdminListMaxLimit
	}
	items, err := store.SaaSAdminAccessAssignments(r.Context(), h.platformAdminTenantID, SaaSAdminAccessAssignmentOptions{Keyword: keyword, Limit: limit})
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"assignments": saasAdminAccessAssignmentPayloads(items)})
}

func (h *SaaSAdminHandler) AccessAssignment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasAdminAccessStore(w)
	if !ok {
		return
	}
	var input SaaSAdminAccessAssignmentUpdate
	if err := decodeSaaSAdminAccessJSON(r, &input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input.RoleIDs = uniquePositiveInt64s(input.RoleIDs)
	if input.UserID <= 0 || input.ExpectedVersion < 0 || len(input.RoleIDs) > 20 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "userId、expectedVersion 或 roleIds 格式错误", nil)
		return
	}
	if input.UserID == user.ID && user.IsSuperAdmin != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "非超级管理员不能修改自己的平台授权", nil)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionAccessAssignmentSave, 0) {
		return
	}
	input.ActorUserID = user.ID
	input.ActorTenantID = user.TenantID
	result, err := store.UpdateSaaSAdminAccessAssignment(r.Context(), h.platformAdminTenantID, input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{
		"assignment": saasAdminAccessAssignmentPayload(result.Assignment), "operationId": result.OperationID,
	})
}

func (h *SaaSAdminHandler) saasAdminAccessStore(w http.ResponseWriter) (SaaSAdminAccessStore, bool) {
	store, ok := h.store.(SaaSAdminAccessStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "SaaS admin access store is not configured", nil)
		return nil, false
	}
	return store, true
}

func (h *SaaSAdminHandler) saasAdminAccessProfile(ctx context.Context, user User) (SaaSAdminAccessProfile, error) {
	if user.TenantID != h.platformAdminTenantID {
		return SaaSAdminAccessProfile{}, ErrPermissionDenied
	}
	if user.IsSuperAdmin == 1 {
		return SaaSAdminAccessProfile{
			UserID: user.ID, UserName: user.Name, Phone: user.Phone, TenantID: user.TenantID,
			IsPlatformSuperAdmin: true, Permissions: []string{"*"},
		}, nil
	}
	store, ok := h.store.(SaaSAdminAccessStore)
	if !ok || store == nil {
		return SaaSAdminAccessProfile{}, ErrPermissionDenied
	}
	return store.SaaSAdminAccessProfile(ctx, user.ID, h.platformAdminTenantID)
}

func decodeSaaSAdminAccessJSON(r *http.Request, target any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil {
		return err
	}
	if len(body) == 0 || len(body) > 1<<20 {
		return errors.New("JSON 请求为空或超过 1 MiB")
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("JSON 格式错误: " + err.Error())
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("JSON 只能包含一个对象")
	}
	return nil
}

func validateSaaSAdminAccessRoleUpsert(input *SaaSAdminAccessRoleUpsert) error {
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Permissions = uniqueSortedStrings(input.Permissions)
	if input.ID < 0 || !saasAdminAccessRoleCodePattern.MatchString(input.Code) {
		return errors.New("code 必须是 3 至 48 位小写字母、数字或下划线，并以字母开头")
	}
	if input.Name == "" || len([]rune(input.Name)) > 80 || len([]rune(input.Description)) > 255 {
		return errors.New("name 必填且最多 80 个字符，description 最多 255 个字符")
	}
	if input.Status != 1 && input.Status != 2 {
		return errors.New("status 必须是 1 或 2")
	}
	if input.ID > 0 && input.ExpectedVersion <= 0 {
		return errors.New("更新岗位时 expectedVersion 必须大于 0")
	}
	if len(input.Permissions) == 0 || len(input.Permissions) > len(SaaSAdminPermissionCatalog()) {
		return errors.New("permissions 必须包含至少一个有效权限")
	}
	for _, permission := range input.Permissions {
		if !SaaSAdminPermissionValid(permission) {
			return errors.New("未知权限: " + permission)
		}
	}
	permissionSet := make(map[string]struct{}, len(input.Permissions))
	for _, permission := range input.Permissions {
		permissionSet[permission] = struct{}{}
	}
	dependencies := map[string][]string{
		SaaSAdminPermissionTenantsManage:       {SaaSAdminPermissionTenantsRead},
		SaaSAdminPermissionOperationsManage:    {SaaSAdminPermissionOperationsRead},
		SaaSAdminPermissionNotificationsManage: {SaaSAdminPermissionNotificationsRead},
		SaaSAdminPermissionFinanceManage:       {SaaSAdminPermissionFinanceRead},
		SaaSAdminPermissionAuditManage:         {SaaSAdminPermissionAuditRead},
		SaaSAdminPermissionAccessManage:        {SaaSAdminPermissionOverviewRead, SaaSAdminPermissionAuditRead},
		SaaSAdminPermissionApprovalsReview:     {SaaSAdminPermissionApprovalsRead},
		SaaSAdminPermissionApprovalsExecute:    {SaaSAdminPermissionApprovalsRead},
		SaaSAdminPermissionApprovalsManage:     {SaaSAdminPermissionApprovalsRead},
		SaaSAdminPermissionSystemManage:        {SaaSAdminPermissionSystemRead, SaaSAdminPermissionAuditRead},
		SaaSAdminPermissionIntegrationsManage:  {SaaSAdminPermissionIntegrationsRead, SaaSAdminPermissionAuditRead},
		SaaSAdminPermissionBackupsManage:       {SaaSAdminPermissionBackupsRead, SaaSAdminPermissionAuditRead},
		SaaSAdminPermissionComplianceManage:    {SaaSAdminPermissionComplianceRead, SaaSAdminPermissionAuditRead, SaaSAdminPermissionApprovalsRead},
		SaaSAdminPermissionIdentityManage:      {SaaSAdminPermissionIdentityRead, SaaSAdminPermissionAuditRead, SaaSAdminPermissionApprovalsRead},
		SaaSAdminPermissionBrandingManage:      {SaaSAdminPermissionBrandingRead, SaaSAdminPermissionAuditRead},
		SaaSAdminPermissionDomainsManage:       {SaaSAdminPermissionDomainsRead, SaaSAdminPermissionAuditRead},
	}
	for permission, required := range dependencies {
		if _, exists := permissionSet[permission]; !exists {
			continue
		}
		for _, dependency := range required {
			if _, exists := permissionSet[dependency]; !exists {
				return errors.New(permission + " 必须同时包含 " + dependency)
			}
		}
	}
	return nil
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func saasAdminPermissionCatalogPayload() []map[string]any {
	items := make([]map[string]any, 0, len(SaaSAdminPermissionCatalog()))
	for _, item := range SaaSAdminPermissionCatalog() {
		items = append(items, map[string]any{
			"code": item.Code, "name": item.Name, "category": item.Category, "description": item.Description, "write": item.Write,
		})
	}
	return items
}

func saasAdminAccessRolePayload(role SaaSAdminAccessRole) map[string]any {
	return map[string]any{
		"id": role.ID, "code": role.Code, "name": role.Name, "description": role.Description,
		"status": role.Status, "isSystem": role.IsSystem, "version": role.Version,
		"permissions": role.Permissions, "assignmentCount": role.AssignmentCount,
		"createdBy": role.CreatedBy, "updatedBy": role.UpdatedBy, "createdAt": role.CreatedAt, "updatedAt": role.UpdatedAt,
	}
}

func saasAdminAccessRolePayloads(roles []SaaSAdminAccessRole) []map[string]any {
	items := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		items = append(items, saasAdminAccessRolePayload(role))
	}
	return items
}

func saasAdminAccessProfilePayload(profile SaaSAdminAccessProfile) map[string]any {
	return map[string]any{
		"userId": profile.UserID, "userName": profile.UserName, "phone": profile.Phone, "tenantId": profile.TenantID,
		"isPlatformSuperAdmin": profile.IsPlatformSuperAdmin, "version": profile.Version,
		"roles": saasAdminAccessRolePayloads(profile.Roles), "permissions": profile.Permissions,
	}
}

func saasAdminAccessAssignmentPayload(item SaaSAdminAccessAssignment) map[string]any {
	return map[string]any{
		"userId": item.UserID, "userName": item.UserName, "phone": item.Phone, "status": item.Status,
		"isSuperAdmin": item.IsSuperAdmin, "version": item.Version, "roles": saasAdminAccessRolePayloads(item.Roles),
		"permissions": item.Permissions, "updatedBy": item.UpdatedBy, "updatedAt": item.UpdatedAt,
	}
}

func saasAdminAccessAssignmentPayloads(items []SaaSAdminAccessAssignment) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasAdminAccessAssignmentPayload(item))
	}
	return result
}
