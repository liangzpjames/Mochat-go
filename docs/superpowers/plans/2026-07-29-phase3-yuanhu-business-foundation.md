# Phase 3 圆弧业务基础实施计划

> **供智能体执行：** 必须使用 `subagent-driven-development`（推荐）或 `executing-plans` 技能，按任务逐项实施。本计划使用复选框跟踪进度。

**目标：** 完成 Phase 3 第一个具备生产形态的垂直业务闭环：客户主数据、线索、商机、负责人/协作人，以及营销和报表后续依赖的稳定契约。

**架构：** 在现有多租户 Go 服务内扩展边界清晰的 SCRM 领域，不引入另一套应用框架。React Dashboard 通过现有 API Client 和查询缓存调用带版本的类型化接口。领域事件为后续报表、风险和 AI 模块提供输入，但第一阶段不依赖这些模块的实现。

**技术栈：** Go 1.26、MySQL/MariaDB、Redis、React 19、TypeScript、Ant Design、TanStack Query、Vitest、Playwright、Docker Compose。

## 全局约束

- 每条记录必须包含租户范围；企业级记录还必须通过当前用户的企业访问校验。
- 数据权限必须支持本人、协作人、部门、下级部门和全租户范围。
- 可变资源使用乐观版本控制；可能重试的写操作使用稳定幂等键。
- 本阶段不实现 AI 功能。
- 自动化测试不得发送真实消息、执行真实群发、购买或触发有破坏性的第三方操作。
- 在真实页面替换统一承接页期间，Phase 2 已有 URL 和路由行为必须保持稳定。
- Phase 2.2 的 `GET/POST /api/phase2-2/scrm/leads` 是默认关闭的架构与装配 pilot 验证面，不是 Phase 3 正式 API 契约。Phase 3 的资源、路径、请求/响应和兼容性要求以任务 1 产出的契约及任务 3 的正式实现为准，不得把 pilot 路由当作必须延续的产品接口。

---

### 任务 1：固定 SCRM 领域词汇和接口契约

**文件：**

- 新建：`docs/phases/phase-3-yuanhu-benchmark/scrm-domain-contract.md`
- 新建：`docs/phases/phase-3-yuanhu-benchmark/scrm-api-contract.yaml`
- 测试：`scripts/check_phase3_scrm_contract.mjs`

**接口：**

- 产出规范化的 `Lead`、`ContactProfile`、`CustomerAssignment`、`Opportunity`、`OpportunityStage` 和 `FollowUp` 模型。
- 产出后续任务共同依赖的状态转换规则和错误码。

- [ ] **步骤 1：编写失败的契约检查**

创建 Node 检查脚本，要求所有可变资源定义 `tenantId`、`version`、`createdAt`、`updatedAt`、权限规则和允许的状态转换。

- [ ] **步骤 2：确认检查失败**

执行：

```bash
node scripts/check_phase3_scrm_contract.mjs
```

预期：失败，因为两个契约文件尚不存在。

- [ ] **步骤 3：编写领域和 API 契约**

明确以下规则：

- 线索状态：`new`、`qualified`、`converted`、`discarded`；
- 客户分配状态：`owned`、`collaborating`、`public_pool`；
- 商机使用可配置阶段，并包含终态 `won` 和 `lost`；
- 跟进记录为不可变事件；
- 版本冲突返回 `409`，数据权限拒绝返回 `403`，非法状态转换返回 `422`。

- [ ] **步骤 4：验证契约**

执行：

```bash
node scripts/check_phase3_scrm_contract.mjs
```

预期：通过，并输出全部资源和状态转换。

- [ ] **步骤 5：提交**

```bash
git add docs/phases/phase-3-yuanhu-benchmark scripts/check_phase3_scrm_contract.mjs
git commit -m "docs: define phase3 SCRM contracts"
```

### 任务 2：增加租户隔离的 SCRM 持久层

**文件：**

- 新建：`deploy/standalone/migrations/0099_scrm_customer_lifecycle.up.sql`
- 新建：`deploy/standalone/migrations/0099_scrm_customer_lifecycle.down.sql`
- 新建：`internal/modules/scrm/domain/customer_lifecycle.go`
- 新建：`internal/modules/scrm/domain/customer_lifecycle_test.go`
- 新建：`internal/modules/scrm/ports/customer_lifecycle_repository.go`
- 新建：`internal/modules/scrm/adapters/mysql/customer_lifecycle_repository.go`
- 新建：`internal/modules/scrm/adapters/mysql/customer_lifecycle_repository_test.go`
- 新建：`internal/modules/scrm/adapters/mysql/customer_lifecycle_repository_integration_test.go`
- 修改：`scripts/smoke_schema_migrate.sh`

