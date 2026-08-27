import { describe, expect, it, vi } from 'vitest';

import { createArchiveComponentClient } from './archive-component-client';

describe('createArchiveComponentClient', () => {
  const componentID = '8ff7bf2d-5604-43bc-a600-3ec91d575085';

  it('creates a short-lived same-origin component session', async () => {
    const request = vi.fn().mockResolvedValue({
      sessionUrl: `/dashboard/archive/components/session/${'a'.repeat(43)}`,
      expiresIn: 60,
    });
    const client = createArchiveComponentClient({ request }, 'https://dashboard.example');

    await expect(client.createSession(`/dashboard/archive/components/${componentID}/session`)).resolves.toEqual({
      sessionUrl: `/dashboard/archive/components/session/${'a'.repeat(43)}`,
      expiresIn: 60,
    });
    expect(request).toHaveBeenCalledWith(
      new URL(`/dashboard/archive/components/${componentID}/session`, 'https://dashboard.example'),
      { method: 'POST' },
    );
  });

  it('rejects untrusted inputs and malformed responses', async () => {
    const request = vi.fn().mockResolvedValue({ sessionUrl: 'https://attacker.example/content', expiresIn: 60 });
    const client = createArchiveComponentClient({ request }, 'https://dashboard.example');
    await expect(client.createSession('https://attacker.example/session')).rejects.toThrow('invalid archive component URL');
    await expect(client.createSession(`/dashboard/archive/components/${componentID}/session`)).rejects.toThrow('invalid archive component session');
  });
});
