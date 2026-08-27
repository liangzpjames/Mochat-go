import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import { FileAudioPage } from './file-audio-page';
import type { FileAudioApi } from './file-audio-api';

afterEach(() => cleanup());

function renderPage(api: FileAudioApi, initialEntry = '/chat/file-audio') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const access = { corp: { id: '9' }, session: {}, menu: [], allowedRoutes: new Set(), allowedActions: new Set() } as never;
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <DashboardAccessProvider value={access}>
        <QueryClientProvider client={client}>
          <FileAudioPage api={api} />
        </QueryClientProvider>
      </DashboardAccessProvider>
    </MemoryRouter>,
  );
}

const sample = {
  list: [
    {
      id: 1,
      originalName: '企微同步-客户咨询-001.wav',
      contentType: 'audio/wav',
      sizeBytes: 4096,
      durationSeconds: 32,
      source: 'wecom_sync',
      messageId: 'wecom-dev-001',
      senderName: '张伟',
      receiverName: '陈经理',
      syncedAt: '2026-08-07T10:00:00+08:00',
      createdAt: '2026-08-07T10:00:00+08:00',
      playUrl: '/dashboard/chat/media/1/content',
    },
  ],
  total: 1,
  page: 1,
  perPage: 20,
};

describe('FileAudioPage', () => {
  it('centers the empty state with plain-language guidance', async () => {
    const api: FileAudioApi = { list: vi.fn().mockResolvedValue({ list: [], total: 0, page: 1, perPage: 20 }) };
    renderPage(api);

    const emptyState = await screen.findByRole('region', { name: '音频数据说明' });
    expect(emptyState.classList.contains('phase35-empty-state')).toBe(true);
    expect(screen.getByText('企业微信同步后，录音会自动出现在这里。')).not.toBeNull();
    expect(screen.queryByText(/Provider/i)).toBeNull();
  });

  it('renders synchronized recordings as a read-only table without upload or delete controls', async () => {
    const list = vi.fn().mockResolvedValue(sample);
    const api: FileAudioApi = { list };
    renderPage(api);

    expect(await screen.findByText('企微同步-客户咨询-001.wav')).not.toBeNull();
    expect(list).toHaveBeenCalledWith(1, 20, { sender: '', receiver: '', from: '', to: '' });
    expect(screen.queryByRole('button', { name: '上传录音' })).toBeNull();
    expect(screen.queryByRole('button', { name: /删除/ })).toBeNull();
    expect(screen.queryByText('人工上传')).toBeNull();
    expect(screen.getAllByText('企微同步').length).toBeGreaterThan(0);
    expect(screen.getByText('32 秒')).not.toBeNull();
    expect(screen.queryByText(/能力未接入|功能未接入/)).toBeNull();
  });

  it('queries sender, receiver and synced date range then resets active page', async () => {
    const list = vi.fn().mockResolvedValue(sample);
    const api: FileAudioApi = { list };
    renderPage(api, '/chat/file-audio?page=3');
    await screen.findByText('企微同步-客户咨询-001.wav');

    fireEvent.change(screen.getByLabelText('发送人'), { target: { value: '张伟' } });
    fireEvent.change(screen.getByLabelText('接收人'), { target: { value: '陈经理' } });
    fireEvent.change(screen.getByLabelText('发送日期-开始'), { target: { value: '2026-08-01' } });
    fireEvent.change(screen.getByLabelText('发送日期-结束'), { target: { value: '2026-08-20' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));

    await waitFor(() => expect(list).toHaveBeenLastCalledWith(1, 20, {
      sender: '张伟', receiver: '陈经理', from: '2026-08-01', to: '2026-08-20',
    }));
  });

  it('keeps the audio source authenticated and resets filters without write actions', async () => {
    const list = vi.fn().mockResolvedValue(sample);
    const api: FileAudioApi = { list };
    renderPage(api, '/chat/file-audio?sender=张伟&receiver=陈经理&from=2026-08-01&to=2026-08-20');
    await screen.findByText('企微同步-客户咨询-001.wav');

    const audio = document.querySelector('audio');
    expect(audio?.getAttribute('src')).toBe('/dashboard/chat/media/1/content');
    fireEvent.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(1, 20, { sender: '', receiver: '', from: '', to: '' }));
  });
});
