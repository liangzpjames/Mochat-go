import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  extractBackendRegisteredAPIs,
  extractFrontendAPIUsages,
  extractMigrationPermissionResourceMappings,
  validateDashboardPageRBACCatalog,
} from './check_dashboard_page_rbac_catalog.mjs';

const adminPaths = [
  '/company-setting/staff',
  '/setting/role',
  '/setting/additional',
  '/setting/authorization',
];

function fixture() {
  const pages = Array.from({ length: 53 }, (_, index) => ({
    path: index === 0 ? '/index' : `/page-${index}`,
    title: `页面 ${index}`,
    groupId: index === 0 ? null : 'group',
  }));
  adminPaths.forEach((path, index) => {
    pages[49 + index] = { path, title: `管理页 ${index}`, groupId: 'company-settings' };
  });
  const catalog = pages.map((page, index) => ({
    code: `dashboard.page.${index}`,
    path: page.path,
    name: page.title,
    groupCode: page.groupId,
    sort: index + 1,
    superadminOnly: adminPaths.includes(page.path),
    resources: index === 0
      ? [{ method: 'GET', pathPattern: '/dashboard/reports/overview', scopeRequired: true }]
      : [],
  }));
  return {
    manifest: { pages },
    catalog,
    apiUsages: ['GET /dashboard/reports/overview'],
    registeredAPIs: ['GET /dashboard/reports/overview'],
    exemptions: [],
  };
}

test('accepts exactly 53 pages, 49 ordinary pages and four protected management pages', () => {
  const result = validateDashboardPageRBACCatalog(fixture());
  assert.equal(result.pageCount, 53);
  assert.equal(result.ordinaryPageCount, 49);
  assert.equal(result.superadminOnlyCount, 4);
  assert.equal(result.resourceCount, 1);
});

test('rejects manifest and catalog page drift', () => {
  const input = fixture();
  input.catalog.pop();
  assert.throws(() => validateDashboardPageRBACCatalog(input), /catalog paths must exactly match manifest paths/);
});

test('rejects duplicate paths and resource registrations', () => {
  const duplicatePath = fixture();
  duplicatePath.catalog[1].path = duplicatePath.catalog[0].path;
  assert.throws(() => validateDashboardPageRBACCatalog(duplicatePath), /duplicate catalog path/);

  const duplicateResource = fixture();
  duplicateResource.catalog[0].resources.push({
    method: 'GET',
    pathPattern: '/dashboard/reports/overview',
    scopeRequired: true,
  });
  assert.throws(() => validateDashboardPageRBACCatalog(duplicateResource), /duplicate resource mapping/);
});

test('requires an explicit scope flag for every resource mapping', () => {
  const input = fixture();
  delete input.catalog[0].resources[0].scopeRequired;
  assert.throws(() => validateDashboardPageRBACCatalog(input), /resource scopeRequired must be boolean/);
});

test('allows one API resource to authorize more than one page permission', () => {
  const input = fixture();
  input.catalog[1].resources.push({
    method: 'GET',
    pathPattern: '/dashboard/reports/overview',
    scopeRequired: true,
  });
  assert.doesNotThrow(() => validateDashboardPageRBACCatalog(input));
});

test('rejects incorrect superadmin-only management page flags', () => {
  const input = fixture();
  input.catalog.find((page) => page.path === '/setting/role').superadminOnly = false;
  assert.throws(() => validateDashboardPageRBACCatalog(input), /superadmin_only paths must exactly match/);
});

test('rejects frontend dashboard api usage without a page mapping', () => {
  const input = fixture();
  input.apiUsages.push('POST /dashboard/scrm/orders');
  input.registeredAPIs.push('POST /dashboard/scrm/orders');
  assert.throws(() => validateDashboardPageRBACCatalog(input), /unmapped dashboard api usage: POST \/dashboard\/scrm\/orders/);
});

test('rejects mapped resources that are absent from registered server routes', () => {
  const input = fixture();
  input.registeredAPIs = [];
  assert.throws(() => validateDashboardPageRBACCatalog(input), /mapped dashboard api is not registered by server/);
});

test('only exact exemptions may omit a page mapping', () => {
  const input = fixture();
  input.apiUsages.push('POST /dashboard/user/auth');
  input.registeredAPIs.push('POST /dashboard/user/auth');
  input.exemptions.push('POST /dashboard/user/auth');
  assert.doesNotThrow(() => validateDashboardPageRBACCatalog(input));

  input.apiUsages.push('POST /dashboard/user/auth-shadow');
  input.registeredAPIs.push('POST /dashboard/user/auth-shadow');
  assert.throws(() => validateDashboardPageRBACCatalog(input), /unmapped dashboard api usage: POST \/dashboard\/user\/auth-shadow/);
});

test('extracts a newly added production frontend request as an independent usage fact', () => {
  const usages = extractFrontendAPIUsages([
    `export async function load(client) {
      return client.request('/scrm/new-resource', { method: 'POST' });
    }`,
  ]);
  const input = fixture();
  input.apiUsages = usages;
  input.registeredAPIs.push('POST /dashboard/scrm/new-resource');
  assert.throws(
    () => validateDashboardPageRBACCatalog(input),
    /unmapped dashboard api usage: POST \/dashboard\/scrm\/new-resource/,
  );
});

