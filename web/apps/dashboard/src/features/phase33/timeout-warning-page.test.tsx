import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { expect, it, vi } from 'vitest';
import { DashboardAccessProvider } from '../../app/access-context';
import type { AccessContext } from '../../app/access-loader';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { TimeoutWarningPage } from './timeout-warning-page';

const access: AccessContext={session:{token:'token',userId:'1',corpId:'7',expiresAt:null},corp:{id:'7',name:'测试企业',authorized:true},menu:[],allowedRoutes:new Set(['/ai-insight/v2/timeout']),allowedActions:new Set()};
it('loads timeout records and exposes all three tabs',async()=>{
 const read=vi.fn().mockResolvedValue({items:[]});const api:BusinessWorkbenchApi={read,write:vi.fn()};
 render(<MemoryRouter><QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><DashboardAccessProvider value={access}><TimeoutWarningPage api={api}/></DashboardAccessProvider></QueryClientProvider></MemoryRouter>);
 expect(screen.getByRole('button',{name:'超时记录'})).toBeTruthy();expect(screen.getByRole('button',{name:'规则配置'})).toBeTruthy();expect(screen.getByRole('button',{name:'高级设置'})).toBeTruthy();
 await waitFor(()=>expect(read).toHaveBeenCalledWith('/timeout-warning/records',{page:1,perPage:20}));
 expect(screen.queryByText('数据提供方未接入')).toBeNull();
});
