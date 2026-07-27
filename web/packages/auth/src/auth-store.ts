import {
  SESSION_STORAGE_KEYS,
  browserStorageAdapter,
  type Session,
  type StorageAdapter,
} from './session';

function parseValue(storage: StorageAdapter, key: string): unknown {
  const value = storage.getItem(key);
  return value === null ? undefined : JSON.parse(value);
}

export function createAuthStore(storage: StorageAdapter = browserStorageAdapter): {
  clearSession(): void;
  getSession(): Session | null;
  setSession(session: Session): void;
} {
  const clearSession = () => {
    storage.removeItem(SESSION_STORAGE_KEYS.token);
    storage.removeItem(SESSION_STORAGE_KEYS.userId);
    storage.removeItem(SESSION_STORAGE_KEYS.corpId);
    storage.removeItem(SESSION_STORAGE_KEYS.expiresAt);
  };

  return {
    clearSession,
    getSession() {
      try {
        const token = parseValue(storage, SESSION_STORAGE_KEYS.token);
        const userId = parseValue(storage, SESSION_STORAGE_KEYS.userId);
        const corpId = parseValue(storage, SESSION_STORAGE_KEYS.corpId);
        const expiresAt = parseValue(storage, SESSION_STORAGE_KEYS.expiresAt);
        if (
          typeof token !== 'string'
          || typeof userId !== 'string'
          || (corpId !== null && typeof corpId !== 'string')
          || (expiresAt !== null && typeof expiresAt !== 'number')
        ) {
          clearSession();
          return null;
        }
        return { token, userId, corpId, expiresAt };
      } catch {
        clearSession();
        return null;
      }
    },
    setSession(value) {
      storage.setItem(SESSION_STORAGE_KEYS.token, JSON.stringify(value.token));
      storage.setItem(SESSION_STORAGE_KEYS.userId, JSON.stringify(value.userId));
      storage.setItem(SESSION_STORAGE_KEYS.corpId, JSON.stringify(value.corpId));
      storage.setItem(SESSION_STORAGE_KEYS.expiresAt, JSON.stringify(value.expiresAt));
    },
  };
}
