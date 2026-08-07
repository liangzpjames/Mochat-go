import { useQuery } from '@tanstack/react-query';
import { useOptionalDashboardAccess } from '../../app/access-context';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import type { AiInsightApi } from './ai-insight-api';

const aiRestrictedCopy = {
  status: 'AI 能力未接入',
  detail: '当前未接入可用的 AI 分析 Provider，暂无分析结果。',
};

export const aiInsightPageConfigs = {
  'session-analysis': { title: '会话分析', description: '按会话维度分析沟通量、时长与关键词', page: 'session-analysis' },
  'smart-analysis': { title: '智能分析', description: 'AI 对话摘要、意图识别与跟进建议', page: 'smart-analysis' },
  emotion: { title: '情绪识别', description: '按会话识别客户情绪倾向与异常波动', page: 'emotion' },
  'employee-score': { title: '员工评分', description: '按员工评估响应时效与沟通质量', page: 'employee-score' },
  'communication-keyword': { title: '沟通关键词', description: '会话高频词、敏感词命中与趋势', page: 'communication-keyword' },
} as const;

function sessionLabel(value: unknown): string {
  const raw = String(value ?? '');
  if (raw === '' || raw === 'archive') return '归档会话';
  return raw;
}

function friendlyTime(value: string): string {
  if (!value) return '--';
  return `${value.slice(5, 10)} ${value.slice(11, 16)}`;
}

export function AiInsightPage({ api, page }: { api: AiInsightApi; page: keyof typeof aiInsightPageConfigs }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const config = aiInsightPageConfigs[page];
  const query = useQuery({
    queryKey: ['ai-insight', page, corpId],
    queryFn: () => api.read(config.page, Number(corpId)),
    enabled: Boolean(corpId),
  });
  const result = query.data;
  const limitations = result?.limitations ?? [];
  const limited = result?.capability !== 'ready';
  const generatedAt = result?.generatedAt ?? '';
  const count = result?.data.length ?? 0;

  return (
    <Phase35PageShell title={config.title} description={config.description} actions={<span className="phase35-chip">AI 能力状态：{limited ? aiRestrictedCopy.status : '已就绪'}</span>}>
      <div className="phase35-page">
        <section className="phase35-kpis" aria-label="AI 洞察指标">
          <article className="phase35-kpi phase35-kpi-primary">
            <span>分析结果</span><strong>{limited ? aiRestrictedCopy.status : count}</strong><small>{limited ? aiRestrictedCopy.detail : '当前返回的分析结果条数'}</small>
          </article>
          <article className="phase35-kpi phase35-kpi-green">
            <span>能力状态</span><strong>{limited ? aiRestrictedCopy.status : '已就绪'}</strong><small>AI Provider 当前状态</small>
          </article>
          <article className="phase35-kpi phase35-kpi-violet">
            <span>生成时间</span><strong>{friendlyTime(generatedAt)}</strong><small>{generatedAt ? '最近一次分析生成时间' : '尚未生成（AI 能力未接入）'}</small>
          </article>
        </section>

        <section className="phase35-card">
          <header className="phase35-card-header"><div><h2>能力说明</h2><p>本页由 AI 分析能力提供数据</p></div></header>
          <Phase35DataState loading={query.isLoading} error={query.isError} onRetry={() => void query.refetch()}>
            {limited ? (
              <>
                <ul className="phase35-limits" role="status">
                  {limitations.map((item) => <li key={item}>{item}</li>)}
                </ul>
                <p><a href="/ai-setting/agent">前往接入 AI 能力</a></p>
              </>
            ) : (
              <p role="status">AI 分析能力已就绪{generatedAt !== '' ? `（生成时间：${generatedAt.slice(0, 19).replace('T', ' ')}）` : ''}</p>
            )}
          </Phase35DataState>
        </section>

        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header"><div><h2>分析结果</h2><p>按分析任务读取结果，无数据时展示暂无数据状态</p></div><span className="phase35-chip">{limited ? '未接入' : `${count} 条`}</span></header>
          {limited ? (
            <p className="phase35-empty">{aiRestrictedCopy.detail}</p>
          ) : (
            <Phase35DataState loading={query.isLoading} error={query.isError} empty={!count} onRetry={() => void query.refetch()}>
              <div className="phase35-table">
                <table>
                  <thead><tr><th>会话</th><th>结果</th></tr></thead>
                  <tbody>{result?.data.map((item, index) => (
                    <tr key={index}>
                      <td>{sessionLabel((item as { sessionId?: string }).sessionId ?? index + 1)}</td>
                      <td className="phase35-ai-summary">{String((item as { summary?: string }).summary ?? '—')}</td>
                    </tr>
                  ))}</tbody>
                </table>
              </div>
            </Phase35DataState>
          )}
        </section>
      </div>
    </Phase35PageShell>
  );
}
