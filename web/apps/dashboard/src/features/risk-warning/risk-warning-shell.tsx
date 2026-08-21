import type { FormEvent, ReactNode } from 'react';

export { RiskWarningDrawer } from './risk-warning-drawer';

export function RiskWarningShell({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <section className={`risk-warning-workspace ${className}`.trim()}>{children}</section>;
}

export function RiskWarningPageHeader({
  eyebrow,
  title,
  meta,
  actions,
}: {
  eyebrow?: string;
  title: string;
  meta?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className="risk-warning-page-header">
      <div>
        {eyebrow ? <span className="risk-warning-eyebrow">{eyebrow}</span> : null}
        <h1>{title}</h1>
        {meta ? <div className="risk-warning-page-meta">{meta}</div> : null}
      </div>
      {actions ? <div className="risk-warning-page-actions">{actions}</div> : null}
    </header>
  );
}

export function RiskWarningTabs({
  active,
  tabs,
  onChange,
}: {
  active: string;
  tabs: readonly { id: string; label: string }[];
  onChange: (id: string) => void;
}) {
  return (
    <nav className="risk-warning-tabs" aria-label="风险预警菜单">
      {tabs.map((tab) => (
        <button key={tab.id} type="button" aria-pressed={active === tab.id} onClick={() => onChange(tab.id)}>{tab.label}</button>
      ))}
    </nav>
  );
}

export function RiskWarningQueryBar({
  children,
  fetching,
  onQuery,
  onReset,
  onRefresh,
}: {
  children: ReactNode;
  fetching: boolean;
  onQuery: () => void;
  onReset: () => void;
  onRefresh: () => void;
}) {
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onQuery();
  };
  return (
    <form className="risk-warning-query" onSubmit={submit}>
      <div className="risk-warning-query-fields">{children}</div>
      <div className="risk-warning-query-actions">
        <button type="submit" className="risk-warning-primary-button" disabled={fetching}>查询</button>
        <button type="button" className="risk-warning-secondary-button" onClick={onReset} disabled={fetching}>重置</button>
        <button type="button" className="risk-warning-secondary-button" aria-label="刷新" onClick={onRefresh} disabled={fetching}>刷新</button>
      </div>
    </form>
  );
}

export function ScannerStatusStrip({
  status,
}: {
  status: { enabled: boolean; state: 'ready' | 'never_run' | 'failed' | 'disabled'; lastSuccessAt?: string; lastError?: string } | null | undefined;
}) {
  if (!status || status.state === 'ready') return null;
  const message = status.state === 'disabled'
    ? '扫描任务未启用，当前页面不会自动产生新记录。'
    : status.state === 'failed'
      ? `扫描任务最近一次失败${status.lastError ? `：${status.lastError}` : '，请检查运行日志。'}`
      : '扫描任务尚未成功运行，暂无新增记录可供判断。';
  return <div className="risk-warning-status-strip" role="status">{message}</div>;
}
