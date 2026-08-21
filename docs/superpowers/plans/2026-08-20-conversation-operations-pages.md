# 会话运营四页优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `subagent-driven-development`（推荐）或 `executing-plans` 逐任务实施。本计划使用复选框跟踪；每个任务都必须先取得失败测试证据，再写最小实现。

**Goal:** 将 `/chat/file-audio`、`/chat/resign-staff`、`/chat/refuse-archive`、`/customer/inheritance` 改造成结构统一、数据真实、交互完整且通过本地浏览器点击验收的会话运营页面。

**Architecture:** 新增 `conversation-operations` 前端领域目录和独立样式文件，复用已完成的会话归档 API，给拒绝存档与客户继承补类型化 API。后端只扩展现有音频筛选、拒绝存档筛选和客户继承 POST 同步别名，不新增表、不伪造字段。

**Tech Stack:** React 19、TypeScript、TanStack Query、React Router、Vitest、Testing Library、Go `net/http`、MySQL 5.7、Docker Compose、Codex in-app Browser。

## Global Constraints

- 设计基准：`docs/superpowers/specs/2026-08-20-conversation-operations-pages-design.md`。
- 所有用户审阅文案和新增业务文案使用中文；代码标识符与协议字段保持英文。
- 不修改左侧菜单、菜单名称、菜单顺序、图标和顶部 banner。
- 不复制圆弧 AI 的企业信息、示例数据、H5 地址和配置截图。
- 页面不设置 `1320px` 最大宽度；以 `2560×1440` 为主验收，同时回归 `1440×900`。
- 查询、重置和刷新只在查询操作区出现；页头不保留重复刷新按钮。
- 页面不得渲染“能力未接入”“功能未接入”类文案，也不得保留不能闭环的标签、禁用按钮或占位模块；必须后续专项建设的能力只登记到 `docs/PROJECT_PROGRESS.zh-CN.md`。
- 新行为严格 TDD；先运行并记录正确失败，再写最小实现。
- 不覆盖现有未提交改动；开始每个任务前运行 `git status --short` 并只暂存该任务文件。
- 不新增数据库表和迁移；SQL 必须兼容 MySQL 5.7。
- Docker 只重建 `app`；禁止 `down -v`、`down --volumes` 和删除 MySQL/Redis 命名卷。
- 真实浏览器验收不得在真实企业数据上确认同步或转接；只验证确认前路径和取消路径。写链路由 fake Provider 和前端 mock 测试验证。

---

## File Structure

### 新建

- `web/apps/dashboard/src/features/conversation-operations/conversation-operations-shell.tsx`：四页共享的紧凑页头、标签和查询操作区。
- `web/apps/dashboard/src/features/conversation-operations/conversation-operations-format.ts`：日期、状态、来源和缺失值格式化。
- `web/apps/dashboard/src/features/phase35/file-audio-api.test.ts`：音频列表、上传和删除不接受客户端企业覆盖的合同测试。
- `web/apps/dashboard/src/features/conversation-operations/resigned-employee-page.tsx`：离职员工三栏工作区编排。
- `web/apps/dashboard/src/features/conversation-operations/resigned-employee-page.test.tsx`：离职目录、会话、详情和 URL 行为测试。
- `web/apps/dashboard/src/features/conversation-global/employee-conversation-directory.test.tsx`：目录锁定为离职模式的回归测试。
- `web/apps/dashboard/src/features/conversation-global/employee-conversation-list.test.tsx`：离职页允许会话类型的回归测试。
- `web/apps/dashboard/src/features/conversation-operations/refuse-archive-api.ts`：拒绝存档类型化 API 和返回解析。
- `web/apps/dashboard/src/features/conversation-operations/refuse-archive-api.test.ts`：查询参数和解析测试。
- `web/apps/dashboard/src/features/conversation-operations/refuse-archive-page.tsx`：客户/群聊标签、筛选、分页和跟进抽屉。
- `web/apps/dashboard/src/features/conversation-operations/refuse-archive-page.test.tsx`：拒绝存档页面行为测试。
- `web/apps/dashboard/src/features/conversation-operations/contact-transfer-api.ts`：继承读取、同步、转接和日志类型化 API。
- `web/apps/dashboard/src/features/conversation-operations/contact-transfer-api.test.ts`：客户继承 API 合同测试。
- `web/apps/dashboard/src/features/conversation-operations/customer-inheritance-page.tsx`：离职/在职继承与继承记录。
- `web/apps/dashboard/src/features/conversation-operations/customer-inheritance-page.test.tsx`：继承读取、选择、确认和结果测试。
- `web/apps/dashboard/src/styles/conversation-operations.css`：四页独立布局与按钮样式。
- `web/apps/dashboard/src/styles/conversation-operations-layout.test.ts`：宽屏、响应式和交互态样式契约。
- `internal/modules/chat-media/transport/http/store_test.go`：音频日期过滤 SQL 合同测试。

### 修改

- `web/apps/dashboard/src/features/phase35/file-audio-api.ts`：添加上传日期过滤参数。
- `web/apps/dashboard/src/features/phase35/file-audio-page.tsx`：改为紧凑查询区、上传对话框和录音列表。
- `web/apps/dashboard/src/features/phase35/file-audio-page.test.tsx`：补 URL、上传、删除、播放失败、越界行为和后续专项入口缺席断言。
- `web/apps/dashboard/src/features/conversation-global/employee-conversation-list.tsx`：允许调用页限制可呈现的会话类型。
- `web/apps/dashboard/src/benchmark/page-registry.tsx`：注入三个新页面与客户继承 API。
- `web/apps/dashboard/src/benchmark/page-registry.test.tsx`：断言四个路由不再落到旧通用组件。
- `web/apps/dashboard/src/main.tsx`：创建客户继承/拒绝存档 API 并导入独立样式。
- `internal/modules/chat-media/transport/http/store.go`：按上传日期过滤真实音频记录。
- `internal/modules/chat-media/transport/http/handler.go`：解析和校验日期参数。
- `internal/modules/chat-media/transport/http/handler_test.go`：验证列表筛选、非法日期和企业范围。
- `internal/dashboard/phase33_closure.go`：扩展 `RefuseArchiveFilter`。
- `internal/dashboard/phase33_closure_handler.go`：接收拒绝存档新增筛选参数。
- `internal/dashboard/phase33_closure_handler_test.go`：验证筛选透传。
- `internal/store/phase33_closure.go`：追加真实 SQL 条件。
- `internal/dashboard/contact_transfer.go`：让待分配同步同时接受旧 GET 和新 POST。
- `internal/dashboard/contact_transfer_test.go`：验证 POST 同步别名和 fake Provider。
- `internal/server/server.go`：注册 `POST /dashboard/contactTransfer/sync`。
- `internal/server/server_test.go`：验证新路由暴露。
- `internal/dashboard/dashboard_page_catalog.json`：登记继承页实际读取和写入资源。
- `internal/dashboard/dashboard_access_guard_test.go`：验证资源权限。
- `web/apps/dashboard/src/styles/index.css`：只删除被新样式取代且仅服务旧四页的规则；不追加新规则。

## Shared Interfaces

```ts
export type QueryPage<T> = {
  items: readonly T[];
  total: number;
  page: number;
  perPage: number;
};

```

客户继承旧接口返回数组或 `{list,lastTime}`，前端 API 层必须转换成明确类型；页面组件不得再消费 `Record<string, unknown>`。

---

### Task 1: 建立共享页面骨架与样式契约

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-operations/conversation-operations-shell.tsx`
- Create: `web/apps/dashboard/src/features/conversation-operations/conversation-operations-format.ts`
- Create: `web/apps/dashboard/src/styles/conversation-operations.css`
- Create: `web/apps/dashboard/src/styles/conversation-operations-layout.test.ts`
- Modify: `web/apps/dashboard/src/main.tsx`

**Interfaces:**

- Produces: `ConversationOperationsShell`、`ConversationTabs`、`ConversationQueryBar`、`formatDateTime`、`displayValue`。
- Consumes: React `ReactNode`；不依赖任何领域 API。

- [ ] **Step 1: 写失败的布局契约测试**

```ts
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const css = readFileSync(new URL('./conversation-operations.css', import.meta.url), 'utf8');

describe('conversation operations layout contract', () => {
  it('fills wide screens and defines the resigned employee three-column grid', () => {
    expect(css).toContain('.conversation-operations-page');
    expect(css).toMatch(/max-width:\s*none/);
    expect(css).toMatch(/grid-template-columns:\s*300px\s+minmax\(360px,\s*0\.9fr\)\s+minmax\(560px,\s*1\.35fr\)/);
  });

  it('does not move controls on hover and collapses below 1024px', () => {
    expect(css).not.toMatch(/:hover[^}]*transform\s*:/s);
    expect(css).toMatch(/@media\s*\(max-width:\s*1024px\)/);
    expect(css).toMatch(/\.resigned-employee-workspace[^{]*\{[^}]*grid-template-columns:\s*1fr/s);
  });
});
```

- [ ] **Step 2: 运行测试并确认因样式文件不存在而失败**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/styles/conversation-operations-layout.test.ts`

