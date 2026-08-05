import type { ReactNode } from 'react';
export function Phase35PageShell({ title, description, actions, children }: { title: string; description?: string; actions?: ReactNode; children: ReactNode }) {
  return <section className="dashboard-content"><header className="dashboard-page-header"><div><h1>{title}</h1>{description && <p>{description}</p>}</div>{actions && <div>{actions}</div>}</header>{children}</section>;
}
