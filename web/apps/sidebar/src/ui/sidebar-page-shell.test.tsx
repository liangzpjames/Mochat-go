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
    expect(customer.getAttribute('href')).toBe('/?agentId=7&wxExternalUserid=external-1&contactId=23&tab=customers');
    expect(screen.getByRole('link', { name: '会话' }).getAttribute('href')).toBe('/?agentId=7&wxExternalUserid=external-1&contactId=23&tab=conversations');
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe('/?agentId=7&wxExternalUserid=external-1&contactId=23&tab=profile');
    expect(customer.getAttribute('aria-current')).toBe('page');
  });

  it('does not carry an authentication callback state into business navigation', () => {
    render(
      <MemoryRouter initialEntries={['/contact?wxExternalUserid=external-1&state=callback-state#profile']}>
        <SidebarPageShell title="客户资料">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: '客户' }).getAttribute('href')).toBe(
      '/?wxExternalUserid=external-1&tab=customers',
    );
  });

  it('keeps batch context while routing all three workspace tabs to safe root views', () => {
    render(
      <MemoryRouter initialEntries={['/contactBatchAdd?batchId=9&id=5&agentId=7#task']}>
        <SidebarPageShell title="批量加好友">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: '客户' }).getAttribute('href')).toBe('/?agentId=7&batchId=9&tab=customers');
    expect(screen.getByRole('link', { name: '会话' }).getAttribute('href')).toBe('/?agentId=7&batchId=9&tab=conversations');
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe('/?agentId=7&batchId=9&tab=profile');
  });

  it.each([
    ['/contactSop?id=4&agentId=7', '/?agentId=7&contactSopId=4&tab=profile', '/?agentId=7&contactSopId=4&tab=conversations'],
    ['/roomSop?id=5&agentId=7', '/?agentId=7&roomSopId=5&tab=profile', '/?agentId=7&roomSopId=5&tab=conversations'],
    ['/?agentId=7&roomSopId=5', '/?agentId=7&roomSopId=5&tab=profile', '/?agentId=7&roomSopId=5&tab=conversations'],
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

  it('always exposes all three safe workspace links without customer context', () => {
    render(
      <MemoryRouter initialEntries={['/?agentId=7']}>
        <SidebarPageShell title="员工工作台">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: '客户' }).getAttribute('href')).toBe('/?agentId=7&tab=customers');
    expect(screen.getByRole('link', { name: '会话' }).getAttribute('href')).toBe('/?agentId=7&tab=conversations');
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe('/?agentId=7&tab=profile');
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
      '/sidebar-app/?wxExternalUserid=external-1&contactId=23&tab=customers',
    );
    expect(screen.getByRole('link', { name: '会话' }).getAttribute('href')).toBe(
      '/sidebar-app/?wxExternalUserid=external-1&contactId=23&tab=conversations',
    );
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('href')).toBe(
      '/sidebar-app/?wxExternalUserid=external-1&contactId=23&tab=profile',
    );
  });

  it.each([
    ['/contactSop?id=4', '会话'],
    ['/roomSop?id=5', '会话'],
    ['/contactBatchAdd?batchId=9', '客户'],
    ['/medium?agentId=7', '客户'],
    ['/', '客户'],
    ['/?tab=conversations', '会话'],
    ['/?tab=profile', '我的'],
  ])('marks %s as the current employee workspace section', (path, currentLabel) => {
    render(
      <MemoryRouter initialEntries={[path]}>
        <SidebarPageShell title="员工工作台">内容</SidebarPageShell>
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: currentLabel }).getAttribute('aria-current')).toBe('page');
  });
});
