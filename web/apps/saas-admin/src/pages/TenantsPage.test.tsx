// @vitest-environment jsdom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { getByRole } from '@testing-library/dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => {
  class MockApiError extends Error {
    status: number
    httpCode: number
    machineCode: string

    constructor(message: string, status: number, machineCode: string, httpCode = status) {
      super(message)
      this.status = status
      this.httpCode = httpCode
      this.machineCode = machineCode
    }
  }
  const apiRequest = vi.fn()
  const integrationPath = (tenantId: number, suffix = '') => `/dashboard/saasAdmin/tenants/${tenantId}/wecom-integration${suffix}`
  const executeGoverned = vi.fn(async (options: { approvalMode?: { required: boolean; policies: Array<{ actionType: string; enabled: boolean; expiryHours: number }> }; actionType: string; payload: unknown; approvalPayload?: unknown; approvalIdempotencyKey?: string; reason: string; directPath: string; directHeaders?: HeadersInit }) => {
    const policy = options.approvalMode?.policies.find((item) => item.actionType === options.actionType)
    if (options.approvalMode?.required && policy?.enabled) {
      const data = await apiRequest('/dashboard/saasAdmin/approvalRequest', {
        method: 'POST',
        body: JSON.stringify({ actionType: options.actionType, payload: options.approvalPayload ?? options.payload, reason: options.reason, idempotencyKey: options.approvalIdempotencyKey || 'approval-test', expiresInHours: policy.expiryHours }),
      })
      return { approvalRequested: true, data }
    }
    const data = await apiRequest(options.directPath, { method: 'POST', body: JSON.stringify(options.payload), headers: options.directHeaders })
    return { approvalRequested: false, data }
  })
  return {
    ApiError: MockApiError,
    apiRequest,
    executeGoverned,
    hasPermission: vi.fn((_permissions: string[], _permission: string) => true),
    jsonRequest: vi.fn((method: string, payload?: unknown) => payload === undefined ? { method } : { method, body: JSON.stringify(payload) }),
    activationDeliveryURL: vi.fn((path: string, origin = 'http://localhost') => new URL(path, origin).toString()),
    fetchWeComIntegration: vi.fn((tenantId: number) => apiRequest(integrationPath(tenantId))),
    fetchWeComIntegrationAudits: vi.fn((tenantId: number) => apiRequest(integrationPath(tenantId, '/audits'))),
    saveDelegatedWeComIntegration: vi.fn((tenantId: number, payload: unknown) => apiRequest(integrationPath(tenantId), { method: 'PUT', body: JSON.stringify(payload) })),
  }
})

vi.mock('@/lib/api', () => mocks)

import TenantsPage, { tenantAIProviderState } from './TenantsPage'
import type { WeComIntegrationRecord } from '@/lib/api'
import type { AccessProfile, ApprovalPoliciesData } from '@/lib/types'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const limitKeys = [
  'maxCorps', 'maxUsers', 'maxContacts', 'maxRooms', 'maxAgents', 'channelCodes', 'shopCodes', 'radars', 'lotteries',
  'roomInfinitePulls', 'roomFissions', 'roomClockIns', 'roomQualities', 'roomCalendars', 'roomReminds', 'contactSops',
  'roomSops', 'sensitiveWords', 'storageMb', 'contactMessageBatches', 'roomMessageBatches', 'roomTagPulls',
  'workRoomAutoPulls', 'workFissions', 'officialAccounts', 'asyncExecutions',
]

const plan = {
  id: 11,
  code: 'pro',
  name: '专业版',
  description: '',
  status: 1,
  version: 3,
  limits: Object.fromEntries(limitKeys.map((key) => [key, 5])),
}

const tenant = {
  tenantId: 41,
  tenantName: '测试客户',
  tenantStatus: 1,
  packageCode: 'pro',
  packageName: '专业版',
  packageStatus: 1,
  packageVersion: 3,
  expiresAt: '2026-09-11 23:59:59',
  expired: false,
  expiringSoon: false,
  openAlertCount: 0,
  maxUsageMetric: 'users',
  maxUsageCurrent: 1,
  maxUsageLimit: 5,
  maxUsageRatio: 0.2,
  maxUsageLabel: '账号',
}

const profile: AccessProfile = { isPlatformSuperAdmin: false, permissions: ['platform.tenants.manage', 'platform.integrations.read', 'platform.integrations.manage'], phone: '13800000000', roles: [], tenantId: 0, userId: 700, userName: '平台管理员', version: 1 }
const approvalMode: ApprovalPoliciesData = { required: false, policies: [] }

const selfBuiltCurrent: WeComIntegrationRecord = {
  id: 'integration-current', mode: 'self_built', slot: 'current', status: 'active', verifiedWxCorpId: 'ww-fixture-corp', agentId: '1000002', providerAppId: '', credentialConfigured: true, credentialHint: '••••self', scope: ['archive.read', 'contacts.read'], scopeDigest: 'digest-current', missingCapabilities: [], generation: 8, version: 8, verificationLevel: 'local_contract', verifiedAt: '2026-08-27T00:00:00Z', lastErrorCode: '', updatedAt: '2026-08-27T00:00:00Z',
}

const tenantB = {
  ...tenant,
  tenantId: 52,
  tenantName: '第二测试客户',
  packageCode: 'basic',
  packageName: '基础版',
}

const delegatedCandidate: WeComIntegrationRecord = {
  id: 'integration-candidate', mode: 'third_party_delegated', slot: 'candidate', status: 'active', verifiedWxCorpId: 'ww-fixture-corp', agentId: '', providerAppId: 'provider-fixture', credentialConfigured: true, credentialHint: '••••code', scope: ['archive.read'], scopeDigest: 'digest-candidate', missingCapabilities: [], generation: 8, version: 9, verificationLevel: 'local_contract', verifiedAt: '2026-08-27T01:00:00Z', lastErrorCode: '', updatedAt: '2026-08-27T01:00:00Z',
}

