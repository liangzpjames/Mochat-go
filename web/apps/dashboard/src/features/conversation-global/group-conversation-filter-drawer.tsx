import { useEffect, useState } from 'react';

import type { GroupRoomFilterOptions } from './conversation-global-api';

export type GroupConversationFilterValue = { employeeId: string; customerId: string; groupId: string };
type Props = { open: boolean; value: GroupConversationFilterValue; options: GroupRoomFilterOptions | undefined; onSubmit: (value: GroupConversationFilterValue) => void; onClose: () => void };

export function GroupConversationFilterDrawer({ open, value, options, onSubmit, onClose }: Props) {
  const [draft, setDraft] = useState(value);
  useEffect(() => { if (open) setDraft(value); }, [open, value]);
  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onClose, open]);
  if (!open) return null;
  const select = (key: keyof GroupConversationFilterValue, label: string, items: readonly { value: string; label: string; count: number }[]) => <label><span>{label}</span><select aria-label={label} value={draft[key]} onChange={(event) => setDraft((current) => ({ ...current, [key]: event.target.value }))}><option value="">全部</option>{items.map((item) => <option key={item.value} value={item.value}>{item.label} · {item.count}</option>)}</select></label>;
  return <div className="group-conversation-filter-backdrop" data-testid="group-filter-backdrop" onClick={onClose} role="presentation"><aside aria-label="筛选群聊" aria-modal="true" className="group-conversation-filter-drawer" onClick={(event) => event.stopPropagation()} role="dialog"><header><div><p className="group-conversation-eyebrow">群聊筛选</p><h2>按真实数据定位</h2></div><button aria-label="关闭筛选" onClick={onClose} type="button">关闭</button></header><div className="group-conversation-filter-fields">{select('employeeId', '员工', options?.employees ?? [])}{select('customerId', '客户', options?.customers ?? [])}{select('groupId', '群分组', options?.groups ?? [])}</div>{options?.capabilities.filter((item) => !item.available).map((item) => <p className="group-conversation-limitation" key={item.key}>{item.reason ?? `${item.key} 暂未接入`}</p>)}<footer><button onClick={() => onSubmit({ employeeId: '', customerId: '', groupId: '' })} type="button">重置</button><button className="group-conversation-query-button" onClick={() => onSubmit(draft)} type="button">查询筛选</button></footer></aside></div>;
}
