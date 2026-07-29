import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { SidebarApp } from './sidebar-app';

afterEach(cleanup);

describe('SidebarApp', () => {
  it('renders customer details and opens an editable remark panel', () => {
    render(
      <MemoryRouter initialEntries={['/contact?wxExternalUserid=external-1']}>
        <SidebarApp request={vi.fn(() => Promise.resolve({}))} />
      </MemoryRouter>,
    );

    expect(screen.getByText('客户资料')).not.toBeNull();
    expect(screen.getByText('互动轨迹')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '修改备注' }));
    expect(screen.getByLabelText('客户备注')).not.toBeNull();
  });

  it('completes fake authorization and shows a usable entry action', () => {
    const request = vi.fn(() => Promise.resolve({ token: 'fake-token' }));
    render(
      <MemoryRouter initialEntries={['/auth?code=fake-code']}>
        <SidebarApp request={request} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: '确认授权' }));
    expect(request).toHaveBeenCalledWith('/sidebar/auth', expect.objectContaining({
      method: 'POST',
    }));
  });
});