Expected: FAIL，错误包含 `ENOENT conversation-operations.css`。

- [ ] **Step 3: 实现共享组件与格式化函数**

```tsx
import type { FormEvent, ReactNode } from 'react';

export function ConversationOperationsShell(props: {
  title: string;
  description: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return <section className="conversation-operations-page">
    <header className="conversation-operations-header">
      <div><h1>{props.title}</h1><p>{props.description}</p></div>
      {props.actions && <div className="conversation-operations-header-actions">{props.actions}</div>}
    </header>
    {props.children}
  </section>;
}

export function ConversationTabs<T extends string>(props: {
  value: T;
  tabs: readonly { value: T; label: string }[];
  onChange(value: T): void;
}) {
  return <div className="conversation-operations-tabs" role="tablist">
    {props.tabs.map((tab) => <button
      aria-selected={props.value === tab.value}
      key={tab.value}
      onClick={() => props.onChange(tab.value)}
      role="tab"
      type="button"
    >{tab.label}</button>)}
  </div>;
}

export function ConversationQueryBar(props: {
  children: ReactNode;
  fetching?: boolean;
  onQuery(): void;
  onReset(): void;
  onRefresh(): void;
}) {
  const submit = (event: FormEvent) => { event.preventDefault(); props.onQuery(); };
  return <form className="conversation-operations-query" onSubmit={submit}>
    <div className="conversation-operations-query-fields">{props.children}</div>
    <div className="conversation-operations-query-actions">
      <button className="is-primary" disabled={props.fetching} type="submit">查询</button>
      <button disabled={props.fetching} onClick={props.onReset} type="button">重置</button>
      <button disabled={props.fetching} onClick={props.onRefresh} type="button">{props.fetching ? '刷新中…' : '刷新'}</button>
    </div>
  </form>;
}
```

```ts
export function displayValue(value: unknown): string {
  return value === null || value === undefined || value === '' ? '--' : String(value);
}

export function formatDateTime(value: string): string {
  if (value.trim() === '') return '--';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false,
  }).format(date).replaceAll('/', '-');
}
```

- [ ] **Step 4: 实现宽屏、按钮、表格、抽屉和响应式样式**

`conversation-operations.css` 至少写入以下不可变合同：

```css
.conversation-operations-page { display: grid; gap: 16px; margin: 0; max-width: none; min-width: 0; }
.conversation-operations-header { align-items: flex-end; display: flex; gap: 24px; justify-content: space-between; padding: 4px 0 0; }
.conversation-operations-header h1 { color: #20283a; font-size: 24px; margin: 0; }
.conversation-operations-header p { color: #737b8c; margin: 8px 0 0; }
.conversation-operations-tabs { border-bottom: 1px solid #e9edf4; display: flex; gap: 28px; }
.conversation-operations-tabs button { background: transparent; border: 0; border-bottom: 2px solid transparent; color: #687287; padding: 12px 2px; }
.conversation-operations-tabs button[aria-selected='true'] { border-bottom-color: #536bf4; color: #3046bd; font-weight: 650; }
.conversation-operations-query { align-items: end; background: #fff; border: 1px solid #e7eaf2; border-radius: 12px; display: flex; gap: 16px; justify-content: space-between; padding: 14px 16px; }
.conversation-operations-query-fields { align-items: end; display: flex; flex: 1; flex-wrap: wrap; gap: 12px; }
.conversation-operations-query-actions { display: flex; gap: 10px; }
.conversation-operations-page button { background: #fff; border: 1px solid #dfe4ee; border-radius: 8px; color: #536bf4; cursor: pointer; padding: 9px 15px; }
.conversation-operations-page button.is-primary { background: #536bf4; border-color: #536bf4; color: #fff; }
.conversation-operations-page button:hover:not(:disabled) { background: #eef2ff; border-color: #8797ef; }
.conversation-operations-page button.is-primary:hover:not(:disabled) { background: #4059df; border-color: #4059df; }
.conversation-operations-page button:disabled { color: #98a2b3; cursor: not-allowed; opacity: .65; }
.resigned-employee-workspace { display: grid; gap: 12px; grid-template-columns: 300px minmax(360px, 0.9fr) minmax(560px, 1.35fr); min-height: calc(100vh - 245px); }
@media (max-width: 1024px) {
  .conversation-operations-query { align-items: stretch; flex-direction: column; }
  .conversation-operations-query-actions { flex-wrap: wrap; }
  .resigned-employee-workspace { grid-template-columns: 1fr; }
}
```

- [ ] **Step 5: 导入样式并运行测试**

在 `main.tsx` 的 `./styles/index.css` 后添加：

```ts
import './styles/conversation-operations.css';
```

Run: `corepack pnpm --filter @mochat/dashboard test -- src/styles/conversation-operations-layout.test.ts`

Expected: PASS，2 tests。

- [ ] **Step 6: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-operations/conversation-operations-shell.tsx web/apps/dashboard/src/features/conversation-operations/conversation-operations-format.ts web/apps/dashboard/src/styles/conversation-operations.css web/apps/dashboard/src/styles/conversation-operations-layout.test.ts web/apps/dashboard/src/main.tsx
git commit -m "feat: add conversation operations page shell"
```

---

### Task 2: 扩展真实音频列表的日期过滤合同

**Files:**

- Modify: `internal/modules/chat-media/transport/http/store.go`
- Create: `internal/modules/chat-media/transport/http/store_test.go`
- Modify: `internal/modules/chat-media/transport/http/handler.go`
- Modify: `internal/modules/chat-media/transport/http/handler_test.go`
- Modify: `web/apps/dashboard/src/features/phase35/file-audio-api.ts`
- Create: `web/apps/dashboard/src/features/phase35/file-audio-api.test.ts`
- Modify: `web/apps/dashboard/src/features/phase35/file-audio-page.test.tsx`

**Interfaces:**

- Produces: `MediaListFilter{CorpID,Page,PerPage,Keyword,CreatedFrom,CreatedTo}`。
- Produces: `FileAudioApi.list(page,perPage,{keyword,createdFrom,createdTo})`、`upload(file)`、`remove(id)`；企业范围只用于前端 query key，不写入请求参数。

- [ ] **Step 1: 写失败的 handler 与前端 API 测试**

后端测试新增：

```go
func TestMediaListPassesValidatedDateFilter(t *testing.T) {
    store := &fakeMediaStore{}
    handler := newTestMediaHandler(t, store)
    req := authenticatedMediaRequest(http.MethodGet, "/dashboard/chat/media?page=2&perPage=20&keyword=%E5%9B%9E%E8%AE%BF&createdFrom=2026-08-01&createdTo=2026-08-20", nil)
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)
    if rec.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()) }
    if store.lastFilter.CreatedFrom != "2026-08-01" || store.lastFilter.CreatedTo != "2026-08-20" { t.Fatalf("filter=%+v", store.lastFilter) }
}

func TestMediaListRejectsInvalidDateRange(t *testing.T) {
    handler := newTestMediaHandler(t, &fakeMediaStore{})
    req := authenticatedMediaRequest(http.MethodGet, "/dashboard/chat/media?createdFrom=2026-08-20&createdTo=2026-08-01", nil)
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)
    if rec.Code != http.StatusBadRequest { t.Fatalf("status=%d", rec.Code) }
}
```

前端测试新增：

```ts
it('sends keyword and upload date filters', async () => {
  const request = vi.fn().mockResolvedValue({ list: [], total: 0, page: 1, perPage: 20 });
  const api = createFileAudioApi({ request });
  await api.list(1, 20, { keyword: '回访', createdFrom: '2026-08-01', createdTo: '2026-08-20' });
  expect(request).toHaveBeenCalledWith('/chat/media?page=1&perPage=20&keyword=%E5%9B%9E%E8%AE%BF&createdFrom=2026-08-01&createdTo=2026-08-20');
});
```

- [ ] **Step 2: 运行并确认类型或签名不匹配失败**

Run: `go test ./internal/modules/chat-media/transport/http -run 'TestMediaList(PassesValidatedDateFilter|RejectsInvalidDateRange)' -count=1`

Expected: FAIL，`MediaListFilter` 或 `lastFilter` 尚不存在。

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/phase35/file-audio-api.test.ts src/features/phase35/file-audio-page.test.tsx`

Expected: FAIL，`list` 仍接收字符串关键字。

- [ ] **Step 3: 引入过滤类型并实现 MySQL 5.7 条件**

```go
type MediaListFilter struct {
    CorpID int64
    Page int
    PerPage int
    Keyword string
    CreatedFrom string
    CreatedTo string
}

type MediaStore interface {
    Create(context.Context, AudioObject) (int64, error)
    GetByID(context.Context, int64) (*AudioObject, error)
    List(context.Context, MediaListFilter) (ListResult, error)
    SoftDelete(context.Context, int64, int64) error
}
```

在 `SQLMediaStore.List` 中使用：

