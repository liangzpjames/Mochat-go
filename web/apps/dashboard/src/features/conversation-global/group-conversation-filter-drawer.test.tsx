import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { GroupConversationFilterDrawer } from './group-conversation-filter-drawer';

describe('GroupConversationFilterDrawer', () => {
  afterEach(cleanup);
  it('submits data-backed filter choices once and closes on the backdrop', () => {
    const onSubmit = vi.fn(); const onClose = vi.fn();
    render(<GroupConversationFilterDrawer open value={{ employeeId: '', customerId: '', groupId: '' }} options={{ employees: [{ value: '9', label: '张三', count: 4 }], customers: [{ value: '31', label: '陈晓明', count: 2 }], groups: [{ value: '4', label: '重点客户', count: 1 }], capabilities: [] }} onSubmit={onSubmit} onClose={onClose} />);
    fireEvent.change(screen.getByLabelText('员工'), { target: { value: '9' } });
    fireEvent.click(screen.getByRole('button', { name: '查询筛选' }));
    expect(onSubmit).toHaveBeenCalledWith({ employeeId: '9', customerId: '', groupId: '' });
    expect(onSubmit).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByTestId('group-filter-backdrop'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
