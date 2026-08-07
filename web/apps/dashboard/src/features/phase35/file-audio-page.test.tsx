import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardAccessProvider } from '../../app/access-context';
import { FileAudioPage } from './file-audio-page';
import type { FileAudioApi } from './file-audio-api';

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function renderPage(api: FileAudioApi) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const access = { corp: { id: '9' }, session: {}, menu: [], allowedRoutes: new Set(), allowedActions: new Set() } as never;
  return render(
    <DashboardAccessProvider value={access}>
      <QueryClientProvider client={client}>
        <FileAudioPage api={api} />
      </QueryClientProvider>
    </DashboardAccessProvider>,
  );
}

const sample = {
  list: [
    {
      id: 1,
      originalName: 'p36-accept.wav',
      contentType: 'audio/wav',
      sizeBytes: 4096,
      durationSeconds: 0,
      createdAt: '2026-08-07T10:00:00+08:00',
      playUrl: '/dashboard/chat/media/1/content',
    },
  ],
  total: 1,
  page: 1,
  perPage: 20,
};

describe('FileAudioPage', () => {
  it('loads the audio list for the current corp and renders playback controls', async () => {
    const list = vi.fn().mockResolvedValue(sample);
    const api: FileAudioApi = { list, upload: vi.fn(), remove: vi.fn() };
    renderPage(api);
    expect(await screen.findByText('p36-accept.wav')).not.toBeNull();
    expect(list).toHaveBeenCalledWith(9, 1, 20, '');
    expect(screen.getByText('WAV')).not.toBeNull();
    expect(screen.getByRole('button', { name: /删除 p36-accept.wav/ })).not.toBeNull();
    expect(document.querySelector('audio')?.getAttribute('src')).toBe('/dashboard/chat/media/1/content');
  });

  it('uploads a supported audio file and refreshes the list', async () => {
    const upload = vi.fn().mockResolvedValue({ id: 2, playUrl: '/dashboard/chat/media/2/content' });
    const list = vi.fn().mockResolvedValue(sample);
    const api: FileAudioApi = { list, upload, remove: vi.fn() };
    renderPage(api);
    await screen.findByText('p36-accept.wav');
    const file = new File(['wave'], 'new.wav', { type: 'audio/wav' });
    const input = screen.getByLabelText('选择音频文件') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [file] } });
    expect(screen.getByText('new.wav')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '上传' }));
    expect(await screen.findByText('上传成功，已写入存储并可回读播放')).not.toBeNull();
    expect(upload).toHaveBeenCalledWith(9, file);
  });

  it('rejects non-audio files client-side before calling the API', async () => {
    const upload = vi.fn();
    const api: FileAudioApi = { list: vi.fn().mockResolvedValue(sample), upload, remove: vi.fn() };
    renderPage(api);
    await screen.findByText('p36-accept.wav');
    const file = new File(['text'], 'notes.txt', { type: 'text/plain' });
    fireEvent.change(screen.getByLabelText('选择音频文件'), { target: { files: [file] } });
    expect(await screen.findByText('仅支持常见音频格式（如 WAV、MP3、AAC）')).not.toBeNull();
    expect(upload).not.toHaveBeenCalled();
  });

  it('keeps the upload button disabled until a file is chosen', async () => {
    const api: FileAudioApi = { list: vi.fn().mockResolvedValue(sample), upload: vi.fn(), remove: vi.fn() };
    renderPage(api);
    await screen.findByText('p36-accept.wav');
    expect((screen.getByRole('button', { name: '上传' }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText('未选择文件')).not.toBeNull();
  });

  it('soft deletes after confirmation', async () => {
    vi.stubGlobal('confirm', vi.fn(() => true));
    const remove = vi.fn().mockResolvedValue({ id: 1 });
    const list = vi.fn().mockResolvedValue(sample);
    const api: FileAudioApi = { list, upload: vi.fn(), remove };
    renderPage(api);
    fireEvent.click(await screen.findByRole('button', { name: /删除 p36-accept.wav/ }));
    await waitFor(() => expect(remove).toHaveBeenCalledWith(9, 1));
    expect(await screen.findByText('已删除')).not.toBeNull();
  });

  it('renders the empty explanation when there are no files', async () => {
    const api: FileAudioApi = {
      list: vi.fn().mockResolvedValue({ list: [], total: 0, page: 1, perPage: 20 }),
      upload: vi.fn(),
      remove: vi.fn(),
    };
    renderPage(api);
    expect(await screen.findByText('还没有可展示的音频文件')).not.toBeNull();
    expect(screen.getByText(/写入本地存储卷/)).not.toBeNull();
  });
});
