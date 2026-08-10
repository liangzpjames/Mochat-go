import { describe, expect, it, vi } from 'vitest';

import { loadMenu } from './menu-api';

describe('loadMenu', () => {
  it('maps the audited permission tree fields', async () => {
    const request = vi.fn(() => Promise.resolve({
      effectivePermissions: [{
        code: 'dashboard.customer.contact',
        name: '客户管理',
        path: '/customer/contact',
        scope: 'department',
        sources: [],
      }],
    }));

    await expect(loadMenu({ request })).resolves.toEqual([{
      name: '客户管理',
      icon: null,
      linkUrl: '/customer/contact',
      linkType: 1,
      children: [],
    }]);
    expect(request).toHaveBeenCalledWith('/access/profile');
  });
});
