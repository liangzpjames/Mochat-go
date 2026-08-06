import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { expect, test, vi } from 'vitest';
import { SettingsPage } from './settings-page';
import { DashboardAccessProvider } from '../../app/access-context';

test('renders typed validated setting fields instead of arbitrary key value input', () => {
  const api = { read: vi.fn(), write: vi.fn() };
  render(<QueryClientProvider client={new QueryClient()}><SettingsPage api={api} /></QueryClientProvider>);
  expect(screen.getByLabelText('设置分类')).not.toBeNull();
  expect(screen.getByLabelText('配置编码')).not.toBeNull();
  expect(screen.getByLabelText('配置值')).not.toBeNull();
  expect(screen.getByLabelText('启用配置')).not.toBeNull();
  expect(screen.getByRole('button', { name: '新增配置' }).hasAttribute('disabled')).toBe(true);
});

test('shows setting actor and update time returned by API',async()=>{const api={read:vi.fn().mockResolvedValue([{id:'s1',type:'customer_source',key:'web',label:'官网',value:'官网',enabled:true,version:2,updatedBy:4,updatedAt:'2026-08-06T08:00:00Z'}]),write:vi.fn()};const access={corp:{id:'9'},session:{},menu:[],allowedRoutes:new Set(),allowedActions:new Set()} as never;render(<DashboardAccessProvider value={access}><QueryClientProvider client={new QueryClient()}><SettingsPage api={api}/></QueryClientProvider></DashboardAccessProvider>);expect(await screen.findByText(/4 \/ 2026-08-06/)).not.toBeNull()});
