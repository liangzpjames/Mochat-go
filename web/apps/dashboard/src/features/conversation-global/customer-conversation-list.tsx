import { DashboardPagination } from '../../components/dashboard-pagination';
import { PageState } from '../../components/page-state/page-state';
import type { CustomerConversationMode, CustomerConversationPage, CustomerConversationProfile, CustomerConversationSummary } from './conversation-global-api';

type Props = {
  customer?: CustomerConversationProfile | undefined;
  data: CustomerConversationPage | undefined;
  mode: CustomerConversationMode;
  page: number;
  selectedConversationId: string | null;
  pending: boolean;
  fetching: boolean;
  error: Error | null;
  onModeChange: (mode: CustomerConversationMode) => void;
  onPageChange: (page: number) => void;
  onRefresh: () => void;
  onSelectConversation: (conversationId: string) => void;
};

function displayName(value: string, fallback: string) {
  return value.trim() || fallback;
}

function shortTime(value: string) {
  return value.length >= 16 ? value.slice(5, 16) : value || '--';
}

function targetName(item: CustomerConversationSummary) {
  return item.targetType === 'room'
    ? displayName(item.targetName, `客户群 ${item.targetId}`)
    : displayName(item.targetName, `客户 ${item.targetId}`);
}

function profileStatusLabel(status: CustomerConversationPage['customer']['profileStatus']) {
  if (status === 'available') return '资料已同步';
  if (status === 'deleted') return '客户已删除';
  return '客户资料未同步';
}

function ConversationCard({
  item,
  selected,
  onSelect,
}: {
  item: CustomerConversationSummary;
  selected: boolean;
  onSelect: () => void;
}) {
  const isGroup = item.targetType === 'room';
  const title = isGroup ? targetName(item) : displayName(item.employeeName, `员工 ${item.employeeId}`);
  const subtitle = isGroup
    ? `归档员工：${displayName(item.employeeName, `员工 ${item.employeeId}`)}`
    : `客户：${targetName(item)}`;
  const canOpen = typeof item.conversationId === 'string' && item.conversationId.trim() !== '';
  const accessibleName = `${title} ${isGroup ? displayName(item.employeeName, `员工 ${item.employeeId}`) : targetName(item)}`;

  return <button
    aria-label={accessibleName}
    aria-pressed={selected}
    className={`customer-conversation-card${selected ? ' is-selected' : ''}`}
    disabled={!canOpen}
    onClick={onSelect}
    title={canOpen ? undefined : '后端未返回稳定会话标识，当前记录无法打开'}
    type="button"
  >
    <span className="customer-conversation-card-avatar" aria-hidden="true">
      {(isGroup ? item.targetAvatar : item.employeeAvatar)?.trim() ? <img alt="" loading="lazy" src={isGroup ? item.targetAvatar : item.employeeAvatar} /> : title.slice(0, 1)}
    </span>
    <span className="customer-conversation-card-copy">
      <span className="customer-conversation-card-title"><strong>{title}</strong><time>{shortTime(item.sentAt)}</time></span>
      <small>{subtitle}</small>
      <p>{item.lastMessage || '暂无可展示的消息内容'}</p>
      <span className="customer-conversation-card-signals">
        {item.relationStatus === 'active' && <em>有效关系</em>}
        {item.relationStatus === 'lost' && <em>关系已流失</em>}
        {item.relationStatus === 'unknown' && <em>关系未知</em>}
        {item.membershipStatus === 'left' && <em>已退群</em>}
        {item.membershipStatus === 'active' && isGroup && <em>群成员关系有效</em>}
        {item.riskCount !== undefined && <em>风险 {item.riskCount}</em>}
        {item.timeoutCount !== undefined && <em>超时 {item.timeoutCount}</em>}
        {item.messageTotal !== undefined && <span>{item.messageTotal} 条消息</span>}
        {item.focused === true && <em aria-label="重点关注">★ 重点关注</em>}
      </span>
    </span>
  </button>;
}

export function CustomerConversationList(props: Props) {
  const customer = 'customer' in props ? props.customer : props.data?.customer;
  const customerTitle = customer
    ? displayName(customer.name, `客户 ${customer.id}`)
    : '请选择客户';
  const list = props.data?.list ?? [];

  return <section aria-label="客户关联会话" className="customer-conversation-pane customer-conversation-list">
    <header className="customer-conversation-pane-header">
      <div className="customer-conversation-customer-header">
        <span className="customer-conversation-customer-avatar" aria-hidden="true">
          {customer?.avatar.trim()
            ? <img alt="" loading="lazy" src={customer.avatar} />
            : customer
              ? <span aria-hidden="true" className="customer-conversation-empty-avatar"><svg aria-hidden="true" focusable="false" viewBox="0 0 24 24"><path d="M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Zm0 2c-4.42 0-8 2.24-8 5v1h16v-1c0-2.76-3.58-5-8-5Z" /></svg></span>
              : <span aria-hidden="true" data-testid="customer-conversation-empty-avatar"><svg aria-hidden="true" focusable="false" viewBox="0 0 24 24"><path d="M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Zm0 2c-4.42 0-8 2.24-8 5v1h16v-1c0-2.76-3.58-5-8-5Z" /></svg></span>}
        </span>
        <span>
          <small>客户关联会话</small>
          <strong>{customerTitle}</strong>
          {customer !== undefined && <em className={`customer-conversation-profile-status is-${customer.profileStatus}`}>{profileStatusLabel(customer.profileStatus)}</em>}
        </span>
      </div>
      <button aria-label="刷新会话" className="customer-conversation-action-button" disabled={props.fetching || customer === undefined} onClick={props.onRefresh} type="button">
        <span>{props.fetching ? '刷新中' : '刷新'}</span>
      </button>
    </header>

    <nav aria-label="客户会话类型" className="customer-conversation-tabs" role="tablist">
      <button aria-selected={props.mode === 'direct'} aria-pressed={props.mode === 'direct'} onClick={() => props.onModeChange('direct')} role="tab" type="button">单聊</button>
      <button aria-selected={props.mode === 'group'} aria-pressed={props.mode === 'group'} onClick={() => props.onModeChange('group')} role="tab" type="button">群聊</button>
    </nav>

    <div className="customer-conversation-scroll customer-conversation-items">
      {customer === undefined && <PageState state="empty" title="请选择客户" description="从客户目录选择客户后查看关联单聊和群聊。" />}
      {customer !== undefined && props.pending && <PageState state="loading" title="正在加载客户会话" />}
      {customer !== undefined && props.error !== null && <PageState state="error" title="客户会话加载失败" description={props.error.message} onRetry={props.onRefresh} />}
      {customer !== undefined && !props.pending && props.error === null && list.length === 0 && <PageState state="empty" title="暂无关联会话" description="该客户在当前会话类型下没有可展示的归档会话。" />}
      {customer !== undefined && props.error === null && list.map((item) => <ConversationCard
        item={item}
        key={item.id}
        onSelect={() => item.conversationId && props.onSelectConversation(item.conversationId)}
        selected={props.selectedConversationId === item.conversationId}
      />)}
    </div>

    {props.data !== undefined && !props.pending && props.error === null && <DashboardPagination
      ariaLabel="客户会话分页"
      onPageChange={props.onPageChange}
      page={props.page}
      pageSize={20}
      total={props.data.total}
    />}
  </section>;
}