**接口：**

- 输入：任务 1 定义的资源和状态转换。
- 产出：`ports` 中面向线索、分配关系、商机、阶段和跟进记录的 repository 接口，以及 `adapters/mysql` 中对应的 MySQL repository 实现。

**MySQL integration contract：**

- `customer_lifecycle_repository_integration_test.go` 必须以 `//go:build integration` build constraint 隔离真实数据库场景；
- `TestSCRMCustomerLifecycleTenantIsolationIntegration` 使用两个确定性租户验证任何 repository 查询、更新和关联读取都不会跨租户；
- `TestSCRMCustomerLifecycleOptimisticLockIntegration` 使用真实 `version` 条件更新验证 stale writer 被拒绝且不会覆盖已提交状态；
- `TestSCRMCustomerLifecyclePublicPoolConcurrentClaimIntegration` 使用多个并发连接竞争同一公海资源，验证恰好一个领取成功且 owner/version 最终一致；
- 三个场景不得 mock SQL、不得在 `MOCHAT_REQUIRE_MYSQL_INTEGRATION=1` 时 skip，并使用独立 fixture/cleanup。

- [ ] **步骤 1：编写 repository 测试**

覆盖：

- 租户隔离；
- 外部业务键重复；
- 乐观版本冲突；
- 公海并发领取；
- 协作人可见范围；
- 商机阶段转换；
- 跟进记录不可变。

- [ ] **步骤 2：确认测试失败**

执行：

```bash
go test ./internal/modules/scrm/domain ./internal/modules/scrm/adapters/mysql -run 'TestSCRM'
MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql -run 'TestSCRMCustomerLifecycle(TenantIsolation|OptimisticLock|PublicPoolConcurrentClaim)Integration'
```

预期：失败，因为 0099 migration、repository ports 和 MySQL adapters 尚不存在。

- [ ] **步骤 3：实现 migration、repository ports 和 MySQL adapters**

数据库表必须使用：

- 显式 `tenant_id`；
- 可空 `corp_id`；
- 整数 `version`；
- 软删除字段；
- 租户范围内唯一业务键；
- 负责人、阶段、状态、下次跟进时间和更新时间索引。

同步修订 `scripts/smoke_schema_migrate.sh`：

- 将 latest migration 和 migration 总数断言从 `0098`/`98` 推进到 `0099`/`99`；
- 在空库 apply、checksum、status、baseline、rollback 和 replay 路径中加入 `0099_scrm_customer_lifecycle`；
- 保留 `0098_scrm_lead_foundation` 的历史 apply/checksum/schema 断言，不得重编号、删除或把 0098 的历史语义改写成 0099；
- rollback latest 时必须证明 0099 的 customer lifecycle 变更被撤销，同时 0098 的 lead foundation 表、migration 记录和 checksum 仍保留；
- replay/reapply 后必须证明 0099 恢复为 latest applied，migration 总数回到 99，且重复 apply 不重放。

- [ ] **步骤 4：验证 repository 测试**

执行：

```bash
go test ./internal/modules/scrm/domain ./internal/modules/scrm/adapters/mysql -run 'TestSCRM'
MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql -run 'TestSCRMCustomerLifecycle(TenantIsolation|OptimisticLock|PublicPoolConcurrentClaim)Integration'
go run ./cmd/mochat-architecture -root .
```

预期：测试和架构边界检查均通过。

- [ ] **步骤 5：验证迁移生命周期**

执行任务 2 同步修订后的完整 lifecycle smoke：

```bash
bash ./scripts/smoke_schema_migrate.sh
```

预期：命令退出码为 0；空库 apply 的 latest/count 为 0099/99；0098 和 0099 均有 64 位 checksum；rollback latest 仅撤销 0099 并保留 0098 历史状态；replay/reapply 恢复 0099/99；重复 apply 不重放且无 checksum 漂移。

- [ ] **步骤 6：提交**

