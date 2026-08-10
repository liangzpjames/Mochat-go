import { describe, expect, it, vi } from 'vitest';

import { loadAccessProfile } from './access-api';

describe('loadAccessProfile', () => {
  it('returns the complete server profile without flattening sources or scope', async () => {
    const profile = {
      userId: 7,
      tenantId: 1,
      corpId: 3,
      isSuperAdmin: false,
      catalog: [{ code: 'contacts', path: '/chat/v2-all', scopeRequired: true }],
      effectivePermissions: [{ code: 'contacts', path: '/chat/v2-all', scope: 'department', sources: [{ type: 'role', id: 2, name: '销售', scope: 'department' }] }],
    };
    const request = vi.fn(() => Promise.resolve(profile));
    await expect(loadAccessProfile({ request })).resolves.toEqual(profile);
    expect(request).toHaveBeenCalledWith('/access/profile');
  });
});