```go
if filter.CreatedFrom != "" {
    where += " AND created_at >= ?"
    args = append(args, filter.CreatedFrom+" 00:00:00")
}
if filter.CreatedTo != "" {
    where += " AND created_at < DATE_ADD(?, INTERVAL 1 DAY)"
    args = append(args, filter.CreatedTo+" 00:00:00")
}
```

- [ ] **Step 4: 在 handler 校验日期并传递过滤对象**

```go
func mediaDateRange(q url.Values) (string, string, error) {
    from, to := strings.TrimSpace(q.Get("createdFrom")), strings.TrimSpace(q.Get("createdTo"))
    for _, value := range []string{from, to} {
        if value == "" { continue }
        if _, err := time.Parse("2006-01-02", value); err != nil { return "", "", errors.New("上传日期格式必须为 YYYY-MM-DD") }
    }
    if from != "" && to != "" && from > to { return "", "", errors.New("上传开始日期不能晚于结束日期") }
    return from, to, nil
}
```

`corpId` 查询参数继续忽略，企业范围只使用登录 principal 的 `CorpID`。

- [ ] **Step 5: 修改前端 API 签名**

```ts
export type AudioListFilter = { keyword: string; createdFrom: string; createdTo: string };

list(page: number, perPage: number, filter: AudioListFilter): Promise<AudioListResult> {
  const params = new URLSearchParams({ page: String(page), perPage: String(perPage) });
  if (filter.keyword.trim()) params.set('keyword', filter.keyword.trim());
  if (filter.createdFrom) params.set('createdFrom', filter.createdFrom);
  if (filter.createdTo) params.set('createdTo', filter.createdTo);
  return client.request<AudioListResult>(`/chat/media?${params.toString()}`);
}
```

同时将 `upload(corpId,file)` 改为 `upload(file)`，请求地址固定 `/chat/media`；将 `remove(corpId,id)` 改为 `remove(id)`，请求地址固定 `/chat/media/{id}`。handler 始终从登录 principal 取得企业。

- [ ] **Step 6: 运行目标测试**

Run: `go test ./internal/modules/chat-media/transport/http -count=1`

Expected: PASS。

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/phase35/file-audio-api.test.ts src/features/phase35/file-audio-page.test.tsx`

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add internal/modules/chat-media/transport/http/store.go internal/modules/chat-media/transport/http/store_test.go internal/modules/chat-media/transport/http/handler.go internal/modules/chat-media/transport/http/handler_test.go web/apps/dashboard/src/features/phase35/file-audio-api.ts web/apps/dashboard/src/features/phase35/file-audio-api.test.ts web/apps/dashboard/src/features/phase35/file-audio-page.test.tsx
git commit -m "feat: filter uploaded recordings by date"
```

---

### Task 3: 重构文件录音页面

**Files:**

- Modify: `web/apps/dashboard/src/features/phase35/file-audio-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/file-audio-page.test.tsx`
- Modify: `web/apps/dashboard/src/styles/conversation-operations.css`

**Interfaces:**

- Consumes: Task 1 页面骨架，Task 2 `AudioListFilter`。
- Produces: URL 参数 `keyword`、`createdFrom`、`createdTo`、`page`。

- [ ] **Step 1: 写失败的页面行为测试**

```tsx
it('shows real recordings without rendering deferred archive-media entries', async () => {
  renderPage(api, '/chat/file-audio');
  expect(await screen.findByText('p36-accept.wav')).toBeTruthy();
  expect(screen.queryByRole('tab', { name: '聊天文件' })).toBeNull();
  expect(screen.queryByText(/能力未接入|功能未接入/)).toBeNull();
});

it('keeps upload in a dialog and closes through cancel, backdrop and Escape', async () => {
  renderPage(api, '/chat/file-audio');
  fireEvent.click(await screen.findByRole('button', { name: '上传录音' }));
  expect(screen.getByRole('dialog', { name: '上传录音' })).toBeTruthy();
  fireEvent.keyDown(document, { key: 'Escape' });
  expect(screen.queryByRole('dialog', { name: '上传录音' })).toBeNull();
});

it('queries upload dates and resets URL state', async () => {
  renderPage(api, '/chat/file-audio?page=3');
  fireEvent.change(screen.getByLabelText('上传开始日期'), { target: { value: '2026-08-01' } });
  fireEvent.change(screen.getByLabelText('上传结束日期'), { target: { value: '2026-08-20' } });
  fireEvent.click(screen.getByRole('button', { name: '查询' }));
  await waitFor(() => expect(api.list).toHaveBeenLastCalledWith(1, 20, expect.objectContaining({ createdFrom: '2026-08-01', createdTo: '2026-08-20' })));
});
```

- [ ] **Step 2: 运行并确认标签和对话框断言失败**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/phase35/file-audio-page.test.tsx`

Expected: FAIL，页面仍是大块上传区，且找不到“上传录音”对话框。

- [ ] **Step 3: 用共享骨架重组页面**

页面顶层必须是：

```tsx
<ConversationOperationsShell
  title="文件录音"
  description="管理录音文件的上传、筛选、播放和删除"
  actions={<button className="is-primary" onClick={() => setUploadOpen(true)} type="button">上传录音</button>}
>
  <RecordingQueryBar />
  <RecordingList />
</ConversationOperationsShell>
```

查询区只放文件名、上传开始日期、上传结束日期、查询、重置、刷新。表格列固定为“文件名、格式、大小、时长、上传时间、数据来源、播放、操作”；数据来源固定文案“人工上传”，因为来源就是当前接口合同而非模拟字段。

- [ ] **Step 4: 将上传区改为可关闭对话框**

使用现有 `DashboardDialog` 或同等可访问对话框：

```tsx
<DashboardDialog open={uploadOpen} title="上传录音" confirmText={upload.isPending ? '上传中…' : '上传'}
  confirmDisabled={!selectedFile || upload.isPending} onCancel={closeUpload} onConfirm={submitUpload}>
  <label>录音文件<input ref={fileInput} aria-label="选择音频文件" accept="audio/*" type="file" onChange={selectFile} /></label>
  <p>支持 WAV、MP3、OGG、FLAC、M4A、AAC、AMR、WebM，单个文件不超过 50MB。</p>
</DashboardDialog>
```

删除使用 `DashboardDialog danger`，不再调用 `window.confirm`。删除当前页最后一条后执行：

```ts
if (items.length === 1 && page > 1) setPage(page - 1);
else void list.refetch();
```

- [ ] **Step 5: 补音频错误态与样式**

每行用 `onError` 标记播放失败，显示“音频加载失败”和重试按钮；不影响其他音频。`durationSeconds <= 0` 渲染 `--`。

- [ ] **Step 6: 运行目标测试**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/phase35/file-audio-page.test.tsx src/styles/conversation-operations-layout.test.ts`

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add web/apps/dashboard/src/features/phase35/file-audio-page.tsx web/apps/dashboard/src/features/phase35/file-audio-page.test.tsx web/apps/dashboard/src/styles/conversation-operations.css
git commit -m "feat: redesign file audio workspace"
```

---

### Task 4: 实现离职员工三栏会话工作区

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-operations/resigned-employee-page.tsx`
- Create: `web/apps/dashboard/src/features/conversation-operations/resigned-employee-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/employee-conversation-directory.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/employee-conversation-directory.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/employee-conversation-list.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/employee-conversation-list.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/styles/conversation-operations.css`

**Interfaces:**

- Consumes: `ConversationGlobalApi.staffDirectory`、`search`、`staffDetail`。
- Reuses: `EmployeeConversationDirectory`、`EmployeeConversationList`、`EmployeeConversationDetail`；目录增加 `lockedMode="departed"`，列表增加 `allowedTypes`，隐藏不属于本页闭环范围的入口。
- Produces: `/chat/resign-staff` URL 状态。

- [ ] **Step 1: 写失败的三栏与数据口径测试**

```tsx
it('requests only departed employees and opens their real conversations', async () => {
  const api = createConversationApiMock();
  renderPage(api, '/chat/resign-staff');
  await waitFor(() => expect(api.staffDirectory).toHaveBeenCalledWith(expect.objectContaining({ mode: 'departed', pageSize: 50 })));
  fireEvent.click(await screen.findByRole('button', { name: /张三.*已离职/ }));
  await waitFor(() => expect(api.search).toHaveBeenCalledWith(expect.objectContaining({ employeeIds: ['9'], pageSize: 20 })));
  fireEvent.click(await screen.findByRole('button', { name: /星河科技/ }));
  await waitFor(() => expect(api.staffDetail).toHaveBeenCalledWith(expect.objectContaining({ conversationId: '9:1:31', pageSize: 50 })));
});

it('distinguishes no departed employee from a departed employee without conversations', async () => {
  const api = createConversationApiMock({ directoryEmployees: [{ ...employee, conversationCount: 0 }], conversations: [] });
  renderPage(api, '/chat/resign-staff?employeeId=9');
  expect(await screen.findByText('该离职员工当前没有可查看的归档会话')).toBeTruthy();
  expect(screen.queryByText('暂无离职员工')).toBeNull();
});
```

