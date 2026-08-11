import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import {
  extractBackendRegisteredAPIs,
  extractMigrationPermissionResourceMappings,
} from './check_dashboard_page_rbac_catalog.mjs';

const GO_EXT = '.go';
const FRONTEND_EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx'];
const ignoredName = (name) => /(?:\.test|\.spec)\.[^.]+$/.test(name) || name.endsWith('_test.go');
const principalConsumerPattern = /DashboardPrincipalFromContext|RequireDashboardPrincipal/;
const saasPrincipalConsumerPattern = /SaaSPrincipalFromContext|RequireSaaSPrincipal/;

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

function principalCorpCompatibilityLocations(files) {
  const evidence = [];
  const candidates = /\bprincipalCorpID\s*\(([^\n)]*)\)/g;
  for (const file of files) {
    const source = stripComments(fs.readFileSync(file, 'utf8'), GO_EXT);
    for (const match of source.matchAll(candidates)) {
      const argumentsText = match[1].trim();
      const argumentCount = argumentsText === '' ? 0 : argumentsText.split(',').length;
      if (!argumentsText.includes('...') && argumentCount <= 2) continue;
      evidence.push({
        file: file.replaceAll('\\', '/'),
        line: lineAt(source, match.index),
        match: match[0],
      });
    }
  }
  return evidence;
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

function skipWhitespace(source, index) {
  let cursor = index;
  while (/\s/.test(source[cursor] || '')) cursor += 1;
  return cursor;
}

