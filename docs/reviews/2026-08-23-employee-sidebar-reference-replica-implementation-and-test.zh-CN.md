# 员工移动侧边栏参考图复刻：实施结果与自测报告

## 1. 交付结论

本次仅优化员工在企业微信中使用的 `web/apps/sidebar`，完成了参考图对应的客户工作台、会话任务、个人中心和客户通讯录，并保留原有客户资料编辑、备注、标签、素材、个人客户 SOP、客户群 SOP、批量加好友及授权恢复流程。未修改 Operation、Dashboard 或 SaaS Admin UI 及其默认 URL 策略，未扩大到 Phase 7，也未接入或伪造真实会话存档。

- 分支：`feat/employee-sidebar-mobile-optimization`
- 精确基线：`b9a47cab45ec872bc81e61a20b06a8a3529311b2`
- 最终代码提交：`4c828199a327bae95502b14ad489f5de3624b16e`（本报告提交另计）
- 2026-08-23 最后一次 `git fetch origin main` 后的远端：`de902ef1797ae1dea2933ee9be2838253b34248e`
- 分叉状态：本分支相对远端为 `47` 个独立提交、落后 `35` 个提交；合入前必须基于评审结论做受控变基，不能直接混合未验收主线。
- 本地审阅地址：`http://127.0.0.1:28083/sidebar-app/login?agentId=7&target=%2F`
- Docker 实载地址：`http://127.0.0.1:28080/sidebar-app/`；因验收库没有真实 `mc_work_agent`，真实企业微信 OAuth 标记为 SKIP。

## 2. 设计与实施文档

- 高保真中文设计：`docs/superpowers/specs/2026-08-23-employee-sidebar-reference-replica-design.zh-CN.md`
- 高保真中文实施计划：`docs/superpowers/plans/2026-08-23-employee-sidebar-reference-replica.zh-CN.md`
- 前序员工移动端设计：`docs/superpowers/specs/2026-08-22-employee-sidebar-mobile-optimization-design.zh-CN.md`
- 前序员工移动端实施计划：`docs/superpowers/plans/2026-08-22-employee-sidebar-mobile-optimization.zh-CN.md`
- 本实施与自测报告：`docs/reviews/2026-08-23-employee-sidebar-reference-replica-implementation-and-test.zh-CN.md`

## 3. 参考图映射与实现差异

| 参考图 | 逐图结论 | 实现页面 | 实现证据 |
| --- | --- | --- | --- |
| `2e66797253a09c2a453eddd76fa10f68.jpg` | 员工身份、企业归属、分组设置与退出层级 | `/?tab=profile` 个人中心 | `docs/reviews/evidence/2026-08-23-sidebar-reference-replica/implemented-profile-390x844.png` |
| `7837dae832938f84613d03cc55e2c50b.jpg` | 会话数据总览、任务捷径及信息说明 | `/?tab=conversations` 会话任务 | `docs/reviews/evidence/2026-08-23-sidebar-reference-replica/implemented-conversation-390x844.png` |
| `b8e1cff0d6eeaeb79c6ebeb7173af11d.jpg` | 蓝色业务主视觉、四项指标、五个快捷入口和待办 | `/` 客户经营工作台 | `docs/reviews/evidence/2026-08-23-sidebar-reference-replica/implemented-customer-390x844.png` |
| `cb8f4a742f5f46ff33a3011fa7f8647b.jpg` | 搜索、客户长列表、头像、标签与底部三栏导航 | `/?tab=customers&view=contacts` 客户通讯录 | `docs/reviews/evidence/2026-08-23-sidebar-reference-replica/implemented-contacts-390x844.png` |

实现忠实复用了截图的信息密度、蓝色层次、紧凑卡片、快捷入口、列表节奏和固定底栏，但没有复制参考产品品牌、原生系统顶栏或静态图片。由于本期明确不接入真实会话存档，会话页不展示聊天数、回复率、敏感词等不可验证指标，而是明确说明数据范围，并使用真实 SOP、群 SOP 和批量加好友任务替代。审阅服务顶部的“本地安全审阅”工具条只存在于 28083，不属于生产 UI。