- [ ] **Step 2: 运行并确认路由仍渲染旧原始字段表**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-operations/resigned-employee-page.test.tsx`

Expected: FAIL，新页面文件不存在。

- [ ] **Step 3: 实现 URL 解析和查询编排**

使用以下固定默认值：

```ts
const staffPageSize = 50 as const;
const conversationPageSize = 20 as const;
const messagePageSize = 50 as const;
```

目录请求必须固定：

```ts
api.staffDirectory?.({ mode: 'departed', keyword: employeeKeyword, departmentId, page: employeePage, pageSize: staffPageSize })
```

会话请求必须固定 `employeeIds: [String(employeeId)]`；详情使用 `staffDetail`。非法 `employeeId`、`page`、`conversationType` 和日期在 `useEffect` 中从 URL 删除。

- [ ] **Step 4: 锁定离职目录模式**

给 `EmployeeConversationDirectory` 增加可选属性：

```ts
lockedMode?: StaffDirectoryMode;
```

当 `lockedMode` 存在时，不渲染“企业架构/重点关注”和“全部成员/存档成员/离职成员”模式切换按钮，但保留部门树、员工搜索和员工列表。新增回归测试：

```tsx
it('locks the directory to departed mode without exposing other mode switches', () => {
  render(<EmployeeConversationDirectory {...props} lockedMode="departed" mode="departed" />);
  expect(screen.queryByRole('tab', { name: '重点关注' })).toBeNull();
  expect(screen.queryByRole('button', { name: /全部成员/ })).toBeNull();
  expect(screen.getByRole('button', { name: /张三.*已离职/ })).toBeTruthy();
});
```

给 `EmployeeConversationList` 增加可选属性：

```ts
allowedTypes?: readonly ConversationType[];
```

默认值保持现有员工会话页的类型集合；离职员工页传入 `['', 'customer', 'room', 'employee']`。组件只渲染传入集合中的筛选按钮，并在测试中断言内部群入口及建设进度类文案均不存在。

- [ ] **Step 5: 组合三栏组件和选择规则**

```tsx
<ConversationOperationsShell title="离职员工" description="查看离职员工的真实归档会话和历史消息">
  <ConversationQueryBar fetching={directory.isFetching || conversations.isFetching} onQuery={applyFilters} onReset={resetFilters} onRefresh={refreshAll}>
    <label>开始日期<input aria-label="会话开始日期" type="date" value={startDraft} onChange={(event) => setStartDraft(event.target.value)} /></label>
    <label>结束日期<input aria-label="会话结束日期" type="date" value={endDraft} onChange={(event) => setEndDraft(event.target.value)} /></label>
  </ConversationQueryBar>
  <div className="resigned-employee-workspace">
    <EmployeeConversationDirectory
      data={directory.data}
      employees={directory.data?.employees ?? []}
      error={asError(directory.error)}
      fetching={directory.isFetching}
      hasMore={false}
      keywordDraft={keywordDraft}
      lockedMode="departed"
      mode="departed"
      onDepartmentChange={selectDepartment}
      onKeywordDraftChange={setKeywordDraft}
      onLoadMore={() => undefined}
      onModeChange={() => undefined}
      onRefresh={() => void directory.refetch()}
      onSearch={submitEmployeeSearch}
      onSelectEmployee={selectEmployee}
      pending={directory.isPending}
      selectedDepartmentId={departmentId}
      selectedEmployeeId={employeeId}
    />
    <EmployeeConversationList
      data={conversations.data}
      employee={selectedEmployee}
      error={asError(conversations.error)}
      fetching={conversations.isFetching}
      allowedTypes={['', 'customer', 'room', 'employee']}
      onPageChange={selectConversationPage}
      onRefresh={() => void conversations.refetch()}
      onSelectConversation={selectConversation}
      onTypeChange={selectConversationType}
      page={page}
      pending={conversations.isPending}
      selectedConversationId={conversationId}
      selectedConversationRowId={conversationRowId}
      type={conversationType}
    />
    <EmployeeConversationDetail
      data={detail.data}
      date={messageDate}
      error={asError(detail.error)}
      fetching={detail.isFetching}
      focusError={focusError}
      focusPending={focusMutation.isPending}
      hasMore={Boolean(detail.hasNextPage)}
      keyword={messageKeywordDraft}
      loadingOlder={detail.isFetchingNextPage}
      messages={detailMessages}
      messageTypes={messageTypes}
      onDateChange={selectMessageDate}
      onKeywordChange={setMessageKeywordDraft}
      onKeywordSearch={submitMessageKeyword}
      onLoadOlder={() => void detail.fetchNextPage()}
      onRefresh={() => void detail.refetch()}
      onToggleFocus={() => focusMutation.mutate()}
      onToggleMessageType={toggleMessageType}
      pending={detail.isPending}
    />
  </div>
</ConversationOperationsShell>
```

选择员工时删除 `conversationId` 和详情筛选；选择会话类型/日期时删除当前会话；整行点击进入详情，内部独立控件调用 `stopPropagation()`。

- [ ] **Step 6: 替换路由注册**

在 `createBenchmarkP0Pages` 中直接使用已有 `conversationGlobalApi`：

```tsx
'/chat/resign-staff': <ResignedEmployeePage api={conversationGlobalApi} />,
```

删除旧 `CustomerTransferPage mode="resign"` 注册，不删除旧组件文件，避免波及其他未完成改动。

- [ ] **Step 7: 运行测试**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-operations/resigned-employee-page.test.tsx src/features/conversation-global/employee-conversation-*.test.tsx`

Expected: PASS。

- [ ] **Step 8: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-operations/resigned-employee-page.tsx web/apps/dashboard/src/features/conversation-operations/resigned-employee-page.test.tsx web/apps/dashboard/src/features/conversation-global/employee-conversation-directory.tsx web/apps/dashboard/src/features/conversation-global/employee-conversation-directory.test.tsx web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/styles/conversation-operations.css
git commit -m "feat: add resigned employee conversation workspace"
```

---

### Task 5: 扩展拒绝存档真实筛选

**Files:**

- Modify: `internal/dashboard/phase33_closure.go`
- Modify: `internal/dashboard/phase33_closure_handler.go`
- Modify: `internal/dashboard/phase33_closure_handler_test.go`
- Modify: `internal/store/phase33_closure.go`

**Interfaces:**

- Produces: `RefuseArchiveFilter.SubjectType`、`Employee`、`RefusedFrom`、`RefusedTo`。
- Preserves: `RefuseArchivePage{Items,Total,Page,PerPage}`。

- [ ] **Step 1: 写失败的 handler 透传测试**

```go
func TestPhase33ClosureRefuseRecordsScopesAllFilters(t *testing.T) {
    h, provider := newClosureHandler()
    req := authenticatedClosureRequest(http.MethodGet, "/dashboard/refuse-archive/records?subjectType=room&subject=%E4%BA%A7%E5%93%81%E7%BE%A4&employee=%E5%BC%A0&authorizationStatus=refused&followUpStatus=waiting&refusedFrom=2026-08-01&refusedTo=2026-08-20&page=2&perPage=20", nil)
    rec := httptest.NewRecorder()
    h.RefuseRecords(rec, req)
    if rec.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()) }
    got := provider.lastRefuseFilter
    if got.SubjectType != "room" || got.Employee != "张" || got.RefusedFrom != "2026-08-01" || got.RefusedTo != "2026-08-20" || got.Page != 2 || got.PerPage != 20 { t.Fatalf("filter=%+v", got) }
}
```

- [ ] **Step 2: 运行并确认结构字段不存在**

Run: `go test ./internal/dashboard -run TestPhase33ClosureRefuseRecordsScopesAllFilters -count=1`

Expected: FAIL，`RefuseArchiveFilter` 缺少字段。

- [ ] **Step 3: 扩展过滤类型和日期校验**

```go
type RefuseArchiveFilter struct {
    TenantID int
    CorpID int
    SubjectType string
    Subject string
    Employee string
    AuthorizationStatus string
    FollowUpStatus string
    RefusedFrom string
    RefusedTo string
    Page int
    PerPage int
}
```

handler 只接受 `subjectType` 的 `customer`、`room` 或空值；日期使用 `2006-01-02`，开始晚于结束返回 400。

- [ ] **Step 4: 追加 SQL 条件**

```go
if f.SubjectType != "" { w += " AND subject_type=?"; a = append(a, f.SubjectType) }
if f.Subject != "" { w += " AND (subject_name LIKE ? OR subject_id LIKE ?)"; term := "%"+strings.TrimSpace(f.Subject)+"%"; a = append(a, term, term) }
if f.Employee != "" { w += " AND employee_name LIKE ?"; a = append(a, "%"+strings.TrimSpace(f.Employee)+"%") }
if f.RefusedFrom != "" { w += " AND refused_at>=?"; a = append(a, f.RefusedFrom+" 00:00:00") }
if f.RefusedTo != "" { w += " AND refused_at<DATE_ADD(?, INTERVAL 1 DAY)"; a = append(a, f.RefusedTo+" 00:00:00") }
```

保留 `tenant_id/corp_id` 为第一层范围，禁止先查全表再在 Go 里过滤。

- [ ] **Step 5: 运行 dashboard 与 store 目标测试**

Run: `go test ./internal/dashboard ./internal/store -run 'RefuseArchive|RefuseRecords' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add internal/dashboard/phase33_closure.go internal/dashboard/phase33_closure_handler.go internal/dashboard/phase33_closure_handler_test.go internal/store/phase33_closure.go
git commit -m "feat: filter refuse archive records"
```

---

### Task 6: 实现类型化拒绝存档页面

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-operations/refuse-archive-api.ts`
- Create: `web/apps/dashboard/src/features/conversation-operations/refuse-archive-api.test.ts`
- Create: `web/apps/dashboard/src/features/conversation-operations/refuse-archive-page.tsx`
- Create: `web/apps/dashboard/src/features/conversation-operations/refuse-archive-page.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/styles/conversation-operations.css`

