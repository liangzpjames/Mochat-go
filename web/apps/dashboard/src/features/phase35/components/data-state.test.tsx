import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Phase35DataState } from './data-state';
describe('Phase35DataState', () => {
  it('renders loading, error, forbidden, limitation and content states', () => {
    const { rerender } = render(<Phase35DataState loading>content</Phase35DataState>); expect(screen.getByRole('status').textContent).toContain('加载');
    rerender(<Phase35DataState error={new Error()}>content</Phase35DataState>); expect(screen.getByRole('alert').textContent).toContain('失败');
    rerender(<Phase35DataState forbidden>content</Phase35DataState>); expect(screen.getByRole('alert').textContent).toContain('权限');
    rerender(<Phase35DataState limitations={['会话 Provider 不可用']}>content</Phase35DataState>); expect(screen.getByText(/会话 Provider/)).toBeTruthy();
  });
});
