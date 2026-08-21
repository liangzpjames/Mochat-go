import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { StaffDirectoryPage } from './conversation-global-api';
import { ConversationTrajectoryDirectory } from './conversation-trajectory-directory';

afterEach(cleanup);

const data: StaffDirectoryPage = {
  departments: [],
  employees: [{
    id: 1002,
    name: '李娜',
    avatar: '',
    status: 1,
    departmentIds: [],
    archived: true,
    conversationCount: 3,
    focusedConversationCount: 0,
    lastConversationAt: '2026-08-15 19:15:17',
  }],
  counts: { all: 1, focused: 0, archived: 1, departed: 0 },
  page: 1,
  pageSize: 50,
  total: 1,
  limitations: [],
  capabilities: [],
};

describe('ConversationTrajectoryDirectory 员工摘要', () => {
  it('只展示员工名称，不展示归档状态和会话数量', () => {
    render(
      <ConversationTrajectoryDirectory
        mode="archived"
        draftKeyword=""
        data={data}
        selectedEmployeeId={1002}
        isLoading={false}
        onDraftKeywordChange={vi.fn()}
        onKeywordSubmit={vi.fn()}
        onModeChange={vi.fn()}
        onDepartmentChange={vi.fn()}
        onPageChange={vi.fn()}
        onSelectEmployee={vi.fn()}
      />,
    );

    expect(screen.getByText('李娜')).toBeTruthy();
    expect(screen.queryByText(/已归档/)).toBeNull();
    expect(screen.queryByText(/个会话/)).toBeNull();
    expect(screen.queryByRole('button', { name: '刷新' })).toBeNull();
  });
});
