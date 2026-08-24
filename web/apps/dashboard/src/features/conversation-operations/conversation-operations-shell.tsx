import type { FormEvent, ReactNode } from 'react';

export function ConversationOperationsShell(props: {
  title: string;
  description: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="conversation-operations-page">
      <header className="conversation-operations-header">
        <div>
          <h1>{props.title}</h1>
          <p>{props.description}</p>
        </div>
        {props.actions && <div className="conversation-operations-header-actions">{props.actions}</div>}
      </header>
      {props.children}
    </section>
  );
}

export function ConversationTabs<T extends string>(props: {
  value: T;
  tabs: readonly { value: T; label: string }[];
  onChange(value: T): void;
}) {
  return (
    <div className="conversation-operations-tabs" role="tablist">
      {props.tabs.map((tab) => (
        <button
          aria-selected={props.value === tab.value}
          key={tab.value}
          onClick={() => props.onChange(tab.value)}
          role="tab"
          type="button"
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}

export function ConversationQueryBar(props: {
  children: ReactNode;
  fetching?: boolean;
  onQuery: () => void;
  onReset: () => void;
  onRefresh: () => void;
}) {
  const submit = (event: FormEvent) => {
    event.preventDefault();
    props.onQuery();
  };
  return (
    <form className="conversation-operations-query" onSubmit={submit}>
      <div className="conversation-operations-query-fields">{props.children}</div>
      <div className="conversation-operations-query-actions">
        <button className="is-primary" disabled={props.fetching} type="submit">
          查询
        </button>
        <button disabled={props.fetching} onClick={props.onReset} type="button">
          重置
        </button>
        <button disabled={props.fetching} onClick={props.onRefresh} type="button">
          {props.fetching ? '刷新中…' : '刷新'}
        </button>
      </div>
    </form>
  );
}
