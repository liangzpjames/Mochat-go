# 移动端企业微信风格视觉优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Sidebar 与 Operation 的现有移动端框架升级为统一的企业微信工作台风格，同时保留 12+10 路由、独立认证边界和真实业务合同。

**Architecture:** 在 `@mochat/mobile-foundation` 建立无业务状态的视觉组件与令牌；Sidebar 组合员工端三栏导航和客户工作台，Operation 组合无员工导航的活动页。所有页面继续消费现有 API 与 registry，视觉层不得读取认证存储或产生假业务数据。

**Tech Stack:** React 19.2、TypeScript strict、React Router 7、CSS、Vitest、Testing Library、Playwright、pnpm workspace。

## Global Constraints

- 参考图仅作为布局、配色、层级和触控方式参考；禁止复制“圆弧AI会话”等品牌文字、Logo、人物、机器人或插画素材。
- 不新增 npm 运行时依赖，不引入新 UI 框架，不使用外部图片 URL。
- Sidebar 保持 12 条 manifest 路由；Operation 保持 10 条 manifest 路由；未知路由继续精确 404。
- Sidebar 认证业务页显示“客户 / 会话 / 我的”底部导航；`/login`、`/auth`、`/codeAuth` 与未知路由不显示。
- Sidebar `/sidebar-app` 会话继续使用 `Path=/sidebar-app` Cookie；独立 root mount 继续使用专属 sessionStorage；视觉组件不得读取任何会话存储。
- Operation 不显示员工底部导航，不读取 Sidebar/Dashboard token 或 storage，不发送 Bearer。
- `/contact` 与 `/workFission` 只展示真实 API 字段；其余 20 个入口继续明确“模块待迁移”，不得生成假统计、假进度、假奖品或假成功按钮。
- 390×844 为主验收视口；同时覆盖 1280×900，页面不得横向溢出，所有交互控件最小 44×44px，底部导航不得遮挡内容。
- 所有行为修改按 TDD 执行：先提交可解释的 RED，再写最小实现并验证 GREEN。
- 本计划不操作 Docker、服务器、数据库、数据卷或生产环境；视觉证据只写入 `D:\workspace\mochat-go\output\mobile-visual-refresh-20260814`。

---

### Task 1: 共享移动端视觉组件与令牌

**Files:**
- Create: `web/packages/mobile-foundation/src/shell/mobile-card.tsx`
- Create: `web/packages/mobile-foundation/src/shell/mobile-card.test.tsx`
- Create: `web/packages/mobile-foundation/src/shell/mobile-navigation.tsx`
- Create: `web/packages/mobile-foundation/src/shell/mobile-navigation.test.tsx`
- Modify: `web/packages/mobile-foundation/src/shell/mobile-shell.tsx`
- Modify: `web/packages/mobile-foundation/src/shell/mobile-shell.test.tsx`
- Modify: `web/packages/mobile-foundation/src/shell/mobile-state.tsx`
- Modify: `web/packages/mobile-foundation/src/styles/mobile.css`
- Modify: `web/packages/mobile-foundation/src/index.ts`

**Interfaces:**
- Produces: `MobileCard({ tone, padding, children })`，其中 `tone: 'surface' | 'accent' | 'muted'`，`padding: 'none' | 'compact' | 'comfortable'`。
- Produces: `MobileIconTile({ icon, title, description?, badge?, href?, disabled? })`，有 `href` 时渲染链接，否则渲染不可交互内容；禁止空链接。
- Produces: `MobileBottomNavigation({ label, items })`，`items: ReadonlyArray<{ key; label; icon; href; current }>`。
- Produces: `MobileShell` 新增可选 `eyebrow?: string`、`hero?: ReactNode` 和现有 `actions` 固定底部支持；现有调用保持兼容。

- [ ] **Step 1: 为卡片、入口与导航写失败测试**

在新测试中覆盖：

```tsx
render(<MobileCard tone="accent" padding="compact">重点</MobileCard>);
expect(screen.getByText('重点').closest('section')).toHaveClass('mobile-card--accent');

render(<MobileIconTile icon={<span />} title="客户" href="/contact" />);
expect(screen.getByRole('link', { name: '客户' })).toHaveAttribute('href', '/contact');

render(<MobileBottomNavigation label="员工工作台" items={items} />);
expect(screen.getByRole('navigation', { name: '员工工作台' })).toBeVisible();
expect(screen.getByRole('link', { name: '客户' })).toHaveAttribute('aria-current', 'page');
```

