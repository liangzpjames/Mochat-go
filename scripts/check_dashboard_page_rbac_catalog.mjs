import { readFile } from 'node:fs/promises';
import { readdir } from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const SUPERADMIN_ONLY_PATHS = new Set([
  '/company-setting/staff',
  '/setting/role',
  '/setting/additional',
  '/setting/authorization',
]);

function assertUnique(values, label) {
  const seen = new Set();
  for (const value of values) {
    if (seen.has(value)) {
      throw new Error(`duplicate ${label}: ${value}`);
    }
    seen.add(value);
  }
}

function resourceKey(resource) {
  if (!resource || typeof resource !== 'object') {
    throw new Error('catalog resource must be an object');
  }
  if (typeof resource.scopeRequired !== 'boolean') {
    throw new Error('resource scopeRequired must be boolean');
  }
  const method = String(resource.method ?? '').trim().toUpperCase();
  const pathPattern = String(resource.pathPattern ?? '').trim();
  if (!/^[A-Z]+$/.test(method) || !pathPattern.startsWith('/dashboard/')) {
    throw new Error(`invalid catalog resource: ${method} ${pathPattern}`);
  }
  return `${method} ${pathPattern}`;
}

function sameSet(left, right) {
  return left.size === right.size && [...left].every((value) => right.has(value));
}

async function sourceFiles(root, extensions) {
  const files = [];
  for (const entry of await readdir(root, { withFileTypes: true })) {
    const target = path.join(root, entry.name);
    if (entry.isDirectory()) {
      files.push(...await sourceFiles(target, extensions));
    } else if (extensions.some((extension) => entry.name.endsWith(extension))
      && !entry.name.includes('.test.') && !entry.name.includes('.spec.')) {
      files.push(target);
    }
  }
  return files;
}

function normalizeFrontendPath(rawPath) {
  let value = rawPath.replace(/\$\{[^}]+\}/g, '{id}').split('?')[0];
  if (!value.startsWith('/') || value.startsWith('/dashboard/')) return value;
  value = `/dashboard${value}`;
  return value.replace(/\/{id}(?=\/{id})/g, '/{id}');
}

function normalizeFrontendPaths(rawPath, corpus) {
  if (rawPath.includes('${page}')) {
    const pages = [...corpus.matchAll(/\bpage="([a-z0-9-]+)"/g)].map((match) => match[1]);
    return [...new Set(pages)].map((pageName) => normalizeFrontendPath(rawPath.replace('${page}', pageName)));
  }
  return [normalizeFrontendPath(rawPath)];
}

