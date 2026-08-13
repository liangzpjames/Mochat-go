import { readFileSync, readdirSync } from 'node:fs';
import { extname, join, relative, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

const EXPECTED_ROUTE_COUNTS = {
  Sidebar: 12,
  Operation: 10,
};

const ROOT_SCRIPT = 'node scripts/check_mobile_clients_foundation.mjs && node --test scripts/check_mobile_clients_foundation.test.mjs';
const E2E_SCRIPT = 'playwright test tests/mobile-clients-foundation.spec.ts --workers=1';

const MOJIBAKE_PATTERN = /(?:锛|銆|鈥|鈮|馃|椤甸潰|妯″潡|浠诲姟|绔欏唴|涓嶅瓨鍦|瀹㈡埛|绉诲姩|鐢ㄦ埛|璇锋眰|璺敱)/g;
const FAKE_OUTCOME_PATTERNS = [
  /Math\.random\s*\(/g,
  /\b(?:fixedProgress|progress|percentage|percent)\s*(?:=|:)\s*\d+(?:\.\d+)?\b/g,
  /(?:操作已完成|提交成功|保存成功|领取成功|随机奖品|模拟成功|虚假进度|固定进度)/g,
];

function readJSON(root, relativePath) {
  return JSON.parse(readFileSync(join(root, relativePath), 'utf8'));
}

function normalized(relativePath) {
  return relativePath.replaceAll('\\', '/');
}

function productionSources(root, sourceRoot) {
  const absoluteRoot = join(root, sourceRoot);
  const sources = [];
  const visit = (directory) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      if (entry.isDirectory()) {
        if (/^(?:__tests__|tests?|fixtures?|dist|build|coverage|node_modules)$/i.test(entry.name)) continue;
        visit(join(directory, entry.name));
        continue;
      }
      if (!entry.isFile() || !['.ts', '.tsx'].includes(extname(entry.name))) continue;
      if (/(?:^|\.)(?:test|spec|stories)\.[cm]?[jt]sx?$/i.test(entry.name)) continue;
      const filePath = join(directory, entry.name);
      sources.push({
        path: normalized(relative(root, filePath)),
        source: readFileSync(filePath, 'utf8'),
      });
    }
  };
  visit(absoluteRoot);
  return sources;
}

function manifestPaths(manifest, label, errors) {
  if (!Array.isArray(manifest)) {
    errors.push(`${label} manifest must be an array`);
    return [];
  }
  const paths = [];
  for (const [index, route] of manifest.entries()) {
    if (typeof route !== 'object' || route === null || typeof route.path !== 'string' || !route.path.startsWith('/')) {
      errors.push(`${label} manifest route ${index} has no valid path`);
      continue;
    }
    if (route.target !== 'react') errors.push(`${label} manifest route ${route.path} must target react`);
    paths.push(route.path);
  }
  const unique = new Set(paths);
  if (unique.size !== paths.length) errors.push(`${label} manifest contains duplicate paths`);
  if (paths.length !== EXPECTED_ROUTE_COUNTS[label]) {
    errors.push(`${label} manifest must contain ${EXPECTED_ROUTE_COUNTS[label]} routes; found ${paths.length}`);
  }
  return paths;
}

