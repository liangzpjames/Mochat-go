import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const REQUIRED_ACCOUNT_NAMES = [
  'saasAdmin',
  'dashboardSuperAdmin',
  'dashboardOrdinary',
  'tenantDenied',
  'secondTenantAdmin',
];

const ENV_KEY_PATTERN = /^[A-Z][A-Z0-9_]{1,127}$/;
const ACCOUNT_KEYS = new Set(['loginEnvKey', 'passwordEnvKey']);
const TOP_LEVEL_KEYS = new Set([
  ...REQUIRED_ACCOUNT_NAMES,
  'expectedTenantId',
  'expectedCorpId',
  'expectedWxCorpId',
  // Optional, non-credential metadata that lets a release fixture prove the
  // database snapshot contains exactly one tenant-corp binding.
  'tenantCorpBindings',
]);
const SENSITIVE_KEY_PATTERN = /(?:^|_)(?:password|jwt|secret|token|phone|authorization|cookie)(?:$|_)/i;
const JWT_PATTERN = /(?:^Bearer\s+)?eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/;

function fail(message) {
  throw new Error(`identity single-corp fixture invalid: ${message}`);
}

function isRecord(value) {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function assertNoCredentialMaterial(value, location = 'fixture') {
  if (Array.isArray(value)) {
    value.forEach((item, index) => assertNoCredentialMaterial(item, `${location}[${index}]`));
    return;
  }
  if (!isRecord(value)) {
    if (typeof value === 'string' && JWT_PATTERN.test(value)) fail(`${location} contains a raw JWT`);
    return;
  }
  for (const [key, child] of Object.entries(value)) {
    const childLocation = `${location}.${key}`;
    const isEnvironmentReference = key === 'loginEnvKey' || key === 'passwordEnvKey';
    if (SENSITIVE_KEY_PATTERN.test(key) && !isEnvironmentReference) {
      fail(`${childLocation} is a raw credential field; use an environment-variable key reference`);
    }
    if (typeof child === 'string' && JWT_PATTERN.test(child)) fail(`${childLocation} contains a raw JWT`);
    assertNoCredentialMaterial(child, childLocation);
  }
}

function validateEnvironmentKey(value, location) {
  if (typeof value !== 'string' || !ENV_KEY_PATTERN.test(value)) {
    fail(`${location} must be an environment-variable key, not a credential value`);
  }
  if (/(?:LOGIN|USERNAME|PASSWORD|SECRET|JWT|TOKEN|PHONE)$/i.test(value) === false) {
    fail(`${location} must identify a credential environment variable`);
  }
  return value;
}

function validateCredentialReference(value, accountName) {
  if (!isRecord(value)) fail(`${accountName} is required as a credential environment-variable reference`);
  const unknownKeys = Object.keys(value).filter((key) => !ACCOUNT_KEYS.has(key));
  if (unknownKeys.length > 0) fail(`${accountName} contains unsupported fields: ${unknownKeys.join(', ')}`);
  const loginEnvKey = validateEnvironmentKey(value.loginEnvKey, `${accountName}.loginEnvKey`);
  const passwordEnvKey = validateEnvironmentKey(value.passwordEnvKey, `${accountName}.passwordEnvKey`);
  if (loginEnvKey === passwordEnvKey) fail(`${accountName} login and password environment keys must differ`);
  return { loginEnvKey, passwordEnvKey };
}

function validateBindingSnapshot(value, expectedTenantId, expectedCorpId) {
  if (value === undefined) return;
  if (!Array.isArray(value) || value.length !== 1) {
    fail('tenantCorpBindings must contain exactly one unique binding');
  }
  const seenTenants = new Set();
  const seenCorps = new Set();
  const binding = value[0];
  if (!isRecord(binding)) fail('tenantCorpBindings entries must be objects');
  const unknownKeys = Object.keys(binding).filter((key) => !new Set(['tenantId', 'corpId', 'status']).has(key));
  if (unknownKeys.length > 0) fail(`tenantCorpBindings contains unsupported fields: ${unknownKeys.join(', ')}`);
  if (!Number.isInteger(binding.tenantId) || binding.tenantId <= 0) fail('tenantCorpBindings.tenantId must be positive');
  if (!Number.isInteger(binding.corpId) || binding.corpId <= 0) fail('tenantCorpBindings.corpId must be positive');
  if (binding.status !== undefined && !['pending', 'verified', 'suspended'].includes(binding.status)) {
    fail('tenantCorpBindings.status is invalid');
  }
  seenTenants.add(binding.tenantId);
  seenCorps.add(binding.corpId);
  if (seenTenants.size !== value.length || seenCorps.size !== value.length) fail('tenant-corp bindings must be unique');
  if (binding.tenantId !== expectedTenantId || binding.corpId !== expectedCorpId) {
    fail('tenant-corp binding must match expected tenant and corp');
  }
}

export function validateFixture(fixture) {
  if (!isRecord(fixture)) fail('fixture must be a JSON object');
  assertNoCredentialMaterial(fixture);
  const unknownKeys = Object.keys(fixture).filter((key) => !TOP_LEVEL_KEYS.has(key));
  if (unknownKeys.length > 0) fail(`unsupported top-level fields: ${unknownKeys.join(', ')}`);

  const references = new Map();
  const loginEnvironmentKeys = new Map();
  const passwordEnvironmentKeys = new Map();
  for (const accountName of REQUIRED_ACCOUNT_NAMES) {
    if (!(accountName in fixture)) fail(`${accountName} is required`);
    const reference = validateCredentialReference(fixture[accountName], accountName);
    const fingerprint = `${reference.loginEnvKey}\u0000${reference.passwordEnvKey}`;
    if (references.has(fingerprint)) fail(`credential references must be unique: ${accountName} duplicates ${references.get(fingerprint)}`);
    if (loginEnvironmentKeys.has(reference.loginEnvKey)) {
      fail(`login environment key must be unique: ${accountName} duplicates ${loginEnvironmentKeys.get(reference.loginEnvKey)}`);
    }
    if (passwordEnvironmentKeys.has(reference.passwordEnvKey)) {
      fail(`password environment key must be unique: ${accountName} duplicates ${passwordEnvironmentKeys.get(reference.passwordEnvKey)}`);
    }
    references.set(fingerprint, accountName);
    loginEnvironmentKeys.set(reference.loginEnvKey, accountName);
    passwordEnvironmentKeys.set(reference.passwordEnvKey, accountName);
  }

  if (!Number.isInteger(fixture.expectedTenantId) || fixture.expectedTenantId <= 0) fail('expectedTenantId must be a positive integer');
  if (!Number.isInteger(fixture.expectedCorpId) || fixture.expectedCorpId <= 0) fail('expectedCorpId must be a positive integer');
  if (typeof fixture.expectedWxCorpId !== 'string' || fixture.expectedWxCorpId.trim() === '') fail('expectedWxCorpId is required');
  validateBindingSnapshot(fixture.tenantCorpBindings, fixture.expectedTenantId, fixture.expectedCorpId);

  return {
    accounts: REQUIRED_ACCOUNT_NAMES.length,
    expectedTenantId: fixture.expectedTenantId,
    expectedCorpId: fixture.expectedCorpId,
    expectedWxCorpId: fixture.expectedWxCorpId,
  };
}

export function resolveFixtureCredentials(reference, environment = process.env) {
  const normalized = validateCredentialReference(reference, 'credential');
  const login = environment[normalized.loginEnvKey];
  const password = environment[normalized.passwordEnvKey];
  if (typeof login !== 'string' || login.trim() === '') fail(`${normalized.loginEnvKey} is not set`);
  if (typeof password !== 'string' || password === '') fail(`${normalized.passwordEnvKey} is not set`);
  return { login, password };
}

export function loadFixture(filePath) {
  const absolutePath = path.resolve(filePath);
  return JSON.parse(fs.readFileSync(absolutePath, 'utf8'));
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  const fixturePath = process.argv[2];
  if (!fixturePath) throw new Error('usage: node scripts/validate_identity_single_corp_e2e_fixture.mjs <fixture.json>');
  const result = validateFixture(loadFixture(fixturePath));
  console.log(`identity single-corp E2E fixture valid: ${result.accounts} accounts, tenant=${result.expectedTenantId}, corp=${result.expectedCorpId}, wxCorpId=${result.expectedWxCorpId}`);
}
