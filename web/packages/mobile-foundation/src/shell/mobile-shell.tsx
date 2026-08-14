import type { ReactNode } from 'react';

import { MobileActionDock } from './mobile-state';

export type MobileShellProps = {
  appName: string;
  title: string;
  eyebrow?: string;
  subtitle?: string;
  hero?: ReactNode;
  bottomNavigation?: ReactNode;
  leading?: ReactNode;
  children: ReactNode;
  actions?: ReactNode;
};

export function MobileShell({
  appName,
  title,
  eyebrow,
  subtitle,
  hero,
  bottomNavigation,
  leading,
  children,
  actions,
}: MobileShellProps) {
  const hasBottomNavigation = bottomNavigation !== undefined && bottomNavigation !== null;

  return (
    <div
      className={`mobile-shell mobile-shell--safe-area${hasBottomNavigation ? ' mobile-shell--has-bottom-navigation' : ''}`}
      data-testid="mobile-shell"
    >
      <header className="mobile-shell__header">
        {leading === undefined ? null : (
          <div className="mobile-shell__leading">{leading}</div>
        )}
        <div className="mobile-shell__heading">
          <p className="mobile-shell__app-name">{appName}</p>
          {eyebrow === undefined ? null : (
            <p className="mobile-shell__eyebrow">{eyebrow}</p>
          )}
          <h1 className="mobile-shell__title">{title}</h1>
          {subtitle === undefined ? null : (
            <p className="mobile-shell__subtitle">{subtitle}</p>
          )}
        </div>
      </header>
      {hero === undefined ? null : <section className="mobile-shell__hero">{hero}</section>}
      <main className="mobile-shell__content">{children}</main>
      {hasBottomNavigation ? bottomNavigation : null}
      {actions === undefined ? null : <MobileActionDock>{actions}</MobileActionDock>}
    </div>
  );
}
