# Docker Desktop 合流与存储清理实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 app、内置 worker、独立 archive bridge 合入唯一 `mochat-go-desktop` Compose 项目，保全四个指定卷，删除临时验收栈与其他 Docker 存储，并留下可访问、可重启的本地环境。

**Architecture:** 修改 standalone Compose 和镜像构建，使 bridge 作为内部独立容器、simulator 作为显式 profile 工具。先备份并恢复 desktop 原数据，再验收双模式 fixture，最后按精确对象清单删除非保留容器/卷/镜像/缓存。

**Tech Stack:** Docker Desktop、Docker Compose v2、BuildKit、MariaDB、Redis、PowerShell、Go/Node 构建门禁。

## Global Constraints

- 只保留 `mochat-go-desktop_mysql-data`、`mochat-go-desktop_redis-data`、`mochat-go-desktop_app-storage`、`mochat-go-desktop_audit-anchor-storage` 四个数据卷。
- 不 reset/clean 主工作树，不删除或提交用户的 `.workbuddy/`、`tmp/`、调试脚本、旧构建产物和未提交文档。
- 清理前必须解析并复核每个目标；Windows 删除操作使用同一 PowerShell 上下文和绝对路径。
- bridge 不映射宿主机端口；长期应用入口为 `http://127.0.0.1:18080`。
- 无真实企微配置时 app 页面返回正常空数据，不因为 bridge 未配置导致全页加载失败。
- 迁移、健康检查、重启恢复和浏览器点击验收完成前不得删除临时验收栈。

---

### Task 1: Compose 和镜像合同

**Files:**
- Create: `deploy/standalone/archive-bridge.Dockerfile`
- Create: `deploy/standalone/docker-compose.contract_test.go`
- Modify: `deploy/standalone/docker-compose.yml`
- Modify: `deploy/standalone/README.md`
- Modify: `Dockerfile`
- Modify: `.dockerignore`

**Interfaces:**
- Produces: `archive-bridge` 内部服务和 `archive-simulator` 一次性 profile 服务。
- Consumes: simulator 计划产出的 `/usr/local/bin/mochat-archive-bridge` 与 `/usr/local/bin/mochat-archive-simulator`。

- [ ] **Step 1: 写失败 Compose 合同测试**

测试 bridge 无 `ports`、有 healthcheck、app depends_on healthy bridge、simulator 属于 `archive-fixture` profile、四个卷名稳定、fixture/sdk 互斥配置存在。

- [ ] **Step 2: 运行 RED**

Run: `go test ./deploy/standalone -run TestComposeArchiveBridge -count=1`

Expected: FAIL，compose 没有 bridge/simulator 服务。

- [ ] **Step 3: 实现 compose 和 Dockerfile**

app 使用 `http://archive-bridge:8083`；bridge admin/fixture bearer 从受保护 env file 读取；fixture store 挂载 `app-storage:/app/storage`，不新增 named volume。

- [ ] **Step 4: 运行 GREEN 与配置渲染**

Run: `go test ./deploy/standalone -count=1`

Run: `docker compose -p mochat-go-desktop --profile app --profile archive-fixture -f deploy/standalone/docker-compose.yml config --quiet`

Expected: PASS / exit 0。

- [ ] **Step 5: 提交**

```text
git add deploy/standalone/archive-bridge.Dockerfile deploy/standalone/docker-compose.contract_test.go deploy/standalone/docker-compose.yml deploy/standalone/README.md Dockerfile .dockerignore
git commit -m "build: consolidate archive bridge compose"
```

### Task 2: 全量代码门禁

**Files:**
- Create: `docs/verification/2026-08-27-docker-desktop-gates.zh-CN.md`

- [ ] **Step 1: 执行 Go 和专项合同**

Run: `go test ./... -count=1`

Run: `go test ./internal/dashboard -run Phase4 -count=1`

Run: `go test ./internal/modules/providers/... -run Completion -count=1`

Expected: PASS；MariaDB integration 无 DSN 时在报告中标 `SKIP`。

- [ ] **Step 2: 执行四前端门禁**

Run:

```text
pnpm --filter @mochat/dashboard lint && pnpm --filter @mochat/dashboard typecheck && pnpm --filter @mochat/dashboard test && pnpm --filter @mochat/dashboard build
pnpm --filter @mochat/sidebar lint && pnpm --filter @mochat/sidebar typecheck && pnpm --filter @mochat/sidebar test && pnpm --filter @mochat/sidebar build
pnpm --filter @mochat/operation lint && pnpm --filter @mochat/operation typecheck && pnpm --filter @mochat/operation test && pnpm --filter @mochat/operation build
pnpm --filter @mochat/saas-admin lint && pnpm --filter @mochat/saas-admin typecheck && pnpm --filter @mochat/saas-admin test && pnpm --filter @mochat/saas-admin build
```

Expected: 四个包全部 exit 0。

- [ ] **Step 3: 执行页面证据、圆弧 benchmark 和迁移合同**

Run:

```text
pnpm check:phase4-dashboard-page-rbac
pnpm check:provider-completion
pnpm check:yuanhu-benchmark
pnpm check:dashboard-all-pages-evidence
pnpm check:wecom-archive-saas-activation
```

把每条命令、exit code、PASS/SKIP 写入报告，不以旧结果代替。

- [ ] **Step 4: 执行 diff 门禁并提交报告**

Run: `git diff --check`

Expected: 无输出、exit 0。

```text
git add docs/verification/2026-08-27-docker-desktop-gates.zh-CN.md
git commit -m "docs: record archive desktop gates"
```

### Task 3: 备份并启动 desktop 数据层

