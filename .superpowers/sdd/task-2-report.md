# Phase 3.2 Task 2 报告：统一 Dashboard 外壳和页面视觉组件

## 完成内容

- 保留并纳入既有的品牌标识、固定顶部栏、侧栏层次和卡片表面视觉调整。
- 新增五个可复用类：`.dashboard-page-header`、`.dashboard-filter-bar`、`.dashboard-stat-grid`、`.dashboard-data-card`、`.dashboard-table-actions`。
- 提供 `.dashboard-table-scroll` 及 Ant Design 表格容器横向滚动，移动端将筛选条和操作区折行，并把统计网格收为两列。
- 将 `PageState` 统一为 12px 表面；保留公开接口，补充 loading 的 `aria-busy` 语义和重试按钮样式类。loading、empty、403（forbidden）、409（conflict）及 retry 均由原有 `PageState` 状态接口覆盖。

## TDD 证据

- RED：新增契约测试后，缺少通用类、PageState 12px 圆角及 loading `aria-busy`，共 3 项预期失败。
- GREEN：实现后样式与 PageState 聚焦测试 8/8 通过；brief 指定的三文件聚焦回归 21/21 通过。

## 验证

- `pnpm --filter @mochat/dashboard typecheck` 通过。
- `pnpm --filter @mochat/dashboard test` 通过：55 个测试文件、306 项测试；配置为串行执行，耗时约 126 秒。
- `pnpm --filter @mochat/dashboard build` 通过。
- `git diff --check` 通过。

## 提交范围

仅包含 Task 2 的 Dashboard 源码、测试和本报告；未包含 acceptance 文档或 Phase 3.2 计划文档的既有未提交修改。

## 审查修复（八个真实页面）

- 将 `.dashboard-page-header`、`.dashboard-filter-bar`、`.dashboard-stat-grid`、`.dashboard-data-card`、`.dashboard-table-actions` 按现有结构接入数据概览、全局消息、敏感词、线索、联系人、商机、公海和标签页面；表格页面同时接入 `.dashboard-table-scroll`。
- 数据概览和全局消息的 loading、empty、403、错误和重试已改为 `PageState`；全局消息详情的 loading、404、错误与重试也使用同一组件。其余六页保留并由页面级测试验证现有 `PageState` 重试行为；业务 mutation 冲突提示保持原样。
- `PageState` 支持可选标题、描述和重试文案，并保留既有 `children` 兼容入口，因此保留原有业务提示；移除概览和全局消息的私有 14px 卡片外观，使共享卡片 12px、筛选间距、表格横向滚动与移动端折行在真实 DOM 上生效。

## 本轮 TDD 与验证

- RED：先为八个页面增加渲染级测试，断言共享类与错误后的重试请求；运行聚焦 Vitest 得到 8 个文件、10 项失败，原因是页面未接入共享类或仍使用私有状态容器。
- GREEN：接入后运行 `pnpm --dir web/apps/dashboard exec vitest run src/features/dashboard-overview/dashboard-overview-page.test.tsx src/features/conversation-global/conversation-global-page.test.tsx src/features/sensitive-word/sensitive-word-page.test.tsx src/features/scrm/lead-page.test.tsx src/features/scrm/contact-page.test.tsx src/features/scrm/opportunity-page.test.tsx src/features/scrm/public-pool-page.test.tsx src/features/scrm/tag-page.test.tsx --maxWorkers=1 --minWorkers=1`，8 个文件、29 项通过。
- `pnpm --filter @mochat/dashboard typecheck` 通过。
- Dashboard 全量 Vitest 串行运行通过：55 个文件、313 项测试；首次聚合命令在外部 120 秒时限被终止，随后以扩展时限的直接 Vitest 运行完成，最终耗时 121.40 秒。
- `pnpm --filter @mochat/dashboard build` 通过。
- `git diff --check` 通过。
