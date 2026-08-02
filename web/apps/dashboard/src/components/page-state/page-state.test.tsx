import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { pageStateForError, PageState } from './page-state';

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

  it('keeps loading, empty, forbidden, conflict, and retry states in the public PageState interface', () => {
    const { rerender } = render(<PageState state="loading" />);

    expect(screen.getByRole('status').getAttribute('aria-busy')).toBe('true');

    rerender(<PageState state="empty" />);
    expect(screen.getByRole('status').className).toContain('page-state-empty');

    rerender(<PageState state="forbidden" />);
    expect(screen.queryByRole('button')).toBeNull();

    rerender(<PageState state="conflict" onRetry={() => undefined} />);
    expect(screen.getByRole('button', { name: '重新加载' }).className).toContain('page-state-retry');
  });

  it('keeps the existing children escape hatch for specialized state content', () => {
    render(<PageState state="error"><p>业务特有提示</p></PageState>);

    expect(screen.getByText('业务特有提示')).toBeTruthy();
    expect(screen.queryByText('加载失败')).toBeNull();
  });

  it('maps typed and status-shaped HTTP errors to PageState access and conflict states', () => {
    expect(pageStateForError({ status: 403 })).toBe('forbidden');
    expect(pageStateForError({ status: 409 })).toBe('conflict');
    expect(pageStateForError(new Error('offline'))).toBe('error');
  });
});
