import { useQuery } from '@tanstack/react-query';
import { useOptionalDashboardAccess } from '../../app/access-context';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import type { AiInsightApi } from './ai-insight-api';

export const aiInsightPageConfigs = {
  'session-analysis': { title: '会话分析', description: '按会话维度分析沟通量、时长与关键词', page: 'session-analysis' },
  'smart-analysis': { title: '智能分析', description: 'AI 对话摘要、意图识别与跟进建议', page: 'smart-analysis' },
  emotion: { title: '情绪识别', description: '按会话识别客户情绪倾向与异常波动', page: 'emotion' },
  'employee-score': { title: '员工评分', description: '按员工评估响应时效与沟通质量', page: 'employee-score' },
  'communication-keyword': { title: '沟通关键词', description: '会话高频词、敏感词命中与趋势', page: 'communication-keyword' },
} as const;

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
  const copy = {
    'session-analysis': {
      notice: '会话分析需要会话存档数据。当前未接入会话存档 Provider，暂无分析结果；接入后本页将按会话自动生成分析。',
      empty: '当前未接入会话存档 Provider，暂无会话分析结果。',
    },
    'smart-analysis': {
      notice: '智能分析需要 AI 模型服务。当前未接入 AI Provider，智能分析暂不可用；配置后将提供对话摘要、意图识别与跟进建议。',
      empty: '当前未接入 AI Provider，暂无智能分析结果。',
    },
    emotion: {
      notice: '情绪识别需要情绪分析模型服务。当前未接入 AI Provider，情绪分析暂不可用；配置后将按会话展示情绪倾向与异常波动。',
      empty: '当前未接入 AI Provider，暂无情绪识别结果。',
    },
    'employee-score': {
      notice: '员工评分需要评分模型与会话存档数据。当前未接入相关 Provider，员工评分暂不可用；接入后将按员工展示响应时效、沟通质量与得分。',
      empty: '当前未接入评分所需 Provider，暂无员工评分结果。',
    },
    'communication-keyword': {
      notice: '沟通关键词统计需要会话存档数据。当前未接入会话存档 Provider，关键词统计暂不可用；接入后将展示高频词、敏感词命中与趋势。',
      empty: '当前未接入会话存档 Provider，暂无沟通关键词统计结果。',
    },
  }[page] ?? { notice: '', empty: '' };

  return (
    <Phase35PageShell title={config.title} description={config.description} actions={<span className="phase35-chip">AI 能力状态：{limited ? '未接入' : '已就绪'}</span>}>
      <div className="phase35-page">
        <section className="phase35-card">
          <header className="phase35-card-header"><div><h2>能力说明</h2><p>本页由 AI 分析能力提供数据</p></div></header>
          <Phase35DataState
            loading={query.isLoading}
            error={query.isError}
            onRetry={() => void query.refetch()}
          >
            {limited ? (
              <ul className="phase35-limits" role="status">
                {limitations.map((item) => <li key={item}>{item}</li>)}
              </ul>
            ) : (
              <p role="status">AI 分析能力已就绪</p>
            )}
          </Phase35DataState>
        </section>

        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header"><div><h2>分析结果</h2><p>按分析任务读取结果，无数据时展示空状态</p></div><span className="phase35-chip">{limited ? '受限' : `${result?.data.length ?? 0} 条`}</span></header>
          {limited ? (
            <p className="phase35-empty">Provider 受限，暂无分析结果。</p>
          ) : (
            <Phase35DataState loading={query.isLoading} error={query.isError} empty={!result?.data.length} onRetry={() => void query.refetch()}>
              <div className="phase35-table"><table><thead><tr><th>会话</th><th>结果</th></tr></thead><tbody>{result?.data.map((item, index) => (
                <tr key={index}><td>{String((item as { sessionId?: string }).sessionId ?? index + 1)}</td><td>{String((item as { summary?: string }).summary ?? '—')}</td></tr>
              ))}</tbody></table></div>
            </Phase35DataState>
          )}
        </section>
      </div>
    </Phase35PageShell>
  );
}
