# Phase 3.2 八页验收记录

## 验收范围

Phase 3.2 仅验收以下八个路由：`/index`、`/chat/v2-all`、`/ai-insight/v2/sensitive-word`、`/customer/clue/default`、`/customer/contact`、`/customer/opportunity`、`/customer/public-sea`、`/customer/tags`。页面功能对应关系见 `docs/phase/phase-3.2-dashboard-completion/reports/phase3.2-function-matrix.md`。

## 当前结论

- 八页代码、代表性单元测试、后端集成测试和 Dashboard 构建证据已在 Task 3～10 报告及 `../reports/tasks/task-11-report.md` 汇总。
- `manifest.json` 的八页状态统一为 `integration-passed`，截图状态保持 `unobserved`。
- Playwright 规范已覆盖固定八页的关键合同：查询/详情、导出、创建、阶段推进、跟进、领取和标签维护，并在 mock 后端中验证刷新后再次读取。
- 该 Playwright 规范使用合成会话和 mock API，只用于代码合同回归，不属于真实登录态浏览器验收，也不产生正式截图证据。

## 非浏览器门禁

非浏览器门禁使用：

```bash
node --test scripts/check_phase3_2_dashboard_completion.test.mjs
node scripts/check_yuanhu_benchmark_manifest.mjs
node scripts/check_phase3_2_dashboard_completion.mjs --non-browser
pnpm --filter @mochat/e2e typecheck
```

Task11 本轮仅执行上述短命令与静态检查；耗时的 Dashboard/Go 全量回归由控制器执行，结果不得由本记录预先宣称。

## 浏览器验收：blocked

真实登录态浏览器验收当前明确为 **blocked**。本地服务可访问，但没有可用的管理员登录凭据；已知 smoke 默认账号 `13800000000 / secret123` 被本地服务拒绝。因而以下项目均未标记通过：

- 真实登录、菜单进入、直接 URL、刷新、前进后退；
- 双租户、双企业及本人/协作人/部门/企业数据范围；
- 八页 1440×1000 默认态与关键交互态截图；
- 真实 API 写入后刷新读取；
- Phase 3.2 Playwright browser/e2e 最终验收。

仓库中既有 `docs/phase/phase-3.2-dashboard-completion/evidence/*.png` 不作为本次真实登录证据引用。获得有效本地管理员凭据后，应重新执行浏览器验收、生成截图并人工复核溢出、遮挡、错位、乱码和视觉一致性；只有届时才能把 manifest 从 `integration-passed` 更新为 `e2e-passed`。

## 合并与不适用项

本轮八条矩阵记录均为“已对应”，没有新增“合理合并”或“不适用”结论。
