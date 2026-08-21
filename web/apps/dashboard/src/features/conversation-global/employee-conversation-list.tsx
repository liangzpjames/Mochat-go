import type { ConversationPage, ConversationSummary, ConversationTargetType, StaffDirectoryEmployee } from './conversation-global-api';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { PageState } from '../../components/page-state/page-state';

type TypeValue = '' | ConversationTargetType;
type Props = {
  employee: StaffDirectoryEmployee | undefined;
  data: ConversationPage | undefined;
  selectedConversationId: string | null;
  selectedConversationRowId: string | null;
  type: TypeValue;
  page: number;
  pending: boolean;
  fetching: boolean;
  error: Error | null;
  internalGroupReason: string;
  allowedTypes?: readonly TypeValue[];
  onTypeChange: (type: TypeValue) => void;
  onPageChange: (page: number) => void;
  onSelectConversation: (id: string, rowId: string) => void;
  onRefresh: () => void;
};

function targetLabel(type: ConversationTargetType) { return type === 'customer' ? '客户' : type === 'room' ? '客户群' : '同事'; }
function displayTargetName(item: Pick<ConversationSummary, 'targetType' | 'targetId' | 'targetName'>) {
  if (item.targetName.trim()) return item.targetName;
  if (item.targetType === 'room' && item.targetId === 0) return '客户群';
  return `${targetLabel(item.targetType)} ${item.targetId}`;
}
function shortTime(value: string) { return value.length >= 16 ? value.slice(5, 16) : value || '--'; }

function ConversationRow({ item, selected, onSelect }: { item: ConversationSummary; selected: boolean; onSelect: () => void }) {
  const canOpen = typeof item.conversationId === 'string' && item.conversationId.trim() !== '';
  return <button className={selected ? 'is-selected' : ''} disabled={!canOpen} onClick={onSelect} title={canOpen ? undefined : '后端未返回稳定会话标识，当前记录无法打开'} type="button">
    <span className="employee-conversation-row-avatar">{item.targetAvatar ? <img alt="" src={item.targetAvatar} /> : (item.targetName.trim().slice(0, 1) || '?')}</span>
    <span className="employee-conversation-row-copy"><span><strong>{displayTargetName(item)}</strong><time>{shortTime(item.sentAt)}</time></span><small>{targetLabel(item.targetType)} · {item.messageTotal ?? '--'} 条消息</small><p>{item.lastMessage || '暂无可展示的消息内容'}</p></span>
    {item.focused && <em aria-label="重点关注">★</em>}
  </button>;
}

export function EmployeeConversationList(props: Props) {
  const types = [['', '全部'], ['customer', '客户'], ['room', '客户群'], ['employee', '同事']] as const;
  const visibleTypes = props.allowedTypes === undefined ? types : types.filter(([value]) => props.allowedTypes?.includes(value));
  const matchingRows = props.data?.list.filter((item) => item.conversationId === props.selectedConversationId) ?? [];
  return <section aria-label="会话轨迹" className="employee-conversation-pane employee-conversation-list">
    <header className="employee-conversation-pane-header"><div><small>员工会话轨迹</small><strong>{props.employee?.name ?? '请选择员工'}</strong></div><button aria-label="刷新会话" disabled={!props.employee || props.fetching} onClick={props.onRefresh} type="button">刷新</button></header>
    <nav aria-label="会话类型" className="employee-conversation-tabs">
      {visibleTypes.map(([value, label]) => <button aria-pressed={props.type === value} key={value || 'all'} onClick={() => props.onTypeChange(value)} type="button">{label}</button>)}
      {props.allowedTypes === undefined && <button disabled title={props.internalGroupReason} type="button">内部群</button>}
    </nav>
    <div className="employee-conversation-scroll employee-conversation-items">
      {!props.employee && <PageState state="empty" title="请选择员工" description="从员工目录选择成员后查看真实会话轨迹。" />}
      {props.employee && props.pending && <PageState state="loading" title="正在加载会话" />}
      {props.employee && props.error !== null && <PageState state="error" title="会话轨迹加载失败" description={props.error.message} onRetry={props.onRefresh} />}
      {props.employee && !props.pending && props.error === null && props.data?.list.length === 0 && <PageState state="empty" title="暂无会话" description="该员工在当前类型下没有归档会话。" />}
      {props.data?.list.map((item) => <ConversationRow item={item} key={item.id} onSelect={() => item.conversationId && props.onSelectConversation(item.conversationId, item.id)} selected={item.id === props.selectedConversationRowId || (props.selectedConversationRowId === null && matchingRows.length === 1 && item.conversationId === props.selectedConversationId)} />)}
    </div>
    {props.employee && props.data && <DashboardPagination ariaLabel="会话分页" onPageChange={props.onPageChange} page={props.page} pageSize={20} total={props.data.total} />}
  </section>;
}
