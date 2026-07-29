import { renderToStaticMarkup } from 'react-dom/server';
import type { ButtonHTMLAttributes, ReactElement } from 'react';
import { describe, expect, it, vi } from 'vitest';

import { AsyncBoundary } from './async-boundary';
import { DataPage } from './data-page';
import { FilterBar } from './filter-bar';
import { StatusAction } from './status-action';

describe('AsyncBoundary', () => {
  it('renders loading, empty and retryable error states', () => {
    expect(renderToStaticMarkup(<AsyncBoundary state={{ status: 'loading' }} />)).toContain(
      '正在加载',
    );
    expect(
      renderToStaticMarkup(<AsyncBoundary state={{ status: 'ready', rows: [] }} />),
    ).toContain('暂无数据');
    expect(
      renderToStaticMarkup(
        <AsyncBoundary state={{ status: 'error', message: '网络错误', retry: vi.fn() }} />,
      ),
    ).toContain('重试');
  });
});

describe('DataPage', () => {
  it('exposes controlled page and page-size actions', () => {
    const markup = renderToStaticMarkup(
      <DataPage
        title="客户列表"
        page={2}
        perPage={20}
        total={45}
        onPageChange={vi.fn()}
        onPerPageChange={vi.fn()}
      >
        <div>列表内容</div>
      </DataPage>,
    );

    expect(markup).toContain('客户列表');
    expect(markup).toContain('第 2 / 3 页');
    expect(markup).toContain('value="20"');
  });
});

describe('FilterBar', () => {
  it('renders filters and explicit query and reset actions', () => {
    const markup = renderToStaticMarkup(
      <FilterBar onSearch={vi.fn()} onReset={vi.fn()}>
        <label>
          姓名
          <input />
        </label>
      </FilterBar>,
    );

    expect(markup).toContain('姓名');
    expect(markup).toContain('查询');
    expect(markup).toContain('重置');
  });
});

describe('StatusAction', () => {
  it('requires confirmation before exposing the mutation action', () => {
    const action = vi.fn();
    const confirm = vi.fn(() => false);
    vi.stubGlobal('window', { confirm });
    const element = StatusAction({
      label: '停用',
      confirmation: '确认停用该记录？',
      onConfirm: action,
    }) as ReactElement<ButtonHTMLAttributes<HTMLButtonElement>>;

    expect(element.props.onClick).not.toBe(action);
    element.props.onClick?.({} as never);
    expect(confirm).toHaveBeenCalledWith('确认停用该记录？');
    expect(action).not.toHaveBeenCalled();

    confirm.mockReturnValue(true);
    element.props.onClick?.({} as never);
    expect(action).toHaveBeenCalledOnce();
    vi.unstubAllGlobals();
  });
});
