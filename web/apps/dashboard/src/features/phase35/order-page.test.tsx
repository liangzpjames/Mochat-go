import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, expect, test, vi } from 'vitest';
import { OrderPage } from './order-page';

afterEach(cleanup);

test('uses typed business fields and blocks an incomplete order', () => {
  const api = { read: vi.fn(), write: vi.fn() };
  render(<QueryClientProvider client={new QueryClient()}><OrderPage api={api} /></QueryClientProvider>);
  expect(screen.getByLabelText('联系人')).not.toBeNull();
  expect(screen.getByLabelText('订单标题')).not.toBeNull();
  expect(screen.getByLabelText('金额（元）')).not.toBeNull();
  expect(screen.getByLabelText('备注')).not.toBeNull();
  fireEvent.change(screen.getByLabelText('金额（元）'), { target: { value: '12.345' } });
  expect(screen.getByRole('button', { name: '创建订单' }).hasAttribute('disabled')).toBe(true);
  expect(screen.queryByText(/\{\s*"/)).toBeNull();
});
