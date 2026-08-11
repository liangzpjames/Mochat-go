import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { extractBackendRegisteredAPIs } from './check_dashboard_page_rbac_catalog.mjs';

const GO_EXT = '.go';
const FRONTEND_EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx'];
const ignoredName = (name) => /(?:\.test|\.spec)\.[^.]+$/.test(name) || name.endsWith('_test.go');
const principalConsumerPattern = /DashboardPrincipalFromContext|RequireDashboardPrincipal/;

function walk(directory, predicate = () => true) {
  if (!fs.existsSync(directory)) return [];
  const entries = fs.readdirSync(directory, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const full = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (/^(?:node_modules|vendor|dist|build|fixtures?|migrations?)$/i.test(entry.name)) continue;
      files.push(...walk(full, predicate));
    } else if (predicate(full, entry.name)) {
      files.push(full);
    }
  }
  return files;
}

function stripComments(source, extension) {
  if (extension === GO_EXT) {
    return source
      .replace(/\/\*[\s\S]*?\*\//g, (value) => value.replace(/[^\n]/g, ' '))
      .replace(/(^|\s)\/\/.*$/gm, '$1');
  }
  return source
    .replace(/\/\*[\s\S]*?\*\//g, (value) => value.replace(/[^\n]/g, ' '))
    .replace(/(^|\s)\/\/.*$/gm, '$1');
}

function lineAt(source, index) {
  return source.slice(0, index).split('\n').length;
}

function matchingDelimiter(source, openingIndex, opening = '(', closing = ')') {
  let depth = 0;
  let state = 'code';
  for (let index = openingIndex; index < source.length; index += 1) {
    const current = source[index];
    if (state === 'quoted') {
      if (current === '\\') index += 1;
      else if (current === '"') state = 'code';
      continue;
    }
    if (state === 'raw') {
      if (current === '`') state = 'code';
      continue;
    }
    if (state === 'rune') {
      if (current === '\\') index += 1;
      else if (current === "'") state = 'code';
      continue;
    }
    if (current === '"') state = 'quoted';
    else if (current === '`') state = 'raw';
    else if (current === "'") state = 'rune';
    else if (current === opening) depth += 1;
    else if (current === closing && --depth === 0) return index;
  }
  return -1;
}

function splitTopLevel(source) {
  const parts = [];
  let start = 0;
  let parentheses = 0;
  let brackets = 0;
  let braces = 0;
  let state = 'code';
  for (let index = 0; index < source.length; index += 1) {
    const current = source[index];
    if (state === 'quoted') {
      if (current === '\\') index += 1;
      else if (current === '"') state = 'code';
      continue;
    }
    if (state === 'raw') {
      if (current === '`') state = 'code';
      continue;
    }
    if (state === 'rune') {
      if (current === '\\') index += 1;
      else if (current === "'") state = 'code';
      continue;
    }
    if (current === '"') state = 'quoted';
    else if (current === '`') state = 'raw';
    else if (current === "'") state = 'rune';
    else if (current === '(') parentheses += 1;
    else if (current === ')') parentheses -= 1;
    else if (current === '[') brackets += 1;
    else if (current === ']') brackets -= 1;
    else if (current === '{') braces += 1;
    else if (current === '}') braces -= 1;
    else if (current === ',' && parentheses === 0 && brackets === 0 && braces === 0) {
      parts.push(source.slice(start, index).trim());
      start = index + 1;
    }
  }
  parts.push(source.slice(start).trim());
  return parts.filter(Boolean);
}

function locationFor(file, source, pattern, label = '') {
  const match = source.match(pattern);
  if (!match) return null;
  const line = lineAt(source, match.index);
  return { file: file.replaceAll('\\', '/'), line, match: label || match[0] };
}

function locationsFor(files, pattern, label = '') {
  const locations = [];
  for (const file of files) {
    const extension = path.extname(file);
    const source = stripComments(fs.readFileSync(file, 'utf8'), extension);
    const location = locationFor(file, source, pattern, label);
    if (location) locations.push(location);
  }
  return locations;
}

function productionGoFiles(root) {
  return walk(path.join(root, 'internal'), (file, name) => file.endsWith(GO_EXT) && !ignoredName(name))
    .concat(walk(path.join(root, 'cmd'), (file, name) => file.endsWith(GO_EXT) && !ignoredName(name)));
}

function productionRouteGoFiles(root) {
  const routeRoots = [
    path.join(root, 'internal', 'server'),
    path.join(root, 'internal', 'modules'),
    path.join(root, 'cmd', 'mochat-go'),
  ];
  return routeRoots.flatMap((directory) => walk(directory, (file, name) => file.endsWith(GO_EXT) && !ignoredName(name)));
}

function resolveImport(file, specifier) {
  if (!specifier.startsWith('.')) return null;
  const base = path.resolve(path.dirname(file), specifier);
  const candidates = [
    base,
    ...FRONTEND_EXTENSIONS.map((extension) => `${base}${extension}`),
    ...FRONTEND_EXTENSIONS.map((extension) => path.join(base, `index${extension}`)),
  ];
  return candidates.find((candidate) => fs.existsSync(candidate) && fs.statSync(candidate).isFile()) || null;
}

function frontendProductionFiles(root, app) {
  const sourceRoot = path.join(root, 'web', 'apps', app, 'src');
  const mainCandidates = ['main.tsx', 'main.ts', 'main.jsx', 'main.js'].map((name) => path.join(sourceRoot, name));
  const main = mainCandidates.find((file) => fs.existsSync(file) && fs.statSync(file).isFile());
  if (!main) {
    throw new Error(`missing production entry: ${app}/src/main.tsx|main.ts|main.jsx|main.js`);
  }
  const appCandidates = ['App.tsx', 'App.ts', 'App.jsx', 'App.js']
    .map((name) => path.join(sourceRoot, name))
    .filter((file) => fs.existsSync(file) && fs.statSync(file).isFile());
  const queue = [main, ...appCandidates];
  const seen = new Set();
  while (queue.length) {
    const file = queue.shift();
    if (!file || seen.has(file)) continue;
    seen.add(file);
    const source = stripComments(fs.readFileSync(file, 'utf8'), path.extname(file));
    const imports = [
      ...source.matchAll(/(?:from\s*|import\s*\()(['"])([^'"]+)\1/g),
      ...source.matchAll(/^\s*import\s*(['"])([^'"]+)\1\s*;?/gm),
    ];
    for (const match of imports) {
      const imported = resolveImport(file, match[2]);
      if (imported && !ignoredName(path.basename(imported))) queue.push(imported);
    }
  }
  return [...seen];
}

function assertNo(evidence, message) {
  if (!evidence.length) return;
  const details = evidence.map(({ file, line, match }) => `${file}:${line}${match ? ` (${match})` : ''}`).join(', ');
  throw new Error(`${message}: ${details}`);
}

function requireEvidence(evidence, message) {
  if (evidence.length) return;
  throw new Error(`${message}: no production source evidence`);
}

function goMethod(expression, constants = new Map()) {
  const value = expression.trim().replace(/^\((.*)\)$/, '$1').trim();
  if (/^['"][A-Z]+['"]$/.test(value)) return value.slice(1, -1);
  const constant = constants.get(value);
  if (constant && /^[A-Z]+$/.test(constant)) return constant;
  const method = value.match(/(?:^|\.)(Method(?:Get|Post|Put|Patch|Delete))$/);
  if (method) return method[1].slice('Method'.length).toUpperCase();
  return null;
}

function goString(expression, constants = new Map()) {
  const value = expression.trim().replace(/^\((.*)\)$/, '$1').trim();
  const quoted = value.match(/^['"](.*)['"]$/s);
  if (quoted) return quoted[1];
  if (constants.has(value)) return constants.get(value);
  if (value.includes('+')) {
    const parts = splitTopLevel(value.replaceAll('+', ','));
    const resolved = parts.map((part) => goString(part, constants));
    if (resolved.every((part) => part !== null)) return resolved.join('');
  }
  return null;
}

function collectGoConstants(sources) {
  const constants = new Map();
  let changed = true;
  while (changed) {
    changed = false;
    for (const { body } of sources) {
      for (const match of body.matchAll(/^\s*(?:const\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*(?:string\s*)?=\s*([^\n]+)$/gm)) {
        if (constants.has(match[1])) continue;
        const resolved = goString(match[2], constants);
        if (resolved !== null) {
          constants.set(match[1], resolved);
          changed = true;
        }
      }
    }
  }
  return constants;
}

function goFunctionBodies(file, source) {
  const functions = [];
  const pattern = /func\s+(?:\(\s*([A-Za-z_][A-Za-z0-9_]*)\s+\*?([A-Za-z_][A-Za-z0-9_]*)\s*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)[^{]*\{/g;
  for (const match of source.matchAll(pattern)) {
    const opening = source.indexOf('{', match.index + match[0].length - 1);
    const closing = matchingDelimiter(source, opening, '{', '}');
    if (opening < 0 || closing < 0) continue;
    const receiver = match[2];
    functions.push({
      file: file.replaceAll('\\', '/'),
      line: lineAt(source, match.index),
      symbol: receiver ? `${receiver}.${match[3]}` : match[3],
      receiver,
      name: match[3],
      signature: match[0],
      body: source.slice(opening + 1, closing),
      start: match.index,
      end: closing,
    });
  }
  return functions;
}

function parseFunctionParameterTypes(signature) {
  const opening = signature.indexOf('(');
  const closing = signature.lastIndexOf(')');
  if (opening < 0 || closing <= opening) return new Map();
  const result = new Map();
  for (const parameter of splitTopLevel(signature.slice(opening + 1, closing))) {
    const match = parameter.match(/^([A-Za-z_][A-Za-z0-9_]*)\s+(?:\*?)([A-Za-z_][A-Za-z0-9_]*)$/);
    if (match) result.set(match[1], match[2]);
  }
  return result;
}

function firstMethodReference(expression) {
  const references = [...expression.matchAll(/\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\b/g)]
    .map((match) => `${match[1]}.${match[2]}`)
    .filter((value) => !/^(?:http|https|nethttp|compatserver)\./.test(value));
  return references.at(-1) || null;
}

function lowerFirst(value) {
  return value ? `${value[0].toLowerCase()}${value.slice(1)}` : value;
}

function upperFirst(value) {
  return value ? `${value[0].toUpperCase()}${value.slice(1)}` : value;
}

function collectHandlerBindings(sources) {
  const bindings = new Map();
  for (const { file, body } of sources) {
    for (const match of body.matchAll(/\bWith([A-Za-z_][A-Za-z0-9_]*)Handler\s*\(/g)) {
      const opening = body.indexOf('(', match.index + match[0].length - 1);
      const closing = matchingDelimiter(body, opening);
      if (opening < 0 || closing < 0) continue;
      const reference = firstMethodReference(body.slice(opening + 1, closing));
      if (!reference) continue;
      bindings.set(lowerFirst(match[1]), {
        reference,
        file: file.replaceAll('\\', '/'),
        line: lineAt(body, match.index),
      });
    }
  }
  return bindings;
}

function handlerExpressionFromCall(expression) {
  return firstMethodReference(expression) || expression.trim().replace(/^\((.*)\)$/, '$1').trim();
}

function routeCandidates(sources, constants) {
  const candidates = [];
  for (const { file, body } of sources) {
    for (const match of body.matchAll(/\b(?:router|registrar|r)\.Handle\s*\(/g)) {
      const opening = body.indexOf('(', match.index + match[0].length - 1);
      const closing = matchingDelimiter(body, opening);
      if (opening < 0 || closing < 0) continue;
      const args = splitTopLevel(body.slice(opening + 1, closing));
      const method = goMethod(args[0] || '', constants);
      const route = goString(args[1] || '', constants);
      if (!method || !route?.startsWith('/dashboard/')) continue;
      candidates.push({
        method,
        route,
        handlerExpression: handlerExpressionFromCall(args.slice(2).join(',')),
        file: file.replaceAll('\\', '/'),
        line: lineAt(body, match.index),
        index: match.index,
      });
    }

    for (const match of body.matchAll(/^\s*case\s+(.+):\s*$/gm)) {
      const nextCase = body.slice(match.index + match[0].length).search(/^\s*(?:case|default)\s+.+:\s*$/m);
      const block = body.slice(match.index + match[0].length, nextCase < 0 ? body.length : match.index + match[0].length + nextCase);
      const methods = [...match[1].matchAll(/(?:^|\s|&&)r\.Method\s*==\s*((?:[A-Za-z_][A-Za-z0-9_]*\.)?Method(?:Get|Post|Put|Patch|Delete)|['"][A-Z]+['"])/g)]
        .map((item) => goMethod(item[1], constants)).filter(Boolean);
      const paths = [...match[1].matchAll(/r\.URL\.Path\s*==\s*([^\s&]+)/g)]
        .map((item) => goString(item[1], constants)).filter((value) => value?.startsWith('/dashboard/'));
      const handlerExpressions = [
        ...block.matchAll(/\b([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?)\.ServeHTTP\s*\(/g),
        ...block.matchAll(/\b(?:serveHTTPWithPath|serveHTTPWithPrincipal)\s*\(\s*([^,\n]+)/g),
      ].map((item) => item[1].trim());
      for (const method of methods) for (const route of paths) for (const handlerExpression of handlerExpressions) {
        candidates.push({
          method,
          route,
          handlerExpression,
          file: file.replaceAll('\\', '/'),
          line: lineAt(body, match.index),
          index: match.index,
        });
      }
    }
  }
  return candidates;
}

function methodDefinitions(goFiles) {
  const definitions = [];
  const parameterTypes = new Map();
  for (const file of goFiles) {
    const source = stripComments(fs.readFileSync(file, 'utf8'), GO_EXT);
    const functions = goFunctionBodies(file, source);
    for (const functionBody of functions) {
      definitions.push(functionBody);
      const types = parseFunctionParameterTypes(functionBody.signature);
      for (const [name, type] of types) parameterTypes.set(`${functionBody.file}:${name}`, type);
    }
  }
  return { definitions, parameterTypes };
}

function resolveHandlerMethod(handlerExpression, routeFile, definitions, parameterTypes, bindings) {
  const reference = firstMethodReference(handlerExpression) || handlerExpression;
  let [receiver, method] = reference.split('.');
  if (!method) method = 'ServeHTTP';
  const binding = bindings.get(receiver);
  if (binding) {
    [receiver, method] = binding.reference.split('.');
  }
  const routeFileKey = routeFile.replaceAll('\\', '/');
  const explicitType = parameterTypes.get(`${routeFileKey}:${receiver}`);
  const receiverCandidates = new Set([
    explicitType,
    upperFirst(receiver),
    ...definitions.map((item) => item.receiver).filter((type) => type && lowerFirst(type) === receiver),
  ].filter(Boolean));
  let definition = definitions.find((item) => item.receiver && receiverCandidates.has(item.receiver) && item.name === method);
  if (!definition) definition = definitions.find((item) => item.receiver && item.name === method && lowerFirst(item.receiver) === receiver);
  if (!definition) return null;
  return {
    reference: handlerExpression,
    handlerSymbol: binding?.reference || reference,
    consumerSymbol: definition.symbol,
    handlerSource: `${definition.file}:${definition.line}`,
    consumerSource: `${definition.file}:${definition.line}`,
    consumerBody: definition.body,
  };
}

function dashboardRoutePrincipalEvidence(routeFiles, allGoFiles) {
  const sources = routeFiles.map((file) => ({
    file: file.replaceAll('\\', '/'),
    body: stripComments(fs.readFileSync(file, 'utf8'), GO_EXT),
  }));
  const allSources = allGoFiles.map((file) => ({
    file: file.replaceAll('\\', '/'),
    body: stripComments(fs.readFileSync(file, 'utf8'), GO_EXT),
  }));
  const constants = collectGoConstants(allSources);
  const catalogRoutes = extractBackendRegisteredAPIs(sources);
  const candidates = routeCandidates(sources, constants);
  const candidateByContract = new Map();
  for (const candidate of candidates) {
    const contract = `${candidate.method} ${candidate.route}`;
    if (!candidateByContract.has(contract)) candidateByContract.set(contract, []);
    candidateByContract.get(contract).push(candidate);
  }
  for (const route of catalogRoutes) {
    if (!candidateByContract.has(route.contract)) {
      candidateByContract.set(route.contract, [{
        method: route.contract.split(' ')[0],
        route: route.contract.slice(route.contract.indexOf(' ') + 1),
        handlerExpression: '<unresolved>',
        file: route.file,
        line: route.line,
        index: 0,
      }]);
    }
  }
  const { definitions, parameterTypes } = methodDefinitions(allGoFiles);
  const bindings = collectHandlerBindings(allSources);
  const evidence = [];
  const failures = [];
  for (const [contract, routeCandidatesForContract] of [...candidateByContract.entries()].sort(([left], [right]) => left.localeCompare(right))) {
    const candidate = routeCandidatesForContract.find((item) => resolveHandlerMethod(item.handlerExpression, item.file, definitions, parameterTypes, bindings));
    if (!candidate) {
      const fallback = routeCandidatesForContract[0];
      failures.push(`${contract} -> handler ${fallback.handlerExpression} source:${fallback.file}:${fallback.line}`);
      continue;
    }
    const resolved = resolveHandlerMethod(candidate.handlerExpression, candidate.file, definitions, parameterTypes, bindings);
    if (!principalConsumerPattern.test(resolved.consumerBody)) {
      failures.push(`${contract} -> handler ${resolved.handlerSymbol} source:${resolved.handlerSource} has no DashboardPrincipal consumer in ${resolved.consumerSymbol} source:${resolved.consumerSource}`);
      continue;
    }
    evidence.push({
      method: candidate.method,
      route: candidate.route,
      evidence: `${candidate.method} ${candidate.route} -> handler ${resolved.handlerSymbol} source:${resolved.handlerSource} -> principal consumer ${resolved.consumerSymbol} source:${resolved.consumerSource}`,
      handlerSymbol: resolved.handlerSymbol,
      handlerSource: resolved.handlerSource,
      consumerSymbol: resolved.consumerSymbol,
      consumerSource: resolved.consumerSource,
    });
  }
  if (failures.length) {
    throw new Error(`Dashboard route principal binding failed: ${failures.join('; ')}`);
  }
  requireEvidence(evidence, 'DashboardPrincipal consumer evidence is required');
  return evidence;
}

function storeQueryLocations(files, table) {
  const locations = [];
  const tablePattern = new RegExp('\\b(?:FROM|INTO|UPDATE)\\s+[\\x60]?'+table+'[\\x60]?\\b', 'i');
  const dbCallPattern = /\b(?:Query(?:Row|Context)?|Exec(?:Context)?|Prepare(?:Context)?)\s*\(/;
  for (const file of files) {
    const source = stripComments(fs.readFileSync(file, 'utf8'), GO_EXT);
    for (const functionBody of goFunctionBodies(file, source)) {
      if (!dbCallPattern.test(functionBody.body) || !/\b(?:SELECT|INSERT|UPDATE|DELETE)\b/i.test(functionBody.body)) continue;
      const tableMatch = functionBody.body.match(tablePattern);
      if (!tableMatch) continue;
      locations.push({
        file: functionBody.file,
        line: lineAt(source, functionBody.start + functionBody.body.indexOf(tableMatch[0])),
        match: tableMatch[0],
        symbol: functionBody.symbol,
      });
    }
  }
  return locations;
}

function plaintextSecretEvidence(files) {
  const plaintextColumn = '(?:employee_secret|contact_secret|wx_secret|session_archive_secret|encoding_aes_key|callback_token|token)';
  const legacySQL = new RegExp(`\\b(?:SELECT|INSERT|UPDATE|WHERE|SET|VALUES|COALESCE|IFNULL|fallback)\\b[\\s\\S]{0,280}\\b${plaintextColumn}\\b(?!_ciphertext)`, 'i');
  const secretOutput = /(?:log\.(?:Print|Printf|Println)|writeJSON|json\.NewEncoder|\.Encode\s*\(|audit|before_json|after_json)[\s\S]{0,220}(?:employeeSecret|employee_secret|contactSecret|contact_secret|wxSecret|wx_secret|sessionArchiveSecret|session_archive_secret|encodingAESKey|encoding_aes_key|callbackToken|callback_token|\btoken\b)(?!_ciphertext)/i;
  const evidence = [];
  for (const file of files) {
    const source = stripComments(fs.readFileSync(file, 'utf8'), path.extname(file));
    const sqlLocation = locationFor(file, source, legacySQL, 'legacy plaintext secret SQL');
    const outputLocation = locationFor(file, source, secretOutput, 'plaintext secret response/log/audit');
    if (sqlLocation) evidence.push(sqlLocation);
    if (outputLocation) evidence.push(outputLocation);
  }
  return evidence;
}

function runIdentitySingleCorpGate(root = process.cwd()) {
  const goFiles = productionGoFiles(root);
  const routeFiles = productionRouteGoFiles(root);
  const saasGo = goFiles.filter((file) => /[\\/]saasauth[\\/]/.test(file));
  const dashboardAuthGo = goFiles.filter((file) => /[\\/]dashboardauth[\\/]/.test(file));
  const dashboardGo = goFiles.filter((file) => /[\\/]dashboard[\\/]/.test(file));
  const frontend = [
    ...frontendProductionFiles(root, 'saas-admin'),
    ...frontendProductionFiles(root, 'dashboard'),
  ];
  const allProduction = [...new Set([...goFiles, ...frontend])];

  const saasIdentityTables = storeQueryLocations(saasGo, 'mochat_go_saas_admin_users');
  const dashboardIdentityTables = storeQueryLocations(dashboardAuthGo, 'mochat_go_dashboard_identities');
  requireEvidence(saasIdentityTables, 'SaaS identity table must be used by a saasauth Store query');
  requireEvidence(dashboardIdentityTables, 'Dashboard identity table must be used by a dashboardauth Store query');
  const saasSharedIdentity = locationsFor(saasGo, /\bmc_user\b/);
  assertNo(saasSharedIdentity, 'SaaS identity/auth store must not read mc_user');

  const jwtRealms = {
    saas_admin: locationsFor([...saasGo, ...frontendProductionFiles(root, 'saas-admin')], /saas_admin/),
    dashboard: locationsFor([...dashboardAuthGo, ...frontendProductionFiles(root, 'dashboard')], /(?:^|[^A-Za-z])dashboard(?:$|[^A-Za-z])/),
  };
  requireEvidence(jwtRealms.saas_admin, 'SaaS JWT realm must be explicit');
  requireEvidence(jwtRealms.dashboard, 'Dashboard JWT realm must be explicit');

  const sharedJWT = locationsFor([...saasGo, ...dashboardAuthGo, ...frontend], /MOCHAT_SIMPLE_JWT_SECRET|(?:shared|common)[_-]?(?:jwt|token)|JWT_SECRET\s*=\s*['"][^'"]+['"]/i);
  assertNo(sharedJWT, 'shared JWT configuration or legacy simple JWT reference');

  const forbiddenCorpRoutes = locationsFor(allProduction, /(?:GET\s+|POST\s+|['"`])\/?dashboard\/corp\/(?:select|bind|store)|dashboard\/corp\/(?:select|bind|store)/i, 'legacy corp route');
  assertNo(forbiddenCorpRoutes, 'legacy corp route registered in production');

  const forbiddenSessionCorpFields = locationsFor(
    [...dashboardGo, ...frontend],
    /(?:persistCorpId|mochat_dashboard_corp_id|selectedCorpID|\bcorpId\b\s*[:=])/,
    'corp selection/session field',
  );
  assertNo(forbiddenSessionCorpFields, 'Dashboard session or request still carries corp selection');

  const companySelector = locationsFor(frontend, /新建企业|(?:corp|company)[_-]?(?:provider|selector|switcher)|企业选择器|企业列表/i, 'company selector');
  assertNo(companySelector, 'Dashboard production UI still exposes company selector/new-company flow');

  const plaintextSecretReads = plaintextSecretEvidence(allProduction);
  assertNo(plaintextSecretReads, 'plaintext credential SQL/response/log/audit read in production');

  const dashboardPrincipalConsumers = dashboardRoutePrincipalEvidence(routeFiles, goFiles);

  return {
    saasIdentityTables,
    dashboardIdentityTables,
    jwtRealms,
    dashboardPrincipalConsumers,
    forbiddenCorpRoutes,
    forbiddenSessionCorpFields,
    plaintextSecretReads,
  };
}

export { runIdentitySingleCorpGate };

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const result = runIdentitySingleCorpGate();
  console.log(`identity single-corp gate PASS: SaaS realm=${result.jwtRealms.saas_admin.length}, Dashboard realm=${result.jwtRealms.dashboard.length}, principal consumers=${result.dashboardPrincipalConsumers.length}`);
}
