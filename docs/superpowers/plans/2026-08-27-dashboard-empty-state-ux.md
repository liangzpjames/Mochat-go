# Dashboard 空数据页面展示优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 去除普通用户空态中的技术术语，并让文件录音、好友、客户群、客户分析客户明细的空态在桌面与移动端保持居中、清晰和可操作。

**Architecture:** 保留现有查询和 `Phase35DataState` 分支，为四个自定义空态增加共享展示类；后端限制继续作为数据存在，但前端只渲染固定的业务化概括，避免透传实现细节。概览、报表、AI 和营销工具沿用同一业务文案边界，包含开发术语的外部失败消息在展示前降级为安全提示。样式集中在 Dashboard 全局 Phase 3.5 样式区，文件录音沿用同一类名以保证跨页面一致。

**Tech Stack:** React 19、TypeScript、TanStack Query、Vitest、Testing Library、CSS、Docker Compose

## Global Constraints

- 用户可见文案使用中文，不出现 `Provider`、接口、数据表等开发术语。
- 不改变鉴权、租户隔离、查询参数、同步和分页合同。
- 没有配置或没有数据时展示正常空态，网络或服务错误仍保留错误态与重试。
- 只修改独立 worktree，不触碰 `.superpowers/sdd/progress.md` 及主工作树未提交内容。

---

### Task 1: 建立空态行为合同

**Files:**
- Modify: `web/apps/dashboard/src/features/phase35/file-audio-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/friends-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/group-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/customer-report-page.test.tsx`

**Interfaces:**
- Consumes: `Phase35DataState` 的 `emptyContent: ReactNode`。
- Produces: `phase35-empty-state` DOM 合同以及不透传技术限制消息的行为合同。

- [ ] **Step 1: 写入失败测试**

四个页面分别使用空列表夹具，断言命名空态区域包含 `phase35-empty-state`；客户群断言不含 `Provider`；客户分析使用包含技术术语的 `limitations` 夹具，断言只显示业务提示。

- [ ] **Step 2: 运行测试并确认失败**

Run: `pnpm --filter @mochat/dashboard test -- file-audio-page.test.tsx friends-page.test.tsx group-page.test.tsx customer-report-page.test.tsx`

Expected: FAIL，原因是共享空态类和业务化限制提示尚未实现。

### Task 2: 实现统一空态与业务文案

**Files:**
- Modify: `web/apps/dashboard/src/features/phase35/file-audio-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/friends-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/group-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/customer-report-page.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Consumes: Task 1 的 DOM 与文案合同。
- Produces: `.phase35-empty-state`，以及普通用户可理解的固定限制提示。

- [ ] **Step 1: 给四个空态区域增加共享类**

在各页面的 `emptyContent` 根 `section` 上设置 `className="phase35-empty-state"`，保持现有 `aria-label`。

- [ ] **Step 2: 替换技术化文案**

客户群说明改为“完成企业授权并同步客户群后，数据会自动出现在这里”；客户分析不再渲染 `item.message`，存在限制时只显示“部分数据暂未同步完整，当前仅展示已获取的数据”。

- [ ] **Step 3: 增加响应式居中样式**

使用网格布局、`text-align: center`、安全内边距和说明文本最大宽度；链接保持可见焦点与足够点击面积。

- [ ] **Step 4: 运行目标测试并确认通过**

Run: `pnpm --filter @mochat/dashboard test -- file-audio-page.test.tsx friends-page.test.tsx group-page.test.tsx customer-report-page.test.tsx`

Expected: 目标测试全部 PASS。

### Task 3: 完整验证和本地交付

**Files:**
- Verify: `web/apps/dashboard`
- Verify: isolated Docker Compose project `mochat-wecom-acceptance-20260827`

**Interfaces:**
- Consumes: Task 2 构建产物。
- Produces: 可在 `http://127.0.0.1:19080` 访问的已验证页面。

- [ ] **Step 1: 运行 Dashboard 门禁**

Run: `pnpm --filter @mochat/dashboard lint && pnpm --filter @mochat/dashboard typecheck && pnpm --filter @mochat/dashboard test && pnpm --filter @mochat/dashboard build`

Expected: 全部 PASS。

- [ ] **Step 2: 运行差异检查**

Run: `git diff --check`

Expected: 无输出，退出码 0。

- [ ] **Step 3: 重建并启动应用容器**

使用现有隔离项目的 compose 文件和环境参数，仅重建应用服务，不覆盖其他 Docker 项目或卷；等待健康检查通过。

- [ ] **Step 4: 浏览器逐页验收**

依次访问 `/chat/file-audio`、`/customer/friends`、`/customer/group`、`/data/customer`，核对空态居中、不含技术术语、刷新恢复、控制台和网络无异常；再以 390×844 验证无横向溢出和内容可读性。

- [ ] **Step 5: 提交变更**

Run: `git add` 仅包含本计划列出的文件，再执行 `git commit -m "fix: improve dashboard empty states"`。

Expected: 生成独立提交，`.superpowers/sdd/progress.md` 保持未暂存。

### Task 4: 扩展普通用户文案与可访问性合同

**Files:**
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.tsx`
- Modify: `web/apps/dashboard/src/features/ai-insight/*`
- Modify: `web/apps/dashboard/src/features/phase33/phase33-closure-pages.tsx`
- Modify: `web/apps/dashboard/src/features/phase34/*`
- Add: `web/apps/dashboard/src/styles/plain-language-copy.test.ts`
- Add: `web/apps/dashboard/src/styles/phase35-empty-state-layout.test.ts`

- [ ] **Step 1: 禁止普通用户源码出现开发术语，并验证外部技术错误不会原样展示**
- [ ] **Step 2: 验证共享空态布局无固定宽度且正文对比度不低于 4.5:1**
- [ ] **Step 3: 运行扩展回归和完整 Dashboard 测试**