```bash
git add deploy/standalone/migrations/0099_scrm_customer_lifecycle.* internal/modules/scrm/domain/customer_lifecycle* internal/modules/scrm/ports/customer_lifecycle_repository.go internal/modules/scrm/adapters/mysql/customer_lifecycle_repository* scripts/smoke_schema_migrate.sh
git commit -m "feat: add SCRM customer lifecycle persistence"
```

### 任务 3：实现数据权限和 SCRM API

**文件：**

- 新建：`internal/modules/scrm/application/customer_lifecycle_service.go`
- 新建：`internal/modules/scrm/application/customer_lifecycle_service_test.go`
- 新建：`internal/modules/scrm/transport/http/customer_lifecycle_handler.go`
- 新建：`internal/modules/scrm/transport/http/customer_lifecycle_handler_test.go`
- 修改：`internal/modules/scrm/module.go`
- 修改：`internal/modules/scrm/transport/http/routes.go`

**接口：**

- 输入：任务 2 定义的 repository ports，由 application service 编排数据权限、幂等和状态转换。
- 产出：application service，以及由 `transport/http` 暴露的 `/dashboard/scrm/leads`、`/contacts`、`/assignments`、`/opportunities`、`/stages` 和 `/followUps`。

- [ ] **步骤 1：编写失败的 Handler 测试**

覆盖：

- 列表筛选和分页；
- 创建和状态转换；
- 负责人转移；
- 协作人变更；
- 公海原子领取；
- 权限拒绝；
- 幂等重试；
- 版本冲突。

- [ ] **步骤 2：确认 Handler 测试失败**

执行：

```bash
go test ./internal/modules/scrm/application ./internal/modules/scrm/transport/http -run 'TestSCRM'
```

预期：失败，因为 Handler 尚不存在。

- [ ] **步骤 3：实现 Handler 和路由注册**

复用现有用户解析器、企业授权器、租户上下文、JSON 响应结构和审计规范。不得信任只由客户端提供的租户、负责人或数据权限范围。

- [ ] **步骤 4：验证 Handler 测试**

执行：

```bash
go test ./internal/modules/scrm/application ./internal/modules/scrm/transport/http -run 'TestSCRM'
go run ./cmd/mochat-architecture -root .
```

预期：测试和架构边界检查均通过。

- [ ] **步骤 5：刷新前端/API 清单**

执行：

```bash
pnpm refresh:audit
```

预期：新接口进入审计清单，且不存在无法解释的缺口。

- [ ] **步骤 6：提交**

```bash
git add internal/modules/scrm/application/customer_lifecycle_service* internal/modules/scrm/transport/http/customer_lifecycle_handler* internal/modules/scrm/transport/http/routes.go internal/modules/scrm/module.go docs/phases/phase-1-frontend-foundation/audit
git commit -m "feat: expose tenant-scoped SCRM APIs"
```

### 任务 4：实现联系人和线索 React 页面

**文件：**

- 新建：`web/apps/dashboard/src/features/scrm/scrm-api.ts`
- 新建：`web/apps/dashboard/src/features/scrm/scrm-api.test.ts`
- 新建：`web/apps/dashboard/src/features/scrm/contact-page.tsx`
- 新建：`web/apps/dashboard/src/features/scrm/contact-page.test.tsx`
- 新建：`web/apps/dashboard/src/features/scrm/lead-page.tsx`
- 新建：`web/apps/dashboard/src/features/scrm/lead-page.test.tsx`
- 修改：`web/apps/dashboard/src/main.tsx`
- 修改：`web/apps/dashboard/src/pages/dashboard-page-loaders.ts`

**接口：**

- 输入：任务 3 的 API。
- 产出：联系人和线索路由的真实 React 实现。

- [ ] **步骤 1：编写 API 和组件测试**

覆盖：

- 来源、状态、标签和关键词筛选；
- 分页；
- 新增；
- 负责人转移；
- 协作人更新；
- 放弃；
- 转入公海；
- 批量标签更新；
- 加载、空数据、禁止访问、版本冲突和重试状态。

- [ ] **步骤 2：确认测试失败**

执行：

```bash
pnpm --filter @mochat/dashboard test -- src/features/scrm
```

预期：失败，因为功能尚不存在。

- [ ] **步骤 3：实现类型化 API 和页面**

使用现有 Dashboard 权限上下文、企业 Provider、TanStack Query 和 Ant Design。查询键必须包含租户、企业和筛选条件；冲突状态必须提供刷新提示。

- [ ] **步骤 4：替换统一承接页**

