type Props = { type: number; content: Record<string, unknown> };

type MediaState = 'pending' | 'fetching' | 'ready' | 'failed' | 'missing' | 'corrupt';
type MessageMedia = { id: string; type: string; name: string; mimeType: string; size: number; status: MediaState; url?: string };

function textValue(content: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = content[key];
    if (typeof value === 'string' && value.trim() !== '') return value.trim();
  }
  return '';
}

const unsupported = <span className="conversation-message-unsupported">该类型内容暂不支持预览</span>;

function safeURL(value: string): string {
  if (value.startsWith('/') && !value.startsWith('//') && !value.includes('\\')) return value;
  try {
    const parsed = new URL(value);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? value : '';
  } catch {
    return '';
  }
}

function archiveMediaURL(id: string, value: string): string {
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(id)) return '';
  const expected = `/dashboard/archive/media/${id}/content`;
  return value === expected ? expected : '';
}

function messageMedia(content: Record<string, unknown>): MessageMedia | null {
  const value = content.media;
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return null;
  const media = value as Record<string, unknown>;
  const states: readonly MediaState[] = ['pending', 'fetching', 'ready', 'failed', 'missing', 'corrupt'];
  if (typeof media.id !== 'string' || typeof media.type !== 'string' || typeof media.name !== 'string'
    || typeof media.mimeType !== 'string' || typeof media.size !== 'number'
    || !states.includes(media.status as MediaState)) return null;
  const result: MessageMedia = { id: media.id, type: media.type, name: media.name, mimeType: media.mimeType, size: media.size, status: media.status as MediaState };
  if (typeof media.url === 'string') result.url = archiveMediaURL(media.id, media.url);
  return result;
}

function MediaStateView({ media }: { media: MessageMedia }) {
  if (media.status === 'pending' || media.status === 'fetching') return <span aria-live="polite" role="status">媒体处理中</span>;
  if (media.status === 'failed') return <span role="alert">媒体读取失败，请稍后重试</span>;
  if (media.status === 'missing') return <span role="alert">媒体已缺失</span>;
  if (media.status === 'corrupt') return <span role="alert">媒体已损坏</span>;
  if (!media.url) return <span role="alert">媒体暂不可用</span>;
  const label = media.name.trim() || '会话媒体';
  if (media.type === 'image') return <a href={media.url} rel="noreferrer" target="_blank"><img alt={label} loading="lazy" src={media.url} /></a>;
  if (media.type === 'voice' || media.type === 'audio') return <audio aria-label={`播放${label}`} controls preload="none" src={media.url}>浏览器不支持音频播放</audio>;
  if (media.type === 'video') return <video aria-label={`播放${label}`} controls preload="metadata" src={media.url}>浏览器不支持视频播放</video>;
  if (media.type === 'file') return <a href={`${media.url}${media.url.includes('?') ? '&' : '?'}download=1`} rel="noreferrer" target="_blank">下载{label}</a>;
  return <span role="alert">该媒体类型暂不支持预览</span>;
}

export function ConversationMessageContent({ type, content }: Props) {
  const media = messageMedia(content);
  if (media !== null) return <MediaStateView media={media} />;
  if (type === 1 || type === 100) {
    const text = textValue(content, 'text', 'content');
    return text === '' ? unsupported : <p>{text}</p>;
  }
  if (type === 2) {
    const url = safeURL(textValue(content, 'url', 'imageUrl', 'image_url'));
    return url === '' ? unsupported : <a href={url} rel="noreferrer" target="_blank"><img alt="会话图片" loading="lazy" src={url} /></a>;
  }
  if (type === 3) {
    const url = safeURL(textValue(content, 'url', 'voiceUrl', 'voice_url'));
    return url === '' ? unsupported : <audio aria-label="播放会话语音" controls preload="none" src={url}>浏览器不支持音频播放</audio>;
  }
  if (type === 4) {
    const url = safeURL(textValue(content, 'url', 'videoUrl', 'video_url'));
    return url === '' ? unsupported : <video aria-label="播放会话视频" controls preload="metadata" src={url}>浏览器不支持视频播放</video>;
  }
  if (type === 5) {
    const url = safeURL(textValue(content, 'url', 'fileUrl', 'file_url'));
    const name = textValue(content, 'name', 'fileName', 'filename');
    return url === '' || name === '' ? unsupported : <a href={url} rel="noreferrer" target="_blank">{name}</a>;
  }
  if (type === 6) {
    const url = safeURL(textValue(content, 'url', 'linkUrl', 'link_url'));
    const title = textValue(content, 'title', 'name') || url;
    return url === '' ? unsupported : <a href={url} rel="noreferrer" target="_blank">{title}</a>;
  }
  const fallback = textValue(content, 'text', 'content', 'title', 'name');
  return <div><small>消息类型 {type}</small>{fallback === '' ? unsupported : <p>{fallback}</p>}</div>;
}
