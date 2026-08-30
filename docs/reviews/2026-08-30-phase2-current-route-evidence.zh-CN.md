# 2026-08-30 Phase 2 现行路由证据合同修复与本地验收

## 结论

Phase 2 的历史迁移进度与当前生产可达路由已经分离治理。历史 `migration-routes.json` 继续作为迁移完成记录；当前浏览器证据只覆盖运行时安全合同允许访问的路由：Dashboard 取历史迁移清单与当前 Yuanhu benchmark manifest 的交集，Sidebar 与 Operation 使用各自当前 manifest。现行集合共 23 条路由，完整 Playwright 套件共 25 项（另含未知路由 404 与 API 不回退 SPA），本地运行结果为 25/25 PASS。

本次证据来自本地 Go 静态服务、Playwright Chromium、正式测试 fixture 和受路径限制的测试会话，不调用真实企业微信、真实 AI Provider 或生产服务，也不代表这些外部环境已经验收。

## 根因

1. 原视觉套件与生成器把 2026 年早期的 Dashboard 历史深链当作当前可达路由。当前运行时只向 benchmark manifest 中存在且经服务端授权的页面发放有效权限；非 manifest 深链即使出现在旧迁移记录、甚至服务端返回权限，也会 fail closed 为 403。旧门禁将历史迁移范围误当作当前安全合同，因而要求无法合法生成的证据。
2. Sidebar/Operation 的视觉套件仍断言已经退出的 `react-migrated-page` 占位组件，没有执行当前真实路由、受保护会话和业务 API fixture，因此不能证明现行页面。
3. 构建证据生成器通过 `migrated-dashboard-page` 或 `page-` 文件名猜测动态 chunk。当前 Dashboard 的真实拆包名称已变化，Sidebar/Operation 当前又是合法的单入口 bundle，导致真实构建被误报失败。
4. 原证据行没有区分“历史记录”和“本次现行运行”，历史截图可以继续引用最近一次 Playwright 记录，存在把旧证据冒充当前证据的风险。
5. 原正式证据 runner 只清理旧 Playwright 报告和截图，未先执行生产构建。上一轮在撤销错误的 Dashboard `knownRoutes` 实验后，源码已恢复安全合同，但遗留 `dist` 仍包含错误深链；生成器忠实记录了该旧 bundle，导致后续 `check:audit` 在当前源码重新构建后出现 21 字节漂移。根因是构建输入、构建时间与浏览器证据之间没有 provenance 约束，不是 Vite 非确定性。
6. 第一版源码指纹直接哈希工作树原始字节。在 Windows `core.autocrlf=true` 下，同一 Git blob 可在既有工作树中呈现混合 LF/CRLF，在全新 worktree 中呈现完整 CRLF；候选工作树代表文件实测为 34 行 CRLF、84 行 LF，raw hash 与 Git clean-filter blob hash不同。因此证据虽然绑定同一 tree，换工作树仍会误报源码漂移。与此同时，缺少现场 `dist` 时旧审计只报“没有 JavaScript entry”，未明确指出需要先运行正式 build-before-audit runner。

## 方案与理由

