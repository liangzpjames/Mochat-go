import { describe, expect, it } from 'vitest';

import { createAuthStore } from './auth-store';
import { type Session, type StorageAdapter } from './session';

function memoryStorage(): StorageAdapter & { values: Map<string, string> } {
  const values = new Map<string, string>();
  return {
    values,
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  };
}

describe('Dashboard auth store', () => {
  it('persists only the Dashboard realm session fields', () => {
    const storage = memoryStorage();
    const session: Session = { token: 'Bearer dashboard', userId: '7', userName: null, expiresAt: null };

    createAuthStore(storage).setSession(session);

    expect([...storage.values.keys()]).toEqual(expect.arrayContaining([
      'mochat_dashboard_token',
      'mochat_dashboard_user_id',
      'mochat_dashboard_user_name',
      'mochat_dashboard_expires_at',
    ]));
    expect([...storage.values.keys()]).not.toContain('mochat_dashboard_corp_id');
  });
});