function nextFunctionBodyOpening(source, index) {
  let parentheses = 0;
  let brackets = 0;
  let state = 'code';
  for (let cursor = index; cursor < source.length; cursor += 1) {
    const current = source[cursor];
    if (state === 'quoted') {
      if (current === '\\') cursor += 1;
      else if (current === '"') state = 'code';
      continue;
    }
    if (state === 'raw') {
      if (current === '`') state = 'code';
      continue;
    }
    if (state === 'rune') {
      if (current === '\\') cursor += 1;
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
    else if (current === '{' && parentheses === 0 && brackets === 0) return cursor;
  }
  return -1;
}

function goFunctionBodies(file, source) {
  const functions = [];
  let cursor = 0;
  while (cursor < source.length) {
    const keyword = source.indexOf('func', cursor);
    if (keyword < 0) break;
    const before = source[keyword - 1] || '';
    const after = source[keyword + 4] || '';
    if (/[_A-Za-z0-9]/.test(before) || /[_A-Za-z0-9]/.test(after)) {
      cursor = keyword + 4;
      continue;
    }

    let index = skipWhitespace(source, keyword + 4);
    let receiver = null;
    if (source[index] === '(') {
      const receiverEnd = matchingDelimiter(source, index);
      if (receiverEnd < 0) break;
      const receiverText = source.slice(index + 1, receiverEnd);
      const receiverMatch = receiverText.match(/\b[A-Za-z_][A-Za-z0-9_]*\s+\*?([A-Za-z_][A-Za-z0-9_]*)\b/);
      receiver = receiverMatch?.[1] || null;
      index = skipWhitespace(source, receiverEnd + 1);
    }
    const nameMatch = source.slice(index).match(/^([A-Za-z_][A-Za-z0-9_]*)/);
    if (!nameMatch) {
      cursor = keyword + 4;
      continue;
    }
    const name = nameMatch[1];
    index = skipWhitespace(source, index + name.length);
    if (source[index] !== '(') {
      cursor = index;
      continue;
    }
    const parametersEnd = matchingDelimiter(source, index);
    if (parametersEnd < 0) break;
    const opening = nextFunctionBodyOpening(source, parametersEnd + 1);
    const closing = opening < 0 ? -1 : matchingDelimiter(source, opening, '{', '}');
    if (opening < 0 || closing < 0) {
      cursor = parametersEnd + 1;
      continue;
    }
    functions.push({
      file: file.replaceAll('\\', '/'),
      line: lineAt(source, keyword),
      symbol: receiver ? `${receiver}.${name}` : name,
      receiver,
      name,
      signature: source.slice(keyword, opening + 1),
      body: source.slice(opening + 1, closing),
      start: keyword,
      end: closing,
      parameters: source.slice(index + 1, parametersEnd),
    });
    cursor = closing + 1;
  }
  return functions;
}

function parseFunctionParameterTypes(signature) {
  const groups = [...signature.matchAll(/\(([^()]*)\)/g)];
  const parameterText = groups.at(-1)?.[1];
  if (parameterText === undefined) return new Map();
  const result = new Map();
  for (const parameter of splitTopLevel(parameterText)) {
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
      const argumentsBody = body.slice(opening + 1, closing);
      let reference = firstMethodReference(argumentsBody);
      if (reference && /\.New[A-Za-z_][A-Za-z0-9_]*Handler$/.test(reference)) reference = null;
      const closure = argumentsBody.match(/\bfunc\s*(?:\([^)]*\))?\s*\{/);
      if (!reference) {
        const simpleHandler = argumentsBody.trim().match(/^([A-Za-z_][A-Za-z0-9_]*)$/);
        if (simpleHandler) reference = `${simpleHandler[1]}.ServeHTTP`;
      }
      if (!reference && /\bNew[A-Za-z_][A-Za-z0-9_]*Handler\s*\(/.test(argumentsBody)) {
        reference = `${upperFirst(match[1])}Handler.ServeHTTP`;
      }
      if (!reference && !closure) continue;
      bindings.set(lowerFirst(match[1]), {
        reference,
        closureExpression: closure ? argumentsBody.slice(closure.index).trim() : null,
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
    for (const match of body.matchAll(/\b(?:[A-Za-z_][A-Za-z0-9_]*\.)*[A-Za-z_][A-Za-z0-9_]*\.Handle\s*\(/g)) {
      const opening = body.indexOf('(', match.index + match[0].length - 1);
      const closing = matchingDelimiter(body, opening);
      if (opening < 0 || closing < 0) continue;
      const args = splitTopLevel(body.slice(opening + 1, closing));
      const method = goMethod(args[0] || '', constants);
      const route = goString(args[1] || '', constants);
      if (!method || !route?.startsWith('/dashboard/')) continue;
      const handlerText = args.slice(2).join(',');
      candidates.push({
        method,
        route,
        handlerExpression: handlerExpressionFromCall(handlerText),
        file: file.replaceAll('\\', '/'),
        line: lineAt(body, match.index),
        index: match.index,
        handlerIndex: Math.max(0, body.indexOf(handlerText, opening + 1)),
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
          handlerIndex: Math.max(0, body.indexOf(handlerExpression, match.index + match[0].length)),
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

function resolveClosure(expression, file, line) {
  const opening = expression.indexOf('{');
  const closing = opening < 0 ? -1 : matchingDelimiter(expression, opening, '{', '}');
  if (opening < 0 || closing < 0) return null;
  const source = file.replaceAll('\\', '/');
  return {
    reference: expression,
    handlerSymbol: `closure@${source}:${line}`,
    consumerSymbol: `closure@${source}:${line}`,
    handlerSource: `${source}:${line}`,
    consumerSource: `${source}:${line}`,
    consumerBody: expression.slice(opening + 1, closing),
  };
}

function resolveHandlerMethod(candidate, definitions, parameterTypes, bindings) {
  const handlerExpression = candidate.handlerExpression;
  const directClosure = resolveClosure(handlerExpression, candidate.file, candidate.line);
  if (directClosure) return directClosure;

  const reference = firstMethodReference(handlerExpression) || handlerExpression;
  let [receiver, method] = reference.split('.');
  if (!method) method = 'ServeHTTP';
  const fieldName = reference.includes('.') ? reference.split('.').at(-1) : receiver;
  const binding = bindings.get(fieldName) || bindings.get(receiver);
  if (binding?.closureExpression) {
    const closure = resolveClosure(binding.closureExpression, binding.file, binding.line);
    if (closure) {
      return { ...closure, reference: handlerExpression, handlerSymbol: `${fieldName} -> ${closure.handlerSymbol}` };
    }
  }
  if (binding?.reference) {
    [receiver, method] = binding.reference.split('.');
  }
  const routeFileKey = candidate.file.replaceAll('\\', '/');
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

function policyContracts(body, name, required = false) {
  const declaration = body.match(new RegExp(`(?:var|const)\\s+${name}\\s*=\\s*\\[\\]string\\s*\\{`));
  if (!declaration) {
    if (required) throw new Error(`missing production dashboard route policy: ${name}`);
    return [];
  }
  const opening = declaration.index + declaration[0].lastIndexOf('{');
  const closing = matchingDelimiter(body, opening, '{', '}');
  if (closing < 0) throw new Error(`unterminated production dashboard route policy: ${name}`);
  return [...body.slice(opening + 1, closing).matchAll(/["']((?:GET|POST|PUT|PATCH|DELETE) \/dashboard\/[^"']+)["']/g)]
    .map((match) => match[1]);
}

function dashboardRoutePolicy(root) {
  const policyFile = path.join(root, 'internal', 'dashboard', 'dashboard_route_policy.go');
  if (!fs.existsSync(policyFile)) throw new Error('missing production dashboard route policy');
  const policySource = stripComments(fs.readFileSync(policyFile, 'utf8'), GO_EXT);
  const exactExempt = new Set(policyContracts(policySource, 'exactExemptDashboardRouteContracts', true));
  const publicContracts = new Set(policyContracts(policySource, 'publicDashboardRouteContracts', true));
  const denyOnly = new Set(policyContracts(policySource, 'denyOnlyDashboardRouteContracts', true));
  const explicitPageMapped = policyContracts(policySource, 'pageMappedDashboardRouteContracts');
  const explicitSaaS = policyContracts(policySource, 'saasPrincipalDashboardRouteContracts');
  if ([...publicContracts].some((contract) => !exactExempt.has(contract))) {
    throw new Error('public dashboard route policy must be an exact exemption allowlist');
  }

  const pageMapped = new Set(explicitPageMapped);
  if (!pageMapped.size) {
    const migrationFile = path.join(root, 'deploy', 'standalone', 'migrations', '0127_dashboard_page_rbac.up.sql');
    if (fs.existsSync(migrationFile)) {
      for (const mapping of extractMigrationPermissionResourceMappings(fs.readFileSync(migrationFile, 'utf8'))) {
        const [, contract] = mapping.split('\t');
        if (contract) pageMapped.add(contract);
      }
    }
  }
  return {
    exactExempt,
    publicContracts,
    denyOnly,
    pageMapped,
    saasPrincipal: new Set(explicitSaaS),
  };
}

function dashboardRoutePrincipalEvidence(root, routeFiles, allGoFiles) {
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
  const policy = dashboardRoutePolicy(root);
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
  const dashboardPrincipalRoutes = [];
  const saasPrincipalRoutes = [];
  const publicExactRoutes = [];
  const authenticatedExactRoutes = [];
  const failures = [];
  const saasPrefixes = ['/dashboard/saasAdmin/', '/dashboard/saasAlert/', '/dashboard/saasBilling/'];
  const classify = (method, route) => {
    const contract = `${method} ${route}`;
    if (policy.exactExempt.has(contract)) return 'public-exact';
    if (policy.saasPrincipal.has(contract) || saasPrefixes.some((prefix) => route.startsWith(prefix))) return 'saas-principal';
    if (policy.denyOnly.has(contract)) return 'dashboard-principal-deny-only';
    if (policy.pageMapped.has(contract)) return 'dashboard-principal-page-mapped';
    return null;
  };
  for (const [contract, routeCandidatesForContract] of [...candidateByContract.entries()].sort(([left], [right]) => left.localeCompare(right))) {
    const method = contract.slice(0, contract.indexOf(' '));
    const route = contract.slice(contract.indexOf(' ') + 1);
    const category = classify(method, route);
    if (!category) {
      failures.push(`${contract} -> unknown dashboard route policy; no DashboardPrincipal/SaaSPrincipal consumer category (handler candidates: ${routeCandidatesForContract.map((item) => `${item.handlerExpression} source:${item.file}:${item.line}`).join(', ')})`);
      continue;
    }
    const candidate = routeCandidatesForContract.find((item) => resolveHandlerMethod(item, definitions, parameterTypes, bindings));
    if (!candidate) {
      const fallback = routeCandidatesForContract[0];
      failures.push(`${contract} -> handler ${fallback.handlerExpression} source:${fallback.file}:${fallback.line}`);
      continue;
    }
    const resolved = resolveHandlerMethod(candidate, definitions, parameterTypes, bindings);
    const evidence = {
      method: candidate.method,
      route: candidate.route,
      category,
      evidence: `${candidate.method} ${candidate.route} -> handler ${resolved.handlerSymbol} source:${resolved.handlerSource} -> ${resolved.consumerSymbol} source:${resolved.consumerSource}`,
      handlerSymbol: resolved.handlerSymbol,
      handlerSource: resolved.handlerSource,
      consumerSymbol: resolved.consumerSymbol,
      consumerSource: resolved.consumerSource,
    };
    if (category === 'saas-principal') {
      if (!saasPrincipalConsumerPattern.test(resolved.consumerBody)) {
        failures.push(`${contract} -> handler ${resolved.handlerSymbol} source:${resolved.handlerSource} has no SaaSPrincipal consumer in ${resolved.consumerSymbol} source:${resolved.consumerSource}`);
        continue;
      }
      evidence.evidence += ' -> SaaSPrincipal consumer';
      saasPrincipalRoutes.push(evidence);
    } else if (category === 'public-exact') {
      const isPublic = policy.publicContracts.has(contract);
      evidence.evidence += ` -> exact exemption${isPublic ? ' public' : ' identity-authenticated'}`;
      (isPublic ? publicExactRoutes : authenticatedExactRoutes).push(evidence);
    } else {
      if (!principalConsumerPattern.test(resolved.consumerBody)) {
        failures.push(`${contract} -> handler ${resolved.handlerSymbol} source:${resolved.handlerSource} has no DashboardPrincipal consumer in ${resolved.consumerSymbol} source:${resolved.consumerSource}`);
        continue;
      }
      evidence.evidence += ' -> DashboardPrincipal consumer';
      dashboardPrincipalRoutes.push(evidence);
    }
  }
  if (failures.length) {
    throw new Error(`Dashboard route principal binding failed: ${failures.join('; ')}`);
  }
  requireEvidence([...dashboardPrincipalRoutes, ...saasPrincipalRoutes, ...publicExactRoutes, ...authenticatedExactRoutes], 'Dashboard route evidence is required');
  return { dashboardPrincipalRoutes, saasPrincipalRoutes, publicExactRoutes, authenticatedExactRoutes };
}

function storeQueryLocations(files, table) {
  const locations = [];
  const tablePattern = new RegExp('\\b(?:FROM|INTO|UPDATE)\\s+[\\x60]?'+table+'[\\x60]?\\b', 'i');
  const dbCallPattern = /\b(?:Query(?:Row|Context)?|Exec(?:Context)?|Prepare(?:Context)?|query|exec)\s*\(/;
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
        source: `${functionBody.file}:${lineAt(source, functionBody.start + functionBody.body.indexOf(tableMatch[0]))}`,
      });
    }
  }
  return locations;
}

function identityInterfaceMethods(files, packagePattern) {
  const methods = new Set();
  for (const file of files) {
    const source = stripComments(fs.readFileSync(file, 'utf8'), GO_EXT);
    for (const match of source.matchAll(/type\s+([A-Za-z_][A-Za-z0-9_]*)\s+interface\s*\{/g)) {
      if (!packagePattern.test(match[1])) continue;
      const opening = source.indexOf('{', match.index + match[0].length - 1);
      const closing = matchingDelimiter(source, opening, '{', '}');
      if (opening < 0 || closing < 0) continue;
      for (const method of source.slice(opening + 1, closing).matchAll(/\b([A-Za-z_][A-Za-z0-9_]*)\s*\(/g)) {
        methods.add(method[1]);
      }
    }
  }
  return methods;
}

function identityStoreQueryLocations(
  domainFiles,
  adapterFiles,
  table,
  interfaceNamePattern,
  packageName,
  adapterReceiverPattern,
  adapterFilePattern,
) {
  const domainLocations = storeQueryLocations(domainFiles, table).map((location) => ({
    ...location,
    layer: 'domain-store',
  }));
  const interfaceMethods = identityInterfaceMethods(domainFiles, interfaceNamePattern);
  const adapterLocations = storeQueryLocations(adapterFiles, table)
    .filter((location) => {
      const method = location.symbol.split('.').at(-1);
      if (!interfaceMethods.has(method)) return false;
      const source = stripComments(fs.readFileSync(location.file, 'utf8'), GO_EXT);
      const receiver = location.symbol.split('.').slice(0, -1).join('.');
      const linkedReceiver = adapterReceiverPattern.test(receiver);
      const linkedFile = adapterFilePattern.test(path.basename(location.file));
      const linkedInterface = new RegExp(`var\\s+_\\s+(?:\\([^)]*\\)\\s*)?\\*?${packageName}\\.Store\\b`).test(source);
      return linkedReceiver || linkedFile || linkedInterface;
    })
    .map((location) => ({
      ...location,
      layer: 'internal-store-adapter',
    }));
  return [...domainLocations, ...adapterLocations];
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

function isAllowedDashboardIdentityResolution(functionBody) {
  if (functionBody.name !== 'ResolveIdentity') return false;
  const body = functionBody.body;
  if (!/\bmochat_go_dashboard_identities\b/i.test(body) || !/\bmc_user\b/i.test(body)) return false;
  return !/\b(?:password|password_hash|phone|corp_id|login_identifier)\b/i.test(body);
}

function runIdentitySingleCorpGate(root = process.cwd()) {
  const goFiles = productionGoFiles(root);
  const routeFiles = productionRouteGoFiles(root);
  const saasGo = goFiles.filter((file) => /[\\/]saasauth[\\/]/.test(file));
  const dashboardAuthGo = goFiles.filter((file) => /[\\/]dashboardauth[\\/]/.test(file));
  const storeGo = goFiles.filter((file) => /[\\/]internal[\\/]store[\\/]/.test(file));
  const dashboardGo = goFiles.filter((file) => /[\\/]dashboard[\\/]/.test(file));
  const frontend = [
    ...frontendProductionFiles(root, 'saas-admin'),
    ...frontendProductionFiles(root, 'dashboard'),
  ];
  const allProduction = [...new Set([...goFiles, ...frontend])];

  const principalCorpCompatibility = principalCorpCompatibilityLocations(goFiles);
  assertNo(principalCorpCompatibility, 'principalCorpID compatibility helper with variadic or extra arguments is forbidden');
  const legacyModulePrincipalResolver = locationsFor(
    routeFiles,
    /\b(?:[A-Za-z_][A-Za-z0-9_]*\.)?NewSCRMPrincipalResolver\s*\(/,
    'legacy SCRM principal resolver',
  );
  assertNo(legacyModulePrincipalResolver, 'legacy SCRM principal resolver cannot be used by production composition');

  const saasIdentityTables = identityStoreQueryLocations(
    saasGo,
    storeGo,
    'mochat_go_saas_admin_users',
    /^(?:Store|SaaSIdentityStore|SaaSAuthStore)$/,
    'saasauth',
    /SaaS|Saas/i,
    /saas[_-]?identity/i,
  );
  const dashboardIdentityTables = identityStoreQueryLocations(
    dashboardAuthGo,
    storeGo,
    'mochat_go_dashboard_identities',
    /^(?:Store|DashboardIdentityStore|DashboardAuthStore)$/,
    'dashboardauth',
    /Dashboard|dashboard/i,
    /dashboard[_-]?identity/i,
  );
  requireEvidence(saasIdentityTables, 'SaaS identity table must be used by a saasauth Store query');
  requireEvidence(dashboardIdentityTables, 'Dashboard identity table must be used by a dashboardauth Store query');
  const saasIdentityFiles = [...new Set(saasIdentityTables.map((location) => location.file))];
  const dashboardIdentityFiles = [...new Set(dashboardIdentityTables.map((location) => location.file))];
  const saasSharedIdentity = locationsFor([...saasGo, ...saasIdentityFiles], /\bmc_user\b/);
  assertNo(saasSharedIdentity, 'SaaS identity/auth store must not read mc_user');
  const dashboardSharedIdentity = locationsFor(dashboardAuthGo, /\bmc_user\b/);
  for (const file of dashboardIdentityFiles) {
      const source = stripComments(fs.readFileSync(file, 'utf8'), GO_EXT);
      for (const functionBody of goFunctionBodies(file, source)) {
        const match = functionBody.body.match(/\bmc_user\b/);
      if (!match || isAllowedDashboardIdentityResolution(functionBody)) continue;
      dashboardSharedIdentity.push({
        file: functionBody.file,
        line: lineAt(source, functionBody.start + functionBody.body.indexOf(match[0])),
        match: match[0],
      });
    }
  }
  assertNo(dashboardSharedIdentity, 'Dashboard identity/auth store must not read mc_user');

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

  const routeEvidence = dashboardRoutePrincipalEvidence(root, routeFiles, goFiles);
  const dashboardPrincipalConsumers = routeEvidence.dashboardPrincipalRoutes;

  return {
    saasIdentityTables,
    dashboardIdentityTables,
    jwtRealms,
    dashboardPrincipalConsumers,
    dashboardPrincipalRoutes: routeEvidence.dashboardPrincipalRoutes,
    saasPrincipalRoutes: routeEvidence.saasPrincipalRoutes,
    publicExactRoutes: routeEvidence.publicExactRoutes,
    authenticatedExactRoutes: routeEvidence.authenticatedExactRoutes,
    forbiddenCorpRoutes,
    forbiddenSessionCorpFields,
    plaintextSecretReads,
    principalCorpCompatibility,
    legacyModulePrincipalResolver,
  };
}

function formatIdentitySingleCorpGateEvidence(result) {
  const lines = [];
  for (const location of result.saasIdentityTables || []) {
    lines.push(`SaaS identity Store query: ${location.symbol} source:${location.source}`);
  }
  for (const location of result.dashboardIdentityTables || []) {
    lines.push(`Dashboard identity Store query: ${location.symbol} source:${location.source}`);
  }
  for (const route of result.saasPrincipalRoutes || []) {
    lines.push(`SaaS principal route: ${route.method} ${route.route} -> handler ${route.handlerSymbol} source:${route.handlerSource} -> ${route.consumerSymbol} source:${route.consumerSource}`);
  }
  for (const route of result.dashboardPrincipalRoutes || []) {
    lines.push(`Dashboard principal route: ${route.method} ${route.route} -> handler ${route.handlerSymbol} source:${route.handlerSource} -> ${route.consumerSymbol} source:${route.consumerSource}`);
  }
  for (const route of result.publicExactRoutes || []) {
    lines.push(`public exact route: ${route.method} ${route.route} -> handler ${route.handlerSymbol} source:${route.handlerSource} -> ${route.consumerSymbol} source:${route.consumerSource}`);
  }
  for (const route of result.authenticatedExactRoutes || []) {
    lines.push(`identity-auth exact route: ${route.method} ${route.route} -> handler ${route.handlerSymbol} source:${route.handlerSource} -> ${route.consumerSymbol} source:${route.consumerSource}`);
  }
  return lines.join('\n');
}

export { formatIdentitySingleCorpGateEvidence, runIdentitySingleCorpGate };

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const result = runIdentitySingleCorpGate();
  console.log(formatIdentitySingleCorpGateEvidence(result));
  console.log(`identity single-corp gate PASS: SaaS realm=${result.jwtRealms.saas_admin.length}, Dashboard realm=${result.jwtRealms.dashboard.length}, SaaS Store queries=${result.saasIdentityTables.length}, Dashboard Store queries=${result.dashboardIdentityTables.length}, SaaS principal routes=${result.saasPrincipalRoutes.length}, Dashboard principal routes=${result.dashboardPrincipalRoutes.length}, public exact routes=${result.publicExactRoutes.length}, identity-auth exact routes=${result.authenticatedExactRoutes.length}`);
}
