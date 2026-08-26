import type { ApprovalPoliciesData, GovernedResult } from './types'
import { clearSaaSSession, readSaaSSession, saasLoginURL, saveSaaSSession, type SaaSSession } from './auth-session'

interface ApiEnvelope<T> {
  code: number
  data: T
  errorCode?: string
  message?: string
  msg?: string
}

export class ApiError extends Error {
  status: number
  httpCode: number
  machineCode: string

  constructor(message: string, status: number, machineCode: string, httpCode = status) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.httpCode = httpCode
    this.machineCode = machineCode
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
    throw new ApiError(`服务响应格式错误（HTTP ${response.status}）`, response.status, 'INVALID_RESPONSE')
  }
  const httpCode = Number(body.code || response.status)
  if (!response.ok || httpCode >= 400) {
    throw new ApiError(body.msg || body.message || '认证失败', response.status, body.errorCode || 'AUTH_REQUEST_FAILED', httpCode)
  }
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
    throw new ApiError('登录状态已失效', 401, 'SESSION_INVALID')
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
    throw new ApiError(`服务响应格式错误（HTTP ${response.status}）`, response.status, 'INVALID_RESPONSE')
  }
  const httpCode = Number(body.code || response.status)
  if (!response.ok || httpCode >= 400) {
    if (response.status === 401) {
      clearStoredToken()
      location.assign(loginURL())
    }
    throw new ApiError(body.msg || body.message || `请求失败（HTTP ${response.status}）`, response.status, body.errorCode || 'API_REQUEST_FAILED', httpCode)
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

export type WeComIntegrationMode = 'self_built' | 'third_party_delegated'

export interface WeComIntegrationRecord {
  id: string
  mode: WeComIntegrationMode
  slot: 'current' | 'candidate'
  status: 'unconfigured' | 'pending_verification' | 'active' | 'suspended' | 'revoked' | 'failed'
  verifiedWxCorpId: string
  agentId: string
  providerAppId: string
  credentialConfigured: boolean
  credentialHint: string
  scope: string[]
  scopeDigest: string
  missingCapabilities: string[]
  generation: number
  version: number
  verificationLevel: string
  verifiedAt: string
  lastErrorCode: string
  updatedAt: string
}

export interface WeComIntegrationView {
  tenantId: number
  corpId: number
  current: WeComIntegrationRecord | null
  candidate: WeComIntegrationRecord | null
}

export interface WeComIntegrationCandidateInput {
  mode: WeComIntegrationMode
  agentId?: string
  providerAppId?: string
  employeeSecret?: string
  contactSecret?: string
  agentSecret?: string
  chatSecret?: string
  permanentCode?: string
  scope: string[]
  version: number
}

export interface WeComIntegrationAudit {
  id: number
  action: string
  targetId: string
  before: string
  after: string
  actorUserId: number
  createdAt: string
}

function weComIntegrationPath(tenantId: number, suffix = '') {
  if (!Number.isInteger(tenantId) || tenantId <= 0) throw new Error('租户 ID 无效')
  return `/dashboard/saasAdmin/tenants/${tenantId}/wecom-integration${suffix}`
}

export function fetchWeComIntegration(tenantId: number) {
  return apiRequest<WeComIntegrationView>(weComIntegrationPath(tenantId))
}

export function saveWeComIntegrationCandidate(tenantId: number, input: WeComIntegrationCandidateInput) {
  return apiRequest<WeComIntegrationRecord>(weComIntegrationPath(tenantId, '/candidate'), jsonRequest('PUT', input))
}

export function verifyWeComIntegrationCandidate(tenantId: number, version: number) {
  return apiRequest<WeComIntegrationRecord>(weComIntegrationPath(tenantId, '/candidate/verify'), jsonRequest('POST', { version }))
}

export function switchWeComIntegration(tenantId: number, version: number) {
  return apiRequest<WeComIntegrationView>(weComIntegrationPath(tenantId, '/switch'), jsonRequest('POST', { version }))
}

export function rollbackWeComIntegration(tenantId: number, version: number) {
  return apiRequest<WeComIntegrationView>(weComIntegrationPath(tenantId, '/rollback'), jsonRequest('POST', { version }))
}

export function fetchWeComIntegrationAudits(tenantId: number) {
  return apiRequest<WeComIntegrationAudit[]>(weComIntegrationPath(tenantId, '/audits'))
}

export function activationDeliveryURL(path: string, origin = location.origin): string {
  const base = new URL(origin)
  const value = new URL(path, base)
  if (value.origin !== base.origin) throw new Error('激活入口必须与当前站点同源')
  if (value.pathname !== '/activate' || value.searchParams.has('token') || !value.hash.startsWith('#token=')) {
    throw new Error('激活入口必须使用 fragment')
  }
  return value.toString()
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
  approvalPayload?: unknown
  approvalIdempotencyKey?: string
  reason: string
  directPath: string
  directMethod?: 'POST' | 'PUT'
  directHeaders?: HeadersInit
}): Promise<GovernedResult<T>> {
  if (policyRequiresApproval(options.approvalMode, options.actionType)) {
    const policy = options.approvalMode?.policies.find((item) => item.actionType === options.actionType)
    const data = await apiRequest<T>('/dashboard/saasAdmin/approvalRequest', jsonRequest('POST', {
      actionType: options.actionType,
      payload: options.approvalPayload ?? options.payload,
      reason: options.reason,
      expiresInHours: Number(policy?.expiryHours || 24),
      idempotencyKey: options.approvalIdempotencyKey || `saas-admin:${options.actionType}:${Date.now()}`,
    }))
    return { approvalRequested: true, data }
  }
  const directRequest = jsonRequest(options.directMethod || 'POST', options.payload)
  if (options.directHeaders) directRequest.headers = options.directHeaders
  const data = await apiRequest<T>(options.directPath, directRequest)
  return { approvalRequested: false, data }
}
