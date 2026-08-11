export interface SaaSSession {
  token: string
  userId: number
  userName: string
  expiresAt: number
  mustRotatePassword?: boolean
}

export const SAAS_SESSION_KEYS = {
  token: 'mochat_saas_admin_token',
  userId: 'mochat_saas_admin_user_id',
  userName: 'mochat_saas_admin_user_name',
  expiresAt: 'mochat_saas_admin_expires_at',
  mustRotatePassword: 'mochat_saas_admin_must_rotate_password',
} as const

function storage() {
  return globalThis.localStorage
}

export function readSaaSSession(): SaaSSession | null {
  const token = storage().getItem(SAAS_SESSION_KEYS.token)?.trim() || ''
  const userId = Number(storage().getItem(SAAS_SESSION_KEYS.userId) || 0)
  const userName = storage().getItem(SAAS_SESSION_KEYS.userName) || ''
  const expiresAt = Number(storage().getItem(SAAS_SESSION_KEYS.expiresAt) || 0)
  const mustRotatePassword = storage().getItem(SAAS_SESSION_KEYS.mustRotatePassword) === '1'
  if (!token || !Number.isInteger(userId) || userId <= 0 || !Number.isFinite(expiresAt) || expiresAt <= 0) return null
  return { token, userId, userName, expiresAt, mustRotatePassword }
}

export function saveSaaSSession(session: SaaSSession) {
  storage().setItem(SAAS_SESSION_KEYS.token, session.token)
  storage().setItem(SAAS_SESSION_KEYS.userId, String(session.userId))
  storage().setItem(SAAS_SESSION_KEYS.userName, session.userName)
  storage().setItem(SAAS_SESSION_KEYS.expiresAt, String(session.expiresAt))
  storage().setItem(SAAS_SESSION_KEYS.mustRotatePassword, session.mustRotatePassword ? '1' : '0')
}

export function clearSaaSSession() {
  for (const key of Object.values(SAAS_SESSION_KEYS)) storage().removeItem(key)
  if (typeof document !== 'undefined') {
    document.cookie = 'MOCHAT_SAAS_ADMIN_TOKEN=; Path=/saas-admin; Max-Age=0; SameSite=Lax'
  }
}

export function saasLoginURL() {
  const redirect = `${location.pathname}${location.search}${location.hash}`
  return `/saas/login?redirect=${encodeURIComponent(redirect)}`
}
