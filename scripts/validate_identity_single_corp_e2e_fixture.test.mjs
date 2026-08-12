import assert from 'node:assert/strict';
import test from 'node:test';
import { resolveFixtureCredentials, validateFixture } from './validate_identity_single_corp_e2e_fixture.mjs';

const accountNames = [
  'saasAdmin',
  'dashboardSuperAdmin',
  'dashboardOrdinary',
  'tenantDenied',
  'secondTenantAdmin',
];

function reference(index) {
  return {
    loginEnvKey: `MOCHAT_IDENTITY_FIXTURE_${index}_LOGIN`,
    passwordEnvKey: `MOCHAT_IDENTITY_FIXTURE_${index}_PASSWORD`,
  };
}

function validFixture() {
  return {
    saasAdmin: reference(1),
    dashboardSuperAdmin: reference(2),
    dashboardOrdinary: reference(3),
    tenantDenied: reference(4),
    secondTenantAdmin: reference(5),
    expectedTenantId: 41,
    expectedCorpId: 701,
    expectedWxCorpId: 'ww_identity_single_corp',
    tenantCorpBindings: [{ tenantId: 41, corpId: 701, status: 'verified' }],
  };
}

test('validates the complete five-account fixture without reading credential values', () => {
  const result = validateFixture(validFixture());

  assert.deepEqual(result, {
    accounts: accountNames.length,
    expectedTenantId: 41,
    expectedCorpId: 701,
    expectedWxCorpId: 'ww_identity_single_corp',
  });
});

test('requires all five account references and positive single-corp identifiers', () => {
  const fixture = validFixture();
  delete fixture.dashboardOrdinary;
  assert.throws(() => validateFixture(fixture), /dashboardOrdinary.*required/i);

  const invalidIDs = validFixture();
  invalidIDs.expectedTenantId = 0;
  assert.throws(() => validateFixture(invalidIDs), /expectedTenantId.*positive/i);
});

test('rejects plaintext password, JWT, Secret, and phone fields anywhere in fixture JSON', () => {
  for (const [key, value] of [
    ['password', 'fixture-password'],
    ['jwt', 'eyJhbGciOiJIUzI1NiJ9.fixture.signature'],
    ['secret', 'fixture-secret'],
    ['phone', '13800000000'],
  ]) {
    const fixture = validFixture();
    fixture.dashboardSuperAdmin[key] = value;
    assert.throws(() => validateFixture(fixture), new RegExp(`${key}`, 'i'));
  }
});

test('rejects duplicate credential references and duplicate or mismatched tenant-corp bindings', () => {
  const duplicateCredential = validFixture();
  duplicateCredential.tenantDenied = duplicateCredential.dashboardOrdinary;
  assert.throws(() => validateFixture(duplicateCredential), /credential reference.*unique/i);

  const duplicateLoginKey = validFixture();
  duplicateLoginKey.tenantDenied = {
    ...duplicateLoginKey.tenantDenied,
    loginEnvKey: duplicateLoginKey.dashboardOrdinary.loginEnvKey,
  };
  assert.throws(() => validateFixture(duplicateLoginKey), /login environment key.*unique/i);

  const duplicateBinding = validFixture();
  duplicateBinding.tenantCorpBindings = [
    { tenantId: 41, corpId: 701, status: 'verified' },
    { tenantId: 41, corpId: 701, status: 'verified' },
  ];
  assert.throws(() => validateFixture(duplicateBinding), /unique.*binding|duplicate.*tenant.*corp/i);

  const mismatchedBinding = validFixture();
  mismatchedBinding.tenantCorpBindings = [{ tenantId: 41, corpId: 702, status: 'verified' }];
  assert.throws(() => validateFixture(mismatchedBinding), /match.*expected|expected.*corp/i);
});

test('does not accept token fields as a shortcut for proving distinct realms', () => {
  const fixture = validFixture();
  fixture.saasToken = 'not-a-real-token';
  fixture.dashboardToken = 'not-a-real-token';
  assert.throws(() => validateFixture(fixture), /token.*not permitted|token/i);
});

test('resolves only the referenced environment keys and never returns fixture JSON material', () => {
  const fixture = validFixture();
  const environment = Object.fromEntries(accountNames.flatMap((name, index) => {
    const ref = fixture[name];
    return [[ref.loginEnvKey, `login-${index}`], [ref.passwordEnvKey, `password-${index}`]];
  }));

  const resolved = resolveFixtureCredentials(fixture.dashboardSuperAdmin, environment);
  assert.deepEqual(resolved, { login: 'login-1', password: 'password-1' });
});
