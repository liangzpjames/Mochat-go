# 客户会话菜单优化实施文档

## 1. 交付结论

本轮已完成 `/chat/v2-customer` 客户会话菜单的 Yuanhu 风格工作台改造，并保持现有菜单入口与顶部导航不变。页面现在采用与员工会话菜单一致的信息架构：左侧客户目录、中间会话消息、右侧客户详情/关联会话，支持宽屏三栏、桌面端抽屉和窄屏单栏布局。

实现遵循“只展示 MoChat 真实数据”的约束：目录、消息、详情和关联会话均由客户会话 API 返回；没有后端字段时不伪造业务结果，能力字段仍保留在 API 数据合同中，客户工作区不再渲染能力缺口提示卡，专项边界统一登记到总进度文档。

## 2. 页面与交互范围

| 区域 | 已实现内容 | 数据/行为来源 |
| --- | --- | --- |
| 客户目录 | 搜索、状态/消息类型/日期筛选、固定分页、列表语义、客户名称/头像/最近时间/会话数、整卡选择 | `GET /dashboard/work-message/customer/directory` |
| 会话消息 | 文本、图片、语音、视频、文件、链接预览；未知类型明确提示不支持 | `GET /dashboard/work-message/customer/messages` |
| 客户详情 | 客户资料、资料缺失回退、员工/客户方向、群聊成员身份、消息统计 | `GET /dashboard/work-message/customer/detail` |
| 关联会话 | 客户直聊和客户群会话列表、头像/资料状态、整卡跳转 | `GET /dashboard/work-message/customer/related-conversations` |
| 页面状态 | 加载、空态、错误态、刷新、遮罩关闭、分页和筛选状态保持 | React Query + URL 路由状态 |

客户资料缺失时，后端返回稳定的非空展示名与 `profileStatus=missing/deleted`，前端不会把缺资料误显示为成功加载的空白资料。群聊详情显示真实群成员身份；无成员资料时显示明确的缺口状态。

## 3. 视觉与响应式实现

- 三栏宽屏栅格：目录 `288px`、详情 `360px`、消息区 `minmax(560px, 1fr)`，在 2560×1440 下利用主内容区并避免大块无意义留白。
- 每个栏位独立滚动，消息区不会推动目录或详情；页面高度使用 `100dvh`，兼容移动浏览器地址栏变化。
- 客户详情统计采用四列紧凑卡片；消息类型筛选、关键词和日期控件压缩为低高度筛选区，桌面端不再出现“3 张卡片加 1 张换行卡片”的大块留白。
- 客户页在 `1200px` 以上保持左侧客户列表、中间关联会话、右侧消息详情三栏；`1200–1399px` 使用收缩列宽确保详情操作按钮仍在视口内；`1199px` 以下目录切换为抽屉，`768px` 以下进入单栏模式。
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

`5c36eaf` 路由与权限、`7c5ff27` 客户目录、`2a94bde` 关联会话、`390291a` 关联会话头像与资料状态修复、`b0dc60b` 客户详情、`13f7969` 客户名称回退、`8787fa7` 页面编排、`d2b60ee` 页面测试稳定性、`42eb452` 响应式布局、`9a2ca77` 会话 API 合并冲突修复、`cb2cd80` 消息内容渲染器、`406678c` 操作区按钮、头像回退、空选择缓存和关键词 SQL 修复。

## 6. 测试与构建证据

以下命令均在 `D:\workspace\mochat-go\mochat-go` 执行：

| 验证项 | 命令/结果 |
| --- | --- |
| 客户目标前端测试 | 5 个文件、25 个测试通过 |
| Dashboard 全量前端测试 | `corepack pnpm --filter @mochat/dashboard test`：114 个文件中 113 个完成，701/702 个测试通过；唯一失败为既有轨迹滚动用例的全量时序波动，隔离重跑该文件 4/4 通过 |
| 类型检查 | `corepack pnpm --filter @mochat/dashboard typecheck` 通过 |
| 生产构建 | `corepack pnpm --filter @mochat/dashboard build` 通过；仅有既有的大 chunk 警告 |
| Go 客户目标 | 客户目录 SQL、客户会话 SQL/详情相关目标测试通过 |
| Go 相关包全量 | 工作区并行增量已发现 0144 migration，而既有健康/迁移断言仍期望 0143，导致全量包命令出现基线断言失配；不影响本轮客户目标测试 |
| 启动命令 | `go test ./cmd/mochat-go -count=1` 通过 |
| MariaDB 方言集成 | `TestCustomerDirectoryMariaDBIntegration`、`TestCustomerConversationMariaDBIntegration`、`TestCustomerDetailMariaDBIntegration` 因未设置 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 明确 SKIP；不能记为方言集成通过 |
| 空白检查 | 暂存客户变更 `git diff --cached --check` 通过 |

Dashboard 全量测试期间出现的 `jsdom window.getComputedStyle` stderr 是现有 Ant Design 测试环境警告，不影响退出码；生产构建的 chunk 大小警告也未导致构建失败。

## 7. Docker 验收

已按本地 Compose 项目 `mochat-go-desktop` 仅重建 `app` 服务，未执行 `down --volumes`、卷删除或 MySQL/Redis 重建。最终健康状态：

- `mochat-go-desktop-app-1`：healthy，端口 `18080 -> 8080`。
- `mochat-go-desktop-mysql-1`：healthy，数据卷保留。
- `mochat-go-desktop-redis-1`：healthy，数据卷保留。
- 保留卷：`mochat-go-desktop_app-storage`、`mochat-go-desktop_audit-anchor-storage`、`mochat-go-desktop_mysql-data`、`mochat-go-desktop_redis-data`。

