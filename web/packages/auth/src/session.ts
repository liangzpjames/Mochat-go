export type Session = {
  token: string;
  userId: string;
  userName?: string | null;
  expiresAt: number | null;
};

export interface StorageAdapter {
  getItem: (key: string) => string | null;
  setItem: (key: string, value: string) => void;
  removeItem: (key: string) => void;
}

export const SESSION_STORAGE_KEYS = {
  token: 'mochat_dashboard_token',
  userId: 'mochat_dashboard_user_id',
  userName: 'mochat_dashboard_user_name',
  expiresAt: 'mochat_dashboard_expires_at',
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
