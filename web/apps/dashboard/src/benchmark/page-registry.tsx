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
import { KeywordLibraryPage, MessageInterceptPage } from '../features/phase33/message-intercept-pages';
import { RefuseArchivePage, SilentCustomerPage } from '../features/phase33/phase33-closure-pages';
import type { BusinessWorkbenchApi } from '../features/business-workbench/business-workbench-page';
import { ChannelCodePage, GroupCodePage, LiveCodeShortChainPage } from '../features/phase34/acquisition-pages';
import { GroupTemplatePage, RedirectLinkPage, WechatCustomerServicePage } from '../features/phase34/conversion-pages';
import { FriendsCirclePage, PreciseGroupSendPage } from '../features/phase34/content-reach-pages';
import { MaterialManagementPage } from '../features/phase34/material-management/material-management-page';
import { FriendsPage } from '../features/phase35/friends-page';
import { GroupPage } from '../features/phase35/group-page';
import { OrderPage } from '../features/phase35/order-page';
import { SettingsPage } from '../features/phase35/settings-page';
import { CustomerReportPage } from '../features/phase35/customer-report-page';
import { ConversionReportPage } from '../features/phase35/conversion-report-page';
import { EmployeeReportPage } from '../features/phase35/employee-report-page';
import { BehaviorReportPage } from '../features/phase35/behavior-report-page';
import { ReportPage } from '../features/phase35/report-page';
import type { AISettingsApi } from '../features/ai-settings/ai-settings-api';
import { KnowledgeBasePage } from '../features/ai-settings/knowledge-base-page';
import { AgentPage } from '../features/ai-settings/agent-page';
import type { AiInsightApi } from '../features/ai-insight/ai-insight-api';
import { AiInsightPage } from '../features/ai-insight/ai-insight-pages';
import { CompanyWebsitePage } from '../features/company-settings/website-page';
import { CompanyStaffPage } from '../features/company-settings/staff-page';
import { CompanyRolePage } from '../features/company-settings/role-page';
import { CompanyAdditionalPage } from '../features/company-settings/additional-page';
import { CompanyAuthorizationPage } from '../features/company-settings/authorization-page';
import type { FileAudioApi } from '../features/phase35/file-audio-api';
import { FileAudioPage } from '../features/phase35/file-audio-page';
import { createCorpAdminApi } from '../features/corp/corp-admin-api';
import { createUserAdminApi } from '../features/user-admin/user-admin-api';
import { createRoleApi } from '../features/role/role-api';
import { createMenuAdminApi } from '../features/menu-admin/menu-admin-api';

export type PageRegistry = Readonly<Record<string, ReactNode>>;

type CorpAdminApi = ReturnType<typeof createCorpAdminApi>;
type UserAdminApi = ReturnType<typeof createUserAdminApi>;
type RoleApi = ReturnType<typeof createRoleApi>;
type MenuAdminApi = ReturnType<typeof createMenuAdminApi>;

