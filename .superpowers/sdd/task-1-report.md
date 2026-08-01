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

## 审查问题修复（后续提交）

### 修复内容

- `validateFinalPhase32Manifest` 不再提供默认空矩阵；未传入或空白矩阵会报 `missing required Phase 3.2 function matrix`。
- 功能矩阵扩展为 13 列：对“合理合并”和“不适用”强制记录 `decisionReason`、`alternativeEntry`、`decisionVerification`；重复的 `page + referenceFeature`、Markdown 行列数错误和额外/重复的 Phase 3.2 路由都会失败。
- 所有闭合项的 `tests` 与 `evidence` 都必须指向存在的仓库相对文件。当前矩阵仍明确标记为待完成，未伪造业务证据。
- 完成页面门禁会扫描 `web/apps/dashboard/src/benchmark/page-registry.tsx` 中八个目标路由的实际注册，拒绝 `DemoPage`、`PlaceholderPage` 和 `demo-fixtures` 引用。
- pending 检测现覆盖 `待`、`TODO`、`TBD`、`pending`、`unfinished`、`not started`；无浏览器验收回归测试现在构造完整八页，并将其中一页设为 `unit-passed` 后明确断言 `/index` 缺少浏览器验收。

### RED / GREEN 记录（审查修复）

- RED：`node --test scripts/check_phase3_2_dashboard_completion.test.mjs`
  - 结果：`# pass 2`、`# fail 8`、退出码 `1`。失败覆盖空矩阵、决策记录、列数、重复项、pending 词、闭合文件、额外路由和真实 `DemoPage` 注册。
- GREEN：`node --test scripts/check_phase3_2_dashboard_completion.test.mjs`
  - 结果：`# pass 10`、`# fail 0`、退出码 `0`。
- 完整门禁：`node scripts/check_phase3_2_dashboard_completion.mjs`
  - 结果：退出码 `1`（预期）；逐项列出八页共 48 个未闭合字段，未将当前八页的 `unit-passed` 状态误报为完成。

### 补充源码扫描 RED / GREEN

- RED：新增 `PlaceholderPage` 与 `demo-fixtures` 路由注册扫描测试后运行 `node --test scripts/check_phase3_2_dashboard_completion.test.mjs`。
  - 结果：退出码 `1`，提示未导出 `validateCompletedPageSources`。
- GREEN：导出并复用该实际源码扫描器后重新运行同一命令。
  - 结果：`# pass 11`、`# fail 0`、退出码 `0`。

### 最终覆盖复跑

- `node --test scripts/check_phase3_2_dashboard_completion.test.mjs`
  - 结果：`# pass 13`、`# fail 0`、退出码 `0`；补回八页缺失与 fixture 闭合字段回归覆盖。

## 第二轮审查问题修复

### RED / GREEN 记录

- RED：先为矩阵额外路由、目标页 phase、跨行注册扫描和仓库文件路径约束增加回归测试，再运行 `node --test scripts/check_phase3_2_dashboard_completion.test.mjs`。
  - 初始结果：退出码 `1`，`isRepositoryFile` 尚未导出。
  - 导出并完成其余最小实现前的结果：`# pass 15`、`# fail 2`、退出码 `1`；剩余失败为目标页错误 phase 未被拒绝，以及当前 Windows 会话不允许创建文件符号链接（`EPERM`）。路径解析回归测试改用 Windows junction，以等价地覆盖重解析后留在仓库内和逃逸到仓库外的情形。
- GREEN：`node --test scripts/check_phase3_2_dashboard_completion.test.mjs`
  - 结果：`# pass 17`、`# fail 0`、退出码 `0`。
- 完整门禁：`node scripts/check_phase3_2_dashboard_completion.mjs`
  - 结果：退出码 `1`（预期），八个目标页各有 frontend、api、permission、persistence、tests、evidence 六项待完成，共 `48` 个缺口。

### 本轮变更

- `validateFunctionMatrix` 拒绝不在八个 `phase32TargetRoutes` 中的任何矩阵行。
- 目标 manifest 路由必须显式 `phase: "3.2"`；错误 phase 返回包含路径和实际 phase 的精确错误。
- `validateCompletedPageSources` 按目标路由键提取完整注册表达式，再扫描 `DemoPage`、`PlaceholderPage` 和从 `demo-fixtures` 导入的标识符；不再要求路由键与引用处于同一行。
- `isRepositoryFile` 仅接受仓库内相对普通文件，并在 `realpath` 解析后再次校验仓库边界和普通文件类型。
