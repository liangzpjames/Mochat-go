# AI 设置菜单整体优化：实施与自测报告

日期：2026-08-23

分支：`feat/ai-settings-menu-optimization`

基线：`23c50a80`
范围状态：已完成，可进入代码合并审阅

## 1. 交付结论

本任务依据菜单、路由、页面注册、接口、数据库与权限资源核定后，确认 Dashboard 的“AI 设置”实际包含且仅包含以下两个页面：

- `/ai-setting/ai-knowledge-base`：AI 知识库
- `/ai-setting/agent`：智能体管理

两个页面已经完成统一工作区优化，继续使用现有真实 MySQL 持久化链路，不包含前端静态业务数组、伪成功或假数据。后端同时补齐了严格写入合同、企业/租户隔离、跨实体引用校验、稳定错误码、事务审计和 Agent 页面读取知识库所需的最小 RBAC 依赖。

本次没有扩展文档上传、解析、索引、检索、模型、提示词、工具、发布或真实智能体运行能力。页面显式说明这些能力尚未接入；Phase 7 真实企微会话存档继续保持暂停。

## 2. 设计与计划

- 设计文档：`docs/superpowers/specs/2026-08-23-ai-settings-menu-optimization-design.md`
- 实施计划：`docs/superpowers/plans/2026-08-23-ai-settings-menu-optimization.md`

设计文档记录了现状、圆弧 AI 调研、范围矩阵、数据来源、交互模型、视觉原则、权限/租户/安全边界、响应式策略与验收标准。实施计划按照测试驱动顺序拆分了接口合同、事务审计、RBAC、页面状态、交互、运行时迁移与最终门禁。

## 3. 圆弧 AI 调研结论

使用已登录会话，从 `https://web-ai-analysis-work-wechat.yuanhu.com/#/ai-setting/agent` 开始逐项真实点击，确认参考产品的 AI 设置菜单同样由知识库和智能体两个入口组成，并检查了：

- 页面标题、说明、指标与列表的信息层级；
- 关键词/状态筛选、查询与重置；
- 知识库新增、编辑、删除与管理入口；
- 智能体新增、编辑、状态与知识库关联表单；
- 弹窗/抽屉、关闭、空态及窄屏表现；
- 2560、1280 和 820 宽度下的布局变化。

圆弧 AI 用于确认业务结构与交互顺序。本仓库实现沿用已验收 Dashboard 的导航、顶栏、留白、按钮、筛选卡、表格、确认反馈与宽屏规则，没有复制参考页面在窄屏下的裁切和缩放问题。

## 4. 实施内容

### 4.1 后端合同与安全边界

- 写入请求限制为单个 JSON 对象，最大 1 MiB，拒绝未知字段和尾随 JSON。
- 名称按 Unicode 字符计数，限制为 2–128 字符；说明限制为 512 字符。
- 知识库登记文档数限制为 `0..2147483647`；状态必须显式为 `0` 或 `1`。
- `tenant_id`、`corp_id` 与操作人全部来自登录 Principal；前端传入的 `corpId` 只用于现有查询兼容，不进入严格写入体，也不能覆盖 Principal。
- 智能体关联知识库 ID 去重，并在当前租户、当前企业、未删除记录范围内校验。
- Agent 写入会在同一事务中按稳定顺序锁定关联知识库；知识库删除会先锁定目标记录，再在同一事务中检查引用并软删除，消除“先校验、后写入”产生的并发竞态。
- 被任何未删除智能体引用的知识库禁止删除，返回 HTTP 409、`AI_SETTINGS_KNOWLEDGE_BASE_REFERENCED` 和真实 `referenceCount`。
- 顶层 `null`、数组、标量、未知字段、尾随 JSON 与超过 1 MiB 的请求体均按无效 JSON 拒绝。
- 未找到、校验失败与存储失败使用稳定机器码；响应同时提供 `errorCode`，前端可映射成安全中文提示，不暴露数据库错误。
- 存储中的畸形知识库 ID JSON 不再静默回退，按存储失败处理。

### 4.2 事务审计与迁移

新增迁移：

- `deploy/standalone/migrations/0155_ai_settings_integrity_audit.up.sql`
- `deploy/standalone/migrations/0155_ai_settings_integrity_audit.down.sql`

迁移创建 `mochat_go_ai_settings_audits`，保存租户、企业、操作人、实体类型、实体 ID、动作和变更字段。知识库与智能体的 create/update/delete 均在业务写入的同一事务中记录审计；业务写入或审计任一失败都会整体回滚。

同一迁移为 Agent 页面补充读取知识库列表所需的最小 GET 权限依赖，不授予知识库写权限。

### 4.3 Dashboard 页面

两个页面统一具备：

