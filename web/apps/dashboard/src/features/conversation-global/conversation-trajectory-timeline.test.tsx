import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { ConversationTrajectoryDay } from './conversation-global-api';
import { ConversationTrajectoryTimeline } from './conversation-trajectory-timeline';

afterEach(cleanup);

function renderTimeline(date: string) {
  const onDateChange = vi.fn();
  render(
    <ConversationTrajectoryTimeline
      data={undefined}
      date={date}
      conversationType="all"
      isLoading={false}
      onDateChange={onDateChange}
      onTypeChange={vi.fn()}
      onOpenEvent={vi.fn()}
    />,
  );
  return onDateChange;
}

describe('ConversationTrajectoryTimeline 日期切换', () => {
  it('前一天按日历只回退一天', () => {
    const onDateChange = renderTimeline('2026-08-20');

    expect(screen.queryByRole('button', { name: '刷新' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '前一天' }));

    expect(onDateChange).toHaveBeenCalledWith('2026-08-19');
  });

  it('从19号点击后一天回到20号', () => {
    const onDateChange = renderTimeline('2026-08-19');

    fireEvent.click(screen.getByRole('button', { name: '后一天' }));

    expect(onDateChange).toHaveBeenCalledWith('2026-08-20');
  });

  it('后一天按日历只前进一天并正确跨月', () => {
    const onDateChange = renderTimeline('2026-01-31');

    fireEvent.click(screen.getByRole('button', { name: '后一天' }));

    expect(onDateChange).toHaveBeenCalledWith('2026-02-01');
  });
});

describe('ConversationTrajectoryTimeline 时间轴滚动', () => {
  it('选中人员后只滚动时间轴内部，不滚动外层页面', () => {
    const scrollTo = vi.fn();
    const scrollIntoView = vi.fn();
    const originalScrollTo = HTMLElement.prototype.scrollTo;
    const originalScrollIntoView = HTMLElement.prototype.scrollIntoView;
    Object.defineProperty(HTMLElement.prototype, 'scrollTo', { configurable: true, value: scrollTo });
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: scrollIntoView });

    const data: ConversationTrajectoryDay = {
      employee: { id: 1002, name: '李娜', avatar: '' },
      date: '2026-08-20',
      timezone: 'Asia/Shanghai',
      metrics: {},
      events: [{
        id: 'event-1', conversationId: 'conversation-1', hour: '14', targetType: 'customer', targetId: 2001,
        targetName: '陈晓明', targetAvatar: '', targetStatus: 'available', messageTotal: 2,
        firstMessageAt: '2026-08-20 14:00:00', lastMessageAt: '2026-08-20 14:05:00',
      }],
      unmatchedTargetMessages: 0,
      limitations: [],
      capabilities: [],
    };

    try {
      const { container, rerender } = render(
        <ConversationTrajectoryTimeline
          data={undefined}
          date="2026-08-20"
          conversationType="all"
          isLoading={false}
          onDateChange={vi.fn()}
          onTypeChange={vi.fn()}
          onOpenEvent={vi.fn()}
        />,
      );
      rerender(
        <ConversationTrajectoryTimeline
          data={data}
          date="2026-08-20"
          conversationType="all"
          isLoading={false}
          onDateChange={vi.fn()}
          onTypeChange={vi.fn()}
          onOpenEvent={vi.fn()}
        />,
      );

      const timeline = container.querySelector('.conversation-trajectory-scroll');
      expect(timeline).not.toBeNull();
      expect(scrollIntoView).not.toHaveBeenCalled();
      expect(scrollTo).toHaveBeenCalledWith(expect.objectContaining({ top: expect.any(Number), behavior: 'auto' }));
      expect(scrollTo.mock.instances[0]).toBe(timeline);
    } finally {
      if (originalScrollTo) Object.defineProperty(HTMLElement.prototype, 'scrollTo', { configurable: true, value: originalScrollTo });
      else delete (HTMLElement.prototype as Partial<HTMLElement>).scrollTo;
      if (originalScrollIntoView) Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: originalScrollIntoView });
      else delete (HTMLElement.prototype as Partial<HTMLElement>).scrollIntoView;
    }
  });
});
