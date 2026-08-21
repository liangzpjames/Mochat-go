import { describe, expect, it, vi } from 'vitest';

import { createSensitiveWordApi } from './sensitive-word-api';

describe('SensitiveWordApi', () => {
  it('serializes list filters and normalizes the page response', async () => {
    const request = vi.fn().mockResolvedValue({
      page: { perPage: 10, total: 1, totalPage: 1 },
      list: [{ sensitiveWordId: 11, groupId: 2, groupName: '默认', name: '报价', status: 1, version: 'v1' }],
    });
    const api = createSensitiveWordApi({ request });

    await expect(api.list({ groupId: 2, keywords: '报价', page: 2, perPage: 10 })).resolves.toEqual({
      items: [{ id: 11, groupId: 2, groupName: '默认', name: '报价', status: 1, version: 'v1', employeeHitCount: 0, customerHitCount: 0, createdAt: '' }],
      total: 1,
      page: 2,
      perPage: 20,
    });
    expect(request).toHaveBeenCalledWith('/sensitiveWord/index?groupId=2&keyWords=%E6%8A%A5%E4%BB%B7&page=2&perPage=20');
  });

  it('sends versioned and idempotent payloads for every mutation', async () => {
	const request = vi.fn().mockResolvedValue({ version: 'v2', idempotent: false });
	const api = createSensitiveWordApi({ request });

	await api.create({ groupId: 2, names: ['报价', '合同'], version: '0', idempotencyKey: 'sensitive-1' });
	await api.setEnabled({ id: 11, enabled: false, version: 'v1', idempotencyKey: 'sensitive-2' });
	await api.move({ id: 11, groupId: 3, version: 'v1', idempotencyKey: 'sensitive-3' });
	await api.remove({ id: 11, version: 'v1', idempotencyKey: 'sensitive-4', confirmed: true });
	await api.createGroup({ names: ['财务'], version: '0', idempotencyKey: 'sensitive-5' });
	await api.renameGroup({ id: 3, name: '高风险', version: 'g1', idempotencyKey: 'sensitive-6' });

	expect(request).toHaveBeenNthCalledWith(1, '/sensitiveWord/store', expect.objectContaining({
	  method: 'POST',
	  body: JSON.stringify({ groupId: 2, name: '报价,合同', version: '0', idempotencyKey: 'sensitive-1' }),
	}));
	expect(request).toHaveBeenNthCalledWith(2, '/sensitiveWord/statusUpdate', expect.objectContaining({ body: JSON.stringify({ sensitiveWordId: 11, status: 2, version: 'v1', idempotencyKey: 'sensitive-2' }) }));
	expect(request).toHaveBeenNthCalledWith(3, '/sensitiveWord/move', expect.objectContaining({ body: JSON.stringify({ sensitiveWordId: 11, groupId: 3, version: 'v1', idempotencyKey: 'sensitive-3' }) }));
	expect(request).toHaveBeenNthCalledWith(4, '/sensitiveWord/destroy', expect.objectContaining({ body: JSON.stringify({ sensitiveWordId: 11, version: 'v1', idempotencyKey: 'sensitive-4', confirmed: true }) }));
	expect(request).toHaveBeenNthCalledWith(5, '/sensitiveWordGroup/store', expect.objectContaining({ body: JSON.stringify({ name: '财务', version: '0', idempotencyKey: 'sensitive-5' }) }));
	expect(request).toHaveBeenNthCalledWith(6, '/sensitiveWordGroup/update', expect.objectContaining({ body: JSON.stringify({ groupId: 3, name: '高风险', version: 'g1', idempotencyKey: 'sensitive-6' }) }));
  });

  it('serializes record filters and normalizes monitor rows', async () => {
	const request = vi.fn().mockResolvedValue({
	  page: { perPage: 20, total: 1, totalPage: 1 },
	  list: [{ sensitiveWordsMonitorId: 9, sensitiveWordName: '报价', source: 2, sourceText: '员工', triggerName: '小王', triggerScenario: '客户群', triggerTime: '2026-07-01 10:00:00' }],
	});
	const api = createSensitiveWordApi({ request });

	await expect(api.matches({ employeeIds: [3, 5], workRoomId: 9, groupId: 2, triggerStart: '2026-07-01 00:00:00', triggerEnd: '2026-07-02 00:00:00', page: 2, perPage: 20 })).resolves.toEqual({
	  items: [{ id: 9, sensitiveWordName: '报价', sensitiveWordID: 0, source: 2, sourceText: '员工', triggerName: '小王', triggerScenario: '客户群', triggerTime: '2026-07-01 10:00:00', contentPreview: '', workRoomID: 0 }],
	  total: 1,
	  page: 2,
	  perPage: 20,
	});
	expect(request).toHaveBeenCalledWith('/sensitiveWordsMonitor/index?employeeId=3%2C5&workRoomId=9&intelligentGroupId=2&triggerStart=2026-07-01+00%3A00%3A00&triggerEnd=2026-07-02+00%3A00%3A00&page=2&perPage=20');
  });

  it('preserves real word statistics and uses a fixed page size', async () => {
    const request = vi.fn().mockResolvedValue({
      page: { perPage: 20, total: 1, totalPage: 1 },
      list: [{ sensitiveWordId: 11, groupId: 2, groupName: '默认', name: '报价', status: 1, version: 'v1', employeeNum: 4, contactNum: 9, createdAt: '2026-08-21 10:00:00' }],
    });
    const api = createSensitiveWordApi({ request });
    await expect(api.list({ groupId: 2, keywords: '', status: 1, page: 1, perPage: 20 })).resolves.toEqual(expect.objectContaining({
      items: [expect.objectContaining({ employeeHitCount: 4, customerHitCount: 9, createdAt: '2026-08-21 10:00:00' })], perPage: 20,
    }));
    expect(request).toHaveBeenCalledWith('/sensitiveWord/index?groupId=2&keyWords=&status=1&page=1&perPage=20');
  });

  it('reads monitor scanner status from a dedicated endpoint', async () => {
    const request = vi.fn().mockResolvedValue({ enabled: false, state: 'disabled', lastError: '' });
    const api = createSensitiveWordApi({ request });
    await expect(api.monitorStatus()).resolves.toEqual(expect.objectContaining({ enabled: false, state: 'disabled' }));
    expect(request).toHaveBeenCalledWith('/sensitiveWordsMonitor/status');
  });

  it('normalizes monitor detail messages into readable fields', async () => {
    const request = vi.fn().mockResolvedValue([{ sender: '小王', msgType: 1, sendTime: '2026-07-01 10:00:00', isTrigger: 1, msgContent: { content: '报价不可外发' } }]);
    const api = createSensitiveWordApi({ request });
    await expect(api.matchDetail(9)).resolves.toEqual([{ sender: '小王', messageType: '文本', sendTime: '2026-07-01 10:00:00', isTrigger: true, content: '报价不可外发' }]);
    expect(request).toHaveBeenCalledWith('/sensitiveWordsMonitor/show?id=9');
  });
});
