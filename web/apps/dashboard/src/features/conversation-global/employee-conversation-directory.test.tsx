import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { EmployeeConversationDirectory } from './employee-conversation-directory';

const props = {
  data: { departments: [{ id: 10, parentId: 0, name: '销售部', employeeCount: 3, children: [] }], employees: [{ id: 9, name: '张三', avatar: '', status: 5 as const, departmentIds: [10], archived: true, conversationCount: 1, focusedConversationCount: 0, lastConversationAt: '' }], counts: { all: 1, focused: 0, archived: 1, departed: 1 }, page: 1, pageSize: 50 as const, total: 1, limitations: [], capabilities: [] },
  employees: [{ id: 9, name: '张三', avatar: '', status: 5 as const, departmentIds: [], archived: true, conversationCount: 1, focusedConversationCount: 0, lastConversationAt: '' }],
  mode: 'departed' as const,
  selectedDepartmentId: null,
  selectedEmployeeId: null,
  keywordDraft: '',
  pending: false,
  fetching: false,
  error: null,
  hasMore: false,
  onKeywordDraftChange: () => undefined,
  onSearch: (event: React.FormEvent<HTMLFormElement>) => event.preventDefault(),
  onRefresh: () => undefined,
  onModeChange: () => undefined,
  onDepartmentChange: () => undefined,
  onSelectEmployee: () => undefined,
  onLoadMore: () => undefined,
};

describe('EmployeeConversationDirectory', () => {
  afterEach(() => cleanup());

  it('locks the directory to departed mode without exposing other mode switches', () => {
    render(<EmployeeConversationDirectory {...props} lockedMode="departed" />);
    expect(screen.queryByRole('tab', { name: '重点关注' })).toBeNull();
    expect(screen.queryByRole('button', { name: /全部成员/ })).toBeNull();
    expect(screen.getByRole('button', { name: /张三.*已离职/ })).not.toBeNull();
  });

  it('uses a compact department picker without misleading employee counts', () => {
    const onDepartmentChange = vi.fn();
    render(<EmployeeConversationDirectory {...props} onDepartmentChange={onDepartmentChange} />);
    const trigger = screen.getByRole('button', { name: '部门：全部部门' });
    expect(trigger.textContent).toBe('部门：全部部门⌄');
    expect(screen.queryByRole('button', { name: /销售部 3/ })).toBeNull();
    fireEvent.click(trigger);
    expect(screen.getByRole('listbox', { name: '部门选择' })).toBeTruthy();
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('listbox', { name: '部门选择' })).toBeNull();
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('option', { name: '销售部' }));
    expect(onDepartmentChange).toHaveBeenCalledWith(10);
    expect(screen.queryByRole('listbox', { name: '部门选择' })).toBeNull();
  });
});
