import { useEffect, useRef, useState } from 'react';
import { ApiError } from '@mochat/api-client';

import { useArchiveMediaClient } from './archive-media-client';

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

type PreviewState =
  | { key: string; status: 'idle' | 'loading' }
  | { key: string; status: 'ready'; objectURL: string }
  | { key: string; status: 'error'; unauthorized: boolean };

function isUnauthorized(error: unknown): boolean {
  return error instanceof ApiError && (error.kind === 'unauthorized' || error.status === 401);
}

function useArchiveMediaPreview(media: MessageMedia, enabled: boolean, retry: number): PreviewState {
  const client = useArchiveMediaClient();
  const key = `${media.id}:${media.url ?? ''}:${retry}`;
  const [state, setState] = useState<PreviewState>({ key, status: enabled ? 'loading' : 'idle' });
  const current = state.key === key ? state : { key, status: enabled ? 'loading' : 'idle' } as PreviewState;
  useEffect(() => {
    if (!enabled) {
      setState({ key, status: 'idle' });
      return undefined;
    }
    if (client === null || !media.url) {
      setState({ key, status: 'error', unauthorized: false });
      return undefined;
    }
    const controller = new AbortController();
    let objectURL = '';
    setState({ key, status: 'loading' });
    void client.download(media.url, { signal: controller.signal }).then(({ blob }) => {
      if (controller.signal.aborted) return;
      objectURL = URL.createObjectURL(blob);
      if (controller.signal.aborted) {
        URL.revokeObjectURL(objectURL);
        objectURL = '';
        return;
      }
      setState({ key, status: 'ready', objectURL });
    }).catch((error: unknown) => {
      if (controller.signal.aborted) return;
      if (objectURL !== '') {
        URL.revokeObjectURL(objectURL);
        objectURL = '';
      }
      setState({ key, status: 'error', unauthorized: isUnauthorized(error) });
    });
    return () => {
      controller.abort();
      if (objectURL !== '') URL.revokeObjectURL(objectURL);
    };
  }, [client, enabled, key, media.url]);
  return current;
}

function safeDownloadFilename(raw: string, fallback: string): string {
  const sanitize = (value: string) => {
    const basename = value.trim().split(/[\\/]/).pop() ?? '';
    // eslint-disable-next-line no-control-regex -- downloaded filenames must reject C0 controls.
    return basename.replace(/[\u0000-\u001f\u007f\\/:*?"<>|]/g, '_').trim();
  };
  let candidate = sanitize(raw);
  if (candidate === '' || candidate === '.' || candidate === '..') candidate = sanitize(fallback);
  if (candidate === '' || candidate === '.' || candidate === '..') candidate = '会话媒体';
  if (/^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(candidate)) candidate = `_${candidate}`;
  return Array.from(candidate).slice(0, 180).join('');
}

function ArchiveMediaDownload({ media, label }: { media: MessageMedia; label: string }) {
  const client = useArchiveMediaClient();
  const mounted = useRef(true);
  const activeRequest = useRef<AbortController | null>(null);
  const [status, setStatus] = useState<'idle' | 'loading' | 'error' | 'unauthorized'>('idle');
  useEffect(() => () => {
    mounted.current = false;
    activeRequest.current?.abort();
  }, []);
  const download = async () => {
    if (client === null || !media.url || status === 'loading') {
      setStatus('error');
      return;
    }
    const controller = new AbortController();
    activeRequest.current?.abort();
    activeRequest.current = controller;
    setStatus('loading');
    let objectURL = '';
    try {
      const result = await client.download(`${media.url}?download=1`, { signal: controller.signal });
      if (controller.signal.aborted) return;
      objectURL = URL.createObjectURL(result.blob);
      const anchor = document.createElement('a');
      anchor.href = objectURL;
      anchor.download = safeDownloadFilename(result.filename, label);
      anchor.rel = 'noreferrer';
      document.body.append(anchor);
      anchor.click();
      anchor.remove();
      if (mounted.current) setStatus('idle');
    } catch (error) {
      if (!controller.signal.aborted && mounted.current) setStatus(isUnauthorized(error) ? 'unauthorized' : 'error');
    } finally {
      if (objectURL !== '') URL.revokeObjectURL(objectURL);
      if (activeRequest.current === controller) activeRequest.current = null;
    }
  };
  return <span>
    <button disabled={status === 'loading'} onClick={() => void download()} type="button">下载{label}</button>
    {status === 'loading' ? <span aria-live="polite" role="status">文件下载中</span> : null}
    {status === 'unauthorized' ? <span role="alert">登录状态已失效，请重新登录后重试</span> : null}
    {status === 'error' ? <span role="alert">文件下载失败，请重试</span> : null}
  </span>;
}

function MediaStateView({ media }: { media: MessageMedia }) {
  const preview = media.status === 'ready' && ['image', 'voice', 'audio', 'video'].includes(media.type) && Boolean(media.url);
  const [retry, setRetry] = useState(0);
  const previewState = useArchiveMediaPreview(media, preview, retry);
  if (media.status === 'pending' || media.status === 'fetching') return <span aria-live="polite" role="status">媒体处理中</span>;
  if (media.status === 'failed') return <span role="alert">媒体读取失败，请稍后重试</span>;
  if (media.status === 'missing') return <span role="alert">媒体已缺失</span>;
  if (media.status === 'corrupt') return <span role="alert">媒体已损坏</span>;
  if (!media.url) return <span role="alert">媒体暂不可用</span>;
  const label = media.name.trim() || '会话媒体';
  if (preview) {
    if (previewState.status === 'loading' || previewState.status === 'idle') return <span aria-live="polite" role="status">媒体加载中</span>;
    if (previewState.status === 'error') return <span role="alert">
      {previewState.unauthorized ? '登录状态已失效，请重新登录后重试' : '媒体加载失败，请重试'}
      <button onClick={() => setRetry((value) => value + 1)} type="button">重试加载{label}</button>
    </span>;
    if (previewState.status !== 'ready') return <span role="alert">媒体加载失败，请重试</span>;
    if (media.type === 'image') return <a href={previewState.objectURL} rel="noreferrer" target="_blank"><img alt={label} loading="lazy" src={previewState.objectURL} /></a>;
    if (media.type === 'voice' || media.type === 'audio') return <audio aria-label={`播放${label}`} controls preload="none" src={previewState.objectURL}>浏览器不支持音频播放</audio>;
    return <video aria-label={`播放${label}`} controls preload="metadata" src={previewState.objectURL}>浏览器不支持视频播放</video>;
  }
  if (media.type === 'file') return <ArchiveMediaDownload label={label} media={media} />;
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
