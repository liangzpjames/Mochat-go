import { useEffect, useMemo, useRef } from 'react';
import type { ConversationTrajectoryDay, ConversationTrajectoryEvent, ConversationTrajectoryType } from './conversation-global-api';

const metricCards = [
  { key: 'internalSingle', title: '内部单聊', subject: '沟通同事数' },
  { key: 'externalSingle', title: '外部单聊', subject: '沟通客户数' },
  { key: 'internalGroup', title: '内部群聊', subject: '内部群数' },
  { key: 'externalGroup', title: '外部群聊', subject: '沟通客户群数' },
] as const;

type Props = { data: ConversationTrajectoryDay | undefined; date: string; conversationType: ConversationTrajectoryType; isLoading: boolean; onDateChange(date: string): void; onTypeChange(type: ConversationTrajectoryType): void; onOpenEvent(event: ConversationTrajectoryEvent, button: HTMLButtonElement): void };

function formatTime(value: string): string { return value.includes(' ') ? value.slice(11, 16) : value.slice(-5); }
function targetLabel(event: ConversationTrajectoryEvent): string { return event.targetName || (event.targetStatus === 'missing' ? '未知会话对象' : event.targetType === 'room' ? '未命名客户群' : '未命名对象'); }

export function moveTrajectoryDate(date: string, delta: number): string {
  const [yearText, monthText, dayText] = date.split('-');
  const year = Number(yearText ?? '');
  const month = Number(monthText ?? '');
  const day = Number(dayText ?? '');
  if (![year, month, day].every(Number.isFinite)) return date;
  const base = new Date(Date.UTC(year, month - 1, day));
  base.setUTCDate(base.getUTCDate() + delta);
  return base.toISOString().slice(0, 10);
}

export function ConversationTrajectoryTimeline(props: Props) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const eventsByHour = useMemo(() => {
    const groups = new Map<string, ConversationTrajectoryEvent[]>();
    for (const event of props.data?.events ?? []) groups.set(event.hour, [...(groups.get(event.hour) ?? []), event]);
    return groups;
  }, [props.data?.events]);
  useEffect(() => {
    const scrollContainer = scrollRef.current;
    if (!props.data || !scrollContainer) return;
    const earliest = [...eventsByHour.keys()].sort()[0];
    const target = scrollContainer.querySelector<HTMLElement>(`#trajectory-hour-${earliest && earliest < '08' ? earliest : '08'}`);
    if (!target) return;
    const targetTop = target.getBoundingClientRect().top - scrollContainer.getBoundingClientRect().top + scrollContainer.scrollTop;
    const nextScrollTop = Math.max(0, targetTop - 12);
    if (typeof scrollContainer.scrollTo === 'function') scrollContainer.scrollTo({ top: nextScrollTop, behavior: 'auto' });
    else scrollContainer.scrollTop = nextScrollTop;
  }, [props.data?.employee.id, props.data?.date]);
  const currentDate = props.date;
  const moveDate = (delta: number) => {
    props.onDateChange(moveTrajectoryDate(currentDate, delta));
  };
  return <section className="conversation-trajectory-main" aria-label="会话轨迹时间轴">
    <header className="conversation-trajectory-toolbar">
      <div><button type="button" aria-label="前一天" onClick={() => moveDate(-1)}>‹</button><strong>{props.data?.date ?? currentDate}</strong><button type="button" aria-label="后一天" disabled={currentDate >= new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai' }).format(new Date())} onClick={() => moveDate(1)}>›</button></div>
      <select aria-label="会话类型" value={props.conversationType} onChange={(event) => props.onTypeChange(event.target.value as ConversationTrajectoryType)}><option value="all">全部会话</option><option value="employee">内部单聊</option><option value="customer">外部单聊</option><option value="room">客户群</option></select>
    </header>
    {props.data?.limitations.map((item) => <p role="status" className="conversation-trajectory-limitation" key={item.key}>{item.reason}</p>)}
    <div className="conversation-trajectory-metrics">{metricCards.map((card) => { const metric = props.data?.metrics[card.key]; return <article key={card.key}><span>{card.title}</span><strong>{metric?.status === 'available' ? metric.subjectTotal ?? 0 : '--'}</strong><small>{metric?.status === 'available' ? `${card.subject} · ${metric.messageTotal ?? 0} 条消息` : metric?.reason ?? '暂不可用'}</small></article>; })}</div>
    <div className="conversation-trajectory-scroll" ref={scrollRef}>{Array.from({ length: 24 }, (_, hour) => { const key = String(hour).padStart(2, '0'); const events = eventsByHour.get(key) ?? []; return <section className="conversation-trajectory-hour" data-testid="trajectory-hour" id={`trajectory-hour-${key}`} aria-label={`${key}:00`} key={key}><time>{key}:00</time><div>{events.length === 0 && <span className="conversation-trajectory-hour-empty">暂无活动</span>}{events.map((event) => <button type="button" key={event.id} aria-label={`${targetLabel(event)} ${event.messageTotal} 条 ${formatTime(event.firstMessageAt)}-${formatTime(event.lastMessageAt)}`} onClick={(element) => props.onOpenEvent(event, element.currentTarget)}><strong>{targetLabel(event)}</strong><span>{event.messageTotal} 条消息 · {formatTime(event.firstMessageAt)}-{formatTime(event.lastMessageAt)}</span></button>)}</div></section>; })}</div>
  </section>;
}
