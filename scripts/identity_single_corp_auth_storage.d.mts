export type IdentityStorageRealm = 'dashboard' | 'saas';

export function parseStoredToken(rawValue: unknown, realm: IdentityStorageRealm): string;
