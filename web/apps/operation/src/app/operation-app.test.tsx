import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { OperationApp } from './operation-app';

afterEach(cleanup);

describe('OperationApp', () => {
  it('renders a lottery activity and confirms the draw through the Go endpoint', () => {
    const request = vi.fn(() => Promise.resolve({ prizeName: '体验券' }));
    render(
      <MemoryRouter initialEntries={['/lottery?activityId=7']}>
        <OperationApp request={request} />
      </MemoryRouter>,
    );

    expect(screen.getByText('抽奖活动')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '立即抽奖' }));
    expect(request).toHaveBeenCalledWith('/operation/lottery', expect.objectContaining({
      method: 'POST',
    }));
  });

  it('shows progress details for a fission activity', () => {
    render(
      <MemoryRouter initialEntries={['/fissionSpeed?activityId=8']}>
        <OperationApp request={vi.fn(() => Promise.resolve({}))} />
      </MemoryRouter>,
    );

    expect(screen.getByText('群裂变进度')).not.toBeNull();
    expect(screen.getByText('已完成 3 / 5')).not.toBeNull();
  });
});
