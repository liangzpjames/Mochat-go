import { ApiError } from '@mochat/api-client';
import { useQuery } from '@tanstack/react-query';

import type { ProviderStatus, ProviderStatusApi } from './provider-status-api';

export function ProviderStatusPage({ api, isSuperAdmin: _isSuperAdmin = false }: { api: ProviderStatusApi; isSuperAdmin?: boolean }) {
  const query = useQuery({ queryKey: ['provider-status'], queryFn: () => api.getStatus(), retry: false });

  if (query.isPending) {
    return <section aria-label="服务运行状态" className="phase35-card provider-status-card"><p>正在读取服务运行状态…</p></section>;
  }

  if (query.isError) {
    return (
      <section aria-label="服务运行状态" className="phase35-card provider-status-card">
        <p role="alert">{providerStatusErrorMessage(query.error)}</p>
        <button type="button" onClick={() => void query.refetch()}>重试</button>
      </section>
    );
  }

  const statuses = query.data?.providers ?? [];
  return (
    <section aria-label="服务运行状态" className="phase35-card provider-status-card">
      <header className="phase35-card-header"><h2>服务运行状态</h2></header>
      {statuses.length === 0
        ? <p>暂无服务状态信息。</p>
        : <div className="provider-status-list">{statuses.map((status) => <ProviderStatusRow key={status.kind} status={status} />)}</div>}
    </section>
  );
}

function ProviderStatusRow({ status }: { status: ProviderStatus }) {
  const { label: stateLabel, description } = serviceStateSummary(status.state);
  return (
    <article className={`provider-status-row provider-status-row-${status.state}`} aria-label={`${serviceLabel(status.kind)} ${stateLabel}`}>
      <div><h3>{serviceLabel(status.kind)}</h3><p>{description}</p></div>
      <strong>{stateLabel}</strong>
    </article>
  );
}

function providerStatusErrorMessage(error: unknown): string {
  if (error instanceof ApiError && error.machineCode === 'CORP_CONFIGURATION_REQUIRED') {
    return '企业服务尚未完成配置，请联系管理员。';
  }
  if (error instanceof ApiError && (error.status === 403 || error.machineCode === 'DASHBOARD_PERMISSION_DENIED')) {
    return '当前账号无权查看服务运行状态。';
  }
  return '服务状态暂时无法获取，请稍后重试。';
}

function serviceLabel(kind: string): string {
  switch (kind) {
    case 'wecom_standard': return '企业微信';
    case 'wecom_archive': return '会话存档';
    case 'audio_storage': return '文件与录音';
    case 'ai': return '智能分析';
    default: return '其他服务';
  }
}

function serviceStateSummary(state: ProviderStatus['state']): { label: string; description: string } {
  switch (state) {
    case 'ready': return { label: '运行正常', description: '服务运行正常' };
    case 'limited': return { label: '需要完善', description: '部分功能尚未配置完成' };
    case 'unavailable': return { label: '暂不可用', description: '当前无法使用，请联系管理员' };
  }
}
