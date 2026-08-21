import { useEffect, useRef, type CSSProperties, type ReactNode } from 'react';

export function RiskWarningDrawer({
  open,
  title,
  description,
  children,
  onClose,
  width = '600px',
}: {
  open: boolean;
  title: string;
  description?: string;
  children: ReactNode;
  onClose: () => void;
  width?: string;
}) {
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const restoreFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!open) return undefined;
    restoreFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    closeButtonRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      restoreFocusRef.current?.focus();
    };
  }, [onClose, open]);

  if (!open) return null;

  return (
    <div className="risk-warning-drawer-layer">
      <button
        type="button"
        aria-label="关闭详情遮罩"
        data-testid="risk-warning-drawer-overlay"
        className="risk-warning-drawer-overlay"
        onClick={onClose}
      />
      <aside
        className="risk-warning-drawer"
        style={{ '--risk-warning-drawer-width': width } as CSSProperties}
        aria-label={title}
        role="dialog"
        aria-modal="true"
      >
        <header className="risk-warning-drawer-header">
          <div>
            <h2>{title}</h2>
            {description ? <p>{description}</p> : null}
          </div>
          <button ref={closeButtonRef} type="button" className="risk-warning-icon-button" aria-label="关闭" onClick={onClose}>×</button>
        </header>
        <div className="risk-warning-drawer-body">{children}</div>
      </aside>
    </div>
  );
}
