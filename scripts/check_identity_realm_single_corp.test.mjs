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
    fs.mkdir(path.join(root, 'internal', 'server'), { recursive: true }),
    fs.mkdir(path.join(root, 'internal', 'modules'), { recursive: true }),
    fs.mkdir(path.join(root, 'cmd', 'mochat-go'), { recursive: true }),
    fs.mkdir(path.join(root, 'web', 'apps', 'saas-admin', 'src'), { recursive: true }),
    fs.mkdir(path.join(root, 'web', 'apps', 'dashboard', 'src'), { recursive: true }),
  ]);
  await Promise.all([
    fs.writeFile(path.join(root, 'internal', 'saasauth', 'store.go'), 'package saasauth\n\nfunc Authenticate(db DB, login string) { db.QueryRow("SELECT id, password_hash FROM mochat_go_saas_admin_users WHERE login_name = ?", login) }\n'),
    fs.writeFile(path.join(root, 'internal', 'dashboardauth', 'store.go'), 'package dashboardauth\n\nfunc Authenticate(db DB, login string) { db.QueryRow("SELECT user_id, password_hash FROM mochat_go_dashboard_identities WHERE login_identifier = ?", login) }\n'),
    fs.writeFile(path.join(root, 'internal', 'dashboard', 'handler.go'), 'package dashboard\n\ntype Handler struct{}\n\nfunc (h Handler) Index() { _ = DashboardPrincipalFromContext() }\n'),
    fs.writeFile(path.join(root, 'internal', 'server', 'routes.go'), 'package server\n\nfunc register(router Router, handler Handler) { router.Handle("GET", "/dashboard/index", handler.Index) }\n'),
    fs.writeFile(path.join(root, 'internal', 'dashboard', 'dashboard_route_policy.go'), 'package dashboard\nvar exactExemptDashboardRouteContracts = []string{"POST /dashboard/auth"}\nvar publicDashboardRouteContracts = []string{"POST /dashboard/auth"}\nvar denyOnlyDashboardRouteContracts = []string{"DELETE /dashboard/deny"}\nvar pageMappedDashboardRouteContracts = []string{"GET /dashboard/index", "GET /dashboard/server", "GET /dashboard/cmd", "POST /dashboard/module", "POST /dashboard/closure", "GET /dashboard/same-file"}\nvar saasPrincipalDashboardRouteContracts = []string{"GET /dashboard/saasAdmin/settings"}\n'),
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
    assert.match(result.dashboardPrincipalConsumers[0].evidence, /GET \/dashboard\/index/);
    assert.match(result.dashboardPrincipalConsumers[0].handlerSymbol, /handler\.Index/);
    assert.match(result.dashboardPrincipalConsumers[0].handlerSource, /handler\.go:\d+/);
    assert.match(result.dashboardPrincipalConsumers[0].consumerSource, /handler\.go:\d+/);
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
    await fs.writeFile(path.join(root, 'internal', 'dashboard', 'unbound.go'), 'package dashboard\n\ntype UnboundHandler struct{}\nfunc (h UnboundHandler) Unbound() { println("business") }\n');
    await fs.appendFile(path.join(root, 'internal', 'server', 'routes.go'), '\nfunc registerUnbound(router Router, handler UnboundHandler) { router.Handle("POST", "/dashboard/unbound", handler.Unbound) }\n');
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

test('GREEN: a request credential is accepted when encrypted immediately', async () => {
  await withFixture(async (root) => {
    await fs.mkdir(path.join(root, 'internal', 'companyprofile'), { recursive: true });
    await fs.writeFile(path.join(root, 'internal', 'companyprofile', 'service.go'), 'package companyprofile\nfunc Save(input Request) { ciphertext, _ := Encrypt(input.EmployeeSecret); _ = ciphertext }\ntype Request struct { EmployeeSecret string `json:"employeeSecret"` }\n');
  }, async (root) => {
    assert.doesNotThrow(() => runIdentitySingleCorpGate(root));
  });
});

test('GREEN: identity SQL may live in internal/store behind auth interfaces', async () => {
  await withFixture(async (root) => {
    await fs.mkdir(path.join(root, 'internal', 'store'), { recursive: true });
    await fs.writeFile(path.join(root, 'internal', 'saasauth', 'store.go'), 'package saasauth\ntype Store interface { Authenticate(db DB, login string) }\n');
    await fs.writeFile(path.join(root, 'internal', 'dashboardauth', 'store.go'), 'package dashboardauth\ntype Store interface { Authenticate(db DB, login string) }\n');
    await fs.writeFile(path.join(root, 'internal', 'store', 'identity.go'), 'package store\ntype SaaSIdentityStore struct{}\ntype DashboardIdentityStore struct{}\nfunc (s SaaSIdentityStore) Authenticate(db DB, login string) { db.QueryRow("SELECT id, password_hash FROM mochat_go_saas_admin_users WHERE login_name = ?", login) }\nfunc (s DashboardIdentityStore) Authenticate(db DB, login string) { db.QueryRow("SELECT user_id, password_hash FROM mochat_go_dashboard_identities WHERE login_identifier = ?", login) }\n');
  }, async (root) => {
    assert.doesNotThrow(() => runIdentitySingleCorpGate(root));
  });
});

test('GREEN: public auth exact routes do not require DashboardPrincipal', async () => {
  await withFixture(async (root) => {
    await fs.appendFile(path.join(root, 'internal', 'dashboard', 'handler.go'), '\nfunc (h Handler) Auth() { println("login") }\n');
    await fs.appendFile(path.join(root, 'internal', 'server', 'routes.go'), '\nfunc registerAuth(router Router, handler Handler) { router.Handle("POST", "/dashboard/auth", handler.Auth) }\n');
  }, async (root) => {
    const result = runIdentitySingleCorpGate(root);
    assert.ok(result.publicExactRoutes.some((route) => route.route === '/dashboard/auth'));
  });
});

test('GREEN: SaaS realm routes require SaaSPrincipal rather than DashboardPrincipal', async () => {
  await withFixture(async (root) => {
    await fs.writeFile(path.join(root, 'internal', 'dashboard', 'saas_admin.go'), 'package dashboard\ntype SaaSHandler struct{}\nfunc (h SaaSHandler) Settings() { _ = SaaSPrincipalFromContext() }\n');
    await fs.appendFile(path.join(root, 'internal', 'server', 'routes.go'), '\nfunc registerSaaS(router Router, handler SaaSHandler) { router.Handle("GET", "/dashboard/saasAdmin/settings", handler.Settings) }\n');
  }, async (root) => {
    const result = runIdentitySingleCorpGate(root);
    assert.ok(result.saasPrincipalRoutes.some((route) => route.route === '/dashboard/saasAdmin/settings'));
  });
});

test('RED: an unclassified dashboard route cannot be treated as page-mapped', async () => {
  await withFixture(async (root) => {
    await fs.appendFile(path.join(root, 'internal', 'server', 'routes.go'), '\nfunc registerUnknown(router Router, handler Handler) { router.Handle("GET", "/dashboard/unknown", handler.Index) }\n');
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /unknown dashboard route|route policy|dashboard\/unknown/i);
  });
});

test('RED: a principal token in another same-file handler cannot back the registered handler', async () => {
  await withFixture(async (root) => {
    await fs.appendFile(path.join(root, 'internal', 'dashboard', 'handler.go'), '\nfunc (h Handler) SameFile() { println("business") }\nfunc (h Handler) Unrelated() { _ = DashboardPrincipalFromContext() }\n');
    await fs.appendFile(path.join(root, 'internal', 'server', 'routes.go'), '\nfunc registerSameFile(router Router, handler Handler) { router.Handle("GET", "/dashboard/same-file", handler.SameFile) }\n');
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /DashboardPrincipal.*consumer|principal.*consumer/i);
  });
});

