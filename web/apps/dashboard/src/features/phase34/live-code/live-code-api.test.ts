import { describe, expect, it } from 'vitest';

import { paginationFrom, recordsFrom, stateText, statisticsFrom } from './live-code-api';

describe('live code API payload adapters', () => {
  it('unwraps dashboard envelopes and preserves pagination', () => {
    const payload = { code: 200, data: { page: { total: 21, totalPage: 2, perPage: 20 }, list: [{ id: 1, name: '官网咨询' }] } };
    expect(recordsFrom(payload)).toEqual([{ id: 1, name: '官网咨询' }]);
    expect(paginationFrom(payload, 1)).toEqual({ total: 21, totalPage: 2, perPage: 20 });
  });

  it('marks missing statistics as unavailable instead of inventing values', () => {
    const page = statisticsFrom({ data: { summary: { available: false, codeCount: 1 }, list: [{ id: 9, name: '展会引流' }] } });
    expect(page.summary.available).toBe(false);
    expect(page.summary.codeCount).toBe(1);
    expect(page.rows[0]?.name).toBe('展会引流');
  });

  it('uses explicit Chinese labels for lifecycle states', () => {
    expect(stateText('active')).toBe('运行中');
    expect(stateText('expired')).toBe('已过期');
    expect(stateText('future')).toBe('future');
  });
});
