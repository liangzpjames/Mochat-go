# Task 6 交付报告

## 完成内容

- 将 AI 设置前端从单助手模型改为双助手模型，按 `systemKey` 区分“会话分析助手”和“智能分析助手”。
- 在 API 层增加真实合同解析与互斥规则校验：
  - `session-analysis` 仅接受 `sessionAnalysisRule`
  - `smart-analysis` 仅接受 `smartAnalysisRule`
  - 缺少固定助手、串字段或重复字段时返回诚实错误态
- 重写分析助手页面：
  - 双卡并排展示，窄屏自动纵向堆叠
  - 两张卡各自独立刷新、独立编辑、独立保存
  - 会话分析助手使用“客户分析提示词 / 员工质检提示词”双 textarea
  - 智能分析助手使用“智能分析目标 + 会话范围卡片 + 回看天数 + 最少消息数”
  - 两个编辑器 DTO 严格互斥，切换时 draft 不串改
  - 未保存时支持 Escape 二次确认
  - 知识库沿用安全关联逻辑，允许保留既有关联停用库，阻止新增其他停用库
- 为知识库页刷新按钮补上统一 `secondary action` 样式类，并通过样式合同测试锁定尺寸。
- 清理页面与测试中的“默认智能体 / 默认智能分析规则 / 同一个助手服务两页”等旧术语。

## 验证结果

- 定向 Vitest：
  - `pnpm exec vitest run src/features/ai-settings/ai-settings-api.test.ts src/features/ai-settings/ai-settings-pages.test.tsx src/styles/ai-settings-layout.test.ts`
  - 结果：3 个测试文件全部通过，26/26 通过
- Dashboard TypeScript：
  - `pnpm typecheck`
  - 结果：通过
- 任务范围 ESLint：
  - `pnpm exec eslint src/features/ai-settings/ai-settings-api.ts src/features/ai-settings/ai-settings-api.test.ts src/features/ai-settings/agent-page.tsx src/features/ai-settings/knowledge-base-page.tsx src/features/ai-settings/ai-settings-pages.test.tsx src/styles/ai-settings-layout.test.ts`
  - 结果：通过
- Diff 检查：
  - `git diff --check`
  - 结果：通过

## 影响文件

- `web/apps/dashboard/src/features/ai-settings/ai-settings-api.ts`
- `web/apps/dashboard/src/features/ai-settings/ai-settings-api.test.ts`
- `web/apps/dashboard/src/features/ai-settings/agent-page.tsx`
- `web/apps/dashboard/src/features/ai-settings/ai-settings-pages.test.tsx`
- `web/apps/dashboard/src/features/ai-settings/knowledge-base-page.tsx`
- `web/apps/dashboard/src/styles/ai-settings-layout.test.ts`
- `web/apps/dashboard/src/styles/index.css`

## 关注点

- 定向页面测试通过时，Ant Design Modal 在 JSDOM 下仍会输出 `window.getComputedStyle(..., pseudoElt)` 的既有 stderr 噪音，但不影响测试结果，当前也不是本任务新增失败项。
