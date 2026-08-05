import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import type { Phase35Api, Row } from './api';
import { numericId, records, text } from './api';

export function FriendsPage({ api }: { api: Phase35Api }) {
  const [keyword, setKeyword] = useState(''); const [applied, setApplied] = useState(''); const [selected, setSelected] = useState<Row>();
  const list = useQuery({ queryKey: ['phase35-friends', applied], queryFn: () => api.read('/workContact/index', { name: applied, page: 1, perPage: 20 }) });
  const detail = useQuery({ queryKey: ['phase35-friend', selected ? numericId(selected) : 0], enabled: !!selected, queryFn: () => api.read('/workContact/show', { id: numericId(selected!) }) });
  const rows = records(list.data);
  return <section className="dashboard-content"><header className="dashboard-page-header"><div><p className="dashboard-eyebrow">SCRM · 好友</p><h1>好友</h1><p>查询企微联系人、负责人和标签，详情来自服务端。</p></div></header><div className="dashboard-filter-bar"><label>好友名称<input aria-label="好友名称" value={keyword} onChange={e => setKeyword(e.target.value)} /></label><button type="button" onClick={() => setApplied(keyword.trim())}>查询</button><button type="button" onClick={() => void list.refetch()}>刷新</button></div>{list.isLoading ? <p>加载中…</p> : list.isError ? <p role="alert">好友加载失败：{String(list.error)}</p> : rows.length === 0 ? <p>暂无符合条件的好友。</p> : <table><thead><tr><th>好友</th><th>负责人</th><th>标签</th><th>操作</th></tr></thead><tbody>{rows.map(row => <tr key={numericId(row)}><td>{text(row.name ?? row.contactName)}</td><td>{text(row.employeeName ?? row.ownerName)}</td><td>{Array.isArray(row.tags) ? row.tags.map(text).join('、') : text(row.tagNames)}</td><td><button type="button" aria-label={`查看 ${text(row.name ?? row.contactName)}`} onClick={() => setSelected(row)}>查看</button></td></tr>)}</tbody></table>}{selected && <aside role="complementary" aria-label="好友详情"><button type="button" onClick={() => setSelected(undefined)}>关闭</button>{detail.isLoading ? <p>详情加载中…</p> : detail.isError ? <p role="alert">详情加载失败</p> : <pre>{JSON.stringify(detail.data, null, 2)}</pre>}</aside>}</section>;
}
