import { describe, expect, it, vi } from 'vitest';

import { createProviderStatusApi } from './provider-status-api';

describe('provider status api', () => {
  it('uses the dashboard-relative status endpoint and normalizes runtime state', async () => {
    const request = vi.fn().mockResolvedValue({
      providers: [{
        kind: 'wecom_standard',
        state: 'ready',
        code: 'wecom.runtime_verified',
        source: 'external',
        capabilities: ['employee_sync'],
        capabilityStatuses: [],
        lastSuccessAt: '2026-08-14T08:00:00Z',
      }],
      freshAt: '2026-08-14T08:01:00Z',
    });
    const result = await createProviderStatusApi({ request }).getStatus();

    expect(request).toHaveBeenCalledWith('/providers/status');
    expect(result.providers[0]).toMatchObject({
      kind: 'wecom_standard', state: 'ready', code: 'wecom.runtime_verified', source: 'external',
    });
    expect(result.providers[0]?.capabilities).toEqual(['employee_sync']);
  });

  it('does not invent status or expose unknown response fields', async () => {
    const request = vi.fn().mockResolvedValue({
      providers: [{ kind: 'unknown', state: 'broken', source: 'leak', secret: 'plaintext-secret' }],
      secret: 'plaintext-secret',
    });
    const result = await createProviderStatusApi({ request }).getStatus();

    expect(result.providers[0]).toEqual({
      kind: 'unknown', state: 'unavailable', code: 'provider.invalid_state', source: 'external',
      capabilities: [],
      capabilityStatuses: [],
    });
    expect(JSON.stringify(result)).not.toContain('plaintext-secret');
  });
});
