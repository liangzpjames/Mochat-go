import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { PageState } from './page-state';

describe('PageState', () => {
  afterEach(cleanup);

  it('renders the conflict message and retries', async () => {
    const onRetry = vi.fn();

    render(<PageState state="conflict" onRetry={onRetry} />);

    expect(screen.getByText('数据已被其他操作更新')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it('renders an explicit forbidden state without a retry action', () => {
    render(<PageState state="forbidden" />);

    expect(screen.getByText('暂无权限查看此内容')).toBeTruthy();
    expect(screen.queryByRole('button', { name: '重新加载' })).toBeNull();
  });
});
