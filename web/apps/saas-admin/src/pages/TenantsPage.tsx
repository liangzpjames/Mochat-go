import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bot, Building2, Eye, KeyRound, Plus, RefreshCw, Search, ShieldCheck, UserRoundCog, UserRoundPlus, UserX } from 'lucide-react'
import { toast } from 'sonner'
import {
  ApiError,
  activationDeliveryURL,
  apiRequest,
  executeGoverned,
  fetchWeComIntegration,
  fetchWeComIntegrationAudits,
  hasPermission,
  jsonRequest,
  saveDelegatedWeComIntegration,
  type WeComIntegrationMode,
  type WeComIntegrationRecord,
  type WeComIntegrationView,
} from '@/lib/api'
import type {
  DashboardAdminGovernanceResult,
  DashboardAdminProvisionResult,
  DashboardAdminGovernanceData,
  OverviewData,
  PackagePlan,
  PackagesData,
  TenantDetailData,
  TenantAIProvider,
  TenantAIProviderData,
  TenantAIProviderSaveData,
  TenantSummary,
} from '@/lib/types'
import type { GovernedResult } from '@/lib/types'
import {
  Badge,
  Button,
  ConfirmAction,
  Dialog,
  EmptyState,
  ErrorState,
  Field,
  Input,
  LoadingState,
  PageHeader,
  ProgressBar,
  Select,
  SectionHeader,
  TableShell,
  formatDate,
} from '@/components/ui'
import type { PageProps } from './shared'
import { packageLimitKeys, queryKeys, tenantStatusView, usageTone } from './shared'

interface CreateTenantForm {
  tenantName: string
  adminLoginIdentifier: string
  adminName: string
  packageId: string
  subscriptionStatus: 'trialing' | 'active'
  billingCycle: 'custom' | 'monthly' | 'yearly' | 'lifetime'
  expiresAt: string
  idempotencyKey: string
  wecomIntegrationMode: '' | WeComIntegrationMode
}

interface TenantAIProviderForm {
  providerCode: 'deepseek' | 'openai' | 'dashscope' | 'custom'
  baseUrl: string
  model: string
  apiKey: string
  effectiveAt: string
  expiresAt: string
  status: 'active' | 'disabled'
  version: number
}

interface WeComIntegrationForm {
  mode: WeComIntegrationMode
  agentId: string
  providerAppId: string
  employeeSecret: string
  contactSecret: string
  agentSecret: string
  chatSecret: string
  permanentCode: string
  scope: string[]
  version: number
}

interface ActivationDelivery {
  operationKind: 'create' | 'resend'
  operationEpoch: number
  tenantId: number
  dashboardUserId: number
  tenantName: string
  accountName: string
  accountHint: string
  path: string
  expiresAt: string
}

interface WeComOperationContext {
  tenantId: number
  operationEpoch: number
}

interface WeComSaveOperation extends WeComOperationContext {
  payload: Omit<Parameters<typeof saveDelegatedWeComIntegration>[1], 'permanentCode'>
}

interface ProvisionOperation {
  operationEpoch: number
  form: CreateTenantForm
}

interface ResendOperation {
  operationEpoch: number
  tenantId: number
  dashboardUserId: number
  tenantName: string
  accountName: string
  accountHint: string
  expectedVersion: number
  requestKeySlot: string
  requestKey: string
}

interface ReplaceOperation {
  tenantId: number
  currentAdminId: number
  newAdminId: number
  expectedVersion: number
  requestKeySlot: string
  requestKey: string
}

interface StatusOperation {
  tenantId: number
  targetUserId: number
  enabled: boolean
  expectedVersion: number
  requestKeySlot: string
  requestKey: string
}

function governanceRequestKeySlot(action: string, tenantId: number, targetIds: number[], version: number) {
  return [action, tenantId, ...targetIds, `v${version}`].join(':')
}

interface TenantsPageProps extends PageProps {
  activationMutationObserverProbe?: (snapshot: {
    create: { status: string; data: GovernedResult<DashboardAdminProvisionResult> | undefined }
    resend: { status: string; data: GovernedResult<DashboardAdminGovernanceResult> | undefined }
  }) => void
}

