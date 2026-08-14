export type ProviderStatusState = 'ready' | 'limited' | 'unavailable';
export type ProviderStatusSource = 'external' | 'simulated' | 'local' | 'code_only';

export type ProviderStatus = {
  kind: string;
  state: ProviderStatusState;
  code: string;
  source: ProviderStatusSource;
  reason?: string;
  action?: string;
  capabilities: string[];
  missing?: string[];
  lastSyncAt?: string;
  lastSuccessAt?: string;
  lastFailureAt?: string;
  lastErrorCode?: string;
  capabilityStatuses: CapabilityStatus[];
};

export type CapabilityStatus = {
  capability: string;
  state: ProviderStatusState;
  code?: string;
  source?: ProviderStatusSource;
  reason?: string;
  action?: string;
  missing?: string[];
  lastSyncAt?: string;
  lastSuccessAt?: string;
  lastFailureAt?: string;
  lastErrorCode?: string;
};

export type ProviderStatusView = {
  providers: ProviderStatus[];
  freshAt: string;
};

type ApiClient = {
  request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T>;
};

export type ProviderStatusApi = {
  getStatus(): Promise<ProviderStatusView>;
};

export function createProviderStatusApi(client: ApiClient): ProviderStatusApi {
  return {
    async getStatus() {
      return normalizeView(await client.request<unknown>('/providers/status'));
    },
  };
}

function normalizeView(value: unknown): ProviderStatusView {
  const source = record(value);
  const providers = Array.isArray(source.providers) ? source.providers.map(normalizeStatus) : [];
  return {
    providers,
    freshAt: stringValue(source.freshAt) ?? '',
  };
}

function normalizeStatus(value: unknown): ProviderStatus {
  const source = record(value);
  const rawState = source.state;
  const state: ProviderStatusState = rawState === 'ready' || rawState === 'limited' || rawState === 'unavailable'
    ? rawState
    : 'unavailable';
  const rawSource = source.source;
  const providerSource: ProviderStatusSource = rawSource === 'external' || rawSource === 'simulated' || rawSource === 'local' || rawSource === 'code_only'
    ? rawSource
    : 'external';
  const result: ProviderStatus = {
    kind: stringValue(source.kind) ?? 'unknown',
    state,
    code: stringValue(source.code) ?? 'provider.invalid_state',
    source: providerSource,
    capabilities: stringArray(source.capabilities),
    capabilityStatuses: [],
  };
  if (state === 'unavailable' && rawState !== state) result.code = 'provider.invalid_state';
  addOptionalString(result, 'reason', source.reason);
  addOptionalString(result, 'action', source.action);
  addOptionalStringArray(result, 'missing', source.missing);
  addOptionalString(result, 'lastSyncAt', source.lastSyncAt);
  addOptionalString(result, 'lastSuccessAt', source.lastSuccessAt);
  addOptionalString(result, 'lastFailureAt', source.lastFailureAt);
  addOptionalString(result, 'lastErrorCode', source.lastErrorCode);
  result.capabilityStatuses = Array.isArray(source.capabilityStatuses)
    ? source.capabilityStatuses.map(normalizeCapabilityStatus)
    : [];
  return result;
}

function normalizeCapabilityStatus(value: unknown): CapabilityStatus {
  const source = record(value);
  const rawState = source.state;
  const state: ProviderStatusState = rawState === 'ready' || rawState === 'limited' || rawState === 'unavailable'
    ? rawState
    : 'unavailable';
  const rawSource = source.source;
  const capabilitySource: ProviderStatusSource | undefined = rawSource === 'external' || rawSource === 'simulated' || rawSource === 'local' || rawSource === 'code_only'
    ? rawSource
    : undefined;
  const result: CapabilityStatus = {
    capability: stringValue(source.capability) ?? 'unknown',
    state,
  };
  addOptionalString(result, 'code', source.code);
  if (capabilitySource !== undefined) result.source = capabilitySource;
  addOptionalString(result, 'reason', source.reason);
  addOptionalString(result, 'action', source.action);
  addOptionalStringArray(result, 'missing', source.missing);
  addOptionalString(result, 'lastSyncAt', source.lastSyncAt);
  addOptionalString(result, 'lastSuccessAt', source.lastSuccessAt);
  addOptionalString(result, 'lastFailureAt', source.lastFailureAt);
  addOptionalString(result, 'lastErrorCode', source.lastErrorCode);
  return result;
}

function record(value: unknown): Record<string, unknown> {
  return isRecord(value) ? value : {};
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() !== '' ? value : undefined;
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
}

type OptionalProviderStatusField = 'code' | 'reason' | 'action' | 'lastSyncAt' | 'lastSuccessAt' | 'lastFailureAt' | 'lastErrorCode';

function addOptionalString(target: ProviderStatus | CapabilityStatus, key: OptionalProviderStatusField, value: unknown): void {
  const normalized = stringValue(value);
  if (normalized === undefined) return;
  switch (key) {
    case 'code': if ('code' in target) target.code = normalized; break;
    case 'reason': target.reason = normalized; break;
    case 'action': target.action = normalized; break;
    case 'lastSyncAt': target.lastSyncAt = normalized; break;
    case 'lastSuccessAt': target.lastSuccessAt = normalized; break;
    case 'lastFailureAt': target.lastFailureAt = normalized; break;
    case 'lastErrorCode': target.lastErrorCode = normalized; break;
  }
}

function addOptionalStringArray(target: ProviderStatus | CapabilityStatus, key: 'missing', value: unknown): void {
  const normalized = stringArray(value);
  if (normalized.length > 0) target[key] = normalized;
}
