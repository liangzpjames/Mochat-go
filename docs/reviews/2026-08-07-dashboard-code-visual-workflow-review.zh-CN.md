# Dashboard 代码 / 视觉 / 工作流综合审查报告

- 审查日期：2026-08-07
- 审查基线：`main` @ `b262be2`（运行中部署与此一致，构建产物哈希相同）
- 审查范围：Dashboard 前端（`web/apps/dashboard/src`）、SCRM/报表/AI/文件录音后端（`internal/modules/{scrm,reporting,ai-insight,ai-settings,chat-media,providers}`）、门禁脚本、运行中应用 `http://127.0.0.1:18080`
- 证据目录：`D:\workspace\mochat-go\output\dashboard-review-20260807\`（12 张截图、识图原始记录 677 行、DOM 验证 JSON、vitest 日志）

---

## 1. 审查方法与工具链

本轮按“代码审查、视觉审查、工作流审查”三线并行开展：

1. 代码审查：全量质量门禁复跑 + 关键文件逐读（订单状态机/报表口径/上传校验/权限与租户隔离/前后端契约）。
2. 视觉审查：Playwright 截取 12 个页面，交 `qwen3-vl-plus`（严格缺陷审查模式）逐图审阅，再用 DOM computed style、bounding box、请求拦截做交叉验证，剔除模型误报。
3. 工作流审查：真实登录后对订单闭环、报表筛选、漏斗下钻、AI 受限态、文件录音、深链接刷新等做浏览器实测，并核对前端请求参数与后端解析代码。

说明：本轮子代理消息通道故障（两个子代理均未收到任务正文），因此三线审查全部由主代理亲自完成；不影响结论，但工作量与上下文消耗高于预期。

---

## 2. 客观基线（全部通过）

| 检查项 | 结果 |
| --- | --- |
| `go test ./...` | 通过（全模块 ok，无 FAIL） |
| Dashboard 测试 | 88 个文件 510/510 通过（落盘复跑退出码 0） |
| `pnpm --filter @mochat/dashboard typecheck` | 通过 |
| `pnpm --filter @mochat/dashboard build` | 通过（产物与容器内一致：index-Bwe9_Dna.js） |
| `pnpm check:debt-clearance` | 27/27 达标 |
| `pnpm check:phase3-5-dashboard` | 9/9 通过 |
| `git diff --check` | 干净 |
| 容器健康 | app/mysql/redis 均 healthy，`/readyz` 200 |
| AI 外部调用 | 测试期关闭有效：`mochat_go_ai_analysis` 行数保持 16 不变 |
| 滚动结构 | 顶部 banner/左侧菜单固定，内容区独立滚动（文档高度=视口，内容区 overflow-y:auto） |
| 菜单高亮 | 抽查 8 个页面，激活菜单与当前路由全部一致 |

---

## 3. 已复核排除的误报（避免后续返工）

识图模型共输出 40+ 条疑似问题，经 DOM/代码交叉验证后，以下属于误报或主观项，**不需要修改**：

| 模型报告 | 复核结论 |
| --- | --- |
| “默认租户演示企业”是蓝色按钮/高亮冲突 | 实为静态胶囊徽标（cursor:auto、padding 5px 9px、圆角 999px），非可点击元素 |
| 菜单“展开但箭头朝上，逻辑矛盾” | 展开=⌃、收起=⌄ 是常规交互，DOM `aria-expanded` 与箭头一致 |
| “趋势图无 X 轴标签” | DOM 实测存在标签 `08-06 / 08-07`（`.phase35-trend-label`） |
| “2026 年是未来时间” | 模型知识截止误判，系统当前日期即 2026-08-07 |
| “报表默认日期不合理/未来时间” | 默认区间为本月 1 日 00:00 至下月 1 日 00:00（半开区间），属既定设计 |
| “运行界面是旧构建” | 容器内 dist 哈希与本地 `main` 构建一致，部署与 HEAD 同步 |
| “SaaS 登录页 Logo 缺失” | 属 SaaS 登录页自身布局，非资源损坏；但 404 资源错误另有记录（见 I6） |

---

## 4. 问题清单

### 4.1 Critical（先修，影响数据正确性或“宣称功能实际不可用”）

**C1｜报表“员工/部门筛选”前后端参数不匹配，筛选实际不生效**

- 位置：前端 `web/apps/dashboard/src/features/phase35/report-query.tsx`（发送 `employeeId` / `departmentId`）；后端 `internal/modules/reporting/transport/http/handler.go` `parseQuery`（只读 `employeeIds` / `departmentIds`）。
- 证据：实测设置员工=4、部门=1 后点击查询，请求为 `/dashboard/reports/customer?...&employeeId=4&departmentId=1&...`；后端解析不到对应字段，筛选被静默丢弃。
- 影响：客户分析/会话分析/转化分析/行为分析/综合报表五页的员工与部门筛选全部无效，属于“界面在、功能假”。
- 修复方向：统一参数名（建议后端同时兼容单复数，前端按复数发送），补充契约测试与一次 E2E 断言请求参数和结果变化。

**C2｜转化漏斗“点击阶段下钻”没有真实明细**

- 位置：`web/apps/dashboard/src/features/phase35/conversion-report-page.tsx` + `components/conversion-funnel.tsx`。
- 证据：实测点击“线索”阶段，抽屉只显示“线索阶段共 0 条记录。当前筛选范围暂无该阶段明细，请调整日期、员工或部门后查询。”——无该阶段的真实对象列表，仅一个计数与通用文案。
- 影响：验收口径里的“漏斗下钻”名不副实，业务人员无法从漏斗追踪到具体线索/商机/订单。
- 修复方向：后端提供按阶段的明细接口（对象列表+分页+负责人/时间），前端抽屉改为真实表格；若保持“仅计数”设计，则需同步修改 manifest 验收口径，不能两套说法。

### 4.2 Important（影响业务正确性、安全或关键体验）

**I1｜深层路由/刷新存在竞态，偶发被重定向到 `/index`**

- 位置：Dashboard 路由与 access-context 初始化（`web/apps/dashboard/src/app/access-context.tsx`、路由入口）。
- 证据：实测连续 3 次直接访问 `/data/customer`，第 1 次被重定向到 `/index`（URL 变为 /index、h1 为“数据概览”），后 2 次正常。刷新、书签、分享链接不可靠。
- 修复方向：登录后先完成 corp/access 恢复再匹配目标路由；未就绪时渲染 loading 而非重定向；补“刷新深层路由 5 次”的 Playwright 回归。

**I2｜订单列表无分页、全量拉取**

- 位置：后端 `internal/modules/scrm/transport/http/order_handler.go`（`ListContext` 无 page/pageSize/total）；前端 `order-page.tsx`。
- 证据：订单页一次请求返回全部订单，页面只显示“共 4 条”且没有任何翻页按钮。
- 影响：订单量增长后接口超时、页面卡顿；与客户报表等页面的分页体验不一致。
- 修复方向：订单列表接口增加分页与总数，前端复用 `ReportDetailTable` 风格的分页控件。

**I3｜订单 ID 由客户端生成且带验收前缀**

- 位置：`order-page.tsx`（`id: P35-ORDER-${Date.now()}`）；后端 `domain.NewOrder` 仅校验非空。
- 证据：当前库内 5 笔订单全部是 `P35-ORDER-*` 前缀；生产环境继续这样创建会永久携带验收痕迹，且 ID 可被客户端伪造/冲突。
- 修复方向：订单 ID 改为服务端生成（UUID/雪花）；前端不再传 id；存量 `P35-ORDER-*` 数据提供一次性迁移或仅在验收库保留。

**I4｜文件录音上传只校验 Content-Type 头，未校验文件内容与扩展名**

- 位置：`internal/modules/chat-media/transport/http/handler.go` `upload`。
- 证据：代码只判断 `header.Content-Type` 以 `audio/` 开头；扩展名直接取原始文件名且无白名单；文本文件改名 `.wav` 并声明 `audio/wav` 即可通过。软删除后磁盘文件不清理（仅 DB 置 deleted_at）。
- 影响：可存入非音频内容，浪费存储并存在内容伪造风险；删除后文件残留。
- 修复方向：增加魔数检测（WAV/MP3/AAC 文件头）、扩展名白名单、服务端确定 Content-Type；软删除后增加异步磁盘清理任务。

**I5｜综合报表 KPI 卡片对异构指标直接求和，口径有误导**

- 位置：`report-page.tsx` `sections` 定义（`keys.reduce((sum,key)=>sum+Number(summary[key]??0),0)`）。
- 证据：“客户概览”= customer+contact 求和（同一批对象被重复计数）；“转化漏斗”= lead+opportunity+won（不含订单），因此出现“订单经营 1、转化漏斗 0”的观感矛盾。
- 修复方向：每张 KPI 卡只展示单一主指标（或明确的“分子/分母”），去掉异构求和；漏斗卡增加“未进入订单阶段”的口径说明。

**I6｜SaaS 登录页预填测试手机号，并存在 404 资源**

- 位置：`/security/login`（SaaS/安全登录页）。
- 证据：实测输入框预填 `13800000000`；页面加载报 2 次 404（资源缺失）。
- 影响：生产环境暴露测试账号痕迹，易被误用；404 资源需定位（favicon/图标等）。
- 修复方向：移除预填；定位并修复 404；补登录失败错误提示区。

**I7｜AI 洞察受限态文案重复、术语不统一**

- 位置：`ai-insight/*` 五页及 `employee-report-page.tsx`。
- 证据：DOM 实测“受限”出现 3 次（状态徽标、KPI、结果区），另混用“AI 能力未接入”“Provider 受限”“尚未生成”等说法，且无跳转配置入口。
- 修复方向：统一为单一受限状态组件（状态徽标+一句原因+“前往接入”入口），全站维护术语表。

**I8｜分页控件覆盖不一致**

- 位置：行为分析/转化分析/综合报表/订单页只有“共 N 条，第 X 页”文本或完全无分页；客户分析/好友/客户群/文件录音有上一页/下一页按钮。
- 修复方向：将 `ReportDetailTable` 的分页控件统一推广到所有列表型报表页，并保证“总条数=行数”一致。

### 4.3 Minor（一致性、可访问性、数据卫生）

- **M1**：顶部“账号 1”直接显示数字 userId，无姓名/头像；产品化应显示员工姓名。
- **M2**：报表“员工/部门”筛选是纯数字输入，无员工列表选择，普通用户不知道填什么（且当前因 C1 完全不生效）。
- **M3**：订单状态“待支付/已支付/已完成/已取消”无颜色语义（同为灰/黑色文字）。
- **M4**：联系人页提示“筛选和当前详情均保存在 URL”对销售用户不友好，建议改“链接/网址”。
- **M5**：综合报表“查看详情”用 `<a href>` 整页跳转，应改 SPA 路由避免整页刷新。
- **M6**：KPI 卡片顶部色条（蓝/绿/紫/橙）无图例与语义说明。
- **M7**：验收数据污染仍存在：`P35-ACCEPT-API??` 联系人名称（含问号）、`P36-ACCEPT-知识库-<时间戳>` 命名、一笔空标题订单；清理 SQL 已备好（`deploy/standalone/acceptance/cleanup_phase3_final.sql`），由用户确认后执行。
- **M8**：chat-media 的 `List`/`GetByID` 未显式带 `tenant_id` 条件（当前 corpId 全局唯一且鉴权按 corp 校验，可接受，但建议加防御性条件）。
- **M9**：AI 知识库空文档无引导按钮；客户继承空态无“去分配”动作入口。

---

## 5. 解决方案设计

### 批次 A：数据正确性优先（预计 1–2 天）

| 编号 | 对应问题 | 方案 | 验收标准 |
| --- | --- | --- | --- |
| A1 | C1 | 前后端统一报表筛选参数（后端兼容 `employeeIds/departmentIds`，前端改复数）；`parseQuery` 增加单测；E2E 断言请求 URL 含参数 | 员工/部门筛选后请求参数正确，且数据库层面结果集变化可验证 |
| A2 | C2 | 后端新增“按阶段明细”查询（复用 detail report 的列表+分页），前端抽屉渲染真实表格（对象/时间/负责人），移除通用占位文案 | 点击漏斗任一步骤，抽屉展示该阶段真实记录并可翻页 |
| A3 | I2 | 订单列表接口支持 `page/pageSize/total`；前端加翻页控件并保持“共 N 条”与行数一致 | 100 笔订单数据下分页正常，接口响应含 total |
| A4 | I3 | 订单 ID 改服务端生成；前端不再传 id；提供存量 `P35-ORDER-*` 迁移/清理脚本 | 新订单 ID 为服务端格式；存量数据迁移脚本 dry-run 通过 |

### 批次 B：安全与关键体验（预计 3–5 天）

| 编号 | 对应问题 | 方案 | 验收标准 |
| --- | --- | --- | --- |
| B1 | I4 | 上传加魔数检测与扩展名白名单；Content-Type 以检测结果为准；软删后异步清理磁盘文件 | 文本改名 `.wav` 被拒；删除后 content URL 404 且磁盘文件最终消失 |
| B2 | I1 | 登录后 access 恢复完成前不重定向；路由初始化等待 corp/access；补深链回归测试 | 连续 5 次刷新深层路由均停留在原页面 |
| B3 | I5 | KPI 卡改为单一主指标或“分子/分母”，去除异构求和；漏斗卡补充口径说明 | 综合报表四卡数值与对应明细页主指标一致，无重复计数 |
| B4 | I6 | 移除登录页手机号预填；定位并修复 404 资源；补错误提示区 | 登录页无预填；控制台无 404 |
| B5 | I7 | 统一受限状态组件与术语表；状态区提供“前往接入”入口 | 五页 AI 受限文案唯一、可操作 |

### 批次 C：一致性收尾（可与 B 并行）

- C1：顶部显示员工姓名（后端返回姓名，前端降级为账号 ID）。
- C2：订单状态颜色语义化（待支付=橙、已支付=蓝、已完成=绿、已取消=灰）。
- C3：分页控件统一推广到行为/转化/综合报表/订单页。
- C4：综合报表“查看详情”改 SPA 路由跳转。
- C5：员工筛选改为员工下拉（后端提供员工列表接口，按权限范围过滤）。
- C6：KPI 色条加图例/语义说明。
- C7：联系人“URL”措辞改“链接”。
- C8：破坏性操作（知识库/角色/规则删除与停用）统一 Popconfirm，并抽查权限禁用态。
- C9：空态引导（知识库“新建”、客户继承“去分配”）。
- C10：执行验收数据清理 SQL（需用户确认），并验证联系人名称不再出现 `??`。

---

## 5.5 批次 A 执行结果（2026-08-07 晚）

批次 A（数据正确性）已全部完成并通过门禁、接口与浏览器三重验证，尚未提交（改动保留在工作区，等待用户确认后提交）。

| 编号 | 状态 | 实施内容 | 验证结果 |
| --- | --- | --- | --- |
| A1 | ✅ | 报表筛选参数对齐：前端发送 `employeeIds/departmentIds`（复数），后端主读复数并兼容单数回退；抽出 `buildReportFilterParams` 纯函数 | 实测 `employeeIds=999` 四个报表归 0；`employeeId=4` 单数兼容正常；`employeeIds=4` 全量数据返回；5 个报表页不受影响 |
| A2 | ✅ | 转化漏斗真实下钻：`ReportQuery` 新增 `stage`，后端按 lead/contact/opportunity/won/order 返回真实对象明细（对象 ID、名称/联系人、负责人、创建日期），支持分页与员工筛选；前端抽屉改为真实表格 + 小号分页 | 实测联系人阶段返回 4 条真实明细、订单阶段返回 1 条；无 stage 时仍只返回漏斗汇总；抽屉展示“共 N 条，第 1 页” |
| A3 | ✅ | 订单列表分页：`ListContext` 增加 `page/pageSize/total`，SQL 加 COUNT + LIMIT/OFFSET；响应改为 `{items,total,page,pageSize}`；前端订单页加翻页控件 | 实测 total=5，page1 返回 3 条、page2 返回 2 条；单页数据时“下一页”正确禁用；多页交互由单元测试覆盖 |
| A4 | ✅ | 订单 ID 服务端化：`NewOrder` 在 ID 为空时生成 UUID，前端创建订单不再传 `id`；存量 `P35-ORDER-*` 保留展示 | 实测新订单返回 `d7677c13-03d2-4ab9-80d6-25513360cd63`（非 P35 前缀）并出现在列表第一页 |

### A 批期间顺带修复

- 明细抽屉浮层：`Phase35DetailDrawer` 此前没有对应 CSS（组件类名无样式），导致“抽屉”实际是页尾内嵌块。已补右侧浮层 + 半透明遮罩 + 吸顶头部 + 表格横向滚动，影响转化/行为/好友/群/订单共 5 个详情抽屉。
- 前端测试基建：`conversion-report-page.test.tsx` 补充 `afterEach(cleanup)`，避免多测试残留 DOM 串扰。

### A 批门禁

| 检查项 | 结果 |
| --- | --- |
| `go test ./...` | 通过（含 reporting/scrm 全模块） |
| Dashboard 测试 | 88 个文件 514/514 通过 |
| `pnpm --filter @mochat/dashboard typecheck` / `build` | 通过 |
| `pnpm check:debt-clearance` | 27/27 达标 |
| `git diff --check` | 干净 |
| 部署 | `deploy_docker_desktop.ps1` 完成，容器 healthy，`/readyz` 200 |
| 浏览器验收 | 转化抽屉真实表格 + 分页、订单页“共 5 条”与服务端 UUID 均可见；识图复核订单页无问题，抽屉遮罩经 computed style 验证 |

证据目录：`D:\workspace\mochat-go\output\a-batch-ui-verify-20260807\`（截图、`evidence.json`、识图复核记录）。

---

## 5.6 批次 B 执行结果（2026-08-07 晚，A 批之后）

批次 B（安全与关键体验）已全部完成并通过门禁、接口与浏览器三重验证，改动保留在工作区，等待用户确认后提交。

| 编号 | 状态 | 实施内容 | 验证结果 |
| --- | --- | --- | --- |
| B1 | ✅ | 音频上传改为魔数检测（WAV/MP3/OGG/FLAC/M4A/AAC/AMR/WebM），忽略客户端 Content-Type 与扩展名，按检测结果写入规范扩展名；软删后同步清理磁盘文件 | 文本改名 `.wav` 上传返回 400；`.mp3` 名 + octet-stream 的 WAV 内容被识别为 `audio/wav` 并落盘 `.wav`；删除后 content URL 404 |
| B2 | ✅ | 登录页恢复同源 `returnTo`（过滤 `//`、反斜杠网络路径、`/login`）；`CorpProvider` 自动选择企业时保留当前深链，不再无条件跳 `firstRoute` | 连续 5 次刷新 `/data/customer` 均停留原页；`returnTo=/data/employee?x=1` 登录后正确落地该页；新增 `deep-link-acceptance.mjs` 回归脚本 |
| B3 | ✅ | 综合报表四张 KPI 卡改为单一主指标（customer/lead/order/behavior），去除 customer+contact、lead+opportunity+won、order+amount 等异构求和，并补充口径说明 | 实测 UI 四卡数值与报表 API 主指标一致（4/0/1/9），无重复计数 |
| B4 | ✅ | 移除 SaaS 身份登录页手机号/密码预填（含 main/compose/部署脚本中的 env 注入）；默认 favicon/Logo/背景改为内联 SVG data URI，消除 404 | `/security/login` 输入框为空；浏览器网络无 `/favicon.ico`、`/img/*` 404；页面含内联 SVG 资产 |
| B5 | ✅ | AI 洞察五页受限态文案统一为“AI 能力未接入 / 当前未接入可用的 AI 分析 Provider，暂无分析结果”，并提供“前往接入 AI 能力”入口 | `/ai-insight/communication-keyword` 受限态文案唯一、入口链接存在 |
| B6 | ✅ | 角色停用/删除增加系统预置角色保护（remarks 为“系统预置全权限角色”或旧 `bootstrap full-access role` 时拒绝） | `PUT /dashboard/role/statusUpdate` 对内置角色返回 400；删除同样被拒 |

### B 批顺带修复

- 明细抽屉浮层样式（同 A 批记录，影响 5 个详情抽屉）。
- 登录页内联 SVG 编码修正（空格 `%20` 编码，避免 Logo 渲染成灰块）。
- 新增 `web/e2e/deep-link-acceptance.mjs`、`web/e2e/b-batch-acceptance.mjs` 作为可重复验收脚本。

### B 批门禁

| 检查项 | 结果 |
| --- | --- |
| `go test ./...` | 通过（全模块 ok，无 FAIL） |
| Dashboard 测试 | 88 个文件 517/517 通过 |
| `pnpm --filter @mochat/dashboard typecheck` / `build` | 通过 |
| `pnpm check:debt-clearance` | 27/27 达标 |
| `git diff --check` | 干净 |
| 部署 | `deploy_docker_desktop.ps1` 完成，容器 healthy，`/readyz` 200 |
| 浏览器验收 | `b-batch-acceptance.mjs` 全部通过（B2 刷新/回跳、B3 主指标、B4 无预填无 404、B5 受限态入口）；识图复核综合报表无问题、登录页无预填且无灰块 |

证据目录：`D:\workspace\mochat-go\output\b-batch-ui-verify-20260807\`（登录页、综合报表截图 + 识图复核记录）。

### 遗留（批次 C 与后续）

- C 批一致性项（状态色、分页推广、员工下拉、URL 措辞、KPI 图例、Popconfirm、空态引导、验收数据清理等）未在本轮执行。
- 识图模型对登录页提出 placeholder、对比度、按钮对齐等主观/次要意见，归入 C 批视觉收尾。

---

## 5.7 批次 C 执行结果（2026-08-07 深夜，B 批之后）

批次 C（一致性收尾）已全部完成，改动保留在工作区，等待用户确认后提交。

| 编号 | 状态 | 实施内容 | 验证结果 |
| --- | --- | --- | --- |
| C1 | ✅ | 顶部账号区显示员工姓名：会话存储新增 `userName`（登录响应 `session.userName`），前端降级为“账号 ID” | 实测头部显示“超级管理员”，不再显示“账号 1” |
| C2 | ✅ | 订单状态语义色徽标：待支付=橙、已支付=蓝、已完成=绿、已取消=灰 | 订单列表 5 行均渲染 `.order-status` 彩色徽标 |
| C3 | ✅ | 分页控件统一：行为分析页增加上一页/下一页；转化/订单此前已完成；综合报表无列表页 | 行为页翻页按钮存在，与其余页样式一致 |
| C4 | ✅ | 综合报表“查看详情”改 SPA 路由跳转（react-router Link） | 点击后 URL 变为 `/data/customer`，无整页刷新 |
| C5 | ✅ | 员工筛选由数字输入改为员工下拉（`/workEmployee/index` 按权限返回） | 5 个报表页下拉含“全部员工”+ 3 名员工选项 |
| C6 | ✅ | 新增 KPI 色条图例（蓝=规模、绿=转化、紫=结构/经营、橙=行为/风险） | 四个报表页均渲染 `.phase35-kpi-legend` |
| C7 | ✅ | 联系人/商机页“保存在 URL”措辞改为“保存在链接” | 页面文本已替换，不再出现 URL 字样 |
| C8 | ✅ | 破坏性操作统一 Popconfirm：知识库/智能体删除、角色停用/删除、风险行为与超时规则停用/删除；确认按钮使用警示色 | 知识库删除点击后弹出确认层（取消/确认） |
| C9 | ✅ | 空态引导：智能体“点击新建智能体开始创建”；客户继承空态提供“去分配”入口 | 对应空态文案与入口已实现并有单测 |
| C10 | ⏸️ | 验收数据清理 SQL 已存在（`deploy/standalone/acceptance/cleanup_phase3_final.sql`） | 涉及删数据，等待用户确认后执行，本轮未运行 |

### C 批门禁

| 检查项 | 结果 |
| --- | --- |
| `go test ./...` | 通过（无 FAIL） |
| Dashboard 测试 | 88 个文件 518/518 通过 |
| `@mochat/auth` 测试 | 4/4 通过（会话 userName 持久化） |
| typecheck / build | 通过 |
| 部署 | 完成，容器 healthy，`/readyz` 200 |
| 浏览器验收 | `c-batch-acceptance.mjs` 全部通过；`b-batch-acceptance.mjs` 回归通过 |
| 识图复核 | 订单彩色状态徽标、知识库确认弹层可见；其余页面以 DOM 断言为准（截图受视口高度限制） |

证据目录：`D:\workspace\mochat-go\output\c-batch-ui-verify-20260807\`（头部、报表图例、行为分页、订单状态、知识库截图 + `c-batch-acceptance.mjs` 输出）。

### 说明

- 识图模型对删除确认按钮“确认”使用主色提出意见，已改为 `danger` 警示色。
- 好友/客户群页的纯文本分页因单行压缩文件改动风险较高，未纳入本轮（不在审查 C3 清单内）。

---

## 6. 结论

- 整体质量：门禁全绿、53 页实现真实、部署与 HEAD 同步、核心业务闭环（订单→报表）数据自洽，**可以继续在此基础上迭代**。
- 必须先修：C1（筛选假功能）、C2（下钻假功能）属于“界面在、功能假”，与项目一贯的完成口径冲突，建议作为下一轮开发的第一优先级。
- 建议节奏：批次 A 完成后即做一次小验收（测试 + 浏览器 + 识图复核），再进入批次 B/C；所有修复完成后补一轮全量回归。

## 附：证据文件

- 截图：`D:\workspace\mochat-go\output\dashboard-review-20260807\01-overview.png` … `12-saas-health.png`
- 识图原始记录：`vision-review.txt`（12 页逐条审阅）
- DOM 交叉验证：`dom-verify.json`
- 门禁日志：`vitest-basic.log` / `vitest-default.log`
