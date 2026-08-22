# 活码选择交互优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将渠道/群活码人员选择改为普通单击独立切换，并用真实群聊选择弹窗替代群聊 JSON 输入，同时优化抽屉底部按钮。

**Architecture:** 继续使用现有 `LiveCodeCreateDialog` 和写接口，在页面文件内增加真实群聊查询、纯切换函数及两个选择组件。群聊弹窗维护独立草稿，确认后生成有序 `selectedRoomIds`；提交时自动映射为既有 `rooms` JSON 字符串，不修改后端契约。

**Tech Stack:** React 18、TypeScript、TanStack Query、Vitest、Testing Library、CSS、Docker Compose。

## Global Constraints

- 所有展示人员、群聊、人数和容量必须来自当前企业权限范围内的真实接口。
- 群聊最多选择 5 个，按选择顺序提交；普通单击切换，只有再次点击同一项才取消。
- 用户界面不得展示或要求编辑群聊 JSON。
- 不修改全局菜单、顶部 banner、数据库迁移或现有 Provider 写接口。
- Docker 交付不得删除 MySQL、Redis 或命名卷。

---

### Task 1: 用失败测试固定人员与群聊选择契约

**Files:**
- Modify: `web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx`
- Test: `web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx`

**Interfaces:**
- Consumes: `ChannelCodePage`、`GroupCodePage`、`BusinessWorkbenchApi`
- Produces: 人员 toggle、群聊弹窗、无 JSON、自动 payload 映射的行为契约

- [ ] **Step 1: 新增人员逐项切换失败测试**

```tsx
fireEvent.click(await screen.findByRole('button', { name: '选择成员 李娜' }));
fireEvent.click(screen.getByRole('button', { name: '选择成员 王强' }));
expect(screen.getByText('已选择 2 人')).toBeTruthy();
fireEvent.click(screen.getByRole('button', { name: '取消选择成员 李娜' }));
expect(screen.getByText('已选择 1 人')).toBeTruthy();
```

- [ ] **Step 2: 新增群聊弹窗和提交映射失败测试**

```tsx
fireEvent.click(screen.getByRole('button', { name: '选择群聊' }));
fireEvent.click(await screen.findByRole('button', { name: /选择群聊 客户群一/ }));
fireEvent.click(screen.getByRole('button', { name: /选择群聊 客户群二/ }));
fireEvent.click(screen.getByRole('button', { name: '确认选择' }));
expect(screen.queryByLabelText('群聊配置 JSON')).toBeNull();
expect(screen.getByText('已选择 2 个群聊')).toBeTruthy();
```

- [ ] **Step 3: 运行测试并确认因缺少新交互而失败**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/acquisition-pages.test.tsx`

Expected: FAIL，找不到“选择成员 李娜”“选择群聊”或仍存在“群聊配置 JSON”。

- [ ] **Step 4: 提交测试**

```powershell
git add web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx
git commit -m "test: define live code choice interactions"
```

### Task 2: 实现人员单击切换与真实群聊查询

**Files:**
- Modify: `web/apps/dashboard/src/features/phase34/live-code/live-code-pages.tsx`
- Test: `web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx`

**Interfaces:**
- Consumes: `api.read('/workEmployee/index', ...)`、`api.read('/workRoom/roomIndex', {})`
- Produces: `toggleNumericChoice(values: number[], id: number): number[]`、`LiveCodeRoomOption`

- [ ] **Step 1: 增加纯切换函数和群聊类型**

```tsx
type LiveCodeRoomOption = {
  id: number;
  name: string;
  currentNum: number;
  roomMax: number;
};

function toggleNumericChoice(values: number[], id: number): number[] {
  return values.includes(id) ? values.filter((value) => value !== id) : [...values, id];
}
```

- [ ] **Step 2: 从真实接口读取群聊选项**

```tsx
async function fetchLiveCodeRooms(api: BusinessWorkbenchApi): Promise<LiveCodeRoomOption[]> {
  const rows = recordsFrom(await api.read('/workRoom/roomIndex', {}));
  return rows.map((row) => ({
    id: numberOf(value(row, 'roomId', 'workRoomId', 'id')),
    name: textOf(value(row, 'roomName', 'name'), '未命名群聊'),
    currentNum: numberOf(value(row, 'currentNum', 'memberNum')),
    roomMax: numberOf(value(row, 'roomMax')),
  })).filter((room) => room.id > 0);
}
```

- [ ] **Step 3: 将员工状态从逗号字符串改为 `number[]` 并渲染整行 toggle 按钮**

```tsx
const [employeeIDs, setEmployeeIDs] = useState<number[]>([]);
<button
  type="button"
  aria-label={`${selected ? '取消选择' : '选择'}成员 ${employee.name}`}
  aria-pressed={selected}
  onClick={() => setEmployeeIDs((current) => toggleNumericChoice(current, employee.id))}
>
  <span>{employee.name}</span><span aria-hidden="true">{selected ? '✓' : '+'}</span>
</button>
```

- [ ] **Step 4: 运行目标测试，确认人员测试通过**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/acquisition-pages.test.tsx`

Expected: 人员逐项切换测试 PASS；群聊弹窗测试仍 FAIL。

### Task 3: 实现双栏群聊选择弹窗并隐藏 JSON

**Files:**
- Modify: `web/apps/dashboard/src/features/phase34/live-code/live-code-pages.tsx`
- Test: `web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx`

