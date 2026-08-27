export type ViewKey = 'overview' | 'tenants' | 'packages' | 'team' | 'system' | 'launch'

export interface PermissionDefinition {
  category: string
  code: string
  description: string
  name: string
  write: boolean
}

export interface AccessProfile {
  isPlatformSuperAdmin: boolean
  permissions: string[]
  phone: string
  roles: Array<{ id: number; code: string; name: string }>
  tenantId: number
  userId: number
  userName: string
  version: number
}

export interface AccessProfileData {
  permissions: PermissionDefinition[]
  profile: AccessProfile
}

export interface ApprovalPolicy {
  actionType: string
  enabled: boolean
  expiryHours: number
  name: string
  requiredApprovals: number
}

export interface ApprovalPoliciesData {
  required: boolean
  policies: ApprovalPolicy[]
}

export interface OverviewSummary {
  activeTenantPackageCount: number
  corpCount: number
  enabledPackageCount: number
  expiredTenantCount: number
  expiringSoonTenantCount: number
  openAlertCount: number
  pendingNotificationCount: number
  tenantCount: number
  userCount: number
}

export interface TenantSummary {
  expired: boolean
  expiresAt: string
  expiringSoon: boolean
  maxUsageCurrent: number
  maxUsageLabel: string
  maxUsageLimit: number
  maxUsageMetric: string
  maxUsageRatio: number
  openAlertCount: number
  packageCode: string
  packageName: string
  packageStatus: number
  packageVersion: number
  tenantId: number
  tenantName: string
  tenantStatus: number
  bindingVersion?: number
  dashboardUserId?: number
}

export interface OverviewData {
  access: Record<string, unknown>
  canPlatformScope: boolean
  generatedAt: string
  platformAdminTenantId: number
  scope: string
  summary: OverviewSummary
  tenantPopulation: number
  tenants: TenantSummary[]
}

export interface UsageMetric {
  current: number
  label: string
  limit: number
  metric: string
  openAlertCount: number
  usageRatio: number
}

export interface TenantDetailData {
  metrics: UsageMetric[]
  operations: OperationRecord[]
  platformAdminTenantId: number
  summary: OverviewSummary
  tenant: TenantSummary
  tenantId: number
}

export interface TenantAIProvider {
  tenantId: number
  providerCode: 'deepseek' | 'openai' | 'dashscope' | 'custom' | ''
  baseUrl: string
  model: string
  apiKeyConfigured: boolean
  apiKeyHint: string
  credentialProtection: 'usable' | 'unavailable' | 'unconfigured' | string
  effectiveAt: string
  expiresAt: string
  status: 'active' | 'disabled' | ''
  version: number
  updatedAt: string
}

export interface TenantAIProviderData {
  configured: boolean
  provider: TenantAIProvider
}

export interface TenantAIProviderSaveData {
  provider: TenantAIProvider
}

export interface PackagePlan {
  id: number
  code: string
  description: string
  limits: Record<string, number>
  name: string
  status: number
  version: number
}

export interface DashboardAdminProvisionResult {
  tenantId: number
  dashboardUserId: number
  bindingCorpId: number
  activationToken?: string
  activationPath?: string
  activationExpiresAt?: string
  idempotent?: boolean
}

export interface DashboardAdminGovernanceResult {
  tenantId: number
  dashboardUserId: number
  version: number
  activationToken?: string
  activationPath?: string
  activationExpiresAt?: string
  idempotent?: boolean
}

export interface DashboardIdentityRecord {
  id: number
  name: string
  loginIdentifier: string
  userStatus: number
  identityStatus: number
  activatedAt: string
  isSuperAdmin: boolean
  availableActions: Array<'resend_activation' | 'disable' | 'restore' | 'replace_current' | 'replacement_candidate'>
  blockedReasons: Record<string, string>
}

export interface DashboardAdminGovernanceData {
  tenantId: number
  bindingVersion: number
  identities: DashboardIdentityRecord[]
}

export interface PackagesData {
  packages: PackagePlan[]
}

export interface AccessAssignment {
  isSuperAdmin: boolean
  permissions: string[]
  phone: string
  roles: Array<{ id: number; code: string; name: string }>
  status: number
  updatedAt: string
  updatedBy: number
  userId: number
  userName: string
  version: number
}

