# 员工使用的移动侧边栏整体优化设计

## 1. 文档结论

本设计只覆盖企业员工在企业微信中使用的 `web/apps/sidebar`，保留既有 12 条 URL、`/sidebar-app` 部署前缀、Sidebar 员工 JWT、现有 Go/PHP 兼容 API 和企业微信 JS-SDK 安全语义。设计基线为 `origin/main@b9a47cab45ec872bc81e61a20b06a8a3529311b2`。

采用“Sidebar 业务域真实纵切面 + 无业务共享移动基础”的方案：客户、备注、标签、画像、素材、个人 SOP、群 SOP、批量加好友分别拥有与用途相符的页面和状态；`@mochat/mobile-foundation` 只承载壳层、状态、卡片、导航和通用交互，不承载 Sidebar 会话或业务数据。截图没有覆盖、仓库也没有真实契约的统计、订单、AI 会话、客服状态等能力不实现、不伪造。

## 2. 产品目标、使用场景与非目标

### 2.1 产品目标

- 员工从单聊或群聊工具栏进入后，能快速确认当前客户/群、完成资料维护、发送素材和处理 SOP。
- 12 条历史路由全部可达，不再以同一种“模块待迁移”占位卡片冒充业务页。
- 真实 API 有数据时显示可信内容；无数据、无权限、断网、超时和参数缺失时给出诚实且可恢复的状态。
- iOS、Android 企业微信 WebView 在 320px 至 430px 宽度、刘海/底部安全区、软键盘和长内容场景下保持可操作。
- 登录、回调、过期会话和兼容授权入口继续 fail-closed，不因视觉优化削弱鉴权。

### 2.2 员工高频场景

1. 单聊侧边栏：确认客户身份、备注、标签、所在群、归属员工、画像和互动轨迹。
2. 客户维护：修改备注、追加企业标签、编辑画像字段并返回客户详情。
3. 会话触达：搜索/筛选素材，选择真实素材并通过企业微信 `sendChatMessage` 发送。
4. 任务跟进：查看个人客户 SOP 内容并复制文本；查看客户群 SOP、打开群聊并标记完成。
5. 批量加好友：按状态查看管理员分配的号码、复制号码并调用 `navigateToAddCustomer`。
6. 身份恢复：登录参数校验、OAuth 回调、401 清理 Sidebar 会话后重新授权、兼容 `/codeAuth` 安全恢复。

### 2.3 非目标

- 不修改 `web/apps/operation`、Dashboard、SaaS Admin、Phase 7 或真实会话存档能力。
- 不复制参考产品的品牌、Logo、机器人、人物插画、AI 会话、行为预警、订单和经营统计。
- 不新增前端随机数据、固定成功、纯内存“已保存”或浏览器刷新后丢失的生产状态。
- 不重构 Dashboard API，不把 Sidebar 员工 JWT 替换为 Dashboard 登录态。
- 不修改 `docs/PROJECT_PROGRESS.zh-CN.md`。

## 3. 参考图逐图解析与页面映射

参考图位于 `D:\workspace\mochat-go\docs\移动端参考页面`，已逐张通过本地图片查看能力检查，而非根据文件名推断。

### 3.1 `2e66797253a09c2a453eddd76fa10f68.jpg`

- 页面/状态：底部“我的”选中态；顶部是宿主返回、关闭、标题和更多菜单。
- 信息层级：员工头像与企业/部门资料卡为第一层；客服状态为第二层；权限、企业设置、客服、清缓存为分组列表；底部三栏导航常驻。
- 控件与节奏：白色 20px 左右圆角卡片、约 16px 页面边距、卡片内 16–20px、列表行约 56–64px、细分隔线、右侧箭头；蓝绿色仅用于状态与选中态。
- 安全区：顶部系统状态栏和宿主导航不由 H5 重绘；底部导航之下留系统手势条空间。
- 项目映射：`/` 员工工作台和三栏导航的视觉参考。由于仓库无员工资料/客服状态 API，首页只显示真实路由入口与当前上下文，不显示截图中的虚假个人资料或状态。