function methodsNearCall(tail, fallback) {
  const methods = [...tail.matchAll(/['"](GET|POST|PUT|PATCH|DELETE)['"]/gi)]
    .map((match) => match[1].toUpperCase());
  if (methods.length === 0 && /^\s*,\s*(?:json|jsonRequest)\s*\(/.test(tail)) methods.push('POST');
  return [...new Set(methods.length > 0 ? methods : [fallback.toUpperCase()])];
}

function matchingDelimiter(source, openingIndex, open, close) {
  let depth = 0;
  let state = 'code';
  for (let index = openingIndex; index < source.length; index += 1) {
    const character = source[index];
    if (state !== 'code') {
      if (character === '\\') index += 1;
      else if ((state === 'single' && character === "'")
        || (state === 'double' && character === '"')
        || (state === 'template' && character === '`')) state = 'code';
    } else if (character === "'") state = 'single';
    else if (character === '"') state = 'double';
    else if (character === '`') state = 'template';
    else if (character === open) depth += 1;
    else if (character === close && --depth === 0) return index;
  }
  return source.length - 1;
}

export function extractFrontendAPIUsages(sourceBodies) {
  const usages = new Set();
  const corpus = sourceBodies.join('\n');
  for (const body of sourceBodies) {
    const variables = new Map();
    for (const match of body.matchAll(/\bconst\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([^;]+);/g)) {
      const paths = [...match[2].matchAll(/([`'"])(\/[^`'"\r\n]+)\1/g)].map((item) => item[2]);
      if (paths.length > 0) variables.set(match[1], paths);
    }
    for (const match of body.matchAll(/\b(readEndpoint|writeEndpoint)\s*:\s*([`'"])(\/[^`'"\r\n]+)\2/g)) {
      const fallback = match[1] === 'readEndpoint' ? 'GET' : 'POST';
      for (const normalizedPath of normalizeFrontendPaths(match[3], corpus)) {
        usages.add(`${fallback} ${normalizedPath}`);
      }
    }
    const callPattern = /(client\.request(?:<[^;()]+>)?|api\.(read|write))\s*\(/g;
    for (const match of body.matchAll(callPattern)) {
      const argumentStart = match.index + match[0].length;
      let cursor = argumentStart;
      let state = 'code';
      let nested = 0;
      for (; cursor < body.length; cursor += 1) {
        const character = body[cursor];
        if (state !== 'code') {
          if (character === '\\') cursor += 1;
          else if ((state === 'single' && character === "'")
            || (state === 'double' && character === '"')
            || (state === 'template' && character === '`')) state = 'code';
        } else if (character === "'") state = 'single';
        else if (character === '"') state = 'double';
        else if (character === '`') state = 'template';
        else if ('([{'.includes(character)) nested += 1;
        else if (')]}'.includes(character)) {
          if (nested === 0) break;
          nested -= 1;
        } else if (character === ',' && nested === 0) break;
      }
      const expression = body.slice(argumentStart, cursor).trim();
      let rawPaths = [...expression.matchAll(/([`'"])(\/[^`'"\r\n]+)\1/g)]
        .map((item) => item[2]);
      if (rawPaths.length === 0 && variables.has(expression)) rawPaths = variables.get(expression);
      const callEnd = matchingDelimiter(body, argumentStart - 1, '(', ')');
      const tail = body.slice(cursor, callEnd + 1);
      const fallback = match[2] === 'write' ? 'POST' : 'GET';
      for (const rawPath of rawPaths) {
        if (rawPath.length > 240) continue;
        for (const normalizedPath of normalizeFrontendPaths(rawPath, corpus)) {
          for (const method of methodsNearCall(tail, fallback)) usages.add(`${method} ${normalizedPath}`);
        }
      }
    }
  }
  return [...usages].sort();
}

function moduleKey(file) {
  return file.replace(/\\/g, '/').replace(/\.(?:ts|tsx)$/, '').replace(/\/index$/, '');
}

export async function productionDashboardSourceFiles() {
  const allFiles = await sourceFiles('web/apps/dashboard/src', ['.ts', '.tsx']);
  const byModule = new Map();
  for (const file of allFiles) {
    byModule.set(moduleKey(file), file);
    byModule.set(moduleKey(file).replace(/\/index$/, ''), file);
  }
  const entry = path.normalize('web/apps/dashboard/src/benchmark/page-registry.tsx');
  const mainFile = path.normalize('web/apps/dashboard/src/main.tsx');
  const mainBody = await readFile(mainFile, 'utf8');
  const seeds = new Set([entry]);
  for (const shellModule of [
    'web/apps/dashboard/src/app/access-loader',
    'web/apps/dashboard/src/features/auth/auth-api',
    'web/apps/dashboard/src/features/auth/session-actions',
    'web/apps/dashboard/src/features/company-settings/company-profile-api',
    'web/apps/dashboard/src/features/navigation/menu-api',
  ]) {
    const target = byModule.get(moduleKey(path.normalize(shellModule)));
    if (!target) throw new Error(`missing production Dashboard shell module: ${shellModule}`);
    seeds.add(target);
  }
  const call = mainBody.match(/createBenchmarkP0Pages\(\{([^}]+)\}\)/s);
  const apiNames = call?.[1].split(',').map((value) => value.trim()).filter(Boolean) ?? [];
  for (const apiName of apiNames) {
    const assignment = mainBody.match(new RegExp(`const\\s+${apiName}\\s*=\\s*([A-Za-z0-9_]+)\\(`));
    const creator = assignment?.[1];
    if (!creator) continue;
    for (const imported of mainBody.matchAll(/import\s+[\s\S]*?\sfrom\s+['"]([^'"]+)['"];?/g)) {
      if (!imported[0].includes(creator) || !imported[1].startsWith('.')) continue;
      const key = moduleKey(path.normalize(path.join(path.dirname(mainFile), imported[1])));
      const target = byModule.get(key);
      if (target) seeds.add(target);
    }
  }

  const visited = new Set();
  const queue = [...seeds];
  while (queue.length > 0) {
    const file = queue.pop();
    if (!file || visited.has(file)) continue;
    visited.add(file);
    const body = await readFile(file, 'utf8');
    for (const imported of body.matchAll(/(?:import|export)\s+[\s\S]*?\sfrom\s+['"]([^'"]+)['"];?/g)) {
      if (!imported[1].startsWith('.')) continue;
      const key = moduleKey(path.normalize(path.join(path.dirname(file), imported[1])));
      const target = byModule.get(key);
      if (target && !visited.has(target)) queue.push(target);
    }
  }
  return [...visited];
}

export async function scanFrontendAPIUsages() {
  const files = await productionDashboardSourceFiles();
  const bodies = await Promise.all(files.map((file) => readFile(file, 'utf8')));
  return extractFrontendAPIUsages(bodies);
}

function goMethod(expression) {
  const literal = expression.match(/['"](GET|POST|PUT|PATCH|DELETE)['"]/i);
  if (literal) return literal[1].toUpperCase();
  const named = expression.match(/Method(Get|Post|Put|Patch|Delete)/);
  return named ? named[1].toUpperCase() : null;
}

function resolveGoString(expression, constants) {
  const parts = expression.trim().split('+').map((part) => part.trim());
  let value = '';
  for (const part of parts) {
    const literal = part.match(/^['"]([^'"]*)['"]$/);
    if (literal) value += literal[1];
    else if (constants.has(part)) value += constants.get(part);
    else if (part.includes('.') && constants.has(part.slice(part.lastIndexOf('.') + 1))) {
      value += constants.get(part.slice(part.lastIndexOf('.') + 1));
    }
    else return null;
  }
  return value;
}

function stripGoComments(source) {
  let result = '';
  let state = 'code';
  for (let index = 0; index < source.length; index += 1) {
    const current = source[index];
    const next = source[index + 1];
    if (state === 'line-comment') {
      if (current === '\n') {
        result += current;
        state = 'code';
      } else result += ' ';
    } else if (state === 'block-comment') {
      if (current === '*' && next === '/') {
        result += '  ';
        index += 1;
        state = 'code';
      } else result += current === '\n' ? '\n' : ' ';
    } else if (state === 'quoted') {
      result += current;
      if (current === '\\') {
        result += next ?? '';
        index += 1;
      } else if (current === '"') state = 'code';
    } else if (state === 'raw') {
      result += current;
      if (current === '`') state = 'code';
    } else if (state === 'rune') {
      result += current;
      if (current === '\\') {
        result += next ?? '';
        index += 1;
      } else if (current === "'") state = 'code';
    } else if (current === '/' && next === '/') {
      result += '  ';
      index += 1;
      state = 'line-comment';
    } else if (current === '/' && next === '*') {
      result += '  ';
      index += 1;
      state = 'block-comment';
    } else {
      result += current;
      if (current === '"') state = 'quoted';
      else if (current === '`') state = 'raw';
      else if (current === "'") state = 'rune';
    }
  }
  return result;
}

function lineAt(source, index) {
  return source.slice(0, index).split('\n').length;
}

function matchingBrace(source, openingIndex) {
  let depth = 0;
  let state = 'code';
  for (let index = openingIndex; index < source.length; index += 1) {
    const current = source[index];
    if (state === 'quoted') {
      if (current === '\\') index += 1;
      else if (current === '"') state = 'code';
    } else if (state === 'raw') {
      if (current === '`') state = 'code';
    } else if (state === 'rune') {
      if (current === '\\') index += 1;
      else if (current === "'") state = 'code';
    } else if (current === '"') state = 'quoted';
    else if (current === '`') state = 'raw';
    else if (current === "'") state = 'rune';
    else if (current === '{') depth += 1;
    else if (current === '}' && --depth === 0) return index;
  }
  return -1;
}

function stringListValues(expression, constants) {
  const values = [];
  for (const item of expression.split(',')) {
    const value = resolveGoString(item, constants);
    if (value !== null) values.push(value);
    else {
      const method = goMethod(item);
      if (method) values.push(method);
    }
  }
  return values;
}

function loopBindings(source, constants) {
  const loops = [];
  const pattern = /for\s+_,\s*([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*range\s*\[\]string\s*\{([^}]*)\}\s*\{/g;
  for (const match of source.matchAll(pattern)) {
    const opening = match.index + match[0].lastIndexOf('{');
    const end = matchingBrace(source, opening);
    if (end >= 0) {
      loops.push({
        name: match[1],
        values: stringListValues(match[2], constants),
        start: opening,
        end,
      });
    }
  }
  return loops;
}

function expressionValues(expression, kind, constants, loops, index) {
  const direct = kind === 'method' ? goMethod(expression) : resolveGoString(expression, constants);
  if (direct !== null) return [direct];
  const loop = loops.find((candidate) => candidate.name === expression.trim()
    && candidate.start < index && index < candidate.end);
  if (loop) return kind === 'method'
    ? loop.values.map((value) => goMethod(value) ?? value.toUpperCase())
    : loop.values;
  const containingLoop = loops.find((candidate) => expression.includes(candidate.name)
    && candidate.start < index && index < candidate.end);
  if (!containingLoop || kind === 'method') return [];
  return containingLoop.values
    .map((value) => resolveGoString(expression.replaceAll(containingLoop.name, JSON.stringify(value)), constants))
    .filter(Boolean);
}

function collectGoConstants(sources) {
  const constants = new Map();
  let changed = true;
  while (changed) {
    changed = false;
    for (const { body } of sources) {
      for (const match of body.matchAll(/^\s*(?:const\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*(?:string\s*)?=\s*([^\n]+)$/gm)) {
        if (constants.has(match[1])) continue;
        const resolved = resolveGoString(match[2], constants);
        if (resolved !== null) {
          constants.set(match[1], resolved);
          changed = true;
        }
      }
    }
  }
  return constants;
}

export function extractBackendRegisteredAPIs(sourceBodies) {
  const sources = sourceBodies.map((source, index) => ({
    file: typeof source === 'string' ? `<source-${index + 1}>` : source.file,
    body: stripGoComments(typeof source === 'string' ? source : source.body),
  }));
  const constants = collectGoConstants(sources);
  const routes = new Map();
  const addRoute = (method, routePath, file, source, index) => {
    if (!method || !routePath?.startsWith('/dashboard/')) return;
    const contract = `${method.toUpperCase()} ${routePath}`;
    if (!routes.has(contract)) routes.set(contract, { contract, file, line: lineAt(source, index) });
  };

  for (const { file, body: source } of sources) {
    for (const match of source.matchAll(/^\s*case\s+(.+):\s*$/gm)) {
      const methods = [...match[1].matchAll(/r\.Method\s*==\s*((?:[A-Za-z_][A-Za-z0-9_]*\.)?Method(?:Get|Post|Put|Patch|Delete)|['"][A-Z]+['"])/g)]
        .map((item) => goMethod(item[1])).filter(Boolean);
      const paths = [...match[1].matchAll(/r\.URL\.Path\s*==\s*['"](\/dashboard\/[^'"]+)['"]/g)]
        .map((item) => item[1]);
      for (const method of methods) for (const routePath of paths) {
        addRoute(method, routePath, file, source, match.index);
      }
    }

    const loops = loopBindings(source, constants);
    for (const match of source.matchAll(/(?:registrar|router)\.Handle\(\s*([^,]+),\s*([^,]+),/g)) {
      if (match[1].trim() === 'route.method' && match[2].trim() === 'route.path') continue;
      const methods = expressionValues(match[1], 'method', constants, loops, match.index);
      const paths = expressionValues(match[2], 'path', constants, loops, match.index);
      for (const method of methods) for (const routePath of paths) {
        addRoute(method, routePath, file, source, match.index);
      }
    }

    if (/registrar\.Handle\(\s*route\.method\s*,\s*route\.path\s*,/.test(source)) {
      for (const match of source.matchAll(/\{\s*((?:[A-Za-z_][A-Za-z0-9_]*\.)?Method(?:Get|Post|Put|Patch|Delete)|['"][A-Z]+['"]),\s*([^,\n]+),\s*[^,\n]+\}/g)) {
        const methods = expressionValues(match[1], 'method', constants, [], match.index);
        const paths = expressionValues(match[2], 'path', constants, [], match.index);
        for (const method of methods) for (const routePath of paths) {
          addRoute(method, routePath, file, source, match.index);
        }
      }
    }
  }
  return [...routes.values()].sort((left, right) => left.contract.localeCompare(right.contract));
}

export function extractGoDashboardRoutePolicy(source) {
  const body = stripGoComments(source);
  const contracts = (name) => {
    const block = body.match(new RegExp(`var\\s+${name}\\s*=\\s*\\[\\]string\\s*\\{([\\s\\S]*?)\\n\\}`));
    if (!block) throw new Error(`missing Go dashboard route policy: ${name}`);
    return [...block[1].matchAll(/['"]((?:GET|POST|PUT|PATCH|DELETE) \/dashboard\/[^'"]+)['"]/g)]
      .map((match) => match[1]);
  };
  return {
    exactExempt: contracts('exactExemptDashboardRouteContracts'),
    denyOnly: contracts('denyOnlyDashboardRouteContracts'),
  };
}

export function extractMigrationPermissionResourceMappings(source) {
  const mappings = [];
  for (const match of source.matchAll(/(?:SELECT|UNION ALL SELECT)\s+'([^']+)'(?:\s+AS `permission_code`)?\s*,\s*'(GET|POST|PUT|PATCH|DELETE)'(?:\s+AS `http_method`)?\s*,\s*'(\/dashboard\/[^']+)'(?:\s+AS `path_pattern`)?\s*,\s*([01])(?:\s+AS `scope_required`)?/g)) {
    mappings.push(`${match[1]}\t${match[2]} ${match[3]}\t${match[4]}`);
  }
  return mappings;
}

export async function scanBackendRegisteredAPIs() {
  const internalFiles = await sourceFiles('internal', ['.go']);
  const compositionFiles = await sourceFiles(path.join('cmd', 'mochat-go'), ['.go']);
  const production = [
    ...internalFiles.filter((file) => !file.endsWith('_test.go') && (file.includes(`${path.sep}server${path.sep}`)
      || file.includes(`${path.sep}modules${path.sep}`))),
    ...compositionFiles.filter((file) => !file.endsWith('_test.go')),
  ];
  const sources = await Promise.all(production.map(async (file) => ({
    file: file.replace(/\\/g, '/'),
    body: await readFile(file, 'utf8'),
  })));
  return extractBackendRegisteredAPIs(sources);
}

function routePatternCovers(registeredContract, resourceContract) {
  const registeredSpace = registeredContract.indexOf(' ');
  const resourceSpace = resourceContract.indexOf(' ');
  if (registeredContract.slice(0, registeredSpace) !== resourceContract.slice(0, resourceSpace)) {
    return false;
  }
  const registeredSegments = registeredContract.slice(registeredSpace + 1).split('/');
  const resourceSegments = resourceContract.slice(resourceSpace + 1).split('/');
  return registeredSegments.length === resourceSegments.length
    && registeredSegments.every((segment, index) => /^\{[^/{}]+\}$/.test(segment)
      || segment === resourceSegments[index]);
}

function isDashboardRBACRoute(contract) {
  const routePath = contract.slice(contract.indexOf(' ') + 1);
  return !routePath.startsWith('/dashboard/saasAdmin/')
    && !routePath.startsWith('/dashboard/saasAlert/')
    && !routePath.startsWith('/dashboard/saasBilling/');
}

export function validateDashboardPageRBACCatalog({
  manifest,
  catalog,
  apiUsages = [],
  registeredAPIs = [],
  registeredSources = new Map(),
  exemptions = [],
  denyOnly = [],
  seededMappings,
}) {
  if (!Array.isArray(manifest?.pages) || !Array.isArray(catalog)) {
    throw new Error('manifest.pages and catalog must be arrays');
  }
  if (manifest.pages.length !== 53 || catalog.length !== 53) {
    throw new Error('catalog paths must exactly match manifest paths');
  }

  const manifestPaths = manifest.pages.map((page) => page.path);
  const catalogPaths = catalog.map((page) => page.path);
  assertUnique(manifestPaths, 'manifest path');
  assertUnique(catalogPaths, 'catalog path');
  if (!sameSet(new Set(manifestPaths), new Set(catalogPaths))) {
    throw new Error('catalog paths must exactly match manifest paths');
  }

  const protectedPaths = new Set(
    catalog.filter((page) => page.superadminOnly === true).map((page) => page.path),
  );
  if (!sameSet(protectedPaths, SUPERADMIN_ONLY_PATHS)) {
    throw new Error('superadmin_only paths must exactly match protected management paths');
  }

  const mappedResources = [];
  const catalogMappings = [];
  for (const page of catalog) {
    if (!Array.isArray(page.resources)) {
      throw new Error(`catalog page resources must be an array: ${page.path}`);
    }
    const pageResources = page.resources.map(resourceKey);
    assertUnique(pageResources, 'resource mapping');
    for (const resource of pageResources) {
      mappedResources.push(resource);
      const source = page.resources.find((candidate) => resourceKey(candidate) === resource);
      catalogMappings.push(`${page.code}\t${resource}\t${source.scopeRequired ? 1 : 0}`);
    }
  }
  if (seededMappings !== undefined) {
    assertUnique(seededMappings, 'migration resource seed');
    if (!sameSet(new Set(catalogMappings), new Set(seededMappings))) {
      throw new Error('migration permission resource seed must exactly match catalog');
    }
  }

  const mapped = new Set(mappedResources);
  const registered = new Set(registeredAPIs);
  const exempt = new Set(exemptions);
  const denied = new Set(denyOnly);
  for (const route of exempt) {
    if (denied.has(route)) throw new Error(`dashboard route has conflicting policy classes: ${route}`);
  }
  for (const route of denied) {
    if ([...mapped].some((resource) => routePatternCovers(route, resource))) {
      throw new Error(`deny-only dashboard route is mapped to a page: ${route}`);
    }
  }
  for (const usage of apiUsages) {
    // A deny-only endpoint may be consumed by an explicitly protected page;
    // it remains outside the page permission mapping and is still rejected by
    // the runtime superadmin/tenant guard.
    if (!mapped.has(usage) && !exempt.has(usage) && !denied.has(usage)) {
      throw new Error(`unmapped dashboard api usage: ${usage}`);
    }
  }
  for (const resource of mapped) {
    if (![...registered].some((route) => routePatternCovers(route, resource))) {
      throw new Error(`mapped dashboard api is not registered by server: ${resource}`);
    }
  }
  for (const route of registered) {
    if (![...mapped].some((resource) => routePatternCovers(route, resource))
      && !exempt.has(route) && !denied.has(route)) {
      const source = registeredSources.get(route);
      throw new Error(`unmapped registered dashboard api: ${route}${source ? ` (${source})` : ''}`);
    }
  }
  for (const route of [...exempt, ...denied]) {
    if (!registered.has(route)) throw new Error(`classified dashboard route is not registered: ${route}`);
  }

  return {
    pageCount: catalog.length,
    ordinaryPageCount: catalog.length - protectedPaths.size,
    superadminOnlyCount: protectedPaths.size,
    resourceCount: mapped.size,
    unmappedAPIUsageCount: 0,
  };
}

async function main() {
  const manifest = JSON.parse(
    await readFile('web/apps/dashboard/src/benchmark/manifest.json', 'utf8'),
  );
  const document = JSON.parse(
    await readFile('internal/dashboard/dashboard_page_catalog.json', 'utf8'),
  );
  const routePolicy = extractGoDashboardRoutePolicy(
    await readFile('internal/dashboard/dashboard_route_policy.go', 'utf8'),
  );
  const seededMappings = extractMigrationPermissionResourceMappings(
    await readFile('deploy/standalone/migrations/0127_dashboard_page_rbac.up.sql', 'utf8'),
  );
  const apiUsages = await scanFrontendAPIUsages();
  const backendRoutes = (await scanBackendRegisteredAPIs())
    .filter((route) => isDashboardRBACRoute(route.contract));
  const registeredAPIs = backendRoutes.map((route) => route.contract);
  const registeredSources = new Map(backendRoutes.map((route) => [
    route.contract,
    `${route.file}:${route.line}`,
  ]));
  const result = validateDashboardPageRBACCatalog({
    manifest,
    catalog: document.pages,
    apiUsages,
    registeredAPIs,
    registeredSources,
    exemptions: routePolicy.exactExempt,
    denyOnly: routePolicy.denyOnly,
    seededMappings,
  });
  console.log(
    `${result.pageCount} pages, ${result.ordinaryPageCount} ordinary, `
      + `${result.superadminOnlyCount} superadmin_only, `
      + `${result.unmappedAPIUsageCount} unmapped dashboard API usages`,
  );
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
