import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import type { Phase35Api, Row } from './api';
import { records, text } from './api';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { Phase35DetailDrawer } from './components/detail-drawer';
import { MetricCardGrid } from './components/metric-card-grid';

const friendName = (row: Row) => String(row.name ?? row.contactName ?? '--');
const friendOwner = (row: Row) => String(row.employeeName ?? row.ownerName ?? '--');
const friendTags = (row: Row) => Array.isArray(row.tag)
  ? row.tag.map((tag) => String((tag as Row)?.tagName ?? tag)).join('、')
  : Array.isArray(row.tags) ? row.tags.map(text).join('、') : text(row.tagNames);

export function FriendsPage({ api }: { api: Phase35Api }) {
  const [keyword, setKeyword] = useState('');
  const [owner, setOwner] = useState('');
  const [applied, setApplied] = useState({ keyword: '', owner: '' });
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<Row>();
  const list = useQuery({ queryKey: ['phase35-friends', applied, page], queryFn: () => api.read('/workContact/index', { keyWords: applied.keyword, employeeId: applied.owner, page, perPage: 20 }) });
  const selectedContactId = Number(selected?.contactId ?? selected?.id ?? 0);
  const selectedEmployeeId = Number(selected?.employeeId ?? selected?.ownerId ?? 0);
  const detailReady = Boolean(selected && selectedContactId > 0 && selectedEmployeeId > 0);
  const detail = useQuery({ queryKey: ['phase35-friend', selectedContactId, selectedEmployeeId], enabled: detailReady, queryFn: () => api.read('/workContact/show', { contactId: selectedContactId, employeeId: selectedEmployeeId }) });
  const rows = records(list.data);
  const payload = (list.data ?? {}) as Row;
  const detailRow = (detail.data ?? {}) as Row;
  const total = Number(payload.total ?? rows.length);
  return (
    <Phase35PageShell title="好友" description="查询企业微信联系人、负责人、标签和关联业务记录">
      <div className="phase35-page">
        <section className="phase35-card phase35-filter-card">
          <form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); setPage(1); setApplied({ keyword: keyword.trim(), owner: owner.trim() }); }}>
            <label>好友名称<input aria-label="好友名称" value={keyword} onChange={(event) => setKeyword(event.target.value)} /></label>
            <label>负责人<input aria-label="负责人" value={owner} onChange={(event) => setOwner(event.target.value)} /></label>
            <button type="submit">查询</button>
            <button type="button" onClick={() => { setKeyword(''); setOwner(''); setPage(1); setApplied({ keyword: '', owner: '' }); }}>重置</button>
          </form>
        </section>
        <MetricCardGrid items={[{ label: '好友总数', value: total }, { label: '当前页展示', value: rows.length }]} />
        <section className="phase35-card">
          <header className="phase35-card-header"><div><h2>好友列表</h2><p>企业微信联系人及其负责人、标签</p></div><span className="phase35-chip">共 {total} 条</span></header>
          <div className="phase35-table-card">
            <Phase35DataState loading={list.isLoading} error={list.isError} empty={!list.isLoading && !rows.length} emptyContent={<section aria-label="好友数据接入说明"><h2>还没有可展示的企业微信好友</h2><p>好友数据来自企业微信通讯录同步。请先完成企业微信接入和联系人同步，再查看负责人、标签与来源。</p><a href="/customer/contact">查看客户联系人</a></section>} onRetry={() => void list.refetch()}>
              <table><thead><tr><th>好友</th><th>备注</th><th>负责人</th><th>标签</th><th>来源</th><th>更新时间</th><th>操作</th></tr></thead>
              <tbody>{rows.map((row) => (
                <tr key={String(row.id ?? row.contactId ?? 0)}>
                  <td>{friendName(row)}</td>
                  <td>{text(row.remark)}</td>
                  <td>{friendOwner(row)}</td>
                  <td>{friendTags(row)}</td>
                  <td>{text(row.addWayText ?? row.source)}</td>
                  <td>{text(row.createTime ?? row.updatedAt)}</td>
                  <td><button type="button" aria-label={`查看 ${friendName(row)}`} onClick={() => setSelected(row)}>查看详情</button></td>
                </tr>))}
              </tbody></table>
              <p>第 {page} 页，共 {total} 条</p>
            </Phase35DataState>
          </div>
        </section>
        <Phase35DetailDrawer title="好友详情" open={Boolean(selected)} onClose={() => setSelected(undefined)}>
          {!detailReady ? <p role="status">暂无可用详情数据</p> : detail.isLoading ? <p>正在加载详情…</p> : detail.isError ? <p role="alert">详情加载失败</p> : (
            <dl>
              <dt>姓名</dt><dd>{text(detailRow.name ?? selected?.name)}</dd>
              <dt>负责人</dt><dd>{friendOwner(detailRow)}</dd>
              <dt>标签</dt><dd>{friendTags(detailRow)}</dd>
              <dt>来源</dt><dd>{text(detailRow.addWayText ?? detailRow.source ?? detailRow.genderText)}</dd>
              <dt>备注</dt><dd>{text(detailRow.remark ?? detailRow.description)}</dd>
            </dl>)}
        </Phase35DetailDrawer>
      </div>
    </Phase35PageShell>
  );
}