### 3.2 `7837dae832938f84613d03cc55e2c50b.jpg`

- 页面/状态：底部“会话”选中态；上方有浅蓝横幅，主体是“行为预警”和“会话存档”卡片/宫格。
- 信息层级：横幅说明产品价值；一级区块标题配时间筛选；蓝色强调卡承载主指标；白卡承载次指标；四宫格作为高频入口。
- 控件与颜色：蓝色渐变强调、白色圆角面板、图标底座、胶囊描边按钮；主要数字大于标签和说明。
- 项目映射：只提取“会话类任务优先、强调卡 + 列表/宫格”的层级，用于 `/contactSop`、`/roomSop`。不实现行为预警、超时预警和会话存档。

### 3.3 `b8e1cff0d6eeaeb79c6ebeb7173af11d.jpg`

- 页面/状态：底部“客户”选中态；横幅下是“数据增长”指标卡、客户域五宫格、线索 Tab 和悬浮新增按钮。
- 信息层级：范围/时间筛选位于标题右侧；2×2 指标卡中首卡用蓝色强调；业务入口独立白卡；内容区用 Tab 切换。
- 交互：卡片和宫格是入口，不把整个页面做成单一列表；悬浮按钮避开底部导航与安全区。
- 项目映射：`/contact` 客户摘要、快捷操作和内容 Tab 的参考；只显示 API 返回的客户资料、轨迹和画像，不显示增长/商机/订单统计。

### 3.4 `cb8f4a742f5f46ff33a3011fa7f8647b.jpg`

- 页面/状态：客户列表空态；顶部包含返回、搜索、归属筛选、横向分类 Tab、数量和筛选；主区显示大面积空状态；底部“客户”选中。
- 信息层级：搜索和筛选固定在列表之前；Tab 可横向收缩但不造成文档级横向溢出；空态图形居中、文案简短；新增按钮位于右下安全区上方。
- 项目映射：`/medium` 的搜索/类型/分组/空态和 `/contactBatchAdd` 的状态筛选/空态。当前 12 路由没有客户列表路由，因此不新增第 13 条 URL。

### 3.5 共同视觉原则

- 页面底色采用低饱和浅蓝，业务内容使用白色圆角面板，避免重边框和桌面 Dashboard 风格。
- 页面水平边距 16px，区块间距 16px，卡内 12–16px；控件最小触控区 44×44px。
- 主色 `#1677ff`，强调渐变 `#5f80ff → #1688ff`；绿色仅表示成功，橙色表示提醒，红色表示破坏性/错误。
- 主标题 20px/600–700，区块标题 17–18px/600，正文 14–16px，辅助文字 12–14px；长名称允许换行或两行截断。
- H5 不重复绘制企业微信宿主的返回/关闭/更多栏；业务子页内部只在需要保存/取消时提供轻量操作栏。

## 4. 12 路由现状、差距与数据来源矩阵

