import type { ReactNode } from 'react';

export type MobileBottomNavigationItem = {
  key: string;
  label: string;
  icon: ReactNode;
  href: string;
  current: boolean;
};

export type MobileBottomNavigationProps = {
  label: string;
  items: MobileBottomNavigationItem[];
};

export function MobileBottomNavigation({
  label,
  items,
}: MobileBottomNavigationProps) {
  return (
    <nav
      aria-label={label}
      className="mobile-bottom-navigation mobile-bottom-navigation--safe-area"
    >
      {items.map((item) => (
        <a
          aria-current={item.current ? 'page' : undefined}
          className="mobile-bottom-navigation__item"
          href={item.href}
          key={item.key}
        >
          <span aria-hidden="true" className="mobile-bottom-navigation__icon">
            {item.icon}
          </span>
          <span className="mobile-bottom-navigation__label">{item.label}</span>
        </a>
      ))}
    </nav>
  );
}
