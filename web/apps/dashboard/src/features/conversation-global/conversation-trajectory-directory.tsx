import type { StaffDirectoryEmployee, StaffDirectoryPage } from './conversation-global-api';

export type TrajectoryDirectoryMode = 'archived' | 'organization';

type Props = {
  mode: TrajectoryDirectoryMode;
  draftKeyword: string;
  data: StaffDirectoryPage | undefined;
  selectedEmployeeId: number | null;
  isLoading: boolean;
  onDraftKeywordChange(value: string): void;
  onKeywordSubmit(): void;
  onModeChange(mode: TrajectoryDirectoryMode): void;
  onDepartmentChange(id: number | null): void;
  onPageChange(page: number): void;
  onSelectEmployee(id: number): void;
};

export function ConversationTrajectoryDirectory(props: Props) {
  const { data } = props;
  return <aside className="conversation-trajectory-directory" aria-label="会话轨迹员工目录">
    <div className="conversation-trajectory-directory-tabs" role="tablist" aria-label="员工范围">
      <button type="button" role="tab" aria-selected={props.mode === 'archived'} onClick={() => props.onModeChange('archived')}>存档员工</button>
      <button type="button" role="tab" aria-selected={props.mode === 'organization'} onClick={() => props.onModeChange('organization')}>组织架构</button>
    </div>
    <form className="conversation-trajectory-directory-search" onSubmit={(event) => { event.preventDefault(); props.onKeywordSubmit(); }}>
      <label htmlFor="trajectory-employee-keyword">员工名称</label>
      <input id="trajectory-employee-keyword" aria-label="员工名称" value={props.draftKeyword} onChange={(event) => props.onDraftKeywordChange(event.target.value)} />
      <button type="submit" aria-label="查询员工">查询</button>
    </form>
    <div className="conversation-trajectory-departments">
      <button type="button" className={!data?.departments.length ? 'is-selected' : ''} onClick={() => props.onDepartmentChange(null)}>全部员工</button>
      {data?.departments.map((department) => <button type="button" key={department.id} onClick={() => props.onDepartmentChange(department.id)}>{department.name}<span>{department.employeeCount}</span></button>)}
    </div>
    {data?.limitations.map((item) => <p role="status" className="conversation-trajectory-limitation" key={item.key}>{item.reason}</p>)}
    <div className="conversation-trajectory-employees" aria-busy={props.isLoading}>
      {data?.employees.map((employee) => <button type="button" key={employee.id} aria-current={props.selectedEmployeeId === employee.id ? 'true' : undefined} className={props.selectedEmployeeId === employee.id ? 'is-selected' : ''} onClick={() => props.onSelectEmployee(employee.id)}>
        <span className="conversation-trajectory-avatar">{employee.avatar ? <img src={employee.avatar} alt="" /> : employee.name.slice(0, 1)}</span>
        <span><strong>{employee.name}</strong></span>
      </button>)}
      {!props.isLoading && data?.employees.length === 0 && <p className="conversation-trajectory-empty">暂无符合条件的员工</p>}
    </div>
    {data && data.total > data.pageSize && <nav className="conversation-trajectory-pagination" aria-label="员工分页"><button type="button" aria-label="上一页" disabled={data.page <= 1} onClick={() => props.onPageChange(data.page - 1)}>‹</button><span>{data.page}</span><button type="button" aria-label="下一页" disabled={data.page * data.pageSize >= data.total} onClick={() => props.onPageChange(data.page + 1)}>›</button></nav>}
  </aside>;
}
