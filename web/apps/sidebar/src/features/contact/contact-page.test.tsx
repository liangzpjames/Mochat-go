import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MobileApiError } from '@mochat/mobile-foundation';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { loadContactSummary } from './contact-api';
import { ContactPage } from './contact-page';

afterEach(cleanup);

function renderContact(
  path: string,
  request: ReturnType<typeof vi.fn>,
  onReauthenticate = vi.fn(),
) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <ContactPage request={request} onReauthenticate={onReauthenticate} />
    </MemoryRouter>,
  );
  return { onReauthenticate };
}

describe('Sidebar contact summary', () => {
  it('requests and renders the current external contact summary', async () => {
    const request = vi.fn().mockResolvedValue({
      id: 31,
      name: '林晓',
      avatar: '/avatars/contact-31.png',
      corpId: 9,
    });

    renderContact('/contact?wxExternalUserid=external-user-1&agentId=7', request);

    expect(await screen.findByRole('heading', { name: '林晓' })).not.toBeNull();
    expect(screen.getByRole('img', { name: '林晓的头像' }).getAttribute('src')).toBe(
      '/avatars/contact-31.png',
    );
    expect(request).toHaveBeenCalledWith(
      '/workContact/detail?wxExternalUserid=external-user-1',
      { method: 'GET' },
    );
  });

  it('shows a parameter error and sends no request without an external user ID', () => {
    const request = vi.fn();

    renderContact('/contact?agentId=7', request);

    expect(screen.getByText('客户参数错误')).not.toBeNull();
    expect(request).not.toHaveBeenCalled();
  });

  it('invokes Sidebar re-authentication on a 401', async () => {
    const request = vi.fn().mockRejectedValue(
      new MobileApiError('unauthorized', '登录已失效', { status: 401 }),
    );
    const onReauthenticate = vi.fn();

    renderContact('/contact?wxExternalUserid=external-user-1&agentId=7', request, onReauthenticate);

    await waitFor(() => expect(onReauthenticate).toHaveBeenCalledTimes(1));
    expect(screen.getByText('登录状态已失效')).not.toBeNull();
  });

  it('retries a network failure and calls the API again', async () => {
    const request = vi.fn()
      .mockRejectedValueOnce(new MobileApiError('network', '网络请求失败', { retryable: true }))
      .mockResolvedValueOnce({ id: 31, name: '林晓', avatar: null, corpId: 9 });

    renderContact('/contact?wxExternalUserid=external-user-1&agentId=7', request);
    fireEvent.click(await screen.findByRole('button', { name: '重试' }));

    expect(await screen.findByRole('heading', { name: '林晓' })).not.toBeNull();
    expect(request).toHaveBeenCalledTimes(2);
  });

  it('rejects malformed successful data instead of rendering an empty contact', async () => {
    const request = vi.fn().mockResolvedValue({
      id: 31,
      name: '',
      avatar_url: '/wrong-shape.png',
      corp_id: 9,
    });

    await expect(loadContactSummary(request, 'external-user-1')).rejects.toMatchObject({
      kind: 'validation',
    });
  });
});
