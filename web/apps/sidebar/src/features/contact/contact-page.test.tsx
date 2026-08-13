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

const contactPath = '/contact?wxExternalUserid=external-user-1&agentId=7';
const rawContact = { id: 31, name: '林晓', avatar: null, corpId: 9 };

describe('Sidebar contact summary', () => {
  it('requests and renders the current external contact summary', async () => {
    const request = vi.fn().mockResolvedValue({
      ...rawContact,
      avatar: '/avatars/contact-31.png',
    });

    renderContact(contactPath, request);

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

    renderContact(contactPath, request, onReauthenticate);

    await waitFor(() => expect(onReauthenticate).toHaveBeenCalledTimes(1));
    expect(screen.getByText('登录状态已失效')).not.toBeNull();
    expect(screen.queryByRole('button', { name: '重试' })).toBeNull();
  });

  it.each([
    ['forbidden', new MobileApiError('forbidden', '没有客户查看权限', { status: 403 }), 'forbidden', false],
    ['not-found', new MobileApiError('not-found', '客户不存在', { status: 404 }), 'not-found', false],
    ['validation', new MobileApiError('validation', '客户参数无效', { status: 400 }), 'error', false],
    ['conflict', new MobileApiError('conflict', '客户状态冲突', { status: 409 }), 'error', false],
    ['network', new MobileApiError('network', '网络请求失败'), 'error', true],
    ['server', new MobileApiError('server', '服务暂时不可用', { status: 503 }), 'error', true],
    ['explicit retryable', new MobileApiError('aborted', '请求超时', { retryable: true }), 'error', true],
    ['unknown', new Error('未知客户错误'), 'error', false],
  ] as const)(
    'maps %s to MobileState %s with retry=%s',
    async (_label, error, expectedKind, retryable) => {
      const request = vi.fn().mockRejectedValue(error);

      renderContact(contactPath, request);

      expect((await screen.findAllByText(error.message)).length).toBeGreaterThanOrEqual(1);
      expect(document.querySelector(`.mobile-state--${expectedKind}`)).not.toBeNull();
      if (retryable) {
        expect(screen.getByRole('button', { name: '重试' })).not.toBeNull();
      } else {
        expect(screen.queryByRole('button', { name: '重试' })).toBeNull();
      }
    },
  );

  it('retries a network failure and calls the API again', async () => {
    const request = vi.fn()
      .mockRejectedValueOnce(new MobileApiError('network', '网络请求失败', { retryable: true }))
      .mockResolvedValueOnce(rawContact);

    renderContact(contactPath, request);
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
