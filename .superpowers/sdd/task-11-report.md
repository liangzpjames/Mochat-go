# Task 11 非浏览器收敛报告

## 范围与状态

- 基线：`a76da2d`
- 范围：Phase 3.2 固定八页的 e2e 合同、功能矩阵、acceptance、manifest 与完成门禁。
- 非浏览器状态：已实现，验证结果见下文。
- 浏览器状态：**blocked**。本地没有可用管理员登录凭据，未运行或宣称真实登录 browser/e2e 通过，未生成正式截图。

## 八页证据索引

| 路由 | unit | integration | build | browser |
| --- | --- | --- | --- | --- |
| `/index` | `internal/dashboard/corp_data_test.go` | `.superpowers/sdd/task-3-report.md` | Task 3 报告中的 Dashboard typecheck/build | blocked：无有效真实登录凭据 |
| `/chat/v2-all` | `internal/dashboard/auto_tag_dashboard_test.go` | `.superpowers/sdd/task-4-report.md` | Task 4 报告中的 Dashboard typecheck/build | blocked：无有效真实登录凭据 |
| `/ai-insight/v2/sensitive-word` | `internal/dashboard/sensitive_word_test.go` | `.superpowers/sdd/task-5-report.md` | Task 5 报告中的 Dashboard typecheck | blocked：无有效真实登录凭据 |
| `/customer/clue/default` | `web/apps/dashboard/src/features/scrm/lead-page.test.tsx` | `.superpowers/sdd/task-6-report.md` | Task 6 报告中的 Dashboard typecheck | blocked：无有效真实登录凭据 |
| `/customer/contact` | `web/apps/dashboard/src/features/scrm/contact-page.test.tsx` | `.superpowers/sdd/task-7-report.md` | Task 7 报告中的 Dashboard typecheck | blocked：无有效真实登录凭据 |
| `/customer/opportunity` | `web/apps/dashboard/src/features/scrm/opportunity-page.test.tsx` | `.superpowers/sdd/task-8-report.md` | Task 8 报告中的 Dashboard typecheck | blocked：无有效真实登录凭据 |
| `/customer/public-sea` | `web/apps/dashboard/src/features/scrm/public-pool-page.test.tsx` | `.superpowers/sdd/task-9-report.md` | Task 9 报告中的 Dashboard typecheck | blocked：无有效真实登录凭据 |
| `/customer/tags` | `web/apps/dashboard/src/features/scrm/tag-page.test.tsx` | `.superpowers/sdd/task-10-report.md` | Task 10 报告中的 Dashboard typecheck/build | blocked：无有效真实登录凭据 |

## Task11 变更

- `phase3-2-dashboard.spec.ts` 改为恰好八个路由合同；写操作使用可持久化 mock 状态，并在刷新后再次读取。
- 删除 mock e2e 自动写入正式截图的行为，避免把合成会话证据误当真实登录验收。
- 完成门禁新增八页 e2e 合同静态校验和 `--non-browser` 模式；最终模式继续严格要求 `e2e-passed`。
- 功能矩阵只保留八页，manifest 八页标为 `integration-passed`/`unobserved`，证据路径改为实际存在的设计与中文验收文档。
- 中文 acceptance 明确区分非浏览器证据与真实登录 browser/e2e 证据。
- 审计并纳入实施计划中已确认的 Task 1 门禁语义修订：完整阶段门禁仅允许在 Task11 最终验收后通过。

## 本轮短命令验证

- `node --test scripts/check_phase3_2_dashboard_completion.test.mjs`：22/22 通过。
- `pnpm --filter @mochat/e2e typecheck`：通过。
- 最终门禁预期仍失败：八页没有 `e2e-passed`，这是 browser blocked 的正确结果，不得改写为通过。
- Dashboard/Go 耗时全量回归：按控制器要求不在本轮重复执行。

## 工作区隔离

本提交只精确暂存 Task11 docs/test/gate 文件。控制器未提交的 `internal/dashboard/saas_admin_system_health.go` 与 `internal/dashboard/saas_admin_test.go` 不在提交中。
