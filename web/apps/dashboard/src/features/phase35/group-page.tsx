import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import type { Phase35Api, Row } from './api';
import { numericId, records, text } from './api';

export function GroupPage({ api }: { api: Phase35Api }) {
  const [keyword, setKeyword] = useState(''); const [applied, setApplied] = useState(''); const [selected, setSelected] = useState<Row>();
  const list = useQuery({ queryKey: ['phase35-groups', applied], queryFn: () => api.read('/workRoom/index', { name: applied, page: 1, perPage: 20 }) });
  const detail = useQuery({ queryKey: ['phase35-group', selected ? numericId(selected) : 0], enabled: !!selected, queryFn: () => api.read('/workRoom/roomIndex', { roomId: numericId(selected!) }) });
  const rows = records(list.data); const members = records(detail.data && typeof detail.data === 'object' ? (detail.data as Row).members : undefined);
  return <section className="dashboard-content"><header className="dashboard-page-header"><div><p className="dashboard-eyebrow">SCRM · 客户群</p><h1>客户群</h1><p>查询真实群、群主、成员和群统计。</p></div></header><div className="dashboard-filter-bar"><label>群名称<input aria-label="群名称" value={keyword} onChange={e => setKeyword(e.target.value)} /></label><button type="button" onClick={() => setApplied(keyword.trim())}>查询</button><button type="button" onClick={() => void list.refetch()}>刷新</button></div>{list.isLoading ? <p>加载中…</p> : list.isError ? <p role="alert">客户群加载失败</p> : rows.length === 0 ? <p>暂无符合条件的客户群。</p> : <table><thead><tr><th>群名称</th><th>群主</th><th>成员</th><th>操作</th></tr></thead><tbody>{rows.map(row => <tr key={numericId(row)}><td>{text(row.name ?? row.roomName)}</td><td>{text(row.ownerName)}</td><td>{text(row.memberCount)}</td><td><button type="button" aria-label={`查看 ${text(row.name ?? row.roomName)}`} onClick={() => setSelected(row)}>查看</button></td></tr>)}</tbody></table>}{selected && <aside role="complementary" aria-label="客户群详情"><button type="button" onClick={() => setSelected(undefined)}>关闭</button>{detail.isLoading ? <p>详情加载中…</p> : detail.isError ? <p role="alert">详情加载失败</p> : <>{members.map(member => <p key={numericId(member)}>{text(member.name)}</p>)}<pre>{JSON.stringify(detail.data, null, 2)}</pre></>}</aside>}</section>;
}
