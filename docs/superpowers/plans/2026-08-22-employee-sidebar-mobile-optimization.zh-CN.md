# 员工使用的移动侧边栏整体优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development task-by-task; use superpowers:systematic-debugging for every unexpected failure; use superpowers:verification-before-completion before commits and final delivery. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变 Sidebar 12 条历史 URL 和员工鉴权边界的前提下，把当前占位式 React Sidebar 实现为覆盖客户资料维护、素材触达、个人/群 SOP、批量加好友及登录恢复的真实移动端。

**Architecture:** 业务代码按 contact、medium、sop、batch-add、wecom 五个域拆分；每个域的 API 层先校验未知 JSON，再交给页面状态机。Router 只组合路由、鉴权和 401 恢复，Sidebar 专用 UI 保持在应用内；本计划预期不修改 `@mochat/mobile-foundation`，除非 TDD 证明至少两个移动应用需要同一无业务能力。

**Tech Stack:** React 19.2、TypeScript 5.9 strict、React Router 7、Vite 8、Vitest、Testing Library、Playwright、原生 CSS、现有 `@mochat/mobile-foundation`、企业微信 JS-SDK、Go `/sidebar/*` API。

## Global Constraints

- 精确基线：`b9a47cab45ec872bc81e61a20b06a8a3529311b2`；分支：`feat/employee-sidebar-mobile-optimization`。
- 只修改 `web/apps/sidebar`、相关测试/E2E/部署验收脚本和本任务文档；不修改 `docs/PROJECT_PROGRESS.zh-CN.md`。
- 不修改 `web/apps/operation`、Dashboard、SaaS Admin、Phase 7 或会话存档业务。
- 保留 `/`、`/auth`、`/codeAuth`、`/contact`、`/contact/editDetail`、`/contact/remark`、`/contact/settingTag`、`/contactBatchAdd`、`/contactSop`、`/login`、`/medium`、`/roomSop`。
- 生产页面不得包含 fixture、随机数据、固定成功、假统计或纯内存业务结果。
- 所有生产行为严格执行 RED→GREEN→REFACTOR；每个 RED 必须因缺少目标行为失败，不因测试拼写/环境错误失败。
- API 固定 `/sidebar` scope；401 清理 Sidebar 会话并重新授权；403 不清理会话；target 只允许同源内部路径。
- 触控目标最小 44×44px；支持 safe-area、软键盘、长文本、慢网、断网、空态和防重复提交。
- Docker 验收保留所有命名卷，不执行 `down -v`、`volume rm` 或等价命令。

---

## 文件结构与责任

```text
web/apps/sidebar/src/
  app/
    catalog.ts                         # 12 路由中文元数据
    sidebar-router.tsx                 # 路由组合、鉴权、401 恢复
    sidebar-router.test.tsx
  auth/
    code-auth.ts                       # 兼容 callValues 解析与安全恢复
    code-auth.test.ts
    sidebar-session.ts                 # 既有 Sidebar 会话
  features/contact/
    contact-api.ts                     # 客户 detail/show/track/portrait/tag/update/upload 校验
    contact-api.test.ts
    contact-context.ts                 # query -> 可信 ContactContext
    contact-context.test.tsx
    contact-page.tsx                   # 摘要、快捷操作、轨迹、画像、SOP 提醒
    contact-page.test.tsx
    contact-edit-page.tsx              # 画像编辑
    contact-edit-page.test.tsx
    contact-remark-page.tsx            # 备注编辑
    contact-remark-page.test.tsx
    contact-tag-page.tsx               # 标签追加
    contact-tag-page.test.tsx
  features/medium/
    medium-api.ts                      # 分组、列表、mediaId 刷新
    medium-api.test.ts
    medium-page.tsx                    # 搜索、筛选、分页、选择、发送
    medium-page.test.tsx
  features/sop/
    sop-api.ts                         # 个人/群 SOP 解析与群 SOP 完成
    sop-api.test.ts
    sop-content.tsx                    # 文本/图片等内容块
    contact-sop-page.tsx
    contact-sop-page.test.tsx
    room-sop-page.tsx
    room-sop-page.test.tsx
  features/batch-add/
    batch-add-api.ts                   # 批次详情解析
    batch-add-api.test.ts
    batch-add-page.tsx                 # 状态筛选、复制、企微加客户
    batch-add-page.test.tsx
  wecom/
    wecom-bridge.ts                    # JSSDK 配置、能力检测、invoke
    wecom-bridge.test.ts
  ui/
    sidebar-page-shell.tsx             # 三栏导航、上下文保留
    sidebar-page-shell.test.tsx
    sidebar-primitives.tsx             # SidebarSection/StickyActions/反馈条
  styles.css                           # Sidebar 视觉、响应式、键盘/安全区

web/e2e/tests/
  mobile-clients-foundation.spec.ts    # 保留现有两个移动端基准
  sidebar-employee-mobile.spec.ts      # 12 路由和业务状态专项验收

scripts/
  capture_sidebar_employee_mobile_evidence.mjs

docs/reviews/
  2026-08-22-employee-sidebar-mobile-acceptance.zh-CN.md
```

