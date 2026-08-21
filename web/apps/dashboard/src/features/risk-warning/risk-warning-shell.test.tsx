import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { RiskWarningDrawer, RiskWarningQueryBar } from './risk-warning-shell';

describe('risk warning shared shell', () => {
  it('submits the query form without querying while fields are edited', () => {
    const onQuery = vi.fn();
    render(
      <RiskWarningQueryBar fetching={false} onQuery={onQuery} onReset={vi.fn()} onRefresh={vi.fn()}>
        <label>风险等级<select aria-label="风险等级"><option>全部</option></select></label>
      </RiskWarningQueryBar>,
    );

    fireEvent.change(screen.getByLabelText('风险等级'), { target: { value: '全部' } });
    expect(onQuery).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    expect(onQuery).toHaveBeenCalledTimes(1);
  });

  it('closes a drawer from the overlay and Escape key', () => {
    const onClose = vi.fn();
    const { rerender } = render(
      <RiskWarningDrawer open title="风险详情" onClose={onClose}><p>详情</p></RiskWarningDrawer>,
    );

    fireEvent.click(screen.getByTestId('risk-warning-drawer-overlay'));
    expect(onClose).toHaveBeenCalledTimes(1);
    rerender(<RiskWarningDrawer open title="风险详情" onClose={onClose}><p>详情</p></RiskWarningDrawer>);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});
