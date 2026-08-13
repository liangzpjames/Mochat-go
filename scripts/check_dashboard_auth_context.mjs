import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const defaultExemptions = [
  { method: 'PUT', route: '/dashboard/user/logout', handlerSymbol: 'LogoutHandler.ServeHTTP', operation: 'DeleteUserCorpCache' },
  { method: 'PUT', route: '/dashboard/user/passwordUpdate', handlerSymbol: 'UserAdminHandler.PasswordUpdate', operation: 'DeleteUserCorpCache' },
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
  const code = sanitizeGo(source);
  for (let index = opening; index < code.length; index += 1) {
    if (code[index] === '{') depth += 1;
    else if (code[index] === '}' && --depth === 0) return index;
  }
  return -1;
}

function lineAt(source, index) {
  return source.slice(0, index).split('\n').length;
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
      definitions.set(symbol, {
        symbol,
        receiver: match[1],
        method: match[2],
        body: source.slice(opening + 1, closing),
        file: file.replaceAll('\\', '/'),
        line: lineAt(source, match.index),
      });
    }
  }
  return definitions;
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

function compositionBindings(files, fieldsByOption) {
  const variableTypes = new Map();
  const sources = files.map((file) => ({ file, source: sanitizeGo(fs.readFileSync(file, 'utf8'), { commentsOnly: true }) }));
  for (const { source } of sources) {
    for (const match of source.matchAll(/\b([A-Za-z_][A-Za-z0-9_]*)\s*:?=\s*(?:[A-Za-z_][A-Za-z0-9_]*\.)?New([A-Za-z_][A-Za-z0-9_]*)\s*\(/g)) variableTypes.set(match[1], match[2]);
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
        const type = variableTypes.get(reference[1]);
        if (type) { handlerSymbol = `${type}.${reference[2]}`; break; }
      }
      if (!handlerSymbol) {
        const simple = argument.trim().match(/^([A-Za-z_][A-Za-z0-9_]*)$/);
        const type = simple && variableTypes.get(simple[1]);
        if (type) handlerSymbol = `${type}.ServeHTTP`;
      }
      if (handlerSymbol) bindings.set(fieldsByOption.get(match[1]), { handlerSymbol, compositionSource: `${file.replaceAll('\\', '/')}:${lineAt(source, match.index)}` });
    }
  }
  return bindings;
}

function reachableDefinitions(definition, definitions, visited = new Set()) {
  if (!definition || visited.has(definition.symbol)) return [];
  visited.add(definition.symbol);
  const result = [definition];
  const code = sanitizeGo(definition.body, { preserveAuthorization: true });
  for (const call of code.matchAll(/\b[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?\.([A-Za-z_][A-Za-z0-9_]*)\s*\(/g)) {
    const helper = definitions.get(`${definition.receiver}.${call[1]}`);
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

function auditDashboardAuthContext(root = process.cwd(), options = {}) {
  const goFiles = walkGo(root);
  const serverFiles = goFiles.filter((file) => file.replaceAll('\\', '/').includes('/internal/server/'));
  const compositionFiles = goFiles.filter((file) => file.replaceAll('\\', '/').includes('/cmd/mochat-go/'));
  const definitions = functionDefinitions(goFiles);
  const fields = optionFields(serverFiles);
  const bindings = compositionBindings(compositionFiles, fields);
  const routes = serverRoutes(serverFiles).map((route) => ({ ...route, ...(bindings.get(route.field) || {}) })).filter((route) => route.handlerSymbol);
  const exemptions = options.exemptions || defaultExemptions;
  const violations = [];
  for (const route of routes) {
    const definition = definitions.get(route.handlerSymbol);
    for (const reachable of reachableDefinitions(definition, definitions)) {
      for (const operation of operationsIn(reachable)) {
        const exempt = exemptions.some((item) => item.method === route.method && item.route === route.route && item.handlerSymbol === route.handlerSymbol && item.operation === operation);
        if (!exempt) violations.push({ method: route.method, route: route.route, handlerSymbol: route.handlerSymbol, operation, source: `${reachable.file}:${reachable.line}` });
      }
    }
  }
  violations.sort((left, right) => `${left.route}\t${left.operation}`.localeCompare(`${right.route}\t${right.operation}`));
  const uniqueRoutes = [...new Map(routes.map((route) => [`${route.method} ${route.route} ${route.handlerSymbol}`, route])).values()];
  return { routes: uniqueRoutes, violations };
}

function formatDashboardAuthContextAudit(result, options = {}) {
	const lines = options.includeRoutes
		? result.routes.map((route) => `${route.method} ${route.route} -> ${route.handlerSymbol} dispatch:${route.dispatchSource} composition:${route.compositionSource}`)
		: [];
  for (const violation of result.violations) lines.push(`VIOLATION ${violation.method} ${violation.route} -> ${violation.handlerSymbol} -> ${violation.operation} source:${violation.source}`);
  lines.push(`dashboard auth context gate: handlers=${new Set(result.routes.map((route) => route.handlerSymbol)).size}, routes=${result.routes.length}, legacy auth violations=${result.violations.length}`);
  return lines.join('\n');
}

export { auditDashboardAuthContext, formatDashboardAuthContextAudit };

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
	const result = auditDashboardAuthContext();
	console.log(formatDashboardAuthContextAudit(result, { includeRoutes: process.argv.includes('--verbose') }));
  if (result.violations.length) process.exitCode = 1;
}