export interface AccessRole {
  assignmentCount: number
  code: string
  description: string
  id: number
  isSystem: boolean
  name: string
  permissions: string[]
  status: number
  version: number
}

export interface SystemHealthSummary {
  checkCount: number
  criticalCount: number
  healthState: 'healthy' | 'warning' | 'critical' | string
  healthyCount: number
  issueCount: number
  warningCount: number
}

export interface SystemHealthCheck {
  category: string
  code: string
  current: number
  detail: string
  metadata: Record<string, unknown>
  name: string
  severity: 'warning' | 'critical' | string
  status: 'healthy' | 'warning' | 'critical' | string
  threshold: number
}

export interface SystemHealthData {
  checks: SystemHealthCheck[]
  incidents: Array<Record<string, unknown>>
  summary: SystemHealthSummary
}

export interface BackupRun {
  artifactName: string
  backupNo: string
  createdBy: number
  encrypted: boolean
  finishedAt: string
  id: number
  migrationCount: number
  migrationVersion: string
  replicaStatus: string
  sha256: string
  sizeBytes: number
  startedAt: string
  status: string
  tableCount: number
  verificationStatus: string
  verifiedAt: string
  version: number
}

export interface BackupOverviewData {
  config: Record<string, unknown>
  policy: {
    intervalMinutes: number
    maxBackupAgeMinutes: number
    minSuccessfulBackups: number
    requireEncryption: boolean
    requireOffsiteReplica: boolean
    restoreDrillIntervalDays: number
    retentionDays: number
    status: string
    version: number
  }
  runs: BackupRun[]
  summary: {
    failedCount: number
    runCount: number
    runningCount: number
    successfulCount: number
    successfulDrillCount: number
    verifiedCount: number
  }
}

export interface OperationRecord {
  action: string
  actorTenantId: number
  actorUserId: number
  createdAt: string
  id: number
  remark: string
  targetId: string
  targetName: string
  targetType: string
  tenantId: number
}

export interface OperationsData {
  operations: OperationRecord[]
  returnedCount: number
  summary: Record<string, number>
}

export interface ReleaseAction {
  createdAt: string
  dueAt: string
  dueState: string
  evidenceComplete: boolean
  evidenceStatus: string
  id: number
  key: string
  nextAction: string
  note: string
  ownerActive: boolean
  ownerName: string
  ownerPhone: string
  ownerUserId: number
  state: string
  title: string
  updatedAt: string
  updatedBy: number
  version: number
}

export interface ReleaseEvidence {
  artifactSha256: string
  artifactSizeBytes: number
  category: string
  checkedAt: string
  checkedBy: number
  environment: string
  evidenceUrl: string
  id: number
  key: string
  note: string
  required: boolean
  sourceFingerprint: string
  status: string
  title: string
  version: number
}

export interface ReleaseOwner {
  isSuperAdmin: boolean
  name: string
  phone: string
  userId: number
}

export interface ReleaseReadinessData {
  actionSummary: {
    assignedCount: number
    blockedCount: number
    dueSoonCount: number
    overdueCount: number
    resolvedCount: number
    totalCount: number
    unassignedCount: number
    unresolvedCount: number
  }
  actions: ReleaseAction[]
  candidates: Array<Record<string, unknown>>
  evidence: ReleaseEvidence[]
  owners: ReleaseOwner[]
  summary: {
    candidateGateEnabled: boolean
    failedCount: number
    inProgressCount: number
    metadataReady: boolean
    missingCount: number
    passedCount: number
    ready: boolean
    requiredCount: number
    sourceFingerprintAuthoritative: boolean
    sourceFingerprintSource: string
    targetSourceFingerprint: string
  }
}

export interface ApprovalItem {
  approvalCount: number
  actionType: string
  createdAt: string
  createdBy: number
  expiresAt: string
  id: number
  policyVersion: number
  reason: string
  requestNo: string
  requesterName: string
  requesterUserId: number
  requiredApprovals: number
  riskLevel: string
  slaDueAt: string
  status: string
  targetId: string
  targetName: string
  version: number
}

export interface ApprovalsData {
  items: ApprovalItem[]
  returnedCount: number
  summary: Record<string, number>
}

export interface GovernedResult<T> {
  approvalRequested: boolean
  data: T
}
