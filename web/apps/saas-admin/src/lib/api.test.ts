import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  ApiError,
  activationDeliveryURL,
  apiRequest,
  changeSaaSPassword,
  completeSaaSMFA,
  fetchWeComIntegration,
  fetchWeComIntegrationAudits,
  loginSaaS,
  loginURL,
  logoutSaaS,
  readStoredToken,
  rollbackWeComIntegration,
  saveWeComIntegrationCandidate,
  switchWeComIntegration,
  verifyWeComIntegrationCandidate,
} from './api'

describe('SaaS Admin token storage', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key: string) => key === 'ACCESS_TOKEN' ? 'dashboard-token' : null),
      removeItem: vi.fn(),
    })
  })

  it('does not consume the Dashboard token namespace', () => {
    expect(readStoredToken()).toBe('')
  })

  it('redirects to the SaaS login page', () => {
    vi.stubGlobal('location', { pathname: '/saas-admin', search: '', hash: '' })
    expect(loginURL()).toBe('/saas/login?redirect=%2Fsaas-admin')
  })

  it('sends the SaaS bearer to logout and does not use a Dashboard token', async () => {
    const values: Record<string, string> = {
      mochat_saas_admin_token: 'saas-token',
      mochat_saas_admin_user_id: '7',
      mochat_saas_admin_user_name: 'Platform Admin',
      mochat_saas_admin_expires_at: '9999999999',
    }
    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key: string) => values[key] || null),
      removeItem: vi.fn(),
    })
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)

    await logoutSaaS()

    expect(fetchMock).toHaveBeenCalledWith('/saas/auth/logout', expect.objectContaining({
      headers: expect.objectContaining({ Authorization: 'Bearer saas-token' }),
    }))
  })

  it('preserves stable MFA, password, and session machine codes', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const rejected = (errorCode: string, status: number, httpCode = status) => new Response(JSON.stringify({ code: httpCode, errorCode, msg: errorCode }), {
      status,
      headers: { 'Content-Type': 'application/json' },
    })

    fetchMock.mockResolvedValueOnce(rejected('MFA_CHALLENGE_INVALID', 401))
    await expect(completeSaaSMFA('challenge', '000000')).rejects.toMatchObject({
      machineCode: 'MFA_CHALLENGE_INVALID', status: 401, httpCode: 401,
    })

    fetchMock.mockResolvedValueOnce(rejected('PASSWORD_CHANGE_INVALID', 409, 422))
    await expect(changeSaaSPassword('password-change', 'new-password')).rejects.toMatchObject({
      machineCode: 'PASSWORD_CHANGE_INVALID', status: 409, httpCode: 422,
    })

    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key: string) => key === 'mochat_saas_admin_token' ? 'saas-token' : key === 'mochat_saas_admin_user_id' ? '7' : key === 'mochat_saas_admin_expires_at' ? '9999999999' : null),
      removeItem: vi.fn(),
    })
    vi.stubGlobal('location', { assign: vi.fn(), pathname: '/saas-admin', search: '', hash: '' })
    fetchMock.mockResolvedValueOnce(rejected('SESSION_INVALID', 401))
    await expect(apiRequest('/saas/auth/session')).rejects.toMatchObject({
      machineCode: 'SESSION_INVALID', status: 401, httpCode: 401,
    })
    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(new ApiError('test', 400, 'INVALID_REQUEST', 400).machineCode).toBe('INVALID_REQUEST')
  })

  it('returns the password-change continuation from the HTTP 428 challenge', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      code: 428,
      errorCode: 'PASSWORD_CHANGE_REQUIRED',
      msg: 'password change required',
      data: { passwordChangeToken: 'password-change-token', mustRotatePassword: true },
    }), {
      status: 428,
      headers: { 'Content-Type': 'application/json' },
    }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(loginSaaS('platform-admin', 'initial-password')).resolves.toEqual({
      passwordChangeToken: 'password-change-token',
      mustRotatePassword: true,
    })
  })

  it.each([
    ['wrong endpoint', () => changeSaaSPassword('challenge', 'new-password'), { passwordChangeToken: 'next', mustRotatePassword: true }],
    ['wrong envelope code', () => loginSaaS('platform-admin', 'initial-password'), { passwordChangeToken: 'next', mustRotatePassword: true }, 200],
    ['missing token', () => loginSaaS('platform-admin', 'initial-password'), { mustRotatePassword: true }],
    ['rotation not required', () => loginSaaS('platform-admin', 'initial-password'), { passwordChangeToken: 'next', mustRotatePassword: false }],
    ['session smuggling', () => loginSaaS('platform-admin', 'initial-password'), { passwordChangeToken: 'next', mustRotatePassword: true, token: 'session-token' }],
  ])('fails closed for a malformed password-change continuation: %s', async (_name, request, data, code = 428) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      code,
      errorCode: 'PASSWORD_CHANGE_REQUIRED',
      msg: 'password change required',
      data,
    }), {
      status: 428,
      headers: { 'Content-Type': 'application/json' },
    })))

    await expect(request()).rejects.toMatchObject({
      machineCode: 'PASSWORD_CHANGE_REQUIRED',
      status: 428,
    })
  })

  it('uses tenant-scoped WeCom integration endpoints and optimistic versions', async () => {
    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key: string) => key === 'mochat_saas_admin_token' ? 'saas-token' : key === 'mochat_saas_admin_user_id' ? '7' : key === 'mochat_saas_admin_expires_at' ? '9999999999' : null),
      removeItem: vi.fn(),
    })
    const fetchMock = vi.fn().mockImplementation(async () => new Response(JSON.stringify({ code: 200, data: {} }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    vi.stubGlobal('fetch', fetchMock)

    await fetchWeComIntegration(41)
    await saveWeComIntegrationCandidate(41, {
      mode: 'third_party_delegated',
      agentId: '',
      providerAppId: 'provider-app',
      employeeSecret: '',
      contactSecret: '',
      agentSecret: '',
      chatSecret: '',
      permanentCode: 'permanent-code',
      scope: ['archive.read'],
      version: 3,
    })
    await verifyWeComIntegrationCandidate(41, 4)
    await switchWeComIntegration(41, 5)
    await rollbackWeComIntegration(41, 6)
    await fetchWeComIntegrationAudits(41)

    expect(fetchMock.mock.calls.map(([path]) => path)).toEqual([
      '/dashboard/saasAdmin/tenants/41/wecom-integration',
      '/dashboard/saasAdmin/tenants/41/wecom-integration/candidate',
      '/dashboard/saasAdmin/tenants/41/wecom-integration/candidate/verify',
      '/dashboard/saasAdmin/tenants/41/wecom-integration/switch',
      '/dashboard/saasAdmin/tenants/41/wecom-integration/rollback',
      '/dashboard/saasAdmin/tenants/41/wecom-integration/audits',
    ])
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body))).toMatchObject({ mode: 'third_party_delegated', permanentCode: 'permanent-code', version: 3 })
    expect(JSON.parse(String(fetchMock.mock.calls[2]?.[1]?.body))).toEqual({ version: 4 })
    expect(JSON.parse(String(fetchMock.mock.calls[3]?.[1]?.body))).toEqual({ version: 5 })
    expect(JSON.parse(String(fetchMock.mock.calls[4]?.[1]?.body))).toEqual({ version: 6 })
  })

  it('builds an absolute fragment activation URL without moving the token into query', () => {
    const url = activationDeliveryURL('/activate#token=opaque%2Bvalue', 'https://saas.example.test')
    expect(url).toBe('https://saas.example.test/activate#token=opaque%2Bvalue')
    expect(url).not.toContain('?token=')
    expect(() => activationDeliveryURL('/activate?token=leak', 'https://saas.example.test')).toThrow('激活入口必须使用 fragment')
    expect(() => activationDeliveryURL('https://evil.example/activate#token=opaque', 'https://saas.example.test')).toThrow('激活入口必须与当前站点同源')
  })
})
