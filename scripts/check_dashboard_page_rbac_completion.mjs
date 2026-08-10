import { readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { scanBackendRegisteredAPIs, scanFrontendAPIUsages } from './check_dashboard_page_rbac_catalog.mjs';

export function validateCompletionFacts({ catalogOutput, sourceCorpus, e2eSource, smokeSource, packageJSON, frontendSource = '', backendEvidence = '', scopeMappings = '' }) {
  if (!/^53 pages, 49 ordinary, 4 superadmin_only, 0 unmapped dashboard API usages$/.test(catalogOutput.trim())) {
    throw new Error('catalog gate must report 53/49/4 and zero unmapped usages');
  }
  for (const forbidden of ['benchmarkRoutes', 'SaaS 管理后台', 'Dashboard→SaaS']) {
    if (sourceCorpus.includes(forbidden)) throw new Error(`forbidden legacy authorization fact: ${forbidden}`);
  }
  if (/fallback\s*[:=].*allow|allow\s*[:=].*fallback|ordinary.*company-setting\/staff/i.test(sourceCorpus)) throw new Error('ordinary management or fallback allow found');
  if (/DataPermission/.test(sourceCorpus)) throw new Error('scopeRequired path still reads legacy DataPermission');
  if (frontendSource && /dashboard\/access\/profile/.test(frontendSource) && !/dashboard\/access\/catalog/.test(frontendSource)) throw new Error('frontend API extraction missed catalog usage');
  if (backendEvidence && /GET \/dashboard\/access\/users/.test(backendEvidence) && !/source:/.test(backendEvidence)) throw new Error('backend handler evidence must include source file and line');
  if (scopeMappings && !/handler .*\(.+:[0-9]+\) -> guard .*:[0-9]+ -> consumer /.test(scopeMappings)) throw new Error('scope mapping must include handler, guard, and consumer source evidence');
  if (!e2eSource.includes('390') || !e2eSource.includes('53') || !e2eSource.includes('49') || !/for\s*\(const route of (routes|ordinaryRoutes)/.test(e2eSource)) {
    throw new Error('Playwright matrix must declare 53/49/4 and 390px coverage');
  }
  if (!e2eSource.includes('TENANT_ACCESS_DENIED') || !e2eSource.includes('DASHBOARD_PERMISSION_DENIED') || !/status\(\)\)?\.toBe\(403\)/.test(e2eSource)) {
    throw new Error('Playwright matrix must cover both stable 403 machine codes');
  }
  if (/down\s+-v|volume\s+rm|system\s+prune|volume\s+prune/i.test(smokeSource)) {
    throw new Error('smoke script contains destructive Docker operation');
  }
  if (/PSCredential|\-Credential\b|Basic\s+/i.test(smokeSource) || !smokeSource.includes('/dashboard/user/auth') || !smokeSource.includes('Authorization')) {
    throw new Error('smoke must authenticate with dashboard/user/auth and Bearer token, never Basic credentials');
  }
  if (!smokeSource.includes('TargetUserId') || !smokeSource.includes('CrossTenantUserId') || !smokeSource.includes('MutationJson') || !smokeSource.includes('sh -lc') || !smokeSource.includes('COUNT(*)')) throw new Error('full smoke must exercise protected mutation, cross-tenant read, and exact table counts');
  if (!packageJSON.scripts?.['check:phase4-dashboard-page-rbac']) {
    throw new Error('package script check:phase4-dashboard-page-rbac is required');
  }
  if (!e2eSource.includes(".phase35-page-shell") || !e2eSource.includes('document.documentElement.scrollWidth') || !e2eSource.includes('page.on')) {
    throw new Error('Playwright must assert page shell, 390px overflow, and console/network evidence');
  }
  if (!e2eSource.includes('MOCHAT_E2E_LIVE_BASE') || !e2eSource.includes('MOCHAT_E2E_RBAC_FIXTURE_JSON') || !e2eSource.includes('liveFixture') || !e2eSource.includes('directCode') || !e2eSource.includes('roleUnionCode') || !e2eSource.includes('disabledRoleCode') || !e2eSource.includes('noPermission') || !e2eSource.includes('mochat_dashboard_token') || !e2eSource.includes('tenantDenied')) throw new Error('live Playwright fixture/login matrix is required');
  return { pages: 53, ordinary: 49, superadminOnly: 4 };
}

async function readGoFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const full = path.join(directory, entry.name);
    if (entry.isDirectory()) files.push(...await readGoFiles(full));
    else if (entry.name.endsWith('.go')) files.push(full);
  }
  return files;
}