export function createBenchmarkP0Pages({
  dashboardOverviewApi,
  conversationGlobalApi,
  sensitiveWordApi,
  leadApi,
  scrmApi,
  contactApi,
  businessWorkbenchApi,
  aiSettingsApi,
  aiInsightApi,
  fileAudioApi,
  corpAdminApi,
  userAdminApi,
  roleApi,
  menuAdminApi,
}: {
  dashboardOverviewApi: DashboardOverviewApi;
  conversationGlobalApi: ConversationGlobalApi;
  sensitiveWordApi?: SensitiveWordApi;
  leadApi?: LeadApi;
  scrmApi?: ScrmApi;
  contactApi?: ContactApi;
  businessWorkbenchApi?: BusinessWorkbenchApi;
  aiSettingsApi?: AISettingsApi;
  aiInsightApi?: AiInsightApi;
  fileAudioApi?: FileAudioApi;
  corpAdminApi?: CorpAdminApi;
  userAdminApi?: UserAdminApi;
  roleApi?: RoleApi;
  menuAdminApi?: MenuAdminApi;
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
    ...(fileAudioApi === undefined ? {} : { '/chat/file-audio': <FileAudioPage api={fileAudioApi} /> }),
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
    ...(businessWorkbenchApi === undefined ? {} : {
      '/ai-insight/v2/message-intercept': <MessageInterceptPage api={businessWorkbenchApi} />,
      '/ai-insight/v2/keyword-library': <KeywordLibraryPage api={businessWorkbenchApi} />,
      '/ai-insight/v2/silent-customer': <SilentCustomerPage api={businessWorkbenchApi} />,
      '/chat/refuse-archive': <RefuseArchivePage api={businessWorkbenchApi} />,
      '/acquisition/v2-channel-code': <ChannelCodePage api={businessWorkbenchApi} />,
      '/acquisition/group-code': <GroupCodePage api={businessWorkbenchApi} />,
      '/acquisition/live-code-short-chain': <LiveCodeShortChainPage api={businessWorkbenchApi} />,
      '/acquisition/redirect-link': <RedirectLinkPage api={businessWorkbenchApi} />,
      '/acquisition/wechat-customer-service': <WechatCustomerServicePage api={businessWorkbenchApi} />,
      '/acquisition/group-template': <GroupTemplatePage api={businessWorkbenchApi} />,
      '/acquisition/precise-group-send': <PreciseGroupSendPage api={businessWorkbenchApi} />,
      '/acquisition/friends-circle': <FriendsCirclePage api={businessWorkbenchApi} />,
      '/acquisition/material-management': <MaterialManagementPage api={businessWorkbenchApi} />,
      '/customer/friends': <FriendsPage api={businessWorkbenchApi} />,
      '/customer/group': <GroupPage api={businessWorkbenchApi} />,
      '/customer/order': <OrderPage api={businessWorkbenchApi} />,
      '/customer/settings': <SettingsPage api={businessWorkbenchApi} />,
      '/data/customer': <CustomerReportPage api={businessWorkbenchApi} />,
      '/data/conversion': <ConversionReportPage api={businessWorkbenchApi} />,
      '/data/employee': <EmployeeReportPage api={businessWorkbenchApi} />,
      '/data/behavior': <BehaviorReportPage api={businessWorkbenchApi} />,
      '/data/report': <ReportPage api={businessWorkbenchApi} />,
    }),
    ...(aiSettingsApi === undefined ? {} : {
      '/ai-setting/ai-knowledge-base': <KnowledgeBasePage api={aiSettingsApi} />,
      '/ai-setting/agent': <AgentPage api={aiSettingsApi} />,
    }),
    ...(aiInsightApi === undefined ? {} : {
      '/ai-insight/session-analysis': <AiInsightPage api={aiInsightApi} page="session-analysis" />,
      '/ai-insight/smart-analysis': <AiInsightPage api={aiInsightApi} page="smart-analysis" />,
      '/ai-insight/emotion': <AiInsightPage api={aiInsightApi} page="emotion" />,
      '/ai-insight/employee-score': <AiInsightPage api={aiInsightApi} page="employee-score" />,
      '/ai-insight/communication-keyword': <AiInsightPage api={aiInsightApi} page="communication-keyword" />,
    }),
    ...(corpAdminApi === undefined ? {} : { '/company-setting/website': <CompanyWebsitePage api={corpAdminApi} /> }),
    ...(userAdminApi === undefined ? {} : { '/company-setting/staff': <CompanyStaffPage api={userAdminApi} /> }),
    ...(roleApi === undefined ? {} : { '/setting/role': <CompanyRolePage api={roleApi} /> }),
    ...(menuAdminApi === undefined ? {} : {
      '/setting/additional': <CompanyAdditionalPage api={menuAdminApi} />,
      '/setting/authorization': <CompanyAuthorizationPage api={menuAdminApi} />,
    }),
  };
}

const benchmarkP1Pages: PageRegistry = {
  '/chat/v2-staff': <DemoPage config={staffConversationDemo} />,
  '/chat/v2-customer': <DemoPage config={customerConversationDemo} />,
  '/chat/v2-group': <DemoPage config={groupConversationDemo} />,
  '/ai-insight/session-analysis': <DemoPage config={sessionAnalysisDemo} />,
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
