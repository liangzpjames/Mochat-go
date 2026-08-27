import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import { DashboardPagination } from '../../components/dashboard-pagination';
import { Phase35DataState } from './components/data-state';
import { ConversationOperationsShell, ConversationQueryBar } from '../conversation-operations/conversation-operations-shell';
import { displayValue, formatDateTime } from '../conversation-operations/conversation-operations-format';
import type { FileAudioApi } from './file-audio-api';

const pageSize = 20;

function positivePage(value: string | null): number {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 1;
}

export function FileAudioPage({ api }: { api: FileAudioApi }) {
  const [searchParams, setSearchParams] = useSearchParams();
  const page = positivePage(searchParams.get('page'));
  const appliedSender = searchParams.get('sender') ?? '';
  const appliedReceiver = searchParams.get('receiver') ?? '';
  const appliedFrom = searchParams.get('from') ?? '';
  const appliedTo = searchParams.get('to') ?? '';
  const [sender, setSender] = useState(appliedSender);
  const [receiver, setReceiver] = useState(appliedReceiver);
  const [from, setFrom] = useState(appliedFrom);
  const [to, setTo] = useState(appliedTo);

  const list = useQuery({
    queryKey: ['chat-media', page, appliedSender, appliedReceiver, appliedFrom, appliedTo],
    queryFn: () => api.list(page, pageSize, { sender: appliedSender, receiver: appliedReceiver, from: appliedFrom, to: appliedTo }),
  });
  const result = list.data;
  const items = result?.list ?? [];
  const total = Number(result?.total ?? 0);
  const [playbackUrls, setPlaybackUrls] = useState<Record<number, string>>({});

  useEffect(() => {
    if (api.content === undefined || items.length === 0) {
      return;
    }
    let cancelled = false;
    const urls: string[] = [];
    setPlaybackUrls({});
    void Promise.all(items.map(async (item) => {
      try {
        const blob = await api.content?.(item.id);
        if (blob === undefined || cancelled) return;
        const url = URL.createObjectURL(blob);
        urls.push(url);
        setPlaybackUrls((current) => ({ ...current, [item.id]: url }));
      } catch {
        // Keep the row visible; a failed media fetch does not change synced metadata.
      }
    }));
    return () => {
      cancelled = true;
      urls.forEach((url) => URL.revokeObjectURL(url));
    };
  }, [api, items]);

  const applyFilters = (): void => {
    const next = new URLSearchParams();
    if (sender.trim()) next.set('sender', sender.trim());
    if (receiver.trim()) next.set('receiver', receiver.trim());
    if (from) next.set('from', from);
    if (to) next.set('to', to);
    next.set('page', '1');
    setSearchParams(next);
  };

  const resetFilters = (): void => {
    setSender('');
    setReceiver('');
    setFrom('');
    setTo('');
    setSearchParams({ page: '1' });
  };

  const changePage = (nextPage: number): void => {
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      next.set('page', String(nextPage));
      return next;
    });
  };

  return (
    <ConversationOperationsShell
      title="文件录音"
      description="查询企业微信同步的音视频通话，播放使用登录鉴权地址"
    >
      <ConversationQueryBar fetching={list.isFetching} onQuery={applyFilters} onReset={resetFilters} onRefresh={() => void list.refetch()}>
        <label>发送人<input aria-label="按发送人姓名搜索" value={sender} placeholder="按发送人姓名搜索" onChange={(event) => setSender(event.target.value)} /></label>
        <label>接收人<input aria-label="按接收人姓名搜索" value={receiver} placeholder="按接收人姓名搜索" onChange={(event) => setReceiver(event.target.value)} /></label>
        <label>发送日期-开始<input aria-label="发送日期-开始" type="date" value={from} onChange={(event) => setFrom(event.target.value)} onInput={(event) => setFrom(event.currentTarget.value)} /></label>
        <label>发送日期-结束<input aria-label="发送日期-结束" type="date" value={to} onChange={(event) => setTo(event.target.value)} onInput={(event) => setTo(event.currentTarget.value)} /></label>
      </ConversationQueryBar>

      <section className="conversation-operations-card">
        <header className="phase35-card-header">
          <div><h2>音视频通话</h2><p>仅展示来自企业微信同步的录音数据</p></div>
          <span className="phase35-chip">共 {total} 条</span>
        </header>
        <Phase35DataState
          loading={list.isLoading}
          error={list.isError}
          empty={!list.isLoading && !items.length}
          emptyContent={<section aria-label="音频数据说明" className="phase35-empty-state"><h2>暂无同步录音</h2><p>企业微信同步后，录音会自动出现在这里。</p></section>}
          onRetry={() => void list.refetch()}
        >
          <div className="conversation-operations-table-wrap">
            <table>
              <thead><tr><th>音视频通话</th><th>发送人</th><th>接收人</th><th>发送时间</th><th>时长</th><th>来源</th><th>播放</th></tr></thead>
              <tbody>{items.map((item) => (
                <tr key={item.id}>
                  <td>{displayValue(item.originalName)}</td>
                  <td>{displayValue(item.senderName)}</td>
                  <td>{displayValue(item.receiverName)}</td>
                  <td>{formatDateTime(item.syncedAt ?? item.createdAt)}</td>
                  <td>{item.durationSeconds > 0 ? `${item.durationSeconds} 秒` : '--'}</td>
                  <td>{item.source === 'wecom_sync' ? '企微同步' : displayValue(item.source)}</td>
                  <td><audio controls preload="metadata" aria-label={`播放 ${item.originalName}`} src={api.content === undefined ? item.playUrl : playbackUrls[item.id]} /></td>
                </tr>
              ))}</tbody>
            </table>
          </div>
          <DashboardPagination page={page} pageSize={pageSize} total={total} onPageChange={changePage} />
        </Phase35DataState>
      </section>
    </ConversationOperationsShell>
  );
}
