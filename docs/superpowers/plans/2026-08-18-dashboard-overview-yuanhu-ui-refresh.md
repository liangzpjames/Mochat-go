# Dashboard 数据概览圆弧风格优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变后端、权限和真实数据口径的前提下，把 Dashboard `/index` 重排为对标圆弧 AI 会话层级的可售卖级经营驾驶舱，并在保留四个数据卷的前提下释放 Docker Desktop 可回收空间。

**Architecture:** `dashboard-overview-page.tsx` 继续独占查询、URL、日期校验和导出副作用；新增 `dashboard-overview-widgets.tsx` 承载无请求副作用的经营快照、AI、趋势和会话展示组件；样式仍集中在 Dashboard `index.css`，但全部限定在 `.dashboard-overview-page` 下。接口继续使用 `DashboardOverviewApi`，本轮不修改 Go、数据库或响应合同。

**Tech Stack:** React 19、TypeScript 5.9、TanStack Query 5、Ant Design 6、Vitest、Testing Library、Playwright、Docker Desktop / Compose v2。

## Global Constraints

- 设计依据：`docs/superpowers/specs/2026-08-18-dashboard-overview-yuanhu-ui-refresh-design.md`。
- 用户可见文案、验收记录和审阅文档使用中文。
- 只修改数据概览相关 React、测试和页面样式；不修改 reporting 后端、数据库迁移、权限资源或其他 Dashboard 页面。
- 不新增 API 字段，不伪造质检、排行、轨迹、同比或环比数据。
- 保留 `GET /dashboard/reports/overview`、`/dashboard/corpData/index#get`、URL 日期/分页、31 天限制、刷新、CSV 导出和企业时区合同。
- AI 或会话归档未接入时必须显示受限态，不得以业务零值替代 Provider 缺失。
- 不新增 npm 依赖；装饰图形使用 CSS 和现有 Ant Design 基础组件。
- 每个行为改动遵循 TDD：先写失败测试，确认因目标行为缺失而失败，再写最小实现并运行通过。
- 不触碰工作树中既有的 `deploy/standalone/migrations/0127_dashboard_page_rbac.up.sql`、`docs/PROJECT_PROGRESS.zh-CN.md`、`.workbuddy/`、调试脚本和 `web/saas-admin/` 改动。
- Docker Desktop 数据继续位于 `D:\Docker\wsl-data\DockerDesktopWSL`；构建上下文固定为 `D:\workspace\mochat-go\mochat-go`。
- 必须保留 `mochat-go-desktop_mysql-data`、`mochat-go-desktop_redis-data`、`mochat-go-desktop_app-storage`、`mochat-go-desktop_audit-anchor-storage`。
- 禁止 `docker volume prune`、`docker system prune --volumes`、`docker compose down -v`、`docker volume rm` 和 `scripts/deploy_docker_desktop.ps1 -ResetData`。
- 最终只构建并重建 `app`；不重建 MySQL/Redis，不删除数据卷。

---

### Task 1: 安全释放 Docker Desktop 可回收空间

**Files:**
- Create at execution time: `D:\workspace\mochat-go\output\overview-ui-refresh-20260818\docker-before.txt`
- Create at execution time: `D:\workspace\mochat-go\output\overview-ui-refresh-20260818\docker-after.txt`
- Create at execution time: `D:\workspace\mochat-go\output\overview-ui-refresh-20260818\volumes-before.json`
- Create at execution time: `D:\workspace\mochat-go\output\overview-ui-refresh-20260818\volumes-after.json`

**Interfaces:**
- Consumes: Docker Desktop WSL engine、Compose project `mochat-go-desktop`。
- Produces: 清理前后可审计的容量与卷清单；不产生仓库代码改动。

- [ ] **Step 1: 启动 Docker Desktop 并确认 D 盘配置**

从 Docker Desktop 正常入口启动引擎，等待 `docker info` 成功。随后运行：

```powershell
$settingsPath = 'C:\Users\lzpen\AppData\Roaming\Docker\settings-store.json'
$settings = Get-Content -Raw -LiteralPath $settingsPath | ConvertFrom-Json
if ($settings.CustomWslDistroDir -ne 'D:\Docker\wsl-data\DockerDesktopWSL') {
    throw "Docker Desktop 数据目录不是预期 D 盘路径：$($settings.CustomWslDistroDir)"
}
docker info
```

Expected: `docker info` exit 0，配置路径严格等于 `D:\Docker\wsl-data\DockerDesktopWSL`。

- [ ] **Step 2: 记录清理前状态并验证四个卷**

```powershell
$evidenceRoot = 'D:\workspace\mochat-go\output\overview-ui-refresh-20260818'
New-Item -ItemType Directory -Force -Path $evidenceRoot | Out-Null
$volumeNames = @(
    'mochat-go-desktop_mysql-data',
    'mochat-go-desktop_redis-data',
    'mochat-go-desktop_app-storage',
    'mochat-go-desktop_audit-anchor-storage'
)
docker system df -v | Tee-Object -FilePath "$evidenceRoot\docker-before.txt"
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps -a |
    Add-Content -LiteralPath "$evidenceRoot\docker-before.txt"
docker volume inspect $volumeNames |
    Set-Content -LiteralPath "$evidenceRoot\volumes-before.json" -Encoding utf8
```

Expected: `docker volume inspect` exit 0 且 JSON 中包含四个精确卷名。任一卷缺失时停止，不执行清理。

- [ ] **Step 3: 清理非卷资源**

```powershell
docker container prune -f
docker network prune -f
docker image prune -af
docker builder prune -af
```

Expected: 四条命令均 exit 0。命令不得附带 `--volumes`，不得运行任何 volume prune/remove。

- [ ] **Step 4: 验证卷保留并记录逻辑释放量**

```powershell
docker system df -v | Tee-Object -FilePath "$evidenceRoot\docker-after.txt"
docker volume inspect $volumeNames |
    Set-Content -LiteralPath "$evidenceRoot\volumes-after.json" -Encoding utf8
$beforeNames = (Get-Content -Raw "$evidenceRoot\volumes-before.json" | ConvertFrom-Json).Name | Sort-Object
$afterNames = (Get-Content -Raw "$evidenceRoot\volumes-after.json" | ConvertFrom-Json).Name | Sort-Object
if (Compare-Object $beforeNames $afterNames) {
    throw 'Docker 清理前后数据卷清单不一致'
}
Get-Item 'D:\Docker\wsl-data\DockerDesktopWSL\disk\docker_data.vhdx' |
    Select-Object FullName, @{Name='SizeGB';Expression={[math]::Round($_.Length / 1GB, 2)}}
```

