const STORAGE_REALMS = new Set(['dashboard', 'saas']);

export function parseStoredToken(rawValue, realm) {
  if (!STORAGE_REALMS.has(realm)) {
    throw new Error(`unsupported identity storage realm: ${realm}`);
  }
  if (typeof rawValue !== 'string' || rawValue.length === 0) {
    throw new Error(`${realm} token storage value must be a non-empty string`);
  }

  let token = rawValue;
  if (realm === 'dashboard') {
    try {
      token = JSON.parse(rawValue);
    } catch (error) {
      throw new Error(`Dashboard token storage is not valid JSON: ${error instanceof Error ? error.message : 'parse failed'}`);
    }
  }
  if (typeof token !== 'string' || token.trim() === '') {
    throw new Error(`${realm} token storage must decode to a non-empty string`);
  }
  if (realm === 'saas' && /^"[\s\S]*"$/.test(token.trim())) {
    throw new Error('SaaS token storage must remain a raw string, not a JSON-quoted token');
  }
  return token;
}
