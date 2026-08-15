# Provider 基础与企业微信标准同步实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (本 delegated 线程按批次内联执行；每项都需 TDD RED→GREEN、独立测试与提交)。

**Goal:** 建立 truthful Provider 注册/状态基础，修复 WeCom archive 的假 ready，并用现有真实 WeCom HTTP client 完成标准员工同步合同与 Dashboard 状态读接口。

**Architecture:** `internal/modules/providers` 提供来源/状态/能力/稳定错误码与分类注册表；`internal/providerstatus` 负责基于认证 principal 的只读 resolver/HTTP 合同；`internal/dashboard.RoomWelcomeWeComClient` 保持唯一标准 WeCom HTTP 适配器，新增合同测试而不复制 client；Dashboard 通过相对 `/providers/status` API 与可见状态卡片消费结果。

**Tech Stack:** Go 1.26、`net/http`、`httptest`、React 19、TypeScript、Vitest、Node completion gate。

## Global Constraints

- 只在 `D:\workspace\mochat-go\mochat-go\.worktrees\phase6-provider-foundation-wecom-closeout` 写入，保护主工作区 dirty 文件。
- 不操作服务器、生产数据库、Docker、named volumes、`down -v`、`volume rm`、`system prune`。
- 不发送真实 AI/WeCom 测试请求；live 凭据缺失时只做 fake HTTP 合同。
- 不把 fixture、注释、manifest 或字符串门禁当作产品完成证据。
- 不输出或记录 secret、token、私钥、原始外部响应 body。

### Task 1: Provider 状态模型与分类注册表

**Files:**
- Modify: `internal/modules/providers/providers.go`
- Create: `internal/modules/providers/registry.go`
- Test: `internal/modules/providers/registry_test.go`
- Create: `scripts/check_provider_completion.mjs`
- Test: `scripts/check_provider_completion.test.mjs`

**Interfaces:**
- `providers.Status` exposes `Code`, `Source`, `Action`, `Capabilities`, `Missing`, `LastSuccessAt`, `LastFailureAt`, `LastErrorCode` without secret fields.
- `providers.Registration` contains non-empty `Kind`, `Source`, `Capabilities`, and `StatusProvider`.
- `providers.NewRegistry`, `Register`, `Snapshot` are deterministic and reject unknown/empty classifications.

- [ ] **Step 1: Write failing registry tests** for unknown source, duplicate kind, sorted snapshot, and JSON secret absence.
- [ ] **Step 2: Run** `go test ./internal/modules/providers -run 'TestRegistry|TestStatus' -count=1`; expect failure because registry API is absent.
- [ ] **Step 3: Implement** the minimal source enum, extended status fields, machine-code errors, registry validation and defensive snapshot copy.
- [ ] **Step 4: Run** the focused tests and `go test ./internal/modules/providers/... -count=1`; expect pass.
- [ ] **Step 5: Add failing completion-gate fixture tests** that inspect real Go registrations and reject unclassified provider directories or archive ready state.
- [ ] **Step 6: Implement** `scripts/check_provider_completion.mjs` using AST-light source parsing restricted to `.go` files under `internal/modules/providers`, excluding tests/fixtures/comments for evidence.
- [ ] **Step 7: Run** `node --test scripts/check_provider_completion.test.mjs`; expect pass with a temporary test fixture held in memory only.
- [ ] **Step 8: Commit** `git add internal/modules/providers scripts/check_provider_completion* && git commit -m "feat(provider): add classified status registry"`.

### Task 2: WeCom archive truthful status and standard HTTP contract

**Files:**
- Modify: `internal/modules/providers/archive/wecom/archive.go`
- Modify: `internal/modules/providers/archive/wecom/archive_test.go`
- Modify: `internal/dashboard/room_welcome_wecom.go` only if contract hardening is required
- Create/Modify: `internal/dashboard/wecom_provider_contract_test.go`

**Interfaces:**
- `archive.Archive.Status()` returns `limited` with code `archive.getchatdata_unimplemented` whenever all archive credentials are present but no real source is injected.
- `archive.Archive.Sync()` returns `providers.ErrCapabilityUnavailable` for that state and never returns success counts.
- Existing `RoomWelcomeWeComClient` remains the implementation of `WorkEmployeeSyncClient`.