test('GREEN: server switch, cmd composition, and module registrar resolve concrete handlers', async () => {
  await withFixture(async (root) => {
    await fs.appendFile(path.join(root, 'internal', 'dashboard', 'handler.go'), '\nfunc (h Handler) Server() { _ = DashboardPrincipalFromContext() }\nfunc (h Handler) Cmd() { _ = DashboardPrincipalFromContext() }\nfunc (h Handler) Module() { _ = DashboardPrincipalFromContext() }\n');
    await fs.writeFile(path.join(root, 'internal', 'server', 'dispatch.go'), 'package server\nfunc (s *Server) ServeHTTP(w W, r R) {\n  switch {\n  case r.URL.Path == "/dashboard/server" && r.Method == http.MethodGet:\n    s.server.ServeHTTP(w, r)\n  case r.URL.Path == "/dashboard/cmd" && r.Method == http.MethodGet:\n    s.cmd.ServeHTTP(w, r)\n  }\n}\n');
    await fs.writeFile(path.join(root, 'internal', 'server', 'composition.go'), 'package server\nfunc build(handler Handler) { WithServerHandler(http.HandlerFunc(handler.Server)) }\n');
    await fs.writeFile(path.join(root, 'cmd', 'mochat-go', 'main.go'), 'package main\nfunc options(handler Handler) { compatserver.WithCmdHandler(http.HandlerFunc(handler.Cmd)) }\n');
    await fs.writeFile(path.join(root, 'internal', 'modules', 'module.go'), 'package modules\nfunc Register(registrar Registrar, handler Handler) { registrar.Handle(http.MethodPost, "/dashboard/module", handler.Module) }\n');
    await fs.appendFile(path.join(root, 'internal', 'modules', 'module.go'), '\nfunc RegisterClosure(registrar Registrar) { registrar.Handle("POST", "/dashboard/closure", func(w W, r R) { _ = DashboardPrincipalFromContext() }) }\n');
  }, async (root) => {
    const result = runIdentitySingleCorpGate(root);
    for (const route of ['/dashboard/server', '/dashboard/cmd', '/dashboard/module', '/dashboard/closure']) {
      assert.ok(result.dashboardPrincipalRoutes.some((item) => item.route === route), route);
    }
  });
});

