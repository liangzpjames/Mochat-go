import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { CustomerConversationPage } from './conversation-global-api';
import { CustomerConversationList } from './customer-conversation-list';

const groupPage: CustomerConversationPage = {
  customer: { id: 31, name: '陈晓明', avatar: '', profileStatus: 'available' },
  mode: 'group',
  list: [{
    id: 'archive-room-44', conversationId: '9:2:44', employeeId: 9, employeeName: '张伟', employeeAvatar: '',
    targetType: 'room', targetId: 44, targetName: '产品交流群', targetAvatar: '', lastMessage: '请同步报价单',
    sentAt: '2026-08-19 10:00:00', messageTotal: 12, relationStatus: 'active', membershipStatus: 'left',
    riskCount: 2, timeoutCount: 1, focused: true,
  }],
  total: 21,
  page: 1,
  pageSize: 20,
  capabilities: [{ key: 'groupMemberIdentity', available: false, reason: '无法稳定识别群聊入站消息' }],
};

afterEach(cleanup);

describe('CustomerConversationList', () => {
  it('switches direct/group tabs and selects the whole card by stable conversation id', () => {
    const onModeChange = vi.fn();
    const onSelectConversation = vi.fn();

    render(<CustomerConversationList
      data={groupPage}
      error={null}
      fetching={false}
      mode="group"
      onModeChange={onModeChange}
      onPageChange={() => undefined}
      onRefresh={() => undefined}
      onSelectConversation={onSelectConversation}
      page={1}
      pending={false}
      selectedConversationId={null}
    />);

    fireEvent.click(screen.getByRole('tab', { name: '单聊' }));
    expect(onModeChange).toHaveBeenCalledWith('direct');
    fireEvent.click(screen.getByRole('button', { name: /产品交流群.*张伟/ }));
    expect(onSelectConversation).toHaveBeenCalledWith('9:2:44');
    expect(screen.getByText('资料已同步')).toBeTruthy();
    expect(screen.getByText('已退群')).toBeTruthy();
    expect(screen.getByRole('alert').textContent).toContain('无法稳定识别群聊入站消息');
    expect(screen.getByRole('button', { name: '刷新会话' }).textContent).toContain('刷新会话');
  });

  it('renders direct conversation labels, optional signals and fixed-size pagination', () => {
    const directPage: CustomerConversationPage = {
      ...groupPage,
      mode: 'direct',
      list: [{ ...groupPage.list[0]!, targetType: 'customer', targetId: 31, targetName: '陈晓明',
        conversationId: '9:1:31', lastMessage: '已发送合同', membershipStatus: 'active', relationStatus: 'lost', employeeAvatar: 'https://cdn.example/employee.png' }],
      total: 21,
      capabilities: [],
    };
    const onPageChange = vi.fn();

    const { container } = render(<CustomerConversationList
      data={directPage}
      error={null}
      fetching={false}
      mode="direct"
      onModeChange={() => undefined}
      onPageChange={onPageChange}
      onRefresh={() => undefined}
      onSelectConversation={() => undefined}
      page={1}
      pending={false}
      selectedConversationId="9:1:31"
    />);

    expect(screen.getByRole('button', { name: /张伟.*陈晓明/ }).getAttribute('aria-pressed')).toBe('true');
    expect(container.querySelector('.customer-conversation-card-avatar img')?.getAttribute('src')).toBe('https://cdn.example/employee.png');
    expect(screen.getByText('关系已流失')).toBeTruthy();
    expect(screen.getByText('风险 2')).toBeTruthy();
    expect(screen.getByText('超时 1')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '第 2 页' }));
    expect(onPageChange).toHaveBeenCalledWith(2);
    expect(screen.getByRole('navigation', { name: '客户会话分页' }).textContent).toContain('共 21 条');
  });

  it('uses a neutral placeholder icon when no customer is selected', () => {
    render(<CustomerConversationList
      data={undefined}
      error={null}
      fetching={false}
      mode="direct"
      onModeChange={() => undefined}
      onPageChange={() => undefined}
      onRefresh={() => undefined}
      onSelectConversation={() => undefined}
      page={1}
      pending={false}
      selectedConversationId={null}
    />);

    expect(screen.getAllByText('请选择客户')).toHaveLength(2);
    expect(screen.getByTestId('customer-conversation-empty-avatar')).toBeTruthy();
    expect(screen.queryByText('请', { selector: '.customer-conversation-customer-avatar > span' })).toBeNull();
  });

  it('does not keep cached customer data after the selection is cleared', () => {
    render(<CustomerConversationList
      customer={undefined}
      data={groupPage}
      error={null}
      fetching={false}
      mode="direct"
      onModeChange={() => undefined}
      onPageChange={() => undefined}
      onRefresh={() => undefined}
      onSelectConversation={() => undefined}
      page={1}
      pending={false}
      selectedConversationId={null}
    />);

    expect(screen.getAllByText('请选择客户')).toHaveLength(2);
    expect(screen.getByTestId('customer-conversation-empty-avatar')).toBeTruthy();
    expect(screen.queryByText('资料已同步')).toBeNull();
    expect(screen.queryByRole('button', { name: /产品交流群.*张伟/ })).toBeNull();
  });

  it('keeps the selected customer visible while associated conversations are loading', () => {
    render(<CustomerConversationList
      customer={{ id: 31, name: '星河科技', avatar: '', profileStatus: 'available' }}
      data={undefined}
      error={null}
      fetching={true}
      mode="direct"
      onModeChange={() => undefined}
      onPageChange={() => undefined}
      onRefresh={() => undefined}
      onSelectConversation={() => undefined}
      page={1}
      pending={true}
      selectedConversationId={null}
    />);

    expect(screen.getByText('星河科技')).toBeTruthy();
    expect(screen.getByRole('heading', { name: '正在加载客户会话' })).toBeTruthy();
    expect(screen.queryByRole('heading', { name: '请选择客户' })).toBeNull();
  });
});