| 路由 | 已有功能 | 占位/缺失 | 参考图对应 | 本次优化 | 真实数据来源 |
| --- | --- | --- | --- | --- | --- |
| `/` | 鉴权后的工作台和 11 个路由入口 | 所有入口同图标，缺少业务分组与上下文提示 | 图 1“我的”、图 3业务宫格 | 客户维护/触达/任务三组入口；只读说明；无假统计 | 路由注册表；不请求业务统计 |
| `/auth` | 解析后端 Base64 state、写 Sidebar cookie/sessionStorage、安全 target | 错误码说明和恢复路径较弱 | 图 1的分组状态语言 | 加载、成功跳转、失败原因、重新授权；保持安全校验 | `/sidebar/agent/auth` 回调参数 |
| `/codeAuth` | 只有“待迁移”占位 | 旧入口会解析 `callValues`，但旧链路签发 Dashboard token，与新 Sidebar JWT 语义不一致 | 无直接截图 | 作为兼容恢复页：只解析可信字段，不接受旧 realm token，转入 `/login` 的 Sidebar OAuth；非法参数 fail-closed | URL `callValues`、`agentId`、`pageFlag`；不伪造登录 |
| `/contact` | 通过 `wxExternalUserid` 调 `/workContact/detail` 显示四字段 | 缺少详情、备注、标签、所在群、轨迹、画像、SOP 提醒和业务入口 | 图 3客户页、图 4列表/空态 | 先解析客户 ID，再并发读取 show/track/portrait/SOP；摘要卡、维护入口、轨迹/画像分段；部分失败不覆盖已加载摘要 | `GET /sidebar/workContact/detail|show|track`、`GET /sidebar/contactFieldPivot/index`、`GET /sidebar/contactSop/getSopTipInfo` |
| `/contact/editDetail` | 占位 | 无画像表单、加载、校验、保存/取消 | 图 1分组列表 | 按后端字段类型渲染文本、单选、多选、下拉、日期、图片；图片上传走真实接口；提交防重和失败保留输入 | `GET/PUT /sidebar/contactFieldPivot/*`、`POST /sidebar/common/upload` |
| `/contact/remark` | 占位 | 无当前值、长度校验和真实保存 | 图 1列表/表单节奏 | 加载当前备注；1–10 字校验；保存防重；取消不写入；成功返回 | `GET /sidebar/workContact/show`、`PUT /sidebar/workContact/update` |
| `/contact/settingTag` | 占位 | 无分组、已有标签、选择和真实保存 | 图 4搜索/筛选/空态 | 加载已有标签与标签组；按组筛选；已有标签锁定；只追加后端允许的新标签；明确“当前契约不删除标签” | `GET /sidebar/workContact/show`、`GET /sidebar/workContactTagGroup/index`、`GET /sidebar/workContactTag/allTag`、`PUT /sidebar/workContact/update` |
| `/contactBatchAdd` | 占位 | 无批次参数校验、状态筛选、号码列表、复制/加好友 | 图 4列表/空态 | 批次摘要、全部/待分配/待添加/待通过/已添加筛选、真实号码、剪贴板回退、企微添加客户 | `GET /sidebar/contactBatchAdd/detail`、JS-SDK `navigateToAddCustomer` |
| `/contactSop` | 占位 | 无 `id` 单条和当前客户提醒列表、内容复制、客户跟进 | 图 2会话任务卡 | 有 `id` 读单条；无 `id` 时按当前客户读取提醒；文本/图片内容卡、复制、客户信息；不伪造“已发送” | `GET /sidebar/contactSop/getSopInfo|getSopTipInfo`；JS-SDK 不支持的跟进动作显示不可用说明 |
| `/login` | 校验 agentId，生成真实 OAuth 链接 | 恢复说明和重复点击控制弱 | 图 1状态卡 | 明确员工身份、目标页和安全提示；按钮防重复导航；无有效 agentId 时 fail-closed | `/sidebar/agent/auth` |
| `/medium` | 占位 | 无分组/类型/搜索/分页/选择/发送 | 图 4筛选空态、图 2宫格 | 文本/图文/文件/图片/视频分类；分组、搜索、分页；多选；发送前 JSSDK 配置；临时 mediaId 刷新；逐项结果，不随机成功 | `GET /sidebar/mediumGroup/index`、`GET /sidebar/medium/index|mediaIdUpdate`、`GET /sidebar/agent/jssdkConfig`、JS-SDK `sendChatMessage` |
| `/roomSop` | 占位 | 无任务/群信息/完成状态/重复提交 | 图 2会话任务卡 | `id` 校验、任务内容、群摘要、复制、完成操作；已完成禁用；写入失败可重试且不本地伪完成 | `GET /sidebar/roomSop/getSopInfo`、`PUT /sidebar/roomSop/logState`；未签名的打开群聊动作不宣称可用 |

## 5. 信息架构与页面边界

### 5.1 底部导航

- “客户”：`/contact` 及客户资料、批量加好友、素材相关页。
- “会话”：`/contactSop` 和 `/roomSop`。
- “我的”：`/`。
- `/login`、`/auth`、`/codeAuth`、404 不显示底部导航。
- 导航在切换时保留已验证的客户/批次/SOP 上下文 query 和 hash，删除 OAuth `state`；不把某业务的 `id` 错带到另一业务。

