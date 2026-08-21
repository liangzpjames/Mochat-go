import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { MemoryRouter, RouterProvider } from 'react-router';

import { benchmarkManifest, type BenchmarkManifest } from './benchmark-manifest';
import { createBenchmarkP0Pages, createPageRegistry } from './page-registry';
import { FileAudioPage } from '../features/phase35/file-audio-page';
import { ResignedEmployeePage } from '../features/conversation-operations/resigned-employee-page';
import { RefuseArchivePage } from '../features/conversation-operations/refuse-archive-page';
import { CustomerInheritancePage } from '../features/conversation-operations/customer-inheritance-page';
import { createDashboardRouter } from '../app/router';
import { DashboardSessionActionsProvider } from '../features/auth/session-actions';
import { ConversationGlobalPage } from '../features/conversation-global/conversation-global-page';
import { EmployeeConversationPage } from '../features/conversation-global/employee-conversation-page';
import { CustomerConversationPage } from '../features/conversation-global/customer-conversation-page';
import { GroupConversationPage } from '../features/conversation-global/group-conversation-page';
import { ConversationTrajectoryPage } from '../features/conversation-global/conversation-trajectory-page';
import { ConversationExportPage } from '../features/conversation-global/conversation-export-page';
import { RiskWarningPage } from '../features/phase33/risk-warning-pages';
import { CustomerLossPage } from '../features/phase33/customer-loss-page';
import { RiskBehaviorPage } from '../features/phase33/risk-behavior-page';
import { TimeoutWarningPage } from '../features/phase33/timeout-warning-page';
import { KeywordLibraryPage, MessageInterceptPage } from '../features/phase33/message-intercept-pages';
import { SilentCustomerPage } from '../features/phase33/phase33-closure-pages';
import { ChannelCodePage, GroupCodePage, LiveCodeShortChainPage } from '../features/phase34/acquisition-pages';
import { GroupTemplatePage, RedirectLinkPage, WechatCustomerServicePage } from '../features/phase34/conversion-pages';
import { FriendsCirclePage, PreciseGroupSendPage } from '../features/phase34/content-reach-pages';
import { MaterialManagementPage } from '../features/phase34/material-management/material-management-page';

const manifest = {
  groups: [{ id: 'conversation', title: '会话' }],
  pages: [
    { path: '/index', title: '数据概览', groupId: null, level: 'P0' },
    { path: '/chat/staff', title: '员工会话', groupId: 'conversation', level: 'P1' },
    { path: '/chat/export', title: '会话导出', groupId: 'conversation', level: 'P2' },
  ],
} as const;

afterEach(cleanup);

const emptyOverview = {
  cards: [],
  trend: [],
  summary: {
    customer: 0, lead: 0, contact: 0, opportunity: 0, won: 0, order: 0, behavior: 0, employee: 0,
  },
  limitations: [],
  updatedAt: '',
  page: 1,
  pageSize: 20,
  total: 0,
};

function renderDashboardRoute(path: string) {
  const router = createDashboardRouter({
    getSession: () => ({
      token: 'Bearer test',
      userId: '7',
      expiresAt: null,
    }),
    initialEntries: [path],
    loadInitialData: () => Promise.resolve(),
    reactPages: createPageRegistry({
      manifest: benchmarkManifest,
      p0Pages: {},
      p1Pages: {},
    }),
  });

  return render(
    <DashboardSessionActionsProvider onLogout={() => Promise.resolve()} userId="7">
      <RouterProvider router={router} />
    </DashboardSessionActionsProvider>,
  );
}

