import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, expect, test, vi } from 'vitest';
import { SettingsPage } from './settings-page';
import { DashboardAccessProvider } from '../../app/access-context';

afterEach(cleanup);

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

test('cancels setting editing and restores create mode without changing the list', async () => {
  const row = { id: 's1', type: 'funnel_stage', key: 'proposal', label: '转化阶段', value: '方案确认', enabled: true, version: 2, updatedBy: 4, updatedAt: '2026-08-06T08:00:00Z' };
  const api = { read: vi.fn().mockResolvedValue([row]), write: vi.fn() };
  const access = { corp: { id: '9' }, session: {}, menu: [], allowedRoutes: new Set(), allowedActions: new Set() } as never;
  render(<DashboardAccessProvider value={access}><QueryClientProvider client={new QueryClient()}><SettingsPage api={api} /></QueryClientProvider></DashboardAccessProvider>);

  fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
  expect(screen.getByRole('button', { name: '保存修改' })).not.toBeNull();
  expect(screen.getByRole('button', { name: '取消编辑' })).not.toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '取消编辑' }));

  expect(screen.getByRole('button', { name: '新增配置' })).not.toBeNull();
  expect(screen.getByLabelText('配置编码')).toHaveProperty('value', '');
  expect(screen.getByLabelText('配置值')).toHaveProperty('value', '');
  expect(screen.getByText('方案确认')).not.toBeNull();
  expect(api.read).toHaveBeenCalledTimes(1);
});
