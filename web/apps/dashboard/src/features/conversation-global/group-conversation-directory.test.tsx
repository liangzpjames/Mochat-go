import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { GroupRoomDirectoryPage } from './conversation-global-api';
import { GroupConversationDirectory } from './group-conversation-directory';

const data: GroupRoomDirectoryPage = {
  items: [{ id: 71, externalId: 'wr_71', name: '星河客户群', avatar: '', ownerId: 9, ownerName: '张三', memberCount: 6, employeeCount: 2, customerCount: 4, messageCount: 18, lastMessage: '请确认排期', lastMessageAt: '2026-08-19 11:00:00', focused: false, riskCount: 1, timeoutCount: 0, dissolved: false }],
  total: 1, page: 1, pageSize: 50, limitations: [], capabilities: [],
};

describe('GroupConversationDirectory', () => {
  afterEach(cleanup);
  it('does not search while typing and selects the whole room card', () => {
    const onSearch = vi.fn(); const onSelect = vi.fn();
    render(<GroupConversationDirectory data={data} mode="active" keyword="" selectedRoomId={null} pending={false} error={null} onKeywordChange={vi.fn()} onSearch={onSearch} onRefresh={vi.fn()} onModeChange={vi.fn()} onSelect={onSelect} onPageChange={vi.fn()} onOpenFilter={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /星河客户群/ }));
    expect(onSelect).toHaveBeenCalledWith(data.items[0]);
    expect(onSearch).not.toHaveBeenCalled();
    fireEvent.submit(screen.getByRole('search'));
    expect(onSearch).toHaveBeenCalledTimes(1);
  });

  it('keeps refresh next to query and reports unavailable archive data', () => {
    const onRefresh = vi.fn();
    render(<GroupConversationDirectory data={{ ...data, items: [], total: 0, limitations: [{ key: 'archiveUnavailable', reason: '当前企业没有可用的会话存档数据' }] }} mode="active" keyword="" selectedRoomId={null} pending={false} error={null} onKeywordChange={vi.fn()} onSearch={vi.fn()} onRefresh={onRefresh} onModeChange={vi.fn()} onSelect={vi.fn()} onPageChange={vi.fn()} onOpenFilter={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: '刷新群聊' }));
    expect(onRefresh).toHaveBeenCalledTimes(1);
    expect(screen.getByText('当前企业没有可用的会话存档数据')).toBeTruthy();
  });
});