describe('createPageRegistry', () => {
  it('registers the real global conversation P0 page', () => {
    const pages = createBenchmarkP0Pages({
      dashboardOverviewApi: {
        load: () => Promise.resolve(emptyOverview),
        exportCsv: () => Promise.resolve(new Blob()),
      },
      conversationGlobalApi: {
        search: () => Promise.resolve({ list: [], total: 0, page: 1, pageSize: 20 }),
        detail: () => Promise.reject(new Error('not loaded')),
        employees: () => Promise.resolve([]),
      },
    });

    expect((pages['/chat/v2-all'] as { type?: unknown }).type).toBe(ConversationGlobalPage);
  });

  it('registers the employee conversation page', () => {
    const conversationGlobalApi = {
      search: () => Promise.resolve({ list: [], total: 0, page: 1, pageSize: 20 }),
      detail: () => Promise.reject(new Error('not loaded')),
      employees: () => Promise.resolve([]),
    };
    const pages = createBenchmarkP0Pages({
      dashboardOverviewApi: {
        load: () => Promise.resolve(emptyOverview),
        exportCsv: () => Promise.resolve(new Blob()),
      },
      conversationGlobalApi,
    });

    expect((pages['/chat/v2-staff'] as { type?: unknown }).type).toBe(EmployeeConversationPage);
  });

  it('registers file audio on the Phase 3 Final native page with a real storage API', () => {
    const pages = createBenchmarkP0Pages({
      dashboardOverviewApi: { load: () => Promise.resolve(emptyOverview), exportCsv: () => Promise.resolve(new Blob()) },
      conversationGlobalApi: { search: () => Promise.resolve({ list: [], total: 0, page: 1, pageSize: 20 }), detail: () => Promise.reject(new Error('not loaded')) },
      businessWorkbenchApi: { read: () => Promise.resolve({ list: [] }), write: () => Promise.resolve() },
      fileAudioApi: { list: () => Promise.resolve({ list: [], total: 0, page: 1, perPage: 20 }) },
    });
    expect((pages['/chat/file-audio'] as { type?: unknown }).type).toBe(FileAudioPage);
  });

  it('registers all four conversation operation pages with their typed APIs', () => {
    const pages = createBenchmarkP0Pages({
      dashboardOverviewApi: { load: () => Promise.resolve(emptyOverview), exportCsv: () => Promise.resolve(new Blob()) },
      conversationGlobalApi: {} as never,
      fileAudioApi: {} as never,
      refuseArchiveApi: {} as never,
      contactTransferApi: {} as never,
    });
    expect((pages['/chat/file-audio'] as { type?: unknown }).type).toBe(FileAudioPage);
    expect((pages['/chat/resign-staff'] as { type?: unknown }).type).toBe(ResignedEmployeePage);
    expect((pages['/chat/refuse-archive'] as { type?: unknown }).type).toBe(RefuseArchivePage);
    expect((pages['/customer/inheritance'] as { type?: unknown }).type).toBe(CustomerInheritancePage);
  });

  it('registers the customer workspace page and keeps the room scope on the shared page', () => {
    const conversationGlobalApi = {
      search: () => Promise.resolve({ list: [], total: 0, page: 1, pageSize: 20 }),
      detail: () => Promise.reject(new Error('not loaded')),
    };
    const pages = createBenchmarkP0Pages({
      dashboardOverviewApi: {
        load: () => Promise.resolve(emptyOverview),
        exportCsv: () => Promise.resolve(new Blob()),
      },
      conversationGlobalApi,
    });

    expect((pages['/chat/v2-customer'] as { type?: unknown }).type).toBe(CustomerConversationPage);
    expect((pages['/chat/v2-group'] as { type?: unknown }).type).toBe(GroupConversationPage);
  });

  it('registers the conversation trajectory page', () => {
    const pages = createBenchmarkP0Pages({
      dashboardOverviewApi: {
        load: () => Promise.resolve(emptyOverview),
        exportCsv: () => Promise.resolve(new Blob()),
      },
      conversationGlobalApi: {
        search: () => Promise.resolve({ list: [], total: 0, page: 1, pageSize: 20 }),
        detail: () => Promise.reject(new Error('not loaded')),
      },
    });

    expect((pages['/chat/trajectory'] as { type?: unknown }).type).toBe(ConversationTrajectoryPage);
  });

  it('registers the conversation export page', () => {
    const pages = createBenchmarkP0Pages({
      dashboardOverviewApi: {
        load: () => Promise.resolve(emptyOverview),
        exportCsv: () => Promise.resolve(new Blob()),
      },
      conversationGlobalApi: {
        search: () => Promise.resolve({ list: [], total: 0, page: 1, pageSize: 20 }),
        detail: () => Promise.reject(new Error('not loaded')),
      },
    });

    expect((pages['/chat/export'] as { type?: unknown }).type).toBe(ConversationExportPage);
  });

  it('registers all six risk warning routes as native pages when the workbench API is available', () => {
    const pages = createBenchmarkP0Pages({
      dashboardOverviewApi: {
        load: () => Promise.resolve(emptyOverview),
        exportCsv: () => Promise.resolve(new Blob()),
      },
      conversationGlobalApi: {
        search: () => Promise.resolve({ list: [], total: 0, page: 1, pageSize: 20 }),
        detail: () => Promise.reject(new Error('not loaded')),
        employees: () => Promise.resolve([]),
      },
      riskBehaviorApi: {} as never,
      businessWorkbenchApi: { read: () => Promise.resolve({ list: [] }), write: () => Promise.resolve() },
    });

    for (const path of [
      '/ai-insight/v2/risk',
      '/ai-insight/v2/timeout',
      '/ai-insight/v2/customer-loss',
      '/ai-insight/v2/message-intercept',
      '/ai-insight/v2/keyword-library',
      '/ai-insight/v2/silent-customer',
    ]) {
      const expected = path === '/ai-insight/v2/risk' ? RiskBehaviorPage
        : path === '/ai-insight/v2/timeout' ? TimeoutWarningPage
        : path === '/ai-insight/v2/message-intercept' ? MessageInterceptPage
        : path === '/ai-insight/v2/keyword-library' ? KeywordLibraryPage
        : path === '/ai-insight/v2/silent-customer' ? SilentCustomerPage
        : path === '/ai-insight/v2/customer-loss' ? CustomerLossPage
        : RiskWarningPage;
      expect((pages[path] as { type?: unknown }).type).toBe(expected);
    }
  });

  it('registers all three Phase 3.4 batch-one acquisition routes when the workbench API is available', () => {
    const pages = createBenchmarkP0Pages({
      dashboardOverviewApi: { load: () => Promise.resolve(emptyOverview), exportCsv: () => Promise.resolve(new Blob()) },
      conversationGlobalApi: { search: () => Promise.resolve({ list: [], total: 0, page: 1, pageSize: 20 }), detail: () => Promise.reject(new Error('not loaded')) },
      businessWorkbenchApi: { read: () => Promise.resolve({ list: [] }), write: () => Promise.resolve() },
    });

    expect((pages['/acquisition/v2-channel-code'] as { type?: unknown }).type).toBe(ChannelCodePage);
    expect((pages['/acquisition/group-code'] as { type?: unknown }).type).toBe(GroupCodePage);
    expect((pages['/acquisition/live-code-short-chain'] as { type?: unknown }).type).toBe(LiveCodeShortChainPage);
  });

  it('registers all three Phase 3.4 batch-two conversion routes when the workbench API is available', () => {
    const pages = createBenchmarkP0Pages({
      dashboardOverviewApi: { load: () => Promise.resolve(emptyOverview), exportCsv: () => Promise.resolve(new Blob()) },
      conversationGlobalApi: { search: () => Promise.resolve({ list: [], total: 0, page: 1, pageSize: 20 }), detail: () => Promise.reject(new Error('not loaded')) },
      businessWorkbenchApi: { read: () => Promise.resolve({ list: [] }), write: () => Promise.resolve() },
    });

    expect((pages['/acquisition/redirect-link'] as { type?: unknown }).type).toBe(RedirectLinkPage);
    expect((pages['/acquisition/wechat-customer-service'] as { type?: unknown }).type).toBe(WechatCustomerServicePage);
    expect((pages['/acquisition/group-template'] as { type?: unknown }).type).toBe(GroupTemplatePage);
    expect((pages['/acquisition/precise-group-send'] as { type?: unknown }).type).toBe(PreciseGroupSendPage);
    expect((pages['/acquisition/friends-circle'] as { type?: unknown }).type).toBe(FriendsCirclePage);
    expect((pages['/acquisition/material-management'] as { type?: unknown }).type).toBe(MaterialManagementPage);
  });

  it('prefers P0 implementations over P1 implementations at the same path', () => {
    const pages = createPageRegistry({
      manifest,
      p0Pages: { '/index': <p>P0 overview</p> },
      p1Pages: { '/index': <p>P1 overview</p>, '/chat/staff': <p>Staff chat</p> },
    });

    render(<>{pages['/index']}</>);
    expect(screen.getByText('P0 overview')).toBeTruthy();
    expect(screen.queryByText('P1 overview')).toBeNull();
  });

  it('uses a construction placeholder for an unimplemented P2 page', () => {
    const pages = createPageRegistry({ manifest, p0Pages: {}, p1Pages: {} });

    render(<MemoryRouter>{pages['/chat/export']}</MemoryRouter>);
    expect(screen.getByRole('heading', { name: '会话导出' })).toBeTruthy();
    expect(screen.getByText('功能建设中')).toBeTruthy();
  });

  it('returns an element for every manifest path', () => {
    const pages = createPageRegistry({ manifest, p0Pages: {}, p1Pages: {} });

    expect(Object.keys(pages).sort()).toEqual(
      manifest.pages.map((page) => page.path).sort(),
    );
  });

  it('keeps the registry constructible for manifest-only consumers', () => {
    const completedManifest = {
      ...manifest,
      pages: manifest.pages.map((page) => page.path === '/index'
        ? {
          ...page,
          implementation: 'native',
          backend: 'ready',
          acceptance: 'e2e-passed',
        }
        : page),
    } as unknown as BenchmarkManifest;

    expect(() => createPageRegistry({ manifest: completedManifest, p0Pages: {}, p1Pages: {} })).not.toThrow();
  });

  it.each([
    ['/chat/v2-staff', '员工会话'],
    ['/chat/v2-customer', '客户会话'],
    ['/chat/v2-group', '群聊会话'],
    ['/ai-insight/session-analysis', '会话分析'],
    ['/customer/group', '客户群'],
  ])('registers and renders the documented P1 demo route %s', (path, title) => {
    const pages = createPageRegistry({ manifest: benchmarkManifest, p0Pages: {}, p1Pages: {} });

    render(<MemoryRouter>{pages[path]}</MemoryRouter>);
    expect(screen.getByRole('heading', { name: title })).toBeTruthy();
    expect(screen.getByText('未观测交互，仅演示')).toBeTruthy();
  });

  it('renders a manifest P2 route instead of the 404 page', async () => {
    renderDashboardRoute('/chat/trajectory');

    expect(await screen.findByRole('heading', { name: '会话轨迹' })).toBeTruthy();
    expect(screen.queryByRole('heading', { name: '页面不存在' })).toBeNull();
  });

  it('keeps unknown routes on the NotFound page', async () => {
    renderDashboardRoute('/not-registered');

    expect(await screen.findByRole('heading', { name: '页面不存在' })).toBeTruthy();
  });
});
