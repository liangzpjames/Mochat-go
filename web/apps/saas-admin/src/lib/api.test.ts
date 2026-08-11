import { beforeEach, describe, expect, it, vi } from 'vitest'

import { loginURL, logoutSaaS, readStoredToken } from './api'

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
})