test('extracts config, variable and ternary phase endpoint variants with every possible method', () => {
  const usages = extractFrontendAPIUsages([
    `const configs = { loss: { readEndpoint: '/workContact/lossContact' } };
     const endpoint = tab === 'records' ? '/risk/records' : '/risk/rules';
     api.read(endpoint, {});
     api.write('/risk/rules', {}, editing ? 'PUT' : 'POST');`,
  ]);
  assert.deepEqual(usages, [
    'GET /dashboard/risk/records',
    'GET /dashboard/risk/rules',
    'GET /dashboard/workContact/lossContact',
    'POST /dashboard/risk/rules',
    'PUT /dashboard/risk/rules',
  ]);
});

test('extracts only reachable Go dispatch and registrar statements with source locations', () => {
  const routes = extractBackendRegisteredAPIs([
    {
      file: 'internal/server/server.go',
      body: `package server

// case r.URL.Path == "/dashboard/comment-only" && r.Method == http.MethodGet:
const unrelated = "GET /dashboard/unrelated-string"

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  switch {
  case r.URL.Path == "/dashboard/reports/overview" && r.Method == http.MethodGet:
    s.overview.ServeHTTP(w, r)
  }
}`,
    },
    {
      file: 'internal/modules/scrm/transport/http/routes.go',
      body: `package http

func Register(registrar Registrar, handler Handler) {
  registrar.Handle(nethttp.MethodPost, "/dashboard/scrm/new-resource", handler)
}`,
    },
  ]);

  assert.deepEqual(routes, [
    {
      contract: 'GET /dashboard/reports/overview',
      file: 'internal/server/server.go',
      line: 8,
    },
    {
      contract: 'POST /dashboard/scrm/new-resource',
      file: 'internal/modules/scrm/transport/http/routes.go',
      line: 4,
    },
  ]);
});

test('a newly registered backend route fails the gate and reports its source line', () => {
  const input = fixture();
  input.registeredAPIs.push('POST /dashboard/scrm/new-resource');
  input.registeredSources = new Map([
    ['POST /dashboard/scrm/new-resource', 'internal/modules/scrm/routes.go:17'],
  ]);
  assert.throws(
    () => validateDashboardPageRBACCatalog(input),
    /unmapped registered dashboard api: POST \/dashboard\/scrm\/new-resource \(internal\/modules\/scrm\/routes\.go:17\)/,
  );
});

test('deny-only routes are classified without granting a page permission', () => {
  const input = fixture();
  input.registeredAPIs.push('DELETE /dashboard/acceptance/phase35');
  input.denyOnly = ['DELETE /dashboard/acceptance/phase35'];
  assert.doesNotThrow(() => validateDashboardPageRBACCatalog(input));

  input.apiUsages.push('DELETE /dashboard/acceptance/phase35');
  assert.throws(
    () => validateDashboardPageRBACCatalog(input),
    /unmapped dashboard api usage: DELETE \/dashboard\/acceptance\/phase35/,
  );
});

test('rejects routes assigned to both exact-exempt and deny-only classes', () => {
  const input = fixture();
  input.exemptions = ['GET /dashboard/corp/select'];
  input.denyOnly = ['GET /dashboard/corp/select'];
  assert.throws(
    () => validateDashboardPageRBACCatalog(input),
    /dashboard route has conflicting policy classes: GET \/dashboard\/corp\/select/,
  );
});

test('a generic registered handler covers concrete catalog resources without widening permission mappings', () => {
  const input = fixture();
  input.catalog[0].resources[0].pathPattern = '/dashboard/reports/overview';
  input.apiUsages = ['GET /dashboard/reports/overview'];
  input.registeredAPIs = ['GET /dashboard/reports/{kind}'];
  assert.doesNotThrow(() => validateDashboardPageRBACCatalog(input));

  input.apiUsages.push('GET /dashboard/reports/not-a-real-kind');
  assert.throws(
    () => validateDashboardPageRBACCatalog(input),
    /unmapped dashboard api usage: GET \/dashboard\/reports\/not-a-real-kind/,
  );
});

test('extracts router.Handle registrations used by the production composition root', () => {
  const routes = extractBackendRegisteredAPIs([
    {
      file: 'cmd/mochat-go/scrm.go',
      body: `package main

func build(router Router, handler http.Handler) error {
  return router.Handle(http.MethodGet, reportinghttp.ReportsPath, handler)
}`,
    },
    {
      file: 'internal/modules/reporting/transport/http/handler.go',
      body: `package http

const ReportsPath = "/dashboard/reports/{kind}"`,
    },
  ]);
  assert.deepEqual(routes, [{
    contract: 'GET /dashboard/reports/{kind}',
    file: 'cmd/mochat-go/scrm.go',
    line: 4,
  }]);
});

test('migration resource seed is independently parsed and must match the catalog', () => {
  const input = fixture();
  input.seededMappings = extractMigrationPermissionResourceMappings(`
    SELECT 'dashboard.page.0' AS \`permission_code\`, 'GET' AS \`http_method\`,
      '/dashboard/reports/overview' AS \`path_pattern\`, 1 AS \`scope_required\`
  `);
  assert.doesNotThrow(() => validateDashboardPageRBACCatalog(input));

  input.seededMappings = [];
  assert.throws(
    () => validateDashboardPageRBACCatalog(input),
    /migration permission resource seed must exactly match catalog/,
  );
});

test('a non-manifest legacy browser route cannot become a 54th permission unit', () => {
  const input = fixture();
  input.catalog.push({
    code: 'dashboard.legacy.contact_field',
    path: '/contactField/index',
    name: 'legacy',
    groupCode: 'legacy',
    sort: 54,
    superadminOnly: false,
    resources: [],
  });
  assert.throws(
    () => validateDashboardPageRBACCatalog(input),
    /catalog paths must exactly match manifest paths/,
  );
});
