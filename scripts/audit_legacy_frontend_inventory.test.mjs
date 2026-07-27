import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs';
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

function legacyFiles(root, directory = 'web/legacy') {
  const absoluteDirectory = join(root, directory);
  return readdirSync(absoluteDirectory).flatMap((entry) => {
    const relativePath = `${directory}/${entry}`;
    return statSync(join(root, relativePath)).isDirectory() ? legacyFiles(root, relativePath) : [relativePath];
  });
}

function writeManifest(root) {
  const entries = legacyFiles(root)
    .filter((file) => /^web\/legacy\/(dashboard|sidebar|operation)\//.test(file))
    .sort()
    .map((file) => `${createHash('sha256').update(readFileSync(join(root, file))).digest('hex')}  ${file}`);
  write(root, 'web/legacy/SOURCE_MANIFEST.sha256', `${entries.join('\n')}\n`);
}

function createFixture() {
  const root = mkdtempSync(join(tmpdir(), 'mochat-frontend-audit-'));
  const router = 'web/legacy/dashboard/src/router/asyncRouter.js';
  const view = 'web/legacy/dashboard/src/views/example/index.vue';
  const api = 'web/legacy/dashboard/src/api/example.js';
  const asset = 'web/legacy/dashboard/src/assets/example.svg';
  write(root, router, "export const routes = [{ path: '/example', name: 'example', component: () => import('@/views/example/index') }];\n");
  write(root, view, `<template><main v-permission="'/example@edit'"><img src="@/assets/example.svg"></main></template>
<script>
import { example } from '@/api/example'
export default { created () { example({ id: 1 }) } }
</script>
`);
  write(root, api, "export function example (params) { return request({ url: '/api/example', method: 'get', params }) }\n");
  write(root, asset, '<svg/>\n');
  write(root, 'web/legacy/dashboard/package.json', '{"dependencies":{"vue":"^2.6.10"}}\n');
  write(root, 'web/legacy/README.md', '# Legacy frontend reference sources\n\n- Source commit: `3dcd216c188df34f2c3ed489b8e8b9473e635488`\n');
  const auditDirectory = 'docs/handle/frontend-audit';
  write(root, `${auditDirectory}/pages.csv`, `${columns['pages.csv']}\ndashboard,${view},-,legacy,frontend,low,1\ndashboard,${router},/example,legacy,frontend,low,1\n`);
  write(root, `${auditDirectory}/routes.csv`, `${columns['routes.csv']}\ndashboard,/example,example,${router},required,required,*,spa\n`);
  write(root, `${auditDirectory}/apis.csv`, `${columns['apis.csv']}\ndashboard,GET,/api/example,${api},params:id,id,required,corp,-\n`);
  write(root, `${auditDirectory}/permissions.csv`, `${columns['permissions.csv']}\ndashboard,/example,/example,view,${router}\n`);
  write(root, `${auditDirectory}/assets.csv`, `${columns['assets.csv']}\ndashboard,${asset},svg,verified,${view}\n`);
  write(root, `${auditDirectory}/dependencies.csv`, `${columns['dependencies.csv']}\ndashboard,vue,^2.6.10,react,replace,medium\n`);
  writeManifest(root);
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

function runRefresh(root) {
  return execFileSync(process.execPath, [auditScript, '--refresh', '--check', '--root', root], { encoding: 'utf8' });
}

function replace(root, relativePath, from, to) {
  const fullPath = join(root, relativePath);
  writeFileSync(fullPath, readFileSync(fullPath, 'utf8').replace(from, to));
}

function csvRows(root, relativePath) {
  return readFileSync(join(root, relativePath), 'utf8').trim().split(/\r?\n/).slice(1).map((line) => line.split(','));
}

function csvObjects(root, relativePath) {
  const [header, ...rows] = readFileSync(join(root, relativePath), 'utf8').trim().split(/\r?\n/).map((line) => line.split(','));
  return rows.map((row) => Object.fromEntries(header.map((column, index) => [column, row[index]])));
}

function setCsvCell(root, relativePath, keyColumn, keyValue, column, value) {
  const file = join(root, relativePath);
  const lines = readFileSync(file, 'utf8').trim().split(/\r?\n/).map((line) => line.split(','));
  const keyIndex = lines[0].indexOf(keyColumn);
  const columnIndex = lines[0].indexOf(column);
  const row = lines.slice(1).find((values) => values[keyIndex] === keyValue);
  assert.ok(row, `missing ${keyColumn}=${keyValue} in ${relativePath}`);
  row[columnIndex] = value;
  writeFileSync(file, `${lines.map((values) => values.join(',')).join('\n')}\n`);
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
    /frontend-audit: SOURCE_MANIFEST\.sha256:\d+: source file does not exist: web\/legacy\/dashboard\/src\/router\/missing\.js/,
  );
});