**Interfaces:**

```ts
export type RefuseArchiveRecord = {
  id: number; subjectType: 'customer' | 'room'; subjectId: string; subjectName: string;
  employeeId: number; employeeName: string; authorizationStatus: 'refused' | 'pending' | 'authorized' | 'expired';
  source: string; refusedAt: string; authorizedAt: string; lastFollowUpAt: string;
  followUpStatus: '' | 'unfollowed' | 'contacted' | 'waiting' | 'completed'; followUpNote: string;
};
```

- [ ] **Step 1: 写失败的 API 解析测试**

```ts
it('builds all filters and rejects malformed rows', async () => {
  const request = vi.fn().mockResolvedValue({ items: [record], total: 1, page: 2, perPage: 20 });
  const api = createRefuseArchiveApi({ request });
  await expect(api.list({ subjectType: 'customer', subject: '何静', employee: '周涛', authorizationStatus: 'refused', followUpStatus: '', refusedFrom: '2026-08-01', refusedTo: '2026-08-20', page: 2, perPage: 20 })).resolves.toMatchObject({ total: 1 });
  expect(request).toHaveBeenCalledWith(expect.stringContaining('subjectType=customer'));
});
```

- [ ] **Step 2: 写失败的页面行为测试**

```tsx
it('switches between customer and room records and keeps fixed pagination', async () => {
  renderPage(api, '/chat/refuse-archive?archiveTab=customer&page=2');
  await waitFor(() => expect(api.list).toHaveBeenCalledWith(expect.objectContaining({ subjectType: 'customer', page: 2, perPage: 20 })));
  fireEvent.click(screen.getByRole('tab', { name: '群聊' }));
  await waitFor(() => expect(api.list).toHaveBeenLastCalledWith(expect.objectContaining({ subjectType: 'room', page: 1, perPage: 20 })));
});

it('preserves a failed follow-up draft and closes the drawer in three ways', async () => {
  api.followUp.mockRejectedValueOnce(new Error('保存失败'));
  renderPage(api, '/chat/refuse-archive');
  fireEvent.click(await screen.findByRole('button', { name: '跟进 何静' }));
  fireEvent.change(screen.getByLabelText('跟进备注'), { target: { value: '已电话沟通' } });
  fireEvent.click(screen.getByRole('button', { name: '保存跟进' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('保存失败');
  expect(screen.getByLabelText('跟进备注')).toHaveValue('已电话沟通');
});
```

- [ ] **Step 3: 运行并确认新 API/页面不存在**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-operations/refuse-archive-api.test.ts src/features/conversation-operations/refuse-archive-page.test.tsx`

Expected: FAIL，模块不存在。

- [ ] **Step 4: 实现严格返回解析与 API**

```ts
export function createRefuseArchiveApi(client: Client) {
  return {
    async list(input: RefuseArchiveListInput): Promise<RefuseArchivePage> {
      const query = new URLSearchParams(Object.entries(input).filter(([, value]) => value !== '').map(([key, value]) => [key, String(value)]));
      return parseRefuseArchivePage(await client.request(`/refuse-archive/records?${query}`));
    },
    followUp(input: { id: number; status: 'contacted' | 'waiting' | 'completed'; note: string }) {
      return client.request('/refuse-archive/follow-up', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    },
  };
}
```

解析失败抛出“拒绝存档接口返回了无效数据”，不得静默丢弃坏行。

- [ ] **Step 5: 实现页面与跟进抽屉**

状态映射使用常量：

```ts
const authorizationLabels = { refused: '已拒绝', pending: '待授权', authorized: '已授权', expired: '已过期' } as const;
const followUpLabels = { '': '未跟进', unfollowed: '未跟进', contacted: '已联系', waiting: '等待授权', completed: '跟进完成' } as const;
```

没有 `#manage` 权限时操作列显示 `--`。跟进抽屉实现关闭按钮、backdrop 和 `Escape`，保存成功后清除选中项并 invalidate 当前企业拒绝存档 query。

- [ ] **Step 6: 替换路由注册与依赖注入**

`main.tsx` 创建 `refuseArchiveApi`，`page-registry.tsx` 的 `/chat/refuse-archive` 改为新页面。旧 `phase33-closure-pages.tsx` 中的同名导出暂不删除，避免格式化整个混合文件；registry 不再引用它。

- [ ] **Step 7: 运行测试**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-operations/refuse-archive-api.test.ts src/features/conversation-operations/refuse-archive-page.test.tsx src/benchmark/page-registry.test.tsx`

Expected: PASS。

- [ ] **Step 8: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-operations/refuse-archive-api.ts web/apps/dashboard/src/features/conversation-operations/refuse-archive-api.test.ts web/apps/dashboard/src/features/conversation-operations/refuse-archive-page.tsx web/apps/dashboard/src/features/conversation-operations/refuse-archive-page.test.tsx web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/main.tsx web/apps/dashboard/src/styles/conversation-operations.css
git commit -m "feat: redesign refuse archive records"
```

---

### Task 7: 建立客户继承类型化 API 与 POST 同步别名

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-operations/contact-transfer-api.ts`
- Create: `web/apps/dashboard/src/features/conversation-operations/contact-transfer-api.test.ts`
- Modify: `internal/dashboard/contact_transfer.go`
- Modify: `internal/dashboard/contact_transfer_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`

**Interfaces:**

```ts
export type TransferCustomer = { contactId: number; employeeId: number; contactWxId: string; employeeWxId: string; contactName: string; nickName: string; corpName: string; employeeName: string; tags: readonly string[]; transferState: string; addTime: string; addWay: string };
export type TransferRoom = { roomId: number; chatId: string; roomName: string; owner: string; userNum: number; addNum: number; quitNum: number; createTime: string };
export type TransferLog = { mode: 1 | 2 | 3; name: string; employee: string; state: string; corpName: string; roomNum: number; createTime: string };
export type EmployeeOption = { id: number; name: string; wxUserId: string; status: number };
```

- [ ] **Step 1: 写失败的 API 合同测试**

```ts
it('uses the real transfer endpoints and parses business responses', async () => {
  const request = vi.fn()
    .mockResolvedValueOnce({ list: [customer], lastTime: '2026-08-20 10:00:00' })
    .mockResolvedValueOnce([{ errcode: 0 }, { errcode: 40096, errmsg: 'customer refused' }]);
  const api = createContactTransferApi({ request });
  await expect(api.unassigned({ contactName: '', employeeIds: [], addTimeStart: '', addTimeEnd: '' })).resolves.toMatchObject({ lastTime: '2026-08-20 10:00:00' });
  await expect(api.transferCustomers({ type: 1, list: [{ employeeWxId: 'zhang', contactWxId: 'wm1' }], takeoverUserId: 'li' })).resolves.toEqual([{ errcode: 0 }, { errcode: 40096, errmsg: 'customer refused' }]);
});

it('uses POST for unassigned synchronization', async () => {
  const request = vi.fn().mockResolvedValue({});
  await createContactTransferApi({ request }).syncUnassigned();
  expect(request).toHaveBeenCalledWith('/contactTransfer/sync', expect.objectContaining({ method: 'POST' }));
});
```

- [ ] **Step 2: 写失败的后端 POST 路由测试**

```go
func TestContactTransferSyncAcceptsPostAlias(t *testing.T) {
    handler, wecom := newContactTransferTestHandler(t)
    req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/contactTransfer/sync", nil)
    rec := httptest.NewRecorder()
    handler.SaveUnassignedList(rec, req)
    if rec.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()) }
    if wecom.getUnassignedCalls != 1 { t.Fatalf("calls=%d", wecom.getUnassignedCalls) }
}
```

- [ ] **Step 3: 运行并确认模块和 POST 支持缺失**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-operations/contact-transfer-api.test.ts`

Expected: FAIL，模块不存在。

Run: `go test ./internal/dashboard ./internal/server -run 'ContactTransfer.*PostAlias|ContactTransferSync' -count=1`

Expected: FAIL，POST 返回 405 或路由不存在。

- [ ] **Step 4: 实现严格类型化 API**

API 方法固定为：

