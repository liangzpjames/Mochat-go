import { useQuery } from '@tanstack/react-query';

import type { ProviderStatus, ProviderStatusApi } from './provider-status-api';

export function ProviderStatusPage({ api, isSuperAdmin = false }: { api: ProviderStatusApi; isSuperAdmin?: boolean }) {
  const query = useQuery({ queryKey: ['provider-status'], queryFn: () => api.getStatus(), retry: false });
  if (query.isPending) return <section aria-label="Provider 状态" className="phase35-card"><p>正在读取 Provider 状态…</p></section>;
  if (query.isError) return <section aria-label="Provider 状态" className="phase35-card"><p role="alert">Provider 状态暂时不可用，请稍后重试。</p><button type="button" onClick={() => void query.refetch()}>重试</button></section>;
  const statuses = query.data?.providers ?? [];
  return (
    <section aria-label="Provider 状态" className="phase35-card provider-status-card">
      <header className="phase35-card-header"><div><h2>Provider 运行状态</h2><p>状态来自当前运行时与企业作用域凭据，不触发外部测试请求。</p></div></header>
      {statuses.length === 0 ? <p>当前没有可显示的 Provider 能力。</p> : <div className="provider-status-list">{statuses.map((status) => <ProviderStatusRow key={status.kind} status={status} isSuperAdmin={isSuperAdmin} />)}</div>}
    </section>
  );
}

function ProviderStatusRow({ status, isSuperAdmin }: { status: ProviderStatus; isSuperAdmin: boolean }) {
  const label = providerLabel(status.kind);
  const stateLabel = status.state === 'ready' ? '可用' : status.state === 'limited' ? '受限' : '不可用';
  return (
    <article className={`provider-status-row provider-status-row-${status.state}`} aria-label={`${label} ${stateLabel}`}>
      <div><h3>{label}</h3><p>{status.capabilities.join('、') || '未声明能力'}</p></div>
      <strong>{stateLabel}</strong>
      <div><code>{status.code}</code>{!isSuperAdmin && status.action && <p>{status.action}</p>}{isSuperAdmin && status.reason && <p>{status.reason}</p>}{isSuperAdmin && status.missing && status.missing.length > 0 && <p>缺失项：{status.missing.join('、')}</p>}</div>
    </article>
  );
}

function providerLabel(kind: string): string {
  switch (kind) {
    case 'wecom_standard': return '企业微信标准能力';
    case 'wecom_archive': return '会话存档';
    case 'audio_storage': return '音频存储';
    case 'ai': return 'AI Provider';
    default: return kind;
  }
}
