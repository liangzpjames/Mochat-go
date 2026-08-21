import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { GroupRoomDirectoryItem, GroupRoomMessages } from './conversation-global-api';
import { GroupConversationMessages } from './group-conversation-messages';

const room: GroupRoomDirectoryItem = { id: 71, externalId: 'wr_71', name: '星河客户群', avatar: '', ownerId: 9, ownerName: '张三', memberCount: 6, employeeCount: 2, customerCount: 4, messageCount: 18, lastMessage: '请确认排期', lastMessageAt: '2026-08-19 11:00:00', focused: false, riskCount: 1, timeoutCount: 0, dissolved: false };
const data: GroupRoomMessages = { roomId: 71, stats: { messageTotal: 18, employeeTotal: 8, customerTotal: 10, riskTotal: 1, timeoutTotal: 0 }, messages: [{ id: 'm1', senderId: 9, senderName: '张三', senderAvatar: '', senderKind: 'employee', direction: 'outbound', sentAt: '2026-08-19 11:00:00', archiveSource: 'external', archiveSourceId: 'wecom', type: 1, content: { text: '请确认排期' } }], nextBefore: 'cursor-1', hasMore: true, capabilities: [] };

describe('GroupConversationMessages', () => {
  afterEach(cleanup);
  it('submits message keyword only after query and renders backend stats', () => {
    const onSearch = vi.fn();
    render(<GroupConversationMessages room={room} data={data} keyword="" date="" messageTypes={[]} pending={false} fetching={false} loadingOlder={false} error={null} onKeywordChange={vi.fn()} onDateChange={vi.fn()} onToggleMessageType={vi.fn()} onSearch={onSearch} onRefresh={vi.fn()} onLoadOlder={vi.fn()} />);
    expect(screen.getByText('18')).toBeTruthy();
    expect(screen.getByText('请确认排期')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '查询消息' }));
    expect(onSearch).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: '加载更早消息' })).toBeTruthy();
  });
});
