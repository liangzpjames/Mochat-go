export type BindingStatus = 'pending' | 'verified' | 'suspended';

export type SecretConfiguredState = {
  configured: boolean;
  keyId?: string;
  updatedAt?: string;
};

export type CompanyCredentialStates = {
  wecom: SecretConfiguredState;
  agent: SecretConfiguredState;
  archive: SecretConfiguredState;
};

/**
 * The server is the only authority for tenant/corp. The identifiers in this
 * response are display/read-only facts and are deliberately absent from all
 * mutation inputs below.
 */
export type CompanyProfile = {
  tenantId: number;
  corpId: number;
  displayName: string;
  authoritativeCorpName?: string;
  wxCorpId?: string;
  applicationAgentId?: string;
  bindingStatus: BindingStatus;
  bindingVersion: number;
  verifiedAt?: string;
  credentials: CompanyCredentialStates;
  updatedAt: string;
};

export type UpdateCompanyProfileInput = {
  displayName: string;
  expectedVersion: number;
  requestId?: string;
};

export type RotateWeComCredentialsInput = {
  employeeSecret?: string;
  contactSecret?: string;
  callbackToken?: string;
  encodingAESKey?: string;
  chatSecret?: string;
  expectedVersion: number;
  requestId?: string;
};

export type RotateAgentCredentialsInput = {
  agentId?: number;
  wxAgentId?: string;
  wxSecret?: string;
  expectedVersion: number;
  requestId?: string;
};

export type ConfigureApplicationInput = {
  wxAgentId: string;
  secret: string;
  expectedVersion: number;
  requestId?: string;
};

export type RotateArchiveCredentialsInput = {
  chatSecret?: string;
  rsaPublicKey?: string;
  rsaPrivateKey?: string;
  expectedVersion: number;
  requestId?: string;
};

export type CallbackConfiguration = {
  corpId: number;
  callbackUrl: string;
  token: string;
  encodingAESKey: string;
  configured: boolean;
  bindingVersion: number;
};

export type RegenerateCallbackConfigurationInput = {
  expectedVersion: number;
  requestId?: string;
};

export type VerifyCompanyInput = {
  wxCorpId: string;
  expectedVersion: number;
  requestId?: string;
};

export type EmployeeSyncStatus = 'queued' | 'syncing' | 'failed' | 'completed';

export type EmployeeSyncResult = {
  status: EmployeeSyncStatus;
  cursor?: string;
  departmentsCreated: number;
  departmentsUpdated: number;
  employeesCreated: number;
  employeesUpdated: number;
  startedAt?: string;
  finishedAt?: string;
  errorCode?: string;
};

export type EmployeeSyncSnapshot = {
  status: EmployeeSyncStatus;
  cursor?: string;
  departments: number;
  employees: number;
  startedAt?: string;
  finishedAt?: string;
  errorCode?: string;
};

export type CompanyAudit = {
  id: number;
  action: string;
  targetType: string;
  targetId: string;
  changedFields?: string[];
  expectedVersion?: number;
  resultVersion?: number;
  requestId?: string;
  createdAt: string;
};

export type CompanyAuditPage = {
  items: CompanyAudit[];
  page: number;
  perPage: number;
  total: number;
};

type ApiClient = {
  request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T>;
};

export type CompanyProfileApi = {
  getProfile(): Promise<CompanyProfile>;
  updateProfile(input: UpdateCompanyProfileInput): Promise<CompanyProfile>;
  rotateWeComCredentials(input: RotateWeComCredentialsInput): Promise<CompanyProfile>;
  rotateAgentCredentials(input: RotateAgentCredentialsInput): Promise<CompanyProfile>;
  configureApplication(input: ConfigureApplicationInput): Promise<CompanyProfile>;
  rotateArchiveCredentials(input: RotateArchiveCredentialsInput): Promise<CompanyProfile>;
  getCallbackConfiguration(): Promise<CallbackConfiguration>;
  regenerateCallbackConfiguration(input: RegenerateCallbackConfigurationInput): Promise<CallbackConfiguration>;
  verify(input: VerifyCompanyInput): Promise<CompanyProfile>;
  startEmployeeSync(): Promise<EmployeeSyncResult>;
  getSyncStatus(): Promise<EmployeeSyncSnapshot>;
  listAudits(input?: { page?: number; perPage?: number }): Promise<CompanyAuditPage>;
};