- 真实指标卡、能力边界提示、查询卡、列表卡与分页；
- 关键词、状态、页码和每页条数写入 URL，刷新后保持；
- 非法 URL 参数规范化为确定的默认状态；
- 新增、编辑、显式启停、删除与成功/失败反馈；
- pending 期间防止重复提交和危险关闭；
- 未保存变更二次确认，支持关闭按钮和 Escape；遮罩点击不会意外关闭编辑器；
- 筛选空态、初始空态、加载错误态与重新加载；
- 首屏加载或失败时指标显示 `—` 而不是伪造 `0`；缓存刷新失败时保留上次成功值并明确标记“上次成功数据，刷新失败”；
- 智能体展示真实知识库名称；遗失关联会明确标记，不伪造名称；
- 编辑既有智能体时可保留已关联的停用知识库，但不能新关联其他停用知识库；
- 宽屏充分利用空间，390 窄屏下指标、筛选与内容按单列可滚动呈现。

共享 `ConfirmAction` 支持异步 pending，`DashboardDialog` 只在实际关闭后恢复触发焦点。全页安全 E2E 动作改为刷新，不会在证据门禁中触发写操作。

### 4.4 权限与页面目录

- 页面总数仍为 53：48 个普通授权页面、5 个超级管理员页面、0 个未映射 API 使用。
- Agent 页面新增 `GET /dashboard/ai-settings/knowledge-bases` 依赖，用于真实关联选择与名称展示。
- 页面级既有权限模型保持兼容；没有在本任务中扩大为新的读写角色体系。

## 5. 路由与接口

| 页面 | 方法 | 接口 | 说明 |
| --- | --- | --- | --- |
| AI 知识库 | GET | `/dashboard/ai-settings/knowledge-bases` | 当前企业列表 |
| AI 知识库 | POST | `/dashboard/ai-settings/knowledge-bases` | 新增 |
| AI 知识库 | PUT | `/dashboard/ai-settings/knowledge-bases/{id}` | 编辑与显式启停 |
| AI 知识库 | DELETE | `/dashboard/ai-settings/knowledge-bases/{id}` | 引用校验后软删除 |
| 智能体管理 | GET | `/dashboard/ai-settings/agents` | 当前企业列表 |
| 智能体管理 | POST | `/dashboard/ai-settings/agents` | 新增并校验知识库 |
| 智能体管理 | PUT | `/dashboard/ai-settings/agents/{id}` | 编辑与显式启停 |
| 智能体管理 | DELETE | `/dashboard/ai-settings/agents/{id}` | 软删除 |

## 6. 自动化验证

所有结果均来自完成实现后的新鲜执行。

| 命令 | 结果 |
| --- | --- |
| `corepack pnpm --filter @mochat/dashboard typecheck` | 通过 |
| `corepack pnpm --filter @mochat/dashboard test` | 通过：140 个测试文件、827 个测试 |
| `corepack pnpm --filter @mochat/api-client test && typecheck && build` | 通过：21 个测试，类型检查与构建通过 |
| `corepack pnpm --filter @mochat/dashboard build` | 通过；仅有既有大 chunk 提示 |
| AI 设置及共享组件定向 ESLint | 通过 |
| `corepack pnpm --filter @mochat/e2e lint` | 通过 |
| `corepack pnpm --filter @mochat/e2e typecheck` | 通过 |
| `go test ./internal/modules/ai-settings/... ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1` | 通过 |
| `go test ./... -count=1 -p 16` | 通过 |
| `corepack pnpm check:yuanhu-benchmark` | 通过：53 页 |
| `corepack pnpm check:phase4-dashboard-page-rbac` | 通过：53/48/5、0 未映射 |
| `corepack pnpm check:dashboard-all-pages-evidence` | 通过：12 个验证器测试 |
| `corepack pnpm check:e2e-dashboard-rbac-fixture` | 通过：3 个测试 |
| `git diff --check` | 最终执行通过，见提交前验证 |

全量 `corepack pnpm --filter @mochat/dashboard lint` 也已执行，但仓库基线仍有 86 个错误，全部位于本任务未修改的会话、风险预警、敏感词等文件。AI 设置、共享 Dialog/ConfirmAction 和本任务 E2E 文件的定向 lint 均通过；本任务没有越界修复这些既有页面。

## 7. Docker、迁移与数据库验证

- 使用项目 `mochat-go-desktop` 和既有 `.env.local`，只重建 app 服务，没有删除或替换数据卷。
- 保留的数据卷：`app-storage`、`audit-anchor-storage`、`mysql-data`、`redis-data`。
- 在 app 容器中执行 `mochat-migrate -action apply -project-root /app`。
- `mochat_go_schema_migrations` 中存在 `0155_ai_settings_integrity_audit`，应用时间为 `2026-08-23 15:25:47`。
- 审计表存在，Agent→知识库 GET 权限映射存在 1 条。
- 最终 app 健康状态为 `healthy`，`GET /health` 返回 200。
- 浏览器验收临时数据最终均已软删除：活动知识库 0、活动智能体 0。
- 本次真实工作流产生 8 条审计：两类实体各 create=1、update=2、delete=1，且均归属于同一真实登录操作人。

