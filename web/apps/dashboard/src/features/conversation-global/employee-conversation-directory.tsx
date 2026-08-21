import type { FormEvent } from 'react';

import { PageState } from '../../components/page-state/page-state';
import type { StaffDirectoryEmployee, StaffDirectoryMode, StaffDirectoryPage } from './conversation-global-api';
import { EmployeeConversationDepartmentPicker } from './employee-conversation-department-picker';

type Props = {
  data: StaffDirectoryPage | undefined;
  employees: readonly StaffDirectoryEmployee[];
  mode: StaffDirectoryMode;
  lockedMode?: StaffDirectoryMode;
  selectedDepartmentId: number | null;
  selectedEmployeeId: number | null;
  keywordDraft: string;
  pending: boolean;
  fetching: boolean;
  error: Error | null;
  hasMore: boolean;
  onKeywordDraftChange: (value: string) => void;
  onSearch: (event: FormEvent<HTMLFormElement>) => void;
  onRefresh: () => void;
  onModeChange: (mode: StaffDirectoryMode) => void;
  onDepartmentChange: (id: number | null) => void;
  onSelectEmployee: (id: number) => void;
  onLoadMore: () => void;
};

function EmployeeAvatar({ employee }: { employee: StaffDirectoryEmployee }) {
  return employee.avatar.trim() === ''
    ? <span aria-hidden="true">{employee.name.trim().slice(0, 1) || '?'}</span>
    : <img alt="" loading="lazy" src={employee.avatar} />;
}

export function EmployeeConversationDirectory(props: Props) {
  const counts = props.data?.counts ?? { all: 0, focused: 0, archived: 0, departed: 0 };
  return <aside aria-label="员工目录" className="employee-conversation-pane employee-conversation-directory">
    {props.lockedMode === undefined && <div aria-label="目录模式" className="employee-conversation-directory-tabs" role="tablist">
      <button aria-selected={props.mode !== 'focused'} onClick={() => props.onModeChange('all')} role="tab" type="button">企业架构</button>
      <button aria-selected={props.mode === 'focused'} onClick={() => props.onModeChange('focused')} role="tab" type="button">重点关注</button>
    </div>}
    <form className="employee-conversation-directory-search" onSubmit={props.onSearch}>
      <input aria-label="搜索员工" onChange={(event) => props.onKeywordDraftChange(event.target.value)} placeholder="搜索员工" value={props.keywordDraft} />
      <button type="submit">查询</button>
      <button aria-label="刷新员工" disabled={props.fetching} onClick={props.onRefresh} type="button">{props.fetching && !props.pending ? '刷新中' : '刷新'}</button>
    </form>
    {props.lockedMode === undefined && <div className="employee-conversation-directory-filters">
      <button aria-pressed={props.mode === 'all'} onClick={() => props.onModeChange('all')} type="button">全部成员 {counts.all}</button>
      <button aria-pressed={props.mode === 'archived'} onClick={() => props.onModeChange('archived')} type="button">存档成员 {counts.archived}</button>
      <button aria-pressed={props.mode === 'departed'} onClick={() => props.onModeChange('departed')} type="button">离职成员 {counts.departed}</button>
    </div>}
    {props.data?.limitations.map((item) => <p className="employee-conversation-limitation" key={item.key} role="status">{item.reason}</p>)}
    {props.mode !== 'focused' && props.data !== undefined && props.data.departments.length > 0 && <div className="employee-conversation-departments">
      <EmployeeConversationDepartmentPicker onSelect={props.onDepartmentChange} rows={props.data.departments} selectedDepartmentId={props.selectedDepartmentId} />
    </div>}
    <div className="employee-conversation-scroll employee-conversation-employee-list">
      {props.pending && <PageState state="loading" title="正在加载员工" />}
      {props.error !== null && <PageState state="error" title="员工目录加载失败" description={props.error.message} onRetry={props.onRefresh} />}
      {!props.pending && props.error === null && props.employees.length === 0 && <PageState state="empty" title="暂无员工" description="当前组织或筛选条件下没有可访问员工。" />}
      {props.employees.map((employee) => <button aria-pressed={props.selectedEmployeeId === employee.id} className={props.selectedEmployeeId === employee.id ? 'is-selected' : ''} key={employee.id} onClick={() => props.onSelectEmployee(employee.id)} type="button">
        <span className="employee-conversation-avatar"><EmployeeAvatar employee={employee} /></span>
        <span className="employee-conversation-employee-copy"><strong>{employee.name || `员工 ${employee.id}`}</strong><small>{employee.status === 5 ? '已离职' : `${employee.conversationCount} 个会话`}</small></span>
        {employee.focusedConversationCount > 0 && <em title="重点关注会话">★ {employee.focusedConversationCount}</em>}
      </button>)}
      {props.hasMore && <button className="employee-conversation-load-more" disabled={props.fetching} onClick={props.onLoadMore} type="button">加载更多员工</button>}
    </div>
  </aside>;
}
