import { existsSync, readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export const allowedStates = new Set(['passed']);

export const isAcceptedState = (value) =>
  allowedStates.has(value) || /^not-applicable:.+/.test(value);

const capabilityColumns = [
  'read',
  'write',
  'permissions',
  'fake_external',
  'component_tests',
  'browser',
];

const genericMigrationPages = new Set([
  'web/apps/dashboard/src/pages/migrated-dashboard-page.tsx',
  'web/apps/sidebar/src/page.tsx',
  'web/apps/operation/src/page.tsx',
]);

const splitList = (value) => String(value ?? '')
  .split(';')
  .map((item) => item.trim())
  .filter(Boolean);

const parseCsvLine = (line) => {
  const fields = [];
  let current = '';
  let quoted = false;
  for (let index = 0; index < line.length; index += 1) {
    const character = line[index];
    if (character === '"') {
      if (quoted && line[index + 1] === '"') {
        current += '"';
        index += 1;
      } else {
        quoted = !quoted;
      }
    } else if (character === ',' && !quoted) {
      fields.push(current);
      current = '';
    } else {
      current += character;
    }
  }
  fields.push(current);
  return fields;
};

export const parseCsv = (source) => {
  const lines = source.trim().split(/\r?\n/).filter(Boolean);
  if (lines.length === 0) return [];
  const headers = parseCsvLine(lines[0]);
  return lines.slice(1).map((line, index) => {
    const values = parseCsvLine(line);
    return Object.fromEntries(headers.map((header, column) => [
      header,
      values[column] ?? '',
    ]).concat([['rowNumber', index + 2]]));
  });
};

export const validateMatrix = ({
  manifests,
  rows,
  fileExists,
}) => {
  const errors = [];
  const routeCounts = new Map();

  for (const row of rows) {
    const label = `functional-matrix.csv:${row.rowNumber ?? '?'}`;
    if (!Object.hasOwn(manifests, row.app)) {
      errors.push(`${label}: unknown app ${row.app}`);
      continue;
    }
    if (!String(row.feature ?? '').trim()) {
      errors.push(`${label}: feature is required`);
    }
    if (row.implementation !== 'functional') {
      errors.push(`${label}: implementation=${row.implementation || 'missing'} is not functional`);
    }
    for (const column of capabilityColumns) {
      if (!isAcceptedState(row[column])) {
        errors.push(`${label}: ${column}=${row[column] || 'missing'} is not accepted`);
      }
    }

    const routes = splitList(row.routes);
    if (routes.length === 0) errors.push(`${label}: routes is required`);
    for (const route of routes) {
      const key = `${row.app}:${route}`;
      routeCounts.set(key, (routeCounts.get(key) ?? 0) + 1);
    }

    const references = [
      ...splitList(row.legacy_sources),
      ...splitList(row.react_module),
      ...splitList(row.component_tests === 'passed' ? row.evidence : ''),
    ].filter((value) => value !== '-');
    for (const reference of references) {
      if (genericMigrationPages.has(reference)) {
        errors.push(`${label}: ${reference} is a generic migration page`);
      }
      if (!fileExists(reference)) {
        errors.push(`${label}: referenced file does not exist: ${reference}`);
      }
    }
  }

  for (const [app, routes] of Object.entries(manifests)) {
    for (const route of routes) {
      const key = `${app}:${route.path}`;
      const count = routeCounts.get(key) ?? 0;
      if (count === 0) errors.push(`${key} is missing from functional matrix`);
      if (count > 1) errors.push(`${key} appears ${count} times in functional matrix`);
    }
  }

  for (const key of routeCounts.keys()) {
    const [app, ...routeParts] = key.split(':');
    const route = routeParts.join(':');
    if (!manifests[app]?.some((entry) => entry.path === route)) {
      errors.push(`${key} is not present in the application manifest`);
    }
  }

  return errors;
};

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');

const run = () => {
  const manifests = Object.fromEntries(
    ['dashboard', 'sidebar', 'operation'].map((app) => [
      app,
      JSON.parse(readFileSync(
        resolve(repositoryRoot, `web/apps/${app}/src/migration-routes.json`),
        'utf8',
      )),
    ]),
  );
  const matrixPath = resolve(
    repositoryRoot,
    'docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv',
  );
  if (!existsSync(matrixPath)) {
    console.error('phase2.1-frontend: functional matrix is missing');
    process.exitCode = 1;
    return;
  }
  const rows = parseCsv(readFileSync(matrixPath, 'utf8'));
  const errors = validateMatrix({
    manifests,
    rows,
    fileExists: (path) => existsSync(resolve(repositoryRoot, path)),
  });
  if (errors.length > 0) {
    console.error(`phase2.1-frontend: ${errors.length} gap(s)`);
    for (const error of errors) console.error(`- ${error}`);
    process.exitCode = 1;
    return;
  }
  const routeCount = Object.values(manifests)
    .reduce((sum, routes) => sum + routes.length, 0);
  console.log(`phase2.1-frontend: ok (${routeCount} routes, ${rows.length} features)`);
};

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  run();
}
