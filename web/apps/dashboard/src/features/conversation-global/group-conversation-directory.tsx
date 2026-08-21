import type { FormEvent } from 'react';

import { DashboardPagination } from '../../components/dashboard-pagination';
import { PageState } from '../../components/page-state/page-state';
import type { GroupRoomDirectoryItem, GroupRoomDirectoryPage, GroupRoomMode } from './conversation-global-api';

type Props = {
  data: GroupRoomDirectoryPage | undefined;
  mode: GroupRoomMode;
  keyword: string;
  selectedRoomId: number | null;
  pending: boolean;
  error: Error | null;
  onKeywordChange: (value: string) => void;
  onSearch: (event: FormEvent<HTMLFormElement>) => void;
  onRefresh: () => void;
  onModeChange: (mode: GroupRoomMode) => void;
  onSelect: (room: GroupRoomDirectoryItem) => void;
  onPageChange: (page: number) => void;
  onOpenFilter: () => void;
};

function avatar(room: GroupRoomDirectoryItem) {
  return room.avatar.trim() !== '' ? <img alt="" src={room.avatar} /> : <span aria-hidden="true">{room.name.trim().slice(0, 1) || '群'}</span>;
}

function displayName(room: GroupRoomDirectoryItem) { return room.name.trim() || '未命名客户群'; }
function time(value: string) { return value.trim() === '' ? '暂无消息' : value; }

function RoomCard({ room, selected, onSelect }: { room: GroupRoomDirectoryItem; selected: boolean; onSelect: () => void }) {
  return <button aria-current={selected ? 'true' : undefined} aria-label={`${displayName(room)}，${room.memberCount} 名成员`} className={`group-conversation-room-card${selected ? ' is-selected' : ''}`} onClick={onSelect} type="button">
    <span className="group-conversation-room-avatar">{avatar(room)}</span>
    <span className="group-conversation-room-card-main"><strong>{displayName(room)}</strong><small>{room.employeeCount} 名员工 · {room.customerCount} 名客户</small><span>{room.lastMessage || '暂无可展示的归档消息'}</span></span>
    <span className="group-conversation-room-card-meta"><time>{time(room.lastMessageAt)}</time><em>{room.messageCount} 条</em></span>
  </button>;
}

export function GroupConversationDirectory(props: Props) {
  function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); props.onSearch(event); }
  return <section aria-label="群聊目录" className="group-conversation-directory">
    <header className="group-conversation-column-header"><div><p className="group-conversation-eyebrow">会话工作台</p><h1>客户群</h1></div><button aria-label="筛选群聊" className="group-conversation-icon-button" onClick={props.onOpenFilter} type="button">筛选</button></header>
    <form aria-label="群聊查询" className="group-conversation-search" onSubmit={submit} role="search">
      <label><span className="sr-only">搜索群聊</span><input aria-label="搜索群聊" onChange={(event) => props.onKeywordChange(event.target.value)} placeholder="搜索群名称" value={props.keyword} /></label>
      <button className="group-conversation-query-button" type="submit">查询</button><button aria-label="刷新群聊" className="group-conversation-refresh-button" disabled={props.pending} onClick={props.onRefresh} type="button">{props.pending ? '刷新中…' : '刷新'}</button>
    </form>
    <div aria-label="群聊状态" className="group-conversation-mode-tabs" role="tablist"><button aria-selected={props.mode === 'active'} onClick={() => props.onModeChange('active')} role="tab" type="button">进行中</button><button aria-selected={props.mode === 'dissolved'} onClick={() => props.onModeChange('dissolved')} role="tab" type="button">已解散</button></div>
    {props.data?.limitations.map((item) => <p className="group-conversation-limitation" key={item.key} role="alert">{item.reason}</p>)}
    <div className="group-conversation-directory-list">
      {props.pending && <PageState state="loading" title="正在加载群聊" />}
      {props.error !== null && <PageState state="error" title="群聊目录加载失败" description={props.error.message} onRetry={props.onRefresh} />}
      {!props.pending && props.error === null && props.data !== undefined && props.data.items.length === 0 && <PageState state="empty" title={props.mode === 'dissolved' ? '暂无已解散群聊' : '暂无客户群'} description="当前企业没有可展示的真实群聊数据。" />}
      {!props.pending && props.error === null && props.data !== undefined && props.data.items.length > 0 && props.data.items.map((room) => <RoomCard key={room.id} room={room} selected={room.id === props.selectedRoomId} onSelect={() => props.onSelect(room)} />)}
    </div>
    {props.data !== undefined && <DashboardPagination ariaLabel="群聊分页" page={props.data.page} pageSize={50} total={props.data.total} onPageChange={props.onPageChange} />}
  </section>;
}
