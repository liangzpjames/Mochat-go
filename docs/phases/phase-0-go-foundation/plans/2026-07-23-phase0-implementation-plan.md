# MoChat Go Phase 0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立渐进式模块化单体的开发底座、工程门禁、本地 Docker 验证入口与 Phase 0 接手台账，并将成果持续保存到独立 Git 分支。

**Architecture:** 保持现有 Go 单体和业务行为不变，先新增独立的运行角色包和架构规则，不一次性搬迁 `main.go`、`dashboard`、`store` 的全部逻辑。新需求统一进入 `internal/modules/<domain>`，旧代码按“触碰即迁移”逐步收敛。

**Tech Stack:** Go 1.26、标准库、POSIX shell、Docker/Compose、GitHub Actions、Markdown。

## Global Constraints

- 默认运行角色必须为 `all`，保持现有启动行为。
- Phase 0 不修改现有 API 路由、数据库语义、租户隔离和任务业务逻辑。
- 本机无 Go 工具链时，所有 Go 验证必须能在 Docker 中运行。
- 不提交 `.env.local`、认证状态、运行输出、上传数据或真实密钥。
- 新的接手、架构、计划、台账和验收文档统一位于 `docs/handle/`。
- 巨型文件不得在没有例外记录的情况下继续增长。

---

### Task 1: 运行角色基础类型

**Files:**

- Create: `internal/app/runtime/role.go`
- Create: `internal/app/runtime/role_test.go`

**Interfaces:**

- Produces: `type Role string`
- Produces: `ParseRole(string) (Role, error)`
- Produces: `(Role).RunsAPI() bool`
- Produces: `(Role).RunsWorkers() bool`
- Produces: `(Role).RunsScheduler() bool`

- [ ] **Step 1: 写角色解析失败测试**

覆盖空值默认为 `all`、四种合法值和非法值报错。

- [ ] **Step 2: 在 Docker Go 工具链内运行测试并确认失败**