并断言 disabled tile 不渲染链接、所有导航项有图标与文字、`MobileState` 有装饰图形但图形 `aria-hidden=true`。

- [ ] **Step 2: 运行 RED**

Run:

```powershell
corepack pnpm --filter @mochat/mobile-foundation test
```

Expected: FAIL，原因是 `MobileCard`、`MobileIconTile`、`MobileBottomNavigation` 尚未导出，或新 DOM 语义尚不存在。

- [ ] **Step 3: 实现最小共享组件**

实现纯展示组件；图标由调用方传入，组件不得 import router、auth 或 storage。`MobileBottomNavigation` 使用 `<nav>` 和 `<a>`，当前项设置 `aria-current="page"`。`MobileIconTile` 的 disabled 形式使用 `<div aria-disabled="true">`。

- [ ] **Step 4: 建立视觉令牌和响应式样式**

在 `mobile.css` 中增加：

```css
:root {
  --mobile-color-background: #eef6ff;
  --mobile-color-primary: #1677ff;
  --mobile-gradient-primary: linear-gradient(145deg, #5f80ff 0%, #1688ff 100%);
  --mobile-radius-card: 20px;
  --mobile-navigation-height: 68px;
  --mobile-content-max-width: 30rem;
}
```

并实现白色卡片、轻阴影、蓝色强调卡、固定底部导航、安全区、内容底部预留、状态插图式图形、320px 收缩和 `prefers-reduced-motion`。

- [ ] **Step 5: 运行 GREEN 与静态门禁**

Run:

```powershell
corepack pnpm --filter @mochat/mobile-foundation lint
corepack pnpm --filter @mochat/mobile-foundation typecheck
corepack pnpm --filter @mochat/mobile-foundation test
corepack pnpm --filter @mochat/mobile-foundation build
```

Expected: 全部 exit 0；共享包测试总数高于当前 21。

- [ ] **Step 6: 提交 Task 1**

```powershell
git add web/packages/mobile-foundation
git commit -m "feat(mobile): add WeCom-inspired visual primitives"
```

---

### Task 2: Sidebar 员工工作台与客户页视觉

**Files:**
- Create: `web/apps/sidebar/src/ui/sidebar-page-shell.tsx`
- Create: `web/apps/sidebar/src/ui/sidebar-page-shell.test.tsx`
- Modify: `web/apps/sidebar/src/features/contact/contact-page.tsx`
- Modify: `web/apps/sidebar/src/features/contact/contact-page.test.tsx`
- Modify: `web/apps/sidebar/src/app/sidebar-router.tsx`
- Modify: `web/apps/sidebar/src/app/sidebar-router.test.tsx`
- Modify: `web/apps/sidebar/src/styles.css`

**Interfaces:**
- Consumes: Task 1 的 `MobileShell`、`MobileCard`、`MobileIconTile`、`MobileBottomNavigation`。
- Produces: `SidebarPageShell`，接收 `title`、`subtitle?`、`children`、`showNavigation?: boolean`，通过 `useLocation` 计算导航 href 与当前项。
- Navigation mapping: 客户=`/contact`，会话=`/contactSop`，我的=`/`；业务页切换时保留当前 `search` 与 `hash`。

- [ ] **Step 1: 为 Sidebar 导航合同写失败测试**

覆盖以下行为：

```tsx
expect(screen.getByRole('navigation', { name: '员工工作台' })).toBeVisible();
expect(screen.getByRole('link', { name: '客户' })).toHaveAttribute(
  'href',
  '/contact?wxExternalUserid=external-1#profile',
);
expect(screen.getByRole('link', { name: '客户' })).toHaveAttribute('aria-current', 'page');
```

分别打开 `/login`、`/auth`、`/codeAuth`、未知路由并断言 navigation 不存在；打开 `/contactSop` 与 `/` 并断言对应当前项。测试继续使用现有 12 条 registry，不新增路由。

- [ ] **Step 2: 运行 Sidebar RED**

```powershell
corepack pnpm --filter @mochat/sidebar test -- src/ui/sidebar-page-shell.test.tsx src/app/sidebar-router.test.tsx
```

Expected: FAIL，原因是 `SidebarPageShell` 尚不存在、现有页面没有底部导航。

- [ ] **Step 3: 实现 SidebarPageShell 与三栏导航**

使用内联 SVG 绘制客户、会话、我的三个简洁线性图标。导航 href 只组合注册路径与当前 search/hash；不得携带 callback `state`，因为认证路由不渲染该导航。业务页面内容自动预留底部导航高度。

- [ ] **Step 4: 为真实客户资料卡写失败测试**