test('fails when a router-declared route is omitted from routes.csv', () => {
  expectAuditFailure(
    (root) => {
      write(root, 'web/legacy/dashboard/src/router/asyncRouter.js', "export const routes = [{ path: '/example', name: 'example', component: () => import('@/views/example/index') }, { path: '/missing-route', name: 'missingRoute', component: () => import('@/views/example/index') }];\n");
      writeManifest(root);
    },
    /frontend-audit: routes\.csv:1: discovered route is missing: dashboard:\/missing-route/,
  );
});

test('fails when a declared API is omitted from apis.csv', () => {
  expectAuditFailure(
    (root) => {
      write(root, 'web/legacy/dashboard/src/api/example.js', "export const example = () => request({ url: '/api/example', method: 'get' });\nexport const missing = () => request({ url: '/api/missing', method: 'post' });\n");
      writeManifest(root);
    },
    /frontend-audit: apis\.csv:1: discovered API is missing: dashboard:POST:\/api\/missing/,
  );
});

test('fails when a legacy runtime dependency is omitted from dependencies.csv', () => {
  expectAuditFailure(
    (root) => {
      write(root, 'web/legacy/dashboard/package.json', '{"dependencies":{"vue":"^2.6.10","axios":"^0.21.0"}}\n');
      writeManifest(root);
    },
    /frontend-audit: dependencies\.csv:1: discovered dependency is missing: dashboard:axios/,
  );
});

test('fails when a legacy view is omitted from pages.csv', () => {
  expectAuditFailure(
    (root) => {
      write(root, 'web/legacy/dashboard/src/views/missing/index.vue', '<template><main>missing</main></template>\n');
      writeManifest(root);
    },
    /frontend-audit: pages\.csv:1: discovered page is missing: dashboard:web\/legacy\/dashboard\/src\/views\/missing\/index\.vue/,
  );
});

test('fails when a legacy asset is omitted from assets.csv', () => {
  expectAuditFailure(
    (root) => {
      write(root, 'web/legacy/dashboard/src/assets/missing.png', 'PNG');
      writeManifest(root);
    },
    /frontend-audit: assets\.csv:1: discovered asset is missing: dashboard:web\/legacy\/dashboard\/src\/assets\/missing\.png/,
  );
});

test('fails when a legacy source is omitted from the manifest', () => {
  expectAuditFailure(
    (root) => replace(root, 'web/legacy/SOURCE_MANIFEST.sha256', /.*example\.svg\r?\n/, ''),
    /frontend-audit: SOURCE_MANIFEST\.sha256:1: source file is missing from manifest: web\/legacy\/dashboard\/src\/assets\/example\.svg/,
  );
});

test('fails when the manifest repeats a source file', () => {
  expectAuditFailure(
    (root) => {
      const manifest = 'web/legacy/SOURCE_MANIFEST.sha256';
      write(root, manifest, `${readFileSync(join(root, manifest), 'utf8').trim()}\n${readFileSync(join(root, manifest), 'utf8').split(/\r?\n/)[0]}\n`);
    },
    /frontend-audit: SOURCE_MANIFEST\.sha256:\d+: duplicate source file: web\/legacy\/dashboard\/package\.json/,
  );
});

