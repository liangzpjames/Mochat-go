import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

import { auditDashboardAuthContext } from './check_dashboard_auth_context.mjs';

async function fixture() {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'dashboard-auth-context-'));
  await fs.mkdir(path.join(root, 'internal', 'server'), { recursive: true });
  await fs.mkdir(path.join(root, 'internal', 'dashboard'), { recursive: true });
  await fs.mkdir(path.join(root, 'internal', 'modules', 'demo'), { recursive: true });
  await fs.mkdir(path.join(root, 'cmd', 'mochat-go'), { recursive: true });
  await fs.writeFile(path.join(root, 'internal', 'server', 'server.go'), `package server
type Server struct { safe, legacy, header, pageDelete, logout Handler }
func WithSafeHandler(h Handler) Option { return func(s *Server) { s.safe = h } }
func WithLegacyHandler(h Handler) Option { return func(s *Server) { s.legacy = h } }
func WithHeaderHandler(h Handler) Option { return func(s *Server) { s.header = h } }
func WithPageDeleteHandler(h Handler) Option { return func(s *Server) { s.pageDelete = h } }
func WithLogoutHandler(h Handler) Option { return func(s *Server) { s.logout = h } }
func (s *Server) ServeHTTP(w ResponseWriter, r *Request) {
  switch {
  case r.URL.Path == "/dashboard/safe" && r.Method == MethodGet && s.safe != nil:
    s.safe.ServeHTTP(w, r)
  case r.URL.Path == "/dashboard/legacy" && r.Method == MethodGet && s.legacy != nil:
    s.legacy.ServeHTTP(w, r)
  case r.URL.Path == "/dashboard/header" && r.Method == MethodGet && s.header != nil:
    s.header.ServeHTTP(w, r)
  case r.URL.Path == "/dashboard/page/delete-cache" && r.Method == MethodPost && s.pageDelete != nil:
    s.pageDelete.ServeHTTP(w, r)
  case r.URL.Path == "/dashboard/auth/logout" && r.Method == MethodPost && s.logout != nil:
    s.logout.ServeHTTP(w, r)
  }
}
`);
  await fs.writeFile(path.join(root, 'cmd', 'mochat-go', 'main.go'), `package main
func build() {
  page := dashboard.NewPageHandler()
  auth := dashboard.NewDashboardAuthHandler()
  _ = []server.Option{
    server.WithSafeHandler(http.HandlerFunc(page.Safe)),
    server.WithLegacyHandler(http.HandlerFunc(page.Legacy)),
    server.WithHeaderHandler(http.HandlerFunc(page.Header)),
    server.WithPageDeleteHandler(http.HandlerFunc(page.DeleteCache)),
    server.WithLogoutHandler(http.HandlerFunc(auth.Logout)),
  }
}
`);
  await fs.writeFile(path.join(root, 'internal', 'dashboard', 'page.go'), `package dashboard
func (h *PageHandler) Safe(w ResponseWriter, r *Request) {
  proof := "UserCorpCache r.Header.Get(\\\"Authorization\\\")"
  _ = proof
  // h.cache.UserCorpCache(r.Context(), 7)
}
func (h *PageHandler) Legacy(w ResponseWriter, r *Request) { h.cache.UserCorpCache(r.Context(), 7) }
func (h *PageHandler) Header(w ResponseWriter, r *Request) { r.Header.Get("Authorization") }
func (h *PageHandler) DeleteCache(w ResponseWriter, r *Request) { h.cache.DeleteUserCorpCache(r.Context(), 7) }
func (h *PageHandler) Unregistered(w ResponseWriter, r *Request) { h.cache.UserCorpCache(r.Context(), 7) }
func (h *DashboardAuthHandler) Logout(w ResponseWriter, r *Request) { h.cache.DeleteUserCorpCache(r.Context(), 7) }
`);
  await fs.writeFile(path.join(root, 'internal', 'dashboard', 'dashboard_route_registry.go'), `package dashboard
var dashboardRouteRegistry = []DashboardRoute{
  {Method: "GET", Path: "/dashboard/safe", Handler: "PageHandler.Safe", AuthKind: DashboardRouteAuthPrincipal},
  {Method: "GET", Path: "/dashboard/legacy", Handler: "PageHandler.Legacy", AuthKind: DashboardRouteAuthPrincipal},
  {Method: "GET", Path: "/dashboard/header", Handler: "PageHandler.Header", AuthKind: DashboardRouteAuthPrincipal},
  {Method: "POST", Path: "/dashboard/page/delete-cache", Handler: "PageHandler.DeleteCache", AuthKind: DashboardRouteAuthPrincipal},
  {Method: "POST", Path: "/dashboard/auth/logout", Handler: "DashboardAuthHandler.Logout", AuthKind: DashboardRouteAuthPublic},
  {Method: "GET", Path: "/dashboard/module/constant", Handler: "ModuleHandler.Legacy", AuthKind: DashboardRouteAuthPrincipal},
  {Method: "POST", Path: "/dashboard/module/one", Handler: "ModuleHandler.ServeHTTP", AuthKind: DashboardRouteAuthPrincipal},
  {Method: "POST", Path: "/dashboard/module/two", Handler: "ModuleHandler.ServeHTTP", AuthKind: DashboardRouteAuthPrincipal},
  {Method: "PUT", Path: "/dashboard/module/shared", Handler: "SharedHandler.ServeHTTP", AuthKind: DashboardRouteAuthPrincipal},
}
`);
  await fs.writeFile(path.join(root, 'internal', 'dashboard', 'page_test.go'), `package dashboard
func TestProof() { _ = "h.cache.UserCorpCache(r.Context(), 7)" }
`);
  await fs.writeFile(path.join(root, 'internal', 'modules', 'demo', 'routes.go'), `package demo
import nethttp "net/http"
const ModulePath = "/dashboard/module/constant"
type ModuleHandler struct{}
type Module struct { shared *SharedHandler }
type SharedHandler struct{}
func RegisterRoutes(registrar Registrar, handler *ModuleHandler) error {
  registrar.Handle(nethttp.MethodGet, ModulePath, nethttp.HandlerFunc(handler.Legacy))
  for _, page := range []string{"one", "two"} {
    registrar.Handle(nethttp.MethodPost, "/dashboard/module/"+page, handler)
  }
  return nil
}
func (m *Module) Register(registrar Registrar) error {
  return registrar.Handle("PUT", "/dashboard/module/shared", m.shared)
}
func (h *ModuleHandler) Legacy(w ResponseWriter, r *Request) { h.cache.UserCorpCache(r.Context(), 7) }
func (h *ModuleHandler) ServeHTTP(w ResponseWriter, r *Request) { r.Header.Get("Authorization") }
func (h *ModuleHandler) Unregistered(w ResponseWriter, r *Request) { h.cache.UserCorpCache(r.Context(), 7) }
func (h *SharedHandler) ServeHTTP(w ResponseWriter, r *Request) { h.cache.UserCorpCache(r.Context(), 7) }
`);
  return root;
}

