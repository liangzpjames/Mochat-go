/* eslint-disable @typescript-eslint/no-unsafe-assignment -- mock call inspection verifies generated idempotency payloads */
import { ApiError } from '@mochat/api-client';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { FollowUpTimeline } from './follow-up-timeline';

describe('FollowUpTimeline', () => {
  it('appends non-empty follow-up content for a contact', async () => {
    const append = vi.fn().mockResolvedValue({});
    render(<QueryClientProvider client={new QueryClient()}><FollowUpTimeline api={{ listFollowUps: vi.fn().mockResolvedValue({ items: [] }), appendFollowUp: append }} corpId={7} contactId="c1" /></QueryClientProvider>);
    fireEvent.change(await screen.findByLabelText('跟进内容'), { target: { value: '已完成首次沟通' } });
    fireEvent.click(screen.getByRole('button', { name: '添加跟进' }));
    await waitFor(() => expect(append).toHaveBeenCalledWith({ corpId: 7, contactId: 'c1', content: '已完成首次沟通', idempotencyKey: expect.any(String) }));
  });

  it.each([
    [403, 'forbidden'],
    [404, 'not-found'],
    [500, 'error'],
  ])('renders list status %i with PageState retry', async (status, state) => {
    const listFollowUps = vi.fn().mockRejectedValue(new ApiError('validation', 'failed', { status }));
    const { container } = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><FollowUpTimeline api={{ listFollowUps, appendFollowUp: vi.fn() }} corpId={7} contactId="c1" /></QueryClientProvider>);
    await waitFor(() => expect(container.querySelector(`.page-state-${state}`)).not.toBeNull());
    fireEvent.click(container.querySelector('.page-state-retry')!);
    await waitFor(() => expect(listFollowUps).toHaveBeenCalledTimes(2));
  });

  it('renders append loading and 409 conflict with retry', async () => {
    let rejectAppend: ((reason: unknown) => void) | undefined;
    const append = vi.fn().mockImplementationOnce(() => new Promise((_, reject) => { rejectAppend = reject; })).mockResolvedValueOnce({});
    const { container } = render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })}><FollowUpTimeline api={{ listFollowUps: vi.fn().mockResolvedValue({ items: [] }), appendFollowUp: append }} corpId={7} contactId="c1" /></QueryClientProvider>);
    fireEvent.change(await screen.findByLabelText('跟进内容'), { target: { value: '首次沟通' } });
    fireEvent.click(screen.getByRole('button', { name: '添加跟进' }));
    await waitFor(() => expect(container.querySelector('.page-state-loading')).not.toBeNull());
    rejectAppend?.(new ApiError('validation', 'conflict', { status: 409 }));
    await waitFor(() => expect(container.querySelector('.page-state-conflict')).not.toBeNull());
    fireEvent.click(container.querySelector('.page-state-retry')!);
    await waitFor(() => expect(append).toHaveBeenCalledTimes(2));
  });
});

afterEach(cleanup);
