import type { ReactNode } from 'react';
export function Phase35DetailDrawer({ title, open, onClose, children }: { title: string; open: boolean; onClose: () => void; children: ReactNode }) {
  if (!open) return null;
  return <aside role="dialog" aria-label={title}><header><h2>{title}</h2><button type="button" onClick={onClose}>关闭</button></header>{children}</aside>;
}
