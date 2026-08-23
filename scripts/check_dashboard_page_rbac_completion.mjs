import { readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import {
  applyPermissionResourceReconciliation,
  applyCompanySettingsCredentialResourceOverlay,
  applyCutoverPermissionResourceOverlay,
  applyEmployeeAccountResourceOverlay,
  scanBackendRegisteredAPIs,
  scanFrontendAPIUsages,
} from './check_dashboard_page_rbac_catalog.mjs';

export function validateCompletionFacts({ catalogOutput, sourceCorpus, e2eSource, smokeSource, packageJSON, frontendSource = '', backendEvidence = '', scopeMappings = '' }) {
  if (!/^53 pages, 48 ordinary, 5 superadmin_only, 0 unmapped dashboard API usages$/.test(catalogOutput.trim())) {
    throw new Error('catalog gate must report 53/48/5 and zero unmapped usages');
  }
  for (const forbidden of ['benchmarkRoutes', 'SaaS 绠＄悊鍚庡彴', 'Dashboard鈫扴aaS']) {
    if (sourceCorpus.includes(forbidden)) throw new Error(`forbidden legacy authorization fact: ${forbidden}`);
  }
  if (/fallback\s*[:=].*allow|allow\s*[:=].*fallback|ordinary.*company-setting\/staff/i.test(sourceCorpus)) throw new Error('ordinary management or fallback allow found');
  if (/DataPermission/.test(sourceCorpus)) throw new Error('scopeRequired path still reads legacy DataPermission');
  if (frontendSource && /dashboard\/access\/profile/.test(frontendSource) && !/dashboard\/access\/catalog/.test(frontendSource)) throw new Error('frontend API extraction missed catalog usage');
  if (backendEvidence && /GET \/dashboard\/access\/users/.test(backendEvidence) && !/source:/.test(backendEvidence)) throw new Error('backend handler evidence must include source file and line');
  if (scopeMappings && !/handler .*\(.+:[0-9]+\) -> guard .*:[0-9]+ -> consumer .+:[0-9]+/.test(scopeMappings)) throw new Error('scope mapping must include handler, guard, and consumer source evidence');
  if (!e2eSource.includes('390') || !e2eSource.includes('53') || !e2eSource.includes('48') || !e2eSource.includes('5') || !/for\s*\(const route of (routes|ordinaryRoutes)/.test(e2eSource)) {
    throw new Error('Playwright matrix must declare 53/48/5 and 390px coverage');
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
  if (!e2eSource.includes('MOCHAT_E2E_LIVE_BASE') || !e2eSource.includes('MOCHAT_E2E_RBAC_FIXTURE_JSON') || !e2eSource.includes('liveFixture') || !e2eSource.includes('exactAllowedRoutes') || !e2eSource.includes('expectedSources') || !e2eSource.includes('forbiddenCodes') || !e2eSource.includes('forbiddenRoleIds') || !e2eSource.includes('noPermission') || !e2eSource.includes('mochat_dashboard_token') || !e2eSource.includes('tenantDenied')) throw new Error('live Playwright fixture/login matrix is required');
  return { pages: 53, ordinary: 48, superadminOnly: 5 };
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
  const legacySeededMappings = catalog.extractMigrationPermissionResourceMappings(
    await readFile(path.join(root, 'deploy/standalone/migrations/0127_dashboard_page_rbac.up.sql'), 'utf8'),
  );
  const overlaySource = await readFile(
    path.join(root, 'deploy/standalone/migrations/0131_identity_realms_single_corp_cutover.up.sql'),
    'utf8',
  );
  const cutoverMappings = applyCutoverPermissionResourceOverlay({
    legacyMappings: legacySeededMappings,
    overlaySource,
  });
  const companyCredentialMappings = applyCompanySettingsCredentialResourceOverlay({
    mappings: cutoverMappings,
    overlaySource: await readFile(
      path.join(root, 'deploy/standalone/migrations/0132_company_settings_credentials.up.sql'),
      'utf8',
    ),
  });
  const employeeAccountMappings = applyEmployeeAccountResourceOverlay({
    mappings: companyCredentialMappings,
    overlaySource: await readFile(
      path.join(root, 'deploy/standalone/migrations/0133_archive_simulation_registry.up.sql'),
      'utf8',
    ),
  });
  const liveCodeMappings = [...new Set(employeeAccountMappings.concat(catalog.extractMigrationPermissionResourceMappings(
    await readFile(path.join(root, 'deploy/standalone/migrations/0153_live_code_workspace.up.sql'), 'utf8'),
  )))];
  const reconciledMappings = applyPermissionResourceReconciliation({
    mappings: liveCodeMappings,
    overlaySource: await readFile(
      path.join(root, 'deploy/standalone/migrations/0154_dashboard_permission_resource_reconciliation.up.sql'),
      'utf8',
    ),
  });
  const seededMappings = applyPermissionResourceReconciliation({
    mappings: reconciledMappings,
    overlaySource: await readFile(
      path.join(root, 'deploy/standalone/migrations/0155_ai_settings_integrity_audit.up.sql'),
      'utf8',
    ),
  });
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
  const scopeGoRoots = [
    path.join(root, 'internal/dashboard'),
    path.join(root, 'internal/modules/reporting'),
    path.join(root, 'internal/modules/scrm'),
    path.join(root, 'internal/modules/ai-insight'),
    path.join(root, 'cmd/mochat-go'),
  ];
  const scopeGoFiles = (await Promise.all(scopeGoRoots.map((directory) => readGoFiles(directory)))).flat();
  const dashboardGo = (await Promise.all(scopeGoFiles.map((file) => readFile(file, 'utf8')))).join('\n');
  const scopeResources = pageCatalog.pages.flatMap((page) => (page.resources ?? []).filter((resource) => resource.scopeRequired).map((resource) => ({ page: page.code, ...resource })));
  if (scopeResources.length === 0) throw new Error('catalog must declare scopeRequired resources');
  const accessGo = (await Promise.all(scopeGoFiles.filter((file) => /dashboard_access/.test(file) && !file.endsWith('_test.go')).map((file) => readFile(file, 'utf8')))).join('\n');
  if (!dashboardGo.includes('DashboardAccessContext') || /DataPermission/.test(accessGo)) {
    throw new Error('scopeRequired handlers must use DashboardAccessContext and not legacy DataPermission');
  }
  const covers = (registered, resource) => { const [rm, rp] = registered.contract.split(' '); if (rm !== resource.method) return false; const a = rp.split('/'); const b = resource.pathPattern.split('/'); return a.length === b.length && a.every((segment, index) => /^\{[^/]+\}$/.test(segment) || segment === b[index]); };
  const guardFile = path.join(root, 'internal/dashboard/dashboard_access_guard.go');
  const guardBody = await readFile(guardFile, 'utf8');
  const guardLine = guardBody.slice(0, guardBody.indexOf('type DashboardAccessContext')).split('\n').length;
  const consumerCandidates = scopeGoFiles.filter((file) => !file.endsWith('_test.go'));
  const consumerSources = await Promise.all(consumerCandidates.map(async (file) => ({ file, body: await readFile(file, 'utf8') })));
  const functionBody = (body, symbol) => {
    const start = body.indexOf(symbol);
    if (start < 0) return '';
    const brace = body.indexOf('{', start);
    if (brace < 0) return '';
    let depth = 0;
    for (let i = brace; i < body.length; i += 1) {
      if (body[i] === '{') depth += 1;
      else if (body[i] === '}' && --depth === 0) return body.slice(start, i + 1);
    }
    return '';
  };
  const consumerRules = [
    { test: /reports|report/i, evidence: /AllowedEmployeeIDs|EmployeeScopeRestricted|intersectIDs/, files: [/modules[\\/]reporting[\\/]service\.go$/, /cmd[\\/]mochat-go[\\/]scrm\.go$/] },
    { test: /workMessage/i, evidence: /func \(h \*AutoTagHandler\) WorkMessage(FromUsers|ToUsers|Index)\b/, files: [/internal[\\/]dashboard[\\/]auto_tag_dashboard\.go$/], evidenceForRoute: (resource) => {
      if (/fromUsers/i.test(resource.pathPattern)) return /func \(h \*AutoTagHandler\) WorkMessageFromUsers\b/;
      if (/toUsers/i.test(resource.pathPattern)) return /func \(h \*AutoTagHandler\) WorkMessageToUsers\b/;
      return /func \(h \*AutoTagHandler\) WorkMessageIndex\b/;
    } },
    { test: /channelCode/i, evidence: /DashboardAccessFromContext|AllowedEmployeeIDs|DeptEmployeeIDs|ChannelCodeBusinessIDsByOperators/, files: [/internal[\\/]dashboard[\\/]channel_code_routes\.go$/, /internal[\\/]dashboard[\\/]channel_code_write\.go$/], symbolForRoute: (r) => /store/i.test(r.pathPattern) ? 'func (h *ChannelCodeHandler) Store' : 'func (h *ChannelCodeHandler) Index' },
    { test: /workEmployee[\\/]index/i, evidence: /DashboardAccessFromContext|RestrictEmployeeIDs|EmployeeIDs/, files: [/internal[\\/]dashboard[\\/]work_read\.go$/] },
    { test: /workContact[\\/]index/i, evidence: /DashboardAccessFromContext|RestrictEmployees|EmployeeIDs/, files: [/internal[\\/]dashboard[\\/]work_read\.go$/] },
    { test: /workContact[\\/]show/i, evidence: /DashboardAccessFromContext|employee scope denied|AllowedEmployeeIDs/, files: [/internal[\\/]dashboard[\\/]work_read\.go$/] },
    { test: /workRoom[\\/](index|roomIndex)/i, evidence: /DashboardAccessFromContext|RestrictOwner|OwnerIDs/, files: [/internal[\\/]dashboard[\\/]work_read\.go$/] },
    { test: /workRoomAutoPull/i, evidence: /DashboardAccessFromContext|AllowedEmployeeIDs|DeptEmployeeIDs|WorkRoomAutoPullBusinessIDsByOperators/, files: [/internal[\\/]dashboard[\\/]work_room_auto_pull\.go$/], symbolForRoute: (r) => /store/i.test(r.pathPattern) ? 'func (h *WorkRoomAutoPullHandler) Store' : 'func (h *WorkRoomAutoPullHandler) Index' },
    { test: /roomMessageBatchSend/i, evidence: /DashboardAccessFromContext|AllowedEmployeeIDs|RestrictEmployeeIDs|employee scope denied/, files: [/internal[\\/]dashboard[\\/]room_message_batch_send\.go$/], symbolForRoute: (r) => /store/i.test(r.pathPattern) ? 'func (h *RoomMessageBatchSendHandler) Store' : 'func (h *RoomMessageBatchSendHandler) Index' },
    { test: /contactMessageBatchSend/i, evidence: /DashboardAccessFromContext|AllowedEmployeeIDs|RestrictEmployeeIDs|employee scope denied/, files: [/internal[\\/]dashboard[\\/]contact_message_batch_send\.go$/], symbolForRoute: (r) => /store/i.test(r.pathPattern) ? 'func (h *ContactMessageBatchSendHandler) Store' : 'func (h *ContactMessageBatchSendHandler) Index' },
    { test: /friendsCircle\/(taskIndex|taskResultIndex|exportData|publish|taskStore|materialStore)/i, evidence: /DashboardAccessFromContext|employeeIDsWithinDashboardScope|filterFriendsCircle/, files: [/internal[\\/]dashboard[\\/]friends_circle\.go$/], symbolForRoute: (r) => { const p=r.pathPattern; if (/taskIndex/.test(p)) return 'func (h *FriendsCircleHandler) TaskIndex'; if (/taskResultIndex/.test(p)) return 'func (h *FriendsCircleHandler) TaskResultIndex'; if (/taskStore/.test(p)) return 'func (h *FriendsCircleHandler) TaskStore'; if (/publish/.test(p)) return 'func (h *FriendsCircleHandler) Publish'; if (/exportData/.test(p)) return 'func (h *FriendsCircleHandler) ExportData'; return 'func (h *FriendsCircleHandler) MaterialStore'; } },
    { test: /risk[\\/]records/i, evidence: /DashboardAccessFromContext|AllowedEmployeeIDs|RestrictEmployeeIDs|JSON_EXTRACT/, files: [/internal[\\/]dashboard[\\/]risk_behavior_handler\.go$/, /internal[\\/]store[\\/]risk_behavior\.go$/], symbolForRoute: (r) => /audit/i.test(r.pathPattern) ? 'func (h *RiskBehaviorHandler) AuditRecords' : /detail/i.test(r.pathPattern) ? 'func (h *RiskBehaviorHandler) RecordDetail' : 'func (h *RiskBehaviorHandler) Records' },
    { test: /timeout-warning[\\/]records/i, evidence: /DashboardAccessFromContext|AllowedEmployeeIDs|RestrictEmployeeIDs|assigned_employee_id/, files: [/internal[\\/]dashboard[\\/]timeout_warning_handler\.go$/, /internal[\\/]store[\\/]timeout_warning\.go$/], symbolForRoute: (r) => /audit/i.test(r.pathPattern) ? 'func (h *TimeoutWarningHandler) AuditRecords' : /assign/i.test(r.pathPattern) ? 'func (h *TimeoutWarningHandler) AssignRecords' : 'func (h *TimeoutWarningHandler) Records' },
    { test: /sensitiveWordsMonitor/i, evidence: /AllowedEmployeeIDs|intersectPositiveIntIDs|SensitiveWordsMonitorMessageFilter|RestrictEmployeeIDs/, files: [/internal[\\/]dashboard[\\/]sensitive_word\.go$/], symbolForRoute: (r) => /show/i.test(r.pathPattern) ? 'func (h *SensitiveWordHandler) MonitorShow' : 'func (h *SensitiveWordHandler) MonitorIndex' },
    { test: /workContact[\\/]lossContact|contactTransfer/i, evidence: /AllowedEmployeeIDs|intersectPositiveIntIDs|EmployeeIDs/, files: [/internal[\\/]dashboard[\\/]contact_transfer\.go$/] },
    { test: /silent-customer[\\/]records/i, evidence: /DashboardAccessFromContext|AllowedEmployeeIDs|RestrictEmployeeIDs|assigned_employee_id|employee scope denied/, files: [/internal[\\/]dashboard[\\/]phase33_closure_handler\.go$/, /internal[\\/]store[\\/]phase33_closure\.go$/], symbolForRoute: (r) => /action/i.test(r.pathPattern) ? 'func (h *Phase33ClosureHandler) ActSilent' : 'func (h *Phase33ClosureHandler) SilentRecords' },
    { test: /message-intercept[\\/]records/i, evidence: /DashboardAccessFromContext|employee-scoped|employee scope/, files: [/internal[\\/]dashboard[\\/]message_intercept_handler\.go$/], symbolForRoute: (r) => /audit/i.test(r.pathPattern) ? 'func (h *MessageInterceptHandler) Audit' : 'func (h *MessageInterceptHandler) Records' },
    { test: /customerService/i, evidence: /DashboardAccessFromContext|AllowedEmployeeIDs|RestrictEmployeeIDs|employeeIDsWithinDashboardScope/, files: [/internal[\\/]dashboard[\\/]phase34_acquisition_provider\.go$/, /internal[\\/]store[\\/]mysql\.go$/], symbolForRoute: (r) => /index/i.test(r.pathPattern) ? 'func (h *Phase34AcquisitionHandler) CustomerServiceIndex' : /store/i.test(r.pathPattern) ? 'func (h *Phase34AcquisitionHandler) CustomerServiceStore' : 'func (h *Phase34AcquisitionHandler) CustomerServiceSync' },
    { test: /scrm[\\/]tags[\\/].+contacts/i, evidence: /EmployeeScopeRestricted|contact ownership is outside dashboard scope/, files: [/modules[\\/]scrm[\\/]transport[\\/]http[\\/]customer_tag_handler\.go$/], symbolForRoute: () => 'func (h *CustomerTagHandler) MaintainContacts' },
    { test: /ai-insight[\\/](session-analysis|smart-analysis|emotion|employee-score|communication-keyword)/i, evidence: /FetchArchiveTexts|EmployeeScopeRestricted|AllowedEmployeeIDs/, files: [/internal[\\/]modules[\\/]ai-insight[\\/]transport[\\/]http[\\/]handler\.go$/, /internal[\\/]modules[\\/]ai-insight[\\/]transport[\\/]http[\\/]analysis_store\.go$/] },
    { test: /scrm[\\/](contacts|assignments)/i, evidence: /AllowedEmployeeIDs|EmployeeScopeRestricted|restrictOwnerIDs/, files: [/modules[\\/]scrm[\\/]application[\\/]customer_lifecycle_service\.go$/, /modules[\\/]scrm[\\/]transport[\\/]http[\\/]customer_lifecycle_handler\.go$/] },
    { test: /scrm[\\/]leads|customer[\\/]clue/i, evidence: /AllowedEmployeeIDs|EmployeeScopeRestricted|restrictOwnerIDs/, files: [/modules[\\/]scrm[\\/]application[\\/]service\.go$/, /modules[\\/]scrm[\\/]transport[\\/]http[\\/]lead_handler\.go$/] },
    { test: /scrm[\\/]opportunit|customer[\\/]opportunit/i, evidence: /AllowedEmployeeIDs|EmployeeScopeRestricted|owner_id IN/, files: [/modules[\\/]scrm[\\/]adapters[\\/]mysql[\\/]opportunity_repository\.go$/, /modules[\\/]scrm[\\/]transport[\\/]http[\\/]opportunity_handler\.go$/] },
    { test: /scrm[\\/]orders/i, evidence: /AllowedEmployeeIDs|EmployeeScopeRestricted|OwnerID|owner/, files: [/modules[\\/]scrm[\\/]transport[\\/]http[\\/]order_handler\.go$/, /modules[\\/]scrm[\\/]adapters[\\/]mysql[\\/]order_repository\.go$/] },
  ];
  const scopeMappings = scopeResources.map((resource) => {
    const route = backend.find((candidate) => covers(candidate, resource));
    if (!route) throw new Error(`scopeRequired resource has no registered handler: ${resource.method} ${resource.pathPattern}`);
    const rule = consumerRules.find((candidate) => candidate.test.test(resource.pathPattern) || candidate.test.test(route.contract));
    const evidence = rule?.evidenceForRoute ? rule.evidenceForRoute(resource) : rule?.evidence;
    const symbol = rule?.symbolForRoute ? rule.symbolForRoute(resource) : '';
    const consumer = rule && consumerSources.find(({ file, body }) => rule.files.some((pattern) => pattern.test(file)) && evidence.test(symbol ? functionBody(body, symbol) : body));
    if (!consumer) throw new Error(`scopeRequired consumer evidence missing for ${resource.method} ${resource.pathPattern}; handler=${route.file}:${route.line}`);
    const consumerLine = symbol ? consumer.body.split('\n').findIndex((line) => line.includes(symbol.replace(/^func /, ''))) + 1 : consumer.body.split('\n').findIndex((line) => evidence.test(line)) + 1;
    return `${resource.method} ${resource.pathPattern} -> handler ${route.contract} (${route.file}:${route.line}) -> guard internal/dashboard/dashboard_access_guard.go:${guardLine} -> consumer ${consumer.file.replaceAll('\\', '/')}:${consumerLine}`;
  });
  const facts = validateCompletionFacts({ catalogOutput: output, sourceCorpus: `${sourceCorpus}\n${accessGo}`, frontendSource: frontend.map((item) => typeof item === 'string' ? item : (item.contract ?? item.path ?? '')).join('\n'), backendEvidence: backend.map((route) => `source:${route.file}:${route.line} ${route.contract}`).join('\n'), scopeMappings: scopeMappings.join('\n'), e2eSource, smokeSource, packageJSON });
  return { ...facts, scopeRequired: scopeMappings.length, scopeMappings };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const result = await runCompletionGate();
  console.log(`completion gate PASS: ${result.pages}/${result.ordinary}/${result.superadminOnly}; scopeRequired=${result.scopeRequired}`);
  for (const mapping of result.scopeMappings) console.log(`scope mapping: ${mapping}`);
}
