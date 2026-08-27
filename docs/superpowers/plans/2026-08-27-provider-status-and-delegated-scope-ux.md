# Provider 状态与第三方能力范围体验 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把唯一企业资料的运行状态改成业务摘要，隐藏第三方模式的企微配置卡，并让 SaaS 以默认全选的中文选项维护第三方能力范围。

**Architecture:** Provider API 合同保持不变，只在 Dashboard 展示层建立受控中文投影；第三方模式通过现有 `wecomIntegrationMode` 条件完全省略企微配置区。SaaS 在页面模块内维护稳定能力目录，将勾选值转换为既有 scope 数组，未知历史 scope 在保存时原样保留。

**Tech Stack:** React 18、TypeScript、TanStack Query、Vitest、Testing Library、Vite、Go/Docker Compose。

## Global Constraints

- 用户审阅文档使用中文。
- 不回显或记录永久授权码及其他敏感凭据。
- 不改变既有租户的企微对接模式或静默扩大历史授权。
- 通过当前验收 compose project 重建必要服务，不覆盖 MySQL、Redis 或数据卷。
- 使用测试先行；提交不得包含 `.superpowers/sdd/progress.md`。

---

### Task 1: Dashboard 业务化服务状态

**Files:**
- Modify: `web/apps/dashboard/src/features/provider-status/provider-status-page.tsx`
- Test: `web/apps/dashboard/src/features/provider-status/provider-status-page.test.tsx`

**Interfaces:**
- Consumes: `ProviderStatusApi.getStatus(): Promise<ProviderStatusView>`。
- Produces: `ProviderStatusPage` 的业务化中文状态摘要。

- [ ] **Step 1: 写失败测试**：断言四类服务只显示中文名称、业务状态和受控提示，并断言 raw code、source、capability、reason、missing、时间和错误码均不存在。
- [ ] **Step 2: 运行目标测试确认失败**：`pnpm --filter @mochat/dashboard test -- provider-status-page.test.tsx`；预期旧页面仍能找到技术字段。
- [ ] **Step 3: 实现最小展示投影**：把状态标题改为“服务运行状态”，将 `ready/limited/unavailable` 投影为“运行正常/需要完善/暂不可用”，只渲染服务名、状态和受控中文提示。
- [ ] **Step 4: 运行目标测试确认通过**：重复 Step 2 命令，预期全部通过。

### Task 2: 第三方模式隐藏 Dashboard 企微配置区

**Files:**
- Modify: `web/apps/dashboard/src/features/company-settings/website-page.tsx`
- Test: `web/apps/dashboard/src/features/company-settings/company-settings-pages.test.tsx`

**Interfaces:**
- Consumes: `CompanyProfile.wecomIntegrationMode`。
- Produces: 第三方模式仅保留企业身份和审计，不产生自建或托管占位配置卡。

- [ ] **Step 1: 写失败测试**：第三方 profile 下断言页面不出现“企微对接”“企微配置由 SaaS 平台维护”，且回调和同步 API 未调用。
- [ ] **Step 2: 运行目标测试确认失败**：`pnpm --filter @mochat/dashboard test -- company-settings-pages.test.tsx`；预期旧占位卡导致失败。
- [ ] **Step 3: 删除第三方占位分支**：仅在非第三方模式渲染自建应用、回调、会话存档、验证和员工同步区。
- [ ] **Step 4: 运行目标测试确认通过**：重复 Step 2 命令，预期全部通过。

### Task 3: SaaS 中文能力选择器与默认全开

**Files:**
- Modify: `web/apps/saas-admin/src/pages/TenantsPage.tsx`
- Test: `web/apps/saas-admin/src/pages/TenantsPage.test.tsx`

**Interfaces:**
- Consumes: `WeComIntegrationRecord.scope: string[]`。
- Produces: `WeComIntegrationForm.scope: string[]` 与 `saveDelegatedWeComIntegration` 的稳定 scope payload。

- [ ] **Step 1: 写失败测试**：首次配置断言“会话内容与媒体归档”“通讯录与客户资料”默认选中、无 textarea 和内部标识；保存断言 payload 为 `['archive.read','contacts.read']`。
- [ ] **Step 2: 写兼容测试**：已有 `['archive.read']` 时只勾选对应项，摘要显示中文能力和“已开启 1 项”，不显示 raw scope。
- [ ] **Step 3: 运行目标测试确认失败**：`pnpm --filter @mochat/saas-admin test -- TenantsPage.test.tsx`；预期旧文本框和 raw scope 导致失败。
- [ ] **Step 4: 实现能力目录和选择器**：新增两个中文能力定义、全选操作、至少一项校验、未知历史 scope 保留以及中文摘要。
- [ ] **Step 5: 运行目标测试确认通过**：重复 Step 3 命令，预期全部通过。

### Task 4: 工程门禁、Docker 与浏览器验收

**Files:**
- Modify: `docs/superpowers/reports/2026-08-27-wecom-archive-saas-activation-final-report.zh-CN.md`

**Interfaces:**
- Consumes: Tasks 1–3 的构建产物。
- Produces: 可访问的本地服务、真实浏览器证据和中文交付记录。

- [ ] **Step 1: 运行前端门禁**：Dashboard 与 SaaS Admin 分别执行 lint、typecheck、test、build；预期退出码 0。
- [ ] **Step 2: 运行仓库相关门禁**：执行 `git diff --check` 和相关 Go 测试；无 MariaDB DSN 时只记录 SKIP。
- [ ] **Step 3: 重建 app 服务**：使用 compose project `mochat-wecom-acceptance-20260827` 仅重建必要镜像，确认 app、bridge、MySQL、Redis 健康且数据卷未替换。
- [ ] **Step 4: 浏览器真实点击**：在 `http://127.0.0.1:19080/` 检查唯一企业资料的简洁状态、第三方模式隐藏和 SaaS 能力选择/保存/刷新恢复；检查控制台与关键网络请求。
- [ ] **Step 5: 更新报告并提交**：记录真实 PASS/SKIP、端口和外部边界，只 stage 本次文件后提交。