## 公共接口锁定

```ts
export type SidebarRequest = <T>(path: string, init?: RequestInit) => Promise<T>;

export type ContactContext = {
  externalUserId: string;
  contactId: number;
  summary: { id: number; name: string; avatar: string | null; corpId: number };
};

export type WeComBridge = {
  available(): boolean;
  sendChatMessage(input: WeComMessage): Promise<void>;
  navigateToAddCustomer(): Promise<void>;
};

export type PageFailure = {
  kind: 'forbidden' | 'not-found' | 'validation' | 'retryable';
  title: string;
  message: string;
};
```

Router 向所有受保护业务页注入同一个 `request`、`onReauthenticate` 和 `wecom`；业务页不得读取 cookie、Dashboard storage 或全局 fetch。

---

### Task 1：锁定领域 API 与客户上下文

**Files:**

- Modify: `web/apps/sidebar/src/features/contact/contact-api.ts`
- Create: `web/apps/sidebar/src/features/contact/contact-api.test.ts`
- Create: `web/apps/sidebar/src/features/contact/contact-context.ts`
- Create: `web/apps/sidebar/src/features/contact/contact-context.test.tsx`

**Interfaces:**

- Produces: `loadContactContext(request, externalUserId): Promise<ContactContext>`
- Produces: `loadContactWorkspace(request, contactId): Promise<ContactWorkspace>`
- Produces: `loadContactPortrait`, `updateContactPortrait`, `updateContactRemark`, `loadTagGroups`, `loadTags`, `appendContactTags`, `uploadPortraitImage`
- Constraint: every response is `unknown` until validated; `contactId` must be a positive safe integer.

- [ ] **Step 1: 写联系人 detail/show 的失败测试**

```ts
it('resolves wxExternalUserid to a contact then loads server detail', async () => {
  const request = vi.fn()
    .mockResolvedValueOnce({ id: 11, name: '林晓', avatar: null, corpId: 3 })
    .mockResolvedValueOnce({ name: '林晓', remark: '重点客户', tag: [], roomName: [] });
  const context = await loadContactContext(request, 'external-1');
  expect(context.contactId).toBe(11);
  expect(request).toHaveBeenNthCalledWith(1, '/workContact/detail?wxExternalUserid=external-1', { method: 'GET' });
  expect(request).toHaveBeenNthCalledWith(2, '/workContact/show?contactId=11', { method: 'GET' });
});
```

- [ ] **Step 2: 运行 RED**

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/contact/contact-api.test.ts`

Expected: FAIL because `loadContactContext`/new response validators do not exist.

- [ ] **Step 3: 实现最小 detail/show/track/portrait/tag/update API**

```ts
export async function updateContactRemark(request: SidebarRequest, contactId: number, remark: string) {
  await request<unknown>('/workContact/update', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ contactId, remark }),
  });
}
```

Implementation must encode query with `URLSearchParams`, reject malformed arrays/fields with `MobileApiError('validation')`, and serialize multi-select portrait values exactly as the existing API accepts.

- [ ] **Step 4: 补齐格式错误、空数组、401 透传和上传 FormData 测试并运行 GREEN**

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/contact/contact-api.test.ts`

Expected: PASS; no console warnings.

