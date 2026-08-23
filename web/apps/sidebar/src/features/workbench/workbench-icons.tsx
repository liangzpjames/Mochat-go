export type WorkbenchIconKind =
  | 'contacts' | 'groups' | 'material' | 'batch' | 'sop'
  | 'message' | 'shield' | 'building' | 'reload' | 'chevron';

export function WorkbenchIcon({ kind }: { kind: WorkbenchIconKind }) {
  if (kind === 'contacts') return <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="9" cy="8" r="3" /><path d="M3.8 18c.6-3.1 2.35-4.65 5.2-4.65s4.6 1.55 5.2 4.65M16 7.2h4M18 5.2v4" /></svg>;
  if (kind === 'groups') return <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="9" cy="8" r="3" /><circle cx="17" cy="9" r="2.4" /><path d="M3.5 18c.65-3 2.48-4.5 5.5-4.5s4.85 1.5 5.5 4.5M14 14.2c3.3-.4 5.4.85 6.3 3.8" /></svg>;
  if (kind === 'material') return <svg aria-hidden="true" viewBox="0 0 24 24"><rect x="4" y="4" width="16" height="16" rx="3" /><circle cx="9" cy="9" r="1.6" /><path d="m6.5 17 3.8-4 2.7 2.5 2.5-2.4 2 2" /></svg>;
  if (kind === 'batch') return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M6 5h12v14H6zM9 9h6M9 13h6M9 17h4M4 7v10M20 7v10" /></svg>;
  if (kind === 'sop') return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M7 4h10a2 2 0 0 1 2 2v13l-3-2-4 2-4-2-3 2V6a2 2 0 0 1 2-2Z" /><path d="M9 9h6M9 13h5" /></svg>;
  if (kind === 'message') return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M4 5h16v12H9l-5 3Z" /><path d="M8 10h8M8 13h5" /></svg>;
  if (kind === 'shield') return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m12 3 7 3v5c0 4.5-2.35 7.8-7 10-4.65-2.2-7-5.5-7-10V6Z" /><path d="m9 12 2 2 4-4" /></svg>;
  if (kind === 'building') return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M5 20V5h10v15M15 10h4v10M3 20h18M8 9h4M8 13h4" /></svg>;
  if (kind === 'reload') return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M19 8a8 8 0 1 0 .4 7M19 4v4h-4" /></svg>;
  if (kind === 'chevron') return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m9 5 7 7-7 7" /></svg>;
  return null;
}
