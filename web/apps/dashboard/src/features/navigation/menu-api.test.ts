import { describe, expect, it, vi } from 'vitest';

import { loadMenu } from './menu-api';

describe('loadMenu', () => {
  it('maps the audited permission tree fields', async () => {
    const request = vi.fn(() => Promise.resolve([{
      name: '客户管理',
      icon: '',
      linkUrl: '/workContact/index',
      linkType: 1,
      children: [],
    }]));

    await expect(loadMenu({ request })).resolves.toEqual([{
      name: '客户管理',
      icon: null,
      linkUrl: '/workContact/index',
      linkType: 1,
      children: [],
    }]);
    expect(request).toHaveBeenCalledWith('/role/permissionByUser');
  });
});
