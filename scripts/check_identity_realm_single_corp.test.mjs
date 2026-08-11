import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { runIdentitySingleCorpGate } from './check_identity_realm_single_corp.mjs';

async function makeFixtureTree() {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'identity-single-corp-'));
  await Promise.all([
    fs.mkdir(path.join(root, 'internal', 'saasauth'), { recursive: true }),
    fs.mkdir(path.join(root, 'internal', 'dashboardauth'), { recursive: true }),
    fs.mkdir(path.join(root, 'internal', 'dashboard'), { recursive: true }),
    fs.mkdir(path.join(root, 'web', 'apps', 'saas-admin', 'src'), { recursive: true }),
    fs.mkdir(path.join(root, 'web', 'apps', 'dashboard', 'src'), { recursive: true }),
  ]);
  await Promise.all([
    fs.writeFile(path.join(root, 'internal', 'saasauth', 'store.go'), 'package saasauth\n\nfunc Authenticate() { _ = "mochat_go_saas_admin_users" }\n'),
    fs.writeFile(path.join(root, 'internal', 'dashboardauth', 'store.go'), 'package dashboardauth\n\nfunc Authenticate() { _ = "mochat_go_dashboard_identities" }\n'),
    fs.writeFile(path.join(root, 'internal', 'dashboard', 'handler.go'), 'package dashboard\n\nfunc Handler() { _ = DashboardPrincipalFromContext() }\n'),
    fs.writeFile(path.join(root, 'web', 'apps', 'saas-admin', 'src', 'main.tsx'), 'export { login } from "./auth";\n'),
    fs.writeFile(path.join(root, 'web', 'apps', 'dashboard', 'src', 'main.tsx'), 'export { dashboard } from "./dashboard";\n'),
    fs.writeFile(path.join(root, 'web', 'apps', 'saas-admin', 'src', 'auth.ts'), 'export const realm = "saas_admin";\nexport const jwt = "saas-jwt";\n'),
    fs.writeFile(path.join(root, 'web', 'apps', 'dashboard', 'src', 'dashboard.ts'), 'export const realm = "dashboard";\nexport const jwt = "dashboard-jwt";\n'),
  ]);
  return root;
}

async function withFixture(mutator, callback) {
  const root = await makeFixtureTree();
  try {
    await mutator(root);
    return await callback(root);
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
}

test('identity gate accepts a minimal separated realm and principal tree', async () => {
  await withFixture(async () => {}, async (root) => {
    const result = runIdentitySingleCorpGate(root);
    assert.ok(result.jwtRealms.saas_admin.length >= 1);
    assert.ok(result.jwtRealms.dashboard.length >= 1);
    assert.equal(result.forbiddenCorpRoutes.length, 0);
    assert.equal(result.forbiddenSessionCorpFields.length, 0);
    assert.equal(result.plaintextSecretReads.length, 0);
  });
});

test('RED: SaaS identity store cannot read the shared mc_user table', async () => {
  await withFixture(async (root) => {
    await fs.appendFile(path.join(root, 'internal', 'saasauth', 'store.go'), '\nfunc legacy() { _ = "mc_user" }\n');
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /mc_user.*saas|SaaS.*mc_user/i);
  });
});

test('RED: SaaS and Dashboard production JWT configuration must be different realms', async () => {
  await withFixture(async (root) => {
    await fs.writeFile(path.join(root, 'internal', 'saasauth', 'jwt.go'), 'package saasauth\nvar Config = "MOCHAT_SIMPLE_JWT_SECRET"\n');
    await fs.writeFile(path.join(root, 'internal', 'dashboardauth', 'jwt.go'), 'package dashboardauth\nvar Config = "MOCHAT_SIMPLE_JWT_SECRET"\n');
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /shared JWT|JWT.*realm|MOCHAT_SIMPLE_JWT_SECRET/i);
  });
});

test('RED: Dashboard session and corp selection routes are forbidden in production', async () => {
  await withFixture(async (root) => {
    await fs.appendFile(path.join(root, 'internal', 'dashboard', 'handler.go'), '\nfunc legacy() { _ = "corpId" }\n');
    await fs.writeFile(path.join(root, 'internal', 'dashboard', 'routes.go'), 'package dashboard\nvar routes = []string{"GET /dashboard/corp/select", "POST /dashboard/corp/bind", "POST /dashboard/corp/store"}\n');
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /corpId|legacy corp route/i);
  });
});

test('RED: Dashboard handlers must consume DashboardPrincipal from their request context', async () => {
  await withFixture(async (root) => {
    await fs.writeFile(path.join(root, 'internal', 'dashboard', 'unbound.go'), 'package dashboard\nfunc UnboundHandler() { println("business") }\n');
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /principal.*consumer|DashboardPrincipal/i);
  });
});

test('RED: production UI cannot expose new company or selector controls', async () => {
  await withFixture(async (root) => {
    await fs.writeFile(path.join(root, 'web', 'apps', 'dashboard', 'src', 'company.tsx'), 'export function Company() { return <button>新建企业</button>; }\n');
    await fs.appendFile(path.join(root, 'web', 'apps', 'dashboard', 'src', 'dashboard.ts'), '\nexport { Company } from "./company";\n');
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /新建企业|company selector/i);
  });
});

test('RED: production code cannot read plaintext credentials', async () => {
  await withFixture(async (root) => {
    await fs.writeFile(path.join(root, 'internal', 'dashboard', 'secret.go'), 'package dashboard\nfunc Secret() { _ = credential.EmployeeSecret }\n');
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /plaintext secret|EmployeeSecret|secret read/i);
  });
});

test('RED: comments, tests, fixtures, and JSON are not production evidence', async () => {
  await withFixture(async (root) => {
    await fs.mkdir(path.join(root, 'internal', 'dashboard', 'fixtures'), { recursive: true });
    await fs.writeFile(path.join(root, 'internal', 'dashboard', 'comment.go'), '// GET /dashboard/corp/select mc_user corpId\npackage dashboard\n');
    await fs.writeFile(path.join(root, 'internal', 'dashboard', 'ignored_test.go'), 'package dashboard\nfunc TestIgnored() { _ = "mc_user" }\n');
    await fs.writeFile(path.join(root, 'internal', 'dashboard', 'fixtures', 'fixture.go'), 'package fixtures\nfunc Fixture() { _ = "mc_user" }\n');
    await fs.writeFile(path.join(root, 'web', 'apps', 'dashboard', 'src', 'fixture.json'), '{"text":"新建企业 corpId"}\n');
  }, async (root) => {
    assert.doesNotThrow(() => runIdentitySingleCorpGate(root));
  });
});