test('RED: production code cannot select plaintext credentials or expose them in logs/responses', async () => {
  await withFixture(async (root) => {
    await fs.mkdir(path.join(root, 'internal', 'companyprofile'), { recursive: true });
    await fs.writeFile(path.join(root, 'internal', 'companyprofile', 'secret.go'), 'package companyprofile\nconst query = "SELECT employee_secret FROM mc_corp"\nfunc Leak(secret string) { log.Printf("employeeSecret=%s", secret); writeJSON(map[string]string{"employeeSecret": secret}) }\n');
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /plaintext secret|secret response|secret log/i);
  });
});

test('RED: side-effect imports and re-exports are part of the production graph', async () => {
  await withFixture(async (root) => {
    await fs.writeFile(path.join(root, 'web', 'apps', 'dashboard', 'src', 'side-effect.ts'), 'export const forbidden = "新建企业";\n');
    await fs.appendFile(path.join(root, 'web', 'apps', 'dashboard', 'src', 'main.tsx'), '\nimport "./side-effect";\nexport * from "./side-effect";\n');
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /新建企业|company selector/i);
  });
});

test('RED: a missing explicit production entry is not replaced by scanning the whole tree', async () => {
  await withFixture(async (root) => {
    await fs.rm(path.join(root, 'web', 'apps', 'dashboard', 'src', 'main.tsx'));
  }, async (root) => {
    assert.throws(() => runIdentitySingleCorpGate(root), /production entry/i);
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
