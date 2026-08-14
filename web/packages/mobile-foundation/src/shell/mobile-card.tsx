import type { ReactNode } from 'react';

export type MobileCardTone = 'surface' | 'accent' | 'muted';
export type MobileCardPadding = 'none' | 'compact' | 'comfortable';

export type MobileCardProps = {
  tone: MobileCardTone;
  padding: MobileCardPadding;
  children: ReactNode;
};

export function MobileCard({ tone, padding, children }: MobileCardProps) {
  return (
    <section
      className={`mobile-card mobile-card--${tone} mobile-card--padding-${padding}`}
    >
      {children}
    </section>
  );
}

export type MobileIconTileProps = {
  icon: ReactNode;
  title: string;
  description?: string;
  badge?: string;
  href?: string;
  disabled?: boolean;
};

export function MobileIconTile({
  icon,
  title,
  description,
  badge,
  href,
  disabled = false,
}: MobileIconTileProps) {
  const content = (
    <>
      <span aria-hidden="true" className="mobile-icon-tile__icon">
        {icon}
      </span>
      <span className="mobile-icon-tile__content">
        <span className="mobile-icon-tile__title">{title}</span>
        {description === undefined ? null : (
          <span className="mobile-icon-tile__description">{description}</span>
        )}
      </span>
      {badge === undefined ? null : <span className="mobile-icon-tile__badge">{badge}</span>}
    </>
  );

  if (href !== undefined && !disabled) {
    return (
      <a className="mobile-icon-tile" href={href}>
        {content}
      </a>
    );
  }

  return (
    <div
      aria-disabled={disabled || undefined}
      className="mobile-icon-tile"
    >
      {content}
    </div>
  );
}
