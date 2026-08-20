import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { CustomerDirectoryPage } from './conversation-global-api';
import { CustomerConversationDirectory } from './customer-conversation-directory';

const page: CustomerDirectoryPage = {
  customers: [
    {
      id: 31,
      name: '陈晓明',
      avatar: '',
      profileStatus: 'missing',
      activeRelationCount: 2,
      lostRelationCount: 0,
      directConversationCount: 3,
      groupConversationCount: 1,
      focusedConversationCount: 1,
      lastConversationAt: '2026-08-19 10:00:00',
    },
    {
      id: 32,
      name: '',
      avatar: '',
      profileStatus: 'available',
      activeRelationCount: 0,
      lostRelationCount: 1,
      directConversationCount: 0,
      groupConversationCount: 0,
      focusedConversationCount: 0,
      lastConversationAt: '',
    },
  ],
  counts: { all: 3, focused: 2, active: 2, lost: 1 },
  page: 1,
  pageSize: 50,
  total: 51,
  limitations: [{ key: 'profile', reason: '部分客户资料暂未同步' }],
  capabilities: [],
};

afterEach(cleanup);

describe('CustomerConversationDirectory', () => {
  it('switches modes, submits search and selects the whole customer row', () => {
    const onModeChange = vi.fn();
    const onKeywordDraftChange = vi.fn();
    const onSearch = vi.fn((event: React.FormEvent<HTMLFormElement>) => event.preventDefault());
    const onRefresh = vi.fn();
    const onPageChange = vi.fn();
    const onSelectCustomer = vi.fn();

    render(<CustomerConversationDirectory
      data={page}
      error={null}
      fetching={false}
      keywordDraft=""
      mode="all"
      onKeywordDraftChange={onKeywordDraftChange}
      onModeChange={onModeChange}
      onPageChange={onPageChange}
      onRefresh={onRefresh}
      onSearch={onSearch}
      onSelectCustomer={onSelectCustomer}
      page={1}
      pending={false}
      selectedCustomerId={null}
    />);

    fireEvent.click(screen.getByRole('tab', { name: '重点关注 2' }));
    expect(onModeChange).toHaveBeenCalledWith('focused');
    fireEvent.change(screen.getByRole('textbox', { name: '搜索客户' }), { target: { value: '陈' } });
    expect(onKeywordDraftChange).toHaveBeenCalledWith('陈');
    fireEvent.submit(screen.getByRole('search', { name: '客户搜索' }));
    expect(onSearch).toHaveBeenCalled();
    expect(screen.getByRole('button', { name: '搜索客户' }).textContent).toContain('搜索');
    expect(screen.getByRole('button', { name: '刷新客户' }).textContent).toContain('刷新客户');
    fireEvent.click(screen.getByRole('button', { name: /陈晓明/ }));
    expect(onSelectCustomer).toHaveBeenCalledWith(31);
    expect(screen.getByRole('alert').textContent).toContain('客户资料未同步');
    expect(screen.getByRole('button', { name: /客户 32/ })).toBeTruthy();
  });

  it('exposes relation filters, refresh and fixed-size pagination', () => {
    const onModeChange = vi.fn();
    const onRefresh = vi.fn();
    const onPageChange = vi.fn();

    render(<CustomerConversationDirectory
      data={page}
      error={null}
      fetching={false}
      keywordDraft=""
      mode="active"
      onKeywordDraftChange={() => undefined}
      onModeChange={onModeChange}
      onPageChange={onPageChange}
      onRefresh={onRefresh}
      onSearch={(event) => event.preventDefault()}
      onSelectCustomer={() => undefined}
      page={1}
      pending={false}
      selectedCustomerId={31}
    />);

    expect(screen.getByRole('button', { name: /陈晓明/ }).getAttribute('aria-pressed')).toBe('true');
    expect(screen.getAllByTestId('customer-conversation-directory-empty-avatar')).toHaveLength(2);
    expect(screen.queryByText('陈', { selector: '.customer-conversation-avatar > span' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '有效关系 2' }));
    expect(onModeChange).toHaveBeenCalledWith('active');
    fireEvent.click(screen.getByRole('button', { name: '已流失 1' }));
    expect(onModeChange).toHaveBeenCalledWith('lost');
    fireEvent.click(screen.getByRole('button', { name: '刷新客户' }));
    expect(onRefresh).toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '第 2 页' }));
    expect(onPageChange).toHaveBeenCalledWith(2);
    expect(screen.getByRole('navigation', { name: '客户分页' }).textContent).toContain('共 51 条');
  });

  it('uses PageState for loading, error and empty results', () => {
    const common = {
      data: undefined,
      fetching: false,
      keywordDraft: '',
      mode: 'all' as const,
      onKeywordDraftChange: () => undefined,
      onModeChange: () => undefined,
      onPageChange: () => undefined,
      onRefresh: () => undefined,
      onSearch: (event: React.FormEvent<HTMLFormElement>) => event.preventDefault(),
      onSelectCustomer: () => undefined,
      page: 1,
      selectedCustomerId: null,
    };
    const { rerender } = render(<CustomerConversationDirectory {...common} error={null} pending />);
    expect(screen.getByText('正在加载客户')).toBeTruthy();

    rerender(<CustomerConversationDirectory {...common} error={new Error('服务不可用')} pending={false} />);
    expect(screen.getByText('客户目录加载失败')).toBeTruthy();
    expect(screen.getByText('服务不可用')).toBeTruthy();

    rerender(<CustomerConversationDirectory {...common} data={{ ...page, customers: [], total: 0 }} error={null} pending={false} />);
    expect(screen.getByText('暂无客户')).toBeTruthy();
  });

  it('renders customer rows as a list with a time column', () => {
    render(<CustomerConversationDirectory
      data={page}
      error={null}
      fetching={false}
      keywordDraft=""
      mode="all"
      onKeywordDraftChange={() => undefined}
      onModeChange={() => undefined}
      onPageChange={() => undefined}
      onRefresh={() => undefined}
      onSearch={(event) => event.preventDefault()}
      onSelectCustomer={() => undefined}
      page={1}
      pending={false}
      selectedCustomerId={null}
    />);

    expect(screen.getByRole('list', { name: '客户列表' })).toBeTruthy();
    expect(screen.getAllByRole('listitem')).toHaveLength(2);
    expect(screen.getByText('08-19 10:00')).toBeTruthy();
  });
});