### 5.2 领域边界

```text
SidebarRouter/AuthBoundary
├─ auth/            仅处理 Sidebar 会话、OAuth 与兼容恢复
├─ features/contact 客户上下文、摘要、轨迹、画像、备注、标签
├─ features/medium  素材查询、选择和企业微信发送
├─ features/sop     个人/群 SOP 展示与群 SOP 完成
├─ features/batch   批量加好友列表和企微跳转
├─ wecom/           JS-SDK 配置、能力检测、调用和错误归一化
└─ ui/              Sidebar 专用壳、操作栏、表单/列表小组件
```

每个 feature 的 API 文件负责把未知 JSON 校验/转换为稳定领域类型；页面不直接猜测旧接口字段。`wecom` 只包装 `window.wx` 和 `/agent/jssdkConfig`，能力缺失时返回可识别错误，不向业务页报告假成功。

## 6. 统一组件与视觉规范

### 6.1 通用令牌

- 背景 `#eef6ff`；表面 `#fff`；主文字 `#172033`；次文字 `#526075`；弱文字 `#8b96a8`。
- 主色 `#1677ff`；成功 `#13b78a`；提醒 `#f59e42`；错误 `#d92d20`；边界 `#dce7f5`。
- 卡片圆角 18–20px；输入/按钮 12–14px；胶囊 999px；阴影 `0 8px 24px rgba(23,32,51,.06)`。
- 页面/卡片间距遵循 4/8/12/16/24px 阶梯；正文行高不低于 1.5。

### 6.2 共享与 Sidebar 专用组件

- `mobile-foundation`：`MobileShell`、`MobileCard`、`MobileState`、`MobileBottomNavigation`、通用粘性操作区；仅在至少两个移动页面有明确复用价值时修改。
- Sidebar 专用：`SidebarPageShell`、`SidebarSection`、`SidebarFieldRow`、`SidebarFilterBar`、`SidebarStickyActions`、`AsyncBoundary`。
- 业务组件：客户头像/标签、素材卡、SOP 内容块、批量号码行分别留在各 feature，避免“一个通用占位卡”吞掉业务差异。

### 6.3 表单与固定操作区

- 保存/取消子页使用顶部轻量操作栏和底部可选粘性保存区；不会同时出现两个主操作。
- 输入区获得焦点时，固定操作区使用 `position: sticky` 或随内容流移动，配合 `visualViewport`/`scrollIntoView`，不被软键盘永久遮挡。
- 保存期间禁用重复提交并显示“保存中”；失败保留输入；成功以真实 API 响应为准再返回。
- 长文本使用 `overflow-wrap:anywhere`；标签/筛选项可换行；不使用页面级横向滚动。

## 7. 交互状态机

### 7.1 读取型页面

```text
参数校验 ─失败→ 参数错误（不发请求）
   │通过
   ▼
加载 ─成功且有数据→ 内容
 │        └成功但为空→ 空态
 ├401→ 清理 Sidebar 会话 → /login（保留安全 target）
 ├403→ 无权限（不清理会话）
 ├404/业务不存在→ 不存在
 └网络/5xx→ 错误 + 重试
```

复合客户页允许摘要成功、次级模块失败：摘要保留，失败区块单独重试，避免一个轨迹接口让整页白屏。

### 7.2 写入型页面

```text
初始加载 → 编辑中 → 本地校验
                    ├失败→ 就地错误 + 聚焦字段
                    └通过→ submitting（按钮禁用）
                               ├成功→ 成功提示/返回
                               ├409→ 保留输入 + 冲突提示
                               ├401→ 重新授权
                               └网络/5xx→ 保留输入 + 重试
```

取消永不调用写 API；浏览器后退与显式取消语义一致。当前后端标签契约只新增不删除，页面锁定旧标签并说明原因，避免界面允许删除但服务器实际不删除。

### 7.3 企业微信动作

```text
检测 window.wx → 获取签名配置 → agentConfig ready → invoke
    ├宿主/SDK缺失→ “请在企业微信中打开”
    ├签名失败→ 可重试错误
    ├invoke失败→ 显示企微错误，不更新业务状态
    └invoke ok→ 仅报告该动作成功
```