Expected: 四个卷前后完全一致。VHDX 只记录，不执行手工压缩、迁移或删除。

---

### Task 2: 提取可测试的概览展示组件

**Files:**
- Create: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.tsx`
- Create: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.test.tsx`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx`

**Interfaces:**
- Consumes: `DashboardOverviewTrendPoint`、`DashboardOverviewAIInsight`、`DashboardOverviewConversation`。
- Produces: `OverviewMetricCard`、`OverviewModuleHeader`、`OverviewEmptyState`、`OverviewAISummary`、`OverviewTrendChart`、`OverviewConversationWorkspace`、`parseAISummary`。
- `dashboard-overview-page.tsx` 保持导出 `parseAISummary`，避免现有测试和潜在消费者断裂。

- [ ] **Step 1: 写组件边界失败测试**

创建 `dashboard-overview-widgets.test.tsx`：

```tsx
import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it } from 'vitest';

import {
  OverviewAISummary,
  OverviewConversationWorkspace,
  OverviewMetricCard,
  parseAISummary,
} from './dashboard-overview-widgets';

const conversation = {
  customer: { sessions: 11, employeeMessages: 22, customerMessages: 16 },
  room: { sessions: 4, employeeMessages: 6, customerMessages: 6 },
  trend: [{
    date: '2026-08-16',
    customerSessions: 7,
    customerEmployeeMessages: 9,
    customerCustomerMessages: 5,
    roomSessions: 2,
    roomEmployeeMessages: 3,
    roomCustomerMessages: 2,
  }],
};

