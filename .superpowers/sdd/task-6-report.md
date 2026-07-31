# Task 6 Report: P0 全局消息查询与详情闭环

## 状态

- 状态：完成
- 实现提交：`4ed4293bf6a436ed17569b838b9539932cd30f04`
- 分支：`phase3/yuanhu-benchmark-phase1`
- 页面：`/chat/v2-all`
- 数据来源：真实 `work_message_0` 至 `work_message_9` 归档表；生产代码中没有 fixture/mock 数据路径

## 实现结果

- 新增 `ConversationGlobalApi.search(input)` 与 `ConversationGlobalApi.detail(id)`，请求始终携带当前登录会话的企业上下文。
- 复用已有 `/dashboard/workMessage/toUsers` handler，以 `view=global` 提供全局会话摘要；详情通过 `/dashboard/workMessage/detail` 复用已有 `workMessageIndex` handler，没有复制同义查询端点。
- 查询响应使用显式分页结构 `{ list, total, page, pageSize }`。
- 服务端在访问存储前校验请求企业与认证企业一致，并复用既有数据权限解析器将允许查看的员工 ID 下推到 SQL；空权限范围生成 `1=0`。
- 支持关键词、员工、客户、群聊、日期范围、页码和每页数量筛选；无效或非正数 ID/分页参数返回 400，`pageSize` 最大 100，客户与群聊筛选互斥。
- 十张归档表的每个 UNION 分支都带企业条件；会话按员工、目标类型和目标 ID 分组，避免不同员工的同一联系人会话被错误合并。
- 消息排序同时使用消息时间、全局序号、表序号和表内 ID，详情返回最近 200 条并在 Go 层恢复时间正序；响应包含 `messageTotal`、`truncated` 与 `window: "latest"`。
- 详情 ID 优先使用 `msgid`，其次使用全局序号，最后使用分表序号与表内 ID，避免跨表碰撞。
- 不存在、跨企业或超出数据权限的详情统一返回 404。
- 页面筛选与分页写入 URL；包含加载、空数据、403、可重试列表错误、可重试详情错误、详情 404、详情截断提示和越界页码恢复状态。
- 群聊归档没有持久化实际群成员身份时，入站消息显示中性“群成员”，不再把群名称冒充发送者。
- 保留旧版必须指定员工的查询语义；只有显式全局视图允许跨员工检索。

## 修改文件

- `internal/dashboard/auto_tag_dashboard.go`
- `internal/dashboard/auto_tag_dashboard_test.go`
- `internal/server/server.go`
- `internal/server/server_test.go`
- `internal/store/auto_tag.go`
- `internal/store/work_message_test.go`
- `web/apps/dashboard/src/benchmark/page-registry.test.tsx`
- `web/apps/dashboard/src/benchmark/page-registry.tsx`
- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`
- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`
- `web/apps/dashboard/src/features/conversation-global/conversation-global-page.test.tsx`
- `web/apps/dashboard/src/features/conversation-global/conversation-global-page.tsx`
- `web/apps/dashboard/src/main.tsx`
- `web/apps/dashboard/src/styles/index.css`

## TDD 证据

### RED

- 后端初始测试暴露：旧式嵌套分页响应、跨企业参数被接受、RBAC 员工范围未下推、详情仍返回旧列表结构、缺失详情没有 404。
- 存储测试先因缺少全局筛选、员工维度分组、员工关键词、稳定跨分表消息 ID、最近 200 条窗口与严格分页限制而失败。
- 前端初始测试因全局会话 API 和页面模块尚不存在而失败。
- 后续边界回归测试分别确认：
  - 无效数字参数会静默扩大查询范围；
  - 详情 API 会复用旧企业上下文；
  - 群聊入站消息错误使用群名称作为发送者；
  - URL 中过期的越界页码会落入无恢复入口的空状态。

### GREEN

- 上述用例均在最小实现后转绿。
- 定向回归：
  - `go test ./internal/store ./internal/dashboard ./internal/server -run 'Conversation|Chat|WorkMessage' -count=1`：3 个包全部通过。
  - `go test ./internal/store -run WorkMessageUnionSQLScopes -count=1`：通过。
  - Dashboard 全量测试中的 `conversation-global-page.test.tsx` 9 项、`conversation-global-api.test.ts` 4 项全部通过。

## 最终验证

- `go test ./internal/... -count=1`
  - 退出码 0；所有 `internal` 包通过，无失败。
- `pnpm --filter @mochat/dashboard test -- src/features/conversation-global`
  - 退出码 0；43 个测试文件通过，283 项测试通过。
- `pnpm --filter @mochat/dashboard typecheck`
  - 退出码 0；`tsc --noEmit -p tsconfig.json` 无错误。
- `pnpm --filter @mochat/dashboard exec eslint src/features/conversation-global src/benchmark/page-registry.tsx src/benchmark/page-registry.test.tsx src/main.tsx`
  - 退出码 0；无告警或错误。
- `git diff --check`
  - 退出码 0。

## 独立审查与自审

- 独立代码审查确认：端点复用、旧行为兼容、企业/RBAC SQL 约束、跨分表稳定 ID 与排序、最近窗口、URL 状态以及无生产 mock 的总体设计成立。
- 审查提出的群聊发送者误标与越界页码死路已通过新增失败测试后修复。
- 审查发现原 `task-6-report.md` 是无关的 Phase 2.2 SCRM 报告；本文件已完整替换。
- 自审未发现尚未处理的 Critical 或 Important 问题；认证企业校验发生在存储调用前，存储查询也在每个归档表分支重复企业约束。

## 未解决疑问与关注项

- 当前归档模型不保存群聊入站消息的实际群成员身份，因此只能诚实显示“群成员”。若产品需要真实姓名和头像，需要后续扩展采集与存储模型。
- 本任务没有连接真实 MariaDB 执行契约测试或 `EXPLAIN`；十表 UNION、关键词 LIKE 与排序在大数据量下的执行计划仍建议在预发布数据集评估。
- `go run ./cmd/mochat-architecture -root .` 当前失败于既有基线：`internal/store/mysql.go` 为 841529 字节，超过 840395 字节限制；该文件在本任务当前提交与基线提交中的大小相同，本任务未修改它。
