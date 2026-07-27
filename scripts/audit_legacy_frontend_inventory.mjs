import { createHash } from 'node:crypto';
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const specifications = {
  'pages.csv': {
    columns: ['app', 'source_file', 'route', 'status', 'owner', 'risk', 'batch'],
    key: (row) => `${row.app}:${row.route}`,
    sourceColumns: ['source_file'],
  },
  'routes.csv': {
    columns: ['app', 'path', 'name', 'source_file', 'auth', 'corp_context', 'permission', 'render_target'],
    key: (row) => `${row.app}:${row.path}`,
    sourceColumns: ['source_file'],
  },
  'apis.csv': {
    columns: ['app', 'method', 'path', 'source_file', 'request_fields', 'response_fields', 'auth', 'corp_scope', 'go_evidence'],
    key: (row) => `${row.app}:${row.method}:${row.path}`,
    sourceColumns: ['source_file'],
  },
  'permissions.csv': {
    columns: ['app', 'route', 'menu_link_url', 'actions', 'source_file'],
    key: (row) => `${row.app}:${row.route}:${row.menu_link_url}:${row.actions}`,
    sourceColumns: ['source_file'],
  },
  'assets.csv': {
    columns: ['app', 'source_file', 'kind', 'license_status', 'used_by'],
    key: (row) => `${row.app}:${row.source_file}`,
    sourceColumns: ['source_file', 'used_by'],
  },
  'dependencies.csv': {
    columns: ['app', 'package', 'legacy_range', 'replacement', 'decision', 'risk'],
    key: (row) => `${row.app}:${row.package}`,
    sourceColumns: [],
  },
};

function parseCsv(content) {
  const rows = [];
  let row = [];
  let field = '';
  let quoted = false;
  for (let index = 0; index < content.length; index += 1) {
    const character = content[index];
    if (quoted) {
      if (character === '"' && content[index + 1] === '"') {
        field += '"';
        index += 1;
      } else if (character === '"') {
        quoted = false;
      } else {
        field += character;
      }
    } else if (character === '"') {
      quoted = true;
    } else if (character === ',') {
      row.push(field);
      field = '';
    } else if (character === '\n') {
      row.push(field.replace(/\r$/, ''));
      rows.push(row);
      row = [];
      field = '';
    } else {
      field += character;
    }
  }
  if (field.length > 0 || row.length > 0) rows.push([...row, field.replace(/\r$/, '')]);
  return rows.filter((values) => values.some((value) => value.trim() !== ''));
}

function filePath(root, relativePath) {
  return resolve(root, relativePath.replaceAll('/', '/'));
}

function splitReferences(value) {
  return value.split(';').map((item) => item.trim()).filter(Boolean).filter((item) => item !== '-');
}

function audit(root) {
  const errors = [];
  const data = {};
  const error = (file, row, message) => errors.push(`frontend-audit: ${file}:${row}: ${message}`);

  for (const [file, specification] of Object.entries(specifications)) {
    const relativePath = `docs/handle/frontend-audit/${file}`;
    if (!existsSync(filePath(root, relativePath))) {
      error(file, 1, 'file is required');
      data[file] = [];
      continue;
    }
    const rows = parseCsv(readFileSync(filePath(root, relativePath), 'utf8'));
    const header = rows.shift() ?? [];
    if (header.join(',') !== specification.columns.join(',')) {
      error(file, 1, `columns must be ${specification.columns.join(',')}`);
      data[file] = [];
      continue;
    }
    const seen = new Set();
    data[file] = rows.map((values, index) => {
      const rowNumber = index + 2;
      const row = Object.fromEntries(specification.columns.map((column, position) => [column, (values[position] ?? '').trim()]));
      if (values.length !== specification.columns.length) error(file, rowNumber, `expected ${specification.columns.length} columns`);
      for (const column of specification.columns) {
        if (!row[column]) error(file, rowNumber, `${column} is required`);
      }
      const key = specification.key(row);
      if (seen.has(key)) error(file, rowNumber, `duplicate key ${key}`);
      seen.add(key);
      for (const column of specification.sourceColumns) {
        for (const source of splitReferences(row[column])) {
          if (!existsSync(filePath(root, source))) error(file, rowNumber, `${column} does not exist: ${source}`);
        }
      }
      return { ...row, rowNumber };
    });
  }

  for (const page of data['pages.csv'] ?? []) {
    if (!['legacy', 'candidate', 'blocked'].includes(page.status)) error('pages.csv', page.rowNumber, 'status must be legacy, candidate, or blocked');
  }
  const pages = new Set((data['pages.csv'] ?? []).map((page) => `${page.app}:${page.route}`));
  for (const route of data['routes.csv'] ?? []) {
    if (!pages.has(`${route.app}:${route.path}`)) error('routes.csv', route.rowNumber, `route "${route.path}" has no matching page`);
  }
  const routes = new Set((data['routes.csv'] ?? []).map((route) => `${route.app}:${route.path}`));
  for (const permission of data['permissions.csv'] ?? []) {
    if (!routes.has(`${permission.app}:${permission.route}`)) error('permissions.csv', permission.rowNumber, `route "${permission.route}" does not exist`);
  }
  const manifest = filePath(root, 'web/legacy/SOURCE_MANIFEST.sha256');
  if (!existsSync(manifest)) {
    error('SOURCE_MANIFEST.sha256', 1, 'file is required');
  } else {
    const lines = readFileSync(manifest, 'utf8').split(/\r?\n/).filter(Boolean);
    lines.forEach((line, index) => {
      const match = /^([a-fA-F0-9]{64})  (.+)$/.exec(line);
      if (!match) {
        error('SOURCE_MANIFEST.sha256', index + 1, 'expected SHA256 and source file path');
        return;
      }
      const [, expectedHash, source] = match;
      const sourcePath = filePath(root, source);
      if (!existsSync(sourcePath)) {
        error('SOURCE_MANIFEST.sha256', index + 1, `source file does not exist: ${source}`);
        return;
      }
      const actualHash = createHash('sha256').update(readFileSync(sourcePath)).digest('hex');
      if (actualHash !== expectedHash.toLowerCase()) error('SOURCE_MANIFEST.sha256', index + 1, `SHA256 does not match: ${source}`);
    });
  }
  return { errors, data };
}

const argumentsList = process.argv.slice(2);
if (!argumentsList.includes('--check')) {
  console.error('frontend-audit: usage: node scripts/audit_legacy_frontend_inventory.mjs --check [--root <path>]');
  process.exitCode = 1;
} else {
  const rootIndex = argumentsList.indexOf('--root');
  const root = rootIndex === -1 ? process.cwd() : resolve(argumentsList[rootIndex + 1] ?? '');
  const { errors, data } = audit(root);
  if (errors.length > 0) {
    console.error(errors.join('\n'));
    process.exitCode = 1;
  } else {
    console.log(`frontend-audit: ok (${data['pages.csv'].length} pages, ${data['routes.csv'].length} routes, ${data['apis.csv'].length} apis)`);
  }
}
