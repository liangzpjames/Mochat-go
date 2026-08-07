import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';

import { useOptionalDashboardAccess } from '../../app/access-context';
import { Phase35DataState } from './components/data-state';
import { Phase35PageShell } from './components/phase35-page-shell';
import type { AudioObject, FileAudioApi } from './file-audio-api';

const maxAudioBytes = 50 * 1024 * 1024;
const pageSize = 20;

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '--';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(value: string): string {
  if (!value) return '--';
  return value.slice(0, 19).replace('T', ' ');
}

function formatType(value: string): string {
  const map: Record<string, string> = {
    'audio/wav': 'WAV',
    'audio/mpeg': 'MP3',
    'audio/mp4': 'M4A',
    'audio/aac': 'AAC',
    'audio/ogg': 'OGG',
    'audio/flac': 'FLAC',
  };
  if (map[value]) return map[value];
  if (value.startsWith('audio/')) return value.slice(6).toUpperCase();
  return '音频';
}

function messageText(error: unknown): string {
  return error instanceof Error ? error.message : '操作失败';
}

export function FileAudioPage({ api }: { api: FileAudioApi }) {
  const corpId = Number(useOptionalDashboardAccess()?.corp.id ?? 0);
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState('');
  const [appliedKeyword, setAppliedKeyword] = useState('');
  const [selectedName, setSelectedName] = useState('');
  const [notice, setNotice] = useState<{ kind: 'error' | 'success'; text: string } | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const queryClient = useQueryClient();

  const list = useQuery({
    queryKey: ['chat-media', corpId, page, appliedKeyword],
    queryFn: () => api.list(corpId, page, pageSize, appliedKeyword),
    enabled: corpId > 0,
  });
  const result = list.data;
  const items = result?.list ?? [];
  const total = Number(result?.total ?? 0);

  const refresh = (): void => {
    void queryClient.invalidateQueries({ queryKey: ['chat-media', corpId] });
  };

  const upload = useMutation({
    mutationFn: (file: File) => api.upload(corpId, file),
    onSuccess: () => {
      setNotice({ kind: 'success', text: '上传成功，已写入存储并可回读播放' });
      if (fileInput.current !== null) {
        fileInput.current.value = '';
      }
      setSelectedName('');
      setPage(1);
      refresh();
    },
    onError: (error) => setNotice({ kind: 'error', text: messageText(error) }),
  });

  const remove = useMutation({
    mutationFn: (id: number) => api.remove(corpId, id),
    onSuccess: () => {
      setNotice({ kind: 'success', text: '已删除' });
      refresh();
    },
    onError: (error) => setNotice({ kind: 'error', text: messageText(error) }),
  });

  const handleFile = (file: File | undefined): void => {
    if (file === undefined) return;
    if (!file.type.startsWith('audio/')) {
      setNotice({ kind: 'error', text: '仅支持常见音频格式（如 WAV、MP3、AAC）' });
      setSelectedName('');
      return;
    }
    if (file.size > maxAudioBytes) {
      setNotice({ kind: 'error', text: '上传文件超过 50MB 限制' });
      setSelectedName('');
      return;
    }
    setSelectedName(file.name);
  };

  const submitUpload = (): void => {
    const file = fileInput.current?.files?.[0];
    if (file !== undefined && selectedName !== '') {
      upload.mutate(file);
    }
  };

  const handleDelete = (item: AudioObject): void => {
    if (window.confirm(`确认删除「${item.originalName}」？删除后列表不再展示。`)) {
      remove.mutate(item.id);
    }
  };

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <Phase35PageShell
      title="文件录音"
      description="集中管理会话文件与录音：上传后可在线播放或下载"
      actions={<span className="phase35-chip">存储：已接入</span>}
    >
      <div className="phase35-page">
        <section className="phase35-card phase35-filter-card">
          <form
            className="dashboard-filter-bar"
            onSubmit={(event) => {
              event.preventDefault();
              setPage(1);
              setAppliedKeyword(keyword.trim());
            }}
          >
            <label>
              文件名
              <input aria-label="文件名" value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="按文件名筛选" />
            </label>
            <button type="submit">查询</button>
            <button
              type="button"
              onClick={() => {
                setKeyword('');
                setAppliedKeyword('');
                setPage(1);
              }}
            >
              重置
            </button>
          </form>
        </section>

        <section className="phase35-card phase35-filter-card">
          <header className="phase35-card-header">
            <div>
              <h2>上传音频</h2>
              <p>支持常见音频格式（WAV、MP3、AAC 等），单个文件不超过 50MB</p>
            </div>
          </header>
          <div className="dashboard-filter-bar phase35-file-picker">
            <input
              ref={fileInput}
              type="file"
              accept="audio/*"
              aria-label="选择音频文件"
              className="phase35-file-input-hidden"
              onChange={(event) => handleFile(event.target.files?.[0])}
            />
            <span className="phase35-file-name">{selectedName === '' ? '未选择文件' : selectedName}</span>
            <button type="button" disabled={selectedName === ''} onClick={() => fileInput.current?.click()}>
              选择文件
            </button>
            <button
              type="button"
              disabled={selectedName === '' || upload.isPending}
              onClick={submitUpload}
            >
              {upload.isPending ? '上传中…' : '上传'}
            </button>
          </div>
          {notice !== null && (
            <p className={notice.kind === 'error' ? 'phase35-notice-error' : 'phase35-notice-success'} role={notice.kind === 'error' ? 'alert' : 'status'}>
              {notice.text}
            </p>
          )}
        </section>

        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header">
            <div>
              <h2>音频文件列表</h2>
              <p>真实落盘文件，播放使用鉴权 URL</p>
            </div>
            <span className="phase35-chip">共 {total} 条</span>
          </header>
          <Phase35DataState
            loading={list.isLoading}
            error={list.isError}
            empty={!list.isLoading && !items.length}
            emptyContent={
              <section aria-label="音频数据说明">
                <h2>还没有可展示的音频文件</h2>
                <p>上传后文件会写入本地存储卷，并在此列表回读；播放链接经过登录鉴权。</p>
              </section>
            }
            onRetry={() => void list.refetch()}
          >
            <table>
              <thead>
                <tr>
                  <th>文件名</th>
                  <th>类型</th>
                  <th>大小</th>
                  <th>上传时间</th>
                  <th>播放</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>{item.originalName || `--`}</td>
                  <td>{formatType(item.contentType || '')}</td>
                    <td>{formatBytes(item.sizeBytes)}</td>
                    <td>{formatDate(item.createdAt)}</td>
                    <td>
                      <audio controls preload="none" src={item.playUrl}>
                        当前浏览器不支持音频播放
                      </audio>
                    </td>
                    <td>
                      <button type="button" aria-label={`删除 ${item.originalName}`} onClick={() => handleDelete(item)} disabled={remove.isPending}>
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div className="dashboard-pagination">
              <span>
                共 {total} 条，第 {page} / {totalPages} 页
              </span>
              <button type="button" disabled={page <= 1} onClick={() => setPage(page - 1)}>
                上一页
              </button>
              <button type="button" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
                下一页
              </button>
            </div>
          </Phase35DataState>
        </section>
      </div>
    </Phase35PageShell>
  );
}
