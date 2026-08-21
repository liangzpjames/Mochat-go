import type { FormEvent } from 'react';

import type { StaffConversationDetail } from './conversation-global-api';
import { PageState } from '../../components/page-state/page-state';
import { ConversationMessageContent } from './conversation-message-content';

type Props = {
  data: StaffConversationDetail | undefined;
  messages: StaffConversationDetail['messages'];
  keyword: string;
  date: string;
  messageTypes: readonly string[];
  pending: boolean;
  fetching: boolean;
  loadingOlder: boolean;
  error: Error | null;
  focusPending: boolean;
  focusError: string | null;
  hasMore: boolean;
  onKeywordChange: (value: string) => void;
  onKeywordSearch: (event: FormEvent<HTMLFormElement>) => void;
  onDateChange: (value: string) => void;
  onToggleMessageType: (value: string) => void;
  onRefresh: () => void;
  onLoadOlder: () => void;
  onToggleFocus: () => void;
};

const messageTypeOptions = [['text', '文字'], ['image', '图片'], ['voice', '语音'], ['video', '视频'], ['file', '文件'], ['link', '链接']] as const;

function stat(value: number | null) { return value === null ? '--' : value.toLocaleString(); }
function displayTargetName(data: Pick<StaffConversationDetail, 'targetType' | 'targetId' | 'targetName'>) {
  if (data.targetName.trim()) return data.targetName;
  if (data.targetType === 'room' && data.targetId === 0) return '客户群';
  return `会话 ${data.targetId}`;
}

export function EmployeeConversationDetail(props: Props) {
  return <section aria-label="会话详情" className="employee-conversation-pane employee-conversation-detail">
    {props.data === undefined && !props.pending && props.error === null && <PageState state="empty" title="请选择会话" description="选择中间轨迹中的会话后，在这里阅读完整上下文。" />}
    {props.pending && <PageState state="loading" title="正在读取会话" />}
    {props.error !== null && <PageState state="error" title="会话详情加载失败" description={props.error.message} onRetry={props.onRefresh} />}
    {props.data !== undefined && <>
      <header className="employee-conversation-detail-header">
        <div><small>{props.data.targetType === 'customer' ? '客户会话' : props.data.targetType === 'room' ? '客户群会话' : '员工会话'}</small><strong>{displayTargetName(props.data)}</strong><span>{props.data.employeeName}</span></div>
        <div><button aria-label="刷新详情" disabled={props.fetching} onClick={props.onRefresh} type="button">刷新</button><button className={props.data.focused ? 'is-focused' : ''} disabled={props.focusPending} onClick={props.onToggleFocus} type="button">{props.data.focused ? '取消关注' : '重点关注'}</button></div>
      </header>
      {props.focusError && <p className="employee-conversation-limitation" role="alert">{props.focusError}</p>}
      <section aria-label="会话统计" className="employee-conversation-stats">
        <article><span>沟通天数</span><strong>{stat(props.data.stats.communicationDays)}</strong><small>天</small></article>
        <article><span>消息总数</span><strong>{stat(props.data.stats.messageTotal)}</strong><small>条</small></article>
        <article><span>客户消息</span><strong>{stat(props.data.stats.inboundTotal)}</strong><small>条</small></article>
        <article><span>员工消息</span><strong>{stat(props.data.stats.outboundTotal)}</strong><small>条</small></article>
      </section>
      <div className="employee-conversation-detail-filters">
        <div aria-label="消息类型筛选" className="employee-conversation-message-types" role="group">
          {messageTypeOptions.map(([value, label]) => <label key={value}><input aria-label={label} checked={props.messageTypes.includes(value)} onChange={() => props.onToggleMessageType(value)} type="checkbox" /><span>{label}</span></label>)}
        </div>
        <form aria-label="消息内容查询" className="employee-conversation-keyword-search" onSubmit={props.onKeywordSearch}>
          <input aria-label="搜索会话内容" onChange={(event) => props.onKeywordChange(event.target.value)} placeholder="搜索会话内容" value={props.keyword} />
          <button aria-label="查询会话内容" type="submit">查询</button>
        </form>
        <input aria-label="检索日期" onChange={(event) => props.onDateChange(event.target.value)} type="date" value={props.date} />
      </div>
      <div className="employee-conversation-scroll employee-conversation-messages">
        {props.hasMore && <button className="employee-conversation-load-more" disabled={props.loadingOlder} onClick={props.onLoadOlder} type="button">{props.loadingOlder ? '加载中…' : '加载更早消息'}</button>}
        {props.messages.length === 0 && <PageState state="empty" title="暂无匹配消息" description="当前会话在所选筛选条件下没有消息。" />}
        {props.messages.map((message) => <article className={`employee-conversation-message ${message.direction}`} key={message.id}>
          <header><strong>{message.senderName || '未知成员'}</strong><time>{message.sentAt}</time></header>
          <div><ConversationMessageContent content={message.content} type={message.type} /></div>
        </article>)}
      </div>
    </>}
  </section>;
}