**Interfaces:**
- Consumes: `LiveCodeRoomOption[]`、`toggleNumericChoice`
- Produces: 有序 `selectedRoomIDs: number[]` 和 `rooms` 提交字符串

- [ ] **Step 1: 增加已选状态、弹窗草稿和最多 5 个限制**

```tsx
const [selectedRoomIDs, setSelectedRoomIDs] = useState<number[]>([]);
const [roomDraftIDs, setRoomDraftIDs] = useState<number[]>([]);
const toggleRoomDraft = (id: number) => setRoomDraftIDs((current) => {
  if (current.includes(id)) return current.filter((value) => value !== id);
  return current.length >= 5 ? current : [...current, id];
});
```

- [ ] **Step 2: 实现“选择群聊”双栏对话框**

```tsx
<section className="phase34-live-code-picker" role="dialog" aria-label="选择群聊" aria-modal="true">
  <header><h3>选择群聊</h3><button type="button" aria-label="关闭群聊选择">×</button></header>
  <div className="phase34-live-code-picker-grid">
    <div aria-label="可选群聊">{/* 搜索和可选项 */}</div>
    <div aria-label="已选群聊">{/* 已选数量、清空和有序列表 */}</div>
  </div>
  <footer>
    <button type="button">取消</button>
    <button type="button">确认选择</button>
  </footer>
</section>
```

- [ ] **Step 3: 删除 JSON 输入与校验，提交时内部映射**

```tsx
const selectedRooms = selectedRoomIDs
  .map((id) => roomOptions.find((room) => room.id === id))
  .filter((room): room is LiveCodeRoomOption => Boolean(room) && room.roomMax > 0);

onSave({
  corpId: Number(access.corp.id),
  qrcodeName: name.trim(),
  isVerified: 2,
  leadingWords: leadingWords.trim(),
  employees: employeeIDs,
  tags: tagIDs,
  rooms: JSON.stringify(selectedRooms.map((room) => ({ roomId: room.id, maxNum: room.roomMax }))),
});
```

- [ ] **Step 4: 运行目标测试并确认全部通过**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/acquisition-pages.test.tsx`

Expected: PASS。

### Task 4: 优化选择器与底部操作按钮样式

**Files:**
- Modify: `web/apps/dashboard/src/styles/index.css`
- Test: `web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx`

**Interfaces:**
- Consumes: `.phase34-live-code-choice-list`、`.phase34-live-code-picker`、`.phase34-live-code-drawer-footer`
- Produces: 可读的选中态、双栏选择弹窗和固定圆角按钮

- [ ] **Step 1: 添加成员项和群聊弹窗样式**

```css
.phase34-live-code-choice-list button[aria-pressed="true"] { background: #eef2ff; border-color: #8495f5; color: #4258cf; }
.phase34-live-code-picker { background: #fff; border-radius: 14px; box-shadow: 0 24px 70px rgb(22 30 48 / 25%); width: min(720px, calc(100vw - 40px)); }
.phase34-live-code-picker-grid { display: grid; grid-template-columns: 1fr 1fr; min-height: 360px; }
```

- [ ] **Step 2: 添加底部主次按钮样式和焦点态**

```css
.phase34-live-code-drawer-footer button { border-radius: 999px; min-height: 40px; min-width: 108px; }
.phase34-live-code-drawer-footer .phase34-secondary-button { background: #f4f6fa; border-color: #e6eaf1; color: #596275; }
.phase34-live-code-drawer-footer button:focus-visible { box-shadow: 0 0 0 3px rgb(83 107 244 / 20%); outline: none; }
```

- [ ] **Step 3: 执行 ESLint 和目标测试**

Run: `corepack pnpm --filter @mochat/dashboard exec eslint src/features/phase34/live-code/live-code-pages.tsx src/features/phase34/acquisition-pages.test.tsx`

Expected: exit 0。

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/acquisition-pages.test.tsx`

Expected: PASS。

### Task 5: 完整验证、Docker 交付与浏览器验收

**Files:**
- Verify: `web/apps/dashboard/src/features/phase34/live-code/live-code-pages.tsx`
- Verify: `web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx`
- Verify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Consumes: 完整实现
- Produces: 可复核的自动化、构建、容器和点击验收证据

- [ ] **Step 1: 运行前端完整验证**

```powershell
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
git diff --check -- web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx web/apps/dashboard/src/features/phase34/live-code/live-code-pages.tsx web/apps/dashboard/src/styles/index.css
```

Expected: 全部 exit 0。

- [ ] **Step 2: 仅重建必要的 Docker 应用服务并确认健康**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\deploy_docker_desktop.ps1 -ProjectName mochat-go-desktop -SkipHttpCheck`

Expected: app、MySQL、Redis 就绪，数据卷未删除。

- [ ] **Step 3: 浏览器验收渠道活码**

打开 `http://127.0.0.1:18080/acquisition/v2-channel-code`，创建抽屉中连续选择两名员工，再次点击第一名；确认计数从 0→1→2→1，保存按钮状态正确，取消按钮关闭抽屉。

- [ ] **Step 4: 浏览器验收群活码**

打开 `http://127.0.0.1:18080/acquisition/group-code`，确认页面无 JSON 字段；打开群聊选择弹窗，验证搜索、两项累加、再次点击取消、取消不写回、确认写回、清空和最多 5 个限制。

- [ ] **Step 5: 检查布局和错误日志**

在常规桌面和 2560×1440 检查抽屉、弹窗和底部按钮；确认无异常横向滚动、遮罩层级错误或控制台错误。