## 8. 真实浏览器验收

已登录本地 Dashboard，逐项实际执行：

1. 知识库：新建停用记录、查询停用状态、刷新保持 URL 和记录、编辑名称/说明、显式启用。
2. 智能体：新建并关联上述真实知识库、查询启用状态、刷新保持、编辑、显式停用。
3. 在智能体仍引用知识库时删除知识库，确认得到明确 409 中文反馈并展示“仍被 1 个智能体引用”；目标记录保持存在，未发生删除。
4. 关闭按钮、Escape 都会对脏表单给出二次确认；遮罩点击不会意外关闭；继续编辑与放弃更改均实际执行。
5. 停止 app 容器后刷新智能体页面，确认三个指标保留真实缓存值并全部标记“上次成功数据，刷新失败”，列表显示错误态和“重新加载”；恢复容器后重新加载成功。
6. 删除智能体后再次删除知识库成功；刷新和数据库查询确认无活动验收记录，审计保留。
7. 两个页面均用不存在的关键词验证筛选空态。
8. 验收尺寸覆盖 2560x1440、1366x900、1366x620 和 390x844；窄屏菜单真实打开并导航到智能体页面。

证据索引：`web/e2e/artifacts/ai-settings-menu-optimization/evidence.json`

截图：

- `web/e2e/artifacts/ai-settings-menu-optimization/kb-wide-2560x1440.png`
- `web/e2e/artifacts/ai-settings-menu-optimization/kb-mobile-390x844.png`
- `web/e2e/artifacts/ai-settings-menu-optimization/kb-conflict-desktop-1366x900.png`
- `web/e2e/artifacts/ai-settings-menu-optimization/agent-desktop-1366x900.png`
- `web/e2e/artifacts/ai-settings-menu-optimization/agent-short-1366x620.png`
- `web/e2e/artifacts/ai-settings-menu-optimization/agent-mobile-390x844.png`
- `web/e2e/artifacts/ai-settings-menu-optimization/agent-network-failure-1366x620.png`

## 9. 原子实现提交

从基线到本报告前的实现提交如下（按新到旧）：

- `4799efa fix(ai-settings): keep summary metrics truthful`
- `683045a fix(ai-settings): serialize cross-entity integrity`
- `9fe3828 docs(ai-settings): record implementation evidence`
- `f7cf563 fix(ai-settings): expose stable error codes`
- `8dbf674 fix(ai-settings): align write payload contracts`
- `0e0cf14 fix(ai-settings): canonicalize explicit list params`
- `b111e9e fix(ai-settings): harden pagination and pending contracts`
- `0b2931d feat(ai-settings): optimize configuration workspaces`
- `00330b7 test(ai-settings): strengthen list state contracts`
- `217749e test(ai-settings): define api and list state contracts`
- `53fb213 fix(rbac): authorize agent knowledge base dependency`
- `3d31b15 fix(ai-settings): harden audit transaction contracts`
- `0812907 feat(ai-settings): add transactional configuration audit`
- `e5ceb19 fix(ai-settings): classify configuration validation failures`
- `76d666f fix(ai-settings): harden configuration contracts`
- `a926b5b fix(ai-settings): enforce scoped configuration contracts`
- `0fc206f docs(ai-settings): plan optimization implementation`
- `5bf8e9b docs(ai-settings): define optimization design`

所有后端、RBAC、前端分项均经过独立规格与质量复审。最终分支复审发现并发校验窗口、失败态指标真实性、引用数量和顶层 `null` 合同四项问题后，均已按测试驱动补齐并重新执行全量验证；同一独立复审代理复查后给出 `PASS`，未发现新的 Critical、Important 或 Minor 问题。

## 10. 回滚边界与已知限制

- 前端与后端代码可按提交反向回滚。
- 迁移 down 会删除审计表并移除 Agent→知识库 GET 依赖；执行前必须先导出审计数据。当前未执行 down，保留卷数据库维持已迁移状态。
- 文档、检索和真实智能体运行均不在本任务范围内；页面对这些能力保持诚实受限，不做伪成功。
- 没有执行任何外部 Provider 写操作，也不对未接入能力宣称通过。
- 并发正确性已有 SQL 锁顺序合同测试覆盖，但尚未增加高并发真实 MariaDB 竞争型集成测试；这是非阻塞的后续加固项。
- 仓库全量 Dashboard lint 的 86 个既有错误仍需由对应页面任务处理。
- `docs:check` 仍会命中仓库既有的 Phase 3 benchmark README 断链；该链接不属于本任务文档或改动范围。
- 没有修改总进度台账、旧 `web/saas-admin/`、`.workbuddy/`、`tmp/` 或其他任务成果。
