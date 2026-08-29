import { describe, expect, it, vi } from 'vitest';

import { createBusinessWorkbenchApi } from './business-workbench-api';

describe('business workbench API', () => {
  it('encodes query values and sends JSON writes through the authenticated client', async () => {
    const request = vi.fn(() => Promise.resolve({ list: [] }));
    const api = createBusinessWorkbenchApi({ request });

    await api.read('/workContact/index', { keyword: '张三', page: 2, perPage: 20 });
    expect(request).toHaveBeenLastCalledWith(
      '/workContact/index?keyword=%E5%BC%A0%E4%B8%89&page=2&perPage=20',
    );

    await api.write('/greeting/store', { name: '欢迎语', content: '你好' });
    expect(request).toHaveBeenLastCalledWith('/greeting/store', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: '欢迎语', content: '你好' }),
    }));

    await api.write('/scrm/orders', { title: '续费' }, 'POST', { 'Idempotency-Key': 'intent-1' });
    expect(request).toHaveBeenLastCalledWith('/scrm/orders', expect.objectContaining({
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'intent-1' },
    }));
  });
});
