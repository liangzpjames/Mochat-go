import { describe, expect, it, vi } from 'vitest';

import { createContactFieldApi } from './contact-field-api';

describe('contact field api', () => {
  it('uses the audited list query', async () => {
    const request = vi.fn(() => Promise.resolve({ list: [], page: {} }));
    const api = createContactFieldApi({ request });
    await api.list({ status: 2, page: 3, perPage: 20 });
    expect(request).toHaveBeenCalledWith('/contactField/index?status=2&page=3&perPage=20');
  });

  it('uses the audited write contracts', async () => {
    const request = vi.fn(() => Promise.resolve([]));
    const api = createContactFieldApi({ request });
    const field = { label: '城市', type: 1, options: ['上海'], order: 8, status: 1 };
    await api.create(field);
    expect(request).toHaveBeenLastCalledWith('/contactField/store', expect.objectContaining({
      method: 'POST', body: JSON.stringify(field),
    }));
    await api.update({ id: 2, ...field });
    expect(request).toHaveBeenLastCalledWith('/contactField/update', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ id: 2, ...field }),
    }));
    await api.updateStatus(2, 0);
    expect(request).toHaveBeenLastCalledWith('/contactField/statusUpdate', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ id: 2, status: 0 }),
    }));
    await api.remove(2);
    expect(request).toHaveBeenLastCalledWith('/contactField/destroy', expect.objectContaining({
      method: 'DELETE', body: JSON.stringify({ id: 2 }),
    }));
    await api.batchUpdate({ update: [{ id: 2, ...field }], destroy: [4] });
    expect(request).toHaveBeenLastCalledWith('/contactField/batchUpdate', expect.objectContaining({
      method: 'PUT', body: JSON.stringify({ update: [{ id: 2, ...field }], destroy: [4] }),
    }));
  });
});
