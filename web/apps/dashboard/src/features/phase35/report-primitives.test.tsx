import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, expect, test } from 'vitest';

import { ReportPrimitives } from './report-primitives';

afterEach(cleanup);

test('does not expose raw report limitation details', () => {
  render(<ReportPrimitives result={{ limitations: [{ provider: 'report', message: '报表 Provider 未提供' }] }} />);

  expect(screen.getByText('部分数据暂未同步完整，当前仅展示已获取的数据。')).not.toBeNull();
  expect(screen.queryByText(/Provider/i)).toBeNull();
});
