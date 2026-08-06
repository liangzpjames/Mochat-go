# 开发会话 A 任务：企业设置 5 页 + AI 设置 2 页 + AI 洞察 5 页（共 12 页）

> 本文件是唯一任务说明。先完整读完本文件，再开始工作。工作目录：`D:\workspace\mochat-go\mochat-go`（git 根目录）。

## 0. 目标与验收标准

把以下 12 条路由从 `placeholder`/`demo` 推到产品交付口径：`implementation=native`、`backend=ready`、`acceptance=integration-passed` 或 `e2e-passed`（manifest 由主任务更新，你不要改 manifest，但要按此口径实现）。禁止 fixture/demo/placeholder/静态成功；Provider 缺失时用结构化 `limitations` 呈现，禁止伪造 0/空成功。

## 1. 范围（12 条路由）

### 企业设置（5）
- `/company-setting/website` 企业信息：展示 + 编辑（复用旧接口 `/dashboard/corp/*`）
- `/company-setting/staff` 员工权限：列表、启用/停用、角色分配（复用 `/dashboard/user/*`、`/dashboard/role/*`）
- `/setting/role` 角色管理：角色 CRUD + 菜单/权限勾选（复用 `/dashboard/role/*`、`/dashboard/menu/*`）
- `/setting/additional` 附加权限维护（复用 `/dashboard/menu/*` 或补充）
- `/setting/authorization` 授权管理：应用/能力开关（复用 `/dashboard/menu/*` 或补充）

### AI 设置（2）
- `/ai-setting/ai-knowledge-base` AI 知识库 CRUD（新建模块）
- `/ai-setting/agent` 智能体管理 CRUD（新建模块）

### AI 洞察（5）
- `/ai-insight/session-analysis` 会话分析：替换现有 DemoPage 为真实页面
- `/ai-insight/smart-analysis` 智能分析（真实骨架 + 后端合同；能力缺失时 `limitations`）
- `/ai-insight/emotion` 情绪识别（同上）
- `/ai-insight/employee-score` 员工评分（同上）
- `/ai-insight/communication-keyword` 沟通关键词（同上）

## 2. 必读文件（完整读完后开始）

1. `docs/phases/phase-3-dashboard/debt-clearance/2026-08-07-debt-clearance-design.zh-CN.md`（设计+门禁，重点第 4、8 节）
2. `web/apps/dashboard/src/benchmark/manifest.json`（只读，禁止修改）
3. `web/apps/dashboard/src/benchmark/page-registry.tsx`（路由注册方式）
4. `web/apps/dashboard/src/main.tsx`（API 注入方式）
5. `internal/modules/scrm/module.go`、`internal/modules/scrm/transport/http/routes.go`、`internal/app/bootstrap/scrm.go`（新后端模块模式范例；搜索 `RegisterSCRM` 的调用位置照抄接线）
6. `web/apps/dashboard/src/features/phase35/customer-report-page.tsx` 与 `web/apps/dashboard/src/styles/index.css`（Phase 3.5 卡片/按钮样式体系，必须沿用：主按钮 `#536bf4` 白字 8px 圆角、次按钮白底 `#59647a` 字、小号翻页按钮）

## 3. 关键复用（不要重复造轮子）

- 企业设置已有 React API/页面可复用：`features/role/role-api.ts` + `role-page.tsx` + `role-permission-page.tsx`、`features/user-admin/user-admin-api.ts` + `user-admin-page.tsx`、`features/menu-admin/menu-admin-api.ts` + `menu-admin-page.tsx`、`features/corp/corp-admin-api.ts` + `corp-page.tsx`，对应旧后端 `/dashboard/role/*`、`/dashboard/user/*`、`/dashboard/menu/*`、`/dashboard/corp/*`。新路径上的 native 页面优先复用这些 API 与类型；确认旧接口是否满足，缺口再按新模块模式补充。
- 新后端模块参照 scrm 模式：`internal/modules/ai-settings`（AI 知识库 + 智能体 CRUD）、`internal/modules/ai-insight`（5 页合同）。新模块禁止 import `internal/dashboard`；遵循 domain/application/ports/adapters 分层（架构审计会检查依赖规则）。
- 迁移：你独占 `deploy/standalone/migrations` 的 `0124–0126`（up/down 成对，命名风格参照 `0123_phase35_order_collation_align`；先读该目录 README 约定）。另一个开发会话用 `0127+`，不要占用。

## 4. 路由注册（只有你被允许改这两个文件）

- `page-registry.tsx`：为你的 12 条路由加 import 与 `createBenchmarkP0Pages` 条目，保持现有 `...(api === undefined ? {} : {...})` 可选 API 模式；`/ai-insight/session-analysis` 必须在 P0 层覆盖 `benchmarkP1Pages` 里的 DemoPage 兜底。
- `main.tsx`：创建你的新 API 实例并传入 `createBenchmarkP0Pages`。

## 5. 测试（RED→GREEN）

- 每页 Vitest 至少覆盖：加载/数据/空/错误/受限 5 态；Go 侧覆盖权限、校验、幂等/审计。
- 定向验证只允许跑：`go test ./internal/modules/ai-settings/... ./internal/modules/ai-insight/...`、`pnpm --filter @mochat/dashboard exec vitest run <你的 feature 文件>`、`pnpm --filter @mochat/dashboard typecheck`。
- 禁止跑全量 `go test ./...` 与 `pnpm build`（主任务统一跑，避免并发冲突）。

## 6. 禁止事项

- 禁止修改 `web/apps/dashboard/src/benchmark/manifest.json`、`internal/server/compat_manifest_embedded.json`（主任务同步）。
- 禁止提交 commit、禁止重建/部署 Docker、禁止删除数据卷、禁止重启容器。
- 禁止改动 `features/phase33`、`features/conversation-global` 以及 `internal/store` 下会话/风险相关文件（另一个开发会话负责）。
- 禁止伪造数据/静态成功。

## 7. 最终回报（中文，写清楚）

- 改动/新增文件清单；
- 迁移文件清单；
- `page-registry.tsx` 与 `main.tsx` 新增片段（便于主任务复核）；
- 每页 API 合同（方法+路径+主要字段）与数据流；
- 定向测试结果（命令+输出摘要）；
- 遗留缺口与假设。
