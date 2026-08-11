import { describe, expect, it, vi } from 'vitest';

import { createCompanyProfileApi } from './company-profile-api';

function jsonBody(init: RequestInit | undefined): Record<string, unknown> {
  const body = init?.body;
  if (typeof body !== 'string') return {};
  return JSON.parse(body) as Record<string, unknown>;
}

function callInit(request: ReturnType<typeof vi.fn>, index: number): RequestInit | undefined {
  const call = request.mock.calls[index] as unknown as [RequestInfo | URL, RequestInit?] | undefined;
  return call?.[1];
}

describe('company profile api', () => {
  it('loads the unique company profile without exposing secret fields', async () => {
    const request = vi.fn().mockResolvedValue({
      tenantId: 7,
      corpId: 11,
      displayName: 'Acme',
      bindingStatus: 'verified',
      bindingVersion: 4,
      credentials: {
        wecom: { configured: true, keyId: 'key-wecom', secret: 'ciphertext-leak' },
        agent: { configured: false },
        archive: { configured: true, keyId: 'key-archive' },
      },
      employeeSecret: 'plaintext-leak',
    });
    const api = createCompanyProfileApi({ request });

    const profile = await api.getProfile();

    expect(request).toHaveBeenCalledWith('/company/profile');
    expect(profile.displayName).toBe('Acme');
    expect(profile.credentials.wecom.configured).toBe(true);
    expect('employeeSecret' in profile).toBe(false);
    expect('contactSecret' in profile.credentials.wecom).toBe(false);
    expect(JSON.stringify(profile)).not.toContain('leak');
  });

  it('uses company endpoints and never sends tenant, corp, actor, or empty secret fields', async () => {
    const request = vi.fn().mockResolvedValue({});
    const api = createCompanyProfileApi({ request });

    await api.updateProfile({ displayName: 'New name', expectedVersion: 2, requestId: 'profile-1' });
    await api.rotateWeComCredentials({
      employeeSecret: '',
      contactSecret: 'new-contact-secret',
      callbackToken: '',
      encodingAESKey: '',
      chatSecret: '',
      expectedVersion: 3,
      requestId: 'wecom-1',
    });
    await api.rotateAgentCredentials({ agentId: 5, wxAgentId: 'agent-5', wxSecret: '', expectedVersion: 4, requestId: 'agent-1' });
    await api.rotateArchiveCredentials({ chatSecret: '', expectedVersion: 5, requestId: 'archive-1' });
    await api.verify({ wxCorpId: 'wx-corp', expectedVersion: 6, requestId: 'verify-1' });

    expect(request).toHaveBeenNthCalledWith(1, '/company/profile', expect.objectContaining({ method: 'PUT' }));
    expect(jsonBody(callInit(request, 0))).toEqual({ displayName: 'New name', expectedVersion: 2, requestId: 'profile-1' });
    expect(jsonBody(callInit(request, 1))).toEqual({ contactSecret: 'new-contact-secret', expectedVersion: 3, requestId: 'wecom-1' });
    expect(jsonBody(callInit(request, 2))).toEqual({ agentId: 5, wxAgentId: 'agent-5', expectedVersion: 4, requestId: 'agent-1' });
    expect(jsonBody(callInit(request, 3))).toEqual({ expectedVersion: 5, requestId: 'archive-1' });
    expect(jsonBody(callInit(request, 4))).toEqual({ wxCorpId: 'wx-corp', expectedVersion: 6, requestId: 'verify-1' });

    for (let index = 0; index < request.mock.calls.length; index += 1) {
      const body = jsonBody(callInit(request, index));
      expect(body).not.toHaveProperty('tenantId');
      expect(body).not.toHaveProperty('corpId');
      expect(body).not.toHaveProperty('actorId');
    }
  });

  it('uses the employee sync, status, and audit contracts under dashboard/company', async () => {
    const request = vi.fn().mockResolvedValue({});
    const api = createCompanyProfileApi({ request });

    await api.startEmployeeSync();
    await api.getSyncStatus();
    await api.listAudits({ page: 2, perPage: 10 });

    expect(request.mock.calls[0]).toEqual(['/company/employee-sync', { method: 'POST' }]);
    expect(request.mock.calls[1]).toEqual(['/company/sync-status']);
    expect(request.mock.calls[2]).toEqual(['/company/audits?page=2&perPage=10']);
  });
});