断言真实响应只渲染：

```tsx
expect(screen.getByText('测试客户')).toBeVisible();
expect(screen.getByText('客户编号 11')).toBeVisible();
expect(screen.getByText('企业编号 3')).toBeVisible();
expect(screen.queryByText(/手机号|标签|负责人/)).not.toBeInTheDocument();
```

无头像时断言可访问占位说明；loading/error/forbidden/not-found 保持既有语义；网络错误仍只显示一个重试操作。

- [ ] **Step 5: 实现客户卡与 Sidebar 工作台页面**

`/contact` 使用头像资料卡、两项真实信息行和浅蓝页面背景。`/` 使用“客户工作台”CSS hero 与现有业务路由入口宫格；入口只来自 `sidebarRouteRegistry` 的认证业务路由，排除 `/login`、`/auth`、`/codeAuth`。其他待迁移页使用统一卡片状态，不增加假按钮。

- [ ] **Step 6: 完成 Sidebar 样式与 GREEN**

实现参考图式圆角卡、图标底板、分组标题和 fixed navigation；确保 320px 无溢出。运行：

```powershell
corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar build
```

Expected: 全部 exit 0；现有认证、Cookie/sessionStorage、请求和错误分类测试无回归。

- [ ] **Step 7: 提交 Task 2**

```powershell
git add web/apps/sidebar
git commit -m "feat(sidebar): apply employee workbench visual system"
```

---

### Task 3: Operation 活动页视觉

**Files:**
- Modify: `web/apps/operation/src/features/work-fission/work-fission-page.tsx`
- Modify: `web/apps/operation/src/features/work-fission/work-fission-page.test.tsx`
- Modify: `web/apps/operation/src/app/operation-router.tsx`
- Modify: `web/apps/operation/src/app/operation-router.test.tsx`
- Modify: `web/apps/operation/src/styles.css`

**Interfaces:**
- Consumes: Task 1 的 `MobileShell`、`MobileCard`、`MobileIconTile`。
- Preserves: `id -> openUserInfo session -> participant.unionid -> taskData` 请求顺序、`safeRewardUrl`、活动结束判断和错误分类。
- Prohibits: `MobileBottomNavigation`、Sidebar/Dashboard storage、Bearer、外部图片 URL。

- [ ] **Step 1: 为活动视觉结构写失败测试**

成功态断言：

```tsx
expect(screen.getByRole('region', { name: '任务宝活动概览' })).toBeVisible();
expect(screen.getByText('已邀请 2 位好友')).toBeVisible();
expect(screen.getAllByRole('listitem')).toHaveLength(2);
expect(screen.queryByRole('navigation', { name: '员工工作台' })).not.toBeInTheDocument();
```

同时保留 session unionid、raw `[]` OAuth、结束时间、危险奖励 URL 和 retry 现有测试。

- [ ] **Step 2: 运行 Operation RED**

```powershell
corepack pnpm --filter @mochat/operation test -- src/features/work-fission/work-fission-page.test.tsx src/app/operation-router.test.tsx
```

Expected: FAIL，原因是活动 hero、强调进度卡和新语义结构尚不存在。

- [ ] **Step 3: 实现任务宝活动视觉**

使用 CSS 渐变和内联 SVG 装饰构建 hero。摘要卡显示真实邀请数与差额；任务卡显示级别、目标、完成状态、奖励类型和领取状态。`task.reward.url === null` 时不渲染链接；其余行为完全沿用现有数据流。

- [ ] **Step 4: 优化待迁移活动与状态页**

9 个待迁移入口使用统一活动卡和业务说明，不显示底部导航，不创建参与人数、进度、奖品或“立即参与”等假操作。加载、OAuth、无权、结束和网络失败使用共享状态视觉。

- [ ] **Step 5: 完成 Operation GREEN**

```powershell
corepack pnpm --filter @mochat/operation lint
corepack pnpm --filter @mochat/operation typecheck
corepack pnpm --filter @mochat/operation test
corepack pnpm --filter @mochat/operation build
```

Expected: 全部 exit 0；现有 74 项测试及新增视觉结构测试全部通过。

- [ ] **Step 6: 提交 Task 3**

```powershell
git add web/apps/operation
git commit -m "feat(operation): refresh activity mobile experience"
```

---

### Task 4: 完成门禁、双视口与视觉证据