export function createCompanyProfileApi(client: ApiClient): CompanyProfileApi {
  return {
    async getProfile() {
      return normalizeProfile(await client.request<unknown>('/company/profile'));
    },
    async updateProfile(input) {
      return normalizeProfile(await client.request<unknown>('/company/profile', jsonRequest('PUT', input)));
    },
    async rotateWeComCredentials(input) {
      return normalizeProfile(await client.request<unknown>('/company/wecom-credentials', jsonRequest('PUT', compactSecretInput(input))));
    },
    async rotateAgentCredentials(input) {
      const body: Record<string, unknown> = {
        expectedVersion: input.expectedVersion,
        ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
        ...(input.agentId === undefined || input.agentId <= 0 ? {} : { agentId: input.agentId }),
        ...(input.wxAgentId === undefined || input.wxAgentId === '' ? {} : { wxAgentId: input.wxAgentId }),
        ...optionalSecret('wxSecret', input.wxSecret),
      };
      return normalizeProfile(await client.request<unknown>('/company/agent-credentials', jsonRequest('PUT', body)));
    },
    async configureApplication(input) {
      return normalizeProfile(await client.request<unknown>('/company/application-credentials', jsonRequest('PUT', input)));
    },
    async rotateArchiveCredentials(input) {
      return normalizeProfile(await client.request<unknown>('/company/archive-credentials', jsonRequest('PUT', compactSecretInput(input))));
    },
    async getCallbackConfiguration() {
      return normalizeCallbackConfiguration(await client.request<unknown>('/company/callback-configuration'));
    },
    async regenerateCallbackConfiguration(input) {
      return normalizeCallbackConfiguration(await client.request<unknown>('/company/callback-configuration/regenerate', jsonRequest('POST', input)));
    },
    async verify(input) {
      return normalizeProfile(await client.request<unknown>('/company/verify', jsonRequest('POST', input)));
    },
    async startEmployeeSync() {
      return normalizeSyncResult(await client.request<unknown>('/company/employee-sync', { method: 'POST' }));
    },
    async getSyncStatus() {
      return normalizeSyncStatus(await client.request<unknown>('/company/sync-status'));
    },
    async listAudits(input = {}) {
      const query = new URLSearchParams();
      if (input.page !== undefined) query.set('page', String(input.page));
      if (input.perPage !== undefined) query.set('perPage', String(input.perPage));
      const suffix = query.toString();
      return normalizeAuditPage(await client.request<unknown>(`/company/audits${suffix === '' ? '' : `?${suffix}`}`));
    },
  };
}

function jsonRequest(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  };
}

function optionalSecret(name: string, value: string | undefined): Record<string, string> {
  return value === undefined || value === '' ? {} : { [name]: value };
}

function compactSecretInput(input: RotateWeComCredentialsInput | RotateArchiveCredentialsInput): Record<string, unknown> {
  const body: Record<string, unknown> = {
    expectedVersion: input.expectedVersion,
    ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
  };
  for (const name of ['employeeSecret', 'contactSecret', 'callbackToken', 'encodingAESKey', 'chatSecret', 'rsaPublicKey', 'rsaPrivateKey'] as const) {
    const value = (input as Record<string, unknown>)[name];
    Object.assign(body, optionalSecret(name, typeof value === 'string' ? value : undefined));
  }
  return body;
}

function normalizeCallbackConfiguration(value: unknown): CallbackConfiguration {
  const source = record(value);
  return {
    corpId: numberValue(source.corpId),
    callbackUrl: typeof source.callbackUrl === 'string' ? source.callbackUrl : '',
    token: typeof source.token === 'string' ? source.token : '',
    encodingAESKey: typeof source.encodingAESKey === 'string' ? source.encodingAESKey : '',
    configured: source.configured === true,
    bindingVersion: numberValue(source.bindingVersion),
  };
}

function record(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null ? value as Record<string, unknown> : {};
}