## 4. 路由完成状态

| 路由 | 完成状态 | 主要内容与数据来源 |
| --- | --- | --- |
| `/` | PASS | 客户/会话/我的三工作区；真实员工汇总、客户统计、任务统计 |
| `/auth` | PASS | 企业微信 OAuth 回调校验与失败反馈，不弱化安全语义 |
| `/codeAuth` | PASS | 兼容扫码授权参数校验与失败反馈 |
| `/contact` | PASS | 当前客户摘要、轨迹、画像、标签与 SOP 入口 |
| `/contact/editDetail` | PASS | 持久化画像字段编辑、校验、保存、取消 |
| `/contact/remark` | PASS | 备注/描述校验、单次持久化写入、防重复提交 |
| `/contact/settingTag` | PASS | 租户边界内标签选择、追加意图和持久化 |
| `/contactBatchAdd` | PASS | 批次详情、状态筛选、号码选择、JSSDK/剪贴板诚实反馈 |
| `/contactSop` | PASS | 个人客户 SOP 真实触达记录与任务文本复制；复制明确提示“尚未发送”，且该日志没有完成字段，不伪造发送或完成状态 |
| `/login` | PASS | “继续授权”实际跳转至安全回调并恢复目标页 |
| `/medium` | PASS | 素材分组、列表、选择、加载/空态/错误/SDK 不可用反馈 |
| `/roomSop` | PASS | 客户群 SOP、真实完成写入和非零终态恢复 |

根工作台额外提供 `view=contacts`、`view=contactSop`、`view=roomSop`、`view=batchAdd` 业务视图。联系人搜索词、联系人/任务页码、群 SOP 与批量加好友任务状态均写入 URL，可刷新和返回恢复；个人客户 SOP 固定使用 `recorded` 触达记录语义。所有受保护页面固定显示“客户/会话/我的”三栏导航；根挂载链接强制使用 `/sidebar-app/?...`，避免 Go 静态挂载器重定向时丢失查询串。

## 5. API 与数据真实性

新增并接入以下员工侧接口：

- `GET /sidebar/workbench/summary`：员工身份、部门、企业、客户、个人 SOP 触达记录及真实待办汇总。
- `GET /sidebar/workContact/index`：租户和员工范围内的持久化客户目录、分页与搜索。
- `GET /sidebar/workbench/tasks`：`contactSop + recorded` 查询仍能关联有效 SOP 与未删除客户的员工触达记录；`roomSop`、`batchAdd` 按 `pending|done` 查询员工真实任务。

生产实现使用数据库查询，不包含假客户、随机成功或纯前端内存生产状态；查询受当前企业、员工和 JWT 约束，个人 SOP 使用内连接排除孤立日志，批量加好友“已完成”要求同一批次没有任何待处理行。工作台汇总、联系人和任务采用独立加载状态，一个区块失败不会阻断另一个；任何工作台 `401` 都会清理 Sidebar 自己的会话并进入重新授权。未授权访问新汇总接口在 Docker 中返回 401。28083 的数据是代码中明确标记的本地审阅夹具，只用于安全、可重复的 UI 验收；写入夹具会持续到手动重置，不能混同生产能力。

个人中心只显示“员工会话已建立”，明确说明这不代表全部企业应用权限已授权，并标注“权限明细未接入”；“清理本端登录缓存”只清除 Sidebar 会话，不影响其他 Mochat 端。Standalone Compose 仅为 Sidebar 依赖设置 `MOCHAT_API_BASE_URL` 与 `MOCHAT_SIDEBAR_BASE_URL`，没有改变 Dashboard 或 Operation 的默认地址策略。

安全修复还覆盖了客户字段写入原子性、跨企业标签拒绝、员工/企业边界、JSSDK 签名员工范围、回调标签同步失败重试和上下文白名单保留。

## 6. 浏览器与视觉验收