**Files:**
- Modify: `scripts/check_mobile_clients_foundation.mjs`
- Modify: `scripts/check_mobile_clients_foundation.test.mjs`
- Modify: `web/e2e/tests/mobile-clients-foundation.spec.ts`
- Create: `scripts/capture_mobile_visual_evidence.mjs`
- Modify: `package.json`

**Interfaces:**
- Consumes: Task 1–3 的生产页面和现有 Playwright fixture。
- Produces: `pnpm capture:mobile-visual-evidence`，使用现有构建/fixture，将截图写入环境变量 `MOCHAT_MOBILE_VISUAL_OUTPUT` 指向的目录；未设置时明确失败，不写仓库。

- [ ] **Step 1: 为完成门禁写失败测试**

增加独立坏 fixture：

- 删除 Sidebar 三栏导航时失败。
- 在 Operation 引入 `MobileBottomNavigation` 时失败。
- 将 Sidebar token Cookie 改回 `Path=/` 时失败。
- 引入参考品牌文字、外部图片 URL 或假业务数字时失败。
- 删除 390×844、固定导航无遮挡、工作台 active item 或 Operation 无导航断言时失败。

- [ ] **Step 2: 运行门禁 RED**

```powershell
node --test scripts/check_mobile_clients_foundation.test.mjs
```

Expected: 新增坏 fixture 出现 `Missing expected exception`，证明 gate 尚未保护这些行为。

- [ ] **Step 3: 实现来源驱动门禁**

门禁从生产 TS/TSX/CSS、route registry 和真实 E2E 源码提取证据；不得只在同一 fixture 字符串中互相比对。每个坏 fixture 必须单独触发明确错误。

- [ ] **Step 4: 扩展 Playwright 双视口断言**

在现有 48 个 case 中增加：

- Sidebar 认证业务页底部导航可见、当前项正确、最后内容不被遮挡。
- Sidebar 认证/未知页无导航。
- Operation 全部 10 页无员工导航。
- 两个视口 `document.documentElement.scrollWidth <= clientWidth`。
- 继续保持 raw Go envelope、session unionid、console、pageerror、requestfailed、意外 4xx/5xx 和 unexpected request 清洁审计。

- [ ] **Step 5: 实现视觉证据脚本**

脚本使用 Playwright Chromium 和现有 raw Go fixtures，固定捕获：

```text
sidebar-contact-390.png
sidebar-workbench-390.png
sidebar-pending-390.png
operation-work-fission-390.png
operation-pending-390.png
sidebar-contact-1280.png
operation-work-fission-1280.png
```

输出目录只来自 `MOCHAT_MOBILE_VISUAL_OUTPUT`；日志仅输出文件路径，不输出 token、Cookie 或 OAuth state。

- [ ] **Step 6: 运行完整非 Docker 门禁**

```powershell
corepack pnpm --filter @mochat/mobile-foundation lint
corepack pnpm --filter @mochat/mobile-foundation typecheck
corepack pnpm --filter @mochat/mobile-foundation test
corepack pnpm --filter @mochat/mobile-foundation build
corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar build
corepack pnpm --filter @mochat/operation lint
corepack pnpm --filter @mochat/operation typecheck
corepack pnpm --filter @mochat/operation test
corepack pnpm --filter @mochat/operation build
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e lint
corepack pnpm check:mobile-clients-foundation
corepack pnpm --filter @mochat/e2e test:mobile-clients-foundation
$env:MOCHAT_MOBILE_VISUAL_OUTPUT='D:\workspace\mochat-go\output\mobile-visual-refresh-20260814'
corepack pnpm capture:mobile-visual-evidence
git diff --check
```

Expected: 所有命令 exit 0；Playwright 48/48；截图 7/7 存在且人工审阅无横向溢出、遮挡、错误文字或参考品牌素材。

- [ ] **Step 7: 提交 Task 4**

```powershell
git add scripts/check_mobile_clients_foundation.mjs scripts/check_mobile_clients_foundation.test.mjs scripts/capture_mobile_visual_evidence.mjs web/e2e/tests/mobile-clients-foundation.spec.ts package.json
git commit -m "test(mobile): gate WeCom-inspired visual refresh"
```

---

## 最终审阅门禁

完成四个 Task 后必须进行：

1. 逐 Task 规格符合性审阅和代码质量审阅，Critical/Important 全部修复并复审。
2. 从设计提交前的 merge base 到最终 HEAD 做全分支审阅。
3. 人工查看 7 张证据截图，重点检查 390px 底部导航、卡片层级、状态页、桌面居中和 Operation 无员工导航。
4. 确认工作树 clean，主工作区既有 dirty 文件未变化；不部署、不操作 Docker。