- [ ] **Step 5: 编写客户上下文 hook 的 RED/GREEN**

```tsx
it('does not request without wxExternalUserid', () => {
  renderHook(() => useContactContext(request, '?agentId=7'));
  expect(request).not.toHaveBeenCalled();
});
```

Run RED then GREEN with: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/contact/contact-context.test.tsx`

- [ ] **Step 6: 验证并提交**

Run: `corepack pnpm --filter @mochat/sidebar test && corepack pnpm --filter @mochat/sidebar typecheck && git diff --check`

Commit: `feat(sidebar): add validated employee contact domain`

---

### Task 2：统一 Sidebar 页面节奏与安全授权恢复

**Files:**

- Create: `web/apps/sidebar/src/auth/code-auth.ts`
- Create: `web/apps/sidebar/src/auth/code-auth.test.ts`
- Modify: `web/apps/sidebar/src/app/sidebar-router.tsx`
- Modify: `web/apps/sidebar/src/app/sidebar-router.test.tsx`
- Modify: `web/apps/sidebar/src/ui/sidebar-page-shell.tsx`
- Modify: `web/apps/sidebar/src/ui/sidebar-page-shell.test.tsx`
- Create: `web/apps/sidebar/src/ui/sidebar-primitives.tsx`
- Modify: `web/apps/sidebar/src/styles.css`

**Interfaces:**

- Produces: `parseLegacyCodeAuth(params): { ok: true; agentId; target } | { ok: false; message }`
- Produces: `SidebarSection`, `SidebarStickyActions`, `SidebarFeedback`
- Constraint: `/codeAuth` never writes a token from `callValues`; it redirects only to `/login` with a safe target.

- [ ] **Step 1: 为 `/codeAuth` fail-closed 和兼容恢复写 RED**

```ts
it('extracts agentId and converts pageFlag without accepting the legacy token', () => {
  const callValues = btoa(JSON.stringify({ code: 200, msg: '', data: { agentId: 7, act: 'mediumGroup', token: 'dashboard-token' } }));
  expect(parseLegacyCodeAuth(new URLSearchParams({ callValues }))).toEqual({
    ok: true, agentId: '7', target: '/medium',
  });
});
```

Also assert malformed Base64, external target, missing agentId and code != 200 fail closed.

- [ ] **Step 2: 运行 RED，随后实现最小解析器并运行 GREEN**

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/auth/code-auth.test.ts`

- [ ] **Step 3: 写 Router 页面组合 RED**

Assertions: public auth routes have no bottom navigation; each protected route renders its actual feature heading; login action is disabled after activation; `codeAuth` recovery link targets `/login` and contains no token.

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/app/sidebar-router.test.tsx`

- [ ] **Step 4: 实现壳层和 Router 注入**

```tsx
<SidebarPageShell title="编辑客户资料" mode="form" onBack={() => navigate(-1)}>
  <ContactEditPage request={runtime.request} onReauthenticate={onReauthenticate} />
</SidebarPageShell>
```

`SidebarPageShell` must map `/contactSop` and `/roomSop` to “会话”, form pages to “客户”, and omit navigation on auth routes. Context preservation must use an allow-list per destination rather than blindly forwarding all query keys.

- [ ] **Step 5: 写 CSS 响应式与触控合同测试/静态断言**

Assert form actions expose accessible names, primary controls have class tokens whose CSS includes `min-height:44px`, shell reserves bottom navigation/safe-area, and 320px grids collapse without fixed widths.

- [ ] **Step 6: 验证并提交**

Run: `corepack pnpm --filter @mochat/sidebar test && corepack pnpm --filter @mochat/sidebar lint && corepack pnpm --filter @mochat/sidebar typecheck && git diff --check`

Commit: `feat(sidebar): secure auth recovery and mobile shell`

---

### Task 3：客户工作区真实摘要、轨迹、画像与提醒

**Files:**

- Modify: `web/apps/sidebar/src/features/contact/contact-page.tsx`
- Modify: `web/apps/sidebar/src/features/contact/contact-page.test.tsx`
- Modify: `web/apps/sidebar/src/app/catalog.ts`
- Modify: `web/apps/sidebar/src/styles.css`

**Interfaces:**

- Consumes: `loadContactContext`, `loadContactWorkspace`, `loadContactPortrait`, `loadContactSOPTips`
- Produces: customer summary, maintenance links, “概览/互动轨迹/客户画像/SOP” tabs with independent states.

- [ ] **Step 1: 写复合加载的 RED**

```tsx
it('keeps the real summary when a secondary panel fails', async () => {
  renderContact(contactPath, requestWithWorkspaceAndFailedTrack());
  expect(await screen.findByRole('heading', { name: '林晓' })).toBeVisible();
  expect(screen.getByText('互动轨迹加载失败')).toBeVisible();
});
```

Cover real remark/tags/room/owner, empty tags, long names, portrait field types, track retry, SOP empty, 401 once-only callback, and links preserving `wxExternalUserid`.

- [ ] **Step 2: 运行 RED**

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/contact/contact-page.test.tsx`

