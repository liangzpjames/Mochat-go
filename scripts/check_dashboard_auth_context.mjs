import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

import { extractBackendRegisteredAPIs } from './check_dashboard_page_rbac_catalog.mjs';

const defaultExemptions = [
  { method: 'PUT', route: '/dashboard/user/logout', handlerSymbol: 'LogoutHandler.ServeHTTP', operation: 'DeleteUserCorpCache' },
  { method: 'PUT', route: '/dashboard/user/passwordUpdate', handlerSymbol: 'UserAdminHandler.PasswordUpdate', operation: 'DeleteUserCorpCache' },
  ...[
    ['GET', '/dashboard/auth/session'],
    ['POST', '/dashboard/auth/activate'],
    ['POST', '/dashboard/auth/logout'],
    ['POST', '/dashboard/auth/password/reset'],
    ['POST', '/dashboard/auth/password/reset-request'],
    ['POST', '/dashboard/user/auth'],
    ['POST', '/dashboard/user/authMFA'],
    ['PUT', '/dashboard/user/logout'],
  ].map(([method, route]) => ({ method, route, handlerSymbol: 'HTTPHandler.ServeHTTP', operation: 'Authorization' })),
];

function walkGo(directory) {
  if (!fs.existsSync(directory)) return [];
  const files = [];
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const full = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (/^(?:node_modules|vendor|dist|build|fixtures?|migrations?)$/i.test(entry.name)) continue;
      files.push(...walkGo(full));
    } else if (entry.name.endsWith('.go') && !entry.name.endsWith('_test.go')) {
      files.push(full);
    }
  }
  return files;
}

function sanitizeGo(source, { preserveAuthorization = false, commentsOnly = false } = {}) {
  let output = '';
  let state = 'code';
  for (let index = 0; index < source.length; index += 1) {
    const current = source[index];
    const next = source[index + 1];
    if (state === 'line-comment') {
      if (current === '\n') { output += '\n'; state = 'code'; } else output += ' ';
      continue;
    }
    if (state === 'block-comment') {
      if (current === '*' && next === '/') { output += '  '; index += 1; state = 'code'; }
      else output += current === '\n' ? '\n' : ' ';
      continue;
    }
    if (state === 'quoted' || state === 'rune' || state === 'raw') {
      const closing = state === 'quoted' ? '"' : state === 'rune' ? "'" : '`';
      const start = index;
      let value = current;
      while (index + 1 < source.length) {
        index += 1;
        value += source[index];
        if (state !== 'raw' && source[index] === '\\') {
          if (index + 1 < source.length) { index += 1; value += source[index]; }
          continue;
        }
        if (source[index] === closing) break;
      }
      if (commentsOnly) output += value;
      else if (preserveAuthorization && state === 'quoted' && value === '"Authorization"') output += 'AUTHORIZATION_LITERAL'.padEnd(value.length, ' ');
      else output += value.replace(/[^\n]/g, ' ');
      void start;
      state = 'code';
      continue;
    }
    if (current === '/' && next === '/') { output += '  '; index += 1; state = 'line-comment'; continue; }
    if (current === '/' && next === '*') { output += '  '; index += 1; state = 'block-comment'; continue; }
    if (current === '"') { state = 'quoted'; index -= 1; continue; }
    if (current === "'") { state = 'rune'; index -= 1; continue; }
    if (current === '`') { state = 'raw'; index -= 1; continue; }
    output += current;
  }
  return output;
}

function matchingBrace(source, opening) {
  let depth = 0;
  for (let index = opening; index < source.length; index += 1) {
    if (source[index] === '{') depth += 1;
    else if (source[index] === '}' && --depth === 0) return index;
  }
  return -1;
}

function lineAt(source, index) {
  return source.slice(0, index).split('\n').length;
}

