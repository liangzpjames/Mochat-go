import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeAll, expect, it, vi } from 'vitest';
import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { TimeoutWarningPage } from './timeout-warning-page';

const access: AccessContext={session:{token:'token',userId:'1',corpId:'7',expiresAt:null},corp:{id:'7',name:'测试企业',authorized:true},menu:[],allowedRoutes:new Set(['/ai-insight/v2/timeout']),allowedActions:new Set()};
afterEach(cleanup);
beforeAll(() => { globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} }; });
it('loads timeout records and exposes all three tabs',async()=>{
 const read=vi.fn().mockResolvedValue({items:[]});const api:BusinessWorkbenchApi={read,write:vi.fn()};
 render(<MemoryRouter><QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><DashboardAccessProvider value={access}><TimeoutWarningPage api={api}/></DashboardAccessProvider></QueryClientProvider></MemoryRouter>);
 expect(screen.getByRole('button',{name:'超时记录'})).toBeTruthy();expect(screen.getByRole('button',{name:'规则配置'})).toBeTruthy();expect(screen.getByRole('button',{name:'高级设置'})).toBeTruthy();
 await waitFor(()=>expect(read).toHaveBeenCalledWith('/timeout-warning/records',{page:1,perPage:20}));
 expect(screen.queryByText('数据提供方未接入')).toBeNull();
});

it('does not toggle a named timeout rule before confirmation', async () => {
 const read=vi.fn().mockResolvedValue({items:[{id:8,name:'十分钟未回复',status:'enabled',monitorTarget:'all',conversationScopes:['single'],triggerCount:2,createdAt:'2026-08-08'}]});
 const write=vi.fn().mockResolvedValue({});const api:BusinessWorkbenchApi={read,write};
 render(<MemoryRouter><QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><DashboardAccessProvider value={access}><TimeoutWarningPage api={api}/></DashboardAccessProvider></QueryClientProvider></MemoryRouter>);
 fireEvent.click(screen.getByRole('button',{name:'规则配置'}));
 const disable=await screen.findByRole('button',{name:'停用 十分钟未回复'});
 fireEvent.click(disable);
 expect(write).not.toHaveBeenCalledWith('/timeout-warning/rules/status',expect.anything(),'PUT');
 fireEvent.click(await screen.findByRole('button',{name:'确认'}));
 await waitFor(()=>expect(write).toHaveBeenCalledTimes(1));
});
