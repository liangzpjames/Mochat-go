import { Component, type ErrorInfo, type ReactNode } from 'react';

export type MobileActionDockProps = {
  children: ReactNode;
};

export function MobileActionDock({ children }: MobileActionDockProps) {
  return (
    <section
      aria-label="页面操作"
      className="mobile-action-dock mobile-action-dock--safe-area"
      data-testid="mobile-action-dock"
      style={{ minHeight: '44px' }}
    >
      {children}
    </section>
  );
}

export type MobileStateKind = 'loading' | 'empty' | 'error' | 'forbidden' | 'not-found';

export type MobileStateProps = {
  kind: MobileStateKind;
  title?: string;
  description?: string;
  actionLabel?: string;
  onAction?: () => void;
};

const DEFAULT_TITLES: Record<MobileStateKind, string> = {
  loading: '正在加载',
  empty: '暂无数据',
  error: '加载失败',
  forbidden: '无权访问',
  'not-found': '未找到',
};

export function MobileState({
  kind,
  title = DEFAULT_TITLES[kind],
  description,
  actionLabel,
  onAction,
}: MobileStateProps) {
  return (
    <section
      aria-live={kind === 'loading' ? 'polite' : undefined}
      className={`mobile-state mobile-state--${kind}`}
      role={kind === 'error' ? 'alert' : 'status'}
    >
      <h2 className="mobile-state__title">{title}</h2>
      {description === undefined ? null : (
        <p className="mobile-state__description">{description}</p>
      )}
      {actionLabel !== undefined && onAction !== undefined ? (
        <button className="mobile-state__action" type="button" onClick={onAction}>
          {actionLabel}
        </button>
      ) : null}
    </section>
  );
}

export type MobileErrorBoundaryProps = {
  children: ReactNode;
  fallback?: ReactNode;
};

type MobileErrorBoundaryState = {
  hasError: boolean;
};

export class MobileErrorBoundary extends Component<
  MobileErrorBoundaryProps,
  MobileErrorBoundaryState
> {
  state: MobileErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): MobileErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(_error: Error, _info: ErrorInfo): void {
    // Deliberately avoid logging errors here: callers may report sanitized context.
  }

  render(): ReactNode {
    if (this.state.hasError) {
      return this.props.fallback ?? (
        <MobileState
          kind="error"
          title="页面暂时不可用"
          description="请稍后重试。"
        />
      );
    }
    return this.props.children;
  }
}