Run:

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.26-alpine go test ./internal/app/runtime
```

Expected: FAIL，因为包或接口尚不存在。

- [ ] **Step 3: 实现最小角色类型**

角色矩阵：

| Role | API | Workers | Scheduler |
| --- | --- | --- | --- |
| `all` | yes | yes | yes |
| `api` | yes | no | no |
| `worker` | no | yes | no |
| `scheduler` | no | no | yes |

- [ ] **Step 4: 运行包测试并确认通过**

- [ ] **Step 5: 提交**

```bash
git add internal/app/runtime
git commit -m "feat: define application runtime roles"
```

### Task 2: 配置接入和启动边界

**Files:**

- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `deploy/standalone/.env.example`
- Modify: `deploy/standalone/docker-compose.yml`

**Interfaces:**

- Consumes: `runtime.ParseRole`
- Produces: `Config.RuntimeRole runtime.Role`
- Environment: `MOCHAT_GO_RUNTIME_ROLE=all|api|worker|scheduler`

- [ ] **Step 1: 写环境配置失败测试**

断言未设置时为 `all`，设置 `api` 时只启用 API 语义，非法值由 `FromEnv()` 返回错误。

- [ ] **Step 2: Docker 内运行相关测试并确认失败**

- [ ] **Step 3: 在配置中解析运行角色**

不得在 `main.go` 重复解析字符串。

- [ ] **Step 4: 在现有装配代码中加入角色门禁**

- API 角色控制 HTTP 和静态前端监听；
- worker 角色控制 Redis consumer 注册；
- scheduler 角色控制 periodic/cron 注册；
- `all` 保持原行为。

Phase 0 允许现有 worker 与 cron 继续共用一个 `taskrunner.Group`，但注册条件必须通过角色方法区分。

- [ ] **Step 5: 更新 Compose 和 `.env.example`**

默认值显式为 `all`，不写入真实密钥。

- [ ] **Step 6: 运行配置、runtime 和全包测试**

- [ ] **Step 7: 提交**

```bash
git add internal/config cmd/mochat-go deploy/standalone
git commit -m "feat: support api worker and scheduler runtime roles"
```

### Task 3: 架构与文件增长门禁

**Files:**

- Create: `scripts/audit_architecture_boundaries.sh`
- Create: `scripts/architecture-size-baseline.txt`
- Create: `scripts/test_audit_architecture_boundaries.sh`
- Modify: `scripts/test.sh`
- Modify: `.github/workflows/mysql57-amd64.yml`

**Interfaces:**

- Produces: `./scripts/audit_architecture_boundaries.sh`
- Baseline format: `<byte-count> <repository-relative-path>`

- [ ] **Step 1: 写门禁自测**

在临时 fixture 中验证：

- domain 引用 `internal/dashboard` 时失败；
- domain 引用 `internal/store` 时失败；
- 巨型文件超过基线时失败；
- 合法目录通过。

- [ ] **Step 2: 运行门禁自测并确认失败**

- [ ] **Step 3: 实现依赖与大小检查**

固定保护：

- `internal/dashboard/saas_admin_page.go`
- `internal/dashboard/saas_admin.go`
- `internal/store/mysql.go`
- `cmd/mochat-go/main.go`

- [ ] **Step 4: 把门禁接入 `scripts/test.sh` 和 CI**

- [ ] **Step 5: 运行门禁自测与真实仓库门禁**

- [ ] **Step 6: 提交**

```bash
git add scripts .github/workflows/mysql57-amd64.yml
git commit -m "ci: enforce modular architecture boundaries"
```

### Task 4: 新领域模块模板

**Files:**

- Create: `internal/modules/README.md`
- Create: `internal/modules/example/domain/module.go`
- Create: `internal/modules/example/domain/module_test.go`
- Create: `internal/modules/example/application/service.go`
- Create: `internal/modules/example/ports/repository.go`
- Create: `internal/modules/example/adapters/memory/repository.go`
- Create: `internal/modules/example/transport/http/handler.go`

**Interfaces:**

- Produces: 一个可编译但不挂载业务路由的示例模块。
- Domain 类型不得依赖 application、adapter、transport、dashboard 或 store。

- [ ] **Step 1: 写示例领域规则测试并确认失败**

- [ ] **Step 2: 实现最小示例模块**

示例只展示依赖方向，不写入现有数据库和路由。

- [ ] **Step 3: 运行示例包测试和架构门禁**

- [ ] **Step 4: 提交**

```bash
git add internal/modules
git commit -m "docs: add modular domain development template"
```

### Task 5: Docker 开发验证入口

**Files:**

- Create: `deploy/dev/docker-compose.yml`
- Create: `scripts/dev_check.sh`
- Create: `docs/handle/LOCAL_DEVELOPMENT.zh-CN.md`
- Modify: `.dockerignore`

**Interfaces:**

- Produces: `./scripts/dev_check.sh quick`
- Produces: `./scripts/dev_check.sh build`
- Produces: `./scripts/dev_check.sh standalone`

- [ ] **Step 1: 写参数和前置条件自测**

非法参数必须非零退出；Docker 不可用时给出明确提示。

- [ ] **Step 2: 实现统一入口**

- `quick`：Docker 内执行架构门禁、`go test ./...`、`go vet ./...` 和所有命令构建；
- `build`：构建正式 Docker 镜像；
- `standalone`：运行 standalone core/saas/frontend 分组验收。

- [ ] **Step 3: 启动 Docker Desktop 并验证 daemon**

- [ ] **Step 4: 执行 quick 和 build**

- [ ] **Step 5: 记录命令、耗时和结果**

- [ ] **Step 6: 提交**

```bash
git add deploy/dev scripts/dev_check.sh docs/handle/LOCAL_DEVELOPMENT.zh-CN.md .dockerignore
git commit -m "dev: add docker based local verification workflow"
```

### Task 6: Phase 0 接手台账和文档约束

**Files:**

- Create: `docs/handle/README.md`
- Create: `docs/handle/ARCHITECTURE.zh-CN.md`
- Create: `docs/handle/PHASE0_CHECKLIST.zh-CN.md`
- Create: `docs/handle/RESOURCE_AND_SECRET_INVENTORY.zh-CN.md`
- Create: `docs/handle/RISK_REGISTER.zh-CN.md`
- Create: `docs/handle/REQUIREMENT_REGISTER.zh-CN.md`
- Create: `docs/handle/DEFECT_REGISTER.zh-CN.md`
- Create: `docs/handle/PRODUCTION_EVIDENCE_REGISTER.zh-CN.md`

**Interfaces:**

- Produces: 后续接手与开发文档唯一入口 `docs/handle/README.md`。

- [ ] **Step 1: 写目录索引和归档规则**

- [ ] **Step 2: 写架构开发指南**

- [ ] **Step 3: 建立空白资源/密钥清单**

所有不可用项标记为“未具备”，Secret 值栏只能写存储位置，禁止写明文。

- [ ] **Step 4: 建立风险、需求、缺陷、生产证据台账**

把 readiness 的 6 个缺口登记为未完成，不改变原始生产证据。

- [ ] **Step 5: 写 Phase 0 检查表**

- [ ] **Step 6: 扫描占位符、矛盾和错误路径**

- [ ] **Step 7: 提交**

```bash
git add docs/handle
git commit -m "docs: establish phase 0 handover registers"
```

### Task 7: 全量验证、验收记录与分支同步

**Files:**

- Create: `docs/handle/PHASE0_VERIFICATION.zh-CN.md`
- Modify: `docs/handle/PHASE0_CHECKLIST.zh-CN.md`

**Interfaces:**

- Consumes: Tasks 1–6 全部交付物。

- [ ] **Step 1: 执行新鲜 quick 验证**

- [ ] **Step 2: 构建正式镜像**

- [ ] **Step 3: 执行可用的 standalone 分组验收**

- [ ] **Step 4: 扫描待提交文件和敏感信息**

- [ ] **Step 5: 写实际验证记录**

不得把未执行、跳过或失败项写成通过。

- [ ] **Step 6: 提交验证记录**

```bash
git add docs/handle
git commit -m "docs: record phase 0 verification results"
```

- [ ] **Step 7: 推送 Phase 0 分支并核对远端 SHA**

远端 `main` 保持不变；替换 `main` 必须等 Phase 0 验证完成后再执行。
