import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, apiRequest, changeSaaSPassword, completeSaaSMFA, loginURL, logoutSaaS, readStoredToken } from './api'

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
})
