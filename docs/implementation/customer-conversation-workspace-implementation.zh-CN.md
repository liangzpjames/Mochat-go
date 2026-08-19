# 客户会话菜单优化实施文档

## 1. 交付结论

本轮已完成 `/chat/v2-customer` 客户会话菜单的 Yuanhu 风格工作台改造，并保持现有菜单入口与顶部导航不变。页面现在采用与员工会话菜单一致的信息架构：左侧客户目录、中间会话消息、右侧客户详情/关联会话，支持宽屏三栏、桌面端抽屉和窄屏单栏布局。

实现遵循“只展示 MoChat 真实数据”的约束：目录、消息、详情和关联会话均由客户会话 API 返回；没有后端字段的能力显示“未接入/暂无数据”，不使用前端静态业务样例填充。

## 2. 页面与交互范围

| 区域 | 已实现内容 | 数据/行为来源 |
| --- | --- | --- |
| 客户目录 | 搜索、状态/消息类型/日期筛选、固定分页、整卡选择 | `GET /dashboard/work-message/customer/directory` |
| 会话消息 | 文本、图片、语音、视频、文件、链接预览；未知类型明确提示不支持 | `GET /dashboard/work-message/customer/messages` |
| 客户详情 | 客户资料、资料缺失回退、员工/客户方向、群聊成员身份、消息统计 | `GET /dashboard/work-message/customer/detail` |
| 关联会话 | 客户直聊和客户群会话列表、头像/资料状态、整卡跳转 | `GET /dashboard/work-message/customer/related-conversations` |
| 页面状态 | 加载、空态、错误态、刷新、遮罩关闭、分页和筛选状态保持 | React Query + URL 路由状态 |

客户资料缺失时，后端返回稳定的非空展示名与 `profileStatus=missing/deleted`，前端不会把缺资料误显示为成功加载的空白资料。群聊详情显示真实群成员身份；无成员资料时显示明确的缺口状态。

## 3. 视觉与响应式实现

- 三栏宽屏栅格：目录 `288px`、详情 `360px`、消息区 `minmax(560px, 1fr)`，在 2560×1440 下利用主内容区并避免大块无意义留白。
- 每个栏位独立滚动，消息区不会推动目录或详情；页面高度使用 `100dvh`，兼容移动浏览器地址栏变化。
- `1599px` 以下详情切换抽屉，`1199px` 以下目录切换抽屉，`768px` 以下进入单栏模式。
- 加入 hover、选中、键盘焦点、禁用和错误状态，避免仅靠颜色表达状态。
- 保留员工会话菜单既有布局节奏，客户页面通过独立的 `CustomerConversationPage` 编排，避免复制员工业务数据。

主要前端文件：

- `web/apps/dashboard/src/features/conversation-global/customer-conversation-page.tsx`
- `web/apps/dashboard/src/features/conversation-global/customer-conversation-directory.tsx`
- `web/apps/dashboard/src/features/conversation-global/customer-related-conversations.tsx`
- `web/apps/dashboard/src/features/conversation-global/customer-conversation-detail.tsx`
- `web/apps/dashboard/src/features/conversation-global/conversation-message-content.tsx`
- `web/apps/dashboard/src/styles/index.css`

## 4. 后端、权限与迁移

- 新增客户会话 handler、目录查询、消息查询、详情查询和关联会话查询。
- 单聊严格约束 `toUserID == customerID`；未知会话类型返回 404。
- 群聊成员查询同时约束 `corp_id` 与 room/contact 关系，保留历史退群成员的可追溯性。
- 受限员工先校验 `employeeId` 是否属于授权集合，再复用员工会话的消息方向、群成员、统计、游标和筛选算法。
- 新增客户工作台路由与菜单资源，迁移编号为 `0142`；健康检查发布期望同步更新为 `142`。
- 客户目录去重、状态模式、直聊/群聊筛选和同企业约束均在 store 层测试覆盖。

## 5. 主要提交

本轮客户页面相关提交如下（并行开发的员工/群会话增量未在本清单中重复列出）：

`5c36eaf` 路由与权限、`7c5ff27` 客户目录、`2a94bde` 关联会话、`390291a` 关联会话头像与资料状态修复、`b0dc60b` 客户详情、`13f7969` 客户名称回退、`8787fa7` 页面编排、`d2b60ee` 页面测试稳定性、`42eb452` 响应式布局、`9a2ca77` 会话 API 合并冲突修复、`cb2cd80` 消息内容渲染器。

