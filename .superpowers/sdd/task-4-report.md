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
