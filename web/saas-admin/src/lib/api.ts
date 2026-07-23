import type { ApprovalPoliciesData, GovernedResult } from './types'

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
  const values = [
    localStorage.getItem('mochat_go_saas_admin_token'),
    localStorage.getItem('ACCESS_TOKEN'),
  ]
  for (const value of values) {
    if (!value) continue
    let normalized = value.trim()
    try {
      const parsed: unknown = JSON.parse(normalized)
      if (typeof parsed === 'string') normalized = parsed.trim()
    } catch {
      // Existing dashboard tokens may be stored as plain strings.
    }
    if (normalized) return normalized
  }
  return ''
}

export function clearStoredToken() {
  localStorage.removeItem('mochat_go_saas_admin_token')
  localStorage.removeItem('ACCESS_TOKEN')
  document.cookie = 'ACCESS_TOKEN=; Path=/; Max-Age=0; SameSite=Lax'
}

export function loginURL() {
  const redirect = `${location.pathname}${location.search}${location.hash}`
  return `/security/login?redirect=${encodeURIComponent(redirect)}`
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
  return {
    method,
    body: payload === undefined ? undefined : JSON.stringify(payload),
  }
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
