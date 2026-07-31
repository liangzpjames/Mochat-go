import { ApiError } from '@mochat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { updateSearch } from '../../shared/query-state';
import type {
  ConversationGlobalApi,
  ConversationMessage,
  ConversationSearch,
  ConversationTargetType,
} from './conversation-global-api';

type FilterDraft = Pick<
  ConversationSearch,
  'keyword' | 'employeeId' | 'customerId' | 'roomId' | 'from' | 'to'
>;

const defaultPageSize = 20;

function positiveInteger(value: string | null, fallback: number): number {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function filtersFromSearch(search: URLSearchParams): FilterDraft {
  return {
    keyword: search.get('keyword') ?? '',
    employeeId: search.get('employeeId') ?? '',
    customerId: search.get('customerId') ?? '',
    roomId: search.get('roomId') ?? '',
    from: search.get('from') ?? '',
    to: search.get('to') ?? '',
  };
}

function targetTypeLabel(type: ConversationTargetType): string {
  switch (type) {
    case 'employee':
      return '员工';
    case 'customer':
      return '客户';
    case 'room':
      return '群聊';
  }
}

function messageText(message: ConversationMessage): string {
  const content = message.content.content;
  if (typeof content === 'string' && content.trim() !== '') {
    return content;
  }
  return JSON.stringify(message.content);
}

export function ConversationGlobalPage({ api }: { api: ConversationGlobalApi }) {
  const access = useDashboardAccess();
  const [searchParams, setSearchParams] = useSearchParams();
  const currentFilters = filtersFromSearch(searchParams);
  const [draft, setDraft] = useState<FilterDraft>(currentFilters);
  const [filterError, setFilterError] = useState<string | null>(null);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const page = positiveInteger(searchParams.get('page'), 1);
  const pageSize = Math.min(
    positiveInteger(searchParams.get('pageSize'), defaultPageSize),
    100,
  );
  const searchText = searchParams.toString();

  useEffect(() => {
    setDraft(filtersFromSearch(new URLSearchParams(searchText)));
  }, [searchText]);

  const input = useMemo<ConversationSearch>(() => ({
    corpId: access.corp.id,
    ...currentFilters,
    page,
    pageSize,
  }), [
    access.corp.id,
    currentFilters.customerId,
    currentFilters.employeeId,
    currentFilters.from,
    currentFilters.keyword,
    currentFilters.roomId,
    currentFilters.to,
    page,
    pageSize,
  ]);

  const listQuery = useQuery({
    queryKey: [
      'corp',
      access.corp.id,
      'conversation-global',
      input.keyword,
      input.employeeId,
      input.customerId,
      input.roomId,
      input.from,
      input.to,
      input.page,
      input.pageSize,
    ],
    queryFn: () => api.search(input),
  });
  const detailQuery = useQuery({
    queryKey: ['corp', access.corp.id, 'conversation-global-detail', selectedID],
    queryFn: () => api.detail(selectedID ?? ''),
    enabled: selectedID !== null,
    retry: false,
  });

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if ((draft.from === '') !== (draft.to === '')) {
      setFilterError('开始日期和结束日期需要同时填写');
      return;
    }
    if (draft.from !== '' && draft.from > draft.to) {
      setFilterError('开始日期不能晚于结束日期');
      return;
    }
    if (draft.customerId.trim() !== '' && draft.roomId.trim() !== '') {
      setFilterError('客户和群聊筛选不能同时使用');
      return;
    }
    setFilterError(null);
    setSearchParams(updateSearch(searchParams, {
      keyword: draft.keyword,
      employeeId: draft.employeeId,
      customerId: draft.customerId,
      roomId: draft.roomId,
      from: draft.from,
      to: draft.to,
      page: 1,
      pageSize,
    }));
  }

  function changePage(nextPage: number) {
    setSearchParams(updateSearch(searchParams, { page: nextPage, pageSize }));
  }

  const forbidden = listQuery.error instanceof ApiError
    && listQuery.error.kind === 'forbidden';
  const detailNotFound = detailQuery.error instanceof ApiError
    && detailQuery.error.status === 404;
  const totalPages = Math.max(1, Math.ceil((listQuery.data?.total ?? 0) / pageSize));

  return (
    <section className="conversation-global-page">
      <header className="conversation-global-header">
        <div>
          <p className="conversation-global-eyebrow">会话存档</p>
          <h1>全局消息</h1>
          <p>查询当前企业内有权限查看的员工、客户与群聊会话。</p>
        </div>
      </header>

      <form className="conversation-global-filters" onSubmit={applyFilters}>
        <label>
          <span>关键词</span>
          <input
            aria-label="关键词"
            onChange={(event) => setDraft((value) => ({ ...value, keyword: event.target.value }))}
            placeholder="搜索会话对象或消息内容"
            value={draft.keyword}
          />
        </label>
        <label>
          <span>员工 ID</span>
          <input
            aria-label="员工 ID"
            inputMode="numeric"
            onChange={(event) => setDraft((value) => ({ ...value, employeeId: event.target.value }))}
            value={draft.employeeId}
          />
        </label>
        <label>
          <span>客户 ID</span>
          <input
            aria-label="客户 ID"
            inputMode="numeric"
            onChange={(event) => setDraft((value) => ({ ...value, customerId: event.target.value }))}
            value={draft.customerId}
          />
        </label>
        <label>
          <span>群聊 ID</span>
          <input
            aria-label="群聊 ID"
            inputMode="numeric"
            onChange={(event) => setDraft((value) => ({ ...value, roomId: event.target.value }))}
            value={draft.roomId}
          />
        </label>
        <label>
          <span>开始日期</span>
          <input
            aria-label="开始日期"
            onChange={(event) => setDraft((value) => ({ ...value, from: event.target.value }))}
            type="date"
            value={draft.from}
          />
        </label>
        <label>
          <span>结束日期</span>
          <input
            aria-label="结束日期"
            onChange={(event) => setDraft((value) => ({ ...value, to: event.target.value }))}
            type="date"
            value={draft.to}
          />
        </label>
        <button type="submit">查询</button>
      </form>

      {filterError !== null && <p className="conversation-global-inline-error" role="alert">{filterError}</p>}

      {listQuery.isPending && (
        <div className="conversation-global-state" role="status">正在加载全局消息…</div>
      )}
      {forbidden && (
        <div className="conversation-global-state conversation-global-error">
          <h2>无权查看当前企业会话</h2>
          <p>请联系管理员开通会话存档和数据范围权限。</p>
        </div>
      )}
      {listQuery.isError && !forbidden && (
        <div className="conversation-global-state conversation-global-error" role="alert">
          <h2>全局消息加载失败</h2>
          <p>{listQuery.error instanceof Error ? listQuery.error.message : '请稍后重试'}</p>
          <button onClick={() => void listQuery.refetch()} type="button">重试</button>
        </div>
      )}
      {listQuery.data?.list.length === 0 && listQuery.data.total > 0 && (
        <div className="conversation-global-state">
          <h2>当前页暂无会话</h2>
          <p>该页码已超出当前结果范围。</p>
          <button onClick={() => changePage(1)} type="button">返回第一页</button>
        </div>
      )}
      {listQuery.data?.list.length === 0 && listQuery.data.total === 0 && (
        <div className="conversation-global-state">
          <h2>当前筛选条件下暂无会话</h2>
          <p>清除部分筛选条件后重新查询。</p>
        </div>
      )}
      {listQuery.data !== undefined && listQuery.data.list.length > 0 && (
        <div className="conversation-global-results">
          <div className="conversation-global-table-wrap">
            <table>
              <thead>
                <tr>
                  <th>员工</th>
                  <th>会话对象</th>
                  <th>类型</th>
                  <th>最近消息</th>
                  <th>最近时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {listQuery.data.list.map((item) => (
                  <tr key={item.id}>
                    <td>{item.employeeName || `员工 ${item.employeeId}`}</td>
                    <td>{item.targetName || `${targetTypeLabel(item.targetType)} ${item.targetId}`}</td>
                    <td>{targetTypeLabel(item.targetType)}</td>
                    <td className="conversation-global-message-preview">{item.lastMessage || '--'}</td>
                    <td>{item.sentAt || '--'}</td>
                    <td>
                      <button onClick={() => setSelectedID(item.id)} type="button">查看会话</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <footer className="conversation-global-pagination">
            <span>共 {listQuery.data.total} 条，第 {page}/{totalPages} 页</span>
            <div>
              <button disabled={page <= 1} onClick={() => changePage(page - 1)} type="button">上一页</button>
              <button
                disabled={page >= totalPages}
                onClick={() => changePage(page + 1)}
                type="button"
              >
                下一页
              </button>
            </div>
          </footer>
        </div>
      )}

      {selectedID !== null && (
        <div className="conversation-global-drawer-backdrop">
          <aside aria-label="会话详情" className="conversation-global-drawer" role="dialog">
            <header>
              <div>
                <p>会话详情</p>
                <h2>
                  {detailQuery.data === undefined
                    ? '正在读取会话'
                    : `${detailQuery.data.employeeName || `员工 ${detailQuery.data.employeeId}`} · ${detailQuery.data.targetName || `${targetTypeLabel(detailQuery.data.targetType)} ${detailQuery.data.targetId}`}`}
                </h2>
              </div>
              <button onClick={() => setSelectedID(null)} type="button">关闭</button>
            </header>
            {detailQuery.isPending && <p role="status">正在加载会话详情…</p>}
            {detailNotFound && (
              <div className="conversation-global-detail-error" role="alert">
                <p>会话不存在或已无权访问</p>
              </div>
            )}
            {detailQuery.isError && !detailNotFound && (
              <div className="conversation-global-detail-error" role="alert">
                <p>{detailQuery.error instanceof Error ? detailQuery.error.message : '详情加载失败'}</p>
                <button onClick={() => void detailQuery.refetch()} type="button">重试详情</button>
              </div>
            )}
            {detailQuery.data !== undefined && (
              <>
                {detailQuery.data.truncated && (
                  <p className="conversation-global-window-note">
                    当前显示最近 200 条，共 {detailQuery.data.messageTotal} 条消息
                  </p>
                )}
                <div className="conversation-global-messages">
                  {detailQuery.data.messages.map((message) => (
                    <article className={`conversation-global-message ${message.direction}`} key={message.id}>
                      <header>
                        <strong>{message.senderName || '未知成员'}</strong>
                        <time>{message.sentAt}</time>
                      </header>
                      <p>{messageText(message)}</p>
                    </article>
                  ))}
                </div>
              </>
            )}
          </aside>
        </div>
      )}
    </section>
  );
}
