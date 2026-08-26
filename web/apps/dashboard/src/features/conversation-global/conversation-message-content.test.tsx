import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { ConversationMessageContent } from './conversation-message-content';

afterEach(cleanup);

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

  it('renders authenticated ready image, voice, video, and file media', () => {
    const { rerender } = render(<ConversationMessageContent type={2} content={{ media: { id: mediaID, type: 'image', name: '截图.png', mimeType: 'image/png', size: 12, status: 'ready', url: `/dashboard/archive/media/${mediaID}/content` } }} />);
    expect(screen.getByRole('img', { name: '截图.png' }).getAttribute('src')).toBe(`/dashboard/archive/media/${mediaID}/content`);

    rerender(<ConversationMessageContent type={3} content={{ media: { id: mediaID, type: 'voice', name: '语音.wav', mimeType: 'audio/wav', size: 13, status: 'ready', url: `/dashboard/archive/media/${mediaID}/content` } }} />);
    expect(screen.getByLabelText('播放语音.wav').getAttribute('src')).toBe(`/dashboard/archive/media/${mediaID}/content`);

    rerender(<ConversationMessageContent type={4} content={{ media: { id: mediaID, type: 'video', name: '视频.mp4', mimeType: 'video/mp4', size: 14, status: 'ready', url: `/dashboard/archive/media/${mediaID}/content` } }} />);
    expect(screen.getByLabelText('播放视频.mp4').getAttribute('src')).toBe(`/dashboard/archive/media/${mediaID}/content`);

    rerender(<ConversationMessageContent type={5} content={{ media: { id: mediaID, type: 'file', name: '合同.pdf', mimeType: 'application/pdf', size: 15, status: 'ready', url: `/dashboard/archive/media/${mediaID}/content` } }} />);
    expect(screen.getByRole('link', { name: '下载合同.pdf' }).getAttribute('href')).toBe(`/dashboard/archive/media/${mediaID}/content?download=1`);
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