在 `main.tsx` 中接入真实页面，移除对应通用 loader，同时保留现有 URL 和查询参数。

- [ ] **步骤 5：验证测试**

执行：

```bash
pnpm --filter @mochat/dashboard test -- src/features/scrm
```

预期：通过。

- [ ] **步骤 6：提交**

```bash
git add web/apps/dashboard/src/features/scrm web/apps/dashboard/src/main.tsx web/apps/dashboard/src/pages/dashboard-page-loaders.ts
git commit -m "feat: add SCRM lead and contact pages"
```

### 任务 5：实现商机与跟进流程

**文件：**

- 新建：`web/apps/dashboard/src/features/scrm/opportunity-page.tsx`
- 新建：`web/apps/dashboard/src/features/scrm/opportunity-page.test.tsx`
- 新建：`web/apps/dashboard/src/features/scrm/follow-up-timeline.tsx`
- 新建：`web/apps/dashboard/src/features/scrm/follow-up-timeline.test.tsx`
- 修改：`web/apps/dashboard/src/features/scrm/scrm-api.ts`

**接口：**

- 输入：商机、阶段和跟进 API。
- 产出：商机列表/编辑器以及不可变的客户跟进时间线。

- [ ] **步骤 1：编写失败的流程测试**

覆盖：

- 阶段、日期和负责人筛选；
- 预计金额和预计成交时间校验；
- 阶段转换；
- `won`/`lost` 终态规则；
- 负责人转移；
- 协作人可见性；
- 跟进记录按时间顺序展示。

- [ ] **步骤 2：确认测试失败**

执行：

```bash
pnpm --filter @mochat/dashboard test -- src/features/scrm/opportunity-page.test.tsx src/features/scrm/follow-up-timeline.test.tsx
```

预期：失败。

- [ ] **步骤 3：实现商机和时间线 UI**

当前阶段和阶段转换操作必须分开展示；进入 `lost` 时必须填写原因。每条跟进记录展示作者、事件时间、下次跟进时间和来源。

- [ ] **步骤 4：验证流程测试**

再次执行步骤 2 的命令。

预期：通过。

- [ ] **步骤 5：提交**

```bash
git add web/apps/dashboard/src/features/scrm
git commit -m "feat: add opportunity and follow-up workflows"
```

### 任务 6：增加业务事件和第一批报表

**文件：**

- 新建：`internal/modules/scrm/domain/metrics.go`
- 新建：`internal/modules/scrm/application/metrics_service.go`
- 新建：`internal/modules/scrm/application/metrics_service_test.go`
- 新建：`internal/modules/scrm/ports/metrics_repository.go`
- 新建：`internal/modules/scrm/adapters/mysql/metrics_repository.go`
- 新建：`internal/modules/scrm/adapters/mysql/metrics_repository_test.go`
- 新建：`internal/modules/scrm/transport/http/metrics_handler.go`
- 新建：`internal/modules/scrm/transport/http/metrics_handler_test.go`
- 修改：`internal/modules/scrm/module.go`
- 修改：`internal/modules/scrm/module_test.go`
- 修改：`internal/modules/scrm/transport/http/routes.go`
- 新建：`web/apps/dashboard/src/features/scrm/scrm-report-page.tsx`
- 新建：`web/apps/dashboard/src/features/scrm/scrm-report-page.test.tsx`
- 新建：`docs/phases/phase-3-yuanhu-benchmark/metric-dictionary.md`

**接口：**

- 输入：不可变 SCRM 生命周期事件。
- 产出：线索到联系人、联系人到商机的漏斗指标，以及明确的指标定义。

- [ ] **步骤 1：编写指标定义和失败测试**

为以下指标定义分母、事件时间、租户时区、去重规则和迟到事件处理：

- 线索数；
- 有效线索数；
- 线索转化数；
- 进行中商机金额；
- 成交金额；
- 阶段转化率。

同时编写模块级路由注册测试，断言 `module.go` 通过 `transport/http/routes.go` 注册 metrics endpoints，且方法、路径和 handler 均与指标契约一致。

- [ ] **步骤 2：确认测试失败**

执行：

```bash
go test ./internal/modules/scrm ./internal/modules/scrm/application ./internal/modules/scrm/adapters/mysql ./internal/modules/scrm/transport/http -run 'TestSCRMMetrics|TestModuleRegistersSCRMMetricsRoutes'
```

预期：失败。

- [ ] **步骤 3：实现聚合 API**