function goMethod(expression) {
  const match = expression.trim().match(/^(?:(?:[A-Za-z_][A-Za-z0-9_]*)\.)?Method(Get|Post|Put|Patch|Delete|Head)$/);
  if (match) return match[1].toUpperCase();
  const quoted = expression.trim().match(/^["']([A-Z]+)["']$/);
  return quoted?.[1] || null;
}

function resolveGoString(expression, constants) {
  const parts = expression.trim().split(/\s*\+\s*/);
  let value = '';
  for (const part of parts) {
    const quoted = part.match(/^["']([^"']*)["']$/);
    if (quoted) value += quoted[1];
    else if (constants.has(part)) value += constants.get(part);
    else if (part.includes('.') && constants.has(part.slice(part.lastIndexOf('.') + 1))) value += constants.get(part.slice(part.lastIndexOf('.') + 1));
    else return null;
  }
  return value;
}

function collectGoConstants(sources) {
  const constants = new Map();
  let changed = true;
  while (changed) {
    changed = false;
    for (const { source } of sources) {
      for (const match of source.matchAll(/^\s*(?:const\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*(?:string\s*)?=\s*([^\n]+)$/gm)) {
        if (constants.has(match[1])) continue;
        const resolved = resolveGoString(match[2], constants);
        if (resolved !== null) { constants.set(match[1], resolved); changed = true; }
      }
    }
  }
  return constants;
}

function stringListValues(expression, constants) {
  return expression.split(',').map((item) => resolveGoString(item, constants) ?? goMethod(item)).filter(Boolean);
}

function loopBindings(source, constants) {
  const loops = [];
  for (const match of source.matchAll(/for\s+_,\s*([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*range\s*\[\]string\s*\{([^}]*)\}\s*\{/g)) {
    const opening = match.index + match[0].lastIndexOf('{');
    const end = matchingBrace(source, opening);
    if (end >= 0) loops.push({ name: match[1], values: stringListValues(match[2], constants), start: opening, end });
  }
  return loops;
}

function expressionValues(expression, kind, constants, loops, index) {
  const direct = kind === 'method' ? goMethod(expression) : resolveGoString(expression, constants);
  if (direct !== null) return [direct];
  const loop = loops.find((item) => item.name === expression.trim() && item.start < index && index < item.end);
  if (loop) return kind === 'method' ? loop.values.map((value) => goMethod(value) ?? value.toUpperCase()) : loop.values;
  const containing = loops.find((item) => expression.includes(item.name) && item.start < index && index < item.end);
  if (!containing || kind === 'method') return [];
  return containing.values.map((value) => resolveGoString(expression.replaceAll(containing.name, JSON.stringify(value)), constants)).filter(Boolean);
}

function matchingDelimiter(source, opening, open = '(', close = ')') {
  let depth = 0;
  for (let index = opening; index < source.length; index += 1) {
    if (source[index] === open) depth += 1;
    else if (source[index] === close && --depth === 0) return index;
  }
  return -1;
}

function splitTopLevel(expression) {
  const result = [];
  let start = 0;
  let round = 0;
  let square = 0;
  let curly = 0;
  let quote = null;
  for (let index = 0; index < expression.length; index += 1) {
    const value = expression[index];
    if (quote) {
      if (quote !== '`' && value === '\\') index += 1;
      else if (value === quote) quote = null;
      continue;
    }
    if (value === '"' || value === "'" || value === '`') quote = value;
    else if (value === '(') round += 1;
    else if (value === ')') round -= 1;
    else if (value === '[') square += 1;
    else if (value === ']') square -= 1;
    else if (value === '{') curly += 1;
    else if (value === '}') curly -= 1;
    else if (value === ',' && round === 0 && square === 0 && curly === 0) {
      result.push(expression.slice(start, index).trim());
      start = index + 1;
    }
  }
  result.push(expression.slice(start).trim());
  return result;
}

function shortType(expression) {
  if (!expression || /\binterface\b|\bfunc\b/.test(expression)) return null;
  const matches = [...expression.matchAll(/[A-Za-z_][A-Za-z0-9_]*/g)];
  const name = matches.at(-1)?.[0];
  return name && !/^(?:error|any|string|int|int64|bool|HandlerFunc)$/.test(name) ? name : null;
}

function functionScopes(file, source) {
  const scopes = [];
  for (const match of source.matchAll(/\bfunc\s+/g)) {
    let cursor = match.index + match[0].length;
    let receiverType = null;
    if (source[cursor] === '(') {
      const receiverEnd = matchingDelimiter(source, cursor);
      if (receiverEnd < 0) continue;
      receiverType = shortType(source.slice(cursor + 1, receiverEnd));
      cursor = receiverEnd + 1;
    }
    const name = source.slice(cursor).match(/^\s*([A-Za-z_][A-Za-z0-9_]*)/)?.[1];
    if (!name) continue;
    cursor = source.indexOf('(', cursor + source.slice(cursor).indexOf(name) + name.length);
    if (cursor < 0) continue;
    const parametersEnd = matchingDelimiter(source, cursor);
    if (parametersEnd < 0) continue;
    let bodyStart = parametersEnd + 1;
    let round = 0;
    while (bodyStart < source.length) {
      if (source[bodyStart] === '(') round += 1;
      else if (source[bodyStart] === ')') round -= 1;
      else if (source[bodyStart] === '{' && round === 0) break;
      bodyStart += 1;
    }
    if (bodyStart >= source.length) continue;
    const bodyEnd = matchingBrace(source, bodyStart);
    if (bodyEnd < 0) continue;
    const params = new Map();
    for (const parameter of splitTopLevel(source.slice(cursor + 1, parametersEnd))) {
      const value = parameter.trim().match(/^([A-Za-z_][A-Za-z0-9_]*)\s+([\s\S]+)$/);
      if (value) params.set(value[1], shortType(value[2]));
    }
    scopes.push({ file, name, receiverType, params, start: bodyStart, end: bodyEnd, source });
  }
  return scopes;
}

function structFieldTypes(sources) {
  const fields = new Map();
  for (const { source } of sources) {
    for (const match of source.matchAll(/\btype\s+([A-Za-z_][A-Za-z0-9_]*)\s+struct\s*\{/g)) {
      const opening = source.indexOf('{', match.index);
      const closing = matchingBrace(source, opening);
      if (closing < 0) continue;
      for (const line of source.slice(opening + 1, closing).split('\n')) {
        const field = line.trim().match(/^([A-Za-z_][A-Za-z0-9_]*)\s+([^`]+)$/);
        const type = field && shortType(field[2]);
        if (type) fields.set(`${match[1]}.${field[1]}`, type);
      }
    }
  }
  return fields;
}

function localVariableTypes(scope) {
  const variables = new Map(scope.params);
  const body = scope.source.slice(scope.start, scope.end);
  for (const match of body.matchAll(/\b([A-Za-z_][A-Za-z0-9_]*)\s*:?=\s*(?:[A-Za-z_][A-Za-z0-9_]*\.)?New([A-Za-z_][A-Za-z0-9_]*)\s*\(/g)) variables.set(match[1], match[2]);
  for (const match of body.matchAll(/\b([A-Za-z_][A-Za-z0-9_]*)\s*:?=\s*([A-Za-z_][A-Za-z0-9_]*)\s*\[\s*\d+\s*\]/g)) {
    const type = variables.get(match[2]);
    if (type) variables.set(match[1], type);
  }
  return variables;
}

function dashboardRouteRegistry(root) {
  const registryFile = path.join(root, 'internal', 'dashboard', 'dashboard_route_registry.go');
  const source = sanitizeGo(fs.readFileSync(registryFile, 'utf8'), { commentsOnly: true });
  const authKinds = new Map([
    ['DashboardRouteAuthPublic', 'public'],
    ['DashboardRouteAuthIdentity', 'identity-authenticated'],
    ['DashboardRouteAuthPrincipal', 'dashboard-principal'],
    ['DashboardRouteAuthSaaS', 'saas-principal'],
  ]);
  const entries = new Map();
  const duplicates = [];
  const pattern = /\{\s*Method:\s*"([A-Z]+)"\s*,\s*Path:\s*"(\/dashboard\/[^"\n]+)"\s*,\s*Handler:\s*"([A-Za-z_][A-Za-z0-9_.]*)"\s*,\s*AuthKind:\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}/g;
  for (const match of source.matchAll(pattern)) {
    const contract = `${match[1]} ${match[2]}`;
    const entry = { method: match[1], route: match[2], handlerSymbol: match[3], auth: authKinds.get(match[4]), authToken: match[4] };
    if (entries.has(contract)) duplicates.push(contract);
    else entries.set(contract, entry);
  }
  return { entries, duplicates, source: registryFile.replaceAll('\\', '/') };
}

function constructorType(name, definitions) {
  const candidates = [...new Set([...definitions.values()].flat().map((definition) => definition.receiver))]
    .filter((receiver) => name.startsWith(receiver))
    .sort((left, right) => right.length - left.length);
  return candidates[0] || name;
}

function handlerSymbolFrom(expression, scope, fields, definitions, moduleRoot, propagatedTypes) {
  const unwrapped = expression.trim().replace(/^(?:[A-Za-z_][A-Za-z0-9_]*\.)?HandlerFunc\((.*)\)$/s, '$1').trim();
  const reference = unwrapped.match(/^([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)$/);
  const variables = localVariableTypes(scope);
  for (const [name, type] of variables) if (type) variables.set(name, constructorType(type, definitions));
  if (scope.receiverType) variables.set('m', scope.receiverType);
  if (reference) {
    let type = variables.get(reference[1]);
    if (type && scope.receiverType === type) {
      type = fields.get(`${type}.${reference[2]}`);
      if (type && findDefinition(definitions, `${type}.ServeHTTP`, moduleRoot)) return `${type}.ServeHTTP`;
    }
    if (type && findDefinition(definitions, `${type}.${reference[2]}`, moduleRoot)) return `${type}.${reference[2]}`;
    if (type && findDefinition(definitions, `${type}.ServeHTTP`, moduleRoot)) return `${type}.ServeHTTP`;
  }
  const simple = unwrapped.match(/^([A-Za-z_][A-Za-z0-9_]*)$/)?.[1];
  if (simple) {
    const type = variables.get(simple) || propagatedTypes.get(`${scope.file}:${scope.name}:${simple}`);
    if (type && findDefinition(definitions, `${type}.ServeHTTP`, moduleRoot)) return `${type}.ServeHTTP`;
    if (type === 'Handler') return 'http.Handler.ServeHTTP';
  }
  const candidates = [...definitions.values()].flat().filter((definition) => definition.method === 'ServeHTTP' && definition.file.startsWith(moduleRoot));
  return candidates.length === 1 ? candidates[0].symbol : null;
}

function modulePrefix(file) {
  const normalized = file.replaceAll('\\', '/');
  const marker = normalized.indexOf('/internal/modules/');
  if (marker < 0) return normalized.slice(0, normalized.lastIndexOf('/') + 1);
  const after = normalized.indexOf('/', marker + '/internal/modules/'.length);
  return after < 0 ? `${normalized}/` : normalized.slice(0, after + 1);
}

function inferArgumentType(expression, scope, fields) {
  const value = expression.trim().replace(/^(?:[A-Za-z_][A-Za-z0-9_]*\.)?HandlerFunc\((.*)\)$/s, '$1').trim();
  const variables = localVariableTypes(scope);
  const simple = value.match(/^([A-Za-z_][A-Za-z0-9_]*)$/)?.[1];
  if (simple) return variables.get(simple) || null;
  const field = value.match(/^([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)$/);
  if (!field) return null;
  const owner = variables.get(field[1]) || (field[1] === 'm' ? scope.receiverType : null);
  return owner ? fields.get(`${owner}.${field[2]}`) || owner : null;
}

function propagatedParameterTypes(sources, scopes, fields) {
  const propagated = new Map();
  for (const target of scopes) {
    const parameters = [...target.params.keys()];
    if (!parameters.some((name) => target.params.get(name) === null)) continue;
    const prefix = modulePrefix(target.file);
    for (const { file, source } of sources) {
      if (!file.startsWith(prefix)) continue;
      const pattern = new RegExp(`\\b${target.name}\\s*\\(`, 'g');
      for (const call of source.matchAll(pattern)) {
        if (target.file === file && call.index < target.start) continue;
        const opening = source.indexOf('(', call.index);
        const closing = matchingDelimiter(source, opening);
        if (closing < 0) continue;
        const caller = scopes.find((scope) => scope.file === file && scope.start < call.index && call.index < scope.end);
        if (!caller) continue;
        const args = splitTopLevel(source.slice(opening + 1, closing));
        parameters.forEach((parameter, index) => {
          if (target.params.get(parameter) !== null || propagated.has(`${target.file}:${target.name}:${parameter}`)) return;
          const type = inferArgumentType(args[index] || '', caller, fields);
          if (type) propagated.set(`${target.file}:${target.name}:${parameter}`, type);
        });
      }
    }
  }
  return propagated;
}

function registeredHandlerRoutes(sources, definitions) {
  const constants = collectGoConstants(sources);
  const fields = structFieldTypes(sources);
  const scopes = sources.flatMap(({ file, source }) => functionScopes(file, source));
  const propagated = propagatedParameterTypes(sources, scopes, fields);
  const routes = [];
  const pushRoutes = ({ methods, paths, handlerExpression, file, source, index, scope }) => {
    if (!scope) return;
    const handlerSymbol = handlerSymbolFrom(handlerExpression, scope, fields, definitions, modulePrefix(file), propagated);
    if (!handlerSymbol) return;
    const handlerDefinition = findDefinition(definitions, handlerSymbol, modulePrefix(file));
    for (const method of methods) for (const route of paths) {
      if (route.startsWith('/dashboard/')) routes.push({ method, route, handlerSymbol, handlerDefinition, dispatchSource: `${file}:${lineAt(source, index)}`, compositionSource: `${file}:${lineAt(source, scope.start)}`, registrationKind: 'module' });
    }
  };

  for (const { file, source } of sources) {
    const loops = loopBindings(source, constants);
    const handlePattern = /\b(?:registrar|router)\.Handle\s*\(/g;
    for (const match of source.matchAll(handlePattern)) {
      const opening = source.indexOf('(', match.index);
      const closing = matchingDelimiter(source, opening);
      if (closing < 0) continue;
      const args = splitTopLevel(source.slice(opening + 1, closing));
      if (args.length < 3 || (args[0] === 'route.method' && args[1] === 'route.path')) continue;
      const scope = scopes.find((item) => item.file === file && item.start < match.index && match.index < item.end);
      pushRoutes({
        methods: expressionValues(args[0], 'method', constants, loops, match.index),
        paths: expressionValues(args[1], 'path', constants, loops, match.index),
        handlerExpression: args[2], file, source, index: match.index, scope,
      });
    }
    const routeHandle = source.match(/\b(?:registrar|router)\.Handle\s*\(\s*route\.method\s*,\s*route\.path\s*,\s*([^,)]+)/);
    if (routeHandle) {
      for (const match of source.matchAll(/\{\s*((?:[A-Za-z_][A-Za-z0-9_]*\.)?Method(?:Get|Post|Put|Patch|Delete|Head)|["'][A-Z]+["'])\s*,\s*([^,\n}]+)(?:\s*,\s*([^,\n}]+))?\s*\}/g)) {
        const scope = scopes.find((item) => item.file === file && item.start < match.index && match.index < item.end);
        pushRoutes({
          methods: expressionValues(match[1], 'method', constants, [], match.index),
          paths: expressionValues(match[2], 'path', constants, [], match.index),
          handlerExpression: match[3] || routeHandle[1], file, source, index: match.index, scope,
        });
      }
    }
  }
  return routes;
}

function functionDefinitions(files) {
  const definitions = new Map();
  for (const file of files) {
    const original = fs.readFileSync(file, 'utf8');
    const source = sanitizeGo(original, { commentsOnly: true });
    const pattern = /func\s*\(\s*[A-Za-z_][A-Za-z0-9_]*\s+\*?([A-Za-z_][A-Za-z0-9_]*)\s*\)\s*([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)[^{]*\{/g;
    for (const match of source.matchAll(pattern)) {
      const opening = source.indexOf('{', match.index + match[0].length - 1);
      const closing = matchingBrace(source, opening);
      if (closing < 0) continue;
      const symbol = `${match[1]}.${match[2]}`;
      const definition = {
        symbol,
        receiver: match[1],
        method: match[2],
        body: source.slice(opening + 1, closing),
        file: file.replaceAll('\\', '/'),
        line: lineAt(source, match.index),
      };
      const entries = definitions.get(symbol) || [];
      entries.push(definition);
      definitions.set(symbol, entries);
    }
    const standalonePattern = /func\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*\)[^{]*\{/g;
    for (const match of source.matchAll(standalonePattern)) {
      const opening = source.indexOf('{', match.index + match[0].length - 1);
      const closing = matchingBrace(source, opening);
      if (closing < 0) continue;
      const symbol = match[1];
      const definition = {
        symbol,
        receiver: '',
        method: match[1],
        body: source.slice(opening + 1, closing),
        file: file.replaceAll('\\', '/'),
        line: lineAt(source, match.index),
      };
      const entries = definitions.get(symbol) || [];
      entries.push(definition);
      definitions.set(symbol, entries);
    }
  }
  return definitions;
}

function findDefinition(definitions, symbol, preferredPath = '') {
  const candidates = definitions.get(symbol) || [];
  if (preferredPath) {
    const normalized = preferredPath.replaceAll('\\', '/');
    const matches = candidates.filter((candidate) => candidate.file.includes(normalized) || candidate.file.startsWith(normalized));
    if (matches.length === 1) return matches[0];
  }
  return candidates.length === 1 ? candidates[0] : null;
}

function optionFields(serverFiles) {
  const result = new Map();
  for (const file of serverFiles) {
    const source = sanitizeGo(fs.readFileSync(file, 'utf8'), { commentsOnly: true });
    const pattern = /func\s+(With[A-Za-z_][A-Za-z0-9_]*Handler)\s*\([^)]*\)\s+Option\s*\{/g;
    for (const match of source.matchAll(pattern)) {
      const opening = source.indexOf('{', match.index + match[0].length - 1);
      const closing = matchingBrace(source, opening);
      const body = closing < 0 ? '' : source.slice(opening + 1, closing);
      const assignment = body.match(/\b[A-Za-z_][A-Za-z0-9_]*\.([A-Za-z_][A-Za-z0-9_]*)\s*=\s*[A-Za-z_][A-Za-z0-9_]*/);
      if (assignment) result.set(match[1], assignment[1]);
    }
  }
  return result;
}

function serverRoutes(serverFiles) {
  const routes = [];
  for (const file of serverFiles) {
    const source = sanitizeGo(fs.readFileSync(file, 'utf8'), { commentsOnly: true });
    for (const match of source.matchAll(/^\s*case\s+(.+):\s*$/gm)) {
      const next = source.slice(match.index + match[0].length).search(/^\s*(?:case\s+.+|default)\s*:\s*$/m);
      const block = source.slice(match.index + match[0].length, next < 0 ? source.length : match.index + match[0].length + next);
      const paths = [...match[1].matchAll(/r\.URL\.Path\s*==\s*"(\/dashboard\/[^"\n]+)"/g)].map((item) => item[1]);
      const methods = [...match[1].matchAll(/r\.Method\s*==\s*(?:http\.)?Method(Get|Post|Put|Patch|Delete|Head)/g)].map((item) => item[1].toUpperCase());
      const fieldMatch = block.match(/(?:\b[A-Za-z_][A-Za-z0-9_]*\.([A-Za-z_][A-Za-z0-9_]*)\.ServeHTTP\s*\(|\bserveHTTPWith(?:Path|Principal)\s*\(\s*[A-Za-z_][A-Za-z0-9_]*\.([A-Za-z_][A-Za-z0-9_]*))/);
      const field = fieldMatch?.[1] || fieldMatch?.[2];
      if (!field) continue;
      for (const method of methods) for (const route of paths) routes.push({ method, route, field, dispatchSource: `${file.replaceAll('\\', '/')}:${lineAt(source, match.index)}` });
    }
  }
  return routes;
}

function compositionBindings(files, fieldsByOption, definitions) {
  const variableTypes = new Map();
  const sources = files.map((file) => ({ file, source: sanitizeGo(fs.readFileSync(file, 'utf8'), { commentsOnly: true }) }));
  for (const { source } of sources) {
    const imports = new Map();
    for (const imported of source.matchAll(/^\s*(?:([A-Za-z_][A-Za-z0-9_]*)\s+)?"jiyi\/mochat-go\/([^"\n]+)"/gm)) {
      imports.set(imported[1] || imported[2].split('/').at(-1), `/${imported[2]}/`);
    }
    for (const match of source.matchAll(/\b([A-Za-z_][A-Za-z0-9_]*)(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)?\s*:?=\s*(?:([A-Za-z_][A-Za-z0-9_]*)\.)?New([A-Za-z_][A-Za-z0-9_]*)\s*\(/g)) {
      variableTypes.set(match[1], { type: constructorType(match[3], definitions), preferredPath: imports.get(match[2]) || (match[2] ? `/${match[2]}/` : '') });
    }
  }
  const bindings = new Map();
  for (const { file, source } of sources) {
    for (const match of source.matchAll(/\b(?:[A-Za-z_][A-Za-z0-9_]*\.)?(With[A-Za-z_][A-Za-z0-9_]*Handler)\s*\(/g)) {
      if (!fieldsByOption.has(match[1])) continue;
      const opening = source.indexOf('(', match.index + match[0].length - 1);
      let depth = 0;
      let closing = -1;
      const code = sanitizeGo(source);
      for (let index = opening; index < code.length; index += 1) {
        if (code[index] === '(') depth += 1;
        else if (code[index] === ')' && --depth === 0) { closing = index; break; }
      }
      if (closing < 0) continue;
      const argument = source.slice(opening + 1, closing);
      let handlerSymbol;
      for (const reference of argument.matchAll(/\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\b/g)) {
        const binding = variableTypes.get(reference[1]);
        if (binding) { handlerSymbol = `${binding.type}.${reference[2]}`; break; }
      }
      if (!handlerSymbol) {
        const simple = argument.trim().match(/^([A-Za-z_][A-Za-z0-9_]*)$/);
        const binding = simple && variableTypes.get(simple[1]);
        if (binding) handlerSymbol = `${binding.type}.ServeHTTP`;
      }
      let handlerDefinition;
      if (!handlerSymbol) {
        const inline = argument.match(/(?:(?<qualifier>[A-Za-z_][A-Za-z0-9_]*)\.)?New(?<constructor>[A-Za-z_][A-Za-z0-9_]*)\s*\(/);
        const type = inline && constructorType(inline.groups.constructor, definitions);
        if (type) {
          handlerDefinition = findDefinition(definitions, `${type}.ServeHTTP`, inline.groups.qualifier ? `/${inline.groups.qualifier}/` : '');
          if (handlerDefinition) handlerSymbol = `${type}.ServeHTTP`;
          else {
            handlerDefinition = findDefinition(definitions, `New${inline.groups.constructor}`, inline.groups.qualifier ? `/${inline.groups.qualifier}/` : '');
            if (handlerDefinition) handlerSymbol = `New${inline.groups.constructor}`;
          }
        }
      }
      if (handlerSymbol && !handlerDefinition) {
        const simple = argument.trim().match(/^([A-Za-z_][A-Za-z0-9_]*)$/)?.[1];
        const preferredPath = (simple && variableTypes.get(simple)?.preferredPath) || '';
        const qualifier = argument.match(/\b([A-Za-z_][A-Za-z0-9_]*)\.New/)?.[1];
        handlerDefinition = findDefinition(definitions, handlerSymbol, preferredPath || (qualifier ? `/${qualifier}/` : ''));
      }
      if (handlerSymbol) bindings.set(fieldsByOption.get(match[1]), { handlerSymbol, handlerDefinition, compositionSource: `${file.replaceAll('\\', '/')}:${lineAt(source, match.index)}` });
    }
  }
  return bindings;
}

function reachableDefinitions(definition, definitions, visited = new Set()) {
  const identity = definition ? `${definition.file}:${definition.symbol}` : '';
  if (!definition || visited.has(identity)) return [];
  visited.add(identity);
  const result = [definition];
  const code = sanitizeGo(definition.body, { preserveAuthorization: true });
  for (const call of code.matchAll(/\b[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?\.([A-Za-z_][A-Za-z0-9_]*)\s*\(/g)) {
    const helper = findDefinition(definitions, `${definition.receiver}.${call[1]}`, definition.file.slice(0, definition.file.lastIndexOf('/') + 1));
    if (helper) result.push(...reachableDefinitions(helper, definitions, visited));
  }
  return result;
}

function operationsIn(definition) {
  const code = sanitizeGo(definition.body, { preserveAuthorization: true });
  const operations = [];
  if (/\.[ \t\r\n]*UserCorpCache\s*\(/.test(code)) operations.push('UserCorpCache');
  if (/\.[ \t\r\n]*DeleteUserCorpCache\s*\(/.test(code)) operations.push('DeleteUserCorpCache');
  if (/\.Header\s*\.\s*Get\s*\(\s*AUTHORIZATION_LITERAL\s*\)/.test(code)) operations.push('Authorization');
  return operations;
}

function auditDashboardAuthContext(root = process.cwd()) {
  const goFiles = walkGo(root);
  const serverFiles = goFiles.filter((file) => file.replaceAll('\\', '/').includes('/internal/server/'));
  const compositionFiles = goFiles.filter((file) => file.replaceAll('\\', '/').includes('/cmd/mochat-go/'));
  const moduleFiles = goFiles.filter((file) => file.replaceAll('\\', '/').includes('/internal/modules/'));
  const productionSources = [...serverFiles, ...compositionFiles, ...moduleFiles].map((file) => ({
    file: file.replaceAll('\\', '/'),
    source: sanitizeGo(fs.readFileSync(file, 'utf8'), { commentsOnly: true }),
  }));
  const definitions = functionDefinitions(goFiles);
  const fields = optionFields(serverFiles);
  const bindings = compositionBindings(compositionFiles, fields, definitions);
  const dispatchRoutes = serverRoutes(serverFiles).map((route) => ({ ...route, ...(bindings.get(route.field) || {}), registrationKind: 'dispatch' })).filter((route) => route.handlerSymbol);
  const moduleRoutes = registeredHandlerRoutes(productionSources, definitions);
  const registry = dashboardRouteRegistry(root);
  const discoveredRoutes = [...dispatchRoutes, ...moduleRoutes];
  const routes = discoveredRoutes.map((route) => {
    const contract = `${route.method} ${route.route}`;
    const metadata = registry.entries.get(contract);
    return { ...route, auth: metadata?.auth, declaredHandlerSymbol: metadata?.handlerSymbol, authSource: registry.source };
  });
  const catalogContracts = extractBackendRegisteredAPIs(productionSources.map(({ file, source }) => ({ file, body: source })))
    .map((item) => item.contract)
    .filter((contract) => !contract.includes(' /dashboard/saasAdmin/') && !contract.includes(' /dashboard/saasAlert/') && !contract.includes(' /dashboard/saasBilling/'));
  const auditedContracts = new Set(discoveredRoutes.map((route) => `${route.method} ${route.route}`));
  const catalogContractSet = new Set(catalogContracts);
  for (const [contract, metadata] of registry.entries) {
    if (!auditedContracts.has(contract) && catalogContractSet.has(contract)) {
      routes.push({ ...metadata, declaredHandlerSymbol: metadata.handlerSymbol, authSource: registry.source, registrationKind: 'conditional', dispatchSource: registry.source });
    }
  }
  const missingContracts = [...new Set([
    ...catalogContracts.filter((contract) => !auditedContracts.has(contract) && !registry.entries.has(contract)),
    ...discoveredRoutes.map((route) => `${route.method} ${route.route}`).filter((contract) => !registry.entries.has(contract)),
  ])];
  const violations = routes.flatMap((route) => {
    if (!route.auth || !route.declaredHandlerSymbol) {
      return [{ method: route.method, route: route.route, handlerSymbol: route.handlerSymbol || 'missing', operation: 'AUTH_METADATA_MISSING', source: route.dispatchSource }];
    }
    if (route.handlerSymbol !== route.declaredHandlerSymbol) {
      return [{ method: route.method, route: route.route, handlerSymbol: route.handlerSymbol, operation: `HANDLER_METADATA_MISMATCH:${route.declaredHandlerSymbol}`, source: route.dispatchSource }];
    }
    return [];
  });
  for (const contract of registry.duplicates) violations.push({ method: contract.split(' ')[0], route: contract.slice(contract.indexOf(' ') + 1), handlerSymbol: 'duplicate', operation: 'DUPLICATE_AUTH_METADATA', source: registry.source });
  for (const [contract, metadata] of registry.entries) {
    if (!auditedContracts.has(contract) && !catalogContractSet.has(contract)) violations.push({ method: metadata.method, route: metadata.route, handlerSymbol: metadata.handlerSymbol, operation: 'ORPHAN_AUTH_METADATA', source: registry.source });
    if (!metadata.auth) violations.push({ method: metadata.method, route: metadata.route, handlerSymbol: metadata.handlerSymbol, operation: `UNKNOWN_AUTH_KIND:${metadata.authToken}`, source: registry.source });
  }
  const uniqueRoutes = [...new Map(routes.map((route) => [`${route.method} ${route.route} ${route.handlerSymbol}`, route])).values()];
  return { routes: uniqueRoutes, violations, catalogContracts, missingContracts };
}

function formatDashboardAuthContextAudit(result, options = {}) {
	const lines = options.includeRoutes
		? result.routes.map((route) => `${route.method} ${route.route} -> ${route.handlerSymbol} auth:${route.auth} auth-source:${route.authSource} dispatch:${route.dispatchSource} composition:${route.compositionSource}`)
		: [];
  for (const violation of result.violations) lines.push(`VIOLATION ${violation.method} ${violation.route} -> ${violation.handlerSymbol} -> ${violation.operation} source:${violation.source}`);
  for (const contract of result.missingContracts) lines.push(`MISSING ${contract} -> no reachable handler symbol`);
  const moduleRoutes = result.routes.filter((route) => route.registrationKind === 'module');
  lines.push(`dashboard auth context gate: dispatch handlers=${new Set(result.routes.filter((route) => route.registrationKind === 'dispatch').map((route) => route.handlerSymbol)).size}, dispatch routes=${result.routes.filter((route) => route.registrationKind === 'dispatch').length}, module handlers=${new Set(moduleRoutes.map((route) => route.handlerSymbol)).size}, module routes=${moduleRoutes.length}, total handlers=${new Set(result.routes.map((route) => route.handlerSymbol)).size}, total routes=${result.routes.length}, catalog contracts=${result.catalogContracts.length}, missing contracts=${result.missingContracts.length}, legacy auth violations=${result.violations.length}`);
  return lines.join('\n');
}

export { auditDashboardAuthContext, formatDashboardAuthContextAudit };

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
	const result = auditDashboardAuthContext();
	console.log(formatDashboardAuthContextAudit(result, { includeRoutes: process.argv.includes('--verbose') }));
  if (result.violations.length || result.missingContracts.length) process.exitCode = 1;
}
