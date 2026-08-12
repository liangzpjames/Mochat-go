import assert from 'node:assert/strict';
import test from 'node:test';
import { parseStoredToken } from './identity_single_corp_auth_storage.mjs';

const jwt = 'eyJhbGciOiJIUzI1NiJ9.payload.signature';

test('parses the JSON-quoted Dashboard token and keeps the SaaS token raw', () => {
  assert.equal(parseStoredToken(JSON.stringify(jwt), 'dashboard'), jwt);
  assert.equal(parseStoredToken(jwt, 'saas'), jwt);
});

test('fails closed for an invalid Dashboard storage value or non-string token', () => {
  assert.throws(() => parseStoredToken(jwt, 'dashboard'), /JSON|Dashboard/i);
  assert.throws(() => parseStoredToken(JSON.stringify({ token: jwt }), 'dashboard'), /string/i);
  assert.throws(() => parseStoredToken(JSON.stringify(42), 'dashboard'), /string/i);
  assert.throws(() => parseStoredToken('', 'saas'), /empty|token/i);
  assert.throws(() => parseStoredToken('"' + jwt + '"', 'saas'), /raw|quoted|token/i);
});
