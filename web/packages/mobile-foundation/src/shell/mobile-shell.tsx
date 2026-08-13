import type { ReactNode } from 'react';

import { MobileActionDock } from './mobile-state';

export type MobileShellProps = {
  appName: string;
  title: string;
  subtitle?: string;
  leading?: ReactNode;
  children: ReactNode;
  actions?: ReactNode;
};

export function MobileShell({
  appName,
  title,
  subtitle,
  leading,
  children,
  actions,
}: MobileShellProps) {
  return (
    <div
      className="mobile-shell mobile-shell--safe-area"
      data-testid="mobile-shell"
    >
      <header className="mobile-shell__header">
        {leading === undefined ? null : (
          <div className="mobile-shell__leading">{leading}</div>
        )}
        <div className="mobile-shell__heading">
          <p className="mobile-shell__app-name">{appName}</p>
          <h1 className="mobile-shell__title">{title}</h1>
          {subtitle === undefined ? null : (
            <p className="mobile-shell__subtitle">{subtitle}</p>
          )}
        </div>
      </header>
      <main className="mobile-shell__content">{children}</main>
      {actions === undefined ? null : <MobileActionDock>{actions}</MobileActionDock>}
    </div>
  );
}
