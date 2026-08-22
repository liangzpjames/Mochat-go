import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { SidebarRequest } from '../../app/sidebar-router';
import type { WeComBridge } from '../../wecom/wecom-bridge';
import { WorkbenchPage } from './workbench-page';

const bridge: WeComBridge = {
  available: () => false,
  sendChatMessage: vi.fn(),
  navigateToAddCustomer: vi.fn(),
};

const summary = {
  employee: {
    id: 7,
    name: '林小满',
    avatar: null,
    departmentNames: ['客户成功部', '华东组'],
    corpName: 'MoChat 演示企业',
  },
  customers: { total: 126, addedToday: 8, taggedTotal: 93, ownedRoomTotal: 12 },
  tasks: { contactSopPending: 3, roomSopPending: 2, batchAddPending: 1 },
};

function requestFixture<T>(path: string): Promise<T> {
  if (path === '/workbench/summary') return Promise.resolve(summary as T);
  if (path.startsWith('/workContact/index?')) {
    return Promise.resolve({
      page: 1,
      perPage: 20,
      total: 1,
      totalPage: 1,
      items: [{
        id: 31,
        wxExternalUserid: 'wm-contact-31',
        name: '陈晨',
        avatar: null,
        remark: '重点客户',
        status: 1,
        addedAt: '2026-08-23 09:30:00',
        tags: ['已成交', '上海'],
      }],
    } as T);
  }
  if (path.startsWith('/workbench/tasks?')) {
    return Promise.resolve({
      page: 1,
      perPage: 20,
      total: 1,
      totalPage: 1,
      items: [{
        id: 41,
        kind: 'contactSop',
        title: '新客首日跟进',
        subjectName: '陈晨',
        scheduledAt: '2026-08-23 10:00:00',
        state: 'pending',
      }],
    } as T);
  }
  return Promise.reject(new Error(`Unexpected request: ${path}`));
}

function renderPage(path = '/?agentId=7', request?: SidebarRequest) {
  const client = request ?? vi.fn(requestFixture) as unknown as SidebarRequest;
  render(
    <MemoryRouter initialEntries={[path]}>
      <WorkbenchPage bridge={bridge} request={client} />
    </MemoryRouter>,
  );
  return client;
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('WorkbenchPage', () => {
  it('renders the customer workspace from persistent employee summary data', async () => {
    renderPage();

    expect(await screen.findByText('126')).not.toBeNull();
    expect(screen.getByText('客户总数')).not.toBeNull();
    expect(screen.getByText('今日新增')).not.toBeNull();
    expect(screen.getByRole('link', { name: /通讯录/ }).getAttribute('href')).toContain('view=contacts');
    expect(screen.getByRole('link', { name: /素材库/ }).getAttribute('href')).toBe('/medium?agentId=7');
    expect(screen.queryByText('从当前客户会话开始工作')).toBeNull();
  });

  it('renders the conversation workspace and loads a selected real task list', async () => {
    const request = renderPage('/?agentId=7&tab=conversations&view=contactSop');

    expect(await screen.findByText('新客首日跟进')).not.toBeNull();
    expect(screen.getByText('陈晨')).not.toBeNull();
    expect(screen.getByRole('link', { name: /打开任务/ }).getAttribute('href')).toContain('/contactSop?');
    expect(request).toHaveBeenCalledWith(
      expect.stringContaining('kind=contactSop'),
      { method: 'GET' },
    );
  });

  it('renders employee identity and account facts on the profile workspace', async () => {
    renderPage('/?agentId=7&tab=profile');

    expect(await screen.findByText('林小满')).not.toBeNull();
    expect(screen.getAllByText('MoChat 演示企业')).toHaveLength(2);
    expect(screen.getByText('客户成功部 · 华东组')).not.toBeNull();
    expect(screen.getByText('企业微信身份已验证')).not.toBeNull();
  });

  it('loads the contact directory and keeps a submitted search in the real request', async () => {
    const request = renderPage('/?agentId=7&view=contacts');

    const contact = await screen.findByRole('link', { name: /查看陈晨/ });
    expect(contact.textContent).toContain('重点客户');
    expect(screen.getByText('已成交')).not.toBeNull();
    expect(contact.getAttribute('href')).toContain('wxExternalUserid=wm-contact-31');

    fireEvent.change(screen.getByRole('searchbox', { name: '搜索客户' }), { target: { value: '陈晨' } });
    fireEvent.submit(screen.getByRole('search'));
    await waitFor(() => expect(request).toHaveBeenCalledWith(
      expect.stringContaining('keyword=%E9%99%88%E6%99%A8'),
      { method: 'GET' },
    ));
  });

  it('shows an honest error with retry instead of substituting zero metrics', async () => {
    const request = vi.fn(() => Promise.reject(new Error('网络不可用')));
    renderPage('/', request);

    expect(await screen.findByText('工作台加载失败')).not.toBeNull();
    expect(screen.queryByText('客户总数')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    await waitFor(() => expect(request).toHaveBeenCalledTimes(2));
  });
});
