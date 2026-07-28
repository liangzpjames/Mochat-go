import { describe, expect, it, vi } from 'vitest';

import { bindCorp, loadCorps } from './corp-api';

describe('corp API', () => {
  it('maps the audited corp selection response', async () => {
    const request = vi.fn(() => Promise.resolve([
      { corpId: 3, corpName: '迁移企业' },
    ]));

    await expect(loadCorps({ request })).resolves.toEqual([
      { id: '3', name: '迁移企业', authorized: true },
    ]);
    expect(request).toHaveBeenCalledWith('/corp/select');
  });

  it('binds a corp using the audited request body', async () => {
    const request = vi.fn(() => Promise.resolve([]));

    await bindCorp({ request }, '3');

    expect(request).toHaveBeenCalledWith('/corp/bind', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ corpId: 3 }),
    });
  });
});
