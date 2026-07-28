import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AppErrorPage } from './app-error-page';
import { ForbiddenPage } from './forbidden-page';

afterEach(cleanup);

describe('access error pages', () => {
  it('renders a stable forbidden state', () => {
    render(<ForbiddenPage />);

    expect(screen.getByRole('heading').textContent).toContain('无权访问');
  });

  it('retries only after the user requests it', () => {
    const retry = vi.fn();
    render(<AppErrorPage message="服务暂时不可用" retry={retry} />);

    expect(retry).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    expect(retry).toHaveBeenCalledOnce();
  });
});
