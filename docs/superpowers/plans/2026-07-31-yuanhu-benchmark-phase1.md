# 圆弧 AI 对标第一阶段实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 MoChat Go 基础上完成圆弧 AI 的全站菜单与全局框架复刻、全部登记路由可达，以及数据概览、全局消息、敏感词三个首批 P0 页面。

**Architecture:** 以版本化 `benchmark-manifest` 作为菜单、路由和实施状态的唯一来源，在现有 `dashboard` 应用中重建圆弧风格的全局 shell。页面通过 `page-registry` 分为 P0 真实页面、P1 高保真演示页面和 P2 标准建设中页面；P0 页面各自拥有 feature module、API、权限、测试和持久化边界。

**Tech Stack:** React 19、React Router 7、TypeScript 5.9、Ant Design 6、TanStack Query 5、Vitest、Testing Library、Go HTTP API、现有 MySQL/Redis 存储与仓库脚本。

## Global Constraints

- 目标页面以圆弧 AI 的已观测视觉、交互和业务结果为基准；现有界面不构成兼容约束。
- 现有 `auth`、`api-client`、`routing` 等基础设施在满足目标行为时优先复用；页面结构和样式允许替换。
- 菜单和页面等级以 `benchmark-manifest` 为唯一事实来源，禁止维护重复手工清单。
- 第一阶段强制桌面视觉基线为 `1440x1000`；移动端只要求可访问、不白屏和无破坏性横向溢出。
- P0 不得混用 mock 数据；P1 使用独立 fixture/mock 层，升级 P0 时必须移除生产路径中的 mock。
- 所有 P0 查询和命令必须执行租户、企业和权限校验。
- 未观测到的圆弧行为必须记录为“未观测”，不得由实现者猜测补齐。
- 每个任务完成时更新中央清单、运行任务规定的测试，并创建独立提交。

---

## 文件地图

### 新建

- `docs/benchmark/yuanhu/manifest.json`：圆弧菜单、路由、等级、截图版本和状态清单。
- `docs/benchmark/yuanhu/pages/index/spec.md` 等页面档案：单页视觉、交互、状态和复用评估。
- `docs/benchmark/yuanhu/pages/index/states.json` 等页面状态文件：可复现的页面状态与观测标记。
- `web/apps/dashboard/src/benchmark/benchmark-manifest.ts`：构建期校验后的清单导出。
- `web/apps/dashboard/src/benchmark/page-registry.tsx`：路由到 P0/P1/P2 页面实现的映射。
- `web/apps/dashboard/src/benchmark/placeholder-page.tsx`：统一 P2 建设中页面。
- `web/apps/dashboard/src/benchmark/demo-fixtures.ts`：只供 P1 的脱敏、稳定演示数据。
- `web/apps/dashboard/src/features/dashboard-overview/*`：数据概览 P0 页面、API 和测试。
- `web/apps/dashboard/src/features/conversation-global/*`：全局消息 P0 页面、API 和测试。
- `web/apps/dashboard/src/features/sensitive-word/*`：敏感词 P0 页面、API 和测试。
- `web/e2e/benchmark/yuanhu-phase1.spec.ts`：跨路由菜单、刷新、状态和三条 P0 流程。
- `scripts/check_yuanhu_benchmark_manifest.mjs`：清单与页面注册一致性检查。

### 修改

- `web/apps/dashboard/src/app/router.tsx`：接入 page registry 和 P2 fallback。
- `web/apps/dashboard/src/main.tsx`：加载圆弧清单、页面注册和首批 P0 API。
- `web/apps/dashboard/src/layout/dashboard-layout.tsx`：替换为圆弧风格全局 shell 和分组菜单。
- `web/apps/dashboard/src/styles/index.css`：圆弧设计令牌、布局和响应式规则。
- `web/apps/dashboard/src/layout/dashboard-layout.test.tsx`、`web/apps/dashboard/src/app/phase2-completion.test.ts`：扩展导航覆盖测试。
- `package.json`：加入清单检查和第一阶段验收脚本。
- 对应 Go 路由、服务和 store 文件：仅在已有 API 无法支持 P0 契约时新增，不改动无关模块。

