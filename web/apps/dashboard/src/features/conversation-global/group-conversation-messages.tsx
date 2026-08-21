import { useMemo, type FormEvent } from 'react';

import { PageState } from '../../components/page-state/page-state';
import type { GroupRoomDirectoryItem, GroupRoomMessage, GroupRoomMessages } from './conversation-global-api';
import { ConversationMessageContent } from './conversation-message-content';

type Props = {
  room: GroupRoomDirectoryItem | undefined;
  data: GroupRoomMessages | undefined;
  keyword: string;
  date: string;
  messageTypes: readonly string[];
  pending: boolean;
  fetching: boolean;
  loadingOlder: boolean;
  error: Error | null;
  onKeywordChange: (value: string) => void;
  onDateChange: (value: string) => void;
  onToggleMessageType: (value: string) => void;
  onSearch: (event: FormEvent<HTMLFormElement>) => void;
  onRefresh: () => void;
  onLoadOlder: () => void;
  onOpenProfile?: () => void;
};

const messageTypes = [['1', '文字'], ['2', '图片'], ['3', '语音'], ['4', '视频'], ['5', '文件'], ['6', '链接']] as const;
function roomName(room: GroupRoomDirectoryItem | undefined) { return room?.name.trim() || '未命名客户群'; }
function day(value: string) { return value.length >= 10 ? value.slice(0, 10) : '未标注日期'; }

function MessageRow({ message }: { message: GroupRoomMessage }) {
  return <article className={`group-conversation-message ${message.direction}`}><header><span className="group-conversation-message-avatar">{message.senderName.trim().slice(0, 1) || '群'}</span><strong>{message.senderName.trim() || '群成员'}</strong><small>{message.senderKind === 'employee' ? '员工' : '客户'}</small><time>{message.sentAt}</time></header><div className="group-conversation-message-body"><ConversationMessageContent content={message.content} type={message.type} /></div></article>;
}

export function GroupConversationMessages(props: Props) {
  const grouped = useMemo(() => {
    const result: Array<{ label: string; messages: GroupRoomMessage[] }> = [];
    for (const message of props.data?.messages ?? []) {
      const label = day(message.sentAt); const last = result[result.length - 1];
      if (last?.label === label) last.messages.push(message); else result.push({ label, messages: [message] });
    }
    return result;
  }, [props.data?.messages]);
  function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); props.onSearch(event); }
  if (props.room === undefined) return <section aria-label="群消息" className="group-conversation-messages group-conversation-empty-column"><PageState state="empty" title="选择一个客户群" description="从左侧群聊目录选择会话，查看真实归档消息。" /></section>;
  return <section aria-label="群消息" className="group-conversation-messages"><header className="group-conversation-message-header"><div><p className="group-conversation-eyebrow">群消息</p><h2>{roomName(props.room)}</h2><span>{props.room.memberCount} 名成员 · {props.room.messageCount} 条已归档消息</span></div><div className="group-conversation-message-header-actions">{props.onOpenProfile && <button aria-label="查看群资料" className="group-conversation-icon-button" onClick={props.onOpenProfile} type="button">资料</button>}<button aria-label="刷新群消息" className="group-conversation-icon-button" disabled={props.fetching} onClick={props.onRefresh} type="button">{props.fetching ? '刷新中…' : '刷新'}</button></div></header>
    <section aria-label="群消息统计" className="group-conversation-message-stats"><article><span>消息总数</span><strong>{props.data?.stats.messageTotal ?? '--'}</strong></article><article><span>员工发送</span><strong>{props.data?.stats.employeeTotal ?? '--'}</strong></article><article><span>客户发送</span><strong>{props.data?.stats.customerTotal ?? '--'}</strong></article><article><span>风险消息</span><strong>{props.data?.stats.riskTotal ?? '--'}</strong></article></section>
    <div className="group-conversation-message-filters"><form aria-label="群消息查询" onSubmit={submit} role="search"><input aria-label="搜索群消息" onChange={(event) => props.onKeywordChange(event.target.value)} placeholder="搜索消息内容" value={props.keyword} /><button aria-label="查询消息" className="group-conversation-query-button" type="submit">查询</button></form><input aria-label="群消息日期" onChange={(event) => props.onDateChange(event.target.value)} type="date" value={props.date} /><div aria-label="消息类型" className="group-conversation-message-types" role="group"><label><input checked={props.messageTypes.length === 0} onChange={() => undefined} type="checkbox" />全部</label>{messageTypes.map(([value, label]) => <label key={value}><input aria-label={label} checked={props.messageTypes.includes(value)} onChange={() => props.onToggleMessageType(value)} type="checkbox" />{label}</label>)}</div></div>
    {props.pending && <PageState state="loading" title="正在读取群消息" />}{props.error !== null && <PageState state="error" title="群消息加载失败" description={props.error.message} onRetry={props.onRefresh} />}
    {!props.pending && props.error === null && props.data !== undefined && props.data.capabilities.some((item) => !item.available) && <p className="group-conversation-limitation">{props.data.capabilities.filter((item) => !item.available).map((item) => item.reason ?? item.key).join('；')}</p>}
    {!props.pending && props.error === null && props.data !== undefined && props.data.messages.length === 0 && <PageState state="empty" title="该群暂无已归档消息" description="群资料存在，但当前筛选条件下没有真实存档内容。" />}
    {props.data !== undefined && props.data.messages.length > 0 && <div className="group-conversation-message-scroll">{props.data.hasMore && <button className="group-conversation-load-older" disabled={props.loadingOlder} onClick={props.onLoadOlder} type="button">{props.loadingOlder ? '加载中…' : '加载更早消息'}</button>}{grouped.map((group) => <section className="group-conversation-message-day" key={group.label}><h3>{group.label}</h3>{group.messages.map((message) => <MessageRow key={message.id} message={message} />)}</section>)}</div>}
  </section>;
}
