# Phase 1 前端统一底座验收记录

> 验收日期：2026-07-28（UTC+8）
> 前端门禁提交：`ae9f0bdf04d2b876e6e9b1d0dd0a5f99827538da`
> 架构基线提交：`4906ed8`

## 结论

Phase 1 的 workspace、共享契约、Dashboard React 单入口、身份与企业上下文、受控 legacy 路由、首个 React 页面和 CI/E2E 门禁已经落地。前端本地门禁全部通过；Playwright 9 个场景通过真实 Go 静态路由层验证。

本结论不代表生产 readiness 完成。真实企微、微信开放平台、真实租户和生产环境证据仍未具备。Windows 上 `go test ./...` 仍有既有路径分隔符和 POSIX 文件权限断言失败，按当前任务约定未修改；Linux CI/Docker 是该门禁的权威平台。

计划要求的“干净工作区”和宿主机 `bash scripts/dev_check.sh quick` 未被字面满足：前者受未提交且明确排除的 `.superpowers/` 会话草稿影响，后者因本机没有可用 WSL Bash。表中保留失败证据，不把等价验证伪装成原命令成功。

## 验收明细

| time | commit | command | exit_code | evidence | result |
| --- | --- | --- | ---: | --- | --- |
| 2026-07-28 13:20 +08:00 | `ae9f0bd` | `git status --short` | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/known-limitations.txt` | 仅有未跟踪的会话草稿 `.superpowers/`；未达到字面上的空工作区，且该目录未提交 |
| 2026-07-28 13:41 +08:00 | `ae9f0bd` | `corepack pnpm install --frozen-lockfile` | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/windows-frontend-gates.txt` | lockfile 可复现 |
| 2026-07-28 13:26 +08:00 | `ae9f0bd` | `powershell -File scripts/test_frontend_check.ps1` | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/windows-frontend-gates.txt` | Windows 门禁失败传播用例通过 |
| 2026-07-28 13:26 +08:00 | `ae9f0bd` | `powershell -File scripts/frontend_check.ps1 quick` | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/windows-frontend-gates.txt` | audit、依赖、lint、typecheck、unit/contract 全绿 |
| 2026-07-28 13:27 +08:00 | `ae9f0bd` | `powershell -File scripts/frontend_check.ps1 build` | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/windows-frontend-gates.txt` | frozen lockfile 与两端生产构建通过 |
| 2026-07-28 13:27 +08:00 | `ae9f0bd` | `powershell -File scripts/frontend_check.ps1 e2e` | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/windows-frontend-gates.txt` | 登录、企业、401/403/404、React/legacy/API 路由通过 |
| 2026-07-28 13:25 +08:00 | `ae9f0bd` | `go test ./internal/frontend ./cmd/mochat-frontend-e2e` | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/windows-frontend-gates.txt` | Go 静态入口与 E2E server 通过 |
| 2026-07-28 13:25 +08:00 | `ae9f0bd` | `docker run --rm -v "${PWD}:/workspace" -w /workspace alpine:3.20 sh -n scripts/frontend_check.sh` | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/windows-frontend-gates.txt` | Linux shell 语法通过 |
| 2026-07-28 13:30 +08:00 | `ae9f0bd` | `go test ./...`（Windows） | 1 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/known-limitations.txt` | 已知平台限制：反斜杠路径及 POSIX mode 断言；未发现 Task 9 新失败 |
| 2026-07-28 13:34 +08:00 | `ae9f0bd` | `docker run --rm -v "${PWD}:/src:ro" -v mochat-go-mod-cache:/go/pkg/mod -v mochat-go-build-cache:/root/.cache/go-build -w /src golang:1.26-alpine sh -c 'go test ./... && go vet ./...'` | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/linux-go-gates.txt` | Linux 全量 Go 测试与 vet 通过 |
| 2026-07-28 13:48 +08:00 | `4906ed8` | Linux 等价 `dev_check quick` 内部门禁与 command builds | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/linux-go-gates.txt` | 架构审计、自测、dev_check 自测和全部 command build 通过 |
| 2026-07-28 13:41 +08:00 | `ae9f0bd` | `bash scripts/dev_check.sh quick` | 1 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/known-limitations.txt` | Windows 主机缺可用 Bash；完整等价内部命令已在 Linux Docker 通过 |
| 2026-07-28 13:28 +08:00 | `ae9f0bd` | `git diff --check` | 0 | `docs/phases/phase-1-frontend-foundation/evidence/frontend-phase1/windows-frontend-gates.txt` | 无空白错误 |

## 浏览器覆盖

- 受保护 URL 登录并安全返回；
- 多企业选择、切换与本地企业上下文；
- 401 清理会话并返回登录页；
- 403 和未知路由停留在 React 错误边界；
- `/corp/index` 由 React 渲染且动作权限生效；
- legacy allowlist 保留 query/hash；
- 未登记 legacy URL 返回 404；
- `/api` 未命中请求不返回 SPA document。

## 未关闭条件

- GitHub Actions 的远端门禁仍需在目标提交上成功，才能把 Phase 1 标记为“可发布”；本地 Docker Linux 全量 Go 测试与 vet 已通过。
- Dashboard 首包约 1.19 MB，当前仅告警；后续批次需要按页面切分。
- legacy 页面 E2E 已验证 Go 路由和静态 document，后续应增加关键资源及可见业务内容断言。
- 生产 readiness 六项仍受真实账号、租户、基础设施与运行证据阻塞。

## 发布纠偏补充验收（2026-07-28）

Phase 1 的发布链路、可达导航和登录表现已完成纠偏：

| command | result |
| --- | --- |
| `node --test scripts/check_frontend_release_chain.test.mjs` | 3/3 通过，静态验证 Dockerfile、同步脚本和 compose smoke 同时识别两套应用 |
| `pnpm --filter @mochat/dashboard test` | 13 个文件、56 项测试通过 |
| Dashboard `typecheck` / `lint` / `build` | 全部通过；首包体积告警保留给 Phase 2 拆包 |
| Docker Compose 重建 | `standalone-app-1`、MariaDB、Redis 均 healthy |
| `GET /`、`GET /login`、`GET /saas-admin/` | 均为 200 |
| Dashboard 与 SaaS HTML 身份比较 | 内容不同；分别引用 `/assets/index-B-R-tSYK.js` 与 `/saas-admin/assets/index-DaXsz8ef.js` |
| SaaS 静态资源请求 | 200 |
| `node --test scripts/audit_legacy_frontend_inventory.test.mjs` | 30/30 通过；临时审计夹具已包含 Corp Admin Go 契约证据，语义测试不再与真实 `/corp/update` 固定覆盖冲突 |

对应提交：`9f93cd9`（双应用发布）、`aa42a52`（授权导航）、`9a7eb1c`（登录呈现）。本地测试入口保持为 `http://127.0.0.1:18090/` 与 `http://127.0.0.1:18090/saas-admin/`。
