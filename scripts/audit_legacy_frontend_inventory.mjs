import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import { dirname, extname, join, relative, resolve } from 'node:path';

const sourceCommit = '3dcd216c188df34f2c3ed489b8e8b9473e635488';
const auditDirectory = 'docs/handle/frontend-audit';
const applications = ['dashboard', 'sidebar', 'operation'];
const specifications = {
  'pages.csv': { columns: ['app', 'source_file', 'route', 'status', 'owner', 'risk', 'batch'], key: (row) => `${row.app}:${row.source_file}:${row.route}`, sourceColumns: ['source_file'] },
  'routes.csv': { columns: ['app', 'path', 'name', 'source_file', 'auth', 'corp_context', 'permission', 'render_target'], key: (row) => `${row.app}:${row.path}`, sourceColumns: ['source_file'] },
  'apis.csv': { columns: ['app', 'method', 'path', 'source_file', 'request_fields', 'response_fields', 'auth', 'corp_scope', 'go_evidence'], key: (row) => `${row.app}:${row.method}:${row.path}`, sourceColumns: ['source_file'] },
  'permissions.csv': { columns: ['app', 'route', 'menu_link_url', 'actions', 'source_file'], key: (row) => `${row.app}:${row.route}:${row.menu_link_url}`, sourceColumns: ['source_file'] },
  'assets.csv': { columns: ['app', 'source_file', 'kind', 'license_status', 'used_by'], key: (row) => `${row.app}:${row.source_file}`, sourceColumns: ['source_file'] },
  'dependencies.csv': { columns: ['app', 'package', 'legacy_range', 'replacement', 'decision', 'risk'], key: (row) => `${row.app}:${row.package}`, sourceColumns: [] },
};

