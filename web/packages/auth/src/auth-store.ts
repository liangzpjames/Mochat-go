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
    storage.removeItem(SESSION_STORAGE_KEYS.userName);
    storage.removeItem(SESSION_STORAGE_KEYS.expiresAt);
  };

  return {
    clearSession,
    getSession() {
      try {
        const token = parseValue(storage, SESSION_STORAGE_KEYS.token);
        const userId = parseValue(storage, SESSION_STORAGE_KEYS.userId);
        const userName = parseValue(storage, SESSION_STORAGE_KEYS.userName);
        const expiresAt = parseValue(storage, SESSION_STORAGE_KEYS.expiresAt);
        if (
          typeof token !== 'string'
          || typeof userId !== 'string'
          || (expiresAt !== null && typeof expiresAt !== 'number')
          || (userName !== undefined && userName !== null && typeof userName !== 'string')
        ) {
          clearSession();
          return null;
        }
        return { token, userId, expiresAt, userName: typeof userName === 'string' ? userName : null };
      } catch {
        clearSession();
        return null;
      }
    },
    setSession(value) {
      storage.setItem(SESSION_STORAGE_KEYS.token, JSON.stringify(value.token));
      storage.setItem(SESSION_STORAGE_KEYS.userId, JSON.stringify(value.userId));
      storage.setItem(SESSION_STORAGE_KEYS.userName, JSON.stringify(value.userName ?? null));
      storage.setItem(SESSION_STORAGE_KEYS.expiresAt, JSON.stringify(value.expiresAt));
    },
  };
}