## 6. 测试与构建证据

以下命令均在 `D:\workspace\mochat-go\mochat-go` 执行：

| 验证项 | 命令/结果 |
| --- | --- |
| 客户目标前端测试 | 8 个文件、57 个测试通过 |
| Dashboard 全量前端测试 | `corepack pnpm --filter @mochat/dashboard test`：112 个文件、690 个测试通过 |
| 类型检查 | `corepack pnpm --filter @mochat/dashboard typecheck` 通过 |
| 生产构建 | `corepack pnpm --filter @mochat/dashboard build` 通过；仅有既有的大 chunk 警告 |
| Go 目标包 | `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -run 'Test.*(WorkMessageCustomer\|CustomerConversation\|CustomerDirectory\|CustomerDetail\|DashboardPageCatalog\|0142)' -count=1` 通过 |
| Go 相关包全量 | `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1` 通过 |
| 启动命令 | `go test ./cmd/mochat-go -count=1` 通过 |
| MariaDB 方言集成 | `TestCustomerDirectoryMariaDBIntegration`、`TestCustomerConversationMariaDBIntegration`、`TestCustomerDetailMariaDBIntegration` 因未设置 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 明确 SKIP；不能记为方言集成通过 |
| 空白检查 | `git diff --check` 通过 |

Dashboard 全量测试期间出现的 `jsdom window.getComputedStyle` stderr 是现有 Ant Design 测试环境警告，不影响退出码；生产构建的 chunk 大小警告也未导致构建失败。

## 7. Docker 验收

已按本地 Compose 项目 `mochat-go-desktop` 仅重建 `app` 服务，未执行 `down --volumes`、卷删除或 MySQL/Redis 重建。最终健康状态：

- `mochat-go-desktop-app-1`：healthy，端口 `18080 -> 8080`。
- `mochat-go-desktop-mysql-1`：healthy，数据卷保留。
- `mochat-go-desktop-redis-1`：healthy，数据卷保留。
- 保留卷：`mochat-go-desktop_app-storage`、`mochat-go-desktop_audit-anchor-storage`、`mochat-go-desktop_mysql-data`、`mochat-go-desktop_redis-data`。

## 8. 浏览器验收边界

已通过本地浏览器打开真实路由 `http://127.0.0.1:18080/chat/v2-customer`。服务和静态资源可访问，但该运行实例的真实路由在未认证时会转到 `/login?returnTo=%2Fchat%2Fv2-customer`。本轮没有在浏览器中输入手机号或密码，也没有读取或传输凭据，因此无法在无授权登录态下对真实数据列表、详情和四种视口截图做最终视觉确认。

这不是页面代码失败：自动化测试已覆盖 DOM、交互和响应式 CSS 合同；剩余动作是使用受控测试账号登录后，在 `2560×1440`、`1440×1000`、`1024×900`、`390×844` 四个视口复核真实数据与控制台网络错误。登录后应重点检查：目录整卡点击、筛选/分页、消息区独立滚动、详情抽屉遮罩关闭以及刷新后的 URL 状态。

## 9. 已知缺口与后续动作

1. 未提供可在浏览器中使用的受控测试账号，因此 MariaDB 集成和登录态浏览器验收保留为环境前置项。
2. 未知消息类型仍按真实字段显示“该类型内容暂不支持预览”，不会伪造媒体内容。
3. Vite 对现有主 bundle 给出大于 700 kB 的优化建议；这不影响本轮页面交付，可在独立性能迭代中继续拆分。

## 10. 交付判定

- [x] 客户会话菜单与员工会话菜单保持同一布局语言。
- [x] 客户目录、消息、详情、关联会话使用真实 API 与权限链路。
- [x] 宽屏、桌面、抽屉和单栏响应式 CSS 合同通过。
- [x] 前端全量测试、类型检查、生产构建通过。
- [x] Go 相关包与启动命令通过。
- [x] Docker app 已重建，MySQL、Redis 和数据卷保持健康。
- [ ] 受控登录态下的四视口真实数据浏览器验收（等待测试账号/登录态）。

除最后一项环境前置的浏览器登录验收外，本轮实施代码、测试和交付文档已完成。