function normalize(file) { return file.replaceAll('\\', '/'); }
function rootFile(root, file) { return resolve(root, file); }
function fileExists(root, file) { return existsSync(rootFile(root, file)); }
function walk(root, directory) {
  const fullDirectory = rootFile(root, directory);
  if (!existsSync(fullDirectory)) return [];
  return readdirSync(fullDirectory).flatMap((entry) => {
    const entryPath = normalize(join(directory, entry));
    return statSync(rootFile(root, entryPath)).isDirectory() ? walk(root, entryPath) : [entryPath];
  });
}
function decomment(text) { return text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, ''); }
function escapeRegExp(value) { return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'); }
function syntaxMask(text) {
  const mask = [...text];
  let state = 'code';
  for (let index = 0; index < text.length; index += 1) {
    const character = text[index]; const next = text[index + 1];
    if (state === 'line-comment') {
      if (character === '\n') state = 'code'; else mask[index] = ' ';
      continue;
    }
    if (state === 'block-comment') {
      if (character === '*' && next === '/') { mask[index] = ' '; mask[index + 1] = ' '; index += 1; state = 'code'; }
      else if (character !== '\n') mask[index] = ' ';
      continue;
    }
    if (state !== 'code') {
      mask[index] = character === '\n' ? '\n' : ' ';
      if (character === '\\') { if (index + 1 < text.length) mask[index + 1] = ' '; index += 1; }
      else if ((state === 'single-quote' && character === "'") || (state === 'double-quote' && character === '"') || (state === 'template' && character === '`')) state = 'code';
      continue;
    }
    if (character === '/' && next === '/') { mask[index] = ' '; mask[index + 1] = ' '; index += 1; state = 'line-comment'; }
    else if (character === '/' && next === '*') { mask[index] = ' '; mask[index + 1] = ' '; index += 1; state = 'block-comment'; }
    else if (character === "'") { mask[index] = ' '; state = 'single-quote'; }
    else if (character === '"') { mask[index] = ' '; state = 'double-quote'; }
    else if (character === '`') { mask[index] = ' '; state = 'template'; }
  }
  return mask.join('');
}
function matchingDelimiter(text, opening, mask = syntaxMask(text)) {
  const closingByOpening = { '{': '}', '[': ']', '(': ')' };
  const openingCharacter = text[opening]; const closingCharacter = closingByOpening[openingCharacter];
  if (!closingCharacter) return -1;
  let depth = 0;
  for (let index = opening; index < text.length; index += 1) {
    if (mask[index] === openingCharacter) depth += 1;
    else if (mask[index] === closingCharacter) {
      depth -= 1;
      if (depth === 0) return index;
    }
  }
  return -1;
}
function splitTopLevel(text) {
  const mask = syntaxMask(text); const result = []; const stack = []; const openingByClosing = { '}': '{', ']': '[', ')': '(' };
  let start = 0;
  for (let index = 0; index < text.length; index += 1) {
    const character = mask[index];
    if ('{[('.includes(character)) stack.push(character);
    else if ('}])'.includes(character) && stack.at(-1) === openingByClosing[character]) stack.pop();
    else if (character === ',' && stack.length === 0) { result.push(text.slice(start, index).trim()); start = index + 1; }
  }
  result.push(text.slice(start).trim());
  return result.filter(Boolean);
}
function objectProperties(objectText) {
  const trimmed = objectText.trim(); const inner = trimmed.startsWith('{') ? trimmed.slice(1, trimmed.endsWith('}') ? -1 : undefined) : trimmed;
  return splitTopLevel(inner).map((raw) => {
    const segment = raw.trim();
    if (segment.startsWith('...')) return { spread: segment.slice(3).trim(), raw };
    const mask = syntaxMask(segment); const stack = []; const openingByClosing = { '}': '{', ']': '[', ')': '(' };
    let colon = -1;
    for (let index = 0; index < segment.length; index += 1) {
      const character = mask[index];
      if ('{[('.includes(character)) stack.push(character);
      else if ('}])'.includes(character) && stack.at(-1) === openingByClosing[character]) stack.pop();
      else if (character === ':' && stack.length === 0) { colon = index; break; }
    }
    if (colon === -1) {
      return /^[A-Za-z_$][\w$]*$/.test(segment) ? { key: segment, value: segment, raw } : { raw };
    }
    const keyText = segment.slice(0, colon).trim(); const value = segment.slice(colon + 1).trim();
    const keyMatch = /^(?:([A-Za-z_$][\w$]*)|['"]([^'"]+)['"]|\[\s*(['"])(.*?)\3\s*\])$/.exec(keyText);
    return { key: keyMatch ? (keyMatch[1] || keyMatch[2] || keyMatch[4]) : null, computed: keyMatch ? null : keyText, value, raw };
  });
}
function objectPropertyMap(objectText) {
  return new Map(objectProperties(objectText).filter((property) => property.key).map((property) => [property.key, property.value]));
}
function quotedLiteral(value) { return /^(['"])([\s\S]*)\1$/.exec(value.trim())?.[2] ?? null; }
function enclosingObject(text, position) {
  const mask = syntaxMask(text); const openings = [];
  for (let index = 0; index < position; index += 1) {
    if (mask[index] === '{') openings.push(index);
    else if (mask[index] === '}') openings.pop();
  }
  const opening = openings.at(-1); if (opening === undefined) return null;
  const closing = matchingDelimiter(text, opening, mask);
  return closing === -1 ? null : { opening, closing, text: text.slice(opening, closing + 1) };
}
function readExpression(text, start) {
  let opening = start;
  while (/\s/.test(text[opening] ?? '')) opening += 1;
  const mask = syntaxMask(text); const first = text[opening];
  if ('{[('.includes(first ?? '')) {
    const closing = matchingDelimiter(text, opening, mask);
    return { text: text.slice(opening, closing + 1), start: opening, end: closing + 1 };
  }
  const stack = []; const openingByClosing = { '}': '{', ']': '[', ')': '(' };
  for (let index = opening; index < text.length; index += 1) {
    const character = mask[index];
    if ('{[('.includes(character)) stack.push(character);
    else if ('}])'.includes(character)) {
      if (!stack.length) return { text: text.slice(opening, index).trim(), start: opening, end: index };
      if (stack.at(-1) === openingByClosing[character]) stack.pop();
    } else if (stack.length === 0 && (character === ',' || character === ';' || character === '\n')) {
      return { text: text.slice(opening, index).trim(), start: opening, end: index };
    }
  }
  return { text: text.slice(opening).trim(), start: opening, end: text.length };
}
function braceStackAt(text, position) {
  const mask = syntaxMask(text); const stack = [];
  for (let index = 0; index < position; index += 1) {
    if (mask[index] === '{') stack.push(index);
    else if (mask[index] === '}') stack.pop();
  }
  return stack;
}
function parseCsv(content) {
  const rows = []; let row = []; let field = ''; let quoted = false;
  for (let index = 0; index < content.length; index += 1) {
    const character = content[index];
    if (quoted) { if (character === '"' && content[index + 1] === '"') { field += '"'; index += 1; } else if (character === '"') quoted = false; else field += character; }
    else if (character === '"') quoted = true;
    else if (character === ',') { row.push(field); field = ''; }
    else if (character === '\n') { row.push(field.replace(/\r$/, '')); rows.push(row); row = []; field = ''; }
    else field += character;
  }
  if (field.length || row.length) rows.push([...row, field.replace(/\r$/, '')]);
  return rows.filter((values) => values.some((value) => value.trim()));
}
function csvCell(value) { return /[",\n]/.test(value) ? `"${value.replaceAll('"', '""')}"` : value; }
function csv(file, rows) { return `${specifications[file].columns.join(',')}\n${rows.map((row) => specifications[file].columns.map((column) => csvCell(row[column] ?? '')).join(',')).join('\n')}\n`; }
function splitReferences(value) { return value.split(';').map((item) => item.trim()).filter((item) => item && item !== '-'); }
function extensionKind(file) { return extname(file).slice(1) || 'file'; }
function routeName(path) { return path === '/' ? 'root' : path.replace(/^\//, '').replace(/[^a-zA-Z0-9]+(.)/g, (_, next) => next.toUpperCase()) || 'route'; }
function mountedPath(app, path) { return `/${app}${path.startsWith('/') ? path : `/${path}`}`; }
function methodToken(method) { return `http.Method${method[0]}${method.slice(1).toLowerCase()}`; }
function goContentHasContract(content, method, contract) {
  const code = decomment(content).replace(/\/\/.*$/gm, '');
  const exactContract = `${method} ${contract}`; const token = methodToken(method);
  if (new RegExp(`(?:^|[^A-Z])["\`]${escapeRegExp(exactContract)}["\`]`).test(code)) return true;
  for (const match of code.matchAll(/^\s*case\s+(.+):\s*$/gm)) if (match[1].includes(`"${contract}"`) && match[1].includes(token)) return true;
  for (const match of code.matchAll(/\b(?:httptest\.)?NewRequest\s*\(/g)) {
    const opening = code.indexOf('(', match.index); const closing = matchingDelimiter(code, opening);
    if (closing !== -1) {
      const call = code.slice(opening + 1, closing);
      if (call.includes(token) && new RegExp(`["\`]${escapeRegExp(contract)}(?:\\?[^"\`]*)?["\`]`).test(call)) return true;
    }
  }
  for (const match of code.matchAll(new RegExp(`["\`]${escapeRegExp(contract)}["\`]`, 'g'))) {
    const object = enclosingObject(code, match.index);
    if (!object) continue;
    const properties = objectPropertyMap(object.text);
    if (quotedLiteral(properties.get('path') ?? '') === contract && (properties.get('method') ?? '').trim() === token) return true;
  }
  return false;
}
function findGoEvidence(root, app, method, path) {
  const contract = mountedPath(app, path);
  const candidates = ['internal/server', 'internal/dashboard', 'internal/store'].flatMap((directory) => walk(root, directory).filter((file) => file.endsWith('.go')));
  return candidates.find((file) => {
    const content = readFileSync(rootFile(root, file), 'utf8');
    return goContentHasContract(content, method, contract);
  }) ?? '-';
}
function evidenceHasContract(root, evidence, app, method, path) {
  const content = readFileSync(rootFile(root, evidence), 'utf8'); const contract = mountedPath(app, path);
  return goContentHasContract(content, method, contract);
}
function clientSemantics(app) {
  if (app === 'dashboard') return { auth: 'ACCESS_TOKEN', corp: 'dashboard-corp-context', target: 'dashboard-spa' };
  if (app === 'sidebar') return { auth: 'Bearer cookie token', corp: 'sidebar-corp-context', target: 'sidebar-embedded' };
  return { auth: 'no-auth-header', corp: 'operation-corp-context', target: 'operation-spa' };
}
function routeAuth(app, path) {
  if (app === 'dashboard') return path === '/login' ? 'public' : 'ACCESS_TOKEN';
  if (app === 'sidebar') return ['/', '/login', '/auth', '/codeAuth'].includes(path) ? 'public' : 'Bearer cookie token';
  return 'no-auth-header';
}
function replacementFor(pkg) {
  const replacements = { vue: 'react', 'vue-router': 'react-router', vuex: 'zustand', axios: 'fetch-wrapper', 'ant-design-vue': 'antd', vant: 'antd-mobile', 'vue-i18n': 'react-intl', 'vue-echarts': 'echarts-for-react', 'vue-quill-editor': 'react-quill', 'vue-clipboard2': 'clipboard-copy', 'vue-cropper': 'react-easy-crop', 'vue-drag-resize': 'react-rnd', 'vue-pdf': 'react-pdf', 'vue-luck-draw': 'react-custom-roulette' };
  return replacements[pkg] ?? 'no-direct-react-replacement';
}
function decisionFor(pkg) { return replacementFor(pkg) === 'no-direct-react-replacement' ? 'retire-or-reassess' : 'replace'; }
function componentSource(root, app, content, component) {
  if (!component) return null;
  const dynamic = /(?:\(\)\s*=>\s*import\s*\(\s*['"])(?:@\/)?views\/([^'"]+)/.exec(component);
  if (dynamic) {
    const base = `web/legacy/${app}/src/views/${dynamic[1]}`;
    return [ `${base}.vue`, `${base}/index.vue` ].find((file) => fileExists(root, file)) ?? null;
  }
  const staticComponent = /^([A-Za-z_$][\w$]*)$/.exec(component.trim())?.[1];
  if (!staticComponent) return null;
  const imports = [...content.matchAll(new RegExp(`import\\s+${staticComponent}\\s+from\\s+['"]([^'"]+)['"]`, 'g'))];
  if (!imports.length) return null;
  const raw = imports[0][1].replace(/^@\//, '');
  const normalized = raw.startsWith('views/') ? raw.slice(6) : raw.replace(/^\.\.\/views\//, '');
  const base = `web/legacy/${app}/src/views/${normalized}`;
  return [ `${base}.vue`, `${base}/index.vue` ].find((file) => fileExists(root, file)) ?? null;
}
function actionEvidence(root, route, component) {
  if (!component || !fileExists(root, component)) return 'implicit-route-access';
  const text = readFileSync(rootFile(root, component), 'utf8');
  const actions = [...text.matchAll(new RegExp(`${route.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}@([^'"\\s]+)`, 'g'))].map((match) => match[1]);
  return actions.length ? [...new Set(actions)].sort().join(';') : 'implicit-route-access';
}
function assetUses(root, asset) {
  const name = asset.split('/').at(-1);
  const code = applications.flatMap((app) => walk(root, `web/legacy/${app}/src`)).filter((file) => !file.startsWith(asset) && /\.(vue|jsx|js|css|less|scss|html)$/.test(file));
  const usedBy = code.filter((file) => readFileSync(rootFile(root, file), 'utf8').includes(name));
  return usedBy.length ? usedBy.sort().join(';') : 'unreferenced-in-legacy-source';
}

function scriptSource(file, content) {
  if (!file.endsWith('.vue')) return content;
  return /<script(?:\s[^>]*)?>([\s\S]*?)<\/script>/i.exec(content)?.[1] ?? '';
}
function resolveApiImport(root, app, sourceFile, imported) {
  let absolute;
  if (imported.startsWith('@/')) absolute = rootFile(root, `web/legacy/${app}/src/${imported.slice(2)}`);
  else absolute = resolve(dirname(rootFile(root, sourceFile)), imported);
  if (!absolute.endsWith('.js')) absolute += '.js';
  return normalize(relative(root, absolute));
}
function collectApiCallsites(root, app) {
  const result = new Map(); const base = `web/legacy/${app}/src`;
  const sources = walk(root, base).filter((file) => /\.(js|jsx|vue)$/.test(file) && !file.includes('/src/api/'));
  for (const sourceFile of sources) {
    const sourceContent = readFileSync(rootFile(root, sourceFile), 'utf8');
    const modelEvidence = [...sourceContent.matchAll(/\bv-model(?:\.[\w-]+)*\s*=\s*["'](?:this\.)?([A-Za-z_$][\w$]*)\.([A-Za-z_$][\w$]*)["']/g)]
      .map((match) => `this.${match[1]}.${match[2]} = undefined`).join('\n');
    const text = `${decomment(scriptSource(sourceFile, sourceContent))}\n${modelEvidence}`;
    for (const imported of text.matchAll(/import\s*\{([^}]*)\}\s*from\s*['"]([^'"]*(?:\/api\/|@\/api\/)[^'"]*)['"]/g)) {
      const apiFile = resolveApiImport(root, app, sourceFile, imported[2]);
      for (const importedName of imported[1].split(',').map((name) => name.trim()).filter(Boolean)) {
        const [exportName, alias] = importedName.split(/\s+as\s+/); const localName = alias ?? exportName;
        const callPattern = new RegExp(`(?<![.$\\w])${escapeRegExp(localName)}\\s*\\(`, 'g');
        for (const call of text.matchAll(callPattern)) {
          const opening = text.indexOf('(', call.index); const closing = matchingDelimiter(text, opening);
          if (closing === -1 || /^\s*\{/.test(text.slice(closing + 1))) continue;
          const argumentsList = splitTopLevel(text.slice(opening + 1, closing));
          const key = `${apiFile}:${exportName}`;
          if (!result.has(key)) result.set(key, []);
          result.get(key).push({ text, position: call.index, argument: (argumentsList[0] ?? '').trim(), sourceFile });
        }
      }
    }
  }
  return result;
}
function fieldEvidence() { return { fields: new Set(), blockers: new Set(), hasEvidence: false }; }
function mergeFieldEvidence(target, source) {
  for (const field of source.fields) target.fields.add(field);
  for (const blocker of source.blockers) target.blockers.add(blocker);
  target.hasEvidence ||= source.hasEvidence;
  return target;
}
function assignedFields(text, target, start, end) {
  const result = fieldEvidence(); const escaped = escapeRegExp(target); const source = text.slice(start, end);
  const patterns = [
    new RegExp(`${escaped}\\s*\\.\\s*([A-Za-z_$][\\w$]*)\\s*=`, 'g'),
    new RegExp(`${escaped}\\s*\\[\\s*['"]([^'"]+)['"]\\s*\\]\\s*=`, 'g'),
    new RegExp(`${escaped}\\s*\\.\\s*append\\s*\\(\\s*['"]([^'"]+)['"]`, 'g'),
  ];
  for (const pattern of patterns) for (const match of source.matchAll(pattern)) { result.fields.add(match[1]); result.hasEvidence = true; }
  return result;
}
function enclosingMethod(text, position, parameter) {
  for (const opening of braceStackAt(text, position).reverse()) {
    const prefix = text.slice(Math.max(0, opening - 300), opening);
    const method = /(?:^|[,\n{])\s*(?:async\s+)?([A-Za-z_$][\w$]*)\s*\(([^)]*)\)\s*$/.exec(prefix);
    if (!method) continue;
    const parameters = splitTopLevel(method[2]).map((item) => item.split('=')[0].trim());
    const parameterIndex = parameters.indexOf(parameter);
    if (parameterIndex !== -1) return { name: method[1], opening, parameterIndex, defaults: splitTopLevel(method[2]) };
  }
  return null;
}
function resolveLocalMethodParameter(text, method, depth, seen) {
  const result = fieldEvidence(); const key = `method:${method.name}:${method.parameterIndex}`;
  if (seen.has(key)) { result.blockers.add('cyclic-callsite'); return result; }
  const nextSeen = new Set(seen); nextSeen.add(key);
  const pattern = new RegExp(`(?<![\\w$])(?:this\\s*\\.\\s*)?${escapeRegExp(method.name)}\\s*\\(`, 'g');
  let calls = 0;
  for (const call of text.matchAll(pattern)) {
    const opening = text.indexOf('(', call.index); const closing = matchingDelimiter(text, opening);
    if (closing === -1 || /^\s*\{/.test(text.slice(closing + 1)) || opening < method.opening && closing > method.opening) continue;
    const argumentsList = splitTopLevel(text.slice(opening + 1, closing)); const argument = (argumentsList[method.parameterIndex] ?? '').trim();
    calls += 1;
    mergeFieldEvidence(result, resolveExpressionFields(text, argument, call.index, depth + 1, nextSeen));
  }
  const defaultValue = method.defaults[method.parameterIndex]?.split('=').slice(1).join('=').trim();
  if (!calls && defaultValue) mergeFieldEvidence(result, resolveExpressionFields(text, defaultValue, method.opening, depth + 1, nextSeen));
  if (!calls && !defaultValue) result.blockers.add('opaque-method-parameter');
  return result;
}
function resolveExpressionFields(text, expression, position, depth = 0, seen = new Set()) {
  const result = fieldEvidence(); const value = expression.trim();
  if (depth > 7) { result.blockers.add('resolution-depth'); return result; }
  if (!value || value === 'undefined' || value === 'null') { result.hasEvidence = true; return result; }
  if (value.startsWith('{')) {
    result.hasEvidence = true;
    for (const property of objectProperties(value)) {
      if (property.key) result.fields.add(property.key);
      else if (property.spread) mergeFieldEvidence(result, resolveExpressionFields(text, property.spread, position, depth + 1, new Set(seen)));
      else result.blockers.add(property.computed ? 'computed-property' : 'unparsed-property');
    }
    return result;
  }
  if (/^new\s+FormData\s*\(/.test(value)) { result.hasEvidence = true; return result; }
  if (/^this\.[A-Za-z_$][\w$]*$/.test(value)) {
    if (seen.has(value)) { result.blockers.add('cyclic-reference'); return result; }
    const nextSeen = new Set(seen); nextSeen.add(value); const escaped = escapeRegExp(value);
    for (const assignment of text.slice(0, position).matchAll(new RegExp(`${escaped}\\s*=\\s*`, 'g'))) {
      const expressionValue = readExpression(text, assignment.index + assignment[0].length);
      mergeFieldEvidence(result, resolveExpressionFields(text, expressionValue.text, assignment.index, depth + 1, nextSeen));
    }
    const member = value.slice(5); const propertyPattern = new RegExp(`(?:^|[,\\n{])\\s*${escapeRegExp(member)}\\s*:\\s*`, 'g');
    for (const property of text.matchAll(propertyPattern)) {
      const expressionValue = readExpression(text, property.index + property[0].length);
      mergeFieldEvidence(result, resolveExpressionFields(text, expressionValue.text, property.index, depth + 1, nextSeen));
    }
    mergeFieldEvidence(result, assignedFields(text, value, 0, text.length));
    if (!result.hasEvidence && !result.blockers.size) result.blockers.add('opaque-member');
    return result;
  }
  if (/^[A-Za-z_$][\w$]*$/.test(value)) {
    if (seen.has(value)) { result.blockers.add('cyclic-reference'); return result; }
    const nextSeen = new Set(seen); nextSeen.add(value); const escaped = escapeRegExp(value);
    let scopeStart = null; let initializer = null;
    for (const opening of [...braceStackAt(text, position).reverse(), -1]) {
      const scope = text.slice(opening + 1, position);
      const assignments = [...scope.matchAll(new RegExp(`(?:\\b(?:const|let|var)\\s+${escaped}|(?<![.\\w$])${escaped})\\s*=\\s*`, 'g'))];
      if (!assignments.length) continue;
      const assignment = assignments.at(-1); scopeStart = opening + 1;
      initializer = readExpression(text, scopeStart + assignment.index + assignment[0].length);
      break;
    }
    if (initializer) {
      mergeFieldEvidence(result, resolveExpressionFields(text, initializer.text, initializer.start, depth + 1, nextSeen));
      mergeFieldEvidence(result, assignedFields(text, value, scopeStart, position));
      for (const assignment of text.slice(scopeStart, position).matchAll(new RegExp(`Object\\.assign\\s*\\(\\s*${escaped}\\s*,\\s*`, 'g'))) {
        const absolute = scopeStart + assignment.index + assignment[0].length; const assigned = readExpression(text, absolute);
        mergeFieldEvidence(result, resolveExpressionFields(text, assigned.text, absolute, depth + 1, nextSeen));
      }
      if (!result.hasEvidence && !result.blockers.size) result.blockers.add('opaque-variable');
      return result;
    }
    mergeFieldEvidence(result, assignedFields(text, value, 0, position));
    const method = enclosingMethod(text, position, value);
    if (method) mergeFieldEvidence(result, resolveLocalMethodParameter(text, method, depth, nextSeen));
    if (!result.hasEvidence && !result.blockers.size) result.blockers.add('opaque-variable');
    return result;
  }
  const helper = /^this\.([A-Za-z_$][\w$]*)\s*\(([\s\S]*)\)$/.exec(value);
  if (helper) {
    const pattern = new RegExp(`(?:^|[,\\n{])\\s*${escapeRegExp(helper[1])}\\s*\\([^)]*\\)\\s*\\{`, 'g');
    for (const method of text.matchAll(pattern)) {
      const opening = text.indexOf('{', method.index + method[0].length - 1); const closing = matchingDelimiter(text, opening);
      const body = text.slice(opening + 1, closing);
      for (const returned of body.matchAll(/\breturn\s+/g)) {
        const absolute = opening + 1 + returned.index + returned[0].length; const expressionValue = readExpression(text, absolute);
        mergeFieldEvidence(result, resolveExpressionFields(text, expressionValue.text, absolute, depth + 1, new Set(seen)));
      }
    }
    if (!result.hasEvidence && !result.blockers.size) result.blockers.add('opaque-helper');
    return result;
  }
  result.blockers.add(/^[\w$]+(?:\.[\w$]+)+$/.test(value) ? 'scalar-expression' : 'dynamic-expression');
  return result;
}
function formatRequestFields(transport, evidence) {
  const fields = [...evidence.fields].sort(); const blockers = [...evidence.blockers].sort();
  const values = [...fields];
  if (blockers.length) values.push(`blocked[${blockers.join('|')}]`);
  if (!values.length) values.push('none');
  return `${transport}:${values.join(';')}`;
}
function requestFieldsFor(api, calls) {
  if (!api.transport) return 'none';
  const evidence = fieldEvidence();
  const firstParameter = splitTopLevel(api.parameters)[0]?.split('=')[0].trim() ?? '';
  const forwarded = /^[A-Za-z_$][\w$]*$/.test(firstParameter) && api.payload.trim() === firstParameter;
  if (!forwarded) {
    const position = Math.max(0, api.body.lastIndexOf(api.payload));
    mergeFieldEvidence(evidence, resolveExpressionFields(api.body, api.payload, position));
  }
  if (forwarded || evidence.blockers.has('opaque-variable') || evidence.blockers.has('cyclic-reference')) {
    const callEvidence = fieldEvidence();
    for (const call of calls) mergeFieldEvidence(callEvidence, resolveExpressionFields(call.text, call.argument, call.position));
    if (!calls.length) callEvidence.blockers.add('no-callsite');
    mergeFieldEvidence(evidence, callEvidence);
    if (callEvidence.hasEvidence && !callEvidence.blockers.size) {
      evidence.blockers.delete('opaque-variable'); evidence.blockers.delete('cyclic-reference');
    }
  }
  return formatRequestFields(api.transport, evidence);
}

function discover(root) {
  const discovered = { pages: [], routes: [], apis: [], permissions: [], assets: [], dependencies: [] };
  for (const app of applications) {
    const base = `web/legacy/${app}`;
    if (!fileExists(root, base)) continue;
    const apiCallsites = collectApiCallsites(root, app);
    const routerFiles = walk(root, `${base}/src/router`).filter((file) => file.endsWith('.js'));
    const routeKeys = new Set();
    for (const router of routerFiles) {
      const content = decomment(readFileSync(rootFile(root, router), 'utf8'));
      for (const match of content.matchAll(/\bpath\s*:\s*['"]([^'"]+)['"]/g)) {
        const object = enclosingObject(content, match.index); if (!object) continue;
        const properties = objectPropertyMap(object.text); const path = quotedLiteral(properties.get('path') ?? '');
        if (path !== match[1] || !['component', 'redirect', 'children'].some((property) => properties.has(property))) continue;
        const key = `${app}:${path}`;
        if (path && !routeKeys.has(key)) {
          const name = quotedLiteral(properties.get('name') ?? '') ?? '-';
          routeKeys.add(key); discovered.routes.push({ app, path, name, source_file: router, component: componentSource(root, app, content, properties.get('component')) });
        }
      }
    }
    const appRoutes = discovered.routes.filter((item) => item.app === app);
    for (const view of walk(root, `${base}/src/views`).filter((file) => /\.(vue|jsx)$/.test(file))) {
      const route = appRoutes.find((item) => item.component === view)?.path ?? '-';
      discovered.pages.push({ app, source_file: view, route });
    }
    for (const route of appRoutes.filter((item) => !item.component)) discovered.pages.push({ app, source_file: route.source_file, route: route.path });
    for (const route of appRoutes) discovered.permissions.push({ app, route: route.path, source_file: route.component ?? route.source_file, component: route.component });
    for (const apiFile of walk(root, `${base}/src/api`).filter((file) => file.endsWith('.js'))) {
      const content = decomment(readFileSync(rootFile(root, apiFile), 'utf8'));
      for (const exported of content.matchAll(/\bexport\s+function\s+([A-Za-z_$][\w$]*)\s*\(([^)]*)\)\s*\{/g)) {
        const bodyOpening = content.indexOf('{', exported.index + exported[0].length - 1); const bodyClosing = matchingDelimiter(content, bodyOpening);
        if (bodyClosing === -1) continue;
        const body = content.slice(bodyOpening + 1, bodyClosing); const request = /\b(?:request|newRequest|request_op)\s*\(/.exec(body);
        if (!request) continue;
        const callOpening = body.indexOf('(', request.index); const callClosing = matchingDelimiter(body, callOpening);
        const configOpening = body.indexOf('{', callOpening);
        if (callClosing === -1 || configOpening === -1 || configOpening > callClosing) continue;
        const configClosing = matchingDelimiter(body, configOpening); if (configClosing === -1 || configClosing > callClosing) continue;
        const config = body.slice(configOpening, configClosing + 1); const properties = objectPropertyMap(config);
        const path = quotedLiteral(properties.get('url') ?? ''); const methodValue = quotedLiteral(properties.get('method') ?? '');
        if (!path || !methodValue) continue;
        const method = methodValue.toUpperCase(); const transport = properties.has('data') ? 'data' : properties.has('params') ? 'params' : null;
        const payload = transport ? properties.get(transport) : '';
        const api = { app, method, path, source_file: apiFile, exportName: exported[1], parameters: exported[2], body, transport, payload };
        const request_fields = requestFieldsFor(api, apiCallsites.get(`${apiFile}:${api.exportName}`) ?? []);
        if (!discovered.apis.some((item) => item.app === app && item.method === method && item.path === path)) discovered.apis.push({ app, method, path, source_file: apiFile, request_fields });
      }
      for (const exported of content.matchAll(/\bexport\s+const\s+([A-Za-z_$][\w$]*)\s*=\s*(?:\(([^)]*)\)|([A-Za-z_$][\w$]*))\s*=>/g)) {
        const expressionStart = exported.index + exported[0].length; const remaining = content.slice(expressionStart);
        const boundary = remaining.search(/(?:;\s*|\n\s*)export\s+/); const body = boundary === -1 ? remaining : remaining.slice(0, boundary);
        const request = /\b(?:request|newRequest|request_op)\s*\(/.exec(body); if (!request) continue;
        const callOpening = body.indexOf('(', request.index); const callClosing = matchingDelimiter(body, callOpening);
        const configOpening = body.indexOf('{', callOpening);
        if (callClosing === -1 || configOpening === -1 || configOpening > callClosing) continue;
        const configClosing = matchingDelimiter(body, configOpening); if (configClosing === -1 || configClosing > callClosing) continue;
        const properties = objectPropertyMap(body.slice(configOpening, configClosing + 1));
        const path = quotedLiteral(properties.get('url') ?? ''); const methodValue = quotedLiteral(properties.get('method') ?? '');
        if (!path || !methodValue) continue;
        const method = methodValue.toUpperCase(); const transport = properties.has('data') ? 'data' : properties.has('params') ? 'params' : null;
        const payload = transport ? properties.get(transport) : '';
        const api = { app, method, path, source_file: apiFile, exportName: exported[1], parameters: exported[2] ?? exported[3] ?? '', body, transport, payload };
        const request_fields = requestFieldsFor(api, apiCallsites.get(`${apiFile}:${api.exportName}`) ?? []);
        if (!discovered.apis.some((item) => item.app === app && item.method === method && item.path === path)) discovered.apis.push({ app, method, path, source_file: apiFile, request_fields });
      }
    }
    for (const asset of [...walk(root, `${base}/src/assets`), ...walk(root, `${base}/src/static`)]) discovered.assets.push({ app, source_file: asset });
    const packageFile = `${base}/package.json`;
    if (fileExists(root, packageFile)) for (const [pkg, range] of Object.entries(JSON.parse(readFileSync(rootFile(root, packageFile), 'utf8')).dependencies ?? {})) discovered.dependencies.push({ app, package: pkg, legacy_range: String(range) });
  }
  for (const key of Object.keys(discovered)) discovered[key].sort((left, right) => JSON.stringify(left).localeCompare(JSON.stringify(right)));
  return discovered;
}

function generatedInventory(root) {
  const found = discover(root);
  return {
    'pages.csv': found.pages.map((item) => ({ ...item, route: item.route ?? '-', status: 'legacy', owner: 'unassigned', risk: 'medium', batch: 'unassigned' })),
    'routes.csv': found.routes.map((item) => ({ ...item, ...clientSemantics(item.app), auth: routeAuth(item.app, item.path), corp_context: clientSemantics(item.app).corp, permission: actionEvidence(root, item.path, item.component), render_target: clientSemantics(item.app).target })),
    'apis.csv': found.apis.map((item) => ({ ...item, response_fields: 'response.data', auth: clientSemantics(item.app).auth, corp_scope: `/${item.app}`, go_evidence: findGoEvidence(root, item.app, item.method, item.path) })),
    'permissions.csv': found.permissions.map((item) => ({ ...item, menu_link_url: item.route, actions: actionEvidence(root, item.route, item.component) })),
    'assets.csv': found.assets.map((item) => ({ ...item, kind: extensionKind(item.source_file), license_status: 'blocked', used_by: assetUses(root, item.source_file) })),
    'dependencies.csv': found.dependencies.map((item) => ({ ...item, replacement: replacementFor(item.package), decision: decisionFor(item.package), risk: item.package === 'vue' || item.package === 'vue-router' ? 'high' : 'medium' })),
  };
}

function readInventory(root, errors) {
  const result = {};
  for (const [file, specification] of Object.entries(specifications)) {
    const path = `${auditDirectory}/${file}`;
    if (!fileExists(root, path)) { errors.push(`frontend-audit: ${file}:1: file is required`); result[file] = []; continue; }
    const rows = parseCsv(readFileSync(rootFile(root, path), 'utf8')); const header = rows.shift() ?? [];
    if (header.join(',') !== specification.columns.join(',')) { errors.push(`frontend-audit: ${file}:1: columns must be ${specification.columns.join(',')}`); result[file] = []; continue; }
    const seen = new Set();
    result[file] = rows.map((values, index) => {
      const rowNumber = index + 2; const row = Object.fromEntries(specification.columns.map((column, position) => [column, (values[position] ?? '').trim()]));
      if (values.length !== specification.columns.length) errors.push(`frontend-audit: ${file}:${rowNumber}: expected ${specification.columns.length} columns`);
      for (const column of specification.columns) if (!row[column]) errors.push(`frontend-audit: ${file}:${rowNumber}: ${column} is required`);
      const key = specification.key(row); if (seen.has(key)) errors.push(`frontend-audit: ${file}:${rowNumber}: duplicate key ${key}`); seen.add(key);
      for (const column of specification.sourceColumns) for (const source of splitReferences(row[column])) if (source !== 'unreferenced-in-legacy-source' && !fileExists(root, source)) errors.push(`frontend-audit: ${file}:${rowNumber}: ${column} does not exist: ${source}`);
      return { ...row, rowNumber };
    });
  }
  return result;
}

function validateManifest(root, errors) {
  const manifest = 'web/legacy/SOURCE_MANIFEST.sha256';
  if (!fileExists(root, manifest)) { errors.push('frontend-audit: SOURCE_MANIFEST.sha256:1: file is required'); return; }
  const expected = new Set(applications.flatMap((app) => walk(root, `web/legacy/${app}`)));
  const seen = new Set();
  readFileSync(rootFile(root, manifest), 'utf8').split(/\r?\n/).filter(Boolean).forEach((line, index) => {
    const match = /^([a-fA-F0-9]{64})  (.+)$/.exec(line); const row = index + 1;
    if (!match) { errors.push(`frontend-audit: SOURCE_MANIFEST.sha256:${row}: expected SHA256 and source file path`); return; }
    const [, expectedHash, source] = match;
    if (seen.has(source)) errors.push(`frontend-audit: SOURCE_MANIFEST.sha256:${row}: duplicate source file: ${source}`);
    seen.add(source);
    if (!fileExists(root, source)) { errors.push(`frontend-audit: SOURCE_MANIFEST.sha256:${row}: source file does not exist: ${source}`); return; }
    const actualHash = createHash('sha256').update(readFileSync(rootFile(root, source))).digest('hex');
    if (actualHash !== expectedHash.toLowerCase()) errors.push(`frontend-audit: SOURCE_MANIFEST.sha256:${row}: SHA256 does not match: ${source}`);
  });
  for (const source of [...expected].sort()) if (!seen.has(source)) errors.push(`frontend-audit: SOURCE_MANIFEST.sha256:1: source file is missing from manifest: ${source}`);
  for (const source of [...seen].sort()) if (!expected.has(source)) errors.push(`frontend-audit: SOURCE_MANIFEST.sha256:1: manifest source is not a legacy frontend file: ${source}`);
  const readme = 'web/legacy/README.md';
  if (!fileExists(root, readme) || !readFileSync(rootFile(root, readme), 'utf8').includes(`Source commit: \`${sourceCommit}\``)) errors.push(`frontend-audit: README.md:1: source commit must be ${sourceCommit}`);
}

function compareCoverage(inventory, generated, errors) {
  const labels = { 'pages.csv': 'page', 'routes.csv': 'route', 'apis.csv': 'API', 'permissions.csv': 'permission', 'assets.csv': 'asset', 'dependencies.csv': 'dependency' };
  for (const [file, specification] of Object.entries(specifications)) {
    const actual = new Set((inventory[file] ?? []).map(specification.key)); const expected = new Set((generated[file] ?? []).map(specification.key));
    for (const key of [...expected].sort()) if (!actual.has(key)) errors.push(`frontend-audit: ${file}:1: discovered ${labels[file]} is missing: ${key}`);
    for (const key of [...actual].sort()) if (!expected.has(key)) errors.push(`frontend-audit: ${file}:1: inventory ${labels[file]} is not discovered: ${key}`);
  }
}
function compareDerivedEvidence(inventory, generated, errors) {
  const apiSpecification = specifications['apis.csv']; const expectedApis = new Map((generated['apis.csv'] ?? []).map((row) => [apiSpecification.key(row), row]));
  for (const api of inventory['apis.csv'] ?? []) {
    const expected = expectedApis.get(apiSpecification.key(api)); if (!expected) continue;
    if (api.request_fields !== expected.request_fields) errors.push(`frontend-audit: apis.csv:${api.rowNumber}: request_fields must match discovered evidence ${expected.request_fields}`);
  }
  const routeSpecification = specifications['routes.csv']; const expectedRoutes = new Map((generated['routes.csv'] ?? []).map((row) => [routeSpecification.key(row), row]));
  for (const route of inventory['routes.csv'] ?? []) {
    const expected = expectedRoutes.get(routeSpecification.key(route)); if (!expected) continue;
    if (route.name !== expected.name) errors.push(`frontend-audit: routes.csv:${route.rowNumber}: name must match route declaration ${expected.name}`);
    if (route.source_file !== expected.source_file) errors.push(`frontend-audit: routes.csv:${route.rowNumber}: source_file must match route declaration ${expected.source_file}`);
    if (route.auth !== expected.auth) errors.push(`frontend-audit: routes.csv:${route.rowNumber}: auth must match route guard evidence ${expected.auth}`);
  }
}

function audit(root) {
  const errors = []; const inventory = readInventory(root, errors); const generated = generatedInventory(root); validateManifest(root, errors); compareCoverage(inventory, generated, errors); compareDerivedEvidence(inventory, generated, errors);
  for (const page of inventory['pages.csv'] ?? []) if (!['legacy', 'candidate', 'blocked'].includes(page.status)) errors.push(`frontend-audit: pages.csv:${page.rowNumber}: status must be legacy, candidate, or blocked`);
  const pages = new Set((inventory['pages.csv'] ?? []).map((page) => `${page.app}:${page.route}`));
  for (const route of inventory['routes.csv'] ?? []) if (!pages.has(`${route.app}:${route.path}`)) errors.push(`frontend-audit: routes.csv:${route.rowNumber}: route "${route.path}" has no matching page`);
  const routes = new Set((inventory['routes.csv'] ?? []).map((route) => `${route.app}:${route.path}`));
  for (const permission of inventory['permissions.csv'] ?? []) if (!routes.has(`${permission.app}:${permission.route}`)) errors.push(`frontend-audit: permissions.csv:${permission.rowNumber}: route "${permission.route}" does not exist`);
  for (const api of inventory['apis.csv'] ?? []) if (api.go_evidence !== '-') {
    if (!api.go_evidence.startsWith('internal/') || !/^internal\/(server|dashboard|store)\/.+\.go$/.test(api.go_evidence) || !fileExists(root, api.go_evidence)) errors.push(`frontend-audit: apis.csv:${api.rowNumber}: go_evidence must be '-' or an existing Go path under internal/server, internal/dashboard, or internal/store`);
    else if (!evidenceHasContract(root, api.go_evidence, api.app, api.method, api.path)) errors.push(`frontend-audit: apis.csv:${api.rowNumber}: go_evidence does not prove ${api.method} ${mountedPath(api.app, api.path)}`);
  }
  for (const [file, rows] of Object.entries(inventory)) for (const row of rows) for (const [column, value] of Object.entries(row)) if (column !== 'rowNumber' && /\b(TBD|TODO|unknown)\b/i.test(value)) errors.push(`frontend-audit: ${file}:${row.rowNumber}: ${column} contains a prohibited placeholder`);
  return { errors, inventory };
}

function refresh(root) {
  const inventory = generatedInventory(root); mkdirSync(rootFile(root, auditDirectory), { recursive: true } );
  for (const [file, rows] of Object.entries(inventory)) writeFileSync(rootFile(root, `${auditDirectory}/${file}`), csv(file, rows));
  const sources = applications.flatMap((app) => walk(root, `web/legacy/${app}`)).sort();
  writeFileSync(rootFile(root, 'web/legacy/SOURCE_MANIFEST.sha256'), `${sources.map((file) => `${createHash('sha256').update(readFileSync(rootFile(root, file))).digest('hex')}  ${file}`).join('\n')}\n`);
}

const argumentsList = process.argv.slice(2); const rootIndex = argumentsList.indexOf('--root'); const root = rootIndex === -1 ? process.cwd() : resolve(argumentsList[rootIndex + 1] ?? '');
if (argumentsList.includes('--refresh')) refresh(root);
if (!argumentsList.includes('--check')) { console.error('frontend-audit: usage: node scripts/audit_legacy_frontend_inventory.mjs --check [--refresh] [--root <path>]'); process.exitCode = 1; }
else { const { errors, inventory } = audit(root); if (errors.length) { console.error(errors.join('\n')); process.exitCode = 1; } else console.log(`frontend-audit: ok (${inventory['pages.csv'].length} pages, ${inventory['routes.csv'].length} routes, ${inventory['apis.csv'].length} apis)`); }