```ts
assigned(filter): Promise<readonly TransferCustomer[]>;
unassigned(filter): Promise<{ list: readonly TransferCustomer[]; lastTime: string }>;
rooms(filter): Promise<readonly TransferRoom[]>;
logs(filter): Promise<readonly TransferLog[]>;
employees(keyword): Promise<readonly EmployeeOption[]>;
syncUnassigned(): Promise<void>;
transferCustomers(input): Promise<readonly TransferResult[]>;
transferRooms(input): Promise<readonly TransferResult[]>;
```

`employees` 使用 `/workEmployee/index?page=1&perPage=200`，只接受带真实 `wxUserId` 且状态可接替的员工；解析失败抛中文合同错误。

- [ ] **Step 5: 新增 POST 同步路由且保留旧 GET**

`SaveUnassignedList` 方法检查改为：

```go
if r.Method != http.MethodGet && r.Method != http.MethodPost {
    writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
    return
}
```

`server.go` 添加：

```go
case r.URL.Path == "/dashboard/contactTransfer/sync" && r.Method == http.MethodPost && s.contactTransferSync != nil:
    s.contactTransferSync.ServeHTTP(w, r)
```

路由清单同步增加 `POST /dashboard/contactTransfer/sync`。

- [ ] **Step 6: 补客户继承页面资源**

`dashboard.customer.inheritance` 的资源精确增加：

```json
{ "method": "GET", "pathPattern": "/dashboard/contactTransfer/unassignedList", "scopeRequired": false },
{ "method": "GET", "pathPattern": "/dashboard/contactTransfer/info", "scopeRequired": false },
{ "method": "GET", "pathPattern": "/dashboard/contactTransfer/room", "scopeRequired": false },
{ "method": "GET", "pathPattern": "/dashboard/contactTransfer/log", "scopeRequired": false },
{ "method": "GET", "pathPattern": "/dashboard/workEmployee/index", "scopeRequired": false },
{ "method": "POST", "pathPattern": "/dashboard/contactTransfer/sync", "scopeRequired": false },
{ "method": "POST", "pathPattern": "/dashboard/contactTransfer/index", "scopeRequired": false },
{ "method": "POST", "pathPattern": "/dashboard/contactTransfer/room", "scopeRequired": false }
```

保留原有资源，不重复登记。

- [ ] **Step 7: 运行目标测试**

Run: `go test ./internal/dashboard ./internal/server -run 'ContactTransfer|DashboardAccess' -count=1`

Expected: PASS。

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-operations/contact-transfer-api.test.ts`

Expected: PASS。

- [ ] **Step 8: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-operations/contact-transfer-api.ts web/apps/dashboard/src/features/conversation-operations/contact-transfer-api.test.ts internal/dashboard/contact_transfer.go internal/dashboard/contact_transfer_test.go internal/server/server.go internal/server/server_test.go internal/dashboard/dashboard_page_catalog.json internal/dashboard/dashboard_access_guard_test.go
git commit -m "feat: add typed customer inheritance api"
```

---

### Task 8: 实现客户继承读取工作区

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-operations/customer-inheritance-page.tsx`
- Create: `web/apps/dashboard/src/features/conversation-operations/customer-inheritance-page.test.tsx`
- Modify: `web/apps/dashboard/src/styles/conversation-operations.css`

**Interfaces:**

- Consumes: Task 7 `ContactTransferApi`。
- Produces: `inheritanceTab=resigned|active`、`assetTab=customer|room`、筛选和页码 URL 状态。

- [ ] **Step 1: 写失败的读取和页面边界测试**

```tsx
it('loads resigned customers and rooms through their real endpoints', async () => {
  renderPage(api, '/customer/inheritance?inheritanceTab=resigned&assetTab=customer');
  expect(await screen.findByText('重点客户')).toBeTruthy();
  expect(api.unassigned).toHaveBeenCalled();
  fireEvent.click(screen.getByRole('tab', { name: '待分配群聊' }));
  expect(await screen.findByText('产品交流群')).toBeTruthy();
  expect(api.rooms).toHaveBeenCalled();
});

it('shows active customer transfer without deferred product entries', async () => {
  renderPage(api, '/customer/inheritance');
  fireEvent.click(screen.getByRole('tab', { name: '在职继承' }));
  expect(screen.getByText('请选择需要被继承的员工')).toBeTruthy();
  expect(screen.queryByRole('tab', { name: '侧边栏配置' })).toBeNull();
  expect(screen.queryByRole('tab', { name: /在职群聊/ })).toBeNull();
  expect(screen.queryByText(/能力未接入|功能未接入/)).toBeNull();
});
```

- [ ] **Step 2: 运行并确认页面模块不存在**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-operations/customer-inheritance-page.test.tsx`

Expected: FAIL，模块不存在。

- [ ] **Step 3: 实现两个主标签和离职二级标签**

固定主标签：

```ts
const inheritanceTabs = [
  { value: 'resigned', label: '离职继承' },
  { value: 'active', label: '在职继承' },
] as const;
```

离职客户调用 `unassigned`，群聊调用 `rooms`。二级标签切换清空已选记录并把页码重置为 1。旧接口返回全量真实列表，页面按固定 `20` 条进行确定性客户端分页：先过滤，再 `slice((page-1)*20,page*20)`，总数为过滤后数组长度。

- [ ] **Step 4: 实现在职继承读取路径**

员工下拉调用 `employees`；选择原员工后调用：

```ts
api.assigned({ contactName, employeeIds: [sourceEmployee.id], addTimeStart, addTimeEnd })
```

只显示客户资产及其筛选、选择和转接操作；不渲染群聊标签或产品建设说明。没有选择原员工时不请求 `assigned`。

- [ ] **Step 5: 清理非法或历史 URL 状态**

若 URL 中出现历史值 `inheritanceTab=sidebar` 或其他非法枚举，初始化时替换为 `inheritanceTab=resigned`；不得据此渲染占位模块。切换一级标签时清理不兼容的 `assetTab`、页码和选中记录。

- [ ] **Step 6: 运行读取测试**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-operations/customer-inheritance-page.test.tsx`

Expected: 读取测试 PASS；mutation 测试尚未加入。

- [ ] **Step 7: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-operations/customer-inheritance-page.tsx web/apps/dashboard/src/features/conversation-operations/customer-inheritance-page.test.tsx web/apps/dashboard/src/styles/conversation-operations.css
git commit -m "feat: add customer inheritance workspace"
```

---

### Task 9: 完成客户继承同步、批量分配和记录抽屉

**Files:**

- Modify: `web/apps/dashboard/src/features/conversation-operations/customer-inheritance-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-operations/customer-inheritance-page.test.tsx`
- Modify: `web/apps/dashboard/src/styles/conversation-operations.css`

**Interfaces:**

- Consumes: `syncUnassigned`、`transferCustomers`、`transferRooms`、`logs`。
- Produces: 可取消的同步确认、分配确认和继承记录抽屉。

- [ ] **Step 1: 写失败的选择和保护测试**

```tsx
it('disables assignment until records and a different successor are selected', async () => {
  renderPage(api, '/customer/inheritance?inheritanceTab=active');
  selectEvent.selectOptions(await screen.findByLabelText('原跟进员工'), '9');
  fireEvent.click(await screen.findByLabelText('选择客户 31'));
  selectEvent.selectOptions(screen.getByLabelText('接替员工'), '9');
  expect(screen.getByRole('button', { name: '分配给其他员工' })).toBeDisabled();
  expect(screen.getByText('接替员工不能与原跟进员工相同')).toBeTruthy();
});

it('summarizes partial provider failures and retains failed rows', async () => {
  api.transferCustomers.mockResolvedValue([{ errcode: 0 }, { errcode: 40096, errmsg: '客户拒绝' }]);
  renderPage(api, '/customer/inheritance');
  selectTwoCustomersAndSuccessor();
  fireEvent.click(screen.getByRole('button', { name: '确认分配' }));
  expect(await screen.findByRole('status')).toHaveTextContent('成功 1 条，失败 1 条');
  expect(screen.getByLabelText('选择客户 32')).toBeChecked();
});

it('opens inheritance records and keeps main filters after closing', async () => {
  renderPage(api, '/customer/inheritance?contactName=%E9%87%8D%E7%82%B9');
  fireEvent.click(await screen.findByRole('button', { name: '继承记录' }));
  expect(api.logs).toHaveBeenCalledWith(expect.objectContaining({ mode: 1 }));
  fireEvent.click(screen.getByRole('button', { name: '关闭继承记录' }));
  expect(screen.getByLabelText('客户名称')).toHaveValue('重点');
});
```

- [ ] **Step 2: 运行并确认 mutation 行为失败**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-operations/customer-inheritance-page.test.tsx`

Expected: FAIL，确认对话框、部分失败和记录抽屉尚未实现。

- [ ] **Step 3: 实现同步确认和刷新**

只有离职继承客户标签显示“同步离职数据”。确认文案固定为：

```tsx
<DashboardDialog open={syncOpen} title="同步离职待分配数据？" confirmText="开始同步" onCancel={() => setSyncOpen(false)} onConfirm={() => sync.mutate()}>
  <p>将从当前企业的企业微信客户联系接口重新拉取待分配客户。现有待分配快照会被真实同步结果替换。</p>