**Files:**
- Create at runtime inside preserved volume: `/app/storage/backups/local-maintenance-<UTC>/desktop.sql.gz`

- [ ] **Step 1: 记录只读清单**

运行 `docker compose ls -a`、`docker ps -a`、`docker volume ls`、`docker system df -v`，保存脱敏结果；核对四个保留卷标签与挂载。

- [ ] **Step 2: 启动旧 desktop MySQL/Redis 并备份**

只启动 `mochat-go-desktop` 的 mysql/redis；使用容器内 `mariadb-dump` 输出到 app-storage，验证 gzip 可读、SQL 头和非零表数。

- [ ] **Step 3: 构建新镜像并执行迁移**

Run: `docker compose -p mochat-go-desktop --profile app --profile archive-fixture -f deploy/standalone/docker-compose.yml build`

运行一次性 migrate，检查 `0169` ledger/checksum，再启动 bridge/app。

- [ ] **Step 4: 健康和原数据验证**

验证 mysql/redis/bridge/app healthy、`/healthz`、`/readyz`、已有租户与登录身份数量不下降、核心空数据 API 返回 200。

### Task 4: 双模式 Docker 端到端

**Files:**
- Create: `docs/verification/2026-08-27-docker-dual-mode-e2e.zh-CN.md`

- [ ] **Step 1: 建立两个显式本地验收租户**

使用正式 SaaS 开户 API 分别创建 `self_built` 与 `third_party_delegated` 租户；数据集名称带 `MOCHAT-LOCAL-SIM`，密码只写受保护本地文件。

- [ ] **Step 2: 执行 self-built 场景**

通过 CLI seed/send 注入文本、图片、语音、视频、文件；点击或调用正式手动同步；验证 cursor、重放、媒体分片、对象摘要和鉴权读取。

- [ ] **Step 3: 执行 delegated 场景**

完成 suite ticket、授权换码和 data-zone seed/send；验证 metadata 落库、component 鉴权展示、普通数据库/对象存储不含正文。

- [ ] **Step 4: 故障与重启**

注入密文篡改/缺失媒体/过期 token，验证 cursor 不推进；重启 app/bridge/mysql/redis 后恢复并验证无重复消息。

- [ ] **Step 5: fixture cleanup**

先 dry-run 再精确 confirm；验证两个 fixture 数据集被删除，desktop 原有租户和非 fixture 数据保持。

### Task 5: 浏览器验收

**Files:**
- Create: `docs/verification/2026-08-27-docker-browser-acceptance.zh-CN.md`

- [ ] **Step 1: Desktop 页面**

在 `http://127.0.0.1:18080` 登录，实际点击唯一企业资料、立即同步会话、全局消息和会话详情；检查刷新恢复、空态、失败态、网络和控制台。

- [ ] **Step 2: 媒体与组件**

打开自建图片/音频/视频/文件及第三方安全展示组件；未登录、跨租户和无权限请求必须被拒绝。

- [ ] **Step 3: 响应式**

检查桌面宽度和涉及员工端的 `390×844`；记录真实通过/失败项，不用截图代替点击。

### Task 6: 精确删除临时栈与非保留存储

**Files:**
- Create: `docs/verification/2026-08-27-docker-cleanup-report.zh-CN.md`

- [ ] **Step 1: 再次生成删除候选清单**

集合为：所有非 `mochat-go-desktop` 容器、所有不在四个保留名中的 volume、非 desktop Compose 网络、未被运行容器使用的镜像和 build cache。

- [ ] **Step 2: 验证保护集合**

脚本必须断言删除候选与四个保留卷交集为空，并输出每个精确目标；若交集非空立即退出。

- [ ] **Step 3: 删除临时验收项目**

先 `docker compose -p mochat-wecom-acceptance-20260827 down --volumes --remove-orphans`，再删除其剩余精确对象。

- [ ] **Step 4: 删除其他非保留对象**

使用 PowerShell `Remove-Item` 仅处理已解析的宿主机临时路径；Docker 对象使用精确 ID/name，不把 PowerShell 枚举结果交给其他 shell。

- [ ] **Step 5: 清理镜像和构建缓存**

执行 unused image prune 和 BuildKit/buildx cache prune；不停止 desktop 运行容器，不删除其镜像层。

- [ ] **Step 6: 记录回收量**

比较前后 `docker system df -v`；报告逻辑回收量和 Windows VHDX 物理占用边界。

### Task 7: 最终恢复性和交付门禁

**Files:**
- Create: `docs/verification/2026-08-27-docker-desktop-final-report.zh-CN.md`

- [ ] **Step 1: 重启整个 Compose 项目**

Run: `docker compose -p mochat-go-desktop --profile app --profile archive-fixture -f deploy/standalone/docker-compose.yml restart`

Expected: mysql/redis/bridge/app 全部恢复健康。

- [ ] **Step 2: 复验数据、登录和模拟发送**

确认原有数据数量、登录、空数据页和一条新 self-built/third-party 模拟文本均可用；最后清理这两条终验数据。

- [ ] **Step 3: 最终代码与 Compose 门禁**

Run: `go test ./... -count=1`

Run: 四前端 lint/typecheck/test/build。

Run: `docker compose ... config --quiet`

Run: `git diff --check`

Expected: 全部 exit 0，Integration 无 DSN 时仅能记录 SKIP。

- [ ] **Step 4: 提交报告**

```text
git add docs/verification/2026-08-27-docker-browser-acceptance.zh-CN.md docs/verification/2026-08-27-docker-cleanup-report.zh-CN.md docs/verification/2026-08-27-docker-desktop-final-report.zh-CN.md
git commit -m "docs: verify desktop archive deployment"
```
