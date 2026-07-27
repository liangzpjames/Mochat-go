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
    .filter((file) => !file.endsWith('/SOURCE_MANIFEST.sha256'))
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
  write(root, view, '<template><main>example</main></template>\n');
  write(root, api, "export const example = () => request({ url: '/api/example', method: 'get' });\n");
  write(root, asset, '<svg/>\n');
  write(root, 'web/legacy/dashboard/package.json', '{"dependencies":{"vue":"^2.6.10"}}\n');
  const auditDirectory = 'docs/handle/frontend-audit';
  write(root, `${auditDirectory}/pages.csv`, `${columns['pages.csv']}\ndashboard,${view},-,legacy,frontend,low,1\ndashboard,${router},/example,legacy,frontend,low,1\n`);
  write(root, `${auditDirectory}/routes.csv`, `${columns['routes.csv']}\ndashboard,/example,example,${router},required,required,*,spa\n`);
  write(root, `${auditDirectory}/apis.csv`, `${columns['apis.csv']}\ndashboard,GET,/api/example,${api},id,id,required,corp,internal/server/example.go\n`);
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