群 SOP 的“已完成”由 `PUT /roomSop/logState` 写入并再次 `GET /roomSop/getSopInfo` 确认后决定；复制文本成功不等于消息已发送；素材逐项发送，部分失败不会被汇总成全部成功。

## 8. API、数据真实性、鉴权与错误策略

### 8.1 数据真实性

- 所有页面只消费 `/sidebar/*` 的持久化 API 或测试中明确命名的 fixture。
- fixture 仅用于单元/E2E，生产构建不包含 fixture 开关和随机内容。
- API 响应先进行结构校验；结构不合法转换为 `MobileApiError('validation')`，页面显示“响应格式无效”，不静默补假字段。
- 图片/媒体 URL 使用后端返回的 `*FullPath` 或真实 URL；无头像使用纯 CSS/SVG 中性占位，不冒充客户照片。

### 8.2 鉴权与权限

- `/sidebar-app` 使用 Path 为 `/sidebar-app` 的 `token`、`agentId` cookie；根挂载测试只使用 Sidebar 专用 sessionStorage。
- API 客户端固定 `basePath:'/sidebar'`，覆盖调用方传入的 Authorization，阻止跨 scope 路径和开放重定向。
- 401 清理 Sidebar 会话并重新授权；403 保留会话；OAuth target 只允许同源内部路径。
- `/workContact/detail` 与 Sidebar 画像读写必须在 Go 层校验员工—客户—企业归属；已有画像 pivot 还必须匹配本次 `contactId` 与字段 ID，越权统一返回 403。
- `/codeAuth` 不信任旧 Dashboard realm token，不把 `callValues` 直接写入 Sidebar 会话；只提取 agentId/目标意图并进入 `/login`。
- 日志和错误 UI 不输出 token、OAuth code、Base64 state、secret 或完整敏感 URL。

### 8.3 已知契约限制

- `workContact/update` 的标签行为是“新增缺失项，不删除旧标签”；界面必须如实表达。
- `contactSop` 没有“已发送”写接口；只提供读取、复制和企微动作，不保存假完成状态。
- 当前 Go `agentJSSDKAPIs` 包含 `getCurExternalContact`、`sendChatMessage`、`getContext`、`shareAppMessage`、`navigateToAddCustomer`，不包含 `openUserProfile`、`openExistedChatWithMsg`；相关按钮不得伪报可用。
- 当前 Go JSSDK 合同把 `timestamp` 序列化为十进制字符串；前端必须严格校验正整数后转换为 SDK 所需数字，合同测试不得使用理想化数字夹具掩盖真实响应。
- 真正的 OAuth、JS-SDK 签名和宿主 invoke 需要企业微信可信域名、应用凭证和会话环境；自动化只能验证可控的 URL、签名请求、能力检测和错误分支，真实宿主链路标记 SKIP。

## 9. 响应式、安全区、键盘与可访问性

- 设计视口：360×800、390×844、430×932；额外验证 320×568、844×390 横屏。
- `min-height:100dvh`，同时保留 `100vh` 回退；顶部/底部使用 `env(safe-area-inset-*)`。
- 底部导航高度与 `safe-area-inset-bottom` 合并，正文末尾预留导航高度；悬浮/粘性操作不覆盖最后一项。
- 软键盘：表单不锁死 body 高度；焦点字段滚入可视区；横屏下操作栏随文档流或 sticky，不使用不可见的 fixed footer。
- 所有可交互元素至少 44×44px；图标按钮有中文可读名称；纯装饰 SVG `aria-hidden`。
- 加载使用 `aria-live=polite`；写入失败使用 `role=alert`；Tab、筛选和复选框使用原生语义与清晰 focus-visible。
- 颜色对比不只靠颜色表达；成功/失败同时有图标或文字；支持 `prefers-reduced-motion`。
- 头像和素材图片有替代文本；视频不自动播放；长名称、长错误、长标签不造成横向溢出。

