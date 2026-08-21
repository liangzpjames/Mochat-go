import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { EmployeeConversationList } from './employee-conversation-list';

const employee = {
  id: 9, name: '张三', avatar: '', status: 5 as const, departmentIds: [], archived: true,
  conversationCount: 1, focusedConversationCount: 0, lastConversationAt: '2026-08-20 10:00:00',
};

const props = {
  employee,
  data: { list: [], total: 0, page: 1, pageSize: 20 },
  selectedConversationId: null,
  selectedConversationRowId: null,
  type: '' as const,
  page: 1,
  pending: false,
  fetching: false,
  error: null,
  internalGroupReason: '内部群说明不应出现在离职员工页面',
  onTypeChange: () => undefined,
  onPageChange: () => undefined,
  onSelectConversation: () => undefined,
  onRefresh: () => undefined,
};

describe('EmployeeConversationList', () => {
  it('hides internal group entry when the page provides an allowed type list', () => {
    render(<EmployeeConversationList {...props} allowedTypes={['', 'customer', 'room', 'employee']} />);
    expect(screen.queryByRole('button', { name: '内部群' })).toBeNull();
    expect(screen.queryByText(/未接入|能力限制/)).toBeNull();
  });
});
