import { describe, expect, it, vi } from 'vitest';

import { operationAuthHref } from './operation-session';

describe('Operation activity session', () => {
  it('builds the matching activity OAuth URL with its required ID and safe target', () => {
    expect(operationAuthHref(
      'workFission',
      '/workFission?union_id=union-1&fission_id=9#tasks',
      { fissionId: 9 },
    )).toBe(
      '/auth/workFission?id=9&target=%2FworkFission%3Funion_id%3Dunion-1%26fission_id%3D9%23tasks',
    );
  });

  it('rejects invalid activity IDs and external targets without reading another app session', () => {
    const originalCookie = Object.getOwnPropertyDescriptor(Document.prototype, 'cookie');
    let cookieReads = 0;
    Object.defineProperty(document, 'cookie', {
      configurable: true,
      get: () => {
        cookieReads += 1;
        return 'token=sidebar-token';
      },
    });
    const storageSpy = vi.spyOn(Storage.prototype, 'getItem');

    expect(() => operationAuthHref('workFission', '/', { fissionId: 0 })).toThrow(
      '活动 ID 必须为正整数。',
    );
    expect(operationAuthHref('workFission', 'https://evil.example/steal', { fissionId: 9 }))
      .toBe('/auth/workFission?id=9&target=%2F');
    expect(cookieReads).toBe(0);
    expect(storageSpy).not.toHaveBeenCalled();

    storageSpy.mockRestore();
    if (originalCookie === undefined) {
      Reflect.deleteProperty(document, 'cookie');
    } else {
      Object.defineProperty(document, 'cookie', originalCookie);
    }
  });
});
