import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { CustomerConversationDetail } from './conversation-global-api';
import { CustomerConversationDetailPane } from './customer-conversation-detail';

const directDetail: CustomerConversationDetail = {
  conversationId: '9:1:31',
  customerId: 31,
  customerName: '陈晓明',
  profile: { id: 31, name: '陈晓明', avatar: '', profileStatus: 'available' },
  employeeId: 9,
  employeeName: '张伟',
  targetType: 'customer',
  targetId: 31,
  targetName: '陈晓明',
  focused: false,
  stats: { communicationDays: 3, messageTotal: 12, inboundTotal: 5, outboundTotal: 7 },
  messages: [{ id: 'm1', senderName: '张伟', senderAvatar: '', direction: 'outbound', type: 1, content: { text: '您好' }, sentAt: '2026-08-19 10:00:00' }],
  nextBefore: 'cursor-1',
  hasMore: true,
  capabilities: [],
};

const baseProps = {
  data: directDetail,
  messages: directDetail.messages,
  keyword: '',
  date: '',
  messageTypes: [] as readonly string[],
  pending: false,
  fetching: false,
  loadingOlder: false,
  error: null,
  focusPending: false,
  focusError: null,
  hasMore: true,
  onKeywordChange: vi.fn(),
  onKeywordSearch: vi.fn(),
  onDateChange: vi.fn(),
  onToggleMessageType: vi.fn(),
  onRefresh: vi.fn(),
  onLoadOlder: vi.fn(),
  onToggleFocus: vi.fn(),
};

afterEach(cleanup);

describe('CustomerConversationDetailPane', () => {
  it('renders direct stats and invokes message filter, cursor and focus actions', () => {
    render(<CustomerConversationDetailPane {...baseProps} />);

    expect(screen.getByText('客户发送')).toBeTruthy();
    expect(screen.getByText('5')).toBeTruthy();
    fireEvent.click(screen.getByLabelText('图片'));
    expect(baseProps.onToggleMessageType).toHaveBeenCalledWith('image');
    fireEvent.change(screen.getByLabelText('搜索会话内容'), { target: { value: '报价' } });
    expect(baseProps.onKeywordChange).toHaveBeenCalledWith('报价');
    fireEvent.change(screen.getByLabelText('检索日期'), { target: { value: '2026-08-19' } });
    expect(baseProps.onDateChange).toHaveBeenCalledWith('2026-08-19');
    fireEvent.submit(screen.getByRole('button', { name: '查询会话内容' }).closest('form')!);
    expect(baseProps.onKeywordSearch).toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '加载更早消息' }));
    expect(baseProps.onLoadOlder).toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '重点关注' }));
    expect(baseProps.onToggleFocus).toHaveBeenCalled();
    expect(screen.getByText('您好')).toBeTruthy();
  });

  it('uses group inbound label, capability warning and 群成员 sender fallback', () => {
    const groupDetail: CustomerConversationDetail = {
      ...directDetail,
      conversationId: '9:2:44',
      targetType: 'room',
      targetId: 44,
      targetName: '产品交流群',
      stats: { communicationDays: null, messageTotal: null, inboundTotal: null, outboundTotal: null },
      capabilities: [{ key: 'groupMemberIdentity', available: false, reason: '无法稳定识别群聊成员' }],
      messages: [{ ...directDetail.messages[0]!, id: 'm2', senderName: '', direction: 'inbound' }],
    };

    render(<CustomerConversationDetailPane {...baseProps} data={groupDetail} messages={groupDetail.messages} />);

    expect(screen.getByText('非员工消息')).toBeTruthy();
    expect(screen.queryByText('客户发送')).toBeNull();
    expect(screen.getByText('无法稳定识别群聊成员')).toBeTruthy();
    expect(screen.getByText('群成员')).toBeTruthy();
    expect(screen.getAllByText('--').length).toBe(4);
  });

  it('uses the customer profile name when a direct target name is unavailable', () => {
    const { rerender } = render(<CustomerConversationDetailPane {...baseProps} data={{ ...directDetail, targetName: '', customerName: '客户资料名' }} messages={directDetail.messages} />);
    expect(screen.getByText('客户资料名')).toBeTruthy();
    rerender(<CustomerConversationDetailPane {...baseProps} data={{ ...directDetail, targetName: '', customerName: '', profile: { ...directDetail.profile, name: '资料回退名' } }} messages={directDetail.messages} />);
    expect(screen.getByText('资料回退名')).toBeTruthy();
  });

  it('renders loading, error and empty states without replacing the detail header contract', () => {
    const { rerender } = render(<CustomerConversationDetailPane {...baseProps} data={undefined} messages={[]} />);
    expect(screen.getByText('请选择会话')).toBeTruthy();
    rerender(<CustomerConversationDetailPane {...baseProps} pending />);
    expect(screen.getByText('正在读取客户会话')).toBeTruthy();
    rerender(<CustomerConversationDetailPane {...baseProps} error={new Error('接口暂时不可用')} />);
    expect(screen.getByText('客户会话加载失败')).toBeTruthy();
    rerender(<CustomerConversationDetailPane {...baseProps} messages={[]} hasMore={false} />);
    expect(screen.getByText('暂无匹配消息')).toBeTruthy();
  });
});
