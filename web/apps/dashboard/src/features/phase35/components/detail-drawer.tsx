import type { ReactNode } from 'react';
export function Phase35DetailDrawer({ title, open, onClose, children }: { title: string; open: boolean; onClose: () => void; children: ReactNode }) {
  if (!open) return null;
  return <aside role="dialog" aria-modal="true" aria-label={title} className="dashboard-detail-drawer"><header><h2>{title}</h2><button type="button" aria-label={`关闭${title}`} onClick={onClose}>关闭</button></header>{children}</aside>;
}