describe('dashboard overview widgets', () => {
  it('renders a labelled metric without inventing a change rate', () => {
    render(<OverviewMetricCard label="客户总数" note="同客户分析口径" tone="blue" value={137} />);
    const card = screen.getByRole('article', { name: '客户总数' });
    expect(within(card).getByText('137')).toBeTruthy();
    expect(within(card).queryByText(/%/)).toBeNull();
  });

  it('renders structured AI content and a real detail route', () => {
    render(<MemoryRouter><OverviewAISummary insight={{
      capability: 'ready',
      provider: 'dashscope',
      generatedAt: '2026-08-15T23:59:59+08:00',
      summary: '✅ **核心意图**\n1. 立即跟进',
    }} /></MemoryRouter>);
    expect(screen.getByText('核心意图')).toBeTruthy();
    expect(screen.getByRole('link', { name: '查看 AI 洞察' }).getAttribute('href'))
      .toBe('/ai-insight/smart-analysis');
  });

  it('switches the conversation summary, chart and table together', () => {
    render(<OverviewConversationWorkspace conversation={conversation} unavailable={false} />);
    expect(screen.getByRole('img', { name: '近七日客户会话趋势' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /客户群/ }));
    expect(screen.getByRole('img', { name: '近七日客户群趋势' })).toBeTruthy();
    expect(screen.getByLabelText('会话数 2')).toBeTruthy();
  });

  it('shows a provider limitation instead of zero conversation metrics', () => {
    render(<OverviewConversationWorkspace conversation={undefined} unavailable />);
    expect(screen.getByText('会话归档尚未接入')).toBeTruthy();
    expect(screen.queryByText('员工消息数')).toBeNull();
  });

  it('parses supported AI summary lines', () => {
    expect(parseAISummary('---\n✅ **标题**\n1. 第一项\n- 子项\n正文')).toEqual([
      { kind: 'divider' },
      { kind: 'section', text: '✅ **标题**' },
      { kind: 'numbered', index: '1', text: '第一项' },
      { kind: 'bullet', text: '子项' },
      { kind: 'paragraph', text: '正文' },
    ]);
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/dashboard-overview/dashboard-overview-widgets.test.tsx
```

Expected: FAIL，原因是 `dashboard-overview-widgets` 或目标导出不存在。

- [ ] **Step 3: 创建展示组件文件**

创建 `dashboard-overview-widgets.tsx`，实现以下公开结构；原页面中的趋势计算、AI 文本解析和会话字段映射原样迁入对应函数：

```tsx
import { Link } from 'react-router';
import { useState } from 'react';
import type { ReactNode } from 'react';

import type {
  ConversationTrendPoint,
  DashboardOverviewAIInsight,
  DashboardOverviewConversation,
  DashboardOverviewTrendPoint,
} from './dashboard-overview-api';

type AISummaryLine =
  | { kind: 'divider' }
  | { kind: 'section'; text: string }
  | { kind: 'numbered'; index: string; text: string }
  | { kind: 'bullet'; text: string }
  | { kind: 'paragraph'; text: string };

export function parseAISummary(summary: string): AISummaryLine[] {
  return summary.split(/\r?\n/).map((raw) => {
    const line = raw.trim();
    if (line === '') return null;
    if (/^-{3,}$/.test(line)) return { kind: 'divider' as const };
    if (/^[✅⚠️]/.test(line)) return { kind: 'section' as const, text: line };
    const numbered = line.match(/^(\d+)[.、．]\s*(.*)$/);
    if (numbered) return { kind: 'numbered' as const, index: numbered[1], text: numbered[2] };
    if (/^[-•]\s+/.test(line)) {
      return { kind: 'bullet' as const, text: line.replace(/^[-•]\s+/, '') };
    }
    return { kind: 'paragraph' as const, text: line };
  }).filter((line): line is AISummaryLine => line !== null);
}

function renderInline(text: string): ReactNode {
  return text.split(/\*\*(.+?)\*\*/g)
    .map((part, index) => (index % 2 === 1 ? <strong key={index}>{part}</strong> : part));
}

export function OverviewMetricCard({ label, value, note, tone }: {
  label: string;
  value: number;
  note: string;
  tone: 'blue' | 'green' | 'violet' | 'orange';
}) {
  return <article aria-label={label} className={`overview-metric overview-metric-${tone}`}>
    <span aria-hidden="true" className="overview-metric-icon" />
    <div className="overview-metric-copy">
      <span>{label}</span>
      <strong>{value.toLocaleString('zh-CN')}</strong>
      <small>{note}</small>
    </div>
  </article>;
}

export function OverviewModuleHeader({ title, description, extra, headingId }: {
  title: string;
  description?: string;
  extra?: ReactNode;
  headingId: string;
}) {
  return <header className="overview-module-header">
    <div><h2 id={headingId}>{title}</h2>{description && <p>{description}</p>}</div>
    {extra && <div className="overview-module-extra">{extra}</div>}
  </header>;
}

export function OverviewEmptyState({ title, description, action }: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return <div className="overview-empty-visual" role="status">
    <span aria-hidden="true" className="overview-empty-symbol" />
    <strong>{title}</strong>
    <p>{description}</p>
    {action}
  </div>;
}

export function OverviewAISummary({ insight }: { insight?: DashboardOverviewAIInsight }) {
  if (insight?.capability !== 'ready' || insight.summary === '') {
    return <OverviewEmptyState
      action={<Link className="overview-link-button" to="/ai-setting/ai-knowledge-base">前往 AI 设置</Link>}
      description="配置 AI Provider 和知识库后，这里会展示基于真实会话生成的经营建议。"
      title="AI 能力尚未接入"
    />;
  }
  return <div className="overview-ai-panel">
    <div className="overview-ai-summary">
      {parseAISummary(insight.summary).map((line, index) => {
        if (line.kind === 'divider') return <hr className="overview-ai-summary-divider" key={index} />;
        if (line.kind === 'section') return <p className="overview-ai-summary-section" key={index}>{renderInline(line.text)}</p>;
        if (line.kind === 'numbered') return <p className="overview-ai-summary-item" key={index}><span className="overview-ai-summary-index">{line.index}.</span><span>{renderInline(line.text)}</span></p>;
        if (line.kind === 'bullet') return <p className="overview-ai-summary-bullet" key={index}>{renderInline(line.text)}</p>;
        return <p className="overview-ai-summary-paragraph" key={index}>{renderInline(line.text)}</p>;
      })}
    </div>
    <Link className="overview-link-button" to="/ai-insight/smart-analysis">查看 AI 洞察</Link>
  </div>;
}

function maximum(points: readonly DashboardOverviewTrendPoint[]): number {
  return Math.max(1, ...points.map((point) => point.addCustomerNum));
}

export function OverviewTrendChart({ points }: { points: readonly DashboardOverviewTrendPoint[] }) {
  const max = maximum(points);
  return <div aria-label="企业客户增长趋势" className="dashboard-overview-chart" role="img">
    {points.map((point) => <div className="dashboard-overview-chart-column" key={point.date}>
      <span className="dashboard-overview-bar-value">{point.addCustomerNum}</span>
      <div className="dashboard-overview-bars"><span
        aria-label={`新增客户 ${point.addCustomerNum}`}
        className="dashboard-overview-bar dashboard-overview-bar-primary"
        style={{ height: `${Math.max(4, (point.addCustomerNum / max) * 100)}%` }}
      /></div>
      <span>{point.date.slice(5)}</span>
    </div>)}
  </div>;
}

type ConversationKind = 'customer' | 'room';

function kindLabel(kind: ConversationKind): string {
  return kind === 'room' ? '客户群' : '客户会话';
}

function conversationValue(kind: ConversationKind, point: ConversationTrendPoint): number {
  return kind === 'customer' ? point.customerSessions : point.roomSessions;
}

function ConversationChart({ kind, points }: {
  kind: ConversationKind;
  points: readonly ConversationTrendPoint[];
}) {
  const max = Math.max(1, ...points.map((point) => conversationValue(kind, point)));
  return <div aria-label={`近七日${kindLabel(kind)}趋势`} className="dashboard-overview-chart" role="img">
    {points.map((point) => {
      const value = conversationValue(kind, point);
      return <div className="dashboard-overview-chart-column" key={point.date}>
        <span className="dashboard-overview-bar-value">{value}</span>
        <div className="dashboard-overview-bars"><span
          aria-label={`会话数 ${value}`}
          className="dashboard-overview-bar dashboard-overview-bar-primary"
          style={{ height: `${Math.max(4, (value / max) * 100)}%` }}
        /></div>
        <span>{point.date.slice(5)}</span>
      </div>;
    })}
  </div>;
}

export function OverviewConversationWorkspace({ conversation, unavailable }: {
  conversation?: DashboardOverviewConversation;
  unavailable: boolean;
}) {
  const [kind, setKind] = useState<ConversationKind>('customer');
  if (unavailable || conversation === undefined) {
    return <OverviewEmptyState
      description="完成企业微信会话存档配置后，这里会展示客户与客户群的真实消息趋势。"
      title="会话归档尚未接入"
    />;
  }
  const selected = conversation[kind];
  return <div className="overview-conversation">
    <div className="overview-conversation-groups" role="group" aria-label="会话类型">
      {(['customer', 'room'] as const).map((candidate) => {
        const stats = conversation[candidate];
        return <button
          aria-pressed={kind === candidate}
          className={`overview-conversation-group${kind === candidate ? ' overview-conversation-group-active' : ''}`}
          key={candidate}
          onClick={() => setKind(candidate)}
          type="button"
        >
          <span>{kindLabel(candidate)}</span>
          <strong>{stats.sessions.toLocaleString('zh-CN')}</strong>
          <small>区间会话数</small>
        </button>;
      })}
      <dl className="overview-conversation-summary">
        <div><dt>会话数</dt><dd>{selected.sessions.toLocaleString('zh-CN')}</dd></div>
        <div><dt>员工消息数</dt><dd>{selected.employeeMessages.toLocaleString('zh-CN')}</dd></div>
        <div><dt>客户消息数</dt><dd>{selected.customerMessages.toLocaleString('zh-CN')}</dd></div>
      </dl>
    </div>
    <div className="overview-conversation-chart">
      <div className="overview-conversation-chart-title"><h3>{kindLabel(kind)}趋势</h3><span>近七日</span></div>
      {conversation.trend.length === 0
        ? <OverviewEmptyState description="当前周期内没有可展示的会话记录。" title="暂无会话趋势" />
        : <ConversationChart kind={kind} points={conversation.trend} />}
    </div>
    <div aria-label="近七日会话趋势明细" className="overview-conversation-detail">
      <h3>{kindLabel(kind)} · 近七日趋势明细</h3>
      <div className="dashboard-table-scroll"><table><thead><tr><th>日期</th><th>会话数</th><th>员工消息数</th><th>客户消息数</th></tr></thead><tbody>
        {conversation.trend.map((point) => <tr key={point.date}>
          <td>{point.date}</td>
          <td>{kind === 'customer' ? point.customerSessions : point.roomSessions}</td>
          <td>{kind === 'customer' ? point.customerEmployeeMessages : point.roomEmployeeMessages}</td>
          <td>{kind === 'customer' ? point.customerCustomerMessages : point.roomCustomerMessages}</td>
        </tr>)}
      </tbody></table></div>
    </div>
  </div>;
}
```

- [ ] **Step 4: 从页面移除重复展示函数并保持兼容导出**

在 `dashboard-overview-page.tsx` 删除已经迁移的 `TrendChart`、`MetricCard`、`ModuleHeader`、`EmptyVisual`、AI 文本和会话展示函数，加入：

```tsx
import {
  OverviewAISummary,
  OverviewConversationWorkspace,
  OverviewEmptyState,
  OverviewMetricCard,
  OverviewModuleHeader,
  OverviewTrendChart,
} from './dashboard-overview-widgets';

export { parseAISummary } from './dashboard-overview-widgets';
```

- [ ] **Step 5: 运行组件测试确认通过**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/dashboard-overview/dashboard-overview-widgets.test.tsx src/features/dashboard-overview/dashboard-overview-page.test.tsx
```

Expected: PASS，且现有会话切换、AI 解析、日期、403 和导出测试不回退。

- [ ] **Step 6: 提交**

```powershell
git add -- web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.tsx web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.test.tsx web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx
git commit -m "refactor: extract dashboard overview widgets"
```

---

### Task 3: 重排驾驶舱信息层级与操作区

**Files:**
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.test.tsx`

**Interfaces:**
- Consumes: Task 2 的展示组件、现有 `DashboardOverviewApi`、`useDashboardAccess`、`DateRangeFields`、`DashboardPagination`。
- Produces: 五层页面结构；主日期、趋势日期、会话切换、刷新和导出行为保持现有合同。

- [ ] **Step 1: 写信息架构失败测试**

在 `dashboard-overview-page.test.tsx` 增加：

```tsx
it('renders the overview as a five-layer operating cockpit', async () => {
  renderPage({ load: vi.fn(() => Promise.resolve(overview)) });
  await screen.findByText('客户总数');

  expect(screen.getByText('测试企业')).toBeTruthy();
  expect(screen.getByText('统一报表口径')).toBeTruthy();
  expect(screen.getByRole('region', { name: '经营快照' })).toBeTruthy();
  expect(screen.getByRole('region', { name: 'AI 洞察' })).toBeTruthy();
  expect(screen.getByRole('region', { name: '客户增长趋势' })).toBeTruthy();
  expect(screen.getByRole('region', { name: '会话工作台' })).toBeTruthy();
  expect(screen.getByRole('region', { name: '经营趋势明细' })).toBeTruthy();
  expect(screen.getByRole('link', { name: '查看 AI 洞察' })).toBeTruthy();
});

it('keeps data visible while a manual refresh is pending', async () => {
  let resolveRefresh: ((value: DashboardOverview) => void) | undefined;
  const load = vi.fn()
    .mockResolvedValueOnce(overview)
    .mockImplementationOnce(() => new Promise<DashboardOverview>((resolve) => { resolveRefresh = resolve; }));
  renderPage({ load });
  expect(await screen.findByText('137')).toBeTruthy();

  fireEvent.click(screen.getByRole('button', { name: '刷新' }));
  expect(screen.getByText('137')).toBeTruthy();
  expect(screen.getByRole('button', { name: '刷新' }).hasAttribute('disabled')).toBe(true);
  resolveRefresh?.(overview);
  await waitFor(() => expect(screen.getByRole('button', { name: '刷新' }).hasAttribute('disabled')).toBe(false));
});
```

同时把现有 `keeps the complete business dashboard visible for a successful empty response` 中与本轮信息架构冲突的断言替换为：

```tsx
expect(await screen.findByRole('heading', { name: '经营快照' })).toBeTruthy();
expect(screen.getByRole('heading', { name: '数据概览' })).toBeTruthy();
expect(screen.getByRole('heading', { name: 'AI 洞察' })).toBeTruthy();
expect(screen.getByRole('heading', { name: '会话工作台' })).toBeTruthy();
expect(screen.getByRole('heading', { name: '经营趋势明细' })).toBeTruthy();
expect(screen.getByText('AI 能力尚未接入')).toBeTruthy();
expect(screen.getByText('会话归档尚未接入')).toBeTruthy();
expect(screen.getByText('暂无增长趋势')).toBeTruthy();
expect(screen.getByText('暂无经营明细')).toBeTruthy();
expect(screen.queryByRole('heading', { name: '质检数据' })).toBeNull();
expect(screen.queryByRole('heading', { name: '员工会话数据排行' })).toBeNull();
expect(screen.queryByRole('heading', { name: '员工会话轨迹一览' })).toBeNull();
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/dashboard-overview/dashboard-overview-page.test.tsx
```

Expected: FAIL，缺少“经营快照”“会话工作台”等 region 和企业/口径标签。

- [ ] **Step 3: 重写 `BusinessDashboard` 的结构**

保持现有 props、分页和趋势 draft 行为，主体改为以下层级：

```tsx
return <div className="overview-dashboard">
  <section aria-labelledby="overview-snapshot-title" className="overview-module overview-snapshot dashboard-data-card">
    <OverviewModuleHeader
      headingId="overview-snapshot-title"
      title="经营快照"
      description="客户、线索、订单与行为事件，与数据报表保持同一口径"
      extra={<span className="overview-update">更新于 {data.updatedAt || '--'}</span>}
    />
    <div className="overview-metric-grid dashboard-stat-grid">
      <OverviewMetricCard label={data.cards[0]?.label ?? '客户总数'} value={data.summary.customer} note="联系人与分配关系" tone="blue" />
      <OverviewMetricCard label={data.cards[1]?.label ?? '线索总数'} value={data.summary.lead} note="转化漏斗起始阶段" tone="green" />
      <OverviewMetricCard label={data.cards[2]?.label ?? '订单总数'} value={data.summary.order} note="won / completed / paid" tone="violet" />
      <OverviewMetricCard label={data.cards[3]?.label ?? '行为事件'} value={data.summary.behavior} note="订单与设置审计事件" tone="orange" />
    </div>
  </section>

  <div className="overview-intelligence-grid">
    <section aria-labelledby="overview-ai-title" className="overview-module dashboard-data-card">
      <OverviewModuleHeader
        headingId="overview-ai-title"
        title="AI 洞察"
        description="基于真实会话归档生成的经营信号与跟进建议"
        extra={aiInsight?.capability === 'ready'
          ? <span className="overview-update">生成于 {aiInsight.generatedAt.replace('T', ' ').replace('Z', '').slice(0, 19)}</span>
          : <span className="overview-capability-chip">待配置</span>}
      />
      <OverviewAISummary insight={aiInsight} />
    </section>

    <section aria-labelledby="overview-growth-title" className="overview-module dashboard-data-card">
      <OverviewModuleHeader
        headingId="overview-growth-title"
        title="客户增长趋势"
        description="按天统计新增联系人，默认展示最近七天"
        extra={<span className="overview-scope-chip">SCRM · Asia/Shanghai</span>}
      />
      <div className="overview-trend-range"><DateRangeFields
        value={{ startDate: trendDraft.from, endDate: trendDraft.to }}
        endLabel="趋势结束"
        startLabel="趋势开始"
        submitLabel="应用区间"
        onChange={(value) => onTrendDraftChange({ from: value.startDate, to: value.endDate })}
        onValidSubmit={onTrendApply}
      /></div>
      {data.trend.length === 0
        ? <OverviewEmptyState title="暂无增长趋势" description="当前日期范围没有新增客户记录。" />
        : <OverviewTrendChart points={data.trend} />}
    </section>
  </div>

  <section aria-labelledby="overview-conversation-title" className="overview-module dashboard-data-card">
    <OverviewModuleHeader
      headingId="overview-conversation-title"
      title="会话工作台"
      description="客户与客户群会话汇总、消息构成和近七日趋势"
      extra={<span className="overview-scope-chip">数据来源：会话归档</span>}
    />
    <OverviewConversationWorkspace conversation={conversation} unavailable={archiveUnavailable} />
  </section>

  <section aria-labelledby="overview-detail-title" className="overview-module overview-detail-table dashboard-data-card">
    <OverviewModuleHeader headingId="overview-detail-title" title="经营趋势明细" description="与当前筛选和导出范围保持一致" />
    {data.trend.length === 0
      ? <OverviewEmptyState title="暂无经营明细" description="调整日期范围后再次查询。" />
      : <div className="dashboard-table-scroll"><table><thead><tr><th>日期</th><th>新增客户</th></tr></thead><tbody>
          {data.trend.map((point) => <tr key={point.date}><td>{point.date}</td><td>{point.addCustomerNum}</td></tr>)}
        </tbody></table></div>}
    <DashboardPagination page={page} pageSize={pageSize} total={total}
      onPageChange={(nextPage) => setSearchParams(overviewSearch(searchParams, current, nextPage, pageSize))} />
  </section>
</div>;
```

- [ ] **Step 4: 重写页首控制台**

导入 `Button`：

```tsx
import { Button } from 'antd';
```

把顶层 header 改为：

```tsx
<header className="dashboard-overview-header dashboard-page-header dashboard-data-card">
  <div className="dashboard-overview-heading">
    <p className="dashboard-overview-eyebrow">数据中心</p>
    <div className="dashboard-overview-title-row"><h1>数据概览</h1><span>统一报表口径</span></div>
    <p>当前企业：<strong>{access.corp.name}</strong> · 查看客户、转化、行为与会话经营状态。</p>
  </div>
  <div className="dashboard-overview-filters dashboard-filter-bar">
    <DateRangeFields
      value={{ startDate: draft.from, endDate: draft.to }}
      onChange={(value) => setDraft({ from: value.startDate, to: value.endDate })}
      onValidSubmit={applyFilters}
    />
    <div className="dashboard-overview-actions">
      <Button aria-label="刷新" disabled={query.isFetching} loading={query.isFetching} onClick={() => void query.refetch()}>刷新</Button>
      <Button aria-label="导出 CSV" disabled={query.isFetching} onClick={() => void exportCsv()}>导出 CSV</Button>
    </div>
  </div>
</header>
```

- [ ] **Step 5: 运行页面测试确认通过**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/dashboard-overview/dashboard-overview-page.test.tsx src/features/dashboard-overview/dashboard-overview-widgets.test.tsx
```

Expected: PASS，测试数量不少于改动前 16 个页面测试加 5 个 widgets 测试。

- [ ] **Step 6: 提交**

```powershell
git add -- web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.test.tsx
git commit -m "feat: redesign dashboard overview cockpit"
```

---

### Task 4: 建立圆弧式视觉层级和响应式合同

**Files:**
- Create: `web/apps/dashboard/src/styles/dashboard-overview-layout.test.ts`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Consumes: Task 2–3 的 class names。
- Produces: 经营快照四列、智能区 5:7、会话区 4:8、1180px/720px 断点、页面作用域和 reduced-motion 合同。

- [ ] **Step 1: 写样式合同失败测试**

创建 `dashboard-overview-layout.test.ts`：

```ts
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const css = readFileSync('src/styles/index.css', 'utf8');

function rule(selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = css.match(new RegExp(`(?:^|\\n)\\s*${escaped}\\s*\\{([^}]*)\\}`));
  expect(match, `missing ${selector}`).not.toBeNull();
  return match?.[1] ?? '';
}

describe('dashboard overview cockpit layout', () => {
  it('uses a four-card snapshot and a 5:7 intelligence grid', () => {
    expect(rule('.dashboard-overview-page .overview-metric-grid')).toContain('repeat(4, minmax(0, 1fr))');
    expect(rule('.dashboard-overview-page .overview-intelligence-grid')).toContain('minmax(0, 5fr) minmax(0, 7fr)');
  });

  it('uses a 4:8 conversation workspace and scroll-safe tables', () => {
    expect(rule('.dashboard-overview-page .overview-conversation')).toContain('minmax(280px, 4fr) minmax(0, 8fr)');
    expect(rule('.dashboard-overview-page .dashboard-table-scroll')).toContain('overflow-x: auto');
  });

  it('defines tablet, mobile and reduced-motion behavior', () => {
    expect(css).toContain('@media (max-width: 1180px)');
    expect(css).toContain('@media (max-width: 720px)');
    expect(css).toContain('@media (prefers-reduced-motion: reduce)');
    expect(css).toContain('.overview-intelligence-grid { grid-template-columns: 1fr; }');
    expect(css).toContain('.overview-metric-grid { grid-template-columns: 1fr; }');
  });
});
```

- [ ] **Step 2: 运行样式测试确认失败**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/styles/dashboard-overview-layout.test.ts
```

Expected: FAIL，缺少 5:7 智能区、4:8 会话区或新断点合同。

- [ ] **Step 3: 替换 overview 专用样式块**

在 `index.css` 中删除旧 `.dashboard-overview-page` 至 overview 响应式规则，并写入以下新规则；保留其他页面公共规则：

```css
.dashboard-overview-page { display: grid; gap: 18px; margin: 0; max-width: none; }
.dashboard-overview-header { align-items: flex-end; display: flex; gap: 28px; justify-content: space-between; padding: 24px 28px; }
.dashboard-overview-heading { min-width: 260px; }
.dashboard-overview-eyebrow { color: #536bf4 !important; font-size: 12px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
.dashboard-overview-title-row { align-items: center; display: flex; flex-wrap: wrap; gap: 10px; margin: 5px 0 8px; }
.dashboard-overview-title-row h1 { color: #20283a; font-size: 26px; line-height: 1.2; margin: 0; }
.dashboard-overview-title-row span, .overview-capability-chip { background: #eef2ff; border: 1px solid #dde4ff; border-radius: 999px; color: #536bf4; font-size: 12px; padding: 5px 9px; }
.dashboard-overview-heading > p:last-child { color: #788195; margin: 0; }
.dashboard-overview-heading strong { color: #414b5f; font-weight: 600; }
.dashboard-overview-filters { align-items: flex-end; display: flex; flex: 1; flex-wrap: wrap; justify-content: flex-end; }
.dashboard-overview-actions { display: flex; gap: 8px; }
.dashboard-overview-inline-error { background: #fff2f0; border: 1px solid #ffccc7; border-radius: 10px; color: #b42318; margin: 0; padding: 10px 14px; }
.overview-dashboard { display: grid; gap: 18px; }
.overview-module { min-width: 0; overflow: hidden; padding: 24px 26px; }
.overview-module-header { align-items: flex-start; display: flex; gap: 16px; justify-content: space-between; margin-bottom: 20px; }
.overview-module-header h2 { color: #20283a; font-size: 18px; line-height: 1.3; margin: 0; }
.overview-module-header p { color: #8992a3; font-size: 13px; line-height: 1.6; margin: 6px 0 0; }
.overview-module-extra { align-items: center; display: flex; flex-wrap: wrap; gap: 8px; }
.overview-update, .overview-scope-chip { background: #f7f9fc; border: 1px solid #e7ebf2; border-radius: 999px; color: #727d91; font-size: 12px; padding: 6px 10px; white-space: nowrap; }
.dashboard-overview-page .overview-metric-grid { display: grid; gap: 14px; grid-template-columns: repeat(4, minmax(0, 1fr)); }
.overview-metric { align-items: flex-start; background: #fbfcff; border: 1px solid #edf0f6; border-radius: 12px; display: flex; gap: 14px; min-height: 128px; padding: 18px; transition: border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease; }
.overview-metric:hover { border-color: #dce3f6; box-shadow: 0 10px 28px rgb(47 63 111 / 8%); transform: translateY(-1px); }
.overview-metric-icon { background: #e9efff; border-radius: 10px; display: block; flex: 0 0 38px; height: 38px; position: relative; width: 38px; }
.overview-metric-icon::after { background: #5c74ed; border-radius: 4px 4px 8px 4px; content: ''; height: 14px; left: 12px; position: absolute; top: 12px; transform: rotate(-18deg); width: 14px; }
.overview-metric-green .overview-metric-icon { background: #e7f8f2; }
.overview-metric-green .overview-metric-icon::after { background: #39aa82; }
.overview-metric-violet .overview-metric-icon { background: #f0ebff; }
.overview-metric-violet .overview-metric-icon::after { background: #8066db; }
.overview-metric-orange .overview-metric-icon { background: #fff2e5; }
.overview-metric-orange .overview-metric-icon::after { background: #e99a4b; }
.overview-metric-copy { display: grid; gap: 5px; min-width: 0; }
.overview-metric-copy > span { color: #657086; font-size: 13px; }
.overview-metric-copy strong { color: #20283a; font-size: 30px; line-height: 1.05; }
.overview-metric-copy small { color: #99a1af; font-size: 12px; line-height: 1.5; }
.dashboard-overview-page .overview-intelligence-grid { display: grid; gap: 18px; grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); }
.overview-ai-panel { display: grid; gap: 14px; }
.overview-ai-summary { max-height: 330px; overflow: auto; padding-right: 8px; }
.overview-ai-summary p { color: #4f596d; line-height: 1.75; margin: 0; }
.overview-ai-summary p + p { margin-top: 10px; }
.overview-ai-summary-section { color: #263043 !important; font-weight: 700; }
.overview-ai-summary-item { display: grid; gap: 8px; grid-template-columns: 22px minmax(0, 1fr); }
.overview-ai-summary-index { color: #536bf4; font-weight: 700; }
.overview-ai-summary-bullet { padding-left: 16px; position: relative; }
.overview-ai-summary-bullet::before { background: #8fa0f7; border-radius: 50%; content: ''; height: 5px; left: 2px; position: absolute; top: .72em; width: 5px; }
.overview-ai-summary-divider { border: 0; border-top: 1px solid #edf0f6; margin: 16px 0; }
.overview-link-button { align-items: center; align-self: start; background: #eef2ff; border: 1px solid #dde4ff; border-radius: 8px; color: #526be8; display: inline-flex; font-size: 13px; justify-content: center; min-height: 36px; padding: 7px 12px; text-decoration: none; width: fit-content; }
.overview-trend-range { display: flex; justify-content: flex-end; margin-bottom: 10px; }
.dashboard-overview-page .dashboard-overview-chart { align-items: end; display: flex; gap: 12px; height: 250px; margin-top: 12px; overflow-x: auto; padding: 12px 4px 0; }
.dashboard-overview-page .dashboard-overview-chart-column { align-items: center; color: #8991a1; display: grid; flex: 1 0 50px; font-size: 11px; gap: 7px; height: 100%; min-width: 50px; text-align: center; }
.dashboard-overview-page .dashboard-overview-bars { align-items: end; border-bottom: 1px solid #e8ebf2; display: flex; height: 205px; justify-content: center; width: 100%; }
.dashboard-overview-bar { background: linear-gradient(180deg, #7f91f7, #536bf4); border-radius: 6px 6px 2px 2px; display: block; max-width: 16px; min-height: 4px; width: 34%; }
.dashboard-overview-bar-value { color: #687287; font-size: 11px; line-height: 1; }
.dashboard-overview-page .overview-conversation { display: grid; gap: 20px; grid-template-columns: minmax(280px, 4fr) minmax(0, 8fr); }
.overview-conversation-groups { align-content: start; display: grid; gap: 10px; grid-template-columns: repeat(2, minmax(0, 1fr)); }
.overview-conversation-group { background: #fbfcff; border: 1px solid #e8ecf4; border-radius: 12px; color: #526075; cursor: pointer; display: grid; gap: 6px; min-height: 110px; padding: 16px; text-align: left; transition: border-color 160ms ease, box-shadow 160ms ease; }
.overview-conversation-group strong { color: #20283a; font-size: 26px; }
.overview-conversation-group small { color: #99a1af; }
.overview-conversation-group-active { background: linear-gradient(145deg, #5b78f6, #398cf5); border-color: transparent; box-shadow: 0 10px 26px rgb(65 105 230 / 20%); color: #fff; }
.overview-conversation-group-active strong, .overview-conversation-group-active small { color: #fff; }
.overview-conversation-summary { background: #f8faff; border-radius: 12px; display: grid; gap: 0; grid-column: 1 / -1; margin: 0; padding: 4px 16px; }
.overview-conversation-summary div { align-items: center; border-bottom: 1px solid #edf0f6; display: flex; justify-content: space-between; padding: 12px 0; }
.overview-conversation-summary div:last-child { border-bottom: 0; }
.overview-conversation-summary dt { color: #778196; font-size: 13px; }
.overview-conversation-summary dd { color: #263043; font-size: 16px; font-weight: 700; margin: 0; }
.overview-conversation-chart { border-left: 1px solid #edf0f6; min-width: 0; padding-left: 20px; }
.overview-conversation-chart-title { align-items: center; display: flex; justify-content: space-between; }
.overview-conversation-chart-title h3, .overview-conversation-detail h3 { color: #414b5f; font-size: 14px; margin: 0; }
.overview-conversation-chart-title span { color: #8c95a6; font-size: 12px; }
.overview-conversation-detail { grid-column: 1 / -1; padding-top: 4px; }
.overview-conversation-detail h3 { margin-bottom: 12px; }
.overview-empty-visual { align-items: center; color: #8992a3; display: flex; flex-direction: column; justify-content: center; min-height: 210px; padding: 24px; text-align: center; }
.overview-empty-symbol { background: #eef2ff; border-radius: 50%; height: 56px; margin-bottom: 14px; position: relative; width: 56px; }
.overview-empty-symbol::after { border: 2px solid #91a1f4; border-radius: 50%; content: ''; height: 18px; left: 17px; position: absolute; top: 17px; width: 18px; }
.overview-empty-visual strong { color: #4b566a; }
.overview-empty-visual p { font-size: 13px; line-height: 1.6; margin: 7px 0 14px; max-width: 420px; }
.dashboard-overview-page .overview-detail-table table, .dashboard-overview-page .overview-conversation-detail table { border-collapse: collapse; min-width: 520px; width: 100%; }
.dashboard-overview-page .overview-detail-table th, .dashboard-overview-page .overview-detail-table td, .dashboard-overview-page .overview-conversation-detail th, .dashboard-overview-page .overview-conversation-detail td { border-bottom: 1px solid #eef1f5; padding: 12px 14px; text-align: left; }
.dashboard-overview-page .overview-detail-table th, .dashboard-overview-page .overview-conversation-detail th { background: #fafbfe; color: #737d90; font-size: 12px; }
.dashboard-overview-page .overview-detail-table td, .dashboard-overview-page .overview-conversation-detail td { color: #465065; font-size: 13px; }
.dashboard-overview-page .dashboard-table-scroll { overflow-x: auto; }

@media (max-width: 1180px) {
  .dashboard-overview-page .overview-metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .dashboard-overview-page .overview-intelligence-grid { grid-template-columns: 1fr; }
  .dashboard-overview-page .overview-conversation { grid-template-columns: 1fr; }
  .dashboard-overview-page .overview-conversation-chart { border-left: 0; border-top: 1px solid #edf0f6; padding-left: 0; padding-top: 18px; }
}

@media (max-width: 720px) {
  .dashboard-overview-header { align-items: stretch; flex-direction: column; padding: 20px 18px; }
  .dashboard-overview-filters { align-items: stretch; justify-content: flex-start; }
  .dashboard-overview-actions { width: 100%; }
  .dashboard-overview-actions .ant-btn { flex: 1; min-height: 40px; }
  .dashboard-overview-page .overview-metric-grid { grid-template-columns: 1fr; }
  .dashboard-overview-page .overview-conversation-groups { grid-template-columns: 1fr; }
  .dashboard-overview-page .overview-module { padding: 20px 18px; }
  .dashboard-overview-page .overview-module-header { align-items: stretch; flex-direction: column; }
  .dashboard-overview-page .overview-module-extra { justify-content: flex-start; }
  .dashboard-overview-page .overview-trend-range { justify-content: stretch; }
  .dashboard-overview-page .overview-trend-range .date-range-fields { width: 100%; }
}

@media (prefers-reduced-motion: reduce) {
  .dashboard-overview-page .overview-metric, .dashboard-overview-page .overview-conversation-group { transition: none; }
}
```

- [ ] **Step 4: 运行样式和组件测试**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/styles/dashboard-overview-layout.test.ts src/features/dashboard-overview/dashboard-overview-widgets.test.tsx src/features/dashboard-overview/dashboard-overview-page.test.tsx
```

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add -- web/apps/dashboard/src/styles/index.css web/apps/dashboard/src/styles/dashboard-overview-layout.test.ts
git commit -m "style: polish dashboard overview cockpit"
```

---

### Task 5: 增加桌面与 390px 视觉回归

**Files:**
- Create: `web/e2e/tests/dashboard-overview-visual.spec.ts`
- Create at execution time: `D:\workspace\mochat-go\output\overview-ui-refresh-20260818\overview-desktop.png`
- Create at execution time: `D:\workspace\mochat-go\output\overview-ui-refresh-20260818\overview-mobile-390.png`

**Interfaces:**
- Consumes: `mockDashboardBackend`、`seedSession`、Dashboard `/index`。
- Produces: 首屏层级与移动端无横向溢出的 Playwright 合同及两张验收截图。

- [ ] **Step 1: 写 E2E 测试**

```ts
import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from '@playwright/test';
import { mockDashboardBackend, seedSession } from './helpers';

const evidenceRoot = resolve('..', '..', 'output', 'overview-ui-refresh-20260818');
mkdirSync(evidenceRoot, { recursive: true });
const overviewPayload = {
  summary: { customer: 137, lead: 58, contact: 46, opportunity: 30, won: 15, order: 12, behavior: 41, employee: 0 },
  series: [
    { at: '2026-08-12T00:00:00+08:00', value: 4 },
    { at: '2026-08-13T00:00:00+08:00', value: 7 },
    { at: '2026-08-14T00:00:00+08:00', value: 5 },
    { at: '2026-08-15T00:00:00+08:00', value: 9 },
    { at: '2026-08-16T00:00:00+08:00', value: 12 },
  ],
  aiInsight: {
    capability: 'ready', provider: 'dashscope', generatedAt: '2026-08-16T00:00:00+08:00',
    summary: '✅ **核心客户意图**\n1. 商务洽谈与价格协商\n- 今日内发送报价单',
  },
  conversation: {
    customer: { sessions: 11, employeeMessages: 22, customerMessages: 16 },
    room: { sessions: 4, employeeMessages: 6, customerMessages: 6 },
    trend: [{ date: '2026-08-16', customerSessions: 7, customerEmployeeMessages: 9, customerCustomerMessages: 5, roomSessions: 2, roomEmployeeMessages: 3, roomCustomerMessages: 2 }],
  },
  limitations: [],
  freshness: { provider: 'scrm', status: 'available', dataThrough: '2026-08-16T09:30:00+08:00' },
  pagination: { page: 1, pageSize: 20, total: 5 },
};

test.beforeEach(async ({ page }) => {
  await seedSession(page);
  await mockDashboardBackend(page, { menuRoutes: ['/index', '/ai-insight/smart-analysis', '/ai-setting/ai-knowledge-base'] });
  await page.route('**/dashboard/reports/overview**', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 200, msg: 'success', data: overviewPayload }) });
  });
});

test('renders the desktop operating cockpit', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('/index');
  await expect(page.getByRole('region', { name: '经营快照' })).toBeVisible();
  await expect(page.getByRole('region', { name: '会话工作台' })).toBeVisible();
  await page.screenshot({ fullPage: true, path: resolve(evidenceRoot, 'overview-desktop.png') });
});

test('keeps the mobile cockpit within the 390px viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/index');
  await expect(page.getByRole('region', { name: '经营快照' })).toBeVisible();
  const size = await page.evaluate(() => ({ client: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth }));
  expect(size.scroll).toBeLessThanOrEqual(size.client);
  await page.screenshot({ fullPage: true, path: resolve(evidenceRoot, 'overview-mobile-390.png') });
});
```

- [ ] **Step 2: 运行 E2E 并修正测试夹具差异**

Run:

```powershell
corepack pnpm --filter @mochat/e2e exec playwright test tests/dashboard-overview-visual.spec.ts --workers=1
```

Expected: 2 passed，输出两张截图。若 fixture 响应字段与 `parseOverview` 不匹配，只修正测试 fixture 以符合现有 API 合同，不改生产解析器迎合错误 fixture。

- [ ] **Step 3: 提交**

```powershell
git add -- web/e2e/tests/dashboard-overview-visual.spec.ts
git commit -m "test: cover dashboard overview visual layout"
```

---

### Task 6: 全量门禁、Docker 重建与实页验收

**Files:**
- No repository files unless verification exposes a regression covered by Tasks 2–5。
- Write evidence under: `D:\workspace\mochat-go\output\overview-ui-refresh-20260818\`

**Interfaces:**
- Consumes: Tasks 1–5 的代码、测试和已保留卷。
- Produces: 新鲜的测试、构建、容器健康和浏览器验收证据。

- [ ] **Step 1: 运行 Dashboard 定向门禁**

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/dashboard-overview/dashboard-overview-api.test.ts src/features/dashboard-overview/dashboard-overview-widgets.test.tsx src/features/dashboard-overview/dashboard-overview-page.test.tsx src/styles/dashboard-overview-layout.test.ts
corepack pnpm --filter @mochat/dashboard lint
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
```

Expected: 全部 exit 0，无新增 warning/error。

- [ ] **Step 2: 运行 Dashboard 全量测试与 E2E 类型检查**

```powershell
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e exec playwright test tests/dashboard-overview-visual.spec.ts tests/dashboard-responsive.spec.ts --workers=1
```

Expected: 全部测试通过；`/index` 在 390px 无页面级横向溢出。

- [ ] **Step 3: 构建并只重建 `app`**

先再次确认四个卷：

```powershell
docker volume inspect mochat-go-desktop_mysql-data mochat-go-desktop_redis-data mochat-go-desktop_app-storage mochat-go-desktop_audit-anchor-storage
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml build app
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml up -d --no-deps app
```

Expected: build 和 up exit 0；命令中不存在 `-v`、`--volumes` 或 `ResetData`。

- [ ] **Step 4: 验证容器与健康端点**

```powershell
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps
$response = Invoke-WebRequest -UseBasicParsing 'http://127.0.0.1:18080/readyz'
if ($response.StatusCode -ne 200) { throw "readyz status=$($response.StatusCode)" }
docker volume inspect mochat-go-desktop_mysql-data mochat-go-desktop_redis-data mochat-go-desktop_app-storage mochat-go-desktop_audit-anchor-storage
```

Expected: `app` healthy，`/readyz` 200，四个卷仍存在。

- [ ] **Step 5: 浏览器实页检查并暂停**

在本地 `http://127.0.0.1:18080/#/index` 使用真实登录态检查：

- 1440px：经营快照四列、AI/增长双栏、会话左指标右趋势。
- 390px：操作区、KPI、AI/增长和会话全部单列，无页面级横向溢出。
- 正常数据、空数据、AI 未接入、会话归档未接入、403 和请求失败状态文案准确。
- 查询、刷新、导出、趋势日期、会话切换和分页可操作。
- 浏览器控制台无新增 error。

保存真实实页截图和控制台记录到 `D:\workspace\mochat-go\output\overview-ui-refresh-20260818\`。完成后立即暂停，等待用户检查数据概览，不开始下一个页面。

- [ ] **Step 6: 最终状态记录**

```powershell
git status --short
git log -6 --oneline
```

Expected: 只出现用户原有未提交项和本计划明确产生的提交；不推送远端，不合并其他分支。