function registryPaths(source, label, errors) {
  const declaration = /const\s+routeDefinitions\s*=\s*\{/.exec(source);
  if (declaration === null) {
    errors.push(`${label} registry must declare routeDefinitions independently`);
    return [];
  }
  const start = declaration.index + declaration[0].length;
  const end = source.indexOf('} as const', start);
  if (end === -1) {
    errors.push(`${label} registry routeDefinitions must be an as const object`);
    return [];
  }
  const body = source.slice(start, end);
  const paths = [...body.matchAll(/^\s*(['"])(\/[^'"]*)\1\s*:/gm)].map((match) => match[2]);
  if (new Set(paths).size !== paths.length) errors.push(`${label} registry contains duplicate paths`);
  return paths;
}

function compareRegistryToManifest(label, registry, manifest, errors) {
  const manifestSet = new Set(manifest);
  const registrySet = new Set(registry);
  for (const path of registry) {
    if (!manifestSet.has(path)) errors.push(`${label} registry route ${path} is absent from manifest`);
  }
  for (const path of manifest) {
    if (!registrySet.has(path)) errors.push(`${label} manifest route ${path} is absent from registry`);
  }
}

function specArrayBody(source, variable, errors) {
  const declaration = new RegExp(`const\\s+${variable}\\s*=\\s*\\[`).exec(source);
  if (declaration === null) {
    errors.push(`browser spec must declare ${variable}`);
    return '';
  }
  const start = declaration.index + declaration[0].length;
  const end = source.indexOf('] as const', start);
  if (end === -1) {
    errors.push(`browser spec ${variable} must be an as const array`);
    return '';
  }
  return source.slice(start, end);
}

function specCasePaths(source, variable, errors) {
  return [...specArrayBody(source, variable, errors).matchAll(/\bpath\s*:\s*(['"])(\/[^'"]*)\1/g)]
    .map((match) => match[2]);
}

function compareBrowserCases(label, cases, manifest, errors) {
  const expected = new Set(manifest);
  const actual = new Set(cases);
  if (actual.size !== cases.length) errors.push(`${label} browser cases contain duplicate paths`);
  for (const path of cases) {
    if (!expected.has(path)) errors.push(`${label} browser case ${path} is absent from manifest`);
  }
  for (const path of manifest) {
    if (!actual.has(path)) errors.push(`${label} manifest route ${path} has no browser case`);
  }
}

function validateUnknownRoute(label, source, errors) {
  const wildcard = source.match(/\{\s*path\s*:\s*['"]\*['"][\s\S]{0,240}?\}/);
  if (wildcard === null) {
    errors.push(`${label} router must register an unknown-route wildcard`);
    return;
  }
  if (/Navigate[\s\S]*?to\s*=\s*['"]\/['"]|(?:Home|routeDefinitions\[['"]\/['"]\])/i.test(wildcard[0])) {
    errors.push(`${label} unknown route falls back to home`);
    return;
  }
  if (!/NotFound/.test(wildcard[0])) errors.push(`${label} unknown route must render a NotFound page`);
}

function countMatches(source, pattern) {
  return [...source.matchAll(pattern)].length;
}

function braceBody(source, openingPattern) {
  const opening = openingPattern.exec(source);
  if (opening === null) return null;
  const openingBrace = source.indexOf('{', opening.index);
  if (openingBrace === -1) return null;
  let depth = 0;
  for (let index = openingBrace; index < source.length; index += 1) {
    if (source[index] === '{') depth += 1;
    if (source[index] === '}') {
      depth -= 1;
      if (depth === 0) return source.slice(openingBrace + 1, index);
    }
  }
  return null;
}

function validatePackageScripts(root, errors) {
  const rootPackage = readJSON(root, 'package.json');
  const e2ePackage = readJSON(root, 'web/e2e/package.json');
  if (rootPackage.scripts?.['check:mobile-clients-foundation'] !== ROOT_SCRIPT) {
    errors.push('check:mobile-clients-foundation package script is missing or incorrect');
  }
  if (e2ePackage.scripts?.['test:mobile-clients-foundation'] !== E2E_SCRIPT) {
    errors.push('test:mobile-clients-foundation E2E package script is missing or incorrect');
  }
}

function validateRawGoBrowserFixtures(source, errors) {
  if (!/contact\s*:\s*['"]\*\*\/sidebar\/workContact\/detail\?\*['"]/.test(source)) {
    errors.push('contact raw Go envelope fixture endpoint is missing');
  }
  if (!/taskData\s*:\s*['"]\*\*\/operation\/workFission\/taskData\?\*['"]/.test(source)) {
    errors.push('taskData raw Go envelope fixture endpoint is missing');
  }
  if (!/JSON\.stringify\(\{\s*code\s*:\s*200\s*,\s*msg\s*:\s*['"]ok['"]\s*,\s*data\b[\s\S]{0,120}?\}\)/.test(source)) {
    errors.push('raw Go envelope helper must preserve code, msg and data fields');
  }
  if (
    !/const\s+rawContactData\s*=\s*\{[\s\S]{0,400}?\bid\s*:[\s\S]{0,200}?\bname\s*:[\s\S]{0,200}?\bavatar\s*:[\s\S]{0,200}?\bcorpId\s*:/.test(source)
    || !/page\.route\(\s*browserContract\.fixtures\.contact[\s\S]{0,700}?body\s*:\s*rawGoEnvelope\(\s*rawContactData\b/.test(source)
  ) {
    errors.push('contact route must return a raw Go envelope');
  }
  if (
    !/const\s+rawTaskData\s*=\s*\{[\s\S]{0,900}?\binvite_count\s*:[\s\S]{0,240}?\bdiffer_count\s*:[\s\S]{0,240}?\bend_time\s*:[\s\S]{0,300}?\btask\s*:[\s\S]{0,500}?\breceive_status\s*:[\s\S]{0,240}?\bgift_type\s*:[\s\S]{0,240}?\bgift_url\s*:/.test(source)
    || !/page\.route\(\s*browserContract\.fixtures\.taskData[\s\S]{0,700}?body\s*:\s*rawGoEnvelope\(\s*rawTaskData\b/.test(source)
  ) {
    errors.push('taskData route must return a raw Go envelope');
  }
}

function validateBrowserAudit(source, errors) {
  const collectors = [
    [/page\.on\(\s*['"]console['"][\s\S]{0,260}?message\.type\(\)\s*===\s*['"]error['"][\s\S]{0,180}?audit\.consoleErrors\.push\(/, 'console error audit collector is missing'],
    [/page\.on\(\s*['"]pageerror['"][\s\S]{0,220}?audit\.pageErrors\.push\(/, 'pageerror audit collector is missing'],
    [/page\.on\(\s*['"]requestfailed['"][\s\S]{0,260}?audit\.requestFailures\.push\(/, 'requestfailed audit collector is missing'],
    [/page\.on\(\s*['"]response['"][\s\S]{0,260}?response\.status\(\)\s*>=\s*400[\s\S]{0,220}?audit\.unexpectedResponses\.push\(/, '400 response audit collector is missing'],
  ];
  for (const [pattern, message] of collectors) {
    if (!pattern.test(source)) errors.push(message);
  }
  if ((source.match(/audit\.unexpectedRequests\.push\(/g) ?? []).length < 2) {
    errors.push('unexpected request audit collectors are missing');
  }

  const assertions = [
    ['consoleErrors', 'console error'],
    ['pageErrors', 'pageerror'],
    ['requestFailures', 'requestfailed'],
    ['unexpectedResponses', '400 response'],
    ['unexpectedRequests', 'unexpected request'],
  ];
  for (const [field, label] of assertions) {
    const assertion = new RegExp(`expect\\(audit\\.${field}(?:\\s*,[^)]*)?\\)\\.toEqual\\(\\[\\]\\)`);
    if (!assertion.test(source)) errors.push(`${label} clean-audit final assertion is missing`);
  }

  const stableAudit = /async\s+function\s+assertStableCleanAudit\s*\([^)]*\)\s*(?::\s*Promise<\s*void\s*>\s*)?\{[\s\S]{0,500}?await\s+page\.waitForLoadState\(\s*['"]networkidle['"]\s*\)[\s\S]{0,220}?await\s+page\.waitForTimeout\(\s*0\s*\)[\s\S]{0,220}?assertCleanAudit\(\s*audit/.test(source);
  if (!stableAudit) errors.push('networkidle and event-loop stabilization must precede clean audit assertions');
  if ((source.match(/await\s+assertStableCleanAudit\(/g) ?? []).length < 4) {
    errors.push('all known and unknown route groups must execute the stable clean audit');
  }
}

function validateBrowserLayout(source, sidebarCasesBody, operationCasesBody, errors) {
  if (
    !/clientWidth\s*:\s*document\.documentElement\.clientWidth/.test(source)
    || !/scrollWidth\s*:\s*document\.documentElement\.scrollWidth/.test(source)
    || !/expect\(pageShape\.scrollWidth(?:\s*,[^)]*)?\)\.toBeLessThanOrEqual\(pageShape\.clientWidth\)/.test(source)
  ) {
    errors.push('browser layout audit must assert scrollWidth is no greater than clientWidth');
  }

  const caseCount = countMatches(sidebarCasesBody, /\bpath\s*:/g)
    + countMatches(operationCasesBody, /\bpath\s*:/g);
  const actionFlagCount = countMatches(`${sidebarCasesBody}\n${operationCasesBody}`, /\bexpectsAction\s*:\s*(?:true|false)/g);
  if (actionFlagCount !== caseCount) errors.push('every browser route case must declare expectsAction');
  if (!/path\s*:\s*['"]\/login['"][^\n]*expectsAction\s*:\s*true/.test(sidebarCasesBody)) {
    errors.push('Sidebar login browser case must declare expectsAction true');
  }
  if (!/minimumActionHeight\s*:\s*44\b/.test(source)) {
    errors.push('browser contract must declare a 44px minimum action height');
  }
  if (!/if\s*\(\s*expectsAction\s*\)[\s\S]{0,180}?expect\(await\s+controls\.count\(\)\)\.toBeGreaterThanOrEqual\(1\)/.test(source)) {
    errors.push('expectsAction routes must enforce a visible control count of at least one');
  }
  if (!/expect\(box\?\.height\s*\?\?\s*0(?:\s*,[^)]*)?\)\.toBeGreaterThanOrEqual\(browserContract\.minimumActionHeight\)/.test(source)) {
    errors.push('visible action controls must enforce the 44px browser contract');
  }
}

function validateViewportRouteLoops(source, errors) {
  const viewportBody = braceBody(source, /for\s*\(const\s+viewport\s+of\s+viewports\)\s*\{/);
  if (viewportBody === null) {
    errors.push('viewport loop must cover all viewports and both route case arrays');
    return;
  }
  const sidebarBody = braceBody(viewportBody, /for\s*\(const\s+routeCase\s+of\s+sidebarCases\)\s*\{/);
  const operationBody = braceBody(viewportBody, /for\s*\(const\s+routeCase\s+of\s+operationCases\)\s*\{/);
  if (sidebarBody === null || operationBody === null) {
    errors.push('viewport loop must cover all viewports and both route case arrays');
    return;
  }
  for (const [label, loopBody] of [['Sidebar', sidebarBody], ['Operation', operationBody]]) {
    if (!/assertVisiblePage\([^;]+routeCase\.expectsAction\s*\)/.test(loopBody)) {
      errors.push(`${label} viewport loop must pass expectsAction to the layout assertion`);
    }
    if (!/await\s+assertStableCleanAudit\(/.test(loopBody)) {
      errors.push(`${label} viewport loop must execute the stable clean audit`);
    }
  }
}

export function auditMobileClientsFoundation(root = process.cwd()) {
  const resolvedRoot = resolve(root);
  const errors = [];
  const sidebarManifest = manifestPaths(
    readJSON(resolvedRoot, 'web/apps/sidebar/src/migration-routes.json'),
    'Sidebar',
    errors,
  );
  const operationManifest = manifestPaths(
    readJSON(resolvedRoot, 'web/apps/operation/src/migration-routes.json'),
    'Operation',
    errors,
  );

  const sidebarRegistry = registryPaths(
    readFileSync(join(resolvedRoot, 'web/apps/sidebar/src/routes/registry.tsx'), 'utf8'),
    'Sidebar',
    errors,
  );
  const operationRegistry = registryPaths(
    readFileSync(join(resolvedRoot, 'web/apps/operation/src/routes/registry.tsx'), 'utf8'),
    'Operation',
    errors,
  );
  compareRegistryToManifest('Sidebar', sidebarRegistry, sidebarManifest, errors);
  compareRegistryToManifest('Operation', operationRegistry, operationManifest, errors);

  const sources = [
    ...productionSources(resolvedRoot, 'web/apps/sidebar/src'),
    ...productionSources(resolvedRoot, 'web/apps/operation/src'),
  ];
  let directFetch = 0;
  let dashboardSessionReferences = 0;
  let mojibakeMarkers = 0;
  let fakeBusinessOutcomes = 0;
  for (const file of sources) {
    const fetchCount = countMatches(file.source, /\bfetch\s*\(/g);
    if (fetchCount > 0) errors.push(`direct fetch found in production source ${file.path}`);
    directFetch += fetchCount;

    const dashboardCount = countMatches(file.source, /\bmochat_dashboard_[a-z\d_]+\b/gi);
    if (dashboardCount > 0) errors.push(`Dashboard session reference found in ${file.path}`);
    dashboardSessionReferences += dashboardCount;

    const mojibakeCount = countMatches(file.source, MOJIBAKE_PATTERN);
    if (mojibakeCount > 0) errors.push(`mojibake marker found in ${file.path}`);
    mojibakeMarkers += mojibakeCount;

    const fakeCount = FAKE_OUTCOME_PATTERNS.reduce(
      (count, pattern) => count + countMatches(file.source, pattern),
      0,
    );
    if (fakeCount > 0) errors.push(`fake business outcome found in ${file.path}`);
    fakeBusinessOutcomes += fakeCount;
  }

  validateUnknownRoute(
    'Sidebar',
    readFileSync(join(resolvedRoot, 'web/apps/sidebar/src/app/sidebar-router.tsx'), 'utf8'),
    errors,
  );
  validateUnknownRoute(
    'Operation',
    readFileSync(join(resolvedRoot, 'web/apps/operation/src/app/operation-router.tsx'), 'utf8'),
    errors,
  );

  const e2eSource = readFileSync(
    join(resolvedRoot, 'web/e2e/tests/mobile-clients-foundation.spec.ts'),
    'utf8',
  );
  const sidebarCasesBody = specArrayBody(e2eSource, 'sidebarCases', errors);
  const operationCasesBody = specArrayBody(e2eSource, 'operationCases', errors);
  const sidebarCases = specCasePaths(e2eSource, 'sidebarCases', errors);
  const operationCases = specCasePaths(e2eSource, 'operationCases', errors);
  compareBrowserCases('Sidebar', sidebarCases, sidebarManifest, errors);
  compareBrowserCases('Operation', operationCases, operationManifest, errors);

  const hasMobileViewport = /width\s*:\s*390\b[\s\S]{0,80}?height\s*:\s*844\b/.test(e2eSource);
  const hasDesktopViewport = /width\s*:\s*1280\b[\s\S]{0,80}?height\s*:\s*900\b/.test(e2eSource);
  if (!hasMobileViewport) errors.push('browser spec must include the 390 by 844 viewport');
  if (!hasDesktopViewport) errors.push('browser spec must include the 1280 by 900 viewport');
  validateRawGoBrowserFixtures(e2eSource, errors);
  validateBrowserAudit(e2eSource, errors);
  validateBrowserLayout(e2eSource, sidebarCasesBody, operationCasesBody, errors);
  validateViewportRouteLoops(e2eSource, errors);
  if (
    !e2eSource.includes('/sidebar-app/not-a-sidebar-page')
    || !e2eSource.includes('/operation-app/not-an-operation-page')
    || !e2eSource.includes('页面不存在')
    || !e2eSource.includes('toHaveCount(0)')
  ) {
    errors.push('browser spec must prove unknown routes show 404 without home content');
  }

  validatePackageScripts(resolvedRoot, errors);
  if (errors.length > 0) {
    throw new Error(`mobile clients foundation gate failed:\n- ${errors.join('\n- ')}`);
  }

  return {
    sidebarRoutes: sidebarManifest.length,
    operationRoutes: operationManifest.length,
    directFetch,
    dashboardSessionReferences,
    mojibakeMarkers,
    fakeBusinessOutcomes,
    mobileViewportCases: hasMobileViewport ? sidebarCases.length + operationCases.length : 0,
  };
}

export function formatMobileClientsFoundationSummary(result) {
  return [
    `sidebar routes=${result.sidebarRoutes}`,
    `operation routes=${result.operationRoutes}`,
    `direct fetch=${result.directFetch}`,
    `dashboard session references=${result.dashboardSessionReferences}`,
    `mojibake markers=${result.mojibakeMarkers}`,
    `fake business outcomes=${result.fakeBusinessOutcomes}`,
    `mobile viewport cases>=${result.mobileViewportCases}`,
  ].join('\n');
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  console.log(formatMobileClientsFoundationSummary(auditMobileClientsFoundation()));
}