test('exports route to handler and explicit auth metadata for production registrations only', async (t) => {
  const root = await fixture();
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const result = auditDashboardAuthContext(root);
  assert.equal(result.routes.length, 9);
  assert.deepEqual(result.violations, []);
  assert.ok(result.routes.every((route) => route.handlerSymbol && route.auth && route.authSource));
  assert.equal(result.routes.find((route) => route.route === '/dashboard/auth/logout')?.auth, 'public');
  assert.equal(result.routes.find((route) => route.route === '/dashboard/legacy')?.auth, 'dashboard-principal');
  assert.equal(result.violations.some((item) => item.handlerSymbol.endsWith('.Unregistered')), false);
  assert.equal(result.violations.some((item) => item.source.endsWith('_test.go')), false);
});

test('covers every dashboard contract discovered by the production catalog scanner', async (t) => {
  const root = await fixture();
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const result = auditDashboardAuthContext(root);
  assert.deepEqual(result.missingContracts, []);
  assert.equal(result.catalogContracts.length, 9);
});

test('public auth metadata requires the exact method and route contract', async (t) => {
  const root = await fixture();
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const result = auditDashboardAuthContext(root);
  assert.equal(result.routes.find((route) => route.route === '/dashboard/auth/logout' && route.method === 'POST')?.auth, 'public');
  assert.notEqual(result.routes.find((route) => route.route === '/dashboard/auth/logout' && route.method === 'GET')?.auth, 'public');
});

test('missing typed metadata cannot be bypassed by an unbound allowlist or a comment', async (t) => {
  const root = await fixture();
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const registryPath = path.join(root, 'internal', 'dashboard', 'dashboard_route_registry.go');
  const source = await fs.readFile(registryPath, 'utf8');
  await fs.writeFile(registryPath, source.replace(/^.*\/dashboard\/legacy.*\r?\n/m, '') + '\n// {Method: "GET", Path: "/dashboard/legacy", Handler: "PageHandler.Legacy", AuthKind: DashboardRouteAuthPrincipal}\n');
  const result = auditDashboardAuthContext(root, { exactUnboundContracts: new Set(['GET /dashboard/legacy']) });
  assert.ok(result.missingContracts.includes('GET /dashboard/legacy'));
});

test('handler and auth mutations are reported against the typed registry', async (t) => {
  const root = await fixture();
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const registryPath = path.join(root, 'internal', 'dashboard', 'dashboard_route_registry.go');
  const source = await fs.readFile(registryPath, 'utf8');
  await fs.writeFile(registryPath, source.replace('Handler: "PageHandler.Safe"', 'Handler: "PageHandler.Legacy"').replace('AuthKind: DashboardRouteAuthPrincipal', 'AuthKind: DashboardRouteAuthUnknown'));
  const result = auditDashboardAuthContext(root);
  assert.ok(result.violations.some((item) => item.route === '/dashboard/safe'));
});

test('production registry explicitly covers all securityMFA methods as identity authenticated', () => {
  const result = auditDashboardAuthContext(process.cwd());
  for (const method of ['GET', 'POST', 'PUT']) {
    assert.equal(result.routes.find((route) => route.method === method && route.route === '/dashboard/user/securityMFA')?.auth, 'identity-authenticated');
  }
});