## 8. 浏览器验收

已在本地 Compose 的真实登录态下打开 `http://127.0.0.1:18080/chat/v2-customer`，视口为 `1216×1272`。验收步骤与结果如下：

1. 清除客户选择后，左侧客户目录仍显示真实客户列表；中栏显示中性用户图标和“请选择客户”，右侧显示“请选择会话”，不出现“请”字头像，也不沿用缓存客户资料。
2. 点击“朱晨”后，URL 更新为 `customerId=2009&conversationId=1006:1:2009`；中栏显示朱晨的关联会话，右侧详情正常加载。
3. 详情操作区按钮边界为 `x=970..1154`，均在 `1216px` 视口内；“刷新详情”“关注客户”可见且可点击。
4. 实际点击验证“刷新客户”“刷新会话”“刷新详情”、关注切换和消息内容搜索；关注按钮在“关注客户/已关注”之间切换后恢复原状态，消息搜索将 URL 更新为 `messageKeyword=红包` 并渲染 2 条消息。
5. 客户目录搜索 `朱晨` 正常返回客户列表；浏览器控制台错误日志为空。
6. 最新 app 重建后在嵌入浏览器 `1064×1272` 群聊场景复测：能力缺口提示节点为 0；四张统计卡同一行、高度约 `56px`；筛选区高度约 `84px`；消息类型选项的背景、边框、圆角、内边距与员工会话一致。
7. 点击“文字”后选中态变为蓝色并将 `messageTypes=text` 写入 URL；点击“选择客户”打开窄屏目录抽屉，再选择“陈晓明”后客户标题、选中行和详情请求同步更新；控制台日志为空。

## 9. 已知缺口与后续动作

1. 未配置 `MOCHAT_GO_MYSQL_INTEGRATION_DSN`，因此 MariaDB 方言集成按测试约定明确跳过；浏览器已使用现有真实登录态完成 `1280×720` 关键路径，其他视口可在后续视觉回归中扩展。
2. 未知消息类型仍按真实字段显示“该类型内容暂不支持预览”，不会伪造媒体内容。
3. Vite 对现有主 bundle 给出大于 700 kB 的优化建议；这不影响本轮页面交付，可在独立性能迭代中继续拆分。

## 10. 本次客户会话问题修复

针对审阅反馈补充完成以下修复：

1. 左侧客户目录改为真正的列表结构（`role=list`/`role=listitem`），每行展示客户头像、名称、最近会话时间和直聊/群聊数量；在常见 `1280px` 桌面宽度下保持可见，不再被过早的抽屉断点移出屏幕。
2. 选中客户后，页面等待对应客户和会话模式的数据返回，并自动选择第一条有稳定 `conversationId` 的关联会话，随后更新 URL 并触发右侧客户消息详情请求。
3. 未选择客户时，中间栏头像改为中性 SVG 用户图标；空态仍明确显示“请选择客户”，不再把“请”误当作客户头像。
4. 修正客户关联会话 SQL 对不存在的 `mc_work_room.avatar` 和直聊场景不存在的 `latest.current_member` 的引用：群聊头像回退到归档消息 `target_avatar`，直聊不读取群成员字段。
5. 新增列表语义、时间列、空态图标、“选客户后打开首条会话”以及 SQL 字段兼容性的回归测试，并完成本地浏览器真实点击验收。
6. 复用员工会话工作台的轻量操作区规范：客户搜索改为“搜索”，目录/会话/详情刷新分别明确为“刷新客户”“刷新会话”“刷新详情”，关注切换显示“关注客户/已关注”，消息历史显示“加载更早”；按钮统一使用浅底圆角、内联图标、主次色、焦点和禁用态。
7. 修复客户目录关键词查询的 SQL 投影，确保 `archive_name` 可参与筛选；关键词搜索不再触发 `Unknown column 'archive_name'`。
8. 客户目录和中栏客户资料无头像时统一显示中性用户 SVG 图标；取消客户选择后不再使用 React Query 的旧客户快照填充中间栏。
9. 客户页不再在目录或详情中渲染能力缺口提示卡；能力字段仍随真实 API 保留，后续专项记录统一维护在[总进度文档](../PROJECT_PROGRESS.zh-CN.md)。
10. 客户详情消息类型选择框对齐员工会话样式：隐藏原生复选框，使用轻量矩形选项、统一间距和蓝色选中态，减少筛选区高度并保持可键盘操作。

本次反馈修复的定向回归覆盖 5 个前端文件、25 个测试；Go 侧客户目录 SQL、客户会话 SQL/详情相关目标测试均通过。

## 11. 交付判定

- [x] 客户会话菜单与员工会话菜单保持同一布局语言。
- [x] 客户目录、消息、详情、关联会话使用真实 API 与权限链路。
- [x] 宽屏、桌面、抽屉和单栏响应式 CSS 合同通过。
- [x] 客户目标前端测试、类型检查、生产构建通过；Dashboard 全量回归仅有既有轨迹滚动用例时序波动，隔离重跑通过。
- [x] Go 客户目标测试与启动命令通过；相关包全量断言受工作区并行 0144 migration 基线失配影响，已在测试证据中注明。
- [x] Docker app 已重建，MySQL、Redis 和数据卷保持健康。
- [x] 客户会话操作按钮完成文案、图标、主次层级、焦点和禁用态统一。
- [x] 真实登录态下 `1064×1272`/`1216×1272` 客户选择、会话自动打开、无选择空态、紧凑统计筛选和右侧详情关键路径浏览器验收；详情操作按钮均在视口内。

本轮实施代码、测试、浏览器验收和交付文档均已完成。
