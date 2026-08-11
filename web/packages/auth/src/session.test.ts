import { describe, expect, it, vi } from 'vitest';

import { createAuthStore } from './auth-store';
import { SESSION_STORAGE_KEYS, type Session, type StorageAdapter } from './session';

function createMemoryStorage(initial: Record<string, string> = {}): StorageAdapter & {
  values: Map<string, string>;
} {
  const values = new Map(Object.entries(initial));
  return {
    values,
    getItem: vi.fn((key: string) => values.get(key) ?? null),
    setItem: vi.fn((key: string, value: string) => {
      values.set(key, value);
    }),
    removeItem: vi.fn((key: string) => {
      values.delete(key);
    }),
  };
}

const session: Session = {
  token: 'Bearer legacy-token',
  userId: '7',
  userName: '管理员',
  expiresAt: 1_800_000_000_000,
};

describe('createAuthStore', () => {
  it('uses the audited legacy Dashboard token key', () => {
    expect(SESSION_STORAGE_KEYS.token).toBe('mochat_dashboard_token');
    expect('corpId' in SESSION_STORAGE_KEYS).toBe(false);
    expect(Object.values(SESSION_STORAGE_KEYS)).not.toContain('mochat_dashboard_corp_id');
  });

  it('writes and reads a session through the supplied storage adapter', () => {
    const storage = createMemoryStorage();
    const store = createAuthStore(storage);

    store.setSession(session);

    expect(store.getSession()).toEqual(session);
    expect(storage.setItem).toHaveBeenCalledTimes(4);
    expect(storage.getItem).toHaveBeenCalledTimes(4);
  });

  it('clears every session field from the supplied adapter', () => {
    const storage = createMemoryStorage();
    const store = createAuthStore(storage);
    store.setSession(session);

    store.clearSession();

    expect([...storage.values]).toEqual([]);
    expect(storage.removeItem).toHaveBeenCalledTimes(4);
    expect(storage.removeItem).toHaveBeenCalledWith(SESSION_STORAGE_KEYS.token);
    expect(storage.removeItem).toHaveBeenCalledWith(SESSION_STORAGE_KEYS.userId);
    expect(storage.removeItem).toHaveBeenCalledWith(SESSION_STORAGE_KEYS.userName);
    expect(storage.removeItem).toHaveBeenCalledWith(SESSION_STORAGE_KEYS.expiresAt);
  });

  it('returns null and clears storage when a session field contains corrupt JSON', () => {
    const storage = createMemoryStorage({
      [SESSION_STORAGE_KEYS.token]: '{not-json',
      [SESSION_STORAGE_KEYS.userId]: JSON.stringify('7'),
      [SESSION_STORAGE_KEYS.userName]: JSON.stringify('管理员'),
      [SESSION_STORAGE_KEYS.expiresAt]: JSON.stringify(null),
    });
    const store = createAuthStore(storage);

    expect(store.getSession()).toBeNull();
    expect([...storage.values]).toEqual([]);
    expect(storage.removeItem).toHaveBeenCalledTimes(4);
  });
});
