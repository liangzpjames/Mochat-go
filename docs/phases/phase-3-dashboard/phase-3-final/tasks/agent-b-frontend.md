# 子代理 B 任务书：Phase 3 Final 前端（实现，不提交、不部署）

工作目录：`D:\workspace\mochat-go\mochat-go`（git 根目录）。设计契约以 `docs/phases/phase-3-dashboard/phase-3-final/2026-08-07-phase3-final-provider-onboarding-design.zh-CN.md` 为准，本任务书是范围与验收清单。

## 硬性约束

- 禁止 `git commit` / `git push` / `git add`；禁止启动或重建 Docker。
- 只改 `web/apps/dashboard/src` 前端、`scripts/check_debt_clearance.mjs`、以及对应测试；禁止修改 Go 后端（后端由子代理 A 负责；若发现契约冲突，在报告中记录并交给主任务仲裁）。
- 新增页面必须有 Vitest 测试；完成后运行 Dashboard 全量 Vitest、typecheck、build 必须通过。

## 1. `/chat/file-audio` 真实页面

新建 `web/apps/dashboard/src/features/phase35/file-audio-page.tsx`（风格沿用 phase35 卡片体系，按钮样式沿用线索池/phase35 统一按钮，翻页按钮用小号样式）：

- 标题“文件录音”，Phase35PageShell 包裹。
- 上传卡：`<input type="file" accept="audio/*">` + “上传”按钮；调用 `POST /dashboard/chat/media?corpId=`（multipart，字段 `file`）；展示成功/失败内联提示（含类型/大小校验错误文案）。
- 列表卡：表格列=文件名/类型/大小/上传时间/操作；行内 `<audio controls preload="none" src={playUrl}>`；“删除”按钮（软删，删除后刷新列表）。
- 分页：小号翻页按钮（沿用 phase35 分页组件/样式），每页 20 条。
- 空态/加载/错误：沿用 `Phase35DataState`。
- API 客户端：新建 `file-audio-api.ts`（或复用现有 client 模式），响应类型 `{list, total, page, perPage}`，`playUrl` 绝对路径直接给 `<audio>`。

## 2. 路由与注册

- `page-registry`：`/chat/file-audio` 改用新页面组件（替换 `Phase33OperationsPage`）；同步更新 `page-registry.test.tsx` 期望。
- `phase33-operations-page.tsx`：从 `phase33OperationConfigs` 移除 `/chat/file-audio`（保留其余 3 个）；同步更新 `phase33-operations-page.test.tsx`（其第 47/164 行附近对 file-audio 的断言改为新页面或删除）。
- `manifest.json`：`/chat/file-audio` 改为 `implementation:"native"`、`backend:"ready"`、`acceptance:"integration-passed"`、`screenshotVersion:"2026-08-07"`、`phase:"3-final"`，evidence.acceptance 指向 `docs/phases/phase-3-dashboard/phase-3-final/2026-08-07-phase3-final-total-acceptance-report.md`（该文件稍后由主任务创建，路径先写）。
- `scripts/check_debt_clearance.mjs`：`allowedIncomplete` 清空；输出改为“Phase 3 Final：53/53 达标”；其他既有规则（native/backend/acceptance 字段检查）不变。

## 3. AI 洞察前端小改

- `ai-insight-api.ts`：`AiInsightPageResult` 增加可选 `generatedAt?: string`；`data` 元素类型保持宽松。
- `ai-insight-pages.tsx`：`ready` 态表格已可用；如 `generatedAt` 存在，在结果区头部 chip 展示“生成时间：{generatedAt}”；受限文案与现状不变（后端已改为真实原因，前端无需硬编码）。
- 同步 `ai-insight-pages.test.tsx` 若断言受影响。

## 4. 必跑验证与交付证据

- Dashboard 全量 Vitest 通过（新增 file-audio 页面测试：上传成功/失败、列表渲染、播放 URL、删除、分页）。
- `pnpm typecheck`、生产 build（`pnpm build` 或仓库既有 build 命令）通过。
- `git diff --check` 干净。
- 完成后报告：改动文件清单、测试数、三条门禁输出摘要。