## Task 1: 建立圆弧页面档案与清单校验

**Files:**

- Create: `docs/benchmark/yuanhu/manifest.json`
- Create: `docs/benchmark/yuanhu/pages/index/spec.md`
- Create: `docs/benchmark/yuanhu/pages/index/states.json`
- Create: `scripts/check_yuanhu_benchmark_manifest.mjs`
- Modify: `package.json`
- Test: `scripts/check_yuanhu_benchmark_manifest.test.mjs`

**Interfaces:**

- Consumes: 已观测圆弧侧栏路由和现有 `web/apps/dashboard/src/migration-routes.json`。
- Produces: `manifest.json` 中稳定的 `groups`, `pages`, `level`, `status`, `screenshotVersion` 字段；检查脚本以进程码 0/1 表示通过或失败。

- [ ] **Step 1: 写清单失败测试**

在 `scripts/check_yuanhu_benchmark_manifest.test.mjs` 中读取 JSON，断言八个分组存在、`/index` 存在、每个页面有唯一 `path` 和合法 `level`（`P0|P1|P2`），并断言重复路由被拒绝。

- [ ] **Step 2: 运行失败测试**

运行：`node --test scripts/check_yuanhu_benchmark_manifest.test.mjs`

预期：FAIL，原因是清单和检查脚本尚不存在。

- [ ] **Step 3: 创建页面清单和首批页面档案**

将已观测到的约 56 个入口登记为页面对象；首批 P0 为 `/index`、`/chat/v2-all`、`/ai-insight/v2/sensitive-word`，首批 P1 使用设计文档中的九个页面，其余登记为 P2。页面档案必须写明当前未观测的深层状态，不写推测行为。

- [ ] **Step 4: 实现检查脚本**

脚本读取 `manifest.json`，验证分组、路径唯一性、等级枚举、必需字段和截图版本，并输出缺失路径；不要自动修改清单。

- [ ] **Step 5: 运行通过测试并接入 pnpm**

运行：`node --test scripts/check_yuanhu_benchmark_manifest.test.mjs`，预期 PASS；然后在 `package.json` 增加 `check:yuanhu-benchmark` 并运行：`pnpm check:yuanhu-benchmark`，预期输出清单通过。

- [ ] **Step 6: 提交**

```bash
git add docs/benchmark/yuanhu scripts/check_yuanhu_benchmark.mjs scripts/check_yuanhu_benchmark.test.mjs package.json
git commit -m "docs: add yuanhu benchmark manifest"
```

## Task 2: 重建圆弧全局 shell、设计令牌和分组导航

**Files:**

- Modify: `web/apps/dashboard/src/layout/dashboard-layout.tsx`
- Modify: `web/apps/dashboard/src/layout/dashboard-layout.test.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`
- Create: `web/apps/dashboard/src/layout/yuanhu-navigation.ts`
- Create: `web/apps/dashboard/src/layout/yuanhu-navigation.test.ts`

**Interfaces:**

- Consumes: `benchmark-manifest.ts` 的分组页面数据、现有 `AccessContext` 和 `useDashboardSessionActions`。
- Produces: `buildYuanhuNavigation(access, manifest)`；返回带分组、子项、当前状态和权限过滤结果的只读结构，供 `DashboardLayout` 渲染。

- [ ] **Step 1: 写导航纯函数失败测试**

测试空权限、重复路径、分组顺序和当前路径高亮：

```ts
expect(buildYuanhuNavigation(access, manifest).map((g) => g.title)).toEqual([
  '会话', '风险预警', 'AI 洞察', '营销工具', 'SCRM', '数据报表', 'AI 设置', '企业设置',
]);
expect(buildYuanhuNavigation(access, manifest)[0].items[0].activePath).toBe('/chat/v2-all');
```