describe('SaaS 客户租户治理页面', () => {
  let container: HTMLDivElement
  let root: Root
  let client: QueryClient
  let tenantProviderVersion: number
  let integrationView: { tenantId: number; corpId: number; current: WeComIntegrationRecord | null; candidate: WeComIntegrationRecord | null }
  let integrationViewB: { tenantId: number; corpId: number; current: WeComIntegrationRecord | null; candidate: WeComIntegrationRecord | null }
  let activationMutationObserverProbe: ReturnType<typeof vi.fn>

  beforeEach(() => {
    document.body.innerHTML = ''
    container = document.createElement('div')
    document.body.appendChild(container)
    client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mocks.apiRequest.mockReset()
    mocks.hasPermission.mockReturnValue(true)
    vi.stubGlobal('navigator', { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
    let governanceVersion = 1
    tenantProviderVersion = 3
    activationMutationObserverProbe = vi.fn()
		integrationView = { tenantId: 41, corpId: 501, current: { ...selfBuiltCurrent }, candidate: null }
    integrationViewB = { tenantId: 52, corpId: 502, current: { ...delegatedCandidate, id: 'integration-b-current', slot: 'current', generation: 20, version: 20 }, candidate: null }
    mocks.apiRequest.mockImplementation(async (path: string, init?: RequestInit) => {
      if (path.startsWith('/dashboard/saasAdmin/overview')) return { tenants: [tenant, tenantB], summary: {}, access: {}, canPlatformScope: true, generatedAt: '', platformAdminTenantId: 0, scope: 'platform', tenantPopulation: 2 }
      if (path === '/dashboard/saasAdmin/packages') return { packages: [plan] }
      if (path.startsWith('/dashboard/saasAdmin/tenant?')) {
        const requestedTenant = path.includes('tenantId=52') ? tenantB : tenant
        return { tenant: requestedTenant, metrics: [], operations: [], platformAdminTenantId: 0, summary: {}, tenantId: requestedTenant.tenantId }
      }
      if (path === '/dashboard/saasAdmin/tenantAIProvider?tenantId=41') return { configured: true, provider: { tenantId: 41, providerCode: 'deepseek', baseUrl: 'https://api.deepseek.com', model: 'deepseek-chat', apiKeyConfigured: true, apiKeyHint: '••••cafe', credentialProtection: 'usable', effectiveAt: '2026-08-25T00:00:00Z', expiresAt: '2026-09-25T00:00:00Z', status: 'active', version: tenantProviderVersion, updatedAt: '2026-08-25T01:00:00Z' } }
      if (path === '/dashboard/saasAdmin/tenantAIProvider' && init?.method === 'PUT') { tenantProviderVersion += 1; return { provider: { tenantId: 41, providerCode: 'deepseek', baseUrl: 'https://api.deepseek.com', model: 'deepseek-chat', apiKeyConfigured: true, apiKeyHint: '••••ture', credentialProtection: 'usable', effectiveAt: '2026-08-25T00:00:00Z', expiresAt: '2026-09-25T00:00:00Z', status: 'active', version: tenantProviderVersion, updatedAt: '2026-08-25T02:00:00Z' } } }
      if (path === '/dashboard/saasAdmin/tenants/41/dashboard-admins') return { tenantId: 41, bindingVersion: governanceVersion, identities: [{ id: 900, name: '待激活超管', loginIdentifier: '13800000002', userStatus: 1, identityStatus: 1, activatedAt: '', isSuperAdmin: true, availableActions: ['resend_activation'], blockedReasons: {} }, { id: 902, name: '已停用超管', loginIdentifier: '13800000004', userStatus: 2, identityStatus: 2, activatedAt: '2026-08-10T00:00:00Z', isSuperAdmin: true, availableActions: ['restore'], blockedReasons: {} }, { id: 901, name: '替换候选', loginIdentifier: '13800000003', userStatus: 1, identityStatus: 1, activatedAt: '2026-08-10T00:00:00Z', isSuperAdmin: false, availableActions: ['replacement_candidate'], blockedReasons: {} }, { id: 903, name: '当前超管', loginIdentifier: '13800000005', userStatus: 1, identityStatus: 1, activatedAt: '2026-08-10T00:00:00Z', isSuperAdmin: true, availableActions: ['replace_current'], blockedReasons: { disable: 'LAST_SUPER_ADMIN' } }] }
      if (path === '/dashboard/saasAdmin/tenants/52/dashboard-admins') return { tenantId: 52, bindingVersion: 4, identities: [{ id: 950, name: '第二客户超管', loginIdentifier: '13900000005', userStatus: 1, identityStatus: 1, activatedAt: '2026-08-20T00:00:00Z', isSuperAdmin: true, availableActions: ['replace_current'], blockedReasons: { disable: 'LAST_SUPER_ADMIN' } }] }
      if (path === '/dashboard/saasAdmin/tenants/41/wecom-integration' && !init?.method) return integrationView
      if (path === '/dashboard/saasAdmin/tenants/41/wecom-integration' && init?.method === 'PUT') {
        const payload = JSON.parse(String(init.body))
        integrationView = { ...integrationView, current: { ...(integrationView.current || delegatedCandidate), id: integrationView.current?.id || 'delegated-current', slot: 'current', mode: 'third_party_delegated', providerAppId: payload.providerAppId, scope: payload.scope, version: Number(payload.version) + 1, status: 'pending_verification', credentialConfigured: true, credentialHint: '••••saved' }, candidate: null }
        return integrationView.current
      }
      if (path === '/dashboard/saasAdmin/tenants/41/wecom-integration/audits') return [{ id: 71, action: 'wecom.integration.switch', targetId: 'integration-current', before: '{}', after: '{}', actorUserId: 700, createdAt: '2026-08-27T02:00:00Z' }]
      if (path === '/dashboard/saasAdmin/tenantAIProvider?tenantId=52') return { configured: false, provider: { tenantId: 52, providerCode: 'custom', baseUrl: '', model: '', apiKeyConfigured: false, apiKeyHint: '', credentialProtection: 'usable', effectiveAt: '', expiresAt: '', status: 'disabled', version: 0, updatedAt: '' } }
      if (path === '/dashboard/saasAdmin/tenants/52/wecom-integration' && !init?.method) return integrationViewB
      if (path === '/dashboard/saasAdmin/tenants/52/wecom-integration/audits') return []
      if (path === '/dashboard/saasAdmin/tenants/provision') return { tenantId: 42, dashboardUserId: 900, bindingCorpId: 901, activationToken: 'opaque-activation-value', activationPath: '/activate#token=opaque-activation-value', activationExpiresAt: '2026-08-28T00:00:00Z', idempotent: false }
      if (path === '/dashboard/saasAdmin/approvalRequest') return { approval: { id: 101, status: 'pending' }, idempotent: false }
      if (path.includes('/activation/resend')) { governanceVersion = 2; return { tenantId: 41, dashboardUserId: 900, version: 2, activationToken: 'opaque-resend-value', activationPath: '/activate#token=opaque-resend-value', activationExpiresAt: '2026-08-28T01:00:00Z', idempotent: false } }
      if (path.includes('/super-admin/replace') || path.includes('/super-admin/status')) return { tenantId: 41, dashboardUserId: 900, version: 2, idempotent: false }
      throw new Error(`unexpected request ${path} ${JSON.stringify(init)}`)
    })
    root = createRoot(container)
    act(() => root.render(<QueryClientProvider client={client}><TenantsPage profile={profile} approvalMode={approvalMode} navigate={() => undefined} activationMutationObserverProbe={activationMutationObserverProbe} /></QueryClientProvider>))
  })

  afterEach(() => {
    act(() => root.unmount())
  })

  it('开户使用套餐 id、版本和完整额度快照，不接收密码，并只交付完整 fragment 激活入口', async () => {
    await settle()
    clickButton('开通客户')
    setValue('客户公司名称', '新客户')
    setValue('超级管理员', '管理员')
    setValue('11 位手机号', '13800000001')
    setValue('销售套餐', '11')
    setValue('开户企微对接模式', 'self_built')
    setValue('到期日期', '2026-09-11')
    clickButton('提交开户')
    expect(document.body.textContent).toContain('确认开户')
    clickButton('确认开户')
    await settle()

    const provision = mocks.apiRequest.mock.calls.find(([path]) => path === '/dashboard/saasAdmin/tenants/provision')
    expect(provision).toBeTruthy()
    const payload = JSON.parse(String(provision?.[1]?.body)) as Record<string, unknown>
    expect(plan.id).toBe(11)
    expect(payload.packageId).toBe(plan.id)
    expect(payload.expectedVersion).toBe(3)
    expect(payload.wecomIntegrationMode).toBe('self_built')
    expect(Object.keys(payload.limits as Record<string, unknown>)).toHaveLength(26)
    expect(payload).not.toHaveProperty('password')
    expect(document.body.textContent).toContain('一次性激活入口')
    expect(document.body.textContent).toContain('http://localhost/activate#token=opaque-activation-value')
    expect(document.body.textContent).toContain('接收租户：新客户（租户 42）')
    expect(document.body.textContent).toContain('接收账号：管理员（138****0001，用户 900）')
    expect(document.body.textContent).not.toContain('邮件已发送')
    clickButton('复制激活入口')
    await settle()
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('http://localhost/activate#token=opaque-activation-value')
    clickButton('我已记录并关闭')
    await settle()
    expect(document.body.textContent).not.toContain('opaque-activation-value')
    expect(JSON.stringify(client.getMutationCache().getAll().map((mutation) => mutation.state.data))).not.toContain('opaque-activation-value')
    const observerState = activationMutationObserverProbe.mock.calls.at(-1)?.[0] as { create: { status: string; data?: unknown } } | undefined
    expect(observerState?.create.status).toBe('idle')
    expect(JSON.stringify(observerState?.create.data) || '').not.toContain('opaque-activation-value')
  })

  it('A 租户迟到的重发激活结果在切换 B 后不会打开错误租户交付弹窗', async () => {
    const pending = deferred<{ tenantId: number; dashboardUserId: number; version: number; activationPath: string; activationExpiresAt: string; idempotent: boolean }>()
    await settle()
    clickTenantDetails('测试客户')
    await settle()
    await settle()
    const api = mocks.apiRequest.getMockImplementation()
    mocks.apiRequest.mockImplementation((path: string, init?: RequestInit) => path === '/dashboard/saasAdmin/tenants/41/activation/resend' ? pending.promise : api?.(path, init))
    clickButton('重发激活')
    clickButton('确认执行')
    clickTenantDetails('第二测试客户')
    await settle()
    pending.resolve({ tenantId: 41, dashboardUserId: 900, version: 2, activationPath: '/activate#token=late-a-token', activationExpiresAt: '2026-08-28T01:00:00Z', idempotent: false })
    await settle()
    await settle()
    expect(document.body.textContent).toContain('第二测试客户')
    expect(document.body.textContent).not.toContain('late-a-token')
    expect(document.body.textContent).not.toContain('一次性激活入口')
    expect(JSON.stringify(client.getMutationCache().getAll().map((mutation) => mutation.state.data))).not.toContain('late-a-token')
    const observerState = activationMutationObserverProbe.mock.calls.at(-1)?.[0] as { resend: { status: string; data?: unknown } } | undefined
    expect(observerState?.resend.status).toBe('idle')
    expect(JSON.stringify(observerState?.resend.data) || '').not.toContain('late-a-token')
  })

  it.each([
    ['缺失激活路径', { tenantId: 41, dashboardUserId: 900, version: 2, activationToken: 'missing-path-token', activationPath: '', activationExpiresAt: '', idempotent: true }],
    ['租户不匹配', { tenantId: 52, dashboardUserId: 900, version: 2, activationToken: 'mismatch-token', activationPath: '/activate#token=mismatch-token', activationExpiresAt: '2026-08-28T01:00:00Z', idempotent: false }],
  ])('%s 的重发响应不会留在 observer 或 mutation cache', async (_caseName, result) => {
    await settle()
    clickTenantDetails('测试客户')
    await settle()
    await settle()
    const api = mocks.apiRequest.getMockImplementation()
    mocks.apiRequest.mockImplementation((path: string, init?: RequestInit) => path === '/dashboard/saasAdmin/tenants/41/activation/resend' ? Promise.resolve(result) : api?.(path, init))
    clickButton('重发激活')
    clickButton('确认执行')
    await settle()
    await settle()
    expect(document.body.textContent).not.toContain(result.activationToken)
    expect(document.body.textContent).not.toContain('一次性激活入口')
    expect(JSON.stringify(client.getMutationCache().getAll().map((mutation) => mutation.state.data))).not.toContain(result.activationToken)
    const observerState = activationMutationObserverProbe.mock.calls.at(-1)?.[0] as { resend: { status: string; data?: unknown } } | undefined
    expect(observerState?.resend.status).toBe('idle')
    expect(JSON.stringify(observerState?.resend.data) || '').not.toContain(result.activationToken)
  })

  it('详情提供重发、替换、停用、恢复四个确认动作且请求不携带租户或 actor', async () => {
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    expect(document.querySelector('input[placeholder="从开户结果或受控目录填写"]')).toBeNull()
    expect(document.body.textContent).toContain('治理绑定版本')
    expect(document.body.textContent).toContain('重发激活')
    expect(document.body.textContent).toContain('替换超管')
    expect(document.body.textContent).toContain('至少保留一名可登录超级管理员')
    clickButton('重发激活')
    clickButton('确认执行')
    await settle()
    const resend = mocks.apiRequest.mock.calls.find(([path]) => String(path).includes('/activation/resend'))
    expect(resend).toBeTruthy()
    const payload = JSON.parse(String(resend?.[1]?.body)) as Record<string, unknown>
    expect(payload).toEqual({ targetUserId: 900, expectedVersion: 1 })
    expect(payload).not.toHaveProperty('tenantId')
    expect(payload).not.toHaveProperty('actorId')
    expect(document.body.textContent).toContain('v2')
    setValue('重发/停用/恢复对象', '902')
    expect(document.body.textContent).toContain('恢复超管')
    clickButton('恢复超管')
    clickButton('确认执行')
    await settle()
    const restore = mocks.apiRequest.mock.calls.find(([path, init]) => String(path).includes('/super-admin/status') && JSON.parse(String(init?.body)).enabled === true)
    expect(restore).toBeTruthy()
    expect(JSON.parse(String(restore?.[1]?.body))).toMatchObject({ targetUserId: 902, enabled: true, expectedVersion: 2 })
  })

  it('409 不清空服务端治理选择，确认前不发 mutation', async () => {
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    const target = [...document.querySelectorAll('select')].find((item) => item.closest('label')?.textContent?.includes('重发/停用/恢复对象')) as HTMLSelectElement
    if (!target) throw new Error('governance target select not found')
    expect(target.value).toBe('900')
    mocks.apiRequest.mockImplementationOnce(async (path: string) => {
      if (path.includes('/activation/resend')) throw new mocks.ApiError('version conflict', 409, 'VERSION_CONFLICT')
      throw new Error(`unexpected retry ${path}`)
    })
    clickButton('重发激活')
    expect(mocks.apiRequest.mock.calls.filter(([path]) => String(path).includes('/activation/resend'))).toHaveLength(0)
    clickButton('确认执行')
    await settle()
    expect(target.value).toBe('900')
    expect(document.body.textContent).toContain('请刷新治理列表后重试')
    expect(document.body.textContent).not.toContain('VERSION_CONFLICT')
  })

  it('390px 宽度仍保持表格横向滚动、治理操作换行和对话框可用宽度', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    await settle()
    const table = document.querySelector('table')
    expect(table?.parentElement?.className).toContain('overflow-x-auto')

    clickButton('详情')
    await settle()
    await settle()
    const governance = [...document.querySelectorAll('section')].find((section) => section.textContent?.includes('Dashboard 超级管理员治理'))
    expect(governance?.querySelector('.flex.flex-wrap')).not.toBeNull()
    clickButton('重发激活')
    expect(document.querySelector('[role="dialog"]')?.className).toContain('w-[calc(100vw-2rem)]')
  })

  it('审批策略开启时开户只提交规范化审批申请，不直写且不显示令牌', async () => {
    act(() => root.unmount())
    root = createRoot(container)
    const requiredApprovalMode: ApprovalPoliciesData = {
      required: true,
      policies: [{ actionType: 'dashboard.tenant.provision', enabled: true, expiryHours: 12, name: '开户', requiredApprovals: 2 }],
    }
    act(() => root.render(<QueryClientProvider client={client}><TenantsPage profile={profile} approvalMode={requiredApprovalMode} navigate={() => undefined} /></QueryClientProvider>))
    await settle()
    clickButton('开通客户')
    setValue('客户公司名称', '审批客户')
    setValue('超级管理员', '审批管理员')
    setValue('11 位手机号', '13800000001')
    setValue('销售套餐', '11')
    setValue('开户企微对接模式', 'third_party_delegated')
    setValue('到期日期', '2026-09-11')
    clickButton('提交开户')
    clickButton('确认开户')
    await settle()
    expect(mocks.apiRequest.mock.calls.some(([path]) => path === '/dashboard/saasAdmin/approvalRequest')).toBe(true)
    expect(mocks.apiRequest.mock.calls.some(([path]) => path === '/dashboard/saasAdmin/tenants/provision')).toBe(false)
    const request = mocks.apiRequest.mock.calls.find(([path]) => path === '/dashboard/saasAdmin/approvalRequest')
    const body = JSON.parse(String(request?.[1]?.body)) as { actionType: string; payload: Record<string, unknown> }
    expect(body.actionType).toBe('dashboard.tenant.provision')
    expect(body.payload.wecomIntegrationMode).toBe('third_party_delegated')
    expect(body.payload).not.toHaveProperty('password')
    expect(document.body.textContent).not.toContain('opaque-activation-value')
  })

  it('治理审批被受理后重复确认沿用同一幂等键', async () => {
    act(() => root.unmount())
    root = createRoot(container)
    const requiredApprovalMode: ApprovalPoliciesData = {
      required: true,
      policies: [{ actionType: 'dashboard.activation.resend', enabled: true, expiryHours: 12, name: '重发激活', requiredApprovals: 2 }],
    }
    act(() => root.render(<QueryClientProvider client={client}><TenantsPage profile={profile} approvalMode={requiredApprovalMode} navigate={() => undefined} /></QueryClientProvider>))
    await settle()
    clickButton('详情')
    await settle()
    await settle()

    clickButton('重发激活')
    clickButton('确认执行')
    await settle()
    clickButton('重发激活')
    clickButton('确认执行')
    await settle()

    const approvals = mocks.apiRequest.mock.calls.filter(([path]) => path === '/dashboard/saasAdmin/approvalRequest')
    expect(approvals).toHaveLength(2)
    const first = JSON.parse(String(approvals[0]?.[1]?.body)) as { idempotencyKey: string }
    const second = JSON.parse(String(approvals[1]?.[1]?.body)) as { idempotencyKey: string }
    expect(first.idempotencyKey).not.toBe('')
    expect(second.idempotencyKey).toBe(first.idempotencyKey)
  })

  it('按租户读取并保存脱敏 AI Provider，Key 不进入 Query cache 且成功后清空', async () => {
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    expect(document.body.textContent).toContain('租户 AI 分析模型')
    expect(document.body.textContent).toContain('••••cafe')
    clickButton('配置 AI 模型')
    setValue('API Key', 'fixture-new-secret')
    clickButton('保存 AI 配置')
    await settle()

    const save = mocks.apiRequest.mock.calls.find(([path, init]) => path === '/dashboard/saasAdmin/tenantAIProvider' && init?.method === 'PUT')
    expect(save).toBeTruthy()
    expect(JSON.parse(String(save?.[1]?.body))).toMatchObject({ tenantId: 41, providerCode: 'deepseek', model: 'deepseek-chat', apiKey: 'fixture-new-secret', status: 'active', version: 3 })
    expect(JSON.stringify(client.getQueryCache().getAll().map((query) => query.state.data))).not.toContain('fixture-new-secret')
    expect((document.querySelector('input[placeholder="留空则保留现有密钥"]') as HTMLInputElement | null)?.value || '').toBe('')
  })

  it('API Key 保存失败后保留并支持重试，Escape 和关闭后清空，并支持 390px 响应式表单', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 520 })
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置 AI 模型')
    setValue('API Key', 'fixture-discard-on-escape')
    act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })))
    clickButton('配置 AI 模型')
    expect((document.querySelector('input[placeholder="留空则保留现有密钥"]') as HTMLInputElement).value).toBe('')
    expect(document.querySelector('[aria-label="租户 AI Provider 表单"]')?.className).toContain('grid-cols-1')
    expect(document.querySelector('[role="dialog"]')?.className).toContain('max-h-[calc(100vh-2rem)]')

    setValue('API Key', 'fixture-retry-secret')
    mocks.apiRequest.mockImplementationOnce(async () => { throw new mocks.ApiError('配置暂时不可用', 500, 'API_REQUEST_FAILED') })
    clickButton('保存 AI 配置')
    await settle()
    expect((document.querySelector('input[placeholder="留空则保留现有密钥"]') as HTMLInputElement).value).toBe('fixture-retry-secret')
    expect(document.body.textContent).toContain('AI 模型配置保存失败')
    expect(document.body.textContent).not.toContain('API_REQUEST_FAILED')

    clickButton('保存 AI 配置')
    await settle()
    const retry = mocks.apiRequest.mock.calls.filter(([path, init]) => path === '/dashboard/saasAdmin/tenantAIProvider' && init?.method === 'PUT').at(-1)
    expect(JSON.parse(String(retry?.[1]?.body)).apiKey).toBe('fixture-retry-secret')
    expect(document.querySelector('input[placeholder="留空则保留现有密钥"]')).toBeNull()

    clickButton('配置 AI 模型')
    setValue('API Key', 'fixture-discard-on-close')
    const closeButtons = [...document.querySelectorAll('button[aria-label="关闭"]')]
    act(() => closeButtons.at(-1)?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    clickButton('配置 AI 模型')
    expect((document.querySelector('input[placeholder="留空则保留现有密钥"]') as HTMLInputElement).value).toBe('')
  })

  it('切换厂商使用受控预设，拒绝倒置的有效期且不发送请求', async () => {
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置 AI 模型')
    setValue('API Key', 'fixture-cleared-on-provider-switch')
    setValue('Provider 厂商', 'openai')
    expect((document.querySelector('input[placeholder="留空则保留现有密钥"]') as HTMLInputElement).value).toBe('')
    expect((document.querySelector('input[placeholder="https://api.example.com/v1"]') as HTMLInputElement).value).toBe('https://api.openai.com/v1')
    expect((document.querySelector('input[placeholder="模型标识"]') as HTMLInputElement).value).toBe('')
    setValue('模型名称', 'tenant-selected-model')
    setValue('API Key', 'fixture-invalid-window')
    setValue('生效时间', '2026-09-25T00:00')
    setValue('失效时间', '2026-08-25T00:00')
    const before = mocks.apiRequest.mock.calls.filter(([path]) => path === '/dashboard/saasAdmin/tenantAIProvider').length
    clickButton('保存 AI 配置')
    await settle()
    const after = mocks.apiRequest.mock.calls.filter(([path]) => path === '/dashboard/saasAdmin/tenantAIProvider').length
    expect(after).toBe(before)
    expect(document.body.textContent).toContain('有效结束时间必须晚于生效时间')
    expect((document.querySelector('input[placeholder="留空则保留现有密钥"]') as HTMLInputElement).value).toBe('fixture-invalid-window')
  })

  it('409 后立即刷新服务端版本并允许在同一弹窗恢复', async () => {
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置 AI 模型')
    expect(document.body.textContent).toContain('租户 41')
    tenantProviderVersion = 4
    setValue('API Key', 'fixture-version-conflict')
    mocks.apiRequest.mockImplementationOnce(async () => { throw new mocks.ApiError('version conflict', 409, 'VERSION_CONFLICT') })
    clickButton('保存 AI 配置')
    await settle()
    await settle()
    const getCalls = mocks.apiRequest.mock.calls.filter(([path]) => path === '/dashboard/saasAdmin/tenantAIProvider?tenantId=41')
    expect(getCalls.length).toBeGreaterThanOrEqual(2)
    expect(document.body.textContent).toContain('页面已载入最新版本')
    expect(document.body.textContent).not.toContain('刷新治理列表')
    setValue('API Key', 'fixture-after-refresh')
    clickButton('保存 AI 配置')
    await settle()
    const successfulSave = mocks.apiRequest.mock.calls.filter(([path, init]) => path === '/dashboard/saasAdmin/tenantAIProvider' && init?.method === 'PUT').at(-1)
    expect(JSON.parse(String(successfulSave?.[1]?.body)).version).toBe(4)
  })

  it('409 后刷新失败时不误报已载入最新版本', async () => {
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置 AI 模型')
    setValue('API Key', 'fixture-version-conflict-refresh-failure')
    mocks.apiRequest
      .mockImplementationOnce(async () => { throw new mocks.ApiError('version conflict', 409, 'VERSION_CONFLICT') })
      .mockImplementationOnce(async () => { throw new Error('refresh unavailable') })
    clickButton('保存 AI 配置')
    await settle()
    await settle()
    expect(document.body.textContent).toContain('最新版本刷新失败')
    expect(document.body.textContent).not.toContain('页面已载入最新版本')
    expect((document.querySelector('input[placeholder="留空则保留现有密钥"]') as HTMLInputElement).value).toBe('fixture-version-conflict-refresh-failure')
  })

	it('治理幂等键按租户、对象、动作和版本隔离，成功后再次操作生成新键', async () => {
		await settle()
		clickButton('详情')
		await settle()
		await settle()
		clickButton('重发激活')
		clickButton('确认执行')
		await settle()
		clickButton('我已记录并关闭')
		await settle()
		clickButton('重发激活')
		clickButton('确认执行')
		await settle()
		const requests = mocks.apiRequest.mock.calls.filter(([path]) => String(path).includes('/activation/resend'))
		expect(requests).toHaveLength(2)
		const requestID = (call: (typeof requests)[number]) => String((call[1]?.headers as Record<string, string> | undefined)?.['X-Request-ID'] || '')
		expect(requestID(requests[0]!)).not.toBe('')
		expect(requestID(requests[1]!)).not.toBe(requestID(requests[0]!))
	})

  it('自建应用模式只读展示且不提供 SaaS 配置或切换入口', async () => {
    await settle()
    clickButton('详情')
    await settle()
    await settle()

    expect(document.body.textContent).toContain('企微对接模式')
    expect(document.body.textContent).toContain('当前：自建应用')
    expect(document.body.textContent).toContain('本地合同验证')
    expect(document.body.textContent).toContain('wecom.integration.switch')
    expect(document.body.textContent).toContain('Dashboard「唯一企业资料」维护')
    const integrationSection = document.querySelector('section[aria-label="企微对接模式"]')
    expect(integrationSection?.textContent).not.toContain('候选：')
    expect([...document.querySelectorAll('button')].some((button) => /切换|回滚|编辑候选/.test(button.textContent || ''))).toBe(false)
  })

  it('第三方模式只写当前授权且 payload 不包含任何自建 Secret', async () => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', status: 'unconfigured', verificationLevel: '', version: 9 }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置第三方应用')
    setValue('Provider App ID', 'provider-new')
    setValue('永久授权码', 'permanent-secret')
    clickButton('安全保存')
    await settle()
    const save = mocks.apiRequest.mock.calls.filter(([path, init]) => path === '/dashboard/saasAdmin/tenants/41/wecom-integration' && init?.method === 'PUT').at(-1)
    const payload = JSON.parse(String(save?.[1]?.body)) as Record<string, unknown>
    expect(payload).toMatchObject({ mode: 'third_party_delegated', providerAppId: 'provider-new', permanentCode: 'permanent-secret', version: 9 })
    expect(payload).not.toHaveProperty('employeeSecret')
    expect(payload).not.toHaveProperty('contactSecret')
    expect(payload).not.toHaveProperty('agentSecret')
    expect(payload).not.toHaveProperty('chatSecret')
    expect(document.body.textContent).not.toContain('permanent-secret')
    expect(JSON.stringify(client.getQueryCache().getAll().map((query) => query.state.data))).not.toContain('permanent-secret')
    expect(JSON.stringify(client.getMutationCache().getAll().map((mutation) => mutation.state.variables))).not.toContain('permanent-secret')
    expect(document.body.textContent).not.toContain('切换为候选')
  })

  it('第三方配置失败后授权码不会残留在表单、ref 对应行为或 React Query 缓存', async () => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', version: 9 }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置第三方应用')
    setValue('永久授权码', 'failed-permanent-secret')
    mocks.apiRequest.mockImplementationOnce(async () => { throw new mocks.ApiError('raw backend failure', 500, 'WECOM_SAVE_FAILED') })
    clickButton('安全保存')
    await settle()

    expect((document.querySelector('input[placeholder="永久授权码"]') as HTMLInputElement).value).toBe('')
    expect(document.body.textContent).toContain('第三方应用配置保存失败，敏感输入已清空')
    expect(document.body.textContent).not.toContain('failed-permanent-secret')
    expect(document.body.textContent).not.toContain('WECOM_SAVE_FAILED')
    expect(JSON.stringify(client.getQueryCache().getAll().map((query) => query.state.data))).not.toContain('failed-permanent-secret')
    expect(JSON.stringify(client.getMutationCache().getAll().map((mutation) => mutation.state.variables))).not.toContain('failed-permanent-secret')
  })

  it('关闭第三方配置弹窗会清空未提交授权码且不会写入 React Query 缓存', async () => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', version: 9 }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置第三方应用')
    setValue('永久授权码', 'closed-permanent-secret')
    const editor = getByRole(document.body, 'dialog', { name: '配置第三方代开发应用' })
    const close = getByRole(editor, 'button', { name: '关闭' })
    act(() => close.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    clickButton('配置第三方应用')

    expect((document.querySelector('input[placeholder="永久授权码"]') as HTMLInputElement).value).toBe('')
    expect(document.body.textContent).not.toContain('closed-permanent-secret')
    expect(JSON.stringify(client.getQueryCache().getAll().map((query) => query.state.data))).not.toContain('closed-permanent-secret')
    expect(JSON.stringify(client.getMutationCache().getAll().map((mutation) => mutation.state.variables))).not.toContain('closed-permanent-secret')
  })

  it('首次配置默认开启全部公开能力，并以两个能力 code 保存', async () => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', status: 'unconfigured', scope: [], version: 9 }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置第三方应用')

    expect(scopeCheckbox('会话内容与媒体归档').checked).toBe(true)
    expect(scopeCheckbox('通讯录与客户资料').checked).toBe(true)
    setCheckbox('通讯录与客户资料', false)
    expect(scopeCheckbox('通讯录与客户资料').checked).toBe(false)
    clickButton('全部开启')
    expect(scopeCheckbox('通讯录与客户资料').checked).toBe(true)
    expect(document.querySelector('[aria-label="第三方应用配置表单"] textarea')).toBeNull()
    expect(document.body.textContent).not.toContain('archive.read')
    setValue('永久授权码', 'first-configuration-secret')
    clickButton('安全保存')
    await settle()

    const save = latestWeComSave()
    expect(JSON.parse(String(save?.[1]?.body)).scope).toEqual(['archive.read', 'contacts.read'])
  })

  it('已有 archive.read 只勾选归档能力，并在摘要中显示中文已启用数量', async () => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', scope: ['archive.read'], version: 9 }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()

    expect(document.body.textContent).toContain('已开启 1 项')
    expect(document.body.textContent).toContain('会话内容与媒体归档')
    expect(document.body.textContent).not.toContain('archive.read')
    clickButton('配置第三方应用')
    expect(scopeCheckbox('会话内容与媒体归档').checked).toBe(true)
    expect(scopeCheckbox('通讯录与客户资料').checked).toBe(false)
  })

  it('取消全部能力后阻止保存并显示中文错误', async () => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', scope: ['archive.read'], version: 9 }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置第三方应用')
    setCheckbox('会话内容与媒体归档', false)
    clickButton('安全保存')
    await settle()

    expect(document.body.textContent).toContain('请至少选择一项能力')
    expect(latestWeComSave()).toBeUndefined()
  })

  it('保存时保留未知历史能力，且摘要不将其显示为公开能力', async () => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', scope: ['archive.read', 'future.scope'], version: 9 }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()

    expect(document.body.textContent).toContain('已开启 1 项')
    expect(document.body.textContent).toContain('会话内容与媒体归档')
    expect(document.body.textContent).not.toContain('future.scope')
    clickButton('配置第三方应用')
    setValue('永久授权码', 'keep-unknown-secret')
    clickButton('安全保存')
    await settle()

    expect(JSON.parse(String(latestWeComSave()?.[1]?.body)).scope).toEqual(['archive.read', 'future.scope'])
  })

  it('缺失能力仅显示中文已知名，并为未知能力提供通用提示', async () => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', scope: ['archive.read'], missingCapabilities: ['archive.read', 'future.scope'], version: 9 }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()

    expect(document.body.textContent).toContain('缺失能力：会话内容与媒体归档')
    expect(document.body.textContent).toContain('部分能力暂不可用，请联系管理员。')
    expect(document.body.textContent).not.toContain('archive.read')
    expect(document.body.textContent).not.toContain('future.scope')
  })

  it('仅有未知历史能力时允许不扩权轮换并精确保留最终 scope', async () => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', scope: ['future.scope'], version: 9 }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()

    expect(document.body.textContent).toContain('已开启 0 项')
    expect(document.body.textContent).not.toContain('未开启公开能力')
    expect(document.body.textContent).not.toContain('future.scope')
    clickButton('配置第三方应用')
    setValue('永久授权码', 'unknown-only-secret')
    clickButton('安全保存')
    await settle()

    expect(JSON.parse(String(latestWeComSave()?.[1]?.body)).scope).toEqual(['future.scope'])
    expect(JSON.stringify(client.getMutationCache().getAll().map((mutation) => mutation.state.variables))).not.toContain('unknown-only-secret')
  })

  it.each(['WECOM_CREDENTIAL_INVALID', 'FUTURE_BACKEND_FAILURE'])('企微摘要将技术错误码 %s 收敛为受控中文', async (lastErrorCode) => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', lastErrorCode }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()

    expect(document.body.textContent).toContain('当前配置暂不可用，请重新保存或联系管理员。')
    expect(document.body.textContent).not.toContain(lastErrorCode)
  })

  it('第三方配置版本冲突刷新当前版本、清空授权码并允许重试', async () => {
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', version: 9 }, candidate: null }
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置第三方应用')
    setValue('永久授权码', 'discard-on-conflict')
    integrationView = { ...integrationView, current: { ...delegatedCandidate, id: 'delegated-current', slot: 'current', version: 12 }, candidate: null }
    mocks.apiRequest.mockImplementationOnce(async () => { throw new mocks.ApiError('version conflict', 409, 'VERSION_CONFLICT') })
    clickButton('安全保存')
    await settle()
    await settle()
    expect(document.body.textContent).toContain('页面已刷新至最新企微配置')
    expect(document.body.textContent).not.toContain('VERSION_CONFLICT')
    expect((document.querySelector('input[placeholder="永久授权码"]') as HTMLInputElement).value).toBe('')
    setValue('永久授权码', 'retry-secret')
    clickButton('安全保存')
    await settle()
    const retry = mocks.apiRequest.mock.calls.filter(([path, init]) => path === '/dashboard/saasAdmin/tenants/41/wecom-integration' && init?.method === 'PUT').at(-1)
    expect(JSON.parse(String(retry?.[1]?.body)).version).toBe(12)
  })

  it('只有 integrations.read 才查询企微配置，read-only 只显示状态不显示变更操作', async () => {
    act(() => root.unmount())
    client.clear()
    root = createRoot(container)
    const readOnlyProfile = { ...profile, permissions: ['platform.tenants.manage', 'platform.integrations.read'] }
    mocks.hasPermission.mockImplementation((permissions: string[], permission: string) => permissions.includes(permission))
    act(() => root.render(<QueryClientProvider client={client}><TenantsPage profile={readOnlyProfile} approvalMode={approvalMode} navigate={() => undefined} /></QueryClientProvider>))
    await settle()
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    expect(mocks.apiRequest.mock.calls.some(([path]) => path === '/dashboard/saasAdmin/tenants/41/wecom-integration')).toBe(true)
    expect(document.body.textContent).toContain('当前账号只有查看权限，不能写入第三方应用配置。')
    expect([...document.querySelectorAll('button')].some((button) => button.textContent?.includes('配置第三方应用'))).toBe(false)
  })

  it('综合状态覆盖待生效、即将过期、过期、停用和凭证不可用', () => {
    const base = { tenantId: 41, providerCode: 'deepseek' as const, baseUrl: 'https://api.deepseek.com', model: 'tenant-model', apiKeyConfigured: true, apiKeyHint: 'cafe', credentialProtection: 'usable', effectiveAt: '2026-08-20T00:00:00Z', expiresAt: '2026-09-20T00:00:00Z', status: 'active' as const, version: 3, updatedAt: '2026-08-25T00:00:00Z' }
    const now = new Date('2026-08-25T00:00:00Z')
    expect(tenantAIProviderState(true, { ...base, effectiveAt: '2026-08-26T00:00:00Z' }, now).label).toBe('待生效')
    expect(tenantAIProviderState(true, { ...base, expiresAt: '2026-08-30T00:00:00Z' }, now).label).toBe('即将过期')
    expect(tenantAIProviderState(true, { ...base, expiresAt: '2026-08-24T00:00:00Z' }, now).label).toBe('已过期')
    expect(tenantAIProviderState(true, { ...base, status: 'disabled' }, now).label).toBe('已停用')
    expect(tenantAIProviderState(true, { ...base, credentialProtection: 'unavailable' }, now).label).toBe('凭证不可用')
  })

  it('只有 integrations.read 才读取配置，只有 integrations.manage 才显示编辑入口', async () => {
    act(() => root.unmount())
    client.clear()
    root = createRoot(container)
    const readOnlyProfile = { ...profile, permissions: ['platform.tenants.manage', 'platform.integrations.read'] }
    mocks.hasPermission.mockImplementation((permissions: string[], permission: string) => permissions.includes(permission))
    act(() => root.render(<QueryClientProvider client={client}><TenantsPage profile={readOnlyProfile} approvalMode={approvalMode} navigate={() => undefined} /></QueryClientProvider>))
    await settle()
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    expect(mocks.apiRequest.mock.calls.some(([path]) => path === '/dashboard/saasAdmin/tenantAIProvider?tenantId=41')).toBe(true)
    expect([...document.querySelectorAll('button')].some((button) => button.textContent?.includes('配置 AI 模型'))).toBe(false)
  })
})

