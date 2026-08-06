# Phase 3.5 总验收报告

> 更新：2026-08-06 晚

当前状态：**九页代码、自动化门禁、重建部署与浏览器验收证据已闭合；阶段整体可进入合入评审**。仍有三项待补证（P35-ACCEPT 生命周期执行记录、Playwright E2E spec、真实 Provider 数据）。

## 已闭合证据（2026-08-06 实测）

- 门禁：`go test ./...` 通过；Dashboard 476 测试、typecheck、生产构建通过；`pnpm check:phase3-5-dashboard` 9/9；`git diff --check` 干净。
- 部署：`scripts/deploy_docker_desktop.ps1` 重建 `mochat-go-desktop`（保留全部数据卷），迁移应用至 `0123_phase35_order_collation_align`，`/readyz` 200，三容器 healthy。
- 浏览器：九页截图（`D:\workspace\mochat-go\output\phase35-browser-20260806\`），Playwright live-acceptance 脚本 + 视觉模型逐页核验；全部页面无错误/空白，菜单高亮与页面一致。
- 跨页工作流：创建订单（`P35-ACCEPT-验收订单`，¥128）→ 状态流转为已支付 → 转化漏斗订单 1、综合报表订单经营 1、行为审计 8 条。
- 修复：collation 混用（迁移 0123）、transition 路由 PUT/PATCH、报表默认日期偏移、客户明细直出 ID、受限文案重复。

## 待补证项

1. ~~`P35-ACCEPT-*` 数据生命周期~~：2026-08-06 晚已执行 create/verify/cleanup 并留证（审计表 3 条记录，清理为软删除）。
2. ~~Playwright E2E spec~~：`web/e2e/tests/phase35-live.spec.ts` 已入库，真实部署 2 项通过。
3. 好友/客户群/会话分析依赖真实企微与会话存档 Provider，当前为空态/受限态；`/chat/file-audio` 仍按 Phase 3.3 结论保持未完成。

## 验收数据说明

验收数据（`P35-ACCEPT-*` 联系人/订单）创建于运行库 tenant 1 / corp `1536612155`，属测试数据。其中经 PowerShell 创建的 `P35-ACCEPT-API??` 联系人名称含 `?`，为控制台编码问题，清理步骤见主线分析文档。
