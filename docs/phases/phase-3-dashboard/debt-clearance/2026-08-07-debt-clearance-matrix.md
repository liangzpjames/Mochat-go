# Phase 3 坏账清理矩阵

> 状态：已验收（2026-08-07）。`/chat/file-audio` 固定为 `Provider 阻塞`。
> 证据见 `2026-08-07-debt-clearance-acceptance.zh-CN.md`；截图与识图记录见 `D:\workspace\mochat-go\output\debt-clearance-browser\2026-08-07\`。

| 路由 | 领域 | 原状态 | 实现 | 门禁 | 浏览器·视觉 | 状态 |
| --- | --- | --- | --- | --- | --- | --- |
| `/chat/v2-staff` | 会话 | demo | native | 全绿 | 通过 | 已验收 |
| `/chat/v2-customer` | 会话 | demo | native | 全绿 | 通过 | 已验收 |
| `/chat/v2-group` | 会话 | demo | native | 全绿 | 通过 | 已验收 |
| `/chat/trajectory` | 会话 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/chat/export` | 会话 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/chat/file-audio` | 会话 | placeholder | — | — | — | Provider 阻塞 |
| `/chat/resign-staff` | 会话 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/chat/refuse-archive` | 会话 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/customer/inheritance` | 会话 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/v2/risk` | 风险预警 | demo | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/v2/timeout` | 风险预警 | demo | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/v2/customer-loss` | 风险预警 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/v2/message-intercept` | 风险预警 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/v2/keyword-library` | 风险预警 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/v2/silent-customer` | 风险预警 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/session-analysis` | AI 洞察 | demo | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/smart-analysis` | AI 洞察 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/emotion` | AI 洞察 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/employee-score` | AI 洞察 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/ai-insight/communication-keyword` | AI 洞察 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/ai-setting/ai-knowledge-base` | AI 设置 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/ai-setting/agent` | AI 设置 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/company-setting/website` | 企业设置 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/company-setting/staff` | 企业设置 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/setting/role` | 企业设置 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/setting/additional` | 企业设置 | placeholder | native | 全绿 | 通过 | 已验收 |
| `/setting/authorization` | 企业设置 | placeholder | native | 全绿 | 通过 | 已验收 |

## 验收记录

- 门禁：`go test ./...` 全绿；Dashboard 505 测试；typecheck/build 通过；`pnpm check:phase3-5-dashboard` 9/9。
- 浏览器：26/26 页截图无错误/占位符；DOM 证据 `evidence.json`。
- 识图：`qwen3-vl-plus` 严格审查 3 轮，新页面问题已清零（`vision-notes*.txt`）。
- 数据流：知识库/智能体创建→落库→回读闭环（`workflow.json` + MySQL 核验）。
