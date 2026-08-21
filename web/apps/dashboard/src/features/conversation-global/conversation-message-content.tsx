type Props = { type: number; content: Record<string, unknown> };

function textValue(content: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = content[key];
    if (typeof value === 'string' && value.trim() !== '') return value.trim();
  }
  return '';
}

const unsupported = <span className="conversation-message-unsupported">该类型内容暂不支持预览</span>;

export function ConversationMessageContent({ type, content }: Props) {
  if (type === 1 || type === 100) {
    const text = textValue(content, 'text', 'content');
    return text === '' ? unsupported : <p>{text}</p>;
  }
  if (type === 2) {
    const url = textValue(content, 'url', 'imageUrl', 'image_url');
    return url === '' ? unsupported : <a href={url} rel="noreferrer" target="_blank"><img alt="会话图片" loading="lazy" src={url} /></a>;
  }
  if (type === 3) {
    const url = textValue(content, 'url', 'voiceUrl', 'voice_url');
    return url === '' ? unsupported : <audio controls preload="none" src={url}>浏览器不支持音频播放</audio>;
  }
  if (type === 4) {
    const url = textValue(content, 'url', 'videoUrl', 'video_url');
    return url === '' ? unsupported : <video controls preload="metadata" src={url}>浏览器不支持视频播放</video>;
  }
  if (type === 5) {
    const url = textValue(content, 'url', 'fileUrl', 'file_url');
    const name = textValue(content, 'name', 'fileName', 'filename');
    return url === '' || name === '' ? unsupported : <a href={url} rel="noreferrer" target="_blank">{name}</a>;
  }
  if (type === 6) {
    const url = textValue(content, 'url', 'linkUrl', 'link_url');
    const title = textValue(content, 'title', 'name') || url;
    return url === '' ? unsupported : <a href={url} rel="noreferrer" target="_blank">{title}</a>;
  }
  const fallback = textValue(content, 'text', 'content', 'title', 'name');
  return <div><small>消息类型 {type}</small>{fallback === '' ? unsupported : <p>{fallback}</p>}</div>;
}
