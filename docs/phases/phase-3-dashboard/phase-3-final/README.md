# Phase 3 Final：Provider 接入与总验收

> 状态：2026-08-07 总验收闭合，53/53 达标，已合入并推送 `main`（`361b918`）

## 范围

Phase 3 Final 不做新页面开发，目标是让 53 页基准获得真实数据闭环并完成总验收：

1. 本地音频存储/读取 Provider + `/chat/file-audio` 真实页面（上传→落盘→回读→播放→软删）。
2. AI Provider（阿里云百炼 OpenAI 兼容协议）+ AI 洞察 5 页真实分析流（归档文本→AI→落库→页面回读）。
3. 企微会话存档 Provider 适配层（无真实凭证时 `limited`，含契约测试；真实凭证由用户提供后激活）。
4. 53 页全量门禁 + 浏览器/识图验收 + 跨页数据流证据 + 文档收口。

## 目录

- [`2026-08-07-phase3-final-provider-onboarding-design.zh-CN.md`](2026-08-07-phase3-final-provider-onboarding-design.zh-CN.md)：设计定稿（表结构、API 契约、配置项、验收标准）。
- [`2026-08-07-phase3-final-total-acceptance-report.md`](2026-08-07-phase3-final-total-acceptance-report.md)：总验收报告（门禁、部署、浏览器/识图、数据流证据）。
- [`tasks/`](tasks/)：子代理任务书（后端/前端，本次因子代理通道故障由主任务直接实现）。

## 关键交付

- 后端：`internal/modules/providers`（Audio/AI/WeComArchive）、`internal/modules/chat-media`（`/dashboard/chat/media*`）、AI 洞察真实化、迁移 `0126_phase3_final_providers`。
- 前端：`/chat/file-audio` native 页（选择文件→上传→播放→删除→分页）、manifest 状态 `native/ready/integration-passed`。
- 门禁：`pnpm check:phase3-final`（debt-clearance 27/27 + phase3-5 9/9）。
- 验收脚本：`web/e2e/phase3-final-acceptance.mjs`；验收产物在 `D:\workspace\mochat-go\output\phase3-final-browser-20260807\`。
- 测试期 AI API 默认关闭（`MOCHAT_GO_AI_INSIGHT_ENABLED=0`，即使配置 key 也不调用）；开启开关并配置 `MOCHAT_GO_AI_PROVIDER_KEY` 后走真实分析。
- AI 洞察与文件录音页面样式已按 phase35 参考页面收口（KPI 卡、友好类型显示、成功/错误提示条、AI 摘要换行）。

## 遗留

- 真实企微会话存档凭证：提供后激活适配层并做会话五页 + 风险事件浏览器级回读验收。
- 验收数据清理：`deploy/standalone/acceptance/cleanup_phase3_final.sql`（AI 分析、音频对象、归档种子）。