</DashboardDialog>
```

成功后刷新 `unassigned` 和 `rooms`，失败保留对话框并显示错误。

- [ ] **Step 4: 实现批量分配确认**

客户 payload：

```ts
selectedCustomers.map((row) => ({ employeeWxId: row.employeeWxId, contactWxId: row.contactWxId }))
```

群聊 payload：

```ts
selectedRooms.map((row) => row.chatId)
```

确认文案必须包含记录数、原员工（单一时）和接替员工。响应按 `errcode === 0` 统计；成功项取消选择，失败项保留并列出 `errmsg`。在职继承固定 `type=2`，离职继承固定 `type=1`。

- [ ] **Step 5: 实现继承记录抽屉**

记录模式固定映射：

```ts
const recordModes = [
  { value: 1, label: '离职客户' },
  { value: 2, label: '离职群聊' },
  { value: 3, label: '在职客户' },
] as const;
```

抽屉支持关闭按钮、backdrop、`Escape`；筛选名称、接替员工和日期；固定客户端分页 20 条。关闭抽屉不修改主页面 URL 筛选。

- [ ] **Step 6: 运行页面测试**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-operations/customer-inheritance-page.test.tsx src/features/conversation-operations/contact-transfer-api.test.ts`

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-operations/customer-inheritance-page.tsx web/apps/dashboard/src/features/conversation-operations/customer-inheritance-page.test.tsx web/apps/dashboard/src/styles/conversation-operations.css
git commit -m "feat: complete customer inheritance actions"
```

---

### Task 10: 完成路由注入、删除旧样式冲突并做集成回归

**Files:**

- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**

- Produces: 四个目标路由都指向新领域页面。
- Preserves: 其他 49 个 benchmark 路由注册。

- [ ] **Step 1: 写失败的 registry 测试**

```tsx
it('registers all four conversation operations pages with their typed APIs', () => {
  const pages = createBenchmarkP0Pages(dependencies);
  expect(pages['/chat/file-audio'].type).toBe(FileAudioPage);
  expect(pages['/chat/resign-staff'].type).toBe(ResignedEmployeePage);
  expect(pages['/chat/refuse-archive'].type).toBe(RefuseArchivePage);
  expect(pages['/customer/inheritance'].type).toBe(CustomerInheritancePage);
});
```

- [ ] **Step 2: 运行并确认客户继承仍指向旧组件**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/benchmark/page-registry.test.tsx`

Expected: FAIL，`/customer/inheritance` 或其他目标路由仍为旧类型。

- [ ] **Step 3: 完成依赖注入**

`createBenchmarkP0Pages` 增加：

```ts
refuseArchiveApi?: RefuseArchiveApi;
contactTransferApi?: ContactTransferApi;
```

目标注册固定为：

```tsx
'/chat/file-audio': <FileAudioPage api={fileAudioApi} />,
'/chat/resign-staff': <ResignedEmployeePage api={conversationGlobalApi} />,
'/chat/refuse-archive': <RefuseArchivePage api={refuseArchiveApi} />,
'/customer/inheritance': <CustomerInheritancePage api={contactTransferApi} />,
```

`main.tsx` 使用现有 `apiClient` 创建两个新 API 后注入。

- [ ] **Step 4: 删除仅服务旧四页的冲突规则**

从 `index.css` 删除 `.phase33-operations-*` 和拒绝存档旧页专用 `.phase33-risk-warning-*` 中已无消费者的规则；先用 `rg` 确认其他页面仍使用 `.phase33-risk-warning-page` 时只删除旧四页能明确替代的选择器，不做全局格式化。

Run: `rg -n 'phase33-operations|phase33-risk-warning' web/apps/dashboard/src --glob '!styles/index.css'`

Expected: 每个准备删除的选择器均无剩余消费者；有消费者的规则保留。

- [ ] **Step 5: 运行四页与 registry 测试**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/phase35/file-audio-page.test.tsx src/features/conversation-operations src/benchmark/page-registry.test.tsx src/styles/conversation-operations-layout.test.ts`

Expected: PASS。

- [ ] **Step 6: 运行 dashboard 全量测试、类型检查和构建**

Run: `corepack pnpm --filter @mochat/dashboard test`

Expected: PASS，记录实际文件数和测试数。

Run: `corepack pnpm --filter @mochat/dashboard typecheck`

Expected: PASS，exit code 0。

Run: `corepack pnpm --filter @mochat/dashboard build`

Expected: PASS，生成 dashboard production assets。

- [ ] **Step 7: 运行相关 Go 回归**

Run: `go test ./internal/dashboard ./internal/store ./internal/server ./internal/modules/chat-media/... -count=1`

Expected: PASS。

- [ ] **Step 8: 提交**

```powershell
git add web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/benchmark/page-registry.test.tsx web/apps/dashboard/src/main.tsx web/apps/dashboard/src/styles/index.css
git commit -m "feat: register conversation operations pages"
```

---

### Task 11: Docker 部署与真实浏览器点击验收

**Files:**

- No code changes expected.
- Create only if required by project convention: `docs/acceptance/2026-08-20-conversation-operations-pages.md`，内容必须使用中文并记录实际结果；不得预填“通过”。

**Interfaces:**

- Consumes: Task 10 已通过的生产构建。
- Produces: 容器健康、分辨率、点击路径、网络和控制台证据。

- [ ] **Step 1: 记录部署前容器和卷状态**

Run: `docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml ps`

Expected: 能解析配置并列出 `app`、MySQL、Redis；如某服务非 healthy，先诊断，不删除卷。

Run: `docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop`

Expected: 记录现有命名卷列表。

- [ ] **Step 2: 只重建 app**

Run: `docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app`

Expected: app 镜像构建成功且容器重新启动；MySQL、Redis 容器 ID 不变。

- [ ] **Step 3: 等待健康并确认真实入口**

Run: `docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml ps`

Expected: app、MySQL、Redis 达到项目预期健康状态，`http://127.0.0.1:18080` 可访问。

- [ ] **Step 4: 在 `2560×1440` 验收文件录音**

使用 in-app Browser viewport 能力设置 `2560×1440`，打开 `/chat/file-audio`，逐项验证：

1. 页面填满内容区且没有大块上传卡；
2. 页面没有聊天文件标签、产品建设提示或占位模块；
3. 文件名和日期查询、重置、刷新改变请求或恢复 URL；
4. 上传对话框可通过取消、遮罩和 `Escape` 关闭；
5. 选择非音频文件显示错误；
6. 已有音频可播放或显示可理解的单行播放错误；
7. 删除确认可取消，不执行真实删除。

- [ ] **Step 5: 在 `2560×1440` 验收离职员工**

打开 `/chat/resign-staff`，验证：

1. 三栏比例接近 `300px / 0.9fr / 1.35fr`，各栏独立滚动；
2. 只列离职员工；
3. 选择员工加载该员工会话，整行点击打开详情；
4. 切换会话类型和日期会重置不兼容详情；
5. URL 保存员工、会话和筛选；刷新后恢复；
6. 员工无会话与无离职员工是不同空态；
7. 内部群入口和产品建设提示不出现在会话类型区。

- [ ] **Step 6: 在 `2560×1440` 验收拒绝存档**

打开 `/chat/refuse-archive`，验证：

1. 客户/群聊标签真实改变 `subjectType` 请求；
2. 主体、员工、日期、授权和跟进状态筛选生效；
3. 状态全部为中文；
4. 分页总数、页码和 URL 一致；
5. 跟进抽屉可通过关闭按钮、遮罩和 `Escape` 关闭；
6. 不点击“保存跟进”提交真实变更。

- [ ] **Step 7: 在 `2560×1440` 验收客户继承**

打开 `/customer/inheritance`，验证：

1. 只呈现离职继承/在职继承两个一级标签；
2. 离职客户/群聊二级标签使用不同真实接口；
3. 勾选、全选、翻页和切换标签会正确保留或清理选择；
4. 未选记录时分配按钮禁用；
5. 接替员工与原员工相同时阻止继续；
6. 同步和分配只点击到确认对话框，然后取消；
7. 继承记录抽屉的三个标签、筛选、分页和三种关闭方式正常；
8. 侧边栏配置、在职群聊和产品建设提示均不出现在页面中。

- [ ] **Step 8: 回归 `1440×900` 和控制台/网络**

将 viewport 改为 `1440×900`，对四页复查：

- 筛选和按钮不重叠；
- 关键文本不截断；
- 表格只在表格容器内横向滚动；
- 页面无异常整页横向滚动；
- console 没有未解释 error；
- 关键请求没有 4xx/5xx；
- 菜单和顶部 banner 未改变。

- [ ] **Step 9: 恢复 viewport 并记录最终健康/卷状态**

清除临时 viewport override。

Run: `docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml ps`

Run: `docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop`