- 新增单一共享路由派生函数，视觉套件、证据生成器和审计门禁都复用同一集合，避免三处手工计数再次漂移。
- 保留全部历史证据行，但统一标记 `currentReachable=false`、`evidenceStatus=historical`、`playwrightRun=null`；只有本次实际执行的 23 条路由可标记为 current。
- Dashboard 增加显式安全回归：非 manifest 历史深链即使获得服务端权限仍必须 403，确保调整门禁不会恢复已关闭的不安全旧深链。
- Sidebar 使用既有 `sidebar-employee-review.json`、限定 `/sidebar-app` 的 token/agent Cookie 与 Go 响应 envelope；Operation 仅对现行任务宝合同提供本地 fixture。未声明的 Sidebar/Operation 请求一律返回 500，并由测试审计数组使套件失败，防止静态数组或兜底成功掩盖缺口。
- 每条现行移动端路由验证真实一级标题、关键模块文本、非 404、原 pathname 和 query/hash 后再截图；桌面和 390px 移动视口均记录非空 PNG 与 SHA-256。
- 正式运行器在启动前精确删除旧 `.last-run`、JSON reporter 和 23 条现行路由的 46 张目标截图；生成器再核对精确测试标题集合、结果文件时间和截图 mtime，异常启动或筛选子集不能给旧证据重新盖章。
- 构建证据改为读取 `dist/index.html` 和真实 `dist/assets`，逐项记录入口、JavaScript 产物、可选 lazy chunk、字节数与 SHA-256；生成器与审计复用同一纯派生函数，门禁现场重算并 `deepEqual`，不再用约定文件名或 JSON 自述推断拆包结果。
- 审计现场重算 46 张当前截图的 SHA-256 并与索引严格相等，截图被替换、损坏或索引陈旧都会失败。
- 正式 `test:e2e:phase2-evidence` runner 现在自行先执行当前源码的 `corepack pnpm build`；构建失败立即关闭，不启动 Playwright。成功后记录三端前端源码输入 SHA-256、构建开始/完成时间，并验证所有实际 `dist` 文件均由本次构建产生。
- 构建 marker 位于被忽略的 `.tmp-phase2-evidence`，避免 Playwright 启动时清理自身 `test-results` 目录造成 marker 丢失。生成器要求 Playwright 开始时间晚于构建完成时间，并把 provenance 与实际 bundle 清单一并写入 `build-audit.json`；审计在 checkout 中重新计算当前源码指纹和全部 bundle 哈希。
- 源码指纹现在仅对已知文本源码扩展名做 `CRLF/CR -> LF` canonicalization，二进制资产继续按原始字节哈希；这样换行展开不影响同内容指纹，真实内容变化仍改变指纹。审计派生产物前先显式验证三端 `dist/index.html` 与 `dist/assets`，缺失时直接提示运行 `test:e2e:phase2-evidence`，不会把“尚未构建”伪装成资产合同失败。

## TDD 与故障证据

- RED：旧完整视觉套件列出 84 项，Dashboard 历史深链先返回 404；尝试把旧深链加入 known routes 后返回 403，证明权限层有意拒绝非 manifest 路由，随后撤销该错误方向。
- RED：动态期望数测试显示旧生成记录 `91 !== 84`，说明生成器仍硬编码历史计数。
- RED：切换到现行集合后，Sidebar 首批页面因陈旧 `react-migrated-page` 断言失败；页面实际已经进入当前登录/业务状态。
- RED：真实构建已存在，但生成器因找不到旧 `migrated-dashboard-page` 文件名报 `dashboard has no dynamic page chunk`。
- RED：旧 build-audit 合同没有 provenance API；陈旧产物 mtime、源码指纹变化、Playwright 早于构建完成三种场景均被新增测试拒绝。
- RED：marker 首次放在 `web/e2e/test-results` 后被 Playwright 生命周期清理；回归测试要求 marker 必须位于独立临时目录，随后完整套件重跑验证。
- RED：相同 fixture 分别写为 LF 与 CRLF 后，旧实现得到不同指纹；空 worktree 调用 build audit 只得到 `dashboard has no JavaScript entry asset`，两项新增测试均按预期失败。
- GREEN：canonicalization 后 LF/CRLF 指纹相同而内容变更指纹不同；缺失 `dist` 返回明确 build-before-audit 指引。完整 Phase 2 单测 10/10 PASS。
- GREEN：共享现行集合为 Dashboard 1、Sidebar 12、Operation 10；完整视觉套件 `25 passed`。
- GREEN：构建证据测试确认三端均有真实 JavaScript 入口和逐资产哈希；Dashboard 记录 10 个实际 lazy chunk，Sidebar/Operation 如实记录 0 个。

## 本地证据