const providerPresets: Record<Exclude<TenantAIProviderForm['providerCode'], 'custom'>, { baseUrl: string }> = {
  deepseek: { baseUrl: 'https://api.deepseek.com' },
  openai: { baseUrl: 'https://api.openai.com/v1' },
  dashscope: { baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
}

function localDateTime(value: string) {
  const date = value ? new Date(value) : null
  if (!date || Number.isNaN(date.getTime())) return ''
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

function defaultProviderForm(): TenantAIProviderForm {
  const now = new Date()
  const expires = new Date(now)
  expires.setFullYear(expires.getFullYear() + 1)
  return { providerCode: 'deepseek', ...providerPresets.deepseek, model: '', apiKey: '', effectiveAt: localDateTime(now.toISOString()), expiresAt: localDateTime(expires.toISOString()), status: 'active', version: 0 }
}

function providerFormFromData(provider: TenantAIProvider): TenantAIProviderForm {
  return {
    providerCode: provider.providerCode || 'custom',
    baseUrl: provider.baseUrl,
    model: provider.model,
    apiKey: '',
    effectiveAt: localDateTime(provider.effectiveAt),
    expiresAt: localDateTime(provider.expiresAt),
    status: provider.status || 'disabled',
    version: provider.version,
  }
}

const delegatedCapabilityCatalog = [
  { code: 'archive.read', name: '会话内容与媒体归档', description: '支持归档消息、图片、录音和文件' },
  { code: 'contacts.read', name: '通讯录与客户资料', description: '支持读取员工、客户和群聊资料' },
] as const

const delegatedCapabilityCodes = new Set<string>(delegatedCapabilityCatalog.map((capability) => capability.code))

function selectedDelegatedCapabilities(scope: string[]) {
  return delegatedCapabilityCatalog.filter((capability) => scope.includes(capability.code))
}

function mergeDelegatedScope(selectedCodes: string[], currentScope: string[]) {
  const selected = new Set(selectedCodes)
  const known = delegatedCapabilityCatalog.filter((capability) => selected.has(capability.code)).map((capability) => capability.code)
  const unknown = currentScope.filter((code) => !delegatedCapabilityCodes.has(code))
  return [...new Set([...known, ...unknown])]
}

function emptyWeComIntegrationForm(candidate?: WeComIntegrationRecord | null): WeComIntegrationForm {
  return {
    mode: candidate?.mode || 'self_built',
    agentId: candidate?.agentId || '',
    providerAppId: candidate?.providerAppId || '',
    employeeSecret: '',
    contactSecret: '',
    agentSecret: '',
    chatSecret: '',
    permanentCode: '',
    scope: candidate?.scope?.length ? [...new Set(candidate.scope)] : delegatedCapabilityCatalog.map((capability) => capability.code),
    version: candidate?.version || 0,
  }
}

function weComModeLabel(mode?: string) {
  if (mode === 'self_built') return '自建应用'
  if (mode === 'third_party_delegated') return '第三方代开发应用'
  return '未配置'
}

function weComStatusLabel(status?: string) {
  const labels: Record<string, string> = {
    unconfigured: '未配置',
    pending_verification: '待验证',
    active: '可用',
    suspended: '已暂停',
    revoked: '已撤销',
    failed: '验证失败',
  }
  return labels[status || ''] || '未知'
}

function weComStatusTone(status?: string) {
  if (status === 'active') return 'success' as const
  if (status === 'pending_verification') return 'warning' as const
  if (status === 'failed' || status === 'revoked') return 'danger' as const
  return 'neutral' as const
}

function safeLoginHint(login: string) {
  const value = login.trim()
  if (/^1\d{10}$/.test(value)) return `${value.slice(0, 3)}****${value.slice(-4)}`
  if (value.length <= 2) return '*'.repeat(value.length)
  return `${value.slice(0, 1)}***${value.slice(-1)}`
}

function WeComIntegrationSummary({ label, record }: { label: '当前' | '候选'; record?: WeComIntegrationRecord | null }) {
  if (!record) {
    return <div className="rounded-md border border-dashed border-zinc-300 bg-white p-3"><strong className="text-sm">{label}：未配置</strong><p className="mt-1 text-xs text-zinc-500">服务端没有返回该槽位配置。</p></div>
  }
  const selectedCapabilities = selectedDelegatedCapabilities(record.scope)
  const missingCapabilities = selectedDelegatedCapabilities(record.missingCapabilities)
  const hasUnknownMissingCapabilities = record.missingCapabilities.some((capability) => !delegatedCapabilityCodes.has(capability))
  const scopeLabel = selectedCapabilities.length === delegatedCapabilityCatalog.length
    ? '全部能力已开启'
    : `已开启 ${selectedCapabilities.length} 项`
  return <div className="rounded-md border border-violet-100 bg-white p-3" aria-label={`${label}企微集成`}>
    <div className="flex flex-wrap items-center justify-between gap-2"><strong className="text-sm">{label}：{weComModeLabel(record.mode)}</strong><Badge tone={weComStatusTone(record.status)}>{weComStatusLabel(record.status)}</Badge></div>
    <dl className="mt-3 grid gap-2 text-xs text-zinc-600 sm:grid-cols-2">
      <div><dt className="text-zinc-400">CorpID</dt><dd className="break-all">{record.verifiedWxCorpId || '由租户企业绑定确定'}</dd></div>
      <div><dt className="text-zinc-400">应用标识</dt><dd className="break-all">{record.mode === 'self_built' ? record.agentId || '未配置 AgentID' : record.providerAppId || '未配置 Provider App ID'}</dd></div>
      <div><dt className="text-zinc-400">凭据</dt><dd>{record.credentialConfigured ? record.credentialHint || '已安全配置' : '未配置'}</dd></div>
      <div><dt className="text-zinc-400">代次 / 版本</dt><dd>g{record.generation} / v{record.version}</dd></div>
      <div className="sm:col-span-2"><dt className="text-zinc-400">能力范围</dt><dd className="break-words"><span>{scopeLabel}</span>{selectedCapabilities.length > 0 && <span>：{selectedCapabilities.map((capability) => capability.name).join('、')}</span>}</dd></div>
    </dl>
    {record.verificationLevel === 'local_contract' && <p className="mt-3 text-xs font-medium text-violet-700">本地合同验证（不代表真实企微线上可用）</p>}
    {record.missingCapabilities.length > 0 && <p role="alert" className="mt-2 text-xs text-red-700">{missingCapabilities.length > 0 && <>缺失能力：{missingCapabilities.map((capability) => capability.name).join('、')}</>}{hasUnknownMissingCapabilities && <>{missingCapabilities.length > 0 ? '；' : ''}部分能力暂不可用，请联系管理员。</>}</p>}
    {record.lastErrorCode && <p role="alert" className="mt-2 text-xs text-red-700">当前配置暂不可用，请重新保存或联系管理员。</p>}
  </div>
}

export function tenantAIProviderState(configured: boolean, provider: TenantAIProvider, now = new Date()) {
  if (!configured) return { label: '未配置', tone: 'neutral' as const, hint: '尚未设置租户专属模型。' }
  if (provider.credentialProtection !== 'usable') return { label: '凭证不可用', tone: 'danger' as const, hint: '密钥当前无法解密，分析失败关闭。' }
  if (provider.status !== 'active') return { label: '已停用', tone: 'neutral' as const, hint: '配置保留，但不会用于分析。' }
  const effectiveAt = new Date(provider.effectiveAt)
  const expiresAt = new Date(provider.expiresAt)
  if (Number.isNaN(effectiveAt.getTime()) || Number.isNaN(expiresAt.getTime()) || expiresAt <= effectiveAt) return { label: '配置无效', tone: 'danger' as const, hint: '有效期配置不完整，分析失败关闭。' }
  if (now < effectiveAt) return { label: '待生效', tone: 'warning' as const, hint: `将于 ${formatDate(provider.effectiveAt)} 生效。` }
  if (now >= expiresAt) return { label: '已过期', tone: 'danger' as const, hint: `已于 ${formatDate(provider.expiresAt)} 失效。` }
  if (expiresAt.getTime() - now.getTime() <= 7 * 24 * 60 * 60 * 1000) return { label: '即将过期', tone: 'warning' as const, hint: `将于 ${formatDate(provider.expiresAt)} 失效。` }
  return { label: '可用', tone: 'success' as const, hint: `有效至 ${formatDate(provider.expiresAt)}。` }
}

const makeRequestKey = (prefix: string) => {
  const uuid = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`
  return `${prefix}-${uuid}`
}

const emptyCreateForm = (): CreateTenantForm => ({
  tenantName: '',
  adminLoginIdentifier: '',
  adminName: '',
  packageId: '',
  subscriptionStatus: 'trialing',
  billingCycle: 'custom',
  expiresAt: '',
  idempotencyKey: makeRequestKey('dashboard-provision'),
  wecomIntegrationMode: '',
})

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError) return error.status === 409 ? `${fallback}；请刷新治理列表后重试，当前选择已保留。` : fallback
  return error instanceof Error ? error.message : fallback
}

function aiProviderErrorMessage(error: unknown) {
  return errorMessage(error, 'AI 模型配置保存失败，API Key 已从页面清除')
}

function aiProviderConflictMessage(refreshed: boolean) {
  return refreshed
    ? '配置已由其他管理员更新；页面已载入最新版本，请重新确认后保存。'
    : '配置已由其他管理员更新，但最新版本刷新失败；请检查网络后重试。'
}

function packageLimitsSnapshot(plan: PackagePlan) {
  const limits: Record<string, number> = {}
  for (const key of packageLimitKeys) {
    const value = plan.limits[key]
    if (typeof value !== 'number' || !Number.isInteger(value) || value < 0) {
      throw new Error(`套餐额度快照缺少 ${key}`)
    }
    limits[key] = value
  }
  return limits
}

function isoEndOfDate(date: string) {
  if (!date) return ''
  const value = new Date(`${date}T23:59:59Z`)
  return Number.isNaN(value.getTime()) ? '' : value.toISOString()
}

export default function TenantsPage({ profile, approvalMode, activationMutationObserverProbe }: TenantsPageProps) {
  const queryClient = useQueryClient()
  const canManage = hasPermission(profile.permissions, 'platform.tenants.manage')
  const canReadAIProvider = hasPermission(profile.permissions, 'platform.integrations.read')
  const canManageAIProvider = hasPermission(profile.permissions, 'platform.integrations.manage')
  const requestKeys = useRef<Record<string, string>>({})
	const clearRequestKey = (slot: string, requestKey: string) => {
		if (requestKeys.current[slot] === requestKey) delete requestKeys.current[slot]
		for (const [candidateSlot, candidateKey] of Object.entries(requestKeys.current)) {
			if (candidateKey === requestKey) delete requestKeys.current[candidateSlot]
		}
	}
  const selectedTenantIDRef = useRef(0)
  const activationOperationEpochRef = useRef(0)
  const weComOperationEpochRef = useRef(0)
  const weComPermanentCodesRef = useRef(new Map<number, string>())
  const createMutationResetRef = useRef<() => void>(() => undefined)
  const resendMutationResetRef = useRef<() => void>(() => undefined)
  const observedCreateOperationEpochRef = useRef(0)
  const observedResendOperationEpochRef = useRef(0)
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const [packageFilter, setPackageFilter] = useState('all')
  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] = useState<CreateTenantForm>(emptyCreateForm)
  const [selectedTenantId, setSelectedTenantId] = useState(0)
  const [targetAdminId, setTargetAdminId] = useState('')
  const [currentAdminId, setCurrentAdminId] = useState('')
  const [replacementAdminId, setReplacementAdminId] = useState('')
  const [activationDelivery, setActivationDelivery] = useState<ActivationDelivery | null>(null)
  const [activationDeliveryOpen, setActivationDeliveryOpen] = useState(false)
  const [aiProviderOpen, setAIProviderOpen] = useState(false)
  const [aiProviderForm, setAIProviderForm] = useState<TenantAIProviderForm>(defaultProviderForm)
  const [aiProviderSaveError, setAIProviderSaveError] = useState('')
  const [weComEditorOpen, setWeComEditorOpen] = useState(false)
  const [weComForm, setWeComForm] = useState<WeComIntegrationForm>(emptyWeComIntegrationForm())
  const [weComError, setWeComError] = useState('')

  const overviewQuery = useQuery({
    queryKey: queryKeys.overview,
    queryFn: () => apiRequest<OverviewData>('/dashboard/saasAdmin/overview?scope=platform&expiringDays=30&riskHighUsageRatio=0.8'),
  })
  const packagesQuery = useQuery({
    queryKey: queryKeys.packages,
    queryFn: () => apiRequest<PackagesData>('/dashboard/saasAdmin/packages'),
  })
  const detailQuery = useQuery({
    queryKey: ['tenant-detail', selectedTenantId],
    queryFn: () => apiRequest<TenantDetailData>(`/dashboard/saasAdmin/tenant?tenantId=${selectedTenantId}&expiringDays=30&operationLimit=12`),
    enabled: selectedTenantId > 0,
  })
  const governanceQuery = useQuery({
    queryKey: ['dashboard-admin-governance', selectedTenantId],
    queryFn: () => apiRequest<DashboardAdminGovernanceData>(`/dashboard/saasAdmin/tenants/${selectedTenantId}/dashboard-admins`),
    enabled: selectedTenantId > 0,
  })
  const aiProviderQuery = useQuery({
    queryKey: ['tenant-ai-provider', selectedTenantId],
    queryFn: () => apiRequest<TenantAIProviderData>(`/dashboard/saasAdmin/tenantAIProvider?tenantId=${selectedTenantId}`),
    enabled: selectedTenantId > 0 && canReadAIProvider,
  })
  const weComIntegrationQuery = useQuery({
    queryKey: ['tenant-wecom-integration', selectedTenantId],
    queryFn: () => fetchWeComIntegration(selectedTenantId),
    enabled: selectedTenantId > 0 && canReadAIProvider,
  })
  const weComAuditQuery = useQuery({
    queryKey: ['tenant-wecom-integration-audits', selectedTenantId],
    queryFn: () => fetchWeComIntegrationAudits(selectedTenantId),
    enabled: selectedTenantId > 0 && canReadAIProvider,
  })

  const closeAIProvider = () => {
    setAIProviderOpen(false)
    setAIProviderForm((form) => ({ ...form, apiKey: '' }))
  }

  const openAIProvider = () => {
    setAIProviderSaveError('')
    const current = aiProviderQuery.data?.provider
    if (current && aiProviderQuery.data?.configured) {
      setAIProviderForm(providerFormFromData(current))
    } else {
      setAIProviderForm(defaultProviderForm())
    }
    setAIProviderOpen(true)
  }

  const aiProviderMutation = useMutation<TenantAIProviderSaveData, unknown>({
    mutationFn: async () => {
      const effectiveAt = new Date(aiProviderForm.effectiveAt)
      const expiresAt = new Date(aiProviderForm.expiresAt)
      if (!aiProviderForm.baseUrl.trim() || !aiProviderForm.model.trim()) throw new Error('请填写接口地址和模型名称')
      if (Number.isNaN(effectiveAt.getTime()) || Number.isNaN(expiresAt.getTime()) || expiresAt <= effectiveAt) throw new Error('有效结束时间必须晚于生效时间')
      if (!aiProviderQuery.data?.configured && !aiProviderForm.apiKey.trim()) throw new Error('首次配置必须填写 API Key')
      return apiRequest<TenantAIProviderSaveData>('/dashboard/saasAdmin/tenantAIProvider', jsonRequest('PUT', {
        tenantId: selectedTenantId,
        providerCode: aiProviderForm.providerCode,
        baseUrl: aiProviderForm.baseUrl.trim(),
        model: aiProviderForm.model.trim(),
        apiKey: aiProviderForm.apiKey.trim(),
        effectiveAt: effectiveAt.toISOString(),
        expiresAt: expiresAt.toISOString(),
        status: aiProviderForm.status,
        version: aiProviderForm.version,
      }))
    },
    onSuccess: async (data) => {
      setAIProviderSaveError('')
      queryClient.setQueryData<TenantAIProviderData>(['tenant-ai-provider', selectedTenantId], { configured: true, provider: data.provider })
      closeAIProvider()
      toast.success('租户 AI 模型配置已保存；保存动作不会触发模型调用')
    },
    onError: async (error) => {
      setAIProviderForm((form) => ({ ...form, apiKey: '' }))
      setAIProviderSaveError('')
      if (error instanceof ApiError && error.status === 409) {
        const refreshed = await aiProviderQuery.refetch()
        const refreshSucceeded = !refreshed.isError && Boolean(refreshed.data?.configured)
        if (refreshSucceeded && refreshed.data?.provider) setAIProviderForm(providerFormFromData(refreshed.data.provider))
        const message = aiProviderConflictMessage(refreshSucceeded)
        setAIProviderSaveError(message)
        toast.error(message)
        return
      }
      const message = aiProviderErrorMessage(error)
      setAIProviderSaveError(message)
      toast.error(message)
    },
  })

  const closeWeComEditor = () => {
    weComOperationEpochRef.current += 1
    weComPermanentCodesRef.current.clear()
    setWeComEditorOpen(false)
    setWeComForm((form) => ({ ...form, employeeSecret: '', contactSecret: '', agentSecret: '', chatSecret: '', permanentCode: '' }))
  }

  const openWeComEditor = () => {
    weComOperationEpochRef.current += 1
    weComPermanentCodesRef.current.clear()
    setWeComError('')
    setWeComForm(emptyWeComIntegrationForm(weComIntegrationQuery.data?.current))
    setWeComEditorOpen(true)
  }

  const isCurrentWeComOperation = (operation: WeComOperationContext) => selectedTenantIDRef.current === operation.tenantId && weComOperationEpochRef.current === operation.operationEpoch

  const invalidateWeComAudit = (tenantId: number) => queryClient.invalidateQueries({ queryKey: ['tenant-wecom-integration-audits', tenantId] })

  const handleWeComError = async (error: unknown, operation: WeComOperationContext, fallback: string) => {
    if (error instanceof ApiError && error.status === 409) {
      let refreshed: WeComIntegrationView | undefined
      try {
        refreshed = await queryClient.fetchQuery({ queryKey: ['tenant-wecom-integration', operation.tenantId], queryFn: () => fetchWeComIntegration(operation.tenantId) })
      } catch {
        refreshed = undefined
      }
      await invalidateWeComAudit(operation.tenantId)
      if (!isCurrentWeComOperation(operation)) return
      setWeComForm(refreshed ? emptyWeComIntegrationForm(refreshed.current) : (form) => ({ ...form, employeeSecret: '', contactSecret: '', agentSecret: '', chatSecret: '', permanentCode: '' }))
      const message = !refreshed
        ? '企微配置版本冲突，最新状态刷新失败，请检查网络后重试。'
        : '企微配置已由其他管理员更新；页面已刷新至最新企微配置，请重新确认。'
      setWeComError(message)
      toast.error(message)
      return
    }
    if (!isCurrentWeComOperation(operation)) return
    setWeComForm((form) => ({ ...form, employeeSecret: '', contactSecret: '', agentSecret: '', chatSecret: '', permanentCode: '' }))
    const message = errorMessage(error, fallback)
    setWeComError(message)
    toast.error(message)
  }

  const saveWeComMutation = useMutation<WeComIntegrationRecord, unknown, WeComSaveOperation>({
    mutationFn: (operation) => {
      const permanentCode = weComPermanentCodesRef.current.get(operation.operationEpoch) || ''
      weComPermanentCodesRef.current.delete(operation.operationEpoch)
      return saveDelegatedWeComIntegration(operation.tenantId, { ...operation.payload, permanentCode })
    },
    onSuccess: async (integration, operation) => {
      queryClient.setQueryData<WeComIntegrationView>(['tenant-wecom-integration', operation.tenantId], (current) => current ? { ...current, current: integration, candidate: null } : current)
      await invalidateWeComAudit(operation.tenantId)
      if (!isCurrentWeComOperation(operation)) return
      setWeComError('')
      closeWeComEditor()
      toast.success('第三方应用配置已安全保存，企微模式保持不变')
    },
    onError: (error, operation) => handleWeComError(error, operation, '第三方应用配置保存失败，敏感输入已清空'),
  })

  const submitWeComCandidate = () => {
    try {
      const tenantId = selectedTenantIDRef.current
      if (!tenantId) throw new Error('租户详情已经关闭，请重新打开后保存')
      const selectedScope = selectedDelegatedCapabilities(weComForm.scope)
      const scope = mergeDelegatedScope(selectedScope.map((capability) => capability.code), weComForm.scope)
      if (scope.length === 0) throw new Error('请至少选择一项能力')
      if (weComForm.mode !== 'third_party_delegated') throw new Error('自建应用配置只能在 Dashboard 唯一企业资料中维护')
      if (!weComForm.providerAppId.trim()) throw new Error('第三方代开发应用必须填写 Provider App ID')
      const operationEpoch = ++weComOperationEpochRef.current
      const payload = { mode: weComForm.mode, scope, version: weComForm.version, providerAppId: weComForm.providerAppId.trim() }
      weComPermanentCodesRef.current.set(operationEpoch, weComForm.permanentCode.trim())
      setWeComForm((form) => ({ ...form, employeeSecret: '', contactSecret: '', agentSecret: '', chatSecret: '', permanentCode: '' }))
      saveWeComMutation.mutate({ tenantId, payload, operationEpoch })
    } catch (error) {
      setWeComForm((form) => ({ ...form, employeeSecret: '', contactSecret: '', agentSecret: '', chatSecret: '', permanentCode: '' }))
      const message = errorMessage(error, '企微候选配置无效')
      setWeComError(message)
      toast.error(message)
    }
  }

  const invalidateTenantData = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.overview }),
      queryClient.invalidateQueries({ queryKey: ['tenant-detail'] }),
      queryClient.invalidateQueries({ queryKey: queryKeys.operations }),
    ])
  }

  const purgeActivationMutation = (mutationKey: string, operationEpoch: number) => {
    const mutationCache = queryClient.getMutationCache()
    for (const mutation of mutationCache.getAll()) {
      const variables = mutation.state.variables as { operationEpoch?: number } | undefined
      if (mutation.options.mutationKey?.[0] === mutationKey && variables?.operationEpoch === operationEpoch) mutationCache.remove(mutation)
    }
  }

  const resetActivationMutation = (kind: 'create' | 'resend', operationEpoch: number) => {
    if (kind === 'create') {
      if (observedCreateOperationEpochRef.current === operationEpoch) {
        createMutationResetRef.current()
        observedCreateOperationEpochRef.current = 0
      }
      purgeActivationMutation('tenant-provision-delivery', operationEpoch)
      return
    }
    if (observedResendOperationEpochRef.current === operationEpoch) {
      resendMutationResetRef.current()
      observedResendOperationEpochRef.current = 0
    }
    purgeActivationMutation('tenant-activation-resend-delivery', operationEpoch)
  }

  const createMutation = useMutation<GovernedResult<DashboardAdminProvisionResult>, unknown, ProvisionOperation>({
    mutationKey: ['tenant-provision-delivery'],
    mutationFn: async (operation) => {
      const form = operation.form
      const plan = (packagesQuery.data?.packages || []).find((item) => String(item.id) === form.packageId && item.status === 1)
      if (!plan) throw new Error('请选择已启用套餐')
      if (!form.tenantName.trim() || !form.adminName.trim()) throw new Error('请填写客户名称和管理员姓名')
      if (form.wecomIntegrationMode !== 'self_built' && form.wecomIntegrationMode !== 'third_party_delegated') throw new Error('请选择企微对接模式')
      if (!/^1\d{10}$/.test(form.adminLoginIdentifier.trim())) throw new Error('管理员手机号格式不正确')
      const expiresAt = isoEndOfDate(form.expiresAt)
      if (!expiresAt) throw new Error('请填写有效到期日期')
      const payload = {
          tenantName: form.tenantName.trim(),
          packageId: plan.id,
          limits: packageLimitsSnapshot(plan),
          subscription: {
            packageCode: plan.code,
            status: form.subscriptionStatus,
            billingCycle: form.billingCycle,
            startsAt: new Date().toISOString(),
            expiresAt,
          },
          adminLoginIdentifier: form.adminLoginIdentifier.trim(),
          adminName: form.adminName.trim(),
          idempotencyKey: form.idempotencyKey,
          expectedVersion: plan.version,
          wecomIntegrationMode: form.wecomIntegrationMode,
      }
      return executeGoverned<DashboardAdminProvisionResult>({
        approvalMode,
        actionType: 'dashboard.tenant.provision',
        payload,
        approvalPayload: payload,
        approvalIdempotencyKey: form.idempotencyKey,
        reason: `开通客户租户 ${form.tenantName.trim()}`,
        directPath: '/dashboard/saasAdmin/tenants/provision',
        directHeaders: { 'X-Request-ID': form.idempotencyKey },
      })
    },
    onSuccess: async (result, operation) => {
      const active = () => activationOperationEpochRef.current === operation.operationEpoch
      if (result.approvalRequested) {
        await queryClient.invalidateQueries({ queryKey: queryKeys.approvals })
        if (active()) {
          setCreateOpen(false)
          toast.success('开户申请已提交审批，待独立复核后执行')
        }
        resetActivationMutation('create', operation.operationEpoch)
        return
      }
      await invalidateTenantData()
      if (!active()) {
        resetActivationMutation('create', operation.operationEpoch)
        return
      }
      setCreateOpen(false)
      setCreateForm(emptyCreateForm())
      if (result.data.activationPath && result.data.tenantId > 0 && result.data.dashboardUserId > 0) {
        setActivationDelivery({ operationKind: 'create', operationEpoch: operation.operationEpoch, tenantId: result.data.tenantId, dashboardUserId: result.data.dashboardUserId, tenantName: operation.form.tenantName.trim(), accountName: operation.form.adminName.trim(), accountHint: safeLoginHint(operation.form.adminLoginIdentifier), path: result.data.activationPath, expiresAt: result.data.activationExpiresAt || '' })
        setActivationDeliveryOpen(true)
      } else {
        resetActivationMutation('create', operation.operationEpoch)
      }
      toast.success(result.data.idempotent ? '开户请求已确认，未重复生成激活入口' : '租户已开通，请通过受控渠道交付一次性激活入口')
    },
    onError: (error, operation) => {
      if (activationOperationEpochRef.current === operation.operationEpoch) toast.error(errorMessage(error, '开户失败，表单内容已保留'))
    },
  })
  createMutationResetRef.current = createMutation.reset

  const resendMutation = useMutation<GovernedResult<DashboardAdminGovernanceResult>, unknown, ResendOperation>({
    mutationKey: ['tenant-activation-resend-delivery'],
    mutationFn: async (operation) => {
      const payload = { targetUserId: operation.dashboardUserId, expectedVersion: operation.expectedVersion }
      return executeGoverned<DashboardAdminGovernanceResult>({
        approvalMode,
        actionType: 'dashboard.activation.resend',
        payload,
        approvalPayload: { tenantId: operation.tenantId, ...payload },
        approvalIdempotencyKey: operation.requestKey,
        reason: `重发租户 ${operation.tenantId} 的 Dashboard 激活`,
        directPath: `/dashboard/saasAdmin/tenants/${operation.tenantId}/activation/resend`,
        directHeaders: { 'X-Request-ID': operation.requestKey },
      })
    },
    onSuccess: async (result, operation) => {
      const active = () => activationOperationEpochRef.current === operation.operationEpoch && selectedTenantIDRef.current === operation.tenantId
      if (result.approvalRequested) {
        await queryClient.invalidateQueries({ queryKey: queryKeys.approvals })
        if (active()) toast.success('重发激活申请已提交审批，待独立复核后执行')
        resetActivationMutation('resend', operation.operationEpoch)
        return
      }
      clearRequestKey(operation.requestKeySlot, operation.requestKey)
      await queryClient.invalidateQueries({ queryKey: ['dashboard-admin-governance', operation.tenantId] })
      await invalidateTenantData()
      if (!active()) {
        resetActivationMutation('resend', operation.operationEpoch)
        return
      }
      if (result.data.tenantId !== operation.tenantId || result.data.dashboardUserId !== operation.dashboardUserId) {
        resetActivationMutation('resend', operation.operationEpoch)
        toast.error('激活结果与请求租户或账号不匹配，已拒绝展示入口')
        return
      }
      if (result.data.activationPath) {
        setActivationDelivery({ operationKind: 'resend', operationEpoch: operation.operationEpoch, tenantId: operation.tenantId, dashboardUserId: operation.dashboardUserId, tenantName: operation.tenantName, accountName: operation.accountName, accountHint: operation.accountHint, path: result.data.activationPath, expiresAt: result.data.activationExpiresAt || '' })
        setActivationDeliveryOpen(true)
      } else {
        resetActivationMutation('resend', operation.operationEpoch)
      }
      toast.success(result.data.idempotent ? '重发请求已确认，激活入口不会重复显示' : '已生成新的激活入口，请通过受控渠道交付')
    },
    onError: (error, operation) => {
      if (activationOperationEpochRef.current === operation.operationEpoch && selectedTenantIDRef.current === operation.tenantId) toast.error(errorMessage(error, '重发激活失败，目标信息已保留'))
    },
  })
  resendMutationResetRef.current = resendMutation.reset

  useEffect(() => {
    activationMutationObserverProbe?.({
      create: { status: createMutation.status, data: createMutation.data },
      resend: { status: resendMutation.status, data: resendMutation.data },
    })
  }, [activationMutationObserverProbe, createMutation.data, createMutation.status, resendMutation.data, resendMutation.status])

  const clearActivationDelivery = () => {
    const delivery = activationDelivery
    activationOperationEpochRef.current += 1
    setActivationDeliveryOpen(false)
    setActivationDelivery(null)
    if (delivery) resetActivationMutation(delivery.operationKind, delivery.operationEpoch)
  }

  const replaceMutation = useMutation<GovernedResult<DashboardAdminGovernanceResult>, unknown, ReplaceOperation>({
    mutationFn: async (operation) => {
      const payload = { currentAdminId: operation.currentAdminId, newAdminId: operation.newAdminId, expectedVersion: operation.expectedVersion }
      return executeGoverned<DashboardAdminGovernanceResult>({
        approvalMode,
        actionType: 'dashboard.superadmin.replace',
        payload,
        approvalPayload: { tenantId: operation.tenantId, ...payload },
        approvalIdempotencyKey: operation.requestKey,
        reason: `替换租户 ${operation.tenantId} 的 Dashboard 超级管理员`,
        directPath: `/dashboard/saasAdmin/tenants/${operation.tenantId}/super-admin/replace`,
        directHeaders: { 'X-Request-ID': operation.requestKey },
      })
    },
    onSuccess: async (result, operation) => {
      if (result.approvalRequested) {
        toast.success('超管替换申请已提交审批，待独立复核后执行')
        await queryClient.invalidateQueries({ queryKey: queryKeys.approvals })
        return
      }
      clearRequestKey(operation.requestKeySlot, operation.requestKey)
      await governanceQuery.refetch()
      toast.success(result.data.idempotent ? '替换请求已确认' : '超级管理员已替换')
      await invalidateTenantData()
    },
    onError: (error) => toast.error(errorMessage(error, '替换超级管理员失败，表单内容已保留')),
  })

  const statusMutation = useMutation<GovernedResult<DashboardAdminGovernanceResult>, unknown, StatusOperation>({
    mutationFn: async (operation) => {
      const payload = { targetUserId: operation.targetUserId, enabled: operation.enabled, expectedVersion: operation.expectedVersion }
      return executeGoverned<DashboardAdminGovernanceResult>({
        approvalMode,
        actionType: 'dashboard.superadmin.status',
        payload,
        approvalPayload: { tenantId: operation.tenantId, ...payload },
        approvalIdempotencyKey: operation.requestKey,
        reason: `${operation.enabled ? '恢复' : '停用'}租户 ${operation.tenantId} 的 Dashboard 超级管理员`,
        directPath: `/dashboard/saasAdmin/tenants/${operation.tenantId}/super-admin/status`,
        directHeaders: { 'X-Request-ID': operation.requestKey },
      })
    },
    onSuccess: async (result, operation) => {
      if (result.approvalRequested) {
        toast.success(`${operation.enabled ? '恢复' : '停用'}超管申请已提交审批，待独立复核后执行`)
        await queryClient.invalidateQueries({ queryKey: queryKeys.approvals })
        return
      }
      clearRequestKey(operation.requestKeySlot, operation.requestKey)
      await governanceQuery.refetch()
      toast.success(result.data.idempotent ? '状态请求已确认' : operation.enabled ? '超级管理员已恢复' : '超级管理员已停用')
      await invalidateTenantData()
    },
    onError: (error) => toast.error(errorMessage(error, '超级管理员状态变更失败，表单内容已保留')),
  })

  const enabledPackages = (packagesQuery.data?.packages || []).filter((item) => item.status === 1)
  const tenants = overviewQuery.data?.tenants || []
  const filteredTenants = useMemo(() => tenants.filter((tenant) => {
    const normalizedKeyword = keyword.trim().toLowerCase()
    const matchesKeyword = !normalizedKeyword || tenant.tenantName.toLowerCase().includes(normalizedKeyword) || String(tenant.tenantId).includes(normalizedKeyword) || tenant.packageName.toLowerCase().includes(normalizedKeyword)
    const matchesStatus = statusFilter === 'all' || String(tenant.tenantStatus) === statusFilter
    const matchesPackage = packageFilter === 'all' || tenant.packageCode === packageFilter
    return matchesKeyword && matchesStatus && matchesPackage
  }), [keyword, packageFilter, statusFilter, tenants])

  const openCreate = () => {
    activationOperationEpochRef.current += 1
    setCreateForm({ ...emptyCreateForm(), packageId: enabledPackages[0] ? String(enabledPackages[0].id) : '' })
    setCreateOpen(true)
  }

  const closeCreate = () => {
    activationOperationEpochRef.current += 1
    setCreateOpen(false)
  }

  const openTenant = (tenant: TenantSummary) => {
    selectedTenantIDRef.current = tenant.tenantId
    activationOperationEpochRef.current += 1
    weComOperationEpochRef.current += 1
    setSelectedTenantId(tenant.tenantId)
    setTargetAdminId('')
    setCurrentAdminId('')
    setReplacementAdminId('')
    closeAIProvider()
    closeWeComEditor()
    setWeComForm(emptyWeComIntegrationForm())
    setWeComError('')
    resendMutation.reset()
    replaceMutation.reset()
    statusMutation.reset()
    requestKeys.current = {}
  }

  useEffect(() => {
    const identities = governanceQuery.data?.identities || []
    if (!selectedTenantId || identities.length === 0) return
	const can = (identity: DashboardAdminGovernanceData['identities'][number], action: string) => identity.availableActions?.includes(action as never)
	const targets = identities.filter((identity) => can(identity, 'resend_activation') || can(identity, 'disable') || can(identity, 'restore'))
	const currents = identities.filter((identity) => can(identity, 'replace_current'))
	const candidates = identities.filter((identity) => can(identity, 'replacement_candidate'))
	setTargetAdminId((value) => targets.some((identity) => identity.id === Number(value)) ? value : targets[0] ? String(targets[0].id) : '')
	setCurrentAdminId((value) => currents.some((identity) => identity.id === Number(value)) ? value : currents[0] ? String(currents[0].id) : '')
	setReplacementAdminId((value) => candidates.some((identity) => identity.id === Number(value)) ? value : candidates[0] ? String(candidates[0].id) : '')
  }, [currentAdminId, governanceQuery.data, selectedTenantId])

  const selectedTenant = detailQuery.data?.tenant
  const coreMetrics = (detailQuery.data?.metrics || []).filter((item) => ['users', 'corps', 'contacts', 'rooms', 'storage_mb', 'channel_codes'].includes(item.metric))
  const createPlan = enabledPackages.find((item) => String(item.id) === createForm.packageId)
  const governanceVersion = governanceQuery.data?.bindingVersion || 0
  const targetIdentity = governanceQuery.data?.identities.find((identity) => identity.id === Number(targetAdminId))
  const currentIdentity = governanceQuery.data?.identities.find((identity) => identity.id === Number(currentAdminId))
  const replacementIdentity = governanceQuery.data?.identities.find((identity) => identity.id === Number(replacementAdminId))
	const governanceCan = (identity: typeof targetIdentity, action: string) => Boolean(identity?.availableActions?.includes(action as never))
	const governanceTargets = (governanceQuery.data?.identities || []).filter((identity) => governanceCan(identity, 'resend_activation') || governanceCan(identity, 'disable') || governanceCan(identity, 'restore'))
	const governanceCurrents = (governanceQuery.data?.identities || []).filter((identity) => governanceCan(identity, 'replace_current'))
	const governanceCandidates = (governanceQuery.data?.identities || []).filter((identity) => governanceCan(identity, 'replacement_candidate'))
	const hasLastSuperAdminBlock = (governanceQuery.data?.identities || []).some((identity) => identity.blockedReasons?.disable === 'LAST_SUPER_ADMIN')
  const governanceSummary = selectedTenant ? `${selectedTenant.tenantName}（租户 ${selectedTenant.tenantId}），管理员 ${targetIdentity?.name || '未选择'}，绑定版本 ${governanceVersion || '加载中'}` : '请先打开一个客户租户详情。'
  const resendMutationError = resendMutation.error && resendMutation.variables && resendMutation.variables.tenantId === selectedTenantIDRef.current && resendMutation.variables.operationEpoch === activationOperationEpochRef.current ? resendMutation.error : null
  const governanceMutationError = resendMutationError || replaceMutation.error || statusMutation.error
  const aiProviderView = aiProviderQuery.data ? tenantAIProviderState(aiProviderQuery.data.configured, aiProviderQuery.data.provider) : null
  const activationURL = useMemo(() => {
    if (!activationDelivery?.path) return ''
    try {
      return activationDeliveryURL(activationDelivery.path)
    } catch {
      return ''
    }
  }, [activationDelivery])

  if (overviewQuery.isLoading || packagesQuery.isLoading) return <LoadingState label="正在加载客户租户" />
  if (overviewQuery.isError || !overviewQuery.data) return <ErrorState message={errorMessage(overviewQuery.error, '无法加载客户租户')} onRetry={() => overviewQuery.refetch()} />

  return (
    <div className="space-y-7">
      <PageHeader title="客户租户" description="SaaS 开户与 Dashboard 超级管理员治理" actions={canManage ? <Button type="button" onClick={openCreate}><Plus className="h-4 w-4" />开通客户</Button> : undefined} />

      <section className="flex flex-col gap-3 rounded-lg border border-zinc-200 bg-white p-3 lg:flex-row lg:items-center" aria-label="客户筛选">
        <label className="relative min-w-0 flex-1"><Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-zinc-400" /><Input value={keyword} onChange={(event) => setKeyword(event.target.value)} className="pl-9" placeholder="搜索客户名称、ID 或套餐" /></label>
        <Select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} className="lg:w-36"><option value="all">全部状态</option><option value="1">正常</option><option value="2">已停用</option></Select>
        <Select value={packageFilter} onChange={(event) => setPackageFilter(event.target.value)} className="lg:w-44"><option value="all">全部套餐</option>{enabledPackages.map((item) => <option key={item.id} value={item.code}>{item.name}</option>)}</Select>
        <span className="px-1 text-sm text-zinc-500">{filteredTenants.length} 个客户</span>
      </section>

      <section className="space-y-3">
        <SectionHeader title="客户列表" description="企业绑定由服务端 principal 决定，页面不提供 corp 选择。" />
        <TableShell>
          {filteredTenants.length === 0 ? (
            <EmptyState icon={<Building2 className="h-5 w-5" />} title={tenants.length === 0 ? '还没有客户租户' : '没有匹配的客户'} description={tenants.length === 0 ? '签约首个客户后，在这里完成一次性开户。' : '调整搜索条件后重试。'} action={tenants.length === 0 && canManage ? <Button type="button" onClick={openCreate}><UserRoundPlus className="h-4 w-4" />开通首个客户</Button> : undefined} />
          ) : (
            <table className="min-w-[900px]"><thead><tr><th>客户</th><th>套餐与到期</th><th>状态</th><th>最高用量</th><th>告警</th><th className="w-20">操作</th></tr></thead><tbody>
              {filteredTenants.map((tenant) => {
                const status = tenantStatusView(tenant.tenantStatus)
                const ratio = Math.round((tenant.maxUsageRatio || 0) * 100)
                return <tr key={tenant.tenantId}><td><strong className="font-medium text-zinc-950">{tenant.tenantName}</strong><div className="mt-1 text-xs text-zinc-500">租户 ID {tenant.tenantId}</div></td><td><div>{tenant.packageName || '未配置'}</div><div className="mt-1 text-xs text-zinc-500">{tenant.expiresAt || '长期有效'}</div></td><td><Badge tone={status.tone}>{status.label}</Badge></td><td><div className="flex min-w-40 items-center justify-between gap-3 text-xs"><span>{tenant.maxUsageLabel || '-'}</span><span>{ratio}%</span></div><div className="mt-2"><ProgressBar value={ratio} tone={usageTone(tenant.maxUsageRatio || 0)} /></div></td><td>{tenant.openAlertCount > 0 ? <Badge tone="danger">{tenant.openAlertCount} 条</Badge> : <Badge tone="success">正常</Badge>}</td><td><Button type="button" variant="ghost" onClick={() => openTenant(tenant)}><Eye className="h-4 w-4" />详情</Button></td></tr>
              })}
            </tbody></table>
          )}
        </TableShell>
      </section>

      <Dialog open={createOpen} onOpenChange={(open) => { if (!open) closeCreate() }} title="开通客户租户" description="创建 SaaS 租户、套餐快照和首个 Dashboard 管理员" footer={<><Button type="button" variant="secondary" onClick={closeCreate}>取消</Button><ConfirmAction title="确认开户" summary={<div className="space-y-1"><p>客户：{createForm.tenantName || '未填写'}</p><p>管理员：{createForm.adminName || '未填写'} / {createForm.adminLoginIdentifier || '未填写'}</p><p>套餐：{createPlan ? `${createPlan.name}（ID ${createPlan.id}，版本 ${createPlan.version}）` : '未选择'}</p><p>企微模式：{createForm.wecomIntegrationMode ? weComModeLabel(createForm.wecomIntegrationMode) : '未选择'}</p><p>模式在开户后不可切换；未填写企微配置仍可进入 Dashboard 查看空数据。</p><p>系统将在成功响应中仅展示一次激活入口，不会创建或传递初始密码，也不会自动发送邮件或短信。</p></div>} confirmLabel="确认开户" loading={createMutation.isPending} onConfirm={() => { const operationEpoch = ++activationOperationEpochRef.current; observedCreateOperationEpochRef.current = operationEpoch; return createMutation.mutateAsync({ operationEpoch, form: { ...createForm } }) }}>提交开户</ConfirmAction></>}>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="客户名称" className="sm:col-span-2"><Input value={createForm.tenantName} onChange={(event) => setCreateForm((form) => ({ ...form, tenantName: event.target.value }))} placeholder="客户公司名称" autoFocus /></Field>
          <Field label="管理员姓名"><Input value={createForm.adminName} onChange={(event) => setCreateForm((form) => ({ ...form, adminName: event.target.value }))} placeholder="超级管理员" /></Field>
          <Field label="管理员手机号"><Input value={createForm.adminLoginIdentifier} onChange={(event) => setCreateForm((form) => ({ ...form, adminLoginIdentifier: event.target.value }))} inputMode="numeric" maxLength={11} placeholder="11 位手机号" /></Field>
          <Field label="开户企微对接模式" hint="必选；开户后不可切换。"><Select value={createForm.wecomIntegrationMode} onChange={(event) => setCreateForm((form) => ({ ...form, wecomIntegrationMode: event.target.value as CreateTenantForm['wecomIntegrationMode'] }))}><option value="">请选择企微对接模式</option><option value="self_built">自建应用（配置由 Dashboard 企业资料维护）</option><option value="third_party_delegated">第三方代开发应用（配置由 SaaS 维护）</option></Select></Field>
          <Field label="销售套餐" hint="开户时复制平台权威的 26 项额度和版本，不可在此编辑。"><Select value={createForm.packageId} onChange={(event) => setCreateForm((form) => ({ ...form, packageId: event.target.value }))}><option value="">请选择已启用套餐</option>{enabledPackages.map((item) => <option key={item.id} value={item.id}>{item.name}（ID {item.id} / v{item.version}）</option>)}</Select></Field>
          <Field label="订阅状态"><Select value={createForm.subscriptionStatus} onChange={(event) => setCreateForm((form) => ({ ...form, subscriptionStatus: event.target.value as CreateTenantForm['subscriptionStatus'] }))}><option value="trialing">试用中</option><option value="active">有效</option></Select></Field>
          <Field label="计费周期"><Select value={createForm.billingCycle} onChange={(event) => setCreateForm((form) => ({ ...form, billingCycle: event.target.value as CreateTenantForm['billingCycle'] }))}><option value="custom">定制</option><option value="monthly">月付</option><option value="yearly">年付</option><option value="lifetime">长期</option></Select></Field>
          <Field label="到期日期"><Input type="date" value={createForm.expiresAt} onChange={(event) => setCreateForm((form) => ({ ...form, expiresAt: event.target.value }))} /></Field>
          <p className="sm:col-span-2 text-xs leading-5 text-zinc-500">幂等键由页面生成并保留到成功；409 或网络错误不会清空表单。激活完成后请通过受控流程交付，不要在备注、日志或截图中复制令牌。</p>
        </div>
      </Dialog>

      <Dialog open={activationDeliveryOpen} onOpenChange={(open) => { if (!open) clearActivationDelivery() }} title="一次性激活入口" description="完整入口只在本次成功响应后显示，关闭后会从页面状态和 DOM 清除。" size="sm">
        <div className="space-y-3"><p className="text-sm text-amber-800">请由平台管理员通过安全的受控渠道交付；系统没有邮件、短信或企微自动发送能力。</p>{activationURL ? <><div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-950"><p>接收租户：{activationDelivery?.tenantName}（租户 {activationDelivery?.tenantId}）</p><p className="mt-1">接收账号：{activationDelivery?.accountName}（{activationDelivery?.accountHint}，用户 {activationDelivery?.dashboardUserId}）</p></div><code className="block break-all rounded-md bg-zinc-950 px-3 py-3 text-xs text-emerald-300">{activationURL}</code><p className="text-xs text-zinc-500">有效期至：{activationDelivery?.expiresAt ? formatDate(activationDelivery.expiresAt) : '服务端未返回'}</p><div className="activation-delivery-actions flex flex-wrap gap-2"><Button type="button" onClick={async () => { try { await navigator.clipboard.writeText(activationURL); toast.success('激活入口已复制，请安全交付') } catch { toast.error('复制失败，请使用受控设备手动复制') } }}>复制激活入口</Button><Button type="button" variant="secondary" onClick={clearActivationDelivery}>我已记录并关闭</Button></div></> : <p role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">服务端返回的激活入口不符合 fragment 安全合同，页面已拒绝展示。</p>}</div>
      </Dialog>

      <Dialog open={selectedTenantId > 0} onOpenChange={(open) => { if (!open) { selectedTenantIDRef.current = 0; activationOperationEpochRef.current += 1; weComOperationEpochRef.current += 1; closeAIProvider(); closeWeComEditor(); setWeComForm(emptyWeComIntegrationForm()); setWeComError(''); setSelectedTenantId(0) } }} title={selectedTenant?.tenantName || '客户详情'} description={selectedTenant ? `租户 ID ${selectedTenant.tenantId}` : '正在加载'} size="lg">
        {detailQuery.isLoading && <LoadingState label="正在加载客户详情" />}
        {detailQuery.isError && <ErrorState message={errorMessage(detailQuery.error, '无法加载客户详情')} onRetry={() => detailQuery.refetch()} />}
        {selectedTenant && <div className="space-y-6">
          <div className="grid gap-3 sm:grid-cols-3"><div className="rounded-md border border-zinc-200 p-3"><span className="text-xs text-zinc-500">当前套餐</span><strong className="mt-1 block text-sm">{selectedTenant.packageName || '未配置'}</strong></div><div className="rounded-md border border-zinc-200 p-3"><span className="text-xs text-zinc-500">租户状态</span><div className="mt-1"><Badge tone={tenantStatusView(selectedTenant.tenantStatus).tone}>{tenantStatusView(selectedTenant.tenantStatus).label}</Badge></div></div><div className="rounded-md border border-zinc-200 p-3"><span className="text-xs text-zinc-500">套餐到期</span><strong className="mt-1 block text-sm">{selectedTenant.expiresAt || '长期有效'}</strong></div></div>
          {canReadAIProvider && <section className="space-y-4 rounded-lg border border-violet-200 bg-violet-50/40 p-4" aria-label="企微对接模式">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <SectionHeader title="企微对接模式" description="模式在开户时确定且不可切换；配置入口按模式严格隔离。" />
              {canManageAIProvider && weComIntegrationQuery.data?.current?.mode === 'third_party_delegated' && <Button type="button" variant="secondary" onClick={openWeComEditor} disabled={weComIntegrationQuery.isLoading}>配置第三方应用</Button>}
            </div>
            {weComIntegrationQuery.isLoading && <LoadingState label="正在加载企微对接配置" />}
            {weComIntegrationQuery.isError && <ErrorState message={errorMessage(weComIntegrationQuery.error, '无法加载企微对接配置')} onRetry={() => weComIntegrationQuery.refetch()} />}
            {weComIntegrationQuery.data && <>
              <WeComIntegrationSummary label="当前" record={weComIntegrationQuery.data.current} />
              <p className="rounded-md border border-violet-100 bg-white px-3 py-2 text-sm text-zinc-700">{weComIntegrationQuery.data.current?.mode === 'self_built' ? '自建应用的 CorpID、AgentID、应用 Secret 和会话存档配置由租户管理员在 Dashboard「唯一企业资料」维护；SaaS 不保存或展示这些 Secret。' : '第三方代开发应用由 SaaS 安全写入 Provider App ID 和永久授权码；未配置时仍可正常登录 Dashboard 查看空数据。'}</p>
              <p className="text-xs text-zinc-500">权威企业绑定 ID：{weComIntegrationQuery.data.corpId}。模式和企业绑定均由服务端确定，页面没有切换入口。</p>
              {weComError && <p role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{weComError}</p>}
              {!canManageAIProvider && <p className="text-xs text-zinc-500">当前账号只有查看权限，不能写入第三方应用配置。</p>}
            </>}
            <div className="space-y-2" aria-label="企微集成审计时间线">
              <strong className="text-sm text-zinc-800">审计时间线</strong>
              {weComAuditQuery.isLoading && <LoadingState label="正在加载企微审计" />}
              {weComAuditQuery.isError && <ErrorState message={errorMessage(weComAuditQuery.error, '无法加载企微审计')} onRetry={() => weComAuditQuery.refetch()} />}
              {weComAuditQuery.data && (weComAuditQuery.data.length === 0 ? <p className="text-xs text-zinc-500">暂无企微集成变更审计。</p> : <ol className="space-y-2">{weComAuditQuery.data.map((audit) => <li key={audit.id} className="rounded-md border border-violet-100 bg-white px-3 py-2 text-xs"><div className="flex flex-wrap justify-between gap-2"><strong className="break-all text-zinc-800">{audit.action}</strong><time className="text-zinc-500">{formatDate(audit.createdAt)}</time></div><p className="mt-1 break-all text-zinc-500">目标 {audit.targetId || '-'} · 操作人 {audit.actorUserId}</p></li>)}</ol>) }
            </div>
          </section>}
          {canReadAIProvider && <section className="space-y-4 rounded-lg border border-blue-200 bg-blue-50/40 p-4" aria-label="租户 AI 分析模型">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <SectionHeader title="租户 AI 分析模型" description="每个客户独立配置 Provider、模型、密钥与有效期；页面只显示脱敏状态。" />
              {canManageAIProvider && <Button type="button" variant="secondary" onClick={() => { aiProviderMutation.reset(); openAIProvider() }} disabled={aiProviderQuery.isLoading}><Bot className="h-4 w-4" />配置 AI 模型</Button>}
            </div>
            {aiProviderQuery.isLoading && <LoadingState label="正在加载租户 AI 配置" />}
            {aiProviderQuery.isError && <ErrorState message={errorMessage(aiProviderQuery.error, '无法加载租户 AI 配置')} onRetry={() => aiProviderQuery.refetch()} />}
            {aiProviderQuery.data && <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <div className="rounded-md border border-blue-100 bg-white p-3"><span className="text-xs text-zinc-500">Provider / 模型</span><strong className="mt-1 block break-words text-sm">{aiProviderQuery.data.configured ? `${aiProviderQuery.data.provider.providerCode} / ${aiProviderQuery.data.provider.model}` : '尚未配置'}</strong></div>
              <div className="rounded-md border border-blue-100 bg-white p-3"><span className="text-xs text-zinc-500">当前状态</span><div className="mt-1"><Badge tone={aiProviderView?.tone || 'neutral'}>{aiProviderView?.label || '未知'}</Badge><p className="mt-1 text-xs text-zinc-500">{aiProviderView?.hint}</p></div></div>
              <div className="rounded-md border border-blue-100 bg-white p-3"><span className="text-xs text-zinc-500">密钥提示</span><strong className="mt-1 block text-sm">{aiProviderQuery.data.provider.apiKeyHint || '无密钥提示'}</strong></div>
              <div className="rounded-md border border-blue-100 bg-white p-3"><span className="text-xs text-zinc-500">配置版本</span><strong className="mt-1 block text-sm">v{aiProviderQuery.data.provider.version || 0}</strong><span className="mt-1 block text-xs text-zinc-500">更新于 {aiProviderQuery.data.provider.updatedAt ? formatDate(aiProviderQuery.data.provider.updatedAt) : '未知'}</span></div>
            </div>}
            {!canManageAIProvider && <p className="text-xs text-zinc-500">当前账号只有查看权限，不能修改模型或密钥。</p>}
          </section>}
          {canManage && <section className="space-y-4 rounded-lg border border-emerald-200 bg-emerald-50/40 p-4" aria-label="Dashboard 超级管理员治理"><SectionHeader title="Dashboard 超级管理员治理" description="只展示服务端判定可执行的动作；不可执行的操作不会发送请求。" />{governanceQuery.isLoading && <LoadingState label="正在加载 Dashboard 身份" />}{governanceQuery.isError && <ErrorState message={errorMessage(governanceQuery.error, '无法加载治理列表')} onRetry={() => governanceQuery.refetch()} />}{governanceQuery.data && <><div className="grid gap-3 sm:grid-cols-2"><Field label="治理绑定版本"><div className="flex h-9 items-center rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-700">v{governanceQuery.data.bindingVersion}</div></Field>{governanceTargets.length > 0 ? <Field label="重发/停用/恢复对象"><Select value={targetAdminId} onChange={(event) => setTargetAdminId(event.target.value)}>{governanceTargets.map((identity) => <option key={identity.id} value={identity.id}>{identity.name || identity.loginIdentifier}（{identity.id}）</option>)}</Select></Field> : <div className="rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-600">当前没有可重发、停用或恢复的账号。</div>}{governanceCurrents.length > 0 ? <Field label="当前超管"><Select value={currentAdminId} onChange={(event) => setCurrentAdminId(event.target.value)}>{governanceCurrents.map((identity) => <option key={identity.id} value={identity.id}>{identity.name || identity.loginIdentifier}（{identity.id}）</option>)}</Select></Field> : null}{governanceCandidates.length > 0 ? <Field label="替换候选"><Select value={replacementAdminId} onChange={(event) => setReplacementAdminId(event.target.value)}>{governanceCandidates.map((identity) => <option key={identity.id} value={identity.id}>{identity.name || identity.loginIdentifier}（{identity.id}）</option>)}</Select></Field> : <div className="rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-600">暂无已激活的替换候选；请先在 Dashboard 完成候选账号激活。</div>}</div>{hasLastSuperAdminBlock && <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">至少保留一名可登录超级管理员，当前唯一有效超管不能停用。</p>}<div className="mb-3 text-xs text-zinc-500">当前：{currentIdentity?.name || '未选择'}；候选：{replacementIdentity?.name || '未选择'}；治理对象：{targetIdentity?.name || '无'}</div>{governanceMutationError && <p role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{errorMessage(governanceMutationError, '治理操作失败，当前选择已保留。')}</p>}<div className="flex flex-wrap gap-2">{governanceCan(targetIdentity, 'resend_activation') && <ConfirmAction title="确认重发激活" summary={governanceSummary} loading={resendMutation.isPending} onConfirm={() => { const tenantId = selectedTenantIDRef.current; const expectedVersion = governanceQuery.data?.bindingVersion || 0; const identity = targetIdentity; const dashboardUserId = identity?.id || 0; if (!tenantId || !expectedVersion || !dashboardUserId || !selectedTenant || !identity) throw new Error('治理列表尚未加载完成'); const requestKeySlot = governanceRequestKeySlot('resend', tenantId, [dashboardUserId], expectedVersion); const requestKey = requestKeys.current[requestKeySlot] || (requestKeys.current[requestKeySlot] = makeRequestKey('dashboard-resend')); const operationEpoch = ++activationOperationEpochRef.current; observedResendOperationEpochRef.current = operationEpoch; return resendMutation.mutateAsync({ tenantId, dashboardUserId, expectedVersion, requestKeySlot, requestKey, tenantName: selectedTenant.tenantName, accountName: identity.name || 'Dashboard 管理员', accountHint: safeLoginHint(identity.loginIdentifier), operationEpoch }) }}><RefreshCw className="h-4 w-4" />重发激活</ConfirmAction>}{currentIdentity && replacementIdentity && <ConfirmAction title="确认替换超管" summary={`${governanceSummary}；当前 ${currentIdentity.name || '未选择'} → 新 ${replacementIdentity.name || '未选择'}`} loading={replaceMutation.isPending} onConfirm={() => { const tenantId = selectedTenantIDRef.current; const currentAdminId = currentIdentity.id; const newAdminId = replacementIdentity.id; const expectedVersion = governanceQuery.data?.bindingVersion || 0; if (!tenantId || !currentAdminId || !newAdminId || currentAdminId === newAdminId || !expectedVersion) throw new Error('请选择有效的当前超管、替换候选'); const requestKeySlot = governanceRequestKeySlot('replace', tenantId, [currentAdminId, newAdminId], expectedVersion); const requestKey = requestKeys.current[requestKeySlot] || (requestKeys.current[requestKeySlot] = makeRequestKey('dashboard-replace')); return replaceMutation.mutateAsync({ tenantId, currentAdminId, newAdminId, expectedVersion, requestKeySlot, requestKey }) }}><ShieldCheck className="h-4 w-4" />替换超管</ConfirmAction>}{governanceCan(targetIdentity, 'disable') && <ConfirmAction title="确认停用超管" summary={governanceSummary} variant="danger" loading={statusMutation.isPending} onConfirm={() => { const tenantId = selectedTenantIDRef.current; const targetUserId = targetIdentity?.id || 0; const expectedVersion = governanceQuery.data?.bindingVersion || 0; if (!tenantId || !targetUserId || !expectedVersion) throw new Error('治理列表尚未加载完成'); const requestKeySlot = governanceRequestKeySlot('disable', tenantId, [targetUserId], expectedVersion); const requestKey = requestKeys.current[requestKeySlot] || (requestKeys.current[requestKeySlot] = makeRequestKey('dashboard-disable')); return statusMutation.mutateAsync({ tenantId, targetUserId, enabled: false, expectedVersion, requestKeySlot, requestKey }) }}><UserX className="h-4 w-4" />停用超管</ConfirmAction>}{governanceCan(targetIdentity, 'restore') && <ConfirmAction title="确认恢复超管" summary={governanceSummary} variant="secondary" loading={statusMutation.isPending} onConfirm={() => { const tenantId = selectedTenantIDRef.current; const targetUserId = targetIdentity?.id || 0; const expectedVersion = governanceQuery.data?.bindingVersion || 0; if (!tenantId || !targetUserId || !expectedVersion) throw new Error('治理列表尚未加载完成'); const requestKeySlot = governanceRequestKeySlot('restore', tenantId, [targetUserId], expectedVersion); const requestKey = requestKeys.current[requestKeySlot] || (requestKeys.current[requestKeySlot] = makeRequestKey('dashboard-restore')); return statusMutation.mutateAsync({ tenantId, targetUserId, enabled: true, expectedVersion, requestKeySlot, requestKey }) }}><UserRoundCog className="h-4 w-4" />恢复超管</ConfirmAction>}</div></>}</section>}
          <section className="space-y-3"><SectionHeader title="核心用量" /><div className="grid gap-3 sm:grid-cols-2">{coreMetrics.map((metric) => { const ratio = metric.limit > 0 ? metric.current / metric.limit : 0; return <div key={metric.metric} className="rounded-md border border-zinc-200 p-3"><div className="flex items-center justify-between gap-3 text-sm"><span>{metric.label}</span><strong>{metric.current} / {metric.limit}</strong></div><div className="mt-3"><ProgressBar value={ratio * 100} tone={usageTone(ratio)} /></div></div> })}</div></section>
          <section className="space-y-3"><SectionHeader title="最近变更" /><TableShell>{(detailQuery.data?.operations || []).length === 0 ? <EmptyState icon={<Building2 className="h-5 w-5" />} title="暂无变更记录" description="租户变更会自动记录。" /> : <table><thead><tr><th>目标</th><th>说明</th><th>时间</th></tr></thead><tbody>{(detailQuery.data?.operations || []).slice(0, 8).map((item) => <tr key={item.id}><td>{item.targetName || item.targetType}</td><td>{item.remark || item.action}</td><td>{formatDate(item.createdAt)}</td></tr>)}</tbody></table>}</TableShell></section>
        </div>}
      </Dialog>

      <Dialog open={weComEditorOpen && selectedTenantId > 0} onOpenChange={(open) => { if (!open) closeWeComEditor() }} title="配置第三方代开发应用" description={selectedTenant ? `${selectedTenant.tenantName}（租户 ${selectedTenant.tenantId}），企微模式不可切换` : '租户配置'} size="lg" footer={<div className="integration-actions flex flex-wrap gap-2"><Button type="button" variant="secondary" onClick={closeWeComEditor}>取消</Button><Button type="button" loading={saveWeComMutation.isPending} onClick={submitWeComCandidate}>安全保存</Button></div>}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2" aria-label="第三方应用配置表单">
          <Field label="对接模式"><div className="flex h-9 items-center rounded-md border border-zinc-200 bg-zinc-50 px-3 text-sm text-zinc-700">第三方代开发应用（不可切换）</div></Field>
          <Field label="配置版本"><div className="flex h-9 items-center rounded-md border border-zinc-200 bg-zinc-50 px-3 text-sm text-zinc-700">v{weComForm.version}</div></Field>
          <Field label="Provider App ID"><Input value={weComForm.providerAppId} onChange={(event) => setWeComForm((form) => ({ ...form, providerAppId: event.target.value }))} placeholder="Provider App ID" /></Field>
          <Field label="永久授权码" hint="仅安全写入或轮换，保存后立即从页面清除且不会回显。"><Input type="password" autoComplete="new-password" value={weComForm.permanentCode} onChange={(event) => setWeComForm((form) => ({ ...form, permanentCode: event.target.value }))} placeholder="永久授权码" /></Field>
          <fieldset className="min-w-0 space-y-1.5 text-sm text-zinc-700 sm:col-span-2"><legend className="font-medium">授权能力</legend><div className="space-y-3 rounded-md border border-zinc-200 bg-white p-3"><div className="flex items-center justify-between gap-3"><p className="text-sm text-zinc-700">可授权能力</p><Button type="button" variant="secondary" onClick={() => setWeComForm((form) => ({ ...form, scope: mergeDelegatedScope(delegatedCapabilityCatalog.map((capability) => capability.code), form.scope) }))}>全部开启</Button></div><div className="grid gap-2">{delegatedCapabilityCatalog.map((capability) => { const inputId = `wecom-capability-${capability.code.replace('.', '-')}`; const descriptionId = `${inputId}-description`; return <div key={capability.code} className="flex items-start gap-3 rounded-md border border-zinc-200 p-3 text-sm text-zinc-700"><input id={inputId} type="checkbox" aria-describedby={descriptionId} className="mt-0.5 h-4 w-4" checked={weComForm.scope.includes(capability.code)} onChange={(event) => setWeComForm((form) => ({ ...form, scope: mergeDelegatedScope(event.target.checked ? [...selectedDelegatedCapabilities(form.scope).map((item) => item.code), capability.code] : selectedDelegatedCapabilities(form.scope).map((item) => item.code).filter((code) => code !== capability.code), form.scope) }))} /><div><label htmlFor={inputId} className="cursor-pointer font-medium text-zinc-900">{capability.name}</label><p id={descriptionId} className="mt-1 text-xs text-zinc-500">{capability.description}</p></div></div> })}</div></div><p className="text-xs font-normal text-zinc-500">请选择当前应用需要的数据范围。</p></fieldset>
          {weComError && <p role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 sm:col-span-2">{weComError}</p>}
          <p className="text-xs leading-5 text-zinc-500 sm:col-span-2">未填写第三方配置也不影响 Dashboard 浏览；保存只更新当前模式的加密授权，不会调用真实企微。</p>
        </div>
      </Dialog>

      <Dialog open={aiProviderOpen && selectedTenantId > 0} onOpenChange={(open) => { if (!open) closeAIProvider() }} title="配置租户 AI 模型" description={selectedTenant ? `${selectedTenant.tenantName}（租户 ${selectedTenant.tenantId}）` : '租户配置'} size="lg" footer={<><Button type="button" variant="secondary" onClick={closeAIProvider}>取消</Button><Button type="button" loading={aiProviderMutation.isPending} onClick={() => aiProviderMutation.mutate()}>保存 AI 配置</Button></>}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2" aria-label="租户 AI Provider 表单">
          <Field label="Provider 厂商"><Select value={aiProviderForm.providerCode} onChange={(event) => {
            const providerCode = event.target.value as TenantAIProviderForm['providerCode']
            const preset = providerCode === 'custom' ? null : providerPresets[providerCode]
            setAIProviderForm((form) => ({ ...form, providerCode, apiKey: '', baseUrl: preset?.baseUrl || '', model: '' }))
          }}><option value="deepseek">DeepSeek</option><option value="openai">OpenAI</option><option value="dashscope">阿里云百炼 / DashScope</option><option value="custom">OpenAI 兼容服务</option></Select></Field>
          <Field label="运行状态"><Select value={aiProviderForm.status} onChange={(event) => setAIProviderForm((form) => ({ ...form, status: event.target.value as TenantAIProviderForm['status'] }))}><option value="active">启用</option><option value="disabled">停用</option></Select></Field>
          <Field label="接口地址" hint="必须为可公开访问的 HTTPS 地址；服务端仍会执行 SSRF 防护。" className="sm:col-span-2"><Input type="url" value={aiProviderForm.baseUrl} onChange={(event) => setAIProviderForm((form) => ({ ...form, baseUrl: event.target.value }))} placeholder="https://api.example.com/v1" /></Field>
          <Field label="模型名称"><Input value={aiProviderForm.model} onChange={(event) => setAIProviderForm((form) => ({ ...form, model: event.target.value }))} placeholder="模型标识" /></Field>
          <Field label="API Key" hint={aiProviderQuery.data?.configured ? `留空保留现有密钥（${aiProviderQuery.data.provider.apiKeyHint || '已配置'}）` : '首次配置必须填写；保存失败或关闭窗口后立即清空。'}><div className="relative"><KeyRound className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-zinc-400" /><Input type="password" autoComplete="new-password" value={aiProviderForm.apiKey} onChange={(event) => setAIProviderForm((form) => ({ ...form, apiKey: event.target.value }))} className="pl-9" placeholder="留空则保留现有密钥" /></div></Field>
          <Field label="生效时间"><Input type="datetime-local" value={aiProviderForm.effectiveAt} onChange={(event) => setAIProviderForm((form) => ({ ...form, effectiveAt: event.target.value }))} /></Field>
          <Field label="失效时间"><Input type="datetime-local" value={aiProviderForm.expiresAt} onChange={(event) => setAIProviderForm((form) => ({ ...form, expiresAt: event.target.value }))} /></Field>
          {aiProviderSaveError && <p role="alert" className="sm:col-span-2 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{aiProviderSaveError}</p>}
          {aiProviderQuery.data?.provider.credentialProtection === 'unavailable' && <p className="sm:col-span-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">现有密钥当前不可解密。请填写新 API Key 后保存；运行时会继续失败关闭，直到凭证保护恢复。</p>}
          <p className="sm:col-span-2 text-xs leading-5 text-zinc-500">保存只更新加密配置，不会测试连接或触发 AI 分析。模型运行还必须满足租户有效期、企业绑定和手动运行权限。</p>
        </div>
      </Dialog>
    </div>
  )
}
