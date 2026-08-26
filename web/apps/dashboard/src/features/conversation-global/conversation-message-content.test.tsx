import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { ApiError } from '@mochat/api-client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ArchiveMediaClientProvider, type ArchiveMediaClient } from './archive-media-client';
import { ConversationMessageContent } from './conversation-message-content';

const objectURL = vi.fn<(blob: Blob) => string>();
const revokeObjectURL = vi.fn<(url: string) => void>();

beforeEach(() => {
  objectURL.mockReset();
  revokeObjectURL.mockReset();
  let sequence = 0;
  objectURL.mockImplementation(() => `blob:archive-media-${++sequence}`);
  Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: objectURL });
  Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL });
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('ConversationMessageContent', () => {
  const mediaID = '8ff7bf2d-5604-43bc-a600-3ec91d575085';
  it('renders legacy type 100 as a real text message', () => {
    render(<ConversationMessageContent type={100} content={{ text: '历史文本消息' }} />);
    expect(screen.getByText('历史文本消息')).toBeTruthy();
    expect(screen.queryByText('消息类型 100')).toBeNull();
  });

  it('renders only real image and file URLs', () => {
    const { rerender } = render(<ConversationMessageContent type={2} content={{ url: 'https://cdn.example/image.png' }} />);
    expect(screen.getByRole('img', { name: '会话图片' }).getAttribute('src')).toBe('https://cdn.example/image.png');
    rerender(<ConversationMessageContent type={5} content={{ name: '报价.pdf', url: 'https://cdn.example/quote.pdf' }} />);
    expect(screen.getByRole('link', { name: '报价.pdf' }).getAttribute('href')).toBe('https://cdn.example/quote.pdf');
  });

  it('shows a truthful unsupported state when required fields are absent', () => {
    render(<ConversationMessageContent type={5} content={{}} />);
    expect(screen.getByText('该类型内容暂不支持预览')).toBeTruthy();
  });

  it('Bearer-fetches ready previews into object URLs and never exposes a protected native URL', async () => {
    const download = vi.fn<ArchiveMediaClient['download']>().mockResolvedValue({ blob: new Blob(['image']), filename: '截图.png' });
    const api: ArchiveMediaClient = { download };
    const content = { media: { id: mediaID, type: 'image', name: '截图.png', mimeType: 'image/png', size: 12, status: 'ready', url: `/dashboard/archive/media/${mediaID}/content` } };
    const { unmount } = render(<ArchiveMediaClientProvider client={api}><ConversationMessageContent type={2} content={content} /></ArchiveMediaClientProvider>);
    expect(screen.getByRole('status').textContent).toContain('媒体加载中');
    expect(screen.queryByRole('img')).toBeNull();
    expect(document.body.innerHTML).not.toContain(`/dashboard/archive/media/${mediaID}/content`);
    const image = await screen.findByRole('img', { name: '截图.png' });
    expect(image.getAttribute('src')).toBe('blob:archive-media-1');
    expect(download).toHaveBeenCalledOnce();
    const [path, init] = download.mock.calls[0]!;
    expect(path).toBe(`/dashboard/archive/media/${mediaID}/content`);
    expect(init?.signal).toBeInstanceOf(AbortSignal);
    unmount();
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:archive-media-1');
  });

  it('revokes the previous preview on media changes and supports audio/video blobs', async () => {
    const api: ArchiveMediaClient = { download: vi.fn().mockResolvedValue({ blob: new Blob(['media']), filename: '媒体.bin' }) };
    const image = { media: { id: mediaID, type: 'image', name: '截图.png', mimeType: 'image/png', size: 12, status: 'ready', url: `/dashboard/archive/media/${mediaID}/content` } };
    const voiceID = '81596770-01df-4b51-b1dc-30f2f2379679';
    const voice = { media: { id: voiceID, type: 'voice', name: '语音.wav', mimeType: 'audio/wav', size: 13, status: 'ready', url: `/dashboard/archive/media/${voiceID}/content` } };
    const videoID = '2b91d440-a26e-482b-ad1b-8ee6f95b284f';
    const video = { media: { id: videoID, type: 'video', name: '视频.mp4', mimeType: 'video/mp4', size: 14, status: 'ready', url: `/dashboard/archive/media/${videoID}/content` } };
    const { rerender } = render(<ArchiveMediaClientProvider client={api}><ConversationMessageContent type={2} content={image} /></ArchiveMediaClientProvider>);
    await screen.findByRole('img');
    rerender(<ArchiveMediaClientProvider client={api}><ConversationMessageContent type={3} content={voice} /></ArchiveMediaClientProvider>);
    const audio = await screen.findByLabelText('播放语音.wav');
    expect(audio.getAttribute('src')).toBe('blob:archive-media-2');
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:archive-media-1');
    rerender(<ArchiveMediaClientProvider client={api}><ConversationMessageContent type={4} content={video} /></ArchiveMediaClientProvider>);
    const videoElement = await screen.findByLabelText('播放视频.mp4');
    expect(videoElement.getAttribute('src')).toBe('blob:archive-media-3');
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:archive-media-2');
  });

  it('downloads files through the authorized client with a safe temporary object URL', async () => {
    let complete!: (value: { blob: Blob; filename: string }) => void;
    const pending = new Promise<{ blob: Blob; filename: string }>((resolve) => { complete = resolve; });
    const download = vi.fn<ArchiveMediaClient['download']>(() => pending);
    const api: ArchiveMediaClient = { download };
    let clickedHref = '';
    let clickedName = '';
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function click(this: HTMLAnchorElement) {
      clickedHref = this.href;
      clickedName = this.download;
    });
    render(<ArchiveMediaClientProvider client={api}><ConversationMessageContent type={5} content={{ media: { id: mediaID, type: 'file', name: '合同.pdf', mimeType: 'application/pdf', size: 15, status: 'ready', url: `/dashboard/archive/media/${mediaID}/content` } }} /></ArchiveMediaClientProvider>);
    fireEvent.click(screen.getByRole('button', { name: '下载合同.pdf' }));
    expect(screen.getByRole('status').textContent).toContain('文件下载中');
    await waitFor(() => expect(download).toHaveBeenCalledOnce());
    const [path, init] = download.mock.calls[0]!;
    expect(path).toBe(`/dashboard/archive/media/${mediaID}/content?download=1`);
    expect(init?.signal).toBeInstanceOf(AbortSignal);
    complete({ blob: new Blob(['pdf']), filename: '../合同?.pdf' });
    await waitFor(() => expect(revokeObjectURL).toHaveBeenCalledWith('blob:archive-media-1'));
    expect(clickedHref).toBe('blob:archive-media-1');
    expect(clickedName).toBe('合同_.pdf');
  });

  it('shows an accessible 401 state and retries without exposing the protected URL', async () => {
    const download = vi.fn()
      .mockRejectedValueOnce(new ApiError('unauthorized', 'Unauthorized', { status: 401 }))
      .mockResolvedValueOnce({ blob: new Blob(['image']), filename: '截图.png' });
    const api: ArchiveMediaClient = { download };
    render(<ArchiveMediaClientProvider client={api}><ConversationMessageContent type={2} content={{ media: { id: mediaID, type: 'image', name: '截图.png', mimeType: 'image/png', size: 12, status: 'ready', url: `/dashboard/archive/media/${mediaID}/content` } }} /></ArchiveMediaClientProvider>);
    expect((await screen.findByRole('alert')).textContent).toContain('登录状态已失效');
    expect(document.body.innerHTML).not.toContain(`/dashboard/archive/media/${mediaID}/content`);
    fireEvent.click(screen.getByRole('button', { name: '重试加载截图.png' }));
    expect((await screen.findByRole('img', { name: '截图.png' })).getAttribute('src')).toBe('blob:archive-media-1');
    expect(download).toHaveBeenCalledTimes(2);
  });

  it('shows each durable media state truthfully and never exposes an SDK locator', () => {
    const { rerender } = render(<ConversationMessageContent type={2} content={{ sdkfileid: 'must-not-render', media: { id: 'pending-id', type: 'image', name: '图.png', mimeType: 'image/png', size: 0, status: 'pending' } }} />);
    expect(screen.getByRole('status').textContent).toContain('媒体处理中');
    expect(document.body.textContent).not.toContain('must-not-render');
    rerender(<ConversationMessageContent type={2} content={{ media: { id: 'failed-id', type: 'image', name: '图.png', mimeType: 'image/png', size: 0, status: 'failed', errorCode: 'archive.media_fetch_failed' } }} />);
    expect(screen.getByRole('alert').textContent).toContain('媒体读取失败');
    rerender(<ConversationMessageContent type={2} content={{ media: { id: 'missing-id', type: 'image', name: '图.png', mimeType: 'image/png', size: 0, status: 'missing' } }} />);
    expect(screen.getByRole('alert').textContent).toContain('媒体已缺失');
    rerender(<ConversationMessageContent type={2} content={{ media: { id: 'corrupt-id', type: 'image', name: '图.png', mimeType: 'image/png', size: 0, status: 'corrupt' } }} />);
    expect(screen.getByRole('alert').textContent).toContain('媒体已损坏');
  });

  it('fails safely when ready media has no authenticated URL and keeps legacy media compatible', () => {
    const { rerender } = render(<ConversationMessageContent type={5} content={{ media: { id: 'file-id', type: 'file', name: '合同.pdf', mimeType: 'application/pdf', size: 1, status: 'ready' } }} />);
    expect(screen.getByRole('alert').textContent).toContain('媒体暂不可用');
    rerender(<ConversationMessageContent type={3} content={{ voiceUrl: 'https://legacy.example/voice.wav' }} />);
    expect(screen.getByLabelText('播放会话语音').getAttribute('src')).toBe('https://legacy.example/voice.wav');
  });

  it('rejects a backslash network-path URL before it reaches the browser', () => {
    render(<ConversationMessageContent type={2} content={{ media: { id: 'image-id', type: 'image', name: '图.png', mimeType: 'image/png', size: 1, status: 'ready', url: '/\\attacker.example/image.png' } }} />);
    expect(screen.getByRole('alert').textContent).toContain('媒体暂不可用');
    expect(screen.queryByRole('img')).toBeNull();
  });
});