- [ ] **Step 2: 运行测试确认失败**

运行：`pnpm --filter @mochat/dashboard test -- src/layout/yuanhu-navigation.test.ts`

预期：FAIL，原因是 `buildYuanhuNavigation` 未定义。

- [ ] **Step 3: 实现导航数据和 shell**

在 `yuanhu-navigation.ts` 中从清单构建分组树并按 `allowedRoutes` 过滤；在 `dashboard-layout.tsx` 中实现 logo、顶部搜索、任务入口、企业信息、分组展开/收起、当前项高亮和移动端可滚动侧栏。保留现有登出动作的真实行为。

- [ ] **Step 4: 写布局可访问性测试**

扩展 `dashboard-layout.test.tsx`，断言 `nav[aria-label="主菜单"]`、八个分组标题、当前链接、顶部搜索框和退出按钮均存在；无权限时只显示统一空状态。

- [ ] **Step 5: 实现圆弧样式**

在 `index.css` 中定义颜色、字号、间距、边框、阴影、固定 header、独立滚动侧栏和内容区；桌面基线使用 `1440x1000`，窄屏下禁止整体横向溢出。

- [ ] **Step 6: 验证并提交**

运行：`pnpm --filter @mochat/dashboard test -- src/layout/dashboard-layout.test.tsx src/layout/yuanhu-navigation.test.ts` 和 `pnpm --filter @mochat/dashboard typecheck`，预期全部 PASS。

```bash
git add web/apps/dashboard/src/layout web/apps/dashboard/src/styles/index.css
git commit -m "feat: rebuild yuanhu dashboard shell"
```

## Task 3: 接入 page registry，保证全部路由可达

**Files:**

- Create: `web/apps/dashboard/src/benchmark/benchmark-manifest.ts`
- Create: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Create: `web/apps/dashboard/src/benchmark/placeholder-page.tsx`
- Create: `web/apps/dashboard/src/benchmark/page-registry.test.tsx`
- Modify: `web/apps/dashboard/src/app/router.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/app/phase2-completion.test.ts`

**Interfaces:**

- Consumes: JSON manifest from Task 1 and existing React route elements.
- Produces: `createPageRegistry({ manifest, p0Pages, p1Pages })` and a deterministic element for every registered path.

- [ ] **Step 1: 写 registry 失败测试**

断言 P0 优先于 P1、未提供实现的 P2 返回 `PlaceholderPage`、所有 manifest path 都有 element，并且未知路径仍进入 `NotFoundPage`。

- [ ] **Step 2: 运行失败测试**

运行：`pnpm --filter @mochat/dashboard test -- src/benchmark/page-registry.test.tsx`

预期：FAIL，原因是 registry 文件不存在。

- [ ] **Step 3: 实现 registry 和 P2 页面**

`PlaceholderPage` 显示圆弧风格的标题、面包屑、建设中说明和返回入口，不显示内部 P0/P1 等级；registry 按清单生成 route elements，并将既有可复用 React 页面作为已实现元素传入。

- [ ] **Step 4: 修改 router/main 接入 registry**

将旧的 `reactPages` 直接展开逻辑替换为 registry 输出；保留登录、权限 loader、错误页和 catch-all。所有圆弧菜单路由必须在 `knownRoutes` 中。

- [ ] **Step 5: 运行全路由测试**

运行：`pnpm --filter @mochat/dashboard test -- src/benchmark/page-registry.test.tsx src/app/phase2-completion.test.ts`，预期 PASS；再运行 `pnpm --filter @mochat/dashboard build`，预期构建成功。

- [ ] **Step 6: 提交**

```bash
git add web/apps/dashboard/src/benchmark web/apps/dashboard/src/app/router.tsx web/apps/dashboard/src/main.tsx web/apps/dashboard/src/app/phase2-completion.test.ts
git commit -m "feat: make yuanhu routes reachable"
```

## Task 4: 建立 P1 高保真演示页面层

**Files:**

