import type { ConversationExportTask, ConversationExportTaskPage } from './conversation-global-api';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { exportTypeLabel } from './conversation-export-stepper';

function taskStatus(task: ConversationExportTask): string {
  if (task.status === 'pending') return '排队中';
  if (task.status === 'running') return '生成中';
  if (task.status === 'completed') return '可下载';
  if (task.status === 'failed') return '失败';
  if (task.status === 'expired') return '已过期';
  return task.status;
}

function taskDateTime(value: string): string {
  return value.replace('T', ' ').slice(0, 16);
}

export function ConversationExportTasks({
  data,
  loading,
  refreshing,
  onClose,
  onPageChange,
  onDownload,
}: {
  data: ConversationExportTaskPage | undefined;
  loading: boolean;
  refreshing: boolean;
  onClose: () => void;
  onPageChange: (page: number) => void;
  onDownload: (task: ConversationExportTask) => void;
}) {
  return <div className="conversation-export-drawer-mask" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
    <aside className="conversation-export-drawer" role="dialog" aria-modal="true" aria-labelledby="conversation-export-tasks-title">
      <header>
        <div className="conversation-export-drawer-heading">
          <span className="conversation-export-eyebrow">任务中心</span>
          <div><h2 id="conversation-export-tasks-title">导出任务</h2><span>{data ? `共 ${data.total} 个任务` : '查看任务状态与下载文件'}</span></div>
        </div>
        <div className="conversation-export-drawer-header-actions">
          {refreshing && <span className="conversation-export-task-sync" role="status"><i />正在同步任务状态</span>}
          <button type="button" aria-label="关闭任务抽屉" onClick={onClose}>×</button>
        </div>
      </header>
      <div className="conversation-export-task-list">
        {loading && !data && <div className="conversation-export-table-empty">正在加载任务…</div>}
        {!loading && data?.items.length === 0 && <div className="conversation-export-table-empty">暂无导出任务</div>}
        {data?.items.map((task) => <article className="conversation-export-task" key={task.id}>
          <div className="conversation-export-task-head">
            <div className="conversation-export-task-title"><span className="conversation-export-task-icon">↓</span><div><strong>{exportTypeLabel(task.exportType)}导出</strong><small>#{task.id}</small></div></div>
            <span className={`conversation-export-status is-${task.status}`}>{taskStatus(task)}</span>
          </div>
          <div className="conversation-export-task-range"><span>消息日期</span><strong>{task.startAt.slice(0, 10)} 至 {task.endAt.slice(0, 10)}</strong></div>
          <div className="conversation-export-task-metrics">
            <span><small>导出对象</small><strong>{task.objectCount} 个对象</strong></span>
            <span><small>{task.status === 'completed' ? '实际消息' : '预计消息'}</small><strong>{task.status === 'completed' ? task.messageCount : task.estimatedMessageCount} 条消息</strong></span>
            <span><small>文件数量</small><strong>{task.fileCount > 0 ? `${task.fileCount} 个文件` : '生成中'}</strong></span>
          </div>
          {task.errorMessage && <small className="conversation-export-form-error">{task.errorMessage}</small>}
          <div className="conversation-export-task-actions">
            <time dateTime={task.createdAt}>创建于 {taskDateTime(task.createdAt)}</time>
            {task.status === 'completed' && <button className="conversation-export-primary conversation-export-task-download" aria-label={`下载 ${task.artifactName || '导出文件'}`} type="button" onClick={() => onDownload(task)}>下载文件</button>}
          </div>
        </article>)}
      </div>
      <footer><DashboardPagination ariaLabel="导出任务分页" page={data?.page ?? 1} pageSize={20} total={data?.total ?? 0} onPageChange={onPageChange} /></footer>
    </aside>
  </div>;
}
