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
function findGoEvidence(root, path) {
  const candidates = ['internal/server', 'internal/dashboard', 'internal/store'].flatMap((directory) => walk(root, directory).filter((file) => file.endsWith('.go')));
  return candidates.find((file) => readFileSync(rootFile(root, file), 'utf8').includes(path)) ?? '-';
}

function discover(root) {
  const discovered = { pages: [], routes: [], apis: [], permissions: [], assets: [], dependencies: [] };
  for (const app of applications) {
    const base = `web/legacy/${app}`;
    if (!fileExists(root, base)) continue;
    const routerFiles = walk(root, `${base}/src/router`).filter((file) => file.endsWith('.js'));
    const routeKeys = new Set();
    for (const router of routerFiles) {
      const content = decomment(readFileSync(rootFile(root, router), 'utf8'));
      for (const match of content.matchAll(/\bpath\s*:\s*['"]([^'"]+)['"]/g)) {
        const path = match[1];
        const key = `${app}:${path}`;
        if (path && !routeKeys.has(key)) { routeKeys.add(key); discovered.routes.push({ app, path, name: routeName(path), source_file: router }); }
      }
    }
    for (const view of walk(root, `${base}/src/views`).filter((file) => /\.(vue|jsx)$/.test(file))) discovered.pages.push({ app, source_file: view });
    for (const route of discovered.routes.filter((item) => item.app === app)) discovered.pages.push({ app, source_file: route.source_file, route: route.path });
    for (const route of discovered.routes.filter((item) => item.app === app)) discovered.permissions.push({ app, route: route.path, source_file: route.source_file });
    for (const apiFile of walk(root, `${base}/src/api`).filter((file) => file.endsWith('.js'))) {
      const content = decomment(readFileSync(rootFile(root, apiFile), 'utf8'));
      for (const match of content.matchAll(/\burl\s*:\s*['"]([^'"]+)['"][\s\S]{0,160}?\bmethod\s*:\s*['"]([^'"]+)['"]/g)) {
        const method = match[2].toUpperCase(); const path = match[1];
        if (!discovered.apis.some((api) => api.app === app && api.method === method && api.path === path)) discovered.apis.push({ app, method, path, source_file: apiFile });
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
    'routes.csv': found.routes.map((item) => ({ ...item, auth: 'unknown', corp_context: 'unknown', permission: '-', render_target: 'legacy' })),
    'apis.csv': found.apis.map((item) => ({ ...item, request_fields: '-', response_fields: '-', auth: 'unknown', corp_scope: 'unknown', go_evidence: findGoEvidence(root, item.path) })),
    'permissions.csv': found.permissions.map((item) => ({ ...item, menu_link_url: item.route, actions: '-' })),
    'assets.csv': found.assets.map((item) => ({ ...item, kind: extensionKind(item.source_file), license_status: 'blocked', used_by: '-' })),
    'dependencies.csv': found.dependencies.map((item) => ({ ...item, replacement: 'TBD', decision: 'replace', risk: 'medium' })),
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
      for (const column of specification.sourceColumns) for (const source of splitReferences(row[column])) if (!fileExists(root, source)) errors.push(`frontend-audit: ${file}:${rowNumber}: ${column} does not exist: ${source}`);
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

function audit(root) {
  const errors = []; const inventory = readInventory(root, errors); const generated = generatedInventory(root); validateManifest(root, errors); compareCoverage(inventory, generated, errors);
  for (const page of inventory['pages.csv'] ?? []) if (!['legacy', 'candidate', 'blocked'].includes(page.status)) errors.push(`frontend-audit: pages.csv:${page.rowNumber}: status must be legacy, candidate, or blocked`);
  const pages = new Set((inventory['pages.csv'] ?? []).map((page) => `${page.app}:${page.route}`));
  for (const route of inventory['routes.csv'] ?? []) if (!pages.has(`${route.app}:${route.path}`)) errors.push(`frontend-audit: routes.csv:${route.rowNumber}: route "${route.path}" has no matching page`);
  const routes = new Set((inventory['routes.csv'] ?? []).map((route) => `${route.app}:${route.path}`));
  for (const permission of inventory['permissions.csv'] ?? []) if (!routes.has(`${permission.app}:${permission.route}`)) errors.push(`frontend-audit: permissions.csv:${permission.rowNumber}: route "${permission.route}" does not exist`);
  for (const api of inventory['apis.csv'] ?? []) if (api.go_evidence !== '-' && (!api.go_evidence.startsWith('internal/') || !/^internal\/(server|dashboard|store)\/.+\.go$/.test(api.go_evidence) || !fileExists(root, api.go_evidence))) errors.push(`frontend-audit: apis.csv:${api.rowNumber}: go_evidence must be '-' or an existing Go path under internal/server, internal/dashboard, or internal/store`);
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
