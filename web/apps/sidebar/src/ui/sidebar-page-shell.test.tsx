import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it } from 'vitest';

import { SidebarPageShell } from './sidebar-page-shell';

afterEach(cleanup);

describe('SidebarPageShell', () => {
  it('keeps only the current customer context when navigating to the customer tab', () => {
    render(
      <MemoryRouter initialEntries={['/contact?wxExternalUserid=external-1#profile']}>
        <SidebarPageShell title="客户资料">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    const customer = screen.getByRole('link', { name: '客户' });
    expect(screen.getByRole('navigation', { name: '员工工作台' })).not.toBeNull();
    expect(customer.getAttribute('href')).toBe('/contact?wxExternalUserid=external-1');
    expect(customer.getAttribute('aria-current')).toBe('page');
  });

  it('does not carry an authentication callback state into business navigation', () => {
    render(
      <MemoryRouter initialEntries={['/contact?wxExternalUserid=external-1&state=callback-state#profile']}>
        <SidebarPageShell title="客户资料">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: '客户' }).getAttribute('href')).toBe(
      '/contact?wxExternalUserid=external-1',
    );
  });

  it('does not leak a batch or room task id across bottom-navigation domains', () => {
    render(
      <MemoryRouter initialEntries={['/contactBatchAdd?batchId=9&id=5&agentId=7#task']}>
        <SidebarPageShell title="批量加好友">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: '客户' }).getAttribute('href')).toBe('/contact?agentId=7');
    expect(screen.getByRole('link', { name: '会话' }).getAttribute('href')).toBe('/contactSop?agentId=7');
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe('/?agentId=7');
  });

  it('keeps bottom navigation inside the prefixed Sidebar mount', () => {
    render(
      <MemoryRouter
        basename="/sidebar-app"
        initialEntries={['/sidebar-app/contact?wxExternalUserid=external-1']}
      >
        <SidebarPageShell title="客户资料">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: '客户' }).getAttribute('href')).toBe(
      '/sidebar-app/contact?wxExternalUserid=external-1',
    );
    expect(screen.getByRole('link', { name: '会话' }).getAttribute('href')).toBe(
      '/sidebar-app/contactSop?wxExternalUserid=external-1',
    );
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe(
      '/sidebar-app',
    );
  });

  it.each([
    ['/contactSop', '会话'],
    ['/roomSop?id=5', '会话'],
    ['/', '我的'],
  ])('marks %s as the current employee workspace section', (path, currentLabel) => {
    render(
      <MemoryRouter initialEntries={[path]}>
        <SidebarPageShell title="员工工作台">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: currentLabel }).getAttribute('aria-current')).toBe('page');
  });
});