Expected: 健康状态正常，命名卷集合与部署前一致。

- [ ] **Step 10: 写实际验收报告**

报告必须记录：

- 四个目标路由；
- 四页实际数据来源，以及总进度文档中登记的后续专项没有被渲染为页面入口；
- 每条测试/构建命令的真实结果；
- `2560×1440`、`1440×900` 的点击结果；
- 控制台和网络异常；
- app/MySQL/Redis 健康状态；
- 数据卷前后对比；
- 未执行真实同步、跟进、转接和删除的安全说明。

报告中不得把未运行项目写成通过。

---

## Implementation Order and Checkpoints

1. Tasks 1–3：完成文件录音，共享样式已稳定；
2. Task 4：完成离职员工，独立运行会话域回归；
3. Tasks 5–6：完成拒绝存档，独立运行前后端回归；
4. Tasks 7–9：完成客户继承读写闭环；
5. Task 10：统一注册和全量工程验证；
6. Task 11：Docker 和真实浏览器点击验收。

每个 checkpoint 都必须保证目标测试为绿；如果前一任务测试未通过，不进入下一任务。

## Review Gate

本文档仅用于用户审阅。用户批准设计和实施文档后，才选择 `subagent-driven-development` 或 `executing-plans` 开始实施；批准前不修改业务代码、不创建提交。

---

## 文件录音菜单修复追加实施计划（2026-08-21）

**目标：** 将 `/chat/file-audio` 从人工上传列表修复为对齐圆弧 AI 的只读企微同步录音列表，并让开发环境 3 条模拟录音可真实播放且展示正确时长。

**范围：** 只修改文件录音页面、音频 HTTP 模块、Dashboard 资源目录、开发模拟数据和相关文档；不修改其他三个会话运营页面。

### 追加任务 A：先补失败测试

- 前端页面测试断言：不渲染上传/删除/人工上传，不再调用 `upload/remove`，查询发送人、接收人和日期参数，展示来源为“企微同步”。
- 前端 API 测试断言：只暴露 `list`，请求参数为 `sender`、`receiver`、`from`、`to`。
- Go handler 测试断言：POST/DELETE 返回 405，GET 只传递同步录音过滤条件；缺失时长的 WAV 对象在响应中补算时长。
- SQL store 测试断言：按 `sender_name`、`receiver_name`、`synced_at` 查询并排除非企微来源对象。

### 追加任务 B：实现只读录音接口

- `internal/modules/chat-media/transport/http/store.go` 增加来源、发送人、接收人、同步时间字段和只读查询条件；兼容 MySQL 5.7。
- `internal/modules/chat-media/transport/http/handler.go` 仅注册 GET 列表和 GET 内容；POST/DELETE 直接返回 405；内容读取保持企业权限校验。
- 新增媒体时长探测 helper，至少支持 WAV PCM 和常见 MP3 帧，缺失时长时从已落盘字节补算。
- 新增迁移 `0145_chat_media_sync_metadata`，为已有音频对象补充 `source`、`sender_name`、`receiver_name`、`message_id`、`synced_at` 字段，并为同步查询建立联合索引。

### 追加任务 C：替换页面查询交互

- `web/apps/dashboard/src/features/phase35/file-audio-api.ts` 删除上传/删除方法，改为发送人、接收人、日期范围查询。
- `web/apps/dashboard/src/features/phase35/file-audio-page.tsx` 删除上传/删除 UI，列表改为音视频通话、发送人、接收人、发送时间、时长和播放；保留查询、重置、刷新、分页和鉴权播放。
- 更新页面、API、样式和路由资源测试，确保不出现“能力未接入/功能未接入”或手工上传文案。

### 追加任务 D：建立开发模拟企微录音数据

- 新增开发脚本生成 3 个不同秒数的有效 WAV 文件，并以 `source=wecom_sync`、发送人/接收人和同步时间写入当前企业的音频对象表。
- 只在本地 Docker 开发环境执行脚本；不把模拟数据初始化逻辑放入生产启动路径，不删除 MySQL/Redis 数据卷。

### 追加任务 E：回归与浏览器验收

- 运行文件录音前端目标测试、chat-media Go 目标测试、迁移合同测试和 Dashboard production build。
- 仅重建 `mochat-go-desktop` 的 app 服务，确认 app/MySQL/Redis healthy。
- 使用 2560×1440 与 1440×900 检查查询、重置、刷新、分页、三条录音播放和时长；控制台无 error/warn，页面无上传入口和能力建设提示。

## 离职员工部门选择器追加实施计划（2026-08-21）

**目标：** 统一员工会话与离职员工的部门筛选控件，移除部门人数误导，并生成可用于验收的离职员工开发数据。

**架构：** 新建轻量 `EmployeeConversationDepartmentPicker` 组件，由共享员工目录传入部门树和当前选择；目录仅负责查询状态和选择回调。开发夹具脚本直接操作本地 MariaDB 的企业员工、部门关系和消息分表，使用固定前缀幂等写入。

### Task A：先写失败测试并记录红灯

- 修改 `web/apps/dashboard/src/features/conversation-global/employee-conversation-directory.test.tsx`，断言部门触发器不含数字、点击后出现 `listbox`，选择部门回调正确。
- 修改 `web/apps/dashboard/src/styles/employee-conversation-layout.test.ts`，将部门固定 190px 的断言改为紧凑单行控件和弹层样式断言。
- 扩展 `web/apps/dashboard/src/features/conversation-operations/resigned-employee-page.test.tsx`，使用部门和离职员工夹具，断言部门筛选、员工选择和会话查询能够连通。
- 运行上述测试，确认因新组件和新布局尚未实现而失败。

### Task B：实现共享紧凑部门选择器

- 新建 `web/apps/dashboard/src/features/conversation-global/employee-conversation-department-picker.tsx`，实现触发器、listbox、option、层级缩进、Escape 关闭和选择后关闭。
- 修改 `employee-conversation-directory.tsx`，移除部门树内 `employeeCount` 文本，改用新组件；保留全部部门、当前选择和数据加载状态。
- 修改 `web/apps/dashboard/src/styles/index.css`，将部门区域从固定 190px 改为单行触发器，弹层使用目录内绝对定位和最大高度滚动，不改变三栏工作区断点。

### Task C：生成本地离职员工开发夹具

- 新建 `scripts/seed_dev_resigned_employees/main.go`，按固定前缀创建/更新 3 名 `status=5` 员工、部门关系和 3 条归档消息；支持 `-dsn`、`-corp-id` 和 `-cleanup` 参数。
- 执行夹具并通过 `/workMessage/staffDirectory?mode=departed` 验证 3 名员工、部门筛选和会话数据。

### Task D：回归、部署与浏览器验收

- 运行目录、离职员工、员工会话样式专项测试和 Dashboard TypeScript 检查。
- 仅重建 `mochat-go-desktop-app-1`，确认 MySQL、Redis、app 健康且不删除数据卷。
- 在 `/chat/resign-staff` 和 `/chat/v2-staff` 检查部门控件展开/收起、部门选择、员工选择、会话加载、筛选 URL、Escape 关闭和无横向溢出。
- 将测试、夹具和浏览器证据补充到验收记录与总进度台账。

## 客户继承实测问题修复追加实施计划（2026-08-21）

1. 先补前端 API、页面和 Store SQL 失败测试，覆盖群聊逐项结果、群聊页同步按钮、备注/名称/外部 ID 查询和单侧日期过滤。
2. 在 API 层将企业微信失败群集合按提交顺序归一化；在 Store 层统一客户展示字段查询，并将日期边界改为独立生效且包含结束日。
3. 将同步离职数据按钮从客户二级标签提升到离职继承一级标签，保留原确认对话框和真实同步接口。
4. 运行专项前端、Store、Dashboard 测试、TypeScript 与生产构建；仅重建 app 服务，再用浏览器复核筛选、群聊切换、同步确认取消、转接确认取消和继承记录抽屉。

## 客户继承确认分配错误追加实施记录（2026-08-21）

- 根因：开发样例企业 `wwSIM...` 凭据未接入真实企业微信，确认分配时触发真实接口 `40013 invalid corpid`。
- 实施：为明确的开发模拟凭据增加客户/群聊本地成功响应；真实企业凭据保持原 HTTP Provider 路径。
- 验证：新增 Go Provider 测试，确认模拟转接不发外部请求且返回成功结果；随后重建 app 并用浏览器点击确认分配。

### 客户继承确认分配错误追加实施结果（2026-08-21）

- 凭据链路补齐：开发容器缺少 `preview-wecom-v1` 密钥时，Store 仅对明确的 `wwSIM...`/`SIM-...` 样例读取本地开发凭据；生产凭据仍返回解密错误，不走明文回退。
- 权限链路补齐：将 `POST /dashboard/contactTransfer/sync` 纳入精确免映射白名单，并增加 RBAC 回归测试，避免同步请求在业务 Handler 前被拒绝。
- 最终浏览器验收：同步成功提示出现；确认分配客户 2013 至张伟后返回成功 1 条、失败 0 条，行移除，数据流闭环。