| 场景 | 结果 |
| --- | --- |
| 360×800 | PASS；12 路由可达，无横向溢出，紧凑功能块仍满足触控要求 |
| 390×844 | PASS；四个参考图映射工作区、保存/取消、加载/空态/错误态和长文本均验证 |
| 430×932 | PASS；12 路由可达，无遮挡与横向溢出 |
| 320×568 窄屏 | PASS；素材选择与固定底栏可用 |
| 844×390 横屏输入 | PASS；备注输入聚焦后不被横向裁切 |
| 390×480 键盘等价场景 | PASS；搜索输入与底栏不重叠 |
| 390×844 Playwright 终验 | PASS；四张映射截图为本轮新构建现场生成，控制台、页面错误、失败请求和异常响应均为 0 |
| in-app Browser 终验 | PASS；当前窗口 `clientWidth=scrollWidth`，无横向溢出，控制台 warning/error 为 0；已保留可点击审阅标签页 |
| 继续授权 | PASS；实际点击后进入客户工作台 |
| 客户→会话→任务→我的→通讯录 | PASS；真实 Go 挂载路径下查询参数不丢失 |

其他截图：

- `implemented-customer-360x800.png`
- `implemented-customer-430x932.png`
- `implemented-contacts-keyboard-equivalent-390x480.png`
- `docs/reviews/evidence/employee-sidebar-mobile/landscape-focused-remark-844x390.png`
- `docs/reviews/evidence/employee-sidebar-mobile/narrow-medium-320x568.png`

## 7. 自测结果

| 命令/检查 | 结果 | 证据摘要 |
| --- | --- | --- |
| `corepack pnpm --filter @mochat/sidebar test` | PASS | 14 文件，153/153 |
| `corepack pnpm --filter @mochat/sidebar typecheck` | PASS | `tsc --noEmit` 退出码 0 |
| `corepack pnpm --filter @mochat/sidebar lint` | PASS | ESLint 退出码 0 |
| `corepack pnpm --filter @mochat/sidebar build` | PASS | 生产构建成功；JS 371.19 KB，只有分包建议警告 |
| `node --test scripts/sidebar_review_server.test.mjs` | PASS | 13/13，含状态语义与真实分页夹具 |
| `node --test scripts/check_standalone_public_urls.test.mjs` | PASS | 3/3；仅注入 API 与 Sidebar URL |
| `corepack pnpm check:mobile-clients-foundation` | PASS | 59/59；Sidebar 12、Operation 10、direct fetch 0、fake outcomes 0 |
| `corepack pnpm --filter @mochat/e2e typecheck` | PASS | 退出码 0 |
| `corepack pnpm --filter @mochat/e2e lint` | PASS | ESLint 退出码 0 |
| `sidebar-employee-mobile.spec.ts` | PASS | 45/45，一次完整运行全绿 |
| `mobile-clients-foundation.spec.ts` | PASS | 56/56，一次完整运行全绿 |
| `sidebar-review-login.spec.ts` | PASS | 1/1，继续授权真实点击通过 |
| `go test ./internal/dashboard ./internal/store ./internal/server ./internal/config ./cmd/mochat-go -count=1` | PASS | 五个相关 Go 包全部通过 |
| `git diff --check` | PASS | 无空白错误 |
| Docker production image build | PASS | 最新 Sidebar bundle 已进入镜像 `sha256:84daeaff848ac487a2fa5c66ba28686d209f3076e4030df302c33c158f84bae1` |
| Docker health/HTTP | PASS | app/mysql/redis healthy；28080、28081 返回 200；新 API 未授权返回 401；容器仅含 API/Sidebar 两个公开 URL 默认值 |
| Docker 命名卷保护 | PASS | app-storage、audit-anchor-storage、mysql-data、redis-data 全部保留 |
| 真实企业微信 OAuth | SKIP | 验收库无 `mc_work_agent`，回调诚实返回“应用不存在”；未伪造 PASS |

本任务没有修改 `web/packages/mobile-foundation`，因此无需额外运行该包的变更门禁；根级移动端基础门禁和现有移动 E2E 已完整运行。

## 8. 提交列表