function clickButton(label: string) {
  const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.replace(/\s+/g, ' ').trim().includes(label))
  if (!button) throw new Error(`button ${label} not found`)
  act(() => button.dispatchEvent(new MouseEvent('click', { bubbles: true })))
}

function clickTenantDetails(tenantName: string) {
  const row = [...document.querySelectorAll('tbody tr')].find((item) => item.textContent?.includes(tenantName))
  const button = [...(row?.querySelectorAll('button') || [])].find((item) => item.textContent?.includes('详情'))
  if (!button) throw new Error(`tenant details ${tenantName} not found`)
  act(() => button.dispatchEvent(new MouseEvent('click', { bubbles: true })))
}

function setValue(labelOrPlaceholder: string, value: string) {
  const field = [...document.querySelectorAll('input, select')].find((item) => item.getAttribute('placeholder') === labelOrPlaceholder || item.closest('label')?.textContent?.includes(labelOrPlaceholder)) as HTMLInputElement | HTMLSelectElement | undefined
  if (!field) throw new Error(`field ${labelOrPlaceholder} not found`)
  act(() => {
    const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(field), 'value')?.set
    setter?.call(field, value)
    field.dispatchEvent(new Event('input', { bubbles: true }))
    field.dispatchEvent(new Event('change', { bubbles: true }))
  })
}

function scopeCheckbox(label: string) {
  return getByRole(document.body, 'checkbox', { name: label }) as HTMLInputElement
}

function setCheckbox(label: string, checked: boolean) {
  const input = scopeCheckbox(label)
  if (input.checked === checked) return
  act(() => input.dispatchEvent(new MouseEvent('click', { bubbles: true })))
}

function latestWeComSave() {
  return mocks.apiRequest.mock.calls.filter(([path, init]) => path === '/dashboard/saasAdmin/tenants/41/wecom-integration' && init?.method === 'PUT').at(-1)
}

async function settle() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })

  return { promise, resolve, reject }
}
