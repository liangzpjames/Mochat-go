import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { expect, test, vi } from 'vitest';
import { SettingsPage } from './settings-page';

test('renders typed validated setting fields instead of arbitrary key value input', () => {
  const api = { read: vi.fn(), write: vi.fn() };
  render(<QueryClientProvider client={new QueryClient()}><SettingsPage api={api} /></QueryClientProvider>);
  expect(screen.getByLabelText('设置分类')).not.toBeNull();
  expect(screen.getByLabelText('配置编码')).not.toBeNull();
  expect(screen.getByLabelText('配置值')).not.toBeNull();
  expect(screen.getByLabelText('启用配置')).not.toBeNull();
  expect(screen.getByRole('button', { name: '新增配置' }).hasAttribute('disabled')).toBe(true);
});
