// @vitest-environment jsdom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => {
  const apiRequest = vi.fn()
  return {
    apiRequest,
    executeGoverned: vi.fn(),
    hasPermission: vi.fn(() => true),
    jsonRequest: vi.fn((method: string, payload?: unknown) => payload === undefined ? { method } : { method, body: JSON.stringify(payload) }),
    toast: { success: vi.fn(), error: vi.fn() },
  }
})

vi.mock('@/lib/api', () => mocks)
vi.mock('sonner', () => ({ toast: mocks.toast }))

import TeamPage from './TeamPage'
import type { AccessProfile, ApprovalPoliciesData } from '@/lib/types'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const profile: AccessProfile = {
  isPlatformSuperAdmin: false,
  permissions: ['platform.access.manage', 'platform.approvals.read', 'platform.approvals.review', 'platform.approvals.execute'],
  phone: '13800000000', roles: [], tenantId: 0, userId: 9, userName: '复核执行人', version: 1,
}
const approvalMode: ApprovalPoliciesData = { required: true, policies: [] }

describe('SaaS 审批执行中的一次性激活令牌', () => {
  let container: HTMLDivElement
  let root: Root
  let client: QueryClient

  beforeEach(() => {
    document.body.innerHTML = ''
    container = document.createElement('div')
    document.body.appendChild(container)
    client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mocks.apiRequest.mockReset()
    mocks.toast.success.mockReset()
    mocks.toast.error.mockReset()
    mocks.apiRequest.mockImplementation(async (path: string) => {
      if (path.startsWith('/dashboard/saasAdmin/accessAssignments')) return { assignments: [{ userId: 12, userName: '平台成员', phone: '13800000001', roles: [], permissions: [], status: 1, version: 1, isSuperAdmin: false, updatedAt: '', updatedBy: 0 }] }
      if (path === '/dashboard/saasAdmin/accessRoles') return { roles: [] }
      if (path.startsWith('/dashboard/saasAdmin/approvals')) return { items: [{ id: 81, requestNo: 'APR-81', actionType: 'dashboard.activation.resend', requesterUserId: 8, requesterName: '申请人', status: 'approved', approvalCount: 2, requiredApprovals: 2, expiresAt: '2026-08-12T00:00:00Z', slaDueAt: '2026-08-11T12:00:00Z', createdAt: '2026-08-11T00:00:00Z', targetName: '待激活管理员', version: 6, policyVersion: 1, reason: '重发激活', riskLevel: 'critical', targetId: '52' }], returnedCount: 1, summary: {} }
      if (path === '/dashboard/saasAdmin/approvalExecute') return { approval: { id: 81, status: 'executed', version: 7 }, result: { activationToken: 'one-time-activation-value' } }
      throw new Error(`unexpected request ${path}`)
    })
    root = createRoot(container)
    act(() => root.render(<QueryClientProvider client={client}><TeamPage profile={profile} approvalMode={approvalMode} navigate={() => undefined} /></QueryClientProvider>))
  })

  afterEach(() => {
    act(() => root.unmount())
  })

  it('首次执行读取 nested result 的令牌，关闭后从 DOM 和状态清空且不进入 toast', async () => {
    await settle()
    clickButton('执行')
    await settle()
    expect(document.body.textContent).toContain('一次性激活令牌')
    expect(document.body.textContent).toContain('one-time-activation-value')
    expect(mocks.toast.success.mock.calls.flat().join('|')).not.toContain('one-time-activation-value')
    clickButton('我已记录并关闭')
    expect(document.body.textContent).not.toContain('one-time-activation-value')
    expect(document.body.textContent).not.toContain('一次性激活令牌')
  })

  it('recovered 执行结果没有令牌时不弹出一次性令牌对话框', async () => {
    mocks.apiRequest.mockImplementation(async (path: string) => {
      if (path.startsWith('/dashboard/saasAdmin/accessAssignments')) return { assignments: [{ userId: 12, userName: '平台成员', phone: '13800000001', roles: [], permissions: [], status: 1, version: 1, isSuperAdmin: false, updatedAt: '', updatedBy: 0 }] }
      if (path === '/dashboard/saasAdmin/accessRoles') return { roles: [] }
      if (path.startsWith('/dashboard/saasAdmin/approvals')) return { items: [{ id: 81, requestNo: 'APR-81', actionType: 'dashboard.activation.resend', requesterUserId: 8, requesterName: '申请人', status: 'approved', approvalCount: 2, requiredApprovals: 2, expiresAt: '2026-08-12T00:00:00Z', slaDueAt: '2026-08-11T12:00:00Z', createdAt: '2026-08-11T00:00:00Z', targetName: '待激活管理员', version: 6, policyVersion: 1, reason: '重发激活', riskLevel: 'critical', targetId: '52' }], returnedCount: 1, summary: {} }
      if (path === '/dashboard/saasAdmin/approvalExecute') return { approval: { id: 81, status: 'executed', version: 7 }, result: { recovered: true, effectOperationId: 99 } }
      throw new Error(`unexpected request ${path}`)
    })
    await settle()
    clickButton('执行')
    await settle()
    expect(document.body.textContent).not.toContain('一次性激活令牌')
    expect(document.body.textContent).not.toContain('one-time-activation-value')
  })
})

function clickButton(label: string) {
  const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.replace(/\s+/g, ' ').trim().includes(label))
  if (!button) throw new Error(`button ${label} not found`)
  act(() => button.dispatchEvent(new MouseEvent('click', { bubbles: true })))
}

async function settle() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}
