# 开发会话 B 任务：会话 8 页 + 风险预警 6 页（共 14 页）

> 本文件是唯一任务说明。先完整读完本文件，再开始工作。工作目录：`D:\workspace\mochat-go\mochat-go`（git 根目录）。

## 0. 目标与验收标准

把以下 14 条路由从 `demo`/`placeholder` 推到产品交付口径：`implementation=native`、`backend=ready`、`acceptance=integration-passed` 或 `e2e-passed`（manifest 由主任务更新，你不要改 manifest，但按此口径实现）。组件已注册，核心工作是：逐页复用审计、补齐 Go 合同、真实数据流；Provider 缺失时结构化 `limitations`，禁止伪造 0/空成功。

## 1. 范围（14 条路由）

### 会话（8）
- `/chat/v2-staff` 员工会话
- `/chat/v2-customer` 客户会话
- `/chat/v2-group` 群聊会话
- `/chat/trajectory` 会话轨迹
- `/chat/export` 会话导出
- `/chat/resign-staff` 离职员工
- `/chat/refuse-archive` 拒绝存档
- `/customer/inheritance` 客户继承

`/chat/file-audio` 保持阻塞，禁止改动。

### 风险预警（6）
- `/ai-insight/v2/risk` 风险行为
- `/ai-insight/v2/timeout` 超时预警
- `/ai-insight/v2/customer-loss` 客户流失
- `/ai-insight/v2/message-intercept` 消息拦截
- `/ai-insight/v2/keyword-library` 关键词库
- `/ai-insight/v2/silent-customer` 沉默客户

## 2. 必读文件（完整读完后开始）

1. `docs/phases/phase-3-dashboard/debt-clearance/2026-08-07-debt-clearance-design.zh-CN.md`（重点第 4 节统一要求、第 8 节审阅结论与 8.5 数据流工作流）
2. `docs/phases/phase-3-dashboard/phase-3.3/reports/phase3.3-function-matrix.md`（每页核心流程/依赖/权限/持久化/风险，逐页核对）
3. `docs/phases/phase-3-dashboard/phase-3.3/acceptance/` 下全部验收文档（了解哪些 Provider/规则/页面已完成）
4. `web/apps/dashboard/src/features/phase33/**` 与 `features/conversation-global/**`（现有组件与 API 客户端，逐个读懂）
5. `internal/store/work_message*.go`、`internal/store/timeout_warning*.go`、`internal/dashboard/timeout_warning*.go`、`internal/server/server.go` 中相关路由段（用 `rg` 找 `/dashboard/workMessage`、timeout、risk、intercept 等）

## 3. 执行要点

- 逐页审计：页面读取的 API 与后端路由是否存在、返回结构与前端类型是否一致、tenant/corp/员工数据范围限制是否到位、写操作是否有权限/校验/审计。
- 补齐 Go 合同：缺的接口在既有 store/handler 上完成；禁止新增 `internal/dashboard` 文件（如确需新端点，用新模块模式 `internal/modules/<name>` 并参照 `internal/modules/scrm` 的接线方式：`internal/app/bootstrap` 新增 Register 函数并在服务启动处调用，搜索 `RegisterSCRM` 照抄模式）。
- 会话归档统一合同（`/workMessage/*` 或既有归档接口）；风险事件写入真实业务事件（消息拦截命中、超时触发、关键词库版本发布等），页面回读真实数据。
- Provider 缺失：页面结构化 `limitations`，不得伪造 0/空成功（沿用 Phase 3.5 空态与受限态呈现）。
- 样式：沿用现有页面样式；如可行统一到 phase35 卡片/按钮体系（主按钮 `#536bf4` 白字 8px 圆角、次按钮白底 `#59647a` 字、小号翻页按钮）。只允许在你自己 feature 目录内或 `index.css` 末尾追加最小样式，不大改共享样式。
- 迁移：你独占 `deploy/standalone/migrations` 的 `0127–0129`（up/down 成对，先读该目录 README 约定）。另一个开发会话用 `0124–0126`，不要占用。

## 4. 测试（RED→GREEN）

- 每页 Vitest 至少覆盖：加载/数据/空/错误/受限 5 态；Go store/handler 补单测。
- 定向验证只允许跑：`go test ./internal/store/... ./internal/dashboard/...`、`pnpm --filter @mochat/dashboard exec vitest run <你的 feature 文件>`、`pnpm --filter @mochat/dashboard typecheck`。
- 禁止跑全量 `go test ./...` 与 `pnpm build`（主任务统一跑，避免并发冲突）。

## 5. 禁止事项

- 禁止修改 `web/apps/dashboard/src/benchmark/manifest.json`、`internal/server/compat_manifest_embedded.json`、`web/apps/dashboard/src/benchmark/page-registry.tsx`、`web/apps/dashboard/src/main.tsx`（另一开发会话负责新页注册）。
- 禁止提交 commit、禁止重建/部署 Docker、禁止删除数据卷、禁止重启容器。
- 禁止改动 `features/ai-settings`、`features/ai-insight`、`features/company-settings`、`internal/modules/ai-*`（另一开发会话负责）。
- 禁止伪造数据/静态成功。

## 6. 最终回报（中文，写清楚）

- 每页“页面 → API → 后端路由 → 真实表/事件”对照表；
- 改动文件清单；迁移文件清单；
- 每个 Provider 状态（已就绪/受限/缺失）与 `limitations` 呈现位置；
- 定向测试结果（命令+输出摘要）；
- 遗留缺口与假设。
