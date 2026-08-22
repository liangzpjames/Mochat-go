import {
  MobileBottomNavigation,
  MobileShell,
  type MobileBottomNavigationItem,
} from '@mochat/mobile-foundation';
import type { ReactNode } from 'react';
import { useHref, useLocation } from 'react-router';

type SidebarPageShellProps = {
  title: string;
  subtitle?: string;
  eyebrow?: string;
  hero?: ReactNode;
  children: ReactNode;
};

type SidebarNavigationKey = 'customers' | 'conversations' | 'profile';

export type SidebarBusinessNavigation = {
  available: boolean;
  reason?: string;
  suffix: string;
};

function navigationKey(pathname: string): SidebarNavigationKey {
  if (pathname === '/contactSop' || pathname === '/roomSop') return 'conversations';
  if (pathname === '/' || pathname === '') return 'profile';
  return 'customers';
}

function positiveParam(params: URLSearchParams, name: string): string | null {
  const value = params.get(name)?.trim() ?? '';
  return /^[1-9]\d*$/.test(value) ? value : null;
}

export function sidebarBusinessNavigation(
  targetPath: string,
  sourcePath: string,
  search: string,
): SidebarBusinessNavigation {
  const source = new URLSearchParams(search);
  const context = new URLSearchParams();
  const copy = (name: string) => {
    const value = source.get(name)?.trim();
    if (value) context.set(name, value);
  };
  copy('agentId');
  copy('wxExternalUserid');
  for (const name of ['contactId', 'contactSopId', 'roomSopId', 'batchId']) {
    const value = positiveParam(source, name);
    if (value !== null) context.set(name, value);
  }
  const sourceTaskId = positiveParam(source, 'id');
  if (sourceTaskId !== null && sourcePath === '/contactSop') context.set('contactSopId', sourceTaskId);
  if (sourceTaskId !== null && sourcePath === '/roomSop') context.set('roomSopId', sourceTaskId);

  const customerRoute = targetPath === '/contact'
    || targetPath === '/contact/editDetail'
    || targetPath === '/contact/remark'
    || targetPath === '/contact/settingTag';
  let available = true;
  let reason: string | undefined;
  if (customerRoute && !context.has('wxExternalUserid')) {
    available = false;
    reason = '需从企业微信客户会话进入';
  } else if (targetPath === '/contactSop' && !context.has('contactSopId') && !context.has('contactId')) {
    available = false;
    reason = '需从个人客户 SOP 任务或客户详情进入';
  } else if (targetPath === '/roomSop' && !context.has('roomSopId')) {
    available = false;
    reason = '需从客户群 SOP 任务进入';
  } else if (targetPath === '/contactBatchAdd' && !context.has('batchId')) {
    available = false;
    reason = '需从批量加好友任务进入';
  }

  const target = new URLSearchParams(context);
  if (targetPath === '/contactSop' && target.has('contactSopId')) {
    target.set('id', target.get('contactSopId') ?? '');
    target.delete('contactSopId');
  }
  if (targetPath === '/roomSop' && target.has('roomSopId')) {
    target.set('id', target.get('roomSopId') ?? '');
    target.delete('roomSopId');
  }
  const serialized = target.toString();
  return {
    available,
    ...(reason === undefined ? {} : { reason }),
    suffix: serialized.length === 0 ? '' : `?${serialized}`,
  };
}

export function sidebarBusinessContextSuffix(targetPath: string, search: string, sourcePath = ''): string {
  return sidebarBusinessNavigation(targetPath, sourcePath, search).suffix;
}

function SidebarLineIcon({ kind }: { kind: SidebarNavigationKey }) {
  if (kind === 'customers') {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24">
        <circle cx="12" cy="8" fill="none" r="3.25" stroke="currentColor" strokeWidth="1.8" />
        <path d="M5.5 19c.8-3.25 3.05-4.9 6.5-4.9s5.7 1.65 6.5 4.9" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      </svg>
    );
  }
  if (kind === 'conversations') {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24">
        <path d="M5 6.5A3.5 3.5 0 0 1 8.5 3h7A3.5 3.5 0 0 1 19 6.5v4A3.5 3.5 0 0 1 15.5 14H12l-3.75 3v-3H8.5A3.5 3.5 0 0 1 5 10.5Z" fill="none" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.8" />
        <path d="M9 8.5h6" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      </svg>
    );
  }
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <circle cx="12" cy="8" fill="none" r="3.25" stroke="currentColor" strokeWidth="1.8" />
      <path d="M6 20c.8-3.6 2.8-5.4 6-5.4s5.2 1.8 6 5.4" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
    </svg>
  );
}

export function SidebarPageShell({
  title,
  subtitle,
  eyebrow,
  hero,
  children,
}: SidebarPageShellProps) {
  const location = useLocation();
  const current = navigationKey(location.pathname);
  const customers = sidebarBusinessNavigation('/contact', location.pathname, location.search);
  const contactConversation = sidebarBusinessNavigation('/contactSop', location.pathname, location.search);
  const roomConversation = sidebarBusinessNavigation('/roomSop', location.pathname, location.search);
  const conversationPath = location.pathname === '/roomSop' || (!contactConversation.available && roomConversation.available)
    ? '/roomSop'
    : '/contactSop';
  const conversation = conversationPath === '/roomSop' ? roomConversation : contactConversation;
  const profile = sidebarBusinessNavigation('/', location.pathname, location.search);
  const customersHref = useHref(`/contact${customers.suffix}`);
  const conversationsHref = useHref(`${conversationPath}${conversation.suffix}`);
  const profileRootHref = useHref('/');
  const profileHref = profile.suffix.length === 0
    ? profileRootHref
    : `${profileRootHref.endsWith('/') ? profileRootHref : `${profileRootHref}/`}${profile.suffix}`;
  const items: MobileBottomNavigationItem[] = [];
  if (customers.available) items.push({ key: 'customers', label: '客户', icon: <SidebarLineIcon kind="customers" />, href: customersHref, current: current === 'customers' });
  if (conversation.available) items.push({ key: 'conversations', label: '会话', icon: <SidebarLineIcon kind="conversations" />, href: conversationsHref, current: current === 'conversations' });
  items.push({ key: 'profile', label: '我的', icon: <SidebarLineIcon kind="profile" />, href: profileHref, current: current === 'profile' });

  return (
    <MobileShell
      appName="MoChat 客户侧边栏"
      bottomNavigation={<MobileBottomNavigation label="员工工作台" items={items} />}
      title={title}
      {...(eyebrow === undefined ? {} : { eyebrow })}
      {...(hero === undefined ? {} : { hero })}
      {...(subtitle === undefined ? {} : { subtitle })}
    >
      {children}
    </MobileShell>
  );
}
