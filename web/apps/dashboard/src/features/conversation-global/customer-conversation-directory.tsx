import type { FormEvent } from 'react';

import { DashboardPagination } from '../../components/dashboard-pagination';
import { PageState } from '../../components/page-state/page-state';
import type { CustomerDirectoryCustomer, CustomerDirectoryMode, CustomerDirectoryPage } from './conversation-global-api';

type Props = {
  data: CustomerDirectoryPage | undefined;
  mode: CustomerDirectoryMode;
  keywordDraft: string;
  page: number;
  selectedCustomerId: number | null;
  pending: boolean;
  fetching: boolean;
  error: Error | null;
  onModeChange: (mode: CustomerDirectoryMode) => void;
  onKeywordDraftChange: (value: string) => void;
  onSearch: (event: FormEvent<HTMLFormElement>) => void;
  onRefresh: () => void;
  onPageChange: (page: number) => void;
  onSelectCustomer: (id: number) => void;
};

function customerName(customer: CustomerDirectoryCustomer) {
  return customer.name.trim() || `客户 ${customer.id}`;
}

function shortTime(value: string) {
  return value.length >= 16 ? value.slice(5, 16) : value || '--';
}

function CustomerAvatar({ customer }: { customer: CustomerDirectoryCustomer }) {
  const name = customerName(customer);
  return customer.avatar.trim() === ''
    ? <span aria-hidden="true">{name.slice(0, 1)}</span>
    : <img alt="" loading="lazy" src={customer.avatar} />;
}

export function CustomerConversationDirectory(props: Props) {
  const counts = props.data?.counts ?? { all: 0, focused: 0, active: 0, lost: 0 };
  const customers = props.data?.customers ?? [];

  return <aside aria-label="客户目录" className="customer-conversation-pane customer-conversation-directory">
    <div aria-label="客户目录模式" className="customer-conversation-directory-tabs" role="tablist">
      <button aria-selected={props.mode !== 'focused'} onClick={() => props.onModeChange('all')} role="tab" type="button">客户列表 {counts.all}</button>
      <button aria-selected={props.mode === 'focused'} onClick={() => props.onModeChange('focused')} role="tab" type="button">重点关注 {counts.focused}</button>
    </div>
    <form aria-label="客户搜索" className="customer-conversation-directory-search" onSubmit={props.onSearch} role="search">
      <input aria-label="搜索客户" onChange={(event) => props.onKeywordDraftChange(event.target.value)} placeholder="搜索客户" value={props.keywordDraft} />
      <button type="submit">查询</button>
      <button aria-label="刷新客户" disabled={props.fetching} onClick={props.onRefresh} type="button">{props.fetching && !props.pending ? '刷新中' : '刷新'}</button>
    </form>
    {props.data?.limitations.map((item) => <p className="customer-conversation-limitation" key={item.key} role="status">{item.reason}</p>)}
    <div aria-label="客户列表" className="customer-conversation-scroll customer-conversation-customer-list" role="list">
      {props.pending && <PageState state="loading" title="正在加载客户" />}
      {props.error !== null && <PageState description={props.error.message} onRetry={props.onRefresh} state="error" title="客户目录加载失败" />}
      {!props.pending && props.error === null && customers.length === 0 && <PageState description="当前筛选条件下没有可访问客户。" state="empty" title="暂无客户" />}
      {customers.map((customer) => {
        const name = customerName(customer);
        return <div key={customer.id} role="listitem">
          <button
            aria-pressed={props.selectedCustomerId === customer.id}
            className={`customer-conversation-customer-card${props.selectedCustomerId === customer.id ? ' is-selected' : ''}`}
            onClick={() => props.onSelectCustomer(customer.id)}
            type="button"
          >
            <span className="customer-conversation-avatar"><CustomerAvatar customer={customer} /></span>
            <span className="customer-conversation-customer-copy">
              <span className="customer-conversation-customer-title"><strong>{name}</strong><time dateTime={customer.lastConversationAt || undefined}>{shortTime(customer.lastConversationAt)}</time></span>
              <small>{customer.directConversationCount} 单聊 · {customer.groupConversationCount} 群聊</small>
            </span>
            {customer.focusedConversationCount > 0 && <em title="重点关注会话">★ {customer.focusedConversationCount}</em>}
            {customer.profileStatus !== 'available' && <span className="customer-conversation-profile-warning" role="alert">客户资料未同步</span>}
          </button>
        </div>;
      })}
    </div>
    {props.data !== undefined && !props.pending && props.error === null && <DashboardPagination
      ariaLabel="客户分页"
      onPageChange={props.onPageChange}
      page={props.page}
      pageSize={props.data.pageSize}
      total={props.data.total}
    />}
    <div aria-label="客户关系筛选" className="customer-conversation-directory-filters">
      <button aria-pressed={props.mode === 'active'} onClick={() => props.onModeChange('active')} type="button">有效关系 {counts.active}</button>
      <button aria-pressed={props.mode === 'lost'} onClick={() => props.onModeChange('lost')} type="button">已流失 {counts.lost}</button>
    </div>
  </aside>;
}
