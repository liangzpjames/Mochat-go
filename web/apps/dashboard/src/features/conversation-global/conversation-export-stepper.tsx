import type { ConversationExportType } from './conversation-global-api';

const cards: readonly { type: ConversationExportType; title: string; description: string; icon: string }[] = [
  { type: 'employee', title: '按员工导出', description: '查看员工名下可访问的客户单聊与外部群聊消息。', icon: '人' },
  { type: 'customer', title: '按客户导出', description: '按客户聚合导出已归档的单聊和外部群聊消息。', icon: '客' },
  { type: 'room', title: '按群聊导出', description: '选择已接入的外部客户群，导出完整归档消息。', icon: '群' },
];

export function ConversationExportStepper({
  value,
  onChange,
  onNext,
}: {
  value: ConversationExportType;
  onChange: (value: ConversationExportType) => void;
  onNext: () => void;
}) {
  return <section className="conversation-export-stepper" aria-label="导出类型">
    <div className="conversation-export-stepper-track">
      <span className="conversation-export-step is-active"><b>1</b><span>选择导出对象</span></span>
      <i aria-hidden="true" />
      <span className="conversation-export-step"><b>2</b><span>设置导出条件</span></span>
    </div>
    <div className="conversation-export-type-grid">
      {cards.map((card) => <button
        className={`conversation-export-type-card${value === card.type ? ' is-selected' : ''}`}
        key={card.type}
        type="button"
        aria-label={card.title}
        aria-pressed={value === card.type}
        onClick={() => onChange(card.type)}
      >
        <span className="conversation-export-type-icon">{card.icon}</span>
        <span><strong>{card.title}</strong><small>{card.description}</small></span>
        <span className="conversation-export-type-check" aria-hidden="true">{value === card.type ? '✓' : ''}</span>
      </button>)}
    </div>
    <div className="conversation-export-stepper-actions">
      <button className="conversation-export-primary" type="button" onClick={onNext}>下一步</button>
    </div>
  </section>;
}

export function exportTypeLabel(type: ConversationExportType): string {
  return type === 'employee' ? '员工' : type === 'customer' ? '客户' : '群聊';
}