- [ ] **Step 1: Change archive tests first** to require `limited`, exact machine code, no secret in reason, and `errors.Is(err, providers.ErrCapabilityUnavailable)` for complete credentials.
- [ ] **Step 2: Run** `go test ./internal/modules/providers/archive/wecom -run 'TestArchiveStatusAndSync' -count=1`; expect failure because complete credentials currently report ready and activation text error.
- [ ] **Step 3: Implement** the fail-closed state and stable error without changing crypto helpers.
- [ ] **Step 4: Run** archive tests; expect pass.
- [ ] **Step 5: Write fake-server RED tests** for `gettoken`, department list, user list, query parameters, WeCom `errcode`, missing access token, and no secret leakage.
- [ ] **Step 6: Run** `go test ./internal/dashboard -run 'TestRoomWelcomeWeComClient.*Contract' -count=1`; expect failure only for newly required contract behavior.
- [ ] **Step 7: Make the smallest client hardening changes** needed to normalize stable errors and preserve token caching; do not add production fixtures or live calls.
- [ ] **Step 8: Run** focused Dashboard WeCom tests and existing employee sync tests; commit `feat(provider): close WeCom archive and standard sync contract`.

### Task 3: Authenticated Provider status service and Dashboard route

**Files:**
- Create: `internal/providerstatus/model.go`
- Create: `internal/providerstatus/service.go`
- Create: `internal/providerstatus/http.go`
- Create: `internal/providerstatus/service_test.go`
- Create: `internal/providerstatus/http_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/company_profile_route_test.go` or create a dedicated route test

**Interfaces:**
- `providerstatus.Resolver.Resolve(context.Context, dashboardprincipal.DashboardPrincipal) (providerstatus.View, error)`.
- `providerstatus.NewHTTPHandler(resolver)` serves `GET /dashboard/providers/status` and returns the existing `{code,errorCode,msg,data}` envelope.
- View contains only principal-visible statuses; no tenant/corp/actor input fields are accepted.

- [ ] **Step 1: Write RED service/HTTP tests** for missing principal, scope passed from context, non-superadmin filtered view, superadmin diagnostic view, body/query scope injection rejection, and stable error codes.
- [ ] **Step 2: Run** focused providerstatus tests; expect missing package/API failures.
- [ ] **Step 3: Implement** resolver contract, status projection, handler authentication and route-safe envelope.
- [ ] **Step 4: Add `WithProviderStatusHandler` and `GET /dashboard/providers/status` dispatch** with server route tests.
- [ ] **Step 5: Run** `go test ./internal/providerstatus ./internal/server -run 'ProviderStatus|provider status' -count=1` and relevant existing route tests.
- [ ] **Step 6: Commit** `feat(dashboard): expose scoped provider status contract`.

### Task 4: Dashboard status API and UI

**Files:**
- Create: `web/apps/dashboard/src/features/provider-status/provider-status-api.ts`
- Create: `web/apps/dashboard/src/features/provider-status/provider-status-api.test.ts`
- Create: `web/apps/dashboard/src/features/provider-status/provider-status-page.tsx`
- Create: `web/apps/dashboard/src/features/provider-status/provider-status-page.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx` only if a manifest route already exists for provider diagnostics

**Interfaces:**
- API calls relative `/providers/status`, normalizes only `ready`/`limited`/`unavailable`, and preserves machine code/source/action.
- Page renders status locally near each Provider, hides absent diagnostic fields, and never renders raw error body/secret values.

- [ ] **Step 1: Write Vitest RED tests** for relative endpoint, status normalization, source/action display, empty/error state and secret omission.
- [ ] **Step 2: Run** `corepack pnpm --filter @mochat/dashboard test -- provider-status`; expect failure because files are absent.
- [ ] **Step 3: Implement** API normalization and compact status card/page using existing dashboard shell styles.
- [ ] **Step 4: Add the page only through an existing authorized settings route if one exists; otherwise keep the UI component/API ready without inventing an unmanifested page.**
- [ ] **Step 5: Run** focused Vitest, typecheck and build; commit `feat(dashboard): render provider runtime statuses`.

### Task 5: Final verification and handoff

**Files:**
- Modify: `docs/superpowers/reviews/2026-08-14-provider-foundation-wecom-closeout-review.zh-CN.md`

- [ ] **Step 1: Run focused Go suites** for providers, archive, dashboard WeCom contract, providerstatus, and server routes.
- [ ] **Step 2: Run Dashboard** `corepack pnpm --filter @mochat/dashboard typecheck`, `lint`, `test`, `build`.
- [ ] **Step 3: Run** `node scripts/check_provider_completion.mjs` and capture real source counts/result.
- [ ] **Step 4: Run** `go test ./... -count=1`; distinguish unchanged migration expectation failures from changed failures and do not fix unrelated baseline drift.
- [ ] **Step 5: Inspect** `git diff --check`, `git status --short`, changed-file list, and all commit SHAs; confirm no Docker/DB/output/server changes.
- [ ] **Step 6: Request read-only code review** over the final commit range; fix Critical/Important findings with tests and independent commits.
- [ ] **Step 7: Commit** the Chinese review/handoff record and stop before deployment, reporting external live blockers and main-workspace overlap.
