// @vitest-environment jsdom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
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

  return {
    ApiError: MockApiError,
    loginSaaS: vi.fn(),
    completeSaaSMFA: vi.fn(),
    changeSaaSPassword: vi.fn(),
    persistSaaSLogin: vi.fn((result: { token?: string; userId?: number; expiresAt?: number }) => result.token && result.userId && result.expiresAt ? result : null),
  }
})

vi.mock('@/lib/api', () => mocks)

import LoginPage from './LoginPage'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('SaaS login component flow', () => {
  let container: HTMLDivElement
  let root: Root
  let assign: ReturnType<typeof vi.fn>

  beforeEach(() => {
    document.body.innerHTML = ''
    container = document.createElement('div')
    document.body.appendChild(container)
    assign = vi.fn()
    vi.stubGlobal('location', { assign, pathname: '/saas/login', search: '', hash: '' })
    mocks.loginSaaS.mockReset()
    mocks.completeSaaSMFA.mockReset()
    mocks.changeSaaSPassword.mockReset()
    mocks.persistSaaSLogin.mockClear()
    root = createRoot(container)
    act(() => root.render(<LoginPage />))
  })

  afterEach(() => {
    act(() => root.unmount())
    vi.unstubAllGlobals()
  })

  it('completes credentials, one-time enrollment, TOTP, password rotation, and session redirect', async () => {
    mocks.loginSaaS.mockResolvedValueOnce({ enrollmentToken: 'enrollment-token', enrollmentSecret: 'TEST-ENROLLMENT-SECRET', otpAuthURL: 'otpauth://test', mustRotatePassword: true })
    mocks.completeSaaSMFA.mockResolvedValueOnce({ passwordChangeToken: 'password-change-token', mustRotatePassword: true })
    mocks.changeSaaSPassword.mockResolvedValueOnce({ token: 'saas-session-token', userId: 7, userName: 'Platform Admin', expiresAt: 12345 })

    setInput(0, 'platform-admin')
    setInput(1, 'initial-password')
    await submit()
    expect(mocks.loginSaaS).toHaveBeenCalledWith('platform-admin', 'initial-password')
    expect(container.textContent).toContain('TEST-ENROLLMENT-SECRET')
    expect(assign).not.toHaveBeenCalled()

    setInput(0, '123456')
    await submit()
    expect(mocks.completeSaaSMFA).toHaveBeenCalledWith('enrollment-token', '123456')
    expect(container.querySelector('input[type="password"]')).not.toBeNull()
    expect(assign).not.toHaveBeenCalled()

    setInput(0, 'rotated-password')
    await submit()
    expect(mocks.changeSaaSPassword).toHaveBeenCalledWith('password-change-token', 'rotated-password')
    expect(assign).toHaveBeenCalledWith('/saas-admin/')
  })

  it('uses an ordinary persistent MFA challenge after enrollment', async () => {
    mocks.loginSaaS.mockResolvedValueOnce({ challengeToken: 'login-mfa-token', mfaRequired: true })
    mocks.completeSaaSMFA.mockResolvedValueOnce({ token: 'saas-session-token', userId: 7, expiresAt: 12345 })

    setInput(0, 'platform-admin')
    setInput(1, 'rotated-password')
    await submit()
    expect(container.textContent).not.toContain('TEST-ENROLLMENT-SECRET')
    setInput(0, '654321')
    await submit()

    expect(mocks.completeSaaSMFA).toHaveBeenCalledWith('login-mfa-token', '654321')
    expect(assign).toHaveBeenCalledWith('/saas-admin/')
  })

  it('keeps the credentials stage and permits retry after a uniform authentication error', async () => {
    mocks.loginSaaS.mockRejectedValueOnce(new mocks.ApiError('invalid credentials', 401, 'INVALID_CREDENTIALS'))
    mocks.loginSaaS.mockResolvedValueOnce({ challengeToken: 'retry-mfa-token', mfaRequired: true })

    setInput(0, 'platform-admin')
    setInput(1, 'wrong-password')
    await submit()
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('invalid credentials')
    expect(container.querySelectorAll('input')).toHaveLength(2)

    await submit()
    expect(mocks.loginSaaS).toHaveBeenCalledTimes(2)
    expect(container.querySelectorAll('input')).toHaveLength(1)
    expect(assign).not.toHaveBeenCalled()
  })

  it('does not enter the management page until the final session token exists', async () => {
    mocks.loginSaaS.mockResolvedValueOnce({ enrollmentToken: 'enrollment-token', enrollmentSecret: 'TEST-ENROLLMENT-SECRET' })
    mocks.completeSaaSMFA.mockResolvedValueOnce({ passwordChangeToken: 'password-change-token', mustRotatePassword: true })

    setInput(0, 'platform-admin')
    setInput(1, 'initial-password')
    await submit()
    setInput(0, '123456')
    await submit()

    expect(assign).not.toHaveBeenCalled()
    expect(mocks.persistSaaSLogin).not.toHaveBeenCalled()
  })

  it('keeps the form full-width at a 390px viewport', () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    const section = container.querySelector('section')
    expect(section?.className).toContain('w-full')
    expect(section?.className).toContain('max-w-md')
    expect(section?.className).not.toContain('min-w-')
    expect(container.querySelector('form')?.className).toContain('grid')
  })
})

function setInput(index: number, value: string) {
  const input = document.querySelectorAll('input')[index] as HTMLInputElement | undefined
  if (!input) throw new Error(`input ${index} not found`)
  act(() => {
    const valueSetter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set
    valueSetter?.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
    input.dispatchEvent(new Event('change', { bubbles: true }))
  })
}

async function submit() {
  const form = document.querySelector('form')
  if (!form) throw new Error('login form not found')
  await act(async () => {
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await Promise.resolve()
  })
}
