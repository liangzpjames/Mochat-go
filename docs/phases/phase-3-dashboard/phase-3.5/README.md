# Phase 3.5：SCRM 扩展与数据报表

> 同步时间：2026-08-06 晚

## 阶段范围

9 个路由：`/customer/friends`、`/customer/group`、`/customer/order`、`/customer/settings`（SCRM 扩展 4 页）与 `/data/customer`、`/data/employee`、`/data/conversion`、`/data/behavior`、`/data/report`（真实数据报表 5 页）。

## 当前状态

- **代码与自动化完成**：九页全部 `native`；`go test ./...`、Dashboard 476 测试、typecheck、生产构建、`pnpm check:phase3-5-dashboard`（9/9）2026-08-06 实测通过。
- **部署与浏览器验收已闭合**：重建 `mochat-go-desktop`（保留数据卷），迁移应用至 `0123_phase35_order_collation_align`；九页截图 + 视觉模型核验通过；订单跨页工作流（创建 → 已支付 → 转化漏斗/综合报表联动）已验证。
- **待补证**：`P35-ACCEPT-*` 生命周期执行记录与 Playwright E2E spec 已于 2026-08-06 晚补齐；剩余为真实 Provider 数据；`/chat/file-audio` 按 Phase 3.3 结论保持未完成。

## 关键文档

- 设计：[产品化返工设计](design/2026-08-05-phase3.5-productization-design.md)、[SCRM 报表设计](design/2026-08-05-phase3.5-scrm-reporting-design.md)
- 计划：[产品化计划](plans/2026-08-05-phase3.5-productization-plan.md)、[SCRM 报表计划](plans/2026-08-05-phase3.5-scrm-reporting-plan.md)
- 进度：[功能矩阵](reports/phase3.5-function-matrix.md)、[总验收报告](reports/phase3.5-total-acceptance-report.md)、[复用审计](reports/reuse-audit.md)
- 验收：[非浏览器验收记录](acceptance/PHASE3.5_ACCEPTANCE.zh-CN.md)
- 全局：[主线进度分析](../../../2026-08-06-mainline-progress-analysis.zh-CN.md)、[主目标与执行路线](../../../2026-08-06-main-goals-and-dashboard-roadmap.zh-CN.md)

## 验收数据

`scripts/phase35_acceptance_data.ps1`（create/verify/cleanup，仅命中 `P35-ACCEPT-` 前缀）与 `scripts/phase35_acceptance_data.test.mjs`。运行库执行证据待补齐。
