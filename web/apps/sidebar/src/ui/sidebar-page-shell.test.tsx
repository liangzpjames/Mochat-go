import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it } from 'vitest';

import { SidebarPageShell } from './sidebar-page-shell';

afterEach(cleanup);

describe('SidebarPageShell', () => {
  it('keeps the customer and resolved SOP context across customer, conversation and profile tabs', () => {
    render(
      <MemoryRouter initialEntries={['/contact?wxExternalUserid=external-1&contactId=23&agentId=7#profile']}>
        <SidebarPageShell title="客户资料">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    const customer = screen.getByRole('link', { name: '客户' });
    expect(screen.getByRole('navigation', { name: '员工工作台' })).not.toBeNull();
    expect(customer.getAttribute('href')).toBe('/contact?agentId=7&wxExternalUserid=external-1&contactId=23');
    expect(screen.getByRole('link', { name: '会话' }).getAttribute('href')).toBe('/contactSop?agentId=7&wxExternalUserid=external-1&contactId=23');
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe('/?agentId=7&wxExternalUserid=external-1&contactId=23');
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

  it('keeps batch context on profile without creating customer or conversation fake links', () => {
    render(
      <MemoryRouter initialEntries={['/contactBatchAdd?batchId=9&id=5&agentId=7#task']}>
        <SidebarPageShell title="批量加好友">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.queryByRole('link', { name: '客户' })).toBeNull();
    expect(screen.queryByRole('link', { name: '会话' })).toBeNull();
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe('/?agentId=7&batchId=9');
  });

  it.each([
    ['/contactSop?id=4&agentId=7', '/?agentId=7&contactSopId=4', '/contactSop?agentId=7&id=4'],
    ['/roomSop?id=5&agentId=7', '/?agentId=7&roomSopId=5', '/roomSop?agentId=7&id=5'],
    ['/?agentId=7&roomSopId=5', '/?agentId=7&roomSopId=5', '/roomSop?agentId=7&id=5'],
  ])('round-trips task context from %s through profile', (source, profileHref, conversationHref) => {
    render(
      <MemoryRouter initialEntries={[source]}>
        <SidebarPageShell title="员工工作台">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe(profileHref);
    expect(screen.getByRole('link', { name: '会话' }).getAttribute('href')).toBe(conversationHref);
    cleanup();
  });

  it('does not expose customer or conversation bottom links without business context', () => {
    render(
      <MemoryRouter initialEntries={['/?agentId=7']}>
        <SidebarPageShell title="员工工作台">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.queryByRole('link', { name: '客户' })).toBeNull();
    expect(screen.queryByRole('link', { name: '会话' })).toBeNull();
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe('/?agentId=7');
  });

  it('keeps bottom navigation inside the prefixed Sidebar mount', () => {
    render(
      <MemoryRouter
        basename="/sidebar-app"
        initialEntries={['/sidebar-app/contact?wxExternalUserid=external-1&contactId=23']}
      >
        <SidebarPageShell title="客户资料">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: '客户' }).getAttribute('href')).toBe(
      '/sidebar-app/contact?wxExternalUserid=external-1&contactId=23',
    );
    expect(screen.getByRole('link', { name: '会话' }).getAttribute('href')).toBe(
      '/sidebar-app/contactSop?wxExternalUserid=external-1&contactId=23',
    );
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe(
      '/sidebar-app/?wxExternalUserid=external-1&contactId=23',
    );
  });

  it.each([
    ['/contactSop?id=4', '会话'],
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