## 10. 兼容、迁移与回滚

### 10.1 兼容与迁移

- 不改变 12 条路由、basename、API 前缀和 query 名；旧入口仍可访问。
- 新客户上下文优先使用 `wxExternalUserid`，再通过 `/workContact/detail` 获取可信 `contactId`；已有 `contactId` 仅在正整数且仍由服务端权限约束时使用。
- 先增加领域 API 校验和组件测试，再逐路由替换占位；每一阶段保持构建可运行。
- 若需要扩展 `mobile-foundation`，仅增加通用 sticky/action 状态，不引入 Sidebar token、客户或企微 SDK。
- Go 调研确认 Sidebar 客户详情与画像接口缺少资源级授权，因此增加只收紧非法访问的员工—客户—企业归属校验，并先补 403 合同测试；不改成功响应结构、不迁移数据库。

### 10.2 回滚

- 设计/计划、领域 API、客户维护、素材/企微、SOP/批量、验收证据分别原子提交。
- 任一业务域可通过回退对应提交恢复为基线占位页，不影响 Sidebar OAuth 和其他端。
- 不迁移数据库、不删除 Docker 命名卷；Go 安全收紧可随对应原子提交回滚，因此回滚不涉及数据恢复。
- 最终不直接合入 main；功能分支保留精确 base SHA，若 `origin/main` 推进，由主线负责人基于验收后的提交做显式 rebase/cherry-pick。

## 11. 客观验收标准

### 11.1 自动化门禁

- `@mochat/sidebar` 全部测试、typecheck、lint、production build 退出码 0。
- 若修改 `@mochat/mobile-foundation`，其全部测试、typecheck、lint、build 退出码 0。
- `corepack pnpm check:mobile-clients-foundation` 不回退。
- 相关 Web workspace 测试与移动 E2E 退出码 0；若修改 Go，相关 Go 包与合同测试退出码 0。
- `git diff --check` 无错误；`docs/PROJECT_PROGRESS.zh-CN.md` 无差异；无 Operation/Dashboard/SaaS Admin/Phase 7 业务改动。

### 11.2 路由与业务验收

- 12 条路由均可达、标题唯一、未知路由为 404；公开路由无员工导航，受保护路由无会话时进入登录。
- `/contact` 覆盖加载、真实内容、局部空态、局部错误、重试、长文本、滚动和 401 恢复。
- 三个客户编辑路由覆盖加载、校验、取消、保存、防重、失败保留输入和返回。
- `/medium` 覆盖搜索、类型、分组、分页、空态、选择、JSSDK 不可用、发送部分失败和 mediaId 刷新。
- `/contactSop`、`/roomSop` 覆盖参数缺失、内容列表、复制、真实完成、重复提交和完成后禁用。
- `/contactBatchAdd` 覆盖五种状态、空态、复制回退和企微不可用提示。
- `/login`、`/auth`、`/codeAuth` 覆盖非法参数、成功/失败回调、安全 target 和过期会话。

### 11.3 浏览器与视觉验收

- Docker 环境实际加载 Sidebar，保留所有命名卷。
- 360×800、390×844、430×932 全部走查；额外完成 320×568、844×390/表单键盘等价场景。
- 12 路由无白屏、console error、未处理 Promise、文档级横向溢出、底部遮挡或小于 44px 的主要触控区。
- 四张参考图分别映射到：首页/导航、SOP、客户详情、素材/批量空态；每类保留参考图和实现后截图，并写明未复制的能力与原因。
- 真实企业微信 OAuth/JS-SDK 宿主动作如缺凭证/可信域名，标记 SKIP 并列出已通过的可控分支，不得标记 PASS。

## 12. 设计自检

- 未包含 `TBD`、`TODO` 或未定义的生产假数据入口。
- 12 路由、允许范围、API 前缀和鉴权语义与仓库基线一致。
- 参考图只决定视觉与信息层级，不把截图中的外部产品能力扩大到 Mochat。
- 写操作均以真实服务端响应为准；缺失能力有明确不可用反馈。
- 设计可拆成文件级 TDD 任务并支持按业务域回滚。
