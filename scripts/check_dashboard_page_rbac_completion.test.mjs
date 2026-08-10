import test from 'node:test';
import assert from 'node:assert/strict';
import { validateCompletionFacts } from './check_dashboard_page_rbac_completion.mjs';

const base = {
  catalogOutput: '53 pages, 49 ordinary, 4 superadmin_only, 0 unmapped dashboard API usages',
  sourceCorpus: 'loadAccessProfile();',
  e2eSource: 'const MOCHAT_E2E_LIVE_BASE=""; const MOCHAT_E2E_RBAC_FIXTURE_JSON=""; const liveFixture={ exactAllowedRoutes: [], expectedSources: [], forbiddenCodes: [], forbiddenRoleIds: [] }; const directCode=""; const roleUnionCode=""; const disabledRoleCode=""; const noPermission={}; const tenantDenied={}; const mochat_dashboard_token=""; const routes=[]; const ordinaryRoutes=[]; for (const route of routes) {} for (const route of ordinaryRoutes) {}; const ordinary=49; const protectedPages=4; const allPages=53; viewport=390; TENANT_ACCESS_DENIED; DASHBOARD_PERMISSION_DENIED; profileResponse.status()).toBe(403); .phase35-page-shell; document.documentElement.scrollWidth; page.on("response", fn);',
  smokeSource: 'docker compose ps; docker volume ls; Invoke-RestMethod /dashboard/user/auth; headers.Authorization = Bearer token; TargetUserId CrossTenantUserId MutationJson sh -lc COUNT(*)',
  packageJSON: { scripts: { 'check:phase4-dashboard-page-rbac': 'node scripts/check_dashboard_page_rbac_completion.mjs' } },
};

test('completion gate accepts complete matrix facts', () => {
  assert.deepEqual(validateCompletionFacts(base), { pages: 53, ordinary: 49, superadminOnly: 4 });
});

test('completion gate rejects legacy benchmark authorization facts', () => {
  assert.throws(() => validateCompletionFacts({ ...base, sourceCorpus: 'benchmarkRoutes = []' }), /legacy authorization/);
});

test('completion gate rejects destructive smoke scripts', () => {
  assert.throws(() => validateCompletionFacts({ ...base, smokeSource: 'docker volume rm test' }), /destructive/);
});

test('completion gate rejects missing machine-code or 390 matrix coverage', () => {
  assert.throws(() => validateCompletionFacts({ ...base, e2eSource: '53 pages only' }), /Playwright matrix/);
});

test('completion gate rejects structural regressions, not just matching strings', () => {
  for (const sourceCorpus of ['ordinary company-setting/staff fallback: allow', 'handler uses DataPermission']) {
    assert.throws(() => validateCompletionFacts({ ...base, sourceCorpus }), /ordinary management|DataPermission/);
  }
  assert.throws(() => validateCompletionFacts({ ...base, catalogOutput: '52 pages, 48 ordinary, 4 superadmin_only, 0 unmapped dashboard API usages' }), /catalog gate/);
  assert.throws(() => validateCompletionFacts({ ...base, frontendSource: '/dashboard/access/profile' }), /frontend API/);
});

test('RED: missing backend handler evidence fails', () => {
  assert.throws(() => validateCompletionFacts({ ...base, backendEvidence: 'GET /dashboard/access/users' }), /handler evidence/);
});

test('RED: scope mapping without handler/guard/consumer source fails', () => {
  assert.throws(() => validateCompletionFacts({ ...base, scopeMappings: 'GET /dashboard/reports/overview -> DashboardAccessContext' }), /scope mapping/);
});

test('RED: scopeRequired route cannot be mislabeled tenant-only', () => {
  assert.throws(() => validateCompletionFacts({ ...base, scopeMappings: 'GET /dashboard/workEmployee/index -> handler GET /dashboard/workEmployee/index (server.go:1) -> guard internal/dashboard/dashboard_access_guard.go:1 -> tenant-only config' }), /scope mapping/);
});

test('RED: ordinary management and fallback allow fail independently', () => {
  assert.throws(() => validateCompletionFacts({ ...base, sourceCorpus: 'ordinary company-setting/staff fallback: allow' }), /ordinary management/);
});
