# Task 4A：全局消息前端合同报告

状态：DONE_WITH_CONCERNS

## 完成内容

- `ConversationSearch` 以 `keyword`、`conversationType`、`employeeIds`、`startAt`、`endAt`、`page` 和 `pageSize` 定义并序列化全局消息列表请求；详情只传真实归档消息 `id`，两端均不再由客户端传入企业范围。
- 页面从 URL 恢复筛选与分页状态，支持查询、重置、页码切换和会话详情抽屉；员工 ID 被规范为去空白、去重的重复查询参数。
- 列表与详情均保留 `PageState` 的可重试错误处理；详情的 404、普通 403 和归档未授权 40301 分别呈现受控状态。归档未授权提示现已在列表和详情一致显示。
- 群聊列表或筛选会展示成员身份能力限制提示，并继续复用 Task 2 的页面头部、筛选、数据卡片、表格滚动和分页视觉类。

## TDD 记录

- RED：新增“详情请求的归档未授权与列表保持一致”用例后，页面错误显示为“无权读取会话详情”，目标“当前企业未开通会话内容存档”不存在。
- GREEN：抽取 40301 识别逻辑并用于列表和详情；聚焦页面 Vitest 通过 14/14。
- 类型检查 RED：群聊能力用例中数组下标在严格类型下可能为 `undefined`，导致 `ConversationSummary` 推断失败。
- GREEN：使用已类型化列表映射构造群聊数据；Dashboard typecheck 通过。

## 验证

- `pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts src/features/conversation-global/conversation-global-page.test.tsx`：最终聚焦回归 17/17 通过。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- `pnpm --filter @mochat/dashboard build`：通过，Vite 构建 1674 个模块。
- 未执行浏览器验收，也未更新功能矩阵，符合 Task 4A 范围。

## 关注点

- 全量 `pnpm --filter @mochat/dashboard test` 在环境 64 秒命令时限内被终止，未输出失败用例；本任务以四个目标文件的聚焦 Vitest、typecheck 和 build 作为验证证据。
- 本提交只包含本任务指定的四个 `conversation-global` 文件及本报告；工作树中已有的后端、入口、样式、验收和计划文档改动均未暂存或覆盖。

---

# Task 4B：全局消息后端闭环报告

状态：DONE_WITH_BROWSER_EVIDENCE_DEFERRED

## 完成内容

- 全局消息列表支持 `keyword`、`conversationType`、重复或逗号分隔的 `employeeIds`、`startAt`、`endAt`、`page`、`pageSize`；日期使用 `[startAt, endAt + 1 day)` 半开区间，页大小上限为 100。
- 服务端仅从认证会话取得 tenant/corp；员工筛选与 RBAC 数据范围取交集，受限空范围在 SQL 层直接返回零。列表和详情复用相同员工范围，详情以真实归档消息 ID 定位并再次校验权限。
- 列表 `/workMessage/toUsers` 与详情 `/workMessage/detail` 统一使用 `/dashboard/workMessage/toUsers#get` RBAC key，并有精确回归证明两条请求链路一致；普通 403、存档未授权 40301 与越权/不存在 404 仍分别保留。
- 未开通会话存档返回 `40301`，普通 RBAC 拒绝返回 403，不存在或越权详情统一返回 404；列表和详情均不会跨租户、跨企业泄露数据。
- 十张 `mc_work_message_*` 表继续使用固定表名和参数化值组成 `UNION ALL`；关键词中的 `%`、`_`、反斜杠按字面量转义，避免把用户输入扩展为通配模式。
- 真实 MariaDB fixture 覆盖双租户、三企业、存档开关、员工范围、空范围、对象类型、关键词、时间边界、分页和三种归档 ID 查询。性能门禁逐表验证既有 `corp_employee`、`corp_msgid`、`corp_seq` 索引列前缀，并要求列表/详情 EXPLAIN 每个分支估算行数不超过 512；非 `ALL` 计划必须实际使用索引。
- 未保留 0106 索引迁移：实测新增复合索引存在时，优化器仍可能选择既有索引；小型 fixture 上选择 257 行 `ALL` 也是合理成本决策。生产查询已把企业、员工、类型、时间和归档 ID 谓词下推到分表，现有必要索引齐全，没有证据证明新增迁移必要。
- `main.tsx` 的单参数 API 构造接线是 Task 4A 合同所需；`styles/index.css` 仅补齐对象类型选择、重置按钮和群聊能力提示样式，两者均保留。`access-context.test.tsx` 换行噪音已清除。
- 功能矩阵 `/chat/v2-all` 已更新前端、API、权限、持久化和自动化测试证据；浏览器证据仍留给 Task 11，未伪造。

## 2026-08-02 全局消息体验优化

- 本轮仅优化 `/chat/v2-all`，未修改员工会话、客户会话、群聊会话等其他会话页面。
- 对照圆弧页面补充真实查询摘要、员工/客户/群聊快捷筛选和当前查询刷新入口；保留关键词、员工、日期、分页、详情及 URL 恢复能力。
- 未开通会话内容存档时继续展示真实授权状态，不使用演示数字填充圆弧统计卡片。
- 修复二级页面刷新后当前菜单分组折叠的问题：当前路由所属分组首次加载和跨组导航时自动展开，用户仍可手动折叠。
- 自动化证据：全局消息单页 16 项测试通过，菜单与页面组合测试 30 项通过，Dashboard 全量测试 371 项通过；类型检查与生产构建通过。
- 浏览器证据：部署后刷新 `/chat/v2-all`，“会话”分组保持展开，“全局消息”选中，快捷筛选、查询表单和归档未授权状态可见。

## 验证结果

- `go test -count=1 ./internal/dashboard -run 'WorkMessage|workMessage'`：通过。
- `go test -count=1 ./internal/store -run 'WorkMessage|workMessage'`：通过。
- `go test -count=1 ./internal/migration ./internal/server`：通过。
- `node scripts/check_phase3_2_mysql_integration.mjs`：通过；隔离 MariaDB 中 SCRM MySQL 与 store 集成包均通过，容器和数据卷已清理。
- `pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts src/features/conversation-global/conversation-global-page.test.tsx`：2 个文件、17/17 通过。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- `pnpm --filter @mochat/dashboard build`：通过，Vite 构建 1674 个模块。
- 控制器复跑 Dashboard 全量测试：55 个文件、332/332 个测试通过。
- `go vet ./internal/dashboard ./internal/store ./internal/migration ./internal/server`：通过。
- `git diff --check`：通过（仅 Git 的 LF/CRLF 提示，无空白错误）。

## 已知失败与遗留风险

- 一次扩大范围的 Go 联跑暴露三个非 Task 4 失败：Dashboard 系统健康测试仍期望迁移数 104、续费任务使用当前时间导致快照漂移、共享桌面数据库统计信息令 Task 3 的 CorpData EXPLAIN 选择 `ALL`。Task 4 聚焦包、server、migration 和隔离 MariaDB 均已通过。
- 未执行浏览器验收；当前没有可用本地登录凭据，浏览器搜索、重置、分页、详情和错误态证据仍由 Task 11 补齐。
