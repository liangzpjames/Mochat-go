import type { ReactNode } from 'react';

export function DashboardPageShell({
  eyebrow,
  title,
  description,
  status,
  primaryAction,
  children,
  className,
}: {
  eyebrow?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  status?: ReactNode;
  primaryAction?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <main className={['dashboard-page-shell', className].filter(Boolean).join(' ')}>
      <header className="dashboard-page-shell__header">
        <div className="dashboard-page-shell__heading">
          {eyebrow !== undefined && <div className="dashboard-page-shell__eyebrow">{eyebrow}</div>}
          <h1>{title}</h1>
          {description !== undefined && <p className="dashboard-page-shell__description">{description}</p>}
          {status !== undefined && <div className="dashboard-page-shell__status">{status}</div>}
        </div>
        {primaryAction !== undefined && <div className="dashboard-page-shell__primary-action">{primaryAction}</div>}
      </header>
      <div className="dashboard-page-shell__content">{children}</div>
    </main>
  );
}
