import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { DemoPage } from './demo-page';
import { staffConversationDemo } from './demo-fixtures';

afterEach(cleanup);

describe('DemoPage', () => {
  it('filters the stable fixture when searching', () => {
    render(<DemoPage config={staffConversationDemo} />);

    fireEvent.change(screen.getByRole('searchbox', { name: '搜索员工会话' }), {
      target: { value: '林晨' },
    });

    expect(screen.getByText('林晨')).toBeTruthy();
    expect(screen.queryByText('周明')).toBeNull();
  });

  it('changes the current page through pagination', () => {
    render(<DemoPage config={staffConversationDemo} />);

    fireEvent.click(screen.getByRole('button', { name: '第 2 页' }));

    expect(screen.getByText('许诺')).toBeTruthy();
    expect(screen.queryByText('林晨')).toBeNull();
  });

  it('opens a detail drawer from the only detail action', () => {
    render(<DemoPage config={staffConversationDemo} />);

    fireEvent.click(screen.getByRole('button', { name: '查看林晨详情' }));

    expect(screen.getByRole('dialog', { name: '员工会话详情' })).toBeTruthy();
    expect(screen.getByText('演示数据，仅用于界面预览')).toBeTruthy();
  });

  it('shows an empty state when no fixture row matches', () => {
    render(<DemoPage config={staffConversationDemo} />);

    fireEvent.change(screen.getByRole('searchbox', { name: '搜索员工会话' }), {
      target: { value: '不存在' },
    });

    expect(screen.getByText('暂无匹配的演示数据')).toBeTruthy();
  });

  it('does not claim that demo data was persisted', () => {
    render(<DemoPage config={staffConversationDemo} />);

    expect(screen.queryByText(/已保存|已持久化|保存成功/)).toBeNull();
  });
});