- [ ] **Step 3: 实现摘要优先、次级区块独立状态**

Use `Promise.allSettled` only for independent secondary requests after the trusted contact ID is resolved. Never replace a loaded summary with a full-page error caused by track/portrait/SOP.

- [ ] **Step 4: GREEN、重构、全量回归**

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/contact/contact-page.test.tsx && corepack pnpm --filter @mochat/sidebar test`

- [ ] **Step 5: 提交**

Commit: `feat(sidebar): build real employee customer workspace`

---

### Task 4：备注、标签与画像三个写入页面

**Files:**

- Create: `web/apps/sidebar/src/features/contact/contact-edit-page.tsx`
- Create: `web/apps/sidebar/src/features/contact/contact-edit-page.test.tsx`
- Create: `web/apps/sidebar/src/features/contact/contact-remark-page.tsx`
- Create: `web/apps/sidebar/src/features/contact/contact-remark-page.test.tsx`
- Create: `web/apps/sidebar/src/features/contact/contact-tag-page.tsx`
- Create: `web/apps/sidebar/src/features/contact/contact-tag-page.test.tsx`
- Modify: `web/apps/sidebar/src/styles.css`

**Interfaces:**

- All pages consume `ContactContext`, `SidebarRequest`, `onReauthenticate`, `onDone`, `onCancel`.
- Every submit function returns only after real API resolution and guards re-entry with `submitting`.

- [ ] **Step 1: 备注页 RED**

```tsx
it('rejects blank and over-10-character remarks without calling update', async () => {
  renderRemarkPage();
  fireEvent.click(screen.getByRole('button', { name: '保存' }));
  expect(screen.getByRole('alert')).toHaveTextContent('请输入 1 至 10 个字符');
  expect(request).not.toHaveBeenCalledWith('/workContact/update', expect.anything());
});
```

Cover current value, cancel, double click, network retry preserving input, 401 and success return.

- [ ] **Step 2: 备注页 GREEN**

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/contact/contact-remark-page.test.tsx`

- [ ] **Step 3: 标签页 RED/GREEN**

Tests must prove existing tags are locked, group filter calls `allTag`, new IDs are de-duplicated, empty selection does not write, old tags cannot be removed, and failed save preserves selection.

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/contact/contact-tag-page.test.tsx`

- [ ] **Step 4: 画像页 RED/GREEN**

Tests must cover text/single/multi/dropdown/date/image, image size/type validation, real multipart upload, server response path, cancel, JSON serialization, empty portrait, duplicate submit and keyboard focus order.

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/contact/contact-edit-page.test.tsx`

- [ ] **Step 5: 三页全量回归与提交**

Run: `corepack pnpm --filter @mochat/sidebar test && corepack pnpm --filter @mochat/sidebar lint && corepack pnpm --filter @mochat/sidebar typecheck && git diff --check`

Commit: `feat(sidebar): restore customer editing workflows`

---

### Task 5：企业微信 JS-SDK 桥接

**Files:**

- Create: `web/apps/sidebar/src/wecom/wecom-bridge.ts`
- Create: `web/apps/sidebar/src/wecom/wecom-bridge.test.ts`
- Modify: `web/apps/sidebar/src/main.tsx`
- Modify: `web/apps/sidebar/index.html`

**Interfaces:**

