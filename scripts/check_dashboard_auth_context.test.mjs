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

test('audits only production handlers reachable from real server dispatch and composition', async (t) => {
  const root = await fixture();
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const result = auditDashboardAuthContext(root, {
    exemptions: [{ method: 'POST', route: '/dashboard/auth/logout', handlerSymbol: 'DashboardAuthHandler.Logout', operation: 'DeleteUserCorpCache' }],
  });
  assert.equal(result.routes.length, 9);
  assert.deepEqual(result.violations.map((item) => [item.route, item.operation]), [
    ['/dashboard/header', 'Authorization'],
    ['/dashboard/legacy', 'UserCorpCache'],
    ['/dashboard/module/constant', 'UserCorpCache'],
    ['/dashboard/module/one', 'Authorization'],
    ['/dashboard/module/shared', 'UserCorpCache'],
    ['/dashboard/module/two', 'Authorization'],
    ['/dashboard/page/delete-cache', 'DeleteUserCorpCache'],
  ]);
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

test('requires exemption method route symbol and operation to match exactly', async (t) => {
  const root = await fixture();
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const result = auditDashboardAuthContext(root, {
    exemptions: [{ method: 'POST', route: '/dashboard/auth/logout', handlerSymbol: 'WrongHandler.Logout', operation: 'DeleteUserCorpCache' }],
  });
  assert.equal(result.violations.some((item) => item.route === '/dashboard/auth/logout' && item.operation === 'DeleteUserCorpCache'), true);
});
