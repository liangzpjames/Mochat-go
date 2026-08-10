import { readFile, writeFile } from 'node:fs/promises';

const catalogPath = 'internal/dashboard/dashboard_page_catalog.json';
const migrationPath = 'deploy/standalone/migrations/0127_dashboard_page_rbac.up.sql';

const document = JSON.parse(await readFile(catalogPath, 'utf8'));
const original = await readFile(migrationPath, 'utf8');
const eol = original.includes('\r\n') ? '\r\n' : '\n';
const resourceInsert = original.indexOf('INSERT INTO `mochat_go_dashboard_permission_resources`');
const marker = `INNER JOIN (${eol}`;
const start = original.indexOf(marker, resourceInsert);
const end = original.indexOf(`${eol}) resource_seed ON`, start);
if (resourceInsert < 0 || start < 0 || end < 0) {
  throw new Error('0127 permission resource seed markers were not found');
}

const quote = (value) => `'${String(value).replaceAll("'", "''")}'`;
const rows = document.pages.flatMap((page) => page.resources.map((resource) => ({
  permissionCode: page.code,
  method: resource.method,
  pathPattern: resource.pathPattern,
  scopeRequired: resource.scopeRequired ? 1 : 0,
})));
const seed = rows.map((row, index) => {
  const prefix = index === 0 ? '  SELECT ' : '  UNION ALL SELECT ';
  const aliases = index === 0
    ? [' AS `permission_code`', ' AS `http_method`', ' AS `path_pattern`', ' AS `scope_required`']
    : ['', '', '', ''];
  return `${prefix}${quote(row.permissionCode)}${aliases[0]}, `
    + `${quote(row.method)}${aliases[1]}, ${quote(row.pathPattern)}${aliases[2]}, `
    + `${row.scopeRequired}${aliases[3]}`;
}).join(eol);

await writeFile(migrationPath, original.slice(0, start + marker.length) + seed + original.slice(end));
console.log(`synced ${rows.length} dashboard permission resource seed rows`);
