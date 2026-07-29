# Phase 2：前端逐页迁移

## 状态

路由迁移范围已完成：审计清单中的 135/135 个页面均由 React 路由承接，Dashboard、Sidebar、Operation 的 manifest 中不再包含 legacy target。

这里的“完成”特指可由仓库确定性验证的路由、渲染、静态挂载、构建和发布链路，不代表未知真实业务数据与第三方集成已经完成等价性验收。后者记录在[真实业务验收债务](real-business-validation-debt.md)，后续可直接按文档追加场景和结果。

## 已纳入验收的范围

- Dashboard 61 条业务路由、Sidebar 12 条路由、Operation 10 条路由均由 manifest 驱动并切换至 React。
- 专用业务页继续使用各自 API 与交互组件；尚无可确定业务规格的页面由统一 React 页面模块承接。
- 三个应用均具有动态导入产物；Sidebar 和 Operation 同时支持独立根路径与 Go 子路径挂载。
- Playwright 强制每条迁移路由出现可见 React 内容，并验证 query/hash 保留；83 条 manifest 路由生成桌面与移动端共 166 张截图。
- Go 静态服务、默认 dist 配置及 Docker 构建/复制链路均指向 `web/apps/*/dist`。
- 旧 `web/dashboard/dist`、`web/sidebar/dist`、`web/operation/dist` 已退出运行时与版本库。

## 证据

- [Playwright 运行记录](evidence/playwright-run.json)
- [逐路由证据与截图哈希](evidence/route-evidence.json)
- [逐路由回滚记录](evidence/rollback-records.json)
- [构建体积与动态 chunk 审计](evidence/build-audit.json)
- [桌面与移动端截图](evidence/screenshots)
- [路由迁移进度](audit/phase2-progress.csv)

运行 `pnpm evidence:phase2` 会读取最近一次通过的 Playwright 结果、校验所有截图并重建带 SHA-256 的证据索引；`pnpm check:audit` 校验 React-only manifest、生产构建、证据完整性与 legacy 退出状态。

## 回滚边界

旧 dist 已退出，因此不能只把 manifest target 改回 `legacy`。逐路由记录给出基线提交和隔离恢复步骤：必须在恢复分支中同时取回对应 legacy 产物、运行时挂载与 manifest，然后重新执行完整安全、测试和构建门禁。仓库已验证路由隔离与静态挂载机制；生产环境恢复演练尚未执行，记录中明确标记为 `productionRestoreDrillExecuted: false`。
