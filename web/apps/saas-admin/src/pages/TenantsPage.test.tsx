// @vitest-environment jsdom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
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
  }
})

vi.mock('@/lib/api', () => mocks)

import TenantsPage from './TenantsPage'
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

describe('SaaS 客户租户治理页面', () => {
  let container: HTMLDivElement
  let root: Root
  let client: QueryClient

  beforeEach(() => {
    document.body.innerHTML = ''
    container = document.createElement('div')
    document.body.appendChild(container)
    client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mocks.apiRequest.mockReset()
    mocks.hasPermission.mockReturnValue(true)
    let governanceVersion = 1
    mocks.apiRequest.mockImplementation(async (path: string, init?: RequestInit) => {
      if (path.startsWith('/dashboard/saasAdmin/overview')) return { tenants: [tenant], summary: {}, access: {}, canPlatformScope: true, generatedAt: '', platformAdminTenantId: 0, scope: 'platform', tenantPopulation: 1 }
      if (path === '/dashboard/saasAdmin/packages') return { packages: [plan] }
      if (path.startsWith('/dashboard/saasAdmin/tenant?')) return { tenant, metrics: [], operations: [], platformAdminTenantId: 0, summary: {}, tenantId: 41 }
      if (path === '/dashboard/saasAdmin/tenantAIProvider?tenantId=41') return { configured: true, provider: { tenantId: 41, providerCode: 'deepseek', baseUrl: 'https://api.deepseek.com', model: 'deepseek-chat', apiKeyConfigured: true, apiKeyHint: '••••cafe', credentialProtection: 'ready', effectiveAt: '2026-08-25T00:00:00Z', expiresAt: '2026-09-25T00:00:00Z', status: 'active', version: 3, updatedAt: '2026-08-25T01:00:00Z' } }
      if (path === '/dashboard/saasAdmin/tenantAIProvider' && init?.method === 'PUT') return { provider: { tenantId: 41, providerCode: 'deepseek', baseUrl: 'https://api.deepseek.com', model: 'deepseek-chat', apiKeyConfigured: true, apiKeyHint: '••••ture', credentialProtection: 'ready', effectiveAt: '2026-08-25T00:00:00Z', expiresAt: '2026-09-25T00:00:00Z', status: 'active', version: 4, updatedAt: '2026-08-25T02:00:00Z' } }
      if (path === '/dashboard/saasAdmin/tenants/41/dashboard-admins') return { tenantId: 41, bindingVersion: governanceVersion, identities: [{ id: 900, name: '待激活超管', loginIdentifier: '13800000002', userStatus: 1, identityStatus: 1, activatedAt: '', isSuperAdmin: true }, { id: 902, name: '已停用超管', loginIdentifier: '13800000004', userStatus: 2, identityStatus: 2, activatedAt: '2026-08-10T00:00:00Z', isSuperAdmin: true }, { id: 901, name: '替换候选', loginIdentifier: '13800000003', userStatus: 1, identityStatus: 1, activatedAt: '2026-08-10T00:00:00Z', isSuperAdmin: false }] }
      if (path === '/dashboard/saasAdmin/tenants/provision') return { tenantId: 42, dashboardUserId: 900, bindingCorpId: 901, activationToken: 'opaque-activation-value', idempotent: false }
      if (path === '/dashboard/saasAdmin/approvalRequest') return { approval: { id: 101, status: 'pending' }, idempotent: false }
      if (path.includes('/activation/resend')) { governanceVersion = 2; return { tenantId: 41, dashboardUserId: 900, version: 2, activationToken: 'opaque-resend-value', idempotent: false } }
      if (path.includes('/super-admin/replace') || path.includes('/super-admin/status')) return { tenantId: 41, dashboardUserId: 900, version: 2, idempotent: false }
      throw new Error(`unexpected request ${path} ${JSON.stringify(init)}`)
    })
    root = createRoot(container)
    act(() => root.render(<QueryClientProvider client={client}><TenantsPage profile={profile} approvalMode={approvalMode} navigate={() => undefined} /></QueryClientProvider>))
  })

  afterEach(() => {
    act(() => root.unmount())
  })

  it('开户使用套餐 id、版本和完整额度快照，不接收密码，并在首次成功显示激活令牌', async () => {
    await settle()
    clickButton('开通客户')
    setValue('客户公司名称', '新客户')
    setValue('超级管理员', '管理员')
    setValue('11 位手机号', '13800000001')
    setValue('销售套餐', '11')
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
    expect(Object.keys(payload.limits as Record<string, unknown>)).toHaveLength(26)
    expect(payload).not.toHaveProperty('password')
    expect(document.body.textContent).toContain('一次性激活令牌')
    clickButton('我已记录并关闭')
    expect(document.body.textContent).not.toContain('opaque-activation-value')
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
    expect(document.body.textContent).toContain('停用超管')
    expect(document.body.textContent).toContain('恢复超管')
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
    expect(document.body.textContent).toContain('VERSION_CONFLICT')
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
    setValue('到期日期', '2026-09-11')
    clickButton('提交开户')
    clickButton('确认开户')
    await settle()
    expect(mocks.apiRequest.mock.calls.some(([path]) => path === '/dashboard/saasAdmin/approvalRequest')).toBe(true)
    expect(mocks.apiRequest.mock.calls.some(([path]) => path === '/dashboard/saasAdmin/tenants/provision')).toBe(false)
    const request = mocks.apiRequest.mock.calls.find(([path]) => path === '/dashboard/saasAdmin/approvalRequest')
    const body = JSON.parse(String(request?.[1]?.body)) as { actionType: string; payload: Record<string, unknown> }
    expect(body.actionType).toBe('dashboard.tenant.provision')
    expect(body.payload).not.toHaveProperty('password')
    expect(document.body.textContent).not.toContain('opaque-activation-value')
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

  it('API Key 在失败和 Escape 关闭后清空，并支持 390px 响应式表单', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
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

    setValue('API Key', 'fixture-discard-on-failure')
    mocks.apiRequest.mockImplementationOnce(async () => { throw new mocks.ApiError('version conflict', 409, 'VERSION_CONFLICT') })
    clickButton('保存 AI 配置')
    await settle()
    expect((document.querySelector('input[placeholder="留空则保留现有密钥"]') as HTMLInputElement).value).toBe('')
    expect(document.body.textContent).toContain('VERSION_CONFLICT')
  })

  it('切换厂商使用受控预设，拒绝倒置的有效期且不发送请求', async () => {
    await settle()
    clickButton('详情')
    await settle()
    await settle()
    clickButton('配置 AI 模型')
    setValue('Provider 厂商', 'openai')
    expect((document.querySelector('input[placeholder="https://api.example.com/v1"]') as HTMLInputElement).value).toBe('https://api.openai.com/v1')
    expect((document.querySelector('input[placeholder="模型标识"]') as HTMLInputElement).value).toBe('gpt-4.1-mini')
    setValue('API Key', 'fixture-invalid-window')
    setValue('生效时间', '2026-09-25T00:00')
    setValue('失效时间', '2026-08-25T00:00')
    const before = mocks.apiRequest.mock.calls.filter(([path]) => path === '/dashboard/saasAdmin/tenantAIProvider').length
    clickButton('保存 AI 配置')
    await settle()
    const after = mocks.apiRequest.mock.calls.filter(([path]) => path === '/dashboard/saasAdmin/tenantAIProvider').length
    expect(after).toBe(before)
    expect(document.body.textContent).toContain('有效结束时间必须晚于生效时间')
    expect((document.querySelector('input[placeholder="留空则保留现有密钥"]') as HTMLInputElement).value).toBe('')
  })

  it('只有 integrations.read 才读取配置，只有 integrations.manage 才显示编辑入口', async () => {
    act(() => root.unmount())
    client.clear()
    root = createRoot(container)
    const readOnlyProfile = { ...profile, permissions: ['platform.tenants.manage', 'platform.integrations.read'] }
    mocks.hasPermission.mockImplementation((permissions: string[], permission: string) => permissions.includes(permission))
    act(() => root.render(<QueryClientProvider client={client}><TenantsPage profile={readOnlyProfile} approvalMode={approvalMode} navigate={() => undefined} /></QueryClientProvider>))
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

async function settle() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}