- Create: `web/apps/dashboard/src/benchmark/demo-fixtures.ts`
- Create: `web/apps/dashboard/src/benchmark/demo-page.tsx`
- Create: `web/apps/dashboard/src/benchmark/demo-page.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**

- Consumes: Task 3 registry、页面档案中的已观测字段和状态。
- Produces: `DemoPage` 支持标题、筛选栏、状态标签、表格、分页、详情抽屉和稳定 fixtures；fixture 不进入 API client 或生产 P0 分支。

- [ ] **Step 1: 写 P1 交互失败测试**

断言搜索会过滤 fixture、分页改变当前页、点击唯一详情按钮打开抽屉、空结果显示空状态，且刷新后不声称数据已持久化。

- [ ] **Step 2: 运行失败测试**

运行：`pnpm --filter @mochat/dashboard test -- src/benchmark/demo-page.test.tsx`

预期：FAIL，原因是 `DemoPage` 未定义。

- [ ] **Step 3: 实现通用高保真演示页**

使用共享 UI 组件实现圆弧样式的页面头部、筛选、表格、分页、详情抽屉和空状态；每个 P1 路由只提供字段配置和 fixture，不复制页面骨架。

- [ ] **Step 4: 注册首批 P1 页面**

将员工会话、客户会话、群聊会话、风险行为、超时预警、会话分析、渠道活码、联系人和客户群接入 registry；每个页面引用自己的页面档案。

- [ ] **Step 5: 验证并提交**

运行：`pnpm --filter @mochat/dashboard test -- src/benchmark/demo-page.test.tsx src/benchmark/page-registry.test.tsx` 和 `pnpm --filter @mochat/dashboard lint`，预期 PASS。

```bash
git add web/apps/dashboard/src/benchmark
git commit -m "feat: add yuanhu high fidelity demo pages"
```

## Task 5: 实现 P0 数据概览真实闭环

**Files:**

- Create: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.ts`
- Create: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx`
- Create: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.test.ts`
- Create: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `internal/server/server.go`（注册或分发概览 handler）
- Modify: `internal/store/mysql.go`（复用或补充统计查询；仅在契约测试证明缺失时修改）
- Test: `internal/server/server_test.go` 与对应 `internal/store/*_test.go`

**Interfaces:**

- Consumes: 当前企业上下文、真实 API client 和圆弧数据概览页面档案。
- Produces: `DashboardOverviewApi.load(input: { corpId: string; from: string; to: string }): Promise<DashboardOverview>`；页面展示真实统计卡和趋势数据，并有加载、空和错误状态。

- [ ] **Step 1: 先搜索已有后端能力并写契约测试**

搜索：`rg -n "corpData|statistics|dashboard|overview" internal web/apps/dashboard/src`。复用已有统计服务时为其补充 endpoint contract test；缺失时先在 Go 测试中定义响应字段、租户过滤和日期边界。

- [ ] **Step 2: 运行契约测试确认失败**

运行：`go test ./internal/app/... ./internal/store/... -run 'Overview|CorpData|Statistics' -count=1`

预期：缺失能力对应测试 FAIL；已有能力对应测试 PASS，不得重复创建同义 endpoint。

- [ ] **Step 3: 实现最小真实 API 与存储查询**

实现按当前租户和企业过滤的统计查询；返回稳定的 `cards`, `trend`, `updatedAt` 字段；日期范围采用明确的本地时区边界；未授权企业返回 403，空数据返回 200 加空数组。

- [ ] **Step 4: 写前端 API 和页面测试**

断言日期筛选序列化、403 显示无权限、200 空数组显示空状态、刷新重新请求真实 client，并断言卡片和趋势值来自响应而非 fixture。

- [ ] **Step 5: 实现页面并接入 P0 registry**

以圆弧截图复刻统计卡、趋势图、筛选区、更新时间和错误反馈；通过 `DashboardOverviewApi` 注入 API，禁止组件内直接拼 URL。

- [ ] **Step 6: 运行验证并提交**