export async function runCompletionGate(root = process.cwd()) {
  const catalog = await import('./check_dashboard_page_rbac_catalog.mjs');
  const manifest = JSON.parse(await readFile(path.join(root, 'web/apps/dashboard/src/benchmark/manifest.json'), 'utf8'));
  const pageCatalog = JSON.parse(await readFile(path.join(root, 'internal/dashboard/dashboard_page_catalog.json'), 'utf8'));
  const routePolicy = catalog.extractGoDashboardRoutePolicy(await readFile(path.join(root, 'internal/dashboard/dashboard_route_policy.go'), 'utf8'));
  const seededMappings = catalog.extractMigrationPermissionResourceMappings(await readFile(path.join(root, 'deploy/standalone/migrations/0127_dashboard_page_rbac.up.sql'), 'utf8'));
  const frontend = await scanFrontendAPIUsages();
  const backend = (await scanBackendRegisteredAPIs()).filter((route) => {
    const routePath = route.contract.slice(route.contract.indexOf(' ') + 1);
    return routePath.startsWith('/dashboard/')
      && !routePath.startsWith('/dashboard/saasAdmin/')
      && !routePath.startsWith('/dashboard/saasAlert/')
      && !routePath.startsWith('/dashboard/saasBilling/');
  });
  const result = catalog.validateDashboardPageRBACCatalog({
    manifest,
    catalog: pageCatalog.pages,
    apiUsages: frontend,
    registeredAPIs: backend.map((route) => route.contract),
    registeredSources: new Map(backend.map((route) => [route.contract, `${route.file}:${route.line}`])),
    exemptions: routePolicy.exactExempt,
    denyOnly: routePolicy.denyOnly,
    seededMappings,
  });
  const output = `${result.pageCount} pages, ${result.ordinaryPageCount} ordinary, ${result.superadminOnlyCount} superadmin_only, ${result.unmappedAPIUsageCount} unmapped dashboard API usages`;
  const [packageJSON, e2eSource, smokeSource] = await Promise.all([
    readFile(path.join(root, 'package.json'), 'utf8').then(JSON.parse),
    readFile(path.join(root, 'web/e2e/tests/dashboard-page-rbac.spec.ts'), 'utf8'),
    readFile(path.join(root, 'scripts/smoke_dashboard_page_rbac.ps1'), 'utf8'),
  ]);
  const sourceCorpus = [
    await readFile(path.join(root, 'web/apps/dashboard/src/app/access-loader.ts'), 'utf8'),
    await readFile(path.join(root, 'web/apps/dashboard/src/layout/dashboard-layout.tsx'), 'utf8'),
  ].join('\n');
  const dashboardGoFiles = await readGoFiles(path.join(root, 'internal/dashboard'));
  const dashboardGo = (await Promise.all(dashboardGoFiles.map((file) => readFile(file, 'utf8')))).join('\n');
  const scopeResources = pageCatalog.pages.flatMap((page) => (page.resources ?? []).filter((resource) => resource.scopeRequired).map((resource) => ({ page: page.code, ...resource })));
  if (scopeResources.length === 0) throw new Error('catalog must declare scopeRequired resources');
  const accessGo = (await Promise.all(dashboardGoFiles.filter((file) => /dashboard_access/.test(file) && !file.endsWith('_test.go')).map((file) => readFile(file, 'utf8')))).join('\n');
  if (!dashboardGo.includes('DashboardAccessContext') || /DataPermission/.test(accessGo)) {
    throw new Error('scopeRequired handlers must use DashboardAccessContext and not legacy DataPermission');
  }
  const covers = (registered, resource) => { const [rm, rp] = registered.contract.split(' '); if (rm !== resource.method) return false; const a = rp.split('/'); const b = resource.pathPattern.split('/'); return a.length === b.length && a.every((segment, index) => /^\{[^/]+\}$/.test(segment) || segment === b[index]); };
  const guardFile = path.join(root, 'internal/dashboard/dashboard_access_guard.go');
  const guardBody = await readFile(guardFile, 'utf8');
  const guardLine = guardBody.slice(0, guardBody.indexOf('type DashboardAccessContext')).split('\n').length;
  const consumerCandidates = dashboardGoFiles.filter((file) => !file.endsWith('_test.go') && /dashboard_access|corp_data|scrm/.test(file));
  const consumerEvidence = (await Promise.all(consumerCandidates.map(async (file) => ({ file, body: await readFile(file, 'utf8') })))).filter(({ body }) => /DashboardEmployeeScope|AllowedEmployeeIDs|DashboardAccessContext/.test(body)).map(({ file, body }) => `${file.replaceAll('\\', '/')}:${body.split('\n').findIndex((line) => /DashboardEmployeeScope|AllowedEmployeeIDs|DashboardAccessContext/.test(line)) + 1}`);
  if (consumerEvidence.length === 0) throw new Error('scopeRequired consumer evidence missing');
  const scopeMappings = scopeResources.map((resource) => { const route = backend.find((candidate) => covers(candidate, resource)); if (!route) throw new Error(`scopeRequired resource has no registered handler: ${resource.method} ${resource.pathPattern}`); return `${resource.method} ${resource.pathPattern} -> handler ${route.contract} (${route.file}:${route.line}) -> guard internal/dashboard/dashboard_access_guard.go:${guardLine} -> consumer ${consumerEvidence[0]}`; });
  const facts = validateCompletionFacts({ catalogOutput: output, sourceCorpus: `${sourceCorpus}\n${accessGo}`, frontendSource: frontend.map((item) => typeof item === 'string' ? item : (item.contract ?? item.path ?? '')).join('\n'), backendEvidence: backend.map((route) => `source:${route.file}:${route.line} ${route.contract}`).join('\n'), scopeMappings: scopeMappings.join('\n'), e2eSource, smokeSource, packageJSON });
  return { ...facts, scopeRequired: scopeMappings.length, scopeMappings };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const result = await runCompletionGate();
  console.log(`completion gate PASS: ${result.pages}/${result.ordinary}/${result.superadminOnly}; scopeRequired=${result.scopeRequired}`);
  for (const mapping of result.scopeMappings) console.log(`scope mapping: ${mapping}`);
}