```ts
export type WeComMessage =
  | { type: 'text'; content: string }
  | { type: 'news'; link: string; title: string; description: string; imageUrl: string }
  | { type: 'image' | 'video' | 'file'; mediaId: string };

export function createWeComBridge(input: {
  request: SidebarRequest;
  agentId: () => string | null;
  href: () => string;
  sdk: () => WeComSDK | undefined;
}): WeComBridge;
```

- [ ] **Step 1: 写 SDK 缺失、签名失败、invoke 失败/成功的 RED**

```ts
await expect(bridge.sendChatMessage({ type: 'text', content: '你好' }))
  .rejects.toMatchObject({ kind: 'unavailable' });
expect(request).not.toHaveBeenCalled();
```

Verify `/agent/jssdkConfig?agentId=...&uriPath=...`, `agentConfig`, exact invoke payloads, one controlled retry, and no token/code in errors.

- [ ] **Step 2: 实现最小桥接并运行 GREEN**

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/wecom/wecom-bridge.test.ts`

- [ ] **Step 3: 接入运行时和官方脚本**

`index.html` loads the same two enterprise scripts already used by `web/legacy/sidebar/public/index.html`; production code accesses them through typed `window.wx`, never through direct fetch.

- [ ] **Step 4: 验证并提交**

Run: `corepack pnpm --filter @mochat/sidebar test && corepack pnpm --filter @mochat/sidebar typecheck && corepack pnpm --filter @mochat/sidebar build && git diff --check`

Commit: `feat(sidebar): add guarded wecom action bridge`

---

### Task 6：真实素材库查询、选择与发送

**Files:**

- Create: `web/apps/sidebar/src/features/medium/medium-api.ts`
- Create: `web/apps/sidebar/src/features/medium/medium-api.test.ts`
- Create: `web/apps/sidebar/src/features/medium/medium-page.tsx`
- Create: `web/apps/sidebar/src/features/medium/medium-page.test.tsx`
- Modify: `web/apps/sidebar/src/styles.css`
- Modify: `web/apps/sidebar/src/app/sidebar-router.tsx`

**Interfaces:**

- Produces: `loadMediumGroups`, `loadMediumPage`, `refreshMediumMediaId`
- Page types: 1 text, 2 image, 3 news, 5 video, 7 file; unsupported types are not rendered as sendable.

- [ ] **Step 1: API validator RED/GREEN**

Test raw Go `{ list, page:{total} }`, all content variants, missing mediaId, malformed content, query encoding and pagination.

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/medium/medium-api.test.ts`

- [ ] **Step 2: 页面筛选与空态 RED/GREEN**

Tests cover group, type, search submit, load-more, stale request cancellation, empty state, long file names, image alt text and no horizontal fixed width.

- [ ] **Step 3: 选择与发送 RED**

```tsx
it('reports partial send failure without claiming all items succeeded', async () => {
  wecom.sendChatMessage.mockResolvedValueOnce().mockRejectedValueOnce(new Error('企微拒绝'));
  await selectTwoAndSend();
  expect(screen.getByRole('alert')).toHaveTextContent('1 项发送成功，1 项发送失败');
});
```

Also cover zero selection, duplicate click, expired media ID refresh, bridge unavailable, retry and selection retention.

- [ ] **Step 4: 实现发送队列并运行 GREEN**

Send sequentially to keep deterministic feedback and avoid flooding WebView invoke. Never infer success before each `invoke` callback returns ok.

- [ ] **Step 5: 验证并提交**

Run: `corepack pnpm --filter @mochat/sidebar test && corepack pnpm --filter @mochat/sidebar lint && corepack pnpm --filter @mochat/sidebar typecheck && git diff --check`

Commit: `feat(sidebar): restore real material selection and sending`

---

### Task 7：个人客户 SOP 与客户群 SOP

**Files:**

- Create: `web/apps/sidebar/src/features/sop/sop-api.ts`
- Create: `web/apps/sidebar/src/features/sop/sop-api.test.ts`
- Create: `web/apps/sidebar/src/features/sop/sop-content.tsx`
- Create: `web/apps/sidebar/src/features/sop/contact-sop-page.tsx`
- Create: `web/apps/sidebar/src/features/sop/contact-sop-page.test.tsx`
- Create: `web/apps/sidebar/src/features/sop/room-sop-page.tsx`
- Create: `web/apps/sidebar/src/features/sop/room-sop-page.test.tsx`
- Modify: `web/apps/sidebar/src/app/sidebar-router.tsx`
- Modify: `web/apps/sidebar/src/styles.css`

