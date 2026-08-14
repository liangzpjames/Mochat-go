import {
  MobileBottomNavigation,
  MobileShell,
  type MobileBottomNavigationItem,
} from '@mochat/mobile-foundation';
import type { ReactNode } from 'react';
import { useLocation } from 'react-router';

type SidebarPageShellProps = {
  title: string;
  subtitle?: string;
  eyebrow?: string;
  hero?: ReactNode;
  children: ReactNode;
};

type SidebarNavigationKey = 'customers' | 'conversations' | 'profile';

function navigationKey(pathname: string): SidebarNavigationKey {
  if (pathname === '/contactSop') return 'conversations';
  if (pathname === '/' || pathname === '') return 'profile';
  return 'customers';
}

export function sidebarBusinessContextSuffix(search: string, hash: string): string {
  const params = new URLSearchParams(search);
  params.delete('state');
  const serialized = params.toString();
  return `${serialized.length === 0 ? '' : `?${serialized}`}${hash}`;
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
  const suffix = sidebarBusinessContextSuffix(location.search, location.hash);
  const current = navigationKey(location.pathname);
  const items: MobileBottomNavigationItem[] = [
    { key: 'customers', label: '客户', icon: <SidebarLineIcon kind="customers" />, href: `/contact${suffix}`, current: current === 'customers' },
    { key: 'conversations', label: '会话', icon: <SidebarLineIcon kind="conversations" />, href: `/contactSop${suffix}`, current: current === 'conversations' },
    { key: 'profile', label: '我的', icon: <SidebarLineIcon kind="profile" />, href: `/${suffix}`, current: current === 'profile' },
  ];

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