test('manifest hashes match a clean archive from the pinned legacy commit', () => {
  const root = mkdtempSync(join(tmpdir(), 'mochat-legacy-archive-'));
  try {
    const archive = join(root, 'legacy.tar');
    writeFileSync(archive, execFileSync('git', ['archive', '--format=tar', '3dcd216c188df34f2c3ed489b8e8b9473e635488', 'dashboard', 'sidebar', 'operation'], { cwd: repositoryRoot, maxBuffer: 32 * 1024 * 1024 }));
    const extracted = join(root, 'extracted');
    mkdirSync(extracted);
    execFileSync('tar', ['-xf', archive, '-C', extracted]);
    const manifest = readFileSync(join(repositoryRoot, 'web/legacy/SOURCE_MANIFEST.sha256'), 'utf8').trim().split(/\r?\n/);
    assert.equal(manifest.length, legacyFiles(extracted, '.').length);
    for (const line of manifest) {
      const [, hash, source] = /^(\w{64})  (.+)$/.exec(line);
      const archivePath = source.replace(/^web\/legacy\//, '');
      assert.equal(createHash('sha256').update(readFileSync(join(extracted, archivePath))).digest('hex'), hash);
    }
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test('committed HEAD archive preserves the four canonical source blobs and modes', () => {
  const paths = [
    'dashboard/src/router/asyncRouter.js',
    'dashboard/src/views/chatTool/enhance.vue',
    'operation/src/router/index.js',
    'sidebar/src/router/routes.js',
  ];
  const pinned = execFileSync('git', ['ls-tree', '3dcd216c188df34f2c3ed489b8e8b9473e635488', '--', ...paths], { cwd: repositoryRoot, encoding: 'utf8' });
  const head = execFileSync('git', ['ls-tree', 'HEAD', '--', ...paths.map((path) => `web/legacy/${path}`)], { cwd: repositoryRoot, encoding: 'utf8' });
  const pinnedEntries = new Map(pinned.trim().split(/\r?\n/).map((line) => { const [metadata, path] = line.split('\t'); const [mode, , blob] = metadata.split(' '); return [path, `${mode}:${blob}`]; }));
  const headEntries = new Map(head.trim().split(/\r?\n/).map((line) => { const [metadata, path] = line.split('\t'); const [mode, , blob] = metadata.split(' '); return [path.replace(/^web\/legacy\//, ''), `${mode}:${blob}`]; }));
  assert.deepEqual(headEntries, pinnedEntries);
});

test('refresh derives transport and actual request properties from each exported function', () => {
  const root = createFixture();
  try {
    write(root, 'web/legacy/dashboard/src/api/example.js', `
export function example (params) {
  return request({ url: '/api/example', method: 'get', params })
}
export function store (params) {
  const payload = { corpId: params.corpId, displayName: params.name }
  return request({ url: '/corp/store', method: 'post', data: payload })
}
export function destructured ({ token, redirect }) {
  return request({ url: '/api/destructured', method: 'put', data: { token, redirect } })
}
export function filtered ({ unused }) {
  return request({ url: '/api/filtered', method: 'post', data: { actual: true } })
}
export function blocked (params) {
  return request({ url: '/api/blocked', method: 'delete', data: params })
}
`);
    write(root, 'web/legacy/dashboard/src/views/example/index.vue', `<template><main></main></template>
<script>
import { example, store } from '@/api/example'
export default {
  created () {
    example({ id: 1, search: '' })
    store({ corpId: 7, name: 'Acme' })
  }
}
</script>
`);
    writeManifest(root);

    runRefresh(root);
    const apis = csvObjects(root, 'docs/handle/frontend-audit/apis.csv');
    const byPath = new Map(apis.map((api) => [api.path, api]));
    assert.equal(byPath.get('/api/example').request_fields, 'params:id;search');
    assert.equal(byPath.get('/corp/store').request_fields, 'data:corpId;displayName');
    assert.equal(byPath.get('/api/destructured').request_fields, 'data:redirect;token');
    assert.equal(byPath.get('/api/filtered').request_fields, 'data:actual');
    assert.equal(byPath.get('/api/blocked').request_fields, 'data:blocked[no-callsite]');
    assert.ok(apis.every((api) => !/^(?:data|params):params$|^none$/.test(api.request_fields)));
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test('fails when request_fields are replaced with a generic payload expression', () => {
  const root = createFixture();
  try {
    runRefresh(root);
    setCsvCell(root, 'docs/handle/frontend-audit/apis.csv', 'path', '/api/example', 'request_fields', 'params:params');
    const result = runAudit(root);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /frontend-audit: apis\.csv:2: request_fields must match discovered evidence params:id/);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test('refresh limits route metadata to declaration object boundaries and derives route auth', () => {
  const root = createFixture();
  try {
    write(root, 'web/legacy/dashboard/src/router/base/index.js', `
export const baseRouterMap = [
  { path: '/login', name: 'login', component: () => import('@/views/login/login') }
]
`);
    write(root, 'web/legacy/dashboard/src/router/navigationGuards.js', `
const whiteList = ['login']
router.beforeEach((to, from, next) => {
  if (storage.get('ACCESS_TOKEN') && to.path !== '/login') next()
  else if (whiteList.includes(to.name)) next()
  else next({ path: '/login' })
})
`);
    write(root, 'web/legacy/dashboard/src/views/login/login.vue', '<template><main>login</main></template>\n');

    write(root, 'web/legacy/sidebar/src/router/index.js', `
router.beforeEach((to, from, next) => {
  if (checkLogin(to, from, next) === false) next({ path: '/login' })
  else next()
})
`);
    write(root, 'web/legacy/sidebar/src/router/routes.js', `
const routes = [
  { path: '/', name: 'index', component: () => import('views/index') },
  { path: '/login', name: 'login', component: () => import('views/login') },
  { path: '/codeAuth', name: 'codeAuth', component: () => import('views/codeAuth') },
  { path: '/auth', name: 'auth', component: () => import('views/auth') },
  { path: '/contact', name: 'contact', component: () => import('views/contact') }
]
export default routes
`);
    write(root, 'web/legacy/sidebar/src/utils/index.js', `
export function checkLogin (to) {
  if (to.fullPath === '/' || to.path === '/codeAuth' || to.path === '/auth' || to.path === '/login') return
  return Boolean(getCookie('token'))
}
`);
    for (const view of ['index', 'login', 'codeAuth', 'auth', 'contact']) {
      write(root, `web/legacy/sidebar/src/views/${view}.vue`, `<template><main>${view}</main></template>\n`);
    }
    write(root, 'web/legacy/sidebar/package.json', '{"dependencies":{}}\n');

    write(root, 'web/legacy/operation/src/router/index.js', `
import root from '../views/root'
import workFission from '../views/workFission'
import speed from '../views/speed'
import lottery from '../views/lottery'
import explain from '../views/explain'
import roomClockIn from '../views/roomClockIn'
const routes = [
  { path: '/', component: root },
  { path: '/workFission', name: 'workFissionIndex', component: workFission },
  { path: '/speed', component: speed },
  { path: '/lottery', name: 'lotteryIndex', component: lottery },
  { path: '/explain', component: explain },
  { path: '/roomClockIn', name: '/roomClockIn', component: roomClockIn }
]
`);
    for (const view of ['root', 'workFission', 'speed', 'lottery', 'explain', 'roomClockIn']) {
      write(root, `web/legacy/operation/src/views/${view}.vue`, `<template><main>${view}</main></template>\n`);
    }
    write(root, 'web/legacy/operation/package.json', '{"dependencies":{}}\n');
    writeManifest(root);

    runRefresh(root);
    const routes = csvObjects(root, 'docs/handle/frontend-audit/routes.csv');
    const route = (app, path) => routes.find((item) => item.app === app && item.path === path);
    assert.equal(route('operation', '/').name, '-');
    assert.equal(route('operation', '/speed').name, '-');
    assert.equal(route('operation', '/explain').name, '-');
    assert.equal(route('dashboard', '/login').auth, 'public');
    assert.equal(route('dashboard', '/example').auth, 'ACCESS_TOKEN');
    assert.equal(route('sidebar', '/').auth, 'public');
    assert.equal(route('sidebar', '/login').auth, 'public');
    assert.equal(route('sidebar', '/auth').auth, 'public');
    assert.equal(route('sidebar', '/codeAuth').auth, 'public');
    assert.equal(route('sidebar', '/contact').auth, 'Bearer cookie token');
    assert.match(route('sidebar', '/login').source_file, /src\/router\/routes\.js$/);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test('fails when Go evidence combines a route with an unrelated HTTP method', () => {
  const root = createFixture();
  try {
    runRefresh(root);
    write(root, 'internal/server/example.go', `package server
import "net/http"
var routes = []struct{ method, path string }{
  {method: http.MethodPost, path: "/dashboard/api/example"},
}
var unrelated = http.MethodGet
// "GET /dashboard/api/example" is documentation, not executable evidence.
`);
    setCsvCell(root, 'docs/handle/frontend-audit/apis.csv', 'path', '/api/example', 'go_evidence', 'internal/server/example.go');
    const result = runAudit(root);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /frontend-audit: apis\.csv:2: go_evidence does not prove GET \/dashboard\/api\/example/);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test('accepts Go evidence when method and route share one exact contract construct', () => {
  const root = createFixture();
  try {
    runRefresh(root);
    write(root, 'internal/server/example.go', `package server
var routes = []string{"GET /dashboard/api/example"}
`);
    setCsvCell(root, 'docs/handle/frontend-audit/apis.csv', 'path', '/api/example', 'go_evidence', 'internal/server/example.go');
    const result = runAudit(root);
    assert.equal(result.status, 0, result.output);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test('refresh derives semantic audit fields instead of migration placeholders', () => {
  const root = createFixture();
  try {
    runRefresh(root);
    const audit = 'docs/handle/frontend-audit';
    const [page] = csvRows(root, `${audit}/pages.csv`).filter((row) => row[1].includes('/views/example/'));
    const [route] = csvRows(root, `${audit}/routes.csv`);
    const [api] = csvRows(root, `${audit}/apis.csv`);
    const [permission] = csvRows(root, `${audit}/permissions.csv`);
    const [asset] = csvRows(root, `${audit}/assets.csv`);
    const [dependency] = csvRows(root, `${audit}/dependencies.csv`);
    assert.equal(page[2], '/example');
    assert.equal(route[4], 'ACCESS_TOKEN');
    assert.equal(route[5], 'dashboard-corp-context');
    assert.equal(api[4], 'params:id');
    assert.equal(api[5], 'response.data');
    assert.equal(api[6], 'ACCESS_TOKEN');
    assert.equal(api[7], '/dashboard');
    assert.equal(permission[3], 'edit');
    assert.match(asset[4], /views\/example\/index\.vue/);
    assert.equal(dependency[3], 'react');
    assert.equal(dependency[4], 'replace');
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
