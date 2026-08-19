import type { FormEvent } from 'react';

import { PageState } from '../../components/page-state/page-state';
import { ConversationMessageContent } from './conversation-message-content';
import type { CustomerConversationDetail } from './conversation-global-api';

type Props = {
  data: CustomerConversationDetail | undefined;
  messages: CustomerConversationDetail['messages'];
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

function stat(value: number | null) {
  return value === null ? '--' : value.toLocaleString();
}

function displayTargetName(data: Pick<CustomerConversationDetail, 'targetType' | 'targetId' | 'targetName' | 'customerName' | 'profile'>) {
  if (data.targetName.trim()) return data.targetName;
  if (data.targetType !== 'room' && data.customerName.trim()) return data.customerName;
  if (data.targetType !== 'room' && data.profile.name.trim()) return data.profile.name;
  return data.targetType === 'room' ? `客户群 ${data.targetId}` : `客户 ${data.targetId}`;
}

function senderName(data: Pick<CustomerConversationDetail, 'targetType'>, name: string) {
  if (name.trim()) return name;
  return data.targetType === 'room' ? '群成员' : '未知成员';
}

function capabilityReason(key: string, reason?: string) {
  if (reason?.trim()) return reason;
  if (key === 'groupMemberIdentity') return '无法稳定识别群聊消息发送方';
  return '当前客户会话能力不可用';
}

export function CustomerConversationDetailPane(props: Props) {
  const data = props.data;
  const unavailableCapabilities = data?.capabilities.filter((item) => !item.available) ?? [];
  const inboundLabel = data?.targetType === 'room' ? '非员工消息' : '客户发送';

  return <section aria-label="客户消息详情" className="customer-conversation-pane customer-conversation-detail">
    {data === undefined && !props.pending && props.error === null && <PageState state="empty" title="请选择会话" description="选择中间会话后，在这里阅读完整上下文。" />}
    {props.pending && <PageState state="loading" title="正在读取客户会话" />}
    {props.error !== null && <PageState state="error" title="客户会话加载失败" description={props.error.message} onRetry={props.onRefresh} />}
    {data !== undefined && <>
      <header className="customer-conversation-detail-header">
        <div>
          <small>{data.targetType === 'room' ? '客户群会话' : '客户单聊'}</small>
          <strong>{displayTargetName(data)}</strong>
          <span>{data.employeeName.trim() || `员工 ${data.employeeId}`}</span>
        </div>
        <div>
          <button aria-label="刷新客户详情" disabled={props.fetching} onClick={props.onRefresh} type="button">{props.fetching ? '刷新中' : '刷新'}</button>
          <button aria-label={data.focused ? '取消重点关注' : '重点关注'} className={data.focused ? 'is-focused' : ''} disabled={props.focusPending} onClick={props.onToggleFocus} type="button">{data.focused ? '取消关注' : '重点关注'}</button>
        </div>
      </header>
      {data.profile.profileStatus !== 'available' && <p className="customer-conversation-limitation" role="alert">客户资料未同步</p>}
      {props.focusError && <p className="customer-conversation-limitation" role="alert">{props.focusError}</p>}
      {unavailableCapabilities.length > 0 && <div className="customer-conversation-capability-alert" role="alert">
        {unavailableCapabilities.map((item) => <p key={item.key}>{capabilityReason(item.key, item.reason)}</p>)}
      </div>}
      <section aria-label="会话统计" className="customer-conversation-stats">
        <article><span>沟通天数</span><strong>{stat(data.stats.communicationDays)}</strong><small>天</small></article>
        <article><span>消息总数</span><strong>{stat(data.stats.messageTotal)}</strong><small>条</small></article>
        <article><span>{inboundLabel}</span><strong>{stat(data.stats.inboundTotal)}</strong><small>条</small></article>
        <article><span>员工发送</span><strong>{stat(data.stats.outboundTotal)}</strong><small>条</small></article>
      </section>
      <div className="customer-conversation-detail-filters">
        <div aria-label="消息类型筛选" className="customer-conversation-message-types" role="group">
          {messageTypeOptions.map(([value, label]) => <label key={value}><input aria-label={label} checked={props.messageTypes.includes(value)} onChange={() => props.onToggleMessageType(value)} type="checkbox" /><span>{label}</span></label>)}
        </div>
        <form aria-label="消息内容查询" className="customer-conversation-keyword-search" onSubmit={props.onKeywordSearch}>
          <input aria-label="搜索会话内容" onChange={(event) => props.onKeywordChange(event.target.value)} placeholder="搜索会话内容" value={props.keyword} />
          <button aria-label="查询会话内容" type="submit">查询</button>
        </form>
        <input aria-label="检索日期" onChange={(event) => props.onDateChange(event.target.value)} type="date" value={props.date} />
      </div>
      <div className="customer-conversation-scroll customer-conversation-messages">
        {props.hasMore && <button className="customer-conversation-load-more" disabled={props.loadingOlder} onClick={props.onLoadOlder} type="button">{props.loadingOlder ? '加载中…' : '加载更早消息'}</button>}
        {props.messages.length === 0 && <PageState state="empty" title="暂无匹配消息" description="当前会话在所选筛选条件下没有消息。" />}
        {props.messages.map((message) => <article className={`customer-conversation-message ${message.direction}`} key={message.id}>
          <header><strong>{senderName(data, message.senderName)}</strong><time>{message.sentAt}</time></header>
          <div><ConversationMessageContent content={message.content} type={message.type} /></div>
        </article>)}
      </div>
    </>}
  </section>;
}
