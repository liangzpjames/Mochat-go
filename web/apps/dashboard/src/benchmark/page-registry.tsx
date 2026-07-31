import type { ReactNode } from 'react';

import type { BenchmarkManifest } from './benchmark-manifest';
import type { DashboardOverviewApi } from '../features/dashboard-overview/dashboard-overview-api';
import { DashboardOverviewPage } from '../features/dashboard-overview/dashboard-overview-page';
import type { ConversationGlobalApi } from '../features/conversation-global/conversation-global-api';
import { ConversationGlobalPage } from '../features/conversation-global/conversation-global-page';
import {
  channelCodeDemo,
  contactDemo,
  customerConversationDemo,
  customerGroupDemo,
  groupConversationDemo,
  riskBehaviorDemo,
  sessionAnalysisDemo,
  staffConversationDemo,
  timeoutWarningDemo,
} from './demo-fixtures';
import { DemoPage } from './demo-page';
import { PlaceholderPage } from './placeholder-page';

export type PageRegistry = Readonly<Record<string, ReactNode>>;

export function createBenchmarkP0Pages({
  dashboardOverviewApi,
  conversationGlobalApi,
}: {
  dashboardOverviewApi: DashboardOverviewApi;
  conversationGlobalApi: ConversationGlobalApi;
}): PageRegistry {
  return {
    '/index': <DashboardOverviewPage api={dashboardOverviewApi} />,
    '/chat/v2-all': <ConversationGlobalPage api={conversationGlobalApi} />,
  };
}

const benchmarkP1Pages: PageRegistry = {
  '/chat/v2-staff': <DemoPage config={staffConversationDemo} />,
  '/chat/v2-customer': <DemoPage config={customerConversationDemo} />,
  '/chat/v2-group': <DemoPage config={groupConversationDemo} />,
  '/ai-insight/v2/risk': <DemoPage config={riskBehaviorDemo} />,
  '/ai-insight/v2/timeout': <DemoPage config={timeoutWarningDemo} />,
  '/ai-insight/session-analysis': <DemoPage config={sessionAnalysisDemo} />,
  '/acquisition/v2-channel-code': <DemoPage config={channelCodeDemo} />,
  '/customer/contact': <DemoPage config={contactDemo} />,
  '/customer/group': <DemoPage config={customerGroupDemo} />,
};

export function createPageRegistry({
  manifest,
  p0Pages,
  p1Pages,
}: {
  manifest: BenchmarkManifest;
  p0Pages: PageRegistry;
  p1Pages: PageRegistry;
}): PageRegistry {
  const groupTitles = new Map(manifest.groups.map((group) => [group.id, group.title]));
  const pages = Object.fromEntries(manifest.pages.map((page) => [
    page.path,
    p0Pages[page.path]
      ?? p1Pages[page.path]
      ?? benchmarkP1Pages[page.path]
      ?? <PlaceholderPage
        title={page.title}
        {...(page.groupId === null
          ? {}
          : { groupTitle: groupTitles.get(page.groupId) ?? '工作台' })}
      />,
  ]));

  for (const page of manifest.pages) {
    const isCompleted = page.backend === 'ready' && page.acceptance === 'e2e-passed';
    const hasInjectedPage = p0Pages[page.path] !== undefined || p1Pages[page.path] !== undefined;
    if (isCompleted && !hasInjectedPage) {
      throw new Error(`completed page requires an injected real implementation: ${page.path}`);
    }
  }

  return pages;
}
