import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';

import {
  applyPermissionResourceReconciliation,
  applyCompanySettingsCredentialResourceOverlay,
  extractBackendRegisteredAPIs,
  extractCutoverPermissionResourceMappings,
  applyCutoverPermissionResourceOverlay,
  extractFrontendAPIUsages,
  extractMigrationPermissionResourceMappings,
  productionDashboardSourceFiles,
  validateDashboardPageRBACCatalog,
} from './check_dashboard_page_rbac_catalog.mjs';

const adminPaths = [
  '/company-setting/website',
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
    pages[48 + index] = { path, title: `管理页 ${index}`, groupId: 'company-settings' };
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

test('accepts exactly 53 pages, 48 ordinary pages and five protected management pages', () => {
  const result = validateDashboardPageRBACCatalog(fixture());
  assert.equal(result.pageCount, 53);
  assert.equal(result.ordinaryPageCount, 48);
  assert.equal(result.superadminOnlyCount, 5);
  assert.equal(result.resourceCount, 1);
});

test('agent page receives only the shared knowledge-base GET dependency', async () => {
  const catalog = JSON.parse(await readFile('internal/dashboard/dashboard_page_catalog.json', 'utf8'));
  const agentPage = catalog.pages.find((page) => page.code === 'dashboard.ai_setting.agent');
  assert.ok(agentPage, 'agent page must remain registered');
  assert.deepEqual(
    agentPage.resources.filter((resource) => resource.pathPattern.includes('knowledge-bases')),
    [{ method: 'GET', pathPattern: '/dashboard/ai-settings/knowledge-bases', scopeRequired: false }],
  );
});

test('0155 seeds and rolls back only the Agent knowledge-base GET dependency', async () => {
  const [up, down] = await Promise.all([
    readFile('deploy/standalone/migrations/0155_ai_settings_integrity_audit.up.sql', 'utf8'),
    readFile('deploy/standalone/migrations/0155_ai_settings_integrity_audit.down.sql', 'utf8'),
  ]);
  assert.deepEqual(extractMigrationPermissionResourceMappings(up), [
    'dashboard.ai_setting.agent\tGET /dashboard/ai-settings/knowledge-bases\t0',
  ]);
  assert.match(down, /permission\.`code` = 'dashboard\.ai_setting\.agent'/);
  assert.match(down, /resource\.`http_method` = 'GET'/);
  assert.match(down, /resource\.`path_pattern` = '\/dashboard\/ai-settings\/knowledge-bases'/);
  assert.doesNotMatch(down, /dashboard\.ai_setting\.ai_knowledge_base/);
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

test('activation is an exact public auth contract, while authenticated auth routes stay explicit', () => {
  const input = fixture();
  input.apiUsages.push('POST /dashboard/auth/activate');
  input.registeredAPIs.push('POST /dashboard/auth/activate');
  input.exemptions.push('POST /dashboard/auth/activate');
  assert.doesNotThrow(() => validateDashboardPageRBACCatalog(input));

  input.apiUsages.push('POST /dashboard/auth/password/reset-request');
  input.registeredAPIs.push('POST /dashboard/auth/password/reset-request');
  assert.throws(
    () => validateDashboardPageRBACCatalog(input),
    /unmapped dashboard api usage: POST \/dashboard\/auth\/password\/reset-request/,
  );
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

test('deny-only routes can be consumed by an explicitly protected page without granting a page permission', () => {
  const input = fixture();
  input.registeredAPIs.push('DELETE /dashboard/acceptance/phase35');
  input.denyOnly = ['DELETE /dashboard/acceptance/phase35'];
  assert.doesNotThrow(() => validateDashboardPageRBACCatalog(input));

  input.apiUsages.push('DELETE /dashboard/acceptance/phase35');
  assert.doesNotThrow(() => validateDashboardPageRBACCatalog(input));

  input.apiUsages.push('POST /dashboard/acceptance/phase35');
  input.registeredAPIs.push('POST /dashboard/acceptance/phase35');
  assert.throws(
    () => validateDashboardPageRBACCatalog(input),
    /unmapped dashboard api usage: POST \/dashboard\/acceptance\/phase35/,
  );
});

test('company deny-only routes may be consumed only by the protected company page', () => {
  const input = fixture();
  const companyPage = input.catalog.find((page) => page.path === '/company-setting/website');
  companyPage.resources = [{ method: 'GET', pathPattern: '/dashboard/company/profile', scopeRequired: false }];
  input.apiUsages.push('GET /dashboard/company/profile');
  input.registeredAPIs.push('GET /dashboard/company/profile');
  input.denyOnly = ['GET /dashboard/company/profile'];
  assert.doesNotThrow(() => validateDashboardPageRBACCatalog(input));

  companyPage.resources = [];
  input.catalog[1].resources = [{ method: 'GET', pathPattern: '/dashboard/company/profile', scopeRequired: false }];
  assert.throws(
    () => validateDashboardPageRBACCatalog(input),
    /deny-only dashboard route is mapped to a page: GET \/dashboard\/company\/profile/,
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

test('production shell follows the single-company profile API instead of the removed corp selector', async () => {
  const files = await productionDashboardSourceFiles();
  const normalized = files.map((file) => file.replaceAll('\\', '/'));
  assert.ok(normalized.some((file) => file.endsWith('/features/company-settings/company-profile-api.ts')));
  assert.ok(!normalized.some((file) => file.includes('/features/corp/corp-api')));
  assert.ok(!normalized.some((file) => file.includes('/features/corp/corp-provider')));
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

test('extracts two-field route table entries dispatched through route.method and route.path', () => {
  const routes = extractBackendRegisteredAPIs([{
    file: 'internal/modules/example/routes.go',
    body: `package example

func Register(registrar Registrar, handler Handler) {
  routes := []struct{ method, path string }{
    {http.MethodGet, "/dashboard/example/records"},
    {http.MethodPost, "/dashboard/example/records"},
  }
  for _, route := range routes {
    registrar.Handle(route.method, route.path, handler)
  }
}`,
  }]);
  assert.deepEqual(routes.map((route) => route.contract), [
    'GET /dashboard/example/records',
    'POST /dashboard/example/records',
  ]);
});

test('0154 reconciliation adds current resources and deactivates only exact stale mappings', () => {
  const mappings = [
    'dashboard.page.0\tGET /dashboard/reports/overview\t1',
    'dashboard.page.0\tPOST /dashboard/reports/stale\t1',
  ];
  const overlaySource = `
    INSERT INTO mochat_go_dashboard_permission_resources
    SELECT permission.id, resource_seed.http_method, resource_seed.path_pattern, resource_seed.scope_required
    FROM mochat_go_dashboard_permissions permission
    INNER JOIN (
      SELECT 'dashboard.page.1' AS permission_code, 'GET' AS http_method,
        '/dashboard/reports/current' AS path_pattern, 0 AS scope_required
    ) resource_seed ON resource_seed.permission_code = permission.code;

    UPDATE mochat_go_dashboard_permission_resources resource
    INNER JOIN mochat_go_dashboard_permissions permission ON permission.id = resource.permission_id
    INNER JOIN (
      SELECT 'dashboard.page.0' AS permission_code, 'POST' AS http_method,
        '/dashboard/reports/stale' AS path_pattern
    ) deactivation_seed ON deactivation_seed.permission_code = permission.code
      AND deactivation_seed.http_method = resource.http_method
      AND deactivation_seed.path_pattern = resource.path_pattern
    SET resource.status = 0, resource.deleted_at = NOW();
  `;

  assert.deepEqual(applyPermissionResourceReconciliation({ mappings, overlaySource }), [
    'dashboard.page.0\tGET /dashboard/reports/overview\t1',
    'dashboard.page.1\tGET /dashboard/reports/current\t0',
  ]);
});

test('0154 reconciliation restores the channel update mapping after an older overlay disabled it', async () => {
  const body = await readFile('deploy/standalone/migrations/0154_dashboard_permission_resource_reconciliation.up.sql', 'utf8');
  assert.match(body, /dashboard\.acquisition\.v2_channel_code[\s\S]*PUT[\s\S]*\/dashboard\/channelCode\/update/);
  assert.match(body, /SET resource\.`status` = 1,[\s\S]*resource\.`deleted_at` = NULL/);
});

test('0154 reconciliation rejects a deactivation that is absent from the effective seed', () => {
  assert.throws(
    () => applyPermissionResourceReconciliation({
      mappings: ['dashboard.page.0\tGET /dashboard/reports/overview\t1'],
      overlaySource: `
        UPDATE mochat_go_dashboard_permission_resources resource
        INNER JOIN (SELECT 'dashboard.page.0' AS permission_code, 'POST' AS http_method,
          '/dashboard/reports/missing' AS path_pattern) deactivation_seed ON 1 = 1;
      `,
    }),
    /0154 reconciliation deactivation does not match effective seed/,
  );
});

test('0131 cutover resource seed replaces the deployed legacy company mappings', () => {
  const mappings = extractCutoverPermissionResourceMappings(`
    INSERT INTO mochat_go_dashboard_permission_resources
      (permission_id, http_method, path_pattern)
    SELECT permission.id, resource_seed.http_method, resource_seed.path_pattern
    FROM mochat_go_dashboard_permissions permission
    INNER JOIN (
      SELECT 'GET', '/dashboard/company/profile'
      UNION ALL SELECT 'PUT', '/dashboard/company/profile'
    ) resource_seed
  `);
  assert.deepEqual(mappings, [
    'dashboard.company_setting.website\tGET /dashboard/company/profile\t0',
    'dashboard.company_setting.website\tPUT /dashboard/company/profile\t0',
  ]);
});

test('0132 adds the unified application and callback resources without rewriting 0127 or 0131', () => {
  const existing = ['dashboard.company_setting.website\tGET /dashboard/company/profile\t0'];
  const result = applyCompanySettingsCredentialResourceOverlay({
    mappings: existing,
    overlaySource: `
      INSERT INTO mochat_go_dashboard_permission_resources
      SELECT permission.id, resource_seed.http_method, resource_seed.path_pattern
      FROM mochat_go_dashboard_permissions permission
      INNER JOIN (
        SELECT 'PUT', '/dashboard/company/application-credentials'
        UNION ALL SELECT 'GET', '/dashboard/company/callback-configuration'
        UNION ALL SELECT 'POST', '/dashboard/company/callback-configuration/regenerate'
      ) resource_seed
    `,
  });
  assert.equal(result.length, 4);
});

test('RED: 0131 overlay is required before legacy mappings can be compared', () => {
  const legacyMappings = extractMigrationPermissionResourceMappings(`
    SELECT 'dashboard.company_setting.website', 'GET', '/dashboard/corp/index', 0
  `);
  assert.throws(
    () => applyCutoverPermissionResourceOverlay({ legacyMappings, overlaySource: '' }),
    /0131 cutover overlay must explicitly restrict company permission/,
  );
});

test('RED: 0131 overlay must contain every catalog company resource', () => {
  const legacyMappings = extractMigrationPermissionResourceResourceMappingsForTest();
  const overlaySource = cutoverSourceForTest([
    ['GET', '/dashboard/company/profile'],
  ]);
  assert.throws(
    () => applyCutoverPermissionResourceOverlay({ legacyMappings, overlaySource }),
    /0131 cutover overlay must replace all company resources/,
  );
});

test('RED: 0131 overlay cannot leave the company permission grantable', () => {
  const legacyMappings = extractMigrationPermissionResourceResourceMappingsForTest();
  const overlaySource = cutoverSourceForTest([
    ['GET', '/dashboard/company/profile'],
    ['PUT', '/dashboard/company/profile'],
    ['POST', '/dashboard/company/verify'],
    ['POST', '/dashboard/company/employee-sync'],
    ['GET', '/dashboard/company/sync-status'],
    ['PUT', '/dashboard/company/wecom-credentials'],
    ['PUT', '/dashboard/company/agent-credentials'],
    ['PUT', '/dashboard/company/archive-credentials'],
    ['GET', '/dashboard/company/audits'],
  ]).replace("restriction = 'superadmin_only', superadmin_only = 1", "restriction = 'ordinary', superadmin_only = 0");
  assert.throws(
    () => applyCutoverPermissionResourceOverlay({ legacyMappings, overlaySource }),
    /0131 company permission must remain superadmin_only/,
  );
});

function extractMigrationPermissionResourceResourceMappingsForTest() {
  return extractMigrationPermissionResourceMappings(`
    SELECT 'dashboard.company_setting.website', 'GET', '/dashboard/corp/index', 0
    UNION ALL SELECT 'dashboard.company_setting.website', 'PUT', '/dashboard/corp/update', 0
  `);
}

function cutoverSourceForTest(resources) {
  const values = resources.map(([method, path], index) =>
    index === 0 ? `SELECT '${method}' AS http_method, '${path}' AS path_pattern` : `UNION ALL SELECT '${method}', '${path}'`,
  ).join('\n  ');
  return `
    UPDATE mochat_go_dashboard_permissions
    SET restriction = 'superadmin_only', superadmin_only = 1
    WHERE code = 'dashboard.company_setting.website';
    DELETE resource FROM mochat_go_dashboard_permission_resources resource
    INNER JOIN mochat_go_dashboard_permissions permission ON permission.id = resource.permission_id
    WHERE permission.code = 'dashboard.company_setting.website'
      AND resource.path_pattern IN ('/dashboard/corp/index', '/dashboard/corp/update');
    INSERT INTO mochat_go_dashboard_permission_resources (permission_id, http_method, path_pattern)
    SELECT permission.id, resource_seed.http_method, resource_seed.path_pattern
    FROM mochat_go_dashboard_permissions permission
    INNER JOIN (
      ${values}
    ) resource_seed ON 1 = 1
    WHERE permission.code = 'dashboard.company_setting.website';
  `;
}

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
