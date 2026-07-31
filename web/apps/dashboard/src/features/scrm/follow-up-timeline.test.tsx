import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';
import { FollowUpTimeline } from './follow-up-timeline';

describe('FollowUpTimeline', () => {
  it('appends non-empty follow-up content for a contact', async () => {
    const append = vi.fn().mockResolvedValue({});
    render(<QueryClientProvider client={new QueryClient()}><FollowUpTimeline api={{ listFollowUps: vi.fn().mockResolvedValue({ items: [] }), appendFollowUp: append }} corpId={7} contactId="c1" /></QueryClientProvider>);
    fireEvent.change(screen.getByLabelText('跟进内容'), { target: { value: '已完成首次沟通' } });
    fireEvent.click(screen.getByRole('button', { name: '添加跟进' }));
    await waitFor(() => expect(append).toHaveBeenCalledWith({ corpId: 7, contactId: 'c1', content: '已完成首次沟通', idempotencyKey: expect.any(String) }));
  });
});