运行：`go test ./internal/... -count=1`、`pnpm --filter @mochat/dashboard test -- src/features/dashboard-overview`、`pnpm --filter @mochat/dashboard build`。

```bash
git add internal web/apps/dashboard/src/features/dashboard-overview web/apps/dashboard/src/main.tsx web/apps/dashboard/src/benchmark/page-registry.tsx
git commit -m "feat: add real dashboard overview"
```

## Task 6: 实现 P0 全局消息查询与详情闭环

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `internal/server/server.go`（注册或分发 `workMessage` handler）
- Modify: `internal/store/mysql.go` 或 `internal/store/work_message_archive_sync.go`（先搜索已有会话查询实现）
- Test: `internal/server/server_test.go` 与对应 `internal/store/*_test.go`

**Interfaces:**

- Consumes: 页面档案中的消息筛选、分页、会话列表和详情抽屉行为；当前企业上下文。
- Produces: `ConversationGlobalApi.search(input: ConversationSearch): Promise<ConversationPage>` 和 `ConversationGlobalApi.detail(id: string): Promise<ConversationDetail>`。

- [ ] **Step 1: 写后端和 API 失败测试**

覆盖关键字、员工、客户、群聊、日期、分页、空结果、租户隔离和不存在详情的 404；请求必须携带当前企业上下文。

- [ ] **Step 2: 运行失败测试**

运行：`go test ./internal/... -run 'Conversation|Chat' -count=1` 与 `pnpm --filter @mochat/dashboard test -- src/features/conversation-global/conversation-global-api.test.ts`，预期新契约部分 FAIL。

- [ ] **Step 3: 实现查询 API**

采用明确的分页响应 `{ list, total, page, pageSize }`；服务层先执行租户、企业和权限检查，再查询消息和会话摘要；详情不返回不属于当前企业的数据。

- [ ] **Step 4: 实现页面交互**

复刻圆弧筛选栏、列表列、分页、空状态和详情抽屉；筛选条件写入 URL，打开详情失败时显示可重试错误，不丢失当前筛选。

- [ ] **Step 5: 运行验证并提交**

运行：`go test ./internal/... -count=1`、`pnpm --filter @mochat/dashboard test -- src/features/conversation-global`、`pnpm --filter @mochat/dashboard typecheck`。

```bash
git add internal web/apps/dashboard/src/features/conversation-global web/apps/dashboard/src/main.tsx web/apps/dashboard/src/benchmark/page-registry.tsx
git commit -m "feat: add real global conversation search"
```

## Task 7: 实现 P0 敏感词配置与命中查询闭环

**Files:**

