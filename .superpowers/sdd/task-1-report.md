# Phase 3.2 Task 1 完成报告

## 实现内容

- 新增八个指定页面的原子功能矩阵，列出 `page`、`referenceFeature`、`decision`、`mochatEntry`、`frontend`、`api`、`permission`、`persistence`、`tests`、`evidence`。
- 新增 `validateFunctionMatrix(markdown)`：校验必需列、允许结论、必填执行字段和八页覆盖，并返回全部精确原因。
- 完成门禁会将矩阵与 manifest 交叉校验：`e2e-passed` 页面不得含 fixture 或待完成字段；完整阶段门禁会逐项拒绝未闭合功能。
- 将八个目标页面的 `acceptance` 从过早的 `e2e-passed` 降为 `unit-passed`，保留 `backend: ready`。

## RED / GREEN 记录

- RED：`node --test scripts/check_phase3_2_dashboard_completion.test.mjs`
  - 关键输出：`SyntaxError: ... does not provide an export named 'validateFunctionMatrix'`，退出码 `1`。
- GREEN：`node --test scripts/check_phase3_2_dashboard_completion.test.mjs`
  - 关键输出：`# pass 6`、`# fail 0`，退出码 `0`。
- 阶段门禁：`node scripts/check_phase3_2_dashboard_completion.mjs`
  - 关键输出：`Phase 3.2 unclosed function matrix items (48)`，退出码 `1`；八页各列出 frontend、api、permission、persistence、tests、evidence 六项待闭合内容，符合当前阶段必须失败的预期。

## 变更文件

- `docs/phase/phase-3.2-dashboard-completion/reports/phase3.2-function-matrix.md`
- `scripts/check_phase3_2_dashboard_completion.mjs`
- `scripts/check_phase3_2_dashboard_completion.test.mjs`
- `web/apps/dashboard/src/benchmark/manifest.json`
- `.superpowers/sdd/task-1-report.md`（本报告，用户明确要求的交付物）

## 自审

- 矩阵仅覆盖八个指定路由，不把其他页面纳入 Phase 3.2。
- 门禁单测覆盖缺列、非法结论、空证据、缺少七个目标页面和 e2e 页面仍使用 fixture。
- 完整门禁不会把未验证能力当作阶段完成；当前未闭合字段使用 `待 Task N` 明示并可被机器识别。
- 未暂存或修改工作树中已有的视觉相关改动。

## 关注点

- 完整阶段门禁当前有意失败，直到后续任务逐项用真实实现、测试和浏览器证据替换全部 48 个待闭合字段；Task 11 才能恢复 `e2e-passed`。
