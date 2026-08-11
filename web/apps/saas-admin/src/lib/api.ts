import type { ApprovalPoliciesData, GovernedResult } from './types'
import { clearSaaSSession, readSaaSSession, saasLoginURL, saveSaaSSession, type SaaSSession } from './auth-session'

interface ApiEnvelope<T> {
  code: number
  data: T
  message?: string
  msg?: string
}

export class ApiError extends Error {
  status: number
  code: number

  constructor(message: string, status: number, code: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export function readStoredToken(): string {
  return readSaaSSession()?.token || ''
}

export function clearStoredToken() {
  clearSaaSSession()
}

export function loginURL() {
  return saasLoginURL()
}

export interface SaaSLoginResult {
  token?: string
  userId?: number
  userName?: string
  expiresAt?: number
  challengeToken?: string
  enrollmentToken?: string
  enrollmentSecret?: string
  otpAuthURL?: string
  expiresIn?: number
  passwordChangeToken?: string
  mfaRequired?: boolean
  mustRotatePassword?: boolean
}

async function saasAuthRequest<T>(path: string, init: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: { Accept: 'application/json', 'Content-Type': 'application/json', ...(init.headers || {}) },
    credentials: 'same-origin',
  })
  let body: ApiEnvelope<T> & { errorCode?: string }
  try {
    body = (await response.json()) as ApiEnvelope<T> & { errorCode?: string }
  } catch {
    throw new ApiError(`服务响应格式错误（HTTP ${response.status}）`, response.status, response.status)
  }
  if (!response.ok) throw new ApiError(body.msg || body.message || '认证失败', response.status, response.status)
  return body.data
}

export async function loginSaaS(login: string, password: string): Promise<SaaSLoginResult> {
  return saasAuthRequest<SaaSLoginResult>('/saas/auth/login', jsonRequest('POST', { login, password }))
}

export async function completeSaaSMFA(challengeToken: string, code: string): Promise<SaaSLoginResult> {
  return saasAuthRequest<SaaSLoginResult>('/saas/auth/mfa', jsonRequest('POST', { challengeToken, code }))
}

export async function changeSaaSPassword(passwordChangeToken: string, newPassword: string): Promise<SaaSLoginResult> {
  return saasAuthRequest<SaaSLoginResult>('/saas/auth/password', jsonRequest('POST', { passwordChangeToken, newPassword }))
}

export function persistSaaSLogin(result: SaaSLoginResult): SaaSSession | null {
  if (!result.token || !result.userId || !result.expiresAt) return null
  const session: SaaSSession = {
    token: result.token,
    userId: result.userId,
    userName: result.userName || '',
    expiresAt: result.expiresAt,
  }
  if (result.mustRotatePassword !== undefined) session.mustRotatePassword = result.mustRotatePassword
  saveSaaSSession(session)
  return session
}

export async function logoutSaaS() {
  const token = readStoredToken()
  try {
    await fetch('/saas/auth/logout', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json',
        ...(token ? { Authorization: token.toLowerCase().startsWith('bearer ') ? token : `Bearer ${token}` } : {}),
      },
    })
  } finally {
    clearSaaSSession()
  }
}

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = readStoredToken()
  if (!token) {
    location.assign(loginURL())
    throw new ApiError('登录状态已失效', 401, 401)
  }

  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  headers.set('Authorization', token.toLowerCase().startsWith('bearer ') ? token : `Bearer ${token}`)
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')

  const response = await fetch(path, { ...init, headers, credentials: 'same-origin' })
  let body: ApiEnvelope<T>
  try {
    body = (await response.json()) as ApiEnvelope<T>
  } catch {
    throw new ApiError(`服务响应格式错误（HTTP ${response.status}）`, response.status, response.status)
  }
  const code = Number(body.code || response.status)
  if (!response.ok || code >= 400) {
    if (response.status === 401) {
      clearStoredToken()
      location.assign(loginURL())
    }
    throw new ApiError(body.msg || body.message || `请求失败（HTTP ${response.status}）`, response.status, code)
  }
  return body.data
}

export function jsonRequest(method: 'POST' | 'PUT' | 'DELETE', payload?: unknown): RequestInit {
  if (payload === undefined) return { method }
  return { method, body: JSON.stringify(payload) }
}

export function hasPermission(permissions: string[], permission: string): boolean {
  return permissions.includes('*') || permissions.includes(permission)
}

function policyRequiresApproval(mode: ApprovalPoliciesData | undefined, actionType: string): boolean {
  if (!mode?.required) return false
  const policy = mode.policies.find((item) => item.actionType === actionType)
  return Boolean(policy?.enabled)
}

export async function executeGoverned<T>(options: {
  approvalMode: ApprovalPoliciesData | undefined
  actionType: string
  payload: unknown
  reason: string
  directPath: string
  directMethod?: 'POST' | 'PUT'
}): Promise<GovernedResult<T>> {
  if (policyRequiresApproval(options.approvalMode, options.actionType)) {
    const policy = options.approvalMode?.policies.find((item) => item.actionType === options.actionType)
    const data = await apiRequest<T>('/dashboard/saasAdmin/approvalRequest', jsonRequest('POST', {
      actionType: options.actionType,
      payload: options.payload,
      reason: options.reason,
      expiresInHours: Number(policy?.expiryHours || 24),
      idempotencyKey: `saas-admin:${options.actionType}:${Date.now()}`,
    }))
    return { approvalRequested: true, data }
  }
  const data = await apiRequest<T>(
    options.directPath,
    jsonRequest(options.directMethod || 'POST', options.payload),
  )
  return { approvalRequested: false, data }
}
