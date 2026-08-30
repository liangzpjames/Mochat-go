# Phase 2：前端逐页迁移

## 状态

历史路由迁移清单已完成 React 承接；当前可达范围则以运行时安全合同为准。Dashboard 只把“历史迁移清单与当前 Yuanhu benchmark manifest 的交集”视为现行路由，Sidebar 与 Operation 继续直接使用各自当前 manifest。历史证据保留，但不得冒充当前可达证据。

这里的“完成”特指可由仓库确定性验证的路由、渲染、静态挂载、构建和发布链路，不代表未知真实业务数据与第三方集成已经完成等价性验收。后者记录在[真实业务验收债务](real-business-validation-debt.md)，后续可直接按文档追加场景和结果。

## 已纳入验收的范围

- 当前可达集合为 Dashboard 1 条、Sidebar 12 条、Operation 10 条；Dashboard 未进入当前 benchmark manifest 的历史深链继续 fail closed（403）。
- 专用业务页继续使用各自 API 与交互组件；尚无可确定业务规格的页面由统一 React 页面模块承接。
- Dashboard 具有按页面拆分的异步产物；Sidebar 与 Operation 当前为单入口 bundle，并支持独立根路径与 Go 子路径挂载。构建审计记录真实入口、全部 JavaScript 产物、可选异步 chunk、字节数与 SHA-256，不用文件名猜测拆包结果。
- Playwright 强制当前每条可达路由出现与正式 fixture 合同一致的可见内容，并验证 query/hash 保留；23 条现行路由生成桌面与移动端共 46 张当前截图。
- Go 静态服务、默认 dist 配置及 Docker 构建/复制链路均指向 `web/apps/*/dist`。
- 旧 `web/dashboard/dist`、`web/sidebar/dist`、`web/operation/dist` 已退出运行时与版本库。

## 证据

- [Playwright 运行记录](evidence/playwright-run.json)
- [逐路由证据与截图哈希](evidence/route-evidence.json)
- [逐路由回滚记录](evidence/rollback-records.json)
- [构建入口、产物清单、体积与哈希审计](evidence/build-audit.json)
- [桌面与移动端截图](evidence/screenshots)
- [路由迁移进度](audit/phase2-progress.csv)

先运行 `pnpm test:e2e:phase2-evidence` 完整执行现行套件并生成 JSON reporter，再运行 `pnpm evidence:phase2`；生成器会校验唯一 spec、实际用例数、逐项结果、所有截图和构建产物，并重建带 SHA-256 的证据索引。`pnpm check:audit` 校验 React-only manifest、现行安全路由、生产构建、证据完整性与 legacy 退出状态。

## 回滚边界

旧 dist 已退出，因此不能只把 manifest target 改回 `legacy`。逐路由记录给出基线提交和隔离恢复步骤：必须在恢复分支中同时取回对应 legacy 产物、运行时挂载与 manifest，然后重新执行完整安全、测试和构建门禁。仓库已验证路由隔离与静态挂载机制；生产环境恢复演练尚未执行，记录中明确标记为 `productionRestoreDrillExecuted: false`。