- Create: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-api.ts`
- Create: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.tsx`
- Create: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-api.test.ts`
- Create: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `internal/server/server.go`（复用现有 sensitive-word handler 注册点）
- Modify: `internal/store/mysql.go` 与 `internal/store/sensitive_word_monitor.go`（复用已有词库与命中查询）
- Test: `internal/server/server_test.go` 与对应 `internal/store/*_test.go`

**Interfaces:**

- Consumes: 敏感词页面档案中的词库列表、启停、添加/编辑、命中记录和权限行为。
- Produces: `SensitiveWordApi.list`, `create`, `update`, `setEnabled` 和 `matches`，所有写操作返回新的版本或更新时间。

- [ ] **Step 1: 写失败测试**

覆盖词条唯一性、空白和长度校验、启停权限、并发版本冲突、命中记录分页和租户隔离；前端断言成功 toast、表单错误和重复提交禁用。

- [ ] **Step 2: 运行失败测试**

运行：`go test ./internal/... -run 'Sensitive|Keyword' -count=1` 与 `pnpm --filter @mochat/dashboard test -- src/features/sensitive-word`，预期新契约部分 FAIL。

- [ ] **Step 3: 实现存储、服务和 API**

敏感词写操作使用唯一约束和版本校验；冲突返回 409；列表和命中记录始终按当前租户、企业和权限过滤；审计记录包含操作者、动作、目标和结果。

- [ ] **Step 4: 实现页面和状态**

复刻圆弧表格、搜索、启停开关、编辑弹窗、命中详情和分页；加载、空、错误、冲突和无权限状态必须独立可见。

- [ ] **Step 5: 运行验证并提交**

运行：`go test ./internal/... -count=1`、`pnpm --filter @mochat/dashboard test -- src/features/sensitive-word`、`pnpm --filter @mochat/dashboard lint`。

```bash
git add internal web/apps/dashboard/src/features/sensitive-word web/apps/dashboard/src/main.tsx web/apps/dashboard/src/benchmark/page-registry.tsx
git commit -m "feat: add real sensitive word management"
```

## Task 8: 建立第一阶段浏览器流程与视觉回归门槛

**Files:**

- Create: `web/e2e/benchmark/yuanhu-phase1.spec.ts`
- Create: `docs/benchmark/yuanhu/acceptance/phase1.md`
- Modify: `package.json`
- Modify: `scripts/check_yuanhu_benchmark_manifest.mjs`
- Modify: `web/apps/dashboard/src/app/phase2-completion.test.ts`

**Interfaces:**

- Consumes: Task 1-7 的清单、shell、P1 页面和三个 P0 页面。
- Produces: 可重复运行的桌面端导航、刷新、P0 核心流程和截图验收；失败时明确输出 route、状态和截图路径。

- [ ] **Step 1: 写浏览器验收失败用例**

覆盖登录后访问 `/index`、八个分组首项、随机 P1、随机 P2、浏览器刷新、前进后退、全局消息筛选、敏感词新增启停和概览日期筛选；每一步检查可见标题和无 console error。

- [ ] **Step 2: 运行并记录失败基线**

运行：`pnpm --filter @mochat/e2e test -- web/e2e/benchmark/yuanhu-phase1.spec.ts`，将失败原因记录到 `docs/benchmark/yuanhu/acceptance/phase1.md`，不通过截图的页面不得标为完成。

- [ ] **Step 3: 补齐截图与视口配置**

固定 `1440x1000` 视口、统一脱敏 fixture 和截图目录；比较布局、导航高亮、筛选栏、表格、弹窗、抽屉和空/错误状态。

- [ ] **Step 4: 接入一键验收命令**

在根 `package.json` 增加 `check:yuanhu-phase1`，依次执行清单检查、dashboard typecheck/test/build 和 e2e；任一步失败即返回非零。

- [ ] **Step 5: 运行最终门槛并提交**

运行：`pnpm check:yuanhu-phase1`，预期清单、单元、后端和浏览器流程全部 PASS，验收文档列出实际截图和测试命令。

```bash
git add web/e2e/benchmark docs/benchmark/yuanhu/acceptance package.json scripts web/apps/dashboard/src/app/phase2-completion.test.ts
git commit -m "test: add yuanhu phase1 acceptance gate"
```

## 计划自检

- 规格覆盖：清单、复用决策、全局 shell、P0/P1/P2、页面采集、数据流、异常、AI 长任务、测试和阶段验收分别由 Task 1-8 覆盖。
- 占位符扫描：计划中没有 `TBD`、`TODO`、尖括号路径或“以后补充”步骤；Task 5-7 已明确现有服务器和存储文件，是否新增文件只能在契约测试证明缺失后按现有项目命名规则决定。
- 类型一致性：Task 1 输出 manifest；Task 2 使用 manifest 构造 navigation；Task 3 以 manifest 建立 registry；Task 4-7 向 registry 注入实现；Task 8 验收全部前置产物。
- 数据真实性：Task 4 的 fixture 只绑定 P1；Task 5-7 的 API 契约、后端测试和页面测试明确禁止生产路径 mock。
- 范围边界：本计划只覆盖第一阶段导航、P1 演示和三个 P0 页面；其余 P0 页面继续沿用同一页面档案、registry 和单页闭环模板，不在本计划中隐式扩张。