- Playwright：`corepack pnpm test:e2e:phase2-evidence`，先完整执行 12/13 workspace 的生产 build，再运行 25/25 PASS；本地 Go webServer 由仓库 Playwright 配置启动并在测试后退出。最终构建 provenance 为 canonical 源码指纹 `4fbd2cc5f235ba4052a63821653d97cfce54fd37e53eb581731f2831911c72f9`，构建时间 `2026-08-30T11:23:40.627Z` 至 `2026-08-30T11:24:01.580Z`，Playwright 于 `2026-08-30T11:24:02.797Z` 启动；生成器校验 JSON reporter 中的唯一 spec、实际 25 个用例、逐项结果和报告 SHA-256，筛选子集不能生成全量证据。
- 证据生成：`corepack pnpm evidence:phase2`，输出 `routes=23`。
- Phase 2 审计：`corepack pnpm check:audit`，输出 `routes=23 legacy_targets=0 screenshots=46`。
- 历史进度治理：`corepack pnpm check:phase2-progress`，输出 `82/82 React (100.0%), 0 legacy`；这是历史迁移进度，不是当前可达路由数。
- Phase 2.1：`corepack pnpm check:phase2.1`，输出 `82 routes, 40 features`。
- Dashboard 当前路由桌面截图：75,824 bytes，SHA-256 `44c574f0561315956a1adb24f9fe9815c2058b45076d23c34beef68b8a744f41`。
- Dashboard 当前路由移动截图：162,916 bytes，SHA-256 `9c7249e731c4257c2f2506a3f544ea4e114a0c734df440bfb3dab3a42166d263`。
- 视觉抽检：Dashboard 企业资料桌面、Sidebar 客户资料移动端、Operation 任务宝桌面均有真实可见内容且没有 404/兜底成功页面。

## 关联红门禁的合同迁移依据

- Phase 3.5：`/chat/file-audio` 已由当前产品合同纳入 Phase 3.5，manifest 的 `phase` 精确改为 `3.5`；门禁继续使用 phase 白名单和 10/10 exact-set，没有扩大允许值或恢复旧逻辑。
- Dashboard Page RBAC：迁移 0176 是 0173 基线后的正式权限 overlay。完成门禁现在按迁移顺序消费 0176 后的最终集合并继续 exact-set 比对，避免把合法新增权限误判为越界，也不接受未注册权限。
- Phase 3.2 MySQL：旧门禁复制 0104 局部 schema，却用当前 0176 后代码执行，造成历史快照与现行模型语义冲突；改为启动无 volume 的一次性 MariaDB 10.6 空实例，通过 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 调用正式 0001–最新 migration registry，在随机隔离 schema 中 seed 场景并回滚清理。runner 强制 `-tags=integration`，解析 `go test -json`，只有 9 个明确场景全部实际 run 且 pass 才成功；静态测试同时禁止 Compose、`down -v`、mount 和任何 Docker volume。本门禁验证当前实现，不冒充独立的 MySQL 5.7 全量迁移验收。
- Phase 3.2 浏览器：旧断言引用已退出的筛选器、标题和敏感词表单文案；测试改为当前生产页面真实可见合同和真实交互闭环，最终 Playwright 8/8 PASS，没有通过静态数组或兜底成功降级。
- Yuanhu Phase 1：默认 fixture 只声明 `/index` 与 `/workContact/index`，但用例还验证 `/chat/v2-all` 分组；修复为该用例显式声明两条所需 menu route，未知/未授权路由的安全过滤保持不变，完整门禁最终 1/1 PASS。
- Dashboard 导航搜索：全仓 workspace 并发下，输入事件后立即同步查询链接会与 React 的异步结果提交竞态；失败测试先复现 969/970，再改为等待目标链接真实出现。随后在同等并发负载下全仓测试和两次独立 Dashboard 970 套件均通过，未增加 sleep 或放宽断言。

## 明确边界

- fixture 只证明本地页面、会话、响应 envelope、路由与失败关闭合同，不证明真实企业微信授权、JSSDK、真实租户数据或生产网络。
- Dashboard 截图中的“待验证/待配置”是本地 fixture 下的真实空状态，不伪造企业绑定成功。
- `productionRestoreDrillExecuted` 保持 `false`；本次只验证隔离恢复步骤和门禁合同，没有执行生产恢复。
- 历史截图继续保留用于追溯，但没有绑定本次 `playwright-run.json`，不能作为当前可达性证据。