function stringValue(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

function numberValue(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function optionalNumber(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function normalizeCredentialState(value: unknown): SecretConfiguredState {
  const source = record(value);
  const result: SecretConfiguredState = { configured: source.configured === true };
  const keyId = stringValue(source.keyId);
  const updatedAt = stringValue(source.updatedAt);
  if (keyId !== undefined) result.keyId = keyId;
  if (updatedAt !== undefined) result.updatedAt = updatedAt;
  return result;
}

function normalizeProfile(value: unknown): CompanyProfile {
  const source = record(value);
  const credentials = record(source.credentials);
  const status = source.bindingStatus === 'verified' || source.bindingStatus === 'suspended'
    ? source.bindingStatus
    : 'pending';
  const result: CompanyProfile = {
    tenantId: numberValue(source.tenantId),
    corpId: numberValue(source.corpId),
    displayName: typeof source.displayName === 'string' ? source.displayName : '',
    bindingStatus: status,
    bindingVersion: numberValue(source.bindingVersion),
    credentials: {
      wecom: normalizeCredentialState(credentials.wecom),
      agent: normalizeCredentialState(credentials.agent),
      archive: normalizeCredentialState(credentials.archive),
    },
    updatedAt: typeof source.updatedAt === 'string' ? source.updatedAt : '',
  };
  const authoritativeCorpName = stringValue(source.authoritativeCorpName);
  const wxCorpId = stringValue(source.wxCorpId);
  const applicationAgentId = stringValue(source.applicationAgentId);
  const verifiedAt = stringValue(source.verifiedAt);
  if (authoritativeCorpName !== undefined) result.authoritativeCorpName = authoritativeCorpName;
  if (wxCorpId !== undefined) result.wxCorpId = wxCorpId;
  if (applicationAgentId !== undefined) result.applicationAgentId = applicationAgentId;
  if (verifiedAt !== undefined) result.verifiedAt = verifiedAt;
  return result;
}

function normalizeSyncResult(value: unknown): EmployeeSyncResult {
  const source = record(value);
  const result: EmployeeSyncResult = {
    status: syncStatus(source.status),
    departmentsCreated: numberValue(source.departmentsCreated),
    departmentsUpdated: numberValue(source.departmentsUpdated),
    employeesCreated: numberValue(source.employeesCreated),
    employeesUpdated: numberValue(source.employeesUpdated),
  };
  const cursor = stringValue(source.cursor);
  const startedAt = stringValue(source.startedAt);
  const finishedAt = stringValue(source.finishedAt);
  const errorCode = stringValue(source.errorCode);
  if (cursor !== undefined) result.cursor = cursor;
  if (startedAt !== undefined) result.startedAt = startedAt;
  if (finishedAt !== undefined) result.finishedAt = finishedAt;
  if (errorCode !== undefined) result.errorCode = errorCode;
  return result;
}

function normalizeSyncStatus(value: unknown): EmployeeSyncSnapshot {
  const source = record(value);
  const result: EmployeeSyncSnapshot = {
    status: syncStatus(source.status),
    departments: numberValue(source.departments),
    employees: numberValue(source.employees),
  };
  const cursor = stringValue(source.cursor);
  const startedAt = stringValue(source.startedAt);
  const finishedAt = stringValue(source.finishedAt);
  const errorCode = stringValue(source.errorCode);
  if (cursor !== undefined) result.cursor = cursor;
  if (startedAt !== undefined) result.startedAt = startedAt;
  if (finishedAt !== undefined) result.finishedAt = finishedAt;
  if (errorCode !== undefined) result.errorCode = errorCode;
  return result;
}

function syncStatus(value: unknown): EmployeeSyncStatus {
  return value === 'syncing' || value === 'failed' || value === 'completed' ? value : 'queued';
}

function normalizeAuditPage(value: unknown): CompanyAuditPage {
  const source = record(value);
  const items = Array.isArray(source.items) ? source.items : [];
  return {
    items: items.map((item): CompanyAudit => {
      const audit = record(item);
      const changedFields = Array.isArray(audit.changedFields)
        ? audit.changedFields.filter((field): field is string => typeof field === 'string')
        : undefined;
      const result: CompanyAudit = {
        id: numberValue(audit.id),
        action: typeof audit.action === 'string' ? audit.action : '',
        targetType: typeof audit.targetType === 'string' ? audit.targetType : '',
        targetId: typeof audit.targetId === 'string' ? audit.targetId : '',
        createdAt: typeof audit.createdAt === 'string' ? audit.createdAt : '',
      };
      const expectedVersion = optionalNumber(audit.expectedVersion);
      const resultVersion = optionalNumber(audit.resultVersion);
      const requestId = stringValue(audit.requestId);
      if (changedFields !== undefined) result.changedFields = changedFields;
      if (expectedVersion !== undefined) result.expectedVersion = expectedVersion;
      if (resultVersion !== undefined) result.resultVersion = resultVersion;
      if (requestId !== undefined) result.requestId = requestId;
      return result;
    }),
    page: numberValue(source.page),
    perPage: numberValue(source.perPage),
    total: numberValue(source.total),
  };
}
