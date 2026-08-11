import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  clearSaaSSession,
  readSaaSSession,
  saveSaaSSession,
  saasLoginURL,
} from './auth-session'

describe('SaaS Admin session boundary', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key: string) => key === 'mochat_dashboard_token' ? 'dashboard-token' : null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
    })
    vi.stubGlobal('location', { pathname: '/saas-admin', search: '', hash: '' })
  })

  it('uses only the SaaS namespace and never consumes Dashboard storage', () => {
    expect(readSaaSSession()).toBeNull()
    saveSaaSSession({ token: 'saas-token', userId: 7, userName: 'Platform Admin', expiresAt: 123 })
    expect(localStorage.setItem).toHaveBeenCalledWith('mochat_saas_admin_token', expect.any(String))
    expect(localStorage.setItem).not.toHaveBeenCalledWith('mochat_dashboard_token', expect.anything())
  })

  it('clears only SaaS keys and redirects to the SaaS login page', () => {
    clearSaaSSession()
    expect(localStorage.removeItem).toHaveBeenCalledWith('mochat_saas_admin_token')
    expect(localStorage.removeItem).not.toHaveBeenCalledWith('mochat_dashboard_token')
    expect(saasLoginURL()).toBe('/saas/login?redirect=%2Fsaas-admin')
  })
})
