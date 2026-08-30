# 2026-08-30 Phase 2 现行路由证据合同修复与本地验收

## 结论

Phase 2 的历史迁移进度与当前生产可达路由已经分离治理。历史 `migration-routes.json` 继续作为迁移完成记录；当前浏览器证据只覆盖运行时安全合同允许访问的路由：Dashboard 取历史迁移清单与当前 Yuanhu benchmark manifest 的交集，Sidebar 与 Operation 使用各自当前 manifest。现行集合共 23 条路由，完整 Playwright 套件共 25 项（另含未知路由 404 与 API 不回退 SPA），本地运行结果为 25/25 PASS。

本次证据来自本地 Go 静态服务、Playwright Chromium、正式测试 fixture 和受路径限制的测试会话，不调用真实企业微信、真实 AI Provider 或生产服务，也不代表这些外部环境已经验收。

## 根因

1. 原视觉套件与生成器把 2026 年早期的 Dashboard 历史深链当作当前可达路由。当前运行时只向 benchmark manifest 中存在且经服务端授权的页面发放有效权限；非 manifest 深链即使出现在旧迁移记录、甚至服务端返回权限，也会 fail closed 为 403。旧门禁将历史迁移范围误当作当前安全合同，因而要求无法合法生成的证据。
2. Sidebar/Operation 的视觉套件仍断言已经退出的 `react-migrated-page` 占位组件，没有执行当前真实路由、受保护会话和业务 API fixture，因此不能证明现行页面。
3. 构建证据生成器通过 `migrated-dashboard-page` 或 `page-` 文件名猜测动态 chunk。当前 Dashboard 的真实拆包名称已变化，Sidebar/Operation 当前又是合法的单入口 bundle，导致真实构建被误报失败。
4. 原证据行没有区分“历史记录”和“本次现行运行”，历史截图可以继续引用最近一次 Playwright 记录，存在把旧证据冒充当前证据的风险。

## 方案与理由

- 新增单一共享路由派生函数，视觉套件、证据生成器和审计门禁都复用同一集合，避免三处手工计数再次漂移。
- 保留全部历史证据行，但统一标记 `currentReachable=false`、`evidenceStatus=historical`、`playwrightRun=null`；只有本次实际执行的 23 条路由可标记为 current。
- Dashboard 增加显式安全回归：非 manifest 历史深链即使获得服务端权限仍必须 403，确保调整门禁不会恢复已关闭的不安全旧深链。
- Sidebar 使用既有 `sidebar-employee-review.json`、限定 `/sidebar-app` 的 token/agent Cookie 与 Go 响应 envelope；Operation 仅对现行任务宝合同提供本地 fixture。未声明的 Sidebar/Operation 请求一律返回 500，并由测试审计数组使套件失败，防止静态数组或兜底成功掩盖缺口。
- 每条现行移动端路由验证真实一级标题、关键模块文本、非 404、原 pathname 和 query/hash 后再截图；桌面和 390px 移动视口均记录非空 PNG 与 SHA-256。
- 正式运行器在启动前精确删除旧 `.last-run`、JSON reporter 和 23 条现行路由的 46 张目标截图；生成器再核对精确测试标题集合、结果文件时间和截图 mtime，异常启动或筛选子集不能给旧证据重新盖章。
- 构建证据改为读取 `dist/index.html` 和真实 `dist/assets`，逐项记录入口、JavaScript 产物、可选 lazy chunk、字节数与 SHA-256；生成器与审计复用同一纯派生函数，门禁现场重算并 `deepEqual`，不再用约定文件名或 JSON 自述推断拆包结果。
- 审计现场重算 46 张当前截图的 SHA-256 并与索引严格相等，截图被替换、损坏或索引陈旧都会失败。

## TDD 与故障证据

- RED：旧完整视觉套件列出 84 项，Dashboard 历史深链先返回 404；尝试把旧深链加入 known routes 后返回 403，证明权限层有意拒绝非 manifest 路由，随后撤销该错误方向。
- RED：动态期望数测试显示旧生成记录 `91 !== 84`，说明生成器仍硬编码历史计数。
- RED：切换到现行集合后，Sidebar 首批页面因陈旧 `react-migrated-page` 断言失败；页面实际已经进入当前登录/业务状态。
- RED：真实构建已存在，但生成器因找不到旧 `migrated-dashboard-page` 文件名报 `dashboard has no dynamic page chunk`。
- GREEN：共享现行集合为 Dashboard 1、Sidebar 12、Operation 10；完整视觉套件 `25 passed`。
- GREEN：构建证据测试确认三端均有真实 JavaScript 入口和逐资产哈希；Dashboard 记录 10 个实际 lazy chunk，Sidebar/Operation 如实记录 0 个。

## 本地证据

- Playwright：`corepack pnpm test:e2e:phase2-evidence`，25/25 PASS，本地 Go webServer 由仓库 Playwright 配置启动并在测试后退出；生成器校验 JSON reporter 中的唯一 spec、实际 25 个用例、逐项结果和报告 SHA-256，筛选子集不能生成全量证据。
- 证据生成：`corepack pnpm evidence:phase2`，输出 `routes=23`。
- Phase 2 审计：`corepack pnpm check:audit`，输出 `routes=23 legacy_targets=0 screenshots=46`。
- 历史进度治理：`corepack pnpm check:phase2-progress`，输出 `82/82 React (100.0%), 0 legacy`；这是历史迁移进度，不是当前可达路由数。
- Phase 2.1：`corepack pnpm check:phase2.1`，输出 `82 routes, 40 features`。
- Dashboard 当前路由桌面截图：75,824 bytes，SHA-256 `44c574f0561315956a1adb24f9fe9815c2058b45076d23c34beef68b8a744f41`。
- Dashboard 当前路由移动截图：162,916 bytes，SHA-256 `9c7249e731c4257c2f2506a3f544ea4e114a0c734df440bfb3dab3a42166d263`。
- 视觉抽检：Dashboard 企业资料桌面、Sidebar 客户资料移动端、Operation 任务宝桌面均有真实可见内容且没有 404/兜底成功页面。

## 明确边界

- fixture 只证明本地页面、会话、响应 envelope、路由与失败关闭合同，不证明真实企业微信授权、JSSDK、真实租户数据或生产网络。
- Dashboard 截图中的“待验证/待配置”是本地 fixture 下的真实空状态，不伪造企业绑定成功。
- `productionRestoreDrillExecuted` 保持 `false`；本次只验证隔离恢复步骤和门禁合同，没有执行生产恢复。
- 历史截图继续保留用于追溯，但没有绑定本次 `playwright-run.json`，不能作为当前可达性证据。