以下为基线至最终代码提交的 47 个提交；本报告由其后的文档提交保存。

```text
f3cf86b docs: design employee sidebar mobile optimization
5bdf032 docs: plan employee sidebar mobile optimization
3669353 feat(sidebar): add validated employee contact domain
265cca6 feat(sidebar): migrate employee mobile workflows
524e0ed test(sidebar): add employee mobile acceptance
11e3058 test(mobile): align group SOP foundation contract
486bd6f fix(sidebar): close employee mobile review gaps
d6ccc8b test(sidebar): refresh employee mobile evidence
7408c82 docs: record employee sidebar mobile acceptance
2bca48e fix(sidebar): enforce tenant boundaries across employee flows
2d032af test(sidebar): cover employee mobile business states
32ef7f8 docs: finalize employee sidebar acceptance
6b8be76 docs: design compact sidebar review flow
55b758e docs: plan compact sidebar review implementation
c27bc83 style(sidebar): tighten employee mobile density
85de3a6 test(sidebar): require visible workbench tiles
b883edb test(sidebar): share employee review fixtures
7698ec9 feat(sidebar): add local review server
b5cefcf fix(sidebar): enforce review loopback binding
6f467b2 fix(deploy): publish reachable mobile client urls
16de3a9 test(sidebar): refresh compact mobile evidence
8537d98 docs: record compact sidebar review acceptance
70f1e87 docs: complete compact sidebar verification record
52eb0c5 fix(sidebar): bind jssdk signing to employee scope
34006b9 fix(sidebar): reject cross-corp contact tags
47b58e0 fix(sidebar): enforce employee data consistency
dc4f3a4 fix(sidebar): preserve employee workflow context
c4ca0f4 fix(sidebar): align contextual mobile navigation
1144b34 fix(sidebar): close employee tenant sync boundaries
36d47fe fix(sidebar): preserve tag sync intent in workers
2b9b445 test(sidebar): refresh final browser evidence
c7b2b2c fix(sidebar): retry callback tag sync failures
11a2425 docs: finalize employee sidebar security acceptance
82eb676 docs: design high fidelity sidebar reference replica
8aee0fe docs: plan high fidelity sidebar implementation
3de2355 feat(sidebar): define employee workbench contracts
5542d07 feat(sidebar): query employee workbench data
1cb58ba feat(sidebar): expose employee workbench APIs
1a05ee4 feat(sidebar): rebuild employee mobile workspaces
ae22d50 test(sidebar): cover reference workspaces in browser
4e18a01 test(sidebar): align mobile foundation acceptance
0209771 docs(sidebar): refresh mobile acceptance evidence
7838ce3 test(sidebar): enforce three-tab workbench navigation
acbd9ab fix(sidebar): preserve workbench query navigation
f8d6736 docs(sidebar): report reference replica verification
6b62682 fix(sidebar): make employee workbench states truthful
4c82819 fix(deploy): scope standalone urls to sidebar dependencies
```

## 9. 已知风险、迁移与回滚

- 构建成功但 Sidebar 单 JS chunk 为 371.19 KB，超过 350 KB 提示阈值；不影响本次功能，后续可在单独性能任务中按页面拆包。
- 真实企微 OAuth、JSSDK 发消息和加好友仍依赖外部企业微信应用、域名与签名凭证；本次只对仓库可控入口和失败语义判定 PASS。
- 远端主线已推进 35 个提交。合入前应在备份分支上逐提交变基、重跑本报告全部门禁，不能直接合并未知主线结果。
- 回滚可按原子提交逆序执行；本轮评审修复回滚点为 `4c82819` 与 `6b62682`，高保真工作台此前的直接回滚点为 `acbd9ab`、`7838ce3`、`0209771`、`4e18a01`、`ae22d50`、`1a05ee4`、`1cb58ba`、`5542d07`、`3de2355`。回滚 API 时必须同时回滚前端调用与数据库查询契约。
- Docker 回滚只需重建目标提交的 app 镜像并 `up -d --no-deps app`；不得删除四个命名卷。
