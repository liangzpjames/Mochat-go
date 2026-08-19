import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { ConversationMessageContent } from './conversation-message-content';

afterEach(cleanup);

describe('ConversationMessageContent', () => {
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
});
