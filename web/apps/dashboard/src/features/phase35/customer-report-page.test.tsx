import {render,screen} from '@testing-library/react';
import {QueryClient,QueryClientProvider} from '@tanstack/react-query';
import {vi,test,expect} from 'vitest';
import {CustomerReportPage} from './customer-report-page';
test('renders customer report summary',async()=>{const api={read:vi.fn().mockResolvedValue({summary:{customer:2},items:[]}),write:vi.fn()};render(<QueryClientProvider client={new QueryClient()}><CustomerReportPage api={api}/></QueryClientProvider>);expect((await screen.findAllByText('2')).length).toBeGreaterThan(0)});
test('explains customer data source and next action when empty',async()=>{const api={read:vi.fn().mockResolvedValue({summary:{customer:0},items:[],pagination:{total:0,page:1,pageSize:20}}),write:vi.fn()};render(<QueryClientProvider client={new QueryClient()}><CustomerReportPage api={api}/></QueryClientProvider>);expect(await screen.findByRole('region',{name:'客户分析数据说明'})).not.toBeNull();expect(screen.getByText(/真实 SCRM 联系人/)).not.toBeNull()});
