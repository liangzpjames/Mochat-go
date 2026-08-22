import { describe, expect, it } from 'vitest';

import { parseLegacyCodeAuth } from './code-auth';

function payload(data: unknown, code = 200, msg = ''): string {
  return btoa(JSON.stringify({ code, msg, data }));
}

describe('legacy code auth recovery', () => {
  it('maps legacy actions to a Sidebar OAuth target without accepting its token', () => {
    const result = parseLegacyCodeAuth(new URLSearchParams({
      callValues: payload({ agentId: 7, act: 'mediumGroup', token: 'dashboard-token' }),
    }));

    expect(result).toEqual({ ok: true, agentId: '7', target: '/medium' });
    expect(JSON.stringify(result)).not.toContain('dashboard-token');
  });

  it.each([
    ['customer', '/contact'],
    ['contact', '/contact'],
    ['contactSop', '/contactSop'],
    ['roomSop', '/roomSop'],
    ['contactBatchAdd', '/contactBatchAdd'],
  ])('maps %s to %s and keeps a positive id only for SOP/batch pages', (act, target) => {
    const result = parseLegacyCodeAuth(new URLSearchParams({
      callValues: payload({ agentId: '9', act, id: 55, batchId: 77 }),
    }));
    expect(result).toEqual({
      ok: true,
      agentId: '9',
      target: act === 'contactSop' || act === 'roomSop'
        ? `${target}?id=55`
        : act === 'contactBatchAdd' ? `${target}?batchId=77` : target,
    });
  });

  it.each([
    new URLSearchParams(),
    new URLSearchParams({ callValues: 'not-base64' }),
    new URLSearchParams({ callValues: payload({ act: 'contact' }) }),
    new URLSearchParams({ callValues: payload({ agentId: 7, act: 'unknown' }) }),
    new URLSearchParams({ callValues: payload({ agentId: 7, act: 'contact' }, 500, 'failed') }),
  ])('fails closed for invalid callback data', (params) => {
    expect(parseLegacyCodeAuth(params)).toMatchObject({ ok: false });
  });
});
