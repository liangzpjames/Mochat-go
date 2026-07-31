import { beforeEach, describe, expect, it, vi } from 'vitest'

import { readStoredToken } from './api'

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
})