支持日期范围、企业、部门、负责人、来源和阶段筛选；返回汇总、趋势、分布以及可分页的明细引用。

- [ ] **步骤 4：验证后端测试与架构边界**

执行：

```bash
go test ./internal/modules/scrm ./internal/modules/scrm/application ./internal/modules/scrm/adapters/mysql ./internal/modules/scrm/transport/http -run 'TestSCRMMetrics|TestModuleRegistersSCRMMetricsRoutes'
go run ./cmd/mochat-architecture -root .
```

预期：测试和架构边界检查均通过。

- [ ] **步骤 5：实现并测试报表页面**

执行：

```bash
pnpm --filter @mochat/dashboard test -- src/features/scrm/scrm-report-page.test.tsx
```

预期：完成汇总、趋势、分布和明细状态后通过。

- [ ] **步骤 6：提交**

```bash
git add internal/modules/scrm/domain/metrics.go internal/modules/scrm/application/metrics_service* internal/modules/scrm/ports/metrics_repository.go internal/modules/scrm/adapters/mysql/metrics_repository* internal/modules/scrm/transport/http/metrics_handler* internal/modules/scrm/transport/http/routes.go internal/modules/scrm/module.go internal/modules/scrm/module_test.go web/apps/dashboard/src/features/scrm/scrm-report-page* docs/phases/phase-3-yuanhu-benchmark/metric-dictionary.md
git commit -m "feat: add SCRM funnel reporting"
```

### 任务 7：端到端证据和发布门禁

**文件：**

- 新建：`web/e2e/tests/phase3-scrm.spec.ts`
- 新建：`docs/phases/phase-3-yuanhu-benchmark/acceptance.md`
- 修改：`package.json`

**接口：**

- 输入：前六项任务的全部能力。
- 产出：可重复执行的 SCRM 验收证据和明确发布结论。

- [ ] **步骤 1：编写失败的 Playwright 场景**

覆盖：

- 创建并转化线索；
- 联系人负责人和协作人；
- 公海并发领取；
- 商机阶段推进；
- 跟进时间线；
- 权限拒绝；
- 版本冲突刷新；
- 漏斗报表更新。

- [ ] **步骤 2：确认 Playwright 失败**

执行：

```bash
pnpm --filter @mochat/e2e test:e2e -- phase3-scrm.spec.ts
```

预期：在 fixture 和 UI 完成前失败。

- [ ] **步骤 3：增加确定性租户 Fixture**

使用隔离的租户、企业和用户 ID。仅模拟外部企业微信边界；SCRM 行为必须经过真实 Go API 和数据库。

- [ ] **步骤 4：执行全部 Phase 3 门禁**

```bash
go run ./cmd/mochat-architecture -root .
go test ./internal/architecture/... ./internal/app/modules/... ./internal/modules/... ./internal/frontend
go test -race ./internal/modules/...
go test ./...
go vet ./...
go build ./cmd/mochat-go ./cmd/mochat-inventory ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./cmd/mochat-saas-maintenance ./cmd/mochat-architecture
MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql
bash ./scripts/smoke_schema_migrate.sh
pnpm test
pnpm build
pnpm check:audit
pnpm --filter @mochat/e2e test:e2e -- phase3-scrm.spec.ts
docker compose -f deploy/standalone/docker-compose.yml --profile app up -d --build
```

预期：全部命令退出码为 0；应用、MySQL 和 Redis 均为 healthy。MySQL integration 必须使用真实数据库且禁止 skip；本地没有已迁移 DSN 时，使用 `.github/workflows/mysql57-amd64.yml` 的 SCRM MySQL integration atomic step 提供 MySQL 5.7、设置 require 变量并执行该命令。Migration smoke 以 Linux checkout/CI 为权威执行面，并且必须使用任务 2 修订后的 lifecycle：latest/count 为 0099/99，0099 完成 apply/checksum/rollback/replay，同时保留 0098 的历史 migration、checksum 和 schema 语义。

- [ ] **步骤 5：记录验收结果**

记录场景、角色、Fixture、预期结果、实际结果、截图/Trace，以及尚未完成的真实企业微信验证债务。

- [ ] **步骤 6：提交**

```bash
git add web/e2e/tests/phase3-scrm.spec.ts docs/phases/phase-3-yuanhu-benchmark/acceptance.md package.json
git commit -m "test: add phase3 SCRM release gate"
```
