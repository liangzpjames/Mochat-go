export type Session = {
  token: string;
  userId: string;
  corpId: string | null;
  expiresAt: number | null;
};

export interface StorageAdapter {
  getItem: (key: string) => string | null;
  setItem: (key: string, value: string) => void;
  removeItem: (key: string) => void;
}

export const SESSION_STORAGE_KEYS = {
  token: 'ACCESS_TOKEN',
  userId: 'userId',
  corpId: 'corpId',
  expiresAt: 'expiresAt',
} as const;

export const browserStorageAdapter: StorageAdapter = {
  getItem(key) {
    return window.localStorage.getItem(key);
  },
  setItem(key, value) {
    window.localStorage.setItem(key, value);
  },
  removeItem(key) {
    window.localStorage.removeItem(key);
  },
};
