import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { ErrorState } from './error-state';
import { LoadingState } from './loading-state';

describe('LoadingState', () => {
  it('announces a customizable loading label as polite status', () => {
    const markup = renderToStaticMarkup(<LoadingState label="正在载入联系人" />);

    expect(markup).toContain('role="status"');
    expect(markup).toContain('aria-live="polite"');
    expect(markup).toContain('正在载入联系人');
  });
});

describe('ErrorState', () => {
  it('announces configuration failure details as an alert', () => {
    const markup = renderToStaticMarkup(
      <ErrorState title="前端配置错误" message="路由清单无效，应用已停止加载。" />,
    );

    expect(markup).toContain('role="alert"');
    expect(markup).toContain('前端配置错误');
    expect(markup).toContain('路由清单无效，应用已停止加载。');
  });
});
