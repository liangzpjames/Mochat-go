import type { FormEvent, ReactNode } from 'react';

export interface FilterBarProps {
  children: ReactNode;
  onSearch: () => void;
  onReset: () => void;
}

export function FilterBar({ children, onSearch, onReset }: FilterBarProps) {
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSearch();
  }

  return (
    <form role="search" onSubmit={submit}>
      {children}
      <button type="submit">查询</button>
      <button type="button" onClick={onReset}>
        重置
      </button>
    </form>
  );
}