**Interfaces:**

- Produces: `loadContactSOPInfo`, `loadContactSOPTips`, `loadRoomSOPInfo`, `markRoomSOPDone`
- `SOPContentItem` accepts validated `text` plus media URL variants; unknown variants display an explicit unsupported item, not a broken image.

- [ ] **Step 1: API shape RED/GREEN**

Cover creator/time/tipTime/contact/room/task content/state, invalid IDs, empty content and malformed server payload.

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/sop/sop-api.test.ts`

- [ ] **Step 2: 个人 SOP RED/GREEN**

With positive `id`, load single info; without id, resolve current contact and load tips. Test empty, copy success/failure, long text, image, no fake sent state, 401 and retry.

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/sop/contact-sop-page.test.tsx`

- [ ] **Step 3: 群 SOP RED/GREEN**

Test missing id sends no request; state 0 enables “我已完成”; state != 0 disables it; double click produces one PUT; failed PUT leaves pending; successful PUT refreshes from GET before showing completed.

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/sop/room-sop-page.test.tsx`

- [ ] **Step 4: 验证并提交**

Run: `corepack pnpm --filter @mochat/sidebar test && corepack pnpm --filter @mochat/sidebar lint && corepack pnpm --filter @mochat/sidebar typecheck && git diff --check`

Commit: `feat(sidebar): restore employee sop workflows`

---

### Task 8：批量加好友与首页信息架构

**Files:**

- Create: `web/apps/sidebar/src/features/batch-add/batch-add-api.ts`
- Create: `web/apps/sidebar/src/features/batch-add/batch-add-api.test.ts`
- Create: `web/apps/sidebar/src/features/batch-add/batch-add-page.tsx`
- Create: `web/apps/sidebar/src/features/batch-add/batch-add-page.test.tsx`
- Modify: `web/apps/sidebar/src/app/sidebar-router.tsx`
- Modify: `web/apps/sidebar/src/app/sidebar-router.test.tsx`
- Modify: `web/apps/sidebar/src/styles.css`

**Interfaces:**

- Produces: `loadBatchAddDetail(request, batchId, status)` with status 0..4.
- Page consumes `navigator.clipboard` through an injected copier and `wecom.navigateToAddCustomer` through the bridge.

- [ ] **Step 1: API RED/GREEN**

Test employeeName, phone/status rows, numeric status labels, malformed batchId, URL encoding and empty list.

- [ ] **Step 2: 页面 RED/GREEN**

Cover five filters, loading, empty, retry, long/invalid phone display, copy failure, JS-SDK unavailable, and one navigate call after a successful copy.

Run: `corepack pnpm --filter @mochat/sidebar exec vitest run src/features/batch-add/*.test.tsx src/features/batch-add/*.test.ts`

- [ ] **Step 3: 首页按业务域分组 RED/GREEN**

Assert “客户维护”“会话触达”“任务处理” groups contain only registered routes; no fixed customer totals/today/success text; each link keeps only applicable context.

- [ ] **Step 4: 验证并提交**

Run: `corepack pnpm --filter @mochat/sidebar test && corepack pnpm --filter @mochat/sidebar lint && corepack pnpm --filter @mochat/sidebar typecheck && corepack pnpm --filter @mochat/sidebar build && git diff --check`

Commit: `feat(sidebar): complete batch add and employee workbench`

---

### Task 9：移动 E2E、Docker 与视觉证据

**Files:**

- Modify: `web/e2e/tests/mobile-clients-foundation.spec.ts`
- Create: `web/e2e/tests/sidebar-employee-mobile.spec.ts`
- Create: `scripts/capture_sidebar_employee_mobile_evidence.mjs`
- Modify only if contract requires: `scripts/check_mobile_clients_foundation.mjs`

**Interfaces:**

- E2E fixtures are explicitly named `rawGoFixture*` and live only under test code.
- Evidence output directory: `output/playwright/sidebar-employee-mobile-2026-08-22/` (gitignored runtime evidence unless repository policy requires tracked artifacts).

- [ ] **Step 1: 更新现有 route cases 的 RED**

Replace “模块待迁移” expectations with actual route-specific headings; retain Operation cases byte-for-byte except formatting required by the test file.

Run: `corepack pnpm --filter @mochat/e2e exec playwright test tests/mobile-clients-foundation.spec.ts --workers=1`

Expected: RED until new route fixtures and assertions are wired.

- [ ] **Step 2: 添加 Sidebar 专项 E2E fixtures 和业务状态**

Cover all Sidebar endpoints, assert Authorization, no unexpected request/4xx/5xx, loading delay, success, empty, server error/retry, remark save/cancel/validation, tag append, portrait save, medium selection/SDK unavailable, SOP complete and batch filters.

- [ ] **Step 3: 覆盖视口与几何断言**

```ts
const viewports = [
  { name: '360x800', width: 360, height: 800 },
  { name: '390x844', width: 390, height: 844 },
  { name: '430x932', width: 430, height: 932 },
  { name: 'narrow-320x568', width: 320, height: 568 },
  { name: 'landscape-844x390', width: 844, height: 390 },
];
```

At every viewport assert `scrollWidth <= clientWidth`, last content above bottom nav, primary controls >=44px, focused input visible after keyboard-equivalent viewport resize, and no console/pageerror/unhandled rejection.

- [ ] **Step 4: 运行 Playwright GREEN**

Run: `corepack pnpm --filter @mochat/e2e typecheck && corepack pnpm --filter @mochat/e2e exec playwright test tests/mobile-clients-foundation.spec.ts tests/sidebar-employee-mobile.spec.ts --workers=1`

- [ ] **Step 5: Docker 环境验收（保留卷）**

Read `deploy/standalone/README.md` first. Use a unique Compose project name, `docker compose up -d --build` (never `down -v`), wait on health checks, open `/sidebar-app/`, run the same Playwright suite against Docker, then `docker compose stop` for only the task project.

- [ ] **Step 6: 捕获四组对照证据**

Generate 390×844 implementation screenshots for `/`, `/contact`, `/contactSop` or `/roomSop`, `/medium` empty state, plus 360×800 and 430×932 key pages. Record mapping:

- 图 1 → Sidebar 首页/“我的”导航；
- 图 2 → 个人/群 SOP；
- 图 3 → 客户工作区；
- 图 4 → 素材筛选/空态或批量加好友空态。

- [ ] **Step 7: 验证并提交**

Run: `corepack pnpm check:mobile-clients-foundation && git diff --check`

Commit: `test(sidebar): add full employee mobile acceptance`

---

### Task 10：最终门禁、审查与中文验收报告

**Files:**

- Create: `docs/reviews/2026-08-22-employee-sidebar-mobile-acceptance.zh-CN.md`
- Modify: this plan only to check completed boxes if repository convention permits

**Interfaces:**

- Report records command, timestamp, exit code, PASS/FAIL/SKIP, evidence path and external blocker.

- [ ] **Step 1: 运行 Sidebar 与共享层完整门禁**

```powershell
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar build
corepack pnpm --filter @mochat/mobile-foundation test
corepack pnpm --filter @mochat/mobile-foundation typecheck
corepack pnpm --filter @mochat/mobile-foundation lint
corepack pnpm --filter @mochat/mobile-foundation build
corepack pnpm check:mobile-clients-foundation
```

- [ ] **Step 2: 运行相关 workspace/E2E/Go 门禁**

```powershell
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e exec playwright test tests/mobile-clients-foundation.spec.ts tests/sidebar-employee-mobile.spec.ts --workers=1
```

若没有 Go diff，报告写 `Go/API tests: N/A（未修改 Go/API）`；若有 Go diff，运行 `go test ./internal/dashboard ./internal/server ./internal/store -count=1` 及精确合同测试。

- [ ] **Step 3: 运行仓库完整性检查**

```powershell
git diff --check
git status --short
git diff --name-only b9a47cab45ec872bc81e61a20b06a8a3529311b2...HEAD
git diff --exit-code b9a47cab45ec872bc81e61a20b06a8a3529311b2...HEAD -- docs/PROJECT_PROGRESS.zh-CN.md
```

确认没有 Operation/Dashboard/SaaS Admin/Phase 7 业务文件。

- [ ] **Step 4: 使用 `requesting-code-review` 做独立审查**

Provide reviewer with exact base/head, design path, plan path and requirements. Fix every Critical/Important issue with a new failing test before implementation; rerun affected and full gates.

- [ ] **Step 5: 编写中文验收报告**

Report must include: design/plan paths, base SHA, branch, commits, 12 route status, all PASS/FAIL/SKIP, screenshot evidence, data truth sources, external WeCom skips, known risks, rollback commit boundaries, Docker project/volume preservation and origin/main drift.

- [ ] **Step 6: 最终提交与 fresh verification**

Commit: `docs: record employee sidebar mobile acceptance`

After commit, rerun the exact full gate commands and `git status --short`. Completion may be claimed only from this fresh output.

---

## 测试矩阵

| 域 | 单元/组件 | 路由 | E2E | 失败分支 |
| --- | --- | --- | --- | --- |
| 鉴权 | session、codeAuth、安全 target | login/auth/codeAuth | 公开/受保护、401 恢复 | 非法参数、过期、外部 target、存储拒绝 |
| 客户 | API 校验、上下文、摘要、轨迹、画像 | contact | 延迟/内容/空/局部错误 | 401/403/404/409/5xx/断网 |
| 写入 | remark/tag/portrait | 三个子路由 | 保存/取消/校验/防重 | 保留输入、上传失败、旧标签锁定 |
| 素材 | 分组/列表/分页/mediaId | medium | 搜索/筛选/选择/部分失败 | SDK 缺失、签名失败、invoke 失败 |
| SOP | 个人/群 API 与内容 | contactSop/roomSop | 复制/完成/刷新 | 缺 id、空内容、重复提交、写失败 |
| 批量加好友 | 详情/状态映射 | contactBatchAdd | 五状态/复制/企微跳转 | 空态、剪贴板拒绝、SDK 缺失 |
| 壳层 | 导航、safe-area、44px | 全部 12 路由 | 5 个视口 | 横向溢出、底部遮挡、键盘等价 |

## 浏览器验收矩阵

| 视口 | 12 路由可达 | 客户长内容 | 表单键盘等价 | 底部安全区 | 截图 |
| --- | --- | --- | --- | --- | --- |
| 360×800 | 必测 | 必测 | 备注 | 必测 | 客户/SOP |
| 390×844 | 必测 | 必测 | 画像 | 必测 | 四组主证据 |
| 430×932 | 必测 | 必测 | 标签 | 必测 | 首页/素材 |
| 320×568 | 关键路由 | 必测 | 备注 | 必测 | 窄屏 |
| 844×390 | 关键路由 | 必测 | 画像 | 必测 | 横屏 |

## 提交与回滚策略

1. `docs: design ...`：设计基线，可独立保留。
2. `docs: plan ...`：实施契约，可独立保留。
3. `feat(sidebar): add validated ...`：领域契约；回滚不影响基线 UI。
4. `feat(sidebar): secure ...`：壳与授权；可单独回滚到旧壳。
5. `feat(sidebar): build/restore ...`：按客户、写入、企微/素材、SOP、批量原子回滚。
6. `test(sidebar): ...`：E2E 与证据，不改变生产行为。
7. `docs: record ...`：最终结果。

不 squash 掉安全/业务边界，不直接合入 main，不强推。若 `origin/main` 在任务期间前进，报告记录新 SHA 和“需显式 rebase/cherry-pick”，不盲目合并未验收工作。

## 计划自检

- 每项生产行为前都有明确 RED 命令和预期失败原因。
- 设计文档的 12 路由、数据真实性、鉴权、WebView、响应式和回滚要求均有对应任务。
- 文件责任互斥：API 校验、页面状态、JS-SDK、Router 组合没有混在单一大文件。
- 没有要求实现截图中仓库缺失的统计、订单、AI 会话或会话存档。
- 所有可控门禁有精确命令；企业微信真实宿主动作允许且仅允许因外部凭证/环境标记 SKIP。
