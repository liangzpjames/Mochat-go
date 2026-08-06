import { fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { expect, test, vi } from 'vitest';
import { ConversionReportPage } from './conversion-report-page';
test('renders Chinese rates and opens a stage drill-down',async()=>{const api={read:vi.fn().mockResolvedValue({summary:{lead:10,contact:8,contactRate:.8,opportunity:4,opportunityRate:.5,won:2,wonRate:.5,order:1,orderRate:.5},items:[],pagination:{page:1,pageSize:20,total:0}}),write:vi.fn()};render(<QueryClientProvider client={new QueryClient()}><ConversionReportPage api={api}/></QueryClientProvider>);expect(await screen.findByText('联系人转化率')).not.toBeNull();fireEvent.click(await screen.findByRole('button',{name:/商机/}));expect(screen.getByRole('dialog',{name:'商机阶段明细'})).not.toBeNull()});
