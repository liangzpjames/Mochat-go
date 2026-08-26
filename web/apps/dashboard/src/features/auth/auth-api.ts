import type { Session } from '@mochat/auth';

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

export type LoginInput = {
  phone: string;
  password: string;
};

type AuthResponse = {
  token?: string;
  userId?: number | string;
  expiresAt?: number;
  expire?: number;
  session?: { userName?: string };
  enrollmentToken?: string;
  enrollmentSecret?: string;
  otpAuthURL?: string;
  challengeToken?: string;
  passwordChangeToken?: string;
};

export type DashboardAuthPending =
  | {
    kind: 'mfa-enrollment';
    enrollmentToken: string;
    enrollmentSecret: string;
    otpAuthURL: string;
    expiresAt: number;
  }
  | {
    kind: 'mfa';
    challengeToken: string;
    expiresAt: number;
  }
  | {
    kind: 'password-change';
    passwordChangeToken: string;
    expiresAt: number;
  };

export type DashboardAuthResult = Session | DashboardAuthPending;

export type MFAInput =
  | { challengeToken: string; code: string }
  | { passwordChangeToken: string; newPassword: string };

export type ActivationInput = {
  activationToken: string;
  password: string;
};

export type DashboardActivationStatus = {
  status: 'valid' | 'expired' | 'activated' | 'revoked' | 'invalid';
  tenantName: string;
  accountHint: string;
  expiresAt: number;
  primaryAction: 'activate' | 'login' | 'contact_admin';
};

export async function inspectDashboardActivation(
  client: ApiClient,
  activationToken: string,
): Promise<DashboardActivationStatus> {
  try {
    return await client.request('/auth/activation/status', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ activationToken }),
    }) as DashboardActivationStatus;
  } catch (reason) {
    const reasonMachineCode = typeof reason === 'object' && reason !== null && 'machineCode' in reason
      ? reason.machineCode
      : undefined;
    const reasonErrorCode = typeof reason === 'object' && reason !== null && 'errorCode' in reason
      ? reason.errorCode
      : undefined;
    const errorCode = typeof reasonMachineCode === 'string' && reasonMachineCode !== ''
      ? reasonMachineCode
      : typeof reasonErrorCode === 'string' && reasonErrorCode !== ''
        ? reasonErrorCode
        : 'ACTIVATION_STATUS_FAILED';
    const error = new Error('无法检查激活入口，请重试') as Error & { errorCode: string };
    error.errorCode = errorCode;
    throw error;
  }
}

function tokenUserId(token: string): string {
  const compactToken = token.replace(/^Bearer\s+/i, '');
  const payload = compactToken.split('.')[1];
  if (payload === undefined) {
    throw new Error('登录响应包含无效令牌');
  }
  try {
    const normalized = payload.replaceAll('-', '+').replaceAll('_', '/');
    const padding = '='.repeat((4 - normalized.length % 4) % 4);
    const decoded = JSON.parse(atob(normalized + padding)) as { uid?: unknown };
    if (
      (typeof decoded.uid !== 'number' && typeof decoded.uid !== 'string')
      || String(decoded.uid) === ''
    ) {
      throw new Error('missing uid');
    }
    return String(decoded.uid);
  } catch {
    throw new Error('登录响应包含无效令牌');
  }
}

export async function authenticate(
  client: ApiClient,
  input: LoginInput,
  now = Date.now(),
): Promise<DashboardAuthResult> {
  const result = await client.request('/user/auth', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  }) as AuthResponse;
  return mapAuthResponse(result, now);
}

export async function completeMFA(
  client: ApiClient,
  input: MFAInput,
  now = Date.now(),
): Promise<DashboardAuthResult> {
  const result = await client.request('/user/authMFA', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  }) as AuthResponse;
  return mapAuthResponse(result, now);
}

export async function activate(
  client: ApiClient,
  input: ActivationInput,
): Promise<void> {
  await client.request('/auth/activate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
}

function mapAuthResponse(result: AuthResponse, now: number): DashboardAuthResult {
  if (result.enrollmentToken !== undefined) {
    if (
      result.enrollmentSecret === undefined
      || result.otpAuthURL === undefined
      || result.expiresAt === undefined
    ) {
      throw new Error('Dashboard authentication response is incomplete');
    }
    return {
      kind: 'mfa-enrollment',
      enrollmentToken: result.enrollmentToken,
      enrollmentSecret: result.enrollmentSecret,
      otpAuthURL: result.otpAuthURL,
      expiresAt: result.expiresAt * 1_000,
    };
  }
  if (result.challengeToken !== undefined) {
    if (result.expiresAt === undefined) {
      throw new Error('Dashboard MFA response is incomplete');
    }
    return {
      kind: 'mfa',
      challengeToken: result.challengeToken,
      expiresAt: result.expiresAt * 1_000,
    };
  }
  if (result.passwordChangeToken !== undefined) {
    if (result.expiresAt === undefined) {
      throw new Error('Dashboard password change response is incomplete');
    }
    return {
      kind: 'password-change',
      passwordChangeToken: result.passwordChangeToken,
      expiresAt: result.expiresAt * 1_000,
    };
  }
  if (result.token === undefined) {
    throw new Error('Dashboard authentication response is incomplete');
  }
  return {
    token: /^Bearer\s/i.test(result.token)
      ? result.token
      : `Bearer ${result.token}`,
    userId: result.userId === undefined ? tokenUserId(result.token) : String(result.userId),
    userName: result.session?.userName ?? null,
    expiresAt: result.expiresAt === undefined
      ? now + (result.expire ?? 0) * 1_000
      : result.expiresAt * 1_000,
  };
}

export async function logout(client: ApiClient): Promise<void> {
  await client.request('/user/logout', {
    method: 'PUT',
  });
}
