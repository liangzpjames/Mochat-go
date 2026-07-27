import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const auditScript = join(repositoryRoot, 'scripts', 'audit_legacy_frontend_inventory.mjs');
const columns = {
  'pages.csv': 'app,source_file,route,status,owner,risk,batch',
  'routes.csv': 'app,path,name,source_file,auth,corp_context,permission,render_target',
  'apis.csv': 'app,method,path,source_file,request_fields,response_fields,auth,corp_scope,go_evidence',
  'permissions.csv': 'app,route,menu_link_url,actions,source_file',
  'assets.csv': 'app,source_file,kind,license_status,used_by',
  'dependencies.csv': 'app,package,legacy_range,replacement,decision,risk',
};

function write(root, relativePath, content = '') {
  const file = join(root, relativePath);
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, content);
}

function createFixture() {
  const root = mkdtempSync(join(tmpdir(), 'mochat-frontend-audit-'));
  const router = 'web/legacy/dashboard/src/router/asyncRouter.js';
  const api = 'web/legacy/dashboard/src/api/example.js';
  const asset = 'web/legacy/dashboard/src/assets/example.svg';
  write(root, router, 'export default {};\n');
  write(root, api, 'export default {};\n');
  write(root, asset, '<svg/>\n');
  const auditDirectory = 'docs/handle/frontend-audit';
  write(root, `${auditDirectory}/pages.csv`, `${columns['pages.csv']}\ndashboard,${router},/example,legacy,frontend,low,1\n`);
  write(root, `${auditDirectory}/routes.csv`, `${columns['routes.csv']}\ndashboard,/example,example,${router},required,required,*,spa\n`);
  write(root, `${auditDirectory}/apis.csv`, `${columns['apis.csv']}\ndashboard,GET,/api/example,${api},id,id,required,corp,internal/server/example.go\n`);
  write(root, `${auditDirectory}/permissions.csv`, `${columns['permissions.csv']}\ndashboard,/example,/example,view,${router}\n`);
  write(root, `${auditDirectory}/assets.csv`, `${columns['assets.csv']}\ndashboard,${asset},svg,verified,${router}\n`);
  write(root, `${auditDirectory}/dependencies.csv`, `${columns['dependencies.csv']}\ndashboard,vue,^2,react,replace,medium\n`);
  const hash = createHash('sha256').update('export default {};\n').digest('hex');
  write(root, 'web/legacy/SOURCE_MANIFEST.sha256', `${hash}  ${router}\n`);
  return root;
}

function runAudit(root) {
  try {
    execFileSync(process.execPath, [auditScript, '--check', '--root', root], { encoding: 'utf8', stdio: 'pipe' });
    return { status: 0, output: '' };
  } catch (error) {
    return { status: error.status, output: `${error.stdout ?? ''}${error.stderr ?? ''}` };
  }
}

function replace(root, relativePath, from, to) {
  const fullPath = join(root, relativePath);
  writeFileSync(fullPath, readFileSync(fullPath, 'utf8').replace(from, to));
}

function expectAuditFailure(mutate, expected) {
  const root = createFixture();
  try {
    mutate(root);
    const result = runAudit(root);
    assert.notEqual(result.status, 0);
    assert.match(result.output, expected);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

test('fails when a route has no matching page', () => {
  expectAuditFailure(
    (root) => replace(root, 'docs/handle/frontend-audit/routes.csv', '/example,example', '/orphan,orphan'),
    /frontend-audit: routes\.csv:2: route "\/orphan" has no matching page/,
  );
});

test('fails when an API method or path is missing', () => {
  expectAuditFailure(
    (root) => replace(root, 'docs/handle/frontend-audit/apis.csv', 'GET,/api/example', ',/api/example\ndashboard,GET,,web/legacy/dashboard/src/api/example.js,id,id,required,corp,internal/server/example.go'),
    /frontend-audit: apis\.csv:[23]: (method|path) is required/,
  );
});

test('fails when a dependency decision is missing', () => {
  expectAuditFailure(
    (root) => replace(root, 'docs/handle/frontend-audit/dependencies.csv', ',replace,medium', ',,medium'),
    /frontend-audit: dependencies\.csv:2: decision is required/,
  );
});

test('fails when an asset license status is missing', () => {
  expectAuditFailure(
    (root) => replace(root, 'docs/handle/frontend-audit/assets.csv', ',verified,', ',,'),
    /frontend-audit: assets\.csv:2: license_status is required/,
  );
});

test('fails when the source manifest names a missing source file', () => {
  expectAuditFailure(
    (root) => replace(root, 'web/legacy/SOURCE_MANIFEST.sha256', 'asyncRouter.js', 'missing.js'),
    /frontend-audit: SOURCE_MANIFEST\.sha256:1: source file does not exist: web\/legacy\/dashboard\/src\/router\/missing\.js/,
  );
});
