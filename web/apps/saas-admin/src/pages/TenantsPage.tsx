import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bot, Building2, Eye, KeyRound, Plus, RefreshCw, Search, ShieldCheck, UserRoundCog, UserRoundPlus, UserX } from 'lucide-react'
import { toast } from 'sonner'
import { ApiError, apiRequest, executeGoverned, hasPermission, jsonRequest } from '@/lib/api'
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
})

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError) return `${error.machineCode}：${error.message}${error.status === 409 ? '；请刷新治理列表后重试，当前选择已保留。' : ''}`
  return error instanceof Error ? error.message : fallback
}

function aiProviderErrorMessage(error: unknown) {
  return errorMessage(error, 'AI 模型配置保存失败，API Key 已从页面清除')
}

function aiProviderConflictMessage(error: ApiError, refreshed: boolean) {
  return refreshed
    ? `${error.machineCode}：配置已由其他管理员更新；页面已载入最新版本，请重新确认后保存。`
    : `${error.machineCode}：配置已由其他管理员更新，但最新版本刷新失败；请检查网络后重试。`
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

export default function TenantsPage({ profile, approvalMode }: PageProps) {
  const queryClient = useQueryClient()
  const canManage = hasPermission(profile.permissions, 'platform.tenants.manage')
  const canReadAIProvider = hasPermission(profile.permissions, 'platform.integrations.read')
  const canManageAIProvider = hasPermission(profile.permissions, 'platform.integrations.manage')
  const requestKeys = useRef<Record<string, string>>({})
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const [packageFilter, setPackageFilter] = useState('all')
  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] = useState<CreateTenantForm>(emptyCreateForm)
  const [selectedTenantId, setSelectedTenantId] = useState(0)
  const [targetAdminId, setTargetAdminId] = useState('')
  const [currentAdminId, setCurrentAdminId] = useState('')
  const [replacementAdminId, setReplacementAdminId] = useState('')
  const [activationToken, setActivationToken] = useState('')
  const [activationTokenOpen, setActivationTokenOpen] = useState(false)
  const [aiProviderOpen, setAIProviderOpen] = useState(false)
  const [aiProviderForm, setAIProviderForm] = useState<TenantAIProviderForm>(defaultProviderForm)
  const [aiProviderSaveError, setAIProviderSaveError] = useState('')

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
        const message = aiProviderConflictMessage(error, refreshSucceeded)
        setAIProviderSaveError(message)
        toast.error(message)
        return
      }
      const message = aiProviderErrorMessage(error)
      setAIProviderSaveError(message)
      toast.error(message)
    },
  })

  const invalidateTenantData = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.overview }),
      queryClient.invalidateQueries({ queryKey: ['tenant-detail'] }),
      queryClient.invalidateQueries({ queryKey: queryKeys.operations }),
    ])
  }

  const createMutation = useMutation<GovernedResult<DashboardAdminProvisionResult>, unknown>({
    mutationFn: async () => {
      const plan = (packagesQuery.data?.packages || []).find((item) => String(item.id) === createForm.packageId && item.status === 1)
      if (!plan) throw new Error('请选择已启用套餐')
      if (!createForm.tenantName.trim() || !createForm.adminName.trim()) throw new Error('请填写客户名称和管理员姓名')
      if (!/^1\d{10}$/.test(createForm.adminLoginIdentifier.trim())) throw new Error('管理员手机号格式不正确')
      const expiresAt = isoEndOfDate(createForm.expiresAt)
      if (!expiresAt) throw new Error('请填写有效到期日期')
      const payload = {
          tenantName: createForm.tenantName.trim(),
          packageId: plan.id,
          limits: packageLimitsSnapshot(plan),
          subscription: {
            packageCode: plan.code,
            status: createForm.subscriptionStatus,
            billingCycle: createForm.billingCycle,
            startsAt: new Date().toISOString(),
            expiresAt,
          },
          adminLoginIdentifier: createForm.adminLoginIdentifier.trim(),
          adminName: createForm.adminName.trim(),
          idempotencyKey: createForm.idempotencyKey,
          expectedVersion: plan.version,
      }
      return executeGoverned<DashboardAdminProvisionResult>({
        approvalMode,
        actionType: 'dashboard.tenant.provision',
        payload,
        approvalPayload: payload,
        approvalIdempotencyKey: createForm.idempotencyKey,
        reason: `开通客户租户 ${createForm.tenantName.trim()}`,
        directPath: '/dashboard/saasAdmin/tenants/provision',
        directHeaders: { 'X-Request-ID': createForm.idempotencyKey },
      })
    },
    onSuccess: async (result) => {
      if (result.approvalRequested) {
        setCreateOpen(false)
        toast.success('开户申请已提交审批，待独立复核后执行')
        await queryClient.invalidateQueries({ queryKey: queryKeys.approvals })
        return
      }
      setCreateOpen(false)
      setCreateForm(emptyCreateForm())
      if (result.data.activationToken) {
        setActivationToken(result.data.activationToken)
        setActivationTokenOpen(true)
      }
      toast.success(result.data.idempotent ? '开户请求已确认，未重复发放激活令牌' : '租户已开通，请安全交付一次性激活令牌')
      await invalidateTenantData()
    },
    onError: (error) => toast.error(errorMessage(error, '开户失败，表单内容已保留')),
  })

  const resendMutation = useMutation<GovernedResult<DashboardAdminGovernanceResult & { activationToken?: string }>, unknown>({
    mutationFn: async () => {
      const tenantId = selectedTenantId
      const userId = Number(targetAdminId)
      const version = governanceQuery.data?.bindingVersion || 0
      if (tenantId <= 0 || userId <= 0 || version <= 0) throw new Error('治理列表尚未加载完成')
      const requestKey = requestKeys.current.resend || (requestKeys.current.resend = makeRequestKey('dashboard-resend'))
      const payload = { targetUserId: userId, expectedVersion: version }
      return executeGoverned<DashboardAdminGovernanceResult & { activationToken?: string }>({
        approvalMode,
        actionType: 'dashboard.activation.resend',
        payload,
        approvalPayload: { tenantId, ...payload },
        approvalIdempotencyKey: requestKey,
        reason: `重发租户 ${tenantId} 的 Dashboard 激活`,
        directPath: `/dashboard/saasAdmin/tenants/${tenantId}/activation/resend`,
        directHeaders: { 'X-Request-ID': requestKey },
      })
    },
    onSuccess: async (result) => {
      if (result.approvalRequested) {
        toast.success('重发激活申请已提交审批，待独立复核后执行')
        await queryClient.invalidateQueries({ queryKey: queryKeys.approvals })
        return
      }
      delete requestKeys.current.resend
      if (result.data.activationToken) {
        setActivationToken(result.data.activationToken)
        setActivationTokenOpen(true)
      }
      toast.success(result.data.idempotent ? '重发请求已确认，激活令牌不会重复显示' : '已生成新的激活令牌')
      await governanceQuery.refetch()
      await invalidateTenantData()
    },
    onError: (error) => toast.error(errorMessage(error, '重发激活失败，目标信息已保留')),
  })

  const replaceMutation = useMutation<GovernedResult<DashboardAdminGovernanceResult>, unknown>({
    mutationFn: async () => {
      const tenantId = selectedTenantId
      const currentId = Number(currentAdminId)
      const nextId = Number(replacementAdminId)
      const version = governanceQuery.data?.bindingVersion || 0
      if (tenantId <= 0 || currentId <= 0 || nextId <= 0 || currentId === nextId || version <= 0) throw new Error('请选择有效的当前超管、替换候选')
      const requestKey = requestKeys.current.replace || (requestKeys.current.replace = makeRequestKey('dashboard-replace'))
      const payload = { currentAdminId: currentId, newAdminId: nextId, expectedVersion: version }
      return executeGoverned<DashboardAdminGovernanceResult>({
        approvalMode,
        actionType: 'dashboard.superadmin.replace',
        payload,
        approvalPayload: { tenantId, ...payload },
        approvalIdempotencyKey: requestKey,
        reason: `替换租户 ${tenantId} 的 Dashboard 超级管理员`,
        directPath: `/dashboard/saasAdmin/tenants/${tenantId}/super-admin/replace`,
        directHeaders: { 'X-Request-ID': requestKey },
      })
    },
    onSuccess: async (result) => {
      if (result.approvalRequested) {
        toast.success('超管替换申请已提交审批，待独立复核后执行')
        await queryClient.invalidateQueries({ queryKey: queryKeys.approvals })
        return
      }
      delete requestKeys.current.replace
      await governanceQuery.refetch()
      toast.success(result.data.idempotent ? '替换请求已确认' : '超级管理员已替换')
      await invalidateTenantData()
    },
    onError: (error) => toast.error(errorMessage(error, '替换超级管理员失败，表单内容已保留')),
  })

  const statusMutation = useMutation<GovernedResult<DashboardAdminGovernanceResult>, unknown, boolean>({
    mutationFn: async (enabled) => {
      const tenantId = selectedTenantId
      const userId = Number(targetAdminId)
      const version = governanceQuery.data?.bindingVersion || 0
      if (tenantId <= 0 || userId <= 0 || version <= 0) throw new Error('治理列表尚未加载完成')
      const operation = enabled ? 'restore' : 'disable'
      const requestKey = requestKeys.current[operation] || (requestKeys.current[operation] = makeRequestKey(`dashboard-${operation}`))
      const payload = { targetUserId: userId, enabled, expectedVersion: version }
      return executeGoverned<DashboardAdminGovernanceResult>({
        approvalMode,
        actionType: 'dashboard.superadmin.status',
        payload,
        approvalPayload: { tenantId, ...payload },
        approvalIdempotencyKey: requestKey,
        reason: `${enabled ? '恢复' : '停用'}租户 ${tenantId} 的 Dashboard 超级管理员`,
        directPath: `/dashboard/saasAdmin/tenants/${tenantId}/super-admin/status`,
        directHeaders: { 'X-Request-ID': requestKey },
      })
    },
    onSuccess: async (result, enabled) => {
      if (result.approvalRequested) {
        toast.success(`${enabled ? '恢复' : '停用'}超管申请已提交审批，待独立复核后执行`)
        await queryClient.invalidateQueries({ queryKey: queryKeys.approvals })
        return
      }
      delete requestKeys.current[enabled ? 'restore' : 'disable']
      await governanceQuery.refetch()
      toast.success(result.data.idempotent ? '状态请求已确认' : enabled ? '超级管理员已恢复' : '超级管理员已停用')
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
    setCreateForm({ ...emptyCreateForm(), packageId: enabledPackages[0] ? String(enabledPackages[0].id) : '' })
    setCreateOpen(true)
  }

  const openTenant = (tenant: TenantSummary) => {
    setSelectedTenantId(tenant.tenantId)
    setTargetAdminId('')
    setCurrentAdminId('')
    setReplacementAdminId('')
    closeAIProvider()
  }

  useEffect(() => {
    const identities = governanceQuery.data?.identities || []
    if (!selectedTenantId || identities.length === 0) return
    const current = identities.find((identity) => identity.isSuperAdmin && identity.userStatus === 1 && identity.identityStatus === 1 && Boolean(identity.activatedAt)) || identities.find((identity) => identity.isSuperAdmin)
    const candidate = identities.find((identity) => identity.userStatus === 1 && identity.identityStatus === 1 && Boolean(identity.activatedAt) && identity.id !== current?.id)
    setTargetAdminId((value) => identities.some((identity) => identity.id === Number(value) && identity.isSuperAdmin) ? value : current ? String(current.id) : '')
    setCurrentAdminId((value) => identities.some((identity) => identity.id === Number(value) && identity.isSuperAdmin) ? value : current ? String(current.id) : '')
    setReplacementAdminId((value) => identities.some((identity) => identity.id === Number(value) && identity.id !== Number(currentAdminId) && identity.userStatus === 1 && identity.identityStatus === 1 && Boolean(identity.activatedAt)) ? value : candidate ? String(candidate.id) : '')
  }, [currentAdminId, governanceQuery.data, selectedTenantId])

  const selectedTenant = detailQuery.data?.tenant
  const coreMetrics = (detailQuery.data?.metrics || []).filter((item) => ['users', 'corps', 'contacts', 'rooms', 'storage_mb', 'channel_codes'].includes(item.metric))
  const createPlan = enabledPackages.find((item) => String(item.id) === createForm.packageId)
  const governanceVersion = governanceQuery.data?.bindingVersion || 0
  const targetIdentity = governanceQuery.data?.identities.find((identity) => identity.id === Number(targetAdminId))
  const currentIdentity = governanceQuery.data?.identities.find((identity) => identity.id === Number(currentAdminId))
  const replacementIdentity = governanceQuery.data?.identities.find((identity) => identity.id === Number(replacementAdminId))
  const governanceSummary = selectedTenant ? `${selectedTenant.tenantName}（租户 ${selectedTenant.tenantId}），管理员 ${targetIdentity?.name || '未选择'}，绑定版本 ${governanceVersion || '加载中'}` : '请先打开一个客户租户详情。'
  const governanceMutationError = resendMutation.error || replaceMutation.error || statusMutation.error
  const aiProviderView = aiProviderQuery.data ? tenantAIProviderState(aiProviderQuery.data.configured, aiProviderQuery.data.provider) : null

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

      <Dialog open={createOpen} onOpenChange={setCreateOpen} title="开通客户租户" description="创建 SaaS 租户、套餐快照和首个 Dashboard 管理员" footer={<><Button type="button" variant="secondary" onClick={() => setCreateOpen(false)}>取消</Button><ConfirmAction title="确认开户" summary={<div className="space-y-1"><p>客户：{createForm.tenantName || '未填写'}</p><p>管理员：{createForm.adminName || '未填写'} / {createForm.adminLoginIdentifier || '未填写'}</p><p>套餐：{createPlan ? `${createPlan.name}（ID ${createPlan.id}，版本 ${createPlan.version}）` : '未选择'}</p><p>系统将在成功响应中仅展示一次激活令牌，不会创建或传递初始密码。</p></div>} confirmLabel="确认开户" loading={createMutation.isPending} onConfirm={() => createMutation.mutateAsync()}>提交开户</ConfirmAction></>}>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="客户名称" className="sm:col-span-2"><Input value={createForm.tenantName} onChange={(event) => setCreateForm((form) => ({ ...form, tenantName: event.target.value }))} placeholder="客户公司名称" autoFocus /></Field>
          <Field label="管理员姓名"><Input value={createForm.adminName} onChange={(event) => setCreateForm((form) => ({ ...form, adminName: event.target.value }))} placeholder="超级管理员" /></Field>
          <Field label="管理员手机号"><Input value={createForm.adminLoginIdentifier} onChange={(event) => setCreateForm((form) => ({ ...form, adminLoginIdentifier: event.target.value }))} inputMode="numeric" maxLength={11} placeholder="11 位手机号" /></Field>
          <Field label="销售套餐" hint="开户时复制平台权威的 26 项额度和版本，不可在此编辑。"><Select value={createForm.packageId} onChange={(event) => setCreateForm((form) => ({ ...form, packageId: event.target.value }))}><option value="">请选择已启用套餐</option>{enabledPackages.map((item) => <option key={item.id} value={item.id}>{item.name}（ID {item.id} / v{item.version}）</option>)}</Select></Field>
          <Field label="订阅状态"><Select value={createForm.subscriptionStatus} onChange={(event) => setCreateForm((form) => ({ ...form, subscriptionStatus: event.target.value as CreateTenantForm['subscriptionStatus'] }))}><option value="trialing">试用中</option><option value="active">有效</option></Select></Field>
          <Field label="计费周期"><Select value={createForm.billingCycle} onChange={(event) => setCreateForm((form) => ({ ...form, billingCycle: event.target.value as CreateTenantForm['billingCycle'] }))}><option value="custom">定制</option><option value="monthly">月付</option><option value="yearly">年付</option><option value="lifetime">长期</option></Select></Field>
          <Field label="到期日期"><Input type="date" value={createForm.expiresAt} onChange={(event) => setCreateForm((form) => ({ ...form, expiresAt: event.target.value }))} /></Field>
          <p className="sm:col-span-2 text-xs leading-5 text-zinc-500">幂等键由页面生成并保留到成功；409 或网络错误不会清空表单。激活完成后请通过受控流程交付，不要在备注、日志或截图中复制令牌。</p>
        </div>
      </Dialog>

      <Dialog open={activationTokenOpen} onOpenChange={(open) => { setActivationTokenOpen(open); if (!open) setActivationToken('') }} title="一次性激活令牌" description="此令牌只在本次成功响应后显示，关闭后无法再次查看。" size="sm">
        <div className="space-y-3"><p className="text-sm text-amber-800">请使用安全的受控交付渠道传给对应管理员；不要写入工单正文、日志或截图。</p><code className="block break-all rounded-md bg-zinc-950 px-3 py-3 text-xs text-emerald-300">{activationToken}</code><Button type="button" variant="secondary" onClick={() => { setActivationTokenOpen(false); setActivationToken('') }}>我已记录并关闭</Button></div>
      </Dialog>

      <Dialog open={selectedTenantId > 0} onOpenChange={(open) => { if (!open) { closeAIProvider(); setSelectedTenantId(0) } }} title={selectedTenant?.tenantName || '客户详情'} description={selectedTenant ? `租户 ID ${selectedTenant.tenantId}` : '正在加载'} size="lg">
        {detailQuery.isLoading && <LoadingState label="正在加载客户详情" />}
        {detailQuery.isError && <ErrorState message={errorMessage(detailQuery.error, '无法加载客户详情')} onRetry={() => detailQuery.refetch()} />}
        {selectedTenant && <div className="space-y-6">
          <div className="grid gap-3 sm:grid-cols-3"><div className="rounded-md border border-zinc-200 p-3"><span className="text-xs text-zinc-500">当前套餐</span><strong className="mt-1 block text-sm">{selectedTenant.packageName || '未配置'}</strong></div><div className="rounded-md border border-zinc-200 p-3"><span className="text-xs text-zinc-500">租户状态</span><div className="mt-1"><Badge tone={tenantStatusView(selectedTenant.tenantStatus).tone}>{tenantStatusView(selectedTenant.tenantStatus).label}</Badge></div></div><div className="rounded-md border border-zinc-200 p-3"><span className="text-xs text-zinc-500">套餐到期</span><strong className="mt-1 block text-sm">{selectedTenant.expiresAt || '长期有效'}</strong></div></div>
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
          {canManage && <section className="space-y-4 rounded-lg border border-emerald-200 bg-emerald-50/40 p-4"><SectionHeader title="Dashboard 超级管理员治理" description="对象和绑定版本来自服务端只读治理列表；页面不接受手填用户 ID 或版本。" />{governanceQuery.isLoading && <LoadingState label="正在加载 Dashboard 身份" />}{governanceQuery.isError && <ErrorState message={errorMessage(governanceQuery.error, '无法加载治理列表')} onRetry={() => governanceQuery.refetch()} />}{governanceQuery.data && <><div className="grid gap-3 sm:grid-cols-2"><Field label="治理绑定版本"><div className="flex h-9 items-center rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-700">v{governanceQuery.data.bindingVersion}</div></Field><Field label="重发/停用/恢复对象"><Select value={targetAdminId} onChange={(event) => setTargetAdminId(event.target.value)}><option value="">请选择 Dashboard 超管</option>{governanceQuery.data.identities.filter((identity) => identity.isSuperAdmin).map((identity) => <option key={identity.id} value={identity.id}>{identity.name || identity.loginIdentifier}（{identity.id}，{identity.userStatus === 1 && identity.identityStatus === 1 && identity.activatedAt ? '启用' : '停用'}）</option>)}</Select></Field><Field label="当前超管"><Select value={currentAdminId} onChange={(event) => setCurrentAdminId(event.target.value)}><option value="">请选择当前超管</option>{governanceQuery.data.identities.filter((identity) => identity.isSuperAdmin).map((identity) => <option key={identity.id} value={identity.id}>{identity.name || identity.loginIdentifier}（{identity.id}）</option>)}</Select></Field><Field label="替换候选"><Select value={replacementAdminId} onChange={(event) => setReplacementAdminId(event.target.value)}><option value="">请选择已激活候选</option>{governanceQuery.data.identities.filter((identity) => identity.id !== Number(currentAdminId) && identity.userStatus === 1 && identity.identityStatus === 1 && Boolean(identity.activatedAt)).map((identity) => <option key={identity.id} value={identity.id}>{identity.name || identity.loginIdentifier}（{identity.id}）</option>)}</Select></Field></div><div className="mb-3 text-xs text-zinc-500">当前：{currentIdentity?.name || '未选择'}；候选：{replacementIdentity?.name || '未选择'}；目标状态：{targetIdentity?.isSuperAdmin ? (targetIdentity.userStatus === 1 && targetIdentity.identityStatus === 1 && targetIdentity.activatedAt ? '启用' : '停用') : '未选择'}</div>{governanceMutationError && <p role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{errorMessage(governanceMutationError, '治理操作失败，当前选择已保留。')}</p>}<div className="flex flex-wrap gap-2"><ConfirmAction title="确认重发激活" summary={governanceSummary} loading={resendMutation.isPending} disabled={!targetIdentity || !targetIdentity.isSuperAdmin || targetIdentity.userStatus !== 1 || targetIdentity.identityStatus !== 1 || Boolean(targetIdentity.activatedAt)} onConfirm={() => resendMutation.mutateAsync()}><RefreshCw className="h-4 w-4" />重发激活</ConfirmAction><ConfirmAction title="确认替换超管" summary={`${governanceSummary}；当前 ${currentIdentity?.name || '未选择'} → 新 ${replacementIdentity?.name || '未选择'}`} loading={replaceMutation.isPending} disabled={!currentIdentity?.isSuperAdmin || currentIdentity.userStatus !== 1 || currentIdentity.identityStatus !== 1 || !currentIdentity.activatedAt || !replacementIdentity} onConfirm={() => replaceMutation.mutateAsync()}><ShieldCheck className="h-4 w-4" />替换超管</ConfirmAction><ConfirmAction title="确认停用超管" summary={governanceSummary} variant="danger" loading={statusMutation.isPending} disabled={!targetIdentity?.isSuperAdmin || targetIdentity.userStatus !== 1 || targetIdentity.identityStatus !== 1 || !targetIdentity.activatedAt} onConfirm={() => statusMutation.mutateAsync(false)}><UserX className="h-4 w-4" />停用超管</ConfirmAction><ConfirmAction title="确认恢复超管" summary={governanceSummary} variant="secondary" loading={statusMutation.isPending} disabled={!targetIdentity?.isSuperAdmin || targetIdentity.userStatus !== 2 || targetIdentity.identityStatus !== 2 || !targetIdentity.activatedAt} onConfirm={() => statusMutation.mutateAsync(true)}><UserRoundCog className="h-4 w-4" />恢复超管</ConfirmAction></div></>}</section>}
          <section className="space-y-3"><SectionHeader title="核心用量" /><div className="grid gap-3 sm:grid-cols-2">{coreMetrics.map((metric) => { const ratio = metric.limit > 0 ? metric.current / metric.limit : 0; return <div key={metric.metric} className="rounded-md border border-zinc-200 p-3"><div className="flex items-center justify-between gap-3 text-sm"><span>{metric.label}</span><strong>{metric.current} / {metric.limit}</strong></div><div className="mt-3"><ProgressBar value={ratio * 100} tone={usageTone(ratio)} /></div></div> })}</div></section>
          <section className="space-y-3"><SectionHeader title="最近变更" /><TableShell>{(detailQuery.data?.operations || []).length === 0 ? <EmptyState icon={<Building2 className="h-5 w-5" />} title="暂无变更记录" description="租户变更会自动记录。" /> : <table><thead><tr><th>目标</th><th>说明</th><th>时间</th></tr></thead><tbody>{(detailQuery.data?.operations || []).slice(0, 8).map((item) => <tr key={item.id}><td>{item.targetName || item.targetType}</td><td>{item.remark || item.action}</td><td>{formatDate(item.createdAt)}</td></tr>)}</tbody></table>}</TableShell></section>
        </div>}
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
