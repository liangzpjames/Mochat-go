import type { ReactNode } from 'react';

import type { BenchmarkManifest } from './benchmark-manifest';
import type { DashboardOverviewApi } from '../features/dashboard-overview/dashboard-overview-api';
import { DashboardOverviewPage } from '../features/dashboard-overview/dashboard-overview-page';
import type { ConversationGlobalApi } from '../features/conversation-global/conversation-global-api';
import { ConversationGlobalPage } from '../features/conversation-global/conversation-global-page';
import { EmployeeConversationPage } from '../features/conversation-global/employee-conversation-page';
import { ConversationTrajectoryPage } from '../features/conversation-global/conversation-trajectory-page';
import { ConversationExportPage } from '../features/conversation-global/conversation-export-page';
import {
  channelCodeDemo,
  customerConversationDemo,
  customerGroupDemo,
  groupConversationDemo,
  sessionAnalysisDemo,
  staffConversationDemo,
} from './demo-fixtures';
import { DemoPage } from './demo-page';
import { PlaceholderPage } from './placeholder-page';
import type { SensitiveWordApi } from '../features/sensitive-word/sensitive-word-api';
import { SensitiveWordPage } from '../features/sensitive-word/sensitive-word-page';
import type { LeadApi } from '../features/scrm/lead-api';
import { LeadPage } from '../features/scrm/lead-page';
import type { ScrmApi } from '../features/scrm/scrm-api';
import { PublicPoolPage } from '../features/scrm/public-pool-page';
import { OpportunityPage } from '../features/scrm/opportunity-page';
import { TagPage, type CustomerTagApi } from '../features/scrm/tag-page';
import type { ContactApi } from '../features/scrm/contact-api';
import { ContactPage } from '../features/scrm/contact-page';
import { Phase33OperationsPage, phase33OperationConfigs } from '../features/phase33/phase33-operations-page';
import { RiskWarningPage, riskWarningConfigs } from '../features/phase33/risk-warning-pages';
import { CustomerLossPage } from '../features/phase33/customer-loss-page';
import { CustomerTransferPage } from '../features/phase33/customer-transfer-page';
import { RiskBehaviorPage } from '../features/phase33/risk-behavior-page';
import { TimeoutWarningPage } from '../features/phase33/timeout-warning-page';
import type { BusinessWorkbenchApi } from '../features/business-workbench/business-workbench-page';

export type PageRegistry = Readonly<Record<string, ReactNode>>;

export function createBenchmarkP0Pages({
  dashboardOverviewApi,
  conversationGlobalApi,
  sensitiveWordApi,
  leadApi,
  scrmApi,
  contactApi,
  businessWorkbenchApi,
}: {
  dashboardOverviewApi: DashboardOverviewApi;
  conversationGlobalApi: ConversationGlobalApi;
  sensitiveWordApi?: SensitiveWordApi;
  leadApi?: LeadApi;
  scrmApi?: ScrmApi;
  contactApi?: ContactApi;
  businessWorkbenchApi?: BusinessWorkbenchApi;
}): PageRegistry {
  return {
    '/index': <DashboardOverviewPage api={dashboardOverviewApi} />,
    '/chat/v2-all': <ConversationGlobalPage api={conversationGlobalApi} />,
    '/chat/v2-staff': <EmployeeConversationPage api={conversationGlobalApi} />,
    '/chat/v2-customer': <ConversationGlobalPage api={conversationGlobalApi} fixedConversationType="customer" />,
    '/chat/v2-group': <ConversationGlobalPage api={conversationGlobalApi} fixedConversationType="room" />,
    '/chat/trajectory': <ConversationTrajectoryPage api={conversationGlobalApi} />,
    '/chat/export': <ConversationExportPage api={conversationGlobalApi} />,
    ...(sensitiveWordApi === undefined ? {} : { '/ai-insight/v2/sensitive-word': <SensitiveWordPage api={sensitiveWordApi} /> }),
    ...(leadApi === undefined ? {} : { '/customer/clue/default': <LeadPage api={leadApi} /> }),
    ...(scrmApi === undefined ? {} : {
      '/customer/public-sea': <PublicPoolPage api={scrmApi} />,
      '/customer/opportunity': <OpportunityPage api={scrmApi} />,
      '/customer/tags': <TagPage api={scrmApi as CustomerTagApi} />,
    }),
    ...(contactApi === undefined ? {} : { '/customer/contact': <ContactPage api={contactApi} /> }),
    ...(businessWorkbenchApi === undefined ? {} : Object.fromEntries(
      Object.entries(phase33OperationConfigs).map(([path, config]) => [
        path,
        <Phase33OperationsPage key={path} api={businessWorkbenchApi} config={config} />,
      ]),
    )),
    ...(businessWorkbenchApi === undefined ? {} : { '/ai-insight/v2/customer-loss': <CustomerLossPage api={businessWorkbenchApi} /> }),
    ...(businessWorkbenchApi === undefined ? {} : {
      '/customer/inheritance': <CustomerTransferPage api={businessWorkbenchApi} mode="inheritance" />,
      '/chat/resign-staff': <CustomerTransferPage api={businessWorkbenchApi} mode="resign" />,
    }),
    ...(businessWorkbenchApi === undefined ? {} : Object.fromEntries(
      Object.entries(riskWarningConfigs).map(([path, config]) => [
        path,
        <RiskWarningPage key={path} api={businessWorkbenchApi} config={config} />,
      ]),
    )),
    ...(businessWorkbenchApi === undefined ? {} : { '/ai-insight/v2/risk': <RiskBehaviorPage api={businessWorkbenchApi} /> }),
    ...(businessWorkbenchApi === undefined ? {} : { '/ai-insight/v2/timeout': <TimeoutWarningPage api={businessWorkbenchApi} /> }),
  };
}

const benchmarkP1Pages: PageRegistry = {
  '/chat/v2-staff': <DemoPage config={staffConversationDemo} />,
  '/chat/v2-customer': <DemoPage config={customerConversationDemo} />,
  '/chat/v2-group': <DemoPage config={groupConversationDemo} />,
  '/ai-insight/session-analysis': <DemoPage config={sessionAnalysisDemo} />,
  '/acquisition/v2-channel-code': <DemoPage config={channelCodeDemo} />,
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
    void isCompleted;
    void hasInjectedPage;
  }

  return pages;
}
